import {
  buildCompanionLocalDateKey,
  buildInitialCompanionDailyDigest,
  clearCompanionFocusTask,
  persistCompanionDailyDigest,
  persistCompanionFocusTask,
  readCompanionFocusTask,
} from './companionStorage'
import type {
  CompanionDailyDigest,
  CompanionFeedbackDifficulty,
  CompanionFeedbackTrainingType,
  CompanionFocusTaskDraft,
  CompanionPlanDetail,
  CompanionPlanProgress,
  CompanionPlanTask,
  CompanionStudyLogPayload,
  CompanionTaskFeedbackDraft,
  CompanionTaskFeedbackPayload,
  CompanionTaskActionSource,
  CompanionTaskStatus,
} from './companionTypes'

interface CompanionQuestionSetLookupQuestion {
  id: number
  title: string
}

interface CompanionQuestionSetLookupSource {
  questions?: CompanionQuestionSetLookupQuestion[]
}

/**
 * 将阶段枚举转换成更适合解释文案使用的中文名称，避免续接提示里直接暴露原始枚举值。
 */
function formatCompanionPhaseText(value?: string): string {
  switch (String(value || '').trim()) {
    case 'drill':
      return '专项突破'
    case 'review':
      return '复盘纠偏'
    case 'mock':
      return '模拟验证'
    case 'foundation':
      return '打基础'
    default:
      return ''
  }
}

/**
 * 判断任务日期是否与当前本地日期处于同一天，用于筛选今日目标。
 */
export function isSameLocalDay(value?: string): boolean {
  if (!value) {
    return false
  }

  const targetDate = new Date(value)
  if (Number.isNaN(targetDate.getTime())) {
    return false
  }

  const now = new Date()
  return (
    targetDate.getFullYear() === now.getFullYear()
    && targetDate.getMonth() === now.getMonth()
    && targetDate.getDate() === now.getDate()
  )
}

/**
 * 按执行优先级对任务排序，让进行中任务和更靠前的任务优先浮到列表顶部。
 */
export function sortCompanionTasksForExecution(tasks: CompanionPlanTask[]): CompanionPlanTask[] {
  const statusPriority: Record<string, number> = {
    in_progress: 0,
    pending: 1,
    skipped: 2,
    completed: 3,
  }

  return [...tasks].sort((left, right) => {
    const statusDelta = (statusPriority[left.status] ?? 9) - (statusPriority[right.status] ?? 9)
    if (statusDelta !== 0) {
      return statusDelta
    }

    const dayDelta = (left.day_number || 0) - (right.day_number || 0)
    if (dayDelta !== 0) {
      return dayDelta
    }

    return (left.sort_order || 0) - (right.sort_order || 0)
  })
}

/**
 * 从当前计划中提炼“今日目标”，优先展示今天到期的任务，再回退到最靠前的未完成任务。
 */
export function deriveTodayGoals(plan: CompanionPlanDetail | null): CompanionPlanTask[] {
  if (!plan?.tasks?.length) {
    return []
  }

  const todayTasks = sortCompanionTasksForExecution(
    plan.tasks.filter((item) => isSameLocalDay(item.due_date) && item.status !== 'completed' && item.status !== 'skipped'),
  )
  if (todayTasks.length > 0) {
    return todayTasks.slice(0, 3)
  }

  return sortCompanionTasksForExecution(plan.tasks.filter((item) => item.status !== 'completed' && item.status !== 'skipped')).slice(0, 3)
}

/**
 * 从当前计划中提炼“进行中目标”，优先显示进行中任务，再回退到首个未完成任务。
 */
export function deriveActiveGoals(plan: CompanionPlanDetail | null): CompanionPlanTask[] {
  if (!plan?.tasks?.length) {
    return []
  }

  const inProgressTasks = plan.tasks.filter((item) => item.status === 'in_progress')
  if (inProgressTasks.length > 0) {
    return sortCompanionTasksForExecution(inProgressTasks).slice(0, 2)
  }

  const pendingTask = sortCompanionTasksForExecution(plan.tasks).find((item) => item.status !== 'completed' && item.status !== 'skipped')
  return pendingTask ? [pendingTask] : []
}

