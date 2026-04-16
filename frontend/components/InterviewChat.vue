<template>
  <div class="interview-chat flex flex-col h-full">
    <div ref="messagesContainer" class="flex-1 overflow-y-auto p-4 space-y-3">
      <div
        v-for="(msg, idx) in messages"
        :key="idx"
        :class="['flex', msg.role === 'user' ? 'justify-end' : 'justify-start']"
      >
        <div
          :class="[
            'max-w-[75%] px-4 py-2 rounded-2xl',
            msg.role === 'user'
              ? 'bg-blue-500 text-white rounded-br-sm'
              : 'bg-gray-100 text-gray-800 rounded-bl-sm'
          ]"
        >
          <p class="whitespace-pre-wrap text-sm">{{ msg.content }}</p>
          <span class="text-xs opacity-60 mt-1 block">{{ msg.time || '' }}</span>
        </div>
      </div>
      <div v-if="loading" class="flex justify-start">
        <div class="bg-gray-100 text-gray-500 px-4 py-2 rounded-2xl rounded-bl-sm">
          <span class="animate-pulse">正在思考...</span>
        </div>
      </div>
    </div>

    <div class="border-t p-3 flex gap-2">
      <el-input
        v-model="inputText"
        :placeholder="placeholder"
        :disabled="disabled"
        @keyup.enter="handleSend"
      />
      <el-button type="primary" @click="handleSend" :disabled="disabled || !inputText.trim()">
        发送
      </el-button>
    </div>
  </div>
</template>

<script setup lang="ts">
const props = withDefaults(defineProps<{
  messages: Array<{ role: string; content: string; time?: string }>
  loading?: boolean
  disabled?: boolean
  placeholder?: string
}>(), {
  loading: false,
  disabled: false,
  placeholder: '输入消息...'
})

const emit = defineEmits<{
  send: [text: string]
}>()

const inputText = ref('')
const messagesContainer = ref<HTMLElement | null>(null)

const handleSend = () => {
  if (!inputText.value.trim() || props.disabled) return
  emit('send', inputText.value.trim())
  inputText.value = ''
}

watch(() => props.messages.length, () => {
  nextTick(() => {
    if (messagesContainer.value) {
      messagesContainer.value.scrollTop = messagesContainer.value.scrollHeight
    }
  })
})
</script>
