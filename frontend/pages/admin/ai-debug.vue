<script setup lang="ts">
import { ElMessage } from 'element-plus'

definePageMeta({
  title: 'AI调试',
  layout: 'admin',
  middleware: ['admin'],
})

interface DebugResult {
  trace_id: string
  scene: string
  prompt_source: string
  selected_prompt_id?: number
  selected_prompt_name?: string
  rendered_prompt: string
  runtime_config: Record<string, string>
  scene_config: Record<string, string>
  request_messages?: Array<{ role: string; content: string }>
  provider: string
  model: string
  latency_ms: number
  model_output?: string
  model_error?: string
}

interface AICallLog {
  id: number
  trace_id: string
  source: string
  scene: string
  industry_id?: number
  prompt_source: string
  selected_prompt_id?: number
  selected_prompt_name?: string
  rendered_prompt: string
  request_messages: string
  runtime_config: string
  scene_config: string
  provider: string
  model: string
  user_input: string
  model_output: string
  model_error: string
  latency_ms: number
  is_success: boolean
  created_at: string
}

const api = useApi()
const loading = ref(false)
const running = ref(false)
const historyLoading = ref(false)
const industries = ref<Array<{ id: number; name: string }>>([])
const result = ref<DebugResult | null>(null)
const history = ref<AICallLog[]>([])
const historyTotal = ref(0)
const detailVisible = ref(false)
const selectedLog = ref<AICallLog | null>(null)

const sceneOptions = [
  { label: '面试', value: 'interview' },
  { label: '学习计划', value: 'plan' },
  { label: '陪伴聊天', value: 'companion' },
  { label: '刷题分析', value: 'quiz' },
]

const sourceOptions = [
  { label: '全部来源', value: '' },
  { label: '管理端调试', value: 'admin_debug' },
  { label: '面试运行时', value: 'interview_runtime' },
  { label: '学习计划运行时', value: 'plan_runtime' },
  { label: 'Quiz 运行时', value: 'quiz_runtime' },
]

const presetVariables: Record<string, Record<string, string>> = {
  interview: {
    industry_code: 'go',
    difficulty: 'medium',
    topics: '并发,网络,数据库',
    question_count: '5',
  },
  plan: {
    industry_code: 'go',
    level: 'intermediate',
    daily_study_time: '90',
    duration_days: '14',
    goal_description: '准备后端面试',
  },
  companion: {
    user_emotion: 'tired',
    latest_user_message: '今天学得有点累',
  },
  quiz: {
    language: 'go',
    question: '请分析一个并发安全相关的答案',
  },
}

const form = ref({
  scene: 'quiz',
  industry_id: undefined as number | undefined,
  template_id: undefined as number | undefined,
  run_model: true,
  user_input: '',
  template_content: '',
  variables_text: JSON.stringify(presetVariables.quiz, null, 2),
})

const historyFilters = ref({
  page: 1,
  page_size: 10,
  source: '',
  scene: '',
  status: '',
  trace_id: '',
})

// parseVariables 将 JSON 文本转换为变量映射。
const parseVariables = (): Record<string, string> => {
  const raw = form.value.variables_text.trim()
  if (!raw) {
    return {}
  }

  const parsed = JSON.parse(raw) as Record<string, unknown>
  const resultMap: Record<string, string> = {}
  Object.entries(parsed).forEach(([key, value]) => {
    if (value === null || value === undefined) {
      return
    }
    resultMap[key] = String(value)
  })
  return resultMap
}

// formatJSON 将对象格式化为可读 JSON。
const formatJSON = (value: unknown): string => {
  return JSON.stringify(value ?? {}, null, 2)
}

// tryFormatJSON 将 JSON 字符串尽量格式化展示。
const tryFormatJSON = (value: string): string => {
  if (!value.trim()) {
    return ''
  }

  try {
    return JSON.stringify(JSON.parse(value), null, 2)
  } catch {
    return value
  }
}

