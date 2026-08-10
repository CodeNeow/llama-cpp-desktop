<template>
  <div class="page">
    <div class="page-header">
      <h1 class="page-title">API</h1>
      <p class="page-subtitle">llama-server 路由器模式</p>
    </div>

    <!-- Server status -->
    <section class="status-card">
      <div class="status-row">
        <div class="status-indicator">
          <span class="status-dot" :class="serverRunning ? 'running' : 'stopped'"></span>
          <span class="status-text">{{ serverRunning ? '运行中' : '已停止' }}</span>
          <span v-if="serverRunning" class="status-url">http://{{ cfg.host }}:{{ cfg.port }}</span>
        </div>
        <button
          class="server-btn"
          :class="serverRunning ? 'btn-stop' : 'btn-start'"
          @click="toggleServer"
        >
          {{ serverRunning ? '停止服务' : '启动服务' }}
        </button>
      </div>

      <!-- Server log -->
      <div v-if="serverLog.length" class="server-log" ref="logEl">
        <div v-for="(line, i) in serverLog" :key="i" class="log-line">{{ line }}</div>
      </div>
    </section>

    <!-- Config -->
    <section class="cfg-section">
      <h2 class="section-title">服务器配置</h2>
      <div class="cfg-grid">
        <div class="cfg-item">
          <label>Host</label>
          <input v-model="cfg.host" class="cfg-input" :disabled="serverRunning" />
        </div>
        <div class="cfg-item">
          <label>Port</label>
          <input v-model.number="cfg.port" type="number" min="1024" max="65535" step="1" class="cfg-input cfg-num" :disabled="serverRunning" />
        </div>
        <div class="cfg-item">
          <label>最大并发模型</label>
          <input v-model.number="cfg.maxModels" type="number" min="1" step="1" class="cfg-input cfg-num" :disabled="serverRunning" />
        </div>
        <div class="cfg-item">
          <label>Prompt 缓存 (MiB)</label>
          <input v-model.number="cfg.cacheRam" type="number" min="0" step="1" class="cfg-input cfg-num" :disabled="serverRunning" placeholder="8192" />
        </div>
      </div>
    </section>

    <!-- Available models -->
    <section class="models-section">
      <h2 class="section-title">可用模型 ({{ modelCount }})</h2>
      <p class="section-desc">通过 API 请求中的 <code>model</code> 字段指定，llama-server 会自动加载/卸载</p>
      <div class="model-tags">
        <span v-for="m in availableModels" :key="m" class="model-tag">{{ m }}</span>
      </div>
      <div v-if="modelCount === 0" class="empty-hint">
        将 .gguf 文件放入模型目录即可自动识别
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, onUnmounted, nextTick, watch } from 'vue'
import { getServerConfig, getModels, getServerStatus, refreshModels, saveServerConfig, startServer, stopServer } from '../wails'

const serverRunning = ref(false)
const serverLog = ref<string[]>([])
const logEl = ref<HTMLElement | null>(null)

const cfg = reactive({
  host: '127.0.0.1',
  port: 8080,
  maxModels: 1,
  cacheRam: 8192,
})

const availableModels = ref<string[]>([])
const modelCount = ref(0)

watch(serverLog, async () => {
  await nextTick()
  if (logEl.value) logEl.value.scrollTop = logEl.value.scrollHeight
})

// 配置实时保存（#13）：修改后 debounce 500ms 静默保存；后端拒绝非法值（如 0.0.0.0）由后端兜底
let configLoaded = false
let saveTimer: ReturnType<typeof setTimeout> | null = null
watch(cfg, () => {
  if (!configLoaded) return
  if (saveTimer) clearTimeout(saveTimer)
  saveTimer = setTimeout(() => {
    saveServerConfig({
      host: cfg.host,
      port: cfg.port,
      maxModels: cfg.maxModels,
      cacheRam: cfg.cacheRam,
    }).catch((e) => {
      serverLog.value.push('配置保存失败: ' + (e instanceof Error ? e.message : String(e)))
    })
  }, 500)
})

onUnmounted(() => { if (saveTimer) clearTimeout(saveTimer) })

onMounted(async () => {
  // Load server config
  try {
    const scfg = await getServerConfig()
    Object.assign(cfg, scfg)
  } catch {}
  // 初始配置加载触发的 watch 回调会在下一次 flush 中执行（此时 configLoaded 仍为 false，
  // 直接跳过保存），await nextTick() 等待该次 flush 完成后才启用自动保存，避免加载即保存。
  await nextTick()
  configLoaded = true

  // Load available models
  try {
    const models = await getModels() as any[]
    if (models) {
      availableModels.value = models.map((m: any) => m.name)
      modelCount.value = models.length
    }
  } catch {}

  // Check server status
  checkServerStatus()
})

