<template>
  <div v-if="visible" class="task-dock" ref="dockEl">
    <!-- Popover card: absolute overlay landing on the pill's original spot,
         zero layout impact, so the root size (and --dock-reserve) never changes
         when expanding. The <Transition> slides the card up + fades it in from
         the pill position, pairing with the pill's fade-out into a "morph". -->
    <Transition name="dock-pop">
      <div v-if="expanded" class="dock-popover">
        <!-- Header -->
        <div class="dock-header">
          <span class="dock-title">{{ t('dock.title') }}</span>
          <button class="dock-toggle" @click="expanded = !expanded" :title="expanded ? t('dock.collapse') : t('dock.expand')">
            <svg v-if="expanded" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="6 9 12 15 18 9"/></svg>
            <svg v-else width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="18 15 12 9 6 15"/></svg>
          </button>
        </div>

        <!-- Downloads section -->
        <div v-if="hasDownloads" class="dock-section">
          <div class="dock-section-title">{{ t('dock.downloads') }}</div>

          <!-- App self-update download; clicking the row reopens the update modal -->
          <div
            v-if="updateActive"
            class="dock-task clickable"
            :title="t('dock.viewUpdate')"
            @click="openUpdateModal"
          >
            <div class="dock-task-header">
              <span class="dock-task-name">llama-desktop {{ updateDownload?.version }}</span>
              <span class="dock-task-status" :class="'status-' + updateDownload?.status">
                {{ updateStatusLabel(updateDownload?.status || '') }}
              </span>
            </div>
            <div v-if="updateDownload?.status === 'downloading'" class="dock-bar-wrap">
              <div class="dock-bar">
                <div class="dock-fill" :style="{ width: updateDownload.progress + '%' }"></div>
              </div>
              <span class="dock-percent">{{ updateDownload.progress }}%</span>
            </div>
          </div>

          <!-- Llama.cpp download -->
          <div v-if="llamaActive" class="dock-task">
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
      </div>
    </Transition>

    <!-- Persistent compact pill: the collapsed form of the dock (download/task
         and model counters). While the popover is open the pill is visually
         hidden and yields its spot to the card (morph, see .dock-pill-hidden),
         but keeps occupying layout so --dock-reserve stays constant. -->
    <button
      class="dock-pill"
      :class="{ 'dock-pill-hidden': expanded }"
      @click="expanded = !expanded"
      :aria-label="t('dock.title')"
      :title="t('dock.title')"
    >
      <span v-if="hasDownloads" class="pill-seg">
        <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" y1="15" x2="12" y2="3"/></svg>
        <span>{{ activeDownloadCount }}</span>
      </span>
      <span v-if="serverRunning && loadedModels.length > 0" class="pill-seg">
        <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="4" y="4" width="16" height="16" rx="2"/><rect x="9" y="9" width="6" height="6"/><line x1="9" y1="1" x2="9" y2="4"/><line x1="15" y1="1" x2="15" y2="4"/><line x1="9" y1="20" x2="9" y2="23"/><line x1="15" y1="20" x2="15" y2="23"/><line x1="20" y1="9" x2="23" y2="9"/><line x1="20" y1="14" x2="23" y2="14"/><line x1="1" y1="9" x2="4" y2="9"/><line x1="1" y1="14" x2="4" y2="14"/></svg>
        <span>{{ loadedModels.length }}</span>
      </span>
      <span v-if="pillAlert" class="pill-alert"></span>
    </button>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, watchEffect, onMounted, onUnmounted } from 'vue'
import {
  getLlamaCppDownloadStatus,
  getDownloadTasks,
  getServerStatus,
  getLoadedModels,
  unloadModel
} from '../wails'
import { activeLlamaCppDownload, activeModelTasks, activeUpdateDownload, shouldShowDock } from '../lib/dock'
import { useDockReserve } from '../lib/dockSpace'
import { updateState } from '../lib/update'
import { t } from '../lib/i18n'

// ─── State ────────────────────────────────────────────────────────

interface LoadedModel {
  id: string
  type: string
  status: string
}

