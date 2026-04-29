export interface InterviewConfigForm {
  difficulty: string
  questionCount: string
  topicsText: string
}

export interface InterviewQuestion {
  question: string
  topic: string
  difficulty: string
  type: string
  hints?: string
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
}

export interface InterviewCreatePayload {
  industry_code: string
  difficulty: string
  topics: string[]
  question_count: number
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
  created_at: string
}

export interface InterviewDetailResponse {
  id: number
  industry_code: string
  status: string
  score: number
  total_questions: number
  messages: InterviewMessage[]
  started_at?: string
  ended_at?: string
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
}
