import { describe, it, expect } from 'vitest'
import { downloadVisibility } from '../lib/llamaDownload'

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
