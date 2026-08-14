<template>
  <div class="app-layout">
    <Sidebar />
    <main class="main-content">
      <!-- Custom title bar -->
      <div class="title-bar" v-if="isDesktop">
        <div></div>
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
  </div>
</template>

<script setup lang="ts">
import { onMounted } from 'vue'
import Sidebar from './components/Sidebar.vue'
import UpdateModal from './components/UpdateModal.vue'
import { updateState, checkForUpdate, shouldAutoCheck, closeUpdateModal } from './lib/update'
import { t } from './lib/i18n'
import { getOS } from './wails'

const w = window as any
const isDesktop = !!(w.go || w.electronAPI)  // Wails or Electron

function minimize() {
  w.runtime?.WindowMinimise()
}
function maximize() {
  w.runtime?.WindowToggleMaximise()
}

// 关闭按钮：Windows 上缩到系统托盘（llama-server 可后台继续运行），
// 其他平台保持原直接退出行为。getOS 失败（如 vite 单独运行）时静默回退
// 直接退出，与既有 w.runtime?. 可选链风格一致。
async function closeWindow() {
  let onWindows = false
  try {
    const info = await getOS()
    onWindows = info.os === 'windows'
  } catch {
    // 后端不可用（vite 单独运行）：保持默认行为
  }
  if (onWindows) {
    w.runtime?.WindowHide()
  } else {
    w.runtime?.Quit()
  }
}

// 启动静默检查更新：距上次检查超过 48 小时才自动检查（本地时间），
// 发现新版本才弹出更新窗口，不打断使用。
onMounted(() => {
  if (shouldAutoCheck()) {
    checkForUpdate()
  }
})
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

.window-controls {
  display: flex;
  gap: 8px;
  --wails-draggable: no-drag;
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

/* Route transitions */
.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.2s ease, transform 0.2s ease;
}

.fade-enter-from {
  opacity: 0;
  transform: translateY(8px);
}

.fade-leave-to {
  opacity: 0;
  transform: translateY(-8px);
}
</style>
