export default defineNuxtConfig({
  devtools: { enabled: true },
  modules: [
    '@pinia/nuxt',
    '@element-plus/nuxt',
    '@nuxtjs/tailwindcss',
  ],
  css: ['~/assets/css/main.css'],
  runtimeConfig: {
    public: {
      apiBase: 'http://localhost:8080/api',
    },
  },
  // 混合渲染策略
  routeRules: {
    '/': { prerender: true },          // SSG - 首页
    '/dashboard/**': { ssr: true },     // SSR - 仪表盘
    '/admin/**': { ssr: false },        // CSR - 管理后台
  },
  // Element Plus 自动导入
  elementPlus: {
    importStyle: 'scss',
  },
})
