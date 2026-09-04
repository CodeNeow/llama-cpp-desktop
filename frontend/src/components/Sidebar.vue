<template>
  <aside class="sidebar" :class="{ collapsed: appConfig.sidebarCollapsed }">
    <div class="sidebar-header">
      <div class="logo">
        <!-- Mascot brand mark v2: chibi llama head, subtle-3/4 turn
             (far side 85%), C1 drooping petal ears + blush — same geometry
             as build/appicon.svg -->
        <svg viewBox="0 0 96 96" class="logo-icon" aria-hidden="true">
          <defs>
            <linearGradient id="logoGrad" x1="0" y1="0" x2="1" y2="1">
              <stop offset="0%" stop-color="#6366F1" />
              <stop offset="55%" stop-color="#8B5CF6" />
              <stop offset="100%" stop-color="#A855F7" />
            </linearGradient>
          </defs>
          <rect width="96" height="96" rx="22" fill="url(#logoGrad)" />
          <path d="M29.19 32.25 C23.85 32.32 18.44 27.29 18.68 22.08 C18.79 19.47 21.57 19.2 25.43 22.25 C28.39 24.48 30.89 26.87 32.79 28.9 C33.54 29.8 33.99 30.22 34.24 30.52 Z" fill="#EEF2FF" stroke="#43377B" stroke-width="2.44" stroke-linejoin="round" />
          <path d="M62.25 40.5 C72.25 40.68 87.62 36.39 91.44 30.39 C93.03 27.88 90.53 25.52 85.84 27.25 C80.3 29.35 73.43 32.85 67.89 36.49 C66.02 37.77 64.2 38.86 63.07 39.54 C62.57 40 62.34 40.14 62.25 40.5 Z" fill="#EEF2FF" stroke="#43377B" stroke-width="2.44" stroke-linejoin="round" />
          <path d="M27.54 30.52 C24.02 29.89 20.31 26.18 20.43 22.98 C20.5 20.79 22.82 20.69 25.87 23.27 C28.08 25.18 29.98 27.22 31.48 29.01 Z" fill="#C9CFF7" />
          <path d="M65.98 37.95 C73.71 37.95 85.89 34.03 89.35 29.48 C90.35 27.79 88.62 26.2 85.21 27.48 C80.11 29.3 73.8 32.94 68.7 36.31 C67.11 37.27 66.02 37.77 65.98 37.95 Z" fill="#C9CFF7" />
          <path d="M19.31 60.38 C19.31 47.25 22.5 37.5 28.24 31.12 C30.79 28.5 33.34 26.25 35.57 24.75 C36.84 22.12 37.8 20.25 38.76 19.12 Q40.03 13.5 43.22 15.75 Q45.13 16.5 45.77 20.25 Q47.68 12.75 53.25 13.5 Q55.88 13.88 56.62 19.5 Q59.62 13.5 66 15.38 Q68.62 16.5 69.38 20.25 C72 24 75.38 28.12 77.62 33 C81.75 38.62 85.88 47.62 85.88 60.38 C85.88 75.38 70.5 81.38 48.75 81.38 C30.15 81.38 19.31 75 19.31 60.38 Z" fill="#EEF2FF" stroke="#43377B" stroke-width="2.44" stroke-linejoin="round" />
          <path d="M34.61 48.75 C34.61 41.25 40.67 37.12 49.12 37.12 C58.88 37.12 66 41.25 65.62 48.75 C65.44 55.5 62.62 61.12 57.38 64.12 C52.12 66.75 46.41 66.75 41.94 64.12 C37.48 61.12 34.77 55.5 34.61 48.75 Z" fill="#FFFFFF" stroke="#43377B" stroke-width="2.06" />
          <ellipse cx="58.5" cy="56.25" rx="4.31" ry="2.62" fill="#F2D8EC" />
          <ellipse cx="40.03" cy="55.5" rx="3.56" ry="2.25" fill="#F2D8EC" />
          <ellipse cx="52.12" cy="55.88" rx="2.44" ry="1.69" fill="#43377B" />
          <ellipse cx="55.12" cy="47.62" rx="2.81" ry="3.56" fill="#2A2547" />
          <ellipse cx="41.94" cy="48" rx="2.39" ry="3.03" fill="#2A2547" />
          <circle cx="54" cy="46.12" r="0.94" fill="#fff" />
          <circle cx="56.06" cy="49.12" r="0.45" fill="#fff" />
          <circle cx="41.15" cy="46.69" r="0.79" fill="#fff" />
          <circle cx="42.58" cy="49.69" r="0.38" fill="#fff" />
          <path d="M48.75 59.62 Q52.12 62.25 55.5 59.25" fill="none" stroke="#43377B" stroke-width="1.88" stroke-linecap="round" />
        </svg>
        <span class="logo-text">MyLlama</span>
      </div>

    </div>

    <nav class="sidebar-nav">
      <router-link
        v-for="item in navItems"
        :key="item.path"
        :to="item.path"
        class="nav-item"
        :class="{ active: isActive(item.path) }"
        :title="appConfig.sidebarCollapsed ? t(item.labelKey) : undefined"
      >
        <span class="nav-icon" v-html="item.icon"></span>
        <span class="nav-label">{{ t(item.labelKey) }}</span>
        <span v-if="isActive(item.path)" class="active-indicator"></span>
      </router-link>
    </nav>

    <div class="sidebar-footer">
      <div class="status-dot" :class="ready ? 'ok' : 'bad'"></div>
      <span class="status-text">{{ ready ? t('nav.ready') : t('nav.notReady') }}</span>
      <button
        class="collapse-toggle"
        :title="appConfig.sidebarCollapsed ? t('nav.expand') : t('nav.collapse')"
        :aria-label="appConfig.sidebarCollapsed ? t('nav.expand') : t('nav.collapse')"
        @click="setSidebarCollapsed(!appConfig.sidebarCollapsed)"
      >
        <svg v-if="appConfig.sidebarCollapsed" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="13 17 18 12 13 7"/><polyline points="6 17 11 12 6 7"/></svg>
        <svg v-else width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="11 17 6 12 11 7"/><polyline points="18 17 13 12 18 7"/></svg>
      </button>
    </div>
  </aside>
