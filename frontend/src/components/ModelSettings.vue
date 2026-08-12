<template>
  <div v-if="visible" class="modal-overlay" @click.self="close" @keydown.esc="close">
    <div
      class="modal"
      role="dialog"
      aria-modal="true"
      aria-labelledby="model-settings-title"
      @keydown.tab="handleTab"
    >
      <div class="modal-header">
        <h2 id="model-settings-title">模型设置</h2>
        <span class="modal-model">{{ modelName }}</span>
        <button ref="closeBtn" class="modal-close" aria-label="关闭" @click="close">✕</button>
      </div>

      <div class="modal-body">
        <!-- Performance -->
        <fieldset class="param-group">
          <legend>性能</legend>
          <div class="param-row">
            <label>CPU 线程 (-t)</label>
            <input v-model.number="cfg.threads" type="number" min="-1" step="1" class="param-input param-num" placeholder="-1 (自动)" />
          </div>
          <div class="param-row">
            <label>GPU 层数 (-ngl)</label>
            <select v-model="cfg.gpuLayers" class="param-input">
              <option value="auto">auto (自动)</option>
              <option value="all">all (全部)</option>
              <option value="0">0 (仅 CPU)</option>
              <option value="10">10</option>
              <option value="20">20</option>
              <option value="30">30</option>
              <option value="40">40</option>
              <option value="50">50</option>
              <option value="99">99</option>
            </select>
          </div>
          <div class="param-row">
            <label>上下文大小 (-c)</label>
            <input v-model.number="cfg.ctxSize" type="number" min="1" step="1" class="param-input param-num" placeholder="4096" />
          </div>
          <div class="param-row">
            <label>Batch 大小 (-b)</label>
            <input v-model.number="cfg.batchSize" type="number" min="1" step="1" class="param-input param-num" placeholder="2048" />
          </div>
          <div class="param-row">
            <label>μBatch 大小 (-ub)</label>
            <input v-model.number="cfg.ubatchSize" type="number" min="1" step="1" class="param-input param-num" placeholder="512" />
          </div>
          <label class="param-check">
            <input type="checkbox" v-model="cfg.flashAttn" />
            <span>Flash Attention (-fa) <em>加速推理</em></span>
          </label>
          <label class="param-check">
            <input type="checkbox" v-model="cfg.cpuMoe" />
            <span>cpu-moe <em>MoE 模型(DeepSeek/Qwen3-MoE)显存不够时,专家层留 CPU、共享层上 GPU</em></span>
          </label>
          <div class="param-row">
            <label>MoE CPU 层数 (-n-cpu-moe)</label>
            <input v-model.number="cfg.nCpuMoe" type="number" min="0" step="1" class="param-input param-num" placeholder="0 (不启用)" />
          </div>
        </fieldset>

        <!-- Memory / Load -->
        <fieldset class="param-group">
          <legend>内存 / 加载</legend>
          <div class="param-hint">KV 缓存量化：<em>q8_0 推荐(省显存几乎无损)；q4_0 最省但质量降；f16/f32 高精度占显存</em></div>
          <div class="param-row">
            <label>KV 缓存 K 类型</label>
            <select v-model="cfg.cacheTypeK" class="param-input">
              <option value="">默认 (f16)</option>
              <option value="f32">f32</option>
              <option value="f16">f16</option>
              <option value="bf16">bf16</option>
              <option value="q8_0">q8_0 (推荐)</option>
              <option value="q4_0">q4_0</option>
              <option value="q4_1">q4_1</option>
              <option value="iq4_nl">iq4_nl</option>
              <option value="q5_0">q5_0</option>
              <option value="q5_1">q5_1</option>
            </select>
          </div>
          <div class="param-row">
            <label>KV 缓存 V 类型</label>
            <select v-model="cfg.cacheTypeV" class="param-input">
              <option value="">默认 (f16)</option>
              <option value="f32">f32</option>
              <option value="f16">f16</option>
              <option value="bf16">bf16</option>
              <option value="q8_0">q8_0 (推荐)</option>
              <option value="q4_0">q4_0</option>
              <option value="q4_1">q4_1</option>
              <option value="iq4_nl">iq4_nl</option>
              <option value="q5_0">q5_0</option>
              <option value="q5_1">q5_1</option>
            </select>
          </div>
          <div class="param-row">
            <label>加载方式</label>
            <select v-model="cfg.loadMode" class="param-input">
              <option value="">默认 mmap</option>
              <option value="mmap">mmap 内存映射(快加载)</option>
              <option value="mlock">mlock 锁内存防 swap</option>
              <option value="mmap+mlock">mmap+mlock 映射+锁定</option>
              <option value="none">none 不映射(慢加载少换页)</option>
              <option value="dio">dio DirectIO</option>
            </select>
          </div>
          <div class="param-hint"><em>内存够选 mlock 更稳(防换页)；默认 mmap 加载快</em></div>
        </fieldset>

        <!-- Multi GPU -->
        <fieldset class="param-group">
          <legend>多 GPU</legend>
          <div class="param-row">
            <label>切分方式</label>
            <select v-model="cfg.splitMode" class="param-input">
              <option value="">默认 layer</option>
              <option value="layer">layer 流水线(默认稳)</option>
              <option value="row">row 并行(更快兼容性差)</option>
              <option value="tensor">tensor 实验性</option>
              <option value="none">none 单卡</option>
            </select>
          </div>
          <div class="param-row">
            <label>每卡张量比例</label>
            <input v-model="cfg.tensorSplit" type="text" class="param-input" placeholder="如 3,1" />
          </div>
          <div class="param-hint"><em>每卡分担比例,如 3,1=第一卡 3/4</em></div>
          <div class="param-row">
            <label>主 GPU</label>
            <input v-model.number="cfg.mainGpu" type="number" min="0" step="1" class="param-input param-num" placeholder="0" />
          </div>
          <div class="param-hint"><em>split-mode=none 时的唯一 GPU</em></div>
        </fieldset>

        <!-- Long context -->
        <fieldset class="param-group">
          <legend>长上下文</legend>
          <div class="param-row">
            <label>RoPE 外推方式</label>
            <select v-model="cfg.ropeScaling" class="param-input">
              <option value="">默认 none</option>
              <option value="none">none</option>
              <option value="linear">linear 线性</option>
              <option value="yarn">yarn YaRN(效果较好)</option>
            </select>
          </div>
          <div class="param-row">
            <label>外推倍数</label>
            <input v-model.number="cfg.ropeScale" type="number" min="0" step="0.5" class="param-input param-num" placeholder="0" />
          </div>
          <div class="param-hint"><em>扩展倍数,2.0=翻倍;超过模型原生上下文才用,越大质量越差</em></div>
        </fieldset>
      </div>

      <div class="modal-footer">
        <span class="footer-msg" v-if="saveSuccess">✓ 已保存</span>
        <span class="footer-msg footer-err" v-else-if="saveError">{{ saveError }}</span>
        <button class="btn-cancel" @click="close">取消</button>
        <button class="btn-save" :disabled="saving" @click="save">
          {{ saving ? '保存中...' : '保存设置' }}
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { reactive, ref, watch, nextTick } from 'vue'

