<template>
  <div class="page">
    <div class="sticky-top">
      <div class="page-header">
        <h1 class="page-title">{{ t('settings.title') }}</h1>
        <p class="page-subtitle">{{ t('settings.subtitle') }}</p>
      </div>

      <!-- Tabs row: same pattern as ModelSettings (icon + label buttons with an
           underline active state); activeTab is plain view state, never persisted -->
      <div class="settings-tabs" role="tablist" :aria-label="t('settings.title')">
        <button
          v-for="(tab, i) in tabs"
          :key="tab.id"
          :id="`${tab.id}-tab`"
          class="tab-btn"
          :class="{ active: activeTab === i }"
          role="tab"
          :aria-selected="activeTab === i"
          :aria-controls="tab.id"
          @click="activeTab = i"
        >
          <span class="tab-icon" v-html="tab.icon"></span>
          {{ tab.label() }}
        </button>
      </div>
    </div>

    <!-- Tab panels (v-show keeps per-tab input state alive across switches) -->
    <!-- Appearance: theme mode + interface language -->
    <div id="tab-appearance" role="tabpanel" aria-labelledby="tab-appearance-tab" v-show="activeTab === 0">
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
    </div>

    <!-- Downloads & directories: download paths for new installs/downloads (imported dirs
         for reuse stay on the Home and Models pages) -->
    <div id="tab-downloads" role="tabpanel" aria-labelledby="tab-downloads-tab" v-show="activeTab === 1">
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
          <div
            class="source-card"
            :class="{ active: downloadSource === 'huggingface' }"
            role="radio"
            :aria-checked="downloadSource === 'huggingface'"
            tabindex="0"
            @click="setSource('huggingface')"
            @keydown.enter="setSource('huggingface')"
          >
            <span class="source-name">{{ t('settings.sourceHuggingFace') }}</span>
            <span v-if="downloadSource === 'huggingface'" class="source-check">✓</span>
          </div>
        </div>
        <p class="source-hint">{{ t('settings.sourceHint') }}</p>
        <p v-if="sourceError" class="source-error">{{ sourceError }}</p>
      </section>

      <!-- Directories -->
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
            <!-- Android has no native directory picker (the Browse* bindings error
                 there): the browse buttons are hidden, paths stay read-only -->
            <button v-if="!isAndroid" class="dir-btn" type="button" @click="chooseLlamaCppDownloadDir">{{ t('settings.choose') }}</button>
          </div>
        </div>
        <div class="dir-row">
          <div class="setting-info">
            <span class="setting-label">{{ t('settings.modelDownloadDir') }}</span>
            <span class="setting-desc">{{ t('settings.modelDownloadDirDesc') }}</span>
          </div>
          <div class="dir-path-row">
            <div class="dir-path">{{ appConfig.modelDownloadDir }}</div>
            <button v-if="!isAndroid" class="dir-btn" type="button" @click="chooseModelDownloadDir">{{ t('settings.choose') }}</button>
          </div>
        </div>
        <!-- Android: no system folder picker; both paths are app-managed and
             read-only, so the rows stay informational with an explicit hint -->
        <p v-if="isAndroid" class="source-hint">{{ t('settings.dirsAndroidHint') }}</p>
        <p class="source-hint">{{ t('settings.directoriesHint') }}</p>
      </section>
    </div>

    <!-- Server: access scope + API key + inference GPU -->
    <div id="tab-server" role="tabpanel" aria-labelledby="tab-server-tab" v-show="activeTab === 2">
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
        <!-- API key always visible: it also protects the inference API in local mode,
             not only when the service is exposed to the LAN -->
        <div class="setting-row api-key-row">
          <div class="setting-info">
            <span class="setting-label">{{ t('settings.apiKey') }}</span>
            <span class="setting-desc">{{ t('settings.apiKeyDesc') }}</span>
          </div>
          <input
            v-model="apiKeyInput"
            type="password"
            class="api-key-input"
            autocomplete="off"
            spellcheck="false"
            :disabled="apiKeySwitching"
            @change="saveApiKey"
          />
        </div>
        <p v-if="apiKeyError" class="source-error">{{ apiKeyError }}</p>
        <!-- Inference GPU selection: pins the llama-server child to the chosen
             CUDA device via CUDA_VISIBLE_DEVICES (empty = auto, default device).
             Persisted through the same whole-serverConfig round-trip as the key.
             Windows-only: the backend device pinning no-ops on other platforms. -->
        <template v-if="showGpu">
          <div class="setting-row gpu-row">
            <div class="setting-info">
              <span class="setting-label">{{ t('settings.gpu.label') }}</span>
              <span class="setting-desc">{{ t('settings.gpu.desc') }}</span>
            </div>
            <ThemedSelect
              class="gpu-select"
              :model-value="gpuValue"
              :options="gpuOptions"
              :placeholder="t('settings.gpu.auto')"
              :disabled="!gpuDetected || gpuSwitching"
              variant="field"
              :label="t('settings.gpu.label')"
              @update:model-value="onGpuSelected"
            />
          </div>
          <p v-if="!gpuDetected" class="source-hint">{{ t('settings.gpu.none') }}</p>
          <p v-else class="source-hint">{{ t('settings.gpu.hint') }}</p>
          <p v-if="gpuError" class="source-error">{{ gpuError }}</p>
        </template>
      </section>
    </div>

    <!-- App & updates: tray + API route mode + updates + about -->
    <div id="tab-app" role="tabpanel" aria-labelledby="tab-app-tab" v-show="activeTab === 3">
      <!-- System tray (Windows only; other platforms exit directly on close) -->
      <section v-if="showTray" class="settings-section">
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
      <section v-if="showApiRoute" class="settings-section">
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
          <div class="theme-toggle" :class="{ disabled: !appConfig.trayEnabled || apiRouteDevBlocked }" @click="toggleApiRouteMode">
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
        <!-- dev builds refuse the relaunch-based switch entirely: it escapes the wails dev supervisor -->
        <p v-if="apiRouteDevBlocked" class="source-hint">{{ t('settings.apiRouteModeDevBlocked') }}</p>
        <!-- headless mode returns to the GUI via the tray menu only: gate the toggle on the tray setting -->
        <p v-else-if="!appConfig.trayEnabled" class="source-hint">{{ t('settings.apiRouteModeRequiresTray') }}</p>
        <p v-if="apiRouteError" class="source-error">{{ apiRouteError }}</p>
      </section>

      <!-- Updates: visible on every platform; the in-app self-update action is
           Windows-only, other platforms get a hint + GitHub Releases link -->
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
          <!-- Windows: native check-for-updates / self-update action -->
          <div v-if="updatesNative" class="update-actions">
            <span v-if="checkError" class="update-error">{{ checkError }}</span>
            <span v-else-if="checkResult && !checkResult.hasUpdate" class="update-latest">{{ t('settings.latest') }}</span>
            <button class="btn-check" :disabled="checking" @click="manualCheck">
              {{ checking ? t('settings.checking') : t('settings.checkUpdate') }}
            </button>
          </div>
          <!-- Other platforms: hint instead of the action; the Releases link
               goes through the shared external-link handler (system browser,
               never an in-WebView navigation) -->
          <p v-else class="source-hint update-hint">
            {{ t('settings.updateNotSupported') }}
            <a
              class="hint-link"
              href="https://github.com/CodeNeow/llama-cpp-desktop/releases"
              @click="handleLinkClick"
            >{{ t('settings.updateReleasesLink') }}</a>
          </p>
        </div>
      </section>

      <!-- About: version / license / repository. The repo URL is plain
           selectable text, NOT an <a> link: clicking a link would navigate the
           WebView away from the app. -->
      <section class="settings-section">
        <h2 class="section-title">
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <circle cx="12" cy="12" r="10"/><line x1="12" y1="16" x2="12" y2="12"/><line x1="12" y1="8" x2="12.01" y2="8"/>
          </svg>
          {{ t('settings.about') }}
        </h2>
        <div class="about-row">
          <span class="setting-label">{{ t('settings.version') }}</span>
          <span class="about-value">{{ appVersion || '—' }}</span>
        </div>
        <div class="about-row">
          <span class="setting-label">{{ t('settings.license') }}</span>
          <span class="about-value">GPL-3.0</span>
        </div>
        <div class="about-row">
          <span class="setting-label">{{ t('settings.repo') }}</span>
          <span class="about-value about-mono">https://github.com/CodeNeow/llama-cpp-desktop</span>
        </div>
      </section>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { appConfig, setTheme, loadConfig, setDownloadSource as applyDownloadSource, setLanguage as applyLanguage, setServerAccessMode as applyServerAccessMode, setApiKey as applyApiKey, setTrayEnabled as applyTrayEnabled } from '../store'
