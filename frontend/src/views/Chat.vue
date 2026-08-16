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
      </div>
    </div>

    <!-- 服务未运行提示 -->
    <div v-if="!serverRunning" class="offline-card">
      <div class="offline-icon">
        <svg width="40" height="40" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
          <circle cx="12" cy="12" r="10"/><line x1="12" y1="8" x2="12" y2="12"/><line x1="12" y1="16" x2="12.01" y2="16"/>
        </svg>
      </div>
      <p class="offline-text">{{ t('chat.serverOff') }}</p>
      <button class="offline-btn" @click="goToApi">{{ t('chat.goApi') }}</button>
    </div>

    <!-- 消息区 -->
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
            <p class="message-content">{{ msg.content }}</p>
            <span v-if="idx === messages.length - 1 && streaming" class="streaming-cursor" />
          </div>
        </div>
      </template>
    </div>

    <!-- 输入区 -->
    <div v-if="serverRunning && routerModels.length > 0" class="input-area">
      <textarea
        ref="inputBox"
        class="chat-input"
        rows="1"
        :placeholder="t('chat.inputPlaceholder')"
        :disabled="!selectedModel || streaming"
        @keydown="onInputKeydown"
        @input="onInputResize"
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
</template>

<script setup lang="ts">
import { ref, computed, onMounted, nextTick, watch } from 'vue'
import { useRouter } from 'vue-router'
import { getServerStatus, getServerConfig } from '../wails'
import { fetchRouterModels, streamChatCompletion, buildChatBody } from '../lib/chat'
import { t } from '../lib/i18n'

const router = useRouter()

const serverRunning = ref(false)
const routerModels = ref<{ id: string; status: string }[]>([])
const selectedModel = ref('')
const messages = ref<{ role: string; content: string }[]>([])
const streaming = ref(false)
let abortController: AbortController | null = null

const messagesContainer = ref<HTMLDivElement | null>(null)
const inputBox = ref<HTMLTextAreaElement | null>(null)

function goToApi() {
  router.push('/api')
}

function onModelChange(e: Event) {
  selectedModel.value = (e.target as HTMLSelectElement).value
}

function clearChat() {
  messages.value = []
  inputBox.value?.focus()
  resetInputHeight()
}

/** 输入框自适应 1-6 行高度：由 @input 触发，发送/清空后重置。 */
function onInputResize() {
  const el = inputBox.value
  if (!el) return
  // 先归零让 scrollHeight 反映真实内容高度
  el.style.height = 'auto'
  // max-height 对应 6 行：line-height 1.5em × 6 + padding 上下各 10px = 20px
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

async function send() {
  const input = inputBox.value
  if (!input || !selectedModel.value) return
  const text = input.value.trim()
  if (!text || streaming.value) return

  messages.value.push({ role: 'user', content: text })
  input.value = ''
  resetInputHeight()
  appendAssistant('')
  streaming.value = true
  scrollToBottom()

  abortController = new AbortController()
  try {
    await streamChatCompletion(
      (await getServerConfig()).port,
      selectedModel.value,
      messages.value.filter(m => m.role !== 'assistant' || m.content).map(m => ({ role: m.role as 'user' | 'assistant', content: m.content })),
      (delta) => {
        const last = messages.value[messages.value.length - 1]
        if (last && last.role === 'assistant') {
          last.content += delta
          scrollToBottom()
        }
      },
      abortController.signal
    )
  } catch (e: any) {
    // 停止生成（AbortError）：保留已生成内容；若一停止就没生成任何内容，移除空气泡
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
    abortController = null
    scrollToBottom()
  }
}

function stop() {
  abortController?.abort()
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
})
</script>

<style scoped>
.chat-page {
  height: calc(100vh - 36px);
  display: flex;
  flex-direction: column;
  padding: 0 48px;
  /* 聊天页不随页面滚动：布局占满视口剩余高度，内部消息区独立滚动 */
}

.chat-toolbar {
  display: flex;
  align-items: center;
  gap: 10px;
  padding-bottom: 16px;
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
}

.chat-clear-btn:hover:not(:disabled) {
  background: var(--hover-bg);
  color: var(--text-primary);
}

.chat-clear-btn:disabled {
  opacity: 0.35;
  cursor: default;
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
  gap: 10px;
  padding: 12px 0 24px;
  border-top: 1px solid var(--border);
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
  /* 自适应 1-6 行 */
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
