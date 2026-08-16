//go:build !windows

package core

import "os"

// readSystemLocale reads the system locale on non-Windows platforms: returns
// the first non-empty value among LC_ALL / LC_MESSAGES / LANG (e.g.
// "zh_CN.UTF-8" / "en_US.UTF-8"), parsed by localeToLanguage via prefix
// match. Returns empty string when all are unset; detectSystemLanguage falls
// back to "en".
func readSystemLocale() string {
	for _, key := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if v := os.Getenv(key); v != "" {
			return v
		}
	}
	return ""
}
