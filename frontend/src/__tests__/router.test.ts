import { describe, it, expect } from 'vitest'
import router from '../router'

// Fixed-viewport pages (Home / Chat / Api) manage their own internal scroll
// bands and reserve TaskDock space themselves; App.vue relies on route meta
// `fixed` to switch .content-area into that mode (no bottom reserve, no own
// scrolling). These tests pin that contract: exactly the three fixed pages
// are flagged, every scrollable page is not. The former /runtime page merged
// into Home and survives only as a redirect record without meta.
describe('router fixed-page meta', () => {
  const fixedPaths = ['/', '/chat', '/api']

  // Scrollable pages keep normal document flow: .content-area scrolls and
  // reserves the TaskDock band for them (subpages and the /models tab children
  // included; /models is the tabbed shell with its own children)
  const scrollingPaths = [
    '/models',
    '/models/download',
    '/models/local',
    '/models/model/:modelId',
    '/models/settings/:modelName',
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
