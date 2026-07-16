import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Button, Spin, Tag, message as antdMessage } from 'antd'
import { CrownOutlined, CheckCircleOutlined } from '@ant-design/icons'
import { extractErrorMessage } from '@makejob/api-client'
import { useAuthStore } from '../../state/auth'
import {
  createMembershipOrder,
  fetchMembershipInfo,
  fetchMembershipOrders,
  fetchMembershipPlans,
  mockPayMembershipOrder,
  type MembershipPlan,
} from './membershipApi'

const THEME = {
  bg: '#f8f9fa',
  cardBg: '#ffffff',
  primary: '#f97316',
  primaryLight: '#fff7ed',
  textMain: '#1f2937',
  textSecondary: '#6b7280',
  textMuted: '#9ca3af',
  border: '#f3f4f6',
  borderHover: '#e5e7eb',
  shadow: '0 1px 2px rgba(0,0,0,0.05)',
  shadowCard: '0 4px 6px -1px rgba(0,0,0,0.07), 0 2px 4px -2px rgba(0,0,0,0.05)',
  radius: 12,
  radiusSm: 8,
  success: '#22c55e',
}

const PLAN_LABEL: Record<string, string> = {
  monthly: '月度会员',
  quarterly: '季度会员',
  yearly: '年度会员',
}

function levelLabel(level: string): string {
  if (level === 'free' || !level) return '免费用户'
  return PLAN_LABEL[level] || level
}

function isPaidLevel(level: string): boolean {
  return level !== 'free' && level !== ''
}

function formatExpire(value?: string): string {
  if (!value) return '—'
  const d = new Date(value)
  if (Number.isNaN(d.getTime())) return value
  return d.toLocaleDateString('zh-CN')
}

/**
 * 会员中心页：展示当前套餐/到期、可开通套餐、订单记录。
 * 免费用户看升级入口，付费用户看状态与续费。
 */
