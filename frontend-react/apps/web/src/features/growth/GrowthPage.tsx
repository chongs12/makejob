import { useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Link, useNavigate } from '@tanstack/react-router'
import { extractErrorMessage, requestJson } from '@makejob/api-client'
import { isSuccessCode, type ApiEnvelope } from '@makejob/shared-types'
import { useAuthStore } from '../../state/auth'
import { AsyncEmptyState, AsyncInlineState, AsyncStatusCard } from '../../shared/asyncState'
import { buildGrowthCompanionContextDraft, persistCompanionPlanContext } from '../../shared/companionContext'
import { DEFAULT_FRONTEND_INDUSTRY_CODE, readSelectedFrontendIndustryCode } from '../../shared/industryContext'
import { requestLoginPrompt } from '../../shared/loginPrompt'
import { fetchMistakeTopics, pickMistakeTopicsByTags, resolveMistakeTopicRoute } from '../../shared/mistakeTopics'
import {
  fetchPracticeRecommendations,
  resolvePracticeRecommendationModeLabel,
  resolvePracticeRecommendationRoute,
  resolvePracticeRecommendationSourceLabel,
} from '../../shared/practiceRecommendations'
import {
  buildPracticeRouteSearch,
  buildPracticeRecommendationRouteSearch,
  buildWeeklyFocusPracticeRouteSearch,
  resolvePracticeQuestionSetTitle,
} from '../../shared/practiceRoute'
import { fetchWeeklyFocus } from '../../shared/weeklyFocus'

interface GrowthCategoryStat {
  category_id: number
  category_name: string
  total: number
  correct: number
  accuracy_rate: number
}

interface GrowthPracticeStats {
  total_answered: number
  correct_count: number
  wrong_count: number
  accuracy_rate: number
  today_count: number
  streak_days: number
  category_stats: GrowthCategoryStat[]
}

interface GrowthStudyLog {
  id: number
  date_key: string
  summary: string
  focus_task_title: string
  completed_count: number
  skipped_count: number
  completed_titles: string[]
  skipped_titles: string[]
  latest_action_text: string
  updated_at: string
}

interface GrowthInterviewSnapshot {
  id: number
  status: string
  score: number
  total_questions: number
  created_at?: string
  ended_at?: string
}

interface GrowthPlanSnapshot {
  id: number
  title: string
  status: string
  total_tasks: number
  completed_tasks: number
  progress: number
  start_date?: string
  end_date?: string
}

interface GrowthCurrentPlan {
  id: number
  title: string
  status: string
  total_tasks: number
  completed_tasks: number
  progress: number
  next_task_title: string
  next_task_source?: string
  next_task_reason?: string
  next_task_source_ref?: string
  next_task_collection_hint?: string
}

interface GrowthFocusSignal {
  focus_tag: string
  topic_code?: string
  topic_title?: string
  topic_problem_pattern?: string
  related_question_sets: string[]
  recommended_actions: string[]
  primary_question_set?: string
  dominant_archive_phase?: string
  dominant_archive_phase_label?: string
  occurrence_count: number
  archive_occurrence_count: number
  interview_occurrence_count: number
  source: string
  source_label: string
  reason: string
}

interface GrowthTrendSummary {
  dominant_source: string
  dominant_source_label: string
  top_focus_tag?: string
  top_topic_code?: string
  top_topic_title?: string
  summary: string
}

interface GrowthSummaryResponse {
  practice_stats?: GrowthPracticeStats | null
  study_days: number
  interview_count: number
  completed_interview_count: number
  average_interview_score: number
  plan_count: number
  current_plan?: GrowthCurrentPlan | null
  focus_signals: GrowthFocusSignal[]
  trend_summary?: GrowthTrendSummary | null
  recent_study_logs: GrowthStudyLog[]
  recent_interviews: GrowthInterviewSnapshot[]
  recent_plans: GrowthPlanSnapshot[]
}

/**
 * 拉取成长档案页需要的聚合摘要数据。
 */
async function fetchGrowthSummary(token: string): Promise<GrowthSummaryResponse> {
  const response = await requestJson<ApiEnvelope<GrowthSummaryResponse>>('/user/growth-summary', {
    token,
  })

  if (!isSuccessCode(response.code) || !response.data) {
    throw new Error(response.message || '获取成长档案失败')
  }

  return response.data
}

/**
 * 将时间值格式化为成长档案页更易读的中文时间文本。
 */
function formatGrowthDateTime(value?: string): string {
  if (!value) {
    return '--'
  }

  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return value
  }

  return date.toLocaleString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  })
}

/**
 * 将日期字符串压缩成适合卡片展示的日期文案。
 */
function formatGrowthDate(value?: string): string {
  if (!value) {
    return '--'
  }

  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return value
  }

  return date.toLocaleDateString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
  })
}

