<script setup lang="ts">
/**
 * Live2D 学习陪伴页
 * 左侧展示陪伴形象，右侧承载实时对话
 */

import { Promotion } from '@element-plus/icons-vue'

definePageMeta({
  layout: false,
  middleware: ['auth'],
})

type ChatMessage = {
  role: 'assistant' | 'user'
  content: string
}

const router = useRouter()
const { $api } = useNuxtApp()

const messages = ref<ChatMessage[]>([])
const inputText = ref('')
const sending = ref(false)
const mood = ref<'happy' | 'thinking' | 'encouraging' | 'neutral'>('happy')
const chatContainer = ref<HTMLElement | null>(null)
const live2DViewport = reactive({ width: 0, height: 0 })

const live2DScene = 'companion'
const { modelConfig, loading: live2DLoading, error: live2DError } = useLive2DModel(live2DScene)

const quickReplies = [
  { text: '今天适合学什么？', emoji: '🌤' },
  { text: '我有点没动力了', emoji: '💪' },
  { text: '帮我复盘一下今天', emoji: '📝' },
]

/* syncLive2DViewport 同步浏览器视口，用于按左半屏计算 Live2D 展示尺寸。 */
const syncLive2DViewport = () => {
  if (!process.client) {
    return
  }

  live2DViewport.width = window.innerWidth
  live2DViewport.height = window.innerHeight
}

/* companionCanvasSize 根据当前视口计算陪伴页的大尺寸画布。 */
const companionCanvasSize = computed(() => {
  const viewportWidth = live2DViewport.width || 1440
  const viewportHeight = live2DViewport.height || 900
  const width = Math.min(Math.max(viewportWidth * 0.5 - 72, 460), 720)
  const height = Math.min(Math.max(viewportHeight - 148, 640), 920)

  return {
    width: Math.round(width),
    height: Math.round(height),
  }
})

/* displayModelConfig 在陪伴页覆盖模型缩放和位置，让角色更贴近半屏展示。 */
const displayModelConfig = computed(() => {
  if (!modelConfig.value) {
    return null
  }

  const baseScale = Number(modelConfig.value.scale ?? 0.4)
  const basePosition = modelConfig.value.position || { x: 0, y: 0 }

  return {
    ...modelConfig.value,
    scale: Math.max(baseScale, 0.98),
    position: {
      x: Number(basePosition.x || 0),
      y: Number(basePosition.y || 0) + 0.08,
    },
  }
})

/* scrollToBottom 将对话滚动到底部。 */
const scrollToBottom = () => {
  nextTick(() => {
    if (chatContainer.value) {
      chatContainer.value.scrollTop = chatContainer.value.scrollHeight
    }
  })
}

/* buildGreeting 根据当前时间生成欢迎语。 */
const buildGreeting = () => {
  const hour = new Date().getHours()
  if (hour < 9) return '早上好，今天我们先从一个轻量目标开始。'
  if (hour < 12) return '上午好，状态不错的话，我们把最难的任务先拿下。'
  if (hour < 14) return '中午好，先别着急卷，吃好休息好也算进度。'
  if (hour < 18) return '下午好，现在适合集中做一轮高价值练习。'
  return '晚上好，适合复盘和收尾，我陪你把今天整理清楚。'
}

/* initGreeting 初始化首条陪伴消息。 */
const initGreeting = () => {
  const greeting = buildGreeting()
  setTimeout(() => {
    messages.value.push({ role: 'assistant', content: greeting })
  }, 500)
}

/* sendMessage 发送消息并处理陪伴回复。 */
const sendMessage = async (text?: string) => {
  const msg = text || inputText.value.trim()
  if (!msg || sending.value) return

  inputText.value = ''
  messages.value.push({ role: 'user', content: msg })
  scrollToBottom()

  sending.value = true
  mood.value = 'thinking'

  try {
    const res = await $api.post<any>('/companion/chat', {
      message: msg,
      messages: messages.value.map(item => ({
        role: item.role,
        content: item.content,
      })),
    })

    if (res.code === 200 && res.data) {
      const reply = res.data.reply || res.data.content || res.data.message || ''
      messages.value.push({ role: 'assistant', content: reply })
      mood.value = res.data.mood || 'happy'
    }
  } catch {
    const fallbackReplies = [
      '先别急，我们把目标拆小一点，今天只完成最关键的一步就够了。',
      '你已经在往前走了。先告诉我卡在哪，我帮你一起拆开。',
      '如果今天状态一般，那就做 20 分钟最重要的题，我陪你撑过去。',
      '先深呼吸一下，再把你脑子里最乱的那件事发给我。',
      '今天也可以是温和推进，不一定非要高强度冲刺。',
    ]

    messages.value.push({
      role: 'assistant',
      content: fallbackReplies[Math.floor(Math.random() * fallbackReplies.length)]!,
    })
    mood.value = 'encouraging'
  } finally {
    sending.value = false
  }

  scrollToBottom()
}

onMounted(() => {
  syncLive2DViewport()
  window.addEventListener('resize', syncLive2DViewport)
  initGreeting()
})

onUnmounted(() => {
  if (!process.client) {
    return
  }

  window.removeEventListener('resize', syncLive2DViewport)
})
</script>

