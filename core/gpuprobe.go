package core

// PCI display-controller classification shared by the platform GPU probes
// (compiled on every platform; the IO lives in the build-tagged siblings
// gpuprobe_linux.go / gpuprobe_other.go). On Linux the llama.cpp release ships
// a single Vulkan tarball that accelerates NVIDIA, AMD and Intel alike, so the
// GPU probe must see non-NVIDIA cards too — the nvidia-smi-only probe cannot.
// The sysfs attribute parsing below is pure and table-testable with fixture
// maps; no os calls here.

import (
	"sort"
	"strconv"
	"strings"
)

// PciGpu is one display controller found on the PCI bus.
type PciGpu struct {
	// Address is the PCI device address as named by sysfs, e.g. "0000:01:00.0".
	Address string
	// Vendor is the classified maker: "nvidia" | "intel" | "amd" | "other".
	Vendor string
	// Name is a human-readable display label ("AMD GPU (01:00.0)"). The sysfs
	// attributes carry no marketing name, so the vendor label plus the PCI
	// address is the identity shown in the UI (nvidia cards keep their
	// nvidia-smi name and never come through this path).
	Name string
}

// PCI vendor ids (sysfs `vendor` attribute, e.g. "0x10de").
const (
	pciVendorIDNvidia = 0x10de
	pciVendorIDIntel  = 0x8086
	pciVendorIDAMD1   = 0x1002 // AMD / ATI
	pciVendorIDAMD2   = 0x1022 // ATI heritage
)

// isPciDisplayClass reports whether a sysfs `class` value marks a display
// controller: 0x0300 VGA, 0x0302 3D or 0x0380 Display. The attribute carries
// the full 24-bit class code (e.g. "0x030000"); only the first four hex
// digits (base class + subclass, value >> 8) decide. Unparsable input is
// never a display controller.
func isPciDisplayClass(class string) bool {
	v, ok := parsePciHex(class)
	if !ok {
		return false
	}
	sub := v >> 8
	return sub == 0x0300 || sub == 0x0302 || sub == 0x0380
}

// pciVendorLabel maps a sysfs `vendor` value to the vendor label used by
// GPUInfo.Vendor; unknown or unparsable vendors degrade to "other".
func pciVendorLabel(vendor string) string {
	v, ok := parsePciHex(vendor)
	if !ok {
		return "other"
	}
	switch v {
	case pciVendorIDNvidia:
		return "nvidia"
	case pciVendorIDIntel:
		return "intel"
	case pciVendorIDAMD1, pciVendorIDAMD2:
		return "amd"
	}
	return "other"
}

// parsePciHex parses a sysfs attribute hex value ("0x10de", "0x030000";
// case-insensitive "0x" header, surrounding whitespace tolerated); ok=false
// on anything else.
func parsePciHex(s string) (uint32, bool) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(strings.ToLower(s), "0x")
	if s == "" {
		return 0, false
	}
	v, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return 0, false
	}
	return uint32(v), true
}

// parsePciGpus builds the display-controller list from per-device facts keyed
// by PCI address; each fact pair is {class, vendor} raw sysfs contents. Only
// display-controller classes become entries; the list is sorted by PCI
// address so the output order is deterministic regardless of map iteration.
// Pure function — no os calls — so tests feed fixture maps directly.
func parsePciGpus(entries map[string][2]string) []PciGpu {
	gpus := make([]PciGpu, 0, len(entries))
	for addr, facts := range entries {
		class, vendor := facts[0], facts[1]
		if !isPciDisplayClass(class) {
			continue
		}
		label := pciVendorLabel(vendor)
		gpus = append(gpus, PciGpu{
			Address: addr,
			Vendor:  label,
			Name:    pciGpuDisplayName(label, addr),
		})
	}
	sort.Slice(gpus, func(i, j int) bool { return gpus[i].Address < gpus[j].Address })
	return gpus
}

// pciVendorDisplayName maps a vendor label to its UI display name.
func pciVendorDisplayName(vendor string) string {
	switch vendor {
	case "nvidia":
		return "NVIDIA"
	case "intel":
		return "Intel"
	case "amd":
		return "AMD"
	}
	return "GPU"
}

// pciGpuDisplayName builds the display label for a PCI-classified card: the
// vendor display name plus the PCI address (the only identity sysfs offers),
// e.g. "AMD GPU (01:00.0)". A leading PCI domain "0000:" is dropped to keep
// the label compact.
func pciGpuDisplayName(vendor, addr string) string {
	short := addr
	// Canonical sysfs form is dddd:bb:dd.f (12 chars with the domain prefix)
	if len(short) == 12 && short[4] == ':' {
		short = short[5:]
	}
	return pciVendorDisplayName(vendor) + " GPU (" + short + ")"
}
