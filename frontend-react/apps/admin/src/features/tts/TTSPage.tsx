import type { FormEvent } from 'react'
import { useEffect, useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  SoundOutlined,
  PlusOutlined,
  DeleteOutlined,
  SaveOutlined,
  ReloadOutlined,
  CheckCircleOutlined,
  ExclamationCircleOutlined,
  InboxOutlined,
} from '@ant-design/icons'
import { Button, Input, InputNumber, Modal, Select, Switch, Tag, Tooltip } from 'antd'
import { extractErrorMessage, requestJson } from '@makejob/api-client'
import { isSuccessCode, type ApiEnvelope } from '@makejob/shared-types'
import { useAdminAuthStore } from '../../state/auth'

type TTSEngine = string
type TTSScene = 'interview' | 'companion'

interface TTSProviderFieldDefinition {
  key: string
  label: string
  description: string
  required: boolean
  secret?: boolean
}

interface TTSProviderDescriptor {
  key: TTSEngine
  label: string
  description: string
  support_status: string
  support_message: string
  auth_template: string
  params_template: string
  auth_fields: TTSProviderFieldDefinition[]
  param_fields: TTSProviderFieldDefinition[]
}

interface TTSConfig {
  id: number
  name: string
  engine: TTSEngine
  voice_id: string
  scene?: string
  auth_config_json: string
  params_json: string
  is_active: boolean
  sort_order: number
  support_status: string
  support_message: string
  created_at?: string
  updated_at?: string
}

interface TTSConfigListResponse {
  configs: TTSConfig[]
  providers: TTSProviderDescriptor[]
  default_bindings: Partial<Record<TTSScene, number>>
}

interface TTSFormState {
  name: string
  engine: TTSEngine
  voiceId: string
  authConfigJson: string
  paramsJson: string
  isActive: boolean
  sortOrder: string
}

interface JSONPreview {
  valid: boolean
  error: string
  formattedJson: string
  parsedObject: Record<string, unknown>
}

interface SceneDefaultFormState {
  interview: string
  companion: string
}

const THEME = {
  bg: '#f4f7fe',
  cardBg: '#ffffff',
  primary: '#4f46e5',
  primaryLight: '#e0e7ff',
  accent: '#f59e0b',
  textMain: '#1e293b',
  textSecondary: '#64748b',
  textMuted: '#94a3b8',
  border: '#e2e8f0',
  success: '#10b981',
  warning: '#f59e0b',
  danger: '#ef4444',
  shadow: '0 8px 32px rgba(31, 38, 135, 0.07)',
  radius: 16,
}

const glassCard = {
  background: 'rgba(255,255,255,0.85)',
  backdropFilter: 'blur(20px) saturate(180%)',
  WebkitBackdropFilter: 'blur(20px) saturate(180%)',
  borderRadius: THEME.radius,
  border: '1px solid rgba(255,255,255,0.6)',
  boxShadow: THEME.shadow,
}

const solidCard = {
  background: THEME.cardBg,
  borderRadius: THEME.radius,
  boxShadow: THEME.shadow,
  border: '1px solid ' + THEME.border,
}

/**
 * 获取后台当前维护的 TTS 配置、供应商目录与默认场景绑定。
 */
async function fetchTTSConfigs(token: string | null): Promise<TTSConfigListResponse> {
  const response = await requestJson<ApiEnvelope<TTSConfigListResponse>>('/admin/tts-configs', {
    method: 'GET',
    token,
  })

  if (!isSuccessCode(response.code)) {
    throw new Error(response.message || '获取 TTS 配置列表失败')
  }

  return response.data
}

/**
 * 创建新的 TTS 配置，并返回服务端保存后的结果。
 */
async function createTTSConfig(token: string | null, payload: Record<string, unknown>): Promise<TTSConfig> {
  const response = await requestJson<ApiEnvelope<TTSConfig>>('/admin/tts-configs', {
    method: 'POST',
    token,
    body: payload,
  })

  if (!isSuccessCode(response.code)) {
    throw new Error(response.message || '创建 TTS 配置失败')
  }

  return response.data
}

/**
 * 更新指定的 TTS 配置记录。
 */
async function updateTTSConfig(token: string | null, id: number, payload: Record<string, unknown>): Promise<void> {
  const response = await requestJson<ApiEnvelope<null>>(`/admin/tts-configs/${id}`, {
    method: 'PUT',
    token,
    body: payload,
  })

  if (!isSuccessCode(response.code)) {
    throw new Error(response.message || '更新 TTS 配置失败')
  }
}

/**
 * 删除指定的 TTS 配置记录。
 */
async function deleteTTSConfig(token: string | null, id: number): Promise<void> {
  const response = await requestJson<ApiEnvelope<null>>(`/admin/tts-configs/${id}`, {
    method: 'DELETE',
    token,
  })

  if (!isSuccessCode(response.code)) {
    throw new Error(response.message || '删除 TTS 配置失败')
  }
}

/**
 * 更新不同场景的默认 TTS 绑定。
 */
