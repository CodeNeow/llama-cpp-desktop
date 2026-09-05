import { describe, it, expect } from 'vitest'
import { buildOnboardingView, heroOnboardSubKey, onboardHintKey } from '../lib/onboarding'
import { buildPlatformState } from '../lib/platform'

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

// ─── Checklist copy placement per tier (Android tablet draft) ────────────────
// buildPlatformState is the canonical way to produce the classifier input, so
// the placement helpers are exercised through real tier combinations.

describe('onboardHintKey', () => {
  it('tablet tier (portrait band) keeps the original "below" copy (draft A①)', () => {
    const s = buildPlatformState('windows', 900)
    expect(s.isTablet).toBe(true)
    expect(onboardHintKey(s)).toBe('home.greet.onboardHint')
  })

  it('phone tier keeps the original "below" copy (unchanged)', () => {
    expect(onboardHintKey(buildPlatformState('android', 390))).toBe(
      'home.greet.onboardHint'
    )
  })

  it('desktop tier never renders the hint (null, unchanged behavior)', () => {
    expect(onboardHintKey(buildPlatformState('windows', 1280))).toBeNull()
    expect(onboardHintKey(buildPlatformState('windows', 1920))).toBeNull()
  })
})

describe('heroOnboardSubKey', () => {
  it('tablet tier / phone point at the steps above the hero (draft A①)', () => {
    expect(heroOnboardSubKey(buildPlatformState('windows', 900))).toBe(
      'home.hero.subOnboard'
    )
    expect(heroOnboardSubKey(buildPlatformState('android', 390))).toBe(
      'home.hero.subOnboard'
    )
  })

  it('desktop keeps the generic offline subline (null, unchanged behavior)', () => {
    expect(heroOnboardSubKey(buildPlatformState('windows', 1280))).toBeNull()
    expect(heroOnboardSubKey(buildPlatformState('linux', 1100))).toBeNull()
  })
})
