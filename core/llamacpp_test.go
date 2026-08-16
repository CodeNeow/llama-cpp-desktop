package core

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// llamaServerBinName returns the llama-server binary filename for the current platform
// (Windows includes .exe suffix), used by tests to construct stubs.
func llamaServerBinName() string {
	if runtime.GOOS == "windows" {
		return "llama-server.exe"
	}
	return "llama-server"
}

// saveDownloadState snapshots llama.cpp download-related globals and restores them
// after the test, preventing full-chain downloadLlamaCpp tests from polluting other
// test cases (same style as saveServerState).
// llamaCacheValid is an atomic independent of downloadMu; read directly and restored
// in cleanup.
func saveDownloadState(t *testing.T) {
	t.Helper()
	downloadMu.Lock()
	orig := *downloadState
	origCancel := downloadCancel
	downloadMu.Unlock()
	origLlamaCache := llamaCacheValid.Load()
	t.Cleanup(func() {
		downloadMu.Lock()
		*downloadState = orig
		downloadCancel = origCancel
		downloadMu.Unlock()
		llamaCacheValid.Store(origLlamaCache)
	})
}

// TestGetLlamaCppInfoDetectsDownloadDir verifies getLlamaCppInfo can detect llama-server
// in the llama-cpp/ download directory (the default landing path after download+extract):
// after switching to a temp directory and creating an empty stub, Installed=true and
// Path is an absolute path; the control group (no llama-cpp/ directory) → Installed=false.
// Previously detection only checked PATH and custom directories; a successfully extracted
// binary could never be recognized as installed (home page showed "not found").
func TestGetLlamaCppInfoDetectsDownloadDir(t *testing.T) {
	// llama-related binaries on PATH would interfere with the control group (misdetected as installed), skip
	for _, bin := range []string{"llama-server", "llama-cli", "llama.cpp", "llama"} {
		if _, err := exec.LookPath(bin); err == nil {
			t.Skipf("PATH contains %s, cannot verify not-installed scenario, skipping", bin)
		}
	}
	saveServerState(t)

	tmp := withTempCwd(t)
	binName := llamaServerBinName()
	if err := os.MkdirAll("llama-cpp", 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join("llama-cpp", binName), []byte("stub"), 0755); err != nil {
		t.Fatal(err)
	}

	info := getLlamaCppInfo()
	if !info.Installed {
		t.Fatal("Installed should be true when llama-cpp/llama-server stub exists")
	}
	wantPath := filepath.Join(tmp, "llama-cpp", binName)
	if info.Path != wantPath {
		t.Errorf("Path = %q, want absolute path %q", info.Path, wantPath)
	}

	// control group: no llama-cpp/ directory should be judged as not installed
	withTempCwd(t)
	if info := getLlamaCppInfo(); info.Installed {
		t.Error("Installed should be false when no llama-cpp/ directory exists")
	}
}

// TestGetLlamaCppInfoDetectsDownloadDirSubdir verifies that when the downloaded zip
// contains a top-level folder (after extraction the binary is under llama-cpp/<one-subdir>/),
// it is still detected, and Path points precisely to the stub inside the subdirectory
// (assert absolute path equality; PATH matches must not misclassify).
func TestGetLlamaCppInfoDetectsDownloadDirSubdir(t *testing.T) {
	saveServerState(t)
	tmp := withTempCwd(t)
	binName := llamaServerBinName()
	subdir := filepath.Join("llama-cpp", "llama-b9999-bin")
	if err := os.MkdirAll(subdir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subdir, binName), []byte("stub"), 0755); err != nil {
		t.Fatal(err)
	}

	info := getLlamaCppInfo()
	if !info.Installed {
		t.Fatal("Installed should be true when llama-cpp/<subdir>/llama-server stub exists")
	}
	wantPath := filepath.Join(tmp, subdir, binName)
	if info.Path != wantPath {
		t.Errorf("Path = %q, want %q", info.Path, wantPath)
	}
}

// TestBuildServerCommandDetectsDownloadDir verifies buildServerCommand can hit the
// llama-server in the llama-cpp/ download directory (previously only checked PATH
// and custom directories; after download the API page could not start the service).
// Download directory has higher priority than PATH; returns absolute path on hit.
func TestBuildServerCommandDetectsDownloadDir(t *testing.T) {
	saveServerState(t)
	tmp := withTempCwd(t)
	binName := llamaServerBinName()
	if err := os.MkdirAll("llama-cpp", 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join("llama-cpp", binName), []byte("stub"), 0755); err != nil {
		t.Fatal(err)
	}

	cfg := ServerConfig{AccessMode: accessLocal, Host: "127.0.0.1", Port: 8080, MaxModels: 1, CacheRAM: 0}
	bin, _ := buildServerCommand(cfg, "/tmp/preset.ini")

	want := filepath.Join(tmp, "llama-cpp", binName)
	if bin != want {
		t.Errorf("bin = %q, want download-dir absolute path %q", bin, want)
	}
}

