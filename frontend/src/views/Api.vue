<template>
  <div class="page">
    <div class="page-header">
      <h1 class="page-title">{{ t('api.title') }}</h1>
      <p class="page-subtitle">{{ t('api.subtitle') }}</p>
    </div>

    <!-- 顶部操作栏：状态灯 + 状态文字 + URL + 按钮组 -->
    <section class="status-card toolbar">
      <div class="status-indicator">
        <span class="status-dot" :class="serverRunning ? 'running' : 'stopped'"></span>
        <span class="status-text">{{ serverRunning ? t('api.running') : t('api.stopped') }}</span>
        <span v-if="serverRunning" class="status-url">http://{{ cfg.host }}:{{ cfg.port }}</span>
        <span v-if="serverRunning && status.uptimeSeconds > 0" class="toolbar-uptime">
          <span class="uptime-value">{{ formatUptime(status.uptimeSeconds, locale) }}</span>
          <span class="uptime-label">{{ t('monitor.uptimeLabel') }}</span>
        </span>
      </div>
      <div class="btn-group">
        <button class="server-btn btn-start" :disabled="serverRunning || busy" @click="doStart">
          {{ t('api.startServer') }}
        </button>
        <button class="server-btn btn-stop" :disabled="!serverRunning || busy" @click="doStop">
          {{ t('api.stopServer') }}
        </button>
        <button class="server-btn btn-restart" :disabled="!serverRunning || busy" @click="doRestart">
          {{ t('api.restart') }}
        </button>
      </div>
    </section>

    <!-- 主区两栏：左日志控制台 + 右监控卡片 -->
    <div class="monitor-grid">
      <!-- 左栏：服务日志控制台（深浅主题下均呈深色控制台观感） -->
      <section class="log-panel">
        <div class="panel-header">
          <span class="panel-title">{{ t('api.logTitle') }}</span>
          <button v-if="serverLog.length" class="log-clear-btn" @click="clearLog">{{ t('api.logClear') }}</button>
        </div>
        <div v-if="serverLog.length" class="console-log" ref="logEl">
          <div v-for="(line, i) in serverLog" :key="i" class="console-line">{{ line }}</div>
        </div>
        <div v-else class="console-empty">{{ t('api.logEmpty') }}</div>
      </section>

      <!-- 右栏：三张卡片自上而下 -->
      <div class="monitor-side">
        <!-- a. 系统监控：CPU / 内存 / 磁盘 -->
        <section class="info-section monitor-card">
          <h2 class="section-title">
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <rect x="4" y="4" width="16" height="16" rx="2" ry="2"/><rect x="9" y="9" width="6" height="6"/><line x1="9" y1="1" x2="9" y2="4"/><line x1="15" y1="1" x2="15" y2="4"/><line x1="9" y1="20" x2="9" y2="23"/><line x1="15" y1="20" x2="15" y2="23"/><line x1="20" y1="9" x2="23" y2="9"/><line x1="20" y1="14" x2="23" y2="14"/><line x1="1" y1="9" x2="4" y2="9"/><line x1="1" y1="14" x2="4" y2="14"/>
            </svg>
            {{ t('api.sysMonitor') }}
          </h2>

          <div class="metric-block">
            <div class="metric-head">
              <span class="metric-name">{{ t('monitor.cpu') }}</span>
            </div>
            <div class="usage-bar-wrapper">
              <div class="usage-bar">
                <div class="usage-fill" :style="{ width: cpuPercent + '%' }"></div>
              </div>
              <span class="usage-text">{{ cpuPercent }}%</span>
            </div>
          </div>

          <div class="metric-block">
            <div class="metric-head">
              <span class="metric-name">{{ t('monitor.memory') }}</span>
            </div>
            <div class="usage-bar-wrapper">
              <div class="usage-bar">
                <div class="usage-fill" :style="{ width: memPercent + '%' }"></div>
              </div>
              <span class="usage-text">{{ memPercent }}%</span>
            </div>
            <div class="metric-sub">
              <span>{{ t('monitor.memUsed', { n: memText(status.memUsed) }) }}</span>
              <span class="metric-divider">/</span>
              <span>{{ t('monitor.memTotal', { n: memText(status.memTotal) }) }}</span>
            </div>
          </div>

          <div v-if="status.disk" class="metric-block">
            <div class="metric-head">
              <span class="metric-name">{{ t('monitor.disk', { path: diskPath }) }}</span>
            </div>
            <div class="usage-bar-wrapper">
              <div class="usage-bar">
                <div class="usage-fill" :style="{ width: diskPercent + '%' }"></div>
              </div>
              <span class="usage-text">{{ diskPercent }}%</span>
            </div>
            <div class="metric-sub">
              <span>{{ t('monitor.diskUsed', { n: memText(status.disk.used) }) }}</span>
              <span class="metric-divider">/</span>
              <span>{{ t('monitor.diskTotal', { n: memText(status.disk.total) }) }}</span>
            </div>
          </div>
        </section>

        <!-- b. GPU 监控 -->
        <section class="info-section monitor-card">
          <h2 class="section-title">
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <rect x="2" y="3" width="20" height="14" rx="2" ry="2"/><line x1="8" y1="21" x2="16" y2="21"/><line x1="12" y1="17" x2="12" y2="21"/>
            </svg>
            GPU
          </h2>
          <div v-if="status.gpus.length > 0">
            <div v-for="gpu in status.gpus" :key="gpu.index" class="gpu-row">
              <div class="gpu-head">
                <span class="gpu-name">{{ gpu.name }}</span>
                <span class="gpu-util">{{ gpu.utilPercent }}%</span>
              </div>
              <div class="usage-bar-wrapper">
                <div class="usage-bar">
                  <div class="usage-fill" :style="{ width: gpu.utilPercent + '%' }"></div>
                </div>
                <span class="usage-text">{{ gpu.utilPercent }}%</span>
              </div>
              <div class="gpu-mem">{{ t('monitor.gpuMem', { used: memText(gpu.memUsed), total: memText(gpu.memTotal) }) }}</div>
            </div>
          </div>
          <div v-else class="info-empty">{{ t('monitor.noGpu') }}</div>
        </section>

        <!-- c. Token 速度：提示词处理 + 生成速度数值 + 矮折线图 -->
        <section class="info-section monitor-card">
          <div class="token-card-head">
            <h2 class="section-title">
              <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                <rect x="2" y="2" width="20" height="8" rx="2" ry="2"/><rect x="2" y="14" width="20" height="8" rx="2" ry="2"/><line x1="6" y1="6" x2="6.01" y2="6"/><line x1="6" y1="18" x2="6.01" y2="18"/>
              </svg>
              {{ t('api.tokenSpeed') }}
            </h2>
            <div v-if="status.serverRunning" class="token-uptime">
              <span class="uptime-value">{{ formatUptime(status.uptimeSeconds, locale) }}</span>
              <span class="uptime-label">{{ t('monitor.uptimeLabel') }}</span>
            </div>
          </div>
          <div v-if="!status.serverRunning" class="tps-placeholder">{{ t('monitor.uptimePlaceholder') }}</div>
          <template v-else>
            <div class="tps-cards">
              <div class="tps-card">
                <span class="tps-card-name">{{ t('monitor.promptSpeed') }}</span>
                <span class="tps-card-sub">{{ t('monitor.promptSub') }}</span>
                <div class="tps-card-value">
                  <span class="tps-value">{{ promptTpsText }}</span>
                  <span class="tps-label">tokens/s</span>
                </div>
              </div>
              <div class="tps-card">
                <span class="tps-card-name">{{ t('monitor.decodeSpeed') }}</span>
                <span class="tps-card-sub">{{ t('monitor.decodeSub') }}</span>
                <div class="tps-card-value">
                  <span class="tps-value">{{ decodeTpsText }}</span>
                  <span class="tps-label">tokens/s</span>
                </div>
              </div>
            </div>
            <div class="tps-chart">
              <svg :viewBox="`0 0 ${chartWidth} ${chartHeight}`" preserveAspectRatio="none">
                <line class="tps-axis" x1="0" :y1="chartHeight - 2" :x2="chartWidth" :y2="chartHeight - 2" />
                <polyline :points="decodePoints" />
              </svg>
              <div class="tps-chart-meta">
                <span class="tps-chart-label">{{ t('monitor.chartLabel', { n: decodeHistory.length }) }}</span>
              </div>
            </div>
            <p class="tps-footnote">{{ t('monitor.footnote') }}</p>
          </template>
        </section>
      </div>
    </div>

    <!-- Config -->
    <section class="cfg-section">
      <h2 class="section-title">{{ t('api.cfgTitle') }}</h2>
      <div class="cfg-grid">
        <div class="cfg-item">
          <label>{{ t('api.cfgPort') }}</label>
          <input v-model.number="cfg.port" type="number" min="1024" max="65535" step="1" class="cfg-input cfg-num" :disabled="serverRunning" />
        </div>
        <div class="cfg-item">
          <label>{{ t('api.cfgMaxModels') }}</label>
          <input v-model.number="cfg.maxModels" type="number" min="1" step="1" class="cfg-input cfg-num" :disabled="serverRunning" />
        </div>
        <div class="cfg-item">
          <label>{{ t('api.cfgCacheRam') }}</label>
          <input v-model.number="cfg.cacheRam" type="number" min="0" step="1" class="cfg-input cfg-num" :disabled="serverRunning" placeholder="8192" />
        </div>
      </div>
    </section>

    <!-- Available models -->
    <section class="models-section">
      <h2 class="section-title">{{ t('api.modelsTitle', { n: modelCount }) }}</h2>
      <p class="section-desc">{{ t('api.modelsDescPrefix') }} <code>model</code> {{ t('api.modelsDescSuffix') }}</p>
      <div class="model-tags">
        <span v-for="m in availableModels" :key="m" class="model-tag">{{ m }}</span>
      </div>
      <div v-if="modelCount === 0" class="empty-hint">
        {{ t('api.emptyHint') }}
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, onUnmounted, nextTick, watch } from 'vue'
import { getMonitorStatus, getModels, getServerConfig, getServerStatus, refreshModels, saveServerConfig, startServer, stopServer } from '../wails'
import { appendHistory, chartPoints, formatPromptTps, formatUptime, type MonitorStatus } from '../lib/monitor'
import { formatBytes } from '../lib/format'
import { locale, t } from '../lib/i18n'

