/**
 * Remote docs content resolution: pure helpers for the Docs page's
 * remote-first content path (online → disk cache → bundled).
 *
 * The backend (core.GetRemoteDoc) decides online vs disk cache vs nothing;
 * this module layers the bundled fallback on top and formats the badge:
 * - resolveDocContent picks the winning text and the badge state;
 * - formatDocFetchedAt renders the cache badge's fetch timestamp;
 * - DOCS_GITHUB_URLS is the "open on GitHub" escape hatch per locale.
 *
 * Pure module: no Wails/network imports (the RemoteDocResult type is a
 * compile-time-only import), so it unit-tests without mocks.
 */

import type { RemoteDocResult } from '../wails'

/** Which tier currently feeds the docs pane (badge state; 'cached' is the UI spelling of the backend 'cache'). */
export type DocSourceState = 'online' | 'cached' | 'bundled'

/**
 * Resolve the content to display: a usable remote result (resolved, source
 * not "none", non-blank text) wins; anything else falls back to the bundled
 * text with the 'bundled' state. When remote wins, the state mirrors its
 * source ('online' | 'cached').
 */
export function resolveDocContent(
  remote: RemoteDocResult | null,
  bundled: string,
): { text: string; state: DocSourceState } {
  if (remote && remote.source !== 'none' && remote.text.trim() !== '') {
    return { text: remote.text, state: remote.source === 'online' ? 'online' : 'cached' }
  }
  return { text: bundled, state: 'bundled' }
}

/**
 * Format a backend RFC3339 fetchedAt for the cache badge as local
 * 'YYYY-MM-DD HH:mm'. '' for empty/invalid input. Deterministic manual
 * formatting (no locale APIs); the backend runs on this machine, so its
 * timestamp converted to the local zone is the user's own wall clock.
 */
export function formatDocFetchedAt(iso: string): string {
  if (!iso) return ''
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return ''
  const p = (n: number): string => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}`
}

/** Repo URLs of the bundled docs directories, for the "open on GitHub" button. */
export const DOCS_GITHUB_URLS: Record<'zh' | 'en', string> = {
  zh: 'https://github.com/CodeNeow/llama-cpp-desktop/blob/main/frontend/src/docs/zh',
  en: 'https://github.com/CodeNeow/llama-cpp-desktop/blob/main/frontend/src/docs/en',
}
