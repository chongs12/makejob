import { requestJson } from '@makejob/api-client'
import { isSuccessCode, type ApiEnvelope } from '@makejob/shared-types'
import type {
  InterviewAnswerResponse,
  InterviewCodingProcessEvent,
  InterviewCreatePayload,
  InterviewCreateResponse,
  InterviewDetailResponse,
  InterviewHistoryItem,
  InterviewNextQuestionResponse,
  InterviewReportResponse,
  PageResult,
} from './interviewTypes'

/**
 * 统一校验面试相关接口响应，避免每个调用点重复写同样的判空与报错逻辑。
 */
function unwrapInterviewResponseData<T>(response: ApiEnvelope<T>, fallbackMessage: string): T {
  if (!isSuccessCode(response.code) || !response.data) {
    throw new Error(response.message || fallbackMessage)
  }

  return response.data
}

/**
 * 拉取用户面试历史，为入口页提供最近记录和继续入口。
 */
export async function fetchInterviewHistory(token: string): Promise<PageResult<InterviewHistoryItem>> {
  const response = await requestJson<ApiEnvelope<PageResult<InterviewHistoryItem>>>('/interviews?page=1&page_size=6', {
    token,
  })

  return unwrapInterviewResponseData(response, '获取面试历史失败')
}

/**
 * 创建新的 AI 面试会话，并返回首题信息。
 */
export async function createInterviewRequest(token: string, payload: InterviewCreatePayload): Promise<InterviewCreateResponse> {
  const response = await requestJson<ApiEnvelope<InterviewCreateResponse>>('/interviews', {
    method: 'POST',
    token,
    body: payload,
  })

  return unwrapInterviewResponseData(response, '创建面试失败')
}

/**
 * 拉取面试详情，用于进行页恢复当前对话与状态。
 */
export async function fetchInterviewDetail(token: string, interviewId: string): Promise<InterviewDetailResponse> {
  const response = await requestJson<ApiEnvelope<InterviewDetailResponse>>(`/interviews/${interviewId}`, {
    token,
  })

  return unwrapInterviewResponseData(response, '获取面试详情失败')
}

export interface SubmitInterviewAnswerPayload {
  answer: string
  final_code?: string
  language?: string
  question_type?: string
  process_events?: InterviewCodingProcessEvent[]
}

/**
 * 提交当前问题的回答，并在后端直接推进到下一题。
 */
export async function submitInterviewAnswer(
  token: string,
  interviewId: string,
  payload: SubmitInterviewAnswerPayload,
): Promise<InterviewAnswerResponse> {
  const response = await requestJson<ApiEnvelope<InterviewAnswerResponse>>(`/interviews/${interviewId}/answer`, {
    method: 'POST',
    token,
    body: payload,
  })

  return unwrapInterviewResponseData(response, '提交回答失败')
}

/**
 * 手动获取下一题，作为自动推进失败时的恢复入口。
 */
export async function fetchNextInterviewQuestion(token: string, interviewId: string): Promise<InterviewNextQuestionResponse> {
  const response = await requestJson<ApiEnvelope<InterviewNextQuestionResponse>>(`/interviews/${interviewId}/next-question`, {
    method: 'POST',
    token,
  })

  return unwrapInterviewResponseData(response, '获取下一题失败')
}

/**
 * 结束当前面试并触发后端生成报告。
 */
export async function finishInterviewRequest(token: string, interviewId: string): Promise<InterviewReportResponse> {
  const response = await requestJson<ApiEnvelope<InterviewReportResponse>>(`/interviews/${interviewId}/finish`, {
    method: 'POST',
    token,
    body: {},
  })

  return unwrapInterviewResponseData(response, '结束面试失败')
}

/**
 * 获取已完成面试的报告详情。
 */
export async function fetchInterviewReport(token: string, interviewId: string): Promise<InterviewReportResponse> {
  const response = await requestJson<ApiEnvelope<InterviewReportResponse>>(`/interviews/${interviewId}/report`, {
    token,
  })

  return unwrapInterviewResponseData(response, '获取面试报告失败')
}
