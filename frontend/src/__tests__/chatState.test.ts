import { describe, it, expect, vi, beforeEach } from 'vitest'
import { capMessages, ChatMessage, loadChatHistory, persistChat, messages, selectedModel } from '../lib/chatState'

describe('capMessages', () => {
  it('截断保留最近 N 条', () => {
    const msgs: ChatMessage[] = Array.from({ length: 250 }, (_, i) => ({ role: 'user' as const, content: String(i) }))
    const capped = capMessages(msgs, 200)
    expect(capped).toHaveLength(200)
    expect(capped[0].content).toBe('50')
    expect(capped[199].content).toBe('249')
  })

  it('不足上限原样返回', () => {
    const msgs: ChatMessage[] = [{ role: 'user', content: 'a' }]
    expect(capMessages(msgs, 200)).toBe(msgs)
  })

  it('空数组返回空数组', () => {
    expect(capMessages([], 200)).toEqual([])
  })
})

describe('chatState 模块级状态与持久化', () => {
  beforeEach(() => {
    localStorage.clear()
    vi.resetModules()
  })

  it('loadChatHistory 恢复合法 JSON 消息列表', async () => {
    const saved: ChatMessage[] = [
      { role: 'user', content: 'hello' },
      { role: 'assistant', content: 'world' },
    ]
    localStorage.setItem('llama-desktop-chat-messages', JSON.stringify(saved))

    const { loadChatHistory: lch, messages: msgs } = await import('../lib/chatState')
    lch()

    expect(msgs.value).toHaveLength(2)
    expect(msgs.value[0]).toEqual({ role: 'user', content: 'hello' })
  })

  it('loadChatHistory 非法 JSON 静默回退为空数组', async () => {
    localStorage.setItem('llama-desktop-chat-messages', 'not-json')

    const { loadChatHistory: lch, messages: msgs } = await import('../lib/chatState')
    lch()

    expect(msgs.value).toEqual([])
  })

  it('loadChatHistory 超上限数据自动截断到 cap', async () => {
    const saved: ChatMessage[] = Array.from({ length: 250 }, (_, i) => ({ role: 'user' as const, content: String(i) }))
    localStorage.setItem('llama-desktop-chat-messages', JSON.stringify(saved))

    const { loadChatHistory: lch, messages: msgs } = await import('../lib/chatState')
    lch()

    expect(msgs.value).toHaveLength(200)
    expect(msgs.value[0].content).toBe('50')
  })

  it('loadChatHistory 恢复 selectedModel', async () => {
    localStorage.setItem('llama-desktop-chat-model', 'llama-2-7b-chat')

    const { loadChatHistory: lch, selectedModel: model } = await import('../lib/chatState')
    lch()

    expect(model.value).toBe('llama-2-7b-chat')
  })

  it('loadChatHistory 无 MODEL_KEY 时 selectedModel 保持空串', async () => {
    const { loadChatHistory: lch, selectedModel: model } = await import('../lib/chatState')
    lch()

    expect(model.value).toBe('')
  })

  it('persistChat 将 messages 与 selectedModel 写入 localStorage', async () => {
    const { messages: msgs, selectedModel: model, persistChat, loadChatHistory: lch } = await import('../lib/chatState')
    msgs.value = [{ role: 'user', content: 'hi' }]
    model.value = 'test-model'
    persistChat()

    // 通过新模块实例验证落盘结果
    vi.resetModules()
    const { messages: msgs2, selectedModel: model2, loadChatHistory: lch2 } = await import('../lib/chatState')
    lch2()

    expect(msgs2.value).toEqual([{ role: 'user', content: 'hi' }])
    expect(model2.value).toBe('test-model')
  })

  it('persistChat selectedModel 为空时移除 MODEL_KEY', async () => {
    const { selectedModel: model, persistChat } = await import('../lib/chatState')
    model.value = ''
    persistChat()

    expect(localStorage.getItem('llama-desktop-chat-model')).toBeNull()
  })

  it('模块单例隔离：vi.resetModules + 动态 import 得到全新状态', async () => {
    // 第一次导入并修改状态
    const { messages: msgs1, persistChat: p1 } = await import('../lib/chatState')
    msgs1.value = [{ role: 'user', content: 'first' }]
    p1()

    // 清空 localStorage 后重置模块再次导入，验证状态全新（loadChatHistory 读到空）
    localStorage.clear()
    vi.resetModules()
    const { messages: msgs2, loadChatHistory: lch2 } = await import('../lib/chatState')
    lch2()

    expect(msgs2.value).toEqual([])
    // 确认是新 ref 对象
    expect(msgs2).not.toBe(msgs1)
  })
})
