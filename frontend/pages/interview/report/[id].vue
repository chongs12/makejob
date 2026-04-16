<script setup lang="ts">
import { Check, Close, ArrowLeft, RefreshRight } from '@element-plus/icons-vue'

definePageMeta({
  layout: 'default',
  middleware: ['auth'],
})

type InterviewReport = {
  overall_score: number
  total_questions: number
  correct_count: number
  dimension_scores?: Record<string, number>
  strengths?: string[]
  weaknesses?: string[]
  suggestions?: string[]
  summary?: string
}

type ReportResponse = {
  report: InterviewReport
  duration_seconds: number
  completed_at: string
}

const route = useRoute()
const router = useRouter()
const { $api } = useNuxtApp()
const appStore = useAppStore()

const loading = ref(true)
const reportResponse = ref<ReportResponse | null>(null)

const interviewId = computed(() => route.params.id as string)
const report = computed(() => reportResponse.value?.report)

const scoreColor = computed(() => {
  const score = report.value?.overall_score || 0
  if (score >= 80) return '#22c55e'
  if (score >= 60) return '#f59e0b'
  return '#ef4444'
})

const scoreLevel = computed(() => {
  const score = report.value?.overall_score || 0
  if (score >= 90) return '优秀'
  if (score >= 80) return '良好'
  if (score >= 60) return '一般'
  return '需加强'
})

const scoreLevelClass = computed(() => {
  const score = report.value?.overall_score || 0
  if (score >= 80) return 'text-green-600 bg-green-50'
  if (score >= 60) return 'text-amber-600 bg-amber-50'
  return 'text-red-600 bg-red-50'
})

const overallScore = computed(() => report.value?.overall_score || 0)
const dimensions = computed(() => {
  const scores = report.value?.dimension_scores || {}
  return Object.entries(scores).map(([name, score]) => ({ name, score }))
})
const strengths = computed(() => report.value?.strengths || [])
const weaknesses = computed(() => report.value?.weaknesses || [])
const suggestions = computed(() => report.value?.suggestions || [])
const summary = computed(() => report.value?.summary || '')

const duration = computed(() => {
  const seconds = reportResponse.value?.duration_seconds || 0
  const minutes = Math.floor(seconds / 60)
  const remainSeconds = seconds % 60
  return `${minutes}分 ${remainSeconds}秒`
})

const completedAt = computed(() => {
  const value = reportResponse.value?.completed_at
  if (!value) return '-'
  return new Date(value).toLocaleString('zh-CN')
})

const dimensionColor = (score: number) => {
  if (score >= 80) return '#22c55e'
  if (score >= 60) return '#f59e0b'
  return '#ef4444'
}

const loadReport = async () => {
  loading.value = true
  try {
    const res = await $api.get<ReportResponse>(`/interviews/${interviewId.value}/report`)
    if (res.code === 200 && res.data) {
      reportResponse.value = res.data
    }
  } catch (e) {
    console.error('Failed to load report:', e)
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  appStore.setPageTitle('面试报告')
  loadReport()
})
</script>

