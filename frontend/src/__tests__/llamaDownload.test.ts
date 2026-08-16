import { describe, it, expect } from 'vitest'
import { downloadVisibility, initialDownloadAction } from '../lib/llamaDownload'

describe('downloadVisibility', () => {
  it('idle shows button group, hides progress area', () => {
    const v = downloadVisibility('idle')
    expect(v.showButtons).toBe(true)
    expect(v.showProgress).toBe(false)
  })

  it('error shows both button group and progress area (progress area carries error info and retry button)', () => {
    const v = downloadVisibility('error')
    expect(v.showButtons).toBe(true)
    expect(v.showProgress).toBe(true)
  })

  it('fetching/downloading/paused/extracting shows progress area, hides button group', () => {
    for (const status of ['fetching', 'downloading', 'paused', 'extracting']) {
      const v = downloadVisibility(status)
      expect(v.showButtons, `status=${status} should hide button group`).toBe(false)
      expect(v.showProgress, `status=${status} should show progress area`).toBe(true)
    }
  })

  it('done shows progress area (download complete notice), hides button group', () => {
    const v = downloadVisibility('done')
    expect(v.showButtons).toBe(false)
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
  // regression: previous poll branch missed 'paused'; pausing the download and switching pages,
  // returning to home reset status to idle and the paused progress UI was lost, only the
  // "Download llama.cpp" button remained; now 'paused' should also return poll so the home page
  // restores the paused progress area with resume/stop buttons
  it('downloading/fetching/paused/extracting returns poll (resume polling during download), independent of installed param', () => {
    for (const status of ['downloading', 'fetching', 'paused', 'extracting']) {
      expect(initialDownloadAction(status, false), `status=${status} should poll when not installed`).toBe('poll')
      expect(initialDownloadAction(status, true), `status=${status} should poll when installed`).toBe('poll')
    }
  })

  it('done and not installed returns refresh (download complete but not detected, refresh system info)', () => {
    expect(initialDownloadAction('done', false)).toBe('refresh')
  })

  it('done and installed returns none (onMounted fetchSystemInfo already covers installed case)', () => {
    expect(initialDownloadAction('done', true)).toBe('none')
  })

  // regression: previous checkInitialDownloadStatus lacked error branch; download fails during tab switch
  // returning to home page resets status to idle, error info lost, only "Download" button remains; now should return
  // showError so UI displays error info and retry button
  it('error returns showError (download fails during tab switch, returning home shows error and retry button)', () => {
    expect(initialDownloadAction('error', false)).toBe('showError')
    expect(initialDownloadAction('error', true)).toBe('showError')
  })

  it('idle returns none (no download to handle)', () => {
    expect(initialDownloadAction('idle', false)).toBe('none')
    expect(initialDownloadAction('idle', true)).toBe('none')
  })
})
