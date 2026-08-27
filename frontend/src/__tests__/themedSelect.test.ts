// @vitest-environment happy-dom
import { describe, it, expect, vi, afterEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { defineComponent } from 'vue'
import ThemedSelect from '../components/ThemedSelect.vue'
import { selectDisplayLabel } from '../lib/selectOptions'

const options = [
  { value: 'auto', label: 'Auto' },
  { value: 'all', label: 'All' },
  { value: '0', label: 'CPU only' },
]

describe('selectDisplayLabel', () => {
  it('returns the label of the matching option', () => {
    expect(selectDisplayLabel(options, 'all', 'Pick one')).toBe('All')
  })

  it('falls back to the raw value when the value has no matching option', () => {
    expect(selectDisplayLabel(options, 'stale-value', 'Pick one')).toBe('stale-value')
  })

  it('returns the placeholder when the value is empty', () => {
    expect(selectDisplayLabel(options, '', 'Pick one')).toBe('Pick one')
  })

  it('returns the label of the empty-value option when present (e.g. "default" choice)', () => {
    expect(selectDisplayLabel([{ value: '', label: 'Default (F16)' }], '', 'Pick one')).toBe('Default (F16)')
  })
})

describe('ThemedSelect', () => {
  afterEach(() => {
    document.body.innerHTML = ''
    vi.restoreAllMocks()
  })

  function mountSelect(props: Record<string, unknown> = {}) {
    return mount(ThemedSelect, {
      props: {
        options,
        ...props,
      },
      attachTo: document.body,
    })
  }

  it('shows the label of the selected value', () => {
    const wrapper = mountSelect({ modelValue: 'auto' })
    expect(wrapper.find('.themed-select__value').text()).toBe('Auto')
  })

  it('shows the placeholder when nothing is selected', () => {
    const wrapper = mountSelect({ modelValue: '', placeholder: 'Pick a model' })
    expect(wrapper.find('.themed-select__value').text()).toBe('Pick a model')
  })

  it('opens the menu on trigger click and closes on a second click', async () => {
    const wrapper = mountSelect()
    expect(wrapper.find('.themed-select__menu').exists()).toBe(false)
    await wrapper.find('.themed-select__trigger').trigger('click')
    expect(wrapper.find('.themed-select__menu').exists()).toBe(true)
    await wrapper.find('.themed-select__trigger').trigger('click')
    expect(wrapper.find('.themed-select__menu').exists()).toBe(false)
  })

  it('emits update:modelValue and closes after selecting an option', async () => {
    const wrapper = mountSelect({ modelValue: 'auto' })
    await wrapper.find('.themed-select__trigger').trigger('click')
    const options2 = wrapper.findAll('.themed-select__option')
    await options2[1].trigger('click')
    expect(wrapper.emitted('update:modelValue')).toEqual([['all']])
    expect(wrapper.find('.themed-select__menu').exists()).toBe(false)
  })

  it('marks the selected option with the check icon', async () => {
    const wrapper = mountSelect({ modelValue: '0' })
    await wrapper.find('.themed-select__trigger').trigger('click')
    const selected = wrapper.find('.themed-select__option--selected')
    expect(selected.exists()).toBe(true)
    expect(selected.find('.themed-select__option-check').exists()).toBe(true)
  })

  it('closes the menu when clicking outside', async () => {
    const wrapper = mountSelect()
    await wrapper.find('.themed-select__trigger').trigger('click')
    expect(wrapper.find('.themed-select__menu').exists()).toBe(true)
    document.body.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    await wrapper.vm.$nextTick()
    expect(wrapper.find('.themed-select__menu').exists()).toBe(false)
  })

  it('does not open when disabled', async () => {
    const wrapper = mountSelect({ disabled: true })
    await wrapper.find('.themed-select__trigger').trigger('click')
    expect(wrapper.find('.themed-select__menu').exists()).toBe(false)
  })

  it('shows the empty text when there are no options', async () => {
    const wrapper = mountSelect({ options: [], emptyText: 'No models' })
    await wrapper.find('.themed-select__trigger').trigger('click')
    expect(wrapper.find('.themed-select__empty').text()).toBe('No models')
  })

  it('works with v-model through a host component', async () => {
    const Host = defineComponent({
      components: { ThemedSelect },
      template: `<ThemedSelect v-model="value" :options="options" />`,
      data: () => ({ value: 'auto', options }),
    })
    const wrapper = mount(Host, { attachTo: document.body })
    await wrapper.find('.themed-select__trigger').trigger('click')
    await wrapper.findAll('.themed-select__option')[2].trigger('click')
    expect(wrapper.vm.$data.value).toBe('0')
  })

  it('opens the menu when Enter is pressed on the trigger', async () => {
    const wrapper = mountSelect()
    const trigger = wrapper.find('.themed-select__trigger')
    await trigger.trigger('keydown', { key: 'Enter' })
    expect(trigger.attributes('aria-expanded')).toBe('true')
    expect(wrapper.findAll('.themed-select__option')).toHaveLength(3)
  })

  it('moves the highlight down from the selected option and wires aria-activedescendant', async () => {
    const wrapper = mountSelect({ modelValue: 'all' })
    const trigger = wrapper.find('.themed-select__trigger')
    await trigger.trigger('click')
    await trigger.trigger('keydown', { key: 'ArrowDown' })
    const items = wrapper.findAll('.themed-select__option')
    const highlighted = wrapper.findAll('.themed-select__option--highlighted')
    expect(highlighted).toHaveLength(1)
    expect(highlighted[0].element).toBe(items[2].element)
    expect(trigger.attributes('aria-activedescendant')).toBe(items[2].attributes('id'))
  })

  it('wraps ArrowUp from the first option to the last and ArrowDown from the last to the first', async () => {
    const wrapper = mountSelect({ modelValue: 'auto' })
    const trigger = wrapper.find('.themed-select__trigger')
    await trigger.trigger('click')
    const items = wrapper.findAll('.themed-select__option')
    await trigger.trigger('keydown', { key: 'ArrowUp' })
    expect(wrapper.findAll('.themed-select__option--highlighted')[0].element).toBe(items[2].element)
    await trigger.trigger('keydown', { key: 'ArrowDown' })
    expect(wrapper.findAll('.themed-select__option--highlighted')[0].element).toBe(items[0].element)
  })

  it('selects the highlighted option via the full keyboard path and closes the menu', async () => {
    const wrapper = mountSelect({ modelValue: 'auto' })
    const trigger = wrapper.find('.themed-select__trigger')
    await trigger.trigger('keydown', { key: 'Enter' })
    await trigger.trigger('keydown', { key: 'ArrowDown' })
    await trigger.trigger('keydown', { key: 'ArrowDown' })
    await trigger.trigger('keydown', { key: 'Enter' })
    expect(wrapper.emitted('update:modelValue')).toEqual([['0']])
    expect(wrapper.find('.themed-select__menu').exists()).toBe(false)
  })

  it('closes the open menu with Escape', async () => {
    const wrapper = mountSelect()
    const trigger = wrapper.find('.themed-select__trigger')
    await trigger.trigger('click')
    expect(wrapper.find('.themed-select__menu').exists()).toBe(true)
    await trigger.trigger('keydown', { key: 'Escape' })
    expect(wrapper.find('.themed-select__menu').exists()).toBe(false)
  })

  it('moves the highlight to the first option with Home and the last with End', async () => {
    const wrapper = mountSelect({ modelValue: 'all' })
    const trigger = wrapper.find('.themed-select__trigger')
    await trigger.trigger('click')
    const items = wrapper.findAll('.themed-select__option')
    await trigger.trigger('keydown', { key: 'End' })
    expect(wrapper.findAll('.themed-select__option--highlighted')[0].element).toBe(items[2].element)
    await trigger.trigger('keydown', { key: 'Home' })
    expect(wrapper.findAll('.themed-select__option--highlighted')[0].element).toBe(items[0].element)
  })

  it('does not open with the keyboard when disabled', async () => {
    const wrapper = mountSelect({ disabled: true })
    await wrapper.find('.themed-select__trigger').trigger('keydown', { key: 'Enter' })
    expect(wrapper.find('.themed-select__menu').exists()).toBe(false)
  })

  it('highlights the hovered option', async () => {
    const wrapper = mountSelect()
    await wrapper.find('.themed-select__trigger').trigger('click')
    const items = wrapper.findAll('.themed-select__option')
    await items[1].trigger('mouseenter')
    const highlighted = wrapper.findAll('.themed-select__option--highlighted')
    expect(highlighted).toHaveLength(1)
    expect(highlighted[0].element).toBe(items[1].element)
  })
})
