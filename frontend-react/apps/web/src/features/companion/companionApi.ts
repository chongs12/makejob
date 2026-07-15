import { requestJson } from '@makejob/api-client'
import { isSuccessCode, type ApiEnvelope } from '@makejob/shared-types'
import type {
  CompanionCategoryNode,
  CompanionChatReply,
  CompanionAdjustmentPreview,
  CompanionDailyDigest,
  CompanionGeneratePlanPayload,
  CompanionHistoryItem,
  CompanionPlanDetail,
  CompanionPlanProgress,
  CompanionPlanTask,
  CompanionTaskFeedbackPayload,
  CompanionSelectableLive2DModel,
  CompanionStudyLogPayload,
  CompanionTaskStatus,
  ConversationState,
} from './companionTypes'

const MAX_CHAT_HISTORY = 12

/**
 * 规范化计划调整预览响应，确保数组字段在后端省略时仍返回空数组，避免前端渲染阶段取 length 报错。
 */
function normalizeCompanionAdjustmentPreview(preview: CompanionAdjustmentPreview): CompanionAdjustmentPreview {
  return {
    ...preview,
    add: Array.isArray(preview.add) ? preview.add : [],
    remove: Array.isArray(preview.remove) ? preview.remove : [],
    reorder: Array.isArray(preview.reorder) ? preview.reorder : [],
    preview_tasks: Array.isArray(preview.preview_tasks) ? preview.preview_tasks : [],
  }
}

/**
 * 获取陪伴页当前可切换的 Live2D 模型列表，并保留后端给出的推荐顺序。
 */
export async function fetchSelectableCompanionLive2DModels(industryCode: string): Promise<CompanionSelectableLive2DModel[]> {
  const params = new URLSearchParams({
    scene: 'companion',
  })

  if (industryCode.trim()) {
    params.set('industry_code', industryCode.trim())
  }

  const response = await requestJson<ApiEnvelope<CompanionSelectableLive2DModel[]>>(`/live2d/models?${params.toString()}`)

  if (!isSuccessCode(response.code) || !response.data) {
    throw new Error(response.message || '获取陪伴页 Live2D 模型列表失败')
  }

  return response.data
}

/**
 * 拉取当前用户的进行中学习计划，并为左侧目标卡片提供数据。
 */
export async function fetchCurrentPlan(token: string): Promise<CompanionPlanDetail | null> {
  const response = await requestJson<ApiEnvelope<CompanionPlanDetail>>('/plans/current', {
    token,
  })

  if (!isSuccessCode(response.code)) {
    if (response.code === 404) {
      return null
    }
    throw new Error(response.message || '获取当前学习计划失败')
  }

  return response.data || null
}

/**
 * 拉取计划进度统计，为入口页补充更细的任务状态概览。
 */
export async function fetchCompanionPlanProgress(token: string, planId: number): Promise<CompanionPlanProgress> {
  const response = await requestJson<ApiEnvelope<CompanionPlanProgress>>(`/plans/${planId}/progress`, {
    token,
  })

  if (!isSuccessCode(response.code) || !response.data) {
    throw new Error(response.message || '获取计划进度失败')
  }

  return response.data
}

/**
 * 将当前学习陪伴页的每日执行摘要同步到服务端，供成长档案页跨设备查看。
 */
export async function syncCompanionStudyLog(token: string, payload: CompanionStudyLogPayload): Promise<void> {
  const response = await requestJson<ApiEnvelope<unknown>>('/growth/study-log', {
    method: 'POST',
    token,
    body: payload,
  })

  if (!isSuccessCode(response.code)) {
    throw new Error(response.message || '同步学习日志失败')
  }
}

/**
 * 按当前学习方向读取题库分类原始树结构，供页面自行决定如何展示。
 */
export async function fetchCompanionCategoryTree(industryCode: string): Promise<CompanionCategoryNode[]> {
  const params = new URLSearchParams({
    industry_code: industryCode.trim(),
  })
  const response = await requestJson<ApiEnvelope<CompanionCategoryNode[]>>(`/categories?${params.toString()}`)

  if (!isSuccessCode(response.code) || !response.data) {
    throw new Error(response.message || '获取弱项分类失败')
  }

  return response.data
}

/**
 * 调用计划生成接口，创建新的学习计划并返回最新详情。
 */
export async function createCompanionPlan(token: string, payload: CompanionGeneratePlanPayload): Promise<CompanionPlanDetail> {
  const response = await requestJson<ApiEnvelope<CompanionPlanDetail>>('/plans', {
    method: 'POST',
    token,
    body: payload,
  })

  if (!isSuccessCode(response.code) || !response.data) {
    throw new Error(response.message || '生成学习计划失败')
  }

  return response.data
}

/**
 * 更新单个学习任务状态，让陪伴页可以直接驱动计划推进。
 */
export async function updateCompanionTaskStatus(
  token: string,
  planId: number,
  taskId: number,
  status: CompanionTaskStatus,
): Promise<void> {
  const response = await requestJson<ApiEnvelope<null>>(`/plans/${planId}/tasks/${taskId}/status`, {
    method: 'PUT',
    token,
    body: {
      status,
    },
  })

  if (!isSuccessCode(response.code)) {
    throw new Error(response.message || '更新任务状态失败')
  }
}

