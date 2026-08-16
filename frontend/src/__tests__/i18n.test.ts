import { describe, it, expect, beforeEach } from 'vitest'
import { locale, setLocale, t } from '../lib/i18n'

describe('lib/i18n', () => {
  beforeEach(() => {
    setLocale('zh')
  })

  it('t 取当前语言字典的值（zh）', () => {
    expect(t('nav.home')).toBe('系统信息')
    expect(t('settings.themeMode')).toBe('主题模式')
  })

  it('t 缺失 key 时原样返回 key', () => {
    expect(t('no.such.key')).toBe('no.such.key')
  })

  it('t 支持 {name} 占位符插值', () => {
    expect(t('home.cpu.coresValue', { n: 8 })).toBe('8 核')
    expect(t('models.saveFailed', { msg: 'boom' })).toBe('保存失败: boom')
  })

  it('setLocale 切换语言后 t 返回对应语言', () => {
    setLocale('en')
    expect(t('nav.home')).toBe('System Info')
    expect(t('settings.themeMode')).toBe('Theme Mode')
    // 英文插值
    expect(t('home.cpu.coresValue', { n: 8 })).toBe('8 cores')
  })

  it('locale 默认 zh，setLocale 更新 locale.value', () => {
    expect(locale.value).toBe('zh')
    setLocale('en')
    expect(locale.value).toBe('en')
    setLocale('zh')
    expect(locale.value).toBe('zh')
  })
})
