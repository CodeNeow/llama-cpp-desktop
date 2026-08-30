/**
 * Layout constants and pure helpers shared across components.
 *
 * Three-tier responsive model (source of truth for breakpoint literals):
 *   - mobile:  width <= MOBILE_MAX (767px)
 *   - tablet:  MOBILE_MAX < width <= TABLET_MAX (768..1099px)
 *   - desktop: width > TABLET_MAX (1100px and up)
 *
 * CSS custom properties do not work inside `@media` conditions, so every
 * media query in `src/styles/global.css` and in component <style> blocks
 * must repeat these literals (e.g. `@media (max-width: 767px)`). The
 * constants below are therefore the source of truth; global.css documents
 * them in a comment block near the top of the file. When changing a
 * breakpoint here, update every matching `@media` literal in the styles.
 */

/** Viewport widths at or below this value are treated as mobile. */
export const MOBILE_MAX = 767

/** Upper bound of the tablet tier (768..1099px); desktop starts above it. */
export const TABLET_MAX = 1099

/**
 * Viewport width (px) at or below which the sidebar auto-collapses to the
 * icon rail. Matches the existing `@media (max-width: 1099px)` breakpoints
 * used by the fixed-viewport pages (e.g. Api.vue's monitor grid). Kept as a
 * named alias of TABLET_MAX so the sidebar behavior and the tablet tier stay
 * linked to a single number.
 */
export const SIDEBAR_AUTO_COLLAPSE_WIDTH = TABLET_MAX

/** Whether a window of the given width should start with the sidebar collapsed. */
export function shouldCollapseSidebar(width: number): boolean {
  return width <= SIDEBAR_AUTO_COLLAPSE_WIDTH
}
