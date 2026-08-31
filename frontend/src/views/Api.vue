<template>
  <div class="page page-fixed">
    <div class="sticky-top">
      <div class="page-header">
        <h1 class="page-title">{{ t('api.title') }}</h1>
        <p class="page-subtitle">{{ t('api.subtitle') }}</p>
      </div>

      <!-- Status hero (design frame ④ .api-hero island): running status light
           with a pulse ring + state text + uptime pill, the mono address chip
           with a copy affordance, and the generation-speed gradient area chart
           embedded in the same card. Platform-related visibility untouched. -->
      <section class="api-hero">
        <div class="hero-status-row">
          <span class="pulse-dot" :class="{ on: serverRunning }"></span>
          <span class="hero-status-text">{{ serverRunning ? t('api.running') : t('api.stopped') }}</span>
          <span v-if="serverRunning && status.uptimeSeconds > 0" class="hero-uptime">
            {{ formatUptime(status.uptimeSeconds, locale) }}
            <span class="hero-uptime-label">{{ t('monitor.uptimeLabel') }}</span>
          </span>
        </div>

        <!-- Address chip: monospace URL + copy button (transient confirmation) -->
        <button
          v-if="serverRunning"
          class="hero-address"
          type="button"
          :title="copied ? t('api.addressCopied') : t('api.copyAddress')"
          @click.stop="copyAddress"
        >
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" class="address-lock-icon">
            <rect x="4" y="10" width="16" height="10" rx="2"/><path d="M8 10V7a4 4 0 018 0v3"/>
          </svg>
          <span class="address-text">http://{{ cfg.host }}:{{ cfg.port }}</span>
          <span v-if="copied" class="address-copied">{{ t('api.addressCopied') }}</span>
          <svg v-else width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="address-copy-icon">
            <rect x="9" y="9" width="12" height="12" rx="2"/><path d="M5 15H4a2 2 0 01-2-2V4a2 2 0 012-2h9a2 2 0 012 2v1"/>
          </svg>
        </button>

        <!-- Generation speed: gradient area chart (frame ④ chart-wrap) fed by
             the existing 60s decode sampling chain. Stopped state ghosts the
             data (visibility, not display) so the card height never shifts. -->
        <div class="hero-speed">
          <div class="speed-head">
            <span class="speed-label">{{ t('api.speedLabel') }}</span>
            <span class="speed-value" :class="{ 'tps-ghost': !status.serverRunning }" :aria-hidden="!status.serverRunning">
              {{ decodeTpsText }} <small>tok/s</small>
            </span>
          </div>
          <div class="speed-chart" :class="{ 'tps-ghost': !status.serverRunning }" :aria-hidden="!status.serverRunning">
            <svg :viewBox="`-6 -6 ${chartWidth + 12} ${chartHeight + 12}`">
              <defs>
                <linearGradient id="apiSpeedFill" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="0%" stop-color="#8b5cf6" stop-opacity="0.34"/>
                  <stop offset="100%" stop-color="#8b5cf6" stop-opacity="0"/>
                </linearGradient>
                <linearGradient id="apiSpeedLine" x1="0" y1="0" x2="1" y2="0">
                  <stop offset="0%" stop-color="#6366f1"/>
                  <stop offset="100%" stop-color="#a855f7"/>
                </linearGradient>
              </defs>
              <path class="speed-area" :d="decodeAreaPath" />
              <path class="speed-line" :d="decodeLinePath" />
              <circle v-if="decodeEndDot" class="speed-dot" :cx="decodeEndDot.x" :cy="decodeEndDot.y" r="4" />
            </svg>
          </div>
          <div class="speed-meta" :class="{ 'tps-ghost': !status.serverRunning }" :aria-hidden="!status.serverRunning">
            <span>{{ t('monitor.promptSpeed') }} {{ promptTpsText }} tok/s</span>
            <span>{{ t('monitor.chartLabel', { n: decodeHistory.length }) }}</span>
          </div>
          <div v-if="!status.serverRunning" class="speed-placeholder">{{ t('monitor.uptimePlaceholder') }}</div>
        </div>
      </section>

      <!-- Single primary action (frame ④ .btnrow): running = one danger-tinted
           "stop" button while restart is demoted to an icon button; stopped =
           one gradient "start" button. The start/stop/restart bindings, the
           graceful-stop chain and the busy-disable semantics are unchanged —
           only the layout is regrouped. -->
      <div class="action-row">
        <button v-if="serverRunning" class="primary-btn danger" :disabled="busy" @click="doStop">
          <svg width="13" height="13" viewBox="0 0 24 24" fill="currentColor">
            <rect x="5" y="5" width="14" height="14" rx="2"/>
          </svg>
          {{ t('api.stopServer') }}
        </button>
        <button v-else class="primary-btn start" :disabled="busy" @click="doStart">
          {{ t('api.startServer') }}
        </button>
        <button
          v-if="serverRunning"
          class="ghost-icon"
          type="button"
          :disabled="busy"
          :title="t('api.restart')"
          :aria-label="t('api.restart')"
          @click="doRestart"
        >
          <svg width="19" height="19" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.1" stroke-linecap="round" stroke-linejoin="round">
            <path d="M21 12a9 9 0 11-2.6-6.4M21 3v6h-6"/>
          </svg>
        </button>
        <button class="ghost-icon" type="button" @click.stop="showCfg = !showCfg" :aria-expanded="showCfg" :title="t('api.settings')">
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
            <circle cx="12" cy="12" r="3"/>
            <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z"/>
          </svg>
        </button>

        <!-- Server parameters popover (anchored to the action row) -->
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
      </div>
    </div>

    <!-- Main area: scrollable band with the available-models island (frame ④)
         above the terminal-style log console -->
    <div class="page-scroll">
      <!-- Available models (frame ④ island): static chips on a neutral surface -->
      <section class="models-island">
        <div class="island-head">
          <span class="island-title">{{ t('api.modelsHeading') }}</span>
          <span class="island-more">{{ t('api.modelsMore', { n: modelCount }) }}</span>
        </div>
        <div v-if="modelCount > 0" class="model-chips">
          <span v-for="m in availableModels" :key="m" class="model-chip">{{ m }}</span>
        </div>
        <span v-else class="empty-hint">
          {{ t('api.emptyHint') }}
          <button class="empty-cta" @click="goDownloads">{{ t('action.gotoDownloads') }}</button>
        </span>
      </section>

      <!-- Service log: terminal-dark console (unchanged) inside a floating
           island; the clear button stays as the log-toolbar equivalent -->
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

