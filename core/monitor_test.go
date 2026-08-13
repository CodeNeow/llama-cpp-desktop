package core

import "testing"

// ─── parseTPS ─────────────────────────────────────────────────────

// TestParseTPS 验证从服务日志提取解码（生成）速度：只采用解码行（eval time）
// 的数值，预填充行（prompt eval time）必须排除；多行多值取最后一条解码行
// （最新采样为准）、支持小数、支持 token 单复数、无解码行返回 0。单行内的多个
// 匹配只取第一个命中（regexp FindStringSubmatch 语义），以此为准断言。
func TestParseTPS(t *testing.T) {
	cases := []struct {
		name string
		logs []string
		want float64
	}{
		{"no match", []string{"loading model", "another line"}, 0},
		{"single", []string{"12.34 tokens per second"}, 12.34},
		{"singular token", []string{"5 token per second"}, 5},
		{"last of multiple", []string{"12.34 tokens per second", "log noise", "56.7 tokens per second"}, 56.7},
		{"first match in single line", []string{"a 3.5 tokens per second and 4.5 tokens per second"}, 3.5},
		{"empty", nil, 0},
		// 真实 llama-server 输出：预填充行在前、解码行在后（print_timing 实际
		// 打印顺序），须返回解码行数值 89.82，不能取预填充行的 55.32。
		{"prefill then decode",
			[]string{
				"I slot print_timing:      prompt eval time =     271.14 ms /    15 tokens (   18.08 ms per token,    55.32 tokens per second)",
				"I slot print_timing:             eval time =     712.56 ms /    64 tokens (   11.13 ms per token,    89.82 tokens per second)",
			}, 89.82},
		// 只有预填充行、无解码行：预填充行被跳过，无有效值，返回 0。
		{"prefill only",
			[]string{
				"I slot print_timing:      prompt eval time =     271.14 ms /    15 tokens (   18.08 ms per token,    55.32 tokens per second)",
			}, 0},
		// 多轮生成多对 timing：预填充行（含 4230.00 这类长 prompt 高值）不得覆盖
		// 解码值，取最后一轮解码行的 95.40。
		{"multiple rounds",
			[]string{
				"I slot print_timing:      prompt eval time =     271.14 ms /    15 tokens (   18.08 ms per token,    55.32 tokens per second)",
				"I slot print_timing:             eval time =     712.56 ms /    64 tokens (   11.13 ms per token,    89.82 tokens per second)",
				"llama_server: unrelated log noise",
				"I slot print_timing:      prompt eval time =  4230.00 ms / 10000 tokens (    0.42 ms per token,  2362.80 tokens per second)",
				"I slot print_timing:             eval time =     500.00 ms /    50 tokens (   10.00 ms per token,    95.40 tokens per second)",
			}, 95.40},
		// 解码行带 llama-server 前缀（I slot print_timing:）与毫秒/ms 单位等
		// 噪声，正则仍只提取 "tokens per second" 数值。
		{"decode line with prefix noise",
			[]string{
				"llama server   : I slot print_timing:             eval time =     712.56 ms /    64 tokens (   11.13 ms per token,    89.82 tokens per second)",
			}, 89.82},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := parseTPS(c.logs); got != c.want {
				t.Errorf("parseTPS(%v) = %v, want %v", c.logs, got, c.want)
			}
		})
	}
}

// ─── parseNVLine ──────────────────────────────────────────────────

// TestParseNVLine 验证 nvidia-smi 单行 CSV 解析：完整 5 列、缺列（尽力解析）、
// 空串与非法数值不 panic 且对应字段为默认值。内存单位 MiB→bytes。
func TestParseNVLine(t *testing.T) {
	g := parseNVLine("0, NVIDIA GeForce RTX 4090, 45, 1024, 24576")
	if g.Index != 0 || g.Name != "NVIDIA GeForce RTX 4090" || g.UtilPercent != 45 {
		t.Errorf("完整行解析错误: %+v", g)
	}
	if g.MemUsed != 1024*1024*1024 || g.MemTotal != 24576*1024*1024 {
		t.Errorf("内存单位应为 MiB→bytes: used=%d total=%d", g.MemUsed, g.MemTotal)
	}

	// 缺列：尽力解析可用列，缺失字段为 0
	short := parseNVLine("1, Intel Arc, 30")
	if short.Index != 1 || short.Name != "Intel Arc" || short.UtilPercent != 30 {
		t.Errorf("缺列行解析错误: %+v", short)
	}
	if short.MemUsed != 0 || short.MemTotal != 0 {
		t.Errorf("缺列时内存应保持 0: %+v", short)
	}

	// 空串：Split 至少返回一个元素，不得越界 panic，Index 默认 -1
	empty := parseNVLine("")
	if empty.Index != -1 {
		t.Errorf("空串解析 Index = %d, want -1", empty.Index)
	}

	// 非法数值：宽松解析不 panic，对应字段为默认值
	bad := parseNVLine("abc, N, xyz, notnum, 0")
	if bad.Index != -1 || bad.Name != "N" || bad.UtilPercent != 0 || bad.MemUsed != 0 || bad.MemTotal != 0 {
		t.Errorf("非法数值行解析错误: %+v", bad)
	}
}

// ─── parseCPUWindows ──────────────────────────────────────────────

// TestParseCPUWindows 验证 PowerShell 输出解析：取首个可解析数字（多行/多字段
// 均容忍）；无有效数字返回 0。
func TestParseCPUWindows(t *testing.T) {
	if got := parseCPUWindows("45.5\n"); got != 45.5 {
		t.Errorf("parseCPUWindows(%q) = %v, want 45.5", "45.5\n", got)
	}
	if got := parseCPUWindows("some text\n12.25\n"); got != 12.25 {
		t.Errorf("parseCPUWindows 多行应取首个数字, got %v", got)
	}
	if got := parseCPUWindows("no numbers here"); got != 0 {
		t.Errorf("parseCPUWindows 无数字应返回 0, got %v", got)
	}
}

