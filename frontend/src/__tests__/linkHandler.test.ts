import { describe, it, expect, vi, beforeEach } from 'vitest'

// Mock the Wails runtime bridge (side-effect-bearing dependency); the handler
// must route absolute links through Browser.OpenURL and nothing else.
const { browserOpenURL } = vi.hoisted(() => ({ browserOpenURL: vi.fn() }))
vi.mock('@wailsio/runtime', () => ({ Browser: { OpenURL: browserOpenURL } }))

import { externalUrlFor, handleLinkClick } from '../lib/linkHandler'

describe('externalUrlFor', () => {
  it('returns absolute http and https URLs unchanged', () => {
    expect(externalUrlFor('http://example.com')).toBe('http://example.com')
    expect(externalUrlFor('https://example.com/a?b=1#frag')).toBe('https://example.com/a?b=1#frag')
    expect(externalUrlFor('HTTPS://example.com')).toBe('HTTPS://example.com')
  })

  it('rejects protocol-relative URLs', () => {
    expect(externalUrlFor('//host/path')).toBeNull()
  })

  it('rejects javascript: URLs', () => {
    expect(externalUrlFor('javascript:alert(1)')).toBeNull()
  })

  it('rejects anchors, mailto, relative paths and empty strings', () => {
    expect(externalUrlFor('#anchor')).toBeNull()
    expect(externalUrlFor('mailto:user@example.com')).toBeNull()
    expect(externalUrlFor('path/x.md')).toBeNull()
    expect(externalUrlFor('')).toBeNull()
  })
})

describe('handleLinkClick', () => {
  beforeEach(() => {
    browserOpenURL.mockClear()
  })

  /** Dispatch a bubbling click on target inside a container that has the handler bound (mirrors Vue @click). */
  function clickIn(containerHtml: string, selector: string): MouseEvent {
    const pane = document.createElement('div')
    pane.innerHTML = containerHtml
    document.body.appendChild(pane)
    pane.addEventListener('click', handleLinkClick)
    const target = pane.querySelector(selector) as Element
    const event = new MouseEvent('click', { bubbles: true, cancelable: true })
    target.dispatchEvent(event)
    pane.remove()
    return event
  }

  it('opens absolute http(s) links in the system browser and blocks navigation', () => {
    const event = clickIn('<a href="https://example.com/docs">docs</a>', 'a')
    expect(event.defaultPrevented).toBe(true)
    expect(browserOpenURL).toHaveBeenCalledTimes(1)
    expect(browserOpenURL).toHaveBeenCalledWith('https://example.com/docs')
  })

  it('blocks non-http(s) hrefs without opening anything', () => {
    const event = clickIn(
      '<a href="javascript:alert(1)">x</a><a href="#top">y</a><a href="page.md">z</a><a>no-href</a>',
      'a[href="javascript:alert(1)"]'
    )
    expect(event.defaultPrevented).toBe(true)
    expect(browserOpenURL).not.toHaveBeenCalled()
  })

  it('never navigates for relative hrefs even when nested inside markup', () => {
    const event = clickIn('<a href="#anchor"><strong>deep</strong></a>', 'strong')
    expect(event.defaultPrevented).toBe(true)
    expect(browserOpenURL).not.toHaveBeenCalled()
  })

  it('ignores clicks that are not on an anchor', () => {
    const event = clickIn('<p>plain text</p>', 'p')
    expect(event.defaultPrevented).toBe(false)
    expect(browserOpenURL).not.toHaveBeenCalled()
  })
})