/**
 * 将学习计划状态转换为成长档案页使用的中文标签。
 */
function growthPlanStatusLabel(status: string): string {
  const labelMap: Record<string, string> = {
    active: '进行中',
    paused: '已暂停',
    completed: '已完成',
    draft: '草稿',
  }

  return labelMap[status] || status || '未定义'
}

/**
 * 将面试状态转换为成长档案页使用的中文标签。
 */
function growthInterviewStatusLabel(status: string): string {
  const labelMap: Record<string, string> = {
    ongoing: '进行中',
    completed: '已完成',
    cancelled: '已取消',
  }

  return labelMap[status] || status || '未定义'
}

/**
 * 格式化数值，避免空值和小数展示不稳定。
 */
function formatGrowthScore(score: number): string {
  if (!Number.isFinite(score)) {
    return '--'
  }

  return Number(score).toFixed(1)
}

/**
 * 生成最近学习日志的摘要句子，避免列表项视觉上过于松散。
 */
function buildGrowthLogSummary(log: GrowthStudyLog): string {
  if (log.summary.trim()) {
    return log.summary.trim()
  }

  const fragments = [`完成 ${log.completed_count} 项`]
  if (log.skipped_count > 0) {
    fragments.push(`跳过 ${log.skipped_count} 项`)
  }
  if (log.focus_task_title.trim()) {
    fragments.push(`聚焦「${log.focus_task_title.trim()}」`)
  }

  return fragments.join('，')
}

/**
 * 将题单 slug 数组压缩成适合卡片展示的中文标题列表。
 */
function formatGrowthQuestionSets(questionSets: string[]): string {
  const labels = questionSets
    .map((item) => resolvePracticeQuestionSetTitle(item))
    .filter(Boolean)

  return labels.join('、')
}

/**
 * 将学习计划任务来源编码转换为成长档案页可直接展示的中文标签。
 */
function resolveGrowthTaskSourceLabel(source?: string): string {
  const normalizedSource = String(source || '').trim()
	const labelMap: Record<string, string> = {
		weak_topic: '当前弱项',
		goal: '阶段目标',
		default: '默认计划任务',
		practice_recommendation: '练习推荐',
		weekly_focus: '本周重点补强',
		plan_feedback_diagnosis: '训练反馈诊断',
	}

  return labelMap[normalizedSource] || normalizedSource || '未标注来源'
}

/**
 * 读取当前前台方向偏好，供成长档案把趋势和弱项继续带入学习陪伴页。
 */
function resolveGrowthCompanionIndustryCode(): string {
  return readSelectedFrontendIndustryCode().trim() || DEFAULT_FRONTEND_INDUSTRY_CODE
}

/**
 * 输出成长档案主页面，集中展示用户的练习、面试、计划和每日推进轨迹。
 */
