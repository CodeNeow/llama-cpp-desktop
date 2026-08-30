<template>
  <div class="page page-fixed">
    <div class="sticky-top">
      <div class="page-header">
        <h1 class="page-title">{{ t('api.title') }}</h1>
        <p class="page-subtitle">{{ t('api.subtitle') }}</p>
      </div>

      <!-- Top action bar: status light + status text + URL + button group + params popover + available models -->
      <section class="status-card toolbar">
        <div class="toolbar-row">
          <div class="status-indicator">
            <span class="status-dot" :class="serverRunning ? 'running' : 'stopped'"></span>
            <span class="status-text">{{ serverRunning ? t('api.running') : t('api.stopped') }}</span>
            <span v-if="serverRunning" class="status-url">http://{{ cfg.host }}:{{ cfg.port }}</span>
            <span v-if="serverRunning && status.uptimeSeconds > 0" class="toolbar-uptime">
              <span class="uptime-value">{{ formatUptime(status.uptimeSeconds, locale) }}</span>
              <span class="uptime-label">{{ t('monitor.uptimeLabel') }}</span>
            </span>
          </div>
          <div class="btn-group">
            <button class="server-btn btn-start" :disabled="serverRunning || busy" @click="doStart">
              {{ t('api.startServer') }}
            </button>
            <button class="server-btn btn-stop" :disabled="!serverRunning || busy" @click="doStop">
              {{ t('api.stopServer') }}
            </button>
            <button class="server-btn btn-restart" :disabled="!serverRunning || busy" @click="doRestart">
              {{ t('api.restart') }}
            </button>
            <button class="icon-btn" @click.stop="showCfg = !showCfg" :aria-expanded="showCfg" :title="t('api.settings')" type="button">
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
                <circle cx="12" cy="12" r="3"/>
                <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z"/>
              </svg>
            </button>
          </div>
        </div>

        <!-- Server parameters popover -->
        <div v-if="showCfg" class="cfg-popover" @click.stop>
          <div class="cfg-popover-title">{{ t('api.settings') }}</div>
          <div class="cfg-item">
            <label>{{ t('api.cfgPort') }}</label>
            <input v-model.number="cfg.port" type="number" min="1024" max="65535" step="1" class="cfg-input cfg-num" :disabled="serverRunning" />
          </div>
          <div class="cfg-item">
            <label>{{ t('api.cfgMaxModels') }}</label>
            <input v-model.number="cfg.maxModels" type="number" min="1" step="1" class="cfg-input cfg-num" :disabled="serverRunning" />
          </div>
          <div class="cfg-item">
            <label>{{ t('api.cfgCacheRam') }}</label>
            <input v-model.number="cfg.cacheRam" type="number" min="0" step="1" class="cfg-input cfg-num" :disabled="serverRunning" placeholder="8192" />
          </div>
          <div v-if="serverRunning" class="cfg-locked-hint">{{ t('api.cfgLockedHint') }}</div>
        </div>

        <!-- Available models (second row) -->
        <div class="toolbar-models">
          <span class="toolbar-models-title">{{ t('api.modelsTitle', { n: modelCount }) }}</span>
          <div class="model-tags">
            <span v-for="m in availableModels" :key="m" class="model-tag">{{ m }}</span>
          </div>
          <span v-if="modelCount === 0" class="empty-hint">
            {{ t('api.emptyHint') }}
            <button class="empty-cta" @click="goDownloads">{{ t('action.gotoDownloads') }}</button>
          </span>
        </div>
      </section>
    </div>

    <!-- Main area: scrollable band wrapping the monitor grid — stretches to
         fill tall windows, scrolls once the window is too short -->
    <div class="page-scroll">
      <div class="monitor-grid">
      <!-- Left column: service log console (dark console look in both themes) -->
      <section class="log-panel">
        <div class="panel-header">
          <span class="panel-title">{{ t('api.logTitle') }}</span>
          <button v-if="serverLog.length" class="log-clear-btn" @click="clearLog">{{ t('api.logClear') }}</button>
        </div>
        <div v-if="serverLog.length" class="console-log" ref="logEl">
          <div v-for="(line, i) in serverLog" :key="i" class="console-line">{{ line }}</div>
        </div>
        <div v-else class="console-empty">{{ t('api.logEmpty') }}</div>
      </section>

      <!-- Right column: token speed card -->
      <div class="monitor-side">
        <!-- Token speed -->
        <section class="info-section monitor-card">
          <div class="token-card-head">
            <h2 class="section-title">
              <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                <rect x="2" y="2" width="20" height="8" rx="2" ry="2"/><rect x="2" y="14" width="20" height="8" rx="2" ry="2"/><line x1="6" y1="6" x2="6.01" y2="6"/><line x1="6" y1="18" x2="6.01" y2="18"/>
              </svg>
              {{ t('api.tokenSpeed') }}
            </h2>
            <div class="token-uptime" :class="{ 'tps-ghost': !status.serverRunning }" :aria-hidden="!status.serverRunning">
              <span class="uptime-value">{{ formatUptime(status.uptimeSeconds, locale) }}</span>
              <span class="uptime-label">{{ t('monitor.uptimeLabel') }}</span>
            </div>
          </div>
          <div class="tps-body">
            <!-- Both states render the same content: the stopped state keeps the
                 metrics invisible (visibility, not display) so card heights never
                 shift when the service starts or stops; the placeholder overlays -->
            <div class="tps-cards" :class="{ 'tps-ghost': !status.serverRunning }" :aria-hidden="!status.serverRunning">
              <div class="tps-card">
                <div class="tps-card-info">
                  <span class="tps-card-name">{{ t('monitor.promptSpeed') }}</span>
                  <span class="tps-card-sub">{{ t('monitor.promptSub') }}</span>
                </div>
                <div class="tps-card-value">
                  <span class="tps-value">{{ promptTpsText }}</span>
                  <span class="tps-label">tokens/s</span>
                </div>
              </div>
              <div class="tps-card">
                <div class="tps-card-info">
                  <span class="tps-card-name">{{ t('monitor.decodeSpeed') }}</span>
                  <span class="tps-card-sub">{{ t('monitor.decodeSub') }}</span>
                </div>
                <div class="tps-card-value">
                  <span class="tps-value">{{ decodeTpsText }}</span>
                  <span class="tps-label">tokens/s</span>
                </div>
              </div>
            </div>
            <div class="tps-chart" :class="{ 'tps-ghost': !status.serverRunning }" :aria-hidden="!status.serverRunning">
              <svg :viewBox="`0 0 ${chartWidth} ${chartHeight}`" preserveAspectRatio="none">
                <line class="tps-axis" x1="0" :y1="chartHeight - 2" :x2="chartWidth" :y2="chartHeight - 2" />
                <polyline :points="decodePoints" />
              </svg>
              <div class="tps-chart-meta">
                <span class="tps-chart-label">{{ t('monitor.chartLabel', { n: decodeHistory.length }) }}</span>
              </div>
            </div>
            <p class="tps-footnote" :class="{ 'tps-ghost': !status.serverRunning }" :aria-hidden="!status.serverRunning">{{ t('monitor.footnote') }}</p>
            <div v-if="!status.serverRunning" class="tps-placeholder">{{ t('monitor.uptimePlaceholder') }}</div>
          </div>
        </section>
      </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, onUnmounted, nextTick, watch } from 'vue'
