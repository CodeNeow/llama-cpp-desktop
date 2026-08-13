<template>
  <div class="page">
    <div class="page-header">
      <h1 class="page-title">下载</h1>
      <p class="page-subtitle">从 {{ sourceLabel }} 搜索和下载模型</p>
    </div>

    <!-- Search bar -->
    <div class="search-bar">
      <div class="search-input-wrap">
        <svg class="search-icon" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/>
        </svg>
        <input
          v-model="searchQuery"
          class="search-input"
          placeholder="搜索模型... (如 bge, all-MiniLM, gte)"
          @keydown.enter="doSearch"
        />
      </div>
      <button class="search-btn" @click="doSearch" :disabled="searching || !searchQuery">
        {{ searching ? '搜索中...' : '搜索' }}
      </button>
      <button class="search-btn download-btn" @click="showTasksModal = true" title="查看下载任务">
        下载
        <span v-if="activeTaskCount > 0" class="task-badge">{{ activeTaskCount }}</span>
      </button>
    </div>

    <!-- Search results -->
    <div v-if="searchResults.length > 0" class="results-section">
      <h2 class="section-heading">搜索结果 ({{ searchResults.length }})</h2>
      <div class="results-grid">
        <div
          v-for="r in searchResults"
          :key="r.modelId"
          class="result-card"
          :class="{ expanded: expandedModel === r.modelId }"
          @click="toggleModel(r)"
        >
          <div class="result-main">
            <div class="result-icon">
              <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
                <path d="M21 16V8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16z"/>
              </svg>
            </div>
            <div class="result-info">
              <h3 class="result-name">{{ r.modelId }}</h3>
              <div class="result-meta">
                <span class="result-author">{{ r.author }}</span>
                <span v-if="r.pipelineTag" class="result-tag">{{ r.pipelineTag }}</span>
                <span class="result-downloads">⬇ {{ formatNum(r.downloads) }}</span>
                <span v-if="(r.likes || 0) > 0" class="result-likes">♥ {{ formatNum(r.likes) }}</span>
                <span v-if="modelSizes[r.modelId]" class="result-size">💾 {{ formatBytes(modelSizes[r.modelId]) }}</span>
              </div>
            </div>
            <span class="result-arrow" :class="{ open: expandedModel === r.modelId }">▾</span>
          </div>

          <!-- Expanded: description + file list -->
          <div v-if="expandedModel === r.modelId" class="result-files" @click.stop>
            <div v-if="descriptionsLoading[r.modelId]" class="result-desc">加载描述中...</div>
            <div v-else-if="modelDescriptions[r.modelId]" class="result-desc">{{ modelDescriptions[r.modelId] }}</div>
            <div v-if="modelFilesLoading[r.modelId]" class="files-loading">加载文件列表...</div>
            <div v-else-if="modelFiles[r.modelId] && modelFiles[r.modelId].length > 0" class="files-list">
              <label
                v-for="f in sortedFiles(modelFiles[r.modelId])"
                :key="f.filename"
                class="file-item"
                :class="{ selected: selectedFiles[r.modelId]?.includes(f.filename) }"
              >
                <input
                  type="checkbox"
                  :checked="selectedFiles[r.modelId]?.includes(f.filename)"
                  @change="toggleFile(r.modelId, f.filename)"
                />
                <span class="file-name">{{ f.filename }}</span>
                <span v-if="guessQuant(f.filename)" class="file-quant">{{ guessQuant(f.filename) }}</span>
                <span class="file-size" v-if="f.size">{{ formatBytes(f.size) }}</span>
              </label>
              <div class="files-actions">
                <button class="select-all-btn" @click="selectAllFiles(r.modelId)">全选</button>
                <button
                  class="download-files-btn"
                  :disabled="!selectedFiles[r.modelId]?.length"
                  @click="startDownload(r.modelId)"
                >下载选中文件</button>
              </div>
            </div>
            <div v-else class="files-empty">无可用文件</div>
          </div>
        </div>
      </div>
    </div>

    <!-- No results -->
    <div v-else-if="searched && !searching" class="empty-state">
      <h2>未找到结果</h2>
      <p>尝试其他关键词</p>
    </div>

    <!-- Download tasks modal -->
    <transition name="modal-fade">
      <div v-if="showTasksModal" class="modal-overlay" @click.self="showTasksModal = false">
        <div class="modal-panel">
          <div class="modal-header">
            <h2 class="modal-title">下载任务 ({{ tasks.length }})</h2>
            <button class="modal-close" @click="showTasksModal = false" title="关闭">✕</button>
          </div>
          <div class="modal-body">
            <div v-if="tasks.length === 0" class="tasks-empty">暂无下载任务</div>
            <div v-else class="task-list">
              <div v-for="t in tasks" :key="t.id" class="task-card">
                <div class="task-info">
                  <span class="task-name">{{ t.fileName }}</span>
                  <span class="task-model">{{ t.modelId }}</span>
                </div>
                <div class="task-bar-wrap">
                  <div class="task-bar">
                    <div
                      class="task-fill"
                      :class="taskBarClass(t.status)"
                      :style="{ width: t.progress + '%' }"
                    ></div>
                  </div>
                  <span class="task-percent">{{ t.progress }}%</span>
                </div>
                <div class="task-meta">
                  <span class="task-status" :class="'status-' + t.status">{{ statusMap[t.status] || t.status }}</span>
                  <span v-if="t.status === 'downloading' && t.speed > 0" class="task-speed">{{ formatSpeed(t.speed) }}</span>
                  <span class="task-size" v-if="t.sizeHuman && t.sizeHuman !== '0 B'">{{ t.sizeHuman }}</span>
                </div>
                <div class="task-error" v-if="t.error">
                  <span>{{ t.error }}</span>
                </div>
                <div class="task-actions">
                  <button
                    v-if="t.status === 'downloading'"
                    class="task-btn pause-btn"
                    @click="pauseTask(t.id)"
                  >⏸ 暂停</button>
                  <button
                    v-if="t.status === 'paused'"
                    class="task-btn resume-btn"
                    @click="resumeTask(t.id)"
                  >▶ 继续</button>
                  <button
                    v-if="t.status === 'error'"
                    class="task-btn retry-btn"
                    @click="retryTask(t.id)"
                  >↻ 重试</button>
                  <button
                    v-if="t.status === 'downloading' || t.status === 'paused' || t.status === 'queued' || t.status === 'error'"
                    class="task-btn cancel-btn"
                    @click="cancelTask(t.id)"
                  >✕ 取消</button>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </transition>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, onUnmounted } from 'vue'
