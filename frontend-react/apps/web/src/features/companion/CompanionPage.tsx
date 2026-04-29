import type { FormEvent } from 'react'
import { useEffect, useMemo, useRef, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link, useNavigate } from '@tanstack/react-router'
import type { Application } from 'pixi.js'
import type { Live2DModel as Cubism4Live2DModel } from 'pixi-live2d-display/cubism4'
import { extractErrorMessage } from '@makejob/api-client'
import { useAuthStore } from '../../state/auth'
import {
  DEFAULT_FRONTEND_INDUSTRY_CODE as DEFAULT_COMPANION_INDUSTRY_CODE,
  fetchFrontendIndustries as fetchCompanionIndustries,
  formatFrontendIndustryLabel as formatCompanionIndustryLabel,
  persistSelectedFrontendIndustryCode as persistSelectedCompanionIndustryCode,
  readSelectedFrontendIndustryCode as readSelectedCompanionIndustryCode,
  resolvePreferredFrontendIndustry as resolveCompanionIndustry,
  type FrontendIndustry as CompanionIndustry,
} from '../../shared/industryContext'
import { readCurrentBrowserPath } from '../../shared/authRedirect'
import {
  clearCompanionPlanContext,
  readCompanionPlanContext,
  type CompanionPlanContextDraft,
} from '../../shared/companionContext'
import { requestLoginPrompt } from '../../shared/loginPrompt'
import {
  adjustCompanionPlan,
  createCompanionPlan,
  fetchCompanionCategoryTree,
  fetchCompanionPlanProgress,
  fetchCompanionPracticeStats,
  fetchCurrentPlan,
  fetchSelectableCompanionLive2DModels,
  sendCompanionChatRequest,
  updateCompanionTaskStatus,
} from './companionApi'
import {
  buildCompanionDailyDigestText,
  buildCompanionQuickPrompts,
  buildCompanionTaskActionFeedback,
  buildCompanionWorkspaceResumeMessage,
  buildPlanProgressHint,
  buildTaskStatusActions,
  deriveActiveGoals,
  deriveTodayGoals,
  persistCompanionExecutionUpdate,
  resolveFocusedCompanionTask,
  taskStatusLabel,
} from './companionHelpers'
import {
  clearCompanionFocusTask,
  persistCompanionFocusTask,
  persistCompanionSessionSummary,
  persistSelectedCompanionModelKey,
  readCompanionDailyDigest,
  readCompanionFocusTask,
  readCompanionSessionSummary,
  readSelectedCompanionModelKey,
} from './companionStorage'
import type {
  CompanionCategoryNode,
  CompanionCategoryOption,
  CompanionChatReply,
  CompanionDailyDigest,
  CompanionFocusTaskDraft,
  CompanionGeneratePlanForm,
  CompanionGeneratePlanPayload,
  CompanionHistoryItem,
  CompanionPlanDetail,
  CompanionPlanTask,
  CompanionPracticeStats,
  CompanionSelectableLive2DModel,
  CompanionSessionSummary,
  CompanionTaskStatus,
} from './companionTypes'
import { useCompanionStudyLogSync } from './useCompanionStudyLogSync'

declare global {
  interface Window {
    PIXI?: typeof import('pixi.js')
    Live2DCubismCore?: unknown
  }
}

let cubismCoreScriptPromise: Promise<void> | null = null

/**
 * 根据当前计划和历史消息提炼出入口页需要展示的最近会话摘要。
 */
function buildCompanionSessionSummary(
  history: CompanionHistoryItem[],
  plan: CompanionPlanDetail | null,
): CompanionSessionSummary | null {
  const latestAssistantMessage = [...history].reverse().find((item) => item.role === 'assistant')
  const latestUserMessage = [...history].reverse().find((item) => item.role === 'user')

  if (!latestAssistantMessage && !plan) {
    return null
  }

  return {
    updatedAt: Date.now(),
    latestAssistantReply: latestAssistantMessage?.content || '最近还没有新的陪伴回复。',
    latestUserMessage: latestUserMessage?.content || '',
    planTitle: plan?.title || '',
    progress: Math.round(plan?.progress || 0),
  }
}

/**
 * 创建学习计划表单的默认值，避免入口页首次渲染时出现空壳状态。
 */
