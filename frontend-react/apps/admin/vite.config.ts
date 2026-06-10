import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  server: {
    port: 3102,
    proxy: {
      '/api/v1': {
        target: 'http://127.0.0.1:8082',
        changeOrigin: true,
      },
      '/live2d-assets': {
        target: 'http://127.0.0.1:8082',
        changeOrigin: true,
      },
    },
  },
})
