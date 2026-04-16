<script setup lang="ts">
import {
  ArrowLeft,
  ArrowRight,
  Star,
  StarFilled,
  EditPen,
  SuccessFilled,
  CircleCloseFilled,
  Monitor,
} from '@element-plus/icons-vue'
import { normalizeQuestion, normalizeQuestionType, questionTypeLabel } from '~/utils/question'

definePageMeta({
  title: '答题',
  layout: 'default',
  middleware: ['auth'],
})

interface QuestionDetail {
  id: number
  title: string
  content: string
  difficulty: string
  type: string
  options?: string[]
  tags?: string[]
  correct_answer?: string
  analysis?: string
  is_favorite?: boolean
}

const route = useRoute()
const router = useRouter()
const appStore = useAppStore()
const api = useApi()

const questionId = computed(() => Number(route.params.id))

const loading = ref(true)
const submitting = ref(false)
const submitted = ref(false)
const isCorrect = ref(false)
const showNoteDialog = ref(false)
const question = ref<QuestionDetail>({
  id: 0,
  title: '',
  content: '',
  difficulty: '',
  type: '',
  options: [],
  tags: [],
})

const selectedAnswer = ref<number | null>(null)
const multipleAnswers = ref<number[]>([])
const subjectiveAnswer = ref('')
const timeSpent = ref(0)
let timer: ReturnType<typeof setInterval> | null = null

const submitResult = ref<{
  is_correct?: boolean
  correct_answer?: string
  analysis?: string
}>({})

const noteTitle = ref('')
const noteContent = ref('')
const noteSaving = ref(false)

const fetchQuestion = async () => {
  loading.value = true
  submitted.value = false
  selectedAnswer.value = null
  multipleAnswers.value = []
  subjectiveAnswer.value = ''
  submitResult.value = {}

  try {
    const res = await api.get<any>(`/questions/${questionId.value}`)
    question.value = normalizeQuestion(res.data || {}) as QuestionDetail
    appStore.setPageTitle(question.value.title || '答题')
  } catch {
    ElMessage.error('获取题目失败')
  } finally {
    loading.value = false
  }
}

const currentQuestionType = computed(() => normalizeQuestionType(question.value.type))

const handleSubmit = async () => {
  let answer = ''

  if (currentQuestionType.value === 'choice') {
    if (selectedAnswer.value === null) {
      ElMessage.warning('请选择一个答案')
      return
    }
    answer = String.fromCharCode(65 + selectedAnswer.value)
  } else if (currentQuestionType.value === 'multi') {
    if (!multipleAnswers.value.length) {
      ElMessage.warning('请至少选择一个选项')
      return
    }
    answer = multipleAnswers.value
      .sort((a, b) => a - b)
      .map(index => String.fromCharCode(65 + index))
      .join(',')
  } else if (currentQuestionType.value === 'subjective') {
    if (!subjectiveAnswer.value.trim()) {
      ElMessage.warning('请输入答案')
      return
    }
    answer = subjectiveAnswer.value
  } else {
    ElMessage.warning('请作答后提交')
    return
  }

  submitting.value = true
  try {
    const res = await api.post<any>(`/questions/${questionId.value}/submit`, {
      answer,
      time_spent: timeSpent.value,
    })

    submitResult.value = {
      is_correct: res.data?.is_correct,
      correct_answer: res.data?.correct_answer,
      analysis: res.data?.ai_analysis || res.data?.explanation,
    }
    isCorrect.value = !!res.data?.is_correct
    submitted.value = true
  } catch {
    ElMessage.error('提交失败，请重试')
  } finally {
    submitting.value = false
  }
}

const toggleFavorite = async () => {
  try {
    await api.post(`/questions/${questionId.value}/favorite`)
    question.value.is_favorite = !question.value.is_favorite
    ElMessage.success(question.value.is_favorite ? '已收藏' : '已取消收藏')
  } catch {
    ElMessage.error('操作失败')
  }
}

const openNoteDialog = () => {
  noteTitle.value = question.value.title || ''
  noteContent.value = ''
  showNoteDialog.value = true
}

const saveNote = async () => {
  if (!noteContent.value.trim()) {
    ElMessage.warning('请输入笔记内容')
    return
  }

  noteSaving.value = true
  try {
    await api.post('/user/notes', {
      question_id: questionId.value,
      title: noteTitle.value,
      content: noteContent.value,
    })
    ElMessage.success('笔记保存成功')
    showNoteDialog.value = false
  } catch {
    ElMessage.error('保存失败')
  } finally {
    noteSaving.value = false
  }
}

