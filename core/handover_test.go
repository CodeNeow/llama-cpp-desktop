package core

import (
	"bytes"
	"log"
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
// round-trip: pid/port survive losslessly, startedAt is a parseable RFC3339
// timestamp, and serverStartedAt round-trips the running server's real
// creation time (serverTrueStart).
func TestHandoverWriteReadRoundTrip(t *testing.T) {
	withTempCwd(t)
	trueStart := time.Date(2025, 6, 1, 12, 30, 45, 0, time.UTC)
	setServerTrueStart(t, trueStart)

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
	parsed, err := time.Parse(time.RFC3339, rec.ServerStartedAt)
	if err != nil {
		t.Fatalf("serverStartedAt should be RFC3339, got %q: %v", rec.ServerStartedAt, err)
	}
	if !parsed.Equal(trueStart) {
		t.Errorf("serverStartedAt round-trip = %v, want %v", parsed, trueStart)
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

// TestHandoverRecordServerStartedAtOmittedWhenZero verifies writeHandover
// picks up serverTrueStart and, when it is zero (unknown server start time),
// omits the serverStartedAt key from the JSON entirely (omitempty) — the
// successor's start-time check then fails open.
func TestHandoverRecordServerStartedAtOmittedWhenZero(t *testing.T) {
	withTempCwd(t)

	// zero serverTrueStart → no serverStartedAt key in the JSON
	setServerTrueStart(t, time.Time{})
	if err := writeHandover(4242, 8080); err != nil {
		t.Fatalf("writeHandover failed: %v", err)
	}
	data, err := os.ReadFile(handoverFile)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "serverStartedAt") {
		t.Errorf("zero serverTrueStart must omit serverStartedAt from the JSON: %s", data)
	}
	rec, err := readHandover()
	if err != nil {
		t.Fatal(err)
	}
	if rec.ServerStartedAt != "" {
		t.Errorf("zero serverTrueStart should yield empty ServerStartedAt, got %q", rec.ServerStartedAt)
	}

	// non-zero serverTrueStart → the key is present and carries the value
	trueStart := time.Date(2025, 6, 1, 12, 30, 45, 0, time.UTC)
	setServerTrueStart(t, trueStart)
	if err := writeHandover(4242, 8080); err != nil {
		t.Fatalf("writeHandover failed: %v", err)
	}
	data, err = os.ReadFile(handoverFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"serverStartedAt"`) {
		t.Errorf("handover record JSON should contain serverStartedAt: %s", data)
	}
	if rec, err := readHandover(); err != nil {
		t.Fatal(err)
	} else if got, _ := time.Parse(time.RFC3339, rec.ServerStartedAt); !got.Equal(trueStart) {
		t.Errorf("serverStartedAt = %v, want %v", rec.ServerStartedAt, trueStart)
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
// no file → start fresh (nothing to delete); healthy record with a verified
// (or skipped, fail-open) start time → adopt with pid/port and the check
// outcome; unhealthy record, start-time mismatch, or unparsable record →
// delete + start fresh.
func TestDecideHandoverAction(t *testing.T) {
	rec := &handoverRecord{Pid: 100, Port: 8080}

	// no file → plain start, no delete
	plan := decideHandoverAction(false, nil, false, handoverStartCheck{})
	if plan.Adopt || plan.RemoveFile {
		t.Errorf("no record should plan a plain start, got %+v", plan)
	}
	// healthy + verified start time → adopt with identity and outcome
	plan = decideHandoverAction(true, rec, true, handoverStartCheck{Verified: true})
	if !plan.Adopt || plan.PID != 100 || plan.Port != 8080 || plan.RemoveFile {
		t.Errorf("healthy record should be adopted, got %+v", plan)
	}
	if !plan.Start.Verified || plan.Start.SkipReason != "" {
		t.Errorf("adopted plan should carry the verified outcome, got %+v", plan.Start)
	}
	// healthy + skipped (fail-open) start time → still adopt, reason carried
	plan = decideHandoverAction(true, rec, true, handoverStartCheck{SkipReason: "legacy record without serverStartedAt"})
	if !plan.Adopt {
		t.Errorf("healthy record with skipped start-time check should be adopted, got %+v", plan)
	}
	if plan.Start.Verified || plan.Start.SkipReason == "" {
		t.Errorf("adopted plan should carry the skip reason, got %+v", plan.Start)
	}
	// file present but server dead → delete + start fresh
	plan = decideHandoverAction(true, rec, false, handoverStartCheck{Verified: true})
	if plan.Adopt || !plan.RemoveFile {
		t.Errorf("stale record should plan delete + start, got %+v", plan)
	}
	// healthy probes but pid-reuse start-time mismatch → delete + start fresh
	plan = decideHandoverAction(true, rec, true, handoverStartCheck{})
	if plan.Adopt || !plan.RemoveFile {
		t.Errorf("start-time mismatch should plan delete + start, got %+v", plan)
	}
	// corrupt file (rec nil) → delete + start fresh
	plan = decideHandoverAction(true, nil, false, handoverStartCheck{})
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

// injectHandoverProcStart replaces the process creation-time injection point
// with a fixed answer (start, ok), counting the calls; restored on cleanup.
func injectHandoverProcStart(t *testing.T, start time.Time, ok bool) *int32 {
	t.Helper()
	var calls int32
	orig := handoverProcStartTime
	handoverProcStartTime = func(int) (time.Time, bool) {
		atomic.AddInt32(&calls, 1)
		return start, ok
	}
	t.Cleanup(func() { handoverProcStartTime = orig })
	return &calls
}

// setServerTrueStart stamps serverTrueStart for the test and restores the
// previous value afterwards (for tests that do not go through
// saveAdoptedState, which snapshots the var as well).
func setServerTrueStart(t *testing.T, start time.Time) {
	t.Helper()
	serverMu.Lock()
	orig := serverTrueStart
	serverTrueStart = start
	serverMu.Unlock()
	t.Cleanup(func() {
		serverMu.Lock()
		serverTrueStart = orig
		serverMu.Unlock()
	})
}

// TestEvaluateHandover verifies evaluateHandover end-to-end with injected
// probes: healthy record adopts; dead-server record plans deletion; missing
// file plans a plain start. The records here carry no serverStartedAt (zero
// serverTrueStart), so the start-time check fails open and must not consult
// the creation-time probe at all.
func TestEvaluateHandover(t *testing.T) {
	withTempCwd(t)
	setServerTrueStart(t, time.Time{})

	// healthy record → adopt
	if err := writeHandover(100, 8080); err != nil {
		t.Fatal(err)
	}
	probeCalls, aliveCalls := injectHandoverProbes(t, true, true)
	procCalls := injectHandoverProcStart(t, time.Now(), true)
	plan := evaluateHandover()
	if !plan.Adopt || plan.PID != 100 || plan.Port != 8080 || plan.RemoveFile {
		t.Errorf("healthy record should adopt, got %+v", plan)
	}
	if atomic.LoadInt32(probeCalls) == 0 || atomic.LoadInt32(aliveCalls) == 0 {
		t.Error("health probe and pid check should both be consulted")
	}
	if atomic.LoadInt32(procCalls) != 0 {
		t.Error("legacy record without serverStartedAt must skip the start-time check")
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

// TestEvaluateHandoverStartTimeCheck verifies the pid-reuse defense end to
// end: a record carrying serverStartedAt is only adopted when the queried
// process creation time matches (within tolerance); a mismatch treats the
// record as stale. Fail-open cases (legacy record without the field,
// unparseable stored time, unavailable creation-time query) still adopt.
func TestEvaluateHandoverStartTimeCheck(t *testing.T) {
	withTempCwd(t)

	recorded := time.Date(2025, 6, 1, 12, 30, 45, 0, time.UTC)
	setServerTrueStart(t, recorded)
	if err := writeHandover(100, 8080); err != nil {
		t.Fatal(err)
	}

	// matching creation time → adopt, outcome carries "verified"
	_, _ = injectHandoverProbes(t, true, true)
	procCalls := injectHandoverProcStart(t, recorded, true)
	plan := evaluateHandover()
	if !plan.Adopt || plan.RemoveFile {
		t.Errorf("matching start time should adopt, got %+v", plan)
	}
	if !plan.Start.Verified || plan.Start.SkipReason != "" {
		t.Errorf("matching start time should mark the plan verified, got %+v", plan.Start)
	}
	if atomic.LoadInt32(procCalls) == 0 {
		t.Error("record with serverStartedAt must consult the creation-time probe")
	}

	// mismatching creation time → stale record
	_, _ = injectHandoverProbes(t, true, true)
	_ = injectHandoverProcStart(t, recorded.Add(time.Hour), true)
	if plan := evaluateHandover(); plan.Adopt || !plan.RemoveFile {
		t.Errorf("mismatched start time should treat the record as stale, got %+v", plan)
	}

	// unparseable stored time → skip check, adopt with the reason named
	if err := os.WriteFile(handoverFile,
		[]byte(`{"pid":100,"port":8080,"startedAt":"2024-01-01T00:00:00Z","serverStartedAt":"not-a-time"}`), 0644); err != nil {
		t.Fatal(err)
	}
	_, _ = injectHandoverProbes(t, true, true)
	procCalls = injectHandoverProcStart(t, recorded, true)
	plan = evaluateHandover()
	if !plan.Adopt || plan.RemoveFile {
		t.Errorf("unparseable serverStartedAt should fail open and adopt, got %+v", plan)
	}
	if plan.Start.Verified || plan.Start.SkipReason != "serverStartedAt unparseable" {
		t.Errorf("unparseable serverStartedAt should carry its skip reason, got %+v", plan.Start)
	}
	if atomic.LoadInt32(procCalls) != 0 {
		t.Error("unparseable serverStartedAt must skip the creation-time query")
	}

	// creation-time query unavailable (ok=false) → skip check, adopt
	setServerTrueStart(t, recorded)
	if err := writeHandover(100, 8080); err != nil {
		t.Fatal(err)
	}
	_, _ = injectHandoverProbes(t, true, true)
	_ = injectHandoverProcStart(t, time.Time{}, false)
	plan = evaluateHandover()
	if !plan.Adopt || plan.RemoveFile {
		t.Errorf("unavailable creation-time query should fail open and adopt, got %+v", plan)
	}
	if plan.Start.Verified || plan.Start.SkipReason != "process start time unavailable" {
		t.Errorf("unavailable creation-time query should carry its skip reason, got %+v", plan.Start)
	}

	// legacy record without serverStartedAt → skip check, adopt
	setServerTrueStart(t, time.Time{})
	if err := writeHandover(100, 8080); err != nil {
		t.Fatal(err)
	}
	_, _ = injectHandoverProbes(t, true, true)
	procCalls = injectHandoverProcStart(t, recorded, true)
	plan = evaluateHandover()
	if !plan.Adopt || plan.RemoveFile {
		t.Errorf("legacy record should fail open and adopt, got %+v", plan)
	}
	if plan.Start.Verified || plan.Start.SkipReason != "legacy record without serverStartedAt" {
		t.Errorf("legacy record should carry its skip reason, got %+v", plan.Start)
	}
	if atomic.LoadInt32(procCalls) != 0 {
		t.Error("legacy record without serverStartedAt must skip the creation-time query")
	}
}

// TestVerifyHandoverStartTolerance verifies the boundary of the start-time
// comparison against an injected fake clock (fixed creation time) and an
// injected tolerance: |queried − recorded| ≤ tolerance verifies, anything
// beyond is a genuine mismatch (no skip reason).
func TestVerifyHandoverStartTolerance(t *testing.T) {
	recorded := time.Date(2025, 6, 1, 12, 30, 45, 0, time.UTC)
	rec := &handoverRecord{Pid: 100, Port: 8080, ServerStartedAt: recorded.Format(time.RFC3339)}

	origTolerance := handoverStartTolerance
	t.Cleanup(func() { handoverStartTolerance = origTolerance })
	origProcStart := handoverProcStartTime
	t.Cleanup(func() { handoverProcStartTime = origProcStart })

	cases := []struct {
		name      string
		offset    time.Duration
		tolerance time.Duration
		wantMatch bool
	}{
		{"exact match", 0, 5 * time.Second, true},
		{"within tolerance (+4s)", 4 * time.Second, 5 * time.Second, true},
		{"within tolerance (-4s)", -4 * time.Second, 5 * time.Second, true},
		{"boundary inclusive (+5s)", 5 * time.Second, 5 * time.Second, true},
		{"boundary inclusive (-5s)", -5 * time.Second, 5 * time.Second, true},
		{"beyond tolerance (+6s)", 6 * time.Second, 5 * time.Second, false},
		{"beyond tolerance (-6s)", -6 * time.Second, 5 * time.Second, false},
		{"larger injected tolerance", 6 * time.Second, 10 * time.Second, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			handoverStartTolerance = c.tolerance
			handoverProcStartTime = func(int) (time.Time, bool) {
				return recorded.Add(c.offset), true
			}
			got := verifyHandoverStart(rec)
			if got.Verified != c.wantMatch {
				t.Errorf("verifyHandoverStart with offset %v tolerance %v: Verified = %v, want %v (outcome %+v)",
					c.offset, c.tolerance, got.Verified, c.wantMatch, got)
			}
			if !got.Verified && got.SkipReason != "" {
				t.Errorf("a boundary/mismatch verdict must not carry a skip reason, got %q", got.SkipReason)
			}
			if got.Verified && got.ServerStartedAt != rec.ServerStartedAt {
				t.Errorf("verified outcome should carry the record's raw serverStartedAt, got %q", got.ServerStartedAt)
			}
		})
	}
}

// TestAdoptHandoverSetsState verifies adoption flips the server globals:
// running=true, port set, adoptedPid recorded, serverCmd nil. A legacy record
// without a log path must not start a log tailer, and without a
// serverStartedAt the chained serverTrueStart falls back to adoption time.
func TestAdoptHandoverSetsState(t *testing.T) {
	saveAdoptedState(t)

	adoptHandover(31337, 9090, "", handoverStartCheck{SkipReason: "legacy record without serverStartedAt"})

	serverMu.Lock()
	running, port, adopted, cmd := serverRunning, serverPort, adoptedPid, serverCmd
	startTime := serverStartTime
	trueStart := serverTrueStart
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
	if trueStart.IsZero() {
		t.Error("adopt should stamp serverTrueStart (legacy record falls back to adoption time)")
	}
	if tail != nil {
		t.Error("adopting a legacy record without logPath must not start a log tailer")
	}
}

// TestAdoptHandoverChainsServerStartTime verifies the start-time chaining
// across re-handovers: the record's serverStartedAt becomes serverTrueStart
// (so a subsequent handover re-records the ORIGINAL creation time), while an
// unparseable value falls back to adoption time.
func TestAdoptHandoverChainsServerStartTime(t *testing.T) {
	saveAdoptedState(t)

	original := time.Date(2024, 3, 4, 5, 6, 7, 0, time.UTC)
	adoptHandover(31337, 9090, "", handoverStartCheck{ServerStartedAt: original.Format(time.RFC3339), Verified: true})
	serverMu.Lock()
	trueStart := serverTrueStart
	serverMu.Unlock()
	if !trueStart.Equal(original) {
		t.Errorf("adopt should chain the record's serverStartedAt into serverTrueStart, got %v want %v", trueStart, original)
	}

	// unparseable value → adoption-time fallback (recent, non-zero)
	adoptHandover(31337, 9090, "", handoverStartCheck{ServerStartedAt: "not-a-time", SkipReason: "serverStartedAt unparseable"})
	serverMu.Lock()
	trueStart = serverTrueStart
	serverMu.Unlock()
	if trueStart.IsZero() || time.Since(trueStart) > time.Minute {
		t.Errorf("unparseable serverStartedAt should fall back to adoption time, got %v", trueStart)
	}
}

// TestAdoptHandoverLogReflectsStartCheck verifies the single [OK] adoption
// log line stays truthful: "(start time verified)" only when the check
// actually ran and matched, and the fail-open skip reason is named otherwise.
func TestAdoptHandoverLogReflectsStartCheck(t *testing.T) {
	saveAdoptedState(t)
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	adoptHandover(31337, 9090, "", handoverStartCheck{Verified: true})
	if !strings.Contains(buf.String(), "[OK] Adopted llama-server pid=31337 port=9090 (start time verified)") {
		t.Errorf("verified adoption log line wrong: %q", buf.String())
	}

	buf.Reset()
	adoptHandover(31338, 9091, "", handoverStartCheck{SkipReason: "legacy record without serverStartedAt"})
	if !strings.Contains(buf.String(), "[OK] Adopted llama-server pid=31338 port=9091 (start time check skipped: legacy record without serverStartedAt)") {
		t.Errorf("skipped-check adoption log line wrong: %q", buf.String())
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
// JSON written by an older version (no logPath, no serverStartedAt) parses
// with empty fields, and the decision core adopts it with an empty log path
// (no tailing) and a skipped start-time check instead of rejecting it.
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
	if rec.ServerStartedAt != "" {
		t.Errorf("legacy record without serverStartedAt should yield empty ServerStartedAt, got %q", rec.ServerStartedAt)
	}
	// startMatches modeled as a fail-open skip for legacy records.
	plan := decideHandoverAction(true, rec, true, handoverStartCheck{SkipReason: "legacy record without serverStartedAt"})
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

	adoptHandover(2718, 8080, logPath, handoverStartCheck{SkipReason: "legacy record without serverStartedAt"})

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
	origTrueStart := serverTrueStart
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
		serverTrueStart = origTrueStart
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
// when adopted, and no file when the server is not running. The record also
// carries the running server's real creation time (serverTrueStart).
func TestWriteServerHandover(t *testing.T) {
	withTempCwd(t)
	saveAdoptedState(t)
	setServerTrueStart(t, time.Time{})

	// not running → no file, no error
	if err := writeServerHandover(); err != nil {
		t.Fatalf("writeServerHandover with no server should be nil, got %v", err)
	}
	if _, err := os.Stat(handoverFile); !os.IsNotExist(err) {
		t.Error("no handover file should be written when the server is not running")
	}

	// adopted server → adopted pid + port + chained server start time
	trueStart := time.Date(2025, 6, 1, 12, 30, 45, 0, time.UTC)
	serverMu.Lock()
	serverRunning = true
	serverCmd = nil
	adoptedPid = 1234
	serverPort = 8181
	serverTrueStart = trueStart
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
	if parsed, err := time.Parse(time.RFC3339, rec.ServerStartedAt); err != nil || !parsed.Equal(trueStart) {
		t.Errorf("adopted handover record should carry serverStartedAt %v, got %q (err %v)", trueStart, rec.ServerStartedAt, err)
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
