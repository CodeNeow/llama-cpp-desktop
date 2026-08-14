import { reactive } from 'vue'
import { getConfig, setTheme as setThemeBackend, setDownloadSource as setDownloadSourceBackend, setLanguage as setLanguageBackend, setTrayEnabled as setTrayEnabledBackend, getServerConfig, saveServerConfig as saveServerConfigBackend } from './wails'
import { setLocale } from './lib/i18n'

export const appConfig = reactive({
  theme: localStorage.getItem('llama-gui-theme') || 'light',
  llamaCppDir: '',
  modelsDir: '',
  downloadSource: 'hf',
  serverAccessMode: 'local',
  language: 'auto',
  resolvedLanguage: 'zh' as 'zh' | 'en',
  // Windows 系统托盘开关：默认 true（与后端 loadConfig 兜底一致）；仅在
  // loadConfig 拉取到后端持久化值后覆盖。
  trayEnabled: true,
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
    // 后端缺省/未返回 trayEnabled 时保持默认 true（与后端 loadConfig 兜底一致）
    appConfig.trayEnabled = config.trayEnabled !== false
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

/** 切换服务访问范围（"local" | "lan"）：先乐观更新本地状态，再取后端最新
 * 完整 serverConfig 修改 accessMode 后整体保存（避免覆盖用户在同一页设置的
 * 其他字段）；后端失败时回滚本地状态并向上抛错供 UI 提示。saveServerConfig
 * 返回 void，成功后后端已持久化派生 host，无需再刷新。 */
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

/** 切换 Windows 系统托盘开关：本地乐观更新，后端持久化成功即生效
 * （禁用时立即摘托盘图标；再次启用需重启应用，见 wails.ts 注释）。
 * 后端失败回滚本地状态并向上抛错供 UI 提示。 */
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
