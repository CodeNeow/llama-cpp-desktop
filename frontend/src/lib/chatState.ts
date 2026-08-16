import { ref } from 'vue'

/** 单条聊天消息结构 */
export interface ChatMessage {
  role: 'user' | 'assistant'
  content: string
  /** 本消息附带图片 data URL 数组（内存中使用，持久化时剥离，避免 localStorage 膨胀） */
  images?: string[]
}

/** localStorage 消息持久化键（对齐 store.ts 的 llama-desktop- 前缀惯例） */
const MESSAGES_KEY = 'llama-desktop-chat-messages'
/** localStorage 选中模型持久化键 */
const MODEL_KEY = 'llama-desktop-chat-model'

/** 对话消息数上限：超过时丢弃最旧消息，保持最近 N 条 */
const DEFAULT_CAP = 200

/**
 * 保留最近 cap 条消息。
 *
 * 模块级状态保证切页不丢、localStorage 保证重启不丢；
 * 流式 delta 高频写入仅改内存，不逐 token 落盘。
 */
export function capMessages(msgs: ChatMessage[], cap = DEFAULT_CAP): ChatMessage[] {
  if (msgs.length <= cap) return msgs
  return msgs.slice(msgs.length - cap)
}

// ─── 模块级状态（组件卸载不销毁） ───────────────────────────────────────────

/** 当前对话消息列表 */
export const messages = ref<ChatMessage[]>([])

/** 当前选中模型 ID */
export const selectedModel = ref('')

/**
 * 当前是否正在流式生成。
 *
 * 上移到模块级，保证流式期间切页重挂后新实例仍能读到 true，
 * send() 的 `if (streaming.value) return` 防线持续生效，避免并发流式
 * 导致多个 onDelta 闭包交错写入同一条气泡。
 */
export const streaming = ref(false)

/**
 * 当前流式 AbortController 的可变持有器。
 *
 * 上移到模块级，保证切页重挂后 stop() 仍能中断正在进行的流式请求；
 * 局部 abortController 在组件卸载后丢失会导致停止按钮失效。
 */
export const chatAbortController = { current: null as AbortController | null }

// ─── 持久化：启动恢复 ───────────────────────────────────────────────────────

/** 启动时从 localStorage 恢复对话历史，损坏/缺失时静默回退为空。 */
export function loadChatHistory(): void {
  try {
    const raw = localStorage.getItem(MESSAGES_KEY)
    if (raw) {
      const parsed = JSON.parse(raw) as ChatMessage[]
      messages.value = capMessages(parsed)
    }
  } catch {
    // 损坏或非 JSON，静默回退空
    messages.value = []
  }

  try {
    const model = localStorage.getItem(MODEL_KEY)
    if (model !== null) {
      selectedModel.value = model
    }
  } catch {
    // 静默回退
  }
}

// 模块首次加载即恢复（与 store.ts 的 readStoredTheme 等模块级初始化一致）
loadChatHistory()

// ─── 持久化：按需落盘 ───────────────────────────────────────────────────────

/**
 * 将当前 messages + selectedModel 写入 localStorage。
 *
 * 调用方约定（避免高频 I/O）：
 * - 用户点击「发送」后
 * - 流式结束或出错（send 的 finally）
 * - 清空对话时
 *
 * 流式 onDelta 期间不调用本函数，仅修改内存中最后一条消息。
 */
export function persistChat(): void {
  try {
    // 持久化时剥离 images（base64 体积大，防 localStorage 膨胀）；重启后图片不保留，仅保留文本
    const toStore = capMessages(messages.value).map(({ images: _images, ...rest }) => rest)
    localStorage.setItem(MESSAGES_KEY, JSON.stringify(toStore))
  } catch {
    // localStorage 写入失败静默处理，不影响聊天功能
  }
  try {
    if (selectedModel.value) {
      localStorage.setItem(MODEL_KEY, selectedModel.value)
    } else {
      localStorage.removeItem(MODEL_KEY)
    }
  } catch {
    // 静默处理
  }
}
