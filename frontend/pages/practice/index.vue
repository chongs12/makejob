<script setup lang="ts">
import {
  Search,
  Star,
  StarFilled,
  Timer,
  Refresh,
  Document,
  Collection,
  ArrowRight,
} from '@element-plus/icons-vue'
import { normalizeQuestionType, questionTypeLabel } from '~/utils/question'

definePageMeta({
  title: '刷题练习',
  layout: 'default',
  middleware: ['auth'],
})

interface CategoryNode {
  id: number
  name: string
  children?: CategoryNode[]
}

interface Question {
  id: number
  title: string
  difficulty: string
  type: string
  category_name?: string
  pass_rate?: number
  is_favorite?: boolean
}

const appStore = useAppStore()
const router = useRouter()
const api = useApi()

const loading = ref(false)
const categoryLoading = ref(false)
const selectedCategoryId = ref<number | null>(null)
const difficulty = ref('')
const questionType = ref('')
const keyword = ref('')
const currentPage = ref(1)
const pageSize = ref(20)
const total = ref(0)
const categoryTree = ref<CategoryNode[]>([])
const questions = ref<Question[]>([])

const fetchCategories = async () => {
  categoryLoading.value = true
  try {
    const res = await api.get<any>('/categories', { industry_id: 1 })
    categoryTree.value = res.data || []
  } catch (e) {
    console.error('获取分类失败', e)
  } finally {
    categoryLoading.value = false
  }
}

const fetchQuestions = async () => {
  loading.value = true
  try {
    const params: Record<string, any> = {
      page: currentPage.value,
      page_size: pageSize.value,
    }

    if (selectedCategoryId.value) params.category_id = selectedCategoryId.value
    if (difficulty.value) params.difficulty = difficulty.value
    if (questionType.value) params.type = questionType.value
    if (keyword.value) params.keyword = keyword.value

    const res = await api.get<any>('/questions', params)
    questions.value = (res.data?.list || []).map((item: any) => ({
      ...item,
      type: normalizeQuestionType(item.type),
    }))
    total.value = res.data?.total || 0
  } catch (e) {
    console.error('获取题目列表失败', e)
  } finally {
    loading.value = false
  }
}

const handleCategoryClick = (data: CategoryNode) => {
  selectedCategoryId.value = data.id
  currentPage.value = 1
  fetchQuestions()
}

const handleSearch = () => {
  currentPage.value = 1
  fetchQuestions()
}

const handleDifficultyChange = () => {
  currentPage.value = 1
  fetchQuestions()
}

const handleTypeChange = () => {
  currentPage.value = 1
  fetchQuestions()
}

const handlePageChange = (page: number) => {
  currentPage.value = page
  fetchQuestions()
}

const goToQuestion = (row: Question) => {
  if (normalizeQuestionType(row.type) === 'code') {
    router.push(`/practice/editor/${row.id}`)
  } else {
    router.push(`/practice/${row.id}`)
  }
}

const toggleFavorite = async (row: Question) => {
  try {
    await api.post(`/questions/${row.id}/favorite`)
    row.is_favorite = !row.is_favorite
    ElMessage.success(row.is_favorite ? '已收藏' : '已取消收藏')
  } catch {
    ElMessage.error('操作失败')
  }
}

const openGeneratedExam = (data: any, label: string) => {
  const firstQuestion = data?.questions?.[0]
  if (!firstQuestion?.id) {
    ElMessage.info('暂无可用题目')
    return
  }

  ElMessage.success(`${label}已生成，先进入第 1 题`)
  router.push(`/practice/${firstQuestion.id}`)
}

const handleRandomPractice = async () => {
  try {
    const res = await api.post<any>('/exams/random', {
      count: 5,
      difficulty: difficulty.value || 'medium',
      category_id: selectedCategoryId.value || undefined,
    })
    openGeneratedExam(res.data, '随机练习')
  } catch {
    ElMessage.error('随机组卷失败')
  }
}

const handleTimedExam = async () => {
  try {
    const res = await api.post<any>('/exams/timed', {
      count: 5,
      difficulty: difficulty.value || 'medium',
      category_id: selectedCategoryId.value || undefined,
      time_limit_minutes: 30,
    })
    openGeneratedExam(res.data, '限时模拟')
  } catch {
    ElMessage.error('开始模拟失败')
  }
}

