/**
 * Canonical frontend vocabularies for the three download-status state machines
 * mirrored from the backend:
 * - LLAMA_CPP_DOWNLOAD_STATUSES: llama.cpp runtime download (`downloadState.Status`,
 *   documented in AGENTS.md "Go Backend" code standards).
 * - MODEL_TASK_STATUSES: model download tasks (`DlTask.status`).
 * - UPDATE_DOWNLOAD_STATUSES: app self-update download (`UpdateDownloadState.status`).
 *
 * Adding or renaming a backend status requires updating these lists AND the
 * exhaustive Record tables in `src/__tests__/` (llamaDownload / dock / taskStatus):
 * a missing entry fails vue-tsc because those Record tables are keyed by the
 * union types below, and a wrongly-listed entry fails the runtime test that
 * iterates these canonical lists.
 *
 * The pure functions in lib/ (llamaDownload.ts / dock.ts / taskStatus.ts) keep
 * `status: string` signatures on purpose — backend data arrives as plain JSON
 * strings — so this module is consumed by tests and available for future
 * components that want typed statuses.
 */

/** llama.cpp runtime download statuses (backend `downloadState.Status`). */
export const LLAMA_CPP_DOWNLOAD_STATUSES = [
  'idle',
  'fetching',
  'downloading',
  'paused',
  'extracting',
  'done',
  'error',
] as const
export type LlamaCppDownloadStatus = (typeof LLAMA_CPP_DOWNLOAD_STATUSES)[number]

/** Model download task statuses (backend `DlTask.status`). */
export const MODEL_TASK_STATUSES = [
  'queued',
  'fetching',
  'downloading',
  'paused',
  'extracting',
  'done',
  'error',
  'cancelled',
] as const
export type ModelTaskStatus = (typeof MODEL_TASK_STATUSES)[number]

/** App self-update download statuses (frontend `UpdateDownloadState.status`). */
export const UPDATE_DOWNLOAD_STATUSES = ['idle', 'downloading', 'done', 'error'] as const
export type UpdateDownloadStatus = (typeof UPDATE_DOWNLOAD_STATUSES)[number]
