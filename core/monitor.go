package core

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ─── Realtime Monitor (CPU / Memory / GPU / TPS) ──────────────────
//
// The sampler runs in a background goroutine at 1s intervals, caching results
// in monitorStatus (guarded by monitorMu). GetMonitorStatus copies the cache
// under the lock before returning, so Wails calls and sampling never race on
// the same struct. CPU/memory parsing is pure functions; tests cover parsing
// only and do not mock runCmd.

// MonitorStatus is the JSON contract returned to the frontend by
// GetMonitorStatus; fields match frontend/src/lib/monitor.ts MonitorStatus
// one-to-one, so changes must sync on both sides.
type MonitorStatus struct {
	CPUPercent    float64      `json:"cpuPercent"`
	MemUsed       uint64       `json:"memUsed"`
	MemTotal      uint64       `json:"memTotal"`
	GPUs          []MonitorGPU `json:"gpus"`
	ServerRunning bool         `json:"serverRunning"`
	PromptTPS     float64      `json:"promptTps"`
	DecodeTPS     float64      `json:"decodeTps"`
	UptimeSeconds int64        `json:"uptimeSeconds"`
	// Disk is the disk usage of the volume containing the model directory;
	// nil when sampling fails (frontend hides the disk row), and it never
	// blocks other sampling metrics.
	Disk *DiskUsage `json:"disk"`
}

// DiskUsage is the usage of a single disk volume: Path is the volume root
// (Windows like "C:\", non-Windows like "/"), Used = Total - Free.
type DiskUsage struct {
	Path  string `json:"path"`
	Used  uint64 `json:"used"`
	Total uint64 `json:"total"`
}

// MonitorGPU is the per-GPU monitoring data (sampled via nvidia-smi).
type MonitorGPU struct {
	Index       int     `json:"index"`
	Name        string  `json:"name"`
	UtilPercent float64 `json:"utilPercent"`
	MemUsed     uint64  `json:"memUsed"`
	MemTotal    uint64  `json:"memTotal"`
}

// monitorStatus is the sampling cache, guarded by monitorMu.
var monitorStatus MonitorStatus
var monitorMu sync.Mutex

// monitorOnce ensures the sampler is started only once (StartMonitorSampler
// is called by app.Startup; idempotent to prevent duplicate starts).
var monitorOnce sync.Once

// linuxCPUPrevIdle / linuxCPUPrevTotal hold the previous sample's idle and
// total ticks from /proc/stat for delta-based CPU calculation on Linux (only
// the sampling goroutine reads/writes these, so no lock is needed); first
// sample returns 0.
var linuxCPUPrevIdle, linuxCPUPrevTotal uint64

// tpsLogRegex matches the throughput snippet "N tokens per second" in
// llama-server logs (both prompt-eval and decode-eval lines contain this
// snippet; parsePromptTPS / parseDecodeTPS classify which kind per line).
var tpsLogRegex = regexp.MustCompile(`([\d.]+)\s+tokens?\s+per\s+second`)

// tg3sLogRegex matches the realtime decode line's 3-second window speed
// "tg_3s = N t/s" (N is a float).
var tg3sLogRegex = regexp.MustCompile(`tg_3s\s*=\s*([\d.]+)\s+t/s`)

// splitLogLines joins log entries into text then splits by line: legacy
// multi-line entries (multiple lines in one stderr Write) are split into
// independent lines here, complementing the write-side line buffer
// (serverLogTailer) to guarantee print_timing lines enter classification as
// whole lines and prompt prefill values cannot leak into decode speed.
// Pure function: an empty list joins to "" then splits to a single empty line
// (parsing functions naturally return 0 when no keyword matches).
func splitLogLines(logs []string) []string {
	return strings.Split(strings.Join(logs, "\n"), "\n")
}

