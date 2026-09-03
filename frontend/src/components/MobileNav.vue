<template>
  <nav class="mobile-nav">
    <router-link
      v-for="item in NAV_ITEMS"
      :key="item.path"
      :to="item.path"
      class="mobile-nav-item"
      :class="{ active: isActiveNav(item.path, route.path) }"
    >
      <!-- Phone tier prefers the design draft glyph (2.2 stroke, see
           lib/navigation.ts phoneIcon); API / settings have no phone-tier variant
           and fall back to the shared sidebar icon, which already matches. -->
      <span class="mobile-nav-icon" v-html="item.phoneIcon ?? item.icon"></span>
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
// Floating glass style per the v1 mobile design draft frame ① (14px side and
// bottom insets, 68px tall, blur+saturate backdrop, gradient pill behind the
// active entry). Entries and active state come from the shared navigation
// model, so this bar and the desktop sidebar can never drift apart. Its
// occupied band — bottom float gap + bar height + safe-area inset — is
// published globally as --mobile-nav-height (see styles/global.css); App.vue's
// content area and TaskDock's bottom offset consume it to stay clear of the bar.
const route = useRoute()
</script>

<style scoped>
/* Hidden on desktop/tablet: the left sidebar is the only navigation there. */
.mobile-nav {
  display: none;
}

/* mobile (<=767px): floating glass tab bar (v1 mobile design draft frame
   ①): a 68px rounded island hovering 14px above the bottom edge with a
   blur+saturate backdrop; the active entry gets a gradient pill with a white
   label. --safe-area-bottom (env() composed with the Android edge-to-edge
   bridge's JS inset, see styles/global.css) lifts the island clear of
   Android gesture-nav insets; the occupied band (14px gap + 68px bar +
   inset) feeds --mobile-nav-height in global.css (keep both in sync). */
@media (max-width: 767px) {
  .mobile-nav {
    position: fixed;
    left: 14px;
    right: 14px;
    bottom: calc(14px + var(--safe-area-bottom, 0px));
    z-index: 40; /* under the TaskDock pill (50) and the update modal (1000) */
    display: flex;
    align-items: center;
    justify-content: space-around;
    height: 68px;
    background: var(--glass);
    border: 1px solid var(--glass-line);
    border-radius: 26px;
    box-shadow: none;
  }

  .mobile-nav-item {
    flex: 0 1 62px; /* design .gn-item width; shrinks below it on <360px screens */
    min-width: 0;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 3px;
    padding: 8px 0;
    border-radius: 18px;
    text-decoration: none;
    color: var(--text-muted);
    font-size: 10.5px;
    font-weight: 500;
    -webkit-tap-highlight-color: transparent;
  }

  /* Selected entry: brand-gradient pill with white label (design frame ①) */
  .mobile-nav-item.active {
    color: #fff;
    background: var(--grad);
    font-weight: 700;
    box-shadow: none;
  }

  .mobile-nav-icon {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 22px;
    height: 22px;
  }

  /* The nav SVGs are injected via v-html (no scoped data attribute), so the
     22px design size needs :deep to override their width/height attributes */
  .mobile-nav-icon :deep(svg) {
    width: 22px;
    height: 22px;
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
