/** Layout constants and pure helpers shared across components. */

/**
 * Viewport width (px) at or below which the sidebar auto-collapses to the
 * icon rail. Matches the existing `@media (max-width: 1099px)` breakpoints
 * used by the fixed-viewport pages (e.g. Api.vue's monitor grid).
 */
export const SIDEBAR_AUTO_COLLAPSE_WIDTH = 1099

/** Whether a window of the given width should start with the sidebar collapsed. */
export function shouldCollapseSidebar(width: number): boolean {
  return width <= SIDEBAR_AUTO_COLLAPSE_WIDTH
}
