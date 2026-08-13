import { reactive } from 'vue'
import { getConfig, setTheme as setThemeBackend, setDownloadSource as setDownloadSourceBackend } from './wails'

export const appConfig = reactive({
  theme: localStorage.getItem('llama-gui-theme') || 'light',
  llamaCppDir: '',
  modelsDir: '',
  downloadSource: 'hf',
  loaded: false,
})

export async function loadConfig() {
  try {
    const config = await getConfig()
    appConfig.theme = config.theme || 'light'
    appConfig.llamaCppDir = config.llamaCppDir || ''
    appConfig.modelsDir = config.modelsDir || ''
    appConfig.downloadSource = config.downloadSource || 'hf'
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
