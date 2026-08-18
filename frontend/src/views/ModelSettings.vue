<template>
  <div class="page">
    <div class="sticky-top">
      <div class="page-header">
        <button class="back-btn" @click="router.back()">
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round">
            <polyline points="15 18 9 12 15 6"/>
          </svg>
          <span>{{ t('models.back') }}</span>
        </button>
        <h1 class="page-title">{{ decodedModelName }}</h1>
        <p class="page-subtitle">{{ t('modelSettings.subtitle') }}</p>
      </div>

      <!-- Tabs row -->
      <div class="settings-tabs" role="tablist" :aria-label="t('modelSettings.tabsAria')">
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

      <!-- Action area -->
      <div class="settings-actions">
        <span class="action-msg" v-if="saveSuccess">{{ t('modelSettings.saved') }}</span>
        <span class="action-msg action-err" v-else-if="saveError">{{ saveError }}</span>
        <span class="action-spacer"></span>
        <button class="btn-secondary" @click="reset">{{ t('modelSettings.reset') }}</button>
        <button
          class="btn-primary"
          :disabled="saving || hasErrors || !isModified"
          @click="save"
        >
          {{ saving ? t('modelSettings.saving') : t('modelSettings.save') }}
        </button>
      </div>
    </div>

    <!-- Loading -->
    <div v-if="loading" class="loading-placeholder">{{ t('modelSettings.loading') }}</div>

    <!-- Error state -->
    <div v-else-if="loadError" class="error-card">
      <p class="error-text">{{ t('modelSettings.loadFailed') }}：{{ loadError }}</p>
      <button class="retry-btn" @click="loadConfig">{{ t('modelSettings.retry') }}</button>
    </div>

    <!-- Tab panels -->
    <div v-else class="settings-body">
      <!-- Basic -->
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
                <ThemedSelect
                  v-model="cfg.gpuLayers"
                  :options="gpuLayersOptions"
                  :label="t('modelSettings.gpuLayers')"
                />
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

      <!-- Inference -->
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

      <!-- Memory / Loading -->
      <div id="tab-memory" role="tabpanel" aria-labelledby="tab-memory-tab" v-show="activeTab === 2">
        <div class="param-group">
          <h3 class="group-title">{{ t('modelSettings.groupMemory') }}</h3>
          <div class="param-grid col-1">
            <div class="param">
              <label class="param-field">
                <span class="param-label">{{ t('modelSettings.cacheTypeK') }}</span>
                <ThemedSelect
                  v-model="cfg.cacheTypeK"
                  :options="cacheTypeOptions"
                  :label="t('modelSettings.cacheTypeK')"
                />
              </label>
              <p class="param-hint">{{ t('modelSettings.cacheTypeKHint') }}</p>
            </div>
            <div class="param">
              <label class="param-field">
                <span class="param-label">{{ t('modelSettings.cacheTypeV') }}</span>
                <ThemedSelect
                  v-model="cfg.cacheTypeV"
                  :options="cacheTypeOptions"
                  :label="t('modelSettings.cacheTypeV')"
                />
              </label>
              <p class="param-hint">{{ t('modelSettings.cacheTypeVHint') }}</p>
            </div>
            <div class="param">
              <label class="param-field">
                <span class="param-label">{{ t('modelSettings.loadMode') }}</span>
                <ThemedSelect
                  v-model="cfg.loadMode"
                  :options="loadModeOptions"
                  :label="t('modelSettings.loadMode')"
                />
              </label>
              <p class="param-hint">{{ t('modelSettings.loadModeHint') }}</p>
            </div>
          </div>
        </div>
      </div>

      <!-- Multi-GPU -->
      <div id="tab-gpu" role="tabpanel" aria-labelledby="tab-gpu-tab" v-show="activeTab === 3">
        <div class="param-group">
          <h3 class="group-title">{{ t('modelSettings.groupGpu') }}</h3>
          <div class="param-grid col-1">
            <div class="param">
              <label class="param-field">
                <span class="param-label">{{ t('modelSettings.splitMode') }}</span>
                <ThemedSelect
                  v-model="cfg.splitMode"
                  :options="splitModeOptions"
                  :label="t('modelSettings.splitMode')"
                />
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

      <!-- Long context -->
      <div id="tab-context" role="tabpanel" aria-labelledby="tab-context-tab" v-show="activeTab === 4">
        <div class="param-group">
          <h3 class="group-title">{{ t('modelSettings.groupContext') }}</h3>
          <div class="param-grid col-1">
            <div class="param">
              <label class="param-field">
                <span class="param-label">{{ t('modelSettings.ropeScaling') }}</span>
                <ThemedSelect
                  v-model="cfg.ropeScaling"
                  :options="ropeScalingOptions"
                  :label="t('modelSettings.ropeScaling')"
                />
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

      <!-- Advanced -->
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
                <ThemedSelect
                  v-model="cfg.specType"
                  :options="specTypeOptions"
                  :label="t('modelSettings.specType')"
                />
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
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, watch, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { getModelConfig, saveModelConfig } from '../wails'
import { t } from '../lib/i18n'
import ThemedSelect, { type SelectOption } from '../components/ThemedSelect.vue'