import {
  searchDownloads, getModelFiles, getModelDescription, getModelMaxFileSize, startDownload as startHFDownload, getDownloadTasks,
  cancelDownloadTask, retryDownloadTask, pauseDownloadTask, resumeDownloadTask, refreshModels, getDownloadSource
} from '../wails'
import { formatSpeed, formatBytes } from '../lib/format'
import { LatestOnly } from '../lib/latestOnly'
import { hasActiveTask, countActiveTasks } from '../lib/taskStatus'
import { LimitedQueue } from '../lib/limitedQueue'

interface HFResult {
  modelId: string
  author: string
  downloads?: number
  likes?: number
  pipelineTag?: string
  tags?: string[]
}

interface HFFile {
  filename: string
  size?: number
}

interface DlTask {
  id: string
  modelId: string
  fileName: string
  status: string
  progress: number
  total: number
  downloaded: number
  sizeHuman: string
  error: string
  source: string
  speed: number
}

const searchQuery = ref('')
const searching = ref(false)
const searched = ref(false)
const searchResults = ref<HFResult[]>([])
// 当前下载源（"hf" | "modelscope"），挂载时从后端读取，切换由设置页完成
const downloadSource = ref('')
const sourceLabel = computed(() => downloadSource.value === 'modelscope' ? 'ModelScope' : 'HF 镜像')
// 模型大小缓存：值为最大 GGUF 文件字节数，0 表示已尝试查询（无 GGUF 或失败），
// 用于避免重复请求；按 modelId 缓存，跨搜索复用
const modelSizes = reactive<Record<string, number>>({})
// 搜索卡片大小批量请求的并发受限队列：每轮搜索重建实例，避免旧任务堆积
let sizeQueue = new LimitedQueue(4)
const expandedModel = ref('')
const modelFiles = reactive<Record<string, HFFile[]>>({})
const modelFilesLoading = reactive<Record<string, boolean>>({})
const selectedFiles = reactive<Record<string, string[]>>({})
// 模型描述懒加载缓存：描述文本与加载中标记按 modelId 记录，避免展开时重复请求
const modelDescriptions = reactive<Record<string, string>>({})
const descriptionsLoading = reactive<Record<string, boolean>>({})

