/**
 * Pure geometry + storage logic for the draggable TaskDock capsule ("smart
 * pill"): free pointer dragging inside the window, AssistiveTouch-style edge
 * snapping on release (horizontal: nearest left/right edge; vertical: kept
 * where dropped but clamped into a safe band), and viewport-independent
 * position memory in localStorage.
 *
 * Everything here is pure or localStorage-only — no Vue reactivity, no DOM —
 * so TaskDock.vue stays a thin event/transform wiring layer and the math is
 * unit-testable without mocks. The published reactive side of the position
 * lives in lib/dockSpace.ts (`dockSide`).
 *
 * Position model: the horizontal axis is always snapped to an edge, so the
 * stored state is `{ side, yNorm }`:
 *   - `side`   which edge the pill hugs ('left' | 'right'; 16px gap on
 *              desktop, flush with the screen edge on the phone tier);
 *   - `yNorm`  the pill's top edge as a 0..1 fraction WITHIN the vertical
 *              safe band (top gap below the title bar / safe area, bottom gap
 *              above the mobile nav band or the desktop floor). Band-relative
 *              normalization (not raw viewport fraction) makes a stored
 *              position survive resizes and phone <-> desktop tier switches
 *              without ever landing out of bounds.
 * Restoring maps yNorm back through the CURRENT viewport's band, then
 * re-snaps/re-clamps, so a position saved on a tall window degrades gracefully
 * on a short one.
 */

// Horizontal gap between the pill and the window edge it hugs. Desktop value;
// mirrors the `right: 16px` anchor in TaskDock.vue's `.task-dock` and
// Chat.vue's desktop lane arithmetic (`16px + var(--dock-width) + 8px`). The
// phone tier hugs the screen edge FLUSH (see DOCK_EDGE_GAP_MOBILE) — both the
// CSS anchor and the drag math read the gap through DockLayoutMetrics.edgeGap.
export const DOCK_EDGE_GAP = 16

// Phone-tier edge gap: the capsule sticks to the screen sides themselves (0px
// inset), mirroring TaskDock.vue's phone `right: 0` anchor override.
export const DOCK_EDGE_GAP_MOBILE = 0

// Vertical breathing gap below the title bar / safe area that the pill's top
// edge must respect.
export const DOCK_TOP_GAP = 8

// Bottom safe gap (pill bottom edge -> viewport floor): plain 16px on the
// desktop tier; on the phone tier the bottom nav band plus 10px.
export const DOCK_BOTTOM_GAP_DESKTOP = 16
export const DOCK_BOTTOM_GAP_MOBILE = 10

// Bottom offset of TaskDock's CSS anchor (`.task-dock { bottom: ... }`),
// desktop tier: input-area padding 24 + (row 42 - pill 32) / 2 = 29 — the same
// constant documented in lib/dockSpace.ts (DOCK_BOTTOM_OFFSET). Phone tier:
// input-area padding-bottom 10 + (row 44 - pill 44) / 2 = 10. Keep these in
// sync with TaskDock.vue's `.task-dock` rules (change both or the transform
// drifts relative to the CSS anchor).
export const DOCK_ANCHOR_BOTTOM_DESKTOP = 29
export const DOCK_ANCHOR_BOTTOM_MOBILE = 10

// Pointer travel (px) below which a press is still a click (expand/collapse);
// beyond it the gesture is a drag and the release must not expand the card.
export const DOCK_DRAG_THRESHOLD = 6

// Vertical extent of the Chat composer's interaction band, measured UP from
// the viewport floor: a capsule whose bottom edge sits inside this band
// vertically overlaps the chat page's input row, so Chat.vue keeps reserving
// its side lane (see laneFor). Sized generous on purpose — an undersized band
// would leave the pill covering the send button with no lane reserved.
//   Desktop (130): input-area bottom padding 24 + input row 42 = 66px real
//     composer band, + ~42px for the optional auto-start notice pill that
//     rides above the row + ~22px margin ≈ 130.
//   Phone (170): historical value from the retired phone lane (bottom nav
//     band 82 + composer 62 + margin). The phone chat page no longer
//     reserves a lane — TaskDock keeps the capsule clear of the composer by
//     clamping against the MEASURED composer top instead — so the constant
//     has no production consumer left and is kept only as the documented
//     reference for that band's real-world phone size.
export const CHAT_COMPOSER_BAND_DESKTOP = 130
export const CHAT_COMPOSER_BAND_MOBILE = 170

// localStorage key: same `llama-desktop-*` style as the theme/sidebar caches
// in store.ts (pure UI preference, deliberately NOT part of the backend
// config stream).
export const DOCK_POSITION_KEY = 'llama-desktop-dock-position'

/** Which window edge the capsule hugs after snapping. */
export type DockSide = 'left' | 'right'

