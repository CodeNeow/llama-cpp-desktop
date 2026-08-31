package core

import (
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

// ─── parsePromptTPS ────────────────────────────────────────────────

// TestParsePromptTPS verifies prefill (prompt eval) speed extraction from service logs:
// only lines containing "prompt eval time" are used for prefill speed; decode lines
// (eval time) are excluded from this metric; when multiple lines/values exist, the last
// prefill line is used (most recent sample wins); no prefill lines returns 0.
// Real-world sample: `prompt eval time = 357.49 ms / 27 tokens ( 75.53 tokens per second)`.
func TestParsePromptTPS(t *testing.T) {
	cases := []struct {
		name string
		logs []string
		want float64
	}{
		{"no match", []string{"loading model", "another line"}, 0},
		{"single prefill",
			[]string{"I slot print_timing:      prompt eval time =     357.49 ms /    27 tokens ( 75.53 tokens per second)"},
			75.53},
		// prefill coexisting with decode: decode value (72.97) must not enter prefill metric; still use prefill value 75.53.
		{"prefill with decode coexist",
			[]string{
				"I slot print_timing:      prompt eval time =     357.49 ms /    27 tokens ( 75.53 tokens per second)",
				"I slot print_timing:             eval time =   12334.07 ms /   900 tokens ( 72.97 tokens per second)",
			}, 75.53},
		// multiple rounds of generation with multiple timing pairs: take the last prefill line (long prompt high value 2362.80).
		{"multiple rounds take last prefill",
			[]string{
				"I slot print_timing:      prompt eval time =     357.49 ms /    27 tokens ( 75.53 tokens per second)",
				"llama_server: unrelated log noise",
				"I slot print_timing:      prompt eval time =    4230.00 ms / 10000 tokens ( 2362.80 tokens per second)",
			}, 2362.80},
		// decode-only lines, no prefill lines: prefill metric has no value, returns 0.
		{"decode only",
			[]string{
				"I slot print_timing:             eval time =   12334.07 ms /   900 tokens ( 72.97 tokens per second)",
			}, 0},
		{"empty", nil, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := parsePromptTPS(c.logs); got != c.want {
				t.Errorf("parsePromptTPS(%v) = %v, want %v", c.logs, got, c.want)
			}
		})
	}
}

// TestParsePromptTPSMultiLineEntry verifies correct line splitting when a single entry
// contains multiple lines (one stderr Write contains both prefill and decode lines):
// the decode line does not enter the prefill metric; returns prefill value 75.53.
func TestParsePromptTPSMultiLineEntry(t *testing.T) {
	got := parsePromptTPS([]string{
		"I slot print_timing:      prompt eval time =     357.49 ms /    27 tokens ( 75.53 tokens per second)\n" +
			"I slot print_timing:             eval time =   12334.07 ms /   900 tokens ( 72.97 tokens per second)",
	})
	if got != 75.53 {
		t.Errorf("parsePromptTPS multi-line entry = %v, want 75.53", got)
	}
}

// TestParsePromptTPSNewFormat verifies that both types of prefill log lines in the new
// llama.cpp format can have their values extracted:
// real-time lines printed batch-wise during prefill (prompt processing, only appears when
// prefill takes >=3s) and the final line at request end (prompt eval time, present in both
// old and new versions). With multiple lines, the last one wins: real-time lines refresh
// continuously, and the final line appears last in log order, so its value is the
// authoritative final value.
func TestParsePromptTPSNewFormat(t *testing.T) {
	progressLine := "I slot print_timing: id  3 | task 0 | prompt processing, n_tokens =   2048, progress = 0.16, t =  20.47 s / 100.05 tokens per second"
	progressLine2 := "I slot print_timing: id  3 | task 0 | prompt processing, n_tokens =   1024, progress = 0.51, t =  10.48 s / 97.89 tokens per second"
	finalLine := "I slot print_timing: id  3 | task 0 | prompt eval time =    357.49 ms /     27 tokens (   75.53 tokens per second)"

	cases := []struct {
		name string
		logs []string
		want float64
	}{
		// new-format real-time line: tpsLogRegex extracts "100.05 tokens per second".
		{"new format real-time line", []string{progressLine}, 100.05},
		// multiple progress lines: take the last one (real-time lines refresh continuously, latest sample is 97.89).
		{"multiple progress lines take last", []string{progressLine, progressLine2}, 97.89},
		// final line appears after progress lines: final value 75.53 is the authoritative one (overrides previous real-time values).
		{"final line wins after progress", []string{progressLine, progressLine2, finalLine}, 75.53},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := parsePromptTPS(c.logs); got != c.want {
				t.Errorf("parsePromptTPS(%v) = %v, want %v", c.logs, got, c.want)
			}
		})
	}
}

