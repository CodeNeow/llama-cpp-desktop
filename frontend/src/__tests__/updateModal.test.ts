// @vitest-environment happy-dom
import { describe, it, expect, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import UpdateModal, { parseReleaseBullets, stripInlineMarkdown } from '../components/UpdateModal.vue'
import { updateState } from '../lib/update'
import { setLocale } from '../lib/i18n'
import { buildPlatformState, setPlatform } from '../lib/platform'

// Release-notes bullet parser (plain-script export of the SFC) plus the
// phone-tier confirm-view rendering (frame ⑳): bare chips, bullet digest
// list, flexed footer.

describe('stripInlineMarkdown', () => {
  it('strips bold, emphasis, inline code and link targets', () => {
    expect(stripInlineMarkdown('**新** 参数')).toBe('新 参数')
    expect(stripInlineMarkdown('*斜体* 文本')).toBe('斜体 文本')
    expect(stripInlineMarkdown('支持 `深色` 主题')).toBe('支持 深色 主题')
    expect(stripInlineMarkdown('see [docs](https://example.com) now')).toBe('see docs now')
  })
})

describe('parseReleaseBullets', () => {
  it('extracts "- " bullets and strips emphasis', () => {
    const notes = ['## 中文', '', '- **新增** 聊天参数面板', '- 支持 `深色` 主题', '', '常规段落不收录'].join('\n')
    expect(parseReleaseBullets(notes)).toEqual(['新增 聊天参数面板', '支持 深色 主题'])
  })

  it('handles CRLF line endings and drops empty bullets', () => {
    expect(parseReleaseBullets('- a\r\n- \r\n-   \r\n- b\r\n')).toEqual(['a', 'b'])
  })

  it('returns an empty list for bullet-free bodies', () => {
    expect(parseReleaseBullets('plain text\nonly')).toEqual([])
    expect(parseReleaseBullets('')).toEqual([])
  })
})

function mountModal() {
  return mount(UpdateModal, { props: { visible: true } })
}

describe('UpdateModal confirm view per tier', () => {
  afterEach(() => {
    // Restore the desktop default for later tests in this file's module graph
    setPlatform(buildPlatformState('windows', Number.POSITIVE_INFINITY))
    updateState.result = null
    updateState.download = null
  })

  it('phone tier shows bare chips, the bullet list and the flexed footer copy', async () => {
    setPlatform(buildPlatformState('windows', 390))
    setLocale('zh')
    updateState.result = {
      hasUpdate: true,
      version: 'v0.4.1',
      published: '2026-08-28T00:00:00Z',
      notes: '## 中文\n- **新增** 聊天参数面板\n- 修复下载暂停',
    }
    const wrapper = mountModal()
    await flushPromises()

    const chips = wrapper.findAll('.meta-chip')
    expect(chips).toHaveLength(2)
    expect(chips[0].text()).toBe('v0.4.1')
    expect(chips[0].attributes('aria-label')).toBe('新版本：v0.4.1')
    expect(chips[1].text()).not.toContain('发布时间')
    expect(chips[1].attributes('aria-label')).toContain('2026')

    const bullets = wrapper.findAll('.notes-list li')
    expect(bullets).toHaveLength(2)
    expect(bullets[0].text()).toBe('新增 聊天参数面板')
    expect(wrapper.find('.notes-body').exists()).toBe(false)

    expect(wrapper.find('.btn-cancel').text()).toBe('稍后再说')
    expect(wrapper.find('.btn-save').text()).toBe('⬇ 下载更新')
    wrapper.unmount()
  })

  it('desktop tier keeps prefixed chips and the raw notes body', async () => {
    setPlatform(buildPlatformState('windows', Number.POSITIVE_INFINITY))
    setLocale('zh')
    updateState.result = {
      hasUpdate: true,
      version: 'v0.4.1',
      published: '2026-08-28T00:00:00Z',
      notes: '## 中文\n- **新增** 聊天参数面板',
    }
    const wrapper = mountModal()
    await flushPromises()

    expect(wrapper.find('.meta-chip').text()).toBe('新版本：v0.4.1')
    expect(wrapper.find('.notes-body').exists()).toBe(true)
    expect(wrapper.find('.notes-list').exists()).toBe(false)
    wrapper.unmount()
  })
})
