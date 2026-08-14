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
		serverStartTime = time.Time{}
		serverMu.Unlock()
		addServerLog("[ERROR] Failed to start: " + err.Error())
		return err
	}

	serverMu.Lock()
	serverCmd = cmd
	serverRunning = true
	serverStartTime = time.Now()
	serverMu.Unlock()

	go func(cmd *exec.Cmd) {
		err := cmd.Wait()
		serverMu.Lock()
		serverRunning = false
		serverStartTime = time.Time{}
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

// buildServerCommand resolves the llama-server binary (custom dir, then the
// llama-cpp/ download dir, then PATH) and builds its argument list from the
// server config. The preset path points at the generated models INI file.
func buildServerCommand(cfg ServerConfig, presetPath string) (string, []string) {
	// 与 getLlamaCppInfo 共用 resolveLlamaServerBin，保证两处对 llama.cpp
	// 安装位置的解析一致（下载目录解压后即可启动服务）；未命中时回退到
	// 裸二进制名，由 exec.Command 启动时给出错误提示。
	llamaServer := resolveLlamaServerBin()
	if llamaServer == "" {
		llamaServer = "llama-server"
	}

	args := []string{
		"--host", cfg.Host,
		"--port", strconv.Itoa(cfg.Port),
		"--models-dir", effectiveModelsDir(),
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
	parts := strings.SplitN(modelID, "/", 2)
	authorPart := parts[0]
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

	// 校验 modelID 的 repo 部分（DestDir 会以它做 filepath.Join），与
	// authorPart 同策略（#1 路径遍历防御）：无 repo 部分、repoPart 为空、
	// "."、".." 或含 \ / 任一者拒绝。注意 SplitN("a/b/c","/",2) 的
	// repoPart 为 "b/c"，含 "/" 必须被拒，避免落到目标目录之外。
	if len(parts) < 2 {
		return fmt.Errorf("invalid modelID: %q", modelID)
	}
	repoPart := parts[1]
	if repoPart == "" || repoPart == "." || repoPart == ".." ||
		strings.ContainsAny(repoPart, `\/`) {
		return fmt.Errorf("invalid modelID: %q", modelID)
	}

	// 当前下载源决定 URL 构建方式：hf 走 hf-mirror resolve 端点，modelscope
	// 走 legacy repo 端点；任务记录 Source 供队列持久化恢复时重建 URL。
	source := activeDownloadSource()

	// 入队前预校验 source 并预构建一次 URL（#B2）：buildModelDownloadURL 仅在
	// 未知 source 下返回错误（防御纵深，activeDownloadSource 已被白名单约束）。
	// 在入队任何任务之前一次性探测，失败即整体返回错误，不留半入队状态，也
	// 不存在「弹出上一个已入队任务」的误回滚。合法 source 下各文件 URL 构建
	// 不会失败，循环内的错误分支仅作防御。
	if _, err := buildModelDownloadURL(source, modelID, "probe.gguf"); err != nil {
		return err
	}

	dlTasksMu.Lock()
	for _, fileName := range files {
		cleanName := filepath.Clean(fileName)
		url, err := buildModelDownloadURL(source, modelID, cleanName)
		if err != nil {
			// 防御分支：URL 构建失败视为整体错误，已入队任务保持原样（#B2），
			// 先解锁再返回（不能持有 dlTasksMu 返回，见下方 #B1 说明）。
			dlTasksMu.Unlock()
			return err
		}

		dlTaskCounter++
		id := fmt.Sprintf("dl-%d", dlTaskCounter)
		task := &DlTask{
			ID:       id,
			ModelID:  modelID,
			FileName: cleanName,
			DestDir:  filepath.Join(effectiveModelsDir(), authorPart, repoPart),
			Source:   source,
			URL:      url,
			Status:   "queued",
			resumeCh: make(chan struct{}, 1),
		}
		task.ctx, task.cancel = context.WithCancel(context.Background())
		dlTasks = append(dlTasks, task)
		go downloadTask(task)
	}
	// 入队完成后先解锁再持久化队列（#B1）：persistTasksNow → saveConfig 末尾会
	// 再次获取 dlTasksMu 做快照。若此处仍持有 dlTasksMu（如 defer Unlock 作用域
	// 内调用），将自死锁——这是此前 go test 600s 超时卡在 dlTasksMu.Lock() 的根因。
	dlTasksMu.Unlock()
	persistTasksNow()
	return nil
}