// ─── parseDecodeTPS ────────────────────────────────────────────────

// TestParseDecodeTPS verifies generation (decode) real-time speed extraction from service
// logs: the real-time line `n_decoded = 414, tg = 68.82 t/s, tg_3s = 67.32 t/s` uses
// tg_3s only (3-second window real-time value 67.32), not tg (cumulative average 68.82);
// tg_3s takes priority over eval-time fallback (real-time value wins even when the eval
// time line appears after it); when no tg line exists, fall back to eval time line value;
// prefill lines (prompt eval time contains the "eval time" substring) must be excluded
// first and not enter candidates; no candidates returns 0.
// Real-world sample: decode total timing `eval time = 12334.07 ms / 900 tokens ( 72.97 tokens
// per second)`.
func TestParseDecodeTPS(t *testing.T) {
	cases := []struct {
		name string
		logs []string
		want float64
	}{
		{"no match", []string{"loading model", "another line"}, 0},
		{"empty", nil, 0},
		// real-time line: tg and tg_3s coexist, only tg_3s is used (3-second window real-time value); tg (cumulative average) must not be used.
		{"real-time line takes tg_3s",
			[]string{"I slot print_timing:              n_decoded = 414, tg = 68.82 t/s, tg_3s = 67.32 t/s"},
			67.32},
		// multiple real-time lines: take the last tg_3s (most recent sample wins).
		{"multiple real-time lines take last tg_3s",
			[]string{
				"I slot print_timing:              n_decoded = 100, tg = 60.00 t/s, tg_3s = 61.50 t/s",
				"llama_server: unrelated log noise",
				"I slot print_timing:              n_decoded = 414, tg = 68.82 t/s, tg_3s = 70.50 t/s",
			}, 70.50},
		// no real-time line (old binary): fall back to decode total timing line value.
		{"fallback eval time",
			[]string{"I slot print_timing:             eval time =   12334.07 ms /   900 tokens ( 72.97 tokens per second)"},
			72.97},
		// real-time line before eval time line: still return real-time tg_3s (real-time value
		// takes priority over request-end total timing); assertion basis: monitoring uses
		// 3-second window speed during generation.
		{"realtime line before eval line, realtime wins",
			[]string{
				"I slot print_timing:              n_decoded = 414, tg = 68.82 t/s, tg_3s = 67.32 t/s",
				"I slot print_timing:             eval time =   12334.07 ms /   900 tokens ( 72.97 tokens per second)",
			}, 67.32},
		// prefill lines are excluded from candidates: "prompt eval time" is a substring of
		// "eval time" and must be excluded first; when prefill and decode coexist, use
		// decode value 72.97 instead of prefill value 75.53.
		{"prefill excluded, decode value used",
			[]string{
				"I slot print_timing:      prompt eval time =     357.49 ms /    27 tokens ( 75.53 tokens per second)",
				"I slot print_timing:             eval time =   12334.07 ms /   900 tokens ( 72.97 tokens per second)",
			}, 72.97},
		// prefill-only lines, no decode/real-time lines: decode metric has no candidates, returns 0.
		{"prefill only",
			[]string{
				"I slot print_timing:      prompt eval time =     357.49 ms /    27 tokens ( 75.53 tokens per second)",
			}, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := parseDecodeTPS(c.logs); got != c.want {
				t.Errorf("parseDecodeTPS(%v) = %v, want %v", c.logs, got, c.want)
			}
		})
	}
}

