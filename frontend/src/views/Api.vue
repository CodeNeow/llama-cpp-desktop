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
          <div v-if="modelCount === 0" class="empty-hint">{{ t('api.emptyHint') }}</div>
        </div>
      </section>
    </div>

    <!-- Main area two-column: left log console + right monitor cards -->
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

      <!-- Right column: two cards top to bottom -->
      <div class="monitor-side">
        <!-- a. System monitor: CPU / Memory / GPU -->
        <section class="info-section monitor-card">
          <h2 class="section-title">
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <rect x="4" y="4" width="16" height="16" rx="2" ry="2"/><rect x="9" y="9" width="6" height="6"/><line x1="9" y1="1" x2="9" y2="4"/><line x1="15" y1="1" x2="15" y2="4"/><line x1="9" y1="20" x2="9" y2="23"/><line x1="15" y1="20" x2="15" y2="23"/><line x1="20" y1="9" x2="23" y2="9"/><line x1="20" y1="14" x2="23" y2="14"/><line x1="1" y1="9" x2="4" y2="9"/><line x1="1" y1="14" x2="4" y2="14"/>
            </svg>
            {{ t('api.sysMonitor') }}
          </h2>

          <div class="metric-block">
            <div class="metric-head">
              <span class="metric-name">{{ t('monitor.cpu') }}</span>
            </div>
            <div class="usage-bar-wrapper">
              <div class="usage-bar">
                <div class="usage-fill" :style="{ width: cpuPercent + '%' }"></div>
              </div>
              <span class="usage-text">{{ cpuPercent }}%</span>
            </div>
          </div>

          <div class="metric-block">
            <div class="metric-head">
              <span class="metric-name">{{ t('monitor.memory') }}</span>
            </div>
            <div class="usage-bar-wrapper">
              <div class="usage-bar">
                <div class="usage-fill" :style="{ width: memPercent + '%' }"></div>
              </div>
              <span class="usage-text">{{ memPercent }}%</span>
            </div>
            <div class="metric-sub">
              <span>{{ t('monitor.memUsed', { n: memText(status.memUsed) }) }}</span>
              <span class="metric-divider">/</span>
              <span>{{ t('monitor.memTotal', { n: memText(status.memTotal) }) }}</span>
          </div>
        </div>

        <!-- GPU (formerly separate card, now merged into system monitor) -->
        <div class="metric-block">
          <div class="metric-head">
            <span class="metric-name">{{ t('monitor.gpu') }}</span>
          </div>
          <div v-if="status.gpus?.length">
            <div v-for="gpu in status.gpus" :key="gpu.index" class="gpu-row">
              <div class="gpu-head">
                <span class="gpu-name">{{ gpu.name }}</span>
                <span class="gpu-util">{{ gpu.utilPercent }}%</span>
              </div>
              <div class="usage-bar-wrapper">
                <div class="usage-bar">
                  <div class="usage-fill" :style="{ width: gpu.utilPercent + '%' }"></div>
                </div>
                <span class="usage-text">{{ gpu.utilPercent }}%</span>
              </div>
              <div class="gpu-mem">{{ t('monitor.gpuMem', { used: memText(gpu.memUsed), total: memText(gpu.memTotal) }) }}</div>
            </div>
          </div>
          <div v-else class="info-empty">{{ t('monitor.noGpu') }}</div>
        </div>
      </section>

        <!-- c. Token speed -->
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
                <span class="tps-card-name">{{ t('monitor.promptSpeed') }}</span>
                <span class="tps-card-sub">{{ t('monitor.promptSub') }}</span>
                <div class="tps-card-value">
                  <span class="tps-value">{{ promptTpsText }}</span>
                  <span class="tps-label">tokens/s</span>
                </div>
              </div>
              <div class="tps-card">
                <span class="tps-card-name">{{ t('monitor.decodeSpeed') }}</span>
                <span class="tps-card-sub">{{ t('monitor.decodeSub') }}</span>
                <div class="tps-card-value">
                  <span class="tps-value">{{ decodeTpsText }}</span>
                  <span class="tps-label">tokens/s</span>
                </div>
              </div>
            </div>
            <p class="tps-footnote" :class="{ 'tps-ghost': !status.serverRunning }" :aria-hidden="!status.serverRunning">{{ t('monitor.footnote') }}</p>
            <div v-if="!status.serverRunning" class="tps-placeholder">{{ t('monitor.uptimePlaceholder') }}</div>
          </div>
        </section>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, onUnmounted, nextTick, watch } from 'vue'
import { getMonitorStatus, getModels, getServerConfig, getServerStatus, refreshModels, saveServerConfig, startServer, stopServer } from '../wails'
import { formatPromptTps, formatUptime, type MonitorStatus } from '../lib/monitor'
import { formatBytes } from '../lib/format'
import { locale, t } from '../lib/i18n'

const serverRunning = ref(false)
const serverLog = ref<string[]>([])
const logEl = ref<HTMLElement | null>(null)
// Disable all buttons during start/stop/restart to prevent double-clicks
const busy = ref(false)

const cfg = reactive({
  host: '127.0.0.1',
  accessMode: 'local', // Access scope comes from backend config (changed on Settings page); here it is only passed through on save
  port: 8080,
  maxModels: 1,
  cacheRam: 8192,
})

const availableModels = ref<string[]>([])
const modelCount = ref(0)

/** Whether the server parameters panel is expanded */
const showCfg = ref(false)

// ─── Monitoring (merged from Monitor.vue, 1s polling): inference metrics and system load, independent from the lightweight getServerStatus polling above ───
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

let pollTimer: ReturnType<typeof setInterval> | null = null

