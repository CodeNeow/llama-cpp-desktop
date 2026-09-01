/**
 * Unit tests for lib/safeArea — the Android edge-to-edge safe-area bridge.
 * Covers the pure conversion/composition math and the DOM publishing flow
 * (Wails bridge and runtime are mocked — they are the side-effect boundary);
 * the CSS var values the composed --safe-area-top/--safe-area-bottom read.
 */
import { afterEach, beforeEach, describe, expect, it, vi, type Mock } from 'vitest'

vi.mock('@wailsio/runtime', () => ({
  Events: { On: vi.fn(() => () => {}) },
}))
vi.mock('../wails', () => ({
  getSafeArea: vi.fn(),
}))

import { Events } from '@wailsio/runtime'
import { getSafeArea } from '../wails'

const eventsOn = Events.On as unknown as Mock
const getSafeAreaMock = getSafeArea as unknown as Mock

// Fresh module instance per test: the bridge keeps module-level state
// (accumulated insets, init flag) that must not leak between cases.
async function loadModule() {
  vi.resetModules()
  return await import('../lib/safeArea')
}

// Flush the microtask chain of the async refresh() path.
const flush = () => new Promise((resolve) => setTimeout(resolve, 0))

const originalInnerHeight = window.innerHeight
const originalDpr = Object.getOwnPropertyDescriptor(window, 'devicePixelRatio')

function stubViewport(height: number, dpr: number): void {
  Object.defineProperty(window, 'innerHeight', { value: height, configurable: true })
  Object.defineProperty(window, 'devicePixelRatio', { value: dpr, configurable: true })
}

function cssVar(name: string): string {
  return document.documentElement.style.getPropertyValue(name)
}

afterEach(() => {
  Object.defineProperty(window, 'innerHeight', { value: originalInnerHeight, configurable: true })
  if (originalDpr) Object.defineProperty(window, 'devicePixelRatio', originalDpr)
  for (const side of ['top', 'bottom', 'left', 'right']) {
    document.documentElement.style.removeProperty(`--safe-area-js-${side}`)
  }
  document.documentElement.style.removeProperty('--safe-area-js-keyboard')
  vi.clearAllMocks()
})

describe('safeArea pure math', () => {
  it.each([
    [30, 2, 15],
    [84, 2, 42],
    [33, 2.5, 13.2],
    [100, 1, 100],
  ])('pxToCssPx(%d, %d) → %d', async (px, dpr, expected) => {
    const { pxToCssPx } = await import('../lib/safeArea')
    expect(pxToCssPx(px, dpr)).toBeCloseTo(expected, 6)
  })

  it('pxToCssPx guards dpr <= 0 and clamps negative insets', async () => {
    const { pxToCssPx } = await import('../lib/safeArea')
    expect(pxToCssPx(30, 0)).toBe(30)
    expect(pxToCssPx(30, -1)).toBe(30)
    expect(pxToCssPx(-5, 2)).toBe(0)
  })

  it('mergeInsets takes the per-side maximum', async () => {
    const { mergeInsets } = await import('../lib/safeArea')
    expect(
      mergeInsets(
        { top: 84, bottom: 0, left: 0, right: 5 },
        { top: 24, bottom: 48, left: 3, right: 0 },
      ),
    ).toEqual({ top: 84, bottom: 48, left: 3, right: 5 })
  })

  it.each([
    [400, 0, 400], // no viewport resize: pad the full keyboard height
    [400, 400, 0], // viewport already shrank by the keyboard: nothing to pad
    [400, 150, 250], // partial resize: pad the remainder only
    [400, -100, 400], // unrelated resize growth: clamp and pad in full
  ])('imeAvoidancePx(ime=%i, shrink=%i) → %i', async (ime, shrink, expected) => {
    const { imeAvoidancePx } = await import('../lib/safeArea')
    expect(imeAvoidancePx(ime, shrink)).toBe(expected)
  })
})

