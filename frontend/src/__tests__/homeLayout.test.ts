import { describe, it, expect } from 'vitest'
import { pairsResidentWithSystemCard } from '../lib/homeLayout'

describe('pairsResidentWithSystemCard', () => {
  // Portrait-band pairing of the resident-model card with the system summary
  // card: both are compact summary strips, so they share one two-column row
  // only when exactly one model is resident.
  it('pairs when exactly one model is resident', () => {
    expect(pairsResidentWithSystemCard(1)).toBe(true)
  })

  it('does not pair with zero residents (first-use stack stays unchanged)', () => {
    expect(pairsResidentWithSystemCard(0)).toBe(false)
  })

  it('does not pair with multiple residents (each card keeps full width)', () => {
    expect(pairsResidentWithSystemCard(2)).toBe(false)
    expect(pairsResidentWithSystemCard(5)).toBe(false)
  })

  it('treats negative counts as no residents', () => {
    expect(pairsResidentWithSystemCard(-1)).toBe(false)
  })
})
