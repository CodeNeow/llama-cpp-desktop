<template>
  <div class="page page-fixed">
    <div class="sticky-top">
      <div class="page-header">
        <div class="page-header-row">
          <div>
            <h1 class="page-title">{{ t('home.title') }}</h1>
            <p class="page-subtitle">{{ t('home.subtitle') }}</p>
          </div>
        </div>
      </div>

      <!-- Tabs row: same pattern as Models.vue (icon + label buttons with an
           underline active state). The active tab is derived from the route and
           clicks push the child route, so deep links and back/forward
           navigation stay correct. -->
      <div class="env-tabs" role="tablist" :aria-label="t('home.title')">
        <button
          v-for="tab in tabs"
          :key="tab.id"
          :id="`${tab.id}-tab`"
          class="tab-btn"
          :class="{ active: activeTabId === tab.id }"
          role="tab"
          :aria-selected="activeTabId === tab.id"
          :aria-controls="panelId"
          @click="router.push(tab.route)"
        >
          <span class="tab-icon" v-html="tab.icon"></span>
          {{ tab.label() }}
        </button>
      </div>
    </div>

    <!-- Tab panel region: doubles as the page's scroll band (fixed-viewport
         layout — only this region scrolls, never the page). Nested route
         children render through keep-alive, so switching tabs preserves the
         hardware samples and the runtime download state instead of remounting
         them. The bare / route lands on EnvironmentDefault, which resolves the
         smart default tab and replaces itself. -->
    <div :id="panelId" role="tabpanel" :aria-labelledby="`${activeTabId}-tab`" class="page-scroll">
      <router-view v-slot="{ Component }">
        <keep-alive>
          <component :is="Component" />
        </keep-alive>
      </router-view>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { t } from '../lib/i18n'

const route = useRoute()
const router = useRouter()

// Both tabs control the single panel region rendered by the nested router-view
const panelId = 'home-panel'

// Active tab follows the route: /runtime selects the runtime tab, everything
// else under the shell (/system, and the bare / while the default resolver is
// still probing) shows the system tab
const activeTabId = computed(() => (route.path.startsWith('/runtime') ? 'tab-runtime' : 'tab-system'))

// Tab definitions (icons are inline stroke SVGs, mirroring Models.vue)
const tabs = [
  {
    id: 'tab-system',
    route: '/system',
    label: () => t('home.tabSystem'),
    icon: `<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="4" y="4" width="16" height="16" rx="2" ry="2"/><rect x="9" y="9" width="6" height="6"/><line x1="9" y1="1" x2="9" y2="4"/><line x1="15" y1="1" x2="15" y2="4"/><line x1="9" y1="20" x2="9" y2="23"/><line x1="15" y1="20" x2="15" y2="23"/><line x1="20" y1="9" x2="23" y2="9"/><line x1="20" y1="14" x2="23" y2="14"/><line x1="1" y1="9" x2="4" y2="9"/><line x1="1" y1="14" x2="4" y2="14"/></svg>`,
  },
  {
    id: 'tab-runtime',
    route: '/runtime',
    label: () => t('home.tabRuntime'),
    icon: `<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polygon points="12 2 2 7 12 12 22 7 12 2"/><polyline points="2 17 12 22 22 17"/><polyline points="2 12 12 17 22 12"/></svg>`,
  },
]
</script>

<style scoped>
.page {
  /* No top padding: header flush with content top, title aligns with sidebar logo (see global.css .page-header).
     No bottom padding either: the fixed layout (page-fixed) hands bottom spacing
     to the .page-scroll band below */
  padding: 0 48px;
}

/* Fixed-viewport layout (see .page-fixed in global.css): the header band stays
   pinned; only the content band below scrolls */
.page-fixed .sticky-top {
  flex-shrink: 0;
}

.page-header {
  /* Use padding instead of margin: header background covers this gap so content scrolls without leaving a seam.
     Tighter than a standalone page: the tab row below completes the sticky block (same as Models.vue) */
  padding-bottom: 20px;
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

.page-header-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
}

/* ─── Tabs (same pattern as Models.vue) ─── */
.env-tabs {
  display: flex;
  gap: 4px;
  padding: 0;
  border-bottom: 1px solid var(--border);
  margin-bottom: 20px;
  flex-shrink: 0;
}

.tab-btn {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 10px 14px;
  background: none;
  border: none;
  border-bottom: 2px solid transparent;
  margin-bottom: -1px;
  color: var(--text-muted);
  font-size: 13px;
  font-weight: 500;
  font-family: inherit;
  cursor: pointer;
  transition: color 0.15s, border-color 0.15s;
}

.tab-btn:hover {
  color: var(--text-secondary);
}

.tab-btn.active {
  color: var(--accent);
  border-bottom-color: var(--accent);
  font-weight: 600;
}

.tab-btn .tab-icon {
  display: flex;
}

/* ─── Scrollable content band (also the tab panel region) ─── */
.page-scroll {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  /* Bottom clearance so the last row of cards clears the floating TaskDock
     pill (--dock-reserve is bound globally by App.vue; 0 while hidden) */
  padding-bottom: calc(24px + var(--dock-reserve, 0px));
}

/* Thin scrollbar matching .content-area (App.vue) so the inner band does not
   render a full-width system scrollbar */
.page-scroll::-webkit-scrollbar {
  width: 6px;
}

.page-scroll::-webkit-scrollbar-track {
  background: transparent;
}

.page-scroll::-webkit-scrollbar-thumb {
  background: var(--overlay-10);
  border-radius: 3px;
}

.page-scroll::-webkit-scrollbar-thumb:hover {
  background: var(--scrollbar-thumb-hover);
}
</style>
