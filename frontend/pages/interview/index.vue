<script setup lang="ts">
import {
  VideoCamera, TrendCharts, Trophy, Calendar, Plus,
  View, Right,
} from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'

definePageMeta({
  title: '模拟面试',
  layout: 'default',
  middleware: ['auth'],
})

type InterviewItem = {
  id: number
  status: string
  score?: number
  total_questions?: number
  created_at?: string
}

const appStore = useAppStore()
const { $api } = useNuxtApp()
const router = useRouter()

const stats = ref({ total: 0, avgScore: 0, maxScore: 0, monthCount: 0 })
const interviews = ref<InterviewItem[]>([])
const loading = ref(false)
const page = ref(1)
const pageSize = ref(10)
const total = ref(0)

const showDialog = ref(false)
const creating = ref(false)
const form = ref({
  difficulty: 'medium',
  question_count: 5,
  industry_code: 'go',
})

const difficultyOptions = [
  { label: '简单', value: 'easy' },
  { label: '中等', value: 'medium' },
  { label: '困难', value: 'hard' },
  { label: '混合', value: 'mixed' },
]

const industryOptions = [
  { label: 'Go 后端', value: 'go' },
  { label: 'Java', value: 'java' },
  { label: '前端', value: 'frontend' },
  { label: 'Python', value: 'python' },
  { label: 'AI', value: 'ai' },
]

const difficultyColor = (difficulty: string) => {
  const map: Record<string, string> = {
    easy: 'success',
    medium: 'warning',
    hard: 'danger',
    mixed: 'info',
  }
  return map[difficulty] || 'info'
}

const difficultyLabel = (difficulty: string) => {
  const map: Record<string, string> = {
    easy: '简单',
    medium: '中等',
    hard: '困难',
    mixed: '混合',
  }
  return map[difficulty] || '-'
}

const statusLabel = (status: string) => {
  const map: Record<string, string> = {
    ongoing: '进行中',
    completed: '已完成',
    finished: '已完成',
    cancelled: '已取消',
  }
  return map[status] || status
}

const statusType = (status: string) => {
  const map: Record<string, string> = {
    ongoing: 'primary',
    completed: 'success',
    finished: 'success',
    cancelled: 'info',
  }
  return (map[status] || 'info') as any
}

const formatDate = (date: string) => {
  if (!date) return '-'
  return new Date(date).toLocaleDateString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  })
}

const computeStats = (list: InterviewItem[]) => {
  stats.value.total = total.value

  if (!list.length) {
    stats.value.avgScore = 0
    stats.value.maxScore = 0
    stats.value.monthCount = 0
    return
  }

  const scores = list
    .map(item => item.score)
    .filter((score): score is number => typeof score === 'number')

  stats.value.avgScore = scores.length
    ? Math.round(scores.reduce((sum, score) => sum + score, 0) / scores.length)
    : 0
  stats.value.maxScore = scores.length ? Math.max(...scores) : 0

  const now = new Date()
  stats.value.monthCount = list.filter(item => {
    const createdAt = item.created_at ? new Date(item.created_at) : null
    return createdAt && createdAt.getMonth() === now.getMonth() && createdAt.getFullYear() === now.getFullYear()
  }).length
}

const fetchInterviews = async () => {
  loading.value = true
  try {
    const res = await $api.get<any>('/interviews', { page: page.value, page_size: pageSize.value })
    if (res.code === 200 && res.data) {
      interviews.value = res.data.list || []
      total.value = res.data.total || interviews.value.length
      computeStats(interviews.value)
    }
  } catch (e) {
    console.error('Failed to load interviews:', e)
  } finally {
    loading.value = false
  }
}

const createInterview = async () => {
  creating.value = true
  try {
    const res = await $api.post<any>('/interviews', {
      industry_code: form.value.industry_code,
      difficulty: form.value.difficulty,
      question_count: form.value.question_count,
    })

    if (res.code === 200 && res.data) {
      const interviewId = res.data.interview_id || res.data.id
      showDialog.value = false
      ElMessage.success('面试创建成功')
      if (interviewId) {
        router.push(`/interview/${interviewId}`)
      } else {
        await fetchInterviews()
      }
    }
  } catch (e: any) {
    ElMessage.error(e?.data?.message || '创建失败')
  } finally {
    creating.value = false
  }
}

const handlePageChange = (value: number) => {
  page.value = value
  fetchInterviews()
}

onMounted(() => {
  appStore.setPageTitle('模拟面试')
  fetchInterviews()
})
</script>

