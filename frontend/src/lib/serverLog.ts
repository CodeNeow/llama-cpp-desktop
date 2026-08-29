/**
 * Incremental server-log view state: pure helpers backing Api.vue's
 * cursor-based log polling (backend GetServerLogsSince). Kept free of side
 * effects so the reset-on-gap and trim behavior can be table-tested.
 */

// One incremental log entry from the backend ring: seq is the line's
// monotonic cursor value, text the raw line content.
export interface ServerLogEntry {
  seq: number
  text: string
}

// Local view window: mirrors the backend ring cap (core/server.go
// serverLogsCap) so the console never grows unbounded and a full refetch
// covers exactly what the backend can return.
export const SERVER_LOG_WINDOW = 2000

export interface LogViewResult {
  lines: string[]
  cursor: number
  /** true when the fetch fell out of the backend retention window */
  reset: boolean
}

// appendLogEntries merges one incremental fetch into the local view: entries
// are the backend lines appended since cursor, next is the backend's next
// cursor value. When next - cursor exceeds cap, lines were evicted from the
// backend ring between polls and the view can no longer be patched —
// reset=true tells the caller to refetch everything (since 0) and replace the
// view instead of appending.
export function appendLogEntries(
  lines: string[],
  cursor: number,
  entries: ServerLogEntry[],
  next: number,
  cap: number = SERVER_LOG_WINDOW,
): LogViewResult {
  if (next - cursor > cap) {
    return { lines, cursor, reset: true }
  }
  const merged = [...lines, ...entries.map((e) => e.text)]
  const trimmed = merged.length > cap ? merged.slice(merged.length - cap) : merged
  return { lines: trimmed, cursor: Math.max(cursor, next), reset: false }
}

// applyFullLogFetch replaces the whole view with a full fetch (since 0):
// used on mount, after service start/stop actions (every start clears the
// backend ring) and as the reset path after a retention-window gap.
export function applyFullLogFetch(
  entries: ServerLogEntry[],
  next: number,
  cap: number = SERVER_LOG_WINDOW,
): { lines: string[]; cursor: number } {
  const lines = entries.map((e) => e.text)
  return {
    lines: lines.length > cap ? lines.slice(lines.length - cap) : lines,
    cursor: next,
  }
}
