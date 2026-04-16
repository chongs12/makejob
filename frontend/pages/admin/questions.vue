<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { Delete, Edit, Plus, Search, Upload } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'

definePageMeta({
  title: '题库管理',
  layout: 'admin',
  middleware: ['admin'],
})

type QuestionType = 'choice' | 'multi' | 'code' | 'subjective'
type DifficultyType = 'easy' | 'medium' | 'hard'

interface IndustryItem {
  id: number
  code: string
  name: string
}

interface CategoryItem {
  id: number
  industry_id: number
  name: string
  parent_id?: number | null
  sort_order?: number
  icon?: string
  description?: string
}

interface QuestionItem {
  id: number
  industry_id: number
  category_id: number
  category_name?: string
  title: string
  type: QuestionType
  difficulty: DifficultyType
  content: string
  options: string[]
  answer: string
  explanation: string
  tags: string[]
  is_active: boolean
  created_at?: string
}

interface TreeNode extends CategoryItem {
  label: string
  value: number
  children?: TreeNode[]
}

interface QuestionFormState {
  title: string
  industry_id: number | null
  category_path: number[]
  type: QuestionType
  difficulty: DifficultyType
  content: string
  options: string[]
  answer: string
  explanation: string
  tags: string
  is_active: boolean
}

interface CategoryFormState {
  industry_id: number | null
  name: string
  parent_id?: number
  sort_order: number
  icon: string
  description: string
}

const api = useApi()

const activeTab = ref('questions')
const loading = ref(false)
const categoryLoading = ref(false)

const questions = ref<QuestionItem[]>([])
const categories = ref<CategoryItem[]>([])
const industries = ref<IndustryItem[]>([])

const total = ref(0)
const page = ref(1)
const pageSize = ref(10)
const searchKeyword = ref('')
const difficultyFilter = ref('')
const categoryFilter = ref<number[]>([])

const questionDialogVisible = ref(false)
const questionDialogTitle = ref('新增题目')
const editingQuestionId = ref<number | null>(null)

const categoryDialogVisible = ref(false)
const categoryDialogTitle = ref('新增分类')
const editingCategoryId = ref<number | null>(null)

const importDialogVisible = ref(false)
const importJson = ref('')
const importIndustry = ref('')
const importPreview = ref<any[]>([])
const importLoading = ref(false)

const difficultyOptions = [
  { label: '全部难度', value: '' },
  { label: '简单', value: 'easy' },
  { label: '中等', value: 'medium' },
  { label: '困难', value: 'hard' },
]

const difficultyTagMap: Record<string, { type: string; label: string }> = {
  easy: { type: 'success', label: '简单' },
  medium: { type: 'warning', label: '中等' },
  hard: { type: 'danger', label: '困难' },
}

const typeTagMap: Record<string, { type: string; label: string }> = {
  choice: { type: '', label: '单选题' },
  multi: { type: 'warning', label: '多选题' },
  code: { type: 'danger', label: '编程题' },
  subjective: { type: 'info', label: '主观题' },
}

const createQuestionForm = (): QuestionFormState => ({
  title: '',
  industry_id: null,
  category_path: [],
  type: 'choice',
  difficulty: 'medium',
  content: '',
  options: ['', '', '', ''],
  answer: '',
  explanation: '',
  tags: '',
  is_active: true,
})

const createCategoryForm = (): CategoryFormState => ({
  industry_id: null,
  name: '',
  parent_id: undefined,
  sort_order: 0,
  icon: '',
  description: '',
})

const questionForm = ref<QuestionFormState>(createQuestionForm())
const categoryForm = ref<CategoryFormState>(createCategoryForm())

const categoryMap = computed(() => {
  const map = new Map<number, CategoryItem>()
  categories.value.forEach(category => map.set(category.id, category))
  return map
})

