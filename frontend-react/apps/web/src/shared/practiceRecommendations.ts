import { requestJson } from '@makejob/api-client'
import { isSuccessCode, type ApiEnvelope } from '@makejob/shared-types'

export interface PracticeRecommendationQuestion {
  id: number
  title: string
  difficulty: string
  type: string
  category_id: number
  industry_id: number
  category_name?: string
  pass_rate?: number
  tags?: string
}

export interface PracticeRecommendationItem {
  question: PracticeRecommendationQuestion
  focus_tag: string
  topic_code?: string
  reason: string
  source_type: string
  priority: number
  occurrence_count: number
}

export interface PracticeRecommendationResponse {
  focus_tags: string[]
  items: PracticeRecommendationItem[]
}

/**
 * 拉取基于学习档案生成的对症练习推荐，支持按面试上下文缩小范围。
 */
export async function fetchPracticeRecommendations(
  token: string,
  limit = 6,
  interviewId?: number | null,
): Promise<PracticeRecommendationResponse> {
  const params = new URLSearchParams()
  params.set('limit', String(limit))
  if (interviewId && interviewId > 0) {
    params.set('interview_id', String(interviewId))
  }

  const response = await requestJson<ApiEnvelope<PracticeRecommendationResponse>>(`/user/practice-recommendations?${params.toString()}`, {
    token,
  })

  if (!isSuccessCode(response.code) || !response.data) {
    throw new Error(response.message || '获取练习推荐失败')
  }

  return response.data
}

/**
 * 根据题型返回推荐题目应跳转到的前台路由模板。
 */
export function resolvePracticeRecommendationRoute(questionType: string): '/practice/$questionId' | '/practice/editor/$questionId' {
  if (questionType === 'code') {
    return '/practice/editor/$questionId'
  }

  return '/practice/$questionId'
}
