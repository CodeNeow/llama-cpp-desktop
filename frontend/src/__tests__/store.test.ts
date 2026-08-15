import { describe, it, expect, vi, beforeEach } from 'vitest'
import { appConfig, loadConfig, setTheme, setDownloadSource, setLanguage, setServerAccessMode, setTrayEnabled, setSidebarCollapsed, readStoredTheme, readStoredSidebarCollapsed } from '../store'
import { getConfig, setTheme as setThemeBackend, setDownloadSource as setDownloadSourceBackend, setLanguage as setLanguageBackend, setTrayEnabled as setTrayEnabledBackend, setSidebarCollapsed as setSidebarCollapsedBackend, getServerConfig, saveServerConfig as saveServerConfigBackend } from '../wails'
import { locale } from '../lib/i18n'

// mock Wails 桥接层：window.go 仅由 Wails 运行时注入，单测环境不可用
vi.mock('../wails', () => ({
  getConfig: vi.fn(),
  setTheme: vi.fn(),
  setDownloadSource: vi.fn(),
  setLanguage: vi.fn(),
  setTrayEnabled: vi.fn(),
  setSidebarCollapsed: vi.fn(),
  getServerConfig: vi.fn(),
  saveServerConfig: vi.fn(),
}))

const mockGetConfig = vi.mocked(getConfig)
const mockSetTheme = vi.mocked(setThemeBackend)
const mockSetDownloadSource = vi.mocked(setDownloadSourceBackend)
const mockSetLanguage = vi.mocked(setLanguageBackend)
const mockSetTrayEnabled = vi.mocked(setTrayEnabledBackend)
const mockSetSidebarCollapsed = vi.mocked(setSidebarCollapsedBackend)
const mockGetServerConfig = vi.mocked(getServerConfig)
const mockSaveServerConfig = vi.mocked(saveServerConfigBackend)

