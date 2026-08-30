package core

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// ─── Per-OS app path resolution (paths.go) ───────────────────────

// withPathsSeams injects the per-OS resolution seams (runtime.GOOS selector,
// user config root, MkdirAll) and resets the once-resolved app-data base so
// the next appDataDir call re-runs resolution under the injected values.
// Everything is restored after the test, so later tests see the production
// (Windows, real os.UserConfigDir) seams again.
func withPathsSeams(t *testing.T, goos string, userConfigDir string, userConfigDirErr error, mkdirErr error) {
	t.Helper()
	oldGOOS, oldUCD, oldMkdir := pathsGOOS, pathsUserConfigDir, pathsMkdirAll
	t.Cleanup(func() {
		pathsGOOS, pathsUserConfigDir, pathsMkdirAll = oldGOOS, oldUCD, oldMkdir
		resetPathsCache()
	})
	pathsGOOS = goos
	pathsUserConfigDir = func() (string, error) {
		if userConfigDirErr != nil {
			return "", userConfigDirErr
		}
		return userConfigDir, nil
	}
	if mkdirErr != nil {
		pathsMkdirAll = func(string, os.FileMode) error { return mkdirErr }
	} else {
		pathsMkdirAll = os.MkdirAll
	}
	resetPathsCache()
}

// resetPathsCache clears the once-resolved app-data base so the next
// appDataDir call resolves again under the current seams (tests only).
func resetPathsCache() {
	pathsOnce = sync.Once{}
	pathsBase = ""
}

// TestAppDataDirWindowsKeepsCwdRelative verifies the Windows branch is a
// strict no-op: the app-data base stays empty, every state-file and default
// directory resolves to its bare cwd-relative name, the user config root is
// never consulted and no base directory is created on disk.
func TestAppDataDirWindowsKeepsCwdRelative(t *testing.T) {
	root := t.TempDir()
	withPathsSeams(t, "windows", root, nil, nil)
	if got := appDataDir(); got != "" {
		t.Errorf("appDataDir on windows = %q, want empty", got)
	}
	if got := resolveStateFile(configFileName); got != configFileName {
		t.Errorf("resolveStateFile on windows = %q, want bare %q", got, configFileName)
	}
	if got := defaultModelsDir(); got != modelsDirName {
		t.Errorf("defaultModelsDir on windows = %q, want bare %q", got, modelsDirName)
	}
	if got := defaultLlamaCppDir(); got != llamaCppDirName {
		t.Errorf("defaultLlamaCppDir on windows = %q, want bare %q", got, llamaCppDirName)
	}
	if _, err := os.Stat(filepath.Join(root, "llama-desktop")); !os.IsNotExist(err) {
		t.Errorf("windows branch must not create the app-data base, stat err = %v", err)
	}
}

// TestResolveStateFileUnderAppDataNonWindows verifies the non-Windows desktop
// branch: the base directory <UserConfigDir>/llama-desktop is created on
// first use and every name resolves beneath it, stable across calls.
func TestResolveStateFileUnderAppDataNonWindows(t *testing.T) {
	root := t.TempDir()
	withPathsSeams(t, "linux", root, nil, nil)
	wantBase := filepath.Join(root, "llama-desktop")
	if got := appDataDir(); got != wantBase {
		t.Fatalf("appDataDir = %q, want %q", got, wantBase)
	}
	if fi, err := os.Stat(wantBase); err != nil || !fi.IsDir() {
		t.Fatalf("app-data base not created on first use: stat err = %v", err)
	}
	for _, name := range []string{configFileName, handoverFileName, benchCacheFileName, docsCacheDirName, modelsDirName, llamaCppDirName} {
		if got := resolveStateFile(name); got != filepath.Join(wantBase, name) {
			t.Errorf("resolveStateFile(%q) = %q, want %q", name, got, filepath.Join(wantBase, name))
		}
	}
	if got := appDataDir(); got != wantBase {
		t.Errorf("appDataDir not stable across calls: %q, want %q", got, wantBase)
	}
}

// TestAppDataDirUserConfigDirErrorFallsBack verifies the best-effort fallback:
// when the user config root cannot be resolved, the base stays empty, names
// keep their legacy cwd-relative form and a [WARN] is logged.
func TestAppDataDirUserConfigDirErrorFallsBack(t *testing.T) {
	buf := captureLogOutput(t)
	withPathsSeams(t, "darwin", "", errors.New("no config root"), nil)
	if got := appDataDir(); got != "" {
		t.Errorf("appDataDir with failing UserConfigDir = %q, want empty", got)
	}
	if got := resolveStateFile(benchCacheFileName); got != benchCacheFileName {
		t.Errorf("resolveStateFile fallback = %q, want bare %q", got, benchCacheFileName)
	}
	if !strings.Contains(buf.String(), "[WARN]") {
		t.Errorf("fallback should log [WARN], got: %s", buf.String())
	}
}

