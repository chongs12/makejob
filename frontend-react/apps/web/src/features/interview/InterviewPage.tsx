import type { FormEvent } from 'react'
import { useEffect, useMemo, useRef, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link, useNavigate, useParams } from '@tanstack/react-router'
import { extractErrorMessage } from '@makejob/api-client'
import { useAuthStore } from '../../state/auth'
import { InterviewLive2DStage } from './InterviewLive2DStage'
import {
  createInterviewRequest,
  fetchInterviewDetail,
  fetchInterviewHistory,
  fetchInterviewReport,
  fetchNextInterviewQuestion,
  finishInterviewRequest,
  submitInterviewAnswer,
} from './interviewApi'
import {
  appendInterviewMessage,
  buildDefaultInterviewTopics,
  buildInitialInterviewForm,
  buildInterviewReadiness,
  buildInterviewReplayItems,
  buildInterviewReviewDraft,
  buildInterviewWebSocketUrl,
  buildRealtimeInterviewMessage,
  encodePCM16Base64,
  estimateInterviewDialogueDurationMs,
  formatInterviewDateTime,
  formatInterviewDuration,
  interviewDifficultyLabel,
  interviewQuestionTypeLabel,
  interviewStatusLabel,
  INTERVIEW_AUTO_STOP_LEVEL_THRESHOLD,
  INTERVIEW_AUTO_STOP_SILENCE_MS,
  normalizeInterviewDimensions,
  parseInterviewTopics,
  resolveCurrentInterviewQuestion,
  resolveCurrentInterviewQuestionFromMessages,
  splitInterviewDialogueUnits,
} from './interviewHelpers'
import type {
  InterviewAnswerResponse,
  InterviewConfigForm,
  InterviewDetailResponse,
  InterviewFeedback,
  InterviewHistoryItem,
  InterviewMessage,
  InterviewNextQuestionResponse,
  InterviewQuestion,
  InterviewReport,
  InterviewReportResponse,
  InterviewSocketASRPayload,
  InterviewSocketEvent,
  InterviewSocketExpressionPayload,
  InterviewSocketQuestionPayload,
  InterviewSocketStatePayload,
  InterviewSocketTTSPayload,
} from './interviewTypes'
import {
  DEFAULT_FRONTEND_INDUSTRY_CODE as INTERVIEW_DEFAULT_INDUSTRY_CODE,
  fetchFrontendIndustries,
  formatFrontendIndustryLabel,
  persistSelectedFrontendIndustryCode,
  readSelectedFrontendIndustryCode,
  resolvePreferredFrontendIndustry,
  subscribeFrontendIndustryCodeChange,
} from '../../shared/industryContext'
import { readCurrentBrowserPath } from '../../shared/authRedirect'
import {
  buildInterviewCompanionContextDraft,
  persistCompanionPlanContext,
} from '../../shared/companionContext'
import { persistCommunityDraft } from '../../shared/communityDraft'
import { requestLoginPrompt } from '../../shared/loginPrompt'
import { persistPendingPracticeSearch } from '../../shared/practiceSearch'

/**
 * 优先使用浏览器剪贴板能力复制文本，失败时让上层统一兜底提示。
 */
async function copyInterviewText(text: string): Promise<void> {
  if (typeof navigator === 'undefined' || !navigator.clipboard?.writeText) {
    throw new Error('当前浏览器不支持剪贴板写入')
  }

  await navigator.clipboard.writeText(text)
}

/**
 * 渲染 AI 面试入口页，承接创建会话与历史记录查看。
 */
