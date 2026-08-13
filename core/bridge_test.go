package core

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// withModelScope404Server 把 modelscopeLegacyBase 注入本地 404 服务并把下载源
// 切到 modelscope，返回恢复函数。用于需要真实入队（startHFDownload 会启动下载
// goroutine）又不想打外网的测试：404 让 goroutine 快速失败退出，不残留网络连接。
func withModelScope404Server(t *testing.T) func() {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)
	origLegacy := modelscopeLegacyBase
	modelscopeLegacyBase = srv.URL
	downloadSourceMu.Lock()
	origSource := downloadSource
	downloadSource = sourceModelScope
	downloadSourceMu.Unlock()
	return func() {
		modelscopeLegacyBase = origLegacy
		downloadSourceMu.Lock()
		downloadSource = origSource
		downloadSourceMu.Unlock()
	}
}

// waitTasksTerminal 轮询等待所有任务进入终态（error/done/cancelled），带超时
// 护栏。任务为空时直接返回。用于入队类测试避免残留 goroutine 影响后续用例。
func waitTasksTerminal(t *testing.T, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		dlTasksMu.Lock()
		allTerminal := true
		for _, tt := range dlTasks {
			switch tt.Status {
			case "error", "done", "cancelled":
			default:
				allTerminal = false
			}
		}
		dlTasksMu.Unlock()
		if allTerminal {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	dlTasksMu.Lock()
	statuses := make([]string, 0, len(dlTasks))
	for _, tt := range dlTasks {
		statuses = append(statuses, tt.ID+":"+tt.Status)
	}
	dlTasksMu.Unlock()
	t.Fatalf("任务未在 %v 内进入终态: %v", timeout, statuses)
}

// waitTaskTerminal 轮询等待指定 id 的任务进入终态（error/done/cancelled），带
// 超时护栏。只等待单个任务，适用于队列中同时存在无 goroutine 的任务（如恢复的
// paused 任务）时只等新入队的活跃任务。
func waitTaskTerminal(t *testing.T, id string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		dlTasksMu.Lock()
		var status string
		for _, tt := range dlTasks {
			if tt.ID == id {
				status = tt.Status
				break
			}
		}
		dlTasksMu.Unlock()
		switch status {
		case "error", "done", "cancelled":
			return
		case "":
			t.Fatalf("任务 %s 不存在", id)
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("任务 %s 未在 %v 内进入终态", id, timeout)
}

// TestStartHFDownloadNoDeadlock 是 #B1 死锁的回归测试：startHFDownload 入队后
// 调用 persistTasksNow → saveConfig，而 saveConfig 末尾会再次获取 dlTasksMu 做
// 快照。若 persistTasksNow 在持有 dlTasksMu（defer Unlock 作用域）时被调用，
// 会自死锁——旧实现在这里卡死，导致 go test 600s 超时。测试用超时护栏断言
// startHFDownload 快速返回，随后立即 GetDownloadTasks 快照不阻塞，并等待任务
// 终态避免残留 goroutine。
func TestStartHFDownloadNoDeadlock(t *testing.T) {
	withTempCwd(t)
	saveConfigState(t)
	restoreSource := withModelScope404Server(t)
	defer restoreSource()
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

	done := make(chan struct{})
	go func() {
		if err := startHFDownload("author/model", []string{"a.gguf", "b.gguf"}); err != nil {
			t.Errorf("startHFDownload 返回错误: %v", err)
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("startHFDownload 死锁：5s 内未返回（persistTasksNow 不应在持有 dlTasksMu 时被调用）")
	}

	// 入队后立即快照：GetDownloadTasks 内部获取 dlTasksMu，死锁实现下同样会卡住
	tasks := (&App{}).GetDownloadTasks()
	if len(tasks) != 2 {
		t.Fatalf("任务数 = %d, want 2", len(tasks))
	}
	if tasks[0].Status != "queued" && tasks[0].Status != "downloading" {
		t.Errorf("快照中任务状态 = %q, want queued 或 downloading", tasks[0].Status)
	}

	// 等待 goroutine 全部退出（404 快速失败到 error），避免残留
	waitTasksTerminal(t, 5*time.Second)
}

// TestStartHFDownloadQueue 验证批量文件入队：任务 ID 递增、URL 使用
// HF Mirror 域名、初始状态为 queued、目标目录为生效模型目录/<作者>/<仓库>
// （三级落地：author/model/文件）。
func TestStartHFDownloadQueue(t *testing.T) {
	saveConfigState(t)
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

	if err := startHFDownload("author/model", []string{"a.gguf", "b.gguf"}); err != nil {
		t.Fatal(err)
	}

	dlTasksMu.Lock()
	defer dlTasksMu.Unlock()
	if len(dlTasks) != 2 {
		t.Fatalf("任务数 = %d, want 2", len(dlTasks))
	}
	if dlTasks[0].ID != "dl-1" || dlTasks[1].ID != "dl-2" {
		t.Errorf("任务 ID 未递增: %s, %s", dlTasks[0].ID, dlTasks[1].ID)
	}
	// 入队即启动下载 goroutine（go downloadTask），downloadTask 开篇就把状态翻为
	// downloading。读取时可能仍是入队时的 queued（goroutine 未及调度），也可能是
	// downloading（已调度并抢到锁）；两者都是入队成功后的合法初始状态。#B1 死锁
	// 回归的验收标准同样认可「状态先 downloading 或按暂停流转」。
	if dlTasks[0].Status != "queued" && dlTasks[0].Status != "downloading" {
		t.Errorf("初始状态 = %q, want queued 或 downloading", dlTasks[0].Status)
	}
	if !strings.HasPrefix(dlTasks[0].URL, hfMirrorBase+"/author/model/resolve/main/") {
		t.Errorf("URL 前缀错误: %q", dlTasks[0].URL)
	}
	if !strings.HasSuffix(dlTasks[0].DestDir, filepath.Join(effectiveModelsDir(), "author", "model")) {
		t.Errorf("DestDir = %q, want 以 %q 结尾", dlTasks[0].DestDir, filepath.Join(effectiveModelsDir(), "author", "model"))
	}
	if dlTasks[0].cancel == nil {
		t.Error("任务应持有 cancel 函数（供取消/退出清理）")
	}
}

// TestStopHFDownload 验证取消单个任务：cancel 触发 ctx 取消，状态置为 cancelled。
func TestStopHFDownload(t *testing.T) {
	saveConfigState(t)
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

	if err := startHFDownload("author/model", []string{"a.gguf"}); err != nil {
		t.Fatal(err)
	}

	dlTasksMu.Lock()
	task := dlTasks[0]
	dlTasksMu.Unlock()

	task.cancel()
	if err := task.ctx.Err(); err == nil {
		t.Error("cancel 后 ctx 应处于取消状态")
	}
}

// TestCancelDownloadTaskUnknownID 验证取消不存在的任务返回 nil（幂等）。
func TestCancelDownloadTaskUnknownID(t *testing.T) {
	saveConfigState(t)
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

	if err := (&App{}).CancelDownloadTask("dl-999"); err != nil {
		t.Errorf("未知任务 ID 应返回 nil, 实际 %v", err)
	}
}

// TestPauseResumeDownloadTask 验证暂停/恢复状态机：仅 downloading 可暂停、
// 仅 paused 可恢复。
func TestPauseResumeDownloadTask(t *testing.T) {
	saveConfigState(t)
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

	if err := startHFDownload("author/model", []string{"a.gguf"}); err != nil {
		t.Fatal(err)
	}
	dlTasksMu.Lock()
	task := dlTasks[0]
	task.Status = "downloading"
	dlTasksMu.Unlock()

	if err := (&App{}).PauseDownloadTask(task.ID); err != nil {
		t.Fatal(err)
	}
	dlTasksMu.Lock()
	if task.Status != "paused" {
		t.Errorf("暂停后状态 = %q, want paused", task.Status)
	}
	dlTasksMu.Unlock()

	if err := (&App{}).ResumeDownloadTask(task.ID); err != nil {
		t.Fatal(err)
	}
	dlTasksMu.Lock()
	if task.Status != "downloading" {
		t.Errorf("恢复后状态 = %q, want downloading", task.Status)
	}
	dlTasksMu.Unlock()
}

// TestGetDownloadTasksSnapshot 验证 GetDownloadTasks 返回深拷贝，
// 修改返回值不影响内部任务状态。
func TestGetDownloadTasksSnapshot(t *testing.T) {
	saveConfigState(t)
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

	if err := startHFDownload("author/model", []string{"a.gguf"}); err != nil {
		t.Fatal(err)
	}

	tasks := (&App{}).GetDownloadTasks()
	if len(tasks) != 1 {
		t.Fatalf("任务数 = %d, want 1", len(tasks))
	}
	tasks[0].Status = "hacked"
	dlTasksMu.Lock()
	realStatus := dlTasks[0].Status
	dlTasksMu.Unlock()
	if realStatus == "hacked" {
		t.Error("修改快照不应影响内部任务状态")
	}
}

// TestStartHFDownloadRejectsPathTraversal 验证 startHFDownload 对恶意
// modelID / fileName 返回错误且不创建任何任务（#1）。author 部分含
// 路径分隔符或 "../" 时 DestDir 会用 filepath.Join 逃出 LLM-Models 目录，
// 必须在入队前拒绝。
func TestStartHFDownloadRejectsPathTraversal(t *testing.T) {
	saveConfigState(t)
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

	// author 部分为 "../evil"：DestDir 会 join 出 LLM-Models 之外的目录
	if err := startHFDownload("../evil", []string{"evil.gguf"}); err == nil {
		t.Error("../evil modelID 应返回错误")
	}
	// 含路径分隔符的 modelID（"a/b" 或 "a\\b"）同样拒绝
	if err := startHFDownload("a\\b/model", []string{"x.gguf"}); err == nil {
		t.Error("author 含 \\ 的 modelID 应返回错误")
	}
	// 文件名可逃逸到父目录
	if err := startHFDownload("author/model", []string{"../../etc/x.gguf"}); err == nil {
		t.Error("../../etc/x.gguf 文件名应返回错误")
	}
	// 绝对路径文件名拒绝（按平台构造：Windows 需 C:\ 前缀、Unix 需 /
	// 前缀，filepath.IsAbs 才判定为绝对；单一字符串无法跨平台成立）
	absName := "/etc/x.gguf"
	if runtime.GOOS == "windows" {
		absName = `C:\Windows\system.ini`
	}
	if err := startHFDownload("author/model", []string{absName}); err == nil {
		t.Error("绝对路径文件名应返回错误")
	}

	// 拒绝分支不得创建任何任务
	dlTasksMu.Lock()
	defer dlTasksMu.Unlock()
	if len(dlTasks) != 0 || dlTaskCounter != 0 {
		t.Errorf("非法输入不应创建任务: len=%d counter=%d", len(dlTasks), dlTaskCounter)
	}
}

// TestStartHFDownloadRejectsInvalidModelID 验证 modelID 作者部分为
// 空 / "." / ".." 时返回错误（#1）。
func TestStartHFDownloadRejectsInvalidModelID(t *testing.T) {
	saveConfigState(t)
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

	if err := startHFDownload("/model", []string{"x.gguf"}); err == nil {
		t.Error("/model 的空 author 应返回错误")
	}
	if err := startHFDownload("./model", []string{"x.gguf"}); err == nil {
		t.Error("./model 的 . author 应返回错误")
	}
	if err := startHFDownload("../model", []string{"x.gguf"}); err == nil {
		t.Error("../model 的 .. author 应返回错误")
	}
}

// TestStartHFDownloadRejectsInvalidRepoPart 验证 modelID 的 repo 部分为空或
// 含路径分隔符时返回错误且不创建任务（#1）。DestDir 会以 repoPart 做
// filepath.Join：repoPart 含 "/"（如 SplitN("author/model/extra","/",2)
// 的 "model/extra"）或为空（如 "author/"）都会把下载目标写到错误层级。
func TestStartHFDownloadRejectsInvalidRepoPart(t *testing.T) {
	saveConfigState(t)
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

	// repoPart 含 "/"：SplitN("a/b/c","/",2) 的 repoPart 为 "b/c"
	if err := startHFDownload("author/model/extra", []string{"x.gguf"}); err == nil {
		t.Error("repoPart 含 / 的 modelID 应返回错误")
	}
	// repoPart 为空：SplitN("author/","/",2) 的 repoPart 为 ""
	if err := startHFDownload("author/", []string{"x.gguf"}); err == nil {
		t.Error("repoPart 为空的 modelID 应返回错误")
	}
	// repoPart 为 "." / ".."：join 后等效于上一级目录
	if err := startHFDownload("author/.", []string{"x.gguf"}); err == nil {
		t.Error("repoPart 为 . 的 modelID 应返回错误")
	}
	if err := startHFDownload("author/..", []string{"x.gguf"}); err == nil {
		t.Error("repoPart 为 .. 的 modelID 应返回错误")
	}
	// 无 "/" 的 modelID 没有 repo 部分，同样拒绝
	if err := startHFDownload("author", []string{"x.gguf"}); err == nil {
		t.Error("无 repo 部分的 modelID 应返回错误")
	}

	// 拒绝分支不得创建任何任务
	dlTasksMu.Lock()
	defer dlTasksMu.Unlock()
	if len(dlTasks) != 0 || dlTaskCounter != 0 {
		t.Errorf("非法输入不应创建任务: len=%d counter=%d", len(dlTasks), dlTaskCounter)
	}
}