// TestParseDecodeTPSMultiLineEntry verifies correct line splitting when a single entry
// contains multiple lines (real-time line + decode total timing line): extract real-time
// tg_3s 67.32.
func TestParseDecodeTPSMultiLineEntry(t *testing.T) {
	got := parseDecodeTPS([]string{
		"I slot print_timing:              n_decoded = 414, tg = 68.82 t/s, tg_3s = 67.32 t/s\n" +
			"I slot print_timing:             eval time =   12334.07 ms /   900 tokens ( 72.97 tokens per second)",
	})
	if got != 67.32 {
		t.Errorf("parseDecodeTPS multi-line entry = %v, want 67.32", got)
	}
}

// TestParseDecodeTPSNewFormat verifies new-format llama.cpp decode logs (with "id N | task N |"
// prefix and extra whitespace before eval time line values) are still hit by the existing
// keyword Contains and regex:
// tg_3s real-time lines directly extract the 3-second window speed; when no real-time line
// exists, fall back to eval time line value; prefill real-time lines (prompt processing)
// lack "eval time" / "tg_3s =" markers and do not enter decode candidates.
func TestParseDecodeTPSNewFormat(t *testing.T) {
	tg3sLine := "I slot print_timing: id  3 | task 0 | n_decoded =    414, tg =  68.82 t/s, tg_3s =  67.32 t/s"
	evalLine := "I slot print_timing: id  3 | task 0 |        eval time =  12334.07 ms /    900 tokens (   72.97 tokens per second)"
	progressLine := "I slot print_timing: id  3 | task 0 | prompt processing, n_tokens =   2048, progress = 0.16, t =  20.47 s / 100.05 tokens per second"

	cases := []struct {
		name string
		logs []string
		want float64
	}{
		// new-format real-time line with "id  3 | task 0 |" prefix: tg3sLogRegex substring match still hits, returns 67.32.
		{"new format real-time line", []string{tg3sLine}, 67.32},
		// new-format eval time line only (no tg_3s real-time line): fallback parses last eval time 72.97.
		{"new format fallback eval time", []string{evalLine}, 72.97},
		// prefill real-time line only: lacks "eval time" / "tg_3s =" markers, decode metric has no candidate, returns 0.
		{"prefill progress line does not pollute decode", []string{progressLine}, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := parseDecodeTPS(c.logs); got != c.want {
				t.Errorf("parseDecodeTPS(%v) = %v, want %v", c.logs, got, c.want)
			}
		})
	}
}

// ─── splitLogLines ─────────────────────────────────────────────────

// TestSplitLogLines verifies the shared line-splitting helper: embedded newlines within
// a single entry are split into independent lines, order is preserved when multiple
// entries are merged, and an empty list via Join("")→Split("") yields a single empty line
// (each parser returns 0 for empty lines because no keywords match).
func TestSplitLogLines(t *testing.T) {
	if got := splitLogLines([]string{"line1\nline2"}); !reflect.DeepEqual(got, []string{"line1", "line2"}) {
		t.Errorf("splitLogLines multi-line entry = %v, want [line1 line2]", got)
	}
	if got := splitLogLines([]string{"a\nb", "c"}); !reflect.DeepEqual(got, []string{"a", "b", "c"}) {
		t.Errorf("splitLogLines multi-entry = %v, want [a b c]", got)
	}
	if got := splitLogLines([]string{"single"}); !reflect.DeepEqual(got, []string{"single"}) {
		t.Errorf("splitLogLines single-line entry = %v, want [single]", got)
	}
	if got := splitLogLines(nil); !reflect.DeepEqual(got, []string{""}) {
		t.Errorf("splitLogLines(nil) = %v, want [\"\"]", got)
	}
}

// ─── parseNVLine ──────────────────────────────────────────────────