export function InterviewHubPage() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const accessToken = useAuthStore((state) => state.accessToken)
  const user = useAuthStore((state) => state.user)
  const [form, setForm] = useState<InterviewConfigForm>(() => buildInitialInterviewForm())
  const [selectedIndustryCode, setSelectedIndustryCode] = useState(() => readSelectedFrontendIndustryCode() || INTERVIEW_DEFAULT_INDUSTRY_CODE)
  const [message, setMessage] = useState('先选择目标方向，再开始这场文本模拟面试。')

  const industriesQuery = useQuery({
    queryKey: ['frontend-industries'],
    queryFn: fetchFrontendIndustries,
    staleTime: 5 * 60 * 1000,
  })

  const historyQuery = useQuery({
    queryKey: ['interview-history', accessToken],
    queryFn: () => fetchInterviewHistory(accessToken as string),
    enabled: Boolean(accessToken),
    retry: false,
  })

  const selectedIndustry = useMemo(
    () => resolvePreferredFrontendIndustry(industriesQuery.data || [], selectedIndustryCode, INTERVIEW_DEFAULT_INDUSTRY_CODE),
    [industriesQuery.data, selectedIndustryCode],
  )
  const effectiveIndustryCode = selectedIndustry?.code || selectedIndustryCode.trim() || INTERVIEW_DEFAULT_INDUSTRY_CODE
  const effectiveIndustryLabel = formatFrontendIndustryLabel(selectedIndustry, effectiveIndustryCode)

  const createMutation = useMutation({
    mutationFn: (payload: {
      industry_code: string
      difficulty: string
      topics: string[]
      question_count: number
    }) => createInterviewRequest(accessToken as string, payload),
    onSuccess: async (data) => {
      setMessage('面试会话已创建，正在进入面试页。')
      await queryClient.invalidateQueries({ queryKey: ['interview-history'] })
      navigate({
        to: '/interview/$interviewId',
        params: {
          interviewId: String(data.interview_id),
        },
      })
    },
    onError: (error) => {
      setMessage(extractErrorMessage(error, '创建面试失败，请稍后重试'))
    },
  })

  const ongoingInterview = useMemo(
    () => historyQuery.data?.list.find((item) => item.status === 'ongoing') || null,
    [historyQuery.data],
  )

  /**
   * 在行业列表加载后归一化当前选中的行业编码，并同步写回前台公共偏好。
   */
  useEffect(() => {
    const normalizedIndustryCode = effectiveIndustryCode.trim()
    if (!normalizedIndustryCode) {
      return
    }

    persistSelectedFrontendIndustryCode(normalizedIndustryCode)
    if (normalizedIndustryCode !== selectedIndustryCode) {
      setSelectedIndustryCode(normalizedIndustryCode)
    }
  }, [effectiveIndustryCode, selectedIndustryCode])

  /**
   * 切换目标行业时重置推荐主题，避免仍然保留上一方向的面试范围。
   */
  function handleIndustryChange(nextIndustryCode: string): void {
    setSelectedIndustryCode(nextIndustryCode)
    setForm((current) => ({
      ...current,
      topicsText: buildDefaultInterviewTopics(nextIndustryCode),
    }))
    setMessage(`已切换到 ${formatFrontendIndustryLabel(resolvePreferredFrontendIndustry(industriesQuery.data || [], nextIndustryCode), nextIndustryCode)} 面试方向。`)
  }

  /**
   * 提交面试配置表单，并创建新的 AI 面试会话。
   */
  async function handleCreateInterview(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()

    if (!accessToken) {
      requestLoginPrompt('/interview', 'missing')
      return
    }

    const topics = parseInterviewTopics(form.topicsText)
    if (topics.length === 0) {
      setMessage(`至少填写一个主题，例如 ${buildDefaultInterviewTopics(effectiveIndustryCode).split(',')[0]}。`)
      return
    }

    setMessage('Ariu 正在准备你的第一道面试题...')
    try {
      await createMutation.mutateAsync({
        industry_code: effectiveIndustryCode,
        difficulty: form.difficulty,
        topics,
        question_count: Number(form.questionCount) || 5,
      })
    } catch {
      if (!useAuthStore.getState().accessToken) {
        requestLoginPrompt('/interview', 'expired')
      }
    }
  }

  return (
    <section className="page-panel interview-page-panel">
      <div className="interview-hero">
        <div className="interview-hero-copy">
          <span className="page-tag">AI 面试主链路</span>
          <h1>{user?.username ? `${user.username}，开始一场 ${effectiveIndustryLabel} 模拟面试` : `开始一场 ${effectiveIndustryLabel} 模拟面试`}</h1>
          <p className="page-copy">
            当前已经支持实时面试闭环：选择行业、配置题量和主题，进入会话后逐题作答，系统直接推进下一题，并在结束后统一生成报告。
          </p>
          <div className="interview-metrics">
            <article className="metric-card">
              <strong>{historyQuery.data?.total ?? '--'}</strong>
              <span>历史面试次数</span>
            </article>
            <article className="metric-card">
              <strong>{ongoingInterview ? '1' : '0'}</strong>
              <span>进行中会话</span>
            </article>
            <article className="metric-card">
              <strong>{effectiveIndustryLabel}</strong>
              <span>当前目标方向</span>
            </article>
          </div>
        </div>

        <article className="section-card interview-sidecard">
          <span className="section-kicker">当前阶段策略</span>
          <div className="timeline-list">
            <div className="timeline-item">
              <strong>先把文本面试跑通</strong>
              <p>优先完成创建、答题、下一题、结束、报告这条主链路，不先做语音和动作系统。</p>
            </div>
            <div className="timeline-item">
              <strong>现在直接切换真实行业</strong>
              <p>面试页已接行业列表和公共偏好，后续 companion、practice 会沿用同一份方向上下文。</p>
            </div>
            <div className="timeline-item">
              <strong>后端已接 AI runtime</strong>
              <p>前台现在要做的是把已有接口变成完整产品流，而不是继续停留在占位页。</p>
            </div>
          </div>
        </article>
      </div>

      <div className="interview-hub-board">
        <section className="status-card interview-builder">
            <div className="companion-card-head">
              <div>
                <span className="section-kicker">创建面试</span>
                <h2>先定难度、题量和想覆盖的主题</h2>
              </div>
              <span className="companion-card-note">当前行业：{effectiveIndustryLabel}</span>
            </div>

            <form className="stack-form" onSubmit={handleCreateInterview}>
              <div className="interview-form-grid">
                <label className="field">
                  <span>目标方向</span>
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

                <label className="field">
                  <span>难度</span>
                  <select
                  value={form.difficulty}
                  onChange={(event) => setForm((current) => ({ ...current, difficulty: event.target.value }))}
                >
                  <option value="easy">{interviewDifficultyLabel('easy')}</option>
                  <option value="medium">{interviewDifficultyLabel('medium')}</option>
                  <option value="hard">{interviewDifficultyLabel('hard')}</option>
                  <option value="mixed">{interviewDifficultyLabel('mixed')}</option>
                </select>
              </label>

              <label className="field">
                <span>题量</span>
                <input
                  type="number"
                  min={3}
                  max={20}
                  value={form.questionCount}
                  onChange={(event) => setForm((current) => ({ ...current, questionCount: event.target.value }))}
                />
              </label>

              {industriesQuery.isError ? (
                <p className="companion-empty-text">
                  {extractErrorMessage(industriesQuery.error, '行业列表读取失败，当前将回退到默认方向。')}
                </p>
              ) : null}
            </div>

            <label className="field">
              <span>主题（逗号或换行分隔）</span>
              <textarea
                rows={4}
                value={form.topicsText}
                onChange={(event) => setForm((current) => ({ ...current, topicsText: event.target.value }))}
                placeholder={`例如：${buildDefaultInterviewTopics(effectiveIndustryCode)}`}
              />
            </label>

            <div className="page-actions">
              <button className="primary-button" type="submit" disabled={createMutation.isPending}>
                {createMutation.isPending ? '创建中...' : '开始这场面试'}
              </button>
              {ongoingInterview ? (
                <Link
                  className="secondary-button hero-link-button"
                  to="/interview/$interviewId"
                  params={{ interviewId: String(ongoingInterview.id) }}
                >
                  继续进行中的会话
                </Link>
              ) : null}
            </div>
            <p className="companion-composer-message">
              {message || (accessToken ? '配置完成后即可开始文本面试。' : '登录后可创建和查看你的面试会话。')}
            </p>
          </form>
        </section>

        <section className="status-card interview-history-panel">
          <div className="companion-card-head">
            <div>
              <span className="section-kicker">历史记录</span>
              <h2>最近的 AI 面试会话</h2>
            </div>
            <span className="companion-card-note">{accessToken ? '已同步登录态面试记录' : '登录后显示'}</span>
          </div>

          {!accessToken ? (
            <div className="timeline-item">
              <strong>请先登录</strong>
              <p>面试接口需要登录态。登录后你可以创建新会话，也能回到上次尚未完成的面试。</p>
              <button className="secondary-link interactive-link-button" type="button" onClick={() => requestLoginPrompt('/interview', 'missing')}>
                前往登录
              </button>
            </div>
          ) : null}

          {historyQuery.isLoading ? <p className="companion-empty-text">面试历史加载中...</p> : null}
          {historyQuery.isError ? (
            <p className="companion-empty-text">
              {historyQuery.error instanceof Error ? historyQuery.error.message : '面试历史加载失败'}
            </p>
          ) : null}

          {historyQuery.data?.list?.length ? (
            <div className="interview-history-list">
              {historyQuery.data.list.map((item) => (
                <article className="interview-history-item" key={item.id}>
                  <div className="card-inline">
                    <strong>面试 #{item.id}</strong>
                    <span>{interviewStatusLabel(item.status)}</span>
                  </div>
                  <p>
                    题量 {item.total_questions} 题
                    {item.score ? ` · 得分 ${Math.round(item.score)}` : ''}
                  </p>
                  <p>开始时间：{formatInterviewDateTime(item.started_at || item.created_at)}</p>
                  <div className="page-actions">
                    {item.status === 'ongoing' ? (
                      <Link className="secondary-link" to="/interview/$interviewId" params={{ interviewId: String(item.id) }}>
                        继续面试
                      </Link>
                    ) : (
                      <Link className="secondary-link" to="/interview/$interviewId/report" params={{ interviewId: String(item.id) }}>
                        查看报告
                      </Link>
                    )}
                  </div>
                </article>
              ))}
            </div>
          ) : null}

          {accessToken && !historyQuery.isLoading && !historyQuery.data?.list?.length ? (
              <div className="timeline-item">
                <strong>还没有面试记录</strong>
                <p>从左侧创建第一场模拟面试，系统会自动保存历史记录和后续报告。</p>
              </div>
            ) : null}
        </section>
      </div>
    </section>
  )
}

