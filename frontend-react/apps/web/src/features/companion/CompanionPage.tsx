import type { FormEvent } from 'react'
import { useEffect, useMemo, useRef, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import type { Application } from 'pixi.js'
import type { Live2DModel as Cubism4Live2DModel } from 'pixi-live2d-display/cubism4'
import { requestJson, extractErrorMessage } from '@makejob/api-client'
import { isSuccessCode, type ApiEnvelope } from '@makejob/shared-types'
import { useAuthStore } from '../../state/auth'

const ARIU_MODEL_URL = '/live2d-assets/ariu/ariu.model3.json'
const MAX_CHAT_HISTORY = 12

type CompanionMessageRole = 'assistant' | 'user'

interface CompanionPlanTask {
  id: number
  title: string
  description: string
  task_type: string
  status: string
  due_date?: string
  day_number: number
}

interface CompanionPlanDetail {
  id: number
  title: string
  description: string
  progress: number
  tasks: CompanionPlanTask[]
}

interface CompanionHistoryItem {
  id: string
  role: CompanionMessageRole
  content: string
  emotion?: string
  action?: string
  createdAt: number
}

interface CompanionChatReply {
  content?: string
  reply?: string
  emotion?: string
  mood?: string
  action?: string
}

declare global {
  interface Window {
    PIXI?: typeof import('pixi.js')
    Live2DCubismCore?: unknown
  }
}

let cubismCoreScriptPromise: Promise<void> | null = null

/**
 * 创建陪伴页首屏默认消息，保证页面在未登录时也能展示完整骨架。
 */
function buildInitialHistory(): CompanionHistoryItem[] {
  return [
    {
      id: 'assistant-welcome',
      role: 'assistant',
      content: '我是 Ariu。先把今天要推进的学习目标摆清楚，我们再一项一项完成。',
      createdAt: Date.now(),
    },
  ]
}

/**
 * 拉取当前用户的进行中学习计划，并为左侧目标卡片提供数据。
 */
async function fetchCurrentPlan(token: string): Promise<CompanionPlanDetail | null> {
  const response = await requestJson<ApiEnvelope<CompanionPlanDetail>>('/plans/current', {
    token,
  })

  if (!isSuccessCode(response.code)) {
    if (response.code === 404) {
      return null
    }
    throw new Error(response.message || '获取当前学习计划失败')
  }

  return response.data || null
}

/**
 * 将当前对话历史压缩成后端陪伴接口可消费的消息列表，避免单次请求过大。
 */
function buildChatPayload(history: CompanionHistoryItem[]) {
  return history
    .filter((item) => item.role === 'assistant' || item.role === 'user')
    .slice(-MAX_CHAT_HISTORY)
    .map((item) => ({
      role: item.role,
      content: item.content,
    }))
}

/**
 * 向后端陪伴接口发送消息，并返回 Ariu 的最新回复内容。
 */
async function sendCompanionChatRequest(
  token: string,
  history: CompanionHistoryItem[],
  plan: CompanionPlanDetail | null,
): Promise<CompanionChatReply> {
  const response = await requestJson<ApiEnvelope<CompanionChatReply>>('/companion/chat', {
    method: 'POST',
    token,
    body: {
      messages: buildChatPayload(history),
      context: {
        current_plan_title: plan?.title || '',
        current_plan_progress: plan?.progress || 0,
        today_goals: deriveTodayGoals(plan).map((item) => item.title),
        active_goals: deriveActiveGoals(plan).map((item) => item.title),
      },
    },
  })

  if (!isSuccessCode(response.code) || !response.data) {
    throw new Error(response.message || 'Ariu 暂时没有回复')
  }

  return response.data
}

/**
 * 判断任务日期是否与当前本地日期处于同一天，用于筛选今日目标。
 */
function isSameLocalDay(value?: string): boolean {
  if (!value) {
    return false
  }

  const targetDate = new Date(value)
  if (Number.isNaN(targetDate.getTime())) {
    return false
  }

  const now = new Date()
  return (
    targetDate.getFullYear() === now.getFullYear()
    && targetDate.getMonth() === now.getMonth()
    && targetDate.getDate() === now.getDate()
  )
}

/**
 * 从当前计划中提炼“今日目标”，优先展示今天到期的任务，再回退到最靠前的未完成任务。
 */
function deriveTodayGoals(plan: CompanionPlanDetail | null): CompanionPlanTask[] {
  if (!plan?.tasks?.length) {
    return []
  }

  const todayTasks = plan.tasks.filter((item) => isSameLocalDay(item.due_date))
  if (todayTasks.length > 0) {
    return todayTasks.slice(0, 3)
  }

  return plan.tasks.filter((item) => item.status !== 'completed').slice(0, 3)
}

/**
 * 从当前计划中提炼“进行中目标”，优先显示进行中任务，再回退到首个未完成任务。
 */
function deriveActiveGoals(plan: CompanionPlanDetail | null): CompanionPlanTask[] {
  if (!plan?.tasks?.length) {
    return []
  }

  const inProgressTasks = plan.tasks.filter((item) => item.status === 'in_progress')
  if (inProgressTasks.length > 0) {
    return inProgressTasks.slice(0, 2)
  }

  const pendingTask = plan.tasks.find((item) => item.status !== 'completed')
  return pendingTask ? [pendingTask] : []
}

/**
 * 将任务状态转换成更适合前台阅读的中文文案。
 */
function taskStatusLabel(status: string): string {
  const labelMap: Record<string, string> = {
    pending: '待开始',
    in_progress: '进行中',
    completed: '已完成',
    skipped: '已跳过',
  }

  return labelMap[status] || '未定义'
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
 * 动态加载 Cubism Core 脚本，确保浏览器端具备解析 Ariu 模型的能力。
 */
function ensureCubismCoreScript(): Promise<void> {
  if (typeof window === 'undefined') {
    return Promise.resolve()
  }

  if (window.Live2DCubismCore) {
    return Promise.resolve()
  }

  if (cubismCoreScriptPromise) {
    return cubismCoreScriptPromise
  }

  cubismCoreScriptPromise = new Promise<void>((resolve, reject) => {
    const existingScript = document.querySelector<HTMLScriptElement>('script[data-live2d-cubism-core="true"]')
    if (existingScript) {
      if (window.Live2DCubismCore) {
        resolve()
        return
      }
      existingScript.addEventListener('load', () => resolve(), { once: true })
      existingScript.addEventListener('error', () => reject(new Error('Cubism Core 脚本加载失败')), { once: true })
      return
    }

    const script = document.createElement('script')
    script.src = '/live2d-assets/live2dcubismcore.min.js'
    script.async = true
    script.dataset.live2dCubismCore = 'true'
    script.onload = () => {
      if (!window.Live2DCubismCore) {
        reject(new Error('Cubism Core 已返回，但页面中未挂载 Live2DCubismCore'))
        return
      }
      resolve()
    }
    script.onerror = () => reject(new Error('Cubism Core 脚本加载失败'))
    document.head.appendChild(script)
  })

  return cubismCoreScriptPromise
}

/**
 * 按舞台容器大小重新计算 Ariu 的站位和缩放，让不同屏幕下都保持 galgame 式构图。
 */
function layoutAriuModel(model: Cubism4Live2DModel, host: HTMLDivElement, baseWidth: number, baseHeight: number): void {
  const safeBaseWidth = Math.max(baseWidth, 1)
  const safeBaseHeight = Math.max(baseHeight, 1)
  const widthScale = (host.clientWidth * 0.74) / safeBaseWidth
  const heightScale = (host.clientHeight * 0.92) / safeBaseHeight
  const scale = Math.max(Math.min(widthScale, heightScale), 0.1)

  model.scale.set(scale)
  model.anchor.set(0.5, 1)
  model.position.set(host.clientWidth * 0.57, host.clientHeight * 0.98)
}

/**
 * 渲染 Ariu 的 Live2D 舞台，并保持与右侧台词框联动。
 */
function AriuStage(props: {
  dialogue: string
  emotion: string
  action: string
  loggedIn: boolean
}) {
  const hostRef = useRef<HTMLDivElement | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    const host = hostRef.current
    if (!host) {
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
      try {
        const PIXI = await import('pixi.js')
        window.PIXI = PIXI
        await ensureCubismCoreScript()
        const { Live2DModel } = await import('pixi-live2d-display/cubism4')

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

        model = await Live2DModel.from(ARIU_MODEL_URL)
        if (destroyed || !app) {
          model.destroy()
          return
        }

        baseWidth = Math.max(model.width, 1)
        baseHeight = Math.max(model.height, 1)
        app.stage.addChild(model)
        syncStageLayout()
        model.focus(host.clientWidth * 0.5, host.clientHeight * 0.58, true)
        setLoading(false)
      } catch (stageError) {
        if (destroyed) {
          return
        }

        setError(stageError instanceof Error ? stageError.message : 'Ariu 模型加载失败')
        setLoading(false)
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
  }, [])

  return (
    <section className="companion-stage-panel">
      <div className="companion-stage-topbar">
        <div className="companion-stage-badges">
          <span className="page-tag">Ariu 陪伴中</span>
          <span className="companion-state-pill">情绪：{props.emotion}</span>
          <span className="companion-state-pill">动作：{props.action}</span>
        </div>
        <span className="companion-stage-copy">{props.loggedIn ? '已连接学习计划' : '未登录，当前为展示模式'}</span>
      </div>

      <div className="companion-stage-canvas-wrap">
        <div className="companion-stage-canvas" ref={hostRef} />

        {loading ? (
          <div className="companion-stage-overlay">
            <strong>正在唤醒 Ariu</strong>
            <span>模型资源加载中，请稍等片刻。</span>
          </div>
        ) : null}

        {error ? (
          <div className="companion-stage-overlay companion-stage-overlay-error">
            <strong>模型加载失败</strong>
            <span>{error}</span>
          </div>
        ) : null}

        <div className="companion-stage-dialogue">
          <span className="section-kicker">Ariu</span>
          <p>{props.dialogue}</p>
        </div>
      </div>
    </section>
  )
}

/**
 * 渲染单个目标卡片中的任务列表，统一展示标题、类型和状态。
 */
function GoalList(props: { items: CompanionPlanTask[]; emptyText: string }) {
  if (props.items.length === 0) {
    return <p className="companion-empty-text">{props.emptyText}</p>
  }

  return (
    <div className="companion-goal-list">
      {props.items.map((item) => (
        <article className="companion-goal-item" key={item.id}>
          <div className="companion-goal-head">
            <strong>{item.title}</strong>
            <span>{taskStatusLabel(item.status)}</span>
          </div>
          <p>{item.description || '当前任务暂无详细说明。'}</p>
          <div className="companion-goal-meta">
            <span>类型：{item.task_type || 'study'}</span>
            <span>Day {item.day_number || 1}</span>
          </div>
        </article>
      ))}
    </div>
  )
}

/**
 * 提供学习陪伴核心页面，整合 Ariu 舞台、计划侧栏和聊天输入区。
 */
export function CompanionWorkspacePage() {
  const accessToken = useAuthStore((state) => state.accessToken)
  const user = useAuthStore((state) => state.user)
  const [history, setHistory] = useState<CompanionHistoryItem[]>(() => buildInitialHistory())
  const [composer, setComposer] = useState('')
  const [sending, setSending] = useState(false)
  const [composerMessage, setComposerMessage] = useState('')

  const currentPlanQuery = useQuery({
    queryKey: ['companion-current-plan', accessToken],
    queryFn: () => fetchCurrentPlan(accessToken as string),
    enabled: Boolean(accessToken),
    retry: false,
  })

  const todayGoals = useMemo(() => deriveTodayGoals(currentPlanQuery.data || null), [currentPlanQuery.data])
  const activeGoals = useMemo(() => deriveActiveGoals(currentPlanQuery.data || null), [currentPlanQuery.data])
  const currentDialogue = useMemo(() => resolveCurrentDialogue(history), [history])
  const stageFeedback = useMemo(() => resolveStageFeedback(history), [history])

  /**
   * 处理用户输入并在必要时请求后端陪伴接口，完成 Ariu 的一轮回复。
   */
  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()

    const content = composer.trim()
    if (!content) {
      setComposerMessage('先输入你想让 Ariu 帮你推进的内容。')
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

    if (!accessToken) {
      setHistory((current) => [
        ...current,
        {
          id: `assistant-login-${Date.now()}`,
          role: 'assistant',
          content: '先登录，我就能结合你的学习计划来安排今天的节奏。',
          emotion: 'reminder',
          action: 'idle',
          createdAt: Date.now(),
        },
      ])
      return
    }

    setSending(true)

    try {
      const reply = await sendCompanionChatRequest(accessToken, [...history, userMessage], currentPlanQuery.data || null)
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
      setComposerMessage(extractErrorMessage(error, 'Ariu 暂时没接上服务，请稍后重试'))
    } finally {
      setSending(false)
    }
  }

  return (
    <section className="page-panel companion-page-panel">
      <div className="companion-layout">
        <aside className="companion-sidebar">
          <div className="companion-sidebar-head">
            <span className="page-tag">学习陪伴</span>
            <h1>{user?.username ? `${user.username} 的 Ariu 陪伴页` : 'Ariu 学习陪伴页'}</h1>
            <p className="page-copy">
              左侧专门放今天要推进的目标与完整对话记录，右侧保持 galgame 式角色舞台，让 Ariu 的反馈始终停留在主视野里。
            </p>
          </div>

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
              emptyText={accessToken ? '当前没有进行中的任务，Ariu 会把下一项未完成目标顶上来。' : '登录后会显示你当前正在推进的任务。'}
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
                    <strong>{item.role === 'assistant' ? 'Ariu' : '你'}</strong>
                    <span>{new Date(item.createdAt).toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' })}</span>
                  </div>
                  <p>{item.content}</p>
                </div>
              ))}
            </div>
          </article>
        </aside>

        <div className="companion-stage-shell">
          <AriuStage
            dialogue={currentDialogue}
            emotion={stageFeedback.emotion}
            action={stageFeedback.action}
            loggedIn={Boolean(accessToken)}
          />

          <section className="status-card companion-input-panel">
            <div className="companion-card-head">
              <div>
                <span className="section-kicker">输入区</span>
                <h2>直接让 Ariu 帮你拆解问题</h2>
              </div>
              <span className="companion-card-note">{sending ? 'Ariu 思考中…' : 'Enter 发送'}</span>
            </div>

            <form className="companion-composer" onSubmit={handleSubmit}>
              <textarea
                value={composer}
                onChange={(event) => setComposer(event.target.value)}
                placeholder="例如：帮我安排今晚的 Go 并发复习顺序，或者总结一下今天还差什么没完成。"
                rows={4}
              />
              <div className="companion-composer-actions">
                <p className="companion-composer-message">
                  {composerMessage || (accessToken ? '已登录，可直接使用 AI 陪伴接口。' : '未登录时会显示本地提示，不会请求后端陪伴接口。')}
                </p>
                <button className="primary-button" type="submit" disabled={sending}>
                  {sending ? '发送中...' : '发送给 Ariu'}
                </button>
              </div>
            </form>
          </section>
        </div>
      </div>
    </section>
  )
}

export default CompanionWorkspacePage
