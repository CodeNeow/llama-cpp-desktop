<template>
  <div class="page">
    <div class="page-header">
      <h1 class="page-title">模型</h1>
      <p class="page-subtitle">
        <template v-if="models.length">{{ models.length }} 个模型</template>
        <template v-else>将 GGUF 模型文件放入 LLM-Models 目录</template>
      </p>
    </div>

    <!-- Loading skeleton -->
    <div v-if="loading" class="loading-grid">
      <div v-for="i in 3" :key="i" class="skeleton-card">
        <div class="skeleton-line skeleton-title"></div>
        <div class="skeleton-line"></div>
        <div class="skeleton-line skeleton-short"></div>
      </div>
    </div>

    <!-- Error state -->
    <div v-else-if="error" class="error-card">
      <div class="error-icon">
        <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
          <circle cx="12" cy="12" r="10"/><line x1="12" y1="8" x2="12" y2="12"/><line x1="12" y1="16" x2="12.01" y2="16"/>
        </svg>
      </div>
      <h2>无法获取模型列表</h2>
      <p>{{ error }}</p>
      <button class="retry-btn" @click="fetchModels">重试</button>
    </div>

    <!-- Empty state -->
    <div v-else-if="models.length === 0" class="empty-state">
      <div class="empty-icon">
        <svg width="64" height="64" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1" stroke-linecap="round" stroke-linejoin="round">
          <path d="M21 16V8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16z"/>
          <polyline points="3.27 6.96 12 12.01 20.73 6.96"/>
          <line x1="12" y1="22.08" x2="12" y2="12"/>
        </svg>
      </div>
      <h2>暂无模型</h2>
      <p>将 .gguf 模型文件放入项目根目录的 <code>LLM-Models</code> 文件夹中</p>
      <div class="empty-hint">
        <span>示例路径：</span>
        <code>LLM-Models/qwen2.5-7b-instruct-q4_k_m.gguf</code>
      </div>
    </div>

    <!-- Model list -->
    <div v-else class="model-list">
      <div v-for="model in models" :key="model.path" class="model-card">
        <div class="model-icon">
          <svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
            <path d="M21 16V8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16z"/>
          </svg>
        </div>
        <div class="model-info">
          <div class="model-header">
            <h3 class="model-name">{{ model.name }}</h3>
            <span v-if="model.hasMmproj" class="mmproj-badge" title="支持多模态 (mmproj)">👁️ 多模态</span>
            <button class="model-settings-btn" title="设置" @click.stop="openSettings(model)">
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                <circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-2 2 2 2 0 0 1-2-2v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1-2-2 2 2 0 0 1 2-2h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 2-2 2 2 0 0 1 2 2v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 2 2 2 2 0 0 1-2 2h-.09a1.65 1.65 0 0 0-1.51 1z"/>
              </svg>
            </button>
          </div>
          <div class="model-author" v-if="model.author">{{ model.author }}</div>
          <div class="model-meta">
            <span v-if="model.architecture && model.architecture !== '-'" class="meta-tag arch-tag">{{ model.architecture }}</span>
            <span v-if="model.quantization && model.quantization !== '-'" class="meta-tag quant-tag">{{ model.quantization }}</span>
            <span class="meta-tag size-tag">{{ model.sizeHuman }}</span>
          </div>
          <div class="model-path">{{ model.path }}</div>
        </div>
      </div>
    </div>

    <ModelSettings
      :visible="settingsVisible"
      :modelName="settingsModelName"
      :initialConfig="settingsConfig"
      :saving="saving"
      :saveError="saveError"
      :saveSuccess="saveSuccess"
      @close="settingsVisible = false"
      @save="saveSettings"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import ModelSettings from '../components/ModelSettings.vue'
import type { ModelConfig } from '../components/ModelSettings.vue'
import { getModels, getModelConfig, saveModelConfig } from '../wails'

interface ModelInfo {
  author: string
  name: string
  path: string
  sizeBytes: number
  sizeHuman: string
  architecture: string
  quantization: string
  hasMmproj: boolean
}

const models = ref<ModelInfo[]>([])
const loading = ref(true)
const error = ref('')

// Settings dialog
const settingsVisible = ref(false)
const settingsModelName = ref('')
const settingsModelId = ref('')
const settingsConfig = ref<Partial<ModelConfig>>({})

async function openSettings(model: ModelInfo) {
  settingsModelName.value = model.name
  settingsModelId.value = model.name

  // Load saved config from backend
  try {
    const cfg = await getModelConfig(model.name)
    settingsConfig.value = cfg || {}
  } catch {
    settingsConfig.value = {}
  }
  settingsVisible.value = true
}

const saving = ref(false)
const saveError = ref('')
const saveSuccess = ref(false)

async function saveSettings(config: ModelConfig) {
  saving.value = true
  saveError.value = ''
  saveSuccess.value = false
  try {
    await saveModelConfig(settingsModelId.value, config)
    saveSuccess.value = true
    setTimeout(() => {
      settingsVisible.value = false
      saveSuccess.value = false
    }, 800)
  } catch (e: any) {
    saveError.value = '保存失败: ' + (e.message || '未知错误')
  } finally {
    saving.value = false
  }
}

async function fetchModels() {
  loading.value = true
  error.value = ''
  try {
    models.value = await getModels() as ModelInfo[]
  } catch (e: any) {
    error.value = `无法连接后端服务：${e.message}`
  } finally {
    loading.value = false
  }
}

onMounted(fetchModels)
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

