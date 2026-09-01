// @vitest-environment happy-dom
// isEditableTarget classifies real DOM elements, so the suite needs a DOM.
import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import {
  keyboardVisible,
  isEditableTarget,
  isKeyboardShrink,
  initKeyboardTracking,
  stopKeyboardTracking,
  KEYBOARD_SHRINK_THRESHOLD_PX,
} from '../lib/keyboard'

afterEach(() => {
  // Detach window listeners and reset the module-level reactive state so
  // tests never leak into each other.
  stopKeyboardTracking()
  keyboardVisible.value = false
})

describe('isEditableTarget', () => {
  it('accepts input / textarea / select elements', () => {
    expect(isEditableTarget(document.createElement('input'))).toBe(true)
    expect(isEditableTarget(document.createElement('textarea'))).toBe(true)
    expect(isEditableTarget(document.createElement('select'))).toBe(true)
  })

  it('accepts contenteditable hosts', () => {
    const el = document.createElement('div')
    el.setAttribute('contenteditable', 'true')
    expect(isEditableTarget(el)).toBe(true)
  })

  it('rejects plain elements and non-element targets', () => {
    expect(isEditableTarget(document.createElement('button'))).toBe(false)
    expect(isEditableTarget(document.createElement('div'))).toBe(false)
    expect(isEditableTarget(null)).toBe(false)
    expect(isEditableTarget(window)).toBe(false)
    expect(isEditableTarget(document.createTextNode('x'))).toBe(false)
  })
})

describe('isKeyboardShrink', () => {
  it('fires at or beyond the threshold', () => {
    expect(isKeyboardShrink(844, 844 - KEYBOARD_SHRINK_THRESHOLD_PX)).toBe(true)
    expect(isKeyboardShrink(844, 600)).toBe(true)
  })

  it('stays quiet for sub-threshold deltas (caret / IME bars, dvh jitter)', () => {
    expect(isKeyboardShrink(844, 844 - KEYBOARD_SHRINK_THRESHOLD_PX + 1)).toBe(false)
    expect(isKeyboardShrink(844, 844)).toBe(false)
    expect(isKeyboardShrink(844, 844 + 50)).toBe(false)
  })

  it('supports a custom threshold', () => {
    expect(isKeyboardShrink(800, 700, 50)).toBe(true)
    expect(isKeyboardShrink(800, 790, 50)).toBe(false)
  })

  it('degrades non-finite inputs to false', () => {
    expect(isKeyboardShrink(NaN, 600)).toBe(false)
    expect(isKeyboardShrink(844, NaN)).toBe(false)
    expect(isKeyboardShrink(Infinity, 600)).toBe(false)
  })
})

describe('initKeyboardTracking', () => {
  beforeEach(() => {
    keyboardVisible.value = false
  })

  it('is a no-op double init (single detach wins)', () => {
    initKeyboardTracking()
    expect(() => initKeyboardTracking()).not.toThrow()
    stopKeyboardTracking()
    // The second stop detaches nothing but must not throw either
    expect(() => stopKeyboardTracking()).not.toThrow()
  })

  it('ignores focus on non-editable elements', () => {
    initKeyboardTracking()
    const btn = document.createElement('button')
    document.body.appendChild(btn)
    btn.dispatchEvent(new FocusEvent('focusin', { bubbles: true, composed: true }))
    expect(keyboardVisible.value).toBe(false)
    btn.remove()
  })

  it('does not flag a keyboard from focus alone without a viewport shrink', () => {
    initKeyboardTracking()
    const input = document.createElement('textarea')
    document.body.appendChild(input)
    // happy-dom exposes no shrinking visualViewport; innerHeight - visual
    // height cannot cross the threshold, so the ref must stay false even
    // with an editable focus.
    input.dispatchEvent(new FocusEvent('focusin', { bubbles: true, composed: true }))
    expect(keyboardVisible.value).toBe(false)
    input.dispatchEvent(new FocusEvent('focusout', { bubbles: true, composed: true }))
    input.remove()
  })
})
