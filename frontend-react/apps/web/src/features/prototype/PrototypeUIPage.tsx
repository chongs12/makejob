/**
 * PROTOTYPE — throwaway UI exploration
 * Question: "CompanionWorkspace 重构后应该长什么样？"
 * Faithful reproduction of Open-LLM-VTuber-Web visual design
 * Live2D: directly uses live2dStageRuntime, no preset/expression logic
 * Delete this file and the /prototype-ui route when done.
 */
import { useState, useRef, useEffect, useMemo, Component, type ReactNode } from 'react'
import { Link, useNavigate } from '@tanstack/react-router'
import { Modal, Spin } from 'antd'
import {
  SettingOutlined,
  UsergroupAddOutlined,
  HistoryOutlined,
  PlusOutlined,
  DownOutlined,
  SendOutlined,
  LeftOutlined,
  ArrowLeftOutlined,
} from '@ant-design/icons'
import { loadLive2DRuntime } from '../../shared/live2dRuntime'
import { resolveSelectableLive2DBackgroundImageUrl, type SelectableLive2DModelMotion } from '../../shared/live2dModelCatalog'
import { useLive2DDialoguePlayback } from '../../shared/useLive2DDialoguePlayback'
import type { Live2DDirective } from '../../shared/live2dDirective'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useAuthStore } from '../../state/auth'
import { useFrontendIndustriesQuery, usePracticeStatsQuery } from '../../shared/frontendQueries'
import { DEFAULT_FRONTEND_INDUSTRY_CODE, readSelectedFrontendIndustryCode, resolvePreferredFrontendIndustry, persistSelectedFrontendIndustryCode } from '../../shared/industryContext'
import { requestLoginPrompt } from '../../shared/loginPrompt'
import { applyCompanionPlanAdjustment, fetchCurrentPlan, fetchCompanionPlanProgress, fetchSelectableCompanionLive2DModels, previewCompanionPlanAdjustment, sendCompanionChatRequest, recognizeSpeech } from '../companion/companionApi'
import { buildCompanionCurrentPlanQueryKey, buildCompanionPlanProgressQueryKey, buildCompanionLive2DModelsQueryKey, invalidateCompanionPlanQueries } from '../../shared/queryKeys'
import { deriveTodayGoals, deriveActiveGoals, resolveFocusedCompanionTask, buildCompanionWorkspaceResumeMessage } from '../companion/companionHelpers'
import { readCompanionFocusTask, readCompanionDailyDigest, persistCompanionSessionSummary, readSelectedCompanionModelKey, persistSelectedCompanionModelKey } from '../companion/companionStorage'
import { buildCompanionSessionSummary } from '../companion/companionShared'
import { useCompanionStudyLogSync } from '../companion/useCompanionStudyLogSync'
import { SectionErrorBoundary } from '../../shared/SectionErrorBoundary'
import { buildPracticeRouteSearch } from '../../shared/practiceRoute'
import type { CompanionPlanDetail, CompanionPlanTask, CompanionHistoryItem, CompanionChatReply, CompanionDailyDigest, CompanionAdjustmentPreview, SuggestedAction, InlineTrigger, PendingAction, ConversationState } from '../companion/companionTypes'

/* ========== Public demo model fallback ========== */
const DEMO_MODEL_URL = 'https://cdn.jsdelivr.net/gh/guansss/pixi-live2d-display@v0.4.0/test/assets/haru/haru_greeter_t03.model3.json'

/* ========== Mouth shape sync — drives Live2D mouth params from TTS amplitude ========== */

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

/* ========== Live2D Canvas with full interaction ========== */

function SimpleLive2DCanvas({ modelUrl, backgroundImageUrl, mouthOpen, motions, directive }: { modelUrl: string; backgroundImageUrl?: string; mouthOpen: number; motions: SelectableLive2DModelMotion[]; directive: Live2DDirective | null }) {
  const hostRef = useRef<HTMLDivElement>(null)
  const runtimeRef = useRef<any>(null)
  const [status, setStatus] = useState('等待加载...')
  const [error, setError] = useState('')
  const [bgFailed, setBgFailed] = useState(false)
  const dragRef = useRef<{ pointerId: number; startX: number; startY: number; originX: number; originY: number } | null>(null)
  const [isDragging, setIsDragging] = useState(false)

  // Scale state for wheel zoom — initialized to 1, then recalibrated after model loads
  const scaleRef = useRef(1)
  const targetScaleRef = useRef(1)
  const animFrameRef = useRef<number>(0)
  const EASING = 0.15
  const MIN_SCALE = 0.3

  // 嘴型开合由父组件传入，每帧写入模型参数，避免 React 重渲染参与逐帧更新。
  const mouthOpenRef = useRef(0)
  useEffect(() => {
    mouthOpenRef.current = mouthOpen
  }, [mouthOpen])

  // 记录最近一次播放的动作，用于节流，避免同 key 短时间内重复触发。
  const lastMotionRef = useRef<{ key: string; at: number } | null>(null)

  /**
   * 当后端指令带来新的 motion_key 时，在模型动作清单里查到对应 group 并播放一次。
   * 不做 settings hydrate，仅播放模型 model3.json 自带声明的动作。
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
  const MAX_SCALE = 3.0
  const WHEEL_STEP = 0.02

  useEffect(() => {
    const host = hostRef.current
    if (!host || !modelUrl) return

    let disposed = false
    let resizeObserver: ResizeObserver | null = null

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

        let model
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

        // Use anchor-based positioning (same as live2dStageRuntime.ts)
        model.anchor.set(0.5, 1) // bottom-center anchor

        // Calculate the "natural" fit scale once after model loads
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

        // Initialize scale to the natural fit (scaleRef stays at1 here;
        // we store the baseFit and multiply later)
        const baseFit = getFitScale()

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

        // ResizeObserver for responsive scaling
        resizeObserver = new ResizeObserver(() => {
          if (!host || disposed) return
          app.renderer.resize(host.clientWidth, host.clientHeight)
          // Recalculate baseFit on resize
          // (we update the closure variable via re-assigning a let)
          applyScale()
        })
        resizeObserver.observe(host)

        // Mouse tracking for eye follow (only when not dragging)
        host.addEventListener('mousemove', (e) => {
          if (disposed || dragRef.current) return
          const rect = host.getBoundingClientRect()
          model.focus(e.clientX - rect.left, e.clientY - rect.top)
        })

        host.addEventListener('wheel', (e) => {
          e.preventDefault()
          if (disposed) return
          const direction = e.deltaY > 0 ? -1 : 1
          // Multiplicative scaling relative to current
          targetScaleRef.current = Math.max(
            MIN_SCALE,
            Math.min(MAX_SCALE, targetScaleRef.current * (1 + WHEEL_STEP * direction))
          )
          if (!animFrameRef.current) {
            animFrameRef.current = requestAnimationFrame(animateScale)
          }
        }, { passive: false })

        // Smooth scale animation
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

  // Drag to move
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

  function handlePointerMove(e: React.PointerEvent) {
    if (!dragRef.current || !runtimeRef.current) return
    const model = runtimeRef.current.model
    model.x = dragRef.current.originX + (e.clientX - dragRef.current.startX)
    model.y = dragRef.current.originY + (e.clientY - dragRef.current.startY)
  }

  function handlePointerUp() {
    dragRef.current = null
    setIsDragging(false)
  }

  return (
    <>
      {/* Background image */}
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
      {/* Canvas host */}
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
      {/* Hint */}
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
      {/* Error */}
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
      {/* Loading */}
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

