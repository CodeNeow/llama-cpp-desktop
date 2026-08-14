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
        <h2 id="model-settings-title">{{ t('modelSettings.title') }}</h2>
        <span class="modal-model">{{ modelName }}</span>
        <button ref="closeBtn" class="modal-close" :aria-label="t('modelSettings.close')" @click="close">✕</button>
      </div>

      <div class="modal-tabs" role="tablist" :aria-label="t('modelSettings.tabsAria')">
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
          {{ tab.label() }}
        </button>
      </div>

      <div class="modal-body">
        <!-- 基础 -->
        <div id="tab-base" role="tabpanel" aria-labelledby="tab-base-tab" v-show="activeTab === 0">
          <div class="param-group">
            <h3 class="group-title">{{ t('modelSettings.groupBase') }}</h3>
            <div class="param-grid">
              <div class="param">
                <label class="param-field">
                  <span class="param-label">{{ t('modelSettings.threads') }}</span>
                  <input v-model.number="cfg.threads" type="number" min="-1" step="1" class="param-input" :placeholder="t('modelSettings.threadsPlaceholder')" />
                </label>
                <p class="param-hint">{{ t('modelSettings.threadsHint') }}</p>
              </div>
              <div class="param">
                <label class="param-field">
                  <span class="param-label">{{ t('modelSettings.gpuLayers') }}</span>
                  <select v-model="cfg.gpuLayers" class="param-input">
                    <option value="auto">{{ t('modelSettings.gpuAuto') }}</option>
                    <option value="all">{{ t('modelSettings.gpuAll') }}</option>
                    <option value="0">{{ t('modelSettings.gpuCpuOnly') }}</option>
                    <option value="10">10</option>
                    <option value="20">20</option>
                    <option value="30">30</option>
                    <option value="40">40</option>
                    <option value="50">50</option>
                    <option value="99">99</option>
                  </select>
                </label>
                <p class="param-hint">{{ t('modelSettings.gpuLayersHint') }}</p>
              </div>
              <div class="param">
                <label class="param-field">
                  <span class="param-label">{{ t('modelSettings.ctxSize') }}</span>
                  <input v-model.number="cfg.ctxSize" type="number" min="1" step="1" class="param-input" placeholder="4096" />
                </label>
                <p class="param-hint">{{ t('modelSettings.ctxSizeHint') }}</p>
              </div>
              <div class="param">
                <label class="param-field">
                  <span class="param-label">{{ t('modelSettings.batchSize') }}</span>
                  <input v-model.number="cfg.batchSize" type="number" min="1" step="1" class="param-input" placeholder="2048" />
                </label>
                <p class="param-hint">{{ t('modelSettings.batchSizeHint') }}</p>
              </div>
              <div class="param">
                <label class="param-field">
                  <span class="param-label">{{ t('modelSettings.ubatchSize') }}</span>
                  <input v-model.number="cfg.ubatchSize" type="number" min="1" step="1" class="param-input" placeholder="512" />
                </label>
                <p class="param-hint">{{ t('modelSettings.ubatchSizeHint') }}</p>
              </div>
            </div>
          </div>
        </div>

        <!-- 推理 -->
        <div id="tab-infer" role="tabpanel" aria-labelledby="tab-infer-tab" v-show="activeTab === 1">
          <div class="param-group">
            <h3 class="group-title">{{ t('modelSettings.groupInfer') }}</h3>
            <div class="param-grid col-1">
              <div class="param">
                <div class="toggle-row">
                  <span class="toggle-text">
                    Flash Attention
                    <span class="toggle-sub">{{ t('modelSettings.flashAttnSub') }}</span>
                  </span>
                  <label class="switch">
                    <input type="checkbox" v-model="cfg.flashAttn" aria-label="Flash Attention" />
                    <span class="slider"></span>
                  </label>
                </div>
                <p class="param-hint">{{ t('modelSettings.flashAttnHint') }}</p>
              </div>
              <div class="param">
                <div class="toggle-row">
                  <span class="toggle-text">
                    cpu-moe
                    <span class="toggle-sub">{{ t('modelSettings.cpuMoeSub') }}</span>
                  </span>
                  <label class="switch">
                    <input type="checkbox" v-model="cfg.cpuMoe" aria-label="cpu-moe" />
                    <span class="slider"></span>
                  </label>
                </div>
                <p class="param-hint">{{ t('modelSettings.cpuMoeHint') }}</p>
              </div>
              <div class="param">
                <label class="param-field">
                  <span class="param-label">{{ t('modelSettings.nCpuMoe') }}</span>
                  <input v-model.number="cfg.nCpuMoe" type="number" min="0" step="1" class="param-input" :placeholder="t('modelSettings.nCpuMoePlaceholder')" />
                </label>
                <p class="param-hint">{{ t('modelSettings.nCpuMoeHint') }}</p>
              </div>
            </div>
          </div>
        </div>

        <!-- 内存 / 加载 -->
        <div id="tab-memory" role="tabpanel" aria-labelledby="tab-memory-tab" v-show="activeTab === 2">
          <div class="param-group">
            <h3 class="group-title">{{ t('modelSettings.groupMemory') }}</h3>
            <div class="param-grid col-1">
              <div class="param">
                <label class="param-field">
                  <span class="param-label">{{ t('modelSettings.cacheTypeK') }}</span>
                  <select v-model="cfg.cacheTypeK" class="param-input">
                    <option value="">{{ t('modelSettings.defaultF16') }}</option>
                    <option value="f32">f32</option>
                    <option value="f16">f16</option>
                    <option value="bf16">bf16</option>
                    <option value="q8_0">{{ t('modelSettings.q8Recommended') }}</option>
                    <option value="q4_0">q4_0</option>
                    <option value="q4_1">q4_1</option>
                    <option value="iq4_nl">iq4_nl</option>
                    <option value="q5_0">q5_0</option>
                    <option value="q5_1">q5_1</option>
                  </select>
                </label>
                <p class="param-hint">{{ t('modelSettings.cacheTypeKHint') }}</p>
              </div>
              <div class="param">
                <label class="param-field">
                  <span class="param-label">{{ t('modelSettings.cacheTypeV') }}</span>
                  <select v-model="cfg.cacheTypeV" class="param-input">
                    <option value="">{{ t('modelSettings.defaultF16') }}</option>
                    <option value="f32">f32</option>
                    <option value="f16">f16</option>
                    <option value="bf16">bf16</option>
                    <option value="q8_0">{{ t('modelSettings.q8Recommended') }}</option>
                    <option value="q4_0">q4_0</option>
                    <option value="q4_1">q4_1</option>
                    <option value="iq4_nl">iq4_nl</option>
                    <option value="q5_0">q5_0</option>
                    <option value="q5_1">q5_1</option>
                  </select>
                </label>
                <p class="param-hint">{{ t('modelSettings.cacheTypeVHint') }}</p>
              </div>
              <div class="param">
                <label class="param-field">
                  <span class="param-label">{{ t('modelSettings.loadMode') }}</span>
                  <select v-model="cfg.loadMode" class="param-input">
                    <option value="">{{ t('modelSettings.loadDefaultMmap') }}</option>
                    <option value="mmap">{{ t('modelSettings.loadMmap') }}</option>
                    <option value="mlock">{{ t('modelSettings.loadMlock') }}</option>
                    <option value="mmap+mlock">{{ t('modelSettings.loadMmapMlock') }}</option>
                    <option value="none">{{ t('modelSettings.loadNone') }}</option>
                    <option value="dio">{{ t('modelSettings.loadDio') }}</option>
                  </select>
                </label>
                <p class="param-hint">{{ t('modelSettings.loadModeHint') }}</p>
              </div>
            </div>
          </div>
        </div>

        <!-- 多 GPU -->
        <div id="tab-gpu" role="tabpanel" aria-labelledby="tab-gpu-tab" v-show="activeTab === 3">
          <div class="param-group">
            <h3 class="group-title">{{ t('modelSettings.groupGpu') }}</h3>
            <div class="param-grid col-1">
              <div class="param">
                <label class="param-field">
                  <span class="param-label">{{ t('modelSettings.splitMode') }}</span>
                  <select v-model="cfg.splitMode" class="param-input">
                    <option value="">{{ t('modelSettings.splitDefaultLayer') }}</option>
                    <option value="layer">{{ t('modelSettings.splitLayer') }}</option>
                    <option value="row">{{ t('modelSettings.splitRow') }}</option>
                    <option value="tensor">{{ t('modelSettings.splitTensor') }}</option>
                    <option value="none">{{ t('modelSettings.splitNone') }}</option>
                  </select>
                </label>
                <p class="param-hint">{{ t('modelSettings.splitModeHint') }}</p>
              </div>
              <div class="param">
                <label class="param-field">
                  <span class="param-label">{{ t('modelSettings.tensorSplit') }}</span>
                  <input v-model="cfg.tensorSplit" type="text" class="param-input" :placeholder="t('modelSettings.tensorSplitPlaceholder')" />
                </label>
                <p class="param-hint">{{ t('modelSettings.tensorSplitHint') }}</p>
              </div>
              <div class="param">
                <label class="param-field">
                  <span class="param-label">{{ t('modelSettings.mainGpu') }}</span>
                  <input v-model.number="cfg.mainGpu" type="number" min="0" step="1" class="param-input" placeholder="0" />
                </label>
                <p class="param-hint">{{ t('modelSettings.mainGpuHint') }}</p>
              </div>
            </div>
          </div>
        </div>

        <!-- 长上下文 -->
        <div id="tab-context" role="tabpanel" aria-labelledby="tab-context-tab" v-show="activeTab === 4">
          <div class="param-group">
            <h3 class="group-title">{{ t('modelSettings.groupContext') }}</h3>
            <div class="param-grid col-1">
              <div class="param">
                <label class="param-field">
                  <span class="param-label">{{ t('modelSettings.ropeScaling') }}</span>
                  <select v-model="cfg.ropeScaling" class="param-input">
                    <option value="">{{ t('modelSettings.ropeDefaultNone') }}</option>
                    <option value="none">none</option>
                    <option value="linear">{{ t('modelSettings.ropeLinear') }}</option>
                    <option value="yarn">{{ t('modelSettings.ropeYarn') }}</option>
                  </select>
                </label>
                <p class="param-hint">{{ t('modelSettings.ropeScalingHint') }}</p>
              </div>
              <div class="param">
                <label class="param-field">
                  <span class="param-label">{{ t('modelSettings.ropeScale') }}</span>
                  <input v-model.number="cfg.ropeScale" type="number" min="0" step="0.5" class="param-input" placeholder="0" />
                </label>
                <p class="param-hint">{{ t('modelSettings.ropeScaleHint') }}</p>
              </div>
            </div>
          </div>
        </div>

        <!-- 高级 -->
        <div id="tab-advanced" role="tabpanel" aria-labelledby="tab-advanced-tab" v-show="activeTab === 5">
          <div class="param-group">
            <h3 class="group-title">{{ t('modelSettings.groupAdvanced') }}</h3>
            <div class="param-grid col-1">
              <div class="param">
                <label class="param-field">
                  <span class="param-label">{{ t('modelSettings.mmproj') }}</span>
                  <input v-model="cfg.mmproj" type="text" class="param-input" :placeholder="t('modelSettings.mmprojPlaceholder')" />
                </label>
                <p class="param-hint">{{ t('modelSettings.mmprojHint') }}</p>
              </div>
              <div class="param">
                <div class="toggle-row">
                  <span class="toggle-text">
                    {{ t('modelSettings.reasoning') }}
                    <span class="toggle-sub">{{ t('modelSettings.reasoningSub') }}</span>
                  </span>
                  <label class="switch">
                    <input type="checkbox" v-model="cfg.reasoning" aria-label="Reasoning" />
                    <span class="slider"></span>
                  </label>
                </div>
                <p class="param-hint">{{ t('modelSettings.reasoningHint') }}</p>
              </div>
              <div class="param">
                <label class="param-field">
                  <span class="param-label">{{ t('modelSettings.specType') }}</span>
                  <select v-model="cfg.specType" class="param-input">
                    <option value="">{{ t('modelSettings.specOff') }}</option>
                    <option value="draft-mtp">draft-mtp</option>
                  </select>
                </label>
                <p class="param-hint">{{ t('modelSettings.specTypeHint') }}</p>
              </div>
              <div class="param">
                <label class="param-field">
                  <span class="param-label">{{ t('modelSettings.specDraftNMax') }}</span>
                  <input v-model.number="cfg.specDraftNMax" type="number" min="0" step="1" class="param-input" placeholder="0" />
                </label>
                <p class="param-hint">{{ t('modelSettings.specDraftNMaxHint') }}</p>
              </div>
            </div>
          </div>
        </div>
      </div>

      <div class="modal-footer">
        <span class="footer-msg" v-if="saveSuccess">{{ t('modelSettings.saved') }}</span>
        <span class="footer-msg footer-err" v-else-if="saveError">{{ saveError }}</span>
        <button class="btn-cancel" @click="close">{{ t('modelSettings.cancel') }}</button>
        <button class="btn-save" :disabled="saving" @click="save">
          {{ saving ? t('modelSettings.saving') : t('modelSettings.save') }}
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { reactive, ref, watch, nextTick } from 'vue'
import { t } from '../lib/i18n'

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

