import { BrowserOpenURL } from '../../wailsjs/runtime'

/**
 * Absolute http(s) URL detection for rendered-markdown links. Only a scheme
 * prefix is checked (case-insensitive, matching browser scheme parsing):
 * protocol-relative //host, javascript:, mailto:, #anchors, relative paths
 * and the empty string all fall through to null.
 */
const ABSOLUTE_HTTP_URL_RE = /^https?:\/\//i

/**
 * Return the href when it is an absolute http(s) URL, null otherwise.
 * Pure decision core of handleLinkClick so the policy is unit-testable
 * without a DOM.
 */
export function externalUrlFor(href: string): string | null {
  return ABSOLUTE_HTTP_URL_RE.test(href) ? href : null
}

/**
 * Delegated click handler for containers that render untrusted markdown
 * (model description, chat messages, in-app docs): the WebView must never
 * navigate on a link click. Bind on the container with @click — every click
 * that lands inside an anchor is intercepted first; absolute http(s) links
 * then open in the system browser via Wails' BrowserOpenURL, every other
 * href (relative, #anchor, javascript:, mailto:, empty) is silently blocked.
 */
export function handleLinkClick(event: MouseEvent): void {
  // Deepest element under the pointer; text nodes are never click targets in
  // the DOM event model, but guard non-element targets defensively.
  const target = event.target instanceof Element ? event.target : null
  const anchor = target?.closest('a')
  if (!anchor) return
  // Always block the WebView navigation, even when the href turns out to be
  // unopenable — a link click must never replace the app UI.
  event.preventDefault()
  const external = externalUrlFor(anchor.getAttribute('href') ?? '')
  if (external) BrowserOpenURL(external)
}
