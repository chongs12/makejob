<script setup lang="ts">
/**
 * 顶部导航栏组件
 * Logo + 导航链接 + 用户区
 */

import { 
  HomeFilled, 
  EditPen, 
  VideoCamera, 
  Calendar,
  MagicStick,
  ArrowDown
} from '@element-plus/icons-vue'

const authStore = useAuthStore()
const appStore = useAppStore()
const router = useRouter()

// 主导航链接
const navLinks = [
  { path: '/', icon: HomeFilled, title: '首页' },
  { path: '/practice', icon: EditPen, title: '刷题' },
  { path: '/interview', icon: VideoCamera, title: '模拟面试' },
  { path: '/plan', icon: Calendar, title: '学习计划' },
  { path: '/companion', icon: MagicStick, title: '陪伴' },
]

// 用户下拉菜单
const userDropdownItems = [
  { path: '/dashboard', title: '个人中心', divided: false },
  { path: '/settings', title: '个人设置', divided: false },
  { path: '/membership', title: '会员中心', divided: false },
  { path: '', title: '退出登录', divided: true, action: 'logout' },
]

// 处理下拉菜单点击
const handleDropdownCommand = (command: string) => {
  if (command === 'logout') {
    authStore.logout()
  } else {
    router.push(command)
  }
}
</script>

<template>
  <header class="fixed top-0 left-0 right-0 h-16 bg-white border-b border-secondary-200 z-50">
    <div class="h-full container-custom flex items-center justify-between">
      <!-- Logo -->
      <NuxtLink to="/" class="flex items-center gap-2">
        <div class="w-9 h-9 bg-gradient-to-br from-primary-500 to-primary-700 rounded-lg flex items-center justify-center">
          <span class="text-white font-bold">M</span>
        </div>
        <span class="text-xl font-bold text-secondary-900">MakeJob</span>
      </NuxtLink>
      
      <!-- 导航链接 -->
      <nav class="hidden md:flex items-center gap-1">
        <NuxtLink
          v-for="link in navLinks"
          :key="link.path"
          :to="link.path"
          class="flex items-center gap-2 px-4 py-2 rounded-lg text-secondary-600 hover:bg-secondary-100 hover:text-secondary-900 transition-colors"
          :class="{ 'bg-primary-50 text-primary-600': $route.path === link.path }"
        >
          <el-icon :size="18">
            <component :is="link.icon" />
          </el-icon>
          <span class="text-sm font-medium">{{ link.title }}</span>
        </NuxtLink>
      </nav>
      
      <!-- 用户区 -->
      <div class="flex items-center gap-4">
        <!-- 未登录状态 -->
        <template v-if="!authStore.isLoggedIn">
          <NuxtLink 
            to="/auth/login"
            class="text-secondary-600 hover:text-secondary-900 text-sm font-medium"
          >
            登录
          </NuxtLink>
          <NuxtLink 
            to="/auth/register"
            class="px-4 py-2 bg-primary-600 text-white rounded-lg text-sm font-medium hover:bg-primary-700 transition-colors"
          >
            注册
          </NuxtLink>
        </template>
        
        <!-- 已登录状态 -->
        <template v-else>
          <el-dropdown @command="handleDropdownCommand" trigger="click">
            <div class="flex items-center gap-2 cursor-pointer hover:bg-secondary-100 rounded-lg px-2 py-1 transition-colors">
              <el-avatar :size="32" :src="authStore.avatar" />
              <span class="text-sm text-secondary-700 hidden sm:block">{{ authStore.username }}</span>
              <el-icon class="text-secondary-400">
                <ArrowDown />
              </el-icon>
            </div>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item 
                  v-for="item in userDropdownItems" 
                  :key="item.title"
                  :divided="item.divided"
                  :command="item.path || item.action"
                >
                  {{ item.title }}
                </el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </template>
      </div>
    </div>
  </header>
</template>
