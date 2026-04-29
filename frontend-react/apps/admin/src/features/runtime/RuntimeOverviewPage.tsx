import { useMemo } from 'react'
import { Link } from '@tanstack/react-router'
import { useQuery } from '@tanstack/react-query'
import { extractErrorMessage } from '@makejob/api-client'
import { useAdminAuthStore } from '../../state/auth'
import { fetchAICallLogs, fetchDashboardStats, fetchRuntimeAIConfig, fetchScraperTasks } from './runtimeApi'

/**
 * 将时间字段格式化成后台总览可读的本地时间文本。
 */
function formatRuntimeDateTime(value?: string): string {
  if (!value) {
    return '--'
  }

  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return value
  }

  return date.toLocaleString('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  })
}

/**
 * 将 AI 日志状态转换成后台页面可读标签。
 */
function buildAICallStatusLabel(isSuccess: boolean): string {
  return isSuccess ? '成功' : '失败'
}

/**
 * 将抓取任务类型转换为后台概览更易读的中文标签。
 */
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

/**
 * 提供后台真实运行态总览，聚合 AI 配置、生效告警、最近错误和任务快照。
 */
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

  const overviewError = useMemo(() => {
    const error = dashboardQuery.error || aiConfigQuery.error || aiLogsQuery.error || scraperTasksQuery.error
    return error ? extractErrorMessage(error, '运行态总览加载失败') : ''
  }, [aiConfigQuery.error, aiLogsQuery.error, dashboardQuery.error, scraperTasksQuery.error])

  if (dashboardQuery.isLoading || aiConfigQuery.isLoading || aiLogsQuery.isLoading || scraperTasksQuery.isLoading) {
    return (
      <section className="admin-panel">
        <span className="admin-tag">运行总览</span>
        <h2>后台运行总览</h2>
        <p className="admin-copy">正在加载 AI 生效配置、最近失败日志和任务状态。</p>
      </section>
    )
  }

  if (overviewError) {
    return (
      <section className="admin-panel">
        <span className="admin-tag">运行总览</span>
        <h2>后台运行总览</h2>
        <p className="admin-copy">{overviewError}</p>
      </section>
    )
  }

  const stats = dashboardQuery.data
  const runtimeConfig = aiConfigQuery.data
  const failedLogs = aiLogsQuery.data?.list || []
  const recentTasks = scraperTasksQuery.data?.list || []

  return (
    <section className="admin-panel admin-runtime-page">
      <div className="admin-runtime-page__hero">
        <div>
          <span className="admin-tag">运行总览</span>
          <h2>统一查看当前系统运行状态</h2>
          <p className="admin-copy">
            这里直接展示 AI 当前生效配置、最近失败调用和最近抓取任务，减少在多个页面来回切换排查。
          </p>
        </div>
        <div className="admin-runtime-page__actions">
          <Link className="admin-link" to="/runtime">
            打开运行任务页
          </Link>
          <Link className="admin-link" to="/question-pipeline">
            打开题目流水线
          </Link>
        </div>
      </div>

      <div className="admin-runtime-page__stats">
        <article className="admin-runtime-card">
          <span>总用户数</span>
          <strong>{stats?.total_users ?? 0}</strong>
        </article>
        <article className="admin-runtime-card">
          <span>题库数量</span>
          <strong>{stats?.total_questions ?? 0}</strong>
        </article>
        <article className="admin-runtime-card">
          <span>面试次数</span>
          <strong>{stats?.total_interviews ?? 0}</strong>
        </article>
        <article className="admin-runtime-card">
          <span>今日活跃</span>
          <strong>{stats?.today_active_users ?? 0}</strong>
        </article>
        <article className="admin-runtime-card">
          <span>新增用户</span>
          <strong>{stats?.new_users_today ?? 0}</strong>
        </article>
        <article className="admin-runtime-card">
          <span>Pro 会员</span>
          <strong>{stats?.pro_members ?? 0}</strong>
        </article>
      </div>

      <div className="admin-runtime-page__layout">
        <article className="admin-runtime-panel-card">
          <div className="admin-runtime-panel-card__head">
            <div>
              <h3>当前 AI 生效配置</h3>
              <p>管理员：{user?.username || '-'}</p>
            </div>
            <Link className="admin-link" to="/ai-configs">
              去 AI 配置页
            </Link>
          </div>
          <div className="admin-runtime-kv">
            <span>主 Provider</span>
            <strong>{runtimeConfig?.configs.ai_provider || '未设置'}</strong>
          </div>
          <div className="admin-runtime-kv">
            <span>默认模型</span>
            <strong>{runtimeConfig?.configs.ai_model || '未设置'}</strong>
          </div>
          <div className="admin-runtime-kv">
            <span>Fallback Provider</span>
            <strong>{runtimeConfig?.configs.ai_fallback_provider || '未启用'}</strong>
          </div>
          <div className="admin-runtime-kv">
            <span>当前支持</span>
            <strong>{runtimeConfig?.support.primary_providers.join(' / ') || '无'}</strong>
          </div>
          <div className="admin-runtime-list-block">
            <h4>运行告警</h4>
            {(runtimeConfig?.warnings || []).length > 0 ? (
              <ul>
                {runtimeConfig?.warnings.map((warning) => (
                  <li key={warning}>{warning}</li>
                ))}
              </ul>
            ) : (
              <p>当前没有运行时告警。</p>
            )}
          </div>
          <div className="admin-runtime-list-block">
            <h4>支持说明</h4>
            {(runtimeConfig?.support.notes || []).length > 0 ? (
              <ul>
                {runtimeConfig?.support.notes.map((note) => (
                  <li key={note}>{note}</li>
                ))}
              </ul>
            ) : (
              <p>当前没有额外说明。</p>
            )}
          </div>
        </article>

        <article className="admin-runtime-panel-card">
          <div className="admin-runtime-panel-card__head">
            <div>
              <h3>最近 AI 失败调用</h3>
              <p>优先显示失败日志，便于直接定位 trace。</p>
            </div>
            <Link className="admin-link" to="/runtime">
              查看全部日志
            </Link>
          </div>
          {failedLogs.length > 0 ? (
            <div className="admin-runtime-table">
              <div className="admin-runtime-table__row admin-runtime-table__row--head">
                <span>时间</span>
                <span>场景</span>
                <span>Provider</span>
                <span>Trace</span>
              </div>
              {failedLogs.map((item) => (
                <div className="admin-runtime-table__row" key={item.id}>
                  <span>{formatRuntimeDateTime(item.created_at)}</span>
                  <span>{item.scene || item.source || '-'}</span>
                  <span>{item.provider || item.model || '-'}</span>
                  <code>{item.trace_id || '-'}</code>
                </div>
              ))}
            </div>
          ) : (
            <p className="admin-runtime-empty">最近没有失败的 AI 调用记录。</p>
          )}
        </article>

        <article className="admin-runtime-panel-card">
          <div className="admin-runtime-panel-card__head">
            <div>
              <h3>最近抓取任务</h3>
              <p>基于当前任务表先统一查看抓取、导入和流水线生成任务。</p>
            </div>
            <Link className="admin-link" to="/runtime">
              查看全部任务
            </Link>
          </div>
          {recentTasks.length > 0 ? (
            <div className="admin-runtime-table">
              <div className="admin-runtime-table__row admin-runtime-table__row--head">
                <span>时间</span>
                <span>类型</span>
                <span>来源</span>
                <span>状态</span>
                <span>标题</span>
              </div>
              {recentTasks.map((item) => (
                <div className="admin-runtime-table__row" key={item.id}>
                  <span>{formatRuntimeDateTime(item.created_at)}</span>
                  <span>{buildScraperTaskTypeLabel(item.task_type)}</span>
                  <span>{item.source || '-'}</span>
                  <span>{item.status || '-'}</span>
                  <span>{item.source_title || item.source_url || '-'}</span>
                </div>
              ))}
            </div>
          ) : (
            <p className="admin-runtime-empty">当前还没有异步任务记录。</p>
          )}
        </article>

        <article className="admin-runtime-panel-card">
          <div className="admin-runtime-panel-card__head">
            <div>
              <h3>当前排查入口</h3>
              <p>把原本分散的调试入口先集中到后台统一路径。</p>
            </div>
          </div>
          <div className="admin-runtime-actions-grid">
            <Link className="admin-link" to="/question-pipeline">
              题目流水线调试
            </Link>
            <Link className="admin-link" to="/ai-configs">
              AI 配置校验
            </Link>
            <Link className="admin-link" to="/runtime">
              运行日志与任务
            </Link>
          </div>
          <p className="admin-runtime-note">最近 AI 失败数：{failedLogs.filter((item) => !item.is_success).length}</p>
          <p className="admin-runtime-note">最近异步任务数：{recentTasks.length}</p>
          <p className="admin-runtime-note">当前登录管理员：{user?.email || '-'}</p>
          <p className="admin-runtime-note">
            最近失败状态标签：{failedLogs[0] ? buildAICallStatusLabel(failedLogs[0].is_success) : '暂无'}
          </p>
        </article>
      </div>
    </section>
  )
}
