import type { FormEvent } from 'react'
import { useEffect, useMemo, useRef, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
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
import { prewarmLive2DRuntime } from '../../shared/live2dRuntime'
import { readSelectedLive2DModelKey } from '../../shared/live2dModelCatalog'
import { useLive2DDialoguePlayback } from '../../shared/useLive2DDialoguePlayback'
import {
  buildCompanionCurrentPlanQueryKey,
  buildCompanionLive2DModelsQueryKey,
  buildCompanionPlanProgressQueryKey,
  buildPracticeQuestionSetDetailQueryKey,
  invalidateCompanionPlanQueries,
} from '../../shared/queryKeys'
import { buildPracticeRouteSearch, resolvePracticeQuestionSetTitle } from '../../shared/practiceRoute'
import { fetchQuestionSetDetail } from '../../shared/practiceCatalog'
import { SectionErrorBoundary } from '../../shared/SectionErrorBoundary'
import { CompanionLive2DStage } from './CompanionLive2DStage'
import {
  adjustCompanionPlan,
  fetchCompanionGreeting,
  fetchCompanionPlanProgress,
  fetchCurrentPlan,
  fetchSelectableCompanionLive2DModels,
  recognizeSpeech,
  sendCompanionChatRequest,
  submitCompanionTaskFeedback,
  updateCompanionTaskStatus,
} from './companionApi'
import {
  buildCompanionContinuePrompt,
  buildCompanionTaskFeedbackPayload,
  buildCompanionDailyDigestText,
  buildCompanionPhaseAdjustmentHint,
  buildCompanionQuickPrompts,
  buildCompanionTaskActionFeedback,
  buildDefaultCompanionTaskFeedbackDraft,
  buildCompanionWorkspaceResumeMessage,
  buildPlanProgressHint,
  deriveActiveGoals,
  deriveTodayGoals,
  persistCompanionExecutionUpdate,
  resolveCompanionTaskQuestionId,
  resolveFocusedCompanionTask,
  taskStatusLabel,
} from './companionHelpers'
import { buildCompanionSessionSummary, CompanionPlanPhaseSection, CompanionTaskFeedbackPanel, formatCompanionDateTime, formatCompanionPhaseLabel, GoalList } from './companionShared'
import {
  clearCompanionFocusTask,
  persistCompanionFocusTask,
  persistCompanionSessionSummary,
  readCompanionDailyDigest,
  readCompanionFocusTask,
} from './companionStorage'
import type {
  CompanionDailyDigest,
  CompanionFocusTaskDraft,
  CompanionHistoryItem,
  CompanionPlanDetail,
  CompanionPlanTask,
  CompanionSessionSummary,
  CompanionTaskFeedbackDraft,
  CompanionTaskStatus,
} from './companionTypes'
import { useCompanionStudyLogSync } from './useCompanionStudyLogSync'

const COMPANION_INITIAL_DIALOGUE = '我是你的学习陪伴助手。先把今天要推进的学习目标摆清楚，我们再一项一项完成。'

/**
 * 创建陪伴页首屏默认消息，保证页面在未登录时也能展示完整骨架。
 */
function buildInitialHistory(): CompanionHistoryItem[] {
  return [
      {
        id: 'assistant-welcome',
        role: 'assistant',
        content: COMPANION_INITIAL_DIALOGUE,
        createdAt: Date.now(),
      },
  ]
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
 * 提供学习陪伴核心页面，整合 Ariu 舞台、计划侧栏和聊天输入区。
 */
export function CompanionWorkspacePage() {
  const accessToken = useAuthStore((state) => state.accessToken)
  const user = useAuthStore((state) => state.user)
  const queryClient = useQueryClient()
  const hasInjectedResumeMessageRef = useRef(false)
  const greetingFetchedRef = useRef(false)
  const [history, setHistory] = useState<CompanionHistoryItem[]>(() => buildInitialHistory())
  const [composer, setComposer] = useState('')
  const [sending, setSending] = useState(false)
  const [composerMessage, setComposerMessage] = useState('')
  const [taskActionTaskId, setTaskActionTaskId] = useState<number | null>(null)
  const [planActionMessage, setPlanActionMessage] = useState('')
  const [feedbackTask, setFeedbackTask] = useState<CompanionPlanTask | null>(null)
  const [feedbackDraft, setFeedbackDraft] = useState<CompanionTaskFeedbackDraft>(() => buildDefaultCompanionTaskFeedbackDraft(null, null))
  const [preferredIndustryCode, setPreferredIndustryCode] = useState(() => readSelectedCompanionIndustryCode() || DEFAULT_COMPANION_INDUSTRY_CODE)
  const [dailyDigest, setDailyDigest] = useState<CompanionDailyDigest | null>(() => readCompanionDailyDigest())
  const [focusTaskDraft, setFocusTaskDraft] = useState<CompanionFocusTaskDraft | null>(() => readCompanionFocusTask())
  const [stageEnabled, setStageEnabled] = useState(false)
  const [selectedLive2DModelKey, setSelectedLive2DModelKey] = useState(() => readSelectedLive2DModelKey('companion', readSelectedCompanionIndustryCode() || DEFAULT_COMPANION_INDUSTRY_CODE))
  const [isRecording, setIsRecording] = useState(false)
  const [isRecognizing, setIsRecognizing] = useState(false)
  const recordStreamRef = useRef<MediaStream | null>(null)
  const recordAudioContextRef = useRef<AudioContext | null>(null)
  const recordSourceRef = useRef<MediaStreamAudioSourceNode | null>(null)
  const recordProcessorRef = useRef<ScriptProcessorNode | null>(null)
  const recordPCMRef = useRef<number[]>([])
  const {
    liveDialogue: stageDialogue,
    isDialogueTyping: isStageDialogueTyping,
    mouthOpen: stageMouthOpen,
    startDialogueTyping,
    stopCurrentPlayback,
    syncDialogueImmediately,
    playTTSAudio,
  } = useLive2DDialoguePlayback({
    initialDialogue: COMPANION_INITIAL_DIALOGUE,
    onPlaybackError: (error) => {
      setComposerMessage(extractErrorMessage(error, '陪伴语音播放失败，已回退到文本模式。'))
    },
  })
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
  const quickPrompts = useMemo(() => buildCompanionQuickPrompts(currentPlanQuery.data || null, focusedTask), [currentPlanQuery.data, focusedTask])
  const stageFeedback = useMemo(() => resolveStageFeedback(history), [history])
  const latestAssistantMessage = useMemo(() => [...history].reverse().find((item) => item.role === 'assistant') || null, [history])
  const planProgressHint = useMemo(() => buildPlanProgressHint(planProgressQuery.data), [planProgressQuery.data])
  const phaseAdjustmentHint = useMemo(() => buildCompanionPhaseAdjustmentHint(currentPlanQuery.data || null), [currentPlanQuery.data])
  const workspaceIndustryCode = currentPlanQuery.data?.industry_code?.trim() || preferredIndustryCode.trim() || DEFAULT_COMPANION_INDUSTRY_CODE
  const workspaceIndustry = useMemo(
    () => resolveCompanionIndustry(industriesQuery.data || [], workspaceIndustryCode),
    [industriesQuery.data, workspaceIndustryCode],
  )
  const workspaceIndustryLabel = formatCompanionIndustryLabel(workspaceIndustry, workspaceIndustryCode)
  const isPlanGenerating = currentPlanQuery.data?.status === 'generating'
  const isPlanPanelLoading = currentPlanQuery.isLoading || (Boolean(accessToken && currentPlanQuery.data?.id) && planProgressQuery.isLoading)

  /**
   * 统一追加陪伴助手消息，并按场景决定是立即更新舞台，还是触发字幕/TTS 播放。
   */
  function appendAssistantMessage(
    message: CompanionHistoryItem,
    playback: 'immediate' | 'typing' = 'immediate',
    audioUrl?: string,
  ): void {
    setHistory((current) => [...current, message])

    if (playback === 'typing') {
      if (audioUrl) {
        void playTTSAudio(audioUrl, message.content)
        return
      }

      startDialogueTyping(message.content)
      return
    }

    syncDialogueImmediately(message.content)
  }

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
      const { projectedPlan, nextDigest, nextFocusTask } = persistCompanionExecutionUpdate(
        currentPlanQuery.data || null,
        variables.task,
        variables.status,
        'room',
        dailyDigest,
      )
      setTaskActionTaskId(null)
      setFeedbackTask(null)
      setFeedbackDraft(buildDefaultCompanionTaskFeedbackDraft(null, null))
      setDailyDigest(nextDigest)
      setFocusTaskDraft(nextFocusTask)
      setPlanActionMessage(variables.feedback ? '已记录训练反馈，并把任务标记为已完成。' : `任务状态已更新为「${taskStatusLabel(variables.status)}」。`)
      appendAssistantMessage({
        id: `assistant-task-${Date.now()}`,
        role: 'assistant',
        content: buildCompanionTaskActionFeedback(variables.task, variables.status, projectedPlan, nextDigest),
        emotion: variables.status === 'completed' ? 'encouraging' : 'steady',
        action: variables.status === 'completed' ? 'celebrate' : 'nod',
        live2dDirective: null,
        createdAt: Date.now(),
      })
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
   * 页面加载时调用后端 greeting 接口，生成包含面试上下文的个性化打招呼。
   * 失败时保留默认欢迎语，不影响页面正常使用。
   */
  useEffect(() => {
    if (greetingFetchedRef.current || !accessToken) {
      return
    }
    greetingFetchedRef.current = true

    fetchCompanionGreeting(accessToken, selectedLive2DModelKey)
      .then((reply) => {
        const greetingText = reply.reply || reply.content || COMPANION_INITIAL_DIALOGUE
        setHistory((current) =>
          current.map((item) =>
            item.id === 'assistant-welcome'
              ? {
                  id: 'assistant-greeting',
                  role: 'assistant' as const,
                  content: greetingText,
                  emotion: reply.emotion || 'steady',
                  action: reply.action || 'idle',
                  live2dDirective: reply.live2d_directive || null,
                  createdAt: Date.now(),
                }
              : item,
          ),
        )
        syncDialogueImmediately(greetingText)
        if (reply.audio_url) {
          void playTTSAudio(reply.audio_url, greetingText)
        }
      })
      .catch(() => {
        // 打招呼失败时保留默认欢迎语
      })
  }, [accessToken, selectedLive2DModelKey, syncDialogueImmediately, playTTSAudio])

  /**
   * 当房间拿到当前计划后，自动注入一条续接提示，避免每次进入都像从零开始。
   */
  useEffect(() => {
    if (hasInjectedResumeMessageRef.current || !accessToken || !currentPlanQuery.data || isPlanGenerating) {
      return
    }

    hasInjectedResumeMessageRef.current = true
    appendAssistantMessage({
      id: `assistant-resume-${currentPlanQuery.data.id}`,
      role: 'assistant',
      content: buildCompanionWorkspaceResumeMessage(currentPlanQuery.data, focusedTask, dailyDigest),
      emotion: 'steady',
      action: 'nod',
      live2dDirective: null,
      createdAt: Date.now(),
    })
  }, [accessToken, currentPlanQuery.data, dailyDigest, focusedTask, isPlanGenerating])

  /**
   * 当当前计划或入口页偏好发生变化时，同步持久化当前陪伴场景应使用的行业上下文。
   */
  useEffect(() => {
    if (!workspaceIndustryCode) {
      return
    }

    persistSelectedCompanionIndustryCode(workspaceIndustryCode)
    setSelectedLive2DModelKey(readSelectedLive2DModelKey('companion', workspaceIndustryCode))
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
    if (isPlanGenerating) {
      setPlanActionMessage(currentPlanQuery.data.task_error || '当前计划仍在生成中，待任务落库后才能更新任务状态。')
      return
    }

    if (status === 'completed') {
      setFeedbackTask(task)
      setFeedbackDraft(buildDefaultCompanionTaskFeedbackDraft(currentPlanQuery.data || null, task))
      setPlanActionMessage(`完成「${task.title}」前，先补一份训练反馈，后续调整计划会更准。`)
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
   * 提交任务训练反馈，并在成功后把任务状态一并更新为已完成。
   */
  async function handleSubmitTaskFeedback() {
    if (!accessToken) {
      requestLoginPrompt('/companion/room', 'missing')
      return
    }

    if (!currentPlanQuery.data?.id || !feedbackTask) {
      setPlanActionMessage('请先登录并确保当前存在可操作的学习计划。')
      return
    }

    setTaskActionTaskId(feedbackTask.id)
    setPlanActionMessage(`正在记录「${feedbackTask.title}」的训练反馈...`)
    try {
      await updateTaskMutation.mutateAsync({
        task: feedbackTask,
        status: 'completed',
        feedback: feedbackDraft,
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
    if (isPlanGenerating) {
      setPlanActionMessage(currentPlanQuery.data.task_error || '当前计划仍在生成中，暂时不能调整，请等待异步任务完成。')
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
   * 启动麦克风录音，采集完成后调用 ASR 接口识别语音。
   */
  async function handleToggleRecording() {
    if (!accessToken) {
      requestLoginPrompt('/companion/room', 'missing')
      return
    }

    if (isRecording) {
      // 停止录音
      if (recordProcessorRef.current) {
        recordProcessorRef.current.disconnect()
        recordProcessorRef.current.onaudioprocess = null
        recordProcessorRef.current = null
      }
      if (recordSourceRef.current) {
        recordSourceRef.current.disconnect()
        recordSourceRef.current = null
      }
      if (recordStreamRef.current) {
        recordStreamRef.current.getTracks().forEach((track) => track.stop())
        recordStreamRef.current = null
      }
      if (recordAudioContextRef.current) {
        void recordAudioContextRef.current.close()
        recordAudioContextRef.current = null
      }
      setIsRecording(false)

      // 发送录音数据到 ASR
      if (recordPCMRef.current.length === 0) {
        setComposerMessage('未采集到有效音频，请重试。')
        return
      }

      const pcmInt16 = new Int16Array(recordPCMRef.current)
      const audioBlob = new Blob([pcmInt16], { type: 'audio/pcm' })
      recordPCMRef.current = []

      setIsRecognizing(true)
      setComposerMessage('正在识别语音...')

      try {
        const result = await recognizeSpeech(accessToken, audioBlob, 'pcm', 16000, 'zh-CN')
        if (result.text) {
          setComposer(result.text)
          setComposerMessage(`识别完成（置信度 ${(result.confidence * 100).toFixed(0)}%），可编辑后发送。`)
        } else {
          setComposerMessage('未识别到有效内容，请重试。')
        }
      } catch (error) {
        setComposerMessage(extractErrorMessage(error, '语音识别失败，请稍后重试'))
      } finally {
        setIsRecognizing(false)
      }
      return
    }

    // 开始录音
    try {
      const stream = await navigator.mediaDevices.getUserMedia({ audio: true })
      const audioContext = new AudioContext({ sampleRate: 16000 })
      await audioContext.resume()

      const source = audioContext.createMediaStreamSource(stream)
      const processor = audioContext.createScriptProcessor(4096, 1, 1)

      recordPCMRef.current = []
      processor.onaudioprocess = (event) => {
        const channelData = event.inputBuffer.getChannelData(0)
        // Float32 → Int16
        for (let i = 0; i < channelData.length; i++) {
          const sample = Math.max(-1, Math.min(1, channelData[i]))
          recordPCMRef.current.push(sample < 0 ? sample * 0x8000 : sample * 0x7FFF)
        }
      }

      source.connect(processor)
      processor.connect(audioContext.destination)

      recordStreamRef.current = stream
      recordAudioContextRef.current = audioContext
      recordSourceRef.current = source
      recordProcessorRef.current = processor

      setIsRecording(true)
      setComposerMessage('正在录音，再次点击麦克风停止...')
    } catch (error) {
      setComposerMessage(extractErrorMessage(error, '麦克风权限获取失败，请检查浏览器设置'))
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
    if (isPlanGenerating) {
      setComposerMessage(currentPlanQuery.data?.task_error || '学习计划仍在生成中，等任务落库后再让陪伴助手继续拆解。')
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
    stopCurrentPlayback()
    setHistory((current) => [...current, userMessage])

    setSending(true)

    try {
      const reply = await sendCompanionChatRequest(
        accessToken,
        [...history, userMessage],
        currentPlanQuery.data || null,
        focusedTask,
        dailyDigest,
        selectedLive2DModelKey,
        {
          deriveTodayGoals,
          deriveActiveGoals,
        },
      )
      const replyContent = reply.reply || reply.content || '我在，你继续说。'
      appendAssistantMessage(
        {
          id: `assistant-${Date.now()}`,
          role: 'assistant',
          content: replyContent,
          emotion: reply.emotion || reply.mood || '',
          action: reply.action || '',
          live2dDirective: reply.live2d_directive || null,
          createdAt: Date.now(),
        },
        'typing',
        reply.audio_url,
      )
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
                    {isPlanGenerating ? '计划生成中' : (currentPlanQuery.data ? `${Math.round(currentPlanQuery.data.progress || 0)}%` : '--')}
                  </span>
                </div>
                {isPlanGenerating ? (
                  <div className="timeline-item">
                    <strong>学习计划生成中</strong>
                    <p>{currentPlanQuery.data?.task_error || '系统正在异步整理完整计划、阶段蓝图和任务清单，当前页面会自动刷新；后续拆分为独立计划服务后，仍沿用同一消息契约。'}</p>
                  </div>
                ) : null}
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
                {currentPlanQuery.data?.phase ? <p className="companion-empty-text">当前阶段：{formatCompanionPhaseLabel(currentPlanQuery.data.phase)}</p> : null}
                {currentPlanQuery.data?.phase_goal ? <p className="companion-empty-text">阶段目标：{currentPlanQuery.data.phase_goal}</p> : null}
                {currentPlanQuery.data ? <CompanionPlanPhaseSection plan={currentPlanQuery.data} compact /> : null}
                <div className="page-actions">
                  <button
                    className="secondary-button"
                    type="button"
                    disabled={!currentPlanQuery.data?.id || adjustPlanMutation.isPending || isPlanGenerating}
                    onClick={() => void handleAdjustPlan()}
                  >
                    {adjustPlanMutation.isPending ? '调整中...' : '重新调整计划'}
                  </button>
                  {!currentPlanQuery.data?.id ? (
                    <Link className="secondary-link" to="/companion">
                      去入口页生成计划
                    </Link>
                  ) : null}
                </div>
              </article>
            )}

            <article className="status-card companion-goal-card">
              <div className="companion-card-head">
                <div>
                  <span className="section-kicker">当前续接</span>
                  <h2>{isPlanGenerating ? '等待计划生成完成' : (focusedTask ? focusedTask.title : '等待任务接入')}</h2>
                </div>
                <span className="companion-card-note">{isPlanGenerating ? '计划生成中' : (focusedTask ? taskStatusLabel(focusedTask.status) : '暂无聚焦任务')}</span>
              </div>
              <p className="companion-empty-text">
                {isPlanGenerating
                  ? (currentPlanQuery.data?.task_error || '系统会在计划生成完成后自动补齐今日续接任务，你现在可以先停留在本页等待刷新。')
                  : (focusedTask?.description || dailyDigestText)}
              </p>
              {!isPlanGenerating && focusedTask?.phase ? <p className="companion-empty-text">所处阶段：{formatCompanionPhaseLabel(focusedTask.phase)}</p> : null}
              {!isPlanGenerating && focusedTask?.phase_goal ? <p className="companion-empty-text">阶段目标：{focusedTask.phase_goal}</p> : null}
              {!isPlanGenerating && phaseAdjustmentHint ? <p className="companion-empty-text">阶段调整说明：{phaseAdjustmentHint}</p> : null}
              {!isPlanGenerating && focusedTask?.source_label ? <p className="companion-empty-text">任务来源：{focusedTask.source_label}</p> : null}
              {!isPlanGenerating && focusedTask?.reason ? <p className="companion-empty-text">安排原因：{focusedTask.reason}</p> : null}
              {!isPlanGenerating && focusedTask?.priority_explanation ? <p className="companion-empty-text">优先级说明：{focusedTask.priority_explanation}</p> : null}
              {!isPlanGenerating && focusedTask?.collection_hint ? <p className="companion-empty-text">建议题单：{resolvePracticeQuestionSetTitle(focusedTask.collection_hint)}</p> : null}
              {!isPlanGenerating && focusedTask?.source_ref ? <p className="companion-empty-text">来源引用：{focusedTask.source_ref}</p> : null}
              <div className="companion-hub-meta">
                <span>{isPlanGenerating ? '计划生成完成后，这里会展示最新续接摘要。' : dailyDigestText}</span>
                {focusTaskDraft?.updatedAt ? <span>最近续接：{formatCompanionDateTime(focusTaskDraft.updatedAt)}</span> : null}
              </div>
              {!isPlanGenerating && focusedTask?.collection_hint ? (
                <div className="page-actions">
                  <Link
                    className="secondary-link"
                    to="/practice"
                    search={buildPracticeRouteSearch({
                      questionSetSlug: focusedTask.collection_hint,
                      source: 'practice_recommendation',
                      title: focusedTask.title,
                      reason: focusedTask.reason,
                    })}
                  >
                    去刷建议题单
                  </Link>
                </div>
              ) : null}
              <div className="companion-quick-actions">
                {quickPrompts.map((item) => (
                  <button className="secondary-button" disabled={isPlanGenerating} key={item.label} type="button" onClick={() => handleApplyQuickPrompt(item.content)}>
                    {item.label}
                  </button>
                ))}
              </div>
            </article>

            {feedbackTask ? (
              <CompanionTaskFeedbackPanel
                task={feedbackTask}
                draft={feedbackDraft}
                pending={updateTaskMutation.isPending && taskActionTaskId === feedbackTask.id}
                message={planActionMessage}
                onChange={setFeedbackDraft}
                onSubmit={() => void handleSubmitTaskFeedback()}
                onCancel={() => {
                  setFeedbackTask(null)
                  setFeedbackDraft(buildDefaultCompanionTaskFeedbackDraft(null, null))
                  setPlanActionMessage('已取消本次反馈填写，你也可以稍后再记录。')
                }}
              />
            ) : null}

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
                emptyText={isPlanGenerating ? '计划生成完成后，这里会自动接入今日最值得先推进的任务。' : (accessToken ? '当前没有识别到今日目标，可以先去生成学习计划。' : '登录后会自动同步你的今日学习目标。')}
                onStatusChange={handleTaskStatusChange}
                pendingTaskId={taskActionTaskId}
                onContinueTask={(task) => handleApplyQuickPrompt(buildCompanionContinuePrompt(currentPlanQuery.data || null, task))}
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
                emptyText={isPlanGenerating ? '计划生成完成前，当前不会暴露可继续推进的执行任务。' : (accessToken ? '当前没有进行中的任务，陪伴助手会把下一项未完成目标顶上来。' : '登录后会显示你当前正在推进的任务。')}
                onStatusChange={handleTaskStatusChange}
                pendingTaskId={taskActionTaskId}
                onContinueTask={(task) => handleApplyQuickPrompt(buildCompanionContinuePrompt(currentPlanQuery.data || null, task))}
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
          resetKeys={[stageEnabled, workspaceIndustryCode, history.length]}
        >
          <div className="companion-stage-shell">
            {stageEnabled ? (
              <CompanionLive2DStage
                dialogue={stageDialogue}
                isTyping={isStageDialogueTyping}
                emotion={stageFeedback.emotion}
                action={stageFeedback.action}
                mouthOpen={stageMouthOpen}
                directive={latestAssistantMessage?.live2dDirective || null}
                loggedIn={Boolean(accessToken)}
                industryCode={workspaceIndustryCode}
                selectedModelKey={selectedLive2DModelKey}
                onChangeModelKey={setSelectedLive2DModelKey}
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
                <span className="companion-card-note">{isPlanGenerating ? '等待计划生成' : (sending ? '陪伴助手思考中…' : 'Enter 发送')}</span>
              </div>

              <form className="companion-composer" onSubmit={handleSubmit}>
                <textarea
                  value={composer}
                  onChange={(event) => setComposer(event.target.value)}
                  placeholder={isPlanGenerating ? '学习计划仍在生成中，任务落库后再继续和陪伴助手推进执行。' : (focusedTask ? `例如：帮我继续推进「${focusedTask.title}」，或者总结一下今天还差什么没完成。` : '例如：帮我安排今晚的 Go 并发复习顺序，或者总结一下今天还差什么没完成。')}
                  rows={4}
                  disabled={isPlanGenerating || sending}
                />
                <div className="companion-quick-actions">
                  {quickPrompts.map((item) => (
                    <button className="secondary-button" disabled={isPlanGenerating} key={item.label} type="button" onClick={() => handleApplyQuickPrompt(item.content)}>
                      {item.label}
                    </button>
                  ))}
                </div>
                <div className="companion-composer-actions">
                  <p className="companion-composer-message">
                    {composerMessage || (isPlanGenerating
                      ? (currentPlanQuery.data?.task_error || '系统正在异步生成学习计划，生成完成后会自动开放陪伴输入。')
                      : (accessToken ? '已登录，可直接使用 AI 陪伴接口。' : '未登录时会显示本地提示，不会请求后端陪伴接口。'))}
                  </p>
                  <div className="companion-composer-buttons">
                    <button
                      className={`secondary-button companion-mic-button ${isRecording ? 'is-recording' : ''}`}
                      type="button"
                      disabled={sending || isPlanGenerating || isRecognizing}
                      onClick={handleToggleRecording}
                      title={isRecording ? '点击停止录音' : '点击开始语音输入'}
                    >
                      {isRecognizing ? '识别中...' : (isRecording ? '⏹ 停止' : '🎤 语音')}
                    </button>
                    <button className="primary-button" type="submit" disabled={sending || isPlanGenerating || isRecording}>
                      {isPlanGenerating ? '等待计划完成...' : (sending ? '发送中...' : '发送给陪伴助手')}
                    </button>
                  </div>
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
