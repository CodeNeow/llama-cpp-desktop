package core

import (
	"strings"
	"sync"
)

// ─── UI Language (i18n) ────────────────────────────────────────────
//
// Supports three language preferences: zh (Chinese) / en (English) / auto
// (follow system, default). All user-facing error/status strings from the
// backend are returned via tr(zh, en) according to the active language; the
// frontend syncs UI text through resolvedLanguage in GetConfig.

// languageMu guards reads/writes of the global currentLanguage, following the
// project's "explicit mutex" convention (configMu / downloadSourceMu / etc.).
var languageMu sync.RWMutex

// currentLanguage holds the language preference: zh / en / auto, default auto
// (follow system).
var currentLanguage = "auto"

// effectiveLanguage returns the active language (zh/en): for "auto" it returns
// the system locale detection result (cached by sync.Once in
// detectSystemLanguage); zh/en are returned as-is. Values outside the
// allowlist fall back to "zh" (matching loadConfig's fallback strategy).
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

// tr returns text according to the active language: en returns the English
// string, everything else (zh / out-of-range fallback) returns Chinese. Used
// to wrap all user-facing error/status strings in the backend; formatting
// arguments are passed through unchanged, i.e. fmt.Errorf(tr("中文 %s",
// "english %s"), arg).
func tr(zh, en string) string {
	if effectiveLanguage() == "en" {
		return en
	}
	return zh
}

// localeToLanguage parses a locale string into a language (zh/en): after
// trimming whitespace and lowercasing, a "zh" prefix yields "zh", otherwise
// "en". Prefix matching covers Windows "zh-CN", Unix "zh_CN", bare "zh", and
// traditional locales like "zh-Hant-TW". Pure function shared by
// locale_windows.go (GetUserDefaultLocaleName) and locale_other.go
// (LC_ALL/LC_MESSAGES/LANG) for language detection; directly unit-testable.
func localeToLanguage(s string) string {
	lower := strings.ToLower(strings.TrimSpace(s))
	if strings.HasPrefix(lower, "zh") {
		return "zh"
	}
	return "en"
}

// detectLanguageOnce ensures system language detection runs only once;
// detectedLanguage caches the result. Both live in this file (separated from
// platform files) so platform files don't duplicate them: platform files only
// need to implement readSystemLocale() string (Windows reads
// GetUserDefaultLocaleName, other platforms read LC_ALL/LC_MESSAGES/LANG in
// order), and detectSystemLanguage drives the process from here.
var detectLanguageOnce sync.Once

// detectedLanguage caches the detected system language (zh/en), written only
// by detectSystemLanguage; tests can overwrite it to pin the auto branch
// result (no re-run after Once has fired).
var detectedLanguage = "en"

// detectSystemLanguage detects the system language (zh/en): delegates to the
// platform-specific readSystemLocale to read the system locale, parses it via
// localeToLanguage; both parse failure and empty result fall back to "en".
// Result is cached by sync.Once for the lifetime of the process.
func detectSystemLanguage() string {
	detectLanguageOnce.Do(func() {
		detectedLanguage = localeToLanguage(readSystemLocale())
	})
	return detectedLanguage
}
