import { useMemo } from 'react'
import { Link } from '@tanstack/react-router'
import { useQuery } from '@tanstack/react-query'
import { extractErrorMessage } from '@makejob/api-client'
import {
  Alert,
  Badge,
  Button,
  Card,
  Col,
  ConfigProvider,
  Empty,
  Row,
  Space,
  Spin,
  Tag,
  Typography,
} from 'antd'
import {
  DashboardOutlined,
  UserOutlined,
  BookOutlined,
  VideoCameraOutlined,
  FireOutlined,
  UserAddOutlined,
  CrownOutlined,
  RobotOutlined,
  WarningOutlined,
  ToolOutlined,
  RightOutlined,
  ClockCircleOutlined,
  FileTextOutlined,
  LinkOutlined,
  ApiOutlined,
} from '@ant-design/icons'
import { useAdminAuthStore } from '../../state/auth'
import { fetchAICallLogs, fetchDashboardStats, fetchRuntimeAIConfig, fetchScraperTasks } from './runtimeApi'

const { Title, Text, Paragraph } = Typography

function formatRuntimeDateTime(value?: string): string {
  if (!value) return '--'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  })
}

function buildAICallStatusLabel(isSuccess: boolean): string {
  return isSuccess ? '成功' : '失败'
}

function buildScraperTaskTypeLabel(taskType?: string): string {
  switch ((taskType || '').trim()) {
    case 'import_questions':
      return '异步导入'
    case 'question_pipeline_build':
      return '流水线生成'
    case 'fetch_snapshot':
      return '抓取快照'
    default:
      return taskType?.trim() || '-'
  }
}

function buildScraperTaskStatusColor(status?: string): string {
  switch ((status || '').trim().toLowerCase()) {
    case 'success':
    case 'completed':
      return '#22c55e'
    case 'running':
    case 'pending':
      return '#3b82f6'
    case 'failed':
    case 'error':
      return '#ef4444'
    default:
      return '#94a3b8'
  }
}

/* ------------------------------------------------------------------ */
/*  视觉 token（与 AI 配置页完全一致）                                  */
/* ------------------------------------------------------------------ */

const THEME = {
  token: {
    borderRadius: 14,
    borderRadiusLG: 20,
    colorPrimary: '#2563eb',
    colorBgContainer: '#ffffff',
    colorBorder: '#e2e8f0',
    colorBorderSecondary: '#f1f5f9',
    colorText: '#0f172a',
    colorTextSecondary: '#64748b',
    colorTextTertiary: '#94a3b8',
    fontFamily: 'Inter, "SF Pro Display", -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif',
    controlHeight: 44,
    controlHeightLG: 52,
    controlHeightSM: 36,
    boxShadow: '0 0 0 1px rgba(0,0,0,0.03), 0 2px 8px rgba(0,0,0,0.04), 0 12px 24px rgba(0,0,0,0.03)',
    boxShadowSecondary: '0 0 0 1px rgba(0,0,0,0.04), 0 8px 16px rgba(0,0,0,0.06), 0 24px 48px rgba(0,0,0,0.04)',
  },
}

const glassCard = {
  background: 'rgba(255, 255, 255, 0.85)',
  backdropFilter: 'blur(20px) saturate(180%)',
  WebkitBackdropFilter: 'blur(20px) saturate(180%)',
  borderRadius: 20,
  border: '1px solid rgba(255, 255, 255, 0.6)',
  boxShadow: '0 0 0 1px rgba(0,0,0,0.03), 0 2px 8px rgba(0,0,0,0.04), 0 12px 24px rgba(0,0,0,0.03)',
  transition: 'all 0.35s cubic-bezier(0.4, 0, 0.2, 1)',
} as React.CSSProperties

const solidCard = {
  background: '#ffffff',
  borderRadius: 20,
  border: '1px solid #f1f5f9',
  boxShadow: '0 0 0 1px rgba(0,0,0,0.02), 0 4px 12px rgba(0,0,0,0.03)',
  transition: 'all 0.35s cubic-bezier(0.4, 0, 0.2, 1)',
} as React.CSSProperties

