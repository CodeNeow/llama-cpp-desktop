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

// TestPickBestAssetForWindowsCUDAExcludesCudart 验证 Windows+CUDA 环境选择
// 主程序资产时排除 cudart 运行库资产：llama.cpp b10342 起 Windows CUDA 构建
// 拆分为 cudart 运行库 zip（cudart-llama-bin-win-cuda-XX.X-x64.zip）与主程序
// zip（llama-b*-bin-win-cuda-XX.X-x64.zip）两个资产，两者评分相同且 cudart
// 在 release 列表中排在主程序之前，若不排除会只选中运行库、漏掉主程序
// （用户现象：解压产物只有 cublas64_12.dll / cublasLt64_12.dll /
// cudart64_12.dll，没有 llama-server.exe）。断言按 release 顺序构造的
// [cudart, 主程序 cuda, 主程序 cpu] 列表选出主程序 cuda 资产而非排在前面的
// cudart。
func TestPickBestAssetForWindowsCUDAExcludesCudart(t *testing.T) {
	assets := []GitHubAsset{
		{Name: "cudart-llama-bin-win-cuda-12.4-x64.zip"},
		{Name: "llama-b9999-bin-win-cuda-12.4-x64.zip"},
		{Name: "llama-b9999-bin-win-cpu-x64.zip"},
	}
	got := pickBestAssetFor(assets, "windows", "amd64", true, "12.4")
	if got == nil || got.Name != "llama-b9999-bin-win-cuda-12.4-x64.zip" {
		t.Errorf("cudart 排在前时仍应选主程序 cuda 资产, 实际 %v", got)
	}
}

// TestPickCudartAssetFor 验证 cudart 运行库资产匹配：
//   - 精确版本命中且大小写不敏感（cudaVer=12.4 → cudart-...cuda-12.4-x64.zip）；
//   - 不存在的版本返回 nil（11.8 不在列表中）；
//   - 无 cudart 资产返回 nil；
//   - 空版本回退为任一 win cudart 资产（返回列表中第一个，best-effort 覆盖
//     无 nvcc、toolkit 版本解析失败的主机，保证全链路附加下载可验证）。
func TestPickCudartAssetFor(t *testing.T) {
	assets := []GitHubAsset{
		{Name: "cudart-llama-bin-win-cuda-12.4-x64.zip"},
		{Name: "cudart-llama-bin-win-cuda-13.3-x64.zip"},
	}
	if got := pickCudartAssetFor(assets, "12.4"); got == nil || got.Name != "cudart-llama-bin-win-cuda-12.4-x64.zip" {
		t.Errorf("cudaVer=12.4 应命中 12.4 资产, 实际 %v", got)
	}
	if got := pickCudartAssetFor(assets, "13.3"); got == nil || got.Name != "cudart-llama-bin-win-cuda-13.3-x64.zip" {
		t.Errorf("cudaVer=13.3 应命中 13.3 资产, 实际 %v", got)
	}
	if got := pickCudartAssetFor(assets, "11.8"); got != nil {
		t.Errorf("不存在的版本 11.8 应返回 nil, 实际 %v", got)
	}
	// 大小写不敏感：全大写资产名同样命中
	upper := []GitHubAsset{{Name: "CUDART-LLAMA-BIN-WIN-CUDA-12.4-X64.ZIP"}}
	if got := pickCudartAssetFor(upper, "12.4"); got == nil {
		t.Error("资产名大小写不敏感应命中")
	}
	// 无 cudart 资产返回 nil
	noCudart := []GitHubAsset{{Name: "llama-b9999-bin-win-cuda-12.4-x64.zip"}}
	if got := pickCudartAssetFor(noCudart, "12.4"); got != nil {
		t.Errorf("无 cudart 资产应返回 nil, 实际 %v", got)
	}
	// 空版本回退为列表中第一个 cudart 资产
	if got := pickCudartAssetFor(assets, ""); got == nil || got.Name != "cudart-llama-bin-win-cuda-12.4-x64.zip" {
		t.Errorf("空版本应回退为第一个 cudart 资产, 实际 %v", got)
	}
	if got := pickCudartAssetFor(nil, ""); got != nil {
		t.Errorf("空版本且无 cudart 资产应返回 nil, 实际 %v", got)
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
	// 错误串经 tr 按当前语言返回，固定 zh 保证「重命名失败」断言与语言无关。
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

// TestDownloadTaskRangeIgnoredRestart 是 #B3 的回归测试：服务器忽略 Range 头
// （带 offset 的续传请求返回 200 + 全量内容）时，downloadTask 必须先截断 .part
// 再重新从 0 下载，否则会把全量内容重复追加到已有部分导致文件损坏。
// 测试构造带 Range 头的场景：先注入一个已有 .part 文件（offset>0）并置
// Downloaded>0 模拟断点续传，httptest 服务器对带 Range 的请求返回 200 全量
// body（对不带 Range 的请求也返回 200 全量 body，保证 offset 归零后能跑通）。
// 断言：最终文件内容等于全量 body（不重复拼接）、服务器对带 Range 的请求恰好
// 只请求一次（offset 归零后重连不带 Range，不陷入无限循环）。
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

	payload := []byte("0123456789abcdef") // 16 字节全量内容
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

	// 预置已有 .part 文件（模拟中断的断点续传状态）与任务进度
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
		t.Fatalf("任务状态 = %q, want done（服务器忽略 Range 时应截断重下）", status)
	}

	got, err := os.ReadFile(filepath.Join(destDir, "model.gguf"))
	if err != nil {
		t.Fatalf("下载文件未落盘: %v", err)
	}
	// 文件内容必须等于全量 body，且不含旧部分（partial）——一旦重复追加即损坏
	if string(got) != string(payload) {
		t.Errorf("文件内容 = %q, want 全量 %q（不得把全量内容追加到旧 .part 上）", got, payload)
	}

	// offset 归零后的重连请求不带 Range，服务器不再收到带 Range 的请求
	if rangeReqCount != 1 {
		t.Errorf("带 Range 的请求次数 = %d, want 1（应只触发一次截断重连，offset=0 后不再带 Range）", rangeReqCount)
	}
}
