import { useQuery } from '@tanstack/react-query'
import { fetchFrontendIndustries } from './industryContext'
import { fetchPracticeStats } from './practiceCatalog'
import { buildFrontendIndustriesQueryKey, buildPracticeStatsQueryKey } from './queryKeys'

/**
 * 统一拉取前台行业列表，确保首页、题库、面试和陪伴页复用同一份缓存配置。
 */
export function useFrontendIndustriesQuery() {
  return useQuery({
    queryKey: buildFrontendIndustriesQueryKey(),
    queryFn: fetchFrontendIndustries,
    staleTime: 5 * 60 * 1000,
    gcTime: 10 * 60 * 1000,
  })
}

/**
 * 统一拉取当前用户的练习统计，减少首页、题库和陪伴页切换时的重复请求。
 */
export function usePracticeStatsQuery(accessToken: string | null) {
  return useQuery({
    queryKey: buildPracticeStatsQueryKey(accessToken),
    queryFn: () => fetchPracticeStats(accessToken as string),
    enabled: Boolean(accessToken),
    retry: false,
    staleTime: 60 * 1000,
  })
}
