<template>
  <div class="page">
    <div class="sticky-top">
      <div class="page-header">
        <div class="page-header-text">
          <h1 class="page-title">{{ t('models.title') }}</h1>
          <p class="page-subtitle">
            <template v-if="models.length">{{ t('models.count', { n: models.length }) }}</template>
            <template v-else>{{ t('models.hint') }}</template>
          </p>
        </div>
        <button class="refresh-btn" :disabled="loading" :title="t('models.refreshTitle')" @click="fetchModels(true)">{{ t('models.refresh') }}</button>
      </div>

      <!-- Models directory sources -->
      <div class="dir-bar">
        <div class="dir-sources">
          <div class="dir-info">
            <span class="dir-label">{{ t('models.downloadDir') }}</span>
            <span class="dir-value">{{ downloadDir || t('settings.dirDefaultModels') }}</span>
          </div>
          <div class="dir-info">
            <span class="dir-label">{{ t('models.importDir') }}</span>
            <span class="dir-value" :class="{ 'dir-empty': !modelsDir }">{{ modelsDir || t('models.dirNotSet') }}</span>
          </div>
        </div>
        <button class="dir-btn" :title="t('models.chooseDirTitle')" @click="chooseModelsDir">{{ t('models.chooseDir') }}</button>
      </div>

      <!-- Auto-tune inline feedback (below the directory bar, auto-cleared) -->
      <div v-if="tuneHint" class="tune-hint">{{ tuneHint }}</div>
      <div v-else-if="tuneError" class="tune-hint tune-hint-error">{{ tuneError }}</div>
    </div>

    <!-- Loading skeleton -->
    <div v-if="loading" class="loading-grid">
      <div v-for="i in 3" :key="i" class="skeleton-card">
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
      <h2>{{ t('models.errorTitle') }}</h2>
      <p>{{ error }}</p>
      <button class="retry-btn" @click="fetchModels()">{{ t('home.retry') }}</button>
    </div>

    <!-- Empty state -->
    <div v-else-if="models.length === 0" class="empty-state">
      <div class="empty-icon">
        <svg width="64" height="64" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1" stroke-linecap="round" stroke-linejoin="round">
          <path d="M21 16V8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16z"/>
          <polyline points="3.27 6.96 12 12.01 20.73 6.96"/>
          <line x1="12" y1="22.08" x2="12" y2="12"/>
        </svg>
      </div>
      <h2>{{ t('models.emptyTitle') }}</h2>
      <p>{{ t('models.emptyHint', { dir: downloadDir || t('settings.dirDefaultModels') }) }}</p>
      <button class="empty-cta" @click="router.push('/downloads')">{{ t('action.gotoDownloads') }}</button>
    </div>

    <!-- Model list -->
    <div v-else class="model-list">
      <div v-for="model in models" :key="model.path" class="model-card">
        <div class="model-icon">
          <svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
            <path d="M21 16V8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16z"/>
          </svg>
        </div>
        <div class="model-info">
          <div class="model-header">
            <h3 class="model-name">{{ model.name }}</h3>
            <span v-if="model.hasMmproj" class="mmproj-badge" :title="t('models.multimodalTitle')">👁️ {{ t('models.multimodal') }}</span>
            <div class="model-actions">
              <button class="model-settings-btn" :title="t('models.settings')" @click.stop="openSettings(model)">
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                  <circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-2 2 2 2 0 0 1-2-2v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1-2-2 2 2 0 0 1 2-2h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 2-2 2 2 0 0 1 2 2v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0-2.83l-.06-.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 2 2 2 2 0 0 1-2 2h-.09a1.65 1.65 0 0 0-1.51 1z"/>
                </svg>
              </button>
              <button
                class="model-settings-btn model-tune-btn"
                :class="{ 'tune-busy': tuningId === model.name }"
                :disabled="tuningId !== ''"
                :title="t('models.tune')"
                @click.stop="tuneModel(model)"
              >
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                  <path d="M12 3l1.9 5.8a2 2 0 0 0 1.3 1.3L21 12l-5.8 1.9a2 2 0 0 0-1.3 1.3L12 21l-1.9-5.8a2 2 0 0 0-1.3-1.3L3 12l5.8-1.9a2 2 0 0 0 1.3-1.3L12 3z"/>
                  <path d="M5 3v4"/><path d="M3 5h4"/><path d="M19 17v4"/><path d="M17 19h4"/>
                </svg>
              </button>
            </div>
          </div>
          <div class="model-author" v-if="model.author">{{ model.author }}</div>
          <div class="source-badge" :class="model.sourceDir === downloadDir ? 'source-download' : 'source-import'">
            {{ model.sourceDir === downloadDir ? t('models.sourceDownload') : t('models.sourceImport') }}
          </div>
          <div class="model-meta">
            <span v-if="model.architecture && model.architecture !== '-'" class="meta-tag arch-tag">{{ model.architecture }}</span>
            <span v-if="model.quantization && model.quantization !== '-'" class="meta-tag quant-tag">{{ model.quantization }}</span>
            <span class="meta-tag size-tag">{{ model.sizeHuman }}</span>
          </div>
          <div class="model-path">{{ model.path }}</div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import type { ModelConfig } from '../views/ModelSettings.vue'
