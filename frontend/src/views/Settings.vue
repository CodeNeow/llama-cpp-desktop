<template>
  <div class="page">
    <div class="page-header">
      <h1 class="page-title">{{ t('settings.title') }}</h1>
      <p class="page-subtitle">{{ t('settings.subtitle') }}</p>
    </div>

    <!-- UI Style -->
    <section class="settings-section">
      <h2 class="section-title">
        <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <circle cx="12" cy="12" r="10"/><path d="M12 6v6l4 2"/>
        </svg>
        {{ t('settings.uiStyle') }}
      </h2>
      <div class="setting-row">
        <div class="setting-info">
          <span class="setting-label">{{ t('settings.themeMode') }}</span>
          <span class="setting-desc">{{ t('settings.themeDesc') }}</span>
        </div>
        <div class="theme-toggle" @click="currentTheme = currentTheme === 'dark' ? 'light' : 'dark'">
          <div class="toggle-track" :class="{ light: currentTheme === 'light' }">
            <div class="toggle-thumb">
              <svg v-if="currentTheme === 'dark'" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
                <path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z"/>
              </svg>
              <svg v-else width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
                <circle cx="12" cy="12" r="5"/><line x1="12" y1="1" x2="12" y2="3"/><line x1="12" y1="21" x2="12" y2="23"/><line x1="4.22" y1="4.22" x2="5.64" y2="5.64"/><line x1="18.36" y1="18.36" x2="19.78" y2="19.78"/><line x1="1" y1="12" x2="3" y2="12"/><line x1="21" y1="12" x2="23" y2="12"/><line x1="4.22" y1="19.78" x2="5.64" y2="18.36"/><line x1="18.36" y1="5.64" x2="19.78" y2="4.22"/>
              </svg>
            </div>
          </div>
          <span class="toggle-label">{{ currentTheme === 'dark' ? t('settings.dark') : t('settings.light') }}</span>
        </div>
      </div>
    </section>

    <!-- 界面语言 -->
    <section class="settings-section">
      <h2 class="section-title">
        <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <circle cx="12" cy="12" r="10"/><line x1="2" y1="12" x2="22" y2="12"/><path d="M12 2a15.3 15.3 0 0 1 4 10 15.3 15.3 0 0 1-4 10 15.3 15.3 0 0 1-4-10 15.3 15.3 0 0 1 4-10z"/>
        </svg>
        {{ t('settings.language') }}
      </h2>
      <div class="source-grid source-grid-col3">
        <div
          class="source-card"
          :class="{ active: appConfig.language === 'auto' }"
          role="radio"
          :aria-checked="appConfig.language === 'auto'"
          tabindex="0"
          @click="setLanguagePref('auto')"
          @keydown.enter="setLanguagePref('auto')"
        >
          <span class="source-name">{{ t('settings.languageAuto') }}</span>
          <span v-if="appConfig.language === 'auto'" class="source-check">✓</span>
        </div>
        <div
          class="source-card"
          :class="{ active: appConfig.language === 'zh' }"
          role="radio"
          :aria-checked="appConfig.language === 'zh'"
          tabindex="0"
          @click="setLanguagePref('zh')"
          @keydown.enter="setLanguagePref('zh')"
        >
          <span class="source-name">{{ t('settings.languageZh') }}</span>
          <span v-if="appConfig.language === 'zh'" class="source-check">✓</span>
        </div>
        <div
          class="source-card"
          :class="{ active: appConfig.language === 'en' }"
          role="radio"
          :aria-checked="appConfig.language === 'en'"
          tabindex="0"
          @click="setLanguagePref('en')"
          @keydown.enter="setLanguagePref('en')"
        >
          <span class="source-name">{{ t('settings.languageEn') }}</span>
          <span v-if="appConfig.language === 'en'" class="source-check">✓</span>
        </div>
      </div>
      <p class="source-hint">{{ t('settings.languageDesc') }}</p>
      <p v-if="languageError" class="source-error">{{ languageError }}</p>
    </section>

    <!-- 下载 -->
    <section class="settings-section">
      <h2 class="section-title">
        <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" y1="15" x2="12" y2="3"/>
        </svg>
        {{ t('settings.downloadSource') }}
      </h2>
      <div class="source-grid">
        <div
          class="source-card"
          :class="{ active: downloadSource === 'hf' }"
          role="radio"
          :aria-checked="downloadSource === 'hf'"
          tabindex="0"
          @click="setSource('hf')"
          @keydown.enter="setSource('hf')"
        >
          <span class="source-name">{{ t('settings.sourceHf') }}</span>
          <span v-if="downloadSource === 'hf'" class="source-check">✓</span>
        </div>
        <div
          class="source-card"
          :class="{ active: downloadSource === 'modelscope' }"
          role="radio"
          :aria-checked="downloadSource === 'modelscope'"
          tabindex="0"
          @click="setSource('modelscope')"
          @keydown.enter="setSource('modelscope')"
        >
          <span class="source-name">{{ t('settings.sourceModelScope') }}</span>
          <span v-if="downloadSource === 'modelscope'" class="source-check">✓</span>
        </div>
      </div>
      <p class="source-hint">{{ t('settings.sourceHint') }}</p>
      <p v-if="sourceError" class="source-error">{{ sourceError }}</p>
    </section>

    <!-- 更新 -->
    <section class="settings-section">
      <h2 class="section-title">
        <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <path d="M20 6 9 17l-5-5"/>
        </svg>
        {{ t('settings.update') }}
      </h2>
      <div class="setting-row">
        <div class="setting-info">
          <span class="setting-label">{{ t('settings.checkUpdate') }}</span>
          <span class="setting-desc">{{ t('settings.updateDesc', { version: appVersion }) }}</span>
        </div>
        <div class="update-actions">
          <span v-if="checkError" class="update-error">{{ checkError }}</span>
          <span v-else-if="checkResult && !checkResult.hasUpdate" class="update-latest">{{ t('settings.latest') }}</span>
          <button class="btn-check" :disabled="checking" @click="manualCheck">
            {{ checking ? t('settings.checking') : t('settings.checkUpdate') }}
          </button>
        </div>
      </div>
    </section>

    <UpdateModal :visible="showUpdateModal" @close="closeUpdateModal" />
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { appConfig, setTheme, loadConfig, setDownloadSource as applyDownloadSource, setLanguage as applyLanguage } from '../store'
import { updateState, checkForUpdate, closeUpdateModal } from '../lib/update'
import { getAppVersion } from '../wails'
import { t } from '../lib/i18n'
import UpdateModal from '../components/UpdateModal.vue'

