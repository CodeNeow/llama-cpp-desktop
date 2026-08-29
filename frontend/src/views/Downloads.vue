<template>
  <div class="downloads-tab">
    <!-- Search bar (non-sticky: the merged page shell owns the sticky header) -->
    <div class="search-bar">
      <div class="search-input-wrap">
        <svg class="search-icon" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/>
        </svg>
        <input
          v-model="searchQuery"
          class="search-input"
          :placeholder="t('downloads.searchPlaceholder')"
          @keydown.enter="doSearch"
        />
      </div>
      <button class="search-btn" @click="doSearch" :disabled="searching || !searchQuery">
        {{ searching ? t('downloads.searching') : t('downloads.search') }}
      </button>
      <button class="search-btn download-btn" @click="showTasksModal = true" :title="t('downloads.downloadTitle')">
        {{ t('downloads.download') }}
        <span v-if="activeTaskCount > 0" class="task-badge">{{ activeTaskCount }}</span>
      </button>
    </div>

    <!-- Search results -->
    <div v-if="searchResults.length > 0" class="results-section">
      <h2 class="section-heading">{{ t('downloads.resultsTitle', { n: searchResults.length }) }}</h2>
      <div class="results-grid">
        <div
          v-for="r in searchResults"
          :key="r.modelId"
          class="result-card"
          @click="goToDetail(r.modelId)"
          :title="t('downloads.viewDetail')"
        >
          <div class="result-main">
            <div class="result-info">
              <h3 class="result-name">{{ r.modelId }}</h3>
              <div class="result-meta">
                <span v-if="r.author" class="result-meta-item">{{ r.author }}</span>
                <span v-if="r.pipelineTag" class="result-meta-item">{{ r.pipelineTag }}</span>
                <span v-if="modelSizes[r.modelId]" class="result-meta-item">{{ formatBytes(modelSizes[r.modelId]) }}</span>
              </div>
            </div>
            <span class="result-arrow">
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                <polyline points="9 18 15 12 9 6"/>
              </svg>
            </span>
          </div>
        </div>
      </div>
    </div>

    <!-- No results -->
    <div v-else-if="searched && !searching" class="empty-state">
      <h2>{{ t('downloads.noResults') }}</h2>
      <p>{{ t('downloads.tryOther') }}</p>
    </div>

    <!-- Download tasks modal -->
    <transition name="modal-fade">
      <div v-if="showTasksModal" class="modal-overlay" @click.self="showTasksModal = false">
        <div class="modal-panel">
          <div class="modal-header">
            <h2 class="modal-title">{{ t('downloads.managerTitle') }}</h2>
            <button class="modal-close" @click="showTasksModal = false" :title="t('downloads.close')">✕</button>
          </div>
          <div class="modal-body">
            <!-- Tabs: in-flight tasks vs finished download history -->
            <div class="task-tabs">
              <button class="task-tab" :class="{ active: taskTab === 'tasks' }" @click="taskTab = 'tasks'">
                {{ t('downloads.tasksTitle', { n: activeTaskList.length }) }}
              </button>
              <button class="task-tab" :class="{ active: taskTab === 'history' }" @click="taskTab = 'history'">
                {{ t('downloads.historyTitle', { n: historyTaskList.length }) }}
              </button>
            </div>
            <div v-if="displayedTaskList.length === 0" class="tasks-empty">
              {{ taskTab === 'tasks' ? t('downloads.noActiveTasks') : t('downloads.historyEmpty') }}
            </div>
            <div v-else class="task-list">
              <div v-for="task in displayedTaskList" :key="task.id" class="task-card">
                <div class="task-info">
                  <span class="task-name">{{ task.fileName }}</span>
                  <span class="task-model">{{ task.modelId }}</span>
                </div>
                <div class="task-bar-wrap">
                  <div class="task-bar">
                    <div
                      class="task-fill"
                      :class="taskBarClass(task.status)"
                      :style="{ width: task.progress + '%' }"
                    ></div>
                  </div>
                  <span class="task-percent">{{ task.progress }}%</span>
                </div>
                <div class="task-meta">
                  <span class="task-status" :class="'status-' + task.status">{{ statusMap[task.status] || task.status }}</span>
                  <span v-if="task.status === 'downloading' && task.speed > 0" class="task-speed">{{ formatSpeed(task.speed) }}</span>
                  <span class="task-size" v-if="task.sizeHuman && task.sizeHuman !== '0 B'">{{ task.sizeHuman }}</span>
                </div>
                <div class="task-error" v-if="task.error">
                  <span>{{ task.error }}</span>
                </div>
                <div class="task-actions">
                  <button
                    v-if="task.status === 'downloading'"
                    class="task-btn pause-btn"
                    @click="pauseTask(task.id)"
                  >⏸ {{ t('downloads.pause') }}</button>
                  <button
                    v-if="task.status === 'paused'"
                    class="task-btn resume-btn"
                    @click="resumeTask(task.id)"
                  >▶ {{ t('downloads.resume') }}</button>
                  <button
                    v-if="task.status === 'error'"
                    class="task-btn retry-btn"
                    @click="retryTask(task.id)"
                  >↻ {{ t('downloads.retry') }}</button>
                  <button
                    v-if="task.status === 'downloading' || task.status === 'paused' || task.status === 'queued' || task.status === 'error'"
                    class="task-btn cancel-btn"
                    @click="cancelTask(task.id)"
                  >✕ {{ t('downloads.cancel') }}</button>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </transition>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import {
  searchDownloads, getModelMaxFileSize, getDownloadTasks,
  cancelDownloadTask, retryDownloadTask, pauseDownloadTask, resumeDownloadTask, refreshModels
} from '../wails'
import { formatSpeed, formatBytes } from '../lib/format'
import { LatestOnly } from '../lib/latestOnly'
import { hasActiveTask, countActiveTasks, visibleTasks, activeTaskItems, finishedTaskItems } from '../lib/taskStatus'
import { LimitedQueue } from '../lib/limitedQueue'
import { searchQuery, searched, searchResults, modelSizes, HFResult } from '../lib/downloadsState'
import { t } from '../lib/i18n'

