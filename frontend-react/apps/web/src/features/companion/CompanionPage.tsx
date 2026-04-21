import type { FormEvent } from 'react'
import { useEffect, useMemo, useRef, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import type { Application } from 'pixi.js'
import type { Live2DModel as Cubism4Live2DModel } from 'pixi-live2d-display/cubism4'
import { requestJson, extractErrorMessage } from '@makejob/api-client'
import { isSuccessCode, type ApiEnvelope } from '@makejob/shared-types'
import { useAuthStore } from '../../state/auth'

const MAX_CHAT_HISTORY = 12
const COMPANION_SESSION_SUMMARY_KEY = 'makejob.companion.session-summary'
const COMPANION_SELECTED_MODEL_KEY_PREFIX = 'makejob.companion.selected-live2d:'
const DEFAULT_COMPANION_INDUSTRY_ID = 1
const DEFAULT_COMPANION_INDUSTRY_CODE = 'go'

type CompanionMessageRole = 'assistant' | 'user'

interface CompanionPlanTask {
  id: number
  title: string
  description: string
  task_type: string
  status: string
  due_date?: string
  completed_at?: string
  day_number: number
  sort_order?: number
}

interface CompanionPlanDetail {
  id: number
  title: string
  description: string
  status: string
  total_tasks: number
  completed_tasks: number
  progress: number
  start_date?: string
  end_date?: string
  tasks: CompanionPlanTask[]
  created_at?: string
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

interface CompanionSessionSummary {
  updatedAt: number
  latestAssistantReply: string
  latestUserMessage: string
  planTitle: string
  progress: number
}

interface CompanionPracticeStats {
  today_count: number
  streak_days: number
}

interface CompanionPlanProgress {
  plan_id: number
  total_tasks: number
  completed_tasks: number
  skipped_tasks: number
  in_progress_tasks: number
  pending_tasks: number
  progress: number
}

type CompanionTaskStatus = 'pending' | 'in_progress' | 'completed' | 'skipped'

interface CompanionCategoryNode {
  id: number
  name: string
  children?: CompanionCategoryNode[]
}

interface CompanionCategoryOption {
  id: number
  name: string
}

interface CompanionGeneratePlanForm {
  level: string
  dailyStudyTime: string
  durationDays: string
  goalDescription: string
  weakTopics: string[]
  weakTopicsText: string
}

interface CompanionGeneratePlanPayload {
  level: string
  daily_study_time: number
  weak_topics: string[]
  goal_description: string
  duration_days: number
  industry_id: number
  industry_code: string
}

interface CompanionSelectableLive2DModel {
  key: string
  name: string
  scene: 'interview' | 'companion'
  model_url: string
  thumbnail_url: string
  source: string
  match_type: string
  is_generic: boolean
  is_recommended: boolean
}

declare global {
  interface Window {
    PIXI?: typeof import('pixi.js')
    Live2DCubismCore?: unknown
  }
}

let cubismCoreScriptPromise: Promise<void> | null = null

/**
 * 从本地缓存恢复最近一次陪伴会话摘要，供入口页显示“上次聊到哪一步”。
 */
function readCompanionSessionSummary(): CompanionSessionSummary | null {
  if (typeof window === 'undefined') {
    return null
  }

  try {
    const raw = window.localStorage.getItem(COMPANION_SESSION_SUMMARY_KEY)
    if (!raw) {
      return null
    }

    return JSON.parse(raw) as CompanionSessionSummary
  } catch {
    return null
  }
}

/**
 * 将最近一次陪伴会话摘要写入本地缓存，避免每次返回入口页都丢失上下文。
 */
function persistCompanionSessionSummary(summary: CompanionSessionSummary): void {
  if (typeof window === 'undefined') {
    return
  }

  window.localStorage.setItem(COMPANION_SESSION_SUMMARY_KEY, JSON.stringify(summary))
}

/**
 * 为当前行业上下文构造陪伴模型选择的本地缓存键。
 */
function buildCompanionSelectedModelStorageKey(industryCode: string): string {
  return `${COMPANION_SELECTED_MODEL_KEY_PREFIX}${industryCode.trim() || 'default'}`
}

/**
 * 读取当前行业下最近一次手动切换的陪伴模型键。
 */
function readSelectedCompanionModelKey(industryCode: string): string {
  if (typeof window === 'undefined') {
    return ''
  }

  return window.localStorage.getItem(buildCompanionSelectedModelStorageKey(industryCode)) || ''
}

/**
 * 记住用户在当前行业上下文下选择的陪伴模型，便于下次直接恢复。
 */
function persistSelectedCompanionModelKey(industryCode: string, modelKey: string): void {
  if (typeof window === 'undefined') {
    return
  }

  window.localStorage.setItem(buildCompanionSelectedModelStorageKey(industryCode), modelKey)
}

/**
 * 根据当前计划和历史消息提炼出入口页需要展示的最近会话摘要。
 */
function buildCompanionSessionSummary(
  history: CompanionHistoryItem[],
  plan: CompanionPlanDetail | null,
): CompanionSessionSummary | null {
  const latestAssistantMessage = [...history].reverse().find((item) => item.role === 'assistant')
  const latestUserMessage = [...history].reverse().find((item) => item.role === 'user')

  if (!latestAssistantMessage && !plan) {
    return null
  }

  return {
    updatedAt: Date.now(),
    latestAssistantReply: latestAssistantMessage?.content || '最近还没有新的陪伴回复。',
    latestUserMessage: latestUserMessage?.content || '',
    planTitle: plan?.title || '',
    progress: Math.round(plan?.progress || 0),
  }
}

/**
 * 创建学习计划表单的默认值，避免入口页首次渲染时出现空壳状态。
 */
function buildInitialPlanForm(): CompanionGeneratePlanForm {
  return {
    level: 'beginner',
    dailyStudyTime: '60',
    durationDays: '14',
    goalDescription: '',
    weakTopics: [],
    weakTopicsText: '',
  }
}

/**
 * 将分类树拍平成弱项选项列表，便于入口页直接复用现有题库分类。
 */
function flattenCompanionCategories(nodes: CompanionCategoryNode[], level = 0): CompanionCategoryOption[] {
  return nodes.flatMap((node) => [
    {
      id: node.id,
      name: `${'　'.repeat(level)}${node.name}`,
    },
    ...flattenCompanionCategories(node.children || [], level + 1),
  ])
}

/**
 * 将用户自由输入的弱项文本拆成数组，兼容逗号和换行两种输入方式。
 */
function parseWeakTopicsText(value: string): string[] {
  return Array.from(
    new Set(
      value
        .split(/[\n,，]/)
        .map((item) => item.trim())
        .filter(Boolean),
    ),
  )
}

/**
 * 将计划生成表单转换为后端可直接消费的请求结构。
 */
function buildGeneratePlanPayload(form: CompanionGeneratePlanForm): CompanionGeneratePlanPayload {
  return {
    level: form.level,
    daily_study_time: Number(form.dailyStudyTime) || 60,
    weak_topics: Array.from(new Set([...form.weakTopics, ...parseWeakTopicsText(form.weakTopicsText)])),
    goal_description: form.goalDescription.trim(),
    duration_days: Number(form.durationDays) || 14,
    industry_id: DEFAULT_COMPANION_INDUSTRY_ID,
    industry_code: DEFAULT_COMPANION_INDUSTRY_CODE,
  }
}

/**
 * 获取陪伴页当前可切换的 Live2D 模型列表，并保留后端给出的推荐顺序。
 */
async function fetchSelectableCompanionLive2DModels(industryCode: string): Promise<CompanionSelectableLive2DModel[]> {
  const params = new URLSearchParams({
    scene: 'companion',
  })

  if (industryCode.trim()) {
    params.set('industry_code', industryCode.trim())
  }

  const response = await requestJson<ApiEnvelope<CompanionSelectableLive2DModel[]>>(`/live2d/models?${params.toString()}`)

  if (!isSuccessCode(response.code) || !response.data) {
    throw new Error(response.message || '获取陪伴页 Live2D 模型列表失败')
  }

  return response.data
}

/**
 * 将时间值格式化为入口页可直接展示的中文时间文本。
 */
function formatCompanionDateTime(value?: string | number): string {
  if (!value) {
    return '--'
  }

  const date = typeof value === 'number' ? new Date(value) : new Date(value)
  if (Number.isNaN(date.getTime())) {
    return String(value)
  }

  return date.toLocaleString('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  })
}

/**
 * 将计划状态转换成更适合前台阅读的中文标签。
 */
function planStatusLabel(status: string): string {
  const map: Record<string, string> = {
    active: '进行中',
    paused: '已暂停',
    completed: '已完成',
    draft: '草稿',
  }

  return map[status] || status || '未定义'
}

/**
 * 将学习等级转换成前台文案，避免表单直接暴露英文枚举值。
 */
function planLevelLabel(level: string): string {
  const map: Record<string, string> = {
    beginner: '初级',
    intermediate: '中级',
    advanced: '高级',
  }

  return map[level] || level || '未设置'
}

/**
 * 从当前计划中找出最近完成的一项任务，供入口页展示最近推进记录。
 */
function deriveLatestCompletedTask(plan: CompanionPlanDetail | null): CompanionPlanTask | null {
  if (!plan?.tasks?.length) {
    return null
  }

  const completedTasks = plan.tasks
    .filter((item) => item.status === 'completed' && item.completed_at)
    .sort((left, right) => new Date(right.completed_at || 0).getTime() - new Date(left.completed_at || 0).getTime())

  return completedTasks[0] || null
}

/**
 * 从当前计划中提取接下来最值得继续推进的未完成任务。
 */
function deriveUpcomingTasks(plan: CompanionPlanDetail | null): CompanionPlanTask[] {
  if (!plan?.tasks?.length) {
    return []
  }

  return [...plan.tasks]
    .filter((item) => item.status !== 'completed' && item.status !== 'skipped')
    .sort((left, right) => {
      if ((left.day_number || 0) !== (right.day_number || 0)) {
        return (left.day_number || 0) - (right.day_number || 0)
      }
      return (left.sort_order || 0) - (right.sort_order || 0)
    })
    .slice(0, 3)
}

/**
 * 根据当前计划和会话摘要生成入口页的继续引导文案。
 */
function buildContinueHint(plan: CompanionPlanDetail | null, summary: CompanionSessionSummary | null): string {
  if (plan?.title) {
    return `当前已经有进行中的计划，建议直接进入学习陪伴页继续推进「${plan.title}」。`
  }

  if (summary?.latestAssistantReply) {
    return '入口页已经记住你上次的对话摘要，进入学习陪伴页后可以从上次节奏继续。'
  }

  return '如果你还没有计划，先在这里生成一份学习计划，再进入学习陪伴页开始今天的推进。'
}

/**
 * 提供学习陪伴的二级入口页，避免顶栏导航直接命中重型 Live2D 页面。
 */
export function CompanionHubPage() {
  const accessToken = useAuthStore((state) => state.accessToken)
  const queryClient = useQueryClient()
  const [sessionSummary, setSessionSummary] = useState<CompanionSessionSummary | null>(() => readCompanionSessionSummary())
  const [planForm, setPlanForm] = useState<CompanionGeneratePlanForm>(() => buildInitialPlanForm())
  const [planFormMessage, setPlanFormMessage] = useState('当前阶段先固定生成 Go 方向学习计划。')

  const currentPlanQuery = useQuery({
    queryKey: ['companion-hub-current-plan', accessToken],
    queryFn: () => fetchCurrentPlan(accessToken as string),
    enabled: Boolean(accessToken),
    retry: false,
  })

  const practiceStatsQuery = useQuery({
    queryKey: ['companion-hub-practice-stats', accessToken],
    queryFn: () => fetchCompanionPracticeStats(accessToken as string),
    enabled: Boolean(accessToken),
    retry: false,
  })

  const categoryOptionsQuery = useQuery({
    queryKey: ['companion-hub-category-options', DEFAULT_COMPANION_INDUSTRY_ID],
    queryFn: fetchCompanionCategoryOptions,
    staleTime: 5 * 60 * 1000,
  })

  const planProgressQuery = useQuery({
    queryKey: ['companion-hub-plan-progress', accessToken, currentPlanQuery.data?.id],
    queryFn: () => fetchCompanionPlanProgress(accessToken as string, currentPlanQuery.data?.id as number),
    enabled: Boolean(accessToken && currentPlanQuery.data?.id),
    retry: false,
  })

  const createPlanMutation = useMutation({
    mutationFn: (payload: CompanionGeneratePlanPayload) => createCompanionPlan(accessToken as string, payload),
    onSuccess: async (plan) => {
      setPlanForm(buildInitialPlanForm())
      setPlanFormMessage(`新计划已生成：${plan.title}`)

      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['companion-hub-current-plan'] }),
        queryClient.invalidateQueries({ queryKey: ['companion-hub-plan-progress'] }),
        queryClient.invalidateQueries({ queryKey: ['companion-current-plan'] }),
      ])
    },
    onError: (error) => {
      setPlanFormMessage(extractErrorMessage(error, '生成学习计划失败，请稍后重试'))
    },
  })

  /**
   * 当用户从独立陪伴页返回入口页时，重新读取最近一次本地会话摘要。
   */
  useEffect(() => {
    function handleFocus(): void {
      setSessionSummary(readCompanionSessionSummary())
    }

    handleFocus()
    window.addEventListener('focus', handleFocus)
    return () => {
      window.removeEventListener('focus', handleFocus)
    }
  }, [])

  const progressText = currentPlanQuery.data
    ? `${Math.round(currentPlanQuery.data.progress || 0)}%`
    : (sessionSummary ? `${sessionSummary.progress}%` : '--')
  const latestCompletedTask = useMemo(() => deriveLatestCompletedTask(currentPlanQuery.data || null), [currentPlanQuery.data])
  const upcomingTasks = useMemo(() => deriveUpcomingTasks(currentPlanQuery.data || null), [currentPlanQuery.data])
  const continueHint = useMemo(
    () => buildContinueHint(currentPlanQuery.data || null, sessionSummary),
    [currentPlanQuery.data, sessionSummary],
  )
  const planStats = planProgressQuery.data
  const completedText = latestCompletedTask
    ? `${latestCompletedTask.title} · ${formatCompanionDateTime(latestCompletedTask.completed_at)}`
    : '还没有已完成任务记录'

  /**
   * 切换计划表单中的弱项标签，方便快速组织 AI 生成计划的输入上下文。
   */
  function handleWeakTopicToggle(topicName: string): void {
    setPlanForm((current) => {
      const hasTopic = current.weakTopics.includes(topicName)
      return {
        ...current,
        weakTopics: hasTopic
          ? current.weakTopics.filter((item) => item !== topicName)
          : [...current.weakTopics, topicName],
      }
    })
  }

  /**
   * 提交入口页计划生成表单，并在成功后刷新当前计划与概览数据。
   */
  async function handleGeneratePlan(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()

    if (!accessToken) {
      setPlanFormMessage('请先登录，再让陪伴助手根据你的情况生成学习计划。')
      return
    }

    const payload = buildGeneratePlanPayload(planForm)
    if (!payload.goal_description) {
      setPlanFormMessage('先写清楚目标，例如“两周内完成 Go 并发与数据库复习”。')
      return
    }

    setPlanFormMessage('陪伴助手正在整理你的阶段计划...')
    await createPlanMutation.mutateAsync(payload)
  }

  return (
    <section className="page-panel companion-hub-panel">
      <div className="companion-hub-shell">
        <div className="companion-hub-hero">
          <div className="companion-hub-copy">
            <span className="page-tag">学习陪伴入口</span>
            <h1>先在这里整理计划，再进入学习陪伴页继续学习</h1>
            <p className="page-copy">
              顶部导航只负责带你来到学习陪伴频道，不直接加载 Live2D 主舞台。入口页现在会承接计划生成、当前进度概览和最近一次陪伴摘要，真正的重场景仍然只放在独立陪伴页。
            </p>
            <div className="hero-actions">
              <Link className="primary-button hero-link-button" to="/companion/room">
                进入学习陪伴页
              </Link>
              <a className="secondary-button hero-link-button" href="/companion/room" target="_blank" rel="noreferrer">
                新窗口打开
              </a>
            </div>

            <div className="companion-hub-metrics">
              <article className="metric-card companion-hub-metric-card">
                <strong>{currentPlanQuery.data?.title || '还没有进行中的计划'}</strong>
                <span>当前计划</span>
                <p>进度 {progressText}</p>
              </article>
              <article className="metric-card companion-hub-metric-card">
                <strong>{practiceStatsQuery.data?.streak_days ?? '--'}</strong>
                <span>连续答题天数</span>
                <p>{accessToken ? '临时复用答题统计口径' : '登录后显示'}</p>
              </article>
              <article className="metric-card companion-hub-metric-card">
                <strong>{planStats ? `${planStats.completed_tasks}/${planStats.total_tasks}` : '--'}</strong>
                <span>任务完成数</span>
                <p>{planStats ? `${Math.round(planStats.progress)}% 已推进` : '等待计划同步'}</p>
              </article>
              <article className="metric-card companion-hub-metric-card">
                <strong>{latestCompletedTask?.title || '暂无'}</strong>
                <span>最近完成任务</span>
                <p>{latestCompletedTask?.completed_at ? formatCompanionDateTime(latestCompletedTask.completed_at) : '完成后会显示'}</p>
              </article>
            </div>
          </div>

          <article className="section-card companion-hub-sidecard">
            <span className="section-kicker">继续提示</span>
            <div className="companion-hub-summary">
              <div className="timeline-item">
                <strong>入口页会记住你上次聊到哪</strong>
                <p>{continueHint}</p>
                <div className="companion-hub-meta">
                  <span>最近同步：{sessionSummary ? formatCompanionDateTime(sessionSummary.updatedAt) : '尚未写入摘要'}</span>
                  <span>{accessToken ? '已同步登录态计划' : '未登录时展示本地摘要'}</span>
                </div>
              </div>

              <div className="timeline-item">
                <strong>最近一次会话摘要</strong>
                <p>{sessionSummary?.latestAssistantReply || '还没有历史会话，第一次进入时会从陪伴助手的欢迎语开始。'}</p>
                {sessionSummary?.latestUserMessage ? (
                  <div className="companion-hub-quote">
                    <span>你上次输入：</span>
                    <strong>{sessionSummary.latestUserMessage}</strong>
                  </div>
                ) : null}
              </div>

              <div className="timeline-item">
                <strong>最近完成记录</strong>
                <p>{completedText}</p>
                <div className="companion-hub-meta">
                  <span>今日答题：{practiceStatsQuery.data?.today_count ?? '--'}</span>
                  <span>独立陪伴页只在进入时加载 Live2D</span>
                </div>
              </div>
            </div>
          </article>
        </div>

        <div className="companion-hub-board">
          <section className="status-card companion-plan-builder">
            <div className="companion-card-head">
              <div>
                <span className="section-kicker">生成计划</span>
                <h2>先把这阶段要学什么交给陪伴助手排出来</h2>
              </div>
              <span className="companion-card-note">当前固定为 Go 方向</span>
            </div>

            <p className="companion-empty-text">
              这一版先接通 `POST /api/plans`。行业配置数据还没独立设计，所以前端先固定生成 Go 方向学习计划，弱项可从现有题库分类里快速勾选。
            </p>

            <form className="stack-form companion-plan-form" onSubmit={handleGeneratePlan}>
              <div className="companion-plan-form-grid">
                <label className="field">
                  <span>当前水平</span>
                  <select
                    value={planForm.level}
                    onChange={(event) => setPlanForm((current) => ({ ...current, level: event.target.value }))}
                  >
                    <option value="beginner">{planLevelLabel('beginner')}</option>
                    <option value="intermediate">{planLevelLabel('intermediate')}</option>
                    <option value="advanced">{planLevelLabel('advanced')}</option>
                  </select>
                </label>

                <label className="field">
                  <span>每日学习时长（分钟）</span>
                  <input
                    type="number"
                    min={15}
                    max={480}
                    value={planForm.dailyStudyTime}
                    onChange={(event) => setPlanForm((current) => ({ ...current, dailyStudyTime: event.target.value }))}
                  />
                </label>

                <label className="field">
                  <span>计划周期（天）</span>
                  <input
                    type="number"
                    min={7}
                    max={90}
                    value={planForm.durationDays}
                    onChange={(event) => setPlanForm((current) => ({ ...current, durationDays: event.target.value }))}
                  />
                </label>

                <label className="field">
                  <span>当前方向</span>
                  <input value="Go 面试 / Go 学习计划" disabled />
                </label>
              </div>

              <label className="field">
                <span>阶段目标</span>
                <textarea
                  value={planForm.goalDescription}
                  onChange={(event) => setPlanForm((current) => ({ ...current, goalDescription: event.target.value }))}
                  placeholder="例如：两周内完成 Go 并发、数据库和 Gin 框架复习，并能开始做中等难度面试题。"
                  rows={4}
                />
              </label>

              <div className="field">
                <span>快速选择弱项</span>
                {categoryOptionsQuery.data?.length ? (
                  <div className="companion-topic-pills">
                    {categoryOptionsQuery.data.slice(0, 12).map((item) => (
                      <button
                        className={`companion-topic-pill ${planForm.weakTopics.includes(item.name) ? 'companion-topic-pill-active' : ''}`}
                        key={item.id}
                        type="button"
                        onClick={() => handleWeakTopicToggle(item.name)}
                      >
                        {item.name.trim()}
                      </button>
                    ))}
                  </div>
                ) : (
                  <p className="companion-empty-text">
                    {categoryOptionsQuery.isLoading ? '正在加载弱项建议...' : '当前没有可用的题库分类建议。'}
                  </p>
                )}
              </div>

              <label className="field">
                <span>补充弱项（逗号或换行分隔）</span>
                <textarea
                  value={planForm.weakTopicsText}
                  onChange={(event) => setPlanForm((current) => ({ ...current, weakTopicsText: event.target.value }))}
                  placeholder="例如：性能优化，并发陷阱，SQL 索引"
                  rows={3}
                />
              </label>

              <div className="companion-composer-actions">
                <p className="companion-composer-message">
                  {planFormMessage || '填写目标后即可让陪伴助手生成一份新的学习计划。'}
                </p>
                <button className="primary-button" type="submit" disabled={!accessToken || createPlanMutation.isPending}>
                  {createPlanMutation.isPending ? '生成中...' : '生成学习计划'}
                </button>
              </div>
            </form>
          </section>

          <div className="companion-hub-main">
            <section className="status-card companion-plan-overview">
              <div className="companion-card-head">
                <div>
                  <span className="section-kicker">当前计划概览</span>
                  <h2>{currentPlanQuery.data?.title || '还没有进行中的学习计划'}</h2>
                </div>
                <span className="companion-card-note">
                  {currentPlanQuery.data ? planStatusLabel(currentPlanQuery.data.status) : '等待计划创建'}
                </span>
              </div>

              {currentPlanQuery.isError ? (
                <p className="companion-empty-text">
                  {currentPlanQuery.error instanceof Error ? currentPlanQuery.error.message : '获取当前计划失败'}
                </p>
              ) : null}

              {currentPlanQuery.data ? (
                <>
                  <p className="companion-empty-text">{currentPlanQuery.data.description || '当前计划暂未补充描述。'}</p>
                  <div className="companion-progress-block">
                    <div className="companion-progress-head">
                      <strong>总进度 {Math.round(currentPlanQuery.data.progress || 0)}%</strong>
                      <span>
                        {currentPlanQuery.data.completed_tasks}/{currentPlanQuery.data.total_tasks} 已完成
                      </span>
                    </div>
                    <div className="companion-progress-bar">
                      <div className="companion-progress-bar-fill" style={{ width: `${Math.round(currentPlanQuery.data.progress || 0)}%` }} />
                    </div>
                  </div>

                  <div className="companion-overview-stats">
                    <div className="companion-stat-chip">
                      <strong>{planStats?.completed_tasks ?? currentPlanQuery.data.completed_tasks}</strong>
                      <span>已完成</span>
                    </div>
                    <div className="companion-stat-chip">
                      <strong>{planStats?.in_progress_tasks ?? deriveActiveGoals(currentPlanQuery.data).length}</strong>
                      <span>进行中</span>
                    </div>
                    <div className="companion-stat-chip">
                      <strong>{planStats?.pending_tasks ?? upcomingTasks.length}</strong>
                      <span>待开始</span>
                    </div>
                    <div className="companion-stat-chip">
                      <strong>{planStats?.skipped_tasks ?? 0}</strong>
                      <span>已跳过</span>
                    </div>
                  </div>

                  <div className="companion-hub-tasklist">
                    <article className="timeline-item">
                      <strong>最近完成任务</strong>
                      <p>{latestCompletedTask?.title || '当前还没有已完成任务。'}</p>
                      <div className="companion-hub-meta">
                        <span>{latestCompletedTask?.completed_at ? `完成于 ${formatCompanionDateTime(latestCompletedTask.completed_at)}` : '完成后这里会显示时间'}</span>
                        <span>{latestCompletedTask ? taskStatusLabel(latestCompletedTask.status) : '等待推进'}</span>
                      </div>
                    </article>

                    <article className="timeline-item">
                      <strong>下一步建议直接推进</strong>
                      {upcomingTasks.length > 0 ? (
                        <div className="companion-upcoming-list">
                          {upcomingTasks.map((item) => (
                            <div className="companion-upcoming-item" key={item.id}>
                              <div className="companion-hub-meta">
                                <span>Day {item.day_number || 1}</span>
                                <span>{taskStatusLabel(item.status)}</span>
                              </div>
                              <p>{item.title}</p>
                            </div>
                          ))}
                        </div>
                      ) : (
                        <p>当前没有待推进任务，说明这份计划可能已经全部完成。</p>
                      )}
                    </article>
                  </div>
                </>
              ) : (
                <div className="timeline-item">
                  <strong>还没有当前计划</strong>
                  <p>先在左侧填写目标，让陪伴助手生成计划。生成成功后，这里会自动显示进度、最近完成记录和下一步任务。</p>
                </div>
              )}
            </section>
          </div>
        </div>
      </div>
    </section>
  )
}

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
 * 拉取计划进度统计，为入口页补充更细的任务状态概览。
 */