const tasks = ref<DlTask[]>([])
let taskPollTimer: ReturnType<typeof setInterval> | null = null
let lastDoneCount = 0

// 下载任务 Modal：仅控制面板显示/隐藏，不影响后台 1 秒轮询
const showTasksModal = ref(false)
// 活跃任务数（downloading/paused/queued），驱动下载按钮角标；fetchTasks 更新 tasks 后自动刷新
const activeTaskCount = computed(() => countActiveTasks(tasks.value))

const statusMap: Record<string, string> = {
  queued: '排队中',
  downloading: '下载中',
  paused: '已暂停',
  done: '已完成',
  error: '下载失败',
  cancelled: '已取消',
}

function formatNum(n: number | undefined): string {
  const v = n || 0
  if (v >= 1000000) return (v / 1000000).toFixed(1) + 'M'
  if (v >= 1000) return (v / 1000).toFixed(1) + 'K'
  return String(v)
}

function taskBarClass(status: string): string {
  if (status === 'paused') return 'paused'
  if (status === 'error') return 'error-fill'
  return ''
}

function sortedFiles(files: HFFile[]): HFFile[] {
  // ModelScope 源的文件可能缺 size，按 0 兜底排序避免 NaN
  return [...files].sort((a, b) => (b.size || 0) - (a.size || 0))
}

function guessQuant(filename: string): string {
  const name = filename.toLowerCase()
  const quants = [
    'Q8_K', 'Q8_0', 'Q6_K', 'Q5_K_M', 'Q5_K_S', 'Q5_1', 'Q5_0',
    'Q4_K_M', 'Q4_K_S', 'Q4_1', 'Q4_0', 'Q3_K_L', 'Q3_K_M', 'Q3_K_S',
    'Q2_K', 'IQ4_NL', 'IQ4_XS', 'IQ3_M', 'IQ3_S', 'IQ3_XXS', 'IQ2_XS', 'IQ2_XXS',
    'F16', 'F32', 'BF16',
  ]
  for (const q of quants) {
    if (name.includes(q.toLowerCase())) return q
  }
  return ''
}

const searchGate = new LatestOnly()
async function doSearch() {
  if (!searchQuery.value) return
  const seq = searchGate.begin()
  searching.value = true
  searched.value = true
  try {
    const results = await searchDownloads(searchQuery.value, 'all')
    if (!searchGate.isLatest(seq)) return  // 过期结果丢弃（#15）
    searchResults.value = results || []
    // 新搜索重建大小队列：旧队列任务让其在缓存保护下自然结束
    sizeQueue = new LimitedQueue(4)
    for (const r of searchResults.value) loadModelSize(r.modelId)
  } catch {} finally {
    if (searchGate.isLatest(seq)) searching.value = false
  }
}

// loadModelSize 惰性获取模型最大 GGUF 大小并写入缓存：并发受限队列控制
// 请求峰值（搜索可返回 200 个模型，不能全部同时请求）；modelSizes 缓存
// 保证同一模型只查询一次，失败或为 0 不再重试（卡片不显示大小）。
function loadModelSize(modelId: string) {
  if (modelId in modelSizes) return
  modelSizes[modelId] = 0 // 占位：标记已尝试，避免重复入队
  sizeQueue.push(async () => {
    try {
      const size = await getModelMaxFileSize(modelId)
      if (size > 0) modelSizes[modelId] = size
    } catch {
      // 查询失败静默忽略，卡片不显示大小
    }
  })
}

async function toggleModel(r: HFResult) {
  if (expandedModel.value === r.modelId) {
    expandedModel.value = ''
    return
  }
  expandedModel.value = r.modelId

  // 模型描述懒加载：已加载过则跳过；失败/为空静默忽略，不阻塞文件列表
  if (!modelDescriptions[r.modelId] && !descriptionsLoading[r.modelId]) {
    descriptionsLoading[r.modelId] = true
    getModelDescription(r.modelId)
      .then(desc => {
        if (desc) modelDescriptions[r.modelId] = desc
      })
      .catch(() => {})
      .finally(() => { descriptionsLoading[r.modelId] = false })
  }

  if (!modelFiles[r.modelId]) {
    modelFilesLoading[r.modelId] = true
    try {
      const files = await getModelFiles(r.modelId)
      modelFiles[r.modelId] = files || []
      selectedFiles[r.modelId] = []
    } catch {} finally {
      modelFilesLoading[r.modelId] = false
    }
  }
}

