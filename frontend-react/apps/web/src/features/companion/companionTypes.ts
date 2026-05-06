export type CompanionMessageRole = 'assistant' | 'user'
export type CompanionTaskActionSource = 'hub' | 'room'
export type CompanionTaskStatus = 'pending' | 'in_progress' | 'completed' | 'skipped'

export interface CompanionPlanTask {
  id: number
  title: string
  description: string
  task_type: string
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

export interface CompanionPlanDetail {
  id: number
  industry_id?: number
  industry_code?: string
  title: string
  description: string
  status: string
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
  createdAt: number
}

export interface CompanionChatReply {
  content?: string
  reply?: string
  emotion?: string
  mood?: string
  action?: string
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
  source: string
  match_type: string
  is_generic: boolean
  is_recommended: boolean
}
