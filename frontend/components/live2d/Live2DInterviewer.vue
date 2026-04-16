<template>
  <div class="live2d-interviewer">
    <Live2DCanvas :width="width" :height="height" :model-path="modelConfig?.path" />
    <div v-if="isSpeaking" class="speaking-indicator mt-2 text-center">
      <el-tag type="primary" size="small" effect="light">
        <el-icon class="is-loading"><Loading /></el-icon> 面试官正在说话...
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
  autoSpeak: true
})

const isSpeaking = ref(false)

const speak = (text: string) => {
  isSpeaking.value = true
  // TTS + Live2D lip sync placeholder
  setTimeout(() => {
    isSpeaking.value = false
  }, text.length * 100)
}

defineExpose({ speak })
</script>