// TestAppDataDirMkdirFailureFallsBack verifies the best-effort fallback when
// the base directory cannot be created: empty base, legacy cwd-relative names
// and a [WARN] log.
func TestAppDataDirMkdirFailureFallsBack(t *testing.T) {
	buf := captureLogOutput(t)
	withPathsSeams(t, "linux", t.TempDir(), nil, errors.New("disk full"))
	if got := appDataDir(); got != "" {
		t.Errorf("appDataDir with failing MkdirAll = %q, want empty", got)
	}
	if got := resolveStateFile(configFileName); got != configFileName {
		t.Errorf("resolveStateFile fallback = %q, want bare %q", got, configFileName)
	}
	if !strings.Contains(buf.String(), "[WARN]") {
		t.Errorf("fallback should log [WARN], got: %s", buf.String())
	}
}

// ─── Legacy cwd config migration (non-Windows only) ──────────────

// withLegacyMigrationSeams combines a temp cwd, non-Windows path seams and a
// default (unresolved) configFile so migrateLegacyConfig targets the
// app-data base. Returns the temp cwd and the log buffer.
func withLegacyMigrationSeams(t *testing.T, goos string) (string, *bytes.Buffer) {
	t.Helper()
	tmp := withTempCwd(t)
	buf := captureLogOutput(t)
	withPathsSeams(t, goos, t.TempDir(), nil, nil)
	// Resolve via the seams: undo the withTempCwd pin (its cleanup restores
	// the bare default afterwards).
	configFile = configFileName
	return tmp, buf
}

// TestMigrateCwdConfigToAppData verifies the non-Windows migration: a legacy
// cwd-relative llama-desktop-config.json is copied into the app-data base
// ([INFO], source kept), matching the design constraint that only the config
// migrates — caches and the handover record regenerate on demand.
func TestMigrateCwdConfigToAppData(t *testing.T) {
	tmp, buf := withLegacyMigrationSeams(t, "linux")
	legacy := []byte(`{"theme":"dark"}`)
	if err := os.WriteFile(filepath.Join(tmp, configFileName), legacy, 0644); err != nil {
		t.Fatal(err)
	}
	migrateLegacyConfig()
	target := filepath.Join(appDataDir(), configFileName)
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("migrated config missing at %s: %v", target, err)
	}
	if !bytes.Equal(data, legacy) {
		t.Errorf("migrated content = %q, want %q", data, legacy)
	}
	if _, err := os.Stat(filepath.Join(tmp, configFileName)); err != nil {
		t.Errorf("legacy cwd config must be kept, stat err = %v", err)
	}
	if !strings.Contains(buf.String(), "[INFO]") {
		t.Errorf("migration should log [INFO], got: %s", buf.String())
	}
}

// TestMigrateCwdConfigToAppDataSkippedWhenTargetExists verifies the
// existence short-circuit: an already-present config at the target location
// is never overwritten by the legacy copy.
func TestMigrateCwdConfigToAppDataSkippedWhenTargetExists(t *testing.T) {
	tmp, buf := withLegacyMigrationSeams(t, "linux")
	existing := []byte(`{"theme":"light"}`)
	if err := os.WriteFile(configFilePath(), existing, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, configFileName), []byte(`{"theme":"dark"}`), 0644); err != nil {
		t.Fatal(err)
	}
	migrateLegacyConfig()
	data, err := os.ReadFile(configFilePath())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, existing) {
		t.Errorf("existing target overwritten: got %q, want %q", data, existing)
	}
	if strings.Contains(buf.String(), "[INFO]") {
		t.Errorf("no migration should run when the target exists, got: %s", buf.String())
	}
}

// TestMigrateCwdConfigWindowsNoop verifies the Windows branch performs no
// cwd→app-data migration: the bare cwd-relative config is both source and
// target, so the file stays untouched and nothing is logged.
func TestMigrateCwdConfigWindowsNoop(t *testing.T) {
	tmp, buf := withLegacyMigrationSeams(t, "windows")
	legacy := []byte(`{"theme":"dark"}`)
	if err := os.WriteFile(filepath.Join(tmp, configFileName), legacy, 0644); err != nil {
		t.Fatal(err)
	}
	migrateLegacyConfig()
	data, err := os.ReadFile(filepath.Join(tmp, configFileName))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, legacy) {
		t.Errorf("windows migration must be a no-op, content changed to %q", data)
	}
	if strings.Contains(buf.String(), "Migrated legacy cwd config") {
		t.Errorf("windows must not log a cwd config migration, got: %s", buf.String())
	}
}

// TestMigrateGuiConfigToAppDataNonWindows verifies the unchanged llama-gui
// migration now targets the app-data base on non-Windows platforms: the
// cwd-relative llama-gui-config.json content lands at the resolved config
// path and the source stays in place.
func TestMigrateGuiConfigToAppDataNonWindows(t *testing.T) {
	tmp, _ := withLegacyMigrationSeams(t, "linux")
	gui := []byte(`{"theme":"dark","trayEnabled":false}`)
	if err := os.WriteFile(filepath.Join(tmp, legacyConfigFile), gui, 0644); err != nil {
		t.Fatal(err)
	}
	migrateLegacyConfig()
	data, err := os.ReadFile(configFilePath())
	if err != nil {
		t.Fatalf("gui-era config not migrated to %s: %v", configFilePath(), err)
	}
	if !bytes.Equal(data, gui) {
		t.Errorf("gui migration content = %q, want %q", data, gui)
	}
	if _, err := os.Stat(filepath.Join(tmp, legacyConfigFile)); err != nil {
		t.Errorf("gui-era source must be kept, stat err = %v", err)
	}
}
