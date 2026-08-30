import { describe, it, expect } from 'vitest'
import router from '../router'

// Fixed-viewport pages (Home / Chat / Api) manage their own internal scroll
// bands and reserve TaskDock space themselves; App.vue relies on route meta
// `fixed` to switch .content-area into that mode (no bottom reserve, no own
// scrolling). These tests pin that contract: exactly the three fixed pages
// are flagged, every scrollable page is not. Home is the tabbed System
// Environment shell: its /system and /runtime tab children carry no meta of
// their own (children do not inherit meta in getRoutes()); at runtime the
// parent's fixed meta still applies to matched child routes.
describe('router fixed-page meta', () => {
  const fixedPaths = ['/', '/chat', '/api']

  // Scrollable pages keep normal document flow: .content-area scrolls and
  // reserves the TaskDock band for them (subpages and the tab children of the
  // /models and / shells included)
  const scrollingPaths = [
    '/models',
    '/models/download',
    '/models/local',
    '/models/model/:modelId',
    '/models/settings/:modelName',
    '/system',
    '/runtime',
    '/settings',
    '/docs',
  ]

  it('marks exactly the three fixed-viewport pages', () => {
    const flagged = router
      .getRoutes()
      .filter((r) => r.meta.fixed === true)
      .map((r) => r.path)
      .sort()
    expect(flagged).toEqual([...fixedPaths].sort())
  })

  it('leaves scrollable pages unflagged', () => {
    for (const path of scrollingPaths) {
      const route = router.getRoutes().find((r) => r.path === path)
      expect(route?.meta.fixed).toBeUndefined()
    }
  })

  it('registers the in-app docs route', () => {
    const route = router.getRoutes().find((r) => r.path === '/docs')
    expect(route).toBeDefined()
    expect(route?.name).toBe('Docs')
  })
})
