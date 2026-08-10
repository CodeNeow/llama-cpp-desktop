<template>
  <div class="page">
    <div class="page-header">
      <h1 class="page-title">主页</h1>
      <p class="page-subtitle">系统状态概览</p>
    </div>

    <!-- Loading skeleton -->
    <div v-if="loading" class="loading-grid">
      <div v-for="i in 5" :key="i" class="skeleton-card">
        <div class="skeleton-line skeleton-title"></div>
        <div class="skeleton-line"></div>
        <div class="skeleton-line skeleton-short"></div>
      </div>
    </div>

    <!-- Error state -->
    <div v-else-if="error" class="error-card">
      <div class="error-icon">
        <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
          <circle cx="12" cy="12" r="10"/><line x1="12" y1="8" x2="12" y2="12"/><line x1="12" y1="16" x2="12.01" y2="16"/>
        </svg>
      </div>
      <h2>无法获取系统信息</h2>
      <p>{{ error }}</p>
      <button class="retry-btn" @click="fetchSystemInfo">重试</button>
    </div>

    <!-- Data -->
    <template v-else>
      <!-- CPU Card -->
      <section class="info-section">
        <h2 class="section-title">
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <rect x="4" y="4" width="16" height="16" rx="2" ry="2"/><rect x="9" y="9" width="6" height="6"/><line x1="9" y1="1" x2="9" y2="4"/><line x1="15" y1="1" x2="15" y2="4"/><line x1="9" y1="20" x2="9" y2="23"/><line x1="15" y1="20" x2="15" y2="23"/><line x1="20" y1="9" x2="23" y2="9"/><line x1="20" y1="14" x2="23" y2="14"/><line x1="1" y1="9" x2="4" y2="9"/><line x1="1" y1="14" x2="4" y2="14"/>
          </svg>
          处理器
        </h2>
        <div class="info-grid">
          <div class="info-item">
            <span class="info-label">型号</span>
            <span class="info-value">{{ info.cpu.model }}</span>
          </div>
          <div class="info-item">
            <span class="info-label">物理核心</span>
            <span class="info-value">{{ info.cpu.cores }} 核</span>
          </div>
          <div class="info-item">
            <span class="info-label">逻辑核心</span>
            <span class="info-value">{{ info.cpu.logicalCpus }} 线程</span>
          </div>
        </div>
      </section>

      <!-- Memory Card -->
      <section class="info-section">
        <h2 class="section-title">
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <rect x="2" y="6" width="20" height="12" rx="2"/><line x1="6" y1="10" x2="6" y2="14"/><line x1="10" y1="10" x2="10" y2="14"/><line x1="14" y1="10" x2="14" y2="14"/><line x1="18" y1="10" x2="18" y2="14"/>
          </svg>
          内存
        </h2>
        <div class="info-grid">
          <div class="info-item">
            <span class="info-label">总内存</span>
            <span class="info-value">{{ formatGB(info.memory.totalGb) }}</span>
          </div>
          <div class="info-item">
            <span class="info-label">可用内存</span>
            <span class="info-value">{{ formatGB(info.memory.freeGb) }}</span>
          </div>
          <div class="info-item info-item-full">
            <span class="info-label">使用率</span>
            <div class="usage-bar-wrapper">
              <div class="usage-bar">
                <div class="usage-fill" :style="{ width: memoryUsagePercent + '%' }"></div>
              </div>
              <span class="usage-text">{{ memoryUsagePercent }}%</span>
            </div>
          </div>
        </div>
      </section>

      <!-- GPU Card -->
      <section class="info-section">
        <h2 class="section-title">
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <rect x="2" y="3" width="20" height="14" rx="2" ry="2"/><line x1="8" y1="21" x2="16" y2="21"/><line x1="12" y1="17" x2="12" y2="21"/>
          </svg>
          显卡
        </h2>
        <div v-if="info.gpu && info.gpu.length > 0">
          <div v-for="(gpu, i) in info.gpu" :key="i" class="info-grid gpu-grid">
            <div class="info-item info-item-full">
              <span class="info-label">型号</span>
              <span class="info-value gpu-name">{{ gpu.name }}</span>
            </div>
            <div class="info-item">
              <span class="info-label">显存</span>
              <span class="info-value">{{ formatMB(gpu.memoryMb) }}</span>
            </div>
            <div class="info-item">
              <span class="info-label">驱动版本</span>
              <span class="info-value">{{ gpu.driverVersion }}</span>
            </div>
          </div>
        </div>
        <div v-else class="info-empty">
          <span>未检测到 NVIDIA 显卡</span>
        </div>
      </section>

      <!-- CUDA Card -->
      <section class="info-section">
        <h2 class="section-title">
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <polygon points="12 2 22 8.5 22 15.5 12 22 2 15.5 2 8.5 12 2"/><line x1="12" y1="22" x2="12" y2="15.5"/><polyline points="22 8.5 12 15.5 2 8.5"/>
          </svg>
          CUDA
        </h2>
        <div class="info-grid">
          <div class="info-item">
            <span class="info-label">状态</span>
            <span class="info-value">
              <span class="status-badge" :class="info.cuda.available ? 'available' : 'unavailable'">
                {{ info.cuda.available ? '可用' : '不可用' }}
              </span>
            </span>
          </div>
          <div class="info-item">
            <span class="info-label">驱动版本</span>
            <span class="info-value">{{ info.cuda.driverVersion || 'N/A' }}</span>
          </div>
          <div class="info-item">
            <span class="info-label">Toolkit 版本</span>
            <span class="info-value">{{ info.cuda.toolkitVersion || '未安装' }}</span>
          </div>
        </div>
      </section>

      <!-- llama.cpp Card -->
      <section class="info-section">
        <h2 class="section-title">
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <path d="M14.7 6.3a1 1 0 0 0 0 1.4l1.6 1.6a1 1 0 0 0 1.4 0l3.77-3.77a6 6 0 0 1-7.94 7.94l-6.91 6.91a2.12 2.12 0 0 1-3-3l6.91-6.91a6 6 0 0 1 7.94-7.94l-3.76 3.76z"/>
          </svg>
          llama.cpp
        </h2>
        <div class="info-grid">
          <div class="info-item">
            <span class="info-label">状态</span>
            <span class="info-value">
              <span class="status-badge" :class="info.llamaCpp.installed ? 'available' : 'unavailable'">
                {{ info.llamaCpp.installed ? '已安装' : '未找到' }}
              </span>
            </span>
          </div>
          <div class="info-item info-item-full" v-if="info.llamaCpp.installed">
            <span class="info-label">路径</span>
            <span class="info-value path-value">{{ info.llamaCpp.path }}</span>
          </div>
          <div class="info-item info-item-full" v-if="info.llamaCpp.version">
            <span class="info-label">版本</span>
            <span class="info-value">{{ info.llamaCpp.version }}</span>
          </div>
        </div>

        <!-- Download section -->
        <div v-if="!info.llamaCpp.installed && dlStatus.status !== 'done'" class="download-area">
          <!-- Idle: show download + custom buttons -->
          <div v-if="dlStatus.status === 'idle' || dlStatus.status === 'error'" class="download-btns">
            <button class="download-btn" @click="startDownload">
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/>
                <polyline points="7 10 12 15 17 10"/>
                <line x1="12" y1="15" x2="12" y2="3"/>
              </svg>
              下载 llama.cpp
            </button>
            <button class="custom-btn" @click="browseCustomDir">
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"/>
              </svg>
              自定义
            </button>
          </div>
          <div v-if="customPath" class="custom-path-info">
            <span class="custom-path-label">自定义路径：</span>
            <span class="custom-path-value">{{ customPath }}</span>
          </div>

          <!-- Downloading / Paused / Extracting -->
          <div v-else-if="dlStatus.status !== 'idle'" class="download-progress">
            <div class="dl-info">
              <span class="dl-label">{{ statusLabel[dlStatus.status] }}</span>
              <span class="dl-percent" v-if="dlStatus.status === 'downloading' || dlStatus.status === 'paused'">{{ dlStatus.progress }}%</span>
            </div>
            <div class="dl-bar">
              <div
                class="dl-fill"
                :class="{ indeterminate: dlStatus.status === 'fetching' || dlStatus.status === 'extracting', paused: dlStatus.status === 'paused' }"
                :style="(dlStatus.status === 'downloading' || dlStatus.status === 'paused') ? { width: dlStatus.progress + '%' } : {}"
              ></div>
            </div>
            <div class="dl-meta" v-if="dlStatus.fileName">
              <span>{{ dlStatus.fileName }}</span>
              <span v-if="dlStatus.total > 0">{{ formatSize(dlStatus.total) }}</span>
            </div>

            <!-- Action buttons -->
            <div class="dl-actions" v-if="dlStatus.status === 'downloading' || dlStatus.status === 'paused' || dlStatus.status === 'fetching'">
              <button v-if="dlStatus.status === 'downloading'" class="dl-btn pause-btn" @click="pauseDownload">
                <svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor"><rect x="6" y="4" width="4" height="16"/><rect x="14" y="4" width="4" height="16"/></svg>
                暂停
              </button>
              <button v-if="dlStatus.status === 'paused'" class="dl-btn resume-btn" @click="resumeDownload">
                <svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor"><polygon points="5 3 19 12 5 21 5 3"/></svg>
                继续
              </button>
              <button class="dl-btn stop-btn" @click="stopDownload">
                <svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor"><rect x="4" y="4" width="16" height="16" rx="2"/></svg>
                停止
              </button>
            </div>

            <div class="dl-paused-hint" v-if="dlStatus.status === 'paused'">
              下载已暂停，点击"继续"恢复
            </div>

            <div class="dl-error" v-if="dlStatus.status === 'error'">
              <span>{{ dlStatus.error }}</span>
              <button class="retry-btn-sm" @click="startDownload">重试</button>
            </div>
          </div>
        </div>
      </section>

      <!-- OS Info -->
      <section class="info-section">
        <h2 class="section-title">
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <rect x="2" y="3" width="20" height="14" rx="2" ry="2"/><line x1="8" y1="21" x2="16" y2="21"/><line x1="12" y1="17" x2="12" y2="21"/>
          </svg>
          系统
        </h2>
        <div class="info-grid">
          <div class="info-item">
            <span class="info-label">操作系统</span>
            <span class="info-value">{{ osLabel }}</span>
          </div>
          <div class="info-item">
            <span class="info-label">架构</span>
            <span class="info-value">{{ info.arch }}</span>
          </div>
        </div>
      </section>
    </template>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import {
  getCPU, getMemory, getGPU, getCUDA, getLlamaCpp, getOS,
  getLlamaCppDownloadStatus, startLlamaCppDownload, pauseLlamaCppDownload,
  resumeLlamaCppDownload, stopLlamaCppDownload, browseLlamaCppDir
} from '../wails'

