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
  formatFrontendIndustryLabel as formatCompanionIndustryLabel,
  persistSelectedFrontendIndustryCode as persistSelectedCompanionIndustryCode,
  readSelectedFrontendIndustryCode as readSelectedCompanionIndustryCode,
  resolvePreferredFrontendIndustry as resolveCompanionIndustry,
} from '../../shared/industryContext'
import { readCurrentBrowserPath } from '../../shared/authRedirect'
import { useFrontendIndustriesQuery } from '../../shared/frontendQueries'
import { requestLoginPrompt } from '../../shared/loginPrompt'
import { loadLive2DRuntime, prewarmLive2DRuntime } from '../../shared/live2dRuntime'
import {
  buildCompanionCurrentPlanQueryKey,
  buildCompanionLive2DModelsQueryKey,
  buildCompanionPlanProgressQueryKey,
  invalidateCompanionPlanQueries,
} from '../../shared/queryKeys'
import { SectionErrorBoundary } from '../../shared/SectionErrorBoundary'
import {
  adjustCompanionPlan,
  fetchCompanionPlanProgress,
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
  deriveActiveGoals,
  deriveTodayGoals,
  persistCompanionExecutionUpdate,
  resolveFocusedCompanionTask,
  taskStatusLabel,
} from './companionHelpers'
import { buildCompanionSessionSummary, formatCompanionDateTime, GoalList } from './companionShared'
import {
  clearCompanionFocusTask,
  persistCompanionFocusTask,
  persistCompanionSessionSummary,
  persistSelectedCompanionModelKey,
  readCompanionDailyDigest,
  readCompanionFocusTask,
  readSelectedCompanionModelKey,
} from './companionStorage'
import type {
  CompanionDailyDigest,
  CompanionFocusTaskDraft,
  CompanionHistoryItem,
  CompanionPlanDetail,
  CompanionPlanTask,
  CompanionSelectableLive2DModel,
  CompanionSessionSummary,
  CompanionTaskStatus,
} from './companionTypes'
import { useCompanionStudyLogSync } from './useCompanionStudyLogSync'

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
 * 在计划与进度仍在同步时渲染固定骨架，避免侧栏布局因异步结果抖动。
 */
