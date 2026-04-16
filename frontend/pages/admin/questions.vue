<script setup lang="ts">
/**
 * 题库管理页面
 * 双Tab: 题目管理 | 分类管理
 */

import { Search, Plus, Upload, Edit, Delete, Refresh } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'

definePageMeta({
  title: '题库管理',
  layout: 'admin',
  middleware: ['admin']
})

const api = useApi()

// ==================== 题目管理 ====================
const activeTab = ref('questions')
const loading = ref(false)
const questions = ref<any[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(10)
const searchKeyword = ref('')
const difficultyFilter = ref('')
const categoryFilter = ref<number[]>([])

// 分类数据(用于筛选和表单)
const categories = ref<any[]>([])
const categoryTree = ref<any[]>([])

// 难度选项
const difficultyOptions = [
  { label: '全部', value: '' },
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

// 获取题目列表
const fetchQuestions = async () => {
  loading.value = true
  try {
    const params: Record<string, any> = { page: page.value, page_size: pageSize.value }
    if (searchKeyword.value) params.keyword = searchKeyword.value
    if (difficultyFilter.value) params.difficulty = difficultyFilter.value
    if (categoryFilter.value.length) params.category_id = categoryFilter.value[categoryFilter.value.length - 1]

    const res = await api.get<any>('/api/admin/questions', params)
    if (res.code === 0 || res.code === 200) {
      questions.value = res.data?.list || res.data || []
      total.value = res.data?.total || 0
    }
  } catch (error) {
    console.error('获取题目列表失败', error)
  } finally {
    loading.value = false
  }
}

// 获取分类列表
const fetchCategories = async () => {
  try {
    const res = await api.get<any>('/api/admin/categories')
    if (res.code === 0 || res.code === 200) {
      const list = res.data?.list || res.data || []
      categories.value = list
      categoryTree.value = buildTree(list)
    }
  } catch (error) {
    console.error('获取分类失败', error)
  }
}

// 构建分类树
const buildTree = (list: any[], parentId = 0): any[] => {
  return list
    .filter((item: any) => (item.parent_id || 0) === parentId)
    .map((item: any) => ({
      ...item,
      label: item.name,
      value: item.id,
      children: buildTree(list, item.id),
    }))
    .map((item: any) => {
      if (item.children.length === 0) delete item.children
      return item
    })
}

// 搜索
const handleSearch = () => {
  page.value = 1
  fetchQuestions()
}

// ==================== 题目弹窗 ====================
const questionDialogVisible = ref(false)
const questionDialogTitle = ref('新增题目')
const editingQuestionId = ref<number | null>(null)
const questionForm = ref({
  title: '',
  category_id: [] as number[],
  type: 'choice',
  difficulty: 'medium',
  content: '',
  options: ['', '', '', ''],
  answer: '',
  explanation: '',
  tags: '',
})

const openAddQuestion = () => {
  editingQuestionId.value = null
  questionDialogTitle.value = '新增题目'
  questionForm.value = {
    title: '', category_id: [], type: 'choice', difficulty: 'medium',
    content: '', options: ['', '', '', ''], answer: '', explanation: '', tags: '',
  }
  questionDialogVisible.value = true
}

const openEditQuestion = (row: any) => {
  editingQuestionId.value = row.id
  questionDialogTitle.value = '编辑题目'
  questionForm.value = {
    title: row.title || '',
    category_id: row.category_id ? [row.category_id] : [],
    type: row.type || 'choice',
    difficulty: row.difficulty || 'medium',
    content: row.content || '',
    options: row.options?.length ? [...row.options] : ['', '', '', ''],
    answer: row.answer || '',
    explanation: row.explanation || '',
    tags: Array.isArray(row.tags) ? row.tags.join(', ') : (row.tags || ''),
  }
  questionDialogVisible.value = true
}

const addOption = () => {
  questionForm.value.options.push('')
}

const removeOption = (index: number) => {
  if (questionForm.value.options.length > 2) {
    questionForm.value.options.splice(index, 1)
  }
}

const saveQuestion = async () => {
  const form = questionForm.value
  if (!form.title.trim()) {
    ElMessage.warning('请输入题目标题')
    return
  }

  const payload: Record<string, any> = {
    title: form.title,
    category_id: form.category_id.length ? form.category_id[form.category_id.length - 1] : 0,
    type: form.type,
    difficulty: form.difficulty,
    content: form.content,
    answer: form.answer,
    explanation: form.explanation,
    tags: form.tags.split(/[,，]/).map(t => t.trim()).filter(Boolean),
  }
  if (form.type === 'choice' || form.type === 'multi') {
    payload.options = form.options.filter(Boolean)
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
    fetchQuestions()
  } catch (error) {
    ElMessage.error('保存题目失败')
  }
}

// 删除题目
const handleDeleteQuestion = async (row: any) => {
  try {
    await ElMessageBox.confirm(`确定删除题目「${row.title}」吗？此操作不可恢复。`, '确认删除', {
      confirmButtonText: '确定删除',
      cancelButtonText: '取消',
      type: 'warning',
    })
    await api.delete(`/api/admin/questions/${row.id}`)
    ElMessage.success('删除成功')
    fetchQuestions()
  } catch (error: any) {
    if (error !== 'cancel') ElMessage.error('删除失败')
  }
}

// ==================== 批量导入 ====================
const importDialogVisible = ref(false)
const importJson = ref('')
const importIndustry = ref('')
const importPreview = ref<any[]>([])
const importLoading = ref(false)
const industries = ref<any[]>([])

const fetchIndustries = async () => {
  try {
    const res = await api.get<any>('/api/admin/industries')
    if (res.code === 0 || res.code === 200) {
      industries.value = res.data?.list || res.data || []
    }
  } catch (error) {
    console.error('获取行业失败', error)
  }
}

const openImportDialog = () => {
  importJson.value = ''
  importPreview.value = []
  importDialogVisible.value = true
}

const parseImportJson = () => {
  try {
    const parsed = JSON.parse(importJson.value)
    importPreview.value = Array.isArray(parsed) ? parsed : (parsed.questions || [])
    ElMessage.success(`解析成功，共 ${importPreview.value.length} 道题目`)
  } catch {
    ElMessage.error('JSON格式错误，请检查')
  }
}

const handleImport = async () => {
  if (!importIndustry.value) {
    ElMessage.warning('请选择行业')
    return
  }
  if (importPreview.value.length === 0) {
    ElMessage.warning('请先解析JSON')
    return
  }
  importLoading.value = true
  try {
    const res = await api.post<any>('/api/admin/questions/import', {
      industry_code: importIndustry.value,
      questions: importPreview.value,
    })
    if (res.code === 0 || res.code === 200) {
      ElMessage.success(`导入成功: ${res.data?.success_count || importPreview.value.length} 条`)
      importDialogVisible.value = false
      fetchQuestions()
    }
  } catch (error) {
    ElMessage.error('批量导入失败')
  } finally {
    importLoading.value = false
  }
}

// ==================== 分类管理 ====================
const categoryLoading = ref(false)
const categoryDialogVisible = ref(false)
const categoryDialogTitle = ref('添加分类')
const editingCategoryId = ref<number | null>(null)
const categoryForm = ref({ name: '', parent_id: 0, sort_order: 0 })

const openAddCategory = () => {
  editingCategoryId.value = null
  categoryDialogTitle.value = '添加分类'
  categoryForm.value = { name: '', parent_id: 0, sort_order: 0 }
  categoryDialogVisible.value = true
}

const openEditCategory = (data: any) => {
  editingCategoryId.value = data.id
  categoryDialogTitle.value = '编辑分类'
  categoryForm.value = { name: data.name, parent_id: data.parent_id || 0, sort_order: data.sort_order || 0 }
  categoryDialogVisible.value = true
}

const saveCategory = async () => {
  if (!categoryForm.value.name.trim()) {
    ElMessage.warning('请输入分类名称')
    return
  }
  try {
    if (editingCategoryId.value) {
      await api.put(`/api/admin/categories/${editingCategoryId.value}`, categoryForm.value as any)
      ElMessage.success('分类更新成功')
    } else {
      await api.post('/api/admin/categories', categoryForm.value as any)
      ElMessage.success('分类创建成功')
    }
    categoryDialogVisible.value = false
    fetchCategories()
  } catch (error) {
    ElMessage.error('保存分类失败')
  }
}

const handleDeleteCategory = async (node: any, data: any) => {
  try {
    await ElMessageBox.confirm(`确定删除分类「${data.name}」吗？`, '确认删除', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning',
    })
    await api.delete(`/api/admin/categories/${data.id}`)
    ElMessage.success('删除成功')
    fetchCategories()
  } catch (error: any) {
    if (error !== 'cancel') ElMessage.error('删除失败')
  }
}

// 格式化时间
const formatDate = (dateStr: string) => {
  if (!dateStr) return '-'
  return new Date(dateStr).toLocaleString('zh-CN')
}

// 分页
const handlePageChange = (newPage: number) => {
  page.value = newPage
  fetchQuestions()
}

onMounted(() => {
  fetchQuestions()
  fetchCategories()
  fetchIndustries()
})
</script>

<template>
  <div class="space-y-6">
    <h1 class="text-2xl font-bold text-secondary-900">题库管理</h1>

    <el-tabs v-model="activeTab" class="bg-white rounded-lg shadow-sm border border-secondary-200 p-4">
      <!-- 题目管理Tab -->
      <el-tab-pane label="题目管理" name="questions">
        <!-- 工具栏 -->
        <div class="flex items-center justify-between mb-6">
          <div class="flex items-center gap-4">
            <el-input v-model="searchKeyword" placeholder="搜索题目" class="w-64" clearable @keyup.enter="handleSearch" @clear="handleSearch">
              <template #prefix><el-icon><Search /></el-icon></template>
            </el-input>
            <el-cascader v-model="categoryFilter" :options="categoryTree" :props="{ checkStrictly: true, value: 'id', label: 'name', emitPath: true }" placeholder="选择分类" clearable class="w-48" @change="handleSearch" />
            <el-select v-model="difficultyFilter" placeholder="难度" class="w-28" @change="handleSearch">
              <el-option v-for="opt in difficultyOptions" :key="opt.value" :label="opt.label" :value="opt.value" />
            </el-select>
          </div>
          <div class="flex gap-2">
            <el-button type="primary" :icon="Plus" @click="openAddQuestion">新增题目</el-button>
            <el-button plain :icon="Upload" @click="openImportDialog">批量导入</el-button>
          </div>
        </div>

        <!-- 表格 -->
        <el-table :data="questions" v-loading="loading" style="width: 100%">
          <el-table-column prop="id" label="ID" width="70" />
          <el-table-column prop="title" label="标题" min-width="250" show-overflow-tooltip />
          <el-table-column label="分类" width="120">
            <template #default="{ row }">
              <span class="text-sm">{{ row.category_name || '-' }}</span>
            </template>
          </el-table-column>
          <el-table-column label="难度" width="90">
            <template #default="{ row }">
              <el-tag :type="(difficultyTagMap[row.difficulty]?.type as any) || 'info'" size="small">
                {{ difficultyTagMap[row.difficulty]?.label || row.difficulty }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column label="题型" width="90">
            <template #default="{ row }">
              <el-tag :type="(typeTagMap[row.type]?.type as any) || 'info'" size="small">
                {{ typeTagMap[row.type]?.label || row.type }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column label="创建时间" width="180">
            <template #default="{ row }">
              <span class="text-sm text-secondary-500">{{ formatDate(row.created_at) }}</span>
            </template>
          </el-table-column>
          <el-table-column label="操作" width="140" fixed="right">
            <template #default="{ row }">
              <el-button type="primary" link size="small" :icon="Edit" @click="openEditQuestion(row)">编辑</el-button>
              <el-button type="danger" link size="small" :icon="Delete" @click="handleDeleteQuestion(row)">删除</el-button>
            </template>
          </el-table-column>
          <template #empty><el-empty description="暂无题目" /></template>
        </el-table>

        <div class="flex justify-end mt-4">
          <el-pagination v-model:current-page="page" :page-size="pageSize" :total="total" layout="total, prev, pager, next" @current-change="handlePageChange" />
        </div>
      </el-tab-pane>

      <!-- 分类管理Tab -->
      <el-tab-pane label="分类管理" name="categories">
        <div class="flex items-center justify-between mb-4">
          <span class="text-sm text-secondary-500">拖拽排序分类层级结构</span>
          <el-button type="primary" :icon="Plus" @click="openAddCategory">添加分类</el-button>
        </div>
        <el-tree :data="categoryTree" node-key="id" default-expand-all :props="{ label: 'name', children: 'children' }" v-loading="categoryLoading">
          <template #default="{ node, data }">
            <div class="flex items-center justify-between flex-1 pr-2">
              <span>{{ data.name }}</span>
              <div class="flex gap-2">
                <el-button type="primary" link size="small" @click.stop="openEditCategory(data)">编辑</el-button>
                <el-button type="danger" link size="small" @click.stop="handleDeleteCategory(node, data)">删除</el-button>
              </div>
            </div>
          </template>
        </el-tree>
        <el-empty v-if="categoryTree.length === 0" description="暂无分类" />
      </el-tab-pane>
    </el-tabs>

    <!-- 新增/编辑题目弹窗 -->
    <el-dialog v-model="questionDialogVisible" :title="questionDialogTitle" width="700px" :close-on-click-modal="false">
      <el-form :model="questionForm" label-width="80px" label-position="top">
        <el-form-item label="标题">
          <el-input v-model="questionForm.title" placeholder="请输入题目标题" />
        </el-form-item>
        <div class="grid grid-cols-2 gap-4">
          <el-form-item label="分类">
            <el-cascader v-model="questionForm.category_id" :options="categoryTree" :props="{ checkStrictly: true, value: 'id', label: 'name' }" placeholder="选择分类" clearable class="w-full" />
          </el-form-item>
          <el-form-item label="题型">
            <el-radio-group v-model="questionForm.type">
              <el-radio-button value="choice">单选</el-radio-button>
              <el-radio-button value="multi">多选</el-radio-button>
              <el-radio-button value="code">编程</el-radio-button>
              <el-radio-button value="subjective">主观</el-radio-button>
            </el-radio-group>
          </el-form-item>
        </div>
        <el-form-item label="难度">
          <el-radio-group v-model="questionForm.difficulty">
            <el-radio-button value="easy">简单</el-radio-button>
            <el-radio-button value="medium">中等</el-radio-button>
            <el-radio-button value="hard">困难</el-radio-button>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="题目内容">
          <el-input v-model="questionForm.content" type="textarea" :rows="6" placeholder="请输入题目内容" />
        </el-form-item>
        <!-- 选项(仅choice/multi) -->
        <el-form-item v-if="questionForm.type === 'choice' || questionForm.type === 'multi'" label="选项">
          <div class="w-full space-y-2">
            <div v-for="(_, idx) in questionForm.options" :key="idx" class="flex items-center gap-2">
              <span class="text-sm font-medium text-secondary-500 w-6">{{ String.fromCharCode(65 + idx) }}.</span>
              <el-input v-model="questionForm.options[idx]" :placeholder="`选项${String.fromCharCode(65 + idx)}`" />
              <el-button v-if="questionForm.options.length > 2" type="danger" link :icon="Delete" @click="removeOption(idx)" />
            </div>
            <el-button type="primary" link :icon="Plus" @click="addOption">添加选项</el-button>
          </div>
        </el-form-item>
        <el-form-item label="正确答案">
          <el-input v-model="questionForm.answer" placeholder="请输入正确答案" />
        </el-form-item>
        <el-form-item label="解析">
          <el-input v-model="questionForm.explanation" type="textarea" :rows="4" placeholder="请输入解析" />
        </el-form-item>
        <el-form-item label="标签">
          <el-input v-model="questionForm.tags" placeholder="多个标签用逗号分隔" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="questionDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="saveQuestion">保存</el-button>
      </template>
    </el-dialog>

    <!-- 批量导入弹窗 -->
    <el-dialog v-model="importDialogVisible" title="批量导入题目" width="700px" :close-on-click-modal="false">
      <el-form label-position="top">
        <el-form-item label="JSON数据">
          <el-input v-model="importJson" type="textarea" :rows="12" placeholder='请输入JSON数组，格式: [{"title":"...", "content":"...", "type":"choice", "difficulty":"easy", ...}]' style="font-family: monospace" />
        </el-form-item>
        <div class="flex items-center gap-4 mb-4">
          <el-button @click="parseImportJson">解析预览</el-button>
          <span v-if="importPreview.length" class="text-sm text-green-600">已解析 {{ importPreview.length }} 道题目</span>
        </div>
        <!-- 预览区 -->
        <div v-if="importPreview.length" class="bg-secondary-50 rounded-lg p-4 mb-4 max-h-40 overflow-y-auto">
          <div v-for="(q, i) in importPreview.slice(0, 10)" :key="i" class="text-sm text-secondary-700 py-1 border-b border-secondary-200 last:border-0">
            {{ i + 1 }}. {{ q.title || '未命名' }}
          </div>
          <div v-if="importPreview.length > 10" class="text-sm text-secondary-400 mt-2">...还有 {{ importPreview.length - 10 }} 条</div>
        </div>
        <el-form-item label="目标行业">
          <el-select v-model="importIndustry" placeholder="选择行业" class="w-64">
            <el-option v-for="ind in industries" :key="ind.code || ind.id" :label="ind.name" :value="ind.code || ind.id" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="importDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="handleImport" :loading="importLoading" :disabled="importPreview.length === 0">确认导入</el-button>
      </template>
    </el-dialog>

    <!-- 分类弹窗 -->
    <el-dialog v-model="categoryDialogVisible" :title="categoryDialogTitle" width="480px" :close-on-click-modal="false">
      <el-form :model="categoryForm" label-width="80px">
        <el-form-item label="名称">
          <el-input v-model="categoryForm.name" placeholder="请输入分类名称" />
        </el-form-item>
        <el-form-item label="父级分类">
          <el-select v-model="categoryForm.parent_id" placeholder="无(顶级分类)" clearable class="w-full">
            <el-option label="无(顶级分类)" :value="0" />
            <el-option v-for="cat in categories" :key="cat.id" :label="cat.name" :value="cat.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="排序">
          <el-input-number v-model="categoryForm.sort_order" :min="0" :max="999" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="categoryDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="saveCategory">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>