/**
 * 将任务状态转换成更适合前台阅读的中文文案。
 */
export function taskStatusLabel(status: string): string {
  const labelMap: Record<string, string> = {
    pending: '待开始',
    in_progress: '进行中',
    completed: '已完成',
    skipped: '已跳过',
  }

  return labelMap[status] || '未定义'
}

/**
 * 将默认反馈摘要候选片段整理成稳定文本，避免空字段和重复信息污染用户首屏输入。
 */
function buildCompanionFeedbackSummaryText(parts: string[]): string {
  return Array.from(new Set(parts.map((item) => item.trim()).filter(Boolean))).join('；')
}

/**
 * 根据当前任务类型预估默认反馈类型，减少用户首次提交反馈时的输入成本。
 */
function resolveDefaultFeedbackTrainingType(task: CompanionPlanTask | null): CompanionFeedbackTrainingType {
  if (task?.collection_hint?.trim()) {
    return 'coding'
  }

  switch (task?.task_type) {
    case 'practice':
      return 'coding'
    case 'interview':
      return 'short_answer'
    default:
      return 'generic'
  }
}

/**
 * 为任务完成反馈生成一份可直接编辑的默认表单草稿。
 */
export function buildDefaultCompanionTaskFeedbackDraft(
  plan: CompanionPlanDetail | null,
  task: CompanionPlanTask | null,
): CompanionTaskFeedbackDraft {
  const phaseHint = buildCompanionPhaseAdjustmentHint(plan)
  const baseSummaryParts = [
    task?.phase ? `当前任务阶段：${formatCompanionPhaseText(task.phase)}` : '',
    task?.phase_goal ? `阶段目标：${task.phase_goal}` : '',
    phaseHint ? `阶段调整说明：${phaseHint}` : '',
    task?.source_label ? `任务来源：${task.source_label}` : '',
    task?.reason ? `安排原因：${task.reason}` : '',
  ].filter(Boolean)

  return {
    trainingType: resolveDefaultFeedbackTrainingType(task),
    attemptCount: 1,
    timeSpentMinutes: task?.task_type === 'interview' ? 30 : 20,
    difficultySelfAssessment: 'just_right',
    mistakeTagsText: '',
    summary: buildCompanionFeedbackSummaryText(baseSummaryParts),
  }
}

/**
 * 将用户输入的错因标签文本拆成结构化标签数组，兼容中英文逗号和换行。
 */
export function parseCompanionFeedbackMistakeTags(value: string): string[] {
  return Array.from(
    new Set(
      value
        .split(/[\n,，、;；]+/)
        .map((item) => item.trim())
        .filter(Boolean),
    ),
  )
}

/**
 * 从任务来源引用里提取明确题目 ID，命中时可直接回填到训练反馈中。
 */