import { updateState, checkForUpdate } from '../lib/update'
import { getAppVersion, getSystemInfo, getServerConfig, saveServerConfig, browseLlamaCppDownloadDir, browseModelDownloadDir, setApiRouteMode } from '../wails'
import { showTraySetting, showApiRouteSetting, showServingGpuSetting, updateSectionMode, usePlatform } from '../lib/platform'
import { handleLinkClick } from '../lib/linkHandler'
import ThemedSelect, { type SelectOption } from '../components/ThemedSelect.vue'
import { formatMB } from '../lib/format'
import { t } from '../lib/i18n'

// ─── Tabs ─────────────────────────────────────────────────────────────────────
// Index of the currently active tab. Plain view state: it resets to the first
// tab on reload and is intentionally not persisted or encoded in the route.
const activeTab = ref(0)

// Tab definitions (4 groups; icons are inline stroke SVGs, mirroring ModelSettings)
const tabs = [
  {
    id: 'tab-appearance',
    label: () => t('settings.tabAppearance'),
    icon: `<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z"/></svg>`,
  },
  {
    id: 'tab-downloads',
    label: () => t('settings.tabDownloads'),
    icon: `<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" y1="15" x2="12" y2="3"/></svg>`,
  },
  {
    id: 'tab-server',
    label: () => t('settings.tabServer'),
    icon: `<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="2" y="2" width="20" height="8" rx="2" ry="2"/><rect x="2" y="14" width="20" height="8" rx="2" ry="2"/><line x1="6" y1="6" x2="6.01" y2="6"/><line x1="6" y1="18" x2="6.01" y2="18"/></svg>`,
  },
  {
    id: 'tab-app',
    label: () => t('settings.tabApp'),
    icon: `<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><line x1="12" y1="16" x2="12" y2="12"/><line x1="12" y1="8" x2="12.01" y2="8"/></svg>`,
  },
]

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

