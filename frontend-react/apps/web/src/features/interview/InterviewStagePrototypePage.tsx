/**
 * 面试舞台页（默认舞台，/interview/$interviewId）
 * 深色沉浸式布局：全屏 Live2D 面试官舞台 + 右上浮层（面试记录 / 模型切换，按需唤起）+ 底部作答栏。
 * 支持两类语音面试链路：
 *  - 标准语音（非实时）：题目 TTS 播报字幕、语音/文字作答、自动推进下一题、结束面试生成报告。
 *  - 实时语音（WebSocket）：流式麦克风上行、PCM 流式播放、自动开录、turn 管理，付费会员实时面试直接在本页完成。
 * 编程题（Monaco 工作区）仍由旧版舞台 /interview/$interviewId/legacy 承载，命中时以 notice 卡片引导跳转 legacy。
 */
import { useState, useRef, useEffect, useMemo, Component, type ReactNode } from 'react'
import { Link, useNavigate, useParams } from '@tanstack/react-router'
import { Spin } from 'antd'
import {
  ArrowLeftOutlined,
  DownOutlined,
  HistoryOutlined,
  SendOutlined,
  SettingOutlined,
} from '@ant-design/icons'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { extractErrorMessage } from '@makejob/api-client'
import { useAuthStore } from '../../state/auth'
import { loadLive2DRuntime, prewarmLive2DRuntime } from '../../shared/live2dRuntime'
import {
  fetchSelectableLive2DModels,
  persistSelectedLive2DModelKey,
  readSelectedLive2DModelKey,
  resolveSelectableLive2DBackgroundImageUrl,
  type SelectableLive2DModelMotion,
} from '../../shared/live2dModelCatalog'
import { useLive2DDialoguePlayback } from '../../shared/useLive2DDialoguePlayback'
import { usePCMStreamPlayer } from '../../shared/usePCMStreamPlayer'
import type { Live2DDirective } from '../../shared/live2dDirective'
import {
  DEFAULT_FRONTEND_INDUSTRY_CODE as INTERVIEW_DEFAULT_INDUSTRY_CODE,
  persistSelectedFrontendIndustryCode,
  readSelectedFrontendIndustryCode,
} from '../../shared/industryContext'
import { requestLoginPrompt } from '../../shared/loginPrompt'
import { recognizeSpeech } from '../companion/companionApi'
import {
  fetchInterviewDetail,
  fetchNextInterviewQuestion,
  finishInterviewRequest,
  submitInterviewAnswer,
  synthesizeInterviewSpeech,
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

/* ========== 公共兜底模型（与陪伴房间原型一致） ========== */
const DEMO_MODEL_URL = 'https://cdn.jsdelivr.net/gh/guansss/pixi-live2d-display@v0.4.0/test/assets/haru/haru_greeter_t03.model3.json'

/* ========== 颜色令牌 — 与陪伴房间（PrototypeUIPage）保持一致 ========== */

const C = {
  gray900: '#1a202c',
  gray800: '#2d3748',
  gray700: '#4a5568',
  gray600: '#718096',
  gray500: '#a0aec0',
  white: 'rgba(255,255,255,0.92)',
  white700: 'rgba(255,255,255,0.72)',
  white500: 'rgba(255,255,255,0.50)',
  white200: 'rgba(255,255,255,0.20)',
  white100: 'rgba(255,255,255,0.10)',
  white050: 'rgba(255,255,255,0.05)',
  blue500: '#4299e1',
  green500: '#48bb78',
  red500: '#f56565',
  yellow500: '#ecc94b',
  orange500: '#f97316',
}

/* ========== 嘴型同步 — 由 TTS 振幅驱动 Live2D 嘴型参数 ========== */

const MOUTH_PARAM_CANDIDATES = ['ParamMouthOpen', 'ParamMouthOpenY']

/**
 * 把当前嘴型开合值写入模型参数，兼容 Cubism3/4 与旧版参数名。
 */
function applyLive2DMouthOpen(model: any, value: number): void {
  const coreModel = model?.internalModel?.coreModel
  if (!coreModel?.setParameterValueById) {
    return
  }
  for (const parameterId of MOUTH_PARAM_CANDIDATES) {
    try {
      coreModel.setParameterValueById(parameterId, value)
    } catch {
      // 部分模型不持有该参数，跳过即可。
    }
  }
}

/* ========== Live2D 画布 — 复用陪伴房间的直连运行时实现（拖拽/缩放/视线跟随） ========== */

/**
 * 自包含的 Live2D 画布：直接操作 pixi 运行时渲染面试官模型，
 * 支持拖拽移动、滚轮缩放、鼠标视线跟随、嘴型同步与后端动作指令播放。
 */
function StageLive2DCanvas({ modelUrl, backgroundImageUrl, mouthOpen, motions, directive }: { modelUrl: string; backgroundImageUrl?: string; mouthOpen: number; motions: SelectableLive2DModelMotion[]; directive: Live2DDirective | null }) {
  const hostRef = useRef<HTMLDivElement>(null)
  const runtimeRef = useRef<any>(null)
  const [status, setStatus] = useState('等待加载...')
  const [error, setError] = useState('')
  const [bgFailed, setBgFailed] = useState(false)
  const dragRef = useRef<{ pointerId: number; startX: number; startY: number; originX: number; originY: number } | null>(null)
  const [isDragging, setIsDragging] = useState(false)

  // 滚轮缩放状态：模型加载后以自然贴合比例为基准做乘法缩放
  const scaleRef = useRef(1)
  const targetScaleRef = useRef(1)
  const animFrameRef = useRef<number>(0)
  const EASING = 0.15
  const MIN_SCALE = 0.3
  const MAX_SCALE = 3.0
  const WHEEL_STEP = 0.02

  // 嘴型开合由父组件传入，通过 ref 每帧写入模型参数，避免 React 重渲染参与逐帧更新。
  const mouthOpenRef = useRef(0)
  useEffect(() => {
    mouthOpenRef.current = mouthOpen
  }, [mouthOpen])

  // 记录最近一次播放的动作，用于节流，避免同 key 短时间内重复触发。
  const lastMotionRef = useRef<{ key: string; at: number } | null>(null)

  /**
   * 当后端指令带来新的 motion_key 时，在模型动作清单里查到对应 group 并播放一次。
   */
  useEffect(() => {
    const runtime = runtimeRef.current
    const motionKey = directive?.motion_key
    if (!runtime || !motionKey) {
      return
    }

    const now = Date.now()
    if (lastMotionRef.current?.key === motionKey && now - lastMotionRef.current.at < 900) {
      return
    }

    const motionDef = motions.find((item) => item.key === motionKey)
    if (!motionDef) {
      return
    }

    const group = motionDef.group || 'auto'
    const priority = directive?.motion_priority === 'force'
      ? runtime.motionPriority.FORCE
      : runtime.motionPriority.NORMAL

    let cancelled = false
    void (async () => {
      try {
        const started = await runtime.model.motion(group, 0, priority)
        if (!cancelled && started) {
          lastMotionRef.current = { key: motionKey, at: now }
        }
      } catch {
        // 模型不持有该动作时静默跳过。
      }
    })()

    return () => {
      cancelled = true
    }
  }, [directive?.motion_key, motions])

  useEffect(() => {
    const host = hostRef.current
    if (!host || !modelUrl) return

    let disposed = false
    let resizeObserver: ResizeObserver | null = null

    /**
     * 挂载 pixi 应用并加载 Live2D 模型，失败时回退到公共演示模型。
     */
    async function mount() {
      setStatus('正在加载 Live2D 运行时...')
      try {
        const { PIXI, Live2DModel, MotionPriority } = await loadLive2DRuntime()
        if (disposed) return

        setStatus('正在加载模型...')

        const app = new PIXI.Application({
          width: Math.max(host!.clientWidth, 320),
          height: Math.max(host!.clientHeight, 320),
          autoStart: true,
          backgroundAlpha: 0,
          antialias: true,
          autoDensity: true,
          resolution: Math.min(window.devicePixelRatio || 1, 2),
        })

        const canvas = app.view as HTMLCanvasElement
        canvas.style.width = '100%'
        canvas.style.height = '100%'
        canvas.style.display = 'block'
        host!.replaceChildren(canvas)

        // Live2D 模型实例来自动态运行时，无静态类型可用，按 any 处理。
        let model: any
        try {
          model = await Live2DModel.from(modelUrl)
        } catch (primaryError) {
          if (disposed) return
          console.warn('Primary model failed, trying demo:', primaryError)
          setStatus('主模型加载失败，尝试备用...')
          model = await Live2DModel.from(DEMO_MODEL_URL)
        }

        if (disposed) {
          app.destroy(true, { children: true, texture: false, baseTexture: false })
          return
        }

        app.stage.addChild(model)
        // 用缩放后的实际宽高做左上角定位，不依赖 anchor(0.5,1)：部分时机/模型下 anchor 未生效会让整体偏向一侧。
        model.anchor.set(0, 0)

        // 模型自然尺寸（scale=1 时读取一次，避免后续缩放后 getLocalBounds 抖动）。
        const naturalWidth = Math.max(model.width, 1)
        const naturalHeight = Math.max(model.height, 1)

        /**
         * 计算模型贴合舞台的自然缩放比例。
         */
        function getFitScale(): number {
          if (!host) return 0.1
          const w = host.clientWidth
          const h = host.clientHeight
          const widthScale = (w * 0.82) / naturalWidth
          const heightScale = (h * 1.04) / naturalHeight
          return Math.max(Math.min(widthScale, heightScale), 0.1)
        }

        const baseFit = getFitScale()

        /**
         * 把当前缩放比例与站位应用到模型。
         */
        function applyScale() {
          if (!host || disposed) return
          const w = host.clientWidth
          const h = host.clientHeight
          const finalScale = baseFit * scaleRef.current
          model.scale.set(finalScale)
          const scaledWidth = naturalWidth * finalScale
          const scaledHeight = naturalHeight * finalScale
          model.x = (w - scaledWidth) / 2
          model.y = h * 0.94 - scaledHeight
        }
        applyScale()

        resizeObserver = new ResizeObserver(() => {
          if (!host || disposed) return
          app.renderer.resize(host.clientWidth, host.clientHeight)
          applyScale()
        })
        resizeObserver.observe(host!)

        // 鼠标移动时视线跟随（拖拽中不生效）
        host!.addEventListener('mousemove', (e) => {
          if (disposed || dragRef.current) return
          const rect = host!.getBoundingClientRect()
          model.focus(e.clientX - rect.left, e.clientY - rect.top)
        })

        host!.addEventListener('wheel', (e) => {
          e.preventDefault()
          if (disposed) return
          const direction = e.deltaY > 0 ? -1 : 1
          targetScaleRef.current = Math.max(
            MIN_SCALE,
            Math.min(MAX_SCALE, targetScaleRef.current * (1 + WHEEL_STEP * direction))
          )
          if (!animFrameRef.current) {
            animFrameRef.current = requestAnimationFrame(animateScale)
          }
        }, { passive: false })

        /**
         * 平滑过渡到目标缩放比例。
         */
        function animateScale() {
          const diff = targetScaleRef.current - scaleRef.current
          if (Math.abs(diff) < 0.001) {
            scaleRef.current = targetScaleRef.current
            animFrameRef.current = 0
            applyScale()
            return
          }
          scaleRef.current += diff * EASING
          applyScale()
          animFrameRef.current = requestAnimationFrame(animateScale)
        }

        // 在 internalModel 应用完 motion/表情/自然动作之后、绘制烘焙之前写入最新嘴型开合，
        // 避免被动作曲线每帧覆盖。注意：pixi-live2d-display 只在 internalModel 上提供
        // beforeModelUpdate 等事件，模型层并不存在 afterUpdate 事件。
        model.internalModel?.on?.('beforeModelUpdate', () => {
          applyLive2DMouthOpen(model, mouthOpenRef.current)
        })

        runtimeRef.current = { app, model, motionPriority: MotionPriority }
        setStatus('模型已就绪')
      } catch (err) {
        if (disposed) return
        setError(err instanceof Error ? err.message : '模型加载失败')
        setStatus('加载失败')
      }
    }

    mount()

    return () => {
      disposed = true
      if (animFrameRef.current) {
        cancelAnimationFrame(animFrameRef.current)
        animFrameRef.current = 0
      }
      if (resizeObserver) resizeObserver.disconnect()
      if (runtimeRef.current) {
        runtimeRef.current.app.destroy(true, { children: true, texture: false, baseTexture: false })
        runtimeRef.current = null
      }
    }
  }, [modelUrl])

  /**
   * 按下指针开始拖拽模型站位。
   */
  function handlePointerDown(e: React.PointerEvent) {
    if (!runtimeRef.current) return
    const model = runtimeRef.current.model
    dragRef.current = {
      pointerId: e.pointerId,
      startX: e.clientX,
      startY: e.clientY,
      originX: model.x,
      originY: model.y,
    }
    setIsDragging(true)
    e.currentTarget.setPointerCapture(e.pointerId)
  }

  /**
   * 拖拽中按位移更新模型站位。
   */
  function handlePointerMove(e: React.PointerEvent) {
    if (!dragRef.current || !runtimeRef.current) return
    const model = runtimeRef.current.model
    model.x = dragRef.current.originX + (e.clientX - dragRef.current.startX)
    model.y = dragRef.current.originY + (e.clientY - dragRef.current.startY)
  }

  /**
   * 结束拖拽并复位拖拽状态。
   */
  function handlePointerUp() {
    dragRef.current = null
    setIsDragging(false)
  }

  return (
    <>
      {backgroundImageUrl && !bgFailed && (
        <img
          src={backgroundImageUrl}
          alt=""
          style={{
            position: 'absolute', top: 0, left: 0, width: '100%', height: '100%',
            objectFit: 'cover', zIndex: 0,
          }}
          onError={() => setBgFailed(true)}
        />
      )}
      <div
        ref={hostRef}
        style={{
          position: 'absolute', inset: 0, zIndex: 1,
          cursor: isDragging ? 'grabbing' : 'grab',
        }}
        onPointerDown={handlePointerDown}
        onPointerMove={handlePointerMove}
        onPointerUp={handlePointerUp}
        onPointerCancel={handlePointerUp}
      />
      {status === '模型已就绪' && !error && (
        <div style={{
          position: 'absolute', bottom: 80, left: '50%', transform: 'translateX(-50%)',
          zIndex: 5, padding: '6px 14px', borderRadius: 20,
          background: 'rgba(0,0,0,0.4)', backdropFilter: 'blur(8px)',
          fontSize: 12, color: C.white500, pointerEvents: 'none',
          opacity: 0.7,
        }}>
          拖拽移动 · 滚轮缩放 · 移动鼠标跟随视线
        </div>
      )}
      {error && (
        <div style={{
          position: 'absolute', inset: 0, zIndex: 2,
          display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', gap: 12,
          background: 'rgba(0,0,0,0.5)',
        }}>
          <p style={{ color: C.red500, fontSize: 14, margin: 0 }}>模型加载失败</p>
          <p style={{ color: C.white500, fontSize: 12, margin: 0 }}>{error}</p>
        </div>
      )}
      {status !== '模型已就绪' && !error && (
        <div style={{
          position: 'absolute', inset: 0, zIndex: 2,
          display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', gap: 12,
          background: 'rgba(0,0,0,0.3)',
        }}>
          <Spin size="large" />
          <p style={{ color: C.white500, fontSize: 14, margin: 0 }}>{status}</p>
        </div>
      )}
    </>
  )
}

/* ========== 错误边界 ========== */

class StageErrorBoundary extends Component<{ children: ReactNode }, { hasError: boolean; error: string }> {
  state = { hasError: false, error: '' }
  static getDerivedStateFromError(error: Error) {
    return { hasError: true, error: error.message }
  }
  render() {
    if (this.state.hasError) {
      return (
        <div style={{
          position: 'absolute', inset: 0,
          background: 'radial-gradient(ellipse at 50% 60%, #1e3a5f 0%, #0f1b2d 70%)',
          display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', gap: 12,
        }}>
          <p style={{ color: C.red500, fontSize: 14, margin: 0 }}>Live2D 加载出错</p>
          <p style={{ color: C.white500, fontSize: 12, margin: 0, maxWidth: 400, textAlign: 'center' }}>{this.state.error}</p>
          <button onClick={() => this.setState({ hasError: false, error: '' })} style={{
            padding: '6px 16px', borderRadius: 6, border: `1px solid ${C.white200}`,
            background: 'transparent', color: C.white700, cursor: 'pointer', fontSize: 12,
          }}>重试</button>
        </div>
      )
    }
    return this.props.children
  }
}

/* ========== 面试消息气泡 — 复用陪伴房间 ChatBubble 视觉 ========== */

/**
 * 渲染单条面试记录气泡，AI 面试官与候选人用不同头像色区分，反馈消息带角标。
 */
function InterviewChatBubble({ msg }: { msg: InterviewMessage }) {
  const isAI = msg.role === 'ai'
  const roleName = isAI ? (msg.message_type === 'feedback' ? 'AI 反馈' : 'AI 面试官') : '你'
  return (
    <div style={{
      display: 'flex', gap: 8, padding: '8px 12px', borderRadius: 6, cursor: 'default', transition: 'background 0.2s',
    }}
      onMouseEnter={(e) => { e.currentTarget.style.background = C.white050 }}
      onMouseLeave={(e) => { e.currentTarget.style.background = 'transparent' }}
    >
      <div style={{
        width: 28, height: 28, borderRadius: '50%', flexShrink: 0,
        background: isAI ? C.blue500 : C.green500,
        display: 'flex', alignItems: 'center', justifyContent: 'center',
        fontSize: 12, fontWeight: 700, color: '#fff',
      }}>
        {isAI ? 'AI' : 'Me'}
      </div>
      <div style={{ flex: 1, minWidth: 0 }}>
        <div style={{ fontSize: 14, fontWeight: 600, color: C.white700, marginBottom: 2 }}>{roleName}</div>
        <div style={{ fontSize: 14, color: C.white, lineHeight: 1.5, background: C.gray700, borderRadius: 8, padding: '8px 12px', maxWidth: '90%', whiteSpace: 'pre-wrap' }}>{msg.content}</div>
        <div style={{ fontSize: 11, color: C.white500, marginTop: 4 }}>{formatInterviewDateTime(msg.created_at)}</div>
      </div>
    </div>
  )
}

/**
 * 把面试状态编码转换成可读的中文标签（舞台徽标与浮层共用）。
 */
function formatInterviewStatusLabel(status: string): string {
  switch (status) {
    case 'preparing':
      return '题目准备中'
    case 'ongoing':
      return '进行中'
    case 'report_generating':
      return '报告生成中'
    case 'completed':
      return '已完成'
    default:
      return status || '未知'
  }
}

/* ========== 右上浮层 - 面试记录 / 模型切换（按需唤起，不常驻，保持沉浸） ========== */

type StageOverlayPanelKey = 'history' | 'model'

/**
 * 渲染右上角按需浮层：面试记录或模型切换。点遮罩或关闭按钮收起，不占据常驻布局。
 */
function StageOverlayPanel({ panel, onClose, messages, modelOptions, selectedModelKey, setSelectedModelKey, modelOptionsQuery }: {
  panel: StageOverlayPanelKey
  onClose: () => void
  messages: InterviewMessage[]
  modelOptions: any[]
  selectedModelKey: string
  setSelectedModelKey: (key: string) => void
  modelOptionsQuery: any
}) {
  const listRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (listRef.current) listRef.current.scrollTop = listRef.current.scrollHeight
  }, [messages.length])

  const title = panel === 'history' ? '面试记录' : '切换模型'

  return (
    <>
      {/* 遮罩：点击收起浮层 */}
      <div onClick={onClose} style={{ position: 'fixed', inset: 0, zIndex: 20 }} />
      <div style={{
        position: 'fixed', top: 64, right: 20, zIndex: 21, width: 360,
        maxHeight: '70vh', background: C.gray900, border: `1px solid ${C.white200}`,
        borderRadius: 12, overflow: 'hidden', display: 'flex', flexDirection: 'column',
        boxShadow: '0 20px 60px rgba(0,0,0,0.45)',
      }}>
        {/* 头部 */}
        <div style={{
          display: 'flex', alignItems: 'center', justifyContent: 'space-between',
          padding: '12px 16px', borderBottom: `1px solid ${C.white100}`,
        }}>
          <span style={{ fontSize: 15, fontWeight: 600, color: '#fff' }}>{title}</span>
          <button
            type="button"
            onClick={onClose}
            style={{ background: 'transparent', border: 'none', color: C.white500, cursor: 'pointer', fontSize: 16, lineHeight: 1, padding: 0 }}
          >
            ✕
          </button>
        </div>

        {/* 内容 */}
        <div ref={listRef} style={{
          flex: 1, overflowY: 'auto', padding: 16,
          display: 'flex', flexDirection: 'column', gap: 8,
        }}>
          {panel === 'history' ? (
            messages.length === 0 ? (
              <p style={{ fontSize: 13, color: C.gray500, textAlign: 'center', padding: 24 }}>面试开始后，题目和回答会记录在这里</p>
            ) : (
              messages.map((msg, index) => <InterviewChatBubble key={`${msg.role}-${msg.created_at}-${index}`} msg={msg} />)
            )
          ) : (
            <>
              <p style={{ fontSize: 13, color: C.gray500, margin: 0 }}>选择要在舞台展示的 AI 面试官模型</p>
              {modelOptions.length === 0 ? (
                <p style={{ fontSize: 13, color: C.gray500, textAlign: 'center', padding: 16 }}>
                  {modelOptionsQuery.isLoading ? '正在加载模型列表...' : '暂无可用模型'}
                </p>
              ) : (
                modelOptions.map((m) => (
                  <div
                    key={m.key}
                    onClick={() => setSelectedModelKey(m.key)}
                    style={{
                      padding: '14px 16px', borderRadius: 8,
                      border: `1px solid ${m.key === selectedModelKey ? C.orange500 : C.white200}`,
                      background: m.key === selectedModelKey ? 'rgba(249,115,22,0.08)' : 'transparent',
                      cursor: 'pointer', transition: 'all 0.15s',
                    }}
                  >
                    <div style={{ fontSize: 14, fontWeight: 600, color: m.key === selectedModelKey ? C.orange500 : C.white }}>
                      {m.name} {m.key === selectedModelKey && <span style={{ fontSize: 11, marginLeft: 8 }}>当前使用</span>}
                    </div>
                    {m.source && <div style={{ fontSize: 12, color: C.gray500, marginTop: 4 }}>{m.source}</div>}
                  </div>
                ))
              )}
            </>
          )}
        </div>
      </div>
    </>
  )
}