// parsePromptTPS extracts the last prompt prefill line's "N tokens per second"
// value (tokens/s) from server log lines, i.e. prompt processing / prefill
// speed. llama-server prefill timing comes in two line forms, realtime lines
// (newer llama.cpp) and terminal lines (both old and new):
//
//	I slot print_timing: id  3 | task 0 | prompt processing, n_tokens =   2048, progress = 0.16, t =  20.47 s / 100.05 tokens per second
//	I slot print_timing: id  3 | task 0 | prompt eval time =    357.49 ms /     27 tokens (   75.53 tokens per second)
//
// Realtime lines (prompt processing) are progress lines printed per batch
// during prefill, appearing only when prefill takes >=3s (short prefill has
// no such line); terminal lines (prompt eval time) print at the end of every
// request. Both are extracted, and the last value wins: realtime lines keep
// refreshing, and log order naturally puts the terminal line last, so the
// terminal value is the final authoritative one; older binaries only have
// terminal lines, which are still compatible. This value reflects prompt
// throughput (thousands of t/s on long prompts), distinct from decode speed,
// shown by the monitoring page's "Inference" module. Pure function: returns 0
// when no prefill line exists; when multiple lines have multiple values, the
// last prefill line wins (latest sample); within a single line, only the
// first regexp match is used; supports decimals.
func parsePromptTPS(logs []string) float64 {
	var last float64
	for _, line := range splitLogLines(logs) {
		// Recognize both prefill line types: newer realtime line (prompt
		// processing) and older/newer terminal line (prompt eval time).
		if !strings.Contains(line, "prompt processing") && !strings.Contains(line, "prompt eval time") {
			continue
		}
		if m := tpsLogRegex.FindStringSubmatch(line); m != nil {
			if v, err := strconv.ParseFloat(m[1], 64); err == nil {
				last = v
			}
		}
	}
	return last
}

// parseDecodeTPS extracts the realtime generation (decode) speed (tokens/s)
// from server log lines. Router-mode llama-server prints a realtime stats
// line approximately every 3 seconds during generation, and a terminal timing
// line when the request ends:
//
//	I slot print_timing: id  3 | task 0 | n_decoded =    414, tg =  68.82 t/s, tg_3s =  67.32 t/s
//	I slot print_timing: id  3 | task 0 |        eval time =  12334.07 ms /    900 tokens (   72.97 tokens per second)
//
// Newer lines have the "id N | task N |" prefix, and eval-time lines have
// extra leading spaces: line classification relies on keyword Contains, and
// value extraction relies on tpsLogRegex / tg3sLogRegex substring matches, so
// prefixes and spaces do not affect matching, and older prefix-less lines are
// still compatible. Realtime lines use tg for cumulative average decode speed
// and tg_3s for the recent 3-second window speed (realtime value); this
// function only takes tg_3s and prefers it (even when an eval time line
// follows, realtime wins over the terminal total); older binaries without
// realtime lines fall back to the last eval time line's value. The fallback
// branch strictly requires the "eval time" marker: truncated fragments that
// only contain the "tokens per second" snippet are no longer accepted
// (fragment reassembly is the responsibility of the write-side line buffer
// serverLogTailer). Former "TPS always 0" root cause: the old implementation
// only parsed eval time lines printed at generation end, while realtime lines
// lack "tokens per second", leaving no value during generation.
// Pure function: returns 0 when no candidate exists; when multiple lines
// have multiple values, the last line wins (latest sample); supports decimals.
func parseDecodeTPS(logs []string) float64 {
	var lastTg3s, lastEval float64
	for _, line := range splitLogLines(logs) {
		if strings.Contains(line, "tg_3s =") {
			if m := tg3sLogRegex.FindStringSubmatch(line); m != nil {
				if v, err := strconv.ParseFloat(m[1], 64); err == nil {
					lastTg3s = v
				}
			}
			continue
		}
		// Exclude prefill lines first: "prompt eval time" is a substring of
		// "eval time", and it also contains the "tokens per second" snippet
		// (e.g. 75.53); skipping it prevents polluting decode speed.
		if strings.Contains(line, "prompt eval time") {
			continue
		}
		if strings.Contains(line, "eval time") {
			if m := tpsLogRegex.FindStringSubmatch(line); m != nil {
				if v, err := strconv.ParseFloat(m[1], 64); err == nil {
					lastEval = v
				}
			}
		}
	}
	if lastTg3s > 0 {
		return lastTg3s
	}
	return lastEval
}

