<template>
  <div class="page">
    <div class="sticky-top">
      <div class="page-header">
        <h1 class="page-title">{{ t('settings.title') }}</h1>
        <p class="page-subtitle">{{ t('settings.subtitle') }}</p>
      </div>
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

    <!-- Interface Language -->
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

    <!-- Downloads -->
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

    <!-- Directories: download paths for new installs/downloads (imported dirs
         for reuse stay on the Home and Models pages) -->
    <section class="settings-section">
      <h2 class="section-title">
        <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"/>
        </svg>
        {{ t('settings.directories') }}
      </h2>
      <div class="dir-row">
        <div class="setting-info">
          <span class="setting-label">{{ t('settings.llamaCppDownloadDir') }}</span>
          <span class="setting-desc">{{ t('settings.llamaCppDownloadDirDesc') }}</span>
        </div>
        <div class="dir-path-row">
          <div class="dir-path">{{ appConfig.llamaCppDownloadDir }}</div>
          <button class="dir-btn" type="button" @click="chooseLlamaCppDownloadDir">{{ t('settings.choose') }}</button>
        </div>
      </div>
      <div class="dir-row">
        <div class="setting-info">
          <span class="setting-label">{{ t('settings.modelDownloadDir') }}</span>
          <span class="setting-desc">{{ t('settings.modelDownloadDirDesc') }}</span>
        </div>
        <div class="dir-path-row">
          <div class="dir-path">{{ appConfig.modelDownloadDir }}</div>
          <button class="dir-btn" type="button" @click="chooseModelDownloadDir">{{ t('settings.choose') }}</button>
        </div>
      </div>
      <p class="source-hint">{{ t('settings.directoriesHint') }}</p>
    </section>

    <!-- Server access scope -->
    <section class="settings-section">
      <h2 class="section-title">
        <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <circle cx="12" cy="12" r="10"/><line x1="2" y1="12" x2="22" y2="12"/>
        </svg>
        {{ t('settings.accessScope') }}
      </h2>
      <div class="source-grid">
        <div
          class="source-card"
          :class="{ active: appConfig.serverAccessMode === 'local' }"
          role="radio"
          :aria-checked="appConfig.serverAccessMode === 'local'"
          tabindex="0"
          @click="setAccessScope('local')"
          @keydown.enter="setAccessScope('local')"
        >
          <span class="source-name">{{ t('settings.accessLocal') }}</span>
          <span v-if="appConfig.serverAccessMode === 'local'" class="source-check">✓</span>
        </div>
        <div
          class="source-card"
          :class="{ active: appConfig.serverAccessMode === 'lan' }"
          role="radio"
          :aria-checked="appConfig.serverAccessMode === 'lan'"
          tabindex="0"
          @click="setAccessScope('lan')"
          @keydown.enter="setAccessScope('lan')"
        >
          <span class="source-name">{{ t('settings.accessLan') }}</span>
          <span v-if="appConfig.serverAccessMode === 'lan'" class="source-check">✓</span>
        </div>
      </div>
      <p class="source-hint">{{ t('settings.accessDesc') }}</p>
      <p v-if="accessError" class="source-error">{{ accessError }}</p>
    </section>

    <!-- System tray (Windows only; other platforms exit directly on close) -->
    <section v-if="isWindows" class="settings-section">
      <h2 class="section-title">
        <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <rect x="2" y="7" width="20" height="14" rx="2" ry="2"/><path d="M16 21V5a2 2 0 0 0-2-2h-4a2 2 0 0 0-2 2v16"/>
        </svg>
        {{ t('settings.tray') }}
      </h2>
      <div class="setting-row">
        <div class="setting-info">
          <span class="setting-label">{{ t('settings.tray') }}</span>
          <span class="setting-desc">{{ t('settings.trayDesc') }}</span>
        </div>
        <div class="theme-toggle" @click="toggleTray">
          <div class="toggle-track" :class="{ light: appConfig.trayEnabled }">
            <div class="toggle-thumb">
              <svg v-if="appConfig.trayEnabled" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
                <polyline points="20 6 9 17 4 12"/>
              </svg>
              <svg v-else width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
                <line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/>
              </svg>
            </div>
          </div>
          <span class="toggle-label">{{ appConfig.trayEnabled ? t('settings.on') : t('settings.off') }}</span>
        </div>
      </div>
      <!-- systray cannot restart in same process: after disabling, re-enable requires app restart -->
      <p class="source-hint">{{ t('settings.trayHint') }}</p>
      <p v-if="trayError" class="source-error">{{ trayError }}</p>
    </section>

    <!-- API route mode (Windows only): restart into headless tray+server mode -->
    <section v-if="isWindows" class="settings-section">
      <h2 class="section-title">
        <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <circle cx="18" cy="5" r="3"/><circle cx="6" cy="12" r="3"/><circle cx="18" cy="19" r="3"/><line x1="8.59" y1="13.51" x2="15.42" y2="17.49"/><line x1="15.41" y1="6.51" x2="8.59" y2="10.49"/>
        </svg>
        {{ t('settings.apiRouteMode') }}
      </h2>
      <div class="setting-row">
        <div class="setting-info">
          <span class="setting-label">{{ t('settings.apiRouteMode') }}</span>
          <span class="setting-desc">{{ t('settings.apiRouteModeDesc') }}</span>
        </div>
        <div class="theme-toggle" @click="toggleApiRouteMode">
          <div class="toggle-track" :class="{ light: appConfig.apiRouteMode }">
            <div class="toggle-thumb">
              <svg v-if="appConfig.apiRouteMode" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
                <polyline points="20 6 9 17 4 12"/>
              </svg>
              <svg v-else width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
                <line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/>
              </svg>
            </div>
          </div>
          <span class="toggle-label">{{ appConfig.apiRouteMode ? t('settings.on') : t('settings.off') }}</span>
        </div>
      </div>
      <p v-if="apiRouteError" class="source-error">{{ apiRouteError }}</p>
    </section>

    <!-- Updates -->
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
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { appConfig, setTheme, loadConfig, setDownloadSource as applyDownloadSource, setLanguage as applyLanguage, setServerAccessMode as applyServerAccessMode, setTrayEnabled as applyTrayEnabled } from '../store'
import { updateState, checkForUpdate } from '../lib/update'
import { getAppVersion, getOS, getServerConfig, browseLlamaCppDownloadDir, browseModelDownloadDir, setApiRouteMode } from '../wails'
import { t } from '../lib/i18n'

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

