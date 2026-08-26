/**
 * Quick-start onboarding checklist pure functions (Home.vue checklist card).
 * The card walks new users through the three setup steps and disappears once
 * every step is done or the user dismisses it. Pure state derivation only —
 * no backend calls, easy to unit-test.
 */

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

// Target route per step: models download lives on /downloads (search first),
// not /models which manages already-present GGUF files.
const STEP_ROUTES: Record<OnboardingStepId, string> = {
  runtime: '/runtime',
  models: '/downloads',
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