// StartMonitorSampler starts the background sampler (sync.Once guarantees it
// starts only once). Sampling interval is 1s, infinite loop until process
// exit; each round refreshes the monitorStatus cache.
func StartMonitorSampler() {
	monitorOnce.Do(func() {
		go func() {
			for {
				sampleMonitor()
				time.Sleep(1 * time.Second)
			}
		}()
	})
}

// GetMonitorStatus returns the current monitoring sample snapshot: reads the
// monitorStatus cache (monitorMu), while ServerRunning / UptimeSeconds /
// PromptTPS / DecodeTPS are fetched live (serverMu / serverLogsMu), so these
// high-frequency fields remain real-time within the cache sampling period.
func GetMonitorStatus() *MonitorStatus {
	monitorMu.Lock()
	st := monitorStatus
	monitorMu.Unlock()

	serverMu.Lock()
	running := serverRunning
	startTime := serverStartTime
	serverMu.Unlock()
	st.ServerRunning = running
	if running && !startTime.IsZero() {
		st.UptimeSeconds = int64(time.Since(startTime).Seconds())
	} else {
		st.UptimeSeconds = 0
	}

	// TPS: read the last 50 log lines (serverLogsMu), compute prefill and
	// decode speeds on the fly
	serverLogsMu.Lock()
	entries := serverLogs
	if n := len(entries); n > 50 {
		entries = entries[n-50:]
	}
	tail := make([]string, len(entries))
	for i, e := range entries {
		tail[i] = e.text
	}
	serverLogsMu.Unlock()
	st.PromptTPS = parsePromptTPS(tail)
	st.DecodeTPS = parseDecodeTPS(tail)

	return &st
}

// sampleMonitor runs one round of system sampling and updates the cache
// (monitorMu). Platform parsers use runCmd or readProcFile: Windows shells
// out to PowerShell (hideWindow suppresses the console window), kernel
// pseudo-files go through readProcFile (in-process os.ReadFile on Android,
// `cat` elsewhere).
func sampleMonitor() {
	var st MonitorStatus
	switch runtime.GOOS {
	case "windows":
		st.CPUPercent = sampleCPUWindows()
		st.MemTotal, st.MemUsed = sampleMemWindows()
	case "linux":
		// Desktop Linux: /proc/stat + /proc/meminfo are readable and exec is
		// available (readProcFile shells out to `cat` here).
		st.CPUPercent = sampleCPULinux()
		st.MemTotal, st.MemUsed = sampleMemLinux()
	case "android":
		// Android is Linux-kernel so /proc/meminfo parses the same way, but
		// the app sandbox cannot exec (readProcFile falls back to an
		// in-process os.ReadFile there) and /proc/stat is additionally
		// SELinux-blocked for apps, so system-wide CPU% is unobtainable: the
		// llama-server child's /proc/<pid>/stat deltas stand in instead (its
		// share of total CPU capacity, same 0-100 scale as desktop).
		st.CPUPercent = sampleAndroidServiceCPU()
		st.MemTotal, st.MemUsed = sampleMemLinux()
	case "darwin":
		st.CPUPercent = sampleCPUDarwin()
		st.MemTotal, st.MemUsed = sampleMemDarwin()
	}
	st.GPUs = sampleGPUs()
	st.Disk = sampleDiskUsage()
	monitorMu.Lock()
	monitorStatus = st
	monitorMu.Unlock()
}

// ─── Disk Sampling ────────────────────────────────────────────────