function buildInitialPlanForm(): CompanionGeneratePlanForm {
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
function flattenCompanionCategories(nodes: CompanionCategoryNode[], level = 0): CompanionCategoryOption[] {
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
function buildGeneratePlanPayload(form: CompanionGeneratePlanForm, industryCode: string): CompanionGeneratePlanPayload {
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
 * 将跨页带入的学习陪伴上下文合并进计划表单，尽量保留用户已经手动输入的内容。
 */
function applyCompanionPlanContextToForm(
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
 * 将陪伴上下文的推荐参数整理成入口页提示文案，帮助用户理解当前预填依据。
 */
function buildCompanionContextPresetText(draft: CompanionPlanContextDraft): string {
  return `${planLevelLabel(draft.recommendedLevel)} · ${draft.recommendedDailyStudyTime} 分钟/天 · ${draft.recommendedDurationDays} 天`
}

/**
 * 将时间值格式化为入口页可直接展示的中文时间文本。
 */
function formatCompanionDateTime(value?: string | number): string {
  if (!value) {
    return '--'
  }

  const date = typeof value === 'number' ? new Date(value) : new Date(value)
  if (Number.isNaN(date.getTime())) {
    return String(value)
  }

  return date.toLocaleString('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  })
}

/**
 * 将计划状态转换成更适合前台阅读的中文标签。
 */
function planStatusLabel(status: string): string {
  const map: Record<string, string> = {
    active: '进行中',
    paused: '已暂停',
    completed: '已完成',
    draft: '草稿',
  }

  return map[status] || status || '未定义'
}

/**
 * 将学习等级转换成前台文案，避免表单直接暴露英文枚举值。
 */
function planLevelLabel(level: string): string {
  const map: Record<string, string> = {
    beginner: '初级',
    intermediate: '中级',
    advanced: '高级',
  }

  return map[level] || level || '未设置'
}

/**
 * 从当前计划中找出最近完成的一项任务，供入口页展示最近推进记录。
 */
function deriveLatestCompletedTask(plan: CompanionPlanDetail | null): CompanionPlanTask | null {
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
function deriveUpcomingTasks(plan: CompanionPlanDetail | null): CompanionPlanTask[] {
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
function buildContinueHint(plan: CompanionPlanDetail | null, summary: CompanionSessionSummary | null): string {
  if (plan?.title) {
    return `当前已经有进行中的计划，建议直接进入学习陪伴页继续推进「${plan.title}」。`
  }

  if (summary?.latestAssistantReply) {
    return '入口页已经记住你上次的对话摘要，进入学习陪伴页后可以从上次节奏继续。'
  }

  return '如果你还没有计划，先在这里生成一份学习计划，再进入学习陪伴页开始今天的推进。'
}

/**
 * 提供学习陪伴的二级入口页，避免顶栏导航直接命中重型 Live2D 页面。
 */
export function CompanionHubPage() {
  const navigate = useNavigate()
  const accessToken = useAuthStore((state) => state.accessToken)
  const queryClient = useQueryClient()
  const hasAppliedPlanContextRef = useRef(false)
  const [sessionSummary, setSessionSummary] = useState<CompanionSessionSummary | null>(() => readCompanionSessionSummary())
  const [dailyDigest, setDailyDigest] = useState<CompanionDailyDigest | null>(() => readCompanionDailyDigest())
  const [focusTaskDraft, setFocusTaskDraft] = useState<CompanionFocusTaskDraft | null>(() => readCompanionFocusTask())
  const [planContextDraft, setPlanContextDraft] = useState<CompanionPlanContextDraft | null>(() => readCompanionPlanContext())
  const [planForm, setPlanForm] = useState<CompanionGeneratePlanForm>(() => buildInitialPlanForm())
  const [selectedIndustryCode, setSelectedIndustryCode] = useState(() => readSelectedCompanionIndustryCode() || DEFAULT_COMPANION_INDUSTRY_CODE)
  const [planFormMessage, setPlanFormMessage] = useState('先选择学习方向，再让陪伴助手生成对应计划。')
  const [taskActionTaskId, setTaskActionTaskId] = useState<number | null>(null)

  const industriesQuery = useQuery({
    queryKey: ['frontend-industries'],
    queryFn: fetchCompanionIndustries,
    staleTime: 5 * 60 * 1000,
  })

  const currentPlanQuery = useQuery({
    queryKey: ['companion-hub-current-plan', accessToken],
    queryFn: () => fetchCurrentPlan(accessToken as string),
    enabled: Boolean(accessToken),
    retry: false,
  })

  const practiceStatsQuery = useQuery({
    queryKey: ['companion-hub-practice-stats', accessToken],
    queryFn: () => fetchCompanionPracticeStats(accessToken as string),
    enabled: Boolean(accessToken),
    retry: false,
  })

  const selectedIndustry = useMemo(
    () => resolveCompanionIndustry(industriesQuery.data || [], selectedIndustryCode),
    [industriesQuery.data, selectedIndustryCode],
  )
  const effectiveIndustryCode = selectedIndustry?.code || selectedIndustryCode.trim() || DEFAULT_COMPANION_INDUSTRY_CODE
  const effectiveIndustryLabel = formatCompanionIndustryLabel(selectedIndustry, effectiveIndustryCode)
  const currentPlanIndustry = useMemo(
    () => resolveCompanionIndustry(industriesQuery.data || [], currentPlanQuery.data?.industry_code || ''),
    [currentPlanQuery.data?.industry_code, industriesQuery.data],
  )

  const categoryOptionsQuery = useQuery({
    queryKey: ['companion-hub-category-options', effectiveIndustryCode],
    queryFn: async () => flattenCompanionCategories(await fetchCompanionCategoryTree(effectiveIndustryCode)),
    enabled: Boolean(effectiveIndustryCode),
    staleTime: 5 * 60 * 1000,
  })

  const planProgressQuery = useQuery({
    queryKey: ['companion-hub-plan-progress', accessToken, currentPlanQuery.data?.id],
    queryFn: () => fetchCompanionPlanProgress(accessToken as string, currentPlanQuery.data?.id as number),
    enabled: Boolean(accessToken && currentPlanQuery.data?.id),
    retry: false,
  })

  const createPlanMutation = useMutation({
    mutationFn: (payload: CompanionGeneratePlanPayload) => createCompanionPlan(accessToken as string, payload),
    onSuccess: async (plan) => {
      clearCompanionPlanContext()
      setPlanContextDraft(null)
      setPlanForm(buildInitialPlanForm())
      if (plan.industry_code) {
        setSelectedIndustryCode(plan.industry_code)
      }
      setPlanFormMessage(`新计划已生成：${plan.title}`)

      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['companion-hub-current-plan'] }),
        queryClient.invalidateQueries({ queryKey: ['companion-hub-plan-progress'] }),
        queryClient.invalidateQueries({ queryKey: ['companion-current-plan'] }),
      ])
    },
    onError: (error) => {
      setPlanFormMessage(extractErrorMessage(error, '生成学习计划失败，请稍后重试'))
    },
  })

  const updateTaskMutation = useMutation({
    mutationFn: (payload: { task: CompanionPlanTask; status: CompanionTaskStatus }) =>
      updateCompanionTaskStatus(accessToken as string, currentPlanQuery.data?.id as number, payload.task.id, payload.status),
    onSuccess: async (_, variables) => {
      const { nextDigest, nextFocusTask } = persistCompanionExecutionUpdate(
        currentPlanQuery.data || null,
        variables.task,
        variables.status,
        'hub',
        dailyDigest,
      )
      setTaskActionTaskId(null)
      setDailyDigest(nextDigest)
      setFocusTaskDraft(nextFocusTask)
      setPlanFormMessage(`任务状态已更新为「${taskStatusLabel(variables.status)}」。`)

      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['companion-hub-current-plan'] }),
        queryClient.invalidateQueries({ queryKey: ['companion-hub-plan-progress'] }),
        queryClient.invalidateQueries({ queryKey: ['companion-current-plan'] }),
        queryClient.invalidateQueries({ queryKey: ['companion-current-plan-progress'] }),
      ])
    },
    onError: (error) => {
      setTaskActionTaskId(null)
      setPlanFormMessage(extractErrorMessage(error, '更新任务状态失败，请稍后重试'))
    },
  })

  /**
   * 当用户从独立陪伴页返回入口页时，重新读取最近一次本地会话摘要。
   */
  useEffect(() => {
    function handleFocus(): void {
      setSessionSummary(readCompanionSessionSummary())
      setDailyDigest(readCompanionDailyDigest())
      setFocusTaskDraft(readCompanionFocusTask())
    }

    handleFocus()
    window.addEventListener('focus', handleFocus)
    return () => {
      window.removeEventListener('focus', handleFocus)
    }
  }, [])

  /**
   * 在行业列表加载后统一归一化当前选中的行业编码，并写回本地缓存。
   */
  useEffect(() => {
    const normalizedIndustryCode = effectiveIndustryCode.trim()
    if (!normalizedIndustryCode) {
      return
    }

    persistSelectedCompanionIndustryCode(normalizedIndustryCode)
    if (normalizedIndustryCode !== selectedIndustryCode) {
      setSelectedIndustryCode(normalizedIndustryCode)
    }
  }, [effectiveIndustryCode, selectedIndustryCode])

  /**
   * 首次进入学习陪伴入口页时，自动把跨页带来的上下文草稿写入计划表单。
   */
  useEffect(() => {
    if (!planContextDraft || hasAppliedPlanContextRef.current) {
      return
    }

    hasAppliedPlanContextRef.current = true
    setSelectedIndustryCode(planContextDraft.industryCode || DEFAULT_COMPANION_INDUSTRY_CODE)
    setPlanForm((current) => applyCompanionPlanContextToForm(current, planContextDraft))
    setPlanFormMessage(`已根据面试报告 #${planContextDraft.interviewId || '-'} 自动带入强化计划上下文。`)
  }, [planContextDraft])

  const progressText = currentPlanQuery.data
    ? `${Math.round(currentPlanQuery.data.progress || 0)}%`
    : (sessionSummary ? `${sessionSummary.progress}%` : '--')
  const latestCompletedTask = useMemo(() => deriveLatestCompletedTask(currentPlanQuery.data || null), [currentPlanQuery.data])
  const upcomingTasks = useMemo(() => deriveUpcomingTasks(currentPlanQuery.data || null), [currentPlanQuery.data])
  const todayGoals = useMemo(() => deriveTodayGoals(currentPlanQuery.data || null), [currentPlanQuery.data])
  const focusedTask = useMemo(
    () => resolveFocusedCompanionTask(currentPlanQuery.data || null, focusTaskDraft),
    [currentPlanQuery.data, focusTaskDraft],
  )
  const dailyDigestText = useMemo(
    () => buildCompanionDailyDigestText(currentPlanQuery.data || null, dailyDigest, focusedTask),
    [currentPlanQuery.data, dailyDigest, focusedTask],
  )
  useCompanionStudyLogSync(accessToken, currentPlanQuery.data || null, dailyDigest, focusedTask)
  const continueHint = useMemo(
    () => buildContinueHint(currentPlanQuery.data || null, sessionSummary),
    [currentPlanQuery.data, sessionSummary],
  )
  const planStats = planProgressQuery.data
  const completedText = latestCompletedTask
    ? `${latestCompletedTask.title} · ${formatCompanionDateTime(latestCompletedTask.completed_at)}`
    : '还没有已完成任务记录'

  /**
   * 切换入口页当前学习方向，并清空上一方向快速勾选的弱项标签。
   */
  function handleIndustryChange(nextIndustryCode: string): void {
    setSelectedIndustryCode(nextIndustryCode)
    setPlanForm((current) => ({
      ...current,
      weakTopics: [],
    }))
    setPlanFormMessage(`已切换到 ${formatCompanionIndustryLabel(resolveCompanionIndustry(industriesQuery.data || [], nextIndustryCode), nextIndustryCode)} 方向。`)
  }

  /**
   * 切换计划表单中的弱项标签，方便快速组织 AI 生成计划的输入上下文。
   */
  function handleWeakTopicToggle(topicName: string): void {
    setPlanForm((current) => {
      const hasTopic = current.weakTopics.includes(topicName)
      return {
        ...current,
        weakTopics: hasTopic
          ? current.weakTopics.filter((item) => item !== topicName)
          : [...current.weakTopics, topicName],
      }
    })
  }

  /**
   * 从入口页直接进入陪伴房间，并把当前最值得继续的任务写入续接草稿。
   */
  function handleContinueTask(task?: CompanionPlanTask | null): void {
    const targetTask = task || focusedTask
    if (currentPlanQuery.data?.id && targetTask) {
      persistCompanionFocusTask({
        planId: currentPlanQuery.data.id,
        taskId: targetTask.id,
        title: targetTask.title,
        status: targetTask.status,
        source: 'hub',
        updatedAt: Date.now(),
      })
      setFocusTaskDraft(readCompanionFocusTask())
    }

    navigate({
      to: '/companion/room',
    })
  }

  /**
   * 在入口页直接推进今日任务，减少用户为了改状态来回切页的成本。
   */
  async function handleHubTaskStatusChange(task: CompanionPlanTask, status: CompanionTaskStatus) {
    if (!accessToken) {
      requestLoginPrompt('/companion', 'missing')
      return
    }

    if (!currentPlanQuery.data?.id) {
      setPlanFormMessage('请先登录并确保当前存在可推进的学习计划。')
      return
    }

    setTaskActionTaskId(task.id)
    setPlanFormMessage(`正在把「${task.title}」更新为「${taskStatusLabel(status)}」...`)
    try {
      await updateTaskMutation.mutateAsync({
        task,
        status,
      })
    } catch {
      if (!useAuthStore.getState().accessToken) {
        requestLoginPrompt('/companion', 'expired')
      }
    }
  }

  /**
   * 重新将当前上下文草稿带入计划表单，便于用户在修改后快速恢复推荐内容。
   */
  function handleApplyPlanContextDraft(): void {
    if (!planContextDraft) {
      return
    }

    setSelectedIndustryCode(planContextDraft.industryCode || DEFAULT_COMPANION_INDUSTRY_CODE)
    setPlanForm(() => applyCompanionPlanContextToForm(buildInitialPlanForm(), {
      ...planContextDraft,
      weakTopics: Array.from(new Set(planContextDraft.weakTopics)),
    }))
    setPlanFormMessage(`已重新带入面试报告 #${planContextDraft.interviewId || '-'} 的强化计划上下文。`)
  }

  /**
   * 清空当前学习陪伴上下文草稿，避免旧报告继续影响后续手动生成计划。
   */
  function handleClearPlanContextDraft(): void {
    clearCompanionPlanContext()
    setPlanContextDraft(null)
    setPlanFormMessage('已清除面试报告上下文，当前计划表单将按你手动填写的内容继续。')
  }

  /**
   * 提交入口页计划生成表单，并在成功后刷新当前计划与概览数据。
   */
  async function handleGeneratePlan(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()

    if (!accessToken) {
      requestLoginPrompt('/companion', 'missing')
      return
    }

    const payload = buildGeneratePlanPayload(planForm, effectiveIndustryCode)
    if (!payload.goal_description) {
      setPlanFormMessage(`先写清楚目标，例如“两周内完成 ${effectiveIndustryLabel} 方向的重点模块复习”。`)
      return
    }

    setPlanFormMessage('陪伴助手正在整理你的阶段计划...')
    try {
      await createPlanMutation.mutateAsync(payload)
    } catch {
      if (!useAuthStore.getState().accessToken) {
        requestLoginPrompt('/companion', 'expired')
      }
    }
  }

  return (
    <section className="page-panel companion-hub-panel">
      <div className="companion-hub-shell">
        <div className="companion-hub-hero">
          <div className="companion-hub-copy">
            <span className="page-tag">学习陪伴入口</span>
            <h1>先在这里整理计划，再进入学习陪伴页继续学习</h1>
            <p className="page-copy">
              顶部导航只负责带你来到学习陪伴频道，不直接加载 Live2D 主舞台。入口页现在会承接计划生成、当前进度概览和最近一次陪伴摘要，真正的重场景仍然只放在独立陪伴页。
            </p>
            <div className="hero-actions">
              <Link className="primary-button hero-link-button" to="/companion/room">
                进入学习陪伴页
              </Link>
              <a className="secondary-button hero-link-button" href="/companion/room" target="_blank" rel="noreferrer">
                新窗口打开
              </a>
            </div>

            <div className="companion-hub-metrics">
              <article className="metric-card companion-hub-metric-card">
                <strong>{currentPlanQuery.data?.title || '还没有进行中的计划'}</strong>
                <span>当前计划</span>
                <p>进度 {progressText}</p>
              </article>
              <article className="metric-card companion-hub-metric-card">
                <strong>{practiceStatsQuery.data?.streak_days ?? '--'}</strong>
                <span>连续答题天数</span>
                <p>{accessToken ? '临时复用答题统计口径' : '登录后显示'}</p>
              </article>
              <article className="metric-card companion-hub-metric-card">
                <strong>{planStats ? `${planStats.completed_tasks}/${planStats.total_tasks}` : '--'}</strong>
                <span>任务完成数</span>
                <p>{planStats ? `${Math.round(planStats.progress)}% 已推进` : '等待计划同步'}</p>
              </article>
              <article className="metric-card companion-hub-metric-card">
                <strong>{latestCompletedTask?.title || '暂无'}</strong>
                <span>最近完成任务</span>
                <p>{latestCompletedTask?.completed_at ? formatCompanionDateTime(latestCompletedTask.completed_at) : '完成后会显示'}</p>
              </article>
            </div>
          </div>

          <article className="section-card companion-hub-sidecard">
            <span className="section-kicker">继续提示</span>
            <div className="companion-hub-summary">
              <div className="timeline-item">
                <strong>入口页会记住你上次聊到哪</strong>
                <p>{continueHint}</p>
                <div className="companion-hub-meta">
                  <span>最近同步：{sessionSummary ? formatCompanionDateTime(sessionSummary.updatedAt) : '尚未写入摘要'}</span>
                  <span>{accessToken ? '已同步登录态计划' : '未登录时展示本地摘要'}</span>
                </div>
              </div>

              <div className="timeline-item">
                <strong>最近一次会话摘要</strong>
                <p>{sessionSummary?.latestAssistantReply || '还没有历史会话，第一次进入时会从陪伴助手的欢迎语开始。'}</p>
                {sessionSummary?.latestUserMessage ? (
                  <div className="companion-hub-quote">
                    <span>你上次输入：</span>
                    <strong>{sessionSummary.latestUserMessage}</strong>
                  </div>
                ) : null}
              </div>

              <div className="timeline-item">
                <strong>最近完成记录</strong>
                <p>{completedText}</p>
                <div className="companion-hub-meta">
                  <span>今日答题：{practiceStatsQuery.data?.today_count ?? '--'}</span>
                  <span>独立陪伴页只在进入时加载 Live2D</span>
                </div>
              </div>
            </div>
          </article>
        </div>

        <div className="companion-hub-board">
          <section className="status-card companion-plan-builder">
              <div className="companion-card-head">
                <div>
                  <span className="section-kicker">生成计划</span>
                  <h2>先把这阶段要学什么交给陪伴助手排出来</h2>
                </div>
              <span className="companion-card-note">当前方向：{effectiveIndustryLabel}</span>
              </div>

              <p className="companion-empty-text">
               当前已接通真实行业列表、分类筛选和计划生成链路。先选定方向，再让陪伴助手围绕这一领域拆出阶段计划和重点弱项。
              </p>

              {planContextDraft ? (
                <article className="timeline-item companion-context-card">
                  <div className="companion-card-head">
                    <div>
                      <span className="section-kicker">来自面试报告</span>
                      <h3>报告 #{planContextDraft.interviewId || '-'} 已接入学习陪伴入口</h3>
                    </div>
                    <span className="companion-card-note">{planContextDraft.industryLabel}</span>
                  </div>

                  <p>{planContextDraft.summary || '当前报告未提供总结，已优先带入低分维度和后续建议。'}</p>

                  <div className="companion-hub-meta">
                    <span>准备度：{planContextDraft.readinessLabel || '待补强'}</span>
                    <span>推荐计划：{buildCompanionContextPresetText(planContextDraft)}</span>
                  </div>

                  {planContextDraft.weakTopics.length ? (
                    <div className="community-tag-row">
                      {planContextDraft.weakTopics.map((item) => <span key={item}>{item}</span>)}
                    </div>
                  ) : null}

                  {planContextDraft.suggestions.length ? (
                    <ul className="interview-bullet-list companion-context-list">
                      {planContextDraft.suggestions.map((item) => <li key={item}>{item}</li>)}
                    </ul>
                  ) : null}

                  <div className="page-actions">
                    <button className="secondary-button" type="button" onClick={handleApplyPlanContextDraft}>
                      重新带入表单
                    </button>
                    <button className="ghost-button" type="button" onClick={handleClearPlanContextDraft}>
                      清除上下文
                    </button>
                  </div>
                </article>
              ) : null}

            <form className="stack-form companion-plan-form" onSubmit={handleGeneratePlan}>
              <div className="companion-plan-form-grid">
                <label className="field">
                  <span>当前水平</span>
                  <select
                    value={planForm.level}
                    onChange={(event) => setPlanForm((current) => ({ ...current, level: event.target.value }))}
                  >
                    <option value="beginner">{planLevelLabel('beginner')}</option>
                    <option value="intermediate">{planLevelLabel('intermediate')}</option>
                    <option value="advanced">{planLevelLabel('advanced')}</option>
                  </select>
                </label>

                <label className="field">
                  <span>每日学习时长（分钟）</span>
                  <input
                    type="number"
                    min={15}
                    max={480}
                    value={planForm.dailyStudyTime}
                    onChange={(event) => setPlanForm((current) => ({ ...current, dailyStudyTime: event.target.value }))}
                  />
                </label>

                <label className="field">
                  <span>计划周期（天）</span>
                  <input
                    type="number"
                    min={7}
                    max={90}
                    value={planForm.durationDays}
                    onChange={(event) => setPlanForm((current) => ({ ...current, durationDays: event.target.value }))}
                  />
                </label>

                <label className="field">
                  <span>当前方向</span>
                  <select
                    value={effectiveIndustryCode}
                    disabled={industriesQuery.isLoading || !industriesQuery.data?.length}
                    onChange={(event) => handleIndustryChange(event.target.value)}
                  >
                    {industriesQuery.data?.map((industry) => (
                      <option key={industry.id} value={industry.code}>
                        {industry.name}
                      </option>
                    ))}
                    {!industriesQuery.data?.length ? (
                      <option value={effectiveIndustryCode}>{effectiveIndustryLabel}</option>
                    ) : null}
                  </select>
                </label>
              </div>

              {industriesQuery.isError ? (
                <p className="companion-empty-text">
                  {extractErrorMessage(industriesQuery.error, '行业列表读取失败，当前将回退到默认方向。')}
                </p>
              ) : null}

              <label className="field">
                  <span>阶段目标</span>
                  <textarea
                    value={planForm.goalDescription}
                    onChange={(event) => setPlanForm((current) => ({ ...current, goalDescription: event.target.value }))}
                    placeholder={`例如：两周内完成 ${effectiveIndustryLabel} 方向的并发、数据库和框架复习，并能开始做中等难度题目。`}
                    rows={4}
                  />
                </label>

              <div className="field">
                <span>快速选择弱项</span>
                {categoryOptionsQuery.data?.length ? (
                  <div className="companion-topic-pills">
                    {categoryOptionsQuery.data.slice(0, 12).map((item) => (
                      <button
                        className={`companion-topic-pill ${planForm.weakTopics.includes(item.name) ? 'companion-topic-pill-active' : ''}`}
                        key={item.id}
                        type="button"
                        onClick={() => handleWeakTopicToggle(item.name)}
                      >
                        {item.name.trim()}
                      </button>
                    ))}
                  </div>
                ) : (
                  <p className="companion-empty-text">
                    {categoryOptionsQuery.isLoading ? '正在加载弱项建议...' : `当前方向 ${effectiveIndustryLabel} 暂无可用的题库分类建议。`}
                  </p>
                )}
              </div>

              <label className="field">
                <span>补充弱项（逗号或换行分隔）</span>
                <textarea
                  value={planForm.weakTopicsText}
                  onChange={(event) => setPlanForm((current) => ({ ...current, weakTopicsText: event.target.value }))}
                  placeholder="例如：性能优化，并发陷阱，SQL 索引"
                  rows={3}
                />
              </label>

              <div className="companion-composer-actions">
                <p className="companion-composer-message">
                  {planFormMessage || '填写目标后即可让陪伴助手生成一份新的学习计划。'}
                </p>
                <button className="primary-button" type="submit" disabled={!accessToken || createPlanMutation.isPending}>
                  {createPlanMutation.isPending ? '生成中...' : '生成学习计划'}
                </button>
              </div>
            </form>
          </section>

          <div className="companion-hub-main">
            <section className="status-card companion-hub-execution">
              <div className="companion-card-head">
                <div>
                  <span className="section-kicker">今日执行</span>
                  <h2>先把今天真正要推进的任务抓出来</h2>
                </div>
                <span className="companion-card-note">{focusedTask ? '已找到续接任务' : '等待任务接入'}</span>
              </div>

              <article className="timeline-item">
                <strong>{focusedTask ? `继续「${focusedTask.title}」` : '当前还没有明确续接任务'}</strong>
                <p>{dailyDigestText}</p>
                <div className="page-actions">
                  <button className="primary-button" type="button" onClick={() => handleContinueTask(focusedTask)}>
                    {focusedTask ? '进入陪伴页继续' : '进入陪伴页'}
                  </button>
                  {focusedTask ? (
                    <button className="secondary-button" type="button" onClick={() => void handleHubTaskStatusChange(focusedTask, 'completed')}>
                      直接记为完成
                    </button>
                  ) : null}
                </div>
              </article>

              <GoalList
                items={todayGoals}
                emptyText={accessToken ? '今天没有待推进任务，当前计划可能已经推进完毕。' : '登录后会在这里显示今天最该推进的任务。'}
                onStatusChange={handleHubTaskStatusChange}
                pendingTaskId={taskActionTaskId}
                onContinueTask={handleContinueTask}
              />
            </section>

            <section className="status-card companion-plan-overview">
              <div className="companion-card-head">
                <div>
                  <span className="section-kicker">当前计划概览</span>
                  <h2>{currentPlanQuery.data?.title || '还没有进行中的学习计划'}</h2>
                </div>
                <span className="companion-card-note">
                  {currentPlanQuery.data ? `${formatCompanionIndustryLabel(currentPlanIndustry, currentPlanQuery.data.industry_code || effectiveIndustryCode)} · ${planStatusLabel(currentPlanQuery.data.status)}` : '等待计划创建'}
                </span>
              </div>

              {currentPlanQuery.isError ? (
                <p className="companion-empty-text">
                  {currentPlanQuery.error instanceof Error ? currentPlanQuery.error.message : '获取当前计划失败'}
                </p>
              ) : null}

              {currentPlanQuery.data ? (
                <>
                  <p className="companion-empty-text">{currentPlanQuery.data.description || '当前计划暂未补充描述。'}</p>
                  <div className="companion-progress-block">
                    <div className="companion-progress-head">
                      <strong>总进度 {Math.round(currentPlanQuery.data.progress || 0)}%</strong>
                      <span>
                        {currentPlanQuery.data.completed_tasks}/{currentPlanQuery.data.total_tasks} 已完成
                      </span>
                    </div>
                    <div className="companion-progress-bar">
                      <div className="companion-progress-bar-fill" style={{ width: `${Math.round(currentPlanQuery.data.progress || 0)}%` }} />
                    </div>
                  </div>

                  <div className="companion-overview-stats">
                    <div className="companion-stat-chip">
                      <strong>{planStats?.completed_tasks ?? currentPlanQuery.data.completed_tasks}</strong>
                      <span>已完成</span>
                    </div>
                    <div className="companion-stat-chip">
                      <strong>{planStats?.in_progress_tasks ?? deriveActiveGoals(currentPlanQuery.data).length}</strong>
                      <span>进行中</span>
                    </div>
                    <div className="companion-stat-chip">
                      <strong>{planStats?.pending_tasks ?? upcomingTasks.length}</strong>
                      <span>待开始</span>
                    </div>
                    <div className="companion-stat-chip">
                      <strong>{planStats?.skipped_tasks ?? 0}</strong>
                      <span>已跳过</span>
                    </div>
                  </div>

                  <div className="companion-hub-tasklist">
                    <article className="timeline-item">
                      <strong>最近完成任务</strong>
                      <p>{latestCompletedTask?.title || '当前还没有已完成任务。'}</p>
                      <div className="companion-hub-meta">
                        <span>{latestCompletedTask?.completed_at ? `完成于 ${formatCompanionDateTime(latestCompletedTask.completed_at)}` : '完成后这里会显示时间'}</span>
                        <span>{latestCompletedTask ? taskStatusLabel(latestCompletedTask.status) : '等待推进'}</span>
                      </div>
                    </article>

                    <article className="timeline-item">
                      <strong>下一步建议直接推进</strong>
                      {upcomingTasks.length > 0 ? (
                        <div className="companion-upcoming-list">
                          {upcomingTasks.map((item) => (
                            <div className="companion-upcoming-item" key={item.id}>
                              <div className="companion-hub-meta">
                                <span>Day {item.day_number || 1}</span>
                                <span>{taskStatusLabel(item.status)}</span>
                              </div>
                              <p>{item.title}</p>
                            </div>
                          ))}
                        </div>
                      ) : (
                        <p>当前没有待推进任务，说明这份计划可能已经全部完成。</p>
                      )}
                    </article>
                  </div>
                </>
              ) : (
                <div className="timeline-item">
                  <strong>还没有当前计划</strong>
                  <p>先在左侧填写目标，让陪伴助手生成计划。生成成功后，这里会自动显示进度、最近完成记录和下一步任务。</p>
                </div>
              )}
            </section>
          </div>
        </div>
      </div>
    </section>
  )
}

