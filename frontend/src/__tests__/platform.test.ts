import { describe, it, expect } from 'vitest'
import {
  parseOs,
  buildPlatformState,
  usePlatform,
  setPlatform,
  type OsId,
  type PlatformState,
} from '../lib/platform'
import { MOBILE_MAX, TABLET_MAX } from '../lib/layout'

describe('parseOs', () => {
  it.each<[string | null | undefined, OsId]>([
    ['windows', 'windows'],
    ['linux', 'linux'],
    ['darwin', 'darwin'],
    ['android', 'android'],
    ['ios', 'ios'],
    ['', 'other'],
    [null, 'other'],
    [undefined, 'other'],
    ['freebsd', 'other'],
    ['Windows', 'other'],
  ])('maps %j to %j', (raw, expected) => {
    expect(parseOs(raw)).toBe(expected)
  })
})

describe('buildPlatformState viewport tier boundaries', () => {
  it.each<[number, boolean, boolean, boolean]>([
    [MOBILE_MAX, true, false, false], // 767: mobile tier (inclusive)
    [MOBILE_MAX + 1, false, true, false], // 768: tablet tier starts
    [TABLET_MAX, false, true, false], // 1099: tablet tier (inclusive)
    [TABLET_MAX + 1, false, false, true], // 1100: desktop tier starts
    [1920, false, false, true],
  ])(
    'desktop OS at width %i -> mobile=%s tablet=%s desktop=%s',
    (width, isMobile, isTablet, isDesktop) => {
      const state = buildPlatformState('windows', width)
      expect(state.isMobile).toBe(isMobile)
      expect(state.isTablet).toBe(isTablet)
      expect(state.isDesktop).toBe(isDesktop)
    },
  )
})

describe('buildPlatformState mobile OS classification', () => {
  it('android forces isMobile at any width, including desktop-tier widths', () => {
    for (const width of [360, MOBILE_MAX, MOBILE_MAX + 1, TABLET_MAX, TABLET_MAX + 1, 1920]) {
      const state = buildPlatformState('android', width)
      expect(state.isMobile).toBe(true)
      expect(state.isTablet).toBe(false)
      expect(state.isDesktop).toBe(false)
      expect(state.isAndroid).toBe(true)
      expect(state.isIOS).toBe(false)
    }
  })

  it('ios forces isMobile at any width', () => {
    const state = buildPlatformState('ios', 1920)
    expect(state.isMobile).toBe(true)
    expect(state.isTablet).toBe(false)
    expect(state.isDesktop).toBe(false)
    expect(state.isIOS).toBe(true)
  })

  it('desktop OS at narrow width forces isMobile via the viewport rule', () => {
    const state = buildPlatformState('windows', 375)
    expect(state.isMobile).toBe(true)
    expect(state.isTablet).toBe(false)
    expect(state.isDesktop).toBe(false)
    expect(state.isAndroid).toBe(false)
    expect(state.isIOS).toBe(false)
  })
})

describe('buildPlatformState capability matrix', () => {
  it.each<[OsId, number, boolean, boolean]>([
    ['windows', 1920, true, true], // desktop windows: tray + frameless titlebar
    ['windows', MOBILE_MAX, true, false], // tray is an OS capability: still true in the mobile viewport tier
    ['linux', 1920, false, true], // linux desktop: no tray today, frameless ok
    ['darwin', 1920, false, true], // macOS desktop: no tray today, frameless ok
    ['android', 1920, false, false], // mobile OS: native chrome, no custom titlebar
    ['ios', 390, false, false],
    ['other', 1920, false, true],
  ])('%s @ %i -> supportsTray=%s supportsFramelessTitlebar=%s', (os, width, tray, titlebar) => {
    const state = buildPlatformState(os, width)
    expect(state.supportsTray).toBe(tray)
    expect(state.supportsFramelessTitlebar).toBe(titlebar)
  })
})

describe('reactive platform singleton', () => {
  it('starts from a desktop-windows default before any wiring', () => {
    const platform = usePlatform()
    expect(platform.value.os).toBe('windows')
    expect(platform.value.isDesktop).toBe(true)
    expect(platform.value.supportsTray).toBe(true)
  })

  it('reflects setPlatform updates in the shared readonly ref', () => {
    const android: PlatformState = buildPlatformState('android', 412)
    setPlatform(android)
    expect(usePlatform().value.isMobile).toBe(true)
    expect(usePlatform().value.supportsFramelessTitlebar).toBe(false)

    // Restore the default so other suites observe the pristine singleton.
    setPlatform(buildPlatformState('windows', Number.POSITIVE_INFINITY))
    expect(usePlatform().value.isDesktop).toBe(true)
  })
})
