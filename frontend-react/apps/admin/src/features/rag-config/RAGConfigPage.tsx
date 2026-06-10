import type { FormEvent } from 'react'
import { useEffect, useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { extractErrorMessage, requestJson } from '@makejob/api-client'
import { isSuccessCode, type ApiEnvelope } from '@makejob/shared-types'
import { useAdminAuthStore } from '../../state/auth'

type RAGConfigGroup = 'basic' | 'milvus' | 'embedding'

interface AdminConfigItem {
  id?: number
  config_key: string
  config_value: string
  config_type: string
  description?: string
}

interface RAGSystemStatus {
  enabled: boolean
  milvus_connected: boolean
  collection: string
  embed_model: string
}

interface RAGConfigResponse {
  configs: Record<string, string>
  items: AdminConfigItem[]
  status: RAGSystemStatus
  warnings: string[]
}

interface RAGConnectionTestResult {
  milvus_ok: boolean
  embedding_ok: boolean
  error?: string
}

interface RAGConfigFieldMeta {
  key: string
  label: string
  description: string
  type: 'string' | 'number' | 'boolean'
  group: RAGConfigGroup
  secret?: boolean
  placeholder?: string
  options?: { value: string; label: string }[]
}

const RAG_CONFIG_FIELDS: RAGConfigFieldMeta[] = [
  {
    key: 'ai_rag_enabled',
    label: '启用RAG',
    description: '是否启用RAG语义检索功能',
    type: 'boolean',
    group: 'basic',
  },
  {
    key: 'ai_rag_collection',
    label: 'Collection名称',
    description: 'Milvus中的Collection名称',
    type: 'string',
    group: 'basic',
    placeholder: 'interview_questions',
  },
  {
    key: 'ai_rag_top_k',
    label: '返回数量',
    description: '默认返回的相似文档数量（1-50）',
    type: 'number',
    group: 'basic',
  },
  {
    key: 'ai_rag_score_threshold',
    label: '相似度阈值',
    description: '低于此阈值的结果将被过滤（0-1）',
    type: 'number',
    group: 'basic',
  },
  {
    key: 'ai_rag_milvus_addr',
    label: 'Milvus地址',
    description: 'Milvus服务地址',
    type: 'string',
    group: 'milvus',
    placeholder: 'localhost:19530',
  },
  {
    key: 'ai_rag_milvus_user',
    label: '用户名',
    description: 'Milvus用户名',
    type: 'string',
    group: 'milvus',
    placeholder: 'root',
  },
  {
    key: 'ai_rag_milvus_password',
    label: '密码',
    description: 'Milvus密码',
    type: 'string',
    group: 'milvus',
    secret: true,
    placeholder: 'Milvus',
  },
  {
    key: 'ai_rag_embed_api_key',
    label: 'API Key',
    description: '火山引擎API Key（留空则复用主API Key）',
    type: 'string',
    group: 'embedding',
    secret: true,
    placeholder: '留空复用主API Key',
  },
  {
    key: 'ai_rag_embed_model',
    label: '模型ID',
    description: 'Embedding模型ID（如doubao-embedding-large-text-240915）',
    type: 'string',
    group: 'embedding',
    placeholder: 'doubao-embedding-large-text-240915',
  },
  {
    key: 'ai_rag_embed_base_url',
    label: 'API端点',
    description: 'Ark API端点地址',
    type: 'string',
    group: 'embedding',
    placeholder: 'https://ark.cn-beijing.volces.com/api/v3',
  },
]

const GROUP_LABELS: Record<RAGConfigGroup, string> = {
  basic: '基础配置',
  milvus: 'Milvus配置',
  embedding: 'Embedding配置',
}

async function fetchRAGConfigs(token: string | null): Promise<RAGConfigResponse> {
  const response = await requestJson<ApiEnvelope<RAGConfigResponse>>('/admin/rag-configs', {
    method: 'GET',
    token,
  })
  if (!isSuccessCode(response.code)) {
    throw new Error(response.message || '获取RAG配置失败')
  }
  return response.data
}

async function updateRAGConfigs(token: string | null, configs: Record<string, string>): Promise<void> {
  const response = await requestJson<ApiEnvelope<unknown>>('/admin/rag-configs', {
    method: 'PUT',
    token,
    body: configs,
  })
  if (!isSuccessCode(response.code)) {
    throw new Error(response.message || '保存RAG配置失败')
  }
}

async function testRAGConnection(token: string | null): Promise<RAGConnectionTestResult> {
  const response = await requestJson<ApiEnvelope<RAGConnectionTestResult>>('/admin/rag-configs/test', {
    method: 'POST',
    token,
  })
  if (!isSuccessCode(response.code)) {
    throw new Error(response.message || '测试连接失败')
  }
  return response.data
}

function groupRAGConfigFields(metas: RAGConfigFieldMeta[]): Record<RAGConfigGroup, RAGConfigFieldMeta[]> {
  return metas.reduce<Record<RAGConfigGroup, RAGConfigFieldMeta[]>>(
    (result, field) => {
      result[field.group].push(field)
      return result
    },
    { basic: [], milvus: [], embedding: [] },
  )
}

function normalizeRAGConfigValue(meta: RAGConfigFieldMeta, value: string): string {
  if (meta.type === 'boolean') {
    return String(value).trim().toLowerCase() === 'true' ? 'true' : 'false'
  }
  return value.trim()
}

function collectChangedKeys(
  draft: Record<string, string>,
  configs: Record<string, string>,
  metas: RAGConfigFieldMeta[],
): string[] {
  return metas
    .filter((meta) => normalizeRAGConfigValue(meta, draft[meta.key] ?? '') !== (configs[meta.key] ?? ''))
    .map((meta) => meta.key)
}

function getEmbedModelLabel(model: string): string {
  if (!model) return '未配置'
  return model
}

export function RAGConfigPage() {
  const accessToken = useAdminAuthStore((state) => state.accessToken)
  const queryClient = useQueryClient()
  const [draft, setDraft] = useState<Record<string, string>>({})
  const [message, setMessage] = useState('读取RAG配置中')
  const [testResult, setTestResult] = useState<RAGConnectionTestResult | null>(null)

  const configQuery = useQuery({
    queryKey: ['admin', 'rag-configs', accessToken],
    queryFn: () => fetchRAGConfigs(accessToken),
    enabled: Boolean(accessToken),
  })

  const groupedFields = useMemo(() => groupRAGConfigFields(RAG_CONFIG_FIELDS), [])

  useEffect(() => {
    if (configQuery.data?.configs) {
      setDraft(configQuery.data.configs)
      setMessage('已同步当前RAG配置，可以直接编辑。')
    }
  }, [configQuery.data])

  const changedKeys = useMemo(() => {
    if (!configQuery.data) return []
    return collectChangedKeys(draft, configQuery.data.configs, RAG_CONFIG_FIELDS)
  }, [draft, configQuery.data])

  const saveMutation = useMutation({
    mutationFn: async () => {
      await updateRAGConfigs(accessToken, draft)
    },
    onSuccess: async () => {
      setMessage('RAG配置已保存。')
      setTestResult(null)
      await queryClient.invalidateQueries({ queryKey: ['admin', 'rag-configs'] })
    },
    onError: (error) => {
      setMessage(extractErrorMessage(error, '保存RAG配置失败'))
    },
  })

  const testMutation = useMutation({
    mutationFn: async () => {
      return testRAGConnection(accessToken)
    },
    onSuccess: (result) => {
      setTestResult(result)
      setMessage('连接测试完成。')
    },
    onError: (error) => {
      setMessage(extractErrorMessage(error, '测试连接失败'))
    },
  })

  function updateDraft(key: string, value: string): void {
    setDraft((prev) => ({ ...prev, [key]: value }))
  }

  function handleSubmit(event: FormEvent<HTMLFormElement>): void {
    event.preventDefault()
    setMessage('正在保存RAG配置...')
    saveMutation.mutate()
  }

  function handleTest(): void {
    setMessage('正在测试连接...')
    testMutation.mutate()
  }

  if (configQuery.isLoading) {
    return (
      <section className="admin-panel">
        <span className="admin-tag">RAG系统</span>
        <h2>RAG 配置</h2>
        <p className="admin-copy">正在加载RAG配置。</p>
      </section>
    )
  }

  if (configQuery.isError || !configQuery.data) {
    return (
      <section className="admin-panel">
        <span className="admin-tag">RAG系统</span>
        <h2>RAG 配置</h2>
        <p className="admin-copy">{extractErrorMessage(configQuery.error, '读取RAG配置失败')}</p>
      </section>
    )
  }

  const status = configQuery.data.status || { enabled: false, embed_model: '', collection: '' }

  return (
    <section className="admin-panel admin-rag-config">
      <div className="admin-rag-config__hero">
        <div>
          <span className="admin-tag">RAG系统</span>
          <h2>RAG 配置</h2>
          <p className="admin-copy">
            管理RAG语义检索系统的配置，包括Milvus向量数据库连接和Embedding模型设置。保存配置后需要重启服务或等待热更新生效。
          </p>
        </div>
        <div className="admin-rag-config__status">
          <strong>{changedKeys.length}</strong>
          <span>处待保存改动</span>
        </div>
      </div>

      {/* 状态概览 */}
      <div className="admin-rag-config__summary">
        <article className="admin-rag-field">
          <span className="admin-rag-field__label">RAG状态</span>
          <strong className={status.enabled ? 'is-valid' : ''}>{status.enabled ? '已启用' : '未启用'}</strong>
        </article>
        <article className="admin-rag-field">
          <span className="admin-rag-field__label">Embedding模型</span>
          <strong>{getEmbedModelLabel(status.embed_model)}</strong>
        </article>
        <article className="admin-rag-field">
          <span className="admin-rag-field__label">Collection</span>
          <strong>{status.collection || '-'}</strong>
        </article>
      </div>

      {/* 警告信息 */}
      {(configQuery.data.warnings || []).length > 0 && (
        <div className="admin-rag-config__warnings">
          {(configQuery.data.warnings || []).map((warning) => (
            <p key={warning} className="is-warning">{warning}</p>
          ))}
        </div>
      )}

      {/* 配置表单 */}
      <form className="admin-rag-config__form" onSubmit={handleSubmit}>
        {/* 基础配置 */}
        <section className="admin-rag-config__section">
          <div className="admin-rag-config__section-head">
            <h3>{GROUP_LABELS.basic}</h3>
            <p>RAG系统的基础配置项。</p>
          </div>
          <div className="admin-rag-config__grid">
            {groupedFields.basic.map((field) => (
              <label className="admin-ai-field" key={field.key}>
                <span className="admin-ai-field__label">{field.label}</span>
                <span className="admin-ai-field__hint">{field.description}</span>
                {field.type === 'boolean' ? (
                  <div className="admin-ai-config__switch">
                    <input
                      type="checkbox"
                      checked={draft[field.key] === 'true'}
                      onChange={(e) => updateDraft(field.key, e.target.checked ? 'true' : 'false')}
                    />
                    <span>{draft[field.key] === 'true' ? '已启用' : '未启用'}</span>
                  </div>
                ) : (
                  <input
                    type={field.type === 'number' ? 'number' : 'text'}
                    value={draft[field.key] ?? ''}
                    onChange={(e) => updateDraft(field.key, e.target.value)}
                    placeholder={field.placeholder}
                    step={field.type === 'number' ? 'any' : undefined}
                  />
                )}
              </label>
            ))}
          </div>
        </section>

        {/* Milvus配置 */}
        <section className="admin-rag-config__section">
          <div className="admin-rag-config__section-head">
            <h3>{GROUP_LABELS.milvus}</h3>
            <p>Milvus向量数据库连接配置。</p>
          </div>
          <div className="admin-rag-config__grid">
            {groupedFields.milvus.map((field) => (
              <label className="admin-ai-field" key={field.key}>
                <span className="admin-ai-field__label">{field.label}</span>
                <span className="admin-ai-field__hint">{field.description}</span>
                <input
                  type={field.secret ? 'password' : 'text'}
                  value={draft[field.key] ?? ''}
                  onChange={(e) => updateDraft(field.key, e.target.value)}
                  placeholder={field.placeholder}
                />
              </label>
            ))}
          </div>
        </section>

        {/* Embedding配置 */}
        <section className="admin-rag-config__section">
          <div className="admin-rag-config__section-head">
            <h3>{GROUP_LABELS.embedding}</h3>
            <p>Embedding模型配置。</p>
          </div>
          <div className="admin-rag-config__grid">
            {groupedFields.embedding.map((field) => (
              <label className="admin-ai-field" key={field.key}>
                <span className="admin-ai-field__label">{field.label}</span>
                <span className="admin-ai-field__hint">{field.description}</span>
                {field.options ? (
                  <select
                    value={draft[field.key] ?? ''}
                    onChange={(e) => updateDraft(field.key, e.target.value)}
                  >
                    {field.options.map((opt) => (
                      <option key={opt.value} value={opt.value}>{opt.label}</option>
                    ))}
                  </select>
                ) : (
                  <input
                    type={field.secret ? 'password' : 'text'}
                    value={draft[field.key] ?? ''}
                    onChange={(e) => updateDraft(field.key, e.target.value)}
                    placeholder={field.placeholder}
                  />
                )}
              </label>
            ))}
          </div>
        </section>

        {/* 操作按钮 */}
        <div className="admin-rag-config__actions">
          <button
            className="admin-link"
            type="submit"
            disabled={changedKeys.length === 0 || saveMutation.isPending}
          >
            {saveMutation.isPending ? '保存中...' : `保存配置 (${changedKeys.length}项变更)`}
          </button>
          <button
            className="admin-link"
            type="button"
            onClick={handleTest}
            disabled={testMutation.isPending}
          >
            {testMutation.isPending ? '测试中...' : '测试连接'}
          </button>
        </div>
      </form>

      {/* 测试结果 */}
      {testResult && (
        <div className="admin-rag-config__test-result">
          <h3>连接测试结果</h3>
          <div className="admin-rag-config__test-items">
            <div className={testResult.milvus_ok ? 'is-valid' : 'is-error'}>
              <span>Milvus连接</span>
              <strong>{testResult.milvus_ok ? '正常' : '失败'}</strong>
            </div>
            <div className={testResult.embedding_ok ? 'is-valid' : 'is-error'}>
              <span>Embedding连接</span>
              <strong>{testResult.embedding_ok ? '正常' : '失败'}</strong>
            </div>
          </div>
          {testResult.error && <p className="is-error">{testResult.error}</p>}
        </div>
      )}

      {/* 状态消息 */}
      <div className="admin-rag-config__message">
        <p>{message}</p>
      </div>
    </section>
  )
}
