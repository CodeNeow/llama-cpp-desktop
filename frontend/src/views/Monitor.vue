<template>
  <div class="page">
    <div class="page-header">
      <h1 class="page-title">监控</h1>
      <p class="page-subtitle">推理服务与系统资源实时状态（每 1 秒刷新）</p>
    </div>

    <!-- 推理服务：提示词处理 + 生成速度 -->
    <section class="info-section">
      <h2 class="section-title">
        <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <rect x="2" y="2" width="20" height="8" rx="2" ry="2"/><rect x="2" y="14" width="20" height="8" rx="2" ry="2"/><line x1="6" y1="6" x2="6.01" y2="6"/><line x1="6" y1="18" x2="6.01" y2="18"/>
        </svg>
        推理服务
      </h2>
      <div class="server-head">
        <span class="status-badge" :class="status.serverRunning ? 'available' : 'unavailable'">
          {{ status.serverRunning ? '运行中' : '未启动' }}
        </span>
        <div v-if="status.serverRunning" class="uptime-block">
          <span class="uptime-value">{{ formatUptime(status.uptimeSeconds) }}</span>
          <span class="uptime-label">运行时长</span>
        </div>
      </div>
      <div v-if="!status.serverRunning" class="tps-placeholder">启动服务后显示</div>
      <template v-else>
        <div class="tps-cards">
          <div class="tps-card">
            <span class="tps-card-name">提示词处理速度</span>
            <span class="tps-card-sub">提示词预填充（prefill）</span>
            <div class="tps-card-value">
              <span class="tps-value">{{ promptTpsText }}</span>
              <span class="tps-label">tokens/s</span>
            </div>
          </div>
          <div class="tps-card">
            <span class="tps-card-name">生成速度</span>
            <span class="tps-card-sub">实时解码（decode）</span>
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
            <span class="tps-chart-label">近 {{ decodeHistory.length }} 秒生成速度</span>
          </div>
        </div>
        <p class="tps-footnote">提示词处理速度在请求结束后更新 · 思考与回答同属解码过程</p>
      </template>
    </section>

    <!-- CPU -->
    <section class="info-section">
      <h2 class="section-title">
        <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <rect x="4" y="4" width="16" height="16" rx="2" ry="2"/><rect x="9" y="9" width="6" height="6"/><line x1="9" y1="1" x2="9" y2="4"/><line x1="15" y1="1" x2="15" y2="4"/><line x1="9" y1="20" x2="9" y2="23"/><line x1="15" y1="20" x2="15" y2="23"/><line x1="20" y1="9" x2="23" y2="9"/><line x1="20" y1="14" x2="23" y2="14"/><line x1="1" y1="9" x2="4" y2="9"/><line x1="1" y1="14" x2="4" y2="14"/>
        </svg>
        处理器
      </h2>
      <div class="usage-bar-wrapper">
        <div class="usage-bar">
          <div class="usage-fill" :style="{ width: cpuPercent + '%' }"></div>
        </div>
        <span class="usage-text">{{ cpuPercent }}%</span>
      </div>
    </section>

    <!-- Memory -->
    <section class="info-section">
      <h2 class="section-title">
        <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <rect x="2" y="6" width="20" height="12" rx="2"/><line x1="6" y1="10" x2="6" y2="14"/><line x1="10" y1="10" x2="10" y2="14"/><line x1="14" y1="10" x2="14" y2="14"/><line x1="18" y1="10" x2="18" y2="14"/>
        </svg>
        内存
      </h2>
      <div class="usage-bar-wrapper">
        <div class="usage-bar">
          <div class="usage-fill" :style="{ width: memPercent + '%' }"></div>
        </div>
        <span class="usage-text">{{ memPercent }}%</span>
      </div>
      <div class="metric-sub">
        <span>已用 {{ memText(status.memUsed) }}</span>
        <span class="metric-divider">/</span>
        <span>总计 {{ memText(status.memTotal) }}</span>
      </div>
    </section>

    <!-- GPU -->
    <section class="info-section">
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
          <div class="gpu-mem">显存 {{ memText(gpu.memUsed) }} / {{ memText(gpu.memTotal) }}</div>
        </div>
      </div>
      <div v-else class="info-empty">未检测到 NVIDIA GPU</div>
    </section>

  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { getMonitorStatus } from '../wails'
