import { describe, it, expect, vi, beforeEach } from 'vitest'
import { appConfig, loadConfig, setTheme, setDownloadSource, setLanguage } from '../store'
import { getConfig, setTheme as setThemeBackend, setDownloadSource as setDownloadSourceBackend, setLanguage as setLanguageBackend } from '../wails'
import { locale } from '../lib/i18n'

// mock Wails 桥接层：window.go 仅由 Wails 运行时注入，单测环境不可用
vi.mock('../wails', () => ({
  getConfig: vi.fn(),
  setTheme: vi.fn(),
  setDownloadSource: vi.fn(),
  setLanguage: vi.fn(),
}))

const mockGetConfig = vi.mocked(getConfig)
const mockSetTheme = vi.mocked(setThemeBackend)
const mockSetDownloadSource = vi.mocked(setDownloadSourceBackend)
const mockSetLanguage = vi.mocked(setLanguageBackend)

describe('store', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    localStorage.clear()
    appConfig.theme = 'light'
    appConfig.llamaCppDir = ''
    appConfig.modelsDir = ''
    appConfig.downloadSource = 'hf'
    appConfig.language = 'auto'
    appConfig.resolvedLanguage = 'zh'
    appConfig.loaded = false
    locale.value = 'zh'
  })

  it('loadConfig 成功时写入主题、llamaCppDir 与 modelsDir', async () => {
    mockGetConfig.mockResolvedValue({ theme: 'light', llamaCppDir: 'C:/llama-cpp', modelsDir: 'D:/models', downloadSource: 'modelscope', language: 'auto', resolvedLanguage: 'en' })

    await loadConfig()

    expect(appConfig.theme).toBe('light')
    expect(appConfig.llamaCppDir).toBe('C:/llama-cpp')
    expect(appConfig.modelsDir).toBe('D:/models')
    expect(appConfig.downloadSource).toBe('modelscope')
    expect(appConfig.loaded).toBe(true)
    expect(localStorage.getItem('llama-gui-theme')).toBe('light')
    expect(document.documentElement.getAttribute('data-theme')).toBe('light')
  })

  it('loadConfig 读 language/resolvedLanguage 并联动 locale', async () => {
    mockGetConfig.mockResolvedValue({ theme: 'dark', llamaCppDir: '', modelsDir: '', downloadSource: '', language: 'en', resolvedLanguage: 'en' })

    await loadConfig()

    expect(appConfig.language).toBe('en')
    expect(appConfig.resolvedLanguage).toBe('en')
    expect(locale.value).toBe('en')
  })

  it('loadConfig auto 解析结果 zh 时 locale 切为中文', async () => {
    mockGetConfig.mockResolvedValue({ theme: 'dark', llamaCppDir: '', modelsDir: '', downloadSource: '', language: 'auto', resolvedLanguage: 'zh' })

    await loadConfig()

    expect(appConfig.language).toBe('auto')
    expect(appConfig.resolvedLanguage).toBe('zh')
    expect(locale.value).toBe('zh')
  })

  it('loadConfig 后端未返回 modelsDir 时兜底为空串', async () => {
    mockGetConfig.mockResolvedValue({ theme: 'dark', llamaCppDir: 'C:/llama-cpp', modelsDir: '', downloadSource: '', language: 'auto', resolvedLanguage: 'zh' })

    await loadConfig()

    expect(appConfig.modelsDir).toBe('')
    expect(appConfig.theme).toBe('dark')
  })

  it('loadConfig 后端未返回 downloadSource 时兜底为 hf', async () => {
    mockGetConfig.mockResolvedValue({ theme: 'dark', llamaCppDir: '', modelsDir: '', downloadSource: '', language: 'auto', resolvedLanguage: 'zh' })

    await loadConfig()

    expect(appConfig.downloadSource).toBe('hf')
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
    expect(localStorage.getItem('llama-gui-theme')).toBe('light')
    expect(document.documentElement.getAttribute('data-theme')).toBe('light')
  })

  it('setTheme 后端调用失败时静默降级，不抛错', async () => {
    mockSetTheme.mockRejectedValue(new Error('backend unavailable'))

    await expect(setTheme('light')).resolves.toBeUndefined()
    expect(appConfig.theme).toBe('light')
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
})
