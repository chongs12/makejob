import type { Live2DDirective } from '../../shared/live2dDirective'

export interface InterviewConfigForm {
  difficulty: string
  questionCount: string
  topicsText: string
  live2dModelKey: string
  interviewMode: 'general' | 'resume_driven'
  interviewType: 'knowledge' | 'job'
  resumeText: string
  jobDescription: string
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
  report_template?: string
  report_data_json?: string
}

// 知识点专项面试报告结构化数据（对应后端 report_data_json，report_template === 'knowledge' 时使用）
export interface KnowledgeReportData {
  overall_score: number
  rating: string
  conclusion: string
  basic_info: {
    knowledge_topics: string[]
    question_type: string
    duration_seconds: number
    total_questions: number
    correct_count: number
    accuracy: number
  }
  question_reviews: Array<{
    question_index: number
    question: string
    user_answer: string
    score: number
    max_score: number
    errors: string[]
    omissions: string[]
    highlights: string[]
    standard_answer: string
    key_points: string[]
  }>
  dimension_scores: Array<{
    dimension: string
    score: number
    comment: string
  }>
  mastered_points: string[]
  blind_spots: Array<{
    topic: string
    level: string
    detail: string
  }>
  study_suggestions: Array<{
    focus: string
    detail: string
  }>
  next_quiz_topics: Array<{
    topic: string
    reason: string
  }>
}

// 岗位求职面试报告结构化数据（对应后端 report_data_json，report_template === 'job' 时使用）
export interface JobReportData {
  overall_score: number
  rating: string
  hire_recommendation: string
  basic_info: {
    candidate_name: string
    target_position: string
    interview_type: string
    duration_seconds: number
    total_questions: number
    overall_score: number
    rating: string
  }
  jd_match_overview: {
    matched_items: string[]
    missing_items: string[]
    hard_requirements_met: boolean
    resume_highlights: string[]
    resume_hard_wounds: string[]
  }
  question_reviews: Array<{
    question_index: number
    question: string
    user_answer: string
    score: number
    max_score: number
    highlights: string[]
    loopholes: string[]
    pitfalls: string[]
    taboos: string[]
  }>
  dimension_scores: Array<{
    dimension: string
    score: number
    weight: number
    comment: string
  }>
  core_advantages: string[]
  weaknesses_risks: Array<{
    item: string
    level: string // 致命 | 轻微
    impact: string
  }>
  hire_decision: {
    decision: string
    rationale: string
  }
  optimization_plan: Array<{
    aspect: string
    detail: string
  }>
  next_round_questions: Array<{
    question: string
    focus: string
    difficulty: string
  }>
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
  difficulty?: string
  topics?: string[]
  question_count?: number
  live2d_model_key?: string
  interview_mode?: 'general' | 'resume_driven'
  interview_type?: 'knowledge' | 'job'
  resume_text?: string
  job_description?: string
}

export interface InterviewCreateResponse {
  interview_id: number
  status: string
  async_task_id?: number
  task_status?: string
  task_error?: string
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
  async_task_id?: number
  task_status?: string
  task_error?: string
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
  status: string
  async_task_id?: number
  task_status?: string
  task_error?: string
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
  | 'assistant_transcript_partial'
  | 'assistant_transcript_final'
  | 'assistant_audio_chunk'
  | 'assistant_turn_finished'
  | 'barge_in'
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
  mode?: string
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

export interface InterviewSocketAssistantTranscriptPayload {
  text: string
  is_final: boolean
  question_id?: string
  reply_id?: string
}

export interface InterviewSocketAssistantAudioChunkPayload {
  audio_base64: string
  format: string
  sample_rate: number
}

export interface InterviewSocketAssistantTurnPayload {
  text: string
  question_no: number
  is_question: boolean
  live2d_directive?: Live2DDirective | null
}
