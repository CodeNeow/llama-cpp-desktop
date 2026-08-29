<template>
  <div class="chat-page">
    <div class="sticky-top">
      <div class="page-header">
        <h1 class="page-title">{{ t('chat.title') }}</h1>
        <p class="page-subtitle">{{ t('chat.subtitle') }}</p>
      </div>
      <div class="chat-toolbar">
        <!-- Model picker: themed dropdown (popup list is rendered in-app, so it
             follows the theme — a native select's popup is OS-rendered).
             Options come from the local model scan, so picking works with the
             service stopped; sending then auto-starts it. -->
        <ThemedSelect
          variant="toolbar"
          :model-value="selectedModel"
          :options="modelOptions"
          :disabled="serviceStarting || streaming"
          :placeholder="t('chat.model')"
          :label="t('chat.model')"
          :empty-text="t('chat.noModels')"
          @update:model-value="pickModel"
        />
        <button
          class="chat-clear-btn"
          @click="clearChat"
          :disabled="messages.length === 0 || streaming"
        >
          {{ t('chat.clear') }}
        </button>
        <button
          class="chat-settings-btn"
          @click.stop="showParams = !showParams"
          :aria-expanded="showParams"
          :title="t('chat.settings')"
          type="button"
        >
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
            <circle cx="12" cy="12" r="3"/>
            <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z"/>
          </svg>
        </button>

        <!-- Chat parameters panel -->
        <div v-if="showParams" class="params-popover" @click.stop>
          <div class="params-header">{{ t('chat.settings') }}</div>
          <div class="params-row">
            <label class="params-label" for="chat-temp">{{ t('chat.temperature') }}</label>
            <input id="chat-temp" class="params-input" type="number" step="0.05" min="0" max="2" v-model.number="chatParams.temperature" />
          </div>
          <div class="params-row">
            <label class="params-label" for="chat-topp">{{ t('chat.topP') }}</label>
            <input id="chat-topp" class="params-input" type="number" step="0.05" min="0" max="1" v-model.number="chatParams.topP" />
          </div>
          <div class="params-row">
            <label class="params-label" for="chat-topk">{{ t('chat.topK') }}</label>
            <input id="chat-topk" class="params-input" type="number" step="1" min="0" v-model.number="chatParams.topK" />
          </div>
          <div class="params-row">
            <label class="params-label" for="chat-rep">{{ t('chat.repeatPenalty') }}</label>
            <input id="chat-rep" class="params-input" type="number" step="0.05" min="1" max="2" v-model.number="chatParams.repeatPenalty" />
          </div>
          <div class="params-row">
            <label class="params-label" for="chat-maxtok">{{ t('chat.maxTokens') }}</label>
            <input id="chat-maxtok" class="params-input" type="number" min="-1" v-model.number="chatParams.maxTokens" />
          </div>
          <div class="params-row params-row-full">
            <label class="params-label" for="chat-sys">{{ t('chat.systemPrompt') }}</label>
            <textarea id="chat-sys" class="params-textarea" rows="2" v-model="chatParams.systemPrompt" :placeholder="t('chat.systemPromptPh')"></textarea>
          </div>
          <div class="params-footer">
            <button class="params-reset-btn" @click="resetParams">{{ t('chat.resetDefaults') }}</button>
          </div>
        </div>
      </div>
    </div>

    <!-- Messages area -->
    <!-- Delegated link handler: links in assistant markdown open in the system
         browser, the WebView never navigates (see lib/linkHandler.ts) -->
    <div ref="messagesContainer" class="messages-area" @click="handleLinkClick">
      <div v-if="modelOptions.length === 0" class="empty-hint">{{ t('chat.noModels') }}</div>
      <template v-else>
        <div v-if="messages.length === 0" class="empty-hint">{{ t('chat.emptyHint') }}</div>
        <div
          v-for="(msg, idx) in messages"
          :key="idx"
          class="message-row"
          :class="msg.role === 'user' ? 'is-user' : 'is-assistant'"
        >
          <div class="message-bubble">
            <span class="message-role">{{ msg.role === 'user' ? t('chat.you') : t('chat.assistant') }}</span>
            <!-- Reasoning (thinking) block, assistant messages with thinking output only -->
            <div v-if="msg.reasoning" class="reasoning-block" :class="{ expanded: isReasoningExpanded(idx, msg) }">
              <button class="reasoning-header" type="button" @click="toggleReasoning(idx)">
                <span>{{ t('chat.thinking') }}</span>
                <svg class="reasoning-chevron" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                  <polyline points="6 9 12 15 18 9"/>
                </svg>
              </button>
              <!-- Ref/scroll wiring is bound to the last message only: that is the
                   streaming stick-to-bottom target (see setReasoningBodyRef) -->
              <div
                v-if="isReasoningExpanded(idx, msg)"
                class="reasoning-body"
                :ref="idx === messages.length - 1 ? setReasoningBodyRef : undefined"
                @scroll="onReasoningScroll"
              >{{ msg.reasoning }}</div>
            </div>
            <!-- Images (attached to user messages) -->
            <div v-if="msg.images && msg.images.length" class="message-images">
              <img v-for="(img, i) in msg.images" :key="i" :src="img" class="message-image" alt="" />
            </div>
            <!-- Assistant output renders as markdown (raw HTML escaped by
                 renderMarkdown); user input stays plain text with preserved breaks -->
            <div
              v-if="msg.role === 'assistant'"
              class="message-content markdown-body"
              v-html="renderMarkdown(msg.content)"
            ></div>
            <p v-else class="message-content">{{ msg.content }}</p>
            <span v-if="idx === messages.length - 1 && streaming" class="streaming-cursor" />
            <!-- Per-phase token rates footer, present after streaming ends -->
            <div v-if="statsLine(msg)" class="message-stats">{{ statsLine(msg) }}</div>
          </div>
        </div>
      </template>
    </div>

    <!-- Input area -->
    <div class="input-area">
      <!-- Auto-start / model-switch notice: sits above the input row so status
           feedback lands next to the send action; the error variant adds the
           guided CTA that fixes the reported blocker -->
      <div
        v-if="serviceStarting || switchingModel || startError"
        class="start-notice"
        :class="{ 'start-notice--error': !!startError }"
        role="status"
      >
        <template v-if="startError">
          <span class="start-notice-text">{{ startError }}</span>
          <button v-if="startErrorCause === 'needModels'" class="start-notice-btn" @click="goDownloads">
            {{ t('action.gotoDownloads') }}
          </button>
          <button v-else-if="startErrorCause === 'needRuntime'" class="start-notice-btn" @click="goRuntime">
            {{ t('chat.goRuntime') }}
          </button>
        </template>
        <template v-else-if="serviceStarting">
          <span class="start-notice-spinner" aria-hidden="true"></span>
          <span>{{ t('chat.startingServer') }}</span>
        </template>
        <template v-else>
          <span>{{ t('chat.switchingModel') }}</span>
        </template>
      </div>
      <!-- Pending attachment preview bar -->
      <div v-if="pendingImages.length" class="pending-bar">
        <div class="pending-item" v-for="(img, i) in pendingImages" :key="i">
          <img :src="img" class="pending-thumb" alt="" />
          <button class="pending-remove" @click="removePendingImage(i)" :title="t('chat.removeImage')">✕</button>
        </div>
      </div>
      <div class="input-row">
        <button class="attach-btn" @click="triggerAttach" :title="t('chat.attach')" type="button">
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <path d="M21.44 11.05l-9.19 9.19a6 6 0 0 1-8.49-8.49l9.19-9.19a4 4 0 0 1 5.66 5.66l-9.2 9.19a2 2 0 0 1-2.83-2.83l8.49-8.48"/>
          </svg>
        </button>
        <input
          ref="fileInput"
          type="file"
          accept="image/*"
          multiple
          class="file-input-hidden"
          @change="onFileSelected"
        />
        <textarea
          ref="inputBox"
          class="chat-input"
          rows="1"
          :placeholder="t('chat.inputPlaceholder')"
          :disabled="serviceStarting || streaming || !selectedModel"
          @keydown="onInputKeydown"
          @input="onInputResize"
          @paste="onInputPaste"
        ></textarea>
        <button
          v-if="!streaming"
          class="send-btn"
          :disabled="serviceStarting || !selectedModel"
          @click="send"
        >
          {{ t('chat.send') }}
        </button>
        <button v-else class="send-btn stop-btn" @click="stop">
          {{ t('chat.stop') }}
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, nextTick, watch, onUnmounted, type ComponentPublicInstance } from 'vue'
import { useRouter } from 'vue-router'
import { getServerStatus, getServerConfig, getModels, getLlamaCpp, startServer, unloadModel } from '../wails'
import { chatReadiness, fetchRouterModels, modelsToUnload, streamChatCompletion, tokenRates, type ChatReadiness } from '../lib/chat'
import { messages, selectedModel, streaming, chatAbortController, persistChat, reconcileSelectedModel, chatParams, persistChatParams, type ChatMessage, type ChatParams } from '../lib/chatState'
import { nudgeDock } from '../lib/dockNudge'
import { t } from '../lib/i18n'
import { renderMarkdown } from '../lib/markdown'
import { handleLinkClick } from '../lib/linkHandler'
import { isNearBottom } from '../lib/scroll'
import ThemedSelect, { type SelectOption } from '../components/ThemedSelect.vue'

