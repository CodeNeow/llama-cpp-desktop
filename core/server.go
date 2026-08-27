package core

import (
	"bytes"
	"log"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// ─── llama-server lifecycle state ───────────────────────────────
// Process handle, log ring buffer and the line-reassembling log writer for the
// llama-server child process; start/stop logic lives in bridge.go.

var serverCmd *exec.Cmd
var serverLogs []string
var serverLogsMu sync.Mutex
var serverRunning bool

// serverMu guards the full lifecycle of serverCmd and serverRunning
// (create/start/stop/cleanup), separate in responsibility from serverLogsMu
// which only guards serverLogs (#3). Any path holding both locks must acquire
// them in the order "serverMu first, then serverLogsMu" to avoid deadlock.
var serverMu sync.Mutex

// serverStartTime records when llama-server started successfully (guarded by
// serverMu), used by GetMonitorStatus to compute uptime; zeroed when the
// process exits (in the cmd.Wait goroutine).
var serverStartTime time.Time

// serverPort records the port used by the successfully started llama-server
// (guarded by serverMu), 0 means not running. Router API queries use this
// value instead of the current config, so editing the config mid-run cannot
// redirect queries to the wrong address.
var serverPort int

// serverLogWriter reassembles child-process stdout/stderr writes into whole
// lines before they enter the ring log, preventing log entries from being cut
// in half by arbitrary chunks. Previously Write treated each arbitrary stderr
// chunk as one log entry (addServerLog(strings.TrimSpace(string(p)))):
// llama-server writes in small chunks, so a print_timing line could be split
// across multiple Writes — user-pasted logs showed "0.00.136.078" appearing as
// a standalone entry, and fragments like "( 0.63 ms per token, 2362.80 tokens
// per second)" reduced to the second half; the latter no longer contains the
// "prompt eval time" marker, so parseTPS cannot classify it as a prefill line
// and long-prompt prefill speeds like 2362.80 leaked into TPS. Line buffering
// makes addServerLog always receive complete lines, eliminating truncation at
// the root (the log line-splitting and the TPS misreading share the same
// cause).
//
// Each instance holds its own buffer and mutex (the project's "explicit mutex"
// convention); in bridge.go, cmd.Stdout and cmd.Stderr each get their own
// instance buffering its own stream without interference.
type serverLogWriter struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (w *serverLogWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.buf.Write(p)
	for {
		line, err := w.buf.ReadString('\n')
		if err != nil {
			// No trailing newline: ReadString has already consumed the whole
			// buffer and line is an unfinished fragment. Reset and write the
			// fragment back so the next Write completes the line; with no
			// fragment just reset, preventing the buffer from growing without
			// bound on consumed bytes.
			w.buf.Reset()
			if line != "" {
				w.buf.WriteString(line)
			}
			break
		}
		if line = strings.TrimSpace(line); line != "" {
			addServerLog(line)
		}
	}
	return len(p), nil
}

func addServerLog(msg string) {
	serverLogsMu.Lock()
	serverLogs = append(serverLogs, msg)
	if len(serverLogs) > 200 {
		serverLogs = serverLogs[len(serverLogs)-100:]
	}
	serverLogsMu.Unlock()
	log.Println("[llama-server]", msg)
}

// effectiveHost derives the actual listen address from the access scope:
// lan → "0.0.0.0", any other value (including empty and invalid) →
// "127.0.0.1". Pure function shared by SaveServerConfig normalization,
// loadConfig compatibility, and buildServerCommand, keeping Host consistent
// everywhere.
func effectiveHost(mode string) string {
	if mode == accessLAN {
		return "0.0.0.0"
	}
	return "127.0.0.1"
}
