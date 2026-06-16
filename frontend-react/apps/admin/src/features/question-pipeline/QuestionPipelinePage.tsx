import type { FormEvent } from 'react'
import { useEffect, useMemo, useRef, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { buildAuthorizationHeader, extractErrorMessage, getApiBaseUrl, requestJson } from '@makejob/api-client'
import { isSuccessCode, type ApiEnvelope } from '@makejob/shared-types'
import { flushSync } from 'react-dom'
import {
  Alert,
  Badge,
  Button,
  Card,
  Col,
  ConfigProvider,
  Empty,
  Input,
  InputNumber,
  message,
  Row,
  Select,
  Space,
  Spin,
  Switch,
  Tag,
  Typography,
} from 'antd'
import {
  ThunderboltOutlined,
  ReloadOutlined,
  CheckCircleOutlined,
  PlusOutlined,
  StopOutlined,
  CloudUploadOutlined,
  CloudSyncOutlined,
  LoadingOutlined,
  WarningOutlined,
  CodeOutlined,
  BookOutlined,
  TagOutlined,
  LinkOutlined,
  FileTextOutlined,
  DownOutlined,
  UpOutlined,
} from '@ant-design/icons'
import { useAdminAuthStore } from '../../state/auth'
import { fetchScraperTaskDetail } from '../runtime/runtimeApi'
import type { ScraperTaskDetail } from '../runtime/runtimeTypes'

const { Title, Text, Paragraph } = Typography

type QuestionType = 'choice' | 'multi' | 'code' | 'subjective'
type QuestionDifficulty = 'easy' | 'medium' | 'hard'
type QuestionPipelineGenerationMode = 'direct_single'
type QuestionEvaluationMode = 'analysis_only' | 'testcase'

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

interface QuestionTestCase {
  input: string
  expected_output: string
  description?: string
}

interface QuestionReferenceSolution {
  language: string
  title?: string
  code: string
  explanation?: string
}

interface QuestionJudgeConfig {
  evaluation_mode: QuestionEvaluationMode
  default_language: string
  allowed_languages: string[]
  starter_code: string
  public_test_cases: QuestionTestCase[]
  hidden_test_cases: QuestionTestCase[]
  reference_solutions: QuestionReferenceSolution[]
  time_limit_ms: number
  memory_limit_mb: number
}

interface QuestionPipelineCard {
  id: string
  title: string
  content: string
  type: QuestionType
  difficulty: QuestionDifficulty
  category: string
  answer: string
  solution: string
  explanation: string
  tags: string[]
  judge_config?: QuestionJudgeConfig | null
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
  solution?: unknown
  explanation?: unknown
  tags?: unknown
  judge_config?: unknown
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
  failure_stage?: unknown
  candidate_excerpt?: unknown
  repair_attempted?: unknown
  supplement_attempted?: unknown
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
  candidateCount: string
  includeScraped: boolean
  includeGenerated: boolean
  sources: string[]
}

interface EditablePipelineCard extends QuestionPipelineCard {
  selected: boolean
  tagsText: string
  judgeConfigText: string
}

interface StreamErrorPayload {
  message?: unknown
}

interface PipelineDebugEntry {
  id: string
  message: string
  traceId: string
  rawOutput: string
  failureStage: string
  candidateExcerpt: string
  repairAttempted: boolean
  supplementAttempted: boolean
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

/* ------------------------------------------------------------------ */
/*  视觉 token（与前两页完全一致）                                      */
/* ------------------------------------------------------------------ */

const THEME = {
  token: {
    borderRadius: 14,
    borderRadiusLG: 20,
    colorPrimary: '#2563eb',
    colorBgContainer: '#ffffff',
    colorBorder: '#e2e8f0',
    colorBorderSecondary: '#f1f5f9',
    colorText: '#0f172a',
    colorTextSecondary: '#64748b',
    colorTextTertiary: '#94a3b8',
    fontFamily: 'Inter, "SF Pro Display", -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif',
    controlHeight: 44,
    controlHeightLG: 52,
    controlHeightSM: 36,
    boxShadow: '0 0 0 1px rgba(0,0,0,0.03), 0 2px 8px rgba(0,0,0,0.04), 0 12px 24px rgba(0,0,0,0.03)',
    boxShadowSecondary: '0 0 0 1px rgba(0,0,0,0.04), 0 8px 16px rgba(0,0,0,0.06), 0 24px 48px rgba(0,0,0,0.04)',
  },
}

const glassCard = {
  background: 'rgba(255, 255, 255, 0.85)',
  backdropFilter: 'blur(20px) saturate(180%)',
  WebkitBackdropFilter: 'blur(20px) saturate(180%)',
  borderRadius: 20,
  border: '1px solid rgba(255, 255, 255, 0.6)',
  boxShadow: '0 0 0 1px rgba(0,0,0,0.03), 0 2px 8px rgba(0,0,0,0.04), 0 12px 24px rgba(0,0,0,0.03)',
  transition: 'all 0.35s cubic-bezier(0.4, 0, 0.2, 1)',
} as React.CSSProperties

const solidCard = {
  background: '#ffffff',
  borderRadius: 20,
  border: '1px solid #f1f5f9',
  boxShadow: '0 0 0 1px rgba(0,0,0,0.02), 0 4px 12px rgba(0,0,0,0.03)',
  transition: 'all 0.35s cubic-bezier(0.4, 0, 0.2, 1)',
} as React.CSSProperties

/* ------------------------------------------------------------------ */
/*  API 与工具函数（全部保留原逻辑）                                     */
/* ------------------------------------------------------------------ */

async function fetchIndustries(token: string | null): Promise<Industry[]> {
  const response = await requestJson<ApiEnvelope<Industry[]>>('/admin/industries', { method: 'GET', token })
  if (!isSuccessCode(response.code)) throw new Error(response.message || '获取行业列表失败')
  return response.data
}

async function fetchCategories(token: string | null): Promise<Category[]> {
  const response = await requestJson<ApiEnvelope<Category[]>>('/admin/categories', { method: 'GET', token })
  if (!isSuccessCode(response.code)) throw new Error(response.message || '获取分类列表失败')
  return response.data
}

async function fetchScraperSources(token: string | null): Promise<ScraperSource[]> {
  const response = await requestJson<ApiEnvelope<ScraperSource[]>>('/admin/scraper/sources', { method: 'GET', token })
  if (!isSuccessCode(response.code)) throw new Error(response.message || '获取抓取来源失败')
  return response.data
}

async function generateQuestionPipeline(
  token: string | null,
  payload: Record<string, unknown>,
): Promise<QuestionPipelineGenerateResponse> {
  const response = await requestJson<ApiEnvelope<RawQuestionPipelineGenerateResponse | null>>('/admin/question-pipeline/generate', {
    method: 'POST',
    token,
    body: payload,
  })
  if (!isSuccessCode(response.code)) throw new Error(response.message || '生成题目流水线失败')
  return normalizeQuestionPipelineGenerateResponse(response.data)
}

function buildQuestionPipelineGeneratePayload(form: PipelineFormState): Record<string, unknown> {
  return {
    industry_code: form.industryCode,
    requirement: form.requirement.trim(),
    agent_prompt: form.agentPrompt.trim(),
    generation_mode: 'direct_single',
    candidate_count: Number(form.candidateCount) || 8,
    include_scraped: form.includeScraped,
    include_generated: form.includeGenerated,
    sources: form.includeScraped ? form.sources : [],
  }
}

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

  if (!response.ok) throw new Error(await buildQuestionPipelineStreamErrorMessage(response))
  if (!response.body) throw new Error('流式生成接口未返回可读取的数据流。')

  const reader = response.body.getReader()
  const decoder = new TextDecoder('utf-8')
  let buffer = ''

  const processEventBlock = (block: string): void => {
    const normalizedBlock = block.trim()
    if (!normalizedBlock) return

    let eventName = ''
    const dataLines: string[] = []
    for (const line of normalizedBlock.split(/\r?\n/)) {
      if (line.startsWith('event:')) {
        eventName = line.slice('event:'.length).trim()
      } else if (line.startsWith('data:')) {
        dataLines.push(line.slice('data:'.length).trim())
      }
    }
    if (dataLines.length === 0) return

    const rawPayload = JSON.parse(dataLines.join('\n')) as RawQuestionPipelineStreamEvent
    const effectiveEvent = typeof rawPayload.event === 'string' && rawPayload.event.trim()
      ? rawPayload.event.trim()
      : eventName
    const message = typeof rawPayload.message === 'string' ? rawPayload.message.trim() : ''
    const traceId = typeof rawPayload.trace_id === 'string' ? rawPayload.trace_id.trim() : ''
    const rawOutput = typeof rawPayload.raw_output === 'string' ? rawPayload.raw_output.trim() : ''
    const failureStage = typeof rawPayload.failure_stage === 'string' ? rawPayload.failure_stage.trim() : ''
    const candidateExcerpt = typeof rawPayload.candidate_excerpt === 'string' ? rawPayload.candidate_excerpt.trim() : ''
    const repairAttempted = rawPayload.repair_attempted === true
    const supplementAttempted = rawPayload.supplement_attempted === true
    const slotIndex = typeof rawPayload.slot_index === 'number' ? rawPayload.slot_index : 0
    const retryIndex = typeof rawPayload.retry_index === 'number' ? rawPayload.retry_index : 0

    switch (effectiveEvent) {
      case 'status':
        if (message) callbacks.onStatus(message)
        return
      case 'warning':
        if (message) {
          callbacks.onWarning(
            rawOutput || traceId || failureStage || candidateExcerpt
              ? {
                  id: `${traceId || 'warning'}-${failureStage || 'unknown'}-${slotIndex || 0}-${retryIndex || 0}-${message}`,
                  message,
                  traceId,
                  rawOutput,
                  failureStage,
                  candidateExcerpt,
                  repairAttempted,
                  supplementAttempted,
                  slotIndex,
                  retryIndex,
                }
              : null,
            message,
          )
        }
        return
      case 'error':
        if (message) callbacks.onError(message)
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
        if (message) callbacks.onStatus(message)
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
      if (remaining) buffer += remaining
      if (buffer.trim()) processEventBlock(buffer)
      break
    }
  }
}

async function buildQuestionPipelineStreamErrorMessage(response: Response): Promise<string> {
  const fallback = `流式生成失败，状态码：${response.status}`
  const rawText = (await response.text()).trim()
  if (!rawText) return fallback
  try {
    const payload = JSON.parse(rawText) as StreamErrorPayload
    if (typeof payload.message === 'string' && payload.message.trim()) return payload.message.trim()
  } catch { /* noop */ }
  return rawText
}

async function queueQuestionPipelineGenerateTask(
  token: string | null,
  payload: Record<string, unknown>,
): Promise<AdminAsyncTaskResponse> {
  const response = await requestJson<ApiEnvelope<AdminAsyncTaskResponse>>('/admin/question-pipeline/generate/async', {
    method: 'POST',
    token,
    body: payload,
  })
  if (!isSuccessCode(response.code) || !response.data) throw new Error(response.message || '创建异步题目流水线任务失败')
  return response.data
}

async function importQuestionPipeline(
  token: string | null,
  payload: Record<string, unknown>,
): Promise<BatchImportResponse> {
  const response = await requestJson<ApiEnvelope<BatchImportResponse>>('/admin/question-pipeline/import', {
    method: 'POST',
    token,
    body: payload,
  })
  if (!isSuccessCode(response.code)) throw new Error(response.message || '导入题目流水线失败')
  return response.data
}

async function queueQuestionPipelineImportTask(
  token: string | null,
  payload: Record<string, unknown>,
): Promise<AdminAsyncTaskResponse> {
  const response = await requestJson<ApiEnvelope<AdminAsyncTaskResponse>>('/admin/scraper/import/async', {
    method: 'POST',
    token,
    body: payload,
  })
  if (!isSuccessCode(response.code) || !response.data) throw new Error(response.message || '创建异步导入任务失败')
  return response.data
}

function buildInitialPipelineForm(): PipelineFormState {
  return {
    industryCode: '',
    requirement: '',
    agentPrompt: '确保每张题卡考察不同考点，严格遵守岗位要求里的题型与输出结构，避免模板化和重复表述。',
    candidateCount: '8',
    includeScraped: true,
    includeGenerated: true,
    sources: [],
  }
}

function buildEditableCards(cards: QuestionPipelineCard[]): EditablePipelineCard[] {
  return cards.map((card) => ({
    ...card,
    selected: true,
    tagsText: (card.tags || []).join(', '),
    judgeConfigText: formatQuestionJudgeConfigText(card.judge_config),
  }))
}

function buildSelectedImportPayload(cards: EditablePipelineCard[]) {
  return cards
    .filter((item) => item.selected)
    .map((item) => {
      const judgeConfig = item.type === 'code' ? parseQuestionPipelineJudgeConfigText(item.judgeConfigText) : undefined
      if (item.type === 'code') {
        if (!item.solution.trim()) throw new Error(`编程题《${item.title || '未命名题卡'}》缺少代码思路解析`)
        validateQuestionPipelineCodeJudgeConfig(judgeConfig)
      }
      return {
        title: item.title.trim(),
        content: item.content.trim(),
        type: item.type,
        difficulty: item.difficulty,
        category: item.category,
        answer: item.answer.trim(),
        solution: item.solution.trim(),
        explanation: item.explanation.trim(),
        tags: parseTagsInput(item.tagsText),
        judge_config: judgeConfig,
      }
    })
}

function restoreQuestionPipelineFormFromTaskPayload(
  current: PipelineFormState,
  payloadJSON?: string,
): PipelineFormState {
  if (!payloadJSON?.trim()) return current
  try {
    const payload = JSON.parse(payloadJSON) as Record<string, unknown>
    const candidateCount = typeof payload.candidate_count === 'number' ? String(payload.candidate_count) : current.candidateCount
    return {
      ...current,
      industryCode: typeof payload.industry_code === 'string' ? payload.industry_code.trim() : current.industryCode,
      requirement: typeof payload.requirement === 'string' ? payload.requirement.trim() : current.requirement,
      agentPrompt: typeof payload.agent_prompt === 'string' ? payload.agent_prompt.trim() : current.agentPrompt,
      candidateCount,
      includeScraped: typeof payload.include_scraped === 'boolean' ? payload.include_scraped : current.includeScraped,
      includeGenerated: typeof payload.include_generated === 'boolean' ? payload.include_generated : current.includeGenerated,
      sources: Array.isArray(payload.sources)
        ? payload.sources.filter((item): item is string => typeof item === 'string').map((item) => item.trim()).filter(Boolean)
        : current.sources,
    }
  } catch { return current }
}

function restoreQuestionPipelineResponseFromTask(task: ScraperTaskDetail): QuestionPipelineGenerateResponse {
  if (!task.result_json?.trim()) throw new Error('当前任务还没有可恢复的候选题卡结果')
  try {
    return normalizeQuestionPipelineGenerateResponse(JSON.parse(task.result_json) as RawQuestionPipelineGenerateResponse)
  } catch { throw new Error('题目流水线任务结果解析失败') }
}

function mergeEditableCards(current: EditablePipelineCard[], incoming: EditablePipelineCard[]): EditablePipelineCard[] {
  const merged = [...current]
  for (const card of incoming) {
    const index = merged.findIndex((item) => item.id === card.id)
    if (index >= 0) merged[index] = { ...merged[index], ...card }
    else merged.push(card)
  }
  return merged
}

function mergePipelineDebugEntries(current: PipelineDebugEntry[], incoming: PipelineDebugEntry): PipelineDebugEntry[] {
  const index = current.findIndex((item) => item.id === incoming.id)
  if (index >= 0) {
    const next = [...current]
    next[index] = incoming
    return next
  }
  return [...current, incoming]
}

function reconcileEditableCards(current: EditablePipelineCard[], incoming: EditablePipelineCard[]): EditablePipelineCard[] {
  return incoming.map((card) => {
    const existing = current.find((item) => item.id === card.id)
    if (!existing) return card
    return {
      ...card,
      selected: existing.selected,
      title: existing.title,
      content: existing.content,
      type: existing.type,
      difficulty: existing.difficulty,
      category: existing.category,
      answer: existing.answer,
      solution: existing.solution,
      explanation: existing.explanation,
      tagsText: existing.tagsText,
      judge_config: existing.judge_config,
      judgeConfigText: existing.judgeConfigText,
    }
  })
}

function normalizeStringList(value: unknown): string[] {
  if (!Array.isArray(value)) return []
  return value.filter((item): item is string => typeof item === 'string').map((item) => item.trim()).filter(Boolean)
}

function buildDefaultPipelineJudgeConfig(): QuestionJudgeConfig {
  return {
    evaluation_mode: 'testcase',
    default_language: 'go',
    allowed_languages: ['go'],
    starter_code: '',
    public_test_cases: [],
    hidden_test_cases: [],
    reference_solutions: [],
    time_limit_ms: 2000,
    memory_limit_mb: 128,
  }
}

function normalizeQuestionJudgeConfigValue(value: unknown): QuestionJudgeConfig | null {
  if (!value) return null
  let payload = value
  if (typeof value === 'string') {
    try { payload = JSON.parse(value) as unknown } catch { return null }
  }
  if (!payload || typeof payload !== 'object' || Array.isArray(payload)) return null
  const record = payload as Record<string, unknown>
  const evaluationMode: QuestionEvaluationMode = record.evaluation_mode === 'testcase' ? 'testcase' : 'analysis_only'
  const defaultLanguage = typeof record.default_language === 'string' && record.default_language.trim() ? record.default_language.trim() : 'go'
  const allowedLanguages = normalizeStringList(record.allowed_languages)
  return {
    evaluation_mode: evaluationMode,
    default_language: defaultLanguage,
    allowed_languages: allowedLanguages.length > 0 ? allowedLanguages : [defaultLanguage],
    starter_code: typeof record.starter_code === 'string' ? record.starter_code : '',
    public_test_cases: Array.isArray(record.public_test_cases) ? (record.public_test_cases as QuestionTestCase[]) : [],
    hidden_test_cases: Array.isArray(record.hidden_test_cases) ? (record.hidden_test_cases as QuestionTestCase[]) : [],
    reference_solutions: Array.isArray(record.reference_solutions) ? (record.reference_solutions as QuestionReferenceSolution[]) : [],
    time_limit_ms: typeof record.time_limit_ms === 'number' && record.time_limit_ms > 0 ? record.time_limit_ms : 2000,
    memory_limit_mb: typeof record.memory_limit_mb === 'number' && record.memory_limit_mb > 0 ? record.memory_limit_mb : 128,
  }
}

function formatQuestionJudgeConfigText(value?: QuestionJudgeConfig | null): string {
  return JSON.stringify(value || buildDefaultPipelineJudgeConfig(), null, 2)
}

function parseQuestionPipelineJudgeConfigText(value: string): QuestionJudgeConfig | undefined {
  const trimmed = value.trim()
  if (!trimmed) return undefined
  let parsed: unknown
  try { parsed = JSON.parse(trimmed) as unknown } catch { throw new Error('编程题 judge_config 必须是合法 JSON') }
  const normalized = normalizeQuestionJudgeConfigValue(parsed)
  if (!normalized) throw new Error('编程题 judge_config 必须是 JSON 对象')
  return normalized
}

function validateQuestionPipelineCodeJudgeConfig(value?: QuestionJudgeConfig): void {
  if (!value) throw new Error('编程题缺少 judge_config')
  if (value.evaluation_mode !== 'testcase') throw new Error('编程题 judge_config 必须使用 testcase 判题模式')
  if ((value.public_test_cases || []).length !== 3) throw new Error('编程题必须提供 3 条公开测试用例')
  if ((value.hidden_test_cases || []).length === 0) throw new Error('编程题必须提供隐藏测试用例')
  if ((value.reference_solutions || []).length === 0) throw new Error('编程题必须提供代码参考答案')
}

function normalizeQuestionType(value: unknown): QuestionType {
  const normalized = typeof value === 'string' ? value.trim().toLowerCase() : ''
  if (normalized === 'choice' || normalized === 'single' || normalized === 'singlechoice') return 'choice'
  if (normalized === 'multi' || normalized === 'multiple' || normalized === 'multiplechoice') return 'multi'
  if (normalized === 'code' || normalized === 'coding' || normalized === 'programming' || normalized === 'algorithm' || normalized === '编程题' || normalized === '代码题' || normalized === '算法题') return 'code'
  if (normalized === 'subjective' || normalized === 'qa' || normalized === 'essay' || normalized === '问答题' || normalized === '主观题') return 'subjective'
  return 'subjective'
}

function normalizeQuestionDifficulty(value: unknown): QuestionDifficulty {
  if (value === 'easy' || value === 'medium' || value === 'hard') return value
  return 'medium'
}

function normalizeQuestionPipelineGenerationMode(value: unknown): QuestionPipelineGenerationMode {
  return 'direct_single'
}

function normalizeQuestionPipelineCard(card: RawQuestionPipelineCard, index: number): QuestionPipelineCard {
  const title = typeof card.title === 'string' ? card.title.trim() : ''
  const content = typeof card.content === 'string' ? card.content.trim() : ''
  const answer = typeof card.answer === 'string' ? card.answer.trim() : ''
  const solution = typeof card.solution === 'string' ? card.solution.trim() : ''
  const type = normalizeQuestionType(card.type)
  const judgeConfig = normalizeQuestionJudgeConfigValue(card.judge_config)
  return {
    id: typeof card.id === 'string' && card.id.trim() ? card.id.trim() : `pipeline-card-${index + 1}`,
    title,
    content,
    type,
    difficulty: normalizeQuestionDifficulty(card.difficulty),
    category: typeof card.category === 'string' ? card.category.trim() : '',
    answer,
    solution,
    explanation: typeof card.explanation === 'string' ? card.explanation.trim() : '',
    tags: normalizeStringList(card.tags),
    judge_config: type === 'code' ? (judgeConfig || buildDefaultPipelineJudgeConfig()) : judgeConfig,
    confidence: typeof card.confidence === 'number' ? card.confidence : 0,
    source_type: typeof card.source_type === 'string' ? card.source_type.trim() : 'generated',
    source_label: typeof card.source_label === 'string' ? card.source_label.trim() : 'AI 智能体生成',
    source_title: typeof card.source_title === 'string' ? card.source_title.trim() : '',
    source_url: typeof card.source_url === 'string' ? card.source_url.trim() : '',
  }
}

function normalizeQuestionPipelineGenerateResponse(
  payload: RawQuestionPipelineGenerateResponse | null | undefined,
): QuestionPipelineGenerateResponse {
  if (!payload || typeof payload !== 'object') throw new Error('生成接口已返回成功，但未携带候选题卡数据。')
  const cards = Array.isArray(payload.cards)
    ? payload.cards.map((item, index) => normalizeQuestionPipelineCard((item || {}) as RawQuestionPipelineCard, index)).filter((item) => item.title && item.content && item.answer)
    : []
  if (cards.length === 0) throw new Error('生成接口已返回成功，但没有可展示的候选题卡。')
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

function formatQuestionPipelineFailureStage(stage: string): string {
  switch (stage.trim()) {
    case 'parse': return '结构解析失败'
    case 'supplement': return '编程题补齐失败'
    case 'constraint': return '约束校验失败'
    case 'slot_exhausted': return '重试耗尽'
    case 'model_call': return '模型调用失败'
    case 'provider': return 'Provider 配置异常'
    case 'normalize': return '题卡归一化失败'
    default: return stage.trim() || '未标注阶段'
  }
}

function questionPipelineGenerationModeLabel(_mode: QuestionPipelineGenerationMode): string {
  return '逐张直生'
}

function parseTagsInput(input: string): string[] {
  const values = input.split(/,|，/).map((item) => item.trim()).filter(Boolean)
  return Array.from(new Set(values))
}

function sourceTypeLabel(sourceType: string): string {
  return sourceType === 'generated' ? 'AI 改写' : '抓取清洗'
}

function filterCategoriesByIndustry(
  categories: Category[],
  industries: Industry[],
  industryCode: string,
): Category[] {
  const industry = industries.find((item) => item.code === industryCode)
  if (!industry) return []
  return categories.filter((item) => item.industry_id === industry.id).sort((left, right) => left.sort_order - right.sort_order || left.id - right.id)
}

function buildPipelineFormError(form: PipelineFormState): string {
  if (!form.industryCode) return '请选择题库目标行业。'
  if (!form.requirement.trim()) return '请填写岗位要求、题目方向或清洗要求。'
  if (!form.includeGenerated && !form.includeScraped) return '抓取与 AI 生成至少要启用一种。'
  return ''
}

/* ------------------------------------------------------------------ */
/*  题目流水线页面主体                                                  */
/* ------------------------------------------------------------------ */

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
  const [statusMessage, setStatusMessage] = useState('填写岗位要求后即可生成候选题卡。')
  const [isStreaming, setIsStreaming] = useState(false)
  const [asyncGenerateTaskId, setAsyncGenerateTaskId] = useState('')
  const [expandedDebugId, setExpandedDebugId] = useState<string | null>(null)

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
    if (!sourcesQuery.data || form.sources.length > 0) return
    const activeSources = sourcesQuery.data.filter((item) => item.is_active).map((item) => item.name)
    setForm((current) => ({ ...current, sources: activeSources }))
  }, [form.sources.length, sourcesQuery.data])

  useEffect(() => {
    if (!industriesQuery.data || form.industryCode) return
    const firstActiveIndustry = industriesQuery.data.find((item) => item.is_active)
    if (!firstActiveIndustry) return
    setForm((current) => ({ ...current, industryCode: firstActiveIndustry.code }))
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
      setStatusMessage(`已通过${questionPipelineGenerationModeLabel(result.generation_mode)}生成 ${result.cards.length} 张候选题卡，请确认后再导入题库。`)
    },
    onError: (error) => {
      setStatusMessage(extractErrorMessage(error, '生成题目流水线失败'))
    },
  })

  const queueGenerateTaskMutation = useMutation({
    mutationFn: async () => queueQuestionPipelineGenerateTask(token, buildQuestionPipelineGeneratePayload(form)),
    onSuccess: async (task) => {
      setAsyncGenerateTaskId(String(task?.id ?? ''))
      setStatusMessage(`已创建异步题目流水线任务 #${task?.id ?? ''}，可稍后按任务 ID 恢复结果或前往运行任务页查看状态。`)
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['admin-runtime-scraper-tasks'] }),
        queryClient.invalidateQueries({ queryKey: ['admin-runtime-scraper-tasks-overview'] }),
      ])
    },
    onError: (error) => {
      setStatusMessage(extractErrorMessage(error, '创建异步题目流水线任务失败'))
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
      setStatusMessage(`已导入 ${result.success_count ?? 0} 道题，失败 ${result.fail_count ?? 0} 道。`)
      if ((result.success_count ?? 0) > 0) setCards((current) => current.filter((item) => !item.selected))
      setWarnings(result.errors || [])
    },
    onError: (error) => {
      setStatusMessage(extractErrorMessage(error, '导入题目流水线失败'))
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
      setStatusMessage(`已创建异步导入任务 #${task?.id ?? ''}，可前往运行任务页查看执行状态。`)
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['admin-runtime-scraper-tasks'] }),
        queryClient.invalidateQueries({ queryKey: ['admin-runtime-scraper-tasks-overview'] }),
      ])
    },
    onError: (error) => {
      setStatusMessage(extractErrorMessage(error, '创建异步导入任务失败'))
    },
  })

  const loadGenerateTaskResultMutation = useMutation({
    mutationFn: async () => {
      const taskID = Number(asyncGenerateTaskId)
      if (!Number.isFinite(taskID) || taskID <= 0) throw new Error('请输入有效的异步任务 ID')
      return fetchScraperTaskDetail(token, taskID)
    },
    onSuccess: (task) => {
      if ((task.task_type || '') !== 'question_pipeline_build') {
        setStatusMessage(`任务 #${task.id} 不是题目流水线生成任务，请确认任务 ID。`)
        return
      }
      setForm((current) => restoreQuestionPipelineFormFromTaskPayload(current, task.payload_json))
      if (task.status === 'pending' || task.status === 'running') {
        setStatusMessage(`任务 #${task.id} 当前状态为 ${task.status}，请稍后重新读取结果。`)
        return
      }
      if (task.status === 'failed') {
        setWarnings(task.error_msg ? [task.error_msg] : [])
        setStatusMessage(`任务 #${task.id} 执行失败，可前往运行任务页查看详情后重试。`)
        return
      }
      const result = restoreQuestionPipelineResponseFromTask(task)
      setCards(buildEditableCards(result.cards))
      setFreshCardIds([])
      setDebugEntries([])
      setWarnings(result.warnings || [])
      setStats(result.stats)
      setStatusMessage(`已从异步任务 #${task.id} 恢复 ${result.cards.length} 张候选题卡，请确认后再导入题库。`)
    },
    onError: (error) => {
      setStatusMessage(extractErrorMessage(error, '读取异步题目流水线结果失败'))
    },
  })

  function markFreshCard(cardId: string): void {
    if (!cardId) return
    const currentTimer = freshCardTimerRef.current[cardId]
    if (currentTimer) window.clearTimeout(currentTimer)
    setFreshCardIds((current) => (current.includes(cardId) ? current : [...current, cardId]))
    freshCardTimerRef.current[cardId] = window.setTimeout(() => {
      setFreshCardIds((current) => current.filter((item) => item !== cardId))
      delete freshCardTimerRef.current[cardId]
    }, 1600)
  }

  function handleCancelStream(): void {
    streamAbortRef.current?.abort()
    setIsStreaming(false)
    setStatusMessage('已取消本次流式生成。')
  }

  function handleGenerate(event: FormEvent<HTMLFormElement>): void {
    event.preventDefault()
    if (form.includeGenerated) {
      const controller = new AbortController()
      streamAbortRef.current?.abort()
      streamAbortRef.current = controller
      Object.values(freshCardTimerRef.current).forEach((timer) => window.clearTimeout(timer))
      freshCardTimerRef.current = {}
      setCards([])
      setFreshCardIds([])
      setDebugEntries([])
      setWarnings([])
      setStats({ searched_count: 0, fetched_count: 0, scraped_count: 0, generated_count: 0, candidate_count: 0, selected_sources: 0 })
      setStatusMessage('正在建立流式生成连接。')
      setIsStreaming(true)

      void streamQuestionPipeline(token, buildQuestionPipelineGeneratePayload(form), controller.signal, {
        onStatus: (nextMessage) => { setStatusMessage(nextMessage) },
        onWarning: (debugEntry, warning) => {
          setWarnings((current) => (current.includes(warning) ? current : [...current, warning]))
          if (debugEntry) setDebugEntries((current) => mergePipelineDebugEntries(current, debugEntry))
        },
        onError: (errorMessage) => { setStatusMessage(errorMessage) },
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
          setStatusMessage(`已通过${questionPipelineGenerationModeLabel(result.generation_mode)}生成 ${result.cards.length} 张候选题卡，请确认后再导入题库。`)
        },
      })
        .catch((error) => {
          if (controller.signal.aborted) return
          setStatusMessage(extractErrorMessage(error, '流式生成题目流水线失败'))
        })
        .finally(() => {
          if (streamAbortRef.current === controller) streamAbortRef.current = null
          setIsStreaming(false)
        })
      return
    }
    setStatusMessage('正在生成候选题卡。')
    setDebugEntries([])
    setWarnings([])
    generateMutation.mutate()
  }

  function handleQueueGenerateTask(): void {
    setStatusMessage('正在创建异步题目流水线任务。')
    queueGenerateTaskMutation.mutate()
  }

  function handleLoadGenerateTaskResult(): void {
    setStatusMessage('正在读取异步题目流水线结果。')
    setWarnings([])
    loadGenerateTaskResultMutation.mutate()
  }

  function handleImportSelected(): void {
    setStatusMessage('正在导入已选题卡。')
    importMutation.mutate()
  }

  function handleQueueImportSelected(): void {
    setStatusMessage('正在创建异步导入任务。')
    queueImportTaskMutation.mutate()
  }

  function toggleSource(sourceName: string): void {
    setForm((current) => ({
      ...current,
      sources: current.sources.includes(sourceName)
        ? current.sources.filter((item) => item !== sourceName)
        : [...current.sources, sourceName],
    }))
  }

  function setAllCardsSelected(nextSelected: boolean): void {
    setCards((current) => current.map((item) => ({ ...item, selected: nextSelected })))
  }

  function updateCardField<K extends keyof EditablePipelineCard>(
    cardId: string,
    field: K,
    value: EditablePipelineCard[K],
  ): void {
    setCards((current) => current.map((item) => (item.id === cardId ? { ...item, [field]: value } : item)))
  }

  function handleCardTypeChange(cardId: string, nextType: QuestionType): void {
    setCards((current) => current.map((item) => {
      if (item.id !== cardId) return item
      if (nextType !== 'code') return { ...item, type: nextType }
      let nextJudgeConfig = item.judge_config || buildDefaultPipelineJudgeConfig()
      try {
        nextJudgeConfig = parseQuestionPipelineJudgeConfigText(item.judgeConfigText || '') || nextJudgeConfig
      } catch { nextJudgeConfig = nextJudgeConfig || buildDefaultPipelineJudgeConfig() }
      return {
        ...item,
        type: nextType,
        judge_config: nextJudgeConfig,
        judgeConfigText: formatQuestionJudgeConfigText(nextJudgeConfig),
      }
    }))
  }

  const mutationPending =
    generateMutation.isPending ||
    isStreaming ||
    queueGenerateTaskMutation.isPending ||
    importMutation.isPending ||
    queueImportTaskMutation.isPending ||
    loadGenerateTaskResultMutation.isPending

  if (industriesQuery.isLoading || categoriesQuery.isLoading || sourcesQuery.isLoading) {
    return (
      <ConfigProvider theme={THEME}>
        <div style={{ minHeight: '100vh', display: 'flex', alignItems: 'center', justifyContent: 'center', background: '#f0f2f5' }}>
          <Spin size="large" tip="正在加载行业、分类与抓取来源配置..." />
        </div>
      </ConfigProvider>
    )
  }

  if (industriesQuery.isError || categoriesQuery.isError || sourcesQuery.isError) {
    return (
      <ConfigProvider theme={THEME}>
        <div style={{ padding: 40, maxWidth: 800, margin: '0 auto' }}>
          <Alert
            message="读取题目流水线配置失败"
            description={extractErrorMessage(industriesQuery.error || categoriesQuery.error || sourcesQuery.error, '请稍后重试')}
            type="error"
            showIcon
            style={{ borderRadius: 20, padding: 24 }}
          />
        </div>
      </ConfigProvider>
    )
  }

  return (
    <ConfigProvider theme={THEME}>
      <div
        style={{
          minHeight: '100vh',
          background: '#f0f2f5',
          padding: '32px 24px 64px',
          fontFamily: THEME.token.fontFamily as string,
        }}
      >
        <div style={{ maxWidth: 1200, margin: '0 auto' }}>
          {/* ===== 毛玻璃标题栏 ===== */}
          <div
            style={{
              ...glassCard,
              padding: '28px 32px',
              marginBottom: 28,
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'space-between',
              flexWrap: 'wrap',
              gap: 16,
            }}
          >
            <Space direction="vertical" size={8} style={{ flex: 1, minWidth: 280 }}>
              <Space align="center" size={12}>
                <div
                  style={{
                    width: 40,
                    height: 40,
                    borderRadius: 14,
                    background: 'linear-gradient(135deg, #f59e0b, #d97706)',
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'center',
                    color: '#fff',
                    fontSize: 20,
                    boxShadow: '0 4px 12px rgba(245, 158, 11, 0.3)',
                  }}
                >
                  <ThunderboltOutlined />
                </div>
                <div>
                  <Title level={4} style={{ margin: 0, fontWeight: 700, letterSpacing: '-0.02em' }}>
                    题目流水线
                    {cards.length > 0 && (
                      <Badge
                        count={cards.length}
                        style={{ backgroundColor: '#3b82f6', marginLeft: 10, boxShadow: '0 2px 6px rgba(59, 130, 246, 0.35)' }}
                      />
                    )}
                  </Title>
                </div>
              </Space>
              <Paragraph type="secondary" style={{ margin: 0, maxWidth: 640, fontSize: 14, lineHeight: 1.6 }}>
                输入岗位要求和智能体命令后，系统统一采用逐张直生模式生成候选题卡，实时展示单卡结果、失败原因与原始调试输出。
              </Paragraph>
            </Space>

            <div
              style={{
                padding: '16px 22px',
                borderRadius: 18,
                background: isStreaming
                  ? 'linear-gradient(135deg, #fef3c7, #fde68a)'
                  : 'linear-gradient(135deg, #f1f5f9, #e2e8f0)',
                color: isStreaming ? '#92400e' : '#0f172a',
                display: 'flex',
                flexDirection: 'column',
                gap: 4,
                textAlign: 'center',
                minWidth: 130,
                transition: 'all 0.4s ease',
              }}
            >
              <div style={{ fontSize: 32, fontWeight: 800, lineHeight: 1, letterSpacing: '-0.03em' }}>
                {cards.length}
              </div>
              <Text style={{ fontSize: 13, fontWeight: 500, opacity: 0.8 }}>
                候选题卡
              </Text>
              <Text style={{ fontSize: 12, opacity: 0.65 }}>
                {isStreaming ? '逐张落屏中...' : `已勾选 ${selectedCount} 张`}
              </Text>
            </div>
          </div>

          {/* ===== 生成表单区 ===== */}
          <form onSubmit={handleGenerate}>
            <div style={{ ...solidCard, padding: '28px 32px', marginBottom: 28 }}>
              <Row gutter={[24, 24]}>
                <Col xs={24} md={12}>
                  <Space direction="vertical" size={8} style={{ width: '100%' }}>
                    <Text style={{ fontSize: 14, fontWeight: 600, color: '#0f172a' }}>目标行业</Text>
                    <Select
                      value={form.industryCode || undefined}
                      onChange={(value) => setForm((current) => ({ ...current, industryCode: value }))}
                      placeholder="请选择行业"
                      size="large"
                      style={{ width: '100%', borderRadius: 14 }}
                      options={(industriesQuery.data || []).map((industry) => ({ label: industry.name, value: industry.code }))}
                    />
                  </Space>
                </Col>
                <Col xs={24} md={12}>
                  <Space direction="vertical" size={8} style={{ width: '100%' }}>
                    <Text style={{ fontSize: 14, fontWeight: 600, color: '#0f172a' }}>候选数量</Text>
                    <Select
                      value={form.candidateCount}
                      onChange={(value) => setForm((current) => ({ ...current, candidateCount: value }))}
                      size="large"
                      style={{ width: '100%', borderRadius: 14 }}
                      options={[
                        { label: '6 张', value: '6' },
                        { label: '8 张', value: '8' },
                        { label: '12 张', value: '12' },
                        { label: '16 张', value: '16' },
                      ]}
                    />
                  </Space>
                </Col>
                <Col xs={24}>
                  <Space direction="vertical" size={8} style={{ width: '100%' }}>
                    <Text style={{ fontSize: 14, fontWeight: 600, color: '#0f172a' }}>岗位要求 / 清洗目标</Text>
                    <Input.TextArea
                      value={form.requirement}
                      onChange={(e) => setForm((current) => ({ ...current, requirement: e.target.value }))}
                      placeholder="例如：生成 Go 后端高级工程师面试题，重点覆盖并发、MySQL、Redis、微服务治理，结合真实项目经验，输出中高级难度。"
                      rows={4}
                      style={{ borderRadius: 14, fontSize: 14, resize: 'vertical' }}
                    />
                  </Space>
                </Col>
                <Col xs={24}>
                  <Space direction="vertical" size={8} style={{ width: '100%' }}>
                    <Text style={{ fontSize: 14, fontWeight: 600, color: '#0f172a' }}>智能体命令 / 自定义提示词</Text>
                    <Input.TextArea
                      value={form.agentPrompt}
                      onChange={(e) => setForm((current) => ({ ...current, agentPrompt: e.target.value }))}
                      placeholder="例如：参考 Go 语言核心特性生成 8 道互不重复的问答题，聚焦语言理解，不要项目题，不要八股套话。"
                      rows={3}
                      style={{ borderRadius: 14, fontSize: 14, resize: 'vertical' }}
                    />
                  </Space>
                </Col>
              </Row>

              {/* 策略开关 */}
              <div style={{ marginTop: 24, display: 'flex', gap: 32, flexWrap: 'wrap' }}>
                <label style={{ display: 'inline-flex', alignItems: 'center', gap: 10, cursor: 'pointer' }}>
                  <Switch
                    checked={form.includeScraped}
                    onChange={(checked) => setForm((current) => ({ ...current, includeScraped: checked }))}
                    style={{ backgroundColor: form.includeScraped ? '#3b82f6' : '#cbd5e1' }}
                  />
                  <Text style={{ fontSize: 14, fontWeight: 500 }}>抓取相关面经素材</Text>
                </label>
                <label style={{ display: 'inline-flex', alignItems: 'center', gap: 10, cursor: 'pointer' }}>
                  <Switch
                    checked={form.includeGenerated}
                    onChange={(checked) => setForm((current) => ({ ...current, includeGenerated: checked }))}
                    style={{ backgroundColor: form.includeGenerated ? '#3b82f6' : '#cbd5e1' }}
                  />
                  <Text style={{ fontSize: 14, fontWeight: 500 }}>调用大模型生成 / 改写</Text>
                </label>
              </div>

              {/* 来源选择 */}
              <div style={{ marginTop: 20 }}>
                <Text type="secondary" style={{ fontSize: 13, marginBottom: 12, display: 'block' }}>抓取来源</Text>
                <Space size={10} wrap>
                  {(sourcesQuery.data || []).map((source) => (
                    <Tag
                      key={source.name}
                      onClick={() => form.includeScraped && toggleSource(source.name)}
                      style={{
                        cursor: form.includeScraped ? 'pointer' : 'not-allowed',
                        borderRadius: 10,
                        padding: '6px 14px',
                        fontSize: 13,
                        border: form.sources.includes(source.name)
                          ? '1px solid #3b82f6'
                          : '1px solid #e2e8f0',
                        background: form.sources.includes(source.name) ? '#eff6ff' : '#f8fafc',
                        color: form.sources.includes(source.name) ? '#2563eb' : '#64748b',
                        opacity: form.includeScraped ? 1 : 0.5,
                        transition: 'all 0.2s ease',
                      }}
                    >
                      {form.sources.includes(source.name) && <CheckCircleOutlined style={{ marginRight: 6, fontSize: 12 }} />}
                      {source.label}
                    </Tag>
                  ))}
                </Space>
              </div>

              {/* 状态条 */}
              <div
                style={{
                  marginTop: 24,
                  padding: '16px 20px',
                  borderRadius: 16,
                  background: formError ? '#fef2f2' : '#f0fdf4',
                  border: formError ? '1px solid #fecaca' : '1px solid #bbf7d0',
                  display: 'flex',
                  justifyContent: 'space-between',
                  alignItems: 'center',
                  flexWrap: 'wrap',
                  gap: 14,
                }}
              >
                <Space direction="vertical" size={4}>
                  <Text strong style={{ fontSize: 14, color: formError ? '#991b1b' : '#166534' }}>流水线状态</Text>
                  <Text style={{ fontSize: 13, color: formError ? '#b91c1c' : '#15803d' }}>{formError || statusMessage}</Text>
                </Space>
                <Space size={10}>
                  {isStreaming && (
                    <Tag
                      color="processing"
                      style={{ borderRadius: 10, padding: '4px 12px', fontSize: 13 }}
                      icon={<LoadingOutlined />}
                    >
                      正在逐张生成
                    </Tag>
                  )}
                  <Button
                    icon={<StopOutlined />}
                    onClick={handleCancelStream}
                    disabled={!isStreaming}
                    style={{ borderRadius: 12, height: 40 }}
                  >
                    停止生成
                  </Button>
                  <Button
                    type="primary"
                    icon={isStreaming ? <LoadingOutlined /> : <ThunderboltOutlined />}
                    htmlType="submit"
                    disabled={Boolean(formError) || generateMutation.isPending || isStreaming || queueGenerateTaskMutation.isPending}
                    loading={generateMutation.isPending || isStreaming}
                    style={{
                      borderRadius: 12,
                      height: 40,
                      padding: '0 22px',
                      fontWeight: 600,
                      background: 'linear-gradient(135deg, #3b82f6, #2563eb)',
                      border: 'none',
                      boxShadow: '0 4px 12px rgba(37, 99, 235, 0.3)',
                    }}
                  >
                    生成候选题卡
                  </Button>
                  <Button
                    icon={<CloudUploadOutlined />}
                    onClick={handleQueueGenerateTask}
                    disabled={Boolean(formError) || generateMutation.isPending || isStreaming || queueGenerateTaskMutation.isPending}
                    loading={queueGenerateTaskMutation.isPending}
                    style={{ borderRadius: 12, height: 40, border: '1px solid #e2e8f0', background: '#f8fafc' }}
                  >
                    异步生成
                  </Button>
                </Space>
              </div>

              {/* 异步恢复 */}
              <div style={{ marginTop: 16, display: 'flex', gap: 12, alignItems: 'flex-end', flexWrap: 'wrap' }}>
                <Space direction="vertical" size={6} style={{ flex: 1, minWidth: 200 }}>
                  <Text type="secondary" style={{ fontSize: 13 }}>异步任务 ID</Text>
                  <Input
                    value={asyncGenerateTaskId}
                    onChange={(e) => setAsyncGenerateTaskId(e.target.value.replace(/[^\d]/g, ''))}
                    placeholder="输入题目流水线任务 ID"
                    size="large"
                    style={{ borderRadius: 14 }}
                  />
                </Space>
                <Button
                  icon={<ReloadOutlined />}
                  onClick={handleLoadGenerateTaskResult}
                  disabled={!asyncGenerateTaskId || loadGenerateTaskResultMutation.isPending}
                  loading={loadGenerateTaskResultMutation.isPending}
                  style={{ borderRadius: 12, height: 44, border: '1px solid #e2e8f0' }}
                >
                  恢复异步结果
                </Button>
              </div>
            </div>
          </form>

          {/* ===== 统计数字 ===== */}
          {stats && (
            <Row gutter={[20, 20]} style={{ marginBottom: 28 }}>
              {[
                { label: '搜索结果', value: stats.searched_count, color: '#6366f1', bg: '#eef2ff' },
                { label: '已抓取素材', value: stats.fetched_count, color: '#3b82f6', bg: '#eff6ff' },
                { label: 'AI 产出题卡', value: stats.generated_count, color: '#0ea5e9', bg: '#f0f9ff' },
                { label: '最终候选', value: stats.candidate_count, color: '#10b981', bg: '#f0fdf4' },
              ].map((item) => (
                <Col xs={12} md={6} key={item.label}>
                  <div
                    style={{
                      ...solidCard,
                      padding: '20px 22px',
                      display: 'flex',
                      alignItems: 'center',
                      gap: 14,
                    }}
                    onMouseEnter={(e) => {
                      e.currentTarget.style.transform = 'translateY(-3px)'
                      e.currentTarget.style.boxShadow = '0 8px 24px rgba(0,0,0,0.08)'
                    }}
                    onMouseLeave={(e) => {
                      e.currentTarget.style.transform = 'none'
                      e.currentTarget.style.boxShadow = solidCard.boxShadow as string
                    }}
                  >
                    <div
                      style={{
                        width: 40,
                        height: 40,
                        borderRadius: 12,
                        background: item.bg,
                        color: item.color,
                        display: 'flex',
                        alignItems: 'center',
                        justifyContent: 'center',
                        fontSize: 18,
                        fontWeight: 700,
                        flexShrink: 0,
                      }}
                    >
                      {item.value}
                    </div>
                    <Text type="secondary" style={{ fontSize: 13, fontWeight: 500 }}>{item.label}</Text>
                  </div>
                </Col>
              ))}
            </Row>
          )}

          {/* ===== 警告区 ===== */}
          {warnings.length > 0 && (
            <div
              style={{
                ...solidCard,
                padding: '20px 24px',
                marginBottom: 28,
                background: '#fffbeb',
                border: '1px solid #fef3c7',
              }}
            >
              <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 10 }}>
                <WarningOutlined style={{ color: '#f59e0b' }} />
                <Text strong style={{ color: '#92400e' }}>流水线警告</Text>
              </div>
              <Space direction="vertical" size={6} style={{ width: '100%' }}>
                {warnings.map((warning) => (
                  <Text key={warning} style={{ color: '#a16207', fontSize: 13, lineHeight: 1.7 }}>{warning}</Text>
                ))}
              </Space>
            </div>
          )}

          {/* ===== 调试信息 ===== */}
          {debugEntries.length > 0 && (
            <div style={{ ...solidCard, padding: '28px 32px', marginBottom: 28 }}>
              <Title level={5} style={{ margin: '0 0 16px', fontWeight: 700 }}>
                <CodeOutlined style={{ marginRight: 8, color: '#64748b' }} />
                原始输出调试
                <Text type="secondary" style={{ fontSize: 13, fontWeight: 400, marginLeft: 8 }}>共 {debugEntries.length} 次失败调用</Text>
              </Title>
              <Space direction="vertical" size={10} style={{ width: '100%' }}>
                {debugEntries.map((entry) => {
                  const isExpanded = expandedDebugId === entry.id
                  return (
                    <div
                      key={entry.id}
                      style={{
                        borderRadius: 14,
                        border: '1px solid #f1f5f9',
                        overflow: 'hidden',
                        transition: 'all 0.3s ease',
                      }}
                    >
                      <div
                        onClick={() => setExpandedDebugId(isExpanded ? null : entry.id)}
                        style={{
                          padding: '14px 18px',
                          background: isExpanded ? '#f8fafc' : '#ffffff',
                          cursor: 'pointer',
                          display: 'flex',
                          justifyContent: 'space-between',
                          alignItems: 'center',
                        }}
                      >
                        <Space size={8} wrap>
                          {entry.slotIndex > 0 && (
                            <Tag size="small" style={{ borderRadius: 6, margin: 0, background: '#e0e7ff', color: '#3730a3', border: 'none' }}>
                              第 {entry.slotIndex} 张
                            </Tag>
                          )}
                          {entry.retryIndex > 0 && (
                            <Tag size="small" style={{ borderRadius: 6, margin: 0, background: '#fef3c7', color: '#92400e', border: 'none' }}>
                              重试 {entry.retryIndex}
                            </Tag>
                          )}
                          <Tag size="small" style={{ borderRadius: 6, margin: 0, background: '#fee2e2', color: '#991b1b', border: 'none' }}>
                            {formatQuestionPipelineFailureStage(entry.failureStage)}
                          </Tag>
                          {entry.traceId && (
                            <Text type="secondary" style={{ fontSize: 12, fontFamily: 'monospace' }}>
                              {entry.traceId.slice(0, 16)}...
                            </Text>
                          )}
                        </Space>
                        {isExpanded ? <UpOutlined style={{ color: '#94a3b8' }} /> : <DownOutlined style={{ color: '#94a3b8' }} />}
                      </div>
                      {isExpanded && (
                        <div style={{ padding: '16px 18px', borderTop: '1px solid #f1f5f9' }}>
                          <Paragraph style={{ marginBottom: 12, fontSize: 13 }}>{entry.message}</Paragraph>
                          {(entry.repairAttempted || entry.supplementAttempted) && (
                            <Space size={16} style={{ marginBottom: 12 }}>
                              <Tag size="small" style={{ borderRadius: 6, background: entry.repairAttempted ? '#d1fae5' : '#f1f5f9', color: entry.repairAttempted ? '#065f46' : '#64748b', border: 'none' }}>
                                {entry.repairAttempted ? '已触发 JSON 修复' : '未触发 JSON 修复'}
                              </Tag>
                              <Tag size="small" style={{ borderRadius: 6, background: entry.supplementAttempted ? '#d1fae5' : '#f1f5f9', color: entry.supplementAttempted ? '#065f46' : '#64748b', border: 'none' }}>
                                {entry.supplementAttempted ? '已触发编程题补齐' : '未触发编程题补齐'}
                              </Tag>
                            </Space>
                          )}
                          {entry.candidateExcerpt && (
                            <pre
                              style={{
                                margin: '0 0 12px',
                                padding: 14,
                                borderRadius: 12,
                                background: '#0f172a',
                                color: '#e2e8f0',
                                fontSize: 12,
                                lineHeight: 1.6,
                                overflow: 'auto',
                                whiteSpace: 'pre-wrap',
                                wordBreak: 'break-word',
                              }}
                            >
                              {entry.candidateExcerpt}
                            </pre>
                          )}
                          <pre
                            style={{
                              margin: 0,
                              padding: 14,
                              borderRadius: 12,
                              background: '#f8fafc',
                              color: '#475569',
                              fontSize: 12,
                              lineHeight: 1.6,
                              overflow: 'auto',
                              whiteSpace: 'pre-wrap',
                              wordBreak: 'break-word',
                              border: '1px solid #f1f5f9',
                            }}
                          >
                            {entry.rawOutput || '当前事件未携带原始输出。'}
                          </pre>
                        </div>
                      )}
                    </div>
                  )
                })}
              </Space>
            </div>
          )}

          {/* ===== 结果操作栏 ===== */}
          {cards.length > 0 && (
            <div
              style={{
                ...glassCard,
                padding: '16px 24px',
                marginBottom: 28,
                display: 'flex',
                justifyContent: 'space-between',
                alignItems: 'center',
                flexWrap: 'wrap',
                gap: 14,
                position: 'sticky',
                top: 16,
                zIndex: 20,
              }}
            >
              <Space size={10}>
                <Button
                  size="small"
                  onClick={() => setAllCardsSelected(true)}
                  style={{ borderRadius: 10, border: '1px solid #e2e8f0' }}
                >
                  全选
                </Button>
                <Button
                  size="small"
                  onClick={() => setAllCardsSelected(false)}
                  style={{ borderRadius: 10, border: '1px solid #e2e8f0' }}
                >
                  全不选
                </Button>
                <Tag color="processing" style={{ borderRadius: 8, margin: 0 }}>已勾选 {selectedCount} 张</Tag>
              </Space>
              <Space size={10}>
                <Button
                  type="primary"
                  icon={<CheckCircleOutlined />}
                  onClick={handleImportSelected}
                  disabled={selectedCount === 0 || importMutation.isPending || queueImportTaskMutation.isPending}
                  loading={importMutation.isPending}
                  style={{
                    borderRadius: 12,
                    background: 'linear-gradient(135deg, #10b981, #059669)',
                    border: 'none',
                    boxShadow: '0 4px 12px rgba(16, 185, 129, 0.3)',
                  }}
                >
                  导入已选 {selectedCount} 张
                </Button>
                <Button
                  icon={<CloudSyncOutlined />}
                  onClick={handleQueueImportSelected}
                  disabled={selectedCount === 0 || importMutation.isPending || queueImportTaskMutation.isPending}
                  loading={queueImportTaskMutation.isPending}
                  style={{ borderRadius: 12, border: '1px solid #e2e8f0' }}
                >
                  异步入队
                </Button>
              </Space>
            </div>
          )}

          {/* ===== 题卡结果 ===== */}
          {cards.length === 0 ? (
            <div style={{ ...solidCard, padding: '48px 32px', textAlign: 'center' }}>
              <Empty
                image={Empty.PRESENTED_IMAGE_SIMPLE}
                description="还没有候选题卡"
              >
                <Paragraph type="secondary" style={{ maxWidth: 400, margin: '0 auto' }}>
                  先填写岗位要求并执行一次流水线，结果会以卡片形式展示在这里。
                </Paragraph>
              </Empty>
            </div>
          ) : (
            <Row gutter={[20, 20]}>
              {cards.map((card) => {
                const isFresh = freshCardIds.includes(card.id)
                return (
                  <Col xs={24} md={12} key={card.id}>
                    <div
                      style={{
                        ...solidCard,
                        padding: '24px 28px',
                        border: card.selected
                          ? '2px solid #3b82f6'
                          : isFresh
                          ? '2px solid rgba(16, 185, 129, 0.6)'
                          : '1px solid #f1f5f9',
                        background: card.selected ? '#fafdff' : isFresh ? '#f6fffa' : '#ffffff',
                        boxShadow: card.selected
                          ? '0 4px 16px rgba(59, 130, 246, 0.12)'
                          : isFresh
                          ? '0 8px 24px rgba(16, 185, 129, 0.15)'
                          : solidCard.boxShadow as string,
                        transform: isFresh ? 'translateY(-3px)' : 'none',
                        animation: isFresh ? 'cardFreshEnter 0.5s ease' : 'none',
                        position: 'relative',
                        overflow: 'hidden',
                      }}
                    >
                      {/* 左侧选中指示条 */}
                      {card.selected && (
                        <div
                          style={{
                            position: 'absolute',
                            top: 0,
                            left: 0,
                            width: 4,
                            height: '100%',
                            background: 'linear-gradient(180deg, #3b82f6, #2563eb)',
                            borderRadius: '2px 0 0 2px',
                          }}
                        />
                      )}

                      {/* 卡片头部 */}
                      <div
                        style={{
                          display: 'flex',
                          justifyContent: 'space-between',
                          alignItems: 'flex-start',
                          marginBottom: 16,
                        }}
                      >
                        <label style={{ display: 'flex', alignItems: 'center', gap: 10, cursor: 'pointer' }}>
                          <input
                            type="checkbox"
                            checked={card.selected}
                            onChange={(event) => updateCardField(card.id, 'selected', event.target.checked)}
                            style={{ width: 18, height: 18, accentColor: '#3b82f6' }}
                          />
                          <Text strong style={{ fontSize: 14, color: '#0f172a' }}>加入题库</Text>
                        </label>
                        <Space size={6}>
                          <Tag
                            style={{
                              borderRadius: 8,
                              background: card.source_type === 'generated' ? '#eff6ff' : '#f0fdf4',
                              color: card.source_type === 'generated' ? '#1d4ed8' : '#15803d',
                              border: 'none',
                              fontSize: 12,
                            }}
                          >
                            {sourceTypeLabel(card.source_type)}
                          </Tag>
                          <Tag
                            style={{
                              borderRadius: 8,
                              background: '#f8fafc',
                              color: '#64748b',
                              border: 'none',
                              fontSize: 12,
                            }}
                          >
                            {Math.round(card.confidence * 100)}% 置信度
                          </Tag>
                        </Space>
                      </div>

                      {/* Meta */}
                      <Space size={8} style={{ marginBottom: 12, flexWrap: 'wrap' }}>
                        <Tag size="small" style={{ borderRadius: 6, background: '#e0e7ff', color: '#3730a3', border: 'none' }}>
                          {card.source_label || '未标注来源'}
                        </Tag>
                        <Tag size="small" style={{ borderRadius: 6, background: '#f1f5f9', color: '#475569', border: 'none' }}>
                          {QUESTION_TYPE_OPTIONS.find((item) => item.value === card.type)?.label || card.type}
                        </Tag>
                        <Tag
                          size="small"
                          style={{
                            borderRadius: 6,
                            background:
                              card.difficulty === 'easy'
                                ? '#f0fdf4'
                                : card.difficulty === 'hard'
                                ? '#fef2f2'
                                : '#fffbeb',
                            color:
                              card.difficulty === 'easy'
                                ? '#15803d'
                                : card.difficulty === 'hard'
                                ? '#dc2626'
                                : '#d97706',
                            border: 'none',
                          }}
                        >
                          {QUESTION_DIFFICULTY_OPTIONS.find((item) => item.value === card.difficulty)?.label || card.difficulty}
                        </Tag>
                      </Space>

                      {/* 来源标题/链接 */}
                      {card.source_title && (
                        <Text style={{ fontSize: 13, color: '#475569', marginBottom: 8, display: 'block' }}>
                          素材：{card.source_title}
                        </Text>
                      )}
                      {card.source_url && (
                        <a
                          href={card.source_url}
                          target="_blank"
                          rel="noreferrer"
                          style={{ fontSize: 13, color: '#2563eb', textDecoration: 'none', display: 'inline-flex', alignItems: 'center', gap: 4, marginBottom: 14 }}
                        >
                          <LinkOutlined style={{ fontSize: 11 }} />
                          查看原始来源
                        </a>
                      )}

                      {/* 题目标题 */}
                      <Space direction="vertical" size={6} style={{ width: '100%', marginBottom: 14 }}>
                        <Text style={{ fontSize: 13, fontWeight: 600, color: '#0f172a' }}>题目标题</Text>
                        <Input
                          value={card.title}
                          onChange={(e) => updateCardField(card.id, 'title', e.target.value)}
                          size="large"
                          style={{ borderRadius: 12 }}
                        />
                      </Space>

                      {/* 题型/难度/分类 */}
                      <Row gutter={[12, 12]} style={{ marginBottom: 14 }}>
                        <Col span={8}>
                          <Space direction="vertical" size={6} style={{ width: '100%' }}>
                            <Text style={{ fontSize: 12, fontWeight: 600, color: '#64748b' }}>题型</Text>
                            <Select
                              value={card.type}
                              onChange={(value) => handleCardTypeChange(card.id, value as QuestionType)}
                              size="large"
                              style={{ width: '100%', borderRadius: 12 }}
                              options={QUESTION_TYPE_OPTIONS}
                            />
                          </Space>
                        </Col>
                        <Col span={8}>
                          <Space direction="vertical" size={6} style={{ width: '100%' }}>
                            <Text style={{ fontSize: 12, fontWeight: 600, color: '#64748b' }}>难度</Text>
                            <Select
                              value={card.difficulty}
                              onChange={(value) => updateCardField(card.id, 'difficulty', value as QuestionDifficulty)}
                              size="large"
                              style={{ width: '100%', borderRadius: 12 }}
                              options={QUESTION_DIFFICULTY_OPTIONS}
                            />
                          </Space>
                        </Col>
                        <Col span={8}>
                          <Space direction="vertical" size={6} style={{ width: '100%' }}>
                            <Text style={{ fontSize: 12, fontWeight: 600, color: '#64748b' }}>分类</Text>
                            <Select
                              value={card.category || undefined}
                              onChange={(value) => updateCardField(card.id, 'category', value)}
                              placeholder="选择分类"
                              size="large"
                              style={{ width: '100%', borderRadius: 12 }}
                              options={categoryOptions.map((option) => ({ label: option.name, value: option.name }))}
                            />
                          </Space>
                        </Col>
                      </Row>

                      {/* 题目内容 */}
                      <Space direction="vertical" size={6} style={{ width: '100%', marginBottom: 14 }}>
                        <Text style={{ fontSize: 12, fontWeight: 600, color: '#64748b' }}>题目内容</Text>
                        <Input.TextArea
                          value={card.content}
                          onChange={(e) => updateCardField(card.id, 'content', e.target.value)}
                          rows={3}
                          style={{ borderRadius: 12, fontSize: 13, resize: 'vertical' }}
                        />
                      </Space>

                      {/* 答案 */}
                      <Space direction="vertical" size={6} style={{ width: '100%', marginBottom: 14 }}>
                        <Text style={{ fontSize: 12, fontWeight: 600, color: '#64748b' }}>{card.type === 'code' ? '代码参考答案' : '标准答案'}</Text>
                        <Input.TextArea
                          value={card.answer}
                          onChange={(e) => updateCardField(card.id, 'answer', e.target.value)}
                          rows={2}
                          style={{ borderRadius: 12, fontSize: 13, resize: 'vertical' }}
                        />
                      </Space>

                      {/* 代码思路（仅编程题） */}
                      {card.type === 'code' && (
                        <Space direction="vertical" size={6} style={{ width: '100%', marginBottom: 14 }}>
                          <Text style={{ fontSize: 12, fontWeight: 600, color: '#64748b' }}>代码思路解析</Text>
                          <Input.TextArea
                            value={card.solution}
                            onChange={(e) => updateCardField(card.id, 'solution', e.target.value)}
                            rows={2}
                            style={{ borderRadius: 12, fontSize: 13, resize: 'vertical' }}
                          />
                        </Space>
                      )}

                      {/* 解析 */}
                      <Space direction="vertical" size={6} style={{ width: '100%', marginBottom: 14 }}>
                        <Text style={{ fontSize: 12, fontWeight: 600, color: '#64748b' }}>{card.type === 'code' ? '考察意图 / 补充说明' : '解析'}</Text>
                        <Input.TextArea
                          value={card.explanation}
                          onChange={(e) => updateCardField(card.id, 'explanation', e.target.value)}
                          rows={2}
                          style={{ borderRadius: 12, fontSize: 13, resize: 'vertical' }}
                        />
                      </Space>

                      {/* 标签 */}
                      <Space direction="vertical" size={6} style={{ width: '100%', marginBottom: 14 }}>
                        <Text style={{ fontSize: 12, fontWeight: 600, color: '#64748b' }}><TagOutlined style={{ marginRight: 4 }} />标签</Text>
                        <Input
                          value={card.tagsText}
                          onChange={(e) => updateCardField(card.id, 'tagsText', e.target.value)}
                          placeholder="用逗号分隔多个标签"
                          size="large"
                          style={{ borderRadius: 12 }}
                        />
                      </Space>

                      {/* 判题配置（仅编程题） */}
                      {card.type === 'code' && (
                        <Space direction="vertical" size={6} style={{ width: '100%' }}>
                          <Text style={{ fontSize: 12, fontWeight: 600, color: '#64748b' }}><CodeOutlined style={{ marginRight: 4 }} />判题配置 judge_config</Text>
                          <Input.TextArea
                            value={card.judgeConfigText}
                            onChange={(e) => updateCardField(card.id, 'judgeConfigText', e.target.value)}
                            rows={6}
                            style={{
                              borderRadius: 12,
                              fontSize: 12,
                              fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Consolas, monospace',
                              background: '#f8fafc',
                              resize: 'vertical',
                            }}
                          />
                        </Space>
                      )}
                    </div>
                  </Col>
                )
              })}
            </Row>
          )}
        </div>

        {/* CSS 动画 */}
        <style>{`
          @keyframes cardFreshEnter {
            0% {
              opacity: 0;
              transform: translateY(16px) scale(0.97);
            }
            60% {
              opacity: 1;
              transform: translateY(-4px) scale(1.01);
            }
            100% {
              opacity: 1;
              transform: translateY(-3px) scale(1);
            }
          }
        `}</style>
      </div>
    </ConfigProvider>
  )
}
