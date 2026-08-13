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

      <div class="modal-tabs" role="tablist" aria-label="参数分类">
        <button
          v-for="(tab, i) in tabs"
          :key="tab.id"
          :id="`${tab.id}-tab`"
          class="tab-btn"
          :class="{ active: activeTab === i }"
          role="tab"
          :aria-selected="activeTab === i"
          :aria-controls="tab.id"
          @click="activeTab = i"
        >
          <span class="tab-icon" v-html="tab.icon"></span>
          {{ tab.label }}
        </button>
      </div>

      <div class="modal-body">
        <!-- 基础 -->
        <div id="tab-base" role="tabpanel" aria-labelledby="tab-base-tab" v-show="activeTab === 0">
          <div class="param-group">
            <h3 class="group-title">基础</h3>
            <div class="param-grid">
              <div class="param">
                <label class="param-field">
                  <span class="param-label">CPU 线程</span>
                  <input v-model.number="cfg.threads" type="number" min="-1" step="1" class="param-input" placeholder="-1 自动" />
                </label>
                <p class="param-hint">-1 自动;CPU 弱可适当调低,发热/占用明显时减少</p>
              </div>
              <div class="param">
                <label class="param-field">
                  <span class="param-label">GPU 层数</span>
                  <select v-model="cfg.gpuLayers" class="param-input">
                    <option value="auto">auto ★ 推荐</option>
                    <option value="all">all (全部)</option>
                    <option value="0">0 (仅 CPU)</option>
                    <option value="10">10</option>
                    <option value="20">20</option>
                    <option value="30">30</option>
                    <option value="40">40</option>
                    <option value="50">50</option>
                    <option value="99">99</option>
                  </select>
                </label>
                <p class="param-hint">auto 自动卸载全部层;显存小(4-8GB)选 10~30;纯 CPU 推理选 0</p>
              </div>
              <div class="param">
                <label class="param-field">
                  <span class="param-label">上下文大小 (tokens)</span>
                  <input v-model.number="cfg.ctxSize" type="number" min="1" step="1" class="param-input" placeholder="4096" />
                </label>
                <p class="param-hint">对话/文档越长设越大;受显存限制,过大会加载失败</p>
              </div>
              <div class="param">
                <label class="param-field">
                  <span class="param-label">Batch 大小 (tokens)</span>
                  <input v-model.number="cfg.batchSize" type="number" min="1" step="1" class="param-input" placeholder="2048" />
                </label>
                <p class="param-hint">吞吐优先可加大,默认 2048 稳妥</p>
              </div>
              <div class="param">
                <label class="param-field">
                  <span class="param-label">μBatch 大小 (tokens)</span>
                  <input v-model.number="cfg.ubatchSize" type="number" min="1" step="1" class="param-input" placeholder="512" />
                </label>
                <p class="param-hint">通常不超过 Batch,默认 512</p>
              </div>
            </div>
          </div>
        </div>

        <!-- 推理 -->
        <div id="tab-infer" role="tabpanel" aria-labelledby="tab-infer-tab" v-show="activeTab === 1">
          <div class="param-group">
            <h3 class="group-title">推理</h3>
            <div class="param-grid col-1">
              <div class="param">
                <div class="toggle-row">
                  <span class="toggle-text">
                    Flash Attention
                    <span class="toggle-sub">加速推理,推荐开启</span>
                  </span>
                  <label class="switch">
                    <input type="checkbox" v-model="cfg.flashAttn" aria-label="Flash Attention" />
                    <span class="slider"></span>
                  </label>
                </div>
                <p class="param-hint">加速推理并省显存,推荐开启</p>
              </div>
              <div class="param">
                <div class="toggle-row">
                  <span class="toggle-text">
                    cpu-moe
                    <span class="toggle-sub">显存不足时启用</span>
                  </span>
                  <label class="switch">
                    <input type="checkbox" v-model="cfg.cpuMoe" aria-label="cpu-moe" />
                    <span class="slider"></span>
                  </label>
                </div>
                <p class="param-hint">MoE 模型(DeepSeek/Qwen3-MoE)显存不足时,专家层留 CPU、共享层上 GPU</p>
              </div>
              <div class="param">
                <label class="param-field">
                  <span class="param-label">MoE CPU 层数</span>
                  <input v-model.number="cfg.nCpuMoe" type="number" min="0" step="1" class="param-input" placeholder="0 不启用" />
                </label>
                <p class="param-hint">前 N 层 MoE 专家留 CPU,0 不启用;显存不足时逐步加大</p>
              </div>
            </div>
          </div>
        </div>

        <!-- 内存 / 加载 -->
        <div id="tab-memory" role="tabpanel" aria-labelledby="tab-memory-tab" v-show="activeTab === 2">
          <div class="param-group">
            <h3 class="group-title">内存 / 加载</h3>
            <div class="param-grid col-1">
              <div class="param">
                <label class="param-field">
                  <span class="param-label">KV 缓存 K 类型</span>
                  <select v-model="cfg.cacheTypeK" class="param-input">
                    <option value="">默认 (f16)</option>
                    <option value="f32">f32</option>
                    <option value="f16">f16</option>
                    <option value="bf16">bf16</option>
                    <option value="q8_0">q8_0 ★ 推荐</option>
                    <option value="q4_0">q4_0</option>
                    <option value="q4_1">q4_1</option>
                    <option value="iq4_nl">iq4_nl</option>
                    <option value="q5_0">q5_0</option>
                    <option value="q5_1">q5_1</option>
                  </select>
                </label>
                <p class="param-hint">q8_0 推荐(省显存几乎无损);f32/f16 精度高占显存;q4 系最省</p>
              </div>
              <div class="param">
                <label class="param-field">
                  <span class="param-label">KV 缓存 V 类型</span>
                  <select v-model="cfg.cacheTypeV" class="param-input">
                    <option value="">默认 (f16)</option>
                    <option value="f32">f32</option>
                    <option value="f16">f16</option>
                    <option value="bf16">bf16</option>
                    <option value="q8_0">q8_0 ★ 推荐</option>
                    <option value="q4_0">q4_0</option>
                    <option value="q4_1">q4_1</option>
                    <option value="iq4_nl">iq4_nl</option>
                    <option value="q5_0">q5_0</option>
                    <option value="q5_1">q5_1</option>
                  </select>
                </label>
                <p class="param-hint">同 K 类型,一般跟随 K 保持一致</p>
              </div>
              <div class="param">
                <label class="param-field">
                  <span class="param-label">加载方式</span>
                  <select v-model="cfg.loadMode" class="param-input">
                    <option value="">默认 mmap</option>
                    <option value="mmap">mmap 内存映射(快加载)</option>
                    <option value="mlock">mlock 锁内存防 swap</option>
                    <option value="mmap+mlock">mmap+mlock 映射+锁定</option>
                    <option value="none">none 不映射(慢加载少换页)</option>
                    <option value="dio">dio DirectIO</option>
                  </select>
                </label>
                <p class="param-hint">内存够用选 mlock 防换页更稳;默认 mmap 加载最快</p>
              </div>
            </div>
          </div>
        </div>

        <!-- 多 GPU -->
        <div id="tab-gpu" role="tabpanel" aria-labelledby="tab-gpu-tab" v-show="activeTab === 3">
          <div class="param-group">
            <h3 class="group-title">多 GPU</h3>
            <div class="param-grid col-1">
              <div class="param">
                <label class="param-field">
                  <span class="param-label">切分方式</span>
                  <select v-model="cfg.splitMode" class="param-input">
                    <option value="">默认 layer</option>
                    <option value="layer">layer 流水线(默认稳)</option>
                    <option value="row">row 并行(更快兼容性差)</option>
                    <option value="tensor">tensor 实验性</option>
                    <option value="none">none 单卡</option>
                  </select>
                </label>
                <p class="param-hint">单卡选 none;多卡默认 layer 稳定,row 更快但兼容性差</p>
              </div>
              <div class="param">
                <label class="param-field">
                  <span class="param-label">每卡比例</span>
                  <input v-model="cfg.tensorSplit" type="text" class="param-input" placeholder="如 3,1" />
                </label>
                <p class="param-hint">每张卡分担的比例,如 3,1 = 第一卡 3/4、第二卡 1/4</p>
              </div>
              <div class="param">
                <label class="param-field">
                  <span class="param-label">主 GPU</span>
                  <input v-model.number="cfg.mainGpu" type="number" min="0" step="1" class="param-input" placeholder="0" />
                </label>
                <p class="param-hint">split-mode=none 时唯一使用的 GPU;多卡时承载 KV 与中间结果</p>
              </div>
            </div>
          </div>
        </div>

        <!-- 长上下文 -->
        <div id="tab-context" role="tabpanel" aria-labelledby="tab-context-tab" v-show="activeTab === 4">
          <div class="param-group">
            <h3 class="group-title">长上下文</h3>
            <div class="param-grid col-1">
              <div class="param">
                <label class="param-field">
                  <span class="param-label">RoPE 外推方式</span>
                  <select v-model="cfg.ropeScaling" class="param-input">
                    <option value="">默认 none</option>
                    <option value="none">none</option>
                    <option value="linear">linear 线性</option>
                    <option value="yarn">yarn YaRN(效果较好)</option>
                  </select>
                </label>
                <p class="param-hint">超过模型原生上下文才需要;yarn 效果较好、linear 更简单</p>
              </div>
              <div class="param">
                <label class="param-field">
                  <span class="param-label">外推倍数</span>
                  <input v-model.number="cfg.ropeScale" type="number" min="0" step="0.5" class="param-input" placeholder="0" />
                </label>
                <p class="param-hint">2.0 = 上下文翻倍;倍数越大质量损失越明显,够用即可</p>
              </div>
            </div>
          </div>
        </div>

        <!-- 高级 -->
        <div id="tab-advanced" role="tabpanel" aria-labelledby="tab-advanced-tab" v-show="activeTab === 5">
          <div class="param-group">
            <h3 class="group-title">高级</h3>
            <div class="param-grid col-1">
              <div class="param">
                <label class="param-field">
                  <span class="param-label">视觉投影文件 (mmproj)</span>
                  <input v-model="cfg.mmproj" type="text" class="param-input" placeholder="留空自动查找" />
                </label>
                <p class="param-hint">留空时自动使用模型同目录下的 mmproj 文件;填写则显式指定视觉模型路径</p>
              </div>
              <div class="param">
                <div class="toggle-row">
                  <span class="toggle-text">
                    Reasoning 推理输出
                    <span class="toggle-sub">关闭模型思考输出</span>
                  </span>
                  <label class="switch">
                    <input type="checkbox" v-model="cfg.reasoning" aria-label="Reasoning" />
                    <span class="slider"></span>
                  </label>
                </div>
                <p class="param-hint">开启后以 --reasoning off 关闭模型思考输出</p>
              </div>
              <div class="param">
                <label class="param-field">
                  <span class="param-label">投机解码 (MTP)</span>
                  <select v-model="cfg.specType" class="param-input">
                    <option value="">关闭</option>
                    <option value="draft-mtp">draft-mtp</option>
                  </select>
                </label>
                <p class="param-hint">MTP 投机预测,需模型支持</p>
              </div>
              <div class="param">
                <label class="param-field">
                  <span class="param-label">额外预测 token 数</span>
                  <input v-model.number="cfg.specDraftNMax" type="number" min="0" step="1" class="param-input" placeholder="0" />
                </label>
                <p class="param-hint">额外预测 token 数,>0 生效</p>
              </div>
            </div>
          </div>
        </div>
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
  mmproj: string
  reasoning: boolean
  specType: string
  specDraftNMax: number
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
  mmproj: '',
  reasoning: false,
  specType: '',
  specDraftNMax: 0,
}

