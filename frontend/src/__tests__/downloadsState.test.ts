import { describe, it, expect } from 'vitest'
import {
  downloadSourceLabelKey,
  searchQuery,
  searchResults,
  modelSizes,
  SEARCH_SUGGESTIONS,
  showsEmptySearchGuidance,
} from '../lib/downloadsState'

describe('downloadSourceLabelKey', () => {
  // Tablet download-tab summary card (draft B8 "下载源 HF 镜像"): maps the
  // persisted source value onto the shared short-label keys; unknown values
  // degrade to the hf-mirror label (the store default)
  it('maps each source value to its short-label key', () => {
    expect(downloadSourceLabelKey('hf')).toBe('settings.sourceHfShort')
    expect(downloadSourceLabelKey('huggingface')).toBe('settings.sourceOfficialShort')
    expect(downloadSourceLabelKey('modelscope')).toBe('settings.sourceMsShort')
  })

  it('unknown values degrade to the hf-mirror label key', () => {
    expect(downloadSourceLabelKey('')).toBe('settings.sourceHfShort')
    expect(downloadSourceLabelKey('other')).toBe('settings.sourceHfShort')
  })
})

describe('module-level search state', () => {
  // Regression guard for the hoisted state contract: the module starts empty
  // so tab switches / detail round-trips re-render from a clean baseline.
  it('starts empty', () => {
    expect(searchQuery.value).toBe('')
    expect(searchResults.value).toEqual([])
    expect(modelSizes).toEqual({})
  })
})

describe('SEARCH_SUGGESTIONS', () => {
  // Empty-search guidance card chips (portrait band): 2-3 tappable keywords
  // that fill the search box — safe-by-construction content only.
  it('offers two to three unique, non-blank keywords', () => {
    expect(SEARCH_SUGGESTIONS.length).toBeGreaterThanOrEqual(2)
    expect(SEARCH_SUGGESTIONS.length).toBeLessThanOrEqual(3)
    expect(new Set(SEARCH_SUGGESTIONS).size).toBe(SEARCH_SUGGESTIONS.length)
    for (const s of SEARCH_SUGGESTIONS) {
      expect(s.trim().length).toBeGreaterThan(0)
    }
  })
})

describe('showsEmptySearchGuidance', () => {
  // Portrait-band gate (768..1099px): isTablet alone also covers the Android
  // landscape-tablet band, so isTabletLandscape must subtract it — the
  // landscape track and every other tier keep their rendering unchanged.
  it('renders only in the portrait tablet band', () => {
    expect(showsEmptySearchGuidance(true, false)).toBe(true)
    expect(showsEmptySearchGuidance(false, false)).toBe(false)
    expect(showsEmptySearchGuidance(true, true)).toBe(false)
    expect(showsEmptySearchGuidance(false, true)).toBe(false)
  })
})