import { useRouter } from 'vue-router'
import { getMonitorStatus, getModels, getServerConfig, getServerLogsSince, getServerStatus, refreshModels, saveServerConfig, startServer, stopServer } from '../wails'
import { applyFullLogFetch, appendLogEntries, type ServerLogEntry } from '../lib/serverLog'
import { appendHistory, chartPoints, formatPromptTps, formatUptime, type MonitorStatus } from '../lib/monitor'
import { locale, t } from '../lib/i18n'

const serverRunning = ref(false)
const serverLog = ref<string[]>([])
// Incremental log fetch cursor: the backend ring seq the next poll continues
// from (0 = everything retained). Non-reactive on purpose — polling must not
// trigger renders by itself; only the lines array does.
let logCursor = 0
const logEl = ref<HTMLElement | null>(null)
// Disable all buttons during start/stop/restart to prevent double-clicks
const busy = ref(false)

const router = useRouter()

/** Empty-model hint CTA: jump to the merged Models page's download tab */
function goDownloads() {
  router.push('/models/download')
}

const cfg = reactive({
  host: '127.0.0.1',
  accessMode: 'local', // Access scope comes from backend config (changed on Settings page); here it is only passed through on save
  apiKey: '', // Optional API key comes from backend config (changed on Settings page); passed through on save only — never edited here, so saving this page cannot wipe it
  deviceId: '', // Serving-GPU UUID comes from backend config (changed on Settings page); passed through on save only — never edited here, so saving this page cannot wipe it
  port: 8080,
  maxModels: 1,
  cacheRam: 8192,
})

