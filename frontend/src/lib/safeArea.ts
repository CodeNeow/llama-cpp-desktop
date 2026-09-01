/**
 * Android edge-to-edge safe-area bridge (status bar on top, gesture/nav bar +
 * soft keyboard on the bottom).
 *
 * The Android shell draws edge-to-edge (transparent system-bar colors in
 * themes.xml plus setDecorFitsSystemWindows(false) in MainActivity), and the
 * Android WebView reports env(safe-area-inset-*) ≡ 0 — so the CSS env() base
 * layer alone cannot keep content clear of the bars there. This module feeds
 * the missing insets in from the native side and publishes them as the
 * --safe-area-js-* custom properties on <html>; styles/global.css composes
 * them with env() into --safe-area-top/--safe-area-bottom. Desktop stays a
 * no-op: every source is zero there.
 *
 * Two native sources, per side the larger wins:
 *  - GetSafeArea binding (pull): system-bar insets, read at startup and again
 *    on window resize (rotation / fold changes re-dispatch bar geometry).
 *  - "common:safearea" event (push, MainActivity's decor-view insets
 *    listener): system-bar insets plus the soft-keyboard (IME) height on
 *    every insets pass — the channel is the same one the mobile features use
 *    (WailsBridge.emitEvent → app.Event.Emit → Events.On).
 *
 * Keyboard avoidance: the viewport meta already declares
 * interactive-widget=resizes-content, so on WebView builds where the IME
 * shrinks the layout viewport the page shrinks on its own. The IME inset is
 * therefore padded only insofar as the viewport did NOT already shrink for it
 * (imeAvoidancePx), which avoids double-compensating the keyboard height.
 */

import { Events } from '@wailsio/runtime'
import { getSafeArea } from '../wails'

// The event name MainActivity's insets listener emits (WailsBridge.emitEvent
// → nativeEmitEvent → app.Event.Emit).
const SAFE_AREA_EVENT = 'common:safearea'

// One side's inset values, in CSS pixels.
export interface CssInsets {
  top: number
  bottom: number
  left: number
  right: number
}

const ZERO_INSETS: CssInsets = { top: 0, bottom: 0, left: 0, right: 0 }

// pxToCssPx converts a physical-pixel inset to CSS pixels. A dpr <= 0 (some
// WebViews report 0 before first paint) falls back to a 1:1 scale; negative
// insets (never expected from the platform) clamp to 0.
export function pxToCssPx(px: number, dpr: number): number {
  const scale = dpr > 0 ? dpr : 1
  return Math.max(0, px) / scale
}

// mergeInsets takes the per-side maximum of two inset sets. Both native
// sources report the same system bars, so the merge only ever replaces a
// stale side — it cannot inflate beyond what the platform reported.
export function mergeInsets(a: CssInsets, b: CssInsets): CssInsets {
  return {
    top: Math.max(a.top, b.top),
    bottom: Math.max(a.bottom, b.bottom),
    left: Math.max(a.left, b.left),
    right: Math.max(a.right, b.right),
  }
}

// imeAvoidancePx returns how much extra bottom padding the soft keyboard
// still needs after the browser's own viewport resize is accounted for
// (interactive-widget=resizes-content). viewportShrunkCss may be negative
// (an unrelated resize) — clamped, so the full IME height is padded then.
export function imeAvoidancePx(imeCss: number, viewportShrunkCss: number): number {
  return Math.max(0, imeCss - Math.max(0, viewportShrunkCss))
}

// ─── Module state (CSS pixels) ───────────────────────────────────────────────

// Latest system-bar insets: the merge of every pull and push seen so far.
let bars: CssInsets = { ...ZERO_INSETS }
// Latest soft-keyboard height from the push channel (0 = keyboard closed).
let imeCss = 0
// Viewport height last observed with the keyboard down; the difference to the
// current height is how much the browser already absorbed of the IME.
let imeBaselineHeight = 0
let initialized = false

function hasWindow(): boolean {
  return typeof window !== 'undefined'
}

