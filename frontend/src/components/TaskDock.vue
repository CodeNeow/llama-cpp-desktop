<template>
  <div v-if="visible" class="task-dock">
    <!-- Header -->
    <div class="dock-header">
      <span class="dock-title">{{ t('dock.title') }}</span>
      <button class="dock-toggle" @click="expanded = !expanded" :title="expanded ? t('dock.collapse') : t('dock.expand')">
        <svg v-if="expanded" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="6 9 12 15 18 9"/></svg>
        <svg v-else width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="18 15 12 9 6 15"/></svg>
      </button>
    </div>

    <!-- Expanded content -->
    <template v-if="expanded">
      <!-- Downloads section -->
      <div v-if="hasDownloads" class="dock-section">
        <div class="dock-section-title">{{ t('dock.downloads') }}</div>

        <!-- Llama.cpp download -->
        <div v-if="llamaStatus" class="dock-task">
          <div class="dock-task-header">
            <span class="dock-task-name">llama.cpp</span>
            <span class="dock-task-status" :class="'status-' + llamaStatus.status">{{ statusLabel(llamaStatus.status) }}</span>
          </div>
          <div v-if="isProgressStatus(llamaStatus.status)" class="dock-bar-wrap">
            <div class="dock-bar">
              <div
                class="dock-fill"
                :class="{ paused: llamaStatus.status === 'paused' }"
                :style="{ width: llamaStatus.progress + '%' }"
              ></div>
            </div>
            <span class="dock-percent">{{ llamaStatus.progress }}%</span>
          </div>
        </div>

        <!-- Model download tasks -->
        <div v-for="task in activeTasks" :key="task.id" class="dock-task">
          <div class="dock-task-header">
            <span class="dock-task-name" :title="task.fileName">{{ truncatedName(task.fileName) }}</span>
            <span class="dock-task-status" :class="'status-' + task.status">{{ statusLabel(task.status) }}</span>
          </div>
          <div v-if="task.status === 'downloading' || task.status === 'paused'" class="dock-bar-wrap">
            <div class="dock-bar">
              <div
                class="dock-fill"
                :class="{ paused: task.status === 'paused' }"
                :style="{ width: task.progress + '%' }"
              ></div>
            </div>
            <span class="dock-percent">{{ task.progress }}%</span>
          </div>
        </div>
      </div>

      <!-- Models in memory section -->
      <div v-if="serverRunning && loadedModels.length > 0" class="dock-section">
        <div class="dock-section-title">{{ t('dock.modelsInMemory', { n: loadedModels.length }) }}</div>
        <div v-for="model in loadedModels" :key="model.id" class="dock-model-item">
          <div class="dock-model-row">
            <span class="dock-model-badge" :class="'type-' + model.type">{{ typeLabel(model.type) }}</span>
            <span class="dock-model-id" :title="model.id">{{ truncatedName(model.id) }}</span>
            <span class="dock-model-status">{{ modelStatusLabel(model.status) }}</span>
            <button
              class="dock-unload-btn"
              :disabled="unloadingId === model.id"
              @click="handleUnload(model.id)"
            >
              {{ unloadingId === model.id ? t('dock.unloading') : t('dock.unload') }}
            </button>
          </div>
          <div v-if="unloadErrors[model.id]" class="dock-unload-error">
            {{ t('dock.unloadFailed', { msg: unloadErrors[model.id] }) }}
          </div>
        </div>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, onUnmounted } from 'vue'
import {
  getLlamaCppDownloadStatus,
  getDownloadTasks,
  getServerStatus,
  getLoadedModels,
  unloadModel
} from '../wails'
import { activeLlamaCppDownload, activeModelTasks, shouldShowDock } from '../lib/dock'
import { t } from '../lib/i18n'

// ─── State ────────────────────────────────────────────────────────

interface LoadedModel {
  id: string
  type: string
  status: string
}

const expanded = ref(true)
let pollTimer: ReturnType<typeof setInterval> | null = null

// Llama.cpp download
const llamaStatus = ref<{ status: string; progress: number } | null>(null)

// Model download tasks
const allTasks = ref<{ id: string; fileName: string; status: string; progress: number }[]>([])

// Server status
const serverRunning = ref(false)

// Loaded models from router
const loadedModels = ref<LoadedModel[]>([])

