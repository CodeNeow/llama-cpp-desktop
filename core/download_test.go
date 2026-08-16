package core

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestPickBestAssetForWindowsCUDA verifies that in Windows+CUDA environments, the asset
// with an exact match to the host CUDA version is preferred.
func TestPickBestAssetForWindowsCUDA(t *testing.T) {
	assets := []GitHubAsset{
		{Name: "llama-b3840-bin-win-cuda-12.8-x64.zip"},
		{Name: "llama-b3840-bin-win-cuda-12.4-x64.zip"},
		{Name: "llama-b3840-bin-win-avx2-x64.zip"},
		{Name: "llama-b3840-bin-win-cpu-x64.zip"},
	}
	got := pickBestAssetFor(assets, "windows", "amd64", true, "12.8")
	if got == nil || got.Name != "llama-b3840-bin-win-cuda-12.8-x64.zip" {
		t.Errorf("should pick cuda-12.8 exact-match build, got %v", got)
	}
}

// TestPickBestAssetForWindowsCPU verifies that when no GPU is present, the avx2 build
// is selected instead of cuda.
func TestPickBestAssetForWindowsCPU(t *testing.T) {
	assets := []GitHubAsset{
		{Name: "llama-b3840-bin-win-cuda-12.8-x64.zip"},
		{Name: "llama-b3840-bin-win-avx2-x64.zip"},
		{Name: "llama-b3840-bin-win-cpu-x64.zip"},
	}
	got := pickBestAssetFor(assets, "windows", "amd64", false, "")
	if got == nil || got.Name != "llama-b3840-bin-win-avx2-x64.zip" {
		t.Errorf("no GPU should pick avx2 build, got %v", got)
	}
}

// TestPickBestAssetForLinuxArm64 verifies that Linux arm64 matches the arm64 archive.
func TestPickBestAssetForLinuxArm64(t *testing.T) {
	assets := []GitHubAsset{
		{Name: "llama-b3840-bin-linux-x64.zip"},
		{Name: "llama-b3840-bin-linux-arm64.zip"},
		{Name: "llama-b3840-bin-macos-arm64.zip"},
	}
	got := pickBestAssetFor(assets, "linux", "arm64", false, "")
	if got == nil || got.Name != "llama-b3840-bin-linux-arm64.zip" {
		t.Errorf("should pick linux arm64 build, got %v", got)
	}
}

// TestPickBestAssetForNoMatch verifies that nil is returned when no asset matches
// the current platform.
func TestPickBestAssetForNoMatch(t *testing.T) {
	assets := []GitHubAsset{{Name: "llama-b3840-bin-macos-arm64.zip"}}
	if got := pickBestAssetFor(assets, "windows", "amd64", false, ""); got != nil {
		t.Errorf("no windows assets should return nil, got %v", got)
	}
	if got := pickBestAssetFor(nil, "windows", "amd64", false, ""); got != nil {
		t.Errorf("nil asset list should return nil, got %v", got)
	}
}

// TestPickBestAssetForWindowsCUDAExcludesCudart verifies that when selecting the main
// program asset in a Windows+CUDA environment, cudart runtime assets are excluded:
// since llama.cpp b10342, Windows CUDA builds are split into cudart runtime zip
// (cudart-llama-bin-win-cuda-XX.X-x64.zip) and main program zip (llama-b*-bin-win-cuda-XX.X-x64.zip).
// Both score the same and cudart appears before the main program in the release list;
// without exclusion, only the runtime library would be selected and the main program
// would be missed (user-visible symptom: extracted artifacts only contain
// cublas64_12.dll / cublasLt64_12.dll / cudart64_12.dll, no llama-server.exe).
// Assertion: from a list ordered [cudart, main cuda, main cpu], the main cuda asset
// is selected rather than the preceding cudart.
func TestPickBestAssetForWindowsCUDAExcludesCudart(t *testing.T) {
	assets := []GitHubAsset{
		{Name: "cudart-llama-bin-win-cuda-12.4-x64.zip"},
		{Name: "llama-b9999-bin-win-cuda-12.4-x64.zip"},
		{Name: "llama-b9999-bin-win-cpu-x64.zip"},
	}
	got := pickBestAssetFor(assets, "windows", "amd64", true, "12.4")
	if got == nil || got.Name != "llama-b9999-bin-win-cuda-12.4-x64.zip" {
		t.Errorf("should still pick main cuda asset when cudart comes first, got %v", got)
	}
}

