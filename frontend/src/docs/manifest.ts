/**
 * Docs page manifest: the single source of truth for the bundled tutorial
 * content under frontend/src/docs/.
 *
 * Design decisions:
 * - Section titles live in the i18n dictionaries (titleKey), NOT in the
 *   markdown files — the TOC then switches language instantly, without
 *   loading any content. The markdown bodies therefore start at `##` level;
 * - Content is loaded lazily per section via dynamic `?raw` imports, so each
 *   markdown file becomes its own chunk fetched only when that section is
 *   opened (the Docs route itself is the only consumer of this module);
 * - The Record<DocSectionId, ...> loaders are checked by TypeScript: adding a
 *   section id to docSections without a matching zh/en file (or vice versa)
 *   fails the type check and the build.
 */

import type { Locale } from '../lib/i18n'

/** Identifiers of every docs section, in TOC display order. */
export type DocSectionId =
  | 'quickstart'
  | 'runtime'
  | 'downloads'
  | 'models'
  | 'api'
  | 'chat'
  | 'settings'
  | 'headless'
  | 'faq'

/** One docs section: a stable id plus the i18n key of its display title. */
export interface DocSection {
  id: DocSectionId
  titleKey: string
}

/** Section list consumed by the Docs page TOC and by the manifest integrity test. */
export const docSections: readonly DocSection[] = [
  { id: 'quickstart', titleKey: 'docs.section.quickstart' },
  { id: 'runtime', titleKey: 'docs.section.runtime' },
  { id: 'downloads', titleKey: 'docs.section.downloads' },
  { id: 'models', titleKey: 'docs.section.models' },
  { id: 'api', titleKey: 'docs.section.api' },
  { id: 'chat', titleKey: 'docs.section.chat' },
  { id: 'settings', titleKey: 'docs.section.settings' },
  { id: 'headless', titleKey: 'docs.section.headless' },
  { id: 'faq', titleKey: 'docs.section.faq' },
]

/**
 * Lazy raw-text loaders per locale and section. `?raw` imports return the
 * file content as the default export; each dynamic import is a separate
 * Vite chunk, keeping the docs out of the main bundle.
 */
const docLoaders: Record<Locale, Record<DocSectionId, () => Promise<string>>> = {
  zh: {
    quickstart: () => import('./zh/quickstart.md?raw').then((m) => m.default),
    runtime: () => import('./zh/runtime.md?raw').then((m) => m.default),
    downloads: () => import('./zh/downloads.md?raw').then((m) => m.default),
    models: () => import('./zh/models.md?raw').then((m) => m.default),
    api: () => import('./zh/api.md?raw').then((m) => m.default),
    chat: () => import('./zh/chat.md?raw').then((m) => m.default),
    settings: () => import('./zh/settings.md?raw').then((m) => m.default),
    headless: () => import('./zh/headless.md?raw').then((m) => m.default),
    faq: () => import('./zh/faq.md?raw').then((m) => m.default),
  },
  en: {
    quickstart: () => import('./en/quickstart.md?raw').then((m) => m.default),
    runtime: () => import('./en/runtime.md?raw').then((m) => m.default),
    downloads: () => import('./en/downloads.md?raw').then((m) => m.default),
    models: () => import('./en/models.md?raw').then((m) => m.default),
    api: () => import('./en/api.md?raw').then((m) => m.default),
    chat: () => import('./en/chat.md?raw').then((m) => m.default),
    settings: () => import('./en/settings.md?raw').then((m) => m.default),
    headless: () => import('./en/headless.md?raw').then((m) => m.default),
    faq: () => import('./en/faq.md?raw').then((m) => m.default),
  },
}

/** Load one section's markdown body in the given locale. */
export function loadDocSection(id: DocSectionId, locale: Locale): Promise<string> {
  return docLoaders[locale][id]()
}