/**
 * 创建陪伴页首屏默认消息，保证页面在未登录时也能展示完整骨架。
 */
function buildInitialHistory(): CompanionHistoryItem[] {
  return [
    {
      id: 'assistant-welcome',
      role: 'assistant',
      content: '我是你的学习陪伴助手。先把今天要推进的学习目标摆清楚，我们再一项一项完成。',
      createdAt: Date.now(),
    },
  ]
}

/**
 * 选取右侧舞台当前应该展示的台词，优先使用最近一条 Ariu 的回复。
 */
function resolveCurrentDialogue(history: CompanionHistoryItem[]): string {
  const latestAssistantMessage = [...history].reverse().find((item) => item.role === 'assistant')
  return latestAssistantMessage?.content || '我们开始吧。'
}

/**
 * 读取当前最新的情绪和动作标签，供舞台状态条使用。
 */
function resolveStageFeedback(history: CompanionHistoryItem[]): { emotion: string; action: string } {
  const latestAssistantMessage = [...history].reverse().find((item) => item.role === 'assistant')

  return {
    emotion: latestAssistantMessage?.emotion || 'steady',
    action: latestAssistantMessage?.action || 'idle',
  }
}

/**
 * 将 Live2D 模型来源转换成更直观的中文说明，方便前台展示当前命中结果。
 */