// sampleDiskUsage samples the disk usage of the volume containing the model
// download directory: the target volume is the root of the absolute path of
// effectiveModelDownloadDir (read under modelDownloadDirMu). On non-Windows
// platforms where filepath.VolumeName is empty, it falls back to the volume
// root of the current working directory. Used = Total - Free is computed
// inside the platform-specific diskUsageForPath. Returns nil on sampling
// failure, without blocking other sampling metrics.
func sampleDiskUsage() *DiskUsage {
	dir := effectiveModelDownloadDir()
	abs, err := filepath.Abs(dir)
	if err != nil {
		abs = dir
	}
	root := filepath.VolumeName(abs)
	if root == "" {
		wd, err := os.Getwd()
		if err != nil {
			return nil
		}
		root = filepath.VolumeName(wd)
		if root == "" {
			root = string(filepath.Separator)
		}
	} else {
		// On Windows, volume names look like "C:"; append the separator to get
		// "C:\", which points to the volume root.
		root += string(filepath.Separator)
	}
	d, err := diskUsageForPath(root)
	if err != nil {
		return nil
	}
	return d
}

// ─── CPU Sampling ─────────────────────────────────────────────────

// parseCPUWindows parses the PowerShell Win32_Processor average load
// percentage output
// (`(Get-CimInstance Win32_Processor | Measure-Object -Property LoadPercentage
// -Average).Average`), returning a float between 0 and 100. Multi-core /
// multi-package output may contain multiple lines/values; takes the first
// parseable number, or 0 when none is valid.
func parseCPUWindows(out string) float64 {
	for _, line := range strings.Split(out, "\n") {
		for _, f := range strings.Fields(line) {
			if v, err := strconv.ParseFloat(f, 64); err == nil {
				return v
			}
		}
	}
	return 0
}

// sampleCPUWindows queries the average load percentage via PowerShell WMI.
func sampleCPUWindows() float64 {
	return parseCPUWindows(runCmd("powershell", "-NoProfile", "-Command",
		"(Get-CimInstance Win32_Processor | Measure-Object -Property LoadPercentage -Average).Average"))
}

// parseProcStat parses the first line "cpu ..." of /proc/stat for idle and
// total ticks (jiffies). Pure function returning (idle, total); returns
// (0,0) on parse failure or when no cpu line is found.
func parseProcStat(out string) (idle, total uint64) {
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 5 || fields[0] != "cpu" {
			continue
		}
		var sum uint64
		for _, f := range fields[1:] {
			if v, err := strconv.ParseUint(f, 10, 64); err == nil {
				sum += v
			}
		}
		idleVal, err := strconv.ParseUint(fields[4], 10, 64)
		if err != nil {
			idleVal = 0
		}
		return idleVal, sum
	}
	return 0, 0
}

// cpuPercentFromDeltas computes CPU usage (%) from two sampling deltas:
// idle and total increments, 100*(1 - Δidle/Δtotal); returns 0 when
// Δtotal<=0 (no valid interval). Pure function, shared by parseProcStat tests
// and sampleCPULinux.
func cpuPercentFromDeltas(prevIdle, prevTotal, curIdle, curTotal uint64) float64 {
	totalDelta := curTotal - prevTotal
	if totalDelta <= 0 {
		return 0
	}
	idleDelta := curIdle - prevIdle
	if idleDelta > totalDelta {
		idleDelta = totalDelta
	}
	return 100 * (1 - float64(idleDelta)/float64(totalDelta))
}

// sampleCPULinux computes CPU usage (%) from two /proc/stat sampling deltas.
// The sampler holds prevIdle/prevTotal as package-level state; returns 0
// when no previous sample exists.
func sampleCPULinux() float64 {
	curIdle, curTotal := parseProcStat(readProcFile("/proc/stat"))
	if curTotal == 0 {
		return 0
	}
	if linuxCPUPrevTotal == 0 {
		linuxCPUPrevIdle = curIdle
		linuxCPUPrevTotal = curTotal
		return 0
	}
	pct := cpuPercentFromDeltas(linuxCPUPrevIdle, linuxCPUPrevTotal, curIdle, curTotal)
	linuxCPUPrevIdle = curIdle
	linuxCPUPrevTotal = curTotal
	return pct
}