// Decode speed history: appended on 1s polling, keeps latest 60 samples
// (appendHistory default cap=60); rendered as the frame ④ gradient area chart
const decodeHistory = ref<number[]>([])
const chartWidth = 560
const chartHeight = 96

let pollTimer: ReturnType<typeof setInterval> | null = null

const promptTpsText = computed(() => formatPromptTps(status.value.promptTps))

const decodeTpsText = computed(() => status.value.decodeTps.toFixed(1))

const decodePoints = computed(() => chartPoints(decodeHistory.value, chartWidth, chartHeight))

// Frame ④ area chart: the sampled polyline (chartPoints) is re-expressed as
// SVG paths — a gradient stroke line, a gradient-filled area closed down to
// the baseline, and a dot marking the newest sample.
const decodeLinePath = computed(() => {
  const pts = decodePoints.value
  return pts ? 'M' + pts.split(' ').join(' L ') : ''
})

const decodeAreaPath = computed(() => {
  const line = decodeLinePath.value
  return line ? `${line} L ${chartWidth},${chartHeight} L 0,${chartHeight} Z` : ''
})

const decodeEndDot = computed<{ x: number; y: number } | null>(() => {
  const pts = decodePoints.value
  if (!pts) return null
  const last = pts.split(' ').pop()!.split(',')
  return { x: Number(last[0]), y: Number(last[1]) }
})

