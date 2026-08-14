package core

import "testing"

// TestLocaleToLanguage 验证 localeToLanguage 纯函数：前缀 "zh"（不区分大小写、
// 忽略首尾空白）映射为 zh，覆盖 Windows 的 "zh-CN"、Unix 的 "zh_CN"、裸 "zh"
// 与繁体中文区域 "zh-Hant-TW"；其余（含空串与英文区域）一律映射 en。
func TestLocaleToLanguage(t *testing.T) {
	cases := []struct {
		locale string
		want   string
	}{
		{"zh-CN", "zh"},
		{"zh_CN", "zh"},
		{"zh", "zh"},
		{"zh-Hant-TW", "zh"},
		{"ZH-CN", "zh"},          // 大小写不敏感
		{"  zh-CN.UTF-8 ", "zh"}, // 首尾空白忽略，带编码后缀也命中前缀
		{"zh_CN.UTF-8", "zh"},
		{"en-US", "en"},
		{"en_US", "en"},
		{"en", "en"},
		{"", "en"}, // 空串（检测失败）兜底 en
		{"fr", "en"},
		{"ja-JP", "en"},
		{"de_DE.UTF-8", "en"},
	}
	for _, c := range cases {
		if got := localeToLanguage(c.locale); got != c.want {
			t.Errorf("localeToLanguage(%q) = %q, want %q", c.locale, got, c.want)
		}
	}
}
