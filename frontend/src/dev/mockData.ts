/**
 * Mock preview data layer (dev only, wired in exclusively by the
 * `--mode mock` Vite alias in vite.config.ts → src/dev/mockRuntime.ts).
 *
 * Holds the whole mutable fake backend state (config / system probe / models /
 * server / downloads / docs) and the handler table keyed by the bare binding
 * method name. Shapes mirror the generated binding classes in
 * frontend/bindings/github.com/CodeNeow/llama-cpp-desktop/core/models.ts, which
 * in turn mirror the Go structs in core/. Data is tuned for the Android phone
 * layout walkthrough: OS = android/arm64, GPU list empty (android probe is
 * unsupported → no GPU/CUDA cards on Home), tray / API-route / serving-GPU
 * settings hidden, update section in link mode.
 */

// ─── Small helpers ───────────────────────────────────────────────────────────

/** Promise-based delay. */
function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms))
}

/** Symmetric random jitter around a base value, clamped at 0. */
function jitter(base: number, amp: number): number {
  return Math.max(0, base + (Math.random() * 2 - 1) * amp)
}

/** GiB → bytes. */
const GiB = 1024 * 1024 * 1024

// ─── Config (mirrors the persisted llama-desktop-config.json) ────────────────

const config: Record<string, any> = {
  theme: 'light',
  llamaCppDir: 'C:\\Users\\demo\\llama.cpp',
  modelsDir: 'C:\\Users\\demo\\LLM-Models',
  llamaCppDownloadDir: 'C:\\Users\\demo\\llama.cpp-dl',
  modelDownloadDir: 'C:\\Users\\demo\\model-downloads',
  downloadSource: 'hf',
  language: 'zh',
  resolvedLanguage: 'zh',
  trayEnabled: false,
  apiRouteMode: false,
  sidebarCollapsed: false,
  onboardingDismissed: true,
}

// ─── System probes (android semantics: no GPU/CUDA, CPU-only accel) ─────────

const os = { os: 'android', arch: 'arm64' }

const cpu = { model: 'Mock Snapdragon 8 Gen 3', cores: 8, logicalCpus: 8 }

const memory = { totalGb: 16, freeGb: 9.4 }

// Android: gpuProbesUnsupported → always empty, no GPU/CUDA cards on Home.
const gpus: any[] = []

const cuda = { available: false, driverVersion: '', toolkitVersion: '' }

const llamaCpp = {
  installed: true,
  path: 'C:\\Users\\demo\\llama.cpp\\llama-server',
  version: 'b5913',
  cudartInstalled: false,
  cudartVersion: '',
  accel: 'cpu',
}

const disk = { path: 'C:\\', used: 231 * GiB, total: 512 * GiB }

const systemInfo = {
  os: os.os,
  arch: os.arch,
  cpu,
  memory,
  gpu: gpus,
  cuda,
  llamaCpp,
  disk,
}

// ─── Local models (download dir + external import dir) ───────────────────────

const IMPORT_DIR = 'C:\\Users\\demo\\Imported-Models'

const models: any[] = [
  {
    author: '',
    name: 'Qwen3-4B-Instruct-Q4_K_M',
    path: 'C:\\Users\\demo\\model-downloads\\Qwen3-4B-Instruct-Q4_K_M.gguf',
    sizeBytes: 2_476_000_000,
    sizeHuman: '2.3 GB',
    architecture: 'qwen3',
    quantization: 'Q4_K_M',
    hasMmproj: false,
    sourceDir: config.modelDownloadDir,
    alias: 'Qwen3-4B-Instruct-Q4_K_M',
  },
  {
    author: '',
    name: 'Qwen2.5-Coder-7B-Instruct-Q4_K_M',
    path: 'C:\\Users\\demo\\model-downloads\\Qwen2.5-Coder-7B-Instruct-Q4_K_M.gguf',
    sizeBytes: 4_689_000_000,
    sizeHuman: '4.4 GB',
    architecture: 'qwen2',
    quantization: 'Q4_K_M',
    hasMmproj: false,
    sourceDir: config.modelDownloadDir,
    alias: 'Qwen2.5-Coder-7B-Instruct-Q4_K_M',
  },
  {
    author: '',
    name: 'Llama-3.2-3B-Instruct-Q5_K_M',
    path: IMPORT_DIR + '\\Llama-3.2-3B-Instruct-Q5_K_M.gguf',
    sizeBytes: 2_291_000_000,
    sizeHuman: '2.1 GB',
    architecture: 'llama',
    quantization: 'Q5_K_M',
    hasMmproj: false,
    sourceDir: IMPORT_DIR,
    alias: 'Llama-3.2-3B-Instruct-Q5_K_M',
  },
  {
    author: '',
    name: 'TinyLlama-1.1B-Chat-v1.0-Q8_0',
    path: IMPORT_DIR + '\\TinyLlama-1.1B-Chat-v1.0-Q8_0.gguf',
    sizeBytes: 1_146_000_000,
    sizeHuman: '1.1 GB',
    architecture: 'llama',
    quantization: 'Q8_0',
    hasMmproj: false,
    sourceDir: IMPORT_DIR,
    alias: 'TinyLlama-1.1B-Chat-v1.0-Q8_0',
  },
]

