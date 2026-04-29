export interface DashboardStats {
  total_users: number
  total_questions: number
  total_interviews: number
  today_active_users: number
  pro_members: number
  new_users_today: number
}

export interface AdminConfigItem {
  id?: number
  config_key: string
  config_value: string
  config_type: string
  description?: string
}

export interface AIConfigResponse {
  configs: Record<string, string>
  items: AdminConfigItem[]
  support: {
    primary_providers: string[]
    fallback_providers: string[]
    notes: string[]
  }
  warnings: string[]
}

export interface AICallLogItem {
  id: number
  created_at: string
  trace_id: string
  task_id?: number
  source: string
  scene: string
  provider: string
  model: string
  latency_ms: number
  model_error: string
  is_success: boolean
}

export interface AICallLogDetail extends AICallLogItem {
  updated_at: string
  industry_id?: number
  prompt_source: string
  selected_prompt_id?: number
  selected_prompt_name: string
  rendered_prompt: string
  request_messages: string
  runtime_config: string
  scene_config: string
  user_input: string
  model_output: string
}

export interface ScraperTaskItem {
  id: number
  created_at: string
  started_at?: string
  finished_at?: string
  task_type?: string
  source_url: string
  source_title: string
  source: string
  status: string
  question_count: number
  imported_count: number
  retry_count?: number
  error_msg?: string
}

export interface ScraperTaskDetail extends ScraperTaskItem {
  updated_at: string
  payload_json?: string
  result_json?: string
}

export interface PageResult<T> {
  list: T[]
  total: number
  page: number
  page_size: number
}
