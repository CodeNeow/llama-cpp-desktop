import { describe, it, expect } from 'vitest'
import {
  MOBILE_MAX,
  TABLET_MAX,
  SIDEBAR_AUTO_COLLAPSE_WIDTH,
  shouldCollapseSidebar,
} from '../lib/layout'

describe('breakpoint constants', () => {
  it('pin the shared three-tier breakpoint literals', () => {
    expect(MOBILE_MAX).toBe(767)
    expect(TABLET_MAX).toBe(1099)
  })

  it('sidebar collapse width stays linked to the tablet tier upper bound', () => {
    expect(SIDEBAR_AUTO_COLLAPSE_WIDTH).toBe(TABLET_MAX)
  })
})

describe('shouldCollapseSidebar', () => {
  it('breakpoint constant matches the shared 1099px narrow-viewport width', () => {
    expect(SIDEBAR_AUTO_COLLAPSE_WIDTH).toBe(1099)
  })

  it('collapses exactly at the boundary (inclusive)', () => {
    expect(shouldCollapseSidebar(1099)).toBe(true)
  })

  it('stays expanded one pixel above the boundary', () => {
    expect(shouldCollapseSidebar(1100)).toBe(false)
  })

  it('collapses below the boundary', () => {
    expect(shouldCollapseSidebar(800)).toBe(true)
  })

  it('stays expanded on wide windows', () => {
    expect(shouldCollapseSidebar(1920)).toBe(false)
  })
})
