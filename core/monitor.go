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
	TPS           float64      `json:"tps"`
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

// tpsLogRegex 匹配 llama-server 日志中的吞吐行，如 "12.34 tokens per second"。
// 预填充（prompt eval）与解码（eval）行都含该片段，是否采用由 parseTPS 按行
// 分类决定（预填充行先行排除）。
var tpsLogRegex = regexp.MustCompile(`([\d.]+)\s+tokens?\s+per\s+second`)

// parseTPS 从服务日志行中提取最后一条解码行的 "N tokens per second" 数值
// （tokens/s）。llama-server 每次生成结束会打印两行 timing：
//
//	I slot print_timing:      prompt eval time =     271.14 ms /    15 tokens (   18.08 ms per token,    55.32 tokens per second)
//	I slot print_timing:             eval time =     712.56 ms /    64 tokens (   11.13 ms per token,    89.82 tokens per second)
//
// 第一行是提示词预填充（prefill）速度，长 prompt 上可达数千 t/s（如监控页曾显示
// 2362.8 t/s），不是用户认知的推理速度；第二行才是生成解码速度，必须只取解码行。
// 注意 "prompt eval time" 是 "eval time" 的子串，必须先判 prompt 再判 eval。
// 纯函数：无解码行返回 0；多行多值取最后一条解码行（最新采样为准）；支持小数。
func parseTPS(logs []string) float64 {
	var last float64
	for _, line := range logs {
		// 预填充行先行排除：其中同样含 "tokens per second" 片段（如 55.32），
		// 不跳过会污染 TPS（长 prompt 预填充可达 2362.8 t/s 这类荒谬值）。
		if strings.Contains(line, "prompt eval time") {
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
// ServerRunning / UptimeSeconds / TPS 每次现取（serverMu / serverLogsMu），保证
// 缓存采样周期内这些高频变化字段仍是实时的。
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

	// TPS：读服务日志尾部 50 条（serverLogsMu）
	serverLogsMu.Lock()
	logs := serverLogs
	if n := len(logs); n > 50 {
		logs = logs[n-50:]
	}
	tail := make([]string, len(logs))
	copy(tail, logs)
	serverLogsMu.Unlock()
	st.TPS = parseTPS(tail)

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
