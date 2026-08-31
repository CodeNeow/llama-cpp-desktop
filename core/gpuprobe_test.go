package core

import "testing"

// TestParsePciGpus verifies the pure sysfs classification: only display
// controller classes (0x0300 VGA / 0x0302 3D / 0x0380 Display) become entries,
// vendor ids map to nvidia/intel/amd/other, names carry vendor + short PCI
// address, non-display devices (audio, USB, NVMe) are skipped, and the output
// is sorted by PCI address regardless of map iteration order.
func TestParsePciGpus(t *testing.T) {
	entries := map[string][2]string{
		// AMD dGPU: full 24-bit VGA class code, AMD/ATI vendor
		"0000:03:00.0": {"0x030000", "0x1002"},
		// NVIDIA dGPU: 3D controller class
		"0000:01:00.0": {"0x030200", "0x10de"},
		// Intel iGPU: VGA class
		"0000:00:02.0": {"0x030000", "0x8086"},
		// Display-only controller (0x0380), unknown vendor
		"0000:04:00.0": {"0x038000", "0x1234"},
		// AMD vendor id 0x1022 (ATI heritage)
		"0000:05:00.0": {"0x030000", "0x1022"},
		// Non-display devices must be skipped
		"0000:00:1f.3": {"0x040100", "0x8086"}, // audio
		"0000:02:00.0": {"0x010802", "0x144d"}, // NVMe
		"0000:00:14.0": {"0x0c0330", "0x8086"}, // USB
		// Unparsable class/vendor never crash and never classify
		"0000:06:00.0": {"garbage", ""},
	}
	got := parsePciGpus(entries)
	if len(got) != 5 {
		t.Fatalf("parsePciGpus len = %d (%+v), want 5 display controllers", len(got), got)
	}
	// Sorted by PCI address
	wantSorted := []string{"0000:00:02.0", "0000:01:00.0", "0000:03:00.0", "0000:04:00.0", "0000:05:00.0"}
	for i, addr := range wantSorted {
		if got[i].Address != addr {
			t.Errorf("got[%d].Address = %q, want %q", i, got[i].Address, addr)
		}
	}
	// Vendor classification
	wantVendor := map[string]string{
		"0000:00:02.0": "intel",
		"0000:01:00.0": "nvidia",
		"0000:03:00.0": "amd",
		"0000:04:00.0": "other",
		"0000:05:00.0": "amd",
	}
	for _, g := range got {
		if wantVendor[g.Address] != g.Vendor {
			t.Errorf("vendor[%s] = %q, want %q", g.Address, g.Vendor, wantVendor[g.Address])
		}
	}
	// Names carry the vendor display name and the domain-trimmed address
	// (address 0000:03:00.0 is the AMD entry in the sorted list)
	for _, g := range got {
		if g.Address == "0000:03:00.0" && g.Name != "AMD GPU (03:00.0)" {
			t.Errorf("name = %q, want %q", g.Name, "AMD GPU (03:00.0)")
		}
	}
}

// TestParsePciGpusEmpty verifies empty and nil inputs yield an empty (non-nil
// handling is the caller's concern) result rather than a panic.
func TestParsePciGpusEmpty(t *testing.T) {
	if got := parsePciGpus(nil); len(got) != 0 {
		t.Errorf("parsePciGpus(nil) = %+v, want empty", got)
	}
	if got := parsePciGpus(map[string][2]string{}); len(got) != 0 {
		t.Errorf("parsePciGpus(empty) = %+v, want empty", got)
	}
}

// TestIsPciDisplayClassAndVendorLabel pins the attribute parsing rules:
// case-insensitive 0x header, whitespace tolerance, the three display classes,
// and vendor fallbacks.
func TestIsPciDisplayClassAndVendorLabel(t *testing.T) {
	for _, c := range []string{"0x030000", "0X030200", "0x038000", " 0x030000\n"} {
		if !isPciDisplayClass(c) {
			t.Errorf("isPciDisplayClass(%q) = false, want true", c)
		}
	}
	for _, c := range []string{"0x040300", "0x010802", "0x0c0330", "", "zz", "0x"} {
		if isPciDisplayClass(c) {
			t.Errorf("isPciDisplayClass(%q) = true, want false", c)
		}
	}
	cases := []struct {
		raw, want string
	}{
		{"0x10de", "nvidia"},
		{"0x8086", "intel"},
		{"0x1002", "amd"},
		{"0x1022", "amd"},
		{"0x1234", "other"},
		{"", "other"},
	}
	for _, c := range cases {
		if got := pciVendorLabel(c.raw); got != c.want {
			t.Errorf("pciVendorLabel(%q) = %q, want %q", c.raw, got, c.want)
		}
	}
}
