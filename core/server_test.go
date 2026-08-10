package core

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

// saveServerState 记录 server 相关全局状态并在测试结束后恢复。
func saveServerState(t *testing.T) (origLogs []string, origDir string) {
	t.Helper()
	serverLogsMu.Lock()
	origLogs = serverLogs
	serverLogsMu.Unlock()
	serverMu.Lock()
	origRunning := serverRunning
	origCmd := serverCmd
	serverMu.Unlock()
	customLlamaCppMu.Lock()
	origDir = customLlamaCppDir
	customLlamaCppMu.Unlock()
	modelsDirMu.Lock()
	origModelsDir := customModelsDir
	modelsDirMu.Unlock()
	t.Cleanup(func() {
		serverLogsMu.Lock()
		serverLogs = origLogs
		serverLogsMu.Unlock()
		serverMu.Lock()
		serverRunning = origRunning
		serverCmd = origCmd
		serverMu.Unlock()
		customLlamaCppMu.Lock()
		customLlamaCppDir = origDir
		customLlamaCppMu.Unlock()
		modelsDirMu.Lock()
		customModelsDir = origModelsDir
		modelsDirMu.Unlock()
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

// TestBuildServerCommandCustomModelsDir 验证设置自定义模型目录后，
// buildServerCommand 的 args 中 --models-dir 使用自定义目录而非默认
// LLM-Models。
func TestBuildServerCommandCustomModelsDir(t *testing.T) {
	saveServerState(t)
	customModels := t.TempDir()
	modelsDirMu.Lock()
	customModelsDir = customModels
	modelsDirMu.Unlock()

	cfg := ServerConfig{Host: "127.0.0.1", Port: 8080, MaxModels: 1, CacheRAM: 0}
	_, args := buildServerCommand(cfg, "/tmp/preset.ini")

	found := false
	for i, a := range args {
		if a == "--models-dir" && i+1 < len(args) && args[i+1] == customModels {
			found = true
		}
	}
	if !found {
		t.Errorf("args 中 --models-dir 应为 %q, 实际 args = %v", customModels, args)
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

// TestConcurrentStopServerInternalIdempotent 验证服务未启动时并发调用
// stopServerInternal 幂等安全（#3）：无论并发多少次都返回 nil 且不 panic，
// serverRunning 保持 false。此前 serverRunning 由 serverLogsMu 保护，
// stop 路径锁外读 serverCmd.Process，重构后改为 serverMu 锁内取副本。
func TestConcurrentStopServerInternalIdempotent(t *testing.T) {
	saveServerState(t)
	serverMu.Lock()
	serverRunning = false
	serverCmd = nil
	serverMu.Unlock()

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := stopServerInternal(); err != nil {
				t.Errorf("stopServerInternal 应返回 nil, 实际 %v", err)
			}
		}()
	}
	wg.Wait()

	serverMu.Lock()
	running := serverRunning
	serverMu.Unlock()
	if running {
		t.Error("服务未启动时并发 stop 后 serverRunning 应为 false")
	}
}

// TestConcurrentStartStopServer 验证 StartServer 与 StopServer 高频交错
// 不 panic（#3）。在空 LLM-Models 目录下 startServerInternal 于预设生成
// 阶段失败并返回，不会真正启动 llama-server 子进程；StopServer 在未启动
// 状态幂等返回 nil。两把锁（serverMu/serverLogsMu）的获取顺序不变量在
// 此交错路径下不得死锁。
func TestConcurrentStartStopServer(t *testing.T) {
	withTempCwd(t)
	saveServerState(t)
	saveConfigState(t)
	serverMu.Lock()
	serverRunning = false
	serverCmd = nil
	serverMu.Unlock()

	app := &App{}
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			// 空模型目录：StartServer 返回错误，但不 panic、不启动子进程
			app.StartServer()
		}()
		go func() {
			defer wg.Done()
			app.StopServer()
		}()
	}
	wg.Wait()
}

// TestHelperProcess 作为 llama-server 的替身子进程：当环境变量
// GO_WANT_HELPER_PROCESS=1 时进入循环（模拟运行中的服务），供
// TestStopServerInternalKillsRunningProcess 验证进程终止路径。
func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	for {
		time.Sleep(100 * time.Millisecond)
	}
}

// TestStopServerInternalKillsRunningProcess 验证 stopServerInternal 对
// 真实运行进程的终止路径（#3）：serverCmd 为已启动的替身子进程时，
// 通过锁内副本调用 Process.Signal 应能终止进程；若仍锁外读全局
// serverCmd 或在锁内持锁调用，此测试会暴露死锁/崩溃。
func TestStopServerInternalKillsRunningProcess(t *testing.T) {
	saveServerState(t)
	serverMu.Lock()
	serverRunning = false
	serverCmd = nil
	serverMu.Unlock()

	cmd := exec.Command(os.Args[0], "-test.run=^TestHelperProcess$")
	cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	serverMu.Lock()
	serverRunning = true
	serverCmd = cmd
	serverMu.Unlock()
	defer func() {
		serverMu.Lock()
		serverRunning = false
		serverCmd = nil
		serverMu.Unlock()
		cmd.Process.Kill()
	}()

	// 在独立 goroutine 中调用 cmd.Wait，stop 成功后 Wait 应能返回
	waitDone := make(chan struct{})
	go func() {
		cmd.Wait()
		close(waitDone)
	}()

	if err := stopServerInternal(); err != nil {
		t.Fatalf("stopServerInternal 返回错误: %v", err)
	}

	select {
	case <-waitDone:
		// 进程已终止，符合预期
	case <-time.After(5 * time.Second):
		t.Fatal("stopServerInternal 后替身进程未被终止")
	}
}
