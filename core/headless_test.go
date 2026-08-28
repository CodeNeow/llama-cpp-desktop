package core

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// ─── Handover file ────────────────────────────────────────────────

// TestHandoverWriteReadRoundTrip verifies the handover record write→read
// round-trip: pid/port survive losslessly and startedAt is a parseable
// RFC3339 timestamp.
func TestHandoverWriteReadRoundTrip(t *testing.T) {
	withTempCwd(t)

	if err := writeHandover(4242, 8080); err != nil {
		t.Fatalf("writeHandover failed: %v", err)
	}
	rec, err := readHandover()
	if err != nil {
		t.Fatalf("readHandover failed: %v", err)
	}
	if rec.Pid != 4242 || rec.Port != 8080 {
		t.Errorf("round-trip pid/port mismatch: %+v", rec)
	}
	if _, err := time.Parse(time.RFC3339, rec.StartedAt); err != nil {
		t.Errorf("startedAt should be RFC3339, got %q: %v", rec.StartedAt, err)
	}

	// removeHandover deletes the file and tolerates a missing file.
	if err := removeHandover(); err != nil {
		t.Errorf("removeHandover failed: %v", err)
	}
	if err := removeHandover(); err != nil {
		t.Errorf("removeHandover on missing file should be nil, got %v", err)
	}
	if _, err := os.Stat(handoverFile); !os.IsNotExist(err) {
		t.Errorf("handover file should be gone, stat err = %v", err)
	}
}

