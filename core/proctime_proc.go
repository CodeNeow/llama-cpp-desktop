//go:build linux

package core

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// procClockTicks is USER_HZ, the clock-tick frequency /proc/<pid>/stat's
// starttime is expressed in. Linux userland has standardized on 100
// (CONFIG_HZ=100 exposed as CLK_TCK; see man 2 times / man 5 proc).
const procClockTicks = 100

// processStartTime queries a process's real creation time from procfs: field
// 22 (starttime, clock ticks since boot) of /proc/<pid>/stat plus the boot
// time (btime line of /proc/stat) yield the absolute creation time. Used by
// the handover start-time check to detect pid reuse. ok is false on any
// read/parse failure; callers treat that as "check unavailable" and fail open
// (see evaluateHandover).
func processStartTime(pid int) (time.Time, bool) {
	if pid <= 0 {
		return time.Time{}, false
	}
	stat, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return time.Time{}, false
	}
	boot, err := procBootTime()
	if err != nil {
		return time.Time{}, false
	}
	ticks, ok := procStatStartTime(string(stat))
	if !ok {
		return time.Time{}, false
	}
	return boot.Add(time.Duration(ticks) * (time.Second / procClockTicks)), true
}

// procStatStartTime extracts field 22 (starttime) from a /proc/<pid>/stat
// payload. Field 2 (comm) is parenthesized and may itself contain spaces and
// parentheses, so field indexing must start after the LAST ')' — the standard
// procfs parsing caveat (man 5 proc). Everything after comm is numeric (state,
// ppid, ...), so no later field can reintroduce a ')'.
func procStatStartTime(stat string) (int64, bool) {
	idx := strings.LastIndex(stat, ")")
	if idx < 0 || idx+2 > len(stat) {
		return 0, false
	}
	// Fields after "comm " start at field 3 (state); starttime is field 22,
	// i.e. index 22-3 = 19.
	fields := strings.Fields(stat[idx+2:])
	if len(fields) < 20 {
		return 0, false
	}
	ticks, err := strconv.ParseInt(fields[19], 10, 64)
	if err != nil {
		return 0, false
	}
	return ticks, true
}

// procBootTime reads the system boot time (btime, seconds since the Unix
// epoch) from /proc/stat.
func procBootTime() (time.Time, error) {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return time.Time{}, err
	}
	for _, line := range strings.Split(string(data), "\n") {
		const prefix = "btime "
		if strings.HasPrefix(line, prefix) {
			sec, err := strconv.ParseInt(strings.TrimSpace(line[len(prefix):]), 10, 64)
			if err != nil {
				return time.Time{}, fmt.Errorf("parse btime from /proc/stat: %w", err)
			}
			return time.Unix(sec, 0).UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("btime line not found in /proc/stat")
}
