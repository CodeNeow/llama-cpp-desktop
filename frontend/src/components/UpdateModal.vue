<template>
  <div v-if="visible" class="modal-overlay" @click.self="close">
    <div class="modal" role="dialog" aria-modal="true" aria-labelledby="update-title">
      <div class="modal-header">
        <h2 id="update-title">{{ t('updateModal.title') }}</h2>
        <span class="modal-version">{{ version }}</span>
        <button ref="closeBtn" class="modal-close" :aria-label="t('updateModal.close')" @click="close">✕</button>
      </div>

      <div class="modal-body">
        <!-- Download in progress / finished -->
        <template v-if="download && (download.status === 'downloading' || download.status === 'done' || download.status === 'error')">
          <div class="dl-status">
            <!-- Downloading: centered glyph + thick gradient bar + percent + transferred size -->
            <div v-if="download.status === 'downloading'" class="dl-downloading">
              <div class="dl-glyph" aria-hidden="true">
                <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                  <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" y1="15" x2="12" y2="3"/>
                </svg>
              </div>
              <p class="dl-version">{{ t('updateModal.downloading', { version: download.version }) }}</p>
              <div class="dl-progress-bar">
                <div class="dl-progress-fill" :style="{ width: download.progress + '%' }"></div>
              </div>
              <span class="dl-percent">{{ download.progress }}%</span>
              <p v-if="download.total > 0" class="dl-size">{{ formatBytes(download.downloaded) }} / {{ formatBytes(download.total) }}</p>
            </div>

            <template v-else-if="download.status === 'done'">
              <div class="done-msg">
                <span class="ok-icon">✓</span>
                <div>
                  <p class="done-title">{{ t('updateModal.doneTitle') }}</p>
                  <p class="done-detail">{{ t('updateModal.doneDetail') }}</p>
                  <code class="done-path">{{ download.filePath }}</code>
                  <!-- setup installer: prompt to run the installer; portable or legacy state (kind empty): keep the replace-exe hint -->
                  <p class="done-tip">{{ download.kind === 'setup' ? t('updateModal.doneTipSetup') : t('updateModal.doneTip') }}</p>
                </div>
              </div>
            </template>

            <template v-else-if="download.status === 'error'">
              <p class="err-msg">{{ t('updateModal.downloadFailed', { msg: download.error }) }}</p>
            </template>
          </div>
        </template>

        <!-- Awaiting confirmation -->
        <template v-else>
          <p class="new-version-tip">{{ t('updateModal.newVersionTip') }}</p>
          <div class="meta">
            <span class="meta-chip">{{ t('updateModal.newVersion', { version }) }}</span>
            <span v-if="published" class="meta-chip plain">{{ t('updateModal.published', { date: formatDate(published) }) }}</span>
          </div>
          <div v-if="notes" class="release-notes">
            <h3 class="notes-title">{{ t('updateModal.notesTitle') }}</h3>
            <pre class="notes-body">{{ notes }}</pre>
          </div>
        </template>
      </div>

      <div class="modal-footer">
        <span v-if="download && download.status === 'error'" class="footer-msg footer-err">{{ download.error }}</span>
        <!-- Downloading: cancel the download, or hide the modal and continue in the background -->
        <template v-if="downloading">
          <button class="btn-cancel" @click="cancelUpdateDownload">{{ t('updateModal.cancelDownload') }}</button>
          <button class="btn-save" @click="close">{{ t('updateModal.hideDownload') }}</button>
        </template>
        <!-- Done: single acknowledge button -->
        <template v-else-if="download && download.status === 'done'">
          <button class="btn-save" @click="close">{{ t('updateModal.gotIt') }}</button>
        </template>
        <!-- Confirm view and error view share the close / start(retry) actions -->
        <template v-else>
          <button class="btn-cancel" @click="close">{{ t('updateModal.cancel') }}</button>
          <button class="btn-save" @click="downloadNow">{{ t('updateModal.download') }}</button>
        </template>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { updateState, startUpdateDownload, cancelUpdateDownload, extractReleaseNotes } from '../lib/update'
import { locale, t } from '../lib/i18n'
import { formatBytes } from '../lib/format'

const props = defineProps<{ visible: boolean }>()
const emit = defineEmits<{ close: [] }>()

const closeBtn = ref<HTMLButtonElement | null>(null)

const download = computed(() => updateState.download)
const version = computed(() => updateState.result?.version || '')
const published = computed(() => updateState.result?.published || '')
const notes = computed(() => extractReleaseNotes(updateState.result?.notes || '', locale.value))
const downloading = computed(() => download.value?.status === 'downloading')

watch(() => props.visible, (v) => {
  if (v) nextTick(() => closeBtn.value?.focus())
})

function formatDate(iso: string): string {
  const d = new Date(iso)
  if (isNaN(d.getTime())) return iso
  return d.toLocaleDateString(locale.value === 'en' ? 'en-US' : 'zh-CN')
}
function downloadNow() {
  startUpdateDownload()
}

function close() {
  emit('close')
}
</script>

<style scoped>
.modal-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0,0,0,0.6);
  backdrop-filter: blur(4px);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
}

.modal {
  background: var(--bg-secondary);
  border: 1px solid var(--border);
  border-radius: 16px;
  width: 520px;
  max-height: 80vh;
  display: flex;
  flex-direction: column;
  box-shadow: 0 20px 60px rgba(0,0,0,0.5);
}