<template>
  <div>
    <div class="mb-6 flex items-center justify-between">
      <div>
        <h1 class="text-2xl font-bold text-gray-900">模拟面试</h1>
        <p class="text-gray-500 mt-1">AI 驱动的真实面试模拟体验，提升面试能力</p>
      </div>
      <el-button type="primary" size="large" :icon="Plus" @click="showDialog = true" class="!rounded-xl !px-8 !h-12 !text-base shadow-lg shadow-blue-200">
        开始新面试
      </el-button>
    </div>

    <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-6 mb-6">
      <StatsCard title="总面试次数" :value="stats.total" :icon="VideoCamera" bg-color="#ecf5ff" icon-color="#409eff" suffix="次" />
      <StatsCard title="平均分" :value="stats.avgScore" :icon="TrendCharts" bg-color="#fdf6ec" icon-color="#e6a23c" suffix="分" />
      <StatsCard title="最高分" :value="stats.maxScore" :icon="Trophy" bg-color="#f0f9eb" icon-color="#67c23a" suffix="分" />
      <StatsCard title="本月面试" :value="stats.monthCount" :icon="Calendar" bg-color="#fef0f0" icon-color="#f56c6c" suffix="次" />
    </div>

    <div class="bg-white rounded-lg shadow-sm p-6">
      <h2 class="text-lg font-semibold text-gray-800 mb-4">面试历史</h2>

      <el-table :data="interviews" v-loading="loading" stripe style="width: 100%" empty-text="还没有面试记录，开始你的第一次模拟面试吧">
        <el-table-column label="日期" min-width="160">
          <template #default="{ row }">
            {{ formatDate(row.created_at) }}
          </template>
        </el-table-column>
        <el-table-column label="题数" width="80" align="center">
          <template #default="{ row }">{{ row.total_questions || '-' }}</template>
        </el-table-column>
        <el-table-column label="得分" min-width="180">
          <template #default="{ row }">
            <div v-if="row.score != null" class="flex items-center gap-2">
              <el-progress :percentage="row.score" :stroke-width="10" :color="row.score >= 80 ? '#67c23a' : row.score >= 60 ? '#e6a23c' : '#f56c6c'" class="flex-1" />
              <span class="text-sm font-semibold w-10 text-right">{{ row.score }}分</span>
            </div>
            <span v-else class="text-gray-400">-</span>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="100" align="center">
          <template #default="{ row }">
            <el-tag :type="statusType(row.status)" size="small">
              {{ statusLabel(row.status) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="180" align="center">
          <template #default="{ row }">
            <el-button v-if="row.status === 'completed' || row.status === 'finished'" type="primary" text size="small" :icon="View" @click="router.push(`/interview/report/${row.id}`)">
              查看报告
            </el-button>
            <el-button v-if="row.status === 'ongoing'" type="success" text size="small" :icon="Right" @click="router.push(`/interview/${row.id}`)">
              继续面试
            </el-button>
          </template>
        </el-table-column>
      </el-table>

      <div v-if="total > pageSize" class="mt-4 flex justify-center">
        <el-pagination background layout="prev, pager, next" :total="total" :page-size="pageSize" :current-page="page" @current-change="handlePageChange" />
      </div>
    </div>

    <el-dialog v-model="showDialog" title="配置新面试" width="520px" :close-on-click-modal="false" class="!rounded-xl">
      <div class="space-y-6">
        <div>
          <label class="block text-sm font-medium text-gray-700 mb-2">难度选择</label>
          <el-radio-group v-model="form.difficulty" size="large">
            <el-radio-button v-for="opt in difficultyOptions" :key="opt.value" :value="opt.value">{{ opt.label }}</el-radio-button>
          </el-radio-group>
        </div>
        <div>
          <label class="block text-sm font-medium text-gray-700 mb-2">题目数量: {{ form.question_count }} 题</label>
          <el-slider v-model="form.question_count" :min="3" :max="15" :step="1" show-stops />
        </div>
        <div>
          <label class="block text-sm font-medium text-gray-700 mb-2">行业方向</label>
          <el-select v-model="form.industry_code" placeholder="选择行业" size="large" class="w-full">
            <el-option v-for="opt in industryOptions" :key="opt.value" :label="opt.label" :value="opt.value" />
          </el-select>
        </div>
      </div>
      <template #footer>
        <el-button @click="showDialog = false">取消</el-button>
        <el-button type="primary" :loading="creating" @click="createInterview" class="!px-8">
          开始面试
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>
