<template>
  <aside class="sidebar" :class="{ collapsed: appConfig.sidebarCollapsed }">
    <div class="sidebar-header">
      <div class="logo">
        <svg viewBox="0 0 32 32" class="logo-icon">
          <defs>
            <linearGradient id="logoGrad" x1="0%" y1="0%" x2="100%" y2="100%">
              <stop offset="0%" stop-color="#a78bfa" />
              <stop offset="100%" stop-color="#6366f1" />
            </linearGradient>
          </defs>
          <rect x="2" y="2" width="28" height="28" rx="8" fill="url(#logoGrad)" />
          <path d="M10 20 L16 8 L22 20" stroke="white" stroke-width="2.5" fill="none" stroke-linecap="round" stroke-linejoin="round" />
          <circle cx="16" cy="22" r="2" fill="white" />
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
        :title="appConfig.sidebarCollapsed ? item.label() : undefined"
      >
        <span class="nav-icon" v-html="item.icon"></span>
        <span class="nav-label">{{ item.label() }}</span>
        <span v-if="isActive(item.path)" class="active-indicator"></span>
      </router-link>
    </nav>

    <div class="sidebar-footer">
      <div class="status-dot"></div>
      <span class="status-text">{{ t('nav.ready') }}</span>
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
import { t } from '../lib/i18n'
import { appConfig, setSidebarCollapsed } from '../store'

const route = useRoute()

const navItems = [
  {
    path: '/',
    label: () => t('nav.home'),
    icon: `<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M3 9l9-7 9 7v11a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"/><polyline points="9 22 9 12 15 12 15 22"/></svg>`
  },
  {
    path: '/downloads',
    label: () => t('nav.downloads'),
    icon: `<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" y1="15" x2="12" y2="3"/></svg>`
  },
  {
    path: '/models',
    label: () => t('nav.models'),
    icon: `<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 16V8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16z"/><polyline points="3.27 6.96 12 12.01 20.73 6.96"/><line x1="12" y1="22.08" x2="12" y2="12"/></svg>`
  },
  {
    path: '/api',
    label: () => t('nav.api'),
    icon: `<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="4 17 10 11 4 5"/><line x1="12" y1="19" x2="20" y2="19"/></svg>`
  },
  {
    path: '/monitor',
    label: () => t('nav.monitor'),
    icon: `<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="3 3 3 21 21 21"/><polyline points="7 15 11 9 15 13 21 5"/></svg>`
  },
  {
    path: '/settings',
    label: () => t('nav.settings'),
    icon: `<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-2 2 2 2 0 0 1-2-2v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1-2-2 2 2 0 0 1 2-2h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 2-2 2 2 0 0 1 2 2v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 2 2 2 2 0 0 1-2 2h-.09a1.65 1.65 0 0 0-1.51 1z"/></svg>`
  }
]

function isActive(path: string): boolean {
  if (path === '/') return route.path === '/'
  return route.path.startsWith(path)
}
</script>

<style scoped>
.sidebar {
  width: 240px;
  min-width: 240px;
  height: 100vh;
  background: var(--bg-secondary);
  border-right: 1px solid var(--border);
  display: flex;
  flex-direction: column;
  --wails-draggable: drag;
  user-select: none;
  backdrop-filter: blur(20px);
  /* 展开 240px ↔ 收起 64px 之间的宽度过渡，曲线与 nav-item 的
     transition: all 0.2s cubic-bezier(0.4, 0, 0.2, 1) 保持一致 */
  transition: width 0.2s cubic-bezier(0.4, 0, 0.2, 1);
}

/* 收起态：64px 纯图标栏；文字全部 display:none 隐藏（不引入 v-if，
   避免与宽度过渡打架），图标经 justify-content:center 居中 */
.sidebar.collapsed {
  width: 64px;
  min-width: 64px;
}

.sidebar-header {
  padding: 28px 20px 20px;
  --wails-draggable: drag;
}

.logo {
  display: flex;
  align-items: center;
  gap: 12px;
}

.logo-icon {
  width: 32px;
  height: 32px;
  flex-shrink: 0;
  filter: drop-shadow(0 4px 8px rgba(99, 102, 241, 0.4));
}

.logo-text {
  font-size: 18px;
  font-weight: 700;
  background: linear-gradient(135deg, #a78bfa 0%, #6366f1 100%);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
  letter-spacing: -0.3px;
}

/* 收起态 header：只显示 logo 图标，水平内边距收窄到 16px 使 32px 图标居中 */
.sidebar.collapsed .sidebar-header {
  padding: 28px 16px 20px;
}

.sidebar.collapsed .logo {
  justify-content: center;
  gap: 0;
}

.sidebar.collapsed .logo-text {
  display: none;
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
}

/* 收起态 nav：label 隐藏、图标居中；active-indicator 保持 left:0 */
.sidebar.collapsed .nav-label {
  display: none;
}

.sidebar.collapsed .nav-item {
  justify-content: center;
  gap: 0;
  padding: 11px 0;
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
}

.status-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: #22c55e;
  box-shadow: 0 0 8px rgba(34, 197, 94, 0.5);
  animation: pulse 2s infinite;
}

@keyframes pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.5; }
}

.status-text {
  font-size: 12px;
  color: var(--text-dim);
}

/* 收起态 footer：整个状态区（状态点 + 「系统就绪」文字）隐藏，footer 仅剩
   切换按钮作为唯一 flex 子项居中显示。按钮绝对定位在收起态被解除（position:
   static），转为普通 flex 子项后由 justify-content:center 居中 */
.sidebar.collapsed .sidebar-footer {
  justify-content: center;
  padding: 12px 8px;
}

.sidebar.collapsed .status-dot,
.sidebar.collapsed .status-text {
  display: none;
}

/* 收起态按钮脱离 absolute 定位，作为 footer 的 flex 子项居中 */
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
</style>
