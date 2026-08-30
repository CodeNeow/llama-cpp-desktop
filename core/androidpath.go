package core

// Shared Android storage helpers. The production anchors come from the JNI
// bridge (androidpath_android.go, Context.getFilesDir); the pure path parsing
// here is deliberately build-tag free so a single desktop test binary can
// exercise it (see androidpath_test.go).

import (
	"path/filepath"
	"strings"
)

const (
	// androidHostPackage is the Java package of the embedding Android host.
	// build/android keeps the Wails v3 template package because the JNI entry
	// symbols (Java_com_wails_app_*) are hardcoded to it.
	androidHostPackage = "com.wails.app"

	// androidFilesDirFallback is the static app-private files dir used when
	// the JNI bridge is unreachable. It matches the host package above and is
	// always writable by the app process itself.
	androidFilesDirFallback = "/data/data/" + androidHostPackage + "/files"
)

// androidExternalBase derives the app-specific external-storage base dir
// (/storage/emulated/<n>/Android/data/<pkg>/files) from the app-private
// internal files dir (/data/data/<pkg>/files or /data/user/<n>/<pkg>/files).
// The v3 JNI bridge exposes only getFilesDir, so the external dir — preferred
// for multi-GB model storage — is derived by path symmetry instead. ok is
// false when the internal path does not match either known layout.
func androidExternalBase(filesDir string) (string, bool) {
	parts := strings.Split(filepath.ToSlash(filepath.Clean(filesDir)), "/")
	if len(parts) < 4 || parts[len(parts)-1] != "files" {
		return "", false
	}
	pkg := parts[len(parts)-2]
	if pkg == "" || pkg == "data" || pkg == "user" || pkg == "0" {
		return "", false
	}
	user := "0"
	if len(parts) >= 5 && parts[len(parts)-4] == "user" {
		user = parts[len(parts)-3]
	}
	return "/storage/emulated/" + user + "/Android/data/" + pkg + "/files", true
}
