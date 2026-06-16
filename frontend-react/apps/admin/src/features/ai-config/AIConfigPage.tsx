import type { FormEvent } from 'react'
import { useEffect, useMemo, useRef, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { extractErrorMessage, requestJson } from '@makejob/api-client'
import { isSuccessCode, type ApiEnvelope } from '@makejob/shared-types'
import {
  Alert,
  Badge,
  Button,
  Card,
  Col,
  ConfigProvider,
  Empty,
  Form,
  Input,
  InputNumber,
  List,
  message,
  Modal,
  Row,
  Select,
  Space,
  Spin,
  Switch,
  Tag,
  Typography,
} from 'antd'
import {
  ReloadOutlined,
  SaveOutlined,
  PlusOutlined,
  DeleteOutlined,
  EditOutlined,
  CheckCircleOutlined,
  ExclamationCircleOutlined,
  RobotOutlined,
  ThunderboltOutlined,
  BranchesOutlined,
  SafetyOutlined,
} from '@ant-design/icons'
import { useAdminAuthStore } from '../../state/auth'

const { Title, Text, Paragraph } = Typography
const { confirm } = Modal

type AdminConfigValueType = 'string' | 'number' | 'boolean'
type AIConfigGroup = 'provider' | 'runtime' | 'scene'

interface AdminConfigItem {
  id?: number
  config_key: string
  config_value: string
  config_type: string
  description?: string
}

interface AIPresetSummary {
  id: number
  name: string
  is_active: boolean
  updated_at?: string
  configs: Record<string, string>
}

interface AIConfigResponse {
  configs: Record<string, string>
  items: AdminConfigItem[]
  support: {
    primary_providers: string[]
    fallback_providers: string[]
    notes: string[]
  }
  warnings: string[]
  presets: AIPresetSummary[]
  active_preset_id?: number | null
}

interface AIConfigFieldOption {
  value: string
  label: string
}

interface AIConfigFieldMeta {
  key: string
  label: string
  description: string
  type: AdminConfigValueType
  group: AIConfigGroup
  secret?: boolean
  placeholder?: string
  options?: AIConfigFieldOption[]
}

const DEFAULT_AI_CONFIG_FIELDS: AIConfigFieldMeta[] = [
  {
    key: 'ai_provider',
    label: '主提供方',
    description: '设置默认使用的 AI Provider，当前只应展示 runtime 真实支持的选项。',
    type: 'string',
    group: 'provider',
    options: [
      { value: 'eino', label: 'Eino' },
      { value: 'openai', label: 'OpenAI' },
      { value: 'azure', label: 'Azure OpenAI' },
      { value: 'mock', label: 'Mock' },
    ],
  },
  {
    key: 'ai_fallback_provider',
    label: '兜底提供方',
    description: '主 Provider 失败时自动回退到该 Provider，当前只展示 runtime 已启用的兜底选项。',
    type: 'string',
    group: 'provider',
    options: [
      { value: 'mock', label: 'Mock' },
      { value: 'eino', label: 'Eino' },
      { value: 'openai', label: 'OpenAI' },
      { value: 'azure', label: 'Azure OpenAI' },
    ],
  },
  {
    key: 'ai_model',
    label: '默认模型',
    description: '所有场景默认继承该模型，场景模型为空时会回退到这里。',
    type: 'string',
    group: 'provider',
    placeholder: '例如 gpt-4o-mini',
  },
  {
    key: 'ai_api_key',
    label: 'API Key',
    description: '上游 AI 服务访问密钥。',
    type: 'string',
    group: 'provider',
    secret: true,
    placeholder: '输入上游 API Key',
  },
  {
    key: 'ai_base_url',
    label: 'Base URL',
    description: '自定义上游服务地址，留空时走默认地址。',
    type: 'string',
    group: 'provider',
    placeholder: 'https://example.com/v1',
  },
  {
    key: 'ai_temperature',
    label: 'Temperature',
    description: '控制生成结果随机性，通常建议 0.1 到 1.0。',
    type: 'number',
    group: 'runtime',
  },
  {
    key: 'ai_top_p',
    label: 'Top P',
    description: '控制 nucleus sampling 采样范围。',
    type: 'number',
    group: 'runtime',
  },
  {
    key: 'ai_max_tokens',
    label: 'Max Tokens',
    description: '限制单次生成的最大 token 数。',
    type: 'number',
    group: 'runtime',
  },
  {
    key: 'ai_timeout_seconds',
    label: '超时时间',
    description: 'AI 请求超时时间，单位秒。',
    type: 'number',
    group: 'runtime',
  },
  {
    key: 'ai_enable_stream',
    label: '启用流式响应',
    description: '开启后将优先走流式返回能力。',
    type: 'boolean',
    group: 'runtime',
  },
  {
    key: 'ai_scene_interview_model',
    label: '面试场景模型',
    description: '题卡流水线使用面试场景模型；为空时继承默认模型。',
    type: 'string',
    group: 'scene',
    placeholder: '留空则继承默认模型',
  },
  {
    key: 'ai_scene_plan_model',
    label: '计划场景模型',
    description: '学习计划生成场景的模型覆盖。',
    type: 'string',
    group: 'scene',
    placeholder: '留空则继承默认模型',
  },
  {
    key: 'ai_scene_companion_model',
    label: '陪伴场景模型',
    description: '学习陪伴房间对话场景的模型覆盖。',
    type: 'string',
    group: 'scene',
    placeholder: '留空则继承默认模型',
  },
  {
    key: 'ai_scene_quiz_model',
    label: '刷题分析模型',
    description: '题目解析、批改等场景的模型覆盖。',
    type: 'string',
    group: 'scene',
    placeholder: '留空则继承默认模型',
  },
]

const AI_PROVIDER_LABELS: Record<string, string> = {
  eino: 'Eino',
  openai: 'OpenAI',
  azure: 'Azure OpenAI',
  mock: 'Mock',
}

async function fetchAIConfigs(token: string | null): Promise<AIConfigResponse> {
  const response = await requestJson<ApiEnvelope<AIConfigResponse>>('/admin/ai-configs', {
    method: 'GET',
    token,
  })
  if (!isSuccessCode(response.code)) {
    throw new Error(response.message || '获取 AI 配置失败')
  }
  return response.data
}

async function updateAIConfigs(token: string | null, configs: Record<string, string>): Promise<void> {
  const response = await requestJson<ApiEnvelope<null>>('/admin/ai-configs', {
    method: 'PUT',
    token,
    body: { configs },
  })
  if (!isSuccessCode(response.code)) {
    throw new Error(response.message || '保存 AI 配置失败')
  }
}

async function createAIPreset(
  token: string | null,
  payload: { name: string; configs: Record<string, string> },
): Promise<AIPresetSummary> {
  const response = await requestJson<ApiEnvelope<AIPresetSummary>>('/admin/ai-config-presets', {
    method: 'POST',
    token,
    body: payload,
  })
  if (!isSuccessCode(response.code)) {
    throw new Error(response.message || '创建 AI 预设失败')
  }
  return response.data
}

async function updateAIPreset(
  token: string | null,
  id: number,
  payload: { name?: string; configs?: Record<string, string> },
): Promise<AIPresetSummary> {
  const response = await requestJson<ApiEnvelope<AIPresetSummary>>(`/admin/ai-config-presets/${id}`, {
    method: 'PUT',
    token,
    body: payload,
  })
  if (!isSuccessCode(response.code)) {
    throw new Error(response.message || '更新 AI 预设失败')
  }
  return response.data
}

async function deleteAIPreset(token: string | null, id: number): Promise<void> {
  const response = await requestJson<ApiEnvelope<null>>(`/admin/ai-config-presets/${id}`, {
    method: 'DELETE',
    token,
  })
  if (!isSuccessCode(response.code)) {
    throw new Error(response.message || '删除 AI 预设失败')
  }
}

async function applyAIPreset(token: string | null, id: number): Promise<AIConfigResponse> {
  const response = await requestJson<ApiEnvelope<AIConfigResponse>>(`/admin/ai-config-presets/${id}/apply`, {
    method: 'POST',
    token,
  })
  if (!isSuccessCode(response.code)) {
    throw new Error(response.message || '应用 AI 预设失败')
  }
  return response.data
}

function getAIProviderLabel(value: string): string {
  return AI_PROVIDER_LABELS[value] || value
}

function buildProviderOptions(values: string[], currentValue: string, allowEmpty: boolean): AIConfigFieldOption[] {
  const options: AIConfigFieldOption[] = []
  if (allowEmpty) {
    options.push({ value: '', label: '不启用' })
  }
  values.forEach((value) => {
    options.push({ value, label: getAIProviderLabel(value) })
  })
  const trimmedCurrentValue = currentValue.trim()
  if (trimmedCurrentValue && !options.some((option) => option.value === trimmedCurrentValue)) {
    options.push({
      value: trimmedCurrentValue,
      label: `${getAIProviderLabel(trimmedCurrentValue)}（当前旧值，不再支持）`,
    })
  }
  return options
}

function buildAIConfigFieldMetas(
  items: AdminConfigItem[],
  configs: Record<string, string>,
  support: AIConfigResponse['support'] | undefined,
): AIConfigFieldMeta[] {
  const itemMap = new Map(items.map((item) => [item.config_key, item]))
  return DEFAULT_AI_CONFIG_FIELDS.map((field) => {
    const current = itemMap.get(field.key)
    const nextField: AIConfigFieldMeta = {
      ...field,
      type: (current?.config_type as AdminConfigValueType) || field.type,
      description: current?.description || field.description,
    }
    if (field.key === 'ai_provider') {
      nextField.options = buildProviderOptions(support?.primary_providers || ['eino'], configs[field.key] ?? '', false)
      nextField.description = '当前 runtime 仅允许选择真实已接入的主 Provider。'
    }
    if (field.key === 'ai_fallback_provider') {
      nextField.options = buildProviderOptions(support?.fallback_providers || [], configs[field.key] ?? '', true)
      nextField.description = '当前 runtime 尚未启用额外兜底 Provider，如无特殊说明应保持留空。'
    }
    return nextField
  })
}

function buildRuntimeSummary(configs: Record<string, string>): Array<{ label: string; value: string; icon: React.ReactNode; gradient: string }> {
  const interviewModel = configs.ai_scene_interview_model?.trim() || configs.ai_model || '未配置'
  return [
    {
      label: '当前主 Provider',
      value: getAIProviderLabel(configs.ai_provider || '未配置'),
      icon: <RobotOutlined />,
      gradient: 'linear-gradient(135deg, #6366f1 0%, #4f46e5 100%)',
    },
    {
      label: '当前默认模型',
      value: configs.ai_model || '未配置',
      icon: <ThunderboltOutlined />,
      gradient: 'linear-gradient(135deg, #3b82f6 0%, #2563eb 100%)',
    },
    {
      label: '题卡流水线模型',
      value: interviewModel,
      icon: <BranchesOutlined />,
      gradient: 'linear-gradient(135deg, #0ea5e9 0%, #0284c7 100%)',
    },
    {
      label: '当前兜底 Provider',
      value: configs.ai_fallback_provider ? getAIProviderLabel(configs.ai_fallback_provider) : '不启用',
      icon: <SafetyOutlined />,
      gradient: 'linear-gradient(135deg, #64748b 0%, #475569 100%)',
    },
  ]
}

function buildAIConfigDraft(configs: Record<string, string>, metas: AIConfigFieldMeta[]): Record<string, string> {
  return metas.reduce<Record<string, string>>((result, field) => {
    result[field.key] = configs[field.key] ?? ''
    return result
  }, {})
}

function normalizeAIConfigValue(meta: AIConfigFieldMeta, value: string): string {
  if (meta.type === 'boolean') {
    return String(value).trim().toLowerCase() === 'true' ? 'true' : 'false'
  }
  return value.trim()
}

function buildNormalizedConfigs(draft: Record<string, string>, metas: AIConfigFieldMeta[]): Record<string, string> {
  return metas.reduce<Record<string, string>>((result, meta) => {
    result[meta.key] = normalizeAIConfigValue(meta, draft[meta.key] ?? '')
    return result
  }, {})
}

function collectChangedConfigKeys(
  draft: Record<string, string>,
  configs: Record<string, string>,
  metas: AIConfigFieldMeta[],
): string[] {
  return metas
    .filter((meta) => normalizeAIConfigValue(meta, draft[meta.key] ?? '') !== (configs[meta.key] ?? ''))
    .map((meta) => meta.key)
}

function groupAIConfigFields(metas: AIConfigFieldMeta[]): Record<AIConfigGroup, AIConfigFieldMeta[]> {
  return metas.reduce<Record<AIConfigGroup, AIConfigFieldMeta[]>>(
    (result, field) => {
      result[field.group].push(field)
      return result
    },
    { provider: [], runtime: [], scene: [] },
  )
}

function formatPresetUpdatedAt(value?: string): string {
  if (!value) return '时间未知'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '时间未知'
  return date.toLocaleString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  })
}

