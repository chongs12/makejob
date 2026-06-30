import { useEffect, useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Link, useNavigate, useParams } from '@tanstack/react-router'
import { Button, Spin, Tag, Empty } from 'antd'
import {
  ArrowLeftOutlined,
  TrophyOutlined,
  CheckCircleOutlined,
  ClockCircleOutlined,
  FileTextOutlined,
  BulbOutlined,
  RocketOutlined,
  CopyOutlined,
  EditOutlined,
  RightOutlined,
  AimOutlined,
  FireOutlined,
} from '@ant-design/icons'
import { extractErrorMessage } from '@makejob/api-client'
import { useAuthStore } from '../../state/auth'
import {
  buildInterviewCompanionContextDraft,
  persistCompanionPlanContext,
} from '../../shared/companionContext'
import { persistCommunityDraft } from '../../shared/communityDraft'
import {
  DEFAULT_FRONTEND_INDUSTRY_CODE as INTERVIEW_DEFAULT_INDUSTRY_CODE,
  formatFrontendIndustryLabel,
  persistSelectedFrontendIndustryCode,
  readSelectedFrontendIndustryCode,
  resolvePreferredFrontendIndustry,
  subscribeFrontendIndustryCodeChange,
} from '../../shared/industryContext'
import { useFrontendIndustriesQuery } from '../../shared/frontendQueries'
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

const THEME = {
  bg: '#f8f9fa',
  cardBg: '#ffffff',
  primary: '#f97316',
  primaryLight: '#fff7ed',
  primaryDark: '#ea580c',
  textMain: '#1f2937',
  textSecondary: '#6b7280',
  textMuted: '#9ca3af',
  border: '#f3f4f6',
  borderHover: '#e5e7eb',
  shadow: '0 1px 2px rgba(0,0,0,0.05)',
  shadowCard: '0 4px 6px -1px rgba(0,0,0,0.07), 0 2px 4px -2px rgba(0,0,0,0.05)',
  shadowHover: '0 10px 15px -3px rgba(0,0,0,0.08), 0 4px 6px -4px rgba(0,0,0,0.05)',
  radius: 12,
  radiusSm: 8,
  success: '#22c55e',
  warning: '#f59e0b',
  danger: '#ef4444',
  accent: '#3b82f6',
  purple: '#8b5cf6',
}

