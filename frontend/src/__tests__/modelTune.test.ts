import { describe, it, expect } from 'vitest'
import { tunedSummaryParams } from '../lib/modelTune'

describe('tunedSummaryParams', () => {
  // Table: cacheTypeK is the only field with a fallback (empty = backend
  // default f16); the other fields pass through unchanged
  const cases: Array<{
    name: string
    cfg: { gpuLayers: string; ctxSize: number; cacheTypeK: string; threads: number }
    expected: { gpu: string; ctx: number; cache: string; threads: number }
  }> = [
    {
      name: 'quantized cache passes through (q8_0)',
      cfg: { gpuLayers: 'all', ctxSize: 32768, cacheTypeK: 'q8_0', threads: 8 },
      expected: { gpu: 'all', ctx: 32768, cache: 'q8_0', threads: 8 },
    },
    {
      name: 'empty cacheTypeK falls back to f16',
      cfg: { gpuLayers: 'auto', ctxSize: 4096, cacheTypeK: '', threads: 4 },
      expected: { gpu: 'auto', ctx: 4096, cache: 'f16', threads: 4 },
    },
  ]

  for (const c of cases) {
    it(c.name, () => {
      expect(tunedSummaryParams(c.cfg)).toEqual(c.expected)
    })
  }

  it('gpuLayers / ctxSize / threads pass through unchanged', () => {
    const out = tunedSummaryParams({ gpuLayers: '20', ctxSize: 8192, cacheTypeK: 'q4_0', threads: 16 })
    expect(out.gpu).toBe('20')
    expect(out.ctx).toBe(8192)
    expect(out.threads).toBe(16)
  })
})
