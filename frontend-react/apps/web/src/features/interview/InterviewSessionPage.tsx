import type { FormEvent } from 'react'
import { useEffect, useMemo, useRef, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link, useNavigate, useParams } from '@tanstack/react-router'
import { extractErrorMessage } from '@makejob/api-client'
import { useAuthStore } from '../../state/auth'
import { AsyncInlineState } from '../../shared/asyncState'
import { readCurrentBrowserPath } from '../../shared/authRedirect'
import { readSelectedLive2DModelKey } from '../../shared/live2dModelCatalog'
import { usePCMStreamPlayer } from '../../shared/usePCMStreamPlayer'
import { useLive2DDialoguePlayback } from '../../shared/useLive2DDialoguePlayback'
import {
  DEFAULT_FRONTEND_INDUSTRY_CODE as INTERVIEW_DEFAULT_INDUSTRY_CODE,
  persistSelectedFrontendIndustryCode,
  readSelectedFrontendIndustryCode,
} from '../../shared/industryContext'
import { requestLoginPrompt } from '../../shared/loginPrompt'
import { prewarmLive2DRuntime } from '../../shared/live2dRuntime'
import { InterviewLive2DStage } from './InterviewLive2DStage'
import {
  fetchInterviewDetail,
  fetchNextInterviewQuestion,
  finishInterviewRequest,
  submitInterviewAnswer,
  type SubmitInterviewAnswerPayload,
} from './interviewApi'
import {
  appendInterviewMessage,
  buildInterviewWebSocketUrl,
  buildRealtimeInterviewMessage,
  encodePCM16Base64FromInt16,
  formatInterviewDateTime,
  INTERVIEW_AUTO_STOP_LEVEL_THRESHOLD,
  INTERVIEW_AUTO_STOP_SILENCE_MS,
  INTERVIEW_MAX_RECORDING_MS,
  resampleFloat32ToPCM16,
  resolveCurrentInterviewQuestion,
  resolveCurrentInterviewQuestionFromMessages,
} from './interviewHelpers'
import type {
  InterviewCodingProcessEvent,
  InterviewMessage,
  InterviewQuestion,
  InterviewSocketASRPayload,
  InterviewSocketAssistantAudioChunkPayload,
  InterviewSocketAssistantTranscriptPayload,
  InterviewSocketAssistantTurnPayload,
  InterviewSocketEvent,
  InterviewSocketExpressionPayload,
  InterviewSocketQuestionPayload,
  InterviewSocketStatePayload,
  InterviewSocketTTSPayload,
} from './interviewTypes'

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
  const [stageDirective, setStageDirective] = useState<InterviewQuestion['live2d_directive'] | null>(null)
  const [streamMouthOpen, setStreamMouthOpen] = useState(0)
  const [recognitionPartial, setRecognitionPartial] = useState('')
  const [recognitionFinal, setRecognitionFinal] = useState('')
  const [codingLanguage, setCodingLanguage] = useState('go')
  const [codeContent, setCodeContent] = useState('')
  const [codingRunMessage, setCodingRunMessage] = useState('编程题模式下可以先运行记录过程，再提交最终代码。')
  const [codingEventCount, setCodingEventCount] = useState(0)
  const [wsTraceId, setWsTraceId] = useState('')
  const [wsConnected, setWsConnected] = useState(false)
  const [isRecording, setIsRecording] = useState(false)
  const [hasRequestedMicrophonePermission, setHasRequestedMicrophonePermission] = useState(false)
  const [hasGrantedMicrophonePermission, setHasGrantedMicrophonePermission] = useState(false)
  const [assistantTurnCount, setAssistantTurnCount] = useState(0)
  const [selectedLive2DModelKey, setSelectedLive2DModelKey] = useState(() => readSelectedLive2DModelKey('interview', readSelectedFrontendIndustryCode() || INTERVIEW_DEFAULT_INDUSTRY_CODE))
  const {
    liveDialogue,
    isDialogueTyping,
    mouthOpen,
    stopDialogueTyping,
    startDialogueTyping,
    stopCurrentPlayback,
    syncDialogueImmediately,
    playTTSAudio,
  } = useLive2DDialoguePlayback({
    initialDialogue: '连接成功后，AI 面试官会在这里播报当前题目。',
    onPlaybackFinished: () => {
      setSessionState((current) => (current.status === 'speaking' ? { status: 'ready', message: '语音播报完成，可继续作答。' } : current))
    },
    onPlaybackError: (error) => {
      setMessage(extractErrorMessage(error, '自动播放语音失败，已回退到文本模式。'))
    },
  })
  const wsRef = useRef<WebSocket | null>(null)
  const assistantTranscriptRef = useRef('')
  const recordStreamRef = useRef<MediaStream | null>(null)
  const recordAudioContextRef = useRef<AudioContext | null>(null)
  const recordSourceRef = useRef<MediaStreamAudioSourceNode | null>(null)
  const recordProcessorRef = useRef<ScriptProcessorNode | null>(null)
  const recordSilenceTimeoutRef = useRef<number | null>(null)
  const recordMaxDurationTimerRef = useRef<number | null>(null)
  const recordSpeechDetectedRef = useRef(false)
  const recordStopRequestedRef = useRef(false)
  const recordPendingPCMRef = useRef<number[]>([])
  const recordFrameQueueRef = useRef<string[]>([])
  const recordFrameTimerRef = useRef<number | null>(null)
  const recordFrameDrainTimerRef = useRef<number | null>(null)
  const editorContainerRef = useRef<HTMLDivElement | null>(null)
  const editorInstanceRef = useRef<any>(null)
  const monacoRef = useRef<any>(null)
  const codingEventsRef = useRef<InterviewCodingProcessEvent[]>([])
  const lastCodingSnapshotRef = useRef('')
  const codingSnapshotTimerRef = useRef<number | null>(null)
  const codingIdleTimerRef = useRef<number | null>(null)
  const { enqueuePCM16Base64, preparePlayback, stop: stopPCMStreamPlayback } = usePCMStreamPlayer({
    onLevelChange: setStreamMouthOpen,
  })

  const detailQuery = useQuery({
    queryKey: ['interview-detail', accessToken, interviewId],
    queryFn: () => fetchInterviewDetail(accessToken as string, interviewId),
    enabled: Boolean(accessToken && interviewId),
    retry: false,
    refetchOnWindowFocus: false,
  })
  const submitMutation = useMutation({
    mutationFn: (payload: SubmitInterviewAnswerPayload) => submitInterviewAnswer(accessToken as string, interviewId, payload),
    onSuccess: async (data) => {
      setAnswer('')
      setRecognitionFinal('')
      setRecognitionPartial('')
      setCodingRunMessage('本题提交完成，正在等待下一题或生成报告。')
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
  const isCodingQuestion = currentQuestion?.type === 'coding'
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
    setSelectedLive2DModelKey(readSelectedLive2DModelKey('interview', detailQuery.data.industry_code))
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
      assistantTranscriptRef.current = restoredQuestion.question
      setStageDirective(restoredQuestion.live2d_directive || null)
      setStageEmotion(restoredQuestion.live2d_directive?.emotion || 'neutral')
      stopDialogueTyping(restoredQuestion.question)
    }
  }, [detailQuery.data])

  /**
   * 面试页挂载后尽快预热 Live2D 运行时，减少首次渲染面试官舞台时的额外等待。
   */
  useEffect(() => {
    if (typeof window === 'undefined') {
      return undefined
    }

    if ('requestIdleCallback' in window) {
      const idleId = window.requestIdleCallback(() => {
        prewarmLive2DRuntime()
      }, { timeout: 1200 })

      return () => {
        window.cancelIdleCallback(idleId)
      }
    }

    const timer = window.setTimeout(() => {
      prewarmLive2DRuntime()
    }, 600)

    return () => {
      window.clearTimeout(timer)
    }
  }, [sessionIndustryCode])

  /**
   * 向当前编程题过程缓存中追加一条事件，并同步刷新界面上的事件数量。
   */
  function appendCodingProcessEvent(type: string, payload?: Record<string, unknown>): void {
    if (!isCodingQuestion) {
      return
    }

    codingEventsRef.current = [
      ...codingEventsRef.current,
      {
        type,
        timestamp_ms: Date.now(),
        payload,
      },
    ]
    setCodingEventCount(codingEventsRef.current.length)
  }

  /**
   * 清理代码快照和长停顿检测定时器，避免旧题目状态影响当前题目。
   */
  function clearCodingTimers(): void {
    if (codingSnapshotTimerRef.current !== null) {
      window.clearTimeout(codingSnapshotTimerRef.current)
      codingSnapshotTimerRef.current = null
    }
    if (codingIdleTimerRef.current !== null) {
      window.clearTimeout(codingIdleTimerRef.current)
      codingIdleTimerRef.current = null
    }
  }

  /**
   * 以当前题目元数据重置编程题编辑状态，保证切题后不会残留上一题过程数据。
   */
  function resetCodingWorkspace(question: InterviewQuestion | null): void {
    const nextLanguage = question?.language || 'go'
    const nextCode = question?.starter_code || ''
    codingEventsRef.current = []
    lastCodingSnapshotRef.current = nextCode.trim()
    setCodingEventCount(0)
    setCodingLanguage(nextLanguage)
    setCodeContent(nextCode)
    setCodingRunMessage(
      question
        ? '编程题模式已准备好，可以先运行记录过程，再提交最终代码。'
        : '当前不是编程题。',
    )

    if (editorInstanceRef.current && editorInstanceRef.current.getValue() !== nextCode) {
      editorInstanceRef.current.setValue(nextCode)
    }
  }

  /**
   * 在进入或切换编程题时同步编辑器语言与默认模板。
   */
  useEffect(() => {
    clearCodingTimers()
    if (!isCodingQuestion) {
      resetCodingWorkspace(null)
      return
    }

    resetCodingWorkspace(currentQuestion)
  }, [currentQuestion?.question, currentQuestion?.language, currentQuestion?.starter_code, isCodingQuestion])

  /**
   * 初始化编程题编辑器实例，并把内容变化同步回页面状态。
   */
  useEffect(() => {
    if (!isCodingQuestion || !editorContainerRef.current) {
      if (editorInstanceRef.current) {
        editorInstanceRef.current.dispose()
        editorInstanceRef.current = null
      }
      return undefined
    }

    let disposed = false

    async function initializeEditor(): Promise<void> {
      if (!editorContainerRef.current || editorInstanceRef.current) {
        return
      }

      const loader = await import('@monaco-editor/loader')
      const monaco = await loader.default.init()
      if (disposed || !editorContainerRef.current) {
        return
      }

      monacoRef.current = monaco
      editorInstanceRef.current = monaco.editor.create(editorContainerRef.current, {
        value: codeContent || currentQuestion?.starter_code || '',
        language: codingLanguage,
        theme: 'vs-dark',
        fontSize: 14,
        minimap: { enabled: false },
        automaticLayout: true,
        scrollBeyondLastLine: false,
        lineNumbers: 'on',
        tabSize: 2,
      })
      editorInstanceRef.current.onDidChangeModelContent(() => {
        setCodeContent(editorInstanceRef.current.getValue())
      })
    }

    void initializeEditor()

    return () => {
      disposed = true
      if (editorInstanceRef.current) {
        editorInstanceRef.current.dispose()
        editorInstanceRef.current = null
      }
    }
  }, [currentQuestion?.question, codingLanguage, codeContent, isCodingQuestion])

  /**
   * 切换编程题语言高亮时同步更新 Monaco 模型语言。
   */
  useEffect(() => {
    if (!isCodingQuestion || !editorInstanceRef.current?.getModel() || !monacoRef.current) {
      return
    }

    monacoRef.current.editor.setModelLanguage(editorInstanceRef.current.getModel(), codingLanguage)
  }, [codingLanguage, isCodingQuestion])

  /**
   * 以节流方式写入代码快照事件，避免每次按键都形成冗余过程记录。
   */
  useEffect(() => {
    if (!isCodingQuestion) {
      return
    }

    if (codingSnapshotTimerRef.current !== null) {
      window.clearTimeout(codingSnapshotTimerRef.current)
    }
    codingSnapshotTimerRef.current = window.setTimeout(() => {
      const normalizedCode = codeContent.trim()
      if (!normalizedCode || normalizedCode === lastCodingSnapshotRef.current) {
        return
      }

      appendCodingProcessEvent('code_snapshot', {
        code: codeContent,
        line_count: codeContent.split('\n').length,
        language: codingLanguage,
      })
      lastCodingSnapshotRef.current = normalizedCode
    }, 900)

    return () => {
      if (codingSnapshotTimerRef.current !== null) {
        window.clearTimeout(codingSnapshotTimerRef.current)
        codingSnapshotTimerRef.current = null
      }
    }
  }, [codeContent, codingLanguage, isCodingQuestion])

  /**
   * 记录编程题长停顿事件，用于后端后续分析候选人在关键节点的卡顿情况。
   */
  useEffect(() => {
    if (!isCodingQuestion || detailQuery.data?.status !== 'ongoing') {
      return
    }

    if (codingIdleTimerRef.current !== null) {
      window.clearTimeout(codingIdleTimerRef.current)
    }
    codingIdleTimerRef.current = window.setTimeout(() => {
      appendCodingProcessEvent('idle_timeout', {
        idle_ms: 30000,
        language: codingLanguage,
      })
      setCodingRunMessage('已记录一次较长停顿，继续编码或运行后再提交。')
    }, 30000)

    return () => {
      if (codingIdleTimerRef.current !== null) {
        window.clearTimeout(codingIdleTimerRef.current)
        codingIdleTimerRef.current = null
      }
    }
  }, [codeContent, codingLanguage, detailQuery.data?.status, isCodingQuestion])

  /**
   * 记录一次本地“运行代码”动作；当前版本未接真实判题器，只采集过程数据并给出占位反馈。
   */
  function handleRunCodingQuestion(): void {
    if (!isCodingQuestion) {
      return
    }

    if (!codeContent.trim()) {
      setCodingRunMessage('请先输入代码，再记录运行过程。')
      return
    }

    appendCodingProcessEvent('run_code', {
      language: codingLanguage,
      code: codeContent,
    })
    appendCodingProcessEvent('run_result', {
      has_error: false,
      status: 'recorded_only',
      stdout: '当前版本尚未接真实判题器，本次运行仅用于记录过程数据。',
    })
    setCodingRunMessage('已记录一次运行过程；首版暂不执行真实代码。')
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
   * 按 20ms 一帧的节奏发送排队中的 PCM 数据，尽量贴近火山实时语音文档建议的麦克风上行节奏。
   */
  function ensureQueuedAudioFramesSending(): void {
    if (recordFrameTimerRef.current !== null) {
      return
    }

    recordFrameTimerRef.current = window.setInterval(() => {
      if (!wsRef.current || wsRef.current.readyState !== WebSocket.OPEN) {
        return
      }
      const nextFrame = recordFrameQueueRef.current.shift()
      if (!nextFrame) {
        return
      }

      wsRef.current.send(
        JSON.stringify({
          type: 'audio_chunk',
          data: {
            audio_base64: nextFrame,
          },
        }),
      )
    }, 20)
  }

  /**
   * 停止音频发送定时器，避免录音结束后仍然持有旧的发送循环。
   */
  function stopQueuedAudioFramesSending(): void {
    if (recordFrameTimerRef.current !== null) {
      window.clearInterval(recordFrameTimerRef.current)
      recordFrameTimerRef.current = null
    }
  }

  /**
   * 结束录音时等待排队中的音频帧按既定节奏发完，再补发 audio_end，避免瞬时突发整段语音。
   */
  function finishQueuedAudioFrames(reason: 'manual' | 'auto'): void {
    const socket = wsRef.current
    if (!socket || socket.readyState !== WebSocket.OPEN) {
      recordFrameQueueRef.current = []
      stopQueuedAudioFramesSending()
      if (recordFrameDrainTimerRef.current !== null) {
        window.clearInterval(recordFrameDrainTimerRef.current)
        recordFrameDrainTimerRef.current = null
      }
      return
    }

    const sendAudioEnd = () => {
      if (recordFrameDrainTimerRef.current !== null) {
        window.clearInterval(recordFrameDrainTimerRef.current)
        recordFrameDrainTimerRef.current = null
      }
      stopQueuedAudioFramesSending()
      socket.send(
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

    if (recordFrameQueueRef.current.length === 0) {
      sendAudioEnd()
      return
    }

    if (recordFrameDrainTimerRef.current !== null) {
      window.clearInterval(recordFrameDrainTimerRef.current)
    }
    recordFrameDrainTimerRef.current = window.setInterval(() => {
      const activeSocket = wsRef.current
      if (!activeSocket || activeSocket.readyState !== WebSocket.OPEN) {
        recordFrameQueueRef.current = []
        stopQueuedAudioFramesSending()
        if (recordFrameDrainTimerRef.current !== null) {
          window.clearInterval(recordFrameDrainTimerRef.current)
          recordFrameDrainTimerRef.current = null
        }
        return
      }
      if (recordFrameQueueRef.current.length > 0) {
        return
      }

      sendAudioEnd()
    }, 20)
  }

  /**
   * 预先解锁浏览器音频播放上下文，避免实时 PCM 首包到达时才触发自动播放限制。
   */
  async function ensureRealtimeAudioPlaybackReady(): Promise<void> {
    try {
      await preparePlayback()
    } catch (error) {
      setMessage(extractErrorMessage(error, '浏览器阻止了自动播放，请点击“开始语音回答”后重试。'))
    }
  }

  /**
   * 提前向浏览器申请麦克风权限，避免候选人真正开始回答时才弹授权框打断节奏。
   */
  async function ensureMicrophonePermission(): Promise<boolean> {
    if (!canRecord) {
      return false
    }
    if (hasGrantedMicrophonePermission) {
      return true
    }

    setHasRequestedMicrophonePermission(true)
    setMessage('正在请求麦克风授权，请留意浏览器权限提示。')
    // 不在此处 await AudioContext.resume()——浏览器自动播放策略会在无用户交互时
    // 导致 resume() 的 Promise 永远 pending，从而阻塞后续的 getUserMedia 调用。
    // 音频播放上下文会在用户真正开始录音（startVoiceCapture）时由用户点击手势解锁。
    void ensureRealtimeAudioPlaybackReady()
    try {
      const stream = await navigator.mediaDevices.getUserMedia({
        audio: true,
      })
      stream.getTracks().forEach((track) => track.stop())
      setHasGrantedMicrophonePermission(true)
      setMessage('麦克风权限已就绪，面试官播报结束后会自动开始收音。')
      return true
    } catch (error) {
      setMessage(extractErrorMessage(error, '麦克风权限请求失败，请点击“开始语音回答”并允许浏览器访问麦克风。'))
      return false
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

    await ensureRealtimeAudioPlaybackReady()
    try {
      const stream = await navigator.mediaDevices.getUserMedia({
        audio: true,
      })
      const recordContext = new AudioContext({
        sampleRate: 16000,
      })
      await recordContext.resume()
      if (recordContext.state !== 'running') {
        throw new Error('浏览器未真正启动录音上下文，请点击“开始语音回答”后重试。')
      }
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
      recordPendingPCMRef.current = []
      recordFrameQueueRef.current = []
      if (recordFrameDrainTimerRef.current !== null) {
        window.clearInterval(recordFrameDrainTimerRef.current)
        recordFrameDrainTimerRef.current = null
      }
      clearRecordSilenceTimer()
      ensureQueuedAudioFramesSending()
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
        const pcmChunk = resampleFloat32ToPCM16(channelData, event.inputBuffer.sampleRate, 16000)
        if (!pcmChunk.length) {
          return
        }

        recordPendingPCMRef.current.push(...pcmChunk)
        while (recordPendingPCMRef.current.length >= 320) {
          const frame = new Int16Array(recordPendingPCMRef.current.slice(0, 320))
          recordPendingPCMRef.current = recordPendingPCMRef.current.slice(320)
          const audioBase64 = encodePCM16Base64FromInt16(frame)
          if (!audioBase64) {
            continue
          }
          recordFrameQueueRef.current.push(audioBase64)
        }
      }

      recordStreamRef.current = stream
      recordAudioContextRef.current = recordContext
      recordSourceRef.current = source
      recordProcessorRef.current = processor
      setHasGrantedMicrophonePermission(true)
      setRecognitionPartial('')
      setRecognitionFinal('')
      setIsRecording(true)
      setMessage('正在实时识别你的回答，请继续说；停顿后会自动提交。')
      if (recordMaxDurationTimerRef.current !== null) {
        window.clearTimeout(recordMaxDurationTimerRef.current)
      }
      recordMaxDurationTimerRef.current = window.setTimeout(() => {
        recordMaxDurationTimerRef.current = null
        setMessage('已达到单轮最大录音时长，正在自动提交。')
        stopVoiceCapture('auto')
      }, INTERVIEW_MAX_RECORDING_MS)
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
    if (recordMaxDurationTimerRef.current !== null) {
      window.clearTimeout(recordMaxDurationTimerRef.current)
      recordMaxDurationTimerRef.current = null
    }
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
    if (reason !== 'cleanup' && recordPendingPCMRef.current.length > 0) {
      const finalFrame = new Int16Array(recordPendingPCMRef.current)
      const audioBase64 = encodePCM16Base64FromInt16(finalFrame)
      if (audioBase64) {
        recordFrameQueueRef.current.push(audioBase64)
      }
    }
    recordPendingPCMRef.current = []
    if (reason === 'cleanup') {
      recordFrameQueueRef.current = []
      stopQueuedAudioFramesSending()
      if (recordFrameDrainTimerRef.current !== null) {
        window.clearInterval(recordFrameDrainTimerRef.current)
        recordFrameDrainTimerRef.current = null
      }
    } else {
      finishQueuedAudioFrames(reason)
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
      setMessage('实时面试链路已连接，正在确认当前会话模式。')
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
              appendInterviewMessage(current, buildRealtimeInterviewMessage('ai', 'text', questionText, questionPayload ? {
                question: questionText,
                topic: '',
                difficulty: '',
                type: questionPayload.type || 'technical',
                hints: questionPayload.hints,
                language: questionPayload.language,
                starter_code: questionPayload.starter_code,
                editor_mode: questionPayload.editor_mode,
                evaluation_mode: questionPayload.evaluation_mode,
                live2d_directive: questionPayload.live2d_directive || null,
              } : null)),
            )
            setStageDirective(questionPayload?.live2d_directive || null)
            setStageEmotion(questionPayload?.live2d_directive?.emotion || 'neutral')
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
          case 'assistant_transcript_partial': {
            const transcriptPayload = payload.data as InterviewSocketAssistantTranscriptPayload | undefined
            const nextText = transcriptPayload?.text || payload.content || ''
            if (nextText) {
              assistantTranscriptRef.current = nextText
              syncDialogueImmediately(nextText)
            }
            break
          }
          case 'assistant_transcript_final': {
            const transcriptPayload = payload.data as InterviewSocketAssistantTranscriptPayload | undefined
            const nextText = transcriptPayload?.text || payload.content || ''
            if (nextText) {
              assistantTranscriptRef.current = nextText
              syncDialogueImmediately(nextText)
            }
            break
          }
          case 'assistant_audio_chunk': {
            const audioChunkPayload = payload.data as InterviewSocketAssistantAudioChunkPayload | undefined
            if (audioChunkPayload?.audio_base64) {
              void enqueuePCM16Base64(audioChunkPayload.audio_base64, audioChunkPayload.sample_rate).catch((error) => {
                setMessage(extractErrorMessage(error, '浏览器阻止了实时语音播放，请点击“开始语音回答”后重试。'))
              })
            }
            break
          }
          case 'assistant_turn_finished': {
            const turnPayload = payload.data as InterviewSocketAssistantTurnPayload | undefined
            const finalText = turnPayload?.text || payload.content || ''
            if (finalText) {
              assistantTranscriptRef.current = finalText
              syncDialogueImmediately(finalText)
              setRuntimeMessages((current) =>
                appendInterviewMessage(
                  current,
                  buildRealtimeInterviewMessage(
                    'ai',
                    'text',
                    finalText,
                    turnPayload?.is_question
                      ? {
                          question: finalText,
                          topic: '',
                          difficulty: '',
                          type: 'technical',
                          live2d_directive: turnPayload?.live2d_directive || null,
                        }
                      : null,
                  ),
                ),
              )
            }
            setStageDirective(turnPayload?.live2d_directive || null)
            setStageEmotion(turnPayload?.live2d_directive?.emotion || 'neutral')
            setAssistantTurnCount((current) => current + 1)
            break
          }
          case 'barge_in': {
            stopCurrentPlayback(assistantTranscriptRef.current)
            stopPCMStreamPlayback()
            setStreamMouthOpen(0)
            break
          }
          case 'live2d_expression': {
            const expressionPayload = payload.data as InterviewSocketExpressionPayload | undefined
            setStageEmotion(expressionPayload?.emotion || 'neutral')
            setStageDirective(expressionPayload ? {
              emotion: expressionPayload.emotion,
              action: expressionPayload.action,
              source: expressionPayload.source,
              expression_mix: expressionPayload.expression_mix,
              parameter_overrides: expressionPayload.parameter_overrides,
              intensity: expressionPayload.intensity,
              duration_ms: expressionPayload.duration_ms,
              mouth_open: expressionPayload.mouth_open,
            } : null)
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
            setSessionState({
              status: 'error',
              message: payload.content || '实时面试链路发生错误。',
            })
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
      stopPCMStreamPlayback()
      setStreamMouthOpen(0)
      setSessionState((current) => (current.status === 'error'
        ? current
        : { status: 'idle', message: '实时链路已断开，当前只能使用 HTTP 文本回退模式。' }))
      setMessage('实时面试链路已断开，当前会退回 HTTP 模式，此模式不会触发语音播报。')
    }

    socket.onerror = () => {
      stopPCMStreamPlayback()
      setStreamMouthOpen(0)
      setSessionState({
        status: 'error',
        message: '实时面试链路连接异常，请检查后端实时语音配置。',
      })
      setMessage('实时面试链路连接异常，当前会退回 HTTP 模式，此模式不会触发语音播报。')
    }

    return () => {
      socket.close()
      wsRef.current = null
    }
  }, [accessToken, interviewId])

  /**
   * 实时语音面试进入页面后先抢占一次麦克风权限，避免真正开始回答时才被浏览器权限弹窗打断。
   */
  useEffect(() => {
    if (!wsConnected || sessionState.mode !== 'realtime' || !canRecord || hasRequestedMicrophonePermission) {
      return
    }

    void ensureMicrophonePermission()
  }, [canRecord, hasRequestedMicrophonePermission, sessionState.mode, wsConnected])

  /**
   * 当实时面试官完成一轮播报并进入 ready 状态后，自动开始收音，让候选人可以直接开口回答。
   */
  useEffect(() => {
    if (sessionState.mode !== 'realtime' || sessionState.status !== 'ready') {
      return
    }
    if (!wsConnected || !canRecord || isRecording || isCodingQuestion) {
      return
    }
    if (!hasGrantedMicrophonePermission || assistantTurnCount <= 0) {
      return
    }
    if (answer.trim() || recognitionPartial.trim()) {
      return
    }

    const timer = window.setTimeout(() => {
      void startVoiceCapture()
    }, 260)

    return () => {
      window.clearTimeout(timer)
    }
  }, [
    answer,
    assistantTurnCount,
    canRecord,
    hasGrantedMicrophonePermission,
    isCodingQuestion,
    isRecording,
    recognitionPartial,
    sessionState.mode,
    sessionState.status,
    wsConnected,
  ])

  /**
   * 在页面卸载时释放音频播放和录音相关资源，避免浏览器残留占用。
   */
  useEffect(() => {
    return () => {
      clearCodingTimers()
      stopCurrentPlayback()
      stopPCMStreamPlayback()
      stopQueuedAudioFramesSending()
      if (recordFrameDrainTimerRef.current !== null) {
        window.clearInterval(recordFrameDrainTimerRef.current)
        recordFrameDrainTimerRef.current = null
      }
      stopVoiceCapture('cleanup')
    }
  }, [])

  /**
   * 提交当前问题的答案，优先走 WebSocket 实时链路，断开时回退到 HTTP。
   */
  async function handleSubmitAnswer(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()

    const content = answer.trim()
    const finalCode = codeContent.trim()
    if (isCodingQuestion) {
      if (!finalCode) {
        setMessage('编程题需要先填写最终代码。')
        return
      }

      const processEvents = [
        ...codingEventsRef.current,
        {
          type: 'submit_code',
          timestamp_ms: Date.now(),
          payload: {
            language: codingLanguage,
            code: codeContent,
            note: content,
          },
        },
      ]

      setMessage('编程题将通过 HTTP 提交完整过程数据并生成后续题目。')
      await submitMutation.mutateAsync({
        answer: content,
        final_code: codeContent,
        language: codingLanguage,
        question_type: currentQuestion?.type,
        process_events: processEvents,
      })
      return
    }

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
    await submitMutation.mutateAsync({
      answer: content,
    })
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
            mouthOpen={Math.max(mouthOpen, streamMouthOpen)}
            directive={stageDirective || currentQuestion?.live2d_directive || null}
            selectedModelKey={selectedLive2DModelKey}
            onChangeModelKey={setSelectedLive2DModelKey}
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
                {isRecording ? '麦克风采集中，停顿后自动提交...' : (sessionState.status === 'error'
                  ? '实时链路错误'
                  : (wsConnected ? '实时链路优先' : '当前为 HTTP 回退模式'))}
              </span>
            </div>

            <form className="companion-composer" onSubmit={handleSubmitAnswer}>
              <div className="interview-answer-status-stack">
                <p className="interview-answer-status-primary">{sessionState.message || message}</p>
                {recognitionPartial ? <p className="interview-answer-status-secondary">实时识别：{recognitionPartial}</p> : null}
                {recognitionFinal ? <p className="interview-answer-status-secondary">识别结果：{recognitionFinal}</p> : null}
              </div>
              {isCodingQuestion ? (
                <div className="timeline-item">
                  <strong>编程题工作区</strong>
                  <p>
                    当前语言：{codingLanguage}
                    {currentQuestion?.hints ? ` · 提示：${currentQuestion.hints}` : ''}
                  </p>
                  <div
                    ref={editorContainerRef}
                    style={{
                      minHeight: 320,
                      width: '100%',
                      borderRadius: 16,
                      overflow: 'hidden',
                      border: '1px solid rgba(255,255,255,0.08)',
                    }}
                  />
                  <div className="page-actions" style={{ marginTop: 12 }}>
                    <button className="secondary-button" type="button" onClick={handleRunCodingQuestion}>
                      运行并记录过程
                    </button>
                    <label className="field" style={{ minWidth: 160 }}>
                      <span>代码语言</span>
                      <select value={codingLanguage} onChange={(event) => setCodingLanguage(event.target.value)}>
                        <option value="go">Go</option>
                        <option value="java">Java</option>
                        <option value="javascript">JavaScript</option>
                        <option value="python">Python</option>
                      </select>
                    </label>
                  </div>
                  <p className="interview-answer-status-secondary">{codingRunMessage}</p>
                  <p className="interview-answer-status-secondary">当前已记录 {codingEventCount} 条过程事件。</p>
                </div>
              ) : null}
              <textarea
                rows={7}
                value={answer}
                onChange={(event) => setAnswer(event.target.value)}
                placeholder={isCodingQuestion ? '可选：补充你的解题思路、复杂度权衡或边界条件说明。' : '直接输入你的回答。建议先说思路，再补关键点、权衡和落地细节。'}
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
                    disabled={!canRecord || isCodingQuestion}
                    onClick={() => {
                      if (isRecording) {
                        stopVoiceCapture()
                        return
                      }
                      void startVoiceCapture()
                    }}
                  >
                    {isRecording ? '手动停止并提交' : (hasGrantedMicrophonePermission ? '开始语音回答' : '授权麦克风并开始语音回答')}
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
                    {submitMutation.isPending ? '提交中...' : (isCodingQuestion ? '提交代码与过程数据' : '手动提交答案')}
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

        {detailQuery.isLoading ? <AsyncInlineState className="companion-empty-text" message="正在恢复面试记录..." /> : null}
        {detailQuery.isError ? (
          <AsyncInlineState
            className="companion-empty-text"
            message={detailQuery.error instanceof Error ? detailQuery.error.message : '面试记录加载失败'}
            tone="error"
          />
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