</template>

<script setup lang="ts">
import { useRoute } from 'vue-router'
import { ref, onMounted, onUnmounted } from 'vue'
import { t } from '../lib/i18n'
import { isSystemReady } from '../lib/systemReady'
import { SIDEBAR_AUTO_COLLAPSE_WIDTH } from '../lib/layout'
import { NAV_ITEMS, isActiveNav } from '../lib/navigation'
import { getLlamaCpp, getModels } from '../wails'
import { appConfig, setSidebarCollapsed } from '../store'

const route = useRoute()

// System ready state: llama.cpp is installed and at least one local model exists
const ready = ref(false)

/** Refresh ready state: queries llama.cpp and model list in parallel; fails silently degrade to not-ready */
async function refreshReady(): Promise<void> {
  let installed = false
  let models: Awaited<ReturnType<typeof getModels>> = []

  // Parallel queries, non-blocking; backend unavailable under standalone vite, silently caught
  try {
    const llamaInfo = await getLlamaCpp()
    installed = !!llamaInfo?.installed
  } catch {
    // Keep the default false when the backend is unavailable
  }

  try {
    models = await getModels()
  } catch {
    // Keep the default empty array when the backend is unavailable
  }

  ready.value = isSystemReady(installed, models.length)
}

onMounted(() => {
  refreshReady()
  // Refresh every 15s: after a llama.cpp or model download finishes, auto-updates within at most 15 seconds
  const timer = setInterval(refreshReady, 15000)
  onUnmounted(() => clearInterval(timer))

  // Narrow-window watcher: collapse once when the window crosses from wide to
  // narrow. No auto-expand on the way back — a user who manually expands the
  // sidebar in a narrow window is not fought by this watcher.
  const narrowQuery = window.matchMedia(`(max-width: ${SIDEBAR_AUTO_COLLAPSE_WIDTH}px)`)
  const onNarrowChange = (e: MediaQueryListEvent): void => {
    if (e.matches) setSidebarCollapsed(true)
  }
  narrowQuery.addEventListener('change', onNarrowChange)
  onUnmounted(() => narrowQuery.removeEventListener('change', onNarrowChange))
})

