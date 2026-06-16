import type { FormEvent } from 'react'
import { useEffect, useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  DatabaseOutlined,
  CheckCircleOutlined,
  CloseCircleOutlined,
  ThunderboltOutlined,
  SaveOutlined,
  WarningOutlined,
} from '@ant-design/icons'
import { Button, Card, Input, InputNumber, Switch, Tag, Tooltip } from 'antd'
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

const GROUP_LABELS: Record<RAGConfigGroup, { label: string; color: string }> = {
  basic: { label: '基础配置', color: '#3b82f6' },
  milvus: { label: 'Milvus配置', color: '#8b5cf6' },
  embedding: { label: 'Embedding配置', color: '#10b981' },
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
  const [messageText, setMessageText] = useState('读取RAG配置中')
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
      setMessageText('已同步当前RAG配置，可以直接编辑。')
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
      setMessageText('RAG配置已保存。')
      setTestResult(null)
      await queryClient.invalidateQueries({ queryKey: ['admin', 'rag-configs'] })
    },
    onError: (error) => {
      setMessageText(extractErrorMessage(error, '保存RAG配置失败'))
    },
  })

  const testMutation = useMutation({
    mutationFn: async () => {
      return testRAGConnection(accessToken)
    },
    onSuccess: (result) => {
      setTestResult(result)
      setMessageText('连接测试完成。')
    },
    onError: (error) => {
      setMessageText(extractErrorMessage(error, '测试连接失败'))
    },
  })

  function updateDraft(key: string, value: string): void {
    setDraft((prev) => ({ ...prev, [key]: value }))
  }

  function handleSubmit(event: FormEvent<HTMLFormElement>): void {
    event.preventDefault()
    setMessageText('正在保存RAG配置...')
    saveMutation.mutate()
  }

  function handleTest(): void {
    setMessageText('正在测试连接...')
    testMutation.mutate()
  }

  if (configQuery.isLoading) {
    return (
      <div style={{ padding: '24px 32px 32px', background: THEME.bg, minHeight: '100vh' }}>
        <div style={{ ...glassCard, padding: '24px 28px' }}>
          <h2 style={{ margin: 0, fontSize: 22, fontWeight: 700, color: THEME.textMain }}>RAG 配置</h2>
          <p style={{ margin: '8px 0 0', color: THEME.textSecondary }}>正在加载RAG配置...</p>
        </div>
      </div>
    )
  }

  if (configQuery.isError || !configQuery.data) {
    return (
      <div style={{ padding: '24px 32px 32px', background: THEME.bg, minHeight: '100vh' }}>
        <div style={{ ...glassCard, padding: '24px 28px' }}>
          <h2 style={{ margin: 0, fontSize: 22, fontWeight: 700, color: THEME.textMain }}>RAG 配置</h2>
          <p style={{ margin: '8px 0 0', color: THEME.danger }}>
            {extractErrorMessage(configQuery.error, '读取RAG配置失败')}
          </p>
        </div>
      </div>
    )
  }

  const status = configQuery.data.status || { enabled: false, embed_model: '', collection: '' }

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
              background: 'linear-gradient(135deg, #6366f1, #4f46e5)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              boxShadow: '0 4px 14px rgba(79, 70, 229, 0.35)',
              flexShrink: 0,
            }}
          >
            <DatabaseOutlined style={{ fontSize: 22, color: '#fff' }} />
          </div>
          <div>
            <h1 style={{ margin: 0, fontSize: 22, fontWeight: 700, color: THEME.textMain, lineHeight: 1.3 }}>
              RAG 配置
            </h1>
            <p style={{ margin: '4px 0 0', fontSize: 13, color: THEME.textSecondary }}>
              管理RAG语义检索系统的配置，包括Milvus向量数据库连接和Embedding模型设置
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
            <span
              style={{
                fontSize: 24,
                fontWeight: 700,
                color: status.enabled ? THEME.success : THEME.textMuted,
              }}
            >
              {status.enabled ? '已启用' : '未启用'}
            </span>
            <span style={{ fontSize: 12, color: THEME.textSecondary }}>RAG状态</span>
          </div>
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
            <span style={{ fontSize: 24, fontWeight: 700, color: THEME.primary }}>{changedKeys.length}</span>
            <span style={{ fontSize: 12, color: THEME.textSecondary }}>处待保存改动</span>
          </div>
        </div>
      </div>

      {/* Status Summary Cards */}
      <div
        style={{
          display: 'grid',
          gridTemplateColumns: 'repeat(auto-fit, minmax(200px, 1fr))',
          gap: 16,
          marginBottom: 20,
        }}
      >
        <div style={{ ...solidCard, padding: '18px 20px', display: 'flex', alignItems: 'center', gap: 14 }}>
          <div
            style={{
              width: 40,
              height: 40,
              borderRadius: 12,
              background: status.enabled ? 'rgba(16, 185, 129, 0.1)' : 'rgba(148, 163, 184, 0.1)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              flexShrink: 0,
            }}
          >
            {status.enabled ? (
              <CheckCircleOutlined style={{ fontSize: 20, color: THEME.success }} />
            ) : (
              <CloseCircleOutlined style={{ fontSize: 20, color: THEME.textMuted }} />
            )}
          </div>
          <div>
            <div style={{ fontSize: 12, color: THEME.textSecondary, marginBottom: 2 }}>RAG状态</div>
            <div style={{ fontSize: 16, fontWeight: 600, color: status.enabled ? THEME.success : THEME.textMuted }}>
              {status.enabled ? '已启用' : '未启用'}
            </div>
          </div>
        </div>

        <div style={{ ...solidCard, padding: '18px 20px', display: 'flex', alignItems: 'center', gap: 14 }}>
          <div
            style={{
              width: 40,
              height: 40,
              borderRadius: 12,
              background: 'rgba(99, 102, 241, 0.1)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              flexShrink: 0,
            }}
          >
            <DatabaseOutlined style={{ fontSize: 20, color: THEME.primary }} />
          </div>
          <div>
            <div style={{ fontSize: 12, color: THEME.textSecondary, marginBottom: 2 }}>Collection</div>
            <Tooltip title={status.collection || '未配置'}>
              <div
                style={{
                  fontSize: 16,
                  fontWeight: 600,
                  color: status.collection ? THEME.textMain : THEME.textMuted,
                  overflow: 'hidden',
                  textOverflow: 'ellipsis',
                  whiteSpace: 'nowrap',
                  maxWidth: 200,
                }}
              >
                {status.collection || '未配置'}
              </div>
            </Tooltip>
          </div>
        </div>

        <div style={{ ...solidCard, padding: '18px 20px', display: 'flex', alignItems: 'center', gap: 14 }}>
          <div
            style={{
              width: 40,
              height: 40,
              borderRadius: 12,
              background: 'rgba(139, 92, 246, 0.1)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              flexShrink: 0,
            }}
          >
            <ThunderboltOutlined style={{ fontSize: 20, color: '#8b5cf6' }} />
          </div>
          <div style={{ minWidth: 0 }}>
            <div style={{ fontSize: 12, color: THEME.textSecondary, marginBottom: 2 }}>Embedding模型</div>
            <Tooltip title={getEmbedModelLabel(status.embed_model)}>
              <div
                style={{
                  fontSize: 16,
                  fontWeight: 600,
                  color: status.embed_model ? THEME.textMain : THEME.textMuted,
                  overflow: 'hidden',
                  textOverflow: 'ellipsis',
                  whiteSpace: 'nowrap',
                  maxWidth: 240,
                }}
              >
                {getEmbedModelLabel(status.embed_model)}
              </div>
            </Tooltip>
          </div>
        </div>
      </div>

      {/* Warnings */}
      {(configQuery.data.warnings || []).length > 0 && (
        <div
          style={{
            ...solidCard,
            padding: '16px 20px',
            marginBottom: 20,
            background: '#fffbeb',
            border: '1px solid #fde68a',
          }}
        >
          {(configQuery.data.warnings || []).map((warning) => (
            <div
              key={warning}
              style={{
                display: 'flex',
                alignItems: 'center',
                gap: 8,
                color: '#92400e',
                fontSize: 13,
                fontWeight: 500,
              }}
            >
              <WarningOutlined />
              {warning}
            </div>
          ))}
        </div>
      )}

      {/* Config Form */}
      <form onSubmit={handleSubmit} style={{ display: 'flex', flexDirection: 'column', gap: 20 }}>
        {(Object.keys(groupedFields) as RAGConfigGroup[]).map((group) => {
          const groupCfg = GROUP_LABELS[group]
          const fields = groupedFields[group]
          return (
            <Card
              key={group}
              title={
                <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
                  <div
                    style={{
                      width: 6,
                      height: 18,
                      borderRadius: 3,
                      background: groupCfg.color,
                    }}
                  />
                  <span style={{ fontSize: 16, fontWeight: 600, color: THEME.textMain }}>{groupCfg.label}</span>
                </div>
              }
              style={{ ...solidCard }}
              bodyStyle={{ padding: '20px 24px' }}
            >
              <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(300px, 1fr))', gap: '20px 24px' }}>
                {fields.map((field) => (
                  <div key={field.key}>
                    <label
                      style={{
                        display: 'block',
                        fontSize: 13,
                        fontWeight: 500,
                        color: THEME.textSecondary,
                        marginBottom: 6,
                      }}
                    >
                      {field.label}
                    </label>
                    <div style={{ fontSize: 12, color: THEME.textMuted, marginBottom: 8, lineHeight: 1.4 }}>
                      {field.description}
                    </div>
                    {field.type === 'boolean' ? (
                      <Switch
                        checked={draft[field.key] === 'true'}
                        onChange={(checked) => updateDraft(field.key, checked ? 'true' : 'false')}
                        checkedChildren="已启用"
                        unCheckedChildren="未启用"
                      />
                    ) : field.type === 'number' ? (
                      <InputNumber
                        value={draft[field.key] ? Number(draft[field.key]) : undefined}
                        onChange={(val) => updateDraft(field.key, val !== null && val !== undefined ? String(val) : '')}
                        placeholder={field.placeholder}
                        step={field.key.includes('score') ? 0.1 : 1}
                        style={{ width: '100%', borderRadius: 10 }}
                      />
                    ) : (
                      <Input
                        type={field.secret ? 'password' : 'text'}
                        value={draft[field.key] ?? ''}
                        onChange={(e) => updateDraft(field.key, e.target.value)}
                        placeholder={field.placeholder}
                        style={{ borderRadius: 10 }}
                      />
                    )}
                  </div>
                ))}
              </div>
            </Card>
          )
        })}

        {/* Sticky Action Bar */}
        <div
          style={{
            ...glassCard,
            padding: '14px 24px',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            gap: 12,
            position: 'sticky',
            bottom: 16,
            zIndex: 10,
          }}
        >
          <div style={{ fontSize: 13, color: THEME.textSecondary }}>
            {changedKeys.length > 0 ? (
              <span>
                已修改 <strong style={{ color: THEME.primary }}>{changedKeys.length}</strong> 项配置
              </span>
            ) : (
              <span>暂无未保存的修改</span>
            )}
          </div>
          <div style={{ display: 'flex', gap: 10 }}>
            <Button
              icon={<ThunderboltOutlined />}
              onClick={handleTest}
              loading={testMutation.isPending}
              disabled={saveMutation.isPending}
              style={{ borderRadius: 10 }}
            >
              {testMutation.isPending ? '测试中...' : '测试连接'}
            </Button>
            <Button
              type="primary"
              htmlType="submit"
              icon={<SaveOutlined />}
              loading={saveMutation.isPending}
              disabled={changedKeys.length === 0 || saveMutation.isPending}
              style={{
                borderRadius: 10,
                background: 'linear-gradient(135deg, #3b82f6, #2563eb)',
                border: 'none',
                boxShadow: '0 4px 14px rgba(37, 99, 235, 0.35)',
              }}
            >
              {saveMutation.isPending ? '保存中...' : `保存配置`}
            </Button>
          </div>
        </div>
      </form>

      {/* Test Result */}
      {testResult && (
        <div style={{ ...solidCard, padding: '20px 24px', marginTop: 4, marginBottom: 20 }}>
          <h3 style={{ margin: '0 0 16px', fontSize: 15, fontWeight: 600, color: THEME.textMain }}>连接测试结果</h3>
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(200px, 1fr))', gap: 12 }}>
            <div
              style={{
                padding: '14px 18px',
                borderRadius: 12,
                background: testResult.milvus_ok ? '#f0fdf4' : '#fef2f2',
                border: `1px solid ${testResult.milvus_ok ? '#bbf7d0' : '#fecaca'}`,
                display: 'flex',
                alignItems: 'center',
                gap: 10,
              }}
            >
              {testResult.milvus_ok ? (
                <CheckCircleOutlined style={{ fontSize: 20, color: THEME.success }} />
              ) : (
                <CloseCircleOutlined style={{ fontSize: 20, color: THEME.danger }} />
              )}
              <div>
                <div style={{ fontSize: 12, color: THEME.textSecondary }}>Milvus连接</div>
                <div
                  style={{
                    fontSize: 15,
                    fontWeight: 600,
                    color: testResult.milvus_ok ? THEME.success : THEME.danger,
                  }}
                >
                  {testResult.milvus_ok ? '正常' : '失败'}
                </div>
              </div>
            </div>
            <div
              style={{
                padding: '14px 18px',
                borderRadius: 12,
                background: testResult.embedding_ok ? '#f0fdf4' : '#fef2f2',
                border: `1px solid ${testResult.embedding_ok ? '#bbf7d0' : '#fecaca'}`,
                display: 'flex',
                alignItems: 'center',
                gap: 10,
              }}
            >
              {testResult.embedding_ok ? (
                <CheckCircleOutlined style={{ fontSize: 20, color: THEME.success }} />
              ) : (
                <CloseCircleOutlined style={{ fontSize: 20, color: THEME.danger }} />
              )}
              <div>
                <div style={{ fontSize: 12, color: THEME.textSecondary }}>Embedding连接</div>
                <div
                  style={{
                    fontSize: 15,
                    fontWeight: 600,
                    color: testResult.embedding_ok ? THEME.success : THEME.danger,
                  }}
                >
                  {testResult.embedding_ok ? '正常' : '失败'}
                </div>
              </div>
            </div>
          </div>
          {testResult.error && (
            <div
              style={{
                marginTop: 12,
                padding: '10px 14px',
                borderRadius: 10,
                background: '#fef2f2',
                border: '1px solid #fecaca',
                color: '#dc2626',
                fontSize: 13,
              }}
            >
              {testResult.error}
            </div>
          )}
        </div>
      )}

      {/* Message Toast */}
      {messageText && (
        <div
          style={{
            position: 'fixed',
            bottom: 24,
            left: '50%',
            transform: 'translateX(-50%)',
            zIndex: 1000,
            padding: '12px 24px',
            borderRadius: 12,
            background: messageText.includes('失败') || messageText.includes('错误') ? '#fef2f2' : '#f0fdf4',
            border: `1px solid ${messageText.includes('失败') || messageText.includes('错误') ? '#fecaca' : '#bbf7d0'}`,
            color: messageText.includes('失败') || messageText.includes('错误') ? '#dc2626' : '#16a34a',
            fontSize: 14,
            fontWeight: 500,
            boxShadow: THEME.shadow,
            animation: 'fadeInUp 0.3s ease',
          }}
        >
          {messageText}
        </div>
      )}

      <style>{`
        @keyframes fadeInUp {
          from { opacity: 0; transform: translateX(-50%) translateY(10px); }
          to { opacity: 1; transform: translateX(-50%) translateY(0); }
        }
      `}</style>
    </div>
  )
}
