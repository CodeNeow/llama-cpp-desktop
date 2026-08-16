import { createRouter, createWebHashHistory } from 'vue-router'
import Home from '../views/Home.vue'
import Chat from '../views/Chat.vue'
import Models from '../views/Models.vue'
import Api from '../views/Api.vue'
import Downloads from '../views/Downloads.vue'
import Settings from '../views/Settings.vue'
import ModelSettings from '../views/ModelSettings.vue'
import ModelDetail from '../views/ModelDetail.vue'

const routes = [
  {
    path: '/',
    name: 'Home',
    component: Home,
    meta: { title: '系统信息', icon: 'home' }
  },
  {
    path: '/chat',
    name: 'Chat',
    component: Chat,
    meta: { title: '本地聊天', icon: 'message-circle' }
  },
  {
    path: '/downloads',
    name: 'Downloads',
    component: Downloads,
    meta: { title: '模型下载', icon: 'download' }
  },
  {
    // 下载页子页面：搜索结果 → 模型详情（文件列表 + 描述），不添加到侧边栏导航。
    // modelId 可能含斜杠（如 org/name），用 encodeURIComponent 导航，此处自动解码。
    path: '/downloads/model/:modelId',
    name: 'ModelDetail',
    component: ModelDetail,
    meta: { title: '模型详情' }
  },
  {
    path: '/models',
    name: 'Models',
    component: Models,
    meta: { title: '模型管理', icon: 'cube' }
  },
  {
    // 模型设置子页面（独立路由页，非弹窗）：不添加到侧边栏导航。
    // modelName 可能含特殊字符，用 encodeURIComponent 编码，此处自动解码。
    path: '/models/settings/:modelName',
    name: 'ModelSettings',
    component: ModelSettings,
    meta: { title: '模型设置' }
  },
  {
    path: '/api',
    name: 'Api',
    component: Api,
    meta: { title: 'API 路由', icon: 'terminal' }
  },
  {
    path: '/monitor',
    // 监控已并入 API 页：保留旧链接兼容，重定向到 /api
    redirect: '/api'
  },
  {
    path: '/settings',
    name: 'Settings',
    component: Settings,
    meta: { title: '偏好设置', icon: 'settings' }
  }
]

const router = createRouter({
  history: createWebHashHistory(),
  routes
})

export default router
