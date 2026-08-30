//go:build android && cgo

package core

// Android storage anchors backed by the Wails v3 JNI bridge (cgo builds —
// every real Android app build; the !cgo tooling variant is androidpath_nocgo.go).

import (
	"log"
	"os"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// androidFilesDir returns the app-private internal files dir
// (Context.getFilesDir()) through the v3 JNI bridge — the only writable
// anchor the Go half has on Android, where the process cwd is the read-only
// filesystem root and neither $HOME nor $XDG_CONFIG_HOME exist. Falls back to
// the static /data/data/<host package>/files path when the bridge is
// unreachable (both resolve to the same directory in production). Returns ""
// (with a [WARN]) when even that directory cannot be created.
func androidFilesDir() string {
	dir := application.Android.StoragePath()
	if dir == "" {
		dir = androidFilesDirFallback
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Printf("[WARN] Cannot create Android files dir %s: %v", dir, err)
		return ""
	}
	return dir
}

// androidModelsBase returns the base directory for large model storage: the
// app-specific external-storage dir when it can be derived from the internal
// files dir and created, the internal dir itself otherwise. Models go to
// external storage (same physical disk as internal on modern devices, no
// per-app quota pressure, no execute bit needed); the llama.cpp runtime stays
// internal because SELinux forbids exec on the external (FUSE) mount.
func androidModelsBase() string {
	files := androidFilesDir()
	if files == "" {
		return ""
	}
	ext, ok := androidExternalBase(files)
	if !ok {
		return files
	}
	if err := os.MkdirAll(ext, 0755); err != nil {
		log.Printf("[WARN] Cannot create Android external storage dir %s, keeping models on internal storage: %v", ext, err)
		return files
	}
	return ext
}
