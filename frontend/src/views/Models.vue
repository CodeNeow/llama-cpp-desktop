<template>
  <div class="page">
    <div class="sticky-top">
      <div class="page-header">
        <h1 class="page-title">{{ t('models.title') }}</h1>
      </div>

      <!-- Tabs row: same pattern as Settings.vue (icon + label buttons with an
           underline active state). Unlike Settings, the active tab is derived
           from the route and clicks push the child route, so deep links and
           back/forward navigation stay correct. -->
      <div class="models-tabs" role="tablist" :aria-label="t('models.title')">
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

    <!-- Tab panel region: nested route children rendered through a keep-alive,
         so switching tabs preserves the search input/results and the local
         model list instead of remounting them. Each tab owns its own actions:
         the local tab's rescan button lives in its directory bar. -->
    <div :id="panelId" role="tabpanel" :aria-labelledby="`${activeTabId}-tab`">
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
const panelId = 'models-panel'

// Active tab follows the route: /models/local selects the local tab, anything
// else under /models (including /models/download) shows the download tab
const activeTabId = computed(() => (route.path.startsWith('/models/local') ? 'tab-local' : 'tab-download'))

// Tab definitions (download first: it is the default landing tab; icons are
// inline stroke SVGs, mirroring Settings.vue)
const tabs = [
  {
    id: 'tab-download',
    route: '/models/download',
    label: () => t('models.tabDownload'),
    icon: `<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" y1="15" x2="12" y2="3"/></svg>`,
  },
  {
    id: 'tab-local',
    route: '/models/local',
    label: () => t('models.tabLocal'),
    icon: `<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 16V8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16z"/><polyline points="3.27 6.96 12 12.01 20.73 6.96"/><line x1="12" y1="22.08" x2="12" y2="12"/></svg>`,
  },
]
</script>

<style scoped>
.page {
  /* No top padding: header flush with content top, title aligns with sidebar logo (see global.css .page-header) */
  padding: 0 48px 60px;
}

.page-header {
  /* Use padding instead of margin: header background covers this gap so content scrolls without leaving a seam.
     Tighter than a standalone page: the tab row below completes the sticky block (same as Settings) */
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

/* ─── Tabs (same pattern as Settings.vue) ─── */
.models-tabs {
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
</style>