async function updateTTSSceneDefaults(token: string | null, payload: Record<string, unknown>): Promise<void> {
  const response = await requestJson<ApiEnvelope<null>>('/admin/tts-configs/defaults', {
    method: 'PUT',
    token,
    body: payload,
  })

  if (!isSuccessCode(response.code)) {
    throw new Error(response.message || '更新场景默认 TTS 绑定失败')
  }
}

/**
 * 返回适合新建配置时使用的默认供应商。
 */
function resolveDefaultProvider(providers: TTSProviderDescriptor[]): TTSProviderDescriptor | undefined {
  return providers.find((provider) => provider.support_status === 'supported') || providers[0]
}

/**
 * 构造新建 TTS 表单的默认值。
 */
function buildInitialTTSForm(providers: TTSProviderDescriptor[]): TTSFormState {
  const provider = resolveDefaultProvider(providers)
  return {
    name: '',
    engine: provider?.key || 'volcengine',
    voiceId: '',
    authConfigJson: provider?.auth_template || '{}',
    paramsJson: provider?.params_template || '{}',
    isActive: true,
    sortOrder: '0',
  }
}

/**
 * 将数据库中的 TTS 记录转换成前端可编辑表单。
 */
function buildTTSForm(config: TTSConfig, providers: TTSProviderDescriptor[]): TTSFormState {
  const provider = providers.find((item) => item.key === config.engine)
  return {
    name: config.name,
    engine: config.engine,
    voiceId: config.voice_id,
    authConfigJson: config.auth_config_json || provider?.auth_template || '{}',
    paramsJson: config.params_json || provider?.params_template || '{}',
    isActive: config.is_active,
    sortOrder: String(config.sort_order),
  }
}

/**
 * 将表单状态转换为后端请求体。
 */
function buildTTSPayload(form: TTSFormState): Record<string, unknown> {
  return {
    name: form.name.trim(),
    engine: form.engine,
    voice_id: form.voiceId.trim(),
    auth_config_json: form.authConfigJson.trim(),
    params_json: form.paramsJson.trim(),
    is_active: form.isActive,
    sort_order: Number(form.sortOrder) || 0,
  }
}

/**
 * 把后台默认绑定转换为前端编辑态。
 */
function buildDefaultBindingsForm(defaultBindings?: Partial<Record<TTSScene, number>>): SceneDefaultFormState {
  return {
    interview: String(defaultBindings?.interview || 0),
    companion: String(defaultBindings?.companion || 0),
  }
}

/**
 * 把默认绑定编辑态转换为后端请求体。
 */
function buildDefaultBindingsPayload(form: SceneDefaultFormState): Record<string, unknown> {
  return {
    default_bindings: {
      interview: Number(form.interview) || 0,
      companion: Number(form.companion) || 0,
    },
  }
}

/**
 * 解析 JSON 对象文本并给出格式化预览。
 */
function buildJSONPreview(rawJson: string): JSONPreview {
  const trimmed = rawJson.trim()
  if (!trimmed) {
    return {
      valid: true,
      error: '',
      formattedJson: '{}',
      parsedObject: {},
    }
  }

  try {
    const parsed = JSON.parse(trimmed) as unknown
    if (!parsed || Array.isArray(parsed) || typeof parsed !== 'object') {
      return {
        valid: false,
        error: '配置必须是 JSON 对象，例如 {"api_key":"xxx"}',
        formattedJson: '{}',
        parsedObject: {},
      }
    }

    return {
      valid: true,
      error: '',
      formattedJson: JSON.stringify(parsed, null, 2),
      parsedObject: parsed as Record<string, unknown>,
    }
  } catch (error) {
    return {
      valid: false,
      error: extractErrorMessage(error, 'JSON 解析失败'),
      formattedJson: '{}',
      parsedObject: {},
    }
  }
}

/**
 * 构造更贴近运行时实际效果的参数预览，并把上方 Voice ID 合并到供应商最终会消费的字段里。
 */
function buildEffectiveParamsPreview(form: TTSFormState, rawPreview: JSONPreview): JSONPreview {
  if (!rawPreview.valid) {
    return rawPreview
  }

  const payload: Record<string, unknown> = {
    ...rawPreview.parsedObject,
  }
  const voiceId = form.voiceId.trim()
  if (voiceId) {
    switch (form.engine) {
      case 'xiaomi_mimo':
        payload.voice = voiceId
        break
      case 'volcengine':
        payload.voice_type = voiceId
        break
      default:
        payload.voice_id = voiceId
        break
    }
  }

  return {
    valid: true,
    error: '',
    formattedJson: JSON.stringify(payload, null, 2),
    parsedObject: payload,
  }
}

/**
 * 返回更适合后台列表展示的供应商标签。
 */
function ttsEngineLabel(engine: string, providerMap: Map<string, TTSProviderDescriptor>): string {
  return providerMap.get(engine)?.label || engine
}

/**
 * 返回更适合后台展示的场景标签。
 */
function ttsSceneLabel(scene: TTSScene): string {
  return scene === 'interview' ? '面试' : '学习陪伴'
}

/**
 * 把状态值格式化为更容易理解的中文标签。
 */