const availableModels = ref<string[]>([])
const modelCount = ref(0)

/** Whether the server parameters panel is expanded */
const showCfg = ref(false)

// ─── Monitoring (merged from Monitor.vue, 1s polling): inference metrics (TPS / uptime / running state); the same tick also drives the incremental log poll (pollServerLog) ───
const status = ref<MonitorStatus>({
  cpuPercent: 0,
  memUsed: 0,
  memTotal: 0,
  gpus: [],
  serverRunning: false,
  promptTps: 0,
  decodeTps: 0,
  uptimeSeconds: 0,
})

// Decode speed line chart history: appended on 1s polling, keeps latest 60 samples (appendHistory default cap=60)
const decodeHistory = ref<number[]>([])
const chartWidth = 560
const chartHeight = 120

let pollTimer: ReturnType<typeof setInterval> | null = null

const promptTpsText = computed(() => formatPromptTps(status.value.promptTps))

const decodeTpsText = computed(() => status.value.decodeTps.toFixed(1))

const decodePoints = computed(() => chartPoints(decodeHistory.value, chartWidth, chartHeight))

async function fetchMonitorStatus() {
  try {
    const s = await getMonitorStatus()
    status.value = s
    // Polling linkage also handles llama-server being killed externally: backend sets false, frontend corrects button state and param lock within 1s
    serverRunning.value = s.serverRunning
    decodeHistory.value = appendHistory(decodeHistory.value, s.decodeTps)
  } catch {
    // Polling failure: silently keep previous data, don't disrupt monitor display
  }
}

function startPolling() {
  stopPolling()
  pollTimer = setInterval(() => {
    fetchMonitorStatus()
    pollServerLog()
  }, 1000)
  fetchMonitorStatus()
}

function stopPolling() {
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
}

watch(serverLog, async () => {
  await nextTick()
  if (logEl.value) logEl.value.scrollTop = logEl.value.scrollHeight
})

// Real-time config save (#13): debounce 500ms silent save; backend rejects illegal values (e.g. 0.0.0.0) as fallback
let configLoaded = false
let saveTimer: ReturnType<typeof setTimeout> | null = null
watch(cfg, () => {
  if (!configLoaded) return
  if (saveTimer) clearTimeout(saveTimer)
  saveTimer = setTimeout(() => {
    saveServerConfig({
      host: cfg.host,
      accessMode: cfg.accessMode,
      apiKey: cfg.apiKey,
      deviceId: cfg.deviceId,
      port: cfg.port,
      maxModels: cfg.maxModels,
      cacheRam: cfg.cacheRam,
    }).catch((e) => {
      serverLog.value.push(t('api.saveFailed', { msg: e instanceof Error ? e.message : String(e) }))
    })
  }, 500)
})