import { getModels, getModelConfig, saveModelConfig, refreshModels, getConfig, browseModelsDir, tuneModelConfig } from '../wails'
import { t } from '../lib/i18n'

const router = useRouter()

interface ModelInfo {
  author: string
  name: string
  path: string
  sizeBytes: number
  sizeHuman: string
  architecture: string
  quantization: string
  hasMmproj: boolean
  sourceDir: string
}

const models = ref<ModelInfo[]>([])
const loading = ref(true)
const error = ref('')

// Model sources shown in the directory bar: the model download path (where new
// downloads land) and the imported model directory (reuse of existing models).
const downloadDir = ref('')
const modelsDir = ref('')

async function loadModelsDir() {
  try {
    const cfg = await getConfig()
    downloadDir.value = cfg.modelDownloadDir || ''
    modelsDir.value = cfg.modelsDir || ''
  } catch {}
}

async function chooseModelsDir() {
  try {
    const dir = await browseModelsDir()
    // Only update display and rescan when user actually picks a dir (non-empty); cancel (empty string) leaves existing dir unchanged
    if (dir) {
      modelsDir.value = dir
      await fetchModels(true)
    }
  } catch {}
}

// Settings: navigate to the dedicated settings page for this model
function openSettings(model: ModelInfo) {
  router.push('/models/settings/' + encodeURIComponent(model.name))
}

// ─── Auto-tune ───────────────────────────────────────────────────────────────

// One tune runs at a time: tuningId holds the busy model's name (empty = idle).
const tuningId = ref('')
// Inline feedback below the directory bar; each auto-clears after 5 seconds.
const tuneHint = ref('')
const tuneError = ref('')
let tuneHintTimer: ReturnType<typeof setTimeout> | undefined
let tuneErrorTimer: ReturnType<typeof setTimeout> | undefined

function clearTuneTimers() {
  if (tuneHintTimer) clearTimeout(tuneHintTimer)
  if (tuneErrorTimer) clearTimeout(tuneErrorTimer)
}

function showTuneMessage(err: string, msg: string) {
  clearTuneTimers()
  if (err) {
    tuneHint.value = ''
    tuneError.value = err
    tuneErrorTimer = setTimeout(() => { tuneError.value = '' }, 5000)
  } else {
    tuneError.value = ''
    tuneHint.value = msg
    tuneHintTimer = setTimeout(() => { tuneHint.value = '' }, 5000)
  }
}

// Auto-tune: backend reads the GGUF metrics + hardware snapshot, computes and
// persists the optimal params; the applied values are surfaced inline.
async function tuneModel(model: ModelInfo) {
  if (tuningId.value) return
  tuningId.value = model.name
  try {
    const cfg = (await tuneModelConfig(model.name)) as ModelConfig
    showTuneMessage('', t('models.tuned', {
      gpu: cfg.gpuLayers,
      ctx: cfg.ctxSize,
      cache: cfg.cacheTypeK || 'f16',
      threads: cfg.threads,
    }))
  } catch (e) {
    showTuneMessage(t('models.tuneError', { msg: e instanceof Error ? e.message : String(e) }), '')
  } finally {
    tuningId.value = ''
  }
}

