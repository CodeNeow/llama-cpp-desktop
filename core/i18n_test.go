package core

import "testing"

// setLanguageForTest 直接写 currentLanguage 并恢复调用前取值，避免污染其他测试。
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

// setDetectedLanguageForTest 覆盖系统语言检测缓存（detectedLanguage），供
// auto 分支测试锁定系统检测结果。先调用一次 detectSystemLanguage 让
// detectLanguageOnce 落定（此时用真实系统 locale 写入 detectedLanguage），随后
// 立刻用测试值覆盖；此后 detectSystemLanguage 直接返回缓存（覆盖后的测试值），
// 不再被真实 locale 干扰。detectedLanguage 读写均受 languageMu 保护。
func setDetectedLanguageForTest(t *testing.T, lang string) string {
	t.Helper()
	detectSystemLanguage() // 落定 sync.Once，避免首次调用用真实 locale 覆盖测试值
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

// TestTrByLanguage 验证 tr 按当前生效语言返回对应串：zh 与非法值兜底中文、
// en 返回英文。auto 分支（依赖系统检测）单独在下方用检测缓存锁定验证。
func TestTrByLanguage(t *testing.T) {
	setLanguageForTest(t, "zh")
	if got := tr("中文", "English"); got != "中文" {
		t.Errorf("zh 应返回中文, 实际 %q", got)
	}
	setLanguageForTest(t, "en")
	if got := tr("中文", "English"); got != "English" {
		t.Errorf("en 应返回英文, 实际 %q", got)
	}
}

// TestTrAutoUsesDetectedLanguage 验证 auto 模式按系统检测缓存返回对应语言：
// 检测缓存为 zh 时 tr 返回中文，为 en 时返回英文。
func TestTrAutoUsesDetectedLanguage(t *testing.T) {
	setLanguageForTest(t, "auto")
	setDetectedLanguageForTest(t, "zh")
	if got := tr("中文", "English"); got != "中文" {
		t.Errorf("auto + 检测 zh 应返回中文, 实际 %q", got)
	}
	setDetectedLanguageForTest(t, "en")
	if got := tr("中文", "English"); got != "English" {
		t.Errorf("auto + 检测 en 应返回英文, 实际 %q", got)
	}
}

// TestEffectiveLanguage 验证 effectiveLanguage 对 zh/en 原样返回、auto 按
// 检测缓存返回、非法值兜底 zh（与 loadConfig 兜底策略一致）。
func TestEffectiveLanguage(t *testing.T) {
	setLanguageForTest(t, "zh")
	if got := effectiveLanguage(); got != "zh" {
		t.Errorf("zh 应原样返回, 实际 %q", got)
	}
	setLanguageForTest(t, "en")
	if got := effectiveLanguage(); got != "en" {
		t.Errorf("en 应原样返回, 实际 %q", got)
	}
	setLanguageForTest(t, "auto")
	setDetectedLanguageForTest(t, "zh")
	if got := effectiveLanguage(); got != "zh" {
		t.Errorf("auto + 检测 zh 应返回 zh, 实际 %q", got)
	}
	setDetectedLanguageForTest(t, "en")
	if got := effectiveLanguage(); got != "en" {
		t.Errorf("auto + 检测 en 应返回 en, 实际 %q", got)
	}
	setLanguageForTest(t, "illegal")
	if got := effectiveLanguage(); got != "zh" {
		t.Errorf("非法语言应兜底 zh, 实际 %q", got)
	}
}