const router = useRouter()

const serverRunning = ref(false)

/**
 * Chat-initiated server bring-up in progress: disables the input row, shows
 * the starting notice, and blocks re-entrant send() calls.
 */
const serviceStarting = ref(false)

/** Model-swap unload pass in progress: brief, shows the switching notice. */
const switchingModel = ref(false)

/** Inline auto-start error text ('' = none; cleared at the next start attempt). */
const startError = ref('')

/** Guided CTA cause for startError: which page fixes the reported blocker. */
const startErrorCause = ref<'needModels' | 'needRuntime' | ''>('')

/** Local (scanned) models backing the picker; independent of server state. */
const localModels = ref<{ name: string; alias?: string }[]>([])

/** Whether the chat parameters panel is expanded */
const showParams = ref(false)

/**
 * Model options for the picker: value is the llama-server model id —
 * ModelInfo.alias is the display Name sanitized for INI/router use and
 * differs from Name when the name has spaces/special chars or collides
 * (core/gguf.go aliasDedup) — falling back to the name when absent. Label
 * stays the human-readable display name.
 */
const modelOptions = computed<SelectOption[]>(() => localModels.value.map((m) => ({ value: m.alias || m.name, label: m.name })))

const messagesContainer = ref<HTMLDivElement | null>(null)
const inputBox = ref<HTMLTextAreaElement | null>(null)
const fileInput = ref<HTMLInputElement | null>(null)

