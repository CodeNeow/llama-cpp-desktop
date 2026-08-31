<template>
  <!-- The root keeps its fixed right/bottom CSS anchor at all times; the
       capsule position rides a translate() (see the drag section below), so
       the ResizeObserver-measured box — and therefore --dock-reserve and
       --dock-width — is unaffected by where the capsule sits. -->
  <div
    v-if="visible"
    ref="dockEl"
    class="task-dock"
    :class="{ 'task-dock--dragging': dragging, 'task-dock--left': dockSide === 'left' }"
    :style="{ transform: `translate(${dragTranslate.x}px, ${dragTranslate.y}px)` }"
  >
    <!-- Popover card: absolute overlay landing on the pill's original spot,
         zero layout impact, so the root size (and --dock-reserve) never changes
         when expanding. The <Transition> slides the card up + fades it in from
         the pill position, pairing with the pill's fade-out into a "morph".
         Anchoring mirrors the capsule's side (left-hugging pills open the card
         toward the window interior) and flips to top-anchored growth when the
         capsule rides in the upper half of the viewport. -->
    <Transition name="dock-pop">
      <div v-if="expanded" class="dock-popover" :class="{ 'dock-popover--below': cardBelow }">
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
          <div
            v-for="model in loadedModels"
            :key="model.id"
            class="dock-model-item"
            :class="{ 'dock-model-item--unloading': unloadingId === model.id }"
          >
            <div class="dock-model-row">
              <span class="dock-model-badge" :class="'type-' + model.type">{{ typeLabel(model.type) }}</span>
              <span class="dock-model-id" :title="model.id">{{ truncatedName(model.id) }}</span>
              <span class="dock-model-status">{{ unloadingId === model.id ? t('dock.unloading') : modelStatusLabel(model.status) }}</span>
              <!-- Capability gate: direct-mode servers (Android) have no
                   unload route — the single resident leaves memory only by
                   stopping the service, so the affordance is hidden there. -->
              <button
                v-if="canUnloadModels"
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
         and model counters), draggable to either window edge while collapsed.
         While the popover is open the pill is visually hidden and yields its
         spot to the card (morph, see .dock-pill-hidden), but keeps occupying
         layout so --dock-reserve stays constant. Click-vs-drag discrimination
         lives in the pointer handlers: a press moving <= 6px is a click
         (toggles the card, keyboard Enter/Space included), anything more is a
         drag whose release snaps the pill and must not expand the card. -->
    <button
      class="dock-pill"
      :class="{ 'dock-pill-hidden': expanded }"
      @click="onPillClick"
      @pointerdown="onPillPointerDown"
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
import { ref, reactive, computed, watch, watchEffect, nextTick, onMounted, onUnmounted } from 'vue'
import {
  getLlamaCppDownloadStatus,
  getDownloadTasks,
  getServerStatus,
  getLoadedModels,
  unloadModel
} from '../wails'
import { activeLlamaCppDownload, activeModelTasks, activeUpdateDownload, shouldShowDock } from '../lib/dock'
import { dockNudgeCounter, nudgeDock } from '../lib/dockNudge'
import { dockLane, dockSide, dockWidth, useDockReserve } from '../lib/dockSpace'
import {
  anchorTopLeft,
  clampTopPx,
  isBeyondDragThreshold,
  laneFor,
  loadStoredPosition,
  nearestSide,
  normToTop,
  saveStoredPosition,
  topToNorm,
  translateForPosition,
  CHAT_COMPOSER_BAND_DESKTOP,
  CHAT_COMPOSER_BAND_MOBILE,
  DOCK_TOP_GAP,
  DOCK_BOTTOM_GAP_DESKTOP,
  DOCK_BOTTOM_GAP_MOBILE,
  DOCK_ANCHOR_BOTTOM_DESKTOP,
  DOCK_ANCHOR_BOTTOM_MOBILE,
  type DockLayoutMetrics,
  type DockPoint,
  type DockStoredPosition,
} from '../lib/dockPosition'
import { updateState } from '../lib/update'
import { usePlatform } from '../lib/platform'
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

const platform = usePlatform()