/** Close the params panel when clicking outside it */
function onDocClick() {
  showCfg.value = false
}

onUnmounted(() => {
  if (saveTimer) clearTimeout(saveTimer)
  stopPolling()
  document.removeEventListener('click', onDocClick)
})

onMounted(async () => {
  // Monitoring polling: fetch monitor status every 1s (merged into monitor block, period matches original Monitor.vue)
  startPolling()

  // Load server config
  try {
    const scfg = await getServerConfig()
    Object.assign(cfg, scfg)
  } catch {}
// Initial config load's watch callback runs on next flush (configLoaded is still false here,
// so saving is skipped). await nextTick() waits for that flush to complete before enabling
// auto-save, preventing save-on-load.
  await nextTick()
  configLoaded = true

  // Load available models
  try {
    const models = await getModels() as any[]
    if (models) {
      // Show the real llama-server model id (Alias, display casing preserved)
      // so copy-paste from this page always matches the case-sensitive lookup
      availableModels.value = models.map((m: any) => m.alias || m.name)
      modelCount.value = models.length
    }
  } catch {}

  // Check server status
  checkServerStatus()

  document.addEventListener('click', onDocClick)
})

async function checkServerStatus() {
  try {
    const status = await getServerStatus()
    serverRunning.value = status.running
  } catch {}
  // One full fetch re-syncs the view and the cursor: every server start
  // clears the backend ring, so after mount or start/stop/restart actions
  // incremental patching from the old cursor is unreliable.
  await resetLogView()
}

// Full (re)sync of the log view: replaces the local lines with everything the
// backend ring retains (since 0) and adopts the returned cursor.
async function resetLogView() {
  try {
    const full = await getServerLogsSince(0)
    const r = applyFullLogFetch(full.entries, full.next)
    serverLog.value = r.lines
    logCursor = r.cursor
  } catch {}
}

// Incremental log poll: fetch only the lines appended since logCursor and
// append them, so the backend ring is not re-copied and re-rendered on every
// tick. On a poll error, or when the cursor fell out of the backend retention
// window (lines evicted between polls), one full fetch (since 0) replaces the
// view instead.
async function pollServerLog() {
  const requested = logCursor
  let res: { entries: ServerLogEntry[]; next: number } | null = null
  try {
    res = await getServerLogsSince(requested)
  } catch {
    res = null
  }
  // A concurrent resetLogView re-synced the view while the request was in
  // flight — discard the stale result instead of appending duplicates.
  if (logCursor !== requested) return
  if (res && res.next > logCursor) {
    const r = appendLogEntries(serverLog.value, logCursor, res.entries, res.next)
    if (!r.reset) {
      serverLog.value = r.lines
      logCursor = r.cursor
      return
    }
    // Retention-window gap: evicted lines cannot be patched in — refetch all.
  } else if (res) {
    return // nothing new since the last poll
  }
  await resetLogView()
}

// Button tri-state driven by serverRunning + busy: start (stopped enabled), stop/restart (running enabled);
// busy=true disables all during execution. Status is refreshed asynchronously by checkServerStatus;
// no optimistic local flipping (#14)
async function doStart() {
  if (busy.value || serverRunning.value) return
  busy.value = true
  try {
    await refreshModels() // Force a model rescan before start so presets use the latest model list (#18)
    await startServer()
  } catch (e) {
    serverLog.value.push(t('api.toggleFailed', { msg: e instanceof Error ? e.message : String(e) }))
  } finally {
    busy.value = false
    setTimeout(checkServerStatus, 500)
  }
}

async function doStop() {
  if (busy.value || !serverRunning.value) return
  busy.value = true
  try {
    await stopServer()
  } catch (e) {
    serverLog.value.push(t('api.toggleFailed', { msg: e instanceof Error ? e.message : String(e) }))
  } finally {
    busy.value = false
    setTimeout(checkServerStatus, 500)
  }
}

