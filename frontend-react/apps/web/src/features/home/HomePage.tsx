import { useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { extractErrorMessage, requestJson } from '@makejob/api-client'
import { isSuccessCode, type ApiEnvelope } from '@makejob/shared-types'
import { useAuthStore } from '../../state/auth'
import { fetchInterviewHistory } from '../interview/interviewApi'
import type { InterviewHistoryItem } from '../interview/interviewTypes'
import { fetchCompanionPlanProgress, fetchCurrentPlan } from '../companion/companionApi'
import { useFrontendIndustryPreference } from '../../shared/frontendIndustryPreference'
import { usePracticeStatsQuery } from '../../shared/frontendQueries'
import { requestLoginPrompt } from '../../shared/loginPrompt'
import { fetchPracticeCollectionsOverview } from '../../shared/practiceCollections'
import {
  buildCompanionCurrentPlanQueryKey,
  buildCompanionPlanProgressQueryKey,
  buildInterviewHistoryQueryKey,
  buildPracticeCollectionsOverviewQueryKey,
} from '../../shared/queryKeys'
import { AsyncEmptyState, AsyncInlineState } from '../../shared/asyncState'
import {
  difficultyLabel,
  fetchQuestions,
  questionTypeLabel,
} from '../../shared/practiceCatalog'

interface PageResult<T> {
  list: T[]
  total: number
  page: number
  page_size: number
}

interface CommunityPostAuthor {
  id: number
  username: string
  avatar?: string
  role?: string
}

interface CommunityPostItem {
  id: number
  post_type: string
  title: string
  content: string
  summary: string
  tags: string[]
  view_count: number
  comment_count: number
  like_count: number
  is_pinned: boolean
  is_recommended: boolean
  created_at: string
  author: CommunityPostAuthor
}

/**
 * 将时间值格式化为首页卡片更易读的日期时间文本。
 */
function formatDateTime(value?: string | number): string {
  if (!value) {
    return '-'
  }

  const date = typeof value === 'number' ? new Date(value * 1000) : new Date(value)
  if (Number.isNaN(date.getTime())) {
    return String(value)
  }

  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  const hour = String(date.getHours()).padStart(2, '0')
  const minute = String(date.getMinutes()).padStart(2, '0')
  return `${year}-${month}-${day} ${hour}:${minute}`
}

/**
 * 将时间字段转换为相对时间，方便首页动态流展示“刚刚更新”等轻量信息。
 */
function formatRelativeTime(value?: string): string {
  if (!value) {
    return '--'
  }

  const timestamp = new Date(value).getTime()
  if (Number.isNaN(timestamp)) {
    return value
  }

  const diff = Date.now() - timestamp
  if (diff < 60 * 1000) {
    return '刚刚'
  }

  const minutes = Math.floor(diff / (60 * 1000))
  if (minutes < 60) {
    return `${minutes} 分钟前`
  }

  const hours = Math.floor(diff / (60 * 60 * 1000))
  if (hours < 24) {
    return `${hours} 小时前`
  }

  const days = Math.floor(diff / (24 * 60 * 60 * 1000))
  if (days < 7) {
    return `${days} 天前`
  }

  return formatDateTime(value)
}

/**
 * 将长文本裁剪为首页卡片适合展示的摘要长度，避免卡片高度失控。
 */
function truncateText(value: string, maxLength: number): string {
  const normalized = value.trim()
  if (normalized.length <= maxLength) {
    return normalized
  }

  return `${normalized.slice(0, maxLength)}...`
}

/**
 * 拉取首页内容流使用的社区帖子列表，作为首版公开动态内容来源。
 */
async function fetchHomeCommunityPosts(params: {
  page: number
  pageSize: number
}): Promise<PageResult<CommunityPostItem>> {
  const searchParams = new URLSearchParams({
    page: String(params.page),
    page_size: String(params.pageSize),
  })

  const response = await requestJson<ApiEnvelope<PageResult<CommunityPostItem>>>(`/community/posts?${searchParams.toString()}`)
  if (!isSuccessCode(response.code) || !response.data) {
    throw new Error(response.message || '获取首页动态失败')
  }

  return response.data
}

/**
 * 展示当前 React 前台重构的整体方向和优先迁移模块。
 */
export default function HomePage() {
  const accessToken = useAuthStore((state) => state.accessToken)
  const { effectiveIndustryCode, effectiveIndustryLabel, industriesQuery } = useFrontendIndustryPreference()
  const highlightedIndustries = industriesQuery.data?.length
    ? industriesQuery.data
    : [
        { id: 0, code: 'go', name: 'Go 后端' },
        { id: 1, code: 'frontend', name: '前端工程' },
        { id: 2, code: 'java', name: 'Java 后端' },
      ]
  const practicePreviewQuery = useQuery({
    queryKey: ['home-practice-preview', effectiveIndustryCode],
    queryFn: () => fetchQuestions({
      page: 1,
      pageSize: 3,
      difficulty: '',
      keyword: '',
      industryId: highlightedIndustries.find((item) => item.code === effectiveIndustryCode)?.id || null,
      categoryId: null,
    }),
    enabled: Boolean(highlightedIndustries.find((item) => item.code === effectiveIndustryCode)?.id),
  })
  const communityQuery = useQuery({
    queryKey: ['home-community-posts'],
    queryFn: () => fetchHomeCommunityPosts({
      page: 1,
      pageSize: 3,
    }),
    staleTime: 2 * 60 * 1000,
  })
  const statsQuery = usePracticeStatsQuery(accessToken)
  const collectionsOverviewQuery = useQuery({
    queryKey: buildPracticeCollectionsOverviewQueryKey(accessToken),
    queryFn: () => fetchPracticeCollectionsOverview(accessToken as string),
    enabled: Boolean(accessToken),
    retry: false,
  })
  const interviewHistoryQuery = useQuery({
    queryKey: buildInterviewHistoryQueryKey(accessToken),
    queryFn: () => fetchInterviewHistory(accessToken as string),
    enabled: Boolean(accessToken),
    retry: false,
  })
  const currentPlanQuery = useQuery({
    queryKey: buildCompanionCurrentPlanQueryKey(accessToken),
    queryFn: () => fetchCurrentPlan(accessToken as string),
    enabled: Boolean(accessToken),
    retry: false,
  })
  const planProgressQuery = useQuery({
    queryKey: buildCompanionPlanProgressQueryKey(accessToken, currentPlanQuery.data?.id),
    queryFn: () => fetchCompanionPlanProgress(accessToken as string, currentPlanQuery.data?.id as number),
    enabled: Boolean(accessToken && currentPlanQuery.data?.id),
    retry: false,
  })
  const latestInterview: InterviewHistoryItem | undefined = interviewHistoryQuery.data?.list?.slice(0, 3)?.[0]

  return (
    <div className="home-shell">
      <section className="hero-panel">
        <div className="hero-content">
          <span className="page-tag">{effectiveIndustryLabel} Offer 导向学习平台</span>
          <h1>围绕 {effectiveIndustryLabel} 把题库训练、AI 面试和学习陪伴放在同一条成长链路里</h1>
          <p className="page-copy">
            当前首页会沿用你最近选择的行业方向，把题库、社区、AI 面试和学习陪伴统一收拢到同一条主线里；首页只保留轻入口，完整互动统一沉淀到独立社区频道。
          </p>

          <div className="hero-actions">
            <Link className="primary-button hero-link-button" to="/practice" preload="render">
              进入 {effectiveIndustryLabel} 题库
            </Link>
            <Link className="secondary-button hero-link-button" to="/interview" preload="render">
              打开 {effectiveIndustryLabel} 面试入口
            </Link>
          </div>

          <div className="hero-metrics">
            <article className="metric-card">
              <strong>{effectiveIndustryLabel} 真题训练</strong>
              <span>刷题、错题、收藏、笔记统一沉淀</span>
            </article>
            <article className="metric-card">
              <strong>{effectiveIndustryLabel} AI 面试</strong>
              <span>后续承接流式问答、追问与评分</span>
            </article>
            <article className="metric-card">
              <strong>统一方向上下文</strong>
              <span>学习计划、Live2D、提醒与反馈都沿用当前行业偏好</span>
            </article>
          </div>
        </div>

        <aside className="hero-aside">
          <div className="section-card spotlight-card">
            <span className="section-kicker">今日主线</span>
            <h2>先把 {effectiveIndustryLabel} 题库刷顺，再进 AI 面试</h2>
            <p>题库页已经是当前最完整的业务域，现在首页也会直接沿用当前方向，保证从首屏进入任何频道都不脱节。</p>
          </div>
          <div className="section-card mini-feed-card">
            <span className="section-kicker">近期规划</span>
            <ul className="mini-feed-list">
              <li>{effectiveIndustryLabel} 社区已经支持发帖、评论、点赞和我的帖子管理</li>
              <li>首页动态流只展示精简精选，完整浏览在独立社区页</li>
              <li>{effectiveIndustryLabel} 面试页继续补流式交互与报告体验</li>
            </ul>
          </div>
        </aside>
      </section>

      <section className="home-board">
        <div className="home-main-column">
          <article className="section-card section-card-large">
            <div className="section-head">
              <div>
                <span className="section-kicker">社区精选</span>
                <h2>{effectiveIndustryLabel} 社区最新内容</h2>
              </div>
              <Link className="secondary-button hero-link-button" to="/community" preload="intent">进入社区广场</Link>
            </div>

            {communityQuery.isLoading ? <AsyncInlineState className="companion-empty-text" message="首页动态加载中..." /> : null}
            {communityQuery.isError ? (
              <AsyncInlineState
                className="companion-empty-text"
                message={extractErrorMessage(communityQuery.error, '首页动态读取失败，稍后重试。')}
                tone="error"
              />
            ) : null}
            {communityQuery.data?.list?.length ? (
              <div className="feed-stack">
                {communityQuery.data.list.map((post) => (
                  <article className="feed-item" key={post.id}>
                    <div className="feed-item-head">
                      <strong>{post.title || truncateText(post.summary || post.content, 24)}</strong>
                      <span>{formatRelativeTime(post.created_at)}</span>
                    </div>
                    <p>{truncateText(post.summary || post.content, 120)}</p>
                    <div className="card-inline">
                      <span>{post.author?.username || '匿名用户'} · {post.post_type === 'article' ? '文章' : '动态'}</span>
                      <span>浏览 {post.view_count} · 点赞 {post.like_count}</span>
                    </div>
                    <Link className="secondary-link" to="/community/$postId" params={{ postId: String(post.id) }}>
                      查看帖子
                    </Link>
                  </article>
                ))}
              </div>
            ) : (
              <AsyncEmptyState
                title="内容流还没有帖子"
                message="社区闭环已经接通，后续有人发帖后首页这里会直接显示真实内容。"
              />
            )}
          </article>

          <article className="section-card section-card-large">
            <div className="section-head">
              <div>
                <span className="section-kicker">题库推荐</span>
                <h2>{effectiveIndustryLabel} 当前可直接开练的题目</h2>
              </div>
              <Link className="secondary-link" to="/practice" preload="intent">查看全部题目</Link>
            </div>

            {practicePreviewQuery.isLoading ? <AsyncInlineState className="companion-empty-text" message="推荐题单加载中..." /> : null}
            {practicePreviewQuery.isError ? (
              <AsyncInlineState
                className="companion-empty-text"
                message={extractErrorMessage(practicePreviewQuery.error, '推荐题单读取失败')}
                tone="error"
              />
            ) : null}
            {practicePreviewQuery.data?.list?.length ? (
              <div className="home-practice-preview-grid">
                {practicePreviewQuery.data.list.map((question) => (
                  <article className="feature-card" key={question.id}>
                    <div className="card-inline">
                      <strong>#{question.id}</strong>
                      <span>{difficultyLabel(question.difficulty)}</span>
                    </div>
                    <h2>{question.title}</h2>
                    <p>题型：{questionTypeLabel(question.type)}</p>
                    <p>分类：{question.category_name || question.category_id}</p>
                    <div className="page-actions">
                      <Link
                        className="secondary-link"
                        to={question.type === 'code' ? '/practice/editor/$questionId' : '/practice/$questionId'}
                        params={{ questionId: String(question.id) }}
                      >
                        进入做题
                      </Link>
                    </div>
                  </article>
                ))}
              </div>
            ) : (
              <AsyncEmptyState
                title="当前方向还没有题目推荐"
                message="如果行业已切换但这里为空，优先检查该行业下的题库数据是否已完成导入。"
              />
            )}
          </article>
        </div>

        <aside className="home-side-column">
          <article className="section-card sidebar-card">
            <span className="section-kicker">个人工作台</span>
            {accessToken ? (
              <div className="sidebar-links">
                <span className="sidebar-link">今日练习：{statsQuery.data?.today_count ?? '--'}</span>
                <span className="sidebar-link">连续打卡：{statsQuery.data?.streak_days ?? '--'} 天</span>
                <span className="sidebar-link">错题待复习：{collectionsOverviewQuery.data?.wrongQuestions ?? '--'}</span>
                <span className="sidebar-link">学习计划：{currentPlanQuery.data ? `${Math.round(planProgressQuery.data?.progress || currentPlanQuery.data.progress || 0)}%` : '未创建'}</span>
                {latestInterview ? (
                  <Link
                    className="sidebar-link"
                    to={latestInterview.status === 'ongoing' ? '/interview/$interviewId' : '/interview/$interviewId/report'}
                    params={{ interviewId: String(latestInterview.id) }}
                  >
                    {latestInterview.status === 'ongoing' ? '继续最近一场面试' : '查看最近一场报告'}
                  </Link>
                ) : (
                  <Link className="sidebar-link" to="/interview" preload="intent">开始第一场面试</Link>
                )}
              </div>
            ) : (
              <div className="timeline-item">
                <strong>登录后显示你的推进状态</strong>
                <p>首页会汇总今天练习、当前计划和最近面试，避免每次都从各频道单独查看。</p>
              </div>
            )}
          </article>

          <article className="section-card sidebar-card">
            <span className="section-kicker">热门方向</span>
            <div className="tag-cloud">
              {highlightedIndustries.map((industry) => (
                <span key={`${industry.id}-${industry.code}`}>
                  {industry.name}
                  {industry.code === effectiveIndustryCode ? ' · 当前' : ''}
                </span>
              ))}
            </div>
          </article>

          <article className="section-card sidebar-card">
            <span className="section-kicker">社区入口</span>
            <div className="sidebar-links">
              <Link className="sidebar-link" to="/community" preload="intent">浏览全部帖子</Link>
              {accessToken ? (
                <Link className="sidebar-link" to="/community/create" preload="intent">发布刷题复盘</Link>
              ) : (
                <button className="sidebar-link interactive-link-button" type="button" onClick={() => requestLoginPrompt('/community/create', 'missing')}>
                  发布刷题复盘
                </button>
              )}
              {accessToken ? (
                <Link className="sidebar-link" to="/community/mine" preload="intent">管理我的帖子</Link>
              ) : (
                <button className="sidebar-link interactive-link-button" type="button" onClick={() => requestLoginPrompt('/community/create', 'missing')}>
                  登录后发帖
                </button>
              )}
              {accessToken ? (
                <Link className="sidebar-link" to="/practice/notes" preload="intent">把笔记整理成帖子</Link>
              ) : (
                <button className="sidebar-link interactive-link-button" type="button" onClick={() => requestLoginPrompt('/practice/notes', 'missing')}>
                  把笔记整理成帖子
                </button>
              )}
            </div>
          </article>

          <article className="section-card sidebar-card">
            <span className="section-kicker">学习陪伴入口</span>
            <div className="sidebar-links">
              <Link className="sidebar-link" to="/companion" preload="intent">学习计划</Link>
              <Link className="sidebar-link" to="/companion" preload="intent">Live2D 展示</Link>
              {accessToken ? (
                <Link className="sidebar-link" to="/practice/wrong" preload="intent">错题复盘</Link>
              ) : (
                <button className="sidebar-link interactive-link-button" type="button" onClick={() => requestLoginPrompt('/practice/wrong', 'missing')}>
                  错题复盘
                </button>
              )}
              {accessToken ? (
                <Link className="sidebar-link" to="/practice/notes" preload="intent">学习笔记</Link>
              ) : (
                <button className="sidebar-link interactive-link-button" type="button" onClick={() => requestLoginPrompt('/practice/notes', 'missing')}>
                  学习笔记
                </button>
              )}
            </div>
          </article>
        </aside>
      </section>
    </div>
  )
}