/** Pending images to send (data URLs), cleared after sending */
const pendingImages = ref<string[]>([])

/** Explicit user toggles of reasoning-block expansion, keyed by message index (component-local; never persisted) */
const reasoningExpanded = ref<Record<number, boolean>>({})

/**
 * Effective reasoning-block expansion: an explicit user toggle wins; otherwise the
 * block auto-expands only while streaming the last assistant message that has
 * reasoning but no answer content yet, and auto-collapses once content starts
 * arriving or streaming ends.
 */
function isReasoningExpanded(idx: number, msg: ChatMessage): boolean {
  if (idx in reasoningExpanded.value) return reasoningExpanded.value[idx]
  return idx === messages.value.length - 1 && streaming.value && !!msg.reasoning && !msg.content
}

/** Toggle the reasoning block, recording an explicit override for this message. */
function toggleReasoning(idx: number) {
  const msg = messages.value[idx]
  if (!msg) return
  reasoningExpanded.value[idx] = !isReasoningExpanded(idx, msg)
}

// ─── Reasoning body stick-to-bottom ─────────────────────────────────────────

/** Reasoning-body element of the last message (stick-to-bottom scroll target); null when collapsed or unmounted */
const reasoningBodyEl = ref<HTMLDivElement | null>(null)

/** Whether the reasoning body is pinned to its bottom; flipped false when the user scrolls up to read earlier thinking */
const reasoningStuck = ref(true)

/**
 * Function ref for the last message's reasoning body: captures the element and
 * resets the stick state when a NEW element appears (a fresh block starts
 * pinned). Vue re-invokes function refs on every patch with the same element,
 * so the identity guard keeps per-delta re-invocations from resetting a
 * user-scrolled-up state. Element-null transitions (collapse, message stops
 * being last, component unmount/navigation) simply clear the capture.
 */
function setReasoningBodyRef(el: Element | ComponentPublicInstance | null): void {
  const dom = el instanceof HTMLDivElement ? el : null
  if (dom === reasoningBodyEl.value) return
  reasoningBodyEl.value = dom
  reasoningStuck.value = true
}

/** Record near-bottom state from user scrolling; ignores bodies other than the captured target. */
function onReasoningScroll(e: Event) {
  const el = reasoningBodyEl.value
  if (!el || e.target !== el) return
  reasoningStuck.value = isNearBottom(el.scrollTop, el.scrollHeight, el.clientHeight)
}

/**
 * Keep the expanded reasoning body pinned to its bottom while reasoning deltas
 * stream in — but only while the user is themselves near the bottom, so a user
 * reading earlier thinking is never yanked around. Runs after nextTick so the
 * DOM (and scrollHeight) reflects the appended delta; the null check follows
 * scrollToBottom's style and guards element absence after unmount/navigation.
 */
function scrollReasoningToBottom() {
  nextTick(() => {
    const el = reasoningBodyEl.value
    if (el && reasoningStuck.value) el.scrollTop = el.scrollHeight
  })
}

/** One-line stats footer, e.g. "思考 45.2 tok/s · 生成 38.6 tok/s"; each part shown only when defined. */
function statsLine(msg: ChatMessage): string {
  if (!msg.stats) return ''
  const parts: string[] = []
  if (msg.stats.reasoningTps !== undefined) parts.push(t('chat.statsThinking', { v: msg.stats.reasoningTps.toFixed(1) }))
  if (msg.stats.answerTps !== undefined) parts.push(t('chat.statsAnswer', { v: msg.stats.answerTps.toFixed(1) }))
  return parts.join(' · ')
}

/**
 * Fetch the local (scanned) model list and reconcile the persisted selection
 * against the available router ids (alias, falling back to the display name —
 * the same strings the picker and the router expose). Runs on mount
 * (independent of server state) and again after a successful auto-start, so a
 * freshly stocked directory is picked up without leaving the page.
 */
async function refreshLocalModels(): Promise<void> {
  try {
    const list = await getModels()
    localModels.value = Array.isArray(list) ? list : []
  } catch {
    // Backend unavailable (standalone vite) or scan failure: keep current state
    return
  }
  const ids = localModels.value.map((m) => m.alias || m.name)
  // A persisted model id may be stale (renamed/removed since the last run); only
  // reconcile against a non-empty list so an empty directory never rewrites
  // the stored choice
  if (ids.length > 0) {
    const reconciled = reconcileSelectedModel(selectedModel.value, ids)
    if (reconciled.changed) {
      selectedModel.value = reconciled.model
      persistChat()
    }
  }
}