// Restart = sequential stop → start (busy prevents double-clicks during execution)
async function doRestart() {
  if (busy.value || !serverRunning.value) return
  busy.value = true
  try {
    await stopServer()
    await startServer()
  } catch (e) {
    serverLog.value.push(t('api.toggleFailed', { msg: e instanceof Error ? e.message : String(e) }))
  } finally {
    busy.value = false
    setTimeout(checkServerStatus, 500)
  }
}

// Clear log: only clears the frontend display array; the backend ring buffer
// is preserved and its cursor kept, so cleared lines do not replay while new
// lines keep streaming in with the next polls.
function clearLog() {
  serverLog.value = []
}
</script>

<style scoped>
.page {
  /* No top padding: header flush with content top, title aligns with sidebar logo (see global.css .page-header).
     24px bottom band matches the chat page's input-area bottom padding; the floating
     TaskDock pill is cleared by the right column's internal --dock-reserve, not here.
     Width: raised above the global 1280px cap so a maximized window has nearly no
     side margins (chat-page feel); 1920px still bounds ultra-wide monitors. */
  padding: 0 48px 24px;
  max-width: 1920px;
}

/* Fixed-viewport layout (see .page-fixed in global.css): the header + toolbar
   band stays pinned and never compresses; the monitor grid below flexes to
   fill the remaining viewport height */
.page-fixed .sticky-top {
  flex-shrink: 0;
}

.page-header {
  /* Use padding instead of margin: header background covers this gap so content scrolls without leaving a seam */
  padding-bottom: 28px;
}

.page-title {
  font-size: 28px;
  font-weight: 700;
  color: var(--text-primary);
  margin: 0 0 4px;
  line-height: 1.2;
}

.page-subtitle {
  font-size: 14px;
  color: var(--text-dim);
  margin: 0;
}

/* ─── Top action bar ─── */
.toolbar {
  display: flex;
  flex-direction: column;
  gap: 0;
  padding: 16px 24px;
  margin-bottom: 20px;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 14px;
  position: relative; /* Positioning context for the params popover */
}

.toolbar-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  /* Elastic: status text / URL / buttons reflow onto extra rows on narrow
     windows instead of squeezing or clipping */
  flex-wrap: wrap;
}

.status-indicator {
  display: flex;
  align-items: center;
  gap: 10px;
}

.status-dot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  flex-shrink: 0;
}

.status-dot.running {
  background: #22c55e;
  box-shadow: 0 0 8px rgba(34, 197, 94, 0.5);
  animation: pulse 2s infinite;
}

.status-dot.stopped {
  background: rgba(255,255,255,0.2);
}

@keyframes pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.5; }
}

.status-text {
  font-size: 15px;
  font-weight: 600;
  color: var(--text-primary);
}

.status-url {
  font-size: 12px;
  color: var(--accent-light);
  font-family: var(--font-mono);
  background: var(--active-bg);
  padding: 2px 8px;
  border-radius: 4px;
}

/* Uptime moved into the top status bar (so info survives the removal of the old "Inference Service" block) */
.toolbar-uptime {
  display: flex;
  align-items: baseline;
  gap: 6px;
  margin-left: 4px;
}

.btn-group {
  display: flex;
  gap: 10px;
}

.server-btn {
  padding: 10px 24px;
  border: none;
  border-radius: 10px;
  font-size: 14px;
  font-weight: 600;
  cursor: pointer;
  transition: opacity 0.15s;
  color: #fff;
}

/* Disabled state follows the cfg-input dimming convention; the desaturation
   filter keeps the colored start/stop/restart buttons recognizably disabled */
.server-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
  filter: grayscale(0.6) brightness(0.92);
}

