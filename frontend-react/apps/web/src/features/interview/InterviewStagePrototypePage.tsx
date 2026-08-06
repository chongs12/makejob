/**
 * PROTOTYPE — 面试舞台页原型
 * 目标：把面试进行页的视觉风格同步为陪伴房间（/companion/room, PrototypeUIPage）的
 * 深色沉浸式布局：左侧可折叠面板 + 全屏 Live2D 舞台 + 底部输入栏。
 * 相比陪伴页删去了计划/目标/建议动作等模块，聚焦标准语音（非实时）面试链路：
 * 题目 TTS 播报字幕、语音/文字作答、自动推进下一题、结束面试生成报告。
 * 实时语音面试与编程题工作区暂由旧版页面（/interview/$interviewId）承载。
 * 确认效果后此页将替换 InterviewSessionPage，届时删除本文件的 PROTOTYPE 标记。
 */
import { useState, useRef, useEffect, useMemo, Component, type ReactNode } from 'react'
import { Link, useNavigate, useParams } from '@tanstack/react-router'
import { Spin } from 'antd'
import {
  ArrowLeftOutlined,
  DownOutlined,
  HistoryOutlined,
  InfoCircleOutlined,
  LeftOutlined,
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
  formatInterviewDateTime,
  INTERVIEW_AUTO_STOP_LEVEL_THRESHOLD,
  INTERVIEW_AUTO_STOP_SILENCE_MS,
  INTERVIEW_MAX_RECORDING_MS,
  resampleFloat32ToPCM16,
  resolveCurrentInterviewQuestion,
  resolveCurrentInterviewQuestionFromMessages,
} from './interviewHelpers'
import type { InterviewMessage, InterviewQuestion } from './interviewTypes'

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

  // 嘴型开合由父组件传入，每帧写入模型参数，避免 React 重渲染参与逐帧更新。
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
        model.anchor.set(0.5, 1)

        /**
         * 计算模型贴合舞台的自然缩放比例。
         */
        function getFitScale(): number {
          if (!host) return 0.1
          const w = host.clientWidth
          const h = host.clientHeight
          const safeWidth = Math.max(model.width, 1)
          const safeHeight = Math.max(model.height, 1)
          const widthScale = (w * 0.82) / safeWidth
          const heightScale = (h * 1.04) / safeHeight
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
          model.x = w * 0.5
          model.y = h * 0.94
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

        // 每帧把最新嘴型开合写入模型，与音频分析器输出同步驱动说话口型。
        app.ticker.add(() => {
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

/* ========== 左侧边栏 — 面试记录 / 面试信息 / 模型三面板 ========== */

type StageSidebarPanel = 'chat' | 'info' | 'model'

/**
 * 把面试状态编码转换成侧栏可读的中文标签。
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

/**
 * 渲染舞台左侧可折叠边栏：默认展示面试记录，另含面试信息与模型切换面板。
 */
function StageSidebar({ collapsed, onToggle, interviewId, messages, status, totalQuestions, answeredCount, modelOptions, selectedModelKey, setSelectedModelKey, modelOptionsQuery }: {
  collapsed: boolean
  onToggle: () => void
  interviewId: string
  messages: InterviewMessage[]
  status: string
  totalQuestions: number
  answeredCount: number
  modelOptions: any[]
  selectedModelKey: string
  setSelectedModelKey: (key: string) => void
  modelOptionsQuery: any
}) {
  const [activePanel, setActivePanel] = useState<StageSidebarPanel>('chat')
  const listRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (listRef.current) listRef.current.scrollTop = listRef.current.scrollHeight
  }, [messages.length])

  const panelButtons = [
    { key: 'chat' as const, icon: <HistoryOutlined />, label: '记录' },
    { key: 'info' as const, icon: <InfoCircleOutlined />, label: '面试' },
    { key: 'model' as const, icon: <SettingOutlined />, label: '模型' },
  ]

  const panelTitles: Record<StageSidebarPanel, string> = {
    chat: '面试记录',
    info: '面试信息',
    model: '切换模型',
  }

  return (
    <div style={{
      position: 'relative', width: collapsed ? 24 : 440, height: '100%', background: C.gray900,
      transform: collapsed ? 'translateX(calc(-100% + 24px))' : 'translateX(0)',
      transition: 'all 0.3s cubic-bezier(0.4, 0, 0.2, 1)',
      display: 'flex', flexDirection: 'column', gap: 12,
      overflow: collapsed ? 'visible' : 'hidden', paddingBottom: 16, flexShrink: 0,
    }}>
      <div
        onClick={onToggle}
        style={{
          position: 'absolute', right: 0, top: 0, width: 24, height: '100%',
          display: 'flex', alignItems: 'center', justifyContent: 'center',
          cursor: 'pointer', color: C.white700, transition: 'all 0.3s', zIndex: 1,
        }}
        onMouseEnter={(e) => { e.currentTarget.style.color = '#fff' }}
        onMouseLeave={(e) => { e.currentTarget.style.color = C.white700 }}
      >
        <LeftOutlined style={{ transform: collapsed ? 'rotate(180deg)' : 'none', transition: 'transform 0.3s' }} />
      </div>

      {!collapsed && (
        <>
          {/* 面板切换按钮 */}
          <div style={{ width: '100%', display: 'flex', alignItems: 'center', gap: 2, padding: '8px 8px 0' }}>
            {panelButtons.map((btn) => (
              <button
                key={btn.key}
                onClick={() => setActivePanel(btn.key)}
                style={{
                  flex: 1, display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 2,
                  padding: '8px 4px', borderRadius: 8, border: 'none', cursor: 'pointer',
                  background: activePanel === btn.key ? C.white100 : 'transparent',
                  color: activePanel === btn.key ? '#fff' : C.gray500,
                  fontSize: 10, fontWeight: activePanel === btn.key ? 600 : 400,
                  transition: 'all 0.15s',
                }}
              >
                <span style={{ fontSize: 16 }}>{btn.icon}</span>
                {btn.label}
              </button>
            ))}
          </div>

          {/* 面板标题 */}
          <div style={{ padding: '0 16px', fontSize: 16, fontWeight: 600, color: '#fff' }}>
            {panelTitles[activePanel]}
          </div>

          {/* 面板内容 */}
          <div ref={listRef} style={{
            flex: 1, overflowY: 'auto', padding: '0 16px',
            display: 'flex', flexDirection: 'column', gap: 8,
          }}>
            {/* 面试记录面板 */}
            {activePanel === 'chat' && (
              <div style={{
                flex: 1, overflowY: 'auto',
                border: `1px solid ${C.white200}`, borderRadius: 8,
                background: 'rgba(0,0,0,0.25)', padding: 16,
                display: 'flex', flexDirection: 'column', gap: 8,
              }}>
                {messages.length === 0 ? (
                  <p style={{ fontSize: 13, color: C.gray500, textAlign: 'center', padding: 24 }}>面试开始后，题目和回答会记录在这里</p>
                ) : (
                  messages.map((msg, index) => <InterviewChatBubble key={`${msg.role}-${msg.created_at}-${index}`} msg={msg} />)
                )}
              </div>
            )}

            {/* 面试信息面板 */}
            {activePanel === 'info' && (
              <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
                <div style={{
                  padding: '16px', borderRadius: 8,
                  border: `1px solid ${C.white200}`, background: 'rgba(0,0,0,0.2)',
                }}>
                  <div style={{ fontSize: 15, fontWeight: 700, color: '#fff', marginBottom: 12 }}>
                    本场面试概览
                  </div>
                  <div style={{ display: 'flex', gap: 16 }}>
                    {[
                      { label: '状态', value: formatInterviewStatusLabel(status), color: status === 'ongoing' ? C.green500 : C.gray500 },
                      { label: '已答', value: String(answeredCount), color: C.orange500 },
                      { label: '总题数', value: totalQuestions > 0 ? String(totalQuestions) : '--', color: C.gray500 },
                    ].map((s) => (
                      <div key={s.label} style={{ textAlign: 'center' }}>
                        <div style={{ fontSize: 18, fontWeight: 700, color: s.color }}>{s.value}</div>
                        <div style={{ fontSize: 11, color: C.gray500 }}>{s.label}</div>
                      </div>
                    ))}
                  </div>
                </div>

                {(status === 'completed' || status === 'report_generating') && (
                  <Link
                    to="/interview/$interviewId/report"
                    params={{ interviewId }}
                    style={{
                      padding: '10px 12px', borderRadius: 8, border: `1px solid ${C.white200}`,
                      color: C.white, fontSize: 13, fontWeight: 600, textAlign: 'center',
                      textDecoration: 'none', background: C.white050,
                    }}
                  >
                    {status === 'completed' ? '查看面试报告' : '报告生成中，去报告页查看进度'}
                  </Link>
                )}

                <p style={{ fontSize: 12, color: C.gray500, margin: 0, lineHeight: 1.6 }}>
                  标准语音面试：面试官播报题目后即可用麦克风或文字作答，提交后自动进入下一题。
                </p>
              </div>
            )}

            {/* 模型面板 */}
            {activePanel === 'model' && (
              <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
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
              </div>
            )}
          </div>
        </>
      )}
    </div>
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

  const [sidebarCollapsed, setSidebarCollapsed] = useState(false)
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

  const detailQuery = useQuery({
    queryKey: ['interview-detail', accessToken, interviewId],
    queryFn: () => fetchInterviewDetail(accessToken as string, interviewId),
    enabled: Boolean(accessToken && interviewId),
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
      await queryClient.invalidateQueries({ queryKey: ['interview-history'] })
      if (data.is_finished) {
        setMessage('本场面试题目已完成，可点击「结束面试」生成报告。')
        await queryClient.invalidateQueries({ queryKey: ['interview-detail', accessToken, interviewId] })
        return
      }
      // 标准语音链路：提交后自动出下一题，TTS effect 会自动播报新题。
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
      setStageDirective(restoredQuestion.live2d_directive || null)
      if (isRealtime) {
        // 实时会话在原型页只做展示，题目直接铺到字幕，不做 TTS 播报。
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
   * 清理自动判停定时器，避免旧的静音任务误触发。
   */
  function clearRecordSilenceTimer(): void {
    if (recordSilenceTimeoutRef.current !== null) {
      window.clearTimeout(recordSilenceTimeoutRef.current)
      recordSilenceTimeoutRef.current = null
    }
  }

  /**
   * 启动麦克风采集：累积 16k PCM16 采样，检测到停顿或达到最大时长后自动停止并识别。
   */
  async function startVoiceCapture(): Promise<void> {
    if (!canRecord) {
      setMessage('当前浏览器不支持麦克风采集，请直接输入文字作答。')
      return
    }

    try {
      const stream = await navigator.mediaDevices.getUserMedia({
        audio: true,
      })
      const recordContext = new AudioContext({
        sampleRate: 16000,
      })
      await recordContext.resume()
      if (recordContext.state !== 'running') {
        throw new Error('浏览器未真正启动录音上下文，请点击麦克风按钮后重试。')
      }
      const source = recordContext.createMediaStreamSource(stream)
      const processor = recordContext.createScriptProcessor(4096, 1, 1)

      recordStopRequestedRef.current = false
      recordSpeechDetectedRef.current = false
      nonRealtimePcmBufferRef.current = []
      clearRecordSilenceTimer()

      source.connect(processor)
      processor.connect(recordContext.destination)
      processor.onaudioprocess = (event) => {
        if (recordStopRequestedRef.current) {
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

            setMessage('检测到你已停顿，正在自动结束录音并识别。')
            stopVoiceCapture('auto')
          }, INTERVIEW_AUTO_STOP_SILENCE_MS)
        }
        const pcmChunk = resampleFloat32ToPCM16(channelData, event.inputBuffer.sampleRate, 16000)
        if (!pcmChunk.length) {
          return
        }
        nonRealtimePcmBufferRef.current.push(...pcmChunk)
      }

      recordStreamRef.current = stream
      recordAudioContextRef.current = recordContext
      recordSourceRef.current = source
      recordProcessorRef.current = processor
      setIsRecording(true)
      setMessage('正在录音，停顿后会自动识别并填入回答框；也可点击麦克风手动停止。')
      if (recordMaxDurationTimerRef.current !== null) {
        window.clearTimeout(recordMaxDurationTimerRef.current)
      }
      recordMaxDurationTimerRef.current = window.setTimeout(() => {
        recordMaxDurationTimerRef.current = null
        setMessage('已达到单轮最大录音时长，正在自动识别。')
        stopVoiceCapture('auto')
      }, INTERVIEW_MAX_RECORDING_MS)
    } catch (error) {
      setMessage(extractErrorMessage(error, '麦克风权限申请失败，请检查浏览器设置'))
    }
  }

  /**
   * 停止麦克风采集，把累积 PCM16 打成 Blob 送 /companion/asr 识别后回填回答框。
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

    const capturedSamples = nonRealtimePcmBufferRef.current
    nonRealtimePcmBufferRef.current = []
    if (reason !== 'cleanup' && capturedSamples.length > 0) {
      void finishNonRealtimeASR(capturedSamples, reason === 'auto' ? 'auto' : 'manual')
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
   * 页面卸载时释放音频播放与录音资源，避免浏览器残留占用。
   */
  useEffect(() => {
    return () => {
      stopCurrentPlayback()
      stopVoiceCapture('cleanup')
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  /**
   * 提交当前回答（HTTP 链路），成功后由 mutation 自动推进下一题。
   */
  async function handleSubmitAnswer(): Promise<void> {
    if (!accessToken) {
      requestLoginPrompt(`/interview/${interviewId}/stage-proto`, 'missing')
      return
    }

    const content = answer.trim()
    if (!content) {
      setMessage('先输入你的回答，再提交给 AI 面试官。')
      return
    }

    setMessage('正在提交回答，AI 评估后将自动出下一题。')
    await submitMutation.mutateAsync({
      answer: content,
    })
  }

  const submitting = submitMutation.isPending
  const canSubmit = Boolean(answer.trim()) && !isRecording && !submitting && !isAdvancing && isInterviewOngoing
  const recordDisabled = !canRecord || isRecognizing || submitting || isAdvancing || !isInterviewOngoing
    || sessionState.status === 'speaking' || sessionState.status === 'thinking'
  const statusLine = sessionState.message || message

  return (
    <div style={{
      width: '100vw', height: '100vh', overflow: 'hidden',
      background: C.gray900, color: '#fff', display: 'flex', flexDirection: 'row',
    }}>
      <StageSidebar
        collapsed={sidebarCollapsed}
        onToggle={() => setSidebarCollapsed(!sidebarCollapsed)}
        interviewId={interviewId}
        messages={effectiveMessages}
        status={effectiveStatus}
        totalQuestions={detailQuery.data?.total_questions || 0}
        answeredCount={answeredCount}
        modelOptions={modelOptions}
        selectedModelKey={currentModel?.key || selectedModelKey}
        setSelectedModelKey={setSelectedModelKey}
        modelOptionsQuery={modelOptionsQuery}
      />

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
                mouthOpen={mouthOpen}
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

          {/* 旧版页面入口（原型对照用） */}
          <div style={{ position: 'absolute', top: 20, right: 20, zIndex: 10 }}>
            <Link
              to="/interview/$interviewId"
              params={{ interviewId }}
              style={{
                display: 'inline-flex', alignItems: 'center', gap: 6,
                padding: '8px 16px', borderRadius: 20, fontSize: 13, fontWeight: 500,
                color: C.white700, background: 'rgba(0,0,0,0.4)',
                backdropFilter: 'blur(8px)', textDecoration: 'none',
              }}
            >
              旧版页面
            </Link>
          </div>

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
          {isRealtime && isInterviewOngoing ? (
            <StageNoticeCard
              title="实时语音面试请使用旧版页面"
              body="原型页当前聚焦标准语音面试链路，实时语音（WebSocket）会话请回到旧版页面继续。"
              linkTo="/interview/$interviewId"
              linkParams={{ interviewId }}
              linkLabel="回到旧版页面"
            />
          ) : null}
          {isCodingQuestion && isInterviewOngoing ? (
            <StageNoticeCard
              title="编程题请使用旧版页面"
              body="原型页暂未内置代码编辑器，编程题建议回到旧版页面用 Monaco 工作区作答；也可在下方直接文字描述思路提交。"
              linkTo="/interview/$interviewId"
              linkParams={{ interviewId }}
              linkLabel="回到旧版页面作答"
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
