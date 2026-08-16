import { describe, it, expect } from 'vitest'
import { isSystemReady } from '../lib/systemReady'

describe('isSystemReady', () => {
  // 两条件都满足时才返回 true
  it('llama.cpp 已安装且至少有一个模型时返回 true', () => {
    expect(isSystemReady(true, 1)).toBe(true)
    expect(isSystemReady(true, 3)).toBe(true)
  })

  // llama.cpp 未安装时返回 false（无论模型数量）
  it('llama.cpp 未安装时返回 false', () => {
    expect(isSystemReady(false, 0)).toBe(false)
    expect(isSystemReady(false, 1)).toBe(false)
    expect(isSystemReady(false, 100)).toBe(false)
  })

  // 模型数量为零时返回 false（无论 llama.cpp 是否安装）
  it('模型数量为零时返回 false', () => {
    expect(isSystemReady(true, 0)).toBe(false)
    expect(isSystemReady(false, 0)).toBe(false)
  })
})
