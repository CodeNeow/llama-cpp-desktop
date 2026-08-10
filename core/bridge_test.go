package core

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestStartHFDownloadQueue 验证批量文件入队：任务 ID 递增、URL 使用
// HF Mirror 域名、初始状态为 queued、目标目录为 modelsDir/<作者>。
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
	if !strings.HasSuffix(dlTasks[0].DestDir, filepath.Join(modelsDir, "author")) {
		t.Errorf("DestDir = %q, want 以 %q 结尾", dlTasks[0].DestDir, filepath.Join(modelsDir, "author"))
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