// formatDateTime 将时间字符串格式化为本地时间。
const formatDateTime = (value: string): string => {
  if (!value) {
    return '-'
  }

  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return value
  }
  return date.toLocaleString('zh-CN', { hour12: false })
}

// applyPresetVariables 根据场景填充一组默认变量。
const applyPresetVariables = () => {
  const preset = presetVariables[form.value.scene] || {}
  form.value.variables_text = JSON.stringify(preset, null, 2)
}

// fetchIndustries 加载行业下拉选项。
const fetchIndustries = async () => {
  loading.value = true
  try {
    const res = await api.get<Array<{ id: number; name: string }>>('/api/admin/industries')
    if (res.code === 0 || res.code === 200) {
      industries.value = res.data || []
    }
  } catch (error) {
    console.error('获取行业列表失败', error)
  } finally {
    loading.value = false
  }
}

// fetchHistory 加载 AI 调试历史列表。
const fetchHistory = async () => {
  historyLoading.value = true
  try {
    const res = await api.get<any>('/api/admin/ai-call-logs', {
      page: historyFilters.value.page,
      page_size: historyFilters.value.page_size,
      source: historyFilters.value.source || undefined,
      scene: historyFilters.value.scene || undefined,
      status: historyFilters.value.status || undefined,
      trace_id: historyFilters.value.trace_id || undefined,
    })

    if (res.code === 0 || res.code === 200) {
      history.value = res.data?.list || []
      historyTotal.value = res.data?.total || 0
    }
  } catch (error) {
    console.error('获取 AI 调试历史失败', error)
  } finally {
    historyLoading.value = false
  }
}

// runDebug 执行一次后台 AI 调试请求。
const runDebug = async () => {
  let variables: Record<string, string> = {}
  try {
    variables = parseVariables()
  } catch (error) {
    ElMessage.error('变量 JSON 格式不正确')
    return
  }

  running.value = true
  result.value = null
  try {
    const res = await api.post<DebugResult>('/api/admin/prompts/test-render', {
      scene: form.value.scene,
      industry_id: form.value.industry_id ?? null,
      template_id: form.value.template_id ?? null,
      template_content: form.value.template_content.trim(),
      variables,
      run_model: form.value.run_model,
      user_input: form.value.user_input.trim(),
    })

    if (res.code === 0 || res.code === 200) {
      result.value = res.data
      ElMessage.success('调试执行完成')
      historyFilters.value.page = 1
      fetchHistory()
    }
  } catch (error) {
    ElMessage.error('调试执行失败')
  } finally {
    running.value = false
  }
}

// resetResult 清空当前调试结果。
const resetResult = () => {
  result.value = null
}

// viewLogDetail 打开日志详情弹窗。
const viewLogDetail = (log: AICallLog) => {
  selectedLog.value = log
  detailVisible.value = true
}

// resetHistoryFilters 重置历史筛选条件。
const resetHistoryFilters = () => {
  historyFilters.value = {
    page: 1,
    page_size: 10,
    source: '',
    scene: '',
    status: '',
    trace_id: '',
  }
  fetchHistory()
}

// handleSceneChange 切换场景时同步刷新变量示例和结果。
const handleSceneChange = () => {
  applyPresetVariables()
  resetResult()
}

onMounted(() => {
  fetchIndustries()
  fetchHistory()
})
</script>

