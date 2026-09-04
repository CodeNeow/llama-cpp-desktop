import { describe, it, expect } from 'vitest'
import { downloadSourceLabelKey, searchQuery, searchResults, modelSizes } from '../lib/downloadsState'

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
