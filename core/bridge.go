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

	// 在 serverMu 锁内创建命令并绑定日志输出（#3）。这里先不置
	// serverRunning=true：必须在 Start() 成功之后才置位，保证
	// 「serverRunning==true ⟹ serverCmd.Process 非 nil」这一不变量。
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
		serverMu.Unlock()
		addServerLog("[ERROR] Failed to start: " + err.Error())
		return err
	}

	serverMu.Lock()
	serverCmd = cmd
	serverRunning = true
	serverMu.Unlock()

	go func(cmd *exec.Cmd) {
		err := cmd.Wait()
		serverMu.Lock()
		serverRunning = false
		// 仅当全局仍指向本命令时清理，避免覆盖新启动的实例
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
	// 在 serverMu 锁内读取 running/cmd 局部副本，锁外对副本操作（#3），
	// 避免 stopServerInternal 与 start/goroutine 并发访问 serverCmd/Process。
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
	// 校验 modelID 的 author 部分（DestDir 会以它做 filepath.Join），
	// 防止 "../evil"、"."、".." 或含路径分隔符的 modelID 把下载目标
	// 写到 LLM-Models 目录之外（路径遍历 #1）。
	authorPart := strings.SplitN(modelID, "/", 2)[0]
	if authorPart == "" || authorPart == "." || authorPart == ".." ||
		strings.ContainsAny(authorPart, `\/`) {
		return fmt.Errorf("invalid modelID: %q", modelID)
	}

	// 校验每个文件名：清理后的文件名不得为空、以 ".." 开头（目录逃逸）
	// 或为绝对路径。任务统一使用清理后的 cleanName（#1）。
	for _, fileName := range files {
		cleanName := filepath.Clean(fileName)
		if cleanName == "." || strings.HasPrefix(cleanName, "..") || filepath.IsAbs(cleanName) {
			return fmt.Errorf("invalid fileName: %q", fileName)
		}
	}

	dlTasksMu.Lock()
	defer dlTasksMu.Unlock()

	for _, fileName := range files {
		dlTaskCounter++
		id := fmt.Sprintf("dl-%d", dlTaskCounter)

		cleanName := filepath.Clean(fileName)
		task := &DlTask{
			ID:       id,
			ModelID:  modelID,
			FileName: cleanName,
			DestDir:  filepath.Join(modelsDir, authorPart),
			URL:      fmt.Sprintf("%s/%s/resolve/main/%s", hfMirrorBase, modelID, url.PathEscape(cleanName)),
			Status:   "queued",
			resumeCh: make(chan struct{}, 1),
		}
		task.ctx, task.cancel = context.WithCancel(context.Background())
		dlTasks = append(dlTasks, task)
		go downloadTask(task)
	}
	return nil
}