// ─── ModelConfig interface (imported by Models.vue etc.) ───────────────────────
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

// ─── Defaults ─────────────────────────────────────────────────────────────────
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

// ─── Route params ───────────────────────────────────────────────────────────────
const router = useRouter()
const route = useRoute()

// Model name is URL-encoded in the route; decode it for display
const decodedModelName = computed(() => decodeURIComponent(String(route.params.modelName || '')))
// The raw encoded value is used for backend API calls
const modelName = computed(() => String(route.params.modelName || ''))

// ─── Local state ───────────────────────────────────────────────────────────────
const loading = ref(false)
const loadError = ref('')

// Snapshot taken after the config loads successfully (used to compute isModified)
const initialConfig = ref<ModelConfig>({ ...defaults })
const saving = ref(false)
const saveError = ref('')
const saveSuccess = ref(false)

/** Index of the currently active tab */
const activeTab = ref(0)

/** Current config (reactive), initialized from defaults + loaded values */
const cfg = reactive<ModelConfig>({ ...defaults })

// ─── ThemedSelect option lists (mirrors the former native <select> options) ───
const gpuLayersOptions: SelectOption[] = [
  { value: 'auto', label: t('modelSettings.gpuAuto') },
  { value: 'all', label: t('modelSettings.gpuAll') },
  { value: '0', label: t('modelSettings.gpuCpuOnly') },
  { value: '10', label: '10' },
  { value: '20', label: '20' },
  { value: '30', label: '30' },
  { value: '40', label: '40' },
  { value: '50', label: '50' },
  { value: '99', label: '99' },
]
const cacheTypeOptions: SelectOption[] = [
  { value: '', label: t('modelSettings.defaultF16') },
  { value: 'f32', label: 'f32' },
  { value: 'f16', label: 'f16' },
  { value: 'bf16', label: 'bf16' },
  { value: 'q8_0', label: t('modelSettings.q8Recommended') },
  { value: 'q4_0', label: 'q4_0' },
  { value: 'q4_1', label: 'q4_1' },
  { value: 'iq4_nl', label: 'iq4_nl' },
  { value: 'q5_0', label: 'q5_0' },
  { value: 'q5_1', label: 'q5_1' },
]
const loadModeOptions: SelectOption[] = [
  { value: '', label: t('modelSettings.loadDefaultMmap') },
  { value: 'mmap', label: t('modelSettings.loadMmap') },
  { value: 'mlock', label: t('modelSettings.loadMlock') },
  { value: 'mmap+mlock', label: t('modelSettings.loadMmapMlock') },
  { value: 'none', label: t('modelSettings.loadNone') },
  { value: 'dio', label: t('modelSettings.loadDio') },
]
const splitModeOptions: SelectOption[] = [
  { value: '', label: t('modelSettings.splitDefaultLayer') },
  { value: 'layer', label: t('modelSettings.splitLayer') },
  { value: 'row', label: t('modelSettings.splitRow') },
  { value: 'tensor', label: t('modelSettings.splitTensor') },
  { value: 'none', label: t('modelSettings.splitNone') },
]
const ropeScalingOptions: SelectOption[] = [
  { value: '', label: t('modelSettings.ropeDefaultNone') },
  { value: 'none', label: 'none' },
  { value: 'linear', label: t('modelSettings.ropeLinear') },
  { value: 'yarn', label: t('modelSettings.ropeYarn') },
]
const specTypeOptions: SelectOption[] = [
  { value: '', label: t('modelSettings.specOff') },
  { value: 'draft-mtp', label: 'draft-mtp' },
]

