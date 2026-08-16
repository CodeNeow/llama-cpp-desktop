/** Keep only the result of the last initiated async request, discarding stale responses (#15 race protection). */
export class LatestOnly {
  private seq = 0
  /** Start a new request, returning its sequence number */
  begin(): number { return ++this.seq }
  /** Whether that sequence number is still the latest request (false when stale; the caller should discard the result) */
  isLatest(seq: number): boolean { return seq === this.seq }
}
