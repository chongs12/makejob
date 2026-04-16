<script setup lang="ts">
import { ArrowLeft, Clock, Microphone, Promotion } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'

definePageMeta({
  layout: false,
  middleware: ['auth'],
})

type InterviewMessage = {
  role: 'assistant' | 'user' | 'system'
  content: string
  type: 'question' | 'answer' | 'feedback' | 'system'
  score?: number
  feedback?: string
}

const route = useRoute()
const router = useRouter()
const { $api } = useNuxtApp()

const interviewId = computed(() => route.params.id as string)

const interview = ref<any>({})
const messages = ref<InterviewMessage[]>([])
const loading = ref(true)
const inputText = ref('')
const sending = ref(false)
const status = ref('ongoing')
const currentQuestion = ref(0)
const totalQuestions = ref(0)
const aiStatus = ref<'asking' | 'waiting' | 'scoring'>('waiting')

const startTime = ref(Date.now())
const elapsed = ref('00:00')
let timerInterval: ReturnType<typeof setInterval> | null = null

const chatContainer = ref<HTMLElement | null>(null)

const scrollToBottom = () => {
  nextTick(() => {
    if (chatContainer.value) {
      chatContainer.value.scrollTop = chatContainer.value.scrollHeight
    }
  })
}

const updateTimer = () => {
  const diff = Math.floor((Date.now() - startTime.value) / 1000)
  const minutes = Math.floor(diff / 60).toString().padStart(2, '0')
  const seconds = (diff % 60).toString().padStart(2, '0')
  elapsed.value = `${minutes}:${seconds}`
}

const appendQuestion = (question: any) => {
  if (!question?.question) return

  messages.value.push({
    role: 'assistant',
    content: question.question,
    type: 'question',
  })
  currentQuestion.value += 1
  scrollToBottom()
}

const mapMessage = (message: any): InterviewMessage => {
  if (message.message_type === 'feedback') {
    const scoreMatch = typeof message.content === 'string'
      ? message.content.match(/评分:\s*([0-9]+(?:\.[0-9]+)?)/)
      : null

    return {
      role: 'system',
      content: message.content || '',
      type: 'feedback',
      score: scoreMatch ? Number(scoreMatch[1]) : undefined,
      feedback: message.content || '',
    }
  }

  return {
    role: message.role === 'ai' ? 'assistant' : message.role === 'user' ? 'user' : 'system',
    content: message.content || '',
    type: message.role === 'user' ? 'answer' : 'question',
  }
}

const loadInterview = async () => {
  loading.value = true
  try {
    const res = await $api.get<any>(`/interviews/${interviewId.value}`)
    if (res.code === 200 && res.data) {
      interview.value = res.data
      status.value = res.data.status || 'ongoing'
      totalQuestions.value = res.data.total_questions || 0
      messages.value = (res.data.messages || []).map(mapMessage)
      currentQuestion.value = messages.value.filter(item => item.type === 'question').length

      const startedAt = res.data.started_at || res.data.created_at
      if (startedAt) {
        startTime.value = new Date(startedAt).getTime()
      }

      scrollToBottom()
    }
  } catch (e) {
    ElMessage.error('加载面试详情失败')
    console.error(e)
  } finally {
    loading.value = false
  }
}

const sendAnswer = async () => {
  if (!inputText.value.trim() || sending.value || status.value !== 'ongoing') return

  const answer = inputText.value.trim()
  inputText.value = ''

  messages.value.push({ role: 'user', content: answer, type: 'answer' })
  scrollToBottom()

  sending.value = true
  aiStatus.value = 'scoring'
  try {
    const res = await $api.post<any>(`/interviews/${interviewId.value}/answer`, { answer })
    if (res.code === 200 && res.data) {
      if (res.data.feedback) {
        messages.value.push({
          role: 'system',
          content: res.data.feedback.feedback || '',
          type: 'feedback',
          score: res.data.feedback.score,
          feedback: res.data.feedback.feedback,
        })
      }

      if (res.data.next_question) {
        aiStatus.value = 'asking'
        appendQuestion(res.data.next_question)
      }

      if (res.data.is_finished) {
        messages.value.push({
          role: 'system',
          content: '所有题目已答完，可以结束面试查看报告。',
          type: 'system',
        })
      }

      scrollToBottom()
    }
  } catch (e: any) {
    ElMessage.error(e?.data?.message || '提交回答失败')
  } finally {
    sending.value = false
    aiStatus.value = 'waiting'
  }
}

