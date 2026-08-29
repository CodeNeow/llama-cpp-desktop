package core

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// ─── Control plane endpoint tests (httptest, no real bind) ───────
//
// Requests go through controlPlaneMux with hand-set RemoteAddr values, so the
// loopback/foreign-address guards are exercised without binding port 1900.

// controlRequest runs one request against the mux with the given remote
// address and headers and returns the recorded response.
func controlRequest(mux *http.ServeMux, method, target, remoteAddr string, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, nil)
	if remoteAddr != "" {
		req.RemoteAddr = remoteAddr
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

// decodeControlJSON decodes a response body into v, failing the test on a
// non-2xx status or a decode error.
func decodeControlJSON(t *testing.T, rec *httptest.ResponseRecorder, wantStatus int, v interface{}) {
	t.Helper()
	if rec.Code != wantStatus {
		t.Fatalf("status = %d, want %d (body: %s)", rec.Code, wantStatus, rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), v); err != nil {
		t.Fatalf("decode response %q: %v", rec.Body.String(), err)
	}
}

// TestControlPlaneHealth verifies /health always answers 200, never gated —
// even when the shared token env var is set.
func TestControlPlaneHealth(t *testing.T) {
	mux := controlPlaneMux()

	// without token env
	var body map[string]interface{}
	decodeControlJSON(t, controlRequest(mux, http.MethodGet, "/health", "", nil), http.StatusOK, &body)
	if body["status"] != "ok" || body["headless"] != true {
		t.Errorf("health body = %v, want {status:ok, headless:true}", body)
	}

	// with token env set: still ungated
	t.Setenv(controlPlaneTokenEnv, "secret")
	decodeControlJSON(t, controlRequest(mux, http.MethodGet, "/health", "", nil), http.StatusOK, &body)
	if body["status"] != "ok" || body["headless"] != true {
		t.Errorf("health body with token env = %v, want {status:ok, headless:true}", body)
	}
}

// controlStatusBody mirrors the GET /status response for decoding.
type controlStatusBody struct {
	Running bool    `json:"running"`
	Port    int     `json:"port"`
	PID     int     `json:"pid"`
	Adopted bool    `json:"adopted"`
	UptimeS float64 `json:"uptimeS"`
	Version string  `json:"version"`
}

// TestControlPlaneStatus verifies the /status snapshot: running state, port,
// pid (adopted when there is no child handle), uptime and the embedded app
// version, plus the GET-only and token guards.
func TestControlPlaneStatus(t *testing.T) {
	saveAdoptedState(t)
	mux := controlPlaneMux()

	// adopted server: running=true, no child handle, pid from adoptedPid
	serverMu.Lock()
	serverRunning = true
	serverCmd = nil
	adoptedPid = 4321
	serverPort = 8080
	serverStartTime = time.Now().Add(-2 * time.Second)
	serverMu.Unlock()

	var body controlStatusBody
	decodeControlJSON(t, controlRequest(mux, http.MethodGet, "/status", "", nil), http.StatusOK, &body)
	if !body.Running || body.Port != 8080 || body.PID != 4321 || !body.Adopted {
		t.Errorf("status = %+v, want running/port 8080/pid 4321/adopted", body)
	}
	if body.UptimeS <= 0 || body.UptimeS > 60 {
		t.Errorf("uptimeS = %v, want ~2", body.UptimeS)
	}
	if body.Version != currentVersion {
		t.Errorf("version = %q, want %q (same source GetAppVersion uses)", body.Version, currentVersion)
	}

	// not running: zeroed pid/port/uptime
	serverMu.Lock()
	serverRunning = false
	adoptedPid = 0
	serverPort = 0
	serverStartTime = time.Time{}
	serverMu.Unlock()
	decodeControlJSON(t, controlRequest(mux, http.MethodGet, "/status", "", nil), http.StatusOK, &body)
	if body.Running || body.PID != 0 || body.Port != 0 || body.Adopted || body.UptimeS != 0 {
		t.Errorf("stopped status = %+v, want zeroed snapshot", body)
	}

	// non-GET → 405
	if rec := controlRequest(mux, http.MethodPost, "/status", "", nil); rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST /status = %d, want 405", rec.Code)
	}

	// token gating: env set → 401 missing, 403 wrong, 200 correct
	t.Setenv(controlPlaneTokenEnv, "secret")
	if rec := controlRequest(mux, http.MethodGet, "/status", "", nil); rec.Code != http.StatusUnauthorized {
		t.Errorf("missing token = %d, want 401", rec.Code)
	}
	if rec := controlRequest(mux, http.MethodGet, "/status", "", map[string]string{"X-Control-Token": "wrong"}); rec.Code != http.StatusForbidden {
		t.Errorf("wrong token = %d, want 403", rec.Code)
	}
	decodeControlJSON(t, controlRequest(mux, http.MethodGet, "/status", "", map[string]string{"X-Control-Token": "secret"}), http.StatusOK, &body)
}

