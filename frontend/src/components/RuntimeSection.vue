<template>
  <!-- Runtime environment section: embedded in the merged System Environment
       page (Home.vue) below the hardware grid. Owns its own data loading
       (skeleton / error + retry) and all llama.cpp download / custom-directory
       actions; the page owns the sticky header, so there is no page chrome
       here — only the section heading and its cards. -->
  <section id="runtime-section" class="runtime-section">
    <h2 class="section-title">
      <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
        <polygon points="12 2 2 7 12 12 22 7 12 2"/><polyline points="2 17 12 22 22 17"/><polyline points="2 12 12 17 22 12"/>
      </svg>
      {{ t('runtime.title') }}
    </h2>

    <!-- Loading skeleton -->
    <div v-if="loading" class="skeleton-card">
      <div class="skeleton-line skeleton-title"></div>
      <div class="skeleton-line"></div>
      <div class="skeleton-line skeleton-short"></div>
    </div>

    <!-- Error state -->
    <div v-else-if="error" class="error-card">
      <div class="error-icon">
        <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
          <circle cx="12" cy="12" r="10"/><line x1="12" y1="8" x2="12" y2="12"/><line x1="12" y1="16" x2="12.01" y2="16"/>
        </svg>
      </div>
      <h2>{{ t('runtime.errorTitle') }}</h2>
      <p>{{ error }}</p>
      <button class="retry-btn" @click="fetchInfo">{{ t('runtime.retry') }}</button>
    </div>

    <!-- Data: one stacked full-width card per managed dependency (llama.cpp today,
         future runtime dependencies append their own cards below) -->
    <template v-else>
      <!-- llama.cpp Card -->
      <section class="info-section">
        <h2 class="section-title">
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <path d="M14.7 6.3a1 1 0 0 0 0 1.4l1.6 1.6a1 1 0 0 0 1.4 0l3.77-3.77a6 6 0 0 1-7.94 7.94l-6.91 6.91a2.12 2.12 0 0 1-3-3l6.91-6.91a6 6 0 0 1 7.94-7.94l-3.76 3.76z"/>
          </svg>
          {{ t('runtime.llamacpp') }}
        </h2>
        <div class="info-grid">
          <div class="info-item">
            <span class="info-label">{{ t('runtime.llamacpp.status') }}</span>
            <span class="info-value">
              <span class="status-badge" :class="info.installed ? 'available' : 'unavailable'">
                {{ info.installed ? t('runtime.llamacpp.installed') : t('runtime.llamacpp.notFound') }}
              </span>
            </span>
          </div>
          <div class="info-item" v-if="info.version">
            <span class="info-label">{{ t('runtime.llamacpp.version') }}</span>
            <span class="info-value">{{ info.version }}</span>
          </div>
          <div class="info-item info-item-full" v-if="info.installed">
            <span class="info-label">{{ t('runtime.llamacpp.path') }}</span>
            <span class="info-value path-value">{{ info.path }}</span>
          </div>
          <div class="info-item info-item-full" v-if="downloadDir">
            <span class="info-label">{{ t('runtime.downloadDir') }}</span>
            <span class="info-value path-value">{{ downloadDir }}</span>
          </div>
        </div>

        <!-- Installed components: the download fetches the main-program asset plus
             (on Windows CUDA builds) the cudart runtime asset and extracts both into
             the same directory, so each component is reported separately -->
        <div v-if="info.installed" class="components-area">
          <div class="components-title">{{ t('runtime.components') }}</div>
          <div class="comp-row">
            <span class="comp-name">{{ t('runtime.compMain') }}</span>
            <span class="status-badge available">{{ t('runtime.llamacpp.installed') }}</span>
          </div>
          <div v-if="isWindows" class="comp-row">
            <span class="comp-name">
              {{ t('runtime.compCudart') }}
              <span class="comp-desc">{{ t('runtime.compCudartDesc') }}</span>
            </span>
            <span class="status-badge" :class="info.cudartInstalled ? 'available' : 'unavailable'">
              {{ cudartBadge }}
            </span>
          </div>
        </div>

        <!-- Download section: three blocks render independently via v-if (not mutually bound); display conditions unified
             by downloadVisibility, avoiding v-if/v-else-if mutual exclusion that would swallow the progress area -->
        <div v-if="!info.installed && dlStatus.status !== 'done'" class="download-area">
          <!-- Idle/error: show download + custom buttons -->
          <div v-if="dlVisibility.showButtons" class="download-btns">
            <button class="download-btn" @click="startDownload">
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/>
                <polyline points="7 10 12 15 17 10"/>
                <line x1="12" y1="15" x2="12" y2="3"/>
              </svg>
              {{ t('runtime.downloadLlamaCpp') }}
            </button>
            <button class="custom-btn" @click="browseCustomDir">
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"/>
              </svg>
              {{ t('runtime.custom') }}
            </button>
          </div>
          <!-- Custom path info: independent v-if, not bound to progress area -->
          <div v-if="customPath" class="custom-path-info">
            <span class="custom-path-label">{{ t('runtime.customPath') }}</span>
            <span class="custom-path-value">{{ customPath }}</span>
          </div>

          <!-- Downloading / Paused / Extracting / Error: display depends only on download status,
               independent of whether a custom path is set -->
          <div v-if="dlVisibility.showProgress" class="download-progress">
            <!-- Overall summary line + one row per package (the cudart row appears only
                 once its asset actually starts downloading; CPU/Vulkan builds never ship it) -->
            <div class="dl-info">
              <span class="dl-label">{{ statusLabel[dlStatus.status] }}</span>
              <span class="dl-percent" v-if="dlStatus.status === 'downloading' || dlStatus.status === 'paused'">{{ dlStatus.progress }}%</span>
            </div>
            <div v-for="pkg in packages" :key="pkg.id" class="pkg-row">
              <div class="pkg-head">
                <span class="pkg-name">
                  <svg v-if="pkg.done" width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><polyline points="20 6 9 17 4 12"/></svg>
                  {{ pkg.id === 'main' ? t('runtime.pkgMain') : t('runtime.pkgCudart') }}
                </span>
                <span class="pkg-state">
                  <template v-if="pkg.done">{{ t('dl.done') }}</template>
                  <template v-else-if="pkg.active && dlStatus.status === 'extracting'">{{ t('dl.extracting') }}</template>
                  <template v-else>{{ pkg.progress }}%</template>
                </span>
              </div>
              <div class="dl-bar">
                <div
                  class="dl-fill"
                  :class="{ indeterminate: pkg.active && (dlStatus.status === 'fetching' || dlStatus.status === 'extracting'), paused: dlStatus.status === 'paused' }"
                  :style="{ width: pkg.progress + '%' }"
                ></div>
              </div>
              <div class="dl-meta" v-if="pkg.active && dlStatus.fileName">
                <span>{{ dlStatus.fileName }}</span>
                <span v-if="dlStatus.total > 0">{{ formatSize(dlStatus.total) }}</span>
              </div>
            </div>

            <!-- Action buttons -->
            <div class="dl-actions" v-if="dlStatus.status === 'downloading' || dlStatus.status === 'paused' || dlStatus.status === 'fetching'">
              <button v-if="dlStatus.status === 'downloading'" class="dl-btn pause-btn" @click="pauseDownload">
                <svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor"><rect x="6" y="4" width="4" height="16"/><rect x="14" y="4" width="4" height="16"/></svg>
                {{ t('runtime.pause') }}
              </button>
              <button v-if="dlStatus.status === 'paused'" class="dl-btn resume-btn" @click="resumeDownload">
                <svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor"><polygon points="5 3 19 12 5 21 5 3"/></svg>
                {{ t('runtime.resume') }}
              </button>
              <button class="dl-btn stop-btn" @click="stopDownload">
                <svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor"><rect x="4" y="4" width="16" height="16" rx="2"/></svg>
                {{ t('runtime.stop') }}
              </button>
            </div>

            <div class="dl-paused-hint" v-if="dlStatus.status === 'paused'">
              {{ t('runtime.dlPausedHint') }}
            </div>

            <div class="dl-error" v-if="dlStatus.status === 'error'">
              <span>{{ dlStatus.error }}</span>
              <button class="retry-btn-sm" @click="startDownload">{{ t('runtime.retry') }}</button>
            </div>
          </div>
        </div>
      </section>
    </template>
  </section>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted, onUnmounted } from 'vue'
