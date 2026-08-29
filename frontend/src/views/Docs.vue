<template>
  <div class="page docs-page">
    <div class="sticky-top">
      <div class="page-header">
        <div class="page-header-text">
          <h1 class="page-title">{{ t('docs.title') }}</h1>
          <p class="page-subtitle">{{ t('docs.subtitle') }}</p>
        </div>
        <!-- Content source badge + escape hatches: the badge names the tier
             that produced the pane (online fetch / disk cache / bundled),
             refresh forces a fresh network round-trip, the GitHub button
             opens the live docs directory in the browser. -->
        <div class="docs-header-actions">
          <span class="source-badge" :class="'source-' + docSource">{{ sourceLabel }}</span>
          <button class="docs-action-btn" type="button" :disabled="refreshing" @click="refreshRemote">
            {{ refreshing ? t('docs.refreshing') : t('docs.refresh') }}
          </button>
          <button class="docs-action-btn" type="button" @click="openOnGithub">{{ t('docs.openOnGithub') }}</button>
        </div>
      </div>
    </div>

    <div class="docs-layout">
      <!-- In-page section navigator: a sticky vertical list on wide screens,
           collapsing into a wrapping chip row on narrow ones (CSS only) -->
      <nav class="docs-toc" :aria-label="t('docs.toc')">
        <button
          v-for="section in docSections"
          :key="section.id"
          class="toc-item"
          :class="{ active: section.id === activeSection }"
          type="button"
          @click="selectSection(section.id)"
        >
          {{ t(section.titleKey) }}
        </button>
      </nav>

      <!-- Rendered markdown pane. v-html is safe here: the shared markdown-it
           instance (lib/markdown.ts) runs with html:false, so raw HTML in the
           docs source is escaped, never rendered — the same injection defense
           as chat messages. -->
      <!-- Delegated link handler: external links open in the system browser,
           the WebView never navigates (see lib/linkHandler.ts) -->
      <article ref="contentEl" class="docs-content" :class="{ 'is-loading': loading }" v-html="rendered" @click="handleLinkClick"></article>
    </div>

    <!-- Load failure of a bundled asset is practically impossible; still handled without silently swallowing -->
    <div v-if="loadError" class="docs-error">{{ loadError }}</div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, nextTick } from 'vue'
import { docSections, loadDocSection, type DocSectionId } from '../docs/manifest'
import { renderMarkdown } from '../lib/markdown'
import { handleLinkClick } from '../lib/linkHandler'
import { t, locale } from '../lib/i18n'
import { LatestOnly } from '../lib/latestOnly'
import { getRemoteDoc } from '../wails'
import { DOCS_GITHUB_URLS, formatDocFetchedAt, resolveDocContent, type DocSourceState } from '../lib/remoteDocs'
import { BrowserOpenURL } from '../../wailsjs/runtime'

// Currently displayed section; defaults to the first entry of the manifest
const activeSection = ref<DocSectionId>(docSections[0].id)

// Raw markdown of the active section; previous content stays visible (dimmed)
// while a new section loads, so switching never flashes an empty pane
const content = ref('')
const loading = ref(false)
const loadError = ref('')

// Source state of the displayed content (badge): which tier produced it —
// a successful network fetch, the backend's disk cache, or the bundled copy
const docSource = ref<DocSourceState>('bundled')
// RFC3339 fetchedAt of the cached content; the badge shows it for 'cached'
const fetchedAt = ref('')
const refreshing = ref(false)

// Badge label per current state; the cached state carries the fetch time, or
// falls back to the plain label when the timestamp is unavailable (a cached
// file whose meta entry is missing) — never a dangling "· " separator
const sourceLabel = computed(() => {
  if (docSource.value === 'online') return t('docs.sourceOnline')
  if (docSource.value === 'cached') {
    const when = formatDocFetchedAt(fetchedAt.value)
    return when !== '' ? t('docs.sourceCached', { time: when }) : t('docs.sourceCachedPlain')
  }
  return t('docs.sourceBundled')
})

// Race protection: a slow locale/section fetch must not overwrite a newer one.
// Shared by the section load and the refresh button so either path discards
// the other's stale response.
const latest = new LatestOnly()