/**
 * Chat-initiated server bring-up: pre-check models/runtime so blockers surface
 * as guided errors (no pointless startServer call), then start llama-server
 * and poll the router until it answers /models (up to ~30s). Returns true when
 * the service ended up ready to stream.
 */
async function ensureServerReady(): Promise<boolean> {
  startError.value = ''
  startErrorCause.value = ''
  serviceStarting.value = true
  try {
    let readiness: ChatReadiness = 'ok'
    try {
      const [models, rt] = await Promise.all([getModels(), getLlamaCpp()])
      readiness = chatReadiness(Array.isArray(models) ? models.length : 0, rt?.installed === true)
    } catch {
      // Probe failure must not block the attempt: startServer reports the real error
      readiness = 'ok'
    }
    if (readiness === 'needModels') {
      startError.value = t('chat.noModels')
      startErrorCause.value = 'needModels'
      return false
    }
    if (readiness === 'needRuntime') {
      startError.value = `${t('runtime.llamacpp')} ${t('runtime.llamacpp.notFound')}`
      startErrorCause.value = 'needRuntime'
      return false
    }
    await startServer()
    // Poll router readiness: llama-server binds /models a moment after spawn
    const cfg = await getServerConfig()
    const deadline = Date.now() + 30000
    while (Date.now() < deadline) {
      try {
        await fetchRouterModels(cfg.port)
        // Router answered: the service is ready to stream
        serverRunning.value = true
        await refreshLocalModels()
        nudgeDock()
        return true
      } catch {
        // Not ready yet; retry after the poll interval
        await new Promise((r) => setTimeout(r, 500))
      }
    }
    startError.value = t('api.toggleFailed', { msg: 'timeout' })
    return false
  } catch (e: any) {
    // startServer rejections already carry user-facing localized backend errors
    startError.value = e?.message || String(e)
    return false
  } finally {
    serviceStarting.value = false
  }
}

/**
 * Deterministic single-model memory: before streaming, unload every OTHER
 * loaded model so the selected one becomes the only resident. Best-effort:
 * individual unload failures are ignored and never block the chat request.
 */
async function unloadOtherModels(): Promise<void> {
  let toUnload: string[] = []
  try {
    const cfg = await getServerConfig()
    const loaded = await fetchRouterModels(cfg.port)
    toUnload = modelsToUnload(loaded, selectedModel.value)
  } catch {
    // Router unreachable: let the chat request itself surface the real error
    return
  }
  if (toUnload.length === 0) return
  switchingModel.value = true
  try {
    for (const id of toUnload) {
      try {
        await unloadModel(id)
      } catch {
        // Best-effort: an unload failure must not block streaming
      }
    }
    nudgeDock()
  } finally {
    switchingModel.value = false
  }
}

/** Guided fixes for a failed auto-start: model downloads / runtime section. */
function goDownloads() {
  router.push('/models/download')
}

function goRuntime() {
  router.push('/')
}

/** Close the params panel when clicking outside it */
function onDocClick() {
  showParams.value = false
}

function pickModel(id: string) {
  selectedModel.value = id
  persistChat()
}

/** Clear conversation messages (preserves selected model preference) and persist. */
function clearChat() {
  messages.value = []
  reasoningExpanded.value = {}
  persistChat()
  inputBox.value?.focus()
  resetInputHeight()
}

/** Reset chat params to defaults (in-flight requests unaffected; takes effect on next send). */
function resetParams() {
  Object.assign(chatParams, {
    temperature: 0.8,
    topP: 0.95,
    topK: 40,
    repeatPenalty: 1.1,
    maxTokens: -1,
    systemPrompt: '',
  })
  persistChatParams()
}

/** Input auto-resizes to 1-6 rows: triggered by @input, reset after send/clear. */
function onInputResize() {
  const el = inputBox.value
  if (!el) return
  el.style.height = 'auto'
  const maxPx = 1.5 * parseFloat(getComputedStyle(document.documentElement).fontSize) * 6 + 20
  el.style.height = Math.min(el.scrollHeight, maxPx) + 'px'
}

function resetInputHeight() {
  const el = inputBox.value
  if (el) el.style.height = 'auto'
}

function scrollToBottom() {
  nextTick(() => {
    const el = messagesContainer.value
    if (el) el.scrollTop = el.scrollHeight
  })
}

function appendAssistant(content: string) {
  messages.value.push({ role: 'assistant', content })
}

/** Read file as data URL; only image types; silently ignore failures or non-images */
async function readFileAsDataUrl(file: File): Promise<string | null> {
  if (!file.type.startsWith('image/')) {
    return null
  }
  return new Promise((resolve) => {
    const reader = new FileReader()
    reader.onload = () => resolve(typeof reader.result === 'string' ? reader.result : null)
    reader.onerror = () => resolve(null)
    reader.readAsDataURL(file)
  })
}

function triggerAttach() {
  fileInput.value?.click()
}

