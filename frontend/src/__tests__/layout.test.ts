import { describe, it, expect } from 'vitest'
import {
  MOBILE_MAX,
  TABLET_MAX,
  TABLET_PORTRAIT_VIEWPORT_WIDTH,
  SIDEBAR_AUTO_COLLAPSE_WIDTH,
  shouldCollapseSidebar,
  viewportMetaContent,
} from '../lib/layout'

describe('breakpoint constants', () => {
  it('pin the shared three-tier breakpoint literals', () => {
    expect(MOBILE_MAX).toBe(767)
    expect(TABLET_MAX).toBe(1099)
  })

  it('sidebar collapse width stays linked to the tablet tier upper bound', () => {
    expect(SIDEBAR_AUTO_COLLAPSE_WIDTH).toBe(TABLET_MAX)
  })

  it('tablet portrait viewport width sits inside the phone tier', () => {
    expect(TABLET_PORTRAIT_VIEWPORT_WIDTH).toBe(430)
    expect(TABLET_PORTRAIT_VIEWPORT_WIDTH).toBeLessThanOrEqual(MOBILE_MAX)
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

describe('viewportMetaContent (Android tablet viewport switching)', () => {
  const SCALED = 'width=430, viewport-fit=cover, interactive-widget=resizes-content'

  it('keeps the default meta on non-Android platforms at any device size', () => {
    expect(viewportMetaContent(false, 1920, false)).toBeNull()
    expect(viewportMetaContent(false, 800, true)).toBeNull()
    expect(viewportMetaContent(false, 390, true)).toBeNull()
  })

  it('keeps the default meta for an Android phone (device min side <= 767)', () => {
    expect(viewportMetaContent(true, 390, true)).toBeNull()
    expect(viewportMetaContent(true, 390, false)).toBeNull()
    expect(viewportMetaContent(true, 767, true)).toBeNull()
  })

  it('forces the fixed 430px meta for an Android tablet in portrait', () => {
    expect(viewportMetaContent(true, 800, true)).toBe(SCALED)
    expect(viewportMetaContent(true, 768, true)).toBe(SCALED)
    expect(viewportMetaContent(true, 1600, true)).toBe(SCALED)
  })

  it('keeps the default meta for an Android tablet in landscape (desktop tier natively)', () => {
    // isPortrait comes from matchMedia, not from screen width vs height —
    // the Android WebView does NOT rotate its screen dimensions on rotation.
    expect(viewportMetaContent(true, 800, false)).toBeNull()
    expect(viewportMetaContent(true, 1280, false)).toBeNull()
  })
})
