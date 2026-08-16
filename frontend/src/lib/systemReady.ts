/**
 * System readiness check: llama.cpp is installed and at least one local model exists.
 *
 * Ready only when both hard conditions hold:
 * 1) llama.cpp is installed locally (Installed = true)
 * 2) at least one model file exists under LLM-Models/ (modelCount > 0)
 */

/** System ready check: llama.cpp installed and at least one local model */
export function isSystemReady(llamaInstalled: boolean, modelCount: number): boolean {
  return llamaInstalled && modelCount > 0
}