describe('initSafeArea DOM publishing', () => {
  beforeEach(() => {
    stubViewport(768, 2)
  })

  it('publishes the binding pull as CSS vars (physical px ÷ dpr)', async () => {
    getSafeAreaMock.mockResolvedValue({ top: 84, bottom: 28, left: 0, right: 0 })
    const { initSafeArea } = await loadModule()
    initSafeArea()
    await flush()
    expect(cssVar('--safe-area-js-top')).toBe('42px')
    expect(cssVar('--safe-area-js-bottom')).toBe('14px')
    expect(cssVar('--safe-area-js-left')).toBe('0px')
    expect(cssVar('--safe-area-js-right')).toBe('0px')
  })

  it('splits pushes into the system-bar band and the keyboard channel', async () => {
    getSafeAreaMock.mockResolvedValue({ top: 0, bottom: 28, left: 0, right: 0 })
    const { initSafeArea } = await loadModule()
    initSafeArea()
    await flush()

    expect(eventsOn).toHaveBeenCalledWith('common:safearea', expect.any(Function))
    const onPush = eventsOn.mock.calls[0][1] as (data: unknown) => void

    // Keyboard up (800 physical px = 400 css px), viewport not yet resized:
    // the system-bar band stays nav-bar-only (the fixed tab bar must not
    // ride the keyboard) and the full remainder goes to --keyboard-inset.
    onPush({ top: 84, bottom: 28, left: 0, right: 0, ime: 800 })
    expect(cssVar('--safe-area-js-top')).toBe('42px')
    expect(cssVar('--safe-area-js-bottom')).toBe('14px')
    expect(cssVar('--safe-area-js-keyboard')).toBe('400px')

    // The browser viewport shrinks for the keyboard (other browsers /
    // OSes that do resize): the keyboard remainder drops to zero.
    Object.defineProperty(window, 'innerHeight', { value: 368, configurable: true })
    window.dispatchEvent(new Event('resize'))
    await flush()
    expect(cssVar('--safe-area-js-keyboard')).toBe('0px')
    expect(cssVar('--safe-area-js-bottom')).toBe('14px')
  })

  it('unwraps the real native payload: WailsEvent wrapper + JSON string', async () => {
    // The actual on-device contract: Events.On delivers the runtime's
    // WailsEvent wrapper ({name, data}) and MainActivity emits the payload
    // as a JSON string (bridge.emitEvent(name, JSONObject.toString())).
    getSafeAreaMock.mockResolvedValue({ top: 0, bottom: 28, left: 0, right: 0 })
    const { initSafeArea } = await loadModule()
    initSafeArea()
    await flush()
    const onPush = eventsOn.mock.calls[0][1] as (data: unknown) => void

    onPush({
      name: 'common:safearea',
      data: JSON.stringify({ top: 84, bottom: 28, left: 0, right: 0, ime: 800 }),
    })
    expect(cssVar('--safe-area-js-top')).toBe('42px')
    expect(cssVar('--safe-area-js-keyboard')).toBe('400px')
  })

  it('degrades malformed JSON strings inside the wrapper to zeros', async () => {
    getSafeAreaMock.mockResolvedValue({ top: 0, bottom: 0, left: 0, right: 0 })
    const { initSafeArea } = await loadModule()
    initSafeArea()
    await flush()
    const onPush = eventsOn.mock.calls[0][1] as (data: unknown) => void
    expect(() =>
      onPush({ name: 'common:safearea', data: '{not-json' }),
    ).not.toThrow()
    expect(cssVar('--safe-area-js-top')).toBe('0px')
  })

  it('keeps the last nav-bar padding after the keyboard closes', async () => {
    getSafeAreaMock.mockResolvedValue({ top: 0, bottom: 28, left: 0, right: 0 })
    const { initSafeArea } = await loadModule()
    initSafeArea()
    await flush()
    const onPush = eventsOn.mock.calls[0][1] as (data: unknown) => void

    onPush({ top: 0, bottom: 28, left: 0, right: 0, ime: 800 })
    expect(cssVar('--safe-area-js-keyboard')).toBe('400px')
    expect(cssVar('--safe-area-js-bottom')).toBe('14px')
    onPush({ top: 0, bottom: 28, left: 0, right: 0, ime: 0 })
    expect(cssVar('--safe-area-js-keyboard')).toBe('0px')
    expect(cssVar('--safe-area-js-bottom')).toBe('14px')
  })

  it('degrades malformed push payloads to zeros without throwing', async () => {
    getSafeAreaMock.mockResolvedValue({ top: 0, bottom: 0, left: 0, right: 0 })
    const { initSafeArea } = await loadModule()
    initSafeArea()
    await flush()
    const onPush = eventsOn.mock.calls[0][1] as (data: unknown) => void
    expect(() => onPush('not-an-object')).not.toThrow()
    expect(() => onPush(null)).not.toThrow()
    expect(cssVar('--safe-area-js-top')).toBe('0px')
  })

  it('survives a rejecting binding (desktop / standalone vite)', async () => {
    getSafeAreaMock.mockRejectedValue(new Error('no backend'))
    const { initSafeArea } = await loadModule()
    expect(() => initSafeArea()).not.toThrow()
    await flush()
    expect(cssVar('--safe-area-js-top')).toBe('')
  })

  it('is idempotent: one subscription no matter how often it runs', async () => {
    const { initSafeArea } = await loadModule()
    initSafeArea()
    initSafeArea()
    initSafeArea()
    expect(eventsOn).toHaveBeenCalledTimes(1)
  })
})
