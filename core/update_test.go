package core

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// saveUpdateState snapshots update-related globals (API URL, download state, executable-path
// injection) and restores them after the test, preventing cross-test pollution.
func saveUpdateState(t *testing.T) (origAPI string, origState UpdateDownloadState, origExe func() (string, error)) {
	t.Helper()
	origAPI = updateRepoAPI
	updateDownloadMu.Lock()
	origState = *updateDownloadState
	updateDownloadMu.Unlock()
	origExe = updateExePath
	t.Cleanup(func() {
		updateRepoAPI = origAPI
		updateDownloadMu.Lock()
		*updateDownloadState = origState
		updateDownloadMu.Unlock()
		updateExePath = origExe
	})
	return
}

// TestCompareVersions verifies version comparison strips the v prefix and compares
// segment-by-segment; when segment counts differ, missing segments are padded with 0
// (1.0 < 1.0.1).
func TestCompareVersions(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"v0.1.0", "v0.1.1", -1},
		{"v0.1.1", "v0.1.1", 0},
		{"v1.0.0", "v0.9.9", 1},
		{"0.1.0", "v0.1.0", 0},
		{"V0.2.0", "v0.10.0", -1},
		{"v1.0", "v1.0.1", -1},
		{"v2.0.0", "v1.9.9", 1},
		{"v0.1.0", "v0.1.0-alpha", 0}, // non-numeric segments treated as 0
	}
	for _, c := range cases {
		if got := compareVersions(c.a, c.b); got != c.want {
			t.Errorf("compareVersions(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

// TestPickUpdateAsset verifies update asset selection by install type, compatible with
// both old and new naming conventions: setup picks installers (installer / setup names),
// portable picks portable builds (portable name or old names like llama-gui.exe that are
// not installers).
func TestPickUpdateAsset(t *testing.T) {
	// setup: old installer name matches
	if got := pickUpdateAsset([]GitHubAsset{{Name: "llama-gui-amd64-installer.exe"}}, installKindSetup); got == nil || got.Name != "llama-gui-amd64-installer.exe" {
		t.Errorf("setup should pick llama-gui-amd64-installer.exe, got %v", got)
	}
	// setup: new setup name matches (name does not contain "installer" but still matches)
	if got := pickUpdateAsset([]GitHubAsset{{Name: "llama-desktop-setup-v0.2.0-amd64.exe"}}, installKindSetup); got == nil || got.Name != "llama-desktop-setup-v0.2.0-amd64.exe" {
		t.Errorf("setup should pick llama-desktop-setup-v0.2.0-amd64.exe, got %v", got)
	}
	// setup: only portable asset available → nil (must not pick portable by mistake)
	if got := pickUpdateAsset([]GitHubAsset{{Name: "llama-desktop-portable-v0.2.0-amd64.exe"}}, installKindSetup); got != nil {
		t.Errorf("setup with only portable asset should return nil, got %v", got)
	}

	// portable: old llama-gui.exe matches (skips installer)
	assets := []GitHubAsset{
		{Name: "llama-gui-amd64-installer.exe"},
		{Name: "llama-gui.exe", Size: 10516480},
	}
	if got := pickUpdateAsset(assets, installKindPortable); got == nil || got.Name != "llama-gui.exe" {
		t.Errorf("portable should pick llama-gui.exe, got %v", got)
	}
	// portable: new portable name matches
	if got := pickUpdateAsset([]GitHubAsset{{Name: "llama-desktop-portable-v0.2.0-amd64.exe"}}, installKindPortable); got == nil || got.Name != "llama-desktop-portable-v0.2.0-amd64.exe" {
		t.Errorf("portable should pick llama-desktop-portable-v0.2.0-amd64.exe, got %v", got)
	}
	// portable: only setup/installer assets → nil (must not pick installer by mistake)
	if got := pickUpdateAsset([]GitHubAsset{{Name: "llama-desktop-setup-v0.2.0-amd64.exe"}, {Name: "llama-gui-amd64-installer.exe"}}, installKindPortable); got != nil {
		t.Errorf("portable with only installer assets should return nil, got %v", got)
	}

	// empty asset list returns nil for both kinds
	if got := pickUpdateAsset(nil, installKindSetup); got != nil {
		t.Errorf("nil assets (setup) should return nil, got %v", got)
	}
	if got := pickUpdateAsset(nil, installKindPortable); got != nil {
		t.Errorf("nil assets (portable) should return nil, got %v", got)
	}
}

// TestDetectInstallKind verifies install-type detection: executable directory containing
// uninstall.exe is classified as setup (NSIS install); absence means portable; when
// updateExePath returns an error, portable is the fallback.
// updateExePath injection value is saved/restored throughout.
func TestDetectInstallKind(t *testing.T) {
	origExe := updateExePath
	t.Cleanup(func() { updateExePath = origExe })

	dir := t.TempDir()
	// no uninstall.exe → portable (green portable build)
	updateExePath = func() (string, error) {
		return filepath.Join(dir, "llama-desktop.exe"), nil
	}
	if got := detectInstallKind(); got != installKindPortable {
		t.Errorf("no uninstall.exe should detect %q, got %q", installKindPortable, got)
	}

	// has uninstall.exe → setup (NSIS install)
	if err := os.WriteFile(filepath.Join(dir, "uninstall.exe"), []byte("uninstaller"), 0644); err != nil {
		t.Fatal(err)
	}
	if got := detectInstallKind(); got != installKindSetup {
		t.Errorf("with uninstall.exe should detect %q, got %q", installKindSetup, got)
	}

	// updateExePath returns error → fallback to portable
	updateExePath = func() (string, error) {
		return "", errors.New("no executable path")
	}
	if got := detectInstallKind(); got != installKindPortable {
		t.Errorf("updateExePath error should fall back to %q, got %q", installKindPortable, got)
	}
}

// TestCheckForUpdateNewer verifies hasUpdate is true when the remote version is newer
// than the current version, carrying the version number and release notes.
func TestCheckForUpdateNewer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"tag_name":"v9.9.9","name":"Release","body":"new feature","published_at":"2026-08-10T00:00:00Z","assets":[]}`))
	}))
	defer srv.Close()

	res, err := CheckForUpdateAt(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if !res.HasUpdate {
		t.Error("v9.9.9 > current version, hasUpdate should be true")
	}
	if res.Version != "v9.9.9" || !strings.Contains(res.Notes, "new feature") {
		t.Errorf("version info wrong: %+v", res)
	}
}

// TestCheckForUpdateSame verifies hasUpdate is false when the remote version matches
// the current version.
// currentVersion comes from the core/VERSION embedded file (overridden by CI at tag release);
// the assertion only checks the format (vX.Y.Z) without binding to a specific version,
// avoiding test churn on every release.
func TestCheckForUpdateSame(t *testing.T) {
	if !regexp.MustCompile(`^v\d+\.\d+\.\d+$`).MatchString(currentVersion) {
		t.Fatalf("currentVersion should match vX.Y.Z, got %q", currentVersion)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"tag_name":"` + currentVersion + `","name":"Release","assets":[]}`))
	}))
	defer srv.Close()

	res, err := CheckForUpdateAt(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if res.HasUpdate {
		t.Error("hasUpdate should be false when versions match")
	}
}

