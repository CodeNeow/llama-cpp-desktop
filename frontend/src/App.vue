<template>
  <div class="app-layout">
    <Sidebar />
    <main class="main-content">
      <!-- Custom title bar -->
      <div class="title-bar" v-if="isDesktop">
        <div></div>
        <!-- macOS keeps the colorful dots (matches existing style); Windows / Linux / unknown use native flat buttons -->
        <template v-if="platform === 'darwin'">
          <div class="window-controls">
            <button class="win-btn win-min" @click="minimize" :title="t('title.minimize')">
              <svg width="12" height="12" viewBox="0 0 12 12"><rect x="1" y="5.5" width="10" height="1" fill="currentColor"/></svg>
            </button>
            <button class="win-btn win-max" @click="maximize" :title="t('title.maximize')">
              <svg width="12" height="12" viewBox="0 0 12 12"><rect x="1.5" y="1.5" width="9" height="9" stroke="currentColor" stroke-width="1" fill="none"/></svg>
            </button>
            <button class="win-btn win-close" @click="closeWindow" :title="t('title.close')">
              <svg width="12" height="12" viewBox="0 0 12 12"><line x1="1" y1="1" x2="11" y2="11" stroke="currentColor" stroke-width="1.5"/><line x1="11" y1="1" x2="1" y2="11" stroke="currentColor" stroke-width="1.5"/></svg>
            </button>
          </div>
        </template>
        <!-- Windows / Linux / unknown: native flat buttons, 46x36 px filling the title bar -->
        <div v-else class="window-controls native">
          <button class="native-btn" @click="minimize" :title="t('title.minimize')">
            <svg width="10" height="10" viewBox="0 0 10 10"><line x1="1" y1="5" x2="9" y2="5" stroke="currentColor" stroke-width="1"/></svg>
          </button>
          <button class="native-btn" @click="maximize" :title="isMax ? t('title.restore') : t('title.maximize')">
            <!-- Maximize: hollow square; restore: two overlapping squares (rear offset 3,3 semi-transparent + front 0.5,0.5) -->
            <svg v-if="!isMax" width="10" height="10" viewBox="0 0 10 10"><rect x="1" y="1" width="8" height="8" stroke="currentColor" stroke-width="1" fill="none"/></svg>
            <svg v-else width="10" height="10" viewBox="0 0 10 10">
              <rect x="3" y="3" width="7" height="7" stroke="currentColor" stroke-width="1" fill="none" opacity="0.35"/>
              <rect x="0.5" y="0.5" width="7" height="7" stroke="currentColor" stroke-width="1" fill="none"/>
            </svg>
          </button>
          <button class="native-btn close" @click="closeWindow" :title="t('title.close')">
            <svg width="10" height="10" viewBox="0 0 10 10"><line x1="1" y1="1" x2="9" y2="9" stroke="currentColor" stroke-width="1.2"/><line x1="9" y1="1" x2="1" y2="9" stroke="currentColor" stroke-width="1.2"/></svg>
          </button>
        </div>
      </div>
      <div class="content-area">
        <router-view v-slot="{ Component, route }">
          <transition :name="(route.meta.transition as string) || 'fade'" mode="out-in">
            <component :is="Component" :key="route.path" />
          </transition>
        </router-view>
      </div>
    </main>
    <UpdateModal :visible="updateState.showModal" @close="closeUpdateModal" />
    <TaskDock />
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import Sidebar from './components/Sidebar.vue'
import UpdateModal from './components/UpdateModal.vue'
import TaskDock from './components/TaskDock.vue'
import { updateState, checkForUpdate, shouldAutoCheck, closeUpdateModal } from './lib/update'
import { t } from './lib/i18n'
import { appConfig } from './store'
import { getOS } from './wails'

const w = window as any
const isDesktop = !!(w.go || w.electronAPI)  // Wails or Electron

// Current OS: 'darwin' (macOS) / 'windows' / 'linux' / empty string (unknown or backend unavailable)
// Drives window-control button platform adaptation: macOS keeps the colorful dots, other platforms use native flat buttons
const platform = ref('')
const isMax = ref(false)  // Maximized state: query the Wails API when possible, fall back to a local toggle

