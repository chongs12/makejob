const PENDING_PRACTICE_SEARCH_KEY = 'makejob.practice.pending-search'

/**
 * 暂存待执行的题库搜索词，供跳转到题库页后自动接管筛选。
 */
export function persistPendingPracticeSearch(keyword: string): void {
  if (typeof window === 'undefined') {
    return
  }

  if (keyword.trim()) {
    window.localStorage.setItem(PENDING_PRACTICE_SEARCH_KEY, keyword.trim())
  } else {
    window.localStorage.removeItem(PENDING_PRACTICE_SEARCH_KEY)
  }
}

/**
 * 读取并清空待执行的题库搜索词，避免同一关键词重复触发。
 */
export function consumePendingPracticeSearch(): string {
  if (typeof window === 'undefined') {
    return ''
  }

  const keyword = window.localStorage.getItem(PENDING_PRACTICE_SEARCH_KEY) || ''
  window.localStorage.removeItem(PENDING_PRACTICE_SEARCH_KEY)
  return keyword.trim()
}