export default function GrowthPage() {
  const navigate = useNavigate()
  const accessToken = useAuthStore((state) => state.accessToken)

  const growthSummaryQuery = useQuery({
    queryKey: ['growth-summary', accessToken],
    queryFn: () => fetchGrowthSummary(accessToken as string),
    enabled: Boolean(accessToken),
    retry: false,
  })

  const practiceRecommendationsQuery = useQuery({
    queryKey: ['growth-practice-recommendations', accessToken],
    queryFn: () => fetchPracticeRecommendations(accessToken as string, 4),
    enabled: Boolean(accessToken),
    retry: false,
  })

  const weeklyFocusQuery = useQuery({
    queryKey: ['growth-weekly-focus', accessToken],
    queryFn: () => fetchWeeklyFocus(accessToken as string),
    enabled: Boolean(accessToken),
    retry: false,
  })

  const mistakeTopicsQuery = useQuery({
    queryKey: ['growth-mistake-topics'],
    queryFn: () => fetchMistakeTopics([]),
    staleTime: 5 * 60 * 1000,
  })

  const mistakeTopicMap = useMemo(
    () => new Map((mistakeTopicsQuery.data || []).map((topic) => [topic.code, topic])),
    [mistakeTopicsQuery.data],
  )

  const topCategoryStats = useMemo(
    () => (growthSummaryQuery.data?.practice_stats?.category_stats || []).slice(0, 4),
    [growthSummaryQuery.data?.practice_stats?.category_stats],
  )
  const focusTopics = useMemo(
    () => pickMistakeTopicsByTags(practiceRecommendationsQuery.data?.focus_tags || [], mistakeTopicsQuery.data || []),
    [practiceRecommendationsQuery.data?.focus_tags, mistakeTopicsQuery.data],
  )
  const weeklyFocusTopicMap = useMemo(
    () =>
      new Map(
        (weeklyFocusQuery.data?.themes || [])
          .map((theme) => {
            const topicCode = theme.topic_codes[0]
            return [theme.title, topicCode ? mistakeTopicMap.get(topicCode) || null : null] as const
          }),
      ),
    [mistakeTopicMap, weeklyFocusQuery.data?.themes],
  )

  /**
   * 根据补强主题构造正式题库路由并跳转到刷题页，减少用户手动重新组织筛选条件。
   */
  function handleOpenWeeklyFocusPractice(themeTitle: string): void {
    const theme = weeklyFocusQuery.data?.themes.find((item) => item.title === themeTitle)
    if (!theme) {
      navigate({ to: '/practice' })
      return
    }

    const linkedTopic = weeklyFocusTopicMap.get(themeTitle)
    navigate({
      to: '/practice',
      search: buildWeeklyFocusPracticeRouteSearch(theme, linkedTopic),
    })
  }

  /**
   * 将成长档案里的趋势、弱项和建议压缩成陪伴页可直接消费的计划上下文。
   */
  function handleCompanionFollowUp(options: {
    summary: string
    focusTitle: string
    weakTopics: string[]
    suggestions: string[]
  }): void {
    const industryCode = resolveGrowthCompanionIndustryCode()
    persistCompanionPlanContext(buildGrowthCompanionContextDraft({
      industryCode,
      industryLabel: industryCode.toUpperCase(),
      summary: options.summary,
      focusTitle: options.focusTitle,
      weakTopics: options.weakTopics,
      suggestions: options.suggestions,
    }))
    navigate({
      to: '/companion',
    })
  }

  return (
    <section className="page-panel">
      <span className="page-tag">成长档案</span>
      <h1>把练习、面试、计划和每日推进沉淀成一份可回看的成长记录</h1>
      <p className="page-copy">
        这里不再只是登录后的占位工作台，而是你每天刷题、面试、学习陪伴推进结果的聚合页。后续继续做功能时，这里也会成为最稳定的个人闭环首页。
      </p>

      {!accessToken ? (
        <AsyncEmptyState
          title="登录后查看成长档案"
          message="成长档案会把练习、面试、学习计划和每日推进统一沉淀到这里，登录后才能看到你的趋势解释和下一步建议。"
          style={{ marginTop: 24 }}
          action={(
            <button className="secondary-button" type="button" onClick={() => requestLoginPrompt('/growth', 'missing')}>
              去登录
            </button>
          )}
        />
      ) : null}

      {growthSummaryQuery.isLoading ? <AsyncStatusCard message="成长档案加载中..." style={{ marginTop: 24 }} /> : null}

      {growthSummaryQuery.isError ? (
        <AsyncStatusCard
          message={extractErrorMessage(growthSummaryQuery.error, '成长档案读取失败，请稍后重试')}
          style={{ marginTop: 24 }}
          tone="error"
        />
      ) : null}

      {growthSummaryQuery.data ? (
        <>
          <div className="grid-cards" style={{ marginTop: 24 }}>
            <article className="feature-card">
              <h2>累计学习天数</h2>
              <strong>{growthSummaryQuery.data.study_days}</strong>
              <p>只要学习陪伴页有当日推进记录，这里就会累计沉淀下来。</p>
            </article>

            <article className="feature-card">
              <h2>连续答题天数</h2>
              <strong>{growthSummaryQuery.data.practice_stats?.streak_days || 0}</strong>
              <p>今日已答 {growthSummaryQuery.data.practice_stats?.today_count || 0} 题。</p>
            </article>

            <article className="feature-card">
              <h2>累计面试场次</h2>
              <strong>{growthSummaryQuery.data.interview_count}</strong>
              <p>其中已完成 {growthSummaryQuery.data.completed_interview_count} 场。</p>
            </article>

            <article className="feature-card">
              <h2>最近完成面试均分</h2>
              <strong>
                {growthSummaryQuery.data.completed_interview_count > 0
                  ? formatGrowthScore(growthSummaryQuery.data.average_interview_score)
                  : '--'}
              </strong>
              <p>只统计已完成面试，便于观察最近输出质量。</p>
            </article>
          </div>

          <div className="grid-cards" style={{ marginTop: 24 }}>
            <article className="status-card">
              <div className="card-inline">
                <div>
                  <span className="section-kicker">当前主计划</span>
                  <h2>{growthSummaryQuery.data.current_plan?.title || '暂时没有进行中的计划'}</h2>
                </div>
                <span>{growthSummaryQuery.data.current_plan ? `${Math.round(growthSummaryQuery.data.current_plan.progress)}%` : '--'}</span>
              </div>

              {growthSummaryQuery.data.current_plan ? (
                <>
                  <p>状态：{growthPlanStatusLabel(growthSummaryQuery.data.current_plan.status)}</p>
                  <p>
                    任务进度：{growthSummaryQuery.data.current_plan.completed_tasks}/{growthSummaryQuery.data.current_plan.total_tasks}
                  </p>
                  <p>下一步最值得推进：{growthSummaryQuery.data.current_plan.next_task_title || '当前没有待推进任务'}</p>
                  {growthSummaryQuery.data.current_plan.next_task_source ? (
                    <p>任务来源：{resolveGrowthTaskSourceLabel(growthSummaryQuery.data.current_plan.next_task_source)}</p>
                  ) : null}
                  {growthSummaryQuery.data.current_plan.next_task_reason ? (
                    <p>安排原因：{growthSummaryQuery.data.current_plan.next_task_reason}</p>
                  ) : null}
                  {growthSummaryQuery.data.current_plan.next_task_collection_hint ? (
                    <p>建议题单：{resolvePracticeQuestionSetTitle(growthSummaryQuery.data.current_plan.next_task_collection_hint)}</p>
                  ) : null}
                  {growthSummaryQuery.data.current_plan.next_task_source_ref ? (
                    <p>来源引用：{growthSummaryQuery.data.current_plan.next_task_source_ref}</p>
                  ) : null}
                  <div className="page-actions">
                    {growthSummaryQuery.data.current_plan.next_task_collection_hint ? (
                      <Link
                        className="secondary-link"
                        to="/practice"
                        search={buildPracticeRouteSearch({
                          questionSetSlug: growthSummaryQuery.data.current_plan.next_task_collection_hint,
                          source: 'practice_recommendation',
                          title: growthSummaryQuery.data.current_plan.next_task_title,
                          reason: growthSummaryQuery.data.current_plan.next_task_reason,
                        })}
                      >
                        按建议题单去补练
                      </Link>
                    ) : null}
                    <button
                      className="secondary-button"
                      type="button"
                      onClick={() => handleCompanionFollowUp({
                        summary: growthSummaryQuery.data.current_plan?.next_task_reason || '继续围绕当前主计划的下一任务推进，并根据最近趋势收口学习节奏。',
                        focusTitle: growthSummaryQuery.data.current_plan?.next_task_title || growthSummaryQuery.data.current_plan?.title || '当前主计划',
                        weakTopics: [
                          growthSummaryQuery.data.current_plan?.next_task_title || '',
                          growthSummaryQuery.data.current_plan?.next_task_reason || '',
                        ],
                        suggestions: [growthSummaryQuery.data.current_plan?.next_task_reason || '优先推进当前计划里的下一项任务。'],
                      })}
                    >
                      带着当前计划回到陪伴页
                    </button>
                  </div>
                </>
              ) : (
                <>
                  <p>如果你已经有面试报告和练习记录，下一步最值得继续用学习陪伴页把计划执行闭环补齐。</p>
                  <div className="page-actions">
                    <button
                      className="secondary-button"
                      type="button"
                      onClick={() => handleCompanionFollowUp({
                        summary: growthSummaryQuery.data.trend_summary?.summary || '根据成长档案里的最近练习和面试结果，先整理一份可执行的学习计划。',
                        focusTitle: growthSummaryQuery.data.trend_summary?.top_topic_title || growthSummaryQuery.data.trend_summary?.top_focus_tag || '当前趋势主线',
                        weakTopics: growthSummaryQuery.data.focus_signals.map((item) => item.focus_tag).slice(0, 4),
                        suggestions: growthSummaryQuery.data.focus_signals.flatMap((item) => item.recommended_actions || []).slice(0, 4),
                      })}
                    >
                      去生成学习计划
                    </button>
                  </div>
                </>
              )}
            </article>

            <article className="status-card">
              <div className="card-inline">
                <div>
                  <span className="section-kicker">练习概览</span>
                  <h2>题库主线数据</h2>
                </div>
                <span>{growthSummaryQuery.data.plan_count} 份计划</span>
              </div>
              <p>累计答题：{growthSummaryQuery.data.practice_stats?.total_answered || 0}</p>
              <p>答对：{growthSummaryQuery.data.practice_stats?.correct_count || 0}</p>
              <p>答错：{growthSummaryQuery.data.practice_stats?.wrong_count || 0}</p>
              <p>正确率：{formatGrowthScore(growthSummaryQuery.data.practice_stats?.accuracy_rate || 0)}%</p>
              <div className="page-actions">
                <Link className="secondary-link" to="/practice">继续刷题</Link>
                <Link className="secondary-link" to="/practice/wrong">复盘错题</Link>
              </div>
            </article>
          </div>

          {growthSummaryQuery.data.trend_summary?.summary || growthSummaryQuery.data.focus_signals?.length ? (
            <article className="status-card" style={{ marginTop: 24 }}>
              <div className="card-inline">
                <div>
                  <span className="section-kicker">近期趋势</span>
                  <h2>把最近练习与面试的变化压缩成可执行信号</h2>
                </div>
                <button
                  className="secondary-button"
                  type="button"
                  onClick={() => handleCompanionFollowUp({
                    summary: growthSummaryQuery.data.trend_summary?.summary || '根据最近趋势信号重新收束学习计划。',
                    focusTitle: growthSummaryQuery.data.trend_summary?.top_topic_title || growthSummaryQuery.data.trend_summary?.top_focus_tag || '当前趋势主线',
                    weakTopics: growthSummaryQuery.data.focus_signals.map((item) => item.focus_tag).slice(0, 5),
                    suggestions: growthSummaryQuery.data.focus_signals.flatMap((item) => item.recommended_actions || []).slice(0, 4),
                  })}
                >
                  据此调整计划
                </button>
              </div>
              {growthSummaryQuery.data.trend_summary?.summary ? (
                <p>{growthSummaryQuery.data.trend_summary.summary}</p>
              ) : null}
              {growthSummaryQuery.data.trend_summary?.dominant_source_label ? (
                <p>当前主导来源：{growthSummaryQuery.data.trend_summary.dominant_source_label}</p>
              ) : null}
              {growthSummaryQuery.data.trend_summary?.top_topic_title ? (
                <p>当前最值得持续追打的专题：{growthSummaryQuery.data.trend_summary.top_topic_title}</p>
              ) : null}
              {growthSummaryQuery.data.trend_summary?.top_focus_tag ? (
                <p>当前最高频问题标签：{growthSummaryQuery.data.trend_summary.top_focus_tag}</p>
              ) : null}
              {growthSummaryQuery.data.focus_signals?.length ? (
                <div className="grid-cards" style={{ marginTop: 18 }}>
                  {growthSummaryQuery.data.focus_signals.map((item, index) => (
                    <article className="feature-card" key={`growth-focus-signal-${item.focus_tag}-${index}`}>
                      <div className="card-inline">
                        <strong>{item.topic_title || item.focus_tag}</strong>
                        <span>{item.source_label || '趋势信号'}</span>
                      </div>
                      {item.dominant_archive_phase_label ? <p>主导阶段：{item.dominant_archive_phase_label}</p> : null}
                      <p>聚焦标签：{item.focus_tag}</p>
                      <p>最近出现 {item.occurrence_count} 次，其中练习暴露 {item.archive_occurrence_count} 次、面试暴露 {item.interview_occurrence_count} 次</p>
                      {item.reason ? <p>{item.reason}</p> : null}
                      {item.topic_problem_pattern ? <p>问题模式：{item.topic_problem_pattern}</p> : null}
                      {item.primary_question_set ? <p>优先题单：{resolvePracticeQuestionSetTitle(item.primary_question_set)}</p> : null}
                      {item.related_question_sets?.length ? <p>关联题单：{formatGrowthQuestionSets(item.related_question_sets)}</p> : null}
                      {item.recommended_actions?.length ? (
                        <ul className="interview-bullet-list" style={{ marginTop: 12 }}>
                          {item.recommended_actions.map((action) => (
                            <li key={`${item.focus_tag}-${action}`}>{action}</li>
                          ))}
                        </ul>
                      ) : null}
                      <div className="page-actions">
                        <Link
                          className="secondary-link"
                          to="/practice"
                          search={buildPracticeRouteSearch({
                            questionSetSlug: item.primary_question_set || item.related_question_sets?.[0] || '',
                            topicCode: item.topic_code,
                            focusTags: [item.focus_tag],
                            source: 'practice_recommendation',
                            title: item.topic_title || item.focus_tag,
                            reason: item.reason,
                          })}
                        >
                          去题库补练
                        </Link>
                        <button
                          className="secondary-button"
                          type="button"
                          onClick={() => handleCompanionFollowUp({
                            summary: item.reason || `围绕「${item.topic_title || item.focus_tag}」继续收束当前学习主线。`,
                            focusTitle: item.topic_title || item.focus_tag,
                            weakTopics: [item.focus_tag, item.topic_title || '', item.topic_problem_pattern || ''],
                            suggestions: item.recommended_actions || [],
                          })}
                        >
                          带入学习计划
                        </button>
                        {item.topic_code ? (
                          <Link
                            className="secondary-link"
                            to={resolveMistakeTopicRoute()}
                            params={{ topicCode: item.topic_code }}
                          >
                            打开专题
                          </Link>
                        ) : null}
                      </div>
                    </article>
                  ))}
                </div>
              ) : null}
            </article>
          ) : null}

          <article className="status-card" style={{ marginTop: 24 }}>
              <div className="card-inline">
                <div>
                  <span className="section-kicker">本周重点补强</span>
                  <h2>把最近反复暴露的问题压缩成 1 到 3 个主攻主题</h2>
                </div>
                <button
                  className="secondary-button"
                  type="button"
                  onClick={() => handleCompanionFollowUp({
                    summary: weeklyFocusQuery.data?.themes[0]?.reason || '把本周重点补强主题整理成一份连续执行的学习计划。',
                    focusTitle: weeklyFocusQuery.data?.themes[0]?.title || '本周重点补强',
                    weakTopics: weeklyFocusQuery.data?.themes.flatMap((theme) => [theme.title, ...theme.focus_tags]).slice(0, 6) || [],
                    suggestions: weeklyFocusQuery.data?.themes.flatMap((theme) => theme.suggestions || []).slice(0, 4) || [],
                  })}
                >
                  带入学习计划
                </button>
              </div>

            {weeklyFocusQuery.isLoading ? <AsyncInlineState message="正在整理你这周最该优先补强的主题..." style={{ marginTop: 18 }} /> : null}

            {weeklyFocusQuery.isError ? (
              <AsyncInlineState
                message={extractErrorMessage(weeklyFocusQuery.error, '本周补强主题加载失败')}
                style={{ marginTop: 18 }}
                tone="error"
              />
            ) : null}

            {weeklyFocusQuery.data?.themes.length ? (
              <div className="grid-cards" style={{ marginTop: 18 }}>
                {weeklyFocusQuery.data.themes.map((theme) => {
                  const linkedTopic = weeklyFocusTopicMap.get(theme.title)
                  return (
                    <article className="feature-card" key={`growth-weekly-focus-${theme.title}`}>
                      <div className="card-inline">
                        <strong>{theme.title}</strong>
                        <span>{theme.source_label}</span>
                      </div>
                      {theme.dominant_archive_phase_label ? <p>主导阶段：{theme.dominant_archive_phase_label}</p> : null}
                      <p>{theme.reason}</p>
                      {(theme.occurrence_count > 0 || theme.interview_occurrence_count > 0) ? (
                        <p>
                          最近出现 {theme.occurrence_count} 次，其中面试暴露 {theme.interview_occurrence_count} 次
                        </p>
                      ) : null}
                      {theme.focus_tags.length ? (
                        <div className="community-tag-row">
                          {theme.focus_tags.map((tag) => (
                            <span key={`${theme.title}-${tag}`}>{tag}</span>
                          ))}
                        </div>
                      ) : null}
                      {theme.related_question_sets?.length ? (
                        <p style={{ marginTop: 12 }}>
                          关联题单：{formatGrowthQuestionSets(theme.related_question_sets)}
                        </p>
                      ) : null}
                      {theme.suggestions.length ? (
                        <ul className="interview-bullet-list" style={{ marginTop: 12 }}>
                          {theme.suggestions.map((item) => (
                            <li key={`${theme.title}-${item}`}>{item}</li>
                          ))}
                        </ul>
                      ) : null}
                      {linkedTopic ? <p style={{ marginTop: 12 }}>专题提示：{linkedTopic.problem_pattern}</p> : null}
                      <div className="page-actions">
                        <button
                          className="secondary-button"
                          type="button"
                          onClick={() => handleCompanionFollowUp({
                            summary: theme.reason,
                            focusTitle: theme.title,
                            weakTopics: [theme.title, ...theme.focus_tags],
                            suggestions: theme.suggestions,
                          })}
                        >
                          去生成补强计划
                        </button>
                        <button className="secondary-button" type="button" onClick={() => handleOpenWeeklyFocusPractice(theme.title)}>
                          去题库补练
                        </button>
                        {linkedTopic ? (
                          <Link
                            className="secondary-link"
                            to={resolveMistakeTopicRoute()}
                            params={{ topicCode: linkedTopic.code }}
                          >
                            打开专题
                          </Link>
                        ) : null}
                      </div>
                    </article>
                  )
                })}
              </div>
            ) : null}

            {!weeklyFocusQuery.isLoading && !weeklyFocusQuery.isError && !weeklyFocusQuery.data?.themes.length ? (
              <AsyncEmptyState
                title="本周还没有明确主攻主题"
                message="先做几道题或完成一场面试，学习档案和面试报告积累起来后，这里会自动帮你收束出本周最值得优先补强的方向。"
                style={{ marginTop: 18 }}
              />
            ) : null}
          </article>

          <article className="status-card" style={{ marginTop: 24 }}>
            <div className="card-inline">
              <div>
                <span className="section-kicker">最近最值得补的题</span>
                <h2>按最近错因直接安排下一轮练习</h2>
              </div>
              <Link className="secondary-link" to="/practice">进入题库</Link>
            </div>

            {practiceRecommendationsQuery.isLoading ? <AsyncInlineState message="正在生成你的对症练习推荐..." style={{ marginTop: 18 }} /> : null}

            {practiceRecommendationsQuery.isError ? (
              <AsyncInlineState
                message={extractErrorMessage(practiceRecommendationsQuery.error, '练习推荐加载失败')}
                style={{ marginTop: 18 }}
                tone="error"
              />
            ) : null}

            {practiceRecommendationsQuery.data?.focus_tags.length ? (
              <div className="community-tag-row" style={{ marginTop: 18 }}>
                {practiceRecommendationsQuery.data.focus_tags.map((tag) => (
                  <span key={tag}>{tag}</span>
                ))}
              </div>
            ) : null}

            {practiceRecommendationsQuery.data?.items.length ? (
              <div className="grid-cards" style={{ marginTop: 18 }}>
                {practiceRecommendationsQuery.data.items.map((item) => {
                  const linkedTopic = item.topic_code ? mistakeTopicMap.get(item.topic_code) || null : null
                  return (
                    <article className="feature-card" key={`growth-practice-recommendation-${item.question.id}`}>
                      <div className="card-inline">
                        <strong>{item.question.title}</strong>
                        <span>{item.focus_tag}</span>
                      </div>
                      {item.topic_title ? <p>专题：{item.topic_title}</p> : null}
                      {item.dominant_archive_phase_label ? <p>主导阶段：{item.dominant_archive_phase_label}</p> : null}
                      <p>{item.reason}</p>
                      {item.priority_explanation ? <p>优先级说明：{item.priority_explanation}</p> : null}
                      <p>推荐模式：{resolvePracticeRecommendationModeLabel(item.recommendation_mode)}</p>
                      <p>推荐来源：{resolvePracticeRecommendationSourceLabel(item.source_type)}</p>
                      <p>难度：{item.question.difficulty || '未标注'}</p>
                      {item.primary_question_set ? <p>优先题单：{resolvePracticeQuestionSetTitle(item.primary_question_set)}</p> : null}
                      {item.topic_problem_pattern ? <p>问题模式：{item.topic_problem_pattern}</p> : null}
                      {item.related_question_sets?.length ? <p>关联题单：{formatGrowthQuestionSets(item.related_question_sets)}</p> : null}
                      {item.recommended_actions?.length ? (
                        <ul className="interview-bullet-list" style={{ marginTop: 12 }}>
                          {item.recommended_actions.map((action) => (
                            <li key={`${item.question.id}-${action}`}>{action}</li>
                          ))}
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
                        去做这题
                      </Link>
                      {item.topic_code ? (
                        <Link
                          className="secondary-link"
                          to={resolveMistakeTopicRoute()}
                          params={{ topicCode: item.topic_code }}
                        >
                          看错因专题
                        </Link>
                      ) : null}
                    </div>
                    </article>
                  )
                })}
              </div>
            ) : null}

            {!practiceRecommendationsQuery.isLoading && !practiceRecommendationsQuery.isError && !practiceRecommendationsQuery.data?.items.length ? (
              <AsyncEmptyState
                title="还没有足够的推荐依据"
                message="先在题库里完成几道编程题或主观题，学习档案积累出错因标签后，这里会更准确地指出下一步补题方向。"
                style={{ marginTop: 18 }}
                action={(
                  <Link className="secondary-link" to="/practice">先去题库补练</Link>
                )}
              />
            ) : null}

            {focusTopics.length ? (
              <div className="grid-cards" style={{ marginTop: 18 }}>
                {focusTopics.map((topic) => (
                  <article className="feature-card" key={topic.code}>
                    <div className="card-inline">
                      <strong>{topic.title}</strong>
                      <span>{topic.tag}</span>
                    </div>
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
            ) : null}
          </article>

          <article className="status-card" style={{ marginTop: 24 }}>
            <div className="card-inline">
              <div>
                <span className="section-kicker">最近学习日志</span>
                <h2>每天推进了什么，这里会持续沉淀</h2>
              </div>
              <Link className="secondary-link" to="/companion">继续记录今天</Link>
            </div>

            {growthSummaryQuery.data.recent_study_logs.length ? (
              <div className="stack-list" style={{ marginTop: 18 }}>
                {growthSummaryQuery.data.recent_study_logs.map((log) => (
                  <article className="feature-card" key={log.id}>
                    <div className="card-inline">
                      <strong>{log.date_key}</strong>
                      <span>{formatGrowthDateTime(log.updated_at)}</span>
                    </div>
                    <p>{buildGrowthLogSummary(log)}</p>
                    <p>聚焦任务：{log.focus_task_title || '未记录'}</p>
                    <p>完成 {log.completed_count} 项，跳过 {log.skipped_count} 项</p>
                    <p>最新动作：{log.latest_action_text || '暂无'}</p>
                  </article>
                ))}
              </div>
            ) : (
              <div className="timeline-item" style={{ marginTop: 18 }}>
                <strong>还没有服务端学习日志</strong>
                <p>现在学习陪伴页会在你更新任务状态后自动同步每日摘要，后续这里会逐天沉淀。</p>
                <div className="page-actions" style={{ marginTop: 12 }}>
                  <Link className="secondary-link" to="/companion">去陪伴页记录今天</Link>
                </div>
              </div>
            )}
          </article>

          <div className="grid-cards" style={{ marginTop: 24 }}>
            <article className="status-card">
              <div className="card-inline">
                <div>
                  <span className="section-kicker">最近面试</span>
                  <h2>最近几场模拟输出</h2>
                </div>
                <Link className="secondary-link" to="/interview">进入面试页</Link>
              </div>

              {growthSummaryQuery.data.recent_interviews.length ? (
                <div className="stack-list" style={{ marginTop: 18 }}>
                  {growthSummaryQuery.data.recent_interviews.map((interview) => (
                    <article className="feature-card" key={interview.id}>
                      <div className="card-inline">
                        <strong>面试 #{interview.id}</strong>
                        <span>{growthInterviewStatusLabel(interview.status)}</span>
                      </div>
                      <p>得分：{formatGrowthScore(interview.score)}</p>
                      <p>题目数：{interview.total_questions}</p>
                      <p>开始时间：{formatGrowthDateTime(interview.created_at)}</p>
                      <div className="page-actions">
                        <Link className="secondary-link" to={interview.status === 'ongoing' ? '/interview/$interviewId' : '/interview/$interviewId/report'} params={{ interviewId: String(interview.id) }}>
                          查看详情
                        </Link>
                      </div>
                    </article>
                  ))}
                </div>
              ) : (
                <div className="timeline-item" style={{ marginTop: 18 }}>
                  <strong>还没有面试记录</strong>
                  <p>你可以先去 AI 面试页做一场完整模拟，成长档案会自动把记录聚合回来。</p>
                  <div className="page-actions" style={{ marginTop: 12 }}>
                    <Link className="secondary-link" to="/interview">去做一场面试</Link>
                  </div>
                </div>
              )}
            </article>

            <article className="status-card">
              <div className="card-inline">
                <div>
                  <span className="section-kicker">最近学习计划</span>
                  <h2>计划主线留痕</h2>
                </div>
                <Link className="secondary-link" to="/companion">去看计划</Link>
              </div>

              {growthSummaryQuery.data.recent_plans.length ? (
                <div className="stack-list" style={{ marginTop: 18 }}>
                  {growthSummaryQuery.data.recent_plans.map((plan) => (
                    <article className="feature-card" key={plan.id}>
                      <div className="card-inline">
                        <strong>{plan.title}</strong>
                        <span>{growthPlanStatusLabel(plan.status)}</span>
                      </div>
                      <p>
                        任务进度：{plan.completed_tasks}/{plan.total_tasks}
                      </p>
                      <p>完成度：{formatGrowthScore(plan.progress)}%</p>
                      <p>开始时间：{formatGrowthDate(plan.start_date)}</p>
                    </article>
                  ))}
                </div>
              ) : (
                <div className="timeline-item" style={{ marginTop: 18 }}>
                  <strong>还没有学习计划历史</strong>
                  <p>建议先在学习陪伴页生成一份主计划，让练习和面试之后的提升动作真正落地。</p>
                  <div className="page-actions" style={{ marginTop: 12 }}>
                    <Link className="secondary-link" to="/companion">去生成学习计划</Link>
                  </div>
                </div>
              )}
            </article>
          </div>

          <article className="status-card" style={{ marginTop: 24 }}>
            <div className="card-inline">
              <div>
                <span className="section-kicker">分类表现</span>
                <h2>最近最值得补的题型方向</h2>
              </div>
              <Link className="secondary-link" to="/practice">去题库补强</Link>
            </div>

            {topCategoryStats.length ? (
              <div className="grid-cards" style={{ marginTop: 18 }}>
                {topCategoryStats.map((item) => (
                  <article className="feature-card" key={item.category_id}>
                    <h2>{item.category_name}</h2>
                    <p>累计答题：{item.total}</p>
                    <p>答对：{item.correct}</p>
                    <p>正确率：{formatGrowthScore(item.accuracy_rate)}%</p>
                  </article>
                ))}
              </div>
            ) : (
                <div className="timeline-item" style={{ marginTop: 18 }}>
                  <strong>还没有分类统计</strong>
                  <p>等你在题库里产生更多答题记录后，这里会更准确地指出当前薄弱点。</p>
                  <div className="page-actions" style={{ marginTop: 12 }}>
                    <Link className="secondary-link" to="/practice">先去题库做题</Link>
                  </div>
                </div>
              )}
          </article>
        </>
      ) : null}
    </section>
  )
}
