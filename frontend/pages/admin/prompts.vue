<script setup lang="ts">
/**
 * Prompt模板管理页面
 */

import { Plus, Edit, Delete, Document } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'

definePageMeta({
  title: 'Prompt模板',
  layout: 'admin',
  middleware: ['admin']
})

const api = useApi()
const loading = ref(false)

// 场景Tab
const scenes = [
  { key: 'interview', label: '面试' },
  { key: 'companion', label: '陪伴' },
  { key: 'quiz', label: '刷题' },
  { key: 'plan', label: '学习计划' },
]
const activeScene = ref('interview')
const prompts = ref<any[]>([])

// 可用变量
const sceneVariables: Record<string, string[]> = {
  interview: ['username', 'industry', 'difficulty', 'position', 'company'],
  companion: ['username', 'topic', 'mood', 'context'],
  quiz: ['username', 'industry', 'difficulty', 'category', 'question_type'],
  plan: ['username', 'industry', 'target_position', 'current_level', 'duration'],
}

// 获取模板列表
const fetchPrompts = async () => {
  loading.value = true
  try {
    const res = await api.get<any>('/api/admin/prompts', { scene: activeScene.value })
    if (res.code === 0 || res.code === 200) {
      prompts.value = res.data?.list || res.data || []
    }
  } catch (error) {
    console.error('获取模板失败', error)
  } finally {
    loading.value = false
  }
}

// 切换场景
watch(activeScene, () => {
  fetchPrompts()
})

// 弹窗
const dialogVisible = ref(false)
const dialogTitle = ref('新增模板')
const editingId = ref<number | null>(null)
const form = ref({
  name: '',
  scene: 'interview',
  content: '',
  is_active: true,
})

const openAdd = () => {
  editingId.value = null
  dialogTitle.value = '新增模板'
  form.value = { name: '', scene: activeScene.value, content: '', is_active: true }
  dialogVisible.value = true
}

const openEdit = (item: any) => {
  editingId.value = item.id
  dialogTitle.value = '编辑模板'
  form.value = {
    name: item.name || '',
    scene: item.scene || activeScene.value,
    content: item.content || '',
    is_active: item.is_active !== false,
  }
  dialogVisible.value = true
}

const insertVariable = (varName: string) => {
  form.value.content += `{{${varName}}}`
}

const save = async () => {
  if (!form.value.name.trim()) {
    ElMessage.warning('请输入模板名称')
    return
  }
  if (!form.value.content.trim()) {
    ElMessage.warning('请输入模板内容')
    return
  }
  try {
    if (editingId.value) {
      await api.put(`/api/admin/prompts/${editingId.value}`, form.value as any)
      ElMessage.success('模板更新成功')
    } else {
      await api.post('/api/admin/prompts', form.value as any)
      ElMessage.success('模板创建成功')
    }
    dialogVisible.value = false
    fetchPrompts()
  } catch (error) {
    ElMessage.error('保存模板失败')
  }
}

const handleDelete = async (item: any) => {
  try {
    await ElMessageBox.confirm(`确定删除模板「${item.name}」吗？`, '确认删除', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning',
    })
    await api.delete(`/api/admin/prompts/${item.id}`)
    ElMessage.success('删除成功')
    fetchPrompts()
  } catch (error: any) {
    if (error !== 'cancel') ElMessage.error('删除失败')
  }
}

const toggleActive = async (item: any) => {
  try {
    await api.put(`/api/admin/prompts/${item.id}`, { ...item, is_active: !item.is_active } as any)
    ElMessage.success(item.is_active ? '已禁用' : '已启用')
    fetchPrompts()
  } catch (error) {
    ElMessage.error('操作失败')
  }
}

onMounted(() => {
  fetchPrompts()
})
</script>

<template>
  <div class="space-y-6">
    <div class="flex items-center justify-between">
      <h1 class="text-2xl font-bold text-secondary-900">Prompt模板管理</h1>
      <el-button type="primary" :icon="Plus" @click="openAdd">新增模板</el-button>
    </div>

    <div class="bg-white rounded-lg shadow-sm border border-secondary-200">
      <div class="flex min-h-[500px]">
        <!-- 左侧场景Tab -->
        <div class="w-48 border-r border-secondary-200 p-4">
          <div class="space-y-1">
            <div
              v-for="scene in scenes"
              :key="scene.key"
              class="px-4 py-3 rounded-lg cursor-pointer transition-colors text-sm"
              :class="activeScene === scene.key ? 'bg-primary-50 text-primary-600 font-medium' : 'text-secondary-600 hover:bg-secondary-50'"
              @click="activeScene = scene.key"
            >
              {{ scene.label }}
            </div>
          </div>
        </div>

        <!-- 右侧内容 -->
        <div class="flex-1 p-6" v-loading="loading">
          <div v-if="prompts.length > 0" class="space-y-4">
            <div
              v-for="item in prompts"
              :key="item.id"
              class="border border-secondary-200 rounded-lg p-4 hover:border-secondary-300 transition-colors"
            >
              <div class="flex items-center justify-between mb-2">
                <h3 class="font-medium text-secondary-900">{{ item.name }}</h3>
                <div class="flex items-center gap-3">
                  <el-switch
                    :model-value="item.is_active"
                    size="small"
                    @change="toggleActive(item)"
                  />
                  <el-button type="primary" link size="small" :icon="Edit" @click="openEdit(item)">编辑</el-button>
                  <el-button type="danger" link size="small" :icon="Delete" @click="handleDelete(item)">删除</el-button>
                </div>
              </div>
              <p class="text-sm text-secondary-500 line-clamp-3 font-mono bg-secondary-50 rounded p-3">
                {{ item.content }}
              </p>
            </div>
          </div>
          <el-empty v-else description="当前场景暂无模板" />
        </div>
      </div>
    </div>

    <!-- 编辑弹窗 -->
    <el-dialog v-model="dialogVisible" :title="dialogTitle" width="800px" :close-on-click-modal="false">
      <el-form :model="form" label-position="top">
        <el-form-item label="模板名称">
          <el-input v-model="form.name" placeholder="请输入模板名称" />
        </el-form-item>
        <el-form-item label="模板内容">
          <el-input
            v-model="form.content"
            type="textarea"
            :rows="15"
            placeholder="请输入Prompt模板内容，使用 {{变量名}} 插入变量"
            style="font-family: 'Courier New', monospace"
          />
        </el-form-item>
        <el-form-item label="可用变量">
          <div class="flex flex-wrap gap-2">
            <el-button
              v-for="v in sceneVariables[form.scene] || []"
              :key="v"
              size="small"
              plain
              @click="insertVariable(v)"
            >
              {{ `\{\{${v}\}\}` }}
            </el-button>
          </div>
          <p class="text-xs text-secondary-400 mt-2">点击变量可快捷插入到模板内容中</p>
        </el-form-item>
        <el-form-item label="启用状态">
          <el-switch v-model="form.is_active" active-text="启用" inactive-text="禁用" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="save">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>
