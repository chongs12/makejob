import type { FormEvent } from 'react'
import { useEffect, useMemo, useRef, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link, useNavigate, useRouter } from '@tanstack/react-router'
import { extractErrorMessage } from '@makejob/api-client'
import { useAuthStore } from '../../state/auth'
import {
  DEFAULT_FRONTEND_INDUSTRY_CODE as DEFAULT_COMPANION_INDUSTRY_CODE,
  formatFrontendIndustryLabel as formatCompanionIndustryLabel,
  persistSelectedFrontendIndustryCode as persistSelectedCompanionIndustryCode,
  readSelectedFrontendIndustryCode as readSelectedCompanionIndustryCode,
  resolvePreferredFrontendIndustry as resolveCompanionIndustry,
} from '../../shared/industryContext'
import {
  clearCompanionPlanContext,
  readCompanionPlanContext,
  type CompanionPlanContextDraft,
} from '../../shared/companionContext'
import { useFrontendIndustriesQuery, usePracticeStatsQuery } from '../../shared/frontendQueries'
import { requestLoginPrompt } from '../../shared/loginPrompt'
import { fetchMistakeTopics } from '../../shared/mistakeTopics'
import {
  buildCompanionCategoryOptionsQueryKey,
  buildCompanionCurrentPlanQueryKey,
  buildCompanionPlanProgressQueryKey,
  buildCompanionWeeklyFocusQueryKey,
  invalidateCompanionPlanQueries,
} from '../../shared/queryKeys'
import { buildWeeklyFocusPracticeRouteSearch } from '../../shared/practiceRoute'
import { fetchWeeklyFocus, type WeeklyFocusTheme } from '../../shared/weeklyFocus'
import {
  createCompanionPlan,
  fetchCompanionCategoryTree,
  fetchCompanionPlanProgress,
  fetchCurrentPlan,
  updateCompanionTaskStatus,
} from './companionApi'
import {
  buildCompanionDailyDigestText,
  deriveActiveGoals,
  deriveTodayGoals,
  persistCompanionExecutionUpdate,
  resolveFocusedCompanionTask,
  taskStatusLabel,
} from './companionHelpers'
import {
  applyCompanionPlanContextToForm,
  applyWeeklyFocusToPlanForm,
  buildCompanionContextPresetText,
  buildContinueHint,
  buildGeneratePlanPayload,
  buildInitialPlanForm,
  deriveLatestCompletedTask,
  deriveUpcomingTasks,
  flattenCompanionCategories,
  planLevelLabel,
  planStatusLabel,
} from './companionHubHelpers'
import { buildCompanionSessionSummary, formatCompanionDateTime, GoalList } from './companionShared'
import {
  persistCompanionFocusTask,
  readCompanionDailyDigest,
  readCompanionFocusTask,
  readCompanionSessionSummary,
} from './companionStorage'
import type {
  CompanionDailyDigest,
  CompanionFocusTaskDraft,
  CompanionGeneratePlanForm,
  CompanionGeneratePlanPayload,
  CompanionPlanTask,
  CompanionSessionSummary,
  CompanionTaskStatus,
} from './companionTypes'
import { useCompanionStudyLogSync } from './useCompanionStudyLogSync'

/**
 * 在入口页当前计划概览仍在同步时渲染固定骨架，减少右侧概览区布局抖动。
 */
function CompanionHubPlanOverviewSkeleton() {
  return (
    <section className="status-card companion-plan-overview">
      <div className="companion-card-head">
        <div className="companion-skeleton-block companion-skeleton-block-stack">
          <span className="companion-skeleton-line companion-skeleton-line-short" />
          <span className="companion-skeleton-line companion-skeleton-line-medium" />
        </div>
        <span className="companion-skeleton-pill" />
      </div>
      <p className="companion-empty-text">正在同步当前计划概览...</p>
      <div className="companion-progress-block">
        <div className="companion-progress-head">
          <span className="companion-skeleton-line companion-skeleton-line-medium" />
          <span className="companion-skeleton-line companion-skeleton-line-short" />
        </div>
        <div className="companion-progress-bar companion-progress-bar-skeleton">
          <div className="companion-progress-bar-fill companion-progress-bar-fill-skeleton" />
        </div>
      </div>
      <div className="companion-overview-stats">
        {Array.from({ length: 4 }).map((_, index) => (
          <div className="companion-stat-chip companion-stat-chip-skeleton" key={index}>
            <span className="companion-skeleton-line companion-skeleton-line-short" />
            <span className="companion-skeleton-line companion-skeleton-line-medium" />
          </div>
        ))}
      </div>
    </section>
  )
}

