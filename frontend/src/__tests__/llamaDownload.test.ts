import { describe, it, expect } from 'vitest'
import { downloadVisibility, initialDownloadAction, isCudartAsset, packageRows } from '../lib/llamaDownload'
import type { InitialDownloadAction } from '../lib/llamaDownload'
import { LLAMA_CPP_DOWNLOAD_STATUSES } from '../lib/downloadStatus'
import type { LlamaCppDownloadStatus } from '../lib/downloadStatus'

// Tables keyed by LlamaCppDownloadStatus make an unlisted new backend status a compile
// error (vue-tsc fails on the missing Record key), and iterating the canonical
// LLAMA_CPP_DOWNLOAD_STATUSES list makes a wrongly-listed status a runtime test failure.

describe('downloadVisibility', () => {
  // Expected [showButtons, showProgress] per canonical status:
  // - idle: only the button group; error: both blocks (progress area carries the
  //   error info and retry button); every non-idle status shows the progress area.
  const visibilityTable: Record<LlamaCppDownloadStatus, [boolean, boolean]> = {
    idle: [true, false],
    fetching: [false, true],
    downloading: [false, true],
    paused: [false, true],
    extracting: [false, true],
    done: [false, true],
    error: [true, true],
  }

  it('maps every canonical llama.cpp download status to the expected visibility pair', () => {
    for (const status of LLAMA_CPP_DOWNLOAD_STATUSES) {
      const [showButtons, showProgress] = visibilityTable[status]
      const v = downloadVisibility(status)
      expect(v.showButtons, `status=${status} showButtons`).toBe(showButtons)
      expect(v.showProgress, `status=${status} showProgress`).toBe(showProgress)
    }
  })

  it('error shows both button group and progress area (progress area carries error info and retry button)', () => {
    const v = downloadVisibility('error')
    expect(v.showButtons).toBe(true)
    expect(v.showProgress).toBe(true)
  })

  // regression: progress area visibility depends only on download status, function signature has no custom path param —
  // after setting custom llama.cpp dir, progress area still renders; no "buttons disappear with no feedback"
  it('progress area visibility independent of custom path (no path param, semantically unaffected)', () => {
    expect(downloadVisibility('fetching').showProgress).toBe(true)
    expect(downloadVisibility('downloading').showProgress).toBe(true)
    expect(downloadVisibility('extracting').showProgress).toBe(true)
  })
})

describe('initialDownloadAction', () => {
  // Expected action per canonical status for each installed flag; 'poll' statuses are
  // independent of installed (a download in flight cannot have finished installing).
  const actionNotInstalled: Record<LlamaCppDownloadStatus, InitialDownloadAction> = {
    idle: 'none',
    fetching: 'poll',
    downloading: 'poll',
    paused: 'poll',
    extracting: 'poll',
    done: 'refresh',
    error: 'showError',
  }
  const actionInstalled: Record<LlamaCppDownloadStatus, InitialDownloadAction> = {
    idle: 'none',
    fetching: 'poll',
    downloading: 'poll',
    paused: 'poll',
    extracting: 'poll',
    done: 'none',
    error: 'showError',
  }

  it('maps every canonical llama.cpp download status for both installed flags', () => {
    for (const status of LLAMA_CPP_DOWNLOAD_STATUSES) {
      expect(initialDownloadAction(status, false), `status=${status} installed=false`).toBe(actionNotInstalled[status])
      expect(initialDownloadAction(status, true), `status=${status} installed=true`).toBe(actionInstalled[status])
    }
  })

  // regression: previous poll branch missed 'paused'; pausing the download and switching pages,
  // returning to home reset status to idle and the paused progress UI was lost, only the
  // "Download llama.cpp" button remained; now 'paused' should also return poll so the home page
  // restores the paused progress area with resume/stop buttons
  it('paused returns poll for both installed flags (paused progress UI restored after navigating back)', () => {
    expect(initialDownloadAction('paused', false)).toBe('poll')
    expect(initialDownloadAction('paused', true)).toBe('poll')
  })

  // regression: previous checkInitialDownloadStatus lacked error branch; download fails during tab switch
  // returning to home page resets status to idle, error info lost, only "Download" button remains; now should return
  // showError so UI displays error info and retry button
  it('error returns showError (download fails during tab switch, returning home shows error and retry button)', () => {
    expect(initialDownloadAction('error', false)).toBe('showError')
    expect(initialDownloadAction('error', true)).toBe('showError')
  })
})

describe('isCudartAsset', () => {
  it('recognizes the cudart runtime asset name case-insensitively', () => {
    expect(isCudartAsset('cudart-llama-bin-win-cuda-12.4-x64.zip')).toBe(true)
    expect(isCudartAsset('CUDART-llama-bin-win-cuda-12.4-x64.zip')).toBe(true)
  })

  it('does not match main-program assets', () => {
    expect(isCudartAsset('llama-b6084-bin-win-cuda-12.4-x64.zip')).toBe(false)
    expect(isCudartAsset('llama-b6084-bin-win-vulkan-x64.zip')).toBe(false)
    expect(isCudartAsset('')).toBe(false)
  })
})

describe('packageRows', () => {
  it('main asset downloading: a single row carrying the combined progress', () => {
    const rows = packageRows('llama-b6084-bin-win-cuda-12.4-x64.zip', 300, 1000, 0)
    expect(rows).toHaveLength(1)
    expect(rows[0]).toEqual({ id: 'main', active: true, done: false, progress: 30 })
  })

  it('cudart asset downloading: main row done, cudart row scoped to its own byte share', () => {
    // total 1000 = main 600 + cudart 400; 700 cumulative bytes means cudart is at 25%
    const rows = packageRows('cudart-llama-bin-win-cuda-12.4-x64.zip', 700, 1000, 600)
    expect(rows).toHaveLength(2)
    expect(rows[0]).toEqual({ id: 'main', active: false, done: true, progress: 100 })
    expect(rows[1]).toEqual({ id: 'cudart', active: true, done: false, progress: 25 })
  })

  it('completed download with cudart: both rows fully done', () => {
    const rows = packageRows('cudart-llama-bin-win-cuda-12.4-x64.zip', 1000, 1000, 600)
    expect(rows[0].done).toBe(true)
    expect(rows[1].done).toBe(false) // active row, but progress clamps to 100
    expect(rows[1].progress).toBe(100)
  })

  it('mid-download remount with mainBytes unset: cudart share falls back to the combined ratio', () => {
    const rows = packageRows('cudart-llama-bin-win-cuda-12.4-x64.zip', 700, 1000, 0)
    expect(rows[1].progress).toBe(70)
  })

  it('guards against zero and negative byte shares', () => {
    expect(packageRows('llama-b6084-bin-win-cuda-12.4-x64.zip', 0, 0, 0)[0].progress).toBe(0)
    const rows = packageRows('cudart-llama-bin-win-cuda-12.4-x64.zip', 500, 1000, 1000)
    expect(rows[1].progress).toBe(0) // total - mainBytes = 0
    const clamped = packageRows('cudart-llama-bin-win-cuda-12.4-x64.zip', 1200, 1000, 600)
    expect(clamped[1].progress).toBe(100) // clamped above 100
  })
})
