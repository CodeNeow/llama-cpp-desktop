package core

import (
	"sync"
	"testing"
)

// ─── serverLogWriter line-buffered ─────────────────────────────────────

// resetServerLogs clears the service-log ring buffer (protected by serverLogsMu) and
// restores it to empty after the test, preventing log pollution in writer-related test
// cases from affecting subsequent cases (isolation matches TestAddServerLogRingBuffer).
func resetServerLogs(t *testing.T) {
	t.Helper()
	serverLogsMu.Lock()
	serverLogs = nil
	serverLogsMu.Unlock()
	t.Cleanup(func() {
		serverLogsMu.Lock()
		serverLogs = nil
		serverLogsMu.Unlock()
	})
}

// serverLogsCopy copies the current service logs under the lock for assertion use
// (must not read the global serverLogs directly).
func serverLogsCopy() []string {
	serverLogsMu.Lock()
	defer serverLogsMu.Unlock()
	out := make([]string, len(serverLogs))
	copy(out, serverLogs)
	return out
}

// TestServerLogWriterSingleWriteMultiLine verifies that a single Write containing
// multiple lines is split into independent log entries (each line TrimSpace'd to remove
// leading/trailing whitespace), and Write returns len(p), nil satisfying the io.Writer
// contract (accepts the entire input block).
func TestServerLogWriterSingleWriteMultiLine(t *testing.T) {
	resetServerLogs(t)
	w := &serverLogWriter{}
	p := []byte("line one\nline two\nline three\n")
	n, err := w.Write(p)
	if n != len(p) || err != nil {
		t.Fatalf("Write returned (%d, %v), want (%d, nil)", n, err, len(p))
	}
	got := serverLogsCopy()
	want := []string{"line one", "line two", "line three"}
	if len(got) != len(want) {
		t.Fatalf("log entry count = %d, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("entry[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// TestServerLogWriterSplitWriteReassembles verifies that when a print_timing line is
// split across multiple Writes (llama-server outputs in small chunks, a single line can
// be bisected), the buffer reassembles it into one complete entry.
// This is the core scenario fixed this round: the fragment "( 0.63 ms per token, 2362.80 tokens per second)"
// appearing as a standalone entry no longer carries the "prompt eval time" marker, and
// parseTPS would misinterpret the prefill number as decode speed; line buffering must
// guarantee addServerLog receives a complete line in this scenario.
func TestServerLogWriterSplitWriteReassembles(t *testing.T) {
	resetServerLogs(t)
	w := &serverLogWriter{}

	// first half has no newline terminator: must not produce any log entry, fragment stays in buffer
	first := []byte("I slot print_timing:             eval time =     712.56 ms /    64 tokens (   11.13 ms")
	if n, err := w.Write(first); n != len(first) || err != nil {
		t.Fatalf("first-half Write returned (%d, %v), want (%d, nil)", n, err, len(first))
	}
	if got := serverLogsCopy(); len(got) != 0 {
		t.Fatalf("newline-less fragment must not produce entries, got %v", got)
	}

	// second half completes the newline: fragment + second half reassemble into one complete line
	w.Write([]byte(" per token,    89.82 tokens per second)\n"))
	got := serverLogsCopy()
	if len(got) != 1 {
		t.Fatalf("log entry count = %d, want 1: %v", len(got), got)
	}
	want := "I slot print_timing:             eval time =     712.56 ms /    64 tokens (   11.13 ms per token,    89.82 tokens per second)"
	if got[0] != want {
		t.Errorf("reassembled line = %q, want %q", got[0], want)
	}
}

// TestServerLogWriterSkipsBlankLines verifies empty lines and whitespace-only lines
// (including \t) are skipped and do not produce log entries, preventing the ring buffer
// from being polluted by blank lines.
func TestServerLogWriterSkipsBlankLines(t *testing.T) {
	resetServerLogs(t)
	w := &serverLogWriter{}
	w.Write([]byte("a\n\n   \n\t\nb\n"))
	got := serverLogsCopy()
	want := []string{"a", "b"}
	if len(got) != len(want) {
		t.Fatalf("log entry count = %d, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("entry[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// TestServerLogWriterTrailingFragmentRetained verifies a trailing fragment without a
// newline terminator is kept in the buffer without producing an entry; after the next
// Write completes it, the two halves reassemble into one complete line (not discarded, not prematurely
// persisted).
func TestServerLogWriterTrailingFragmentRetained(t *testing.T) {
	resetServerLogs(t)
	w := &serverLogWriter{}
	frag := []byte("tail fragment")
	if n, err := w.Write(frag); n != len(frag) || err != nil {
		t.Fatalf("fragment Write returned (%d, %v), want (%d, nil)", n, err, len(frag))
	}
	if got := serverLogsCopy(); len(got) != 0 {
		t.Fatalf("fragment must not produce entries, got %v", got)
	}
	w.Write([]byte(" completed\n"))
	got := serverLogsCopy()
	if len(got) != 1 || got[0] != "tail fragment completed" {
		t.Errorf("after completion there should be one full line, got %v", got)
	}
}

// TestServerLogWriterConcurrentWrite verifies concurrent Write does not panic and does
// not drop lines: 50 goroutines each write one 3-line block (each block is atomically
// processed under the lock, lines do not interleave across blocks), ring buffer capacity
// 200 (150 < 200, no truncation), total log count should be 150 and every entry is one
// of the three expected lines.
func TestServerLogWriterConcurrentWrite(t *testing.T) {
	resetServerLogs(t)
	w := &serverLogWriter{}
	const goroutines = 50
	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := w.Write([]byte("c1 line\nc2 line\nc3 line\n")); err != nil {
				t.Errorf("concurrent Write returned error: %v", err)
			}
		}()
	}
	wg.Wait()

	got := serverLogsCopy()
	if len(got) != goroutines*3 {
		t.Fatalf("log entry count = %d, want %d (no dropped lines)", len(got), goroutines*3)
	}
	valid := map[string]bool{"c1 line": true, "c2 line": true, "c3 line": true}
	for _, line := range got {
		if !valid[line] {
			t.Errorf("unexpected line appeared: %q", line)
		}
	}
}