function live2DSourceLabel(source: string): string {
  if (source === 'database') {
    return '后台模型'
  }

  if (source === 'bundled') {
    return '内置回退'
  }

  return source || '未知来源'
}

/**
 * 将模型匹配类型转换为前台可读标签，方便用户理解推荐和切换范围。
 */
function live2DMatchTypeLabel(matchType: string): string {
  switch (matchType) {
    case 'industry':
      return '行业推荐'
    case 'generic':
      return '通用模型'
    case 'other':
      return '其他可选'
    case 'bundled':
      return '内置回退'
    default:
      return '可用模型'
  }
}

/**
 * 动态加载 Cubism Core 脚本，确保浏览器端具备解析 Ariu 模型的能力。
 */
function ensureCubismCoreScript(): Promise<void> {
  if (typeof window === 'undefined') {
    return Promise.resolve()
  }

  if (window.Live2DCubismCore) {
    return Promise.resolve()
  }

  if (cubismCoreScriptPromise) {
    return cubismCoreScriptPromise
  }

  cubismCoreScriptPromise = new Promise<void>((resolve, reject) => {
    const existingScript = document.querySelector<HTMLScriptElement>('script[data-live2d-cubism-core="true"]')
    if (existingScript) {
      if (window.Live2DCubismCore) {
        resolve()
        return
      }
      existingScript.addEventListener('load', () => resolve(), { once: true })
      existingScript.addEventListener('error', () => reject(new Error('Cubism Core 脚本加载失败')), { once: true })
      return
    }

    const script = document.createElement('script')
    script.src = '/live2d-assets/live2dcubismcore.min.js'
    script.async = true
    script.dataset.live2dCubismCore = 'true'
    script.onload = () => {
      if (!window.Live2DCubismCore) {
        reject(new Error('Cubism Core 已返回，但页面中未挂载 Live2DCubismCore'))
        return
      }
      resolve()
    }
    script.onerror = () => reject(new Error('Cubism Core 脚本加载失败'))
    document.head.appendChild(script)
  })

  return cubismCoreScriptPromise
}

