<script setup lang="ts">
/**
 * Live2D陪伴页 - 全屏布局
 * 左侧虚拟人展示 + 右侧聊天区域
 */

import { Promotion } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'

definePageMeta({
  layout: false,
  middleware: ['auth'],
})

const router = useRouter()
const { $api } = useNuxtApp()

const messages = ref<{ role: string; content: string }[]>([])
const inputText = ref('')
const sending = ref(false)
const mood = ref<'happy' | 'thinking' | 'encouraging' | 'neutral'>('happy')
const isSpeaking = ref(false)
const chatContainer = ref<HTMLElement | null>(null)
const eyesClosed = ref(false)

// 预设快捷回复
const quickReplies = [
  { text: '今天学什么', emoji: '📚' },
  { text: '鼓励一下我', emoji: '💪' },
  { text: '学习建议', emoji: '💡' },
]

// 随机眨眼
let blinkTimer: ReturnType<typeof setInterval> | null = null
const startBlinking = () => {
  blinkTimer = setInterval(() => {
    eyesClosed.value = true
    setTimeout(() => { eyesClosed.value = false }, 200)
  }, 3000 + Math.random() * 2000)
}

const scrollToBottom = () => {
  nextTick(() => {
    if (chatContainer.value) {
      chatContainer.value.scrollTop = chatContainer.value.scrollHeight
    }
  })
}

// 发送消息
const sendMessage = async (text?: string) => {
  const msg = text || inputText.value.trim()
  if (!msg || sending.value) return
  inputText.value = ''
  messages.value.push({ role: 'user', content: msg })
  scrollToBottom()

  sending.value = true
  mood.value = 'thinking'
  isSpeaking.value = false
  try {
    // 调用陪伴聊天API（使用通用聊天接口）
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
      isSpeaking.value = true
      setTimeout(() => { isSpeaking.value = false }, reply.length * 80)
    }
  } catch (e) {
    // 静默处理，如果API不存在，用本地回复
    const fallbackReplies = [
      '今天也要加油哦！坚持学习一定会有收获的 ✨',
      '你已经很棒了！每天进步一点点，积少成多 💪',
      '建议今天复习一下Go的并发编程，这是面试高频考点 📖',
      '休息一下也很重要哦，劳逸结合效率更高 ☕',
      '你今天的学习状态看起来很不错！继续保持 🎯',
    ]
    messages.value.push({
      role: 'assistant',
      content: fallbackReplies[Math.floor(Math.random() * fallbackReplies.length)]
    })
    mood.value = 'encouraging'
    isSpeaking.value = true
    setTimeout(() => { isSpeaking.value = false }, 2000)
  } finally {
    sending.value = false
  }
  scrollToBottom()
}

// 初始问候
onMounted(() => {
  startBlinking()
  const hour = new Date().getHours()
  let greeting = '你好呀！'
  if (hour < 9) greeting = '早上好！新的一天，元气满满！🌞'
  else if (hour < 12) greeting = '上午好！来一起学习吧！📚'
  else if (hour < 14) greeting = '中午好！吃完饭记得休息一下哦 ☕'
  else if (hour < 18) greeting = '下午好！继续加油，你很棒！💪'
  else greeting = '晚上好！今天辛苦了，还要学习吗？🌙'

  setTimeout(() => {
    messages.value.push({ role: 'assistant', content: greeting })
    isSpeaking.value = true
    setTimeout(() => { isSpeaking.value = false }, 2000)
  }, 500)
})

onUnmounted(() => {
  if (blinkTimer) clearInterval(blinkTimer)
})
</script>

