<script setup lang="ts">
import {
  ArrowLeft,
  Star,
  StarFilled,
  EditPen,
  CaretRight,
  Upload,
  RefreshLeft,
  SuccessFilled,
  CircleCloseFilled,
  Loading as LoadingIcon,
} from '@element-plus/icons-vue'
import { normalizeQuestion } from '~/utils/question'

definePageMeta({
  layout: false,
  middleware: ['auth'],
})

interface QuestionDetail {
  id: number
  title: string
  content: string
  difficulty: string
  type: string
  tags?: string[]
  is_favorite?: boolean
  analysis?: string
}

type RunResult = {
  status: 'success' | 'error' | 'running' | ''
  output?: string
  error?: string
  is_correct?: boolean
}

const route = useRoute()
const router = useRouter()
const api = useApi()

const questionId = computed(() => Number(route.params.id))

const loading = ref(true)
const running = ref(false)
const showResult = ref(false)
const showNoteDialog = ref(false)
const question = ref<QuestionDetail>({
  id: 0,
  title: '',
  content: '',
  difficulty: '',
  type: '',
  tags: [],
})

const editorLanguage = ref('go')
const codeContent = ref('')
const defaultGoTemplate = `package main

import "fmt"

func solution() {
    // 在此编写你的代码
    fmt.Println("Hello, World!")
}

func main() {
    solution()
}
`

const runResult = ref<RunResult>({ status: '' })
const noteTitle = ref('')
const noteContent = ref('')
const noteSaving = ref(false)

const leftWidth = ref(50)
const bottomHeight = ref(220)

const editorContainer = ref<HTMLElement | null>(null)
let monacoRef: any = null
let editorInstance: any = null

const fetchQuestion = async () => {
  loading.value = true
  try {
    const res = await api.get<any>(`/questions/${questionId.value}`)
    question.value = normalizeQuestion(res.data || {}) as QuestionDetail
    if (!codeContent.value) {
      codeContent.value = defaultGoTemplate
    }
    if (editorInstance) {
      editorInstance.setValue(codeContent.value)
    }
  } catch {
    ElMessage.error('获取题目失败')
  } finally {
    loading.value = false
  }
}

const initEditor = async () => {
  if (!editorContainer.value) return

  try {
    const loader = await import('@monaco-editor/loader')
    monacoRef = await loader.default.init()

    editorInstance = monacoRef.editor.create(editorContainer.value, {
      value: codeContent.value || defaultGoTemplate,
      language: editorLanguage.value,
      theme: 'vs-dark',
      fontSize: 14,
      lineNumbers: 'on',
      minimap: { enabled: false },
      scrollBeyondLastLine: false,
      automaticLayout: true,
      tabSize: 4,
      insertSpaces: false,
      wordWrap: 'off',
      padding: { top: 12, bottom: 12 },
      renderLineHighlight: 'all',
      smoothScrolling: true,
      folding: true,
      bracketPairColorization: { enabled: true },
    })

    editorInstance.onDidChangeModelContent(() => {
      codeContent.value = editorInstance.getValue()
    })
  } catch (e) {
    console.error('Monaco Editor 加载失败', e)
  }
}

const applyEvaluationResult = (data: any) => {
  const analysis = data?.ai_analysis || data?.explanation || '评估完成'
  runResult.value = {
    status: data?.is_correct ? 'success' : 'error',
    output: data?.is_correct ? analysis : '',
    error: data?.is_correct ? '' : analysis,
    is_correct: data?.is_correct,
  }
}

const evaluateCode = async () => {
  if (!codeContent.value.trim()) {
    ElMessage.warning('请编写代码后再提交')
    return
  }

  running.value = true
  showResult.value = true
  runResult.value = { status: 'running' }

  try {
    const res = await api.post<any>(`/questions/${questionId.value}/submit`, {
      answer: codeContent.value,
      time_spent: 0,
    })
    applyEvaluationResult(res.data)
  } catch (e: any) {
    runResult.value = {
      status: 'error',
      error: e?.data?.message || '评估失败，请重试',
    }
  } finally {
    running.value = false
  }
}

const handleRun = async () => {
  await evaluateCode()
}

const handleSubmit = async () => {
  await evaluateCode()
  if (runResult.value.is_correct) {
    ElMessage.success('恭喜，通过评估')
  }
}