async function onFileSelected(e: Event) {
  const input = e.target as HTMLInputElement
  const files = input.files
  if (!files) return
  for (const file of Array.from(files)) {
    const dataUrl = await readFileAsDataUrl(file)
    if (dataUrl) {
      pendingImages.value.push(dataUrl)
    }
  }
  // Reset input so the same file can be selected again
  input.value = ''
}

/** Paste handler: extract image/* files from clipboardData */
async function onInputPaste(e: ClipboardEvent) {
  const items = e.clipboardData?.items
  if (!items) return
  const imageFiles: File[] = []
  for (const item of Array.from(items)) {
    if (item.type.startsWith('image/')) {
      const file = item.getAsFile()
      if (file) imageFiles.push(file)
    }
  }
  if (imageFiles.length === 0) return
  for (const file of imageFiles) {
    const dataUrl = await readFileAsDataUrl(file)
    if (dataUrl) {
      pendingImages.value.push(dataUrl)
    }
  }
}

function removePendingImage(index: number) {
  pendingImages.value.splice(index, 1)
}

async function send() {
  // Re-entrancy guards: never interleave streams or start two bring-ups
  if (streaming.value || serviceStarting.value) return
  const input = inputBox.value
  if (!input) return
  const text = input.value.trim()
  if (!text && pendingImages.value.length === 0) return

  // Service offline: auto-start llama-server and wait for readiness before
  // streaming. Guided failures return here with the input left untouched.
  if (!serverRunning.value) {
    const ready = await ensureServerReady()
    if (!ready) return
  }

  // Without a model id there is nothing to address the request to
  if (!selectedModel.value) return

  // Deterministic model swap: unload every OTHER loaded model first
  await unloadOtherModels()

  const imgs = pendingImages.value.length ? [...pendingImages.value] : undefined
  messages.value.push({ role: 'user', content: text, images: imgs })
  persistChat()
  input.value = ''
  pendingImages.value = []
  resetInputHeight()
  appendAssistant('')
  streaming.value = true
  scrollToBottom()

  chatAbortController.current = new AbortController()
  // Per-stream token counters and phase timestamps for tok/s stats; llama-server
  // streams one token per data chunk, so delta counts are exact token counts
  let reasoningTokens = 0
  let answerTokens = 0
  let firstReasoningAt: number | null = null
  let firstAnswerAt: number | null = null
  let requestFailed = false
  try {
    await streamChatCompletion(
      (await getServerConfig()).port,
      selectedModel.value,
      messages.value.filter(m => m.role !== 'assistant' || m.content).map(m => ({ role: m.role as 'user' | 'assistant', content: m.content, images: m.images })),
      (delta) => {
        if (firstAnswerAt === null) firstAnswerAt = performance.now()
        answerTokens++
        const last = messages.value[messages.value.length - 1]
        if (last && last.role === 'assistant') {
          last.content += delta
          scrollToBottom()
        }
      },
      (reasoning) => {
        if (firstReasoningAt === null) firstReasoningAt = performance.now()
        reasoningTokens++
        const last = messages.value[messages.value.length - 1]
        if (last && last.role === 'assistant') {
          last.reasoning = (last.reasoning || '') + reasoning
          scrollToBottom()
          scrollReasoningToBottom()
        }
      },
      chatAbortController.current.signal,
      { ...chatParams }
    )
  } catch (e: any) {
    // Stop generation (AbortError): keep generated content; if nothing was generated, remove the empty bubble
    if (e?.name === 'AbortError') {
      const last = messages.value[messages.value.length - 1]
      if (last && last.role === 'assistant' && !last.content) {
        messages.value.pop()
      }
      return
    }
    requestFailed = true
    const last = messages.value[messages.value.length - 1]
    if (last && last.role === 'assistant') {
      last.content = t('chat.error', { msg: e?.message || String(e) })
    }
  } finally {
    streaming.value = false
    chatAbortController.current = null
    // Per-phase tok/s: reasoning phase spans first reasoning delta → first content
    // delta (or stream end when there is no answer); answer phase first content
    // delta → stream end. Skipped on error (the bubble then shows an error message).
    const last = messages.value[messages.value.length - 1]
    if (!requestFailed && last && last.role === 'assistant') {
      const endAt = performance.now()
      const rates = tokenRates(
        reasoningTokens,
        firstReasoningAt !== null ? (firstAnswerAt ?? endAt) - firstReasoningAt : 0,
        answerTokens,
        firstAnswerAt !== null ? endAt - firstAnswerAt : 0
      )
      if (rates.reasoningTps !== undefined || rates.answerTps !== undefined) {
        last.stats = rates
      }
    }
    persistChat()
    scrollToBottom()
    // The streamed request just loaded the selected model: refresh the dock
    // now so it appears without waiting for the 1s poll
    nudgeDock()
  }
}

/**
 * Stop generation: triggered manually by the caller; does NOT abort on component unmount—
 * the stream continues writing to module-level state after navigation, so returning
 * to the page shows the complete reply (scrollToBottom's null check on the ref
 * prevents errors after unmount).
 *
 * chatAbortController lifted to module scope ensures the new instance's stop() after
 * navigation remount can still interrupt the in-flight streaming request.
 */
