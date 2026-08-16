/**
 * 聊天页纯函数与薄 fetch 封装。
 *
 * 约定：
 * - 纯函数（parseSSEChunks、buildChatBody）便于单测，不依赖网络/IO；
 * - fetch 封装直连本地 llama-server，不走 Wails 绑定，因为需要流式读取；
 * - 端口与运行态由调用方（Chat.vue）通过 getServerConfig / getServerStatus 获取。
 */

/** 路由器模型条目（仅取前端关心的字段） */
export interface RouterModel {
  id: string
  status: string
}

/**
 * 增量解析 SSE 响应文本块。
 *
 * 输入为累计收到的原始响应文本（可能含半截 JSON 行），按 `\n` 切出完整行，
 * 仅处理 `data: ` 前缀行：提取 `choices[0].delta.content`，`[DONE]` 忽略；
 * 非 data 行、格式异常行均静默跳过。
 *
 * @returns { deltas: 本次提取到的内容增量数组, rest: 未完成尾行残片 }
 */
export function parseSSEChunks(buffer: string): { deltas: string[]; rest: string } {
  const deltas: string[] = []
  // 找到最后一个换行，之前的部分可完整切行，最后一个换行之后为残片
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
      // 非 JSON data 行静默忽略
    }
  }
  return { deltas, rest }
}

/**
 * 构造聊天补全请求体。
 *
 * messages 的每一项可选带 images 数组（data URL），有图片时走 OpenAI 兼容
 * 多模态格式：content 变为 parts 数组，text 在前、图片在后，对齐 llama.cpp
 * webui 的发送方式。
 */
export function buildChatBody(model: string, messages: { role: string; content: string; images?: string[] }[]): object {
  return {
    model,
    messages: messages.map(m => ({
      role: m.role,
      content: buildMessageContent(m.content, m.images),
    })),
    stream: true,
  }
}

/**
 * 将文本与可选图片构造成 OpenAI 兼容的消息 content。
 *
 * - 无图片：返回纯文本字符串（保持对不支持多模态后端的兼容）。
 * - 有图片：返回 parts 数组，text part 仅在文本非空时包含，图片各一个 image_url part，
 *   顺序为 text 在前、图片在后，对齐 llama.cpp webui 的 image_url 多模态格式。
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
 * 获取路由器当前可用模型列表（排除 failed 状态）。
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
 * 流式聊天补全：POST /v1/chat/completions，逐 token 回调 onDelta。
 *
 * @throws 非 2xx 时读取 body 中的 error.message 抛出。
 */
export async function streamChatCompletion(
  port: number,
  model: string,
  messages: { role: string; content: string }[],
  onDelta: (text: string) => void,
  signal: AbortSignal
): Promise<void> {
  const res = await fetch(`http://127.0.0.1:${port}/v1/chat/completions`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(buildChatBody(model, messages)),
    signal,
  })

  if (!res.ok) {
    let msg = `HTTP ${res.status}`
    try {
      const err = await res.json()
      if (typeof err?.error?.message === 'string') msg = err.error.message
    } catch {
      // 忽略 body 解析失败
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
    // 处理最后残片
    const { deltas } = parseSSEChunks(buffer)
    for (const d of deltas) onDelta(d)
  } finally {
    reader.releaseLock()
  }
}
