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

// llamaServerBinName 返回当前平台的 llama-server 二进制文件名
// （Windows 带 .exe 后缀），供测试构造 stub 使用。
func llamaServerBinName() string {
	if runtime.GOOS == "windows" {
		return "llama-server.exe"
	}
	return "llama-server"
}

// saveDownloadState 记录 llama.cpp 下载相关全局状态并在测试结束后恢复，
// 避免 downloadLlamaCpp 全链路测试污染其他用例（与 saveServerState 同风格）。
// llamaCacheValid 是独立于 downloadMu 的 atomic，直接读取并在 cleanup 恢复。
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

// TestGetLlamaCppInfoDetectsDownloadDir 验证 getLlamaCppInfo 能识别
// llama-cpp/ 下载目录中的 llama-server（下载解压后的默认落点）：切到临时
// 目录后建空 stub → Installed=true 且 Path 为绝对路径；对照组（无 llama-cpp/
// 目录）→ Installed=false。此前检测只查 PATH 与自定义目录，解压成功的
// 二进制永远无法被识别为已安装（主页显示"未找到"）。
func TestGetLlamaCppInfoDetectsDownloadDir(t *testing.T) {
	// PATH 上存在 llama 相关二进制会干扰对照组（被误判为已安装），跳过
	for _, bin := range []string{"llama-server", "llama-cli", "llama.cpp", "llama"} {
		if _, err := exec.LookPath(bin); err == nil {
			t.Skipf("PATH 中存在 %s，无法验证未安装场景，跳过", bin)
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
		t.Fatal("存在 llama-cpp/llama-server stub 时 Installed 应为 true")
	}
	wantPath := filepath.Join(tmp, "llama-cpp", binName)
	if info.Path != wantPath {
		t.Errorf("Path = %q, want 绝对路径 %q", info.Path, wantPath)
	}

	// 对照组：无 llama-cpp/ 目录时应判定未安装
	withTempCwd(t)
	if info := getLlamaCppInfo(); info.Installed {
		t.Error("无 llama-cpp/ 目录时 Installed 应为 false")
	}
}

// TestGetLlamaCppInfoDetectsDownloadDirSubdir 验证下载 zip 带顶层文件夹
// 时（解压后二进制位于 llama-cpp/<一层子目录>/ 下）同样能被检测到，且
// Path 精确指向子目录内的 stub（断言绝对路径相等，PATH 命中不会误判）。
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
		t.Fatal("llama-cpp/<子目录>/llama-server stub 时 Installed 应为 true")
	}
	wantPath := filepath.Join(tmp, subdir, binName)
	if info.Path != wantPath {
		t.Errorf("Path = %q, want %q", info.Path, wantPath)
	}
}

// TestBuildServerCommandDetectsDownloadDir 验证 buildServerCommand 能命中
// llama-cpp/ 下载目录中的 llama-server（此前只查 PATH 与自定义目录，下载
// 完成后 API 页无法启动服务）。下载目录优先级高于 PATH，命中返回绝对路径。
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

	cfg := ServerConfig{Host: "127.0.0.1", Port: 8080, MaxModels: 1, CacheRAM: 0}
	bin, _ := buildServerCommand(cfg, "/tmp/preset.ini")

	want := filepath.Join(tmp, "llama-cpp", binName)
	if bin != want {
		t.Errorf("bin = %q, want 下载目录绝对路径 %q", bin, want)
	}
}

// TestDownloadLlamaCppInvalidatesLlamaCache 验证 downloadLlamaCpp 解压成功
// 置 done 后 llamaCacheValid 被失效（此前只失效模型缓存，GetLlamaCpp 仍
// 返回挂载时缓存的 false，主页一直显示"未找到"）。全链路走通：httptest
// 返回含平台匹配 zip 资产的 release JSON，zip 内为 llama-server stub；
// githubReleasesAPI 注入本地服务器，避免真实网络。
func TestDownloadLlamaCppInvalidatesLlamaCache(t *testing.T) {
	saveDownloadState(t)
	saveServerState(t)
	withTempCwd(t)

	// 构造含 llama-server stub 的最小 zip
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

	// 按当前平台构造资产名（需含平台关键字才能被 pickBestAsset 选中；
	// Windows 命名如 llama-b9999-bin-win-cpu-x64.zip）
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

	// 预置缓存有效，验证下载完成后被失效
	llamaCacheValid.Store(true)

	downloadLlamaCpp()

	if llamaCacheValid.Load() {
		t.Error("downloadLlamaCpp 完成后 llamaCacheValid 应为 false")
	}
	downloadMu.Lock()
	status := downloadState.Status
	downloadMu.Unlock()
	if status != "done" {
		t.Errorf("下载状态 = %q, want done", status)
	}
	if _, err := os.Stat(filepath.Join("llama-cpp", llamaServerBinName())); err != nil {
		t.Errorf("解压产物缺失: %v", err)
	}
}

// TestDownloadLlamaCppUsesCustomDir 验证设置了自定义 llama.cpp 目录后，
// downloadLlamaCpp 将下载产物解压到该自定义目录而非默认的 llama-cpp/
// （此前固定解压到 llama-cpp/，自定义目录场景下产物落点与检测位置不一致）。
// 全链路走通：httptest 返回含平台匹配 zip 资产的 release JSON，zip 内为
// llama-server stub；customLlamaCppDir 指向另一个临时目录。
func TestDownloadLlamaCppUsesCustomDir(t *testing.T) {
	saveDownloadState(t)
	saveServerState(t)
	// customLlamaCppDir 保存/恢复由 saveServerState 负责，此处直接设置
	customDir := t.TempDir()
	withTempCwd(t)

	customLlamaCppMu.Lock()
	customLlamaCppDir = customDir
	customLlamaCppMu.Unlock()

	// 构造含 llama-server stub 的最小 zip
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

	// 按当前平台构造资产名（与 TestDownloadLlamaCppInvalidatesLlamaCache 同风格）
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

	// 预置缓存有效，验证下载完成后被失效（与对照组一致）
	llamaCacheValid.Store(true)

	downloadLlamaCpp()

	if llamaCacheValid.Load() {
		t.Error("downloadLlamaCpp 完成后 llamaCacheValid 应为 false")
	}
	downloadMu.Lock()
	status := downloadState.Status
	downloadMu.Unlock()
	if status != "done" {
		t.Fatalf("下载状态 = %q, want done", status)
	}

	// 解压产物应落在自定义目录（llama-server stub 直接位于 zip 根级）
	if _, err := os.Stat(filepath.Join(customDir, llamaServerBinName())); err != nil {
		t.Errorf("自定义目录中解压产物缺失: %v", err)
	}
	// 默认 llama-cpp/ 目录不应存在该产物（未装到默认目录）
	if _, err := os.Stat(filepath.Join("llama-cpp", llamaServerBinName())); err == nil {
		t.Error("llama-cpp/ 下不应存在解压产物（应只安装到自定义目录）")
	}
}
