import { useEffect, useMemo, useState } from 'react'
import {
  DEFAULT_FRONTEND_INDUSTRY_CODE,
  type FrontendIndustry,
  formatFrontendIndustryLabel,
  persistSelectedFrontendIndustryCode,
  readSelectedFrontendIndustryCode,
  resolvePreferredFrontendIndustry,
  subscribeFrontendIndustryCodeChange,
} from './industryContext'
import { useFrontendIndustriesQuery } from './frontendQueries'

/**
 * 根据行业主键从前台行业列表中定位真实行业对象，供详情页展示真实方向名称。
 */
export function findFrontendIndustryById(industries: FrontendIndustry[], industryId?: number): FrontendIndustry | null {
  if (!industryId) {
    return null
  }

  return industries.find((item) => item.id === industryId) || null
}

/**
 * 统一维护前台行业偏好，让导航、首页和工作台能共享同一份方向上下文。
 */
export function useFrontendIndustryPreference() {
  const [selectedIndustryCode, setSelectedIndustryCode] = useState(() => readSelectedFrontendIndustryCode() || DEFAULT_FRONTEND_INDUSTRY_CODE)
  const industriesQuery = useFrontendIndustriesQuery()

  const selectedIndustry = useMemo(
    () => resolvePreferredFrontendIndustry(industriesQuery.data || [], selectedIndustryCode),
    [industriesQuery.data, selectedIndustryCode],
  )
  const effectiveIndustryCode = selectedIndustry?.code || selectedIndustryCode.trim() || DEFAULT_FRONTEND_INDUSTRY_CODE
  const effectiveIndustryLabel = formatFrontendIndustryLabel(selectedIndustry, effectiveIndustryCode)

  useEffect(() => {
    const unsubscribe = subscribeFrontendIndustryCodeChange((industryCode) => {
      setSelectedIndustryCode(industryCode || DEFAULT_FRONTEND_INDUSTRY_CODE)
    })

    return unsubscribe
  }, [])

  useEffect(() => {
    const normalizedIndustryCode = effectiveIndustryCode.trim()
    if (!normalizedIndustryCode) {
      return
    }

    persistSelectedFrontendIndustryCode(normalizedIndustryCode)
    if (normalizedIndustryCode !== selectedIndustryCode) {
      setSelectedIndustryCode(normalizedIndustryCode)
    }
  }, [effectiveIndustryCode, selectedIndustryCode])

  return {
    industriesQuery,
    selectedIndustry,
    selectedIndustryCode,
    setSelectedIndustryCode,
    effectiveIndustryCode,
    effectiveIndustryLabel,
  }
}