function toggleFile(modelId: string, filename: string) {
  const arr = selectedFiles[modelId] || []
  const idx = arr.indexOf(filename)
  if (idx >= 0) {
    selectedFiles[modelId] = arr.filter(f => f !== filename)
  } else {
    selectedFiles[modelId] = [...arr, filename]
  }
}

function selectAllFiles(modelId: string) {
  const files = modelFiles[modelId]
  if (!files) return
  selectedFiles[modelId] = files.map(f => f.filename)
}

async function startDownload(modelId: string) {
  const sel = selectedFiles[modelId]
  if (!sel || sel.length === 0) return
  try {
    await startHFDownload(modelId, sel)
    expandedModel.value = ''
    ensurePolling()
  } catch {}
}

async function fetchTasks() {
  try {
    tasks.value = await getDownloadTasks() || []
    // 检测到新增完成任务时强制重扫模型列表（#18）
    const doneCount = tasks.value.filter(t => t.status === 'done').length
    if (doneCount > lastDoneCount) {
      lastDoneCount = doneCount
      refreshModels().catch(() => {})
    }
    // 全部任务进入终态后停止轮询（#16）
    if (!hasActiveTask(tasks.value) && taskPollTimer) {
      clearInterval(taskPollTimer)
      taskPollTimer = null
    }
  } catch {}
}

async function cancelTask(id: string) {
  try {
    await cancelDownloadTask(id)
    ensurePolling()
  } catch {}
}

async function retryTask(id: string) {
  try {
    await retryDownloadTask(id)
    ensurePolling()
  } catch {}
}

async function pauseTask(id: string) {
  try {
    await pauseDownloadTask(id)
    ensurePolling()
  } catch {}
}

async function resumeTask(id: string) {
  try {
    await resumeDownloadTask(id)
    ensurePolling()
  } catch {}
}

/** 启动新下载或手动操作后：轮询已停止则重启，否则立即刷新一次（#16） */
function ensurePolling() {
  if (taskPollTimer) {
    fetchTasks()
  } else {
    startTaskPolling()
  }
}

function startTaskPolling() {
  taskPollTimer = setInterval(fetchTasks, 1000)
  fetchTasks()
}

/** 挂载时读取当前下载源；失败时静默保持默认 "hf"（HF 镜像） */
async function loadDownloadSource() {
  try {
    const source = await getDownloadSource()
    downloadSource.value = source || 'hf'
  } catch {}
}

onMounted(() => {
  startTaskPolling()
  loadDownloadSource()
})
onUnmounted(() => { if (taskPollTimer) clearInterval(taskPollTimer) })
</script>

<style scoped>
.page {
  padding: 36px 48px 60px;
  max-width: 960px;
}

.page-header {
  margin-bottom: 28px;
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

/* ─── Search ─── */
.search-bar {
  display: flex;
  gap: 10px;
  margin-bottom: 32px;
}

.search-input-wrap {
  flex: 1;
  position: relative;
}

.search-icon {
  position: absolute;
  left: 14px;
  top: 50%;
  transform: translateY(-50%);
  color: var(--text-dim);
  pointer-events: none;
}

.search-input {
  width: 100%;
  padding: 10px 14px 10px 42px;
  background: var(--border-light);
  border: 1px solid var(--overlay-8);
  border-radius: 10px;
  color: var(--text-secondary);
  font-size: 14px;
  outline: none;
  transition: border-color 0.2s;
}

.search-input:focus {
  border-color: rgba(99, 102, 241, 0.4);
}

.search-btn {
  padding: 10px 24px;
  background: linear-gradient(135deg, rgba(99, 102, 241, 0.2), rgba(167, 139, 250, 0.15));
  color: #a78bfa;
  border: 1px solid rgba(99, 102, 241, 0.3);
  border-radius: 10px;
  font-size: 14px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s;
}

.search-btn:hover:not(:disabled) {
  background: linear-gradient(135deg, rgba(99, 102, 241, 0.3), rgba(167, 139, 250, 0.25));
}

.search-btn:disabled {
  opacity: 0.4;
  cursor: default;
}

.download-btn {
  position: relative;
}

.task-badge {
  position: absolute;
  top: -7px;
  right: -7px;
  min-width: 18px;
  height: 18px;
  padding: 0 5px;
  border-radius: 9px;
  background: #ef4444;
  color: #fff;
  font-size: 11px;
  font-weight: 700;
  line-height: 18px;
  text-align: center;
  box-sizing: border-box;
}

/* ─── Section heading ─── */
.section-heading {
  font-size: 16px;
  font-weight: 600;
  color: var(--text-secondary);
  margin: 0 0 14px;
}

/* ─── Results ─── */
.results-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 10px;
  margin-bottom: 36px;
}