/**
 * Which side lane the chat page reserves for the capsule: the capsule's side
 * while it rides in the composer band, else 'none' (capsule parked elsewhere
 * or hidden — the composer reclaims the full width).
 */
export type DockLane = 'left' | 'right' | 'none'

/** Persisted position: snapped side + band-relative vertical fraction. */
export interface DockStoredPosition {
  side: DockSide
  yNorm: number
}

/**
 * Everything the math needs about the current viewport and pill. Computed by
 * TaskDock.vue per application (window size, measured pill box via the same
 * offsetWidth/offsetHeight the dock-space observer uses, chrome bands measured
 * from the rendered title bar / mobile nav elements).
 */
export interface DockLayoutMetrics {
  viewportW: number
  viewportH: number
  /** Pill width in px (TaskDock root offsetWidth; transform never changes it). */
  pillW: number
  /** Pill height in px (TaskDock root offsetHeight). */
  pillH: number
  /** Pill top edge lower bound: title bar height + safe area + top gap. */
  minTop: number
  /** Pill bottom edge -> viewport floor gap (desktop 16 / phone nav+10). */
  clampBottomGap: number
  /** CSS anchor bottom offset (desktop 29 / phone 10+nav) of `.task-dock`. */
  anchorBottomOffset: number
  /**
   * Horizontal gap between the pill and the window edge it hugs (desktop 16 /
   * phone 0 — the capsule sticks to the screen sides on phones). Consumed by
   * sideLeftX for BOTH the snap math and the anchorTopLeft reference, so the
   * CSS anchor and the dragged/restored positions can never drift apart.
   */
  edgeGap: number
}

/** Plain x/y point (px, viewport coordinates). */
export interface DockPoint {
  x: number
  y: number
}

function isFiniteNumber(v: unknown): v is number {
  return typeof v === 'number' && Number.isFinite(v)
}

/**
 * Click-vs-drag discrimination: true when the pointer traveled strictly more
 * than the drag threshold (measured as straight-line distance from the press
 * point). A distance of exactly the threshold still counts as a click.
 */
export function isBeyondDragThreshold(dx: number, dy: number): boolean {
  if (!isFiniteNumber(dx) || !isFiniteNumber(dy)) return false
  return Math.hypot(dx, dy) > DOCK_DRAG_THRESHOLD
}

/**
 * Nearest horizontal edge for a release point: the pill hugging side is
 * chosen from the pill CENTER's x. Ties (center exactly at mid-width) go to
 * 'right', matching the legacy default position.
 */
export function nearestSide(centerX: number, viewportW: number): DockSide {
  if (!isFiniteNumber(centerX) || !isFiniteNumber(viewportW) || viewportW <= 0) return 'right'
  return centerX * 2 < viewportW ? 'left' : 'right'
}

/**
 * Lane decision for the chat page: the capsule's side only while its bottom
 * edge (viewport px) reaches into the composer band at the bottom of the
 * window (`capsuleBottom >= viewportH - bandHeight`, inclusive), else 'none'
 * — a capsule parked mid-screen / up top must not keep the composer narrowed.
 * Any invalid input (unknown side, non-finite numbers, non-positive viewport
 * or band) degrades to 'none': no lane is the safe default (worst case is a
 * cosmetic overlap, never a permanently reserved phantom lane).
 */
export function laneFor(
  side: DockSide,
  capsuleBottom: number,
  viewportH: number,
  bandHeight: number
): DockLane {
  if (side !== 'left' && side !== 'right') return 'none'
  if (!isFiniteNumber(capsuleBottom) || !isFiniteNumber(viewportH) || viewportH <= 0) return 'none'
  if (!isFiniteNumber(bandHeight) || bandHeight <= 0) return 'none'
  return capsuleBottom >= viewportH - bandHeight ? side : 'none'
}

/**
 * Pill left edge px for a given side: `edgeGap` from the left edge, or
 * mirrored from the right edge. Degenerate inputs degrade to the left-edge
 * placement at the same gap (the safest visible spot) — desktop callers omit
 * the argument and keep the historical 16px gap.
 */
export function sideLeftX(
  side: DockSide,
  viewportW: number,
  pillW: number,
  edgeGap: number = DOCK_EDGE_GAP
): number {
  if (side === 'left') return edgeGap
  if (!isFiniteNumber(viewportW) || !isFiniteNumber(pillW) || pillW <= 0 || viewportW <= 0) {
    return edgeGap
  }
  return Math.max(edgeGap, viewportW - edgeGap - pillW)
}

/**
 * Top edge of the pill's CSS anchor with zero transform applied — the
 * reference every drag translate is measured against (mirrors
 * `.task-dock { right: <edgeGap>px; bottom: <anchor offset> }`).
 */