// TestPickCudartAssetFor verifies cudart runtime asset matching:
//   - exact version match, case-insensitive (cudaVer=12.4 → cudart-...cuda-12.4-x64.zip);
//   - non-existent version returns nil (11.8 not in list);
//   - no cudart assets returns nil;
//   - empty version falls back to any win cudart asset (returns first in list, best-effort
//     coverage for hosts without nvcc / failed toolkit version parsing, ensuring the
//     full download chain is verifiable).
func TestPickCudartAssetFor(t *testing.T) {
	assets := []GitHubAsset{
		{Name: "cudart-llama-bin-win-cuda-12.4-x64.zip"},
		{Name: "cudart-llama-bin-win-cuda-13.3-x64.zip"},
	}
	if got := pickCudartAssetFor(assets, "12.4"); got == nil || got.Name != "cudart-llama-bin-win-cuda-12.4-x64.zip" {
		t.Errorf("cudaVer=12.4 should match 12.4 asset, got %v", got)
	}
	if got := pickCudartAssetFor(assets, "13.3"); got == nil || got.Name != "cudart-llama-bin-win-cuda-13.3-x64.zip" {
		t.Errorf("cudaVer=13.3 should match 13.3 asset, got %v", got)
	}
	if got := pickCudartAssetFor(assets, "11.8"); got != nil {
		t.Errorf("non-existent version 11.8 should return nil, got %v", got)
	}
	// case-insensitive: all-uppercase asset name also matches
	upper := []GitHubAsset{{Name: "CUDART-LLAMA-BIN-WIN-CUDA-12.4-X64.ZIP"}}
	if got := pickCudartAssetFor(upper, "12.4"); got == nil {
		t.Error("asset name should be case-insensitive")
	}
	// no cudart assets returns nil
	noCudart := []GitHubAsset{{Name: "llama-b9999-bin-win-cuda-12.4-x64.zip"}}
	if got := pickCudartAssetFor(noCudart, "12.4"); got != nil {
		t.Errorf("no cudart assets should return nil, got %v", got)
	}
	// empty version falls back to first cudart asset in list
	if got := pickCudartAssetFor(assets, ""); got == nil || got.Name != "cudart-llama-bin-win-cuda-12.4-x64.zip" {
		t.Errorf("empty version should fall back to first cudart asset, got %v", got)
	}
	if got := pickCudartAssetFor(nil, ""); got != nil {
		t.Errorf("empty version with no cudart assets should return nil, got %v", got)
	}
}

// TestBuildDownloadRequest verifies download requests carry User-Agent and add a Range
// header for resume downloads.
func TestBuildDownloadRequest(t *testing.T) {
	req, err := buildDownloadRequest("https://example.com/model.gguf", 0)
	if err != nil {
		t.Fatal(err)
	}
	if req.Header.Get("User-Agent") != "llama-desktop" {
		t.Errorf("User-Agent = %q, want llama-desktop", req.Header.Get("User-Agent"))
	}
	if req.Header.Get("Range") != "" {
		t.Errorf("no offset should not have Range header: %q", req.Header.Get("Range"))
	}

	req, err = buildDownloadRequest("https://example.com/model.gguf", 1024)
	if err != nil {
		t.Fatal(err)
	}
	if req.Header.Get("Range") != "bytes=1024-" {
		t.Errorf("Range = %q, want bytes=1024-", req.Header.Get("Range"))
	}
}

