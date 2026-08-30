<template>
  <nav class="mobile-nav">
    <router-link
      v-for="item in NAV_ITEMS"
      :key="item.path"
      :to="item.path"
      class="mobile-nav-item"
      :class="{ active: isActiveNav(item.path, route.path) }"
    >
      <span class="mobile-nav-icon" v-html="item.icon"></span>
      <span class="mobile-nav-label">{{ t(item.labelKey) }}</span>
    </router-link>
  </nav>
</template>

<script setup lang="ts">
import { useRoute } from 'vue-router'
import { t } from '../lib/i18n'
import { NAV_ITEMS, isActiveNav } from '../lib/navigation'

// Bottom tab bar of the mobile shell: shown only at the mobile breakpoint
// (<=767px, see lib/layout.ts MOBILE_MAX), where it replaces the sidebar.
// Entries and active state come from the shared navigation model, so this bar
// and the desktop sidebar can never drift apart. Its occupied height is
// published globally as --mobile-nav-height (see styles/global.css); App.vue's
// content area and TaskDock's bottom offset consume it to stay clear of the bar.
const route = useRoute()
</script>

<style scoped>
/* Hidden on desktop/tablet: the left sidebar is the only navigation there. */
.mobile-nav {
  display: none;
}

/* mobile (<=767px): fixed bottom tab bar, one icon+label entry per nav item.
   env(safe-area-inset-bottom) clears Android gesture-nav insets; the same
   expression feeds --mobile-nav-height in global.css (keep both in sync). */
@media (max-width: 767px) {
  .mobile-nav {
    position: fixed;
    left: 0;
    right: 0;
    bottom: 0;
    z-index: 40; /* under the TaskDock pill (50) and the update modal (1000) */
    display: flex;
    align-items: stretch;
    height: calc(58px + env(safe-area-inset-bottom, 0px));
    padding-bottom: env(safe-area-inset-bottom, 0px);
    background: var(--bg-secondary);
    border-top: 1px solid var(--border);
  }

  .mobile-nav-item {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 3px;
    text-decoration: none;
    color: var(--text-muted);
    font-size: 10px;
    font-weight: 500;
    -webkit-tap-highlight-color: transparent;
  }

  .mobile-nav-item.active {
    color: #a78bfa;
  }

  .mobile-nav-icon {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 20px;
    height: 20px;
  }

  .mobile-nav-label {
    line-height: 1;
    max-width: 100%;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
}
</style>