import {
  getLlamaCpp, getLlamaCppDownloadStatus, startLlamaCppDownload, pauseLlamaCppDownload,
  resumeLlamaCppDownload, stopLlamaCppDownload, browseLlamaCppDir, getConfig, getOS
} from '../wails'
import { downloadVisibility, initialDownloadAction, isCudartAsset, packageRows } from '../lib/llamaDownload'
import { t } from '../lib/i18n'

interface LlamaCppInfo {
  installed: boolean
  path: string
  version: string
  cudartInstalled: boolean
  /** CUDA major family of the installed cudart runtime ("13", "12"), "" when unknown */
  cudartVersion: string
}

const info = ref<LlamaCppInfo>({ installed: false, path: '', version: '', cudartInstalled: false, cudartVersion: '' })
const loading = ref(true)
const error = ref('')
// The cudart component row applies to Windows only (the runtime asset is Windows-exclusive)
const isWindows = ref(false)

// Cudart component badge: append the detected CUDA major family (parsed from
// the cudart64_*.dll file name) to the installed label when known
const cudartBadge = computed(() => {
  if (!info.value.cudartInstalled) return t('runtime.compNotDetected')
  return info.value.cudartVersion
    ? `${t('runtime.llamacpp.installed')} · CUDA ${info.value.cudartVersion}`
    : t('runtime.llamacpp.installed')
})

