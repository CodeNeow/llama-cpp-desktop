/**
 * Layout constants and pure helpers shared across components.
 *
 * Three-tier responsive model (source of truth for breakpoint literals):
 *   - mobile:  width <= MOBILE_MAX (767px)
 *   - tablet:  MOBILE_MAX < width <= TABLET_MAX (768..1099px)
 *   - desktop: width > TABLET_MAX (1100px and up)
 *
 * Android landscape-tablet extension (see TABLET_LANDSCAPE_MAX and the
 * orientation helpers below): Android tablets in landscape orientation
 * (width > TABLET_MAX and <= TABLET_LANDSCAPE_MAX with width > height)
 * classify back into the tablet tier instead of desktop. The helpers here
 * are pure viewport math only — the OS gate that restricts the extension to
 * Android lives in lib/platform.ts, so desktop OSes are never re-classified
 * by it.
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
 * Upper bound of the Android landscape-tablet band (TABLET_MAX+1..1360px).
 * Caps the landscape re-classification for Android tablets: the design
 * draft targets a 1280x800 landscape panel and 1360 leaves headroom above
 * it. Widths beyond this value stay in the desktop tier even in landscape.
 * Desktop OSes are never re-classified by it — the OS gate lives in
 * lib/platform.ts, so a desktop window of the same size stays desktop.
 */
export const TABLET_LANDSCAPE_MAX = 1360

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

/**
 * Viewport orientation from width/height: 'landscape' iff width is strictly
 * greater than height. A perfect square (width === height) ties to
 * 'portrait' so the conservative default applies — the landscape-tablet
 * classification never claims an ambiguous viewport.
 */
export function viewportOrientation(width: number, height: number): 'portrait' | 'landscape' {
  return width > height ? 'landscape' : 'portrait'
}

/**
 * Pure viewport math for the Android landscape-tablet band: width inside
 * (TABLET_MAX, TABLET_LANDSCAPE_MAX] AND landscape orientation (width >
 * height). Deliberately WITHOUT any OS gate — callers (lib/platform.ts)
 * combine this with the Android-only check, so desktop OSes are never
 * re-classified by this band.
 */
export function isTabletLandscapeViewport(width: number, height: number): boolean {
  return (
    width > TABLET_MAX &&
    width <= TABLET_LANDSCAPE_MAX &&
    viewportOrientation(width, height) === 'landscape'
  )
}
