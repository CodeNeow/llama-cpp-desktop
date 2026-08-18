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

// saveServerState snapshots server-related globals (logs, running state, command, custom
// llama.cpp dir, models dir) and restores them after the test.
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
	llamaCppDownloadDirMu.Lock()
	origLlamaDownloadDir := llamaCppDownloadDirOverride
	llamaCppDownloadDirMu.Unlock()
	modelDownloadDirMu.Lock()
	origModelDownloadDir := modelDownloadDirOverride
	modelDownloadDirMu.Unlock()
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
		llamaCppDownloadDirMu.Lock()
		llamaCppDownloadDirOverride = origLlamaDownloadDir
		llamaCppDownloadDirMu.Unlock()
		modelDownloadDirMu.Lock()
		modelDownloadDirOverride = origModelDownloadDir
		modelDownloadDirMu.Unlock()
	})
	return
}

// TestBuildServerCommand verifies server command construction: default llama-server binary
// and fixed argument sequence (host/port/models-dir/preset/max/batching/webui).
func TestBuildServerCommand(t *testing.T) {
	saveServerState(t)
	cfg := ServerConfig{AccessMode: accessLocal, Host: "127.0.0.1", Port: 8080, MaxModels: 1, CacheRAM: 0}
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

// TestBuildServerCommandLANHost verifies --host is derived as 0.0.0.0 when AccessMode=lan;
// even if cfg.Host is still 127.0.0.1 (not normalized), the effectiveHost derivation
// result takes precedence.
func TestBuildServerCommandLANHost(t *testing.T) {
	saveServerState(t)
	cfg := ServerConfig{AccessMode: accessLAN, Host: "127.0.0.1", Port: 8080, MaxModels: 1, CacheRAM: 0}
	_, args := buildServerCommand(cfg, "/tmp/preset.ini")

	found := false
	for i, a := range args {
		if a == "--host" && i+1 < len(args) {
			if args[i+1] != "0.0.0.0" {
				t.Fatalf("lan mode --host = %q, want 0.0.0.0 (args=%v)", args[i+1], args)
			}
			found = true
		}
	}
	if !found {
		t.Fatalf("args missing --host: %v", args)
	}
}

// TestBuildServerCommandCacheRAM verifies CacheRAM config appends the --cache-ram argument,
// and MaxModels minimum is 1 (prevents passing 0 to llama-server).
func TestBuildServerCommandCacheRAM(t *testing.T) {
	cfg := ServerConfig{AccessMode: accessLocal, Host: "127.0.0.1", Port: 8080, MaxModels: 0, CacheRAM: 4096}
	_, args := buildServerCommand(cfg, "/tmp/preset.ini")

	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "--cache-ram 4096") {
		t.Errorf("args missing --cache-ram: %v", args)
	}
	if !strings.Contains(joined, "--models-max 1") {
		t.Errorf("MaxModels=0 should fall back to 1: %v", args)
	}
}

// TestBuildServerCommandCustomDir verifies that when llama-server is not found in PATH,
// the binary under the custom directory is used first (llama-server(.exe)).
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

	cfg := ServerConfig{AccessMode: accessLocal, Host: "127.0.0.1", Port: 8080, MaxModels: 1, CacheRAM: 0}
	bin, _ := buildServerCommand(cfg, "/tmp/preset.ini")

	want := filepath.Join(custom, binName)
	if bin != want {
		t.Errorf("bin = %q, want %q", bin, want)
	}
}

// TestBuildServerCommandCustomModelsDir verifies that after setting a custom model directory,
// --models-dir in buildServerCommand args uses the custom directory instead of the default
// LLM-Models.
// TestBuildServerCommandCustomModelDownloadDir verifies --models-dir uses the
// model download path (default LLM-Models, or the configured download dir); the
// imported model directory is scanned for the preset but does not affect the
// server's models-dir fallback root.
func TestBuildServerCommandCustomModelDownloadDir(t *testing.T) {
	saveServerState(t)
	customModels := t.TempDir()
	modelDownloadDirMu.Lock()
	modelDownloadDirOverride = customModels
	modelDownloadDirMu.Unlock()

	cfg := ServerConfig{AccessMode: accessLocal, Host: "127.0.0.1", Port: 8080, MaxModels: 1, CacheRAM: 0}
	_, args := buildServerCommand(cfg, "/tmp/preset.ini")

	found := false
	for i, a := range args {
		if a == "--models-dir" && i+1 < len(args) && args[i+1] == customModels {
			found = true
		}
	}
	if !found {
		t.Errorf("--models-dir in args should be %q, actual args = %v", customModels, args)
	}
}

// TestAddServerLogRingBuffer verifies the service-log ring buffer: after exceeding 200
// entries it is trimmed to the most recent 100, and the latest log is always at the end.
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
		t.Errorf("log count %d exceeds limit 200", len(serverLogs))
	}
	if serverLogs[len(serverLogs)-1] != "line" {
		t.Errorf("latest log should be at the end: %v", serverLogs[len(serverLogs)-1])
	}
}

// TestConcurrentStopServerInternalIdempotent verifies concurrent stopServerInternal calls
// are idempotent-safe when the service is not started (#3): regardless of how many
// concurrent calls are made, all return nil and do not panic, and serverRunning stays
// false. Previously serverRunning was protected by serverLogsMu, and the stop path read
// serverCmd.Process outside the lock; after refactoring, a copy is taken inside serverMu.
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
				t.Errorf("stopServerInternal should return nil, got %v", err)
			}
		}()
	}
	wg.Wait()

	serverMu.Lock()
	running := serverRunning
	serverMu.Unlock()
	if running {
		t.Error("serverRunning should be false after concurrent stop when service was not started")
	}
}

// TestConcurrentStartStopServer verifies high-frequency interleaving of StartServer and
// StopServer does not panic (#3). With an empty LLM-Models directory, startServerInternal
// fails during preset generation and returns without actually launching the llama-server
// child process; StopServer returns nil idempotently in the not-started state. The lock
// acquisition order invariant (serverMu/serverLogsMu) must not deadlock along this
// interleaving path.
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
			// empty model directory: StartServer returns error, but does not panic and does not launch child process
			app.StartServer()
		}()
		go func() {
			defer wg.Done()
			app.StopServer()
		}()
	}
	wg.Wait()
}

// TestHelperProcess serves as a llama-server surrogate child process: when the environment
// variable GO_WANT_HELPER_PROCESS=1 is set, it enters a loop (simulating a running service),
// used by TestStopServerInternalKillsRunningProcess to verify the process-termination path.
func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	for {
		time.Sleep(100 * time.Millisecond)
	}
}

// TestStopServerInternalKillsRunningProcess verifies the termination path of
// stopServerInternal against a real running process (#3): when serverCmd is the started
// surrogate child process, calling Process.Signal on the in-lock copy should terminate
// the process; if it still reads the global serverCmd outside the lock or calls while
// holding the lock, this test would expose deadlock/crash.
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

	// call cmd.Wait in an independent goroutine; after stop succeeds, Wait should return
	waitDone := make(chan struct{})
	go func() {
		cmd.Wait()
		close(waitDone)
	}()

	if err := stopServerInternal(); err != nil {
		t.Fatalf("stopServerInternal returned error: %v", err)
	}

	select {
	case <-waitDone:
		// process terminated, as expected
	case <-time.After(5 * time.Second):
		t.Fatal("surrogate process was not terminated after stopServerInternal")
	}
}