// ─── Android Service CPU Sampling ─────────────────────────────────

// procStatUserHz is USER_HZ, the clock-tick frequency /proc/<pid>/stat's
// utime/stime are expressed in (Linux userland has standardized on 100; same
// constant as procClockTicks in proctime_proc.go, kept separate because that
// file is linux||android-tagged while this sampler compiles everywhere).
const procStatUserHz = 100

// androidServiceCPUPrevTicks / androidServiceCPUPrevWall hold the previous
// sample's llama-server tick counter and wall-clock reading for delta
// computation (only the sampling goroutine reads/writes them, matching
// linuxCPUPrev*); zero / zero time means "no baseline yet — the next sample
// primes it".
var (
	androidServiceCPUPrevTicks uint64
	androidServiceCPUPrevWall  time.Time
)

// runningServerPID returns the running llama-server child's pid (0 when not
// running), reading the server lifecycle state under serverMu. Lock-ordering
// rule respected: the sampler takes serverMu alone and releases it before
// sampleMonitor takes monitorMu at the end of the round.
func runningServerPID() int {
	serverMu.Lock()
	defer serverMu.Unlock()
	if !serverRunning || serverCmd == nil || serverCmd.Process == nil {
		return 0
	}
	return serverCmd.Process.Pid
}

// sampleAndroidServiceCPU computes the running llama-server's share of the
// total CPU capacity (%) from consecutive /proc/<pid>/stat samples. System
// -wide CPU% is unobtainable on Android (no exec; /proc/stat SELinux-blocked
// for apps), so the inference service's own load stands in for the 处理器
// card's ring: 0 while the service is stopped, meaningful load while it
// serves requests. Returns 0 and re-primes the baseline when the service is
// not running or its stat file disappears.
func sampleAndroidServiceCPU() float64 {
	pid := runningServerPID()
	if pid <= 0 {
		androidServiceCPUPrevTicks = 0
		androidServiceCPUPrevWall = time.Time{}
		return 0
	}
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		androidServiceCPUPrevTicks = 0
		androidServiceCPUPrevWall = time.Time{}
		return 0
	}
	ticks, ok := procStatCPUTicks(string(data))
	if !ok {
		androidServiceCPUPrevTicks = 0
		androidServiceCPUPrevWall = time.Time{}
		return 0
	}
	now := time.Now()
	prevTicks, prevWall := androidServiceCPUPrevTicks, androidServiceCPUPrevWall
	androidServiceCPUPrevTicks, androidServiceCPUPrevWall = ticks, now
	return serviceCPUPercentFromDeltas(prevTicks, ticks, prevWall, now, runtime.NumCPU())
}

// procStatCPUTicks extracts utime + stime (fields 14-15, clock ticks) from a
// /proc/<pid>/stat payload. Field 2 (comm) is parenthesized and may contain
// spaces and parentheses, so field indexing starts after the LAST ')' — the
// standard procfs parsing caveat (man 5 proc), same approach as
// procStatStartTime; everything after comm is numeric, so no later field can
// reintroduce a ')'. ok is false on malformed or truncated payloads.
func procStatCPUTicks(stat string) (ticks uint64, ok bool) {
	idx := strings.LastIndex(stat, ")")
	if idx < 0 || idx+2 > len(stat) {
		return 0, false
	}
	// Fields after "comm " start at field 3 (state); utime / stime are
	// fields 14 / 15, i.e. indexes 11 / 12 in this slice.
	fields := strings.Fields(stat[idx+2:])
	if len(fields) < 13 {
		return 0, false
	}
	utime, err := strconv.ParseUint(fields[11], 10, 64)
	if err != nil {
		return 0, false
	}
	stime, err := strconv.ParseUint(fields[12], 10, 64)
	if err != nil {
		return 0, false
	}
	return utime + stime, true
}