// TestHandoverReadCorruptFile verifies a corrupt handover file fails to parse
// (callers treat this as a stale record: delete + start fresh).
func TestHandoverReadCorruptFile(t *testing.T) {
	withTempCwd(t)

	if err := os.WriteFile(handoverFile, []byte("{not json"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := readHandover(); err == nil {
		t.Error("corrupt handover file should fail to parse")
	}
}

// TestHandoverReadMissingFile verifies a missing handover file returns the
// wrapped not-exist error so evaluateHandover can distinguish it from corrupt.
func TestHandoverReadMissingFile(t *testing.T) {
	withTempCwd(t)

	_, err := readHandover()
	if err == nil || !os.IsNotExist(err) {
		t.Errorf("missing handover file should return not-exist error, got %v", err)
	}
}

// ─── Handover decision ────────────────────────────────────────────

// TestDecideHandoverAction verifies the pure decision matrix:
// no file → start fresh (nothing to delete); healthy record → adopt with
// pid/port; unhealthy or unparsable record → delete + start fresh.
func TestDecideHandoverAction(t *testing.T) {
	rec := &handoverRecord{Pid: 100, Port: 8080}

	// no file → plain start, no delete
	plan := decideHandoverAction(false, nil, false)
	if plan.Adopt || plan.RemoveFile {
		t.Errorf("no record should plan a plain start, got %+v", plan)
	}
	// healthy → adopt with identity
	plan = decideHandoverAction(true, rec, true)
	if !plan.Adopt || plan.PID != 100 || plan.Port != 8080 || plan.RemoveFile {
		t.Errorf("healthy record should be adopted, got %+v", plan)
	}
	// file present but server dead → delete + start fresh
	plan = decideHandoverAction(true, rec, false)
	if plan.Adopt || !plan.RemoveFile {
		t.Errorf("stale record should plan delete + start, got %+v", plan)
	}
	// corrupt file (rec nil) → delete + start fresh
	plan = decideHandoverAction(true, nil, false)
	if plan.Adopt || !plan.RemoveFile {
		t.Errorf("corrupt record should plan delete + start, got %+v", plan)
	}
}

// injectHandoverProbes replaces the health/pid injection points for the test
// and restores them afterwards.
func injectHandoverProbes(t *testing.T, healthy bool, alive bool) (probeCalls, aliveCalls *int32) {
	t.Helper()
	var pc, ac int32
	origProbe := probeHandoverHealth
	origAlive := handoverPidAlive
	probeHandoverHealth = func(port int) bool {
		atomic.AddInt32(&pc, 1)
		return healthy
	}
	handoverPidAlive = func(pid int) bool {
		atomic.AddInt32(&ac, 1)
		return alive
	}
	t.Cleanup(func() {
		probeHandoverHealth = origProbe
		handoverPidAlive = origAlive
	})
	return &pc, &ac
}

// TestEvaluateHandover verifies evaluateHandover end-to-end with injected
// probes: healthy record adopts; dead-server record plans deletion; missing
// file plans a plain start.
func TestEvaluateHandover(t *testing.T) {
	withTempCwd(t)

	// healthy record → adopt
	if err := writeHandover(100, 8080); err != nil {
		t.Fatal(err)
	}
	probeCalls, aliveCalls := injectHandoverProbes(t, true, true)
	plan := evaluateHandover()
	if !plan.Adopt || plan.PID != 100 || plan.Port != 8080 || plan.RemoveFile {
		t.Errorf("healthy record should adopt, got %+v", plan)
	}
	if atomic.LoadInt32(probeCalls) == 0 || atomic.LoadInt32(aliveCalls) == 0 {
		t.Error("health probe and pid check should both be consulted")
	}

	// record present but server dead → delete + start
	_, _ = injectHandoverProbes(t, false, true)
	plan = evaluateHandover()
	if plan.Adopt || !plan.RemoveFile {
		t.Errorf("dead server should plan delete + start, got %+v", plan)
	}

	// missing file → plain start
	if err := removeHandover(); err != nil {
		t.Fatal(err)
	}
	plan = evaluateHandover()
	if plan.Adopt || plan.RemoveFile {
		t.Errorf("missing record should plan a plain start, got %+v", plan)
	}

	// corrupt file → delete + start
	if err := os.WriteFile(handoverFile, []byte("junk"), 0644); err != nil {
		t.Fatal(err)
	}
	plan = evaluateHandover()
	if plan.Adopt || !plan.RemoveFile {
		t.Errorf("corrupt record should plan delete + start, got %+v", plan)
	}
}

// TestAdoptHandoverSetsState verifies adoption flips the server globals:
// running=true, port set, adoptedPid recorded, serverCmd nil. A legacy record
// without a log path must not start a log tailer.
func TestAdoptHandoverSetsState(t *testing.T) {
	saveAdoptedState(t)

	adoptHandover(31337, 9090, "")

	serverMu.Lock()
	running, port, adopted, cmd := serverRunning, serverPort, adoptedPid, serverCmd
	startTime := serverStartTime
	tail := serverLogTail
	serverMu.Unlock()
	if !running || port != 9090 || adopted != 31337 {
		t.Errorf("adopt state wrong: running=%v port=%d adopted=%d", running, port, adopted)
	}
	if cmd != nil {
		t.Error("adopted server must leave serverCmd nil (no child handle)")
	}
	if startTime.IsZero() {
		t.Error("adopt should stamp serverStartTime (uptime)")
	}
	if tail != nil {
		t.Error("adopting a legacy record without logPath must not start a log tailer")
	}
}

// ─── Handover record log path ─────────────────────────────────────

// TestHandoverRecordLogPathRoundTrip verifies the handover record carries the
// resolved absolute server log path: writeHandover fills logPath from
// serverLogFile, the JSON contains the key, and readHandover returns it.
func TestHandoverRecordLogPathRoundTrip(t *testing.T) {
	withTempCwd(t)
	orig := serverLogFile
	serverLogFile = filepath.Join(t.TempDir(), "server.log")
	t.Cleanup(func() { serverLogFile = orig })

	if err := writeHandover(4242, 8080); err != nil {
		t.Fatalf("writeHandover failed: %v", err)
	}
	data, err := os.ReadFile(handoverFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"logPath"`) {
		t.Errorf("handover record JSON should contain logPath: %s", data)
	}
	rec, err := readHandover()
	if err != nil {
		t.Fatalf("readHandover failed: %v", err)
	}
	if rec.LogPath != serverLogFile {
		t.Errorf("round-trip logPath = %q, want %q", rec.LogPath, serverLogFile)
	}
	if !filepath.IsAbs(rec.LogPath) {
		t.Errorf("handover logPath must be absolute (successor must not depend on cwd), got %q", rec.LogPath)
	}
}

// TestHandoverRecordLegacyNoLogPath verifies backward compatibility: a record
// JSON written by an older version (no logPath key) parses with an empty
// LogPath, and the decision core adopts it with an empty log path (no
// tailing) instead of rejecting it.
func TestHandoverRecordLegacyNoLogPath(t *testing.T) {
	withTempCwd(t)

	legacy := `{"pid":7,"port":8080,"startedAt":"2024-01-01T00:00:00Z"}`
	if err := os.WriteFile(handoverFile, []byte(legacy), 0644); err != nil {
		t.Fatal(err)
	}
	rec, err := readHandover()
	if err != nil {
		t.Fatalf("legacy record should parse: %v", err)
	}
	if rec.Pid != 7 || rec.Port != 8080 {
		t.Errorf("legacy pid/port wrong: %+v", rec)
	}
	if rec.LogPath != "" {
		t.Errorf("legacy record without logPath should yield empty LogPath, got %q", rec.LogPath)
	}
	plan := decideHandoverAction(true, rec, true)
	if !plan.Adopt || plan.PID != 7 || plan.Port != 8080 || plan.LogPath != "" {
		t.Errorf("legacy record should adopt without log path, got %+v", plan)
	}
}

// TestAdoptHandoverTailsLogFile verifies the adopted-server log capture end
// to end against a temp log file: adoptHandover with a log path points the
// log source at the file and starts a tailer from EOF (pre-existing stale
// content is never replayed), lines the adopted child appends afterwards
// reach the ring, and the adopted-server stop path stops the tailer and its
// goroutine exits.
func TestAdoptHandoverTailsLogFile(t *testing.T) {
	withTempCwd(t)
	saveAdoptedState(t)
	// Shrink the poll interval so appended lines reach the ring quickly.
	origIdle := serverLogIdle
	serverLogIdle = 5 * time.Millisecond
	t.Cleanup(func() { serverLogIdle = origIdle })

	logPath := filepath.Join(t.TempDir(), "adopted-server.log")
	// Pre-existing content from the previous process must NOT be replayed.
	if err := os.WriteFile(logPath, []byte("stale pre-handover line\n"), 0644); err != nil {
		t.Fatal(err)
	}

	adoptHandover(2718, 8080, logPath)

	serverMu.Lock()
	tail := serverLogTail
	logFile := serverLogFile
	serverMu.Unlock()
	if tail == nil {
		t.Fatal("adopting a record with a log path must start a log tailer")
	}
	if logFile != logPath {
		t.Errorf("serverLogFile = %q, want the adopted log path %q", logFile, logPath)
	}
	for _, line := range serverLogsCopy() {
		if strings.Contains(line, "stale") {
			t.Fatalf("stale content was replayed into the ring: %v", serverLogsCopy())
		}
	}

	// Append a line as the adopted child would, then wait for it in the ring.
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("fresh adopted line\n"); err != nil {
		t.Fatal(err)
	}
	f.Close()
	waitForServerLogLine(t, "fresh adopted line", 3*time.Second)

	// The adopted-server stop path must also stop the tailer.
	kills := injectKillByPid(t)
	if !stopAdoptedServerIfAny() {
		t.Fatal("stopAdoptedServerIfAny should stop the adopted server")
	}
	if len(*kills) != 1 || (*kills)[0] != 2718 {
		t.Errorf("adopted server should be killed by pid 2718, kills = %v", *kills)
	}
	serverMu.Lock()
	cleared := serverLogTail
	serverMu.Unlock()
	if cleared != nil {
		t.Error("serverLogTail must be cleared after the adopted server stops")
	}
	select {
	case <-tail.done:
	case <-time.After(3 * time.Second):
		t.Fatal("tailer goroutine did not exit after the adopted server stopped")
	}
	for _, line := range serverLogsCopy() {
		if strings.Contains(line, "stale") {
			t.Fatalf("stale content appeared in the ring: %v", serverLogsCopy())
		}
	}
}

// ─── Adopted-server stop path ─────────────────────────────────────

// injectKillByPid replaces the killProcessByPid injection point, recording the
// killed pids; nothing is actually killed.
func injectKillByPid(t *testing.T) (calls *[]int) {
	t.Helper()
	var got []int
	orig := killProcessByPid
	killProcessByPid = func(pid int) error {
		got = append(got, pid)
		return nil
	}
	t.Cleanup(func() { killProcessByPid = orig })
	return &got
}

// saveAdoptedState snapshots and restores the adopted-server globals plus the
// switch-restart marker, the log tailer handle and the log-file path var. A
// tailer a test left running is stopped on cleanup so its goroutine cannot
// pollute later tests.
func saveAdoptedState(t *testing.T) {
	t.Helper()
	serverMu.Lock()
	origRunning := serverRunning
	origCmd := serverCmd
	origAdopted := adoptedPid
	origPort := serverPort
	origStart := serverStartTime
	origTail := serverLogTail
	origLogFile := serverLogFile
	serverMu.Unlock()
	origSwitch := switchRestartPending.Load()
	t.Cleanup(func() {
		serverMu.Lock()
		curTail := serverLogTail
		serverRunning = origRunning
		serverCmd = origCmd
		adoptedPid = origAdopted
		serverPort = origPort
		serverStartTime = origStart
		serverLogTail = origTail
		serverMu.Unlock()
		serverLogFile = origLogFile
		if curTail != nil && curTail != origTail {
			curTail.Stop()
			curTail.WaitDone(2 * time.Second)
		}
		switchRestartPending.Store(origSwitch)
	})
}

// TestStopServerInternalAdoptedBranch verifies the adopted-server stop branch:
// with serverRunning=true, serverCmd=nil, adoptedPid set, stopping kills by
// the adopted pid (via the injected kill) and deletes the handover file.
func TestStopServerInternalAdoptedBranch(t *testing.T) {
	withTempCwd(t)
	saveAdoptedState(t)
	kills := injectKillByPid(t)

	serverMu.Lock()
	serverRunning = true
	serverCmd = nil
	adoptedPid = 2718
	serverPort = 8080
	serverMu.Unlock()
	if err := writeHandover(2718, 8080); err != nil {
		t.Fatal(err)
	}

	if err := stopServerInternal(); err != nil {
		t.Fatalf("stopServerInternal returned error: %v", err)
	}
	if len(*kills) != 1 || (*kills)[0] != 2718 {
		t.Errorf("adopted server should be killed by pid 2718, kills = %v", *kills)
	}
	if _, err := os.Stat(handoverFile); !os.IsNotExist(err) {
		t.Errorf("handover file should be removed after stopping adopted server, stat err = %v", err)
	}
	serverMu.Lock()
	running, adopted, port := serverRunning, adoptedPid, serverPort
	serverMu.Unlock()
	if running || adopted != 0 || port != 0 {
		t.Errorf("adopted state should be cleared, running=%v adopted=%d port=%d", running, adopted, port)
	}
}

// TestStopAdoptedServerRequiresNilCmd verifies a running child server
// (serverCmd != nil) never takes the adopted branch: no pid kill happens.
func TestStopAdoptedServerRequiresNilCmd(t *testing.T) {
	saveAdoptedState(t)
	kills := injectKillByPid(t)

	serverMu.Lock()
	serverRunning = true
	serverCmd = &exec.Cmd{}
	adoptedPid = 999
	serverMu.Unlock()

	if stopAdoptedServerIfAny() {
		t.Error("child server (cmd != nil) must not take the adopted branch")
	}
	if len(*kills) != 0 {
		t.Errorf("no pid kill expected for child server, kills = %v", *kills)
	}
}

// ─── Handover write from live server state ────────────────────────

// TestWriteServerHandover verifies writeServerHandover maps the live server
// state to the handover record: child pid when we own the child, adopted pid
// when adopted, and no file when the server is not running.
func TestWriteServerHandover(t *testing.T) {
	withTempCwd(t)
	saveAdoptedState(t)

	// not running → no file, no error
	if err := writeServerHandover(); err != nil {
		t.Fatalf("writeServerHandover with no server should be nil, got %v", err)
	}
	if _, err := os.Stat(handoverFile); !os.IsNotExist(err) {
		t.Error("no handover file should be written when the server is not running")
	}

	// adopted server → adopted pid + port
	serverMu.Lock()
	serverRunning = true
	serverCmd = nil
	adoptedPid = 1234
	serverPort = 8181
	serverMu.Unlock()
	if err := writeServerHandover(); err != nil {
		t.Fatal(err)
	}
	rec, err := readHandover()
	if err != nil {
		t.Fatal(err)
	}
	if rec.Pid != 1234 || rec.Port != 8181 {
		t.Errorf("adopted handover record wrong: %+v", rec)
	}

	// child server → child process pid
	serverMu.Lock()
	serverCmd = &exec.Cmd{Process: &os.Process{Pid: 777}}
	adoptedPid = 0
	serverMu.Unlock()
	if err := writeServerHandover(); err != nil {
		t.Fatal(err)
	}
	rec, err = readHandover()
	if err != nil {
		t.Fatal(err)
	}
	if rec.Pid != 777 {
		t.Errorf("child handover record should use child pid 777, got %+v", rec)
	}
}

// ─── Shutdown switch-restart branch ────────────────────────────────

// TestShutdownSkipsStopOnSwitchRestart verifies the Shutdown switch-restart
// branch: with switchRestartPending set, an adopted llama-server survives
// (no pid kill, handover record kept, state untouched).
func TestShutdownSkipsStopOnSwitchRestart(t *testing.T) {
	withTempCwd(t)
	saveAdoptedState(t)
	saveConfigState(t)
	kills := injectKillByPid(t)

	serverMu.Lock()
	serverRunning = true
	serverCmd = nil
	adoptedPid = 5555
	serverPort = 8080
	serverMu.Unlock()
	if err := writeHandover(5555, 8080); err != nil {
		t.Fatal(err)
	}
	switchRestartPending.Store(true)

	app := &App{}
	app.Shutdown(nil)

	if len(*kills) != 0 {
		t.Errorf("switch-restart shutdown must not kill the server, kills = %v", *kills)
	}
	if _, err := os.Stat(handoverFile); err != nil {
		t.Errorf("handover record must survive a switch-restart shutdown: %v", err)
	}
	serverMu.Lock()
	running, adopted := serverRunning, adoptedPid
	serverMu.Unlock()
	if !running || adopted != 5555 {
		t.Errorf("adopted server state must be untouched on switch-restart, running=%v adopted=%d", running, adopted)
	}
}

// TestShutdownStopsAdoptedServer verifies the normal (non-switch) Shutdown
// path stops an adopted llama-server: pid kill via the injection point and
// handover record removal.
func TestShutdownStopsAdoptedServer(t *testing.T) {
	withTempCwd(t)
	saveAdoptedState(t)
	saveConfigState(t)
	kills := injectKillByPid(t)

	serverMu.Lock()
	serverRunning = true
	serverCmd = nil
	adoptedPid = 6666
	serverPort = 8080
	serverMu.Unlock()
	if err := writeHandover(6666, 8080); err != nil {
		t.Fatal(err)
	}
	switchRestartPending.Store(false)

	app := &App{}
	app.Shutdown(nil)

	if len(*kills) != 1 || (*kills)[0] != 6666 {
		t.Errorf("normal shutdown should kill adopted server by pid 6666, kills = %v", *kills)
	}
	if _, err := os.Stat(handoverFile); !os.IsNotExist(err) {
		t.Errorf("handover record should be removed after stopping the adopted server, stat err = %v", err)
	}
	serverMu.Lock()
	running := serverRunning
	serverMu.Unlock()
	if running {
		t.Error("server should not be running after shutdown stop")
	}
}

// ─── Headless startup decision ────────────────────────────────────

// TestShouldRunHeadlessPure verifies the pure headless decision matrix:
// non-Windows never headless; --headless wins; --gui overrides the persisted
// preference; dev builds ignore the preference (only the flag works);
// otherwise the preference decides.
func TestShouldRunHeadlessPure(t *testing.T) {
	cases := []struct {
		name        string
		goos        string
		headlessFlg bool
		guiFlag     bool
		enabled     bool
		devBuild    bool
		want        bool
	}{
		{"non-windows never headless", "linux", true, false, true, false, false},
		{"headless flag wins", "windows", true, true, false, false, true},
		{"headless flag wins in dev build", "windows", true, false, false, true, true},
		{"config enabled", "windows", false, false, true, false, true},
		{"config disabled", "windows", false, false, false, false, false},
		{"gui flag overrides config", "windows", false, true, true, false, false},
		{"dev build ignores config preference", "windows", false, false, true, true, false},
		{"dev build gui flag stays GUI", "windows", false, true, false, true, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := shouldRunHeadless(c.goos, c.headlessFlg, c.guiFlag, c.enabled, c.devBuild); got != c.want {
				t.Errorf("shouldRunHeadless(%q,%v,%v,%v,%v) = %v, want %v", c.goos, c.headlessFlg, c.guiFlag, c.enabled, c.devBuild, got, c.want)
			}
		})
	}
}

// TestStartOrAdoptServerDecision verifies the headless startup decision with
// injected probes: healthy record adopts without starting; with no record the
// fresh-start attempt fails against an empty model directory (expected,
// logged) and must not mark the server running; a stale record is deleted.
// Every failed start must surface through notifyHeadlessServerStartFailed,
// while adopting a healthy record must not notify.
func TestStartOrAdoptServerDecision(t *testing.T) {
	withTempCwd(t)
	saveServerState(t)
	saveAdoptedState(t)

	// Record notify calls instead of popping the real Windows alert dialog
	var notifyErrs []error
	origNotify := notifyHeadlessServerStartFailed
	notifyHeadlessServerStartFailed = func(err error) { notifyErrs = append(notifyErrs, err) }
	t.Cleanup(func() { notifyHeadlessServerStartFailed = origNotify })

	// no record → startServerInternal fails (empty LLM-Models), server not running
	if err := removeHandover(); err != nil {
		t.Fatal(err)
	}
	_, _ = injectHandoverProbes(t, true, true)
	startOrAdoptServer()
	serverMu.Lock()
	running := serverRunning
	serverMu.Unlock()
	if running {
		t.Error("no handover record + failed start must not mark the server running")
	}
	if len(notifyErrs) != 1 || notifyErrs[0] == nil {
		t.Errorf("a failed start must notify once with the error, got %d calls: %v", len(notifyErrs), notifyErrs)
	}

	// healthy record → adopted, no start attempt needed
	if err := writeHandover(4321, 8282); err != nil {
		t.Fatal(err)
	}
	startOrAdoptServer()
	serverMu.Lock()
	running, adopted, port := serverRunning, adoptedPid, serverPort
	serverMu.Unlock()
	if !running || adopted != 4321 || port != 8282 {
		t.Errorf("healthy record should be adopted, running=%v adopted=%d port=%d", running, adopted, port)
	}
	if len(notifyErrs) != 1 {
		t.Errorf("adopting a healthy record must not notify, got %d calls", len(notifyErrs))
	}

	// stale record → deleted even though the subsequent start fails
	if err := writeHandover(9999, 8383); err != nil {
		t.Fatal(err)
	}
	_, _ = injectHandoverProbes(t, false, false)
	startOrAdoptServer()
	if _, err := os.Stat(handoverFile); !os.IsNotExist(err) {
		t.Errorf("stale handover record should be deleted, stat err = %v", err)
	}
	if len(notifyErrs) != 2 || notifyErrs[1] == nil {
		t.Errorf("the start after stale-record cleanup must notify again, got %d calls: %v", len(notifyErrs), notifyErrs)
	}
}

// TestAdoptOrCleanHandover verifies the GUI startup variant: a healthy record
// is adopted, a stale one deleted, and no server is auto-started when the
// record is missing.
func TestAdoptOrCleanHandover(t *testing.T) {
	withTempCwd(t)
	saveServerState(t)
	saveAdoptedState(t)

	// missing record → nothing happens (no auto start in GUI mode)
	if err := removeHandover(); err != nil {
		t.Fatal(err)
	}
	adoptOrCleanHandover()
	serverMu.Lock()
	running := serverRunning
	serverMu.Unlock()
	if running {
		t.Error("GUI startup must not auto-start or adopt with no handover record")
	}

	// stale record → deleted
	if err := writeHandover(1111, 8484); err != nil {
		t.Fatal(err)
	}
	_, _ = injectHandoverProbes(t, false, false)
	adoptOrCleanHandover()
	if _, err := os.Stat(handoverFile); !os.IsNotExist(err) {
		t.Errorf("stale record should be deleted on GUI startup, stat err = %v", err)
	}

	// healthy record → adopted
	if err := writeHandover(2222, 8585); err != nil {
		t.Fatal(err)
	}
	_, _ = injectHandoverProbes(t, true, true)
	adoptOrCleanHandover()
	serverMu.Lock()
	running, adopted, port := serverRunning, adoptedPid, serverPort
	serverMu.Unlock()
	if !running || adopted != 2222 || port != 8585 {
		t.Errorf("GUI startup should adopt a healthy record, running=%v adopted=%d port=%d", running, adopted, port)
	}
}

// TestConfigApiRouteModeRoundTrip verifies apiRouteMode persistence: default
// false when the field is missing (old configs), explicit true survives a
// save→load round-trip.
func TestConfigApiRouteModeRoundTrip(t *testing.T) {
	withTempCwd(t)
	saveConfigState(t)

	// old config without the field → false
	if err := os.WriteFile(configFile, []byte(`{"theme":"light"}`), 0644); err != nil {
		t.Fatal(err)
	}
	loadConfig()
	if ApiRouteMode() {
		t.Error("old config missing apiRouteMode should default to false")
	}

	// explicit true round-trips
	configMu.Lock()
	apiRouteMode = true
	configMu.Unlock()
	saveConfig()
	data, err := os.ReadFile(configFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"apiRouteMode": true`) {
		t.Errorf("saved config should contain apiRouteMode: true, actual: %s", data)
	}
	configMu.Lock()
	apiRouteMode = false
	configMu.Unlock()
	loadConfig()
	if !ApiRouteMode() {
		t.Error("apiRouteMode=true should survive a save→load round-trip")
	}

	// SetApiRouteMode(false) persists without side effects (no relaunch path)
	configMu.Lock()
	apiRouteMode = true
	configMu.Unlock()
	app := &App{}
	if err := app.SetApiRouteMode(false); err != nil {
		t.Fatalf("SetApiRouteMode(false) should be nil: %v", err)
	}
	if ApiRouteMode() {
		t.Error("SetApiRouteMode(false) should persist false")
	}
	switchRestartPending.Store(false)
}

// TestSetApiRouteModeDevRejected verifies the enable path is rejected in a
// dev build (wails dev): the relaunch-based switch escapes the dev supervisor
// and tears the dev session down, so the preference stays false, no successor
// is spawned and the switch-restart marker stays unset.
func TestSetApiRouteModeDevRejected(t *testing.T) {
	withTempCwd(t)
	saveAdoptedState(t)
	saveConfigState(t)
	launches := injectRelaunchSelf(t, false)

	origDev := isDevBuild
	isDevBuild = true
	t.Cleanup(func() { isDevBuild = origDev })

	// Tray on so the dev guard — not the tray guard — is what rejects
	configMu.Lock()
	trayEnabled = true
	apiRouteMode = false
	configMu.Unlock()

	app := &App{}
	if err := app.SetApiRouteMode(true); err == nil {
		t.Fatal("enabling API route mode in a dev build should surface an error")
	}
	if ApiRouteMode() {
		t.Error("apiRouteMode must stay false when the dev guard rejects the call")
	}
	if len(*launches) != 0 {
		t.Errorf("no successor should be spawned when the dev guard rejects, launches = %v", *launches)
	}
	if switchRestartPending.Load() {
		t.Error("switch-restart marker must stay unset when the dev guard rejects")
	}
}

// injectRelaunchSelf replaces the relaunchSelf injection point, recording the
// argument lists the code tried to spawn; no real process starts.
func injectRelaunchSelf(t *testing.T, fail bool) (calls *[][]string) {
	t.Helper()
	var got [][]string
	orig := relaunchSelf
	relaunchSelf = func(args ...string) error {
		got = append(got, args)
		if fail {
			return os.ErrPermission
		}
		return nil
	}
	t.Cleanup(func() { relaunchSelf = orig })
	return &got
}

// TestSetApiRouteModeEnableHandsOver verifies the enable path with an injected
// relaunch: apiRouteMode persists true, a running adopted server gets a
// handover record, the successor is spawned with --headless, and the
// switch-restart marker is set (a nil ctx in tests skips the Wails quit).
func TestSetApiRouteModeEnableHandsOver(t *testing.T) {
	withTempCwd(t)
	saveAdoptedState(t)
	saveConfigState(t)
	launches := injectRelaunchSelf(t, false)

	// Enabling requires the tray (the headless return entry point)
	configMu.Lock()
	trayEnabled = true
	configMu.Unlock()

	serverMu.Lock()
	serverRunning = true
	serverCmd = nil
	adoptedPid = 3141
	serverPort = 8080
	serverMu.Unlock()

	app := &App{}
	if err := app.SetApiRouteMode(true); err != nil {
		t.Fatalf("SetApiRouteMode(true) should succeed with injected relaunch: %v", err)
	}
	if !ApiRouteMode() {
		t.Error("SetApiRouteMode(true) should persist apiRouteMode=true")
	}
	if len(*launches) != 1 || len((*launches)[0]) != 1 || (*launches)[0][0] != "--headless" {
		t.Errorf("successor should be spawned once with --headless, launches = %v", *launches)
	}
	if !switchRestartPending.Load() {
		t.Error("switch-restart marker should be set after a successful relaunch")
	}
	rec, err := readHandover()
	if err != nil {
		t.Fatalf("handover record should be written for the adopted server: %v", err)
	}
	if rec.Pid != 3141 || rec.Port != 8080 {
		t.Errorf("handover record should carry the adopted pid/port, got %+v", rec)
	}
	switchRestartPending.Store(false)
}

// TestSetApiRouteModeEnableRelaunchFailure verifies the enable path when the
// relaunch fails: an error is returned and the switch-restart marker stays
// unset, so the subsequent GUI shutdown still stops the server normally.
func TestSetApiRouteModeEnableRelaunchFailure(t *testing.T) {
	withTempCwd(t)
	saveAdoptedState(t)
	saveConfigState(t)
	_ = injectRelaunchSelf(t, true)

	// Enabling requires the tray (the headless return entry point)
	configMu.Lock()
	trayEnabled = true
	configMu.Unlock()

	app := &App{}
	if err := app.SetApiRouteMode(true); err == nil {
		t.Fatal("relaunch failure should surface an error")
	}
	if switchRestartPending.Load() {
		t.Error("switch-restart marker must stay unset when the relaunch fails")
	}
}

// TestSetApiRouteModeRequiresTray verifies the enable path is rejected while
// the system tray is disabled (the tray menu is the only way back from
// headless mode): the preference stays false, no successor is spawned and
// the switch-restart marker stays unset.
func TestSetApiRouteModeRequiresTray(t *testing.T) {
	withTempCwd(t)
	saveAdoptedState(t)
	saveConfigState(t)
	launches := injectRelaunchSelf(t, false)

	configMu.Lock()
	trayEnabled = false
	apiRouteMode = false
	configMu.Unlock()

	app := &App{}
	if err := app.SetApiRouteMode(true); err == nil {
		t.Fatal("enabling API route mode with the tray disabled should surface an error")
	}
	if ApiRouteMode() {
		t.Error("apiRouteMode must stay false when the tray guard rejects the call")
	}
	if len(*launches) != 0 {
		t.Errorf("no successor should be spawned when the guard rejects, launches = %v", *launches)
	}
	if switchRestartPending.Load() {
		t.Error("switch-restart marker must stay unset when the guard rejects")
	}
}
