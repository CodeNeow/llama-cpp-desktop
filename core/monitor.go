package core

import (
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ─── 实时监控（CPU / 内存 / GPU / TPS）──────────────────────────────
//
// 采样器以 1s 间隔在后台 goroutine 运行，采样结果缓存到 monitorStatus（monitorMu
// 保护），GetMonitorStatus 在锁内拷贝返回，避免 Wails 调用与采样并发读写同一
// 结构。CPU/内存解析全部为纯函数，测试只测解析、不 mock runCmd。

// MonitorStatus 是 GetMonitorStatus 返回给前端的 JSON 契约，字段与
// frontend/src/lib/monitor.ts 的 MonitorStatus 一一对应，改动需两侧同步。
type MonitorStatus struct {
	CPUPercent    float64      `json:"cpuPercent"`
	MemUsed       uint64       `json:"memUsed"`
	MemTotal      uint64       `json:"memTotal"`
	GPUs          []MonitorGPU `json:"gpus"`
	ServerRunning bool         `json:"serverRunning"`
	PromptTPS     float64      `json:"promptTps"`
	DecodeTPS     float64      `json:"decodeTps"`
	UptimeSeconds int64        `json:"uptimeSeconds"`
}

// MonitorGPU 是单个 GPU 的监控数据（nvidia-smi 采样）。
type MonitorGPU struct {
	Index       int     `json:"index"`
	Name        string  `json:"name"`
	UtilPercent float64 `json:"utilPercent"`
	MemUsed     uint64  `json:"memUsed"`
	MemTotal    uint64  `json:"memTotal"`
}

// monitorStatus 为采样缓存，monitorMu 保护读写。
var monitorStatus MonitorStatus
var monitorMu sync.Mutex

// monitorOnce 保证采样器只启动一次（StartMonitorSampler 由 app.Startup 调用，
// 幂等防重复启动）。
var monitorOnce sync.Once

// linuxCPUPrevIdle / linuxCPUPrevTotal 为 Linux /proc/stat 两次采样差值的
// 上次采样值（仅采样 goroutine 读写，无需加锁）；首轮返回 0。
var linuxCPUPrevIdle, linuxCPUPrevTotal uint64

// tpsLogRegex 匹配 llama-server 日志中的吞吐片段 "N tokens per second"
// （预填充 prompt eval 行与解码 eval 行都含该片段，属于哪类由 parsePromptTPS /
// parseDecodeTPS 按行分类决定）。
var tpsLogRegex = regexp.MustCompile(`([\d.]+)\s+tokens?\s+per\s+second`)

// tg3sLogRegex 匹配实时解码行的 3 秒窗口速度 "tg_3s = N t/s"（N 为浮点数）。
var tg3sLogRegex = regexp.MustCompile(`tg_3s\s*=\s*([\d.]+)\s+t/s`)

// splitLogLines 先把日志条目拼成文本再按行切分：历史遗留的多行条目（一次
// stderr Write 含多行）在此被拆成独立行，与写入侧行缓冲（serverLogWriter）
// 双保险，保证 print_timing 行整体进入后续分类、预填充值不会漏入解码速度。
// 纯函数：空列表经 Join 为 "" 再 Split 得到单条空行（各解析函数无关键词命中
// 时自然返回 0）。
func splitLogLines(logs []string) []string {
	return strings.Split(strings.Join(logs, "\n"), "\n")
}

// parsePromptTPS 从服务日志行中提取最后一条预填充行的 "N tokens per second"
// 数值（tokens/s），即提示词处理 / 预填充（prefill）速度。llama-server 的预填充
// 计时来自两类行，实时行（新版 llama.cpp）与终结行（新旧版本都有）：
//
//	I slot print_timing: id  3 | task 0 | prompt processing, n_tokens =   2048, progress = 0.16, t =  20.47 s / 100.05 tokens per second
//	I slot print_timing: id  3 | task 0 | prompt eval time =    357.49 ms /     27 tokens (   75.53 tokens per second)
//
// 实时行（prompt processing）是预填充期间按批打印的进度行，仅当预填充耗时
// >=3s 时出现（短预填充没有该行）；终结行（prompt eval time）在每次请求结束时
// 打印。两者都提取、多行取最后一条：实时行持续刷新，日志顺序天然保证终结行
// 最后出现，终结值即最终权威值；旧版二进制只有终结行，同样兼容。该值反映提示
// 词吞吐（长 prompt 上可达数千 t/s），与解码速度是两个独立指标，由监控页
// 「推理」模块展示。纯函数：无预填充行返回 0；多行多值取最后一条预填充行
// （最新采样为准）；单行内多个匹配只取第一个命中（regexp FindStringSubmatch
// 语义）；支持小数。
func parsePromptTPS(logs []string) float64 {
	var last float64
	for _, line := range splitLogLines(logs) {
		// 两类预填充行都认：新版预填充实时行（prompt processing）与新旧版通用
		// 的请求结束终结行（prompt eval time）。
		if !strings.Contains(line, "prompt processing") && !strings.Contains(line, "prompt eval time") {
			continue
		}
		if m := tpsLogRegex.FindStringSubmatch(line); m != nil {
			if v, err := strconv.ParseFloat(m[1], 64); err == nil {
				last = v
			}
		}
	}
	return last
}

// parseDecodeTPS 从服务日志行中提取生成（解码）实时速度（tokens/s）。路由器
// 模式 llama-server 生成期间每约 3 秒打印一行实时统计、请求结束打印总计时行：
//
//	I slot print_timing: id  3 | task 0 | n_decoded =    414, tg =  68.82 t/s, tg_3s =  67.32 t/s
//	I slot print_timing: id  3 | task 0 |        eval time =  12334.07 ms /    900 tokens (   72.97 tokens per second)
//
// 新版本行带 "id N | task N |" 前缀、eval time 行数值前有多余空格：行分类依赖
// 关键词 Contains、数值提取依赖 tpsLogRegex / tg3sLogRegex 子串匹配，前缀与空格
// 均不影响命中，旧版无前缀行同样兼容。实时行中 tg 为累计平均解码速度、tg_3s 为
// 最近 3 秒窗口速度（实时值），本函数只取 tg_3s 并优先返回（即使其后还有 eval
// time 行，实时值优先于请求结束时的总计时）；旧版二进制无实时行时回退最后一条
// eval time 行的数值。回退分支严格要求 "eval time" 标记：仅剩 "tokens per second"
// 片段的截断分片不再被采用（分片重组是写入侧行缓冲 serverLogWriter 的职责）。
// 曾经的「TPS 恒 0」根因：旧实现只解析生成结束才打印的 eval time 行，而实时行
// 不含 "tokens per second"，导致生成期间无值。
// 纯函数：无候选返回 0；多行多值取最后一行（最新采样为准）；支持小数。
func parseDecodeTPS(logs []string) float64 {
	var lastTg3s, lastEval float64
	for _, line := range splitLogLines(logs) {
		if strings.Contains(line, "tg_3s =") {
			if m := tg3sLogRegex.FindStringSubmatch(line); m != nil {
				if v, err := strconv.ParseFloat(m[1], 64); err == nil {
					lastTg3s = v
				}
			}
			continue
		}
		// 预填充行先行排除：prompt eval time 是 eval time 的子串，且其中同样含
		// "tokens per second" 片段（如 75.53），不跳过会污染解码速度。
		if strings.Contains(line, "prompt eval time") {
			continue
		}
		if strings.Contains(line, "eval time") {
			if m := tpsLogRegex.FindStringSubmatch(line); m != nil {
				if v, err := strconv.ParseFloat(m[1], 64); err == nil {
					lastEval = v
				}
			}
		}
	}
	if lastTg3s > 0 {
		return lastTg3s
	}
	return lastEval
}

// StartMonitorSampler 启动后台采样器（sync.Once 保证只启动一次）。采样间隔
// 1s，无限循环直到进程退出；每一轮刷新 monitorStatus 缓存。
func StartMonitorSampler() {
	monitorOnce.Do(func() {
		go func() {
			for {
				sampleMonitor()
				time.Sleep(1 * time.Second)
			}
		}()
	})
}

// GetMonitorStatus 返回当前监控采样快照：读 monitorStatus 缓存（monitorMu），
// ServerRunning / UptimeSeconds / PromptTPS / DecodeTPS 每次现取（serverMu /
// serverLogsMu），保证缓存采样周期内这些高频变化字段仍是实时的。
func GetMonitorStatus() *MonitorStatus {
	monitorMu.Lock()
	st := monitorStatus
	monitorMu.Unlock()

	serverMu.Lock()
	running := serverRunning
	startTime := serverStartTime
	serverMu.Unlock()
	st.ServerRunning = running
	if running && !startTime.IsZero() {
		st.UptimeSeconds = int64(time.Since(startTime).Seconds())
	} else {
		st.UptimeSeconds = 0
	}

	// TPS：读服务日志尾部 50 条（serverLogsMu），现算预填充与解码速度
	serverLogsMu.Lock()
	logs := serverLogs
	if n := len(logs); n > 50 {
		logs = logs[n-50:]
	}
	tail := make([]string, len(logs))
	copy(tail, logs)
	serverLogsMu.Unlock()
	st.PromptTPS = parsePromptTPS(tail)
	st.DecodeTPS = parseDecodeTPS(tail)

	return &st
}

// sampleMonitor 执行一轮系统采样并更新缓存（monitorMu）。各平台解析均使用
// runCmd（hideWindow 已内置，不弹控制台窗口）。
func sampleMonitor() {
	var st MonitorStatus
	switch runtime.GOOS {
	case "windows":
		st.CPUPercent = sampleCPUWindows()
		st.MemTotal, st.MemUsed = sampleMemWindows()
	case "linux":
		st.CPUPercent = sampleCPULinux()
		st.MemTotal, st.MemUsed = sampleMemLinux()
	case "darwin":
		st.CPUPercent = sampleCPUDarwin()
		st.MemTotal, st.MemUsed = sampleMemDarwin()
	}
	st.GPUs = sampleGPUs()
	monitorMu.Lock()
	monitorStatus = st
	monitorMu.Unlock()
}

// ─── CPU 采样 ─────────────────────────────────────────────────────

// parseCPUWindows 解析 PowerShell Win32_Processor 平均负载百分比输出
// （`(Get-CimInstance Win32_Processor | Measure-Object -Property LoadPercentage
// -Average).Average`），返回 0-100 的浮点数。多核多包输出可能含多行/多值，
// 取首个可解析数字；无有效数字返回 0。
func parseCPUWindows(out string) float64 {
	for _, line := range strings.Split(out, "\n") {
		for _, f := range strings.Fields(line) {
			if v, err := strconv.ParseFloat(f, 64); err == nil {
				return v
			}
		}
	}
	return 0
}

// sampleCPUWindows 通过 PowerShell WMI 查询平均负载百分比。
func sampleCPUWindows() float64 {
	return parseCPUWindows(runCmd("powershell", "-NoProfile", "-Command",
		"(Get-CimInstance Win32_Processor | Measure-Object -Property LoadPercentage -Average).Average"))
}

// parseProcStat 解析 /proc/stat 首行 "cpu ..." 的 idle 与总 ticks（jiffies）。
// 纯函数，返回 (idle, total)；解析失败或找不到 cpu 行返回 (0,0)。
func parseProcStat(out string) (idle, total uint64) {
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 5 || fields[0] != "cpu" {
			continue
		}
		var sum uint64
		for _, f := range fields[1:] {
			if v, err := strconv.ParseUint(f, 10, 64); err == nil {
				sum += v
			}
		}
		idleVal, err := strconv.ParseUint(fields[4], 10, 64)
		if err != nil {
			idleVal = 0
		}
		return idleVal, sum
	}
	return 0, 0
}