// ─── Address chip copy affordance (frame ④): best-effort clipboard write with
// a transient "copied" confirmation; failures keep the chip usable ───
const copied = ref(false)
let copiedTimer: ReturnType<typeof setTimeout> | null = null

async function copyAddress() {
  try {
    await navigator.clipboard.writeText(`http://${cfg.host}:${cfg.port}`)
    copied.value = true
    if (copiedTimer) clearTimeout(copiedTimer)
    copiedTimer = setTimeout(() => {
      copied.value = false
    }, 1600)
  } catch {
    // Clipboard unavailable (permissions / non-secure context): the URL stays
    // selectable text, no error surface needed.
  }
}

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
  if (copiedTimer) clearTimeout(copiedTimer)
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

/* ─── Status hero (frame ④ .api-hero): one island holding state light +
       address chip + speed chart ─── */
.api-hero {
  background: var(--bg-secondary);
  border: 1px solid var(--border);
  border-radius: var(--r-md);
  box-shadow: var(--shadow-island);
  padding: 16px 20px;
  margin-bottom: 12px;
}

.hero-status-row {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
  flex-wrap: wrap;
  row-gap: 6px;
}

/* Running status light: glowing dot with an expanding pulse ring (design .pulse) */
.pulse-dot {
  position: relative;
  width: 12px;
  height: 12px;
  border-radius: 50%;
  background: var(--text-dim);
  flex-shrink: 0;
}

.pulse-dot.on {
  background: var(--success);
  box-shadow: 0 0 8px rgba(34, 197, 94, 0.5);
}

.pulse-dot.on::before {
  content: "";
  position: absolute;
  inset: -5px;
  border-radius: 50%;
  border: 2px solid var(--success);
  opacity: 0.4;
  animation: status-pulse 1.8s infinite;
}

@keyframes status-pulse {
  0% { transform: scale(0.7); opacity: 0.6; }
  100% { transform: scale(1.35); opacity: 0; }
}

.hero-status-text {
  font-size: 17px;
  font-weight: 800;
  color: var(--text-primary);
  letter-spacing: -0.2px;
}

.hero-uptime {
  margin-left: auto;
  font-size: 11.5px;
  background: var(--hover-bg);
  border-radius: 8px;
  padding: 5px 10px;
  font-weight: 700;
  color: var(--text-secondary);
  white-space: nowrap;
  font-variant-numeric: tabular-nums;
}

.hero-uptime-label {
  font-size: 10.5px;
  color: var(--text-dim);
  font-weight: 600;
  margin-left: 4px;
}

/* Address chip: monospace URL row with a copy affordance */
.hero-address {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  margin-top: 12px;
  background: var(--hover-bg);
  border: 1px solid var(--border-light);
  border-radius: 14px;
  padding: 10px 14px;
  font-family: var(--font-mono);
  font-size: 12.5px;
  color: var(--text-secondary);
  cursor: pointer;
  text-align: left;
  transition: border-color 0.2s, color 0.2s;
}

.hero-address:hover {
  border-color: var(--overlay-10);
  color: var(--text-primary);
}

.address-lock-icon,
.address-copy-icon {
  flex-shrink: 0;
}

