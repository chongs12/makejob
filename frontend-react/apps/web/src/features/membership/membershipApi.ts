import { requestJson } from '@makejob/api-client'
import { isSuccessCode, type ApiEnvelope } from '@makejob/shared-types'

/**
 * 统一校验会员相关接口响应，失败抛错。
 */
function unwrapMembershipData<T>(response: ApiEnvelope<T>, fallbackMessage: string): T {
  if (!isSuccessCode(response.code) || !response.data) {
    throw new Error(response.message || fallbackMessage)
  }
  return response.data
}

export interface MembershipPlan {
  plan_type: string
  name: string
  price: number
  duration_days: number
  features: string[]
}

export interface MembershipInfo {
  level: string
  expire_at?: string
  is_active: boolean
  daily_practice_limit: number
  daily_interview_limit: number
  practice_used_today: number
  interview_used_today: number
}

export interface MembershipOrder {
  id: number
  order_no: string
  plan_type: string
  amount: number
  status: string
  created_at?: string
  paid_at?: string
  expires_at?: string
}

export interface MembershipOrdersPage {
  orders: MembershipOrder[]
  total: number
}

/**
 * 拉取当前用户会员状态。
 */
export async function fetchMembershipInfo(token: string): Promise<MembershipInfo> {
  const response = await requestJson<ApiEnvelope<MembershipInfo>>('/membership/info', { token })
  return unwrapMembershipData(response, '获取会员状态失败')
}

/**
 * 拉取可用套餐列表。
 */
export async function fetchMembershipPlans(token: string): Promise<MembershipPlan[]> {
  const response = await requestJson<ApiEnvelope<MembershipPlan[]>>('/membership/plans', { token })
  return unwrapMembershipData(response, '获取套餐列表失败')
}

/**
 * 创建一条 pending 订单。
 */
export async function createMembershipOrder(token: string, planType: string): Promise<MembershipOrder> {
  const response = await requestJson<ApiEnvelope<MembershipOrder>>('/membership/orders', {
    method: 'POST',
    token,
    body: { plan_type: planType },
  })
  return unwrapMembershipData(response, '创建订单失败')
}

/**
 * 预支付阶段模拟支付：对当前用户自己的 pending 订单完成支付，触发会员生效。
 */
export async function mockPayMembershipOrder(token: string, orderId: number): Promise<MembershipOrder> {
  const response = await requestJson<ApiEnvelope<MembershipOrder>>('/membership/checkout/mock-pay', {
    method: 'POST',
    token,
    body: { order_id: orderId },
  })
  return unwrapMembershipData(response, '支付失败')
}

/**
 * 分页拉取订单记录。
 */
export async function fetchMembershipOrders(token: string, page = 1, pageSize = 10): Promise<MembershipOrdersPage> {
  const response = await requestJson<ApiEnvelope<MembershipOrdersPage>>(`/membership/orders?page=${page}&page_size=${pageSize}`, { token })
  return unwrapMembershipData(response, '获取订单记录失败')
}
