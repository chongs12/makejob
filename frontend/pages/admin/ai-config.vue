<script setup lang="ts">
import { Setting } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'

definePageMeta({
  title: 'AI配置',
  layout: 'admin',
  middleware: ['admin']
})

const api = useApi()
const loading = ref(false)
const saving = ref(false)

const configs = ref<Record<string, any>>({
  ai_provider: 'mock',
  ai_model: 'gpt-4o-mini',
  ai_base_url: '',
  ai_api_key: '',
  ai_temperature: 0.7,
  ai_top_p: 0.9,
  ai_max_tokens: 2048,
  ai_timeout_seconds: 30,
  ai_enable_stream: false,
  ai_fallback_provider: 'mock',
  ai_scene_interview_model: '',
  ai_scene_plan_model: '',
  ai_scene_companion_model: '',
  ai_scene_quiz_model: '',
})

const providerOptions = [
  { label: 'Mock', value: 'mock' },
  { label: 'Eino', value: 'eino' },
]

const modelOptions = [
  { label: 'GPT-4o', value: 'gpt-4o' },
  { label: 'GPT-4o Mini', value: 'gpt-4o-mini' },
  { label: 'Doubao', value: 'doubao' },
  { label: 'Qwen', value: 'qwen' },
]

const paramDescriptions: Record<string, string> = {
  ai_provider: 'AI 运行时提供方，当前支持 mock 和 eino。',
  ai_model: '默认模型名。后续真实 Eino 接入时会按这个值选择上游模型。',
  ai_base_url: 'AI 上游服务地址。留空时使用服务端默认配置。',
  ai_api_key: 'AI 上游密钥，仅在后台保存，不应泄漏到前端。',
  ai_temperature: '采样温度，值越高输出越发散。',
  ai_top_p: 'Top-p 采样参数。',
  ai_max_tokens: '单次请求允许的最大输出 token 数。',
  ai_timeout_seconds: '单次 AI 请求超时时间，单位为秒。',
  ai_enable_stream: '是否启用流式输出预留开关。',
  ai_fallback_provider: '主 provider 失败后的回退 provider。',
  ai_scene_interview_model: 'interview 场景专用模型，留空时使用默认模型。',
  ai_scene_plan_model: 'plan 场景专用模型，留空时使用默认模型。',
  ai_scene_companion_model: 'companion 场景专用模型，留空时使用默认模型。',
  ai_scene_quiz_model: 'quiz 场景专用模型，留空时使用默认模型。',
}

const normalizeNumber = (value: unknown, fallback: number) => {
  const parsed = Number(value)
  return Number.isFinite(parsed) ? parsed : fallback
}

const normalizeBoolean = (value: unknown, fallback: boolean) => {
  if (typeof value === 'boolean') return value
  if (typeof value === 'string') {
    if (value === 'true') return true
    if (value === 'false') return false
  }
  return fallback
}

const applyConfigs = (payload: Record<string, any>) => {
  configs.value.ai_provider = payload.ai_provider || configs.value.ai_provider
  configs.value.ai_model = payload.ai_model || configs.value.ai_model
  configs.value.ai_base_url = payload.ai_base_url || ''
  configs.value.ai_api_key = payload.ai_api_key || ''
  configs.value.ai_temperature = normalizeNumber(payload.ai_temperature, 0.7)
  configs.value.ai_top_p = normalizeNumber(payload.ai_top_p, 0.9)
  configs.value.ai_max_tokens = normalizeNumber(payload.ai_max_tokens, 2048)
  configs.value.ai_timeout_seconds = normalizeNumber(payload.ai_timeout_seconds, 30)
  configs.value.ai_enable_stream = normalizeBoolean(payload.ai_enable_stream, false)
  configs.value.ai_fallback_provider = payload.ai_fallback_provider || 'mock'
  configs.value.ai_scene_interview_model = payload.ai_scene_interview_model || ''
  configs.value.ai_scene_plan_model = payload.ai_scene_plan_model || ''
  configs.value.ai_scene_companion_model = payload.ai_scene_companion_model || ''
  configs.value.ai_scene_quiz_model = payload.ai_scene_quiz_model || ''
}

const fetchConfigs = async () => {
  loading.value = true
  try {
    const res = await api.get<any>('/api/admin/ai-configs')
    if (res.code === 0 || res.code === 200) {
      applyConfigs(res.data?.configs || {})
    }
  } catch (error) {
    console.error('获取 AI 配置失败', error)
  } finally {
    loading.value = false
  }
}

const saveConfigs = async () => {
  saving.value = true
  try {
    await api.put('/api/admin/ai-configs', {
      configs: {
        ai_provider: configs.value.ai_provider,
        ai_model: configs.value.ai_model,
        ai_base_url: configs.value.ai_base_url,
        ai_api_key: configs.value.ai_api_key,
        ai_temperature: String(configs.value.ai_temperature),
        ai_top_p: String(configs.value.ai_top_p),
        ai_max_tokens: String(configs.value.ai_max_tokens),
        ai_timeout_seconds: String(configs.value.ai_timeout_seconds),
        ai_enable_stream: String(Boolean(configs.value.ai_enable_stream)),
        ai_fallback_provider: configs.value.ai_fallback_provider,
        ai_scene_interview_model: configs.value.ai_scene_interview_model,
        ai_scene_plan_model: configs.value.ai_scene_plan_model,
        ai_scene_companion_model: configs.value.ai_scene_companion_model,
        ai_scene_quiz_model: configs.value.ai_scene_quiz_model,
      }
    })
    ElMessage.success('配置保存成功')
  } catch (error) {
    ElMessage.error('保存配置失败')
  } finally {
    saving.value = false
  }
}

