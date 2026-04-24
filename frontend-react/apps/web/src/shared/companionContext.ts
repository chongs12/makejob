export interface CompanionPlanContextDraft {
  source: 'interview-report'
  interviewId: string
  createdAt: number
  industryCode: string
  industryLabel: string
  summary: string
  readinessLabel: string
  goalDescription: string
  weakTopics: string[]
  suggestions: string[]
  recommendedLevel: string
  recommendedDailyStudyTime: number
  recommendedDurationDays: number
}

export interface InterviewCompanionContextInput {
  interviewId: string
  industryCode: string
  industryLabel: string
  overallScore: number
  summary: string
  readinessLabel: string
  weakTopics: string[]
  suggestions: string[]
}

const COMPANION_PLAN_CONTEXT_KEY = 'makejob.companion.plan-context'

/**
 * 根据面试报告得分推导更适合当前阶段的学习计划强度，减少用户二次填写成本。
 */
function recommendCompanionPlanPreset(score: number): {
  level: string
  dailyStudyTime: number
  durationDays: number
} {
  if (score >= 85) {
    return {
      level: 'advanced',
      dailyStudyTime: 45,
      durationDays: 7,
    }
  }

  if (score >= 70) {
    return {
      level: 'intermediate',
      dailyStudyTime: 60,
      durationDays: 10,
    }
  }

  if (score >= 55) {
    return {
      level: 'intermediate',
      dailyStudyTime: 75,
      durationDays: 14,
    }
  }

  return {
    level: 'beginner',
    dailyStudyTime: 90,
    durationDays: 21,
  }
}

/**
 * 基于面试报告生成学习陪伴入口页可直接消费的上下文草稿。
 */
export function buildInterviewCompanionContextDraft(
  input: InterviewCompanionContextInput,
): CompanionPlanContextDraft {
  const preset = recommendCompanionPlanPreset(input.overallScore || 0)
  const weakTopics = Array.from(new Set(input.weakTopics.map((item) => item.trim()).filter(Boolean))).slice(0, 6)
  const suggestions = Array.from(new Set(input.suggestions.map((item) => item.trim()).filter(Boolean))).slice(0, 3)
  const focusTopics = weakTopics.slice(0, 3)
  const focusText = focusTopics.length ? focusTopics.join('、') : `${input.industryLabel} 核心模块`

  return {
    source: 'interview-report',
    interviewId: input.interviewId.trim(),
    createdAt: Date.now(),
    industryCode: input.industryCode.trim(),
    industryLabel: input.industryLabel.trim() || '当前方向',
    summary: input.summary.trim(),
    readinessLabel: input.readinessLabel.trim(),
    goalDescription: `基于第 ${input.interviewId.trim() || '-'} 场${input.industryLabel.trim() || '当前方向'}面试报告，优先补强${focusText}，并整理一份可连续执行的强化复习计划。`,
    weakTopics,
    suggestions,
    recommendedLevel: preset.level,
    recommendedDailyStudyTime: preset.dailyStudyTime,
    recommendedDurationDays: preset.durationDays,
  }
}

/**
 * 暂存待带入学习陪伴页的上下文草稿，供跨页面跳转后自动预填表单。
 */
export function persistCompanionPlanContext(payload: CompanionPlanContextDraft): void {
  if (typeof window === 'undefined') {
    return
  }

  window.localStorage.setItem(COMPANION_PLAN_CONTEXT_KEY, JSON.stringify(payload))
}

/**
 * 读取当前学习陪伴上下文草稿，并在缓存损坏时自动清理无效数据。
 */
export function readCompanionPlanContext(): CompanionPlanContextDraft | null {
  if (typeof window === 'undefined') {
    return null
  }

  try {
    const raw = window.localStorage.getItem(COMPANION_PLAN_CONTEXT_KEY)
    if (!raw) {
      return null
    }

    const parsed = JSON.parse(raw) as Partial<CompanionPlanContextDraft>
    return {
      source: 'interview-report',
      interviewId: parsed.interviewId?.trim() || '',
      createdAt: Number(parsed.createdAt) || Date.now(),
      industryCode: parsed.industryCode?.trim() || '',
      industryLabel: parsed.industryLabel?.trim() || '当前方向',
      summary: parsed.summary?.trim() || '',
      readinessLabel: parsed.readinessLabel?.trim() || '',
      goalDescription: parsed.goalDescription?.trim() || '',
      weakTopics: Array.isArray(parsed.weakTopics) ? parsed.weakTopics.map((item) => String(item).trim()).filter(Boolean) : [],
      suggestions: Array.isArray(parsed.suggestions) ? parsed.suggestions.map((item) => String(item).trim()).filter(Boolean) : [],
      recommendedLevel: parsed.recommendedLevel?.trim() || 'beginner',
      recommendedDailyStudyTime: Number(parsed.recommendedDailyStudyTime) || 60,
      recommendedDurationDays: Number(parsed.recommendedDurationDays) || 14,
    }
  } catch {
    clearCompanionPlanContext()
    return null
  }
}

/**
 * 清空已不再需要的学习陪伴上下文草稿，避免旧报告持续污染新计划表单。
 */
export function clearCompanionPlanContext(): void {
  if (typeof window === 'undefined') {
    return
  }

  window.localStorage.removeItem(COMPANION_PLAN_CONTEXT_KEY)
}
