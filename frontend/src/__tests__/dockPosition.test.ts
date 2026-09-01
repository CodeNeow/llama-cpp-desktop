// @vitest-environment happy-dom
// happy-dom provides the localStorage implementation the storage helpers use.
import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import {
  isBeyondDragThreshold,
  nearestSide,
  sideLeftX,
  anchorTopLeft,
  clampTopPx,
  topToNorm,
  normToTop,
  resolvePosition,
  translateForPosition,
  laneFor,
  loadStoredPosition,
  saveStoredPosition,
  CHAT_COMPOSER_BAND_DESKTOP,
  CHAT_COMPOSER_BAND_MOBILE,
  DOCK_POSITION_KEY,
  type DockLayoutMetrics,
} from '../lib/dockPosition'

// Standard desktop-tier metrics: 1280x800 viewport, 48x32 pill, title bar
// 0px -> minTop 8, desktop bottom gap 16, desktop anchor offset 29, desktop
// edge gap 16. The safe vertical band is therefore [8, 800 - 16 - 32] =
// [8, 752].
function layout(over: Partial<DockLayoutMetrics> = {}): DockLayoutMetrics {
  return {
    viewportW: 1280,
    viewportH: 800,
    pillW: 48,
    pillH: 32,
    minTop: 8,
    clampBottomGap: 16,
    anchorBottomOffset: 29,
    edgeGap: 16,
    ...over,
  }
}

afterEach(() => {
  localStorage.clear()
})

// ─── Click-vs-drag threshold ─────────────────────────────────────────────────

describe('isBeyondDragThreshold', () => {
  it('treats travel up to and including 6px as a click', () => {
    expect(isBeyondDragThreshold(0, 0)).toBe(false)
    expect(isBeyondDragThreshold(6, 0)).toBe(false)
    expect(isBeyondDragThreshold(0, -6)).toBe(false)
    expect(isBeyondDragThreshold(3, 5)).toBe(false) // hypot ≈ 5.83
  })

  it('treats travel beyond 6px as a drag (including diagonals)', () => {
    expect(isBeyondDragThreshold(6.1, 0)).toBe(true)
    expect(isBeyondDragThreshold(0, 7)).toBe(true)
    expect(isBeyondDragThreshold(4, 5)).toBe(true) // hypot ≈ 6.40
    expect(isBeyondDragThreshold(-50, 10)).toBe(true)
  })

  it('rejects non-finite deltas as non-drags', () => {
    expect(isBeyondDragThreshold(NaN, 0)).toBe(false)
    expect(isBeyondDragThreshold(1, Infinity)).toBe(false)
  })
})

// ─── Horizontal snap ─────────────────────────────────────────────────────────

describe('nearestSide', () => {
  it('picks the nearest edge from the pill center x', () => {
    expect(nearestSide(100, 1280)).toBe('left')
    expect(nearestSide(1200, 1280)).toBe('right')
  })

  it('breaks exact mid-width ties toward right (legacy default)', () => {
    expect(nearestSide(640, 1280)).toBe('right')
    expect(nearestSide(639.9, 1280)).toBe('left')
  })

  it('degrades to right for invalid input', () => {
    expect(nearestSide(NaN, 1280)).toBe('right')
    expect(nearestSide(100, 0)).toBe('right')
  })
})

describe('sideLeftX', () => {
  it('snaps the left side to the 16px edge gap', () => {
    expect(sideLeftX('left', 1280, 48)).toBe(16)
  })

  it('mirrors the right side 16px off the right edge', () => {
    expect(sideLeftX('right', 1280, 48)).toBe(1216)
  })

  it('degrades to the left-edge spot for a degenerate pill box', () => {
    expect(sideLeftX('right', 1280, 0)).toBe(16)
    expect(sideLeftX('right', NaN, 48)).toBe(16)
  })
})

describe('sideLeftX with a custom edge gap (phone tier hugs the screen edge)', () => {
  it('snaps flush to both edges at gap 0', () => {
    expect(sideLeftX('left', 390, 44, 0)).toBe(0)
    expect(sideLeftX('right', 390, 44, 0)).toBe(346) // 390 - 0 - 44
  })

  it('keeps the pill inside the viewport at gap 0', () => {
    expect(sideLeftX('right', 390, 44, 0)).toBeGreaterThanOrEqual(0)
  })

  it('degrades degenerate inputs to the provided gap', () => {
    expect(sideLeftX('right', 390, 0, 0)).toBe(0)
    expect(sideLeftX('right', NaN, 44, 0)).toBe(0)
  })
})

describe('anchorTopLeft / resolvePosition with the phone edge gap', () => {
  it('anchors the CSS spot flush to the right screen edge at gap 0', () => {
    const phone = layout({ viewportW: 390, pillW: 44, edgeGap: 0 })
    expect(anchorTopLeft(phone).x).toBe(346)
  })

  it('restores a stored position flush to the hugging edge at gap 0', () => {
    const phone = layout({ viewportW: 390, viewportH: 844, pillW: 44, pillH: 44, edgeGap: 0 })
    expect(resolvePosition({ side: 'right', yNorm: 0 }, phone).left).toBe(346)
    expect(resolvePosition({ side: 'left', yNorm: 0 }, phone).left).toBe(0)
  })
})

