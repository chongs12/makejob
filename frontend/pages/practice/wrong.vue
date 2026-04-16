<script setup lang="ts">
/**
 * 错题本页面
 */

import {
  ArrowLeft,
  Refresh,
  View,
  TrendCharts,
} from '@element-plus/icons-vue'

definePageMeta({
  title: '错题本',
  layout: 'default',
  middleware: ['auth'],
})

const appStore = useAppStore()
const router = useRouter()
const api = useApi()

// ========== 状态 ==========
const loading = ref(true)
const currentPage = ref(1)
const pageSize = ref(20)
const total = ref(0)

interface WrongQuestion {
  id: number
  question_id: number
  title: string
  difficulty: string
  type: string
  my_answer: string
  correct_answer: string
  wrong_count?: number
  created_at: string
}
const wrongList = ref<WrongQuestion[]>([])

// 统计
const statsLoading = ref(true)
const stats = ref({
  total_wrong: 0,
  week_new: 0,
})

// ========== 数据获取 ==========
const fetchWrongList = async () => {
  loading.value = true
  try {
    const res = await api.get<any>('/user/wrong-questions', {
      page: currentPage.value,
      page_size: pageSize.value,
    })
    wrongList.value = res.data?.list || res.data?.items || []
    total.value = res.data?.total || 0
    stats.value.total_wrong = res.data?.total || wrongList.value.length
    stats.value.week_new = res.data?.week_new || 0
  } catch (e) {
    console.error('获取错题列表失败', e)
  } finally {
    loading.value = false
    statsLoading.value = false
  }
}

const handlePageChange = (page: number) => {
  currentPage.value = page
  fetchWrongList()
}

const goToQuestion = (row: WrongQuestion) => {
  router.push(`/practice/${row.question_id || row.id}`)
}

// 工具函数
const difficultyColor = (d: string) => {
  const map: Record<string, string> = {
    easy: 'text-emerald-600 bg-emerald-50 border-emerald-200',
    medium: 'text-amber-600 bg-amber-50 border-amber-200',
    hard: 'text-rose-600 bg-rose-50 border-rose-200',
  }
  return map[d] || 'text-secondary-600 bg-secondary-50 border-secondary-200'
}
const difficultyLabel = (d: string) => {
  const map: Record<string, string> = { easy: '简单', medium: '中等', hard: '困难' }
  return map[d] || d || '未知'
}

const formatDate = (dateStr: string) => {
  if (!dateStr) return '-'
  const d = new Date(dateStr)
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')} ${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`
}

onMounted(() => {
  appStore.setPageTitle('错题本')
  fetchWrongList()
})
</script>

<template>
  <div>
    <!-- 顶部 -->
    <div class="flex items-center justify-between mb-6">
      <div class="flex items-center gap-3">
        <el-button text :icon="ArrowLeft" @click="router.push('/practice')">返回练习</el-button>
        <el-divider direction="vertical" />
        <h1 class="text-xl font-semibold text-secondary-900">错题本</h1>
      </div>
    </div>

    <!-- 统计卡片 -->
    <div class="grid grid-cols-2 gap-4 mb-6">
      <div class="bg-white rounded-xl border border-secondary-200 p-5">
        <div class="flex items-center gap-3">
          <div class="w-11 h-11 bg-rose-50 rounded-lg flex items-center justify-center">
            <el-icon :size="22" class="text-rose-500"><TrendCharts /></el-icon>
          </div>
          <div>
            <div class="text-2xl font-bold text-secondary-900">{{ stats.total_wrong }}</div>
            <div class="text-xs text-secondary-500">累计错题</div>
          </div>
        </div>
      </div>
      <div class="bg-white rounded-xl border border-secondary-200 p-5">
        <div class="flex items-center gap-3">
          <div class="w-11 h-11 bg-amber-50 rounded-lg flex items-center justify-center">
            <el-icon :size="22" class="text-amber-500"><Refresh /></el-icon>
          </div>
          <div>
            <div class="text-2xl font-bold text-secondary-900">{{ stats.week_new }}</div>
            <div class="text-xs text-secondary-500">本周新增</div>
          </div>
        </div>
      </div>
    </div>

    <!-- 错题列表 -->
    <div class="bg-white rounded-xl border border-secondary-200 overflow-hidden">
      <el-table
        :data="wrongList"
        v-loading="loading"
        stripe
        style="width: 100%"
        :header-cell-style="{ background: '#f8fafc', color: '#334155', fontWeight: '600', fontSize: '13px' }"
      >
        <el-table-column label="题目" min-width="250">
          <template #default="{ row }">
            <span
              class="text-sm font-medium text-secondary-800 hover:text-primary-600 cursor-pointer transition-colors"
              @click="goToQuestion(row)"
            >
              {{ row.title }}
            </span>
          </template>
        </el-table-column>

        <el-table-column label="难度" width="100" align="center">
          <template #default="{ row }">
            <span
              :class="difficultyColor(row.difficulty)"
              class="inline-block px-2.5 py-0.5 rounded-full text-xs font-medium border"
            >
              {{ difficultyLabel(row.difficulty) }}
            </span>
          </template>
        </el-table-column>

        <el-table-column label="我的答案" width="120" align="center">
          <template #default="{ row }">
            <span class="text-sm font-medium text-rose-600">{{ row.my_answer || '-' }}</span>
          </template>
        </el-table-column>

        <el-table-column label="正确答案" width="120" align="center">
          <template #default="{ row }">
            <span class="text-sm font-medium text-emerald-600">{{ row.correct_answer || '-' }}</span>
          </template>
        </el-table-column>

        <el-table-column label="错误时间" width="170" align="center">
          <template #default="{ row }">
            <span class="text-sm text-secondary-500">{{ formatDate(row.created_at) }}</span>
          </template>
        </el-table-column>

        <el-table-column label="操作" width="140" align="center">
          <template #default="{ row }">
            <el-button size="small" type="primary" text :icon="Refresh" @click="goToQuestion(row)">
              重做
            </el-button>
            <el-button size="small" text :icon="View" @click="goToQuestion(row)">
              查看
            </el-button>
          </template>
        </el-table-column>
      </el-table>

      <!-- 空状态 -->
      <div v-if="!loading && wrongList.length === 0" class="py-20">
        <el-empty>
          <template #description>
            <div>
              <div class="text-lg font-semibold text-secondary-800 mb-1">太棒了！暂无错题</div>
              <div class="text-sm text-secondary-500">继续保持，你的表现非常出色！🎉</div>
            </div>
          </template>
          <el-button type="primary" @click="router.push('/practice')">去刷题</el-button>
        </el-empty>
      </div>

      <!-- 分页 -->
      <div v-if="total > pageSize" class="flex justify-center py-4 border-t border-secondary-100">
        <el-pagination
          v-model:current-page="currentPage"
          :page-size="pageSize"
          :total="total"
          layout="prev, pager, next, total"
          @current-change="handlePageChange"
        />
      </div>
    </div>
  </div>
</template>
