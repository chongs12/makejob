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
  CompanionFocusTaskDraft,
  CompanionPlanDetail,
  CompanionPlanProgress,
  CompanionPlanTask,
  CompanionStudyLogPayload,
  CompanionTaskActionSource,
  CompanionTaskStatus,
} from './companionTypes'

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
export function buildCompanionQuickPrompts(focusedTask: CompanionPlanTask | null): Array<{ label: string; content: string }> {
  if (!focusedTask) {
    return [
      { label: '总结今天差什么', content: '结合我当前的学习计划，帮我总结今天还差什么没完成。' },
      { label: '安排今晚顺序', content: '请结合我当前计划，帮我安排今晚的推进顺序和时间分配。' },
    ]
  }

  return [
    { label: '拆解当前任务', content: `请把「${focusedTask.title}」拆成 3 个 20 分钟内可完成的小步骤。` },
    { label: '我卡住了', content: `我现在卡在「${focusedTask.title}」这项任务上了，请帮我定位阻塞点并给出下一步。` },
    { label: '总结今天差什么', content: `结合我当前聚焦的「${focusedTask.title}」，帮我总结今天还差什么没完成。` },
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
  if (!focusedTask) {
    return `当前计划是「${plan.title}」。${digestText}`
  }

  return `继续今天的推进：先盯住「${focusedTask.title}」。${digestText}`
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
        { status: 'completed', label: '直接完成' },
      ]
    case 'in_progress':
      return [
        { status: 'completed', label: '标记完成' },
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
        { status: 'completed', label: '记为完成' },
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
