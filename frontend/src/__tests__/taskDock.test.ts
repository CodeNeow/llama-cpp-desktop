// @vitest-environment happy-dom
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { nextTick } from 'vue'
import TaskDock from '../components/TaskDock.vue'
import { dockReserve } from '../lib/dockSpace'
import { t } from '../lib/i18n'
import { getDownloadTasks, getServerStatus } from '../wails'

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
// Records instances so tests can manually fire the callback on the last one.

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

/** Mount TaskDock with the default mocks and settle the first poll so the
 * dock becomes visible (1 loaded model by default). */
async function mountDock() {
  const wrapper = mount(TaskDock, { attachTo: document.body })
  await flushPromises() // first poll resolves -> visible
  await nextTick() // template ref + watchEffect settle
  return wrapper
}

/** Wait for a <Transition> leave to finish removing the leaving element.
 * Vue applies/removes the transition classes across two requestAnimationFrame
 * hops before removal; in happy-dom those are 0ms timers and the CSS timing
 * resolves instantly (SFC styles are not loaded), so the render tick plus a
 * macrotask sleep longer than both frames is enough. */
async function flushTransitionLeave() {
  await nextTick()
  await new Promise(resolve => setTimeout(resolve, 20))
  await nextTick()
}

beforeEach(() => {
  MockResizeObserver.instances = []
})

afterEach(() => {
  document.body.innerHTML = ''
  vi.unstubAllGlobals()
  // dockSpace keeps module-level state (loaded once per file); drive it back to
  // 0 so each test starts from a clean reserve.
  dockReserve.value = 0
  vi.clearAllMocks()
})

// ─── Collapsed pill (default state) ─────────────────────────────────────────

describe('TaskDock collapsed pill', () => {
  it('renders collapsed by default: pill only, no popover', async () => {
    vi.stubGlobal('ResizeObserver', MockResizeObserver)
    const wrapper = await mountDock()

    expect(wrapper.find('.dock-pill').exists()).toBe(true)
    expect(wrapper.find('.dock-popover').exists()).toBe(false)
    expect(wrapper.find('.dock-pill').attributes('aria-label')).toBe(t('dock.title'))
    wrapper.unmount()
  })

  it('shows the loaded model count in the pill', async () => {
    vi.stubGlobal('ResizeObserver', MockResizeObserver)
    const wrapper = await mountDock()

    // Default mocks: server running with exactly 1 loaded model
    expect(wrapper.findAll('.pill-seg')).toHaveLength(1)
    expect(wrapper.find('.dock-pill').text()).toContain('1')
    wrapper.unmount()
  })

  it('shows the active download count in the pill', async () => {
    vi.stubGlobal('ResizeObserver', MockResizeObserver)
    // One downloading model task and no running server, so the download count
    // is the pill's only segment
    vi.mocked(getDownloadTasks).mockImplementationOnce(() =>
      Promise.resolve([{ id: 'task-1', fileName: 'model.gguf', status: 'downloading', progress: 10 }])
    )
    vi.mocked(getServerStatus).mockImplementationOnce(() =>
      Promise.resolve({ running: false, log: [] })
    )

    const wrapper = await mountDock()
    expect(wrapper.findAll('.pill-seg')).toHaveLength(1)
    expect(wrapper.find('.dock-pill').text()).toContain('1')
    wrapper.unmount()
  })
})

// ─── Popover expand/collapse interactions ────────────────────────────────────

describe('TaskDock popover interactions', () => {
  it('expands the popover when the pill is clicked', async () => {
    vi.stubGlobal('ResizeObserver', MockResizeObserver)
    const wrapper = await mountDock()

    await wrapper.find('.dock-pill').trigger('click')
    await nextTick() // watchEffect attaches the document listeners
    expect(wrapper.find('.dock-popover').exists()).toBe(true)
    wrapper.unmount()
  })

  it('collapses on outside click but stays open for clicks inside the dock', async () => {
    vi.stubGlobal('ResizeObserver', MockResizeObserver)
    const wrapper = await mountDock()

    await wrapper.find('.dock-pill').trigger('click')
    await nextTick()
    expect(wrapper.find('.dock-popover').exists()).toBe(true)

    // Click on the body (outside the dock): collapse
    document.body.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    await flushTransitionLeave() // <Transition> removes the card asynchronously
    expect(wrapper.find('.dock-popover').exists()).toBe(false)

    // Re-expand, then click inside the popover: must stay open
    await wrapper.find('.dock-pill').trigger('click')
    await nextTick()
    wrapper.find('.dock-popover').element.dispatchEvent(
      new MouseEvent('click', { bubbles: true })
    )
    await nextTick()
    expect(wrapper.find('.dock-popover').exists()).toBe(true)
    wrapper.unmount()
  })

  it('collapses on Escape', async () => {
    vi.stubGlobal('ResizeObserver', MockResizeObserver)
    const wrapper = await mountDock()

    await wrapper.find('.dock-pill').trigger('click')
    await nextTick()
    expect(wrapper.find('.dock-popover').exists()).toBe(true)

    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))
    await flushTransitionLeave() // <Transition> removes the card asynchronously
    expect(wrapper.find('.dock-popover').exists()).toBe(false)
    wrapper.unmount()
  })

  it('hides the pill visually while the popover is open (morph)', async () => {
    vi.stubGlobal('ResizeObserver', MockResizeObserver)
    // SFC styles are not loaded in unit tests (vitest css processing is off),
    // so mirror the component's pill/morph rules to make the pill's computed
    // visibility observable in happy-dom.
    const style = document.createElement('style')
    style.textContent = `
      .dock-pill { visibility: visible; }
      .dock-pill-hidden { visibility: hidden; opacity: 0; pointer-events: none; }
    `
    document.head.appendChild(style)
    const wrapper = await mountDock()

    await wrapper.find('.dock-pill').trigger('click')
    await nextTick()
    // Morph: while the card is open the pill stays in the DOM (its layout
    // placeholder keeps the measured --dock-reserve constant) but is invisible.
    const pill = wrapper.find('.dock-pill')
    expect(pill.exists()).toBe(true)
    expect(getComputedStyle(pill.element).visibility).toBe('hidden')

    // Outside click collapses the card; the pill fades back in.
    document.body.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    await nextTick()
    expect(getComputedStyle(wrapper.find('.dock-pill').element).visibility).toBe('visible')

    style.remove()
    wrapper.unmount()
  })
})

// ─── Dock-space wiring smoke (migrated from dockSpace.test.ts) ──────────────

describe('TaskDock dock-space wiring', () => {
  it('publishes the measured root height; the popover never changes it', async () => {
    vi.stubGlobal('ResizeObserver', MockResizeObserver)
    const wrapper = await mountDock()

    expect(wrapper.find('.task-dock').exists()).toBe(true)
    const dockEl = wrapper.find<HTMLElement>('.task-dock').element
    stubOffsetHeight(dockEl, 120)

    // ResizeObserver reports the initial size asynchronously; simulate that
    // initial callback delivery here
    MockResizeObserver.last.trigger(dockEl)
    expect(dockReserve.value).toBe(144) // 120 + 16 + 8

    // Expanding the popover must not change the root's measured height: the
    // popover is absolutely positioned, so the reserve stays constant
    await wrapper.find('.dock-pill').trigger('click')
    await nextTick()
    MockResizeObserver.last.trigger(dockEl)
    expect(dockReserve.value).toBe(144)

    // Unmount clears the polling interval and the reserved space
    wrapper.unmount()
    expect(dockReserve.value).toBe(0)
  })
})
