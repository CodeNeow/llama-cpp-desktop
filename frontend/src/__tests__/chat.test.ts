import { afterEach, describe, expect, it, vi } from 'vitest'
import { fetchRouterModels, parseSSEChunks, buildChatBody, buildMessageContent, tokenRates, chatReadiness, modelsToUnload, directModeNeedsSwitch, type ChatParams } from '../lib/chat'

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
    const { deltas, reasoningDeltas, rest } = parseSSEChunks('')
    expect(deltas).toEqual([])
    expect(reasoningDeltas).toEqual([])
    expect(rest).toBe('')
  })

  // reasoning-only chunk: goes to reasoningDeltas, not deltas
  it('chunk with only reasoning_content captured in reasoningDeltas, not deltas', () => {
    const { deltas, reasoningDeltas, rest } = parseSSEChunks('data: {"choices":[{"delta":{"reasoning_content":"think"}}]}\n')
    expect(deltas).toEqual([])
    expect(reasoningDeltas).toEqual(['think'])
    expect(rest).toBe('')
  })

  // both fields in one chunk: both captured
  it('chunk with both content and reasoning_content captures both', () => {
    const { deltas, reasoningDeltas } = parseSSEChunks('data: {"choices":[{"delta":{"content":"Hi","reasoning_content":"hm"}}]}\n')
    expect(deltas).toEqual(['Hi'])
    expect(reasoningDeltas).toEqual(['hm'])
  })

  // empty-string reasoning_content is ignored like empty content
  it('empty reasoning_content ignored', () => {
    const { deltas, reasoningDeltas } = parseSSEChunks('data: {"choices":[{"delta":{"content":"Hi","reasoning_content":""}}]}\n')
    expect(deltas).toEqual(['Hi'])
    expect(reasoningDeltas).toEqual([])
  })

  // reasoning JSON truncated between two chunks completes on the next round
  it('split across chunks: reasoning JSON truncated then completed', () => {
    const part1 = 'data: {"choices":[{"delta":{"reasoning_content":"thi'
    const { deltas: d1, reasoningDeltas: r1, rest } = parseSSEChunks(part1)
    expect(d1).toEqual([])
    expect(r1).toEqual([])

    const part2 = 'nk"}}]}\n'
    const { deltas: d2, reasoningDeltas: r2 } = parseSSEChunks(rest + part2)
    expect(d2).toEqual([])
    expect(r2).toEqual(['think'])
  })

  // interleaved content and reasoning lines keep their streams separate
  it('interleaved reasoning and content lines captured separately in order', () => {
    const buf =
      'data: {"choices":[{"delta":{"reasoning_content":"a"}}]}\n' +
      'data: {"choices":[{"delta":{"reasoning_content":"b"}}]}\n' +
      'data: {"choices":[{"delta":{"content":"A"}}]}\n'
    const { deltas, reasoningDeltas } = parseSSEChunks(buf)
    expect(deltas).toEqual(['A'])
    expect(reasoningDeltas).toEqual(['a', 'b'])
  })
})

describe('tokenRates', () => {
  // happy path: both phases defined
  it('computes both phase rates', () => {
    expect(tokenRates(10, 1000, 25, 500)).toEqual({ reasoningTps: 10, answerTps: 50 })
  })

  // zero duration makes the rate undefined (not Infinity)
  it('zero duration yields undefined rates', () => {
    expect(tokenRates(10, 0, 25, 0)).toEqual({})
  })

  // zero tokens makes the rate undefined
  it('zero tokens yields undefined rates', () => {
    expect(tokenRates(0, 1000, 0, 500)).toEqual({})
  })

  // single-sided phases
  it('zero reasoning phase omits reasoningTps only', () => {
    expect(tokenRates(0, 0, 25, 500)).toEqual({ answerTps: 50 })
  })

  it('zero answer phase omits answerTps only', () => {
    expect(tokenRates(10, 1000, 0, 0)).toEqual({ reasoningTps: 10 })
  })

  // raw fractional values: rounding to 1 decimal is a display-layer concern
  it('keeps fractional rates raw without rounding', () => {
    expect(tokenRates(1, 3000, 1, 1500)).toEqual({ reasoningTps: 1 / 3, answerTps: 2 / 3 })
  })
})

describe('chatReadiness', () => {
  // runtime missing takes precedence over a missing model directory
  it('runtime missing yields needRuntime regardless of model count', () => {
    expect(chatReadiness(0, false)).toBe('needRuntime')
    expect(chatReadiness(3, false)).toBe('needRuntime')
  })

  // runtime present but directory empty blocks the start with needModels
  it('runtime ok but no models yields needModels', () => {
    expect(chatReadiness(0, true)).toBe('needModels')
  })

  // both prerequisites met
  it('runtime ok and at least one model yields ok', () => {
    expect(chatReadiness(1, true)).toBe('ok')
    expect(chatReadiness(5, true)).toBe('ok')
  })
})

describe('modelsToUnload', () => {
  // exactly the loaded entries other than the selected id
  it('returns loaded model ids excluding the selected one', () => {
    const loaded = [
      { id: 'a', status: 'loaded' },
      { id: 'b', status: 'loading' },
      { id: 'c', status: 'loaded' },
      { id: 'd', status: 'sleeping' },
    ]
    expect(modelsToUnload(loaded, 'a')).toEqual(['c'])
  })

  // the selected model itself is never unloaded even when loaded
  it('never includes the selected model id', () => {
    expect(modelsToUnload([{ id: 'a', status: 'loaded' }], 'a')).toEqual([])
  })

  // status must be exactly 'loaded': loading/sleeping/failed stay untouched
  it('ignores non-loaded statuses (loading / sleeping / failed)', () => {
    const loaded = [
      { id: 'b', status: 'loading' },
      { id: 'd', status: 'sleeping' },
      { id: 'e', status: 'failed' },
    ]
    expect(modelsToUnload(loaded, 'a')).toEqual([])
  })

  // nothing loaded -> nothing to unload
  it('empty loaded list yields empty result', () => {
    expect(modelsToUnload([], 'a')).toEqual([])
  })
})

