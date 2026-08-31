import { describe, it, expect } from 'vitest'
import { docSections, loadDocSection } from '../docs/manifest'
import { messages, setLocale, t } from '../lib/i18n'

// Manifest integrity for the bundled docs: every section must have BOTH a zh
// and an en markdown file resolvable through the same lazy loader the Docs
// page uses. TypeScript already enforces loader completeness at build time
// (Record<DocSectionId, ...>); these tests pin the runtime contract and the
// content hygiene rules (non-empty, no raw script tags).
describe('docs manifest', () => {
  it('keeps section ids unique', () => {
    const ids = docSections.map((s) => s.id)
    expect(new Set(ids).size).toBe(ids.length)
  })

  it('contains exactly the documented sections in display order', () => {
    expect(docSections.map((s) => s.id)).toEqual([
      'quickstart',
      'runtime',
      'downloads',
      'models',
      'api',
      'chat',
      'settings',
      'headless',
      'faq',
    ])
  })

  it('resolves a non-empty zh and en file for every section', async () => {
    for (const section of docSections) {
      const zh = await loadDocSection(section.id, 'zh')
      const en = await loadDocSection(section.id, 'en')
      expect(zh.trim().length, `zh/${section.id}.md should not be empty`).toBeGreaterThan(0)
      expect(en.trim().length, `en/${section.id}.md should not be empty`).toBeGreaterThan(0)
    }
  })

  it('markdown content contains no raw script tags (injection hygiene)', async () => {
    for (const section of docSections) {
      for (const locale of ['zh', 'en'] as const) {
        const md = await loadDocSection(section.id, locale)
        expect(md.toLowerCase()).not.toContain('<script')
      }
    }
  })

  it('section titles resolve from i18n in both locales (never a raw key)', () => {
    for (const locale of ['zh', 'en'] as const) {
      // Every titleKey must exist in its dictionary so the TOC never shows a raw key
      for (const section of docSections) {
        expect(messages[locale][section.titleKey]).toBeDefined()
      }
      setLocale(locale)
      for (const section of docSections) {
        expect(t(section.titleKey)).not.toBe(section.titleKey)
      }
    }
    setLocale('zh')
  })

  it('page-level docs keys render in both locales', () => {
    setLocale('zh')
    expect(t('docs.title')).toBe('帮助与教程')
    expect(t('docs.toc')).toBe('目录')
    expect(t('docs.loadError', { msg: 'boom' })).toContain('boom')
    setLocale('en')
    expect(t('docs.title')).toBe('Help & Tutorial')
    expect(t('docs.toc')).toBe('Contents')
    expect(t('docs.loadError', { msg: 'boom' })).toContain('boom')
    setLocale('zh')
  })
})