// cpuPercentFromDeltas 由两次采样差值计算 CPU 占用率（%）：idle 与 total 的
// 增量比值，100*(1 - Δidle/Δtotal)；Δtotal<=0（无有效间隔）返回 0。
// 纯函数，供 parseProcStat 测试与 sampleCPULinux 共用。
func cpuPercentFromDeltas(prevIdle, prevTotal, curIdle, curTotal uint64) float64 {
	totalDelta := curTotal - prevTotal
	if totalDelta <= 0 {
		return 0
	}
	idleDelta := curIdle - prevIdle
	if idleDelta > totalDelta {
		idleDelta = totalDelta
	}
	return 100 * (1 - float64(idleDelta)/float64(totalDelta))
}

// sampleCPULinux 用两次 /proc/stat 采样差值计算 CPU 占用率（%）。采样器持
// prevIdle/prevTotal 包级状态；首轮无前值返回 0。
func sampleCPULinux() float64 {
	curIdle, curTotal := parseProcStat(runCmd("cat", "/proc/stat"))
	if curTotal == 0 {
		return 0
	}
	if linuxCPUPrevTotal == 0 {
		linuxCPUPrevIdle = curIdle
		linuxCPUPrevTotal = curTotal
		return 0
	}
	pct := cpuPercentFromDeltas(linuxCPUPrevIdle, linuxCPUPrevTotal, curIdle, curTotal)
	linuxCPUPrevIdle = curIdle
	linuxCPUPrevTotal = curTotal
	return pct
}