// serviceCPUPercentFromDeltas turns two consecutive /proc/<pid>/stat samples
// into the child's share of total CPU capacity: the utime+stime tick delta
// divided by the wall-clock interval scaled by USER_HZ and the core count,
// so multi-threaded inference maps onto the same 0-100 scale the desktop
// system-wide samplers report. Returns 0 for a missing baseline (zero ticks
// or zero time), a non-positive wall interval, a backwards tick counter
// (fresh child after a restart), or a non-positive core count; caps at 100.
// Pure function.
func serviceCPUPercentFromDeltas(prevTicks, curTicks uint64, prevWall, curWall time.Time, ncpu int) float64 {
	if prevTicks == 0 || curTicks == 0 || prevWall.IsZero() || curWall.IsZero() {
		return 0
	}
	seconds := curWall.Sub(prevWall).Seconds()
	if seconds <= 0 || curTicks < prevTicks || ncpu <= 0 {
		return 0
	}
	pct := 100 * float64(curTicks-prevTicks) / (seconds * float64(procStatUserHz) * float64(ncpu))
	if pct > 100 {
		pct = 100
	}
	return pct
}

// parseLoadAvg parses `sysctl -n vm.loadavg` (e.g. "1.23 0.98 0.76 2/345 12345"),
// returning the first value (1-minute load average).
func parseLoadAvg(out string) float64 {
	fields := strings.Fields(strings.TrimSpace(out))
	if len(fields) == 0 {
		return 0
	}
	v, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0
	}
	return v
}

// sampleCPUDarwin estimates CPU usage (approximate) via loadavg / NumCPU * 100.
func sampleCPUDarwin() float64 {
	load := parseLoadAvg(runCmd("sysctl", "-n", "vm.loadavg"))
	ncpu := runtime.NumCPU()
	if ncpu == 0 {
		return 0
	}
	return load * 100 / float64(ncpu)
}

// ─── Memory Sampling ──────────────────────────────────────────────

// parseMemWindowsKB parses PowerShell Win32_OperatingSystem output
// (TotalVisibleMemorySize / FreePhysicalMemory, unit KB). Expects two numeric
// columns and returns (total, free) bytes (KB→bytes ×1024); when fewer than
// two columns are parseable, returns as many as possible.
func parseMemWindowsKB(out string) (total, free uint64) {
	var nums []uint64
	for _, line := range strings.Split(out, "\n") {
		for _, f := range strings.Fields(line) {
			if v, err := strconv.ParseUint(f, 10, 64); err == nil {
				nums = append(nums, v)
			}
		}
	}
	if len(nums) >= 2 {
		return nums[0] * 1024, nums[1] * 1024
	}
	if len(nums) == 1 {
		return nums[0] * 1024, 0
	}
	return 0, 0
}

// sampleMemWindows reads total/free physical memory via PowerShell (KB),
// converts to bytes, and returns (total, used).
func sampleMemWindows() (total, used uint64) {
	out := runCmd("powershell", "-NoProfile", "-Command",
		"$os=Get-CimInstance Win32_OperatingSystem; \"$($os.TotalVisibleMemorySize) $($os.FreePhysicalMemory)\"")
	total, free := parseMemWindowsKB(out)
	if free <= total {
		used = total - free
	}
	return total, used
}

// parseMemLinux parses MemTotal and MemAvailable (KB→bytes) from
// /proc/meminfo, returning (total, avail) bytes. Falls back to MemFree when
// MemAvailable is missing.
func parseMemLinux(out string) (total, avail uint64) {
	total = parseMemInfo(out, "MemTotal") * 1024
	if a := parseMemInfo(out, "MemAvailable"); a > 0 {
		avail = a * 1024
	} else {
		avail = parseMemInfo(out, "MemFree") * 1024
	}
	return
}

