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

// TestCancelDownloadTaskMarksErrorTaskCancelled 验证对 error 终态任务调用
// CancelDownloadTask 立即置 cancelled（bug 根因：error 任务 goroutine 已退出，
// 旧实现只调 cancel() 无任何效果，UI 永远停留在「下载失败」且取消无反应）。
// 修复后锁内先置 Status="cancelled" 再 cancel()，前端轮询立即可见。任务直接
// 构造为 error 终态模拟 goroutine 已死的任务，不启动 downloadTask goroutine。
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

	// 找不到 id 时静默返回 nil，与既有语义一致
	if err := app.CancelDownloadTask("dl-999"); err != nil {
		t.Errorf("未知 id 取消应返回 nil, got %v", err)
	}
}

// TestRetryDownloadTaskCompletesAfterError 验证对 error 终态任务调用
// RetryDownloadTask：重建 ctx、清空错误、Status 恢复为 queued 并重新启动
// downloadTask goroutine，.part 断点续传自动生效；httptest 返回固定内容，
// 任务最终 done 且文件落盘。同时验证 downloading 中的任务不允许重试
// （存在活跃 goroutine，避免并发写同一 .part 文件）与未知 id 静默返回 nil。
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

	// 构造 error 终态任务（模拟下载失败、goroutine 已退出）
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

	// 立即检查：任务应离开 error 终态（goroutine 未及启动时为 queued，
	// 已启动则为 downloading）
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

	// 轮询等待 done
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

	// downloading 中的任务不允许重试（存在活跃 goroutine）
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

	// 未知 id 静默返回 nil，与 CancelDownloadTask 语义一致
	if err := app.RetryDownloadTask("dl-999"); err != nil {
		t.Errorf("未知 id 重试应返回 nil, got %v", err)
	}
}
