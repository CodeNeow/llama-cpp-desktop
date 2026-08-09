import { reactive } from 'vue'
import { getConfig, setTheme as setThemeBackend } from './wails'

export const appConfig = reactive({
  theme: localStorage.getItem('llama-gui-theme') || 'dark',
  llamaCppDir: '',
  loaded: false,
})

export async function loadConfig() {
  try {
    const config = await getConfig()
    appConfig.theme = config.theme || 'dark'
    appConfig.llamaCppDir = config.llamaCppDir || ''
    localStorage.setItem('llama-gui-theme', appConfig.theme)
    document.documentElement.setAttribute('data-theme', appConfig.theme)
  } catch {} finally {
    appConfig.loaded = true
  }
}

export async function setTheme(theme: string) {
  appConfig.theme = theme
  document.documentElement.setAttribute('data-theme', theme)
  localStorage.setItem('llama-gui-theme', theme)
  try {
    await setThemeBackend(theme)
  } catch {}
}
