import { useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
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
function renderAICallStatusText(isSuccess: boolean): string {
  return isSuccess ? 'success' : 'failed'
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

/**
 * 统一渲染抓取任务状态文本，减少后台列表直接暴露内部状态码的阅读成本。
 */
function renderScraperTaskStatusText(status?: string): string {
  switch ((status || '').trim()) {
    case 'pending':
      return '待执行'
    case 'running':
      return '执行中'
    case 'fetched':
      return '已抓取'
    case 'cleaned':
      return '已清洗'
    case 'imported':
      return '已导入'
    case 'succeeded':
      return '已完成'
    case 'failed':
      return '失败'
    default:
      return status?.trim() || '-'
  }
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
      <section className="admin-panel">
        <span className="admin-tag">运行任务</span>
        <h2>运行日志与任务</h2>
        <p className="admin-copy">正在加载 AI 日志、抓取任务和当前运行配置。</p>
      </section>
    )
  }

  if (pageError) {
    return (
      <section className="admin-panel">
        <span className="admin-tag">运行任务</span>
        <h2>运行日志与任务</h2>
        <p className="admin-copy">{pageError}</p>
      </section>
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
    <section className="admin-panel admin-runtime-list-page">
      <div className="admin-runtime-list-page__hero">
        <div>
          <span className="admin-tag">运行任务</span>
          <h2>统一查看日志、任务和当前生效配置</h2>
          <p className="admin-copy">先用现有 AI 调用日志和 scraper task 建立统一排查入口，后续再演进成更完整的任务体系。</p>
        </div>
        <div className="admin-runtime-list-page__summary">
          <strong>{aiLogs?.total || 0}</strong>
          <span>AI 日志</span>
          <strong>{scraperTasks?.total || 0}</strong>
          <span>抓取任务</span>
        </div>
      </div>

      <div className="admin-runtime-inline-config">
        <span>当前主 Provider：{aiConfig?.configs?.ai_provider || '未设置'}</span>
        <span>默认模型：{aiConfig?.configs?.ai_model || '未设置'}</span>
        <span>Fallback：{aiConfig?.configs?.ai_fallback_provider || '未启用'}</span>
      </div>

      <div className="admin-runtime-list-page__layout">
        <section className="admin-runtime-section">
          <div className="admin-runtime-section__head">
            <div>
              <h3>AI 调用日志</h3>
              <p>支持直接按 trace_id、异步任务 ID 与状态过滤。</p>
            </div>
          </div>

          <div className="admin-runtime-filters">
            <label className="admin-field">
              <span>Trace ID</span>
              <input
                value={traceId}
                onChange={(event) => setTraceId(event.target.value)}
                placeholder="输入 trace_id 片段"
              />
            </label>
            <label className="admin-field">
              <span>任务 ID</span>
              <input
                value={taskId}
                onChange={(event) => setTaskId(event.target.value)}
                placeholder="输入异步任务 ID"
              />
            </label>
            <label className="admin-field">
              <span>状态</span>
              <select value={status} onChange={(event) => setStatus(event.target.value)}>
                <option value="">全部</option>
                <option value="failed">失败</option>
                <option value="success">成功</option>
              </select>
            </label>
            <button className="admin-link" type="button" onClick={handleApplyLogFilters}>
              应用筛选
            </button>
          </div>

          <div className="admin-runtime-list-table">
            <div className="admin-runtime-list-table__row admin-runtime-list-table__row--head">
              <span>时间</span>
              <span>场景</span>
              <span>来源</span>
              <span>Provider / Model</span>
              <span>状态</span>
              <span>任务 ID</span>
              <span>Trace ID</span>
            </div>
            {(aiLogs?.list || []).map((item) => (
              <div className="admin-runtime-list-table__row" key={item.id}>
                <span>{formatRuntimeListDateTime(item.created_at)}</span>
                <span>{item.scene || '-'}</span>
                <span>{item.source || '-'}</span>
                <span>{item.provider || '-'} / {item.model || '-'}</span>
                <span>{renderAICallStatusText(item.is_success)}</span>
                <span>{item.task_id || '--'}</span>
                <code>{item.trace_id || '-'}</code>
              </div>
            ))}
          </div>

          <div className="admin-question-pagination">
            <button className="admin-link" type="button" disabled={logPage <= 1} onClick={() => setLogPage((current) => current - 1)}>
              上一页
            </button>
            <span>
              第 {logPage} / {totalLogPages} 页
            </span>
            <button
              className="admin-link"
              type="button"
              disabled={logPage >= totalLogPages}
              onClick={() => setLogPage((current) => current + 1)}
            >
              下一页
            </button>
          </div>
        </section>

        <section className="admin-runtime-section">
          <div className="admin-runtime-section__head">
            <div>
              <h3>抓取任务列表</h3>
              <p>按状态和任务类型筛选，并直接查看异步任务载荷与结果。</p>
            </div>
          </div>
          {taskActionMessage ? <p className="admin-copy">{taskActionMessage}</p> : null}

          <div className="admin-runtime-filters">
            <label className="admin-field">
              <span>状态</span>
              <select value={taskStatus} onChange={(event) => setTaskStatus(event.target.value)}>
                <option value="">全部</option>
                <option value="pending">待执行</option>
                <option value="running">执行中</option>
                <option value="imported">已导入</option>
                <option value="succeeded">已完成</option>
                <option value="failed">失败</option>
                <option value="fetched">已抓取</option>
              </select>
            </label>
            <label className="admin-field">
              <span>类型</span>
              <select value={taskType} onChange={(event) => setTaskType(event.target.value)}>
                <option value="">全部</option>
                <option value="import_questions">异步导入</option>
                <option value="question_pipeline_build">流水线生成</option>
                <option value="fetch_snapshot">抓取快照</option>
              </select>
            </label>
            <button className="admin-link" type="button" onClick={handleApplyTaskFilters}>
              应用筛选
            </button>
          </div>

          <div className="admin-runtime-list-table admin-runtime-list-table--scraper">
            <div className="admin-runtime-list-table__row admin-runtime-list-table__row--head">
              <span>ID</span>
              <span>时间</span>
              <span>类型</span>
              <span>来源</span>
              <span>状态</span>
              <span>标题 / URL</span>
              <span>结果</span>
              <span>操作</span>
            </div>
            {(scraperTasks?.list || []).map((item) => (
              <div className="admin-runtime-list-table__row" key={item.id}>
                <code>{item.id}</code>
                <span>{formatRuntimeListDateTime(item.created_at)}</span>
                <span>{renderScraperTaskTypeText(item.task_type)}</span>
                <span>{item.source || '-'}</span>
                <span>{renderScraperTaskStatusText(item.status)}</span>
                <span>{item.source_title || item.source_url || '-'}</span>
                <span>{buildScraperTaskSummary(item)}</span>
                <span>
                  <div className="admin-runtime-task-actions">
                     <button
                       className="admin-link"
                       type="button"
                       onClick={() => handleToggleTaskDetail(item.id)}
                     >
                       {selectedTaskId === item.id ? '收起详情' : '查看详情'}
                     </button>
                     <button
                       className="admin-link"
                       type="button"
                       onClick={() => handleViewTaskLogs(item.id)}
                     >
                       关联 AI 日志
                     </button>
                     {item.status === 'failed' && ['import_questions', 'question_pipeline_build'].includes(item.task_type || '') ? (
                       <button
                         className="admin-link"
                         type="button"
                         disabled={retryTaskMutation.isPending}
                        onClick={() => {
                          setTaskActionMessage(`正在重新投递任务 #${item.id}...`)
                          retryTaskMutation.mutate(item.id)
                        }}
                      >
                        {retryTaskMutation.isPending ? '重试中...' : '重新投递'}
                      </button>
                    ) : null}
                  </div>
                </span>
              </div>
            ))}
          </div>

          {selectedTaskId ? (
            <article className="admin-runtime-task-detail">
              <div className="admin-runtime-section__head">
                 <div>
                   <h3>任务详情 #{selectedTaskId}</h3>
                   <p>查看入队载荷、执行结果与时间线，并可反查关联 AI 调用日志。</p>
                 </div>
                <button className="admin-link" type="button" onClick={() => setSelectedTaskId(null)}>
                  关闭详情
                </button>
              </div>
              <div className="admin-runtime-task-detail__toolbar">
                <button className="admin-link" type="button" onClick={() => handleViewTaskLogs(selectedTaskId)}>
                  查看关联 AI 日志
                </button>
              </div>

              {scraperTaskDetailQuery.isLoading ? <p className="admin-copy">正在加载任务详情。</p> : null}
              {scraperTaskDetailQuery.isError ? (
                <p className="admin-copy">{extractErrorMessage(scraperTaskDetailQuery.error, '读取任务详情失败')}</p>
              ) : null}

              {!scraperTaskDetailQuery.isLoading && !scraperTaskDetailQuery.isError && taskDetail ? (
                <>
                  <div className="admin-runtime-task-detail__grid">
                    <div className="admin-runtime-task-detail__item">
                      <strong>任务类型</strong>
                      <span>{renderScraperTaskTypeText(taskDetail.task_type)}</span>
                    </div>
                    <div className="admin-runtime-task-detail__item">
                      <strong>任务状态</strong>
                      <span>{renderScraperTaskStatusText(taskDetail.status)}</span>
                    </div>
                    <div className="admin-runtime-task-detail__item">
                      <strong>来源</strong>
                      <span>{taskDetail.source || '-'}</span>
                    </div>
                    <div className="admin-runtime-task-detail__item">
                      <strong>来源标题</strong>
                      <span>{taskDetail.source_title || '-'}</span>
                    </div>
                    <div className="admin-runtime-task-detail__item">
                      <strong>来源 URL</strong>
                      <code>{taskDetail.source_url || '-'}</code>
                    </div>
                    <div className="admin-runtime-task-detail__item">
                      <strong>题目 / 导入 / 重试</strong>
                      <span>{taskDetail.question_count} / {taskDetail.imported_count} / {taskDetail.retry_count || 0}</span>
                    </div>
                    <div className="admin-runtime-task-detail__item">
                      <strong>创建时间</strong>
                      <span>{formatRuntimeListDateTime(taskDetail.created_at)}</span>
                    </div>
                    <div className="admin-runtime-task-detail__item">
                      <strong>开始时间</strong>
                      <span>{formatRuntimeListDateTime(taskDetail.started_at)}</span>
                    </div>
                    <div className="admin-runtime-task-detail__item">
                      <strong>结束时间</strong>
                      <span>{formatRuntimeListDateTime(taskDetail.finished_at)}</span>
                    </div>
                    <div className="admin-runtime-task-detail__item">
                      <strong>更新时间</strong>
                      <span>{formatRuntimeListDateTime(taskDetail.updated_at)}</span>
                    </div>
                    <div className="admin-runtime-task-detail__item admin-runtime-task-detail__item--full">
                      <strong>错误信息</strong>
                      <span>{taskDetail.error_msg || '--'}</span>
                    </div>
                  </div>

                  <div className="admin-runtime-task-detail__blocks">
                    <div className="admin-runtime-task-detail__block">
                      <h4>任务载荷</h4>
                      <pre>{formatTaskJSONText(taskDetail.payload_json)}</pre>
                    </div>
                    <div className="admin-runtime-task-detail__block">
                      <h4>执行结果</h4>
                      <pre>{formatTaskJSONText(taskDetail.result_json)}</pre>
                    </div>
                  </div>

                  <div className="admin-runtime-task-detail__related-logs">
                    <div className="admin-runtime-section__head">
                      <div>
                        <h3>关联 AI 日志</h3>
                        <p>展示当前任务最近关联的模型调用，便于和任务结果一屏对照排查。</p>
                      </div>
                      <button className="admin-link" type="button" onClick={() => handleViewTaskLogs(selectedTaskId)}>
                        打开完整日志列表
                      </button>
                    </div>

                    {relatedAICallLogsQuery.isLoading ? <p className="admin-copy">正在加载关联 AI 日志。</p> : null}
                    {relatedAICallLogsQuery.isError ? (
                      <p className="admin-copy">{extractErrorMessage(relatedAICallLogsQuery.error, '读取关联 AI 日志失败')}</p>
                    ) : null}
                    {!relatedAICallLogsQuery.isLoading && !relatedAICallLogsQuery.isError && (relatedAICallLogs?.list || []).length === 0 ? (
                      <p className="admin-copy">当前任务还没有关联的 AI 调用日志。</p>
                    ) : null}

                    {!relatedAICallLogsQuery.isLoading && !relatedAICallLogsQuery.isError && (relatedAICallLogs?.list || []).length ? (
                      <div className="admin-runtime-related-log-list">
                        {(relatedAICallLogs?.list || []).map((item) => (
                          <article className="admin-runtime-related-log-card" key={item.id}>
                            <div className="admin-runtime-related-log-card__head">
                              <strong>{item.scene || '-'}</strong>
                              <span>{renderAICallStatusText(item.is_success)}</span>
                            </div>
                            <div className="admin-runtime-related-log-card__meta">
                              <span>{formatRuntimeListDateTime(item.created_at)}</span>
                              <span>{buildAICallInlineSummary(item)}</span>
                            </div>
                            <div className="admin-runtime-related-log-card__trace">
                              <span>Trace ID</span>
                              <code>{item.trace_id || '-'}</code>
                            </div>
                            <div className="admin-runtime-related-log-card__actions">
                              <button className="admin-link" type="button" onClick={() => handleToggleRelatedLogDetail(item.id)}>
                                {expandedRelatedLogId === item.id ? '收起原始信息' : '展开原始信息'}
                              </button>
                            </div>

                            {expandedRelatedLogId === item.id ? (
                              <div className="admin-runtime-related-log-card__detail">
                                {relatedAICallLogDetailQuery.isLoading ? <p className="admin-copy">正在加载 AI 日志详情。</p> : null}
                                {relatedAICallLogDetailQuery.isError ? (
                                  <p className="admin-copy">{extractErrorMessage(relatedAICallLogDetailQuery.error, '读取 AI 日志详情失败')}</p>
                                ) : null}
                                {!relatedAICallLogDetailQuery.isLoading && !relatedAICallLogDetailQuery.isError && relatedAICallLogDetailQuery.data ? (
                                  <>
                                    <div className="admin-runtime-related-log-card__detail-toolbar">
                                      <button
                                        className="admin-link"
                                        type="button"
                                        onClick={() => handleCopyRelatedLogText(relatedAICallLogDetailQuery.data.trace_id, 'Trace ID')}
                                      >
                                        复制 Trace ID
                                      </button>
                                      <button
                                        className="admin-link"
                                        type="button"
                                        onClick={() => handleCopyRelatedLogText(relatedAICallLogDetailQuery.data.rendered_prompt, '渲染 Prompt')}
                                      >
                                        复制 Prompt
                                      </button>
                                      <button
                                        className="admin-link"
                                        type="button"
                                        onClick={() => handleCopyRelatedLogText(relatedAICallLogDetailQuery.data.model_output, '模型原始输出')}
                                      >
                                        复制输出
                                      </button>
                                      {normalizeRuntimeCopyText(relatedAICallLogDetailQuery.data.model_error) ? (
                                        <button
                                          className="admin-link"
                                          type="button"
                                          onClick={() => handleCopyRelatedLogText(relatedAICallLogDetailQuery.data.model_error, '模型错误')}
                                        >
                                          复制错误
                                        </button>
                                      ) : null}
                                    </div>
                                    {relatedLogCopyMessage ? <p className="admin-copy">{relatedLogCopyMessage}</p> : null}
                                    <div className="admin-runtime-related-log-card__detail-grid">
                                      <div className="admin-runtime-related-log-card__detail-item">
                                        <strong>Prompt 来源</strong>
                                        <span>{formatRuntimeDetailText(relatedAICallLogDetailQuery.data.prompt_source)}</span>
                                      </div>
                                      <div className="admin-runtime-related-log-card__detail-item">
                                        <strong>模板</strong>
                                        <span>{formatRuntimeDetailText(relatedAICallLogDetailQuery.data.selected_prompt_name)}</span>
                                      </div>
                                      <div className="admin-runtime-related-log-card__detail-item">
                                        <strong>Provider / Model</strong>
                                        <span>{relatedAICallLogDetailQuery.data.provider || '-'} / {relatedAICallLogDetailQuery.data.model || '-'}</span>
                                      </div>
                                      <div className="admin-runtime-related-log-card__detail-item">
                                        <strong>耗时</strong>
                                        <span>{relatedAICallLogDetailQuery.data.latency_ms || 0} ms</span>
                                      </div>
                                    </div>
                                    <div className="admin-runtime-related-log-card__detail-blocks">
                                      <div className="admin-runtime-related-log-card__detail-block">
                                        <h4>用户输入</h4>
                                        <pre>{formatRuntimeDetailText(relatedAICallLogDetailQuery.data.user_input)}</pre>
                                      </div>
                                      <div className="admin-runtime-related-log-card__detail-block">
                                        <h4>渲染 Prompt</h4>
                                        <pre>{formatRuntimeDetailText(relatedAICallLogDetailQuery.data.rendered_prompt)}</pre>
                                      </div>
                                      <div className="admin-runtime-related-log-card__detail-block">
                                        <h4>请求消息</h4>
                                        <pre>{formatTaskJSONText(relatedAICallLogDetailQuery.data.request_messages)}</pre>
                                      </div>
                                      <div className="admin-runtime-related-log-card__detail-block">
                                        <h4>运行配置</h4>
                                        <pre>{formatTaskJSONText(relatedAICallLogDetailQuery.data.runtime_config)}</pre>
                                      </div>
                                      <div className="admin-runtime-related-log-card__detail-block">
                                        <h4>场景配置</h4>
                                        <pre>{formatTaskJSONText(relatedAICallLogDetailQuery.data.scene_config)}</pre>
                                      </div>
                                      <div className="admin-runtime-related-log-card__detail-block">
                                        <h4>模型原始输出</h4>
                                        <pre>{formatRuntimeDetailText(relatedAICallLogDetailQuery.data.model_output)}</pre>
                                      </div>
                                      <div className="admin-runtime-related-log-card__detail-block admin-runtime-related-log-card__detail-block--full">
                                        <h4>模型错误</h4>
                                        <pre>{formatRuntimeDetailText(relatedAICallLogDetailQuery.data.model_error)}</pre>
                                      </div>
                                    </div>
                                  </>
                                ) : null}
                              </div>
                            ) : null}
                          </article>
                        ))}
                      </div>
                    ) : null}
                  </div>
                </>
              ) : null}
            </article>
          ) : null}

          <div className="admin-question-pagination">
            <button className="admin-link" type="button" disabled={taskPage <= 1} onClick={() => setTaskPage((current) => current - 1)}>
              上一页
            </button>
            <span>
              第 {taskPage} / {totalTaskPages} 页
            </span>
            <button
              className="admin-link"
              type="button"
              disabled={taskPage >= totalTaskPages}
              onClick={() => setTaskPage((current) => current + 1)}
            >
              下一页
            </button>
          </div>
        </section>
      </div>
    </section>
  )
}