// Optional llama-server API key (bearer token; empty = no authentication): saved on
// change through setApiKey (whole serverConfig round-trip, same as the access scope).
// The input keeps the user's text on failure so they can fix and retry.
const apiKeyInput = ref('')
const apiKeyError = ref('')
const apiKeySwitching = ref(false)

async function saveApiKey() {
  if (apiKeySwitching.value) return
  apiKeySwitching.value = true
  apiKeyError.value = ''
  try {
    await applyApiKey(apiKeyInput.value)
  } catch {
    apiKeyError.value = t('settings.apiKeyError')
  } finally {
    apiKeySwitching.value = false
  }
}

// Inference GPU selection: pins the serving GPU by stable nvidia-smi UUID
// (backend pins the llama-server child via CUDA_VISIBLE_DEVICES). Options are
// Auto plus one entry per detected GPU (label: name · VRAM · uuid prefix,
// value: full UUID). Saved through the same whole-serverConfig round-trip as
// the access scope / API key; the selection only sticks after a successful
// save so a backend rejection keeps the previous choice visible.
interface GpuEntry {
  name: string
  memoryMb: number
  uuid: string
}
const gpuSnapshot = ref<GpuEntry[] | null>(null)
const gpuValue = ref('')
const gpuError = ref('')
const gpuSwitching = ref(false)