async function fetchCompanionPlanProgress(token: string, planId: number): Promise<CompanionPlanProgress> {
  const response = await requestJson<ApiEnvelope<CompanionPlanProgress>>(`/plans/${planId}/progress`, {
    token,
  })

  if (!isSuccessCode(response.code) || !response.data) {
    throw new Error(response.message || '获取计划进度失败')
  }

  return response.data
}

/**
 * 拉取连续答题统计，作为当前阶段入口页的临时连续天数展示来源。
 */
async function fetchCompanionPracticeStats(token: string): Promise<CompanionPracticeStats> {
  const response = await requestJson<ApiEnvelope<CompanionPracticeStats>>('/user/practice-stats', {
    token,
  })

  if (!isSuccessCode(response.code) || !response.data) {
    throw new Error(response.message || '获取练习统计失败')
  }

  return response.data
}

/**
 * 读取 Go 行业下的题库分类，为计划生成表单提供弱项建议标签。
 */
async function fetchCompanionCategoryOptions(): Promise<CompanionCategoryOption[]> {
  const response = await requestJson<ApiEnvelope<CompanionCategoryNode[]>>(`/categories?industry_id=${DEFAULT_COMPANION_INDUSTRY_ID}`)

  if (!isSuccessCode(response.code) || !response.data) {
    throw new Error(response.message || '获取弱项分类失败')
  }

  return flattenCompanionCategories(response.data)
}

