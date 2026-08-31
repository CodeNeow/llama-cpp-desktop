//go:build linux

package core

// Linux PCI display-controller probe: walks /sys/bus/pci/devices/*/ and reads
// the `class` and `vendor` attribute files of every device, then hands the raw
// contents to the pure classifier in gpuprobe.go. This is what lets the GPU
// list (and through it the vulkan asset selection and the auto-tuner) see
// AMD / Intel / other non-NVIDIA cards on Linux — the nvidia-smi probe alone
// reports nothing there.

import (
	"os"
	"path/filepath"
	"strings"
)

// pciDevicesDir is the sysfs PCI device directory; a var so tests can point
// the probe at a fixture tree.
var pciDevicesDir = "/sys/bus/pci/devices"

// probePciGpus returns the display controllers on the PCI bus; a nil result
// means "none found / sysfs unavailable" (never an error — the GPU list
// degrades to the nvidia-smi entries only).
func probePciGpus() []PciGpu {
	entries, err := os.ReadDir(pciDevicesDir)
	if err != nil {
		return nil
	}
	facts := make(map[string][2]string, len(entries))
	for _, e := range entries {
		base := filepath.Join(pciDevicesDir, e.Name())
		facts[e.Name()] = [2]string{
			readSysAttribute(filepath.Join(base, "class")),
			readSysAttribute(filepath.Join(base, "vendor")),
		}
	}
	return parsePciGpus(facts)
}

// readSysAttribute reads one sysfs attribute file, returning the trimmed
// contents ("" when unreadable — probing must never fail hard).
func readSysAttribute(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}