// TestParseNVLine verifies nvidia-smi single-line CSV parsing: full 5 columns, missing
// columns (best-effort parse), empty string, and illegal numeric values do not panic and
// map to default values. Memory unit MiB→bytes.
func TestParseNVLine(t *testing.T) {
	g := parseNVLine("0, NVIDIA GeForce RTX 4090, 45, 1024, 24576")
	if g.Index != 0 || g.Name != "NVIDIA GeForce RTX 4090" || g.UtilPercent != 45 {
		t.Errorf("full-line parse error: %+v", g)
	}
	if g.MemUsed != 1024*1024*1024 || g.MemTotal != 24576*1024*1024 {
		t.Errorf("memory unit should be MiB→bytes: used=%d total=%d", g.MemUsed, g.MemTotal)
	}

	// missing columns: best-effort parse of available columns, missing fields are 0
	short := parseNVLine("1, Intel Arc, 30")
	if short.Index != 1 || short.Name != "Intel Arc" || short.UtilPercent != 30 {
		t.Errorf("short-line parse error: %+v", short)
	}
	if short.MemUsed != 0 || short.MemTotal != 0 {
		t.Errorf("memory should stay 0 when columns are missing: %+v", short)
	}

	// empty string: Split returns at least one element, no index-out-of-bounds panic, Index defaults to -1
	empty := parseNVLine("")
	if empty.Index != -1 {
		t.Errorf("empty string Index = %d, want -1", empty.Index)
	}

	// illegal numeric values: lenient parse does not panic, corresponding fields are defaults
	bad := parseNVLine("abc, N, xyz, notnum, 0")
	if bad.Index != -1 || bad.Name != "N" || bad.UtilPercent != 0 || bad.MemUsed != 0 || bad.MemTotal != 0 {
		t.Errorf("illegal-value line parse error: %+v", bad)
	}
}

// ─── parseCPUWindows ──────────────────────────────────────────────

// TestParseCPUWindows verifies PowerShell output parsing: takes the first parseable
// number (tolerates multiple lines / multiple fields); no valid number returns 0.
func TestParseCPUWindows(t *testing.T) {
	if got := parseCPUWindows("45.5\n"); got != 45.5 {
		t.Errorf("parseCPUWindows(%q) = %v, want 45.5", "45.5\n", got)
	}
	if got := parseCPUWindows("some text\n12.25\n"); got != 12.25 {
		t.Errorf("parseCPUWindows multi-line should take first number, got %v", got)
	}
	if got := parseCPUWindows("no numbers here"); got != 0 {
		t.Errorf("parseCPUWindows no numbers should return 0, got %v", got)
	}
}

// ─── parseMemWindowsKB ────────────────────────────────────────────

// TestParseMemWindowsKB verifies Windows memory KB parsing: two-column values are
// converted to bytes via ×1024; when only one column is present, best-effort returns
// (total, 0); no numbers returns (0,0).
func TestParseMemWindowsKB(t *testing.T) {
	total, free := parseMemWindowsKB("8000000 4000000")
	if total != 8000000*1024 || free != 4000000*1024 {
		t.Errorf("parseMemWindowsKB = (%d, %d), want (%d, %d)（KB→bytes ×1024）",
			total, free, 8000000*1024, 4000000*1024)
	}

	one, f := parseMemWindowsKB("8000000")
	if one != 8000000*1024 || f != 0 {
		t.Errorf("single-column parse = (%d, %d), want (%d, 0)", one, f, 8000000*1024)
	}

	z, zf := parseMemWindowsKB("")
	if z != 0 || zf != 0 {
		t.Errorf("empty output parse = (%d, %d), want (0,0)", z, zf)
	}
}

// ─── parseProcStat ────────────────────────────────────────────────

// TestParseProcStat verifies /proc/stat first-line cpu idle and total ticks parsing:
// idle is column 4 (user nice system idle ...), total is the sum of all numeric columns;
// missing cpu line returns (0,0); cpu0 and other lines must not be matched accidentally.
func TestParseProcStat(t *testing.T) {
	// standard /proc/stat first line: user nice system idle iowait irq softirq steal guest guest_nice
	out := "cpu  335973 18239 179330 1098596 18998 0 13275 0 0 0\n" +
		"cpu0 100 100 100 100 100\n" +
		"intr 12345\n"
	idle, total := parseProcStat(out)
	if idle != 1098596 {
		t.Errorf("idle = %d, want 1098596", idle)
	}
	wantTotal := uint64(335973 + 18239 + 179330 + 1098596 + 18998 + 0 + 13275)
	if total != wantTotal {
		t.Errorf("total = %d, want %d", total, wantTotal)
	}

	// missing cpu line returns (0,0)
	if idle, total := parseProcStat("intr 123\nctxt 456\n"); idle != 0 || total != 0 {
		t.Errorf("missing cpu line should return (0,0), got (%d, %d)", idle, total)
	}

	// cpu0 line must not match accidentally (fields[0]=="cpu0" != "cpu")
	if idle, total := parseProcStat("cpu0 1 2 3 4 5\n"); idle != 0 || total != 0 {
		t.Errorf("cpu0 line should not match, got (%d, %d)", idle, total)
	}
}