/**
 * 渲染 AI 面试进行页，负责串起实时问答、语音识别、TTS 与 Live2D 面试官。
 */
export function InterviewSessionPage() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const accessToken = useAuthStore((state) => state.accessToken)
  const params = useParams({ strict: false })
  const interviewId = String(params.interviewId || '')
  const [answer, setAnswer] = useState('')
  const [isHistoryExpanded, setIsHistoryExpanded] = useState(false)
  const [message, setMessage] = useState('实时面试链路初始化中。')
  const [runtimeMessages, setRuntimeMessages] = useState<InterviewMessage[]>([])
  const [sessionState, setSessionState] = useState<InterviewSocketStatePayload>({
    status: 'idle',
    message: '正在准备实时面试链路。',
  })
  const [stageEmotion, setStageEmotion] = useState('neutral')
  const [liveDialogue, setLiveDialogue] = useState('连接成功后，AI 面试官会在这里播报当前题目。')
  const [isDialogueTyping, setIsDialogueTyping] = useState(false)
  const [mouthOpen, setMouthOpen] = useState(0)
  const [recognitionPartial, setRecognitionPartial] = useState('')
  const [recognitionFinal, setRecognitionFinal] = useState('')
  const [wsTraceId, setWsTraceId] = useState('')
  const [wsConnected, setWsConnected] = useState(false)
  const [isRecording, setIsRecording] = useState(false)
  const wsRef = useRef<WebSocket | null>(null)
  const audioContextRef = useRef<AudioContext | null>(null)
  const analyserRef = useRef<AnalyserNode | null>(null)
  const analyserFrameRef = useRef<number | null>(null)
  const audioElementRef = useRef<HTMLAudioElement | null>(null)
  const dialogueFrameRef = useRef<number | null>(null)
  const dialoguePlaybackTokenRef = useRef(0)
  const recordStreamRef = useRef<MediaStream | null>(null)
  const recordAudioContextRef = useRef<AudioContext | null>(null)
  const recordSourceRef = useRef<MediaStreamAudioSourceNode | null>(null)
  const recordProcessorRef = useRef<ScriptProcessorNode | null>(null)
  const recordSilenceTimeoutRef = useRef<number | null>(null)
  const recordSpeechDetectedRef = useRef(false)
  const recordStopRequestedRef = useRef(false)

  const detailQuery = useQuery({
    queryKey: ['interview-detail', accessToken, interviewId],
    queryFn: () => fetchInterviewDetail(accessToken as string, interviewId),
    enabled: Boolean(accessToken && interviewId),
    retry: false,
    refetchOnWindowFocus: false,
  })
  const submitMutation = useMutation({
    mutationFn: (content: string) => submitInterviewAnswer(accessToken as string, interviewId, content),
    onSuccess: async (data) => {
      setAnswer('')
      setRecognitionFinal('')
      setRecognitionPartial('')
      setMessage(data.is_finished ? '本场面试题目已完成，建议现在生成报告。' : '答案已提交，下一题已准备好。')
      await queryClient.invalidateQueries({ queryKey: ['interview-detail', accessToken, interviewId] })
      await queryClient.invalidateQueries({ queryKey: ['interview-history'] })
    },
    onError: (error) => {
      setMessage(extractErrorMessage(error, '提交回答失败，请稍后重试'))
    },
  })

  const nextQuestionMutation = useMutation({
    mutationFn: () => fetchNextInterviewQuestion(accessToken as string, interviewId),
    onSuccess: async (data) => {
      setMessage(data.question ? `已恢复到第 ${data.question_no} 题。` : '当前没有更多题目，可直接结束面试生成报告。')
      setRecognitionFinal('')
      setRecognitionPartial('')
      await queryClient.invalidateQueries({ queryKey: ['interview-detail', accessToken, interviewId] })
    },
    onError: (error) => {
      setMessage(extractErrorMessage(error, '获取下一题失败，请稍后重试'))
    },
  })

  const finishMutation = useMutation({
    mutationFn: () => finishInterviewRequest(accessToken as string, interviewId),
    onSuccess: async () => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['interview-detail', accessToken, interviewId] }),
        queryClient.invalidateQueries({ queryKey: ['interview-history'] }),
      ])
      navigate({
        to: '/interview/$interviewId/report',
        params: {
          interviewId,
        },
      })
    },
    onError: (error) => {
      setMessage(extractErrorMessage(error, '结束面试失败，请稍后重试'))
    },
  })

  const effectiveMessages = runtimeMessages.length ? runtimeMessages : (detailQuery.data?.messages || [])
  const effectiveStatus = detailQuery.data?.status || 'ongoing'
  const currentQuestion = useMemo(
    () => resolveCurrentInterviewQuestionFromMessages(effectiveMessages, effectiveStatus, detailQuery.data?.total_questions || 0),
    [detailQuery.data?.total_questions, effectiveMessages, effectiveStatus],
  )
  const sessionIndustryCode = detailQuery.data?.industry_code || readSelectedFrontendIndustryCode() || INTERVIEW_DEFAULT_INDUSTRY_CODE
  const canRecord = typeof navigator !== 'undefined' && Boolean(navigator.mediaDevices?.getUserMedia)
  /**
   * 当面试详情恢复成功后，同步写回当前会话所属行业，供其他频道复用同一方向偏好。
   */
  useEffect(() => {
    if (!detailQuery.data?.industry_code) {
      return
    }

    persistSelectedFrontendIndustryCode(detailQuery.data.industry_code)
  }, [detailQuery.data?.industry_code])

  /**
   * 当详情首次恢复或回退链路刷新成功后，同步本地消息快照。
   */
  useEffect(() => {
    if (!detailQuery.data?.messages) {
      return
    }

    setRuntimeMessages(detailQuery.data.messages)
    const restoredQuestion = resolveCurrentInterviewQuestion(detailQuery.data)
    if (restoredQuestion?.question) {
      stopDialogueTyping(restoredQuestion.question)
    }
  }, [detailQuery.data])

  /**
   * 停止当前字幕动画，并按需将字幕直接收敛到目标文本。
   */
  function stopDialogueTyping(finalText?: string): void {
    dialoguePlaybackTokenRef.current += 1
    if (dialogueFrameRef.current) {
      window.cancelAnimationFrame(dialogueFrameRef.current)
      dialogueFrameRef.current = null
    }
    setIsDialogueTyping(false)
    if (typeof finalText === 'string') {
      setLiveDialogue(finalText)
    }
  }

  /**
   * 按兜底时长或音频实际播放进度推进字幕显示，营造近似跟读的打字机效果。
   */
  function startDialogueTyping(text: string, audio?: HTMLAudioElement | null): void {
    const normalizedText = text.trim()
    stopDialogueTyping()

    if (!normalizedText) {
      setLiveDialogue('')
      return
    }

    const units = splitInterviewDialogueUnits(normalizedText)
    const fallbackDurationMs = estimateInterviewDialogueDurationMs(normalizedText)
    const playbackToken = dialoguePlaybackTokenRef.current + 1
    const startedAt = window.performance.now()
    let lastVisibleCount = -1

    dialoguePlaybackTokenRef.current = playbackToken
    setIsDialogueTyping(true)
    setLiveDialogue('')

    /**
     * 逐帧根据音频进度或兜底时长计算当前应显示的字幕长度。
     */
    function syncDialogueFrame(): void {
      if (dialoguePlaybackTokenRef.current !== playbackToken) {
        return
      }

      const audioDurationMs = audio && Number.isFinite(audio.duration) && audio.duration > 0 ? audio.duration * 1000 : 0
      const totalDurationMs = audioDurationMs || fallbackDurationMs
      const elapsedMs = audio
        ? Math.max(audio.currentTime * 1000, window.performance.now() - startedAt)
        : window.performance.now() - startedAt
      const progress = totalDurationMs > 0 ? Math.min(elapsedMs / totalDurationMs, 1) : 1
      const visibleCount = progress >= 1 ? units.length : Math.max(1, Math.ceil(units.length * progress))

      if (visibleCount !== lastVisibleCount) {
        lastVisibleCount = visibleCount
        setLiveDialogue(units.slice(0, visibleCount).join(''))
      }

      if (visibleCount >= units.length || audio?.ended) {
        dialogueFrameRef.current = null
        setIsDialogueTyping(false)
        setLiveDialogue(normalizedText)
        return
      }

      dialogueFrameRef.current = window.requestAnimationFrame(syncDialogueFrame)
    }

    dialogueFrameRef.current = window.requestAnimationFrame(syncDialogueFrame)
  }

  /**
   * 停止上一段音频并释放分析器资源，避免多个音频上下文叠加。
   */
  function stopCurrentPlayback(finalDialogue?: string): void {
    if (analyserFrameRef.current) {
      window.cancelAnimationFrame(analyserFrameRef.current)
      analyserFrameRef.current = null
    }
    analyserRef.current = null
    if (audioElementRef.current) {
      audioElementRef.current.pause()
      audioElementRef.current.src = ''
      audioElementRef.current = null
    }
    if (audioContextRef.current) {
      void audioContextRef.current.close()
      audioContextRef.current = null
    }
    stopDialogueTyping(finalDialogue)
    setMouthOpen(0)
  }

  /**
   * 播放新的 TTS 音频时，用音频分析器驱动 Live2D 嘴型开合。
   */
  async function playTTSAudio(audioUrl: string, text: string): Promise<void> {
    const dialogueText = text.trim()

    stopCurrentPlayback(dialogueText)

    if (!audioUrl) {
      return
    }

    try {
      const AudioContextCtor = window.AudioContext
      if (!AudioContextCtor) {
        setMessage('当前浏览器不支持音频上下文，已退回文本模式。')
        return
      }

      const audio = new Audio(audioUrl)
      audio.preload = 'auto'
      audioElementRef.current = audio

      const audioContext = new AudioContextCtor()
      const analyser = audioContext.createAnalyser()
      analyser.fftSize = 2048
      const source = audioContext.createMediaElementSource(audio)
      source.connect(analyser)
      analyser.connect(audioContext.destination)
      audioContextRef.current = audioContext
      analyserRef.current = analyser

      /**
       * 读取当前音频振幅，并持续同步到嘴型开合值。
       */
      function syncMouthFromAudio(): void {
        if (!analyserRef.current) {
          setMouthOpen(0)
          return
        }

        const analyserNode = analyserRef.current
        const buffer = new Uint8Array(analyserNode.fftSize)
        analyserNode.getByteTimeDomainData(buffer)
        let sum = 0
        for (const value of buffer) {
          sum += Math.abs(value - 128)
        }
        const normalized = Math.min(sum / buffer.length / 26, 1)
        setMouthOpen(normalized)
        analyserFrameRef.current = window.requestAnimationFrame(syncMouthFromAudio)
      }

      audio.onended = () => {
        stopCurrentPlayback(dialogueText)
        setSessionState((current) => (current.status === 'speaking' ? { status: 'ready', message: '语音播报完成，可继续作答。' } : current))
      }
      audio.onerror = () => {
        stopCurrentPlayback(dialogueText)
      }

      await audioContext.resume()
      analyserFrameRef.current = window.requestAnimationFrame(syncMouthFromAudio)
      await audio.play()
      startDialogueTyping(dialogueText, audio)
    } catch (error) {
      stopCurrentPlayback(dialogueText)
      if (!useAuthStore.getState().accessToken) {
        requestLoginPrompt(readCurrentBrowserPath(), 'expired')
        return
      }
      setMessage(extractErrorMessage(error, '自动播放语音失败，请检查浏览器音频权限'))
    }
  }

  /**
   * 清理当前自动判停定时器，避免旧的静音任务误触发下一轮录音。
   */
  function clearRecordSilenceTimer(): void {
    if (recordSilenceTimeoutRef.current !== null) {
      window.clearTimeout(recordSilenceTimeoutRef.current)
      recordSilenceTimeoutRef.current = null
    }
  }

  /**
   * 启动浏览器麦克风采集，并将 16k PCM 音频实时推送到后端 WebSocket。
   */
  async function startVoiceCapture(): Promise<void> {
    if (!canRecord) {
      setMessage('当前浏览器不支持麦克风采集，请直接输入文字作答。')
      return
    }

    if (!wsRef.current || wsRef.current.readyState !== WebSocket.OPEN) {
      setMessage('实时链路尚未连接，暂时无法启动语音识别。')
      return
    }

    try {
      const stream = await navigator.mediaDevices.getUserMedia({
        audio: true,
      })
      const recordContext = new AudioContext({
        sampleRate: 16000,
      })
      const source = recordContext.createMediaStreamSource(stream)
      const processor = recordContext.createScriptProcessor(4096, 1, 1)

      wsRef.current.send(
        JSON.stringify({
          type: 'audio_start',
          data: {
            language: 'zh-CN',
          },
        }),
      )

      recordStopRequestedRef.current = false
      recordSpeechDetectedRef.current = false
      clearRecordSilenceTimer()
      source.connect(processor)
      processor.connect(recordContext.destination)
      processor.onaudioprocess = (event) => {
        if (recordStopRequestedRef.current || !wsRef.current || wsRef.current.readyState !== WebSocket.OPEN) {
          return
        }

        const channelData = event.inputBuffer.getChannelData(0)
        let signalEnergy = 0
        for (const sample of channelData) {
          signalEnergy += sample * sample
        }
        const rms = Math.sqrt(signalEnergy / Math.max(channelData.length, 1))
        if (rms >= INTERVIEW_AUTO_STOP_LEVEL_THRESHOLD) {
          recordSpeechDetectedRef.current = true
          clearRecordSilenceTimer()
        } else if (recordSpeechDetectedRef.current && recordSilenceTimeoutRef.current === null) {
          recordSilenceTimeoutRef.current = window.setTimeout(() => {
            recordSilenceTimeoutRef.current = null
            if (recordStopRequestedRef.current || !recordSpeechDetectedRef.current) {
              return
            }

            setMessage('检测到你已停顿，正在自动结束并提交本轮回答。')
            stopVoiceCapture('auto')
          }, INTERVIEW_AUTO_STOP_SILENCE_MS)
        }
        wsRef.current.send(
          JSON.stringify({
            type: 'audio_chunk',
            data: {
              audio_base64: encodePCM16Base64(channelData),
            },
          }),
        )
      }

      recordStreamRef.current = stream
      recordAudioContextRef.current = recordContext
      recordSourceRef.current = source
      recordProcessorRef.current = processor
      setRecognitionPartial('')
      setRecognitionFinal('')
      setIsRecording(true)
      setMessage('正在实时识别你的回答，请继续说；停顿后会自动提交。')
    } catch (error) {
      setMessage(extractErrorMessage(error, '麦克风权限申请失败，请检查浏览器设置'))
    }
  }

  /**
   * 停止当前麦克风采集，并按来源决定是否通知后端结束并自动提交本轮 ASR。
   */
  function stopVoiceCapture(reason: 'manual' | 'auto' | 'cleanup' = 'manual'): void {
    const hasActiveRecording = Boolean(
      recordProcessorRef.current || recordSourceRef.current || recordStreamRef.current || recordAudioContextRef.current,
    )
    clearRecordSilenceTimer()
    recordSpeechDetectedRef.current = false
    if (!hasActiveRecording) {
      recordStopRequestedRef.current = false
      setIsRecording(false)
      return
    }

    recordStopRequestedRef.current = true
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
    if (reason !== 'cleanup' && wsRef.current && wsRef.current.readyState === WebSocket.OPEN) {
      wsRef.current.send(
        JSON.stringify({
          type: 'audio_end',
        }),
      )
      setMessage(
        reason === 'auto'
          ? '检测到你已停顿，正在自动提交你的语音回答。'
          : '录音已结束，正在自动提交你的语音回答。',
      )
    }
    setIsRecording(false)
    recordStopRequestedRef.current = false
  }

  /**
   * 订阅面试 WebSocket 事件，并在页面侧同步更新题目、语音和表情状态。
   */
  useEffect(() => {
    if (!accessToken || !interviewId) {
      return undefined
    }

    const socket = new WebSocket(buildInterviewWebSocketUrl(interviewId, accessToken))
    wsRef.current = socket

    socket.onopen = () => {
      setWsConnected(true)
      setMessage('实时面试链路已连接。')
    }

    socket.onmessage = (event) => {
      try {
        const payload = JSON.parse(String(event.data)) as InterviewSocketEvent
        setWsTraceId(payload.trace_id || '')

        switch (payload.type) {
          case 'connected':
            setMessage(payload.content || '实时面试链路已建立。')
            break
          case 'session_ready': {
            const statePayload = payload.data as InterviewSocketStatePayload | undefined
            if (statePayload) {
              setSessionState(statePayload)
              setMessage(statePayload.message || '实时会话已准备好。')
            }
            break
          }
          case 'interview_state': {
            const statePayload = payload.data as InterviewSocketStatePayload | undefined
            if (statePayload) {
              setSessionState(statePayload)
              setMessage(statePayload.message || '')
            }
            break
          }
          case 'user_answer': {
            const answerText = (payload.content || '').trim()
            if (!answerText) {
              break
            }

            setRuntimeMessages((current) =>
              appendInterviewMessage(current, buildRealtimeInterviewMessage('user', 'text', answerText)),
            )
            setAnswer('')
            setRecognitionPartial('')
            setRecognitionFinal('')
            break
          }
          case 'ai_question': {
            const questionPayload = payload.data as InterviewSocketQuestionPayload | undefined
            const questionText = questionPayload?.question || payload.content || ''
            if (!questionText) {
              break
            }

            startDialogueTyping(questionText)
            setRuntimeMessages((current) =>
              appendInterviewMessage(current, buildRealtimeInterviewMessage('ai', 'text', questionText)),
            )
            break
          }
          case 'asr_partial': {
            const asrPayload = payload.data as InterviewSocketASRPayload | undefined
            setRecognitionPartial(asrPayload?.text || payload.content || '')
            break
          }
          case 'asr_final': {
            const asrPayload = payload.data as InterviewSocketASRPayload | undefined
            const recognizedText = asrPayload?.text || payload.content || ''
            setRecognitionPartial('')
            setRecognitionFinal(recognizedText)
            if (recognizedText) {
              setAnswer(recognizedText)
            }
            break
          }
          case 'tts_audio': {
            const ttsPayload = payload.data as InterviewSocketTTSPayload | undefined
            if (ttsPayload?.audio_url) {
              void playTTSAudio(ttsPayload.audio_url, ttsPayload.text || payload.content || '')
            }
            break
          }
          case 'live2d_expression': {
            const expressionPayload = payload.data as InterviewSocketExpressionPayload | undefined
            setStageEmotion(expressionPayload?.emotion || 'neutral')
            break
          }
          case 'finished':
            setMessage(payload.content || '当前题目已完成，可以生成报告。')
            setSessionState({
              status: 'finished',
              message: payload.content || '本场题目已全部完成。',
            })
            break
          case 'error':
            setMessage(payload.content || '实时面试链路发生错误。')
            break
          default:
            break
        }
      } catch {
        setMessage('收到无法解析的实时事件，请检查后端输出格式。')
      }
    }

    socket.onclose = () => {
      setWsConnected(false)
      setMessage('实时面试链路已断开，当前会退回 HTTP 模式，此模式不会触发语音播报。')
    }

    socket.onerror = () => {
      setMessage('实时面试链路连接异常，当前会退回 HTTP 模式，此模式不会触发语音播报。')
    }

    return () => {
      socket.close()
      wsRef.current = null
    }
  }, [accessToken, interviewId])

  /**
   * 在页面卸载时释放音频播放和录音相关资源，避免浏览器残留占用。
   */
  useEffect(() => {
    return () => {
      stopCurrentPlayback()
      stopVoiceCapture('cleanup')
    }
  }, [])

  /**
   * 提交当前问题的答案，优先走 WebSocket 实时链路，断开时回退到 HTTP。
   */
  async function handleSubmitAnswer(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()

    const content = answer.trim()
    if (!content) {
      setMessage('先输入你的回答，再提交给 AI 面试官。')
      return
    }

    if (wsRef.current && wsRef.current.readyState === WebSocket.OPEN) {
      setRuntimeMessages((current) =>
        appendInterviewMessage(current, buildRealtimeInterviewMessage('user', 'text', content)),
      )
      wsRef.current.send(
        JSON.stringify({
          type: 'user_answer',
          content,
        }),
      )
      setAnswer('')
      setRecognitionFinal('')
      setRecognitionPartial('')
      setMessage('答案已提交，AI 正在整理下一题。')
      return
    }

    setMessage('当前实时链路未连接，本次将走 HTTP 回退模式；该模式只返回文本，不会触发 TTS 语音。')
    await submitMutation.mutateAsync(content)
  }

  return (
    <section className="page-panel interview-page-panel interview-session-page-panel">
      <div className="interview-session-layout">
        <div className="interview-stage-shell">
          <Link className="ghost-button interview-stage-floating-back" to="/interview">
            返回面试入口
          </Link>
          <InterviewLive2DStage
            industryCode={sessionIndustryCode}
            dialogue={liveDialogue}
            isTyping={isDialogueTyping}
            emotion={stageEmotion}
            mouthOpen={mouthOpen}
          />
        </div>

        <div className="interview-session-main">
          <section className="status-card interview-answer-panel">
            <div className="companion-card-head">
              <div>
                <span className="section-kicker">回答输入</span>
                <h2>直接回答当前问题</h2>
              </div>
              <span className="companion-card-note">
                {isRecording ? '麦克风采集中，停顿后自动提交...' : (wsConnected ? '实时链路优先' : '当前为 HTTP 回退模式')}
              </span>
            </div>

            <form className="companion-composer" onSubmit={handleSubmitAnswer}>
              <div className="interview-answer-status-stack">
                <p className="interview-answer-status-primary">{sessionState.message || message}</p>
                {recognitionPartial ? <p className="interview-answer-status-secondary">实时识别：{recognitionPartial}</p> : null}
                {recognitionFinal ? <p className="interview-answer-status-secondary">识别结果：{recognitionFinal}</p> : null}
              </div>
              <textarea
                rows={7}
                value={answer}
                onChange={(event) => setAnswer(event.target.value)}
                placeholder="直接输入你的回答。建议先说思路，再补关键点、权衡和落地细节。"
                disabled={detailQuery.data?.status !== 'ongoing'}
              />
              <div className="interview-answer-actions">
                <div className="page-actions">
                  <button
                    className="ghost-button interview-history-trigger"
                    type="button"
                    onClick={() => setIsHistoryExpanded(true)}
                  >
                    {effectiveMessages.length ? `查看记录 (${effectiveMessages.length})` : '查看记录'}
                  </button>
                  <button
                    className="secondary-button"
                    type="button"
                    disabled={!canRecord}
                    onClick={() => {
                      if (isRecording) {
                        stopVoiceCapture()
                        return
                      }
                      void startVoiceCapture()
                    }}
                  >
                    {isRecording ? '手动停止并提交' : '开始语音回答'}
                  </button>
                  <button
                    className="secondary-button"
                    type="button"
                    disabled={nextQuestionMutation.isPending || detailQuery.data?.status !== 'ongoing' || Boolean(currentQuestion)}
                    onClick={() => void nextQuestionMutation.mutateAsync()}
                  >
                    {nextQuestionMutation.isPending ? '恢复中...' : '恢复下一题'}
                  </button>
                  <button
                    className="secondary-button"
                    type="button"
                    disabled={finishMutation.isPending}
                    onClick={() => void finishMutation.mutateAsync()}
                  >
                    {finishMutation.isPending ? '生成报告中...' : '结束面试'}
                  </button>
                  <button
                    className="primary-button"
                    type="submit"
                    disabled={isRecording || submitMutation.isPending || detailQuery.data?.status !== 'ongoing'}
                  >
                    {submitMutation.isPending ? '提交中...' : '手动提交答案'}
                  </button>
                </div>
              </div>
            </form>

            {detailQuery.data?.status === 'completed' ? (
              <div className="timeline-item">
                <strong>当前面试已结束</strong>
                <p>这场面试已经完成，可以直接进入报告页查看总结结果。</p>
                <Link className="secondary-link" to="/interview/$interviewId/report" params={{ interviewId }}>
                  查看面试报告
                </Link>
              </div>
            ) : null}
          </section>
        </div>
      </div>

      {isHistoryExpanded ? (
        <button
          aria-label="关闭面试记录抽屉"
          className="interview-history-drawer-backdrop"
          type="button"
          onClick={() => setIsHistoryExpanded(false)}
        />
      ) : null}

      <aside
        aria-hidden={!isHistoryExpanded}
        className={`status-card interview-history-drawer${isHistoryExpanded ? ' is-open' : ''}`}
      >
        <div className="companion-card-head">
          <div>
            <span className="section-kicker">面试记录</span>
            <h2>完整过程回看</h2>
          </div>
          <div className="page-actions">
            <span className="companion-card-note">{effectiveMessages.length || 0} 条</span>
            <button className="ghost-button interview-history-close" type="button" onClick={() => setIsHistoryExpanded(false)}>
              关闭
            </button>
          </div>
        </div>

        {detailQuery.isLoading ? <p className="companion-empty-text">正在恢复面试记录...</p> : null}
        {detailQuery.isError ? (
          <p className="companion-empty-text">
            {detailQuery.error instanceof Error ? detailQuery.error.message : '面试记录加载失败'}
          </p>
        ) : null}

        {effectiveMessages.length ? (
          <div className="interview-message-list interview-history-drawer-list">
            {effectiveMessages.map((item, index) => (
              <article className={`interview-message-item interview-message-item-${item.role}`} key={`${item.role}-${item.created_at}-${index}`}>
                <div className="interview-message-head">
                  <strong>{item.role === 'ai' ? (item.message_type === 'feedback' ? 'AI 反馈' : 'AI 面试官') : '你'}</strong>
                  <span>{formatInterviewDateTime(item.created_at)}</span>
                </div>
                <p>{item.content}</p>
              </article>
            ))}
          </div>
        ) : null}
      </aside>
    </section>
  )
}

