import { describe, it, expect, beforeEach } from 'vitest'
import { locale, setLocale, t, messages } from '../lib/i18n'

describe('lib/i18n', () => {
  beforeEach(() => {
    setLocale('zh')
  })

  it('zh and en dictionaries stay symmetric (same key set)', () => {
    const zhKeys = Object.keys(messages.zh).sort()
    const enKeys = Object.keys(messages.en).sort()
    expect(enKeys).toEqual(zhKeys)
  })

  it('t gets current locale dict value (zh)', () => {
    expect(t('nav.home')).toBe('本机')
    expect(t('settings.themeMode')).toBe('主题模式')
  })

  it('t returns key as-is when missing', () => {
    expect(t('no.such.key')).toBe('no.such.key')
  })

  it('apiRouteMode settings keys render in both locales and mention the tray return path', () => {
    setLocale('zh')
    expect(t('settings.apiRouteMode')).toBe('API 路由模式')
    expect(t('settings.apiRouteModeDesc')).toContain('显示主窗口')
    expect(t('settings.apiRouteModeDevBlocked')).toContain('wails dev')
    expect(t('settings.apiRouteModeRequiresTray')).toContain('系统托盘')
    expect(t('settings.apiRouteModeError')).not.toBe('settings.apiRouteModeError')
    setLocale('en')
    expect(t('settings.apiRouteMode')).toBe('API Route Mode')
    expect(t('settings.apiRouteModeDesc')).toContain('Show Main Window')
    expect(t('settings.apiRouteModeDevBlocked')).toContain('wails dev')
    expect(t('settings.apiRouteModeRequiresTray')).toContain('system tray')
    expect(t('settings.apiRouteModeError')).not.toBe('settings.apiRouteModeError')
  })

  it('about settings keys render in both locales', () => {
    setLocale('zh')
    expect(t('settings.about')).toBe('关于')
    expect(t('settings.version')).toBe('版本')
    expect(t('settings.license')).toBe('开源协议')
    expect(t('settings.repo')).toBe('项目仓库')
    setLocale('en')
    expect(t('settings.about')).toBe('About')
    expect(t('settings.version')).toBe('Version')
    expect(t('settings.license')).toBe('License')
    expect(t('settings.repo')).toBe('Repository')
  })

  it('runtime section and shared download labels render in both locales', () => {
    setLocale('zh')
    expect(t('runtime.llamacpp.path')).toBe('安装路径')
    expect(t('runtime.downloadDir')).toBe('下载路径')
    expect(t('runtime.compMain')).toContain('llama-server')
    expect(t('runtime.compCudart')).toContain('CUDA')
    expect(t('runtime.pkgCudart')).toContain('CUDA')
    expect(t('dl.downloading')).toBe('正在下载')
    expect(t('dl.error')).toBe('下载失败')
    setLocale('en')
    expect(t('runtime.llamacpp.path')).toBe('Install Path')
    expect(t('runtime.downloadDir')).toBe('Download Path')
    expect(t('runtime.compMain')).toContain('llama-server')
    expect(t('runtime.compCudart')).toContain('CUDA')
    expect(t('runtime.pkgCudart')).toContain('CUDA')
    expect(t('dl.downloading')).toBe('Downloading')
    expect(t('dl.error')).toBe('Download failed')
  })

  it('t supports {name} placeholder interpolation', () => {
    expect(t('home.cpu.coresValue', { n: 8 })).toBe('8 核')
    expect(t('models.saveFailed', { msg: 'boom' })).toBe('保存失败: boom')
  })

  it('models tune keys render in both locales with interpolated tune results', () => {
    setLocale('zh')
    expect(t('models.tune')).not.toBe('models.tune')
    expect(t('models.tuneError', { msg: 'boom' })).toContain('boom')
    expect(t('models.tuneError', { msg: 'boom' })).not.toBe('models.tuneError')
    const zhTuned = t('models.tuned', { gpu: 'all', ctx: 32768, cache: 'f16', threads: 8 })
    expect(zhTuned).not.toBe('models.tuned')
    expect(zhTuned).toContain('32768')
    setLocale('en')
    expect(t('models.tune')).not.toBe('models.tune')
    expect(t('models.tuneError', { msg: 'boom' })).toContain('boom')
    expect(t('models.tuneError', { msg: 'boom' })).not.toBe('models.tuneError')
    const enTuned = t('models.tuned', { gpu: 'all', ctx: 32768, cache: 'f16', threads: 8 })
    expect(enTuned).not.toBe('models.tuned')
    expect(enTuned).toContain('32768')
  })

  it('after setLocale, t returns corresponding language', () => {
    setLocale('en')
    expect(t('nav.home')).toBe('System')
    expect(t('settings.themeMode')).toBe('Theme Mode')
    // English interpolation
    expect(t('home.cpu.coresValue', { n: 8 })).toBe('8 cores')
  })

  it('locale defaults to zh, setLocale updates locale.value', () => {
    expect(locale.value).toBe('zh')
    setLocale('en')
    expect(locale.value).toBe('en')
    setLocale('zh')
    expect(locale.value).toBe('zh')
  })
})