export function anchorTopLeft(layout: DockLayoutMetrics): DockPoint {
  return {
    x: sideLeftX('right', layout.viewportW, layout.pillW, layout.edgeGap),
    y: layout.viewportH - layout.anchorBottomOffset - layout.pillH,
  }
}

/**
 * Clamp a pill top edge into the vertical safe band
 * `[minTop, viewportH - clampBottomGap - pillH]`. When the band is degenerate
 * (viewport shorter than chrome + pill — never expected at real sizes) the
 * top gap wins so the pill never hides under the title bar.
 */
export function clampTopPx(top: number, layout: DockLayoutMetrics): number {
  const { minTop, viewportH, clampBottomGap, pillH } = layout
  if (!isFiniteNumber(top) || !isFiniteNumber(minTop)) return 0
  const maxTop = viewportH - clampBottomGap - pillH
  if (!isFiniteNumber(maxTop) || maxTop < minTop) return minTop
  return Math.min(Math.max(top, minTop), maxTop)
}

/**
 * Map a clamped pill top edge to the stored 0..1 fraction within the safe
 * band. Degenerate bands normalize to 0 (top of whatever band remains).
 */
export function topToNorm(top: number, layout: DockLayoutMetrics): number {
  const { minTop } = layout
  const maxTop = layout.viewportH - layout.clampBottomGap - layout.pillH
  const band = maxTop - minTop
  if (!isFiniteNumber(band) || band <= 0) return 0
  const norm = (top - minTop) / band
  return Math.min(Math.max(norm, 0), 1)
}

/**
 * Inverse of `topToNorm`: map a stored fraction back to a clamped pill top
 * edge for the CURRENT band, so resizes and tier switches re-fit the position
 * proportionally instead of overflowing.
 */
export function normToTop(yNorm: number, layout: DockLayoutMetrics): number {
  const { minTop } = layout
  const maxTop = layout.viewportH - layout.clampBottomGap - layout.pillH
  const band = maxTop - minTop
  if (!isFiniteNumber(yNorm) || !isFiniteNumber(band) || band <= 0) return clampTopPx(minTop, layout)
  return clampTopPx(minTop + Math.min(Math.max(yNorm, 0), 1) * band, layout)
}

/**
 * Resolve a stored position into concrete pill coordinates for the current
 * layout: horizontally snapped to the stored side, vertically re-fitted via
 * the band fraction.
 */
export function resolvePosition(
  stored: DockStoredPosition,
  layout: DockLayoutMetrics
): { left: number; top: number; side: DockSide } {
  const side: DockSide = stored.side === 'left' ? 'left' : 'right'
  return {
    left: sideLeftX(side, layout.viewportW, layout.pillW, layout.edgeGap),
    top: normToTop(stored.yNorm, layout),
    side,
  }
}

/**
 * Transform (px) to apply on the fixed TaskDock root so the pill lands on the
 * stored position. `null` (nothing stored) yields { 0, 0 } — the pill stays
 * exactly at its legacy CSS anchor spot (bottom-right). The root's CSS anchor
 * itself never changes, so a hidden/never-dragged dock is pixel-identical to
 * the pre-drag behavior.
 */
export function translateForPosition(
  stored: DockStoredPosition | null,
  layout: DockLayoutMetrics
): DockPoint {
  if (!stored) return { x: 0, y: 0 }
  const target = resolvePosition(stored, layout)
  const anchor = anchorTopLeft(layout)
  return { x: target.left - anchor.x, y: target.top - anchor.y }
}

/**
 * Read the persisted position. Corrupted / forged data (broken JSON, unknown
 * side, non-finite fraction) degrades to `null` (legacy default spot); an
 * out-of-range-but-finite fraction is clamped back into 0..1. All localStorage
 * access is guarded — a disabled/unavailable store behaves like "no memory".
 */
export function loadStoredPosition(): DockStoredPosition | null {
  try {
    const raw = localStorage.getItem(DOCK_POSITION_KEY)
    if (!raw) return null
    const parsed: unknown = JSON.parse(raw)
    if (typeof parsed !== 'object' || parsed === null) return null
    const { side, yNorm } = parsed as Record<string, unknown>
    if (side !== 'left' && side !== 'right') return null
    if (!isFiniteNumber(yNorm)) return null
    return { side, yNorm: Math.min(Math.max(yNorm, 0), 1) }
  } catch {
    return null
  }
}

/** Persist the position; storage failures are swallowed (pure preference). */
export function saveStoredPosition(pos: DockStoredPosition): void {
  try {
    localStorage.setItem(DOCK_POSITION_KEY, JSON.stringify(pos))
  } catch {
    // Quota / privacy mode: the position simply is not remembered.
  }
}