/* ========== Color tokens ========== */

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
  purple: '#7C5CFF',
}

/* ========== Error Boundary ========== */

class ErrorBoundary extends Component<{ children: ReactNode }, { hasError: boolean; error: string }> {
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

/* ========== ChatBubble — uses CompanionHistoryItem ========== */

function ChatBubble({ msg }: { msg: CompanionHistoryItem }) {
  const isAI = msg.role === 'assistant'
  return (
    <div style={{
      display: 'flex', gap: 8, padding: '8px 12px', borderRadius: 6, cursor: 'pointer', transition: 'background 0.2s',
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
        <div style={{ fontSize: 14, fontWeight: 600, color: C.white700, marginBottom: 2 }}>{isAI ? '陪伴助手' : '你'}</div>
        <div style={{ fontSize: 14, color: C.white, lineHeight: 1.5, background: C.gray700, borderRadius: 8, padding: '8px 12px', maxWidth: '90%' }}>{msg.content}</div>
        <div style={{ fontSize: 11, color: C.white500, marginTop: 4 }}>{new Date(msg.createdAt).toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' })}</div>
      </div>
    </div>
  )
}

/* ========== TaskRow — uses CompanionPlanTask ========== */

function TaskRow({ task }: { task: CompanionPlanTask }) {
  const statusIcon: Record<string, JSX.Element> = {
    completed: <span style={{ color: C.green500, fontSize: 14 }}>✓</span>,
    in_progress: <span style={{ color: '#f97316', fontSize: 14 }}>⚡</span>,
    pending: <span style={{ width: 12, height: 12, borderRadius: '50%', border: `2px solid ${C.gray500}`, display: 'inline-block' }} />,
    skipped: <span style={{ color: C.gray500, fontSize: 14 }}>⏭</span>,
  }
  const status = (task.status || 'pending') as string

  return (
    <div style={{
      display: 'flex', alignItems: 'center', gap: 10, padding: '10px 12px',
      borderRadius: 6, transition: 'background 0.15s',
    }}
      onMouseEnter={(e) => { e.currentTarget.style.background = C.white050 }}
      onMouseLeave={(e) => { e.currentTarget.style.background = 'transparent' }}
    >
      <span>{statusIcon[status] || statusIcon.pending}</span>
      <div style={{ flex: 1, minWidth: 0 }}>
        <div style={{
          fontSize: 13, fontWeight: status === 'in_progress' ? 600 : 500,
          color: status === 'completed' ? C.gray500 : C.white,
          textDecoration: status === 'completed' ? 'line-through' : 'none',
          overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap',
        }}>
          {task.title}
        </div>
        {task.phase && <div style={{ fontSize: 11, color: C.gray500, marginTop: 2 }}>{task.phase}</div>}
      </div>
      {status === 'in_progress' && (
        <span style={{ padding: '2px 8px', borderRadius: 4, fontSize: 11, fontWeight: 600, background: 'rgba(249,115,22,0.15)', color: '#f97316' }}>进行中</span>
      )}
    </div>
  )
}

/* ========== Sidebar ========== */

type SidebarPanel = 'model' | 'plan' | 'goals' | 'chat'

function Sidebar({ collapsed, onToggle, plan, planStats, focusedTask, accessToken, todayGoals, activeGoals, currentPlanQuery, modelOptions, selectedModelKey, setSelectedModelKey, modelOptionsQuery, history, onAdjustPlan, adjustingPlan, planActionMessage }: {
  collapsed: boolean
  onToggle: () => void
  plan: CompanionPlanDetail | null
  planStats: { completed_tasks: number; total_tasks: number; progress: number } | null
  focusedTask: CompanionPlanTask | null
  accessToken: string | null
  todayGoals: CompanionPlanTask[]
  activeGoals: CompanionPlanTask[]
  currentPlanQuery: any
  modelOptions: any[]
  selectedModelKey: string
  setSelectedModelKey: (key: string) => void
  modelOptionsQuery: any
  history: CompanionHistoryItem[]
  onAdjustPlan: () => void
  adjustingPlan: boolean
  planActionMessage: string
}) {
  const [activePanel, setActivePanel] = useState<SidebarPanel>('chat')
  const listRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (listRef.current) listRef.current.scrollTop = listRef.current.scrollHeight
  }, [history.length])

  const panelButtons = [
    { key: 'plan' as const, icon: <UsergroupAddOutlined />, label: '计划' },
    { key: 'goals' as const, icon: <HistoryOutlined />, label: '目标' },
    { key: 'chat' as const, icon: <PlusOutlined />, label: '对话' },
    { key: 'model' as const, icon: <SettingOutlined />, label: '模型' },
  ]

  const panelTitles: Record<SidebarPanel, string> = {
    model: '切换模型',
    plan: '计划进度',
    goals: '今日目标',
    chat: '对话历史',
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
          {/* Header buttons */}
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

          {/* Panel title */}
          <div style={{ padding: '0 16px', fontSize: 16, fontWeight: 600, color: '#fff' }}>
            {panelTitles[activePanel]}
          </div>

          {/* Panel content */}
          <div ref={listRef} style={{
            flex: 1, overflowY: 'auto', padding: '0 16px',
            display: 'flex', flexDirection: 'column', gap: 8,
          }}>
            {/* Model panel */}
            {activePanel === 'model' && (
              <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
                <p style={{ fontSize: 13, color: C.gray500, margin: 0 }}>选择要在舞台展示的 Live2D 角色模型</p>
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
                        border: `1px solid ${m.key === selectedModelKey ? '#f97316' : C.white200}`,
                        background: m.key === selectedModelKey ? 'rgba(249,115,22,0.08)' : 'transparent',
                        cursor: 'pointer', transition: 'all 0.15s',
                      }}
                    >
                      <div style={{ fontSize: 14, fontWeight: 600, color: m.key === selectedModelKey ? '#f97316' : C.white }}>
                        {m.name} {m.key === selectedModelKey && <span style={{ fontSize: 11, marginLeft: 8 }}>当前使用</span>}
                      </div>
                      {m.source && <div style={{ fontSize: 12, color: C.gray500, marginTop: 4 }}>{m.source}</div>}
                    </div>
                  ))
                )}
              </div>
            )}

            {/* Plan panel */}
            {activePanel === 'plan' && (
              <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
                <div style={{
                  padding: '16px', borderRadius: 8,
                  border: `1px solid ${C.white200}`, background: 'rgba(0,0,0,0.2)',
                }}>
                  <div style={{ fontSize: 15, fontWeight: 700, color: '#fff', marginBottom: 4 }}>
                    {plan?.status === 'generating' ? '学习计划生成中...' : (plan?.title || '还没有学习计划')}
                  </div>
                  {plan?.phase && (
                    <div style={{ fontSize: 12, color: C.gray500, marginBottom: 12 }}>
                      {plan.phase}阶段
                    </div>
                  )}
                  <div style={{ height: 6, borderRadius: 3, background: C.white100, marginBottom: 8 }}>
                    <div style={{
                      width: `${planStats?.progress ?? plan?.progress ?? 0}%`,
                      height: '100%', borderRadius: 3, background: '#f97316',
                    }} />
                  </div>
                  <div style={{ display: 'flex', gap: 16 }}>
                    {[
                      { label: '已完成', value: planStats?.completed_tasks ?? plan?.completed_tasks ?? 0, color: C.green500 },
                      { label: '进行中', value: activeGoals.length, color: '#f97316' },
                      { label: '待开始', value: (planStats?.total_tasks ?? plan?.total_tasks ?? 0) - (planStats?.completed_tasks ?? 0), color: C.gray500 },
                    ].map((s) => (
                      <div key={s.label} style={{ textAlign: 'center' }}>
                        <div style={{ fontSize: 18, fontWeight: 700, color: s.color }}>{s.value}</div>
                        <div style={{ fontSize: 11, color: C.gray500 }}>{s.label}</div>
                      </div>
                    ))}
                  </div>
                </div>

                {focusedTask && (
                  <div style={{
                    padding: '14px 16px', borderRadius: 8,
                    border: '1px solid #f97316', background: 'rgba(249,115,22,0.06)',
                  }}>
                    <div style={{ fontSize: 11, color: '#f97316', fontWeight: 600, marginBottom: 6 }}>当前续接</div>
                    <div style={{ fontSize: 14, fontWeight: 600, color: '#fff', marginBottom: 8 }}>
                      {focusedTask.title}
                    </div>
                    {focusedTask.phase && (
                      <div style={{ fontSize: 12, color: C.gray500, marginBottom: 4 }}>
                        阶段：{focusedTask.phase}
                      </div>
                    )}
                  </div>
                )}

                {plan && accessToken && (
                  <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
                    <button
                      type="button"
                      onClick={onAdjustPlan}
                      disabled={adjustingPlan}
                      style={{
                        padding: '8px 12px', borderRadius: 8, border: `1px solid ${C.white200}`,
                        background: adjustingPlan ? C.white050 : 'transparent', color: C.white700,
                        fontSize: 13, fontWeight: 600, cursor: adjustingPlan ? 'not-allowed' : 'pointer',
                        transition: 'all 0.15s',
                      }}
                    >
                      {adjustingPlan ? '正在调整计划...' : '重新调整计划'}
                    </button>
                    {planActionMessage && (
                      <p style={{ fontSize: 12, color: C.gray500, margin: 0 }}>{planActionMessage}</p>
                    )}
                  </div>
                )}

                {!plan && accessToken && (
                  <p style={{ fontSize: 13, color: C.gray500, textAlign: 'center', padding: 16 }}>
                    还没有学习计划，可以通过对话让助手帮你创建
                  </p>
                )}

                {!accessToken && (
                  <p style={{ fontSize: 13, color: C.gray500, textAlign: 'center', padding: 16 }}>
                    登录后查看学习计划
                  </p>
                )}
              </div>
            )}

            {/* Goals panel */}
            {activePanel === 'goals' && (
              <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
                <div>
                  <div style={{ fontSize: 12, color: C.gray500, marginBottom: 8, fontWeight: 600 }}>今日目标</div>
                  {todayGoals.length > 0 ? (
                    <div style={{ border: `1px solid ${C.white200}`, borderRadius: 8, overflow: 'hidden' }}>
                      {todayGoals.slice(0, 5).map((task, i) => (
                        <div key={task.id} style={{ borderBottom: i < Math.min(todayGoals.length, 5) - 1 ? `1px solid ${C.white200}` : 'none' }}>
                          <TaskRow task={task} />
                        </div>
                      ))}
                    </div>
                  ) : (
                    <p style={{ fontSize: 13, color: C.gray500, textAlign: 'center', padding: 16 }}>
                      {currentPlanQuery.data ? '今天没有待推进任务' : '还没有学习计划'}
                    </p>
                  )}
                </div>
                <div>
                  <div style={{ fontSize: 12, color: C.gray500, marginBottom: 8, fontWeight: 600 }}>进行目标</div>
                  {activeGoals.length > 0 ? (
                    <div style={{ border: `1px solid ${C.white200}`, borderRadius: 8, overflow: 'hidden' }}>
                      {activeGoals.map((task, i) => (
                        <div key={task.id} style={{ borderBottom: i < activeGoals.length - 1 ? `1px solid ${C.white200}` : 'none' }}>
                          <TaskRow task={task} />
                        </div>
                      ))}
                    </div>
                  ) : (
                    <p style={{ fontSize: 13, color: C.gray500, textAlign: 'center', padding: 16 }}>当前没有进行中的任务</p>
                  )}
                </div>
              </div>
            )}

            {/* Chat panel */}
            {activePanel === 'chat' && (
              <div style={{
                flex: 1, overflowY: 'auto',
                border: `1px solid ${C.white200}`, borderRadius: 8,
                background: 'rgba(0,0,0,0.25)', padding: 16,
                display: 'flex', flexDirection: 'column', gap: 8,
              }}>
                {history.length === 0 ? (
                  <p style={{ fontSize: 13, color: C.gray500, textAlign: 'center', padding: 24 }}>还没有对话记录，发送消息开始聊天</p>
                ) : (
                  history.map((msg) => <ChatBubble key={msg.id} msg={msg} />)
                )}
              </div>
            )}
          </div>
        </>
      )}
    </div>
  )
}

