import type { FormEvent } from 'react'
import { useEffect, useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link, useNavigate } from '@tanstack/react-router'
import { extractErrorMessage } from '@makejob/api-client'
import { useAuthStore } from '../../state/auth'
import { AsyncEmptyState, AsyncInlineState } from '../../shared/asyncState'
import { requestLoginPrompt } from '../../shared/loginPrompt'
import {
  DEFAULT_FRONTEND_INDUSTRY_CODE as INTERVIEW_DEFAULT_INDUSTRY_CODE,
  formatFrontendIndustryLabel,
  persistSelectedFrontendIndustryCode,
  readSelectedFrontendIndustryCode,
  resolvePreferredFrontendIndustry,
} from '../../shared/industryContext'
import { useFrontendIndustriesQuery } from '../../shared/frontendQueries'
import { buildInterviewHistoryQueryKey, invalidateInterviewHistoryQueries } from '../../shared/queryKeys'
import { createInterviewRequest, fetchInterviewHistory } from './interviewApi'
import {
  buildDefaultInterviewTopics,
  buildInitialInterviewForm,
  formatInterviewDateTime,
  interviewDifficultyLabel,
  interviewStatusLabel,
  parseInterviewTopics,
} from './interviewHelpers'
import type { InterviewConfigForm } from './interviewTypes'

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
  const industriesQuery = useFrontendIndustriesQuery()

  const historyQuery = useQuery({
    queryKey: buildInterviewHistoryQueryKey(accessToken),
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
      await invalidateInterviewHistoryQueries(queryClient)
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
                <AsyncInlineState
                  className="companion-empty-text"
                  message={extractErrorMessage(industriesQuery.error, '行业列表读取失败，当前将回退到默认方向。')}
                  tone="error"
                />
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

          {historyQuery.isLoading ? <AsyncInlineState className="companion-empty-text" message="面试历史加载中..." /> : null}
          {historyQuery.isError ? (
            <AsyncInlineState
              className="companion-empty-text"
              message={historyQuery.error instanceof Error ? historyQuery.error.message : '面试历史加载失败'}
              tone="error"
            />
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
            <AsyncEmptyState
              title="还没有面试记录"
              message="从左侧创建第一场模拟面试，系统会自动保存历史记录和后续报告。"
            />
          ) : null}
        </section>
      </div>
    </section>
  )
}

export { InterviewSessionPage } from './InterviewSessionPage'
export { InterviewReportPage } from './InterviewReportPage'
