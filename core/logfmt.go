package core

import (
	"regexp"
	"unicode/utf8"
)

// ─── Log stream normalization (pure) ─────────────────────────────
//
// Normalizes a llama-server's raw stdout/stderr byte stream into display
// pieces. Two jobs, kept pure (no clock, no IO — so they unit-test trivially
// and the caller owns any throttling): strip ANSI control sequences, and turn
// a terminal-style stream ('\n' newlines + '\r' in-place redraws, as progress
// bars emit) into ("line" | "partial", text) events. Ported from the
// FreeToken reference design (daemon/logfmt.py): a '\r' marks the current
// line as displayed-then-overwritten → emitted as a "partial" (the caller may
// throttle these); a '\n' finalizes a "line"; a run of '\r' followed by '\n'
// is ONE newline.

// logPiece is one event emitted by lineAssembler: a finalized "line"
// (newline-terminated) or a "partial" (the current line was drawn then
// overwritten by a carriage-return redraw; the caller may throttle these).
type logPiece struct {
	Kind string // "line" or "partial"
	Text string
}

const (
	pieceLine    = "line"
	piecePartial = "partial"
)

// ansiRe matches CSI/escape sequences: ESC [ ... final-byte, plus a few
// standalone two-char escapes. Enough to clean progress bars and coloured log
// lines without a full terminal emulator (same regex class as the reference).
var ansiRe = regexp.MustCompile(`\x1b(?:[@-Z\\-_]|\[[0-?]*[ -/]*[@-~])`)

// stripANSI removes ANSI/CSI escape sequences from s.
func stripANSI(s string) string {
	return ansiRe.ReplaceAllString(s, "")
}

// lineAssembler feeds raw chunks; it returns finalized display pieces.
// Stateful across chunks (a line may span reads) but deterministic and
// clock-free. It also carries an incomplete trailing UTF-8 sequence across
// Feed calls, so a multibyte rune split by the read boundary is stitched back
// together instead of decoding to replacement-char garbage mid-line (the
// incremental-decode idea from the reference tailer, kept here so Feed itself
// is split-safe).
type lineAssembler struct {
	cur  []byte // current line bytes since the last line/partial emission
	cr   bool   // a bare '\r' was seen; the next char decides newline-vs-redraw
	pend []byte // incomplete trailing UTF-8 sequence held back until more bytes arrive
}

// Feed consumes one chunk and returns the pieces it completed. Chunks may end
// mid-UTF-8-sequence or mid-line; nothing is lost across calls.
func (a *lineAssembler) Feed(chunk string) []logPiece {
	data := chunk
	if len(a.pend) > 0 {
		data = string(a.pend) + chunk
		a.pend = nil
	}
	var out []logPiece
	for i := 0; i < len(data); {
		r, size := utf8.DecodeRuneInString(data[i:])
		if r == utf8.RuneError && size <= 1 {
			if incompleteUTF8Prefix(data[i:]) {
				// Truncated (but otherwise valid) multibyte sequence at the
				// end of this chunk: hold the bytes back for the next Feed.
				a.pend = append(a.pend, data[i:]...)
				break
			}
			// Permanently invalid byte: one replacement char, then move on
			// (mirrors the reference decoder's "replace" error handler).
			a.accept(utf8.RuneError, &out)
			i++
			continue
		}
		a.accept(r, &out)
		i += size
	}
	return out
}

// Flush emits whatever trails after EOF: a dangling partial UTF-8 sequence
// can never complete, so it first decodes to one replacement char (mirrors
// the reference decoder's final=True); then a lone trailing '\r' yields a
// "partial", otherwise any remaining current text yields a final "line".
// After Flush the assembler is back to a clean state.
func (a *lineAssembler) Flush() []logPiece {
	var out []logPiece
	if len(a.pend) > 0 {
		a.pend = nil
		a.accept(utf8.RuneError, &out)
	}
	switch {
	case a.cr:
		a.cr = false
		out = append(out, logPiece{Kind: piecePartial, Text: a.text()})
		a.cur = a.cur[:0]
	case len(a.cur) > 0:
		out = append(out, logPiece{Kind: pieceLine, Text: a.text()})
		a.cur = a.cur[:0]
	}
	return out
}

// accept folds one decoded rune into the assembler state, emitting pieces
// with the reference semantics (ported from logfmt.py LineAssembler.feed).
func (a *lineAssembler) accept(r rune, out *[]logPiece) {
	if r == '\r' {
		// Coalesce a RUN of '\r'. A lone '\r' is a progress-bar redraw
		// (cursor back to column 0); a run is still just "return to column
		// 0", so it collapses to one. This is what makes Windows '\r\r\n'
		// behave like '\r\n': the child's text-mode stdout adds a CR to
		// output that already ended in CRLF, and without coalescing the
		// first '\r' would be misread as a redraw — emitting the real line
		// as a throttled "partial" plus a spurious empty "line" (the phantom
		// blank rows documented in the reference design).
		a.cr = true
		return
	}
	if a.cr {
		a.cr = false
		if r == '\n' {
			// '\r'+…+'\n' — a single newline (covers '\r\n' and '\r\r\n').
			*out = append(*out, logPiece{Kind: pieceLine, Text: a.text()})
			a.cur = a.cur[:0]
			return
		}
		// Bare '\r' run then a real char: the line was drawn, then
		// overwritten by the redraw starting at this char.
		*out = append(*out, logPiece{Kind: piecePartial, Text: a.text()})
		a.cur = a.cur[:0]
	}
	if r == '\n' {
		*out = append(*out, logPiece{Kind: pieceLine, Text: a.text()})
		a.cur = a.cur[:0]
		return
	}
	a.cur = append(a.cur, string(r)...)
}

// text snapshots the current in-progress line (string() copies, so the
// emitted piece stays valid after cur is reused).
func (a *lineAssembler) text() string {
	return string(a.cur)
}

// incompleteUTF8Prefix reports whether b is a non-empty, strictly truncated
// prefix of a valid UTF-8 encoded rune — i.e. a sequence cut off by the read
// boundary — as opposed to permanently invalid bytes.
func incompleteUTF8Prefix(b string) bool {
	if len(b) == 0 || len(b) > 3 {
		return false
	}
	var want int
	switch c := b[0]; {
	case c >= 0xC2 && c <= 0xDF:
		want = 2
	case c >= 0xE0 && c <= 0xEF:
		want = 3
	case c >= 0xF0 && c <= 0xF4:
		want = 4
	default:
		return false // ASCII, or an invalid lead byte (C0/C1/F5..FF)
	}
	if len(b) >= want {
		return false // already a complete rune, not a truncated prefix
	}
	// Byte-indexed on purpose: range over a string would decode each
	// continuation byte to a replacement rune.
	for i := 1; i < len(b); i++ {
		if b[i]&0xC0 != 0x80 {
			return false
		}
	}
	return true
}
