import { buildTaskStatusActions, taskStatusLabel } from './companionHelpers'
import { resolvePracticeQuestionSetTitle } from '../../shared/practiceRoute'
import type {
  CompanionHistoryItem,
  CompanionPlanDetail,
  CompanionPlanTask,
  CompanionSessionSummary,
  CompanionTaskStatus,
} from './companionTypes'

/**
 * 根据当前计划和历史消息提炼出入口页需要展示的最近会话摘要。
 */
export function buildCompanionSessionSummary(
  history: CompanionHistoryItem[],
  plan: CompanionPlanDetail | null,
): CompanionSessionSummary | null {
  const latestAssistantMessage = [...history].reverse().find((item) => item.role === 'assistant')
  const latestUserMessage = [...history].reverse().find((item) => item.role === 'user')

  if (!latestAssistantMessage && !plan) {
    return null
  }

  return {
    updatedAt: Date.now(),
    latestAssistantReply: latestAssistantMessage?.content || '最近还没有新的陪伴回复。',
    latestUserMessage: latestUserMessage?.content || '',
    planTitle: plan?.title || '',
    progress: Math.round(plan?.progress || 0),
  }
}

/**
 * 将时间值格式化为页面可直接展示的中文时间文本。
 */
export function formatCompanionDateTime(value?: string | number): string {
  if (!value) {
    return '--'
  }

  const date = typeof value === 'number' ? new Date(value) : new Date(value)
  if (Number.isNaN(date.getTime())) {
    return String(value)
  }

  return date.toLocaleString('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  })
}

/**
 * 将计划任务里的题单提示规范化为更适合前端展示的中文标题。
 */
function formatCompanionCollectionHint(value?: string): string {
  const normalizedValue = String(value || '').trim()
  if (!normalizedValue) {
    return ''
  }

  return resolvePracticeQuestionSetTitle(normalizedValue)
}

/**
 * 渲染目标任务列表，统一展示状态、说明和可执行动作。
 */
export function GoalList(props: {
  items: CompanionPlanTask[]
  emptyText: string
  onStatusChange?: (task: CompanionPlanTask, status: CompanionTaskStatus) => void
  pendingTaskId?: number | null
  onContinueTask?: (task: CompanionPlanTask) => void
  onPrepareContinueTask?: () => void
}) {
  if (props.items.length === 0) {
    return <p className="companion-empty-text">{props.emptyText}</p>
  }

  return (
    <div className="companion-goal-list">
      {props.items.map((item) => (
        <article className="companion-goal-item" key={item.id}>
          <div className="companion-goal-head">
            <strong>{item.title}</strong>
            <span>{taskStatusLabel(item.status)}</span>
          </div>
          <p>{item.description || '当前任务暂无详细说明。'}</p>
          {item.source_label ? <p>来源：{item.source_label}</p> : null}
          {item.reason ? <p>原因：{item.reason}</p> : null}
          {item.priority_explanation ? <p>优先级说明：{item.priority_explanation}</p> : null}
          {item.collection_hint ? <p>建议题单：{formatCompanionCollectionHint(item.collection_hint)}</p> : null}
          {item.source_ref ? <p>来源引用：{item.source_ref}</p> : null}
          <div className="companion-goal-meta">
            <span>类型：{item.task_type || 'study'}</span>
            <span>Day {item.day_number || 1}</span>
          </div>
          {props.onContinueTask ? (
            <div className="card-inline">
              <button
                className="ghost-button"
                type="button"
                onMouseEnter={props.onPrepareContinueTask}
                onFocus={props.onPrepareContinueTask}
                onClick={() => props.onContinueTask?.(item)}
              >
                围绕这项继续
              </button>
            </div>
          ) : null}
          {props.onStatusChange ? (
            <div className="companion-task-actions">
              {buildTaskStatusActions(item.status).map((action) => (
                <button
                  className="secondary-button companion-task-button"
                  key={`${item.id}-${action.status}`}
                  type="button"
                  disabled={props.pendingTaskId === item.id}
                  onClick={() => props.onStatusChange?.(item, action.status)}
                >
                  {props.pendingTaskId === item.id ? '提交中...' : action.label}
                </button>
              ))}
            </div>
          ) : null}
        </article>
      ))}
    </div>
  )
}
