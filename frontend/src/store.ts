import { reactive } from 'vue'
import { getConfig, setTheme as setThemeBackend, setSidebarCollapsed as setSidebarCollapsedBackend, setDownloadSource as setDownloadSourceBackend, setLanguage as setLanguageBackend, setTrayEnabled as setTrayEnabledBackend, getServerConfig, saveServerConfig as saveServerConfigBackend } from './wails'
import { setLocale } from './lib/i18n'

// 主题 localStorage 键：llama-gui → llama-desktop 更名后旧键仅作读取回退
// （老安装的主题偏好无损接续），写入一律走新键，不删除旧键。
const THEME_KEY = 'llama-desktop-theme'
const LEGACY_THEME_KEY = 'llama-gui-theme'

// 侧边栏收起状态 localStorage 键：'1' 收起、'0'/缺省 展开。
// 主题/托盘偏好双轨模式一致：UI 先读 localStorage 首帧即正确，后端 config 仅
// 作持久化来源（loadConfig 覆盖并同步写回 localStorage）。
const SIDEBAR_KEY = 'llama-desktop-sidebar-collapsed'

/** 读取持久化主题：新键优先，缺失时回退更名前旧键，均无则 light。 */
export function readStoredTheme(): string {
  return localStorage.getItem(THEME_KEY) || localStorage.getItem(LEGACY_THEME_KEY) || 'light'
}

/** 读取持久化侧边栏收起状态：'1' 为收起，其余（含缺省）一律展开。 */
export function readStoredSidebarCollapsed(): boolean {
  return localStorage.getItem(SIDEBAR_KEY) === '1'
}

export const appConfig = reactive({
  theme: readStoredTheme(),
  llamaCppDir: '',
  modelsDir: '',
  downloadSource: 'hf',
  serverAccessMode: 'local',
  language: 'auto',
  resolvedLanguage: 'zh' as 'zh' | 'en',
  // Windows 系统托盘开关：默认 true（与后端 loadConfig 兜底一致）；仅在
  // loadConfig 拉取到后端持久化值后覆盖。
  trayEnabled: true,
  // 侧边栏收起状态：启动首帧从 localStorage 读取（'1' 收起），避免异步
  // loadConfig 返回前侧边栏按展开宽度渲染后再收缩造成布局跳动。
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
    // 后端缺省/未返回 trayEnabled 时保持默认 true（与后端 loadConfig 兜底一致）
    appConfig.trayEnabled = config.trayEnabled !== false
    // 后端缺字段/旧配置 → undefined → false（展开）；成功后同步写回
    // localStorage，保证「后端持久化值」与「本机 UI 缓存」双轨一致。
    appConfig.sidebarCollapsed = config.sidebarCollapsed === true
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

/** 切换侧边栏收起/展开：先乐观更新本地状态并写 localStorage，后端调用失败
 * 时吞错不回滚——纯 UI 偏好，切换视觉状态立即回滚会闪烁，失败仅影响下次
 * 启动的恢复值（回滚模式见 setTrayEnabled，那里回滚不会造成视觉抖动）。
 */
export async function setSidebarCollapsed(collapsed: boolean) {
  appConfig.sidebarCollapsed = collapsed
  localStorage.setItem(SIDEBAR_KEY, collapsed ? '1' : '0')
  try {
    await setSidebarCollapsedBackend(collapsed)
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
