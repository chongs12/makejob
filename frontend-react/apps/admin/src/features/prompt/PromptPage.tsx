import type { FormEvent } from 'react'
import { useEffect, useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  FileTextOutlined,
  PlusOutlined,
  DeleteOutlined,
  SaveOutlined,
  ReloadOutlined,
  SearchOutlined,
  InboxOutlined,
  ExclamationCircleOutlined,
} from '@ant-design/icons'
import { Button, Card, Input, Modal, Select, Switch, Tag, Tooltip } from 'antd'
import { extractErrorMessage, requestJson } from '@makejob/api-client'
import { isSuccessCode, type ApiEnvelope } from '@makejob/shared-types'
import { useAdminAuthStore } from '../../state/auth'

type PromptScene = 'interview' | 'companion' | 'quiz' | 'plan'

interface Industry {
  id: number
  code: string
  name: string
  is_active: boolean
}

interface PromptTemplate {
  id: number
  industry_id?: number | null
  name: string
  scene: PromptScene
  template_content: string
  variables: string
  is_active: boolean
  created_at?: string
  updated_at?: string
}

interface PromptFormState {
  name: string
  scene: PromptScene
  industryId: string
  templateContent: string
  variables: string
  isActive: boolean
}

interface PromptFilters {
  scene: string
  industryId: string
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

const SCENE_CONFIG: Record<PromptScene, { label: string; color: string }> = {
  interview: { label: '面试', color: '#8b5cf6' },
  companion: { label: '陪伴', color: '#3b82f6' },
  quiz: { label: '刷题', color: '#f59e0b' },
  plan: { label: '学习计划', color: '#10b981' },
}

const PROMPT_SCENE_OPTIONS: Array<{ value: PromptScene; label: string }> = [
  { value: 'interview', label: '面试' },
  { value: 'companion', label: '陪伴' },
  { value: 'quiz', label: '刷题' },
  { value: 'plan', label: '学习计划' },
]

/**
 * 拉取后台行业列表，用于 Prompt 过滤器和行业名称映射。
 */
async function fetchIndustries(token: string | null): Promise<Industry[]> {
  const response = await requestJson<ApiEnvelope<Industry[]>>('/admin/industries', {
    method: 'GET',
    token,
  })

  if (!isSuccessCode(response.code)) {
    throw new Error(response.message || '获取行业列表失败')
  }

  return response.data
}

/**
 * 按当前筛选条件读取 Prompt 模板列表，直接复用后台过滤能力。
 */
async function fetchPrompts(token: string | null, filters: PromptFilters): Promise<PromptTemplate[]> {
  const searchParams = new URLSearchParams()
  if (filters.scene) {
    searchParams.set('scene', filters.scene)
  }
  if (filters.industryId) {
    searchParams.set('industry_id', filters.industryId)
  }

  const query = searchParams.toString()
  const path = query ? `/admin/prompts?${query}` : '/admin/prompts'
  const response = await requestJson<ApiEnvelope<PromptTemplate[]>>(path, {
    method: 'GET',
    token,
  })

  if (!isSuccessCode(response.code)) {
    throw new Error(response.message || '获取 Prompt 模板失败')
  }

  return response.data
}

/**
 * 创建新的 Prompt 模板，并返回服务端持久化后的结果。
 */
async function createPrompt(token: string | null, payload: Record<string, unknown>): Promise<PromptTemplate> {
  const response = await requestJson<ApiEnvelope<PromptTemplate>>('/admin/prompts', {
    method: 'POST',
    token,
    body: payload,
  })

  if (!isSuccessCode(response.code)) {
    throw new Error(response.message || '创建 Prompt 模板失败')
  }

  return response.data
}

/**
 * 更新指定 Prompt 模板，保持前端编辑状态与后台记录一致。
 */
async function updatePrompt(token: string | null, id: number, payload: Record<string, unknown>): Promise<void> {
  const response = await requestJson<ApiEnvelope<null>>(`/admin/prompts/${id}`, {
    method: 'PUT',
    token,
    body: payload,
  })

  if (!isSuccessCode(response.code)) {
    throw new Error(response.message || '更新 Prompt 模板失败')
  }
}

/**
 * 删除指定 Prompt 模板，供管理页快速清理无效配置。
 */
async function deletePrompt(token: string | null, id: number): Promise<void> {
  const response = await requestJson<ApiEnvelope<null>>(`/admin/prompts/${id}`, {
    method: 'DELETE',
    token,
  })

  if (!isSuccessCode(response.code)) {
    throw new Error(response.message || '删除 Prompt 模板失败')
  }
}

/**
 * 构造 Prompt 编辑表单默认值，避免首次渲染时出现空对象。
 */
function buildInitialPromptForm(): PromptFormState {
  return {
    name: '',
    scene: 'companion',
    industryId: '',
    templateContent: '',
    variables: '',
    isActive: true,
  }
}

/**
 * 将 Prompt 记录转换为前端可编辑草稿，统一新建和编辑态的数据结构。
 */
function buildPromptForm(prompt?: PromptTemplate | null): PromptFormState {
  if (!prompt) {
    return buildInitialPromptForm()
  }

  return {
    name: prompt.name,
    scene: prompt.scene,
    industryId: prompt.industry_id ? String(prompt.industry_id) : '',
    templateContent: prompt.template_content,
    variables: prompt.variables || '',
    isActive: prompt.is_active,
  }
}

/**
 * 将表单状态整理为后端可直接消费的请求体。
 */
function buildPromptPayload(form: PromptFormState): Record<string, unknown> {
  const payload: Record<string, unknown> = {
    name: form.name.trim(),
    scene: form.scene,
    template_content: form.templateContent.trim(),
    variables: form.variables.trim(),
    is_active: form.isActive,
  }

  if (form.industryId) {
    payload.industry_id = Number(form.industryId)
  } else {
    payload.industry_id = null
  }

  return payload
}

/**
 * 根据行业 ID 生成可读行业名称，空值时按通用模板展示。
 */
function resolvePromptIndustryName(industryId: number | null | undefined, industryMap: Map<number, Industry>): string {
  if (!industryId) {
    return '通用模板'
  }

  return industryMap.get(industryId)?.name || `行业 #${industryId}`
}

/**
 * 截断 Prompt 内容摘要，便于在模板列表中快速浏览。
 */
function summarizePromptContent(content: string): string {
  const compact = content.replace(/\s+/g, ' ').trim()
  if (compact.length <= 120) {
    return compact
  }

  return `${compact.slice(0, 120)}...`
}

/**
 * 提供 Prompt 模板管理页，支持筛选、创建、编辑和删除。
 */
export function PromptPage() {
  const accessToken = useAdminAuthStore((state) => state.accessToken)
  const queryClient = useQueryClient()
  const [filters, setFilters] = useState<PromptFilters>({
    scene: '',
    industryId: '',
  })
  const [selectedPromptId, setSelectedPromptId] = useState<number | null>(null)
  const [form, setForm] = useState<PromptFormState>(buildInitialPromptForm())
  const [messageText, setMessageText] = useState('读取 Prompt 模板中')

  const industriesQuery = useQuery({
    queryKey: ['admin', 'industries', accessToken],
    queryFn: () => fetchIndustries(accessToken),
    enabled: Boolean(accessToken),
  })

  const promptsQuery = useQuery({
    queryKey: ['admin', 'prompts', accessToken, filters.scene, filters.industryId],
    queryFn: () => fetchPrompts(accessToken, filters),
    enabled: Boolean(accessToken),
  })

  const industryMap = useMemo(() => {
    return new Map((industriesQuery.data || []).map((industry) => [industry.id, industry]))
  }, [industriesQuery.data])

  useEffect(() => {
    if (!promptsQuery.data) {
      return
    }

    if (selectedPromptId === null) {
      setMessageText((current) => (current === '读取 Prompt 模板中' ? '已同步 Prompt 列表。' : current))
      return
    }

    const nextPrompt = promptsQuery.data.find((item) => item.id === selectedPromptId)
    if (nextPrompt) {
      setForm(buildPromptForm(nextPrompt))
    }
  }, [promptsQuery.data, selectedPromptId])

  const saveMutation = useMutation({
    mutationFn: async () => {
      const payload = buildPromptPayload(form)

      if (selectedPromptId) {
        await updatePrompt(accessToken, selectedPromptId, payload)
        return selectedPromptId
      }

      const created = await createPrompt(accessToken, payload)
      return created?.id
    },
    onSuccess: async (promptId) => {
      setSelectedPromptId(promptId)
      setMessageText(selectedPromptId ? 'Prompt 模板已更新。' : 'Prompt 模板已创建。')
      await queryClient.invalidateQueries({
        queryKey: ['admin', 'prompts'],
      })
    },
    onError: (error) => {
      setMessageText(extractErrorMessage(error, '保存 Prompt 模板失败，请稍后重试'))
    },
  })

  const deleteMutation = useMutation({
    mutationFn: async (id: number) => {
      await deletePrompt(accessToken, id)
    },
    onSuccess: async () => {
      setSelectedPromptId(null)
      setForm(buildInitialPromptForm())
      setMessageText('Prompt 模板已删除。')
      await queryClient.invalidateQueries({
        queryKey: ['admin', 'prompts'],
      })
    },
    onError: (error) => {
      setMessageText(extractErrorMessage(error, '删除 Prompt 模板失败，请稍后重试'))
    },
  })

  /**
   * 切换到新建模式，清空当前编辑草稿。
   */
  function startCreatingPrompt(): void {
    setSelectedPromptId(null)
    setForm(buildInitialPromptForm())
    setMessageText('已切换到新建 Prompt 模式。')
  }

  /**
   * 将指定 Prompt 装载到右侧编辑区，便于继续维护历史模板。
   */
  function startEditingPrompt(prompt: PromptTemplate): void {
    setSelectedPromptId(prompt.id)
    setForm(buildPromptForm(prompt))
    setMessageText(`正在编辑模板：${prompt.name}`)
  }

  /**
   * 更新 Prompt 草稿字段，保持表单处理逻辑集中。
   */
  function updatePromptField<Key extends keyof PromptFormState>(key: Key, value: PromptFormState[Key]): void {
    setForm((current) => ({
      ...current,
      [key]: value,
    }))
  }

  /**
   * 提交 Prompt 表单并按当前模式执行创建或更新。
   */
  function handleSubmit(event: FormEvent<HTMLFormElement>): void {
    event.preventDefault()
    setMessageText(selectedPromptId ? '正在更新 Prompt 模板。' : '正在创建 Prompt 模板。')
    saveMutation.mutate()
  }

  /**
   * 删除当前选中的 Prompt 模板，并在操作前给出浏览器确认。
   */
  function handleDelete(): void {
    if (!selectedPromptId) {
      return
    }

    Modal.confirm({
      title: '确认删除',
      icon: <ExclamationCircleOutlined />,
      content: '确认删除当前 Prompt 模板吗？删除后不可恢复。',
      okText: '删除',
      okType: 'danger',
      cancelText: '取消',
      onOk: () => {
        setMessageText('正在删除 Prompt 模板。')
        deleteMutation.mutate(selectedPromptId)
      },
    })
  }

  const isLoading = promptsQuery.isLoading || industriesQuery.isLoading
  const isError = promptsQuery.isError || industriesQuery.isError

  if (isLoading) {
    return (
      <div style={{ padding: '24px 32px 32px', background: THEME.bg, minHeight: '100vh' }}>
        <div style={{ ...glassCard, padding: '24px 28px' }}>
          <h2 style={{ margin: 0, fontSize: 22, fontWeight: 700, color: THEME.textMain }}>Prompt 管理</h2>
          <p style={{ margin: '8px 0 0', color: THEME.textSecondary }}>正在加载 Prompt 模板与行业配置...</p>
        </div>
      </div>
    )
  }

  if (isError) {
    return (
      <div style={{ padding: '24px 32px 32px', background: THEME.bg, minHeight: '100vh' }}>
        <div style={{ ...glassCard, padding: '24px 28px' }}>
          <h2 style={{ margin: 0, fontSize: 22, fontWeight: 700, color: THEME.textMain }}>Prompt 管理</h2>
          <p style={{ margin: '8px 0 0', color: THEME.danger }}>
            {extractErrorMessage(promptsQuery.error || industriesQuery.error, '读取 Prompt 管理数据失败')}
          </p>
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
              background: 'linear-gradient(135deg, #3b82f6, #2563eb)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              boxShadow: '0 4px 14px rgba(37, 99, 235, 0.35)',
              flexShrink: 0,
            }}
          >
            <FileTextOutlined style={{ fontSize: 22, color: '#fff' }} />
          </div>
          <div>
            <h1 style={{ margin: 0, fontSize: 22, fontWeight: 700, color: THEME.textMain, lineHeight: 1.3 }}>
              Prompt 管理
            </h1>
            <p style={{ margin: '4px 0 0', fontSize: 13, color: THEME.textSecondary }}>
              维护面试、陪伴、刷题和计划四类 Prompt 模板，支持按场景和行业筛选
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
            <span style={{ fontSize: 24, fontWeight: 700, color: THEME.primary }}>{promptsQuery.data?.length || 0}</span>
            <span style={{ fontSize: 12, color: THEME.textSecondary }}>个模板</span>
          </div>
        </div>
      </div>

      {/* Toolbar */}
      <div
        style={{
          ...solidCard,
          padding: '14px 20px',
          marginBottom: 20,
          display: 'flex',
          alignItems: 'center',
          gap: 12,
          flexWrap: 'wrap',
        }}
      >
        <Select
          placeholder="全部场景"
          allowClear
          value={filters.scene || undefined}
          style={{ width: 140 }}
          onChange={(v) => setFilters((current) => ({ ...current, scene: v || '' }))}
          options={PROMPT_SCENE_OPTIONS}
        />

        <Select
          placeholder="全部行业"
          allowClear
          value={filters.industryId || undefined}
          style={{ width: 160 }}
          onChange={(v) => setFilters((current) => ({ ...current, industryId: v || '' }))}
          options={(industriesQuery.data || []).map((i) => ({ value: String(i.id), label: i.name }))}
        />

        <div style={{ flex: 1 }} />

        <Button
          type="primary"
          icon={<PlusOutlined />}
          onClick={startCreatingPrompt}
          style={{
            borderRadius: 10,
            background: 'linear-gradient(135deg, #3b82f6, #2563eb)',
            border: 'none',
            boxShadow: '0 4px 14px rgba(37, 99, 235, 0.35)',
          }}
        >
          新建模板
        </Button>
      </div>

      {/* Main Content */}
      <div style={{ display: 'flex', gap: 20, flexWrap: 'wrap' }}>
        {/* Left: Prompt List */}
        <div style={{ flex: '1 1 380px', maxWidth: 480, minWidth: 320 }}>
          <div
            style={{
              ...solidCard,
              overflow: 'hidden',
              display: 'flex',
              flexDirection: 'column',
              height: 'calc(100vh - 260px)',
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
              <span style={{ fontSize: 15, fontWeight: 600, color: THEME.textMain }}>模板列表</span>
              <span style={{ fontSize: 12, color: THEME.textMuted }}>
                共 {promptsQuery.data?.length || 0} 条
              </span>
            </div>

            <div style={{ flex: 1, overflowY: 'auto', padding: '8px' }}>
              {(promptsQuery.data || []).length === 0 ? (
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
                  <span style={{ fontSize: 14 }}>当前筛选下还没有 Prompt 模板</span>
                  <span style={{ fontSize: 12 }}>可以切换筛选条件，或者直接创建一个新模板</span>
                </div>
              ) : (
                <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
                  {(promptsQuery.data || []).map((prompt) => {
                    const sceneCfg = SCENE_CONFIG[prompt.scene]
                    const isActive = selectedPromptId === prompt.id
                    return (
                      <div
                        key={prompt.id}
                        role="button"
                        tabIndex={0}
                        onClick={() => startEditingPrompt(prompt)}
                        onKeyDown={(e) => {
                          if (e.key === 'Enter' || e.key === ' ') {
                            startEditingPrompt(prompt)
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
                              wordBreak: 'break-all',
                            }}
                          >
                            {prompt.name}
                          </span>
                          <Tag
                            color={prompt.is_active ? 'success' : 'default'}
                            style={{ fontSize: 11, padding: '0 6px', margin: 0, flexShrink: 0 }}
                          >
                            {prompt.is_active ? '启用中' : '已停用'}
                          </Tag>
                        </div>
                        <div
                          style={{
                            display: 'flex',
                            alignItems: 'center',
                            gap: 8,
                            marginBottom: 6,
                          }}
                        >
                          <Tag
                            style={{
                              fontSize: 11,
                              margin: 0,
                              color: sceneCfg.color,
                              background: sceneCfg.color + '14',
                              border: '1px solid ' + sceneCfg.color + '33',
                            }}
                          >
                            {sceneCfg.label}
                          </Tag>
                          <span style={{ fontSize: 12, color: THEME.textMuted }}>
                            {resolvePromptIndustryName(prompt.industry_id, industryMap)}
                          </span>
                        </div>
                        <Tooltip title={prompt.template_content} placement="bottom">
                          <p
                            style={{
                              margin: 0,
                              fontSize: 12,
                              color: THEME.textSecondary,
                              lineHeight: 1.5,
                              overflow: 'hidden',
                              textOverflow: 'ellipsis',
                              display: '-webkit-box',
                              WebkitLineClamp: 2,
                              WebkitBoxOrient: 'vertical',
                            }}
                          >
                            {summarizePromptContent(prompt.template_content)}
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
        <div style={{ flex: '2 1 480px', minWidth: 360 }}>
          <div
            style={{
              ...solidCard,
              overflow: 'hidden',
              display: 'flex',
              flexDirection: 'column',
              height: 'calc(100vh - 260px)',
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
                  {selectedPromptId ? '编辑 Prompt 模板' : '新建 Prompt 模板'}
                </span>
                <div style={{ fontSize: 12, color: THEME.textMuted, marginTop: 2 }}>{messageText}</div>
              </div>
              <Tag
                style={{
                  fontSize: 12,
                  padding: '2px 10px',
                  color: selectedPromptId ? THEME.primary : THEME.success,
                  background: selectedPromptId ? THEME.primaryLight : '#dcfce7',
                  border: 'none',
                }}
              >
                {selectedPromptId ? `ID #${selectedPromptId}` : '新模板'}
              </Tag>
            </div>

            <form
              onSubmit={handleSubmit}
              style={{ flex: 1, overflowY: 'auto', padding: '20px', display: 'flex', flexDirection: 'column', gap: 16 }}
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
                  模板名称
                </label>
                <Input
                  value={form.name}
                  onChange={(e) => updatePromptField('name', e.target.value)}
                  placeholder="例如 Go 面试官 v2"
                  style={{ borderRadius: 10 }}
                />
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
                    场景
                  </label>
                  <Select
                    value={form.scene}
                    onChange={(v) => updatePromptField('scene', v as PromptScene)}
                    style={{ width: '100%' }}
                    options={PROMPT_SCENE_OPTIONS}
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
                    行业
                  </label>
                  <Select
                    value={form.industryId || undefined}
                    allowClear
                    placeholder="通用模板"
                    onChange={(v) => updatePromptField('industryId', v || '')}
                    style={{ width: '100%' }}
                    options={(industriesQuery.data || []).map((i) => ({
                      value: String(i.id),
                      label: i.name,
                    }))}
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
                  变量说明 JSON
                </label>
                <Input.TextArea
                  value={form.variables}
                  onChange={(e) => updatePromptField('variables', e.target.value)}
                  placeholder={'例如 {"username":"用户名","progress":"当前进度"}'}
                  rows={3}
                  style={{ borderRadius: 10, fontFamily: 'monospace', fontSize: 13 }}
                />
              </div>

              <div style={{ flex: 1, display: 'flex', flexDirection: 'column', minHeight: 120 }}>
                <label
                  style={{
                    display: 'block',
                    fontSize: 13,
                    fontWeight: 500,
                    color: THEME.textSecondary,
                    marginBottom: 6,
                  }}
                >
                  模板内容
                </label>
                <Input.TextArea
                  value={form.templateContent}
                  onChange={(e) => updatePromptField('templateContent', e.target.value)}
                  placeholder="请输入完整 Prompt 模板内容"
                  style={{ flex: 1, borderRadius: 10, minHeight: 180, lineHeight: 1.6 }}
                />
              </div>

              <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
                <Switch
                  checked={form.isActive}
                  onChange={(checked) => updatePromptField('isActive', checked)}
                />
                <span style={{ fontSize: 13, color: THEME.textSecondary }}>
                  {form.isActive ? '当前模板启用中' : '当前模板已停用'}
                </span>
              </div>

              <div style={{ display: 'flex', gap: 10, flexWrap: 'wrap', paddingTop: 4 }}>
                <Button
                  icon={<ReloadOutlined />}
                  onClick={startCreatingPrompt}
                  disabled={saveMutation.isPending || deleteMutation.isPending}
                  style={{ borderRadius: 10 }}
                >
                  重置为新建
                </Button>
                {selectedPromptId && (
                  <Button
                    danger
                    icon={<DeleteOutlined />}
                    onClick={handleDelete}
                    loading={deleteMutation.isPending}
                    disabled={saveMutation.isPending || deleteMutation.isPending}
                    style={{ borderRadius: 10 }}
                  >
                    删除模板
                  </Button>
                )}
                <Button
                  type="primary"
                  htmlType="submit"
                  icon={<SaveOutlined />}
                  loading={saveMutation.isPending}
                  disabled={saveMutation.isPending || deleteMutation.isPending}
                  style={{
                    borderRadius: 10,
                    background: 'linear-gradient(135deg, #3b82f6, #2563eb)',
                    border: 'none',
                    boxShadow: '0 4px 14px rgba(37, 99, 235, 0.35)',
                  }}
                >
                  {saveMutation.isPending
                    ? '保存中...'
                    : selectedPromptId
                      ? '保存修改'
                      : '创建模板'}
                </Button>
              </div>
            </form>
          </div>
        </div>
      </div>
    </div>
  )
}
