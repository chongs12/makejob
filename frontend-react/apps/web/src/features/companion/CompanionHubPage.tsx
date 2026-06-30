import { useEffect, useMemo, useRef, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { Button, Input, Select, Slider, message } from 'antd'
import { RocketOutlined } from '@ant-design/icons'
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
import {
  buildCompanionCategoryOptionsQueryKey,
  buildCompanionCurrentPlanQueryKey,
  buildCompanionPlanProgressQueryKey,
  buildCompanionWeeklyFocusQueryKey,
  buildPracticeQuestionSetDetailQueryKey,
  invalidateCompanionPlanQueries,
} from '../../shared/queryKeys'
import { fetchWeeklyFocus, type WeeklyFocusTheme } from '../../shared/weeklyFocus'
import { fetchQuestionSetDetail } from '../../shared/practiceCatalog'
import {
  createCompanionPlan,
  fetchCompanionCategoryTree,
  fetchCompanionPlanProgress,
  fetchCurrentPlan,
  submitCompanionTaskFeedback,
  updateCompanionTaskStatus,
} from './companionApi'
import {
  buildCompanionTaskFeedbackPayload,
  buildDefaultCompanionTaskFeedbackDraft,
  deriveTodayGoals,
  persistCompanionExecutionUpdate,
  resolveCompanionTaskQuestionId,
  resolveFocusedCompanionTask,
  taskStatusLabel,
} from './companionHelpers'
import {
  applyCompanionPlanContextToForm,
  applyWeeklyFocusToPlanForm,
  buildGeneratePlanPayload,
  buildInitialPlanForm,
  deriveLatestCompletedTask,
  flattenCompanionCategories,
  planLevelLabel,
} from './companionHubHelpers'
import {
  CompanionTaskFeedbackPanel,
  formatCompanionDateTime,
} from './companionShared'
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
  CompanionTaskFeedbackDraft,
  CompanionTaskStatus,
} from './companionTypes'
import { useCompanionStudyLogSync } from './useCompanionStudyLogSync'
import { TopMetricsBar } from '../../../features/companion/TopMetricsBar'
import { TodayTaskFlow } from './TodayTaskFlow'
import { PlanOverview } from './PlanOverview'

const COMPANION_THEME = {
  bg: '#f8f9fa',
  cardBg: '#ffffff',
  primary: '#f97316',
  primaryLight: '#fff7ed',
  primaryDark: '#ea580c',
  textMain: '#1f2937',
  textSecondary: '#6b7280',
  textMuted: '#9ca3af',
  border: '#f3f4f6',
  borderHover: '#e5e7eb',
  shadow: '0 1px 2px rgba(0,0,0,0.05)',
  shadowCard: '0 4px 6px -1px rgba(0,0,0,0.07), 0 2px 4px -2px rgba(0,0,0,0.05)',
  shadowHover: '0 10px 15px -3px rgba(0,0,0,0.08), 0 4px 6px -4px rgba(0,0,0,0.05)',
  radius: 12,
  radiusSm: 8,
  success: '#22c55e',
  warning: '#f59e0b',
  danger: '#ef4444',
}

/**
 * 提供学习陪伴的入口页，聚焦今日任务、计划概览和快速操作。
 */