// ─── Chat composer-band lane decision ────────────────────────────────────────

describe('laneFor', () => {
  const vh = 800

  it('returns the capsule side while its bottom edge reaches the composer band', () => {
    // Desktop band 130 on an 800px viewport: threshold = 800 - 130 = 670.
    expect(laneFor('right', 800, vh, CHAT_COMPOSER_BAND_DESKTOP)).toBe('right')
    expect(laneFor('right', 700, vh, CHAT_COMPOSER_BAND_DESKTOP)).toBe('right')
    expect(laneFor('left', 690, vh, CHAT_COMPOSER_BAND_DESKTOP)).toBe('left')
  })

  it('is inclusive at the exact band threshold', () => {
    expect(laneFor('right', vh - CHAT_COMPOSER_BAND_DESKTOP, vh, CHAT_COMPOSER_BAND_DESKTOP)).toBe('right')
    expect(laneFor('left', vh - CHAT_COMPOSER_BAND_DESKTOP, vh, CHAT_COMPOSER_BAND_DESKTOP)).toBe('left')
  })

  it("returns 'none' once the capsule parks above the band", () => {
    expect(laneFor('right', vh - CHAT_COMPOSER_BAND_DESKTOP - 0.01, vh, CHAT_COMPOSER_BAND_DESKTOP)).toBe('none')
    expect(laneFor('right', 400, vh, CHAT_COMPOSER_BAND_DESKTOP)).toBe('none')
    expect(laneFor('left', 0, vh, CHAT_COMPOSER_BAND_DESKTOP)).toBe('none')
  })

  it('honors the phone band constant (nav band + composer + margin)', () => {
    const ph = 844
    expect(laneFor('left', ph, ph, CHAT_COMPOSER_BAND_MOBILE)).toBe('left')
    expect(laneFor('right', ph - CHAT_COMPOSER_BAND_MOBILE, ph, CHAT_COMPOSER_BAND_MOBILE)).toBe('right')
    expect(laneFor('right', ph - CHAT_COMPOSER_BAND_MOBILE - 1, ph, CHAT_COMPOSER_BAND_MOBILE)).toBe('none')
  })

  it("degrades to 'none' for invalid numbers", () => {
    expect(laneFor('right', NaN, vh, CHAT_COMPOSER_BAND_DESKTOP)).toBe('none')
    expect(laneFor('right', Infinity, vh, CHAT_COMPOSER_BAND_DESKTOP)).toBe('none')
    expect(laneFor('right', -Infinity, vh, CHAT_COMPOSER_BAND_DESKTOP)).toBe('none')
    expect(laneFor('right', 700, 0, CHAT_COMPOSER_BAND_DESKTOP)).toBe('none')
    expect(laneFor('right', 700, -800, CHAT_COMPOSER_BAND_DESKTOP)).toBe('none')
    expect(laneFor('right', 700, vh, 0)).toBe('none')
    expect(laneFor('right', 700, vh, -130)).toBe('none')
    expect(laneFor('right', 700, vh, NaN)).toBe('none')
  })

  it("degrades to 'none' for an invalid side", () => {
    expect(laneFor('top' as never, 800, vh, CHAT_COMPOSER_BAND_DESKTOP)).toBe('none')
    expect(laneFor(undefined as never, 800, vh, CHAT_COMPOSER_BAND_DESKTOP)).toBe('none')
  })
})

// ─── Vertical clamp + normalization round-trip ───────────────────────────────

describe('clampTopPx', () => {
  it('keeps in-band tops unchanged', () => {
    expect(clampTopPx(400, layout())).toBe(400)
    expect(clampTopPx(8, layout())).toBe(8)
    expect(clampTopPx(752, layout())).toBe(752)
  })

  it('clamps against the top gap (title bar + 8px)', () => {
    expect(clampTopPx(0, layout())).toBe(8)
    expect(clampTopPx(-120, layout())).toBe(8)
  })

  it('clamps against the bottom safe band', () => {
    expect(clampTopPx(9999, layout())).toBe(752)
    expect(clampTopPx(753, layout())).toBe(752)
  })

  it('honors a custom clampBottomGap (phone tier: nav band + 10px)', () => {
    const phone = layout({ viewportH: 844, pillH: 44, clampBottomGap: 68 })
    // maxTop = 844 - 68 - 44 = 732
    expect(clampTopPx(800, phone)).toBe(732)
  })

  it('falls back to minTop when the band is degenerate', () => {
    // Viewport shorter than chrome + pill: maxTop < minTop
    expect(clampTopPx(400, layout({ viewportH: 40, pillH: 100 }))).toBe(8)
  })
})