const gpuOptions = computed<SelectOption[]>(() => [
  { value: '', label: t('settings.gpu.auto') },
  ...(gpuSnapshot.value ?? [])
    .filter((g) => g.uuid)
    .map((g) => ({
      value: g.uuid,
      label: `${g.name} · ${formatMB(g.memoryMb)} · ${g.uuid.slice(0, 8)}`,
    })),
])

// A selectable GPU exists only when the probe returned at least one UUID;
// otherwise the selector stays disabled and the no-GPU hint is shown.
const gpuDetected = computed(() => !!gpuSnapshot.value?.some((g) => g.uuid))

async function onGpuSelected(value: string) {
  if (value === gpuValue.value || gpuSwitching.value) return
  gpuSwitching.value = true
  gpuError.value = ''
  try {
    const scfg = await getServerConfig()
    scfg.deviceId = value
    await saveServerConfig(scfg)
    gpuValue.value = value
  } catch {
    gpuError.value = t('settings.gpu.error')
  } finally {
    gpuSwitching.value = false
  }
}

// OS-scoped setting gates: driven by the shared platform state (App.vue wires
// it from the backend getOS() binding), not by viewport tiers. All helpers are
// Windows-only today; see lib/platform.ts for the per-feature rationale.
const platform = usePlatform()
const showTray = computed(() => showTraySetting(platform.value))
const showApiRoute = computed(() => showApiRouteSetting(platform.value))
const showGpu = computed(() => showServingGpuSetting(platform.value))
const updatesNative = computed(() => updateSectionMode(platform.value) === 'native')
// OS-scoped gate for the directory browse buttons: Android has no native
// directory picker (the Browse* bindings error there), so on android the path
// rows stay readable but the pick buttons are hidden.
const isAndroid = computed(() => platform.value.isAndroid)

// System tray toggle: rendered on Windows only. Disabling takes effect immediately (backend removes icon and
// persists); systray cannot restart in same process, so re-enabling requires app restart (hint shown below toggle).
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
// Unavailable under wails dev (vite dev server): the relaunch escapes the wails dev
// supervisor, killing the dev session and leaving the successor without a frontend
// server. import.meta.env.DEV mirrors the backend's `dev` build-tag detection.
const apiRouteDevBlocked = import.meta.env.DEV
const apiRouteError = ref('')
const apiRouteSwitching = ref(false)

