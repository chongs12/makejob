import { requestJson } from '@makejob/api-client'
import { isSuccessCode, type ApiEnvelope } from '@makejob/shared-types'

export interface FrontendIndustry {
  id: number
  code: string
  name: string
  description?: string
  is_active?: boolean
}

export const DEFAULT_FRONTEND_INDUSTRY_CODE = 'go'

const SELECTED_FRONTEND_INDUSTRY_CODE_KEY = 'makejob.companion.selected-industry-code'
const FRONTEND_INDUSTRY_CODE_CHANGE_EVENT = 'makejob.frontend-industry-code-change'

/**
 * 读取前台最近一次选择的行业编码，让不同页面可以共用同一份方向偏好。
 */
export function readSelectedFrontendIndustryCode(): string {
  if (typeof window === 'undefined') {
    return ''
  }

  return window.localStorage.getItem(SELECTED_FRONTEND_INDUSTRY_CODE_KEY) || ''
}

/**
 * 持久化当前选中的行业编码，供陪伴、面试和刷题页面共享。
 */
export function persistSelectedFrontendIndustryCode(industryCode: string): void {
  if (typeof window === 'undefined') {
    return
  }

  window.localStorage.setItem(SELECTED_FRONTEND_INDUSTRY_CODE_KEY, industryCode)
  window.dispatchEvent(new CustomEvent<string>(FRONTEND_INDUSTRY_CODE_CHANGE_EVENT, {
    detail: industryCode,
  }))
}

/**
 * 订阅前台行业偏好变化，兼容同页切换和跨标签页同步两种场景。
 */
export function subscribeFrontendIndustryCodeChange(listener: (industryCode: string) => void): () => void {
  if (typeof window === 'undefined') {
    return () => {}
  }

  /**
   * 处理同页内的自定义事件，确保顶栏和其他旁路区域能立刻拿到最新行业编码。
   */
  function handleIndustryChange(event: Event): void {
    listener((event as CustomEvent<string>).detail || '')
  }

  /**
   * 处理浏览器原生 storage 事件，让多标签页之间也能共享同一份方向偏好。
   */
  function handleStorage(event: StorageEvent): void {
    if (event.key !== SELECTED_FRONTEND_INDUSTRY_CODE_KEY) {
      return
    }

    listener(event.newValue || '')
  }

  window.addEventListener(FRONTEND_INDUSTRY_CODE_CHANGE_EVENT, handleIndustryChange as EventListener)
  window.addEventListener('storage', handleStorage)

  return () => {
    window.removeEventListener(FRONTEND_INDUSTRY_CODE_CHANGE_EVENT, handleIndustryChange as EventListener)
    window.removeEventListener('storage', handleStorage)
  }
}

/**
 * 根据当前偏好和可用行业列表解析最终应使用的行业对象。
 */
export function resolvePreferredFrontendIndustry(
  industries: FrontendIndustry[],
  preferredCode: string,
  fallbackCode = DEFAULT_FRONTEND_INDUSTRY_CODE,
): FrontendIndustry | null {
  const normalizedPreferredCode = preferredCode.trim()
  if (normalizedPreferredCode) {
    const exactIndustry = industries.find((item) => item.code === normalizedPreferredCode)
    if (exactIndustry) {
      return exactIndustry
    }
  }

  return industries.find((item) => item.code === fallbackCode) || industries[0] || null
}

/**
 * 将行业对象或编码转换为可直接展示在页面上的方向名称。
 */
export function formatFrontendIndustryLabel(industry: FrontendIndustry | null, industryCode: string): string {
  if (industry?.name) {
    return industry.name
  }

  const normalizedCode = industryCode.trim()
  if (!normalizedCode) {
    return '默认方向'
  }

  return normalizedCode.toUpperCase()
}

/**
 * 拉取前台可见行业列表，供业务页面构建行业切换器。
 */
export async function fetchFrontendIndustries(): Promise<FrontendIndustry[]> {
  const response = await requestJson<ApiEnvelope<FrontendIndustry[]>>('/industries')

  if (!isSuccessCode(response.code) || !response.data) {
    throw new Error(response.message || '获取行业列表失败')
  }

  return response.data
}
