import { requestJson } from '@makejob/api-client'
import { isSuccessCode, type ApiEnvelope } from '@makejob/shared-types'

export interface CategoryNode {
  id: number
  name: string
  children?: CategoryNode[]
}

export interface PracticeQuestion {
  id: number
  title: string
  difficulty: string
  type: string
  category_id: number
  industry_id: number
  category_name?: string
  pass_rate?: number
  is_favorite?: boolean
  is_answered?: boolean
  tags?: string
}

/**
 * 将 API 返回的题目数据标准化，确保布尔字段有默认值。
 */
export function normalizePracticeQuestion(question: PracticeQuestion): PracticeQuestion {
  return {
    ...question,
    is_favorite: question.is_favorite ?? false,
    is_answered: question.is_answered ?? false,
  }
}

export interface PracticeQuestionSetPreview {
  id: number
  title: string
  type: string
  difficulty: string
}

export interface PracticeQuestionSetSummary {
  slug: string
  title: string
  description: string
  focus_tags: string[]
  question_count: number
  questions: PracticeQuestionSetPreview[]
}

export interface PracticeQuestionSetDetail extends PracticeQuestionSetSummary {}

export interface PracticeStats {
  total_answered: number
  correct_count: number
  wrong_count: number
  accuracy_rate: number
  today_count: number
  streak_days: number
}

export interface GeneratedExamQuestion {
  id: number
  type: string
}

export interface GeneratedExamResponse {
  exam_id: string
  time_limit: number
  questions: GeneratedExamQuestion[]
}

interface PageResult<T> {
  list: T[]
  total: number
  page: number
  page_size: number
}

export const PRACTICE_PAGE_SIZE = 10

/**
 * 将树形分类拍平成选项列表，便于题库页直接挂接筛选器。
 */
export function flattenCategories(nodes: CategoryNode[], level = 0): Array<{ id: number; name: string }> {
  if (!Array.isArray(nodes)) {
    return []
  }
  return nodes.flatMap((node) => [
    { id: node.id, name: `${'　'.repeat(level)}${node.name}` },
    ...flattenCategories(node.children || [], level + 1),
  ])
}

/**
 * 将题目难度转换成中文标签，减少练习场景中的阅读成本。
 */
export function difficultyLabel(difficulty: string): string {
  const map: Record<string, string> = {
    easy: '简单',
    medium: '中等',
    hard: '困难',
  }

  return map[difficulty] || difficulty || '未知'
}

/**
 * 将题型编码转换成统一中文文案，避免各页各自维护显示口径。
 */
export function questionTypeLabel(type: string): string {
  const map: Record<string, string> = {
    choice: '单选题',
    multi: '多选题',
    code: '编程题',
    subjective: '主观题',
  }

  return map[type] || type || '未知题型'
}

/**
 * 根据题型选择最合适的练习入口，编程题直接进入编辑器。
 */
export function resolvePracticeTarget(questionId: number | string, questionType: string): string {
  if (questionType === 'code') {
    return `/practice/editor/${questionId}`
  }

  return `/practice/${questionId}`
}

/**
 * 拉取题库分类树，供题库页筛选器和其他练习入口共用。
 */
export async function fetchCategories(industryCode: string): Promise<CategoryNode[]> {
  const params = new URLSearchParams({
    industry_code: industryCode.trim(),
  })
  const response = await requestJson<ApiEnvelope<CategoryNode[]>>(`/categories?${params.toString()}`)
  if (!isSuccessCode(response.code)) {
    throw new Error(response.message || '获取分类失败')
  }

  return response.data || []
}

/**
 * 拉取题目分页列表，并将题库筛选条件统一映射到后端查询参数。
 */
