import { describe, it, expect } from 'vitest'
import { LatestOnly } from '../lib/latestOnly'

describe('LatestOnly', () => {
  it('begin 返回递增序号', () => {
    const gate = new LatestOnly()
    expect(gate.begin()).toBe(1)
    expect(gate.begin()).toBe(2)
    expect(gate.begin()).toBe(3)
  })

  it('最新请求 isLatest 返回 true', () => {
    const gate = new LatestOnly()
    const seq = gate.begin()
    expect(gate.isLatest(seq)).toBe(true)
  })

  it('先发请求被后发请求覆盖后 isLatest 返回 false', () => {
    const gate = new LatestOnly()
    const first = gate.begin()
    const second = gate.begin()
    expect(gate.isLatest(first)).toBe(false)
    expect(gate.isLatest(second)).toBe(true)
  })

  it('独立实例互不影响', () => {
    const a = new LatestOnly()
    const b = new LatestOnly()
    const sa = a.begin() // a.seq = 1
    b.begin()
    b.begin()            // b.seq = 2
    expect(a.isLatest(sa)).toBe(true)  // a 只认自己实例的序号
    expect(b.isLatest(sa)).toBe(false) // b 不认 a 的序号
    expect(a.isLatest(2)).toBe(false)
    expect(b.isLatest(2)).toBe(true)
  })
})
