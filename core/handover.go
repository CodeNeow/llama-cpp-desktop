package core

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
)

// ─── Server handover file ─────────────────────────────────────────
//
// When switching between GUI and headless (API-route) mode the app relaunches
// itself and exits. To keep llama-server running uninterrupted across the
// switch, the exiting process writes a "handover record" (pid + port) next to
// the config file; the new process probes the record and adopts the still
// running llama-server instead of starting a new one.

// handoverFile is the handover-record path, declared as a var (same style as
// configFile) and resolved cwd-relative like the config file.
var handoverFile = "llama-desktop-server-handover.json"

// handoverRecord is the JSON payload of the handover file: the llama-server
// child pid, the port it listens on, when the record was written, and the
// absolute path of the server log file. LogPath is omitempty and may be
// missing entirely in records written by older versions; readHandover must
// accept such records (an empty LogPath adopts without log tailing).
type handoverRecord struct {
	Pid       int    `json:"pid"`
	Port      int    `json:"port"`
	StartedAt string `json:"startedAt"`
	LogPath   string `json:"logPath,omitempty"`
}

// writeHandover persists the handover record (pid/port, RFC3339 timestamp,
// absolute server log path — the successor must not depend on our cwd).
func writeHandover(pid, port int) error {
	rec := handoverRecord{
		Pid:       pid,
		Port:      port,
		StartedAt: time.Now().Format(time.RFC3339),
		LogPath:   absServerLogPath(),
	}
	data, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal handover record: %w", err)
	}
	if err := atomicWriteFile(handoverFile, data, 0644); err != nil {
		return fmt.Errorf("write handover file: %w", err)
	}
	return nil
}

// readHandover loads the handover record. A missing file and a corrupt file
// are both errors; callers distinguish missing via errors.Is(err, fs.ErrNotExist)
// and treat anything else as a stale record (delete + start fresh).
func readHandover() (*handoverRecord, error) {
	data, err := os.ReadFile(handoverFile)
	if err != nil {
		return nil, err
	}
	var rec handoverRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		return nil, fmt.Errorf("parse handover file: %w", err)
	}
	return &rec, nil
}

// removeHandover deletes the handover record; a missing file is not an error.
func removeHandover() error {
	if err := os.Remove(handoverFile); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove handover file: %w", err)
	}
	return nil
}

// probeHandoverHealth is the injection point for the llama-server liveness
// probe: the default implementation GETs /health on 127.0.0.1:port (base URL
// shared with the router wrapper) and counts any HTTP response — status code
// irrelevant — as healthy; a connection failure means no server on that port.
// Tests replace this variable instead of opening sockets.
var probeHandoverHealth = func(port int) bool {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(routerBaseURL(port) + "/health")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return true
}

// handoverPidAlive is the injection point for the pid-liveness check
// (platform-specific processAlive implementation; tests fake this variable).
var handoverPidAlive = func(pid int) bool {
	return processAlive(pid)
}

// handoverPlan describes what a starting process (GUI or headless) should do
// with a possibly handed-over llama-server.
type handoverPlan struct {
	// Adopt reports the handed-over llama-server is alive and should be
	// adopted (PID/Port carry its identity) instead of starting a new one.
	Adopt bool
	PID   int
	Port  int
	// LogPath carries the record's server log file path ("" in legacy records
	// without logPath → adopt without log tailing).
	LogPath string
	// RemoveFile reports the handover record is stale (server dead or file
	// corrupt) and must be deleted before a fresh server is started.
	RemoveFile bool
}

// decideHandoverAction is the pure decision core of the handover takeover:
//   - no record file            → start fresh (nothing to delete);
//   - record present + healthy  → adopt (pid/port/logPath);
//   - record present + unhealthy
//     (or unparsable, rec == nil) → delete the record and start fresh.
func decideHandoverAction(fileExists bool, rec *handoverRecord, healthy bool) handoverPlan {
	switch {
	case !fileExists:
		return handoverPlan{}
	case rec == nil || !healthy:
		return handoverPlan{RemoveFile: true}
	default:
		return handoverPlan{Adopt: true, PID: rec.Pid, Port: rec.Port, LogPath: rec.LogPath}
	}
}

// evaluateHandover reads the handover file, probes the recorded server and
// returns the plan for the starting process (see decideHandoverAction).
func evaluateHandover() handoverPlan {
	rec, err := readHandover()
	if err != nil {
		if os.IsNotExist(err) {
			return decideHandoverAction(false, nil, false)
		}
		// Corrupt file: exists but unreadable/unparsable → stale record.
		return decideHandoverAction(true, nil, false)
	}
	healthy := probeHandoverHealth(rec.Port) && handoverPidAlive(rec.Pid)
	return decideHandoverAction(true, rec, healthy)
}

// adoptHandover marks a handed-over llama-server as the running server:
// serverRunning=true / serverPort set / adoptedPid recorded / serverCmd nil
// (the child belongs to the previous, exited process — there is no handle to
// Signal/Wait, stopServerInternal kills it by pid instead). logPath is the
// server log file from the handover record: when non-empty and present, it
// becomes the log source and a tailer reading from EOF feeds the ring — the
// adopted child keeps writing to the same file, restoring log capture that
// pipes could not provide. An empty logPath (legacy record) adopts without
// tailing.
func adoptHandover(pid, port int, logPath string) {
	serverMu.Lock()
	serverRunning = true
	serverPort = port
	adoptedPid = pid
	serverCmd = nil
	serverStartTime = time.Now()
	serverMu.Unlock()
	log.Printf("[OK] Adopted llama-server pid=%d port=%d", pid, port)
	adoptServerLogTail(logPath)
}

// adoptServerLogTail points the log capture at an adopted server's log file
// and starts a tailer reading from EOF (never replaying stale content). A
// missing or empty path is tolerated (legacy records, already-deleted files)
// — adoption itself must not fail over log capture.
func adoptServerLogTail(logPath string) {
	if logPath == "" {
		return
	}
	if _, err := os.Stat(logPath); err != nil {
		log.Printf("[WARN] adopted server log file %s not readable, log tailing skipped: %v", logPath, err)
		return
	}
	t, err := startServerLogTailer(logPath, false)
	if err != nil {
		log.Printf("[WARN] adopted server log tailer failed to start: %v", err)
		return
	}
	serverMu.Lock()
	serverLogFile = logPath
	// Defensive: a previous tailer (if any) must not double-append into the
	// ring alongside the new one.
	old := serverLogTail
	serverLogTail = t
	serverMu.Unlock()
	if old != nil {
		old.Stop()
	}
	log.Printf("[OK] Tailing adopted server log: %s", logPath)
}
