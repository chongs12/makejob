<template>
  <div class="live2d-canvas-container" :style="{ width: `${width}px`, height: `${height}px` }">
    <canvas ref="canvasRef" :width="width" :height="height" />
    <div v-if="!isLoaded" class="absolute inset-0 flex items-center justify-center bg-gray-100 rounded-lg">
      <el-icon class="is-loading" :size="32"><Loading /></el-icon>
    </div>
  </div>
</template>

<script setup lang="ts">
import { Loading } from '@element-plus/icons-vue'

const props = withDefaults(defineProps<{
  width?: number
  height?: number
  modelPath?: string
}>(), {
  width: 300,
  height: 400,
  modelPath: ''
})

const canvasRef = ref<HTMLCanvasElement | null>(null)
const isLoaded = ref(false)

onMounted(() => {
  // Live2D SDK 加载逻辑（占位）
  setTimeout(() => {
    isLoaded.value = true
  }, 500)
})
</script>

<style scoped>
.live2d-canvas-container {
  position: relative;
  overflow: hidden;
  border-radius: 0.5rem;
}
</style>