// ─── cpuPercentFromDeltas ─────────────────────────────────────────

// TestCPUPercentFromDeltas verifies CPU usage calculation: Δtotal<=0 returns 0;
// when idle delta exceeds total delta, clamp to 100% (avoids counter wrap / cross-core
// overflow).
func TestCPUPercentFromDeltas(t *testing.T) {
	if got := cpuPercentFromDeltas(100, 1000, 150, 1100); got != 50 {
		t.Errorf("normal delta should yield 50%%, got %v", got)
	}
	if got := cpuPercentFromDeltas(100, 1000, 200, 1000); got != 0 {
		t.Errorf("Δtotal=0 should return 0, got %v", got)
	}
	if got := cpuPercentFromDeltas(100, 1000, 300, 1100); got != 0 {
		t.Errorf("idle delta > total delta should clamp, got %v", got)
	}
}

// ─── parseLoadAvg ─────────────────────────────────────────────────

// TestParseLoadAvg verifies loadavg parsing: takes the first value (1-minute average load);
// empty string or illegal input returns 0.
func TestParseLoadAvg(t *testing.T) {
	if got := parseLoadAvg("1.23 0.98 0.76 2/345 12345"); got != 1.23 {
		t.Errorf("parseLoadAvg = %v, want 1.23", got)
	}
	if got := parseLoadAvg(""); got != 0 {
		t.Errorf("parseLoadAvg(\"\") = %v, want 0", got)
	}
	if got := parseLoadAvg("abc def"); got != 0 {
		t.Errorf("parseLoadAvg illegal input = %v, want 0", got)
	}
}

// ─── parseMemLinux ────────────────────────────────────────────────

// TestParseMemLinux verifies /proc/meminfo parsing: MemTotal is always taken; when
// MemAvailable is missing, fall back to MemFree (KB→bytes ×1024).
func TestParseMemLinux(t *testing.T) {
	out := "MemTotal:       16000000 kB\nMemFree:         1000000 kB\nMemAvailable:    2000000 kB\n"
	total, avail := parseMemLinux(out)
	if total != 16000000*1024 || avail != 2000000*1024 {
		t.Errorf("parseMemLinux = (%d, %d), want (%d, %d)", total, avail, 16000000*1024, 2000000*1024)
	}

	// no MemAvailable → fall back to MemFree
	outNoAvail := "MemTotal:       16000000 kB\nMemFree:         1000000 kB\n"
	total2, avail2 := parseMemLinux(outNoAvail)
	if total2 != 16000000*1024 || avail2 != 1000000*1024 {
		t.Errorf("missing MemAvailable should fall back to MemFree: got (%d, %d), want (%d, %d)",
			total2, avail2, 16000000*1024, 1000000*1024)
	}
}

// ─── diskUsageForPath ─────────────────────────────────────────────

// TestDiskUsageForPath verifies the disk sampling contract (cross-platform runnable):
// samples the volume containing the temp directory, asserting Path is non-empty, Total > 0,
// Used <= Total. Target volume-root construction (Windows volume name + separator /
// non-Windows root "/") is not asserted here; only the numeric invariants shared across
// all platform branches are verified.
func TestDiskUsageForPath(t *testing.T) {
	dir := t.TempDir()
	root := filepath.VolumeName(dir)
	if root == "" {
		root = string(filepath.Separator)
	} else {
		root += string(filepath.Separator)
	}
	d, err := diskUsageForPath(root)
	if err != nil {
		t.Fatalf("diskUsageForPath(%q) sampling failed: %v", root, err)
	}
	if d.Path == "" {
		t.Error("disk sampling Path must not be empty")
	}
	if d.Total == 0 {
		t.Error("disk sampling Total must be > 0")
	}
	if d.Used > d.Total {
		t.Errorf("disk sampling Used=%d must not exceed Total=%d", d.Used, d.Total)
	}
}