// TestStartUpdateDownloadRejectsNotNewer verifies StartUpdateDownload returns an error
// for a version that is not newer than the current version, without starting the download.
func TestStartUpdateDownloadRejectsNotNewer(t *testing.T) {
	app := &App{}
	if err := app.StartUpdateDownload(currentVersion); err == nil {
		t.Error("same version as current should return error")
	}
}

// TestDownloadUpdateRelease verifies the portable update download end-to-end: fetch from
// injected release API → pick portable asset by type → download to the executable's
// directory → status becomes done and the file exists, with a version-and-type-prefixed
// filename (llama-desktop-portable-v<tag>.exe).
func TestDownloadUpdateRelease(t *testing.T) {
	withTempCwd(t)
	saveUpdateState(t)
	// download landing directory uses temp dir to simulate "same directory as executable" (no uninstall.exe → portable)
	exeDir := t.TempDir()
	updateExePath = func() (string, error) {
		return filepath.Join(exeDir, "llama-desktop.exe"), nil
	}

	payload := []byte("MZ fake exe payload")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/dl/llama-gui.exe" {
			w.Header().Set("Content-Length", strconv.Itoa(len(payload)))
			w.Write(payload)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		dlURL := "http://" + r.Host + "/dl/llama-gui.exe"
		w.Write([]byte(`{"tag_name":"v0.2.0","name":"Release","assets":[{"name":"llama-gui-amd64-installer.exe","size":10,"browser_download_url":"https://x/i.exe"},{"name":"llama-gui.exe","size":` + strconv.Itoa(len(payload)) + `,"browser_download_url":"` + dlURL + `"}]}`))
	}))
	defer srv.Close()
	updateRepoAPI = srv.URL

	downloadUpdateRelease("v0.2.0")

	updateDownloadMu.Lock()
	ds := *updateDownloadState
	updateDownloadMu.Unlock()

	if ds.Status != "done" {
		t.Fatalf("download completion status = %q, want done (error: %s)", ds.Status, ds.Error)
	}
	wantPath := filepath.Join(exeDir, "llama-desktop-portable-v0.2.0.exe")
	if ds.FilePath != wantPath {
		t.Errorf("save path = %q, want %q", ds.FilePath, wantPath)
	}
	if ds.Kind != installKindPortable {
		t.Errorf("download kind = %q, want %q", ds.Kind, installKindPortable)
	}
	fi, err := os.Stat(wantPath)
	if err != nil {
		t.Fatalf("target file does not exist: %v", err)
	}
	if fi.Size() != int64(len(payload)) {
		t.Errorf("file size = %d, want %d", fi.Size(), len(payload))
	}
}