// In-memory unload is a router-mode (desktop) capability: direct-mode servers
// (Android) always hold exactly one resident model that can only leave memory
// by stopping the service — hide the row's unload button there.
const canUnloadModels = computed(() => !platform.value.isAndroid)

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

// ─── Draggable capsule: position memory + edge snapping ───────────
// The pill is an AssistiveTouch-style capsule: press-and-move drags it freely
// (transform translate, never right/top writes), releasing snaps it to the
// nearest left/right edge while the dropped vertical spot is clamped into the
// safe band, and the result persists as viewport-normalized fractions in
// localStorage. All geometry lives in lib/dockPosition.ts; this block only
// wires DOM events + platform facts into it. Component-local state only —
// deliberately not in the global store.

// Persisted capsule position, loaded once per mount; null = nothing stored
// (corrupted data degrades here too) = the legacy bottom-right anchor spot.
const storedPosition = ref<DockStoredPosition | null>(loadStoredPosition())

// Current translate (px) applied to the fixed root, and the raw-follow flag
// that switches the root's transform transition off while dragging.
const dragTranslate = ref<DockPoint>({ x: 0, y: 0 })
const dragging = ref(false)

// Expand-card vertical anchor: bottom-anchored (default) the card grows
// upward from the pill; when the capsule rides in the upper half that would
// overflow the viewport top, so the card anchors below the pill instead.
const cardBelow = ref(false)

// Chrome bands measured from the live DOM: the custom title bar renders only
// on frameless desktop shells (absent via v-if elsewhere), and the mobile nav
// is display:none outside the phone tier — offsetHeight is 0 in both cases, so
// one measurement works on every tier. Safe-area insets cannot be read from JS
// (env() is CSS-only); desktop WebView2 always reports 0, and Android
// edge-to-edge under-clamps by the inset amount — an accepted limitation.
function measureChromePx(): { top: number; bottom: number } {
  const titlebar = document.querySelector('.title-bar')
  const nav = document.querySelector('.mobile-nav')
  return {
    top: titlebar instanceof HTMLElement ? titlebar.offsetHeight : 0,
    bottom: nav instanceof HTMLElement ? nav.offsetHeight : 0,
  }
}

// Assemble the pure-math inputs from the live viewport, the measured pill box
// (the same offsetWidth/offsetHeight the dock-space observer reads — a
// transform never changes them) and the current viewport tier.
function layoutMetrics(): DockLayoutMetrics {
  const chrome = measureChromePx()
  const isPhone = platform.value.isMobile
  return {
    viewportW: window.innerWidth,
    viewportH: window.innerHeight,
    pillW: dockEl.value?.offsetWidth || 0,
    pillH: dockEl.value?.offsetHeight || 0,
    minTop: chrome.top + DOCK_TOP_GAP,
    clampBottomGap: isPhone ? chrome.bottom + DOCK_BOTTOM_GAP_MOBILE : DOCK_BOTTOM_GAP_DESKTOP,
    anchorBottomOffset: isPhone
      ? chrome.bottom + DOCK_ANCHOR_BOTTOM_MOBILE
      : DOCK_ANCHOR_BOTTOM_DESKTOP,
  }
}

// Apply the stored position (or the legacy default) as a root translate.
// Skipped while the pill has no real box yet (v-if just mounted, tests).
function applyStoredPosition(): void {
  const layout = layoutMetrics()
  if (layout.pillW <= 0 || layout.pillH <= 0) return
  const stored = storedPosition.value
  dragTranslate.value = translateForPosition(stored, layout)
  dockSide.value = stored?.side ?? 'right'
  updateCardAnchor(layout)
  publishLane(layout)
}

// Chat-page lane state (lib/dockSpace dockLane): 'left'/'right' only while
// the capsule's viewport bottom edge reaches into the composer band at the
// bottom of the window, 'none' while it parks mid-screen / up top. The bottom
// edge is DERIVED from the anchor + current translate + pill height (the same
// reference frame the transform itself is built on) rather than
// getBoundingClientRect, which stays 0/undefined in unit-test DOMs and would
// race the 0.2s snap transition right after a release. Called only at
// settle/restore/resize points — the lane follows the RELEASED anchor, so a
// mid-drag position never flaps the chat padding frame-by-frame.
function publishLane(layout: DockLayoutMetrics): void {
  if (layout.pillW <= 0 || layout.pillH <= 0) return // not laid out yet: republished on measure
  const anchor = anchorTopLeft(layout)
  const capsuleBottom = anchor.y + dragTranslate.value.y + layout.pillH
  const band = platform.value.isMobile ? CHAT_COMPOSER_BAND_MOBILE : CHAT_COMPOSER_BAND_DESKTOP
  dockLane.value = laneFor(dockSide.value, capsuleBottom, layout.viewportH, band)
}

