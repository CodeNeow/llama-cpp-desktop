<template>
  <div class="page page-fixed">
    <div class="sticky-top">
      <div class="page-header">
        <div class="page-header-row">
          <div>
            <h1 class="page-title">{{ t('home.title') }}</h1>
            <p class="page-subtitle">{{ t('home.subtitle') }}</p>
          </div>
          <div class="header-actions">
            <span v-if="lastUpdated" class="updated-at">{{ t('home.updatedAt', { time: lastUpdated }) }}</span>
            <button class="refresh-btn" :disabled="refreshing" @click="fetchSystemInfo(true)">
              <svg :class="{ spinning: refreshing }" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                <polyline points="23 4 23 10 17 10"/><polyline points="1 20 1 14 7 14"/>
                <path d="M3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15"/>
              </svg>
              {{ refreshing ? t('home.refreshing') : t('home.refresh') }}
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- Scrollable content band: only this region scrolls, never the page -->
    <div class="page-scroll">
      <!-- Loading skeleton -->
      <div v-if="loading" class="cards-grid">
        <div v-for="i in 6" :key="i" class="info-section skeleton-card">
          <div class="skeleton-line skeleton-title"></div>
          <div class="skeleton-line"></div>
          <div class="skeleton-line skeleton-short"></div>
        </div>
      </div>

      <!-- Error state: every core probe failed (e.g. backend bridge missing) -->
      <div v-else-if="loadError" class="error-card">
        <div class="error-icon">
          <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
            <circle cx="12" cy="12" r="10"/><line x1="12" y1="8" x2="12" y2="12"/><line x1="12" y1="16" x2="12.01" y2="16"/>
          </svg>
        </div>
        <h2>{{ t('home.errorTitle') }}</h2>
        <p>{{ loadError }}</p>
        <button class="retry-btn" @click="fetchSystemInfo(true)">{{ t('home.retry') }}</button>
      </div>

      <!-- Data -->
      <template v-else>
        <div class="cards-grid">
          <!-- Quick-start checklist: full-width card, hides once every step
               completes or the user dismisses it -->
          <section v-if="onboardingView.visible" class="info-section onboarding-card">
            <div class="onboarding-head">
              <h2 class="section-title">
                <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                  <path d="M4 15s1-1 4-1 5 2 8 2 4-1 4-1V3s-1 1-4 1-5-2-8-2-4 1-4 1z"/><line x1="4" y1="22" x2="4" y2="15"/>
                </svg>
                {{ t('onboarding.title') }}
              </h2>
              <button class="onboarding-dismiss" @click="dismissOnboarding">{{ t('onboarding.dismiss') }}</button>
            </div>
            <ol class="onboarding-steps">
              <li
                v-for="(step, idx) in onboardingView.steps"
                :key="step.id"
                class="onboarding-step"
                :class="{ done: step.done }"
              >
                <span class="step-marker">
                  <svg v-if="step.done" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round" stroke-linejoin="round">
                    <polyline points="20 6 9 17 4 12"/>
                  </svg>
                  <template v-else>{{ idx + 1 }}</template>
                </span>
                <span class="step-label">{{ t(ONBOARDING_LABELS[step.id]) }}</span>
                <button v-if="!step.done" class="step-action" @click="goStep(step.route)">
                  {{ t('onboarding.goto') }}
                  <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                    <line x1="5" y1="12" x2="19" y2="12"/><polyline points="12 5 19 12 12 19"/>
                  </svg>
                </button>
                <span v-else class="step-done-label">{{ t('onboarding.done') }}</span>
              </li>
            </ol>
          </section>

          <!-- GPU Card -->
          <section class="info-section">
            <h2 class="section-title">
              <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                <rect x="2" y="3" width="20" height="14" rx="2" ry="2"/><line x1="8" y1="21" x2="16" y2="21"/><line x1="12" y1="17" x2="12" y2="21"/>
              </svg>
              {{ t('home.gpu') }}
              <span v-if="multiGpu" class="title-chip">×{{ gpuViews.length }}</span>
            </h2>
            <div v-if="gpuViews.length > 0">
              <!-- Aggregate VRAM across GPUs: the number that matters for model offloading -->
              <div v-if="multiGpu && vramTotals" class="usage-block">
                <div class="usage-row">
                  <span class="usage-name">{{ t('home.gpu.vramTotal') }}</span>
                  <span class="usage-caption">{{ formatMB(vramTotals.usedMb) }} / {{ formatMB(vramTotals.totalMb) }}</span>
                  <span class="usage-pct">{{ t('home.usagePercent', { pct: usagePercent(vramTotals.usedMb, vramTotals.totalMb) }) }}</span>
                </div>
                <div
                  class="usage-bar"
                  role="progressbar"
                  :aria-valuenow="usagePercent(vramTotals.usedMb, vramTotals.totalMb)"
                  aria-valuemin="0"
                  aria-valuemax="100"
                  :aria-label="t('home.gpu.vramTotal')"
                >
                  <div class="usage-fill" :style="{ width: usagePercent(vramTotals.usedMb, vramTotals.totalMb) + '%' }"></div>
                </div>
              </div>
              <div
                v-for="(gpu, i) in gpuViews"
                :key="i"
                class="gpu-block"
                :class="{ divided: i > 0 }"
              >
                <div class="usage-row">
                  <span class="usage-name gpu-name" :title="gpu.name">{{ gpu.name }}</span>
                  <span class="usage-caption">{{ formatMB(gpu.usedMb) }} / {{ formatMB(gpu.totalMb) }}</span>
                  <span class="usage-pct">{{ t('home.usagePercent', { pct: usagePercent(gpu.usedMb, gpu.totalMb) }) }}</span>
                </div>
                <div
                  class="usage-bar"
                  role="progressbar"
                  :aria-valuenow="usagePercent(gpu.usedMb, gpu.totalMb)"
                  aria-valuemin="0"
                  aria-valuemax="100"
                  :aria-label="gpu.name"
                >
                  <div class="usage-fill" :style="{ width: usagePercent(gpu.usedMb, gpu.totalMb) + '%' }"></div>
                </div>
                <div class="meta-row">
                  <span>{{ t('home.gpu.memory') }} {{ formatMB(gpu.totalMb) }}</span>
                  <span v-if="gpu.utilPercent !== null">{{ t('home.gpu.util') }} {{ t('home.usagePercent', { pct: gpu.utilPercent }) }}</span>
                  <span>{{ t('home.gpu.computeCap') }} {{ gpu.computeCapability > 0 ? gpu.computeCapability.toFixed(1) : 'N/A' }}</span>
                  <span>{{ t('home.gpu.driver') }} {{ gpu.driverVersion || 'N/A' }}</span>
                </div>
              </div>
            </div>
            <div v-else class="info-empty">
              <span>{{ t('home.gpu.none') }}</span>
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
            <div class="usage-block">
              <div class="usage-row">
                <span class="usage-name">{{ t('home.memory.used') }}</span>
                <span class="usage-caption">{{ formatGB(memoryView.usedGb) }} / {{ formatGB(memoryView.totalGb) }}</span>
                <span class="usage-pct">{{ t('home.usagePercent', { pct: memoryView.pct }) }}</span>
              </div>
              <div
                class="usage-bar"
                role="progressbar"
                :aria-valuenow="memoryView.pct"
                aria-valuemin="0"
                aria-valuemax="100"
                :aria-label="t('home.memory')"
              >
                <div class="usage-fill" :style="{ width: memoryView.pct + '%' }"></div>
              </div>
              <!-- Inside the block on purpose: keeps the bar→details gap at
                   .meta-row's own margin-top, matching the GPU blocks (an
                   outside placement would stack the block's bottom margin) -->
              <div class="meta-row">
                <span>{{ t('home.memory.total') }} {{ formatGB(memoryView.totalGb) }}</span>
                <span>{{ t('home.memory.available') }} {{ formatGB(memoryView.freeGb) }}</span>
              </div>
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
                <span class="info-value">{{ info.cuda.toolkitVersion || (info.cuda.available ? t('home.cuda.na') : t('home.cuda.notInstalled')) }}</span>
              </div>
              <div class="info-item" v-if="info.cuda.available && firstGpuComputeCap > 0">
                <span class="info-label">{{ t('home.cuda.compat') }}</span>
                <!-- warning-text only for 'need': a satisfied floor is good news, not a warning -->
                <span class="info-value" :class="{ 'warning-text': cudaLevel === 'need' }">
                  {{ cudaCompatText }}
                </span>
              </div>
            </div>
          </section>

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
                <span class="info-value">{{ info.cpu.model || 'N/A' }}</span>
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
            <template v-if="liveCpuPct !== null">
              <div class="usage-block">
                <div class="usage-row">
                  <span class="usage-name">{{ t('home.cpu.load') }}</span>
                  <span class="usage-pct">{{ t('home.usagePercent', { pct: liveCpuPct }) }}</span>
                </div>
                <div
                  class="usage-bar"
                  role="progressbar"
                  :aria-valuenow="liveCpuPct"
                  aria-valuemin="0"
                  aria-valuemax="100"
                  :aria-label="t('home.cpu.load')"
                >
                  <div class="usage-fill" :style="{ width: liveCpuPct + '%' }"></div>
                </div>
              </div>
            </template>
          </section>

          <!-- Disk Card -->
          <section class="info-section">
            <h2 class="section-title">
              <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                <rect x="2" y="2" width="20" height="8" rx="2" ry="2"/><rect x="2" y="14" width="20" height="8" rx="2" ry="2"/><line x1="6" y1="6" x2="6.01" y2="6"/><line x1="6" y1="18" x2="6.01" y2="18"/>
              </svg>
              {{ t('home.disk') }}
            </h2>
            <template v-if="diskView">
              <div class="usage-block">
                <div class="usage-row">
                  <span class="usage-name">{{ t('home.disk.used') }}</span>
                  <span class="usage-caption">{{ formatBytes(diskView.used) }} / {{ formatBytes(diskView.total) }}</span>
                  <span class="usage-pct">{{ t('home.usagePercent', { pct: diskPct }) }}</span>
                </div>
                <div
                  class="usage-bar"
                  role="progressbar"
                  :aria-valuenow="diskPct"
                  aria-valuemin="0"
                  aria-valuemax="100"
                  :aria-label="t('home.disk')"
                >
                  <div class="usage-fill" :style="{ width: diskPct + '%' }"></div>
                </div>
                <!-- Inside the block: same bar→details gap as GPU/memory blocks -->
                <div class="meta-row">
                  <span class="meta-path">{{ t('home.disk.path') }} {{ diskView.path }}</span>
                  <span>{{ t('home.disk.free') }} {{ formatBytes(diskView.total - diskView.used) }}</span>
                </div>
              </div>
            </template>
            <div v-else class="info-empty">
              <span>{{ t('home.disk.notAvailable') }}</span>
            </div>
          </section>

          <!-- System Card -->
          <section class="info-section">
            <h2 class="section-title">
              <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                <rect x="3" y="4" width="18" height="12" rx="2"/><line x1="2" y1="20" x2="22" y2="20"/>
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
                <span class="info-value">{{ info.arch || 'N/A' }}</span>
              </div>
              <div class="info-item">
                <span class="info-label">{{ t('home.appVersion') }}</span>
                <span class="info-value">{{ info.appVersion || 'N/A' }}</span>
              </div>
            </div>
          </section>
        </div>
      </template>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { getCPU, getMemory, getGPU, getCUDA, getOS, getDisk, getLlamaCpp, getModels, getMonitorStatus, getAppVersion } from '../wails'
