/**
 * Shared dock-space reservation state: the floating TaskDock pill overlays the
 * bottom-right corner of the window (position: fixed), so page layouts need to
 * reserve matching space to keep controls there clickable:
 * - scrollable pages (App.vue `.content-area`) get bottom padding,
 * - the fixed-viewport chat page (Chat.vue) reserves no bottom band: the pill
 *   sits in the input row's right gap, vertically centered on the send button
 *   (see DOCK_BOTTOM_OFFSET below).
 *
 * TaskDock measures its own height via this composable and publishes it in the
 * module-level `dockReserve` ref; App.vue binds the value to the global CSS
 * variable `--dock-reserve` which scrollable layouts consume. When the dock is
 * hidden the reserve is 0 and the layout matches the pre-dock state exactly.
 */
import { ref, watchEffect, onScopeDispose, type Ref } from 'vue'

// Bottom offset of the fixed dock pill: the vertical centering offset on the
// chat page's input row band, input-area bottom padding 24 + (row height 42 -
// pill height 32) / 2 = 29, which places the pill's center on the send
// button's center. Keep in sync with `bottom: 29px` in TaskDock.vue's
// `.task-dock` style (change both or the reserve drifts). With this offset the
// scrollable-page reserve becomes root height (32) + 29 + 8 = 69px.
const DOCK_BOTTOM_OFFSET = 29

// Small breathing gap between the reserved band and the content above it.
const CONTENT_GAP = 8

// Reserved bottom space in px, 0 while the dock is hidden. App.vue binds this
// to the global CSS variable `--dock-reserve`.
export const dockReserve = ref(0)

/**
 * Pure helper: the px value to reserve for a dock with the given visibility
 * and measured height. Returns 0 when hidden or the height is invalid
 * (NaN / non-finite / <= 0, e.g. a not-yet-laid-out element).
 */
export function dockReservePx(visible: boolean, height: number): number {
  if (!visible) return 0
  if (!Number.isFinite(height) || height <= 0) return 0
  return height + DOCK_BOTTOM_OFFSET + CONTENT_GAP
}

/**
 * Composable: track the dock element and keep `dockReserve` in sync with its
 * measured height.
 *
 * Why ResizeObserver: the dock's height is dynamic — task and model rows come
 * and go, error lines appear, and the card collapses/expands — so a single
 * static read is not enough. The observer callback re-measures via the
 * current `el.value` (the element may have changed between `observe()` and the
 * callback firing) and re-validates it through `dockReservePx`.
 *
 * `typeof ResizeObserver === 'undefined'` guard: jsdom and older environments
 * lack the API; there we fall back to the immediate measurement only.
 *
 * `onScopeDispose` disconnects the observer and zeroes the reserve so an
 * unmounted (or hidden) dock never leaves stale reserved space behind.
 */
export function useDockReserve(el: Ref<HTMLElement | null>, visible: Ref<boolean>): void {
  let observer: ResizeObserver | null = null
  let tracked: HTMLElement | null = null

  function measure() {
    const node = el.value
    dockReserve.value = node ? dockReservePx(visible.value, node.offsetHeight) : 0
  }

  watchEffect(() => {
    const node = el.value
    if (!node || !visible.value) {
      observer?.disconnect()
      observer = null
      tracked = null
      dockReserve.value = 0
      return
    }
    if (tracked !== node) {
      observer?.disconnect()
      tracked = node
      if (typeof ResizeObserver !== 'undefined') {
        observer = new ResizeObserver(() => measure())
        observer.observe(node)
      }
    }
    // Immediate measurement keeps the reserve correct before the first
    // ResizeObserver callback fires (ResizeObserver reports initial size
    // asynchronously; a v-if remount would otherwise flash the old reserve).
    measure()
  })

  onScopeDispose(() => {
    observer?.disconnect()
    observer = null
    tracked = null
    dockReserve.value = 0
  })
}
