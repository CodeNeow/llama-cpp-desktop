<template>
  <div class="page page-fixed">
    <div class="sticky-top">
      <div class="page-header">
        <div class="page-header-row">
          <div class="greet-block">
            <!-- Greeting header (design/android-mockups.html frame ①): a
                 time-of-day greeting replaces the static page title, and the
                 subline answers "is the AI usable right now" from the live
                 service status + resident model — never hardcoded online. -->
            <h1 class="greet-title">{{ t(greetKey) }}</h1>
            <p class="greet-sub">
              <template v-if="greetState === 'online'">
                <span class="greet-live">
                  <span class="greet-dot on"></span>
                  <span class="greet-model" :title="greetModel">{{ greetLine }}</span>
                </span>
                <span class="greet-rest">· {{ t('home.greet.readySuffix') }}</span>
              </template>
              <template v-else>
                <span class="greet-dot" :class="{ idle: greetState === 'idle' }"></span>
                <span class="greet-text">{{ greetLine }}</span>
              </template>
            </p>
          </div>
        </div>
      </div>

      <!-- Tabs row: same pattern as Models.vue (icon + label buttons with an
           underline active state). The active tab is derived from the route and
           clicks push the child route, so deep links and back/forward
           navigation stay correct. The row's right side carries the system
           tab's updated-at stamp + refresh toolbar, wired to the SystemInfoTab
           instance through its exposed API and shown only while that tab is
           active. -->
      <div class="env-tabs-row">
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
        <div v-if="activeTabId === 'tab-system'" class="header-actions">
          <span v-if="systemLastUpdated" class="updated-at">{{ t('home.updatedAt', { time: systemLastUpdated }) }}</span>
          <button class="refresh-btn" :disabled="systemRefreshing" @click="refreshSystem">
            <svg :class="{ spinning: systemRefreshing }" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <polyline points="23 4 23 10 17 10"/><polyline points="1 20 1 14 7 14"/>
              <path d="M3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15"/>
            </svg>
            {{ systemRefreshing ? t('home.refreshing') : t('home.refresh') }}
          </button>
        </div>
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
          <component :is="Component" ref="systemTabRef" />
        </keep-alive>
      </router-view>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { t } from '../lib/i18n'
import { getServerStatus, getLoadedModels, type LoadedModel } from '../wails'

const route = useRoute()
const router = useRouter()

// ─── Greeting header (frame ①) ──────────────────────────────────
// Time-of-day greeting + honest service status. The shell polls the light
// service status pair (getServerStatus + getLoadedModels) every 3s — the same
// facts TaskDock displays, at the SystemInfoTab live-sample cadence. No data
// is invented: before the first successful probe the subline shows a neutral
// loading state, and a stopped service says so.

const now = ref(new Date())
const serverRunning = ref(false)
const statusResolved = ref(false)
const loadedModels = ref<LoadedModel[]>([])

type GreetSlot = 'morning' | 'afternoon' | 'evening'

/** Time-of-day bucket: morning before 12:00, afternoon 12:00–17:59, evening after. */
function greetSlot(d: Date): GreetSlot {
  const h = d.getHours()
  if (h < 12) return 'morning'
  if (h < 18) return 'afternoon'
  return 'evening'
}

const greetKey = computed(() => `home.greet.${greetSlot(now.value)}`)

/** First resident model: prefer one actually loaded, else any loading/sleeping entry. */
const residentModel = computed<LoadedModel | null>(
  () => loadedModels.value.find(m => m.status === 'loaded') ?? loadedModels.value[0] ?? null
)

const greetState = computed<'loading' | 'online' | 'idle' | 'off'>(() => {
  if (!statusResolved.value) return 'loading'
  if (!serverRunning.value) return 'off'
  return residentModel.value ? 'online' : 'idle'
})

const greetModel = computed(() => residentModel.value?.id ?? '')

const greetLine = computed(() => {
  switch (greetState.value) {
    case 'online':
      return t('home.greet.online', { model: residentModel.value!.id })
    case 'idle':
      return t('home.greet.idle')
    case 'off':
      return t('home.greet.offline')
    default:
      return t('home.greet.loading')
  }
})

let statusTimer: ReturnType<typeof setInterval> | null = null

async function pollServiceStatus() {
  try {
    const s = await getServerStatus()
    serverRunning.value = s.running
    if (s.running) {
      try {
        loadedModels.value = await getLoadedModels()
      } catch {
        // Transient router query failure: keep the previous list until next tick
      }
    } else {
      loadedModels.value = []
    }
    statusResolved.value = true
    now.value = new Date()
  } catch {
    // Backend unavailable (e.g. standalone vite): keep the honest loading state
  }
}