// ─── Server state (running on startup so the API page / TaskDock have data) ──

const serverConfig: Record<string, any> = {
  accessMode: 'local',
  host: '127.0.0.1',
  port: 8080,
  maxModels: 4,
  cacheRam: 8192,
  apiKey: '',
  deviceId: '',
}

let serverRunning = true
// Wall-clock anchor for the uptime counter: the service "started" ~97 min ago.
let serverStartedAt = Date.now() - 97 * 60 * 1000
// Android direct-mode resident model (single in-memory model).
let residentModel = 'Qwen3-4B-Instruct-Q4_K_M'

let loadedModels: any[] = [{ id: residentModel, type: 'chat', status: 'loaded' }]

// Server log ring (mirrors core/server.go: monotonic seq + capped lines).
const serverLogsCap = 2000
let logSeq = 0
const serverLogRing: { seq: number; text: string }[] = []

function addServerLog(text: string): void {
  logSeq += 1
  serverLogRing.push({ seq: logSeq, text })
  if (serverLogRing.length > serverLogsCap) serverLogRing.shift()
}

// Initial startup transcript, [INFO]-prefixed like the real capture.
addServerLog(
  '[INFO] Starting llama-server: C:\\Users\\demo\\llama.cpp\\llama-server --host 127.0.0.1 --port 8080 ' +
    '--max-models 4 --cache-ram 8192',
)
addServerLog('main: llama-server version b5913 (mock)')
addServerLog('main: build = cpu-arm64 (mock android build)')
addServerLog('main: system memory used = 2810.4 MiB')
addServerLog('main: listening on http://127.0.0.1:8080')
addServerLog('srv  update_slots: all slots are idle')

function startServerLogs(): void {
  addServerLog(
    '[INFO] Starting llama-server: C:\\Users\\demo\\llama.cpp\\llama-server --host 127.0.0.1 --port 8080 ' +
      '--max-models 4 --cache-ram 8192',
  )
  addServerLog('main: llama-server version b5913 (mock)')
  addServerLog('main: listening on http://127.0.0.1:8080')
  addServerLog('srv  update_slots: all slots are idle')
}

// ─── Monitor sample (jitters per call so the API page chart draws a curve) ───

function monitorStatus(): Record<string, any> {
  if (!serverRunning) {
    return {
      cpuPercent: jitter(6, 4),
      memUsed: jitter(3.1, 0.3),
      memTotal: memory.totalGb,
      gpus,
      serverRunning: false,
      promptTps: 0,
      decodeTps: 0,
      uptimeSeconds: 0,
      disk,
    }
  }
  return {
    cpuPercent: Math.round(jitter(34, 12) * 10) / 10,
    memUsed: Math.round(jitter(7.2, 0.7) * 100) / 100,
    memTotal: memory.totalGb,
    gpus,
    serverRunning: true,
    promptTps: Math.round(jitter(118, 35) * 10) / 10,
    decodeTps: Math.round(jitter(15, 3) * 10) / 10,
    uptimeSeconds: Math.floor((Date.now() - serverStartedAt) / 1000),
    disk,
  }
}

// ─── Model download tasks (progress advances with the wall clock) ────────────

interface MockTask {
  id: string
  modelId: string
  fileName: string
  destDir: string
  source: string
  status: string
  total: number
  base: number
  since: number
  speed: number
  error: string
  queuedAt: number
}