<template>
  <div v-loading="loading" class="max-w-4xl mx-auto">
    <div class="mb-6">
      <el-button text :icon="ArrowLeft" @click="router.push('/interview')" class="!-ml-2 mb-2">返回面试列表</el-button>
      <h1 class="text-2xl font-bold text-gray-900">面试报告</h1>
      <div class="flex items-center gap-4 mt-2 text-sm text-gray-500">
        <span>完成时间: {{ completedAt }}</span>
        <span>面试时长: {{ duration }}</span>
      </div>
    </div>

    <div class="bg-white rounded-lg shadow-sm p-8 mb-6" v-if="report">
      <div class="flex items-center gap-10">
        <div class="flex flex-col items-center">
          <el-progress type="circle" :percentage="overallScore" :width="140" :stroke-width="10" :color="scoreColor">
            <template #default>
              <div class="text-center">
                <span class="text-4xl font-bold" :style="{ color: scoreColor }">{{ overallScore }}</span>
                <span class="block text-sm text-gray-400">分</span>
              </div>
            </template>
          </el-progress>
          <span class="mt-3 px-4 py-1 rounded-full text-sm font-medium" :class="scoreLevelClass">{{ scoreLevel }}</span>
        </div>

        <div class="flex-1 space-y-4" v-if="dimensions.length">
          <div v-for="dim in dimensions" :key="dim.name" class="flex items-center gap-3">
            <span class="w-24 text-sm text-gray-600 text-right">{{ dim.name }}</span>
            <el-progress :percentage="dim.score" :stroke-width="12" :color="dimensionColor(dim.score)" class="flex-1" :show-text="false" />
            <span class="w-12 text-sm font-semibold text-right" :style="{ color: dimensionColor(dim.score) }">{{ dim.score }}分</span>
          </div>
        </div>
      </div>
    </div>

    <div v-if="summary" class="bg-white rounded-lg shadow-sm p-6 mb-6">
      <h3 class="text-lg font-semibold text-gray-800 mb-4">总结</h3>
      <p class="text-sm text-gray-700 whitespace-pre-wrap leading-relaxed">{{ summary }}</p>
    </div>

    <div class="grid grid-cols-1 md:grid-cols-2 gap-6 mb-6" v-if="strengths.length || weaknesses.length">
      <div class="bg-white rounded-lg shadow-sm p-6">
        <h3 class="text-lg font-semibold text-gray-800 mb-4 flex items-center gap-2">
          <span class="w-6 h-6 rounded-full bg-green-100 flex items-center justify-center">
            <el-icon class="text-green-500" :size="14"><Check /></el-icon>
          </span>
          优势
        </h3>
        <ul class="space-y-3">
          <li v-for="(item, idx) in strengths" :key="idx" class="flex items-start gap-2 text-sm text-gray-700">
            <el-icon class="text-green-500 mt-0.5 flex-shrink-0"><Check /></el-icon>
            {{ item }}
          </li>
        </ul>
        <p v-if="!strengths.length" class="text-sm text-gray-400">暂无数据</p>
      </div>

      <div class="bg-white rounded-lg shadow-sm p-6">
        <h3 class="text-lg font-semibold text-gray-800 mb-4 flex items-center gap-2">
          <span class="w-6 h-6 rounded-full bg-red-100 flex items-center justify-center">
            <el-icon class="text-red-500" :size="14"><Close /></el-icon>
          </span>
          待提升
        </h3>
        <ul class="space-y-3">
          <li v-for="(item, idx) in weaknesses" :key="idx" class="flex items-start gap-2 text-sm text-gray-700">
            <el-icon class="text-red-500 mt-0.5 flex-shrink-0"><Close /></el-icon>
            {{ item }}
          </li>
        </ul>
        <p v-if="!weaknesses.length" class="text-sm text-gray-400">暂无数据</p>
      </div>
    </div>

    <div class="bg-white rounded-lg shadow-sm p-6 mb-6" v-if="suggestions.length">
      <h3 class="text-lg font-semibold text-gray-800 mb-4">改进建议</h3>
      <div class="space-y-3">
        <div v-for="(item, idx) in suggestions" :key="idx" class="flex items-start gap-3 text-sm text-gray-700">
          <span class="w-6 h-6 rounded-full bg-blue-100 text-blue-600 flex items-center justify-center flex-shrink-0 text-xs font-bold">
            {{ idx + 1 }}
          </span>
          {{ item }}
        </div>
      </div>
    </div>

    <div class="flex justify-center gap-4 pb-8">
      <el-button size="large" :icon="RefreshRight" @click="router.push('/interview')" class="!px-8">
        再来一次
      </el-button>
      <el-button size="large" type="primary" :icon="ArrowLeft" @click="router.push('/interview')" class="!px-8">
        返回列表
      </el-button>
    </div>
  </div>
</template>
