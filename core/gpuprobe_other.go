//go:build !linux

package core

// Non-Linux stub for the PCI display-controller probe: Windows identifies GPUs
// via nvidia-smi (NVIDIA-only, the only vendor with a CUDA build), macOS via
// the arch-gated Metal probe, and Android is CPU-only with no PCI bus access
// from the app sandbox — nowhere else is a PCI walk meaningful.

// probePciGpus returns nil outside Linux: no PCI GPU entries are ever
// appended to the system GPU list.
func probePciGpus() []PciGpu { return nil }