// TestControlPlaneLogs verifies the /logs proxy over the Part-1 ring: seeded
// entries page by cursor, since filtering matches serverLogsSince, and the
// 405/400 guards fire.
func TestControlPlaneLogs(t *testing.T) {
	resetServerLogs(t)
	mux := controlPlaneMux()
	addServerLog("alpha")
	addServerLog("beta")
	addServerLog("gamma")

	// full fetch (since 0): everything retained + next cursor
	var page ServerLogsPage
	decodeControlJSON(t, controlRequest(mux, http.MethodGet, "/logs?since=0", "", nil), http.StatusOK, &page)
	if len(page.Entries) != 3 || page.Next != 3 {
		t.Fatalf("logs since 0 = %+v, want 3 entries / next 3", page)
	}
	if page.Entries[0].Seq != 0 || page.Entries[0].Text != "alpha" || page.Entries[2].Text != "gamma" {
		t.Errorf("entries = %+v, want seq 0..2 alpha/beta/gamma", page.Entries)
	}

	// mid cursor: suffix only
	decodeControlJSON(t, controlRequest(mux, http.MethodGet, "/logs?since=2", "", nil), http.StatusOK, &page)
	if len(page.Entries) != 1 || page.Entries[0].Seq != 2 || page.Entries[0].Text != "gamma" || page.Next != 3 {
		t.Errorf("logs since 2 = %+v, want seq 2/gamma with next 3", page)
	}

	// future cursor: empty page
	decodeControlJSON(t, controlRequest(mux, http.MethodGet, "/logs?since=99", "", nil), http.StatusOK, &page)
	if len(page.Entries) != 0 || page.Next != 3 {
		t.Errorf("logs since 99 = %+v, want 0 entries / next 3", page)
	}

	// missing since defaults to 0
	decodeControlJSON(t, controlRequest(mux, http.MethodGet, "/logs", "", nil), http.StatusOK, &page)
	if len(page.Entries) != 3 || page.Next != 3 {
		t.Errorf("logs without since = %+v, want 3 entries / next 3", page)
	}

	// unparsable since → 400
	if rec := controlRequest(mux, http.MethodGet, "/logs?since=abc", "", nil); rec.Code != http.StatusBadRequest {
		t.Errorf("logs since=abc = %d, want 400", rec.Code)
	}

	// non-GET → 405
	if rec := controlRequest(mux, http.MethodPost, "/logs", "", nil); rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST /logs = %d, want 405", rec.Code)
	}
}

// TestControlPlaneStop verifies the destructive endpoint's guards in order
// (POST-only → loopback → token) and that the happy path answers immediately
// with {stopping:true} while the actual stop runs asynchronously (it is a
// no-op here: no server is running).
func TestControlPlaneStop(t *testing.T) {
	saveAdoptedState(t)
	serverMu.Lock()
	serverRunning = false
	serverCmd = nil
	adoptedPid = 0
	serverMu.Unlock()

	mux := controlPlaneMux()
	loopback := "127.0.0.1:12345"
	remote := "192.168.1.5:12345"

	// non-POST → 405
	if rec := controlRequest(mux, http.MethodGet, "/stop", loopback, nil); rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET /stop = %d, want 405", rec.Code)
	}

	// non-loopback remote → 403 (independent of the token: env unset here)
	if rec := controlRequest(mux, http.MethodPost, "/stop", remote, nil); rec.Code != http.StatusForbidden {
		t.Errorf("remote /stop = %d, want 403", rec.Code)
	}

	// happy path (token unset → loopback bind is the only gate)
	var body map[string]interface{}
	decodeControlJSON(t, controlRequest(mux, http.MethodPost, "/stop", loopback, nil), http.StatusOK, &body)
	if body["stopping"] != true {
		t.Errorf("stop body = %v, want {stopping:true}", body)
	}
	time.Sleep(50 * time.Millisecond) // let the async stop goroutine finish (no-op)

	// token set: loopback caller still needs the header
	t.Setenv(controlPlaneTokenEnv, "secret")
	if rec := controlRequest(mux, http.MethodPost, "/stop", loopback, nil); rec.Code != http.StatusUnauthorized {
		t.Errorf("missing token /stop = %d, want 401", rec.Code)
	}
	if rec := controlRequest(mux, http.MethodPost, "/stop", loopback, map[string]string{"X-Control-Token": "wrong"}); rec.Code != http.StatusForbidden {
		t.Errorf("wrong token /stop = %d, want 403", rec.Code)
	}
	decodeControlJSON(t, controlRequest(mux, http.MethodPost, "/stop", loopback, map[string]string{"X-Control-Token": "secret"}), http.StatusOK, &body)
	if body["stopping"] != true {
		t.Errorf("stop body with token = %v, want {stopping:true}", body)
	}
	time.Sleep(50 * time.Millisecond)
}

// TestControlPlaneUnknownPath verifies the catch-all answers JSON 404.
func TestControlPlaneUnknownPath(t *testing.T) {
	mux := controlPlaneMux()
	var body map[string]string
	decodeControlJSON(t, controlRequest(mux, http.MethodGet, "/nope", "", nil), http.StatusNotFound, &body)
	if body["error"] != "not found" {
		t.Errorf("body = %v, want {error:not found}", body)
	}
}

// TestControlGuardPanicRecovery verifies a panicking handler becomes a 500
// JSON response instead of crashing the headless process (the FreeToken
// crash-proof rule).
func TestControlGuardPanicRecovery(t *testing.T) {
	boom := controlGuard(func(w http.ResponseWriter, r *http.Request) {
		panic(fmt.Errorf("boom"))
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/boom", nil)
	boom(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("panic handler status = %d, want 500", rec.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode panic response %q: %v", rec.Body.String(), err)
	}
	if body["error"] != "internal error" {
		t.Errorf("panic body = %v, want {error:internal error}", body)
	}
}
