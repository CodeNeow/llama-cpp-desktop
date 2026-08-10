package core

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
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
