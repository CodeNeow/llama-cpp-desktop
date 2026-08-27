import { describe, it, expect } from 'vitest'
import { renderMarkdown, renderDescription } from '../lib/markdown'

describe('renderMarkdown', () => {
  it('renders headings as heading tags', () => {
    expect(renderMarkdown('# Title')).toContain('<h1>Title</h1>')
    expect(renderMarkdown('## Sub')).toContain('<h2>Sub</h2>')
    expect(renderMarkdown('### Small')).toContain('<h3>Small</h3>')
    expect(renderMarkdown('#### Smaller')).toContain('<h4>Smaller</h4>')
  })

  it('renders unordered and ordered lists', () => {
    const ul = renderMarkdown('- a\n- b')
    expect(ul).toContain('<ul>')
    expect(ul).toContain('<li>a</li>')
    expect(ul).toContain('<li>b</li>')

    const ol = renderMarkdown('1. a\n2. b')
    expect(ol).toContain('<ol>')
    expect(ol).toContain('<li>a</li>')
  })

  it('renders nested lists inside list items', () => {
    const out = renderMarkdown('- a\n  - b')
    // two <ul> openings: outer list plus the nested one
    expect(out.match(/<ul>/g)?.length).toBe(2)
    expect(out).toContain('<li>a')
    expect(out).toContain('<li>b</li>')
  })

  it('renders fenced code blocks as pre/code with language class', () => {
    const out = renderMarkdown('```js\nconst x = 1\n```')
    expect(out).toContain('<pre>')
    expect(out).toContain('<code class="language-js">')
    expect(out).toContain('const x = 1')
  })

  it('single newline becomes a <br> (breaks: true)', () => {
    const out = renderMarkdown('line one\nline two')
    expect(out).toContain('<br>')
    expect(out).toContain('line one')
    expect(out).toContain('line two')
  })

  it('escapes raw HTML in input instead of rendering it', () => {
    const out = renderMarkdown('<script>alert(1)</script>')
    expect(out).toContain('&lt;script&gt;')
    expect(out).not.toContain('<script>')
  })

  it('escapes raw HTML attempting attribute injection', () => {
    const out = renderMarkdown('<img src=x onerror=alert(1)>')
    expect(out).not.toContain('<img')
    expect(out).toContain('&lt;img')
  })

  it('linkifies bare URLs into anchors', () => {
    const out = renderMarkdown('see https://example.com/a?b=1 for details')
    expect(out).toContain('<a href="https://example.com/a?b=1">')
  })

  it('renders markdown links with the label as text', () => {
    const out = renderMarkdown('[label](https://example.com)')
    expect(out).toContain('<a href="https://example.com">label</a>')
  })

  it('empty or whitespace-only input returns an empty string', () => {
    expect(renderMarkdown('')).toBe('')
    expect(renderMarkdown('   ')).toBe('')
    expect(renderMarkdown('\n\t  \n')).toBe('')
  })
})

describe('renderDescription', () => {
  it('strips embedded HTML wrappers, keeping the inner text rendered', () => {
    // HF READMEs commonly wrap content in tags like <h1 align="center">Title</h1>
    const out = renderDescription('<h1 align="center">Title</h1>')
    expect(out).toContain('Title')
    expect(out).not.toContain('<h1')
  })

  it('renders markdown syntax after stripping HTML', () => {
    const out = renderDescription('<p>intro</p>\n**bold** claim')
    expect(out).toContain('intro')
    expect(out).toContain('<strong>bold</strong>')
  })

  it('empty or whitespace-only input returns an empty string', () => {
    expect(renderDescription('')).toBe('')
    expect(renderDescription('   ')).toBe('')
    // input consisting only of HTML tags renders nothing after stripping
    expect(renderDescription('<img src="x.png">')).toBe('')
  })
})