/* ========== Footer ========== */

function actionLabel(action: SuggestedAction): string {
  switch (action.type) {
    case 'practice': return '去刷题'
    case 'interview': return '模拟面试'
    case 'adjust_plan': return '调整计划'
    case 'chat': return action.params || '继续追问'
    default: return action.params || action.target || '继续'
  }
}

function Footer({ collapsed, onToggle, composer, setComposer, sending, onSend, isRecording, isRecognizing, onToggleRecording, suggestedActions, onApplyAction }: {
  collapsed: boolean
  onToggle: () => void
  composer: string
  setComposer: (value: string) => void
  sending: boolean
  onSend: () => void
  isRecording: boolean
  isRecognizing: boolean
  onToggleRecording: () => void
  suggestedActions: SuggestedAction[]
  onApplyAction: (action: SuggestedAction) => void
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
          {suggestedActions.length > 0 && (
            <div style={{ display: 'flex', gap: 6, flexWrap: 'wrap', alignItems: 'center' }}>
              {suggestedActions.slice(0, 4).map((action, index) => (
                <button
                  key={`action-${index}`}
                  type="button"
                  onClick={() => onApplyAction(action)}
                  title={action.target || actionLabel(action)}
                  style={{
                    padding: '4px 10px', borderRadius: 14, border: `1px solid ${C.white200}`,
                    background: C.white050, color: C.white, fontSize: 12, cursor: 'pointer',
                    maxWidth: 220, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap',
                  }}
                >
                  {actionLabel(action)}
                </button>
              ))}
            </div>
          )}
          <div style={{ display: 'flex', gap: 8, alignItems: 'flex-end' }}>
          {/* Mic button */}
          <button
            onClick={onToggleRecording}
            disabled={sending || isRecognizing}
            title={isRecording ? '点击停止录音' : '点击开始语音输入'}
            style={{
              width: 50, height: 50, borderRadius: 12, border: 'none', flexShrink: 0,
              background: isRecording ? '#ef4444' : (isRecognizing ? C.gray600 : '#22c55e'),
              color: '#fff', fontSize: 16, fontWeight: 600,
              cursor: (sending || isRecognizing) ? 'not-allowed' : 'pointer',
              display: 'flex', alignItems: 'center', justifyContent: 'center',
              transition: 'all 0.15s',
            }}
          >
            {isRecognizing ? '...' : (isRecording ? '⏹' : '🎤')}
          </button>

          <div style={{ flex: 1, position: 'relative' }}>
            <textarea
              value={composer}
              onChange={(e) => setComposer(e.target.value)}
              onKeyDown={(e) => { if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); onSend() } }}
              placeholder={isRecording ? '正在录音，再次点击麦克风停止...' : (sending ? '陪伴助手思考中...' : '输入消息，和陪伴助手对话...')}
              style={{
                width: '100%', height: 80, minHeight: 80, maxHeight: 80,
                background: C.gray700, border: isRecording ? '2px solid #ef4444' : 'none',
                borderRadius: 12, color: C.white, fontSize: 18,
                padding: '28px 16px 0 16px', resize: 'none', lineHeight: 1.4, outline: 'none',
              }}
            />
          </div>
          <button
            onClick={onSend}
            disabled={sending || !composer.trim() || isRecording}
            style={{
              width: 50, height: 50, borderRadius: 12, border: 'none', flexShrink: 0,
              background: sending ? C.gray600 : C.blue500,
              color: '#fff', fontSize: 20, cursor: (sending || isRecording) ? 'not-allowed' : 'pointer',
              display: 'flex', alignItems: 'center', justifyContent: 'center',
              opacity: (composer.trim() && !isRecording) ? 1 : 0.5,
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

/**
 * 舞台字幕：直接展示 useLive2DDialoguePlayback 推进的当前文本，
 * 与 TTS 音频进度同步，避免独立计时器与音频脱节。
 */
function TypewriterSubtitle({ text, typing, inlineTriggers, onKeywordClick }: {
  text: string
  typing: boolean
  inlineTriggers: InlineTrigger[]
  onKeywordClick: (trigger: InlineTrigger) => void
}) {
  if (!text && !typing) {
    return null
  }

  // 将 text 按 inlineTriggers 中的 keyword 拆分为可点击/纯文本片段
  const segments = useMemo(() => {
    if (!text || inlineTriggers.length === 0) {
      return [{ text, isKeyword: false, trigger: null as InlineTrigger | null }]
    }

    // 收集所有关键词在 text 中的出现位置（大小写不敏感）
    interface Match {
      start: number
      end: number
      trigger: InlineTrigger
    }
    const matches: Match[] = []
    const lowerText = text.toLowerCase()

    for (const trigger of inlineTriggers) {
      const keyword = (trigger.keyword || '').trim()
      if (!keyword) continue
      const lowerKW = keyword.toLowerCase()
      let searchFrom = 0
      while (searchFrom < lowerText.length) {
        const idx = lowerText.indexOf(lowerKW, searchFrom)
        if (idx < 0) break
        matches.push({ start: idx, end: idx + keyword.length, trigger })
        searchFrom = idx + keyword.length
      }
    }

    if (matches.length === 0) {
      return [{ text, isKeyword: false, trigger: null as InlineTrigger | null }]
    }

    // 排序，处理重叠：长匹配优先；重叠时跳过短的那个
    matches.sort((a, b) => a.start - b.start || b.end - a.end)

    const deduped: Match[] = []
    for (const m of matches) {
      const last = deduped[deduped.length - 1]
      if (last && m.start < last.end) {
        // 重叠 — 保留更长的
        if (m.end - m.start > last.end - last.start) {
          deduped[deduped.length - 1] = m
        }
        continue
      }
      deduped.push(m)
    }

    // 按 start 排序，构建 segments
    deduped.sort((a, b) => a.start - b.start)
    const segs: Array<{ text: string; isKeyword: boolean; trigger: InlineTrigger | null }> = []
    let cursor = 0
    for (const m of deduped) {
      if (m.start > cursor) {
        segs.push({ text: text.slice(cursor, m.start), isKeyword: false, trigger: null })
      }
      segs.push({ text: text.slice(m.start, m.end), isKeyword: true, trigger: m.trigger })
      cursor = m.end
    }
    if (cursor < text.length) {
      segs.push({ text: text.slice(cursor), isKeyword: false, trigger: null })
    }
    return segs
  }, [text, inlineTriggers])

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
          }}>陪伴助手</span>
          {typing && <span style={{ fontSize: 11, color: '#9ca3af' }}>输入中...</span>}
        </div>
        <p style={{ margin: 0, fontSize: 15, color: '#fff', lineHeight: 1.6, minHeight: 24 }}>
          {segments.map((seg, i) =>
            seg.isKeyword && seg.trigger ? (
              <span
                key={`kw-${i}`}
                onClick={() => onKeywordClick(seg.trigger!)}
                title={`点击前往「${seg.trigger.keyword}」相关练习`}
                style={{
                  color: '#60a5fa', cursor: 'pointer', textDecoration: 'underline',
                  textUnderlineOffset: 3, fontWeight: 600, transition: 'color 0.15s',
                }}
                onMouseEnter={(e) => { e.currentTarget.style.color = '#93c5fd' }}
                onMouseLeave={(e) => { e.currentTarget.style.color = '#60a5fa' }}
              >
                {seg.text}
              </span>
            ) : (
              <span key={`t-${i}`}>{seg.text}</span>
            )
          )}
          {typing && <span style={{ opacity: 0.5 }}>|</span>}
        </p>
      </div>
    </div>
  )
}

