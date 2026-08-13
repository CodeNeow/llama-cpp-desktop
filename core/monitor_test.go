package core

import (
	"reflect"
	"testing"
)

// ─── parsePromptTPS ────────────────────────────────────────────────

// TestParsePromptTPS 验证从服务日志提取提示词预填充（prefill）速度：只采用含
// "prompt eval time" 的预填充行数值，解码行（eval time）不进入该指标；多行多值
// 取最后一条预填充行（最新采样为准）；无预填充行返回 0。实测样例：
// `prompt eval time = 357.49 ms / 27 tokens ( 75.53 tokens per second)`。
func TestParsePromptTPS(t *testing.T) {
	cases := []struct {
		name string
		logs []string
		want float64
	}{
		{"no match", []string{"loading model", "another line"}, 0},
		{"single prefill",
			[]string{"I slot print_timing:      prompt eval time =     357.49 ms /    27 tokens ( 75.53 tokens per second)"},
			75.53},
		// 预填充+解码共存：解码行数值（72.97）不得进入预填充指标，仍取预填充值 75.53。
		{"prefill with decode coexist",
			[]string{
				"I slot print_timing:      prompt eval time =     357.49 ms /    27 tokens ( 75.53 tokens per second)",
				"I slot print_timing:             eval time =   12334.07 ms /   900 tokens ( 72.97 tokens per second)",
			}, 75.53},
		// 多轮生成多对 timing：取最后一条预填充行（长 prompt 高值 2362.80）。
		{"multiple rounds take last prefill",
			[]string{
				"I slot print_timing:      prompt eval time =     357.49 ms /    27 tokens ( 75.53 tokens per second)",
				"llama_server: unrelated log noise",
				"I slot print_timing:      prompt eval time =    4230.00 ms / 10000 tokens ( 2362.80 tokens per second)",
			}, 2362.80},
		// 只有解码行、无预填充行：预填充指标无值，返回 0。
		{"decode only",
			[]string{
				"I slot print_timing:             eval time =   12334.07 ms /   900 tokens ( 72.97 tokens per second)",
			}, 0},
		{"empty", nil, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := parsePromptTPS(c.logs); got != c.want {
				t.Errorf("parsePromptTPS(%v) = %v, want %v", c.logs, got, c.want)
			}
		})
	}
}

// TestParsePromptTPSMultiLineEntry 验证一个条目内含多行（一次 stderr Write 含
// 预填充+解码两行）时正确切行：解码行不进入预填充指标，返回预填充值 75.53。
func TestParsePromptTPSMultiLineEntry(t *testing.T) {
	got := parsePromptTPS([]string{
		"I slot print_timing:      prompt eval time =     357.49 ms /    27 tokens ( 75.53 tokens per second)\n" +
			"I slot print_timing:             eval time =   12334.07 ms /   900 tokens ( 72.97 tokens per second)",
	})
	if got != 75.53 {
		t.Errorf("parsePromptTPS 多行条目 = %v, want 75.53", got)
	}
}

// ─── parseDecodeTPS ────────────────────────────────────────────────

