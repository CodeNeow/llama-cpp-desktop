import { describe, it, expect } from 'vitest'
import { sortModelFiles, guessQuant } from '../lib/modelFiles'

describe('sortModelFiles', () => {
  // sort by size descending; missing size defaults to 0 (sorted to end)
  it('sort by size descending', () => {
    const files = [
      { filename: 'a.bin', size: 100 },
      { filename: 'b.bin', size: 500 },
      { filename: 'c.bin', size: 200 },
    ]
    const result = sortModelFiles(files)
    expect(result.map(f => f.filename)).toEqual(['b.bin', 'c.bin', 'a.bin'])
  })

  it('missing size defaults to 0 and sorts to end', () => {
    const files = [
      { filename: 'a.bin', size: 100 },
      { filename: 'b.bin' },
      { filename: 'c.bin' },
    ]
    const result = sortModelFiles(files)
    expect(result.map(f => f.filename)).toEqual(['a.bin', 'b.bin', 'c.bin'])
  })

  it('all missing size preserves original relative order (stable sort)', () => {
    const files = [
      { filename: 'a.bin' },
      { filename: 'b.bin' },
    ]
    const result = sortModelFiles(files)
    expect(result.map(f => f.filename)).toEqual(['a.bin', 'b.bin'])
  })

  it('size=0 explicit value equals missing', () => {
    const files = [
      { filename: 'a.bin', size: 0 },
      { filename: 'b.bin', size: 100 },
    ]
    const result = sortModelFiles(files)
    expect(result.map(f => f.filename)).toEqual(['b.bin', 'a.bin'])
  })
})

describe('guessQuant', () => {
  it('recognize Q4_K_M', () => {
    expect(guessQuant('model-Q4_K_M.gguf')).toBe('Q4_K_M')
  })

  it('recognize iq4_xs (case-insensitive)', () => {
    expect(guessQuant('model-iq4_xs.gguf')).toBe('IQ4_XS')
    expect(guessQuant('model-IQ4_XS.gguf')).toBe('IQ4_XS')
  })

  it('recognize BF16', () => {
    expect(guessQuant('model-bf16.gguf')).toBe('BF16')
  })

  it('no quantization recognized returns empty string', () => {
    expect(guessQuant('model.gguf')).toBe('')
    expect(guessQuant('README.md')).toBe('')
  })
})
