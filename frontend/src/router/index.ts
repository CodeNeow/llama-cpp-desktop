import { createRouter, createWebHashHistory, type RouteLocationGeneric } from 'vue-router'

// Route meta typing: `fixed` marks fixed-viewport pages (Home / Chat / Api)
// whose root element fills the window below the titlebar and manages its own
// internal scroll bands. App.vue reads it to opt the shared content area out
// of its scroll behavior and TaskDock bottom reserve.
declare module 'vue-router' {
  interface RouteMeta {
    fixed?: boolean
  }
}
import Home from '../views/Home.vue'
import SystemInfoTab from '../views/SystemInfoTab.vue'
import EnvironmentDefault from '../views/EnvironmentDefault.vue'
import RuntimeSection from '../components/RuntimeSection.vue'
import Chat from '../views/Chat.vue'
import Models from '../views/Models.vue'
import ModelsLocal from '../views/ModelsLocal.vue'
import Api from '../views/Api.vue'
import Downloads from '../views/Downloads.vue'
import Settings from '../views/Settings.vue'
import Docs from '../views/Docs.vue'
import DocsReader from '../views/DocsReader.vue'
import ModelSettings from '../views/ModelSettings.vue'
import ModelDetail from '../views/ModelDetail.vue'

const routes = [
  {
    // System Environment hub: one sidebar entry with an in-page tab bar owned
    // by the shell (fixed-viewport page). Children render through the shell's
    // <router-view> wrapped in <keep-alive> so tab switches keep the hardware
    // samples and the runtime download state alive. The bare / path resolves a
    // smart default landing tab (EnvironmentDefault): Runtime when llama.cpp
    // is missing, System Info once installed.
    path: '/',
    name: 'Home',
    component: Home,
    meta: { title: '本机环境', icon: 'home', fixed: true },
    children: [
      // Entering / resolves the landing tab (see EnvironmentDefault). Named so
      // vue-router does not warn about the nameless empty-path child.
      { path: '', name: 'HomeDefault', component: EnvironmentDefault },
      {
        path: 'system',
        name: 'SystemInfo',
        component: SystemInfoTab,
        meta: { title: '系统信息' }
      },
      {
        path: 'runtime',
        name: 'Runtime',
        component: RuntimeSection,
        meta: { title: '运行环境' }
      }
    ]
  },
  {
    path: '/chat',
    name: 'Chat',
    component: Chat,
    meta: { title: '本地聊天', icon: 'message-circle', fixed: true }
  },
  {
    // Models hub: one sidebar entry with an in-page tab bar owned by the shell.
    // The download tab is the default landing tab; children render through the
    // shell's <router-view> wrapped in <keep-alive> so tab switches keep the
    // search state and the local model list alive.
    path: '/models',
    name: 'Models',
    component: Models,
    meta: { title: '模型管理', icon: 'cube' },
    children: [
      // Entering /models lands on the download tab (search first). Named so
      // vue-router does not warn about the nameless empty-path child.
      { path: '', name: 'ModelsDefault', redirect: '/models/download' },
      {
        path: 'download',
        name: 'Downloads',
        component: Downloads,
        meta: { title: '下载模型' }
      },
      {
        path: 'local',
        name: 'ModelsLocal',
        component: ModelsLocal,
        meta: { title: '我的模型' }
      }
    ]
  },
  {
    // Compat: /downloads merged into the /models shell's download tab
    path: '/downloads',
    redirect: '/models/download'
  },
  {
    // Model detail subpage: search results -> file list + description; not added to sidebar navigation.
    // modelId may contain slashes (e.g. org/name); navigate with encodeURIComponent, decoded automatically here.
    path: '/models/model/:modelId',
    name: 'ModelDetail',
    component: ModelDetail,
    meta: { title: '模型详情' }
  },
  {
    // Compat: old /downloads/model/... links redirect to the new path; a named
    // redirect keeps :modelId (string redirects drop params) so encoded
    // org/name modelIds survive
    path: '/downloads/model/:modelId',
    redirect: (to: RouteLocationGeneric) => ({ name: 'ModelDetail', params: { modelId: to.params.modelId } })
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
    meta: { title: 'API 路由', icon: 'terminal', fixed: true }
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
  },
  {
    // In-app documentation/tutorial: bundled bilingual markdown rendered with
    // the shared (HTML-disabled) markdown-it instance
    path: '/docs',
    name: 'Docs',
    component: Docs,
    meta: { title: '帮助与教程', icon: 'book' }
  },
  {
    // Docs reader subpage (phone tier, Aurora frame ⑱): one section per route,
    // reached from the phone section list. NOT in the sidebar / bottom nav —
    // /docs stays the only nav destination. On desktop tiers DocsReader
    // redirects back to the single-page /docs (the tier check lives in the
    // component: the route table cannot branch on viewport width). An unknown
    // section id is treated the same way.
    path: '/docs/:id',
    name: 'DocsReader',
    component: DocsReader,
    meta: { title: '帮助与教程' }
  }
]

const router = createRouter({
  history: createWebHashHistory(),
  routes
})

export default router