async function toggleApiRouteMode() {
  if (apiRouteSwitching.value) return
  if (apiRouteDevBlocked || !appConfig.trayEnabled) return
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
  // Read the current service access scope from the backend (default local) so the page selection matches the persisted value
  getServerConfig().then((scfg) => {
    if (scfg.accessMode === 'local' || scfg.accessMode === 'lan') {
      appConfig.serverAccessMode = scfg.accessMode
    }
    // seed the API key input from the persisted server config (empty = no authentication)
    apiKeyInput.value = scfg.apiKey || ''
    // seed the serving-GPU selection from the persisted server config (empty = auto)
    gpuValue.value = scfg.deviceId || ''
  }).catch(() => {})
  // GPU option list comes from the (cached) system info snapshot; failures
  // leave the selector disabled with the no-GPU hint.
  getSystemInfo()
    .then((info: { gpu?: GpuEntry[] }) => { gpuSnapshot.value = info.gpu ?? [] })
    .catch(() => { gpuSnapshot.value = [] })
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
  /* Use padding instead of margin: header background covers this gap so content scrolls without leaving a seam.
     Tighter than the old single-column layout: the tab row below completes the sticky block (same as ModelSettings) */
  padding-bottom: 20px;
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

/* ─── Tabs (same pattern as ModelSettings.vue) ─── */
.settings-tabs {
  display: flex;
  gap: 4px;
  padding: 0;
  border-bottom: 1px solid var(--border);
  margin-bottom: 20px;
  flex-shrink: 0;
}

.tab-btn {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 10px 14px;
  background: none;
  border: none;
  border-bottom: 2px solid transparent;
  margin-bottom: -1px;
  color: var(--text-muted);
  font-size: 13px;
  font-weight: 500;
  font-family: inherit;
  cursor: pointer;
  transition: color 0.15s, border-color 0.15s;
}

.tab-btn:hover {
  color: var(--text-secondary);
}

.tab-btn.active {
  color: var(--accent);
  border-bottom-color: var(--accent);
  font-weight: 600;
}

.tab-btn .tab-icon {
  display: flex;
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

.theme-toggle.disabled {
  cursor: not-allowed;
  opacity: 0.45;
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

/* ─── API key input (inside the access-scope section) ─── */
.api-key-row {
  margin-top: 16px;
  align-items: flex-start;
}

/* Inference GPU selector: fixed-width trigger so the row label keeps its width */
.gpu-row {
  margin-top: 16px;
  align-items: flex-start;
}

.gpu-select {
  width: 340px;
  flex-shrink: 0;
}

.api-key-input {
  width: 260px;
  padding: 8px 12px;
  background: var(--bg-primary);
  border: 1px solid var(--border);
  border-radius: 8px;
  color: var(--text-primary);
  font-size: 13px;
  font-family: var(--font-mono);
  outline: none;
  transition: border-color 0.15s;
}

.api-key-input:focus {
  border-color: var(--accent);
}

.api-key-input:disabled {
  opacity: 0.5;
  cursor: not-allowed;
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

/* Non-Windows updates hint: replaces the check action in the same row;
   right-aligned to line up under the action column */
.update-hint {
  max-width: 420px;
  text-align: right;
}

.hint-link {
  color: var(--accent);
  text-decoration: none;
  white-space: nowrap;
}

.hint-link:hover {
  text-decoration: underline;
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

/* ─── About ─── */
.about-row {
  display: flex;
  align-items: baseline;
  gap: 16px;
  padding: 9px 0;
}

.about-row + .about-row {
  border-top: 1px solid var(--border-light);
}

.about-row .setting-label {
  min-width: 72px;
}

.about-value {
  font-size: 13px;
  color: var(--text-muted);
}

/* Repository URL in monospace, kept selectable (user-select: text) so it can
   be copied; not an <a> to avoid navigating the WebView away */
.about-mono {
  font-family: var(--font-mono);
  word-break: break-all;
  user-select: text;
}

/* ─── Phone (<=767px): setting groups stay stacked full-width cards (as on
       desktop) but every row becomes thumb-friendly: the four-tab row scrolls
       inside its own container, label/control rows wrap so selects and inputs
       go full-width, and long directory paths wrap instead of pushing the row
       wide. ─── */
@media (max-width: 767px) {
  .settings-section {
    padding: 16px;
  }

  .settings-tabs {
    overflow-x: auto;
    scrollbar-width: none;
  }

  .settings-tabs::-webkit-scrollbar {
    display: none;
  }

  .tab-btn {
    flex-shrink: 0;
    padding: 12px 14px;
  }

  .setting-row {
    flex-wrap: wrap;
    gap: 12px;
  }

  /* Controls drop under their labels at full width */
  .gpu-select,
  .api-key-input {
    width: 100%;
    flex-shrink: 1;
  }

  .gpu-select :deep(.themed-select__trigger) {
    min-height: 44px;
  }

  .api-key-input {
    min-height: 44px;
  }

  /* Choice cards: single column, 44px+ touch rows */
  .source-grid,
  .source-grid-col3 {
    grid-template-columns: 1fr;
    gap: 8px;
  }

  .source-card {
    min-height: 48px;
  }

  .dir-path-row {
    flex-wrap: wrap;
  }

  .dir-btn {
    min-height: 44px;
    padding: 10px 16px;
  }

  .update-actions {
    flex-wrap: wrap;
  }

  .btn-check {
    min-height: 44px;
  }

  .update-hint {
    text-align: left;
  }
}
</style>
