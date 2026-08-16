<template>
  <div class="page">
    <div class="sticky-top">
      <!-- Back + header -->
      <div class="page-header">
        <button class="back-btn" @click="router.back()">
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round">
            <polyline points="15 18 9 12 15 6"/>
          </svg>
          <span>{{ t('downloads.back') }}</span>
        </button>
        <h1 class="page-title">{{ decodedModelId }}</h1>
        <p class="page-subtitle">{{ fileCountLabel }}</p>
      </div>

      <!-- Sticky action bar -->
      <div v-if="files.length > 0 || filesLoading" class="action-bar">
        <span class="selected-count">{{ selectedCountLabel }}</span>
        <div class="action-actions">
          <button
            class="select-all-btn"
            :disabled="files.length === 0"
            @click="toggleSelectAll"
          >
            {{ allSelected ? t('downloads.deselectAll') : t('downloads.selectAll') }}
          </button>
          <button
            class="download-btn"
            :disabled="selectedCount === 0"
            @click="handleDownload"
          >
            {{ t('downloads.downloadSelected') }}
          </button>
        </div>
      </div>
    </div>

    <!-- Error state -->
    <div v-if="loadError" class="error-card">
      <p class="error-text">{{ t('home.errorTitle') }}：{{ loadError }}</p>
      <button class="retry-btn" @click="retry">{{ t('home.retry') }}</button>
    </div>

    <!-- Model description area -->
    <section class="desc-section">
      <h2 class="section-title">{{ t('downloads.descTitle') }}</h2>
      <div v-if="descLoading" class="desc-loading">{{ t('downloads.loadingDesc') }}</div>
      <div v-else-if="description" class="desc-text">{{ description }}</div>
      <div v-else class="desc-empty">{{ t('downloads.noDesc') }}</div>
    </section>

    <!-- File list -->
    <section class="files-section">
      <h2 class="section-title">{{ t('downloads.fileCount', { n: files.length }) }}</h2>
      <div v-if="filesLoading" class="files-loading">{{ t('downloads.loadingFiles') }}</div>
      <div v-else-if="sortedFilesList.length === 0" class="files-empty">{{ t('downloads.noFiles') }}</div>
      <div v-else class="files-list">
        <label
          v-for="f in sortedFilesList"
          :key="f.filename"
          class="file-item"
          :class="{ selected: selectedFiles.includes(f.filename) }"
        >
          <input
            type="checkbox"
            :checked="selectedFiles.includes(f.filename)"
            @change="toggleFile(f.filename)"
          />
          <span class="file-name">{{ f.filename }}</span>
          <span v-if="guessQuant(f.filename)" class="file-quant">{{ guessQuant(f.filename) }}</span>
          <span v-if="f.size" class="file-size">{{ formatBytes(f.size) }}</span>
        </label>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { getModelFiles, getModelDescription, startDownload } from '../wails'
import { sortModelFiles, guessQuant } from '../lib/modelFiles'
import { formatBytes } from '../lib/format'
import { t } from '../lib/i18n'

const router = useRouter()
const route = useRoute()

  // modelId may contain slashes (org/name), encoded with encodeURIComponent in URL
const decodedModelId = computed(() => decodeURIComponent(String(route.params.modelId || '')))
const modelId = computed(() => String(route.params.modelId || ''))

  // File list (interface fields use any to align with wails.ts return shape)
interface ModelFile {
  filename: string
  size?: number
}

const files = ref<ModelFile[]>([])
const filesLoading = ref(false)
const loadError = ref('')

  // Selection state
const selectedFiles = reactive<string[]>([])

  // Description
const description = ref('')
const descLoading = ref(false)

// File count label
const fileCountLabel = computed(() => {
  const n = files.value.length
  return n > 0 ? t('downloads.fileCount', { n }) : ''
})

// Selected count label
const selectedCount = computed(() => selectedFiles.length)
const selectedCountLabel = computed(() => t('downloads.selectedCount', { n: selectedCount.value }))

// Whether all files are selected
const allSelected = computed(() => files.value.length > 0 && selectedFiles.length === files.value.length)

const sortedFilesList = computed(() => sortModelFiles(files.value))

async function loadData() {
  const mid = modelId.value
  if (!mid) {
    loadError.value = 'Missing model ID'
    return
  }

  // Load file list and description in parallel, each with independent error handling
  filesLoading.value = true
  loadError.value = ''

  const filePromise = getModelFiles(mid)
    .then((result: ModelFile[]) => { files.value = result || [] })
    .catch((err: any) => { if (!loadError.value) loadError.value = err?.message || String(err) })
    .finally(() => { filesLoading.value = false })

  const descPromise = getModelDescription(mid)
    .then((text: string) => { if (text) description.value = text })
    .catch(() => {})
    .finally(() => { descLoading.value = false })

  descLoading.value = true
  await Promise.all([filePromise, descPromise])
}

function toggleFile(filename: string) {
  const idx = selectedFiles.indexOf(filename)
  if (idx >= 0) {
    selectedFiles.splice(idx, 1)
  } else {
    selectedFiles.push(filename)
  }
}

