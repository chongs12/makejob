<script setup lang="ts">
/**
 * 面经采集管理页面
 * 实现搜索、爬取、清洗、导入的完整流程
 */

import { 
  Search, 
  Download, 
  Refresh, 
  View,
  Check,
  Close,
  Delete,
  DocumentCopy,
  Cpu,
  Upload
} from '@element-plus/icons-vue'

definePageMeta({
  title: '面经采集',
  layout: 'admin',
  middleware: ['admin']
})

const api = useApi()

// 数据源类型
interface Source {
  name: string
  label: string
  base_url: string
  is_active: boolean
}

// 搜索结果类型
interface SearchResult {
  title: string
  url: string
  author: string
  date: string
  summary: string
  source: string
  view_count: number
}

// 清洗后的题目类型
interface CleanedQuestion {
  title: string
  content: string
  type: string
  difficulty: string
  category: string
  answer: string
  explanation: string
  tags: string[]
  confidence: number
  selected?: boolean
}

// 任务类型
interface ScraperTask {
  id: number
  source_url: string
  source_title: string
  source: string
  status: string
  question_count: number
  imported_count: number
  error_msg: string
  created_at: string
  updated_at: string
}

// 状态
const loading = ref(false)
const sources = ref<Source[]>([])
const searchKeyword = ref('')
const selectedSource = ref('niuke')
const searchResults = ref<SearchResult[]>([])
const selectedResults = ref<SearchResult[]>([])
const activeTab = ref('search')

// 爬取弹窗状态
const fetchDialogVisible = ref(false)
const fetchLoading = ref(false)
const cleanLoading = ref(false)
const importLoading = ref(false)
const currentStep = ref(0) // 0: 原始内容, 1: 清洗结果, 2: 导入确认
const fetchedContent = ref('')
const fetchedTitle = ref('')
const currentUrl = ref('')
const currentSource = ref('')
const cleanedQuestions = ref<CleanedQuestion[]>([])
const selectedQuestions = ref<CleanedQuestion[]>([])

// 导入相关
const industries = ref<any[]>([])
const selectedIndustry = ref('')

// 任务列表
const tasks = ref<ScraperTask[]>([])
const tasksTotal = ref(0)
const tasksPage = ref(1)
const tasksPageSize = ref(10)

// 获取数据源列表
const fetchSources = async () => {
  try {
    const res = await api.get<{ data: Source[] }>('/api/admin/scraper/sources')
    if (res.code === 0) {
      sources.value = res.data
    }
  } catch (error) {
    console.error('获取数据源失败', error)
  }
}

// 获取行业列表
const fetchIndustries = async () => {
  try {
    const res = await api.get<{ data: any[] }>('/api/admin/industries')
    if (res.code === 0) {
      industries.value = res.data
      if (industries.value.length > 0) {
        selectedIndustry.value = industries.value[0].code
      }
    }
  } catch (error) {
    console.error('获取行业列表失败', error)
  }
}

// 搜索面经
const handleSearch = async () => {
  if (!searchKeyword.value.trim()) {
    ElMessage.warning('请输入搜索关键词')
    return
  }

  loading.value = true
  try {
    const res = await api.post<{ data: SearchResult[] }>('/api/admin/scraper/search', {
      keyword: searchKeyword.value,
      source: selectedSource.value,
      page: 1,
      page_size: 20
    })
    if (res.code === 0) {
      searchResults.value = res.data
      ElMessage.success(`找到 ${res.data.length} 条面经`)
    }
  } catch (error) {
    console.error('搜索失败', error)
  } finally {
    loading.value = false
  }
}

// 选择结果
const handleSelectionChange = (selection: SearchResult[]) => {
  selectedResults.value = selection
}

// 爬取单条面经
const handleFetch = async (row: SearchResult) => {
  currentUrl.value = row.url
  currentSource.value = row.source
  currentStep.value = 0
  fetchedContent.value = ''
  cleanedQuestions.value = []
  
  fetchDialogVisible.value = true
  fetchLoading.value = true

  try {
    const res = await api.post<{ data: any }>('/api/admin/scraper/fetch', {
      url: row.url,
      source: row.source
    })
    if (res.code === 0) {
      fetchedContent.value = res.data.content
      fetchedTitle.value = res.data.title
    }
  } catch (error) {
    console.error('爬取失败', error)
    ElMessage.error('爬取面经失败')
  } finally {
    fetchLoading.value = false
  }
}

