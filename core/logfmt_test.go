package core

import (
	"reflect"
	"testing"
)

// TestStripANSI verifies ANSI/CSI sequence removal: SGR colours, erase and
// cursor-position sequences and standalone two-char escapes are removed,
// while regular text and the \r / \n control characters the line assembler
// depends on are preserved.
func TestStripANSI(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"plain text unchanged", "hello world", "hello world"},
		{"SGR colour stripped", "\x1b[31mred\x1b[0m plain", "red plain"},
		{"erase-line CSI stripped", "\x1b[2Kdrawn", "drawn"},
		{"cursor position CSI stripped", "\x1b[1;2Hx", "x"},
		{"standalone two-char escape stripped", "a\x1bMb", "ab"},
		{"newline and carriage return preserved", "a\r\nb", "a\r\nb"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := stripANSI(c.in); got != c.want {
				t.Errorf("stripANSI(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

// TestLineAssemblerFeed verifies the chunk-boundary-independent line
// semantics ported from the reference design: '\n' finalizes a line, a run of
// '\r' followed by '\n' is ONE newline (covering '\r\n' and the Windows
// text-mode '\r\r\n' phantom-row bug), a '\r' run followed by a real char
// emits the drawn line as a "partial" and starts a new current line, and a
// multibyte UTF-8 sequence split across two Feed calls is stitched back
// together instead of decoding to replacement garbage.
func TestLineAssemblerFeed(t *testing.T) {
	cases := []struct {
		name   string
		chunks []string
		want   []logPiece
	}{
		{"plain LF lines", []string{"one\ntwo\n"},
			[]logPiece{{Kind: pieceLine, Text: "one"}, {Kind: pieceLine, Text: "two"}}},
		{"plain CRLF lines", []string{"one\r\ntwo\r\n"},
			[]logPiece{{Kind: pieceLine, Text: "one"}, {Kind: pieceLine, Text: "two"}}},
		{"CRLF split across chunks", []string{"one\r", "\ntwo\r", "\n"},
			[]logPiece{{Kind: pieceLine, Text: "one"}, {Kind: pieceLine, Text: "two"}}},
		{"lone CR redraw emits partial then new line", []string{"abc\rdef\n"},
			[]logPiece{{Kind: piecePartial, Text: "abc"}, {Kind: pieceLine, Text: "def"}}},
		{"CR run coalesces to one redraw", []string{"a\r\r\rbc\n"},
			[]logPiece{{Kind: piecePartial, Text: "a"}, {Kind: pieceLine, Text: "bc"}}},
		{"windows text-mode CR CRLF yields one line, no phantom row", []string{"hello\r\r\nworld\n"},
			[]logPiece{{Kind: pieceLine, Text: "hello"}, {Kind: pieceLine, Text: "world"}}},
		{"progress bar frames", []string{"10%\r50%\r100%\n"},
			[]logPiece{{Kind: piecePartial, Text: "10%"}, {Kind: piecePartial, Text: "50%"}, {Kind: pieceLine, Text: "100%"}}},
		{"empty LF line emitted raw (ring drops it)", []string{"a\n\nb\n"},
			[]logPiece{{Kind: pieceLine, Text: "a"}, {Kind: pieceLine, Text: ""}, {Kind: pieceLine, Text: "b"}}},
		{"two-byte rune split across feeds", []string{"h\xc3", "\xa9i\n"},
			[]logPiece{{Kind: pieceLine, Text: "héi"}}},
		{"three-byte rune split across feeds", []string{"he\xe4\xb8", "\xad\n"},
			[]logPiece{{Kind: pieceLine, Text: "he中"}}},
		{"four-byte rune split across feeds", []string{"\xf0\x9f", "\x98\x80!\n"},
			[]logPiece{{Kind: pieceLine, Text: "\U0001f600!"}}},
		{"CR redraw before split rune", []string{"x\r\xc3", "\xa9\n"},
			[]logPiece{{Kind: piecePartial, Text: "x"}, {Kind: pieceLine, Text: "é"}}},
		{"invalid byte becomes one replacement char", []string{"a\xffb\n"},
			[]logPiece{{Kind: pieceLine, Text: "a\uFFFDb"}}},
		{"ANSI preserved in pieces (stripping is the caller's job)", []string{"\x1b[31mx\x1b[0m\n"},
			[]logPiece{{Kind: pieceLine, Text: "\x1b[31mx\x1b[0m"}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			asm := &lineAssembler{}
			var got []logPiece
			for _, chunk := range c.chunks {
				got = append(got, asm.Feed(chunk)...)
			}
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("Feed over %q = %+v, want %+v", c.chunks, got, c.want)
			}
		})
	}
}

// TestLineAssemblerFlush verifies EOF behavior: trailing text becomes a final
// line, a lone trailing '\r' becomes a partial (including an empty one, which
// the ring layer drops), a dangling partial UTF-8 sequence flushes as one
// replacement char, and a clean state flushes nothing. Flush resets the
// assembler so a subsequent Flush emits nothing.
func TestLineAssemblerFlush(t *testing.T) {
	cases := []struct {
		name   string
		chunks []string
		want   []logPiece
	}{
		{"trailing text becomes final line", []string{"abc"},
			[]logPiece{{Kind: pieceLine, Text: "abc"}}},
		{"lone CR flushes as partial", []string{"x\r"},
			[]logPiece{{Kind: piecePartial, Text: "x"}}},
		{"CR run with empty current flushes empty partial", []string{"\r\r"},
			[]logPiece{{Kind: piecePartial, Text: ""}}},
		{"CR then dangling rune flushes partial plus replacement line", []string{"\r\xc3"},
			[]logPiece{{Kind: piecePartial, Text: ""}, {Kind: pieceLine, Text: "\uFFFD"}}},
		{"dangling rune flushes as replacement line", []string{"ok\xc3"},
			[]logPiece{{Kind: pieceLine, Text: "ok\uFFFD"}}},
		{"clean state flushes nothing", []string{"a\n"}, nil},
		{"remaining current line flushes", []string{"a\nb"},
			[]logPiece{{Kind: pieceLine, Text: "b"}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			asm := &lineAssembler{}
			for _, chunk := range c.chunks {
				asm.Feed(chunk)
			}
			got := asm.Flush()
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("Flush after %q = %+v, want %+v", c.chunks, got, c.want)
			}
			if again := asm.Flush(); len(again) != 0 {
				t.Errorf("second Flush after %q = %+v, want empty (state must reset)", c.chunks, again)
			}
		})
	}
}
