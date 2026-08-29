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

// Target route per step: the runtime step deep-links to the runtime section of
// the merged System Environment page (Home honors ?section=runtime by
// scrolling to it); model download lives on the download tab of the merged
// Models page (/models/download, search first); the local tab (/models/local)
// manages already-present GGUF files.
const STEP_ROUTES: Record<OnboardingStepId, string> = {
  runtime: '/?section=runtime',
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