async function fetchModels(force = false) {
  loading.value = true
  error.value = ''
  try {
    // force=true forces a rescan (refreshModels); otherwise uses cache (getModels) (#18)
    models.value = (force ? await refreshModels() : await getModels()) as ModelInfo[]
  } catch (e) {
    error.value = t('models.backendError', { msg: e instanceof Error ? e.message : String(e) })
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  loadModelsDir()
  fetchModels()
})

// Pending auto-clear timers must not fire after the page is left
onUnmounted(clearTuneTimers)
</script>

<style scoped>
.page {
  /* No top padding: header flush with content top, title aligns with sidebar logo (see global.css .page-header) */
  padding: 0 48px 60px;
}

.page-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  /* Use padding instead of margin: header background covers this gap so content scrolls without leaving a seam */
  padding-bottom: 36px;
}

.page-header-text {
  min-width: 0;
}

.refresh-btn {
  padding: 8px 18px;
  background: rgba(99, 102, 241, 0.15);
  color: #a78bfa;
  border: 1px solid rgba(99, 102, 241, 0.25);
  border-radius: 8px;
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s;
  flex-shrink: 0;
  white-space: nowrap;
}

.refresh-btn:hover:not(:disabled) {
  background: rgba(99, 102, 241, 0.25);
  border-color: rgba(99, 102, 241, 0.4);
}

.refresh-btn:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}

/* ─── Models directory bar ─── */
.dir-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 24px;
  padding: 12px 16px;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 12px;
}

.dir-sources {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 0;
}

.dir-info {
  display: flex;
  align-items: baseline;
  gap: 10px;
  min-width: 0;
}

.dir-label {
  font-size: 12px;
  color: var(--text-dim);
  flex-shrink: 0;
}

.dir-value {
  font-size: 12px;
  color: var(--text-muted);
  font-family: 'JetBrains Mono', 'Fira Code', monospace;
  word-break: break-all;
}

.dir-empty {
  color: var(--text-dim);
  font-style: italic;
}

.dir-btn {
  padding: 6px 16px;
  background: var(--border-light);
  color: var(--text-muted);
  border: 1px solid var(--overlay-10);
  border-radius: 8px;
  font-size: 12px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.2s;
  flex-shrink: 0;
  white-space: nowrap;
}

.dir-btn:hover {
  background: var(--overlay-8);
  border-color: var(--scrollbar-thumb-hover);
  color: var(--text-primary);
}

/* ─── Auto-tune inline feedback (below the directory bar) ─── */
.tune-hint {
  margin: -12px 0 16px;
  padding: 10px 16px;
  background: var(--surface);
  border: 1px solid var(--border);
  border-left: 3px solid var(--success);
  border-radius: var(--radius-sm);
  color: var(--success);
  font-size: 13px;
  font-weight: 600;
  word-break: break-all;
}

/* Error variant follows this page's existing error palette (see .error-card):
 * no --danger token exists in the theme vocabulary. */
