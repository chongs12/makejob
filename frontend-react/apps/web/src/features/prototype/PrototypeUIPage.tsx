/**
 * PROTOTYPE — throwaway UI exploration
 * Question: "CompanionWorkspace 重构后应该长什么样？"
 * Faithful reproduction of Open-LLM-VTuber-Web visual design
 * Live2D: directly uses live2dStageRuntime, no preset/expression logic
 * Delete this file and the /prototype-ui route when done.
 */
import { useState, useRef, useEffect, useMemo, Component, type ReactNode } from 'react'
import { Link } from '@tanstack/react-router'
import { Spin } from 'antd'
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
import { resolveSelectableLive2DBackgroundImageUrl } from '../../shared/live2dModelCatalog'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useAuthStore } from '../../state/auth'
import { useFrontendIndustriesQuery, usePracticeStatsQuery } from '../../shared/frontendQueries'
import { DEFAULT_FRONTEND_INDUSTRY_CODE, readSelectedFrontendIndustryCode, resolvePreferredFrontendIndustry } from '../../shared/industryContext'
import { requestLoginPrompt } from '../../shared/loginPrompt'
import { fetchCurrentPlan, fetchCompanionPlanProgress, fetchSelectableCompanionLive2DModels, sendCompanionChatRequest, recognizeSpeech } from '../companion/companionApi'
import { buildCompanionCurrentPlanQueryKey, buildCompanionPlanProgressQueryKey, buildCompanionLive2DModelsQueryKey } from '../../shared/queryKeys'
import { deriveTodayGoals, deriveActiveGoals, resolveFocusedCompanionTask } from '../companion/companionHelpers'
import { readCompanionFocusTask } from '../companion/companionStorage'
import type { CompanionPlanDetail, CompanionPlanTask, CompanionHistoryItem, CompanionChatReply } from '../companion/companionTypes'

/* ========== Public demo model fallback ========== */
const DEMO_MODEL_URL = 'https://cdn.jsdelivr.net/gh/guansss/pixi-live2d-display@v0.4.0/test/assets/haru/haru_greeter_t03.model3.json'

/* ========== Live2D Canvas with full interaction ========== */

function SimpleLive2DCanvas({ modelUrl, backgroundImageUrl }: { modelUrl: string; backgroundImageUrl?: string }) {
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
        const { PIXI, Live2DModel } = await loadLive2DRuntime()
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

        runtimeRef.current = { app, model }
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

function Sidebar({ collapsed, onToggle, plan, planStats, focusedTask, accessToken, todayGoals, activeGoals, currentPlanQuery, modelOptions, selectedModelKey, setSelectedModelKey, modelOptionsQuery, history }: {
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
                    {plan?.title || '还没有学习计划'}
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

function Footer({ collapsed, onToggle, composer, setComposer, sending, onSend, isRecording, isRecognizing, onToggleRecording }: {
  collapsed: boolean
  onToggle: () => void
  composer: string
  setComposer: (value: string) => void
  sending: boolean
  onSend: () => void
  isRecording: boolean
  isRecognizing: boolean
  onToggleRecording: () => void
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
        <div style={{ display: 'flex', gap: 8, padding: '0 16px', alignItems: 'flex-end', height: 'calc(100% - 24px)' }}>
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
      )}
    </div>
  )
}

/* ========== Typewriter Subtitle ========== */

function TypewriterSubtitle({ text, sending }: { text: string; sending: boolean }) {
  const [displayed, setDisplayed] = useState('')
  const [done, setDone] = useState(false)

  useEffect(() => {
    setDisplayed('')
    setDone(false)
    let i = 0
    const timer = setInterval(() => {
      if (i < text.length) {
        setDisplayed(text.slice(0, i + 1))
        i++
      } else {
        setDone(true)
        clearInterval(timer)
      }
    }, 30)
    return () => clearInterval(timer)
  }, [text])

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
          {!done && <span style={{ fontSize: 11, color: '#9ca3af' }}>输入中...</span>}
        </div>
        <p style={{ margin: 0, fontSize: 15, color: '#fff', lineHeight: 1.6, minHeight: 24 }}>
          {displayed}
          {!done && <span style={{ opacity: 0.5 }}>|</span>}
        </p>
      </div>
    </div>
  )
}

