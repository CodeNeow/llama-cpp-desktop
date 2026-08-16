import { describe, it, expect, beforeEach } from 'vitest'
import { locale, setLocale, t } from '../lib/i18n'

describe('lib/i18n', () => {
  beforeEach(() => {
    setLocale('zh')
  })

  it('t gets current locale dict value (zh)', () => {
    expect(t('nav.home')).toBe('系统信息')
    expect(t('settings.themeMode')).toBe('主题模式')
  })

  it('t returns key as-is when missing', () => {
    expect(t('no.such.key')).toBe('no.such.key')
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
