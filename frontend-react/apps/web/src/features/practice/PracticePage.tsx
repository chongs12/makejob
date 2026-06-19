import type { FormEvent } from 'react'
import { useEffect, useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Link, useNavigate, useSearch } from '@tanstack/react-router'
import { Button, Input, Select, Tag, Spin, Empty, Pagination, Progress } from 'antd'
import {
  SearchOutlined,
  CheckCircleFilled,
  CheckCircleOutlined,
  StarFilled,
  FireOutlined,
  TrophyOutlined,
  RiseOutlined,
  BookOutlined,
  RocketOutlined,
  RightOutlined,
  ThunderboltOutlined,
  EditOutlined,
  FileTextOutlined,
  HeartOutlined,
  UndoOutlined,
  CloseCircleOutlined,
} from '@ant-design/icons'
import { extractErrorMessage } from '@makejob/api-client'
import { useAuthStore } from '../../state/auth'
import {
  DEFAULT_FRONTEND_INDUSTRY_CODE,
  formatFrontendIndustryLabel,
  persistSelectedFrontendIndustryCode,
  readSelectedFrontendIndustryCode,
  resolvePreferredFrontendIndustry,
} from '../../shared/industryContext'
import { findFrontendIndustryById } from '../../shared/frontendIndustryPreference'
import { useFrontendIndustriesQuery, usePracticeStatsQuery } from '../../shared/frontendQueries'
import { requestLoginPrompt } from '../../shared/loginPrompt'
import { AsyncEmptyState, AsyncInlineState } from '../../shared/asyncState'
import { filterPracticeCollectionQuestions } from '../../shared/practiceCollectionMode'
import {
  buildPracticeCategoriesQueryKey,
  buildPracticeCollectionsOverviewQueryKey,
  buildPracticeQuestionSetDetailQueryKey,
  buildPracticeQuestionSetsQueryKey,
  buildPracticeQuestionsQueryKey,
  buildPracticeRecommendationsQueryKey,
} from '../../shared/queryKeys'
import { fetchMistakeTopic, fetchMistakeTopics, resolveMistakeTopicRoute } from '../../shared/mistakeTopics'
import {
  fetchPracticeRecommendations,
  resolvePracticeRecommendationModeLabel,
  resolvePracticeRecommendationRoute,
  resolvePracticeRecommendationSourceLabel,
} from '../../shared/practiceRecommendations'
import {
  buildPracticeRecommendationRouteSearch,
  buildPracticeRouteSearch,
  readPracticeRouteFocusTags,
  resolvePracticeQuestionSetTitle,
  resolvePracticeRouteSourceLabel,
  type PracticeRouteSearch,
} from '../../shared/practiceRoute'
import { fetchPracticeCollectionsOverview } from '../../shared/practiceCollections'
import {
  difficultyLabel,
  fetchCategories,
  fetchQuestionSetDetail,
  fetchQuestionSets,
  fetchQuestions,
  flattenCategories,
  generateExamRequest,
  PRACTICE_PAGE_SIZE,
  questionTypeLabel,
  resolvePracticeTarget,
} from '../../shared/practiceCatalog'

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
  info: '#3b82f6',
  shadow: '0 1px 3px rgba(0,0,0,0.04)',
  shadowCard: '0 4px 20px rgba(0,0,0,0.06)',
  radius: 16,
}

const cardBase = {
  background: THEME.cardBg,
  borderRadius: THEME.radius,
  boxShadow: THEME.shadow,
  border: `1px solid ${THEME.border}`,
}

const difficultyColor: Record<string, string> = {
  easy: '#22c55e',
  medium: '#f59e0b',
  hard: '#ef4444',
}

const difficultyBg: Record<string, string> = {
  easy: '#f0fdf4',
  medium: '#fffbeb',
  hard: '#fef2f2',
}

function resolveSetColor(slug: string): string {
  const colors = ['#3b82f6', '#10b981', '#f59e0b', '#ef4444', '#8b5cf6', '#ec4899', '#06b6d4']
  let hash = 0
  for (let i = 0; i < slug.length; i++) hash = slug.charCodeAt(i) + ((hash << 5) - hash)
  return colors[Math.abs(hash) % colors.length]
}

/**
 * 提供刷题总入口，统一承接题库筛选、题单补练、错因专题和模拟练习。
 */
