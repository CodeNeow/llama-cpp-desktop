/**
 * Shared navigation model: the five primary destinations consumed by both
 * navigation surfaces — the desktop/tablet left sidebar (Sidebar.vue) and the
 * mobile bottom tab bar (MobileNav.vue in App.vue's mobile shell).
 *
 * Pure data + pure matcher: no Vue reactivity, no router instance, no DOM —
 * fully unit-testable without mocks. Labels are i18n keys (resolved by the
 * consumer via t(item.labelKey)) so locale switches re-render both surfaces.
 *
 * IA note (mobile redesign, v1 mobile design draft frame ⑤): the docs
 * destination left the navigation bars and moved into an entry card at the
 * top of the Settings page; the /docs route itself is unchanged. Its book
 * icon is kept below (DOCS_ICON) as the single source for that entry card.
 */

/** One primary navigation destination. */
export interface NavItem {
  /** Router target path (top-level route). */
  path: string
  /** i18n dictionary key for the display label (see lib/i18n.ts `nav.*`). */
  labelKey: string
  /** Inline stroke-SVG icon string, rendered via v-html (same style as Sidebar). */
  icon: string
  /**
   * Optional phone-tier icon (docs/branding/android-design.html frame ① glass
   * nav): same 24×24 stroke-SVG model but with the draft's own path data and
   * stroke-width 2.2. Sidebar.vue keeps consuming `icon`, so the desktop glyph
   * set is untouched; MobileNav prefers this one when present and falls back
   * to `icon` (API / settings already match the draft).
   */
  phoneIcon?: string
}

/** Sidebar order: Home → Chat → Models → API → Settings (Docs lives in the Settings entry card). */
export const NAV_ITEMS: NavItem[] = [
  {
    path: '/',
    labelKey: 'nav.home',
    icon: `<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M3 9l9-7 9 7v11a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"/><polyline points="9 22 9 12 15 12 15 22"/></svg>`,
    phoneIcon: `<svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"><path d="M3 10.5L12 3l9 7.5V21H3z"/></svg>`
  },
  {
    path: '/chat',
    labelKey: 'nav.chat',
    icon: `<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"/></svg>`,
    phoneIcon: `<svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 12a8 8 0 01-8 8H4l2.5-3A8 8 0 1121 12z"/></svg>`
  },
  {
    path: '/models',
    labelKey: 'nav.models',
    icon: `<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 16V8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16z"/><polyline points="3.27 6.96 12 12.01 20.73 6.96"/><line x1="12" y1="22.08" x2="12" y2="12"/></svg>`,
    phoneIcon: `<svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 2l8 4.5v9L12 20l-8-4.5v-9L12 2z"/></svg>`
  },
  {
    path: '/api',
    labelKey: 'nav.api',
    icon: `<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="4 17 10 11 4 5"/><line x1="12" y1="19" x2="20" y2="19"/></svg>`
  },
  {
    path: '/settings',
    labelKey: 'nav.settings',
    icon: `<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-2 2 2 2 0 0 1-2-2v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1-2-2 2 2 0 0 1 2-2h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 2-2 2 2 0 0 1 2 2v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 2 2 2 2 0 0 1-2 2h-.09a1.65 1.65 0 0 0-1.51 1z"/></svg>`
  }
]

/**
 * Book icon of the former Docs nav entry (same stroke style as the other nav
 * icons): kept as the single source for the Settings-page "Help & Tutorial"
 * entry card that replaced the sixth navigation slot. Rendered via v-html by
 * the consumer, like NAV_ITEMS icons.
 */
export const DOCS_ICON = `<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M4 19.5A2.5 2.5 0 0 1 6.5 17H20"/><path d="M6.5 2H20v20H6.5A2.5 2.5 0 0 1 4 19.5v-15A2.5 2.5 0 0 1 6.5 2z"/></svg>`

/**
 * Whether a nav entry is active for the current route path. Ported verbatim
 * from Sidebar.vue's isActive:
 *   - the home entry ('/') also covers the System Environment tab children
 *     ('/system', '/runtime') — '/system' is a distinct prefix from
 *     '/settings' (settings must not light up on the system info tab);
 *   - every other entry matches by prefix (covers child routes like
 *     /models/download, /models/local, /models/model/:id).
 */
export function isActiveNav(itemPath: string, routePath: string): boolean {
  if (itemPath === '/') {
    return routePath === '/' || routePath.startsWith('/system') || routePath.startsWith('/runtime')
  }
  return routePath.startsWith(itemPath)
}