.tune-hint-error {
  border-left-color: rgba(239, 68, 68, 0.6);
  color: rgba(239, 68, 68, 0.9);
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

.page-subtitle code {
  background: var(--border);
  padding: 2px 6px;
  border-radius: 4px;
  font-size: 13px;
}

/* ─── Model list ─── */
.model-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.model-card {
  display: flex;
  align-items: flex-start;
  gap: 18px;
  padding: 20px 24px;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 14px;
  transition: all 0.2s ease;
}

.model-card:hover {
  background: var(--border-light);
  border-color: var(--overlay-10);
  transform: translateX(4px);
}

.model-icon {
  width: 48px;
  height: 48px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(99, 102, 241, 0.12);
  border-radius: 12px;
  color: #a78bfa;
  flex-shrink: 0;
  margin-top: 2px;
}

.model-info {
  flex: 1;
  min-width: 0;
}

.model-header {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 4px;
}

/* Both header icon buttons group at the row's end: the wrapper owns
 * margin-left:auto so two auto margins cannot split the free space between
 * them (which pushed the settings icon into the middle of the row). */
.model-actions {
  margin-left: auto;
  display: flex;
  align-items: center;
  gap: 4px;
  flex-shrink: 0;
}

.model-settings-btn {
  width: 30px;
  height: 30px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: none;
  border: none;
  color: var(--text-dim);
  cursor: pointer;
  border-radius: 6px;
  /* Faintly visible at rest so the actions are discoverable without hover;
     the tooltip titles label them. Full opacity on card hover. */
  opacity: 0.55;
  transition: opacity 0.2s, background 0.15s, color 0.15s;
  flex-shrink: 0;
}

.model-card:hover .model-settings-btn {
  opacity: 1;
}

.model-settings-btn:hover {
  background: var(--hover-bg);
  color: var(--accent-light);
}

/* Tune button: spinning sparkle while the backend computes the plan */
.model-tune-btn.tune-busy svg {
  animation: tune-spin 1s linear infinite;
}

.model-tune-btn:disabled {
  cursor: wait;
}

@keyframes tune-spin {
  to { transform: rotate(360deg); }
}

.model-name {
  font-size: 16px;
  font-weight: 600;
  color: var(--text-secondary);
  margin: 0;
  word-break: break-word;
  line-height: 1.4;
}

.mmproj-badge {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 2px 10px;
  background: rgba(168, 85, 247, 0.12);
  color: #c084fc;
  border: 1px solid rgba(168, 85, 247, 0.2);
  border-radius: 20px;
  font-size: 11px;
  font-weight: 600;
  white-space: nowrap;
  flex-shrink: 0;
}

.model-author {
  font-size: 12px;
  color: var(--text-dim);
  margin-bottom: 6px;
}

.source-badge {
  display: inline-flex;
  align-items: center;
  padding: 2px 10px;
  border-radius: 20px;
  font-size: 11px;
  font-weight: 600;
  margin-bottom: 8px;
}

.source-download {
  background: rgba(99, 102, 241, 0.1);
  color: var(--accent-light);
  border: 1px solid rgba(99, 102, 241, 0.22);
}

.source-import {
  background: rgba(168, 85, 247, 0.1);
  color: #c084fc;
  border: 1px solid rgba(168, 85, 247, 0.2);
}

.model-meta {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
  margin-bottom: 8px;
}

.meta-tag {
  display: inline-flex;
  align-items: center;
  padding: 3px 10px;
  border-radius: 6px;
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.3px;
}

.arch-tag {
  background: rgba(59, 130, 246, 0.12);
  color: #60a5fa;
  border: 1px solid rgba(59, 130, 246, 0.2);
}

.quant-tag {
  background: rgba(34, 197, 94, 0.1);
  color: #4ade80;
  border: 1px solid rgba(34, 197, 94, 0.18);
}

.size-tag {
  background: var(--border-light);
  color: var(--text-muted);
  border: 1px solid var(--overlay-8);
}

.model-path {
  font-size: 11px;
  color: var(--overlay-20);
  font-family: 'JetBrains Mono', 'Fira Code', monospace;
  /* Single-line ellipsis: long model paths truncate at the end instead of wrapping */
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  line-height: 1.4;
}

/* ─── Empty ─── */
.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 56px 32px;
  background: linear-gradient(135deg, rgba(99, 102, 241, 0.04) 0%, rgba(167, 139, 250, 0.02) 100%);
  border: 1px solid var(--border);
  border-radius: 16px;
  text-align: center;
}

.empty-icon {
  color: rgba(99, 102, 241, 0.3);
  margin-bottom: 20px;
}

.empty-state h2 {
  font-size: 18px;
  font-weight: 600;
  color: var(--text-secondary);
  margin: 0 0 8px;
}

.empty-state p {
  font-size: 13px;
  color: var(--text-dim);
  margin: 0 0 20px;
  max-width: 420px;
  line-height: 1.6;
}

.empty-state code {
  background: var(--border);
  padding: 2px 6px;
  border-radius: 4px;
  font-size: 12px;
  color: var(--text-secondary);
}

/* Empty-state CTA: jump to the downloads page */
.empty-cta {
  padding: 8px 24px;
  background: var(--active-bg);
  color: var(--accent-light);
  border: 1px solid transparent;
  border-radius: 8px;
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  transition: background 0.2s;
}

.empty-cta:hover {
  background: var(--accent-glow);
}

/* ─── Loading skeleton ─── */
.loading-grid {
  display: flex;
  flex-direction: column;
  gap: 12px;
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
</style>
