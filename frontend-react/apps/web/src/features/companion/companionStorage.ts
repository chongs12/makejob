import type {
  CompanionDailyDigest,
  CompanionFocusTaskDraft,
  CompanionSessionSummary,
} from './companionTypes'

const COMPANION_SESSION_SUMMARY_KEY = 'makejob.companion.session-summary'
const COMPANION_SELECTED_MODEL_KEY_PREFIX = 'makejob.companion.selected-live2d:'
const COMPANION_FOCUS_TASK_KEY = 'makejob.companion.focus-task'
const COMPANION_DAILY_DIGEST_KEY = 'makejob.companion.daily-digest'

/**
 * 从本地缓存恢复最近一次陪伴会话摘要，供入口页显示“上次聊到哪一步”。
 */
export function readCompanionSessionSummary(): CompanionSessionSummary | null {
  if (typeof window === 'undefined') {
    return null
  }

  try {
    const raw = window.localStorage.getItem(COMPANION_SESSION_SUMMARY_KEY)
    if (!raw) {
      return null
    }

    return JSON.parse(raw) as CompanionSessionSummary
  } catch {
    return null
  }
}

/**
 * 将最近一次陪伴会话摘要写入本地缓存，避免每次返回入口页都丢失上下文。
 */
export function persistCompanionSessionSummary(summary: CompanionSessionSummary): void {
  if (typeof window === 'undefined') {
    return
  }

  window.localStorage.setItem(COMPANION_SESSION_SUMMARY_KEY, JSON.stringify(summary))
}

/**
 * 生成学习陪伴执行闭环使用的本地日期键，避免跨天复用旧的今日摘要。
 */
export function buildCompanionLocalDateKey(date = new Date()): string {
  return [
    date.getFullYear(),
    String(date.getMonth() + 1).padStart(2, '0'),
    String(date.getDate()).padStart(2, '0'),
  ].join('-')
}

/**
 * 读取当前被标记为“继续推进”的任务草稿，供入口页和陪伴房间共用。
 */
export function readCompanionFocusTask(): CompanionFocusTaskDraft | null {
  if (typeof window === 'undefined') {
    return null
  }

  try {
    const raw = window.localStorage.getItem(COMPANION_FOCUS_TASK_KEY)
    if (!raw) {
      return null
    }

    const parsed = JSON.parse(raw) as Partial<CompanionFocusTaskDraft>
    const planId = Number(parsed.planId)
    const taskId = Number(parsed.taskId)
    if (!planId || !taskId) {
      clearCompanionFocusTask()
      return null
    }

    return {
      planId,
      taskId,
      title: parsed.title?.trim() || '',
      status: parsed.status?.trim() || 'pending',
      source: parsed.source === 'hub' ? 'hub' : 'room',
      updatedAt: Number(parsed.updatedAt) || Date.now(),
    }
  } catch {
    clearCompanionFocusTask()
    return null
  }
}

/**
 * 记住当前最值得继续推进的任务，便于跨页面返回时直接续接。
 */
export function persistCompanionFocusTask(task: CompanionFocusTaskDraft): void {
  if (typeof window === 'undefined') {
    return
  }

  window.localStorage.setItem(COMPANION_FOCUS_TASK_KEY, JSON.stringify(task))
}

/**
 * 清空当前任务续接草稿，避免已完成或已失效任务继续污染入口提示。
 */
export function clearCompanionFocusTask(): void {
  if (typeof window === 'undefined') {
    return
  }

  window.localStorage.removeItem(COMPANION_FOCUS_TASK_KEY)
}

/**
 * 构造今日执行摘要的默认骨架，保证首次进入时也能稳定写入本地缓存。
 */
export function buildInitialCompanionDailyDigest(): CompanionDailyDigest {
  return {
    dateKey: buildCompanionLocalDateKey(),
    updatedAt: Date.now(),
    completedTitles: [],
    skippedTitles: [],
    latestActionText: '',
  }
}

/**
 * 读取学习陪伴页的今日执行摘要，并在跨天时自动重置为新的一天。
 */
export function readCompanionDailyDigest(): CompanionDailyDigest | null {
  if (typeof window === 'undefined') {
    return null
  }

  try {
    const raw = window.localStorage.getItem(COMPANION_DAILY_DIGEST_KEY)
    if (!raw) {
      return null
    }

    const parsed = JSON.parse(raw) as Partial<CompanionDailyDigest>
    const todayKey = buildCompanionLocalDateKey()
    if (parsed.dateKey !== todayKey) {
      clearCompanionDailyDigest()
      return null
    }

    return {
      dateKey: todayKey,
      updatedAt: Number(parsed.updatedAt) || Date.now(),
      completedTitles: Array.isArray(parsed.completedTitles) ? parsed.completedTitles.map((item) => String(item).trim()).filter(Boolean) : [],
      skippedTitles: Array.isArray(parsed.skippedTitles) ? parsed.skippedTitles.map((item) => String(item).trim()).filter(Boolean) : [],
      latestActionText: parsed.latestActionText?.trim() || '',
    }
  } catch {
    clearCompanionDailyDigest()
    return null
  }
}

/**
 * 持久化今日执行摘要，供入口页和陪伴房间同步展示今天的推进情况。
 */
export function persistCompanionDailyDigest(digest: CompanionDailyDigest): void {
  if (typeof window === 'undefined') {
    return
  }

  window.localStorage.setItem(COMPANION_DAILY_DIGEST_KEY, JSON.stringify(digest))
}

/**
 * 清空过期或无效的今日执行摘要，避免旧日期数据继续显示在当前页面。
 */
export function clearCompanionDailyDigest(): void {
  if (typeof window === 'undefined') {
    return
  }

  window.localStorage.removeItem(COMPANION_DAILY_DIGEST_KEY)
}

/**
 * 为当前行业上下文构造陪伴模型选择的本地缓存键。
 */
export function buildCompanionSelectedModelStorageKey(industryCode: string): string {
  return `${COMPANION_SELECTED_MODEL_KEY_PREFIX}${industryCode.trim() || 'default'}`
}

/**
 * 读取当前行业下最近一次手动切换的陪伴模型键。
 */
export function readSelectedCompanionModelKey(industryCode: string): string {
  if (typeof window === 'undefined') {
    return ''
  }

  return window.localStorage.getItem(buildCompanionSelectedModelStorageKey(industryCode)) || ''
}

/**
 * 记住用户在当前行业上下文下选择的陪伴模型，便于下次直接恢复。
 */
export function persistSelectedCompanionModelKey(industryCode: string, modelKey: string): void {
  if (typeof window === 'undefined') {
    return
  }

  window.localStorage.setItem(buildCompanionSelectedModelStorageKey(industryCode), modelKey)
}
