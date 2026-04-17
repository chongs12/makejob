<script setup lang="ts">
/**
 * 管理后台布局
 * 左侧深色管理导航 + 右侧内容区
 */

import { 
  HomeFilled, 
  UserFilled, 
  QuestionFilled, 
  OfficeBuilding,
  Setting,
  Document,
  Monitor,
  MagicStick,
  Microphone,
  ShoppingCart,
  Download
} from '@element-plus/icons-vue'

// 初始化
const appStore = useAppStore()
const authStore = useAuthStore()
const route = useRoute()

onMounted(() => {
  appStore.initAppState()
  authStore.initAuth()
})

// 管理菜单配置
const adminMenus = [
  { path: '/admin', icon: HomeFilled, title: '仪表盘' },
  { path: '/admin/users', icon: UserFilled, title: '用户管理' },
  { path: '/admin/questions', icon: QuestionFilled, title: '题库管理' },
  { path: '/admin/industries', icon: OfficeBuilding, title: '行业管理' },
  { path: '/admin/scraper', icon: Download, title: '面经采集' },
  { path: '/admin/ai-config', icon: Setting, title: 'AI配置' },
  { path: '/admin/ai-debug', icon: Monitor, title: 'AI调试' },
  { path: '/admin/prompts', icon: Document, title: 'Prompt模板' },
  { path: '/admin/live2d', icon: MagicStick, title: 'Live2D管理' },
  { path: '/admin/tts', icon: Microphone, title: 'TTS管理' },
  { path: '/admin/orders', icon: ShoppingCart, title: '订单管理' },
]

// 判断菜单是否激活
const isActive = (path: string) => {
  return route.path === path || route.path.startsWith(`${path}/`)
}
</script>

<template>
  <div class="min-h-screen bg-secondary-50">
    <!-- 顶部栏 -->
    <header class="fixed top-0 left-0 right-0 h-16 bg-secondary-900 text-white z-50 flex items-center justify-between px-6">
      <div class="flex items-center gap-3">
        <div class="w-8 h-8 bg-primary-500 rounded-lg flex items-center justify-center">
          <span class="font-bold text-sm">M</span>
        </div>
        <span class="font-semibold">MakeJob 管理后台</span>
      </div>
      
      <div class="flex items-center gap-4">
        <NuxtLink to="/" class="text-secondary-400 hover:text-white transition-colors text-sm">
          返回前台
        </NuxtLink>
        <el-dropdown>
          <div class="flex items-center gap-2 cursor-pointer">
            <el-avatar :size="32" :src="authStore.avatar" />
            <span class="text-sm">{{ authStore.username || '管理员' }}</span>
          </div>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item @click="authStore.logout()">退出登录</el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
      </div>
    </header>
    
    <div class="flex pt-16">
      <!-- 左侧管理导航 -->
      <aside class="fixed left-0 top-16 bottom-0 w-64 bg-secondary-800 text-white overflow-y-auto z-40">
        <nav class="p-4">
          <ul class="space-y-1">
            <li v-for="menu in adminMenus" :key="menu.path">
              <NuxtLink
                :to="menu.path"
                class="flex items-center gap-3 px-4 py-3 rounded-lg transition-colors"
                :class="isActive(menu.path) 
                  ? 'bg-primary-600 text-white' 
                  : 'text-secondary-300 hover:bg-secondary-700 hover:text-white'"
              >
                <el-icon :size="18">
                  <component :is="menu.icon" />
                </el-icon>
                <span>{{ menu.title }}</span>
              </NuxtLink>
            </li>
          </ul>
        </nav>
      </aside>
      
      <!-- 主内容区 -->
      <main class="flex-1 ml-64 p-6 min-h-[calc(100vh-4rem)]">
        <slot />
      </main>
    </div>
  </div>
</template>