// Download state
interface DlStatus {
  status: string  // idle | fetching | downloading | paused | extracting | done | error
  progress: number
  total: number
  downloaded: number
  fileName: string
  version: string
  error: string
}
const dlStatus = ref<DlStatus>({ status: 'idle', progress: 0, total: 0, downloaded: 0, fileName: '', version: '', error: '' })
const customPath = ref('')
const downloadDir = ref('')
let pollTimer: ReturnType<typeof setInterval> | null = null

// Download area visibility: button group / custom path info / progress area render independently (see lib/llamaDownload)
const dlVisibility = computed(() => downloadVisibility(dlStatus.value.status))

// Cumulative byte count snapshotted when the file name switches from the main
// asset to the cudart asset: the backend reports one combined Downloaded/Total
// pair across both sequential packages, so the cudart row's own share is
// (Downloaded − mainBytes) / (Total − mainBytes). A mid-download remount joins
// with the cudart asset already current (mainBytes still 0); the best available
// snapshot is the cumulative count at that moment. Reset when the file name
// clears (retry/idle).
const mainBytes = ref(0)
watch(() => dlStatus.value.fileName, (name, prev) => {
  if (!name) {
    mainBytes.value = 0
    return
  }
  if (isCudartAsset(name) && (mainBytes.value === 0 || (prev && !isCudartAsset(prev)))) {
    mainBytes.value = dlStatus.value.downloaded
  }
})

// Per-package progress rows for the download area (see lib/llamaDownload.packageRows)
const packages = computed(() =>
  packageRows(dlStatus.value.fileName, dlStatus.value.downloaded, dlStatus.value.total, mainBytes.value))