// Unload state
const unloadingId = ref('')
const unloadErrors = reactive<Record<string, string>>({})

// ─── Computed ─────────────────────────────────────────────────────

const activeTasks = computed(() => activeModelTasks(allTasks.value))

const hasDownloads = computed(() => {
  return (llamaStatus.value && activeLlamaCppDownload(llamaStatus.value.status)) || activeTasks.value.length > 0
})

const visible = computed(() =>
  shouldShowDock(
    llamaStatus.value ? activeLlamaCppDownload(llamaStatus.value.status) : false,
    activeTasks.value.length,
    loadedModels.value.length
  )
)

// ─── Polling ──────────────────────────────────────────────────────

async function poll() {
  try {
    const [llama, tasks, server] = await Promise.all([
      getLlamaCppDownloadStatus(),
      getDownloadTasks(),
      getServerStatus()
    ])

    llamaStatus.value = {
      status: llama.status,
      progress: llama.progress || 0
    }

    allTasks.value = tasks.map(t => ({
      id: t.id,
      fileName: t.fileName,
      status: t.status,
      progress: t.progress || 0
    }))

    serverRunning.value = server.running

    if (server.running) {
      try {
        loadedModels.value = await getLoadedModels()
      } catch {
        // transient: keep previous models until next tick
      }
    } else {
      loadedModels.value = []
    }
  } catch {
    // Wails backend not available (standalone vite): silently ignore
  }
}

function startPolling() {
  stopPolling()
  pollTimer = setInterval(poll, 1000)
  poll()
}

function stopPolling() {
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
}

// ─── Unload ───────────────────────────────────────────────────────

async function handleUnload(id: string) {
  unloadingId.value = id
  delete unloadErrors[id]
  try {
    await unloadModel(id)
    // success: next poll will refresh the list
  } catch (e: any) {
    unloadErrors[id] = e?.message || 'unknown'
  } finally {
    unloadingId.value = ''
  }
}

// ─── Labels ───────────────────────────────────────────────────────

function statusLabel(status: string): string {
  const map: Record<string, string> = {
    fetching: t('home.dlFetching'),
    downloading: t('home.dlDownloading'),
    paused: t('home.dlPaused'),
    extracting: t('home.dlExtracting'),
    done: '',
    error: t('home.dlError'),
    idle: '',
    queued: t('downloads.statusQueued'),
    cancelled: t('downloads.statusCancelled')
  }
  return map[status] || status
}

function isProgressStatus(status: string): boolean {
  return status === 'downloading' || status === 'paused'
}

function typeLabel(type: string): string {
  const map: Record<string, string> = {
    chat: t('dock.modelType.chat'),
    audio: t('dock.modelType.audio'),
    image: t('dock.modelType.image'),
    video: t('dock.modelType.video')
  }
  return map[type] || type
}

function modelStatusLabel(status: string): string {
  const map: Record<string, string> = {
    loaded: t('dock.modelStatus.loaded'),
    loading: t('dock.modelStatus.loading'),
    sleeping: t('dock.modelStatus.sleeping')
  }
  return map[status] || status
}

function truncatedName(name: string, maxLen = 22): string {
  if (name.length <= maxLen) return name
  const half = Math.floor(maxLen / 2) - 1
  return name.slice(0, half) + '…' + name.slice(-half)
}

// ─── Lifecycle ────────────────────────────────────────────────────

onMounted(() => {
  startPolling()
})

onUnmounted(() => {
  stopPolling()
})
</script>

<style scoped>
.task-dock {
  position: fixed;
  right: 20px;
  bottom: 20px;
  width: 340px;
  max-height: 60vh;
  z-index: 50;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 14px;
  box-shadow: 0 8px 30px rgba(0, 0, 0, 0.25);
  display: flex;
  flex-direction: column;
  overflow: hidden;
  font-size: 12px;
}

.dock-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 14px;
  border-bottom: 1px solid var(--border-light);
  flex-shrink: 0;
}

.dock-title {
  font-size: 13px;
  font-weight: 700;
  color: var(--text-primary);
}

.dock-toggle {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 24px;
  border-radius: 6px;
  border: none;
  background: transparent;
  color: var(--text-dim);
  cursor: pointer;
  transition: all 0.15s;
}

.dock-toggle:hover {
  background: var(--hover-bg);
  color: var(--text-primary);
}