/* ------------------------------------------------------------------ */
/*  KPI 数据定义                                                       */
/* ------------------------------------------------------------------ */

interface KPIDef {
  label: string
  key: string
  icon: React.ReactNode
  gradient: string
  getValue: (stats: any) => string | number
}

function useKPIS(stats: any): KPIDef[] {
  return [
    {
      label: '总用户数',
      key: 'total_users',
      icon: <UserOutlined />,
      gradient: 'linear-gradient(135deg, #6366f1 0%, #4f46e5 100%)',
      getValue: (s) => s?.total_users ?? 0,
    },
    {
      label: '题库数量',
      key: 'total_questions',
      icon: <BookOutlined />,
      gradient: 'linear-gradient(135deg, #3b82f6 0%, #2563eb 100%)',
      getValue: (s) => s?.total_questions ?? 0,
    },
    {
      label: '面试次数',
      key: 'total_interviews',
      icon: <VideoCameraOutlined />,
      gradient: 'linear-gradient(135deg, #0ea5e9 0%, #0284c7 100%)',
      getValue: (s) => s?.total_interviews ?? 0,
    },
    {
      label: '今日活跃',
      key: 'today_active_users',
      icon: <FireOutlined />,
      gradient: 'linear-gradient(135deg, #f59e0b 0%, #d97706 100%)',
      getValue: (s) => s?.today_active_users ?? 0,
    },
    {
      label: '新增用户',
      key: 'new_users_today',
      icon: <UserAddOutlined />,
      gradient: 'linear-gradient(135deg, #10b981 0%, #059669 100%)',
      getValue: (s) => s?.new_users_today ?? 0,
    },
    {
      label: 'Pro 会员',
      key: 'pro_members',
      icon: <CrownOutlined />,
      gradient: 'linear-gradient(135deg, #8b5cf6 0%, #7c3aed 100%)',
      getValue: (s) => s?.pro_members ?? 0,
    },
  ]
}

/* ------------------------------------------------------------------ */
/*  主组件                                                             */
/* ------------------------------------------------------------------ */

