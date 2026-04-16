<script setup lang="ts">
/**
 * Live2D模型管理页面
 */

import { Plus, Edit, Delete } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'

definePageMeta({
  title: 'Live2D管理',
  layout: 'admin',
  middleware: ['admin']
})

const api = useApi()
const loading = ref(false)
const models = ref<any[]>([])

// 场景选项
const sceneOptions = [
  { label: '面试', value: 'interview' },
  { label: '陪伴', value: 'companion' },
]

// 获取模型列表
const fetchModels = async () => {
  loading.value = true
  try {
    const res = await api.get<any>('/api/admin/live2d-models')
    if (res.code === 0 || res.code === 200) {
      models.value = res.data?.list || res.data || []
    }
  } catch (error) {
    console.error('获取Live2D模型失败', error)
  } finally {
    loading.value = false
  }
}

// 弹窗
const dialogVisible = ref(false)
const dialogTitle = ref('新增模型')
const editingId = ref<number | null>(null)
const form = ref({
  name: '',
  scene: 'interview',
  industry_code: '',
  model_url: '',
  thumbnail_url: '',
  config_json: '',
  is_active: true,
})

// 行业列表
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

const openAdd = () => {
  editingId.value = null
  dialogTitle.value = '新增模型'
  form.value = { name: '', scene: 'interview', industry_code: '', model_url: '', thumbnail_url: '', config_json: '{}', is_active: true }
  dialogVisible.value = true
}

const openEdit = (item: any) => {
  editingId.value = item.id
  dialogTitle.value = '编辑模型'
  form.value = {
    name: item.name || '',
    scene: item.scene || 'interview',
    industry_code: item.industry_code || '',
    model_url: item.model_url || '',
    thumbnail_url: item.thumbnail_url || '',
    config_json: typeof item.config_json === 'string' ? item.config_json : JSON.stringify(item.config_json || {}, null, 2),
    is_active: item.is_active !== false,
  }
  dialogVisible.value = true
}

const save = async () => {
  if (!form.value.name.trim()) {
    ElMessage.warning('请输入模型名称')
    return
  }
  try {
    const payload = { ...form.value } as any
    // 尝试解析config_json
    try { payload.config_json = JSON.parse(form.value.config_json) } catch { /* keep as string */ }

    if (editingId.value) {
      await api.put(`/api/admin/live2d-models/${editingId.value}`, payload)
      ElMessage.success('模型更新成功')
    } else {
      await api.post('/api/admin/live2d-models', payload)
      ElMessage.success('模型创建成功')
    }
    dialogVisible.value = false
    fetchModels()
  } catch (error) {
    ElMessage.error('保存模型失败')
  }
}

const handleDelete = async (item: any) => {
  try {
    await ElMessageBox.confirm(`确定删除模型「${item.name}」吗？`, '确认删除', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning',
    })
    await api.delete(`/api/admin/live2d-models/${item.id}`)
    ElMessage.success('删除成功')
    fetchModels()
  } catch (error: any) {
    if (error !== 'cancel') ElMessage.error('删除失败')
  }
}

const getSceneLabel = (scene: string) => {
  return sceneOptions.find(s => s.value === scene)?.label || scene
}

onMounted(() => {
  fetchModels()
  fetchIndustries()
})
</script>

<template>
  <div class="space-y-6">
    <div class="flex items-center justify-between">
      <h1 class="text-2xl font-bold text-secondary-900">Live2D模型管理</h1>
      <el-button type="primary" :icon="Plus" @click="openAdd">新增模型</el-button>
    </div>

    <!-- 模型卡片网格 -->
    <div v-loading="loading" class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4">
      <div
        v-for="item in models"
        :key="item.id"
        class="bg-white rounded-lg shadow-sm border border-secondary-200 overflow-hidden hover:shadow-md transition-shadow"
      >
        <!-- 缩略图 -->
        <div class="h-40 bg-secondary-100 flex items-center justify-center">
          <img v-if="item.thumbnail_url" :src="item.thumbnail_url" :alt="item.name" class="h-full w-full object-cover" />
          <div v-else class="text-center text-secondary-400">
            <div class="text-4xl mb-2">🎭</div>
            <span class="text-xs">暂无缩略图</span>
          </div>
        </div>
        <div class="p-4">
          <div class="flex items-center justify-between mb-2">
            <h3 class="font-medium text-secondary-900 truncate">{{ item.name }}</h3>
            <el-tag :type="item.is_active ? 'success' : 'info'" size="small">
              {{ item.is_active ? '启用' : '禁用' }}
            </el-tag>
          </div>
          <div class="flex items-center gap-2 mb-3">
            <el-tag size="small" type="warning">{{ getSceneLabel(item.scene) }}</el-tag>
            <span v-if="item.industry_code" class="text-xs text-secondary-500">{{ item.industry_code }}</span>
          </div>
          <div class="flex items-center gap-2 pt-3 border-t border-secondary-100">
            <el-button type="primary" link size="small" :icon="Edit" @click="openEdit(item)">编辑</el-button>
            <el-button type="danger" link size="small" :icon="Delete" @click="handleDelete(item)">删除</el-button>
          </div>
        </div>
      </div>
    </div>
    <el-empty v-if="!loading && models.length === 0" description="暂无Live2D模型" />

    <!-- 新增/编辑弹窗 -->
    <el-dialog v-model="dialogVisible" :title="dialogTitle" width="600px" :close-on-click-modal="false">
      <el-form :model="form" label-width="100px">
        <el-form-item label="名称">
          <el-input v-model="form.name" placeholder="模型名称" />
        </el-form-item>
        <div class="grid grid-cols-2 gap-4">
          <el-form-item label="场景">
            <el-select v-model="form.scene" class="w-full">
              <el-option v-for="opt in sceneOptions" :key="opt.value" :label="opt.label" :value="opt.value" />
            </el-select>
          </el-form-item>
          <el-form-item label="行业绑定">
            <el-select v-model="form.industry_code" placeholder="选填" clearable class="w-full">
              <el-option v-for="ind in industries" :key="ind.code || ind.id" :label="ind.name" :value="ind.code" />
            </el-select>
          </el-form-item>
        </div>
        <el-form-item label="模型URL">
          <el-input v-model="form.model_url" placeholder="Live2D模型资源URL" />
        </el-form-item>
        <el-form-item label="缩略图URL">
          <el-input v-model="form.thumbnail_url" placeholder="模型缩略图URL" />
        </el-form-item>
        <el-form-item label="配置JSON">
          <el-input v-model="form.config_json" type="textarea" :rows="6" placeholder='{"scale": 1.0, "x": 0, "y": 0}' style="font-family: monospace" />
        </el-form-item>
        <el-form-item label="启用">
          <el-switch v-model="form.is_active" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="save">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>