function CompanionPlanLoadingSkeleton() {
  return (
    <article className="status-card companion-progress-card">
      <div className="companion-card-head">
        <div className="companion-skeleton-block companion-skeleton-block-stack">
          <span className="companion-skeleton-line companion-skeleton-line-short" />
          <span className="companion-skeleton-line companion-skeleton-line-medium" />
        </div>
        <span className="companion-skeleton-pill" />
      </div>
      <div className="companion-progress-block">
        <div className="companion-progress-head">
          <span className="companion-skeleton-line companion-skeleton-line-medium" />
          <span className="companion-skeleton-line companion-skeleton-line-short" />
        </div>
        <div className="companion-progress-bar companion-progress-bar-skeleton">
          <div className="companion-progress-bar-fill companion-progress-bar-fill-skeleton" />
        </div>
      </div>
      <div className="interview-overview-stats">
        {Array.from({ length: 4 }).map((_, index) => (
          <div className="companion-stat-chip companion-stat-chip-skeleton" key={index}>
            <span className="companion-skeleton-line companion-skeleton-line-short" />
            <span className="companion-skeleton-line companion-skeleton-line-medium" />
          </div>
        ))}
      </div>
      <p className="companion-empty-text">正在同步当前计划与进度...</p>
    </article>
  )
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
    queryKey: buildCompanionLive2DModelsQueryKey(props.industryCode),
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
        const { PIXI, Live2DModel } = await loadLive2DRuntime()

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
  const [stageEnabled, setStageEnabled] = useState(false)
  const industriesQuery = useFrontendIndustriesQuery()

  const currentPlanQuery = useQuery({
    queryKey: buildCompanionCurrentPlanQueryKey(accessToken),
    queryFn: () => fetchCurrentPlan(accessToken as string),
    enabled: Boolean(accessToken),
    retry: false,
  })

  const planProgressQuery = useQuery({
    queryKey: buildCompanionPlanProgressQueryKey(accessToken, currentPlanQuery.data?.id),
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
  const isPlanPanelLoading = currentPlanQuery.isLoading || (Boolean(accessToken && currentPlanQuery.data?.id) && planProgressQuery.isLoading)

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
      await invalidateCompanionPlanQueries(queryClient)
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
      await invalidateCompanionPlanQueries(queryClient)
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
   * 在房间空闲时预热 Live2D 运行时与模型列表缓存，减少首次启用舞台时的等待。
   */
  useEffect(() => {
    if (stageEnabled || !workspaceIndustryCode || typeof window === 'undefined') {
      return undefined
    }

    const warmup = () => {
      prewarmLive2DRuntime()
      void queryClient.prefetchQuery({
        queryKey: buildCompanionLive2DModelsQueryKey(workspaceIndustryCode),
        queryFn: () => fetchSelectableCompanionLive2DModels(workspaceIndustryCode),
        staleTime: 60 * 1000,
      })
    }

    if ('requestIdleCallback' in window) {
      const idleId = window.requestIdleCallback(() => {
        warmup()
      }, { timeout: 1500 })

      return () => {
        window.cancelIdleCallback(idleId)
      }
    }

    const timer = window.setTimeout(() => {
      warmup()
    }, 800)

    return () => {
      window.clearTimeout(timer)
    }
  }, [queryClient, stageEnabled, workspaceIndustryCode])

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

  /**
   * 提前启动角色舞台所需运行时和模型列表请求，避免点击启用后再从零开始加载。
   */
  function handlePrepareStage(): void {
    prewarmLive2DRuntime()
    void queryClient.prefetchQuery({
      queryKey: buildCompanionLive2DModelsQueryKey(workspaceIndustryCode),
      queryFn: () => fetchSelectableCompanionLive2DModels(workspaceIndustryCode),
      staleTime: 60 * 1000,
    })
  }

  /**
   * 在用户明确需要沉浸式角色舞台时再启用 Live2D，并复用已经启动的预热链路。
   */
  function handleEnableStage(): void {
    handlePrepareStage()
    setStageEnabled(true)
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
        <SectionErrorBoundary
          className="companion-sidebar"
          title="侧栏区域加载失败"
          description="计划概览、目标列表或对话记录在渲染时出现异常。你可以重试当前区域，右侧舞台仍可继续使用。"
          resetKeys={[currentPlanQuery.data?.id, history.length, focusedTask?.id, accessToken]}
        >
          <aside className="companion-sidebar">
            <div className="companion-sidebar-head">
              <span className="page-tag">学习陪伴</span>
              <h1>{user?.username ? `${user.username} 的学习陪伴页` : '学习陪伴页'}</h1>
              <p className="page-copy">
                左侧专门放今天要推进的目标与完整对话记录，右侧保留角色舞台，并支持在当前场景下切换可用模型。
              </p>
            </div>

            {isPlanPanelLoading ? <CompanionPlanLoadingSkeleton /> : (
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
            )}

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
        </SectionErrorBoundary>

        <SectionErrorBoundary
          className="companion-stage-shell"
          title="舞台区域加载失败"
          description="角色舞台或输入区在渲染时出现异常。你可以重试当前区域，左侧计划与历史记录仍然可继续使用。"
          retryLabel="重新挂载舞台区域"
          onRetry={() => setStageEnabled(false)}
          resetKeys={[stageEnabled, workspaceIndustryCode, currentDialogue, history.length]}
        >
          <div className="companion-stage-shell">
            {stageEnabled ? (
              <AriuStage
                dialogue={currentDialogue}
                emotion={stageFeedback.emotion}
                action={stageFeedback.action}
                loggedIn={Boolean(accessToken)}
                industryCode={workspaceIndustryCode}
              />
            ) : (
              <section className="companion-stage-panel">
                <div className="companion-stage-topbar">
                  <div className="companion-stage-badges">
                    <span className="page-tag">Live2D 待启用</span>
                    <span className="companion-state-pill">情绪：{stageFeedback.emotion || 'steady'}</span>
                    <span className="companion-state-pill">动作：{stageFeedback.action || 'idle'}</span>
                  </div>
                  <div className="companion-stage-side">
                    <span className="companion-stage-copy">为减少首次进入房间时的资源加载，角色舞台会在你主动启用后再加载。</span>
                  </div>
                </div>
                <div className="companion-stage-host companion-stage-host-idle">
                  <div className="companion-empty-state">
                    <strong>当前已暂停加载 Live2D 舞台</strong>
                    <p>聊天、计划查看和任务推进已可直接使用；当你需要沉浸式陪伴视图时，再启动角色舞台即可。</p>
                    <button
                      className="primary-button"
                      type="button"
                      onClick={handleEnableStage}
                      onFocus={handlePrepareStage}
                      onMouseEnter={handlePrepareStage}
                    >
                      启用角色舞台
                    </button>
                  </div>
                </div>
              </section>
            )}

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
        </SectionErrorBoundary>
      </div>
    </section>
  )
}

export default CompanionWorkspacePage
