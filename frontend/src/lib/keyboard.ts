/**
 * Soft-keyboard visibility for the phone tier.
 *
 * index.html pins `interactive-widget=resizes-content`: while the Android
 * soft keyboard is up the LAYOUT viewport shrinks, so fixed-position chrome
 * anchored to the viewport floor (the mobile bottom nav) rides up on top of
 * the composer it is supposed to sit under. The fix needs to know "keyboard
 * is open" — no DOM event carries that fact, so it is derived from two
 * signals:
 *   1. an editable element holds focus (focusin / focusout, capture phase so
 *      it sees targets before any consumer stops propagation), and
 *   2. the visual viewport is meaningfully shorter than the layout viewport
 *      (the resize heuristic; the threshold keeps cursor-caret / IME-suggestion
 *      bars and sub-pixel dvh jitter from counting as a keyboard).
 *
 * Both must hold: an editable focus alone says nothing (hardware keyboards,
 * tablets), and a viewport shrink alone happens on browser-toolbar collapse.
 * App.vue consumes the published ref and toggles the html.keyboard-open class
 * (global.css hides the mobile nav and zeroes its height band). Pure
 * predicates are exported separately so the heuristic is unit-testable
 * without a DOM or window.
 */
import { ref } from 'vue'

// Layout-vs-visual viewport height difference (px) beyond which an editable
// focus is attributed to a raised soft keyboard. Generous on purpose: false
// negatives only lose the nav-hiding polish, false positives would hide the
// nav during ordinary desktop window resizing (where no keyboard exists).
export const KEYBOARD_SHRINK_THRESHOLD_PX = 120

/** Reactive keyboard visibility: true only while editable focus + viewport shrink both hold. */
export const keyboardVisible = ref(false)

/**
 * Whether the event target is a text-entry surface: input, textarea, select
 * (opens pickers/keyboards on mobile) or any contenteditable host. Non-element
 * targets degrade to false.
 */
export function isEditableTarget(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) return false
  if (target.isContentEditable) return true
  const tag = target.tagName
  return tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT'
}

/**
 * Pure viewport-shrink heuristic: true when the visual viewport is at least
 * `threshold` px shorter than the layout viewport. Non-finite inputs degrade
 * to false (no keyboard is provable from missing data).
 */
export function isKeyboardShrink(
  layoutViewportH: number,
  visualViewportH: number,
  threshold: number = KEYBOARD_SHRINK_THRESHOLD_PX
): boolean {
  if (!Number.isFinite(layoutViewportH) || !Number.isFinite(visualViewportH)) return false
  return layoutViewportH - visualViewportH >= threshold
}

// Detach fn of the active tracking session; null while not tracking (guards
// double init from HMR / repeated App mounts).
let detach: (() => void) | null = null

// Editable-focus half of the two-signal conjunction, kept outside the ref:
// the published ref only flips when BOTH signals agree.
let editableFocused = false

function recompute(): void {
  const vv = typeof window !== 'undefined' ? window.visualViewport : null
  keyboardVisible.value =
    editableFocused && !!vv && isKeyboardShrink(window.innerHeight, vv.height)
}

/**
 * Start tracking (called once from App.vue onMounted; no-op when already
 * active or when window is unavailable). Keyboard visibility rides two
 * signals — see the module doc. focusout clears the editable half
 * asynchronously: the browser fires focusout BEFORE the next focusin, so a
 * synchronous recompute would flap the class while the caret hops between
 * the chat textarea and a composer button; the deferred check re-reads the
 * settled focus state instead.
 */
export function initKeyboardTracking(): void {
  if (detach || typeof window === 'undefined') return
  editableFocused = false

  const onFocusIn = (e: FocusEvent) => {
    editableFocused = isEditableTarget(e.target)
    recompute()
  }
  const onFocusOut = (e: FocusEvent) => {
    if (!isEditableTarget(e.target)) return
    editableFocused = false
    window.setTimeout(recompute, 0)
  }
  const onVVResize = () => recompute()

  window.addEventListener('focusin', onFocusIn, true)
  window.addEventListener('focusout', onFocusOut, true)
  window.visualViewport?.addEventListener('resize', onVVResize)

  detach = () => {
    window.removeEventListener('focusin', onFocusIn, true)
    window.removeEventListener('focusout', onFocusOut, true)
    window.visualViewport?.removeEventListener('resize', onVVResize)
    detach = null
    editableFocused = false
    keyboardVisible.value = false
  }
}

/** Stop tracking and reset the published state (App unmount / tests). */
export function stopKeyboardTracking(): void {
  detach?.()
}
