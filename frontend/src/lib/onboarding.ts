/**
 * Quick-start onboarding checklist pure functions (Home.vue checklist card).
 * The card walks new users through the three setup steps and disappears once
 * every step is done or the user dismisses it. Pure state derivation only —
 * no backend calls, easy to unit-test.
 */

import type { PlatformState } from './platform'

export interface OnboardingFacts {
  /** llama.cpp runtime resolved/installed (GetLlamaCpp().installed) */
  runtimeInstalled: boolean
  /** at least one model is registered (GetModels().length > 0) */
  hasModels: boolean
  /** llama-server currently running (live monitor sample) */
  serviceRunning: boolean
  /** user dismissed the checklist, persisted via onboardingDismissed config */
  dismissed: boolean
}

export type OnboardingStepId = 'runtime' | 'models' | 'service'

export interface OnboardingStep {
  id: OnboardingStepId
  done: boolean
  /** route to navigate to when the step is pending */
  route: string
}

export interface OnboardingView {
  steps: OnboardingStep[]
  allDone: boolean
  /** render rule: hidden once dismissed OR every step is complete */
  visible: boolean
}

// Target route per step: the runtime step opens the Runtime Environment tab of
// the System Environment page (/runtime); model download lives on the download
// tab of the merged Models page (/models/download, search first); the local
// tab (/models/local) manages already-present GGUF files.
const STEP_ROUTES: Record<OnboardingStepId, string> = {
  runtime: '/runtime',
  models: '/models/download',
  service: '/api'
}

/** Build the render model for the checklist from the current system facts. */
export function buildOnboardingView(facts: OnboardingFacts): OnboardingView {
  const steps: OnboardingStep[] = (
    [
      { id: 'runtime' as const, done: facts.runtimeInstalled },
      { id: 'models' as const, done: facts.hasModels },
      { id: 'service' as const, done: facts.serviceRunning }
    ]
  ).map(step => ({ ...step, route: STEP_ROUTES[step.id] }))
  const allDone = steps.every(step => step.done)
  return { steps, allDone, visible: !facts.dismissed && !allDone }
}

// ─── Checklist copy placement per tier (Android tablet draft) ────────────────
// The tablet design draft draws the first-use Home with the checklist stacked
// above the hero (portrait frame A①), so the copy that points at "the steps
// below" applies to the touch tiers. Desktop never renders the
// checklist-anchored copy at all (unchanged behavior).

/** Minimal platform shape the placement helpers need. */
type OnboardPlacementState = Pick<PlatformState, 'isMobile' | 'isTablet'>

/**
 * i18n key of the greeting-subline checklist hint ("complete the 3 steps …"),
 * or null where the hint never renders (desktop tiers). Phone and tablet
 * portrait return the "below" copy (draft frame A①).
 */
export function onboardHintKey(state: OnboardPlacementState): string | null {
  if (state.isMobile || state.isTablet) return 'home.greet.onboardHint'
  return null
}

/**
 * i18n key of the offline hero subline that points at the checklist steps
 * ("finish steps 1–2 …"), or null to keep the generic offline subline
 * (desktop tiers, unchanged). Same split as {@link onboardHintKey}: the
 * "above" variant belongs to phone / tablet portrait (draft frame A①).
 */
export function heroOnboardSubKey(state: OnboardPlacementState): string | null {
  if (state.isMobile || state.isTablet) return 'home.hero.subOnboard'
  return null
}
