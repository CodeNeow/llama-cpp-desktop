package core

import (
	"strings"
	"sync"
)

// ─── 界面语言（i18n）──────────────────────────────────────────────
//
// 支持三种语言偏好：zh（中文）/ en（英文）/ auto（跟随系统，默认）。
// 后端所有面向用户的错误串/状态文案通过 tr(zh, en) 按当前生效语言返回；
// 前端通过 GetConfig 的 resolvedLanguage 同步刷新界面文案。

// languageMu 保护全局 currentLanguage 的读写，遵循项目 configMu /
// downloadSourceMu 等「显式互斥」惯例。
var languageMu sync.RWMutex

// currentLanguage 为语言偏好取值：zh / en / auto，默认 auto（跟随系统）。
var currentLanguage = "auto"

// effectiveLanguage 返回当前生效语言（zh/en）：auto 时按系统语言检测结果
// 返回（detectSystemLanguage 结果由 sync.Once 缓存）；zh/en 原样返回；
// 白名单之外的非法值兜底 zh（与 loadConfig 的兜底策略一致）。
func effectiveLanguage() string {
	languageMu.RLock()
	lang := currentLanguage
	languageMu.RUnlock()
	switch lang {
	case "zh", "en":
		return lang
	case "auto":
		return detectSystemLanguage()
	default:
		return "zh"
	}
}

// tr 按当前生效语言返回文案：en 返回英文，其余（zh / 非法值兜底）返回中文。
// 供后端所有面向用户的错误串/状态文案包裹使用；格式化参数保持不变，即
// fmt.Errorf(tr("中文 %s", "english %s"), arg) 的 arg 原样传入。
func tr(zh, en string) string {
	if effectiveLanguage() == "en" {
		return en
	}
	return zh
}

// localeToLanguage 把区域设置字符串解析为语言（zh/en）：去掉首尾空白并小写
// 后，前缀为 "zh" 则返回 "zh"，否则返回 "en"。前缀匹配天然覆盖 Windows 的
// "zh-CN"、Unix 的 "zh_CN"、以及裸 "zh" 三种格式（含繁体中文区域如
// "zh-Hant-TW"）。纯函数，供 locale_windows.go（GetUserDefaultLocaleName）与
// locale_other.go（LC_ALL/LC_MESSAGES/LANG）两侧的语言检测共用，可直接单测。
func localeToLanguage(s string) string {
	lower := strings.ToLower(strings.TrimSpace(s))
	if strings.HasPrefix(lower, "zh") {
		return "zh"
	}
	return "en"
}

// detectLanguageOnce 保证系统语言检测只执行一次；detectedLanguage 缓存检测
// 结果。二者放在本文件（与 platform 文件分离）以避免平台文件各自重复定义：
// 平台文件只需实现 readSystemLocale() string（windows 读 GetUserDefaultLocaleName、
// 其他平台依次读 LC_ALL/LC_MESSAGES/LANG），detectSystemLanguage 在此统一驱动。
var detectLanguageOnce sync.Once

// detectedLanguage 为系统语言检测缓存（zh/en），仅由 detectSystemLanguage
// 写入；测试可通过直接覆盖它锁定 auto 分支的检测结果（Once 已落定后不再重跑）。
var detectedLanguage = "en"

// detectSystemLanguage 检测系统语言（zh/en）：委托平台文件 readSystemLocale
// 读取系统区域设置，经 localeToLanguage 解析；解析结果与调用失败（返回空串）
// 均兜底 en。结果由 sync.Once 缓存，整个进程生命周期只检测一次。
func detectSystemLanguage() string {
	detectLanguageOnce.Do(func() {
		detectedLanguage = localeToLanguage(readSystemLocale())
	})
	return detectedLanguage
}
