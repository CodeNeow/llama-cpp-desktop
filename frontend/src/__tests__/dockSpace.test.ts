// @vitest-environment happy-dom
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { defineComponent, ref, nextTick } from 'vue'
import { dockReserve, dockReservePx, dockWidth, dockWidthPx, dockSide, dockLane, useDockReserve } from '../lib/dockSpace'

// Mock the Wails bridge (window.go is injected only by the Wails runtime).
// Update-related exports are included because lib/update imports the same
// module transitively via TaskDock.
vi.mock('../wails', () => ({
  getLlamaCppDownloadStatus: vi.fn(() => Promise.resolve({ status: 'idle', progress: 0 })),
  getDownloadTasks: vi.fn(() => Promise.resolve([])),
  getServerStatus: vi.fn(() => Promise.resolve({ running: true })),
  getLoadedModels: vi.fn(() =>
    Promise.resolve([{ id: 'test-model', type: 'chat', status: 'loaded' }])
  ),
  unloadModel: vi.fn(() => Promise.resolve()),
  checkForUpdate: vi.fn(() => Promise.resolve({ hasUpdate: false, version: '', notes: '', published: '' })),
  startUpdateDownload: vi.fn(() => Promise.resolve()),
  stopUpdateDownload: vi.fn(() => Promise.resolve()),
  installUpdate: vi.fn(() => Promise.resolve()),
  getUpdateDownloadStatus: vi.fn(() =>
    Promise.resolve({
      status: 'idle', progress: 0, total: 0, downloaded: 0,
      version: '', filePath: '', error: '', kind: '', installer: false,
    })
  ),
}))

// ─── Controllable ResizeObserver stub ───────────────────────────────────────
// Records instances and calls so tests can assert observe/disconnect behavior
// and manually fire the callback.

type ROEntry = { target: HTMLElement }

class MockResizeObserver {
  static instances: MockResizeObserver[] = []
  readonly callback: (entries: ROEntry[], observer: MockResizeObserver) => void
  readonly observe = vi.fn()
  readonly disconnect = vi.fn()
  readonly unobserve = vi.fn()
  constructor(callback: (entries: ROEntry[], observer: MockResizeObserver) => void) {
    this.callback = callback
    MockResizeObserver.instances.push(this)
  }
  /** Fire the callback as the browser would after a size change of `target`. */
  trigger(target: HTMLElement) {
    this.callback([{ target }], this)
  }
  static get last(): MockResizeObserver {
    const inst = MockResizeObserver.instances[MockResizeObserver.instances.length - 1]
    if (!inst) throw new Error('no MockResizeObserver instance created')
    return inst
  }
}

/** happy-dom's offsetHeight is always 0; stub a concrete height instead. */
function stubOffsetHeight(el: HTMLElement, height: number) {
  Object.defineProperty(el, 'offsetHeight', { value: height, configurable: true })
}

/** happy-dom's offsetWidth is always 0 too; stub a concrete width. */
function stubOffsetWidth(el: HTMLElement, width: number) {
  Object.defineProperty(el, 'offsetWidth', { value: width, configurable: true })
}

beforeEach(() => {
  MockResizeObserver.instances = []
})

afterEach(() => {
  document.body.innerHTML = ''
  vi.unstubAllGlobals()
  // dockSpace keeps module-level state (loaded once per file); drive it back to
  // 0 / the default side / no lane so each test starts from a clean reserve.
  dockReserve.value = 0
  dockWidth.value = 0
  dockSide.value = 'right'
  dockLane.value = 'none'
})

// ─── dockReservePx (pure) ────────────────────────────────────────────────────

describe('dockReservePx', () => {
  it('returns 0 when not visible', () => {
    expect(dockReservePx(false, 39)).toBe(0)
    expect(dockReservePx(false, 500)).toBe(0)
  })

  it('returns height + bottom offset + gap when visible', () => {
    // 39 (collapsed dock header) + 29 (bottom: 29px) + 8 (content gap)
    expect(dockReservePx(true, 39)).toBe(76)
  })

  it('returns 0 for invalid heights', () => {
    expect(dockReservePx(true, 0)).toBe(0)
    expect(dockReservePx(true, -5)).toBe(0)
    expect(dockReservePx(true, NaN)).toBe(0)
    expect(dockReservePx(true, Infinity)).toBe(0)
  })
})

// ─── dockWidthPx (pure) ──────────────────────────────────────────────────────

describe('dockWidthPx', () => {
  it('returns the measured width unchanged when visible (no offsets added)', () => {
    expect(dockWidthPx(true, 40)).toBe(40)
    expect(dockWidthPx(true, 80)).toBe(80)
  })

  it('returns 0 when not visible', () => {
    expect(dockWidthPx(false, 40)).toBe(0)
    expect(dockWidthPx(false, 500)).toBe(0)
  })

  it('returns 0 for invalid widths', () => {
    expect(dockWidthPx(true, 0)).toBe(0)
    expect(dockWidthPx(true, -5)).toBe(0)
    expect(dockWidthPx(true, NaN)).toBe(0)
    expect(dockWidthPx(true, Infinity)).toBe(0)
  })
})

// ─── dockLane (chat composer-band lane, written by TaskDock) ─────────────────

describe('dockLane', () => {
  it("starts retracted ('none') — no lane until a docked capsule claims one", () => {
    expect(dockLane.value).toBe('none')
  })
})

// ─── useDockReserve (composable) ─────────────────────────────────────────────

