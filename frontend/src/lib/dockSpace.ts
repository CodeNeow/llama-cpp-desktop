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
 *
 * The same composable also publishes the collapsed pill's measured WIDTH as
 * `dockWidth` (global CSS variable `--dock-width` via App.vue). The pill's
 * width is content-driven (one segment per active counter), so a fixed reserve
 * lane sized for a single-segment pill (e.g. Chat.vue's old hardcoded 64px
 * phone right padding) overflows when both download and model counters show.
 * Pages that keep controls clear of the pill consume `--dock-width` instead of
 * guessing the width. Like the height, it is 0 while the dock is hidden.
 *
 * Since the pill became a draggable capsule (lib/dockPosition.ts) it can hug
 * EITHER window edge, so the module also publishes `dockSide` ('left' |
 * 'right', updated by TaskDock whenever the snapped position changes). The
 * chat page mirrors its reserve lane to the pill's side via this ref, and
 * App.vue binds it globally as the `--dock-side` CSS variable. Default
 * 'right' = the legacy fixed spot.
 *
 * Because the capsule can also park AWAY from the composer (mid-screen, top),
 * the module publishes `dockLane` ('left' | 'right' | 'none'): the side lane
 * exists only while the capsule rides in the composer band at the bottom of
 * the window, and 'none' retracts it (Chat.vue back to its base gutters).
 */
import { ref, watchEffect, onScopeDispose, type Ref } from 'vue'
import type { DockLane, DockSide } from './dockPosition'

// Bottom offset of the fixed dock pill: the vertical centering offset on the
// chat page's input row band. DESKTOP-ONLY value: input-area bottom padding
// 24 + (row height 42 - pill height 32) / 2 = 29, which places the pill's
// center on the send button's center. Keep in sync with `bottom: 29px` in
// TaskDock.vue's `.task-dock` style (change both or the reserve drifts). With
// this offset the scrollable-page reserve becomes root height (32) + 29 + 8 =
// 69px. Phones (<=767px) have a different composer (row 44, input-area bottom
// padding 10), so TaskDock.vue's media query overrides `bottom` to
// calc(10px + var(--mobile-nav-height)); this constant stays desktop-tuned,
// which only slightly over-reserves on phone scrollable pages (harmless extra
// bottom padding).
const DOCK_BOTTOM_OFFSET = 29

// Small breathing gap between the reserved band and the content above it.
const CONTENT_GAP = 8

// Reserved bottom space in px, 0 while the dock is hidden. App.vue binds this
// to the global CSS variable `--dock-reserve`.
export const dockReserve = ref(0)

// Measured collapsed-pill width in px, 0 while the dock is hidden. App.vue
// binds this to the global CSS variable `--dock-width`.
export const dockWidth = ref(0)

// Which window edge the draggable capsule currently hugs ('left' | 'right').
// TaskDock.vue writes it whenever the snapped position changes (restore, drag
// release, resize re-fit) and resets it to 'right' on unmount, mirroring the
// reserve-reset pattern below.
export const dockSide = ref<DockSide>('right')

// Whether the chat page currently needs a reserve lane at all: 'left'/'right'
// only while the capsule (vertically) rides in the composer band at the bottom
// of the window, 'none' while it is parked elsewhere or hidden. TaskDock.vue
// recomputes it via dockPosition.laneFor at the same points where it republish
// the side (restore, drag release, resize re-fit) and resets it to 'none' when
// the dock hides or unmounts. Consumers (Chat.vue) import this ref directly —
// no global CSS variable needed, the lane only feeds one page's paddings.
export const dockLane = ref<DockLane>('none')

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
 * Pure helper: the pill width to publish for the given visibility and
 * measured width (no offsets added — consumers compose their own lane,
 * e.g. `calc(16px + var(--dock-width, 0px) + 8px)`). Returns 0 when hidden
 * or the width is invalid (NaN / non-finite / <= 0).
 */
export function dockWidthPx(visible: boolean, width: number): number {
  if (!visible) return 0
  if (!Number.isFinite(width) || width <= 0) return 0
  return width
}

/**
 * Composable: track the dock element and keep `dockReserve` (height lane) and
 * `dockWidth` (collapsed-pill width) in sync with its measured box.
 *
 * Why ResizeObserver: the dock's box is dynamic — task and model rows come
 * and go, error lines appear, the card collapses/expands, and the pill's
 * width follows its active counter segments — so a single static read is not
 * enough. The observer callback re-measures via the current `el.value` (the
 * element may have changed between `observe()` and the callback firing) and
 * re-validates both values through the pure helpers.
 *
 * The observed element is TaskDock's fixed positioning root whose ONLY
 * in-flow content is the collapsed pill (the expanded card is absolutely
 * positioned), so its box IS the pill's box. While the popover is open the
 * pill is visually hidden with `visibility` (still occupying layout), which
 * keeps the measured width stable — exactly the collapsed-form width pages
 * need to reserve against.
 *
 * `typeof ResizeObserver === 'undefined'` guard: jsdom and older environments
 * lack the API; there we fall back to the immediate measurement only.
 *
 * `onScopeDispose` disconnects the observer and zeroes both refs so an
 * unmounted (or hidden) dock never leaves stale reserved space behind.
 */
export function useDockReserve(el: Ref<HTMLElement | null>, visible: Ref<boolean>): void {
  let observer: ResizeObserver | null = null
  let tracked: HTMLElement | null = null

  function measure() {
    const node = el.value
    dockReserve.value = node ? dockReservePx(visible.value, node.offsetHeight) : 0
    dockWidth.value = node ? dockWidthPx(visible.value, node.offsetWidth) : 0
  }

  watchEffect(() => {
    const node = el.value
    if (!node || !visible.value) {
      observer?.disconnect()
      observer = null
      tracked = null
      dockReserve.value = 0
      dockWidth.value = 0
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
    dockWidth.value = 0
  })
}