.address-text {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.address-copied {
  font-family: var(--font-sans);
  font-size: 11px;
  font-weight: 700;
  color: var(--success);
  flex-shrink: 0;
}

/* ─── Generation speed area chart (frame ④ chart-wrap, inside the hero) ─── */
.hero-speed {
  position: relative;
  margin-top: 14px;
}

.speed-head {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
  gap: 12px;
}

.speed-label {
  font-size: 12px;
  color: var(--text-muted);
  font-weight: 600;
}

.speed-value {
  font-size: 19px;
  font-weight: 800;
  color: var(--text-primary);
  font-variant-numeric: tabular-nums;
}

.speed-value small {
  font-size: 11px;
  color: var(--text-dim);
  font-weight: 600;
}

.speed-chart {
  margin-top: 8px;
}

/* Uniform viewBox scaling (no preserveAspectRatio=none): stroke width and the
   end dot stay undistorted at every card width; the -6..+6 viewBox margin
   keeps the end dot from clipping at the edges; aspect-ratio pins the box so
   the stopped-state ghost reserves the exact same space */
.speed-chart svg {
  display: block;
  width: 100%;
  height: auto;
  aspect-ratio: 572 / 108;
}

.speed-area {
  fill: url(#apiSpeedFill);
  stroke: none;
}

.speed-line {
  fill: none;
  stroke: url(#apiSpeedLine);
  stroke-width: 2.6;
  stroke-linecap: round;
  stroke-linejoin: round;
}

.speed-dot {
  fill: #8b5cf6;
}

.speed-meta {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  margin-top: 6px;
  font-size: 11px;
  color: var(--text-dim);
}

/* visibility (not display) keeps the stopped state reserving exactly the
   running state's space, so the hero height never shifts on start/stop */
.tps-ghost {
  visibility: hidden;
}

.speed-placeholder {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 13px;
  color: var(--text-dim);
}

/* ─── Single primary action (frame ④ .btnrow): one big action button + icon
       buttons ─── */
.action-row {
  position: relative; /* Anchor for the server params popover */
  display: flex;
  align-items: stretch;
  gap: 10px;
  margin-bottom: 14px;
}

.primary-btn {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 9px;
  padding: 15px 0;
  border: none;
  border-radius: 20px;
  font-size: 14px;
  font-weight: 800;
  font-family: inherit;
  cursor: pointer;
  color: #fff;
  transition: filter 0.2s, opacity 0.15s;
}

.primary-btn:hover:not(:disabled) {
  filter: brightness(1.06);
}

/* Disabled state follows the cfg-input dimming convention; the desaturation
   filter keeps the colored buttons recognizably disabled */
.primary-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
  filter: grayscale(0.6) brightness(0.92);
}

/* Stopped → the one executable action rides the brand gradient (frame ⑥) */
.primary-btn.start {
  background: var(--grad);
  box-shadow: 0 8px 20px rgba(124, 92, 246, 0.35);
}

/* Running → the one primary action is the danger-tinted stop */
.primary-btn.danger {
  background: rgba(239, 68, 68, 0.12);
  color: var(--danger);
  border: 1px solid rgba(239, 68, 68, 0.25);
}

/* Ghost icon buttons: restart (demoted from a full button) + server params */
.ghost-icon {
  flex: 0 0 54px;
  width: 54px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--bg-secondary);
  border: 1px solid var(--border);
  border-radius: 20px;
  box-shadow: var(--shadow-island);
  color: var(--text-secondary);
  cursor: pointer;
  transition: color 0.2s, background 0.2s, opacity 0.15s;
}

.ghost-icon:hover:not(:disabled) {
  color: var(--text-primary);
  background: var(--hover-bg);
}

.ghost-icon:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.ghost-icon[aria-expanded='true'] {
  color: var(--text-primary);
  background: var(--hover-bg);
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

/* ─── Scrollable content band: available-models island above the log island ─── */
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

/* ─── Available models (frame ④ island): static chips on a neutral surface ─── */
.models-island {
  background: var(--bg-secondary);
  border: 1px solid var(--border);
  border-radius: var(--r-md);
  box-shadow: var(--shadow-island);
  padding: 16px 18px;
  margin-bottom: 14px;
}

.island-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 10px;
}

.island-title {
  font-size: 13px;
  font-weight: 700;
  color: var(--text-muted);
  letter-spacing: 0.3px;
}

.island-more {
  font-size: 12px;
  font-weight: 700;
  color: var(--accent-light);
}

.model-chips {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  max-height: 96px;
  overflow-y: auto;
}

.model-chip {
  font-size: 12px;
  font-weight: 600;
  background: var(--hover-bg);
  color: var(--text-secondary);
  border: 1px solid var(--border-light);
  border-radius: 999px;
  padding: 7px 13px;
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

/* ─── Log island: terminal-dark console inside a floating island ─── */
.log-panel {
  /* Fills the remaining band height on tall windows; the band scrolls once
     the window is too short */
  flex: 1 1 auto;
  min-height: 380px;
  /* Release the flex child default min sizes so oversized content shrinks
     and scrolls internally instead of blowing out the band */
  min-width: 0;
  display: flex;
  flex-direction: column;
  padding: 16px 20px;
  background: var(--bg-secondary);
  border: 1px solid var(--border);
  border-radius: var(--r-md);
  box-shadow: var(--shadow-island);
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

/* Subtle in-console scrollbars: this box is the horizontal scroll container
   for overlong `pre` lines (overflow-x: auto above), and mobile overlay
   scrollbars are invisible at rest — the thin styled bar is the standing
   affordance that a clipped line tail is reachable by scrolling. Fixed
   white-alpha thumbs on purpose: the console background is terminal-dark in
   BOTH themes (#0b0b10), so theme-inverted light-theme thumb tokens would
   vanish here (same fixed-color precedent as .console-line above). */
.console-log::-webkit-scrollbar {
  width: 6px;
  height: 6px;
}

.console-log::-webkit-scrollbar-thumb {
  background: rgba(255, 255, 255, 0.16);
  border-radius: 3px;
}

.console-log::-webkit-scrollbar-thumb:hover {
  background: rgba(255, 255, 255, 0.28);
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

/* ─── Compact mode (viewport height <= 799px): keep the fixed, scroll-free
   layout but compress secondary elements so everything fits without clipping
   down to the 900x600 minimum window. Fixed values only — nothing scales
   continuously with the window size. */
@media (max-height: 799px) {
  .page-subtitle { display: none; }
  .page-header { padding-bottom: 14px; }
  .api-hero { padding: 10px 16px; }
  .hero-address { margin-top: 8px; padding: 8px 12px; }
  .hero-speed { margin-top: 10px; }
  /* The chart yields on short windows (same tradeoff as the former monitor
     card): the big speed value in the head keeps the metric readable */
  .speed-chart { display: none; }
  .speed-meta { display: none; }
  .action-row { margin-bottom: 10px; }
  .primary-btn { padding: 12px 0; }
  .ghost-icon { flex: 0 0 48px; }
  .models-island { padding: 10px 14px; margin-bottom: 10px; }
  .log-panel { padding: 10px 14px; min-height: 300px; }
}

/* ─── Phone (<=767px): the hero and the single primary action adapt to the
       thumb — the big button spans the row at 44px+ and the icon buttons stay
       square; the log console scrolls inside its own bounded band (12px mono).
       Server params popover must never exceed the viewport. ─── */
@media (max-width: 767px) {
  /* Phone heading = the design's 24px phone tier (same as Home's .greet-title
     phone rule, same 1.2 line-height), so every page header block reads the
     same height as the greeting */
  .page-title {
    font-size: 24px;
  }

  .api-hero {
    padding: 14px 16px;
  }

  .hero-address {
    font-size: 12px;
  }

  .action-row {
    gap: 8px;
  }

  /* The big action spans the row; nowrap keeps the zh labels intact */
  .primary-btn {
    min-height: 44px;
    white-space: nowrap;
  }

  .ghost-icon {
    flex: 0 0 44px;
    width: 44px;
    border-radius: 16px;
  }

  .cfg-popover {
    width: min(300px, calc(100vw - 32px));
  }

  .models-island {
    padding: 14px 16px;
  }

  .log-panel {
    flex: none;
    height: clamp(200px, 36vh, 320px);
    min-height: 0;
    padding: 14px;
  }

  .console-log {
    font-size: 12px;
  }

  .empty-cta {
    min-height: 44px;
    padding: 8px 14px;
  }
}
</style>
