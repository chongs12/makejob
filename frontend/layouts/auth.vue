<script setup lang="ts">
/**
 * 认证页面布局
 * 简洁居中布局，用于登录/注册页面
 */
</script>

<template>
  <div class="min-h-screen bg-gradient-to-br from-primary-50 via-white to-secondary-50 flex items-center justify-center p-4">
    <!-- 背景装饰 -->
    <div class="fixed inset-0 overflow-hidden pointer-events-none">
      <div class="absolute -top-40 -right-40 w-80 h-80 bg-primary-200 rounded-full mix-blend-multiply filter blur-3xl opacity-30 animate-pulse" />
      <div class="absolute -bottom-40 -left-40 w-80 h-80 bg-secondary-200 rounded-full mix-blend-multiply filter blur-3xl opacity-30 animate-pulse" />
    </div>
    
    <!-- 内容区域 -->
    <div class="relative z-10 w-full max-w-md">
      <!-- Logo和产品描述 -->
      <div class="text-center mb-8">
        <NuxtLink to="/" class="inline-flex items-center gap-3 mb-4">
          <div class="w-12 h-12 bg-gradient-to-br from-primary-500 to-primary-700 rounded-xl flex items-center justify-center shadow-lg">
            <span class="text-white font-bold text-xl">M</span>
          </div>
          <span class="text-2xl font-bold text-secondary-900">MakeJob</span>
        </NuxtLink>
        <p class="text-secondary-600">AI驱动的面试刷题与模拟面试平台</p>
      </div>
      
      <!-- 表单卡片 -->
      <div class="bg-white rounded-2xl shadow-xl p-8">
        <slot />
      </div>
      
      <!-- 底部链接 -->
      <div class="mt-8 text-center text-sm text-secondary-500">
        <p>&copy; 2024 MakeJob. All rights reserved.</p>
      </div>
    </div>
  </div>
</template>

<style scoped>
@keyframes pulse {
  0%, 100% {
    opacity: 0.3;
    transform: scale(1);
  }
  50% {
    opacity: 0.5;
    transform: scale(1.1);
  }
}

.animate-pulse {
  animation: pulse 4s ease-in-out infinite;
}
</style>
