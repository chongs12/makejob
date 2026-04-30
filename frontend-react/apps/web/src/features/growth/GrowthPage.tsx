import { useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Link, useNavigate } from '@tanstack/react-router'
import { extractErrorMessage, requestJson } from '@makejob/api-client'
import { isSuccessCode, type ApiEnvelope } from '@makejob/shared-types'
import { useAuthStore } from '../../state/auth'
import { fetchMistakeTopics, pickMistakeTopicsByTags, resolveMistakeTopicRoute } from '../../shared/mistakeTopics'
import { persistPracticeFocusSearch } from '../../shared/practiceFocus'
import { fetchPracticeRecommendations, resolvePracticeRecommendationRoute } from '../../shared/practiceRecommendations'
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
}

interface GrowthSummaryResponse {
  practice_stats?: GrowthPracticeStats | null
  study_days: number
  interview_count: number
  completed_interview_count: number
  average_interview_score: number
  plan_count: number
  current_plan?: GrowthCurrentPlan | null
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
   * 根据补强主题预填题库搜索词并跳转到刷题页，减少用户手动重新组织筛选条件。
   */
  function handleOpenWeeklyFocusPractice(themeTitle: string): void {
    const theme = weeklyFocusQuery.data?.themes.find((item) => item.title === themeTitle)
    if (!theme) {
      navigate({ to: '/practice' })
      return
    }

    const linkedTopic = weeklyFocusTopicMap.get(themeTitle)
    persistPracticeFocusSearch(linkedTopic?.related_question_sets[0] || '', theme.focus_tags, theme.title)
    navigate({ to: '/practice' })
  }

  return (
    <section className="page-panel">
      <span className="page-tag">成长档案</span>
      <h1>把练习、面试、计划和每日推进沉淀成一份可回看的成长记录</h1>
      <p className="page-copy">
        这里不再只是登录后的占位工作台，而是你每天刷题、面试、学习陪伴推进结果的聚合页。后续继续做功能时，这里也会成为最稳定的个人闭环首页。
      </p>

      {growthSummaryQuery.isLoading ? (
        <div className="status-card" style={{ marginTop: 24 }}>成长档案加载中...</div>
      ) : null}

      {growthSummaryQuery.isError ? (
        <div className="status-card" style={{ marginTop: 24 }}>
          {extractErrorMessage(growthSummaryQuery.error, '成长档案读取失败，请稍后重试')}
        </div>
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
                  <div className="page-actions">
                    <Link className="secondary-link" to="/companion">回到学习陪伴</Link>
                  </div>
                </>
              ) : (
                <>
                  <p>如果你已经有面试报告和练习记录，下一步最值得继续用学习陪伴页把计划执行闭环补齐。</p>
                  <div className="page-actions">
                    <Link className="secondary-link" to="/companion">去生成学习计划</Link>
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

          <article className="status-card" style={{ marginTop: 24 }}>
            <div className="card-inline">
              <div>
                <span className="section-kicker">本周重点补强</span>
                <h2>把最近反复暴露的问题压缩成 1 到 3 个主攻主题</h2>
              </div>
              <Link className="secondary-link" to="/companion">带入学习计划</Link>
            </div>

            {weeklyFocusQuery.isLoading ? (
              <p style={{ marginTop: 18 }}>正在整理你这周最该优先补强的主题...</p>
            ) : null}

            {weeklyFocusQuery.isError ? (
              <p style={{ marginTop: 18 }}>
                {extractErrorMessage(weeklyFocusQuery.error, '本周补强主题加载失败')}
              </p>
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
                      <p>{theme.reason}</p>
                      {theme.focus_tags.length ? (
                        <div className="community-tag-row">
                          {theme.focus_tags.map((tag) => (
                            <span key={`${theme.title}-${tag}`}>{tag}</span>
                          ))}
                        </div>
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
                        <Link className="secondary-link" to="/companion">去生成补强计划</Link>
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
              <div className="timeline-item" style={{ marginTop: 18 }}>
                <strong>本周还没有明确主攻主题</strong>
                <p>先做几道题或完成一场面试，学习档案和面试报告积累起来后，这里会自动帮你收束出本周最值得优先补强的方向。</p>
              </div>
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

            {practiceRecommendationsQuery.isLoading ? (
              <p style={{ marginTop: 18 }}>正在生成你的对症练习推荐...</p>
            ) : null}

            {practiceRecommendationsQuery.isError ? (
              <p style={{ marginTop: 18 }}>
                {extractErrorMessage(practiceRecommendationsQuery.error, '练习推荐加载失败')}
              </p>
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
                {practiceRecommendationsQuery.data.items.map((item) => (
                  <article className="feature-card" key={`growth-practice-recommendation-${item.question.id}`}>
                    <div className="card-inline">
                      <strong>{item.question.title}</strong>
                      <span>{item.focus_tag}</span>
                    </div>
                    <p>{item.reason}</p>
                    <p>难度：{item.question.difficulty || '未标注'}</p>
                    <div className="page-actions">
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
                ))}
              </div>
            ) : null}

            {!practiceRecommendationsQuery.isLoading && !practiceRecommendationsQuery.isError && !practiceRecommendationsQuery.data?.items.length ? (
              <div className="timeline-item" style={{ marginTop: 18 }}>
                <strong>还没有足够的推荐依据</strong>
                <p>先在题库里完成几道编程题或主观题，学习档案积累出错因标签后，这里会更准确地指出下一步补题方向。</p>
              </div>
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
              </div>
            )}
          </article>
        </>
      ) : null}
    </section>
  )
}
