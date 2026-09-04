import { describe, it, expect } from 'vitest'
import {
  MOBILE_MAX,
  TABLET_MAX,
  TABLET_LANDSCAPE_MAX,
  SIDEBAR_AUTO_COLLAPSE_WIDTH,
  shouldCollapseSidebar,
  viewportOrientation,
  isTabletLandscapeViewport,
} from '../lib/layout'

describe('breakpoint constants', () => {
  it('pin the shared three-tier breakpoint literals', () => {
    expect(MOBILE_MAX).toBe(767)
    expect(TABLET_MAX).toBe(1099)
  })

  it('sidebar collapse width stays linked to the tablet tier upper bound', () => {
    expect(SIDEBAR_AUTO_COLLAPSE_WIDTH).toBe(TABLET_MAX)
  })

  it('caps the Android landscape-tablet band at 1360px (1280x800 draft + headroom)', () => {
    expect(TABLET_LANDSCAPE_MAX).toBe(1360)
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

describe('viewportOrientation', () => {
  it.each<[number, number, 'portrait' | 'landscape']>([
    [800, 1280, 'portrait'], // Android tablet held upright
    [1280, 800, 'landscape'], // Android tablet rotated to landscape
    [1280, 1280, 'portrait'], // perfect square ties to portrait (conservative default)
    [375, 812, 'portrait'], // phone portrait
    [1920, 1080, 'landscape'], // desktop window
  ])('%ix%i -> %s', (width, height, expected) => {
    expect(viewportOrientation(width, height)).toBe(expected)
  })
})

describe('isTabletLandscapeViewport (pure viewport math, no OS gate)', () => {
  it.each<[number, number, boolean]>([
    [1280, 800, true], // design-draft target: inside the band and landscape
    [1100, 900, true], // band lower edge is inclusive (TABLET_MAX + 1)
    [1099, 800, false], // below the band: plain tablet-tier width
    [1361, 900, false], // beyond TABLET_LANDSCAPE_MAX: desktop stays desktop
    [1280, 1280, false], // square viewport ties to portrait -> outside the band
  ])('%ix%i -> %s', (width, height, expected) => {
    expect(isTabletLandscapeViewport(width, height)).toBe(expected)
  })
})