.page-subtitle code {
  background: var(--border);
  padding: 2px 6px;
  border-radius: 4px;
  font-size: 13px;
}

/* ─── Model list ─── */
.model-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.model-card {
  display: flex;
  align-items: flex-start;
  gap: 18px;
  padding: 20px 24px;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 14px;
  transition: all 0.2s ease;
}

.model-card:hover {
  background: var(--border-light);
  border-color: var(--overlay-10);
  transform: translateX(4px);
}

.model-icon {
  width: 48px;
  height: 48px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(99, 102, 241, 0.12);
  border-radius: 12px;
  color: #a78bfa;
  flex-shrink: 0;
  margin-top: 2px;
}

.model-info {
  flex: 1;
  min-width: 0;
}

.model-header {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 4px;
}

.model-settings-btn {
  width: 30px;
  height: 30px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: none;
  border: none;
  color: var(--text-dim);
  cursor: pointer;
  border-radius: 6px;
  opacity: 0;
  transition: opacity 0.2s, background 0.15s, color 0.15s;
  flex-shrink: 0;
  margin-left: auto;
}

.model-card:hover .model-settings-btn {
  opacity: 1;
}

.model-settings-btn:hover {
  background: var(--hover-bg);
  color: var(--accent-light);
}

.model-name {
  font-size: 16px;
  font-weight: 600;
  color: var(--text-secondary);
  margin: 0;
  word-break: break-word;
  line-height: 1.4;
}

.mmproj-badge {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 2px 10px;
  background: rgba(168, 85, 247, 0.12);
  color: #c084fc;
  border: 1px solid rgba(168, 85, 247, 0.2);
  border-radius: 20px;
  font-size: 11px;
  font-weight: 600;
  white-space: nowrap;
  flex-shrink: 0;
}

.model-author {
  font-size: 12px;
  color: var(--text-dim);
  margin-bottom: 10px;
}

.model-meta {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
  margin-bottom: 8px;
}

.meta-tag {
  display: inline-flex;
  align-items: center;
  padding: 3px 10px;
  border-radius: 6px;
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.3px;
}

.arch-tag {
  background: rgba(59, 130, 246, 0.12);
  color: #60a5fa;
  border: 1px solid rgba(59, 130, 246, 0.2);
}

.quant-tag {
  background: rgba(34, 197, 94, 0.1);
  color: #4ade80;
  border: 1px solid rgba(34, 197, 94, 0.18);
}

.size-tag {
  background: var(--border-light);
  color: var(--text-muted);
  border: 1px solid var(--overlay-8);
}

.model-path {
  font-size: 11px;
  color: var(--overlay-20);
  font-family: 'JetBrains Mono', 'Fira Code', monospace;
  word-break: break-all;
  line-height: 1.4;
}

/* ─── Empty ─── */
.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 56px 32px;
  background: linear-gradient(135deg, rgba(99, 102, 241, 0.04) 0%, rgba(167, 139, 250, 0.02) 100%);
  border: 1px solid var(--border);
  border-radius: 16px;
  text-align: center;
}

.empty-icon {
  color: rgba(99, 102, 241, 0.3);
  margin-bottom: 20px;
}

.empty-state h2 {
  font-size: 18px;
  font-weight: 600;
  color: var(--text-secondary);
  margin: 0 0 8px;
}

.empty-state p {
  font-size: 13px;
  color: var(--text-dim);
  margin: 0 0 20px;
  max-width: 420px;
  line-height: 1.6;
}

.empty-state code {
  background: var(--border);
  padding: 2px 6px;
  border-radius: 4px;
  font-size: 12px;
  color: var(--text-secondary);
}

.empty-hint {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 16px;
  background: rgba(0, 0, 0, 0.2);
  border-radius: 8px;
  font-size: 12px;
  color: var(--text-dim);
}

.empty-hint code {
  color: #a78bfa;
}

/* ─── Loading skeleton ─── */
.loading-grid {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.skeleton-card {
  padding: 24px 28px;
  background: var(--surface);
  border: 1px solid var(--skeleton-bg);
  border-radius: 14px;
}

.skeleton-line {
  height: 12px;
  background: var(--skeleton-bg);
  border-radius: 4px;
  margin-bottom: 10px;
  animation: shimmer 1.5s infinite;
}

.skeleton-title {
  width: 30%;
  height: 16px;
}

.skeleton-short {
  width: 50%;
}

@keyframes shimmer {
  0%, 100% { opacity: 0.4; }
  50% { opacity: 0.8; }
}

/* ─── Error ─── */
.error-card {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 56px 32px;
  background: rgba(239, 68, 68, 0.04);
  border: 1px solid rgba(239, 68, 68, 0.12);
  border-radius: 16px;
  text-align: center;
}

.error-icon {
  color: rgba(239, 68, 68, 0.5);
  margin-bottom: 16px;
}

.error-card h2 {
  font-size: 18px;
  font-weight: 600;
  color: var(--text-secondary);
  margin: 0 0 8px;
}

.error-card p {
  font-size: 13px;
  color: var(--text-dim);
  margin: 0 0 20px;
}

.retry-btn {
  padding: 8px 24px;
  background: rgba(99, 102, 241, 0.15);
  color: #a78bfa;
  border: 1px solid rgba(99, 102, 241, 0.25);
  border-radius: 8px;
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.2s;
}

.retry-btn:hover {
  background: rgba(99, 102, 241, 0.25);
  border-color: rgba(99, 102, 241, 0.4);
}
</style>
