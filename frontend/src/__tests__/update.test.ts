import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import {
  updateState,
  checkForUpdate,
  startUpdateDownload,
  shouldAutoCheck,
  closeUpdateModal,
  CHECK_INTERVAL_MS,
} from '../lib/update'
import {
  checkForUpdate as checkForUpdateBackend,
  startUpdateDownload as startUpdateDownloadBackend,
  getUpdateDownloadStatus,
} from '../wails'

// mock Wails bridge: window.go is injected by Wails runtime only, unavailable in test env
vi.mock('../wails', () => ({
  checkForUpdate: vi.fn(),
  startUpdateDownload: vi.fn(),
  getUpdateDownloadStatus: vi.fn(),
}))

const mockCheckForUpdate = vi.mocked(checkForUpdateBackend)
const mockStartUpdateDownload = vi.mocked(startUpdateDownloadBackend)
const mockGetStatus = vi.mocked(getUpdateDownloadStatus)

function resetState() {
  updateState.checking = false
  updateState.result = null
  updateState.download = null
  updateState.showModal = false
  updateState.error = ''
}

describe('lib/update', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    vi.useRealTimers()
    localStorage.clear()
    resetState()
  })

  afterEach(() => {
    closeUpdateModal() // cleanup polling timer
    vi.useRealTimers()
  })

  it('shouldAutoCheck: never checked returns true', () => {
    expect(shouldAutoCheck()).toBe(true)
  })

  it('shouldAutoCheck: less than 48h since last check returns false', () => {
    localStorage.setItem('llama-desktop-last-update-check', String(Date.now() - 60 * 60 * 1000))
    expect(shouldAutoCheck()).toBe(false)
  })

  it('shouldAutoCheck: more than 48h since last check returns true', () => {
    localStorage.setItem('llama-desktop-last-update-check', String(Date.now() - CHECK_INTERVAL_MS - 1000))
    expect(shouldAutoCheck()).toBe(true)
  })

  it('shouldAutoCheck: falls back to old key when new key missing (llama-gui rename migration); old install throttle not reset', () => {
    localStorage.setItem('llama-gui-last-update-check', String(Date.now() - 60 * 60 * 1000))
    expect(shouldAutoCheck()).toBe(false)
  })

  it('checkForUpdate discovers new version, writes check time, and opens update modal', async () => {
    mockCheckForUpdate.mockResolvedValue({
      hasUpdate: true,
      version: 'v0.2.0',
      notes: '新增功能',
      published: '2026-08-10T00:00:00Z',
    })

    await checkForUpdate()

    expect(updateState.result?.hasUpdate).toBe(true)
    expect(updateState.result?.version).toBe('v0.2.0')
    expect(updateState.showModal).toBe(true)
    expect(updateState.checking).toBe(false)
    // refresh last check time after completion to avoid repeated auto-check on next launch
    expect(localStorage.getItem('llama-desktop-last-update-check')).not.toBeNull()
  })

  it('checkForUpdate no new version: no modal but records check time', async () => {
    mockCheckForUpdate.mockResolvedValue({ hasUpdate: false, version: 'v0.1.0', notes: '', published: '' })

    await checkForUpdate()

    expect(updateState.showModal).toBe(false)
    expect(localStorage.getItem('llama-desktop-last-update-check')).not.toBeNull()
  })

  it('checkForUpdate failure sets error silently, no modal', async () => {
    mockCheckForUpdate.mockRejectedValue(new Error('network down'))

    await checkForUpdate()

    expect(updateState.error).toBe('检查更新失败，请确认网络后重试')
    expect(updateState.showModal).toBe(false)
    expect(updateState.checking).toBe(false)
  })

  it('startUpdateDownload starts download and polls progress, stops after completion', async () => {
    vi.useFakeTimers()
    mockCheckForUpdate.mockResolvedValue({ hasUpdate: true, version: 'v0.2.0', notes: '', published: '' })
    mockStartUpdateDownload.mockResolvedValue(undefined)
    mockGetStatus.mockResolvedValue({
      status: 'done', progress: 100, total: 100, downloaded: 100,
      version: 'v0.2.0', filePath: 'C:/app/llama-desktop-portable-v0.2.0.exe', error: '', kind: 'portable',
    })

    await checkForUpdate()
    startUpdateDownload()

    expect(mockStartUpdateDownload).toHaveBeenCalledWith('v0.2.0')
    expect(updateState.download?.status).toBe('downloading')

    await vi.advanceTimersByTimeAsync(1100) // trigger one poll
    expect(updateState.download?.status).toBe('done')
    expect(updateState.download?.filePath).toContain('llama-desktop-portable-v0.2.0.exe')

    // polling should stop after completion (advancing time should not repeat fetches)
    const callsAfterDone = mockGetStatus.mock.calls.length
    await vi.advanceTimersByTimeAsync(2200)
    expect(mockGetStatus.mock.calls.length).toBe(callsAfterDone)
  })

  it('closeUpdateModal closes modal and stops polling', () => {
    updateState.showModal = true
    closeUpdateModal()
    expect(updateState.showModal).toBe(false)
  })
})
