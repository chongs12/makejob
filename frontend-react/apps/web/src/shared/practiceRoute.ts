interface PracticeFocusQuestionSetMeta {
  slug: string
  title: string
  keyword: string
}

interface PracticeWeeklyFocusRouteIntent {
  title: string
  reason: string
  focus_tags: string[]
}

interface PracticeLinkedTopicContext {
  code: string
  related_question_sets: string[]
}

interface PracticeMistakeTopicRouteIntent {
  code: string
  tag: string
  title: string
  problem_pattern: string
  related_question_sets?: string[]
}

interface PracticeRecommendationRouteIntent {
  focus_tag: string
  topic_code?: string
  reason: string
  question_title: string
}

export interface PracticeRouteSearch {
  industry?: string
  keyword?: string
  difficulty?: string
  category?: number
  page?: number
  questionSet?: string
  topic?: string
  focus?: string
  source?: string
  title?: string
  reason?: string
}

export interface PracticeRouteIntent {
  industryCode?: string
  keyword?: string
  difficulty?: string
  categoryId?: number | null
  page?: number
  questionSetSlug?: string
  topicCode?: string
  focusTags?: string[]
  source?: string
  title?: string
  reason?: string
}

const PRACTICE_FOCUS_QUESTION_SET_META: PracticeFocusQuestionSetMeta[] = [
  { slug: 'go-runtime-core', title: 'Go 运行时基础题单', keyword: 'slice' },
  { slug: 'go-concurrency-debug', title: '并发控制与排错题单', keyword: 'channel' },
  { slug: 'gin-backend-flow', title: 'Gin 后端实战题单', keyword: 'Gin' },
  { slug: 'gorm-database-core', title: 'GORM 与数据库题单', keyword: 'GORM' },
  { slug: 'microservice-network', title: '微服务与网络基础题单', keyword: 'gRPC' },
  { slug: 'algorithm-structure', title: '算法与数据结构补强题单', keyword: '数据结构' },
]

/**
 * 根据题单 slug 解析更适合前端展示的中文标题。
 */
export function resolvePracticeQuestionSetTitle(slug: string): string {
  const normalizedSlug = slug.trim()
  if (!normalizedSlug) {
    return ''
  }

  const matched = PRACTICE_FOCUS_QUESTION_SET_META.find((item) => item.slug === normalizedSlug)
  return matched?.title || normalizedSlug
}

/**
 * 为“去题库补练”动作挑选一个稳定可用的搜索关键词。
 */
export function resolvePracticeFocusKeyword(questionSetSlug: string, focusTags: string[], fallbackTitle: string): string {
  const matched = PRACTICE_FOCUS_QUESTION_SET_META.find((item) => item.slug === questionSetSlug.trim())
  if (matched?.keyword) {
    return matched.keyword
  }

  const firstTag = focusTags.map((item) => item.trim()).find(Boolean)
  if (firstTag) {
    return firstTag
  }

  return fallbackTitle.trim()
}

/**
 * 解析题库路由中的 focus 参数，统一还原为去重后的专题标签数组。
 */
export function readPracticeRouteFocusTags(search: PracticeRouteSearch): string[] {
  return normalizePracticeFocusTags((search.focus || '').split(','))
}

/**
 * 根据路由中的来源编码返回更适合展示给用户的中文标签。
 */
export function resolvePracticeRouteSourceLabel(source: string): string {
  const map: Record<string, string> = {
    header_search: '顶部搜索',
    weekly_focus: '本周补强主题',
    mistake_topic: '错因专题',
    practice_recommendation: '练习推荐',
    interview_follow_up: '面试补练',
    question_set: '核心题单',
  }

  return map[source.trim()] || '补练入口'
}

/**
 * 将任意来源的输入对象规范化为题库页可复用的正式路由搜索协议。
 */
export function buildPracticeRouteSearch(intent: PracticeRouteIntent): PracticeRouteSearch {
  const normalizedFocusTags = normalizePracticeFocusTags(intent.focusTags || [])
  const normalizedQuestionSetSlug = String(intent.questionSetSlug || '').trim()
  const shouldFallbackToKeyword = !normalizedQuestionSetSlug
  const keywordFallback = shouldFallbackToKeyword && normalizedFocusTags.length
    ? resolvePracticeFocusKeyword('', normalizedFocusTags, intent.title || '')
    : ''
  const keyword = (intent.keyword || keywordFallback).trim()

  return normalizePracticeRouteSearch({
    industry: intent.industryCode,
    keyword,
    difficulty: intent.difficulty,
    category: intent.categoryId || undefined,
    page: intent.page,
    questionSet: intent.questionSetSlug,
    topic: intent.topicCode,
    focus: normalizedFocusTags.join(','),
    source: intent.source,
    title: intent.title,
    reason: intent.reason,
  })
}

/**
 * 对题库路由搜索对象做最小清洗，避免把空值和非法页码带进 URL。
 */
