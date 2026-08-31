import { describe, it, expect } from 'vitest'
import {
  clampStep,
  stepNumber,
  stepMaxTokens,
  isUnlimitedMaxTokens,
  sliderFillPercent,
  formatParamValue,
  MAX_TOKENS_UNLIMITED,
  MAX_TOKENS_MIN,
  MAX_TOKENS_MAX,
  MAX_TOKENS_STEP,
  MAX_TOKENS_UNLIMITED_ENTER,
} from '../lib/chatState'

// Pure helpers behind the phone params sheet (design frame ⑤): slider/stepper
// clamping, the max-tokens unlimited (-1) mapping and the readout formats.

describe('clampStep', () => {
  it('snaps a value onto the step grid', () => {
    expect(clampStep(0.83, 0, 2, 0.05)).toBe(0.85)
    expect(clampStep(0.849, 0, 2, 0.05)).toBe(0.85)
    expect(clampStep(38, 1, 200, 1)).toBe(38)
  })

  it('clamps into [min, max]', () => {
    expect(clampStep(2.4, 0, 2, 0.05)).toBe(2)
    expect(clampStep(-1, 0, 2, 0.05)).toBe(0)
    expect(clampStep(500, 1, 200, 1)).toBe(200)
    expect(clampStep(0, 1, 200, 1)).toBe(1)
  })

  it('avoids binary-float dust from decimal steps', () => {
    // 1.1 + 0.05 in float math is 1.1500000000000001
    expect(stepNumber(1.1, 1, 1, 2, 0.05)).toBe(1.15)
    expect(stepNumber(1.15, -1, 1, 2, 0.05)).toBe(1.1)
  })

  it('degrades non-finite input to the lower bound', () => {
    expect(clampStep(Number.NaN, 0, 2, 0.05)).toBe(0)
    expect(clampStep(Number.POSITIVE_INFINITY, 0, 2, 0.05)).toBe(0)
    expect(clampStep(Number.NEGATIVE_INFINITY, 1, 200, 1)).toBe(1)
  })
})

describe('stepNumber', () => {
  it('moves by one step in both directions', () => {
    expect(stepNumber(40, 1, 1, 200, 1)).toBe(41)
    expect(stepNumber(40, -1, 1, 200, 1)).toBe(39)
  })

  it('stops at the range bounds without wrapping', () => {
    expect(stepNumber(1, -1, 1, 200, 1)).toBe(1)
    expect(stepNumber(200, 1, 1, 200, 1)).toBe(200)
    expect(stepNumber(2, 1, 1, 2, 0.05)).toBe(2)
  })
})

describe('stepMaxTokens (unlimited -1 sentinel)', () => {
  it('enters the finite range at the documented value from unlimited', () => {
    expect(MAX_TOKENS_UNLIMITED).toBe(-1)
    expect(stepMaxTokens(MAX_TOKENS_UNLIMITED, 1)).toBe(MAX_TOKENS_UNLIMITED_ENTER)
    expect(MAX_TOKENS_UNLIMITED_ENTER).toBe(4096)
  })

  it('maps a step below the minimum back to unlimited', () => {
    expect(stepMaxTokens(MAX_TOKENS_MIN, -1)).toBe(MAX_TOKENS_UNLIMITED)
    expect(stepMaxTokens(MAX_TOKENS_UNLIMITED, -1)).toBe(MAX_TOKENS_UNLIMITED)
  })

  it('steps within the finite range', () => {
    expect(stepMaxTokens(256, 1)).toBe(512)
    expect(stepMaxTokens(4096, -1)).toBe(3840)
    expect(stepMaxTokens(4096, 1)).toBe(4352)
    expect(MAX_TOKENS_STEP).toBe(256)
  })

  it('clamps at the maximum without escaping the range', () => {
    expect(stepMaxTokens(MAX_TOKENS_MAX, 1)).toBe(MAX_TOKENS_MAX)
    expect(stepMaxTokens(MAX_TOKENS_MAX, -1)).toBe(MAX_TOKENS_MAX - MAX_TOKENS_STEP)
  })
})

describe('isUnlimitedMaxTokens', () => {
  it('flags the sentinel and sub-minimum values', () => {
    expect(isUnlimitedMaxTokens(-1)).toBe(true)
    expect(isUnlimitedMaxTokens(0)).toBe(true)
    expect(isUnlimitedMaxTokens(256)).toBe(false)
    expect(isUnlimitedMaxTokens(4096)).toBe(false)
  })
})

describe('sliderFillPercent', () => {
  it('maps a value into 0..100 of the track', () => {
    expect(sliderFillPercent(0.8, 0, 2)).toBe(40)
    expect(sliderFillPercent(0, 0, 2)).toBe(0)
    expect(sliderFillPercent(2, 0, 2)).toBe(100)
  })

  it('clamps out-of-range and degrades degenerate ranges', () => {
    expect(sliderFillPercent(-0.5, 0, 2)).toBe(0)
    expect(sliderFillPercent(3, 0, 2)).toBe(100)
    expect(sliderFillPercent(1, 1, 1)).toBe(0)
    expect(sliderFillPercent(Number.NaN, 0, 2)).toBe(0)
  })
})

describe('formatParamValue', () => {
  it('renders the two-decimal slider readout', () => {
    expect(formatParamValue(0.8)).toBe('0.80')
    expect(formatParamValue(1)).toBe('1.00')
    expect(formatParamValue(0.95)).toBe('0.95')
  })

  it('degrades non-finite values', () => {
    expect(formatParamValue(Number.NaN)).toBe('0.00')
  })
})