// ─── Tab definitions (6 categories, icons are inline stroke SVGs) ────────────────────────────
const tabs = [
  {
    id: 'tab-base',
    label: () => t('modelSettings.tabBase'),
    icon: `<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-2 2 2 2 0 0 1-2-2v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1-2-2 2 2 0 0 1 2-2h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 2-2 2 2 0 0 1 2 2v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0-2.83l-.06-.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 2 2 2 2 0 0 1-2 2h-.09a1.65 1.65 0 0 0-1.51 1z"/></svg>`,
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

// ─── Validation ───────────────────────────────────────────────────────────────────
/** Current set of validation errors */
const errors = ref<Set<string>>(new Set())

function validate(): boolean {
  const errs = new Set<string>()
  if (cfg.ctxSize <= 0) errs.add('ctxSize')
  if (cfg.batchSize <= 0) errs.add('batchSize')
  if (cfg.ubatchSize <= 0) errs.add('ubatchSize')
  errors.value = errs
  return errs.size === 0
}

const hasErrors = computed(() => errors.value.size > 0)

/** Whether any field differs from its initial value */
const isModified = computed(() => {
  for (const key of Object.keys(defaults) as (keyof ModelConfig)[]) {
    if (cfg[key] !== initialConfig.value[key]) return true
  }
  return false
})

// Re-validate whenever a field changes
watch(
  () => ({ ...cfg }),
  () => {
    validate()
    saveSuccess.value = false
  },
  { deep: true }
)

// ─── Route linkage: reload when modelName changes ────────────────────────────────
watch(modelName, () => {
  loadConfig()
})

// ─── Load / Reset / Save ──────────────────────────────────────────────────────
async function loadConfig() {
  loading.value = true
  loadError.value = ''
  saveError.value = ''
  saveSuccess.value = false
  activeTab.value = 0

  const name = modelName.value
  if (!name) {
    loadError.value = 'Missing model name'
    loading.value = false
    return
  }

  try {
    const remote = await getModelConfig(name)
    const merged: ModelConfig = { ...defaults, ...(remote || {}) }
    // Reset the reactive cfg with the merged values (keeps the reactive reference)
    for (const key of Object.keys(defaults) as (keyof ModelConfig)[]) {
      ;(cfg as any)[key] = merged[key]
    }
    initialConfig.value = { ...merged }
    errors.value = new Set()
  } catch (e: any) {
    loadError.value = e?.message || String(e)
  } finally {
    loading.value = false
  }
}

function reset() {
  // Fall back to initialConfig (values from the last load/save), keeping the reactive reference
  for (const key of Object.keys(defaults) as (keyof ModelConfig)[]) {
    ;(cfg as any)[key] = initialConfig.value[key]
  }
  errors.value = new Set()
  saveSuccess.value = false
  saveError.value = ''
}

async function save() {
  if (!validate()) return
  saving.value = true
  saveError.value = ''
  saveSuccess.value = false
  try {
    await saveModelConfig(modelName.value, { ...cfg })
    // Update initialConfig after a successful save; isModified becomes false and the save button disables
    initialConfig.value = { ...cfg }
    saveSuccess.value = true
    setTimeout(() => {
      router.back()
    }, 300)
  } catch (e: any) {
    saveError.value = e?.message || String(e)
  } finally {
    saving.value = false
  }
}

// ─── Lifecycle ────────────────────────────────────────────────────────────────
onMounted(() => {
  loadConfig()
})
</script>

<style scoped>
.page {
  padding: 0 48px 60px;
}

.page-header {
  padding-bottom: 20px;
}

.back-btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  background: transparent;
  border: none;
  color: var(--text-muted);
  font-size: 13px;
  cursor: pointer;
  padding: 4px 0;
  margin-bottom: 12px;
  border-radius: 6px;
  transition: color 0.2s;
  font-family: inherit;
}

.back-btn:hover {
  color: var(--text-secondary);
}

.page-title {
  font-size: 24px;
  font-weight: 700;
  color: var(--text-primary);
  margin: 0 0 4px;
  letter-spacing: -0.5px;
  line-height: 1.2;
  word-break: break-all;
}

.page-subtitle {
  font-size: 13px;
  color: var(--text-dim);
  margin: 0;
}

/* ─── Tabs ─── */
.settings-tabs {
  display: flex;
  gap: 4px;
  padding: 0;
  border-bottom: 1px solid var(--border);
  margin-bottom: 0;
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

/* ─── Action area ─── */
.settings-actions {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 16px 0;
  border-bottom: 1px solid var(--border);
  margin-bottom: 20px;
}

.action-msg {
  font-size: 13px;
  font-weight: 600;
  color: var(--success);
}

.action-err {
  color: #ef4444;
}

.action-spacer {
  flex: 1;
}

.btn-secondary {
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

.btn-secondary:hover {
  background: var(--hover-bg);
}

.btn-primary {
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

.btn-primary:hover:not(:disabled) {
  opacity: 0.85;
}

.btn-primary:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

/* ─── Body ─── */
.settings-body {
  /* tabs preserve entered values; no extra container needed */
}

.loading-placeholder {
  padding: 48px 0;
  text-align: center;
  color: var(--text-dim);
  font-size: 14px;
}

.error-card {
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 14px 18px;
  background: rgba(239, 68, 68, 0.06);
  border: 1px solid rgba(239, 68, 68, 0.15);
  border-radius: 10px;
  margin-bottom: 24px;
}

.error-text {
  flex: 1;
  font-size: 13px;
  color: #f87171;
  margin: 0;
}

.retry-btn {
  padding: 6px 16px;
  background: rgba(99, 102, 241, 0.1);
  color: #a78bfa;
  border: 1px solid rgba(99, 102, 241, 0.2);
  border-radius: 6px;
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s;
  white-space: nowrap;
  font-family: inherit;
}

.retry-btn:hover {
  background: rgba(99, 102, 241, 0.2);
}

/* ─── Parameter cards (ported from original modal, scoped to this page) ─── */
.param-group {
  background: var(--bg-card);
  border: 1px solid var(--border-light);
  border-radius: var(--radius-md);
  padding: 16px 18px;
  margin-bottom: 16px;
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

.param-hint {
  margin-top: 6px;
  font-size: 11px;
  color: var(--text-dim);
  line-height: 1.5;
}

/* Pill toggle switch */
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
</style>
