import { createApp } from 'vue'
import App from './App.vue'
import router from './router'
import './styles/global.css'
import { loadConfig, appConfig } from './store'

// Apply cached theme immediately to avoid flash
document.documentElement.setAttribute('data-theme', appConfig.theme)

loadConfig().then(() => {
  const app = createApp(App)
  app.use(router)
  app.mount('#app')
})