const buildTree = (list: CategoryItem[], parentId: number | null = null, industryId?: number): TreeNode[] => {
  const normalizedParentId = parentId ?? 0

  return list
    .filter(item => (item.parent_id ?? 0) === normalizedParentId)
    .filter(item => industryId ? item.industry_id === industryId : true)
    .sort((a, b) => {
      const orderDiff = (a.sort_order ?? 0) - (b.sort_order ?? 0)
      if (orderDiff !== 0) return orderDiff
      return a.id - b.id
    })
    .map((item) => {
      const children = buildTree(list, item.id, industryId)
      return {
        ...item,
        label: item.name,
        value: item.id,
        ...(children.length ? { children } : {}),
      }
    })
}

const categoryTree = computed(() => buildTree(categories.value))

const questionCategoryTree = computed(() => {
  if (!questionForm.value.industry_id) return []
  return buildTree(categories.value, null, questionForm.value.industry_id)
})

const categoryParentOptions = computed(() => {
  if (!categoryForm.value.industry_id) return []

  const blockedIds = getCategoryDescendantIds(editingCategoryId.value)
  if (editingCategoryId.value) {
    blockedIds.add(editingCategoryId.value)
  }

  return categories.value
    .filter(category => category.industry_id === categoryForm.value.industry_id)
    .filter(category => !blockedIds.has(category.id))
})

const getCategoryPath = (categoryId?: number | null): number[] => {
  if (!categoryId) return []

  const path: number[] = []
  const visited = new Set<number>()
  let currentId: number | null | undefined = categoryId

  while (currentId) {
    if (visited.has(currentId)) break
    visited.add(currentId)

    const category = categoryMap.value.get(currentId)
    if (!category) break

    path.unshift(category.id)
    currentId = category.parent_id ?? null
  }

  return path
}

const getCategoryDescendantIds = (categoryId?: number | null): Set<number> => {
  const descendants = new Set<number>()
  if (!categoryId) return descendants

  const queue = [categoryId]
  while (queue.length) {
    const currentId = queue.shift()
    if (!currentId) continue

    categories.value
      .filter(category => (category.parent_id ?? 0) === currentId)
      .forEach((category) => {
        if (descendants.has(category.id)) return
        descendants.add(category.id)
        queue.push(category.id)
      })
  }

  return descendants
}

const resetQuestionCategoryIfInvalid = () => {
  const selectedCategoryId = questionForm.value.category_path.at(-1)
  if (!selectedCategoryId || !questionForm.value.industry_id) return

  const category = categoryMap.value.get(selectedCategoryId)
  if (!category || category.industry_id !== questionForm.value.industry_id) {
    questionForm.value.category_path = []
  }
}

const resetCategoryParentIfInvalid = () => {
  if (!categoryForm.value.parent_id || !categoryForm.value.industry_id) return

  const parent = categoryMap.value.get(categoryForm.value.parent_id)
  if (!parent || parent.industry_id !== categoryForm.value.industry_id) {
    categoryForm.value.parent_id = undefined
  }
}

watch(() => questionForm.value.industry_id, () => {
  resetQuestionCategoryIfInvalid()
})

watch(() => categoryForm.value.industry_id, () => {
  resetCategoryParentIfInvalid()
})

watch(() => questionForm.value.type, (type) => {
  if (type === 'choice' || type === 'multi') {
    if (questionForm.value.options.length < 2) {
      questionForm.value.options = ['', '']
    }
    return
  }

  questionForm.value.options = []
})

const fetchQuestions = async () => {
  loading.value = true
  try {
    const params: Record<string, any> = {
      page: page.value,
      page_size: pageSize.value,
    }

    if (searchKeyword.value.trim()) params.keyword = searchKeyword.value.trim()
    if (difficultyFilter.value) params.difficulty = difficultyFilter.value
    if (categoryFilter.value.length) params.category_id = categoryFilter.value.at(-1)

    const res = await api.get<{ list: QuestionItem[]; total: number }>('/api/admin/questions', params)
    if (res.code === 0 || res.code === 200) {
      questions.value = res.data?.list || []
      total.value = res.data?.total || 0
    }
  } catch (error) {
    console.error('获取题目列表失败', error)
  } finally {
    loading.value = false
  }
}