.btn-start { background: #22c55e; }
.btn-start:hover:not(:disabled) { opacity: 0.85; }
.btn-stop { background: #ef4444; }
.btn-stop:hover:not(:disabled) { opacity: 0.85; }
.btn-restart { background: var(--accent); }
.btn-restart:hover:not(:disabled) { opacity: 0.85; }

.icon-btn {
  width: 40px;
  height: 40px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 10px;
  color: var(--text-secondary);
  cursor: pointer;
  transition: all 0.2s;
  flex-shrink: 0;
}

.icon-btn:hover {
  background: var(--hover-bg);
  color: var(--text-primary);
}

.icon-btn[aria-expanded='true'] {
  background: var(--hover-bg);
  color: var(--text-primary);
}

/* ─── Server parameters popover ─── */
.cfg-popover {
  position: absolute;
  right: 0;
  top: calc(100% + 8px);
  z-index: 30;
  width: 300px;
  background: var(--bg-secondary);
  border: 1px solid var(--border);
  border-radius: 12px;
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.2);
  padding: 16px;
}

.cfg-popover-title {
  font-size: 13px;
  font-weight: 600;
  color: var(--text-primary);
  margin-bottom: 10px;
}

.cfg-item {
  display: flex;
  flex-direction: column;
  gap: 4px;
  margin-bottom: 8px;
}

.cfg-item:last-of-type {
  margin-bottom: 0;
}

.cfg-item label {
  font-size: 12px;
  color: var(--text-dim);
  font-weight: 500;
}

.cfg-input {
  padding: 8px 12px;
  background: var(--bg-primary);
  border: 1px solid var(--border);
  border-radius: 8px;
  color: var(--text-primary);
  font-size: 13px;
  outline: none;
  transition: border-color 0.15s;
}

.cfg-input:focus { border-color: var(--accent); }
.cfg-input:disabled { opacity: 0.5; cursor: not-allowed; }

.cfg-num { width: 100%; }

.cfg-locked-hint {
  font-size: 11px;
  color: var(--text-dim);
  margin-top: 10px;
  text-align: center;
}

/* ─── Available models (toolbar second row) ─── */
.toolbar-models {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  margin-top: 10px;
  padding-top: 10px;
  border-top: 1px solid var(--border-light);
}

.toolbar-models-title {
  font-size: 12px;
  color: var(--text-dim);
  font-weight: 500;
  flex-shrink: 0;
  margin-top: 3px;
}

.toolbar-models .model-tags {
  flex-wrap: wrap;
  gap: 4px;
  max-height: 64px;
  overflow-y: auto;
}

.toolbar-models .model-tag {
  font-size: 11px;
  padding: 2px 10px;
}

.toolbar-models .empty-hint {
  font-size: 12px;
}

/* ─── Scrollable content band ─── */
.page-scroll {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  /* TaskDock pill clearance while the band scrolls (--dock-reserve is bound
     globally by App.vue; 0 while hidden) */
  padding-bottom: calc(24px + var(--dock-reserve, 0px));
}

/* ─── Main area two-column: left log console + right inference-monitor card ─── */
.monitor-grid {
  display: grid;
  /* minmax(0, Nfr): floor the tracks at 0 (not auto) so long unbreakable
     strings cannot widen a column past its share; the narrow-viewport block
     at the end of this style sheet collapses the grid to one column */
  grid-template-columns: minmax(0, 6fr) minmax(0, 4fr);
  /* Rows share the track height exactly: startup log lines are unbounded, so
     minmax(0, 1fr) plus min-height: 0 on children locks the row height and
     excess logs scroll inside the console */
  grid-template-rows: minmax(0, 1fr);
  gap: 16px;
  /* Stretch to fill the scroll band on tall windows, but never shrink below
     MIN-HEIGHT: smaller windows scroll the band instead of clipping panels */
  flex: 1;
  min-height: 460px;
}

/* ─── Left column: log panel ─── */
.log-panel {
  height: 100%;
  /* Release the grid child default min sizes so oversized content shrinks
     and scrolls internally instead of blowing out the track */
  min-height: 0;
  min-width: 0;
  display: flex;
  flex-direction: column;
  padding: 18px 20px;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 14px;
}

.panel-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 10px;
}