<template>
  <div class="h-screen flex bg-gray-50">
    <div class="w-1/2 bg-gradient-to-br from-indigo-100 via-purple-50 to-pink-100 relative overflow-hidden flex flex-col items-center justify-center">
      <div class="absolute inset-0 overflow-hidden pointer-events-none">
        <div
          v-for="i in 20"
          :key="i"
          class="absolute rounded-full opacity-20 animate-float"
          :class="i % 3 === 0 ? 'bg-indigo-300' : i % 3 === 1 ? 'bg-purple-300' : 'bg-pink-300'"
          :style="{
            width: `${6 + Math.random() * 12}px`,
            height: `${6 + Math.random() * 12}px`,
            left: `${Math.random() * 100}%`,
            top: `${Math.random() * 100}%`,
            animationDelay: `${Math.random() * 5}s`,
            animationDuration: `${4 + Math.random() * 4}s`,
          }"
        />
      </div>

      <button
        @click="router.push('/dashboard')"
        class="absolute top-4 left-4 z-10 text-gray-500 hover:text-gray-700 bg-white/60 backdrop-blur rounded-full w-10 h-10 flex items-center justify-center transition-colors"
      >
        ←
      </button>

      <div class="relative animate-companion-float max-w-[calc(100%-2rem)]">
        <Live2DCompanion
          :width="companionCanvasSize.width"
          :height="companionCanvasSize.height"
          :model-config="displayModelConfig"
          :loading="live2DLoading"
          :error="live2DError"
          :mood="mood"
        />
      </div>

      <h3 class="mt-6 text-xl font-bold text-gray-800">学习陪伴中</h3>
      <el-tag
        :type="mood === 'happy' ? 'success' : mood === 'thinking' ? 'warning' : mood === 'encouraging' ? 'primary' : 'info'"
        class="mt-2"
        effect="light"
      >
        {{ mood === 'happy' ? '心情不错' : mood === 'thinking' ? '认真思考' : mood === 'encouraging' ? '正在鼓励你' : '平静陪伴' }}
      </el-tag>
      <p class="mt-3 text-sm text-gray-500 bg-white/60 backdrop-blur px-4 py-2 rounded-full">
        {{ modelConfig?.source === 'database' ? '已接入后台模型配置' : '当前使用内置默认模型' }}
      </p>
    </div>

    <div class="w-1/2 flex flex-col">
      <div class="h-14 border-b border-gray-200 bg-white flex items-center px-6">
        <span class="font-bold text-gray-900">AI 学习陪伴</span>
      </div>

      <div ref="chatContainer" class="flex-1 overflow-y-auto p-6 space-y-4">
        <div
          v-for="(msg, idx) in messages"
          :key="idx"
          :class="['flex', msg.role === 'user' ? 'justify-end' : 'justify-start']"
        >
          <div v-if="msg.role === 'assistant'" class="flex gap-3 max-w-[85%]">
            <div class="w-8 h-8 rounded-full bg-gradient-to-br from-indigo-400 to-purple-400 flex-shrink-0 flex items-center justify-center">
              <span class="text-white text-xs">AI</span>
            </div>
            <div class="bg-gradient-to-br from-purple-50 to-pink-50 border border-purple-100 text-gray-800 px-4 py-3 rounded-2xl rounded-bl-sm">
              <p class="text-sm leading-relaxed whitespace-pre-wrap">{{ msg.content }}</p>
            </div>
          </div>

          <div v-else class="max-w-[80%] bg-blue-500 text-white px-4 py-3 rounded-2xl rounded-br-sm">
            <p class="text-sm leading-relaxed">{{ msg.content }}</p>
          </div>
        </div>

        <div v-if="sending" class="flex gap-3">
          <div class="w-8 h-8 rounded-full bg-gradient-to-br from-indigo-400 to-purple-400 flex-shrink-0 flex items-center justify-center">
            <span class="text-white text-xs">AI</span>
          </div>
          <div class="bg-purple-50 border border-purple-100 text-purple-400 px-4 py-3 rounded-2xl rounded-bl-sm">
            <span class="flex gap-1">
              <span class="w-2 h-2 bg-purple-300 rounded-full animate-bounce" style="animation-delay: 0ms" />
              <span class="w-2 h-2 bg-purple-300 rounded-full animate-bounce" style="animation-delay: 150ms" />
              <span class="w-2 h-2 bg-purple-300 rounded-full animate-bounce" style="animation-delay: 300ms" />
            </span>
          </div>
        </div>
      </div>

      <div class="px-6 pb-2 flex gap-2">
        <button
          v-for="q in quickReplies"
          :key="q.text"
          @click="sendMessage(q.text)"
          class="px-3 py-1.5 bg-white border border-gray-200 rounded-full text-sm text-gray-600 hover:bg-purple-50 hover:border-purple-200 hover:text-purple-600 transition-colors"
        >
          {{ q.emoji }} {{ q.text }}
        </button>
      </div>

      <div class="border-t border-gray-200 bg-white p-4">
        <div class="flex gap-3 items-end">
          <el-input
            v-model="inputText"
            type="textarea"
            :rows="2"
            placeholder="把你现在的学习状态或问题发给我……"
            :disabled="sending"
            @keydown.enter.ctrl="sendMessage()"
            resize="none"
            class="flex-1"
          />
          <el-button
            type="primary"
            circle
            :loading="sending"
            :disabled="!inputText.trim()"
            @click="sendMessage()"
            class="!w-10 !h-10"
          >
            <el-icon><Promotion /></el-icon>
          </el-button>
        </div>
        <p class="text-xs text-gray-400 mt-1">Ctrl + Enter 发送</p>
      </div>
    </div>
  </div>
</template>

<style scoped>
@keyframes companionFloat {
  0%, 100% { transform: translateY(0); }
  50% { transform: translateY(-10px); }
}

.animate-companion-float {
  animation: companionFloat 3s ease-in-out infinite;
}

@keyframes float {
  0%, 100% { transform: translateY(0) scale(1); opacity: 0.2; }
  50% { transform: translateY(-20px) scale(1.1); opacity: 0.35; }
}

.animate-float {
  animation: float 5s ease-in-out infinite;
}
</style>