// Shared navigation model (lib/navigation.ts): the same entries feed the
// mobile bottom tab bar (MobileNav.vue), so both surfaces stay in sync.
const navItems = NAV_ITEMS

function isActive(path: string): boolean {
  return isActiveNav(path, route.path)
}
</script>

<style scoped>
.sidebar {
  width: 236px;
  min-width: 236px;
  height: 100vh;
  background: var(--bg-secondary);
  border-right: 1px solid var(--border);
  display: flex;
  flex-direction: column;
  --wails-draggable: drag;
  user-select: none;
  /* Expand 236px ↔ collapse 64px; width and min-width must transition together —
     an instant min-width jump would mask the expand animation */
  transition: width 0.2s cubic-bezier(0.4, 0, 0.2, 1), min-width 0.2s cubic-bezier(0.4, 0, 0.2, 1);
}

/* Collapsed state: 64px icon-only bar; text hides via max-width: 0 + opacity: 0
   (animatable, prevents mid-transition wrap); icons center via padding, not justify-content */
.sidebar.collapsed {
  width: 64px;
  min-width: 64px;
}

.sidebar-header {
  padding: 14px 12px 18px;
  --wails-draggable: drag;
}

.logo {
  display: flex;
  align-items: center;
  gap: 12px;
  transition: gap 0.2s cubic-bezier(0.4, 0, 0.2, 1);
}

.logo-icon {
  width: 32px;
  height: 32px;
  flex-shrink: 0;
  filter: none;
}

.logo-text {
  font-size: 16px;
  font-weight: 700;
  background: linear-gradient(135deg, #6366f1 0%, #a855f7 100%);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
  letter-spacing: -0.3px;
  white-space: nowrap;
  overflow: hidden;
  max-width: 140px;
  opacity: 1;
  transition: max-width 0.2s cubic-bezier(0.4, 0, 0.2, 1), opacity 0.2s ease;
}

/* Collapsed logo: gap animates to 0; the 32px icon stays at header padding-left 16px, exactly centered in the 64px rail */
.sidebar.collapsed .logo {
  gap: 0;
}

.sidebar.collapsed .logo-text {
  max-width: 0;
  opacity: 0;
}

.sidebar-nav {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 8px 12px;
  --wails-draggable: no-drag;
}

.nav-item {
  display: flex;
  align-items: center;
  gap: 11px;
  padding: 0 12px;
  height: 40px;
  border-radius: 12px;
  text-decoration: none;
  color: var(--text-muted);
  font-size: 13.5px;
  font-weight: 600;
  position: relative;
  transition: all 0.2s cubic-bezier(0.4, 0, 0.2, 1);
  cursor: pointer;
}

.nav-item:hover {
  color: var(--text-primary);
  background: var(--surface-2);
}

.nav-item.active {
  background: var(--grad-soft);
  color: #6d28d9;
  font-weight: 700;
}

html[data-theme='dark'] .nav-item.active {
  color: var(--accent-light);
}

.nav-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 20px;
  height: 20px;
  flex-shrink: 0;
  opacity: 0.75;
  transition: opacity 0.2s ease;
}

.nav-item.active .nav-icon {
  opacity: 1;
  color: #a78bfa;
}

.nav-label {
  line-height: 1;
  white-space: nowrap;
  overflow: hidden;
  max-width: 120px;
  opacity: 1;
  transition: max-width 0.2s cubic-bezier(0.4, 0, 0.2, 1), opacity 0.2s ease;
}

/* Collapsed nav: label shrinks to zero width; icon centered via horizontal padding
   (item inner width is 40px after nav padding 12px, (40 - 20) / 2 = 10 centers the 20px icon;
   active-indicator keeps left: 0) */
.sidebar.collapsed .nav-label {
  max-width: 0;
  opacity: 0;
}

.sidebar.collapsed .nav-item {
  gap: 0;
  padding: 11px 10px;
}

