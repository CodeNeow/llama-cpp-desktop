// @vitest-environment happy-dom
// isEditableTarget classifies real DOM elements, so the suite needs a DOM.
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import {
  keyboardVisible,
  isEditableTarget,
  isKeyboardShrink,
  initKeyboardTracking,
  stopKeyboardTracking,
  pinRootScroll,
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

describe('pinRootScroll / scroll re-pin', () => {
  /** Swap in a stub visualViewport (happy-dom exposes none that can shrink); returns a restore fn. */
  function stubVisualViewport(height: number): { fake: EventTarget; restore: () => void } {
    const original = Object.getOwnPropertyDescriptor(window, 'visualViewport')
    const fake = new EventTarget() as EventTarget & { height: number }
    fake.height = height
    Object.defineProperty(window, 'visualViewport', { value: fake, configurable: true })
    return {
      fake,
      restore: () => {
        if (original) Object.defineProperty(window, 'visualViewport', original)
        else delete (window as { visualViewport?: unknown }).visualViewport
      },
    }
  }

  it('scrolls the root to the origin', () => {
    const scrollTo = vi.spyOn(window, 'scrollTo').mockImplementation(() => {})
    pinRootScroll()
    expect(scrollTo).toHaveBeenCalledWith(0, 0)
    scrollTo.mockRestore()
  })

  it('re-pins on focus and viewport resizes only while the keyboard heuristic holds', () => {
    const scrollTo = vi.spyOn(window, 'scrollTo').mockImplementation(() => {})
    // happy-dom default innerHeight (768): stub starts at full height (no
    // shrink yet) and is later narrowed by 168px, crossing the 120px
    // keyboard threshold.
    const stubbed = stubVisualViewport(768)
    try {
      initKeyboardTracking()
      const input = document.createElement('textarea')
      document.body.appendChild(input)

      // Editable focus without a shrink: no keyboard, so the root scroll
      // must stay untouched (desktop window resizing never pins).
      input.dispatchEvent(new FocusEvent('focusin', { bubbles: true, composed: true }))
      expect(keyboardVisible.value).toBe(false)
      expect(scrollTo).not.toHaveBeenCalled()

      // The shrink arrives while the editable focus is still held (IME show):
      // the ref flips open and the focus-scroll raise gets pinned back.
      ;(stubbed.fake as EventTarget & { height: number }).height = 600
      stubbed.fake.dispatchEvent(new Event('resize'))
      expect(keyboardVisible.value).toBe(true)
      expect(scrollTo).toHaveBeenCalledTimes(1)

      // Refocusing while the keyboard is open re-pins too (focusin path).
      input.dispatchEvent(new FocusEvent('focusin', { bubbles: true, composed: true }))
      expect(scrollTo).toHaveBeenCalledTimes(2)

      // Every further resize inside the keyboard window re-pins (Chromium may
      // raise the root scroller again on each IME-driven viewport change).
      stubbed.fake.dispatchEvent(new Event('resize'))
      expect(scrollTo).toHaveBeenCalledTimes(3)
    } finally {
      scrollTo.mockRestore()
      stubbed.restore()
      stopKeyboardTracking()
    }
  })
})
