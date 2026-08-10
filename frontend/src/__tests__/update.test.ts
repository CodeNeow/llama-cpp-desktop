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

// mock Wails 桥接层：window.go 仅由 Wails 运行时注入，单测环境不可用
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
    closeUpdateModal() // 清理轮询定时器
    vi.useRealTimers()
  })

  it('shouldAutoCheck：从未检查过返回 true', () => {
    expect(shouldAutoCheck()).toBe(true)
  })

  it('shouldAutoCheck：距上次检查不足 48 小时返回 false', () => {
    localStorage.setItem('llama-gui-last-update-check', String(Date.now() - 60 * 60 * 1000))
    expect(shouldAutoCheck()).toBe(false)
  })

  it('shouldAutoCheck：距上次检查超过 48 小时返回 true', () => {
    localStorage.setItem('llama-gui-last-update-check', String(Date.now() - CHECK_INTERVAL_MS - 1000))
    expect(shouldAutoCheck()).toBe(true)
  })

  it('checkForUpdate 发现新版本时写入检查时间并弹出更新窗口', async () => {
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
    // 检查完成后刷新最近检查时间，避免下次启动重复自动检查
    expect(localStorage.getItem('llama-gui-last-update-check')).not.toBeNull()
  })

  it('checkForUpdate 无新版本时不弹窗但记录检查时间', async () => {
    mockCheckForUpdate.mockResolvedValue({ hasUpdate: false, version: 'v0.1.0', notes: '', published: '' })

    await checkForUpdate()

    expect(updateState.showModal).toBe(false)
    expect(localStorage.getItem('llama-gui-last-update-check')).not.toBeNull()
  })

  it('checkForUpdate 失败时静默置错误，不弹窗', async () => {
    mockCheckForUpdate.mockRejectedValue(new Error('network down'))

    await checkForUpdate()

    expect(updateState.error).toBe('检查更新失败，请确认网络后重试')
    expect(updateState.showModal).toBe(false)
    expect(updateState.checking).toBe(false)
  })

  it('startUpdateDownload 启动下载并轮询进度，完成后停止轮询', async () => {
    vi.useFakeTimers()
    mockCheckForUpdate.mockResolvedValue({ hasUpdate: true, version: 'v0.2.0', notes: '', published: '' })
    mockStartUpdateDownload.mockResolvedValue(undefined)
    mockGetStatus.mockResolvedValue({
      status: 'done', progress: 100, total: 100, downloaded: 100,
      version: 'v0.2.0', filePath: 'C:/app/llama-gui-v0.2.0.exe', error: '',
    })

    await checkForUpdate()
    startUpdateDownload()

    expect(mockStartUpdateDownload).toHaveBeenCalledWith('v0.2.0')
    expect(updateState.download?.status).toBe('downloading')

    await vi.advanceTimersByTimeAsync(1100) // 触发一次轮询
    expect(updateState.download?.status).toBe('done')
    expect(updateState.download?.filePath).toContain('llama-gui-v0.2.0.exe')

    // 完成后轮询应停止（再推进时间不应重复拉取）
    const callsAfterDone = mockGetStatus.mock.calls.length
    await vi.advanceTimersByTimeAsync(2200)
    expect(mockGetStatus.mock.calls.length).toBe(callsAfterDone)
  })

  it('closeUpdateModal 关闭窗口并停止轮询', () => {
    updateState.showModal = true
    closeUpdateModal()
    expect(updateState.showModal).toBe(false)
  })
})
