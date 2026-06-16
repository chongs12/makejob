import { useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  RocketOutlined,
  SearchOutlined,
  ReloadOutlined,
  EyeOutlined,
  EyeInvisibleOutlined,
  FileTextOutlined,
  CopyOutlined,
  CheckCircleOutlined,
  CloseCircleOutlined,
  ExclamationCircleOutlined,
  ThunderboltOutlined,
  ApiOutlined,
  DownOutlined,
  UpOutlined,
} from '@ant-design/icons'
import { Button, Card, Input, Select, Tag, Tooltip } from 'antd'
import { extractErrorMessage } from '@makejob/api-client'
import { useAdminAuthStore } from '../../state/auth'
import {
  fetchAICallLogDetail,
  fetchAICallLogs,
  fetchRuntimeAIConfig,
  fetchScraperTaskDetail,
  fetchScraperTasks,
  retryScraperTask,
} from './runtimeApi'

const LOG_PAGE_SIZE = 10
const TASK_PAGE_SIZE = 10

const THEME = {
  bg: '#f4f7fe',
  cardBg: '#ffffff',
  primary: '#4f46e5',
  primaryLight: '#e0e7ff',
  accent: '#f59e0b',
  textMain: '#1e293b',
  textSecondary: '#64748b',
  textMuted: '#94a3b8',
  border: '#e2e8f0',
  success: '#10b981',
  warning: '#f59e0b',
  danger: '#ef4444',
  shadow: '0 8px 32px rgba(31, 38, 135, 0.07)',
  radius: 16,
}

const glassCard = {
  background: 'rgba(255,255,255,0.85)',
  backdropFilter: 'blur(20px) saturate(180%)',
  WebkitBackdropFilter: 'blur(20px) saturate(180%)',
  borderRadius: THEME.radius,
  border: '1px solid rgba(255,255,255,0.6)',
  boxShadow: THEME.shadow,
}

const solidCard = {
  background: THEME.cardBg,
  borderRadius: THEME.radius,
  boxShadow: THEME.shadow,
  border: '1px solid ' + THEME.border,
}

/**
 * 规范化需要复制的文本，避免把纯空白内容写入剪贴板。
 */
function normalizeRuntimeCopyText(value?: string): string {
  return value?.trim() || ''
}

/**
 * 将文本写入剪贴板，并在不支持 Clipboard API 时回退到 textarea 复制方案。
 */
async function copyRuntimeTextToClipboard(value: string): Promise<void> {
  const normalizedValue = normalizeRuntimeCopyText(value)
  if (!normalizedValue) {
    throw new Error('当前没有可复制的内容')
  }

  if (typeof navigator !== 'undefined' && navigator.clipboard?.writeText) {
    await navigator.clipboard.writeText(normalizedValue)
    return
  }

  if (typeof document === 'undefined') {
    throw new Error('当前环境不支持复制到剪贴板')
  }

  const textarea = document.createElement('textarea')
  textarea.value = normalizedValue
  textarea.setAttribute('readonly', 'true')
  textarea.style.position = 'fixed'
  textarea.style.opacity = '0'
  document.body.appendChild(textarea)
  textarea.select()
  textarea.setSelectionRange(0, normalizedValue.length)

  try {
    const copied = document.execCommand('copy')
    if (!copied) {
      throw new Error('浏览器拒绝了复制操作')
    }
  } finally {
    document.body.removeChild(textarea)
  }
}

/**
 * 将普通文本格式化为详情区的稳定展示内容，空值统一回退为占位符。
 */
function formatRuntimeDetailText(value?: string): string {
  if (!value?.trim()) {
    return '--'
  }
  return value.trim()
}

/**
 * 将后台运行时间统一格式化成简洁时间，方便表格内快速扫描。
 */
function formatRuntimeListDateTime(value?: string): string {
  if (!value) {
    return '--'
  }

  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return value
  }

  return date.toLocaleString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  })
}

/**
 * 统一渲染 AI 调用状态标签，便于日志表格快速识别成功与失败。
 */
function renderAICallStatusTag(isSuccess: boolean): React.ReactNode {
  return isSuccess ? (
    <Tag color="success" style={{ fontSize: 11, margin: 0 }}>success</Tag>
  ) : (
    <Tag color="error" style={{ fontSize: 11, margin: 0 }}>failed</Tag>
  )
}

/**
 * 统一渲染抓取任务类型文本，便于区分同步抓取留痕与异步导入任务。
 */
