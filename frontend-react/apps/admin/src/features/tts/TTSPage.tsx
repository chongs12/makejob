import type { FormEvent } from 'react'
import { useEffect, useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { extractErrorMessage, requestJson } from '@makejob/api-client'
import { isSuccessCode, type ApiEnvelope } from '@makejob/shared-types'
import { useAdminAuthStore } from '../../state/auth'

type TTSEngine = 'elevenlabs' | 'minimax' | 'aliyun' | 'xunfei'
type TTSScene = 'interview' | 'companion'

interface TTSConfig {
  id: number
  name: string
  engine: TTSEngine
  voice_id: string
  scene: TTSScene
  params_json: string
  is_active: boolean
  sort_order: number
  created_at?: string
  updated_at?: string
}

interface TTSFormState {
  name: string
  engine: TTSEngine
  voiceId: string
  scene: TTSScene
  paramsJson: string
  isActive: boolean
  sortOrder: string
}

interface TTSParamsPreview {
  valid: boolean
  error: string
  formattedJson: string
}

const TTS_ENGINE_OPTIONS: Array<{ value: TTSEngine; label: string }> = [
  { value: 'elevenlabs', label: 'ElevenLabs' },
  { value: 'minimax', label: 'MiniMax' },
  { value: 'aliyun', label: '阿里云' },
  { value: 'xunfei', label: '讯飞' },
]

const TTS_SCENE_OPTIONS: Array<{ value: TTSScene; label: string }> = [
  { value: 'companion', label: '学习陪伴' },
  { value: 'interview', label: '面试' },
]

/**
 * 获取后台当前维护的 TTS 配置列表。
 */
async function fetchTTSConfigs(token: string | null): Promise<TTSConfig[]> {
  const response = await requestJson<ApiEnvelope<TTSConfig[]>>('/admin/tts-configs', {
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
 * 构造 TTS 表单初始值，便于新建态直接使用。
 */
function buildInitialTTSForm(): TTSFormState {
  return {
    name: '',
    engine: 'minimax',
    voiceId: '',
    scene: 'companion',
    paramsJson: '{\n  "speed": 1,\n  "volume": 1\n}',
    isActive: true,
    sortOrder: '0',
  }
}

/**
 * 将数据库中的 TTS 记录转换成前端可编辑表单。
 */
function buildTTSForm(config?: TTSConfig | null): TTSFormState {
  if (!config) {
    return buildInitialTTSForm()
  }

  return {
    name: config.name,
    engine: config.engine,
    voiceId: config.voice_id,
    scene: config.scene,
    paramsJson: config.params_json || '',
    isActive: config.is_active,
    sortOrder: String(config.sort_order),
  }
}

/**
 * 将表单状态转换为后端 TTS 请求体。
 */
function buildTTSPayload(form: TTSFormState): Record<string, unknown> {
  return {
    name: form.name.trim(),
    engine: form.engine,
    voice_id: form.voiceId.trim(),
    scene: form.scene,
    params_json: form.paramsJson.trim(),
    is_active: form.isActive,
    sort_order: Number(form.sortOrder) || 0,
  }
}

/**
 * 将 TTS 引擎值转换为更可读的中文标签。
 */
function ttsEngineLabel(engine: string): string {
  return TTS_ENGINE_OPTIONS.find((option) => option.value === engine)?.label || engine
}

/**
 * 将 TTS 场景值转换为后台页可读标签。
 */
function ttsSceneLabel(scene: string): string {
  return TTS_SCENE_OPTIONS.find((option) => option.value === scene)?.label || scene
}

/**
 * 解析 TTS 额外参数 JSON，并给出格式化预览或错误提示。
 */
function buildTTSParamsPreview(rawJson: string): TTSParamsPreview {
  const trimmed = rawJson.trim()
  if (!trimmed) {
    return {
      valid: true,
      error: '',
      formattedJson: '{}',
    }
  }

  try {
    const parsed = JSON.parse(trimmed) as unknown
    if (!parsed || Array.isArray(parsed) || typeof parsed !== 'object') {
      return {
        valid: false,
        error: '额外参数必须是 JSON 对象，例如 {"speed":1}',
        formattedJson: '{}',
      }
    }

    return {
      valid: true,
      error: '',
      formattedJson: JSON.stringify(parsed, null, 2),
    }
  } catch (error) {
    return {
      valid: false,
      error: extractErrorMessage(error, '额外参数 JSON 解析失败'),
      formattedJson: '{}',
    }
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
 * 校验当前 TTS 表单，提前发现必填项和 JSON 格式问题。
 */
function validateTTSForm(form: TTSFormState, paramsPreview: TTSParamsPreview): string {
  if (!form.name.trim()) {
    return '音色名称不能为空'
  }
  if (!form.voiceId.trim()) {
    return 'Voice ID 不能为空'
  }
  if (!paramsPreview.valid) {
    return paramsPreview.error
  }

  return ''
}

/**
 * 提供后台 TTS 配置管理页，支持创建、编辑、删除和 JSON 预校验。
 */
export function TTSPage() {
  const accessToken = useAdminAuthStore((state) => state.accessToken)
  const queryClient = useQueryClient()
  const [selectedConfigId, setSelectedConfigId] = useState<number | null>(null)
  const [form, setForm] = useState<TTSFormState>(buildInitialTTSForm())
  const [message, setMessage] = useState('读取 TTS 配置中')

  const configsQuery = useQuery({
    queryKey: ['admin', 'tts-configs', accessToken],
    queryFn: () => fetchTTSConfigs(accessToken),
    enabled: Boolean(accessToken),
  })

  const paramsPreview = useMemo(() => buildTTSParamsPreview(form.paramsJson), [form.paramsJson])
  const formError = useMemo(() => validateTTSForm(form, paramsPreview), [form, paramsPreview])

  useEffect(() => {
    if (!configsQuery.data) {
      return
    }

    if (selectedConfigId === null) {
      setMessage((current) => (current === '读取 TTS 配置中' ? '已同步 TTS 配置列表。' : current))
      return
    }

    const nextConfig = configsQuery.data.find((item) => item.id === selectedConfigId)
    if (nextConfig) {
      setForm(buildTTSForm(nextConfig))
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

  const deleteMutation = useMutation({
    mutationFn: async (id: number) => {
      await deleteTTSConfig(accessToken, id)
    },
    onSuccess: async () => {
      setSelectedConfigId(null)
      setForm(buildInitialTTSForm())
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
    setForm(buildInitialTTSForm())
    setMessage('已切换到新建 TTS 配置模式。')
  }

  /**
   * 将指定的 TTS 配置装载到右侧编辑区。
   */
  function startEditingTTSConfig(config: TTSConfig): void {
    setSelectedConfigId(config.id)
    setForm(buildTTSForm(config))
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

  if (configsQuery.isLoading) {
    return (
      <section className="admin-panel">
        <span className="admin-tag">语音中心</span>
        <h2>TTS 配置</h2>
        <p className="admin-copy">正在加载后台 TTS 配置列表。</p>
      </section>
    )
  }

  if (configsQuery.isError) {
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
            当前页用于维护面试和陪伴场景使用的音色配置。支持不同引擎、音色 ID、排序优先级和额外参数 JSON 的编辑。
          </p>
        </div>
        <div className="admin-tts-page__summary">
          <strong>{configsQuery.data?.length || 0}</strong>
          <span>条配置</span>
        </div>
      </div>

      <div className="admin-tts-page__toolbar">
        <button className="admin-link" type="button" onClick={startCreatingTTSConfig}>
          新建 TTS 配置
        </button>
      </div>

      <div className="admin-tts-page__layout">
        <div className="admin-tts-list">
          {(configsQuery.data || []).length === 0 ? (
            <div className="admin-tts-card admin-tts-card--empty">
              <strong>当前还没有 TTS 配置记录</strong>
              <p>可以先创建一条陪伴或面试场景的音色配置。</p>
            </div>
          ) : (
            (configsQuery.data || []).map((config) => (
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
                  <span>{ttsEngineLabel(config.engine)}</span>
                  <span>{ttsSceneLabel(config.scene)}</span>
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
            <span>音色名称</span>
            <input
              value={form.name}
              onChange={(event) => updateTTSField('name', event.target.value)}
              placeholder="例如 Ariu 陪伴女声"
            />
          </label>

          <div className="admin-tts-editor__grid">
            <label className="admin-field">
              <span>引擎</span>
              <select
                value={form.engine}
                onChange={(event) => updateTTSField('engine', event.target.value as TTSEngine)}
              >
                {TTS_ENGINE_OPTIONS.map((option) => (
                  <option key={option.value} value={option.value}>
                    {option.label}
                  </option>
                ))}
              </select>
            </label>

            <label className="admin-field">
              <span>使用场景</span>
              <select value={form.scene} onChange={(event) => updateTTSField('scene', event.target.value as TTSScene)}>
                {TTS_SCENE_OPTIONS.map((option) => (
                  <option key={option.value} value={option.value}>
                    {option.label}
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
            <span>Voice ID</span>
            <input
              value={form.voiceId}
              onChange={(event) => updateTTSField('voiceId', event.target.value)}
              placeholder="请输入引擎中的音色 ID"
            />
          </label>

          <label className="admin-field">
            <span>额外参数 JSON</span>
            <textarea
              className="admin-tts-editor__params"
              value={form.paramsJson}
              onChange={(event) => updateTTSField('paramsJson', event.target.value)}
              placeholder='例如 {"speed":1,"volume":1}'
            />
          </label>

          <div className={`admin-tts-editor__status ${formError ? 'is-error' : 'is-valid'}`}>
            <strong>表单检查</strong>
            <span>{formError || '当前 TTS 表单已通过基础校验，可以提交保存。'}</span>
          </div>

          <div className="admin-tts-editor__effective-json">
            <div className="admin-tts-editor__effective-head">
              <strong>参数预览</strong>
              <span>{paramsPreview.valid ? '已通过 JSON 校验' : '当前展示回退空对象'}</span>
            </div>
            <pre>{paramsPreview.formattedJson}</pre>
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
              disabled={saveMutation.isPending || deleteMutation.isPending}
            >
              重置为新建
            </button>
            {selectedConfigId ? (
              <button
                className="admin-link"
                type="button"
                onClick={handleDelete}
                disabled={saveMutation.isPending || deleteMutation.isPending}
              >
                {deleteMutation.isPending ? '删除中...' : '删除配置'}
              </button>
            ) : null}
            <button
              className="admin-link"
              type="submit"
              disabled={Boolean(formError) || saveMutation.isPending || deleteMutation.isPending}
            >
              {saveMutation.isPending ? '保存中...' : selectedConfigId ? '保存修改' : '创建配置'}
            </button>
          </div>
        </form>
      </div>
    </section>
  )
}
