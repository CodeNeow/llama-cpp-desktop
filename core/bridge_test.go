package core

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// withModelScope404Server injects a local 404 server as modelscopeLegacyBase and
// switches the download source to modelscope, returning a restore function.
// Used when a test needs real queueing (startHFDownload spawns a download goroutine)
// but must avoid external network: the 404 response makes the goroutine fail fast
// without leaving open connections.
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

// waitTasksTerminal polls until all tasks reach a terminal state (error/done/cancelled),
// with a timeout guard. Returns immediately when no tasks exist. Used by queue tests
// to avoid stray goroutines polluting subsequent test cases.
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
	t.Fatalf("tasks did not reach terminal state in %v: %v", timeout, statuses)
}

// waitTaskTerminal polls until the task with the given id reaches a terminal state
// (error/done/cancelled), with a timeout guard. Waits for a single task only;
// useful when the queue also contains tasks with no goroutine (e.g. resumed paused
// tasks) and we only need to wait for the newly enqueued active one.
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
			t.Fatalf("task %s not found", id)
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("task %s did not reach terminal state in %v", id, timeout)
}

// cancelAllTasks cancels every non-terminal download task and waits for their
// goroutines to exit. Queue tests enqueue real download goroutines; without
// waiting, a goroutine can outlive its test and later call saveConfig (the
// configFile path is cwd-relative) from a subsequent test's temp cwd,
// truncating that test's config file and making its loadConfig read empty
// JSON. Must be called before the test's defer resets dlTasks.
func cancelAllTasks(t *testing.T) {
	t.Helper()
	dlTasksMu.Lock()
	for _, tt := range dlTasks {
		switch tt.Status {
		case "error", "done", "cancelled":
		default:
			if tt.cancel != nil {
				tt.cancel()
			}
		}
	}
	dlTasksMu.Unlock()
	waitTasksTerminal(t, 5*time.Second)
}

// TestStartHFDownloadNoDeadlock is a #B1 deadlock regression test: startHFDownload
// calls persistTasksNow → saveConfig after enqueue, and saveConfig re-acquires dlTasksMu
// at its tail for snapshotting. If persistTasksNow is called while dlTasksMu is held
// (inside a defer Unlock scope), it deadlocks on itself — the old implementation hung
// here, causing go test 600s timeouts. The test asserts startHFDownload returns fast,
// then GetDownloadTasks snapshots without blocking, and waits for task terminal state
// to avoid stray goroutines.
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
			t.Errorf("startHFDownload returned error: %v", err)
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("startHFDownload deadlocked: did not return within 5s (persistTasksNow should not be called while holding dlTasksMu)")
	}

	// Snapshot immediately after enqueue: GetDownloadTasks acquires dlTasksMu internally;
	// a deadlocked implementation would also hang here.
	tasks := (&App{}).GetDownloadTasks()
	if len(tasks) != 2 {
		t.Fatalf("task count = %d, want 2", len(tasks))
	}
	if tasks[0].Status != "queued" && tasks[0].Status != "downloading" {
		t.Errorf("snapshot status = %q, want queued or downloading", tasks[0].Status)
	}

	// Wait for goroutines to exit (404 fails fast to error state) to avoid leakage.
	waitTasksTerminal(t, 5*time.Second)
}

// TestStartHFDownloadQueue verifies batch file enqueue: IDs increment, URLs use
// HF Mirror domain, initial status is queued, and destination is effectiveModelDownloadDir/<author>/<repo>
// (three-level layout: author/model/file).
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
	if len(dlTasks) != 2 {
		t.Fatalf("task count = %d, want 2", len(dlTasks))
	}
	if dlTasks[0].ID != "dl-1" || dlTasks[1].ID != "dl-2" {
		t.Errorf("task IDs did not increment: %s, %s", dlTasks[0].ID, dlTasks[1].ID)
	}
	// Enqueue immediately spawns the download goroutine (go downloadTask), and the
	// downloadTask entry flips status to downloading. At read time the status may
	// still be queued (goroutine not yet scheduled) or already downloading (scheduled
	// and acquired the lock); both are valid initial states after enqueue. The #B1
	// deadlock regression also accepts "status becomes downloading first or flows through pause".
	if dlTasks[0].Status != "queued" && dlTasks[0].Status != "downloading" {
		t.Errorf("initial status = %q, want queued or downloading", dlTasks[0].Status)
	}
	if !strings.HasPrefix(dlTasks[0].URL, hfMirrorBase+"/author/model/resolve/main/") {
		t.Errorf("URL prefix wrong: %q", dlTasks[0].URL)
	}
	if !strings.HasSuffix(dlTasks[0].DestDir, filepath.Join(effectiveModelDownloadDir(), "author", "model")) {
		t.Errorf("DestDir = %q, want to end with %q", dlTasks[0].DestDir, filepath.Join(effectiveModelDownloadDir(), "author", "model"))
	}
	if dlTasks[0].cancel == nil {
		t.Error("task should hold cancel func (used by cancel / exit cleanup)")
	}
	dlTasksMu.Unlock()

	cancelAllTasks(t) // stop the real download goroutine before it outlives this test
}