.result-card {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 12px;
  cursor: pointer;
  transition: all 0.2s;
  overflow: hidden;
}

.result-card:hover {
  border-color: var(--overlay-10);
}

.result-card.expanded {
  border-color: rgba(99, 102, 241, 0.2);
}

.result-main {
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 12px 16px;
}

.result-icon {
  width: 40px;
  height: 40px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(99, 102, 241, 0.1);
  border-radius: 10px;
  color: #a78bfa;
  flex-shrink: 0;
}

.result-info {
  flex: 1;
  min-width: 0;
}

.result-name {
  font-size: 14px;
  font-weight: 600;
  color: var(--text-secondary);
  margin: 0 0 4px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.result-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  font-size: 12px;
  color: var(--text-dim);
}

.result-likes {
  color: #f472b6;
}

.result-size {
  color: #22c55e;
}

.result-tag {
  background: rgba(99, 102, 241, 0.1);
  color: #a78bfa;
  padding: 1px 8px;
  border-radius: 4px;
  font-size: 11px;
}

.result-arrow {
  color: var(--text-dim);
  font-size: 14px;
  transition: transform 0.2s;
  flex-shrink: 0;
}

.result-arrow.open {
  transform: rotate(180deg);
}

/* ─── File list ─── */
.result-files {
  border-top: 1px solid var(--border);
  padding: 14px 18px;
  max-height: 300px;
  overflow-y: auto;
}

.result-desc {
  font-size: 12px;
  line-height: 1.6;
  color: var(--text-dim);
  margin-bottom: 12px;
  padding: 10px 12px;
  background: var(--bg-card);
  border-radius: 8px;
}

.files-loading, .files-empty {
  font-size: 13px;
  color: var(--text-dim);
  padding: 8px 0;
}

.files-list {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.file-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 6px 10px;
  border-radius: 6px;
  cursor: pointer;
  transition: background 0.15s;
  font-size: 13px;
}

.file-item:hover {
  background: var(--bg-card);
}

.file-item.selected {
  background: rgba(99, 102, 241, 0.08);
}

.file-item input {
  accent-color: #6366f1;
  flex-shrink: 0;
}

.file-name {
  flex: 1;
  color: var(--text-secondary);
  word-break: break-all;
}

.file-size {
  color: var(--text-dim);
  font-size: 11px;
  flex-shrink: 0;
}

.file-quant {
  background: rgba(34, 197, 94, 0.1);
  color: #4ade80;
  padding: 1px 7px;
  border-radius: 4px;
  font-size: 10px;
  font-weight: 600;
  flex-shrink: 0;
  letter-spacing: 0.3px;
}

.files-actions {
  display: flex;
  gap: 8px;
  margin-top: 10px;
  padding-top: 10px;
  border-top: 1px solid var(--border-light);
}

.select-all-btn {
  padding: 6px 14px;
  background: transparent;
  border: 1px solid var(--overlay-10);
  border-radius: 6px;
  color: var(--text-muted);
  font-size: 12px;
  cursor: pointer;
}

.select-all-btn:hover {
  border-color: var(--overlay-20);
  color: var(--text-primary);
}

.download-files-btn {
  padding: 6px 18px;
  background: rgba(99, 102, 241, 0.15);
  color: #a78bfa;
  border: 1px solid rgba(99, 102, 241, 0.25);
  border-radius: 6px;
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s;
}

.download-files-btn:hover:not(:disabled) {
  background: rgba(99, 102, 241, 0.25);
}

.download-files-btn:disabled {
  opacity: 0.4;
  cursor: default;
}

/* ─── Tasks ─── */
.task-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.task-card {
  padding: 16px 20px;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 12px;
}

.task-info {
  display: flex;
  justify-content: space-between;
  margin-bottom: 8px;
}

.task-name {
  font-size: 13px;
  font-weight: 600;
  color: var(--text-secondary);
  word-break: break-all;
}

.task-model {
  font-size: 11px;
  color: var(--text-dim);
  flex-shrink: 0;
  margin-left: 12px;
}