const serverRunning = ref(false)
const serverLog = ref<string[]>([])
const logEl = ref<HTMLElement | null>(null)
// 启停/重启执行期间禁用全部按钮，防止连点
const busy = ref(false)

const cfg = reactive({
  host: '127.0.0.1',
  accessMode: 'local', // 服务访问范围来自后端配置（设置页修改），此处仅随保存透传
  port: 8080,
  maxModels: 1,
  cacheRam: 8192,
})

const availableModels = ref<string[]>([])
const modelCount = ref(0)

// ─── 监控（原 Monitor.vue 合并，1s 轮询）：推理指标与系统负载，与上方 getServerStatus 轻量轮询相互独立 ───
const status = ref<MonitorStatus>({
  cpuPercent: 0,
  memUsed: 0,
  memTotal: 0,
  gpus: [],
  serverRunning: false,
  promptTps: 0,
  decodeTps: 0,
  uptimeSeconds: 0,
  disk: null,
})

// 生成速度折线图历史：1s 轮询追加，保留最近 60 个采样（appendHistory 默认 cap=60）
const decodeHistory = ref<number[]>([])
const chartWidth = 560
const chartHeight = 120

let pollTimer: ReturnType<typeof setInterval> | null = null

const cpuPercent = computed(() => Math.round(status.value.cpuPercent))

