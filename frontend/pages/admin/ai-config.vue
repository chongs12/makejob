<script setup lang="ts">
/**
 * AI配置页面
 */

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
const showApiKey = ref(false)

// 配置表单
const configs = ref<Record<string, any>>({
  ai_model: 'gpt-4o-mini',
  api_key: '',
  temperature: 0.7,
  top_p: 0.9,
  daily_free_quiz_limit: 10,
  daily_free_interview_limit: 3,
})

// 模型选项
const modelOptions = [
  { label: 'GPT-4o', value: 'gpt-4o' },
  { label: 'GPT-4o Mini', value: 'gpt-4o-mini' },
  { label: '豆包', value: 'doubao' },
  { label: '通义千问', value: 'qwen' },
]

// 参数说明
const paramDescriptions: Record<string, string> = {
  ai_model: '选择用于生成内容的AI模型，不同模型在能力和成本上有差异',
  api_key: 'AI服务提供商的API密钥，请妥善保管',
  temperature: '控制生成内容的随机性，值越高结果越多样，越低越确定（推荐0.5-0.9）',
  top_p: '核采样参数，控制生成内容的多样性（推荐0.8-1.0）',
  daily_free_quiz_limit: '免费用户每日可刷题次数上限',
  daily_free_interview_limit: '免费用户每日可进行模拟面试次数上限',
}

const fetchConfigs = async () => {
  loading.value = true
  try {
    const res = await api.get<any>('/api/admin/ai-configs')
    if (res.code === 0 || res.code === 200) {
      const data = res.data?.configs || res.data || {}
      // 合并已有配置
      Object.keys(data).forEach(key => {
        if (key in configs.value) {
          configs.value[key] = data[key]
        }
      })
    }
  } catch (error) {
    console.error('获取AI配置失败', error)
  } finally {
    loading.value = false
  }
}

const saveConfigs = async () => {
  saving.value = true
  try {
    await api.put('/api/admin/ai-configs', { configs: configs.value })
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
      <!-- 左侧配置区 -->
      <div class="lg:col-span-2 space-y-6">
        <!-- 模型配置区 -->
        <div class="bg-white rounded-lg shadow-sm border border-secondary-200 p-6">
          <h2 class="text-lg font-semibold text-secondary-900 mb-6 flex items-center gap-2">
            <el-icon class="text-primary-500"><Setting /></el-icon>
            模型配置
          </h2>
          <el-form label-position="top">
            <el-form-item label="当前模型">
              <el-select v-model="configs.ai_model" class="w-full">
                <el-option v-for="opt in modelOptions" :key="opt.value" :label="opt.label" :value="opt.value" />
              </el-select>
            </el-form-item>
            <el-form-item label="API Key">
              <el-input
                v-model="configs.api_key"
                :type="showApiKey ? 'text' : 'password'"
                placeholder="请输入API Key"
                :show-password="true"
              />
            </el-form-item>
            <el-form-item label="Temperature">
              <div class="flex items-center gap-4 w-full">
                <el-slider v-model="configs.temperature" :min="0" :max="2" :step="0.1" class="flex-1" />
                <span class="text-sm font-mono text-secondary-700 w-12 text-right">{{ configs.temperature }}</span>
              </div>
            </el-form-item>
            <el-form-item label="Top P">
              <div class="flex items-center gap-4 w-full">
                <el-slider v-model="configs.top_p" :min="0" :max="1" :step="0.1" class="flex-1" />
                <span class="text-sm font-mono text-secondary-700 w-12 text-right">{{ configs.top_p }}</span>
              </div>
            </el-form-item>
          </el-form>
        </div>

        <!-- 限制配置区 -->
        <div class="bg-white rounded-lg shadow-sm border border-secondary-200 p-6">
          <h2 class="text-lg font-semibold text-secondary-900 mb-6">限制配置</h2>
          <el-form label-position="top">
            <div class="grid grid-cols-2 gap-6">
              <el-form-item label="每日免费刷题限制">
                <el-input-number v-model="configs.daily_free_quiz_limit" :min="0" :max="100" class="w-full" />
              </el-form-item>
              <el-form-item label="每日免费面试限制">
                <el-input-number v-model="configs.daily_free_interview_limit" :min="0" :max="50" class="w-full" />
              </el-form-item>
            </div>
          </el-form>
        </div>
      </div>

      <!-- 右侧参数说明 -->
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
