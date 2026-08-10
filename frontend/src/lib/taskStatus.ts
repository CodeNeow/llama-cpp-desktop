/** 判断任务列表中是否仍有活跃任务（需要继续轮询）。终态：done/error/cancelled。 */
export function hasActiveTask(tasks: { status: string }[]): boolean {
  return tasks.some(t => t.status === 'downloading' || t.status === 'paused' || t.status === 'queued')
}
