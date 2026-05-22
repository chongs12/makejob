import type { Live2DDirective } from '../../shared/live2dDirective'

export interface InterviewConfigForm {
  difficulty: string
  questionCount: string
  topicsText: string
  live2dModelKey: string
}

export interface InterviewQuestion {
  question: string
  topic: string
  difficulty: string
  type: string
  hints?: string
  language?: string
  starter_code?: string
  editor_mode?: string
  evaluation_mode?: string
  live2d_directive?: Live2DDirective | null
}

export interface InterviewFeedback {
  score: number
  is_correct: boolean
  feedback: string
  key_points: string[]
  suggestions: string
  follow_up: string
}

export interface InterviewReport {
  overall_score: number
  total_questions: number
  correct_count: number
  dimension_scores: Record<string, number>
  strengths: string[]
  weaknesses: string[]
  suggestions: string[]
  summary: string
  coding_diagnostics?: InterviewCodingDiagnosis[]
}

export interface InterviewCodingDiagnosis {
  question_index: number
  language: string
  score: number
  mistake_tags: string[]
  strength_tags: string[]
  evidence: string[]
  suggestions: string[]
  process_summary: string
}

export interface InterviewCreatePayload {
  industry_code: string
  difficulty: string
  topics: string[]
  question_count: number
  live2d_model_key?: string
}

export interface InterviewCreateResponse {
  interview_id: number
  status: string
  first_question?: InterviewQuestion | null
  created_at: string
}

export interface InterviewHistoryItem {
  id: number
  status: string
  score: number
  total_questions: number
  started_at?: string
  ended_at?: string
  created_at?: string
}

export interface InterviewMessage {
  role: string
  content: string
  message_type: string
  question?: InterviewQuestion | null
  created_at: string
}

export interface InterviewDetailResponse {
  id: number
  industry_code: string
  status: string
  score: number
  total_questions: number
  messages: InterviewMessage[]
  current_question?: InterviewQuestion | null
  started_at?: string
  ended_at?: string
}

export interface InterviewCodingProcessEvent {
  type: string
  timestamp_ms: number
  payload?: Record<string, unknown>
}

export interface InterviewAnswerResponse {
  feedback?: InterviewFeedback | null
  next_question?: InterviewQuestion | null
  is_finished: boolean
}

export interface InterviewNextQuestionResponse {
  question?: InterviewQuestion | null
  question_no: number
  is_last: boolean
}

export interface InterviewReportResponse {
  interview_id: number
  report?: InterviewReport | null
  duration_seconds: number
  completed_at: string
}

export interface PageResult<T> {
  list: T[]
  total: number
  page: number
  page_size: number
}

export type InterviewSocketEventType =
  | 'connected'
  | 'session_ready'
  | 'interview_state'
  | 'user_answer'
  | 'ai_question'
  | 'asr_partial'
  | 'asr_final'
  | 'tts_audio'
  | 'live2d_expression'
  | 'audio_start'
  | 'audio_chunk'
  | 'audio_end'
  | 'error'
  | 'finished'

export interface InterviewSocketEvent<T = Record<string, unknown>> {
  type: InterviewSocketEventType
  content?: string
  data?: T
  timestamp?: number
  trace_id?: string
  interview_id?: number
}

export interface InterviewSocketStatePayload {
  status: string
  message: string
}

export interface InterviewSocketQuestionPayload {
  question: string
  question_no: number
  type: string
  hints?: string
  language?: string
  starter_code?: string
  editor_mode?: string
  evaluation_mode?: string
  live2d_directive?: Live2DDirective | null
}

export interface InterviewSocketASRPayload {
  text: string
  is_final: boolean
  confidence: number
}

export interface InterviewSocketTTSPayload {
  kind: string
  text: string
  audio_url: string
  duration: number
  format: string
  sample_rate: number
}

export interface InterviewSocketExpressionPayload {
  emotion: string
  action: string
  source: string
  expression_mix?: Live2DDirective['expression_mix']
  parameter_overrides?: Live2DDirective['parameter_overrides']
  intensity?: number
  duration_ms?: number
  mouth_open?: number
}
