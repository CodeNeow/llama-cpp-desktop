/// <reference types="vite/client" />

declare module '*.vue' {
  import type { DefineComponent } from 'vue'
  const component: DefineComponent<{}, {}, any>
  export default component
}

interface ElectronAPI {
  getVersion: () => Promise<string>
  platform: string
  minimizeWindow: () => void
  maximizeWindow: () => void
  closeWindow: () => void
  invokeBackend: (channel: string, ...args: unknown[]) => Promise<unknown>
  onBackendEvent: (callback: (event: string, data: unknown) => void) => void
}

declare global {
  interface Window {
    electronAPI?: ElectronAPI
  }
}