function supportStatusLabel(status: string): string {
  switch (status) {
    case 'ready':
      return '可运行'
    case 'planned':
      return '预留中'
    case 'invalid':
      return '参数有误'
    case 'legacy_unsupported':
      return '遗留不可运行'
    default:
      return status || '未知状态'
  }
}

const STATUS_COLOR: Record<string, string> = {
  ready: 'success',
  planned: 'warning',
  invalid: 'error',
  legacy_unsupported: 'default',
}

/**
 * 截断过长的音色 ID，减少列表区阅读负担。
 */
function shortenVoiceId(value: string): string {
  const trimmed = value.trim()
  if (trimmed.length <= 48) {
    return trimmed
  }

  return `${trimmed.slice(0, 48)}...`
}

/**
 * 把字段定义列表拼成简洁的说明文本。
 */
function formatFieldDefinitions(fields: TTSProviderFieldDefinition[]): string {
  if (fields.length === 0) {
    return '当前没有额外字段说明。'
  }

  return fields
    .map((field) => `${field.required ? '必填' : '可选'} ${field.label} (${field.key})：${field.description}`)
    .join('\n')
}

/**
 * 校验当前 TTS 表单，提前发现必填项和 JSON 格式问题。
 */
function validateTTSForm(form: TTSFormState, authPreview: JSONPreview, paramsPreview: JSONPreview): string {
  if (!form.name.trim()) {
    return '音色名称不能为空'
  }
  if (!form.voiceId.trim()) {
    return 'Voice ID 不能为空'
  }
  if (!authPreview.valid) {
    return authPreview.error
  }
  if (!paramsPreview.valid) {
    return paramsPreview.error
  }

  return ''
}

/**
 * 提供后台 TTS 配置管理页，支持供应商目录、默认绑定和 Live2D 复用场景。
 */