.modal-header {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 20px 24px;
  border-bottom: 1px solid var(--border);
}

.modal-header h2 {
  font-size: 17px;
  font-weight: 700;
  color: var(--text-primary);
  margin: 0;
}

.modal-version {
  font-size: 12px;
  color: var(--text-dim);
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.modal-close {
  width: 28px;
  height: 28px;
  border: none;
  background: none;
  color: var(--text-dim);
  font-size: 16px;
  cursor: pointer;
  border-radius: 6px;
  transition: all 0.15s;
}

.modal-close:hover {
  background: var(--hover-bg);
  color: var(--text-primary);
}

.modal-body {
  padding: 20px 24px;
  overflow-y: auto;
  flex: 1;
}

.new-version-tip {
  margin: 0 0 14px;
  font-size: 14px;
  color: var(--text-secondary);
}

.meta {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 14px;
}

/* Version / published chips: accent-tinted for the version, neutral for the date */
.meta-chip {
  display: inline-flex;
  align-items: center;
  padding: 3px 10px;
  border-radius: 999px;
  font-size: 12px;
  font-weight: 600;
  border: 1px solid rgba(99, 102, 241, 0.35);
  background: rgba(99, 102, 241, 0.08);
  color: var(--accent-light);
}

.meta-chip.plain {
  border-color: var(--border);
  background: var(--bg-primary);
  color: var(--text-muted);
}

.release-notes {
  border: 1px solid var(--border-light);
  border-radius: 10px;
  padding: 12px 14px;
  background: var(--bg-primary);
}

.notes-title {
  margin: 0 0 8px;
  font-size: 12px;
  font-weight: 700;
  color: var(--accent-light);
  text-transform: uppercase;
  letter-spacing: 0.5px;
}

.notes-body {
  margin: 0;
  font-size: 12px;
  line-height: 1.6;
  color: var(--text-secondary);
  white-space: pre-wrap;
  word-break: break-word;
  font-family: inherit;
  max-height: 40vh;
  overflow-y: auto;
}

/* ─── Downloads area ─── */
.dl-status {
  min-height: 120px;
  display: flex;
  align-items: center;
  justify-content: center;
}

/* Downloading: centered vertical layout — gradient glyph, thick gradient bar,
   prominent percent, transferred size. Colors reuse the app's indigo-violet accents. */
.dl-downloading {
  width: 100%;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
  padding: 8px 0;
}

.dl-glyph {
  width: 44px;
  height: 44px;
  border-radius: 12px;
  background: linear-gradient(135deg, #6366f1, #a78bfa);
  color: #fff;
  display: flex;
  align-items: center;
  justify-content: center;
  box-shadow: 0 6px 16px rgba(99, 102, 241, 0.3);
}

.dl-version {
  margin: 0;
  font-size: 15px;
  font-weight: 600;
  color: var(--text-primary);
}

.dl-progress-bar {
  width: 100%;
  height: 10px;
  background: var(--overlay-10);
  border-radius: 5px;
  overflow: hidden;
}

.dl-progress-fill {
  height: 100%;
  background: linear-gradient(90deg, #6366f1, #a78bfa);
  border-radius: 5px;
  transition: width 0.3s ease;
}

.dl-percent {
  font-size: 22px;
  font-weight: 700;
  color: var(--text-primary);
  letter-spacing: 0.5px;
  line-height: 1;
}

.dl-size {
  margin: 0;
  font-size: 12px;
  color: var(--text-muted);
}

.done-msg {
  display: flex;
  gap: 12px;
  align-items: flex-start;
  width: 100%;
}

.ok-icon {
  width: 32px;
  height: 32px;
  border-radius: 50%;
  background: var(--success);
  color: #fff;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 16px;
  flex-shrink: 0;
}

.done-title {
  margin: 0 0 6px;
  font-size: 15px;
  font-weight: 700;
  color: var(--text-primary);
}

.done-detail {
  margin: 0 0 6px;
  font-size: 13px;
  color: var(--text-muted);
}

.done-path {
  display: block;
  font-size: 12px;
  color: var(--text-secondary);
  background: var(--bg-primary);
  border: 1px solid var(--border-light);
  border-radius: 6px;
  padding: 6px 8px;
  word-break: break-all;
  margin-bottom: 8px;
}

.done-tip {
  margin: 0;
  font-size: 12px;
  color: var(--accent-light);
}

.err-msg {
  margin: 0;
  font-size: 13px;
  color: #ef4444;
}

.modal-footer {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  padding: 16px 24px;
  border-top: 1px solid var(--border);
}

.btn-cancel {
  padding: 8px 20px;
  background: transparent;
  border: 1px solid var(--border);
  border-radius: 8px;
  color: var(--text-muted);
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.15s;
}

.btn-cancel:hover:not(:disabled) {
  background: var(--hover-bg);
}

.btn-save {
  padding: 8px 24px;
  background: var(--accent);
  border: none;
  border-radius: 8px;
  color: #fff;
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
  transition: opacity 0.15s;
}

.btn-save:hover:not(:disabled) {
  opacity: 0.85;
}

.btn-save:disabled,
.btn-cancel:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.footer-msg {
  font-size: 13px;
  font-weight: 600;
  margin-right: auto;
}

.footer-err {
  color: #ef4444;
}
</style>
