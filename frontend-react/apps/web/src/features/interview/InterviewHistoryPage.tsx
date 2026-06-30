import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Link, useNavigate } from '@tanstack/react-router'
import { Button, Tag, Empty, Spin, Pagination } from 'antd'
import {
  ArrowLeftOutlined,
  ClockCircleOutlined,
  TrophyOutlined,
  PlayCircleOutlined,
  FileTextOutlined,
} from '@ant-design/icons'
import { useAuthStore } from '../../state/auth'
import { requestLoginPrompt } from '../../shared/loginPrompt'
import { buildInterviewHistoryQueryKey } from '../../shared/queryKeys'
import { fetchInterviewHistory } from './interviewApi'
import {
  formatInterviewDateTime,
  interviewStatusLabel,
} from './interviewHelpers'

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
  shadowHover: '0 10px 15px -3px rgba(0,0,0,0.08), 0 4px 6px -4px rgba(0,0,0,0.05)',
  radius: 12,
  radiusSm: 8,
  success: '#22c55e',
  warning: '#f59e0b',
  danger: '#ef4444',
}

const PAGE_SIZE = 20

/**
 * 面试历史记录页面，展示用户所有面试记录。
 */
export function InterviewHistoryPage() {
  const navigate = useNavigate()
  const accessToken = useAuthStore((state) => state.accessToken)
  const [page, setPage] = useState(1)

  const historyQuery = useQuery({
    queryKey: [...buildInterviewHistoryQueryKey(accessToken), page, PAGE_SIZE],
    queryFn: () => fetchInterviewHistory(accessToken as string, page, PAGE_SIZE),
    enabled: Boolean(accessToken),
    retry: false,
  })

  const pagedHistory = historyQuery.data?.list || []
  const total = historyQuery.data?.total || 0

  const cardStyle = {
    background: THEME.cardBg,
    borderRadius: THEME.radius,
    border: `1px solid ${THEME.border}`,
    boxShadow: THEME.shadow,
    padding: '24px',
  }

  if (!accessToken) {
    return (
      <div style={{ minHeight: '100vh', background: THEME.bg }}>
        <div style={{ maxWidth: 800, margin: '0 auto', padding: '64px 24px', textAlign: 'center' }}>
          <Empty
            image={Empty.PRESENTED_IMAGE_SIMPLE}
            description="登录后查看面试历史记录"
          />
          <Button
            type="primary"
            style={{ marginTop: 16, background: THEME.primary, borderColor: THEME.primary }}
            onClick={() => requestLoginPrompt('/interview/history', 'missing')}
          >
            前往登录
          </Button>
        </div>
      </div>
    )
  }

  return (
    <div style={{ minHeight: '100vh', background: THEME.bg }}>
      {/* 顶部标题栏 */}
      <div style={{
        background: THEME.cardBg,
        borderBottom: `1px solid ${THEME.border}`,
        boxShadow: THEME.shadow,
      }}>
        <div style={{
          maxWidth: 1200,
          margin: '0 auto',
          padding: '20px 24px',
        }}>
          <Link
            to="/interview"
            style={{
              display: 'inline-flex',
              alignItems: 'center',
              gap: 6,
              fontSize: 14,
              color: THEME.textSecondary,
              textDecoration: 'none',
              marginBottom: 16,
            }}
          >
            <ArrowLeftOutlined />
            返回面试主页
          </Link>

          <h1 style={{ fontSize: 22, fontWeight: 700, color: THEME.textMain, margin: '0 0 8px' }}>
            面试历史记录
          </h1>
          <p style={{ fontSize: 14, color: THEME.textSecondary, margin: 0 }}>
            共 {total} 条面试记录
          </p>
        </div>
      </div>

      {/* 主内容区 */}
      <div style={{ maxWidth: 1200, margin: '0 auto', padding: '24px' }}>
        {historyQuery.isLoading ? (
          <div style={{ textAlign: 'center', padding: '48px 0' }}><Spin /></div>
        ) : pagedHistory.length === 0 ? (
          <div style={cardStyle}>
            <Empty
              image={Empty.PRESENTED_IMAGE_SIMPLE}
              description="还没有面试记录"
            />
          </div>
        ) : (
          <>
            <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
              {pagedHistory.map((item) => (
                <div
                  key={item.id}
                  style={{
                    ...cardStyle,
                    transition: 'all 0.2s ease',
                    cursor: 'pointer',
                  }}
                  onMouseEnter={(e) => {
                    e.currentTarget.style.boxShadow = THEME.shadowHover
                    e.currentTarget.style.borderColor = THEME.borderHover
                  }}
                  onMouseLeave={(e) => {
                    e.currentTarget.style.boxShadow = THEME.shadow
                    e.currentTarget.style.borderColor = THEME.border
                  }}
                  onClick={() => {
                    if (item.status === 'ongoing' || item.status === 'preparing') {
                      navigate({ to: '/interview/$interviewId', params: { interviewId: String(item.id) } })
                    } else {
                      navigate({ to: '/interview/$interviewId/report', params: { interviewId: String(item.id) } })
                    }
                  }}
                >
                  <div style={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between' }}>
                    <div style={{ flex: 1 }}>
                      <div style={{ display: 'flex', alignItems: 'center', gap: 12, marginBottom: 12 }}>
                        <span style={{ fontSize: 18, fontWeight: 700, color: THEME.textMain }}>
                          面试 #{item.id}
                        </span>
                        <Tag
                          color={item.status === 'completed' ? 'success' : item.status === 'ongoing' ? 'processing' : item.status === 'preparing' ? 'warning' : 'default'}
                        >
                          {interviewStatusLabel(item.status)}
                        </Tag>
                      </div>

                      <div style={{ display: 'flex', gap: 24, marginBottom: 12 }}>
                        <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                          <FileTextOutlined style={{ color: THEME.textMuted }} />
                          <span style={{ fontSize: 14, color: THEME.textSecondary }}>
                            {item.total_questions} 题
                          </span>
                        </div>

                        {item.score != null && (
                          <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                            <TrophyOutlined style={{ color: THEME.warning }} />
                            <span style={{ fontSize: 14, color: THEME.textSecondary }}>
                              得分 {Math.round(item.score)}
                            </span>
                          </div>
                        )}

                        <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                          <ClockCircleOutlined style={{ color: THEME.textMuted }} />
                          <span style={{ fontSize: 14, color: THEME.textSecondary }}>
                            {formatInterviewDateTime(item.started_at || item.created_at)}
                          </span>
                        </div>
                      </div>
                    </div>

                    <div>
                      {item.status === 'ongoing' || item.status === 'preparing' ? (
                        <Button
                          type="primary"
                          icon={<PlayCircleOutlined />}
                          style={{
                            background: THEME.primary,
                            borderColor: THEME.primary,
                            borderRadius: 8,
                          }}
                        >
                          {item.status === 'preparing' ? '查看准备进度' : '继续面试'}
                        </Button>
                      ) : (
                        <Button
                          icon={<FileTextOutlined />}
                          style={{ borderRadius: 8 }}
                        >
                          查看报告
                        </Button>
                      )}
                    </div>
                  </div>
                </div>
              ))}
            </div>

            {/* 分页 */}
            {total > PAGE_SIZE && (
              <div style={{ display: 'flex', justifyContent: 'center', marginTop: 32 }}>
                <Pagination
                  current={page}
                  pageSize={PAGE_SIZE}
                  total={total}
                  onChange={setPage}
                  showSizeChanger={false}
                />
              </div>
            )}
          </>
        )}
      </div>
    </div>
  )
}
