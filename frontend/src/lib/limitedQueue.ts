/**
 * 并发受限任务队列：控制异步任务的最大同时执行数（如搜索结果卡片批量请求
 * 模型大小时，避免 200 个 HTTP 请求同时涌向后端/镜像站）。
 */
export class LimitedQueue {
  private queue: Array<() => Promise<void>> = []
  private running = 0
  private readonly max: number

  constructor(max: number) {
    if (!(max >= 1)) throw new Error(`LimitedQueue max 必须 >= 1，got ${max}`)
    this.max = max
  }

  /** 入队一个任务，由队列调度执行（超出并发上限时排队等待）。 */
  push(task: () => Promise<void>): void {
    this.queue.push(task)
    this.pump()
  }

  /** 当前正在执行的任务数（含运行中与排队中合计）。 */
  pending(): number {
    return this.running + this.queue.length
  }

  private pump(): void {
    while (this.running < this.max && this.queue.length > 0) {
      const task = this.queue.shift()!
      this.running++
      // 队列按 fire-and-forget 语义执行：任务错误由任务自身处理（如调用方
      // try/catch），这里吞掉 rejection 避免产生未处理 Promise rejection
      task()
        .catch(() => {})
        .finally(() => {
          this.running--
          this.pump()
        })
    }
  }
}
