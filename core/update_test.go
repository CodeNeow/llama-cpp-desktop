package core

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"testing"
)

// saveUpdateState 记录更新相关全局状态（API URL、下载状态、可执行路径注入），
// 供测试结束后恢复，避免污染其他测试。
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

// TestCompareVersions 验证版本号比较剥去 v 前缀、按点分段逐段比较；
// 数值相等但段数不同时以缺省 0 补齐比较（1.0 < 1.0.1）。
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
		{"v0.1.0", "v0.1.0-alpha", 0}, // 非数字段按 0 处理
	}
	for _, c := range cases {
		if got := compareVersions(c.a, c.b); got != c.want {
			t.Errorf("compareVersions(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

// TestPickUpdateAsset 验证更新资产挑选：优先主程序 exe，跳过 installer。
func TestPickUpdateAsset(t *testing.T) {
	assets := []GitHubAsset{
		{Name: "llama-gui-amd64-installer.exe"},
		{Name: "llama-gui.exe", Size: 10516480},
	}
	got := pickUpdateAsset(assets)
	if got == nil || got.Name != "llama-gui.exe" {
		t.Errorf("应挑中 llama-gui.exe, 实际 %v", got)
	}
	if got := pickUpdateAsset([]GitHubAsset{{Name: "llama-gui-amd64-installer.exe"}}); got != nil {
		t.Errorf("只有 installer 时应返回 nil, 实际 %v", got)
	}
	if got := pickUpdateAsset(nil); got != nil {
		t.Errorf("空资产列表应返回 nil, 实际 %v", got)
	}
}

// TestCheckForUpdateNewer 验证远程版本高于当前版本时 hasUpdate 为 true，
// 且携带版本号与发布说明。
func TestCheckForUpdateNewer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"tag_name":"v9.9.9","name":"Release","body":"新增功能","published_at":"2026-08-10T00:00:00Z","assets":[]}`))
	}))
	defer srv.Close()

	res, err := CheckForUpdateAt(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if !res.HasUpdate {
		t.Error("v9.9.9 > 当前版本, hasUpdate 应为 true")
	}
	if res.Version != "v9.9.9" || !strings.Contains(res.Notes, "新增功能") {
		t.Errorf("版本信息错误: %+v", res)
	}
}

// TestCheckForUpdateSame 验证远程版本与当前版本相同时 hasUpdate 为 false。
// currentVersion 来自 core/VERSION 嵌入文件（tag 发布时由 CI 覆盖），
// 断言只校验格式（vX.Y.Z）而不绑定具体版本，避免每次发版改动测试。
func TestCheckForUpdateSame(t *testing.T) {
	if !regexp.MustCompile(`^v\d+\.\d+\.\d+$`).MatchString(currentVersion) {
		t.Fatalf("currentVersion 应形如 vX.Y.Z, 实际 %q", currentVersion)
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
		t.Error("版本相同时 hasUpdate 应为 false")
	}
}

// TestStartUpdateDownloadRejectsNotNewer 验证 StartUpdateDownload 对
// 不高于当前版本的版本号返回错误，不启动下载。
func TestStartUpdateDownloadRejectsNotNewer(t *testing.T) {
	app := &App{}
	if err := app.StartUpdateDownload(currentVersion); err == nil {
		t.Error("与当前版本相同的版本应返回错误")
	}
}

// TestDownloadUpdateRelease 验证更新下载完整流程：从注入的 release API
// 拉取信息 → 下载 exe 到可执行文件同目录 → 状态置 done 且文件存在，
// 文件名带版本号（llama-gui-v<tag>.exe）。
func TestDownloadUpdateRelease(t *testing.T) {
	withTempCwd(t)
	saveUpdateState(t)
	// 下载落盘目录用临时目录模拟「可执行文件同目录」
	exeDir := t.TempDir()
	updateExePath = func() (string, error) {
		return filepath.Join(exeDir, "llama-gui.exe"), nil
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
		t.Fatalf("下载完成状态 = %q, want done（错误: %s）", ds.Status, ds.Error)
	}
	wantPath := filepath.Join(exeDir, "llama-gui-v0.2.0.exe")
	if ds.FilePath != wantPath {
		t.Errorf("保存路径 = %q, want %q", ds.FilePath, wantPath)
	}
	fi, err := os.Stat(wantPath)
	if err != nil {
		t.Fatalf("目标文件不存在: %v", err)
	}
	if fi.Size() != int64(len(payload)) {
		t.Errorf("文件大小 = %d, want %d", fi.Size(), len(payload))
	}
}

// TestDownloadUpdateReleaseCrossDeviceFallback 验证更新下载在跨设备场景
// （renameFile 注入 EXDEV，对应 Windows 系统临时目录与可执行文件不同盘）
// 下通过复制回退完成保存：状态置 done、目标文件存在且内容正确。同时验证
// EXDEV 分支优先于删旧重试（目标已存在的旧文件不会被误删后再失败丢失）。
func TestDownloadUpdateReleaseCrossDeviceFallback(t *testing.T) {
	withTempCwd(t)
	saveUpdateState(t)
	// 下载落盘目录用临时目录模拟「可执行文件同目录」（与源临时文件不同设备）
	exeDir := t.TempDir()
	updateExePath = func() (string, error) {
		return filepath.Join(exeDir, "llama-gui.exe"), nil
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

	// 注入 renameFile 模拟跨设备失败（moveFile 内部调用该包级变量）
	origRename := renameFile
	renameFile = func(oldpath, newpath string) error {
		return syscall.EXDEV
	}
	defer func() { renameFile = origRename }()

	// 目标路径已存在旧版本文件，验证跨设备回退不会先删旧文件导致丢失
	wantPath := filepath.Join(exeDir, "llama-gui-v0.2.0.exe")
	if err := os.WriteFile(wantPath, []byte("old version"), 0644); err != nil {
		t.Fatal(err)
	}

	downloadUpdateRelease("v0.2.0")

	updateDownloadMu.Lock()
	ds := *updateDownloadState
	updateDownloadMu.Unlock()

	if ds.Status != "done" {
		t.Fatalf("跨设备回退后状态 = %q, want done（错误: %s）", ds.Status, ds.Error)
	}
	if ds.FilePath != wantPath {
		t.Errorf("保存路径 = %q, want %q", ds.FilePath, wantPath)
	}
	got, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatalf("目标文件不存在: %v", err)
	}
	if string(got) != string(payload) {
		t.Errorf("目标内容 = %q, want %q", got, payload)
	}
}