// Flip the expand card to top-anchored growth when the capsule's (resolved)
// spot sits in the upper half of the viewport.
function updateCardAnchor(layout: DockLayoutMetrics): void {
  const anchor = anchorTopLeft(layout)
  const stored = storedPosition.value
  const top = stored ? normToTop(stored.yNorm, layout) : anchor.y
  cardBelow.value = top + layout.pillH < layout.viewportH / 2
}

// Drag gesture bookkeeping: press point + translate at press, whether the
// pointer traveled beyond the click threshold, and the click-suppression flag
// (the browser synthesizes a click after a drag pointerup, which must not
// toggle the card).
const dragPress = { px: 0, py: 0, tx: 0, ty: 0 }
let dragMoved = false
let suppressNextClick = false

function onPillPointerDown(e: PointerEvent) {
  if (expanded.value) return // popover open: pill hidden and never draggable
  if (e.isPrimary === false) return
  suppressNextClick = false
  dragMoved = false
  dragPress.px = e.clientX
  dragPress.py = e.clientY
  dragPress.tx = dragTranslate.value.x
  dragPress.ty = dragTranslate.value.y
  // Track the gesture on the window so the pointer may leave the small pill
  // mid-drag; works identically for mouse and touch (scroll interference is
  // blocked by touch-action: none on the pill).
  window.addEventListener('pointermove', onWindowPointerMove)
  window.addEventListener('pointerup', onWindowPointerUp)
  window.addEventListener('pointercancel', onWindowPointerCancel)
}

function onWindowPointerMove(e: PointerEvent) {
  const dx = e.clientX - dragPress.px
  const dy = e.clientY - dragPress.py
  if (!dragMoved) {
    if (!isBeyondDragThreshold(dx, dy)) return
    dragMoved = true
    dragging.value = true
  }
  const layout = layoutMetrics()
  let ny = dragPress.ty + dy
  // Live vertical clamp keeps the capsule inside its safe band mid-drag; the
  // horizontal axis stays free until the release snap picks the nearest edge.
  if (layout.pillW > 0 && layout.pillH > 0) {
    const anchor = anchorTopLeft(layout)
    ny = clampTopPx(anchor.y + ny, layout) - anchor.y
  }
  dragTranslate.value = { x: dragPress.tx + dx, y: ny }
}

function onWindowPointerUp() {
  finishDrag()
}

// Touch cancellation (e.g. an incoming system gesture) parks the capsule the
// same way a release does.
function onWindowPointerCancel() {
  finishDrag()
}

function finishDrag() {
  window.removeEventListener('pointermove', onWindowPointerMove)
  window.removeEventListener('pointerup', onWindowPointerUp)
  window.removeEventListener('pointercancel', onWindowPointerCancel)
  if (dragMoved) {
    // Swallow exactly the drag-release compatibility click (self-clearing so
    // later clicks and keyboard activation still toggle), then snap.
    suppressNextClick = true
    setTimeout(() => {
      suppressNextClick = false
    }, 0)
    settleAfterDrag()
  }
  dragMoved = false
  dragging.value = false
}

// Snap on release: hug the nearest horizontal edge, keep the dropped vertical
// position clamped into the safe band, persist it, and republish the side.
function settleAfterDrag() {
  const layout = layoutMetrics()
  if (layout.pillW <= 0 || layout.pillH <= 0) return
  const anchor = anchorTopLeft(layout)
  const curLeft = anchor.x + dragTranslate.value.x
  const curTop = anchor.y + dragTranslate.value.y
  const side = nearestSide(curLeft + layout.pillW / 2, layout.viewportW)
  const stored: DockStoredPosition = {
    side,
    yNorm: topToNorm(clampTopPx(curTop, layout), layout),
  }
  storedPosition.value = stored
  saveStoredPosition(stored)
  dockSide.value = side
  // The root's transform transition (re-enabled now that `dragging` dropped)
  // glides the capsule from the release point onto the snapped spot.
  dragTranslate.value = translateForPosition(stored, layout)
  updateCardAnchor(layout)
  publishLane(layout)
}