// sampleMemLinux reads /proc/meminfo and computes (total, used) bytes.
func sampleMemLinux() (total, used uint64) {
	total, avail := parseMemLinux(readProcFile("/proc/meminfo"))
	if avail <= total {
		used = total - avail
	}
	return total, used
}

// sampleMemDarwin computes (total, used) bytes using `sysctl -n hw.memsize`
// (total bytes) + `vm_stat` (free page count). Falls back to 16384 for
// hw.pagesize on parse failure (arm64 default page size).
func sampleMemDarwin() (total, used uint64) {
	out := runCmd("sysctl", "-n", "hw.memsize")
	total, _ = strconv.ParseUint(strings.TrimSpace(out), 10, 64)
	freePages := parseVMStat(runCmd("vm_stat"), "free")
	if freePages == 0 {
		return total, 0
	}
	pageSize := uint64(16384)
	if psStr := strings.TrimSpace(runCmd("sysctl", "-n", "hw.pagesize")); psStr != "" {
		if ps, err := strconv.ParseUint(psStr, 10, 64); err == nil && ps > 0 {
			pageSize = ps
		}
	}
	free := freePages * pageSize
	if free <= total {
		used = total - free
	} else {
		used = 0
	}
	return total, used
}

// ─── GPU Sampling ─────────────────────────────────────────────────

// parseNVLine parses one line of nvidia-smi csv output
// (`--query-gpu=index,name,utilization.gpu,memory.used,memory.total
// --format=csv,noheader,nounits`). Returns MonitorGPU; missing columns
// default to 0 (lenient parsing); when fewer than 5 columns are present,
// parses whatever is available.
func parseNVLine(line string) MonitorGPU {
	g := MonitorGPU{Index: -1}
	parts := strings.Split(line, ",")
	if v, err := strconv.Atoi(strings.TrimSpace(parts[0])); err == nil {
		g.Index = v
	}
	if len(parts) > 1 {
		g.Name = strings.TrimSpace(parts[1])
	}
	if len(parts) > 2 {
		if v, err := strconv.ParseFloat(strings.TrimSpace(parts[2]), 64); err == nil {
			g.UtilPercent = v
		}
	}
	if len(parts) > 3 {
		if v, err := strconv.ParseUint(strings.TrimSpace(parts[3]), 10, 64); err == nil {
			g.MemUsed = v * 1024 * 1024 // MiB → bytes
		}
	}
	if len(parts) > 4 {
		if v, err := strconv.ParseUint(strings.TrimSpace(parts[4]), 10, 64); err == nil {
			g.MemTotal = v * 1024 * 1024
		}
	}
	return g
}

// sampleGPUs calls nvidia-smi to sample all GPUs; returns an empty list (no
// error) on failure (no nvidia-smi / no GPU). Memory unit conversion from
// MiB to bytes happens in parseNVLine.
//
// Always returns a non-nil slice: json.Marshal serializes a nil slice as
// null, while the frontend MonitorStatus contract declares gpus as an array
// and Api.vue dereferences status.gpus.length — null would crash the page
// render on machines without nvidia-smi / an NVIDIA GPU.
func sampleGPUs() []MonitorGPU {
	// Android has no nvidia-smi in the app sandbox: return the empty non-nil
	// slice directly instead of attempting a doomed exec every sample tick.
	if gpuProbesUnsupported() {
		return make([]MonitorGPU, 0)
	}
	out := runCmd("nvidia-smi",
		"--query-gpu=index,name,utilization.gpu,memory.used,memory.total",
		"--format=csv,noheader,nounits")
	if out == "" {
		return make([]MonitorGPU, 0)
	}
	gpus := make([]MonitorGPU, 0)
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		g := parseNVLine(line)
		if g.Index < 0 {
			continue
		}
		gpus = append(gpus, g)
	}
	return gpus
}