/**
 * 调用计划生成接口，创建新的学习计划并返回最新详情。
 */
async function createCompanionPlan(token: string, payload: CompanionGeneratePlanPayload): Promise<CompanionPlanDetail> {
  const response = await requestJson<ApiEnvelope<CompanionPlanDetail>>('/plans', {
    method: 'POST',
    token,
    body: payload,
  })

  if (!isSuccessCode(response.code) || !response.data) {
    throw new Error(response.message || '生成学习计划失败')
  }

  return response.data
}

/**
 * 更新单个学习任务状态，让陪伴页可以直接驱动计划推进。
 */
async function updateCompanionTaskStatus(
  token: string,
  planId: number,
  taskId: number,
  status: CompanionTaskStatus,
): Promise<void> {
  const response = await requestJson<ApiEnvelope<null>>(`/plans/${planId}/tasks/${taskId}`, {
    method: 'PUT',
    token,
    body: {
      status,
    },
  })

  if (!isSuccessCode(response.code)) {
    throw new Error(response.message || '更新任务状态失败')
  }
}

/**
 * 请求后端重新调整当前学习计划，返回新的计划详情。
 */
async function adjustCompanionPlan(token: string, planId: number): Promise<CompanionPlanDetail> {
  const response = await requestJson<ApiEnvelope<CompanionPlanDetail>>(`/plans/${planId}/adjust`, {
    method: 'POST',
    token,
    body: {},
  })

  if (!isSuccessCode(response.code) || !response.data) {
    throw new Error(response.message || '调整学习计划失败')
  }

  return response.data
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
 * 向后端陪伴接口发送消息，并返回陪伴助手的最新回复内容。
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
    throw new Error(response.message || '陪伴助手暂时没有回复')
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
 * 根据任务当前状态给出下一步可操作按钮，避免陪伴页出现无意义的状态切换。
 */
function buildTaskStatusActions(status: string): Array<{ status: CompanionTaskStatus; label: string }> {
  switch (status) {
    case 'pending':
      return [
        { status: 'in_progress', label: '开始' },
        { status: 'completed', label: '直接完成' },
      ]
    case 'in_progress':
      return [
        { status: 'completed', label: '标记完成' },
        { status: 'pending', label: '退回待办' },
        { status: 'skipped', label: '跳过' },
      ]
    case 'completed':
      return [
        { status: 'pending', label: '重新打开' },
      ]
    case 'skipped':
      return [
        { status: 'pending', label: '恢复待办' },
        { status: 'completed', label: '记为完成' },
      ]
    default:
      return []
  }
}

/**
 * 从计划统计中生成易读的阶段说明，帮助用户快速判断当前阻塞点。
 */
function buildPlanProgressHint(progress: CompanionPlanProgress | undefined): string {
  if (!progress) {
    return '进度统计加载后，这里会告诉你当前最需要处理的是待办、进行中还是跳过项。'
  }

  if (progress.in_progress_tasks > 0) {
    return `当前有 ${progress.in_progress_tasks} 项任务正在推进，优先把进行中内容收口。`
  }

  if (progress.pending_tasks > 0) {
    return `还有 ${progress.pending_tasks} 项任务待开始，适合让陪伴助手帮你拆分下一步。`
  }

  if (progress.skipped_tasks > 0) {
    return `已有 ${progress.skipped_tasks} 项任务被跳过，后续可以考虑重新调整计划。`
  }

  return '这份计划当前已经没有待推进任务，可以考虑调整计划或开始新的阶段。'
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
  industryCode: string
}) {
  const hostRef = useRef<HTMLDivElement | null>(null)
  const [selectedModelKey, setSelectedModelKey] = useState(() => readSelectedCompanionModelKey(props.industryCode))
  const [stageLoading, setStageLoading] = useState(false)
  const [stageError, setStageError] = useState('')

  const modelOptionsQuery = useQuery({
    queryKey: ['companion-live2d-selectable-models', props.industryCode],
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
 * 渲染单个目标卡片中的任务列表，统一展示标题、类型和状态。
 */
function GoalList(props: {
  items: CompanionPlanTask[]
  emptyText: string
  onStatusChange?: (task: CompanionPlanTask, status: CompanionTaskStatus) => void
  pendingTaskId?: number | null
}) {
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
          {props.onStatusChange ? (
            <div className="companion-task-actions">
              {buildTaskStatusActions(item.status).map((action) => (
                <button
                  className="secondary-button companion-task-button"
                  key={`${item.id}-${action.status}`}
                  type="button"
                  disabled={props.pendingTaskId === item.id}
                  onClick={() => props.onStatusChange?.(item, action.status)}
                >
                  {props.pendingTaskId === item.id ? '提交中...' : action.label}
                </button>
              ))}
            </div>
          ) : null}
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
  const queryClient = useQueryClient()
  const [history, setHistory] = useState<CompanionHistoryItem[]>(() => buildInitialHistory())
  const [composer, setComposer] = useState('')
  const [sending, setSending] = useState(false)
  const [composerMessage, setComposerMessage] = useState('')
  const [taskActionTaskId, setTaskActionTaskId] = useState<number | null>(null)
  const [planActionMessage, setPlanActionMessage] = useState('')

  const currentPlanQuery = useQuery({
    queryKey: ['companion-current-plan', accessToken],
    queryFn: () => fetchCurrentPlan(accessToken as string),
    enabled: Boolean(accessToken),
    retry: false,
  })

  const planProgressQuery = useQuery({
    queryKey: ['companion-current-plan-progress', accessToken, currentPlanQuery.data?.id],
    queryFn: () => fetchCompanionPlanProgress(accessToken as string, currentPlanQuery.data?.id as number),
    enabled: Boolean(accessToken && currentPlanQuery.data?.id),
    retry: false,
  })

  const todayGoals = useMemo(() => deriveTodayGoals(currentPlanQuery.data || null), [currentPlanQuery.data])
  const activeGoals = useMemo(() => deriveActiveGoals(currentPlanQuery.data || null), [currentPlanQuery.data])
  const currentDialogue = useMemo(() => resolveCurrentDialogue(history), [history])
  const stageFeedback = useMemo(() => resolveStageFeedback(history), [history])
  const planProgressHint = useMemo(() => buildPlanProgressHint(planProgressQuery.data), [planProgressQuery.data])

  const updateTaskMutation = useMutation({
    mutationFn: (payload: { taskId: number; status: CompanionTaskStatus }) =>
      updateCompanionTaskStatus(accessToken as string, currentPlanQuery.data?.id as number, payload.taskId, payload.status),
    onSuccess: async (_, variables) => {
      setTaskActionTaskId(null)
      setPlanActionMessage(`任务状态已更新为「${taskStatusLabel(variables.status)}」。`)
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['companion-current-plan'] }),
        queryClient.invalidateQueries({ queryKey: ['companion-current-plan-progress'] }),
        queryClient.invalidateQueries({ queryKey: ['companion-hub-current-plan'] }),
        queryClient.invalidateQueries({ queryKey: ['companion-hub-plan-progress'] }),
      ])
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
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['companion-current-plan'] }),
        queryClient.invalidateQueries({ queryKey: ['companion-current-plan-progress'] }),
        queryClient.invalidateQueries({ queryKey: ['companion-hub-current-plan'] }),
        queryClient.invalidateQueries({ queryKey: ['companion-hub-plan-progress'] }),
      ])
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
   * 在陪伴页直接切换任务状态，让计划推进不必退回入口页操作。
   */
  async function handleTaskStatusChange(task: CompanionPlanTask, status: CompanionTaskStatus) {
    if (!accessToken || !currentPlanQuery.data?.id) {
      setPlanActionMessage('请先登录并确保当前存在可操作的学习计划。')
      return
    }

    setTaskActionTaskId(task.id)
    setPlanActionMessage(`正在把「${task.title}」更新为「${taskStatusLabel(status)}」...`)
    await updateTaskMutation.mutateAsync({
      taskId: task.id,
      status,
    })
  }

  /**
   * 触发后端动态调整计划，适合在任务阻塞或节奏需要重排时使用。
   */
  async function handleAdjustPlan() {
    if (!accessToken || !currentPlanQuery.data?.id) {
      setPlanActionMessage('请先登录并生成学习计划后再调整。')
      return
    }

    setPlanActionMessage('陪伴助手正在重新整理你的计划节奏...')
    await adjustPlanMutation.mutateAsync()
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
      setComposerMessage(extractErrorMessage(error, '陪伴助手暂时没接上服务，请稍后重试'))
    } finally {
      setSending(false)
    }
  }

  return (
    <section className="page-panel companion-page-panel">
      <div className="companion-room-toolbar">
        <Link className="ghost-button companion-room-back" to="/companion">
          返回陪伴入口
        </Link>
        <span className="companion-room-note">当前为学习陪伴独立页</span>
      </div>

      <div className="companion-layout">
        <aside className="companion-sidebar">
          <div className="companion-sidebar-head">
            <span className="page-tag">学习陪伴</span>
            <h1>{user?.username ? `${user.username} 的学习陪伴页` : '学习陪伴页'}</h1>
            <p className="page-copy">
              左侧专门放今天要推进的目标与完整对话记录，右侧保留角色舞台，并支持在当前场景下切换可用模型。
            </p>
          </div>

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

        <div className="companion-stage-shell">
          <AriuStage
            dialogue={currentDialogue}
            emotion={stageFeedback.emotion}
            action={stageFeedback.action}
            loggedIn={Boolean(accessToken)}
            industryCode={DEFAULT_COMPANION_INDUSTRY_CODE}
          />

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
                placeholder="例如：帮我安排今晚的 Go 并发复习顺序，或者总结一下今天还差什么没完成。"
                rows={4}
              />
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
      </div>
    </section>
  )
}

export default CompanionWorkspacePage