const cardStyle = {
  background: THEME.cardBg,
  borderRadius: THEME.radius,
  border: `1px solid ${THEME.border}`,
  boxShadow: THEME.shadow,
  padding: '24px',
}

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
  const [reportMessage, setReportMessage] = useState('')

  const reportQuery = useQuery({
    queryKey: ['interview-report', accessToken, interviewId],
    queryFn: () => fetchInterviewReport(accessToken as string, interviewId),
    enabled: Boolean(accessToken && interviewId),
    retry: false,
    refetchOnWindowFocus: false,
    refetchInterval: (query) => (query.state.data?.status === 'report_generating' ? 3000 : false),
  })
  const detailQuery = useQuery({
    queryKey: ['interview-report-detail', accessToken, interviewId],
    queryFn: () => fetchInterviewDetail(accessToken as string, interviewId),
    enabled: Boolean(accessToken && interviewId),
    retry: false,
    refetchOnWindowFocus: false,
  })
  const industriesQuery = useFrontendIndustriesQuery()
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
    queryFn: () => fetchMistakeTopics([], accessToken),
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

  useEffect(() => {
    const unsubscribe = subscribeFrontendIndustryCodeChange((industryCode) => {
      setSelectedIndustryCode(industryCode || INTERVIEW_DEFAULT_INDUSTRY_CODE)
    })
    return unsubscribe
  }, [])

  useEffect(() => {
    if (!detailQuery.data?.industry_code) return
    persistSelectedFrontendIndustryCode(detailQuery.data.industry_code)
    setSelectedIndustryCode(detailQuery.data.industry_code)
  }, [detailQuery.data?.industry_code])

  function handlePracticeFollowUp(keyword: string): void {
    navigate({ to: '/practice', search: buildInterviewFollowUpPracticeRouteSearch(keyword, primaryFollowUpTopic) })
  }

  function handleCreateCommunityReview(): void {
    if (!reviewDraft) { setReportMessage('当前报告还未准备好复盘草稿。'); return }
    if (!useAuthStore.getState().accessToken) { requestLoginPrompt('/community/create', 'missing'); return }
    persistCommunityDraft(reviewDraft)
    navigate({ to: '/community/create' })
  }

  async function handleCopyReviewDraft(): Promise<void> {
    if (!reviewDraft) { setReportMessage('当前报告还未准备好复盘草稿。'); return }
    try {
      await copyInterviewText(reviewDraft.content)
      setReportMessage('复盘草稿正文已复制。')
    } catch (error) {
      setReportMessage(extractErrorMessage(error, '复制失败'))
    }
  }

  function handleCompanionFollowUp(): void {
    if (!report) { setReportMessage('报告还未加载完成。'); return }
    persistCompanionPlanContext(
      buildInterviewCompanionContextDraft({
        interviewId,
        industryCode: reportIndustryCode,
        industryLabel: reportIndustryLabel,
        overallScore: report.overall_score || 0,
        summary: report.summary || '',
        readinessLabel: readiness.label,
        weakTopics: [...codingMistakeTags, ...weakestDimensions.map((item) => item.label), ...(report.weaknesses || [])],
        suggestions: report.suggestions || [],
      }),
    )
    navigate({ to: '/companion' })
  }

  function handleCompanionRecommendationFollowUp(input: { focusTag: string; topicTitle?: string; reason: string; suggestions: string[] }): void {
    persistCompanionPlanContext(
      buildInterviewCompanionContextDraft({
        interviewId,
        industryCode: reportIndustryCode,
        industryLabel: reportIndustryLabel,
        overallScore: report?.overall_score || 0,
        summary: input.reason,
        readinessLabel: readiness.label,
        weakTopics: [input.topicTitle || '', input.focusTag],
        suggestions: input.suggestions,
      }),
    )
    navigate({ to: '/companion' })
  }

  const bulletListStyle: React.CSSProperties = { margin: 0, paddingLeft: 20, listStyle: 'disc' }
  const bulletItemStyle: React.CSSProperties = { fontSize: 14, color: THEME.textSecondary, lineHeight: 1.8, marginBottom: 4 }
  const sectionTitleStyle: React.CSSProperties = { fontSize: 16, fontWeight: 700, color: THEME.textMain, margin: '0 0 12px' }
  const sectionCardInnerStyle = { ...cardStyle, marginBottom: 16 }

  return (
    <div style={{ minHeight: '100vh', background: THEME.bg }}>
      {/* 顶部工具栏 */}
      <div style={{ background: THEME.cardBg, borderBottom: `1px solid ${THEME.border}`, boxShadow: THEME.shadow }}>
        <div style={{ maxWidth: 1200, margin: '0 auto', padding: '12px 24px', display: 'flex', alignItems: 'center', gap: 16 }}>
          <Link to="/interview" style={{ display: 'inline-flex', alignItems: 'center', gap: 6, fontSize: 14, color: THEME.textSecondary, textDecoration: 'none' }}>
            <ArrowLeftOutlined /> 返回面试入口
          </Link>
          <span style={{ fontSize: 13, color: THEME.textMuted }}>面试报告 · {reportIndustryLabel}</span>
        </div>
      </div>

      {/* 主内容区 */}
      <div style={{ maxWidth: 1200, margin: '0 auto', padding: '24px', display: 'grid', gridTemplateColumns: '1fr 300px', gap: 24 }}>
        {/* 左侧主内容 */}
        <div>
          {/* 标题 */}
          <div style={{ ...cardStyle, marginBottom: 24 }}>
            <div style={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', marginBottom: 16 }}>
              <div>
                <span style={{ display: 'inline-block', padding: '3px 10px', borderRadius: 6, background: THEME.primaryLight, color: THEME.primary, fontSize: 12, fontWeight: 600, marginBottom: 12 }}>面试报告</span>
                <h1 style={{ fontSize: 22, fontWeight: 700, color: THEME.textMain, margin: '0 0 4px' }}>面试 #{interviewId} 结果总结</h1>
                <p style={{ fontSize: 14, color: THEME.textMuted, margin: 0 }}>所属方向：{reportIndustryLabel}</p>
              </div>
              <span style={{ fontSize: 13, color: THEME.textMuted, flexShrink: 0 }}>
                {reportCompletedAt ? `完成于 ${formatInterviewDateTime(reportCompletedAt)}` : ''}
              </span>
            </div>

            {/* 加载/错误状态 */}
            {!accessToken && (
              <div style={{ textAlign: 'center', padding: '32px 0' }}>
                <p style={{ fontSize: 14, color: THEME.textSecondary, marginBottom: 16 }}>登录后查看面试报告</p>
                <Button type="primary" onClick={() => requestLoginPrompt(`/interview/${interviewId}/report`, 'missing')} style={{ background: THEME.primary, borderColor: THEME.primary, borderRadius: 8 }}>去登录</Button>
              </div>
            )}
            {accessToken && reportQuery.isLoading && (
              <div style={{ textAlign: 'center', padding: '32px 0' }}><Spin /><p style={{ fontSize: 14, color: THEME.textMuted, marginTop: 12 }}>报告加载中...</p></div>
            )}
            {reportQuery.isError && (
              <div style={{ textAlign: 'center', padding: '32px 0' }}>
                <p style={{ fontSize: 14, color: THEME.danger, marginBottom: 16 }}>{reportQuery.error instanceof Error ? reportQuery.error.message : '面试报告加载失败'}</p>
                <Link to="/interview/$interviewId" params={{ interviewId }}><Button style={{ borderRadius: 8 }}>返回面试页</Button></Link>
              </div>
            )}
            {reportQuery.data?.status === 'report_generating' && !report && (
              <div style={{ textAlign: 'center', padding: '32px 0' }}>
                <Spin />
                <p style={{ fontSize: 14, color: THEME.textSecondary, marginTop: 12 }}>{reportQuery.data.task_error || '系统正在生成报告，页面会自动刷新。'}</p>
              </div>
            )}
            {accessToken && !reportQuery.isLoading && !reportQuery.isError && reportQuery.data && !report && reportQuery.data.status !== 'report_generating' && (
              <div style={{ textAlign: 'center', padding: '32px 0' }}>
                <p style={{ fontSize: 14, color: THEME.textSecondary, marginBottom: 8 }}>
                  当前面试状态：{reportQuery.data.status || '未知'}
                </p>
                <p style={{ fontSize: 13, color: THEME.textMuted }}>
                  报告数据暂未返回，可能需要先完成面试或等待报告生成。
                </p>
                <Link to="/interview/$interviewId" params={{ interviewId }}>
                  <Button style={{ borderRadius: 8, marginTop: 12 }}>返回面试页</Button>
                </Link>
              </div>
            )}
          </div>

          {report && (
            <>
              {/* 核心指标 */}
              <div style={{ display: 'grid', gridTemplateColumns: 'repeat(4, 1fr)', gap: 16, marginBottom: 24 }}>
                {[
                  { icon: <TrophyOutlined />, value: Math.round(report.overall_score || 0), label: '总分', color: THEME.primary, bg: THEME.primaryLight },
                  { icon: <CheckCircleOutlined />, value: report.correct_count, label: '命中题数', color: THEME.success, bg: '#f0fdf4' },
                  { icon: <FileTextOutlined />, value: report.total_questions, label: '总题量', color: THEME.accent, bg: '#eff6ff' },
                  { icon: <ClockCircleOutlined />, value: formatInterviewDuration(reportDuration), label: '面试时长', color: THEME.purple, bg: '#f5f3ff' },
                ].map((item) => (
                  <div key={item.label} style={{ ...cardStyle, display: 'flex', alignItems: 'center', gap: 12, padding: '16px 20px' }}>
                    <div style={{ width: 40, height: 40, borderRadius: 10, background: item.bg, display: 'flex', alignItems: 'center', justifyContent: 'center', color: item.color, fontSize: 18 }}>
                      {item.icon}
                    </div>
                    <div>
                      <div style={{ fontSize: 22, fontWeight: 800, color: THEME.textMain, lineHeight: 1 }}>{item.value}</div>
                      <div style={{ fontSize: 12, color: THEME.textMuted, marginTop: 4 }}>{item.label}</div>
                    </div>
                  </div>
                ))}
              </div>

              {/* 准备度 + 补强项 + 下一步 */}
              <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr 1fr', gap: 16, marginBottom: 24 }}>
                <div style={cardStyle}>
                  <div style={{ fontSize: 12, color: THEME.textMuted, marginBottom: 4 }}>当前准备度</div>
                  <div style={{ fontSize: 18, fontWeight: 700, color: THEME.textMain, marginBottom: 8 }}>{readiness.label}</div>
                  <p style={{ fontSize: 13, color: THEME.textSecondary, margin: 0 }}>{readiness.description}</p>
                </div>
                <div style={cardStyle}>
                  <div style={{ fontSize: 12, color: THEME.textMuted, marginBottom: 4 }}>优先补强项</div>
                  {weakestDimensions.length ? (
                    <div style={{ display: 'flex', flexWrap: 'wrap', gap: 8 }}>
                      {weakestDimensions.map((item) => (
                        <Tag key={item.key} color="warning">{item.label} {item.score}分</Tag>
                      ))}
                    </div>
                  ) : <p style={{ fontSize: 13, color: THEME.textMuted, margin: 0 }}>暂无维度数据</p>}
                </div>
                <div style={cardStyle}>
                  <div style={{ fontSize: 12, color: THEME.textMuted, marginBottom: 12 }}>下一步动作</div>
                  <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
                    <Button size="small" icon={<AimOutlined />} onClick={() => handlePracticeFollowUp(primaryWeakKeyword || reportIndustryLabel)} style={{ borderRadius: 6, justifyContent: 'flex-start' }}>去题库补弱项</Button>
                    <Button size="small" icon={<RocketOutlined />} onClick={handleCompanionFollowUp} style={{ borderRadius: 6, justifyContent: 'flex-start' }}>去生成强化计划</Button>
                  </div>
                </div>
              </div>

              {/* 消息提示 */}
              {reportMessage && (
                <div style={{ ...cardStyle, marginBottom: 24, padding: '12px 20px', background: THEME.primaryLight, border: 'none', fontSize: 14, color: THEME.primary }}>{reportMessage}</div>
              )}

              {/* 总结 */}
              <div style={sectionCardInnerStyle}>
                <h3 style={sectionTitleStyle}>总结</h3>
                <p style={{ fontSize: 14, color: THEME.textSecondary, lineHeight: 1.8, margin: 0 }}>{report.summary || '当前报告未生成总结。'}</p>
              </div>

              {/* 编程题诊断 */}
              {codingDiagnostics.length > 0 && (
                <div style={sectionCardInnerStyle}>
                  <h3 style={sectionTitleStyle}>编程题过程诊断</h3>
                  <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
                    {codingDiagnostics.map((item) => (
                      <div key={`coding-diagnosis-${item.question_index}`} style={{ padding: '16px', borderRadius: THEME.radiusSm, background: '#fafaf9', border: `1px solid ${THEME.border}` }}>
                        <div style={{ fontSize: 14, fontWeight: 700, color: THEME.textMain, marginBottom: 8 }}>
                          第 {item.question_index + 1} 题 · {item.language || '未标注语言'} · {Math.round(item.score || 0)} 分
                        </div>
                        <p style={{ fontSize: 13, color: THEME.textSecondary, margin: '0 0 8px' }}>{item.process_summary || '当前没有返回过程总结。'}</p>
                        <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '8px 16px', fontSize: 13 }}>
                          <div><span style={{ color: THEME.textMuted }}>错因：</span><span style={{ color: THEME.textSecondary }}>{item.mistake_tags?.length ? item.mistake_tags.join('、') : '暂无'}</span></div>
                          <div><span style={{ color: THEME.textMuted }}>优势：</span><span style={{ color: THEME.textSecondary }}>{item.strength_tags?.length ? item.strength_tags.join('、') : '暂无'}</span></div>
                        </div>
                        {item.evidence?.length > 0 && <p style={{ fontSize: 13, color: THEME.textSecondary, margin: '8px 0 0' }}>证据：{item.evidence.join('；')}</p>}
                        {item.suggestions?.length > 0 && <p style={{ fontSize: 13, color: THEME.textSecondary, margin: '4px 0 0' }}>建议：{item.suggestions.join('；')}</p>}
                      </div>
                    ))}
                  </div>
                </div>
              )}

              {/* 错因专题卡 */}
              {codingMistakeTopics.length > 0 && (
                <div style={sectionCardInnerStyle}>
                  <h3 style={sectionTitleStyle}>错因专题卡</h3>
                  <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
                    {codingMistakeTopics.map((topic) => (
                      <div key={topic.code} style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '12px 16px', borderRadius: THEME.radiusSm, background: '#fafaf9', border: `1px solid ${THEME.border}` }}>
                        <div style={{ flex: 1 }}>
                          <div style={{ fontSize: 14, fontWeight: 600, color: THEME.textMain }}>{topic.title}</div>
                          <p style={{ fontSize: 13, color: THEME.textSecondary, margin: '4px 0 0' }}>{topic.problem_pattern}</p>
                        </div>
                        <Link to={resolveMistakeTopicRoute()} params={{ topicCode: topic.code }}>
                          <Button size="small" style={{ borderRadius: 6 }}>打开专题</Button>
                        </Link>
                      </div>
                    ))}
                  </div>
                </div>
              )}

              {/* 针对这场面试的补题建议 */}
              <div style={sectionCardInnerStyle}>
                <h3 style={sectionTitleStyle}>针对这场面试的补题建议</h3>
                {practiceRecommendationsQuery.isLoading && <div style={{ textAlign: 'center', padding: '16px 0' }}><Spin /><p style={{ fontSize: 13, color: THEME.textMuted, marginTop: 8 }}>正在生成补题建议...</p></div>}
                {practiceRecommendationsQuery.isError && <p style={{ fontSize: 13, color: THEME.danger }}>{extractErrorMessage(practiceRecommendationsQuery.error, '补题建议加载失败')}</p>}
                {practiceRecommendationsQuery.data?.focus_tags.length ? (
                  <div style={{ display: 'flex', flexWrap: 'wrap', gap: 8, marginBottom: 16 }}>
                    {practiceRecommendationsQuery.data.focus_tags.map((tag) => <Tag key={tag}>{tag}</Tag>)}
                  </div>
                ) : null}
                {practiceRecommendationsQuery.data?.items.length ? (
                  <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
                    {practiceRecommendationsQuery.data.items.map((item) => {
                      const linkedTopic = item.topic_code ? mistakeTopicMap.get(item.topic_code) || null : null
                      return (
                        <div key={`rec-${item.question.id}`} style={{ padding: '16px', borderRadius: THEME.radiusSm, border: `1px solid ${THEME.border}`, background: THEME.cardBg }}>
                          <div style={{ fontSize: 15, fontWeight: 700, color: THEME.textMain, marginBottom: 8 }}>{item.question.title}</div>
                          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '4px 16px', fontSize: 13, color: THEME.textSecondary, marginBottom: 12 }}>
                            {item.topic_title && <div><span style={{ color: THEME.textMuted }}>专题：</span>{item.topic_title}</div>}
                            <div><span style={{ color: THEME.textMuted }}>聚焦标签：</span>{item.focus_tag}</div>
                            <div><span style={{ color: THEME.textMuted }}>难度：</span>{item.question.difficulty || '未标注'}</div>
                            <div><span style={{ color: THEME.textMuted }}>优先级：</span>第 {item.priority} 位</div>
                          </div>
                          <p style={{ fontSize: 13, color: THEME.textSecondary, margin: '0 0 12px' }}>{item.reason}</p>
                          {item.recommended_actions?.length ? (
                            <ul style={{ ...bulletListStyle, marginBottom: 12 }}>
                              {item.recommended_actions.map((action) => <li key={`${item.question.id}-${action}`} style={bulletItemStyle}>{action}</li>)}
                            </ul>
                          ) : null}
                          <div style={{ display: 'flex', flexWrap: 'wrap', gap: 8 }}>
                            <Link to="/practice" search={buildPracticeRecommendationRouteSearch({ focus_tag: item.focus_tag, topic_code: item.topic_code, primary_question_set: item.primary_question_set, reason: item.reason, question_title: item.question.title }, linkedTopic)}>
                              <Button size="small" type="primary" style={{ borderRadius: 6, background: THEME.primary, borderColor: THEME.primary }}>进入这组补练</Button>
                            </Link>
                            <Link to={resolvePracticeRecommendationRoute(item.question.type)} params={{ questionId: String(item.question.id) }}>
                              <Button size="small" style={{ borderRadius: 6 }}>直接去补这题</Button>
                            </Link>
                            <Button size="small" onClick={() => handleCompanionRecommendationFollowUp({ focusTag: item.focus_tag, topicTitle: item.topic_title, reason: item.reason, suggestions: item.recommended_actions || [] })} style={{ borderRadius: 6 }}>带入学习计划</Button>
                            {item.topic_code && (
                              <Link to={resolveMistakeTopicRoute()} params={{ topicCode: item.topic_code }}>
                                <Button size="small" style={{ borderRadius: 6 }}>查看错因专题</Button>
                              </Link>
                            )}
                          </div>
                        </div>
                      )
                    })}
                  </div>
                ) : null}
                {!practiceRecommendationsQuery.isLoading && !practiceRecommendationsQuery.isError && !practiceRecommendationsQuery.data?.items.length && (
                  <p style={{ fontSize: 13, color: THEME.textMuted }}>当前这场面试还没有形成足够明确的补题推荐。</p>
                )}
              </div>

              {/* 优势 / 待加强 / 建议 */}
              <div style={sectionCardInnerStyle}>
                <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr 1fr', gap: 24 }}>
                  {[
                    { title: '优势', items: report.strengths, color: THEME.success },
                    { title: '待加强点', items: report.weaknesses, color: THEME.warning },
                    { title: '后续建议', items: report.suggestions, color: THEME.accent },
                  ].map((section) => (
                    <div key={section.title}>
                      <h4 style={{ fontSize: 14, fontWeight: 700, color: section.color, margin: '0 0 12px' }}>{section.title}</h4>
                      {section.items?.length ? (
                        <ul style={bulletListStyle}>
                          {section.items.map((item) => <li key={item} style={bulletItemStyle}>{item}</li>)}
                        </ul>
                      ) : <p style={{ fontSize: 13, color: THEME.textMuted }}>暂无数据</p>}
                    </div>
                  ))}
                </div>
              </div>

              {/* 维度评分 */}
              <div style={sectionCardInnerStyle}>
                <h3 style={sectionTitleStyle}>维度评分</h3>
                {dimensionItems.length ? (
                  <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(120px, 1fr))', gap: 12 }}>
                    {dimensionItems.map((item) => {
                      const scoreColor = item.score >= 80 ? THEME.success : item.score >= 60 ? THEME.warning : THEME.danger
                      return (
                        <div key={item.key} style={{ textAlign: 'center', padding: '16px 12px', borderRadius: THEME.radiusSm, background: '#fafaf9', border: `1px solid ${THEME.border}` }}>
                          <div style={{ fontSize: 24, fontWeight: 800, color: scoreColor, lineHeight: 1 }}>{item.score}</div>
                          <div style={{ fontSize: 12, color: THEME.textSecondary, marginTop: 6 }}>{item.label}</div>
                        </div>
                      )
                    })}
                  </div>
                ) : <p style={{ fontSize: 13, color: THEME.textMuted }}>当前报告没有返回维度评分。</p>}
                <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 16, marginTop: 16 }}>
                  <div>
                    <h4 style={{ fontSize: 13, fontWeight: 700, color: THEME.success, margin: '0 0 8px' }}>最强维度</h4>
                    {strongestDimensions.length ? (
                      <ul style={bulletListStyle}>{strongestDimensions.map((item) => <li key={item.key} style={bulletItemStyle}>{item.label} {item.score} 分</li>)}</ul>
                    ) : <p style={{ fontSize: 13, color: THEME.textMuted }}>暂无数据</p>}
                  </div>
                  <div>
                    <h4 style={{ fontSize: 13, fontWeight: 700, color: THEME.warning, margin: '0 0 8px' }}>优先补强维度</h4>
                    {weakestDimensions.length ? (
                      <ul style={bulletListStyle}>{weakestDimensions.map((item) => <li key={item.key} style={bulletItemStyle}>{item.label} {item.score} 分</li>)}</ul>
                    ) : <p style={{ fontSize: 13, color: THEME.textMuted }}>暂无数据</p>}
                  </div>
                </div>
              </div>

              {/* 社区复盘模板 */}
              <div style={sectionCardInnerStyle}>
                <div style={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', marginBottom: 16 }}>
                  <div>
                    <h3 style={{ ...sectionTitleStyle, margin: 0 }}>社区复盘模板</h3>
                    <p style={{ fontSize: 13, color: THEME.textMuted, margin: '4px 0 0' }}>把这场面试的结果整理成帖子，后续可以在社区里补充复盘和讨论。</p>
                  </div>
                  <div style={{ display: 'flex', gap: 8 }}>
                    <Button size="small" icon={<CopyOutlined />} onClick={() => void handleCopyReviewDraft()} style={{ borderRadius: 6 }}>复制草稿</Button>
                    <Button size="small" type="primary" icon={<EditOutlined />} onClick={handleCreateCommunityReview} style={{ borderRadius: 6, background: THEME.primary, borderColor: THEME.primary }}>去社区发复盘</Button>
                  </div>
                </div>
                {reviewDraft ? (
                  <div style={{ padding: '16px', borderRadius: THEME.radiusSm, background: '#fafaf9', border: `1px solid ${THEME.border}`, fontSize: 13, color: THEME.textSecondary, lineHeight: 1.8, whiteSpace: 'pre-wrap' }}>{reviewDraft.content}</div>
                ) : <p style={{ fontSize: 13, color: THEME.textMuted }}>当前没有可生成的复盘模板。</p>}
              </div>

              {/* 答题轨迹 */}
              <div style={sectionCardInnerStyle}>
                <h3 style={sectionTitleStyle}>答题轨迹</h3>
                {replayItems.length ? (
                  <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
                    {replayItems.map((item, index) => (
                      <div key={`${item.askedAt}-${index}`} style={{ padding: '16px', borderRadius: THEME.radiusSm, border: `1px solid ${THEME.border}` }}>
                        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 8 }}>
                          <span style={{ fontSize: 14, fontWeight: 700, color: THEME.textMain }}>第 {index + 1} 题</span>
                          <span style={{ fontSize: 12, color: THEME.textMuted }}>{formatInterviewDateTime(item.answeredAt || item.askedAt)}</span>
                        </div>
                        <p style={{ fontSize: 13, color: THEME.textMain, margin: '0 0 8px' }}><strong>问题：</strong>{item.question}</p>
                        <p style={{ fontSize: 13, color: THEME.textSecondary, margin: 0 }}><strong>回答：</strong>{item.answer || '本题未记录到用户回答。'}</p>
                      </div>
                    ))}
                  </div>
                ) : <p style={{ fontSize: 13, color: THEME.textMuted }}>当前没有可回放的答题轨迹。</p>}
              </div>
            </>
          )}
        </div>

        {/* 右侧边栏 */}
        <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
          <div style={cardStyle}>
            <span style={{ display: 'inline-block', padding: '3px 10px', borderRadius: 6, background: THEME.primaryLight, color: THEME.primary, fontSize: 12, fontWeight: 600, marginBottom: 16 }}>下一步建议</span>
            <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
              {[
                { icon: <AimOutlined />, label: '先去补题库弱项', onClick: () => handlePracticeFollowUp(primaryWeakKeyword || reportIndustryLabel) },
                { icon: <RocketOutlined />, label: '去学习陪伴继续推进计划', onClick: handleCompanionFollowUp },
                { icon: <FireOutlined />, label: '再开一场新的面试', onClick: () => navigate({ to: '/interview' }) },
                { icon: <EditOutlined />, label: '去社区发复盘', onClick: handleCreateCommunityReview },
              ].map((item) => (
                <button
                  key={item.label}
                  type="button"
                  onClick={item.onClick}
                  style={{ display: 'flex', alignItems: 'center', gap: 10, width: '100%', padding: '10px 12px', borderRadius: 8, border: `1px solid ${THEME.border}`, background: THEME.cardBg, cursor: 'pointer', fontSize: 14, color: THEME.textMain, textAlign: 'left', transition: 'all 0.15s ease' }}
                  onMouseEnter={(e) => { e.currentTarget.style.borderColor = THEME.primary; e.currentTarget.style.color = THEME.primary }}
                  onMouseLeave={(e) => { e.currentTarget.style.borderColor = THEME.border; e.currentTarget.style.color = THEME.textMain }}
                >
                  <span style={{ color: THEME.textMuted }}>{item.icon}</span>
                  {item.label}
                </button>
              ))}
            </div>
          </div>

          <div style={{ ...cardStyle, background: THEME.primaryLight, border: 'none' }}>
            <div style={{ fontSize: 13, fontWeight: 600, color: THEME.primaryDark, marginBottom: 8 }}>报告提示</div>
            <p style={{ fontSize: 13, color: THEME.textSecondary, margin: 0, lineHeight: 1.6 }}>
              这版报告会把强弱项、后续建议、题库补练和社区复盘串成一条动作链，建议优先处理最低分维度。
            </p>
          </div>
        </div>
      </div>
    </div>
  )
}