function parseCompanionQuestionIdFromSourceRef(sourceRef?: string): number | undefined {
  const normalizedSourceRef = String(sourceRef || '').trim()
  if (!normalizedSourceRef) {
    return undefined
  }

  const matched = normalizedSourceRef.match(/(?:question|practice-question|question_id)[:#-](\d+)/i)
  if (!matched) {
    return undefined
  }

  const questionId = Number(matched[1])
  return Number.isFinite(questionId) && questionId > 0 ? questionId : undefined
}

/**
 * 根据计划任务标题和题单详情最佳努力匹配对应题目 ID，避免反馈链路丢失可用题目上下文。
 */
export function resolveCompanionTaskQuestionId(
  task: CompanionPlanTask | null,
  questionSetDetail?: CompanionQuestionSetLookupSource | null,
): number | undefined {
  const sourceRefQuestionId = parseCompanionQuestionIdFromSourceRef(task?.source_ref)
  if (sourceRefQuestionId) {
    return sourceRefQuestionId
  }

  const normalizedTitle = String(task?.title || '').trim().toLowerCase()
  if (!normalizedTitle || !questionSetDetail?.questions?.length) {
    return undefined
  }

  const exactMatch = questionSetDetail.questions.find((item) => item.title.trim().toLowerCase() === normalizedTitle)
  if (exactMatch) {
    return exactMatch.id
  }

  const fuzzyMatches = questionSetDetail.questions.filter((item) => {
    const normalizedQuestionTitle = item.title.trim().toLowerCase()
    return normalizedQuestionTitle.includes(normalizedTitle) || normalizedTitle.includes(normalizedQuestionTitle)
  })
  if (fuzzyMatches.length === 1) {
    return fuzzyMatches[0].id
  }

  return undefined
}

/**
 * 将前端反馈草稿整理成后端学习计划反馈接口可直接接收的请求体。
 */
export function buildCompanionTaskFeedbackPayload(
  draft: CompanionTaskFeedbackDraft,
  questionId?: number,
): CompanionTaskFeedbackPayload {
  const difficulty = (draft.difficultySelfAssessment || '').trim() as CompanionFeedbackDifficulty
  return {
    training_type: draft.trainingType,
    question_id: questionId && questionId > 0 ? questionId : undefined,
    mistake_tags: parseCompanionFeedbackMistakeTags(draft.mistakeTagsText),
    attempt_count: Math.max(0, Math.floor(draft.attemptCount || 0)),
    time_spent_seconds: Math.max(0, Math.floor(draft.timeSpentMinutes || 0)) * 60,
    difficulty_self_assessment: difficulty || undefined,
    summary: draft.summary.trim(),
  }
}

/**
 * 根据当前计划和本地续接草稿找出最应该继续推进的任务。
 */
export function resolveFocusedCompanionTask(
  plan: CompanionPlanDetail | null,
  draft: CompanionFocusTaskDraft | null,
): CompanionPlanTask | null {
  if (!plan?.tasks?.length) {
    return null
  }

  if (draft && draft.planId === plan.id) {
    const matchedTask = plan.tasks.find((item) => item.id === draft.taskId)
    if (matchedTask && matchedTask.status !== 'completed' && matchedTask.status !== 'skipped') {
      return matchedTask
    }
  }

  return deriveActiveGoals(plan)[0] || deriveTodayGoals(plan)[0] || null
}

/**
 * 根据当前执行情况生成“今日小结”文案，帮助用户一眼判断今天还剩什么。
 */
export function buildCompanionDailyDigestText(
  plan: CompanionPlanDetail | null,
  digest: CompanionDailyDigest | null,
  focusedTask?: CompanionPlanTask | null,
): string {
  if (!plan) {
    return '登录并生成计划后，这里会汇总你今天完成了什么、还剩什么。'
  }

  const completedToday = digest?.completedTitles.length || 0
  const skippedToday = digest?.skippedTitles.length || 0
  const remainingToday = deriveTodayGoals(plan).length
  const effectiveFocusedTask = focusedTask || resolveFocusedCompanionTask(plan, readCompanionFocusTask())

  if (completedToday === 0 && skippedToday === 0 && remainingToday === 0) {
    return '今天没有待推进的任务，当前计划已经收口，可以考虑开始新阶段。'
  }

  if (completedToday === 0 && remainingToday > 0) {
    return `今天还没有记到完成项，建议先从「${effectiveFocusedTask?.title || deriveTodayGoals(plan)[0]?.title || '当前首个任务'}」开始。`
  }

  const segments = [`今天已完成 ${completedToday} 项`]
  if (skippedToday > 0) {
    segments.push(`跳过 ${skippedToday} 项`)
  }
  if (remainingToday > 0) {
    segments.push(`还剩 ${remainingToday} 项待推进`)
  }
  if (effectiveFocusedTask) {
    segments.push(`下一步继续「${effectiveFocusedTask.title}」`)
  }

  return segments.join('，') + '。'
}

/**
 * 根据当前计划里的阶段入口与调整摘要生成一段更适合前端直接展示的阶段解释文案。
 */
export function buildCompanionPhaseAdjustmentHint(plan: CompanionPlanDetail | null): string {
  if (!plan) {
    return ''
  }

  const entryPhase = String(plan.entry_phase || '').trim()
  const currentPhase = String(plan.phase || '').trim()
  const entryPhaseLabel = formatCompanionPhaseText(entryPhase)
  const currentPhaseLabel = formatCompanionPhaseText(currentPhase)
  const summaries = Array.isArray(plan.adjustment_summaries)
    ? plan.adjustment_summaries.map((item) => String(item).trim()).filter(Boolean)
    : []

  const parts: string[] = []
  if (entryPhaseLabel && currentPhaseLabel && entryPhase !== currentPhase) {
    parts.push(`本轮先从${entryPhaseLabel}切入，当前已推进到${currentPhaseLabel}。`)
  } else if (entryPhaseLabel) {
    parts.push(`本轮从${entryPhaseLabel}切入，说明当前更适合先按这一阶段收口。`)
  } else if (currentPhaseLabel && summaries.length > 0) {
    parts.push(`当前计划正处于${currentPhaseLabel}。`)
  }

  if (summaries[0]) {
    parts.push(`触发原因：${summaries[0]}`)
  }

  return parts.join('')
}

/**
 * 为“围绕这项继续”之类的追问生成一段更贴近当前阶段上下文的默认提问。
 */
export function buildCompanionContinuePrompt(
  plan: CompanionPlanDetail | null,
  task: CompanionPlanTask | null,
): string {
  const phaseHint = buildCompanionPhaseAdjustmentHint(plan)
  const taskTitle = String(task?.title || '').trim()
  if (!taskTitle) {
    return phaseHint
      ? `结合我当前计划（${phaseHint}），告诉我接下来最该推进什么。`
      : '结合我当前计划，告诉我接下来最该推进什么。'
  }

  return phaseHint
    ? `请结合我当前计划（${phaseHint}），继续推进「${taskTitle}」，先告诉我下一步最该做什么。`
    : `请帮我继续推进「${taskTitle}」，先告诉我下一步最该做什么。`
}

/**
 * 将本地今日执行摘要整理成服务端学习日志接口可直接消费的请求体。
 */
export function buildCompanionStudyLogPayload(
  plan: CompanionPlanDetail | null,
  digest: CompanionDailyDigest | null,
  focusedTask: CompanionPlanTask | null,
): CompanionStudyLogPayload | null {
  if (!digest || digest.dateKey !== buildCompanionLocalDateKey()) {
    return null
  }

  return {
    date_key: digest.dateKey,
    plan_id: plan?.id,
    summary: buildCompanionDailyDigestText(plan, digest, focusedTask),
    focus_task_title: focusedTask?.title || '',
    completed_count: digest.completedTitles.length,
    skipped_count: digest.skippedTitles.length,
    completed_titles: digest.completedTitles,
    skipped_titles: digest.skippedTitles,
    latest_action_text: digest.latestActionText,
  }
}

/**
 * 在前端本地投影任务状态更新结果，便于立即生成续接提示和今日小结。
 */
export function applyCompanionTaskStatusLocally(
  plan: CompanionPlanDetail | null,
  taskId: number,
  status: CompanionTaskStatus,
): CompanionPlanDetail | null {
  if (!plan) {
    return null
  }

  const tasks = plan.tasks.map((item) => {
    if (item.id !== taskId) {
      return item
    }

    return {
      ...item,
      status,
      completed_at: status === 'completed' ? new Date().toISOString() : undefined,
    }
  })
  const completedTasks = tasks.filter((item) => item.status === 'completed').length
  const progress = tasks.length > 0 ? (completedTasks / tasks.length) * 100 : 0

  return {
    ...plan,
    tasks,
    completed_tasks: completedTasks,
    progress,
    status: completedTasks >= tasks.length && tasks.length > 0 ? 'completed' : 'active',
  }
}

/**
 * 将任务动作写入今日执行摘要，供入口页和房间页同步展示最新推进记录。
 */
export function recordCompanionTaskAction(
  task: CompanionPlanTask,
  status: CompanionTaskStatus,
  digest: CompanionDailyDigest | null,
): CompanionDailyDigest {
  const todayDigest = digest && digest.dateKey === buildCompanionLocalDateKey() ? digest : buildInitialCompanionDailyDigest()
  const completedTitles = todayDigest.completedTitles.filter((item) => item !== task.title)
  const skippedTitles = todayDigest.skippedTitles.filter((item) => item !== task.title)

  if (status === 'completed') {
    completedTitles.unshift(task.title)
  }
  if (status === 'skipped') {
    skippedTitles.unshift(task.title)
  }

  const nextDigest: CompanionDailyDigest = {
    ...todayDigest,
    updatedAt: Date.now(),
    completedTitles: Array.from(new Set(completedTitles)).slice(0, 8),
    skippedTitles: Array.from(new Set(skippedTitles)).slice(0, 8),
    latestActionText: `已将「${task.title}」更新为${taskStatusLabel(status)}。`,
  }
  persistCompanionDailyDigest(nextDigest)
  return nextDigest
}

/**
 * 同步写入任务续接草稿与今日执行摘要，保证入口页和房间页对推进状态的感知一致。
 */
export function persistCompanionExecutionUpdate(
  plan: CompanionPlanDetail | null,
  task: CompanionPlanTask,
  status: CompanionTaskStatus,
  source: CompanionTaskActionSource,
  digest: CompanionDailyDigest | null,
): { projectedPlan: CompanionPlanDetail | null; nextDigest: CompanionDailyDigest; nextFocusTask: CompanionFocusTaskDraft | null } {
  const projectedPlan = applyCompanionTaskStatusLocally(plan, task.id, status)
  const nextDigest = recordCompanionTaskAction(task, status, digest)
  const focusedTask = projectedPlan ? resolveFocusedCompanionTask(projectedPlan, null) : null

  if (projectedPlan && focusedTask) {
    const nextFocusTask: CompanionFocusTaskDraft = {
      planId: projectedPlan.id,
      taskId: focusedTask.id,
      title: focusedTask.title,
      status: focusedTask.status,
      source,
      updatedAt: Date.now(),
    }
    persistCompanionFocusTask(nextFocusTask)
    return {
      projectedPlan,
      nextDigest,
      nextFocusTask,
    }
  }

  clearCompanionFocusTask()
  return {
    projectedPlan,
    nextDigest,
    nextFocusTask: null,
  }
}

/**
 * 根据当前聚焦任务生成可直接发送给陪伴助手的快捷提问入口。
 */
export function buildCompanionQuickPrompts(
  plan: CompanionPlanDetail | null,
  focusedTask: CompanionPlanTask | null,
): Array<{ label: string; content: string }> {
  const phaseHint = buildCompanionPhaseAdjustmentHint(plan)
  if (!focusedTask) {
    return [
      { label: '总结今天差什么', content: '结合我当前的学习计划，帮我总结今天还差什么没完成。' },
      {
        label: '解释这轮安排',
        content: phaseHint
          ? `结合我当前计划（${phaseHint}），解释一下为什么这轮要这样安排。`
          : '结合我当前计划，解释一下为什么这轮要这样安排。',
      },
      { label: '安排今晚顺序', content: '请结合我当前计划，帮我安排今晚的推进顺序和时间分配。' },
    ]
  }

  return [
    {
      label: '拆解当前任务',
      content: phaseHint
        ? `请结合我当前计划（${phaseHint}），把「${focusedTask.title}」拆成 3 个 20 分钟内可完成的小步骤。`
        : `请把「${focusedTask.title}」拆成 3 个 20 分钟内可完成的小步骤。`,
    },
    {
      label: '为什么先做这项',
      content: phaseHint
        ? `结合我当前计划（${phaseHint}），解释为什么现在要先推进「${focusedTask.title}」，并告诉我第一步该怎么开始。`
        : `解释为什么现在要先推进「${focusedTask.title}」，并告诉我第一步该怎么开始。`,
    },
    {
      label: '总结今天差什么',
      content: phaseHint
        ? `结合我当前聚焦的「${focusedTask.title}」以及当前计划（${phaseHint}），帮我总结今天还差什么没完成。`
        : `结合我当前聚焦的「${focusedTask.title}」，帮我总结今天还差什么没完成。`,
    },
  ]
}

/**
 * 为陪伴房间生成首条续接提示，让用户打开页面后立即知道今天该接着做什么。
 */
export function buildCompanionWorkspaceResumeMessage(
  plan: CompanionPlanDetail | null,
  focusedTask: CompanionPlanTask | null,
  digest: CompanionDailyDigest | null,
): string {
  if (!plan) {
    return '还没有接入学习计划。先去入口页生成一份计划，我再陪你把今天的节奏推进下去。'
  }

  const digestText = buildCompanionDailyDigestText(plan, digest, focusedTask)
  const phaseHint = buildCompanionPhaseAdjustmentHint(plan)
  if (!focusedTask) {
    return phaseHint
      ? `当前计划是「${plan.title}」。${phaseHint}${digestText}`
      : `当前计划是「${plan.title}」。${digestText}`
  }

  return phaseHint
    ? `继续今天的推进：先盯住「${focusedTask.title}」。${phaseHint}${digestText}`
    : `继续今天的推进：先盯住「${focusedTask.title}」。${digestText}`
}

/**
 * 根据任务动作生成陪伴房间内的即时反馈文案，让状态变化不会显得生硬。
 */
export function buildCompanionTaskActionFeedback(
  task: CompanionPlanTask,
  status: CompanionTaskStatus,
  projectedPlan: CompanionPlanDetail | null,
  digest: CompanionDailyDigest | null,
): string {
  const focusedTask = resolveFocusedCompanionTask(projectedPlan, null)
  if (status === 'completed') {
    return `已记录你完成「${task.title}」。${buildCompanionDailyDigestText(projectedPlan, digest, focusedTask)}`
  }

  if (status === 'in_progress') {
    return `已开始推进「${task.title}」。先把这项任务收口，再决定是否切下一项。`
  }

  if (status === 'skipped') {
    return `已把「${task.title}」标记为跳过。${buildCompanionDailyDigestText(projectedPlan, digest, focusedTask)}`
  }

  return `已把「${task.title}」重新放回待办。${buildCompanionDailyDigestText(projectedPlan, digest, focusedTask)}`
}

/**
 * 根据任务当前状态给出下一步可操作按钮，避免陪伴页出现无意义的状态切换。
 */
export function buildTaskStatusActions(status: string): Array<{ status: CompanionTaskStatus; label: string }> {
  switch (status) {
    case 'pending':
      return [
        { status: 'in_progress', label: '开始' },
        { status: 'completed', label: '反馈后完成' },
      ]
    case 'in_progress':
      return [
        { status: 'completed', label: '反馈后完成' },
        { status: 'pending', label: '退回待办' },
        { status: 'skipped', label: '跳过' },
      ]
    case 'completed':
      return [
        { status: 'pending', label: '重新打开' },
      ]
    case 'skipped':
      return [
        { status: 'pending', label: '恢复待办' },
        { status: 'completed', label: '反馈后完成' },
      ]
    default:
      return []
  }
}

/**
 * 从计划统计中生成易读的阶段说明，帮助用户快速判断当前阻塞点。
 */
export function buildPlanProgressHint(progress: CompanionPlanProgress | undefined): string {
  if (!progress) {
    return '进度统计加载后，这里会告诉你当前最需要处理的是待办、进行中还是跳过项。'
  }

  if (progress.in_progress_tasks > 0) {
    return `当前有 ${progress.in_progress_tasks} 项任务正在推进，优先把进行中内容收口。`
  }

  if (progress.pending_tasks > 0) {
    return `还有 ${progress.pending_tasks} 项任务待开始，适合让陪伴助手帮你拆分下一步。`
  }

  if (progress.skipped_tasks > 0) {
    return `已有 ${progress.skipped_tasks} 项任务被跳过，后续可以考虑重新调整计划。`
  }

  return '这份计划当前已经没有待推进任务，可以考虑调整计划或开始新的阶段。'
}