describe('directModeNeedsSwitch', () => {
  // the direct-mode server answers exactly the model it hosts
  it('is false while the selected model is the resident one', () => {
    expect(directModeNeedsSwitch(['m1'], 'm1')).toBe(false)
    expect(directModeNeedsSwitch(['m1', 'm2'], 'm2')).toBe(false)
  })

  it('is true when the selected model is not resident (old model would answer)', () => {
    expect(directModeNeedsSwitch(['m1'], 'm2')).toBe(true)
  })

  it('is true with an empty resident list (service up but nothing loaded)', () => {
    expect(directModeNeedsSwitch([], 'm1')).toBe(true)
  })

  it('matches full ids exactly (no prefix/substring or case-insensitive matching)', () => {
    expect(directModeNeedsSwitch(['qwen-7b'], 'qwen-7b-instruct')).toBe(true)
    expect(directModeNeedsSwitch(['Qwen3-8B-GGUF'], 'qwen3-8b-gguf')).toBe(true)
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

describe('fetchRouterModels', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  function jsonResponse(status: number, body: unknown): Response {
    return {
      ok: status >= 200 && status < 300,
      status,
      json: () => Promise.resolve(body),
    } as unknown as Response
  }

  it('maps the router /models listing and drops failed entries', async () => {
    const fetchMock = vi.fn((url: string) => {
      expect(url).toBe('http://127.0.0.1:8080/models')
      return Promise.resolve(jsonResponse(200, {
        data: [
          { id: 'm1', status: { value: 'loaded' } },
          { id: 'm2', status: { value: 'failed' } },
        ],
      }))
    })
    vi.stubGlobal('fetch', fetchMock)
    await expect(fetchRouterModels(8080)).resolves.toEqual([
      { id: 'm1', status: 'loaded' },
    ])
  })

  it('on 404 falls back to /v1/models and maps ids to loaded', async () => {
    // Direct-mode servers (Android) have no router /models route: the single
    // resident model is listed through the OpenAI-compatible endpoint, where
    // every served model is by definition loaded.
    const fetchMock = vi.fn((url: string) => {
      if (url === 'http://127.0.0.1:8080/models') {
        return Promise.resolve(jsonResponse(404, {}))
      }
      expect(url).toBe('http://127.0.0.1:8080/v1/models')
      return Promise.resolve(jsonResponse(200, {
        data: [{ id: 'resident-a' }, { id: 'resident-b' }],
      }))
    })
    vi.stubGlobal('fetch', fetchMock)
    await expect(fetchRouterModels(8080)).resolves.toEqual([
      { id: 'resident-a', status: 'loaded' },
      { id: 'resident-b', status: 'loaded' },
    ])
    expect(fetchMock).toHaveBeenCalledTimes(2)
  })

  it('maps the native OpenAI /models shape to loaded without /v1/models fallback', async () => {
    // Newer llama.cpp direct-mode builds answer /models natively with the
    // OpenAI shape (data[].id, object:"model", no per-entry status): the
    // entries are resident by definition, so they map to 'loaded'.
    const fetchMock = vi.fn((url: string) => {
      expect(url).toBe('http://127.0.0.1:8080/models')
      return Promise.resolve(jsonResponse(200, {
        object: 'list',
        data: [{ id: 'stories260K', object: 'model', created: 1750000000, owned_by: 'llama.cpp' }],
      }))
    })
    vi.stubGlobal('fetch', fetchMock)
    await expect(fetchRouterModels(8080)).resolves.toEqual([
      { id: 'stories260K', status: 'loaded' },
    ])
    expect(fetchMock).toHaveBeenCalledTimes(1)
  })

  it('returns an empty list when /models answers 200 with no entries', async () => {
    // Both shapes empty (router mode with nothing loaded) must stay empty —
    // no /v1/models fallback, no synthesized loaded entries.
    vi.stubGlobal('fetch', vi.fn((url: string) => {
      expect(url).toBe('http://127.0.0.1:8080/models')
      return Promise.resolve(jsonResponse(200, { object: 'list', data: [] }))
    }))
    await expect(fetchRouterModels(8080)).resolves.toEqual([])
  })

  it('keeps non-loaded router statuses as-is instead of remapping them to loaded', async () => {
    // Router-shape entries carry real statuses: pass-through (minus failed).
    // Remapping these to 'loaded' would break unload bookkeeping upstream.
    vi.stubGlobal('fetch', vi.fn((url: string) => {
      expect(url).toBe('http://127.0.0.1:8080/models')
      return Promise.resolve(jsonResponse(200, {
        data: [
          { id: 'm1', status: { value: 'unloaded' } },
          { id: 'm2', status: { value: 'failed' } },
        ],
      }))
    }))
    await expect(fetchRouterModels(8080)).resolves.toEqual([
      { id: 'm1', status: 'unloaded' },
    ])
  })

  it('propagates an error when both /models and /v1/models fail', async () => {
    vi.stubGlobal('fetch', vi.fn(() => Promise.resolve(jsonResponse(404, {}))))
    await expect(fetchRouterModels(8080)).rejects.toThrow('GET /v1/models failed: 404')
  })
})
