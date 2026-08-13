import { createRouter, createWebHashHistory } from 'vue-router'
import Home from '../views/Home.vue'
import Models from '../views/Models.vue'
import Api from '../views/Api.vue'
import Downloads from '../views/Downloads.vue'
import Monitor from '../views/Monitor.vue'
import Settings from '../views/Settings.vue'

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
    name: 'Monitor',
    component: Monitor,
    meta: { title: '监控', icon: 'chart' }
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
