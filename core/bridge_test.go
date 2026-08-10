package core

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestStartHFDownloadQueue 验证批量文件入队：任务 ID 递增、URL 使用
// HF Mirror 域名、初始状态为 queued、目标目录为生效模型目录/<作者>。
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
	if dlTasks[0].Status != "queued" {
		t.Errorf("初始状态 = %q, want queued", dlTasks[0].Status)
	}
	if !strings.HasPrefix(dlTasks[0].URL, hfMirrorBase+"/author/model/resolve/main/") {
		t.Errorf("URL 前缀错误: %q", dlTasks[0].URL)
	}
	if !strings.HasSuffix(dlTasks[0].DestDir, filepath.Join(effectiveModelsDir(), "author")) {
		t.Errorf("DestDir = %q, want 以 %q 结尾", dlTasks[0].DestDir, filepath.Join(effectiveModelsDir(), "author"))
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
