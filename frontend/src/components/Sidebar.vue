<template>
  <aside class="sidebar" :class="{ collapsed: appConfig.sidebarCollapsed }">
    <div class="sidebar-header">
      <div class="logo">
        <!-- Concept-C brand mark: brand-gradient rounded plate + white L
             glyph + AI sparkle (same geometry as build/appicon.svg) -->
        <svg viewBox="0 0 96 96" class="logo-icon" aria-hidden="true">
          <defs>
            <linearGradient id="logoGrad" x1="0" y1="0" x2="1" y2="1">
              <stop offset="0%" stop-color="#6366F1" />
              <stop offset="55%" stop-color="#8B5CF6" />
              <stop offset="100%" stop-color="#A855F7" />
            </linearGradient>
          </defs>
          <rect width="96" height="96" rx="22" fill="url(#logoGrad)" />
          <path d="M28 16 h17 v45 h30 v17 H28 Z" fill="#fff" />
          <path d="M69 12 l4 10 10 4 -10 4 -4 10 -4 -10 -10 -4 10 -4 Z" fill="#fff" opacity=".95" />
        </svg>
        <span class="logo-text">Llama Desktop</span>
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
  width: 200px;
  min-width: 200px;
  height: 100vh;
  background: var(--bg-secondary);
  border-right: 1px solid var(--border);
  display: flex;
  flex-direction: column;
  --wails-draggable: drag;
  user-select: none;
  backdrop-filter: blur(20px);
  /* Expand 200px ↔ collapse 64px; width and min-width must transition together —
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
  padding: 28px 16px 20px;
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
  filter: drop-shadow(0 4px 8px rgba(99, 102, 241, 0.4));
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
  gap: 12px;
  padding: 11px 14px;
  border-radius: 10px;
  text-decoration: none;
  color: var(--text-muted);
  font-size: 14px;
  font-weight: 500;
  position: relative;
  transition: all 0.2s cubic-bezier(0.4, 0, 0.2, 1);
  cursor: pointer;
}

.nav-item:hover {
  color: var(--text-primary);
  background: var(--hover-bg);
}

.nav-item.active {
  color: var(--text-primary);
  background: var(--active-bg);
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
  left: 0;
  top: 50%;
  transform: translateY(-50%);
  width: 3px;
  height: 20px;
  border-radius: 0 3px 3px 0;
  background: linear-gradient(180deg, #a78bfa, #6366f1);
}

.sidebar-footer {
  padding: 16px 20px;
  display: flex;
  align-items: center;
  gap: 10px;
  border-top: 1px solid var(--border);
  --wails-draggable: no-drag;
  position: relative;
  transition: gap 0.2s cubic-bezier(0.4, 0, 0.2, 1), padding 0.2s cubic-bezier(0.4, 0, 0.2, 1);
}

.status-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: #22c55e;
  box-shadow: 0 0 8px rgba(34, 197, 94, 0.5);
  animation: pulse 2s infinite;
  transition: width 0.2s cubic-bezier(0.4, 0, 0.2, 1);
}

/* Ready: green breathing */
.status-dot.ok {
  background: #22c55e;
  box-shadow: 0 0 8px rgba(34, 197, 94, 0.5);
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