function renderScraperTaskTypeText(taskType?: string): string {
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

const TASK_STATUS_CONFIG: Record<string, { label: string; color: string }> = {
  pending: { label: '待执行', color: THEME.warning },
  running: { label: '执行中', color: THEME.primary },
  fetched: { label: '已抓取', color: '#3b82f6' },
  cleaned: { label: '已清洗', color: '#8b5cf6' },
  imported: { label: '已导入', color: THEME.success },
  succeeded: { label: '已完成', color: THEME.success },
  failed: { label: '失败', color: THEME.danger },
}

/**
 * 统一渲染抓取任务状态文本，减少后台列表直接暴露内部状态码的阅读成本。
 */
function renderScraperTaskStatusTag(status?: string): React.ReactNode {
  const cfg = TASK_STATUS_CONFIG[status?.trim() || '']
  if (!cfg) return <Tag style={{ fontSize: 11, margin: 0 }}>{status?.trim() || '-'}</Tag>
  return <Tag color={cfg.color} style={{ fontSize: 11, margin: 0 }}>{cfg.label}</Tag>
}

/**
 * 生成抓取任务列表中的结果摘要，优先暴露错误信息，其次展示题目数、导入数与重试次数。
 */
function buildScraperTaskSummary(item: {
  question_count: number
  imported_count: number
  retry_count?: number
  error_msg?: string
}): string {
  if (item.error_msg?.trim()) {
    return item.error_msg.trim()
  }
  return `题目 ${item.question_count} / 导入 ${item.imported_count} / 重试 ${item.retry_count || 0}`
}

/**
 * 汇总单条 AI 调用日志的关键观察点，优先暴露错误，其次显示耗时与来源模型。
 */
function buildAICallInlineSummary(item: {
  latency_ms: number
  model_error: string
  provider: string
  model: string
}): string {
  if (item.model_error?.trim()) {
    return item.model_error.trim()
  }
  return `耗时 ${item.latency_ms || 0} ms / ${item.provider || '-'} / ${item.model || '-'}`
}

/**
 * 将任务详情中的 JSON 字符串格式化为可读文本，便于直接在后台查看入队载荷和执行结果。
 */
function formatTaskJSONText(value?: string): string {
  if (!value?.trim()) {
    return '--'
  }

  try {
    return JSON.stringify(JSON.parse(value), null, 2)
  } catch {
    return value
  }
}

/**
 * 提供后台统一运行任务页，集中查看 AI 调用日志与抓取任务历史。
 */
export function RuntimeTasksPage() {
  const token = useAdminAuthStore((state) => state.accessToken)
  const queryClient = useQueryClient()
  const [traceId, setTraceId] = useState('')
  const [taskId, setTaskId] = useState('')
  const [status, setStatus] = useState('')
  const [submittedTraceId, setSubmittedTraceId] = useState('')
  const [submittedTaskId, setSubmittedTaskId] = useState('')
  const [submittedStatus, setSubmittedStatus] = useState('')
  const [logPage, setLogPage] = useState(1)
  const [taskStatus, setTaskStatus] = useState('')
  const [taskType, setTaskType] = useState('')
  const [submittedTaskStatus, setSubmittedTaskStatus] = useState('')
  const [submittedTaskType, setSubmittedTaskType] = useState('')
  const [taskPage, setTaskPage] = useState(1)
  const [selectedTaskId, setSelectedTaskId] = useState<number | null>(null)
  const [expandedRelatedLogId, setExpandedRelatedLogId] = useState<number | null>(null)
  const [relatedLogCopyMessage, setRelatedLogCopyMessage] = useState('')
  const [taskActionMessage, setTaskActionMessage] = useState('')

  const aiConfigQuery = useQuery({
    queryKey: ['admin-runtime-ai-config-inline'],
    queryFn: () => fetchRuntimeAIConfig(token),
    enabled: Boolean(token),
  })

  const aiLogsQuery = useQuery({
    queryKey: ['admin-runtime-ai-logs', logPage, submittedTraceId, submittedTaskId, submittedStatus],
    queryFn: () =>
      fetchAICallLogs(token, {
        page: logPage,
        pageSize: LOG_PAGE_SIZE,
        traceId: submittedTraceId,
        taskId: submittedTaskId,
        status: submittedStatus,
      }),
    enabled: Boolean(token),
  })

  const scraperTasksQuery = useQuery({
    queryKey: ['admin-runtime-scraper-tasks', taskPage, submittedTaskStatus, submittedTaskType],
    queryFn: () =>
      fetchScraperTasks(token, {
        page: taskPage,
        pageSize: TASK_PAGE_SIZE,
        status: submittedTaskStatus,
        taskType: submittedTaskType,
      }),
    enabled: Boolean(token),
  })

  const scraperTaskDetailQuery = useQuery({
    queryKey: ['admin-runtime-scraper-task-detail', selectedTaskId],
    queryFn: () => fetchScraperTaskDetail(token, selectedTaskId as number),
    enabled: Boolean(token && selectedTaskId),
  })

  const relatedAICallLogsQuery = useQuery({
    queryKey: ['admin-runtime-related-ai-logs', selectedTaskId],
    queryFn: () =>
      fetchAICallLogs(token, {
        page: 1,
        pageSize: 6,
        taskId: String(selectedTaskId),
      }),
    enabled: Boolean(token && selectedTaskId),
  })

  const relatedAICallLogDetailQuery = useQuery({
    queryKey: ['admin-runtime-related-ai-log-detail', expandedRelatedLogId],
    queryFn: () => fetchAICallLogDetail(token, expandedRelatedLogId as number),
    enabled: Boolean(token && expandedRelatedLogId),
  })

  const retryTaskMutation = useMutation({
    mutationFn: (taskId: number) => retryScraperTask(token, taskId),
    onSuccess: async (task) => {
      setTaskActionMessage(`任务 #${task.id} 已重新投递，等待 worker 继续消费。`)
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['admin-runtime-scraper-tasks'] }),
        queryClient.invalidateQueries({ queryKey: ['admin-runtime-scraper-tasks-overview'] }),
        queryClient.invalidateQueries({ queryKey: ['admin-runtime-scraper-task-detail', task.id] }),
      ])
    },
    onError: (error) => {
      setTaskActionMessage(extractErrorMessage(error, '重试任务失败'))
    },
  })

  const pageError = useMemo(() => {
    const error = aiConfigQuery.error || aiLogsQuery.error || scraperTasksQuery.error
    return error ? extractErrorMessage(error, '运行任务页加载失败') : ''
  }, [aiConfigQuery.error, aiLogsQuery.error, scraperTasksQuery.error])

  /**
   * 提交日志筛选条件，并将分页重置到第一页。
   */
  function handleApplyLogFilters(): void {
    setSubmittedTraceId(traceId.trim())
    setSubmittedTaskId(taskId.trim())
    setSubmittedStatus(status.trim())
    setLogPage(1)
  }

  /**
   * 提交抓取任务筛选条件，并收起旧详情以避免新旧筛选结果混淆。
   */
  function handleApplyTaskFilters(): void {
    setSubmittedTaskStatus(taskStatus.trim())
    setSubmittedTaskType(taskType.trim())
    setTaskPage(1)
    setSelectedTaskId(null)
  }

  /**
   * 切换任务详情查看状态，便于在列表和详情之间快速往返排查。
   */
  function handleToggleTaskDetail(taskId: number): void {
    setRelatedLogCopyMessage('')
    setExpandedRelatedLogId(null)
    setSelectedTaskId((current) => (current === taskId ? null : taskId))
  }

  /**
   * 按异步任务 ID 反查关联 AI 调用日志，便于从任务详情直接跳回模型调用记录。
   */
  function handleViewTaskLogs(targetTaskId: number): void {
    const normalizedTaskId = String(targetTaskId)
    setTaskId(normalizedTaskId)
    setSubmittedTaskId(normalizedTaskId)
    setStatus('')
    setSubmittedStatus('')
    setTraceId('')
    setSubmittedTraceId('')
    setLogPage(1)
  }

  /**
   * 切换关联 AI 日志详情展开状态，避免一次性拉取所有原始调试信息。
   */
  function handleToggleRelatedLogDetail(logId: number): void {
    setRelatedLogCopyMessage('')
    setExpandedRelatedLogId((current) => (current === logId ? null : logId))
  }

  /**
   * 复制关联 AI 日志中的关键排障文本，并给出简短反馈。
   */
  async function handleCopyRelatedLogText(value: string, label: string): Promise<void> {
    try {
      await copyRuntimeTextToClipboard(value)
      setRelatedLogCopyMessage(`${label}已复制到剪贴板。`)
    } catch (error) {
      setRelatedLogCopyMessage(extractErrorMessage(error, `${label}复制失败`))
    }
  }

  if (aiConfigQuery.isLoading || aiLogsQuery.isLoading || scraperTasksQuery.isLoading) {
    return (
      <div style={{ padding: '24px 32px 32px', background: THEME.bg, minHeight: '100vh' }}>
        <div style={{ ...glassCard, padding: '24px 28px' }}>
          <h2 style={{ margin: 0, fontSize: 22, fontWeight: 700, color: THEME.textMain }}>运行日志与任务</h2>
          <p style={{ margin: '8px 0 0', color: THEME.textSecondary }}>正在加载 AI 日志、抓取任务和当前运行配置...</p>
        </div>
      </div>
    )
  }

  if (pageError) {
    return (
      <div style={{ padding: '24px 32px 32px', background: THEME.bg, minHeight: '100vh' }}>
        <div style={{ ...glassCard, padding: '24px 28px' }}>
          <h2 style={{ margin: 0, fontSize: 22, fontWeight: 700, color: THEME.textMain }}>运行日志与任务</h2>
          <p style={{ margin: '8px 0 0', color: THEME.danger }}>{pageError}</p>
        </div>
      </div>
    )
  }

  const aiLogs = aiLogsQuery.data
  const scraperTasks = scraperTasksQuery.data
  const aiConfig = aiConfigQuery.data
  const taskDetail = scraperTaskDetailQuery.data
  const relatedAICallLogs = relatedAICallLogsQuery.data
  const totalLogPages = Math.max(1, Math.ceil((aiLogs?.total || 0) / LOG_PAGE_SIZE))
  const totalTaskPages = Math.max(1, Math.ceil((scraperTasks?.total || 0) / TASK_PAGE_SIZE))

  return (
    <div style={{ padding: '24px 32px 32px', background: THEME.bg, minHeight: '100vh' }}>
      {/* Header */}
      <div
        style={{
          ...glassCard,
          padding: '24px 28px',
          marginBottom: 20,
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          gap: 16,
          flexWrap: 'wrap',
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', gap: 16 }}>
          <div
            style={{
              width: 48,
              height: 48,
              borderRadius: 14,
              background: 'linear-gradient(135deg, #f59e0b, #d97706)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              boxShadow: '0 4px 14px rgba(245, 158, 11, 0.35)',
              flexShrink: 0,
            }}
          >
            <RocketOutlined style={{ fontSize: 22, color: '#fff' }} />
          </div>
          <div>
            <h1 style={{ margin: 0, fontSize: 22, fontWeight: 700, color: THEME.textMain, lineHeight: 1.3 }}>
              运行日志与任务
            </h1>
            <p style={{ margin: '4px 0 0', fontSize: 13, color: THEME.textSecondary }}>
              统一查看 AI 调用日志、抓取任务和当前生效配置
            </p>
          </div>
        </div>

        <div style={{ display: 'flex', gap: 12, flexWrap: 'wrap' }}>
          <div
            style={{
              ...solidCard,
              padding: '12px 20px',
              display: 'flex',
              alignItems: 'center',
              gap: 8,
              minWidth: 120,
            }}
          >
            <span style={{ fontSize: 24, fontWeight: 700, color: THEME.primary }}>{aiLogs?.total || 0}</span>
            <span style={{ fontSize: 12, color: THEME.textSecondary }}>AI 日志</span>
          </div>
          <div
            style={{
              ...solidCard,
              padding: '12px 20px',
              display: 'flex',
              alignItems: 'center',
              gap: 8,
              minWidth: 120,
            }}
          >
            <span style={{ fontSize: 24, fontWeight: 700, color: THEME.accent }}>{scraperTasks?.total || 0}</span>
            <span style={{ fontSize: 12, color: THEME.textSecondary }}>抓取任务</span>
          </div>
        </div>
      </div>

      {/* Inline Config Chips */}
      <div
        style={{
          display: 'flex',
          gap: 10,
          flexWrap: 'wrap',
          marginBottom: 20,
        }}
      >
        {[
          {
            label: '当前主 Provider',
            value: aiConfig?.configs?.ai_provider || '未设置',
            icon: <ApiOutlined />,
          },
          {
            label: '默认模型',
            value: aiConfig?.configs?.ai_model || '未设置',
            icon: <ThunderboltOutlined />,
          },
          {
            label: 'Fallback',
            value: aiConfig?.configs?.ai_fallback_provider || '未启用',
            icon: <CheckCircleOutlined />,
          },
        ].map((chip) => (
          <div
            key={chip.label}
            style={{
              ...solidCard,
              padding: '10px 16px',
              display: 'flex',
              alignItems: 'center',
              gap: 10,
            }}
          >
            <span style={{ fontSize: 16, color: THEME.primary }}>{chip.icon}</span>
            <div>
              <div style={{ fontSize: 11, color: THEME.textMuted }}>{chip.label}</div>
              <div style={{ fontSize: 14, fontWeight: 600, color: THEME.textMain }}>{chip.value}</div>
            </div>
          </div>
        ))}
      </div>

      {/* AI Logs Section */}
      <Card
        title={
          <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
            <div
              style={{
                width: 6,
                height: 18,
                borderRadius: 3,
                background: THEME.primary,
              }}
            />
            <span style={{ fontSize: 16, fontWeight: 600, color: THEME.textMain }}>AI 调用日志</span>
          </div>
        }
        style={{ ...solidCard, marginBottom: 20 }}
        bodyStyle={{ padding: '20px 24px' }}
      >
        {/* Filters */}
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            gap: 12,
            flexWrap: 'wrap',
            marginBottom: 16,
            paddingBottom: 16,
            borderBottom: '1px solid ' + THEME.border,
          }}
        >
          <Input
            placeholder="Trace ID"
            value={traceId}
            onChange={(e) => setTraceId(e.target.value)}
            style={{ width: 180, borderRadius: 10 }}
            prefix={<SearchOutlined style={{ color: THEME.textMuted }} />}
          />
          <Input
            placeholder="任务 ID"
            value={taskId}
            onChange={(e) => setTaskId(e.target.value)}
            style={{ width: 160, borderRadius: 10 }}
          />
          <Select
            placeholder="状态"
            allowClear
            value={status || undefined}
            onChange={(v) => setStatus(v || '')}
            style={{ width: 120 }}
            options={[
              { value: 'failed', label: '失败' },
              { value: 'success', label: '成功' },
            ]}
          />
          <Button
            type="primary"
            icon={<SearchOutlined />}
            onClick={handleApplyLogFilters}
            style={{
              borderRadius: 10,
              background: 'linear-gradient(135deg, #3b82f6, #2563eb)',
              border: 'none',
            }}
          >
            应用筛选
          </Button>
        </div>

        {/* Table */}
        <div
          style={{
            overflowX: 'auto',
            borderRadius: 12,
            border: '1px solid ' + THEME.border,
          }}
        >
          <div
            style={{
              display: 'grid',
              gridTemplateColumns: 'minmax(140px, 1fr) 80px 80px minmax(140px, 1.5fr) 70px 80px minmax(120px, 1fr)',
              gap: 8,
              padding: '10px 14px',
              background: '#f8fafc',
              borderBottom: '1px solid ' + THEME.border,
              fontSize: 12,
              fontWeight: 600,
              color: THEME.textSecondary,
            }}
          >
            <span>时间</span>
            <span>场景</span>
            <span>来源</span>
            <span>Provider / Model</span>
            <span>状态</span>
            <span>任务 ID</span>
            <span>Trace ID</span>
          </div>
          {(aiLogs?.list || []).map((item) => (
            <div
              key={item.id}
              style={{
                display: 'grid',
                gridTemplateColumns: 'minmax(140px, 1fr) 80px 80px minmax(140px, 1.5fr) 70px 80px minmax(120px, 1fr)',
                gap: 8,
                padding: '12px 14px',
                borderBottom: '1px solid ' + THEME.border,
                fontSize: 13,
                color: THEME.textMain,
                alignItems: 'center',
                transition: 'background 0.15s',
              }}
              onMouseEnter={(e) => {
                e.currentTarget.style.background = '#f8fafc'
              }}
              onMouseLeave={(e) => {
                e.currentTarget.style.background = 'transparent'
              }}
            >
              <span style={{ color: THEME.textSecondary, fontSize: 12 }}>{formatRuntimeListDateTime(item.created_at)}</span>
              <span>{item.scene || '-'}</span>
              <span>{item.source || '-'}</span>
              <span style={{ fontSize: 12 }}>{item.provider || '-'} / {item.model || '-'}</span>
              <span>{renderAICallStatusTag(item.is_success)}</span>
              <span style={{ fontSize: 12, color: THEME.textSecondary }}>{item.task_id || '--'}</span>
              <code
                style={{
                  fontSize: 11,
                  color: THEME.primary,
                  background: THEME.primaryLight,
                  padding: '2px 6px',
                  borderRadius: 4,
                  overflow: 'hidden',
                  textOverflow: 'ellipsis',
                  whiteSpace: 'nowrap',
                }}
              >
                {item.trace_id || '-'}
              </code>
            </div>
          ))}
        </div>

        {aiLogs?.list?.length === 0 && (
          <div
            style={{
              padding: '40px 0',
              textAlign: 'center',
              color: THEME.textMuted,
              fontSize: 14,
            }}
          >
            暂无 AI 调用日志
          </div>
        )}

        {/* Pagination */}
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            gap: 12,
            marginTop: 16,
          }}
        >
          <Button
            size="small"
            disabled={logPage <= 1}
            onClick={() => setLogPage((current) => current - 1)}
          >
            上一页
          </Button>
          <span style={{ fontSize: 13, color: THEME.textSecondary }}>
            第 {logPage} / {totalLogPages} 页
          </span>
          <Button
            size="small"
            disabled={logPage >= totalLogPages}
            onClick={() => setLogPage((current) => current + 1)}
          >
            下一页
          </Button>
        </div>
      </Card>

      {/* Scraper Tasks Section */}
      <Card
        title={
          <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
            <div
              style={{
                width: 6,
                height: 18,
                borderRadius: 3,
                background: THEME.accent,
              }}
            />
            <span style={{ fontSize: 16, fontWeight: 600, color: THEME.textMain }}>抓取任务列表</span>
          </div>
        }
        style={{ ...solidCard, marginBottom: 20 }}
        bodyStyle={{ padding: '20px 24px' }}
      >
        {taskActionMessage && (
          <div
            style={{
              marginBottom: 14,
              padding: '10px 14px',
              borderRadius: 10,
              background: taskActionMessage.includes('失败') || taskActionMessage.includes('错误') ? '#fef2f2' : '#f0fdf4',
              border: `1px solid ${taskActionMessage.includes('失败') || taskActionMessage.includes('错误') ? '#fecaca' : '#bbf7d0'}`,
              color: taskActionMessage.includes('失败') || taskActionMessage.includes('错误') ? '#dc2626' : '#16a34a',
              fontSize: 13,
              fontWeight: 500,
            }}
          >
            {taskActionMessage}
          </div>
        )}

        {/* Filters */}
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            gap: 12,
            flexWrap: 'wrap',
            marginBottom: 16,
            paddingBottom: 16,
            borderBottom: '1px solid ' + THEME.border,
          }}
        >
          <Select
            placeholder="状态"
            allowClear
            value={taskStatus || undefined}
            onChange={(v) => setTaskStatus(v || '')}
            style={{ width: 140 }}
            options={[
              { value: 'pending', label: '待执行' },
              { value: 'running', label: '执行中' },
              { value: 'imported', label: '已导入' },
              { value: 'succeeded', label: '已完成' },
              { value: 'failed', label: '失败' },
              { value: 'fetched', label: '已抓取' },
            ]}
          />
          <Select
            placeholder="类型"
            allowClear
            value={taskType || undefined}
            onChange={(v) => setTaskType(v || '')}
            style={{ width: 160 }}
            options={[
              { value: 'import_questions', label: '异步导入' },
              { value: 'question_pipeline_build', label: '流水线生成' },
              { value: 'fetch_snapshot', label: '抓取快照' },
            ]}
          />
          <Button
            type="primary"
            icon={<SearchOutlined />}
            onClick={handleApplyTaskFilters}
            style={{
              borderRadius: 10,
              background: 'linear-gradient(135deg, #3b82f6, #2563eb)',
              border: 'none',
            }}
          >
            应用筛选
          </Button>
        </div>

        {/* Table */}
        <div
          style={{
            overflowX: 'auto',
            borderRadius: 12,
            border: '1px solid ' + THEME.border,
          }}
        >
          <div
            style={{
              display: 'grid',
              gridTemplateColumns: '60px minmax(120px, 1fr) 90px 80px 80px minmax(120px, 1fr) minmax(160px, 1.5fr) 140px',
              gap: 8,
              padding: '10px 14px',
              background: '#f8fafc',
              borderBottom: '1px solid ' + THEME.border,
              fontSize: 12,
              fontWeight: 600,
              color: THEME.textSecondary,
            }}
          >
            <span>ID</span>
            <span>时间</span>
            <span>类型</span>
            <span>来源</span>
            <span>状态</span>
            <span>标题 / URL</span>
            <span>结果</span>
            <span style={{ textAlign: 'center' }}>操作</span>
          </div>
          {(scraperTasks?.list || []).map((item) => (
            <div
              key={item.id}
              style={{
                display: 'grid',
                gridTemplateColumns: '60px minmax(120px, 1fr) 90px 80px 80px minmax(120px, 1fr) minmax(160px, 1.5fr) 140px',
                gap: 8,
                padding: '12px 14px',
                borderBottom: '1px solid ' + THEME.border,
                fontSize: 13,
                color: THEME.textMain,
                alignItems: 'center',
                transition: 'background 0.15s',
              }}
              onMouseEnter={(e) => {
                e.currentTarget.style.background = '#f8fafc'
              }}
              onMouseLeave={(e) => {
                e.currentTarget.style.background = 'transparent'
              }}
            >
              <code style={{ fontSize: 11, color: THEME.textSecondary }}>{item.id}</code>
              <span style={{ color: THEME.textSecondary, fontSize: 12 }}>{formatRuntimeListDateTime(item.created_at)}</span>
              <span style={{ fontSize: 12 }}>{renderScraperTaskTypeText(item.task_type)}</span>
              <span style={{ fontSize: 12 }}>{item.source || '-'}</span>
              <span>{renderScraperTaskStatusTag(item.status)}</span>
              <Tooltip title={item.source_url || item.source_title}>
                <span
                  style={{
                    fontSize: 12,
                    overflow: 'hidden',
                    textOverflow: 'ellipsis',
                    whiteSpace: 'nowrap',
                  }}
                >
                  {item.source_title || item.source_url || '-'}
                </span>
              </Tooltip>
              <Tooltip title={buildScraperTaskSummary(item)}>
                <span
                  style={{
                    fontSize: 12,
                    color: item.error_msg?.trim() ? THEME.danger : THEME.textSecondary,
                    overflow: 'hidden',
                    textOverflow: 'ellipsis',
                    whiteSpace: 'nowrap',
                  }}
                >
                  {buildScraperTaskSummary(item)}
                </span>
              </Tooltip>
              <div style={{ display: 'flex', gap: 6, justifyContent: 'center' }}>
                <Button
                  size="small"
                  type={selectedTaskId === item.id ? 'primary' : 'default'}
                  icon={selectedTaskId === item.id ? <EyeInvisibleOutlined /> : <EyeOutlined />}
                  onClick={() => handleToggleTaskDetail(item.id)}
                  style={{ borderRadius: 6, fontSize: 12 }}
                >
                  {selectedTaskId === item.id ? '收起' : '详情'}
                </Button>
                <Button
                  size="small"
                  icon={<FileTextOutlined />}
                  onClick={() => handleViewTaskLogs(item.id)}
                  style={{ borderRadius: 6, fontSize: 12 }}
                >
                  日志
                </Button>
                {item.status === 'failed' &&
                  ['import_questions', 'question_pipeline_build'].includes(item.task_type || '') && (
                    <Button
                      size="small"
                      icon={<ReloadOutlined spin={retryTaskMutation.isPending && retryTaskMutation.variables === item.id} />}
                      loading={retryTaskMutation.isPending && retryTaskMutation.variables === item.id}
                      disabled={retryTaskMutation.isPending}
                      onClick={() => {
                        setTaskActionMessage(`正在重新投递任务 #${item.id}...`)
                        retryTaskMutation.mutate(item.id)
                      }}
                      style={{ borderRadius: 6, fontSize: 12 }}
                    >
                      重试
                    </Button>
                  )}
              </div>
            </div>
          ))}
        </div>

        {scraperTasks?.list?.length === 0 && (
          <div
            style={{
              padding: '40px 0',
              textAlign: 'center',
              color: THEME.textMuted,
              fontSize: 14,
            }}
          >
            暂无抓取任务
          </div>
        )}

        {/* Pagination */}
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            gap: 12,
            marginTop: 16,
          }}
        >
          <Button
            size="small"
            disabled={taskPage <= 1}
            onClick={() => setTaskPage((current) => current - 1)}
          >
            上一页
          </Button>
          <span style={{ fontSize: 13, color: THEME.textSecondary }}>
            第 {taskPage} / {totalTaskPages} 页
          </span>
          <Button
            size="small"
            disabled={taskPage >= totalTaskPages}
            onClick={() => setTaskPage((current) => current + 1)}
          >
            下一页
          </Button>
        </div>

        {/* Task Detail */}
        {selectedTaskId && (
          <div
            style={{
              ...solidCard,
              marginTop: 20,
              overflow: 'hidden',
            }}
          >
            <div
              style={{
                padding: '16px 20px',
                borderBottom: '1px solid ' + THEME.border,
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'space-between',
              }}
            >
              <div>
                <span style={{ fontSize: 15, fontWeight: 600, color: THEME.textMain }}>
                  任务详情 #{selectedTaskId}
                </span>
                <div style={{ fontSize: 12, color: THEME.textMuted, marginTop: 2 }}>
                  查看入队载荷、执行结果与时间线
                </div>
              </div>
              <Button
                size="small"
                onClick={() => setSelectedTaskId(null)}
                style={{ borderRadius: 8 }}
              >
                关闭详情
              </Button>
            </div>

            <div style={{ padding: '16px 20px' }}>
              <Button
                icon={<FileTextOutlined />}
                size="small"
                onClick={() => handleViewTaskLogs(selectedTaskId)}
                style={{ borderRadius: 8, marginBottom: 16 }}
              >
                查看关联 AI 日志
              </Button>

              {scraperTaskDetailQuery.isLoading && (
                <p style={{ color: THEME.textSecondary, fontSize: 13 }}>正在加载任务详情...</p>
              )}
              {scraperTaskDetailQuery.isError && (
                <p style={{ color: THEME.danger, fontSize: 13 }}>
                  {extractErrorMessage(scraperTaskDetailQuery.error, '读取任务详情失败')}
                </p>
              )}

              {!scraperTaskDetailQuery.isLoading && !scraperTaskDetailQuery.isError && taskDetail ? (
                <>
                  <div
                    style={{
                      display: 'grid',
                      gridTemplateColumns: 'repeat(auto-fit, minmax(200px, 1fr))',
                      gap: '12px 16px',
                      marginBottom: 20,
                    }}
                  >
                    {[
                      { label: '任务类型', value: renderScraperTaskTypeText(taskDetail.task_type) },
                      { label: '任务状态', value: renderScraperTaskStatusTag(taskDetail.status) },
                      { label: '来源', value: taskDetail.source || '-' },
                      { label: '来源标题', value: taskDetail.source_title || '-' },
                      {
                        label: '来源 URL',
                        value: taskDetail.source_url || '-',
                        code: true,
                      },
                      {
                        label: '题目 / 导入 / 重试',
                        value: `${taskDetail.question_count} / ${taskDetail.imported_count} / ${taskDetail.retry_count || 0}`,
                      },
                      { label: '创建时间', value: formatRuntimeListDateTime(taskDetail.created_at) },
                      { label: '开始时间', value: formatRuntimeListDateTime(taskDetail.started_at) },
                      { label: '结束时间', value: formatRuntimeListDateTime(taskDetail.finished_at) },
                      { label: '更新时间', value: formatRuntimeListDateTime(taskDetail.updated_at) },
                    ].map((field) => (
                      <div key={field.label}>
                        <div style={{ fontSize: 11, color: THEME.textMuted, marginBottom: 2 }}>{field.label}</div>
                        {field.code ? (
                          <code style={{ fontSize: 12, color: THEME.textSecondary }}>{field.value}</code>
                        ) : (
                          <div style={{ fontSize: 13, fontWeight: 500, color: THEME.textMain }}>{field.value}</div>
                        )}
                      </div>
                    ))}
                    <div style={{ gridColumn: '1 / -1' }}>
                      <div style={{ fontSize: 11, color: THEME.textMuted, marginBottom: 2 }}>错误信息</div>
                      <div
                        style={{
                          fontSize: 13,
                          color: taskDetail.error_msg ? THEME.danger : THEME.textMuted,
                        }}
                      >
                        {taskDetail.error_msg || '--'}
                      </div>
                    </div>
                  </div>

                  <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 16, marginBottom: 20 }}>
                    <div>
                      <div
                        style={{
                          fontSize: 12,
                          fontWeight: 600,
                          color: THEME.textSecondary,
                          marginBottom: 8,
                        }}
                      >
                        任务载荷
                      </div>
                      <pre
                        style={{
                          margin: 0,
                          padding: 14,
                          borderRadius: 10,
                          background: '#0f172a',
                          color: '#e2e8f0',
                          fontSize: 12,
                          lineHeight: 1.6,
                          overflowX: 'auto',
                          maxHeight: 300,
                          overflowY: 'auto',
                        }}
                      >
                        {formatTaskJSONText(taskDetail.payload_json)}
                      </pre>
                    </div>
                    <div>
                      <div
                        style={{
                          fontSize: 12,
                          fontWeight: 600,
                          color: THEME.textSecondary,
                          marginBottom: 8,
                        }}
                      >
                        执行结果
                      </div>
                      <pre
                        style={{
                          margin: 0,
                          padding: 14,
                          borderRadius: 10,
                          background: '#0f172a',
                          color: '#e2e8f0',
                          fontSize: 12,
                          lineHeight: 1.6,
                          overflowX: 'auto',
                          maxHeight: 300,
                          overflowY: 'auto',
                        }}
                      >
                        {formatTaskJSONText(taskDetail.result_json)}
                      </pre>
                    </div>
                  </div>

                  {/* Related AI Logs */}
                  <div
                    style={{
                      borderTop: '1px solid ' + THEME.border,
                      paddingTop: 20,
                    }}
                  >
                    <div
                      style={{
                        display: 'flex',
                        alignItems: 'center',
                        justifyContent: 'space-between',
                        marginBottom: 14,
                      }}
                    >
                      <div>
                        <span style={{ fontSize: 15, fontWeight: 600, color: THEME.textMain }}>
                          关联 AI 日志
                        </span>
                        <div style={{ fontSize: 12, color: THEME.textMuted, marginTop: 2 }}>
                          展示当前任务最近关联的模型调用
                        </div>
                      </div>
                      <Button
                        size="small"
                        icon={<FileTextOutlined />}
                        onClick={() => handleViewTaskLogs(selectedTaskId)}
                        style={{ borderRadius: 8 }}
                      >
                        打开完整日志列表
                      </Button>
                    </div>

                    {relatedAICallLogsQuery.isLoading && (
                      <p style={{ color: THEME.textSecondary, fontSize: 13 }}>正在加载关联 AI 日志...</p>
                    )}
                    {relatedAICallLogsQuery.isError && (
                      <p style={{ color: THEME.danger, fontSize: 13 }}>
                        {extractErrorMessage(relatedAICallLogsQuery.error, '读取关联 AI 日志失败')}
                      </p>
                    )}
                    {!relatedAICallLogsQuery.isLoading &&
                      !relatedAICallLogsQuery.isError &&
                      (relatedAICallLogs?.list || []).length === 0 && (
                        <p style={{ color: THEME.textMuted, fontSize: 13 }}>当前任务还没有关联的 AI 调用日志。</p>
                      )}

                    {!relatedAICallLogsQuery.isLoading &&
                      !relatedAICallLogsQuery.isError &&
                      (relatedAICallLogs?.list || []).length > 0 && (
                        <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
                          {(relatedAICallLogs?.list || []).map((item) => (
                            <div
                              key={item.id}
                              style={{
                                borderRadius: 12,
                                border: '1px solid ' + THEME.border,
                                overflow: 'hidden',
                              }}
                            >
                              <div
                                style={{
                                  padding: '12px 16px',
                                  display: 'flex',
                                  alignItems: 'center',
                                  justifyContent: 'space-between',
                                  gap: 12,
                                  cursor: 'pointer',
                                  background: '#fafafa',
                                  transition: 'background 0.15s',
                                }}
                                onClick={() => handleToggleRelatedLogDetail(item.id)}
                                onMouseEnter={(e) => {
                                  e.currentTarget.style.background = '#f1f5f9'
                                }}
                                onMouseLeave={(e) => {
                                  e.currentTarget.style.background = '#fafafa'
                                }}
                              >
                                <div style={{ display: 'flex', alignItems: 'center', gap: 12, flexWrap: 'wrap' }}>
                                  <strong style={{ fontSize: 14, color: THEME.textMain }}>{item.scene || '-'}</strong>
                                  {renderAICallStatusTag(item.is_success)}
                                  <span style={{ fontSize: 12, color: THEME.textSecondary }}>
                                    {formatRuntimeListDateTime(item.created_at)}
                                  </span>
                                  <span style={{ fontSize: 12, color: THEME.textMuted }}>
                                    {buildAICallInlineSummary(item)}
                                  </span>
                                </div>
                                <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                                  <code
                                    style={{
                                      fontSize: 11,
                                      color: THEME.primary,
                                      background: THEME.primaryLight,
                                      padding: '2px 6px',
                                      borderRadius: 4,
                                    }}
                                  >
                                    {item.trace_id || '-'}
                                  </code>
                                  {expandedRelatedLogId === item.id ? (
                                    <UpOutlined style={{ color: THEME.textMuted }} />
                                  ) : (
                                    <DownOutlined style={{ color: THEME.textMuted }} />
                                  )}
                                </div>
                              </div>

                              {expandedRelatedLogId === item.id && (
                                <div style={{ padding: 16, borderTop: '1px solid ' + THEME.border }}>
                                  {relatedAICallLogDetailQuery.isLoading && (
                                    <p style={{ color: THEME.textSecondary, fontSize: 13 }}>正在加载 AI 日志详情...</p>
                                  )}
                                  {relatedAICallLogDetailQuery.isError && (
                                    <p style={{ color: THEME.danger, fontSize: 13 }}>
                                      {extractErrorMessage(
                                        relatedAICallLogDetailQuery.error,
                                        '读取 AI 日志详情失败',
                                      )}
                                    </p>
                                  )}
                                  {!relatedAICallLogDetailQuery.isLoading &&
                                    !relatedAICallLogDetailQuery.isError &&
                                    relatedAICallLogDetailQuery.data && (
                                      <>
                                        <div
                                          style={{
                                            display: 'flex',
                                            gap: 8,
                                            flexWrap: 'wrap',
                                            marginBottom: 12,
                                          }}
                                        >
                                          {[
                                            {
                                              label: 'Trace ID',
                                              value: relatedAICallLogDetailQuery.data.trace_id,
                                            },
                                            {
                                              label: '渲染 Prompt',
                                              value: relatedAICallLogDetailQuery.data.rendered_prompt,
                                            },
                                            {
                                              label: '模型原始输出',
                                              value: relatedAICallLogDetailQuery.data.model_output,
                                            },
                                            ...(normalizeRuntimeCopyText(
                                              relatedAICallLogDetailQuery.data.model_error,
                                            )
                                              ? [
                                                  {
                                                    label: '模型错误',
                                                    value: relatedAICallLogDetailQuery.data.model_error,
                                                  },
                                                ]
                                              : []),
                                          ].map((btn) => (
                                            <Button
                                              key={btn.label}
                                              size="small"
                                              icon={<CopyOutlined />}
                                              onClick={() =>
                                                handleCopyRelatedLogText(btn.value || '', btn.label)
                                              }
                                              style={{ borderRadius: 6, fontSize: 12 }}
                                            >
                                              复制{btn.label}
                                            </Button>
                                          ))}
                                        </div>

                                        {relatedLogCopyMessage && (
                                          <div
                                            style={{
                                              marginBottom: 12,
                                              padding: '8px 12px',
                                              borderRadius: 8,
                                              background: '#f0fdf4',
                                              border: '1px solid #bbf7d0',
                                              color: '#16a34a',
                                              fontSize: 12,
                                              fontWeight: 500,
                                            }}
                                          >
                                            {relatedLogCopyMessage}
                                          </div>
                                        )}

                                        <div
                                          style={{
                                            display: 'grid',
                                            gridTemplateColumns: 'repeat(auto-fit, minmax(180px, 1fr))',
                                            gap: '10px 16px',
                                            marginBottom: 16,
                                          }}
                                        >
                                          {[
                                            {
                                              label: 'Prompt 来源',
                                              value: formatRuntimeDetailText(
                                                relatedAICallLogDetailQuery.data.prompt_source,
                                              ),
                                            },
                                            {
                                              label: '模板',
                                              value: formatRuntimeDetailText(
                                                relatedAICallLogDetailQuery.data.selected_prompt_name,
                                              ),
                                            },
                                            {
                                              label: 'Provider / Model',
                                              value: `${relatedAICallLogDetailQuery.data.provider || '-'} / ${relatedAICallLogDetailQuery.data.model || '-'}`,
                                            },
                                            {
                                              label: '耗时',
                                              value: `${relatedAICallLogDetailQuery.data.latency_ms || 0} ms`,
                                            },
                                          ].map((field) => (
                                            <div key={field.label}>
                                              <div
                                                style={{
                                                  fontSize: 11,
                                                  color: THEME.textMuted,
                                                  marginBottom: 2,
                                                }}
                                              >
                                                {field.label}
                                              </div>
                                              <div
                                                style={{
                                                  fontSize: 13,
                                                  fontWeight: 500,
                                                  color: THEME.textMain,
                                                }}
                                              >
                                                {field.value}
                                              </div>
                                            </div>
                                          ))}
                                        </div>

                                        <div
                                          style={{
                                            display: 'flex',
                                            flexDirection: 'column',
                                            gap: 12,
                                          }}
                                        >
                                          {[
                                            {
                                              title: '用户输入',
                                              content: formatRuntimeDetailText(
                                                relatedAICallLogDetailQuery.data.user_input,
                                              ),
                                            },
                                            {
                                              title: '渲染 Prompt',
                                              content: formatRuntimeDetailText(
                                                relatedAICallLogDetailQuery.data.rendered_prompt,
                                              ),
                                            },
                                            {
                                              title: '请求消息',
                                              content: formatTaskJSONText(
                                                relatedAICallLogDetailQuery.data.request_messages,
                                              ),
                                            },
                                            {
                                              title: '运行配置',
                                              content: formatTaskJSONText(
                                                relatedAICallLogDetailQuery.data.runtime_config,
                                              ),
                                            },
                                            {
                                              title: '场景配置',
                                              content: formatTaskJSONText(
                                                relatedAICallLogDetailQuery.data.scene_config,
                                              ),
                                            },
                                            {
                                              title: '模型原始输出',
                                              content: formatRuntimeDetailText(
                                                relatedAICallLogDetailQuery.data.model_output,
                                              ),
                                            },
                                            {
                                              title: '模型错误',
                                              content: formatRuntimeDetailText(
                                                relatedAICallLogDetailQuery.data.model_error,
                                              ),
                                            },
                                          ].map((block) => (
                                            <div key={block.title}>
                                              <div
                                                style={{
                                                  fontSize: 12,
                                                  fontWeight: 600,
                                                  color: THEME.textSecondary,
                                                  marginBottom: 6,
                                                }}
                                              >
                                                {block.title}
                                              </div>
                                              <pre
                                                style={{
                                                  margin: 0,
                                                  padding: 12,
                                                  borderRadius: 10,
                                                  background: '#0f172a',
                                                  color: '#e2e8f0',
                                                  fontSize: 12,
                                                  lineHeight: 1.6,
                                                  overflowX: 'auto',
                                                  maxHeight: 200,
                                                  overflowY: 'auto',
                                                }}
                                              >
                                                {block.content}
                                              </pre>
                                            </div>
                                          ))}
                                        </div>
                                      </>
                                    )}
                                </div>
                              )}
                            </div>
                          ))}
                        </div>
                      )}
                  </div>
                </>
              ) : null}
            </div>
          </div>
        )}
      </Card>
    </div>
  )
}