interface SystemInfo {
  os: string
  arch: string
  cpu: { model: string; cores: number; logicalCpus: number }
  memory: { totalGb: number; freeGb: number }
  gpu: { name: string; memoryMb: number; driverVersion: string }[]
  cuda: { available: boolean; driverVersion: string; toolkitVersion: string }
  llamaCpp: { installed: boolean; path: string; version: string }
}

const info = ref<SystemInfo>({
  os: '', arch: '',
  cpu: { model: '', cores: 0, logicalCpus: 0 },
  memory: { totalGb: 0, freeGb: 0 },
  gpu: [],
  cuda: { available: false, driverVersion: '', toolkitVersion: '' },
  llamaCpp: { installed: false, path: '', version: '' }
})
const sectionsReady = reactive({
  cpu: false, memory: false, gpu: false, cuda: false, llamaCpp: false, os: false
})
const loading = ref(true)
const error = ref('')

// Download state
interface DlStatus {
  status: string  // idle | fetching | downloading | extracting | done | error
  progress: number
  total: number
  downloaded: number
  fileName: string
  version: string
  error: string
}
const dlStatus = ref<DlStatus>({ status: 'idle', progress: 0, total: 0, downloaded: 0, fileName: '', version: '', error: '' })
const customPath = ref('')
let pollTimer: ReturnType<typeof setInterval> | null = null