const cfg = reactive<ModelConfig>({ ...defaults, ...props.initialConfig })

const closeBtn = ref<HTMLButtonElement | null>(null)

/** 当前激活的 tab 索引，配合 v-show 切换面板（不丢已输入未保存的值） */
const activeTab = ref(0)

/** 6 个参数分类 tab，图标为内联 stroke SVG（16×16，风格与 Sidebar 导航一致） */
const tabs = [
  {
    id: 'tab-base',
    label: '基础',
    icon: `<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-2 2 2 2 0 0 1-2-2v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1-2-2 2 2 0 0 1 2-2h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 2-2 2 2 0 0 1 2 2v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 2 2 2 2 0 0 1-2 2h-.09a1.65 1.65 0 0 0-1.51 1z"/></svg>`,
  },
  {
    id: 'tab-infer',
    label: '推理',
    icon: `<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polygon points="13 2 3 14 12 14 11 22 21 10 12 10 13 2"/></svg>`,
  },
  {
    id: 'tab-memory',
    label: '内存/加载',
    icon: `<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 16V8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16z"/><polyline points="3.27 6.96 12 12.01 20.73 6.96"/><line x1="12" y1="22.08" x2="12" y2="12"/></svg>`,
  },
  {
    id: 'tab-gpu',
    label: '多 GPU',
    icon: `<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polygon points="12 2 2 7 12 12 22 7 12 2"/><polyline points="2 17 12 22 22 17"/><polyline points="2 12 12 17 22 12"/></svg>`,
  },
  {
    id: 'tab-context',
    label: '长上下文',
    icon: `<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="2" y="9" width="20" height="6" rx="1"/><line x1="6" y1="9" x2="6" y2="12"/><line x1="10" y1="9" x2="10" y2="13"/><line x1="14" y1="9" x2="14" y2="12"/><line x1="18" y1="9" x2="18" y2="13"/></svg>`,
  },
  {
    id: 'tab-advanced',
    label: '高级',
    icon: `<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M14.7 6.3a1 1 0 0 0 0 1.4l1.6 1.6a1 1 0 0 0 1.4 0l3.77-3.77a6 6 0 0 1-7.94 7.94l-6.91 6.91a2.12 2.12 0 0 1-3-3l6.91-6.91a6 6 0 0 1 7.94-7.94l-3.76 3.76z"/></svg>`,
  },
]