/* ------------------------------------------------------------------ */
/*  现代感视觉 token                                                   */
/* ------------------------------------------------------------------ */

const THEME = {
  token: {
    borderRadius: 14,
    borderRadiusLG: 20,
    colorPrimary: '#2563eb',
    colorBgContainer: '#ffffff',
    colorBorder: '#e2e8f0',
    colorBorderSecondary: '#f1f5f9',
    colorText: '#0f172a',
    colorTextSecondary: '#64748b',
    colorTextTertiary: '#94a3b8',
    fontFamily: 'Inter, "SF Pro Display", -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif',
    controlHeight: 44,
    controlHeightLG: 52,
    controlHeightSM: 36,
    boxShadow: '0 0 0 1px rgba(0,0,0,0.03), 0 2px 8px rgba(0,0,0,0.04), 0 12px 24px rgba(0,0,0,0.03)',
    boxShadowSecondary: '0 0 0 1px rgba(0,0,0,0.04), 0 8px 16px rgba(0,0,0,0.06), 0 24px 48px rgba(0,0,0,0.04)',
  },
}

const glassCard = {
  background: 'rgba(255, 255, 255, 0.85)',
  backdropFilter: 'blur(20px) saturate(180%)',
  WebkitBackdropFilter: 'blur(20px) saturate(180%)',
  borderRadius: 20,
  border: '1px solid rgba(255, 255, 255, 0.6)',
  boxShadow: '0 0 0 1px rgba(0,0,0,0.03), 0 2px 8px rgba(0,0,0,0.04), 0 12px 24px rgba(0,0,0,0.03)',
  transition: 'all 0.35s cubic-bezier(0.4, 0, 0.2, 1)',
} as React.CSSProperties