import { t } from '../lib/i18n'
import { usagePercent, formatGB, formatMB, formatBytes } from '../lib/format'
import { aggregateVram, buildGpuDisplays, cudaCompatLevel, type GpuStaticInfo } from '../lib/sysinfo'
import { buildOnboardingView, type OnboardingStepId } from '../lib/onboarding'
import type { MonitorStatus } from '../lib/monitor'
import { appConfig, setOnboardingDismissed } from '../store'

interface SystemInfo {
  os: string
  arch: string
  cpu: { model: string; cores: number; logicalCpus: number }
  memory: { totalGb: number; freeGb: number }
  gpu: GpuStaticInfo[]
  cuda: { available: boolean; driverVersion: string; toolkitVersion: string }
  disk: { path: string; used: number; total: number } | null
  appVersion: string
}

/** Live metrics sampled periodically via GetMonitorStatus (byte units); falls back to the static snapshot. */
interface LiveSnapshot {
  cpuPercent: number | null
  memUsedBytes: number
  memTotalBytes: number
  gpus: MonitorStatus['gpus']
  disk: { path: string; used: number; total: number } | null
  serverRunning: boolean
}

// Quick-start checklist facts probed alongside the system info
const runtimeInstalled = ref(false)
const hasModels = ref(false)
// CUDA major family of the installed cudart runtime ("" when unknown); also
// feeds the three-state CUDA compat row below
const cudartVersion = ref('')