// ─── procStatCPUTicks / serviceCPUPercentFromDeltas ───────────────

// TestProcStatCPUTicks verifies utime+stime extraction from /proc/<pid>/stat
// payloads: field indexing must start after the LAST ')' so a comm containing
// spaces and parentheses ("llama-se: r(v) 1") cannot shift the numeric
// fields; utime is field 14 and stime field 15 (indexes 11/12 after comm).
// Malformed or truncated payloads return ok=false.
func TestProcStatCPUTicks(t *testing.T) {
	cases := []struct {
		name  string
		stat  string
		ticks uint64
		ok    bool
	}{
		{
			// Real-shape payload: 1 (comm) S ... utime(14)=1200 stime(15)=300,
			// comm itself carries spaces and parentheses.
			name:  "comm with spaces and parens",
			stat:  "4769 (llama-se: r(v) 1) S 1 4769 4769 0 -1 4194560 100 0 0 0 1200 300 0 0 20 5 1 0 1000 100 100 18446744073709551615 1 1 1 0 0 0 0 0 0 0 0 0 0 0 0 0",
			ticks: 1500,
			ok:    true,
		},
		{"no parens", "4769 broken", 0, false},
		{"too short after comm", "4769 (srv) S 1 2 3 4 5 6 7 8 9 10", 0, false},
		{"non-numeric utime", "4769 (srv) S 1 2 3 4 5 6 7 8 9 10 x 12 13", 0, false},
		{"non-numeric stime", "4769 (srv) S 1 2 3 4 5 6 7 8 9 10 11 x", 0, false},
		{"empty", "", 0, false},
	}
	for _, tc := range cases {
		ticks, ok := procStatCPUTicks(tc.stat)
		if ok != tc.ok {
			t.Errorf("%s: ok = %v, want %v", tc.name, ok, tc.ok)
			continue
		}
		if ok && ticks != tc.ticks {
			t.Errorf("%s: ticks = %d, want %d", tc.name, ticks, tc.ticks)
		}
	}
}

// TestServiceCPUPercentFromDeltas verifies the llama-server child's share of
// total CPU capacity: ticks/(seconds*USER_HZ*cores)*100; zero-value or zero
// -time baselines (missing prior sample), non-positive intervals, backwards
// tick counters (fresh child after restart) and non-positive core counts all
// yield 0; the result is capped at 100.
func TestServiceCPUPercentFromDeltas(t *testing.T) {
	base := time.Unix(1000, 0)
	oneSecLater := base.Add(time.Second)

	cases := []struct {
		name      string
		prevTicks uint64
		curTicks  uint64
		prevWall  time.Time
		curWall   time.Time
		ncpu      int
		want      float64
	}{
		// 150 ticks in 1s on 1 core at USER_HZ=100 → 150% of one core, i.e.
		// the full capacity share → capped at 100.
		{"saturating single core capped", 100, 250, base, oneSecLater, 1, 100},
		// 150 ticks in 1s across 2 cores → half of total capacity.
		{"half of two cores", 100, 250, base, oneSecLater, 2, 75},
		// 50 ticks in 1s across 4 cores → 12.5% of total capacity.
		{"quarter load on four cores", 100, 150, base, oneSecLater, 4, 12.5},
		{"missing prev baseline", 0, 150, time.Time{}, oneSecLater, 1, 0},
		{"missing cur baseline", 150, 0, base, oneSecLater, 1, 0},
		{"backwards ticks restart", 500, 100, base, oneSecLater, 1, 0},
		{"zero interval", 100, 150, base, base, 1, 0},
		{"negative interval", 100, 150, oneSecLater, base, 1, 0},
		{"zero cores", 0, 150, base, oneSecLater, 0, 0},
	}
	for _, tc := range cases {
		got := serviceCPUPercentFromDeltas(tc.prevTicks, tc.curTicks, tc.prevWall, tc.curWall, tc.ncpu)
		if got != tc.want {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
}
