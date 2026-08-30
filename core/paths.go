package core

// ─── App path resolution ─────────────────────────────────────────
//
// Per-OS resolution of the app's state-file and default-directory locations.
//
// Windows keeps the historical layout unchanged: every state file (config /
// handover record / bench cache / docs cache) and every default directory
// (LLM-Models, llama-cpp) resolves to its bare cwd-relative name, exactly as
// before this file existed (in practice the process cwd is the install dir).
//
// Non-Windows desktop platforms (darwin / linux) resolve a per-user app-data
// base via os.UserConfigDir() + "/llama-desktop", created on first use:
// macOS .app bundles launch with cwd = "/" and Linux launchers may pick
// arbitrary working directories, so bare names would scatter state across
// the filesystem. Android (GOOS=android, the Wails v3 app target) resolves
// the same way when the embedding host provides $HOME; when os.UserConfigDir
// fails there (stock Android app processes set neither $HOME nor
// $XDG_CONFIG_HOME), a $HOME/llama-desktop fallback applies, and a [WARN] +
// legacy cwd-relative layout when $HOME is unavailable too (see
// androidHomeBase for the v3 host details).
//
// The per-OS branch is a runtime.GOOS switch (not build tags) so a single
// test binary exercises every branch via the injected seams below.

import (
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

// Canonical state-file and default-directory names. The state-file vars
// (configFile, handoverFile, benchCacheFile, docsCacheDir) initialize to the
// bare names and resolve through the *Path getters at use time; the default
// directories resolve through defaultModelsDir / defaultLlamaCppDir.
const (
	configFileName     = "llama-desktop-config.json"
	handoverFileName   = "llama-desktop-server-handover.json"
	benchCacheFileName = "llama-desktop-benchcache.json"
	docsCacheDirName   = "llama-desktop-docscache"
	modelsDirName      = "LLM-Models"
	llamaCppDirName    = "llama-cpp"
)

// Injection seams (same style as cmdTimeout / benchMeasureFn): tests swap
// these to drive the per-OS branches from a single test binary.
var (
	// pathsGOOS is the OS branch selector (runtime.GOOS in production).
	pathsGOOS = runtime.GOOS
	// pathsUserConfigDir resolves the per-user config root (os.UserConfigDir
	// in production).
	pathsUserConfigDir = os.UserConfigDir
	// pathsMkdirAll creates the base directory (os.MkdirAll in production).
	pathsMkdirAll = os.MkdirAll
)

// The app-data base resolves once per process; pathsBase stays empty whenever
// the platform keeps cwd-relative paths (Windows) or resolution failed
// (non-Windows fallback to the legacy cwd-relative behavior).
var (
	pathsOnce sync.Once
	pathsBase string
)

// appDataDir returns the per-OS app-data base directory, or "" when paths
// must stay cwd-relative: always on Windows; on other platforms only when
// os.UserConfigDir fails or the base directory cannot be created (best-effort
// — the failure is [WARN]-logged once and the legacy cwd-relative layout
// applies, matching pre-app-data releases).
func appDataDir() string {
	pathsOnce.Do(func() {
		if pathsGOOS == "windows" {
			return
		}
		root, err := pathsUserConfigDir()
		if err != nil {
			// Android: os.UserConfigDir fails whenever the embedding app
			// process lacks $HOME / $XDG_CONFIG_HOME — fall back to
			// $HOME/llama-desktop (created on first use) or the legacy
			// cwd-relative layout; see androidHomeBase.
			if pathsGOOS == "android" {
				pathsBase = androidHomeBase()
				return
			}
			log.Printf("[WARN] Cannot resolve user config dir, keeping cwd-relative app paths: %v", err)
			return
		}
		if root == "" {
			log.Println("[WARN] User config dir is empty, keeping cwd-relative app paths")
			return
		}
		base := filepath.Join(root, "llama-desktop")
		if err := pathsMkdirAll(base, 0755); err != nil {
			log.Printf("[WARN] Cannot create app data dir %s, keeping cwd-relative app paths: %v", base, err)
			return
		}
		pathsBase = base
	})
	return pathsBase
}

// androidHomeBase resolves the Android app-data base as $HOME/llama-desktop,
// created on first use; returns "" (with a [WARN]) when $HOME is unset or the
// directory cannot be created, keeping the legacy cwd-relative layout.
//
// Why the fallback exists: os.UserConfigDir always fails on stock Android —
// the Go code runs inside the embedding app process, which provides neither
// $HOME nor $XDG_CONFIG_HOME. The Wails v3 Android host sets no environment
// variables of its own (v3/pkg/application/application_android.go declares no
// os.Setenv; the registered-main flow it drives — RegisterAndroidMain at
// application_android.go:364, started via the JNI nativeInit export at
// application_android.go:681-695 — carries no env setup), and it exposes the
// app-private files dir (activity.getFilesDir()) only through the JNI
// mobile-features bridge: StoragePath in
// v3/pkg/application/mobile_features_android.go:197-199 ←
// getStoragePath() in
// v3/internal/commands/build_assets/android/app/src/main/java/com/wails/app/WailsBridge.java:1100-1103.
// Reaching that API from core/ would require importing the wails application
// package behind an android+cgo build-tag split, so this phase falls back to
// $HOME when the embedding host (or a future bridge wiring) provides one.
func androidHomeBase() string {
	home := os.Getenv("HOME")
	if home == "" {
		log.Println("[WARN] Android $HOME is not set, keeping cwd-relative app paths")
		return ""
	}
	base := filepath.Join(home, "llama-desktop")
	if err := pathsMkdirAll(base, 0755); err != nil {
		log.Printf("[WARN] Cannot create app data dir %s, keeping cwd-relative app paths: %v", base, err)
		return ""
	}
	return base
}

// resolveStateFile resolves a state-file (or directory) name to its active
// location: under the app-data base on non-Windows desktop platforms, or the
// bare cwd-relative name on Windows / fallback.
func resolveStateFile(name string) string {
	if base := appDataDir(); base != "" {
		return filepath.Join(base, name)
	}
	return name
}
