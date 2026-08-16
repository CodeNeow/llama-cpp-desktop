import { describe, it, expect } from 'vitest'
import { isSystemReady } from '../lib/systemReady'

describe('isSystemReady', () => {
  // returns true only when both conditions are met
  it('llama.cpp installed and at least one model present returns true', () => {
    expect(isSystemReady(true, 1)).toBe(true)
    expect(isSystemReady(true, 3)).toBe(true)
  })

  // llama.cpp not installed returns false (regardless of model count)
  it('llama.cpp not installed returns false', () => {
    expect(isSystemReady(false, 0)).toBe(false)
    expect(isSystemReady(false, 1)).toBe(false)
    expect(isSystemReady(false, 100)).toBe(false)
  })

  // zero model count returns false (regardless of llama.cpp install status)
  it('zero model count returns false', () => {
    expect(isSystemReady(true, 0)).toBe(false)
    expect(isSystemReady(false, 0)).toBe(false)
  })
})