<template>
  <div class="h-screen flex bg-gray-50">
    <!-- 左侧: 虚拟人展示区 -->
    <div class="w-1/2 bg-gradient-to-br from-indigo-100 via-purple-50 to-pink-100 relative overflow-hidden flex flex-col items-center justify-center">
      <!-- 装饰粒子 -->
      <div class="absolute inset-0 overflow-hidden pointer-events-none">
        <div v-for="i in 20" :key="i" class="absolute rounded-full opacity-20 animate-float"
          :class="i % 3 === 0 ? 'bg-indigo-300' : i % 3 === 1 ? 'bg-purple-300' : 'bg-pink-300'"
          :style="{
            width: `${6 + Math.random() * 12}px`,
            height: `${6 + Math.random() * 12}px`,
            left: `${Math.random() * 100}%`,
            top: `${Math.random() * 100}%`,
            animationDelay: `${Math.random() * 5}s`,
            animationDuration: `${4 + Math.random() * 4}s`,
          }" />
      </div>

      <!-- 返回按钮 -->
      <button @click="router.push('/dashboard')"
        class="absolute top-4 left-4 z-10 text-gray-500 hover:text-gray-700 bg-white/60 backdrop-blur rounded-full w-10 h-10 flex items-center justify-center transition-colors">
        ←
      </button>

      <!-- CSS角色 -->
      <div class="relative animate-companion-float">
        <!-- 主体 -->
        <div class="w-44 h-44 rounded-full bg-gradient-to-br from-indigo-400 via-purple-400 to-pink-400 shadow-2xl shadow-purple-200 flex items-center justify-center relative">
          <!-- 脸部 -->
          <div class="relative">
            <!-- 眼睛 -->
            <div class="flex gap-6 mb-3">
              <div class="relative">
                <div class="w-5 h-5 bg-white rounded-full flex items-center justify-center transition-all"
                  :class="eyesClosed ? '!h-1 !rounded-sm' : ''">
                  <div v-if="!eyesClosed" class="w-2.5 h-2.5 bg-gray-800 rounded-full" />
                </div>
              </div>
              <div class="relative">
                <div class="w-5 h-5 bg-white rounded-full flex items-center justify-center transition-all"
                  :class="eyesClosed ? '!h-1 !rounded-sm' : ''">
                  <div v-if="!eyesClosed" class="w-2.5 h-2.5 bg-gray-800 rounded-full" />
                </div>
              </div>
            </div>
            <!-- 腮红 -->
            <div class="flex gap-12 mb-1">
              <div class="w-4 h-2.5 bg-pink-300/60 rounded-full" />
              <div class="w-4 h-2.5 bg-pink-300/60 rounded-full" />
            </div>
            <!-- 嘴巴 -->
            <div class="flex justify-center">
              <div class="transition-all duration-200"
                :class="isSpeaking ? 'w-4 h-4 bg-pink-400 rounded-full animate-mouth' : 'w-6 h-3 border-b-2 border-pink-400 rounded-b-full'" />
            </div>
          </div>
        </div>
        <!-- 光晕 -->
        <div class="absolute -inset-4 rounded-full bg-gradient-to-br from-indigo-200/30 to-pink-200/30 blur-xl -z-10" />
      </div>

      <h3 class="mt-6 text-xl font-bold text-gray-800">小助手</h3>
      <el-tag :type="mood === 'happy' ? 'success' : mood === 'thinking' ? 'warning' : mood === 'encouraging' ? 'primary' : 'info'"
        class="mt-2" effect="light">
        {{ mood === 'happy' ? '😊 开心' : mood === 'thinking' ? '🤔 思考中' : mood === 'encouraging' ? '💪 加油' : '😐 平静' }}
      </el-tag>
    </div>

    <!-- 右侧: 聊天区域 -->
    <div class="w-1/2 flex flex-col">
      <!-- 顶部 -->
      <div class="h-14 border-b border-gray-200 bg-white flex items-center px-6">
        <span class="font-bold text-gray-900">AI 学习陪伴</span>
      </div>

      <!-- 消息列表 -->
      <div ref="chatContainer" class="flex-1 overflow-y-auto p-6 space-y-4">
        <div v-for="(msg, idx) in messages" :key="idx"
          :class="['flex', msg.role === 'user' ? 'justify-end' : 'justify-start']">
          <!-- AI消息 -->
          <div v-if="msg.role === 'assistant'" class="flex gap-3 max-w-[85%]">
            <div class="w-8 h-8 rounded-full bg-gradient-to-br from-indigo-400 to-purple-400 flex-shrink-0 flex items-center justify-center">
              <span class="text-white text-xs">AI</span>
            </div>
            <div class="bg-gradient-to-br from-purple-50 to-pink-50 border border-purple-100 text-gray-800 px-4 py-3 rounded-2xl rounded-bl-sm">
              <p class="text-sm leading-relaxed whitespace-pre-wrap">{{ msg.content }}</p>
            </div>
          </div>
          <!-- 用户消息 -->
          <div v-else class="max-w-[80%] bg-blue-500 text-white px-4 py-3 rounded-2xl rounded-br-sm">
            <p class="text-sm leading-relaxed">{{ msg.content }}</p>
          </div>
        </div>

        <!-- 思考中 -->
        <div v-if="sending" class="flex gap-3">
          <div class="w-8 h-8 rounded-full bg-gradient-to-br from-indigo-400 to-purple-400 flex-shrink-0 flex items-center justify-center">
            <span class="text-white text-xs">AI</span>
          </div>
          <div class="bg-purple-50 border border-purple-100 text-purple-400 px-4 py-3 rounded-2xl rounded-bl-sm">
            <span class="flex gap-1">
              <span class="w-2 h-2 bg-purple-300 rounded-full animate-bounce" style="animation-delay:0ms" />
              <span class="w-2 h-2 bg-purple-300 rounded-full animate-bounce" style="animation-delay:150ms" />
              <span class="w-2 h-2 bg-purple-300 rounded-full animate-bounce" style="animation-delay:300ms" />
            </span>
          </div>
        </div>
      </div>

      <!-- 快捷回复 -->
      <div class="px-6 pb-2 flex gap-2">
        <button v-for="q in quickReplies" :key="q.text"
          @click="sendMessage(q.text)"
          class="px-3 py-1.5 bg-white border border-gray-200 rounded-full text-sm text-gray-600 hover:bg-purple-50 hover:border-purple-200 hover:text-purple-600 transition-colors">
          {{ q.emoji }} {{ q.text }}
        </button>
      </div>

      <!-- 输入区 -->
      <div class="border-t border-gray-200 bg-white p-4">
        <div class="flex gap-3 items-end">
          <el-input
            v-model="inputText"
            type="textarea"
            :rows="2"
            placeholder="和小助手聊聊天..."
            :disabled="sending"
            @keydown.enter.ctrl="sendMessage()"
            resize="none"
            class="flex-1"
          />
          <el-button type="primary" circle :loading="sending" :disabled="!inputText.trim()"
            @click="sendMessage()" class="!w-10 !h-10">
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

@keyframes mouth {
  0%, 100% { transform: scaleY(0.5); }
  50% { transform: scaleY(1); }
}
.animate-mouth {
  animation: mouth 0.3s ease-in-out infinite;
}
</style>