// TestFetchLatestReleaseAt verifies fetching and parsing the latest release from an
// injected URL.
func TestFetchLatestReleaseAt(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"tag_name":"b3840","name":"Release","assets":[{"name":"a.zip","size":10,"browser_download_url":"https://x/a.zip"}]}`))
	}))
	defer srv.Close()

	rel, err := fetchLatestReleaseAt(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if rel.TagName != "b3840" || len(rel.Assets) != 1 || rel.Assets[0].Name != "a.zip" {
		t.Errorf("release parse error: %+v", rel)
	}
}

// TestFetchLatestReleaseAtHTTPError verifies a non-200 response returns an error.
func TestFetchLatestReleaseAtHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	if _, err := fetchLatestReleaseAt(srv.URL); err == nil {
		t.Error("500 response should return error")
	}
}

// TestDownloadTaskRenameFailure verifies that when the rename in the downloadTask
// completion branch fails, the task is marked as error (#10). Previously the os.Rename
// return value was ignored, causing silent done status with the file not in place;
// after the fix, renameFile (injectable package-level variable) failure immediately
// sets error. The test uses httptest to provide a fixed byte stream, injects renameFile
// failure, runs downloadTask end-to-end, and asserts task status and error message.
func TestDownloadTaskRenameFailure(t *testing.T) {
	// error string is returned by tr in the current language; pin zh so the
	// Chinese-prefix assertion below is language-independent.
	setLanguageForTest(t, "zh")
	withTempCwd(t)
	dlTasksMu.Lock()
	dlTasks = nil
	dlTaskCounter = 0
	dlTasksMu.Unlock()
	defer func() {
		dlTasksMu.Lock()
		dlTasks = nil
		dlTaskCounter = 0
		dlTasksMu.Unlock()
	}()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("fake model bytes"))
	}))
	defer srv.Close()

	// inject renameFile failure
	origRename := renameFile
	renameFile = func(oldpath, newpath string) error {
		return errors.New("mock rename fail")
	}
	defer func() { renameFile = origRename }()

	task := &DlTask{
		ID:       "dl-1",
		ModelID:  "author/model",
		FileName: "model.gguf",
		DestDir:  filepath.Join(effectiveModelsDir(), "author"),
		URL:      srv.URL,
	}
	task.ctx, task.cancel = context.WithCancel(context.Background())
	defer task.cancel()

	downloadTask(task)

	dlTasksMu.Lock()
	status := task.Status
	errMsg := task.Error
	dlTasksMu.Unlock()

	if status != "error" {
		t.Errorf("status after rename failure = %q, want error", status)
	}
	if !strings.Contains(errMsg, "重命名失败") {
		t.Errorf("error message should contain 重命名失败: %q", errMsg)
	}
}

// TestMoveFileCrossDeviceFallbackCopy verifies that moveFile falls back to
// copy + delete source when renameFile returns the current platform's real
// cross-device error (Windows cross-drive ERROR_NOT_SAME_DEVICE=17 /
// Unix cross-mount EXDEV, constant crossDeviceRenameErr is platform-specific,
// wrapped in LinkError to simulate os.Rename's real error shape):
// asserts destination content matches source, source is deleted, destination
// retains source file permissions (Linux update exe depends on execute bit;
// Windows os.Stat always reports 0666, so assert destination mode matches
// source actual mode rather than hardcoded 0755). renameFile is a package-level
// injection point, restored by defer.
func TestMoveFileCrossDeviceFallbackCopy(t *testing.T) {
	src := filepath.Join(t.TempDir(), "src.bin")
	dst := filepath.Join(t.TempDir(), "dst.bin")
	payload := []byte("cross device payload")
	if err := os.WriteFile(src, payload, 0755); err != nil {
		t.Fatal(err)
	}
	srcFi, err := os.Stat(src)
	if err != nil {
		t.Fatal(err)
	}
	origMode := srcFi.Mode().Perm()

	origRename := renameFile
	renameFile = func(oldpath, newpath string) error {
		return &os.LinkError{Op: "rename", Old: oldpath, New: newpath, Err: crossDeviceRenameErr}
	}
	defer func() { renameFile = origRename }()

	if err := moveFile(src, dst); err != nil {
		t.Fatalf("cross-device fallback should succeed: %v", err)
	}
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("failed to read destination: %v", err)
	}
	if string(got) != string(payload) {
		t.Errorf("destination content = %q, want %q", got, payload)
	}
	if _, err := os.Stat(src); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("source should be deleted, stat err = %v", err)
	}
	dstFi, err := os.Stat(dst)
	if err != nil {
		t.Fatal(err)
	}
	if dstFi.Mode().Perm() != origMode {
		t.Errorf("destination mode = %v, want source mode %v", dstFi.Mode().Perm(), origMode)
	}
}

// TestMoveFileNonCrossDeviceError verifies that when renameFile returns a non-cross-device
// error and the destination does not exist (simulating the TestDownloadTaskRenameFailure
// scenario), moveFile does not trigger copy fallback, does not mistakenly delete/move the
// source file, and returns the original error as-is.
func TestMoveFileNonCrossDeviceError(t *testing.T) {
	src := filepath.Join(t.TempDir(), "src.bin")
	dst := filepath.Join(t.TempDir(), "dst.bin")
	if err := os.WriteFile(src, []byte("payload"), 0644); err != nil {
		t.Fatal(err)
	}

	origRename := renameFile
	renameFile = func(oldpath, newpath string) error {
		return errors.New("mock rename fail")
	}
	defer func() { renameFile = origRename }()

	err := moveFile(src, dst)
	if err == nil {
		t.Fatal("non-EXDEV failure should return error")
	}
	if !strings.Contains(err.Error(), "mock rename fail") {
		t.Errorf("error should preserve original error message: %v", err)
	}
	if _, statErr := os.Stat(src); statErr != nil {
		t.Errorf("source file should not be moved or deleted on non-EXDEV failure: %v", statErr)
	}
	if _, statErr := os.Stat(dst); !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf("destination file should not be created on non-EXDEV failure")
	}
}

// TestMoveFileExdevDoesNotDeleteExistingDest verifies cross-device detection takes
// priority over "delete old and retry": when the destination already contains an old
// file and the operation is cross-device, moveFile copies over instead of deleting
// the old file first, preventing data loss if the copy fails. renameFile is injected
// with the current platform's real cross-device error (crossDeviceRenameErr, wrapped
// in LinkError); asserts old file content is overwritten and source file is deleted.
func TestMoveFileExdevDoesNotDeleteExistingDest(t *testing.T) {
	src := filepath.Join(t.TempDir(), "src.bin")
	dst := filepath.Join(t.TempDir(), "dst.bin")
	if err := os.WriteFile(src, []byte("new content"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, []byte("old content"), 0644); err != nil {
		t.Fatal(err)
	}

	origRename := renameFile
	renameFile = func(oldpath, newpath string) error {
		return &os.LinkError{Op: "rename", Old: oldpath, New: newpath, Err: crossDeviceRenameErr}
	}
	defer func() { renameFile = origRename }()

	if err := moveFile(src, dst); err != nil {
		t.Fatalf("cross-device fallback should succeed: %v", err)
	}
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new content" {
		t.Errorf("destination content = %q, want overwritten with new content", got)
	}
	if _, err := os.Stat(src); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("source should be deleted, stat err = %v", err)
	}
}

// TestDownloadTaskRangeIgnoredRestart is a #B3 regression test: when the server
// ignores the Range header (a resume request with offset returns 200 + full content),
// downloadTask must first truncate the .part file and re-download from 0, otherwise
// the full content would be repeatedly appended to the existing partial content,
// corrupting the file.
// The test constructs a Range-header scenario: first injects an existing .part file
// (offset>0) and sets Downloaded>0 to simulate interrupted resume; the httptest server
// returns 200 full body for Range requests (and also for non-Range requests, ensuring
// the reconnection after offset-zero can complete).
// Assertions: final file content equals full body (no duplicate concatenation); the
// server receives exactly one Range request (after offset-zero the reconnection does
// not use Range, avoiding infinite loops).
func TestDownloadTaskRangeIgnoredRestart(t *testing.T) {
	withTempCwd(t)
	dlTasksMu.Lock()
	dlTasks = nil
	dlTaskCounter = 0
	dlTasksMu.Unlock()
	defer func() {
		dlTasksMu.Lock()
		dlTasks = nil
		dlTaskCounter = 0
		dlTasksMu.Unlock()
	}()

	payload := []byte("0123456789abcdef") // 16 bytes full content
	rangeReqCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Range") != "" {
			rangeReqCount++
		}
		w.Header().Set("Content-Length", strconv.Itoa(len(payload)))
		w.Write(payload)
	}))
	defer srv.Close()

	destDir := filepath.Join(effectiveModelsDir(), "author")
	if err := os.MkdirAll(destDir, 0755); err != nil {
		t.Fatal(err)
	}

	// pre-populate existing .part file (simulating interrupted resume state) and task progress
	tmpPath := filepath.Join(destDir, "model.gguf.part")
	if err := os.WriteFile(tmpPath, []byte("partial"), 0644); err != nil {
		t.Fatal(err)
	}

	task := &DlTask{
		ID:         "dl-1",
		ModelID:    "author/model",
		FileName:   "model.gguf",
		DestDir:    destDir,
		URL:        srv.URL,
		Status:     "queued",
		Downloaded: int64(len("partial")),
		Total:      int64(len(payload) + len("partial")),
	}
	task.ctx, task.cancel = context.WithCancel(context.Background())
	defer task.cancel()

	downloadTask(task)

	dlTasksMu.Lock()
	status := task.Status
	dlTasksMu.Unlock()
	if status != "done" {
		t.Fatalf("task status = %q, want done (server ignoring Range must truncate and redownload)", status)
	}

	got, err := os.ReadFile(filepath.Join(destDir, "model.gguf"))
	if err != nil {
		t.Fatalf("downloaded file not written to disk: %v", err)
	}
	// file content must equal the full body and not contain the old partial
	// bytes — appending the full body onto the old partial corrupts the file
	if string(got) != string(payload) {
		t.Errorf("file content = %q, want full %q (must not append the full body onto the old .part)", got, payload)
	}

	// after offset resets to zero the reconnect carries no Range header; the
	// server must not see another Range request
	if rangeReqCount != 1 {
		t.Errorf("Range request count = %d, want 1 (truncate-reconnect happens once; offset=0 sends no Range header)", rangeReqCount)
	}
}
