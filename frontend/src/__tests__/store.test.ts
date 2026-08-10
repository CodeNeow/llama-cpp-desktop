import { describe, it, expect, vi, beforeEach } from 'vitest'
import { appConfig, loadConfig, setTheme } from '../store'
import { getConfig, setTheme as setThemeBackend } from '../wails'

// mock Wails 桥接层：window.go 仅由 Wails 运行时注入，单测环境不可用
vi.mock('../wails', () => ({
  getConfig: vi.fn(),
  setTheme: vi.fn(),
}))

const mockGetConfig = vi.mocked(getConfig)
const mockSetTheme = vi.mocked(setThemeBackend)

describe('store', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    localStorage.clear()
    appConfig.theme = 'dark'
    appConfig.llamaCppDir = ''
    appConfig.loaded = false
  })

  it('loadConfig 成功时写入主题与 llamaCppDir', async () => {
    mockGetConfig.mockResolvedValue({ theme: 'light', llamaCppDir: 'C:/llama-cpp' })

    await loadConfig()

    expect(appConfig.theme).toBe('light')
    expect(appConfig.llamaCppDir).toBe('C:/llama-cpp')
    expect(appConfig.loaded).toBe(true)
    expect(localStorage.getItem('llama-gui-theme')).toBe('light')
    expect(document.documentElement.getAttribute('data-theme')).toBe('light')
  })

  it('loadConfig 失败时仍标记 loaded，保留默认主题', async () => {
    mockGetConfig.mockRejectedValue(new Error('backend unavailable'))

    await loadConfig()

    expect(appConfig.theme).toBe('dark')
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
})
