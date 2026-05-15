import type { FormEvent } from 'react'
import { useEffect, useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { extractErrorMessage, requestJson } from '@makejob/api-client'
import { isSuccessCode, type ApiEnvelope } from '@makejob/shared-types'
import { useAdminAuthStore } from '../../state/auth'

type AdminConfigValueType = 'string' | 'number' | 'boolean'
type AIConfigGroup = 'provider' | 'runtime' | 'scene'

interface AdminConfigItem {
  id?: number
  config_key: string
  config_value: string
  config_type: string
  description?: string
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

/**
 * 读取当前 AI 配置列表，并对非成功响应做统一错误抛出。
 */
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

/**
 * 提交完整 AI 配置草稿，确保后端可一次性覆盖当前运行配置。
 */
async function updateAIConfigs(token: string | null, configs: Record<string, string>): Promise<void> {
  const response = await requestJson<ApiEnvelope<null>>('/admin/ai-configs', {
    method: 'PUT',
    token,
    body: {
      configs,
    },
  })

  if (!isSuccessCode(response.code)) {
    throw new Error(response.message || '保存 AI 配置失败')
  }
}

/**
 * 根据 provider 标识生成后台配置页可读标签，未知值保留原始内容便于排查历史脏数据。
 */
function getAIProviderLabel(value: string): string {
  return AI_PROVIDER_LABELS[value] || value
}

/**
 * 基于后端支持范围构造 Provider 下拉选项，并在存在历史无效值时保留一个告警选项。
 */
function buildProviderOptions(values: string[], currentValue: string, allowEmpty: boolean): AIConfigFieldOption[] {
  const options: AIConfigFieldOption[] = []
  if (allowEmpty) {
    options.push({
      value: '',
      label: '不启用',
    })
  }

  values.forEach((value) => {
    options.push({
      value,
      label: getAIProviderLabel(value),
    })
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

/**
 * 将后端返回的配置项元信息、运行时支持范围与当前配置合并，保证默认项也能渲染。
 */
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

/**
 * 计算当前页头部要展示的运行时摘要，便于管理员快速确认主 Provider、模型和兜底值。
 */
function buildRuntimeSummary(configs: Record<string, string>): Array<{ label: string; value: string }> {
  const interviewModel = configs.ai_scene_interview_model?.trim() || configs.ai_model || '未配置'

  return [
    {
      label: '当前主 Provider',
      value: getAIProviderLabel(configs.ai_provider || '未配置'),
    },
    {
      label: '当前默认模型',
      value: configs.ai_model || '未配置',
    },
    {
      label: '题卡流水线模型',
      value: interviewModel,
    },
    {
      label: '当前兜底 Provider',
      value: configs.ai_fallback_provider ? getAIProviderLabel(configs.ai_fallback_provider) : '不启用',
    },
  ]
}

/**
 * 生成配置页顶部最需要管理员关注的运行时提示。
 */
function buildRuntimeAttentionNotes(response: AIConfigResponse | undefined): string[] {
  if (!response) {
    return []
  }

  return [...response.support.notes, ...response.warnings]
}

/**
 * 基于接口返回的有效配置生成可编辑草稿，避免表单首次渲染为空。
 */
function buildAIConfigDraft(configs: Record<string, string>, metas: AIConfigFieldMeta[]): Record<string, string> {
  return metas.reduce<Record<string, string>>((result, field) => {
    result[field.key] = configs[field.key] ?? ''
    return result
  }, {})
}

/**
 * 将字符串布尔值转换为复选框可直接消费的布尔结果。
 */
function parseBooleanConfigValue(value?: string): boolean {
  return String(value).trim().toLowerCase() === 'true'
}

/**
 * 根据字段类型将用户输入收敛为最终提交值，避免布尔值和空白值格式混乱。
 */
function normalizeAIConfigValue(meta: AIConfigFieldMeta, value: string): string {
  if (meta.type === 'boolean') {
    return parseBooleanConfigValue(value) ? 'true' : 'false'
  }

  return value.trim()
}

/**
 * 计算当前草稿与服务端生效配置之间的变更键列表，用于页面提示保存范围。
 */
function collectChangedConfigKeys(
  draft: Record<string, string>,
  configs: Record<string, string>,
  metas: AIConfigFieldMeta[],
): string[] {
  return metas
    .filter((meta) => normalizeAIConfigValue(meta, draft[meta.key] ?? '') !== (configs[meta.key] ?? ''))
    .map((meta) => meta.key)
}

/**
 * 根据配置键返回数字输入框步进值，兼顾采样参数与整数参数。
 */
function resolveNumberStep(key: string): string {
  if (key === 'ai_temperature' || key === 'ai_top_p') {
    return '0.1'
  }

  return '1'
}

/**
 * 将字段按业务区块分组，便于管理页按“提供方/运行时/场景”组织表单。
 */
function groupAIConfigFields(metas: AIConfigFieldMeta[]): Record<AIConfigGroup, AIConfigFieldMeta[]> {
  return metas.reduce<Record<AIConfigGroup, AIConfigFieldMeta[]>>(
    (result, field) => {
      result[field.group].push(field)
      return result
    },
    {
      provider: [],
      runtime: [],
      scene: [],
    },
  )
}

/**
 * 提供 AI 运行配置管理页，支持读取、编辑和保存生效配置。
 */
export function AIConfigPage() {
  const accessToken = useAdminAuthStore((state) => state.accessToken)
  const queryClient = useQueryClient()
  const [draft, setDraft] = useState<Record<string, string>>({})
  const [message, setMessage] = useState('读取当前 AI 运行配置中')

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
  const runtimeAttentionNotes = useMemo(() => buildRuntimeAttentionNotes(configQuery.data), [configQuery.data])

  useEffect(() => {
    if (!configQuery.data) {
      return
    }

    setDraft(buildAIConfigDraft(configQuery.data.configs, fieldMetas))
    setMessage((current) =>
      current === '读取当前 AI 运行配置中' ? '已同步当前运行配置，可以直接编辑后保存。' : current,
    )
  }, [configQuery.data, fieldMetas])

  const changedKeys = useMemo(() => {
    if (!configQuery.data) {
      return []
    }

    return collectChangedConfigKeys(draft, configQuery.data.configs, fieldMetas)
  }, [configQuery.data, draft, fieldMetas])

  const saveMutation = useMutation({
    mutationFn: async () => {
      const nextConfigs = fieldMetas.reduce<Record<string, string>>((result, meta) => {
        result[meta.key] = normalizeAIConfigValue(meta, draft[meta.key] ?? '')
        return result
      }, {})

      await updateAIConfigs(accessToken, nextConfigs)
    },
    onSuccess: async () => {
      setMessage('AI 配置已保存，当前页面数据已刷新为最新生效值。')
      await queryClient.invalidateQueries({
        queryKey: ['admin', 'ai-configs'],
      })
    },
    onError: (error) => {
      setMessage(extractErrorMessage(error, '保存 AI 配置失败，请稍后重试'))
    },
  })

  /**
   * 更新指定配置项的草稿值，避免每个字段重复拼装 setState 逻辑。
   */
  function updateDraftValue(key: string, value: string): void {
    setDraft((current) => ({
      ...current,
      [key]: value,
    }))
  }

  /**
   * 将当前草稿整体恢复为服务端最新生效值，便于放弃未保存改动。
   */
  function resetDraft(): void {
    if (!configQuery.data) {
      return
    }

    setDraft(buildAIConfigDraft(configQuery.data.configs, fieldMetas))
    setMessage('已恢复为当前服务端生效配置。')
  }

  /**
   * 提交页面表单并触发 AI 配置保存。
   */
  function handleSubmit(event: FormEvent<HTMLFormElement>): void {
    event.preventDefault()
    setMessage('正在保存 AI 配置，请稍候。')
    saveMutation.mutate()
  }

  if (configQuery.isLoading) {
    return (
      <section className="admin-panel">
        <span className="admin-tag">AI 运行时</span>
        <h2>AI 配置</h2>
        <p className="admin-copy">正在加载后台 AI 运行配置。</p>
      </section>
    )
  }

  if (configQuery.isError || !configQuery.data) {
    return (
      <section className="admin-panel">
        <span className="admin-tag">AI 运行时</span>
        <h2>AI 配置</h2>
        <p className="admin-copy">{extractErrorMessage(configQuery.error, '读取 AI 配置失败')}</p>
      </section>
    )
  }

  return (
    <section className="admin-panel admin-ai-config">
      <div className="admin-ai-config__hero">
        <div>
          <span className="admin-tag">AI 运行时</span>
          <h2>AI 配置</h2>
          <p className="admin-copy">
            当前页直接管理运行时 Provider、默认模型和场景模型覆盖。保存后会整体回写后台配置，陪伴、计划、面试等能力会按新配置生效；题卡流水线使用面试场景模型。
          </p>
          {runtimeAttentionNotes.length ? (
            <ul className="admin-ai-config__notes">
              {runtimeAttentionNotes.map((note) => (
                <li key={note}>{note}</li>
              ))}
            </ul>
          ) : null}
        </div>
        <div className="admin-ai-config__status">
          <strong>{changedKeys.length}</strong>
          <span>处待保存改动</span>
        </div>
      </div>

      <div className="admin-ai-config__grid">
        {runtimeSummary.map((item) => (
          <article className="admin-ai-field" key={item.label}>
            <span className="admin-ai-field__label">{item.label}</span>
            <strong>{item.value}</strong>
          </article>
        ))}
      </div>

      <form className="admin-ai-config__form" onSubmit={handleSubmit}>
        <section className="admin-ai-config__section">
          <div className="admin-ai-config__section-head">
            <h3>提供方与模型</h3>
            <p>主模型、兜底 Provider 与访问凭据。</p>
          </div>
          <div className="admin-ai-config__grid">
            {groupedFields.provider.map((field) => (
              <label className="admin-ai-field" key={field.key}>
                <span className="admin-ai-field__label">{field.label}</span>
                <span className="admin-ai-field__hint">{field.description}</span>
                {field.options ? (
                  <select
                    value={draft[field.key] ?? ''}
                    onChange={(event) => updateDraftValue(field.key, event.target.value)}
                  >
                    {field.options.map((option) => (
                      <option key={option.value} value={option.value}>
                        {option.label}
                      </option>
                    ))}
                  </select>
                ) : (
                  <input
                    type={field.secret ? 'password' : 'text'}
                    value={draft[field.key] ?? ''}
                    placeholder={field.placeholder}
                    onChange={(event) => updateDraftValue(field.key, event.target.value)}
                  />
                )}
                <code>{field.key}</code>
              </label>
            ))}
          </div>
        </section>

        <section className="admin-ai-config__section">
          <div className="admin-ai-config__section-head">
            <h3>运行时参数</h3>
            <p>采样、超时与流式响应开关。</p>
          </div>
          <div className="admin-ai-config__grid">
            {groupedFields.runtime.map((field) => (
              <label className="admin-ai-field" key={field.key}>
                <span className="admin-ai-field__label">{field.label}</span>
                <span className="admin-ai-field__hint">{field.description}</span>
                {field.type === 'boolean' ? (
                  <span className="admin-ai-field__checkbox">
                    <input
                      type="checkbox"
                      checked={parseBooleanConfigValue(draft[field.key])}
                      onChange={(event) => updateDraftValue(field.key, String(event.target.checked))}
                    />
                    <span>{parseBooleanConfigValue(draft[field.key]) ? '已开启' : '已关闭'}</span>
                  </span>
                ) : (
                  <input
                    type="number"
                    step={resolveNumberStep(field.key)}
                    value={draft[field.key] ?? ''}
                    onChange={(event) => updateDraftValue(field.key, event.target.value)}
                  />
                )}
                <code>{field.key}</code>
              </label>
            ))}
          </div>
        </section>

        <section className="admin-ai-config__section">
          <div className="admin-ai-config__section-head">
            <h3>场景模型覆盖</h3>
            <p>留空表示沿用默认模型，仅在场景差异明显时单独指定。题卡流水线使用面试场景模型。</p>
          </div>
          <div className="admin-ai-config__grid">
            {groupedFields.scene.map((field) => (
              <label className="admin-ai-field" key={field.key}>
                <span className="admin-ai-field__label">{field.label}</span>
                <span className="admin-ai-field__hint">{field.description}</span>
                <input
                  type="text"
                  value={draft[field.key] ?? ''}
                  placeholder={field.placeholder}
                  onChange={(event) => updateDraftValue(field.key, event.target.value)}
                />
                <code>{field.key}</code>
              </label>
            ))}
          </div>
        </section>

        <div className="admin-ai-config__footer">
          <p className="admin-ai-config__message">{message}</p>
          <div className="admin-ai-config__actions">
            <button className="admin-link" type="button" onClick={resetDraft} disabled={saveMutation.isPending}>
              恢复生效值
            </button>
            <button
              className="admin-link"
              type="submit"
              disabled={saveMutation.isPending || changedKeys.length === 0}
            >
              {saveMutation.isPending ? '保存中...' : '保存 AI 配置'}
            </button>
          </div>
        </div>
      </form>
    </section>
  )
}
