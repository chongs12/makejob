<script setup lang="ts">
/**
 * 默认布局
 * 顶部导航栏 + 左侧边栏 + 右侧内容区
 */

// 初始化应用状态
const appStore = useAppStore()
const authStore = useAuthStore()

onMounted(() => {
  appStore.initAppState()
  authStore.initAuth()
})
</script>

<template>
  <div class="min-h-screen bg-secondary-50">
    <!-- 顶部导航栏 -->
    <AppHeader />
    
    <div class="flex pt-16">
      <!-- 左侧边栏 -->
      <AppSidebar />
      
      <!-- 主内容区 -->
      <main 
        class="flex-1 transition-all duration-300"
        :class="{ 'ml-64': !appStore.sidebarCollapsed, 'ml-16': appStore.sidebarCollapsed }"
      >
        <div class="p-6 min-h-[calc(100vh-4rem-4rem)]">
          <slot />
        </div>
        
        <!-- 页脚 -->
        <AppFooter />
      </main>
    </div>
  </div>
</template>