// TestDownloadLlamaCppInvalidatesLlamaCache verifies that after downloadLlamaCpp
// successfully extracts and sets status to done, llamaCacheValid is invalidated
// (previously only model cache was invalidated; GetLlamaCpp still returned the
// mounted-cached false, home page always showed "not found"). Full chain end-to-end:
// httptest returns release JSON containing platform-matching zip assets, zip contains
// llama-server stub; githubReleasesAPI is injected with a local server to avoid
// real network.
func TestDownloadLlamaCppInvalidatesLlamaCache(t *testing.T) {
	saveDownloadState(t)
	saveServerState(t)
	withTempCwd(t)

	// build a minimal zip containing a llama-server stub
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create(llamaServerBinName())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("stub")); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	zipBytes := buf.Bytes()

	// construct asset name for current platform (must contain platform keyword to be picked by pickBestAsset;
	// Windows names like llama-b9999-bin-win-cpu-x64.zip)
	platformKey := map[string]string{"windows": "win", "darwin": "macos", "linux": "linux"}[runtime.GOOS]
	archKey := map[string]string{"amd64": "x64", "arm64": "arm64"}[runtime.GOARCH]
	assetName := fmt.Sprintf("llama-b9999-bin-%s-%s.zip", platformKey, archKey)

	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/release":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(GitHubRelease{
				TagName: "b9999",
				Assets: []GitHubAsset{{
					Name:               assetName,
					Size:               int64(len(zipBytes)),
					BrowserDownloadURL: srv.URL + "/llama.zip",
				}},
			})
		case "/llama.zip":
			w.Write(zipBytes)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	origAPI := githubReleasesAPI
	githubReleasesAPI = srv.URL + "/release"
	defer func() { githubReleasesAPI = origAPI }()

	// pre-populate cache as valid; verify it is invalidated after download completes
	llamaCacheValid.Store(true)

	downloadLlamaCpp()

	if llamaCacheValid.Load() {
		t.Error("llamaCacheValid should be false after downloadLlamaCpp completes")
	}
	downloadMu.Lock()
	status := downloadState.Status
	downloadMu.Unlock()
	if status != "done" {
		t.Errorf("download status = %q, want done", status)
	}
	if _, err := os.Stat(filepath.Join("llama-cpp", llamaServerBinName())); err != nil {
		t.Errorf("extracted artifact missing: %v", err)
	}
}

// TestDownloadLlamaCppUsesCustomDir verifies that after setting a custom llama.cpp
// directory, downloadLlamaCpp extracts download artifacts into that custom directory
// instead of the default llama-cpp/ (previously extraction was fixed to llama-cpp/,
// causing product landing and detection positions to mismatch in custom-dir scenarios).
// Full chain end-to-end: httptest returns release JSON containing platform-matching
// zip assets, zip contains llama-server stub; customLlamaCppDir points to another
// temp directory.
func TestDownloadLlamaCppUsesCustomDir(t *testing.T) {
	saveDownloadState(t)
	saveServerState(t)
	// customLlamaCppDir save/restore is handled by saveServerState, set it directly here
	customDir := t.TempDir()
	withTempCwd(t)

	customLlamaCppMu.Lock()
	customLlamaCppDir = customDir
	customLlamaCppMu.Unlock()

	// build a minimal zip containing a llama-server stub
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create(llamaServerBinName())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("stub")); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	zipBytes := buf.Bytes()

	// construct asset name for current platform (same style as TestDownloadLlamaCppInvalidatesLlamaCache)
	platformKey := map[string]string{"windows": "win", "darwin": "macos", "linux": "linux"}[runtime.GOOS]
	archKey := map[string]string{"amd64": "x64", "arm64": "arm64"}[runtime.GOARCH]
	assetName := fmt.Sprintf("llama-b9999-bin-%s-%s.zip", platformKey, archKey)

	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/release":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(GitHubRelease{
				TagName: "b9999",
				Assets: []GitHubAsset{{
					Name:               assetName,
					Size:               int64(len(zipBytes)),
					BrowserDownloadURL: srv.URL + "/llama.zip",
				}},
			})
		case "/llama.zip":
			w.Write(zipBytes)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	origAPI := githubReleasesAPI
	githubReleasesAPI = srv.URL + "/release"
	defer func() { githubReleasesAPI = origAPI }()

	// pre-populate cache as valid; verify it is invalidated after download completes (same as control group)
	llamaCacheValid.Store(true)

	downloadLlamaCpp()

	if llamaCacheValid.Load() {
		t.Error("llamaCacheValid should be false after downloadLlamaCpp completes")
	}
	downloadMu.Lock()
	status := downloadState.Status
	downloadMu.Unlock()
	if status != "done" {
		t.Fatalf("download status = %q, want done", status)
	}

	// extracted artifact should land in the custom directory (llama-server stub directly at zip root)
	if _, err := os.Stat(filepath.Join(customDir, llamaServerBinName())); err != nil {
		t.Errorf("extracted artifact missing in custom directory: %v", err)
	}
	// the default llama-cpp/ directory should not contain the artifact (not installed to default dir)
	if _, err := os.Stat(filepath.Join("llama-cpp", llamaServerBinName())); err == nil {
		t.Error("extracted artifact should not exist under llama-cpp/ (should install to custom dir only)")
	}
}