const memoryUsagePercent = computed(() => {
  if (info.value.memory.totalGb === 0) return 0
  const used = info.value.memory.totalGb - info.value.memory.freeGb
  return Math.round((used / info.value.memory.totalGb) * 100)
})

const osLabel = computed(() => {
  const labels: Record<string, string> = {
    windows: 'Windows',
    linux: 'Linux',
    darwin: 'macOS'
  }
  return labels[info.value.os] || info.value.os || '...'
})

function formatGB(gb: number): string {
  if (gb <= 0) return 'N/A'
  return `${gb.toFixed(1)} GB`
}

function formatMB(mb: number): string {
  if (mb <= 0) return 'N/A'
  if (mb >= 1024) return `${(mb / 1024).toFixed(1)} GB`
  return `${mb} MB`
}

function formatSize(bytes: number): string {
  if (bytes <= 0) return ''
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(0)} KB`
  if (bytes < 1024 * 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
  return `${(bytes / (1024 * 1024 * 1024)).toFixed(2)} GB`
}

const statusLabel: Record<string, string> = {
  idle: '',
  fetching: '正在获取最新版本信息...',
  downloading: '正在下载',
  paused: '已暂停',
  extracting: '正在解压...',
  done: '下载完成',
  error: '下载失败'
}

// Poll download status
function pollDownloadStatus() {
  getLlamaCppDownloadStatus()
    .then((data: any) => {
      dlStatus.value = data as DlStatus
      if (dlStatus.value.status === 'done' || dlStatus.value.status === 'error') {
        stopPolling()
        if (dlStatus.value.status === 'done') {
          setTimeout(fetchSystemInfo, 500)
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
    dlStatus.value.error = `无法连接后端: ${e.message}`
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
      await fetchSystemInfo()
    }
  } catch {}
}

// Check download status on mount (in case there's an ongoing download)
function checkInitialDownloadStatus() {
  getLlamaCppDownloadStatus()
    .then((data: any) => {
      const s = data as DlStatus
      if (s.status === 'downloading' || s.status === 'fetching' || s.status === 'extracting') {
        dlStatus.value = s
        startPolling()
      } else if (s.status === 'done' && !info.value.llamaCpp.installed) {
        fetchSystemInfo()
      }
    })
    .catch(() => {})
}

async function fetchSystemInfo() {
  loading.value = true
  error.value = ''

  const fetchers: [() => Promise<any>, keyof typeof sectionsReady, (data: any) => void][] = [
    [getCPU, 'cpu', (d) => { info.value.cpu = d }],
    [getMemory, 'memory', (d) => { info.value.memory = d }],
    [getGPU, 'gpu', (d) => { info.value.gpu = d || [] }],
    [getCUDA, 'cuda', (d) => { info.value.cuda = d }],
    [getLlamaCpp, 'llamaCpp', (d) => { info.value.llamaCpp = d }],
    [getOS, 'os', (d) => { info.value.os = d.os; info.value.arch = d.arch }],
  ]

  // Fire all fetches in parallel, resolve each independently
  let pending = fetchers.length
  fetchers.forEach(([fn, key, setter]) => {
    fn()
      .then(data => { setter(data) })
      .catch(() => {})
      .finally(() => {
        sectionsReady[key] = true
        pending--
        if (pending <= 0) loading.value = false
      })
  })

  // Show content after 600ms even if not all done
  setTimeout(() => { if (pending > 0) loading.value = false }, 600)
}

onMounted(() => {
  fetchSystemInfo().then(checkInitialDownloadStatus)
})
</script>

<style scoped>
.page {
  padding: 36px 48px 60px;
  max-width: 960px;
}

.page-header {
  margin-bottom: 36px;
}

.page-title {
  font-size: 28px;
  font-weight: 700;
  color: var(--text-primary);
  margin: 0 0 4px;
  letter-spacing: -0.5px;
}

.page-subtitle {
  font-size: 14px;
  color: var(--text-dim);
  margin: 0;
}

/* ─── Section ─── */
.info-section {
  margin-bottom: 20px;
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
  color: #a78bfa;
  flex-shrink: 0;
}

/* ─── Info grid ─── */
.info-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 14px;
}

.gpu-grid {
  margin-bottom: 12px;
  padding-bottom: 12px;
  border-bottom: 1px solid var(--border-light);
}

.gpu-grid:last-child {
  margin-bottom: 0;
  padding-bottom: 0;
  border-bottom: none;
}

.info-item {
  display: flex;
  flex-direction: column;
  gap: 4px;
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
  word-break: break-all;
}

.gpu-name {
  color: #a78bfa;
  font-weight: 600;
}

.path-value {
  font-family: 'JetBrains Mono', 'Fira Code', monospace;
  font-size: 12px;
  color: var(--text-muted);
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
  color: #22c55e;
  border: 1px solid rgba(34, 197, 94, 0.2);
}

.status-badge.unavailable {
  background: rgba(239, 68, 68, 0.1);
  color: #ef4444;
  border: 1px solid rgba(239, 68, 68, 0.15);
}

/* ─── Memory bar ─── */
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

/* ─── Empty ─── */
.info-empty {
  padding: 16px 0;
  color: var(--text-dim);
  font-size: 13px;
  text-align: center;
}

/* ─── Loading skeleton ─── */
.loading-grid {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

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
  background: rgba(99, 102, 241, 0.15);
  color: #a78bfa;
  border: 1px solid rgba(99, 102, 241, 0.25);
  border-radius: 8px;
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.2s;
}

.retry-btn:hover {
  background: rgba(99, 102, 241, 0.25);
  border-color: rgba(99, 102, 241, 0.4);
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
  color: #a78bfa;
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

/* ─── Download progress ─── */
.download-progress {
  display: flex;
  flex-direction: column;
  gap: 10px;
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
  color: #a78bfa;
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
  color: #a78bfa;
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