const endInterview = async () => {
  try {
    await ElMessageBox.confirm('确定要结束本次面试吗？', '结束面试', {
      type: 'warning',
    })
  } catch {
    return
  }

  try {
    const res = await $api.post<any>(`/interviews/${interviewId.value}/finish`)
    if (res.code === 200) {
      status.value = 'completed'
      ElMessage.success('面试已结束')
      if (timerInterval) clearInterval(timerInterval)
      router.push(`/interview/report/${interviewId.value}`)
    }
  } catch {
    ElMessage.error('结束面试失败')
  }
}

const goBack = () => {
  router.push('/interview')
}

const viewReport = () => {
  router.push(`/interview/report/${interviewId.value}`)
}

onMounted(() => {
  loadInterview()
  timerInterval = setInterval(updateTimer, 1000)
})

onUnmounted(() => {
  if (timerInterval) clearInterval(timerInterval)
})
</script>

<template>
  <div class="h-screen flex flex-col bg-gray-50">
    <header class="h-14 bg-white border-b border-gray-200 flex items-center px-4 gap-4 flex-shrink-0 z-10">
      <el-button text :icon="ArrowLeft" @click="goBack">返回</el-button>
      <div class="h-5 w-px bg-gray-200" />
      <span class="font-bold text-gray-900">模拟面试</span>
      <div class="flex-1" />
      <div class="flex items-center gap-4 text-sm text-gray-500">
        <span v-if="totalQuestions > 0" class="bg-blue-50 text-blue-600 px-3 py-1 rounded-full font-medium">
          第 {{ currentQuestion }} / {{ totalQuestions }} 题
        </span>
        <span class="flex items-center gap-1">
          <el-icon><Clock /></el-icon>
          {{ elapsed }}
        </span>
      </div>
      <el-button v-if="status === 'ongoing'" type="danger" size="small" @click="endInterview">
        结束面试
      </el-button>
      <el-button v-else type="primary" size="small" @click="viewReport">
        查看报告
      </el-button>
    </header>

    <div class="flex-1 flex overflow-hidden" v-loading="loading">
      <div class="w-[35%] border-r border-gray-200 bg-gradient-to-b from-blue-50 to-indigo-50 flex flex-col items-center justify-center p-8">
        <div class="relative">
          <div class="w-36 h-36 rounded-full bg-gradient-to-br from-blue-400 via-indigo-500 to-purple-600 flex items-center justify-center shadow-2xl shadow-indigo-200 animate-breathing">
            <svg class="w-20 h-20 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5"
                d="M9.75 3.104v5.714a2.25 2.25 0 01-.659 1.591L5 14.5M9.75 3.104c-.251.023-.501.05-.75.082m.75-.082a24.301 24.301 0 014.5 0m0 0v5.714a2.25 2.25 0 00.659 1.591L19 14.5M14.25 3.104c.251.023.501.05.75.082M19 14.5l-2.47 2.47a2.25 2.25 0 01-1.591.659H9.061a2.25 2.25 0 01-1.591-.659L5 14.5m14 0V7.5a2.25 2.25 0 00-2.25-2.25h-9.5A2.25 2.25 0 005 7.5v7" />
            </svg>
          </div>
          <div
            class="absolute -bottom-1 -right-1 w-8 h-8 rounded-full border-4 border-white"
            :class="aiStatus === 'waiting' ? 'bg-green-400' : aiStatus === 'asking' ? 'bg-blue-400 animate-pulse' : 'bg-amber-400 animate-pulse'"
          />
        </div>

        <h3 class="mt-6 text-xl font-bold text-gray-900">AI 面试官</h3>
        <p
          class="mt-2 text-sm px-4 py-1.5 rounded-full"
          :class="{
            'bg-green-100 text-green-700': aiStatus === 'waiting',
            'bg-blue-100 text-blue-700': aiStatus === 'asking',
            'bg-amber-100 text-amber-700': aiStatus === 'scoring',
          }"
        >
          {{ aiStatus === 'waiting' ? '等待回答...' : aiStatus === 'asking' ? '出题中...' : '评分中...' }}
        </p>

        <div class="mt-8 w-full max-w-xs space-y-3 bg-white/60 backdrop-blur rounded-xl p-4 text-sm text-gray-600">
          <div class="flex justify-between"><span>状态</span><el-tag size="small">{{ status }}</el-tag></div>
          <div class="flex justify-between"><span>题数</span><span class="font-medium text-gray-900">{{ totalQuestions }} 题</span></div>
          <div class="flex justify-between"><span>用时</span><span class="font-medium text-gray-900">{{ elapsed }}</span></div>
        </div>
      </div>

      <div class="flex-1 flex flex-col">
        <div ref="chatContainer" class="flex-1 overflow-y-auto p-6 space-y-4">
          <template v-for="(msg, idx) in messages" :key="idx">
            <div v-if="msg.role === 'assistant'" class="flex justify-start">
              <div class="max-w-[80%] bg-blue-50 border border-blue-100 text-gray-800 px-4 py-3 rounded-2xl rounded-bl-sm">
                <p class="whitespace-pre-wrap text-sm leading-relaxed">{{ msg.content }}</p>
              </div>
            </div>

            <div v-else-if="msg.role === 'user'" class="flex justify-end">
              <div class="max-w-[80%] bg-green-500 text-white px-4 py-3 rounded-2xl rounded-br-sm">
                <p class="whitespace-pre-wrap text-sm leading-relaxed">{{ msg.content }}</p>
              </div>
            </div>

            <div v-else-if="msg.type === 'feedback'" class="flex justify-center px-8">
              <div class="w-full max-w-lg bg-white border border-amber-200 rounded-xl p-4 shadow-sm">
                <div class="flex items-center gap-3 mb-2">
                  <span class="text-2xl font-bold" :class="(msg.score ?? 0) >= 80 ? 'text-green-500' : (msg.score ?? 0) >= 60 ? 'text-amber-500' : 'text-red-500'">
                    {{ msg.score ?? '-' }}分
                  </span>
                  <el-progress
                    :percentage="msg.score ?? 0"
                    :stroke-width="8"
                    :show-text="false"
                    :color="(msg.score ?? 0) >= 80 ? '#22c55e' : (msg.score ?? 0) >= 60 ? '#f59e0b' : '#ef4444'"
                    class="flex-1"
                  />
                </div>
                <p class="text-sm text-gray-600 whitespace-pre-wrap">{{ msg.feedback || msg.content }}</p>
              </div>
            </div>

            <div v-else class="flex justify-center">
              <span class="text-xs text-gray-400 bg-gray-100 px-3 py-1 rounded-full">{{ msg.content }}</span>
            </div>
          </template>

          <div v-if="sending" class="flex justify-start">
            <div class="bg-blue-50 border border-blue-100 text-blue-500 px-4 py-3 rounded-2xl rounded-bl-sm">
              <span class="flex items-center gap-2 text-sm">
                <span class="flex gap-1">
                  <span class="w-2 h-2 bg-blue-400 rounded-full animate-bounce" style="animation-delay: 0ms" />
                  <span class="w-2 h-2 bg-blue-400 rounded-full animate-bounce" style="animation-delay: 150ms" />
                  <span class="w-2 h-2 bg-blue-400 rounded-full animate-bounce" style="animation-delay: 300ms" />
                </span>
                AI 正在思考...
              </span>
            </div>
          </div>
        </div>

        <div class="border-t border-gray-200 bg-white p-4">
          <div class="flex gap-3 items-end">
            <el-input
              v-model="inputText"
              type="textarea"
              :rows="3"
              placeholder="输入你的回答..."
              :disabled="status !== 'ongoing' || sending"
              @keydown.enter.ctrl="sendAnswer"
              resize="vertical"
              class="flex-1"
            />
            <div class="flex flex-col gap-2">
              <el-button circle size="large" disabled class="!w-10 !h-10">
                <el-icon><Microphone /></el-icon>
              </el-button>
              <el-button type="primary" circle size="large" :loading="sending" :disabled="!inputText.trim() || status !== 'ongoing'" @click="sendAnswer" class="!w-10 !h-10">
                <el-icon><Promotion /></el-icon>
              </el-button>
            </div>
          </div>
          <p class="text-xs text-gray-400 mt-1">Ctrl + Enter 发送</p>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
@keyframes breathing {
  0%, 100% { transform: scale(1); }
  50% { transform: scale(1.05); }
}

.animate-breathing {
  animation: breathing 3s ease-in-out infinite;
}
</style>