describe('store', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    localStorage.clear()
    appConfig.theme = 'light'
    appConfig.llamaCppDir = ''
    appConfig.modelsDir = ''
    appConfig.downloadSource = 'hf'
    appConfig.serverAccessMode = 'local'
    appConfig.language = 'auto'
    appConfig.resolvedLanguage = 'zh'
    appConfig.trayEnabled = true
    // 重置侧边栏收起状态，避免用例间污染（readStoredSidebarCollapsed 在模块
    // 加载时已执行一次，此处显式重置以保证每个用例从展开态开始）
    appConfig.sidebarCollapsed = false
    appConfig.loaded = false
    locale.value = 'zh'
  })

  it('loadConfig 成功时写入主题、llamaCppDir 与 modelsDir', async () => {
    mockGetConfig.mockResolvedValue({ theme: 'light', llamaCppDir: 'C:/llama-cpp', modelsDir: 'D:/models', downloadSource: 'modelscope', language: 'auto', resolvedLanguage: 'en', trayEnabled: false })

    await loadConfig()

    expect(appConfig.theme).toBe('light')
    expect(appConfig.llamaCppDir).toBe('C:/llama-cpp')
    expect(appConfig.modelsDir).toBe('D:/models')
    expect(appConfig.downloadSource).toBe('modelscope')
    expect(appConfig.trayEnabled).toBe(false)
    expect(appConfig.loaded).toBe(true)
    expect(localStorage.getItem('llama-desktop-theme')).toBe('light')
    expect(document.documentElement.getAttribute('data-theme')).toBe('light')
  })

  it('loadConfig 读 language/resolvedLanguage 并联动 locale', async () => {
    mockGetConfig.mockResolvedValue({ theme: 'dark', llamaCppDir: '', modelsDir: '', downloadSource: '', language: 'en', resolvedLanguage: 'en', trayEnabled: true })

    await loadConfig()

    expect(appConfig.language).toBe('en')
    expect(appConfig.resolvedLanguage).toBe('en')
    expect(locale.value).toBe('en')
  })

  it('loadConfig auto 解析结果 zh 时 locale 切为中文', async () => {
    mockGetConfig.mockResolvedValue({ theme: 'dark', llamaCppDir: '', modelsDir: '', downloadSource: '', language: 'auto', resolvedLanguage: 'zh', trayEnabled: true })

    await loadConfig()

    expect(appConfig.language).toBe('auto')
    expect(appConfig.resolvedLanguage).toBe('zh')
    expect(locale.value).toBe('zh')
  })

  it('loadConfig 后端未返回 modelsDir 时兜底为空串', async () => {
    mockGetConfig.mockResolvedValue({ theme: 'dark', llamaCppDir: 'C:/llama-cpp', modelsDir: '', downloadSource: '', language: 'auto', resolvedLanguage: 'zh', trayEnabled: true })

    await loadConfig()

    expect(appConfig.modelsDir).toBe('')
    expect(appConfig.theme).toBe('dark')
  })

  it('loadConfig 后端未返回 downloadSource 时兜底为 hf', async () => {
    mockGetConfig.mockResolvedValue({ theme: 'dark', llamaCppDir: '', modelsDir: '', downloadSource: '', language: 'auto', resolvedLanguage: 'zh', trayEnabled: true })

    await loadConfig()

    expect(appConfig.downloadSource).toBe('hf')
  })

  it('loadConfig 后端未返回 trayEnabled 时兜底为 true', async () => {
    // 模拟旧后端/缺字段响应：trayEnabled 不存在时 store 应保持默认 true
    //（as any 仅限测试中构造缺字段响应，与后端 GetConfig 强类型返回不一致）
    mockGetConfig.mockResolvedValue({ theme: 'dark', llamaCppDir: '', modelsDir: '', downloadSource: '', language: 'auto', resolvedLanguage: 'zh' } as any)

    await loadConfig()

    expect(appConfig.trayEnabled).toBe(true)
  })

  it('loadConfig 失败时仍标记 loaded，保留默认主题', async () => {
    mockGetConfig.mockRejectedValue(new Error('backend unavailable'))

    await loadConfig()

    expect(appConfig.theme).toBe('light')
    expect(appConfig.loaded).toBe(true)
  })

  it('setTheme 更新主题并写入 localStorage', async () => {
    mockSetTheme.mockResolvedValue(undefined)

    await setTheme('light')

    expect(appConfig.theme).toBe('light')
    expect(localStorage.getItem('llama-desktop-theme')).toBe('light')
    expect(document.documentElement.getAttribute('data-theme')).toBe('light')
  })

  it('setTheme 后端调用失败时静默降级，不抛错', async () => {
    mockSetTheme.mockRejectedValue(new Error('backend unavailable'))

    await expect(setTheme('light')).resolves.toBeUndefined()
    expect(appConfig.theme).toBe('light')
  })

  it('readStoredTheme 新键缺失时回退旧键（llama-gui 更名迁移），双键均无时 light', () => {
    expect(readStoredTheme()).toBe('light')

    // 老安装只有旧键：主题偏好应无损接续
    localStorage.setItem('llama-gui-theme', 'dark')
    expect(readStoredTheme()).toBe('dark')

    // 新键优先于旧键
    localStorage.setItem('llama-desktop-theme', 'light')
    expect(readStoredTheme()).toBe('light')
  })

  it('setTheme 只写新键，不触碰旧键（旧键留给未升级实例回退）', async () => {
    mockSetTheme.mockResolvedValue(undefined)
    localStorage.setItem('llama-gui-theme', 'dark')

    await setTheme('dark')

    expect(localStorage.getItem('llama-desktop-theme')).toBe('dark')
    expect(localStorage.getItem('llama-gui-theme')).toBe('dark')
  })

  it('setDownloadSource 成功后更新本地状态并调用后端', async () => {
    mockSetDownloadSource.mockResolvedValue(undefined)

    await setDownloadSource('modelscope')

    expect(mockSetDownloadSource).toHaveBeenCalledWith('modelscope')
    expect(appConfig.downloadSource).toBe('modelscope')
  })

  it('setDownloadSource 后端失败时回滚本地状态并向调用方抛错', async () => {
    mockSetDownloadSource.mockRejectedValue(new Error('backend unavailable'))

    await expect(setDownloadSource('modelscope')).rejects.toThrow('backend unavailable')
    expect(appConfig.downloadSource).toBe('hf')
  })

  it('setLanguage 成功后用后端返回的生效语言刷新 locale', async () => {
    mockSetLanguage.mockResolvedValue('en')

    await setLanguage('en')

    expect(mockSetLanguage).toHaveBeenCalledWith('en')
    expect(appConfig.language).toBe('en')
    expect(appConfig.resolvedLanguage).toBe('en')
    expect(locale.value).toBe('en')
  })

  it('setLanguage auto 时按后端检测结果刷新 locale', async () => {
    mockSetLanguage.mockResolvedValue('zh')

    await setLanguage('auto')

    expect(appConfig.language).toBe('auto')
    expect(appConfig.resolvedLanguage).toBe('zh')
    expect(locale.value).toBe('zh')
  })

  it('setLanguage 后端失败时回滚偏好并向调用方抛错', async () => {
    mockSetLanguage.mockRejectedValue(new Error('backend unavailable'))

    await expect(setLanguage('en')).rejects.toThrow('backend unavailable')
    expect(appConfig.language).toBe('auto')
  })

  it('setTrayEnabled 成功后更新本地状态并调用后端', async () => {
    mockSetTrayEnabled.mockResolvedValue(undefined)

    await setTrayEnabled(false)

    expect(mockSetTrayEnabled).toHaveBeenCalledWith(false)
    expect(appConfig.trayEnabled).toBe(false)
  })

  it('setTrayEnabled 后端失败时回滚本地状态并向调用方抛错', async () => {
    mockSetTrayEnabled.mockRejectedValue(new Error('backend unavailable'))

    await expect(setTrayEnabled(false)).rejects.toThrow('backend unavailable')
    expect(appConfig.trayEnabled).toBe(true)
  })

  it('setServerAccessMode 成功后取完整配置并带 accessMode 保存，更新本地状态', async () => {
    mockGetServerConfig.mockResolvedValue({ accessMode: 'local', host: '127.0.0.1', port: 8080, maxModels: 1, cacheRam: 8192 })
    mockSaveServerConfig.mockResolvedValue(undefined)

    await setServerAccessMode('lan')

    expect(appConfig.serverAccessMode).toBe('lan')
    // 保存的是后端最新完整配置（含用户可能在别处设置的字段），仅覆盖 accessMode
    expect(mockSaveServerConfig).toHaveBeenCalledWith({ accessMode: 'lan', host: '127.0.0.1', port: 8080, maxModels: 1, cacheRam: 8192 })
  })

  it('setServerAccessMode 取配置失败时回滚本地状态并向调用方抛错', async () => {
    mockGetServerConfig.mockRejectedValue(new Error('backend unavailable'))

    await expect(setServerAccessMode('lan')).rejects.toThrow('backend unavailable')
    expect(appConfig.serverAccessMode).toBe('local')
    expect(mockSaveServerConfig).not.toHaveBeenCalled()
  })

  it('setServerAccessMode 保存失败时回滚本地状态并向调用方抛错', async () => {
    mockGetServerConfig.mockResolvedValue({ accessMode: 'local', host: '127.0.0.1', port: 8080, maxModels: 1, cacheRam: 8192 })
    mockSaveServerConfig.mockRejectedValue(new Error('backend unavailable'))

    await expect(setServerAccessMode('lan')).rejects.toThrow('backend unavailable')
    expect(appConfig.serverAccessMode).toBe('local')
  })

  it('loadConfig 后端返回 sidebarCollapsed: true 时状态收起并写 localStorage', async () => {
    mockGetConfig.mockResolvedValue({ theme: 'dark', llamaCppDir: '', modelsDir: '', downloadSource: '', language: 'auto', resolvedLanguage: 'zh', trayEnabled: true, sidebarCollapsed: true })

    await loadConfig()

    expect(appConfig.sidebarCollapsed).toBe(true)
    expect(localStorage.getItem('llama-desktop-sidebar-collapsed')).toBe('1')
  })

  it('loadConfig 后端未返回 sidebarCollapsed（旧配置）时兜底为展开', async () => {
    // 模拟旧后端/缺字段响应：sidebarCollapsed 不存在时 store 应兜底 false（展开）
    mockGetConfig.mockResolvedValue({ theme: 'dark', llamaCppDir: '', modelsDir: '', downloadSource: '', language: 'auto', resolvedLanguage: 'zh', trayEnabled: true } as any)

    await loadConfig()

    expect(appConfig.sidebarCollapsed).toBe(false)
    expect(localStorage.getItem('llama-desktop-sidebar-collapsed')).toBe('0')
  })

  it('loadConfig 后端返回 sidebarCollapsed: false 时保持展开', async () => {
    appConfig.sidebarCollapsed = true // 先置非默认值，验证 false 覆盖为展开
    mockGetConfig.mockResolvedValue({ theme: 'dark', llamaCppDir: '', modelsDir: '', downloadSource: '', language: 'auto', resolvedLanguage: 'zh', trayEnabled: true, sidebarCollapsed: false })

    await loadConfig()

    expect(appConfig.sidebarCollapsed).toBe(false)
    expect(localStorage.getItem('llama-desktop-sidebar-collapsed')).toBe('0')
  })

  it('setSidebarCollapsed 成功后更新状态、写 localStorage 并调用后端', async () => {
    mockSetSidebarCollapsed.mockResolvedValue(undefined)

    await setSidebarCollapsed(true)

    expect(mockSetSidebarCollapsed).toHaveBeenCalledWith(true)
    expect(appConfig.sidebarCollapsed).toBe(true)
    expect(localStorage.getItem('llama-desktop-sidebar-collapsed')).toBe('1')
  })

  it('setSidebarCollapsed 后端失败时吞错不回滚（纯 UI 偏好，失败仅影响下次启动恢复值）', async () => {
    mockSetSidebarCollapsed.mockRejectedValue(new Error('backend unavailable'))

    await expect(setSidebarCollapsed(true)).resolves.toBeUndefined()
    expect(appConfig.sidebarCollapsed).toBe(true)
    expect(localStorage.getItem('llama-desktop-sidebar-collapsed')).toBe('1')
  })

  it('readStoredSidebarCollapsed: localStorage "1" 为收起，空/其他为展开', () => {
    expect(readStoredSidebarCollapsed()).toBe(false)

    localStorage.setItem('llama-desktop-sidebar-collapsed', '1')
    expect(readStoredSidebarCollapsed()).toBe(true)

    // 显式写入 '0' 展开（与 setSidebarCollapsed 写回格式一致）
    localStorage.setItem('llama-desktop-sidebar-collapsed', '0')
    expect(readStoredSidebarCollapsed()).toBe(false)
  })
})