// makeZip constructs a minimal zip containing files (filename → content) and returns
// the byte stream, used by downloadLlamaCpp full-chain tests to simulate release assets.
func makeZip(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// TestDownloadLlamaCppDownloadsCudartOnWindows verifies that on Windows, llama.cpp CUDA
// builds (since b10342, main program and cudart runtime are split into two zip assets)
// download both the main program and cudart assets and extract them to the same directory:
// the target directory contains both llama-server and cublas64_12.dll stub (previously
// only the first-matched cudart asset was selected, extraction artifacts only contained
// the runtime library with no main program); downloadState.Total is the sum of both
// asset sizes, FileName stops at the last downloaded asset name. Asset name version
// is fixed at 12.4 and the GPU probe is stubbed to a pre-Blackwell compute capability,
// keeping asset selection deterministic regardless of host toolkit or GPU.
// Non-Windows platforms do not attach the Windows-exclusive cudart
// asset (matching logic is covered by unit tests), skipped.
func TestDownloadLlamaCppDownloadsCudartOnWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("cudart co-download is Windows-only; skipped on non-Windows (matching logic covered by unit tests)")
	}
	saveDownloadState(t)
	saveServerState(t)
	withTempCwd(t)

	// build the main-program zip (llama-server stub) and the cudart zip (cublas64_12.dll stub)
	mainZip := makeZip(t, map[string]string{llamaServerBinName(): "stub"})
	cudartZip := makeZip(t, map[string]string{"cublas64_12.dll": "stub"})

	// fixed version label keeps asset selection deterministic regardless of the host toolkit
	ver := "12.4"
	mainName := fmt.Sprintf("llama-b9999-bin-win-cuda-%s-x64.zip", ver)
	cudartName := fmt.Sprintf("cudart-llama-bin-win-cuda-%s-x64.zip", ver)

	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/release":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(GitHubRelease{
				TagName: "b9999",
				// mimic real release order: cudart listed first, verifying the main program is still selected
				Assets: []GitHubAsset{
					{Name: cudartName, Size: int64(len(cudartZip)), BrowserDownloadURL: srv.URL + "/cudart.zip"},
					{Name: mainName, Size: int64(len(mainZip)), BrowserDownloadURL: srv.URL + "/main.zip"},
				},
			})
		case "/main.zip":
			w.Write(mainZip)
		case "/cudart.zip":
			w.Write(cudartZip)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	origAPI := githubReleasesAPI
	githubReleasesAPI = srv.URL + "/release"
	defer func() { githubReleasesAPI = origAPI }()

	// stub the GPU probe: a pre-Blackwell host (no CUDA floor) keeps asset selection
	// deterministic — Blackwell hosts would floor-skip the 12.4 fixture asset
	origCC := probeGPUComputeCap
	probeGPUComputeCap = func() string { return "8.9" }
	defer func() { probeGPUComputeCap = origCC }()

	downloadLlamaCpp()

	downloadMu.Lock()
	status := downloadState.Status
	fileName := downloadState.FileName
	total := downloadState.Total
	errMsg := downloadState.Error
	downloadMu.Unlock()

	if status != "done" {
		t.Fatalf("download status = %q, want done (error: %s)", status, errMsg)
	}
	// with main program first then cudart, FileName should stop at the last downloaded cudart asset name
	if fileName != cudartName {
		t.Errorf("FileName = %q, want last downloaded cudart asset %q", fileName, cudartName)
	}
	// progress Total should be the sum of both asset sizes
	wantTotal := int64(len(mainZip) + len(cudartZip))
	if total != wantTotal {
		t.Errorf("Total = %d, want sum of both assets %d", total, wantTotal)
	}
	if _, err := os.Stat(filepath.Join("llama-cpp", llamaServerBinName())); err != nil {
		t.Errorf("main program artifact missing after extraction: %v", err)
	}
	if _, err := os.Stat(filepath.Join("llama-cpp", "cublas64_12.dll")); err != nil {
		t.Errorf("cudart runtime artifact missing after extraction: %v", err)
	}
}