const router = useRouter()

const ONBOARDING_LABELS: Record<OnboardingStepId, string> = {
  runtime: 'onboarding.step.runtime',
  models: 'onboarding.step.models',
  service: 'onboarding.step.service'
}

const info = ref<SystemInfo>({
  os: '', arch: '',
  cpu: { model: '', cores: 0, logicalCpus: 0 },
  memory: { totalGb: 0, freeGb: 0 },
  gpu: [],
  cuda: { available: false, driverVersion: '', toolkitVersion: '' },
  disk: null,
  appVersion: ''
})
const live = ref<LiveSnapshot | null>(null)
const loading = ref(true)
const refreshing = ref(false)
const loadError = ref('')
const lastUpdated = ref('')

let pollTimer: ReturnType<typeof setInterval> | null = null

// ─── Derived views ───────────────────────────────────────────────

const gpuViews = computed(() => buildGpuDisplays(info.value.gpu, live.value?.gpus ?? null))
const multiGpu = computed(() => gpuViews.value.length > 1)
const vramTotals = computed(() => aggregateVram(gpuViews.value))

const memoryView = computed(() => {
  const l = live.value
  if (l && l.memTotalBytes > 0) {
    const gb = 1024 ** 3
    const usedGb = l.memUsedBytes / gb
    const totalGb = l.memTotalBytes / gb
    return {
      usedGb,
      totalGb,
      freeGb: Math.max(0, totalGb - usedGb),
      pct: usagePercent(l.memUsedBytes, l.memTotalBytes)
    }
  }
  const total = info.value.memory.totalGb
  const free = info.value.memory.freeGb
  return {
    usedGb: Math.max(0, total - free),
    totalGb: total,
    freeGb: free,
    pct: usagePercent(total - free, total)
  }
})