// AI清洗
const handleClean = async () => {
  cleanLoading.value = true
  currentStep.value = 1

  try {
    const res = await api.post<{ data: any }>('/api/admin/scraper/clean', {
      content: fetchedContent.value,
      industry_code: selectedIndustry.value,
      source: currentSource.value,
      source_url: currentUrl.value
    })
    if (res.code === 0) {
      cleanedQuestions.value = res.data.questions.map((q: CleanedQuestion) => ({
        ...q,
        selected: true
      }))
      selectedQuestions.value = [...cleanedQuestions.value]
      currentStep.value = 1
    }
  } catch (error) {
    console.error('清洗失败', error)
    ElMessage.error('AI清洗失败')
  } finally {
    cleanLoading.value = false
  }
}

// 切换题目选择
const toggleQuestionSelection = (question: CleanedQuestion) => {
  question.selected = !question.selected
  selectedQuestions.value = cleanedQuestions.value.filter(q => q.selected)
}

// 进入导入步骤
const handleImportStep = () => {
  if (selectedQuestions.value.length === 0) {
    ElMessage.warning('请至少选择一道题目')
    return
  }
  currentStep.value = 2
}

// 确认导入
const handleImport = async () => {
  if (!selectedIndustry.value) {
    ElMessage.warning('请选择目标行业')
    return
  }

  importLoading.value = true

  try {
    const res = await api.post<{ data: any }>('/api/admin/scraper/import', {
      industry_code: selectedIndustry.value,
      questions: selectedQuestions.value,
      source_url: currentUrl.value,
      source_title: fetchedTitle.value
    })
    if (res.code === 0) {
      ElMessage.success(`成功导入 ${res.data.success_count} 道题目`)
      fetchDialogVisible.value = false
      fetchTasks()
    }
  } catch (error) {
    console.error('导入失败', error)
    ElMessage.error('导入题目失败')
  } finally {
    importLoading.value = false
  }
}

// 获取任务列表
const fetchTasks = async () => {
  loading.value = true
  try {
    const res = await api.get<{ data: any }>(`/api/admin/scraper/tasks?page=${tasksPage.value}&page_size=${tasksPageSize.value}`)
    if (res.code === 0) {
      tasks.value = res.data.list || []
      tasksTotal.value = res.data.total || 0
    }
  } catch (error) {
    console.error('获取任务列表失败', error)
  } finally {
    loading.value = false
  }
}

// 获取状态标签类型
const getStatusType = (status: string) => {
  const statusMap: Record<string, string> = {
    pending: 'info',
    fetched: 'warning',
    cleaned: 'primary',
    imported: 'success',
    failed: 'danger'
  }
  return statusMap[status] || 'info'
}

// 获取状态文本
const getStatusText = (status: string) => {
  const statusMap: Record<string, string> = {
    pending: '待处理',
    fetched: '已爬取',
    cleaned: '已清洗',
    imported: '已导入',
    failed: '失败'
  }
  return statusMap[status] || status
}

// 获取难度颜色
const getDifficultyColor = (difficulty: string) => {
  const colorMap: Record<string, string> = {
    easy: 'text-green-600 bg-green-50',
    medium: 'text-yellow-600 bg-yellow-50',
    hard: 'text-red-600 bg-red-50'
  }
  return colorMap[difficulty] || 'text-gray-600 bg-gray-50'
}

// 获取难度文本
const getDifficultyText = (difficulty: string) => {
  const textMap: Record<string, string> = {
    easy: '简单',
    medium: '中等',
    hard: '困难'
  }
  return textMap[difficulty] || difficulty
}

// 获取题目类型文本
const getTypeText = (type: string) => {
  const textMap: Record<string, string> = {
    choice: '单选题',
    multi: '多选题',
    code: '编程题',
    subjective: '主观题'
  }
  return textMap[type] || type
}

// 初始化
onMounted(() => {
  fetchSources()
  fetchIndustries()
  fetchTasks()
})
</script>