.dock-section {
  padding: 10px 14px;
  border-bottom: 1px solid var(--border-light);
}

.dock-section:last-child {
  border-bottom: none;
}

.dock-section-title {
  font-size: 11px;
  font-weight: 600;
  color: var(--text-dim);
  text-transform: uppercase;
  letter-spacing: 0.4px;
  margin-bottom: 8px;
}

.dock-task {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 6px 0;
}

.dock-task + .dock-task {
  border-top: 1px solid var(--border-light);
  margin-top: 4px;
  padding-top: 8px;
}

.dock-task-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 8px;
}

.dock-task-name {
  font-size: 12px;
  font-weight: 500;
  color: var(--text-secondary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  min-width: 0;
}

.dock-task-status {
  font-size: 11px;
  font-weight: 500;
  flex-shrink: 0;
}

.status-fetching { color: var(--text-dim); }
.status-downloading { color: #a78bfa; }
.status-paused { color: #fbbf24; }
.status-extracting { color: #22c55e; }
.status-error { color: #ef4444; }
.status-done { color: #22c55e; }
.status-idle { color: var(--text-dim); }
.status-queued { color: var(--text-dim); }
.status-cancelled { color: var(--overlay-20); }

.dock-bar-wrap {
  display: flex;
  align-items: center;
  gap: 8px;
}

.dock-bar {
  flex: 1;
  height: 4px;
  background: var(--border);
  border-radius: 2px;
  overflow: hidden;
}

.dock-fill {
  height: 100%;
  border-radius: 2px;
  background: linear-gradient(90deg, #6366f1, #a78bfa);
  transition: width 0.3s ease;
}

.dock-fill.paused {
  background: linear-gradient(90deg, #fbbf24, #f59e0b);
}

.dock-percent {
  font-size: 11px;
  font-weight: 700;
  color: #a78bfa;
  min-width: 32px;
  text-align: right;
}

/* ─── Models in memory ─── */

.dock-model-item {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 5px 0;
}

.dock-model-item + .dock-model-item {
  border-top: 1px solid var(--border-light);
  margin-top: 4px;
  padding-top: 6px;
}

.dock-model-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.dock-model-badge {
  display: inline-flex;
  align-items: center;
  padding: 1px 8px;
  border-radius: 10px;
  font-size: 10px;
  font-weight: 700;
  flex-shrink: 0;
  letter-spacing: 0.2px;
}

.type-chat {
  background: rgba(167, 139, 250, 0.12);
  color: #a78bfa;
  border: 1px solid rgba(167, 139, 250, 0.2);
}

.type-audio {
  background: rgba(34, 197, 94, 0.1);
  color: #22c55e;
  border: 1px solid rgba(34, 197, 94, 0.18);
}

.type-image {
  background: rgba(59, 130, 246, 0.1);
  color: #3b82f6;
  border: 1px solid rgba(59, 130, 246, 0.18);
}

.type-video {
  background: rgba(245, 158, 11, 0.1);
  color: #f59e0b;
  border: 1px solid rgba(245, 158, 11, 0.18);
}

.dock-model-id {
  flex: 1;
  font-weight: 500;
  color: var(--text-secondary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  min-width: 0;
  font-family: 'JetBrains Mono', 'Fira Code', monospace;
  font-size: 11px;
}

.dock-model-status {
  font-size: 11px;
  color: var(--text-dim);
  flex-shrink: 0;
}

.dock-unload-btn {
  padding: 3px 10px;
  border-radius: 6px;
  font-size: 11px;
  font-weight: 500;
  cursor: pointer;
  border: 1px solid rgba(239, 68, 68, 0.2);
  background: rgba(239, 68, 68, 0.06);
  color: rgba(239, 68, 68, 0.8);
  transition: all 0.15s;
  flex-shrink: 0;
}

.dock-unload-btn:hover:not(:disabled) {
  background: rgba(239, 68, 68, 0.14);
  border-color: rgba(239, 68, 68, 0.35);
  color: rgba(239, 68, 68, 0.95);
}

.dock-unload-btn:disabled {
  opacity: 0.6;
  cursor: wait;
}

.dock-unload-error {
  padding: 6px 10px;
  background: rgba(239, 68, 68, 0.06);
  border-radius: 6px;
  font-size: 11px;
  color: #ef4444;
}
</style>