const fetchCategories = async () => {
  categoryLoading.value = true
  try {
    const res = await api.get<CategoryItem[]>('/api/admin/categories')
    if (res.code === 0 || res.code === 200) {
      categories.value = Array.isArray(res.data) ? res.data : []
    }
  } catch (error) {
    console.error('获取分类列表失败', error)
  } finally {
    categoryLoading.value = false
  }
}

const fetchIndustries = async () => {
  try {
    const res = await api.get<IndustryItem[]>('/api/admin/industries')
    if (res.code === 0 || res.code === 200) {
      industries.value = Array.isArray(res.data) ? res.data : []
    }
  } catch (error) {
    console.error('获取行业列表失败', error)
  }
}

const handleSearch = () => {
  page.value = 1
  fetchQuestions()
}

const handlePageChange = (newPage: number) => {
  page.value = newPage
  fetchQuestions()
}

const openAddQuestion = () => {
  editingQuestionId.value = null
  questionDialogTitle.value = '新增题目'
  questionForm.value = createQuestionForm()
  questionDialogVisible.value = true
}

const openEditQuestion = (row: QuestionItem) => {
  editingQuestionId.value = row.id
  questionDialogTitle.value = '编辑题目'
  questionForm.value = {
    title: row.title || '',
    industry_id: row.industry_id || null,
    category_path: getCategoryPath(row.category_id),
    type: row.type || 'choice',
    difficulty: row.difficulty || 'medium',
    content: row.content || '',
    options: row.options?.length ? [...row.options] : (row.type === 'choice' || row.type === 'multi' ? ['', ''] : []),
    answer: row.answer || '',
    explanation: row.explanation || '',
    tags: Array.isArray(row.tags) ? row.tags.join(', ') : '',
    is_active: row.is_active ?? true,
  }
  questionDialogVisible.value = true
}

const addOption = () => {
  questionForm.value.options.push('')
}

const removeOption = (index: number) => {
  if (questionForm.value.options.length <= 2) return
  questionForm.value.options.splice(index, 1)
}

const saveQuestion = async () => {
  const form = questionForm.value
  const categoryId = form.category_path.at(-1)
  const normalizedOptions = form.options.map(item => item.trim()).filter(Boolean)
  const normalizedTags = form.tags
    .split(/[,，]/)
    .map(item => item.trim())
    .filter(Boolean)

  if (!form.title.trim()) {
    ElMessage.warning('请输入题目标题')
    return
  }
  if (!form.industry_id) {
    ElMessage.warning('请选择所属行业')
    return
  }
  if (!categoryId) {
    ElMessage.warning('请选择题目分类')
    return
  }
  if (!form.content.trim()) {
    ElMessage.warning('请输入题目内容')
    return
  }
  if (!form.answer.trim()) {
    ElMessage.warning('请输入答案')
    return
  }
  if ((form.type === 'choice' || form.type === 'multi') && normalizedOptions.length < 2) {
    ElMessage.warning('选择题至少需要两个选项')
    return
  }

  const payload = {
    title: form.title.trim(),
    industry_id: form.industry_id,
    category_id: categoryId,
    type: form.type,
    difficulty: form.difficulty,
    content: form.content.trim(),
    options_json: form.type === 'choice' || form.type === 'multi' ? JSON.stringify(normalizedOptions) : '',
    answer: form.answer.trim(),
    explanation: form.explanation.trim(),
    tags: normalizedTags.join(','),
    is_active: form.is_active,
  }

  try {
    if (editingQuestionId.value) {
      await api.put(`/api/admin/questions/${editingQuestionId.value}`, payload)
      ElMessage.success('题目更新成功')
    } else {
      await api.post('/api/admin/questions', payload)
      ElMessage.success('题目创建成功')
    }

    questionDialogVisible.value = false
    await fetchQuestions()
  } catch (error) {
    console.error('保存题目失败', error)
  }
}