const goToQuestion = (offset: number) => {
  const nextId = questionId.value + offset
  if (nextId > 0) {
    router.push(`/practice/${nextId}`)
  }
}

const goToEditor = () => {
  router.push(`/practice/editor/${questionId.value}`)
}

const difficultyColor = (difficulty: string) => {
  const map: Record<string, string> = {
    easy: 'text-emerald-600 bg-emerald-50 border-emerald-200',
    medium: 'text-amber-600 bg-amber-50 border-amber-200',
    hard: 'text-rose-600 bg-rose-50 border-rose-200',
  }
  return map[difficulty] || 'text-secondary-600 bg-secondary-50 border-secondary-200'
}

const difficultyLabel = (difficulty: string) => {
  const map: Record<string, string> = { easy: '简单', medium: '中等', hard: '困难' }
  return map[difficulty] || difficulty || '未知'
}

const formatTime = (seconds: number) => {
  const minutes = Math.floor(seconds / 60)
  const remainSeconds = seconds % 60
  return `${minutes.toString().padStart(2, '0')}:${remainSeconds.toString().padStart(2, '0')}`
}

onMounted(() => {
  fetchQuestion()
  timer = setInterval(() => {
    if (!submitted.value) timeSpent.value += 1
  }, 1000)
})

onUnmounted(() => {
  if (timer) clearInterval(timer)
})

watch(() => route.params.id, () => {
  if (route.params.id) {
    timeSpent.value = 0
    fetchQuestion()
  }
})
</script>