// Detect the OS on startup; on failure silently keep the empty string (same style as existing getOS optional chaining)
onMounted(async () => {
  try {
    const info = await getOS()
    platform.value = (info as { os?: string }).os ?? ''
  } catch {
    // Backend unavailable (standalone vite) or parse failure: keep the default empty string
  }
  // Existing silent update check logic unchanged
  if (shouldAutoCheck()) {
    checkForUpdate()
  }
})

function minimize() {
  w.runtime?.WindowMinimise()
}

// Maximize/restore: toggle the window; flip the icon state locally first for instant feedback,
// then correct it later against the real state if the runtime exposes WindowIsMaximised
// (Toggle is asynchronous, so an immediate query could read a stale value).
function maximize() {
  w.runtime?.WindowToggleMaximise()
  isMax.value = !isMax.value
  if (w.runtime?.WindowIsMaximised) {
    setTimeout(async () => {
      try {
        isMax.value = await w.runtime.WindowIsMaximised()
      } catch {
        // On query failure keep the locally flipped value
      }
    }, 150)
  }
}

// Close button: only minimize to tray when on Windows with the system tray enabled
// (Settings page toggle; llama-server keeps running in the background); otherwise quit.
// Tray state is read from the store cache — main.ts already ran loadConfig before
// mount, so the backend config (including persisted trayEnabled) is guaranteed to be
// loaded by the time the user clicks close; no extra getConfig call needed. If getOS
// fails (e.g. standalone vite), silently fall back to a direct quit, matching the
// existing w.runtime?. optional-chaining style.
async function closeWindow() {
  let onWindows = false
  try {
    const info = await getOS()
    onWindows = info.os === 'windows'
  } catch {
    // Backend unavailable (standalone vite): keep default behavior
  }
  if (onWindows && appConfig.trayEnabled) {
    w.runtime?.WindowHide()
  } else {
    w.runtime?.Quit()
  }
}
</script>

<style scoped>
.app-layout {
  display: flex;
  width: 100vw;
  height: 100vh;
  overflow: hidden;
  background: var(--bg-primary);
}

.main-content {
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

/* ─── Title bar ─── */
.title-bar {
  height: 36px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 12px;
  --wails-draggable: drag;
  user-select: none;
  flex-shrink: 0;
}

/* ─── Two window-control button styles ───
 * darwin (macOS): .win-btn colorful dots (yellow/green/red), matching the system look;
 * windows / linux / unknown: .window-controls.native flat square-corner buttons,
 *   36px tall filling the title bar, hover background var(--overlay-20), close button hover red with white glyph.
 * Trigger: render the dots when platform === 'darwin', otherwise the native flat group.
 * --wails-draggable: no-drag preserved: .window-controls existing setting carries over to the new button group.
 */
.window-controls {
  display: flex;
  gap: 8px;
  --wails-draggable: no-drag;
}

.window-controls.native {
  gap: 0;  /* Native buttons sit flush together, no spacing */
}

.native-btn {
  width: 46px;
  height: 36px;
  border: none;
  border-radius: 0;
  background: transparent;
  color: var(--text-secondary);
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 0;
  transition: background 0.1s ease;
}

.native-btn:hover {
  background: var(--overlay-20);
}

/* Close button hover: red background, white glyph */
.native-btn.close:hover {
  background: #e81123;
  color: #fff;
}

.win-btn {
  width: 18px;
  height: 18px;
  border-radius: 50%;
  border: none;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 0;
  transition: opacity 0.15s;
  opacity: 0.6;
}

.win-btn:hover { opacity: 1; }

.win-min {
  background: #fbbf24;
  color: #92400e;
}

.win-max {
  background: #22c55e;
  color: #14532d;
}

.win-close {
  background: #ef4444;
  color: #7f1d1d;
}

/* ─── Content area ─── */
.content-area {
  flex: 1;
  overflow-y: auto;
  overflow-x: hidden;
}

.content-area::-webkit-scrollbar {
  width: 6px;
}

.content-area::-webkit-scrollbar-track {
  background: transparent;
}

.content-area::-webkit-scrollbar-thumb {
  background: var(--overlay-10);
  border-radius: 3px;
}

.content-area::-webkit-scrollbar-thumb:hover {
  background: var(--scrollbar-thumb-hover);
}

/* Route transitions — opacity-only, avoiding sub-pixel horizontal jumps from composited-layer/layout switches during translate animations (fixes centered-text jitter of the chat page offline hint) */
.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.2s ease;
}

.fade-enter-from {
  opacity: 0;
}

.fade-leave-to {
  opacity: 0;
}
</style>
