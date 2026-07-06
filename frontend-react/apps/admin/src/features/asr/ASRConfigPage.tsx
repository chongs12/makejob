import type { FormEvent } from 'react'
import { useEffect, useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  AudioOutlined,
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

// ---------- 类型定义 ----------

interface ASRProviderFieldDefinition {
  key: string
  label: string
  description: string
  required: boolean
  secret?: boolean
}

interface ASRProviderDescriptor {
  key: string
  label: string
  description: string
  auth_template: string
  params_template: string
  auth_fields: ASRProviderFieldDefinition[]
  param_fields: ASRProviderFieldDefinition[]
}

interface ASRConfig {
  id: number
  name: string
  engine: string
  auth_config_json: string
  params_json: string
  is_active: boolean
  sort_order: number
  created_at?: string
  updated_at?: string
}

interface ASRConfigListResponse {
  configs: ASRConfig[]
  providers: ASRProviderDescriptor[]
  default_config_id: number
}

interface ASRFormState {
  name: string
  engine: string
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

// ---------- 供应商定义 ----------

const ASR_PROVIDERS: ASRProviderDescriptor[] = [
  {
    key: 'volcengine',
    label: '火山引擎 (Volcengine)',
    description: '字节跳动旗下语音识别服务，支持流式和批量识别，中文效果优秀。',
    auth_template: JSON.stringify({ app_id: '', access_token: '' }, null, 2),
    params_template: JSON.stringify({ cluster: 'volcengine_streaming_common', resource_id: '', workflow: '', audio_format: 'pcm', sample_rate: 16000, language: 'zh-CN' }, null, 2),
    auth_fields: [
      { key: 'app_id', label: 'App ID', description: '火山语音控制台的 App ID', required: true },
      { key: 'access_token', label: 'Access Token', description: '火山语音控制台的 API Key', required: true, secret: true },
    ],
    param_fields: [
      { key: 'cluster', label: 'Cluster', description: '集群标识，默认 volcengine_streaming_common', required: false },
      { key: 'resource_id', label: 'Resource ID', description: '资源 ID（可选）', required: false },
      { key: 'workflow', label: 'Workflow', description: '工作流 ID（可选）', required: false },
      { key: 'audio_format', label: '音频格式', description: 'pcm / wav', required: false },
      { key: 'sample_rate', label: '采样率', description: '默认 16000', required: false },
      { key: 'language', label: '语言', description: 'zh-CN / en-US', required: false },
    ],
  },
  {
    key: 'xiaomi_mimo',
    label: '小米 MiMo ASR',
    description: '小米 MiMo-V2.5-ASR 模型，支持中英双语及方言识别，兼容 OpenAI 接口。',
    auth_template: JSON.stringify({ api_key: '' }, null, 2),
    params_template: JSON.stringify({ model: 'mimo-v2.5-asr', language: 'auto', base_url: 'https://api.xiaomimimo.com/v1' }, null, 2),
    auth_fields: [
      { key: 'api_key', label: 'API Key', description: '小米 MiMo 平台 API Key', required: true, secret: true },
    ],
    param_fields: [
      { key: 'model', label: '模型', description: 'mimo-v2.5-asr', required: false },
      { key: 'language', label: '语言', description: 'auto（自动检测）/ zh / en', required: false },
      { key: 'base_url', label: 'Base URL', description: 'https://api.xiaomimimo.com/v1', required: false },
    ],
  },
  {
    key: 'openai_whisper',
    label: 'OpenAI Whisper',
    description: 'OpenAI Whisper API，支持多语言语音识别，适合非中文场景。',
    auth_template: JSON.stringify({ api_key: '' }, null, 2),
    params_template: JSON.stringify({ model: 'whisper-1', language: 'zh', response_format: 'json' }, null, 2),
    auth_fields: [
      { key: 'api_key', label: 'API Key', description: 'OpenAI API Key', required: true, secret: true },
    ],
    param_fields: [
      { key: 'model', label: '模型', description: 'whisper-1', required: false },
      { key: 'language', label: '语言', description: 'zh / en / ja 等 ISO 代码', required: false },
      { key: 'response_format', label: '返回格式', description: 'json / text / verbose_json', required: false },
    ],
  },
  {
    key: 'azure_speech',
    label: 'Azure Speech',
    description: '微软 Azure 语音识别服务，企业级稳定性和多语言支持。',
    auth_template: JSON.stringify({ subscription_key: '', region: '' }, null, 2),
    params_template: JSON.stringify({ language: 'zh-CN' }, null, 2),
    auth_fields: [
      { key: 'subscription_key', label: 'Subscription Key', description: 'Azure 语音服务订阅密钥', required: true, secret: true },
      { key: 'region', label: 'Region', description: '区域，如 eastasia', required: true },
    ],
    param_fields: [
      { key: 'language', label: '语言', description: 'zh-CN / en-US 等', required: false },
    ],
  },
]

const PROVIDER_MAP = new Map(ASR_PROVIDERS.map((p) => [p.key, p]))

// ---------- 工具函数 ----------

function buildDefaultForm(engine = 'volcengine'): ASRFormState {
  const provider = PROVIDER_MAP.get(engine)
  return {
    name: '',
    engine,
    authConfigJson: provider?.auth_template || '{}',
    paramsJson: provider?.params_template || '{}',
    isActive: true,
    sortOrder: '0',
  }
}

function buildFormFromConfig(config: ASRConfig): ASRFormState {
  return {
    name: config.name,
    engine: config.engine,
    authConfigJson: config.auth_config_json || '{}',
    paramsJson: config.params_json || '{}',
    isActive: config.is_active,
    sortOrder: String(config.sort_order),
  }
}

function buildJSONPreview(rawJson: string): JSONPreview {
  const trimmed = rawJson.trim()
  if (!trimmed) {
    return { valid: true, error: '', formattedJson: '{}', parsedObject: {} }
  }
  try {
    const parsed = JSON.parse(trimmed) as unknown
    if (!parsed || Array.isArray(parsed) || typeof parsed !== 'object') {
      return { valid: false, error: '配置必须是 JSON 对象', formattedJson: '{}', parsedObject: {} }
    }
    return { valid: true, error: '', formattedJson: JSON.stringify(parsed, null, 2), parsedObject: parsed as Record<string, unknown> }
  } catch (error) {
    return { valid: false, error: extractErrorMessage(error, 'JSON 解析失败'), formattedJson: '{}', parsedObject: {} }
  }
}

function validateASRForm(form: ASRFormState, authPreview: JSONPreview, paramsPreview: JSONPreview): string | null {
  if (!form.name.trim()) return '配置名称不能为空'
  if (!form.engine) return '请选择 ASR 引擎'
  if (!authPreview.valid) return `鉴权配置格式错误: ${authPreview.error}`
  if (!paramsPreview.valid) return `参数配置格式错误: ${paramsPreview.error}`

  const provider = PROVIDER_MAP.get(form.engine)
  if (provider) {
    for (const field of provider.auth_fields) {
      if (field.required && !authPreview.parsedObject[field.key]) {
        return `鉴权配置缺少必填字段: ${field.label}`
      }
    }
  }
  return null
}

function buildPayload(form: ASRFormState): Record<string, unknown> {
  return {
    name: form.name.trim(),
    engine: form.engine,
    auth_config_json: form.authConfigJson.trim(),
    params_json: form.paramsJson.trim(),
    is_active: form.isActive,
    sort_order: Number(form.sortOrder) || 0,
  }
}

// ---------- 主题 ----------

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
  background: THEME.cardBg,
  borderRadius: THEME.radius,
  boxShadow: THEME.shadow,
  border: '1px solid ' + THEME.border,
}

const solidCard = {
  background: THEME.cardBg,
  borderRadius: THEME.radius,
  boxShadow: THEME.shadow,
  border: '1px solid ' + THEME.border,
}

// ---------- 页面组件 ----------

export function ASRConfigPage() {
  const accessToken = useAdminAuthStore((state) => state.accessToken)
  const queryClient = useQueryClient()
  const [selectedConfigId, setSelectedConfigId] = useState<number | null>(null)
  const [form, setForm] = useState<ASRFormState>(buildDefaultForm())
  const [messageText, setMessageText] = useState('读取 ASR 配置中')
  const [defaultConfigId, setDefaultConfigId] = useState<string>('0')

  const configsQuery = useQuery({
    queryKey: ['admin-asr-configs', accessToken],
    queryFn: async () => {
      const response = await requestJson<ApiEnvelope<ASRConfigListResponse>>('/admin/asr-configs', { token: accessToken })
      if (!isSuccessCode(response.code)) throw new Error(response.message || '获取 ASR 配置失败')
      return response.data || { configs: [], providers: [] }
    },
    enabled: Boolean(accessToken),
  })

  const currentProvider = useMemo(() => PROVIDER_MAP.get(form.engine), [form.engine])
  const authPreview = useMemo(() => buildJSONPreview(form.authConfigJson), [form.authConfigJson])
  const paramsPreview = useMemo(() => buildJSONPreview(form.paramsJson), [form.paramsJson])
  const formError = useMemo(() => validateASRForm(form, authPreview, paramsPreview), [authPreview, form, paramsPreview])

  useEffect(() => {
    if (!configsQuery.data) return
    setDefaultConfigId(String(configsQuery.data.default_config_id || 0))
    if (selectedConfigId === null) {
      setMessageText((current) => (current === '读取 ASR 配置中' ? '已同步 ASR 配置列表。' : current))
      return
    }
    const nextConfig = configsQuery.data.configs.find((item) => item.id === selectedConfigId)
    if (nextConfig) {
      setForm(buildFormFromConfig(nextConfig))
    }
  }, [configsQuery.data, selectedConfigId])

  const saveDefaultMutation = useMutation({
    mutationFn: async () => {
      await requestJson<ApiEnvelope<unknown>>('/admin/asr-configs/default', {
        method: 'PUT',
        token: accessToken,
        body: { config_id: Number(defaultConfigId) || 0 },
      })
    },
    onSuccess: async () => {
      setMessageText('默认 ASR 配置已更新。')
      await queryClient.invalidateQueries({ queryKey: ['admin-asr-configs'] })
    },
    onError: (error) => {
      setMessageText(extractErrorMessage(error, '更新默认 ASR 配置失败'))
    },
  })

  const saveMutation = useMutation({
    mutationFn: async () => {
      const payload = buildPayload(form)
      if (selectedConfigId) {
        await requestJson<ApiEnvelope<ASRConfig>>(`/admin/asr-configs/${selectedConfigId}`, { method: 'PUT', token: accessToken, body: payload })
        return selectedConfigId
      }
      const response = await requestJson<ApiEnvelope<ASRConfig>>('/admin/asr-configs', { method: 'POST', token: accessToken, body: payload })
      return response.data?.id
    },
    onSuccess: async (configId) => {
      setSelectedConfigId(configId ?? null)
      setMessageText(selectedConfigId ? 'ASR 配置已更新。' : 'ASR 配置已创建。')
      await queryClient.invalidateQueries({ queryKey: ['admin-asr-configs'] })
    },
    onError: (error) => {
      setMessageText(extractErrorMessage(error, '保存 ASR 配置失败'))
    },
  })

  const deleteMutation = useMutation({
    mutationFn: async (id: number) => {
      await requestJson<ApiEnvelope<unknown>>(`/admin/asr-configs/${id}`, { method: 'DELETE', token: accessToken })
    },
    onSuccess: async () => {
      setSelectedConfigId(null)
      setForm(buildDefaultForm())
      setMessageText('ASR 配置已删除。')
      await queryClient.invalidateQueries({ queryKey: ['admin-asr-configs'] })
    },
    onError: (error) => {
      setMessageText(extractErrorMessage(error, '删除 ASR 配置失败'))
    },
  })

  function startCreating(): void {
    setSelectedConfigId(null)
    setForm(buildDefaultForm())
    setMessageText('已切换到新建 ASR 配置模式。')
  }

  function startEditing(config: ASRConfig): void {
    setSelectedConfigId(config.id)
    setForm(buildFormFromConfig(config))
    setMessageText(`正在编辑: ${config.name}`)
  }

  function updateField<Key extends keyof ASRFormState>(key: Key, value: ASRFormState[Key]): void {
    setForm((current) => ({ ...current, [key]: value }))
  }

  function handleEngineChange(nextEngine: string): void {
    const provider = PROVIDER_MAP.get(nextEngine)
    setForm((current) => ({
      ...current,
      engine: nextEngine,
      authConfigJson: selectedConfigId && current.engine === nextEngine ? current.authConfigJson : provider?.auth_template || '{}',
      paramsJson: selectedConfigId && current.engine === nextEngine ? current.paramsJson : provider?.params_template || '{}',
    }))
  }

  function handleSubmit(event: FormEvent<HTMLFormElement>): void {
    event.preventDefault()
    if (formError) { setMessageText(formError); return }
    setMessageText(selectedConfigId ? '正在更新 ASR 配置...' : '正在创建 ASR 配置...')
    saveMutation.mutate()
  }

  function handleDelete(): void {
    if (!selectedConfigId) return
    Modal.confirm({
      title: '确认删除',
      icon: <ExclamationCircleOutlined />,
      content: '确认删除当前 ASR 配置吗？删除后不可恢复。',
      okText: '删除', okType: 'danger', cancelText: '取消',
      onOk: () => { setMessageText('正在删除...'); deleteMutation.mutate(selectedConfigId) },
    })
  }

  // ---------- 渲染 ----------

  if (configsQuery.isLoading) {
    return (
      <div style={{ padding: '24px 32px 32px', background: THEME.bg, minHeight: '100vh' }}>
        <div style={{ ...glassCard, padding: '24px 28px' }}>
          <h2 style={{ margin: 0, fontSize: 22, fontWeight: 700, color: THEME.textMain }}>ASR 语音识别配置</h2>
          <p style={{ margin: '8px 0 0', color: THEME.textSecondary }}>正在加载配置列表...</p>
        </div>
      </div>
    )
  }

  if (configsQuery.isError || !configsQuery.data) {
    return (
      <div style={{ padding: '24px 32px 32px', background: THEME.bg, minHeight: '100vh' }}>
        <div style={{ ...glassCard, padding: '24px 28px' }}>
          <h2 style={{ margin: 0, fontSize: 22, fontWeight: 700, color: THEME.textMain }}>ASR 语音识别配置</h2>
          <p style={{ margin: '8px 0 0', color: THEME.danger }}>{extractErrorMessage(configsQuery.error, '读取 ASR 配置失败')}</p>
        </div>
      </div>
    )
  }

  const configs = configsQuery.data.configs || []

  return (
    <div style={{ padding: '24px 32px 32px', background: THEME.bg, minHeight: '100vh' }}>
      {/* Header */}
      <div style={{ ...glassCard, padding: '24px 28px', marginBottom: 20, display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 16, flexWrap: 'wrap' }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 16 }}>
          <div style={{ width: 48, height: 48, borderRadius: 14, background: 'linear-gradient(135deg, #3b82f6, #2563eb)', display: 'flex', alignItems: 'center', justifyContent: 'center', boxShadow: '0 4px 14px rgba(59, 130, 246, 0.35)', flexShrink: 0 }}>
            <AudioOutlined style={{ fontSize: 22, color: '#fff' }} />
          </div>
          <div>
            <h1 style={{ margin: 0, fontSize: 22, fontWeight: 700, color: THEME.textMain, lineHeight: 1.3 }}>ASR 语音识别配置</h1>
            <p style={{ margin: '4px 0 0', fontSize: 13, color: THEME.textSecondary }}>维护 ASR 供应商配置，支持火山引擎、OpenAI Whisper、Azure Speech</p>
          </div>
        </div>
        <div style={{ display: 'flex', gap: 12, flexWrap: 'wrap' }}>
          <div style={{ ...solidCard, padding: '12px 20px', display: 'flex', alignItems: 'center', gap: 8, minWidth: 120 }}>
            <span style={{ fontSize: 24, fontWeight: 700, color: THEME.primary }}>{configs.length}</span>
            <span style={{ fontSize: 12, color: THEME.textSecondary }}>条配置</span>
          </div>
          <Button icon={<ReloadOutlined />} onClick={() => configsQuery.refetch()} style={{ borderRadius: 10 }}>刷新</Button>
          <Button type="primary" icon={<PlusOutlined />} onClick={startCreating} style={{ borderRadius: 10, background: 'linear-gradient(135deg, #3b82f6, #2563eb)', border: 'none', boxShadow: '0 4px 14px rgba(37, 99, 235, 0.35)' }}>新建 ASR 配置</Button>
        </div>
      </div>

      {/* Default Binding Toolbar */}
      <div style={{ ...solidCard, padding: '16px 20px', marginBottom: 20 }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 12, flexWrap: 'wrap' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 8, flex: '1 1 280px' }}>
            <span style={{ fontSize: 13, fontWeight: 500, color: THEME.textSecondary, whiteSpace: 'nowrap' }}>
              全局默认 ASR
            </span>
            <Select
              value={defaultConfigId}
              onChange={(v) => setDefaultConfigId(v)}
              style={{ flex: 1 }}
              options={[
                { value: '0', label: '未设置，使用数据库第一条启用配置' },
                ...configs.filter((c) => c.is_active).map((config) => ({
                  value: String(config.id),
                  label: `${config.name} (${PROVIDER_MAP.get(config.engine)?.label || config.engine})`,
                })),
              ]}
            />
          </div>
          <Button
            type="primary"
            icon={<SaveOutlined />}
            onClick={() => saveDefaultMutation.mutate()}
            loading={saveDefaultMutation.isPending}
            style={{ borderRadius: 10, background: 'linear-gradient(135deg, #3b82f6, #2563eb)', border: 'none', boxShadow: '0 4px 14px rgba(37, 99, 235, 0.35)' }}
          >
            保存默认配置
          </Button>
        </div>
      </div>

      {/* Main Content */}
      <div style={{ display: 'flex', gap: 20, flexWrap: 'wrap' }}>
        {/* Left: Config List */}
        <div style={{ flex: '1 1 340px', maxWidth: 420, minWidth: 300 }}>
          <div style={{ ...solidCard, overflow: 'hidden', display: 'flex', flexDirection: 'column', height: 'calc(100vh - 300px)', minHeight: 500 }}>
            <div style={{ padding: '16px 20px', borderBottom: '1px solid ' + THEME.border, display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
              <span style={{ fontSize: 15, fontWeight: 600, color: THEME.textMain }}>配置列表</span>
              <span style={{ fontSize: 12, color: THEME.textMuted }}>共 {configs.length} 条</span>
            </div>
            <div style={{ flex: 1, overflowY: 'auto', padding: '8px' }}>
              {configs.length === 0 ? (
                <div style={{ height: '100%', display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', color: THEME.textMuted, gap: 12 }}>
                  <InboxOutlined style={{ fontSize: 40 }} />
                  <span style={{ fontSize: 14 }}>当前还没有 ASR 配置记录</span>
                  <span style={{ fontSize: 12 }}>可以先新建一条供应商配置</span>
                </div>
              ) : (
                <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
                  {configs.map((config) => {
                    const isActive = selectedConfigId === config.id
                    return (
                      <div
                        key={config.id}
                        role="button"
                        tabIndex={0}
                        onClick={() => startEditing(config)}
                        onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ' ') startEditing(config) }}
                        style={{
                          padding: '14px 16px', borderRadius: 12, cursor: 'pointer',
                          border: isActive ? '1.5px solid ' + THEME.primary : '1.5px solid transparent',
                          background: isActive ? '#f5f3ff' : '#fafafa', transition: 'all 0.2s ease',
                        }}
                      >
                        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 6 }}>
                          <span style={{ fontSize: 14, fontWeight: 600, color: THEME.textMain }}>{config.name}</span>
                          <div style={{ display: 'flex', gap: 4 }}>
                            {config.is_active ? <Tag color="green">启用</Tag> : <Tag>禁用</Tag>}
                          </div>
                        </div>
                        <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                          <Tag color="blue">{PROVIDER_MAP.get(config.engine)?.label || config.engine}</Tag>
                          <span style={{ fontSize: 12, color: THEME.textMuted }}>ID: {config.id}</span>
                        </div>
                      </div>
                    )
                  })}
                </div>
              )}
            </div>
          </div>
        </div>

        {/* Right: Edit Form */}
        <div style={{ flex: '2 1 480px', minWidth: 380 }}>
          <form onSubmit={handleSubmit} style={{ ...solidCard, padding: '24px 28px', display: 'flex', flexDirection: 'column', gap: 20 }}>
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
              <span style={{ fontSize: 16, fontWeight: 600, color: THEME.textMain }}>
                {selectedConfigId ? `编辑配置 #${selectedConfigId}` : '新建 ASR 配置'}
              </span>
              {selectedConfigId && (
                <Tooltip title="删除此配置">
                  <Button danger icon={<DeleteOutlined />} size="small" onClick={handleDelete} loading={deleteMutation.isPending}>删除</Button>
                </Tooltip>
              )}
            </div>

            {/* 基本信息 */}
            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 16 }}>
              <div>
                <label style={{ fontSize: 13, fontWeight: 500, color: THEME.textSecondary, marginBottom: 6, display: 'block' }}>配置名称 *</label>
                <Input value={form.name} onChange={(e) => updateField('name', e.target.value)} placeholder="例如：生产环境火山引擎" />
              </div>
              <div>
                <label style={{ fontSize: 13, fontWeight: 500, color: THEME.textSecondary, marginBottom: 6, display: 'block' }}>ASR 引擎 *</label>
                <Select value={form.engine} onChange={handleEngineChange} style={{ width: '100%' }} options={ASR_PROVIDERS.map((p) => ({ value: p.key, label: p.label }))} />
              </div>
            </div>

            {currentProvider && (
              <div style={{ padding: '12px 16px', background: '#f0f9ff', borderRadius: 10, border: '1px solid #bae6fd' }}>
                <p style={{ margin: 0, fontSize: 13, color: THEME.textSecondary }}>{currentProvider.description}</p>
              </div>
            )}

            {/* 鉴权配置 */}
            <div>
              <label style={{ fontSize: 13, fontWeight: 500, color: THEME.textSecondary, marginBottom: 6, display: 'block' }}>鉴权配置 (JSON) *</label>
              {currentProvider?.auth_fields.map((field) => (
                <div key={field.key} style={{ marginBottom: 8 }}>
                  <span style={{ fontSize: 12, color: THEME.textMuted }}>{field.label}{field.required ? ' *' : ''}: <code>{field.key}</code></span>
                </div>
              ))}
              <Input.TextArea value={form.authConfigJson} onChange={(e) => updateField('authConfigJson', e.target.value)} rows={4} style={{ fontFamily: 'monospace', fontSize: 13 }} />
              {!authPreview.valid && <p style={{ margin: '4px 0 0', fontSize: 12, color: THEME.danger }}>{authPreview.error}</p>}
            </div>

            {/* 参数配置 */}
            <div>
              <label style={{ fontSize: 13, fontWeight: 500, color: THEME.textSecondary, marginBottom: 6, display: 'block' }}>参数配置 (JSON)</label>
              {currentProvider?.param_fields.map((field) => (
                <div key={field.key} style={{ marginBottom: 4 }}>
                  <span style={{ fontSize: 12, color: THEME.textMuted }}>{field.label}: <code>{field.key}</code> — {field.description}</span>
                </div>
              ))}
              <Input.TextArea value={form.paramsJson} onChange={(e) => updateField('paramsJson', e.target.value)} rows={5} style={{ fontFamily: 'monospace', fontSize: 13 }} />
              {!paramsPreview.valid && <p style={{ margin: '4px 0 0', fontSize: 12, color: THEME.danger }}>{paramsPreview.error}</p>}
            </div>

            {/* 状态 */}
            <div style={{ display: 'flex', gap: 16, alignItems: 'center' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                <span style={{ fontSize: 13, color: THEME.textSecondary }}>启用</span>
                <Switch checked={form.isActive} onChange={(v) => updateField('isActive', v)} />
              </div>
              <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                <span style={{ fontSize: 13, color: THEME.textSecondary }}>排序</span>
                <InputNumber value={Number(form.sortOrder)} onChange={(v) => updateField('sortOrder', String(v || 0))} min={0} style={{ width: 80 }} />
              </div>
            </div>

            {/* 操作栏 */}
            <div style={{ display: 'flex', alignItems: 'center', gap: 12, paddingTop: 8, borderTop: '1px solid ' + THEME.border }}>
              <p style={{ flex: 1, margin: 0, fontSize: 13, color: messageText.includes('失败') || messageText.includes('错误') ? THEME.danger : THEME.textSecondary }}>{messageText}</p>
              <Button type="primary" htmlType="submit" icon={<SaveOutlined />} loading={saveMutation.isPending} disabled={!!formError} style={{ borderRadius: 10, background: 'linear-gradient(135deg, #3b82f6, #2563eb)', border: 'none', boxShadow: '0 4px 14px rgba(37, 99, 235, 0.35)' }}>
                {selectedConfigId ? '保存修改' : '创建配置'}
              </Button>
            </div>
          </form>
        </div>
      </div>
    </div>
  )
}
