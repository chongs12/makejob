import type { FormEvent } from 'react'
import { useEffect, useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Link, useNavigate, useSearch } from '@tanstack/react-router'
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
import { AsyncEmptyState, AsyncInlineState, AsyncStatusCard } from '../../shared/asyncState'
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
  const [examMessage, setExamMessage] = useState('等待组卷')
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
    }),
    enabled: !activeQuestionSetSlug,
  })

  const questionSetsQuery = useQuery({
    queryKey: buildPracticeQuestionSetsQueryKey(selectedIndustry?.id || null),
    queryFn: () => fetchQuestionSets(selectedIndustry?.id || null),
    enabled: Boolean(selectedIndustry?.id),
  })

  const activeQuestionSetQuery = useQuery({
    queryKey: buildPracticeQuestionSetDetailQueryKey(selectedIndustry?.id || null, activeQuestionSetSlug),
    queryFn: () => fetchQuestionSetDetail(selectedIndustry?.id || null, activeQuestionSetSlug),
    enabled: Boolean(activeQuestionSetSlug),
  })
  const activeTopicCode = routeSearch.topic || ''
  const activeTopicQuery = useQuery({
    queryKey: ['mistake-topic-detail', activeTopicCode],
    queryFn: () => fetchMistakeTopic(activeTopicCode),
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
    queryFn: () => fetchPracticeRecommendations(accessToken as string, 6),
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
    queryFn: () => fetchMistakeTopics(recommendationTopicCodes),
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
    () => filterPracticeCollectionQuestions(activeQuestionSetQuery.data?.questions || [], {
      keyword,
      difficulty,
    }),
    [activeQuestionSetQuery.data?.questions, difficulty, keyword],
  )

  /**
   * 合并并清洗题库路由参数后统一跳转，保证筛选状态和补练上下文都能固化到 URL。
   */
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

  /**
   * 在行业列表恢复后同步前台公共偏好，保证刷题、面试和陪伴使用同一方向上下文。
   */
  useEffect(() => {
    const normalizedIndustryCode = effectiveIndustryCode.trim()
    if (!normalizedIndustryCode) {
      return
    }

    persistSelectedFrontendIndustryCode(normalizedIndustryCode)
  }, [effectiveIndustryCode])

  /**
   * 当路由中的题库搜索条件变化时，同步更新输入框展示值，保证刷新和回退后仍能还原状态。
   */
  useEffect(() => {
    setKeywordInput(keyword)
  }, [keyword])

  /**
   * 应用搜索条件并回到第一页，避免保留过期分页状态。
   */
  function handleSearchSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    navigatePractice({
      keyword: keywordInput.trim(),
      page: 1,
    })
  }

  /**
   * 切换刷题行业时重置分类和分页，避免沿用上一行业的筛选状态。
   */
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

  /**
   * 生成随机练习或限时模拟，并跳转到第一道题。
   */
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
        params: {
          questionId: String(firstQuestion.id),
        },
      })
    } catch (error) {
      if (!useAuthStore.getState().accessToken) {
        requestLoginPrompt('/practice', 'expired')
        return
      }
      setExamMessage(extractErrorMessage(error, '组卷失败'))
    }
  }

  /**
   * 清空当前补练上下文，只保留用户手动选择的基础筛选条件。
   */
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

  /**
   * 在当前补练上下文中切换到指定题单，尽量把模糊专题入口收敛成稳定题目集合。
   */
  function handleApplyQuestionSetContext(questionSetSlug: string): void {
    navigatePractice({
      questionSet: questionSetSlug,
      page: 1,
    })
  }

  return (
    <section className="page-panel">
      <span className="page-tag">刷题总览</span>
      <h1>刷题模式</h1>
      <p className="page-copy">
        这一版已经接入真实题目列表、练习统计、错题本、收藏夹、笔记和代码题编辑器。当前题库方向：{effectiveIndustryLabel}。
      </p>

      <div className="channel-portal-grid">
        <article className="channel-entry-card">
          <span className="section-kicker">题目训练</span>
          <h2>从筛题到做题</h2>
          <p>按关键词、难度、分类缩小范围，直接进入普通题详情页或代码题编辑器。</p>
          <Link className="secondary-link" to="/practice">当前页继续筛题</Link>
        </article>
        <article className="channel-entry-card">
          <span className="section-kicker">复盘沉淀</span>
          <h2>错题、收藏、笔记</h2>
          <p>把高频错题、值得重做的题和个人题解收束到同一个练习域里，形成复盘闭环。</p>
          {accessToken ? (
            <Link className="secondary-link" to="/practice/notes">直接看笔记</Link>
          ) : (
            <button className="secondary-link interactive-link-button" type="button" onClick={() => requestLoginPrompt('/practice/notes', 'missing')}>
              直接看笔记
            </button>
          )}
        </article>
        <article className="channel-entry-card">
          <span className="section-kicker">模拟练习</span>
          <h2>随机练习与限时模拟</h2>
          <p>从题库入口直接生成练习流，不额外跳后台工具页，保持单一训练主线。</p>
          <button className="secondary-button" type="button" onClick={() => void handleGenerateExam('timed')}>
            立即开始模拟
          </button>
        </article>
      </div>

      <div className="quick-links">
        {accessToken ? (
          <Link className="secondary-link" to="/practice/wrong">进入错题本</Link>
        ) : (
          <button className="secondary-link interactive-link-button" type="button" onClick={() => requestLoginPrompt('/practice/wrong', 'missing')}>
            进入错题本
          </button>
        )}
        {accessToken ? (
          <Link className="secondary-link" to="/practice/favorites">查看收藏夹</Link>
        ) : (
          <button className="secondary-link interactive-link-button" type="button" onClick={() => requestLoginPrompt('/practice/favorites', 'missing')}>
            查看收藏夹
          </button>
        )}
        {accessToken ? (
          <Link className="secondary-link" to="/practice/notes">查看笔记</Link>
        ) : (
          <button className="secondary-link interactive-link-button" type="button" onClick={() => requestLoginPrompt('/practice/notes', 'missing')}>
            查看笔记
          </button>
        )}
      </div>

      {(routeSearch.title || routeSearch.reason || focusTags.length || routeSearch.questionSet || routeSearch.topic) ? (
        <article className="status-card" style={{ marginTop: 24 }}>
          <div className="card-inline">
            <div>
              <span className="section-kicker">{resolvePracticeRouteSourceLabel(routeSearch.source || '')}</span>
              <h2>{routeSearch.title || '当前补练上下文'}</h2>
            </div>
            <button className="secondary-button" type="button" onClick={handleClearPracticeContext}>
              清空上下文
            </button>
          </div>
          {routeSearch.reason ? <p style={{ marginTop: 12 }}>{routeSearch.reason}</p> : null}
          {routeSearch.questionSet ? (
            <p style={{ marginTop: 12 }}>
              当前题单：<strong>{resolvePracticeQuestionSetTitle(routeSearch.questionSet)}</strong>
            </p>
          ) : null}
          {activeTopicQuery.data ? (
            <p style={{ marginTop: 12 }}>
              当前专题：<strong>{activeTopicQuery.data.title}</strong>
            </p>
          ) : null}
          {focusTags.length ? (
            <div className="community-tag-row" style={{ marginTop: 12 }}>
              {focusTags.map((tag) => (
                <span key={`practice-focus-${tag}`}>{tag}</span>
              ))}
            </div>
          ) : null}
          {routeSearch.topic ? (
            <div className="page-actions" style={{ marginTop: 12 }}>
              <Link className="secondary-link" to={resolveMistakeTopicRoute()} params={{ topicCode: routeSearch.topic }}>
                查看对应错因专题
              </Link>
            </div>
          ) : null}
          {activeTopicQuery.isLoading ? <AsyncInlineState message="正在读取当前专题上下文..." style={{ marginTop: 12 }} /> : null}
          {activeTopicQuery.isError ? (
            <AsyncInlineState
              message={extractErrorMessage(activeTopicQuery.error, '专题上下文读取失败')}
              style={{ marginTop: 12 }}
              tone="error"
            />
          ) : null}
          {activeTopicQuery.data?.problem_pattern && activeTopicQuery.data.problem_pattern !== routeSearch.reason ? (
            <p style={{ marginTop: 12 }}>{activeTopicQuery.data.problem_pattern}</p>
          ) : null}
          {activeTopicQuery.data?.related_question_sets.length && !routeSearch.questionSet ? (
            <div style={{ marginTop: 16 }}>
              <strong>该专题已绑定正式题单，建议直接进入以下练习集合</strong>
              <div className="page-actions" style={{ marginTop: 12, flexWrap: 'wrap' }}>
                {activeTopicQuery.data.related_question_sets.map((item) => (
                  <button className="secondary-button" key={item} type="button" onClick={() => handleApplyQuestionSetContext(item)}>
                    {resolvePracticeQuestionSetTitle(item)}
                  </button>
                ))}
              </div>
            </div>
          ) : null}
          {activeQuestionSetQuery.isLoading ? <AsyncInlineState message="正在加载当前题单..." style={{ marginTop: 12 }} /> : null}
          {activeQuestionSetQuery.isError ? (
            <AsyncInlineState
              message={extractErrorMessage(activeQuestionSetQuery.error, '题单详情加载失败')}
              style={{ marginTop: 12 }}
              tone="error"
            />
          ) : null}
          {activeQuestionSetQuery.data ? (
            <div style={{ marginTop: 16 }}>
              <strong>当前已进入正式题单模式</strong>
              <p style={{ marginTop: 12 }}>
                下方主列表会固定在
                <strong> {activeQuestionSetQuery.data.title} </strong>
                这组题目内继续筛选；你现在改关键词或难度，不会退回到全量题库搜索。
              </p>
              {activeQuestionSetQuery.data.questions.length ? (
                <div style={{ marginTop: 12 }}>
                  {activeQuestionSetQuery.data.questions.slice(0, 3).map((item) => (
                    <div key={`active-question-set-preview-${item.id}`} style={{ marginBottom: 8 }}>
                      <Link className="secondary-link" to={resolvePracticeTarget(item.id, item.type)} params={{ questionId: String(item.id) }}>
                        {item.title}
                      </Link>
                    </div>
                  ))}
                </div>
              ) : null}
            </div>
          ) : null}
        </article>
      ) : null}

      {accessToken && statsQuery.data ? (
        <div className="stats-grid">
          <article className="feature-card">
            <h2>累计作答</h2>
            <p>{statsQuery.data.total_answered}</p>
          </article>
          <article className="feature-card">
            <h2>正确数</h2>
            <p>{statsQuery.data.correct_count}</p>
          </article>
          <article className="feature-card">
            <h2>正确率</h2>
            <p>{statsQuery.data.accuracy_rate.toFixed(2)}%</p>
          </article>
          <article className="feature-card">
            <h2>连续练习</h2>
            <p>{statsQuery.data.streak_days} 天</p>
          </article>
          <article className="feature-card">
            <h2>错题待复习</h2>
            <p>{collectionsOverviewQuery.data?.wrongQuestions ?? '-'}</p>
          </article>
          <article className="feature-card">
            <h2>已收藏</h2>
            <p>{collectionsOverviewQuery.data?.favorites ?? '-'}</p>
          </article>
          <article className="feature-card">
            <h2>笔记沉淀</h2>
            <p>{collectionsOverviewQuery.data?.notes ?? '-'}</p>
          </article>
          <article className="feature-card">
            <h2>今日完成</h2>
            <p>{statsQuery.data.today_count}</p>
          </article>
        </div>
      ) : null}

      {accessToken ? (
        <article className="status-card" style={{ marginTop: 24 }}>
          <div className="card-inline">
            <div>
              <span className="section-kicker">对症练习推荐</span>
              <h2>先补最近反复暴露的问题</h2>
            </div>
            <Link className="secondary-link" to="/practice/wrong">查看错题本</Link>
          </div>

          {practiceRecommendationsQuery.isLoading ? <AsyncInlineState message="正在根据最近错因生成推荐..." style={{ marginTop: 12 }} /> : null}

          {practiceRecommendationsQuery.isError ? (
            <AsyncInlineState
              message={extractErrorMessage(practiceRecommendationsQuery.error, '练习推荐加载失败')}
              style={{ marginTop: 12 }}
              tone="error"
            />
          ) : null}

          {practiceRecommendationsQuery.data?.focus_tags.length ? (
            <div className="community-tag-row" style={{ marginTop: 12 }}>
              {practiceRecommendationsQuery.data.focus_tags.map((tag) => (
                <span key={tag}>{tag}</span>
              ))}
            </div>
          ) : null}

          {practiceRecommendationsQuery.data?.items.length ? (
            <div className="grid-cards" style={{ marginTop: 18 }}>
              {practiceRecommendationsQuery.data.items.map((item) => {
                const linkedTopic = item.topic_code ? recommendationTopicMap.get(item.topic_code) || null : null
                const collectionSearch = buildPracticeRecommendationRouteSearch({
                  focus_tag: item.focus_tag,
                  topic_code: item.topic_code,
                  primary_question_set: item.primary_question_set,
                  reason: item.reason,
                  question_title: item.question.title,
                }, linkedTopic)

                return (
                  <article className="feature-card" key={`practice-recommendation-${item.question.id}`}>
                    <div className="card-inline">
                      <strong>{item.question.title}</strong>
                      <span>{difficultyLabel(item.question.difficulty)}</span>
                    </div>
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
                    <p>题型：{questionTypeLabel(item.question.type)}</p>
                    {item.recommended_actions?.length ? (
                      <ul className="interview-bullet-list" style={{ marginTop: 12 }}>
                        {item.recommended_actions.map((action) => (
                          <li key={`${item.question.id}-${action}`}>{action}</li>
                        ))}
                      </ul>
                    ) : null}
                    <div className="page-actions" style={{ marginTop: 12 }}>
                      <Link
                        className="secondary-link"
                        to="/practice"
                        search={collectionSearch}
                      >
                        进入这组补练
                      </Link>
                      <Link
                        className="secondary-link"
                        to={resolvePracticeRecommendationRoute(item.question.type)}
                        params={{ questionId: String(item.question.id) }}
                      >
                        直接开始补练
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
            <AsyncEmptyState
              title="还没有形成推荐"
              message="先做几道编程题或主观题，系统会根据错因标签逐步给出更具体的补题建议。"
              style={{ marginTop: 18 }}
            />
          ) : null}
        </article>
      ) : null}

      <article className="status-card" style={{ marginTop: 24 }}>
        <div className="card-inline">
          <div>
            <span className="section-kicker">核心题单</span>
            <h2>{effectiveIndustryLabel} 最值得先打通的主题</h2>
          </div>
          <Link className="secondary-link" to="/practice">继续按筛选做题</Link>
        </div>

        {questionSetsQuery.isLoading ? <AsyncInlineState message="正在整理当前方向的核心题单..." style={{ marginTop: 12 }} /> : null}

        {questionSetsQuery.isError ? (
          <AsyncInlineState
            message={extractErrorMessage(questionSetsQuery.error, '核心题单加载失败')}
            style={{ marginTop: 12 }}
            tone="error"
          />
        ) : null}

        {questionSetsQuery.data?.length ? (
          <div className="grid-cards" style={{ marginTop: 18 }}>
            {questionSetsQuery.data.map((set) => (
              <article className="feature-card" key={set.slug}>
                <div className="card-inline">
                  <strong>{set.title}</strong>
                  <span>{set.question_count} 题</span>
                </div>
                <p>{set.description}</p>
                {set.focus_tags.length ? (
                  <div className="community-tag-row" style={{ marginTop: 12 }}>
                    {set.focus_tags.map((tag) => (
                      <span key={`${set.slug}-${tag}`}>{tag}</span>
                    ))}
                  </div>
                ) : null}
                <div className="page-actions" style={{ marginTop: 12 }}>
                  <Link
                    className="secondary-link"
                    to="/practice"
                    search={buildPracticeRouteSearch({
                      industryCode: effectiveIndustryCode,
                      questionSetSlug: set.slug,
                      focusTags: set.focus_tags,
                      source: 'question_set',
                      title: set.title,
                      reason: set.description,
                    })}
                  >
                    进入本题单
                  </Link>
                </div>
                {set.questions.length ? (
                  <div style={{ marginTop: 12 }}>
                    {set.questions.map((item) => (
                      <div key={`${set.slug}-question-${item.id}`} style={{ marginBottom: 8 }}>
                        <Link
                          className="secondary-link"
                          to={resolvePracticeTarget(item.id, item.type)}
                          params={{ questionId: String(item.id) }}
                        >
                          {item.title}
                        </Link>
                      </div>
                    ))}
                  </div>
                ) : null}
              </article>
            ))}
          </div>
        ) : null}

        {!questionSetsQuery.isLoading && !questionSetsQuery.isError && !questionSetsQuery.data?.length ? (
          <AsyncEmptyState
            title="当前方向还没有整理出核心题单"
            message="优先补齐该行业下的高价值题目后，这里会自动收敛成更稳定的主题入口。"
            style={{ marginTop: 18 }}
          />
        ) : null}
      </article>

      <form className="stack-form" onSubmit={handleSearchSubmit}>
        <label className="field">
          <span>行业筛选</span>
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
          <span>搜索题目</span>
          <input
            value={keywordInput}
            onChange={(event) => setKeywordInput(event.target.value)}
            placeholder="输入关键词"
          />
        </label>

        <label className="field">
          <span>难度筛选</span>
          <select
            value={difficulty}
            onChange={(event) => {
              navigatePractice({
                difficulty: event.target.value,
                page: 1,
              })
            }}
          >
            <option value="">全部</option>
            <option value="easy">简单</option>
            <option value="medium">中等</option>
            <option value="hard">困难</option>
          </select>
        </label>

        <label className="field">
          <span>分类筛选</span>
          <select
            value={categoryId || ''}
            disabled={isQuestionSetCollectionMode}
            onChange={(event) => {
              navigatePractice({
                category: event.target.value ? Number(event.target.value) : undefined,
                page: 1,
              })
            }}
          >
            <option value="">全部分类</option>
            {categoryOptions.map((item) => (
              <option key={item.id} value={item.id}>{item.name}</option>
            ))}
          </select>
        </label>
        {isQuestionSetCollectionMode ? (
          <AsyncInlineState
            className="companion-empty-text"
            message="正式题单模式下会优先固定题目集合，分类筛选已暂时关闭。"
          />
        ) : null}

        {industriesQuery.isError ? (
          <AsyncInlineState
            className="companion-empty-text"
            message={extractErrorMessage(industriesQuery.error, '行业列表读取失败，当前将回退到默认题库方向。')}
            tone="error"
          />
        ) : null}

        <div className="page-actions">
          <button className="primary-button" type="submit">
            搜索
          </button>
          <button className="secondary-button" type="button" onClick={() => void handleGenerateExam('random')}>
            随机练习
          </button>
          <button className="secondary-button" type="button" onClick={() => void handleGenerateExam('timed')}>
            限时模拟
          </button>
        </div>
      </form>

      <div className="status-card" style={{ marginTop: 24 }}>
        练习提示：{examMessage}
      </div>

      {!isQuestionSetCollectionMode && questionsQuery.isLoading ? <AsyncStatusCard message="题目列表加载中..." style={{ marginTop: 24 }} /> : null}

      {!isQuestionSetCollectionMode && questionsQuery.isError ? (
        <AsyncStatusCard
          message={questionsQuery.error instanceof Error ? questionsQuery.error.message : '题目列表加载失败'}
          style={{ marginTop: 24 }}
          tone="error"
        />
      ) : null}

      {isQuestionSetCollectionMode && activeQuestionSetQuery.data ? (
        <>
          <div className="status-card" style={{ marginTop: 24 }}>
            <div className="card-inline">
              <div>
                <span className="section-kicker">正式练习集合</span>
                <h2>{activeQuestionSetQuery.data.title}</h2>
              </div>
              <span>{filteredQuestionSetQuestions.length}/{activeQuestionSetQuery.data.question_count} 题</span>
            </div>
            <p style={{ marginTop: 12 }}>{activeQuestionSetQuery.data.description}</p>
          </div>

          {filteredQuestionSetQuestions.length ? (
            <div className="grid-cards" style={{ marginTop: 24 }}>
              {filteredQuestionSetQuestions.map((question) => (
                <article className="feature-card" key={`question-set-mode-${question.id}`}>
                  <div className="card-inline">
                    <strong>#{question.id}</strong>
                    <span>{difficultyLabel(question.difficulty)}</span>
                  </div>
                  <h2>{question.title}</h2>
                  <p>题型：{questionTypeLabel(question.type)}</p>
                  <p>来源：{resolvePracticeQuestionSetTitle(activeQuestionSetSlug)}</p>
                  <div style={{ marginTop: 12 }}>
                    <Link className="secondary-link" to={resolvePracticeTarget(question.id, question.type)} params={{ questionId: String(question.id) }}>
                      进入做题
                    </Link>
                  </div>
                </article>
              ))}
            </div>
          ) : (
            <AsyncEmptyState
              title="当前题单下没有命中结果"
              message="可以切换关键词或难度继续筛选，这里仍然只会展示当前正式题单内的题目。"
              style={{ marginTop: 24 }}
            />
          )}
        </>
      ) : null}

      {!isQuestionSetCollectionMode && questionsQuery.data ? (
        <>
          {questionsQuery.data.list.length ? (
            <div className="grid-cards" style={{ marginTop: 24 }}>
              {questionsQuery.data.list.map((question) => (
                <article className="feature-card" key={question.id}>
                  <div className="card-inline">
                    <strong>#{question.id}</strong>
                    <span>{difficultyLabel(question.difficulty)}</span>
                  </div>
                  <h2>{question.title}</h2>
                  <p>行业：{formatFrontendIndustryLabel(findFrontendIndustryById(industriesQuery.data || [], question.industry_id), effectiveIndustryCode)}</p>
                  <p>题型：{questionTypeLabel(question.type)}</p>
                  <p>分类：{question.category_name || question.category_id}</p>
                  <p>通过率：{typeof question.pass_rate === 'number' ? `${question.pass_rate}%` : '暂无'}</p>
                  <div style={{ marginTop: 12 }}>
                    <Link className="secondary-link" to={question.type === 'code' ? '/practice/editor/$questionId' : '/practice/$questionId'} params={{ questionId: String(question.id) }}>
                      进入做题
                    </Link>
                  </div>
                </article>
              ))}
            </div>
          ) : (
            <AsyncEmptyState
              title="当前筛选条件下暂无题目"
              message="可以切换行业、难度或分类后再试，或者直接使用随机练习快速开始。"
              style={{ marginTop: 24 }}
            />
          )}

          <div className="card-inline" style={{ marginTop: 24 }}>
            <span>共 {questionsQuery.data.total} 题</span>
            <div className="page-actions">
              <button
                className="secondary-button"
                type="button"
                disabled={page <= 1}
                onClick={() => navigatePractice({ page: Math.max(page - 1, 1) })}
              >
                上一页
              </button>
              <span>第 {page} 页</span>
              <button
                className="secondary-button"
                type="button"
                disabled={questionsQuery.data.list.length < PRACTICE_PAGE_SIZE}
                onClick={() => navigatePractice({ page: page + 1 })}
              >
                下一页
              </button>
            </div>
          </div>
        </>
      ) : null}
    </section>
  )
}
