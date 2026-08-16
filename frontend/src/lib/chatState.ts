import { ref, reactive, watch } from 'vue'

/** 单条聊天消息结构 */
export interface ChatMessage {
  role: 'user' | 'assistant'
  content: string
  /** 本消息附带图片 data URL 数组（内存中使用，持久化时剥离，避免 localStorage 膨胀） */
  images?: string[]
}

/** 聊天采样参数与系统提示词 */
export interface ChatParams {
  temperature: number    // 采样温度，默认 0.8
  topP: number           // 默认 0.95
  topK: number           // 默认 40
  repeatPenalty: number  // 默认 1.1
  maxTokens: number      // 最大生成 token，-1 表示不限（llama-server n_predict -1）
  systemPrompt: string   // 系统提示词，空=不注入
}

/** localStorage 消息持久化键（对齐 store.ts 的 llama-desktop- 前缀惯例） */
const MESSAGES_KEY = 'llama-desktop-chat-messages'
/** localStorage 选中模型持久化键 */
const MODEL_KEY = 'llama-desktop-chat-model'
/** localStorage 聊天参数持久化键 */
const CHAT_PARAMS_KEY = 'llama-desktop-chat-params'

/** 对话消息数上限：超过时丢弃最旧消息，保持最近 N 条 */
const DEFAULT_CAP = 200

/** 聊天参数默认值 */
const DEFAULT_CHAT_PARAMS: ChatParams = {
  temperature: 0.8,
  topP: 0.95,
  topK: 40,
  repeatPenalty: 1.1,
  maxTokens: -1,
  systemPrompt: '',
}

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

/** 聊天采样参数（响应式，变更后由 watch 自动持久化） */
export const chatParams = reactive<ChatParams>({ ...DEFAULT_CHAT_PARAMS })

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

/** 启动时从 localStorage 恢复聊天参数，损坏/缺失时静默回退默认值。 */
export function loadChatParams(): void {
  try {
    const raw = localStorage.getItem(CHAT_PARAMS_KEY)
    if (raw) {
      const parsed = JSON.parse(raw) as Partial<ChatParams>
      // 合并默认值，保证新增字段也有兜底；合法字段直接覆盖默认值
      chatParams.temperature = typeof parsed.temperature === 'number' ? parsed.temperature : DEFAULT_CHAT_PARAMS.temperature
      chatParams.topP = typeof parsed.topP === 'number' ? parsed.topP : DEFAULT_CHAT_PARAMS.topP
      chatParams.topK = typeof parsed.topK === 'number' ? parsed.topK : DEFAULT_CHAT_PARAMS.topK
      chatParams.repeatPenalty = typeof parsed.repeatPenalty === 'number' ? parsed.repeatPenalty : DEFAULT_CHAT_PARAMS.repeatPenalty
      chatParams.maxTokens = typeof parsed.maxTokens === 'number' ? parsed.maxTokens : DEFAULT_CHAT_PARAMS.maxTokens
      chatParams.systemPrompt = typeof parsed.systemPrompt === 'string' ? parsed.systemPrompt : DEFAULT_CHAT_PARAMS.systemPrompt
    }
  } catch {
    // 损坏或非 JSON，静默回退默认
  }
}

// 模块首次加载即恢复（与 store.ts 的 readStoredTheme 等模块级初始化一致）
loadChatHistory()
loadChatParams()

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

/**
 * 将当前 chatParams 写入 localStorage。
 *
 * 采用 watch 深度监听自动持久化：任何字段变化后落盘，保证切页/刷新不丢失。
 * 流式生成期间面板可改，但只影响下次发送（当前请求已锁定 buildChatBody 的快照）。
 */
export function persistChatParams(): void {
  try {
    localStorage.setItem(CHAT_PARAMS_KEY, JSON.stringify(chatParams))
  } catch {
    // 静默处理
  }
}

// chatParams 任一字段变化后自动持久化
watch(
  chatParams,
  () => {
    persistChatParams()
  },
  { deep: true }
)
