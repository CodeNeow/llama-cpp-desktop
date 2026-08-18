package core

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// ─── Wails binding helpers ───────────────────────────────────────

// osInterrupt is os.Interrupt on Unix, os.Kill on Windows.
var osInterrupt = func() os.Signal {
	if runtime.GOOS == "windows" {
		return os.Kill
	}
	return os.Interrupt
}()

// ─── Server start/stop (extracted from HTTP handlers) ────────────

func startServerInternal() error {
	serverConfigMu.Lock()
	cfg := cachedServerConfig
	serverConfigMu.Unlock()

	// Generate models preset file
	presetPath, err := generateModelsPreset()
	if err != nil {
		return fmt.Errorf(tr("生成模型预设失败: %w", "failed to generate models preset: %w"), err)
	}

	// Build command
	llamaServer, args := buildServerCommand(cfg, presetPath)

	// Create the command and bind log output inside serverMu (#3). Do not set
	// serverRunning=true yet: it must only be set after Start() succeeds,
	// preserving the invariant "serverRunning==true ⟹ serverCmd.Process != nil".
	serverMu.Lock()
	cmd := exec.Command(llamaServer, args...)
	hideWindow(cmd)
	cmd.Stdout = &serverLogWriter{}
	cmd.Stderr = &serverLogWriter{}
	serverLogsMu.Lock()
	serverLogs = []string{}
	serverLogsMu.Unlock()
	serverMu.Unlock()

	addServerLog(fmt.Sprintf("[INFO] Starting llama-server: %s %s", llamaServer, strings.Join(args, " ")))

	if err := cmd.Start(); err != nil {
		serverMu.Lock()
		serverCmd = nil
		serverRunning = false
		serverStartTime = time.Time{}
		serverMu.Unlock()
		addServerLog("[ERROR] Failed to start: " + err.Error())
		return err
	}

	serverMu.Lock()
	serverCmd = cmd
	serverRunning = true
	serverStartTime = time.Now()
	serverPort = cfg.Port
	serverMu.Unlock()

	go func(cmd *exec.Cmd) {
		err := cmd.Wait()
		serverMu.Lock()
		serverRunning = false
		serverStartTime = time.Time{}
		serverPort = 0
		// Only clear the global cmd reference when it still points to this
		// instance, to avoid clobbering a newly started server.
		if serverCmd == cmd {
			serverCmd = nil
		}
		serverMu.Unlock()
		if err != nil {
			addServerLog("[WARN] llama-server exited: " + err.Error())
		} else {
			addServerLog("[INFO] llama-server stopped")
		}
	}(cmd)

	return nil
}

// buildServerCommand resolves the llama-server binary (custom dir, then the
// llama-cpp/ download dir, then PATH) and builds its argument list from the
// server config. The preset path points at the generated models INI file.
func buildServerCommand(cfg ServerConfig, presetPath string) (string, []string) {
	// Shares resolveLlamaServerBin with getLlamaCppInfo to keep llama.cpp
	// install-location resolution consistent in both places (download dir is
	// ready to serve immediately after extraction); falls back to the bare
	// binary name when not found, letting exec.Command report the error.
	llamaServer := resolveLlamaServerBin()
	if llamaServer == "" {
		llamaServer = "llama-server"
	}

	args := []string{
		"--host", effectiveHost(cfg.AccessMode),
		"--port", strconv.Itoa(cfg.Port),
		"--models-dir", effectiveModelDownloadDir(),
		"--models-preset", presetPath,
		"--models-max", strconv.Itoa(max(cfg.MaxModels, 1)),
		"--cont-batching",
		"--no-webui",
	}
	if cfg.CacheRAM > 0 {
		args = append(args, "--cache-ram", strconv.Itoa(cfg.CacheRAM))
	}
	return llamaServer, args
}

func stopServerInternal() error {
	// Read running/cmd local copies inside serverMu, operate on the copies
	// outside (#3), to avoid concurrent access to serverCmd/Process between
	// stopServerInternal and start/goroutine.
	serverMu.Lock()
	running := serverRunning
	cmd := serverCmd
	serverMu.Unlock()
	if !running || cmd == nil {
		return nil
	}

	addServerLog("[INFO] Stopping llama-server...")

	if err := cmd.Process.Signal(osInterrupt); err != nil {
		cmd.Process.Kill()
	}
	return nil
}

