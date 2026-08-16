import { describe, it, expect } from 'vitest'
import { parseSSEChunks, buildChatBody } from '../lib/chat'

describe('parseSSEChunks', () => {
  // 完整单行 JSON
  it('提取单条 delta', () => {
    const { deltas, rest } = parseSSEChunks('data: {"choices":[{"delta":{"content":"Hello"}}]}\n')
    expect(deltas).toEqual(['Hello'])
    expect(rest).toBe('')
  })

  // 多条 data 行一次到达
  it('多条 data 行一次到达', () => {
    const buf =
      'data: {"choices":[{"delta":{"content":"Hel"}}]}\n' +
      'data: {"choices":[{"delta":{"content":"lo"}}]}\n' +
      'data: [DONE]\n'
    const { deltas, rest } = parseSSEChunks(buf)
    expect(deltas).toEqual(['Hel', 'lo'])
    expect(rest).toBe('')
  })

  // data: [DONE] 忽略
  it('data: [DONE] 忽略', () => {
    const { deltas, rest } = parseSSEChunks('data: [DONE]\n')
    expect(deltas).toEqual([])
    expect(rest).toBe('')
  })

  // 无 delta 行跳过
  it('无 delta 的 data 行跳过', () => {
    const { deltas, rest } = parseSSEChunks('data: {"choices":[{"delta":{}}]}\n')
    expect(deltas).toEqual([])
    expect(rest).toBe('')
  })

  // 非 data 行忽略
  it('非 data 行忽略', () => {
    const { deltas, rest } = parseSSEChunks('keep-alive\ndata: {"choices":[{"delta":{"content":"x"}}]}\n')
    expect(deltas).toEqual(['x'])
    expect(rest).toBe('')
  })

  // 跨块拆分：JSON 被截断在两块之间
  it('跨块拆分：JSON 被截断在两块之间', () => {
    // 第一块无换行，整段为 rest
    const part1 = 'data: {"choices":[{"delta":{"content":"He'
    const { deltas: d1, rest: r1 } = parseSSEChunks(part1)
    expect(d1).toEqual([])
    expect(r1).toBe('data: {"choices":[{"delta":{"content":"He')

    // 第二块续上并含换行
    const part2 = 'llo"}}]}\n'
    const { deltas: d2, rest: r2 } = parseSSEChunks(r1 + part2)
    expect(d2).toEqual(['Hello'])
    expect(r2).toBe('')
  })

  // 跨块拆分：JSON 对象跨两行且刚好断在中间
  it('跨块拆分：JSON 对象跨两行且刚好断在中间', () => {
    const part1 = 'data: {"choices":[{"delta":{"content":"Hel'
    const { deltas: d1, rest: r1 } = parseSSEChunks(part1)
    expect(d1).toEqual([])
    expect(r1).toBe('data: {"choices":[{"delta":{"content":"Hel')

    const part2 = 'lo"}}]}\n'
    const { deltas: d2, rest: r2 } = parseSSEChunks(r1 + part2)
    expect(d2).toEqual(['Hello'])
    expect(r2).toBe('')
  })

  // 尾残片保留
  it('尾残片保留到 rest', () => {
    const buf = 'data: {"choices":[{"delta":{"content":"ok"}}]}\ndata: {"cho'
    const { deltas, rest } = parseSSEChunks(buf)
    expect(deltas).toEqual(['ok'])
    expect(rest).toBe('data: {"cho')
  })

  // 空输入
  it('空输入返回空增量与空残片', () => {
    const { deltas, rest } = parseSSEChunks('')
    expect(deltas).toEqual([])
    expect(rest).toBe('')
  })
})

describe('buildChatBody', () => {
  it('返回含 model / messages / stream 的对象', () => {
    const body = buildChatBody('m1', [
      { role: 'user', content: 'hi' },
      { role: 'assistant', content: 'hey' },
    ])
    expect(body).toEqual({
      model: 'm1',
      messages: [
        { role: 'user', content: 'hi' },
        { role: 'assistant', content: 'hey' },
      ],
      stream: true,
    })
  })
})