export async function fetchQuestions(params: {
  page: number
  pageSize: number
  difficulty: string
  keyword: string
  industryId: number | null
  categoryId: number | null
  token?: string | null
}): Promise<PageResult<PracticeQuestion>> {
  const searchParams = new URLSearchParams({
    page: String(params.page),
    page_size: String(params.pageSize),
  })

  if (params.difficulty) {
    searchParams.set('difficulty', params.difficulty)
  }

  if (params.keyword) {
    searchParams.set('keyword', params.keyword)
  }

  if (params.industryId) {
    searchParams.set('industry_id', String(params.industryId))
  }

  if (params.categoryId) {
    searchParams.set('category_id', String(params.categoryId))
  }

  const response = await requestJson<ApiEnvelope<PageResult<PracticeQuestion>>>(`/questions?${searchParams.toString()}`, {
    token: params.token || undefined,
  })
  if (!isSuccessCode(response.code) || !response.data) {
    throw new Error(response.message || '获取题目列表失败')
  }

  return {
    ...response.data,
    list: (response.data.list || []).map(normalizePracticeQuestion),
  }
}

/**
 * 拉取当前行业下的核心题单摘要，供题库页快速进入高价值主题。
 */
export async function fetchQuestionSets(industryCode: string | null): Promise<PracticeQuestionSetSummary[]> {
  const params = new URLSearchParams()
  if (industryCode) {
    params.set('industry_code', industryCode)
  }

  const suffix = params.toString() ? `?${params.toString()}` : ''
  const response = await requestJson<ApiEnvelope<{ items: PracticeQuestionSetSummary[]; list?: PracticeQuestionSetSummary[] }>>(`/question-sets${suffix}`)
  if (!isSuccessCode(response.code)) {
    throw new Error(response.message || '获取核心题单失败')
  }

  return response.data?.items || response.data?.list || []
}

/**
 * 拉取指定核心题单的完整题目集合，供题库页稳定承接专题补练入口。
 */
export async function fetchQuestionSetDetail(industryCode: string | null, slug: string): Promise<PracticeQuestionSetDetail> {
  const normalizedSlug = slug.trim()
  if (!normalizedSlug) {
    throw new Error('题单不能为空')
  }

  const params = new URLSearchParams()
  if (industryCode) {
    params.set('industry_code', industryCode)
  }

  const suffix = params.toString() ? `?${params.toString()}` : ''
  const response = await requestJson<ApiEnvelope<PracticeQuestionSetDetail>>(`/question-sets/${normalizedSlug}${suffix}`)
  if (!isSuccessCode(response.code) || !response.data) {
    throw new Error(response.message || '获取题单详情失败')
  }

  return response.data
}

/**
 * 拉取当前用户的练习统计，为练习首页和首页概览提供统一数据来源。
 */
export async function fetchPracticeStats(token: string): Promise<PracticeStats> {
  const response = await requestJson<ApiEnvelope<PracticeStats>>('/user/practice-stats', {
    token,
  })

  if (!isSuccessCode(response.code) || !response.data) {
    throw new Error(response.message || '获取练习统计失败')
  }

  return response.data
}

/**
 * 调用随机练习或限时模拟接口，并返回首个题目的跳转信息。
 */
export async function generateExamRequest(params: {
  token: string
  mode: 'random' | 'timed'
  difficulty: string
  industryId: number | null
  categoryId: number | null
}): Promise<GeneratedExamResponse> {
  const endpoint = params.mode === 'timed' ? '/exams/timed' : '/exams/random'
  const body = params.mode === 'timed'
    ? {
        count: 5,
        difficulty: params.difficulty || 'medium',
        industry_id: params.industryId || undefined,
        category_id: params.categoryId || undefined,
        time_limit_minutes: 30,
      }
    : {
        count: 5,
        difficulty: params.difficulty || 'medium',
        industry_id: params.industryId || undefined,
        category_id: params.categoryId || undefined,
      }

  const response = await requestJson<ApiEnvelope<GeneratedExamResponse>>(endpoint, {
    method: 'POST',
    token: params.token,
    body,
  })

  if (!isSuccessCode(response.code) || !response.data) {
    throw new Error(response.message || '生成练习失败')
  }

  return response.data
}