const diskView = computed(() => live.value?.disk ?? info.value.disk)
const diskPct = computed(() => {
  const d = diskView.value
  return d ? usagePercent(d.used, d.total) : 0
})

const liveCpuPct = computed(() => {
  const p = live.value?.cpuPercent
  return p === null || p === undefined ? null : Math.round(p * 10) / 10
})

const osLabel = computed(() => {
  const labels: Record<string, string> = {
    windows: 'Windows',
    linux: 'Linux',
    darwin: 'macOS'
  }
  return labels[info.value.os] || info.value.os || 'N/A'
})

const firstGpuComputeCap = computed(() => info.value.gpu[0]?.computeCapability ?? 0)
const cudaLevel = computed(() => cudaCompatLevel(firstGpuComputeCap.value, cudartVersion.value))

// Three-state CUDA compat row text: 'satisfied' composes the detected CUDA
// major family; the version itself stays untranslated so only the label goes
// through i18n
const cudaCompatText = computed(() => {
  switch (cudaLevel.value) {
    case 'satisfied':
      return `${t('home.cuda.compatSatisfied')} · CUDA ${cudartVersion.value}`
    case 'need':
      return t('home.cuda.compatBlackwell')
    default:
      return t('home.cuda.compatOk')
  }
})

// Quick-start checklist: derives visibility/steps from probed facts + persisted dismissal
const onboardingView = computed(() => buildOnboardingView({
  runtimeInstalled: runtimeInstalled.value,
  hasModels: hasModels.value,
  serviceRunning: live.value?.serverRunning === true,
  dismissed: appConfig.onboardingDismissed
}))