export function CompanionHubPage() {
  const navigate = useNavigate()
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
  const [planFormMessage, setPlanFormMessage] = useState('')
  const [taskActionTaskId, setTaskActionTaskId] = useState<number | null>(null)
  const [feedbackTask, setFeedbackTask] = useState<CompanionPlanTask | null>(null)
  const [feedbackDraft, setFeedbackDraft] = useState<CompanionTaskFeedbackDraft>(() => buildDefaultCompanionTaskFeedbackDraft(null, null))

  const industriesQuery = useFrontendIndustriesQuery()

  const currentPlanQuery = useQuery({
    queryKey: buildCompanionCurrentPlanQueryKey(accessToken),
    queryFn: () => fetchCurrentPlan(accessToken as string),
    enabled: Boolean(accessToken),
    retry: false,
    refetchInterval: (query) => {
      const plan = query.state.data
      if (plan?.status === 'generating' && plan.task_status !== 'failed' && plan.task_status !== 'dead') {
        return 3000
      }
      return false
    },
  })

  const practiceStatsQuery = usePracticeStatsQuery(accessToken)

  const weeklyFocusQuery = useQuery({
    queryKey: buildCompanionWeeklyFocusQueryKey(accessToken),
    queryFn: () => fetchWeeklyFocus(accessToken as string),
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
  const feedbackQuestionSetQuery = useQuery({
    queryKey: buildPracticeQuestionSetDetailQueryKey(currentPlanQuery.data?.industry_id || null, feedbackTask?.collection_hint || ''),
    queryFn: () => fetchQuestionSetDetail(currentPlanQuery.data?.industry_id || null, feedbackTask?.collection_hint || ''),
    enabled: Boolean(feedbackTask?.collection_hint && currentPlanQuery.data?.industry_id),
    staleTime: 5 * 60 * 1000,
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
      setPlanFormMessage(plan.status === 'generating' ? '学习计划任务已提交，系统正在异步生成内容，页面会自动刷新。' : `新计划已生成：${plan.title}`)
      await invalidateCompanionPlanQueries(queryClient)
    },
    onError: (error) => {
      setPlanFormMessage(extractErrorMessage(error, '生成学习计划失败，请稍后重试'))
    },
  })

  const updateTaskMutation = useMutation({
    mutationFn: async (payload: { task: CompanionPlanTask; status: CompanionTaskStatus; feedback?: CompanionTaskFeedbackDraft }) => {
      if (payload.feedback) {
        await submitCompanionTaskFeedback(
          accessToken as string,
          currentPlanQuery.data?.id as number,
          payload.task.id,
          buildCompanionTaskFeedbackPayload(
            payload.feedback,
            resolveCompanionTaskQuestionId(payload.task, feedbackQuestionSetQuery.data || null),
          ),
        )
      }
      await updateCompanionTaskStatus(accessToken as string, currentPlanQuery.data?.id as number, payload.task.id, payload.status)
    },
    onSuccess: async (_, variables) => {
      const { nextDigest, nextFocusTask } = persistCompanionExecutionUpdate(
        currentPlanQuery.data || null,
        variables.task,
        variables.status,
        'hub',
        dailyDigest,
      )
      setTaskActionTaskId(null)
      setFeedbackTask(null)
      setFeedbackDraft(buildDefaultCompanionTaskFeedbackDraft(null, null))
      setDailyDigest(nextDigest)
      setFocusTaskDraft(nextFocusTask)
      setPlanFormMessage(variables.feedback ? '已记录训练反馈，并把任务标记为已完成。' : `任务状态已更新为「${taskStatusLabel(variables.status)}」。`)
      await invalidateCompanionPlanQueries(queryClient)
    },
    onError: (error) => {
      setTaskActionTaskId(null)
      setPlanFormMessage(extractErrorMessage(error, '更新任务状态失败，请稍后重试'))
    },
  })

  /**
   * 窗口重新聚焦时读取最新的本地会话摘要和续接草稿。
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
   * 行业列表加载后归一化当前选中的行业编码。
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
   * 首次进入时自动把跨页带来的上下文草稿写入计划表单。
   */
  useEffect(() => {
    if (!planContextDraft || hasAppliedPlanContextRef.current) {
      return
    }

    hasAppliedPlanContextRef.current = true
    setSelectedIndustryCode(planContextDraft.industryCode || DEFAULT_COMPANION_INDUSTRY_CODE)
    setPlanForm((current) => applyCompanionPlanContextToForm(current, planContextDraft))
    setPlanFormMessage(planContextDraft.source === 'growth-summary'
      ? '已根据成长档案自动带入强化计划上下文。'
      : `已根据面试报告 #${planContextDraft.interviewId || '-'} 自动带入强化计划上下文。`)
  }, [planContextDraft])

  /**
   * 首次进入且表单仍为空时，自动带入本周补强主题。
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
  }, [planContextDraft, weeklyFocusQuery.data?.themes])

  const latestCompletedTask = useMemo(() => deriveLatestCompletedTask(currentPlanQuery.data || null), [currentPlanQuery.data])
  const todayGoals = useMemo(() => deriveTodayGoals(currentPlanQuery.data || null), [currentPlanQuery.data])
  const focusedTask = useMemo(
    () => resolveFocusedCompanionTask(currentPlanQuery.data || null, focusTaskDraft),
    [currentPlanQuery.data, focusTaskDraft],
  )
  useCompanionStudyLogSync(accessToken, currentPlanQuery.data || null, dailyDigest, focusedTask)
  const planStats = planProgressQuery.data
  const isPlanOverviewLoading = currentPlanQuery.isLoading || (Boolean(accessToken && currentPlanQuery.data?.id) && planProgressQuery.isLoading)

  /**
   * 进入陪伴房间继续学习，并把当前聚焦任务写入续接草稿。
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
   * 在入口页直接推进今日任务状态。
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

    if (status === 'completed') {
      setFeedbackTask(task)
      setFeedbackDraft(buildDefaultCompanionTaskFeedbackDraft(currentPlanQuery.data || null, task))
      setPlanFormMessage(`完成「${task.title}」前，先补一份训练反馈。`)
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
   * 提交任务训练反馈并把任务标记为已完成。
   */
  async function handleSubmitTaskFeedback() {
    if (!accessToken) {
      requestLoginPrompt('/companion', 'missing')
      return
    }

    if (!currentPlanQuery.data?.id || !feedbackTask) {
      setPlanFormMessage('请先登录并确保当前存在可推进的学习计划。')
      return
    }

    setTaskActionTaskId(feedbackTask.id)
    setPlanFormMessage(`正在记录「${feedbackTask.title}」的训练反馈...`)
    try {
      await updateTaskMutation.mutateAsync({
        task: feedbackTask,
        status: 'completed',
        feedback: feedbackDraft,
      })
    } catch {
      if (!useAuthStore.getState().accessToken) {
        requestLoginPrompt('/companion', 'expired')
      }
    }
  }

  /**
   * 提交计划生成表单。
   */
  async function handleGeneratePlan() {
    if (!accessToken) {
      requestLoginPrompt('/companion', 'missing')
      return
    }

    if (!planForm.goalDescription.trim()) {
      setPlanFormMessage('请填写学习目标')
      return
    }

    const payload = buildGeneratePlanPayload(planForm, effectiveIndustryCode)
    setPlanFormMessage('陪伴助手正在提交计划生成任务...')
    createPlanMutation.mutate(payload)
  }

  const cardStyle = {
    background: COMPANION_THEME.cardBg,
    borderRadius: COMPANION_THEME.radius,
    border: `1px solid ${COMPANION_THEME.border}`,
    boxShadow: COMPANION_THEME.shadow,
    padding: '24px',
  }

  return (
    <div style={{ minHeight: '100vh', background: COMPANION_THEME.bg }}>
      <TopMetricsBar
        streakDays={practiceStatsQuery.data?.streak_days ?? 0}
        planProgress={Math.round(currentPlanQuery.data?.progress || 0)}
      />

      <div style={{
        maxWidth: 1200,
        margin: '0 auto',
        padding: '24px',
        display: 'grid',
        gridTemplateColumns: '1fr 360px',
        gap: 24,
      }}>
        <div style={{ minWidth: 0 }}>
          <TodayTaskFlow
            focusedTask={focusedTask}
            todayGoals={todayGoals}
            isLoading={currentPlanQuery.isLoading}
            onContinue={handleContinueTask}
            onComplete={(task) => void handleHubTaskStatusChange(task, 'completed')}
            onSkip={(task) => void handleHubTaskStatusChange(task, 'skipped')}
          />
        </div>

        <div>
          <PlanOverview
            plan={currentPlanQuery.data || null}
            planStats={planStats ? {
              completed_tasks: planStats.completed_tasks,
              total_tasks: planStats.total_tasks,
              progress: planStats.progress,
            } : null}
            weeklyFocusThemes={weeklyFocusQuery.data?.themes || []}
            latestCompletedTask={latestCompletedTask}
            isLoading={isPlanOverviewLoading}
          />
        </div>
      </div>

      {/* 计划生成表单 */}
      <div style={{ maxWidth: 1200, margin: '0 auto', padding: '0 24px 48px' }}>
        <div style={cardStyle}>
          <div style={{ marginBottom: 24 }}>
            <h2 style={{ fontSize: 20, fontWeight: 700, color: COMPANION_THEME.textMain, margin: '0 0 8px' }}>
              新建学习计划
            </h2>
            <p style={{ fontSize: 14, color: COMPANION_THEME.textSecondary, margin: 0 }}>
              设定目标，让陪伴助手为你制定专属学习计划
            </p>
          </div>

          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 20, marginBottom: 20 }}>
            <div>
              <label style={{ display: 'block', fontSize: 13, fontWeight: 600, color: COMPANION_THEME.textMain, marginBottom: 8 }}>
                学习方向
              </label>
              <Select
                value={effectiveIndustryCode}
                onChange={(value) => setSelectedIndustryCode(value)}
                style={{ width: '100%' }}
                options={(industriesQuery.data || []).map((i) => ({ value: i.code, label: i.name }))}
              />
            </div>

            <div>
              <label style={{ display: 'block', fontSize: 13, fontWeight: 600, color: COMPANION_THEME.textMain, marginBottom: 8 }}>
                当前水平
              </label>
              <Select
                value={planForm.level}
                onChange={(value) => setPlanForm((prev) => ({ ...prev, level: value }))}
                style={{ width: '100%' }}
                options={[
                  { value: 'beginner', label: planLevelLabel('beginner') },
                  { value: 'intermediate', label: planLevelLabel('intermediate') },
                  { value: 'advanced', label: planLevelLabel('advanced') },
                ]}
              />
            </div>
          </div>

          <div style={{ marginBottom: 20 }}>
            <label style={{ display: 'block', fontSize: 13, fontWeight: 600, color: COMPANION_THEME.textMain, marginBottom: 8 }}>
              学习目标
            </label>
            <Input.TextArea
              value={planForm.goalDescription}
              onChange={(e) => setPlanForm((prev) => ({ ...prev, goalDescription: e.target.value }))}
              placeholder="例如：两周内掌握并发编程核心概念"
              rows={3}
              style={{ borderRadius: 8 }}
            />
          </div>

          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 20, marginBottom: 20 }}>
            <div>
              <label style={{ display: 'block', fontSize: 13, fontWeight: 600, color: COMPANION_THEME.textMain, marginBottom: 8 }}>
                每日学习时长：{planForm.dailyStudyTime} 分钟
              </label>
              <Slider
                min={15}
                max={180}
                step={15}
                value={Number(planForm.dailyStudyTime)}
                onChange={(value) => setPlanForm((prev) => ({ ...prev, dailyStudyTime: String(value) }))}
                marks={{ 15: '15', 60: '60', 120: '120', 180: '180' }}
              />
            </div>

            <div>
              <label style={{ display: 'block', fontSize: 13, fontWeight: 600, color: COMPANION_THEME.textMain, marginBottom: 8 }}>
                计划周期：{planForm.durationDays} 天
              </label>
              <Slider
                min={7}
                max={60}
                step={1}
                value={Number(planForm.durationDays)}
                onChange={(value) => setPlanForm((prev) => ({ ...prev, durationDays: String(value) }))}
                marks={{ 7: '7', 14: '14', 30: '30', 60: '60' }}
              />
            </div>
          </div>

          {categoryOptionsQuery.data && categoryOptionsQuery.data.length > 0 && (
            <div style={{ marginBottom: 20 }}>
              <label style={{ display: 'block', fontSize: 13, fontWeight: 600, color: COMPANION_THEME.textMain, marginBottom: 8 }}>
                弱项（可选）
              </label>
              <Select
                mode="multiple"
                value={planForm.weakTopics}
                onChange={(value) => setPlanForm((prev) => ({ ...prev, weakTopics: value }))}
                style={{ width: '100%' }}
                placeholder="选择需要重点补强的方向"
                options={categoryOptionsQuery.data.map((c) => ({ value: c.name, label: c.name }))}
                maxTagCount={3}
              />
            </div>
          )}

          <div style={{ display: 'flex', alignItems: 'center', gap: 16 }}>
            <Button
              type="primary"
              size="large"
              icon={<RocketOutlined />}
              loading={createPlanMutation.isPending}
              onClick={handleGeneratePlan}
              style={{
                background: COMPANION_THEME.primary,
                borderColor: COMPANION_THEME.primary,
                borderRadius: 8,
                fontWeight: 600,
                height: 48,
                padding: '0 32px',
              }}
            >
              生成学习计划
            </Button>

            {planFormMessage && (
              <span style={{ fontSize: 14, color: COMPANION_THEME.textSecondary }}>
                {planFormMessage}
              </span>
            )}
          </div>
        </div>
      </div>

      {feedbackTask ? (
        <div style={{
          position: 'fixed',
          bottom: 0,
          left: 0,
          right: 0,
          zIndex: 50,
          background: COMPANION_THEME.cardBg,
          borderTop: `1px solid ${COMPANION_THEME.border}`,
          boxShadow: COMPANION_THEME.shadowCard,
        }}>
          <div style={{ maxWidth: 1200, margin: '0 auto', padding: '16px 24px' }}>
            <CompanionTaskFeedbackPanel
              task={feedbackTask}
              draft={feedbackDraft}
              pending={updateTaskMutation.isPending && taskActionTaskId === feedbackTask.id}
              message={planFormMessage}
              onChange={setFeedbackDraft}
              onSubmit={() => void handleSubmitTaskFeedback()}
              onCancel={() => {
                setFeedbackTask(null)
                setFeedbackDraft(buildDefaultCompanionTaskFeedbackDraft(null, null))
                setPlanFormMessage('已取消本次反馈填写。')
              }}
            />
          </div>
        </div>
      ) : null}
    </div>
  )
}