export function TTSPage() {
  const accessToken = useAdminAuthStore((state) => state.accessToken)
  const queryClient = useQueryClient()
  const [selectedConfigId, setSelectedConfigId] = useState<number | null>(null)
  const [form, setForm] = useState<TTSFormState>(buildInitialTTSForm([]))
  const [defaultBindings, setDefaultBindings] = useState<SceneDefaultFormState>({
    interview: '0',
    companion: '0',
  })
  const [messageText, setMessageText] = useState('读取 TTS 配置中')

  const configsQuery = useQuery({
    queryKey: ['admin', 'tts-configs', accessToken],
    queryFn: () => fetchTTSConfigs(accessToken),
    enabled: Boolean(accessToken),
  })

  const providerMap = useMemo(() => {
    return new Map((configsQuery.data?.providers || []).map((provider) => [provider.key, provider]))
  }, [configsQuery.data?.providers])
  const currentProvider = useMemo(() => providerMap.get(form.engine), [form.engine, providerMap])
  const authPreview = useMemo(() => buildJSONPreview(form.authConfigJson), [form.authConfigJson])
  const paramsPreview = useMemo(() => buildJSONPreview(form.paramsJson), [form.paramsJson])
  const effectiveParamsPreview = useMemo(() => buildEffectiveParamsPreview(form, paramsPreview), [form, paramsPreview])
  const formError = useMemo(() => validateTTSForm(form, authPreview, paramsPreview), [authPreview, form, paramsPreview])
  const readyConfigOptions = useMemo(() => {
    return (configsQuery.data?.configs || []).filter((config) => config.support_status === 'supported')
  }, [configsQuery.data?.configs])

  useEffect(() => {
    if (!configsQuery.data) {
      return
    }

    setDefaultBindings(buildDefaultBindingsForm(configsQuery.data.default_bindings))

    if (selectedConfigId === null) {
      setForm((current) => {
        if (current.engine || (configsQuery.data.providers || []).length === 0) {
          return current
        }
        return buildInitialTTSForm(configsQuery.data.providers || [])
      })
      setMessageText((current) => (current === '读取 TTS 配置中' ? '已同步 TTS 配置列表。' : current))
      return
    }

    const nextConfig = (configsQuery.data.configs || []).find((item) => item.id === selectedConfigId)
    if (nextConfig) {
      setForm(buildTTSForm(nextConfig, configsQuery.data.providers || []))
    }
  }, [configsQuery.data, selectedConfigId])

  const saveMutation = useMutation({
    mutationFn: async () => {
      const payload = buildTTSPayload(form)

      if (selectedConfigId) {
        await updateTTSConfig(accessToken, selectedConfigId, payload)
        return selectedConfigId
      }

      const created = await createTTSConfig(accessToken, payload)
      return created?.id
    },
    onSuccess: async (configId) => {
      setSelectedConfigId(configId)
      setMessageText(selectedConfigId ? 'TTS 配置已更新。' : 'TTS 配置已创建。')
      await queryClient.invalidateQueries({
        queryKey: ['admin', 'tts-configs'],
      })
    },
    onError: (error) => {
      setMessageText(extractErrorMessage(error, '保存 TTS 配置失败，请稍后重试'))
    },
  })

  const saveDefaultsMutation = useMutation({
    mutationFn: async () => {
      await updateTTSSceneDefaults(accessToken, buildDefaultBindingsPayload(defaultBindings))
    },
    onSuccess: async () => {
      setMessageText('场景默认 TTS 绑定已更新。')
      await queryClient.invalidateQueries({
        queryKey: ['admin', 'tts-configs'],
      })
    },
    onError: (error) => {
      setMessageText(extractErrorMessage(error, '更新场景默认 TTS 绑定失败，请稍后重试'))
    },
  })

  const deleteMutation = useMutation({
    mutationFn: async (id: number) => {
      await deleteTTSConfig(accessToken, id)
    },
    onSuccess: async () => {
      setSelectedConfigId(null)
      setForm(buildInitialTTSForm(configsQuery.data?.providers || []))
      setMessageText('TTS 配置已删除。')
      await queryClient.invalidateQueries({
        queryKey: ['admin', 'tts-configs'],
      })
    },
    onError: (error) => {
      setMessageText(extractErrorMessage(error, '删除 TTS 配置失败，请稍后重试'))
    },
  })

  /**
   * 切换到新建模式，并清空当前 TTS 表单。
   */
  function startCreatingTTSConfig(): void {
    setSelectedConfigId(null)
    setForm(buildInitialTTSForm(configsQuery.data?.providers || []))
    setMessageText('已切换到新建 TTS 配置模式。')
  }

  /**
   * 将指定的 TTS 配置装载到右侧编辑区。
   */
  function startEditingTTSConfig(config: TTSConfig): void {
    setSelectedConfigId(config.id)
    setForm(buildTTSForm(config, configsQuery.data?.providers || []))
    setMessageText(`正在编辑音色：${config.name}`)
  }

  /**
   * 更新 TTS 表单字段，集中处理输入状态。
   */
  function updateTTSField<Key extends keyof TTSFormState>(key: Key, value: TTSFormState[Key]): void {
    setForm((current) => ({
      ...current,
      [key]: value,
    }))
  }

  /**
   * 切换供应商，并在新建态优先回填该供应商的模板配置。
   */
  function handleEngineChange(nextEngine: string): void {
    const provider = providerMap.get(nextEngine)
    setForm((current) => ({
      ...current,
      engine: nextEngine,
      authConfigJson:
        selectedConfigId && current.engine === nextEngine ? current.authConfigJson : provider?.auth_template || '{}',
      paramsJson:
        selectedConfigId && current.engine === nextEngine ? current.paramsJson : provider?.params_template || '{}',
    }))
  }

  /**
   * 提交 TTS 表单并执行创建或更新。
   */
  function handleSubmit(event: FormEvent<HTMLFormElement>): void {
    event.preventDefault()

    if (formError) {
      setMessageText(formError)
      return
    }

    setMessageText(selectedConfigId ? '正在更新 TTS 配置。' : '正在创建 TTS 配置。')
    saveMutation.mutate()
  }

  /**
   * 删除当前选中的 TTS 配置。
   */
  function handleDelete(): void {
    if (!selectedConfigId) {
      return
    }

    Modal.confirm({
      title: '确认删除',
      icon: <ExclamationCircleOutlined />,
      content: '确认删除当前 TTS 配置吗？删除后不可恢复。',
      okText: '删除',
      okType: 'danger',
      cancelText: '取消',
      onOk: () => {
        setMessageText('正在删除 TTS 配置。')
        deleteMutation.mutate(selectedConfigId)
      },
    })
  }

  /**
   * 保存场景默认 TTS 绑定。
   */
  function handleSaveDefaults(): void {
    setMessageText('正在更新场景默认 TTS 绑定。')
    saveDefaultsMutation.mutate()
  }

  if (configsQuery.isLoading) {
    return (
      <div style={{ padding: '24px 32px 32px', background: THEME.bg, minHeight: '100vh' }}>
        <div style={{ ...glassCard, padding: '24px 28px' }}>
          <h2 style={{ margin: 0, fontSize: 22, fontWeight: 700, color: THEME.textMain }}>TTS 配置</h2>
          <p style={{ margin: '8px 0 0', color: THEME.textSecondary }}>正在加载后台 TTS 配置列表...</p>
        </div>
      </div>
    )
  }

  if (configsQuery.isError || !configsQuery.data) {
    return (
      <div style={{ padding: '24px 32px 32px', background: THEME.bg, minHeight: '100vh' }}>
        <div style={{ ...glassCard, padding: '24px 28px' }}>
          <h2 style={{ margin: 0, fontSize: 22, fontWeight: 700, color: THEME.textMain }}>TTS 配置</h2>
          <p style={{ margin: '8px 0 0', color: THEME.danger }}>{extractErrorMessage(configsQuery.error, '读取 TTS 配置失败')}</p>
        </div>
      </div>
    )
  }

  return (
    <div style={{ padding: '24px 32px 32px', background: THEME.bg, minHeight: '100vh' }}>
      {/* Header */}
      <div
        style={{
          ...glassCard,
          padding: '24px 28px',
          marginBottom: 20,
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          gap: 16,
          flexWrap: 'wrap',
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', gap: 16 }}>
          <div
            style={{
              width: 48,
              height: 48,
              borderRadius: 14,
              background: 'linear-gradient(135deg, #10b981, #059669)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              boxShadow: '0 4px 14px rgba(16, 185, 129, 0.35)',
              flexShrink: 0,
            }}
          >
            <SoundOutlined style={{ fontSize: 22, color: '#fff' }} />
          </div>
          <div>
            <h1 style={{ margin: 0, fontSize: 22, fontWeight: 700, color: THEME.textMain, lineHeight: 1.3 }}>
              TTS 配置
            </h1>
            <p style={{ margin: '4px 0 0', fontSize: 13, color: THEME.textSecondary }}>
              维护 TTS 供应商配置，支持场景默认绑定和 Live2D 复用
            </p>
          </div>
        </div>

        <div style={{ display: 'flex', gap: 12, flexWrap: 'wrap' }}>
          <div
            style={{
              ...solidCard,
              padding: '12px 20px',
              display: 'flex',
              alignItems: 'center',
              gap: 8,
              minWidth: 120,
            }}
          >
            <span style={{ fontSize: 24, fontWeight: 700, color: THEME.primary }}>{(configsQuery.data.configs || []).length}</span>
            <span style={{ fontSize: 12, color: THEME.textSecondary }}>条配置</span>
          </div>
        </div>
      </div>

      {/* Scene Defaults Toolbar */}
      <div
        style={{
          ...solidCard,
          padding: '16px 20px',
          marginBottom: 20,
        }}
      >
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            gap: 12,
            flexWrap: 'wrap',
            marginBottom: 12,
          }}
        >
          <div style={{ display: 'flex', alignItems: 'center', gap: 8, flex: '1 1 280px' }}>
            <span style={{ fontSize: 13, fontWeight: 500, color: THEME.textSecondary, whiteSpace: 'nowrap' }}>
              {ttsSceneLabel('interview')}默认 TTS
            </span>
            <Select
              value={defaultBindings.interview}
              onChange={(v) => setDefaultBindings((current) => ({ ...current, interview: v }))}
              style={{ flex: 1 }}
              options={[
                { value: '0', label: '未设置，回退到 config.yaml' },
                ...readyConfigOptions.map((config) => ({
                  value: String(config.id),
                  label: config.name,
                })),
              ]}
            />
          </div>
          <div style={{ display: 'flex', alignItems: 'center', gap: 8, flex: '1 1 280px' }}>
            <span style={{ fontSize: 13, fontWeight: 500, color: THEME.textSecondary, whiteSpace: 'nowrap' }}>
              {ttsSceneLabel('companion')}默认 TTS
            </span>
            <Select
              value={defaultBindings.companion}
              onChange={(v) => setDefaultBindings((current) => ({ ...current, companion: v }))}
              style={{ flex: 1 }}
              options={[
                { value: '0', label: '未设置，回退到 config.yaml' },
                ...readyConfigOptions.map((config) => ({
                  value: String(config.id),
                  label: config.name,
                })),
              ]}
            />
          </div>
          <Button
            type="primary"
            icon={<SaveOutlined />}
            onClick={handleSaveDefaults}
            loading={saveDefaultsMutation.isPending}
            style={{
              borderRadius: 10,
              background: 'linear-gradient(135deg, #3b82f6, #2563eb)',
              border: 'none',
              boxShadow: '0 4px 14px rgba(37, 99, 235, 0.35)',
            }}
          >
            保存场景默认绑定
          </Button>
          <Button
            icon={<PlusOutlined />}
            onClick={startCreatingTTSConfig}
            style={{ borderRadius: 10 }}
          >
            新建 TTS 配置
          </Button>
        </div>
      </div>

      {/* Main Content */}
      <div style={{ display: 'flex', gap: 20, flexWrap: 'wrap' }}>
        {/* Left: Config List */}
        <div style={{ flex: '1 1 340px', maxWidth: 420, minWidth: 300 }}>
          <div
            style={{
              ...solidCard,
              overflow: 'hidden',
              display: 'flex',
              flexDirection: 'column',
              height: 'calc(100vh - 300px)',
              minHeight: 500,
            }}
          >
            <div
              style={{
                padding: '16px 20px',
                borderBottom: '1px solid ' + THEME.border,
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'space-between',
              }}
            >
              <span style={{ fontSize: 15, fontWeight: 600, color: THEME.textMain }}>配置列表</span>
              <span style={{ fontSize: 12, color: THEME.textMuted }}>
                共 {(configsQuery.data.configs || []).length} 条
              </span>
            </div>

            <div style={{ flex: 1, overflowY: 'auto', padding: '8px' }}>
              {(configsQuery.data.configs || []).length === 0 ? (
                <div
                  style={{
                    height: '100%',
                    display: 'flex',
                    flexDirection: 'column',
                    alignItems: 'center',
                    justifyContent: 'center',
                    color: THEME.textMuted,
                    gap: 12,
                  }}
                >
                  <InboxOutlined style={{ fontSize: 40 }} />
                  <span style={{ fontSize: 14 }}>当前还没有 TTS 配置记录</span>
                  <span style={{ fontSize: 12 }}>可以先新建一条可运行的供应商配置</span>
                </div>
              ) : (
                <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
                  {(configsQuery.data.configs || []).map((config) => {
                    const isActive = selectedConfigId === config.id
                    return (
                      <div
                        key={config.id}
                        role="button"
                        tabIndex={0}
                        onClick={() => startEditingTTSConfig(config)}
                        onKeyDown={(e) => {
                          if (e.key === 'Enter' || e.key === ' ') {
                            startEditingTTSConfig(config)
                          }
                        }}
                        style={{
                          padding: '14px 16px',
                          borderRadius: 12,
                          cursor: 'pointer',
                          border: isActive
                            ? '1.5px solid ' + THEME.primary
                            : '1.5px solid transparent',
                          background: isActive ? '#f5f3ff' : '#fafafa',
                          transition: 'all 0.2s ease',
                          position: 'relative',
                          overflow: 'hidden',
                        }}
                        onMouseEnter={(e) => {
                          if (!isActive) {
                            e.currentTarget.style.background = '#f1f5f9'
                          }
                        }}
                        onMouseLeave={(e) => {
                          if (!isActive) {
                            e.currentTarget.style.background = '#fafafa'
                          }
                        }}
                      >
                        {isActive && (
                          <div
                            style={{
                              position: 'absolute',
                              left: 0,
                              top: '12px',
                              bottom: '12px',
                              width: 3,
                              borderRadius: '0 3px 3px 0',
                              background: THEME.primary,
                            }}
                          />
                        )}
                        <div
                          style={{
                            display: 'flex',
                            alignItems: 'center',
                            justifyContent: 'space-between',
                            gap: 8,
                            marginBottom: 6,
                          }}
                        >
                          <span
                            style={{
                              fontWeight: 600,
                              fontSize: 14,
                              color: THEME.textMain,
                              lineHeight: 1.4,
                            }}
                          >
                            {config.name}
                          </span>
                          <Tag
                            color={config.is_active ? 'success' : 'default'}
                            style={{ fontSize: 11, padding: '0 6px', margin: 0, flexShrink: 0 }}
                          >
                            {config.is_active ? '启用中' : '已停用'}
                          </Tag>
                        </div>
                        <div
                          style={{
                            display: 'flex',
                            alignItems: 'center',
                            gap: 8,
                            fontSize: 12,
                            color: THEME.textSecondary,
                            flexWrap: 'wrap',
                          }}
                        >
                          <span>{ttsEngineLabel(config.engine, providerMap)}</span>
                          <Tag
                            color={STATUS_COLOR[config.support_status] || 'default'}
                            style={{ fontSize: 11, margin: 0 }}
                          >
                            {supportStatusLabel(config.support_status)}
                          </Tag>
                          <span style={{ color: THEME.textMuted }}>排序 {config.sort_order}</span>
                        </div>
                        <Tooltip title={config.voice_id}>
                          <p
                            style={{
                              margin: '6px 0 0',
                              fontSize: 12,
                              color: THEME.textMuted,
                              fontFamily: 'monospace',
                              overflow: 'hidden',
                              textOverflow: 'ellipsis',
                              whiteSpace: 'nowrap',
                            }}
                          >
                            {shortenVoiceId(config.voice_id)}
                          </p>
                        </Tooltip>
                      </div>
                    )
                  })}
                </div>
              )}
            </div>
          </div>
        </div>

        {/* Right: Editor */}
        <div style={{ flex: '2 1 520px', minWidth: 360 }}>
          <div
            style={{
              ...solidCard,
              overflow: 'hidden',
              display: 'flex',
              flexDirection: 'column',
              height: 'calc(100vh - 300px)',
              minHeight: 500,
            }}
          >
            <div
              style={{
                padding: '16px 20px',
                borderBottom: '1px solid ' + THEME.border,
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'space-between',
              }}
            >
              <div>
                <span style={{ fontSize: 15, fontWeight: 600, color: THEME.textMain }}>
                  {selectedConfigId ? '编辑 TTS 配置' : '新建 TTS 配置'}
                </span>
                <div style={{ fontSize: 12, color: THEME.textMuted, marginTop: 2 }}>{messageText}</div>
              </div>
              <Tag
                style={{
                  fontSize: 12,
                  padding: '2px 10px',
                  color: selectedConfigId ? THEME.primary : THEME.success,
                  background: selectedConfigId ? THEME.primaryLight : '#dcfce7',
                  border: 'none',
                }}
              >
                {selectedConfigId ? `ID #${selectedConfigId}` : '新配置'}
              </Tag>
            </div>

            <form
              onSubmit={handleSubmit}
              style={{
                flex: 1,
                overflowY: 'auto',
                padding: '20px',
                display: 'flex',
                flexDirection: 'column',
                gap: 16,
              }}
            >
              <div>
                <label
                  style={{
                    display: 'block',
                    fontSize: 13,
                    fontWeight: 500,
                    color: THEME.textSecondary,
                    marginBottom: 6,
                  }}
                >
                  配置名称
                </label>
                <Input
                  value={form.name}
                  onChange={(e) => updateTTSField('name', e.target.value)}
                  placeholder="例如 豆包陪伴女声"
                  style={{ borderRadius: 10 }}
                />
              </div>

              <div style={{ display: 'grid', gridTemplateColumns: '1fr 120px', gap: 16 }}>
                <div>
                  <label
                    style={{
                      display: 'block',
                      fontSize: 13,
                      fontWeight: 500,
                      color: THEME.textSecondary,
                      marginBottom: 6,
                    }}
                  >
                    供应商
                  </label>
                  <Select
                    value={form.engine}
                    onChange={(v) => handleEngineChange(v)}
                    style={{ width: '100%' }}
                    options={(configsQuery.data.providers || []).map((p) => ({
                      value: p.key,
                      label: p.label,
                    }))}
                  />
                </div>
                <div>
                  <label
                    style={{
                      display: 'block',
                      fontSize: 13,
                      fontWeight: 500,
                      color: THEME.textSecondary,
                      marginBottom: 6,
                    }}
                  >
                    排序权重
                  </label>
                  <InputNumber
                    value={Number(form.sortOrder)}
                    onChange={(val) => updateTTSField('sortOrder', val !== null && val !== undefined ? String(val) : '0')}
                    style={{ width: '100%', borderRadius: 10 }}
                  />
                </div>
              </div>

              <div>
                <label
                  style={{
                    display: 'block',
                    fontSize: 13,
                    fontWeight: 500,
                    color: THEME.textSecondary,
                    marginBottom: 6,
                  }}
                >
                  Voice ID / Speaker
                </label>
                <Input
                  value={form.voiceId}
                  onChange={(e) => updateTTSField('voiceId', e.target.value)}
                  placeholder="请输入当前供应商的音色或说话人 ID"
                  style={{ borderRadius: 10 }}
                />
              </div>

              {/* Provider Status */}
              <div
                style={{
                  padding: '12px 16px',
                  borderRadius: 10,
                  background: currentProvider?.support_status === 'supported' ? '#f0fdf4' : '#fef2f2',
                  border: `1px solid ${currentProvider?.support_status === 'supported' ? '#bbf7d0' : '#fecaca'}`,
                  display: 'flex',
                  alignItems: 'center',
                  gap: 10,
                }}
              >
                {currentProvider?.support_status === 'supported' ? (
                  <CheckCircleOutlined style={{ fontSize: 18, color: THEME.success }} />
                ) : (
                  <ExclamationCircleOutlined style={{ fontSize: 18, color: THEME.danger }} />
                )}
                <div>
                  <div style={{ fontSize: 12, fontWeight: 600, color: THEME.textMain }}>供应商状态</div>
                  <div style={{ fontSize: 12, color: THEME.textSecondary }}>
                    {currentProvider?.support_message || '当前供应商元数据缺失。'}
                  </div>
                </div>
              </div>

              <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 16 }}>
                <div>
                  <label
                    style={{
                      display: 'block',
                      fontSize: 13,
                      fontWeight: 500,
                      color: THEME.textSecondary,
                      marginBottom: 6,
                    }}
                  >
                    鉴权配置 JSON
                  </label>
                  <Input.TextArea
                    value={form.authConfigJson}
                    onChange={(e) => updateTTSField('authConfigJson', e.target.value)}
                    placeholder='例如 {"api_key":"xxx"}'
                    rows={6}
                    style={{
                      borderRadius: 10,
                      fontFamily: 'monospace',
                      fontSize: 13,
                      borderColor: authPreview.valid ? undefined : THEME.danger,
                    }}
                  />
                  {!authPreview.valid && (
                    <div style={{ marginTop: 4, fontSize: 12, color: THEME.danger }}>{authPreview.error}</div>
                  )}
                </div>
                <div>
                  <label
                    style={{
                      display: 'block',
                      fontSize: 13,
                      fontWeight: 500,
                      color: THEME.textSecondary,
                      marginBottom: 6,
                    }}
                  >
                    供应商参数 JSON
                  </label>
                  <Input.TextArea
                    value={form.paramsJson}
                    onChange={(e) => updateTTSField('paramsJson', e.target.value)}
                    placeholder='例如 {"resource_id":"seed-tts-2.0"}'
                    rows={6}
                    style={{
                      borderRadius: 10,
                      fontFamily: 'monospace',
                      fontSize: 13,
                      borderColor: paramsPreview.valid ? undefined : THEME.danger,
                    }}
                  />
                  {!paramsPreview.valid && (
                    <div style={{ marginTop: 4, fontSize: 12, color: THEME.danger }}>{paramsPreview.error}</div>
                  )}
                </div>
              </div>

              {/* Form Check */}
              <div
                style={{
                  padding: '12px 16px',
                  borderRadius: 10,
                  background: formError ? '#fef2f2' : '#f0fdf4',
                  border: `1px solid ${formError ? '#fecaca' : '#bbf7d0'}`,
                  display: 'flex',
                  alignItems: 'center',
                  gap: 10,
                }}
              >
                {formError ? (
                  <ExclamationCircleOutlined style={{ fontSize: 18, color: THEME.danger }} />
                ) : (
                  <CheckCircleOutlined style={{ fontSize: 18, color: THEME.success }} />
                )}
                <div>
                  <div style={{ fontSize: 12, fontWeight: 600, color: THEME.textMain }}>表单检查</div>
                  <div style={{ fontSize: 12, color: THEME.textSecondary }}>
                    {formError || '当前 TTS 表单已通过基础校验，可以提交保存。'}
                  </div>
                </div>
              </div>

              {/* JSON Previews */}
              <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 16 }}>
                <div>
                  <div
                    style={{
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'space-between',
                      marginBottom: 6,
                    }}
                  >
                    <span style={{ fontSize: 12, fontWeight: 600, color: THEME.textSecondary }}>鉴权配置预览</span>
                    <Tag
                      color={authPreview.valid ? 'success' : 'error'}
                      style={{ fontSize: 11, margin: 0 }}
                    >
                      {authPreview.valid ? '已通过 JSON 校验' : '回退空对象'}
                    </Tag>
                  </div>
                  <pre
                    style={{
                      margin: 0,
                      padding: 12,
                      borderRadius: 10,
                      background: '#0f172a',
                      color: '#e2e8f0',
                      fontSize: 12,
                      lineHeight: 1.6,
                      overflowX: 'auto',
                      maxHeight: 160,
                      overflowY: 'auto',
                    }}
                  >
                    {authPreview.formattedJson}
                  </pre>
                </div>
                <div>
                  <div
                    style={{
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'space-between',
                      marginBottom: 6,
                    }}
                  >
                    <span style={{ fontSize: 12, fontWeight: 600, color: THEME.textSecondary }}>参数配置预览</span>
                    <Tag
                      color={paramsPreview.valid ? 'success' : 'error'}
                      style={{ fontSize: 11, margin: 0 }}
                    >
                      {paramsPreview.valid ? '已合并 Voice ID' : '回退空对象'}
                    </Tag>
                  </div>
                  <pre
                    style={{
                      margin: 0,
                      padding: 12,
                      borderRadius: 10,
                      background: '#0f172a',
                      color: '#e2e8f0',
                      fontSize: 12,
                      lineHeight: 1.6,
                      overflowX: 'auto',
                      maxHeight: 160,
                      overflowY: 'auto',
                    }}
                  >
                    {effectiveParamsPreview.formattedJson}
                  </pre>
                </div>
              </div>

              {/* Field Definitions */}
              <div>
                <div
                  style={{
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'space-between',
                    marginBottom: 6,
                  }}
                >
                  <span style={{ fontSize: 12, fontWeight: 600, color: THEME.textSecondary }}>官方字段提示</span>
                  <span style={{ fontSize: 11, color: THEME.textMuted }}>{currentProvider?.label || '未知供应商'}</span>
                </div>
                <pre
                  style={{
                    margin: 0,
                    padding: 14,
                    borderRadius: 10,
                    background: '#f8fafc',
                    color: THEME.textSecondary,
                    fontSize: 12,
                    lineHeight: 1.8,
                    overflowX: 'auto',
                    maxHeight: 180,
                    overflowY: 'auto',
                    border: '1px solid ' + THEME.border,
                  }}
                >
                  {`鉴权字段：\n${formatFieldDefinitions(currentProvider?.auth_fields || [])}\n\n参数字段：\n${formatFieldDefinitions(currentProvider?.param_fields || [])}`}
                </pre>
              </div>

              <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
                <Switch
                  checked={form.isActive}
                  onChange={(checked) => updateTTSField('isActive', checked)}
                />
                <span style={{ fontSize: 13, color: THEME.textSecondary }}>
                  {form.isActive ? '当前配置启用中' : '当前配置已停用'}
                </span>
              </div>

              <div style={{ display: 'flex', gap: 10, flexWrap: 'wrap', paddingTop: 4 }}>
                <Button
                  icon={<ReloadOutlined />}
                  onClick={startCreatingTTSConfig}
                  disabled={saveMutation.isPending || deleteMutation.isPending || saveDefaultsMutation.isPending}
                  style={{ borderRadius: 10 }}
                >
                  重置为新建
                </Button>
                {selectedConfigId && (
                  <Button
                    danger
                    icon={<DeleteOutlined />}
                    onClick={handleDelete}
                    loading={deleteMutation.isPending}
                    disabled={saveMutation.isPending || deleteMutation.isPending || saveDefaultsMutation.isPending}
                    style={{ borderRadius: 10 }}
                  >
                    删除配置
                  </Button>
                )}
                <Button
                  type="primary"
                  htmlType="submit"
                  icon={<SaveOutlined />}
                  loading={saveMutation.isPending}
                  disabled={Boolean(formError) || saveMutation.isPending || deleteMutation.isPending || saveDefaultsMutation.isPending}
                  style={{
                    borderRadius: 10,
                    background: 'linear-gradient(135deg, #3b82f6, #2563eb)',
                    border: 'none',
                    boxShadow: '0 4px 14px rgba(37, 99, 235, 0.35)',
                  }}
                >
                  {saveMutation.isPending ? '保存中...' : selectedConfigId ? '保存修改' : '创建配置'}
                </Button>
              </div>
            </form>
          </div>
        </div>
      </div>
    </div>
  )
}