// Auto-complete: once every step is satisfied, persist the dismissal so the
// card never reappears after the user later stops the service
watch(() => onboardingView.value.allDone, (allDone) => {
  if (allDone && !appConfig.onboardingDismissed) {
    setOnboardingDismissed(true)
  }
})

function dismissOnboarding() {
  setOnboardingDismissed(true)
}

function goStep(route: string) {
  router.push(route)
}

// ─── Data loading ────────────────────────────────────────────────

function stampUpdated() {
  lastUpdated.value = new Date().toLocaleTimeString([], { hour12: false })
}

/**
 * Full probe of the static system info (hardware identity, versions).
 * Probes run in parallel and settle independently; partial failures render
 * as N/A values while an all-fail result switches to the error card.
 */
async function fetchSystemInfo(manual = false) {
  if (manual) {
    refreshing.value = true
  } else {
    loading.value = true
  }
  loadError.value = ''

  // Best-effort, not part of the all-failed detection (cosmetic field)
  getAppVersion()
    .then(v => { info.value.appVersion = v })
    .catch(() => {})
  // Quick-start checklist facts: best-effort probes (cosmetic as well)
  // The getLlamaCpp probe also feeds the CUDA compat row via cudartVersion —
  // it must stay unconditional (not gated behind onboarding visibility) so the
  // compat row has data even when the checklist is dismissed
  getLlamaCpp()
    .then(d => {
      runtimeInstalled.value = d?.installed === true
      cudartVersion.value = typeof d?.cudartVersion === 'string' ? d.cudartVersion : ''
    })
    .catch(() => {})
  getModels()
    .then(list => { hasModels.value = Array.isArray(list) && list.length > 0 })
    .catch(() => {})

  // Each probe applies its result with an explicit narrowing cast: the Wails
  // bridge is untyped (Promise<any>), so the target shape comes from SystemInfo.
  const probes: [Promise<unknown>, (data: unknown) => void][] = [
    [getCPU(), d => { info.value.cpu = d as SystemInfo['cpu'] }],
    [getMemory(), d => { info.value.memory = d as SystemInfo['memory'] }],
    [getGPU(), d => { info.value.gpu = (d as GpuStaticInfo[] | null) || [] }],
    [getCUDA(), d => { info.value.cuda = d as SystemInfo['cuda'] }],
    [getOS(), d => { const o = d as { os: string; arch: string }; info.value.os = o.os; info.value.arch = o.arch }],
    [getDisk(), d => { info.value.disk = d as SystemInfo['disk'] }]
  ]

  const settled = await Promise.allSettled(probes.map(([p]) => p))
  probes.forEach(([, setter], i) => {
    const r = settled[i]
    if (r.status === 'fulfilled') setter(r.value)
  })

  const firstFailure = settled.find((r): r is PromiseRejectedResult => r.status === 'rejected')
  if (firstFailure && settled.every(r => r.status === 'rejected')) {
    const reason = firstFailure.reason
    loadError.value = t('home.backendError', { msg: reason instanceof Error ? reason.message : String(reason ?? '') })
  }

  loading.value = false
  refreshing.value = false
  stampUpdated()
}