const cpuPercent = computed(() => Math.round(status.value.cpuPercent))

const memPercent = computed(() => {
  if (status.value.memTotal <= 0) return 0
  return Math.round((status.value.memUsed / status.value.memTotal) * 100)
})

const promptTpsText = computed(() => formatPromptTps(status.value.promptTps))

const decodeTpsText = computed(() => status.value.decodeTps.toFixed(1))

// VRAM/memory may be 0 (e.g. data not yet arrived); formatBytes(0) returns empty string, fallback to "0 B"
function memText(bytes: number): string {
  return formatBytes(bytes) || '0 B'
}

async function fetchMonitorStatus() {
  try {
    const s = await getMonitorStatus()
    status.value = s
    // Polling linkage also handles llama-server being killed externally: backend sets false, frontend corrects button state and param lock within 1s
    serverRunning.value = s.serverRunning
  } catch {
    // Polling failure: silently keep previous data, don't disrupt monitor display
  }
}

function startPolling() {
  stopPolling()
  pollTimer = setInterval(fetchMonitorStatus, 1000)
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
    serverLog.value = status.log || []
  } catch {}
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

// Clear log: only clears the frontend display array; backend ring buffer is preserved (checkServerStatus will reload backend logs)
function clearLog() {
  serverLog.value = []
}
</script>

<style scoped>
.page {
  /* No top padding: header flush with content top, title aligns with sidebar logo (see global.css .page-header).
     24px bottom band matches the chat page's input-area bottom padding; the floating
     TaskDock pill is cleared by the right column's internal --dock-reserve, not here */
  padding: 0 48px 24px;
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

/* Disabled state follows the cfg-input dimming convention */
.server-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
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

/* ─── Main area two-column: left log console + right monitor cards ─── */
.monitor-grid {
  display: grid;
  grid-template-columns: 6fr 4fr;
  /* Row height must strictly equal the container height: startup log lines are
     unbounded, so minmax(0, 1fr) plus min-height: 0 on children must lock the
     container height; excess logs scroll inside the console instead of stretching
     the layout and pushing the config block below away */
  grid-template-rows: minmax(0, 1fr);
  gap: 16px;
  /* Fixed-viewport layout: the grid flexes to fill the remaining viewport
     height (page itself never scrolls); min-height: 0 lets it shrink below its
     content so the columns' internal scrolls engage */
  flex: 1;
  min-height: 0;
}

/* ─── Left column: log panel ─── */
.log-panel {
  height: 100%;
  /* Release the grid child default min-height:auto so oversized content shrinks and scrolls internally */
  min-height: 0;
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
  background: #0b0b10;
  border-radius: 8px;
  padding: 10px 14px;
  font-family: var(--font-mono);
  font-size: 11px;
}

.console-line {
  color: rgba(255,255,255,0.55);
  padding: 1px 0;
  white-space: pre-wrap;
  word-break: break-all;
}

.console-empty {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  background: #0b0b10;
  border-radius: 8px;
  color: rgba(255,255,255,0.35);
  font-size: 12px;
}

/* ─── Right column: monitor cards ─── */
.monitor-side {
  display: flex;
  flex-direction: column;
  gap: 12px;
  height: 100%;
  min-height: 0;
  /* Fixed, scroll-free column: the two cards share the column height and stretch
     to fill it; metric blocks distribute the leftover space inside their card, so
     no scrollbar appears. The floating TaskDock pill may overlap the footnote
     band when active — no interactive control lives there. */
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

/* Spacing between metric blocks inside system monitor: blocks flex-share the
   card height (basis auto, equal grow) and center their content vertically, so
   the card fills the column and compresses gracefully on short windows */
.metric-block {
  margin-bottom: 14px;
  flex: 1 1 auto;
  min-height: 0;
  display: flex;
  flex-direction: column;
  justify-content: center;
}

.metric-block:last-child {
  margin-bottom: 0;
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

/* ─── Usage bar ─── */
.usage-bar-wrapper {
  display: flex;
  align-items: center;
  gap: 12px;
}

.usage-bar {
  flex: 1;
  height: 6px;
  background: var(--overlay-8);
  border-radius: 3px;
  overflow: hidden;
}

.usage-fill {
  height: 100%;
  border-radius: 3px;
  background: linear-gradient(90deg, #6366f1, #a78bfa);
  transition: width 0.6s ease;
}

.usage-text {
  font-size: 13px;
  font-weight: 600;
  color: #a78bfa;
  min-width: 36px;
  text-align: right;
}

/* ─── Metric sub ─── */
.metric-sub {
  display: flex;
  gap: 6px;
  margin-top: 10px;
  font-size: 12px;
  color: var(--text-dim);
}

.metric-divider {
  color: var(--overlay-20);
}

/* ─── GPU ─── */
.gpu-row {
  padding: 12px 0;
  border-bottom: 1px solid var(--border-light);
}

.gpu-row:last-child {
  border-bottom: none;
  padding-bottom: 0;
}

.gpu-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 10px;
}

.gpu-name {
  font-size: 13px;
  font-weight: 600;
  color: #a78bfa;
  word-break: break-all;
}

.gpu-util {
  font-size: 13px;
  font-weight: 700;
  color: var(--text-secondary);
  flex-shrink: 0;
  margin-left: 12px;
}

.gpu-mem {
  margin-top: 8px;
  font-size: 12px;
  color: var(--text-dim);
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
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
  flex: 1 1 auto;
  min-height: 0;
  align-content: center;
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

/* ─── Empty ─── */
.info-empty {
  padding: 16px 0;
  color: var(--text-dim);
  font-size: 13px;
  text-align: center;
}
</style>