describe('topToNorm / normToTop round-trip', () => {
  it('maps band edges to 0 and 1', () => {
    expect(topToNorm(8, layout())).toBe(0)
    expect(topToNorm(752, layout())).toBe(1)
    expect(normToTop(0, layout())).toBe(8)
    expect(normToTop(1, layout())).toBe(752)
  })

  it('maps mid-band positions proportionally', () => {
    expect(topToNorm(380, layout())).toBeCloseTo(0.5) // (380-8)/744
    expect(normToTop(0.5, layout())).toBeCloseTo(380)
  })

  it('round-trips arbitrary in-band tops exactly', () => {
    for (const top of [8, 64, 200, 381, 500, 751, 752]) {
      expect(normToTop(topToNorm(top, layout()), layout())).toBeCloseTo(top, 6)
    }
  })

  it('clamps out-of-range norms and degrades degenerate bands to minTop', () => {
    expect(normToTop(1.7, layout())).toBe(752)
    expect(normToTop(-0.2, layout())).toBe(8)
    expect(normToTop(NaN, layout())).toBe(8)
    expect(topToNorm(400, layout({ viewportH: 40, pillH: 100 }))).toBe(0)
  })
})

// ─── Position resolution + transform translation ─────────────────────────────

describe('resolvePosition', () => {
  it('snaps horizontally to the stored side and re-fits vertically', () => {
    expect(resolvePosition({ side: 'left', yNorm: 0 }, layout())).toEqual({ left: 16, top: 8, side: 'left' })
    expect(resolvePosition({ side: 'right', yNorm: 1 }, layout())).toEqual({ left: 1216, top: 752, side: 'right' })
  })
})

describe('translateForPosition', () => {
  it('keeps the legacy anchor spot (zero translate) when nothing is stored', () => {
    expect(translateForPosition(null, layout())).toEqual({ x: 0, y: 0 })
  })

  it('computes the offset from the CSS anchor (right edge default)', () => {
    // Anchor top-left: x = 1280-16-48 = 1216, y = 800-29-32 = 739.
    const anchor = anchorTopLeft(layout())
    expect(anchor).toEqual({ x: 1216, y: 739 })
    // Stored bottom-right band edge (top 752) sits 13px BELOW the 739 anchor
    // (the legacy bottom: 29px spot is 13px above the 16px clamp floor):
    // translate y = 752 - 739 = 13; x is unchanged (still hugging right).
    expect(translateForPosition({ side: 'right', yNorm: 1 }, layout())).toEqual({ x: 0, y: 13 })
    // Stored top-left: x = 16 - 1216, y = 8 - 739.
    expect(translateForPosition({ side: 'left', yNorm: 0 }, layout())).toEqual({ x: -1200, y: -731 })
  })
})

// ─── localStorage position memory ────────────────────────────────────────────

describe('stored position persistence', () => {
  beforeEach(() => {
    localStorage.clear()
  })

  it('returns null when nothing is stored', () => {
    expect(loadStoredPosition()).toBeNull()
  })

  it('round-trips a saved position', () => {
    saveStoredPosition({ side: 'left', yNorm: 0.42 })
    expect(loadStoredPosition()).toEqual({ side: 'left', yNorm: 0.42 })
  })

  it('degrades corrupted JSON to null (legacy default spot)', () => {
    localStorage.setItem(DOCK_POSITION_KEY, '{not json')
    expect(loadStoredPosition()).toBeNull()
    localStorage.setItem(DOCK_POSITION_KEY, 'null')
    expect(loadStoredPosition()).toBeNull()
    localStorage.setItem(DOCK_POSITION_KEY, '"string"')
    expect(loadStoredPosition()).toBeNull()
  })

  it('rejects forged shapes: unknown side, non-finite fraction', () => {
    localStorage.setItem(DOCK_POSITION_KEY, JSON.stringify({ side: 'top', yNorm: 0.5 }))
    expect(loadStoredPosition()).toBeNull()
    localStorage.setItem(DOCK_POSITION_KEY, JSON.stringify({ yNorm: 0.5 }))
    expect(loadStoredPosition()).toBeNull()
    localStorage.setItem(DOCK_POSITION_KEY, JSON.stringify({ side: 'left', yNorm: '0.5' }))
    expect(loadStoredPosition()).toBeNull()
    // JSON has no NaN/Infinity: they serialize to null -> rejected
    localStorage.setItem(DOCK_POSITION_KEY, '{"side":"left","yNorm":null}')
    expect(loadStoredPosition()).toBeNull()
  })

  it('clamps out-of-range but finite fractions back into 0..1', () => {
    localStorage.setItem(DOCK_POSITION_KEY, JSON.stringify({ side: 'right', yNorm: 1.7 }))
    expect(loadStoredPosition()).toEqual({ side: 'right', yNorm: 1 })
    localStorage.setItem(DOCK_POSITION_KEY, JSON.stringify({ side: 'right', yNorm: -0.3 }))
    expect(loadStoredPosition()).toEqual({ side: 'right', yNorm: 0 })
  })
})