export function normalizePracticeRouteSearch(search: Partial<PracticeRouteSearch>): PracticeRouteSearch {
  const normalized: PracticeRouteSearch = {}
  const industry = String(search.industry || '').trim()
  const keyword = String(search.keyword || '').trim()
  const difficulty = String(search.difficulty || '').trim()
  const questionSet = String(search.questionSet || '').trim()
  const topic = String(search.topic || '').trim()
  const focus = normalizePracticeFocusTags(String(search.focus || '').split(',')).join(',')
  const source = String(search.source || '').trim()
  const title = String(search.title || '').trim()
  const reason = String(search.reason || '').trim()
  const page = Number(search.page)
  const category = Number(search.category)

  if (industry) {
    normalized.industry = industry
  }
  if (keyword) {
    normalized.keyword = keyword
  }
  if (difficulty) {
    normalized.difficulty = difficulty
  }
  if (Number.isFinite(category) && category > 0) {
    normalized.category = category
  }
  if (Number.isFinite(page) && page > 1) {
    normalized.page = Math.floor(page)
  }
  if (questionSet) {
    normalized.questionSet = questionSet
  }
  if (topic) {
    normalized.topic = topic
  }
  if (focus) {
    normalized.focus = focus
  }
  if (source) {
    normalized.source = source
  }
  if (title) {
    normalized.title = title
  }
  if (reason) {
    normalized.reason = reason
  }

  return normalized
}

/**
 * 为本周补强主题构造稳定的题库跳转协议，优先落到专题绑定的正式题单。
 */
export function buildWeeklyFocusPracticeRouteSearch(
  theme: PracticeWeeklyFocusRouteIntent,
  linkedTopic?: PracticeLinkedTopicContext | null,
): PracticeRouteSearch {
  return buildPracticeRouteSearch({
    questionSetSlug: linkedTopic?.related_question_sets[0] || '',
    topicCode: linkedTopic?.code,
    focusTags: theme.focus_tags,
    source: 'weekly_focus',
    title: theme.title,
    reason: theme.reason,
  })
}

/**
 * 为错因专题详情页构造题库跳转协议，优先命中关联题单，其次保留专题标签上下文。
 */
export function buildMistakeTopicPracticeRouteSearch(
  topic: PracticeMistakeTopicRouteIntent,
  questionSetSlug = '',
): PracticeRouteSearch {
  return buildPracticeRouteSearch({
    questionSetSlug,
    focusTags: [topic.tag],
    topicCode: topic.code,
    source: 'mistake_topic',
    title: topic.title,
    reason: topic.problem_pattern,
  })
}

/**
 * 为面试报告页的“去题库补弱项”动作构造稳定跳转，命中专题时优先进入正式题单。
 */
export function buildInterviewFollowUpPracticeRouteSearch(
  keyword: string,
  linkedTopic?: PracticeMistakeTopicRouteIntent | null,
): PracticeRouteSearch {
  if (linkedTopic) {
    return buildPracticeRouteSearch({
      questionSetSlug: linkedTopic.related_question_sets?.[0] || '',
      focusTags: [linkedTopic.tag],
      topicCode: linkedTopic.code,
      source: 'interview_follow_up',
      title: '面试后补练',
      reason: linkedTopic.problem_pattern || '根据当前面试报告中最需要继续追打的弱项生成。',
    })
  }

  return buildPracticeRouteSearch({
    keyword,
    focusTags: [keyword],
    source: 'interview_follow_up',
    title: '面试后补练',
    reason: '根据当前面试报告中最需要继续追打的弱项生成。',
  })
}

/**
 * 为练习推荐卡片构造稳定的补练入口，优先落到错因专题绑定的正式题单，其次保留专题与标签上下文。
 */
export function buildPracticeRecommendationRouteSearch(
  recommendation: PracticeRecommendationRouteIntent,
  linkedTopic?: PracticeMistakeTopicRouteIntent | null,
): PracticeRouteSearch {
  return buildPracticeRouteSearch({
    questionSetSlug: linkedTopic?.related_question_sets?.[0] || '',
    focusTags: [linkedTopic?.tag || recommendation.focus_tag],
    topicCode: linkedTopic?.code || recommendation.topic_code,
    source: 'practice_recommendation',
    title: recommendation.question_title,
    reason: recommendation.reason,
  })
}

/**
 * 解析路由原始查询参数，并收敛为题库页可直接消费的稳定结构。
 */
export function validatePracticeRouteSearch(rawSearch: Record<string, unknown>): PracticeRouteSearch {
  const getSingleValue = (value: unknown): string => {
    if (typeof value === 'string') {
      return value
    }
    if (Array.isArray(value)) {
      const matched = value.find((item) => typeof item === 'string')
      return typeof matched === 'string' ? matched : ''
    }
    if (typeof value === 'number' && Number.isFinite(value)) {
      return String(value)
    }
    return ''
  }

  const pageValue = Number(getSingleValue(rawSearch.page))
  const categoryValue = Number(getSingleValue(rawSearch.category))

  return normalizePracticeRouteSearch({
    industry: getSingleValue(rawSearch.industry),
    keyword: getSingleValue(rawSearch.keyword),
    difficulty: getSingleValue(rawSearch.difficulty),
    category: Number.isFinite(categoryValue) ? categoryValue : undefined,
    page: Number.isFinite(pageValue) ? pageValue : undefined,
    questionSet: getSingleValue(rawSearch.questionSet),
    topic: getSingleValue(rawSearch.topic),
    focus: getSingleValue(rawSearch.focus),
    source: getSingleValue(rawSearch.source),
    title: getSingleValue(rawSearch.title),
    reason: getSingleValue(rawSearch.reason),
  })
}

/**
 * 统一清理专题标签数组，避免重复值和空白项污染题库搜索协议。
 */
function normalizePracticeFocusTags(tags: string[]): string[] {
  return Array.from(new Set(tags.map((item) => item.trim()).filter(Boolean)))
}
