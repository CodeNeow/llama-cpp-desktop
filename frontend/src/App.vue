<template>
  <div class="app-layout">
    <Sidebar />
    <main class="main-content">
      <!-- Custom title bar -->
      <div class="title-bar" v-if="isDesktop">
        <div></div>
        <!-- macOS 保留彩色圆点（与现有风格一致）；Windows / Linux / 未知使用原生扁平按钮 -->
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
        <!-- Windows / Linux / 未知：原生扁平按钮，宽 46×高 36px 填满标题栏 -->
        <div v-else class="window-controls native">
          <button class="native-btn" @click="minimize" :title="t('title.minimize')">
            <svg width="10" height="10" viewBox="0 0 10 10"><line x1="1" y1="9" x2="9" y2="9" stroke="currentColor" stroke-width="1"/></svg>
          </button>
          <button class="native-btn" @click="maximize" :title="isMax ? t('title.restore') : t('title.maximize')">
            <!-- 最大化：空心方框；还原：两个叠加方框（后框偏移 3,3 半透明 + 前框 0.5,0.5） -->
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

// 当前操作系统：darwin（macOS）/ windows / linux / 空字符串（未知或后端不可用）
// 用于窗口控制按钮平台适配：macOS 保留彩色圆点，其余平台使用原生扁平按钮
const platform = ref('')
const isMax = ref(false)  // 最大化状态：优先通过 Wails API 查询，回退本地翻转

// 初始化时检测操作系统，失败静默保持空字符串（与现有 getOS 可选链风格一致）
onMounted(async () => {
  try {
    const info = await getOS()
    platform.value = (info as { os?: string }).os ?? ''
  } catch {
    // 后端不可用（vite 单独运行）或解析失败：保持默认空字符串
  }
  // 原有静默检查更新逻辑保持不变
  if (shouldAutoCheck()) {
    checkForUpdate()
  }
})

function minimize() {
  w.runtime?.WindowMinimise()
}

// 最大化/还原：执行窗口切换；图标状态先本地翻转给即时反馈，
// 若运行时提供 WindowIsMaximised 查询 API 则延迟校正为真实状态
// （Toggle 生效是异步的，立即查询可能拿到旧值）。
function maximize() {
  w.runtime?.WindowToggleMaximise()
  isMax.value = !isMax.value
  if (w.runtime?.WindowIsMaximised) {
    setTimeout(async () => {
      try {
        isMax.value = await w.runtime.WindowIsMaximised()
      } catch {
        // 查询失败保持本地翻转值
      }
    }, 150)
  }
}

// 关闭按钮：仅当 Windows 且系统托盘已启用（设置页开关）时缩到托盘
// （llama-server 可后台继续运行）；否则直接退出。托盘启用状态优先读 store
// 缓存——main.ts 在 mount 前已 loadConfig，后端配置（含持久化 trayEnabled）
// 在用户点击关闭按钮时必然已加载完成，无需再调 getConfig。getOS 失败
// （如 vite 单独运行）时静默回退直接退出，与既有 w.runtime?. 可选链风格一致。
async function closeWindow() {
  let onWindows = false
  try {
    const info = await getOS()
    onWindows = info.os === 'windows'
  } catch {
    // 后端不可用（vite 单独运行）：保持默认行为
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

/* ─── 两套窗口控制按钮风格 ───
 * darwin（macOS）：.win-btn 彩色圆点（黄/绿/红），与系统视觉一致；
 * windows / linux / 未知：.window-controls.native 扁平无圆角按钮，
 *   高 36px 填满标题栏，hover 背景 var(--overlay-20)，关闭按钮 hover 红底白字。
 * 触发条件：platform === 'darwin' 时渲染圆点，否则渲染原生扁平组。
 * --wails-draggable: no-drag 保持：.window-controls 现有设置沿用到新按钮组。
 */
.window-controls {
  display: flex;
  gap: 8px;
  --wails-draggable: no-drag;
}

.window-controls.native {
  gap: 0;  /* 原生按钮紧贴，无间距 */
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

/* 关闭按钮 hover：红底白字 */
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

/* Route transitions — 纯透明度过渡，避免位移动画引发合成层与布局渲染切换时的亚像素水平跳变（修复聊天页离线提示居中文字抖动） */
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