/* ========== Main Page ========== */

const PROTOTYPE_INITIAL_DIALOGUE = '我是你的学习陪伴助手。先把今天要推进的学习目标摆清楚，我们再一项一项完成。'

export function PrototypeUIPage() {
  const [sidebarCollapsed, setSidebarCollapsed] = useState(false)
  const [footerCollapsed, setFooterCollapsed] = useState(false)

  const accessToken = useAuthStore((s) => s.accessToken)
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const industriesQuery = useFrontendIndustriesQuery()
  const practiceStatsQuery = usePracticeStatsQuery(accessToken)
  const [selectedIndustryCode, setSelectedIndustryCode] = useState(() => readSelectedFrontendIndustryCode() || DEFAULT_FRONTEND_INDUSTRY_CODE)
  const [history, setHistory] = useState<CompanionHistoryItem[]>(() => [
    {
      id: 'assistant-welcome',
      role: 'assistant',
      content: PROTOTYPE_INITIAL_DIALOGUE,
      createdAt: Date.now(),
    },
  ])
  const [composer, setComposer] = useState('')
  const [sending, setSending] = useState(false)
  const [suggestedActions, setSuggestedActions] = useState<SuggestedAction[]>([])
  const [adjustPreview, setAdjustPreview] = useState<CompanionAdjustmentPreview | null>(null)
  const [previewModalOpen, setPreviewModalOpen] = useState(false)
  const [previewLoading, setPreviewLoading] = useState(false)
  const [applyingAdjustPreview, setApplyingAdjustPreview] = useState(false)
  const [planActionMessage, setPlanActionMessage] = useState('')
  const [dailyDigest] = useState<CompanionDailyDigest | null>(() => readCompanionDailyDigest())
  const hasInjectedResumeMessageRef = useRef(false)
  const [selectedModelKey, setSelectedModelKey] = useState(() => readSelectedCompanionModelKey(selectedIndustryCode))
  const [isRecording, setIsRecording] = useState(false)
  const [isRecognizing, setIsRecognizing] = useState(false)

  // 多轮对话状态机 + 字幕行内关键词
  const [conversationState, setConversationState] = useState<ConversationState | null>(null)
  const [pendingAction, setPendingAction] = useState<PendingAction | null>(null)
  const [inlineTriggers, setInlineTriggers] = useState<InlineTrigger[]>([])

  // 统一管理 TTS 播放、字幕打字与嘴型同步，复用 companion/room 同一套播放链路。
  const {
    liveDialogue,
    isDialogueTyping,
    mouthOpen,
    startDialogueTyping,
    stopCurrentPlayback,
    playTTSAudio,
  } = useLive2DDialoguePlayback({
    initialDialogue: PROTOTYPE_INITIAL_DIALOGUE,
    onPlaybackError: (error) => {
      console.warn('陪伴语音播放失败，已回退到文本模式。', error)
    },
  })
  const recordStreamRef = useRef<MediaStream | null>(null)
  const recordAudioContextRef = useRef<AudioContext | null>(null)
  const recordSourceRef = useRef<MediaStreamAudioSourceNode | null>(null)
  const recordProcessorRef = useRef<ScriptProcessorNode | null>(null)
  const recordPCMRef = useRef<number[]>([])

  const selectedIndustry = useMemo(
    () => resolvePreferredFrontendIndustry(industriesQuery.data || [], selectedIndustryCode),
    [industriesQuery.data, selectedIndustryCode],
  )
  const effectiveIndustryCode = selectedIndustry?.code || selectedIndustryCode.trim() || DEFAULT_FRONTEND_INDUSTRY_CODE

  // 行业与模型偏好本地持久化：刷新或返回后能恢复上次选择。
  useEffect(() => {
    if (effectiveIndustryCode) {
      persistSelectedFrontendIndustryCode(effectiveIndustryCode)
    }
  }, [effectiveIndustryCode])

  useEffect(() => {
    if (selectedModelKey) {
      persistSelectedCompanionModelKey(effectiveIndustryCode, selectedModelKey)
    }
  }, [selectedModelKey, effectiveIndustryCode])

  // Plan queries
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

  // Model query
  const modelOptionsQuery = useQuery({
    queryKey: buildCompanionLive2DModelsQueryKey(effectiveIndustryCode),
    queryFn: () => fetchSelectableCompanionLive2DModels(effectiveIndustryCode),
    staleTime: 60 * 1000,
  })

  const modelOptions = (modelOptionsQuery.data || []).filter(Boolean)
  const currentModel = useMemo(() => {
    if (!modelOptions.length) return null
    return modelOptions.find((m) => m.key === selectedModelKey)
      || modelOptions.find((m) => m.is_recommended)
      || modelOptions[0]
      || null
  }, [modelOptions, selectedModelKey])

  // Derived plan data
  const planStats = planProgressQuery.data
  const isPlanGenerating = currentPlanQuery.data?.status === 'generating'
  const todayGoals = useMemo(() => deriveTodayGoals(currentPlanQuery.data || null), [currentPlanQuery.data])
  const activeGoals = useMemo(() => deriveActiveGoals(currentPlanQuery.data || null), [currentPlanQuery.data])
  const focusedTask = useMemo(
    () => resolveFocusedCompanionTask(currentPlanQuery.data || null, readCompanionFocusTask()),
    [currentPlanQuery.data],
  )
  const latestDirective = useMemo<Live2DDirective | null>(
    () => [...history].reverse().find((item) => item.role === 'assistant')?.live2dDirective || null,
    [history],
  )
  const adjustingPlan = previewLoading || applyingAdjustPreview

  // 每日执行摘要与学习日志同步：房内 digest 为只读快照，更新交由主站刷题页回写。
  useCompanionStudyLogSync(accessToken, currentPlanQuery.data || null, dailyDigest, focusedTask)

  // 进入房间且计划就绪后，注入一条续接提示，避免每次都像从零开始。
  useEffect(() => {
    if (hasInjectedResumeMessageRef.current || !accessToken || !currentPlanQuery.data || currentPlanQuery.data.status === 'generating') {
      return
    }
    hasInjectedResumeMessageRef.current = true
    const resumeMessage = buildCompanionWorkspaceResumeMessage(currentPlanQuery.data, focusedTask, dailyDigest)
    setHistory((prev) => [...prev, {
      id: `assistant-resume-${currentPlanQuery.data!.id}`,
      role: 'assistant',
      content: resumeMessage,
      createdAt: Date.now(),
    }])
  }, [accessToken, currentPlanQuery.data, dailyDigest, focusedTask])

  // 将当前会话摘要持续写回入口页可读的本地缓存。
  useEffect(() => {
    const summary = buildCompanionSessionSummary(history, currentPlanQuery.data || null)
    if (summary) {
      persistCompanionSessionSummary(summary)
    }
  }, [history, currentPlanQuery.data])

  // 当 pendingAction.ready 变为 true 时自动触发对应动作。
  // 仅 adjust_plan 自动触发（handleAdjustPlan 会弹出预览 Modal 让用户确认）；
  // practice / interview 必须由用户主动点击关键词或 chip 触发，不能静默跳转。
  useEffect(() => {
    if (!pendingAction?.ready) return
    if (pendingAction.type === 'adjust_plan') {
      void handleAdjustPlan()
    }
    // 无论什么类型，执行一次后立即清除，防止重复触发
    setPendingAction(null)
  }, [pendingAction])

  // 应用结构化引导动作：按 type 跳转刷题/面试页，或触发计划调整，或填入输入框。
  function handleApplyAction(action: SuggestedAction) {
    switch (action.type) {
      case 'practice': {
        const target = (action.target || '').trim()
        if (target) {
          // 有明确 target 时带参数跳转（如关键词），但 target 可能是不存在的题单名，
          // 题库页会自行降级展示
          navigate({
            to: '/practice',
            search: buildPracticeRouteSearch({
              keyword: target,
              source: 'companion_suggested_action',
              title: target,
            }),
          })
        } else {
          // target 为空时直接去题库首页，用户可以浏览所有可用题单
          navigate({ to: '/practice' })
        }
        return
      }
      case 'interview':
        navigate({ to: '/interview' })
        return
      case 'adjust_plan':
        void handleAdjustPlan()
        return
      case 'chat':
      default:
        setComposer(action.params || action.target || '')
    }
  }

  // 字幕关键词点击：弹出确认弹窗，确认后跳转刷题页
  function handleKeywordClick(trigger: InlineTrigger) {
    const keyword = trigger.keyword || ''
    const target = (trigger.target || '').trim()
    Modal.confirm({
      title: '确认前往刷题',
      icon: null,
      content: `确定要前往「${keyword}」相关的练习页面吗？`,
      okText: '去刷题',
      cancelText: '再想想',
      onOk: () => {
        // 用 keyword 作为搜索关键词，不依赖可能不存在的题单 target
        navigate({
          to: '/practice',
          search: buildPracticeRouteSearch({
            keyword: keyword || target,
            source: 'companion_inline_trigger',
            title: keyword,
          }),
        })
      },
    })
  }

  // 真实聊天发送：组装用户消息，调用陪伴 API，追加助手回复
  async function handleSend() {
    if (!composer.trim() || sending) return
    if (isPlanGenerating) return
    if (!accessToken) {
      requestLoginPrompt('/companion', 'missing')
      return
    }

    const userMessage: CompanionHistoryItem = {
      id: `user-${Date.now()}`,
      role: 'user',
      content: composer.trim(),
      createdAt: Date.now(),
    }

    stopCurrentPlayback()
    setHistory((prev) => [...prev, userMessage])
    setComposer('')
    setSuggestedActions([])
    setInlineTriggers([])
    setPendingAction(null)
    setSending(true)

    try {
      const reply: CompanionChatReply = await sendCompanionChatRequest(
        accessToken,
        [...history, userMessage],
        currentPlanQuery.data || null,
        focusedTask,
        dailyDigest,
        selectedModelKey,
        { deriveTodayGoals, deriveActiveGoals },
        conversationState,
      )

      const assistantMessage: CompanionHistoryItem = {
        id: `assistant-${Date.now()}`,
        role: 'assistant',
        content: reply.content || reply.reply || '（陪伴助手暂时没有回复）',
        emotion: reply.emotion || reply.mood,
        action: reply.action,
        live2dDirective: reply.live2d_directive || null,
        createdAt: Date.now(),
      }

      setHistory((prev) => [...prev, assistantMessage])
      setSuggestedActions(reply.suggested_actions || [])
      setInlineTriggers(reply.inline_triggers || [])
      setConversationState(reply.conversation_state || null)
      setPendingAction(reply.pending_action || null)

      // 有 TTS 音频则播放并驱动嘴型/字幕；否则回退到纯打字机字幕。
      if (reply.audio_url) {
        void playTTSAudio(reply.audio_url, assistantMessage.content)
      } else {
        startDialogueTyping(assistantMessage.content)
      }
    } catch (error) {
      const errorMessage: CompanionHistoryItem = {
        id: `error-${Date.now()}`,
        role: 'assistant',
        content: `发送失败：${error instanceof Error ? error.message : '未知错误'}`,
        createdAt: Date.now(),
      }
      setHistory((prev) => [...prev, errorMessage])
      startDialogueTyping(errorMessage.content)
    } finally {
      setSending(false)
    }
  }

  /**
   * 先生成计划调整预览，再由用户确认是否应用，避免一键落库后难以感知差异。
   */
  async function handleAdjustPlan() {
    if (!accessToken) {
      requestLoginPrompt('/companion', 'missing')
      return
    }

    const planId = currentPlanQuery.data?.id
    if (!planId) {
      setPlanActionMessage('还没有学习计划，无法调整。')
      return
    }
    if (currentPlanQuery.data?.status === 'generating') {
      setPlanActionMessage('当前计划仍在生成中，待落库后再调整。')
      return
    }

    setAdjustPreview(null)
    setPreviewModalOpen(false)
    setPreviewLoading(true)
    setPlanActionMessage('陪伴助手正在生成调整预览...')
    try {
      const preview = await previewCompanionPlanAdjustment(accessToken, planId)
      setAdjustPreview(preview)
      setPreviewModalOpen(true)
      setPlanActionMessage(preview.adjustment_summary || '已生成调整预览，请确认后应用。')
    } catch (error) {
      setPlanActionMessage(`调整失败：${error instanceof Error ? error.message : '未知错误'}`)
    } finally {
      setPreviewLoading(false)
    }
  }

  /**
   * 应用已经确认过的计划调整预览，成功后刷新当前计划详情。
   */
  async function handleApplyAdjustPreview() {
    if (!accessToken || !adjustPreview) {
      return
    }

    const planId = currentPlanQuery.data?.id
    if (!planId) {
      setPlanActionMessage('当前没有可应用的学习计划。')
      return
    }

    setApplyingAdjustPreview(true)
    setPlanActionMessage('正在应用计划调整...')
    try {
      await applyCompanionPlanAdjustment(accessToken, planId, adjustPreview.preview_token)
      await invalidateCompanionPlanQueries(queryClient)
      setPreviewModalOpen(false)
      setAdjustPreview(null)
      setPlanActionMessage('计划已重新调整。')
    } catch (error) {
      setPlanActionMessage(`应用调整失败：${error instanceof Error ? error.message : '未知错误'}`)
    } finally {
      setApplyingAdjustPreview(false)
    }
  }

  /**
   * 切换录音状态，录音结束后调用 ASR 接口识别语音。
   */
  async function handleToggleRecording() {
    if (!accessToken) {
      requestLoginPrompt('/companion', 'missing')
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

      if (recordPCMRef.current.length === 0) {
        return
      }

      const pcmInt16 = new Int16Array(recordPCMRef.current)
      const audioBlob = new Blob([pcmInt16], { type: 'audio/pcm' })
      recordPCMRef.current = []

      setIsRecognizing(true)
      try {
        const result = await recognizeSpeech(accessToken, audioBlob, 'pcm', 16000, 'zh-CN')
        if (result.text) {
          setComposer(result.text)
        }
      } catch (error) {
        console.error('语音识别失败:', error)
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
    } catch (error) {
      console.error('麦克风权限获取失败:', error)
    }
  }

  return (
    <>
      <div style={{
        width: '100vw', height: '100vh', overflow: 'hidden',
        background: C.gray900, color: '#fff', display: 'flex', flexDirection: 'row',
      }}>
        <SectionErrorBoundary
          title="侧栏区域加载失败"
          description="计划进度、目标或对话记录在渲染时出现异常。你可以重试当前区域，右侧舞台仍可继续使用。"
          resetKeys={[currentPlanQuery.data?.id, history.length, focusedTask?.id, accessToken]}
        >
        <Sidebar
          collapsed={sidebarCollapsed}
          onToggle={() => setSidebarCollapsed(!sidebarCollapsed)}
          plan={currentPlanQuery.data || null}
          planStats={planStats ? {
            completed_tasks: planStats.completed_tasks,
            total_tasks: planStats.total_tasks,
            progress: planStats.progress,
          } : null}
          focusedTask={focusedTask}
          accessToken={accessToken}
          todayGoals={todayGoals}
          activeGoals={activeGoals}
          currentPlanQuery={currentPlanQuery}
          modelOptions={modelOptions}
          selectedModelKey={selectedModelKey}
          setSelectedModelKey={setSelectedModelKey}
          modelOptionsQuery={modelOptionsQuery}
          history={history}
          onAdjustPlan={() => void handleAdjustPlan()}
          adjustingPlan={adjustingPlan}
          planActionMessage={planActionMessage}
        />
        </SectionErrorBoundary>

        <div style={{
          flex: 1, height: '100%', position: 'relative',
          display: 'flex', flexDirection: 'column',
          transition: 'all 0.3s cubic-bezier(0.4, 0, 0.2, 1)', overflow: 'hidden',
        }}>
          <div style={{ flex: 1, position: 'relative', overflow: 'hidden' }}>
            <ErrorBoundary>
              {/* 等待模型列表查询落定后再挂载 canvas，避免先用兜底 DEMO 模型渲染 -> 查询返回后切换 -> 用户看到闪烁 */}
              {modelOptionsQuery.isLoading ? (
                <div style={{
                  position: 'absolute', inset: 0, zIndex: 1,
                  display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', gap: 12,
                  background: 'radial-gradient(ellipse at 50% 60%, #1e3a5f 0%, #0f1b2d 70%)',
                }}>
                  <Spin size="large" />
                  <p style={{ color: C.white500, fontSize: 14, margin: 0 }}>正在加载模型列表...</p>
                </div>
              ) : (
                <SimpleLive2DCanvas
                  modelUrl={currentModel?.model_url || DEMO_MODEL_URL}
                  backgroundImageUrl={currentModel ? resolveSelectableLive2DBackgroundImageUrl(currentModel) : undefined}
                  mouthOpen={mouthOpen}
                  motions={currentModel?.motions || []}
                  directive={latestDirective}
                />
              )}
            </ErrorBoundary>

            {/* Back button */}
            <div style={{ position: 'absolute', top: 20, left: 20, zIndex: 10 }}>
              <Link
                to="/companion"
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
                <ArrowLeftOutlined /> 返回陪伴入口
              </Link>
            </div>

            {/* Stats badge */}
            <div style={{ position: 'absolute', top: 20, left: 180, zIndex: 10 }}>
              <div style={{
                padding: '8px 16px', borderRadius: 20, fontSize: 13, fontWeight: 500,
                color: '#fff', background: 'rgba(0,0,0,0.4)', backdropFilter: 'blur(8px)',
                display: 'flex', alignItems: 'center', gap: 12,
              }}>
                <span>🔥 连续 {practiceStatsQuery.data?.streak_days ?? 0} 天</span>
                <span>📊 进度 {Math.round(currentPlanQuery.data?.progress ?? 0)}%</span>
              </div>
            </div>

            {/* Dialogue subtitle — text advanced by useLive2DDialoguePlayback, synced with TTS audio */}
            <TypewriterSubtitle text={liveDialogue} typing={isDialogueTyping} inlineTriggers={inlineTriggers} onKeywordClick={handleKeywordClick} />
          </div>

          <div style={{
            width: '100%', height: footerCollapsed ? 24 : (suggestedActions.length > 0 ? 168 : 135),
            transition: 'all 0.3s cubic-bezier(0.4, 0, 0.2, 1)',
            willChange: 'transform', position: 'relative', zIndex: 1,
          }}>
            <Footer
              collapsed={footerCollapsed}
              onToggle={() => setFooterCollapsed(!footerCollapsed)}
              composer={composer}
              setComposer={setComposer}
              sending={sending}
              onSend={handleSend}
              isRecording={isRecording}
              isRecognizing={isRecognizing}
              onToggleRecording={() => void handleToggleRecording()}
              suggestedActions={suggestedActions}
              onApplyAction={handleApplyAction}
            />
          </div>
        </div>
      </div>

      <Modal
        open={previewModalOpen}
        title="计划调整预览"
        onCancel={() => {
          if (applyingAdjustPreview) return
          setPreviewModalOpen(false)
          setAdjustPreview(null)
        }}
        onOk={() => void handleApplyAdjustPreview()}
        okText="确认应用"
        cancelText="取消"
        confirmLoading={applyingAdjustPreview}
        okButtonProps={{ disabled: !adjustPreview }}
        width={760}
        maskClosable={!applyingAdjustPreview}
      >
        <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
          <div style={{ padding: 12, borderRadius: 10, background: '#f5f7fb' }}>
            <div style={{ fontSize: 13, color: '#334155', marginBottom: 8 }}>调整摘要</div>
            <div style={{ color: '#0f172a', lineHeight: 1.7 }}>{adjustPreview?.adjustment_summary || '暂无预览摘要'}</div>
          </div>

          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3, minmax(0, 1fr))', gap: 12 }}>
            <div style={{ padding: 12, borderRadius: 10, background: '#ecfeff', color: '#155e75' }}>新增任务 {adjustPreview?.tasks_added ?? 0}</div>
            <div style={{ padding: 12, borderRadius: 10, background: '#fff7ed', color: '#9a3412' }}>删除任务 {adjustPreview?.tasks_removed ?? 0}</div>
            <div style={{ padding: 12, borderRadius: 10, background: '#eef2ff', color: '#3730a3' }}>重排任务 {adjustPreview?.tasks_reordered ?? 0}</div>
          </div>

          {adjustPreview?.add.length ? (
            <div>
              <div style={{ fontWeight: 600, color: '#0f172a', marginBottom: 8 }}>新增任务</div>
              <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
                {adjustPreview.add.map((task) => (
                  <div key={`add-${task.title}-${task.sort_order}`} style={{ padding: 12, borderRadius: 10, background: '#f8fafc', border: '1px solid #e2e8f0' }}>
                    <div style={{ fontWeight: 600, color: '#0f172a' }}>{task.sort_order}. {task.title}</div>
                    <div style={{ fontSize: 12, color: '#475569', marginTop: 4 }}>阶段：{task.phase || '未分配'} · 时长：{task.duration_minutes || 0} 分钟</div>
                    {task.reason ? <div style={{ fontSize: 12, color: '#64748b', marginTop: 6 }}>{task.reason}</div> : null}
                  </div>
                ))}
              </div>
            </div>
          ) : null}

          {adjustPreview?.remove.length ? (
            <div>
              <div style={{ fontWeight: 600, color: '#0f172a', marginBottom: 8 }}>删除任务</div>
              <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
                {adjustPreview.remove.map((task) => (
                  <div key={`remove-${task.task_id}`} style={{ padding: 12, borderRadius: 10, background: '#fff7ed', border: '1px solid #fed7aa', color: '#9a3412' }}>
                    {task.sort_order}. {task.title} · {task.phase || '未分配'}
                  </div>
                ))}
              </div>
            </div>
          ) : null}

          {adjustPreview?.reorder.length ? (
            <div>
              <div style={{ fontWeight: 600, color: '#0f172a', marginBottom: 8 }}>重排任务</div>
              <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
                {adjustPreview.reorder.map((task) => (
                  <div key={`reorder-${task.task_id}`} style={{ padding: 12, borderRadius: 10, background: '#eef2ff', border: '1px solid #c7d2fe', color: '#3730a3' }}>
                    {task.title} · {task.phase || '未分配'} · {task.from_sort_order} → {task.to_sort_order}
                  </div>
                ))}
              </div>
            </div>
          ) : null}

          {adjustPreview?.preview_tasks.length ? (
            <div>
              <div style={{ fontWeight: 600, color: '#0f172a', marginBottom: 8 }}>应用后任务顺序预览</div>
              <div style={{ maxHeight: 220, overflowY: 'auto', display: 'flex', flexDirection: 'column', gap: 8 }}>
                {adjustPreview.preview_tasks.map((task) => (
                  <div key={`preview-${task.task_id}-${task.sort_order}-${task.title}`} style={{ padding: 12, borderRadius: 10, background: task.is_new ? '#ecfeff' : '#f8fafc', border: `1px solid ${task.is_new ? '#a5f3fc' : '#e2e8f0'}` }}>
                    <div style={{ fontWeight: 600, color: '#0f172a' }}>{task.sort_order}. {task.title}</div>
                    <div style={{ fontSize: 12, color: '#475569', marginTop: 4 }}>状态：{task.status || 'pending'} · 阶段：{task.phase || '未分配'} · 时长：{task.duration_minutes || 0} 分钟</div>
                  </div>
                ))}
              </div>
            </div>
          ) : null}
        </div>
      </Modal>
    </>
  )
}