/* ========== 底部输入栏 — 麦克风 + 回答输入 + 面试操作 ========== */

/**
 * 渲染舞台底部作答区：语音按钮、回答输入框、提交按钮与面试操作 chip（恢复下一题/结束面试）。
 */
function StageFooter({ collapsed, onToggle, answer, setAnswer, statusLine, submitting, canSubmit, onSubmit, isRecording, isRecognizing, recordDisabled, onToggleRecording, onNextQuestion, nextDisabled, nextPending, onFinish, finishDisabled, finishPending }: {
  collapsed: boolean
  onToggle: () => void
  answer: string
  setAnswer: (value: string) => void
  statusLine: string
  submitting: boolean
  canSubmit: boolean
  onSubmit: () => void
  isRecording: boolean
  isRecognizing: boolean
  recordDisabled: boolean
  onToggleRecording: () => void
  onNextQuestion: () => void
  nextDisabled: boolean
  nextPending: boolean
  onFinish: () => void
  finishDisabled: boolean
  finishPending: boolean
}) {
  return (
    <div style={{
      background: collapsed ? 'transparent' : C.gray800,
      borderTopLeftRadius: collapsed ? 0 : 12,
      transform: collapsed ? 'translateY(calc(100% - 24px))' : 'translateY(0)',
      transition: 'all 0.3s cubic-bezier(0.4, 0, 0.2, 1)',
      height: '100%', position: 'relative', overflow: collapsed ? 'visible' : 'hidden', paddingBottom: 16,
    }}>
      <div onClick={onToggle} style={{
        height: 24, display: 'flex', alignItems: 'center', justifyContent: 'center',
        cursor: 'pointer', color: C.white700, transition: 'all 0.3s',
      }}
        onMouseEnter={(e) => { e.currentTarget.style.color = '#fff' }}
        onMouseLeave={(e) => { e.currentTarget.style.color = C.white700 }}
      >
        <DownOutlined style={{ transform: collapsed ? 'rotate(180deg)' : 'none', transition: 'transform 0.3s', fontSize: 10 }} />
      </div>
      {!collapsed && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 6, padding: '0 16px', height: 'calc(100% - 24px)' }}>
          {/* 状态行 + 面试操作 chip */}
          <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap', alignItems: 'center', justifyContent: 'space-between' }}>
            <span style={{ fontSize: 12, color: C.white500, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', flex: 1, minWidth: 0 }}>
              {statusLine}
            </span>
            <div style={{ display: 'flex', gap: 6, flexShrink: 0 }}>
              <button
                type="button"
                onClick={onNextQuestion}
                disabled={nextDisabled}
                style={{
                  padding: '4px 10px', borderRadius: 14, border: `1px solid ${C.white200}`,
                  background: C.white050, color: nextDisabled ? C.white500 : C.white, fontSize: 12,
                  cursor: nextDisabled ? 'not-allowed' : 'pointer',
                }}
              >
                {nextPending ? '恢复中...' : '恢复下一题'}
              </button>
              <button
                type="button"
                onClick={onFinish}
                disabled={finishDisabled}
                style={{
                  padding: '4px 10px', borderRadius: 14, border: '1px solid rgba(245,101,101,0.5)',
                  background: 'rgba(245,101,101,0.12)', color: finishDisabled ? C.white500 : C.red500, fontSize: 12,
                  cursor: finishDisabled ? 'not-allowed' : 'pointer',
                }}
              >
                {finishPending ? '生成报告中...' : '结束面试'}
              </button>
            </div>
          </div>
          <div style={{ display: 'flex', gap: 8, alignItems: 'flex-end' }}>
            {/* 麦克风按钮 */}
            <button
              onClick={onToggleRecording}
              disabled={recordDisabled}
              title={isRecording ? '点击停止录音并识别' : '点击开始语音回答'}
              style={{
                width: 50, height: 50, borderRadius: 12, border: 'none', flexShrink: 0,
                background: isRecording ? '#ef4444' : (isRecognizing || recordDisabled ? C.gray600 : '#22c55e'),
                color: '#fff', fontSize: 16, fontWeight: 600,
                cursor: recordDisabled ? 'not-allowed' : 'pointer',
                display: 'flex', alignItems: 'center', justifyContent: 'center',
                transition: 'all 0.15s',
              }}
            >
              {isRecognizing ? '...' : (isRecording ? '⏹' : '🎤')}
            </button>

            <div style={{ flex: 1, position: 'relative' }}>
              <textarea
                value={answer}
                onChange={(e) => setAnswer(e.target.value)}
                onKeyDown={(e) => { if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); onSubmit() } }}
                placeholder={isRecording
                  ? '正在录音，停顿后自动识别；也可点击麦克风手动停止...'
                  : (submitting ? 'AI 正在评估你的回答...' : '输入你的回答，Enter 提交（Shift+Enter 换行）...')}
                style={{
                  width: '100%', height: 80, minHeight: 80, maxHeight: 80,
                  background: C.gray700, border: isRecording ? '2px solid #ef4444' : 'none',
                  borderRadius: 12, color: C.white, fontSize: 16,
                  padding: '16px 16px 0 16px', resize: 'none', lineHeight: 1.4, outline: 'none',
                }}
              />
            </div>
            <button
              onClick={onSubmit}
              disabled={!canSubmit}
              title="提交回答"
              style={{
                width: 50, height: 50, borderRadius: 12, border: 'none', flexShrink: 0,
                background: submitting ? C.gray600 : C.blue500,
                color: '#fff', fontSize: 20, cursor: canSubmit ? 'pointer' : 'not-allowed',
                display: 'flex', alignItems: 'center', justifyContent: 'center',
                opacity: canSubmit ? 1 : 0.5,
              }}
            >
              <SendOutlined />
            </button>
          </div>
        </div>
      )}
    </div>
  )
}

