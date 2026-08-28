import { ref, reactive, watch } from 'vue'

/** A single chat message. */
export interface ChatMessage {
  role: 'user' | 'assistant'
  content: string
  /** Image data URLs attached to this message (in-memory only; stripped on persist to keep localStorage small) */
  images?: string[]
  /** Accumulated thinking text (assistant only; small enough to persist alongside content) */
  reasoning?: string
  /** Per-phase token rates of the streamed reply (assistant only; persisted for history) */
  stats?: { reasoningTps?: number; answerTps?: number }
}

/** Chat sampling parameters and system prompt. */
export interface ChatParams {
  temperature: number    // Sampling temperature, default 0.8
  topP: number           // Default 0.95
  topK: number           // Default 40
  repeatPenalty: number  // Default 1.1
  maxTokens: number      // Max tokens to generate; -1 means unlimited (llama-server n_predict -1)
  systemPrompt: string   // System prompt; empty means not injected
}

/** localStorage key for persisted messages (follows store.ts llama-desktop- prefix convention) */
const MESSAGES_KEY = 'llama-desktop-chat-messages'
/** localStorage key for the selected model */
const MODEL_KEY = 'llama-desktop-chat-model'
/** localStorage key for chat parameters */
const CHAT_PARAMS_KEY = 'llama-desktop-chat-params'

/** Max number of conversation messages; the oldest are dropped beyond this cap */
const DEFAULT_CAP = 200

/** Default chat parameters. */
const DEFAULT_CHAT_PARAMS: ChatParams = {
  temperature: 0.8,
  topP: 0.95,
  topK: 40,
  repeatPenalty: 1.1,
  maxTokens: -1,
  systemPrompt: '',
}

/**
 * Keep only the most recent `cap` messages.
 *
 * Module-level state survives page switches; localStorage survives restarts;
 * high-frequency streaming deltas only touch memory and are not flushed per token.
 */
export function capMessages(msgs: ChatMessage[], cap = DEFAULT_CAP): ChatMessage[] {
  if (msgs.length <= cap) return msgs
  return msgs.slice(msgs.length - cap)
}

/**
 * Reconcile the persisted selected model against the server's available models.
 *
 * A stored ID still present is kept as-is; a stale ID (model renamed/removed
 * while the app was closed) falls back to the first available model so send()
 * cannot hit "model not found". An empty available list (server not running or
 * no models loaded) keeps the stored choice untouched. Pure: persistence is
 * the caller's job.
 */
export function reconcileSelectedModel(stored: string, available: string[]): { model: string; changed: boolean } {
  if (available.includes(stored)) {
    return { model: stored, changed: false }
  }
  if (available.length > 0) {
    return { model: available[0], changed: true }
  }
  return { model: stored, changed: false }
}

// ─── Module-level state (survives component unmount) ──────────────────────

/** Current conversation messages. */
export const messages = ref<ChatMessage[]>([])

/** Currently selected model ID. */
export const selectedModel = ref('')

/**
 * Whether a streaming generation is currently in progress.
 *
 * Hoisted to module level so a page remount during streaming still reads true;
 * send()'s `if (streaming.value) return` guard keeps working, preventing
 * concurrent streams whose onDelta closures interleave into the same bubble.
 */
export const streaming = ref(false)

/**
 * Mutable holder for the current streaming AbortController.
 *
 * Hoisted to module level so stop() can abort an in-flight stream after a page
 * remount; a component-local abortController would be lost on unmount and the
 * stop button would stop working.
 */
export const chatAbortController = { current: null as AbortController | null }

/** Chat sampling parameters (reactive; auto-persisted on change via watch) */
export const chatParams = reactive<ChatParams>({ ...DEFAULT_CHAT_PARAMS })

// ─── Persistence: restore on startup ──────────────────────────────────────

/** Restore chat history from localStorage on startup; silently fall back to empty when missing or corrupt. */
export function loadChatHistory(): void {
  try {
    const raw = localStorage.getItem(MESSAGES_KEY)
    if (raw) {
      const parsed = JSON.parse(raw) as ChatMessage[]
      messages.value = capMessages(parsed)
    }
  } catch {
    // Corrupt or non-JSON; silently fall back to empty
    messages.value = []
  }

  try {
    const model = localStorage.getItem(MODEL_KEY)
    if (model !== null) {
      selectedModel.value = model
    }
  } catch {
    // Silent fallback
  }
}

/** Restore chat params from localStorage on startup; silently fall back to defaults when missing or corrupt. */
export function loadChatParams(): void {
  try {
    const raw = localStorage.getItem(CHAT_PARAMS_KEY)
    if (raw) {
      const parsed = JSON.parse(raw) as Partial<ChatParams>
      // Merge over defaults so newly added fields keep a fallback; valid fields override the defaults
      chatParams.temperature = typeof parsed.temperature === 'number' ? parsed.temperature : DEFAULT_CHAT_PARAMS.temperature
      chatParams.topP = typeof parsed.topP === 'number' ? parsed.topP : DEFAULT_CHAT_PARAMS.topP
      chatParams.topK = typeof parsed.topK === 'number' ? parsed.topK : DEFAULT_CHAT_PARAMS.topK
      chatParams.repeatPenalty = typeof parsed.repeatPenalty === 'number' ? parsed.repeatPenalty : DEFAULT_CHAT_PARAMS.repeatPenalty
      chatParams.maxTokens = typeof parsed.maxTokens === 'number' ? parsed.maxTokens : DEFAULT_CHAT_PARAMS.maxTokens
      chatParams.systemPrompt = typeof parsed.systemPrompt === 'string' ? parsed.systemPrompt : DEFAULT_CHAT_PARAMS.systemPrompt
    }
  } catch {
    // Corrupt or non-JSON; silently fall back to defaults
  }
}

// Restore immediately on first module load (same pattern as store.ts readStoredTheme module-level init)
loadChatHistory()
loadChatParams()

// ─── Persistence: flush on demand ─────────────────────────────────────────

/**
 * Write current messages + selectedModel to localStorage.
 *
 * Caller contract (avoids high-frequency I/O):
 * - after the user clicks "send"
 * - when streaming ends or errors (send's finally)
 * - when the conversation is cleared
 *
 * Not called during onDelta streaming; only the last in-memory message is mutated.
 */
export function persistChat(): void {
  try {
    // Strip images on persist (base64 payloads would bloat localStorage); text, reasoning and stats survive restarts
    const toStore = capMessages(messages.value).map(({ images: _images, ...rest }) => rest)
    localStorage.setItem(MESSAGES_KEY, JSON.stringify(toStore))
  } catch {
    // Silently ignore localStorage write failures; chat keeps working
  }
  try {
    if (selectedModel.value) {
      localStorage.setItem(MODEL_KEY, selectedModel.value)
    } else {
      localStorage.removeItem(MODEL_KEY)
    }
  } catch {
    // Silent fallback
  }
}

/**
 * Write current chatParams to localStorage.
 *
 * Persisted automatically via a deep watch: any field change flushes to disk,
 * so nothing is lost across page switches or refreshes. Panel edits during
 * streaming only affect the next send (the in-flight request already locked a
 * buildChatBody snapshot).
 */
export function persistChatParams(): void {
  try {
    localStorage.setItem(CHAT_PARAMS_KEY, JSON.stringify(chatParams))
  } catch {
    // Silent fallback
  }
}

// Auto-persist whenever any chatParams field changes
watch(
  chatParams,
  () => {
    persistChatParams()
  },
  { deep: true }
)