// TestDownloadUpdateReleaseSetup verifies setup installer update download: when the
// executable directory contains uninstall.exe (NSIS install), the setup-type asset is
// selected, downloaded as llama-desktop-setup-v<tag>.exe, status becomes done, and kind=setup.
func TestDownloadUpdateReleaseSetup(t *testing.T) {
	withTempCwd(t)
	saveUpdateState(t)
	// download landing directory uses temp dir to simulate "same directory as executable", with uninstall.exe
	// to simulate NSIS install directory
	exeDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(exeDir, "uninstall.exe"), []byte("uninstaller"), 0644); err != nil {
		t.Fatal(err)
	}
	updateExePath = func() (string, error) {
		return filepath.Join(exeDir, "llama-desktop.exe"), nil
	}

	payload := []byte("MZ fake setup payload")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/dl/llama-desktop-setup-v0.2.0-amd64.exe" {
			w.Header().Set("Content-Length", strconv.Itoa(len(payload)))
			w.Write(payload)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		dlURL := "http://" + r.Host + "/dl/llama-desktop-setup-v0.2.0-amd64.exe"
		w.Write([]byte(`{"tag_name":"v0.2.0","name":"Release","assets":[{"name":"llama-desktop-setup-v0.2.0-amd64.exe","size":` + strconv.Itoa(len(payload)) + `,"browser_download_url":"` + dlURL + `"},{"name":"llama-desktop-portable-v0.2.0-amd64.exe","size":10,"browser_download_url":"https://x/p.exe"}]}`))
	}))
	defer srv.Close()
	updateRepoAPI = srv.URL

	downloadUpdateRelease("v0.2.0")

	updateDownloadMu.Lock()
	ds := *updateDownloadState
	updateDownloadMu.Unlock()

	if ds.Status != "done" {
		t.Fatalf("download completion status = %q, want done (error: %s)", ds.Status, ds.Error)
	}
	wantPath := filepath.Join(exeDir, "llama-desktop-setup-v0.2.0.exe")
	if ds.FilePath != wantPath {
		t.Errorf("save path = %q, want %q", ds.FilePath, wantPath)
	}
	if ds.Kind != installKindSetup {
		t.Errorf("download kind = %q, want %q", ds.Kind, installKindSetup)
	}
	fi, err := os.Stat(wantPath)
	if err != nil {
		t.Fatalf("target file does not exist: %v", err)
	}
	if fi.Size() != int64(len(payload)) {
		t.Errorf("file size = %d, want %d", fi.Size(), len(payload))
	}
}

