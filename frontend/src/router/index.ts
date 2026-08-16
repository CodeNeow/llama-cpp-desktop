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
    // Downloads subpage: search results -> model detail (file list + description); not added to sidebar navigation.
    // modelId may contain slashes (e.g. org/name); navigate with encodeURIComponent, decoded automatically here.
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
    // Model settings subpage (standalone route page, not a modal): not added to sidebar navigation.
    // modelName may contain special characters; encode it with encodeURIComponent, decoded automatically here.
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
    // Monitoring merged into the API page: keep old links working via redirect to /api
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