// Click toggle shared by pointer taps and keyboard activation (Enter/Space on
// the button synthesizes a click with no pointerdown, so the suppression flag
// is naturally false there — keyboard expand/collapse is preserved).
function onPillClick() {
  if (suppressNextClick) {
    suppressNextClick = false
    return
  }
  expanded.value = !expanded.value
}

// Re-fit the remembered position when the window resizes or crosses the
// phone/desktop tier: band re-normalization re-snaps and re-clamps it into
// the new viewport without ever landing out of bounds.
function onWindowResize() {
  applyStoredPosition()
}

watch(visible, (v) => {
  if (v) nextTick(applyStoredPosition)
  // Dock hidden (no downloads / models): retract the chat lane immediately —
  // the component itself stays mounted, so the unmount reset below never fires.
  else dockLane.value = 'none'
})

// Self-healing re-fit: the resize event can race the media-query relayout (a
// tier switch may be applied with the pill's previous box still measured, a
// few px off), and the pill box also changes whenever counter segments come
// and go. The dock-space ResizeObserver publishes every such box change as
// `dockWidth`, so re-fit on it — the side/band math is idempotent.
watch(dockWidth, () => {
  if (visible.value) applyStoredPosition()
})

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

// ─── Nudge wiring ─────────────────────────────────────────────────

// Chat-driven model loads/unloads and dock-row unload actions nudge the
// counter for immediate feedback instead of waiting up to 1s for the next
// poll. Vue stops this watcher automatically when the component unmounts.
watch(dockNudgeCounter, () => {
  poll()
})

// ─── Unload ───────────────────────────────────────────────────────

async function handleUnload(id: string) {
  unloadingId.value = id
  delete unloadErrors[id]
  try {
    await unloadModel(id)
    // Instant feedback: drop the row now, then nudge a poll that reconciles
    // against reality (an unload that silently failed re-appears on the poll)
    loadedModels.value = loadedModels.value.filter(m => m.id !== id)
    nudgeDock()
  } catch (e: any) {
    unloadErrors[id] = e?.message || 'unknown'
  } finally {
    unloadingId.value = ''
  }
}

// ─── Labels ───────────────────────────────────────────────────────

function statusLabel(status: string): string {
  const map: Record<string, string> = {
    fetching: t('dl.fetching'),
    downloading: t('dl.downloading'),
    paused: t('dl.paused'),
    extracting: t('dl.extracting'),
    done: '',
    error: t('dl.error'),
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
    downloading: t('dl.downloading'),
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
  window.addEventListener('resize', onWindowResize, { passive: true })
})

onUnmounted(() => {
  stopPolling()
  window.removeEventListener('resize', onWindowResize)
  // Safety net: never leave document/window listeners attached after unmount
  document.removeEventListener('click', onDocClick)
  document.removeEventListener('keydown', onDocKeydown)
  window.removeEventListener('pointermove', onWindowPointerMove)
  window.removeEventListener('pointerup', onWindowPointerUp)
  window.removeEventListener('pointercancel', onWindowPointerCancel)
  // Same reset pattern as the dock-space reserve: a fresh mount starts from
  // the default side until a stored/dragged position republishes it.
  dockSide.value = 'right'
  dockLane.value = 'none'
})
</script>

<style scoped>
/* Pure positioning container: the pill is the only in-flow content, so the
   root size (and thus --dock-reserve) never changes when the popover opens.
   `bottom: 29px` vertically centers the pill on the chat page's input row:
   input-area bottom padding 24 + (row height 42 - pill height 32) / 2 = 29,
   which puts the pill's center on the send button's center. Keep this value
   in sync with DOCK_ANCHOR_BOTTOM_DESKTOP in lib/dockPosition.ts and
   DOCK_BOTTOM_OFFSET in lib/dockSpace.ts; it is desktop-only — the phone
   breakpoint overrides `bottom` from the phone composer metrics in the media
   query below (DOCK_ANCHOR_BOTTOM_MOBILE there). On mobile the bottom tab bar
   takes its own fixed band, so the pill rides above it via the global
   --mobile-nav-height (0px on desktop — the calc is a no-op there).

   This anchor is also the reference frame of the draggable capsule: with no
   transform the pill sits exactly at this legacy spot; a dragged/restored
   position is a translate() on top of it (lib/dockPosition.ts). */
