import {
  DEFAULT_FRONTEND_INDUSTRY_CODE as DEFAULT_COMPANION_INDUSTRY_CODE,
} from '../../shared/industryContext'
import type { CompanionPlanContextDraft } from '../../shared/companionContext'
import type { WeeklyFocusTheme } from '../../shared/weeklyFocus'
import type {
  CompanionCategoryNode,
  CompanionCategoryOption,
  CompanionGeneratePlanForm,
  CompanionGeneratePlanPayload,
  CompanionPlanDetail,
  CompanionPlanTask,
  CompanionSessionSummary,
} from './companionTypes'

/**
 * 创建学习计划表单的默认值，避免入口页首次渲染时出现空壳状态。
 */
export function buildInitialPlanForm(): CompanionGeneratePlanForm {
  return {
    level: 'beginner',
    dailyStudyTime: '60',
    durationDays: '14',
    goalDescription: '',
    weakTopics: [],
    weakTopicsText: '',
  }
}

/**
 * 将分类树拍平成弱项选项列表，便于入口页直接复用现有题库分类。
 */
export function flattenCompanionCategories(nodes: CompanionCategoryNode[], level = 0): CompanionCategoryOption[] {
  if (!Array.isArray(nodes)) {
    return []
  }
  return nodes.flatMap((node) => [
    {
      id: node.id,
      name: `${'　'.repeat(level)}${node.name}`,
    },
    ...flattenCompanionCategories(node.children || [], level + 1),
  ])
}

/**
 * 将用户自由输入的弱项文本拆成数组，兼容逗号和换行两种输入方式。
 */
function parseWeakTopicsText(value: string): string[] {
  return Array.from(
    new Set(
      value
        .split(/[\n,，]/)
        .map((item) => item.trim())
        .filter(Boolean),
    ),
  )
}

/**
 * 将计划生成表单转换为后端可直接消费的请求结构。
 */
export function buildGeneratePlanPayload(form: CompanionGeneratePlanForm, industryCode: string): CompanionGeneratePlanPayload {
  return {
    level: form.level,
    daily_study_time: Number(form.dailyStudyTime) || 60,
    weak_topics: Array.from(new Set([...form.weakTopics, ...parseWeakTopicsText(form.weakTopicsText)])),
    goal_description: form.goalDescription.trim(),
    duration_days: Number(form.durationDays) || 14,
    industry_code: industryCode.trim() || DEFAULT_COMPANION_INDUSTRY_CODE,
  }
}

/**
 * 将学习等级转换成前台文案，避免表单直接暴露英文枚举值。
 */
export function planLevelLabel(level: string): string {
  const map: Record<string, string> = {
    beginner: '初级',
    intermediate: '中级',
    advanced: '高级',
  }

  return map[level] || level || '未设置'
}

/**
 * 将跨页带入的学习陪伴上下文合并进计划表单，尽量保留用户已经手动输入的内容。
 */
export function applyCompanionPlanContextToForm(
  form: CompanionGeneratePlanForm,
  draft: CompanionPlanContextDraft,
): CompanionGeneratePlanForm {
  const initialForm = buildInitialPlanForm()
  const mergedWeakTopics = Array.from(new Set([...draft.weakTopics, ...form.weakTopics])).slice(0, 12)
  const mergedWeakTopicsText = form.weakTopicsText.trim() || draft.weakTopics.join('，')

  return {
    level: !form.level || form.level === initialForm.level ? draft.recommendedLevel : form.level,
    dailyStudyTime:
      !form.dailyStudyTime || form.dailyStudyTime === initialForm.dailyStudyTime
        ? String(draft.recommendedDailyStudyTime)
        : form.dailyStudyTime,
    durationDays:
      !form.durationDays || form.durationDays === initialForm.durationDays
        ? String(draft.recommendedDurationDays)
        : form.durationDays,
    goalDescription: form.goalDescription.trim() || draft.goalDescription,
    weakTopics: mergedWeakTopics,
    weakTopicsText: mergedWeakTopicsText,
  }
}

/**
 * 将本周重点补强主题压缩成一段可直接塞进计划目标的文案。
 */