/**
 * 渲染 AI 面试报告页，集中展示得分、维度、优势与待改进点。
 */
export function InterviewReportPage() {
  const navigate = useNavigate()
  const accessToken = useAuthStore((state) => state.accessToken)
  const params = useParams({ strict: false })
  const interviewId = String(params.interviewId || '')
  const [selectedIndustryCode, setSelectedIndustryCode] = useState(() => readSelectedFrontendIndustryCode() || INTERVIEW_DEFAULT_INDUSTRY_CODE)
  const [reportMessage, setReportMessage] = useState('这份报告已经升级为可执行版本，你可以直接继续补弱项或生成复盘。')

  const reportQuery = useQuery({
    queryKey: ['interview-report', accessToken, interviewId],
    queryFn: () => fetchInterviewReport(accessToken as string, interviewId),
    enabled: Boolean(accessToken && interviewId),
    retry: false,
    refetchOnWindowFocus: false,
  })
  const detailQuery = useQuery({
    queryKey: ['interview-report-detail', accessToken, interviewId],
    queryFn: () => fetchInterviewDetail(accessToken as string, interviewId),
    enabled: Boolean(accessToken && interviewId),
    retry: false,
    refetchOnWindowFocus: false,
  })
  const industriesQuery = useQuery({
    queryKey: ['frontend-industries'],
    queryFn: fetchFrontendIndustries,
    staleTime: 5 * 60 * 1000,
  })
  const reportIndustryCode = detailQuery.data?.industry_code || selectedIndustryCode || INTERVIEW_DEFAULT_INDUSTRY_CODE
  const reportIndustry = useMemo(
    () => resolvePreferredFrontendIndustry(industriesQuery.data || [], reportIndustryCode, INTERVIEW_DEFAULT_INDUSTRY_CODE),
    [industriesQuery.data, reportIndustryCode],
  )
  const reportIndustryLabel = formatFrontendIndustryLabel(reportIndustry, reportIndustryCode)
  const report = reportQuery.data?.report || null
  const reportDuration = reportQuery.data?.duration_seconds || 0
  const reportCompletedAt = reportQuery.data?.completed_at
  const dimensionItems = useMemo(() => normalizeInterviewDimensions(report), [report])
  const strongestDimensions = dimensionItems.slice(0, 3)
  const weakestDimensions = [...dimensionItems].reverse().slice(0, 3)
  const replayItems = useMemo(() => buildInterviewReplayItems(detailQuery.data?.messages || []), [detailQuery.data?.messages])
  const readiness = buildInterviewReadiness(report?.overall_score || 0)
  const primaryWeakKeyword = weakestDimensions[0]?.label || report?.weaknesses?.[0] || ''
  const reviewDraft = report ? buildInterviewReviewDraft(report, detailQuery.data, reportIndustryLabel) : null

  /**
   * 订阅前台行业偏好变化，让报告页在同页切换方向后也能同步显示最新名称。
   */
  useEffect(() => {
    const unsubscribe = subscribeFrontendIndustryCodeChange((industryCode) => {
      setSelectedIndustryCode(industryCode || INTERVIEW_DEFAULT_INDUSTRY_CODE)
    })

    return unsubscribe
  }, [])

  /**
   * 当报告页补拿到会话详情后，优先以真实会话行业覆盖本地偏好。
   */
  useEffect(() => {
    if (!detailQuery.data?.industry_code) {
      return
    }

    persistSelectedFrontendIndustryCode(detailQuery.data.industry_code)
    setSelectedIndustryCode(detailQuery.data.industry_code)
  }, [detailQuery.data?.industry_code])

  /**
   * 将当前最弱项带到题库页，便于直接进入针对性补题。
   */
  function handlePracticeFollowUp(keyword: string): void {
    persistPendingPracticeSearch(keyword)
    navigate({
      to: '/practice',
    })
  }

  /**
   * 把当前报告生成的复盘模板写入社区草稿，并直接跳到发帖页。
   */
  function handleCreateCommunityReview(): void {
    if (!reviewDraft) {
      setReportMessage('当前报告还未准备好复盘草稿。')
      return
    }

    if (!useAuthStore.getState().accessToken) {
      requestLoginPrompt('/community/create', 'missing')
      return
    }

    persistCommunityDraft(reviewDraft)
    navigate({
      to: '/community/create',
    })
  }

  /**
   * 复制当前复盘草稿正文，方便用户在站外或其他位置继续编辑。
   */
  async function handleCopyReviewDraft(): Promise<void> {
    if (!reviewDraft) {
      setReportMessage('当前报告还未准备好复盘草稿。')
      return
    }

    try {
      await copyInterviewText(reviewDraft.content)
      setReportMessage('复盘草稿正文已复制，可以直接粘贴到社区或外部文档。')
    } catch (error) {
      setReportMessage(extractErrorMessage(error, '复制复盘草稿失败'))
    }
  }

  /**
   * 将当前面试报告提炼为学习陪伴计划上下文，并跳转到陪伴入口页继续补强。
   */
  function handleCompanionFollowUp(): void {
    if (!report) {
      setReportMessage('当前报告还未加载完成，暂时无法生成强化计划上下文。')
      return
    }

    persistCompanionPlanContext(
      buildInterviewCompanionContextDraft({
        interviewId,
        industryCode: reportIndustryCode,
        industryLabel: reportIndustryLabel,
        overallScore: report.overall_score || 0,
        summary: report.summary || '',
        readinessLabel: readiness.label,
        weakTopics: [
          ...weakestDimensions.map((item) => item.label),
          ...(report.weaknesses || []),
        ],
        suggestions: report.suggestions || [],
      }),
    )
    navigate({
      to: '/companion',
    })
  }

  return (
    <section className="page-panel interview-page-panel">
      <div className="companion-room-toolbar">
        <Link className="ghost-button companion-room-back" to="/interview">
          返回面试入口
        </Link>
        <span className="companion-room-note">当前为面试报告页 · {reportIndustryLabel}</span>
      </div>

      <div className="interview-report-layout">
        <section className="status-card interview-report-main">
          <div className="companion-card-head">
            <div>
              <span className="section-kicker">面试报告</span>
              <h1>面试 #{interviewId} 结果总结</h1>
              <p className="companion-empty-text">所属方向：{reportIndustryLabel}</p>
            </div>
            <span className="companion-card-note">
              {reportCompletedAt ? `完成于 ${formatInterviewDateTime(reportCompletedAt)}` : '等待报告加载'}
            </span>
          </div>

          {reportQuery.isLoading ? <p className="companion-empty-text">报告加载中...</p> : null}
          {reportQuery.isError ? (
            <div className="timeline-item">
              <strong>报告暂不可用</strong>
              <p>{reportQuery.error instanceof Error ? reportQuery.error.message : '面试报告加载失败'}</p>
              <Link className="secondary-link" to="/interview/$interviewId" params={{ interviewId }}>
                返回面试页
              </Link>
            </div>
          ) : null}

          {report ? (
            <>
              <div className="interview-report-metrics">
                <article className="metric-card">
                  <strong>{Math.round(report.overall_score || 0)}</strong>
                  <span>总分</span>
                </article>
                <article className="metric-card">
                  <strong>{report.correct_count}</strong>
                  <span>命中题数</span>
                </article>
                <article className="metric-card">
                  <strong>{report.total_questions}</strong>
                  <span>总题量</span>
                </article>
                <article className="metric-card">
                  <strong>{formatInterviewDuration(reportDuration)}</strong>
                  <span>面试时长</span>
                </article>
              </div>

              <div className="interview-report-action-grid">
                <article className="timeline-item">
                  <strong>当前准备度</strong>
                  <p>{readiness.label}</p>
                  <p>{readiness.description}</p>
                </article>
                <article className="timeline-item">
                  <strong>优先补强项</strong>
                  {weakestDimensions.length ? (
                    <div className="community-tag-row">
                      {weakestDimensions.map((item) => (
                        <span key={item.key}>{item.label} {item.score} 分</span>
                      ))}
                    </div>
                  ) : (
                    <p>当前没有维度评分数据，建议优先复盘总结和建议区内容。</p>
                  )}
                </article>
                <article className="timeline-item">
                  <strong>下一步动作</strong>
                  <div className="page-actions">
                    <button className="secondary-button" type="button" onClick={() => handlePracticeFollowUp(primaryWeakKeyword || reportIndustryLabel)}>
                      去题库补弱项
                    </button>
                    <button className="secondary-button" type="button" onClick={handleCompanionFollowUp}>
                      去生成强化计划
                    </button>
                  </div>
                </article>
              </div>

              <div className="status-card">{reportMessage}</div>

              <div className="timeline-item">
                <strong>总结</strong>
                <p>{report.summary || '当前报告未生成总结。'}</p>
              </div>

              <div className="interview-report-sections">
                <article className="timeline-item">
                  <strong>优势</strong>
                  {report.strengths?.length ? (
                    <ul className="interview-bullet-list">
                      {report.strengths.map((item) => <li key={item}>{item}</li>)}
                    </ul>
                  ) : (
                    <p>当前没有返回优势项。</p>
                  )}
                </article>

                <article className="timeline-item">
                  <strong>待加强点</strong>
                  {report.weaknesses?.length ? (
                    <ul className="interview-bullet-list">
                      {report.weaknesses.map((item) => <li key={item}>{item}</li>)}
                    </ul>
                  ) : (
                    <p>当前没有返回待加强项。</p>
                  )}
                </article>

                <article className="timeline-item">
                  <strong>后续建议</strong>
                  {report.suggestions?.length ? (
                    <ul className="interview-bullet-list">
                      {report.suggestions.map((item) => <li key={item}>{item}</li>)}
                    </ul>
                  ) : (
                    <p>当前没有返回建议项。</p>
                  )}
                </article>
              </div>

              <article className="timeline-item">
                <strong>维度评分</strong>
                {dimensionItems.length ? (
                  <div className="interview-dimension-grid">
                    {dimensionItems.map((item) => (
                      <div className="companion-stat-chip" key={item.key}>
                        <strong>{item.score}</strong>
                        <span>{item.label}</span>
                      </div>
                    ))}
                  </div>
                ) : (
                  <p>当前报告没有返回维度评分。</p>
                )}
              </article>

              <div className="interview-report-sections">
                <article className="timeline-item">
                  <strong>最强维度</strong>
                  {strongestDimensions.length ? (
                    <ul className="interview-bullet-list">
                      {strongestDimensions.map((item) => <li key={item.key}>{item.label} {item.score} 分</li>)}
                    </ul>
                  ) : (
                    <p>当前没有维度评分数据。</p>
                  )}
                </article>

                <article className="timeline-item">
                  <strong>优先补强维度</strong>
                  {weakestDimensions.length ? (
                    <ul className="interview-bullet-list">
                      {weakestDimensions.map((item) => <li key={item.key}>{item.label} {item.score} 分</li>)}
                    </ul>
                  ) : (
                    <p>当前没有维度评分数据。</p>
                  )}
                </article>
              </div>

              <article className="timeline-item">
                <div className="section-head">
                  <div>
                    <strong>社区复盘模板</strong>
                    <p className="companion-empty-text">把这场面试的结果整理成帖子，后续可以继续在社区里补充复盘和讨论。</p>
                  </div>
                  <div className="page-actions">
                    <button className="secondary-button" type="button" onClick={() => void handleCopyReviewDraft()}>
                      复制草稿
                    </button>
                    <button className="primary-button" type="button" onClick={handleCreateCommunityReview}>
                      去社区发复盘
                    </button>
                  </div>
                </div>
                {reviewDraft ? (
                  <div className="analysis-block">{reviewDraft.content}</div>
                ) : (
                  <p>当前没有可生成的复盘模板。</p>
                )}
              </article>

              <article className="timeline-item">
                <strong>答题轨迹</strong>
                {replayItems.length ? (
                  <div className="interview-replay-list">
                    {replayItems.map((item, index) => (
                      <article className="interview-message-item" key={`${item.askedAt}-${index}`}>
                        <div className="interview-message-head">
                          <strong>第 {index + 1} 题</strong>
                          <span>{formatInterviewDateTime(item.answeredAt || item.askedAt)}</span>
                        </div>
                        <p><strong>问题：</strong>{item.question}</p>
                        <p><strong>回答：</strong>{item.answer || '本题未记录到用户回答。'}</p>
                      </article>
                    ))}
                  </div>
                ) : (
                  <p>当前没有可回放的答题轨迹。</p>
                )}
              </article>
            </>
          ) : null}
        </section>

        <aside className="home-side-column">
          <article className="section-card sidebar-card">
            <span className="section-kicker">下一步建议</span>
            <div className="sidebar-links">
              <button className="sidebar-link sidebar-link-button" type="button" onClick={() => handlePracticeFollowUp(primaryWeakKeyword || reportIndustryLabel)}>
                先去补题库弱项
              </button>
              <button className="sidebar-link sidebar-link-button" type="button" onClick={handleCompanionFollowUp}>
                去学习陪伴继续推进计划
              </button>
              <button className="sidebar-link sidebar-link-button" type="button" onClick={() => navigate({ to: '/interview' })}>
                再开一场新的面试
              </button>
              <button className="sidebar-link sidebar-link-button" type="button" onClick={handleCreateCommunityReview}>
                去社区发复盘
              </button>
            </div>
          </article>

          <article className="section-card sidebar-card">
            <span className="section-kicker">报告提示</span>
            <p>
              这版报告会把强弱项、后续建议、题库补练和社区复盘串成一条动作链，建议优先处理最低分维度。
            </p>
          </article>
        </aside>
      </div>
    </section>
  )
}