.active-indicator {
  position: absolute;
  left: -12px;
  top: 9px;
  bottom: 9px;
  width: 3px;
  border-radius: 0 3px 3px 0;
  background: linear-gradient(180deg, #a78bfa, #6366f1);
}

.sidebar-footer {
  padding: 12px 8px 2px;
  display: flex;
  align-items: center;
  gap: 10px;
  border-top: 1px dashed var(--border);
  --wails-draggable: no-drag;
  position: relative;
  transition: gap 0.2s cubic-bezier(0.4, 0, 0.2, 1), padding 0.2s cubic-bezier(0.4, 0, 0.2, 1);
}

.status-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: #22c55e;
  box-shadow: none;
  animation: pulse 2s infinite;
  transition: width 0.2s cubic-bezier(0.4, 0, 0.2, 1);
}

/* Ready: green breathing */
.status-dot.ok {
  background: #22c55e;
  box-shadow: none;
  animation: pulse 2s infinite;
}

/* Not ready: gray static */
.status-dot.bad {
  background: var(--text-dim);
  box-shadow: none;
  animation: none;
}

@keyframes pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.5; }
}

.status-text {
  font-size: 12px;
  color: var(--text-dim);
  white-space: nowrap;
  overflow: hidden;
  max-width: 100px;
  opacity: 1;
  transition: max-width 0.2s cubic-bezier(0.4, 0, 0.2, 1), opacity 0.2s ease;
}

/* Collapsed footer: status area (dot + text) shrinks to zero width, leaving the toggle
   as the only static flex child centered via justify-content. On expand the toggle returns
   to absolute right: 12px, tracking the growing right edge smoothly */
.sidebar.collapsed .sidebar-footer {
  justify-content: center;
  gap: 0;
  padding: 12px 8px;
}

/* Zero-width dot renders nothing (height stays 8px); pulse animation untouched */
.sidebar.collapsed .status-dot {
  width: 0;
}

.sidebar.collapsed .status-text {
  max-width: 0;
  opacity: 0;
}

/* Collapsed-state button leaves absolute positioning and centers as a flex child of the footer */
.sidebar.collapsed .collapse-toggle {
  position: static;
  transform: none;
}

.collapse-toggle {
  position: absolute;
  right: 12px;
  top: 50%;
  transform: translateY(-50%);
  display: flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  border: none;
  border-radius: 8px;
  background: transparent;
  color: var(--text-muted);
  cursor: pointer;
  padding: 0;
  transition: background 0.15s ease, color 0.15s ease;
}

.collapse-toggle:hover {
  background: var(--hover-bg);
  color: var(--text-primary);
}

/* mobile (<=767px): the bottom tab bar (MobileNav in App.vue's mobile shell)
   replaces the sidebar as navigation, freeing the full width for content */
@media (max-width: 767px) {
  .sidebar {
    display: none;
  }
}

/* tablet (768..1099px): force the COLLAPSED icon rail regardless of the
   persisted preference, so the content area keeps its space on roomy-but-not
   wide screens. Pure CSS — the stored sidebarCollapsed value is untouched, so
   on desktop (>1099px, outside this range) the user's own preference still
   wins. The rules mirror .sidebar.collapsed one-for-one; when the persisted
   preference happens to be collapsed both sets agree. */
@media (min-width: 768px) and (max-width: 1099px) {
  .sidebar {
    width: 64px;
    min-width: 64px;
  }

  .sidebar .logo {
    gap: 0;
  }

  .sidebar .logo-text {
    max-width: 0;
    opacity: 0;
  }

  .sidebar .nav-item {
    gap: 0;
    padding: 11px 10px;
  }

  .sidebar .nav-label {
    max-width: 0;
    opacity: 0;
  }

  .sidebar .sidebar-footer {
    justify-content: center;
    gap: 0;
    padding: 12px 8px;
  }

  .sidebar .status-dot {
    width: 0;
  }

  .sidebar .status-text {
    max-width: 0;
    opacity: 0;
  }

  .sidebar .collapse-toggle {
    position: static;
    transform: none;
  }
}
</style>
