<template>
  <div class="page">
    <div class="sticky-top">
      <div class="page-header">
        <h1 class="page-title">{{ t('home.title') }}</h1>
        <p class="page-subtitle">{{ t('home.subtitle') }}</p>
      </div>
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
      <h2>{{ t('home.errorTitle') }}</h2>
      <p>{{ error }}</p>
      <button class="retry-btn" @click="fetchSystemInfo">{{ t('home.retry') }}</button>
    </div>

    <!-- Data -->
    <template v-else>
      <div class="cards-grid">
        <!-- CPU Card -->
        <section class="info-section">
          <h2 class="section-title">
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <rect x="4" y="4" width="16" height="16" rx="2" ry="2"/><rect x="9" y="9" width="6" height="6"/><line x1="9" y1="1" x2="9" y2="4"/><line x1="15" y1="1" x2="15" y2="4"/><line x1="9" y1="20" x2="9" y2="23"/><line x1="15" y1="20" x2="15" y2="23"/><line x1="20" y1="9" x2="23" y2="9"/><line x1="20" y1="14" x2="23" y2="14"/><line x1="1" y1="9" x2="4" y2="9"/><line x1="1" y1="14" x2="4" y2="14"/>
            </svg>
            {{ t('home.cpu') }}
          </h2>
          <div class="info-grid">
            <div class="info-item info-item-full">
              <span class="info-label">{{ t('home.cpu.model') }}</span>
              <span class="info-value">{{ info.cpu.model }}</span>
            </div>
            <div class="info-item">
              <span class="info-label">{{ t('home.cpu.cores') }}</span>
              <span class="info-value">{{ t('home.cpu.coresValue', { n: info.cpu.cores }) }}</span>
            </div>
            <div class="info-item">
              <span class="info-label">{{ t('home.cpu.threads') }}</span>
              <span class="info-value">{{ t('home.cpu.threadsValue', { n: info.cpu.logicalCpus }) }}</span>
            </div>
          </div>
        </section>

        <!-- Memory Card -->
        <section class="info-section">
          <h2 class="section-title">
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <rect x="2" y="6" width="20" height="12" rx="2"/><line x1="6" y1="10" x2="6" y2="14"/><line x1="10" y1="10" x2="10" y2="14"/><line x1="14" y1="10" x2="14" y2="14"/><line x1="18" y1="10" x2="18" y2="14"/>
            </svg>
            {{ t('home.memory') }}
          </h2>
          <!-- Usage progress bar -->
          <div class="memory-usage">
            <div class="memory-usage-header">
              <span class="memory-usage-label">{{ t('home.memory.usageLabel', { used: formatGB(info.memory.totalGb - info.memory.freeGb), total: formatGB(info.memory.totalGb) }) }}</span>
              <span class="memory-usage-pct">{{ t('home.memory.usagePercent', { pct: usagePercent(info.memory.totalGb - info.memory.freeGb, info.memory.totalGb) }) }}</span>
            </div>
            <div class="usage-bar">
              <div class="usage-fill" :style="{ width: usagePercent(info.memory.totalGb - info.memory.freeGb, info.memory.totalGb) + '%' }"></div>
            </div>
          </div>
          <div class="info-grid">
            <div class="info-item">
              <span class="info-label">{{ t('home.memory.total') }}</span>
              <span class="info-value">{{ formatGB(info.memory.totalGb) }}</span>
            </div>
            <div class="info-item">
              <span class="info-label">{{ t('home.memory.available') }}</span>
              <span class="info-value">{{ formatGB(info.memory.freeGb) }}</span>
            </div>
          </div>
        </section>

        <!-- GPU Card -->
        <section class="info-section">
          <h2 class="section-title">
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <rect x="2" y="3" width="20" height="14" rx="2" ry="2"/><line x1="8" y1="21" x2="16" y2="21"/><line x1="12" y1="17" x2="12" y2="21"/>
            </svg>
            {{ t('home.gpu') }}
          </h2>
          <div v-if="info.gpu && info.gpu.length > 0">
            <div v-for="(gpu, i) in info.gpu" :key="i" class="info-grid gpu-grid">
              <div class="info-item info-item-full">
                <span class="info-label">{{ t('home.gpu.model') }}</span>
                <span class="info-value gpu-name">{{ gpu.name }}</span>
              </div>
              <div class="info-item">
                <span class="info-label">{{ t('home.gpu.memory') }}</span>
                <span class="info-value">{{ formatMB(gpu.memoryMb) }}</span>
              </div>
              <div class="info-item">
                <span class="info-label">{{ t('home.gpu.driver') }}</span>
                <span class="info-value">{{ gpu.driverVersion }}</span>
              </div>
            </div>
          </div>
          <div v-else class="info-empty">
            <span>{{ t('home.gpu.none') }}</span>
          </div>
        </section>

        <!-- CUDA Card -->
        <section class="info-section">
          <h2 class="section-title">
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <polygon points="12 2 22 8.5 22 15.5 12 22 2 15.5 2 8.5 12 2"/><line x1="12" y1="22" x2="12" y2="15.5"/><polyline points="22 8.5 12 15.5 2 8.5"/>
            </svg>
            {{ t('home.cuda') }}
          </h2>
          <div class="info-grid">
            <div class="info-item">
              <span class="info-label">{{ t('home.cuda.status') }}</span>
              <span class="info-value">
                <span class="status-badge" :class="info.cuda.available ? 'available' : 'unavailable'">
                  {{ info.cuda.available ? t('home.cuda.available') : t('home.cuda.unavailable') }}
                </span>
              </span>
            </div>
            <div class="info-item">
              <span class="info-label">{{ t('home.cuda.driver') }}</span>
              <span class="info-value">{{ info.cuda.driverVersion || 'N/A' }}</span>
            </div>
            <div class="info-item">
              <span class="info-label">{{ t('home.cuda.toolkit') }}</span>
              <span class="info-value">{{ info.cuda.toolkitVersion || t('home.cuda.notInstalled') }}</span>
            </div>
          </div>
        </section>

        <!-- OS Info -->
        <section class="info-section">
          <h2 class="section-title">
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <rect x="2" y="3" width="20" height="14" rx="2" ry="2"/><line x1="8" y1="21" x2="16" y2="21"/><line x1="12" y1="17" x2="12" y2="21"/>
            </svg>
            {{ t('home.os') }}
          </h2>
          <div class="info-grid">
            <div class="info-item">
              <span class="info-label">{{ t('home.os.name') }}</span>
              <span class="info-value">{{ osLabel }}</span>
            </div>
            <div class="info-item">
              <span class="info-label">{{ t('home.os.arch') }}</span>
              <span class="info-value">{{ info.arch }}</span>
            </div>
          </div>
        </section>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { getCPU, getMemory, getGPU, getCUDA, getOS } from '../wails'
