<template>
  <div class="live2d-companion">
    <Live2DCanvas :width="width" :height="height" :model-path="modelConfig?.path" />
    <div class="mood-badge mt-2 text-center">
      <el-tag :type="moodType" size="small">{{ moodLabel }}</el-tag>
    </div>
  </div>
</template>

<script setup lang="ts">
import type { Live2DCompanionProps } from './types'

const props = withDefaults(defineProps<Live2DCompanionProps>(), {
  width: 200,
  height: 300,
  mood: 'neutral'
})

const moodMap: Record<string, { type: string; label: string }> = {
  happy: { type: 'success', label: '😊 开心' },
  neutral: { type: 'info', label: '😐 平静' },
  thinking: { type: 'warning', label: '🤔 思考中' },
  encouraging: { type: 'primary', label: '💪 加油' }
}

const moodType = computed(() => (moodMap[props.mood]?.type || 'info') as any)
const moodLabel = computed(() => moodMap[props.mood]?.label || '😐 平静')
</script>
