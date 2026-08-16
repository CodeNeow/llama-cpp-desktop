/**
 * Chat page pure functions and a thin fetch wrapper.
 *
 * Conventions:
 * - Pure functions (parseSSEChunks, buildChatBody) are unit-test friendly, no network/IO;
 * - The fetch wrapper talks to the local llama-server directly instead of going through
 *   Wails bindings, because streaming reads are required;
 * - Port and running state are obtained by the caller (Chat.vue) via getServerConfig / getServerStatus.
 */

/** Router model entry (only the fields the frontend cares about) */
export interface RouterModel {
  id: string
  status: string
}

/** Chat sampling parameters (kept in sync with chatState.ChatParams to avoid a hard dependency of this lib on chatState) */
export interface ChatParams {
  temperature: number
  topP: number
  topK: number
  repeatPenalty: number
  maxTokens: number
  systemPrompt: string
}

/**
 * Incrementally parse a chunk of SSE response text.
 *
 * Input is the raw response text accumulated so far (may contain a half-finished
 * JSON line); split on `\n` to get complete lines and only process lines with the
 * `data: ` prefix: extract `choices[0].delta.content`, ignore `[DONE]`;
 * non-data lines and malformed lines are silently skipped.
 *
 * @returns { deltas: content deltas extracted this round, rest: unfinished trailing line fragment }
 */
export function parseSSEChunks(buffer: string): { deltas: string[]; rest: string } {
  const deltas: string[] = []
  // Find the last newline: everything before it can be split into complete lines; what follows is the fragment
  const lastNewline = buffer.lastIndexOf('\n')
  const processPart = lastNewline >= 0 ? buffer.slice(0, lastNewline) : ''
  const rest = lastNewline >= 0 ? buffer.slice(lastNewline + 1) : buffer

  const lines = processPart.split('\n')
  for (const rawLine of lines) {
    const line = rawLine.trim()
    if (!line.startsWith('data: ')) continue
    const payload = line.slice(6).trim()
    if (payload === '[DONE]') continue
    if (!payload) continue
    try {
      const json = JSON.parse(payload)
      const content = json?.choices?.[0]?.delta?.content
      if (typeof content === 'string' && content) {
        deltas.push(content)
      }
    } catch {
      // Silently ignore non-JSON data lines
    }
  }
  return { deltas, rest }
}

/**
 * Build the chat completion request body.
 *
 * Each messages entry may carry an optional images array (data URLs); with images
 * present it uses the OpenAI-compatible multimodal format: content becomes a parts
 * array with text first and images after, matching how the llama.cpp webui sends.
 *
 * When params is provided, sampling parameters are injected at the top level of
 * the body; a non-empty systemPrompt is inserted as the first system message.
 */
export function buildChatBody(
  model: string,
  messages: { role: string; content: string; images?: string[] }[],
  params?: ChatParams
): object {
  const body: Record<string, unknown> = {
    model,
    messages: messages.map(m => ({
      role: m.role,
      content: buildMessageContent(m.content, m.images),
    })),
    stream: true,
  }

  if (params) {
    body.temperature = params.temperature
    body.top_p = params.topP
    body.top_k = params.topK
    body.repeat_penalty = params.repeatPenalty
    body.max_tokens = params.maxTokens
    if (params.systemPrompt) {
      ;(body.messages as Array<{ role: string; content: string }>).unshift({
        role: 'system',
        content: params.systemPrompt,
      })
    }
  }

  return body
}

/**
 * Build an OpenAI-compatible message content from text and optional images.
 *
 * - No images: return the plain text string (stays compatible with non-multimodal backends).
 * - With images: return a parts array; the text part is included only when the text is
 *   non-empty, and each image becomes one image_url part, text first and images after,
 *   matching the llama.cpp webui image_url multimodal format.
 */
export function buildMessageContent(text: string, images?: string[]): string | Array<{ type: string; text?: string; image_url?: { url: string } }> {
  if (!images || images.length === 0) {
    return text
  }
  const parts: Array<{ type: string; text?: string; image_url?: { url: string } }> = []
  if (text) {
    parts.push({ type: 'text', text })
  }
  for (const url of images) {
    parts.push({ type: 'image_url', image_url: { url } })
  }
  return parts
}

/**
 * Fetch the router's currently available model list (excluding failed entries).
 */
export async function fetchRouterModels(port: number): Promise<RouterModel[]> {
  const res = await fetch(`http://127.0.0.1:${port}/models`)
  if (!res.ok) {
    const err = await res.json().catch(() => ({}))
    throw new Error(err?.error?.message || `GET /models failed: ${res.status}`)
  }
  const json = await res.json()
  const list = Array.isArray(json?.data) ? json.data : []
  return list
    .filter((m: any) => m?.status?.value !== 'failed')
    .map((m: any) => ({ id: m.id, status: m.status?.value ?? 'unknown' }))
}

/**
 * Streaming chat completion: POST /v1/chat/completions, invoking onDelta per token.
 *
 * @throws On non-2xx, reads error.message from the body and throws it.
 */
export async function streamChatCompletion(
  port: number,
  model: string,
  messages: { role: string; content: string }[],
  onDelta: (text: string) => void,
  signal: AbortSignal,
  params?: ChatParams
): Promise<void> {
  const res = await fetch(`http://127.0.0.1:${port}/v1/chat/completions`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(buildChatBody(model, messages, params)),
    signal,
  })

  if (!res.ok) {
    let msg = `HTTP ${res.status}`
    try {
      const err = await res.json()
      if (typeof err?.error?.message === 'string') msg = err.error.message
    } catch {
      // Ignore body parse failures
    }
    throw new Error(msg)
  }

  const reader = res.body?.getReader()
  if (!reader) throw new Error('Response body is not readable')

  const decoder = new TextDecoder()
  let buffer = ''
  try {
    while (true) {
      const { done, value } = await reader.read()
      if (done) break
      buffer += decoder.decode(value, { stream: true })
      const { deltas, rest } = parseSSEChunks(buffer)
      buffer = rest
      for (const d of deltas) onDelta(d)
    }
    // Process the final fragment
    const { deltas } = parseSSEChunks(buffer)
    for (const d of deltas) onDelta(d)
  } finally {
    reader.releaseLock()
  }
}
