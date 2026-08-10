import { describe, it, expect } from 'vitest'
import { downloadVisibility, initialDownloadAction } from '../lib/llamaDownload'

describe('downloadVisibility', () => {
  it('idle 时显示按钮组、不显示进度区', () => {
    const v = downloadVisibility('idle')
    expect(v.showButtons).toBe(true)
    expect(v.showProgress).toBe(false)
  })

  it('error 时按钮组与进度区同时显示（进度区承载错误信息与重试按钮）', () => {
    const v = downloadVisibility('error')
    expect(v.showButtons).toBe(true)
    expect(v.showProgress).toBe(true)
  })

  it('fetching/downloading/paused/extracting 时显示进度区、隐藏按钮组', () => {
    for (const status of ['fetching', 'downloading', 'paused', 'extracting']) {
      const v = downloadVisibility(status)
      expect(v.showButtons, `status=${status} 时按钮组应隐藏`).toBe(false)
      expect(v.showProgress, `status=${status} 时进度区应显示`).toBe(true)
    }
  })

  it('done 时显示进度区（下载完成提示）、隐藏按钮组', () => {
    const v = downloadVisibility('done')
    expect(v.showButtons).toBe(false)
    expect(v.showProgress).toBe(true)
  })

  // 核心回归：进度区显示只依赖下载状态，函数签名不含自定义路径参数——
  // 设置自定义 llama.cpp 目录后进度区仍然渲染，不会出现"按钮消失且无反馈"
  it('进度区显示与自定义路径无关（无路径参数，语义上不被其影响）', () => {
    expect(downloadVisibility('fetching').showProgress).toBe(true)
    expect(downloadVisibility('downloading').showProgress).toBe(true)
    expect(downloadVisibility('extracting').showProgress).toBe(true)
  })
})

describe('initialDownloadAction', () => {
  it('downloading/fetching/extracting 时返回 poll（下载中需恢复轮询），与 installed 参数无关', () => {
    for (const status of ['downloading', 'fetching', 'extracting']) {
      expect(initialDownloadAction(status, false), `status=${status} 未安装时应 poll`).toBe('poll')
      expect(initialDownloadAction(status, true), `status=${status} 已安装时也应 poll`).toBe('poll')
    }
  })

  it('done 且未安装时返回 refresh（下载完成但未检测到安装，刷新系统信息）', () => {
    expect(initialDownloadAction('done', false)).toBe('refresh')
  })

  it('done 且已安装时返回 none（onMounted 的 fetchSystemInfo 已覆盖已安装场景）', () => {
    expect(initialDownloadAction('done', true)).toBe('none')
  })

  // 核心回归：此前 checkInitialDownloadStatus 缺少 error 分支，切页期间下载失败
  // 返回主页时状态回落为 idle，错误信息丢失、只剩"下载"按钮；现在应返回
  // showError，由 UI 显示错误信息与重试按钮
  it('error 时返回 showError（切页期间下载失败，返回主页应显示错误与重试按钮）', () => {
    expect(initialDownloadAction('error', false)).toBe('showError')
    expect(initialDownloadAction('error', true)).toBe('showError')
  })

  it('idle 时返回 none（无下载需处理）', () => {
    expect(initialDownloadAction('idle', false)).toBe('none')
    expect(initialDownloadAction('idle', true)).toBe('none')
  })
})