watch(() => props.visible, (v) => {
  if (v) {
    Object.assign(cfg, defaults, props.initialConfig)
    // 重新打开弹窗时回到第一个 tab，配合下方 Tab 焦点陷阱（#17）
    activeTab.value = 0
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
  border-radius: var(--radius-lg);
  width: 640px;
  max-height: 80vh;
  display: flex;
  flex-direction: column;
  box-shadow: 0 20px 60px rgba(0,0,0,0.5);
}

.modal-header {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 18px 24px;
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
  border-radius: var(--radius-sm);
  transition: all 0.15s;
}

.modal-close:hover {
  background: var(--hover-bg);
  color: var(--text-primary);
}

/* Tab 栏（下划线式） */
.modal-tabs {
  display: flex;
  gap: 4px;
  padding: 0 24px;
  border-bottom: 1px solid var(--border);
  flex-shrink: 0;
}

.tab-btn {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 10px 14px;
  background: none;
  border: none;
  border-bottom: 2px solid transparent;
  margin-bottom: -1px;
  color: var(--text-muted);
  font-size: 13px;
  font-weight: 500;
  font-family: inherit;
  cursor: pointer;
  transition: color 0.15s, border-color 0.15s;
}

.tab-btn:hover {
  color: var(--text-secondary);
}

.tab-btn.active {
  color: var(--accent);
  border-bottom-color: var(--accent);
  font-weight: 600;
}

.tab-btn .tab-icon {
  display: flex;
}

.modal-body {
  padding: 20px 24px;
  overflow-y: auto;
  flex: 1;
}

/* 参数卡片 */
.param-group {
  background: var(--bg-card);
  border: 1px solid var(--border-light);
  border-radius: var(--radius-md);
  padding: 16px 18px;
}

.group-title {
  font-size: 13px;
  font-weight: 700;
  color: var(--text-secondary);
  margin: 0 0 14px;
}

.param-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 14px 18px;
}

.param-grid.col-1 {
  grid-template-columns: 1fr;
}

.param-field {
  display: block;
}

.param-label {
  display: block;
  font-size: 13px;
  font-weight: 600;
  color: var(--text-secondary);
  margin-bottom: 6px;
  cursor: pointer;
}

.param-input {
  width: 100%;
  padding: 8px 10px;
  background: var(--bg-primary);
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
  color: var(--text-primary);
  font-size: 13px;
  font-family: inherit;
  outline: none;
  transition: border-color 0.15s, box-shadow 0.15s;
}

.param-input:hover {
  border-color: var(--overlay-20);
}

.param-input:focus {
  border-color: var(--accent);
  box-shadow: 0 0 0 3px var(--accent-glow);
}

.param-input::placeholder {
  color: var(--text-dim);
}

select.param-input {
  cursor: pointer;
}

.param-hint {
  margin-top: 6px;
  font-size: 11px;
  color: var(--text-dim);
  line-height: 1.5;
}

/* 胶囊开关（toggle switch） */
.toggle-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 12px 14px;
  background: var(--surface);
  border: 1px solid var(--border-light);
  border-radius: var(--radius-sm);
}

