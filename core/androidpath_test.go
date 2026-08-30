package core

import (
	"path/filepath"
	"testing"
)

// ─── Android external-storage derivation (androidpath.go) ─────────

// TestAndroidExternalBaseFromInternalLayouts verifies the path-symmetry
// derivation of the app-specific external-storage dir from the internal
// files dir for both known Android layouts, including secondary-user user
// numbers.
func TestAndroidExternalBaseFromInternalLayouts(t *testing.T) {
	cases := []struct {
		filesDir string
		want     string
		ok       bool
	}{
		{"/data/data/com.wails.app/files", "/storage/emulated/0/Android/data/com.wails.app/files", true},
		{"/data/user/0/com.wails.app/files", "/storage/emulated/0/Android/data/com.wails.app/files", true},
		{"/data/user/10/com.example.app/files", "/storage/emulated/10/Android/data/com.example.app/files", true},
	}
	for _, tc := range cases {
		got, ok := androidExternalBase(tc.filesDir)
		if ok != tc.ok || got != tc.want {
			t.Errorf("androidExternalBase(%q) = (%q, %v), want (%q, %v)", tc.filesDir, got, ok, tc.want, tc.ok)
		}
	}
}

// TestAndroidExternalBaseUnrecognized verifies the guard rails: paths that do
// not end in a "files" segment, or whose parent segment cannot be a package
// name, report ok=false so callers keep model storage on the internal dir.
func TestAndroidExternalBaseUnrecognized(t *testing.T) {
	for _, filesDir := range []string{
		"",
		"/data",
		"/tmp",
		"/some/random/place",
		"/data/data/files",        // parent segment is the "data" mount root
		"/data/user/0/user/files", // parent segment looks like a role word
	} {
		if got, ok := androidExternalBase(filesDir); ok {
			t.Errorf("androidExternalBase(%q) = (%q, true), want not ok", filesDir, got)
		}
	}
}

// TestAndroidHostPackageFallbackConsistency verifies the static fallback
// parses into the same external base the JNI-bridge path would produce, so a
// bridge outage cannot scatter storage across two trees.
func TestAndroidHostPackageFallbackConsistency(t *testing.T) {
	ext, ok := androidExternalBase(androidFilesDirFallback)
	if !ok {
		t.Fatalf("static fallback %q must parse", androidFilesDirFallback)
	}
	want := filepath.ToSlash(filepath.Join("/storage/emulated/0/Android/data", androidHostPackage, "files"))
	if ext != want {
		t.Errorf("fallback external base = %q, want %q", ext, want)
	}
}