function devicePixelRatio(): number {
  return hasWindow() ? window.devicePixelRatio : 1
}

function viewportHeight(): number {
  return hasWindow() ? window.innerHeight : 0
}

// asInsets normalizes a native payload (binding struct or event data map of
// physical px) into CSS-pixel insets; unexpected shapes read as zeros.
function asInsets(raw: unknown): CssInsets {
  const src = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>
  const dpr = devicePixelRatio()
  return {
    top: pxToCssPx(Number(src.top) || 0, dpr),
    bottom: pxToCssPx(Number(src.bottom) || 0, dpr),
    left: pxToCssPx(Number(src.left) || 0, dpr),
    right: pxToCssPx(Number(src.right) || 0, dpr),
  }
}

// writeVar publishes one CSS custom property; all DOM access is guarded so a
// missing document (SSR / bare node tests) can never throw.
function writeVar(name: string, value: number): void {
  try {
    if (typeof document === 'undefined') return
    document.documentElement.style.setProperty(name, `${value}px`)
  } catch {
    // No writable DOM: nothing to publish.
  }
}

// publish recomputes --safe-area-js-* from the current sources. The bottom
// pads the system bars and, while the keyboard is up, whatever part of it the
// browser's own viewport resize did not already absorb.
function publish(): void {
  writeVar('--safe-area-js-top', bars.top)
  writeVar('--safe-area-js-left', bars.left)
  writeVar('--safe-area-js-right', bars.right)
  const viewportShrunk = Math.max(0, imeBaselineHeight - viewportHeight())
  writeVar('--safe-area-js-bottom', Math.max(bars.bottom, imeAvoidancePx(imeCss, viewportShrunk)))
}

// refresh re-pulls the binding-side system-bar insets (startup and every
// window resize).
async function refresh(): Promise<void> {
  try {
    bars = mergeInsets(bars, asInsets(await getSafeArea()))
    publish()
  } catch {
    // Binding unavailable (desktop without the method, standalone vite):
    // env() stays the only source — an expected state, not an error.
  }
}

// onPush applies a "common:safearea" event. Events.On delivers the runtime's
// WailsEvent wrapper ({name, data}) — NOT the payload itself — and the native
// side emits the payload as a JSON string (MainActivity's
// bridge.emitEvent(name, JSONObject.toString())). Unwrap and parse both
// layers; bare payloads (unit tests, older callers) keep working.
function onPush(raw: unknown): void {
  let payload: unknown = raw
  if (payload !== null && typeof payload === 'object' && 'data' in (payload as Record<string, unknown>)) {
    payload = (payload as { data?: unknown }).data
  }
  if (typeof payload === 'string') {
    try {
      payload = JSON.parse(payload)
    } catch {
      payload = {}
    }
  }
  const src = (payload && typeof payload === 'object' ? payload : {}) as Record<string, unknown>
  const currentHeight = viewportHeight()
  const ime = pxToCssPx(Number(src.ime) || 0, devicePixelRatio())
  if (ime <= 0) {
    // Keyboard down: rebase so the next IME show measures the shrink against
    // the current (keyboard-free) viewport height.
    imeBaselineHeight = currentHeight
  }
  imeCss = ime
  bars = mergeInsets(bars, asInsets(src))
  publish()
}

// onResize re-reads the binding on viewport changes (rotation, fold) and
// keeps the keyboard-shrink accounting current.
function onResize(): void {
  if (imeCss <= 0) {
    imeBaselineHeight = viewportHeight()
  }
  void refresh()
}

// initSafeArea wires the pull/push/resize channels and publishes the first
// value. Idempotent, and safe to call everywhere: no Wails runtime and no
// DOM both degrade to the env()-only CSS baseline.
export function initSafeArea(): void {
  if (initialized) return
  initialized = true
  if (!hasWindow()) return
  imeBaselineHeight = viewportHeight()
  try {
    window.addEventListener('resize', onResize, { passive: true })
    Events.On(SAFE_AREA_EVENT, onPush)
  } catch {
    // Runtime without event support: the binding pull below still works.
  }
  void refresh()
}