/**
 * 提供学习陪伴的二级入口页，避免顶栏导航直接命中重型 Live2D 页面。
 */
export function CompanionHubPage() {
  const navigate = useNavigate()
  const router = useRouter()
  const accessToken = useAuthStore((state) => state.accessToken)
  const queryClient = useQueryClient()
  const hasAppliedPlanContextRef = useRef(false)
  const hasAutoAppliedWeeklyFocusRef = useRef(false)
  const [sessionSummary, setSessionSummary] = useState<CompanionSessionSummary | null>(() => readCompanionSessionSummary())
  const [dailyDigest, setDailyDigest] = useState<CompanionDailyDigest | null>(() => readCompanionDailyDigest())
  const [focusTaskDraft, setFocusTaskDraft] = useState<CompanionFocusTaskDraft | null>(() => readCompanionFocusTask())
  const [planContextDraft, setPlanContextDraft] = useState<CompanionPlanContextDraft | null>(() => readCompanionPlanContext())
  const [planForm, setPlanForm] = useState<CompanionGeneratePlanForm>(() => buildInitialPlanForm())
  const [selectedIndustryCode, setSelectedIndustryCode] = useState(() => readSelectedCompanionIndustryCode() || DEFAULT_COMPANION_INDUSTRY_CODE)
  const [planFormMessage, setPlanFormMessage] = useState('先选择学习方向，再让陪伴助手生成对应计划。')
  const [taskActionTaskId, setTaskActionTaskId] = useState<number | null>(null)

  const industriesQuery = useFrontendIndustriesQuery()

  const currentPlanQuery = useQuery({
    queryKey: buildCompanionCurrentPlanQueryKey(accessToken),
    queryFn: () => fetchCurrentPlan(accessToken as string),
    enabled: Boolean(accessToken),
    retry: false,
  })

  const practiceStatsQuery = usePracticeStatsQuery(accessToken)

  const weeklyFocusQuery = useQuery({
    queryKey: buildCompanionWeeklyFocusQueryKey(accessToken),
    queryFn: () => fetchWeeklyFocus(accessToken as string),
    enabled: Boolean(accessToken),
    retry: false,
  })
  const mistakeTopicsQuery = useQuery({
    queryKey: ['companion-weekly-focus-topics'],
    queryFn: () => fetchMistakeTopics([]),
    enabled: Boolean(weeklyFocusQuery.data?.themes.length),
    staleTime: 5 * 60 * 1000,
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
    queryKey: buildCompanionCategoryOptionsQueryKey(effectiveIndustryCode),
    queryFn: async () => flattenCompanionCategories(await fetchCompanionCategoryTree(effectiveIndustryCode)),
    enabled: Boolean(effectiveIndustryCode),
    staleTime: 5 * 60 * 1000,
  })

  const planProgressQuery = useQuery({
    queryKey: buildCompanionPlanProgressQueryKey(accessToken, currentPlanQuery.data?.id),
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
      await invalidateCompanionPlanQueries(queryClient)
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
      await invalidateCompanionPlanQueries(queryClient)
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

  /**
   * 首次进入且表单仍为空壳时，自动把本周补强主题带入，减少用户从零整理输入的成本。
   */
  useEffect(() => {
    if (hasAutoAppliedWeeklyFocusRef.current || planContextDraft || !weeklyFocusQuery.data?.themes.length) {
      return
    }

    hasAutoAppliedWeeklyFocusRef.current = true
    setPlanForm((current) => {
      if (current.goalDescription.trim() || current.weakTopics.length || current.weakTopicsText.trim()) {
        return current
      }
      return applyWeeklyFocusToPlanForm(current, weeklyFocusQuery.data?.themes || [])
    })
    setPlanFormMessage('已根据最近练习和面试记录自动带入本周补强主题，你也可以继续手动调整。')
  }, [planContextDraft, weeklyFocusQuery.data?.themes])

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
  const isPlanOverviewLoading = currentPlanQuery.isLoading || (Boolean(accessToken && currentPlanQuery.data?.id) && planProgressQuery.isLoading)
  const weeklyFocusTopicMap = useMemo(
    () =>
      new Map(
        (weeklyFocusQuery.data?.themes || [])
          .map((theme) => {
            const topicCode = theme.topic_codes[0]
            const linkedTopic = topicCode
              ? (mistakeTopicsQuery.data || []).find((item) => item.code === topicCode) || null
              : null
            return [theme.title, linkedTopic] as const
          }),
      ),
    [mistakeTopicsQuery.data, weeklyFocusQuery.data?.themes],
  )

  /**
   * 预热学习陪伴房间业务代码块，减少从入口页进入房间时的首次等待。
   */
  function preloadCompanionRoomRoute(): void {
    void router.preloadRoute({
      to: '/companion/room',
    })
  }

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
   * 将本周最值得补强的主题直接带入计划表单，减少重复整理输入的成本。
   */
  function handleApplyWeeklyFocus(): void {
    const themes = weeklyFocusQuery.data?.themes || []
    if (!themes.length) {
      setPlanFormMessage('当前还没有足够的补强主题可带入，请先积累一些练习或面试记录。')
      return
    }

    setPlanForm((current) => applyWeeklyFocusToPlanForm(current, themes))
    setPlanFormMessage(`已将 ${themes.length} 个本周补强主题带入计划表单。`)
  }

  /**
   * 以当前补强主题构造正式题库路由并跳到刷题页，方便先补练再回来看计划。
   */
  function handleOpenWeeklyFocusPractice(theme: WeeklyFocusTheme): void {
    const linkedTopic = weeklyFocusTopicMap.get(theme.title) || null
    navigate({
      to: '/practice',
      search: buildWeeklyFocusPracticeRouteSearch(theme, linkedTopic),
    })
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

    const effectiveForm = weeklyFocusQuery.data?.themes.length
      ? applyWeeklyFocusToPlanForm(planForm, weeklyFocusQuery.data.themes)
      : planForm
    setPlanForm(effectiveForm)

    const payload = buildGeneratePlanPayload(effectiveForm, effectiveIndustryCode)
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
              <Link className="primary-button hero-link-button" to="/companion/room" preload="render">
                进入学习陪伴页
              </Link>
              <Link
                className="secondary-button hero-link-button"
                to="/companion/room"
                preload="intent"
                target="_blank"
                rel="noreferrer"
              >
                新窗口打开
              </Link>
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

              <article className="timeline-item companion-context-card">
                <div className="companion-card-head">
                  <div>
                    <span className="section-kicker">本周重点补强</span>
                    <h3>把最近重复暴露的问题直接带入你的学习计划</h3>
                  </div>
                  <span className="companion-card-note">
                    {weeklyFocusQuery.data?.themes.length ? `${weeklyFocusQuery.data.themes.length} 个主题` : '等待主题生成'}
                  </span>
                </div>

                {weeklyFocusQuery.isLoading ? (
                  <p>正在整理你本周最值得优先补强的主题...</p>
                ) : null}

                {weeklyFocusQuery.isError ? (
                  <p>{extractErrorMessage(weeklyFocusQuery.error, '本周补强主题加载失败')}</p>
                ) : null}

                {weeklyFocusQuery.data?.themes.length ? (
                  <>
                    <div className="stack-list">
                      {weeklyFocusQuery.data.themes.map((theme) => (
                        <article className="timeline-item" key={`companion-weekly-focus-${theme.title}`}>
                          <div className="card-inline">
                            <strong>{theme.title}</strong>
                            <span>{theme.source_label}</span>
                          </div>
                          <p>{theme.reason}</p>
                          {theme.focus_tags.length ? (
                            <div className="community-tag-row">
                              {theme.focus_tags.map((item) => (
                                <span key={`${theme.title}-${item}`}>{item}</span>
                              ))}
                            </div>
                          ) : null}
                          {theme.suggestions.length ? (
                            <ul className="interview-bullet-list companion-context-list">
                              {theme.suggestions.map((item) => (
                                <li key={`${theme.title}-${item}`}>{item}</li>
                              ))}
                            </ul>
                          ) : null}
                          <div className="page-actions">
                            <button className="secondary-button" type="button" onClick={() => handleOpenWeeklyFocusPractice(theme)}>
                              先去补练
                            </button>
                          </div>
                        </article>
                      ))}
                    </div>

                    <div className="page-actions">
                      <button className="secondary-button" type="button" onClick={handleApplyWeeklyFocus}>
                        应用到计划表单
                      </button>
                      <Link className="secondary-link" to="/growth">
                        去成长档案查看
                      </Link>
                    </div>
                  </>
                ) : null}

                {!weeklyFocusQuery.isLoading && !weeklyFocusQuery.isError && !weeklyFocusQuery.data?.themes.length ? (
                  <p>先做几道题或完成一场面试后，这里会自动把本周最值得优先补强的 1 到 3 个主题收束出来。</p>
                ) : null}
              </article>

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
                onPrepareContinueTask={preloadCompanionRoomRoute}
              />
            </section>

            {isPlanOverviewLoading ? <CompanionHubPlanOverviewSkeleton /> : (
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
            )}
          </div>
        </div>
      </div>
    </section>
  )
}