// Collapsed by default: only the compact pill is shown; the full card is a
// popover and the state is intentionally not persisted across mounts.
const expanded = ref(false)
let pollTimer: ReturnType<typeof setInterval> | null = null

// Llama.cpp download
// Initialize idle status instead of null to avoid template "possibly null" type error;
// shows idle before first poll, so it doesn't enter the active display area.
const llamaStatus = ref<{ status: string; progress: number }>({ status: 'idle', progress: 0 })

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

// llama.cpp is "active" when status is fetching/downloading/paused/extracting/error;
// idle/done hidden. Avoids a permanent llama.cpp row without progress info.
const llamaActive = computed(() =>
  llamaStatus.value ? activeLlamaCppDownload(llamaStatus.value.status) : false
)

// App self-update download: shown first in the downloads section; the row stays
// for done/error so the backgrounded outcome is visible until the modal is
// reopened and closed (which clears the terminal state in lib/update).
const updateDownload = computed(() => updateState.download)
const updateActive = computed(() =>
  updateDownload.value ? activeUpdateDownload(updateDownload.value.status) : false
)

const hasDownloads = computed(() => {
  return llamaActive.value || activeTasks.value.length > 0 || updateActive.value
})

// Pill summary: total active download rows (app update + llama.cpp + model tasks)
const activeDownloadCount = computed(
  () =>
    (updateActive.value ? 1 : 0) + (llamaActive.value ? 1 : 0) + activeTasks.value.length
)

// Pill alert dot: shown when any tracked download (llama.cpp / app update)
// ended in error
const pillAlert = computed(
  () => llamaStatus.value.status === 'error' || updateDownload.value?.status === 'error'
)

const visible = computed(() =>
  shouldShowDock(
    llamaActive.value,
    activeTasks.value.length,
    loadedModels.value.length,
    updateActive.value
  )
)

// ─── Dock space reservation ───────────────────────────────────────

// Publish the dock's measured height as a global reserve (consumed by App.vue
// as the `--dock-reserve` CSS variable) so overlapped page controls stay
// reachable; must run after `visible` is declared above.
const dockEl = ref<HTMLElement | null>(null)
useDockReserve(dockEl, visible)

// ─── Popover dismissal ────────────────────────────────────────────

// Collapse the popover when clicking outside the dock or pressing Escape.
// The events are not consumed (no preventDefault / stopPropagation), so an
// outside click still lands on the control the user actually aimed at.
function onDocClick(e: MouseEvent) {
  if (dockEl.value && !dockEl.value.contains(e.target as Node)) {
    expanded.value = false
  }
}

function onDocKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape') {
    expanded.value = false
  }
}

watchEffect(() => {
  if (expanded.value) {
    document.addEventListener('click', onDocClick)
    document.addEventListener('keydown', onDocKeydown)
  } else {
    document.removeEventListener('click', onDocClick)
    document.removeEventListener('keydown', onDocKeydown)
  }
})

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

// Update row labels: unlike llama.cpp rows, done/error outcomes stay visible here
function updateStatusLabel(status: string): string {
  const map: Record<string, string> = {
    downloading: t('home.dlDownloading'),
    installing: t('updateModal.installing'),
    done: t('downloads.statusDone'),
    error: t('downloads.statusError')
  }
  return map[status] || status
}

// Clicking the update row reopens the modal (download/result state lives in lib/update)
function openUpdateModal() {
  updateState.showModal = true
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
  // Safety net: never leave document listeners attached after unmount
  document.removeEventListener('click', onDocClick)
  document.removeEventListener('keydown', onDocKeydown)
})
</script>

<style scoped>
/* Pure positioning container: the pill is the only in-flow content, so the
   root size (and thus --dock-reserve) never changes when the popover opens.
   Keep `bottom: 16px` in sync with DOCK_BOTTOM_OFFSET in lib/dockSpace.ts. */
.task-dock {
  position: fixed;
  right: 16px;
  bottom: 16px;
  z-index: 50;
}

/* Full card as an absolute overlay landing on the pill's spot: `bottom: 0`
   aligns the card's bottom edge with the (now hidden) pill's bottom edge, so
   the card reads as the pill morphing into its expanded form. Absolute
   positioning keeps expansion layout-neutral, so reserved pages never jump
   when the popover shows/hides. */