// ─── Directories ────────────────────────────────────────────────
// Download paths decide where new llama.cpp installs and model downloads
// land. The backend persists the choice and refreshes its caches; local state
// is updated only after a non-empty pick so a cancelled dialog leaves the
// display unchanged. Errors are silent (the dialog already reports failure).
async function chooseLlamaCppDownloadDir() {
  const dir = await browseLlamaCppDownloadDir()
  if (dir) appConfig.llamaCppDownloadDir = dir
}

async function chooseModelDownloadDir() {
  const dir = await browseModelDownloadDir()
  if (dir) appConfig.modelDownloadDir = dir
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

// Server access scope (listen address, see backend SaveServerConfig): refreshed from backend on mount
// to stay in sync with persisted values from the API page and other sources.
const accessError = ref('')
const accessSwitching = ref(false)

async function setAccessScope(mode: string) {
  if (mode === appConfig.serverAccessMode || accessSwitching.value) return
  accessSwitching.value = true
  accessError.value = ''
  try {
    await applyServerAccessMode(mode)
  } catch {
    accessError.value = t('settings.accessError')
  } finally {
    accessSwitching.value = false
  }
}

// System tray toggle: rendered on Windows only. Disabling takes effect immediately (backend removes icon and
// persists); systray cannot restart in same process, so re-enabling requires app restart (hint shown below toggle).
const isWindows = ref(false)
const trayError = ref('')
const traySwitching = ref(false)

async function toggleTray() {
  if (traySwitching.value) return
  traySwitching.value = true
  trayError.value = ''
  try {
    await applyTrayEnabled(!appConfig.trayEnabled)
  } catch {
    trayError.value = t('settings.trayError')
  } finally {
    traySwitching.value = false
  }
}

// API-route mode toggle: rendered on Windows only (backend gates headless to Windows).
// Enabling relaunches the app headless and quits the GUI without stopping llama-server;
// in GUI mode the state is always off, so this effectively only ever enables. On success
// the process quits (no visible state change to roll back); failures surface inline.
const apiRouteError = ref('')
const apiRouteSwitching = ref(false)

async function toggleApiRouteMode() {
  if (apiRouteSwitching.value) return
  apiRouteSwitching.value = true
  apiRouteError.value = ''
  try {
    await setApiRouteMode(!appConfig.apiRouteMode)
  } catch {
    apiRouteError.value = t('settings.apiRouteModeError')
  } finally {
    apiRouteSwitching.value = false
  }
}

const appVersion = ref('')
const checking = computed(() => updateState.checking)
const checkError = computed(() => updateState.error)
const checkResult = computed(() => updateState.result)

onMounted(async () => {
  if (!appConfig.loaded) await loadConfig()
  // The system tray setting is rendered on Windows only (other platforms persist it in the backend but have no tray behavior)
  getOS().then((info) => { isWindows.value = info.os === 'windows' }).catch(() => {})
  // Read the current service access scope from the backend (default local) so the page selection matches the persisted value
  getServerConfig().then((scfg) => {
    if (scfg.accessMode === 'local' || scfg.accessMode === 'lan') {
      appConfig.serverAccessMode = scfg.accessMode
    }
  }).catch(() => {})
  getAppVersion().then((v) => { appVersion.value = v }).catch(() => {})
})

async function manualCheck() {
  await checkForUpdate()
}
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

/* ─── Model download source ─── */
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

/* ─── Directories ─── */
.dir-row {
  padding: 10px 0;
}

.dir-row + .dir-row {
  border-top: 1px solid var(--border-light);
}

/* Full path on its own line under the label/desc, button vertically centered
   beside it (same row), so long Windows paths stay fully readable */
.dir-path-row {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-top: 8px;
}

.dir-path {
  flex: 1;
  min-width: 0;
  word-break: break-all;
  line-height: 1.4;
  font-size: 12px;
  font-family: var(--font-mono);
  color: var(--text-muted);
  background: var(--bg-primary);
  border: 1px solid var(--border);
  border-radius: 6px;
  padding: 5px 10px;
}

.dir-btn {
  padding: 6px 14px;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 8px;
  color: var(--text-secondary);
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s;
  flex-shrink: 0;
}

.dir-btn:hover {
  background: var(--hover-bg);
  color: var(--text-primary);
}

/* ─── Updates ─── */
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
