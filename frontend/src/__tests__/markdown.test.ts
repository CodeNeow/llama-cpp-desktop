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

  it('removes HTML comments with their content', () => {
    const out = renderDescription('a<!-- hidden note -->b')
    expect(out).not.toContain('hidden')
    expect(out).toContain('ab')
  })

  it('removes script and style blocks including their content', () => {
    const out = renderDescription('<style>p { color: red }</style>keep me<script>alert(2)</script>')
    expect(out).not.toContain('alert')
    expect(out).not.toContain('color')
    expect(out).toContain('keep me')
  })

  it('turns <br> and closing block tags into line breaks', () => {
    // <br> variants become a newline (breaks:true renders it as <br>)
    expect(renderDescription('line1<br>line2')).toContain('<br>')
    // </p> becomes a newline so adjacent paragraphs never run together
    const paragraphs = renderDescription('<p style="x">first</p><p>second</p>')
    expect(paragraphs).toContain('first')
    expect(paragraphs).toContain('second')
    expect(paragraphs).not.toContain('firstsecond')
  })

  it('keeps a table row with <td> cells on one line', () => {
    // each cell boundary maps to one space, so a row renders as a single line
    const out = renderDescription('<tr><td>a</td><td>b</td></tr>')
    expect(out).toBe('<p>a  b</p>\n')
  })

  it('converts <li> items into markdown list markers', () => {
    const out = renderDescription('<ul><li>one</li><li>two</li></ul>')
    expect(out).toContain('<li>one</li>')
    expect(out).toContain('<li>two</li>')
  })

  it('passes plain markdown through unchanged', () => {
    const out = renderDescription('# Title\n\n- a\n- b\n\n**bold**')
    expect(out).toContain('<h1>Title</h1>')
    expect(out).toContain('<li>a</li>')
    expect(out).toContain('<strong>bold</strong>')
  })

  it('collapses blank-line canyons left by tag removal', () => {
    // </div> becomes a newline; the surrounding newlines then collapse so the
    // two paragraphs end up adjacent instead of separated by a canyon
    const out = renderDescription('<div>a</div>\n\n\n\n\n<div>b</div>')
    expect(out).toBe('<p>a</p>\n<p>b</p>\n')
  })

  it('never lets hostile markup through to the rendered HTML', () => {
    const hostile = '<p onclick="alert(1)">hi</p><script>alert(2)</script><img src=x onerror="alert(3)">'
    const out = renderMarkdown(renderDescription(hostile))
    expect(out).not.toContain('<script')
    expect(out).not.toContain('onerror=')
    expect(out).not.toContain('onclick')
    // tags are stripped whole, so their attributes go with them
    expect(out).not.toContain('<p onclick')
    expect(out).toContain('hi')
  })

  it('de-indents flattened HTML indentation so it does not become a code block', () => {
    // unsloth-style READMEs open with an indented HTML block; after tag
    // stripping the inner lines keep their indentation, which markdown would
    // render as an indented <pre><code> block without this step
    const readme = '<div align="center">\n  To run Qwen3.5 locally - Read our Guide!\n    Unsloth Dynamic 2.0 achieves superior accuracy\n</div>'
    const out = renderDescription(readme)
    expect(out).not.toContain('<pre>')
    expect(out).toContain('To run Qwen3.5 locally - Read our Guide!')
    expect(out).toContain('Unsloth Dynamic 2.0 achieves superior accuracy')
    // the full chain stays text-only as well (a second render escapes, but nothing is lost)
    expect(renderMarkdown(out)).toContain('To run Qwen3.5 locally - Read our Guide!')
  })

  it('keeps indentation inside a ``` fence verbatim', () => {
    const out = renderDescription('intro\n```\n  indented code\n    more\n```\nafter')
    // the spaces survive AND the fence still renders as a real code block
    expect(out).toContain('<pre><code>  indented code')
    expect(out).toContain('    more')
  })

  it('keeps indentation inside a ~~~ fence verbatim', () => {
    const out = renderDescription('~~~\n  keep me indented\n~~~\nplain')
    expect(out).toContain('<pre><code>  keep me indented')
  })

  it('de-indents only lines outside fences in mixed content', () => {
    const out = renderDescription('```\n  code stays\n```\n    paragraph line')
    expect(out).toContain('<pre>')
    expect(out).toContain('  code stays')
    // the indented paragraph line is flattened, the fenced code is not
    expect(out).not.toContain('    paragraph line')
    expect(out).toContain('<p>paragraph line</p>')
  })
})