function formatSize(bytes: number): string {
  if (bytes <= 0) return ''
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(0)} KB`
  if (bytes < 1024 * 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
  return `${(bytes / (1024 * 1024 * 1024)).toFixed(2)} GB`
}

// Download status → text mapping: wrapped in computed so t() re-evaluates on locale switch,
// ensuring status labels update instantly when language changes (computed ref auto-unwraps in template, no template change needed)
const statusLabel = computed<Record<string, string>>(() => ({
  idle: '',
  fetching: t('dl.fetching'),
  downloading: t('dl.downloading'),
  paused: t('dl.paused'),
  extracting: t('dl.extracting'),
  done: t('dl.done'),
  error: t('dl.error'),
}))

// Poll download status
function pollDownloadStatus() {
  getLlamaCppDownloadStatus()
    .then((data: any) => {
      dlStatus.value = data as DlStatus
      if (dlStatus.value.status === 'done' || dlStatus.value.status === 'error') {
        stopPolling()
        if (dlStatus.value.status === 'done') {
          setTimeout(fetchInfo, 500)
        }
      }
    })
    .catch(() => {})
}

function startPolling() {
  stopPolling()
  pollTimer = setInterval(pollDownloadStatus, 500)
  pollDownloadStatus()
}

function stopPolling() {
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
}

async function startDownload() {
  try {
    dlStatus.value = { status: 'fetching', progress: 0, total: 0, downloaded: 0, fileName: '', version: '', error: '' }
    startPolling()
    await startLlamaCppDownload()
  } catch (e: any) {
    dlStatus.value.status = 'error'
    dlStatus.value.error = t('runtime.backendError', { msg: e.message })
    stopPolling()
  }
}

async function pauseDownload() {
  try { await pauseLlamaCppDownload() } catch {}
}

async function resumeDownload() {
  try { await resumeLlamaCppDownload() } catch {}
}

async function stopDownload() {
  try {
    await stopLlamaCppDownload()
    stopPolling()
    setTimeout(pollDownloadStatus, 300)
  } catch {}
}

async function browseCustomDir() {
  try {
    const dir = await browseLlamaCppDir()
    if (dir) {
      customPath.value = dir
      await fetchInfo()
    }
  } catch {}
}

// Restore the custom llama.cpp dir and the download dir from backend config on mount,
// so returning to the page shows both paths (#restore-original)
async function restoreConfigPaths() {
  try {
    const cfg = await getConfig()
    if (cfg.llamaCppDir) customPath.value = cfg.llamaCppDir
    if (cfg.llamaCppDownloadDir) downloadDir.value = cfg.llamaCppDownloadDir
  } catch {}
}

// Check download status on mount (in case there's an ongoing download)
function checkInitialDownloadStatus() {
  getLlamaCppDownloadStatus()
    .then((data: any) => {
      const s = data as DlStatus
      const action = initialDownloadAction(s.status, info.value.installed)
      if (action === 'poll') {
        // Download still in progress; resume polling to keep updating progress
        dlStatus.value = s
        startPolling()
      } else if (action === 'refresh') {
        // Download completed but install not detected; refresh info
        fetchInfo()
      } else if (action === 'showError') {
        // Download failed while away: restore error state; UI auto-shows error info and retry button
        // (downloadVisibility's showButtons / showProgress both cover error)
        dlStatus.value = s
      }
      // 'none': no action needed
    })
    .catch(() => {})
}

async function fetchInfo() {
  loading.value = true
  error.value = ''
  try {
    info.value = await getLlamaCpp()
  } catch (e: any) {
    error.value = t('runtime.backendError', { msg: e.message })
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  getOS()
    .then((o: any) => { isWindows.value = o.os === 'windows' })
    .catch(() => {})
  restoreConfigPaths()
  fetchInfo().then(checkInitialDownloadStatus)
})

// Stop polling on unmount to avoid interval spinning until download finishes;
// returning to the page remounts and restarts polling (download continues in backend goroutine)
onUnmounted(() => {
  stopPolling()
})
</script>

<style scoped>
/* ─── Section: heading + stacked full-width cards below the hardware grid ─── */
.runtime-section {
  /* Clearance from the hardware cards-grid above (the grid owns its own gaps) */
  margin-top: 28px;
}

.info-section {
  margin-bottom: 16px;
  padding: 24px 28px;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 14px;
  transition: border-color 0.2s;
}

.info-section:hover {
  border-color: var(--overlay-10);
}

.section-title {
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 15px;
  font-weight: 600;
  color: var(--text-secondary);
  margin: 0 0 18px;
}

.section-title svg {
  color: var(--accent-light);
  flex-shrink: 0;
}

/* ─── Info grid ─── */
.info-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(140px, 1fr));
  gap: 14px;
}

.info-item {
  display: flex;
  flex-direction: column;
  gap: 4px;
  /* Release the grid-item automatic minimum so the ellipsized path-value
     can shrink to the track width instead of stretching the item */
  min-width: 0;
}

.info-item-full {
  grid-column: 1 / -1;
}

.info-label {
  font-size: 12px;
  color: var(--text-dim);
  text-transform: uppercase;
  letter-spacing: 0.5px;
  font-weight: 500;
}

.info-value {
  font-size: 14px;
  color: var(--text-secondary);
  font-weight: 500;
  /* Soft wrap at word boundaries only; long unbreakable strings may overflow
     visually instead of being cut mid-word (version strings, hex hashes) */
  overflow-wrap: break-word;
}

.path-value {
  font-family: 'JetBrains Mono', 'Fira Code', monospace;
  font-size: 12px;
  color: var(--text-muted);
  /* Single-line ellipsis: install paths stay on one line and truncate at the end */
  display: block;
  min-width: 0;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

/* ─── Status badge ─── */
.status-badge {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 2px 12px;
  border-radius: 20px;
  font-size: 12px;
  font-weight: 600;
}

.status-badge.available {
  background: rgba(34, 197, 94, 0.12);
  color: var(--success);
  border: 1px solid rgba(34, 197, 94, 0.2);
}

.status-badge.unavailable {
  background: rgba(239, 68, 68, 0.1);
  color: var(--danger);
  border: 1px solid rgba(239, 68, 68, 0.15);
}

/* ─── Loading skeleton ─── */
.skeleton-card {
  padding: 24px 28px;
  background: var(--surface);
  border: 1px solid var(--skeleton-bg);
  border-radius: 14px;
}

.skeleton-line {
  height: 12px;
  background: var(--skeleton-bg);
  border-radius: 4px;
  margin-bottom: 10px;
  animation: shimmer 1.5s infinite;
}

.skeleton-title {
  width: 30%;
  height: 16px;
}

.skeleton-short {
  width: 50%;
}

@keyframes shimmer {
  0%, 100% { opacity: 0.4; }
  50% { opacity: 0.8; }
}

/* ─── Error ─── */
.error-card {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 56px 32px;
  background: rgba(239, 68, 68, 0.04);
  border: 1px solid rgba(239, 68, 68, 0.12);
  border-radius: 16px;
  text-align: center;
}

.error-icon {
  color: rgba(239, 68, 68, 0.5);
  margin-bottom: 16px;
}

.error-card h2 {
  font-size: 18px;
  font-weight: 600;
  color: var(--text-secondary);
  margin: 0 0 8px;
}

.error-card p {
  font-size: 13px;
  color: var(--text-dim);
  margin: 0 0 20px;
}

.retry-btn {
  padding: 8px 24px;
  background: var(--active-bg);
  color: var(--accent-light);
  border: 1px solid var(--overlay-20);
  border-radius: 8px;
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.2s;
}

.retry-btn:hover {
  background: var(--accent-glow);
}

/* ─── Download area ─── */
.download-area {
  margin-top: 20px;
  padding-top: 16px;
  border-top: 1px solid var(--border);
}

.download-btns {
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
}

.download-btn {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 10px 22px;
  background: linear-gradient(135deg, rgba(99, 102, 241, 0.2), rgba(167, 139, 250, 0.15));
  color: var(--accent-light);
  border: 1px solid rgba(99, 102, 241, 0.3);
  border-radius: 10px;
  font-size: 14px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s ease;
}

.download-btn:hover {
  background: linear-gradient(135deg, rgba(99, 102, 241, 0.3), rgba(167, 139, 250, 0.25));
  border-color: rgba(99, 102, 241, 0.5);
  color: var(--accent-light);
  transform: translateY(-1px);
  box-shadow: 0 4px 12px rgba(99, 102, 241, 0.2);
}

.download-btn:active {
  transform: translateY(0);
}

/* ─── Custom button ─── */
.custom-btn {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 10px 22px;
  background: var(--border-light);
  color: var(--text-muted);
  border: 1px solid var(--overlay-10);
  border-radius: 10px;
  font-size: 14px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.2s ease;
}

.custom-btn:hover {
  background: var(--overlay-8);
  border-color: var(--scrollbar-thumb-hover);
  color: var(--text-primary);
}

.custom-path-info {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 12px;
  padding: 8px 14px;
  background: rgba(34, 197, 94, 0.06);
  border: 1px solid rgba(34, 197, 94, 0.12);
  border-radius: 8px;
}

.custom-path-label {
  font-size: 12px;
  color: var(--text-dim);
}

.custom-path-value {
  font-size: 12px;
  color: #22c55e;
  font-family: 'JetBrains Mono', 'Fira Code', monospace;
  word-break: break-all;
}

/* ─── Installed components list ─── */
.components-area {
  margin-top: 20px;
  padding-top: 16px;
  border-top: 1px solid var(--border);
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.components-title {
  font-size: 12px;
  color: var(--text-dim);
  text-transform: uppercase;
  letter-spacing: 0.5px;
  font-weight: 500;
}

.comp-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.comp-name {
  font-size: 14px;
  color: var(--text-secondary);
  font-weight: 500;
  display: inline-flex;
  align-items: baseline;
  gap: 8px;
  flex-wrap: wrap;
}

.comp-desc {
  font-size: 12px;
  color: var(--text-dim);
  font-weight: 400;
}

/* ─── Download progress ─── */
.download-progress {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.pkg-row {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding-bottom: 4px;
}

.pkg-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.pkg-name {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  color: var(--text-secondary);
}

.pkg-name svg {
  color: #22c55e;
  flex-shrink: 0;
}

.pkg-state {
  font-size: 12px;
  color: var(--text-muted);
}

.dl-info {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.dl-label {
  font-size: 13px;
  color: var(--text-muted);
}

.dl-percent {
  font-size: 13px;
  font-weight: 700;
  color: var(--accent-light);
}

.dl-bar {
  height: 6px;
  background: var(--overlay-8);
  border-radius: 3px;
  overflow: hidden;
}

.dl-fill {
  height: 100%;
  border-radius: 3px;
  background: linear-gradient(90deg, #6366f1, #a78bfa);
  transition: width 0.3s ease;
}

.dl-fill.indeterminate {
  width: 30% !important;
  animation: dl-slide 1.5s ease-in-out infinite;
}

@keyframes dl-slide {
  0% { transform: translateX(-100%); }
  100% { transform: translateX(430%); }
}

.dl-meta {
  display: flex;
  justify-content: space-between;
  font-size: 11px;
  color: var(--text-dim);
  font-family: 'JetBrains Mono', 'Fira Code', monospace;
}

.dl-error {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 12px;
  background: rgba(239, 68, 68, 0.08);
  border-radius: 8px;
  font-size: 12px;
  color: rgba(239, 68, 68, 0.8);
}

/* ─── Download action buttons ─── */
.dl-actions {
  display: flex;
  gap: 8px;
  padding-top: 4px;
}

.dl-btn {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 5px 14px;
  border-radius: 7px;
  font-size: 12px;
  font-weight: 500;
  cursor: pointer;
  border: 1px solid transparent;
  transition: all 0.2s ease;
}

.pause-btn {
  background: rgba(251, 191, 36, 0.12);
  color: #fbbf24;
  border-color: rgba(251, 191, 36, 0.2);
}

.pause-btn:hover {
  background: rgba(251, 191, 36, 0.2);
  border-color: rgba(251, 191, 36, 0.35);
}

.resume-btn {
  background: rgba(34, 197, 94, 0.12);
  color: #22c55e;
  border-color: rgba(34, 197, 94, 0.2);
}

.resume-btn:hover {
  background: rgba(34, 197, 94, 0.2);
  border-color: rgba(34, 197, 94, 0.35);
}

.stop-btn {
  background: rgba(239, 68, 68, 0.08);
  color: rgba(239, 68, 68, 0.7);
  border-color: rgba(239, 68, 68, 0.15);
}

.stop-btn:hover {
  background: rgba(239, 68, 68, 0.15);
  border-color: rgba(239, 68, 68, 0.3);
  color: rgba(239, 68, 68, 0.9);
}

.dl-paused-hint {
  font-size: 12px;
  color: rgba(251, 191, 36, 0.6);
}

.dl-fill.paused {
  background: linear-gradient(90deg, #fbbf24, #f59e0b);
}

.retry-btn-sm {
  padding: 4px 14px;
  background: rgba(99, 102, 241, 0.15);
  color: var(--accent-light);
  border: 1px solid rgba(99, 102, 241, 0.25);
  border-radius: 6px;
  font-size: 12px;
  cursor: pointer;
  transition: all 0.2s;
  flex-shrink: 0;
}

.retry-btn-sm:hover {
  background: rgba(99, 102, 241, 0.25);
}
</style>