<template>
  <div class="space-y-6">
    <!-- 页面标题 -->
    <div class="flex items-center justify-between">
      <h1 class="text-2xl font-bold text-secondary-900">面经采集</h1>
    </div>

    <!-- Tab切换 -->
    <el-tabs v-model="activeTab" class="bg-white rounded-lg border border-secondary-200 p-4">
      <!-- 搜索Tab -->
      <el-tab-pane label="搜索面经" name="search">
        <!-- 搜索区域 -->
        <div class="flex gap-4 mb-6">
          <el-input
            v-model="searchKeyword"
            placeholder="输入搜索关键词，如：字节跳动 Go 后端"
            class="w-96"
            clearable
            @keyup.enter="handleSearch"
          >
            <template #prefix>
              <el-icon><Search /></el-icon>
            </template>
          </el-input>
          <el-select v-model="selectedSource" class="w-32">
            <el-option
              v-for="source in sources"
              :key="source.name"
              :label="source.label"
              :value="source.name"
            />
          </el-select>
          <el-button type="primary" @click="handleSearch" :loading="loading">
            <el-icon class="mr-1"><Search /></el-icon>
            搜索
          </el-button>
        </div>

        <!-- 搜索结果列表 -->
        <el-table
          :data="searchResults"
          style="width: 100%"
          @selection-change="handleSelectionChange"
          v-loading="loading"
        >
          <el-table-column type="selection" width="55" />
          <el-table-column prop="title" label="标题" min-width="300">
            <template #default="{ row }">
              <div class="flex flex-col">
                <span class="font-medium text-secondary-900 line-clamp-1">{{ row.title }}</span>
                <span class="text-xs text-secondary-500 mt-1 line-clamp-1">{{ row.summary }}</span>
              </div>
            </template>
          </el-table-column>
          <el-table-column prop="source" label="来源" width="100">
            <template #default="{ row }">
              <el-tag size="small" type="info">{{ row.source }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="author" label="作者" width="120" />
          <el-table-column prop="date" label="日期" width="120" />
          <el-table-column prop="view_count" label="浏览量" width="100" align="right" />
          <el-table-column label="操作" width="100" fixed="right">
            <template #default="{ row }">
              <el-button type="primary" size="small" @click="handleFetch(row)">
                <el-icon class="mr-1"><Download /></el-icon>
                爬取
              </el-button>
            </template>
          </el-table-column>
        </el-table>
      </el-tab-pane>

      <!-- 历史记录Tab -->
      <el-tab-pane label="历史记录" name="history">
        <el-table :data="tasks" style="width: 100%" v-loading="loading">
          <el-table-column prop="source_title" label="来源标题" min-width="250">
            <template #default="{ row }">
              <div class="flex flex-col">
                <span class="font-medium text-secondary-900 line-clamp-1">{{ row.source_title || '未知标题' }}</span>
                <span class="text-xs text-secondary-500 mt-1 line-clamp-1">{{ row.source_url }}</span>
              </div>
            </template>
          </el-table-column>
          <el-table-column prop="source" label="来源" width="100">
            <template #default="{ row }">
              <el-tag size="small" type="info">{{ row.source }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="question_count" label="提取题目" width="100" align="center" />
          <el-table-column prop="imported_count" label="已导入" width="100" align="center" />
          <el-table-column prop="status" label="状态" width="100">
            <template #default="{ row }">
              <el-tag :type="getStatusType(row.status)" size="small">
                {{ getStatusText(row.status) }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="created_at" label="创建时间" width="180">
            <template #default="{ row }">
              {{ new Date(row.created_at).toLocaleString() }}
            </template>
          </el-table-column>
        </el-table>

        <div class="flex justify-end mt-4">
          <el-pagination
            v-model:current-page="tasksPage"
            :page-size="tasksPageSize"
            :total="tasksTotal"
            layout="total, prev, pager, next"
            @current-change="fetchTasks"
          />
        </div>
      </el-tab-pane>
    </el-tabs>

    <!-- 爬取与清洗弹窗 -->
    <el-dialog
      v-model="fetchDialogVisible"
      :title="fetchedTitle || '面经内容'"
      width="90%"
      top="5vh"
      :close-on-click-modal="false"
    >
      <!-- 步骤条 -->
      <el-steps :active="currentStep" class="mb-6">
        <el-step title="爬取内容" />
        <el-step title="AI清洗" />
        <el-step title="确认导入" />
      </el-steps>

      <!-- Step 0: 原始内容 -->
      <div v-if="currentStep === 0" v-loading="fetchLoading">
        <div class="mb-4">
          <span class="text-sm text-secondary-500">来源: {{ currentUrl }}</span>
        </div>
        <el-input
          v-model="fetchedContent"
          type="textarea"
          :rows="20"
          readonly
          placeholder="面经内容将显示在这里..."
        />
        <div class="flex justify-end mt-4">
          <el-button @click="fetchDialogVisible = false">取消</el-button>
          <el-button type="primary" @click="handleClean" :loading="cleanLoading" :disabled="!fetchedContent">
            <el-icon class="mr-1"><Cpu /></el-icon>
            AI清洗
          </el-button>
        </div>
      </div>

      <!-- Step 1: 清洗结果 -->
      <div v-if="currentStep === 1" v-loading="cleanLoading">
        <div class="mb-4 flex items-center justify-between">
          <span class="text-sm text-secondary-500">
            已提取 {{ cleanedQuestions.length }} 道题目，已选择 {{ selectedQuestions.length }} 道
          </span>
          <div class="flex gap-2">
            <el-button size="small" @click="cleanedQuestions.forEach(q => q.selected = true); selectedQuestions = [...cleanedQuestions]">
              全选
            </el-button>
            <el-button size="small" @click="cleanedQuestions.forEach(q => q.selected = false); selectedQuestions = []">
              取消全选
            </el-button>
          </div>
        </div>

        <div class="max-h-96 overflow-y-auto space-y-4">
          <div
            v-for="(question, index) in cleanedQuestions"
            :key="index"
            class="border rounded-lg p-4 transition-all cursor-pointer"
            :class="question.selected ? 'border-primary-500 bg-primary-50' : 'border-secondary-200 hover:border-secondary-300'"
            @click="toggleQuestionSelection(question)"
          >
            <div class="flex items-start justify-between mb-2">
              <div class="flex items-center gap-2">
                <el-checkbox :model-value="question.selected" @click.stop />
                <span class="font-medium text-secondary-900">{{ index + 1 }}. {{ question.title }}</span>
              </div>
              <div class="flex items-center gap-2">
                <el-tag :class="getDifficultyColor(question.difficulty)" size="small">
                  {{ getDifficultyText(question.difficulty) }}
                </el-tag>
                <el-tag type="info" size="small">{{ getTypeText(question.type) }}</el-tag>
                <el-tag type="warning" size="small">{{ question.category }}</el-tag>
              </div>
            </div>
            <div class="text-sm text-secondary-600 mb-2">{{ question.content }}</div>
            <div class="flex items-center gap-2">
              <span class="text-xs text-secondary-500">置信度:</span>
              <el-progress
                :percentage="question.confidence * 100"
                :stroke-width="6"
                :color="question.confidence < 0.7 ? '#f56c6c' : '#67c23a'"
                :show-text="false"
                class="w-24"
              />
              <span class="text-xs" :class="question.confidence < 0.7 ? 'text-red-500' : 'text-green-500'">
                {{ (question.confidence * 100).toFixed(0) }}%
              </span>
            </div>
            <div v-if="question.tags?.length" class="mt-2 flex flex-wrap gap-1">
              <el-tag v-for="tag in question.tags" :key="tag" size="small" type="info" effect="plain">
                {{ tag }}
              </el-tag>
            </div>
          </div>
        </div>

        <div class="flex justify-between mt-4">
          <el-button @click="currentStep = 0">
            <el-icon class="mr-1"><Refresh /></el-icon>
            重新爬取
          </el-button>
          <div class="flex gap-2">
            <el-button @click="fetchDialogVisible = false">取消</el-button>
            <el-button type="primary" @click="handleImportStep" :disabled="selectedQuestions.length === 0">
              <el-icon class="mr-1"><Upload /></el-icon>
              确认导入 ({{ selectedQuestions.length }})
            </el-button>
          </div>
        </div>
      </div>

      <!-- Step 2: 导入确认 -->
      <div v-if="currentStep === 2">
        <div class="mb-6">
          <label class="block text-sm font-medium text-secondary-700 mb-2">选择目标行业</label>
          <el-select v-model="selectedIndustry" class="w-64">
            <el-option
              v-for="industry in industries"
              :key="industry.code"
              :label="industry.name"
              :value="industry.code"
            />
          </el-select>
        </div>

        <div class="bg-secondary-50 rounded-lg p-4 mb-4">
          <div class="text-sm text-secondary-600 mb-2">即将导入的题目:</div>
          <div class="text-lg font-semibold text-secondary-900">{{ selectedQuestions.length }} 道</div>
        </div>

        <div class="flex justify-between mt-4">
          <el-button @click="currentStep = 1">
            <el-icon class="mr-1"><Refresh /></el-icon>
            返回编辑
          </el-button>
          <div class="flex gap-2">
            <el-button @click="fetchDialogVisible = false">取消</el-button>
            <el-button type="primary" @click="handleImport" :loading="importLoading">
              <el-icon class="mr-1"><Check /></el-icon>
              确认导入
            </el-button>
          </div>
        </div>
      </div>
    </el-dialog>
  </div>
</template>
