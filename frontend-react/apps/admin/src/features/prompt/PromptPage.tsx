import type { FormEvent } from 'react'
import { useEffect, useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
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
 * 将场景枚举转换成后台页可读中文标签。
 */
function promptSceneLabel(scene: string): string {
  return PROMPT_SCENE_OPTIONS.find((item) => item.value === scene)?.label || scene
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
  const [message, setMessage] = useState('读取 Prompt 模板中')

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
      setMessage((current) => (current === '读取 Prompt 模板中' ? '已同步 Prompt 列表。' : current))
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
      setMessage(selectedPromptId ? 'Prompt 模板已更新。' : 'Prompt 模板已创建。')
      await queryClient.invalidateQueries({
        queryKey: ['admin', 'prompts'],
      })
    },
    onError: (error) => {
      setMessage(extractErrorMessage(error, '保存 Prompt 模板失败，请稍后重试'))
    },
  })

  const deleteMutation = useMutation({
    mutationFn: async (id: number) => {
      await deletePrompt(accessToken, id)
    },
    onSuccess: async () => {
      setSelectedPromptId(null)
      setForm(buildInitialPromptForm())
      setMessage('Prompt 模板已删除。')
      await queryClient.invalidateQueries({
        queryKey: ['admin', 'prompts'],
      })
    },
    onError: (error) => {
      setMessage(extractErrorMessage(error, '删除 Prompt 模板失败，请稍后重试'))
    },
  })

  /**
   * 切换到新建模式，清空当前编辑草稿。
   */
  function startCreatingPrompt(): void {
    setSelectedPromptId(null)
    setForm(buildInitialPromptForm())
    setMessage('已切换到新建 Prompt 模式。')
  }

  /**
   * 将指定 Prompt 装载到右侧编辑区，便于继续维护历史模板。
   */
  function startEditingPrompt(prompt: PromptTemplate): void {
    setSelectedPromptId(prompt.id)
    setForm(buildPromptForm(prompt))
    setMessage(`正在编辑模板：${prompt.name}`)
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
    setMessage(selectedPromptId ? '正在更新 Prompt 模板。' : '正在创建 Prompt 模板。')
    saveMutation.mutate()
  }

  /**
   * 删除当前选中的 Prompt 模板，并在操作前给出浏览器确认。
   */
  function handleDelete(): void {
    if (!selectedPromptId) {
      return
    }

    if (!window.confirm('确认删除当前 Prompt 模板吗？删除后不可恢复。')) {
      return
    }

    setMessage('正在删除 Prompt 模板。')
    deleteMutation.mutate(selectedPromptId)
  }

  if (promptsQuery.isLoading || industriesQuery.isLoading) {
    return (
      <section className="admin-panel">
        <span className="admin-tag">Prompt 中心</span>
        <h2>Prompt 管理</h2>
        <p className="admin-copy">正在加载 Prompt 模板与行业配置。</p>
      </section>
    )
  }

  if (promptsQuery.isError || industriesQuery.isError) {
    return (
      <section className="admin-panel">
        <span className="admin-tag">Prompt 中心</span>
        <h2>Prompt 管理</h2>
        <p className="admin-copy">
          {extractErrorMessage(promptsQuery.error || industriesQuery.error, '读取 Prompt 管理数据失败')}
        </p>
      </section>
    )
  }

  return (
    <section className="admin-panel admin-prompt-page">
      <div className="admin-prompt-page__hero">
        <div>
          <span className="admin-tag">Prompt 中心</span>
          <h2>Prompt 管理</h2>
          <p className="admin-copy">
            当前页用于维护面试、陪伴、刷题和计划四类 Prompt 模板。右侧支持新建或编辑，左侧支持按场景和行业筛选。
          </p>
        </div>
        <div className="admin-prompt-page__summary">
          <strong>{promptsQuery.data?.length || 0}</strong>
          <span>个模板</span>
        </div>
      </div>

      <div className="admin-prompt-page__toolbar">
        <label className="admin-field">
          <span>筛选场景</span>
          <select
            value={filters.scene}
            onChange={(event) => setFilters((current) => ({ ...current, scene: event.target.value }))}
          >
            <option value="">全部场景</option>
            {PROMPT_SCENE_OPTIONS.map((scene) => (
              <option key={scene.value} value={scene.value}>
                {scene.label}
              </option>
            ))}
          </select>
        </label>

        <label className="admin-field">
          <span>筛选行业</span>
          <select
            value={filters.industryId}
            onChange={(event) => setFilters((current) => ({ ...current, industryId: event.target.value }))}
          >
            <option value="">全部行业</option>
            {(industriesQuery.data || []).map((industry) => (
              <option key={industry.id} value={industry.id}>
                {industry.name}
              </option>
            ))}
          </select>
        </label>

        <button className="admin-link" type="button" onClick={startCreatingPrompt}>
          新建模板
        </button>
      </div>

      <div className="admin-prompt-page__layout">
        <div className="admin-prompt-list">
          {(promptsQuery.data || []).length === 0 ? (
            <div className="admin-prompt-card admin-prompt-card--empty">
              <strong>当前筛选下还没有 Prompt 模板</strong>
              <p>可以先切换筛选条件，或者直接创建一个新模板。</p>
            </div>
          ) : (
            (promptsQuery.data || []).map((prompt) => (
              <button
                key={prompt.id}
                type="button"
                className={`admin-prompt-card ${selectedPromptId === prompt.id ? 'admin-prompt-card--active' : ''}`}
                onClick={() => startEditingPrompt(prompt)}
              >
                <div className="admin-prompt-card__head">
                  <strong>{prompt.name}</strong>
                  <span>{prompt.is_active ? '启用中' : '已停用'}</span>
                </div>
                <div className="admin-prompt-card__meta">
                  <span>{promptSceneLabel(prompt.scene)}</span>
                  <span>{resolvePromptIndustryName(prompt.industry_id, industryMap)}</span>
                </div>
                <p>{summarizePromptContent(prompt.template_content)}</p>
              </button>
            ))
          )}
        </div>

        <form className="admin-prompt-editor" onSubmit={handleSubmit}>
          <div className="admin-prompt-editor__head">
            <div>
              <h3>{selectedPromptId ? '编辑 Prompt 模板' : '新建 Prompt 模板'}</h3>
              <p>{message}</p>
            </div>
            {selectedPromptId ? (
              <span className="admin-tag">ID #{selectedPromptId}</span>
            ) : (
              <span className="admin-tag">新模板</span>
            )}
          </div>

          <label className="admin-field">
            <span>模板名称</span>
            <input
              value={form.name}
              onChange={(event) => updatePromptField('name', event.target.value)}
              placeholder="例如 Go 面试官 v2"
            />
          </label>

          <div className="admin-prompt-editor__grid">
            <label className="admin-field">
              <span>场景</span>
              <select
                value={form.scene}
                onChange={(event) => updatePromptField('scene', event.target.value as PromptScene)}
              >
                {PROMPT_SCENE_OPTIONS.map((scene) => (
                  <option key={scene.value} value={scene.value}>
                    {scene.label}
                  </option>
                ))}
              </select>
            </label>

            <label className="admin-field">
              <span>行业</span>
              <select
                value={form.industryId}
                onChange={(event) => updatePromptField('industryId', event.target.value)}
              >
                <option value="">通用模板</option>
                {(industriesQuery.data || []).map((industry) => (
                  <option key={industry.id} value={industry.id}>
                    {industry.name}
                  </option>
                ))}
              </select>
            </label>
          </div>

          <label className="admin-field">
            <span>变量说明 JSON</span>
            <textarea
              value={form.variables}
              onChange={(event) => updatePromptField('variables', event.target.value)}
              placeholder='例如 {"username":"用户名","progress":"当前进度"}'
            />
          </label>

          <label className="admin-field">
            <span>模板内容</span>
            <textarea
              className="admin-prompt-editor__content"
              value={form.templateContent}
              onChange={(event) => updatePromptField('templateContent', event.target.value)}
              placeholder="请输入完整 Prompt 模板内容"
            />
          </label>

          <label className="admin-prompt-editor__switch">
            <input
              type="checkbox"
              checked={form.isActive}
              onChange={(event) => updatePromptField('isActive', event.target.checked)}
            />
            <span>{form.isActive ? '当前模板启用中' : '当前模板已停用'}</span>
          </label>

          <div className="admin-prompt-editor__actions">
            <button
              className="admin-link"
              type="button"
              onClick={startCreatingPrompt}
              disabled={saveMutation.isPending || deleteMutation.isPending}
            >
              重置为新建
            </button>
            {selectedPromptId ? (
              <button
                className="admin-link"
                type="button"
                onClick={handleDelete}
                disabled={saveMutation.isPending || deleteMutation.isPending}
              >
                {deleteMutation.isPending ? '删除中...' : '删除模板'}
              </button>
            ) : null}
            <button className="admin-link" type="submit" disabled={saveMutation.isPending || deleteMutation.isPending}>
              {saveMutation.isPending ? '保存中...' : selectedPromptId ? '保存修改' : '创建模板'}
            </button>
          </div>
        </form>
      </div>
    </section>
  )
}
