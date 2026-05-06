import { useEffect, useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Link, useNavigate, useParams } from '@tanstack/react-router'
import { extractErrorMessage } from '@makejob/api-client'
import { useAuthStore } from '../../state/auth'
import { AsyncEmptyState, AsyncInlineState } from '../../shared/asyncState'
import {
  buildInterviewCompanionContextDraft,
  persistCompanionPlanContext,
} from '../../shared/companionContext'
import { persistCommunityDraft } from '../../shared/communityDraft'
import {
  DEFAULT_FRONTEND_INDUSTRY_CODE as INTERVIEW_DEFAULT_INDUSTRY_CODE,
  fetchFrontendIndustries,
  formatFrontendIndustryLabel,
  persistSelectedFrontendIndustryCode,
  readSelectedFrontendIndustryCode,
  resolvePreferredFrontendIndustry,
  subscribeFrontendIndustryCodeChange,
} from '../../shared/industryContext'
import { requestLoginPrompt } from '../../shared/loginPrompt'
import { fetchMistakeTopics, pickMistakeTopicsByTags, resolveMistakeTopicRoute } from '../../shared/mistakeTopics'
import {
  fetchPracticeRecommendations,
  resolvePracticeRecommendationModeLabel,
  resolvePracticeRecommendationRoute,
  resolvePracticeRecommendationSourceLabel,
} from '../../shared/practiceRecommendations'
import {
  buildInterviewFollowUpPracticeRouteSearch,
  buildPracticeRecommendationRouteSearch,
  resolvePracticeQuestionSetTitle,
} from '../../shared/practiceRoute'
import { fetchInterviewDetail, fetchInterviewReport } from './interviewApi'
import {
  buildInterviewReadiness,
  buildInterviewReplayItems,
  buildInterviewReviewDraft,
  formatInterviewDateTime,
  formatInterviewDuration,
  normalizeInterviewDimensions,
} from './interviewHelpers'

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
 * 渲染面试报告页，并把补题、陪伴计划和社区复盘串成后续动作链。
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
  const numericInterviewId = Number(interviewId)
  const report = reportQuery.data?.report || null
  const reportDuration = reportQuery.data?.duration_seconds || 0
  const reportCompletedAt = reportQuery.data?.completed_at
  const dimensionItems = useMemo(() => normalizeInterviewDimensions(report), [report])
  const strongestDimensions = dimensionItems.slice(0, 3)
  const weakestDimensions = [...dimensionItems].reverse().slice(0, 3)
  const codingDiagnostics = report?.coding_diagnostics || []
  const codingMistakeTags = useMemo(
    () => Array.from(new Set(codingDiagnostics.flatMap((item) => item.mistake_tags || []))),
    [codingDiagnostics],
  )
  const replayItems = useMemo(() => buildInterviewReplayItems(detailQuery.data?.messages || []), [detailQuery.data?.messages])
  const readiness = buildInterviewReadiness(report?.overall_score || 0)
  const reviewDraft = report ? buildInterviewReviewDraft(report, detailQuery.data, reportIndustryLabel) : null
  const practiceRecommendationsQuery = useQuery({
    queryKey: ['interview-practice-recommendations', accessToken, interviewId],
    queryFn: () => fetchPracticeRecommendations(accessToken as string, 4, numericInterviewId),
    enabled: Boolean(accessToken && Number.isFinite(numericInterviewId) && numericInterviewId > 0),
    retry: false,
    refetchOnWindowFocus: false,
  })
  const mistakeTopicsQuery = useQuery({
    queryKey: ['interview-mistake-topics'],
    queryFn: () => fetchMistakeTopics([]),
    enabled: Boolean(codingMistakeTags.length),
    retry: false,
    staleTime: 5 * 60 * 1000,
  })
  const codingMistakeTopics = useMemo(
    () => pickMistakeTopicsByTags(codingMistakeTags, mistakeTopicsQuery.data || []),
    [codingMistakeTags, mistakeTopicsQuery.data],
  )
  const mistakeTopicMap = useMemo(
    () => new Map((mistakeTopicsQuery.data || []).map((topic) => [topic.code, topic])),
    [mistakeTopicsQuery.data],
  )
  const primaryWeakKeyword = codingMistakeTags[0] || weakestDimensions[0]?.label || report?.weaknesses?.[0] || ''
  const primaryFollowUpTopic = codingMistakeTopics[0] || null

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
    navigate({
      to: '/practice',
      search: buildInterviewFollowUpPracticeRouteSearch(keyword, primaryFollowUpTopic),
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
          ...codingMistakeTags,
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

          {reportQuery.isLoading ? <AsyncInlineState className="companion-empty-text" message="报告加载中..." /> : null}
          {reportQuery.isError ? (
            <AsyncEmptyState
              title="报告暂不可用"
              message={reportQuery.error instanceof Error ? reportQuery.error.message : '面试报告加载失败'}
              action={<Link className="secondary-link" to="/interview/$interviewId" params={{ interviewId }}>返回面试页</Link>}
            />
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

              {codingDiagnostics.length ? (
                <article className="timeline-item">
                  <strong>编程题过程诊断</strong>
                  <div className="interview-report-sections">
                    {codingDiagnostics.map((item) => (
                      <article className="timeline-item" key={`coding-diagnosis-${item.question_index}`}>
                        <strong>第 {item.question_index + 1} 题 · {item.language || '未标注语言'} · {Math.round(item.score || 0)} 分</strong>
                        <p>{item.process_summary || '当前没有返回过程总结。'}</p>
                        <p>错因标签：{item.mistake_tags?.length ? item.mistake_tags.join('、') : '暂无'}</p>
                        <p>优势标签：{item.strength_tags?.length ? item.strength_tags.join('、') : '暂无'}</p>
                        <p>证据摘要：{item.evidence?.length ? item.evidence.join('；') : '暂无'}</p>
                        <p>建议动作：{item.suggestions?.length ? item.suggestions.join('；') : '暂无'}</p>
                      </article>
                    ))}
                  </div>
                </article>
              ) : null}

              {codingMistakeTopics.length ? (
                <article className="timeline-item">
                  <strong>错因专题卡</strong>
                  <div className="interview-report-sections" style={{ marginTop: 16 }}>
                    {codingMistakeTopics.map((topic) => (
                      <article className="timeline-item" key={topic.code}>
                        <strong>{topic.title}</strong>
                        <p>{topic.problem_pattern}</p>
                        <div className="page-actions">
                          <Link
                            className="secondary-link"
                            to={resolveMistakeTopicRoute()}
                            params={{ topicCode: topic.code }}
                          >
                            打开专题
                          </Link>
                        </div>
                      </article>
                    ))}
                  </div>
                </article>
              ) : null}

              <article className="timeline-item">
                <strong>针对这场面试的补题建议</strong>
                {practiceRecommendationsQuery.isLoading ? <AsyncInlineState message="正在生成面向本场面试的补题建议..." /> : null}
                {practiceRecommendationsQuery.isError ? (
                  <AsyncInlineState message={extractErrorMessage(practiceRecommendationsQuery.error, '补题建议加载失败')} tone="error" />
                ) : null}
                {practiceRecommendationsQuery.data?.focus_tags.length ? (
                  <div className="community-tag-row" style={{ marginTop: 12 }}>
                    {practiceRecommendationsQuery.data.focus_tags.map((tag) => (
                      <span key={tag}>{tag}</span>
                    ))}
                  </div>
                ) : null}
                {practiceRecommendationsQuery.data?.items.length ? (
                  <div className="interview-report-sections" style={{ marginTop: 16 }}>
                    {practiceRecommendationsQuery.data.items.map((item) => {
                      const linkedTopic = item.topic_code ? mistakeTopicMap.get(item.topic_code) || null : null
                      return (
                        <article className="timeline-item" key={`interview-practice-recommendation-${item.question.id}`}>
                          <strong>{item.question.title}</strong>
                          {item.topic_title ? <p>专题：{item.topic_title}</p> : null}
                          <p>聚焦标签：{item.focus_tag}</p>
                          <p>{item.reason}</p>
                          <p>推荐优先级：第 {item.priority} 位</p>
                          <p>推荐模式：{resolvePracticeRecommendationModeLabel(item.recommendation_mode)}</p>
                          <p>推荐来源：{resolvePracticeRecommendationSourceLabel(item.source_type)}</p>
                          {item.priority_explanation ? <p>优先级说明：{item.priority_explanation}</p> : null}
                          {item.primary_question_set ? <p>优先题单：{resolvePracticeQuestionSetTitle(item.primary_question_set)}</p> : null}
                          {item.topic_problem_pattern ? <p>问题模式：{item.topic_problem_pattern}</p> : null}
                          {item.related_question_sets?.length ? (
                            <p>关联题单：{item.related_question_sets.map((set) => resolvePracticeQuestionSetTitle(set)).filter(Boolean).join('、')}</p>
                          ) : null}
                          {item.recommended_actions?.length ? (
                            <ul className="interview-bullet-list">
                              {item.recommended_actions.map((action) => <li key={`${item.question.id}-${action}`}>{action}</li>)}
                            </ul>
                          ) : null}
                          <div className="page-actions">
                            <Link
                              className="secondary-link"
                              to="/practice"
                              search={buildPracticeRecommendationRouteSearch({
                                focus_tag: item.focus_tag,
                                topic_code: item.topic_code,
                                primary_question_set: item.primary_question_set,
                                reason: item.reason,
                                question_title: item.question.title,
                              }, linkedTopic)}
                          >
                            进入这组补练
                          </Link>
                          <Link
                            className="secondary-link"
                            to={resolvePracticeRecommendationRoute(item.question.type)}
                            params={{ questionId: String(item.question.id) }}
                          >
                            直接去补这题
                          </Link>
                          {item.topic_code ? (
                            <Link
                              className="secondary-link"
                              to={resolveMistakeTopicRoute()}
                              params={{ topicCode: item.topic_code }}
                            >
                              查看错因专题
                            </Link>
                          ) : null}
                        </div>
                        </article>
                      )
                    })}
                  </div>
                ) : null}
                {!practiceRecommendationsQuery.isLoading && !practiceRecommendationsQuery.isError && !practiceRecommendationsQuery.data?.items.length ? (
                  <AsyncInlineState message="当前这场面试还没有形成足够明确的补题推荐，可以先按弱项关键词去题库继续搜索练习。" />
                ) : null}
              </article>

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
