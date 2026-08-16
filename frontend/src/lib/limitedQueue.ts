/**
 * Concurrency-limited task queue: caps the number of asynchronously executing tasks
 * (e.g. when search result cards batch-request model sizes, avoiding 200 HTTP
 * requests hitting the backend/mirror at once).
 */
export class LimitedQueue {
  private queue: Array<() => Promise<void>> = []
  private running = 0
  private readonly max: number

  constructor(max: number) {
    // Internal programming error; fixed English constant (invisible to users, catches illegal input during development only).
    if (!(max >= 1)) throw new Error(`LimitedQueue max must be >= 1, got ${max}`)
    this.max = max
  }

  /** Enqueue a task; the queue schedules its execution (waits when above the concurrency cap). */
  push(task: () => Promise<void>): void {
    this.queue.push(task)
    this.pump()
  }

  /** Number of tasks currently in flight (running plus queued). */
  pending(): number {
    return this.running + this.queue.length
  }

  private pump(): void {
    while (this.running < this.max && this.queue.length > 0) {
      const task = this.queue.shift()!
      this.running++
      // The queue runs tasks fire-and-forget: task errors are handled by the tasks
      // themselves (e.g. caller try/catch); rejections are swallowed here to avoid
      // unhandled Promise rejections
      task()
        .catch(() => {})
        .finally(() => {
          this.running--
          this.pump()
        })
    }
  }
}