// parseLoadAvg 解析 `sysctl -n vm.loadavg`（如 "1.23 0.98 0.76 2/345 12345"），
// 返回第一个值（1 分钟平均负载）。
func parseLoadAvg(out string) float64 {
	fields := strings.Fields(strings.TrimSpace(out))
	if len(fields) == 0 {
		return 0
	}
	v, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0
	}
	return v
}

// sampleCPUDarwin 用 loadavg / NumCPU * 100 估算 CPU 占用率（近似）。
func sampleCPUDarwin() float64 {
	load := parseLoadAvg(runCmd("sysctl", "-n", "vm.loadavg"))
	ncpu := runtime.NumCPU()
	if ncpu == 0 {
		return 0
	}
	return load * 100 / float64(ncpu)
}

// ─── 内存采样 ─────────────────────────────────────────────────────

// parseMemWindowsKB 解析 PowerShell Win32_OperatingSystem 输出
// （TotalVisibleMemorySize / FreePhysicalMemory，单位 KB）。期望输出两列数值，
// 返回 (total, free) 字节（KB→bytes ×1024）。解析不到两列时按可解析数量尽力返回。
func parseMemWindowsKB(out string) (total, free uint64) {
	var nums []uint64
	for _, line := range strings.Split(out, "\n") {
		for _, f := range strings.Fields(line) {
			if v, err := strconv.ParseUint(f, 10, 64); err == nil {
				nums = append(nums, v)
			}
		}
	}
	if len(nums) >= 2 {
		return nums[0] * 1024, nums[1] * 1024
	}
	if len(nums) == 1 {
		return nums[0] * 1024, 0
	}
	return 0, 0
}