/**
 * 按舞台容器大小重新计算 Ariu 的站位和缩放，让不同屏幕下都保持 galgame 式构图。
 */
function layoutAriuModel(model: Cubism4Live2DModel, host: HTMLDivElement, baseWidth: number, baseHeight: number): void {
  const safeBaseWidth = Math.max(baseWidth, 1)
  const safeBaseHeight = Math.max(baseHeight, 1)
  const preferredWidthScale = (host.clientWidth * 0.86) / safeBaseWidth
  const guardedHeightScale = (host.clientHeight * 1.12) / safeBaseHeight
  const scale = Math.max(Math.min(preferredWidthScale, guardedHeightScale) * 0.9, 0.1)

  model.scale.set(scale)
  model.anchor.set(0.5, 1)
  model.position.set(host.clientWidth * 0.5, host.clientHeight * 0.93)
}

/**
 * 渲染 Ariu 的 Live2D 舞台，并保持与右侧台词框联动。
 */
function AriuStage(props: {
  dialogue: string
  emotion: string
  action: string
  loggedIn: boolean
  industryCode: string
}) {
  const hostRef = useRef<HTMLDivElement | null>(null)
  const [selectedModelKey, setSelectedModelKey] = useState(() => readSelectedCompanionModelKey(props.industryCode))
  const [stageLoading, setStageLoading] = useState(false)
  const [stageError, setStageError] = useState('')

  const modelOptionsQuery = useQuery({
    queryKey: ['companion-live2d-selectable-models', props.industryCode],
    queryFn: () => fetchSelectableCompanionLive2DModels(props.industryCode),
    staleTime: 60 * 1000,
  })

  useEffect(() => {
    setSelectedModelKey(readSelectedCompanionModelKey(props.industryCode))
  }, [props.industryCode])

  const modelOptions = modelOptionsQuery.data || []
  const currentModel = useMemo(() => {
    const explicitModel = modelOptions.find((item) => item.key === selectedModelKey)
    if (explicitModel) {
      return explicitModel
    }

    return modelOptions.find((item) => item.is_recommended) || modelOptions[0] || null
  }, [modelOptions, selectedModelKey])
  const currentModelName = currentModel?.name || '陪伴助手'
  const isLoading = modelOptionsQuery.isLoading || stageLoading
  const errorMessage = modelOptionsQuery.isError
    ? extractErrorMessage(modelOptionsQuery.error, '读取陪伴页 Live2D 模型列表失败')
    : stageError
  const stageStatusText = currentModel
    ? `当前已选择 ${currentModelName} · ${live2DSourceLabel(currentModel.source)} / ${live2DMatchTypeLabel(currentModel.match_type)}`
    : (props.loggedIn ? '已连接学习计划' : '未登录，当前为展示模式')

  useEffect(() => {
    if (!currentModel?.key) {
      return
    }

    persistSelectedCompanionModelKey(props.industryCode, currentModel.key)
    if (currentModel.key !== selectedModelKey) {
      setSelectedModelKey(currentModel.key)
    }
  }, [currentModel?.key, props.industryCode, selectedModelKey])

  useEffect(() => {
    const host = hostRef.current
    if (!host || !currentModel?.model_url) {
      return undefined
    }

    let destroyed = false
    let app: Application | null = null
    let model: Cubism4Live2DModel | null = null
    let baseWidth = 1
    let baseHeight = 1

    /**
     * 同步 Pixi 画布和 Ariu 位置，避免窗口缩放后构图跑偏。
     */
    function syncStageLayout(): void {
      if (!host || !app || !model) {
        return
      }

      app.renderer.resize(host.clientWidth, host.clientHeight)
      layoutAriuModel(model, host, baseWidth, baseHeight)
    }

    /**
     * 让 Ariu 跟随指针轻微偏头，形成更接近 galgame 的陪伴感。
     */
    function handlePointerMove(event: PointerEvent): void {
      if (!model) {
        return
      }

      const rect = host.getBoundingClientRect()
      model.focus(event.clientX - rect.left, event.clientY - rect.top)
    }

    /**
     * 当鼠标离开舞台时，将 Ariu 的视线缓慢拉回中心区域。
     */
    function handlePointerLeave(): void {
      if (!model) {
        return
      }

      model.focus(host.clientWidth * 0.5, host.clientHeight * 0.58, true)
    }

    const resizeObserver = new ResizeObserver(() => {
      syncStageLayout()
    })

    /**
     * 初始化 Pixi 舞台并异步加载 Ariu 模型资源。
     */
    async function mountStage(): Promise<void> {
      setStageLoading(true)
      setStageError('')

      try {
        const PIXI = await import('pixi.js')
        window.PIXI = PIXI
        await ensureCubismCoreScript()
        const { Live2DModel } = await import('pixi-live2d-display/cubism4')

        app = new PIXI.Application({
          width: Math.max(host.clientWidth, 320),
          height: Math.max(host.clientHeight, 320),
          autoStart: true,
          backgroundAlpha: 0,
          antialias: true,
          autoDensity: true,
          resolution: Math.min(window.devicePixelRatio || 1, 2),
        })

        host.replaceChildren(app.view as HTMLCanvasElement)
        resizeObserver.observe(host)
        host.addEventListener('pointermove', handlePointerMove)
        host.addEventListener('pointerleave', handlePointerLeave)

        model = await Live2DModel.from(currentModel.model_url)
        if (destroyed || !app) {
          model.destroy()
          return
        }

        baseWidth = Math.max(model.width, 1)
        baseHeight = Math.max(model.height, 1)
        app.stage.addChild(model)
        syncStageLayout()
        model.focus(host.clientWidth * 0.5, host.clientHeight * 0.58, true)
        setStageLoading(false)
      } catch (stageError) {
        if (destroyed) {
          return
        }

        setStageError(stageError instanceof Error ? stageError.message : 'Live2D 模型加载失败')
        setStageLoading(false)
      }
    }

    void mountStage()

    return () => {
      destroyed = true
      resizeObserver.disconnect()
      host.removeEventListener('pointermove', handlePointerMove)
      host.removeEventListener('pointerleave', handlePointerLeave)

      if (model && app) {
        app.stage.removeChild(model)
        model.destroy()
      }

      app?.destroy(true)
    }
  }, [currentModel?.model_url])

  return (
    <section className="companion-stage-panel">
      <div className="companion-stage-topbar">
        <div className="companion-stage-badges">
          <span className="page-tag">当前模型：{currentModelName}</span>
          <span className="companion-state-pill">情绪：{props.emotion}</span>
          <span className="companion-state-pill">动作：{props.action}</span>
        </div>
        <div className="companion-stage-side">
          <span className="companion-stage-copy">{stageStatusText}</span>
          {modelOptions.length > 1 ? (
            <label className="companion-stage-selector">
              <span>切换模型</span>
              <select
                value={currentModel?.key || ''}
                onChange={(event) => setSelectedModelKey(event.target.value)}
              >
                {modelOptions.map((item) => (
                  <option key={item.key} value={item.key}>
                    {`${item.name} · ${live2DMatchTypeLabel(item.match_type)}`}
                  </option>
                ))}
              </select>
            </label>
          ) : null}
        </div>
      </div>

      <div className="companion-stage-canvas-wrap">
        <div className="companion-stage-canvas" ref={hostRef} />

        {isLoading ? (
          <div className="companion-stage-overlay">
            <strong>{modelOptionsQuery.isLoading ? '正在读取可用模型' : `正在加载 ${currentModelName}`}</strong>
            <span>
              {modelOptionsQuery.isLoading
                ? '前台正在读取当前场景下的可切换 Live2D 模型。'
                : '模型资源加载中，请稍等片刻。'}
            </span>
          </div>
        ) : null}

        {errorMessage ? (
          <div className="companion-stage-overlay companion-stage-overlay-error">
            <strong>模型加载失败</strong>
            <span>{errorMessage}</span>
          </div>
        ) : null}

        <div className="companion-stage-dialogue">
          <span className="section-kicker">{currentModelName}</span>
          <p>{props.dialogue}</p>
        </div>
      </div>
    </section>
  )
}

