/**
 * Pure scroll-geometry helpers, unit-testable without a DOM.
 */

/**
 * Whether a scrollable element is at (near) its bottom, within `threshold` px.
 *
 * Used for stick-to-bottom behavior: while the user is at the bottom, incoming
 * content keeps the view pinned to the newest text; once they scroll up past
 * the threshold, their position is preserved until they scroll back near the
 * bottom (re-attach). A box whose content is fully visible (scrollHeight no
 * larger than clientHeight) counts as at-bottom.
 */
export function isNearBottom(scrollTop: number, scrollHeight: number, clientHeight: number, threshold = 24): boolean {
  return scrollHeight - scrollTop - clientHeight <= threshold
}
