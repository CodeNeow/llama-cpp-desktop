package core

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

// TestCancelDownloadTaskMarksErrorTaskCancelled verifies that calling CancelDownloadTask on an error-terminal task
// immediately sets Status to "cancelled" (bug root cause: error-task goroutine has already exited; the old implementation
// only called cancel() with no visible effect, so the UI stayed on "download failed" and cancel was unresponsive).
// After the fix, the lock sets Status="cancelled" before cancel(), so the frontend polling becomes visible immediately.
// The task is directly constructed as an error-terminal task simulating a dead goroutine; the downloadTask goroutine is not started.
func TestCancelDownloadTaskMarksErrorTaskCancelled(t *testing.T) {
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

	task := &DlTask{
		ID:       "dl-1",
		ModelID:  "author/model",
		FileName: "model.gguf",
		Status:   "error",
		Error:    "stream error: stream ID 1; CANCEL; received from peer",
	}
	task.ctx, task.cancel = context.WithCancel(context.Background())
	dlTasks = append(dlTasks, task)

	app := &App{}
	if err := app.CancelDownloadTask("dl-1"); err != nil {
		t.Fatalf("CancelDownloadTask 返回错误: %v", err)
	}

	dlTasksMu.Lock()
	status := task.Status
	dlTasksMu.Unlock()
	if status != "cancelled" {
		t.Errorf("error 任务取消后状态 = %q, want cancelled（锁内置状态应立即生效）", status)
	}
	if task.ctx.Err() == nil {
		t.Error("取消后 ctx 应已结束")
	}

	// unknown id returns nil silently, consistent with existing semantics
	if err := app.CancelDownloadTask("dl-999"); err != nil {
		t.Errorf("未知 id 取消应返回 nil, got %v", err)
	}
}

// TestRetryDownloadTaskCompletesAfterError verifies that calling RetryDownloadTask on an error-terminal task
// reconstructs ctx, clears the error, restores Status to queued, and restarts the downloadTask goroutine;
// the .part resume transfer is automatically effective; httptest returns fixed content, the task eventually reaches
// done and the file is written to disk. It also verifies that a task in downloading state cannot be retried
// (an active goroutine exists, preventing concurrent writes to the same .part file) and that unknown ids return nil silently.
func TestRetryDownloadTaskCompletesAfterError(t *testing.T) {
	withTempCwd(t)
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

	payload := []byte("fake model bytes for retry")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", strconv.Itoa(len(payload)))
		w.Write(payload)
	}))
	defer srv.Close()

	// construct error-terminal task (simulating download failure with goroutine exited)
	task := &DlTask{
		ID:       "dl-1",
		ModelID:  "author/model",
		FileName: "model.gguf",
		DestDir:  filepath.Join(effectiveModelsDir(), "author"),
		URL:      srv.URL,
		Status:   "error",
		Error:    "stream error: stream ID 1; CANCEL; received from peer",
	}
	task.ctx, task.cancel = context.WithCancel(context.Background())
	dlTasks = append(dlTasks, task)

	app := &App{}
	if err := app.RetryDownloadTask("dl-1"); err != nil {
		t.Fatalf("RetryDownloadTask 返回错误: %v", err)
	}

	// immediate check: task should leave error-terminal state (queued if goroutine hasn't started yet,
	// downloading if it has)
	dlTasksMu.Lock()
	status := task.Status
	errMsg := task.Error
	dlTasksMu.Unlock()
	if status != "queued" && status != "downloading" {
		t.Fatalf("重试后状态 = %q, want queued 或 downloading", status)
	}
	if errMsg != "" {
		t.Errorf("重试后应清空旧错误信息: %q", errMsg)
	}

	// poll until done
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		dlTasksMu.Lock()
		status = task.Status
		dlTasksMu.Unlock()
		if status == "done" {
			break
		}
		if status == "error" {
			dlTasksMu.Lock()
			e := task.Error
			dlTasksMu.Unlock()
			t.Fatalf("重试下载失败: %q", e)
		}
		time.Sleep(10 * time.Millisecond)
	}
	dlTasksMu.Lock()
	status = task.Status
	dlTasksMu.Unlock()
	if status != "done" {
		t.Fatalf("重试后任务未完成, 状态 = %q", status)
	}

	got, err := os.ReadFile(filepath.Join(effectiveModelsDir(), "author", "model.gguf"))
	if err != nil {
		t.Fatalf("下载文件未落盘: %v", err)
	}
	if string(got) != string(payload) {
		t.Errorf("文件内容 = %q, want %q", got, payload)
	}

	// tasks in downloading state are not allowed to retry (active goroutine exists)
	active := &DlTask{
		ID:       "dl-2",
		ModelID:  "author/model",
		FileName: "active.gguf",
		DestDir:  filepath.Join(effectiveModelsDir(), "author"),
		URL:      srv.URL,
		Status:   "downloading",
	}
	active.ctx, active.cancel = context.WithCancel(context.Background())
	dlTasks = append(dlTasks, active)
	if err := app.RetryDownloadTask("dl-2"); err != nil {
		t.Fatalf("downloading 任务重试应静默返回 nil, got %v", err)
	}
	dlTasksMu.Lock()
	status = active.Status
	dlTasksMu.Unlock()
	if status != "downloading" {
		t.Errorf("downloading 任务重试后状态 = %q, want 保持 downloading", status)
	}

	// unknown id silently returns nil, consistent with CancelDownloadTask semantics
	if err := app.RetryDownloadTask("dl-999"); err != nil {
		t.Errorf("未知 id 重试应返回 nil, got %v", err)
	}
}
