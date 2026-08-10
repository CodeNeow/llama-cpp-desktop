package core

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// TestPickBestAssetForWindowsCUDA 验证 Windows+CUDA 环境下优先选择与
// 本机 CUDA 版本精确匹配的 cuda 构建。
func TestPickBestAssetForWindowsCUDA(t *testing.T) {
	assets := []GitHubAsset{
		{Name: "llama-b3840-bin-win-cuda-12.8-x64.zip"},
		{Name: "llama-b3840-bin-win-cuda-12.4-x64.zip"},
		{Name: "llama-b3840-bin-win-avx2-x64.zip"},
		{Name: "llama-b3840-bin-win-cpu-x64.zip"},
	}
	got := pickBestAssetFor(assets, "windows", "amd64", true, "12.8")
	if got == nil || got.Name != "llama-b3840-bin-win-cuda-12.8-x64.zip" {
		t.Errorf("应选 cuda-12.8 精确匹配构建, 实际 %v", got)
	}
}

// TestPickBestAssetForWindowsCPU 验证无 GPU 时选择 avx2 构建而非 cuda。
func TestPickBestAssetForWindowsCPU(t *testing.T) {
	assets := []GitHubAsset{
		{Name: "llama-b3840-bin-win-cuda-12.8-x64.zip"},
		{Name: "llama-b3840-bin-win-avx2-x64.zip"},
		{Name: "llama-b3840-bin-win-cpu-x64.zip"},
	}
	got := pickBestAssetFor(assets, "windows", "amd64", false, "")
	if got == nil || got.Name != "llama-b3840-bin-win-avx2-x64.zip" {
		t.Errorf("无 GPU 时应选 avx2 构建, 实际 %v", got)
	}
}

// TestPickBestAssetForLinuxArm64 验证 Linux arm64 匹配 arm64 归档。
func TestPickBestAssetForLinuxArm64(t *testing.T) {
	assets := []GitHubAsset{
		{Name: "llama-b3840-bin-linux-x64.zip"},
		{Name: "llama-b3840-bin-linux-arm64.zip"},
		{Name: "llama-b3840-bin-macos-arm64.zip"},
	}
	got := pickBestAssetFor(assets, "linux", "arm64", false, "")
	if got == nil || got.Name != "llama-b3840-bin-linux-arm64.zip" {
		t.Errorf("应选 linux arm64 构建, 实际 %v", got)
	}
}

// TestPickBestAssetForNoMatch 验证没有匹配当前平台的资产时返回 nil。
func TestPickBestAssetForNoMatch(t *testing.T) {
	assets := []GitHubAsset{{Name: "llama-b3840-bin-macos-arm64.zip"}}
	if got := pickBestAssetFor(assets, "windows", "amd64", false, ""); got != nil {
		t.Errorf("无 windows 资产应返回 nil, 实际 %v", got)
	}
	if got := pickBestAssetFor(nil, "windows", "amd64", false, ""); got != nil {
		t.Errorf("空资产列表应返回 nil, 实际 %v", got)
	}
}

// TestBuildDownloadRequest 验证下载请求带 User-Agent，续传时加 Range 头。
func TestBuildDownloadRequest(t *testing.T) {
	req, err := buildDownloadRequest("https://example.com/model.gguf", 0)
	if err != nil {
		t.Fatal(err)
	}
	if req.Header.Get("User-Agent") != "llama-gui" {
		t.Errorf("User-Agent = %q, want llama-gui", req.Header.Get("User-Agent"))
	}
	if req.Header.Get("Range") != "" {
		t.Errorf("无偏移时不应有 Range 头: %q", req.Header.Get("Range"))
	}

	req, err = buildDownloadRequest("https://example.com/model.gguf", 1024)
	if err != nil {
		t.Fatal(err)
	}
	if req.Header.Get("Range") != "bytes=1024-" {
		t.Errorf("Range = %q, want bytes=1024-", req.Header.Get("Range"))
	}
}