interface DlTask {
  id: string
  modelId: string
  fileName: string
  status: string
  progress: number
  total: number
  downloaded: number
  sizeHuman: string
  error: string
  source: string
  speed: number
}

const router = useRouter()

const searching = ref(false)
// Concurrency-limited queue for search cards' batched size requests
let sizeQueue = new LimitedQueue(4)

const tasks = ref<DlTask[]>([])
let taskPollTimer: ReturnType<typeof setInterval> | null = null
let lastDoneCount = 0

// Download tasks modal
const showTasksModal = ref(false)
const activeTaskCount = computed(() => countActiveTasks(tasks.value))
const visibleTaskList = computed(() => visibleTasks(tasks.value))
// Tabbed split: in-flight tasks on one tab, finished downloads (done/error)
// on a history tab, so past downloads no longer mix into the task list
const activeTaskList = computed(() => activeTaskItems(visibleTaskList.value))
const historyTaskList = computed(() => finishedTaskItems(visibleTaskList.value))
const taskTab = ref<'tasks' | 'history'>('tasks')
const displayedTaskList = computed(() => (taskTab.value === 'tasks' ? activeTaskList.value : historyTaskList.value))

const statusMap = computed<Record<string, string>>(() => ({
  queued: t('downloads.statusQueued'),
  downloading: t('downloads.statusDownloading'),
  paused: t('downloads.statusPaused'),
  done: t('downloads.statusDone'),
  error: t('downloads.statusError'),
  cancelled: t('downloads.statusCancelled'),
}))

function taskBarClass(status: string): string {
  if (status === 'paused') return 'paused'
  if (status === 'error') return 'error-fill'
  return ''
}

function goToDetail(modelId: string) {
  router.push('/models/model/' + encodeURIComponent(modelId))
}

const searchGate = new LatestOnly()
async function doSearch() {
  if (!searchQuery.value) return
  const seq = searchGate.begin()
  searching.value = true
  searched.value = true
  try {
    const results = await searchDownloads(searchQuery.value, 'all')
    if (!searchGate.isLatest(seq)) return
    searchResults.value = results || []
    sizeQueue = new LimitedQueue(4)
    for (const r of searchResults.value) loadModelSize(r.modelId)
  } catch {} finally {
    if (searchGate.isLatest(seq)) searching.value = false
  }
}

function loadModelSize(modelId: string) {
  if (modelId in modelSizes) return
  modelSizes[modelId] = 0
  sizeQueue.push(async () => {
    try {
      const size = await getModelMaxFileSize(modelId)
      if (size > 0) modelSizes[modelId] = size
    } catch {
      // Silently ignore query failures
    }
  })
}

async function fetchTasks() {
  try {
    tasks.value = await getDownloadTasks() || []
    const doneCount = tasks.value.filter(t => t.status === 'done').length
    if (doneCount > lastDoneCount) {
      lastDoneCount = doneCount
      refreshModels().catch(() => {})
    }
    if (!hasActiveTask(tasks.value) && taskPollTimer) {
      clearInterval(taskPollTimer)
      taskPollTimer = null
    }
  } catch {}
}

async function cancelTask(id: string) {
  try {
    await cancelDownloadTask(id)
    ensurePolling()
  } catch {}
}

async function retryTask(id: string) {
  try {
    await retryDownloadTask(id)
    ensurePolling()
  } catch {}
}

async function pauseTask(id: string) {
  try {
    await pauseDownloadTask(id)
    ensurePolling()
  } catch {}
}

