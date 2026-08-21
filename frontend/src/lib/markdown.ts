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
