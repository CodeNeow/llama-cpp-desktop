import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { resolve } from 'path'

// Functional config: every mode except 'mock' produces exactly the previous
// static configuration. Only `vite --mode mock` (npm run dev:mock) additionally
// aliases "@wailsio/runtime" to the in-repo fake runtime (src/dev/mockRuntime.ts)
// so the whole app runs against mocked backend data in a plain browser.
export default defineConfig(({ mode }) => ({
  plugins: [
    vue(),
  ],
  resolve: {
    alias: {
      '@': resolve(__dirname, 'src'),
      ...(mode === 'mock'
        ? { '@wailsio/runtime': resolve(__dirname, 'src/dev/mockRuntime.ts') }
        : {}),
    },
  },
  server: {
    port: 5173,
  },
  build: {
    outDir: 'dist',
  },
}))
