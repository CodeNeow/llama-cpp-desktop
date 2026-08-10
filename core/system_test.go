package core

import "testing"

// ─── parseLinuxCPUModel ──────────────────────────────────────────

// TestParseLinuxCPUModel 验证从 /proc/cpuinfo 文本提取 "model name" 行并去除冒号后空白。
func TestParseLinuxCPUModel(t *testing.T) {
	out := "processor\t: 0\n" +
		"model name\t: Intel(R) Core(TM) i7-10700K CPU @ 3.80GHz\n" +
		"stepping\t: 5\n"
	if got := parseLinuxCPUModel(out); got != "Intel(R) Core(TM) i7-10700K CPU @ 3.80GHz" {
		t.Errorf("parseLinuxCPUModel = %q", got)
	}
	if got := parseLinuxCPUModel("no model here"); got != "" {
		t.Errorf("无 model name 行应返回空串，实际 %q", got)
	}
}

// TestCountString 验证按子串统计行数（用于 Linux 核数统计 "processor" 行）。
func TestCountString(t *testing.T) {
	out := "processor\t: 0\nprocessor\t: 1\nprocessor\t: 2\nmodel\t: x\n"
	if got := countString(out, "processor"); got != 3 {
		t.Errorf("countString = %d, want 3", got)
	}
}

// TestParseCoresDarwin 验证 darwin 核数字符串解析，非法输入回退为 0。
func TestParseCoresDarwin(t *testing.T) {
	if got := parseCoresDarwin("8"); got != 8 {
		t.Errorf("parseCoresDarwin(8) = %d", got)
	}
	if got := parseCoresDarwin("abc"); got != 0 {
		t.Errorf("parseCoresDarwin(abc) = %d, want 0", got)
	}
}

// ─── parseMemInfo / parseVMStat ──────────────────────────────────

// TestParseMemInfo 验证从 /proc/meminfo 提取指定键的 kB 数值。
func TestParseMemInfo(t *testing.T) {
	out := "MemTotal:       33554432 kB\n" +
		"MemFree:        10000000 kB\n" +
		"MemAvailable:   20000000 kB\n"
	if got := parseMemInfo(out, "MemTotal"); got != 33554432 {
		t.Errorf("MemTotal = %d, want 33554432", got)
	}
	if got := parseMemInfo(out, "MemAvailable"); got != 20000000 {
		t.Errorf("MemAvailable = %d, want 20000000", got)
	}
	if got := parseMemInfo(out, "SwapTotal"); got != 0 {
		t.Errorf("不存在的键应返回 0，实际 %d", got)
	}
}

// TestParseVMStat 验证 darwin vm_stat 输出中 "Pages free: N" 形式的解析。
func TestParseVMStat(t *testing.T) {
	out := "Mach Virtual Memory Statistics: (page size of 16384 bytes)\n" +
		"Pages free:                            123456.\n" +
		"Pages active:                          456789.\n"
	if got := parseVMStat(out, "free"); got != 123456 {
		t.Errorf("parseVMStat(free) = %d, want 123456", got)
	}
}

// TestDefaultServerConfig 验证服务器配置默认值兜底（端口 8080、host 127.0.0.1）。
func TestDefaultServerConfig(t *testing.T) {
	cfg := defaultServerConfig()
	if cfg.Host != "127.0.0.1" || cfg.Port != 8080 || cfg.MaxModels != 1 || cfg.CacheRAM != 8192 {
		t.Errorf("默认服务器配置不符合预期: %+v", cfg)
	}
}