const solidCard = {
  background: '#ffffff',
  borderRadius: 20,
  border: '1px solid #f1f5f9',
  boxShadow: '0 0 0 1px rgba(0,0,0,0.02), 0 4px 12px rgba(0,0,0,0.03)',
  transition: 'all 0.35s cubic-bezier(0.4, 0, 0.2, 1)',
} as React.CSSProperties

export function AIConfigPage() {
  const accessToken = useAdminAuthStore((state) => state.accessToken)
  const queryClient = useQueryClient()
  const [draft, setDraft] = useState<Record<string, string>>({})
  const [selectedPresetId, setSelectedPresetId] = useState<number | null>(null)
  const [form] = Form.useForm()

  const [nameModalOpen, setNameModalOpen] = useState(false)
  const [nameModalTitle, setNameModalTitle] = useState('')
  const [nameModalValue, setNameModalValue] = useState('')
  const nameModalActionRef = useRef<(name: string) => void>(() => {})

  const configQuery = useQuery({
    queryKey: ['admin', 'ai-configs', accessToken],
    queryFn: () => fetchAIConfigs(accessToken),
    enabled: Boolean(accessToken),
  })

  const fieldMetas = useMemo(
    () => buildAIConfigFieldMetas(configQuery.data?.items || [], configQuery.data?.configs || {}, configQuery.data?.support),
    [configQuery.data?.configs, configQuery.data?.items, configQuery.data?.support],
  )
  const groupedFields = useMemo(() => groupAIConfigFields(fieldMetas), [fieldMetas])
  const runtimeSummary = useMemo(() => buildRuntimeSummary(configQuery.data?.configs || {}), [configQuery.data?.configs])
  const activePreset = useMemo(
    () => configQuery.data?.presets.find((preset) => preset.id === configQuery.data?.active_preset_id) || null,
    [configQuery.data],
  )
  const selectedPreset = useMemo(
    () => configQuery.data?.presets.find((preset) => preset.id === selectedPresetId) || null,
    [configQuery.data, selectedPresetId],
  )

  useEffect(() => {
    if (!configQuery.data) return
    setSelectedPresetId(configQuery.data.active_preset_id ?? null)
    const newDraft = buildAIConfigDraft(configQuery.data.configs, fieldMetas)
    setDraft(newDraft)
    form.setFieldsValue(newDraft)
  }, [configQuery.data, fieldMetas, form])

  const normalizedDraft = useMemo(() => buildNormalizedConfigs(draft, fieldMetas), [draft, fieldMetas])
  const changedKeys = useMemo(() => {
    if (!configQuery.data) return []
    return collectChangedConfigKeys(draft, configQuery.data.configs, fieldMetas)
  }, [configQuery.data, draft, fieldMetas])
  const selectedPresetChangedKeys = useMemo(() => {
    if (!selectedPreset) return []
    return collectChangedConfigKeys(draft, selectedPreset.configs, fieldMetas)
  }, [draft, fieldMetas, selectedPreset])

  const saveMutation = useMutation({
    mutationFn: async () => {
      await updateAIConfigs(accessToken, normalizedDraft)
    },
    onSuccess: async () => {
      message.success({ content: 'AI 配置已保存', icon: <CheckCircleOutlined /> })
      await queryClient.invalidateQueries({ queryKey: ['admin', 'ai-configs'] })
    },
    onError: (error) => {
      message.error(extractErrorMessage(error, '保存 AI 配置失败'))
    },
  })

  const createPresetMutation = useMutation({
    mutationFn: async (name: string) => {
      return createAIPreset(accessToken, { name, configs: normalizedDraft })
    },
    onSuccess: async (preset) => {
      setSelectedPresetId(preset.id)
      message.success({ content: `已创建预设：${preset.name}`, icon: <CheckCircleOutlined /> })
      await queryClient.invalidateQueries({ queryKey: ['admin', 'ai-configs'] })
    },
    onError: (error) => {
      message.error(extractErrorMessage(error, '创建 AI 预设失败'))
    },
  })

  const renamePresetMutation = useMutation({
    mutationFn: async ({ id, name }: { id: number; name: string }) => {
      return updateAIPreset(accessToken, id, { name })
    },
    onSuccess: async (preset) => {
      setSelectedPresetId(preset.id)
      message.success({ content: `已重命名预设：${preset.name}`, icon: <CheckCircleOutlined /> })
      await queryClient.invalidateQueries({ queryKey: ['admin', 'ai-configs'] })
    },
    onError: (error) => {
      message.error(extractErrorMessage(error, '重命名 AI 预设失败'))
    },
  })

  const deletePresetMutation = useMutation({
    mutationFn: async (presetId: number) => {
      await deleteAIPreset(accessToken, presetId)
    },
    onSuccess: async () => {
      setSelectedPresetId(configQuery.data?.active_preset_id ?? null)
      message.success({ content: 'AI 预设已删除', icon: <CheckCircleOutlined /> })
      await queryClient.invalidateQueries({ queryKey: ['admin', 'ai-configs'] })
    },
    onError: (error) => {
      message.error(extractErrorMessage(error, '删除 AI 预设失败'))
    },
  })

  const applyPresetMutation = useMutation({
    mutationFn: async (presetId: number) => {
      return applyAIPreset(accessToken, presetId)
    },
    onSuccess: async (_response, presetId) => {
      setSelectedPresetId(presetId)
      message.success({ content: '所选 AI 预设已应用为当前全局运行配置', icon: <CheckCircleOutlined /> })
      await queryClient.invalidateQueries({ queryKey: ['admin', 'ai-configs'] })
    },
    onError: (error) => {
      message.error(extractErrorMessage(error, '应用 AI 预设失败'))
    },
  })

  function updateDraftValue(key: string, value: string): void {
    setDraft((current) => ({ ...current, [key]: value }))
  }

  function resetDraftToRuntime(): void {
    if (!configQuery.data) return
    setSelectedPresetId(configQuery.data.active_preset_id ?? null)
    const newDraft = buildAIConfigDraft(configQuery.data.configs, fieldMetas)
    setDraft(newDraft)
    form.setFieldsValue(newDraft)
    message.info('已恢复为当前服务端生效配置')
  }

  function loadPresetToDraft(preset: AIPresetSummary): void {
    setSelectedPresetId(preset.id)
    const newDraft = buildAIConfigDraft(preset.configs, fieldMetas)
    setDraft(newDraft)
    form.setFieldsValue(newDraft)
    message.info(`已将预设"${preset.name}"载入草稿`)
  }

  function openNameModal(title: string, defaultValue: string, action: (name: string) => void): void {
    setNameModalTitle(title)
    setNameModalValue(defaultValue)
    nameModalActionRef.current = action
    setNameModalOpen(true)
  }

  function handleCreatePreset(): void {
    openNameModal('创建 AI 预设', '', (name) => {
      if (!name.trim()) return
      createPresetMutation.mutate(name.trim())
    })
  }

  function handleRenamePreset(): void {
    if (!selectedPreset) return
    openNameModal('重命名预设', selectedPreset.name, (name) => {
      const trimmed = name.trim()
      if (!trimmed || trimmed === selectedPreset.name) return
      renamePresetMutation.mutate({ id: selectedPreset.id, name: trimmed })
    })
  }

  function handleDeletePreset(): void {
    if (!selectedPreset || selectedPreset.is_active) return
    confirm({
      title: `确认删除预设"${selectedPreset.name}"吗？`,
      icon: <ExclamationCircleOutlined />,
      content: '删除后不可恢复。',
      okText: '删除',
      okType: 'danger',
      cancelText: '取消',
      onOk: () => deletePresetMutation.mutate(selectedPreset.id),
    })
  }

  function handleApplyPreset(): void {
    if (!selectedPreset) return
    confirm({
      title: `确认将预设"${selectedPreset.name}"应用为当前全局 AI 配置吗？`,
      icon: <ExclamationCircleOutlined />,
      okText: '应用',
      cancelText: '取消',
      onOk: () => applyPresetMutation.mutate(selectedPreset.id),
    })
  }

  function handleSubmit(event: FormEvent<HTMLFormElement>): void {
    event.preventDefault()
    saveMutation.mutate()
  }

  const mutationPending =
    saveMutation.isPending ||
    createPresetMutation.isPending ||
    renamePresetMutation.isPending ||
    deletePresetMutation.isPending ||
    applyPresetMutation.isPending

  /* ------------------------------------------------------------------ */
  /*  加载 / 错误态                                                      */
  /* ------------------------------------------------------------------ */

  if (configQuery.isLoading) {
    return (
      <ConfigProvider theme={THEME}>
        <div style={{ minHeight: '100vh', display: 'flex', alignItems: 'center', justifyContent: 'center', background: '#f0f2f5' }}>
          <Spin size="large" tip="正在加载 AI 运行配置..." />
        </div>
      </ConfigProvider>
    )
  }

  if (configQuery.isError || !configQuery.data) {
    return (
      <ConfigProvider theme={THEME}>
        <div style={{ padding: 40, maxWidth: 800, margin: '0 auto' }}>
          <Alert
            message="读取 AI 配置失败"
            description={extractErrorMessage(configQuery.error, '请检查网络或权限后重试')}
            type="error"
            showIcon
            style={{ borderRadius: 20, padding: 24 }}
          />
        </div>
      </ConfigProvider>
    )
  }

  const attentionNotes = [...(configQuery.data.support?.notes || []), ...(configQuery.data.warnings || [])]

  /* ------------------------------------------------------------------ */
  /*  主界面                                                             */
  /* ------------------------------------------------------------------ */

  return (
    <ConfigProvider theme={THEME}>
      <div
        style={{
          minHeight: '100vh',
          background: '#f0f2f5',
          padding: '32px 24px 64px',
          fontFamily: THEME.token.fontFamily as string,
        }}
      >
        <div style={{ maxWidth: 1200, margin: '0 auto' }}>
          {/* ===== 页面标题区（毛玻璃风） ===== */}
          <div
            style={{
              ...glassCard,
              padding: '28px 32px',
              marginBottom: 28,
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'space-between',
              flexWrap: 'wrap',
              gap: 16,
            }}
          >
            <Space direction="vertical" size={8} style={{ flex: 1, minWidth: 280 }}>
              <Space align="center" size={12}>
                <div
                  style={{
                    width: 40,
                    height: 40,
                    borderRadius: 14,
                    background: 'linear-gradient(135deg, #3b82f6, #2563eb)',
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'center',
                    color: '#fff',
                    fontSize: 20,
                    boxShadow: '0 4px 12px rgba(37, 99, 235, 0.3)',
                  }}
                >
                  <RobotOutlined />
                </div>
                <div>
                  <Title level={4} style={{ margin: 0, fontWeight: 700, letterSpacing: '-0.02em' }}>
                    AI 配置
                    {changedKeys.length > 0 && (
                      <Badge
                        count={changedKeys.length}
                        style={{
                          backgroundColor: '#f59e0b',
                          marginLeft: 10,
                          boxShadow: '0 2px 6px rgba(245, 158, 11, 0.35)',
                        }}
                      />
                    )}
                  </Title>
                </div>
              </Space>
              <Paragraph type="secondary" style={{ margin: 0, maxWidth: 600, fontSize: 14, lineHeight: 1.6 }}>
                管理 Provider、默认模型和场景模型覆盖。保存后会改写全局生效值；预设用于保存整页配置快照，支持一键切换。
              </Paragraph>
            </Space>

            <Space size={12}>
              <Button
                size="large"
                icon={<ReloadOutlined />}
                onClick={resetDraftToRuntime}
                disabled={mutationPending}
                style={{
                  borderRadius: 14,
                  height: 48,
                  padding: '0 24px',
                  border: '1px solid #e2e8f0',
                  background: 'rgba(255,255,255,0.7)',
                  fontWeight: 500,
                }}
              >
                恢复生效值
              </Button>
              <Button
                type="primary"
                size="large"
                icon={<SaveOutlined />}
                onClick={() => saveMutation.mutate()}
                loading={saveMutation.isPending}
                disabled={mutationPending || changedKeys.length === 0}
                style={{
                  borderRadius: 14,
                  height: 48,
                  padding: '0 28px',
                  fontWeight: 600,
                  background: 'linear-gradient(135deg, #3b82f6, #2563eb)',
                  border: 'none',
                  boxShadow: '0 4px 14px rgba(37, 99, 235, 0.35)',
                }}
              >
                保存 AI 配置
              </Button>
            </Space>
          </div>

          {/* ===== 运行时提示 ===== */}
          {attentionNotes.length > 0 && (
            <div style={{ marginBottom: 24 }}>
              <Alert
                message="运行时提示"
                description={
                  <ul style={{ margin: 0, paddingLeft: 18 }}>
                    {attentionNotes.map((note) => (
                      <li key={note} style={{ lineHeight: 1.8 }}>{note}</li>
                    ))}
                  </ul>
                }
                type="warning"
                showIcon
                style={{ borderRadius: 20, padding: '16px 24px', border: 'none', background: '#fffbeb' }}
              />
            </div>
          )}

          {/* ===== KPI 统计卡片区 ===== */}
          <Row gutter={[20, 20]} style={{ marginBottom: 28 }}>
            {runtimeSummary.map((item, idx) => (
              <Col xs={12} md={6} key={item.label}>
                <div
                  style={{
                    ...solidCard,
                    padding: '24px 20px',
                    cursor: 'default',
                  }}
                  onMouseEnter={(e) => {
                    e.currentTarget.style.transform = 'translateY(-4px) scale(1.01)'
                    e.currentTarget.style.boxShadow = '0 8px 24px rgba(0,0,0,0.08), 0 0 0 1px rgba(0,0,0,0.03)'
                  }}
                  onMouseLeave={(e) => {
                    e.currentTarget.style.transform = 'none'
                    e.currentTarget.style.boxShadow = solidCard.boxShadow as string
                  }}
                >
                  <div style={{ display: 'flex', alignItems: 'center', gap: 14, marginBottom: 14 }}>
                    <div
                      style={{
                        width: 44,
                        height: 44,
                        borderRadius: 14,
                        background: item.gradient,
                        display: 'flex',
                        alignItems: 'center',
                        justifyContent: 'center',
                        color: '#fff',
                        fontSize: 20,
                        boxShadow: '0 4px 10px rgba(0,0,0,0.12)',
                        flexShrink: 0,
                      }}
                    >
                      {item.icon}
                    </div>
                    <Text type="secondary" style={{ fontSize: 13, fontWeight: 500 }}>
                      {item.label}
                    </Text>
                  </div>
                  <div
                    style={{
                      fontSize: 22,
                      fontWeight: 700,
                      color: '#0f172a',
                      letterSpacing: '-0.02em',
                      wordBreak: 'break-all',
                    }}
                  >
                    {item.value}
                  </div>
                </div>
              </Col>
            ))}
          </Row>

          {/* ===== 预设管理区 ===== */}
          <div style={{ ...solidCard, padding: '28px 32px', marginBottom: 28 }}>
            <div style={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', flexWrap: 'wrap', gap: 16, marginBottom: 24 }}>
              <Space direction="vertical" size={4}>
                <Title level={5} style={{ margin: 0, fontWeight: 700 }}>
                  配置预设
                </Title>
                <Space size={8}>
                  {activePreset ? (
                    <Tag
                      color="success"
                      style={{ borderRadius: 8, padding: '2px 10px', fontWeight: 500 }}
                    >
                      <CheckCircleOutlined style={{ marginRight: 4 }} />
                      当前生效：{activePreset.name}
                    </Tag>
                  ) : (
                    <Tag style={{ borderRadius: 8, padding: '2px 10px' }}>未绑定预设</Tag>
                  )}
                  {selectedPreset && !selectedPreset.is_active && (
                    <Tag color="processing" style={{ borderRadius: 8, padding: '2px 10px' }}>
                      草稿来源：{selectedPreset.name}
                      {selectedPresetChangedKeys.length > 0 && `（${selectedPresetChangedKeys.length} 处差异）`}
                    </Tag>
                  )}
                </Space>
              </Space>

              <Space size={10}>
                <Button
                  icon={<PlusOutlined />}
                  onClick={handleCreatePreset}
                  loading={createPresetMutation.isPending}
                  style={{
                    borderRadius: 12,
                    height: 40,
                    fontWeight: 500,
                    border: '1px solid #e2e8f0',
                    background: '#f8fafc',
                  }}
                >
                  另存为预设
                </Button>
                <Button
                  type="primary"
                  icon={<CheckCircleOutlined />}
                  onClick={handleApplyPreset}
                  loading={applyPresetMutation.isPending}
                  disabled={!selectedPreset}
                  style={{
                    borderRadius: 12,
                    height: 40,
                    fontWeight: 600,
                    background: 'linear-gradient(135deg, #3b82f6, #2563eb)',
                    border: 'none',
                    boxShadow: '0 4px 12px rgba(37, 99, 235, 0.3)',
                  }}
                >
                  应用所选预设
                </Button>
                <Button
                  icon={<EditOutlined />}
                  onClick={handleRenamePreset}
                  disabled={!selectedPreset}
                  style={{ borderRadius: 12, height: 40, border: '1px solid #e2e8f0' }}
                >
                  重命名
                </Button>
                <Button
                  danger
                  icon={<DeleteOutlined />}
                  onClick={handleDeletePreset}
                  disabled={!selectedPreset || selectedPreset.is_active}
                  style={{ borderRadius: 12, height: 40 }}
                >
                  删除
                </Button>
              </Space>
            </div>

            {configQuery.data.presets.length === 0 ? (
              <Empty
                image={Empty.PRESENTED_IMAGE_SIMPLE}
                description="还没有 AI 预设"
                style={{ padding: '40px 0' }}
              >
                <Button
                  type="primary"
                  icon={<PlusOutlined />}
                  onClick={handleCreatePreset}
                  style={{
                    borderRadius: 12,
                    background: 'linear-gradient(135deg, #3b82f6, #2563eb)',
                    border: 'none',
                  }}
                >
                  创建第一条预设
                </Button>
              </Empty>
            ) : (
              <List
                grid={{ gutter: 16, xs: 1, sm: 2, md: 3, lg: 3, xl: 4 }}
                dataSource={configQuery.data.presets}
                renderItem={(preset) => {
                  const isSelected = selectedPresetId === preset.id
                  return (
                    <List.Item>
                      <div
                        onClick={() => loadPresetToDraft(preset)}
                        style={{
                          padding: '18px 20px',
                          borderRadius: 18,
                          background: isSelected ? '#eff6ff' : '#ffffff',
                          border: isSelected ? '2px solid #3b82f6' : '1px solid #f1f5f9',
                          boxShadow: isSelected
                            ? '0 4px 16px rgba(59, 130, 246, 0.15)'
                            : '0 1px 3px rgba(0,0,0,0.03)',
                          cursor: 'pointer',
                          transition: 'all 0.3s cubic-bezier(0.4, 0, 0.2, 1)',
                          position: 'relative',
                          overflow: 'hidden',
                        }}
                        onMouseEnter={(e) => {
                          if (!isSelected) {
                            e.currentTarget.style.transform = 'translateY(-3px)'
                            e.currentTarget.style.boxShadow = '0 12px 24px rgba(0,0,0,0.06)'
                            e.currentTarget.style.borderColor = '#e2e8f0'
                          }
                        }}
                        onMouseLeave={(e) => {
                          if (!isSelected) {
                            e.currentTarget.style.transform = 'none'
                            e.currentTarget.style.boxShadow = '0 1px 3px rgba(0,0,0,0.03)'
                            e.currentTarget.style.borderColor = '#f1f5f9'
                          }
                        }}
                      >
                        {isSelected && (
                          <div
                            style={{
                              position: 'absolute',
                              top: 0,
                              left: 0,
                              width: 4,
                              height: '100%',
                              background: 'linear-gradient(180deg, #3b82f6, #2563eb)',
                              borderRadius: '2px 0 0 2px',
                            }}
                          />
                        )}
                        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start' }}>
                          <Text strong style={{ fontSize: 15, color: '#0f172a' }}>
                            {preset.name}
                          </Text>
                          {preset.is_active ? (
                            <Tag color="success" style={{ borderRadius: 8, fontSize: 12, margin: 0 }}>
                              生效中
                            </Tag>
                          ) : isSelected ? (
                            <Tag color="processing" style={{ borderRadius: 8, fontSize: 12, margin: 0 }}>
                              已载入
                            </Tag>
                          ) : (
                            <Tag style={{ borderRadius: 8, fontSize: 12, margin: 0, background: '#f8fafc', borderColor: '#e2e8f0', color: '#64748b' }}>
                              可应用
                            </Tag>
                          )}
                        </div>
                        <Space size={8} style={{ marginTop: 12, flexWrap: 'wrap' }}>
                          <Tag
                            style={{
                              borderRadius: 8,
                              background: '#f1f5f9',
                              borderColor: '#e2e8f0',
                              color: '#475569',
                              fontSize: 12,
                            }}
                          >
                            {getAIProviderLabel(preset.configs.ai_provider || '未配置')}
                          </Tag>
                          <Text type="secondary" style={{ fontSize: 12 }}>
                            {preset.configs.ai_model || '未配置模型'}
                          </Text>
                        </Space>
                        <div style={{ marginTop: 10 }}>
                          <Text type="secondary" style={{ fontSize: 12 }}>
                            更新于 {formatPresetUpdatedAt(preset.updated_at)}
                          </Text>
                        </div>
                      </div>
                    </List.Item>
                  )
                }}
              />
            )}
          </div>

          {/* ===== 配置表单区 ===== */}
          <Form form={form} layout="vertical" onSubmitCapture={handleSubmit} autoComplete="off">
            <Row gutter={[24, 24]}>
              {/* 提供方与模型 */}
              <Col xs={24} lg={12}>
                <div style={{ ...solidCard, padding: '28px 32px' }}>
                  <Title level={5} style={{ margin: '0 0 24px', fontWeight: 700 }}>
                    提供方与模型
                  </Title>
                  <Space direction="vertical" style={{ width: '100%' }} size={20}>
                    {groupedFields.provider.map((field) => (
                      <Form.Item
                        key={field.key}
                        label={
                          <Text style={{ fontSize: 14, fontWeight: 600, color: '#0f172a' }}>
                            {field.label}
                          </Text>
                        }
                        extra={
                          <Text type="secondary" style={{ fontSize: 12 }}>
                            {field.description}
                          </Text>
                        }
                        style={{ marginBottom: 0 }}
                      >
                        {field.options ? (
                          <Select
                            value={draft[field.key] ?? ''}
                            onChange={(value) => updateDraftValue(field.key, value)}
                            options={field.options}
                            style={{ width: '100%' }}
                            size="large"
                            dropdownStyle={{ borderRadius: 14 }}
                          />
                        ) : (
                          <Input
                            type={field.secret ? 'password' : 'text'}
                            value={draft[field.key] ?? ''}
                            placeholder={field.placeholder}
                            onChange={(e) => updateDraftValue(field.key, e.target.value)}
                            size="large"
                            style={{ borderRadius: 14 }}
                          />
                        )}
                        <Text type="secondary" style={{ fontSize: 12, marginTop: 4, display: 'block', fontFamily: 'monospace' }}>
                          {field.key}
                        </Text>
                      </Form.Item>
                    ))}
                  </Space>
                </div>
              </Col>

              {/* 运行时参数 */}
              <Col xs={24} lg={12}>
                <div style={{ ...solidCard, padding: '28px 32px' }}>
                  <Title level={5} style={{ margin: '0 0 24px', fontWeight: 700 }}>
                    运行时参数
                  </Title>
                  <Space direction="vertical" style={{ width: '100%' }} size={20}>
                    {groupedFields.runtime.map((field) => (
                      <Form.Item
                        key={field.key}
                        label={
                          <Text style={{ fontSize: 14, fontWeight: 600, color: '#0f172a' }}>
                            {field.label}
                          </Text>
                        }
                        extra={
                          <Text type="secondary" style={{ fontSize: 12 }}>
                            {field.description}
                          </Text>
                        }
                        style={{ marginBottom: 0 }}
                      >
                        {field.type === 'boolean' ? (
                          <Switch
                            checked={String(draft[field.key]).trim().toLowerCase() === 'true'}
                            onChange={(checked) => updateDraftValue(field.key, String(checked))}
                            style={{
                              backgroundColor: String(draft[field.key]).trim().toLowerCase() === 'true' ? '#3b82f6' : '#cbd5e1',
                            }}
                          />
                        ) : (
                          <InputNumber
                            style={{ width: '100%', borderRadius: 14 }}
                            step={field.key === 'ai_temperature' || field.key === 'ai_top_p' ? 0.1 : 1}
                            value={draft[field.key] === '' ? undefined : Number(draft[field.key])}
                            onChange={(value) => updateDraftValue(field.key, value === undefined ? '' : String(value))}
                            size="large"
                          />
                        )}
                        <Text type="secondary" style={{ fontSize: 12, marginTop: 4, display: 'block', fontFamily: 'monospace' }}>
                          {field.key}
                        </Text>
                      </Form.Item>
                    ))}
                  </Space>
                </div>
              </Col>

              {/* 场景模型覆盖 */}
              <Col xs={24}>
                <div style={{ ...solidCard, padding: '28px 32px' }}>
                  <Title level={5} style={{ margin: '0 0 8px', fontWeight: 700 }}>
                    场景模型覆盖
                  </Title>
                  <Paragraph type="secondary" style={{ marginBottom: 24, fontSize: 14 }}>
                    留空表示沿用默认模型，仅在场景差异明显时单独指定。题卡流水线使用面试场景模型。
                  </Paragraph>
                  <Row gutter={[24, 20]}>
                    {groupedFields.scene.map((field) => (
                      <Col xs={24} sm={12} md={8} key={field.key}>
                        <Form.Item
                          label={
                            <Text style={{ fontSize: 14, fontWeight: 600, color: '#0f172a' }}>
                              {field.label}
                            </Text>
                          }
                          extra={
                            <Text type="secondary" style={{ fontSize: 12 }}>
                              {field.description}
                            </Text>
                          }
                          style={{ marginBottom: 0 }}
                        >
                          <Input
                            value={draft[field.key] ?? ''}
                            placeholder={field.placeholder}
                            onChange={(e) => updateDraftValue(field.key, e.target.value)}
                            size="large"
                            style={{ borderRadius: 14 }}
                          />
                          <Text type="secondary" style={{ fontSize: 12, marginTop: 4, display: 'block', fontFamily: 'monospace' }}>
                            {field.key}
                          </Text>
                        </Form.Item>
                      </Col>
                    ))}
                  </Row>
                </div>
              </Col>
            </Row>

            {/* 底部浮动操作栏 */}
            <div
              style={{
                position: 'sticky',
                bottom: 24,
                marginTop: 32,
                padding: '16px 28px',
                borderRadius: 18,
                background: 'rgba(255, 255, 255, 0.92)',
                backdropFilter: 'blur(16px)',
                WebkitBackdropFilter: 'blur(16px)',
                border: '1px solid rgba(255, 255, 255, 0.6)',
                boxShadow: '0 8px 32px rgba(0,0,0,0.08), 0 0 0 1px rgba(0,0,0,0.03)',
                display: 'flex',
                justifyContent: 'flex-end',
                alignItems: 'center',
                gap: 16,
                zIndex: 10,
              }}
            >
              {changedKeys.length > 0 && (
                <Text type="warning" style={{ fontSize: 14, fontWeight: 500 }}>
                  有 {changedKeys.length} 处待保存改动
                </Text>
              )}
              {selectedPreset && selectedPresetChangedKeys.length > 0 && (
                <Text type="secondary" style={{ fontSize: 13 }}>
                  与所选预设有 {selectedPresetChangedKeys.length} 处差异
                </Text>
              )}
              <Button
                size="large"
                icon={<ReloadOutlined />}
                onClick={resetDraftToRuntime}
                disabled={mutationPending}
                style={{
                  borderRadius: 14,
                  height: 46,
                  padding: '0 22px',
                  border: '1px solid #e2e8f0',
                  background: 'rgba(255,255,255,0.8)',
                }}
              >
                恢复生效值
              </Button>
              <Button
                type="primary"
                size="large"
                icon={<SaveOutlined />}
                onClick={() => saveMutation.mutate()}
                loading={saveMutation.isPending}
                disabled={mutationPending || changedKeys.length === 0}
                style={{
                  borderRadius: 14,
                  height: 46,
                  padding: '0 26px',
                  fontWeight: 600,
                  background: 'linear-gradient(135deg, #3b82f6, #2563eb)',
                  border: 'none',
                  boxShadow: '0 4px 14px rgba(37, 99, 235, 0.35)',
                }}
              >
                保存 AI 配置
              </Button>
            </div>
          </Form>
        </div>

        {/* 名称输入弹窗 */}
        <Modal
          title={nameModalTitle}
          open={nameModalOpen}
          onOk={() => {
            nameModalActionRef.current(nameModalValue)
            setNameModalOpen(false)
          }}
          onCancel={() => setNameModalOpen(false)}
          destroyOnClose
          okButtonProps={{
            style: {
              borderRadius: 12,
              background: 'linear-gradient(135deg, #3b82f6, #2563eb)',
              border: 'none',
              boxShadow: '0 4px 12px rgba(37, 99, 235, 0.3)',
            },
          }}
          cancelButtonProps={{ style: { borderRadius: 12 } }}
          styles={{
            body: { padding: '24px 32px 32px' },
            header: { padding: '20px 32px 0', borderBottom: 'none' },
            footer: { padding: '0 32px 24px', borderTop: 'none' },
          }}
        >
          <Input
            value={nameModalValue}
            onChange={(e) => setNameModalValue(e.target.value)}
            placeholder="请输入名称"
            onPressEnter={() => {
              nameModalActionRef.current(nameModalValue)
              setNameModalOpen(false)
            }}
            size="large"
            style={{ borderRadius: 14, marginTop: 8 }}
          />
        </Modal>
      </div>
    </ConfigProvider>
  )
}
