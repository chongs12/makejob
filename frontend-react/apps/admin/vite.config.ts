import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  // 生产构建资源前缀：管理端部署在 nginx 的 /admin 路径下（K8s 与主页面同端口访问）
  base: '/admin/',
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
