package core

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

// referenceFileMode returns the os.Stat mode a file prepared with perm reports
// on the current OS. Exact numeric perms are not portable assertions (Windows
// chmod only toggles the read-only bit and Stat reports 0666/0444 for
// writable/read-only files), so tests compare atomicWriteFile's result against
// a reference file prepared with the same perm via plain os calls.
func referenceFileMode(t *testing.T, dir string, perm os.FileMode) os.FileMode {
	t.Helper()
	ref := filepath.Join(dir, "reference")
	if err := os.WriteFile(ref, nil, perm); err != nil {
		t.Fatalf("write reference file: %v", err)
	}
	// Explicit chmod makes the reference umask-independent, matching the
	// umask-independent chmod inside atomicWriteFile.
	if err := os.Chmod(ref, perm); err != nil {
		t.Fatalf("chmod reference file: %v", err)
	}
	fi, err := os.Stat(ref)
	if err != nil {
		t.Fatalf("stat reference file: %v", err)
	}
	return fi.Mode()
}

// tempLitter returns leftover atomicWriteFile temp files (names containing
// ".tmp-") in dir; it must be empty after both success and failure.
func tempLitter(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	var litter []string
	for _, e := range entries {
		if strings.Contains(e.Name(), ".tmp-") {
			litter = append(litter, e.Name())
		}
	}
	return litter
}

// TestAtomicWriteFileCreatesNewFile verifies a fresh write lands with the
// exact content and the requested mode, and leaves no temp litter.
func TestAtomicWriteFileCreatesNewFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	data := []byte("{\"theme\":\"dark\"}")
	if err := atomicWriteFile(path, data, 0644); err != nil {
		t.Fatalf("atomicWriteFile failed: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read written file: %v", err)
	}
	if string(got) != string(data) {
		t.Errorf("content mismatch: got %q, want %q", got, data)
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat written file: %v", err)
	}
	if want := referenceFileMode(t, dir, 0644); fi.Mode() != want {
		t.Errorf("mode mismatch: got %v, want %v", fi.Mode(), want)
	}
	if litter := tempLitter(t, dir); len(litter) > 0 {
		t.Errorf("temp litter left after success: %v", litter)
	}
}

// TestAtomicWriteFileOverwritesExistingFile verifies an overwrite replaces the
// content and updates the mode to the requested perm. Plain os.WriteFile keeps
// an existing file's old mode; atomicWriteFile writes a fresh temp file chmod'ed
// to the requested perm, so a restricted old mode is rewritten. The old mode is
// first restricted to 0600 to prove the mode is actually updated, not just
// preserved (on Windows the read-only-bit-only chmod granularity makes the
// transition unobservable, hence the GOOS-guarded exact check).
func TestAtomicWriteFileOverwritesExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte("old"), 0644); err != nil {
		t.Fatalf("seed existing file: %v", err)
	}
	if err := os.Chmod(path, 0600); err != nil {
		t.Fatalf("chmod existing file: %v", err)
	}
	data := []byte("{\"theme\":\"light\"}")
	if err := atomicWriteFile(path, data, 0644); err != nil {
		t.Fatalf("atomicWriteFile failed: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read overwritten file: %v", err)
	}
	if string(got) != string(data) {
		t.Errorf("content mismatch: got %q, want %q", got, data)
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat overwritten file: %v", err)
	}
	if want := referenceFileMode(t, dir, 0644); fi.Mode() != want {
		t.Errorf("mode mismatch after overwrite: got %v, want %v", fi.Mode(), want)
	}
	if runtime.GOOS != "windows" && fi.Mode().Perm() != 0o644 {
		t.Errorf("perm not updated to 0644: got %v", fi.Mode().Perm())
	}
	if litter := tempLitter(t, dir); len(litter) > 0 {
		t.Errorf("temp litter left after success: %v", litter)
	}
}

// TestAtomicWriteFileRenameFailureKeepsOriginalAndCleansTemp verifies the
// failure path: when the atomic swap fails, the original file must stay intact
// and the temp file must be removed (no *.tmp-* litter). The swap is forced to
// fail through the atomicRename injection point (same style as renameFile /
// killProcessByPid / updateLauncher).
func TestAtomicWriteFileRenameFailureKeepsOriginalAndCleansTemp(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	original := []byte("original content")
	if err := os.WriteFile(path, original, 0644); err != nil {
		t.Fatalf("seed original file: %v", err)
	}
	var renameSrc string
	origRename := atomicRename
	atomicRename = func(from, to string) error {
		renameSrc = from
		return errors.New("simulated rename failure")
	}
	defer func() { atomicRename = origRename }()

	if err := atomicWriteFile(path, []byte("new content"), 0644); err == nil {
		t.Fatal("atomicWriteFile must fail when the rename step fails")
	}
	// Original content intact: a failed swap must not clobber the target.
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read original file after failed write: %v", err)
	}
	if string(got) != string(original) {
		t.Errorf("original file clobbered: got %q, want %q", got, original)
	}
	// The temp file must be created in the SAME directory as the target with
	// the documented naming (dot prefix + ".tmp-" + unique suffix), never a
	// fixed name (concurrent writers would clobber each other's temp).
	if renameSrc == "" {
		t.Fatal("rename injection point was not called")
	}
	if d := filepath.Dir(renameSrc); d != dir {
		t.Errorf("temp file not created in the target directory: got %q, want %q", d, dir)
	}
	if base := filepath.Base(renameSrc); !strings.HasPrefix(base, "."+filepath.Base(path)+".tmp-") {
		t.Errorf("unexpected temp file naming: %q", base)
	}
	if litter := tempLitter(t, dir); len(litter) > 0 {
		t.Errorf("temp litter left after failure: %v", litter)
	}
}

// TestAtomicWriteFileConcurrentSameTarget drives concurrent atomicWriteFile
// calls on the SAME target, mirroring the production overlap (concurrent
// SaveModelConfig / persistTasksNow both end in saveConfig). On Windows,
// concurrent MoveFileEx(REPLACE_EXISTING) swaps on one destination transiently
// fail with ERROR_ACCESS_DENIED (Errno 5) / ERROR_SHARING_VIOLATION (Errno 32)
// while the previous winner's handle on the target is released; the bounded
// transient-error retry inside atomicWriteFile must absorb them. Asserts every
// call succeeds and the final content is exactly one of the written payloads
// (never torn or mixed). Kept under ~2s: 8 writers x 20 rounds of rapid-fire
// renames give the race room to fire without slowing the suite.
func TestAtomicWriteFileConcurrentSameTarget(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	const writers = 8
	const rounds = 20
	payloads := make([][]byte, writers)
	for w := range payloads {
		payloads[w] = []byte(fmt.Sprintf(`{"writer":%d}`, w))
	}
	var wg sync.WaitGroup
	errCh := make(chan error, writers)
	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for r := 0; r < rounds; r++ {
				if err := atomicWriteFile(path, payloads[w], 0644); err != nil {
					errCh <- fmt.Errorf("writer %d round %d: %w", w, r, err)
					return
				}
			}
		}(w)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Errorf("concurrent atomicWriteFile failed: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read final file: %v", err)
	}
	found := false
	for _, p := range payloads {
		if string(got) == string(p) {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("final content is not one of the written payloads: %q", got)
	}
	if litter := tempLitter(t, dir); len(litter) > 0 {
		t.Errorf("temp litter left after concurrent writes: %v", litter)
	}
}