/**
 * 渲染单个目标卡片中的任务列表，统一展示标题、类型和状态。
 */
function GoalList(props: {
  items: CompanionPlanTask[]
  emptyText: string
  onStatusChange?: (task: CompanionPlanTask, status: CompanionTaskStatus) => void
  pendingTaskId?: number | null
  onContinueTask?: (task: CompanionPlanTask) => void
}) {
  if (props.items.length === 0) {
    return <p className="companion-empty-text">{props.emptyText}</p>
  }

  return (
    <div className="companion-goal-list">
      {props.items.map((item) => (
        <article className="companion-goal-item" key={item.id}>
          <div className="companion-goal-head">
            <strong>{item.title}</strong>
            <span>{taskStatusLabel(item.status)}</span>
          </div>
          <p>{item.description || '当前任务暂无详细说明。'}</p>
          <div className="companion-goal-meta">
            <span>类型：{item.task_type || 'study'}</span>
            <span>Day {item.day_number || 1}</span>
          </div>
          {props.onContinueTask ? (
            <div className="card-inline">
              <button className="ghost-button" type="button" onClick={() => props.onContinueTask?.(item)}>
                围绕这项继续
              </button>
            </div>
          ) : null}
          {props.onStatusChange ? (
            <div className="companion-task-actions">
              {buildTaskStatusActions(item.status).map((action) => (
                <button
                  className="secondary-button companion-task-button"
                  key={`${item.id}-${action.status}`}
                  type="button"
                  disabled={props.pendingTaskId === item.id}
                  onClick={() => props.onStatusChange?.(item, action.status)}
                >
                  {props.pendingTaskId === item.id ? '提交中...' : action.label}
                </button>
              ))}
            </div>
          ) : null}
        </article>
      ))}
    </div>
  )
}

/**
 * 提供学习陪伴核心页面，整合 Ariu 舞台、计划侧栏和聊天输入区。
 */
