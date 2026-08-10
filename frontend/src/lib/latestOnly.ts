/** 只保留最后一次发起的异步请求结果，丢弃过期响应（#15 竞态保护）。 */
export class LatestOnly {
  private seq = 0
  /** 发起新请求，返回本次请求序号 */
  begin(): number { return ++this.seq }
  /** 该序号是否仍是最新请求（过期返回 false，调用方应丢弃结果） */
  isLatest(seq: number): boolean { return seq === this.seq }
}
