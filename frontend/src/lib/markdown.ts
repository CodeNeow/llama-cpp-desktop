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

/**
 * Render a model README description for the model detail page. HF/ModelScope
 * READMEs wrap their content in HTML tags like <h1 align="center">, <img> or
 * <p>; strip them first so the inner text survives as plain markdown, then
 * render it with renderMarkdown. markdown-it runs with html:false, so any
 * leftover markup is escaped instead of rendered and javascript: link targets
 * are refused — the same trust posture as chat messages. Empty input returns
 * '' (renderMarkdown's whitespace-only guard).
 */
export function renderDescription(text: string): string {
  const stripped = text.replace(/<[^>]*>/g, '')
  return renderMarkdown(stripped)
}