// ─── llama.cpp download trigger ──────────────────────────────────

func startLlamaCppDownload() {
	downloadMu.Lock()
	downloadState.Status = "fetching"
	downloadState.Paused = false
	downloadMu.Unlock()
	go downloadLlamaCpp()
}

// ─── HF Mirror download trigger ──────────────────────────────────

func startHFDownload(modelID string, files []string) error {
	// Validate the author segment of modelID (DestDir uses it in filepath.Join)
	// to prevent "../evil", ".", "..", or modelIDs containing path separators
	// from writing downloads outside LLM-Models (path traversal #1).
	parts := strings.SplitN(modelID, "/", 2)
	authorPart := parts[0]
	if authorPart == "" || authorPart == "." || authorPart == ".." ||
		strings.ContainsAny(authorPart, `\/`) {
		return fmt.Errorf("invalid modelID: %q", modelID)
	}

	// Validate each file name: the cleaned name must not be empty, start with
	// ".." (directory escape), or be an absolute path. Tasks uniformly use the
	// cleaned cleanName (#1).
	for _, fileName := range files {
		cleanName := filepath.Clean(fileName)
		if cleanName == "." || strings.HasPrefix(cleanName, "..") || filepath.IsAbs(cleanName) {
			return fmt.Errorf("invalid fileName: %q", fileName)
		}
	}

	// Validate the repo segment of modelID (DestDir uses it in filepath.Join),
	// same strategy as authorPart (#1 path traversal defense): missing repo
	// segment, empty repoPart, ".", "..", or containing \ / are all rejected.
	// Note that SplitN("a/b/c","/",2) yields repoPart "b/c", so "/" must be
	// rejected to avoid landing outside the target directory.
	if len(parts) < 2 {
		return fmt.Errorf("invalid modelID: %q", modelID)
	}
	repoPart := parts[1]
	if repoPart == "" || repoPart == "." || repoPart == ".." ||
		strings.ContainsAny(repoPart, `\/`) {
		return fmt.Errorf("invalid modelID: %q", modelID)
	}

	// The active download source determines URL construction: hf uses the
	// hf-mirror resolve endpoint, modelscope uses the legacy repo endpoint;
	// the task records Source so the queue can rebuild URLs on restore.
	source := activeDownloadSource()

	// Pre-validate source and pre-build one URL before enqueuing (#B2):
	// buildModelDownloadURL only returns error for unknown sources (defense in
	// depth; activeDownloadSource is already allowlisted). Probing once before
	// any task is enqueued means failure returns a single error, leaving no
	// half-enqueued state and no need to roll back previously enqueued tasks.
	// For valid sources, per-file URL construction never fails; error branches
	// inside the loop are defensive only.
	if _, err := buildModelDownloadURL(source, modelID, "probe.gguf"); err != nil {
		return err
	}

	dlTasksMu.Lock()
	for _, fileName := range files {
		cleanName := filepath.Clean(fileName)
		url, err := buildModelDownloadURL(source, modelID, cleanName)
		if err != nil {
			// Defensive branch: URL build failure is treated as a global error;
			// already-enqueued tasks remain as-is (#B2). Unlock before
			// returning (must not return while holding dlTasksMu; see #B1
			// explanation below).
			dlTasksMu.Unlock()
			return err
		}

		dlTaskCounter++
		id := fmt.Sprintf("dl-%d", dlTaskCounter)
		task := &DlTask{
			ID:       id,
			ModelID:  modelID,
			FileName: cleanName,
			DestDir:  filepath.Join(effectiveModelDownloadDir(), authorPart, repoPart),
			Source:   source,
			URL:      url,
			Status:   "queued",
			resumeCh: make(chan struct{}, 1),
		}
		task.ctx, task.cancel = context.WithCancel(context.Background())
		dlTasks = append(dlTasks, task)
		go downloadTask(task)
	}
	// Unlock before persisting the queue (#B1): persistTasksNow → saveConfig
	// acquires dlTasksMu again at the end to take a snapshot. If dlTasksMu is
	// still held here (e.g. inside a defer Unlock scope), it deadlocks with
	// itself — this was the root cause of a previous go test 600s timeout
	// stuck on dlTasksMu.Lock().
	dlTasksMu.Unlock()
	persistTasksNow()
	return nil
}