function toggleSelectAll() {
  if (allSelected.value) {
    selectedFiles.length = 0
  } else {
    selectedFiles.length = 0
    for (const f of files.value) selectedFiles.push(f.filename)
  }
}

async function handleDownload() {
  if (selectedFiles.length === 0) return
  try {
    await startDownload(modelId.value, [...selectedFiles])
    router.push('/downloads')
  } catch {
    // On failure, silently stay on this page (consistent with existing page behavior)
  }
}

function retry() {
  loadData()
}

onMounted(() => {
  loadData()
})
</script>

<style scoped>
.page {
  padding: 0 48px 60px;
}

.page-header {
  padding-bottom: 20px;
}

.back-btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  background: transparent;
  border: none;
  color: var(--text-muted);
  font-size: 13px;
  cursor: pointer;
  padding: 4px 0;
  margin-bottom: 12px;
  border-radius: 6px;
  transition: color 0.2s;
}

.back-btn:hover {
  color: var(--text-secondary);
}

.page-title {
  font-size: 24px;
  font-weight: 700;
  color: var(--text-primary);
  margin: 0 0 4px;
  letter-spacing: -0.5px;
  line-height: 1.2;
  word-break: break-all;
}

.page-subtitle {
  font-size: 13px;
  color: var(--text-dim);
  margin: 0;
}

/* ─── Action bar ─── */
.action-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 10px 0;
  margin-top: 12px;
  border-top: 1px solid var(--border);
}

.selected-count {
  font-size: 13px;
  color: var(--text-dim);
}

.action-actions {
  display: flex;
  gap: 8px;
}

.select-all-btn {
  padding: 7px 16px;
  background: transparent;
  border: 1px solid var(--overlay-10);
  border-radius: 8px;
  color: var(--text-muted);
  font-size: 13px;
  cursor: pointer;
  transition: all 0.2s;
}

.select-all-btn:hover:not(:disabled) {
  border-color: var(--overlay-20);
  color: var(--text-primary);
}

.select-all-btn:disabled {
  opacity: 0.4;
  cursor: default;
}

.download-btn {
  padding: 7px 22px;
  background: linear-gradient(135deg, rgba(99, 102, 241, 0.2), rgba(167, 139, 250, 0.15));
  color: #a78bfa;
  border: 1px solid rgba(99, 102, 241, 0.3);
  border-radius: 8px;
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s;
}

.download-btn:hover:not(:disabled) {
  background: linear-gradient(135deg, rgba(99, 102, 241, 0.3), rgba(167, 139, 250, 0.25));
}

.download-btn:disabled {
  opacity: 0.4;
  cursor: default;
}

/* ─── Error ─── */
.error-card {
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 14px 18px;
  background: rgba(239, 68, 68, 0.06);
  border: 1px solid rgba(239, 68, 68, 0.15);
  border-radius: 10px;
  margin-bottom: 24px;
}

.error-text {
  flex: 1;
  font-size: 13px;
  color: #f87171;
  margin: 0;
}

.retry-btn {
  padding: 6px 16px;
  background: rgba(99, 102, 241, 0.1);
  color: #a78bfa;
  border: 1px solid rgba(99, 102, 241, 0.2);
  border-radius: 6px;
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s;
  white-space: nowrap;
}

.retry-btn:hover {
  background: rgba(99, 102, 241, 0.2);
}

/* ─── Section titles ─── */
.section-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--text-secondary);
  margin: 0 0 12px;
}

.desc-section {
  margin-bottom: 28px;
}

.desc-loading,
.desc-empty {
  font-size: 13px;
  color: var(--text-dim);
  padding: 10px 0;
}

.desc-text {
  font-size: 13px;
  line-height: 1.7;
  color: var(--text-dim);
  padding: 12px 16px;
  background: var(--bg-card);
  border-radius: 8px;
  white-space: pre-wrap;
  word-break: break-all;
}

/* ─── Files ─── */
.files-section {
  margin-bottom: 24px;
}

.files-loading,
.files-empty {
  font-size: 13px;
  color: var(--text-dim);
  padding: 10px 0;
}

.files-list {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.file-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px 12px;
  border-radius: 8px;
  cursor: pointer;
  transition: background 0.15s;
  font-size: 13px;
}

.file-item:hover {
  background: var(--bg-card);
}

.file-item.selected {
  background: rgba(99, 102, 241, 0.08);
}

.file-item input {
  accent-color: #6366f1;
  flex-shrink: 0;
}

.file-name {
  flex: 1;
  color: var(--text-secondary);
  word-break: break-all;
}

.file-size {
  color: var(--text-dim);
  font-size: 11px;
  flex-shrink: 0;
}

.file-quant {
  background: rgba(34, 197, 94, 0.1);
  color: #4ade80;
  padding: 1px 7px;
  border-radius: 4px;
  font-size: 10px;
  font-weight: 600;
  flex-shrink: 0;
  letter-spacing: 0.3px;
}
</style>