export function RuntimeOverviewPage() {
  const token = useAdminAuthStore((state) => state.accessToken)
  const user = useAdminAuthStore((state) => state.user)

  const dashboardQuery = useQuery({
    queryKey: ['admin-dashboard-stats'],
    queryFn: () => fetchDashboardStats(token),
    enabled: Boolean(token),
  })

  const aiConfigQuery = useQuery({
    queryKey: ['admin-runtime-ai-config'],
    queryFn: () => fetchRuntimeAIConfig(token),
    enabled: Boolean(token),
  })

  const aiLogsQuery = useQuery({
    queryKey: ['admin-runtime-ai-logs-overview'],
    queryFn: () => fetchAICallLogs(token, { page: 1, pageSize: 5, status: 'failed' }),
    enabled: Boolean(token),
  })

  const scraperTasksQuery = useQuery({
    queryKey: ['admin-runtime-scraper-tasks-overview'],
    queryFn: () => fetchScraperTasks(token, { page: 1, pageSize: 5 }),
    enabled: Boolean(token),
  })

  const stats = dashboardQuery.data
  const runtimeConfig = aiConfigQuery.data
  const failedLogs = aiLogsQuery.data?.list || []
  const recentTasks = scraperTasksQuery.data?.list || []

  const kpis = useKPIS(stats)

  const isLoading =
    dashboardQuery.isLoading || aiConfigQuery.isLoading || aiLogsQuery.isLoading || scraperTasksQuery.isLoading

  const overviewError = useMemo(() => {
    const error = dashboardQuery.error || aiConfigQuery.error || aiLogsQuery.error || scraperTasksQuery.error
    return error ? extractErrorMessage(error, '运行态总览加载失败') : ''
  }, [aiConfigQuery.error, aiLogsQuery.error, dashboardQuery.error, scraperTasksQuery.error])

  if (isLoading) {
    return (
      <ConfigProvider theme={THEME}>
        <div
          style={{
            minHeight: '100vh',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            background: '#f0f2f5',
          }}
        >
          <Spin size="large" tip="正在加载运行态总览..." />
        </div>
      </ConfigProvider>
    )
  }

  if (overviewError) {
    return (
      <ConfigProvider theme={THEME}>
        <div style={{ minHeight: '100vh', background: '#f0f2f5', padding: '40px 24px' }}>
          <div style={{ maxWidth: 800, margin: '0 auto' }}>
            <Alert
              message="运行态总览加载失败"
              description={overviewError}
              type="error"
              showIcon
              style={{ borderRadius: 20, padding: 24 }}
            />
          </div>
        </div>
      </ConfigProvider>
    )
  }

  return (
    <ConfigProvider theme={THEME}>
      <div
        style={{
          minHeight: '100vh',
          background: '#f0f2f5',
          padding: '32px 24px 64px',
          fontFamily: THEME.token.fontFamily as string,
        }}
      >
        <div style={{ maxWidth: 1200, margin: '0 auto' }}>
          {/* ===== 毛玻璃标题栏 ===== */}
          <div
            style={{
              ...glassCard,
              padding: '28px 32px',
              marginBottom: 28,
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'space-between',
              flexWrap: 'wrap',
              gap: 16,
            }}
          >
            <Space direction="vertical" size={8} style={{ flex: 1, minWidth: 280 }}>
              <Space align="center" size={12}>
                <div
                  style={{
                    width: 40,
                    height: 40,
                    borderRadius: 14,
                    background: 'linear-gradient(135deg, #3b82f6, #2563eb)',
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'center',
                    color: '#fff',
                    fontSize: 20,
                    boxShadow: '0 4px 12px rgba(37, 99, 235, 0.3)',
                  }}
                >
                  <DashboardOutlined />
                </div>
                <Title level={4} style={{ margin: 0, fontWeight: 700, letterSpacing: '-0.02em' }}>
                  运行总览
                </Title>
              </Space>
              <Paragraph type="secondary" style={{ margin: 0, maxWidth: 640, fontSize: 14, lineHeight: 1.6 }}>
                统一查看系统运行状态，聚合 AI 配置、最近失败调用和任务快照，减少多页面切换排查。
              </Paragraph>
            </Space>

            <Space size={12}>
              <Link to="/runtime">
                <Button
                  size="large"
                  icon={<ToolOutlined />}
                  style={{
                    borderRadius: 14,
                    height: 48,
                    padding: '0 24px',
                    fontWeight: 500,
                    border: '1px solid #e2e8f0',
                    background: 'rgba(255,255,255,0.7)',
                  }}
                >
                  运行任务页
                </Button>
              </Link>
              <Link to="/question-pipeline">
                <Button
                  type="primary"
                  size="large"
                  icon={<RightOutlined />}
                  style={{
                    borderRadius: 14,
                    height: 48,
                    padding: '0 24px',
                    fontWeight: 600,
                    background: 'linear-gradient(135deg, #3b82f6, #2563eb)',
                    border: 'none',
                    boxShadow: '0 4px 14px rgba(37, 99, 235, 0.35)',
                  }}
                >
                  题目流水线
                </Button>
              </Link>
            </Space>
          </div>

          {/* ===== KPI 统计区 ===== */}
          <Row gutter={[20, 20]} style={{ marginBottom: 28 }}>
            {kpis.map((kpi) => (
              <Col xs={12} md={8} lg={8} key={kpi.key}>
                <div
                  style={{
                    ...solidCard,
                    padding: '24px 20px',
                    cursor: 'default',
                  }}
                  onMouseEnter={(e) => {
                    e.currentTarget.style.transform = 'translateY(-4px) scale(1.01)'
                    e.currentTarget.style.boxShadow =
                      '0 8px 24px rgba(0,0,0,0.08), 0 0 0 1px rgba(0,0,0,0.03)'
                  }}
                  onMouseLeave={(e) => {
                    e.currentTarget.style.transform = 'none'
                    e.currentTarget.style.boxShadow = solidCard.boxShadow as string
                  }}
                >
                  <div style={{ display: 'flex', alignItems: 'center', gap: 14, marginBottom: 14 }}>
                    <div
                      style={{
                        width: 44,
                        height: 44,
                        borderRadius: 14,
                        background: kpi.gradient,
                        display: 'flex',
                        alignItems: 'center',
                        justifyContent: 'center',
                        color: '#fff',
                        fontSize: 20,
                        boxShadow: '0 4px 10px rgba(0,0,0,0.12)',
                        flexShrink: 0,
                      }}
                    >
                      {kpi.icon}
                    </div>
                    <Text type="secondary" style={{ fontSize: 13, fontWeight: 500 }}>
                      {kpi.label}
                    </Text>
                  </div>
                  <div
                    style={{
                      fontSize: 28,
                      fontWeight: 800,
                      color: '#0f172a',
                      letterSpacing: '-0.03em',
                      lineHeight: 1,
                    }}
                  >
                    {kpi.getValue(stats)}
                  </div>
                </div>
              </Col>
            ))}
          </Row>

          {/* ===== 主内容区：两栏布局 ===== */}
          <Row gutter={[24, 24]}>
            {/* 左侧：AI 配置 + 排查入口 */}
            <Col xs={24} lg={10}>
              <Space direction="vertical" size={24} style={{ width: '100%' }}>
                {/* AI 生效配置 */}
                <div style={{ ...solidCard, padding: '28px 32px' }}>
                  <div
                    style={{
                      display: 'flex',
                      alignItems: 'flex-start',
                      justifyContent: 'space-between',
                      marginBottom: 20,
                      flexWrap: 'wrap',
                      gap: 12,
                    }}
                  >
                    <Space direction="vertical" size={4}>
                      <Title level={5} style={{ margin: 0, fontWeight: 700 }}>
                        <RobotOutlined style={{ marginRight: 8, color: '#3b82f6' }} />
                        当前 AI 生效配置
                      </Title>
                      <Text type="secondary" style={{ fontSize: 13 }}>
                        管理员：{user?.username || '-'}
                      </Text>
                    </Space>
                    <Link to="/ai-configs">
                      <Button
                        type="primary"
                        size="small"
                        icon={<RightOutlined />}
                        style={{
                          borderRadius: 10,
                          fontWeight: 500,
                          background: 'linear-gradient(135deg, #3b82f6, #2563eb)',
                          border: 'none',
                        }}
                      >
                        去配置页
                      </Button>
                    </Link>
                  </div>

                  <Space direction="vertical" size={12} style={{ width: '100%' }}>
                    {[
                      { label: '主 Provider', value: runtimeConfig?.configs?.ai_provider || '未设置' },
                      { label: '默认模型', value: runtimeConfig?.configs?.ai_model || '未设置' },
                      { label: 'Fallback Provider', value: runtimeConfig?.configs?.ai_fallback_provider || '未启用' },
                      {
                        label: '当前支持',
                        value: runtimeConfig?.support?.primary_providers?.join(' / ') || '无',
                      },
                    ].map((item) => (
                      <div
                        key={item.label}
                        style={{
                          display: 'flex',
                          justifyContent: 'space-between',
                          alignItems: 'center',
                          padding: '12px 16px',
                          borderRadius: 14,
                          background: '#f8fafc',
                        }}
                      >
                        <Text type="secondary" style={{ fontSize: 13 }}>{item.label}</Text>
                        <Text strong style={{ fontSize: 14, color: '#0f172a' }}>
                          {item.value}
                        </Text>
                      </div>
                    ))}
                  </Space>

                  {/* 运行告警 */}
                  {(runtimeConfig?.warnings || []).length > 0 && (
                    <div
                      style={{
                        marginTop: 16,
                        padding: '14px 18px',
                        borderRadius: 14,
                        background: '#fffbeb',
                        border: '1px solid #fef3c7',
                      }}
                    >
                      <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 8 }}>
                        <WarningOutlined style={{ color: '#f59e0b' }} />
                        <Text strong style={{ color: '#92400e', fontSize: 13 }}>运行告警</Text>
                      </div>
                      <ul style={{ margin: 0, paddingLeft: 18 }}>
                        {runtimeConfig?.warnings.map((warning) => (
                          <li key={warning} style={{ color: '#a16207', fontSize: 13, lineHeight: 1.8 }}>
                            {warning}
                          </li>
                        ))}
                      </ul>
                    </div>
                  )}

                  {/* 支持说明 */}
                  {(runtimeConfig?.support?.notes || []).length > 0 && (
                    <div
                      style={{
                        marginTop: 12,
                        padding: '14px 18px',
                        borderRadius: 14,
                        background: '#f0f9ff',
                        border: '1px solid #e0f2fe',
                      }}
                    >
                      <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 8 }}>
                        <FileTextOutlined style={{ color: '#0ea5e9' }} />
                        <Text strong style={{ color: '#0369a1', fontSize: 13 }}>支持说明</Text>
                      </div>
                      <ul style={{ margin: 0, paddingLeft: 18 }}>
                        {runtimeConfig?.support?.notes.map((note) => (
                          <li key={note} style={{ color: '#0c4a6e', fontSize: 13, lineHeight: 1.8 }}>
                            {note}
                          </li>
                        ))}
                      </ul>
                    </div>
                  )}
                </div>

                {/* 排查入口 */}
                <div style={{ ...solidCard, padding: '28px 32px' }}>
                  <Title level={5} style={{ margin: '0 0 20px', fontWeight: 700 }}>
                    <ApiOutlined style={{ marginRight: 8, color: '#8b5cf6' }} />
                    当前排查入口
                  </Title>
                  <Paragraph type="secondary" style={{ marginBottom: 20, fontSize: 13 }}>
                    把原本分散的调试入口集中到统一路径。
                  </Paragraph>

                  <Space direction="vertical" size={10} style={{ width: '100%' }}>
                    {[
                      { to: '/question-pipeline', label: '题目流水线调试', color: '#0ea5e9', bg: '#f0f9ff' },
                      { to: '/ai-configs', label: 'AI 配置校验', color: '#3b82f6', bg: '#eff6ff' },
                      { to: '/runtime', label: '运行日志与任务', color: '#6366f1', bg: '#eef2ff' },
                    ].map((link) => (
                      <Link key={link.to} to={link.to} style={{ textDecoration: 'none', display: 'block' }}>
                        <div
                          style={{
                            display: 'flex',
                            alignItems: 'center',
                            justifyContent: 'space-between',
                            padding: '14px 18px',
                            borderRadius: 14,
                            background: link.bg,
                            border: `1px solid ${link.bg}`,
                            transition: 'all 0.25s ease',
                            cursor: 'pointer',
                          }}
                          onMouseEnter={(e) => {
                            e.currentTarget.style.borderColor = link.color + '40'
                            e.currentTarget.style.transform = 'translateX(4px)'
                          }}
                          onMouseLeave={(e) => {
                            e.currentTarget.style.borderColor = link.bg
                            e.currentTarget.style.transform = 'none'
                          }}
                        >
                          <Text strong style={{ color: link.color, fontSize: 14 }}>
                            {link.label}
                          </Text>
                          <RightOutlined style={{ color: link.color, fontSize: 12 }} />
                        </div>
                      </Link>
                    ))}
                  </Space>

                  <div
                    style={{
                      marginTop: 20,
                      padding: '14px 18px',
                      borderRadius: 14,
                      background: '#f8fafc',
                    }}
                  >
                    <Space direction="vertical" size={4} style={{ width: '100%' }}>
                      <div style={{ display: 'flex', justifyContent: 'space-between' }}>
                        <Text type="secondary" style={{ fontSize: 12 }}>最近 AI 失败数</Text>
                        <Text strong style={{ fontSize: 13, color: failedLogs.filter((item) => !item.is_success).length > 0 ? '#ef4444' : '#0f172a' }}>
                          {failedLogs.filter((item) => !item.is_success).length}
                        </Text>
                      </div>
                      <div style={{ display: 'flex', justifyContent: 'space-between' }}>
                        <Text type="secondary" style={{ fontSize: 12 }}>最近异步任务数</Text>
                        <Text strong style={{ fontSize: 13 }}>{recentTasks.length}</Text>
                      </div>
                      <div style={{ display: 'flex', justifyContent: 'space-between' }}>
                        <Text type="secondary" style={{ fontSize: 12 }}>当前登录管理员</Text>
                        <Text strong style={{ fontSize: 13 }}>{user?.email || '-'}</Text>
                      </div>
                    </Space>
                  </div>
                </div>
              </Space>
            </Col>

            {/* 右侧：失败日志 + 抓取任务 */}
            <Col xs={24} lg={14}>
              <Space direction="vertical" size={24} style={{ width: '100%' }}>
                {/* 最近 AI 失败调用 */}
                <div style={{ ...solidCard, padding: '28px 32px' }}>
                  <div
                    style={{
                      display: 'flex',
                      alignItems: 'flex-start',
                      justifyContent: 'space-between',
                      marginBottom: 20,
                      flexWrap: 'wrap',
                      gap: 12,
                    }}
                  >
                    <Space direction="vertical" size={4}>
                      <Title level={5} style={{ margin: 0, fontWeight: 700 }}>
                        <WarningOutlined style={{ marginRight: 8, color: '#ef4444' }} />
                        最近 AI 失败调用
                      </Title>
                      <Paragraph type="secondary" style={{ margin: 0, fontSize: 13 }}>
                        优先显示失败日志，便于直接定位 trace。
                      </Paragraph>
                    </Space>
                    <Link to="/runtime">
                      <Button
                        size="small"
                        icon={<RightOutlined />}
                        style={{ borderRadius: 10, fontWeight: 500 }}
                      >
                        查看全部
                      </Button>
                    </Link>
                  </div>

                  {failedLogs.length === 0 ? (
                    <Empty
                      image={Empty.PRESENTED_IMAGE_SIMPLE}
                      description="最近没有失败的 AI 调用记录"
                      style={{ padding: '32px 0' }}
                    />
                  ) : (
                    <Space direction="vertical" size={10} style={{ width: '100%' }}>
                      <div
                        style={{
                          display: 'grid',
                          gridTemplateColumns: '100px 100px 100px 1fr',
                          gap: 12,
                          padding: '10px 16px',
                          borderRadius: 12,
                          background: '#f1f5f9',
                          fontSize: 12,
                          fontWeight: 600,
                          color: '#475569',
                        }}
                      >
                        <span>时间</span>
                        <span>场景</span>
                        <span>Provider</span>
                        <span>Trace</span>
                      </div>
                      {failedLogs.map((item) => (
                        <div
                          key={item.id}
                          style={{
                            display: 'grid',
                            gridTemplateColumns: '100px 100px 100px 1fr',
                            gap: 12,
                            padding: '14px 16px',
                            borderRadius: 14,
                            background: '#fafafa',
                            transition: 'all 0.2s ease',
                            cursor: 'default',
                          }}
                          onMouseEnter={(e) => {
                            e.currentTarget.style.background = '#f1f5f9'
                          }}
                          onMouseLeave={(e) => {
                            e.currentTarget.style.background = '#fafafa'
                          }}
                        >
                          <Text style={{ fontSize: 13, color: '#64748b' }}>
                            <ClockCircleOutlined style={{ marginRight: 4, fontSize: 11 }} />
                            {formatRuntimeDateTime(item.created_at)}
                          </Text>
                          <Text style={{ fontSize: 13, color: '#0f172a' }}>
                            {item.scene || item.source || '-'}
                          </Text>
                          <Tag
                            size="small"
                            style={{
                              borderRadius: 8,
                              fontSize: 12,
                              margin: 0,
                              width: 'fit-content',
                              border: 'none',
                              background: '#fee2e2',
                              color: '#991b1b',
                            }}
                          >
                            {item.provider || item.model || '-'}
                          </Tag>
                          <Text
                            style={{
                              fontSize: 12,
                              fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace',
                              color: '#94a3b8',
                              overflowWrap: 'anywhere',
                            }}
                          >
                            {item.trace_id || '-'}
                          </Text>
                        </div>
                      ))}
                    </Space>
                  )}
                </div>

                {/* 最近抓取任务 */}
                <div style={{ ...solidCard, padding: '28px 32px' }}>
                  <div
                    style={{
                      display: 'flex',
                      alignItems: 'flex-start',
                      justifyContent: 'space-between',
                      marginBottom: 20,
                      flexWrap: 'wrap',
                      gap: 12,
                    }}
                  >
                    <Space direction="vertical" size={4}>
                      <Title level={5} style={{ margin: 0, fontWeight: 700 }}>
                        <LinkOutlined style={{ marginRight: 8, color: '#10b981' }} />
                        最近抓取任务
                      </Title>
                      <Paragraph type="secondary" style={{ margin: 0, fontSize: 13 }}>
                        抓取、导入和流水线生成任务的最新快照。
                      </Paragraph>
                    </Space>
                    <Link to="/runtime">
                      <Button
                        size="small"
                        icon={<RightOutlined />}
                        style={{ borderRadius: 10, fontWeight: 500 }}
                      >
                        查看全部
                      </Button>
                    </Link>
                  </div>

                  {recentTasks.length === 0 ? (
                    <Empty
                      image={Empty.PRESENTED_IMAGE_SIMPLE}
                      description="当前还没有异步任务记录"
                      style={{ padding: '32px 0' }}
                    />
                  ) : (
                    <Space direction="vertical" size={10} style={{ width: '100%' }}>
                      <div
                        style={{
                          display: 'grid',
                          gridTemplateColumns: '100px 110px 90px 90px 1fr',
                          gap: 12,
                          padding: '10px 16px',
                          borderRadius: 12,
                          background: '#f1f5f9',
                          fontSize: 12,
                          fontWeight: 600,
                          color: '#475569',
                        }}
                      >
                        <span>时间</span>
                        <span>类型</span>
                        <span>来源</span>
                        <span>状态</span>
                        <span>标题</span>
                      </div>
                      {recentTasks.map((item) => {
                        const statusColor = buildScraperTaskStatusColor(item.status)
                        return (
                          <div
                            key={item.id}
                            style={{
                              display: 'grid',
                              gridTemplateColumns: '100px 110px 90px 90px 1fr',
                              gap: 12,
                              padding: '14px 16px',
                              borderRadius: 14,
                              background: '#fafafa',
                              transition: 'all 0.2s ease',
                              cursor: 'default',
                              alignItems: 'center',
                            }}
                            onMouseEnter={(e) => {
                              e.currentTarget.style.background = '#f1f5f9'
                            }}
                            onMouseLeave={(e) => {
                              e.currentTarget.style.background = '#fafafa'
                            }}
                          >
                            <Text style={{ fontSize: 13, color: '#64748b' }}>
                              <ClockCircleOutlined style={{ marginRight: 4, fontSize: 11 }} />
                              {formatRuntimeDateTime(item.created_at)}
                            </Text>
                            <Tag
                              size="small"
                              style={{
                                borderRadius: 8,
                                fontSize: 12,
                                margin: 0,
                                width: 'fit-content',
                                border: 'none',
                                background: '#e0e7ff',
                                color: '#3730a3',
                              }}
                            >
                              {buildScraperTaskTypeLabel(item.task_type)}
                            </Tag>
                            <Text style={{ fontSize: 13, color: '#0f172a' }}>
                              {item.source || '-'}
                            </Text>
                            <div
                              style={{
                                display: 'flex',
                                alignItems: 'center',
                                gap: 6,
                              }}
                            >
                              <div
                                style={{
                                  width: 8,
                                  height: 8,
                                  borderRadius: '50%',
                                  background: statusColor,
                                  flexShrink: 0,
                                }}
                              />
                              <Text style={{ fontSize: 13, color: '#0f172a' }}>
                                {item.status || '-'}
                              </Text>
                            </div>
                            <Text
                              style={{
                                fontSize: 13,
                                color: '#475569',
                                overflowWrap: 'anywhere',
                              }}
                            >
                              {item.source_title || item.source_url || '-'}
                            </Text>
                          </div>
                        )
                      })}
                    </Space>
                  )}
                </div>
              </Space>
            </Col>
          </Row>
        </div>
      </div>
    </ConfigProvider>
  )
}
