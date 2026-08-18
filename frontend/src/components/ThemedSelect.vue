<template>
  <div class="themed-select" :class="`themed-select--${variant}`">
    <button
      type="button"
      class="themed-select__trigger"
      :disabled="disabled"
      :aria-expanded="open"
      aria-haspopup="listbox"
      :aria-label="label"
      @click.stop="open = !open"
      @keydown.esc="open = false"
    >
      <span class="themed-select__value">{{ displayValue }}</span>
      <svg class="themed-select__chevron" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
        <polyline points="6 9 12 15 18 9"/>
      </svg>
    </button>
    <div v-if="open" class="themed-select__menu" role="listbox" @keydown.esc="open = false">
      <button
        v-for="opt in options"
        :key="opt.value"
        type="button"
        class="themed-select__option"
        :class="{ 'themed-select__option--selected': opt.value === modelValue }"
        role="option"
        :aria-selected="opt.value === modelValue"
        @click="select(opt.value)"
      >
        <span class="themed-select__option-label">{{ opt.label }}</span>
        <svg v-if="opt.value === modelValue" class="themed-select__option-check" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
          <polyline points="20 6 9 17 4 12"/>
        </svg>
      </button>
      <div v-if="options.length === 0" class="themed-select__empty">{{ emptyText }}</div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { selectDisplayLabel } from '../lib/selectOptions'

export interface SelectOption {
  value: string
  label: string
}

const props = withDefaults(defineProps<{
  modelValue?: string
  options: SelectOption[]
  placeholder?: string
  disabled?: boolean
  label?: string
  emptyText?: string
  /** 'field' matches form inputs (ModelSettings), 'toolbar' matches the chat toolbar */
  variant?: 'field' | 'toolbar'
}>(), {
  modelValue: '',
  placeholder: '',
  disabled: false,
  label: '',
  emptyText: '',
  variant: 'field',
})

const emit = defineEmits<{
  (e: 'update:modelValue', value: string): void
}>()

const open = ref(false)

const displayValue = computed(() => selectDisplayLabel(props.options, props.modelValue, props.placeholder))

function select(value: string) {
  emit('update:modelValue', value)
  open.value = false
}

/** Close the menu when clicking anywhere outside the component */
function onDocClick() {
  open.value = false
}

onMounted(() => document.addEventListener('click', onDocClick))
onUnmounted(() => document.removeEventListener('click', onDocClick))
</script>

<style scoped>
.themed-select {
  position: relative;
}

/* ─── Trigger ─── */
.themed-select__trigger {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  text-align: left;
  font-size: 13px;
  font-weight: 500;
  font-family: inherit;
  outline: none;
  cursor: pointer;
  transition: all 0.2s;
}

.themed-select__value {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.themed-select__chevron {
  flex-shrink: 0;
  color: var(--text-dim);
  transition: transform 0.2s;
}

.themed-select__trigger[aria-expanded='true'] .themed-select__chevron {
  transform: rotate(180deg);
}

.themed-select__trigger:disabled {
  opacity: 0.35;
  cursor: default;
}

/* field variant: blends with .param-input form fields */
.themed-select--field .themed-select__trigger {
  padding: 8px 10px;
  background: var(--bg-primary);
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
  color: var(--text-primary);
}

.themed-select--field .themed-select__trigger:hover:not(:disabled) {
  border-color: var(--overlay-20);
}

.themed-select--field .themed-select__trigger:focus-visible {
  border-color: var(--accent);
  box-shadow: 0 0 0 3px var(--accent-glow);
}

.themed-select--field .themed-select__trigger[aria-expanded='true'] {
  border-color: var(--accent);
  box-shadow: 0 0 0 3px var(--accent-glow);
}

/* toolbar variant: blends with the chat toolbar controls */
.themed-select--toolbar .themed-select__trigger {
  padding: 8px 12px;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 8px;
  color: var(--text-primary);
  min-width: 240px;
  max-width: 380px;
}

.themed-select--toolbar .themed-select__trigger:hover:not(:disabled) {
  background: var(--hover-bg);
}

.themed-select--toolbar .themed-select__trigger:focus-visible {
  border-color: rgba(99, 102, 241, 0.4);
}

.themed-select--toolbar .themed-select__trigger[aria-expanded='true'] {
  background: var(--hover-bg);
  border-color: rgba(99, 102, 241, 0.4);
}

/* ─── Menu (in-app so it follows the theme in both light and dark) ─── */
.themed-select__menu {
  position: absolute;
  left: 0;
  right: 0;
  top: calc(100% + 6px);
  z-index: 30;
  min-width: 240px;
  max-height: 300px;
  overflow-y: auto;
  background: var(--bg-secondary);
  border: 1px solid var(--border);
  border-radius: 10px;
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.25);
  padding: 4px;
}

.themed-select__option {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  padding: 8px 10px;
  background: transparent;
  border: none;
  border-radius: 6px;
  color: var(--text-secondary);
  font-size: 13px;
  font-weight: 500;
  font-family: inherit;
  text-align: left;
  cursor: pointer;
  transition: all 0.15s;
}

.themed-select__option:hover {
  background: var(--hover-bg);
  color: var(--text-primary);
}

.themed-select__option--selected {
  color: var(--accent-light);
}

.themed-select__option-label {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.themed-select__option-check {
  flex-shrink: 0;
}

.themed-select__empty {
  padding: 10px;
  color: var(--text-dim);
  font-size: 13px;
  text-align: center;
}
</style>
