/** 判断任务列表中是否仍有活跃任务（需要继续轮询）。终态：done/error/cancelled。 */
export function hasActiveTask(tasks: { status: string }[]): boolean {
  return tasks.some(t => t.status === 'downloading' || t.status === 'paused' || t.status === 'queued')
}

/** 统计活跃任务数量（下载中/已暂停/排队中），用于下载按钮角标计数。 */
export function countActiveTasks(tasks: { status: string }[]): number {
  return tasks.filter(t => t.status === 'downloading' || t.status === 'paused' || t.status === 'queued').length
}
