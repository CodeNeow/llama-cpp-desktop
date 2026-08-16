import { describe, it, expect } from 'vitest'
import { sortModelFiles, guessQuant } from '../lib/modelFiles'

describe('sortModelFiles', () => {
  // 按 size 降序，缺 size 以 0 兜底（排在末尾）
  it('size 降序排列', () => {
    const files = [
      { filename: 'a.bin', size: 100 },
      { filename: 'b.bin', size: 500 },
      { filename: 'c.bin', size: 200 },
    ]
    const result = sortModelFiles(files)
    expect(result.map(f => f.filename)).toEqual(['b.bin', 'c.bin', 'a.bin'])
  })

  it('缺 size 以 0 兜底排末尾', () => {
    const files = [
      { filename: 'a.bin', size: 100 },
      { filename: 'b.bin' },
      { filename: 'c.bin' },
    ]
    const result = sortModelFiles(files)
    expect(result.map(f => f.filename)).toEqual(['a.bin', 'b.bin', 'c.bin'])
  })

  it('全部缺 size 保持原相对顺序（稳定排序）', () => {
    const files = [
      { filename: 'a.bin' },
      { filename: 'b.bin' },
    ]
    const result = sortModelFiles(files)
    expect(result.map(f => f.filename)).toEqual(['a.bin', 'b.bin'])
  })

  it('size=0 显式值等同于缺失', () => {
    const files = [
      { filename: 'a.bin', size: 0 },
      { filename: 'b.bin', size: 100 },
    ]
    const result = sortModelFiles(files)
    expect(result.map(f => f.filename)).toEqual(['b.bin', 'a.bin'])
  })
})

describe('guessQuant', () => {
  it('识别 Q4_K_M', () => {
    expect(guessQuant('model-Q4_K_M.gguf')).toBe('Q4_K_M')
  })

  it('识别 iq4_xs（大小写不敏感）', () => {
    expect(guessQuant('model-iq4_xs.gguf')).toBe('IQ4_XS')
    expect(guessQuant('model-IQ4_XS.gguf')).toBe('IQ4_XS')
  })

  it('识别 BF16', () => {
    expect(guessQuant('model-bf16.gguf')).toBe('BF16')
  })

  it('无量化识别结果返回空串', () => {
    expect(guessQuant('model.gguf')).toBe('')
    expect(guessQuant('README.md')).toBe('')
  })
})