.task-dock {
  position: fixed;
  right: 16px;
  bottom: calc(29px + var(--mobile-nav-height, 0px));
  z-index: 50;
  /* One-shot glide when a released capsule snaps onto its edge spot or a
     restored position re-fits after a resize; disabled while dragging so the
     pill tracks the pointer 1:1 (see .task-dock--dragging). */
  transition: transform 0.2s ease;
}

/* Raw-follow mode during an active pointer drag: no transform transition,
   compositor hint for jitter-free tracking (removed on release). */
.task-dock--dragging {
  transition: none;
  will-change: transform;
}

/* Full card as an absolute overlay landing on the pill's spot: `bottom: 0`
   aligns the card's bottom edge with the (now hidden) pill's bottom edge, so
   the card reads as the pill morphing into its expanded form. Absolute
   positioning keeps expansion layout-neutral, so reserved pages never jump
   when the popover shows/hides. Width caps at the viewport (minus the shared
   16px gutters) so the card stays fully visible on narrow windows whatever
   the capsule's side. */
.dock-popover {
  position: absolute;
  bottom: 0;
  right: 0;
  width: min(300px, calc(100vw - 32px));
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

/* Mirrored anchoring for a LEFT-hugging capsule: the card opens toward the
   window interior instead of hanging off the left edge of the screen. */
.task-dock--left .dock-popover {
  left: 0;
  right: auto;
}

/* Capsule in the upper half of the viewport: bottom-anchored growth would
   overflow the window top (max-height 50vh), so anchor the card's top edge to
   the pill's top edge and grow downward instead (flipped by the component's
   cardBelow flag). */
.dock-popover--below {
  bottom: auto;
  top: 0;
}

/* Persistent compact pill: collapsed-state summary of downloads and models,
   and the drag handle of the capsule. touch-action: none makes the whole pill
   a drag surface on touch (no page scroll/refresh gestures start from it);
   user-select: none keeps a mouse drag from rubber-band selecting text. */
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
  touch-action: none;
  user-select: none;
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
  transition: opacity 0.15s;
}

/* Row-level pending state: dim the row while its unload request is in flight
   (the row status slot shows the 卸载中 label for the same window) */
.dock-model-item--unloading {
  opacity: 0.5;
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

/* ─── Phone (<=767px): the pill grows to a 44px touch target (it is the only
       dock control visible without expanding) while keeping its band above the
       bottom tab bar. Horizontal padding tightens to 8px so a single-segment
       pill stays ~40px wide (2x8 padding + 12 icon + 4 gap + 8 count digit).
       The pill's width is content-driven (one segment per active counter), so
       consumers must not hardcode it: the pill's measured width is published
       as --dock-width (lib/dockSpace, same ResizeObserver as --dock-reserve)
       and Chat.vue's phone right-padding lane is computed from it, keeping the
       send button 8px clear of the pill's left edge at any segment count. The
       bottom offset is recomputed from the phone composer metrics instead of
       the desktop-tuned 29px, which rode
       high enough to cover the composer's top: Chat.vue phone input-area
       padding-bottom 10 + (row 44 - pill 44) / 2 = 10, and
       var(--mobile-nav-height) keeps the band above the bottom tab bar. The
       popover caps at the viewport width, and in-card controls get
       touch-sized hit areas. The measured --dock-reserve tracks the pill's new
       height automatically (ResizeObserver on offsetHeight). ─── */
@media (max-width: 767px) {
  .task-dock {
    bottom: calc(10px + var(--mobile-nav-height, 0px));
  }

  .dock-pill {
    height: 44px;
    padding: 0 8px;
  }

  .dock-toggle {
    width: 32px;
    height: 32px;
  }

  .dock-unload-btn {
    min-height: 44px;
    padding: 8px 14px;
  }
}
</style>
