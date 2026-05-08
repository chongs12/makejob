import { requestJson } from '@makejob/api-client'
import { isSuccessCode, type ApiEnvelope } from '@makejob/shared-types'

export interface WeeklyFocusTheme {
  title: string
  reason: string
  source: string
  source_label: string
  focus_tags: string[]
  topic_codes: string[]
  related_question_sets: string[]
  dominant_archive_phase?: string
  dominant_archive_phase_label?: string
  occurrence_count: number
  interview_occurrence_count: number
  suggestions: string[]
}

export interface WeeklyFocusResponse {
  themes: WeeklyFocusTheme[]
}

/**
 * 拉取当前用户本周最值得优先补强的主题摘要。
 */
export async function fetchWeeklyFocus(token: string): Promise<WeeklyFocusResponse> {
  const response = await requestJson<ApiEnvelope<WeeklyFocusResponse>>('/user/weekly-focus', {
    token,
  })

  if (!isSuccessCode(response.code) || !response.data) {
    throw new Error(response.message || '获取本周补强主题失败')
  }

  return response.data
}
