package core

import "testing"

// ─── parseAndroidCPUModel ────────────────────────────────────────

// TestParseAndroidCPUModel verifies the SoC identity fallback for /proc/cpuinfo
// text without an x86 "model name" line: the "Hardware" line wins, then the
// ro.soc.* build.prop properties, then "" (the caller keeps its generic
// "(unknown model)" fallback).
func TestParseAndroidCPUModel(t *testing.T) {
	// Snapdragon-style: Android kernels append a "Hardware" line to cpuinfo.
	cpuinfo := "Processor\t: AArch64 Processor rev 1 (aarch64)\n" +
		"processor\t: 0\n" +
		"BogoMIPS\t: 38.40\n" +
		"Hardware\t: Qualcomm Technologies, Inc SM8550\n"
	if got := parseAndroidCPUModel(cpuinfo, ""); got != "Qualcomm Technologies, Inc SM8550" {
		t.Errorf("Hardware line: parseAndroidCPUModel = %q", got)
	}

	// Dimensity-style: no Hardware line; build.prop carries the SoC identity.
	buildProp := "# begin build properties\n" +
		"ro.product.model=phone\n" +
		"ro.soc.manufacturer=Mediatek\n" +
		"ro.soc.model=MT6985\n"
	if got := parseAndroidCPUModel("processor\t: 0\n", buildProp); got != "Mediatek MT6985" {
		t.Errorf("build.prop: parseAndroidCPUModel = %q, want %q", got, "Mediatek MT6985")
	}

	// Model-only or manufacturer-only build.prop entries are used alone.
	if got := parseAndroidCPUModel("", "ro.soc.model=SM8550\n"); got != "SM8550" {
		t.Errorf("model only: parseAndroidCPUModel = %q, want %q", got, "SM8550")
	}
	if got := parseAndroidCPUModel("", "ro.soc.manufacturer=Qualcomm\n"); got != "Qualcomm" {
		t.Errorf("manufacturer only: parseAndroidCPUModel = %q, want %q", got, "Qualcomm")
	}

	// No signal anywhere: empty string keeps the generic fallback in charge.
	if got := parseAndroidCPUModel("", ""); got != "" {
		t.Errorf("empty inputs: parseAndroidCPUModel = %q, want empty", got)
	}

	// An empty Hardware value falls through to build.prop.
	if got := parseAndroidCPUModel("Hardware\t:\n", "ro.soc.manufacturer=Qualcomm\nro.soc.model=SM8550\n"); got != "Qualcomm SM8550" {
		t.Errorf("empty Hardware value: parseAndroidCPUModel = %q, want %q", got, "Qualcomm SM8550")
	}
}

// ─── countPerformanceCPUs ────────────────────────────────────────

// TestCountPerformanceCPUs verifies the big.LITTLE performance-core count:
// CPUs within 80% of the highest max frequency count as performance cores;
// unavailable or degenerate data returns 0.
func TestCountPerformanceCPUs(t *testing.T) {
	// Snapdragon-style 4+4 split (kHz values): only the big cluster qualifies.
	bigLittle := []int{1804800, 1804800, 1804800, 1804800, 2841600, 2841600, 2841600, 2841600}
	if got := countPerformanceCPUs(bigLittle); got != 4 {
		t.Errorf("4+4 split: countPerformanceCPUs = %d, want 4", got)
	}

	// 1+3+4 topology: prime + the three big cores count, little cluster not.
	triCluster := []int{1804800, 1804800, 1804800, 1804800, 2496000, 2496000, 2496000, 2841600}
	if got := countPerformanceCPUs(triCluster); got != 4 {
		t.Errorf("1+3+4 split: countPerformanceCPUs = %d, want 4", got)
	}

	// Boundary: exactly 80% of max still counts, one step below does not.
	if got := countPerformanceCPUs([]int{2000, 1600}); got != 2 {
		t.Errorf("boundary at 80%%: countPerformanceCPUs = %d, want 2", got)
	}
	if got := countPerformanceCPUs([]int{2000, 1599}); got != 1 {
		t.Errorf("below 80%%: countPerformanceCPUs = %d, want 1", got)
	}

	// Uniform topology: every core counts (no split → no capping downstream).
	if got := countPerformanceCPUs([]int{2000000, 2000000, 2000000}); got != 3 {
		t.Errorf("uniform: countPerformanceCPUs = %d, want 3", got)
	}

	// Degenerate inputs: no data, empty entries, or no positive frequency.
	if got := countPerformanceCPUs(nil); got != 0 {
		t.Errorf("nil: countPerformanceCPUs = %d, want 0", got)
	}
	if got := countPerformanceCPUs([]int{}); got != 0 {
		t.Errorf("empty: countPerformanceCPUs = %d, want 0", got)
	}
	if got := countPerformanceCPUs([]int{0, 0, 0}); got != 0 {
		t.Errorf("all zeros: countPerformanceCPUs = %d, want 0", got)
	}
}
