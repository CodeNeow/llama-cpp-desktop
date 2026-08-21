import { describe, it, expect } from 'vitest'
import { isNearBottom } from '../lib/scroll'

describe('isNearBottom', () => {
  // scrollTop at its maximum (scrollHeight - clientHeight): exactly at the bottom
  it('at the bottom returns true', () => {
    expect(isNearBottom(200, 300, 100)).toBe(true)
  })

  // scrolled away from the bottom
  it('scrolled up returns false', () => {
    expect(isNearBottom(0, 300, 100)).toBe(false)
    expect(isNearBottom(150, 300, 100)).toBe(false)
  })

  // content shorter than the viewport: fully visible, nothing to scroll
  it('small box fully visible returns true', () => {
    expect(isNearBottom(0, 50, 100)).toBe(true)
  })

  // exactly at the threshold re-attaches; one pixel further away does not
  it('re-attach boundary sits exactly at the threshold', () => {
    expect(isNearBottom(176, 300, 100)).toBe(true)  // 24px from the bottom
    expect(isNearBottom(175, 300, 100)).toBe(false) // 25px from the bottom
  })

  it('custom threshold is respected', () => {
    expect(isNearBottom(196, 300, 100, 8)).toBe(true)  // 4px from the bottom
    expect(isNearBottom(190, 300, 100, 8)).toBe(false) // 10px from the bottom
  })
})
