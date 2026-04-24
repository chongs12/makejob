import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import { resolve } from 'node:path'

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      url: resolve(__dirname, 'src/shims/browserUrl.ts'),
    },
  },
  server: {
    port: 3101,
    proxy: {
      '/api': {
        target: 'http://127.0.0.1:8080',
        changeOrigin: true,
        ws: true,
      },
      '/live2d-assets': {
        target: 'http://127.0.0.1:8080',
        changeOrigin: true,
      },
    },
  },
})