async function loadSection(): Promise<void> {
  const seq = latest.begin()
  loading.value = true
  loadError.value = ''
  // Bundled text of this round; the remote result falls back to it (or to the
  // pane's current text if the bundled asset itself failed — near-impossible)
  let bundled = ''
  try {
    const md = await loadDocSection(activeSection.value, locale.value)
    if (!latest.isLatest(seq)) return
    bundled = md
    content.value = md
  } catch (e) {
    if (!latest.isLatest(seq)) return
    loadError.value = t('docs.loadError', { msg: e instanceof Error ? e.message : String(e) })
  } finally {
    if (latest.isLatest(seq)) loading.value = false
  }

  // Remote-first refresh of the freshly shown section, under the SAME
  // LatestOnly sequence: a quick section switch discards the stale response.
  // Network trouble is an expected state — swallowed here (console.warn), the
  // pane keeps the bundled text and the badge stays 'bundled'.
  try {
    const remote = await getRemoteDoc(locale.value, activeSection.value)
    if (!latest.isLatest(seq)) return
    const resolved = resolveDocContent(remote, bundled || content.value)
    content.value = resolved.text
    docSource.value = resolved.state
    fetchedAt.value = resolved.state === 'bundled' ? '' : remote.fetchedAt
  } catch (e) {
    if (!latest.isLatest(seq)) return
    console.warn('remote docs fetch failed; staying with bundled content', e)
  }
}

// Refresh button: force a network round-trip for the current section. A usable
// result swaps content + badge; an unusable one (or a binding rejection)
// silently keeps the current content — network trouble never surfaces as an
// error UI here.
async function refreshRemote(): Promise<void> {
  if (refreshing.value) return
  refreshing.value = true
  const seq = latest.begin()
  try {
    const remote = await getRemoteDoc(locale.value, activeSection.value, true)
    if (!latest.isLatest(seq)) return
    const resolved = resolveDocContent(remote, content.value)
    if (resolved.state !== 'bundled') {
      content.value = resolved.text
      docSource.value = resolved.state
      fetchedAt.value = remote.fetchedAt
    }
  } catch (e) {
    console.warn('remote docs refresh failed; keeping current content', e)
  } finally {
    refreshing.value = false
  }
}

// Escape hatch: open the current locale's docs directory on GitHub in the
// system browser (Wails runtime), so the newest released tutorial is one
// click away even when the remote fetch path is unavailable.
function openOnGithub(): void {
  BrowserOpenURL(DOCS_GITHUB_URLS[locale.value])
}

// Reload on section switch and on language switch (content follows app language reactively)
watch([activeSection, locale], () => { void loadSection() }, { immediate: true })

// Rendered HTML via the shared markdown-it instance (html disabled — see template comment)
const rendered = computed(() => renderMarkdown(content.value))

const contentEl = ref<HTMLElement | null>(null)

function selectSection(id: DocSectionId): void {
  if (id === activeSection.value) return
  activeSection.value = id
  // Bring the freshly selected section's top into view. The scroll container
  // is the shared .content-area ancestor (App.vue); the article's
  // scroll-margin-top clears the sticky page header.
  void nextTick(() => contentEl.value?.scrollIntoView({ block: 'start' }))
}
</script>

<style scoped>
.docs-page {
  /* No top padding: header flush with content top, title aligns with sidebar logo (see global.css .page-header) */
  padding: 0 48px 60px;
}

.page-header {
  /* Flex row: title block left, badge + actions right (same layout as Models.vue) */
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 12px 16px;
  /* Use padding instead of margin: header background covers this gap so content scrolls without leaving a seam */
  padding-bottom: 36px;
}

.page-header-text {
  min-width: 0;
}

/* ─── Header actions: source badge + refresh + open-on-GitHub ─── */
.docs-header-actions {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-shrink: 0;
}

.source-badge {
  display: inline-flex;
  align-items: center;
  padding: 4px 12px;
  border-radius: 999px;
  border: 1px solid var(--border);
  background: var(--surface);
  font-size: 12px;
  white-space: nowrap;
  color: var(--text-muted);
}

.source-badge.source-online {
  color: var(--accent-light);
  border-color: var(--accent-glow);
  background: var(--active-bg);
}

.source-badge.source-cached {
  color: var(--text-secondary);
}

.docs-action-btn {
  padding: 6px 14px;
  background: var(--surface);
  color: var(--text-secondary);
  border: 1px solid var(--border);
  border-radius: 8px;
  font-size: 12.5px;
  cursor: pointer;
  transition: all 0.2s;
  white-space: nowrap;
}

.docs-action-btn:hover:not(:disabled) {
  background: var(--hover-bg);
  color: var(--text-primary);
}

.docs-action-btn:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}

.page-title {
  font-size: 28px;
  font-weight: 700;
  color: var(--text-primary);
  margin: 0 0 4px;
  letter-spacing: -0.5px;
  line-height: 1.2;
}