async function resumeTask(id: string) {
  try {
    await resumeDownloadTask(id)
    ensurePolling()
  } catch {}
}

function ensurePolling() {
  if (taskPollTimer) {
    fetchTasks()
  } else {
    startTaskPolling()
  }
}

function startTaskPolling() {
  taskPollTimer = setInterval(fetchTasks, 1000)
  fetchTasks()
}

onMounted(() => {
  startTaskPolling()
})
onUnmounted(() => { if (taskPollTimer) clearInterval(taskPollTimer) })
</script>

<style scoped>
.downloads-tab {
  /* No page padding: the merged shell (.page) already provides the horizontal
     padding and the bottom reserve; this panel only fills the tab body */
  min-width: 0;
}

/* ─── Search ─── */
.search-bar {
  display: flex;
  gap: 10px;
  margin-bottom: 32px;
}

.search-input-wrap {
  flex: 1;
  position: relative;
}

.search-icon {
  position: absolute;
  left: 14px;
  top: 50%;
  transform: translateY(-50%);
  color: var(--text-dim);
  pointer-events: none;
}

.search-input {
  width: 100%;
  padding: 10px 14px 10px 42px;
  background: var(--border-light);
  border: 1px solid var(--overlay-8);
  border-radius: 10px;
  color: var(--text-secondary);
  font-size: 14px;
  outline: none;
  transition: border-color 0.2s;
}

.search-input:focus {
  border-color: rgba(99, 102, 241, 0.4);
}

.search-btn {
  padding: 10px 24px;
  background: linear-gradient(135deg, rgba(99, 102, 241, 0.2), rgba(167, 139, 250, 0.15));
  color: #a78bfa;
  border: 1px solid rgba(99, 102, 241, 0.3);
  border-radius: 10px;
  font-size: 14px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s;
}

.search-btn:hover:not(:disabled) {
  background: linear-gradient(135deg, rgba(99, 102, 241, 0.3), rgba(167, 139, 250, 0.25));
}

.search-btn:disabled {
  opacity: 0.4;
  cursor: default;
}

.download-btn {
  position: relative;
}

.task-badge {
  position: absolute;
  top: -7px;
  right: -7px;
  min-width: 18px;
  height: 18px;
  padding: 0 5px;
  border-radius: 9px;
  background: #ef4444;
  color: #fff;
  font-size: 11px;
  font-weight: 700;
  line-height: 18px;
  text-align: center;
  box-sizing: border-box;
}

/* ─── Section heading ─── */
.section-heading {
  font-size: 16px;
  font-weight: 600;
  color: var(--text-secondary);
  margin: 0 0 14px;
}

/* ─── Results ─── */
.results-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 10px;
  margin-bottom: 36px;
}

.result-card {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 10px;
  cursor: pointer;
  transition: all 0.2s;
  padding: 12px 16px;
}

.result-card:hover {
  border-color: var(--overlay-10);
}

.result-main {
  display: flex;
  align-items: center;
  gap: 12px;
}

.result-info {
  flex: 1;
  min-width: 0;
}

.result-name {
  font-size: 15px;
  font-weight: 600;
  color: var(--text-primary);
  margin: 0 0 4px;
  /* Wrap at word boundaries; model names like "...uncensored-GGUF" must not be cut mid-word */
  overflow-wrap: break-word;
  line-height: 1.35;
}

.result-meta {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0;
  font-size: 12px;
  color: var(--text-dim);
}

.result-meta-item + .result-meta-item::before {
  content: '·';
  margin: 0 6px;
}

.result-arrow {
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--text-dim);
  flex-shrink: 0;
  transition: color 0.2s;
}

.result-card:hover .result-arrow {
  color: var(--text-secondary);
}

/* ─── Tasks ─── */
.task-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.task-card {
  padding: 16px 20px;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 12px;
}

.task-info {
  display: flex;
  justify-content: space-between;
  margin-bottom: 8px;
}

.task-name {
  font-size: 13px;
  font-weight: 600;
  color: var(--text-secondary);
  word-break: break-all;
}

.task-model {
  font-size: 11px;
  color: var(--text-dim);
  flex-shrink: 0;
  margin-left: 12px;
}

.task-bar-wrap {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 8px;
}

.task-bar {
  flex: 1;
  height: 5px;
  background: var(--border);
  border-radius: 3px;
  overflow: hidden;
}