const tasks: MockTask[] = [
  {
    id: 'task-qwen3-8b',
    modelId: 'Qwen/Qwen3-8B-GGUF',
    fileName: 'Qwen3-8B-Q4_K_M.gguf',
    destDir: config.modelDownloadDir,
    source: 'hf',
    status: 'downloading',
    total: 6_200_000_000,
    base: Math.floor(6_200_000_000 * 0.45),
    since: Date.now(),
    speed: 9_500_000,
    error: '',
    queuedAt: 0,
  },
  {
    id: 'task-tinyllama-done',
    modelId: 'TinyLlama/tinyllama-1.1b-chat-v1.0-GGUF',
    fileName: 'tinyllama-1.1b-chat-v1.0.Q8_0.gguf',
    destDir: config.modelDownloadDir,
    source: 'hf',
    status: 'done',
    total: 1_146_000_000,
    base: 1_146_000_000,
    since: 0,
    speed: 0,
    error: '',
    queuedAt: 0,
  },
]

/** Lazily advance a running task: wall-clock bytes since the last resume, flipping to done at 100%. */
function taskDownloaded(t: MockTask): number {
  if (t.status === 'downloading') {
    const bytes = Math.min(t.total, t.base + ((Date.now() - t.since) / 1000) * t.speed)
    if (bytes >= t.total) {
      t.status = 'done'
      t.base = t.total
      t.speed = 0
      return t.total
    }
    return bytes
  }
  return t.base
}

/** Promote tasks that sat in 'queued' for over 1.5s to 'downloading'. */
function promoteQueued(t: MockTask): void {
  if (t.status === 'queued' && Date.now() - t.queuedAt > 1500) {
    t.status = 'downloading'
    t.since = Date.now()
  }
}

function taskJson(t: MockTask): Record<string, any> {
  promoteQueued(t)
  const downloaded = Math.floor(taskDownloaded(t))
  return {
    id: t.id,
    modelId: t.modelId,
    fileName: t.fileName,
    destDir: t.destDir,
    source: t.source,
    status: t.status,
    progress: t.total > 0 ? Math.round((downloaded / t.total) * 100) : 0,
    total: t.total,
    downloaded,
    sizeHuman: (t.total / GiB).toFixed(1) + ' GB',
    speed: t.status === 'downloading' ? t.speed : 0,
    error: t.error,
  }
}

// ─── llama.cpp runtime download state ────────────────────────────────────────

const llamacppDl = {
  status: 'idle',
  paused: false,
  total: 0,
  base: 0,
  since: 0,
  version: '',
  error: '',
}

function llamacppDownloaded(): number {
  if (llamacppDl.status === 'downloading') {
    const bytes = Math.min(llamacppDl.total, llamacppDl.base + ((Date.now() - llamacppDl.since) / 1000) * 6_400_000)
    if (bytes >= llamacppDl.total) {
      llamacppDl.status = 'done'
      llamacppDl.base = llamacppDl.total
      return llamacppDl.total
    }
    return bytes
  }
  return llamacppDl.base
}

function llamacppState(): Record<string, any> {
  const downloaded = Math.floor(llamacppDownloaded())
  return {
    status: llamacppDl.status,
    paused: llamacppDl.paused,
    progress: llamacppDl.total > 0 ? Math.round((downloaded / llamacppDl.total) * 100) : 0,
    total: llamacppDl.total,
    downloaded,
    fileName: llamacppDl.version ? 'llama-cpp-android-arm64-' + llamacppDl.version + '.zip' : '',
    version: llamacppDl.version,
    error: llamacppDl.error,
  }
}

// ─── Search catalog (HF-shaped results for the Downloads tab) ────────────────

function gguf(name: string, size: number): Record<string, any> {
  return { rfilename: name, size }
}

