import { describe, it, expect } from 'vitest'
import { parseSSEChunks, buildChatBody, buildMessageContent, type ChatParams } from '../lib/chat'

describe('parseSSEChunks', () => {
  // complete single-line JSON
  it('extract single delta', () => {
    const { deltas, rest } = parseSSEChunks('data: {"choices":[{"delta":{"content":"Hello"}}]}\n')
    expect(deltas).toEqual(['Hello'])
    expect(rest).toBe('')
  })

  // multiple data lines arrive at once
  it('multiple data lines arrive at once', () => {
    const buf =
      'data: {"choices":[{"delta":{"content":"Hel"}}]}\n' +
      'data: {"choices":[{"delta":{"content":"lo"}}]}\n' +
      'data: [DONE]\n'
    const { deltas, rest } = parseSSEChunks(buf)
    expect(deltas).toEqual(['Hel', 'lo'])
    expect(rest).toBe('')
  })

  // ignore data: [DONE]
  it('data: [DONE] ignored', () => {
    const { deltas, rest } = parseSSEChunks('data: [DONE]\n')
    expect(deltas).toEqual([])
    expect(rest).toBe('')
  })

  // skip lines with no delta
  it('skip lines with no delta', () => {
    const { deltas, rest } = parseSSEChunks('data: {"choices":[{"delta":{}}]}\n')
    expect(deltas).toEqual([])
    expect(rest).toBe('')
  })

  // ignore non-data lines
  it('ignore non-data lines', () => {
    const { deltas, rest } = parseSSEChunks('keep-alive\ndata: {"choices":[{"delta":{"content":"x"}}]}\n')
    expect(deltas).toEqual(['x'])
    expect(rest).toBe('')
  })

  // split across chunks: JSON truncated between two chunks
  it('split across chunks: JSON truncated between two chunks', () => {
    // first chunk has no newline, entire buffer becomes rest
    const part1 = 'data: {"choices":[{"delta":{"content":"He'
    const { deltas: d1, rest: r1 } = parseSSEChunks(part1)
    expect(d1).toEqual([])
    expect(r1).toBe('data: {"choices":[{"delta":{"content":"He')

    // second chunk continues and includes a newline
    const part2 = 'llo"}}]}\n'
    const { deltas: d2, rest: r2 } = parseSSEChunks(r1 + part2)
    expect(d2).toEqual(['Hello'])
    expect(r2).toBe('')
  })

  // split across chunks: JSON spans two lines and breaks mid-object
  it('split across chunks: JSON spans two lines and breaks mid-object', () => {
    const part1 = 'data: {"choices":[{"delta":{"content":"Hel'
    const { deltas: d1, rest: r1 } = parseSSEChunks(part1)
    expect(d1).toEqual([])
    expect(r1).toBe('data: {"choices":[{"delta":{"content":"Hel')

    const part2 = 'lo"}}]}\n'
    const { deltas: d2, rest: r2 } = parseSSEChunks(r1 + part2)
    expect(d2).toEqual(['Hello'])
    expect(r2).toBe('')
  })

  // trailing fragment preserved
  it('trailing fragment preserved in rest', () => {
    const buf = 'data: {"choices":[{"delta":{"content":"ok"}}]}\ndata: {"cho'
    const { deltas, rest } = parseSSEChunks(buf)
    expect(deltas).toEqual(['ok'])
    expect(rest).toBe('data: {"cho')
  })

  // empty input
  it('empty input returns empty deltas and empty rest', () => {
    const { deltas, rest } = parseSSEChunks('')
    expect(deltas).toEqual([])
    expect(rest).toBe('')
  })
})

describe('buildChatBody', () => {
  it('without params only contains model/messages/stream, no sampling keys', () => {
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

  it('with params includes temperature/top_p/top_k/repeat_penalty/max_tokens at top level', () => {
    const params: ChatParams = {
      temperature: 0.7,
      topP: 0.9,
      topK: 20,
      repeatPenalty: 1.05,
      maxTokens: 1024,
      systemPrompt: '',
    }
    const body = buildChatBody('m1', [{ role: 'user', content: 'hi' }], params) as Record<string, unknown>
    expect(body).toMatchObject({
      model: 'm1',
      stream: true,
      temperature: 0.7,
      top_p: 0.9,
      top_k: 20,
      repeat_penalty: 1.05,
      max_tokens: 1024,
    })
  })

  it('when systemPrompt is non-empty, prepend system message to messages', () => {
    const params: ChatParams = {
      temperature: 0.8,
      topP: 0.95,
      topK: 40,
      repeatPenalty: 1.1,
      maxTokens: -1,
      systemPrompt: '你是一个有帮助的助手',
    }
    const body = buildChatBody('m1', [{ role: 'user', content: 'hi' }], params) as Record<string, unknown>
    const msgs = body.messages as Array<{ role: string; content: string }>
    expect(msgs[0]).toEqual({ role: 'system', content: '你是一个有帮助的助手' })
    expect(msgs[1]).toEqual({ role: 'user', content: 'hi' })
  })

  it('when systemPrompt is empty, do not inject system message', () => {
    const params: ChatParams = {
      temperature: 0.8,
      topP: 0.95,
      topK: 40,
      repeatPenalty: 1.1,
      maxTokens: -1,
      systemPrompt: '',
    }
    const body = buildChatBody('m1', [{ role: 'user', content: 'hi' }], params) as Record<string, unknown>
    const msgs = body.messages as Array<{ role: string; content: string }>
    expect(msgs.every(m => m.role !== 'system')).toBe(true)
  })
})

describe('buildMessageContent', () => {
  it('without images returns plain text string', () => {
    expect(buildMessageContent('hello')).toBe('hello')
    expect(buildMessageContent('')).toBe('')
  })

  it('with images returns array with text first, images after', () => {
    const result = buildMessageContent('describe this', ['data:image/png;base64,abc', 'data:image/png;base64,def'])
    expect(result).toEqual([
      { type: 'text', text: 'describe this' },
      { type: 'image_url', image_url: { url: 'data:image/png;base64,abc' } },
      { type: 'image_url', image_url: { url: 'data:image/png;base64,def' } },
    ])
  })

  it('when text is empty and images present, omit text part', () => {
    const result = buildMessageContent('', ['data:image/png;base64,abc'])
    expect(result).toEqual([
      { type: 'image_url', image_url: { url: 'data:image/png;base64,abc' } },
    ])
  })
})
