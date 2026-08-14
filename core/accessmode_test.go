package core

import "testing"

// TestEffectiveHost 验证 effectiveHost 纯函数：lan → 0.0.0.0，其余取值
// （含空串与非法值）一律 → 127.0.0.1。SaveServerConfig / loadConfig /
// buildServerCommand 三处共用该派生，保证 Host 口径一致。
func TestEffectiveHost(t *testing.T) {
	cases := []struct {
		mode string
		want string
	}{
		{accessLocal, "127.0.0.1"},
		{accessLAN, "0.0.0.0"},
		{"", "127.0.0.1"},       // 空串兜底本机
		{"local ", "127.0.0.1"}, // 带空白非法值兜底本机
		{"wan", "127.0.0.1"},    // 白名单外非法值兜底本机
		{"0.0.0.0", "127.0.0.1"},
	}
	for _, c := range cases {
		if got := effectiveHost(c.mode); got != c.want {
			t.Errorf("effectiveHost(%q) = %q, want %q", c.mode, got, c.want)
		}
	}
}