/** 6 个参数分类 tab，图标为内联 stroke SVG（16×16，风格与 Sidebar 导航一致）。
 * label 用函数在每次渲染时取当前语言文案，切换语言即时生效。 */
const tabs = [
  {
    id: 'tab-base',
    label: () => t('modelSettings.tabBase'),
    icon: `<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-2 2 2 2 0 0 1-2-2v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1-2-2 2 2 0 0 1 2-2h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 2-2 2 2 0 0 1 2 2v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 2 2 2 2 0 0 1-2 2h-.09a1.65 1.65 0 0 0-1.51 1z"/></svg>`,
  },
  {
    id: 'tab-infer',
    label: () => t('modelSettings.tabInfer'),
    icon: `<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polygon points="13 2 3 14 12 14 11 22 21 10 12 10 13 2"/></svg>`,
  },
  {
    id: 'tab-memory',
    label: () => t('modelSettings.tabMemory'),
    icon: `<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 16V8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16z"/><polyline points="3.27 6.96 12 12.01 20.73 6.96"/><line x1="12" y1="22.08" x2="12" y2="12"/></svg>`,
  },
  {
    id: 'tab-gpu',
    label: () => t('modelSettings.tabGpu'),
    icon: `<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polygon points="12 2 2 7 12 12 22 7 12 2"/><polyline points="2 17 12 22 22 17"/><polyline points="2 12 12 17 22 12"/></svg>`,
  },
  {
    id: 'tab-context',
    label: () => t('modelSettings.tabContext'),
    icon: `<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="2" y="9" width="20" height="6" rx="1"/><line x1="6" y1="9" x2="6" y2="12"/><line x1="10" y1="9" x2="10" y2="13"/><line x1="14" y1="9" x2="14" y2="12"/><line x1="18" y1="9" x2="18" y2="13"/></svg>`,
  },
  {
    id: 'tab-advanced',
    label: () => t('modelSettings.tabAdvanced'),
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
  height: min(620px, 80vh);
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
  scrollbar-gutter: stable;
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