// TestDownloadUpdateReleaseCrossDeviceFallback verifies update download completes via
// copy fallback in a cross-device scenario (renameFile is injected with EXDEV, corresponding
// to Windows system temp directory and executable on different drives):
// status becomes done, target file exists and content is correct. Also verifies the EXDEV
// branch takes priority over delete-and-retry (an existing old file at the target path is
// not deleted first and then lost on failure).
func TestDownloadUpdateReleaseCrossDeviceFallback(t *testing.T) {
	withTempCwd(t)
	saveUpdateState(t)
	// download landing directory uses temp dir to simulate "same directory as executable" (different device from source temp file)
	exeDir := t.TempDir()
	updateExePath = func() (string, error) {
		return filepath.Join(exeDir, "llama-desktop.exe"), nil
	}

	payload := []byte("MZ fake exe payload cross device")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/dl/llama-gui.exe" {
			w.Header().Set("Content-Length", strconv.Itoa(len(payload)))
			w.Write(payload)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		dlURL := "http://" + r.Host + "/dl/llama-gui.exe"
		w.Write([]byte(`{"tag_name":"v0.2.0","name":"Release","assets":[{"name":"llama-gui.exe","size":` + strconv.Itoa(len(payload)) + `,"browser_download_url":"` + dlURL + `"}]}`))
	}))
	defer srv.Close()
	updateRepoAPI = srv.URL

	// inject renameFile to simulate cross-device failure (moveFile calls this package-level variable;
	// crossDeviceRenameErr is the real cross-device error for the current platform:
	// Windows cross-drive ERROR_NOT_SAME_DEVICE=17 / Unix EXDEV, wrapped in LinkError to simulate real shape)
	origRename := renameFile
	renameFile = func(oldpath, newpath string) error {
		return &os.LinkError{Op: "rename", Old: oldpath, New: newpath, Err: crossDeviceRenameErr}
	}
	defer func() { renameFile = origRename }()

	// target path already contains an old-version file; verify cross-device fallback does not delete the old file first and cause data loss
	wantPath := filepath.Join(exeDir, "llama-desktop-portable-v0.2.0.exe")
	if err := os.WriteFile(wantPath, []byte("old version"), 0644); err != nil {
		t.Fatal(err)
	}

	downloadUpdateRelease("v0.2.0")

	updateDownloadMu.Lock()
	ds := *updateDownloadState
	updateDownloadMu.Unlock()

	if ds.Status != "done" {
		t.Fatalf("status after cross-device fallback = %q, want done (error: %s)", ds.Status, ds.Error)
	}
	if ds.FilePath != wantPath {
		t.Errorf("save path = %q, want %q", ds.FilePath, wantPath)
	}
	if ds.Kind != installKindPortable {
		t.Errorf("download kind = %q, want %q", ds.Kind, installKindPortable)
	}
	got, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatalf("target file does not exist: %v", err)
	}
	if string(got) != string(payload) {
		t.Errorf("target content = %q, want %q", got, payload)
	}
}
