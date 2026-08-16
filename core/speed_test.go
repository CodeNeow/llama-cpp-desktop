package core

import "testing"

// ─── computeSpeed ─────────────────────────────────────────────────

// TestComputeSpeed verifies the computeSpeed pure function for download speed:
// returns bytes/s for a normal interval; returns 0 when elapsed is non-positive or delta
// is non-positive (cannot compute or no valid progress, speed must not be negative/Inf).
func TestComputeSpeed(t *testing.T) {
	if got := computeSpeed(2.0, 100); got != 50 {
		t.Errorf("computeSpeed(2, 100) = %v, want 50", got)
	}
	// zero interval: cannot compute → 0
	if got := computeSpeed(0, 100); got != 0 {
		t.Errorf("computeSpeed(0, 100) = %v, want 0", got)
	}
	// negative interval (clock-skew defense)
	if got := computeSpeed(-1, 100); got != 0 {
		t.Errorf("computeSpeed(-1, 100) = %v, want 0", got)
	}
	// negative delta (pause-resume downloaded-back scenarios) → 0
	if got := computeSpeed(1.0, -100); got != 0 {
		t.Errorf("computeSpeed(1, -100) = %v, want 0", got)
	}
	// delta is 0: no progress → 0
	if got := computeSpeed(1.0, 0); got != 0 {
		t.Errorf("computeSpeed(1, 0) = %v, want 0", got)
	}
}
