import { requestJson } from '@makejob/api-client'
import { isSuccessCode, type ApiEnvelope } from '@makejob/shared-types'

interface PageResult<T> {
  list: T[]
  total: number
  page: number
  page_size: number
}

export interface PracticeCollectionsOverview {
  favorites: number
  wrongQuestions: number
  notes: number
}

/**
 * 拉取指定刷题收藏列表的总数，供首页和题库总览统一展示聚合数据。
 */
async function fetchCollectionTotal(token: string, path: string, fallbackMessage: string): Promise<number> {
  const response = await requestJson<ApiEnvelope<PageResult<unknown>>>(`${path}?page=1&page_size=1`, {
    token,
  })

  if (!isSuccessCode(response.code) || !response.data) {
    throw new Error(response.message || fallbackMessage)
  }

  return response.data.total
}

/**
 * 汇总收藏、错题和笔记总量，供刷题域入口页展示当前积累规模。
 */
export async function fetchPracticeCollectionsOverview(token: string): Promise<PracticeCollectionsOverview> {
  const [favorites, wrongQuestions, notes] = await Promise.all([
    fetchCollectionTotal(token, '/user/favorites', '获取收藏列表失败'),
    fetchCollectionTotal(token, '/user/wrong-questions', '获取错题本失败'),
    fetchCollectionTotal(token, '/user/notes', '获取笔记列表失败'),
  ])

  return {
    favorites,
    wrongQuestions,
    notes,
  }
}