/** Poll live metrics (RAM / VRAM / utilization / disk) while the page is mounted. */
async function pollLive() {
  try {
    const s = await getMonitorStatus()
    live.value = {
      cpuPercent: Number.isFinite(s.cpuPercent) ? s.cpuPercent : null,
      memUsedBytes: s.memUsed,
      memTotalBytes: s.memTotal,
      gpus: Array.isArray(s.gpus) ? s.gpus : [],
      disk: s.disk ?? null,
      serverRunning: s.serverRunning === true
    }
    stampUpdated()
  } catch {
    // Transient sampling failure: keep displaying the last good sample
  }
}

onMounted(() => {
  fetchSystemInfo()
  pollLive()
  pollTimer = setInterval(pollLive, 3000)
})

onUnmounted(() => {
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
})
</script>

<style scoped>
.page {
  /* No top padding: header flush with content top, title aligns with sidebar logo (see global.css .page-header).
     No bottom padding either: the fixed layout (page-fixed) hands bottom spacing
     to the .page-scroll band below */
  padding: 0 48px;
}

/* Fixed-viewport layout (see .page-fixed in global.css): the header band stays
   pinned; only the content band below scrolls */
.page-fixed .sticky-top {
  flex-shrink: 0;
}

.page-header {
  /* Use padding instead of margin: header background covers this gap so content scrolls without leaving a seam */
  padding-bottom: 36px;
}

/* Scrollable content band: absorbs the remaining viewport height and scrolls
   internally, so the page itself never scrolls */
.page-scroll {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  /* Bottom clearance so the last row of cards clears the floating TaskDock
     pill (--dock-reserve is bound globally by App.vue; 0 while hidden) */
  padding-bottom: calc(24px + var(--dock-reserve, 0px));
}

/* Thin scrollbar matching .content-area (App.vue) so the inner band does not
   render a full-width system scrollbar */
.page-scroll::-webkit-scrollbar {
  width: 6px;
}

.page-scroll::-webkit-scrollbar-track {
  background: transparent;
}

.page-scroll::-webkit-scrollbar-thumb {
  background: var(--overlay-10);
  border-radius: 3px;
}

.page-scroll::-webkit-scrollbar-thumb:hover {
  background: var(--scrollbar-thumb-hover);
}

.page-header-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-shrink: 0;
}

.updated-at {
  font-size: 12px;
  color: var(--text-dim);
  white-space: nowrap;
  font-variant-numeric: tabular-nums;
}

.refresh-btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 6px 14px;
  background: transparent;
  color: var(--text-secondary);
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.2s;
}

.refresh-btn:hover:not(:disabled) {
  background: var(--hover-bg);
  border-color: var(--overlay-20);
  color: var(--text-primary);
}

.refresh-btn:disabled {
  opacity: 0.6;
  cursor: default;
}

.refresh-btn svg.spinning {
  animation: spin 0.8s linear infinite;
}

@keyframes spin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
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

.title-chip {
  padding: 0 8px;
  border-radius: 10px;
  background: var(--active-bg);
  color: var(--accent-light);
  font-size: 11px;
  font-weight: 600;
  line-height: 18px;
}

/* ─── Quick-start checklist ─── */
.onboarding-card {
  grid-column: 1 / -1;
}

.onboarding-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
}

.onboarding-head .section-title {
  margin-bottom: 14px;
}

.onboarding-dismiss {
  border: none;
  background: transparent;
  color: var(--text-dim);
  font-size: 12px;
  cursor: pointer;
  padding: 4px 10px;
  border-radius: var(--radius-sm);
  transition: color 0.15s, background 0.15s;
  flex-shrink: 0;
}

.onboarding-dismiss:hover {
  color: var(--text-secondary);
  background: var(--hover-bg);
}

