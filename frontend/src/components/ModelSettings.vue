<template>
  <div v-if="visible" class="modal-overlay" @click.self="close">
    <div class="modal">
      <div class="modal-header">
        <h2>模型设置</h2>
        <span class="modal-model">{{ modelName }}</span>
        <button class="modal-close" @click="close">✕</button>
      </div>

      <div class="modal-body">
        <!-- Performance -->
        <fieldset class="param-group">
          <legend>性能</legend>
          <div class="param-row">
            <label>CPU 线程 (-t)</label>
            <input v-model.number="cfg.threads" type="number" class="param-input param-num" placeholder="-1 (自动)" />
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
            <input v-model.number="cfg.ctxSize" type="number" class="param-input param-num" placeholder="4096" />
          </div>
          <div class="param-row">
            <label>Batch 大小 (-b)</label>
            <input v-model.number="cfg.batchSize" type="number" class="param-input param-num" placeholder="2048" />
          </div>
          <div class="param-row">
            <label>μBatch 大小 (-ub)</label>
            <input v-model.number="cfg.ubatchSize" type="number" class="param-input param-num" placeholder="512" />
          </div>
          <label class="param-check">
            <input type="checkbox" v-model="cfg.flashAttn" />
            <span>Flash Attention (-fa) <em>加速推理</em></span>
          </label>
        </fieldset>

        <!-- Memory -->
        <fieldset class="param-group">
          <legend>内存 / KV 缓存</legend>
          <div class="param-row">
            <label>KV 缓存 K 类型</label>
            <select v-model="cfg.cacheTypeK" class="param-input">
              <option value="">默认 (f16)</option>
              <option value="q8_0">q8_0 (推荐)</option>
              <option value="q4_0">q4_0</option>
              <option value="f16">f16</option>
              <option value="bf16">bf16</option>
            </select>
          </div>
          <div class="param-row">
            <label>KV 缓存 V 类型</label>
            <select v-model="cfg.cacheTypeV" class="param-input">
              <option value="">默认 (f16)</option>
              <option value="q8_0">q8_0 (推荐)</option>
              <option value="q4_0">q4_0</option>
              <option value="f16">f16</option>
              <option value="bf16">bf16</option>
            </select>
          </div>
          <label class="param-check">
            <input type="checkbox" v-model="cfg.mlock" />
            <span>mlock <em>锁定内存防止 swap</em></span>
          </label>
          <label class="param-check">
            <input type="checkbox" v-model="cfg.noMmap" />
            <span>no-mmap <em>禁止内存映射 (内存紧张时)</em></span>
          </label>
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
import { reactive, watch } from 'vue'

export interface ModelConfig {
  threads: number
  gpuLayers: string
  ctxSize: number
  batchSize: number
  ubatchSize: number
  flashAttn: boolean
  cacheTypeK: string
  cacheTypeV: string
  mlock: boolean
  noMmap: boolean
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
  mlock: false,
  noMmap: false,
}

const cfg = reactive<ModelConfig>({ ...defaults, ...props.initialConfig })

watch(() => props.visible, (v) => {
  if (v) Object.assign(cfg, defaults, props.initialConfig)
})

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
  width: 520px;
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
