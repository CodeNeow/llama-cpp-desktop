import { describe, it, expect } from 'vitest'
import { buildOnboardingView } from '../lib/onboarding'

const facts = (overrides: Partial<Parameters<typeof buildOnboardingView>[0]> = {}) => ({
  runtimeInstalled: false,
  hasModels: false,
  serviceRunning: false,
  dismissed: false,
  ...overrides
})

describe('buildOnboardingView', () => {
  it('fresh install: all steps pending and visible', () => {
    const view = buildOnboardingView(facts())
    expect(view.visible).toBe(true)
    expect(view.allDone).toBe(false)
    expect(view.steps.map(s => s.done)).toEqual([false, false, false])
  })

  it('steps carry their target routes in fixed order (runtime → models download tab → api)', () => {
    const view = buildOnboardingView(facts())
    expect(view.steps.map(s => s.id)).toEqual(['runtime', 'models', 'service'])
    expect(view.steps.map(s => s.route)).toEqual(['/runtime', '/models/download', '/api'])
  })

  it('partial completion keeps the card visible with mixed step states', () => {
    const view = buildOnboardingView(facts({ runtimeInstalled: true, hasModels: true }))
    expect(view.visible).toBe(true)
    expect(view.steps.map(s => s.done)).toEqual([true, true, false])
    expect(view.allDone).toBe(false)
  })

  it('all steps done hides the card even when not dismissed', () => {
    const view = buildOnboardingView(
      facts({ runtimeInstalled: true, hasModels: true, serviceRunning: true })
    )
    expect(view.allDone).toBe(true)
    expect(view.visible).toBe(false)
  })

  it('dismissed hides the card regardless of progress', () => {
    const view = buildOnboardingView(facts({ dismissed: true }))
    expect(view.visible).toBe(false)
    // dismissal does not fake completion: steps stay pending
    expect(view.allDone).toBe(false)
    expect(view.steps.every(s => !s.done)).toBe(true)
  })
})