const memPercent = computed(() => {
  if (status.value.memTotal <= 0) return 0
  return Math.round((status.value.memUsed / status.value.memTotal) * 100)
})

const diskPath = computed(() => status.value.disk?.path ?? '')

const diskPercent = computed(() => {
  const d = status.value.disk
  if (!d || d.total <= 0) return 0
  return Math.round((d.used / d.total) * 100)
})

const promptTpsText = computed(() => formatPromptTps(status.value.promptTps))

const decodeTpsText = computed(() => status.value.decodeTps.toFixed(1))

const decodePoints = computed(() => chartPoints(decodeHistory.value, chartWidth, chartHeight))

// 显存/内存/磁盘可能为 0（如数据未到），formatBytes(0) 返回空串，此处兜底为 "0 B"
function memText(bytes: number): string {
  return formatBytes(bytes) || '0 B'
}

async function fetchMonitorStatus() {
  try {
    const s = await getMonitorStatus()
    status.value = s
    decodeHistory.value = appendHistory(decodeHistory.value, s.decodeTps)
  } catch {
    // 轮询失败静默保持上次数据，不打断监控展示
  }
}

function startPolling() {
  stopPolling()
  pollTimer = setInterval(fetchMonitorStatus, 1000)
  fetchMonitorStatus()
}

