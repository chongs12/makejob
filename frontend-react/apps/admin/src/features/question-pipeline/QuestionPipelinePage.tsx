import type { FormEvent } from 'react'
import { useEffect, useMemo, useRef, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { buildAuthorizationHeader, extractErrorMessage, getApiBaseUrl, requestJson } from '@makejob/api-client'
import { isSuccessCode, type ApiEnvelope } from '@makejob/shared-types'
import { flushSync } from 'react-dom'
import { useAdminAuthStore } from '../../state/auth'
import { fetchScraperTaskDetail } from '../runtime/runtimeApi'
import type { ScraperTaskDetail } from '../runtime/runtimeTypes'

type QuestionType = 'choice' | 'multi' | 'code' | 'subjective'
type QuestionDifficulty = 'easy' | 'medium' | 'hard'
type QuestionPipelineGenerationMode = 'planned' | 'direct_single'

interface Industry {
  id: number
  code: string
  name: string
  is_active: boolean
}

interface Category {
  id: number
  industry_id: number
  name: string
  sort_order: number
}

interface ScraperSource {
  name: string
  label: string
  base_url: string
  is_active: boolean
}

interface QuestionPipelineCard {
  id: string
  title: string
  content: string
  type: QuestionType
  difficulty: QuestionDifficulty
  category: string
  answer: string
  explanation: string
  tags: string[]
  confidence: number
  source_type: string
  source_label: string
  source_title: string
  source_url: string
}

interface QuestionPipelineStats {
  searched_count: number
  fetched_count: number
  scraped_count: number
  generated_count: number
  candidate_count: number
  selected_sources: number
}

interface QuestionPipelineGenerateResponse {
  industry_code: string
  requirement: string
  generation_mode: QuestionPipelineGenerationMode
  cards: QuestionPipelineCard[]
  warnings?: string[]
  stats: QuestionPipelineStats
}

interface RawQuestionPipelineCard {
  id?: unknown
  title?: unknown
  content?: unknown
  type?: unknown
  difficulty?: unknown
  category?: unknown
  answer?: unknown
  explanation?: unknown
  tags?: unknown
  confidence?: unknown
  source_type?: unknown
  source_label?: unknown
  source_title?: unknown
  source_url?: unknown
}

interface RawQuestionPipelineGenerateResponse {
  industry_code?: unknown
  requirement?: unknown
  generation_mode?: unknown
  cards?: unknown
  warnings?: unknown
  stats?: unknown
}

interface RawQuestionPipelineStreamEvent {
  event?: unknown
  message?: unknown
  trace_id?: unknown
  raw_output?: unknown
  slot_index?: unknown
  retry_index?: unknown
  card?: unknown
  response?: unknown
}

interface BatchImportResponse {
  total_count: number
  success_count: number
  fail_count: number
  errors?: string[]
}

interface AdminAsyncTaskResponse {
  id: number
  status: string
  task_type?: string
}

interface PipelineFormState {
  industryCode: string
  requirement: string
  agentPrompt: string
  generationMode: QuestionPipelineGenerationMode
  candidateCount: string
  includeScraped: boolean
  includeGenerated: boolean
  sources: string[]
}

interface EditablePipelineCard extends QuestionPipelineCard {
  selected: boolean
  tagsText: string
}

interface StreamErrorPayload {
  message?: unknown
}

interface PipelineDebugEntry {
  id: string
  message: string
  traceId: string
  rawOutput: string
  slotIndex: number
  retryIndex: number
}

const QUESTION_TYPE_OPTIONS: Array<{ value: QuestionType; label: string }> = [
  { value: 'subjective', label: '主观题' },
  { value: 'code', label: '编程题' },
  { value: 'choice', label: '单选题' },
  { value: 'multi', label: '多选题' },
]

const QUESTION_DIFFICULTY_OPTIONS: Array<{ value: QuestionDifficulty; label: string }> = [
  { value: 'easy', label: '简单' },
  { value: 'medium', label: '中等' },
  { value: 'hard', label: '困难' },
]

/**
 * 获取后台行业列表，供流水线指定题库目标行业。
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
 * 获取后台分类列表，用于题卡落入现有题库分类。
 */
async function fetchCategories(token: string | null): Promise<Category[]> {
  const response = await requestJson<ApiEnvelope<Category[]>>('/admin/categories', {
    method: 'GET',
    token,
  })

  if (!isSuccessCode(response.code)) {
    throw new Error(response.message || '获取分类列表失败')
  }

  return response.data
}

/**
 * 获取后台可用抓取来源列表，用于选择面经抓取渠道。
 */
async function fetchScraperSources(token: string | null): Promise<ScraperSource[]> {
  const response = await requestJson<ApiEnvelope<ScraperSource[]>>('/admin/scraper/sources', {
    method: 'GET',
    token,
  })

  if (!isSuccessCode(response.code)) {
    throw new Error(response.message || '获取抓取来源失败')
  }

  return response.data
}

/**
 * 调用后台题目流水线生成接口，产出待确认题卡。
 */
async function generateQuestionPipeline(
  token: string | null,
  payload: Record<string, unknown>,
): Promise<QuestionPipelineGenerateResponse> {
  const response = await requestJson<ApiEnvelope<RawQuestionPipelineGenerateResponse | null>>('/admin/question-pipeline/generate', {
    method: 'POST',
    token,
    body: payload,
  })

  if (!isSuccessCode(response.code)) {
    throw new Error(response.message || '生成题目流水线失败')
  }

  return normalizeQuestionPipelineGenerateResponse(response.data)
}

/**
 * 将当前表单转换为后台题目流水线生成请求载荷，供同步生成与异步入队复用。
 */
function buildQuestionPipelineGeneratePayload(form: PipelineFormState): Record<string, unknown> {
  return {
    industry_code: form.industryCode,
    requirement: form.requirement.trim(),
    agent_prompt: form.agentPrompt.trim(),
    generation_mode: form.generationMode,
    candidate_count: Number(form.candidateCount) || 8,
    include_scraped: form.includeScraped,
    include_generated: form.includeGenerated,
    sources: form.includeScraped ? form.sources : [],
  }
}

/**
 * 以 SSE 方式读取题目流水线流式事件，让逐张直生模式可以边生成边落屏。
 */
async function streamQuestionPipeline(
  token: string | null,
  payload: Record<string, unknown>,
  signal: AbortSignal,
  callbacks: {
    onStatus: (message: string) => void
    onWarning: (warning: PipelineDebugEntry | null, message: string) => void
    onError: (message: string) => void
    onCard: (card: QuestionPipelineCard) => void
    onComplete: (response: QuestionPipelineGenerateResponse) => void
  },
): Promise<void> {
  const baseUrl = getApiBaseUrl().replace(/\/+$/, '')
  const response = await fetch(`${baseUrl}/admin/question-pipeline/generate/stream`, {
    method: 'POST',
    headers: {
      ...buildAuthorizationHeader(token),
      Accept: 'text/event-stream',
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(payload),
    signal,
  })

  if (!response.ok) {
    throw new Error(await buildQuestionPipelineStreamErrorMessage(response))
  }
  if (!response.body) {
    throw new Error('流式生成接口未返回可读取的数据流。')
  }

  const reader = response.body.getReader()
  const decoder = new TextDecoder('utf-8')
  let buffer = ''

  const processEventBlock = (block: string): void => {
    const normalizedBlock = block.trim()
    if (!normalizedBlock) {
      return
    }

    let eventName = ''
    const dataLines: string[] = []
    for (const line of normalizedBlock.split(/\r?\n/)) {
      if (line.startsWith('event:')) {
        eventName = line.slice('event:'.length).trim()
      } else if (line.startsWith('data:')) {
        dataLines.push(line.slice('data:'.length).trim())
      }
    }

    if (dataLines.length === 0) {
      return
    }

    const rawPayload = JSON.parse(dataLines.join('\n')) as RawQuestionPipelineStreamEvent
    const effectiveEvent = typeof rawPayload.event === 'string' && rawPayload.event.trim()
      ? rawPayload.event.trim()
      : eventName
    const message = typeof rawPayload.message === 'string' ? rawPayload.message.trim() : ''
    const traceId = typeof rawPayload.trace_id === 'string' ? rawPayload.trace_id.trim() : ''
    const rawOutput = typeof rawPayload.raw_output === 'string' ? rawPayload.raw_output.trim() : ''
    const slotIndex = typeof rawPayload.slot_index === 'number' ? rawPayload.slot_index : 0
    const retryIndex = typeof rawPayload.retry_index === 'number' ? rawPayload.retry_index : 0

    switch (effectiveEvent) {
      case 'status':
        if (message) {
          callbacks.onStatus(message)
        }
        return
      case 'warning':
        if (message) {
          callbacks.onWarning(
            rawOutput || traceId
              ? {
                  id: `${traceId || 'warning'}-${slotIndex || 0}-${retryIndex || 0}-${message}`,
                  message,
                  traceId,
                  rawOutput,
                  slotIndex,
                  retryIndex,
                }
              : null,
            message,
          )
        }
        return
      case 'error':
        if (message) {
          callbacks.onError(message)
        }
        return
      case 'card':
        if (rawPayload.card && typeof rawPayload.card === 'object') {
          callbacks.onCard(normalizeQuestionPipelineCard(rawPayload.card as RawQuestionPipelineCard, 0))
        }
        return
      case 'complete':
        callbacks.onComplete(
          normalizeQuestionPipelineGenerateResponse(
            (rawPayload.response || null) as RawQuestionPipelineGenerateResponse | null,
          ),
        )
        return
      default:
        if (message) {
          callbacks.onStatus(message)
        }
    }
  }

  while (true) {
    const { value, done } = await reader.read()
    buffer += decoder.decode(value || new Uint8Array(), { stream: !done })

    let separatorMatch = buffer.match(/\r?\n\r?\n/)
    while (separatorMatch && typeof separatorMatch.index === 'number') {
      const block = buffer.slice(0, separatorMatch.index)
      buffer = buffer.slice(separatorMatch.index + separatorMatch[0].length)
      processEventBlock(block)
      separatorMatch = buffer.match(/\r?\n\r?\n/)
    }

    if (done) {
      const remaining = decoder.decode()
      if (remaining) {
        buffer += remaining
      }
      if (buffer.trim()) {
        processEventBlock(buffer)
      }
      break
    }
  }
}

/**
 * 解析流式生成接口的失败响应，优先提取后端返回的明确错误消息。
 */
async function buildQuestionPipelineStreamErrorMessage(response: Response): Promise<string> {
  const fallback = `流式生成失败，状态码：${response.status}`
  const rawText = (await response.text()).trim()
  if (!rawText) {
    return fallback
  }

  try {
    const payload = JSON.parse(rawText) as StreamErrorPayload
    if (typeof payload.message === 'string' && payload.message.trim()) {
      return payload.message.trim()
    }
  } catch {
    // 保持回退文案即可，这里无需额外处理。
  }

  return rawText
}

/**
 * 创建异步题目流水线生成任务，交给后台 worker 延后执行。
 */
async function queueQuestionPipelineGenerateTask(
  token: string | null,
  payload: Record<string, unknown>,
): Promise<AdminAsyncTaskResponse> {
  const response = await requestJson<ApiEnvelope<AdminAsyncTaskResponse>>('/admin/question-pipeline/generate/async', {
    method: 'POST',
    token,
    body: payload,
  })

  if (!isSuccessCode(response.code) || !response.data) {
    throw new Error(response.message || '创建异步题目流水线任务失败')
  }

  return response.data
}

/**
 * 导入当前勾选后的候选题卡到正式题库。
 */
async function importQuestionPipeline(
  token: string | null,
  payload: Record<string, unknown>,
): Promise<BatchImportResponse> {
  const response = await requestJson<ApiEnvelope<BatchImportResponse>>('/admin/question-pipeline/import', {
    method: 'POST',
    token,
    body: payload,
  })

  if (!isSuccessCode(response.code)) {
    throw new Error(response.message || '导入题目流水线失败')
  }

  return response.data
}

/**
 * 将当前勾选题卡入队为异步导入任务，交给后台 worker 后续消费。
 */
async function queueQuestionPipelineImportTask(
  token: string | null,
  payload: Record<string, unknown>,
): Promise<AdminAsyncTaskResponse> {
  const response = await requestJson<ApiEnvelope<AdminAsyncTaskResponse>>('/admin/scraper/import/async', {
    method: 'POST',
    token,
    body: payload,
  })

  if (!isSuccessCode(response.code) || !response.data) {
    throw new Error(response.message || '创建异步导入任务失败')
  }

  return response.data
}

/**
 * 构造流水线表单初始值，避免页面初次渲染时出现空引用。
 */
function buildInitialPipelineForm(): PipelineFormState {
  return {
    industryCode: '',
    requirement: '',
    agentPrompt: '确保每张题卡考察不同考点，优先生成真正区分度高的问答题，避免模板化和重复表述。',
    generationMode: 'planned',
    candidateCount: '8',
    includeScraped: true,
    includeGenerated: true,
    sources: [],
  }
}

/**
 * 将后端题卡转换为前端可编辑状态。
 */
function buildEditableCards(cards: QuestionPipelineCard[]): EditablePipelineCard[] {
  return cards.map((card) => ({
    ...card,
    selected: true,
    tagsText: (card.tags || []).join(', '),
  }))
}

/**
 * 将勾选题卡整理成统一导入载荷，供同步导入与异步入队共用。
 */
function buildSelectedImportPayload(cards: EditablePipelineCard[]) {
  return cards
    .filter((item) => item.selected)
    .map((item) => ({
      title: item.title.trim(),
      content: item.content.trim(),
      type: item.type,
      difficulty: item.difficulty,
      category: item.category,
      answer: item.answer.trim(),
      explanation: item.explanation.trim(),
      tags: parseTagsInput(item.tagsText),
    }))
}

/**
 * 根据异步任务载荷恢复流水线表单，便于后台任务完成后回到当前页面继续筛题和导入。
 */
function restoreQuestionPipelineFormFromTaskPayload(
  current: PipelineFormState,
  payloadJSON?: string,
): PipelineFormState {
  if (!payloadJSON?.trim()) {
    return current
  }

  try {
    const payload = JSON.parse(payloadJSON) as Record<string, unknown>
    const candidateCount = typeof payload.candidate_count === 'number'
      ? String(payload.candidate_count)
      : current.candidateCount
    return {
      ...current,
      industryCode: typeof payload.industry_code === 'string' ? payload.industry_code.trim() : current.industryCode,
      requirement: typeof payload.requirement === 'string' ? payload.requirement.trim() : current.requirement,
      agentPrompt: typeof payload.agent_prompt === 'string' ? payload.agent_prompt.trim() : current.agentPrompt,
      generationMode: normalizeQuestionPipelineGenerationMode(payload.generation_mode),
      candidateCount,
      includeScraped: typeof payload.include_scraped === 'boolean' ? payload.include_scraped : current.includeScraped,
      includeGenerated: typeof payload.include_generated === 'boolean' ? payload.include_generated : current.includeGenerated,
      sources: Array.isArray(payload.sources)
        ? payload.sources.filter((item): item is string => typeof item === 'string').map((item) => item.trim()).filter(Boolean)
        : current.sources,
    }
  } catch {
    return current
  }
}

/**
 * 将异步任务详情中的结果 JSON 还原为前端统一的题目流水线结果结构。
 */
function restoreQuestionPipelineResponseFromTask(task: ScraperTaskDetail): QuestionPipelineGenerateResponse {
  if (!task.result_json?.trim()) {
    throw new Error('当前任务还没有可恢复的候选题卡结果')
  }

  try {
    return normalizeQuestionPipelineGenerateResponse(JSON.parse(task.result_json) as RawQuestionPipelineGenerateResponse)
  } catch {
    throw new Error('题目流水线任务结果解析失败')
  }
}

/**
 * 将流式到达的新题卡合并进当前列表，避免完成事件重复插入相同卡片。
 */
function mergeEditableCards(current: EditablePipelineCard[], incoming: EditablePipelineCard[]): EditablePipelineCard[] {
  const merged = [...current]
  for (const card of incoming) {
    const index = merged.findIndex((item) => item.id === card.id)
    if (index >= 0) {
      merged[index] = {
        ...merged[index],
        ...card,
      }
    } else {
      merged.push(card)
    }
  }
  return merged
}

/**
 * 合并流式调试记录，便于页面展示每次失败调用的 trace_id 与原始输出。
 */
function mergePipelineDebugEntries(current: PipelineDebugEntry[], incoming: PipelineDebugEntry): PipelineDebugEntry[] {
  const index = current.findIndex((item) => item.id === incoming.id)
  if (index >= 0) {
    const next = [...current]
    next[index] = incoming
    return next
  }
  return [...current, incoming]
}

/**
 * 按后端最终返回顺序重建题卡列表，同时保留用户已修改过的勾选与编辑内容。
 */
function reconcileEditableCards(current: EditablePipelineCard[], incoming: EditablePipelineCard[]): EditablePipelineCard[] {
  return incoming.map((card) => {
    const existing = current.find((item) => item.id === card.id)
    if (!existing) {
      return card
    }

    return {
      ...card,
      selected: existing.selected,
      title: existing.title,
      content: existing.content,
      type: existing.type,
      difficulty: existing.difficulty,
      category: existing.category,
      answer: existing.answer,
      explanation: existing.explanation,
      tagsText: existing.tagsText,
    }
  })
}

/**
 * 将未知值安全转换为字符串数组，避免后端返回 null 时前端直接崩溃。
 */
function normalizeStringList(value: unknown): string[] {
  if (!Array.isArray(value)) {
    return []
  }

  return value
    .filter((item): item is string => typeof item === 'string')
    .map((item) => item.trim())
    .filter(Boolean)
}

/**
 * 规范化题型枚举，保证页面下拉框始终拿到受控值。
 */
function normalizeQuestionType(value: unknown): QuestionType {
  if (value === 'choice' || value === 'multi' || value === 'code' || value === 'subjective') {
    return value
  }
  return 'subjective'
}

/**
 * 规范化难度枚举，避免接口脏数据导致页面渲染状态异常。
 */
function normalizeQuestionDifficulty(value: unknown): QuestionDifficulty {
  if (value === 'easy' || value === 'medium' || value === 'hard') {
    return value
  }
  return 'medium'
}

/**
 * 规范化题目流水线生成模式，避免接口返回未知值导致页面状态不一致。
 */
function normalizeQuestionPipelineGenerationMode(value: unknown): QuestionPipelineGenerationMode {
  return value === 'direct_single' ? 'direct_single' : 'planned'
}

/**
 * 将单张原始题卡转换为前端稳定可渲染的数据结构。
 */
function normalizeQuestionPipelineCard(card: RawQuestionPipelineCard, index: number): QuestionPipelineCard {
  const title = typeof card.title === 'string' ? card.title.trim() : ''
  const content = typeof card.content === 'string' ? card.content.trim() : ''
  const answer = typeof card.answer === 'string' ? card.answer.trim() : ''

  return {
    id: typeof card.id === 'string' && card.id.trim() ? card.id.trim() : `pipeline-card-${index + 1}`,
    title,
    content,
    type: normalizeQuestionType(card.type),
    difficulty: normalizeQuestionDifficulty(card.difficulty),
    category: typeof card.category === 'string' ? card.category.trim() : '',
    answer,
    explanation: typeof card.explanation === 'string' ? card.explanation.trim() : '',
    tags: normalizeStringList(card.tags),
    confidence: typeof card.confidence === 'number' ? card.confidence : 0,
    source_type: typeof card.source_type === 'string' ? card.source_type.trim() : 'generated',
    source_label: typeof card.source_label === 'string' ? card.source_label.trim() : 'AI 智能体生成',
    source_title: typeof card.source_title === 'string' ? card.source_title.trim() : '',
    source_url: typeof card.source_url === 'string' ? card.source_url.trim() : '',
  }
}

/**
 * 规范化生成接口响应，确保页面只消费结构稳定的候选题卡数据。
 */
function normalizeQuestionPipelineGenerateResponse(
  payload: RawQuestionPipelineGenerateResponse | null | undefined,
): QuestionPipelineGenerateResponse {
  if (!payload || typeof payload !== 'object') {
    throw new Error('生成接口已返回成功，但未携带候选题卡数据。')
  }

  const cards = Array.isArray(payload.cards)
    ? payload.cards
        .map((item, index) => normalizeQuestionPipelineCard((item || {}) as RawQuestionPipelineCard, index))
        .filter((item) => item.title && item.content && item.answer)
    : []

  if (cards.length === 0) {
    throw new Error('生成接口已返回成功，但没有可展示的候选题卡。')
  }

  const rawStats = payload.stats && typeof payload.stats === 'object' ? (payload.stats as Record<string, unknown>) : {}

  return {
    industry_code: typeof payload.industry_code === 'string' ? payload.industry_code.trim() : '',
    requirement: typeof payload.requirement === 'string' ? payload.requirement.trim() : '',
    generation_mode: normalizeQuestionPipelineGenerationMode(payload.generation_mode),
    cards,
    warnings: normalizeStringList(payload.warnings),
    stats: {
      searched_count: typeof rawStats.searched_count === 'number' ? rawStats.searched_count : 0,
      fetched_count: typeof rawStats.fetched_count === 'number' ? rawStats.fetched_count : 0,
      scraped_count: typeof rawStats.scraped_count === 'number' ? rawStats.scraped_count : 0,
      generated_count: typeof rawStats.generated_count === 'number' ? rawStats.generated_count : 0,
      candidate_count: typeof rawStats.candidate_count === 'number' ? rawStats.candidate_count : cards.length,
      selected_sources: typeof rawStats.selected_sources === 'number' ? rawStats.selected_sources : 0,
    },
  }
}

/**
 * 为题目流水线生成模式返回简洁中文文案，方便页面提示当前链路。
 */
function questionPipelineGenerationModeLabel(mode: QuestionPipelineGenerationMode): string {
  return mode === 'direct_single' ? '逐张直生' : '两阶段规划'
}

/**
 * 将标签输入框转换为标签数组，统一去空与去重。
 */
function parseTagsInput(input: string): string[] {
  const values = input.split(/,|，/).map((item) => item.trim()).filter(Boolean)
  return Array.from(new Set(values))
}

/**
 * 为题卡来源类型生成简短中文文案。
 */
function sourceTypeLabel(sourceType: string): string {
  return sourceType === 'generated' ? 'AI 改写' : '抓取清洗'
}

/**
 * 为当前行业过滤可选分类，避免导入时再次打到跨行业外键错误。
 */
function filterCategoriesByIndustry(
  categories: Category[],
  industries: Industry[],
  industryCode: string,
): Category[] {
  const industry = industries.find((item) => item.code === industryCode)
  if (!industry) {
    return []
  }

  return categories
    .filter((item) => item.industry_id === industry.id)
    .sort((left, right) => left.sort_order - right.sort_order || left.id - right.id)
}

/**
 * 生成页面中的表单校验消息，控制按钮可用性。
 */
function buildPipelineFormError(form: PipelineFormState): string {
  if (!form.industryCode) {
    return '请选择题库目标行业。'
  }
  if (!form.requirement.trim()) {
    return '请填写岗位要求、题目方向或清洗要求。'
  }
  if (!form.includeGenerated && !form.includeScraped) {
    return '抓取与 AI 生成至少要启用一种。'
  }
  return ''
}

export function QuestionPipelinePage() {
  const token = useAdminAuthStore((state) => state.accessToken)
  const queryClient = useQueryClient()
  const streamAbortRef = useRef<AbortController | null>(null)
  const freshCardTimerRef = useRef<Record<string, number>>({})
  const [form, setForm] = useState<PipelineFormState>(() => buildInitialPipelineForm())
  const [cards, setCards] = useState<EditablePipelineCard[]>([])
  const [freshCardIds, setFreshCardIds] = useState<string[]>([])
  const [debugEntries, setDebugEntries] = useState<PipelineDebugEntry[]>([])
  const [warnings, setWarnings] = useState<string[]>([])
  const [stats, setStats] = useState<QuestionPipelineStats | null>(null)
  const [message, setMessage] = useState('填写岗位要求后即可生成候选题卡。')
  const [isStreaming, setIsStreaming] = useState(false)
  const [asyncGenerateTaskId, setAsyncGenerateTaskId] = useState('')

  const industriesQuery = useQuery({
    queryKey: ['admin-industries'],
    queryFn: () => fetchIndustries(token),
  })

  const categoriesQuery = useQuery({
    queryKey: ['admin-categories'],
    queryFn: () => fetchCategories(token),
  })

  const sourcesQuery = useQuery({
    queryKey: ['admin-scraper-sources'],
    queryFn: () => fetchScraperSources(token),
  })

  useEffect(() => {
    if (!sourcesQuery.data || form.sources.length > 0) {
      return
    }

    const activeSources = sourcesQuery.data.filter((item) => item.is_active).map((item) => item.name)
    setForm((current) => ({
      ...current,
      sources: activeSources,
    }))
  }, [form.sources.length, sourcesQuery.data])

  useEffect(() => {
    if (!industriesQuery.data || form.industryCode) {
      return
    }

    const firstActiveIndustry = industriesQuery.data.find((item) => item.is_active)
    if (!firstActiveIndustry) {
      return
    }

    setForm((current) => ({
      ...current,
      industryCode: firstActiveIndustry.code,
    }))
  }, [form.industryCode, industriesQuery.data])

  useEffect(() => () => {
    streamAbortRef.current?.abort()
    Object.values(freshCardTimerRef.current).forEach((timer) => window.clearTimeout(timer))
  }, [])

  const categoryOptions = useMemo(
    () => filterCategoriesByIndustry(categoriesQuery.data || [], industriesQuery.data || [], form.industryCode),
    [categoriesQuery.data, industriesQuery.data, form.industryCode],
  )

  const selectedCount = useMemo(() => cards.filter((item) => item.selected).length, [cards])
  const formError = useMemo(() => buildPipelineFormError(form), [form])

  const generateMutation = useMutation({
    mutationFn: async () => generateQuestionPipeline(token, buildQuestionPipelineGeneratePayload(form)),
    onSuccess: (result) => {
      setCards(buildEditableCards(result.cards))
      setFreshCardIds([])
      setDebugEntries([])
      setWarnings(result.warnings || [])
      setStats(result.stats)
      setMessage(`已通过${questionPipelineGenerationModeLabel(result.generation_mode)}生成 ${result.cards.length} 张候选题卡，请确认后再导入题库。`)
    },
    onError: (error) => {
      setMessage(extractErrorMessage(error, '生成题目流水线失败'))
    },
  })

  const queueGenerateTaskMutation = useMutation({
    mutationFn: async () => queueQuestionPipelineGenerateTask(token, buildQuestionPipelineGeneratePayload(form)),
    onSuccess: async (task) => {
      setAsyncGenerateTaskId(String(task.id))
      setMessage(`已创建异步题目流水线任务 #${task.id}，可稍后按任务 ID 恢复结果或前往运行任务页查看状态。`)
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['admin-runtime-scraper-tasks'] }),
        queryClient.invalidateQueries({ queryKey: ['admin-runtime-scraper-tasks-overview'] }),
      ])
    },
    onError: (error) => {
      setMessage(extractErrorMessage(error, '创建异步题目流水线任务失败'))
    },
  })

  const importMutation = useMutation({
    mutationFn: async () =>
      importQuestionPipeline(token, {
        industry_code: form.industryCode,
        cards: buildSelectedImportPayload(cards),
      }),
    onSuccess: (result) => {
      void queryClient.invalidateQueries({ queryKey: ['admin-questions'] })
      setMessage(`已导入 ${result.success_count} 道题，失败 ${result.fail_count} 道。`)
      if (result.success_count > 0) {
        setCards((current) => current.filter((item) => !item.selected))
      }
      setWarnings(result.errors || [])
    },
    onError: (error) => {
      setMessage(extractErrorMessage(error, '导入题目流水线失败'))
    },
  })

  const queueImportTaskMutation = useMutation({
    mutationFn: async () =>
      queueQuestionPipelineImportTask(token, {
        industry_code: form.industryCode,
        source_title: form.requirement.trim().slice(0, 120) || '题目流水线候选题卡',
        questions: buildSelectedImportPayload(cards),
      }),
    onSuccess: async (task) => {
      setMessage(`已创建异步导入任务 #${task.id}，可前往运行任务页查看执行状态。`)
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['admin-runtime-scraper-tasks'] }),
        queryClient.invalidateQueries({ queryKey: ['admin-runtime-scraper-tasks-overview'] }),
      ])
    },
    onError: (error) => {
      setMessage(extractErrorMessage(error, '创建异步导入任务失败'))
    },
  })

  const loadGenerateTaskResultMutation = useMutation({
    mutationFn: async () => {
      const taskID = Number(asyncGenerateTaskId)
      if (!Number.isFinite(taskID) || taskID <= 0) {
        throw new Error('请输入有效的异步任务 ID')
      }
      return fetchScraperTaskDetail(token, taskID)
    },
    onSuccess: (task) => {
      if (task.task_type !== 'question_pipeline_build') {
        setMessage(`任务 #${task.id} 不是题目流水线生成任务，请确认任务 ID。`)
        return
      }

      setForm((current) => restoreQuestionPipelineFormFromTaskPayload(current, task.payload_json))
      if (task.status === 'pending' || task.status === 'running') {
        setMessage(`任务 #${task.id} 当前状态为 ${task.status}，请稍后重新读取结果。`)
        return
      }
      if (task.status === 'failed') {
        setWarnings(task.error_msg ? [task.error_msg] : [])
        setMessage(`任务 #${task.id} 执行失败，可前往运行任务页查看详情后重试。`)
        return
      }

      const result = restoreQuestionPipelineResponseFromTask(task)
      setCards(buildEditableCards(result.cards))
      setFreshCardIds([])
      setDebugEntries([])
      setWarnings(result.warnings || [])
      setStats(result.stats)
      setMessage(`已从异步任务 #${task.id} 恢复 ${result.cards.length} 张候选题卡，请确认后再导入题库。`)
    },
    onError: (error) => {
      setMessage(extractErrorMessage(error, '读取异步题目流水线结果失败'))
    },
  })

  /**
   * 为刚到达的流式题卡添加短暂高亮，强化逐张落屏的视觉反馈。
   */
  function markFreshCard(cardId: string): void {
    if (!cardId) {
      return
    }

    const currentTimer = freshCardTimerRef.current[cardId]
    if (currentTimer) {
      window.clearTimeout(currentTimer)
    }

    setFreshCardIds((current) => (current.includes(cardId) ? current : [...current, cardId]))
    freshCardTimerRef.current[cardId] = window.setTimeout(() => {
      setFreshCardIds((current) => current.filter((item) => item !== cardId))
      delete freshCardTimerRef.current[cardId]
    }, 1600)
  }

  /**
   * 取消当前流式生成请求，避免继续占用模型调用与页面状态。
   */
  function handleCancelStream(): void {
    streamAbortRef.current?.abort()
    setIsStreaming(false)
    setMessage('已取消本次流式生成。')
  }

  /**
   * 提交流水线生成表单，请求后台返回候选题卡。
   */
  function handleGenerate(event: FormEvent<HTMLFormElement>): void {
    event.preventDefault()
    if (form.generationMode === 'direct_single' && form.includeGenerated) {
      const controller = new AbortController()
      streamAbortRef.current?.abort()
      streamAbortRef.current = controller
      Object.values(freshCardTimerRef.current).forEach((timer) => window.clearTimeout(timer))
      freshCardTimerRef.current = {}
      setCards([])
      setFreshCardIds([])
      setDebugEntries([])
      setWarnings([])
      setStats({
        searched_count: 0,
        fetched_count: 0,
        scraped_count: 0,
        generated_count: 0,
        candidate_count: 0,
        selected_sources: 0,
      })
      setMessage('正在建立流式生成连接。')
      setIsStreaming(true)

      void streamQuestionPipeline(
        token,
        buildQuestionPipelineGeneratePayload(form),
        controller.signal,
        {
          onStatus: (nextMessage) => {
            setMessage(nextMessage)
          },
          onWarning: (debugEntry, warning) => {
            setWarnings((current) => (current.includes(warning) ? current : [...current, warning]))
            if (debugEntry) {
              setDebugEntries((current) => mergePipelineDebugEntries(current, debugEntry))
            }
          },
          onError: (errorMessage) => {
            setMessage(errorMessage)
          },
          onCard: (card) => {
            flushSync(() => {
              setCards((current) => mergeEditableCards(current, buildEditableCards([card])))
              setStats((current) => ({
                searched_count: current?.searched_count || 0,
                fetched_count: current?.fetched_count || 0,
                scraped_count: current?.scraped_count || 0,
                generated_count: (current?.generated_count || 0) + 1,
                candidate_count: current?.candidate_count || 0,
                selected_sources: current?.selected_sources || 0,
              }))
            })
            markFreshCard(card.id)
          },
          onComplete: (result) => {
            setCards((current) => reconcileEditableCards(current, buildEditableCards(result.cards)))
            setWarnings((current) => Array.from(new Set([...current, ...(result.warnings || [])])))
            setStats(result.stats)
            setMessage(`已通过${questionPipelineGenerationModeLabel(result.generation_mode)}生成 ${result.cards.length} 张候选题卡，请确认后再导入题库。`)
          },
        },
      )
        .catch((error) => {
          if (controller.signal.aborted) {
            return
          }
          setMessage(extractErrorMessage(error, '流式生成题目流水线失败'))
        })
        .finally(() => {
          if (streamAbortRef.current === controller) {
            streamAbortRef.current = null
          }
          setIsStreaming(false)
        })
      return
    }

    setMessage('正在生成候选题卡。')
    setDebugEntries([])
    setWarnings([])
    generateMutation.mutate()
  }

  /**
   * 将当前流水线请求入队为异步生成任务，适合耗时较长或希望脱离页面等待的场景。
   */
  function handleQueueGenerateTask(): void {
    setMessage('正在创建异步题目流水线任务。')
    queueGenerateTaskMutation.mutate()
  }

  /**
   * 按任务 ID 重新读取异步生成结果，便于任务完成后恢复候选题卡继续人工确认。
   */
  function handleLoadGenerateTaskResult(): void {
    setMessage('正在读取异步题目流水线结果。')
    setWarnings([])
    loadGenerateTaskResultMutation.mutate()
  }

  /**
   * 提交当前勾选题卡，将其批量写入正式题库。
   */
  function handleImportSelected(): void {
    setMessage('正在导入已选题卡。')
    importMutation.mutate()
  }

  /**
   * 将当前勾选题卡创建为异步导入任务，适合批量导入或交给独立 worker 后台处理。
   */
  function handleQueueImportSelected(): void {
    setMessage('正在创建异步导入任务。')
    queueImportTaskMutation.mutate()
  }

  /**
   * 切换抓取来源选择状态，控制抓取素材范围。
   */
  function toggleSource(sourceName: string): void {
    setForm((current) => ({
      ...current,
      sources: current.sources.includes(sourceName)
        ? current.sources.filter((item) => item !== sourceName)
        : [...current.sources, sourceName],
    }))
  }

  /**
   * 批量切换题卡勾选状态，便于一次性导入或排除。
   */
  function setAllCardsSelected(nextSelected: boolean): void {
    setCards((current) =>
      current.map((item) => ({
        ...item,
        selected: nextSelected,
      })),
    )
  }

  /**
   * 更新单张题卡的可编辑字段，保持导入前的最后确认态。
   */
  function updateCardField<K extends keyof EditablePipelineCard>(
    cardId: string,
    field: K,
    value: EditablePipelineCard[K],
  ): void {
    setCards((current) =>
      current.map((item) => (item.id === cardId ? { ...item, [field]: value } : item)),
    )
  }

  if (industriesQuery.isLoading || categoriesQuery.isLoading || sourcesQuery.isLoading) {
    return (
      <section className="admin-panel">
        <span className="admin-tag">题目流水线</span>
        <h2>题目流水线</h2>
        <p className="admin-copy">正在加载行业、分类与抓取来源配置。</p>
      </section>
    )
  }

  if (industriesQuery.isError || categoriesQuery.isError || sourcesQuery.isError) {
    return (
      <section className="admin-panel">
        <span className="admin-tag">题目流水线</span>
        <h2>题目流水线</h2>
        <p className="admin-copy">
          {extractErrorMessage(
            industriesQuery.error || categoriesQuery.error || sourcesQuery.error,
            '读取题目流水线配置失败',
          )}
        </p>
      </section>
    )
  }

  return (
    <section className={`admin-panel admin-question-pipeline-page ${isStreaming ? 'admin-question-pipeline-page--streaming' : ''}`}>
      <div className="admin-question-pipeline-page__hero">
        <div>
          <span className="admin-tag">题目流水线</span>
          <h2>大模型题库流水线</h2>
          <p className="admin-copy">
            输入岗位要求和智能体命令后，你可以切换“两阶段规划”或“逐张直生”两种模式。前者先拆解考点再批量生成，后者按单卡循环生成，便于对比稳定性与指令遵循效果。
          </p>
        </div>
        <div className="admin-question-pipeline-page__summary">
          <strong>{cards.length}</strong>
          <span>候选题卡</span>
          <small>{isStreaming ? '逐张落屏中' : `已勾选 ${selectedCount} 张`}</small>
        </div>
      </div>

      <form className="admin-question-pipeline-composer" onSubmit={handleGenerate}>
        <div className="admin-question-pipeline-composer__grid">
          <label className="admin-field">
            <span>目标行业</span>
            <select
              value={form.industryCode}
              onChange={(event) => setForm((current) => ({ ...current, industryCode: event.target.value }))}
            >
              <option value="">请选择行业</option>
              {(industriesQuery.data || []).map((industry) => (
                <option key={industry.code} value={industry.code}>
                  {industry.name}
                </option>
              ))}
            </select>
          </label>

          <label className="admin-field">
            <span>候选数量</span>
            <select
              value={form.candidateCount}
              onChange={(event) => setForm((current) => ({ ...current, candidateCount: event.target.value }))}
            >
              <option value="6">6 张</option>
              <option value="8">8 张</option>
              <option value="12">12 张</option>
              <option value="16">16 张</option>
            </select>
          </label>

          <label className="admin-field">
            <span>生成模式</span>
            <select
              value={form.generationMode}
              onChange={(event) =>
                setForm((current) => ({
                  ...current,
                  generationMode: normalizeQuestionPipelineGenerationMode(event.target.value),
                }))
              }
            >
              <option value="planned">两阶段规划</option>
              <option value="direct_single">逐张直生</option>
            </select>
          </label>
        </div>

        <label className="admin-field">
          <span>岗位要求 / 清洗目标</span>
          <textarea
            className="admin-question-pipeline-composer__requirement"
            value={form.requirement}
            onChange={(event) => setForm((current) => ({ ...current, requirement: event.target.value }))}
            placeholder="例如：生成 Go 后端高级工程师面试题，重点覆盖并发、MySQL、Redis、微服务治理，结合真实项目经验，输出中高级难度。"
          />
        </label>

        <label className="admin-field">
          <span>智能体命令 / 自定义提示词</span>
          <textarea
            className="admin-question-pipeline-composer__prompt"
            value={form.agentPrompt}
            onChange={(event) => setForm((current) => ({ ...current, agentPrompt: event.target.value }))}
            placeholder="例如：参考 Go 语言核心特性生成 8 道互不重复的问答题，聚焦语言理解，不要项目题，不要八股套话。"
          />
        </label>

        <div className="admin-question-pipeline-composer__strategy">
          <label className="admin-question-pipeline-composer__switch">
            <input
              type="checkbox"
              checked={form.includeScraped}
              onChange={(event) => setForm((current) => ({ ...current, includeScraped: event.target.checked }))}
            />
            <span>抓取相关面经素材</span>
          </label>

          <label className="admin-question-pipeline-composer__switch">
            <input
              type="checkbox"
              checked={form.includeGenerated}
              onChange={(event) => setForm((current) => ({ ...current, includeGenerated: event.target.checked }))}
            />
            <span>调用大模型生成 / 改写</span>
          </label>
        </div>

        <div className="admin-question-pipeline-composer__sources">
          {(sourcesQuery.data || []).map((source) => (
            <label key={source.name} className="admin-question-pipeline-composer__source-chip">
              <input
                type="checkbox"
                checked={form.sources.includes(source.name)}
                onChange={() => toggleSource(source.name)}
                disabled={!form.includeScraped}
              />
              <span>{source.label}</span>
            </label>
          ))}
        </div>

        <div className={`admin-question-pipeline-composer__status ${formError ? 'is-error' : 'is-valid'}`}>
          <div>
            <strong>流水线状态</strong>
            <span>{formError || message}</span>
          </div>
          <div className="admin-question-pipeline-composer__status-actions">
            {isStreaming ? (
              <span className="admin-question-pipeline-progress">
                <i className="admin-question-pipeline-progress__dot" />
                正在逐张生成
              </span>
            ) : null}
            <button
              className="admin-link"
              type="button"
              onClick={handleCancelStream}
              disabled={!isStreaming}
            >
              停止生成
            </button>
            <button
              className="admin-link"
              type="submit"
              disabled={Boolean(formError) || generateMutation.isPending || isStreaming || queueGenerateTaskMutation.isPending}
            >
              {generateMutation.isPending || isStreaming ? '生成中...' : '生成候选题卡'}
            </button>
            <button
              className="admin-link"
              type="button"
              onClick={handleQueueGenerateTask}
              disabled={Boolean(formError) || generateMutation.isPending || isStreaming || queueGenerateTaskMutation.isPending}
            >
              {queueGenerateTaskMutation.isPending ? '入队中...' : '异步生成候选题卡'}
            </button>
          </div>
        </div>

        <div className="admin-question-pipeline-composer__task-restore">
          <label className="admin-field">
            <span>异步任务 ID</span>
            <input
              value={asyncGenerateTaskId}
              onChange={(event) => setAsyncGenerateTaskId(event.target.value.replace(/[^\d]/g, ''))}
              placeholder="输入题目流水线任务 ID"
            />
          </label>
          <button
            className="admin-link"
            type="button"
            onClick={handleLoadGenerateTaskResult}
            disabled={!asyncGenerateTaskId || loadGenerateTaskResultMutation.isPending}
          >
            {loadGenerateTaskResultMutation.isPending ? '读取中...' : '恢复异步结果'}
          </button>
        </div>
      </form>

      {stats ? (
        <div className="admin-question-pipeline-stats">
          <div className="admin-question-pipeline-stats__item">
            <strong>{stats.searched_count}</strong>
            <span>搜索结果</span>
          </div>
          <div className="admin-question-pipeline-stats__item">
            <strong>{stats.fetched_count}</strong>
            <span>已抓取素材</span>
          </div>
          <div className="admin-question-pipeline-stats__item">
            <strong>{stats.generated_count}</strong>
            <span>AI 产出题卡</span>
          </div>
          <div className="admin-question-pipeline-stats__item">
            <strong>{stats.candidate_count}</strong>
            <span>最终候选</span>
          </div>
        </div>
      ) : null}

      {warnings.length > 0 ? (
        <div className="admin-question-pipeline-warnings">
          {warnings.map((warning) => (
            <p key={warning}>{warning}</p>
          ))}
        </div>
      ) : null}

      {debugEntries.length > 0 ? (
        <div className="admin-question-pipeline-debug">
          <div className="admin-question-pipeline-debug__head">
            <strong>原始输出调试</strong>
            <span>共记录 {debugEntries.length} 次失败调用，可直接查看模型原始输出。</span>
          </div>
          {debugEntries.map((entry) => (
            <details key={entry.id} className="admin-question-pipeline-debug__item">
              <summary>
                {entry.slotIndex > 0 ? `第 ${entry.slotIndex} 张` : '未定位卡位'}
                {entry.retryIndex > 0 ? ` · 第 ${entry.retryIndex} 次尝试` : ''}
                {entry.traceId ? ` · trace_id: ${entry.traceId}` : ''}
              </summary>
              <p>{entry.message}</p>
              {entry.traceId ? <p>你也可以去 AI 调用日志页按 trace_id 检索这次调用。</p> : null}
              <pre>{entry.rawOutput || '当前事件未携带原始输出。'}</pre>
            </details>
          ))}
        </div>
      ) : null}

      <div className="admin-question-pipeline-results__toolbar">
        <div className="admin-question-pipeline-results__selection">
          <button className="admin-link" type="button" onClick={() => setAllCardsSelected(true)} disabled={cards.length === 0}>
            全选
          </button>
          <button className="admin-link" type="button" onClick={() => setAllCardsSelected(false)} disabled={cards.length === 0}>
            全不选
          </button>
        </div>
        <button
          className="admin-link"
          type="button"
          onClick={handleImportSelected}
          disabled={selectedCount === 0 || importMutation.isPending || queueImportTaskMutation.isPending}
        >
          {importMutation.isPending ? '导入中...' : `导入已选 ${selectedCount} 张`}
        </button>
        <button
          className="admin-link"
          type="button"
          onClick={handleQueueImportSelected}
          disabled={selectedCount === 0 || importMutation.isPending || queueImportTaskMutation.isPending}
        >
          {queueImportTaskMutation.isPending ? '入队中...' : `异步入队 ${selectedCount} 张`}
        </button>
      </div>

      {cards.length === 0 ? (
        <div className="admin-question-pipeline-empty">
          <strong>还没有候选题卡</strong>
          <p>先填写岗位要求并执行一次流水线，结果会以卡片形式展示在这里。</p>
        </div>
      ) : (
        <div className="admin-question-pipeline-results">
          {cards.map((card) => (
            <article
              key={card.id}
              className={`admin-question-pipeline-card ${card.selected ? 'admin-question-pipeline-card--active' : ''} ${freshCardIds.includes(card.id) ? 'admin-question-pipeline-card--fresh' : ''}`}
            >
              <div className="admin-question-pipeline-card__head">
                <label className="admin-question-pipeline-card__checkbox">
                  <input
                    type="checkbox"
                    checked={card.selected}
                    onChange={(event) => updateCardField(card.id, 'selected', event.target.checked)}
                  />
                  <span>加入题库</span>
                </label>
                <div className="admin-question-pipeline-card__badges">
                  <span>{sourceTypeLabel(card.source_type)}</span>
                  <span>{Math.round(card.confidence * 100)}% 置信度</span>
                </div>
              </div>

              <div className="admin-question-pipeline-card__meta">
                <span>{card.source_label || '未标注来源'}</span>
                <span>{QUESTION_TYPE_OPTIONS.find((item) => item.value === card.type)?.label || card.type}</span>
                <span>{QUESTION_DIFFICULTY_OPTIONS.find((item) => item.value === card.difficulty)?.label || card.difficulty}</span>
              </div>

              {card.source_title ? <p className="admin-question-pipeline-card__source-title">素材：{card.source_title}</p> : null}
              {card.source_url ? (
                <a className="admin-question-pipeline-card__source-link" href={card.source_url} target="_blank" rel="noreferrer">
                  查看原始来源
                </a>
              ) : null}

              <label className="admin-field">
                <span>题目标题</span>
                <input value={card.title} onChange={(event) => updateCardField(card.id, 'title', event.target.value)} />
              </label>

              <div className="admin-question-pipeline-card__grid">
                <label className="admin-field">
                  <span>题型</span>
                  <select
                    value={card.type}
                    onChange={(event) => updateCardField(card.id, 'type', event.target.value as QuestionType)}
                  >
                    {QUESTION_TYPE_OPTIONS.map((option) => (
                      <option key={option.value} value={option.value}>
                        {option.label}
                      </option>
                    ))}
                  </select>
                </label>

                <label className="admin-field">
                  <span>难度</span>
                  <select
                    value={card.difficulty}
                    onChange={(event) => updateCardField(card.id, 'difficulty', event.target.value as QuestionDifficulty)}
                  >
                    {QUESTION_DIFFICULTY_OPTIONS.map((option) => (
                      <option key={option.value} value={option.value}>
                        {option.label}
                      </option>
                    ))}
                  </select>
                </label>

                <label className="admin-field">
                  <span>分类</span>
                  <select value={card.category} onChange={(event) => updateCardField(card.id, 'category', event.target.value)}>
                    {categoryOptions.map((option) => (
                      <option key={option.id} value={option.name}>
                        {option.name}
                      </option>
                    ))}
                  </select>
                </label>
              </div>

              <label className="admin-field">
                <span>题目内容</span>
                <textarea
                  className="admin-question-pipeline-card__content"
                  value={card.content}
                  onChange={(event) => updateCardField(card.id, 'content', event.target.value)}
                />
              </label>

              <label className="admin-field">
                <span>标准答案</span>
                <textarea
                  className="admin-question-pipeline-card__answer"
                  value={card.answer}
                  onChange={(event) => updateCardField(card.id, 'answer', event.target.value)}
                />
              </label>

              <label className="admin-field">
                <span>解析</span>
                <textarea
                  className="admin-question-pipeline-card__answer"
                  value={card.explanation}
                  onChange={(event) => updateCardField(card.id, 'explanation', event.target.value)}
                />
              </label>

              <label className="admin-field">
                <span>标签</span>
                <input value={card.tagsText} onChange={(event) => updateCardField(card.id, 'tagsText', event.target.value)} />
              </label>
            </article>
          ))}
        </div>
      )}
    </section>
  )
}
