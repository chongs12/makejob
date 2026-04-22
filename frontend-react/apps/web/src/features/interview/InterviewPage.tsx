import type { FormEvent } from 'react'
import { useEffect, useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link, useNavigate, useParams } from '@tanstack/react-router'
import { extractErrorMessage, requestJson } from '@makejob/api-client'
import { isSuccessCode, type ApiEnvelope } from '@makejob/shared-types'
import { useAuthStore } from '../../state/auth'
import {
  DEFAULT_FRONTEND_INDUSTRY_CODE as INTERVIEW_DEFAULT_INDUSTRY_CODE,
  fetchFrontendIndustries,
  formatFrontendIndustryLabel,
  persistSelectedFrontendIndustryCode,
  readSelectedFrontendIndustryCode,
  resolvePreferredFrontendIndustry,
  subscribeFrontendIndustryCodeChange,
} from '../../shared/industryContext'

interface InterviewConfigForm {
  difficulty: string
  questionCount: string
  topicsText: string
}

interface InterviewQuestion {
  question: string
  topic: string
  difficulty: string
  type: string
  hints?: string
}

interface InterviewFeedback {
  score: number
  is_correct: boolean
  feedback: string
  key_points: string[]
  suggestions: string
  follow_up: string
}

interface InterviewReport {
  overall_score: number
  total_questions: number
  correct_count: number
  dimension_scores: Record<string, number>
  strengths: string[]
  weaknesses: string[]
  suggestions: string[]
  summary: string
}

interface InterviewCreateResponse {
  interview_id: number
  status: string
  first_question?: InterviewQuestion | null
  created_at: string
}

interface InterviewHistoryItem {
  id: number
  status: string
  score: number
  total_questions: number
  started_at?: string
  ended_at?: string
  created_at?: string
}

interface InterviewMessage {
  role: string
  content: string
  message_type: string
  created_at: string
}

interface InterviewDetailResponse {
  id: number
  industry_code: string
  status: string
  score: number
  total_questions: number
  messages: InterviewMessage[]
  started_at?: string
  ended_at?: string
}

interface InterviewAnswerResponse {
  feedback?: InterviewFeedback | null
  next_question?: InterviewQuestion | null
  is_finished: boolean
}

interface InterviewNextQuestionResponse {
  question?: InterviewQuestion | null
  question_no: number
  is_last: boolean
}

interface InterviewReportResponse {
  interview_id: number
  report?: InterviewReport | null
  duration_seconds: number
  completed_at: string
}

interface PageResult<T> {
  list: T[]
  total: number
  page: number
  page_size: number
}

/**
 * 根据当前行业生成更贴近方向语境的默认面试主题，减少用户首次填写成本。
 */
function buildDefaultInterviewTopics(industryCode: string): string {
  const topicMap: Record<string, string> = {
    go: 'Go基础, 并发编程, Gin, 数据库',
    java: 'Java基础, JVM, Spring, 数据库',
    frontend: 'HTML/CSS, JavaScript, React, 工程化',
  }

  return topicMap[industryCode.trim()] || '基础概念, 核心框架, 数据结构, 工程实践'
}

/**
 * 创建 AI 面试入口页默认表单，保证首次进入时就能直接提交。
 */
function buildInitialInterviewForm(): InterviewConfigForm {
  return {
    difficulty: 'medium',
    questionCount: '5',
    topicsText: buildDefaultInterviewTopics(INTERVIEW_DEFAULT_INDUSTRY_CODE),
  }
}

/**
 * 将主题输入拆成后端需要的数组结构，兼容逗号和换行分隔。
 */