function stopPolling() {
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
}

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
      accessMode: cfg.accessMode,
      port: cfg.port,
      maxModels: cfg.maxModels,
      cacheRam: cfg.cacheRam,
    }).catch((e) => {
      serverLog.value.push(t('api.saveFailed', { msg: e instanceof Error ? e.message : String(e) }))
    })
  }, 500)
})

onUnmounted(() => {
  if (saveTimer) clearTimeout(saveTimer)
  stopPolling()
})

onMounted(async () => {
  // 监控轮询：每秒拉取监控状态（并入监控区块，周期与原有 Monitor.vue 一致）
  startPolling()

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

// 按钮三态由 serverRunning + busy 驱动：启动（stopped 可用）、停止/重启（running 可用）；
// 执行期间 busy=true 全禁用防连点。状态统一由 checkServerStatus 延迟刷新真实值，不做乐观翻转（#14）
async function doStart() {
  if (busy.value || serverRunning.value) return
  busy.value = true
  try {
    await refreshModels() // 启动前强制重扫模型，确保预设基于最新模型列表（#18）
    await startServer()
  } catch (e) {
    serverLog.value.push(t('api.toggleFailed', { msg: e instanceof Error ? e.message : String(e) }))
  } finally {
    busy.value = false
    setTimeout(checkServerStatus, 500)
  }
}

async function doStop() {
  if (busy.value || !serverRunning.value) return
  busy.value = true
  try {
    await stopServer()
  } catch (e) {
    serverLog.value.push(t('api.toggleFailed', { msg: e instanceof Error ? e.message : String(e) }))
  } finally {
    busy.value = false
    setTimeout(checkServerStatus, 500)
  }
}

// 重启 = 顺序 stop → start（执行期间 busy 防连点）
async function doRestart() {
  if (busy.value || !serverRunning.value) return
  busy.value = true
  try {
    await stopServer()
    await startServer()
  } catch (e) {
    serverLog.value.push(t('api.toggleFailed', { msg: e instanceof Error ? e.message : String(e) }))
  } finally {
    busy.value = false
    setTimeout(checkServerStatus, 500)
  }
}

// 清空日志：仅清空前端展示数组，后端环形缓冲保留（checkServerStatus 会重新带回后端日志）
function clearLog() {
  serverLog.value = []
}
</script>

<style scoped>
.page {
  /* 无顶部内边距：页头贴内容区顶，标题顶部与侧边栏 logo 图标平齐（见 global.css .page-header） */
  padding: 0 48px 60px;
}

.page-header {
  /* 用 padding 而非 margin：页头背景覆盖该间距，内容滚过时不留缝 */
  padding-bottom: 28px;
}

.page-title {
  font-size: 28px;
  font-weight: 700;
  color: var(--text-primary);
  margin: 0 0 4px;
  line-height: 1.2;
}

.page-subtitle {
  font-size: 14px;
  color: var(--text-dim);
  margin: 0;
}

/* ─── 顶部操作栏 ─── */
.toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 16px 24px;
  margin-bottom: 20px;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 14px;
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

/* 运行时长并入顶部状态栏（原「推理服务」区块取消后信息不丢失） */
.toolbar-uptime {
  display: flex;
  align-items: baseline;
  gap: 6px;
  margin-left: 4px;
}

.btn-group {
  display: flex;
  gap: 10px;
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

/* 禁用态沿用 cfg-input 的弱化惯例 */
.server-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.btn-start { background: #22c55e; }
.btn-start:hover:not(:disabled) { opacity: 0.85; }
.btn-stop { background: #ef4444; }
.btn-stop:hover:not(:disabled) { opacity: 0.85; }
.btn-restart { background: var(--accent); }
.btn-restart:hover:not(:disabled) { opacity: 0.85; }

/* ─── 主区两栏：左日志控制台 + 右监控卡片 ─── */
.monitor-grid {
  display: grid;
  grid-template-columns: 6fr 4fr;
  /* 行高严格等于容器高度：启动日志行数不受限，必须由 minmax(0, 1fr) 与子项 min-height: 0
     锁死容器高度，超出的日志在控制台内部滚动，避免撑开布局顶走下方配置区块 */
  grid-template-rows: minmax(0, 1fr);
  gap: 16px;
  /* 两栏等高；高度随窗口缩放收缩（min(440px, 55vh)） */
  height: min(440px, 55vh);
  margin-bottom: 20px;
}

/* ─── 左栏：日志面板 ─── */
.log-panel {
  height: 100%;
  /* 解除 grid 子项默认 min-height:auto，内容超高时允许收缩，交由内部滚动 */
  min-height: 0;
  display: flex;
  flex-direction: column;
  padding: 18px 20px;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 14px;
}

.panel-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 10px;
}

.panel-title {
  font-size: 15px;
  font-weight: 600;
  color: var(--text-secondary);
}

.log-clear-btn {
  border: none;
  background: transparent;
  color: var(--text-dim);
  font-size: 12px;
  cursor: pointer;
  padding: 2px 6px;
  border-radius: 4px;
  transition: color 0.15s, background 0.15s;
}

.log-clear-btn:hover {
  color: var(--accent-light);
  background: var(--active-bg);
}

/* 深色控制台：深浅主题下均为固定深色底（终端观感），浅色主题下亦然 */
.console-log {
  flex: 1;
  /* 解除 flex 子项默认 min-height:auto，日志行数再多也收缩在面板内滚动 */
  min-height: 0;
  overflow-y: auto;
  background: #0b0b10;
  border-radius: 8px;
  padding: 10px 14px;
  font-family: var(--font-mono);
  font-size: 11px;
}

.console-line {
  color: rgba(255,255,255,0.55);
  padding: 1px 0;
  white-space: pre-wrap;
  word-break: break-all;
}

.console-empty {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  background: #0b0b10;
  border-radius: 8px;
  color: rgba(255,255,255,0.35);
  font-size: 12px;
}

/* ─── 右栏：监控卡片 ─── */
.monitor-side {
  display: flex;
  flex-direction: column;
  gap: 12px;
  height: 100%;
  /* 解除 grid 子项默认 min-height:auto，右栏高度锁定，卡片超高时由自身 overflow-y 滚动 */
  min-height: 0;
  overflow-y: auto;
}

.monitor-card {
  margin-bottom: 0;
  flex-shrink: 0;
  padding: 16px 20px;
}

/* 系统监控内的指标块间距 */
.metric-block {
  margin-bottom: 14px;
}

.metric-block:last-child {
  margin-bottom: 0;
}

/* ─── 配置 ─── */
.cfg-section {
  margin-bottom: 20px;
}

/* 统一 section-title：flex 布局兼容监控区块的图标标题，纯文本标题作为单一 flex 子项渲染外观不变 */
.section-title {
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 15px;
  font-weight: 600;
  color: var(--text-secondary);
  margin: 0 0 14px;
}

.section-title svg {
  color: var(--accent-light);
  flex-shrink: 0;
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

/* ─── 模型 ─── */
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

/* ─── 监控区块（自 Monitor.vue 移植） ─── */

/* ─── Info section card ─── */
.info-section {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 14px;
  transition: border-color 0.2s;
}

.info-section:hover {
  border-color: var(--overlay-10);
}

/* ─── Usage bar ─── */
.usage-bar-wrapper {
  display: flex;
  align-items: center;
  gap: 12px;
}

.usage-bar {
  flex: 1;
  height: 6px;
  background: var(--overlay-8);
  border-radius: 3px;
  overflow: hidden;
}

.usage-fill {
  height: 100%;
  border-radius: 3px;
  background: linear-gradient(90deg, #6366f1, #a78bfa);
  transition: width 0.6s ease;
}

.usage-text {
  font-size: 13px;
  font-weight: 600;
  color: #a78bfa;
  min-width: 36px;
  text-align: right;
}

/* ─── Metric sub ─── */
.metric-sub {
  display: flex;
  gap: 6px;
  margin-top: 10px;
  font-size: 12px;
  color: var(--text-dim);
}

.metric-divider {
  color: var(--overlay-20);
}

/* ─── GPU ─── */
.gpu-row {
  padding: 12px 0;
  border-bottom: 1px solid var(--border-light);
}

.gpu-row:last-child {
  border-bottom: none;
  padding-bottom: 0;
}

.gpu-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 10px;
}

.gpu-name {
  font-size: 13px;
  font-weight: 600;
  color: #a78bfa;
  word-break: break-all;
}

.gpu-util {
  font-size: 13px;
  font-weight: 700;
  color: var(--text-secondary);
  flex-shrink: 0;
  margin-left: 12px;
}

.gpu-mem {
  margin-top: 8px;
  font-size: 12px;
  color: var(--text-dim);
}

/* ─── Token 卡头部（含运行时长小字） ─── */
.token-card-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 14px;
}

.token-card-head .section-title {
  margin: 0;
}

.token-uptime {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  flex-shrink: 0;
}

.uptime-value {
  font-size: 14px;
  font-weight: 600;
  color: var(--text-secondary);
}

.uptime-label,
.tps-label {
  font-size: 11px;
  color: var(--text-dim);
}

.tps-placeholder {
  padding: 24px 0;
  font-size: 13px;
  color: var(--text-dim);
  text-align: center;
}

/* ─── TPS metric cards ─── */
.tps-cards {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
}

.tps-card {
  display: flex;
  flex-direction: column;
  padding: 16px;
  background: var(--surface);
  border: 1px solid var(--border-light);
  border-radius: var(--radius-md);
}

.tps-card-name {
  font-size: 14px;
  font-weight: 600;
  color: var(--text-secondary);
}

.tps-card-sub {
  font-size: 11px;
  color: var(--text-dim);
}

.tps-card-value {
  display: flex;
  align-items: baseline;
  gap: 8px;
  margin-top: 10px;
}

.tps-value {
  font-size: 28px;
  font-weight: 700;
  color: var(--accent-light);
  line-height: 1.1;
}

.tps-footnote {
  margin: 12px 0 0;
  font-size: 12px;
  color: var(--text-dim);
}

/* ─── TPS chart ─── */
.tps-chart {
  margin-top: 16px;
}

.tps-chart svg {
  display: block;
  width: 100%;
  height: 90px;
}

.tps-chart polyline {
  fill: none;
  stroke: var(--accent);
  stroke-width: 2;
  stroke-linecap: round;
  stroke-linejoin: round;
}

.tps-axis {
  stroke: var(--text-muted);
  stroke-width: 1;
  stroke-dasharray: 3 3;
}

.tps-chart-meta {
  display: flex;
  justify-content: flex-end;
  margin-top: 4px;
}

.tps-chart-label {
  font-size: 11px;
  color: var(--text-muted);
}

/* ─── Empty ─── */
.info-empty {
  padding: 16px 0;
  color: var(--text-dim);
  font-size: 13px;
  text-align: center;
}
</style>
