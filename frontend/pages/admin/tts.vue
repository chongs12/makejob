<script setup lang="ts">
/**
 * TTS音色管理页面
 */

import { Plus, Edit, Delete, VideoPlay } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'

definePageMeta({
  title: 'TTS管理',
  layout: 'admin',
  middleware: ['admin']
})

const api = useApi()
const loading = ref(false)
const configs = ref<any[]>([])

// 引擎选项
const engineOptions = [
  { label: 'ElevenLabs', value: 'elevenlabs' },
  { label: 'MiniMax', value: 'minimax' },
  { label: '阿里云', value: 'aliyun' },
  { label: '讯飞', value: 'xunfei' },
]

// 场景选项
const sceneOptions = [
  { label: '面试', value: 'interview' },
  { label: '陪伴', value: 'companion' },
  { label: '通用', value: 'general' },
]

const engineTagMap: Record<string, string> = {
  elevenlabs: '',
  minimax: 'success',
  aliyun: 'warning',
  xunfei: 'danger',
}

// 获取配置列表
const fetchConfigs = async () => {
  loading.value = true
  try {
    const res = await api.get<any>('/api/admin/tts-configs')
    if (res.code === 0 || res.code === 200) {
      configs.value = res.data?.list || res.data || []
    }
  } catch (error) {
    console.error('获取TTS配置失败', error)
  } finally {
    loading.value = false
  }
}

// 弹窗
const dialogVisible = ref(false)
const dialogTitle = ref('新增配置')
const editingId = ref<number | null>(null)
const form = ref({
  name: '',
  engine: 'elevenlabs',
  voice_id: '',
  scene: 'general',
  speed: 1.0,
  pitch: 1.0,
  params_json: '{}',
  is_active: true,
})

const openAdd = () => {
  editingId.value = null
  dialogTitle.value = '新增配置'
  form.value = { name: '', engine: 'elevenlabs', voice_id: '', scene: 'general', speed: 1.0, pitch: 1.0, params_json: '{}', is_active: true }
  dialogVisible.value = true
}

const openEdit = (item: any) => {
  editingId.value = item.id
  dialogTitle.value = '编辑配置'
  form.value = {
    name: item.name || '',
    engine: item.engine || 'elevenlabs',
    voice_id: item.voice_id || '',
    scene: item.scene || 'general',
    speed: item.speed || 1.0,
    pitch: item.pitch || 1.0,
    params_json: typeof item.params_json === 'string' ? item.params_json : JSON.stringify(item.params_json || {}, null, 2),
    is_active: item.is_active !== false,
  }
  dialogVisible.value = true
}

const save = async () => {
  if (!form.value.name.trim()) {
    ElMessage.warning('请输入名称')
    return
  }
  try {
    const payload = { ...form.value } as any
    try { payload.params_json = JSON.parse(form.value.params_json) } catch { /* keep */ }

    if (editingId.value) {
      await api.put(`/api/admin/tts-configs/${editingId.value}`, payload)
      ElMessage.success('配置更新成功')
    } else {
      await api.post('/api/admin/tts-configs', payload)
      ElMessage.success('配置创建成功')
    }
    dialogVisible.value = false
    fetchConfigs()
  } catch (error) {
    ElMessage.error('保存配置失败')
  }
}

const handleDelete = async (item: any) => {
  try {
    await ElMessageBox.confirm(`确定删除配置「${item.name}」吗？`, '确认删除', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning',
    })
    await api.delete(`/api/admin/tts-configs/${item.id}`)
    ElMessage.success('删除成功')
    fetchConfigs()
  } catch (error: any) {
    if (error !== 'cancel') ElMessage.error('删除失败')
  }
}

// 试听（预留）
const handlePreview = (item: any) => {
  ElMessage.info('试听功能开发中')
}

onMounted(() => {
  fetchConfigs()
})
</script>

<template>
  <div class="space-y-6">
    <div class="flex items-center justify-between">
      <h1 class="text-2xl font-bold text-secondary-900">TTS音色管理</h1>
      <el-button type="primary" :icon="Plus" @click="openAdd">新增配置</el-button>
    </div>

    <!-- 表格 -->
    <div class="bg-white rounded-lg shadow-sm border border-secondary-200">
      <el-table :data="configs" v-loading="loading" style="width: 100%">
        <el-table-column prop="name" label="名称" min-width="150" />
        <el-table-column label="引擎" width="120">
          <template #default="{ row }">
            <el-tag :type="(engineTagMap[row.engine] as any) || 'info'" size="small">{{ row.engine }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="voice_id" label="音色ID" min-width="150" show-overflow-tooltip />
        <el-table-column label="场景" width="100">
          <template #default="{ row }">
            <span class="text-sm">{{ sceneOptions.find(s => s.value === row.scene)?.label || row.scene }}</span>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="90">
          <template #default="{ row }">
            <el-tag :type="row.is_active ? 'success' : 'info'" size="small">
              {{ row.is_active ? '启用' : '禁用' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="180" fixed="right">
          <template #default="{ row }">
            <el-button type="success" link size="small" :icon="VideoPlay" @click="handlePreview(row)">试听</el-button>
            <el-button type="primary" link size="small" :icon="Edit" @click="openEdit(row)">编辑</el-button>
            <el-button type="danger" link size="small" :icon="Delete" @click="handleDelete(row)">删除</el-button>
          </template>
        </el-table-column>
        <template #empty><el-empty description="暂无TTS配置" /></template>
      </el-table>
    </div>

    <!-- 新增/编辑弹窗 -->
    <el-dialog v-model="dialogVisible" :title="dialogTitle" width="600px" :close-on-click-modal="false">
      <el-form :model="form" label-width="90px">
        <el-form-item label="名称">
          <el-input v-model="form.name" placeholder="如: 女声温柔" />
        </el-form-item>
        <div class="grid grid-cols-2 gap-4">
          <el-form-item label="引擎">
            <el-select v-model="form.engine" class="w-full">
              <el-option v-for="opt in engineOptions" :key="opt.value" :label="opt.label" :value="opt.value" />
            </el-select>
          </el-form-item>
          <el-form-item label="场景">
            <el-select v-model="form.scene" class="w-full">
              <el-option v-for="opt in sceneOptions" :key="opt.value" :label="opt.label" :value="opt.value" />
            </el-select>
          </el-form-item>
        </div>
        <el-form-item label="音色ID">
          <el-input v-model="form.voice_id" placeholder="引擎对应的音色ID" />
        </el-form-item>
        <div class="grid grid-cols-2 gap-4">
          <el-form-item label="语速">
            <div class="flex items-center gap-3 w-full">
              <el-slider v-model="form.speed" :min="0.5" :max="2.0" :step="0.1" class="flex-1" />
              <span class="text-sm font-mono w-8">{{ form.speed }}</span>
            </div>
          </el-form-item>
          <el-form-item label="音调">
            <div class="flex items-center gap-3 w-full">
              <el-slider v-model="form.pitch" :min="0.5" :max="2.0" :step="0.1" class="flex-1" />
              <span class="text-sm font-mono w-8">{{ form.pitch }}</span>
            </div>
          </el-form-item>
        </div>
        <el-form-item label="参数JSON">
          <el-input v-model="form.params_json" type="textarea" :rows="4" placeholder="{}" style="font-family: monospace" />
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