import { appendHistory, chartPoints, formatPromptTps, formatUptime, type MonitorStatus } from '../lib/monitor'
import { formatBytes } from '../lib/format'

const status = ref<MonitorStatus>({
  cpuPercent: 0,
  memUsed: 0,
  memTotal: 0,
  gpus: [],
  serverRunning: false,
  promptTps: 0,
  decodeTps: 0,
  uptimeSeconds: 0,
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

const promptTpsText = computed(() => formatPromptTps(status.value.promptTps))

const decodeTpsText = computed(() => status.value.decodeTps.toFixed(1))

const decodePoints = computed(() => chartPoints(decodeHistory.value, chartWidth, chartHeight))

// 显存/内存可能为 0（如数据未到），formatBytes(0) 返回空串，此处兜底为 "0 B"
function memText(bytes: number): string {
  return formatBytes(bytes) || '0 B'
}

async function fetchMonitorStatus() {
  try {
    const s = await getMonitorStatus()
    status.value = s
    decodeHistory.value = appendHistory(decodeHistory.value, s.decodeTps)
  } catch {
    // 轮询失败静默保持上次数据，不打断监控页
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

onMounted(startPolling)
onUnmounted(stopPolling)
</script>

<style scoped>
.page {
  padding: 36px 48px 60px;
  max-width: 960px;
}

.page-header {
  margin-bottom: 36px;
}

.page-title {
  font-size: 28px;
  font-weight: 700;
  color: var(--text-primary);
  margin: 0 0 4px;
  letter-spacing: -0.5px;
}

.page-subtitle {
  font-size: 14px;
  color: var(--text-dim);
  margin: 0;
}

/* ─── Section ─── */
.info-section {
  margin-bottom: 20px;
  padding: 24px 28px;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 14px;
  transition: border-color 0.2s;
}

.info-section:hover {
  border-color: var(--overlay-10);
}

.section-title {
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 15px;
  font-weight: 600;
  color: var(--text-secondary);
  margin: 0 0 18px;
}

.section-title svg {
  color: var(--accent-light);
  flex-shrink: 0;
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

/* ─── Status badge ─── */
.status-badge {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 2px 12px;
  border-radius: 20px;
  font-size: 12px;
  font-weight: 600;
}

.status-badge.available {
  background: rgba(34, 197, 94, 0.12);
  color: #22c55e;
  border: 1px solid rgba(34, 197, 94, 0.2);
}

.status-badge.unavailable {
  background: rgba(239, 68, 68, 0.1);
  color: #ef4444;
  border: 1px solid rgba(239, 68, 68, 0.15);
}

/* ─── Server head ─── */
.server-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 32px;
  margin-bottom: 18px;
}

.uptime-block {
  display: flex;
  flex-direction: column;
}

.tps-value {
  font-size: 32px;
  font-weight: 700;
  color: var(--accent-light);
  line-height: 1.1;
}

.tps-label,
.uptime-label {
  font-size: 11px;
  color: var(--text-dim);
}

.uptime-value {
  font-size: 14px;
  font-weight: 600;
  color: var(--text-secondary);
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
  padding: 20px;
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
  margin-top: 12px;
}

.tps-footnote {
  margin: 12px 0 0;
  font-size: 12px;
  color: var(--text-dim);
}

/* ─── TPS chart ─── */
.tps-chart {
  margin-top: 18px;
}

.tps-chart svg {
  display: block;
  width: 100%;
  height: 120px;
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
