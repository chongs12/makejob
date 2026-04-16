<script setup lang="ts">
/**
 * 行业管理页面
 */

import { Plus, Edit, OfficeBuilding } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'

definePageMeta({
  title: '行业管理',
  layout: 'admin',
  middleware: ['admin']
})

const api = useApi()
const loading = ref(false)
const industries = ref<any[]>([])

// 弹窗
const dialogVisible = ref(false)
const dialogTitle = ref('新增行业')
const editingId = ref<number | null>(null)
const form = ref({ code: '', name: '', description: '', icon: '', is_active: true })

// 图标选项
const iconOptions = ['💻', '🌐', '📱', '🎮', '🤖', '📊', '🔧', '🏢', '🎨', '📡']

const fetchIndustries = async () => {
  loading.value = true
  try {
    const res = await api.get<any>('/api/admin/industries')
    if (res.code === 0 || res.code === 200) {
      industries.value = res.data?.list || res.data || []
    }
  } catch (error) {
    console.error('获取行业列表失败', error)
  } finally {
    loading.value = false
  }
}

const openAdd = () => {
  editingId.value = null
  dialogTitle.value = '新增行业'
  form.value = { code: '', name: '', description: '', icon: '💻', is_active: true }
  dialogVisible.value = true
}

const openEdit = (item: any) => {
  editingId.value = item.id
  dialogTitle.value = '编辑行业'
  form.value = {
    code: item.code || '',
    name: item.name || '',
    description: item.description || '',
    icon: item.icon || '💻',
    is_active: item.is_active !== false,
  }
  dialogVisible.value = true
}

const save = async () => {
  if (!form.value.code.trim() || !form.value.name.trim()) {
    ElMessage.warning('请填写行业编码和名称')
    return
  }
  try {
    if (editingId.value) {
      await api.put(`/api/admin/industries/${editingId.value}`, form.value as any)
      ElMessage.success('行业更新成功')
    } else {
      await api.post('/api/admin/industries', form.value as any)
      ElMessage.success('行业创建成功')
    }
    dialogVisible.value = false
    fetchIndustries()
  } catch (error) {
    ElMessage.error('保存行业失败')
  }
}

const toggleActive = async (item: any) => {
  try {
    await api.put(`/api/admin/industries/${item.id}`, { ...item, is_active: !item.is_active } as any)
    ElMessage.success(item.is_active ? '已禁用' : '已启用')
    fetchIndustries()
  } catch (error) {
    ElMessage.error('操作失败')
  }
}

onMounted(() => {
  fetchIndustries()
})
</script>

<template>
  <div class="space-y-6">
    <div class="flex items-center justify-between">
      <h1 class="text-2xl font-bold text-secondary-900">行业管理</h1>
      <el-button type="primary" :icon="Plus" @click="openAdd">新增行业</el-button>
    </div>

    <!-- MVP提示 -->
    <el-alert title="当前MVP阶段仅Go语言行业处于激活状态，后续将陆续开放更多行业" type="info" :closable="true" show-icon />

    <!-- 行业卡片网格 -->
    <div v-loading="loading" class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
      <div
        v-for="item in industries"
        :key="item.id"
        class="bg-white rounded-lg shadow-sm border border-secondary-200 p-6 hover:shadow-md transition-shadow"
      >
        <div class="flex items-start justify-between mb-4">
          <div class="flex items-center gap-3">
            <span class="text-3xl">{{ item.icon || '💻' }}</span>
            <div>
              <h3 class="font-semibold text-secondary-900">{{ item.name }}</h3>
              <span class="text-xs text-secondary-400 font-mono">{{ item.code }}</span>
            </div>
          </div>
          <el-tag :type="item.is_active ? 'success' : 'info'" size="small">
            {{ item.is_active ? '激活' : '禁用' }}
          </el-tag>
        </div>
        <p class="text-sm text-secondary-600 mb-4 line-clamp-2">{{ item.description || '暂无描述' }}</p>
        <div class="flex items-center gap-2 pt-4 border-t border-secondary-100">
          <el-button type="primary" link size="small" :icon="Edit" @click="openEdit(item)">编辑</el-button>
          <el-switch
            :model-value="item.is_active"
            size="small"
            active-text="启用"
            inactive-text="禁用"
            @change="toggleActive(item)"
          />
        </div>
      </div>
    </div>
    <el-empty v-if="!loading && industries.length === 0" description="暂无行业数据" />

    <!-- 新增/编辑弹窗 -->
    <el-dialog v-model="dialogVisible" :title="dialogTitle" width="500px" :close-on-click-modal="false">
      <el-form :model="form" label-width="80px">
        <el-form-item label="编码">
          <el-input v-model="form.code" placeholder="如: golang, java" :disabled="!!editingId" />
        </el-form-item>
        <el-form-item label="名称">
          <el-input v-model="form.name" placeholder="如: Go语言" />
        </el-form-item>
        <el-form-item label="描述">
          <el-input v-model="form.description" type="textarea" :rows="3" placeholder="行业描述" />
        </el-form-item>
        <el-form-item label="图标">
          <div class="flex flex-wrap gap-2">
            <span
              v-for="ico in iconOptions"
              :key="ico"
              class="text-2xl cursor-pointer p-2 rounded-lg transition-colors"
              :class="form.icon === ico ? 'bg-primary-100 ring-2 ring-primary-500' : 'hover:bg-secondary-100'"
              @click="form.icon = ico"
            >{{ ico }}</span>
          </div>
        </el-form-item>
        <el-form-item label="状态">
          <el-switch v-model="form.is_active" active-text="激活" inactive-text="禁用" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="save">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>