function stop() {
  chatAbortController.current?.abort()
}

function onInputKeydown(e: KeyboardEvent) {
  if (e.key === 'Enter' && !e.shiftKey) {
    e.preventDefault()
    send()
  }
}

watch(selectedModel, () => {
  inputBox.value?.focus()
})

onMounted(async () => {
  try {
    const status = await getServerStatus()
    serverRunning.value = status.running
  } catch {
    // Backend unavailable (standalone vite): keep the offline default
  }
  // Picker options come from the local scan, so they work with the service
  // stopped; reconcile the persisted choice against the names that exist
  await refreshLocalModels()
  document.addEventListener('click', onDocClick)
})

onUnmounted(() => {
  document.removeEventListener('click', onDocClick)
})
</script>

<style scoped>
.chat-page {
  /* No bottom reserve for the task dock: the pill sits in the input row's
     right gap (see padding-right) and shares the send button's vertical band,
     so the page layout simply fills the viewport below the titlebar. */
  height: calc(100vh - 36px);
  display: flex;
  flex-direction: column;
  /* Right 72 = pill band (16 right offset + 48 pill width) + 8px gap: the
     input row (send button) and the messages scrollbar stop left of the pill,
     keeping the pill vertically aligned with the send button. */
  padding: 0 72px 0 48px;
  /* Chat page does not scroll with page: layout fills remaining viewport height, messages area scrolls independently */
}

/* Never compress the fixed header/input bands; the messages area absorbs overflow instead */
.chat-page .sticky-top {
  flex-shrink: 0;
}

.chat-page .page-title {
  font-size: 28px;
  font-weight: 700;
  color: var(--text-primary);
  margin: 0 0 4px;
  letter-spacing: -0.5px;
  line-height: 1.2;
  word-break: break-all;
}

.chat-page .page-subtitle {
  font-size: 14px;
  color: var(--text-dim);
  margin: 0;
}

  /* Align with Downloads and other pages: unified spacing from top title to bottom content */
.chat-page .page-header {
  padding-bottom: 28px;
}

.chat-toolbar {
  display: flex;
  align-items: center;
  gap: 10px;
  padding-bottom: 16px;
  position: relative; /* Anchor for params popover positioning */
}

.chat-clear-btn {
  padding: 8px 14px;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 8px;
  color: var(--text-secondary);
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.2s;
  margin-left: auto;
}

.chat-clear-btn:hover:not(:disabled) {
  background: var(--hover-bg);
  color: var(--text-primary);
}

.chat-clear-btn:disabled {
  opacity: 0.35;
  cursor: default;
}

.chat-settings-btn {
  width: 36px;
  height: 36px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 8px;
  color: var(--text-secondary);
  cursor: pointer;
  transition: all 0.2s;
  flex-shrink: 0;
}

.chat-settings-btn:hover {
  background: var(--hover-bg);
  color: var(--text-primary);
}

.chat-settings-btn[aria-expanded='true'] {
  background: var(--hover-bg);
  color: var(--text-primary);
}

/* ─── Params Popover ─── */
.params-popover {
  position: absolute;
  right: 0;
  top: calc(100% + 8px);
  z-index: 30;
  width: 320px;
  background: var(--bg-secondary);
  border: 1px solid var(--border);
  border-radius: 12px;
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.2);
  padding: 16px;
}

.params-header {
  font-size: 13px;
  font-weight: 600;
  color: var(--text-primary);
  margin-bottom: 10px;
}

.params-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 8px;
}

.params-row-full {
  flex-direction: column;
  align-items: stretch;
}

.params-label {
  font-size: 13px;
  color: var(--text-secondary);
  white-space: nowrap;
}

.params-input {
  width: 110px;
  padding: 6px 8px;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 6px;
  color: var(--text-primary);
  font-size: 13px;
  outline: none;
}

.params-input:focus {
  border-color: rgba(99, 102, 241, 0.4);
}

.params-textarea {
  width: 100%;
  padding: 8px;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 8px;
  color: var(--text-primary);
  font-size: 13px;
  font-family: var(--font-sans);
  line-height: 1.5;
  outline: none;
  resize: vertical;
}

.params-textarea:focus {
  border-color: rgba(99, 102, 241, 0.4);
}

.params-textarea::placeholder {
  color: var(--text-dim);
}

.params-footer {
  display: flex;
  justify-content: flex-end;
  margin-top: 10px;
}

.params-reset-btn {
  padding: 6px 12px;
  background: transparent;
  border: 1px solid var(--border);
  border-radius: 6px;
  color: var(--text-secondary);
  font-size: 12px;
  cursor: pointer;
  transition: all 0.2s;
}

.params-reset-btn:hover {
  background: var(--hover-bg);
  color: var(--text-primary);
}

/* ─── Auto-start / model-switch notice ─── */
.start-notice {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 12px;
  border-radius: 8px;
  font-size: 12px;
  font-weight: 500;
  color: var(--text-secondary);
  background: var(--surface);
  border: 1px solid var(--border);
}

