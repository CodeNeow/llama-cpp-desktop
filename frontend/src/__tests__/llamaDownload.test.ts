import { describe, it, expect } from 'vitest'
import { downloadVisibility, initialDownloadAction } from '../lib/llamaDownload'
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