/* ========== 舞台字幕 — 打字机效果播报当前题目/反馈 ========== */

/**
 * 渲染舞台底部悬浮字幕，展示 AI 面试官正在播报的题目或反馈文本。
 */
function StageSubtitle({ text, typing }: { text: string; typing: boolean }) {
  if (!text && !typing) {
    return null
  }

  return (
    <div style={{
      position: 'absolute', bottom: 24, left: '50%', transform: 'translateX(-50%)',
      zIndex: 10, maxWidth: '70%', width: 'auto',
    }}>
      <div style={{
        background: 'rgba(0,0,0,0.7)', backdropFilter: 'blur(12px)',
        borderRadius: 12, padding: '14px 24px',
        border: '1px solid rgba(255,255,255,0.1)',
      }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 6 }}>
          <span style={{
            padding: '2px 8px', borderRadius: 4, fontSize: 11, fontWeight: 600,
            background: 'rgba(66,153,225,0.3)', color: '#93c5fd',
          }}>AI 面试官</span>
          {typing && <span style={{ fontSize: 11, color: '#9ca3af' }}>播报中...</span>}
        </div>
        <p style={{ margin: 0, fontSize: 15, color: '#fff', lineHeight: 1.6, minHeight: 24, whiteSpace: 'pre-wrap' }}>
          {text}
          {typing && <span style={{ opacity: 0.5 }}>|</span>}
        </p>
      </div>
    </div>
  )
}