describe('useDockReserve', () => {
  /**
   * Minimal harness: one div whose template ref feeds the composable; the two
   * refs are exposed on the vm for the test to flip.
   */
  function mountHarness() {
    const Host = defineComponent({
      setup() {
        const el = ref<HTMLElement | null>(null)
        const visible = ref(false)
        useDockReserve(el, visible)
        return { el, visible }
      },
      template: `<div ref="el"></div>`,
    })
    const wrapper = mount(Host, { attachTo: document.body })
    const vm = wrapper.vm as unknown as {
      el: HTMLElement | null
      visible: boolean
      $refs: { el?: HTMLElement }
    }
    return { wrapper, vm }
  }

  function harnessElement(vm: { el: HTMLElement | null; $refs: { el?: HTMLElement } }): HTMLElement {
    const node = vm.el ?? vm.$refs.el
    if (!node) throw new Error('harness element not mounted')
    return node
  }

  it('keeps reserve 0 while not visible', () => {
    vi.stubGlobal('ResizeObserver', MockResizeObserver)
    const { wrapper } = mountHarness()
    expect(dockReserve.value).toBe(0)
    expect(MockResizeObserver.instances).toHaveLength(0)
    wrapper.unmount()
  })

  it('measures immediately and observes when visible', async () => {
    vi.stubGlobal('ResizeObserver', MockResizeObserver)
    const { wrapper, vm } = mountHarness()
    stubOffsetHeight(harnessElement(vm), 100)

    vm.visible = true
    await nextTick() // watchEffect (flush: pre) re-runs on the next tick

    expect(dockReserve.value).toBe(137) // 100 + 29 + 8
    expect(MockResizeObserver.instances).toHaveLength(1)
    expect(MockResizeObserver.instances[0].observe).toHaveBeenCalledTimes(1)
    wrapper.unmount()
  })

  it('re-measures when the ResizeObserver callback fires', async () => {
    vi.stubGlobal('ResizeObserver', MockResizeObserver)
    const { wrapper, vm } = mountHarness()
    const node = harnessElement(vm)
    stubOffsetHeight(node, 100)
    vm.visible = true
    await nextTick()
    expect(dockReserve.value).toBe(137)

    stubOffsetHeight(node, 60)
    MockResizeObserver.last.trigger(node)
    expect(dockReserve.value).toBe(97) // 60 + 29 + 8
    wrapper.unmount()
  })

  it('zeroes the reserve and disconnects when hidden', async () => {
    vi.stubGlobal('ResizeObserver', MockResizeObserver)
    const { wrapper, vm } = mountHarness()
    const node = harnessElement(vm)
    stubOffsetHeight(node, 100)
    vm.visible = true
    await nextTick()
    expect(dockReserve.value).toBe(137)

    vm.visible = false
    await nextTick()
    expect(dockReserve.value).toBe(0)
    expect(MockResizeObserver.instances[0].disconnect).toHaveBeenCalled()
    wrapper.unmount()
  })

  it('resets the reserve on unmount', async () => {
    vi.stubGlobal('ResizeObserver', MockResizeObserver)
    const { wrapper, vm } = mountHarness()
    stubOffsetHeight(harnessElement(vm), 100)
    vm.visible = true
    await nextTick()
    expect(dockReserve.value).toBe(137)

    wrapper.unmount()
    expect(dockReserve.value).toBe(0)
  })

  it('falls back to immediate measurement without ResizeObserver support', async () => {
    // jsdom-like environment: no ResizeObserver at all; the immediate
    // measurement still publishes the reserve.
    vi.stubGlobal('ResizeObserver', undefined)
    const { wrapper, vm } = mountHarness()
    stubOffsetHeight(harnessElement(vm), 50)
    vm.visible = true
    await nextTick()
    expect(dockReserve.value).toBe(87) // 50 + 29 + 8
    wrapper.unmount()
    expect(dockReserve.value).toBe(0)
  })

  // ─── Width lane (--dock-width) ─────────────────────────────────────────────

  it('publishes the measured pill width alongside the height when visible', async () => {
    vi.stubGlobal('ResizeObserver', MockResizeObserver)
    const { wrapper, vm } = mountHarness()
    const node = harnessElement(vm)
    stubOffsetHeight(node, 100)
    stubOffsetWidth(node, 40)
    vm.visible = true
    await nextTick()

    // Height keeps its offset arithmetic; width passes through untouched.
    expect(dockReserve.value).toBe(137)
    expect(dockWidth.value).toBe(40)
    wrapper.unmount()
  })

  it('re-measures the width when the ResizeObserver callback fires', async () => {
    // A second pill segment (e.g. download + model counters) widens the pill;
    // the observer must republish the new width for the reserved lane.
    vi.stubGlobal('ResizeObserver', MockResizeObserver)
    const { wrapper, vm } = mountHarness()
    const node = harnessElement(vm)
    stubOffsetHeight(node, 100)
    stubOffsetWidth(node, 40)
    vm.visible = true
    await nextTick()
    expect(dockWidth.value).toBe(40)

    stubOffsetWidth(node, 80)
    MockResizeObserver.last.trigger(node)
    expect(dockWidth.value).toBe(80)
    wrapper.unmount()
  })

  it('zeroes the width when hidden and on unmount', async () => {
    vi.stubGlobal('ResizeObserver', MockResizeObserver)
    const { wrapper, vm } = mountHarness()
    const node = harnessElement(vm)
    stubOffsetHeight(node, 100)
    stubOffsetWidth(node, 40)
    vm.visible = true
    await nextTick()
    expect(dockWidth.value).toBe(40)

    vm.visible = false
    await nextTick()
    expect(dockWidth.value).toBe(0)

    stubOffsetWidth(node, 40)
    vm.visible = true
    await nextTick()
    expect(dockWidth.value).toBe(40)
    wrapper.unmount()
    expect(dockWidth.value).toBe(0)
  })
})