const handleReset = () => {
  codeContent.value = defaultGoTemplate
  if (editorInstance) {
    editorInstance.setValue(defaultGoTemplate)
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

const startDragH = (event: MouseEvent) => {
  const startX = event.clientX
  const startWidth = leftWidth.value

  const onMove = (moveEvent: MouseEvent) => {
    const delta = moveEvent.clientX - startX
    const nextWidth = startWidth + (delta / window.innerWidth) * 100
    leftWidth.value = Math.max(25, Math.min(75, nextWidth))
  }

  const onUp = () => {
    document.removeEventListener('mousemove', onMove)
    document.removeEventListener('mouseup', onUp)
  }

  document.addEventListener('mousemove', onMove)
  document.addEventListener('mouseup', onUp)
}

const startDragV = (event: MouseEvent) => {
  const startY = event.clientY
  const startHeight = bottomHeight.value

  const onMove = (moveEvent: MouseEvent) => {
    const delta = startY - moveEvent.clientY
    bottomHeight.value = Math.max(100, Math.min(420, startHeight + delta))
  }

  const onUp = () => {
    document.removeEventListener('mousemove', onMove)
    document.removeEventListener('mouseup', onUp)
  }

  document.addEventListener('mousemove', onMove)
  document.addEventListener('mouseup', onUp)
}

const difficultyColor = (difficulty: string) => {
  const map: Record<string, string> = {
    easy: 'text-emerald-500',
    medium: 'text-amber-500',
    hard: 'text-rose-500',
  }
  return map[difficulty] || 'text-secondary-500'
}

const difficultyLabel = (difficulty: string) => {
  const map: Record<string, string> = { easy: '简单', medium: '中等', hard: '困难' }
  return map[difficulty] || difficulty || '未知'
}

const languageOptions = [
  { label: 'Go', value: 'go' },
  { label: 'Python', value: 'python' },
  { label: 'JavaScript', value: 'javascript' },
  { label: 'Java', value: 'java' },
  { label: 'C++', value: 'cpp' },
]

const handleLanguageChange = (language: string) => {
  if (editorInstance?.getModel() && monacoRef) {
    monacoRef.editor.setModelLanguage(editorInstance.getModel(), language)
  }
}

onMounted(async () => {
  await fetchQuestion()
  await nextTick()
  await initEditor()
})

onUnmounted(() => {
  if (editorInstance) {
    editorInstance.dispose()
  }
})
</script>

<template>
  <div class="h-screen flex flex-col bg-[#1e1e2e] text-white overflow-hidden select-none">
    <header class="h-12 flex-shrink-0 bg-[#181825] border-b border-[#313244] flex items-center justify-between px-4">
      <div class="flex items-center gap-3">
        <el-button text size="small" class="!text-secondary-400 hover:!text-white" @click="router.push('/practice')">
          <el-icon class="mr-1"><ArrowLeft /></el-icon>
          <span class="text-xs">返回列表</span>
        </el-button>
        <el-divider direction="vertical" class="!border-[#313244]" />
        <span class="text-sm font-medium text-secondary-300 truncate max-w-md">
          {{ question.title || '加载中...' }}
        </span>
        <span v-if="question.difficulty" :class="difficultyColor(question.difficulty)" class="text-xs font-medium">
          {{ difficultyLabel(question.difficulty) }}
        </span>
      </div>
      <div class="flex items-center gap-2">
        <el-button text size="small" class="!text-secondary-400 hover:!text-amber-400" @click="toggleFavorite">
          <el-icon :size="16">
            <StarFilled v-if="question.is_favorite" class="text-amber-400" />
            <Star v-else />
          </el-icon>
        </el-button>
        <el-button text size="small" class="!text-secondary-400 hover:!text-white" @click="openNoteDialog">
          <el-icon :size="16"><EditPen /></el-icon>
        </el-button>
      </div>
    </header>

    <div class="flex-1 flex overflow-hidden" :style="{ height: `calc(100vh - 3rem - ${showResult ? bottomHeight : 0}px)` }">
      <div class="overflow-y-auto bg-[#1e1e2e]" :style="{ width: `${leftWidth}%` }">
        <div class="p-6" v-loading="loading" element-loading-background="rgba(30,30,46,0.8)">
          <div class="flex items-center gap-3 mb-4">
            <h1 class="text-lg font-bold text-white">{{ question.title }}</h1>
          </div>

          <div class="flex flex-wrap gap-2 mb-6">
            <span :class="difficultyColor(question.difficulty)" class="px-2.5 py-0.5 rounded text-xs font-medium bg-white/5">
              {{ difficultyLabel(question.difficulty) }}
            </span>
            <span v-for="tag in (question.tags || [])" :key="tag" class="px-2.5 py-0.5 rounded text-xs font-medium text-secondary-400 bg-white/5">
              {{ tag }}
            </span>
          </div>

          <div class="text-sm text-secondary-300 leading-relaxed whitespace-pre-wrap mb-6">
            {{ question.content }}
          </div>
        </div>
      </div>

      <div class="w-1.5 flex-shrink-0 bg-[#313244] hover:bg-primary-500/50 cursor-col-resize transition-colors relative group" @mousedown="startDragH">
        <div class="absolute inset-y-0 -left-1 -right-1"></div>
      </div>

      <div class="flex-1 flex flex-col overflow-hidden bg-[#1e1e1e]">
        <div class="h-10 flex-shrink-0 bg-[#252526] border-b border-[#3c3c3c] flex items-center justify-between px-3">
          <el-select v-model="editorLanguage" size="small" style="width: 120px" @change="handleLanguageChange" class="editor-lang-select">
            <el-option v-for="lang in languageOptions" :key="lang.value" :label="lang.label" :value="lang.value" />
          </el-select>
          <el-button size="small" text class="!text-secondary-400 hover:!text-white" @click="handleReset">
            <el-icon class="mr-1"><RefreshLeft /></el-icon>
            <span class="text-xs">重置</span>
          </el-button>
        </div>

        <div ref="editorContainer" class="flex-1 min-h-0"></div>

        <div class="h-12 flex-shrink-0 bg-[#252526] border-t border-[#3c3c3c] flex items-center justify-end px-4 gap-3">
          <el-button size="default" @click="handleRun" :loading="running" class="!bg-[#3c3c3c] !text-white !border-[#3c3c3c] hover:!bg-[#4c4c4c]">
            <el-icon class="mr-1" v-if="!running"><CaretRight /></el-icon>
            运行
          </el-button>
          <el-button size="default" type="success" @click="handleSubmit" :loading="running">
            <el-icon class="mr-1" v-if="!running"><Upload /></el-icon>
            提交
          </el-button>
        </div>
      </div>
    </div>

    <div v-if="showResult" class="h-1.5 flex-shrink-0 bg-[#313244] hover:bg-primary-500/50 cursor-row-resize transition-colors" @mousedown="startDragV"></div>

    <div v-if="showResult" class="flex-shrink-0 bg-[#181825] border-t border-[#313244] overflow-hidden" :style="{ height: `${bottomHeight}px` }">
      <div class="flex items-center justify-between px-4 h-9 border-b border-[#313244]">
        <div class="flex items-center gap-2">
          <span class="text-xs font-semibold text-secondary-400">评估结果</span>
          <span v-if="runResult.status === 'success'" class="text-xs font-medium text-emerald-400">通过</span>
          <span v-else-if="runResult.status === 'error'" class="text-xs font-medium text-rose-400">待改进</span>
        </div>
        <el-button text size="small" class="!text-secondary-500 hover:!text-white !text-xs" @click="showResult = false">
          关闭
        </el-button>
      </div>
      <div class="p-4 overflow-y-auto" :style="{ height: `${bottomHeight - 36}px` }">
        <div v-if="runResult.status === 'running'" class="flex items-center gap-3 text-secondary-400">
          <el-icon class="is-loading" :size="18"><LoadingIcon /></el-icon>
          <span class="text-sm">正在评估...</span>
        </div>

        <div v-else-if="runResult.status === 'success'">
          <div class="flex items-center gap-2 mb-3">
            <el-icon :size="22" class="text-emerald-400"><SuccessFilled /></el-icon>
            <span class="text-base font-semibold text-emerald-400">通过</span>
          </div>
          <div class="bg-[#1e1e2e] rounded p-3">
            <pre class="text-sm text-secondary-300 font-mono whitespace-pre-wrap">{{ runResult.output }}</pre>
          </div>
        </div>

        <div v-else-if="runResult.status === 'error'">
          <div class="flex items-center gap-2 mb-3">
            <el-icon :size="22" class="text-rose-400"><CircleCloseFilled /></el-icon>
            <span class="text-base font-semibold text-rose-400">待改进</span>
          </div>
          <div class="bg-[#1e1e2e] rounded p-3">
            <pre class="text-sm text-rose-300 whitespace-pre-wrap">{{ runResult.error }}</pre>
          </div>
        </div>
      </div>
    </div>

    <el-dialog v-model="showNoteDialog" title="添加笔记" width="520px" destroy-on-close class="editor-note-dialog">
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

<style scoped>
.editor-lang-select :deep(.el-input__wrapper) {
  background-color: #3c3c3c;
  border-color: #3c3c3c;
  box-shadow: none;
}

.editor-lang-select :deep(.el-input__inner) {
  color: #ccc;
  font-size: 12px;
}
</style>
