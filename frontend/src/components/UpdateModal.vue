<template>
  <div v-if="visible" class="modal-overlay" @click.self="close">
    <div class="modal" role="dialog" aria-modal="true" aria-labelledby="update-title">
      <div class="modal-header">
        <h2 id="update-title">{{ t('updateModal.title') }}</h2>
        <span class="modal-version">{{ version }}</span>
        <button ref="closeBtn" class="modal-close" :aria-label="t('updateModal.close')" @click="close">✕</button>
      </div>

      <div class="modal-body">
        <!-- 下载中 -->
        <template v-if="download && (download.status === 'downloading' || download.status === 'done' || download.status === 'error')">
          <div class="dl-status">
            <template v-if="download.status === 'downloading'">
              <div class="progress-bar">
                <div class="progress-fill" :style="{ width: download.progress + '%' }"></div>
              </div>
              <div class="progress-info">
                <span>{{ t('updateModal.downloading', { version: download.version }) }}</span>
                <span>{{ download.progress }}%</span>
              </div>
            </template>

            <template v-else-if="download.status === 'done'">
              <div class="done-msg">
                <span class="ok-icon">✓</span>
                <div>
                  <p class="done-title">{{ t('updateModal.doneTitle') }}</p>
                  <p class="done-detail">{{ t('updateModal.doneDetail') }}</p>
                  <code class="done-path">{{ download.filePath }}</code>
                  <!-- setup 安装版：提示运行安装器；portable 或旧状态（kind 为空）：保持替换 exe 提示 -->
                  <p class="done-tip">{{ download.kind === 'setup' ? t('updateModal.doneTipSetup') : t('updateModal.doneTip') }}</p>
                </div>
              </div>
            </template>

            <template v-else-if="download.status === 'error'">
              <p class="err-msg">{{ t('updateModal.downloadFailed', { msg: download.error }) }}</p>
            </template>
          </div>
        </template>

        <!-- 待确认 -->
        <template v-else>
          <p class="new-version-tip">{{ t('updateModal.newVersionTip') }}</p>
          <div class="meta">
            <span class="meta-item">{{ t('updateModal.newVersion', { version }) }}</span>
            <span v-if="published" class="meta-item">{{ t('updateModal.published', { date: formatDate(published) }) }}</span>
          </div>
          <div v-if="notes" class="release-notes">
            <h3 class="notes-title">{{ t('updateModal.notesTitle') }}</h3>
            <pre class="notes-body">{{ notes }}</pre>
          </div>
        </template>
      </div>

      <div class="modal-footer">
        <span v-if="download && download.status === 'error'" class="footer-msg footer-err">{{ download.error }}</span>
        <template v-if="!(download && (download.status === 'downloading' || download.status === 'done'))">
          <button class="btn-cancel" :disabled="downloading" @click="close">{{ t('updateModal.cancel') }}</button>
          <button class="btn-save" :disabled="downloading" @click="downloadNow">
            {{ downloading ? t('updateModal.downloadingBtn') : t('updateModal.download') }}
          </button>
        </template>
        <template v-else-if="download && download.status === 'done'">
          <button class="btn-save" @click="close">{{ t('updateModal.gotIt') }}</button>
        </template>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { updateState, startUpdateDownload } from '../lib/update'
import { locale, t } from '../lib/i18n'

const props = defineProps<{ visible: boolean }>()
const emit = defineEmits<{ close: [] }>()

const closeBtn = ref<HTMLButtonElement | null>(null)

const download = computed(() => updateState.download)
const version = computed(() => updateState.result?.version || '')
const published = computed(() => updateState.result?.published || '')
const notes = computed(() => updateState.result?.notes || '')
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
  flex-direction: column;
  gap: 4px;
  margin-bottom: 14px;
}

.meta-item {
  font-size: 13px;
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
}

/* ─── 下载区 ─── */
.dl-status {
  min-height: 120px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.progress-bar {
  width: 100%;
  height: 8px;
  background: var(--overlay-10);
  border-radius: 4px;
  overflow: hidden;
  margin-bottom: 10px;
}

.progress-fill {
  height: 100%;
  background: var(--accent);
  border-radius: 4px;
  transition: width 0.3s ease;
}

.progress-info {
  display: flex;
  justify-content: space-between;
  font-size: 13px;
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
