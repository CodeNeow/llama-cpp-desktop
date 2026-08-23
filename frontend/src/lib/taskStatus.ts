/** Whether the task list still has active tasks (polling must continue). Terminal states: done/error/cancelled. */
export function hasActiveTask(tasks: { status: string }[]): boolean {
  return tasks.some(t => t.status === 'downloading' || t.status === 'paused' || t.status === 'queued')
}

/** Count active tasks (downloading/paused/queued), used for the download button badge. */
export function countActiveTasks(tasks: { status: string }[]): number {
  return tasks.filter(t => t.status === 'downloading' || t.status === 'paused' || t.status === 'queued').length
}

/** Task list for modal display: cancelled tasks are filtered out (user-cancelled, no longer shown; retry stays available for error/done) */
export function visibleTasks<T extends { status: string }>(tasks: T[]): T[] {
  return tasks.filter(t => t.status !== 'cancelled')
}

/** Active (in-flight) tasks for the task group of the downloads modal:
 * queued / downloading / paused. Finished tasks render separately below
 * (finishedTaskItems) so history no longer mixes into the task list. */
export function activeTaskItems<T extends { status: string }>(tasks: T[]): T[] {
  return tasks.filter(t => t.status === 'downloading' || t.status === 'paused' || t.status === 'queued')
}

/** Finished tasks for the download-history group (done / error, retry stays
 * available for error); cancelled stays hidden via visibleTasks. */
export function finishedTaskItems<T extends { status: string }>(tasks: T[]): T[] {
  return tasks.filter(t => t.status === 'done' || t.status === 'error')
}
