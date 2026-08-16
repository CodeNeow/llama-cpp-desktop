import { reactive } from 'vue'
import { getConfig, setTheme as setThemeBackend, setSidebarCollapsed as setSidebarCollapsedBackend, setDownloadSource as setDownloadSourceBackend, setLanguage as setLanguageBackend, setTrayEnabled as setTrayEnabledBackend, getServerConfig, saveServerConfig as saveServerConfigBackend } from './wails'
import { setLocale } from './lib/i18n'

// Theme localStorage key: after the llama-gui → llama-desktop rename the legacy key is read-only fallback
// (preserving theme preference of old installs); writes always use the new key and never delete the old one.
const THEME_KEY = 'llama-desktop-theme'
const LEGACY_THEME_KEY = 'llama-gui-theme'

// Sidebar collapsed-state localStorage key: '0' explicitly expanded, '1'/missing collapsed.
// Same dual-track pattern as theme/tray preferences: UI reads localStorage for a correct
// first frame, while the backend config is only the persistence source (loadConfig
// overwrites it and syncs back to localStorage).
const SIDEBAR_KEY = 'llama-desktop-sidebar-collapsed'

/** Read the persisted theme: prefer the new key, fall back to the pre-rename legacy key, else 'light'. */
export function readStoredTheme(): string {
  return localStorage.getItem(THEME_KEY) || localStorage.getItem(LEGACY_THEME_KEY) || 'light'
}

/** Read the persisted sidebar collapsed state: '0' means explicitly expanded, anything else ('1'/missing) collapsed.
 * First run (key missing) defaults to collapsed to avoid the layout jump of rendering expanded then shrinking. */
export function readStoredSidebarCollapsed(): boolean {
  return localStorage.getItem(SIDEBAR_KEY) !== '0'
}

export const appConfig = reactive({
  theme: readStoredTheme(),
  llamaCppDir: '',
  modelsDir: '',
  downloadSource: 'hf',
  serverAccessMode: 'local',
  language: 'auto',
  resolvedLanguage: 'zh' as 'zh' | 'en',
  // Windows system tray toggle: default true (matches the backend loadConfig fallback);
  // only overridden once loadConfig fetches the persisted backend value.
  trayEnabled: true,
  // Sidebar collapsed state: read from localStorage for the first frame ('0' expanded,
  // otherwise collapsed) so the sidebar does not render at the wrong width before the
  // async loadConfig returns; defaults to collapsed when the key is missing (matching
  // the backend's preset true fallback).
  sidebarCollapsed: readStoredSidebarCollapsed(),
  loaded: false,
})

export async function loadConfig() {
  try {
    const config = await getConfig()
    appConfig.theme = config.theme || 'light'
    appConfig.llamaCppDir = config.llamaCppDir || ''
    appConfig.modelsDir = config.modelsDir || ''
    appConfig.downloadSource = config.downloadSource || 'hf'
    appConfig.language = config.language || 'auto'
    appConfig.resolvedLanguage = config.resolvedLanguage === 'en' ? 'en' : 'zh'
    // Keep the default true when the backend omits/does not return trayEnabled (matches the backend loadConfig fallback)
    appConfig.trayEnabled = config.trayEnabled !== false
    // Backend field missing / old backend not returning it → undefined → default collapsed
    // (matching the backend loadConfig preset true fallback); only an explicit false
    // (user's expanded preference) yields false; on success sync back to localStorage
    // so the "persisted backend value" and the "local UI cache" stay consistent.
    appConfig.sidebarCollapsed = config.sidebarCollapsed !== false
    setLocale(appConfig.resolvedLanguage)
    localStorage.setItem(THEME_KEY, appConfig.theme)
    localStorage.setItem(SIDEBAR_KEY, appConfig.sidebarCollapsed ? '1' : '0')
    document.documentElement.setAttribute('data-theme', appConfig.theme)
  } catch {} finally {
    appConfig.loaded = true
  }
}

export async function setTheme(theme: string) {
  appConfig.theme = theme
  document.documentElement.setAttribute('data-theme', theme)
  localStorage.setItem(THEME_KEY, theme)
  try {
    await setThemeBackend(theme)
  } catch {}
}

/** Toggle sidebar collapsed/expanded: optimistically update local state and write
 * localStorage first; swallow backend errors without rollback — it is a pure UI
 * preference, and an immediate visual rollback would flicker; failure only affects
 * the value restored on next launch (for the rollback pattern see setTrayEnabled,
 * where rolling back causes no visual jitter).
 */
export async function setSidebarCollapsed(collapsed: boolean) {
  appConfig.sidebarCollapsed = collapsed
  localStorage.setItem(SIDEBAR_KEY, collapsed ? '1' : '0')
  try {
    await setSidebarCollapsedBackend(collapsed)
  } catch {}
}

/** Switch model download source ("hf" | "modelscope"): update local state first; on backend failure roll back and rethrow for UI feedback. */
export async function setDownloadSource(source: string) {
  const previous = appConfig.downloadSource
  appConfig.downloadSource = source
  try {
    await setDownloadSourceBackend(source)
  } catch (e) {
    appConfig.downloadSource = previous
    throw e
  }
}

/** Switch service access scope ("local" | "lan"): optimistically update local state,
 * then fetch the latest full serverConfig from the backend, change accessMode, and save
 * it whole (avoiding clobbering other fields the user set on the same page); on backend
 * failure roll back local state and rethrow for UI feedback. saveServerConfig returns
 * void; after success the backend has persisted the derived host, no refresh needed. */
export async function setServerAccessMode(mode: string) {
  const previous = appConfig.serverAccessMode
  appConfig.serverAccessMode = mode
  try {
    const scfg = await getServerConfig()
    scfg.accessMode = mode
    await saveServerConfigBackend(scfg)
  } catch (e) {
    appConfig.serverAccessMode = previous
    throw e
  }
}

/** Switch UI language ("zh" | "en" | "auto"): optimistically update the preference
 * locally; on backend success refresh the locale with the returned effective
 * resolvedLanguage (for auto, resolved from the system detection) so it applies
 * immediately; on backend failure roll back the preference and rethrow for UI feedback. */
export async function setLanguage(language: string) {
  const previous = appConfig.language
  appConfig.language = language
  try {
    const resolved = await setLanguageBackend(language)
    const l = resolved === 'en' ? 'en' : 'zh'
    appConfig.resolvedLanguage = l
    setLocale(l)
  } catch (e) {
    appConfig.language = previous
    throw e
  }
}

/** Toggle the Windows system tray switch: optimistic local update, takes effect once
 * the backend persists it (disabling removes the tray icon immediately; re-enabling
 * requires an app restart, see the comment in wails.ts).
 * On backend failure roll back local state and rethrow for UI feedback. */
export async function setTrayEnabled(enabled: boolean) {
  const previous = appConfig.trayEnabled
  appConfig.trayEnabled = enabled
  try {
    await setTrayEnabledBackend(enabled)
  } catch (e) {
    appConfig.trayEnabled = previous
    throw e
  }
}