// TestStopHFDownload verifies cancelling a single task: cancel triggers context
// cancellation and status becomes cancelled.
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
		t.Error("context should be cancelled after cancel()")
	}

	cancelAllTasks(t) // wait for the goroutine to actually exit
}

// TestCancelDownloadTaskUnknownID verifies cancelling a non-existent task returns nil (idempotent).
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
		t.Errorf("unknown task ID should return nil, got %v", err)
	}
}

// TestPauseResumeDownloadTask verifies pause/resume state machine: only downloading
// can be paused; only paused can be resumed.
func TestPauseResumeDownloadTask(t *testing.T) {
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

	if err := startHFDownload("author/model", []string{"a.gguf"}); err != nil {
		t.Fatal(err)
	}
	dlTasksMu.Lock()
	id := dlTasks[0].ID
	dlTasksMu.Unlock()
	// Must wait for download goroutine to exit (local 404 fails fast to error state)
	// before manually mutating status: downloadTask unconditionally flips status to
	// downloading at entry (engine.go downloadTask prologue). While the goroutine is
	// alive, setting downloading → pause may be overwritten back to downloading by the
	// goroutine's entry write landing after Pause — CI full-load runners have hit this
	// intermittent failure. Production paths have no such interleaving (downloading is
	// only set by the goroutine itself; Pause on queued is a no-op); this is a race
	// introduced by test-forged state.
	waitTaskTerminal(t, id, 5*time.Second)

	dlTasksMu.Lock()
	task := dlTasks[0]
	task.Status = "downloading"
	dlTasksMu.Unlock()

	if err := (&App{}).PauseDownloadTask(task.ID); err != nil {
		t.Fatal(err)
	}
	dlTasksMu.Lock()
	if task.Status != "paused" {
		t.Errorf("status after pause = %q, want paused", task.Status)
	}
	dlTasksMu.Unlock()

	if err := (&App{}).ResumeDownloadTask(task.ID); err != nil {
		t.Fatal(err)
	}
	dlTasksMu.Lock()
	if task.Status != "downloading" {
		t.Errorf("status after resume = %q, want downloading", task.Status)
	}
	dlTasksMu.Unlock()
}

// TestGetDownloadTasksSnapshot verifies GetDownloadTasks returns a deep copy:
// mutating the returned slice must not affect internal task state.
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
		t.Fatalf("task count = %d, want 1", len(tasks))
	}
	tasks[0].Status = "hacked"
	dlTasksMu.Lock()
	realStatus := dlTasks[0].Status
	dlTasksMu.Unlock()
	if realStatus == "hacked" {
		t.Error("mutating snapshot must not affect internal task state")
	}

	cancelAllTasks(t) // stop the real download goroutine before it outlives this test
}

// TestStartHFDownloadRejectsPathTraversal verifies startHFDownload returns an error
// for malicious modelID / fileName and creates no tasks (#1). When the author segment
// contains a path separator or "../", DestDir escapes LLM-Models via filepath.Join,
// so rejection must happen before enqueue.
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

	// author segment is "../evil": DestDir joins outside LLM-Models
	if err := startHFDownload("../evil", []string{"evil.gguf"}); err == nil {
		t.Error("../evil modelID should return error")
	}
	// modelID containing path separator ("a/b" or "a\\b") is also rejected
	if err := startHFDownload("a\\b/model", []string{"x.gguf"}); err == nil {
		t.Error("modelID with \\ in author should return error")
	}
	// filename can escape to parent directory
	if err := startHFDownload("author/model", []string{"../../etc/x.gguf"}); err == nil {
		t.Error("../../etc/x.gguf filename should return error")
	}
	// absolute-path filenames are rejected (platform-dependent: Windows needs C:\ prefix,
	// Unix needs / prefix for filepath.IsAbs; no single string works cross-platform)
	absName := "/etc/x.gguf"
	if runtime.GOOS == "windows" {
		absName = `C:\Windows\system.ini`
	}
	if err := startHFDownload("author/model", []string{absName}); err == nil {
		t.Error("absolute-path filename should return error")
	}

	// rejection branch must create zero tasks
	dlTasksMu.Lock()
	defer dlTasksMu.Unlock()
	if len(dlTasks) != 0 || dlTaskCounter != 0 {
		t.Errorf("invalid input must not create tasks: len=%d counter=%d", len(dlTasks), dlTaskCounter)
	}
}

