import { useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { Button, Tag, Badge, Spin, Empty, Progress } from 'antd'
import {
  FireOutlined,
  BookOutlined,
  RocketOutlined,
  CheckCircleOutlined,
  ClockCircleOutlined,
  RightOutlined,
  UserOutlined,
  TrophyOutlined,
  StarOutlined,
  ThunderboltOutlined,
  RiseOutlined,
  CodeOutlined,
  MessageOutlined,
  EyeOutlined,
  LikeOutlined,
  EditOutlined,
} from '@ant-design/icons'
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

const THEME = {
  bg: '#fafafa',
  cardBg: '#ffffff',
  primary: '#f97316',
  primaryDark: '#ea580c',
  primaryLight: '#fff7ed',
  accent: '#3b82f6',
  textMain: '#1c1917',
  textSecondary: '#57534e',
  textMuted: '#a8a29e',
  border: '#e7e5e4',
  success: '#22c55e',
  warning: '#f59e0b',
  danger: '#ef4444',
  shadow: '0 1px 3px rgba(0,0,0,0.04)',
  shadowCard: '0 4px 20px rgba(0,0,0,0.06)',
  radius: 16,
}

function formatDateTime(value?: string | number): string {
  if (!value) return '-'
  const date = typeof value === 'number' ? new Date(value * 1000) : new Date(value)
  if (Number.isNaN(date.getTime())) return String(value)
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  const hour = String(date.getHours()).padStart(2, '0')
  const minute = String(date.getMinutes()).padStart(2, '0')
  return `${year}-${month}-${day} ${hour}:${minute}`
}

function formatRelativeTime(value?: string): string {
  if (!value) return '--'
  const timestamp = new Date(value).getTime()
  if (Number.isNaN(timestamp)) return value
  const diff = Date.now() - timestamp
  if (diff < 60 * 1000) return '刚刚'
  const minutes = Math.floor(diff / (60 * 1000))
  if (minutes < 60) return `${minutes} 分钟前`
  const hours = Math.floor(diff / (60 * 60 * 1000))
  if (hours < 24) return `${hours} 小时前`
  const days = Math.floor(diff / (24 * 60 * 60 * 1000))
  if (days < 7) return `${days} 天前`
  return formatDateTime(value)
}

function truncateText(value: string, maxLength: number): string {
  const normalized = value.trim()
  if (normalized.length <= maxLength) return normalized
  return `${normalized.slice(0, maxLength)}...`
}

async function fetchHomeCommunityPosts(params: { page: number; pageSize: number }): Promise<PageResult<CommunityPostItem>> {
  const searchParams = new URLSearchParams({ page: String(params.page), page_size: String(params.pageSize) })
  const response = await requestJson<ApiEnvelope<PageResult<CommunityPostItem>>>(`/community/posts?${searchParams.toString()}`)
  if (!isSuccessCode(response.code) || !response.data) {
    throw new Error(response.message || '获取首页动态失败')
  }
  return response.data
}

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
      page: 1, pageSize: 5, difficulty: '', keyword: '',
      industryId: highlightedIndustries.find((item) => item.code === effectiveIndustryCode)?.id || null,
      categoryId: null,
    }),
    enabled: Boolean(highlightedIndustries.find((item) => item.code === effectiveIndustryCode)?.id),
  })
  const communityQuery = useQuery({
    queryKey: ['home-community-posts'],
    queryFn: () => fetchHomeCommunityPosts({ page: 1, pageSize: 3 }),
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

  const todayCount = statsQuery.data?.today_count ?? 0
  const streakDays = statsQuery.data?.streak_days ?? 0
  const wrongCount = collectionsOverviewQuery.data?.wrongQuestions ?? 0
  const planProgress = currentPlanQuery.data
    ? Math.round(planProgressQuery.data?.progress || currentPlanQuery.data.progress || 0)
    : 0

  const dailyQuestion = practicePreviewQuery.data?.list?.[0]

  return (
    <div style={{ background: THEME.bg, minHeight: '100vh' }}>
      {/* ===== Hero Section ===== */}
      <div style={{ padding: '56px 24px 48px', maxWidth: 1200, margin: '0 auto' }}>
        <div
          style={{
            display: 'grid',
            gridTemplateColumns: '1fr 420px',
            gap: 48,
            alignItems: 'start',
          }}
        >
          {/* Left: Value prop */}
          <div style={{ paddingTop: 16 }}>
            <div
              style={{
                display: 'inline-flex',
                alignItems: 'center',
                gap: 6,
                padding: '4px 12px',
                borderRadius: 20,
                background: THEME.primaryLight,
                color: THEME.primaryDark,
                fontSize: 12,
                fontWeight: 700,
                marginBottom: 20,
              }}
            >
              <FireOutlined />
              {effectiveIndustryLabel} Offer 导向学习平台
            </div>

            <h1
              style={{
                margin: 0,
                fontSize: 'clamp(32px, 4vw, 48px)',
                fontWeight: 800,
                color: THEME.textMain,
                lineHeight: 1.15,
                letterSpacing: -1,
                textWrap: 'balance',
              }}
            >
              把题库训练、AI 面试和学习陪伴放在同一条成长链路里
            </h1>

            <p
              style={{
                margin: '20px 0 0',
                fontSize: 17,
                color: THEME.textSecondary,
                lineHeight: 1.7,
                maxWidth: 480,
              }}
            >
              首页沿用你最近选择的行业方向，把题库、社区、AI 面试和学习陪伴统一收拢到同一条主线里。
            </p>

            <div style={{ display: 'flex', gap: 12, marginTop: 36, flexWrap: 'wrap' }}>
              <Link to="/practice" preload="render">
                <Button
                  type="primary"
                  size="large"
                  icon={<BookOutlined />}
                  style={{
                    borderRadius: 12,
                    height: 48,
                    fontSize: 15,
                    fontWeight: 600,
                    background: THEME.primary,
                    borderColor: THEME.primary,
                    boxShadow: '0 4px 16px rgba(249,115,22,0.25)',
                  }}
                >
                  进入 {effectiveIndustryLabel} 题库
                </Button>
              </Link>
              <Link to="/interview" preload="render">
                <Button
                  size="large"
                  icon={<RocketOutlined />}
                  style={{
                    borderRadius: 12,
                    height: 48,
                    fontSize: 15,
                    fontWeight: 600,
                    background: '#fff',
                    color: THEME.textMain,
                    border: `1px solid ${THEME.border}`,
                  }}
                >
                  开始 AI 面试
                </Button>
              </Link>
            </div>          </div>

          {/* Right: Daily Question Card */}
          <div
            style={{
              background: THEME.cardBg,
              borderRadius: 20,
              border: `1px solid ${THEME.border}`,
              boxShadow: THEME.shadowCard,
              overflow: 'hidden',
            }}
          >
            {/* Card Header */}
            <div
              style={{
                padding: '20px 24px',
                background: 'linear-gradient(135deg, #fef3c7, #fff7ed)',
                borderBottom: `1px solid ${THEME.border}`,
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'space-between',
              }}
            >
              <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
                <div
                  style={{
                    width: 36,
                    height: 36,
                    borderRadius: 10,
                    background: THEME.primary,
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'center',
                    color: '#fff',
                    fontSize: 16,
                  }}
                >
                  <ThunderboltOutlined />
                </div>
                <div>
                  <div style={{ fontSize: 14, fontWeight: 700, color: THEME.textMain }}>每日一题</div>
                  <div style={{ fontSize: 12, color: THEME.textMuted }}>{new Date().toLocaleDateString('zh-CN', { month: 'long', day: 'numeric', weekday: 'long' })}</div>
                </div>
              </div>
              <Tag color="orange" style={{ borderRadius: 8, fontSize: 11, fontWeight: 600 }}>NEW</Tag>
            </div>

            {/* Card Body */}
            <div style={{ padding: '24px' }}>
              {practicePreviewQuery.isLoading ? (
                <div style={{ padding: 32, textAlign: 'center' }}>
                  <Spin />
                </div>
              ) : dailyQuestion ? (
                <>
                  <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 12 }}>
                    <Tag
                      style={{
                        borderRadius: 8,
                        fontSize: 12,
                        color: dailyQuestion.difficulty === 'easy' ? THEME.success : dailyQuestion.difficulty === 'hard' ? THEME.danger : THEME.warning,
                        background: dailyQuestion.difficulty === 'easy' ? '#f0fdf4' : dailyQuestion.difficulty === 'hard' ? '#fef2f2' : '#fffbeb',
                        border: 'none',
                        fontWeight: 600,
                      }}
                    >
                      {difficultyLabel(dailyQuestion.difficulty)}
                    </Tag>
                    <span style={{ fontSize: 13, color: THEME.textMuted }}>
                      {questionTypeLabel(dailyQuestion.type)} · {dailyQuestion.category_name}
                    </span>
                  </div>

                  <h3
                    style={{
                      margin: '0 0 12px',
                      fontSize: 20,
                      fontWeight: 700,
                      color: THEME.textMain,
                      lineHeight: 1.4,
                    }}
                  >
                    {dailyQuestion.title}
                  </h3>

                  <p
                    style={{
                      margin: '0 0 20px',
                      fontSize: 14,
                      color: THEME.textSecondary,
                      lineHeight: 1.6,
                    }}
                  >
                    {truncateText(dailyQuestion.content, 120)}
                  </p>

                  <Link
                    to={
                      dailyQuestion.type === 'code'
                        ? '/practice/editor/$questionId'
                        : '/practice/$questionId'
                    }
                    params={{ questionId: String(dailyQuestion.id) }}
                  >
                    <Button
                      type="primary"
                      block
                      size="large"
                      icon={<CodeOutlined />}
                      style={{
                        borderRadius: 12,
                        height: 44,
                        fontWeight: 600,
                        background: THEME.primary,
                        borderColor: THEME.primary,
                      }}
                    >
                      开始做题
                    </Button>
                  </Link>
                </>
              ) : (
                <Empty
                  image={Empty.PRESENTED_IMAGE_SIMPLE}
                  description="暂无推荐题目"
                />
              )}
            </div>
          </div>
        </div>
      </div>

      {/* ===== Main Content ===== */}
      <div style={{ maxWidth: 1200, margin: '0 auto', padding: '0 24px 64px', display: 'grid', gridTemplateColumns: '1fr 340px', gap: 32 }}>
        {/* Left Column */}
        <div style={{ display: 'flex', flexDirection: 'column', gap: 32 }}>
          {/* Problem List */}
          <div
            style={{
              background: THEME.cardBg,
              borderRadius: THEME.radius,
              border: `1px solid ${THEME.border}`,
              boxShadow: THEME.shadow,
              overflow: 'hidden',
            }}
          >
            <div
              style={{
                padding: '20px 24px',
                borderBottom: `1px solid ${THEME.border}`,
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'space-between',
              }}
            >
              <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
                <BookOutlined style={{ fontSize: 18, color: THEME.primary }} />
                <span style={{ fontSize: 16, fontWeight: 700, color: THEME.textMain }}>{effectiveIndustryLabel} 推荐题目</span>
              </div>
              <Link to="/practice" preload="intent">
                <Button type="text" icon={<RightOutlined />} style={{ color: THEME.textMuted }}>
                  查看全部
                </Button>
              </Link>
            </div>

            {practicePreviewQuery.isLoading ? (
              <div style={{ padding: 40, textAlign: 'center' }}>
                <Spin tip="加载中..." />
              </div>
            ) : practicePreviewQuery.data?.list?.length ? (
              <div>
                {practicePreviewQuery.data.list.map((question, index) => {
                  const diffColor =
                    question.difficulty === 'easy'
                      ? THEME.success
                      : question.difficulty === 'hard'
                        ? THEME.danger
                        : THEME.warning
                  return (
                    <Link
                      key={question.id}
                      to={
                        question.type === 'code'
                          ? '/practice/editor/$questionId'
                          : '/practice/$questionId'
                      }
                      params={{ questionId: String(question.id) }}
                      style={{ textDecoration: 'none' }}
                    >
                      <div
                        style={{
                          display: 'grid',
                          gridTemplateColumns: '40px 1fr auto auto',
                          gap: 16,
                          alignItems: 'center',
                          padding: '14px 24px',
                          borderBottom: index === (practicePreviewQuery.data?.list?.length || 0) - 1 ? 'none' : `1px solid ${THEME.border}`,
                          transition: 'background 0.2s ease',
                          cursor: 'pointer',
                        }}
                        onMouseEnter={(e) => { e.currentTarget.style.background = '#fafaf9' }}
                        onMouseLeave={(e) => { e.currentTarget.style.background = 'transparent' }}
                      >
                        <span
                          style={{
                            fontSize: 13,
                            fontWeight: 700,
                            color: THEME.textMuted,
                            fontFamily: 'monospace',
                          }}
                        >
                          {index + 1}
                        </span>

                        <div style={{ minWidth: 0 }}>
                          <div
                            style={{
                              fontSize: 14,
                              fontWeight: 600,
                              color: THEME.textMain,
                              marginBottom: 2,
                              overflow: 'hidden',
                              textOverflow: 'ellipsis',
                              whiteSpace: 'nowrap',
                            }}
                          >
                            {question.title}
                          </div>
                          <div style={{ fontSize: 12, color: THEME.textMuted }}>
                            {question.category_name}
                          </div>
                        </div>

                        <Tag
                          style={{
                            margin: 0,
                            borderRadius: 8,
                            fontSize: 11,
                            fontWeight: 600,
                            color: diffColor,
                            background: `${diffColor}10`,
                            border: 'none',
                          }}
                        >
                          {difficultyLabel(question.difficulty)}
                        </Tag>

                        <span style={{ fontSize: 12, color: THEME.textMuted, width: 60, textAlign: 'right' }}>
                          {questionTypeLabel(question.type)}
                        </span>
                      </div>
                    </Link>
                  )
                })}
              </div>
            ) : (
              <Empty
                image={Empty.PRESENTED_IMAGE_SIMPLE}
                description="暂无推荐题目"
              />
            )}
          </div>

          {/* Community */}
          <div
            style={{
              background: THEME.cardBg,
              borderRadius: THEME.radius,
              border: `1px solid ${THEME.border}`,
              boxShadow: THEME.shadow,
              overflow: 'hidden',
            }}
          >
            <div
              style={{
                padding: '20px 24px',
                borderBottom: `1px solid ${THEME.border}`,
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'space-between',
              }}
            >
              <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
                <MessageOutlined style={{ fontSize: 18, color: THEME.accent }} />
                <span style={{ fontSize: 16, fontWeight: 700, color: THEME.textMain }}>社区最新</span>
              </div>
              <Link to="/community" preload="intent">
                <Button type="text" icon={<RightOutlined />} style={{ color: THEME.textMuted }}>
                  进入社区
                </Button>
              </Link>
            </div>

            {communityQuery.isLoading ? (
              <div style={{ padding: 40, textAlign: 'center' }}>
                <Spin />
              </div>
            ) : communityQuery.data?.list?.length ? (
              <div style={{ padding: '8px 0' }}>
                {communityQuery.data.list.map((post) => (
                  <Link
                    key={post.id}
                    to="/community/$postId"
                    params={{ postId: String(post.id) }}
                    style={{ textDecoration: 'none', display: 'block' }}
                  >
                    <div
                      style={{
                        padding: '16px 24px',
                        transition: 'background 0.2s ease',
                        cursor: 'pointer',
                      }}
                      onMouseEnter={(e) => { e.currentTarget.style.background = '#fafaf9' }}
                      onMouseLeave={(e) => { e.currentTarget.style.background = 'transparent' }}
                    >
                      <div
                        style={{
                          display: 'flex',
                          alignItems: 'center',
                          justifyContent: 'space-between',
                          marginBottom: 6,
                        }}
                      >
                        <span
                          style={{
                            fontSize: 14,
                            fontWeight: 600,
                            color: THEME.textMain,
                            overflow: 'hidden',
                            textOverflow: 'ellipsis',
                            whiteSpace: 'nowrap',
                            flex: 1,
                          }}
                        >
                          {post.title || truncateText(post.summary || post.content, 28)}
                        </span>
                        <span style={{ fontSize: 12, color: THEME.textMuted, flexShrink: 0, marginLeft: 12 }}>
                          {formatRelativeTime(post.created_at)}
                        </span>
                      </div>
                      <p
                        style={{
                          margin: '0 0 10px',
                          fontSize: 13,
                          color: THEME.textSecondary,
                          lineHeight: 1.5,
                        }}
                      >
                        {truncateText(post.summary || post.content, 100)}
                      </p>
                      <div style={{ display: 'flex', alignItems: 'center', gap: 16, fontSize: 12, color: THEME.textMuted }}>
                        <span>{post.author?.username || '匿名用户'}</span>
                        <span style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
                          <EyeOutlined /> {post.view_count}
                        </span>
                        <span style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
                          <LikeOutlined /> {post.like_count}
                        </span>
                      </div>
                    </div>
                  </Link>
                ))}
              </div>
            ) : (
              <Empty
                image={Empty.PRESENTED_IMAGE_SIMPLE}
                description="社区还没有帖子"
              />
            )}
          </div>
        </div>

        {/* Right Sidebar */}
        <div style={{ display: 'flex', flexDirection: 'column', gap: 20 }}>
          {/* Stats Card */}
          <div
            style={{
              background: THEME.cardBg,
              borderRadius: THEME.radius,
              border: `1px solid ${THEME.border}`,
              boxShadow: THEME.shadow,
              padding: 24,
            }}
          >
            <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 20 }}>
              <TrophyOutlined style={{ fontSize: 16, color: THEME.primary }} />
              <span style={{ fontSize: 15, fontWeight: 700, color: THEME.textMain }}>学习数据</span>
            </div>

            {accessToken ? (
              <>
                <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 12, marginBottom: 20 }}>
                  <div
                    style={{
                      padding: '16px 12px',
                      borderRadius: 12,
                      background: THEME.primaryLight,
                      textAlign: 'center',
                    }}
                  >
                    <div style={{ fontSize: 28, fontWeight: 800, color: THEME.primary, lineHeight: 1 }}>
                      {todayCount}
                    </div>
                    <div style={{ fontSize: 12, color: THEME.textSecondary, marginTop: 4 }}>今日练习</div>
                  </div>
                  <div
                    style={{
                      padding: '16px 12px',
                      borderRadius: 12,
                      background: '#f0fdf4',
                      textAlign: 'center',
                    }}
                  >
                    <div style={{ fontSize: 28, fontWeight: 800, color: THEME.success, lineHeight: 1 }}>
                      {streakDays}
                    </div>
                    <div style={{ fontSize: 12, color: THEME.textSecondary, marginTop: 4 }}>连续打卡</div>
                  </div>
                </div>

                <div style={{ marginBottom: 16 }}>
                  <div
                    style={{
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'space-between',
                      marginBottom: 6,
                    }}
                  >
                    <span style={{ fontSize: 13, color: THEME.textSecondary }}>错题待复习</span>
                    <span style={{ fontSize: 13, fontWeight: 700, color: wrongCount > 0 ? THEME.danger : THEME.textMuted }}>
                      {wrongCount} 道
                    </span>
                  </div>
                  <div style={{ height: 6, borderRadius: 3, background: '#f5f5f4', overflow: 'hidden' }}>
                    <div
                      style={{
                        height: '100%',
                        width: `${Math.min(wrongCount * 5, 100)}%`,
                        borderRadius: 3,
                        background: wrongCount > 0 ? THEME.danger : '#d6d3d1',
                        transition: 'width 0.5s ease',
                      }}
                    />
                  </div>
                </div>

                <div style={{ marginBottom: 16 }}>
                  <div
                    style={{
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'space-between',
                      marginBottom: 6,
                    }}
                  >
                    <span style={{ fontSize: 13, color: THEME.textSecondary }}>学习计划进度</span>
                    <span style={{ fontSize: 13, fontWeight: 700, color: THEME.textMain }}>
                      {currentPlanQuery.data ? `${planProgress}%` : '未创建'}
                    </span>
                  </div>
                  <Progress
                    percent={planProgress}
                    showInfo={false}
                    strokeColor={THEME.primary}
                    trailColor="#f5f5f4"
                    size="small"
                  />
                </div>

                {latestInterview ? (
                  <Link
                    to={
                      latestInterview.status === 'ongoing' || latestInterview.status === 'preparing'
                        ? '/interview/$interviewId'
                        : '/interview/$interviewId/report'
                    }
                    params={{ interviewId: String(latestInterview.id) }}
                  >
                    <Button
                      block
                      icon={<RocketOutlined />}
                      style={{
                        borderRadius: 10,
                        fontWeight: 600,
                        background: '#fafaf9',
                        border: `1px solid ${THEME.border}`,
                        color: THEME.textMain,
                      }}
                    >
                      {latestInterview.status === 'ongoing' || latestInterview.status === 'preparing'
                        ? '继续最近面试'
                        : '查看面试报告'}
                    </Button>
                  </Link>
                ) : (
                  <Link to="/interview" preload="intent">
                    <Button
                      block
                      type="primary"
                      icon={<RocketOutlined />}
                      style={{
                        borderRadius: 10,
                        fontWeight: 600,
                        background: THEME.primary,
                        borderColor: THEME.primary,
                      }}
                    >
                      开始第一场面试
                    </Button>
                  </Link>
                )}
              </>
            ) : (
              <div style={{ textAlign: 'center', padding: '12px 0' }}>
                <UserOutlined style={{ fontSize: 32, color: THEME.border, marginBottom: 12 }} />
                <div style={{ fontSize: 14, fontWeight: 600, color: THEME.textMain, marginBottom: 4 }}>
                  登录后查看学习数据
                </div>
                <p style={{ margin: 0, fontSize: 13, color: THEME.textSecondary }}>
                  汇总今日练习、当前计划和最近面试进度
                </p>
              </div>
            )}
          </div>

          {/* Hot Industries */}
          <div
            style={{
              background: THEME.cardBg,
              borderRadius: THEME.radius,
              border: `1px solid ${THEME.border}`,
              boxShadow: THEME.shadow,
              padding: 24,
            }}
          >
            <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 16 }}>
              <RiseOutlined style={{ fontSize: 16, color: THEME.warning }} />
              <span style={{ fontSize: 15, fontWeight: 700, color: THEME.textMain }}>热门方向</span>
            </div>
            <div style={{ display: 'flex', flexWrap: 'wrap', gap: 8 }}>
              {highlightedIndustries.map((industry) => {
                const isActive = industry.code === effectiveIndustryCode
                return (
                  <Tag
                    key={`${industry.id}-${industry.code}`}
                    style={{
                      borderRadius: 10,
                      fontSize: 13,
                      padding: '4px 12px',
                      fontWeight: isActive ? 600 : 400,
                      color: isActive ? '#fff' : THEME.textSecondary,
                      background: isActive ? THEME.primary : '#fafaf9',
                      border: isActive ? `1px solid ${THEME.primary}` : `1px solid ${THEME.border}`,
                      cursor: 'default',
                    }}
                  >
                    {industry.name}
                  </Tag>
                )
              })}
            </div>
          </div>

          {/* Quick Links */}
          <div
            style={{
              background: THEME.cardBg,
              borderRadius: THEME.radius,
              border: `1px solid ${THEME.border}`,
              boxShadow: THEME.shadow,
              padding: 24,
            }}
          >
            <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 16 }}>
              <StarOutlined style={{ fontSize: 16, color: THEME.accent }} />
              <span style={{ fontSize: 15, fontWeight: 700, color: THEME.textMain }}>快速入口</span>
            </div>
            <div style={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
              {[
                { to: '/community', label: '浏览社区帖子', icon: <MessageOutlined /> },
                accessToken
                  ? { to: '/practice/wrong', label: '错题复盘', icon: <BookOutlined /> }
                  : { action: () => requestLoginPrompt('/practice/wrong', 'missing'), label: '错题复盘', icon: <BookOutlined /> },
                accessToken
                  ? { to: '/practice/notes', label: '学习笔记', icon: <EditOutlined /> }
                  : { action: () => requestLoginPrompt('/practice/notes', 'missing'), label: '学习笔记', icon: <EditOutlined /> },
                { to: '/companion', label: '学习计划', icon: <RocketOutlined /> },
                { to: '/growth', label: '成长档案', icon: <RiseOutlined /> },
              ].map((item, index) => {
                const row = (
                  <div
                    key={index}
                    style={{
                      display: 'flex',
                      alignItems: 'center',
                      gap: 10,
                      padding: '10px 12px',
                      borderRadius: 10,
                      fontSize: 13,
                      color: THEME.textSecondary,
                      transition: 'all 0.2s ease',
                      cursor: 'pointer',
                    }}
                    onMouseEnter={(e) => {
                      e.currentTarget.style.background = '#fafaf9'
                      e.currentTarget.style.color = THEME.textMain
                    }}
                    onMouseLeave={(e) => {
                      e.currentTarget.style.background = 'transparent'
                      e.currentTarget.style.color = THEME.textSecondary
                    }}
                  >
                    <span style={{ fontSize: 14, color: THEME.textMuted }}>{item.icon}</span>
                    <span style={{ flex: 1 }}>{item.label}</span>
                    <RightOutlined style={{ fontSize: 10, color: THEME.textMuted }} />
                  </div>
                )
                if ('action' in item && item.action) {
                  return (
                    <button
                      key={index}
                      type="button"
                      onClick={item.action}
                      style={{ all: 'unset', display: 'block', width: '100%' }}
                    >
                      {row}
                    </button>
                  )
                }
                return (
                  <Link
                    key={index}
                    to={item.to}
                    preload="intent"
                    style={{ textDecoration: 'none', display: 'block' }}
                  >
                    {row}
                  </Link>
                )
              })}
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}