/* ========== 舞台居中提示卡 — 准备中/报告生成中等特殊状态 ========== */

/**
 * 在舞台中央渲染一张状态提示卡，用于题目准备中、报告生成中等阻断态。
 */
function StageNoticeCard({ title, body, linkTo, linkParams, linkLabel }: {
  title: string
  body: string
  linkTo?: string
  linkParams?: Record<string, string>
  linkLabel?: string
}) {
  return (
    <div style={{
      position: 'absolute', top: '38%', left: '50%', transform: 'translate(-50%, -50%)',
      zIndex: 12, maxWidth: 420, width: '86%',
      background: 'rgba(0,0,0,0.72)', backdropFilter: 'blur(12px)',
      borderRadius: 12, border: '1px solid rgba(255,255,255,0.12)',
      padding: '20px 24px', display: 'flex', flexDirection: 'column', gap: 10,
    }}>
      <strong style={{ fontSize: 15, color: '#fff' }}>{title}</strong>
      <p style={{ margin: 0, fontSize: 13, color: C.white700, lineHeight: 1.7 }}>{body}</p>
      {linkTo && linkLabel && (
        <Link
          to={linkTo}
          params={linkParams}
          style={{
            alignSelf: 'flex-start', padding: '6px 14px', borderRadius: 8,
            border: `1px solid ${C.white200}`, color: C.white, fontSize: 13,
            textDecoration: 'none', background: C.white050,
          }}
        >
          {linkLabel}
        </Link>
      )}
    </div>
  )
}

/* ========== 主页面 ========== */

/**
 * 面试舞台原型页：以陪伴房间的深色沉浸式布局承载标准语音面试全流程，
 * 包含题目 TTS 播报字幕、语音/文字作答、自动推进下一题与结束面试。
 */