// TestStartHFDownloadRejectsInvalidModelID verifies empty / "." / ".." author parts
// return an error (#1).
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
		t.Error("/model with empty author should return error")
	}
	if err := startHFDownload("./model", []string{"x.gguf"}); err == nil {
		t.Error("./model with . author should return error")
	}
	if err := startHFDownload("../model", []string{"x.gguf"}); err == nil {
		t.Error("../model with .. author should return error")
	}
}

// TestStartHFDownloadRejectsInvalidRepoPart verifies empty or path-separator-containing
// repo parts return an error without creating tasks (#1). DestDir uses repoPart in
// filepath.Join: repoPart containing "/" (e.g. SplitN("author/model/extra","/",2) → "model/extra")
// or empty (e.g. "author/") both write the download target to a wrong directory level.
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

	// repoPart contains "/": SplitN("a/b/c","/",2) → repoPart = "b/c"
	if err := startHFDownload("author/model/extra", []string{"x.gguf"}); err == nil {
		t.Error("modelID with / in repoPart should return error")
	}
	// repoPart is empty: SplitN("author/","/",2) → repoPart = ""
	if err := startHFDownload("author/", []string{"x.gguf"}); err == nil {
		t.Error("modelID with empty repoPart should return error")
	}
	// repoPart is "." / "..": equivalent to parent directory after join
	if err := startHFDownload("author/.", []string{"x.gguf"}); err == nil {
		t.Error("modelID with . repoPart should return error")
	}
	if err := startHFDownload("author/..", []string{"x.gguf"}); err == nil {
		t.Error("modelID with .. repoPart should return error")
	}
	// modelID with no "/" has no repo part; also rejected
	if err := startHFDownload("author", []string{"x.gguf"}); err == nil {
		t.Error("modelID without repoPart should return error")
	}

	// rejection branch must create zero tasks
	dlTasksMu.Lock()
	defer dlTasksMu.Unlock()
	if len(dlTasks) != 0 || dlTaskCounter != 0 {
		t.Errorf("invalid input must not create tasks: len=%d counter=%d", len(dlTasks), dlTaskCounter)
	}
}

// TestStartHFDownloadUsesDownloadDirOverride verifies new model downloads land
// under the configured model download path (not the imported model directory).
func TestStartHFDownloadUsesDownloadDirOverride(t *testing.T) {
	saveConfigState(t)
	restoreSource := withModelScope404Server(t)
	defer restoreSource()

	dlDir := t.TempDir()
	modelDownloadDirMu.Lock()
	modelDownloadDirOverride = dlDir
	modelDownloadDirMu.Unlock()
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
	destDir := dlTasks[0].DestDir
	dlTasksMu.Unlock()
	want := filepath.Join(dlDir, "author", "model")
	if destDir != want {
		t.Errorf("DestDir = %q, want %q (model download path must be used)", destDir, want)
	}
	cancelAllTasks(t)
}

// TestCudaDeviceEnvEmpty verifies an empty serving-GPU selection (auto) yields
// nil so exec.Command inherits the parent environment unchanged (historical
// behavior).
func TestCudaDeviceEnvEmpty(t *testing.T) {
	if got := cudaDeviceEnv(""); got != nil {
		t.Errorf("cudaDeviceEnv(\"\") = %v, want nil", got)
	}
}

// TestCudaDeviceEnvOverridesInherited verifies a non-empty device UUID pins the
// child environment: the inherited variables are kept and CUDA_VISIBLE_DEVICES
// is appended LAST so it overrides any inherited value (last-wins semantics).
func TestCudaDeviceEnvOverridesInherited(t *testing.T) {
	env := cudaDeviceEnv("GPU-12345678-9abc-def0-1234-56789abcdef0")
	if len(env) < len(os.Environ()) {
		t.Fatalf("cudaDeviceEnv kept %d entries, want >= %d (inherited environment)", len(env), len(os.Environ()))
	}
	want := "CUDA_VISIBLE_DEVICES=GPU-12345678-9abc-def0-1234-56789abcdef0"
	if env[len(env)-1] != want {
		t.Errorf("last env entry = %q, want %q (override must be appended last)", env[len(env)-1], want)
	}
	inherited := false
	for _, kv := range env[:len(env)-1] {
		if strings.HasPrefix(kv, "PATH=") {
			inherited = true
			break
		}
	}
	if !inherited {
		t.Error("cudaDeviceEnv should keep the inherited environment entries")
	}
}