.page-subtitle {
  font-size: 14px;
  color: var(--text-dim);
  margin: 0;
}

/* ─── Layout: sticky TOC left, content right ─── */
.docs-layout {
  display: flex;
  align-items: flex-start;
  gap: 24px;
}

/* Sticky offset below the sticky page header (title + subtitle + header
   padding amount to roughly 96px; 112px leaves a comfortable gap) */
.docs-toc {
  width: 190px;
  flex-shrink: 0;
  position: sticky;
  top: 112px;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.toc-item {
  text-align: left;
  padding: 8px 14px;
  border: 1px solid transparent;
  border-radius: 8px;
  background: transparent;
  color: var(--text-muted);
  font-size: 13.5px;
  cursor: pointer;
  transition: background 0.15s ease, color 0.15s ease;
}

.toc-item:hover {
  background: var(--hover-bg);
  color: var(--text-primary);
}

.toc-item.active {
  background: var(--active-bg);
  color: var(--text-primary);
  font-weight: 600;
}

/* ─── Content pane ─── */
.docs-content {
  flex: 1;
  min-width: 0;
  /* scroll-margin-top keeps the article top clear of the sticky page header
     when scrollIntoView jumps here after a TOC click */
  scroll-margin-top: 112px;
  background: var(--bg-card);
  border: 1px solid var(--border);
  border-radius: 14px;
  padding: 24px 28px 32px;
  color: var(--text-secondary);
  font-size: 14px;
  line-height: 1.75;
  transition: opacity 0.15s ease;
}

/* Dim (do not blank) the previous section while a new one loads */
.docs-content.is-loading {
  opacity: 0.45;
}

.docs-error {
  margin-top: 12px;
  font-size: 13px;
  color: var(--danger);
}

/* ─── Rendered markdown ───
   v-html content does not carry the scope attribute, so child selectors need
   :deep() (same approach as ModelDetail.vue's description styles). */
.docs-content :deep(h2) {
  font-size: 19px;
  font-weight: 700;
  color: var(--text-primary);
  margin: 22px 0 12px;
  padding-bottom: 8px;
  border-bottom: 1px solid var(--border);
}

.docs-content :deep(h2:first-child) {
  margin-top: 0;
}

.docs-content :deep(h3) {
  font-size: 15.5px;
  font-weight: 600;
  color: var(--text-primary);
  margin: 20px 0 8px;
}

.docs-content :deep(p) {
  margin: 10px 0;
}

.docs-content :deep(ul),
.docs-content :deep(ol) {
  margin: 10px 0;
  padding-left: 1.5em;
}

.docs-content :deep(li) {
  margin: 4px 0;
}

.docs-content :deep(strong) {
  color: var(--text-primary);
}

.docs-content :deep(a) {
  color: var(--accent-light);
  word-break: break-all;
}

/* Guard for future docs images: never wider than the content pane */
.docs-content :deep(img) {
  max-width: 100%;
  height: auto;
}

.docs-content :deep(code) {
  font-family: var(--font-mono);
  font-size: 12.5px;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 5px;
  padding: 1px 5px;
}

.docs-content :deep(pre) {
  background: var(--bg-primary);
  border: 1px solid var(--border);
  border-radius: 10px;
  padding: 14px 16px;
  overflow-x: auto;
  margin: 12px 0;
}

.docs-content :deep(pre code) {
  background: transparent;
  border: none;
  border-radius: 0;
  padding: 0;
  line-height: 1.6;
}

.docs-content :deep(table) {
  border-collapse: collapse;
  width: 100%;
  margin: 12px 0;
  font-size: 13.5px;
}

.docs-content :deep(th),
.docs-content :deep(td) {
  border: 1px solid var(--border);
  padding: 7px 12px;
  text-align: left;
}

.docs-content :deep(th) {
  background: var(--surface);
  color: var(--text-primary);
  font-weight: 600;
}

.docs-content :deep(blockquote) {
  margin: 10px 0;
  padding: 6px 14px;
  border-left: 3px solid var(--accent);
  background: var(--surface);
  color: var(--text-muted);
}

/* ─── Narrow windows: TOC collapses into a wrapping chip row above the content ─── */
@media (max-width: 900px) {
  .docs-layout {
    flex-direction: column;
  }

  .docs-toc {
    position: static;
    width: auto;
    flex-direction: row;
    flex-wrap: wrap;
  }

  .docs-content {
    scroll-margin-top: 0;
    width: 100%;
  }
}
</style>
