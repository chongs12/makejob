<template>
  <div class="live2d-placeholder flex items-center justify-center bg-gradient-to-br from-blue-50 to-purple-50 rounded-xl border-2 border-dashed border-gray-300"
    :style="{ width: `${width}px`, height: `${height}px` }"
  >
    <div class="text-center">
      <div class="text-4xl mb-2">🤖</div>
      <p class="text-sm text-gray-500">Live2D 模型加载中</p>
      <p class="text-xs text-gray-400 mt-1">{{ message }}</p>
    </div>
  </div>
</template>

<script setup lang="ts">
withDefaults(defineProps<{
  width?: number
  height?: number
  message?: string
}>(), {
  width: 300,
  height: 400,
  message: '请稍候...'
})
</script>