onMounted(() => {
  fetchConfigs()
})
</script>

<template>
  <div class="space-y-6" v-loading="loading">
    <div class="flex items-center justify-between">
      <h1 class="text-2xl font-bold text-secondary-900">AI配置</h1>
      <el-button type="primary" @click="saveConfigs" :loading="saving">保存配置</el-button>
    </div>

    <div class="grid grid-cols-1 lg:grid-cols-3 gap-6">
      <div class="lg:col-span-2 space-y-6">
        <div class="bg-white rounded-lg shadow-sm border border-secondary-200 p-6">
          <h2 class="text-lg font-semibold text-secondary-900 mb-6 flex items-center gap-2">
            <el-icon class="text-primary-500"><Setting /></el-icon>
            运行时配置
          </h2>

          <el-form label-position="top">
            <div class="grid grid-cols-1 md:grid-cols-2 gap-6">
              <el-form-item label="Provider">
                <el-select v-model="configs.ai_provider" class="w-full">
                  <el-option v-for="opt in providerOptions" :key="opt.value" :label="opt.label" :value="opt.value" />
                </el-select>
              </el-form-item>

              <el-form-item label="回退 Provider">
                <el-select v-model="configs.ai_fallback_provider" class="w-full">
                  <el-option v-for="opt in providerOptions" :key="opt.value" :label="opt.label" :value="opt.value" />
                </el-select>
              </el-form-item>
            </div>

            <el-form-item label="默认模型">
              <el-select v-model="configs.ai_model" class="w-full" filterable allow-create default-first-option>
                <el-option v-for="opt in modelOptions" :key="opt.value" :label="opt.label" :value="opt.value" />
              </el-select>
            </el-form-item>

            <el-form-item label="Base URL">
              <el-input v-model="configs.ai_base_url" placeholder="例如：https://ark.cn-beijing.volces.com/api/v3" />
            </el-form-item>

            <el-form-item label="API Key">
              <el-input
                v-model="configs.ai_api_key"
                type="password"
                show-password
                placeholder="请输入 AI 上游 API Key"
              />
            </el-form-item>

            <div class="grid grid-cols-1 md:grid-cols-2 gap-6">
              <el-form-item label="Temperature">
                <div class="flex items-center gap-4 w-full">
                  <el-slider v-model="configs.ai_temperature" :min="0" :max="2" :step="0.1" class="flex-1" />
                  <span class="text-sm font-mono text-secondary-700 w-12 text-right">{{ configs.ai_temperature }}</span>
                </div>
              </el-form-item>

              <el-form-item label="Top P">
                <div class="flex items-center gap-4 w-full">
                  <el-slider v-model="configs.ai_top_p" :min="0" :max="1" :step="0.1" class="flex-1" />
                  <span class="text-sm font-mono text-secondary-700 w-12 text-right">{{ configs.ai_top_p }}</span>
                </div>
              </el-form-item>
            </div>

            <div class="grid grid-cols-1 md:grid-cols-2 gap-6">
              <el-form-item label="Max Tokens">
                <el-input-number v-model="configs.ai_max_tokens" :min="256" :max="16384" :step="128" class="w-full" />
              </el-form-item>

              <el-form-item label="超时时间(秒)">
                <el-input-number v-model="configs.ai_timeout_seconds" :min="5" :max="120" class="w-full" />
              </el-form-item>
            </div>

            <div class="grid grid-cols-1 md:grid-cols-2 gap-6">
              <el-form-item label="Interview 模型">
                <el-input v-model="configs.ai_scene_interview_model" placeholder="留空时使用默认模型" />
              </el-form-item>

              <el-form-item label="Plan 模型">
                <el-input v-model="configs.ai_scene_plan_model" placeholder="留空时使用默认模型" />
              </el-form-item>
            </div>

            <div class="grid grid-cols-1 md:grid-cols-2 gap-6">
              <el-form-item label="Companion 模型">
                <el-input v-model="configs.ai_scene_companion_model" placeholder="留空时使用默认模型" />
              </el-form-item>

              <el-form-item label="Quiz 模型">
                <el-input v-model="configs.ai_scene_quiz_model" placeholder="留空时使用默认模型" />
              </el-form-item>
            </div>

            <div class="grid grid-cols-1 md:grid-cols-2 gap-6">
              <el-form-item label="流式输出">
                <el-switch v-model="configs.ai_enable_stream" inline-prompt active-text="开" inactive-text="关" />
              </el-form-item>
            </div>
          </el-form>
        </div>
      </div>

      <div class="space-y-4">
        <div class="bg-white rounded-lg shadow-sm border border-secondary-200 p-6">
          <h2 class="text-lg font-semibold text-secondary-900 mb-4">参数说明</h2>
          <div class="space-y-4">
            <div v-for="(desc, key) in paramDescriptions" :key="key" class="pb-3 border-b border-secondary-100 last:border-0">
              <h4 class="text-sm font-medium text-secondary-700 mb-1">{{ key }}</h4>
              <p class="text-xs text-secondary-500 leading-relaxed">{{ desc }}</p>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
