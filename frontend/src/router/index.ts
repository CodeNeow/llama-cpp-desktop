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
    meta: { title: 'home.title', icon: 'home', fixed: true },
    children: [
      // Entering / resolves the landing tab (see EnvironmentDefault). Named so
      // vue-router does not warn about the nameless empty-path child.
      { path: '', name: 'HomeDefault', component: EnvironmentDefault },
      {
        path: 'system',
        name: 'SystemInfo',
        component: SystemInfoTab,
        meta: { title: 'home.tabSystem' }
      },
      {
        path: 'runtime',
        name: 'Runtime',
        component: RuntimeSection,
        meta: { title: 'home.tabRuntime' }
      }
    ]
  },
  {
    path: '/chat',
    name: 'Chat',
    component: Chat,
    meta: { title: 'chat.title', icon: 'message-circle', fixed: true }
  },
  {
    // Models hub: one sidebar entry with an in-page tab bar owned by the shell.
    // The download tab is the default landing tab; children render through the
    // shell's <router-view> wrapped in <keep-alive> so tab switches keep the
    // search state and the local model list alive.
    path: '/models',
    name: 'Models',
    component: Models,
    meta: { title: 'models.title', icon: 'cube' },
    children: [
      // Entering /models lands on the download tab (search first). Named so
      // vue-router does not warn about the nameless empty-path child.
      { path: '', name: 'ModelsDefault', redirect: '/models/download' },
      {
        path: 'download',
        name: 'Downloads',
        component: Downloads,
        meta: { title: 'models.tabDownload' }
      },
      {
        path: 'local',
        name: 'ModelsLocal',
        component: ModelsLocal,
        meta: { title: 'models.tabLocal' }
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
    meta: { title: 'modelDetail.title' }
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
    meta: { title: 'modelSettings.title' }
  },
  {
    path: '/api',
    name: 'Api',
    component: Api,
    meta: { title: 'api.title', icon: 'terminal', fixed: true }
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
    meta: { title: 'settings.title', icon: 'settings' }
  },
  {
    // In-app documentation/tutorial: bundled bilingual markdown rendered with
    // the shared (HTML-disabled) markdown-it instance
    path: '/docs',
    name: 'Docs',
    component: Docs,
    meta: { title: 'docs.title', icon: 'book' }
  },
  {
    // Docs reader subpage (phone tier, design draft frame ⑱): one section per route,
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