// TestParseDecodeTPS 验证从服务日志提取生成（解码）实时速度：实时行
// `n_decoded = 414, tg = 68.82 t/s, tg_3s = 67.32 t/s` 只取 tg_3s（3 秒窗口实时
// 值 67.32）而非 tg（累计均值 68.82）；tg_3s 优先于 eval time 兜底值（实时值
// 优先，即使 eval time 行在其后）；无 tg 行时回退 eval time 行数值；预填充行
// （prompt eval time 含 eval time 子串）须先排除、不进入候选；无候选返回 0。
// 实测样例：解码总计时 `eval time = 12334.07 ms / 900 tokens ( 72.97 tokens per
// second)`。
func TestParseDecodeTPS(t *testing.T) {
	cases := []struct {
		name string
		logs []string
		want float64
	}{
		{"no match", []string{"loading model", "another line"}, 0},
		{"empty", nil, 0},
		// 实时行：tg 与 tg_3s 并存，只取 tg_3s（3 秒窗口实时值），不得取 tg（累计均值）。
		{"real-time line takes tg_3s",
			[]string{"I slot print_timing:              n_decoded = 414, tg = 68.82 t/s, tg_3s = 67.32 t/s"},
			67.32},
		// 多行实时行：取最后一条 tg_3s（最新采样为准）。
		{"multiple real-time lines take last tg_3s",
			[]string{
				"I slot print_timing:              n_decoded = 100, tg = 60.00 t/s, tg_3s = 61.50 t/s",
				"llama_server: unrelated log noise",
				"I slot print_timing:              n_decoded = 414, tg = 68.82 t/s, tg_3s = 70.50 t/s",
			}, 70.50},
		// 无实时行（旧版二进制）：回退解码总计时行数值。
		{"fallback eval time",
			[]string{"I slot print_timing:             eval time =   12334.07 ms /   900 tokens ( 72.97 tokens per second)"},
			72.97},
		// 实时行在前、eval time 行在后：仍返回实时 tg_3s（实时值优先于请求结束时
		// 的总计时），断言依据：生成期间监控取 3 秒窗口速度。
		{"realtime line before eval line, realtime wins",
			[]string{
				"I slot print_timing:              n_decoded = 414, tg = 68.82 t/s, tg_3s = 67.32 t/s",
				"I slot print_timing:             eval time =   12334.07 ms /   900 tokens ( 72.97 tokens per second)",
			}, 67.32},
		// 预填充行不进入候选：prompt eval time 是 eval time 子串，须先排除；
		// 预填充+解码共存时取解码值 72.97 而非预填充值 75.53。
		{"prefill excluded, decode value used",
			[]string{
				"I slot print_timing:      prompt eval time =     357.49 ms /    27 tokens ( 75.53 tokens per second)",
				"I slot print_timing:             eval time =   12334.07 ms /   900 tokens ( 72.97 tokens per second)",
			}, 72.97},
		// 只有预填充行、无解码/实时行：解码指标无候选，返回 0。
		{"prefill only",
			[]string{
				"I slot print_timing:      prompt eval time =     357.49 ms /    27 tokens ( 75.53 tokens per second)",
			}, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := parseDecodeTPS(c.logs); got != c.want {
				t.Errorf("parseDecodeTPS(%v) = %v, want %v", c.logs, got, c.want)
			}
		})
	}
}

// TestParseDecodeTPSMultiLineEntry 验证一个条目内含多行（实时行+解码总计时行）
// 时正确切行：取实时 tg_3s 67.32。
func TestParseDecodeTPSMultiLineEntry(t *testing.T) {
	got := parseDecodeTPS([]string{
		"I slot print_timing:              n_decoded = 414, tg = 68.82 t/s, tg_3s = 67.32 t/s\n" +
			"I slot print_timing:             eval time =   12334.07 ms /   900 tokens ( 72.97 tokens per second)",
	})
	if got != 67.32 {
		t.Errorf("parseDecodeTPS 多行条目 = %v, want 67.32", got)
	}
}

// ─── splitLogLines ─────────────────────────────────────────────────

// TestSplitLogLines 验证共享切行 helper：单条目内嵌多行被拆成独立行、多条目
// 合并后顺序保持、空列表经 Join("")→Split("") 得到单条空行（各解析函数对空行
// 无关键词命中，结果仍为 0）。
func TestSplitLogLines(t *testing.T) {
	if got := splitLogLines([]string{"line1\nline2"}); !reflect.DeepEqual(got, []string{"line1", "line2"}) {
		t.Errorf("splitLogLines 多行条目 = %v, want [line1 line2]", got)
	}
	if got := splitLogLines([]string{"a\nb", "c"}); !reflect.DeepEqual(got, []string{"a", "b", "c"}) {
		t.Errorf("splitLogLines 多条目 = %v, want [a b c]", got)
	}
	if got := splitLogLines([]string{"single"}); !reflect.DeepEqual(got, []string{"single"}) {
		t.Errorf("splitLogLines 单行条目 = %v, want [single]", got)
	}
	if got := splitLogLines(nil); !reflect.DeepEqual(got, []string{""}) {
		t.Errorf("splitLogLines(nil) = %v, want [\"\"]", got)
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
