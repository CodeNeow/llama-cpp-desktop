// @vitest-environment happy-dom
import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { nextTick } from 'vue'
import Chat from '../views/Chat.vue'
import { setPlatform, buildPlatformState } from '../lib/platform'

// Mock the Wails bridge (window.go is injected only by the Wails runtime).
// Only side-effect-bearing modules are mocked; pure lib/ modules are imported
// and exercised for real.
vi.mock('../wails', () => ({
  getServerStatus: vi.fn(() => Promise.resolve({ running: false })),
  getModels: vi.fn(() => Promise.resolve([])),
  getServerConfig: vi.fn(() => Promise.resolve({ port: 8080 })),
  getLlamaCpp: vi.fn(() => Promise.resolve({ installed: true })),
  startServerWithModel: vi.fn(() => Promise.resolve()),
  unloadModel: vi.fn(() => Promise.resolve()),
}))

// vue-router's useRouter is called in setup but only used on user actions;
// provide a no-op stub so mounting does not throw in the test renderer.
vi.mock('vue-router', async () => {
  const actual = await vi.importActual<typeof import('vue-router')>('vue-router')
  return {
    ...actual,
    useRouter: () => ({
      push: vi.fn(),
      replace: vi.fn(),
      go: vi.fn(),
      back: vi.fn(),
      forward: vi.fn(),
    }),
  }
})

beforeEach(() => {
  localStorage.clear()
  vi.clearAllMocks()
})

afterEach(() => {
  document.body.innerHTML = ''
  vi.unstubAllGlobals()
  // Reset platform back to desktop default so later tests start clean.
  setPlatform(buildPlatformState('windows', Number.POSITIVE_INFINITY))
})

describe('Chat.vue Teleport conditional rendering', () => {
  // Desktop/tablet tier: the params sheet Teleport must NOT mount at all.
  // The desktop path uses the in-flow popover (.params-popover), so
  // .params-sheet-root must never appear in document.body, even when the
  // params panel is toggled open.
  it('does not render teleported params sheet on desktop tier', async () => {
    setPlatform(buildPlatformState('windows', 1200))
    const wrapper = mount(Chat, { attachTo: document.body })
    await flushPromises()
    await nextTick()

    expect(document.querySelector('.params-sheet-root')).toBeNull()

    await wrapper.find('.chat-settings-btn').trigger('click')
    await nextTick()

    expect(document.querySelector('.params-sheet-root')).toBeNull()
    expect(wrapper.find('.params-popover').exists()).toBe(true)

    wrapper.unmount()
  })

  // Phone tier: the Teleport IS rendered, and when showParams is true the
  // teleported bottom-sheet markup appears in document.body. This locks in
  // the existing mobile behavior while proving the Teleport is no longer
  // silently absent on phone widths.
  it('renders teleported params sheet on mobile tier when showParams is true', async () => {
    setPlatform(buildPlatformState('windows', 375))
    const wrapper = mount(Chat, { attachTo: document.body })
    await flushPromises()
    await nextTick()

    // Closed by default on a fresh mount: sheet content not in DOM yet because
    // showParams starts false.
    expect(document.querySelector('.params-sheet-root')).toBeNull()

    // Toggle the params panel open via the toolbar button.
    await wrapper.find('.chat-settings-btn').trigger('click')
    await nextTick()

    // The sheet root is teleported to <body> and must be present.
    const sheetRoot = document.querySelector('.params-sheet-root')
    expect(sheetRoot).not.toBeNull()

    // Confirm the teleport target is really document.body, not an in-flow
    // fallback: this branch depends on <Teleport to="body"> being active.
    expect(sheetRoot?.parentNode).toBe(document.body)

    // The inner dialog carries the expected ARIA labeling.
    const sheet = sheetRoot?.querySelector('.params-sheet')
    expect(sheet?.getAttribute('role')).toBe('dialog')
    expect(sheet?.getAttribute('aria-modal')).toBe('true')

    wrapper.unmount()
  })

  // Android tablet in PORTRAIT (tablet draft frame ⑤): the params editor is
  // the centered modal card variant of the same Teleported overlay — the
  // sheet root mounts, the dialog carries the --modal class, and no
  // persistent rail exists (the dedicated tablet layouts are gone).
  it('renders the modal-card params overlay on Android tablet portrait', async () => {
    setPlatform(buildPlatformState('android', 800, 'arm64'))
    const wrapper = mount(Chat, { attachTo: document.body })
    await flushPromises()
    await nextTick()

    expect(document.querySelector('.params-sheet-root')).toBeNull()

    await wrapper.find('.chat-settings-btn').trigger('click')
    await nextTick()

    const sheetRoot = document.querySelector('.params-sheet-root')
    expect(sheetRoot).not.toBeNull()
    expect(sheetRoot?.querySelector('.params-sheet')?.classList.contains('params-sheet--modal')).toBe(true)

    // No persistent rail on any tier (Track B removed)
    expect(wrapper.find('.chat-rail').exists()).toBe(false)

    wrapper.unmount()
  })

  // Android tablet in LANDSCAPE: the viewport meta keeps the default
  // device-width content, so the wide viewport classifies straight into the
  // DESKTOP tier and the anchored popover renders — no dedicated landscape
  // layout, no Teleported overlay.
  it('uses the desktop popover on Android tablet landscape (no dedicated layout)', async () => {
    setPlatform(buildPlatformState('android', 1280, 'arm64'))
    const wrapper = mount(Chat, { attachTo: document.body })
    await flushPromises()
    await nextTick()

    // Desktop tier: no rail anywhere
    expect(wrapper.find('.chat-rail').exists()).toBe(false)
    expect(wrapper.find('.rail-card--params').exists()).toBe(false)

    // Toggling the params state opens the desktop popover, never the
    // Teleported sheet
    await wrapper.find('.chat-settings-btn').trigger('click')
    await nextTick()
    expect(document.querySelector('.params-sheet-root')).toBeNull()
    expect(wrapper.find('.params-popover').exists()).toBe(true)

    wrapper.unmount()
  })

  it('keeps messages-area scroll container and inline banner placement on the desktop tier', async () => {
    setPlatform(buildPlatformState('android', 1280, 'arm64'))
    const wrapper = mount(Chat, { attachTo: document.body })
    await flushPromises()
    await nextTick()

    // The scrollable messages area stays the stick-to-bottom wiring target in
    // every layout branch
    expect(wrapper.find('.chat-body .messages-area').exists()).toBe(true)
    // Fresh idle mount: no precheck banner
    expect(wrapper.find('.chat-precheck-stack').exists()).toBe(false)

    wrapper.unmount()
  })
})
