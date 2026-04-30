import { requestJson } from '@makejob/api-client'
import { isSuccessCode, type ApiEnvelope } from '@makejob/shared-types'

export interface MistakeTopicCard {
  code: string
  tag: string
  title: string
  problem_pattern: string
  root_causes: string[]
  self_check_list: string[]
  practice_directions: string[]
  recommended_actions: string[]
  related_question_sets: string[]
}

export interface PracticeAnalysisPayload {
  is_correct?: boolean
  score?: number
  feedback?: string
  issues?: string[]
  improvements?: string[]
  mistake_tags?: string[]
  strength_tags?: string[]
  time_complexity?: string
  space_complexity?: string
}

/**
 * 批量拉取错因专题卡片，供成长页、面试报告页和练习反馈页共用。
 */
export async function fetchMistakeTopics(codes: string[]): Promise<MistakeTopicCard[]> {
  const normalizedCodes = Array.from(new Set(codes.map((item) => item.trim()).filter(Boolean)))
  const params = new URLSearchParams()
  if (normalizedCodes.length) {
    params.set('codes', normalizedCodes.join(','))
  }

  const suffix = params.toString() ? `?${params.toString()}` : ''
  const response = await requestJson<ApiEnvelope<MistakeTopicCard[]>>(`/mistake-topics${suffix}`)
  if (!isSuccessCode(response.code)) {
    throw new Error(response.message || '获取错因专题失败')
  }

  return response.data || []
}

/**
 * 获取单个错因专题详情，供专题页独立展示。
 */
export async function fetchMistakeTopic(code: string): Promise<MistakeTopicCard> {
  const response = await requestJson<ApiEnvelope<MistakeTopicCard>>(`/mistake-topics/${code}`)
  if (!isSuccessCode(response.code) || !response.data) {
    throw new Error(response.message || '获取错因专题详情失败')
  }

  return response.data
}

/**
 * 返回错因专题详情页路由模板。
 */
export function resolveMistakeTopicRoute(): '/practice/topics/$topicCode' {
  return '/practice/topics/$topicCode'
}

/**
 * 按错因标签从已加载的专题卡片中筛出匹配项，避免前端重复维护标签到编码的映射表。
 */
export function pickMistakeTopicsByTags(tags: string[], topics: MistakeTopicCard[]): MistakeTopicCard[] {
  const normalizedTags = Array.from(new Set(tags.map((item) => item.trim()).filter(Boolean)))
  return topics.filter((topic) => normalizedTags.includes(topic.tag))
}

/**
 * 解析练习判题返回中的 AI 分析 JSON，便于提取错因标签。
 */
export function parsePracticeAnalysis(raw: string | undefined): PracticeAnalysisPayload | null {
  const normalized = (raw || '').trim()
  if (!normalized) {
    return null
  }

  try {
    const parsed = JSON.parse(normalized) as PracticeAnalysisPayload
    return parsed && typeof parsed === 'object' ? parsed : null
  } catch {
    return null
  }
}