// TestFetchLatestReleaseAt 验证从注入的 URL 拉取并解析最新 release。
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
		t.Errorf("release 解析错误: %+v", rel)
	}
}

// TestFetchLatestReleaseAtHTTPError 验证非 200 响应返回错误。
func TestFetchLatestReleaseAtHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	if _, err := fetchLatestReleaseAt(srv.URL); err == nil {
		t.Error("500 响应应返回错误")
	}
}

// TestDownloadTaskRenameFailure 验证 downloadTask 完成分支重命名失败时
// 任务标记为 error（#10）。此前 os.Rename 返回值被忽略，失败会静默把
// 任务置为 done 但文件未就位；修复后 renameFile（可注入包级变量）失败
// 即置 error。测试用 httptest 提供固定字节流，注入 renameFile 失败并
// 完整跑通 downloadTask，断言任务状态与错误信息。
func TestDownloadTaskRenameFailure(t *testing.T) {
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

	// 注入 renameFile 失败
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
		t.Errorf("rename 失败后任务状态 = %q, want error", status)
	}
	if !strings.Contains(errMsg, "重命名失败") {
		t.Errorf("错误信息应包含 重命名失败: %q", errMsg)
	}
}

// TestMoveFileCrossDeviceFallbackCopy 验证 moveFile 在 renameFile 返回
// EXDEV（Windows 跨盘 ERROR_NOT_SAME_DEVICE / Unix 跨挂载点）时回退为
// 复制 + 删除源文件：断言目标内容与源一致、源已删除、目标保留源文件权限
// （Linux 更新 exe 依赖执行位；Windows 上 os.Stat 恒报 0666，故断言与源
// 实际 mode 一致而非硬编码 0755）。renameFile 为包级注入点，defer 恢复。
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
		return syscall.EXDEV
	}
	defer func() { renameFile = origRename }()

	if err := moveFile(src, dst); err != nil {
		t.Fatalf("跨设备回退应成功: %v", err)
	}
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("读取目标文件失败: %v", err)
	}
	if string(got) != string(payload) {
		t.Errorf("目标内容 = %q, want %q", got, payload)
	}
	if _, err := os.Stat(src); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("源文件应已被删除, stat err = %v", err)
	}
	dstFi, err := os.Stat(dst)
	if err != nil {
		t.Fatal(err)
	}
	if dstFi.Mode().Perm() != origMode {
		t.Errorf("目标权限 = %v, want 与源一致 %v", dstFi.Mode().Perm(), origMode)
	}
}

// TestMoveFileNonCrossDeviceError 验证 renameFile 返回非 EXDEV 错误且目标
// 不存在时（模拟 TestDownloadTaskRenameFailure 的场景），moveFile 不触发
// 复制回退、不误删/误动源文件，按原语义返回原始错误。
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
		t.Fatal("非 EXDEV 失败应返回错误")
	}
	if !strings.Contains(err.Error(), "mock rename fail") {
		t.Errorf("错误应保留原始错误信息: %v", err)
	}
	if _, statErr := os.Stat(src); statErr != nil {
		t.Errorf("非 EXDEV 失败时源文件不应被移动或删除: %v", statErr)
	}
	if _, statErr := os.Stat(dst); !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf("非 EXDEV 失败时目标文件不应被创建: %v", statErr)
	}
}

// TestMoveFileExdevDoesNotDeleteExistingDest 验证 EXDEV 判定优先于「删旧重试」：
// 目标已存在旧文件且跨设备时，moveFile 走复制覆盖而非先删除旧文件，避免
// 旧文件在复制失败时丢失。注入 renameFile 返回 EXDEV，断言旧文件内容被覆盖、
// 源文件被删除。
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
		return syscall.EXDEV
	}
	defer func() { renameFile = origRename }()

	if err := moveFile(src, dst); err != nil {
		t.Fatalf("跨设备回退应成功: %v", err)
	}
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new content" {
		t.Errorf("目标内容 = %q, want 覆盖为新内容", got)
	}
	if _, err := os.Stat(src); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("源文件应已被删除, stat err = %v", err)
	}
}
