import { createRouter, createWebHashHistory } from 'vue-router'
import Home from '../views/Home.vue'
import Models from '../views/Models.vue'
import Api from '../views/Api.vue'
import Downloads from '../views/Downloads.vue'
import Settings from '../views/Settings.vue'
import ModelDetail from '../views/ModelDetail.vue'

const routes = [
  {
    path: '/',
    name: 'Home',
    component: Home,
    meta: { title: '主页', icon: 'home' }
  },
  {
    path: '/downloads',
    name: 'Downloads',
    component: Downloads,
    meta: { title: '下载', icon: 'download' }
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
    meta: { title: '模型', icon: 'cube' }
  },
  {
    path: '/api',
    name: 'Api',
    component: Api,
    meta: { title: 'API', icon: 'terminal' }
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
    meta: { title: '设置', icon: 'settings' }
  }
]

const router = createRouter({
  history: createWebHashHistory(),
  routes
})

export default router
