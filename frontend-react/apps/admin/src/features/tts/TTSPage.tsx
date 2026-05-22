import type { FormEvent } from 'react'
import { useEffect, useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
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
  return providers.find((provider) => provider.support_status === 'ready') || providers[0]
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
  const [message, setMessage] = useState('读取 TTS 配置中')

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
    return (configsQuery.data?.configs || []).filter((config) => config.support_status === 'ready')
  }, [configsQuery.data?.configs])

  useEffect(() => {
    if (!configsQuery.data) {
      return
    }

    setDefaultBindings(buildDefaultBindingsForm(configsQuery.data.default_bindings))

    if (selectedConfigId === null) {
      setForm((current) => {
        if (current.engine || configsQuery.data.providers.length === 0) {
          return current
        }
        return buildInitialTTSForm(configsQuery.data.providers)
      })
      setMessage((current) => (current === '读取 TTS 配置中' ? '已同步 TTS 配置列表。' : current))
      return
    }

    const nextConfig = configsQuery.data.configs.find((item) => item.id === selectedConfigId)
    if (nextConfig) {
      setForm(buildTTSForm(nextConfig, configsQuery.data.providers))
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
      return created.id
    },
    onSuccess: async (configId) => {
      setSelectedConfigId(configId)
      setMessage(selectedConfigId ? 'TTS 配置已更新。' : 'TTS 配置已创建。')
      await queryClient.invalidateQueries({
        queryKey: ['admin', 'tts-configs'],
      })
    },
    onError: (error) => {
      setMessage(extractErrorMessage(error, '保存 TTS 配置失败，请稍后重试'))
    },
  })

  const saveDefaultsMutation = useMutation({
    mutationFn: async () => {
      await updateTTSSceneDefaults(accessToken, buildDefaultBindingsPayload(defaultBindings))
    },
    onSuccess: async () => {
      setMessage('场景默认 TTS 绑定已更新。')
      await queryClient.invalidateQueries({
        queryKey: ['admin', 'tts-configs'],
      })
    },
    onError: (error) => {
      setMessage(extractErrorMessage(error, '更新场景默认 TTS 绑定失败，请稍后重试'))
    },
  })

  const deleteMutation = useMutation({
    mutationFn: async (id: number) => {
      await deleteTTSConfig(accessToken, id)
    },
    onSuccess: async () => {
      setSelectedConfigId(null)
      setForm(buildInitialTTSForm(configsQuery.data?.providers || []))
      setMessage('TTS 配置已删除。')
      await queryClient.invalidateQueries({
        queryKey: ['admin', 'tts-configs'],
      })
    },
    onError: (error) => {
      setMessage(extractErrorMessage(error, '删除 TTS 配置失败，请稍后重试'))
    },
  })

  /**
   * 切换到新建模式，并清空当前 TTS 表单。
   */
  function startCreatingTTSConfig(): void {
    setSelectedConfigId(null)
    setForm(buildInitialTTSForm(configsQuery.data?.providers || []))
    setMessage('已切换到新建 TTS 配置模式。')
  }

  /**
   * 将指定的 TTS 配置装载到右侧编辑区。
   */
  function startEditingTTSConfig(config: TTSConfig): void {
    setSelectedConfigId(config.id)
    setForm(buildTTSForm(config, configsQuery.data?.providers || []))
    setMessage(`正在编辑音色：${config.name}`)
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
      setMessage(formError)
      return
    }

    setMessage(selectedConfigId ? '正在更新 TTS 配置。' : '正在创建 TTS 配置。')
    saveMutation.mutate()
  }

  /**
   * 删除当前选中的 TTS 配置。
   */
  function handleDelete(): void {
    if (!selectedConfigId) {
      return
    }

    if (!window.confirm('确认删除当前 TTS 配置吗？删除后不可恢复。')) {
      return
    }

    setMessage('正在删除 TTS 配置。')
    deleteMutation.mutate(selectedConfigId)
  }

  /**
   * 保存场景默认 TTS 绑定。
   */
  function handleSaveDefaults(): void {
    setMessage('正在更新场景默认 TTS 绑定。')
    saveDefaultsMutation.mutate()
  }

  if (configsQuery.isLoading) {
    return (
      <section className="admin-panel">
        <span className="admin-tag">语音中心</span>
        <h2>TTS 配置</h2>
        <p className="admin-copy">正在加载后台 TTS 配置列表。</p>
      </section>
    )
  }

  if (configsQuery.isError || !configsQuery.data) {
    return (
      <section className="admin-panel">
        <span className="admin-tag">语音中心</span>
        <h2>TTS 配置</h2>
        <p className="admin-copy">{extractErrorMessage(configsQuery.error, '读取 TTS 配置失败')}</p>
      </section>
    )
  }

  return (
    <section className="admin-panel admin-tts-page">
      <div className="admin-tts-page__hero">
        <div>
          <span className="admin-tag">语音中心</span>
          <h2>TTS 配置</h2>
          <p className="admin-copy">
            这里维护真实可运行的 TTS 供应商配置。TTS 记录本身只负责保存供应商、鉴权和官方参数，实际使用场景改由
            Live2D 模型绑定与场景默认策略决定。
          </p>
        </div>
        <div className="admin-tts-page__summary">
          <strong>{configsQuery.data.configs.length}</strong>
          <span>条配置</span>
        </div>
      </div>

      <div className="admin-tts-page__toolbar">
        <div className="admin-tts-editor__grid">
          <label className="admin-field">
            <span>{ttsSceneLabel('interview')}默认 TTS</span>
            <select
              value={defaultBindings.interview}
              onChange={(event) => setDefaultBindings((current) => ({ ...current, interview: event.target.value }))}
            >
              <option value="0">未设置，回退到 config.yaml</option>
              {readyConfigOptions.map((config) => (
                <option key={config.id} value={config.id}>
                  {config.name}
                </option>
              ))}
            </select>
          </label>

          <label className="admin-field">
            <span>{ttsSceneLabel('companion')}默认 TTS</span>
            <select
              value={defaultBindings.companion}
              onChange={(event) => setDefaultBindings((current) => ({ ...current, companion: event.target.value }))}
            >
              <option value="0">未设置，回退到 config.yaml</option>
              {readyConfigOptions.map((config) => (
                <option key={config.id} value={config.id}>
                  {config.name}
                </option>
              ))}
            </select>
          </label>
        </div>

        <button className="admin-link" type="button" onClick={handleSaveDefaults} disabled={saveDefaultsMutation.isPending}>
          {saveDefaultsMutation.isPending ? '保存默认绑定中...' : '保存场景默认绑定'}
        </button>

        <button className="admin-link" type="button" onClick={startCreatingTTSConfig}>
          新建 TTS 配置
        </button>
      </div>

      <div className="admin-tts-page__layout">
        <div className="admin-tts-list">
          {configsQuery.data.configs.length === 0 ? (
            <div className="admin-tts-card admin-tts-card--empty">
              <strong>当前还没有 TTS 配置记录</strong>
              <p>可以先新建一条可运行的供应商配置，再到 Live2D 页面绑定使用。</p>
            </div>
          ) : (
            configsQuery.data.configs.map((config) => (
              <button
                key={config.id}
                type="button"
                className={`admin-tts-card ${selectedConfigId === config.id ? 'admin-tts-card--active' : ''}`}
                onClick={() => startEditingTTSConfig(config)}
              >
                <div className="admin-tts-card__head">
                  <strong>{config.name}</strong>
                  <span>{config.is_active ? '启用中' : '已停用'}</span>
                </div>
                <div className="admin-tts-card__meta">
                  <span>{ttsEngineLabel(config.engine, providerMap)}</span>
                  <span>{supportStatusLabel(config.support_status)}</span>
                  <span>排序 {config.sort_order}</span>
                </div>
                <p>{shortenVoiceId(config.voice_id)}</p>
              </button>
            ))
          )}
        </div>

        <form className="admin-tts-editor" onSubmit={handleSubmit}>
          <div className="admin-tts-editor__head">
            <div>
              <h3>{selectedConfigId ? '编辑 TTS 配置' : '新建 TTS 配置'}</h3>
              <p>{message}</p>
            </div>
            <span className="admin-tag">{selectedConfigId ? `ID #${selectedConfigId}` : '新配置'}</span>
          </div>

          <label className="admin-field">
            <span>配置名称</span>
            <input
              value={form.name}
              onChange={(event) => updateTTSField('name', event.target.value)}
              placeholder="例如 豆包陪伴女声"
            />
          </label>

          <div className="admin-tts-editor__grid">
            <label className="admin-field">
              <span>供应商</span>
              <select value={form.engine} onChange={(event) => handleEngineChange(event.target.value)}>
                {configsQuery.data.providers.map((provider) => (
                  <option key={provider.key} value={provider.key}>
                    {provider.label}
                  </option>
                ))}
              </select>
            </label>

            <label className="admin-field">
              <span>排序权重</span>
              <input
                type="number"
                value={form.sortOrder}
                onChange={(event) => updateTTSField('sortOrder', event.target.value)}
                placeholder="0"
              />
            </label>
          </div>

          <label className="admin-field">
            <span>Voice ID / Speaker</span>
            <input
              value={form.voiceId}
              onChange={(event) => updateTTSField('voiceId', event.target.value)}
              placeholder="请输入当前供应商的音色或说话人 ID"
            />
          </label>

          <div className={`admin-tts-editor__status ${currentProvider?.support_status === 'ready' ? 'is-valid' : 'is-error'}`}>
            <strong>供应商状态</strong>
            <span>{currentProvider?.support_message || '当前供应商元数据缺失。'}</span>
          </div>

          <label className="admin-field">
            <span>鉴权配置 JSON</span>
            <textarea
              className="admin-tts-editor__params"
              value={form.authConfigJson}
              onChange={(event) => updateTTSField('authConfigJson', event.target.value)}
              placeholder='例如 {"api_key":"xxx"}'
            />
          </label>

          <label className="admin-field">
            <span>供应商参数 JSON</span>
            <textarea
              className="admin-tts-editor__params"
              value={form.paramsJson}
              onChange={(event) => updateTTSField('paramsJson', event.target.value)}
              placeholder='例如 {"resource_id":"seed-tts-2.0"}'
            />
          </label>

          <div className={`admin-tts-editor__status ${formError ? 'is-error' : 'is-valid'}`}>
            <strong>表单检查</strong>
            <span>{formError || '当前 TTS 表单已通过基础校验，可以提交保存。'}</span>
          </div>

          <div className="admin-tts-editor__effective-json">
            <div className="admin-tts-editor__effective-head">
              <strong>鉴权配置预览</strong>
              <span>{authPreview.valid ? '已通过 JSON 校验' : '当前展示回退空对象'}</span>
            </div>
            <pre>{authPreview.formattedJson}</pre>
          </div>

          <div className="admin-tts-editor__effective-json">
            <div className="admin-tts-editor__effective-head">
              <strong>参数配置预览</strong>
              <span>{paramsPreview.valid ? '已合并上方 Voice ID 后的最终预览' : '当前展示回退空对象'}</span>
            </div>
            <pre>{effectiveParamsPreview.formattedJson}</pre>
          </div>

          <div className="admin-tts-editor__effective-json">
            <div className="admin-tts-editor__effective-head">
              <strong>官方字段提示</strong>
              <span>{currentProvider?.label || '未知供应商'}</span>
            </div>
            <pre>{`鉴权字段：\n${formatFieldDefinitions(currentProvider?.auth_fields || [])}\n\n参数字段：\n${formatFieldDefinitions(currentProvider?.param_fields || [])}`}</pre>
          </div>

          <label className="admin-tts-editor__switch">
            <input
              type="checkbox"
              checked={form.isActive}
              onChange={(event) => updateTTSField('isActive', event.target.checked)}
            />
            <span>{form.isActive ? '当前配置启用中' : '当前配置已停用'}</span>
          </label>

          <div className="admin-tts-editor__actions">
            <button
              className="admin-link"
              type="button"
              onClick={startCreatingTTSConfig}
              disabled={saveMutation.isPending || deleteMutation.isPending || saveDefaultsMutation.isPending}
            >
              重置为新建
            </button>
            {selectedConfigId ? (
              <button
                className="admin-link"
                type="button"
                onClick={handleDelete}
                disabled={saveMutation.isPending || deleteMutation.isPending || saveDefaultsMutation.isPending}
              >
                {deleteMutation.isPending ? '删除中...' : '删除配置'}
              </button>
            ) : null}
            <button
              className="admin-link"
              type="submit"
              disabled={Boolean(formError) || saveMutation.isPending || deleteMutation.isPending || saveDefaultsMutation.isPending}
            >
              {saveMutation.isPending ? '保存中...' : selectedConfigId ? '保存修改' : '创建配置'}
            </button>
          </div>
        </form>
      </div>
    </section>
  )
}
