<script setup lang="ts">
/**
 * 侧边栏组件
 * 功能导航菜单，支持折叠
 */

import { 
  HomeFilled, 
  EditPen, 
  VideoCamera, 
  Calendar,
  MagicStick,
  UserFilled,
  Setting,
  Fold,
  Expand
} from '@element-plus/icons-vue'

const appStore = useAppStore()
const route = useRoute()

// 侧边栏菜单配置
const sidebarMenus = [
  {
    group: '主要功能',
    items: [
      { path: '/', icon: HomeFilled, title: '首页' },
      { path: '/practice', icon: EditPen, title: '刷题练习' },
      { path: '/interview', icon: VideoCamera, title: '模拟面试' },
      { path: '/plan', icon: Calendar, title: '学习计划' },
      { path: '/companion', icon: MagicStick, title: 'Live2D陪伴' },
    ]
  },
  {
    group: '个人中心',
    items: [
      { path: '/dashboard', icon: UserFilled, title: '我的仪表盘' },
      { path: '/membership', icon: MagicStick, title: '会员中心' },
      { path: '/settings', icon: Setting, title: '个人设置' },
    ]
  }
]

// 判断菜单是否激活
const isActive = (path: string) => {
  return route.path === path || route.path.startsWith(`${path}/`)
}
</script>

<template>
  <aside 
    class="fixed left-0 top-16 bottom-0 bg-white border-r border-secondary-200 transition-all duration-300 z-40"
    :class="appStore.sidebarCollapsed ? 'w-16' : 'w-64'"
  >
    <!-- 折叠按钮 -->
    <div 
      class="absolute -right-3 top-4 w-6 h-6 bg-white border border-secondary-200 rounded-full flex items-center justify-center cursor-pointer shadow-sm hover:shadow-md transition-shadow"
      @click="appStore.toggleSidebar"
    >
      <el-icon :size="12" class="text-secondary-500">
        <Fold v-if="!appStore.sidebarCollapsed" />
        <Expand v-else />
      </el-icon>
    </div>
    
    <!-- 菜单内容 -->
    <nav class="p-3 h-full overflow-y-auto">
      <div v-for="(menu, index) in sidebarMenus" :key="index" class="mb-6">
        <!-- 分组标题 -->
        <div 
          v-if="!appStore.sidebarCollapsed" 
          class="px-3 mb-2 text-xs font-semibold text-secondary-400 uppercase tracking-wider"
        >
          {{ menu.group }}
        </div>
        
        <!-- 菜单项 -->
        <ul class="space-y-1">
          <li v-for="item in menu.items" :key="item.path">
            <NuxtLink
              :to="item.path"
              class="flex items-center gap-3 px-3 py-2.5 rounded-lg transition-colors"
              :class="[
                isActive(item.path) 
                  ? 'bg-primary-50 text-primary-600' 
                  : 'text-secondary-600 hover:bg-secondary-100 hover:text-secondary-900',
                appStore.sidebarCollapsed ? 'justify-center' : ''
              ]"
              :title="appStore.sidebarCollapsed ? item.title : ''"
            >
              <el-icon :size="18">
                <component :is="item.icon" />
              </el-icon>
              <span 
                v-if="!appStore.sidebarCollapsed" 
                class="text-sm font-medium"
              >
                {{ item.title }}
              </span>
            </NuxtLink>
          </li>
        </ul>
      </div>
    </nav>
  </aside>
</template>