/* Error variant: message + guided CTA sharing the .stop-btn color family
   (rgba overlays stay readable on both light and dark themes) */
.start-notice--error {
  justify-content: space-between;
  color: #f87171;
  background: rgba(239, 68, 68, 0.08);
  border-color: rgba(239, 68, 68, 0.25);
}

.start-notice-text {
  flex: 1;
  min-width: 0;
  word-break: break-word;
}

.start-notice-btn {
  padding: 4px 12px;
  border-radius: 6px;
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s;
  flex-shrink: 0;
  background: rgba(239, 68, 68, 0.12);
  color: #f87171;
  border: 1px solid rgba(239, 68, 68, 0.3);
}

.start-notice-btn:hover {
  background: rgba(239, 68, 68, 0.2);
}

.start-notice-spinner {
  width: 12px;
  height: 12px;
  border-radius: 50%;
  border: 2px solid var(--border);
  border-top-color: #a78bfa;
  animation: notice-spin 0.8s linear infinite;
  flex-shrink: 0;
}

@keyframes notice-spin {
  to { transform: rotate(360deg); }
}

/* ─── Messages ─── */
.messages-area {
  flex: 1;
  /* min-height: 0 lets this flex child shrink below its content size so
     overflow-y scrolling kicks in; with the default min-height: auto the
     growing conversation pushes .input-area out of the viewport instead */
  min-height: 0;
  overflow-y: auto;
  padding: 8px 0 16px;
}

.empty-hint {
  text-align: center;
  padding: 48px 0;
  color: var(--text-dim);
  font-size: 14px;
}

.message-row {
  display: flex;
  margin-bottom: 14px;
}

.message-row.is-user {
  justify-content: flex-end;
}

.message-row.is-assistant {
  justify-content: flex-start;
}

.message-bubble {
  max-width: 78%;
  padding: 10px 14px;
  border-radius: 12px;
  /* No blanket pre-wrap: markdown output manages its own spacing (code must
     not wrap); plain-text spots scope pre-wrap individually */
  font-size: 13px;
  line-height: 1.6;
  word-break: break-word;
}

.is-user .message-bubble {
  background: rgba(99, 102, 241, 0.18);
  color: var(--text-primary);
}

.is-assistant .message-bubble {
  background: var(--surface);
  color: var(--text-primary);
}

.message-role {
  display: block;
  font-size: 11px;
  font-weight: 600;
  color: var(--text-dim);
  margin-bottom: 2px;
}

.message-images {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 8px;
}

.message-image {
  max-width: 100%;
  border-radius: 8px;
  max-height: 280px;
  object-fit: contain;
}

.message-content {
  margin: 0;
}

/* User input stays plain text: preserve explicit line breaks */
.is-user .message-content {
  white-space: pre-wrap;
}

/* ─── Markdown rendering (assistant bubbles) ───
   v-html content does not carry the scope attribute, so child selectors need
   :deep(). Sizes are relative to the 13px bubble text; colors ride the theme
   CSS variables so light/dark both work. */
.markdown-body :deep(h1),
.markdown-body :deep(h2),
.markdown-body :deep(h3),
.markdown-body :deep(h4),
.markdown-body :deep(h5),
.markdown-body :deep(h6) {
  margin: 12px 0 6px;
  line-height: 1.35;
  color: var(--text-primary);
}

.markdown-body :deep(h1) { font-size: 1.35em; }
.markdown-body :deep(h2) { font-size: 1.2em; }
.markdown-body :deep(h3) { font-size: 1.08em; }
.markdown-body :deep(h4),
.markdown-body :deep(h5),
.markdown-body :deep(h6) { font-size: 1em; }

.markdown-body :deep(p) {
  margin: 6px 0;
}

.markdown-body :deep(ul),
.markdown-body :deep(ol) {
  margin: 6px 0;
  padding-left: 1.4em;
}

.markdown-body :deep(li) {
  margin: 2px 0;
}

.markdown-body :deep(li > ul),
.markdown-body :deep(li > ol) {
  margin: 2px 0;
}

/* Inline code: subtle inset chip */
.markdown-body :deep(code) {
  background: var(--overlay-8);
  border: 1px solid var(--border-light);
  border-radius: 4px;
  padding: 0.5px 5px;
  font-size: 0.92em;
  word-break: break-word;
}

/* Fenced code blocks: monospace via the global code/pre rule; long lines
   scroll horizontally instead of wrapping */
.markdown-body :deep(pre) {
  margin: 8px 0;
  padding: 10px 12px;
  background: var(--bg-primary);
  border: 1px solid var(--border);
  border-radius: 8px;
  overflow-x: auto;
}

.markdown-body :deep(pre code) {
  background: transparent;
  border: none;
  padding: 0;
  font-size: 0.95em;
  line-height: 1.5;
  white-space: pre;
  word-break: normal;
}

.markdown-body :deep(blockquote) {
  margin: 8px 0;
  padding: 2px 0 2px 12px;
  border-left: 3px solid var(--accent-glow);
  color: var(--text-secondary);
}

.markdown-body :deep(blockquote p) {
  margin: 4px 0;
}

