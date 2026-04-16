<script setup lang="ts">
/**
 * 全局加载组件
 * 页面加载状态展示
 */

interface Props {
  /** 加载状态 */
  loading?: boolean
  /** 加载文本 */
  text?: string
  /** 是否全屏覆盖 */
  fullscreen?: boolean
  /** 背景透明度 */
  opacity?: number
}

withDefaults(defineProps<Props>(), {
  loading: true,
  text: '加载中...',
  fullscreen: false,
  opacity: 0.8
})
</script>

<template>
  <div 
    v-if="loading"
    class="flex flex-col items-center justify-center"
    :class="fullscreen ? 'fixed inset-0 z-50 bg-white' : 'py-12'"
    :style="fullscreen ? { backgroundColor: `rgba(255, 255, 255, ${opacity})` } : {}"
  >
    <!-- 加载动画 -->
    <div class="relative">
      <!-- 外圈 -->
      <div class="w-12 h-12 border-4 border-primary-200 border-t-primary-600 rounded-full animate-spin" />
      <!-- 内圈装饰 -->
      <div class="absolute inset-0 flex items-center justify-center">
        <div class="w-4 h-4 bg-primary-600 rounded-full animate-pulse" />
      </div>
    </div>
    
    <!-- 加载文本 -->
    <p v-if="text" class="mt-4 text-sm text-secondary-500">
      {{ text }}
    </p>
  </div>
</template>

<style scoped>
@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}

.animate-spin {
  animation: spin 1s linear infinite;
}

@keyframes pulse {
  0%, 100% {
    opacity: 1;
    transform: scale(1);
  }
  50% {
    opacity: 0.5;
    transform: scale(0.8);
  }
}

.animate-pulse {
  animation: pulse 1.5s ease-in-out infinite;
}
</style>
