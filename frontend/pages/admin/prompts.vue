<script setup lang="ts">
import { Delete, Edit, Plus } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'

definePageMeta({
  title: 'Prompt妯℃澘',
  layout: 'admin',
  middleware: ['admin']
})

const api = useApi()
const loading = ref(false)
const industries = ref<Array<{ id: number; name: string }>>([])

const scenes = [
  { key: 'interview', label: '闈㈣瘯' },
  { key: 'companion', label: '闄即' },
  { key: 'quiz', label: '鍒烽' },
  { key: 'plan', label: '瀛︿範璁″垝' },
]

const sceneVariables: Record<string, string[]> = {
  interview: ['username', 'industry', 'difficulty', 'topics', 'question_count'],
  companion: ['username', 'topic', 'mood', 'context'],
  quiz: ['question_type', 'difficulty', 'question_content', 'correct_answer'],
  plan: ['username', 'level', 'available_time', 'goal'],
}

const activeScene = ref('interview')
const prompts = ref<any[]>([])

const dialogVisible = ref(false)
const dialogTitle = ref('鏂板妯℃澘')
const editingId = ref<number | null>(null)
const form = ref({
  name: '',
  scene: 'interview',
  template_content: '',
  variables: '',
  industry_id: undefined as number | undefined,
  is_active: true,
})

const fetchIndustries = async () => {
  try {
    const res = await api.get<any>('/api/admin/industries')
    if (res.code === 0 || res.code === 200) {
      industries.value = res.data || []
    }
  } catch (error) {
    console.error('鑾峰彇琛屼笟鍒楄〃澶辫触', error)
  }
}

const fetchPrompts = async () => {
  loading.value = true
  try {
    const res = await api.get<any>('/api/admin/prompts', { scene: activeScene.value })
    if (res.code === 0 || res.code === 200) {
      prompts.value = res.data || []
    }
  } catch (error) {
    console.error('鑾峰彇妯℃澘澶辫触', error)
  } finally {
    loading.value = false
  }
}

watch(activeScene, () => {
  fetchPrompts()
})

const resetForm = () => {
  editingId.value = null
  form.value = {
    name: '',
    scene: activeScene.value,
    template_content: '',
    variables: '',
    industry_id: undefined,
    is_active: true,
  }
}

const openAdd = () => {
  dialogTitle.value = '鏂板妯℃澘'
  resetForm()
  dialogVisible.value = true
}

const openEdit = (item: any) => {
  editingId.value = item.id
  dialogTitle.value = '缂栬緫妯℃澘'
  form.value = {
    name: item.name || '',
    scene: item.scene || activeScene.value,
    template_content: item.template_content || '',
    variables: item.variables || '',
    industry_id: item.industry_id || undefined,
    is_active: item.is_active !== false,
  }
  dialogVisible.value = true
}

const insertVariable = (varName: string) => {
  form.value.template_content += `{{${varName}}}`
}

const renderVariableChip = (varName: string) => {
  return `{{${varName}}}`
}

const save = async () => {
  if (!form.value.name.trim()) {
    ElMessage.warning('请输入模板名称')
    return
  }
  if (!form.value.template_content.trim()) {
    ElMessage.warning('请输入模板内容')
    return
  }

  const payload = {
    name: form.value.name.trim(),
    scene: form.value.scene,
    template_content: form.value.template_content,
    variables: form.value.variables,
    industry_id: form.value.industry_id ?? null,
    is_active: form.value.is_active,
  }

  try {
    if (editingId.value) {
      await api.put(`/api/admin/prompts/${editingId.value}`, payload as any)
      ElMessage.success('妯℃澘鏇存柊鎴愬姛')
    } else {
      await api.post('/api/admin/prompts', payload as any)
      ElMessage.success('妯℃澘鍒涘缓鎴愬姛')
    }
    dialogVisible.value = false
    fetchPrompts()
  } catch (error) {
    ElMessage.error('淇濆瓨妯℃澘澶辫触')
  }
}

const handleDelete = async (item: any) => {
  try {
    await ElMessageBox.confirm(`确认删除模板：${item.name}？`, '确认删除', {
      confirmButtonText: '纭畾',
      cancelButtonText: '鍙栨秷',
      type: 'warning',
    })
    await api.delete(`/api/admin/prompts/${item.id}`)
    ElMessage.success('鍒犻櫎鎴愬姛')
    fetchPrompts()
  } catch (error: any) {
    if (error !== 'cancel') {
      ElMessage.error('鍒犻櫎澶辫触')
    }
  }
}