.panel-title {
  font-size: 15px;
  font-weight: 600;
  color: var(--text-secondary);
}

.log-clear-btn {
  border: none;
  background: transparent;
  color: var(--text-dim);
  font-size: 12px;
  cursor: pointer;
  padding: 2px 6px;
  border-radius: 4px;
  transition: color 0.15s, background 0.15s;
}

.log-clear-btn:hover {
  color: var(--accent-light);
  background: var(--active-bg);
}

  /* Dark console: fixed dark background in both themes for a terminal look */
.console-log {
  flex: 1;
  /* Release the flex child default min-height:auto so any number of log lines shrinks and scrolls within the panel */
  min-height: 0;
  overflow-y: auto;
  /* Long log lines scroll horizontally instead of wrapping mid-word (.console-line is pre) */
  overflow-x: auto;
  background: #0b0b10;
  border-radius: 8px;
  padding: 10px 14px;
  font-family: var(--font-mono);
  font-size: 11px;
}

.console-line {
  color: rgba(255,255,255,0.55);
  padding: 1px 0;
  /* Preserve the line's own layout exactly; horizontal scroll lives on .console-log */
  white-space: pre;
}

.console-empty {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  background: #0b0b10;
  border-radius: 8px;
  color: rgba(255,255,255,0.6);
  font-size: 12px;
}

/* ─── Right column: token speed card ─── */
.monitor-side {
  display: flex;
  flex-direction: column;
  gap: 12px;
  height: 100%;
  min-height: 0;
  min-width: 0;
  /* Fixed, scroll-free column: the card stretches to fill the column height,
     so no scrollbar appears while the window stays above the grid's floor */
}

.monitor-card {
  margin-bottom: 0;
  flex: 1 1 auto;
  min-height: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  padding: 16px 20px;
}

.monitor-card .section-title {
  flex-shrink: 0;
}

/* ─── Models ─── */
.model-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.model-tag {
  padding: 4px 12px;
  background: var(--active-bg);
  color: var(--accent-light);
  border: 1px solid rgba(99,102,241,0.2);
  border-radius: 20px;
  font-size: 12px;
  font-weight: 500;
}

.empty-hint {
  font-size: 13px;
  color: var(--text-dim);
}

/* Inline CTA next to the empty-models hint */
.empty-cta {
  margin-left: 8px;
  padding: 2px 10px;
  background: var(--active-bg);
  color: var(--accent-light);
  border: none;
  border-radius: var(--radius-sm);
  font-size: 12px;
  font-weight: 500;
  cursor: pointer;
  transition: background 0.15s;
}

.empty-cta:hover {
  background: var(--accent-glow);
}

/* ─── Monitor block (ported from Monitor.vue) ─── */

/* ─── Info section card ─── */
.info-section {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 14px;
  transition: border-color 0.2s;
}

.info-section:hover {
  border-color: var(--overlay-10);
}

/* ─── Token card header (includes uptime subtext) ─── */
.token-card-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 14px;
  flex-shrink: 0;
}

.token-card-head .section-title {
  margin: 0;
}

.token-uptime {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  flex-shrink: 0;
}

.uptime-value {
  font-size: 14px;
  font-weight: 600;
  color: var(--text-secondary);
}

.uptime-label,
.tps-label {
  font-size: 11px;
  color: var(--text-dim);
}

.tps-body {
  position: relative;
  flex: 1 1 auto;
  min-height: 0;
  display: flex;
  flex-direction: column;
  overflow-y: auto;
}

/* visibility (not display) keeps the stopped state reserving exactly the
   running state's space, so card heights never shift on service start/stop */
.tps-ghost {
  visibility: hidden;
}

