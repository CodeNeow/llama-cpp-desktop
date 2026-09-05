import { describe, it, expect } from 'vitest'
import {
  searchQuery,
  searchResults,
  modelSizes,
  SEARCH_SUGGESTIONS,
  showsEmptySearchGuidance,
} from '../lib/downloadsState'

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
  // Portrait-band gate (768..1099px): isTablet alone mirrors the CSS media
  // band; phone and desktop tiers never render the card.
  it('renders only in the portrait tablet band', () => {
    expect(showsEmptySearchGuidance(true)).toBe(true)
    expect(showsEmptySearchGuidance(false)).toBe(false)
  })
})
