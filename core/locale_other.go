//go:build !windows

package core

import "os"

// readSystemLocale 读取非 Windows 平台的系统区域设置：依次取 LC_ALL /
// LC_MESSAGES / LANG 环境变量中的第一个非空值（如 "zh_CN.UTF-8" /
// "en_US.UTF-8"），经 localeToLanguage 前缀匹配解析；全部为空返回空串，
// 由 detectSystemLanguage 兜底 en。
func readSystemLocale() string {
	for _, key := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if v := os.Getenv(key); v != "" {
			return v
		}
	}
	return ""
}