const searchCatalog: Record<string, any>[] = [
  {
    id: 'Qwen/Qwen3-8B-GGUF',
    modelId: 'Qwen/Qwen3-8B-GGUF',
    author: 'Qwen',
    downloads: 128_400,
    likes: 2_310,
    pipelineTag: 'text-generation',
    tags: ['gguf', 'text-generation', 'qwen3'],
    siblings: [
      gguf('Qwen3-8B-Q4_K_M.gguf', 6_200_000_000),
      gguf('Qwen3-8B-Q5_K_M.gguf', 7_350_000_000),
      gguf('Qwen3-8B-Q8_0.gguf', 8_900_000_000),
      gguf('README.md', 18_400),
    ],
  },
  {
    id: 'Qwen/Qwen3-4B-Instruct-2507-GGUF',
    modelId: 'Qwen/Qwen3-4B-Instruct-2507-GGUF',
    author: 'Qwen',
    downloads: 96_800,
    likes: 1_540,
    pipelineTag: 'text-generation',
    tags: ['gguf', 'text-generation', 'qwen3'],
    siblings: [
      gguf('Qwen3-4B-Instruct-2507-Q4_K_M.gguf', 2_476_000_000),
      gguf('Qwen3-4B-Instruct-2507-Q8_0.gguf', 4_310_000_000),
      gguf('README.md', 15_100),
    ],
  },
  {
    id: 'bartowski/Llama-3.2-3B-Instruct-GGUF',
    modelId: 'bartowski/Llama-3.2-3B-Instruct-GGUF',
    author: 'bartowski',
    downloads: 210_300,
    likes: 3_120,
    pipelineTag: 'text-generation',
    tags: ['gguf', 'llama-3', 'instruct'],
    siblings: [
      gguf('Llama-3.2-3B-Instruct-Q4_K_M.gguf', 2_028_000_000),
      gguf('Llama-3.2-3B-Instruct-Q5_K_M.gguf', 2_291_000_000),
      gguf('Llama-3.2-3B-Instruct-Q8_0.gguf', 3_412_000_000),
      gguf('README.md', 9_800),
    ],
  },
  {
    id: 'google/gemma-2-2b-it-GGUF',
    modelId: 'google/gemma-2-2b-it-GGUF',
    author: 'google',
    downloads: 88_900,
    likes: 1_050,
    pipelineTag: 'text-generation',
    tags: ['gguf', 'gemma', 'instruction-tuned'],
    siblings: [
      gguf('gemma-2-2b-it-Q4_K_M.gguf', 1_672_000_000),
      gguf('gemma-2-2b-it-Q8_0.gguf', 2_700_000_000),
      gguf('README.md', 12_600),
    ],
  },
  {
    id: 'microsoft/Phi-4-mini-instruct-GGUF',
    modelId: 'microsoft/Phi-4-mini-instruct-GGUF',
    author: 'microsoft',
    downloads: 64_500,
    likes: 890,
    pipelineTag: 'text-generation',
    tags: ['gguf', 'phi', 'mini'],
    siblings: [
      gguf('Phi-4-mini-instruct-Q4_K_M.gguf', 2_487_000_000),
      gguf('Phi-4-mini-instruct-Q8_0.gguf', 3_980_000_000),
      gguf('README.md', 11_200),
    ],
  },
  {
    id: 'deepseek-ai/DeepSeek-R1-Distill-Qwen-7B-GGUF',
    modelId: 'deepseek-ai/DeepSeek-R1-Distill-Qwen-7B-GGUF',
    author: 'deepseek-ai',
    downloads: 173_900,
    likes: 2_860,
    pipelineTag: 'text-generation',
    tags: ['gguf', 'reasoning', 'distill'],
    siblings: [
      gguf('DeepSeek-R1-Distill-Qwen-7B-Q4_K_M.gguf', 4_689_000_000),
      gguf('DeepSeek-R1-Distill-Qwen-7B-Q8_0.gguf', 8_120_000_000),
      gguf('README.md', 22_900),
    ],
  },
]

function catalogEntry(modelID: string): Record<string, any> | undefined {
  const key = modelID.toLowerCase()
  return (
    searchCatalog.find((e) => e.id.toLowerCase() === key) ??
    searchCatalog.find((e) => e.id.toLowerCase().includes(key) || key.includes(e.id.toLowerCase().split('/')[1] ?? ''))
  )
}

// ─── Per-model inference configs ─────────────────────────────────────────────

const defaultModelConfig: Record<string, any> = {
  threads: 8,
  gpuLayers: '',
  ctxSize: 4096,
  batchSize: 2048,
  ubatchSize: 2048,
  flashAttn: false,
  cacheTypeK: '',
  cacheTypeV: '',
  loadMode: '',
  cpuMoe: false,
  nCpuMoe: 0,
  splitMode: '',
  tensorSplit: '',
  mainGpu: 0,
  ropeScaling: '',
  ropeScale: 0,
  mmproj: '',
  reasoning: false,
  specType: '',
  specDraftNMax: 0,
}