const difficultyColor = (value: string) => {
  const map: Record<string, string> = {
    easy: 'text-emerald-600 bg-emerald-50 border-emerald-200',
    medium: 'text-amber-600 bg-amber-50 border-amber-200',
    hard: 'text-rose-600 bg-rose-50 border-rose-200',
  }
  return map[value] || 'text-secondary-600 bg-secondary-50 border-secondary-200'
}

const difficultyLabel = (value: string) => {
  const map: Record<string, string> = { easy: '简单', medium: '中等', hard: '困难' }
  return map[value] || value || '未知'
}

const treeProps = {
  label: 'name',
  children: 'children',
}

onMounted(() => {
  appStore.setPageTitle('刷题练习')
  fetchCategories()
  fetchQuestions()
})
</script>

<template>
  <div class="flex gap-6 h-full">
    <aside class="w-64 flex-shrink-0">
      <div class="bg-white rounded-xl border border-secondary-200 overflow-hidden sticky top-6">
        <div class="px-4 py-3 border-b border-secondary-100 flex items-center gap-2">
          <el-icon class="text-primary-500"><Collection /></el-icon>
          <span class="text-sm font-semibold text-secondary-800">题目分类</span>
        </div>
        <div class="p-3 max-h-[calc(100vh-16rem)] overflow-y-auto" v-loading="categoryLoading">
          <el-tree
            :data="categoryTree"
            :props="treeProps"
            node-key="id"
            highlight-current
            default-expand-all
            :expand-on-click-node="false"
            @node-click="handleCategoryClick"
            class="practice-category-tree"
          />
          <div v-if="selectedCategoryId" class="mt-2 px-2">
            <el-button size="small" text type="primary" @click="selectedCategoryId = null; fetchQuestions()">
              清除筛选
            </el-button>
          </div>
        </div>
      </div>
    </aside>

    <div class="flex-1 min-w-0">
      <div class="bg-white rounded-xl border border-secondary-200 p-4 mb-4">
        <div class="flex items-center justify-between gap-4 flex-wrap">
          <div class="flex items-center gap-3 flex-wrap">
            <el-radio-group v-model="difficulty" size="default" @change="handleDifficultyChange">
              <el-radio-button label="">全部</el-radio-button>
              <el-radio-button label="easy">简单</el-radio-button>
              <el-radio-button label="medium">中等</el-radio-button>
              <el-radio-button label="hard">困难</el-radio-button>
            </el-radio-group>

            <el-select v-model="questionType" placeholder="题型" clearable style="width: 130px" @change="handleTypeChange">
              <el-option label="全部题型" value="" />
              <el-option label="选择题" value="choice" />
              <el-option label="多选题" value="multi" />
              <el-option label="编程题" value="code" />
              <el-option label="主观题" value="subjective" />
            </el-select>
          </div>

          <div class="flex items-center gap-3">
            <el-input
              v-model="keyword"
              placeholder="搜索题目..."
              :prefix-icon="Search"
              clearable
              style="width: 220px"
              @keyup.enter="handleSearch"
              @clear="handleSearch"
            />
            <el-button type="primary" plain :icon="Refresh" @click="handleRandomPractice">
              随机练习
            </el-button>
            <el-button type="warning" plain :icon="Timer" @click="handleTimedExam">
              限时模拟
            </el-button>
          </div>
        </div>
      </div>

      <div class="grid grid-cols-3 gap-4 mb-4">
        <NuxtLink
          to="/practice/wrong"
          class="group flex items-center gap-3 bg-white rounded-xl border border-secondary-200 p-4 hover:border-rose-300 hover:shadow-md transition-all"
        >
          <div class="w-10 h-10 bg-rose-50 rounded-lg flex items-center justify-center group-hover:scale-110 transition-transform">
            <el-icon :size="20" class="text-rose-500"><Document /></el-icon>
          </div>
          <div>
            <div class="text-sm font-semibold text-secondary-800">错题本</div>
            <div class="text-xs text-secondary-500">复习错题，查漏补缺</div>
          </div>
          <el-icon class="ml-auto text-secondary-400"><ArrowRight /></el-icon>
        </NuxtLink>

        <div class="group flex items-center gap-3 bg-white rounded-xl border border-secondary-200 p-4 hover:border-amber-300 hover:shadow-md transition-all cursor-pointer" @click="handleRandomPractice">
          <div class="w-10 h-10 bg-amber-50 rounded-lg flex items-center justify-center group-hover:scale-110 transition-transform">
            <el-icon :size="20" class="text-amber-500"><Refresh /></el-icon>
          </div>
          <div>
            <div class="text-sm font-semibold text-secondary-800">随机组卷</div>
            <div class="text-xs text-secondary-500">随机抽题检验水平</div>
          </div>
          <el-icon class="ml-auto text-secondary-400"><ArrowRight /></el-icon>
        </div>

        <div class="group flex items-center gap-3 bg-white rounded-xl border border-secondary-200 p-4 hover:border-primary-300 hover:shadow-md transition-all cursor-pointer" @click="handleTimedExam">
          <div class="w-10 h-10 bg-primary-50 rounded-lg flex items-center justify-center group-hover:scale-110 transition-transform">
            <el-icon :size="20" class="text-primary-500"><Timer /></el-icon>
          </div>
          <div>
            <div class="text-sm font-semibold text-secondary-800">限时模拟</div>
            <div class="text-xs text-secondary-500">模拟真实考试环境</div>
          </div>
          <el-icon class="ml-auto text-secondary-400"><ArrowRight /></el-icon>
        </div>
      </div>

      <div class="bg-white rounded-xl border border-secondary-200 overflow-hidden">
        <el-table
          :data="questions"
          v-loading="loading"
          stripe
          highlight-current-row
          style="width: 100%"
          row-class-name="cursor-pointer"
          @row-click="goToQuestion"
          :header-cell-style="{ background: '#f8fafc', color: '#334155', fontWeight: '600', fontSize: '13px' }"
        >
          <el-table-column type="index" label="#" width="60" align="center" />

          <el-table-column label="题目" min-width="300">
            <template #default="{ row }">
              <div class="flex items-center gap-2">
                <span class="text-sm font-medium text-secondary-800 hover:text-primary-600 transition-colors">
                  {{ row.title }}
                </span>
              </div>
            </template>
          </el-table-column>

          <el-table-column label="难度" width="100" align="center">
            <template #default="{ row }">
              <span :class="difficultyColor(row.difficulty)" class="inline-block px-2.5 py-0.5 rounded-full text-xs font-medium border">
                {{ difficultyLabel(row.difficulty) }}
              </span>
            </template>
          </el-table-column>

          <el-table-column label="题型" width="100" align="center">
            <template #default="{ row }">
              <el-tag size="small" type="info" effect="plain">
                {{ questionTypeLabel(row.type) }}
              </el-tag>
            </template>
          </el-table-column>

          <el-table-column label="通过率" width="100" align="center">
            <template #default="{ row }">
              <span class="text-sm text-secondary-600">
                {{ row.pass_rate != null ? `${(row.pass_rate * 100).toFixed(1)}%` : '-' }}
              </span>
            </template>
          </el-table-column>

          <el-table-column label="" width="60" align="center">
            <template #default="{ row }">
              <el-icon
                :size="18"
                class="cursor-pointer transition-colors"
                :class="row.is_favorite ? 'text-amber-400' : 'text-secondary-300 hover:text-amber-400'"
                @click.stop="toggleFavorite(row)"
              >
                <StarFilled v-if="row.is_favorite" />
                <Star v-else />
              </el-icon>
            </template>
          </el-table-column>
        </el-table>

        <div v-if="!loading && questions.length === 0" class="py-16">
          <el-empty description="暂无匹配题目，换一个筛选条件试试" />
        </div>

        <div v-if="total > 0" class="flex justify-center py-4 border-t border-secondary-100">
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
  </div>
</template>

<style scoped>
.practice-category-tree :deep(.el-tree-node__content) {
  height: 36px;
  border-radius: 6px;
  padding: 0 8px;
}

.practice-category-tree :deep(.el-tree-node.is-current > .el-tree-node__content) {
  background-color: rgb(239 246 255);
  color: rgb(37 99 235);
}
</style>
