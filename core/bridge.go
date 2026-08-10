package core

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
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
		return fmt.Errorf("生成模型预设失败: %w", err)
	}

	// Build command
	llamaServer, args := buildServerCommand(cfg, presetPath)

	serverLogsMu.Lock()
	serverCmd = exec.Command(llamaServer, args...)
	hideWindow(serverCmd)
	serverCmd.Stdout = &serverLogWriter{}
	serverCmd.Stderr = &serverLogWriter{}
	serverLogs = []string{}
	serverRunning = true
	serverLogsMu.Unlock()

	addServerLog(fmt.Sprintf("[INFO] Starting llama-server: %s %s", llamaServer, strings.Join(args, " ")))

	if err := serverCmd.Start(); err != nil {
		serverLogsMu.Lock()
		serverRunning = false
		serverLogsMu.Unlock()
		addServerLog("[ERROR] Failed to start: " + err.Error())
		return err
	}

	go func() {
		err := serverCmd.Wait()
		serverLogsMu.Lock()
		serverRunning = false
		serverLogsMu.Unlock()
		if err != nil {
			addServerLog("[WARN] llama-server exited: " + err.Error())
		} else {
			addServerLog("[INFO] llama-server stopped")
		}
	}()

	return nil
}

// buildServerCommand resolves the llama-server binary (custom dir when the
// binary is not on PATH) and builds its argument list from the server config.
// The preset path points at the generated models INI file.
func buildServerCommand(cfg ServerConfig, presetPath string) (string, []string) {
	llamaServer := "llama-server"
	if _, err := exec.LookPath(llamaServer); err != nil {
		customLlamaCppMu.Lock()
		customDir := customLlamaCppDir
		customLlamaCppMu.Unlock()
		if customDir != "" {
			candidate := filepath.Join(customDir, "llama-server")
			if runtime.GOOS == "windows" {
				candidate += ".exe"
			}
			if _, err := os.Stat(candidate); err == nil {
				llamaServer = candidate
			}
		}
	}

	args := []string{
		"--host", cfg.Host,
		"--port", strconv.Itoa(cfg.Port),
		"--models-dir", modelsDir,
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
	serverLogsMu.Lock()
	if !serverRunning || serverCmd == nil {
		serverLogsMu.Unlock()
		return nil
	}
	serverLogsMu.Unlock()

	addServerLog("[INFO] Stopping llama-server...")

	if err := serverCmd.Process.Signal(osInterrupt); err != nil {
		serverCmd.Process.Kill()
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
	dlTasksMu.Lock()
	defer dlTasksMu.Unlock()

	for _, fileName := range files {
		dlTaskCounter++
		id := fmt.Sprintf("dl-%d", dlTaskCounter)

		task := &DlTask{
			ID:       id,
			ModelID:  modelID,
			FileName: fileName,
			DestDir:  filepath.Join(modelsDir, strings.SplitN(modelID, "/", 2)[0]),
			URL:      fmt.Sprintf("%s/%s/resolve/main/%s", hfMirrorBase, modelID, url.PathEscape(fileName)),
			Status:   "queued",
			pauseCh:  make(chan struct{}, 1),
			resumeCh: make(chan struct{}, 1),
		}
		task.ctx, task.cancel = context.WithCancel(context.Background())
		dlTasks = append(dlTasks, task)
		go downloadTask(task)
	}
	return nil
}