// ─── parseMemWindowsKB ────────────────────────────────────────────

// TestParseMemWindowsKB 验证 Windows 内存 KB 解析：两列数值按 ×1024 换算字节；
// 仅一列时尽力返回 (total, 0)；无数字返回 (0,0)。
func TestParseMemWindowsKB(t *testing.T) {
	total, free := parseMemWindowsKB("8000000 4000000")
	if total != 8000000*1024 || free != 4000000*1024 {
		t.Errorf("parseMemWindowsKB = (%d, %d), want (%d, %d)（KB→bytes ×1024）",
			total, free, 8000000*1024, 4000000*1024)
	}

	one, f := parseMemWindowsKB("8000000")
	if one != 8000000*1024 || f != 0 {
		t.Errorf("单列解析 = (%d, %d), want (%d, 0)", one, f, 8000000*1024)
	}

	z, zf := parseMemWindowsKB("")
	if z != 0 || zf != 0 {
		t.Errorf("空输出解析 = (%d, %d), want (0,0)", z, zf)
	}
}

// ─── parseProcStat ────────────────────────────────────────────────

// TestParseProcStat 验证 /proc/stat 首行 cpu 的 idle 与总 ticks 解析：
// idle 取第 4 列（user nice system idle ... 中的 idle），total 为全部数值列
// 之和；缺失 cpu 行返回 (0,0)；cpu0 等其它行不应误匹配。
func TestParseProcStat(t *testing.T) {
	// 标准 /proc/stat 首行：user nice system idle iowait irq softirq steal guest guest_nice
	out := "cpu  335973 18239 179330 1098596 18998 0 13275 0 0 0\n" +
		"cpu0 100 100 100 100 100\n" +
		"intr 12345\n"
	idle, total := parseProcStat(out)
	if idle != 1098596 {
		t.Errorf("idle = %d, want 1098596", idle)
	}
	wantTotal := uint64(335973 + 18239 + 179330 + 1098596 + 18998 + 0 + 13275)
	if total != wantTotal {
		t.Errorf("total = %d, want %d", total, wantTotal)
	}

	// 缺失 cpu 行返回 (0,0)
	if idle, total := parseProcStat("intr 123\nctxt 456\n"); idle != 0 || total != 0 {
		t.Errorf("缺失 cpu 行应返回 (0,0), got (%d, %d)", idle, total)
	}

	// cpu0 行不应误匹配（fields[0]=="cpu0" != "cpu"）
	if idle, total := parseProcStat("cpu0 1 2 3 4 5\n"); idle != 0 || total != 0 {
		t.Errorf("cpu0 行不应匹配, got (%d, %d)", idle, total)
	}
}

// ─── cpuPercentFromDeltas ─────────────────────────────────────────

// TestCPUPercentFromDeltas 验证 CPU 占用率计算：Δtotal<=0 返回 0；
// idle 增量超过 total 增量时钳制到 100%（避免计数器回绕/跨核场景超界）。
func TestCPUPercentFromDeltas(t *testing.T) {
	if got := cpuPercentFromDeltas(100, 1000, 150, 1100); got != 50 {
		t.Errorf("正常差值应得 50%%, got %v", got)
	}
	if got := cpuPercentFromDeltas(100, 1000, 200, 1000); got != 0 {
		t.Errorf("Δtotal=0 应返回 0, got %v", got)
	}
	if got := cpuPercentFromDeltas(100, 1000, 300, 1100); got != 0 {
		t.Errorf("idle 增量 > total 增量应钳制, got %v", got)
	}
}

// ─── parseLoadAvg ─────────────────────────────────────────────────

// TestParseLoadAvg 验证 loadavg 解析：取第一个值（1 分钟平均负载）；
// 空串或非法输入返回 0。
func TestParseLoadAvg(t *testing.T) {
	if got := parseLoadAvg("1.23 0.98 0.76 2/345 12345"); got != 1.23 {
		t.Errorf("parseLoadAvg = %v, want 1.23", got)
	}
	if got := parseLoadAvg(""); got != 0 {
		t.Errorf("parseLoadAvg(\"\") = %v, want 0", got)
	}
	if got := parseLoadAvg("abc def"); got != 0 {
		t.Errorf("parseLoadAvg 非法输入 = %v, want 0", got)
	}
}

// ─── parseMemLinux ────────────────────────────────────────────────

// TestParseMemLinux 验证 /proc/meminfo 解析：MemTotal 恒取，MemAvailable
// 缺失时回退 MemFree（KB→bytes ×1024）。
func TestParseMemLinux(t *testing.T) {
	out := "MemTotal:       16000000 kB\nMemFree:         1000000 kB\nMemAvailable:    2000000 kB\n"
	total, avail := parseMemLinux(out)
	if total != 16000000*1024 || avail != 2000000*1024 {
		t.Errorf("parseMemLinux = (%d, %d), want (%d, %d)", total, avail, 16000000*1024, 2000000*1024)
	}

	// 无 MemAvailable 时回退 MemFree
	outNoAvail := "MemTotal:       16000000 kB\nMemFree:         1000000 kB\n"
	total2, avail2 := parseMemLinux(outNoAvail)
	if total2 != 16000000*1024 || avail2 != 1000000*1024 {
		t.Errorf("MemAvailable 缺失应回退 MemFree: got (%d, %d), want (%d, %d)",
			total2, avail2, 16000000*1024, 1000000*1024)
	}
}