export interface ModelConfig {
  threads: number
  gpuLayers: string
  ctxSize: number
  batchSize: number
  ubatchSize: number
  flashAttn: boolean
  cacheTypeK: string
  cacheTypeV: string
  loadMode: string
  cpuMoe: boolean
  nCpuMoe: number
  splitMode: string
  tensorSplit: string
  mainGpu: number
  ropeScaling: string
  ropeScale: number
}

const props = defineProps<{
  visible: boolean
  modelName: string
  initialConfig?: Partial<ModelConfig>
  saving?: boolean
  saveError?: string
  saveSuccess?: boolean
}>()

const emit = defineEmits<{
  close: []
  save: [config: ModelConfig]
}>()

const defaults: ModelConfig = {
  threads: -1,
  gpuLayers: 'auto',
  ctxSize: 4096,
  batchSize: 2048,
  ubatchSize: 512,
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
}

const cfg = reactive<ModelConfig>({ ...defaults, ...props.initialConfig })

const closeBtn = ref<HTMLButtonElement | null>(null)

watch(() => props.visible, (v) => {
  if (v) {
    Object.assign(cfg, defaults, props.initialConfig)
    // 弹窗打开后把焦点移入弹窗，配合下方 Tab 焦点陷阱（#17）
    nextTick(() => closeBtn.value?.focus())
  }
})

