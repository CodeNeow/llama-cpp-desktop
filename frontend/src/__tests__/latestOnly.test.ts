import { describe, it, expect } from 'vitest'
import { LatestOnly } from '../lib/latestOnly'

describe('LatestOnly', () => {
  it('begin returns incrementing sequence number', () => {
    const gate = new LatestOnly()
    expect(gate.begin()).toBe(1)
    expect(gate.begin()).toBe(2)
    expect(gate.begin()).toBe(3)
  })

  it('latest request isLatest returns true', () => {
    const gate = new LatestOnly()
    const seq = gate.begin()
    expect(gate.isLatest(seq)).toBe(true)
  })

  it('earlier request overwritten by later request, isLatest returns false', () => {
    const gate = new LatestOnly()
    const first = gate.begin()
    const second = gate.begin()
    expect(gate.isLatest(first)).toBe(false)
    expect(gate.isLatest(second)).toBe(true)
  })

  it('independent instances do not affect each other', () => {
    const a = new LatestOnly()
    const b = new LatestOnly()
    const sa = a.begin() // a.seq = 1
    b.begin()
    b.begin()            // b.seq = 2
    expect(a.isLatest(sa)).toBe(true)  // a only recognizes its own instance's sequence numbers
    expect(b.isLatest(sa)).toBe(false) // b does not recognize a's sequence numbers
    expect(a.isLatest(2)).toBe(false)
    expect(b.isLatest(2)).toBe(true)
  })
})
