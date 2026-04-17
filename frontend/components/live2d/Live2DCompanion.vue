<template>
  <div class="live2d-companion">
    <Live2DCanvas
      :width="width"
      :height="height"
      :model-path="modelConfig?.path"
      :model-config="modelConfig"
      :loading="loading"
      :error="error"
    />
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
  mood: 'neutral',
  loading: false,
  error: '',
})

const moodMap: Record<string, { type: string; label: string }> = {
  happy: { type: 'success', label: '心情不错' },
  neutral: { type: 'info', label: '平静陪伴' },
  thinking: { type: 'warning', label: '认真思考' },
  encouraging: { type: 'primary', label: '正在鼓励你' },
}

const moodType = computed(() => (moodMap[props.mood]?.type || 'info') as any)
const moodLabel = computed(() => moodMap[props.mood]?.label || '平静陪伴')
</script>