const toggleActive = async (item: any) => {
  try {
    await api.put(`/api/admin/prompts/${item.id}`, {
      name: item.name,
      scene: item.scene,
      template_content: item.template_content,
      variables: item.variables,
      industry_id: item.industry_id ?? null,
      is_active: !item.is_active,
    } as any)
    ElMessage.success(item.is_active ? '已停用' : '已启用')
    fetchPrompts()
  } catch (error) {
    ElMessage.error('鎿嶄綔澶辫触')
  }
}

onMounted(() => {
  fetchIndustries()
  fetchPrompts()
})
</script>

<template>
  <div class="space-y-6">
    <div class="flex items-center justify-between">
      <h1 class="text-2xl font-bold text-secondary-900">Prompt妯℃澘绠＄悊</h1>
      <el-button type="primary" :icon="Plus" @click="openAdd">鏂板妯℃澘</el-button>
    </div>

    <div class="bg-white rounded-lg shadow-sm border border-secondary-200">
      <div class="flex min-h-[500px]">
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

        <div class="flex-1 p-6" v-loading="loading">
          <div v-if="prompts.length > 0" class="space-y-4">
            <div
              v-for="item in prompts"
              :key="item.id"
              class="border border-secondary-200 rounded-lg p-4 hover:border-secondary-300 transition-colors"
            >
              <div class="flex items-center justify-between gap-4 mb-2">
                <div>
                  <h3 class="font-medium text-secondary-900">{{ item.name }}</h3>
                  <p class="text-xs text-secondary-400 mt-1">
                    {{ item.industry?.name || '閫氱敤妯℃澘' }}
                  </p>
                </div>
                <div class="flex items-center gap-3">
                  <el-switch :model-value="item.is_active" size="small" @change="toggleActive(item)" />
                  <el-button type="primary" link size="small" :icon="Edit" @click="openEdit(item)">缂栬緫</el-button>
                  <el-button type="danger" link size="small" :icon="Delete" @click="handleDelete(item)">鍒犻櫎</el-button>
                </div>
              </div>

              <p class="text-sm text-secondary-500 whitespace-pre-wrap font-mono bg-secondary-50 rounded p-3 line-clamp-6">
                {{ item.template_content }}
              </p>
            </div>
          </div>
          <el-empty v-else description="褰撳墠鍦烘櫙鏆傛棤妯℃澘" />
        </div>
      </div>
    </div>

    <el-dialog v-model="dialogVisible" :title="dialogTitle" width="860px" :close-on-click-modal="false">
      <el-form :model="form" label-position="top">
        <div class="grid grid-cols-1 md:grid-cols-2 gap-6">
          <el-form-item label="模板名称">
            <el-input v-model="form.name" placeholder="请输入模板名称" />
          </el-form-item>

          <el-form-item label="閫傜敤琛屼笟">
            <el-select v-model="form.industry_id" clearable class="w-full" placeholder="鐣欑┖琛ㄧず閫氱敤妯℃澘">
              <el-option v-for="item in industries" :key="item.id" :label="item.name" :value="item.id" />
            </el-select>
          </el-form-item>
        </div>

        <el-form-item label="模板内容">
          <el-input
            v-model="form.template_content"
            type="textarea"
            :rows="15"
            placeholder="请输入 Prompt 模板内容，使用 {{variable}} 插入变量"
            style="font-family: 'Courier New', monospace"
          />
        </el-form-item>

        <el-form-item label="变量定义(JSON)">
          <el-input
            v-model="form.variables"
            type="textarea"
            :rows="4"
            placeholder='{"username":"用户名","difficulty":"难度"}'
            style="font-family: 'Courier New', monospace"
          />
        </el-form-item>

        <el-form-item label="蹇嵎鎻掑叆鍙橀噺">
          <div class="flex flex-wrap gap-2">
            <el-button
              v-for="v in sceneVariables[form.scene] || []"
              :key="v"
              size="small"
              plain
              @click="insertVariable(v)"
            >
              {{ renderVariableChip(v) }}
            </el-button>
          </div>
        </el-form-item>

        <el-form-item label="状态">
          <el-switch v-model="form.is_active" active-text="鍚敤" inactive-text="绂佺敤" />
        </el-form-item>
      </el-form>

      <template #footer>
        <el-button @click="dialogVisible = false">鍙栨秷</el-button>
        <el-button type="primary" @click="save">淇濆瓨</el-button>
      </template>
    </el-dialog>
  </div>
</template>