function parseInterviewTopics(value: string): string[] {
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
 * 将面试难度枚举转换为前台可读文案。
 */
function interviewDifficultyLabel(difficulty: string): string {
  const map: Record<string, string> = {
    easy: '简单',
    medium: '中等',
    hard: '困难',
    mixed: '混合',
  }

  return map[difficulty] || difficulty || '未知'
}

/**
 * 将面试状态转换成更清晰的中文标签。
 */
function interviewStatusLabel(status: string): string {
  const map: Record<string, string> = {
    ongoing: '进行中',
    completed: '已完成',
    cancelled: '已取消',
  }

  return map[status] || status || '未知状态'
}

/**
 * 将题目类型转换成前台可读标签。
 */
function interviewQuestionTypeLabel(type: string): string {
  const map: Record<string, string> = {
    technical: '技术题',
    behavioral: '行为题',
    coding: '编程题',
  }

  return map[type] || type || '未分类'
}

/**
 * 格式化面试页面中的时间文本，保持整体显示口径一致。
 */
function formatInterviewDateTime(value?: string): string {
  if (!value) {
    return '--'
  }

  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return value
  }

  return date.toLocaleString('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  })
}

/**
 * 将秒数转换为更易读的分钟文本，用于面试报告展示。
 */
function formatInterviewDuration(seconds: number): string {
  if (!seconds) {
    return '不足 1 分钟'
  }

  const minutes = Math.max(Math.round(seconds / 60), 1)
  return `${minutes} 分钟`
}

/**
 * 统计消息列表中用户已回答的题目数，用于推导当前进度。
 */
function countInterviewAnswers(messages: InterviewMessage[]): number {
  return messages.filter((item) => item.role === 'user').length
}

/**
 * 从详情消息里提取当前仍待回答的问题。
 */
function resolveCurrentInterviewQuestion(detail: InterviewDetailResponse | undefined): InterviewQuestion | null {
  if (!detail || detail.status !== 'ongoing') {
    return null
  }

  const answeredCount = countInterviewAnswers(detail.messages)
  if (answeredCount >= detail.total_questions) {
    return null
  }

  const latestQuestionMessage = [...detail.messages]
    .reverse()
    .find((item) => item.role === 'ai' && item.message_type === 'text')

  if (!latestQuestionMessage) {
    return null
  }

  return {
    question: latestQuestionMessage.content,
    topic: '',
    difficulty: '',
    type: 'technical',
    hints: '',
  }
}

/**
 * 提取最近一次反馈消息，供进行页顶部状态卡使用。
 */
function resolveLatestInterviewFeedback(detail: InterviewDetailResponse | undefined): string {
  if (!detail) {
    return ''
  }

  const latestFeedbackMessage = [...detail.messages]
    .reverse()
    .find((item) => item.role === 'ai' && item.message_type === 'feedback')

  return latestFeedbackMessage?.content || ''
}

/**
 * 拉取用户面试历史，为入口页提供最近记录和继续入口。
 */
async function fetchInterviewHistory(token: string): Promise<PageResult<InterviewHistoryItem>> {
  const response = await requestJson<ApiEnvelope<PageResult<InterviewHistoryItem>>>('/interviews?page=1&page_size=6', {
    token,
  })

  if (!isSuccessCode(response.code) || !response.data) {
    throw new Error(response.message || '获取面试历史失败')
  }

  return response.data
}

/**
 * 创建新的 AI 面试会话，并返回首题信息。
 */
async function createInterviewRequest(
  token: string,
  payload: {
    industry_code: string
    difficulty: string
    topics: string[]
    question_count: number
  },
): Promise<InterviewCreateResponse> {
  const response = await requestJson<ApiEnvelope<InterviewCreateResponse>>('/interviews', {
    method: 'POST',
    token,
    body: payload,
  })

  if (!isSuccessCode(response.code) || !response.data) {
    throw new Error(response.message || '创建面试失败')
  }

  return response.data
}

/**
 * 拉取面试详情，用于进行页恢复当前对话与状态。
 */
async function fetchInterviewDetail(token: string, interviewId: string): Promise<InterviewDetailResponse> {
  const response = await requestJson<ApiEnvelope<InterviewDetailResponse>>(`/interviews/${interviewId}`, {
    token,
  })

  if (!isSuccessCode(response.code) || !response.data) {
    throw new Error(response.message || '获取面试详情失败')
  }

  return response.data
}

/**
 * 提交当前问题的回答并获取反馈。
 */
async function submitInterviewAnswer(token: string, interviewId: string, answer: string): Promise<InterviewAnswerResponse> {
  const response = await requestJson<ApiEnvelope<InterviewAnswerResponse>>(`/interviews/${interviewId}/answer`, {
    method: 'POST',
    token,
    body: {
      answer,
    },
  })

  if (!isSuccessCode(response.code) || !response.data) {
    throw new Error(response.message || '提交回答失败')
  }

  return response.data
}

/**
 * 手动获取下一题，作为自动推进失败时的恢复入口。
 */
async function fetchNextInterviewQuestion(token: string, interviewId: string): Promise<InterviewNextQuestionResponse> {
  const response = await requestJson<ApiEnvelope<InterviewNextQuestionResponse>>(`/interviews/${interviewId}/next`, {
    token,
  })

  if (!isSuccessCode(response.code) || !response.data) {
    throw new Error(response.message || '获取下一题失败')
  }

  return response.data
}

/**
 * 结束当前面试并触发后端生成报告。
 */
async function finishInterviewRequest(token: string, interviewId: string): Promise<InterviewReportResponse> {
  const response = await requestJson<ApiEnvelope<InterviewReportResponse>>(`/interviews/${interviewId}/finish`, {
    method: 'POST',
    token,
    body: {},
  })

  if (!isSuccessCode(response.code) || !response.data) {
    throw new Error(response.message || '结束面试失败')
  }

  return response.data
}

/**
 * 获取已完成面试的报告详情。
 */
async function fetchInterviewReport(token: string, interviewId: string): Promise<InterviewReportResponse> {
  const response = await requestJson<ApiEnvelope<InterviewReportResponse>>(`/interviews/${interviewId}/report`, {
    token,
  })

  if (!isSuccessCode(response.code) || !response.data) {
    throw new Error(response.message || '获取面试报告失败')
  }

  return response.data
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
      setMessage('请先登录，再开始你的 AI 模拟面试。')
      return
    }

    const topics = parseInterviewTopics(form.topicsText)
    if (topics.length === 0) {
      setMessage(`至少填写一个主题，例如 ${buildDefaultInterviewTopics(effectiveIndustryCode).split(',')[0]}。`)
      return
    }

    setMessage('Ariu 正在准备你的第一道面试题...')
    await createMutation.mutateAsync({
      industry_code: effectiveIndustryCode,
      difficulty: form.difficulty,
      topics,
      question_count: Number(form.questionCount) || 5,
    })
  }

  return (
    <section className="page-panel interview-page-panel">
      <div className="interview-hero">
        <div className="interview-hero-copy">
          <span className="page-tag">AI 面试主链路</span>
          <h1>{user?.username ? `${user.username}，开始一场 ${effectiveIndustryLabel} 模拟面试` : `开始一场 ${effectiveIndustryLabel} 模拟面试`}</h1>
          <p className="page-copy">
            当前优先跑通文本面试闭环：选择行业、配置题量和主题，进入会话，逐题回答，拿到反馈和报告。语音、TTS、动作联动仍放在后续阶段。
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
              <button className="primary-button" type="submit" disabled={!accessToken || createMutation.isPending}>
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
              <Link className="secondary-link" to="/auth/login">前往登录</Link>
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
 * 渲染 AI 面试进行页，负责展示当前问题、对话历史与答题输入。
 */
export function InterviewSessionPage() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const accessToken = useAuthStore((state) => state.accessToken)
  const params = useParams({ strict: false })
  const interviewId = String(params.interviewId || '')
  const [answer, setAnswer] = useState('')
  const [message, setMessage] = useState('直接输入你的答案，提交后会得到评分反馈。')

  const detailQuery = useQuery({
    queryKey: ['interview-detail', accessToken, interviewId],
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

  const submitMutation = useMutation({
    mutationFn: (content: string) => submitInterviewAnswer(accessToken as string, interviewId, content),
    onSuccess: async (data) => {
      setAnswer('')
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

  const currentQuestion = useMemo(() => resolveCurrentInterviewQuestion(detailQuery.data), [detailQuery.data])
  const answerCount = useMemo(() => countInterviewAnswers(detailQuery.data?.messages || []), [detailQuery.data])
  const latestFeedback = useMemo(() => resolveLatestInterviewFeedback(detailQuery.data), [detailQuery.data])
  const sessionIndustryCode = detailQuery.data?.industry_code || readSelectedFrontendIndustryCode() || INTERVIEW_DEFAULT_INDUSTRY_CODE
  const sessionIndustry = useMemo(
    () => resolvePreferredFrontendIndustry(industriesQuery.data || [], sessionIndustryCode, INTERVIEW_DEFAULT_INDUSTRY_CODE),
    [industriesQuery.data, sessionIndustryCode],
  )
  const sessionIndustryLabel = formatFrontendIndustryLabel(sessionIndustry, sessionIndustryCode)

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
   * 提交当前问题的文字回答，并刷新面试详情。
   */
  async function handleSubmitAnswer(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()

    const content = answer.trim()
    if (!content) {
      setMessage('先输入你的回答，再提交给 AI 面试官。')
      return
    }

    await submitMutation.mutateAsync(content)
  }

  return (
    <section className="page-panel interview-page-panel">
      <div className="companion-room-toolbar">
        <Link className="ghost-button companion-room-back" to="/interview">
          返回面试入口
        </Link>
        <span className="companion-room-note">当前为文本面试进行页 · {sessionIndustryLabel}</span>
      </div>

      <div className="interview-session-layout">
        <aside className="interview-session-sidebar">
          <article className="status-card">
            <div className="companion-card-head">
              <div>
                <span className="section-kicker">当前状态</span>
                <h2>面试 #{interviewId}</h2>
              </div>
              <span className="companion-card-note">{interviewStatusLabel(detailQuery.data?.status || 'ongoing')}</span>
            </div>
            <div className="interview-overview-stats">
              <div className="companion-stat-chip">
                <strong>{answerCount}</strong>
                <span>已回答</span>
              </div>
              <div className="companion-stat-chip">
                <strong>{detailQuery.data?.total_questions || '--'}</strong>
                <span>总题数</span>
              </div>
              <div className="companion-stat-chip">
                <strong>{currentQuestion ? answerCount + 1 : '--'}</strong>
                <span>当前题号</span>
              </div>
            </div>
            <p className="companion-empty-text">
              {detailQuery.data?.started_at ? `开始时间：${formatInterviewDateTime(detailQuery.data.started_at)}` : '面试开始时间待同步'}
            </p>
          </article>

          <article className="status-card">
            <div className="companion-card-head">
              <div>
                <span className="section-kicker">当前题目</span>
                <h2>{currentQuestion ? `第 ${answerCount + 1} 题` : '等待题目恢复'}</h2>
              </div>
              <span className="companion-card-note">
                {currentQuestion ? interviewQuestionTypeLabel(currentQuestion.type) : '无活动题目'}
              </span>
            </div>
            <p className="question-content">{currentQuestion?.question || '如果当前没有题目，可尝试点击“恢复下一题”或直接结束面试。'}</p>
            {currentQuestion?.hints ? <p className="companion-empty-text">提示：{currentQuestion.hints}</p> : null}
          </article>

          <article className="status-card">
            <div className="companion-card-head">
              <div>
                <span className="section-kicker">最近反馈</span>
                <h2>上一轮回答结果</h2>
              </div>
            </div>
            <p className="question-content">{latestFeedback || '提交第一道题答案后，这里会展示最新反馈。'}</p>
          </article>
        </aside>

        <div className="interview-session-main">
          <section className="status-card interview-message-panel">
            <div className="companion-card-head">
              <div>
                <span className="section-kicker">对话历史</span>
                <h2>面试过程完整记录</h2>
              </div>
              <span className="companion-card-note">{detailQuery.data?.messages.length || 0} 条</span>
            </div>

            {detailQuery.isLoading ? <p className="companion-empty-text">正在恢复面试记录...</p> : null}
            {detailQuery.isError ? (
              <p className="companion-empty-text">
                {detailQuery.error instanceof Error ? detailQuery.error.message : '面试记录加载失败'}
              </p>
            ) : null}

            {detailQuery.data?.messages?.length ? (
              <div className="interview-message-list">
                {detailQuery.data.messages.map((item, index) => (
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
          </section>

          <section className="status-card interview-answer-panel">
            <div className="companion-card-head">
              <div>
                <span className="section-kicker">回答输入</span>
                <h2>按当前题目直接作答</h2>
              </div>
              <span className="companion-card-note">
                {submitMutation.isPending ? 'AI 正在评估...' : '支持多段文字回答'}
              </span>
            </div>

            <form className="companion-composer" onSubmit={handleSubmitAnswer}>
              <textarea
                rows={7}
                value={answer}
                onChange={(event) => setAnswer(event.target.value)}
                placeholder="直接输入你的回答。建议先说思路，再补关键点、权衡和落地细节。"
                disabled={detailQuery.data?.status !== 'ongoing'}
              />
              <div className="interview-answer-actions">
                <p className="companion-composer-message">{message}</p>
                <div className="page-actions">
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
                    disabled={submitMutation.isPending || detailQuery.data?.status !== 'ongoing'}
                  >
                    {submitMutation.isPending ? '提交中...' : '提交答案'}
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
    </section>
  )
}

/**
 * 渲染 AI 面试报告页，集中展示得分、维度、优势与待改进点。
 */
export function InterviewReportPage() {
  const accessToken = useAuthStore((state) => state.accessToken)
  const params = useParams({ strict: false })
  const interviewId = String(params.interviewId || '')
  const [selectedIndustryCode, setSelectedIndustryCode] = useState(() => readSelectedFrontendIndustryCode() || INTERVIEW_DEFAULT_INDUSTRY_CODE)

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
              {reportQuery.data?.completed_at ? `完成于 ${formatInterviewDateTime(reportQuery.data.completed_at)}` : '等待报告加载'}
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

          {reportQuery.data?.report ? (
            <>
              <div className="interview-report-metrics">
                <article className="metric-card">
                  <strong>{Math.round(reportQuery.data.report.overall_score || 0)}</strong>
                  <span>总分</span>
                </article>
                <article className="metric-card">
                  <strong>{reportQuery.data.report.correct_count}</strong>
                  <span>命中题数</span>
                </article>
                <article className="metric-card">
                  <strong>{reportQuery.data.report.total_questions}</strong>
                  <span>总题量</span>
                </article>
                <article className="metric-card">
                  <strong>{formatInterviewDuration(reportQuery.data.duration_seconds)}</strong>
                  <span>面试时长</span>
                </article>
              </div>

              <div className="timeline-item">
                <strong>总结</strong>
                <p>{reportQuery.data.report.summary || '当前报告未生成总结。'}</p>
              </div>

              <div className="interview-report-sections">
                <article className="timeline-item">
                  <strong>优势</strong>
                  {reportQuery.data.report.strengths?.length ? (
                    <ul className="interview-bullet-list">
                      {reportQuery.data.report.strengths.map((item) => <li key={item}>{item}</li>)}
                    </ul>
                  ) : (
                    <p>当前没有返回优势项。</p>
                  )}
                </article>

                <article className="timeline-item">
                  <strong>待加强点</strong>
                  {reportQuery.data.report.weaknesses?.length ? (
                    <ul className="interview-bullet-list">
                      {reportQuery.data.report.weaknesses.map((item) => <li key={item}>{item}</li>)}
                    </ul>
                  ) : (
                    <p>当前没有返回待加强项。</p>
                  )}
                </article>

                <article className="timeline-item">
                  <strong>后续建议</strong>
                  {reportQuery.data.report.suggestions?.length ? (
                    <ul className="interview-bullet-list">
                      {reportQuery.data.report.suggestions.map((item) => <li key={item}>{item}</li>)}
                    </ul>
                  ) : (
                    <p>当前没有返回建议项。</p>
                  )}
                </article>
              </div>

              <article className="timeline-item">
                <strong>维度评分</strong>
                {Object.keys(reportQuery.data.report.dimension_scores || {}).length ? (
                  <div className="interview-dimension-grid">
                    {Object.entries(reportQuery.data.report.dimension_scores).map(([key, value]) => (
                      <div className="companion-stat-chip" key={key}>
                        <strong>{Math.round(value)}</strong>
                        <span>{key}</span>
                      </div>
                    ))}
                  </div>
                ) : (
                  <p>当前报告没有返回维度评分。</p>
                )}
              </article>
            </>
          ) : null}
        </section>

        <aside className="home-side-column">
          <article className="section-card sidebar-card">
            <span className="section-kicker">下一步建议</span>
            <div className="sidebar-links">
              <Link className="sidebar-link" to="/practice">先去补题库弱项</Link>
              <Link className="sidebar-link" to="/companion">去学习陪伴继续推进计划</Link>
              <Link className="sidebar-link" to="/interview">再开一场新的面试</Link>
            </div>
          </article>
        </aside>
      </div>
    </section>
  )
}
