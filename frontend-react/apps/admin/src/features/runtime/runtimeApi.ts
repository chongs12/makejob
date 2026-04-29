import { requestJson } from '@makejob/api-client'
import { isSuccessCode, type ApiEnvelope } from '@makejob/shared-types'
import type {
  AIConfigResponse,
  AICallLogDetail,
  AICallLogItem,
  DashboardStats,
  PageResult,
  ScraperTaskDetail,
  ScraperTaskItem,
} from './runtimeTypes'

/**
 * 拉取后台基础统计数据，供运行总览页展示用户、题库与面试规模。
 */
export async function fetchDashboardStats(token: string | null): Promise<DashboardStats> {
  const response = await requestJson<ApiEnvelope<DashboardStats>>('/admin/dashboard', {
    method: 'GET',
    token,
  })

  if (!isSuccessCode(response.code) || !response.data) {
    throw new Error(response.message || '获取后台总览统计失败')
  }

  return response.data
}

/**
 * 拉取当前 AI 运行配置与 runtime 支持范围，供总览页和运行页复用。
 */
export async function fetchRuntimeAIConfig(token: string | null): Promise<AIConfigResponse> {
  const response = await requestJson<ApiEnvelope<AIConfigResponse>>('/admin/ai-configs', {
    method: 'GET',
    token,
  })

  if (!isSuccessCode(response.code) || !response.data) {
    throw new Error(response.message || '获取 AI 运行配置失败')
  }

  return response.data
}

/**
 * 按条件查询 AI 调用日志，供运行态问题排查与 trace 定位使用。
 */
export async function fetchAICallLogs(
  token: string | null,
  params: {
    page?: number
    pageSize?: number
    status?: string
    traceId?: string
    taskId?: string
  } = {},
): Promise<PageResult<AICallLogItem>> {
  const query = new URLSearchParams()
  query.set('page', String(params.page || 1))
  query.set('page_size', String(params.pageSize || 10))
  if (params.status?.trim()) {
    query.set('status', params.status.trim())
  }
  if (params.traceId?.trim()) {
    query.set('trace_id', params.traceId.trim())
  }
  if (params.taskId?.trim()) {
    query.set('task_id', params.taskId.trim())
  }

  const response = await requestJson<ApiEnvelope<PageResult<AICallLogItem>>>(`/admin/ai-call-logs?${query.toString()}`, {
    method: 'GET',
    token,
  })

  if (!isSuccessCode(response.code) || !response.data) {
    throw new Error(response.message || '获取 AI 调用日志失败')
  }

  return response.data
}

/**
 * 拉取单条 AI 调用日志详情，供任务页展开查看 prompt、消息与模型原始输出。
 */
export async function fetchAICallLogDetail(token: string | null, logId: number): Promise<AICallLogDetail> {
  const response = await requestJson<ApiEnvelope<AICallLogDetail>>(`/admin/ai-call-logs/${logId}`, {
    method: 'GET',
    token,
  })

  if (!isSuccessCode(response.code) || !response.data) {
    throw new Error(response.message || '获取 AI 调用日志详情失败')
  }

  return response.data
}

/**
 * 查询抓取任务历史，供管理端统一查看运行态任务结果。
 */
export async function fetchScraperTasks(
  token: string | null,
  params: {
    page?: number
    pageSize?: number
    status?: string
    taskType?: string
  } = {},
): Promise<PageResult<ScraperTaskItem>> {
  const query = new URLSearchParams()
  query.set('page', String(params.page || 1))
  query.set('page_size', String(params.pageSize || 10))
  if (params.status?.trim()) {
    query.set('status', params.status.trim())
  }
  if (params.taskType?.trim()) {
    query.set('task_type', params.taskType.trim())
  }

  const response = await requestJson<ApiEnvelope<PageResult<ScraperTaskItem>>>(`/admin/scraper/tasks?${query.toString()}`, {
    method: 'GET',
    token,
  })

  if (!isSuccessCode(response.code) || !response.data) {
    throw new Error(response.message || '获取抓取任务列表失败')
  }

  return response.data
}

/**
 * 拉取单条抓取任务详情，显式查看异步入队载荷和执行结果，方便后台排障。
 */
export async function fetchScraperTaskDetail(token: string | null, taskId: number): Promise<ScraperTaskDetail> {
  const response = await requestJson<ApiEnvelope<ScraperTaskDetail>>(`/admin/scraper/tasks/${taskId}`, {
    method: 'GET',
    token,
  })

  if (!isSuccessCode(response.code) || !response.data) {
    throw new Error(response.message || '获取抓取任务详情失败')
  }

  return response.data
}

/**
 * 将失败的异步导入任务重新投递为 pending，供独立 worker 再次消费。
 */
export async function retryScraperTask(token: string | null, taskId: number): Promise<ScraperTaskItem> {
  const response = await requestJson<ApiEnvelope<ScraperTaskItem>>(`/admin/scraper/tasks/${taskId}/retry`, {
    method: 'POST',
    token,
    body: {},
  })

  if (!isSuccessCode(response.code) || !response.data) {
    throw new Error(response.message || '重试抓取任务失败')
  }

  return response.data
}