/**
 * 为指定学习任务提交结构化训练反馈，供后续诊断和计划调整直接消费。
 */
export async function submitCompanionTaskFeedback(
  token: string,
  planId: number,
  taskId: number,
  payload: CompanionTaskFeedbackPayload,
): Promise<void> {
  const response = await requestJson<ApiEnvelope<null>>(`/plans/${planId}/tasks/${taskId}/feedback`, {
    method: 'POST',
    token,
    body: payload,
  })

  if (!isSuccessCode(response.code)) {
    throw new Error(response.message || '提交任务训练反馈失败')
  }
}

/**
 * 请求后端重新调整当前学习计划，返回新的计划详情。
 */
export async function adjustCompanionPlan(token: string, planId: number): Promise<CompanionPlanDetail> {
  const response = await requestJson<ApiEnvelope<CompanionPlanDetail>>(`/plans/${planId}/adjust`, {
    method: 'POST',
    token,
    body: {},
  })

  if (!isSuccessCode(response.code) || !response.data) {
    throw new Error(response.message || '调整学习计划失败')
  }

  return response.data
}

/**
 * 生成学习计划调整预览，返回待确认的结构化 diff，不直接执行落库。
 */
export async function previewCompanionPlanAdjustment(token: string, planId: number, reason = ''): Promise<CompanionAdjustmentPreview> {
  const response = await requestJson<ApiEnvelope<CompanionAdjustmentPreview>>(`/plans/${planId}/adjust-preview`, {
    method: 'POST',
    token,
    body: {
      reason,
    },
  })

  if (!isSuccessCode(response.code) || !response.data) {
    throw new Error(response.message || '生成计划调整预览失败')
  }

  return normalizeCompanionAdjustmentPreview(response.data)
}

/**
 * 应用已经确认过的调整预览，并返回最新计划详情。
 */
export async function applyCompanionPlanAdjustment(token: string, planId: number, previewToken: string): Promise<CompanionPlanDetail> {
  const response = await requestJson<ApiEnvelope<CompanionPlanDetail>>(`/plans/${planId}/adjust-apply`, {
    method: 'POST',
    token,
    body: {
      preview_token: previewToken,
    },
  })

  if (!isSuccessCode(response.code) || !response.data) {
    throw new Error(response.message || '应用计划调整失败')
  }

  return response.data
}

/**
 * 将当前对话历史压缩成后端陪伴接口可消费的消息列表，避免单次请求过大。
 */
function buildChatPayload(history: CompanionHistoryItem[]) {
  return history
    .filter((item) => item.role === 'assistant' || item.role === 'user')
    .slice(-MAX_CHAT_HISTORY)
    .map((item) => ({
      role: item.role,
      content: item.content,
    }))
}

/**
 * 调用 ASR 语音识别接口，将音频数据转换为文本。
 */
export async function recognizeSpeech(
  token: string,
  audioData: Blob,
  format = 'pcm',
  sampleRate = 16000,
  language = 'zh-CN',
): Promise<{ text: string; confidence: number; duration: number }> {
  const params = new URLSearchParams({
    format,
    sample_rate: String(sampleRate),
    language,
  })

  const response = await requestJson<ApiEnvelope<{ text: string; confidence: number; duration: number }>>(
    `/companion/asr?${params.toString()}`,
    {
      method: 'POST',
      token,
      body: audioData,
      headers: {
        'Content-Type': 'application/octet-stream',
      },
    },
  )

  if (!isSuccessCode(response.code) || !response.data) {
    throw new Error(response.message || '语音识别失败')
  }

  return response.data
}

/**
 * 向后端陪伴接口发送消息，并返回陪伴助手的最新回复内容。
 */
export async function sendCompanionChatRequest(
  token: string,
  history: CompanionHistoryItem[],
  plan: CompanionPlanDetail | null,
  focusedTask: CompanionPlanTask | null,
  dailyDigest: CompanionDailyDigest | null,
  live2DModelKey: string,
  context: {
    deriveTodayGoals: (plan: CompanionPlanDetail | null) => CompanionPlanTask[]
    deriveActiveGoals: (plan: CompanionPlanDetail | null) => CompanionPlanTask[]
  },
  conversationState?: ConversationState | null,
): Promise<CompanionChatReply> {
  const response = await requestJson<ApiEnvelope<CompanionChatReply>>('/companion/chat', {
    method: 'POST',
    token,
    body: {
      messages: buildChatPayload(history),
      live2d_model_key: live2DModelKey,
      conversation_state_json: conversationState ? JSON.stringify(conversationState) : '',
      context: {
        current_plan_title: plan?.title || '',
        current_plan_progress: plan?.progress || 0,
        today_goals: context.deriveTodayGoals(plan).map((item) => item.title),
        active_goals: context.deriveActiveGoals(plan).map((item) => item.title),
        focused_task_title: focusedTask?.title || '',
        focused_task_description: focusedTask?.description || '',
        completed_today_count: dailyDigest?.completedTitles.length || 0,
        skipped_today_count: dailyDigest?.skippedTitles.length || 0,
        latest_task_action: dailyDigest?.latestActionText || '',
      },
    },
  })

  if (!isSuccessCode(response.code) || !response.data) {
    throw new Error(response.message || '陪伴助手暂时没有回复')
  }

  return response.data
}
