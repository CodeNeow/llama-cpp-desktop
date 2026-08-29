import MarkdownIt from 'markdown-it'

/**
 * Shared markdown-it instance for rendering chat message content.
 *
 * - html: false — raw HTML in model output is escaped, never rendered (injection
 *   defense; markdown-it also refuses javascript:/vbscript: link targets);
 * - breaks: true — single newlines become <br>, matching chat line-breaking
 *   expectations instead of CommonMark's paragraph joining;
 * - linkify: true — bare URLs in the text become anchors.
 */
const md = new MarkdownIt({
  html: false,
  breaks: true,
  linkify: true,
})

/**
 * Render markdown text to HTML for display in assistant message bubbles.
 * Empty or whitespace-only input returns '' so nothing renders for the
 * reasoning-only streaming phase.
 */
export function renderMarkdown(text: string): string {
  if (!text.trim()) return ''
  return md.render(text)
}

// ─── HTML → line-structure flattening for README descriptions ───────────────
//
// renderDescription converts HTML-heavy model READMEs (HF/ModelScope pages
// routinely wrap content in <div align="center">, badge <img>s, <table> rows
// and <p style=...>) into markdown line structure BEFORE the remaining tags
// are stripped, so headings, lists and cell-separated rows survive as
// readable text instead of collapsing into run-together lines. The security
// posture is unchanged: no HTML is ever rendered (renderMarkdown keeps
// html:false) — the pipeline only introduces newlines, spaces and "- " list
// markers, never executable markup.

/** HTML comments, non-greedy across lines so consecutive comments die individually. */
const HTML_COMMENT_RE = /<!--[\s\S]*?-->/g

/** <script>/<style> blocks including their content (attribute-ful open tags, case-insensitive). */
const SCRIPT_BLOCK_RE = /<script\b[^>]*>[\s\S]*?<\/script\s*>/gi
const STYLE_BLOCK_RE = /<style\b[^>]*>[\s\S]*?<\/style\s*>/gi

/** <br> variants (<br>, <br/>, <br />) become a hard line break. */
const BR_TAG_RE = /<br\s*\/?>/gi

/** Closing block-level tags become a line break so each block lands on its own line. */
const CLOSING_BLOCK_TAG_RE = /<\/(?:p|div|li|h[1-6]|tr|table|blockquote|pre)\s*>/gi

/**
 * Table cell boundaries become a single space so a row stays on one line.
 * Opening tags may carry attributes (e.g. <td align="center">), hence [^>]*;
 * \b keeps <thead>/<tbody>/<tfoot> from matching as <t[dh]>.
 */
const TABLE_CELL_TAG_RE = /<\/?t[dh]\b[^>]*>/gi

/** Opening <li> becomes a markdown list marker (attributes tolerated; \b keeps <link> out). */
const LIST_ITEM_TAG_RE = /<li\b[^>]*>/gi

/** Any tag still standing after the conversions is stripped wholesale (the original behavior). */
const ANY_TAG_RE = /<[^>]*>/g

/** Runs of 3+ newlines collapse to one blank line (tag removal leaves canyons otherwise). */
const BLANK_LINE_CANYON_RE = /\n{3,}/g

/** Pipeline step 1: drop HTML comments, content included. */
function removeHtmlComments(text: string): string {
  return text.replace(HTML_COMMENT_RE, '')
}

/** Pipeline step 2: drop <script>/<style> blocks entirely — their content must never surface as text. */
function removeScriptStyleBlocks(text: string): string {
  return text.replace(SCRIPT_BLOCK_RE, '').replace(STYLE_BLOCK_RE, '')
}

/** Pipeline step 3: convert structural tag boundaries into newlines / spaces / list markers. */
function convertHtmlBoundaries(text: string): string {
  return text
    .replace(BR_TAG_RE, '\n')
    .replace(CLOSING_BLOCK_TAG_RE, '\n')
    .replace(TABLE_CELL_TAG_RE, ' ')
    .replace(LIST_ITEM_TAG_RE, '- ')
}

/** Pipeline step 4: strip every remaining tag. */
function stripRemainingTags(text: string): string {
  return text.replace(ANY_TAG_RE, '')
}

/** Leading whitespace of a line outside a fence; 4+ of these mean "code block" in markdown. */
const LEADING_INDENT_RE = /^\s+/

/**
 * Pipeline step 5: remove leading whitespace from every line OUTSIDE fenced
 * code blocks. HTML flattening leaves indentation that carries no meaning
 * anymore, but in markdown it means "code block" — indented HTML headers
 * (e.g. unsloth READMEs) would render their first screen as a <pre><code>
 * block. Fenced code content must be exempt: its indentation is meaningful.
 * Intentional trade-off: markdown nested-list indentation outside fences is
 * also flattened — acceptable, nesting is rare in model READMEs and
 * readability wins.
 */
function stripIndentOutsideFences(text: string): string {
  const out: string[] = []
  let insideFence = false
  for (const line of text.split('\n')) {
    const trimmed = line.trimStart()
    if (trimmed.startsWith('```') || trimmed.startsWith('~~~')) {
      // Fence markers toggle open/close alternately; the trimmed check
      // tolerates indented fences. Fence lines themselves stay verbatim.
      insideFence = !insideFence
      out.push(line)
      continue
    }
    // Inside a fence keep the line verbatim (code indentation is meaningful);
    // outside a fence drop the meaningless flattening leftover.
    out.push(insideFence ? line : line.replace(LEADING_INDENT_RE, ''))
  }
  return out.join('\n')
}

/** Pipeline step 6: collapse runs of 3+ newlines into a single blank line. */
function collapseBlankLines(text: string): string {
  return text.replace(BLANK_LINE_CANYON_RE, '\n\n')
}

/**
 * Render a model README description for the model detail page. HF/ModelScope
 * READMEs wrap their content in HTML like <h1 align="center">, <table> rows
 * or badge <img>s; the flattening pipeline above first converts that
 * structure into markdown line structure, then leftover tags are stripped and
 * the text is rendered with renderMarkdown. markdown-it runs with html:false,
 * so any leftover markup is escaped instead of rendered and javascript: link
 * targets are refused — the same trust posture as chat messages. Empty input
 * returns '' (renderMarkdown's whitespace-only guard).
 */
export function renderDescription(text: string): string {
  const flattened = [
    removeHtmlComments,
    removeScriptStyleBlocks,
    convertHtmlBoundaries,
    stripRemainingTags,
    stripIndentOutsideFences,
    collapseBlankLines,
  ].reduce((current, step) => step(current), text)
  return renderMarkdown(flattened)
}
