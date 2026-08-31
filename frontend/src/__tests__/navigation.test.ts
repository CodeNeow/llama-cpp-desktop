import { describe, it, expect } from 'vitest'
import { NAV_ITEMS, DOCS_ICON, isActiveNav } from '../lib/navigation'

describe('lib/navigation', () => {
  it('exposes the five primary destinations in sidebar order (docs moved into the Settings entry card)', () => {
    expect(NAV_ITEMS.map((i) => i.path)).toEqual([
      '/', '/chat', '/models', '/api', '/settings'
    ])
    expect(NAV_ITEMS.map((i) => i.labelKey)).toEqual([
      'nav.home', 'nav.chat', 'nav.models', 'nav.api', 'nav.settings'
    ])
    // IA regression pin (mobile redesign): /docs is no longer a navigation
    // entry — it is reached from the Settings-page entry card instead
    expect(NAV_ITEMS.map((i) => i.path)).not.toContain('/docs')
  })

  it('keeps the docs book icon exported as the single source for the Settings entry card', () => {
    expect(DOCS_ICON).toContain('<svg')
    expect(DOCS_ICON).toContain('stroke="currentColor"')
  })

  it('every item carries an inline svg icon matching the sidebar stroke style', () => {
    for (const item of NAV_ITEMS) {
      expect(item.icon).toContain('<svg')
      expect(item.icon).toContain('stroke="currentColor"')
    }
  })

  it('isActiveNav: home entry covers the System Environment tab children', () => {
    expect(isActiveNav('/', '/')).toBe(true)
    expect(isActiveNav('/', '/system')).toBe(true)
    expect(isActiveNav('/', '/runtime')).toBe(true)
    // '/settings' is a distinct prefix from '/system': settings must not light up
    expect(isActiveNav('/', '/settings')).toBe(false)
  })

  it('isActiveNav: prefix matching covers child routes of each entry', () => {
    const cases: Array<[string, string, boolean]> = [
      // chat: exact match only
      ['/chat', '/chat', true],
      ['/chat', '/system', false],
      // models: covers the tab shells and the detail / settings children
      ['/models', '/models/download', true],
      ['/models', '/models/local', true],
      ['/models', '/models/model/x', true],
      ['/models', '/models/settings/y', true],
      ['/models', '/downloads', false],
      // api / settings / docs (the /docs rows pin the pure matcher contract:
      // the route left the nav bars but isActiveNav stays a generic prefix
      // matcher and must keep working for any path)
      ['/api', '/api', true],
      ['/api', '/monitor', false],
      ['/settings', '/settings', true],
      ['/settings', '/system', false],
      ['/docs', '/docs', true],
      // unknown routes belong to no entry
      ['/docs', '/unknown', false],
      ['/api', '/unknown', false]
    ]
    for (const [itemPath, routePath, expected] of cases) {
      expect(isActiveNav(itemPath, routePath)).toBe(expected)
    }
  })

  it('isActiveNav: prefix is direction-safe (route shorter than item never matches)', () => {
    expect(isActiveNav('/models', '/model')).toBe(false)
    expect(isActiveNav('/api', '/')).toBe(false)
  })
})