.task-bar-wrap {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 8px;
}

.task-bar {
  flex: 1;
  height: 5px;
  background: var(--border);
  border-radius: 3px;
  overflow: hidden;
}

.task-fill {
  height: 100%;
  border-radius: 3px;
  background: linear-gradient(90deg, #6366f1, #a78bfa);
  transition: width 0.3s;
}

.task-fill.paused {
  background: linear-gradient(90deg, #fbbf24, #f59e0b);
}

.task-fill.error-fill {
  background: rgba(239, 68, 68, 0.6);
}

.task-percent {
  font-size: 12px;
  font-weight: 600;
  color: #a78bfa;
  min-width: 36px;
  text-align: right;
}

.task-meta {
  display: flex;
  justify-content: space-between;
  font-size: 11px;
  margin-bottom: 8px;
}

.task-status {
  font-weight: 500;
}

.status-done { color: #22c55e; }
.status-downloading { color: #a78bfa; }
.status-paused { color: #fbbf24; }
.status-queued { color: var(--text-dim); }
.status-error { color: #ef4444; }
.status-cancelled { color: var(--overlay-20); }

.task-size {
  color: var(--text-dim);
}

.task-speed {
  color: var(--accent-light);
  font-weight: 600;
}

.task-error {
  font-size: 11px;
  color: #ef4444;
  margin-bottom: 8px;
  padding: 6px 10px;
  background: rgba(239, 68, 68, 0.06);
  border-radius: 6px;
}

.task-actions {
  display: flex;
  gap: 6px;
}

.task-btn {
  padding: 4px 12px;
  border-radius: 6px;
  font-size: 11px;
  font-weight: 500;
  cursor: pointer;
  border: 1px solid transparent;
  transition: all 0.2s;
}

.pause-btn {
  background: rgba(251, 191, 36, 0.1);
  color: #fbbf24;
  border-color: rgba(251, 191, 36, 0.2);
}

.pause-btn:hover { background: rgba(251, 191, 36, 0.2); }

.resume-btn {
  background: rgba(34, 197, 94, 0.1);
  color: #22c55e;
  border-color: rgba(34, 197, 94, 0.2);
}

.resume-btn:hover { background: rgba(34, 197, 94, 0.2); }

.retry-btn {
  background: rgba(99, 102, 241, 0.1);
  color: #a78bfa;
  border-color: rgba(99, 102, 241, 0.2);
}

.retry-btn:hover { background: rgba(99, 102, 241, 0.2); }

.cancel-btn {
  background: rgba(239, 68, 68, 0.06);
  color: rgba(239, 68, 68, 0.7);
  border-color: rgba(239, 68, 68, 0.12);
}

.cancel-btn:hover { background: rgba(239, 68, 68, 0.12); }

/* ─── Tasks modal ─── */
.modal-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.5);
  z-index: 1000;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px;
}

.modal-panel {
  width: 100%;
  max-width: 560px;
  max-height: 80vh;
  background: var(--bg-primary);
  border: 1px solid var(--border);
  border-radius: 14px;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  box-shadow: 0 20px 60px rgba(0, 0, 0, 0.4);
}

.modal-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px 20px;
  border-bottom: 1px solid var(--border);
  flex-shrink: 0;
}

.modal-title {
  font-size: 16px;
  font-weight: 600;
  color: var(--text-secondary);
  margin: 0;
}

.modal-close {
  background: transparent;
  border: none;
  color: var(--text-dim);
  font-size: 16px;
  line-height: 1;
  cursor: pointer;
  padding: 6px 8px;
  border-radius: 6px;
  transition: all 0.2s;
}

.modal-close:hover {
  background: var(--hover-bg);
  color: var(--text-primary);
}

.modal-body {
  padding: 16px 20px;
  overflow-y: auto;
}

.tasks-empty {
  padding: 48px 0;
  text-align: center;
  color: var(--text-dim);
  font-size: 14px;
}

.modal-fade-enter-active,
.modal-fade-leave-active {
  transition: opacity 0.2s;
}

.modal-fade-enter-from,
.modal-fade-leave-to {
  opacity: 0;
}

/* ─── Empty ─── */
.empty-state {
  text-align: center;
  padding: 48px;
  color: var(--text-dim);
}

.empty-state h2 {
  font-size: 18px;
  color: var(--text-muted);
  margin: 0 0 8px;
}
</style>
