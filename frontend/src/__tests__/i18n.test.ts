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
    expect(t('nav.home')).toBe('系统信息')
    expect(t('settings.themeMode')).toBe('主题模式')
  })

  it('t returns key as-is when missing', () => {
    expect(t('no.such.key')).toBe('no.such.key')
  })

  it('apiRouteMode settings keys render in both locales and mention the tray return path', () => {
    setLocale('zh')
    expect(t('settings.apiRouteMode')).toBe('API 路由模式')
    expect(t('settings.apiRouteModeDesc')).toContain('显示主窗口')
    expect(t('settings.apiRouteModeRequiresTray')).toContain('系统托盘')
    expect(t('settings.apiRouteModeError')).not.toBe('settings.apiRouteModeError')
    setLocale('en')
    expect(t('settings.apiRouteMode')).toBe('API Route Mode')
    expect(t('settings.apiRouteModeDesc')).toContain('Show Main Window')
    expect(t('settings.apiRouteModeRequiresTray')).toContain('system tray')
    expect(t('settings.apiRouteModeError')).not.toBe('settings.apiRouteModeError')
  })

  it('t supports {name} placeholder interpolation', () => {
    expect(t('home.cpu.coresValue', { n: 8 })).toBe('8 核')
    expect(t('models.saveFailed', { msg: 'boom' })).toBe('保存失败: boom')
  })

  it('after setLocale, t returns corresponding language', () => {
    setLocale('en')
    expect(t('nav.home')).toBe('System Info')
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
