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

    <!-- Content band: fills the window below the sticky header; only scrolls
         when the window is too short for the grid's internal columns -->
    <div class="detail-scroll">
      <!-- Error state -->
      <div v-if="loadError" class="error-card">
        <p class="error-text">{{ t('home.errorTitle') }}：{{ loadError }}</p>
        <button class="retry-btn" @click="retry">{{ t('home.retry') }}</button>
      </div>

      <!-- Two-column layout (mirrors Api.vue's .monitor-grid): description on
           the left, file list on the right, each scrolling independently -->
      <div class="detail-grid">
        <!-- Left column: model description -->
        <section class="desc-section">
          <h2 class="section-title">{{ t('downloads.descTitle') }}</h2>
          <div class="desc-body">
            <div v-if="descLoading" class="desc-loading">{{ t('downloads.loadingDesc') }}</div>
            <!-- Delegated link handler: external links open in the system browser,
                 the WebView never navigates (see lib/linkHandler.ts) -->
            <div v-else-if="description" class="desc-text" v-html="renderDescription(description)" @click="handleLinkClick"></div>
            <div v-else class="desc-empty">{{ t('downloads.noDesc') }}</div>
          </div>
        </section>

        <!-- Right column: file list -->
        <section class="files-section">
          <h2 class="section-title">{{ t('downloads.fileCount', { n: files.length }) }}</h2>
          <div class="files-body">
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
                <span class="file-name" :title="f.filename">{{ f.filename }}</span>
                <span v-if="guessQuant(f.filename)" class="file-quant">{{ guessQuant(f.filename) }}</span>
                <span v-if="f.size" class="file-size">{{ formatBytes(f.size) }}</span>
              </label>
            </div>
          </div>
        </section>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { getModelFiles, getModelDescription, startDownload } from '../wails'
import { sortModelFiles, guessQuant } from '../lib/modelFiles'
import { formatBytes } from '../lib/format'
import { renderDescription } from '../lib/markdown'
import { handleLinkClick } from '../lib/linkHandler'
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
/* Fixed-viewport layout: the router cannot mark this page fixed (route meta
   is outside this file), so the page fills the window below the 36px titlebar
   on its own — same height contract as .page-fixed in global.css plus the
   .content-area dock-reserve term, so the page sits exactly inside the shared
   content box and the content area itself never scrolls. The sticky header
   band stays pinned above the scrolling detail columns; its variable height
   (conditional action bar, wrappable title) is resolved by flexbox, not a
   pixel constant. */
.page {
  padding: 0 48px 0;
  height: calc(100vh - 36px - var(--dock-reserve, 0px));
  display: flex;
  flex-direction: column;
  /* Match .content-area's padding-bottom transition so appearing/disappearing
     TaskDock pill never leaves the page taller than the content box */
  transition: height 0.2s ease;
}

/* Keep the sticky header band from compressing inside the fixed-height page */
.sticky-top {
  flex-shrink: 0;
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

/* ─── Content band: flexes under the sticky header, scrolls only when the
     window is too short; the error card and the grid share it ─── */
.detail-scroll {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
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

/* ─── Two-column grid: description (left) + files (right) ─── */
.detail-grid {
  display: grid;
  /* minmax(0, Nfr): floor the tracks at 0 (not auto) so long unbreakable
     strings (model filenames, README URLs) cannot widen a column past its
     share; the narrow-viewport block at the end collapses the grid */
  grid-template-columns: minmax(0, 6fr) minmax(0, 4fr);
  /* The single row shares the grid height exactly; min-height: 0 on the
     columns lets excess content scroll inside each column instead of
     stretching the row */
  grid-template-rows: minmax(0, 1fr);
  gap: 16px;
  /* Fill the band below the sticky header so both columns scroll internally
     on typical window sizes (no minimum floor: both columns are plain
     scrollable lists, so nothing gets crushed on short windows) */
  flex: 1;
  min-height: 0;
  margin-bottom: 16px;
}

/* ─── Columns: pinned title + internally scrolling body (same structure as
     Api.vue's .log-panel: header flex-shrink: 0, body flex: 1 + min-height: 0
     + overflow-y: auto) ─── */
.desc-section,
.files-section {
  display: flex;
  flex-direction: column;
  min-height: 0;
  min-width: 0;
}

.section-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--text-secondary);
  margin: 0 0 12px;
  flex-shrink: 0;
}

.desc-body,
.files-body {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
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
  word-break: break-all;
}

/* ─── Markdown-rendered description content ───
   v-html content does not carry the scope attribute, so child selectors need
   :deep(). Compact spacing so rendered blocks keep the card look (same
   approach as Chat.vue's .markdown-body rules). */
.desc-text :deep(h1),
.desc-text :deep(h2),
.desc-text :deep(h3),
.desc-text :deep(h4) {
  margin: 12px 0 6px;
  line-height: 1.4;
  color: var(--text-secondary);
}

.desc-text :deep(h1) { font-size: 16px; }
.desc-text :deep(h2) { font-size: 15px; }
.desc-text :deep(h3),
.desc-text :deep(h4) { font-size: 14px; }

.desc-text :deep(h1:first-child),
.desc-text :deep(h2:first-child),
.desc-text :deep(h3:first-child),
.desc-text :deep(h4:first-child) {
  margin-top: 0;
}

.desc-text :deep(p) {
  margin: 6px 0;
}

.desc-text :deep(ul),
.desc-text :deep(ol) {
  margin: 6px 0;
  padding-left: 1.4em;
}

.desc-text :deep(li) {
  margin: 2px 0;
}

.desc-text :deep(a) {
  color: var(--accent-light);
  word-break: break-all;
}

.desc-text :deep(code) {
  background: var(--surface);
  border-radius: 4px;
  padding: 0.5px 5px;
  font-family: var(--font-mono);
  font-size: 12px;
}

/* ─── Files ─── */
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
  /* Two-axis gap: tighter row gap for the wrapped quant/size line */
  gap: 6px 10px;
  /* Let the quant tag and size drop onto a second line when the 4fr column
     is too narrow to fit them beside the filename */
  flex-wrap: wrap;
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
  flex: 1 1 auto;
  /* Truncate long filenames with an ellipsis (full name exposed via the
     title tooltip) instead of wrapping and blowing out the 4fr column; the
     min-width floor pushes the quant tag + size to a second line first */
  min-width: 140px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--text-secondary);
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

/* ─── Narrow viewports (< 1100px, same breakpoint as Api.vue): collapse to a
     single column with the description first, files below. The page releases
     its fixed height so the shared content area scrolls naturally and both
     columns flow unbounded (the sticky header pins via the global
     .sticky-top rule) ─── */
@media (max-width: 1099px) {
  .page {
    height: auto;
  }

  .detail-scroll {
    flex: none;
    overflow: visible;
  }

  .detail-grid {
    grid-template-columns: minmax(0, 1fr);
    grid-template-rows: none;
    /* Content-driven height: the content area scrolls, the grid does not
       stretch; the row gap separates the stacked sections */
    gap: 24px;
    flex: none;
    min-height: 0;
    margin-bottom: 24px;
  }

  .desc-body,
  .files-body {
    flex: none;
    overflow: visible;
  }
}
</style>