const currentTheme = computed({
  get: () => appConfig.theme,
  set: (v) => setTheme(v)
})

const downloadSource = computed(() => appConfig.downloadSource)
const sourceError = ref('')
const sourceSwitching = ref(false)

async function setSource(source: string) {
  if (source === appConfig.downloadSource || sourceSwitching.value) return
  sourceSwitching.value = true
  sourceError.value = ''
  try {
    await applyDownloadSource(source)
  } catch {
    sourceError.value = t('settings.sourceError')
  } finally {
    sourceSwitching.value = false
  }
}

const languageError = ref('')
const languageSwitching = ref(false)

async function setLanguagePref(lang: string) {
  if (lang === appConfig.language || languageSwitching.value) return
  languageSwitching.value = true
  languageError.value = ''
  try {
    await applyLanguage(lang)
  } catch {
    languageError.value = t('settings.languageError')
  } finally {
    languageSwitching.value = false
  }
}

const appVersion = ref('')
const checking = computed(() => updateState.checking)
const checkError = computed(() => updateState.error)
const checkResult = computed(() => updateState.result)
const showUpdateModal = computed(() => updateState.showModal)

onMounted(async () => {
  if (!appConfig.loaded) await loadConfig()
  getAppVersion().then((v) => { appVersion.value = v }).catch(() => {})
})

async function manualCheck() {
  await checkForUpdate()
}
</script>

<style scoped>
.page {
  padding: 36px 48px 60px;
  max-width: 720px;
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
.settings-section {
  padding: 24px 28px;
  background: var(--bg-card);
  border: 1px solid var(--border);
  border-radius: 14px;
  margin-bottom: 16px;
}

.section-title {
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 15px;
  font-weight: 600;
  color: var(--text-secondary);
  margin: 0 0 20px;
}

.section-title svg {
  color: var(--accent-light);
  flex-shrink: 0;
}

/* ─── Setting row ─── */
.setting-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.setting-info {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.setting-label {
  font-size: 14px;
  font-weight: 500;
  color: var(--text-secondary);
}

.setting-desc {
  font-size: 12px;
  color: var(--text-dim);
}

/* ─── Theme toggle ─── */
.theme-toggle {
  display: flex;
  align-items: center;
  gap: 12px;
  cursor: pointer;
  user-select: none;
}

.toggle-track {
  width: 52px;
  height: 28px;
  background: var(--overlay-20);
  border-radius: 14px;
  position: relative;
  transition: background 0.3s ease;
}

.toggle-track.light {
  background: #6366f1;
}

.toggle-thumb {
  position: absolute;
  top: 3px;
  left: 3px;
  width: 22px;
  height: 22px;
  background: #fff;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: transform 0.3s ease;
  color: var(--text-primary);
}

.toggle-track.light .toggle-thumb {
  transform: translateX(24px);
  color: var(--accent);
}

.toggle-thumb svg {
  display: block;
}

.toggle-label {
  font-size: 13px;
  font-weight: 500;
  color: var(--text-muted);
  min-width: 28px;
}

/* ─── 模型下载源 ─── */
.source-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
  margin-bottom: 12px;
}

.source-grid-col3 {
  grid-template-columns: repeat(3, 1fr);
}

.source-card {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  padding: 14px 16px;
  background: var(--bg-primary);
  border: 1px solid var(--border);
  border-radius: 10px;
  cursor: pointer;
  transition: all 0.2s;
  user-select: none;
}

.source-card:hover {
  border-color: var(--overlay-20);
}

.source-card.active {
  border-color: rgba(99, 102, 241, 0.5);
  background: rgba(99, 102, 241, 0.08);
}

.source-name {
  font-size: 13px;
  font-weight: 600;
  color: var(--text-secondary);
}

.source-card.active .source-name {
  color: var(--accent-light);
}

.source-check {
  color: var(--accent-light);
  font-weight: 700;
  flex-shrink: 0;
}

.source-hint {
  font-size: 12px;
  color: var(--text-dim);
  margin: 0;
}

.source-error {
  margin: 10px 0 0;
  font-size: 12px;
  color: #ef4444;
}

/* ─── 更新 ─── */
.update-actions {
  display: flex;
  align-items: center;
  gap: 12px;
}

.update-error {
  font-size: 12px;
  color: #ef4444;
}

.update-latest {
  font-size: 13px;
  font-weight: 600;
  color: var(--success);
}

.btn-check {
  padding: 8px 20px;
  background: var(--accent);
  border: none;
  border-radius: 8px;
  color: #fff;
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
  transition: opacity 0.15s;
}

.btn-check:hover:not(:disabled) {
  opacity: 0.85;
}

.btn-check:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
</style>
