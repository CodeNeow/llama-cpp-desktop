import { reactive } from 'vue'
import { getConfig, setTheme as setThemeBackend, setDownloadSource as setDownloadSourceBackend, setLanguage as setLanguageBackend } from './wails'
import { setLocale } from './lib/i18n'

export const appConfig = reactive({
  theme: localStorage.getItem('llama-gui-theme') || 'light',
  llamaCppDir: '',
  modelsDir: '',
  downloadSource: 'hf',
  language: 'auto',
  resolvedLanguage: 'zh' as 'zh' | 'en',
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
    setLocale(appConfig.resolvedLanguage)
    localStorage.setItem('llama-gui-theme', appConfig.theme)
    document.documentElement.setAttribute('data-theme', appConfig.theme)
  } catch {} finally {
    appConfig.loaded = true
  }
}

export async function setTheme(theme: string) {
  appConfig.theme = theme
  document.documentElement.setAttribute('data-theme', theme)
  localStorage.setItem('llama-gui-theme', theme)
  try {
    await setThemeBackend(theme)
  } catch {}
}

/** 切换模型下载源（"hf" | "modelscope"）：先更新本地状态，后端失败时回滚并向上抛错供 UI 提示。 */
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

/** 切换界面语言（"zh" | "en" | "auto"）：本地乐观更新偏好，后端成功后用
 * 返回的生效语言 resolvedLanguage（auto 时按系统检测结果解析）刷新 locale
 * 即时生效；后端失败回滚偏好并向上抛错供 UI 提示。 */
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