.dock-popover {
  position: absolute;
  bottom: 0;
  right: 0;
  width: 300px;
  max-height: 50vh;
  background: var(--bg-secondary);
  border: 1px solid var(--border);
  border-radius: 12px;
  box-shadow: 0 6px 20px rgba(0, 0, 0, 0.18);
  display: flex;
  flex-direction: column;
  overflow: hidden;
  font-size: 12px;
}

/* Persistent compact pill: collapsed-state summary of downloads and models */
.dock-pill {
  display: flex;
  align-items: center;
  gap: 8px;
  height: 32px;
  padding: 0 12px;
  border-radius: 999px;
  background: var(--bg-secondary);
  border: 1px solid var(--border);
  box-shadow: 0 4px 14px rgba(0, 0, 0, 0.15);
  cursor: pointer;
  color: var(--text-secondary);
  font-size: 12px;
  transition: all 0.15s;
}

.dock-pill:hover {
  background: var(--hover-bg);
  color: var(--text-primary);
}

/* Collapsed-form pill hidden while the popover is open (the "morph" pairing:
   pill fades out, card fades/slides in on the pill's spot). MUST use
   visibility (not v-if / display: none): the hidden pill still occupies
   layout, so the root keeps its 32px height and the ResizeObserver-measured
   --dock-reserve stays constant while expanding/collapsing. The fade-out rides
   the existing `transition: all 0.15s` on .dock-pill, which already covers
   opacity and visibility. */
.dock-pill-hidden {
  opacity: 0;
  visibility: hidden;
  pointer-events: none;
}

/* Popover morph entrance/exit: slides up + fades in from the pill position,
   completing the morph illusion together with the pill's fade-out. */
.dock-pop-enter-active,
.dock-pop-leave-active {
  transition: opacity 0.15s ease, transform 0.15s ease;
}

.dock-pop-enter-from,
.dock-pop-leave-to {
  opacity: 0;
  transform: translateY(6px);
}

/* One metric per segment: 12px inline icon + bold count */
.pill-seg {
  display: flex;
  align-items: center;
  gap: 4px;
  font-weight: 600;
}

/* Error indicator dot */
.pill-alert {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: #ef4444;
  flex-shrink: 0;
}

.dock-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 12px;
  border-bottom: 1px solid var(--border-light);
  flex-shrink: 0;
}

.dock-title {
  font-size: 12px;
  font-weight: 700;
  color: var(--text-primary);
}

.dock-toggle {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 20px;
  height: 20px;
  border-radius: 5px;
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
  padding: 8px 12px;
  border-bottom: 1px solid var(--border-light);
}

.dock-section:last-child {
  border-bottom: none;
}

.dock-section-title {
  font-size: 10px;
  font-weight: 600;
  color: var(--text-dim);
  text-transform: uppercase;
  letter-spacing: 0.4px;
  margin-bottom: 6px;
}

.dock-task {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 6px 0;
}

.dock-task + .dock-task {
  border-top: 1px solid var(--border-light);
  margin-top: 4px;
  padding-top: 8px;
}

/* Clickable task row (app update): reopens the related modal */
.dock-task.clickable {
  cursor: pointer;
}

.dock-task.clickable:hover .dock-task-name {
  color: var(--text-primary);
}

.dock-task-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 8px;
}

.dock-task-name {
  font-size: 11px;
  font-weight: 500;
  color: var(--text-secondary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  min-width: 0;
}

.dock-task-status {
  font-size: 10px;
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
  gap: 6px;
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
  font-size: 10px;
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
  gap: 6px;
}

.dock-model-badge {
  display: inline-flex;
  align-items: center;
  padding: 1px 6px;
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
  font-size: 10px;
}

.dock-model-status {
  font-size: 10px;
  color: var(--text-dim);
  flex-shrink: 0;
}

.dock-unload-btn {
  padding: 2px 8px;
  border-radius: 6px;
  font-size: 10px;
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
  font-size: 10px;
  color: #ef4444;
}
</style>
