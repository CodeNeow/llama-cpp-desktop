package core

import "testing"

// ─── computeSpeed ─────────────────────────────────────────────────

// TestComputeSpeed 验证下载速度纯函数 computeSpeed：
// 正常间隔返回 bytes/s；elapsed 非正或 delta 非正时返回 0（无法计算或没有
// 有效进度，速度不显示为负值/Inf）。
func TestComputeSpeed(t *testing.T) {
	if got := computeSpeed(2.0, 100); got != 50 {
		t.Errorf("computeSpeed(2, 100) = %v, want 50", got)
	}
	// 零间隔：无法计算 → 0
	if got := computeSpeed(0, 100); got != 0 {
		t.Errorf("computeSpeed(0, 100) = %v, want 0", got)
	}
	// 负间隔（时钟回拨防御）
	if got := computeSpeed(-1, 100); got != 0 {
		t.Errorf("computeSpeed(-1, 100) = %v, want 0", got)
	}
	// 负 delta（暂停恢复后 downloaded 回退等场景）→ 0
	if got := computeSpeed(1.0, -100); got != 0 {
		t.Errorf("computeSpeed(1, -100) = %v, want 0", got)
	}
	// delta 为 0：没有进度 → 0
	if got := computeSpeed(1.0, 0); got != 0 {
		t.Errorf("computeSpeed(1, 0) = %v, want 0", got)
	}
}