// sampleMemWindows 通过 PowerShell 读取总/空闲物理内存（KB），换算字节后
// 返回 (total, used)。
func sampleMemWindows() (total, used uint64) {
	out := runCmd("powershell", "-NoProfile", "-Command",
		"$os=Get-CimInstance Win32_OperatingSystem; \"$($os.TotalVisibleMemorySize) $($os.FreePhysicalMemory)\"")
	total, free := parseMemWindowsKB(out)
	if free <= total {
		used = total - free
	}
	return total, used
}

// parseMemLinux 解析 /proc/meminfo 中的 MemTotal 与 MemAvailable（KB→bytes），
// 返回 (total, avail) 字节。MemAvailable 缺失时回退 MemFree。
func parseMemLinux(out string) (total, avail uint64) {
	total = parseMemInfo(out, "MemTotal") * 1024
	if a := parseMemInfo(out, "MemAvailable"); a > 0 {
		avail = a * 1024
	} else {
		avail = parseMemInfo(out, "MemFree") * 1024
	}
	return
}

// sampleMemLinux 读取 /proc/meminfo 计算 (total, used) 字节。
func sampleMemLinux() (total, used uint64) {
	total, avail := parseMemLinux(runCmd("cat", "/proc/meminfo"))
	if avail <= total {
		used = total - avail
	}
	return total, used
}