const handleDeleteQuestion = async (row: QuestionItem) => {
  try {
    await ElMessageBox.confirm(`确定删除题目“${row.title}”吗？该操作不可恢复。`, '删除确认', {
      confirmButtonText: '删除',
      cancelButtonText: '取消',
      type: 'warning',
    })

    await api.delete(`/api/admin/questions/${row.id}`)
    ElMessage.success('题目删除成功')
    await fetchQuestions()
  } catch (error: any) {
    if (error !== 'cancel') {
      console.error('删除题目失败', error)
    }
  }
}

const openImportDialog = () => {
  importJson.value = ''
  importIndustry.value = ''
  importPreview.value = []
  importDialogVisible.value = true
}

const parseImportJson = () => {
  try {
    const parsed = JSON.parse(importJson.value)
    importPreview.value = Array.isArray(parsed) ? parsed : (parsed.questions || [])
    ElMessage.success(`解析成功，共 ${importPreview.value.length} 条题目`)
  } catch (error) {
    ElMessage.error('JSON 格式不正确')
  }
}

const handleImport = async () => {
  if (!importIndustry.value) {
    ElMessage.warning('请选择导入行业')
    return
  }
  if (!importPreview.value.length) {
    ElMessage.warning('请先解析导入内容')
    return
  }

  importLoading.value = true
  try {
    const res = await api.post<{ success_count: number }>('/api/admin/questions/import', {
      industry_code: importIndustry.value,
      questions: importPreview.value,
    })

    if (res.code === 0 || res.code === 200) {
      ElMessage.success(`批量导入成功，共导入 ${res.data?.success_count || importPreview.value.length} 条`)
      importDialogVisible.value = false
      await fetchQuestions()
    }
  } catch (error) {
    console.error('批量导入失败', error)
  } finally {
    importLoading.value = false
  }
}

const openAddCategory = () => {
  editingCategoryId.value = null
  categoryDialogTitle.value = '新增分类'
  categoryForm.value = createCategoryForm()
  categoryDialogVisible.value = true
}

const openEditCategory = (category: CategoryItem) => {
  editingCategoryId.value = category.id
  categoryDialogTitle.value = '编辑分类'
  categoryForm.value = {
    industry_id: category.industry_id || null,
    name: category.name || '',
    parent_id: category.parent_id ?? undefined,
    sort_order: category.sort_order ?? 0,
    icon: category.icon || '',
    description: category.description || '',
  }
  categoryDialogVisible.value = true
}

const saveCategory = async () => {
  const form = categoryForm.value

  if (!form.industry_id) {
    ElMessage.warning('请选择所属行业')
    return
  }
  if (!form.name.trim()) {
    ElMessage.warning('请输入分类名称')
    return
  }

  const payload = {
    industry_id: form.industry_id,
    name: form.name.trim(),
    parent_id: form.parent_id || null,
    sort_order: form.sort_order || 0,
    icon: form.icon.trim(),
    description: form.description.trim(),
  }

  try {
    if (editingCategoryId.value) {
      await api.put(`/api/admin/categories/${editingCategoryId.value}`, payload)
      ElMessage.success('分类更新成功')
    } else {
      await api.post('/api/admin/categories', payload)
      ElMessage.success('分类创建成功')
    }

    categoryDialogVisible.value = false
    await fetchCategories()
  } catch (error) {
    console.error('保存分类失败', error)
  }
}

const handleDeleteCategory = async (category: CategoryItem) => {
  try {
    await ElMessageBox.confirm(`确定删除分类“${category.name}”吗？该操作不可恢复。`, '删除确认', {
      confirmButtonText: '删除',
      cancelButtonText: '取消',
      type: 'warning',
    })

    await api.delete(`/api/admin/categories/${category.id}`)
    ElMessage.success('分类删除成功')
    await fetchCategories()
  } catch (error: any) {
    if (error !== 'cancel') {
      console.error('删除分类失败', error)
    }
  }
}

const formatDate = (value?: string) => {
  if (!value) return '-'
  return new Date(value).toLocaleString('zh-CN')
}