.task-fill {
  height: 100%;
  border-radius: 3px;
  background: linear-gradient(90deg, #6366f1, #a78bfa);
  transition: width 0.3s;
}

.task-fill.paused {
  background: linear-gradient(90deg, #fbbf24, #f59e0b);
}

.task-fill.error-fill {
  background: rgba(239, 68, 68, 0.6);
}

.task-percent {
  font-size: 12px;
  font-weight: 600;
  color: #a78bfa;
  min-width: 36px;
  text-align: right;
}

.task-meta {
  display: flex;
  justify-content: space-between;
  font-size: 11px;
  margin-bottom: 8px;
}

.task-status {
  font-weight: 500;
}

.status-done { color: #22c55e; }
.status-downloading { color: #a78bfa; }
.status-paused { color: #fbbf24; }
.status-queued { color: var(--text-dim); }
.status-error { color: #ef4444; }
.status-cancelled { color: var(--overlay-20); }

.task-size {
  color: var(--text-dim);
}

.task-speed {
  color: var(--accent-light);
  font-weight: 600;
}

.task-error {
  font-size: 11px;
  color: #ef4444;
  margin-bottom: 8px;
  padding: 6px 10px;
  background: rgba(239, 68, 68, 0.06);
  border-radius: 6px;
}

.task-actions {
  display: flex;
  gap: 6px;
}

.task-btn {
  padding: 4px 12px;
  border-radius: 6px;
  font-size: 11px;
  font-weight: 500;
  cursor: pointer;
  border: 1px solid transparent;
  transition: all 0.2s;
}

.pause-btn {
  background: rgba(251, 191, 36, 0.1);
  color: #fbbf24;
  border-color: rgba(251, 191, 36, 0.2);
}

.pause-btn:hover { background: rgba(251, 191, 36, 0.2); }

.resume-btn {
  background: rgba(34, 197, 94, 0.1);
  color: #22c55e;
  border-color: rgba(34, 197, 94, 0.2);
}

.resume-btn:hover { background: rgba(34, 197, 94, 0.2); }

.retry-btn {
  background: rgba(99, 102, 241, 0.1);
  color: #a78bfa;
  border-color: rgba(99, 102, 241, 0.2);
}

.retry-btn:hover { background: rgba(99, 102, 241, 0.2); }

.cancel-btn {
  background: rgba(239, 68, 68, 0.06);
  color: rgba(239, 68, 68, 0.7);
  border-color: rgba(239, 68, 68, 0.12);
}

.cancel-btn:hover { background: rgba(239, 68, 68, 0.12); }

/* ─── Tasks modal ─── */
.modal-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.5);
  z-index: 1000;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px;
}

.modal-panel {
  width: 100%;
  max-width: 560px;
  /* Fixed height: switching between the tasks and history tabs never resizes
     the modal; long lists scroll inside the body instead (80vh caps it on
     very short windows) */
  height: 520px;
  max-height: 80vh;
  background: var(--bg-primary);
  border: 1px solid var(--border);
  border-radius: 14px;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  box-shadow: 0 20px 60px rgba(0, 0, 0, 0.4);
}

.modal-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px 20px;
  border-bottom: 1px solid var(--border);
  flex-shrink: 0;
}

.modal-title {
  font-size: 16px;
  font-weight: 600;
  color: var(--text-secondary);
  margin: 0;
}

.modal-close {
  background: transparent;
  border: none;
  color: var(--text-dim);
  font-size: 16px;
  line-height: 1;
  cursor: pointer;
  padding: 6px 8px;
  border-radius: 6px;
  transition: all 0.2s;
}

.modal-close:hover {
  background: var(--hover-bg);
  color: var(--text-primary);
}

.modal-body {
  padding: 16px 20px;
  overflow-y: auto;
  flex: 1;
  min-height: 0;
}

.tasks-empty {
  padding: 48px 0;
  text-align: center;
  color: var(--text-dim);
  font-size: 14px;
}

/* Tabs switching between in-flight tasks and download history; sticky so they
   stay visible while the list scrolls */
.task-tabs {
  display: flex;
  gap: 8px;
  margin: -16px -20px 12px;
  padding: 0 20px;
  position: sticky;
  top: -16px;
  z-index: 1;
  background: var(--bg-primary);
  border-bottom: 1px solid var(--border-light);
}

.task-tab {
  padding: 8px 14px;
  border: none;
  background: transparent;
  color: var(--text-dim);
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
  border-bottom: 2px solid transparent;
  margin-bottom: -1px;
  transition: color 0.15s, border-color 0.15s;
}

.task-tab:hover {
  color: var(--text-primary);
}

.task-tab.active {
  color: var(--accent-light);
  border-bottom-color: var(--accent);
}

.modal-fade-enter-active,
.modal-fade-leave-active {
  transition: opacity 0.2s;
}

.modal-fade-enter-from,
.modal-fade-leave-to {
  opacity: 0;
}

/* ─── Empty ─── */
.empty-state {
  text-align: center;
  padding: 48px;
  color: var(--text-dim);
}

.empty-state h2 {
  font-size: 18px;
  color: var(--text-muted);
  margin: 0 0 8px;
}
</style>