export function PracticePage() {
  const routeSearch = useSearch({ from: '/practice' })
  const navigate = useNavigate()
  const accessToken = useAuthStore((state) => state.accessToken)
  const [fallbackIndustryCode] = useState(() => readSelectedFrontendIndustryCode() || DEFAULT_FRONTEND_INDUSTRY_CODE)
  const selectedIndustryCode = routeSearch.industry?.trim() || fallbackIndustryCode
  const keyword = routeSearch.keyword || ''
  const difficulty = routeSearch.difficulty || ''
  const categoryId = routeSearch.category || null
  const page = routeSearch.page || 1
  const focusTags = useMemo(() => readPracticeRouteFocusTags(routeSearch), [routeSearch])
  const [keywordInput, setKeywordInput] = useState(() => keyword)
  const [examMessage, setExamMessage] = useState('')
  const industriesQuery = useFrontendIndustriesQuery()

  const selectedIndustry = useMemo(
    () => resolvePreferredFrontendIndustry(industriesQuery.data || [], selectedIndustryCode),
    [industriesQuery.data, selectedIndustryCode],
  )
  const effectiveIndustryCode = selectedIndustry?.code || selectedIndustryCode.trim() || DEFAULT_FRONTEND_INDUSTRY_CODE
  const effectiveIndustryLabel = formatFrontendIndustryLabel(selectedIndustry, effectiveIndustryCode)
  const activeQuestionSetSlug = routeSearch.questionSet || ''

  const categoriesQuery = useQuery({
    queryKey: buildPracticeCategoriesQueryKey(effectiveIndustryCode),
    queryFn: () => fetchCategories(effectiveIndustryCode),
    enabled: Boolean(effectiveIndustryCode),
    staleTime: 10 * 60 * 1000,
    refetchOnWindowFocus: false,
  })

  const questionsQuery = useQuery({
    queryKey: buildPracticeQuestionsQueryKey({
      page,
      industryCode: effectiveIndustryCode,
      industryId: selectedIndustry?.id || null,
      difficulty,
      keyword,
      categoryId,
    }),
    queryFn: () => fetchQuestions({
      page,
      pageSize: PRACTICE_PAGE_SIZE,
      difficulty,
      keyword,
      industryId: selectedIndustry?.id || null,
      categoryId,
      token: accessToken,
    }),
    enabled: !activeQuestionSetSlug,
    staleTime: 2 * 60 * 1000,
    refetchOnWindowFocus: false,
  })

  const questionSetsQuery = useQuery({
    queryKey: buildPracticeQuestionSetsQueryKey(effectiveIndustryCode),
    queryFn: () => fetchQuestionSets(effectiveIndustryCode),
    enabled: Boolean(effectiveIndustryCode),
    staleTime: 10 * 60 * 1000,
    refetchOnWindowFocus: false,
  })

  const activeQuestionSetFromList = useMemo(
    () => (questionSetsQuery.data || []).find((set) => set.slug === activeQuestionSetSlug) || null,
    [questionSetsQuery.data, activeQuestionSetSlug],
  )

  const activeTopicCode = routeSearch.topic || ''
  const activeTopicQuery = useQuery({
    queryKey: ['mistake-topic-detail', activeTopicCode],
    queryFn: () => fetchMistakeTopic(activeTopicCode, accessToken),
    enabled: Boolean(activeTopicCode),
  })

  const statsQuery = usePracticeStatsQuery(accessToken)

  const collectionsOverviewQuery = useQuery({
    queryKey: buildPracticeCollectionsOverviewQueryKey(accessToken),
    queryFn: () => fetchPracticeCollectionsOverview(accessToken as string),
    enabled: Boolean(accessToken),
  })

  const practiceRecommendationsQuery = useQuery({
    queryKey: buildPracticeRecommendationsQueryKey(accessToken),
    queryFn: () => fetchPracticeRecommendations(accessToken as string, 4),
    enabled: Boolean(accessToken),
  })

  const recommendationTopicCodes = useMemo(
    () =>
      Array.from(
        new Set(
          (practiceRecommendationsQuery.data?.items || [])
            .map((item) => item.topic_code?.trim() || '')
            .filter(Boolean),
        ),
      ),
    [practiceRecommendationsQuery.data?.items],
  )

  const recommendationTopicsQuery = useQuery({
    queryKey: ['practice-recommendation-topics', recommendationTopicCodes],
    queryFn: () => fetchMistakeTopics(recommendationTopicCodes, accessToken),
    enabled: Boolean(recommendationTopicCodes.length),
    staleTime: 5 * 60 * 1000,
  })

  const recommendationTopicMap = useMemo(
    () => new Map((recommendationTopicsQuery.data || []).map((topic) => [topic.code, topic])),
    [recommendationTopicsQuery.data],
  )

  const categoryOptions = useMemo(
    () => flattenCategories(categoriesQuery.data || []),
    [categoriesQuery.data],
  )

  const isQuestionSetCollectionMode = Boolean(activeQuestionSetSlug)

  const filteredQuestionSetQuestions = useMemo(
    () =>
      filterPracticeCollectionQuestions(activeQuestionSetFromList?.questions || [], {
        keyword,
        difficulty,
      }),
    [activeQuestionSetFromList?.questions, difficulty, keyword],
  )

  // ===== 提前计算所有 hooks，避免条件分支跳过 hooks 调用 =====
  const totalPages = useMemo(() => {
    const total = questionsQuery.data?.total || 0
    return Math.max(1, Math.ceil(total / PRACTICE_PAGE_SIZE))
  }, [questionsQuery.data?.total])

  const questions = questionsQuery.data?.list || []

  function navigatePractice(nextSearch: Partial<PracticeRouteSearch>, replace = false): void {
    const pickField = <K extends keyof PracticeRouteSearch>(key: K): PracticeRouteSearch[K] =>
      Object.prototype.hasOwnProperty.call(nextSearch, key) ? nextSearch[key] : routeSearch[key]

    navigate({
      to: '/practice',
      search: buildPracticeRouteSearch({
        industryCode: (pickField('industry') || selectedIndustryCode) as string,
        keyword: pickField('keyword'),
        difficulty: pickField('difficulty'),
        categoryId: (pickField('category') ?? null) as number | null,
        page: pickField('page'),
        questionSetSlug: pickField('questionSet'),
        topicCode: pickField('topic'),
        focusTags: readPracticeRouteFocusTags({
          ...routeSearch,
          focus: pickField('focus'),
        }),
        source: pickField('source'),
        title: pickField('title'),
        reason: pickField('reason'),
      }),
      replace,
    })
  }

  useEffect(() => {
    const normalizedIndustryCode = effectiveIndustryCode.trim()
    if (!normalizedIndustryCode) return
    persistSelectedFrontendIndustryCode(normalizedIndustryCode)
  }, [effectiveIndustryCode])

  useEffect(() => {
    setKeywordInput(keyword)
  }, [keyword])

  function handleSearchSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    navigatePractice({ keyword: keywordInput.trim(), page: 1 })
  }

  function handleIndustryChange(nextIndustryCode: string) {
    navigatePractice({
      industry: nextIndustryCode,
      category: undefined,
      page: 1,
      questionSet: undefined,
      topic: undefined,
      focus: undefined,
      source: undefined,
      title: undefined,
      reason: undefined,
    })
    setExamMessage(`已切换到 ${formatFrontendIndustryLabel(resolvePreferredFrontendIndustry(industriesQuery.data || [], nextIndustryCode), nextIndustryCode)} 题库。`)
  }

  async function handleGenerateExam(mode: 'random' | 'timed') {
    if (!accessToken) {
      requestLoginPrompt('/practice', 'missing')
      return
    }
    try {
      const exam = await generateExamRequest({
        token: accessToken,
        mode,
        difficulty,
        industryId: selectedIndustry?.id || null,
        categoryId,
      })
      const firstQuestion = exam.questions?.[0]
      if (!firstQuestion) {
        setExamMessage('当前条件下没有可用题目')
        return
      }
      setExamMessage(mode === 'timed' ? '限时模拟已生成' : '随机练习已生成')
      navigate({
        to: firstQuestion.type === 'code' ? '/practice/editor/$questionId' : '/practice/$questionId',
        params: { questionId: String(firstQuestion.id) },
      })
    } catch (error) {
      if (!useAuthStore.getState().accessToken) {
        requestLoginPrompt('/practice', 'expired')
        return
      }
      setExamMessage(extractErrorMessage(error, '组卷失败'))
    }
  }

  function handleClearPracticeContext(): void {
    navigatePractice({
      questionSet: undefined,
      topic: undefined,
      focus: undefined,
      source: undefined,
      title: undefined,
      reason: undefined,
    })
  }

  function handleApplyQuestionSetContext(questionSetSlug: string): void {
    navigatePractice({ questionSet: questionSetSlug, page: 1 })
  }

  // ===== Question Set Mode =====
  if (isQuestionSetCollectionMode) {
    const QUESTION_SET_PAGE_SIZE = 20
    const questionSetTotal = filteredQuestionSetQuestions.length
    const questionSetTotalPages = Math.max(1, Math.ceil(questionSetTotal / QUESTION_SET_PAGE_SIZE))
    const questionSetPage = Math.min(page, questionSetTotalPages)
    const questionSetPageQuestions = filteredQuestionSetQuestions.slice(
      (questionSetPage - 1) * QUESTION_SET_PAGE_SIZE,
      questionSetPage * QUESTION_SET_PAGE_SIZE,
    )

    // 计算难度分布
    const difficultyStats = filteredQuestionSetQuestions.reduce(
      (acc, q) => {
        const key = q.difficulty || 'unknown'
        acc[key] = (acc[key] || 0) + 1
        return acc
      },
      {} as Record<string, number>,
    )

    return (
      <div style={{ background: THEME.bg, minHeight: '100vh' }}>
        <div style={{ maxWidth: 960, margin: '0 auto', padding: '32px 24px' }}>
          {/* 返回链接 */}
          <div style={{ marginBottom: 24 }}>
            <Link
              to="/practice"
              search={buildPracticeRouteSearch({ industryCode: effectiveIndustryCode, page: 1 })}
              style={{
                fontSize: 14,
                color: THEME.textSecondary,
                textDecoration: 'none',
                display: 'inline-flex',
                alignItems: 'center',
                gap: 4,
              }}
            >
              <RightOutlined style={{ fontSize: 10, transform: 'rotate(180deg)' }} />
              返回题库
            </Link>
          </div>

          {questionSetsQuery.isLoading && (
            <div style={{ padding: 60, textAlign: 'center' }}>
              <Spin tip="题单加载中..." />
            </div>
          )}

          {questionSetsQuery.isError && (
            <div style={{ padding: 40, textAlign: 'center', color: THEME.danger }}>
              {extractErrorMessage(questionSetsQuery.error, '题单加载失败')}
            </div>
          )}

          {!questionSetsQuery.isLoading && !activeQuestionSetFromList && (
            <div style={{ padding: 40, textAlign: 'center', color: THEME.textMuted }}>
              未找到题单
            </div>
          )}

          {activeQuestionSetFromList && (
            <>
              {/* 顶部信息区 */}
              <div
                style={{
                  ...cardBase,
                  padding: '32px',
                  marginBottom: 24,
                }}
              >
                {/* 标题 */}
                <h1 style={{ margin: 0, fontSize: 28, fontWeight: 700, color: THEME.textMain, lineHeight: 1.3 }}>
                  {activeQuestionSetFromList.title}
                </h1>

                {/* 描述 */}
                {activeQuestionSetFromList.description ? (
                  <p style={{ margin: '12px 0 0', fontSize: 15, color: THEME.textSecondary, lineHeight: 1.7 }}>
                    {activeQuestionSetFromList.description}
                  </p>
                ) : null}

                {/* 进度条和统计 */}
                <div style={{ marginTop: 24 }}>
                  <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 8 }}>
                    <span style={{ fontSize: 14, color: THEME.textSecondary }}>
                      进度
                    </span>
                    <span style={{ fontSize: 14, fontWeight: 600, color: THEME.textMain }}>
                      0 / {questionSetTotal} 题
                    </span>
                  </div>
                  <div style={{ height: 8, borderRadius: 4, background: '#f5f5f4', overflow: 'hidden' }}>
                    <div
                      style={{
                        height: '100%',
                        width: '0%',
                        borderRadius: 4,
                        background: THEME.primary,
                        transition: 'width 0.5s ease',
                      }}
                    />
                  </div>
                </div>

                {/* 难度分布 */}
                <div style={{ marginTop: 20, display: 'flex', gap: 16, flexWrap: 'wrap' }}>
                  <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                    <div style={{ width: 8, height: 8, borderRadius: '50%', background: THEME.success }} />
                    <span style={{ fontSize: 13, color: THEME.textSecondary }}>
                      简单 <span style={{ fontWeight: 600, color: THEME.textMain }}>{difficultyStats.easy || 0}</span>
                    </span>
                  </div>
                  <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                    <div style={{ width: 8, height: 8, borderRadius: '50%', background: THEME.warning }} />
                    <span style={{ fontSize: 13, color: THEME.textSecondary }}>
                      中等 <span style={{ fontWeight: 600, color: THEME.textMain }}>{difficultyStats.medium || 0}</span>
                    </span>
                  </div>
                  <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                    <div style={{ width: 8, height: 8, borderRadius: '50%', background: THEME.danger }} />
                    <span style={{ fontSize: 13, color: THEME.textSecondary }}>
                      困难 <span style={{ fontWeight: 600, color: THEME.textMain }}>{difficultyStats.hard || 0}</span>
                    </span>
                  </div>
                </div>
              </div>

              {/* 题目列表 */}
              <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
                {questionSetPageQuestions.map((question, index) => {
                  const diffColor = question.difficulty === 'easy' ? THEME.success : question.difficulty === 'hard' ? THEME.danger : THEME.warning
                  const diffBg = question.difficulty === 'easy' ? '#f0fdf4' : question.difficulty === 'hard' ? '#fef2f2' : '#fffbeb'
                  const questionIndex = (questionSetPage - 1) * QUESTION_SET_PAGE_SIZE + index + 1

                  return (
                    <Link
                      key={question.id}
                      to={question.type === 'code' ? '/practice/editor/$questionId' : '/practice/$questionId'}
                      params={{ questionId: String(question.id) }}
                      style={{ textDecoration: 'none' }}
                    >
                      <div
                        style={{
                          ...cardBase,
                          padding: '16px 20px',
                          display: 'flex',
                          alignItems: 'center',
                          gap: 16,
                          transition: 'all 0.15s ease',
                          cursor: 'pointer',
                        }}
                        onMouseEnter={(e) => {
                          e.currentTarget.style.borderColor = THEME.primary
                          e.currentTarget.style.boxShadow = '0 2px 8px rgba(249,115,22,0.1)'
                        }}
                        onMouseLeave={(e) => {
                          e.currentTarget.style.borderColor = THEME.border
                          e.currentTarget.style.boxShadow = THEME.shadow
                        }}
                      >
                        {/* 状态图标 */}
                        {question.is_answered ? (
                          <CheckCircleFilled style={{ fontSize: 18, color: THEME.success, flexShrink: 0 }} />
                        ) : (
                          <div
                            style={{
                              width: 18,
                              height: 18,
                              borderRadius: '50%',
                              border: `2px solid ${THEME.border}`,
                              flexShrink: 0,
                            }}
                          />
                        )}

                        {/* 题号 */}
                        <span
                          style={{
                            fontSize: 14,
                            fontWeight: 500,
                            color: THEME.textMuted,
                            fontFamily: 'monospace',
                            minWidth: 28,
                            textAlign: 'center',
                          }}
                        >
                          {questionIndex}
                        </span>

                        {/* 题目标题 */}
                        <span
                          style={{
                            flex: 1,
                            fontSize: 14,
                            fontWeight: 500,
                            color: THEME.textMain,
                            overflow: 'hidden',
                            textOverflow: 'ellipsis',
                            whiteSpace: 'nowrap',
                          }}
                        >
                          {question.title}
                        </span>

                        {/* 难度标签 */}
                        <span
                          style={{
                            fontSize: 12,
                            fontWeight: 600,
                            color: diffColor,
                            background: diffBg,
                            padding: '2px 10px',
                            borderRadius: 6,
                            flexShrink: 0,
                          }}
                        >
                          {difficultyLabel(question.difficulty)}
                        </span>
                      </div>
                    </Link>
                  )
                })}
              </div>

              {/* 分页 */}
              {questionSetTotalPages > 1 && (
                <div style={{ display: 'flex', justifyContent: 'center', marginTop: 32 }}>
                  <Pagination
                    current={questionSetPage}
                    total={questionSetTotal}
                    pageSize={QUESTION_SET_PAGE_SIZE}
                    onChange={(p) => navigatePractice({ page: p })}
                    showSizeChanger={false}
                  />
                </div>
              )}
            </>
          )}
        </div>
      </div>
    )
  }

  // ===== Main Mode =====
  return (
    <div style={{ background: THEME.bg, minHeight: '100vh' }}>
      <div style={{ maxWidth: 1200, margin: '0 auto', padding: '32px 24px' }}>
        {/* Header */}
        <div style={{ marginBottom: 24 }}>
          <h1 style={{ margin: 0, fontSize: 24, fontWeight: 800, color: THEME.textMain }}>
            {effectiveIndustryLabel} 题库
          </h1>
          <p style={{ margin: '6px 0 0', fontSize: 14, color: THEME.textSecondary }}>
            按关键词、难度、分类筛选题目，或直接开始练习
          </p>
        </div>

        {/* Filter Bar */}
        <div
          style={{
            ...cardBase,
            padding: '16px 20px',
            marginBottom: 16,
            display: 'flex',
            alignItems: 'center',
            gap: 12,
            flexWrap: 'wrap',
          }}
        >
          <Select
            value={effectiveIndustryCode}
            disabled={industriesQuery.isLoading || !industriesQuery.data?.length}
            onChange={handleIndustryChange}
            style={{ minWidth: 120, borderRadius: 10 }}
            dropdownStyle={{ borderRadius: 10 }}
            placeholder="行业"
          >
            {industriesQuery.data?.map((industry) => (
              <Select.Option key={industry.id} value={industry.code}>{industry.name}</Select.Option>
            ))}
            {!industriesQuery.data?.length && (
              <Select.Option value={effectiveIndustryCode}>{effectiveIndustryLabel}</Select.Option>
            )}
          </Select>

          <Select
            value={difficulty || undefined}
            onChange={(val) => navigatePractice({ difficulty: val || '', page: 1 })}
            allowClear
            placeholder="难度"
            style={{ minWidth: 100, borderRadius: 10 }}
            dropdownStyle={{ borderRadius: 10 }}
          >
            <Select.Option value="easy">简单</Select.Option>
            <Select.Option value="medium">中等</Select.Option>
            <Select.Option value="hard">困难</Select.Option>
          </Select>

          <Select
            value={categoryId || undefined}
            onChange={(val) => navigatePractice({ category: val || undefined, page: 1 })}
            allowClear
            placeholder="分类"
            style={{ minWidth: 140, borderRadius: 10 }}
            dropdownStyle={{ borderRadius: 10 }}
          >
            {categoryOptions.map((item) => (
              <Select.Option key={item.id} value={item.id}>{item.name}</Select.Option>
            ))}
          </Select>

          <form onSubmit={handleSearchSubmit} style={{ flex: 1, minWidth: 180, display: 'flex' }}>
            <Input
              prefix={<SearchOutlined style={{ color: THEME.textMuted }} />}
              value={keywordInput}
              onChange={(e) => setKeywordInput(e.target.value)}
              placeholder="搜索题目关键词"
              style={{ borderRadius: 10 }}
              allowClear
            />
          </form>

          <Button
            size="small"
            onClick={() => void handleGenerateExam('random')}
            style={{ borderRadius: 8, fontWeight: 600 }}
          >
            <ThunderboltOutlined /> 随机练习
          </Button>
          <Button
            size="small"
            onClick={() => void handleGenerateExam('timed')}
            style={{ borderRadius: 8, fontWeight: 600 }}
          >
            <FireOutlined /> 限时模拟
          </Button>
        </div>

        {/* Stats Bar */}
        {accessToken && statsQuery.data && (
          <div
            style={{
              ...cardBase,
              padding: '14px 20px',
              marginBottom: 16,
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'space-between',
              flexWrap: 'wrap',
              gap: '12px 24px',
            }}
          >
            <StatItem label="今日完成" value={String(statsQuery.data.today_count)} color={THEME.primary} />
            <StatItem label="累计作答" value={String(statsQuery.data.total_answered)} color={THEME.textMain} />
            <StatItem label="正确率" value={`${statsQuery.data.accuracy_rate.toFixed(0)}%`} color={THEME.success} />
            <StatItem label="连续打卡" value={`${statsQuery.data.streak_days} 天`} color={THEME.warning} />
            <StatItem label="错题待复习" value={String(collectionsOverviewQuery.data?.wrongQuestions ?? 0)} color={THEME.danger} />
            <StatItem label="已收藏" value={String(collectionsOverviewQuery.data?.favorites ?? 0)} color={THEME.accent} />
          </div>
        )}

        {/* Context Banner */}
        {(routeSearch.title || routeSearch.reason || focusTags.length || routeSearch.questionSet || routeSearch.topic) && (
          <div
            style={{
              ...cardBase,
              padding: '14px 20px',
              marginBottom: 16,
              background: '#eff6ff',
              border: `1px solid ${THEME.accent}20`,
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'space-between',
              flexWrap: 'wrap',
              gap: 12,
            }}
          >
            <div style={{ display: 'flex', alignItems: 'center', gap: 10, flexWrap: 'wrap' }}>
              <span style={{ fontSize: 13, fontWeight: 600, color: THEME.accent }}>
                {resolvePracticeRouteSourceLabel(routeSearch.source || '')}
              </span>
              <span style={{ fontSize: 13, color: THEME.textMain }}>{routeSearch.title || '当前补练上下文'}</span>
              {routeSearch.reason && (
                <span style={{ fontSize: 12, color: THEME.textSecondary }}>{routeSearch.reason}</span>
              )}
            </div>
            <Button size="small" onClick={handleClearPracticeContext} style={{ borderRadius: 8 }}>
              <CloseCircleOutlined /> 清空上下文
            </Button>
          </div>
        )}

        {examMessage && (
          <div
            style={{
              ...cardBase,
              padding: '12px 20px',
              marginBottom: 16,
              background: '#f0fdf4',
              border: `1px solid ${THEME.success}20`,
              fontSize: 13,
              color: THEME.success,
            }}
          >
            {examMessage}
          </div>
        )}

        {/* Main Layout */}
        <div style={{ display: 'grid', gridTemplateColumns: '1fr 300px', gap: 24 }}>
          {/* Left: Table */}
          <div>
            {questionsQuery.isLoading && (
              <div style={{ padding: 60, textAlign: 'center' }}>
                <Spin tip="题目列表加载中..." />
              </div>
            )}

            {questionsQuery.isError && (
              <div style={{ padding: 40, textAlign: 'center', color: THEME.danger }}>
                {extractErrorMessage(questionsQuery.error, '题目列表加载失败')}
              </div>
            )}

            {questionsQuery.data && (
              <>
                {questions.length ? (
                  <>
                    <QuestionTable
                      questions={questions}
                      industries={industriesQuery.data || []}
                      fallbackIndustryCode={effectiveIndustryCode}
                    />
                    <div style={{ marginTop: 20, display: 'flex', justifyContent: 'center' }}>
                      <Pagination
                        current={page}
                        total={questionsQuery.data.total}
                        pageSize={PRACTICE_PAGE_SIZE}
                        onChange={(p) => navigatePractice({ page: p })}
                        showSizeChanger={false}
                      />
                    </div>
                  </>
                ) : (
                  <Empty
                    image={Empty.PRESENTED_IMAGE_SIMPLE}
                    description={
                      <div>
                        <div style={{ fontWeight: 600, color: THEME.textMain, marginBottom: 4 }}>当前筛选条件下暂无题目</div>
                        <Button
                          onClick={() =>
                            navigatePractice({
                              keyword: '',
                              difficulty: '',
                              category: undefined,
                              page: 1,
                              questionSet: undefined,
                              topic: undefined,
                              focus: undefined,
                              source: undefined,
                              title: undefined,
                              reason: undefined,
                            })
                          }
                        >
                          重置筛选
                        </Button>
                      </div>
                    }
                  />
                )}
              </>
            )}
          </div>

          {/* Right: Sidebar */}
          <div style={{ display: 'flex', flexDirection: 'column', gap: 20 }}>
            {/* Recommendations */}
            {accessToken && (
              <div style={{ ...cardBase, padding: 20 }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 14 }}>
                  <RocketOutlined style={{ fontSize: 16, color: THEME.primary }} />
                  <span style={{ fontSize: 14, fontWeight: 700, color: THEME.textMain }}>推荐补练</span>
                </div>

                {practiceRecommendationsQuery.isLoading && <Spin size="small" />}

                {practiceRecommendationsQuery.data?.items?.length ? (
                  <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
                    {practiceRecommendationsQuery.data.items.map((item) => {
                      const linkedTopic = item.topic_code ? recommendationTopicMap.get(item.topic_code) || null : null
                      const collectionSearch = buildPracticeRecommendationRouteSearch(
                        {
                          focus_tag: item.focus_tag,
                          topic_code: item.topic_code,
                          primary_question_set: item.primary_question_set,
                          reason: item.reason,
                          question_title: item.question.title,
                        },
                        linkedTopic,
                      )
                      const diffColor = difficultyColor[item.question.difficulty] || THEME.textMuted
                      return (
                        <div
                          key={`rec-${item.question.id}`}
                          style={{
                            padding: '10px 12px',
                            borderRadius: 10,
                            border: `1px solid ${THEME.border}`,
                            background: '#fafaf9',
                            transition: 'all 0.2s ease',
                          }}
                          onMouseEnter={(e) => {
                            e.currentTarget.style.borderColor = THEME.primary
                          }}
                          onMouseLeave={(e) => {
                            e.currentTarget.style.borderColor = THEME.border
                          }}
                        >
                          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 4 }}>
                            <span
                              style={{
                                fontSize: 13,
                                fontWeight: 600,
                                color: THEME.textMain,
                                overflow: 'hidden',
                                textOverflow: 'ellipsis',
                                whiteSpace: 'nowrap',
                                flex: 1,
                              }}
                            >
                              {item.question.title}
                            </span>
                            <Tag
                              style={{
                                margin: 0,
                                borderRadius: 8,
                                fontSize: 10,
                                fontWeight: 600,
                                color: diffColor,
                                background: `${diffColor}10`,
                                border: 'none',
                                flexShrink: 0,
                              }}
                            >
                              {difficultyLabel(item.question.difficulty)}
                            </Tag>
                          </div>
                          <div style={{ fontSize: 11, color: THEME.textMuted, marginBottom: 6 }}>
                            {item.focus_tag}
                          </div>
                          <Link
                            to="/practice"
                            search={collectionSearch}
                            style={{ fontSize: 12, color: THEME.primary, fontWeight: 600, textDecoration: 'none' }}
                          >
                            进入补练 →
                          </Link>
                        </div>
                      )
                    })}
                  </div>
                ) : !practiceRecommendationsQuery.isLoading ? (
                  <div style={{ fontSize: 12, color: THEME.textMuted }}>
                    先做几道题，系统会根据错因给出推荐
                  </div>
                ) : null}
              </div>
            )}

            {/* Question Sets */}
            <div style={{ ...cardBase, padding: 20 }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 14 }}>
                <BookOutlined style={{ fontSize: 16, color: THEME.accent }} />
                <span style={{ fontSize: 14, fontWeight: 700, color: THEME.textMain }}>核心题单</span>
              </div>

              {questionSetsQuery.isLoading && <Spin size="small" />}

              {questionSetsQuery.data?.length ? (
                <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
                  {questionSetsQuery.data.map((set) => {
                    const setColor = resolveSetColor(set.slug)
                    return (
                      <button
                        key={set.slug}
                        type="button"
                        onClick={() => handleApplyQuestionSetContext(set.slug)}
                        style={{
                          display: 'flex',
                          alignItems: 'flex-start',
                          gap: 0,
                          padding: 0,
                          borderRadius: 10,
                          border: `1px solid ${THEME.border}`,
                          background: THEME.cardBg,
                          cursor: 'pointer',
                          textAlign: 'left',
                          transition: 'all 0.2s ease',
                          width: '100%',
                          overflow: 'hidden',
                        }}
                        onMouseEnter={(e) => {
                          e.currentTarget.style.borderColor = setColor
                          e.currentTarget.style.boxShadow = THEME.shadowCard
                        }}
                        onMouseLeave={(e) => {
                          e.currentTarget.style.borderColor = THEME.border
                          e.currentTarget.style.boxShadow = 'none'
                        }}
                      >
                        <div style={{ width: 4, background: setColor, flexShrink: 0, alignSelf: 'stretch' }} />
                        <div style={{ padding: '10px 12px', flex: 1, minWidth: 0 }}>
                          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 8, marginBottom: 3 }}>
                            <span style={{ fontSize: 13, fontWeight: 600, color: THEME.textMain, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                              {set.title}
                            </span>
                            <Tag color="blue" style={{ margin: 0, fontSize: 11, fontWeight: 600, flexShrink: 0 }}>
                              {set.question_count} 题
                            </Tag>
                          </div>
                          {set.focus_tags?.length ? (
                            <div style={{ display: 'flex', gap: 4, flexWrap: 'wrap' }}>
                              {set.focus_tags.slice(0, 2).map((tag) => (
                                <span key={tag} style={{ fontSize: 11, color: THEME.textMuted }}>#{tag}</span>
                              ))}
                              {set.focus_tags.length > 2 ? (
                                <span style={{ fontSize: 11, color: THEME.textMuted }}>+{set.focus_tags.length - 2}</span>
                              ) : null}
                            </div>
                          ) : null}
                        </div>
                      </button>
                    )
                  })}
                </div>
              ) : !questionSetsQuery.isLoading ? (
                <div style={{ fontSize: 12, color: THEME.textMuted }}>暂无题单</div>
              ) : null}
            </div>

            {/* Quick Links */}
            <div style={{ ...cardBase, padding: 20 }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 14 }}>
                <RiseOutlined style={{ fontSize: 16, color: THEME.success }} />
                <span style={{ fontSize: 14, fontWeight: 700, color: THEME.textMain }}>快速入口</span>
              </div>
              <div style={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
                {[
                  { to: '/practice/wrong', label: '错题本', icon: <UndoOutlined /> },
                  { to: '/practice/favorites', label: '收藏夹', icon: <HeartOutlined /> },
                  { to: '/practice/notes', label: '学习笔记', icon: <EditOutlined /> },
                  { to: '/growth', label: '成长档案', icon: <TrophyOutlined /> },
                ].map((item) => {
                  const requiresAuth = !accessToken
                  const content = (
                    <div
                      key={item.to}
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
                  if (requiresAuth) {
                    return (
                      <button
                        key={item.to}
                        type="button"
                        onClick={() => requestLoginPrompt(item.to, 'missing')}
                        style={{ all: 'unset', display: 'block', width: '100%' }}
                      >
                        {content}
                      </button>
                    )
                  }
                  return (
                    <Link
                      key={item.to}
                      to={item.to}
                      preload="intent"
                      style={{ textDecoration: 'none', display: 'block' }}
                    >
                      {content}
                    </Link>
                  )
                })}
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}

/* ===== Sub Components ===== */

function StatItem({ label, value, color }: { label: string; value: string; color: string }) {
  return (
    <div style={{ display: 'flex', alignItems: 'baseline', gap: 6 }}>
      <span style={{ fontSize: 18, fontWeight: 800, color, lineHeight: 1 }}>{value}</span>
      <span style={{ fontSize: 12, color: THEME.textMuted }}>{label}</span>
    </div>
  )
}

function QuestionTable({
  questions,
  industries,
  fallbackIndustryCode,
}: {
  questions: Array<{
    id: number
    title: string
    difficulty: string
    type: string
    category_id: number
    industry_id: number
    category_name?: string
    pass_rate?: number
    is_favorite?: boolean
    is_answered?: boolean
  }>
  industries: Array<{ id: number; code: string; name: string }>
  fallbackIndustryCode: string
}) {
  return (
    <div
      style={{
        ...cardBase,
        overflow: 'hidden',
      }}
    >
      {/* Table Header */}
      <div
        style={{
          display: 'grid',
          gridTemplateColumns: '44px 1fr 120px 80px 60px 44px',
          gap: 12,
          padding: '12px 20px',
          background: '#fafaf9',
          borderBottom: `1px solid ${THEME.border}`,
          fontSize: 12,
          fontWeight: 600,
          color: THEME.textMuted,
          textTransform: 'uppercase',
          letterSpacing: 0.5,
        }}
      >
        <span>状态</span>
        <span>题目</span>
        <span style={{ textAlign: 'center' }}>分类</span>
        <span style={{ textAlign: 'center' }}>通过率</span>
        <span style={{ textAlign: 'center' }}>难度</span>
        <span></span>
      </div>

      {/* Table Rows */}
      <div>
        {questions.map((question, index) => {
          const passRate = typeof question.pass_rate === 'number' ? question.pass_rate : null
          const passRateColor = passRate === null ? THEME.textMuted : passRate >= 70 ? THEME.success : passRate >= 40 ? THEME.warning : THEME.danger
          const diffColor = difficultyColor[question.difficulty] || THEME.textMuted
          const diffBg = difficultyBg[question.difficulty] || '#f5f5f4'

          return (
            <Link
              key={question.id}
              to={
                question.type === 'code'
                  ? '/practice/editor/$questionId'
                  : '/practice/$questionId'
              }
              params={{ questionId: String(question.id) }}
              style={{ textDecoration: 'none', display: 'block' }}
            >
              <div
                style={{
                  display: 'grid',
                  gridTemplateColumns: '44px 1fr 120px 80px 60px 44px',
                  gap: 12,
                  alignItems: 'center',
                  padding: '12px 20px',
                  borderBottom: index === questions.length - 1 ? 'none' : `1px solid ${THEME.border}`,
                  transition: 'background 0.15s ease',
                  cursor: 'pointer',
                }}
                onMouseEnter={(e) => {
                  e.currentTarget.style.background = '#fafaf9'
                }}
                onMouseLeave={(e) => {
                  e.currentTarget.style.background = 'transparent'
                }}
              >
                {/* Status */}
                <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
                  {question.is_answered ? (
                    <CheckCircleFilled style={{ fontSize: 16, color: THEME.success }} />
                  ) : question.is_favorite ? (
                    <StarFilled style={{ fontSize: 16, color: THEME.warning }} />
                  ) : (
                    <CheckCircleOutlined style={{ fontSize: 16, color: '#d6d3d1' }} />
                  )}
                </div>

                {/* Title + Type */}
                <div style={{ minWidth: 0 }}>
                  <div
                    style={{
                      fontSize: 14,
                      fontWeight: 600,
                      color: THEME.textMain,
                      overflow: 'hidden',
                      textOverflow: 'ellipsis',
                      whiteSpace: 'nowrap',
                      marginBottom: 2,
                    }}
                  >
                    {question.title}
                  </div>
                  <div style={{ fontSize: 11, color: THEME.textMuted }}>
                    {questionTypeLabel(question.type)} · #{question.id}
                  </div>
                </div>

                {/* Category */}
                <div style={{ textAlign: 'center', fontSize: 12, color: THEME.textSecondary, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                  {question.category_name || '-'}
                </div>

                {/* Pass Rate */}
                <div style={{ textAlign: 'center' }}>
                  {passRate !== null ? (
                    <div style={{ display: 'flex', alignItems: 'center', gap: 6, justifyContent: 'center' }}>
                      <div
                        style={{
                          width: 32,
                          height: 4,
                          borderRadius: 2,
                          background: '#e7e5e4',
                          overflow: 'hidden',
                        }}
                      >
                        <div
                          style={{
                            width: `${passRate}%`,
                            height: '100%',
                            borderRadius: 2,
                            background: passRateColor,
                          }}
                        />
                      </div>
                      <span style={{ fontSize: 11, fontWeight: 600, color: passRateColor }}>{passRate}%</span>
                    </div>
                  ) : (
                    <span style={{ fontSize: 11, color: THEME.textMuted }}>-</span>
                  )}
                </div>

                {/* Difficulty */}
                <div style={{ textAlign: 'center' }}>
                  <Tag
                    style={{
                      margin: 0,
                      borderRadius: 8,
                      fontSize: 11,
                      fontWeight: 600,
                      color: diffColor,
                      background: diffBg,
                      border: 'none',
                    }}
                  >
                    {difficultyLabel(question.difficulty)}
                  </Tag>
                </div>

                {/* Favorite */}
                <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
                  {question.is_favorite ? (
                    <StarFilled style={{ fontSize: 14, color: THEME.warning }} />
                  ) : (
                    <StarFilled style={{ fontSize: 14, color: '#e7e5e4' }} />
                  )}
                </div>
              </div>
            </Link>
          )
        })}
      </div>
    </div>
  )
}