<template>
  <div class="space-y-6" v-loading="loading">
    <div class="flex items-center justify-between">
      <h1 class="text-2xl font-bold text-secondary-900">AI调试</h1>
      <div class="flex items-center gap-3">
        <el-button @click="applyPresetVariables">填充示例变量</el-button>
        <el-button type="primary" :loading="running" @click="runDebug">执行调试</el-button>
      </div>
    </div>

    <div class="grid grid-cols-1 xl:grid-cols-5 gap-6">
      <div class="xl:col-span-2 bg-white rounded-lg shadow-sm border border-secondary-200 p-6">
        <h2 class="text-lg font-semibold text-secondary-900 mb-4">调试参数</h2>

        <el-form label-position="top">
          <el-form-item label="场景">
            <el-select v-model="form.scene" class="w-full" @change="handleSceneChange">
              <el-option v-for="item in sceneOptions" :key="item.value" :label="item.label" :value="item.value" />
            </el-select>
          </el-form-item>

          <el-form-item label="行业">
            <el-select v-model="form.industry_id" clearable class="w-full" placeholder="留空表示通用">
              <el-option v-for="item in industries" :key="item.id" :label="item.name" :value="item.id" />
            </el-select>
          </el-form-item>

          <el-form-item label="指定模板 ID">
            <el-input-number v-model="form.template_id" :min="1" class="w-full" placeholder="留空时按运行时规则选模板" />
          </el-form-item>

          <el-form-item label="自定义模板内容">
            <el-input
              v-model="form.template_content"
              type="textarea"
              :rows="8"
              placeholder="留空时按当前激活模板解析；填写后会直接使用这里的模板内容。"
            />
          </el-form-item>

          <el-form-item label="变量 JSON">
            <el-input
              v-model="form.variables_text"
              type="textarea"
              :rows="10"
              placeholder='{"industry_code":"go","difficulty":"medium"}'
            />
          </el-form-item>

          <el-form-item label="试跑用户输入">
            <el-input
              v-model="form.user_input"
              type="textarea"
              :rows="4"
              placeholder="留空时系统会按场景使用默认调试输入。"
            />
          </el-form-item>

          <el-form-item label="执行模型试跑">
            <el-switch v-model="form.run_model" inline-prompt active-text="开" inactive-text="关" />
          </el-form-item>
        </el-form>
      </div>

      <div class="xl:col-span-3 space-y-6">
        <div class="bg-white rounded-lg shadow-sm border border-secondary-200 p-6">
          <div class="flex items-center justify-between mb-4">
            <h2 class="text-lg font-semibold text-secondary-900">调试结果</h2>
            <el-button text @click="resetResult">清空结果</el-button>
          </div>

          <el-empty v-if="!result" description="还没有调试结果" />

          <div v-else class="space-y-6">
            <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
              <div class="rounded-lg bg-secondary-50 p-4">
                <div class="text-xs text-secondary-500 mb-1">Trace ID</div>
                <div class="font-mono text-sm text-secondary-800 break-all">{{ result.trace_id }}</div>
              </div>
              <div class="rounded-lg bg-secondary-50 p-4">
                <div class="text-xs text-secondary-500 mb-1">Prompt 来源</div>
                <div class="text-sm text-secondary-800">{{ result.prompt_source }}</div>
              </div>
              <div class="rounded-lg bg-secondary-50 p-4">
                <div class="text-xs text-secondary-500 mb-1">Provider / 模型</div>
                <div class="text-sm text-secondary-800">{{ result.provider }} / {{ result.model }}</div>
              </div>
              <div class="rounded-lg bg-secondary-50 p-4">
                <div class="text-xs text-secondary-500 mb-1">耗时</div>
                <div class="text-sm text-secondary-800">{{ result.latency_ms }} ms</div>
              </div>
            </div>

            <div v-if="result.selected_prompt_name || result.selected_prompt_id" class="rounded-lg border border-secondary-200 p-4">
              <div class="text-sm font-medium text-secondary-900 mb-2">命中的模板</div>
              <div class="text-sm text-secondary-700">
                {{ result.selected_prompt_name || '未命名模板' }}
                <span v-if="result.selected_prompt_id" class="text-secondary-400">#{{ result.selected_prompt_id }}</span>
              </div>
            </div>

            <div class="rounded-lg border border-secondary-200 p-4">
              <div class="text-sm font-medium text-secondary-900 mb-2">渲染后的 Prompt</div>
              <pre class="text-sm text-secondary-700 whitespace-pre-wrap break-words">{{ result.rendered_prompt }}</pre>
            </div>

            <div v-if="result.request_messages?.length" class="rounded-lg border border-secondary-200 p-4">
              <div class="text-sm font-medium text-secondary-900 mb-3">请求消息</div>
              <div class="space-y-3">
                <div v-for="(message, index) in result.request_messages" :key="index" class="rounded-lg bg-secondary-50 p-3">
                  <div class="text-xs uppercase tracking-wide text-secondary-500 mb-1">{{ message.role }}</div>
                  <pre class="text-sm text-secondary-700 whitespace-pre-wrap break-words">{{ message.content }}</pre>
                </div>
              </div>
            </div>

            <div class="rounded-lg border border-secondary-200 p-4">
              <div class="text-sm font-medium text-secondary-900 mb-2">模型输出</div>
              <pre class="text-sm whitespace-pre-wrap break-words" :class="result.model_error ? 'text-red-600' : 'text-secondary-700'">{{ result.model_error || result.model_output || '未执行模型试跑' }}</pre>
            </div>

            <div class="grid grid-cols-1 md:grid-cols-2 gap-6">
              <div class="rounded-lg border border-secondary-200 p-4">
                <div class="text-sm font-medium text-secondary-900 mb-2">生效运行时配置</div>
                <pre class="text-xs text-secondary-700 whitespace-pre-wrap break-words">{{ formatJSON(result.runtime_config) }}</pre>
              </div>
              <div class="rounded-lg border border-secondary-200 p-4">
                <div class="text-sm font-medium text-secondary-900 mb-2">场景配置</div>
                <pre class="text-xs text-secondary-700 whitespace-pre-wrap break-words">{{ formatJSON(result.scene_config) }}</pre>
              </div>
            </div>
          </div>
        </div>

        <div class="bg-white rounded-lg shadow-sm border border-secondary-200 p-6" v-loading="historyLoading">
          <div class="flex items-center justify-between mb-4">
            <h2 class="text-lg font-semibold text-secondary-900">调试历史</h2>
            <div class="flex items-center gap-2">
              <el-button @click="resetHistoryFilters">重置筛选</el-button>
              <el-button @click="fetchHistory">刷新</el-button>
            </div>
          </div>

          <div class="grid grid-cols-1 md:grid-cols-5 gap-4 mb-4">
            <el-select v-model="historyFilters.scene" clearable placeholder="全部场景" @change="historyFilters.page = 1; fetchHistory()">
              <el-option v-for="item in sceneOptions" :key="item.value" :label="item.label" :value="item.value" />
            </el-select>

            <el-select v-model="historyFilters.status" clearable placeholder="全部状态" @change="historyFilters.page = 1; fetchHistory()">
              <el-option label="成功" value="success" />
              <el-option label="失败" value="failed" />
            </el-select>

            <el-input
              v-model="historyFilters.trace_id"
              placeholder="按 Trace ID 筛选"
              clearable
              @keyup.enter="historyFilters.page = 1; fetchHistory()"
              @clear="historyFilters.page = 1; fetchHistory()"
            />

            <el-button type="primary" @click="historyFilters.page = 1; fetchHistory()">查询</el-button>
          </div>

          <div class="mb-4">
            <el-select v-model="historyFilters.source" clearable placeholder="选择来源" class="w-full md:w-64" @change="historyFilters.page = 1; fetchHistory()">
              <el-option v-for="item in sourceOptions" :key="item.value || 'all'" :label="item.label" :value="item.value" />
            </el-select>
          </div>

          <el-table :data="history" border>
            <el-table-column prop="created_at" label="时间" min-width="170">
              <template #default="{ row }">
                {{ formatDateTime(row.created_at) }}
              </template>
            </el-table-column>
            <el-table-column prop="scene" label="场景" width="120" />
            <el-table-column prop="source" label="来源" min-width="140" show-overflow-tooltip />
            <el-table-column prop="trace_id" label="Trace ID" min-width="220" show-overflow-tooltip />
            <el-table-column prop="provider" label="Provider" width="100" />
            <el-table-column prop="model" label="模型" min-width="140" show-overflow-tooltip />
            <el-table-column prop="prompt_source" label="Prompt来源" width="140" />
            <el-table-column prop="latency_ms" label="耗时(ms)" width="100" />
            <el-table-column label="状态" width="90">
              <template #default="{ row }">
                <el-tag :type="row.is_success ? 'success' : 'danger'">{{ row.is_success ? '成功' : '失败' }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="错误摘要" min-width="200" show-overflow-tooltip>
              <template #default="{ row }">
                {{ row.model_error || '-' }}
              </template>
            </el-table-column>
            <el-table-column label="操作" width="100" fixed="right">
              <template #default="{ row }">
                <el-button link type="primary" @click="viewLogDetail(row)">查看</el-button>
              </template>
            </el-table-column>
          </el-table>

          <div class="flex justify-end mt-4">
            <el-pagination
              background
              layout="total, prev, pager, next"
              :total="historyTotal"
              :current-page="historyFilters.page"
              :page-size="historyFilters.page_size"
              @current-change="(page:number) => { historyFilters.page = page; fetchHistory() }"
            />
          </div>
        </div>
      </div>
    </div>

    <el-dialog v-model="detailVisible" title="日志详情" width="960px">
      <div v-if="selectedLog" class="space-y-4">
        <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
          <div class="rounded-lg bg-secondary-50 p-4">
            <div class="text-xs text-secondary-500 mb-1">Trace ID</div>
            <div class="font-mono text-sm text-secondary-800 break-all">{{ selectedLog.trace_id }}</div>
          </div>
          <div class="rounded-lg bg-secondary-50 p-4">
            <div class="text-xs text-secondary-500 mb-1">状态</div>
            <div class="text-sm text-secondary-800">{{ selectedLog.is_success ? '成功' : '失败' }}</div>
          </div>
        </div>

        <div class="rounded-lg border border-secondary-200 p-4">
          <div class="text-sm font-medium text-secondary-900 mb-2">渲染后的 Prompt</div>
          <pre class="text-sm text-secondary-700 whitespace-pre-wrap break-words">{{ selectedLog.rendered_prompt || '-' }}</pre>
        </div>

        <div class="rounded-lg border border-secondary-200 p-4">
          <div class="text-sm font-medium text-secondary-900 mb-2">请求消息</div>
          <pre class="text-sm text-secondary-700 whitespace-pre-wrap break-words">{{ tryFormatJSON(selectedLog.request_messages) || '-' }}</pre>
        </div>

        <div class="rounded-lg border border-secondary-200 p-4">
          <div class="text-sm font-medium text-secondary-900 mb-2">模型输出 / 错误</div>
          <pre class="text-sm whitespace-pre-wrap break-words" :class="selectedLog.model_error ? 'text-red-600' : 'text-secondary-700'">{{ selectedLog.model_error || selectedLog.model_output || '-' }}</pre>
        </div>

        <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
          <div class="rounded-lg border border-secondary-200 p-4">
            <div class="text-sm font-medium text-secondary-900 mb-2">运行时配置</div>
            <pre class="text-xs text-secondary-700 whitespace-pre-wrap break-words">{{ tryFormatJSON(selectedLog.runtime_config) || '-' }}</pre>
          </div>
          <div class="rounded-lg border border-secondary-200 p-4">
            <div class="text-sm font-medium text-secondary-900 mb-2">场景配置</div>
            <pre class="text-xs text-secondary-700 whitespace-pre-wrap break-words">{{ tryFormatJSON(selectedLog.scene_config) || '-' }}</pre>
          </div>
        </div>
      </div>
    </el-dialog>
  </div>
</template>