.onboarding-steps {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.onboarding-step {
  display: flex;
  align-items: center;
  gap: 12px;
  font-size: 13px;
  color: var(--text-secondary);
}

.step-marker {
  width: 22px;
  height: 22px;
  border-radius: 50%;
  border: 1px solid var(--border);
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 11px;
  font-weight: 600;
  color: var(--text-dim);
  flex-shrink: 0;
}

.onboarding-step.done .step-marker {
  background: rgba(34, 197, 94, 0.12);
  border-color: transparent;
  color: var(--success);
}

.onboarding-step.done .step-label {
  color: var(--text-dim);
}

.step-action {
  margin-left: auto;
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 4px 12px;
  background: var(--active-bg);
  color: var(--accent-light);
  border: none;
  border-radius: var(--radius-sm);
  font-size: 12px;
  font-weight: 500;
  cursor: pointer;
  flex-shrink: 0;
  transition: background 0.15s;
}

.step-action:hover {
  background: var(--accent-glow);
}

.step-done-label {
  margin-left: auto;
  color: var(--success);
  font-size: 12px;
  font-weight: 600;
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
  /* Break anywhere but without the mid-glyph harshness of break-all for
     long model names / versions */
  overflow-wrap: anywhere;
  font-variant-numeric: tabular-nums;
}

/* ─── Usage bars (shared visual language: RAM / VRAM / disk / CPU) ─── */
.usage-block {
  margin-bottom: 14px;
}

.usage-block:last-child {
  margin-bottom: 0;
}

.usage-row {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
  gap: 12px;
  margin-bottom: 8px;
  font-size: 13px;
  /* Live metrics refresh every few seconds; monospaced digits stop the
     values from jittering horizontally as they change */
  font-variant-numeric: tabular-nums;
}

.usage-name {
  color: var(--text-dim);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.usage-caption {
  color: var(--text-muted);
  font-size: 12px;
  white-space: nowrap;
}

.usage-pct {
  font-weight: 700;
  color: var(--accent-light);
  font-size: 13px;
  white-space: nowrap;
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
  background: linear-gradient(90deg, var(--accent), var(--accent-light));
  transition: width 0.6s ease;
}

.gpu-name {
  color: var(--accent-light);
  font-weight: 600;
}

.meta-row {
  display: flex;
  flex-wrap: wrap;
  gap: 4px 16px;
  font-size: 12px;
  color: var(--text-dim);
  /* Uniform breathing room below the usage bar in every card (GPU blocks
     previously sat flush against it while memory/disk used a .spaced hack) */
  margin-top: 10px;
  font-variant-numeric: tabular-nums;
}

.meta-path {
  word-break: break-all;
}

.gpu-block {
  padding: 14px 0;
}

/* Tight rhythm against the card title / card bottom edge */
.gpu-block:first-child {
  padding-top: 0;
}

.gpu-block:last-child {
  padding-bottom: 0;
}

/* Divider only between consecutive GPU blocks */
.gpu-block.divided {
  border-top: 1px solid var(--border-light);
}

.warning-text {
  color: var(--warning);
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
  color: var(--success);
  border: 1px solid rgba(34, 197, 94, 0.2);
}

.status-badge.unavailable {
  background: rgba(239, 68, 68, 0.1);
  color: var(--danger);
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
.skeleton-card {
  min-height: 140px;
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
  /* Fully elastic: tracks appear only while a 400px card fits, so the grid
     collapses 2 → 1 columns on its own on narrow windows (no media queries).
     auto-fill keeps every card the same width instead of letting the last
     incomplete row stretch. */
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(min(100%, 400px), 1fr));
  gap: 20px;
  align-content: start;
}

/* min-width: 0 lets long unbreakable strings (GPU names, disk paths) shrink
   inside the track instead of blowing the grid past the viewport */
.info-section {
  min-width: 0;
  padding: 24px 28px;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 14px;
  transition: border-color 0.2s;
  display: flex;
  flex-direction: column;
}

.info-section:hover {
  border-color: var(--overlay-10);
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
</style>
