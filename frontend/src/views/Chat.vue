<template>
  <div class="chat-page">
    <div class="sticky-top">
      <div class="page-header">
        <h1 class="page-title">{{ t('chat.title') }}</h1>
        <p class="page-subtitle">{{ t('chat.subtitle') }}</p>
      </div>
      <div class="chat-toolbar" v-if="serverRunning">
        <select
          class="chat-model-select"
          :value="selectedModel"
          @change="onModelChange"
          :disabled="routerModels.length === 0 || streaming"
        >
          <option value="" disabled>{{ t('chat.model') }}</option>
          <option v-for="m in routerModels" :key="m.id" :value="m.id">{{ m.id }}</option>
        </select>
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

    <!-- Server offline notice -->
    <div v-if="!serverRunning" class="offline-card">
      <div class="offline-icon">
        <svg width="40" height="40" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
          <circle cx="12" cy="12" r="10"/><line x1="12" y1="8" x2="12" y2="12"/><line x1="12" y1="16" x2="12.01" y2="16"/>
        </svg>
      </div>
      <p class="offline-text">{{ t('chat.serverOff') }}</p>
      <button class="offline-btn" @click="goToApi">{{ t('chat.goApi') }}</button>
    </div>

    <!-- Messages area -->
    <div v-else ref="messagesContainer" class="messages-area">
      <div v-if="routerModels.length === 0" class="empty-hint">{{ t('chat.noModels') }}</div>
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
            <!-- Images (attached to user messages) -->
            <div v-if="msg.images && msg.images.length" class="message-images">
              <img v-for="(img, i) in msg.images" :key="i" :src="img" class="message-image" alt="" />
            </div>
            <p class="message-content">{{ msg.content }}</p>
            <span v-if="idx === messages.length - 1 && streaming" class="streaming-cursor" />
          </div>
        </div>
      </template>
    </div>

    <!-- Input area -->
    <div v-if="serverRunning && routerModels.length > 0" class="input-area">
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
          :disabled="!selectedModel || streaming"
          @keydown="onInputKeydown"
          @input="onInputResize"
          @paste="onInputPaste"
        ></textarea>
        <button
          v-if="!streaming"
          class="send-btn"
          :disabled="!selectedModel"
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
import { ref, computed, onMounted, nextTick, watch, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { getServerStatus, getServerConfig } from '../wails'
import { fetchRouterModels, streamChatCompletion, buildChatBody } from '../lib/chat'
import { messages, selectedModel, streaming, chatAbortController, persistChat, chatParams, persistChatParams, type ChatMessage, type ChatParams } from '../lib/chatState'
import { t } from '../lib/i18n'

const router = useRouter()

const serverRunning = ref(false)
const routerModels = ref<{ id: string; status: string }[]>([])

/** Whether the chat parameters panel is expanded */
const showParams = ref(false)

const messagesContainer = ref<HTMLDivElement | null>(null)
const inputBox = ref<HTMLTextAreaElement | null>(null)
const fileInput = ref<HTMLInputElement | null>(null)

/** Pending images to send (data URLs), cleared after sending */
const pendingImages = ref<string[]>([])

function goToApi() {
  router.push('/api')
}

/** Close the params panel when clicking outside it */
function onDocClick() {
  showParams.value = false
}

function onModelChange(e: Event) {
  selectedModel.value = (e.target as HTMLSelectElement).value
  persistChat()
}

/** Clear conversation messages (preserves selected model preference) and persist. */
function clearChat() {
  messages.value = []
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
  const input = inputBox.value
  if (!input || !selectedModel.value) return
  const text = input.value.trim()
  if (text || pendingImages.value.length > 0) {
    const imgs = pendingImages.value.length ? [...pendingImages.value] : undefined
    messages.value.push({ role: 'user', content: text, images: imgs })
  }
  if (!text && pendingImages.value.length === 0) return
  if (streaming.value) return

  persistChat()
  input.value = ''
  pendingImages.value = []
  resetInputHeight()
  appendAssistant('')
  streaming.value = true
  scrollToBottom()

  chatAbortController.current = new AbortController()
  try {
    await streamChatCompletion(
      (await getServerConfig()).port,
      selectedModel.value,
      messages.value.filter(m => m.role !== 'assistant' || m.content).map(m => ({ role: m.role as 'user' | 'assistant', content: m.content, images: m.images })),
      (delta) => {
        const last = messages.value[messages.value.length - 1]
        if (last && last.role === 'assistant') {
          last.content += delta
          scrollToBottom()
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
    const last = messages.value[messages.value.length - 1]
    if (last && last.role === 'assistant') {
      last.content = t('chat.error', { msg: e?.message || String(e) })
    }
  } finally {
    streaming.value = false
    chatAbortController.current = null
    persistChat()
    scrollToBottom()
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
  const status = await getServerStatus()
  serverRunning.value = status.running
  if (status.running) {
    const cfg = await getServerConfig()
    try {
      routerModels.value = await fetchRouterModels(cfg.port)
    } catch {
      routerModels.value = []
    }
  }
  document.addEventListener('click', onDocClick)
})

onUnmounted(() => {
  document.removeEventListener('click', onDocClick)
})
</script>

<style scoped>
.chat-page {
  height: calc(100vh - 36px);
  display: flex;
  flex-direction: column;
  padding: 0 48px;
  /* Chat page does not scroll with page: layout fills remaining viewport height, messages area scrolls independently */
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

.chat-model-select {
  padding: 8px 12px;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 8px;
  color: var(--text-primary);
  font-size: 13px;
  font-weight: 500;
  outline: none;
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

/* ─── Offline ─── */
.offline-card {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 12px;
  color: var(--text-dim);
}

.offline-icon {
  opacity: 0.5;
}

.offline-text {
  font-size: 14px;
}

.offline-btn {
  padding: 8px 18px;
  background: linear-gradient(135deg, rgba(99, 102, 241, 0.2), rgba(167, 139, 250, 0.15));
  color: #a78bfa;
  border: 1px solid rgba(99, 102, 241, 0.3);
  border-radius: 10px;
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s;
}

.offline-btn:hover {
  background: linear-gradient(135deg, rgba(99, 102, 241, 0.3), rgba(167, 139, 250, 0.25));
}

/* ─── Messages ─── */
.messages-area {
  flex: 1;
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
  white-space: pre-wrap;
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