/* ========== Main Page ========== */

export function PrototypeUIPage() {
  const [sidebarCollapsed, setSidebarCollapsed] = useState(false)
  const [footerCollapsed, setFooterCollapsed] = useState(false)

  const accessToken = useAuthStore((s) => s.accessToken)
  const queryClient = useQueryClient()
  const industriesQuery = useFrontendIndustriesQuery()
  const practiceStatsQuery = usePracticeStatsQuery(accessToken)
  const [selectedIndustryCode, setSelectedIndustryCode] = useState(() => readSelectedFrontendIndustryCode() || DEFAULT_FRONTEND_INDUSTRY_CODE)
  const [history, setHistory] = useState<CompanionHistoryItem[]>([])
  const [composer, setComposer] = useState('')
  const [sending, setSending] = useState(false)
  const [selectedModelKey, setSelectedModelKey] = useState('')
  const [isRecording, setIsRecording] = useState(false)
  const [isRecognizing, setIsRecognizing] = useState(false)
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
  const todayGoals = useMemo(() => deriveTodayGoals(currentPlanQuery.data || null), [currentPlanQuery.data])
  const activeGoals = useMemo(() => deriveActiveGoals(currentPlanQuery.data || null), [currentPlanQuery.data])
  const focusedTask = useMemo(
    () => resolveFocusedCompanionTask(currentPlanQuery.data || null, readCompanionFocusTask()),
    [currentPlanQuery.data],
  )

  // 真实聊天发送：组装用户消息，调用陪伴 API，追加助手回复
  async function handleSend() {
    if (!composer.trim() || sending) return
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

    setHistory((prev) => [...prev, userMessage])
    setComposer('')
    setSending(true)

    try {
      const reply: CompanionChatReply = await sendCompanionChatRequest(
        accessToken,
        [...history, userMessage],
        currentPlanQuery.data || null,
        focusedTask,
        null,
        selectedModelKey,
        { deriveTodayGoals, deriveActiveGoals },
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
    } catch (error) {
      const errorMessage: CompanionHistoryItem = {
        id: `error-${Date.now()}`,
        role: 'assistant',
        content: `发送失败：${error instanceof Error ? error.message : '未知错误'}`,
        createdAt: Date.now(),
      }
      setHistory((prev) => [...prev, errorMessage])
    } finally {
      setSending(false)
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
    <div style={{
      width: '100vw', height: '100vh', overflow: 'hidden',
      background: C.gray900, color: '#fff', display: 'flex', flexDirection: 'row',
    }}>
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
      />

      <div style={{
        flex: 1, height: '100%', position: 'relative',
        display: 'flex', flexDirection: 'column',
        transition: 'all 0.3s cubic-bezier(0.4, 0, 0.2, 1)', overflow: 'hidden',
      }}>
        <div style={{ flex: 1, position: 'relative', overflow: 'hidden' }}>
          <ErrorBoundary>
            <SimpleLive2DCanvas
              modelUrl={currentModel?.model_url || DEMO_MODEL_URL}
              backgroundImageUrl={currentModel ? resolveSelectableLive2DBackgroundImageUrl(currentModel) : undefined}
            />
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

          {/* Dialogue subtitle with typewriter effect */}
          {history.length > 0 && (() => {
            const lastAssistantMsg = [...history].reverse().find((m) => m.role === 'assistant')
            if (!lastAssistantMsg) return null
            return (
              <TypewriterSubtitle
                key={lastAssistantMsg.id}
                text={lastAssistantMsg.content}
                sending={sending}
              />
            )
          })()}
        </div>

        <div style={{
          width: '100%', height: footerCollapsed ? 24 : 135,
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
          />
        </div>
      </div>
    </div>
  )
}