// sampleMemDarwin 用 `sysctl -n hw.memsize`（总字节）+ `vm_stat`（空闲页数）
// 计算 (total, used) 字节。hw.pagesize 解析失败回退 16384（arm64 默认页大小）。
func sampleMemDarwin() (total, used uint64) {
	out := runCmd("sysctl", "-n", "hw.memsize")
	total, _ = strconv.ParseUint(strings.TrimSpace(out), 10, 64)
	freePages := parseVMStat(runCmd("vm_stat"), "free")
	if freePages == 0 {
		return total, 0
	}
	pageSize := uint64(16384)
	if psStr := strings.TrimSpace(runCmd("sysctl", "-n", "hw.pagesize")); psStr != "" {
		if ps, err := strconv.ParseUint(psStr, 10, 64); err == nil && ps > 0 {
			pageSize = ps
		}
	}
	free := freePages * pageSize
	if free <= total {
		used = total - free
	} else {
		used = 0
	}
	return total, used
}

// ─── GPU 采样 ─────────────────────────────────────────────────────

// parseNVLine 解析一行 nvidia-smi csv 输出
// （`--query-gpu=index,name,utilization.gpu,memory.used,memory.total
// --format=csv,noheader,nounits`）。返回 MonitorGPU；缺失列对应字段为 0，
// 不报错（宽松解析）；列数不足 5 时尽力解析可用列。
func parseNVLine(line string) MonitorGPU {
	g := MonitorGPU{Index: -1}
	parts := strings.Split(line, ",")
	if v, err := strconv.Atoi(strings.TrimSpace(parts[0])); err == nil {
		g.Index = v
	}
	if len(parts) > 1 {
		g.Name = strings.TrimSpace(parts[1])
	}
	if len(parts) > 2 {
		if v, err := strconv.ParseFloat(strings.TrimSpace(parts[2]), 64); err == nil {
			g.UtilPercent = v
		}
	}
	if len(parts) > 3 {
		if v, err := strconv.ParseUint(strings.TrimSpace(parts[3]), 10, 64); err == nil {
			g.MemUsed = v * 1024 * 1024 // MiB → bytes
		}
	}
	if len(parts) > 4 {
		if v, err := strconv.ParseUint(strings.TrimSpace(parts[4]), 10, 64); err == nil {
			g.MemTotal = v * 1024 * 1024
		}
	}
	return g
}

// sampleGPUs 调用 nvidia-smi 采样所有 GPU；失败（无 nvidia-smi / 无 GPU）返回
// 空列表不报错。内存单位 MiB→bytes 换算在 parseNVLine 完成。
func sampleGPUs() []MonitorGPU {
	out := runCmd("nvidia-smi",
		"--query-gpu=index,name,utilization.gpu,memory.used,memory.total",
		"--format=csv,noheader,nounits")
	if out == "" {
		return nil
	}
	var gpus []MonitorGPU
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		g := parseNVLine(line)
		if g.Index < 0 {
			continue
		}
		gpus = append(gpus, g)
	}
	return gpus
}
