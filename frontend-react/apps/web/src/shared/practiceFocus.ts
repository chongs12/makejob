import { persistPendingPracticeSearch } from './practiceSearch'

interface PracticeFocusQuestionSetMeta {
  slug: string
  title: string
  keyword: string
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
 * 将补练关键词写入题库页待执行搜索缓存，便于跳转后自动接管筛选。
 */
export function persistPracticeFocusSearch(questionSetSlug: string, focusTags: string[], fallbackTitle: string): string {
  const keyword = resolvePracticeFocusKeyword(questionSetSlug, focusTags, fallbackTitle)
  persistPendingPracticeSearch(keyword)
  return keyword
}
