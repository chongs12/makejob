<template>
  <div class="live2d-interviewer">
    <Live2DCanvas
      :width="width"
      :height="height"
      :model-path="modelConfig?.path"
      :model-config="modelConfig"
      :loading="loading"
      :error="error"
    />
    <div v-if="speakingState" class="speaking-indicator mt-2 text-center">
      <el-tag type="primary" size="small" effect="light">
        <el-icon class="is-loading"><Loading /></el-icon> 面试官正在发问
      </el-tag>
    </div>
  </div>
</template>

<script setup lang="ts">
import { Loading } from '@element-plus/icons-vue'
import type { Live2DInterviewerProps } from './types'

const props = withDefaults(defineProps<Live2DInterviewerProps>(), {
  width: 300,
  height: 400,
  autoSpeak: true,
  loading: false,
  speaking: false,
  error: '',
})

const isSpeaking = ref(false)
const speakingState = computed(() => props.speaking || isSpeaking.value)

// speak 模拟当前阶段的发声状态，后续可替换为真实 TTS 与口型驱动。
const speak = (text: string) => {
  isSpeaking.value = true
  setTimeout(() => {
    isSpeaking.value = false
  }, text.length * 100)
}

defineExpose({ speak })
</script>