.tps-placeholder {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 13px;
  color: var(--text-dim);
}

/* ─── TPS metric cards ─── */
.tps-cards {
  /* Elastic pair: the two mini-cards sit side by side while both fit, and the
     second wraps under the first on narrow columns (auto-fit, no media query).
     Natural height (flex: 0 0 auto) + top alignment so the cards never get
     forced into an equal-height slice that cramps/clips on short windows */
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(min(100%, 190px), 1fr));
  gap: 16px;
  flex: 0 0 auto;
  min-height: 0;
  align-content: start;
}

.tps-card {
  display: flex;
  flex-direction: column;
  padding: 16px;
  background: var(--surface);
  border: 1px solid var(--border-light);
  border-radius: var(--radius-md);
}

.tps-card-name {
  font-size: 14px;
  font-weight: 600;
  color: var(--text-secondary);
}

/* Name + subtitle grouped so tall windows can lay them out beside the value */
.tps-card-info {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.tps-card-sub {
  font-size: 11px;
  color: var(--text-dim);
}

.tps-card-value {
  display: flex;
  align-items: baseline;
  gap: 8px;
  margin-top: 10px;
}

.tps-value {
  font-size: 28px;
  font-weight: 700;
  color: var(--accent-light);
  line-height: 1.1;
}

.tps-footnote {
  flex-shrink: 0;
  margin: 12px 0 0;
  font-size: 12px;
  color: var(--text-dim);
}

/* ─── TPS chart ─── */
.tps-chart {
  margin-top: 16px;
}

.tps-chart svg {
  display: block;
  width: 100%;
  height: 90px;
}

.tps-chart polyline {
  fill: none;
  stroke: var(--accent);
  stroke-width: 2;
  stroke-linecap: round;
  stroke-linejoin: round;
}

.tps-axis {
  stroke: var(--text-muted);
  stroke-width: 1;
  stroke-dasharray: 3 3;
}

.tps-chart-meta {
  display: flex;
  justify-content: flex-end;
  margin-top: 4px;
}

.tps-chart-label {
  font-size: 11px;
  color: var(--text-muted);
}

/* ─── Compact mode (viewport height <= 799px): keep the fixed, scroll-free
   layout but compress secondary elements so everything fits without clipping
   down to the 900x600 minimum window. Fixed values only — nothing scales
   continuously with the window size. */
@media (max-height: 799px) {
  .page-subtitle { display: none; }
  .page-header { padding-bottom: 14px; }
  .toolbar { padding: 10px 16px; margin-bottom: 12px; }
  .toolbar-models { margin-top: 6px; padding-top: 6px; }
  .monitor-grid { gap: 12px; }
  .monitor-side { gap: 8px; }
  .monitor-card { padding: 10px 14px; }
  .token-card-head { margin-bottom: 8px; }
  .tps-card { padding: 10px 12px; }
  .tps-card-sub { display: none; }
  .tps-card-value { margin-top: 6px; }
  .tps-footnote { display: none; }
  .tps-chart { display: none; }
}

/* ─── Narrow viewports (< 1100px): collapse to a single column ──────────────
   Keeping both panels side by side would cram them into unusable slivers.
   The log console takes a bounded proportional band, the monitor card flows
   below it, and the shared .page-scroll band owns scrolling + dock clearance. */
@media (max-width: 1099px) {
  .monitor-grid {
    grid-template-columns: minmax(0, 1fr);
    grid-template-rows: none;
    align-content: start;
    /* Content-driven height: the band scrolls, the grid does not stretch */
    flex: none;
    min-height: 0;
  }

  /* Auto-sized rows cannot resolve height:100% — give the console an
     explicit proportional band so its internal log scroll still engages */
  .log-panel {
    height: clamp(200px, 40vh, 380px);
  }

  /* Content-driven column: cards stack and size to their content */
  .monitor-side {
    height: auto;
  }
}
</style>