async function checkServerStatus() {
  try {
    const status = await getServerStatus()
    serverRunning.value = status.running
    serverLog.value = status.log || []
  } catch {}
}

async function toggleServer() {
  try {
    if (serverRunning.value) {
      await stopServer()
    } else {
      await refreshModels() // 启动前强制重扫模型，确保预设基于最新模型列表（#18）
      await startServer()
    }
  } catch (e) {
    serverLog.value.push('启动/停止服务失败: ' + (e instanceof Error ? e.message : String(e)))
  }
  // 状态统一由 checkServerStatus 延迟刷新真实值，不做乐观翻转（#14）
  setTimeout(checkServerStatus, 500)
}
</script>

<style scoped>
.page {
  padding: 36px 48px 60px;
  max-width: 720px;
}

.page-header {
  margin-bottom: 28px;
}

.page-title {
  font-size: 28px;
  font-weight: 700;
  color: var(--text-primary);
  margin: 0 0 4px;
}

.page-subtitle {
  font-size: 14px;
  color: var(--text-dim);
  margin: 0;
}

/* ─── Status card ─── */
.status-card {
  padding: 20px 24px;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 14px;
  margin-bottom: 20px;
}

.status-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 12px;
}

.status-indicator {
  display: flex;
  align-items: center;
  gap: 10px;
}

.status-dot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  flex-shrink: 0;
}

.status-dot.running {
  background: #22c55e;
  box-shadow: 0 0 8px rgba(34, 197, 94, 0.5);
  animation: pulse 2s infinite;
}

.status-dot.stopped {
  background: rgba(255,255,255,0.2);
}

@keyframes pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.5; }
}

.status-text {
  font-size: 15px;
  font-weight: 600;
  color: var(--text-primary);
}

.status-url {
  font-size: 12px;
  color: var(--accent-light);
  font-family: var(--font-mono);
  background: var(--active-bg);
  padding: 2px 8px;
  border-radius: 4px;
}

.server-btn {
  padding: 10px 24px;
  border: none;
  border-radius: 10px;
  font-size: 14px;
  font-weight: 600;
  cursor: pointer;
  transition: opacity 0.15s;
  color: #fff;
}

.btn-start { background: #22c55e; }
.btn-start:hover { opacity: 0.85; }
.btn-stop { background: #ef4444; }
.btn-stop:hover { opacity: 0.85; }

/* ─── Log ─── */
.server-log {
  max-height: 160px;
  overflow-y: auto;
  background: rgba(0,0,0,0.2);
  border-radius: 8px;
  padding: 10px 14px;
  font-family: var(--font-mono);
  font-size: 11px;
}

.log-line {
  color: rgba(255,255,255,0.5);
  padding: 1px 0;
  white-space: pre-wrap;
  word-break: break-all;
}

/* ─── Config ─── */
.cfg-section {
  margin-bottom: 20px;
}

.section-title {
  font-size: 15px;
  font-weight: 600;
  color: var(--text-secondary);
  margin: 0 0 12px;
}

.section-desc {
  font-size: 12px;
  color: var(--text-dim);
  margin: 0 0 12px;
}

.section-desc code {
  background: var(--active-bg);
  padding: 1px 5px;
  border-radius: 3px;
  font-size: 11px;
  color: var(--accent-light);
}

.cfg-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 10px;
}

.cfg-item {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.cfg-item label {
  font-size: 12px;
  color: var(--text-dim);
  font-weight: 500;
}

.cfg-input {
  padding: 8px 12px;
  background: var(--bg-primary);
  border: 1px solid var(--border);
  border-radius: 8px;
  color: var(--text-primary);
  font-size: 13px;
  outline: none;
  transition: border-color 0.15s;
}

.cfg-input:focus { border-color: var(--accent); }
.cfg-input:disabled { opacity: 0.5; cursor: not-allowed; }

.cfg-num { width: 100%; }

/* ─── Models ─── */
.models-section {
  margin-bottom: 20px;
}

.model-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.model-tag {
  padding: 4px 12px;
  background: var(--active-bg);
  color: var(--accent-light);
  border: 1px solid rgba(99,102,241,0.2);
  border-radius: 20px;
  font-size: 12px;
  font-weight: 500;
}

.empty-hint {
  font-size: 13px;
  color: var(--text-dim);
}
</style>
