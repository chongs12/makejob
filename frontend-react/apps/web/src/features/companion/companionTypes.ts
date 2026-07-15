import type { Live2DDirective } from '../../shared/live2dDirective'

export type CompanionMessageRole = 'assistant' | 'user'
export type CompanionTaskActionSource = 'hub' | 'room'
export type CompanionTaskStatus = 'pending' | 'in_progress' | 'completed' | 'skipped'
export type CompanionFeedbackTrainingType = 'coding' | 'choice' | 'short_answer' | 'generic'
export type CompanionFeedbackDifficulty = '' | 'too_easy' | 'just_right' | 'too_hard'

export interface CompanionPlanTask {
  id: number
  title: string
  description: string
  task_type: string
  phase?: string
  phase_goal?: string
  status: string
  due_date?: string
  completed_at?: string
  day_number: number
  sort_order?: number
  source?: string
  source_label?: string
  reason?: string
  priority_explanation?: string
  source_ref?: string
  collection_hint?: string
}

export interface CompanionTaskFeedbackDraft {
  trainingType: CompanionFeedbackTrainingType
  attemptCount: number
  timeSpentMinutes: number
  difficultySelfAssessment: CompanionFeedbackDifficulty
  mistakeTagsText: string
  summary: string
}

export interface CompanionTaskFeedbackPayload {
  training_type: CompanionFeedbackTrainingType
  question_id?: number
  mistake_tags: string[]
  attempt_count: number
  time_spent_seconds: number
  difficulty_self_assessment?: Exclude<CompanionFeedbackDifficulty, ''>
  summary: string
}

export interface CompanionPhaseBlueprintSummaryEntry {
  phase: string
  phase_goal: string
  start_day: number
  end_day: number
}

export interface CompanionPlanDetail {
  id: number
  industry_id?: number
  industry_code?: string
  title: string
  description: string
  phase?: string
  phase_goal?: string
  entry_phase?: string
  adjustment_summaries?: string[]
  adjustment_reason_codes?: string[]
  phase_blueprint_summary?: CompanionPhaseBlueprintSummaryEntry[]
  status: string
  async_task_id?: number
  task_status?: string
  task_error?: string
  total_tasks: number
  completed_tasks: number
  progress: number
  start_date?: string
  end_date?: string
  tasks: CompanionPlanTask[]
  created_at?: string
}

export interface CompanionHistoryItem {
  id: string
  role: CompanionMessageRole
  content: string
  emotion?: string
  action?: string
  live2dDirective?: Live2DDirective | null
  createdAt: number
}

// SuggestedAction 结构化引导动作，前端按 type 渲染跳转 chip。
export interface SuggestedAction {
  type: string
  target?: string
  params?: string
}

// InlineTrigger 字幕行内可点击关键词。
export interface InlineTrigger {
  keyword: string
  action_type: string  // practice | interview | adjust_plan
  target: string
  position_hint?: string  // head | middle | tail
}

// IntentInfo LLM 意图识别结果。
export interface IntentInfo {
  type: string       // practice | adjust_plan | interview | chat
  confidence: number // 0-1
  stage: string      // collecting_info | ready_to_execute | none
}

// PendingAction 待执行动作，ready=true 时前端自动触发。
export interface PendingAction {
  type: string                // adjust_plan | practice 等
  ready: boolean
  params: Record<string, string>
  missing_info: string[]
}

// ConversationState 多轮对话状态跟踪。
export interface ConversationState {
  phase: string                       // greeting | collecting | ready | executing
  collected_params: Record<string, string>
}

export interface CompanionChatReply {
  content?: string
  reply?: string
  emotion?: string
  mood?: string
  action?: string
  audio_url?: string
  audio_duration?: number
  audio_format?: string
  audio_sample_rate?: number
  live2d_directive?: Live2DDirective | null
  suggestions?: string[]
  suggested_actions?: SuggestedAction[]
  inline_triggers?: InlineTrigger[]
  intent?: IntentInfo | null
  pending_action?: PendingAction | null
  conversation_state?: ConversationState | null
}

export interface CompanionSessionSummary {
  updatedAt: number
  latestAssistantReply: string
  latestUserMessage: string
  planTitle: string
  progress: number
}

export interface CompanionFocusTaskDraft {
  planId: number
  taskId: number
  title: string
  status: string
  source: CompanionTaskActionSource
  updatedAt: number
}

export interface CompanionDailyDigest {
  dateKey: string
  updatedAt: number
  completedTitles: string[]
  skippedTitles: string[]
  latestActionText: string
}

export interface CompanionStudyLogPayload {
  date_key: string
  plan_id?: number
  summary: string
  focus_task_title: string
  completed_count: number
  skipped_count: number
  completed_titles: string[]
  skipped_titles: string[]
  latest_action_text: string
}

export interface CompanionPlanProgress {
  plan_id: number
  total_tasks: number
  completed_tasks: number
  skipped_tasks: number
  in_progress_tasks: number
  pending_tasks: number
  progress: number
}

export interface CompanionAdjustmentPreviewTask {
  task_id: number
  title: string
  description: string
  task_type: string
  phase: string
  phase_goal: string
  duration_minutes: number
  priority: string
  status: string
  sort_order: number
  source: string
  source_label: string
  reason: string
  is_new: boolean
}

export interface CompanionAdjustmentPreviewRemoval {
  task_id: number
  title: string
  phase: string
  sort_order: number
}

export interface CompanionAdjustmentPreviewReorder {
  task_id: number
  title: string
  phase: string
  from_sort_order: number
  to_sort_order: number
}

export interface CompanionAdjustmentPreview {
  preview_token: string
  tasks_added: number
  tasks_removed: number
  tasks_reordered: number
  adjustment_summary: string
  add: CompanionAdjustmentPreviewTask[]
  remove: CompanionAdjustmentPreviewRemoval[]
  reorder: CompanionAdjustmentPreviewReorder[]
  preview_tasks: CompanionAdjustmentPreviewTask[]
}

export interface CompanionCategoryNode {
  id: number
  name: string
  children?: CompanionCategoryNode[]
}

export interface CompanionCategoryOption {
  id: number
  name: string
}

export interface CompanionGeneratePlanForm {
  level: string
  dailyStudyTime: string
  durationDays: string
  goalDescription: string
  weakTopics: string[]
  weakTopicsText: string
}

export interface CompanionGeneratePlanPayload {
  level: string
  daily_study_time: number
  weak_topics: string[]
  goal_description: string
  duration_days: number
  industry_id?: number
  industry_code: string
}

export interface CompanionSelectableLive2DModel {
  key: string
  name: string
  scene: 'interview' | 'companion'
  model_url: string
  thumbnail_url: string
  config_json?: string
  source: string
  match_type: string
  is_generic: boolean
  is_recommended: boolean
  motions?: Array<{
    key: string
    group: string
    file: string
    label: string
  }>
}