/* Tables: block + auto scroll so wide tables stay inside the bubble */
.markdown-body :deep(table) {
  display: block;
  margin: 8px 0;
  border-collapse: collapse;
  overflow-x: auto;
  max-width: 100%;
}

.markdown-body :deep(th),
.markdown-body :deep(td) {
  border: 1px solid var(--border);
  padding: 4px 10px;
  text-align: left;
}

.markdown-body :deep(th) {
  background: var(--surface);
  font-weight: 600;
}

.markdown-body :deep(a) {
  color: var(--accent-light);
}

.markdown-body :deep(a:hover) {
  text-decoration: underline;
}

.markdown-body :deep(img) {
  max-width: 100%;
  height: auto;
}

.markdown-body :deep(hr) {
  border: none;
  border-top: 1px solid var(--border);
  margin: 12px 0;
}

/* Avoid doubled spacing where the markdown content meets the bubble padding */
.markdown-body > :deep(:first-child) {
  margin-top: 0;
}

.markdown-body > :deep(:last-child) {
  margin-bottom: 0;
}

/* ─── Reasoning (thinking) block ─── */
.reasoning-block {
  margin-bottom: 8px;
}

.reasoning-header {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  background: transparent;
  border: none;
  padding: 0;
  color: var(--text-muted);
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
  user-select: none;
}

.reasoning-header:hover {
  color: var(--text-secondary);
}

.reasoning-chevron {
  transition: transform 0.2s;
}

.reasoning-block.expanded .reasoning-chevron {
  transform: rotate(180deg);
}

.reasoning-body {
  margin-top: 4px;
  /* Slightly smaller and muted so long thinking stays secondary to the answer */
  font-size: 12.5px;
  line-height: 1.5;
  font-weight: 500;
  color: var(--text-secondary);
  white-space: pre-wrap;
  max-height: 220px;
  overflow-y: auto;
}

/* ─── Token rate stats footer ─── */
.message-stats {
  margin-top: 6px;
  font-size: 11px;
  color: var(--text-dim);
  user-select: none;
}

/* ─── Streaming cursor ─── */
.streaming-cursor {
  display: inline-block;
  width: 6px;
  height: 14px;
  margin-left: 2px;
  vertical-align: text-bottom;
  background: var(--accent-light);
  border-radius: 1px;
  animation: blink 1s steps(2) infinite;
}

@keyframes blink {
  0%, 100% { opacity: 1; }
  50% { opacity: 0; }
}

/* ─── Input ─── */
.input-area {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 12px 0 24px;
  border-top: 1px solid var(--border);
  /* Stays fixed at the bottom: never compressed by the flex column */
  flex-shrink: 0;
}

.input-row {
  display: flex;
  gap: 10px;
}

.pending-bar {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.pending-item {
  position: relative;
  width: 64px;
  height: 64px;
  border-radius: 8px;
  overflow: hidden;
  border: 1px solid var(--border);
}

.pending-thumb {
  width: 100%;
  height: 100%;
  object-fit: cover;
  display: block;
}

.pending-remove {
  position: absolute;
  top: 2px;
  right: 2px;
  width: 18px;
  height: 18px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(0, 0, 0, 0.55);
  color: #fff;
  border: none;
  border-radius: 50%;
  font-size: 10px;
  cursor: pointer;
  line-height: 1;
}

.attach-btn {
  width: 42px;
  height: 42px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 10px;
  color: var(--text-secondary);
  cursor: pointer;
  transition: all 0.2s;
  flex-shrink: 0;
}

.attach-btn:hover {
  background: var(--hover-bg);
  color: var(--text-primary);
}

.file-input-hidden {
  display: none;
}

.chat-input {
  flex: 1;
  resize: none;
  padding: 10px 14px;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 10px;
  color: var(--text-primary);
  font-size: 14px;
  font-weight: 500;
  font-family: var(--font-sans);
  line-height: 1.5;
  outline: none;
  transition: border-color 0.2s;
  /* Auto-resize 1-6 rows */
  min-height: 42px;
  max-height: calc(1.5em * 6 + 20px);
  overflow-y: auto;
}

.chat-input:focus {
  border-color: rgba(99, 102, 241, 0.4);
}

.chat-input::placeholder {
  color: var(--text-dim);
}

.send-btn {
  padding: 0 22px;
  height: 42px;
  background: linear-gradient(135deg, rgba(99, 102, 241, 0.2), rgba(167, 139, 250, 0.15));
  color: #a78bfa;
  border: 1px solid rgba(99, 102, 241, 0.3);
  border-radius: 10px;
  font-size: 14px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s;
  flex-shrink: 0;
}

.send-btn:hover:not(:disabled) {
  background: linear-gradient(135deg, rgba(99, 102, 241, 0.3), rgba(167, 139, 250, 0.25));
}

.send-btn:disabled {
  opacity: 0.35;
  cursor: default;
}

.stop-btn {
  background: rgba(239, 68, 68, 0.12);
  color: #f87171;
  border-color: rgba(239, 68, 68, 0.25);
}

.stop-btn:hover {
  background: rgba(239, 68, 68, 0.2);
}
</style>