<template>
  <div v-loading="loading" class="min-h-[calc(100vh-10rem)]">
    <div class="flex items-center justify-between mb-6">
      <div class="flex items-center gap-3">
        <el-button text :icon="ArrowLeft" @click="router.push('/practice')">返回列表</el-button>
        <el-divider direction="vertical" />
        <span class="text-sm text-secondary-500">用时 {{ formatTime(timeSpent) }}</span>
      </div>
      <div class="flex items-center gap-2">
        <el-button text @click="toggleFavorite">
          <el-icon :size="18" :class="question.is_favorite ? 'text-amber-400' : 'text-secondary-400'">
            <StarFilled v-if="question.is_favorite" />
            <Star v-else />
          </el-icon>
          <span class="ml-1 text-sm">{{ question.is_favorite ? '已收藏' : '收藏' }}</span>
        </el-button>
        <el-button text :icon="EditPen" @click="openNoteDialog">
          <span class="text-sm">笔记</span>
        </el-button>
      </div>
    </div>

    <div class="flex gap-6" v-if="!loading && question.id">
      <div class="w-[60%] flex-shrink-0">
        <div class="bg-white rounded-xl border border-secondary-200 p-6">
          <div class="flex items-center gap-3 mb-4">
            <h1 class="text-xl font-semibold text-secondary-900">{{ question.title }}</h1>
            <span :class="difficultyColor(question.difficulty)" class="inline-block px-2.5 py-0.5 rounded-full text-xs font-medium border">
              {{ difficultyLabel(question.difficulty) }}
            </span>
            <el-tag size="small" type="info" effect="plain">{{ questionTypeLabel(question.type) }}</el-tag>
          </div>

          <div class="prose prose-sm max-w-none text-secondary-700 leading-relaxed whitespace-pre-wrap mb-6">
            {{ question.content }}
          </div>

          <div v-if="question.tags?.length" class="flex flex-wrap gap-2 mb-4">
            <el-tag v-for="tag in question.tags" :key="tag" size="small" effect="plain" round>
              {{ tag }}
            </el-tag>
          </div>

          <div v-if="currentQuestionType === 'code'" class="mt-6 p-4 bg-secondary-50 rounded-lg border border-secondary-200">
            <div class="flex items-center gap-3">
              <el-icon :size="24" class="text-primary-500"><Monitor /></el-icon>
              <div class="flex-1">
                <div class="text-sm font-medium text-secondary-800">编程题建议使用代码编辑器</div>
                <div class="text-xs text-secondary-500">支持语法高亮与评估结果展示</div>
              </div>
              <el-button type="primary" @click="goToEditor">打开编辑器</el-button>
            </div>
          </div>
        </div>
      </div>

      <div class="flex-1 min-w-0">
        <div class="bg-white rounded-xl border border-secondary-200 p-6 sticky top-6">
          <h3 class="text-sm font-semibold text-secondary-800 mb-4">作答区</h3>

          <div v-if="currentQuestionType === 'choice' && question.options">
            <el-radio-group v-model="selectedAnswer" class="w-full flex flex-col gap-3" :disabled="submitted">
              <el-radio
                v-for="(opt, idx) in question.options"
                :key="idx"
                :value="idx"
                class="!h-auto !py-3 !px-4 !mr-0 rounded-lg border border-secondary-200 hover:border-primary-300 transition-colors"
                :class="{
                  '!border-primary-400 !bg-primary-50': selectedAnswer === idx && !submitted,
                  '!border-emerald-400 !bg-emerald-50': submitted && submitResult.correct_answer === String.fromCharCode(65 + idx),
                  '!border-rose-400 !bg-rose-50': submitted && selectedAnswer === idx && !isCorrect,
                }"
              >
                <span class="font-medium text-secondary-600 mr-2">{{ String.fromCharCode(65 + idx) }}.</span>
                <span class="text-sm text-secondary-800">{{ opt }}</span>
              </el-radio>
            </el-radio-group>
          </div>

          <div v-else-if="currentQuestionType === 'multi' && question.options">
            <el-checkbox-group v-model="multipleAnswers" class="w-full flex flex-col gap-3" :disabled="submitted">
              <el-checkbox
                v-for="(opt, idx) in question.options"
                :key="idx"
                :value="idx"
                class="!h-auto !py-3 !px-4 !mr-0 rounded-lg border border-secondary-200 hover:border-primary-300 transition-colors"
              >
                <span class="font-medium text-secondary-600 mr-2">{{ String.fromCharCode(65 + idx) }}.</span>
                <span class="text-sm text-secondary-800">{{ opt }}</span>
              </el-checkbox>
            </el-checkbox-group>
          </div>

          <div v-else-if="currentQuestionType === 'subjective'">
            <el-input v-model="subjectiveAnswer" type="textarea" :rows="8" placeholder="请输入你的答案..." :disabled="submitted" />
          </div>

          <div v-else-if="currentQuestionType === 'code'" class="text-center py-8">
            <el-icon :size="48" class="text-secondary-300 mb-3"><Monitor /></el-icon>
            <p class="text-sm text-secondary-500 mb-4">请使用代码编辑器作答</p>
            <el-button type="primary" @click="goToEditor">打开代码编辑器</el-button>
          </div>

          <div v-if="currentQuestionType !== 'code'" class="mt-6">
            <el-button v-if="!submitted" type="primary" size="large" class="w-full" :loading="submitting" @click="handleSubmit">
              提交答案
            </el-button>
          </div>

          <div v-if="submitted" class="mt-6">
            <div class="p-4 rounded-lg border text-center mb-4" :class="isCorrect ? 'bg-emerald-50 border-emerald-200' : 'bg-rose-50 border-rose-200'">
              <el-icon :size="48" :class="isCorrect ? 'text-emerald-500' : 'text-rose-500'">
                <SuccessFilled v-if="isCorrect" />
                <CircleCloseFilled v-else />
              </el-icon>
              <div class="mt-2 text-lg font-semibold" :class="isCorrect ? 'text-emerald-700' : 'text-rose-700'">
                {{ isCorrect ? '回答正确' : '回答错误' }}
              </div>
            </div>

            <div v-if="submitResult.correct_answer" class="mb-3">
              <div class="text-xs font-medium text-secondary-500 mb-1">正确答案</div>
              <div class="text-sm font-semibold text-emerald-600">{{ submitResult.correct_answer }}</div>
            </div>

            <el-collapse v-if="submitResult.analysis">
              <el-collapse-item title="详细解析">
                <div class="text-sm text-secondary-700 leading-relaxed whitespace-pre-wrap">
                  {{ submitResult.analysis }}
                </div>
              </el-collapse-item>
            </el-collapse>
          </div>
        </div>
      </div>
    </div>

    <div class="flex justify-between mt-6">
      <el-button :icon="ArrowLeft" @click="goToQuestion(-1)">上一题</el-button>
      <el-button type="primary" @click="goToQuestion(1)">
        下一题<el-icon class="ml-1"><ArrowRight /></el-icon>
      </el-button>
    </div>

    <el-dialog v-model="showNoteDialog" title="添加笔记" width="520px" destroy-on-close>
      <el-form label-position="top">
        <el-form-item label="标题">
          <el-input v-model="noteTitle" placeholder="笔记标题" />
        </el-form-item>
        <el-form-item label="内容">
          <el-input v-model="noteContent" type="textarea" :rows="6" placeholder="记录你的思考..." />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showNoteDialog = false">取消</el-button>
        <el-button type="primary" :loading="noteSaving" @click="saveNote">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>