onMounted(() => {
  pollServiceStatus()
  statusTimer = setInterval(pollServiceStatus, 3000)
})

onUnmounted(() => {
  if (statusTimer) {
    clearInterval(statusTimer)
    statusTimer = null
  }
})

// Both tabs control the single panel region rendered by the nested router-view
const panelId = 'home-panel'

// Active tab follows the route: /runtime selects the runtime tab, everything
// else under the shell (/system, and the bare / while the default resolver is
// still probing) shows the system tab
const activeTabId = computed(() => (route.path.startsWith('/runtime') ? 'tab-runtime' : 'tab-system'))

// Exposed API of the system tab (SystemInfoTab.vue defineExpose): the refresh
// toolbar beside the tab bar delegates the manual re-probe to the active
// system tab instance and mirrors refreshing/lastUpdated. Optional chaining
// everywhere: while the EnvironmentDefault resolver or the RuntimeSection
// child is the bound instance, the exposed API is absent.
interface SystemTabExposed {
  refresh: () => Promise<void> | void
  refreshing: boolean
  lastUpdated: string
}
const systemTabRef = ref<SystemTabExposed | null>(null)
const systemRefreshing = computed(() => systemTabRef.value?.refreshing ?? false)
const systemLastUpdated = computed(() => systemTabRef.value?.lastUpdated ?? '')

/** Header refresh: forces the system tab to re-probe the hardware snapshot. */
function refreshSystem() {
  systemTabRef.value?.refresh()
}

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

/* ─── Greeting header (frame ①): time-of-day greeting replaces the static
       page title; the status subline keeps to one line with an ellipsized
       model id so long router ids cannot push the header band wider ─── */
.greet-block {
  min-width: 0;
}

.greet-title {
  font-size: 26px;
  font-weight: 800;
  color: var(--text-primary);
  margin: 0 0 6px;
  letter-spacing: -0.4px;
  line-height: 1.2;
}

.greet-sub {
  display: flex;
  align-items: center;
  gap: 7px;
  font-size: 13px;
  color: var(--text-dim);
  margin: 0;
  min-width: 0;
  font-variant-numeric: tabular-nums;
}

.greet-live {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  color: var(--success);
  font-weight: 700;
  min-width: 0;
}

.greet-model {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.greet-rest {
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.greet-text {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.greet-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: var(--text-dim);
  flex-shrink: 0;
}

/* Online/resident: glowing green dot (design frame ① .live) */
.greet-dot.on {
  background: var(--success);
  box-shadow: 0 0 6px var(--success);
}

/* Service up but no resident model yet: green but unlit */
.greet-dot.idle {
  background: var(--success);
  opacity: 0.55;
}

.page-header-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
}

/* ─── Tabs row (same pattern as Models.vue): tab bar left, actions right.
       Border and bottom gap live on the row so the system-tab toolbar shares
       the same underline and baseline ─── */
.env-tabs-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  border-bottom: 1px solid var(--border);
  margin-bottom: 20px;
  flex-shrink: 0;
}

.env-tabs {
  display: flex;
  gap: 4px;
  padding: 0;
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-shrink: 0;
}

.updated-at {
  font-size: 12px;
  color: var(--text-dim);
  white-space: nowrap;
  font-variant-numeric: tabular-nums;
}

.refresh-btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 6px 14px;
  background: transparent;
  color: var(--text-secondary);
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.2s;
}

.refresh-btn:hover:not(:disabled) {
  background: var(--hover-bg);
  border-color: var(--overlay-20);
  color: var(--text-primary);
}

.refresh-btn:disabled {
  opacity: 0.6;
  cursor: default;
}

.refresh-btn svg.spinning {
  animation: spin 0.8s linear infinite;
}

@keyframes spin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
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

/* ─── Phone (<=767px): the tab row must never push the page wider than the
       viewport. Tabs scroll horizontally inside their own container (page
       overflow stays impossible); the refresh toolbar wraps onto its own row.
       Tab buttons get 44px touch height. The greeting scales down per the
       design's 24px phone heading. ─── */
@media (max-width: 767px) {
  .greet-title {
    font-size: 24px;
  }

  .greet-rest {
    display: none;
  }

  .env-tabs-row {
    flex-wrap: wrap;
    gap: 0 12px;
  }

  .env-tabs {
    flex: 1 1 100%;
    min-width: 0;
    overflow-x: auto;
    scrollbar-width: none;
  }

  .env-tabs::-webkit-scrollbar {
    display: none;
  }

  .tab-btn {
    flex-shrink: 0;
    padding: 12px 14px;
  }

  .header-actions {
    flex: 1 1 100%;
    justify-content: space-between;
    padding-bottom: 10px;
  }

  .refresh-btn {
    min-height: 36px;
  }
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