export function InterviewStagePrototypePage() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const accessToken = useAuthStore((state) => state.accessToken)
  const params = useParams({ strict: false })
  const interviewId = String(params.interviewId || '')

  const [overlayPanel, setOverlayPanel] = useState<StageOverlayPanelKey | null>(null)
  const [footerCollapsed, setFooterCollapsed] = useState(false)
  const [answer, setAnswer] = useState('')
  const [message, setMessage] = useState('面试链路初始化中。')
  const [runtimeMessages, setRuntimeMessages] = useState<InterviewMessage[]>([])
  const [sessionState, setSessionState] = useState<{ status: string; message: string }>({
    status: 'idle',
    message: '正在恢复面试会话。',
  })
  const [stageDirective, setStageDirective] = useState<InterviewQuestion['live2d_directive'] | null>(null)
  const [isRecording, setIsRecording] = useState(false)
  const [isRecognizing, setIsRecognizing] = useState(false)
  const [isAdvancing, setIsAdvancing] = useState(false)
  const [selectedModelKey, setSelectedModelKey] = useState(() => readSelectedLive2DModelKey('interview', readSelectedFrontendIndustryCode() || INTERVIEW_DEFAULT_INDUSTRY_CODE))

  // 实时语音（WebSocket）链路状态：连接、麦克风权限、流式识别、turn 计数与 PCM 流嘴型。
  const [hasRequestedMicrophonePermission, setHasRequestedMicrophonePermission] = useState(false)
  const [hasGrantedMicrophonePermission, setHasGrantedMicrophonePermission] = useState(false)
  const [wsConnected, setWsConnected] = useState(false)
  const [recognitionPartial, setRecognitionPartial] = useState('')
  const [recognitionFinal, setRecognitionFinal] = useState('')
  const [assistantTurnCount, setAssistantTurnCount] = useState(0)
  const [streamMouthOpen, setStreamMouthOpen] = useState(0)
  const [wsTraceId, setWsTraceId] = useState('')

  // 录音链路资源（非实时：累积 PCM16，停止后送 /companion/asr 识别）
  const recordStreamRef = useRef<MediaStream | null>(null)
  const recordAudioContextRef = useRef<AudioContext | null>(null)
  const recordSourceRef = useRef<MediaStreamAudioSourceNode | null>(null)
  const recordProcessorRef = useRef<ScriptProcessorNode | null>(null)
  const recordSilenceTimeoutRef = useRef<number | null>(null)
  const recordMaxDurationTimerRef = useRef<number | null>(null)
  const recordSpeechDetectedRef = useRef(false)
  const recordStopRequestedRef = useRef(false)
  const nonRealtimePcmBufferRef = useRef<number[]>([])
  // 实时链路：WebSocket 连接、清理定时器、连接复用键、当前 AI 转写文本、PCM 帧队列与发送节奏控制。
  const wsRef = useRef<WebSocket | null>(null)
  const wsCleanupTimerRef = useRef<number | null>(null)
  const wsConnectionKeyRef = useRef('')
  const assistantTranscriptRef = useRef('')
  // 正常结束标记：收到 finished 事件后置 true，onclose 据此跳过 stopPCMStreamPlayback，让结束语播完再跳报告
  const normalFinishRef = useRef(false)
  const recordFrameQueueRef = useRef<string[]>([])
  const recordPendingPCMRef = useRef<number[]>([])
  // 诊断用：onaudioprocess 触发 / 已入队帧数 / 已发送帧数
  const onaudioFiredRef = useRef(false)
  const onAudioCountRef = useRef(0)
  const framePushedRef = useRef(0)
  const chunkSentRef = useRef(0)
  const recordFrameTimerRef = useRef<number | null>(null)
  const recordFrameDrainTimerRef = useRef<number | null>(null)
  // TTS 去重：记录已播报的文本，避免同一题/反馈重复播报
  const lastSpokenContentRef = useRef('')

  // 统一管理 TTS 播放、字幕打字与嘴型同步，与旧版面试页/陪伴房间共用同一播放链路。
  const {
    liveDialogue,
    isDialogueTyping,
    mouthOpen,
    startDialogueTyping,
    stopDialogueTyping,
    stopCurrentPlayback,
    syncDialogueImmediately,
    playTTSAudio,
  } = useLive2DDialoguePlayback({
    initialDialogue: '进入面试后，AI 面试官会在这里播报当前题目。',
    onPlaybackFinished: () => {
      setSessionState((current) => (current.status === 'speaking' ? { status: 'ready', message: '播报完成，可语音或文字作答。' } : current))
    },
    onPlaybackError: (error) => {
      setMessage(extractErrorMessage(error, '自动播放语音失败，已回退到文本模式。'))
    },
  })

  // 实时链路 PCM 流式播放：服务端持续推送的 PCM16 音频块按序接到播放时间线，振幅回传驱动 Live2D 嘴型。
  const {
    enqueuePCM16Base64,
    preparePlayback,
    stop: stopPCMStreamPlayback,
    isPlaying: isPCMPlaying,
    waitForPlaybackEnd: waitForPCMPlaybackEnd,
  } = usePCMStreamPlayer({
    onLevelChange: setStreamMouthOpen,
  })

  const detailQuery = useQuery({
    queryKey: ['interview-detail', accessToken, interviewId],
    queryFn: () => fetchInterviewDetail(accessToken as string, interviewId),
    enabled: Boolean(accessToken && interviewId && /^\d+$/.test(interviewId)),
    retry: false,
    refetchOnWindowFocus: false,
    refetchInterval: (query) => {
      const status = query.state.data?.status
      if (status === 'preparing' || status === 'report_generating') {
        return 3000
      }
      return false
    },
  })

  const nextQuestionMutation = useMutation({
    mutationFn: () => fetchNextInterviewQuestion(accessToken as string, interviewId),
    onSuccess: async (data) => {
      setIsAdvancing(false)
      if (data.question) {
        setMessage(`已进入第 ${data.question_no} 题。`)
      } else {
        setMessage('当前没有更多题目，可直接结束面试生成报告。')
        setSessionState({ status: 'ready', message: '题目已全部完成，可结束面试生成报告。' })
      }
      await queryClient.invalidateQueries({ queryKey: ['interview-detail', accessToken, interviewId] })
    },
    onError: (error) => {
      setIsAdvancing(false)
      setMessage(extractErrorMessage(error, '获取下一题失败，请稍后重试'))
    },
  })

  const submitMutation = useMutation({
    mutationFn: (payload: SubmitInterviewAnswerPayload) => submitInterviewAnswer(accessToken as string, interviewId, payload),
    onSuccess: async (data) => {
      setAnswer('')
      setRecognitionFinal('')
      setRecognitionPartial('')
      await queryClient.invalidateQueries({ queryKey: ['interview-history'] })
      if (data.is_finished) {
        setMessage('本场面试题目已完成，可点击「结束面试」生成报告。')
        await queryClient.invalidateQueries({ queryKey: ['interview-detail', accessToken, interviewId] })
        return
      }
      // 非实时（标准语音）链路：提交后自动出下一题，TTS effect 会自动播报新题；
      // 实时链路答题走 WS，不会进入此分支的自动推进，仅刷新详情等待 WS 推下一题。
      if (!isRealtime) {
        setIsAdvancing(true)
        setMessage('答案已提交，AI 正在出下一题…')
        setSessionState({ status: 'thinking', message: 'AI 正在出下一题…' })
        await queryClient.invalidateQueries({ queryKey: ['interview-detail', accessToken, interviewId] })
        try {
          await nextQuestionMutation.mutateAsync()
        } catch {
          setIsAdvancing(false)
          setMessage('获取下一题失败，可点击「恢复下一题」重试。')
        }
      } else {
        setMessage('答案已提交，下一题已准备好。')
        await queryClient.invalidateQueries({ queryKey: ['interview-detail', accessToken, interviewId] })
      }
    },
    onError: (error) => {
      setIsAdvancing(false)
      setMessage(extractErrorMessage(error, '提交回答失败，请稍后重试'))
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
        params: { interviewId },
      })
    },
    onError: (error) => {
      setMessage(extractErrorMessage(error, '结束面试失败，请稍后重试'))
    },
  })

  const effectiveMessages = runtimeMessages.length ? runtimeMessages : (detailQuery.data?.messages || [])
  const effectiveStatus = detailQuery.data?.status || ''
  const isPreparing = effectiveStatus === 'preparing'
  const isReportGenerating = effectiveStatus === 'report_generating'
  const isInterviewOngoing = effectiveStatus === 'ongoing'
  const isRealtime = Boolean(detailQuery.data?.is_realtime)
  const currentQuestion = useMemo(
    () => resolveCurrentInterviewQuestionFromMessages(effectiveMessages, effectiveStatus, detailQuery.data?.total_questions || 0),
    [detailQuery.data?.total_questions, effectiveMessages, effectiveStatus],
  )
  const isCodingQuestion = currentQuestion?.type === 'coding'
  const sessionIndustryCode = detailQuery.data?.industry_code || readSelectedFrontendIndustryCode() || INTERVIEW_DEFAULT_INDUSTRY_CODE
  const answeredCount = useMemo(
    () => effectiveMessages.filter((item) => item.role === 'user').length,
    [effectiveMessages],
  )
  const canRecord = typeof navigator !== 'undefined' && Boolean(navigator.mediaDevices?.getUserMedia)

  // 面试官 Live2D 模型列表（面试场景），与旧版页面共用同一份缓存键。
  const modelOptionsQuery = useQuery({
    queryKey: ['interview-live2d-models', sessionIndustryCode],
    queryFn: () => fetchSelectableLive2DModels('interview', sessionIndustryCode),
    staleTime: 60 * 1000,
  })
  const modelOptions = modelOptionsQuery.data || []
  const currentModel = useMemo(() => {
    const explicitModel = modelOptions.find((item) => item.key === selectedModelKey)
    if (explicitModel) {
      return explicitModel
    }
    return modelOptions.find((item) => item.is_recommended) || modelOptions[0] || null
  }, [modelOptions, selectedModelKey])

  /**
   * 模型选择变化时写入本地缓存，供下一场面试直接恢复。
   */
  useEffect(() => {
    if (!currentModel?.key) {
      return
    }
    persistSelectedLive2DModelKey('interview', sessionIndustryCode, currentModel.key)
    if (currentModel.key !== selectedModelKey) {
      setSelectedModelKey(currentModel.key)
    }
  }, [currentModel?.key, selectedModelKey, sessionIndustryCode])

  /**
   * 面试详情恢复后同步会话所属行业，并按行业恢复模型选择。
   */
  useEffect(() => {
    if (!detailQuery.data?.industry_code) {
      return
    }
    persistSelectedFrontendIndustryCode(detailQuery.data.industry_code)
    setSelectedLive2DModelKeySafely(detailQuery.data.industry_code)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [detailQuery.data?.industry_code])

  /**
   * 按行业读取已保存的模型 key，只有非空时才覆盖当前选择。
   */
  function setSelectedLive2DModelKeySafely(industryCode: string): void {
    const savedKey = readSelectedLive2DModelKey('interview', industryCode)
    if (savedKey) {
      setSelectedModelKey(savedKey)
    }
  }

  /**
   * 详情首次恢复或刷新成功后同步本地消息快照，并铺好当前题目状态。
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
      if (isRealtime) {
        // 实时会话由 WS 事件驱动 TTS/字幕，这里先把题目文本铺到舞台，不做 TTS 播报。
        stopDialogueTyping(restoredQuestion.question)
      } else {
        setSessionState({ status: 'idle', message: '正在准备题目播报…' })
      }
    } else if (!isRealtime && isInterviewOngoing) {
      setSessionState({ status: 'ready', message: '可点击「恢复下一题」继续，或直接结束面试生成报告。' })
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [detailQuery.data, isInterviewOngoing, isRealtime])

  /**
   * 标准语音面试：每出现一条新的 AI 消息（题目或反馈）就调 /companion/tts 播报，
   * 用字幕+嘴型同步驱动 Live2D 面试官；TTS 失败降级为纯字幕打字机。
   */
  useEffect(() => {
    if (isRealtime || !isInterviewOngoing) {
      return
    }

    const latestAI = [...effectiveMessages]
      .reverse()
      .find((item) => item.role === 'ai' && item.message_type === 'text')
    if (!latestAI) {
      return
    }

    const speakableText = (latestAI.question?.question || latestAI.content || '').trim()
    if (!speakableText || lastSpokenContentRef.current === speakableText) {
      return
    }
    lastSpokenContentRef.current = speakableText
    setSessionState({ status: 'speaking', message: '面试官正在播报，播报结束后可作答。' })

    // 同一文本只播报一次（lastSpokenContentRef 去重），切到新文本时
    // playTTSAudio 内部会先 stopCurrentPlayback 再播新音频，不需要 cleanup。
    void (async () => {
      try {
        const audioUrl = await synthesizeInterviewSpeech(accessToken as string, speakableText)
        if (audioUrl) {
          await playTTSAudio(audioUrl, speakableText)
        } else {
          startDialogueTyping(speakableText)
          setSessionState((current) => (current.status === 'speaking'
            ? { status: 'ready', message: '可以作答：可直接输入文字，或点击麦克风语音回答。' }
            : current))
        }
      } catch {
        startDialogueTyping(speakableText)
        setSessionState((current) => (current.status === 'speaking'
          ? { status: 'ready', message: '语音播报失败，已回退文本模式，可继续作答。' }
          : current))
      }
    })()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [effectiveMessages, isInterviewOngoing, isRealtime, accessToken])

  /**
   * 订阅面试 WebSocket 事件，并在页面侧同步更新题目、语音和 Live2D 指令状态。
   * 仅实时会话建立连接；onmessage 分发 15 类事件，驱动字幕/PCM 播放/录音 turn 管理。
   */
  useEffect(() => {
    if (!accessToken || !interviewId || !detailQuery.data || !isInterviewOngoing || !detailQuery.data.is_realtime) {
      cancelScheduledSocketClose()
      return undefined
    }

    const connectionKey = `${accessToken}:${interviewId}`
    cancelScheduledSocketClose()

    let socket = wsRef.current
    const canReuseSocket = Boolean(
      socket
        && wsConnectionKeyRef.current === connectionKey
        && (socket.readyState === WebSocket.CONNECTING || socket.readyState === WebSocket.OPEN),
    )
    if (!canReuseSocket) {
      if (socket && (socket.readyState === WebSocket.CONNECTING || socket.readyState === WebSocket.OPEN)) {
        socket.close()
      }
      socket = new WebSocket(buildInterviewWebSocketUrl(interviewId, accessToken))
    }

    // 到这里 socket 一定非空：canReuseSocket 为真时它来自非空的 wsRef.current，否则刚 new 出来。
    if (!socket) {
      return undefined
    }

    wsConnectionKeyRef.current = connectionKey
    wsRef.current = socket

    socket.onopen = () => {
      if (wsRef.current !== socket) {
        return
      }
      normalFinishRef.current = false
      setWsConnected(true)
      setMessage('实时面试链路已连接，正在确认当前会话模式。')
    }

    socket.onmessage = (event) => {
      if (wsRef.current !== socket) {
        return
      }
      try {
        const payload = JSON.parse(String(event.data)) as InterviewSocketEvent
        setWsTraceId(payload.trace_id || '')
        console.log('[stage-ws]', payload.type, 'status=', (payload.data as { status?: string } | undefined)?.status, 'content=', payload.content)

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
                setMessage(extractErrorMessage(error, '浏览器阻止了实时语音播放，请点击麦克风按钮后重试。'))
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
          case 'finished': {
            normalFinishRef.current = true
            const finishedMsg = payload.content || '本场面试已结束，正在生成报告。'
            setMessage(finishedMsg)
            setSessionState({ status: 'finished', message: finishedMsg })
            // 等结束语音频/字幕播完再跳报告，避免结束语被截断；最多等 8 秒兜底防卡死。
            void (async () => {
              await Promise.race([
                (async () => {
                  if (isPCMPlaying()) {
                    try { await waitForPCMPlaybackEnd() } catch {}
                  }
                  await new Promise<void>((resolve) => { window.setTimeout(resolve, 400) })
                })(),
                new Promise<void>((resolve) => { window.setTimeout(resolve, 8000) }),
              ])
              navigate({ to: '/interview/$interviewId/report', params: { interviewId } })
            })()
            break
          }
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
      if (wsRef.current !== socket) {
        return
      }
      wsRef.current = null
      wsConnectionKeyRef.current = ''
      setWsConnected(false)
      if (normalFinishRef.current) {
        // 正常结束（收到 finished）：不停 PCM 播放，让结束语播完由 finished handler 跳报告
        return
      }
      stopPCMStreamPlayback()
      setStreamMouthOpen(0)
      setSessionState((current) => (current.status === 'error'
        ? current
        : { status: 'idle', message: '实时链路已断开，当前只能使用 HTTP 文本回退模式。' }))
      setMessage('实时面试链路已断开，当前会退回 HTTP 模式，此模式不会触发语音播报。')
    }

    socket.onerror = () => {
      if (wsRef.current !== socket) {
        return
      }
      stopPCMStreamPlayback()
      setStreamMouthOpen(0)
      setSessionState({
        status: 'error',
        message: '实时面试链路连接异常，请检查后端实时语音配置。',
      })
      setMessage('实时面试链路连接异常，当前会退回 HTTP 模式，此模式不会触发语音播报。')
    }

    return () => {
      scheduleSocketClose(socket, connectionKey)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [accessToken, detailQuery.data, interviewId, isInterviewOngoing])

  /**
   * 实时语音面试进入页面后先抢占一次麦克风权限，避免真正开始回答时才被浏览器权限弹窗打断。
   * 门控用 isRealtime + wsConnected（不依赖 sessionState.mode）。
   */
  useEffect(() => {
    if (!isRealtime || !wsConnected || !canRecord || hasRequestedMicrophonePermission) {
      return
    }

    void ensureMicrophonePermission()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [canRecord, hasRequestedMicrophonePermission, isRealtime, wsConnected])

  /**
   * 当实时面试官完成一轮播报并进入 ready 状态后，自动开始收音，让候选人可以直接开口回答。
   * 等待 PCM 流式播放真正结束后再启动录音，避免 TTS 未播完就开始收音。
   */
  useEffect(() => {
    console.log('[auto-record] gates', { isRealtime, status: sessionState.status, wsConnected, canRecord, isRecording, isCodingQuestion, hasGrantedMicrophonePermission, assistantTurnCount, answer: answer.trim(), recognitionPartial: recognitionPartial.trim() })
    if (!isRealtime || sessionState.status !== 'ready') {
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

    let cancelled = false

    const startAfterPlayback = async () => {
      if (isPCMPlaying()) {
        await waitForPCMPlaybackEnd()
      }
      if (cancelled) return
      // 再等一小段缓冲，确保浏览器音频管道完全排空
      await new Promise<void>((resolve) => {
        window.setTimeout(resolve, 300)
      })
      if (cancelled) return
      console.log('[auto-record] -> startVoiceCapture')
      void startVoiceCapture()
    }

    void startAfterPlayback()

    return () => {
      cancelled = true
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [
    answer,
    assistantTurnCount,
    canRecord,
    hasGrantedMicrophonePermission,
    isCodingQuestion,
    isPCMPlaying,
    isRealtime,
    isRecording,
    recognitionPartial,
    sessionState.status,
    waitForPCMPlaybackEnd,
    wsConnected,
  ])

  /**
   * 页面挂载后尽快预热 Live2D 运行时，减少首次渲染舞台时的额外等待。
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

    const timer = setTimeout(() => {
      prewarmLive2DRuntime()
    }, 600)
    return () => {
      clearTimeout(timer)
    }
  }, [])

  /**
   * 按 20ms 一帧的节奏发送排队中的 PCM 数据，尽量贴近实时语音文档建议的麦克风上行节奏。
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

      chunkSentRef.current += 1
      if (chunkSentRef.current <= 3 || chunkSentRef.current % 50 === 0) {
        console.log('[audio-chunk] send #', chunkSentRef.current, 'queueLeft=', recordFrameQueueRef.current.length)
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
      setMessage(extractErrorMessage(error, '浏览器阻止了自动播放，请点击麦克风按钮后重试。'))
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
      setMessage(extractErrorMessage(error, '麦克风权限请求失败，请点击麦克风按钮并允许浏览器访问麦克风。'))
      return false
    }
  }

  /**
   * 取消已安排的 WebSocket 延迟关闭，避免开发模式下的 StrictMode 清理误杀刚建立的连接。
   */
  function cancelScheduledSocketClose(): void {
    if (wsCleanupTimerRef.current !== null) {
      window.clearTimeout(wsCleanupTimerRef.current)
      wsCleanupTimerRef.current = null
    }
  }

  /**
   * 延迟关闭当前 WebSocket，仅在确认没有被下一轮 effect 复用时才真正断开连接。
   */
  function scheduleSocketClose(socket: WebSocket | null, connectionKey: string): void {
    cancelScheduledSocketClose()
    if (!socket) {
      return
    }

    wsCleanupTimerRef.current = window.setTimeout(() => {
      wsCleanupTimerRef.current = null
      if (wsRef.current !== socket || wsConnectionKeyRef.current !== connectionKey) {
        return
      }

      wsRef.current = null
      wsConnectionKeyRef.current = ''
      socket.close()
    }, 120)
  }

  /**
   * 清理自动判停定时器，避免旧的静音任务误触发。
   */
  function clearRecordSilenceTimer(): void {
    if (recordSilenceTimeoutRef.current !== null) {
      window.clearTimeout(recordSilenceTimeoutRef.current)
      recordSilenceTimeoutRef.current = null
    }
  }

  /**
   * 启动浏览器麦克风采集：实时链路把 16k PCM 帧实时推送到后端 WebSocket；
   * 非实时（标准语音）链路累积 PCM16 采样，停止后打成 Blob 送 /companion/asr 识别。
   */
  async function startVoiceCapture(): Promise<void> {
    console.log('[startVoiceCapture] enter', { isRealtime, wsOpen: wsRef.current?.readyState })
    if (!canRecord) {
      setMessage('当前浏览器不支持麦克风采集，请直接输入文字作答。')
      return
    }

    if (isRealtime && (!wsRef.current || wsRef.current.readyState !== WebSocket.OPEN)) {
      setMessage('实时链路尚未连接，暂时无法启动语音识别。')
      return
    }

    await ensureRealtimeAudioPlaybackReady()
    try {
      const stream = await navigator.mediaDevices.getUserMedia({
        audio: true,
      })
      const micTrack = stream.getAudioTracks()[0]
      console.log('[startVoiceCapture] micTrack', { enabled: micTrack?.enabled, muted: micTrack?.muted, readyState: micTrack?.readyState, sampleRate: micTrack?.getSettings()?.sampleRate })
      const recordContext = new AudioContext({
        sampleRate: 16000,
      })
      await recordContext.resume()
      console.log('[startVoiceCapture] resumed, state=', recordContext.state)
      if (recordContext.state !== 'running') {
        throw new Error('浏览器未真正启动录音上下文，请点击麦克风按钮后重试。')
      }
      const source = recordContext.createMediaStreamSource(stream)
      const processor = recordContext.createScriptProcessor(4096, 1, 1)

      if (isRealtime) {
        wsRef.current?.send(
          JSON.stringify({
            type: 'audio_start',
            data: {
              language: 'zh-CN',
            },
          }),
        )
      }

      recordStopRequestedRef.current = false
      recordSpeechDetectedRef.current = false
      recordPendingPCMRef.current = []
      recordFrameQueueRef.current = []
      nonRealtimePcmBufferRef.current = []
      if (recordFrameDrainTimerRef.current !== null) {
        window.clearInterval(recordFrameDrainTimerRef.current)
        recordFrameDrainTimerRef.current = null
      }
      clearRecordSilenceTimer()
      if (isRealtime) {
        ensureQueuedAudioFramesSending()
      }
      source.connect(processor)
      processor.connect(recordContext.destination)
      processor.onaudioprocess = (event) => {
        if (recordStopRequestedRef.current || (isRealtime && (!wsRef.current || wsRef.current.readyState !== WebSocket.OPEN))) {
          return
        }

        const channelData = event.inputBuffer.getChannelData(0)
        if (!onaudioFiredRef.current) {
          onaudioFiredRef.current = true
          console.log('[onaudioprocess] first frame fired, sampleRate=', event.inputBuffer.sampleRate, 'len=', channelData.length)
        }
        let signalEnergy = 0
        for (const sample of channelData) {
          signalEnergy += sample * sample
        }
        const rms = Math.sqrt(signalEnergy / Math.max(channelData.length, 1))
        onAudioCountRef.current += 1
        if (onAudioCountRef.current <= 5 || onAudioCountRef.current % 50 === 0) {
          let maxAbs = 0
          for (const s of channelData) {
            const a = Math.abs(s)
            if (a > maxAbs) maxAbs = a
          }
          console.log('[onaudioprocess] #', onAudioCountRef.current, 'rms=', rms.toFixed(5), 'maxAbs=', maxAbs.toFixed(5), 'speech=', recordSpeechDetectedRef.current)
        }
        if (rms >= INTERVIEW_AUTO_STOP_LEVEL_THRESHOLD) {
          if (!recordSpeechDetectedRef.current) {
            console.log('[vad] speech detected, rms=', rms.toFixed(4))
          }
          recordSpeechDetectedRef.current = true
          clearRecordSilenceTimer()
        } else if (recordSpeechDetectedRef.current && recordSilenceTimeoutRef.current === null) {
          console.log('[vad] silence timer set (1800ms)')
          recordSilenceTimeoutRef.current = window.setTimeout(() => {
            recordSilenceTimeoutRef.current = null
            if (recordStopRequestedRef.current || !recordSpeechDetectedRef.current) {
              return
            }

            console.log('[vad] silence timer fired -> auto stop')
            setMessage('检测到你已停顿，正在自动结束并提交本轮回答。')
            stopVoiceCapture('auto')
          }, INTERVIEW_AUTO_STOP_SILENCE_MS)
        }
        const pcmChunk = resampleFloat32ToPCM16(channelData, event.inputBuffer.sampleRate, 16000)
        if (!pcmChunk.length) {
          return
        }

        if (isRealtime) {
          recordPendingPCMRef.current.push(...pcmChunk)
          while (recordPendingPCMRef.current.length >= 320) {
            const frame = new Int16Array(recordPendingPCMRef.current.slice(0, 320))
            recordPendingPCMRef.current = recordPendingPCMRef.current.slice(320)
            const audioBase64 = encodePCM16Base64FromInt16(frame)
            if (!audioBase64) {
              continue
            }
            recordFrameQueueRef.current.push(audioBase64)
            framePushedRef.current += 1
            if (framePushedRef.current % 50 === 0) {
              console.log('[audio-frame] pushed #', framePushedRef.current, 'queueLen=', recordFrameQueueRef.current.length)
            }
          }
        } else {
          nonRealtimePcmBufferRef.current.push(...pcmChunk)
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
      setMessage(isRealtime
        ? '正在实时识别你的回答，请继续说；停顿后会自动提交。'
        : '正在录音，停顿后会自动识别并填入回答框；也可点击麦克风手动停止。')
      if (recordMaxDurationTimerRef.current !== null) {
        window.clearTimeout(recordMaxDurationTimerRef.current)
      }
      recordMaxDurationTimerRef.current = window.setTimeout(() => {
        recordMaxDurationTimerRef.current = null
        setMessage(isRealtime ? '已达到单轮最大录音时长，正在自动提交。' : '已达到单轮最大录音时长，正在自动识别。')
        stopVoiceCapture('auto')
      }, INTERVIEW_MAX_RECORDING_MS)
    } catch (error) {
      console.log('[startVoiceCapture] error', error)
      setMessage(extractErrorMessage(error, '麦克风权限申请失败，请检查浏览器设置'))
    }
  }

  /**
   * 停止当前麦克风采集：实时链路按既定节奏把剩余 PCM 帧发完并通知后端结束；
   * 非实时链路把累积的 PCM16 打成 Blob 送 /companion/asr 识别后回填回答框。
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

    setIsRecording(false)
    recordStopRequestedRef.current = false

    if (isRealtime) {
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
    } else {
      const capturedSamples = nonRealtimePcmBufferRef.current
      nonRealtimePcmBufferRef.current = []
      if (reason !== 'cleanup' && capturedSamples.length > 0) {
        void finishNonRealtimeASR(capturedSamples, reason === 'auto' ? 'auto' : 'manual')
      }
    }
  }

  /**
   * 把累积的 PCM16 采样编码为 Blob，POST /companion/asr 识别后回填回答框。
   */
  async function finishNonRealtimeASR(samples: number[], reason: 'manual' | 'auto'): Promise<void> {
    setIsRecognizing(true)
    setMessage(reason === 'auto' ? '检测到你已停顿，正在识别你的语音回答。' : '录音已结束，正在识别你的语音回答。')
    setSessionState({ status: 'thinking', message: '正在识别语音…' })
    try {
      const int16 = new Int16Array(samples)
      const blob = new Blob([int16.buffer], { type: 'application/octet-stream' })
      const result = await recognizeSpeech(accessToken as string, blob, 'pcm', 16000, 'zh-CN')
      const text = (result.text || '').trim()
      if (text) {
        setAnswer(text)
        setMessage('已识别语音回答，可直接提交或修改后提交。')
        setSessionState({ status: 'ready', message: '语音已识别，可提交回答。' })
      } else {
        setMessage('未识别到有效内容，请直接输入文字或重新语音回答。')
        setSessionState({ status: 'ready', message: '未识别到内容，可继续作答。' })
      }
    } catch (error) {
      setMessage(extractErrorMessage(error, '语音识别失败，请直接输入文字作答。'))
      setSessionState({ status: 'ready', message: '语音识别失败，可继续作答。' })
    } finally {
      setIsRecognizing(false)
    }
  }

  /**
   * 页面卸载时释放音频播放、WebSocket 与录音资源，避免浏览器残留占用。
   */
  useEffect(() => {
    return () => {
      cancelScheduledSocketClose()
      stopCurrentPlayback()
      stopPCMStreamPlayback()
      stopQueuedAudioFramesSending()
      if (recordFrameDrainTimerRef.current !== null) {
        window.clearInterval(recordFrameDrainTimerRef.current)
        recordFrameDrainTimerRef.current = null
      }
      stopVoiceCapture('cleanup')
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  /**
   * 提交当前回答：实时链路优先走 WebSocket（不触发 HTTP 自动推进），断开时回退 HTTP。
   */
  async function handleSubmitAnswer(): Promise<void> {
    if (!accessToken) {
      requestLoginPrompt(`/interview/${interviewId}`, 'missing')
      return
    }

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

    setMessage(isRealtime
      ? '当前实时链路未连接，本次将走 HTTP 回退模式；该模式只返回文本，不会触发 TTS 语音。'
      : '正在提交回答，AI 评估后将自动出下一题。')
    await submitMutation.mutateAsync({
      answer: content,
    })
  }

  const submitting = submitMutation.isPending
  const canSubmit = Boolean(answer.trim()) && !isRecording && !submitting && !isAdvancing && isInterviewOngoing
  const recordDisabled = !canRecord || isRecognizing || submitting || isAdvancing || !isInterviewOngoing
    || sessionState.status === 'speaking' || sessionState.status === 'thinking'
    || (isRealtime && !wsConnected)
  const statusLine = sessionState.message || message

  return (
    <div style={{
      width: '100vw', height: '100vh', overflow: 'hidden',
      background: C.gray900, color: '#fff', display: 'flex', flexDirection: 'row',
    }}>
      <div style={{
        flex: 1, height: '100%', position: 'relative',
        display: 'flex', flexDirection: 'column',
        transition: 'all 0.3s cubic-bezier(0.4, 0, 0.2, 1)', overflow: 'hidden',
      }}>
        <div style={{ flex: 1, position: 'relative', overflow: 'hidden' }}>
          <StageErrorBoundary>
            {/* 等待模型列表查询落定后再挂载 canvas，避免先用兜底模型渲染后切换导致闪烁 */}
            {modelOptionsQuery.isLoading ? (
              <div style={{
                position: 'absolute', inset: 0, zIndex: 1,
                display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', gap: 12,
                background: 'radial-gradient(ellipse at 50% 60%, #1e3a5f 0%, #0f1b2d 70%)',
              }}>
                <Spin size="large" />
                <p style={{ color: C.white500, fontSize: 14, margin: 0 }}>正在加载面试官模型...</p>
              </div>
            ) : (
              <StageLive2DCanvas
                modelUrl={currentModel?.model_url || DEMO_MODEL_URL}
                backgroundImageUrl={currentModel ? resolveSelectableLive2DBackgroundImageUrl(currentModel) : undefined}
                mouthOpen={Math.max(mouthOpen, streamMouthOpen)}
                motions={currentModel?.motions || []}
                directive={stageDirective || currentQuestion?.live2d_directive || null}
              />
            )}
          </StageErrorBoundary>

          {/* 返回按钮 */}
          <div style={{ position: 'absolute', top: 20, left: 20, zIndex: 10 }}>
            <Link
              to="/interview"
              style={{
                display: 'inline-flex', alignItems: 'center', gap: 6,
                padding: '8px 16px', borderRadius: 20, fontSize: 14, fontWeight: 500,
                color: '#fff', background: 'rgba(0,0,0,0.4)',
                backdropFilter: 'blur(8px)', textDecoration: 'none',
                transition: 'background 0.15s',
              }}
              onMouseEnter={(e) => { e.currentTarget.style.background = 'rgba(0,0,0,0.6)' }}
              onMouseLeave={(e) => { e.currentTarget.style.background = 'rgba(0,0,0,0.4)' }}
            >
              <ArrowLeftOutlined /> 返回面试入口
            </Link>
          </div>

          {/* 会话状态徽标 */}
          <div style={{ position: 'absolute', top: 20, left: 180, zIndex: 10 }}>
            <div style={{
              padding: '8px 16px', borderRadius: 20, fontSize: 13, fontWeight: 500,
              color: '#fff', background: 'rgba(0,0,0,0.4)', backdropFilter: 'blur(8px)',
              display: 'flex', alignItems: 'center', gap: 12,
            }}>
              <span>🎯 {formatInterviewStatusLabel(effectiveStatus)}</span>
              <span>💬 已答 {answeredCount} 题{detailQuery.data?.total_questions ? ` / 共 ${detailQuery.data.total_questions} 题` : ''}</span>
            </div>
          </div>

          {/* 右上浮层唤起按钮（面试记录 / 模型切换），按需弹出，不常驻 */}
          <div style={{ position: 'absolute', top: 20, right: 20, zIndex: 10, display: 'flex', alignItems: 'center', gap: 8 }}>
            <button
              type="button"
              title="面试记录"
              onClick={() => setOverlayPanel(overlayPanel === 'history' ? null : 'history')}
              style={{
                width: 36, height: 36, borderRadius: 18, border: 'none', cursor: 'pointer',
                display: 'inline-flex', alignItems: 'center', justifyContent: 'center', fontSize: 16,
                color: overlayPanel === 'history' ? '#fff' : C.white700,
                background: overlayPanel === 'history' ? 'rgba(249,115,22,0.85)' : 'rgba(0,0,0,0.4)',
                backdropFilter: 'blur(8px)', transition: 'all 0.15s',
              }}
            >
              <HistoryOutlined />
            </button>
            <button
              type="button"
              title="切换模型"
              onClick={() => setOverlayPanel(overlayPanel === 'model' ? null : 'model')}
              style={{
                width: 36, height: 36, borderRadius: 18, border: 'none', cursor: 'pointer',
                display: 'inline-flex', alignItems: 'center', justifyContent: 'center', fontSize: 16,
                color: overlayPanel === 'model' ? '#fff' : C.white700,
                background: overlayPanel === 'model' ? 'rgba(249,115,22,0.85)' : 'rgba(0,0,0,0.4)',
                backdropFilter: 'blur(8px)', transition: 'all 0.15s',
              }}
            >
              <SettingOutlined />
            </button>
          </div>

          {/* 右上浮层面板：面试记录 / 模型切换 */}
          {overlayPanel && (
            <StageOverlayPanel
              panel={overlayPanel}
              onClose={() => setOverlayPanel(null)}
              messages={effectiveMessages}
              modelOptions={modelOptions}
              selectedModelKey={currentModel?.key || selectedModelKey}
              setSelectedModelKey={setSelectedModelKey}
              modelOptionsQuery={modelOptionsQuery}
            />
          )}

          {/* 特殊状态提示卡 */}
          {detailQuery.isError ? (
            <StageNoticeCard
              title="面试会话恢复失败"
              body={detailQuery.error instanceof Error ? detailQuery.error.message : '面试详情加载失败，请刷新重试。'}
            />
          ) : null}
          {isPreparing ? (
            <StageNoticeCard
              title="题目准备中"
              body={detailQuery.data?.task_error || 'AI 正在生成本场面试的题目，页面会自动刷新，准备完成后开始播报第一题。'}
            />
          ) : null}
          {isReportGenerating ? (
            <StageNoticeCard
              title="报告生成中"
              body="本场面试已结束，系统正在补评答案并生成报告，完成后可直接进入报告页。"
              linkTo="/interview/$interviewId/report"
              linkParams={{ interviewId }}
              linkLabel="去报告页查看进度"
            />
          ) : null}
          {effectiveStatus === 'completed' ? (
            <StageNoticeCard
              title="当前面试已结束"
              body="这场面试已经完成，可以直接进入报告页查看总结结果。"
              linkTo="/interview/$interviewId/report"
              linkParams={{ interviewId }}
              linkLabel="查看面试报告"
            />
          ) : null}
          {isCodingQuestion && isInterviewOngoing ? (
            <StageNoticeCard
              title="编程题请使用旧版页面"
              body="本页暂未内置代码编辑器，编程题建议到旧版舞台用 Monaco 工作区作答；也可在下方直接文字描述思路提交。"
              linkTo="/interview/$interviewId/legacy"
              linkParams={{ interviewId }}
              linkLabel="进入旧版舞台作答"
            />
          ) : null}

          {/* 题目/反馈播报字幕 — 由 useLive2DDialoguePlayback 与 TTS 音频同步推进 */}
          <StageSubtitle text={liveDialogue} typing={isDialogueTyping} />
        </div>

        <div style={{
          width: '100%', height: footerCollapsed ? 24 : 160,
          transition: 'all 0.3s cubic-bezier(0.4, 0, 0.2, 1)',
          willChange: 'transform', position: 'relative', zIndex: 1,
        }}>
          <StageFooter
            collapsed={footerCollapsed}
            onToggle={() => setFooterCollapsed(!footerCollapsed)}
            answer={answer}
            setAnswer={setAnswer}
            statusLine={statusLine}
            submitting={submitting}
            canSubmit={canSubmit}
            onSubmit={() => void handleSubmitAnswer()}
            isRecording={isRecording}
            isRecognizing={isRecognizing}
            recordDisabled={recordDisabled && !isRecording}
            onToggleRecording={() => {
              if (isRecording) {
                stopVoiceCapture()
                return
              }
              void startVoiceCapture()
            }}
            onNextQuestion={() => void nextQuestionMutation.mutateAsync().catch(() => undefined)}
            nextDisabled={nextQuestionMutation.isPending || !isInterviewOngoing || Boolean(currentQuestion)}
            nextPending={nextQuestionMutation.isPending}
            onFinish={() => void finishMutation.mutateAsync().catch(() => undefined)}
            finishDisabled={finishMutation.isPending || !isInterviewOngoing}
            finishPending={finishMutation.isPending}
          />
        </div>
      </div>
    </div>
  )
}
