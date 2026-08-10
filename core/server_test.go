package core

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// saveServerState 记录 server 相关全局状态并在测试结束后恢复。
func saveServerState(t *testing.T) (origLogs []string, origDir string) {
	t.Helper()
	serverLogsMu.Lock()
	origLogs = serverLogs
	serverLogsMu.Unlock()
	customLlamaCppMu.Lock()
	origDir = customLlamaCppDir
	customLlamaCppMu.Unlock()
	t.Cleanup(func() {
		serverLogsMu.Lock()
		serverLogs = origLogs
		serverLogsMu.Unlock()
		customLlamaCppMu.Lock()
		customLlamaCppDir = origDir
		customLlamaCppMu.Unlock()
	})
	return
}

// TestBuildServerCommand 验证服务命令构建：默认 llama-server 二进制
// 与固定参数序列（host/port/models-dir/preset/max/batching/webui）。
func TestBuildServerCommand(t *testing.T) {
	saveServerState(t)
	cfg := ServerConfig{Host: "127.0.0.1", Port: 8080, MaxModels: 1, CacheRAM: 0}
	bin, args := buildServerCommand(cfg, "/tmp/preset.ini")

	if bin != "llama-server" {
		t.Errorf("bin = %q, want llama-server", bin)
	}
	want := []string{
		"--host", "127.0.0.1",
		"--port", "8080",
		"--models-dir", "LLM-Models",
		"--models-preset", "/tmp/preset.ini",
		"--models-max", "1",
		"--cont-batching",
		"--no-webui",
	}
	if len(args) != len(want) {
		t.Fatalf("args = %v, want %v", args, want)
	}
	for i := range want {
		if args[i] != want[i] {
			t.Errorf("args[%d] = %q, want %q", i, args[i], want[i])
		}
	}
}

// TestBuildServerCommandCacheRAM 验证 CacheRAM 配置追加 --cache-ram 参数，
// 且 MaxModels 最小为 1（防止向 llama-server 传 0）。
func TestBuildServerCommandCacheRAM(t *testing.T) {
	cfg := ServerConfig{Host: "127.0.0.1", Port: 8080, MaxModels: 0, CacheRAM: 4096}
	_, args := buildServerCommand(cfg, "/tmp/preset.ini")

	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "--cache-ram 4096") {
		t.Errorf("args 缺少 --cache-ram: %v", args)
	}
	if !strings.Contains(joined, "--models-max 1") {
		t.Errorf("MaxModels=0 应回退为 1: %v", args)
	}
}

// TestBuildServerCommandCustomDir 验证 PATH 无 llama-server 时优先使用
// 自定义目录下存在的 llama-server(.exe) 二进制。
func TestBuildServerCommandCustomDir(t *testing.T) {
	saveServerState(t)
	custom := t.TempDir()
	binName := "llama-server"
	if runtime.GOOS == "windows" {
		binName += ".exe"
	}
	if err := os.WriteFile(filepath.Join(custom, binName), []byte("x"), 0755); err != nil {
		t.Fatal(err)
	}
	customLlamaCppMu.Lock()
	customLlamaCppDir = custom
	customLlamaCppMu.Unlock()

	cfg := ServerConfig{Host: "127.0.0.1", Port: 8080, MaxModels: 1, CacheRAM: 0}
	bin, _ := buildServerCommand(cfg, "/tmp/preset.ini")

	want := filepath.Join(custom, binName)
	if bin != want {
		t.Errorf("bin = %q, want %q", bin, want)
	}
}

// TestAddServerLogRingBuffer 验证服务日志环形缓冲：超过 200 条后裁剪为
// 最近 100 条，最新日志始终在末尾。
func TestAddServerLogRingBuffer(t *testing.T) {
	serverLogsMu.Lock()
	serverLogs = nil
	serverLogsMu.Unlock()
	defer func() {
		serverLogsMu.Lock()
		serverLogs = nil
		serverLogsMu.Unlock()
	}()

	for i := 0; i < 250; i++ {
		addServerLog("line")
	}

	serverLogsMu.Lock()
	defer serverLogsMu.Unlock()
	if len(serverLogs) > 200 {
		t.Errorf("日志数量 %d 超过上限 200", len(serverLogs))
	}
	if serverLogs[len(serverLogs)-1] != "line" {
		t.Errorf("最新日志应在末尾: %v", serverLogs[len(serverLogs)-1])
	}
}