onMounted(async () => {
  await Promise.all([
    fetchIndustries(),
    fetchCategories(),
  ])
  await fetchQuestions()
})
</script>

<template>
  <div class="space-y-6">
    <h1 class="text-2xl font-bold text-secondary-900">题库管理</h1>

    <el-tabs v-model="activeTab" class="rounded-lg border border-secondary-200 bg-white p-4 shadow-sm">
      <el-tab-pane label="题目管理" name="questions">
        <div class="mb-6 flex flex-wrap items-center justify-between gap-4">
          <div class="flex flex-wrap items-center gap-4">
            <el-input
              v-model="searchKeyword"
              placeholder="搜索题目标题或内容"
              class="w-72"
              clearable
              @keyup.enter="handleSearch"
              @clear="handleSearch"
            >
              <template #prefix>
                <el-icon><Search /></el-icon>
              </template>
            </el-input>

            <el-cascader
              v-model="categoryFilter"
              :options="categoryTree"
              :props="{ checkStrictly: true, value: 'id', label: 'name', emitPath: true }"
              placeholder="按分类筛选"
              clearable
              class="w-56"
              @change="handleSearch"
            />

            <el-select v-model="difficultyFilter" placeholder="难度" class="w-32" @change="handleSearch">
              <el-option v-for="item in difficultyOptions" :key="item.value" :label="item.label" :value="item.value" />
            </el-select>
          </div>

          <div class="flex flex-wrap gap-2">
            <el-button type="primary" :icon="Plus" @click="openAddQuestion">新增题目</el-button>
            <el-button plain :icon="Upload" @click="openImportDialog">批量导入</el-button>
          </div>
        </div>

        <el-table :data="questions" v-loading="loading" style="width: 100%">
          <el-table-column prop="id" label="ID" width="70" />
          <el-table-column prop="title" label="标题" min-width="240" show-overflow-tooltip />
          <el-table-column label="行业" width="140">
            <template #default="{ row }">
              <span>{{ industries.find(item => item.id === row.industry_id)?.name || '-' }}</span>
            </template>
          </el-table-column>
          <el-table-column label="分类" width="160">
            <template #default="{ row }">
              <span>{{ row.category_name || '-' }}</span>
            </template>
          </el-table-column>
          <el-table-column label="难度" width="100">
            <template #default="{ row }">
              <el-tag :type="(difficultyTagMap[row.difficulty]?.type as any) || 'info'" size="small">
                {{ difficultyTagMap[row.difficulty]?.label || row.difficulty }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column label="题型" width="100">
            <template #default="{ row }">
              <el-tag :type="(typeTagMap[row.type]?.type as any) || 'info'" size="small">
                {{ typeTagMap[row.type]?.label || row.type }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column label="状态" width="100">
            <template #default="{ row }">
              <el-tag :type="row.is_active ? 'success' : 'info'" size="small">
                {{ row.is_active ? '启用' : '停用' }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column label="创建时间" width="180">
            <template #default="{ row }">
              {{ formatDate(row.created_at) }}
            </template>
          </el-table-column>
          <el-table-column label="操作" width="140" fixed="right">
            <template #default="{ row }">
              <el-button type="primary" link size="small" :icon="Edit" @click="openEditQuestion(row)">编辑</el-button>
              <el-button type="danger" link size="small" :icon="Delete" @click="handleDeleteQuestion(row)">删除</el-button>
            </template>
          </el-table-column>

          <template #empty>
            <el-empty description="暂无题目数据" />
          </template>
        </el-table>

        <div class="mt-4 flex justify-end">
          <el-pagination
            v-model:current-page="page"
            :page-size="pageSize"
            :total="total"
            layout="total, prev, pager, next"
            @current-change="handlePageChange"
          />
        </div>
      </el-tab-pane>

      <el-tab-pane label="分类管理" name="categories">
        <div class="mb-4 flex flex-wrap items-center justify-between gap-4">
          <span class="text-sm text-secondary-500">分类会按父子层级展示，新增和编辑时请确保行业、父级分类匹配。</span>
          <el-button type="primary" :icon="Plus" @click="openAddCategory">新增分类</el-button>
        </div>

        <el-tree
          :data="categoryTree"
          node-key="id"
          default-expand-all
          :props="{ label: 'name', children: 'children' }"
          v-loading="categoryLoading"
        >
          <template #default="{ data }">
            <div class="flex flex-1 items-center justify-between gap-4 pr-2">
              <div class="min-w-0">
                <div class="font-medium text-secondary-900">{{ data.name }}</div>
                <div class="text-xs text-secondary-500">
                  {{ industries.find(item => item.id === data.industry_id)?.name || '未分配行业' }}
                  <span v-if="data.icon"> · {{ data.icon }}</span>
                </div>
              </div>
              <div class="flex shrink-0 gap-2">
                <el-button type="primary" link size="small" @click.stop="openEditCategory(data)">编辑</el-button>
                <el-button type="danger" link size="small" @click.stop="handleDeleteCategory(data)">删除</el-button>
              </div>
            </div>
          </template>
        </el-tree>

        <el-empty v-if="!categoryTree.length" description="暂无分类数据" />
      </el-tab-pane>
    </el-tabs>

    <el-dialog v-model="questionDialogVisible" :title="questionDialogTitle" width="760px" :close-on-click-modal="false">
      <el-form :model="questionForm" label-position="top">
        <el-form-item label="题目标题">
          <el-input v-model="questionForm.title" placeholder="请输入题目标题" />
        </el-form-item>

        <div class="grid gap-4 md:grid-cols-2">
          <el-form-item label="所属行业">
            <el-select v-model="questionForm.industry_id" placeholder="请选择行业" class="w-full" clearable>
              <el-option v-for="item in industries" :key="item.id" :label="item.name" :value="item.id" />
            </el-select>
          </el-form-item>

          <el-form-item label="题目分类">
            <el-cascader
              v-model="questionForm.category_path"
              :options="questionCategoryTree"
              :props="{ checkStrictly: true, value: 'id', label: 'name', emitPath: true }"
              placeholder="请选择分类"
              clearable
              class="w-full"
            />
          </el-form-item>
        </div>

        <div class="grid gap-4 md:grid-cols-2">
          <el-form-item label="题目类型">
            <el-radio-group v-model="questionForm.type">
              <el-radio-button value="choice">单选题</el-radio-button>
              <el-radio-button value="multi">多选题</el-radio-button>
              <el-radio-button value="code">编程题</el-radio-button>
              <el-radio-button value="subjective">主观题</el-radio-button>
            </el-radio-group>
          </el-form-item>

          <el-form-item label="难度">
            <el-radio-group v-model="questionForm.difficulty">
              <el-radio-button value="easy">简单</el-radio-button>
              <el-radio-button value="medium">中等</el-radio-button>
              <el-radio-button value="hard">困难</el-radio-button>
            </el-radio-group>
          </el-form-item>
        </div>

        <el-form-item label="题目内容">
          <el-input v-model="questionForm.content" type="textarea" :rows="6" placeholder="请输入题目内容" />
        </el-form-item>

        <el-form-item v-if="questionForm.type === 'choice' || questionForm.type === 'multi'" label="选项">
          <div class="w-full space-y-2">
            <div v-for="(_, index) in questionForm.options" :key="index" class="flex items-center gap-2">
              <span class="w-6 text-sm font-medium text-secondary-500">{{ String.fromCharCode(65 + index) }}.</span>
              <el-input v-model="questionForm.options[index]" :placeholder="`请输入选项 ${String.fromCharCode(65 + index)}`" />
              <el-button v-if="questionForm.options.length > 2" type="danger" link :icon="Delete" @click="removeOption(index)" />
            </div>
            <el-button type="primary" link :icon="Plus" @click="addOption">添加选项</el-button>
          </div>
        </el-form-item>

        <el-form-item label="答案">
          <el-input v-model="questionForm.answer" placeholder="请输入题目答案" />
        </el-form-item>

        <el-form-item label="解析">
          <el-input v-model="questionForm.explanation" type="textarea" :rows="4" placeholder="请输入题目解析" />
        </el-form-item>

        <div class="grid gap-4 md:grid-cols-2">
          <el-form-item label="标签">
            <el-input v-model="questionForm.tags" placeholder="多个标签请用逗号分隔" />
          </el-form-item>

          <el-form-item label="启用状态">
            <el-switch v-model="questionForm.is_active" inline-prompt active-text="启用" inactive-text="停用" />
          </el-form-item>
        </div>
      </el-form>

      <template #footer>
        <el-button @click="questionDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="saveQuestion">保存</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="importDialogVisible" title="批量导入题目" width="760px" :close-on-click-modal="false">
      <el-form label-position="top">
        <el-form-item label="所属行业">
          <el-select v-model="importIndustry" placeholder="请选择导入行业" class="w-64">
            <el-option v-for="item in industries" :key="item.id" :label="item.name" :value="item.code" />
          </el-select>
        </el-form-item>

        <el-form-item label="JSON 内容">
          <el-input
            v-model="importJson"
            type="textarea"
            :rows="12"
            placeholder='请输入 JSON 数组，例如 [{"title":"题目标题","category_name":"分类名称","type":"choice","difficulty":"easy","content":"题目内容","options_json":"[\"A\",\"B\"]","answer":"A"}]'
            style="font-family: monospace"
          />
        </el-form-item>

        <div class="mb-4 flex items-center gap-4">
          <el-button @click="parseImportJson">解析预览</el-button>
          <span v-if="importPreview.length" class="text-sm text-green-600">已解析 {{ importPreview.length }} 条数据</span>
        </div>

        <div v-if="importPreview.length" class="max-h-48 overflow-y-auto rounded-lg bg-secondary-50 p-4">
          <div
            v-for="(item, index) in importPreview.slice(0, 10)"
            :key="index"
            class="border-b border-secondary-200 py-2 text-sm text-secondary-700 last:border-0"
          >
            {{ index + 1 }}. {{ item.title || '未命名题目' }}
          </div>
          <div v-if="importPreview.length > 10" class="mt-2 text-sm text-secondary-400">
            还有 {{ importPreview.length - 10 }} 条数据未展示
          </div>
        </div>
      </el-form>

      <template #footer>
        <el-button @click="importDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="importLoading" :disabled="!importPreview.length" @click="handleImport">
          开始导入
        </el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="categoryDialogVisible" :title="categoryDialogTitle" width="560px" :close-on-click-modal="false">
      <el-form :model="categoryForm" label-position="top">
        <el-form-item label="所属行业">
          <el-select v-model="categoryForm.industry_id" placeholder="请选择行业" class="w-full" clearable>
            <el-option v-for="item in industries" :key="item.id" :label="item.name" :value="item.id" />
          </el-select>
        </el-form-item>

        <el-form-item label="分类名称">
          <el-input v-model="categoryForm.name" placeholder="请输入分类名称" />
        </el-form-item>

        <el-form-item label="父级分类">
          <el-select v-model="categoryForm.parent_id" placeholder="不选则为顶级分类" clearable class="w-full">
            <el-option v-for="item in categoryParentOptions" :key="item.id" :label="item.name" :value="item.id" />
          </el-select>
        </el-form-item>

        <div class="grid gap-4 md:grid-cols-2">
          <el-form-item label="排序">
            <el-input-number v-model="categoryForm.sort_order" :min="0" :max="999" class="w-full" />
          </el-form-item>

          <el-form-item label="图标">
            <el-input v-model="categoryForm.icon" placeholder="可填写图标名称或 URL" />
          </el-form-item>
        </div>

        <el-form-item label="描述">
          <el-input v-model="categoryForm.description" type="textarea" :rows="4" placeholder="请输入分类描述" />
        </el-form-item>
      </el-form>

      <template #footer>
        <el-button @click="categoryDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="saveCategory">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>