/** 焦点陷阱：Tab / Shift+Tab 在弹窗内可聚焦元素间循环，防止焦点逃逸到页面背景（#17） */
function handleTab(e: KeyboardEvent) {
  const modalEl = e.currentTarget as HTMLElement
  const focusable = Array.from(
    modalEl.querySelectorAll<HTMLElement>(
      'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])'
    )
  ).filter(el => !el.hasAttribute('disabled'))
  if (focusable.length === 0) return
  const first = focusable[0]
  const last = focusable[focusable.length - 1]
  const active = document.activeElement
  if (e.shiftKey) {
    if (active === first || !modalEl.contains(active)) {
      e.preventDefault()
      last.focus()
    }
  } else if (active === last) {
    e.preventDefault()
    first.focus()
  }
}

function close() {
  emit('close')
}

function save() {
  emit('save', { ...cfg })
}
</script>

<style scoped>
.modal-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0,0,0,0.6);
  backdrop-filter: blur(4px);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
}

.modal {
  background: var(--bg-secondary);
  border: 1px solid var(--border);
  border-radius: 16px;
  width: 560px;
  max-height: 80vh;
  display: flex;
  flex-direction: column;
  box-shadow: 0 20px 60px rgba(0,0,0,0.5);
}

.modal-header {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 20px 24px;
  border-bottom: 1px solid var(--border);
}

.modal-header h2 {
  font-size: 17px;
  font-weight: 700;
  color: var(--text-primary);
  margin: 0;
}

.modal-model {
  font-size: 12px;
  color: var(--text-dim);
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.modal-close {
  width: 28px;
  height: 28px;
  border: none;
  background: none;
  color: var(--text-dim);
  font-size: 16px;
  cursor: pointer;
  border-radius: 6px;
  transition: all 0.15s;
}

.modal-close:hover {
  background: var(--hover-bg);
  color: var(--text-primary);
}

.modal-body {
  padding: 20px 24px;
  overflow-y: auto;
  flex: 1;
}

.param-group {
  border: 1px solid var(--border-light);
  border-radius: 10px;
  padding: 14px 16px;
  margin-bottom: 14px;
}

.param-group legend {
  font-size: 12px;
  font-weight: 700;
  color: var(--accent-light);
  padding: 0 6px;
  text-transform: uppercase;
  letter-spacing: 0.5px;
}

.param-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 10px;
}

.param-row label {
  font-size: 13px;
  color: var(--text-secondary);
  flex-shrink: 0;
}

.param-input {
  width: 180px;
  padding: 6px 10px;
  background: var(--bg-primary);
  border: 1px solid var(--border);
  border-radius: 6px;
  color: var(--text-primary);
  font-size: 13px;
  outline: none;
  transition: border-color 0.15s;
}

.param-input:focus {
  border-color: var(--accent);
}

.param-num {
  width: 100px;
}

.param-check {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  color: var(--text-secondary);
  cursor: pointer;
  margin-top: 4px;
}

.param-check input {
  accent-color: var(--accent);
}

.param-check em {
  color: var(--text-dim);
  font-style: normal;
  font-size: 11px;
}

.param-hint {
  font-size: 11px;
  color: var(--text-dim);
  margin-bottom: 10px;
  line-height: 1.5;
}

.param-hint em {
  font-style: normal;
}

.modal-footer {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  padding: 16px 24px;
  border-top: 1px solid var(--border);
}

.btn-cancel {
  padding: 8px 20px;
  background: transparent;
  border: 1px solid var(--border);
  border-radius: 8px;
  color: var(--text-muted);
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.15s;
}

.btn-cancel:hover {
  background: var(--hover-bg);
}

.btn-save {
  padding: 8px 24px;
  background: var(--accent);
  border: none;
  border-radius: 8px;
  color: #fff;
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
  transition: opacity 0.15s;
}

.btn-save:hover:not(:disabled) {
  opacity: 0.85;
}

.btn-save:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.footer-msg {
  font-size: 13px;
  font-weight: 600;
  color: var(--success);
  margin-right: auto;
}

.footer-err {
  color: #ef4444;
}
</style>
