package core

import "testing"

// setLanguageForTest directly writes currentLanguage and restores the pre-call value,
// preventing cross-test pollution.
func setLanguageForTest(t *testing.T, lang string) string {
	t.Helper()
	languageMu.Lock()
	prev := currentLanguage
	currentLanguage = lang
	languageMu.Unlock()
	t.Cleanup(func() {
		languageMu.Lock()
		currentLanguage = prev
		languageMu.Unlock()
	})
	return prev
}

// setDetectedLanguageForTest overrides the system language-detection cache (detectedLanguage),
// used by auto-branch tests to lock the system-detection result.
// First calls detectSystemLanguage once to let detectLanguageOnce settle (at this point the
// real system locale is written into detectedLanguage), then immediately overwrites with the
// test value; thereafter detectSystemLanguage returns the cache directly (the overwritten
// test value), no longer interfered with by the real locale. detectedLanguage reads/writes
// are both protected by languageMu.
func setDetectedLanguageForTest(t *testing.T, lang string) string {
	t.Helper()
	detectSystemLanguage() // settle sync.Once, preventing the first call from overwriting the test value with the real locale
	languageMu.Lock()
	prev := detectedLanguage
	detectedLanguage = lang
	languageMu.Unlock()
	t.Cleanup(func() {
		languageMu.Lock()
		detectedLanguage = prev
		languageMu.Unlock()
	})
	return prev
}

// TestTrByLanguage verifies tr returns the corresponding string for the active language:
// zh and illegal values fall back to Chinese, en returns English.
// The auto branch (dependent on system detection) is verified separately below using the
// detection-cache lock.
func TestTrByLanguage(t *testing.T) {
	setLanguageForTest(t, "zh")
	if got := tr("中文", "English"); got != "中文" {
		t.Errorf("zh should return Chinese, got %q", got)
	}
	setLanguageForTest(t, "en")
	if got := tr("中文", "English"); got != "English" {
		t.Errorf("en should return English, got %q", got)
	}
}

// TestTrAutoUsesDetectedLanguage verifies auto mode returns the corresponding language
// according to the system-detection cache: when the cache is zh, tr returns Chinese;
// when en, tr returns English.
func TestTrAutoUsesDetectedLanguage(t *testing.T) {
	setLanguageForTest(t, "auto")
	setDetectedLanguageForTest(t, "zh")
	if got := tr("中文", "English"); got != "中文" {
		t.Errorf("auto + detected zh should return Chinese, got %q", got)
	}
	setDetectedLanguageForTest(t, "en")
	if got := tr("中文", "English"); got != "English" {
		t.Errorf("auto + detected en should return English, got %q", got)
	}
}

// TestEffectiveLanguage verifies effectiveLanguage returns zh/en as-is, auto follows the
// detection cache, and illegal values fall back to zh (consistent with loadConfig
// fallback strategy).
func TestEffectiveLanguage(t *testing.T) {
	setLanguageForTest(t, "zh")
	if got := effectiveLanguage(); got != "zh" {
		t.Errorf("zh should be returned as-is, got %q", got)
	}
	setLanguageForTest(t, "en")
	if got := effectiveLanguage(); got != "en" {
		t.Errorf("en should be returned as-is, got %q", got)
	}
	setLanguageForTest(t, "auto")
	setDetectedLanguageForTest(t, "zh")
	if got := effectiveLanguage(); got != "zh" {
		t.Errorf("auto + detected zh should return zh, got %q", got)
	}
	setDetectedLanguageForTest(t, "en")
	if got := effectiveLanguage(); got != "en" {
		t.Errorf("auto + detected en should return en, got %q", got)
	}
	setLanguageForTest(t, "illegal")
	if got := effectiveLanguage(); got != "zh" {
		t.Errorf("illegal language should fall back to zh, got %q", got)
	}
}