export function MembershipPage() {
  const accessToken = useAuthStore((state) => state.accessToken)
  const fetchMembership = useAuthStore((state) => state.fetchMembership)
  const queryClient = useQueryClient()
  const [busyPlan, setBusyPlan] = useState<string>('')

  const infoQuery = useQuery({
    queryKey: ['membership', 'info'],
    queryFn: () => fetchMembershipInfo(accessToken as string),
    enabled: Boolean(accessToken),
    retry: false,
  })

  const plansQuery = useQuery({
    queryKey: ['membership', 'plans'],
    queryFn: () => fetchMembershipPlans(accessToken as string),
    enabled: Boolean(accessToken),
    retry: false,
  })

  const ordersQuery = useQuery({
    queryKey: ['membership', 'orders'],
    queryFn: () => fetchMembershipOrders(accessToken as string),
    enabled: Boolean(accessToken),
    retry: false,
  })

  const upgradeMutation = useMutation({
    mutationFn: async (planType: string) => {
      const order = await createMembershipOrder(accessToken as string, planType)
      return mockPayMembershipOrder(accessToken as string, order.id)
    },
    onSuccess: async () => {
      antdMessage.success('开通成功，会员已生效')
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['membership'] }),
        fetchMembership(),
      ])
      setBusyPlan('')
    },
    onError: (error) => {
      antdMessage.error(extractErrorMessage(error, '开通失败，请稍后重试'))
      setBusyPlan('')
    },
  })

  const currentLevel = infoQuery.data?.level || 'free'
  const paid = isPaidLevel(currentLevel)

  async function handleUpgrade(planType: string) {
    setBusyPlan(planType)
    upgradeMutation.mutate(planType)
  }

  const cardStyle = {
    background: THEME.cardBg,
    borderRadius: THEME.radius,
    border: `1px solid ${THEME.border}`,
    boxShadow: THEME.shadow,
    padding: '24px',
  }

  return (
    <div style={{ minHeight: '100vh', background: THEME.bg }}>
      <div style={{ background: THEME.cardBg, borderBottom: `1px solid ${THEME.border}`, boxShadow: THEME.shadow }}>
        <div style={{ maxWidth: 1000, margin: '0 auto', padding: '20px 24px' }}>
          <h1 style={{ fontSize: 22, fontWeight: 700, color: THEME.textMain, margin: 0, display: 'flex', alignItems: 'center', gap: 8 }}>
            <CrownOutlined style={{ color: THEME.primary }} />
            会员中心
          </h1>
        </div>
      </div>

      <div style={{ maxWidth: 1000, margin: '0 auto', padding: '24px', display: 'flex', flexDirection: 'column', gap: 24 }}>
        {/* 当前状态 */}
        <div style={cardStyle}>
          <h2 style={{ fontSize: 16, fontWeight: 700, color: THEME.textMain, margin: '0 0 16px' }}>当前状态</h2>
          {infoQuery.isLoading ? (
            <Spin />
          ) : infoQuery.data ? (
            <div style={{ display: 'flex', gap: 32, flexWrap: 'wrap' }}>
              <div>
                <div style={{ fontSize: 12, color: THEME.textSecondary, marginBottom: 4 }}>套餐</div>
                <Tag color={paid ? 'gold' : 'default'} style={{ fontSize: 14, padding: '4px 12px' }}>
                  {levelLabel(currentLevel)}
                </Tag>
              </div>
              <div>
                <div style={{ fontSize: 12, color: THEME.textSecondary, marginBottom: 4 }}>到期时间</div>
                <div style={{ fontSize: 14, fontWeight: 600, color: THEME.textMain }}>{formatExpire(infoQuery.data.expire_at)}</div>
              </div>
              <div>
                <div style={{ fontSize: 12, color: THEME.textSecondary, marginBottom: 4 }}>今日面试用量</div>
                <div style={{ fontSize: 14, fontWeight: 600, color: THEME.textMain }}>
                  {infoQuery.data.interview_used_today} / {infoQuery.data.daily_interview_limit >= 9999 ? '不限' : infoQuery.data.daily_interview_limit}
                </div>
              </div>
            </div>
          ) : (
            <p style={{ color: THEME.textSecondary, fontSize: 14 }}>暂无会员信息</p>
          )}
        </div>

        {/* 套餐列表 */}
        <div style={cardStyle}>
          <h2 style={{ fontSize: 16, fontWeight: 700, color: THEME.textMain, margin: '0 0 16px' }}>
            {paid ? '续费 / 切换套餐' : '开通会员解锁实时语音面试等专属功能'}
          </h2>
          {plansQuery.isLoading ? (
            <Spin />
          ) : (
            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(220px, 1fr))', gap: 16 }}>
              {(plansQuery.data || []).map((plan: MembershipPlan) => (
                <div
                  key={plan.plan_type}
                  style={{
                    border: `1px solid ${THEME.border}`,
                    borderRadius: THEME.radiusSm,
                    padding: 20,
                    display: 'flex',
                    flexDirection: 'column',
                    gap: 12,
                  }}
                >
                  <div style={{ fontSize: 16, fontWeight: 700, color: THEME.textMain }}>{plan.name}</div>
                  <div style={{ fontSize: 28, fontWeight: 800, color: THEME.primary }}>
                    ¥{plan.price}
                    <span style={{ fontSize: 13, fontWeight: 500, color: THEME.textMuted }}> / {plan.duration_days}天</span>
                  </div>
                  <div style={{ display: 'flex', flexDirection: 'column', gap: 6, flex: 1 }}>
                    {plan.features.map((f) => (
                      <div key={f} style={{ fontSize: 13, color: THEME.textSecondary, display: 'flex', alignItems: 'center', gap: 6 }}>
                        <CheckCircleOutlined style={{ color: THEME.success, fontSize: 12 }} />
                        {f}
                      </div>
                    ))}
                  </div>
                  <Button
                    type="primary"
                    loading={busyPlan === plan.plan_type}
                    onClick={() => handleUpgrade(plan.plan_type)}
                    style={{ background: THEME.primary, borderColor: THEME.primary, borderRadius: 8, fontWeight: 600 }}
                  >
                    {currentLevel === plan.plan_type ? '续费' : '开通'}
                  </Button>
                </div>
              ))}
            </div>
          )}
          <p style={{ fontSize: 12, color: THEME.textMuted, margin: '16px 0 0' }}>
            当前为预支付阶段，点击开通将模拟完成支付并立即生效。真实支付接入后将替换为支付方跳转。
          </p>
        </div>

        {/* 订单记录 */}
        <div style={cardStyle}>
          <h2 style={{ fontSize: 16, fontWeight: 700, color: THEME.textMain, margin: '0 0 16px' }}>订单记录</h2>
          {ordersQuery.isLoading ? (
            <Spin />
          ) : (ordersQuery.data?.orders || []).length === 0 ? (
            <p style={{ color: THEME.textSecondary, fontSize: 14 }}>暂无订单记录</p>
          ) : (
            <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
              {(ordersQuery.data?.orders || []).map((order) => (
                <div
                  key={order.id}
                  style={{
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'space-between',
                    padding: '12px 16px',
                    borderRadius: THEME.radiusSm,
                    border: `1px solid ${THEME.border}`,
                  }}
                >
                  <div>
                    <div style={{ fontSize: 14, fontWeight: 600, color: THEME.textMain }}>
                      {PLAN_LABEL[order.plan_type] || order.plan_type} · ¥{order.amount}
                    </div>
                    <div style={{ fontSize: 12, color: THEME.textMuted }}>{order.order_no}</div>
                  </div>
                  <Tag color={order.status === 'paid' ? 'success' : 'default'}>{order.status}</Tag>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
