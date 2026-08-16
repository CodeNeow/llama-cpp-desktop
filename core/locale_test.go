package core

import "testing"

// TestLocaleToLanguage verifies the localeToLanguage pure function: the "zh" prefix
// (case-insensitive, ignoring leading/trailing whitespace) maps to zh, covering Windows
// "zh-CN", Unix "zh_CN", bare "zh", and Traditional Chinese region "zh-Hant-TW";
// everything else (including empty string and English regions) maps to en.
func TestLocaleToLanguage(t *testing.T) {
	cases := []struct {
		locale string
		want   string
	}{
		{"zh-CN", "zh"},
		{"zh_CN", "zh"},
		{"zh", "zh"},
		{"zh-Hant-TW", "zh"},
		{"ZH-CN", "zh"},          // case-insensitive
		{"  zh-CN.UTF-8 ", "zh"}, // leading/trailing whitespace ignored, encoding suffix still matches prefix
		{"zh_CN.UTF-8", "zh"},
		{"en-US", "en"},
		{"en_US", "en"},
		{"en", "en"},
		{"", "en"}, // empty string (detection failure) falls back to en
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