const modelConfigs: Record<string, Record<string, any>> = {}

// ─── Chat SSE sample answer (POST /v1/chat/completions via the mock runtime) ─

const chatReply =
  '你好！我是运行在 mock 预览模式里的假模型，并没有真的加载任何权重。' +
  '你现在看到的逐字输出，是用来演示聊天页的流式打字效果和手机档布局的。' +
  '真正的安卓构建会通过 Wails 绑定调用 Go 后端，由本机 llama-server 直接以 SSE 返回 OpenAI 兼容流。' +
  '这条回答结束后，可以试试底部标签栏的页面切换、TaskDock 的模型卸载按钮，以及把窗口拉宽对比桌面档布局。'

// ─── Handler table (keyed by bare binding method name) ───────────────────────

export const handlers: Record<string, (...args: any[]) => any> = {
  // ── Config ──
  GetConfig: () => ({ ...config }),
  SetTheme: (theme: string) => {
    config.theme = theme
  },
  SetSidebarCollapsed: (collapsed: boolean) => {
    config.sidebarCollapsed = collapsed
  },
  SetOnboardingDismissed: (dismissed: boolean) => {
    config.onboardingDismissed = dismissed
  },
  SetTrayEnabled: (enabled: boolean) => {
    config.trayEnabled = enabled
  },
  SetApiRouteMode: (enabled: boolean) => {
    config.apiRouteMode = enabled
  },
  SetLanguage: (language: string) => {
    if (language !== 'zh' && language !== 'en' && language !== 'auto') {
      throw new Error('invalid language: ' + language)
    }
    config.language = language
    config.resolvedLanguage = language === 'en' ? 'en' : 'zh'
    return config.resolvedLanguage
  },

  // ── System info ──
  GetSystemInfo: () => ({ ...systemInfo }),
  GetCPU: () => ({ ...cpu }),
  GetMemory: () => ({ ...memory }),
  GetGPU: () => [...gpus],
  GetCUDA: () => ({ ...cuda }),
  GetLlamaCpp: () => ({ ...llamaCpp }),
  GetOS: () => ({ ...os }),
  GetDisk: () => ({ ...disk }),

  // ── Monitor ──
  GetMonitorStatus: () => monitorStatus(),

  // ── Models ──
  GetModels: () => models.map((m) => ({ ...m })),
  RefreshModels: async () => {
    await sleep(300)
    return models.map((m) => ({ ...m }))
  },
  GetModelConfig: (modelID: string) => ({ ...defaultModelConfig, ...(modelConfigs[modelID] ?? {}) }),
  SaveModelConfig: (modelID: string, cfg: any) => {
    modelConfigs[modelID] = { ...defaultModelConfig, ...modelConfigs[modelID], ...cfg }
  },
  TuneModelConfig: (modelID: string) => {
    const tuned = {
      ...defaultModelConfig,
      threads: 8,
      gpuLayers: '999',
      ctxSize: 8192,
      batchSize: 512,
      ubatchSize: 256,
      flashAttn: true,
      cacheTypeK: 'q8_0',
      cacheTypeV: 'q8_0',
      cpuMoe: true,
      loadMode: 'mmap',
    }
    modelConfigs[modelID] = tuned
    return { ...tuned }
  },
  BenchmarkModel: async () => {
    await sleep(2000)
    return { tgTps: 8.4, ngl: '999', threads: 0, usedCpuMoe: true, elapsedS: 74.6 }
  },

  // ── Server ──
  GetServerConfig: () => ({ ...serverConfig }),
  SaveServerConfig: (cfg: any) => {
    Object.assign(serverConfig, cfg)
  },
  GetServerStatus: () => ({ running: serverRunning, log: serverLogRing.map((e) => e.text) }),
  GetServerLogsSince: (since: number) => ({
    entries: serverLogRing.filter((e) => e.seq >= since).map((e) => ({ ...e })),
    next: logSeq + 1,
  }),
  StartServer: () => {
    if (serverRunning) return
    serverRunning = true
    serverStartedAt = Date.now()
    loadedModels = [{ id: residentModel, type: 'chat', status: 'loaded' }]
    startServerLogs()
  },
  StartServerWithModel: (model: string) => {
    if (!serverRunning) {
      serverRunning = true
      serverStartedAt = Date.now()
      residentModel = model
      loadedModels = [{ id: model, type: 'chat', status: 'loaded' }]
      addServerLog('[INFO] direct mode: starting llama-server for model "' + model + '"')
      startServerLogs()
      return
    }
    if (residentModel === model) {
      addServerLog('[INFO] direct mode: model "' + model + '" already resident, start is a no-op')
      return
    }
    addServerLog('[INFO] direct mode: switching model "' + residentModel + '" -> "' + model + '", restarting llama-server')
    residentModel = model
    loadedModels = [{ id: model, type: 'chat', status: 'loaded' }]
    serverStartedAt = Date.now()
    startServerLogs()
  },
  StopServer: () => {
    if (!serverRunning) return
    addServerLog('[INFO] Stopping llama-server...')
    serverRunning = false
    loadedModels = []
    addServerLog('[INFO] llama-server stopped')
  },

  // ── llama.cpp runtime download ──
  GetLlamaCppDownloadStatus: () => llamacppState(),
  StartLlamaCppDownload: () => {
    llamacppDl.status = 'downloading'
    llamacppDl.paused = false
    llamacppDl.total = 58_000_000
    llamacppDl.base = 0
    llamacppDl.since = Date.now()
    llamacppDl.version = 'b5913'
    llamacppDl.error = ''
  },
  PauseLlamaCppDownload: () => {
    if (llamacppDl.status !== 'downloading') return
    llamacppDl.base = llamacppDownloaded()
    llamacppDl.status = 'paused'
    llamacppDl.paused = true
  },
  ResumeLlamaCppDownload: () => {
    if (llamacppDl.status !== 'paused') return
    llamacppDl.status = 'downloading'
    llamacppDl.paused = false
    llamacppDl.since = Date.now()
  },
  StopLlamaCppDownload: () => {
    if (llamacppDl.status !== 'downloading' && llamacppDl.status !== 'paused') return
    llamacppDl.status = 'cancelled'
    llamacppDl.paused = false
  },

  // ── Directory pickers (resolve empty = cancelled dialog) ──
  BrowseLlamaCppDir: async () => {
    await sleep(300)
    return ''
  },
  BrowseLlamaCppDownloadDir: async () => {
    await sleep(300)
    return ''
  },
  BrowseModelsDir: async () => {
    await sleep(300)
    return ''
  },
  BrowseModelDownloadDir: async () => {
    await sleep(300)
    return ''
  },

  // ── App update ──
  GetAppVersion: () => 'v0.3.9-mock',
  CheckForUpdate: () => ({ hasUpdate: false, version: 'v0.3.9-mock', notes: '', published: '' }),
  StartUpdateDownload: () => {},
  GetUpdateDownloadStatus: () => ({
    status: 'idle',
    progress: 0,
    total: 0,
    downloaded: 0,
    version: '',
    filePath: '',
    error: '',
    kind: 'setup',
    installer: false,
  }),
  StopUpdateDownload: () => {},
  InstallUpdate: () => {},

  // ── Downloads (search / files / tasks) ──
  SearchDownloads: async (query: string, filter: string) => {
    await sleep(600)
    void query
    return searchCatalog.map((entry) => ({
      ...entry,
      tags: [...entry.tags, ...(filter ? [filter] : [])],
      siblings: entry.siblings.map((s: any) => ({ ...s })),
    }))
  },
  GetModelFiles: async (modelID: string) => {
    await sleep(250)
    const entry = catalogEntry(modelID)
    if (entry) return entry.siblings.map((s: any) => ({ filename: s.rfilename, size: s.size }))
    const base = modelID.split('/').pop() ?? modelID
    return [
      { filename: base + '-Q4_K_M.gguf', size: 4_690_000_000 },
      { filename: base + '-Q8_0.gguf', size: 7_650_000_000 },
      { filename: 'README.md', size: 12_000 },
    ]
  },
  GetModelMaxFileSize: (modelID: string) => {
    const entry = catalogEntry(modelID)
    if (!entry) return 4_690_000_000
    return entry.siblings.reduce((max: number, s: any) => Math.max(max, s.size), 0)
  },
  GetModelDescription: () =>
    'Mock preview model card: a compact instruction-tuned LLM distilled for on-device inference. ' +
    'Quantized GGUF weights trade a little accuracy for a much smaller memory footprint, ' +
    'so mid-range phones can serve it through llama-server.',
  StartDownload: (modelID: string, files: string[]) => {
    if (!files || files.length === 0) {
      throw new Error('no files selected for download')
    }
    const entry = catalogEntry(modelID)
    const ggufs = entry ? entry.siblings.filter((s: any) => s.rfilename.endsWith('.gguf')) : []
    const total =
      ggufs.find((s: any) => files.includes(s.rfilename))?.size ??
      files.length * 4_690_000_000
    const first = files[0]
    tasks.push({
      id: 'task-' + Date.now(),
      modelId: modelID,
      fileName: first,
      destDir: config.modelDownloadDir,
      source: config.downloadSource,
      status: 'queued',
      total,
      base: 0,
      since: 0,
      speed: 8_200_000,
      error: '',
      queuedAt: Date.now(),
    })
  },
  GetDownloadTasks: () => tasks.map(taskJson),
  CancelDownloadTask: (id: string) => {
    const t = tasks.find((x) => x.id === id)
    if (!t) return
    if (t.status === 'downloading') t.base = taskDownloaded(t)
    t.status = 'cancelled'
    t.speed = 0
  },
  RetryDownloadTask: (id: string) => {
    const t = tasks.find((x) => x.id === id)
    if (!t) return
    if (t.status === 'downloading' || t.status === 'paused') return
    t.status = 'downloading'
    t.base = 0
    t.since = Date.now()
    t.error = ''
  },
  PauseDownloadTask: (id: string) => {
    const t = tasks.find((x) => x.id === id)
    if (!t || t.status !== 'downloading') return
    t.base = taskDownloaded(t)
    t.status = 'paused'
  },
  ResumeDownloadTask: (id: string) => {
    const t = tasks.find((x) => x.id === id)
    if (!t || t.status !== 'paused') return
    t.status = 'downloading'
    t.since = Date.now()
  },

  // ── Download source ──
  GetDownloadSource: () => config.downloadSource,
  SetDownloadSource: (source: string) => {
    if (source !== 'hf' && source !== 'huggingface' && source !== 'modelscope') {
      throw new Error('invalid download source: ' + source)
    }
    config.downloadSource = source
  },

  // ── Router models (TaskDock in-memory list) ──
  GetLoadedModels: () => (serverRunning ? loadedModels.map((m) => ({ ...m })) : []),
  UnloadModel: (id: string) => {
    if (!serverRunning) {
      throw new Error('server is not running')
    }
    if (!id) {
      throw new Error('invalid model id')
    }
    loadedModels = loadedModels.filter((m) => m.id !== id)
    addServerLog('[INFO] unloading model "' + id + '"')
  },

  // ── Remote docs (degrade to the bundled tutorial) ──
  GetRemoteDoc: () => ({ text: '', source: 'none', fetchedAt: '' }),

  // ── Directory setters (frontend callers exist only via future paths; cover anyway) ──
  SetLlamaCppDownloadDir: (dir: string) => {
    if (!dir) throw new Error('empty directory')
    config.llamaCppDownloadDir = dir
  },
  SetModelDownloadDir: (dir: string) => {
    if (!dir) throw new Error('empty directory')
    config.modelDownloadDir = dir
  },
  SetModelsDir: (dir: string) => {
    if (!dir) throw new Error('empty directory')
    config.modelsDir = dir
  },
}

/** Current mock llama-server port (the fetch interceptor matches on this). */
export function mockServerPort(): number {
  return serverConfig.port
}

/** Whether the mock llama-server is running (drives the fetch mock behavior). */
export function mockServerRunning(): boolean {
  return serverRunning
}

/** Loaded chat models, shared with the fetch interceptor's /models listing. */
export function mockLoadedModels(): { id: string; status: string }[] {
  return loadedModels.map((m) => ({ id: m.id, status: m.status }))
}

/** Register one mock-produced server log line (used by the chat fetch mock). */
export function mockAddServerLog(text: string): void {
  addServerLog(text)
}

/** The canned streamed chat reply (used by the chat fetch mock). */
export function mockChatReply(): string {
  return chatReply
}