function buildWeeklyFocusGoalDescription(themes: WeeklyFocusTheme[]): string {
  const titles = Array.from(new Set(themes.map((item) => item.title.trim()).filter(Boolean))).slice(0, 3)
  if (!titles.length) {
    return ''
  }

  const phaseLabels = Array.from(
    new Set(themes.map((item) => item.dominant_archive_phase_label?.trim() || '').filter(Boolean)),
  ).slice(0, 2)
  if (phaseLabels.length) {
    return `本周优先补强${titles.map((item) => `「${item}」`).join('、')}，当前问题主要集中在${phaseLabels.join('、')}，并围绕这些问题安排连续复习、专项练习和复盘。`
  }

  return `本周优先补强${titles.map((item) => `「${item}」`).join('、')}，并围绕这些问题安排连续复习、专项练习和复盘。`
}

/**
 * 将本周补强主题一键合并进计划表单，减少用户手动搬运弱项和目标描述。
 */
export function applyWeeklyFocusToPlanForm(
  form: CompanionGeneratePlanForm,
  themes: WeeklyFocusTheme[],
): CompanionGeneratePlanForm {
  if (!Array.isArray(themes)) {
    return form
  }
  const focusTags = Array.from(
    new Set(
      themes
        .flatMap((theme) => (theme.focus_tags.length ? theme.focus_tags : [theme.title]))
        .map((item) => item.trim())
        .filter(Boolean),
    ),
  ).slice(0, 12)
  const mergedWeakTopics = Array.from(new Set([...focusTags, ...form.weakTopics])).slice(0, 12)
  const mergedWeakTopicsText = Array.from(new Set([...focusTags, ...parseWeakTopicsText(form.weakTopicsText)])).join('，')
  const goalPreset = buildWeeklyFocusGoalDescription(themes)
  const currentGoalDescription = form.goalDescription.trim()
  const nextGoalDescription =
    goalPreset && !currentGoalDescription.includes(goalPreset)
      ? (currentGoalDescription ? `${goalPreset}\n${currentGoalDescription}` : goalPreset)
      : currentGoalDescription

  return {
    ...form,
    goalDescription: nextGoalDescription,
    weakTopics: mergedWeakTopics,
    weakTopicsText: mergedWeakTopicsText,
  }
}

/**
 * 将陪伴上下文的推荐参数整理成入口页提示文案，帮助用户理解当前预填依据。
 */
export function buildCompanionContextPresetText(draft: CompanionPlanContextDraft): string {
  return `${planLevelLabel(draft.recommendedLevel)} · ${draft.recommendedDailyStudyTime} 分钟/天 · ${draft.recommendedDurationDays} 天`
}

/**
 * 将计划状态转换成更适合前台阅读的中文标签。
 */
export function planStatusLabel(status: string): string {
  const map: Record<string, string> = {
    generating: '生成中',
    active: '进行中',
    paused: '已暂停',
    completed: '已完成',
    draft: '草稿',
  }

  return map[status] || status || '未定义'
}

/**
 * 从当前计划中找出最近完成的一项任务，供入口页展示最近推进记录。
 */
export function deriveLatestCompletedTask(plan: CompanionPlanDetail | null): CompanionPlanTask | null {
  if (!plan?.tasks?.length) {
    return null
  }

  const completedTasks = plan.tasks
    .filter((item) => item.status === 'completed' && item.completed_at)
    .sort((left, right) => new Date(right.completed_at || 0).getTime() - new Date(left.completed_at || 0).getTime())

  return completedTasks[0] || null
}

/**
 * 从当前计划中提取接下来最值得继续推进的未完成任务。
 */
export function deriveUpcomingTasks(plan: CompanionPlanDetail | null): CompanionPlanTask[] {
  if (!plan?.tasks?.length) {
    return []
  }

  return [...plan.tasks]
    .filter((item) => item.status !== 'completed' && item.status !== 'skipped')
    .sort((left, right) => {
      if ((left.day_number || 0) !== (right.day_number || 0)) {
        return (left.day_number || 0) - (right.day_number || 0)
      }
      return (left.sort_order || 0) - (right.sort_order || 0)
    })
    .slice(0, 3)
}

/**
 * 根据当前计划和会话摘要生成入口页的继续引导文案。
 */
export function buildContinueHint(plan: CompanionPlanDetail | null, summary: CompanionSessionSummary | null): string {
  if (plan?.title) {
    return `当前已经有进行中的计划，建议直接进入学习陪伴页继续推进「${plan.title}」。`
  }

  if (summary?.latestAssistantReply) {
    return '入口页已经记住你上次的对话摘要，进入学习陪伴页后可以从上次节奏继续。'
  }

  return '如果你还没有计划，先在这里生成一份学习计划，再进入学习陪伴页开始今天的推进。'
}
