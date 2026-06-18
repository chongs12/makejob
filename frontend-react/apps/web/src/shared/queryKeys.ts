import type { QueryClient } from '@tanstack/react-query'

const FRONTEND_INDUSTRIES_QUERY_ROOT = ['frontend', 'industries'] as const
const PRACTICE_STATS_QUERY_ROOT = ['frontend', 'practice-stats'] as const
const PRACTICE_CATEGORIES_QUERY_ROOT = ['practice', 'categories'] as const
const PRACTICE_QUESTIONS_QUERY_ROOT = ['practice', 'questions'] as const
const PRACTICE_QUESTION_SETS_QUERY_ROOT = ['practice', 'question-sets'] as const
const PRACTICE_QUESTION_SET_DETAIL_QUERY_ROOT = ['practice', 'question-set-detail'] as const
const PRACTICE_COLLECTIONS_OVERVIEW_QUERY_ROOT = ['practice', 'collections-overview'] as const
const PRACTICE_RECOMMENDATIONS_QUERY_ROOT = ['practice', 'recommendations'] as const
const INTERVIEW_HISTORY_QUERY_ROOT = ['interview', 'history'] as const
const COMPANION_CURRENT_PLAN_QUERY_ROOT = ['companion', 'current-plan'] as const
const COMPANION_PLAN_PROGRESS_QUERY_ROOT = ['companion', 'plan-progress'] as const
const COMPANION_WEEKLY_FOCUS_QUERY_ROOT = ['companion', 'weekly-focus'] as const
const COMPANION_CATEGORY_OPTIONS_QUERY_ROOT = ['companion', 'category-options'] as const
const COMPANION_LIVE2D_MODELS_QUERY_ROOT = ['companion', 'live2d-models'] as const

/**
 * 生成前台行业列表查询键，供首页、题库、面试和陪伴页复用同一份缓存。
 */
export function buildFrontendIndustriesQueryKey() {
  return FRONTEND_INDUSTRIES_QUERY_ROOT
}

/**
 * 生成练习统计查询键，统一当前用户在不同业务页中的答题统计缓存。
 */
export function buildPracticeStatsQueryKey(accessToken: string | null) {
  return [...PRACTICE_STATS_QUERY_ROOT, accessToken] as const
}

/**
 * 生成题库分类树查询键，保证按行业维度区分缓存结果。
 */
export function buildPracticeCategoriesQueryKey(industryCode: string) {
  return [...PRACTICE_CATEGORIES_QUERY_ROOT, industryCode] as const
}

/**
 * 生成题库题目列表查询键，统一分页和筛选组合下的缓存定位方式。
 */
export function buildPracticeQuestionsQueryKey(params: {
  page: number
  industryCode: string
  industryId: number | null
  difficulty: string
  keyword: string
  categoryId: number | null
}) {
  return [
    ...PRACTICE_QUESTIONS_QUERY_ROOT,
    params.page,
    params.industryCode,
    params.industryId,
    params.difficulty,
    params.keyword,
    params.categoryId,
  ] as const
}

/**
 * 生成题单摘要列表查询键，按当前行业区分缓存。
 */
export function buildPracticeQuestionSetsQueryKey(industryCode: string | null) {
  return [...PRACTICE_QUESTION_SETS_QUERY_ROOT, industryCode] as const
}

/**
 * 生成题单详情查询键，确保详情缓存和题单摘要缓存能明确拆分。
 */
export function buildPracticeQuestionSetDetailQueryKey(industryCode: string | null, slug: string) {
  return [...PRACTICE_QUESTION_SET_DETAIL_QUERY_ROOT, industryCode, slug] as const
}

/**
 * 生成收藏概览查询键，统一首页和题库页的个人沉淀数据缓存。
 */
export function buildPracticeCollectionsOverviewQueryKey(accessToken: string | null) {
  return [...PRACTICE_COLLECTIONS_OVERVIEW_QUERY_ROOT, accessToken] as const
}

/**
 * 生成个性化练习推荐查询键，统一题库页推荐区的缓存入口。
 */
export function buildPracticeRecommendationsQueryKey(accessToken: string | null) {
  return [...PRACTICE_RECOMMENDATIONS_QUERY_ROOT, accessToken] as const
}

/**
 * 生成面试历史查询键，统一面试入口与后续刷新动作的缓存定位。
 */
export function buildInterviewHistoryQueryKey(accessToken: string | null) {
  return [...INTERVIEW_HISTORY_QUERY_ROOT, accessToken] as const
}

/**
 * 生成学习陪伴当前计划查询键，让入口页和房间页共享同一份计划缓存。
 */
export function buildCompanionCurrentPlanQueryKey(accessToken: string | null) {
  return [...COMPANION_CURRENT_PLAN_QUERY_ROOT, accessToken] as const
}

/**
 * 生成学习陪伴计划进度查询键，统一入口页和房间页的进度快照缓存。
 */
export function buildCompanionPlanProgressQueryKey(accessToken: string | null, planId?: number | null) {
  return [...COMPANION_PLAN_PROGRESS_QUERY_ROOT, accessToken, planId ?? null] as const
}

/**
 * 生成学习陪伴周重点查询键，统一入口页在不同刷新入口下的缓存命中。
 */
export function buildCompanionWeeklyFocusQueryKey(accessToken: string | null) {
  return [...COMPANION_WEEKLY_FOCUS_QUERY_ROOT, accessToken] as const
}

/**
 * 生成学习陪伴弱项分类查询键，按当前行业缓存入口页的弱项建议。
 */
export function buildCompanionCategoryOptionsQueryKey(industryCode: string) {
  return [...COMPANION_CATEGORY_OPTIONS_QUERY_ROOT, industryCode] as const
}

/**
 * 生成学习陪伴 Live2D 模型查询键，按行业维度缓存可选模型列表。
 */
export function buildCompanionLive2DModelsQueryKey(industryCode: string) {
  return [...COMPANION_LIVE2D_MODELS_QUERY_ROOT, industryCode] as const
}

/**
 * 统一失效学习陪伴计划相关缓存，避免入口页和房间页各自维护一套刷新名单。
 */
export async function invalidateCompanionPlanQueries(queryClient: QueryClient): Promise<void> {
  await Promise.all([
    queryClient.invalidateQueries({ queryKey: COMPANION_CURRENT_PLAN_QUERY_ROOT }),
    queryClient.invalidateQueries({ queryKey: COMPANION_PLAN_PROGRESS_QUERY_ROOT }),
  ])
}

/**
 * 统一失效面试历史缓存，确保创建新会话后入口页历史列表立即刷新。
 */
export async function invalidateInterviewHistoryQueries(queryClient: QueryClient): Promise<void> {
  await queryClient.invalidateQueries({ queryKey: INTERVIEW_HISTORY_QUERY_ROOT })
}