import { t } from '../lib/i18n'
import { usagePercent } from '../lib/format'

interface SystemInfo {
  os: string
  arch: string
  cpu: { model: string; cores: number; logicalCpus: number }
  memory: { totalGb: number; freeGb: number }
  gpu: { name: string; memoryMb: number; driverVersion: string }[]
  cuda: { available: boolean; driverVersion: string; toolkitVersion: string }
}

const info = ref<SystemInfo>({
  os: '', arch: '',
  cpu: { model: '', cores: 0, logicalCpus: 0 },
  memory: { totalGb: 0, freeGb: 0 },
  gpu: [],
  cuda: { available: false, driverVersion: '', toolkitVersion: '' }
})
const sectionsReady = reactive({
  cpu: false, memory: false, gpu: false, cuda: false, os: false
})
const loading = ref(true)
const error = ref('')

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

async function fetchSystemInfo() {
  loading.value = true
  error.value = ''

  const fetchers: [() => Promise<any>, keyof typeof sectionsReady, (data: any) => void][] = [
    [getCPU, 'cpu', (d) => { info.value.cpu = d }],
    [getMemory, 'memory', (d) => { info.value.memory = d }],
    [getGPU, 'gpu', (d) => { info.value.gpu = d || [] }],
    [getCUDA, 'cuda', (d) => { info.value.cuda = d }],
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
  fetchSystemInfo()
})
</script>

<style scoped>
.page {
  /* No top padding: header flush with content top, title aligns with sidebar logo (see global.css .page-header) */
  padding: 0 48px 60px;
}

.page-header {
  /* Use padding instead of margin: header background covers this gap so content scrolls without leaving a seam */
  padding-bottom: 36px;
}

.page-title {
  font-size: 28px;
  font-weight: 700;
  color: var(--text-primary);
  margin: 0 0 4px;
  letter-spacing: -0.5px;
  line-height: 1.2;
}

.page-subtitle {
  font-size: 14px;
  color: var(--text-dim);
  margin: 0;
}

/* ─── Section ─── */
.info-section {
  margin-bottom: 0;
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
  grid-template-columns: repeat(auto-fit, minmax(140px, 1fr));
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

/* ─── Empty ─── */
.info-empty {
  padding: 16px 0;
  color: var(--text-dim);
  font-size: 13px;
  text-align: center;
}

/* ─── Loading skeleton ─── */
.loading-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 20px;
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

/* ─── Cards grid ─── */
.cards-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 20px;
}

/* ─── Memory usage bar ─── */
.memory-usage {
  margin-bottom: 16px;
}

.memory-usage-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
  font-size: 13px;
}

.memory-usage-label {
  color: var(--text-dim);
}

.memory-usage-pct {
  font-weight: 700;
  color: #a78bfa;
  font-size: 13px;
}

.usage-bar {
  height: 6px;
  background: var(--overlay-8);
  border-radius: 3px;
  overflow: hidden;
}

.usage-fill {
  height: 100%;
  border-radius: 3px;
  background: linear-gradient(90deg, #6366f1, #a78bfa);
  transition: width 0.3s ease;
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
</style>