export function CompanionWorkspacePage() {
  const navigate = useNavigate()
  const accessToken = useAuthStore((state) => state.accessToken)
  const user = useAuthStore((state) => state.user)
  const queryClient = useQueryClient()
  const hasInjectedResumeMessageRef = useRef(false)
  const [history, setHistory] = useState<CompanionHistoryItem[]>(() => buildInitialHistory())
  const [composer, setComposer] = useState('')
  const [sending, setSending] = useState(false)
  const [composerMessage, setComposerMessage] = useState('')
  const [taskActionTaskId, setTaskActionTaskId] = useState<number | null>(null)
  const [planActionMessage, setPlanActionMessage] = useState('')
  const [preferredIndustryCode, setPreferredIndustryCode] = useState(() => readSelectedCompanionIndustryCode() || DEFAULT_COMPANION_INDUSTRY_CODE)
  const [dailyDigest, setDailyDigest] = useState<CompanionDailyDigest | null>(() => readCompanionDailyDigest())
  const [focusTaskDraft, setFocusTaskDraft] = useState<CompanionFocusTaskDraft | null>(() => readCompanionFocusTask())

  const industriesQuery = useQuery({
    queryKey: ['frontend-industries'],
    queryFn: fetchCompanionIndustries,
    staleTime: 5 * 60 * 1000,
  })

  const currentPlanQuery = useQuery({
    queryKey: ['companion-current-plan', accessToken],
    queryFn: () => fetchCurrentPlan(accessToken as string),
    enabled: Boolean(accessToken),
    retry: false,
  })

  const planProgressQuery = useQuery({
    queryKey: ['companion-current-plan-progress', accessToken, currentPlanQuery.data?.id],
    queryFn: () => fetchCompanionPlanProgress(accessToken as string, currentPlanQuery.data?.id as number),
    enabled: Boolean(accessToken && currentPlanQuery.data?.id),
    retry: false,
  })

  const todayGoals = useMemo(() => deriveTodayGoals(currentPlanQuery.data || null), [currentPlanQuery.data])
  const activeGoals = useMemo(() => deriveActiveGoals(currentPlanQuery.data || null), [currentPlanQuery.data])
  const focusedTask = useMemo(
    () => resolveFocusedCompanionTask(currentPlanQuery.data || null, focusTaskDraft),
    [currentPlanQuery.data, focusTaskDraft],
  )
  const dailyDigestText = useMemo(
    () => buildCompanionDailyDigestText(currentPlanQuery.data || null, dailyDigest, focusedTask),
    [currentPlanQuery.data, dailyDigest, focusedTask],
  )
  useCompanionStudyLogSync(accessToken, currentPlanQuery.data || null, dailyDigest, focusedTask)
  const quickPrompts = useMemo(() => buildCompanionQuickPrompts(focusedTask), [focusedTask])
  const currentDialogue = useMemo(() => resolveCurrentDialogue(history), [history])
  const stageFeedback = useMemo(() => resolveStageFeedback(history), [history])
  const planProgressHint = useMemo(() => buildPlanProgressHint(planProgressQuery.data), [planProgressQuery.data])
  const workspaceIndustryCode = currentPlanQuery.data?.industry_code?.trim() || preferredIndustryCode.trim() || DEFAULT_COMPANION_INDUSTRY_CODE
  const workspaceIndustry = useMemo(
    () => resolveCompanionIndustry(industriesQuery.data || [], workspaceIndustryCode),
    [industriesQuery.data, workspaceIndustryCode],
  )
  const workspaceIndustryLabel = formatCompanionIndustryLabel(workspaceIndustry, workspaceIndustryCode)

  const updateTaskMutation = useMutation({
    mutationFn: (payload: { task: CompanionPlanTask; status: CompanionTaskStatus }) =>
      updateCompanionTaskStatus(accessToken as string, currentPlanQuery.data?.id as number, payload.task.id, payload.status),
    onSuccess: async (_, variables) => {
      const { projectedPlan, nextDigest, nextFocusTask } = persistCompanionExecutionUpdate(
        currentPlanQuery.data || null,
        variables.task,
        variables.status,
        'room',
        dailyDigest,
      )
      setTaskActionTaskId(null)
      setDailyDigest(nextDigest)
      setFocusTaskDraft(nextFocusTask)
      setPlanActionMessage(`任务状态已更新为「${taskStatusLabel(variables.status)}」。`)
      setHistory((current) => [
        ...current,
        {
          id: `assistant-task-${Date.now()}`,
          role: 'assistant',
          content: buildCompanionTaskActionFeedback(variables.task, variables.status, projectedPlan, nextDigest),
          emotion: variables.status === 'completed' ? 'encouraging' : 'steady',
          action: variables.status === 'completed' ? 'celebrate' : 'nod',
          createdAt: Date.now(),
        },
      ])
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['companion-current-plan'] }),
        queryClient.invalidateQueries({ queryKey: ['companion-current-plan-progress'] }),
        queryClient.invalidateQueries({ queryKey: ['companion-hub-current-plan'] }),
        queryClient.invalidateQueries({ queryKey: ['companion-hub-plan-progress'] }),
      ])
    },
    onError: (error) => {
      setTaskActionTaskId(null)
      setPlanActionMessage(extractErrorMessage(error, '更新任务状态失败，请稍后重试'))
    },
  })

  const adjustPlanMutation = useMutation({
    mutationFn: () => adjustCompanionPlan(accessToken as string, currentPlanQuery.data?.id as number),
    onSuccess: async (plan) => {
      setPlanActionMessage(`计划已调整：${plan.title}`)
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['companion-current-plan'] }),
        queryClient.invalidateQueries({ queryKey: ['companion-current-plan-progress'] }),
        queryClient.invalidateQueries({ queryKey: ['companion-hub-current-plan'] }),
        queryClient.invalidateQueries({ queryKey: ['companion-hub-plan-progress'] }),
      ])
    },
    onError: (error) => {
      setPlanActionMessage(extractErrorMessage(error, '调整学习计划失败，请稍后重试'))
    },
  })

  /**
   * 将当前陪伴页的最新状态持续写回入口页可读的会话摘要缓存。
   */
  useEffect(() => {
    const summary = buildCompanionSessionSummary(history, currentPlanQuery.data || null)
    if (!summary) {
      return
    }

    persistCompanionSessionSummary(summary)
  }, [history, currentPlanQuery.data])

  /**
   * 当房间拿到当前计划后，自动注入一条续接提示，避免每次进入都像从零开始。
   */
  useEffect(() => {
    if (hasInjectedResumeMessageRef.current || !accessToken || !currentPlanQuery.data) {
      return
    }

    hasInjectedResumeMessageRef.current = true
    setHistory((current) => [
      ...current,
      {
        id: `assistant-resume-${currentPlanQuery.data.id}`,
        role: 'assistant',
        content: buildCompanionWorkspaceResumeMessage(currentPlanQuery.data, focusedTask, dailyDigest),
        emotion: 'steady',
        action: 'nod',
        createdAt: Date.now(),
      },
    ])
  }, [accessToken, currentPlanQuery.data, dailyDigest, focusedTask])

  /**
   * 当当前计划或入口页偏好发生变化时，同步持久化当前陪伴场景应使用的行业上下文。
   */
  useEffect(() => {
    if (!workspaceIndustryCode) {
      return
    }

    persistSelectedCompanionIndustryCode(workspaceIndustryCode)
    if (workspaceIndustryCode !== preferredIndustryCode) {
      setPreferredIndustryCode(workspaceIndustryCode)
    }
  }, [preferredIndustryCode, workspaceIndustryCode])

  /**
   * 当房间识别到新的聚焦任务时，同步更新本地续接草稿，保证返回入口页仍能接上当前任务。
   */
  useEffect(() => {
    if (!currentPlanQuery.data?.id || !focusedTask) {
      clearCompanionFocusTask()
      setFocusTaskDraft(null)
      return
    }

    const nextDraft: CompanionFocusTaskDraft = {
      planId: currentPlanQuery.data.id,
      taskId: focusedTask.id,
      title: focusedTask.title,
      status: focusedTask.status,
      source: 'room',
      updatedAt: Date.now(),
    }
    persistCompanionFocusTask(nextDraft)
    setFocusTaskDraft(nextDraft)
  }, [currentPlanQuery.data?.id, focusedTask])

  /**
   * 在陪伴页直接切换任务状态，让计划推进不必退回入口页操作。
   */
  async function handleTaskStatusChange(task: CompanionPlanTask, status: CompanionTaskStatus) {
    if (!accessToken) {
      requestLoginPrompt('/companion/room', 'missing')
      return
    }

    if (!currentPlanQuery.data?.id) {
      setPlanActionMessage('请先登录并确保当前存在可操作的学习计划。')
      return
    }

    setTaskActionTaskId(task.id)
    setPlanActionMessage(`正在把「${task.title}」更新为「${taskStatusLabel(status)}」...`)
    try {
      await updateTaskMutation.mutateAsync({
        task,
        status,
      })
    } catch {
      if (!useAuthStore.getState().accessToken) {
        requestLoginPrompt('/companion/room', 'expired')
      }
    }
  }

  /**
   * 触发后端动态调整计划，适合在任务阻塞或节奏需要重排时使用。
   */
  async function handleAdjustPlan() {
    if (!accessToken) {
      requestLoginPrompt('/companion/room', 'missing')
      return
    }

    if (!currentPlanQuery.data?.id) {
      setPlanActionMessage('请先登录并生成学习计划后再调整。')
      return
    }

    setPlanActionMessage('陪伴助手正在重新整理你的计划节奏...')
    try {
      await adjustPlanMutation.mutateAsync()
    } catch {
      if (!useAuthStore.getState().accessToken) {
        requestLoginPrompt('/companion/room', 'expired')
      }
    }
  }

  /**
   * 处理用户输入并在必要时请求后端陪伴接口，完成 Ariu 的一轮回复。
   */
  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()

    const content = composer.trim()
    if (!content) {
      setComposerMessage('先输入你想让陪伴助手帮你推进的内容。')
      return
    }

    if (!accessToken) {
      requestLoginPrompt('/companion/room', 'missing')
      return
    }

    const userMessage: CompanionHistoryItem = {
      id: `user-${Date.now()}`,
      role: 'user',
      content,
      createdAt: Date.now(),
    }

    setComposer('')
    setComposerMessage('')
    setHistory((current) => [...current, userMessage])

    setSending(true)

    try {
      const reply = await sendCompanionChatRequest(
        accessToken,
        [...history, userMessage],
        currentPlanQuery.data || null,
        focusedTask,
        dailyDigest,
        {
          deriveTodayGoals,
          deriveActiveGoals,
        },
      )
      const replyContent = reply.reply || reply.content || '我在，你继续说。'

      setHistory((current) => [
        ...current,
        {
          id: `assistant-${Date.now()}`,
          role: 'assistant',
          content: replyContent,
          emotion: reply.emotion || reply.mood || '',
          action: reply.action || '',
          createdAt: Date.now(),
        },
        ])
    } catch (error) {
      if (!useAuthStore.getState().accessToken) {
        requestLoginPrompt(readCurrentBrowserPath(), 'expired')
        return
      }
      setComposerMessage(extractErrorMessage(error, '陪伴助手暂时没接上服务，请稍后重试'))
    } finally {
      setSending(false)
    }
  }

  /**
   * 将快捷提问模板写入输入框，帮助用户围绕当前任务更快开始一轮推进。
   */
  function handleApplyQuickPrompt(content: string): void {
    setComposer(content)
    setComposerMessage('已带入快捷提问，你可以直接发送，也可以再补充细节。')
  }

  return (
    <section className="page-panel companion-page-panel">
      <div className="companion-room-toolbar">
        <Link className="ghost-button companion-room-back" to="/companion">
          返回陪伴入口
        </Link>
        <span className="companion-room-note">当前为学习陪伴独立页 · {workspaceIndustryLabel}</span>
      </div>

      <div className="companion-layout">
        <aside className="companion-sidebar">
          <div className="companion-sidebar-head">
            <span className="page-tag">学习陪伴</span>
            <h1>{user?.username ? `${user.username} 的学习陪伴页` : '学习陪伴页'}</h1>
            <p className="page-copy">
              左侧专门放今天要推进的目标与完整对话记录，右侧保留角色舞台，并支持在当前场景下切换可用模型。
            </p>
          </div>

          <article className="status-card companion-progress-card">
            <div className="companion-card-head">
              <div>
                <span className="section-kicker">计划进度</span>
                <h2>{currentPlanQuery.data?.title || '等待计划接入'}</h2>
              </div>
              <span className="companion-card-note">
                {currentPlanQuery.data ? `${Math.round(currentPlanQuery.data.progress || 0)}%` : '--'}
              </span>
            </div>
            <div className="companion-progress-block">
              <div className="companion-progress-head">
                <strong>当前任务推进情况</strong>
                <span>
                  {planProgressQuery.data?.completed_tasks ?? currentPlanQuery.data?.completed_tasks ?? 0}
                  /
                  {planProgressQuery.data?.total_tasks ?? currentPlanQuery.data?.total_tasks ?? 0}
                </span>
              </div>
              <div className="companion-progress-bar">
                <div className="companion-progress-bar-fill" style={{ width: `${Math.round(planProgressQuery.data?.progress ?? currentPlanQuery.data?.progress ?? 0)}%` }} />
              </div>
            </div>
            <div className="interview-overview-stats">
              <div className="companion-stat-chip">
                <strong>{planProgressQuery.data?.completed_tasks ?? 0}</strong>
                <span>已完成</span>
              </div>
              <div className="companion-stat-chip">
                <strong>{planProgressQuery.data?.in_progress_tasks ?? 0}</strong>
                <span>进行中</span>
              </div>
              <div className="companion-stat-chip">
                <strong>{planProgressQuery.data?.pending_tasks ?? 0}</strong>
                <span>待开始</span>
              </div>
              <div className="companion-stat-chip">
                <strong>{planProgressQuery.data?.skipped_tasks ?? 0}</strong>
                <span>已跳过</span>
              </div>
            </div>
            <p className="companion-empty-text">{planActionMessage || planProgressHint}</p>
            <div className="page-actions">
              <button
                className="secondary-button"
                type="button"
                disabled={!currentPlanQuery.data?.id || adjustPlanMutation.isPending}
                onClick={() => void handleAdjustPlan()}
              >
                {adjustPlanMutation.isPending ? '调整中...' : '重新调整计划'}
              </button>
            </div>
          </article>

          <article className="status-card companion-goal-card">
            <div className="companion-card-head">
              <div>
                <span className="section-kicker">当前续接</span>
                <h2>{focusedTask ? focusedTask.title : '等待任务接入'}</h2>
              </div>
              <span className="companion-card-note">{focusedTask ? taskStatusLabel(focusedTask.status) : '暂无聚焦任务'}</span>
            </div>
            <p className="companion-empty-text">
              {focusedTask?.description || dailyDigestText}
            </p>
            <div className="companion-hub-meta">
              <span>{dailyDigestText}</span>
              {focusTaskDraft?.updatedAt ? <span>最近续接：{formatCompanionDateTime(focusTaskDraft.updatedAt)}</span> : null}
            </div>
            <div className="companion-quick-actions">
              {quickPrompts.map((item) => (
                <button className="secondary-button" key={item.label} type="button" onClick={() => handleApplyQuickPrompt(item.content)}>
                  {item.label}
                </button>
              ))}
            </div>
          </article>

          <article className="status-card companion-goal-card">
            <div className="companion-card-head">
              <div>
                <span className="section-kicker">今日目标</span>
                <h2>今天优先推进这些内容</h2>
              </div>
              <strong>{currentPlanQuery.data ? `${Math.round(currentPlanQuery.data.progress || 0)}%` : '--'}</strong>
            </div>
              <GoalList
                items={todayGoals}
                emptyText={accessToken ? '当前没有识别到今日目标，可以先去生成学习计划。' : '登录后会自动同步你的今日学习目标。'}
                onStatusChange={handleTaskStatusChange}
                pendingTaskId={taskActionTaskId}
                onContinueTask={(task) => handleApplyQuickPrompt(`请帮我继续推进「${task.title}」，先告诉我下一步最该做什么。`)}
              />
            </article>

          <article className="status-card companion-goal-card">
            <div className="companion-card-head">
              <div>
                <span className="section-kicker">进行目标</span>
                <h2>当前最值得盯住的任务</h2>
              </div>
              <span className="companion-card-note">{currentPlanQuery.data?.title || '等待计划接入'}</span>
            </div>
              <GoalList
                items={activeGoals}
                emptyText={accessToken ? '当前没有进行中的任务，陪伴助手会把下一项未完成目标顶上来。' : '登录后会显示你当前正在推进的任务。'}
                onStatusChange={handleTaskStatusChange}
                pendingTaskId={taskActionTaskId}
                onContinueTask={(task) => handleApplyQuickPrompt(`围绕「${task.title}」这项任务，帮我安排接下来的推进顺序。`)}
              />
            </article>

          <article className="status-card companion-history-card">
            <div className="companion-card-head">
              <div>
                <span className="section-kicker">对话历史</span>
                <h2>完整记录保留在这里</h2>
              </div>
              <span className="companion-card-note">{history.length} 条</span>
            </div>
            <div className="companion-history-list">
              {history.map((item) => (
                <div className={`companion-history-item companion-history-item-${item.role}`} key={item.id}>
                  <div className="companion-history-head">
                    <strong>{item.role === 'assistant' ? '陪伴助手' : '你'}</strong>
                    <span>{new Date(item.createdAt).toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' })}</span>
                  </div>
                  <p>{item.content}</p>
                </div>
              ))}
            </div>
          </article>
        </aside>

        <div className="companion-stage-shell">
          <AriuStage
            dialogue={currentDialogue}
            emotion={stageFeedback.emotion}
            action={stageFeedback.action}
            loggedIn={Boolean(accessToken)}
            industryCode={workspaceIndustryCode}
          />

          <section className="status-card companion-input-panel">
            <div className="companion-card-head">
              <div>
                <span className="section-kicker">输入区</span>
                <h2>直接让陪伴助手帮你拆解问题</h2>
              </div>
              <span className="companion-card-note">{sending ? '陪伴助手思考中…' : 'Enter 发送'}</span>
            </div>

            <form className="companion-composer" onSubmit={handleSubmit}>
              <textarea
                value={composer}
                onChange={(event) => setComposer(event.target.value)}
                placeholder={focusedTask ? `例如：帮我继续推进「${focusedTask.title}」，或者总结一下今天还差什么没完成。` : '例如：帮我安排今晚的 Go 并发复习顺序，或者总结一下今天还差什么没完成。'}
                rows={4}
              />
              <div className="companion-quick-actions">
                {quickPrompts.map((item) => (
                  <button className="secondary-button" key={item.label} type="button" onClick={() => handleApplyQuickPrompt(item.content)}>
                    {item.label}
                  </button>
                ))}
              </div>
              <div className="companion-composer-actions">
                <p className="companion-composer-message">
                  {composerMessage || (accessToken ? '已登录，可直接使用 AI 陪伴接口。' : '未登录时会显示本地提示，不会请求后端陪伴接口。')}
                </p>
                <button className="primary-button" type="submit" disabled={sending}>
                  {sending ? '发送中...' : '发送给陪伴助手'}
                </button>
              </div>
            </form>
          </section>
        </div>
      </div>
    </section>
  )
}

export default CompanionWorkspacePage