.toggle-text {
  display: flex;
  flex-direction: column;
  gap: 2px;
  font-size: 13px;
  font-weight: 600;
  color: var(--text-secondary);
}

.toggle-sub {
  font-size: 11px;
  font-weight: 400;
  color: var(--text-dim);
  line-height: 1.5;
}

.switch {
  position: relative;
  display: inline-block;
  width: 40px;
  height: 22px;
  flex-shrink: 0;
}

.switch input {
  opacity: 0;
  width: 0;
  height: 0;
}

.slider {
  position: absolute;
  cursor: pointer;
  inset: 0;
  background: var(--overlay-20);
  border-radius: 22px;
  transition: background 0.2s;
}

.slider:before {
  content: "";
  position: absolute;
  width: 16px;
  height: 16px;
  left: 3px;
  top: 3px;
  background: #fff;
  border-radius: 50%;
  transition: transform 0.2s;
}

.switch input:checked + .slider {
  background: var(--accent);
}

.switch input:checked + .slider:before {
  transform: translateX(18px);
}

.switch input:focus-visible + .slider {
  outline: 2px solid var(--accent);
  outline-offset: 2px;
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
  border-radius: var(--radius-sm);
  color: var(--text-muted);
  font-size: 13px;
  font-weight: 600;
  font-family: inherit;
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
  border-radius: var(--radius-sm);
  color: #fff;
  font-size: 13px;
  font-weight: 600;
  font-family: inherit;
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
