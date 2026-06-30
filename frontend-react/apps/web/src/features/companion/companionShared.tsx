import { useState } from 'react'
import { Link } from '@tanstack/react-router'
import { buildTaskStatusActions, taskStatusLabel } from './companionHelpers'
import { buildPracticeRouteSearch, resolvePracticeQuestionSetTitle } from '../../shared/practiceRoute'
import type {
  CompanionFeedbackDifficulty,
  CompanionHistoryItem,
  CompanionPhaseBlueprintSummaryEntry,
  CompanionPlanDetail,
  CompanionPlanTask,
  CompanionSessionSummary,
  CompanionTaskFeedbackDraft,
  CompanionTaskStatus,
} from './companionTypes'

const THEME = {
  bg: '#f8f9fa',
  cardBg: '#ffffff',
  primary: '#f97316',
  primaryLight: '#fff7ed',
  primaryDark: '#ea580c',
  textMain: '#1f2937',
  textSecondary: '#6b7280',
  textMuted: '#9ca3af',
  border: '#f3f4f6',
  borderHover: '#e5e7eb',
  shadow: '0 1px 2px rgba(0,0,0,0.05)',
  shadowCard: '0 4px 6px -1px rgba(0,0,0,0.07), 0 2px 4px -2px rgba(0,0,0,0.05)',
  shadowHover: '0 10px 15px -3px rgba(0,0,0,0.08), 0 4px 6px -4px rgba(0,0,0,0.05)',
  radius: 12,
  radiusSm: 8,
  success: '#22c55e',
  warning: '#f59e0b',
  danger: '#ef4444',
}

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
 * 将学习阶段枚举转换成陪伴页更适合展示的中文标签。
 */
export function formatCompanionPhaseLabel(value?: string): string {
  switch (String(value || '').trim()) {
    case 'drill':
      return '专项突破'
    case 'review':
      return '复盘纠偏'
    case 'mock':
      return '模拟验证'
    case 'foundation':
      return '打基础'
    default:
      return '阶段未标注'
  }
}

/**
 * 将反馈训练类型转换成前端表单里可直接显示的中文标签。
 */
function companionFeedbackTrainingTypeLabel(value: string): string {
  const labelMap: Record<string, string> = {
    coding: '编程题',
    choice: '选择题',
    short_answer: '简答题',
    generic: '通用任务',
  }
  return labelMap[value] || '通用任务'
}

/**
 * 将反馈难度枚举转换成前端表单里可直接显示的中文标签。
 */
function companionFeedbackDifficultyLabel(value: CompanionFeedbackDifficulty): string {
  const labelMap: Record<CompanionFeedbackDifficulty, string> = {
    '': '暂不填写',
    too_easy: '太简单',
    just_right: '刚好',
    too_hard: '太难',
  }
  return labelMap[value]
}

/**
 * 渲染任务完成后的反馈表单，保证陪伴页能在前端采集真实训练信号。
 */
export function CompanionTaskFeedbackPanel(props: {
  task: CompanionPlanTask | null
  draft: CompanionTaskFeedbackDraft
  pending: boolean
  message: string
  onChange: (draft: CompanionTaskFeedbackDraft) => void
  onSubmit: () => void
  onCancel: () => void
}) {
  if (!props.task) {
    return null
  }

  const labelStyle: React.CSSProperties = {
    display: 'flex',
    flexDirection: 'column',
    gap: 4,
    fontSize: 13,
    color: THEME.textSecondary,
  }

  const inputStyle: React.CSSProperties = {
    padding: '8px 12px',
    borderRadius: THEME.radiusSm,
    border: `1px solid ${THEME.border}`,
    fontSize: 14,
    color: THEME.textMain,
    outline: 'none',
    background: '#fff',
  }

  return (
    <article style={{
      background: THEME.cardBg,
      borderRadius: THEME.radius,
      border: `1px solid ${THEME.border}`,
      boxShadow: THEME.shadowCard,
      padding: '24px',
    }}>
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 16 }}>
        <div>
          <span style={{ fontSize: 12, fontWeight: 600, color: THEME.primary, textTransform: 'uppercase' }}>任务完成反馈</span>
          <h2 style={{ fontSize: 16, fontWeight: 700, color: THEME.textMain, margin: '4px 0 0' }}>{props.task.title}</h2>
        </div>
      </div>
      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 16, marginBottom: 16 }}>
        <label style={labelStyle}>
          <span>训练类型</span>
          <select
            style={inputStyle}
            value={props.draft.trainingType}
            onChange={(event) => props.onChange({
              ...props.draft,
              trainingType: event.target.value as CompanionTaskFeedbackDraft['trainingType'],
            })}
          >
            {(['generic', 'coding', 'choice', 'short_answer'] as const).map((item) => (
              <option key={item} value={item}>{companionFeedbackTrainingTypeLabel(item)}</option>
            ))}
          </select>
        </label>
        <label style={labelStyle}>
          <span>尝试次数</span>
          <input
            style={inputStyle}
            min={0}
            type="number"
            value={props.draft.attemptCount}
            onChange={(event) => props.onChange({
              ...props.draft,
              attemptCount: Math.max(0, Number(event.target.value || 0)),
            })}
          />
        </label>
        <label style={labelStyle}>
          <span>耗时（分钟）</span>
          <input
            style={inputStyle}
            min={0}
            type="number"
            value={props.draft.timeSpentMinutes}
            onChange={(event) => props.onChange({
              ...props.draft,
              timeSpentMinutes: Math.max(0, Number(event.target.value || 0)),
            })}
          />
        </label>
        <label style={labelStyle}>
          <span>难度感受</span>
          <select
            style={inputStyle}
            value={props.draft.difficultySelfAssessment}
            onChange={(event) => props.onChange({
              ...props.draft,
              difficultySelfAssessment: event.target.value as CompanionFeedbackDifficulty,
            })}
          >
            {(['', 'too_easy', 'just_right', 'too_hard'] as const).map((item) => (
              <option key={item} value={item}>{companionFeedbackDifficultyLabel(item)}</option>
            ))}
          </select>
        </label>
      </div>
      <div style={{ display: 'flex', flexDirection: 'column', gap: 16, marginBottom: 16 }}>
        <label style={labelStyle}>
          <span>错因标签</span>
          <input
            style={inputStyle}
            type="text"
            value={props.draft.mistakeTagsText}
            onChange={(event) => props.onChange({
              ...props.draft,
              mistakeTagsText: event.target.value,
            })}
            placeholder="例如：边界处理、状态定义、并发顺序"
          />
        </label>
        <label style={labelStyle}>
          <span>补充说明</span>
          <textarea
            style={{ ...inputStyle, minHeight: 80, resize: 'vertical' }}
            value={props.draft.summary}
            onChange={(event) => props.onChange({
              ...props.draft,
              summary: event.target.value,
            })}
            placeholder="可以写下卡住点、错法、是否已经总结出下一步改法。"
          />
        </label>
      </div>
      {props.message ? <p style={{ fontSize: 13, color: THEME.textMuted, margin: '0 0 16px' }}>{props.message}</p> : null}
      <div style={{ display: 'flex', gap: 12, alignItems: 'center', flexWrap: 'wrap' }}>
        <button
          style={{
            padding: '10px 24px',
            borderRadius: THEME.radiusSm,
            border: 'none',
            background: props.pending ? THEME.textMuted : THEME.primary,
            color: '#fff',
            fontSize: 14,
            fontWeight: 600,
            cursor: props.pending ? 'not-allowed' : 'pointer',
          }}
          type="button"
          disabled={props.pending}
          onClick={props.onSubmit}
        >
          {props.pending ? '记录中...' : '记录反馈并完成任务'}
        </button>
        <button
          style={{
            padding: '10px 24px',
            borderRadius: THEME.radiusSm,
            border: `1px solid ${THEME.border}`,
            background: 'transparent',
            color: THEME.textSecondary,
            fontSize: 14,
            cursor: props.pending ? 'not-allowed' : 'pointer',
          }}
          type="button"
          disabled={props.pending}
          onClick={props.onCancel}
        >
          暂不填写
        </button>
        {props.task.collection_hint ? (
          <Link
            style={{ fontSize: 14, color: THEME.primary, textDecoration: 'none' }}
            to="/practice"
            search={buildPracticeRouteSearch({
              questionSetSlug: props.task.collection_hint,
              source: 'practice_recommendation',
              title: props.task.title,
              reason: props.task.reason,
            })}
          >
            先去刷建议题单
          </Link>
        ) : null}
      </div>
    </article>
  )
}

/**
 * 渲染目标任务列表，统一展示状态、说明和可执行动作。
 * 默认只显示任务名称和状态，点击展开查看详情。
 */
export function GoalList(props: {
  items: CompanionPlanTask[]
  emptyText: string
  onStatusChange?: (task: CompanionPlanTask, status: CompanionTaskStatus) => void
  pendingTaskId?: number | null
  onContinueTask?: (task: CompanionPlanTask) => void
  onPrepareContinueTask?: () => void
}) {
  const [expandedId, setExpandedId] = useState<number | null>(null)

  if (props.items.length === 0) {
    return <p style={{ fontSize: 14, color: THEME.textMuted, textAlign: 'center', padding: '24px 0' }}>{props.emptyText}</p>
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
      {props.items.map((item) => {
        const isExpanded = expandedId === item.id

        return (
          <article
            key={item.id}
            style={{
              background: THEME.cardBg,
              borderRadius: THEME.radius,
              border: `1px solid ${isExpanded ? THEME.borderHover : THEME.border}`,
              boxShadow: isExpanded ? THEME.shadowCard : THEME.shadow,
              padding: '16px 20px',
              transition: 'border-color 0.2s, box-shadow 0.2s',
            }}
          >
            <div
              style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', cursor: 'pointer' }}
              onClick={() => setExpandedId(isExpanded ? null : item.id)}
            >
              <strong style={{ fontSize: 15, color: THEME.textMain }}>{item.title}</strong>
              <span style={{
                fontSize: 12,
                fontWeight: 600,
                color: item.status === 'completed' ? THEME.success : item.status === 'in_progress' ? THEME.primary : THEME.textMuted,
                background: item.status === 'completed' ? '#f0fdf4' : item.status === 'in_progress' ? THEME.primaryLight : THEME.bg,
                padding: '2px 10px',
                borderRadius: 999,
              }}>
                {taskStatusLabel(item.status)}
              </span>
            </div>

            {isExpanded && (
              <div style={{ marginTop: 12, paddingTop: 12, borderTop: `1px solid ${THEME.border}` }}>
                <p style={{ fontSize: 14, color: THEME.textSecondary, margin: '0 0 8px' }}>
                  {item.description || '当前任务暂无详细说明。'}
                </p>
                {item.phase ? <p style={{ fontSize: 13, color: THEME.textMuted, margin: '0 0 4px' }}>阶段：{formatCompanionPhaseLabel(item.phase)}</p> : null}
                {item.phase_goal ? <p style={{ fontSize: 13, color: THEME.textMuted, margin: '0 0 4px' }}>阶段目标：{item.phase_goal}</p> : null}
                {item.source_label ? <p style={{ fontSize: 13, color: THEME.textMuted, margin: '0 0 4px' }}>来源：{item.source_label}</p> : null}
                {item.reason ? <p style={{ fontSize: 13, color: THEME.textMuted, margin: '0 0 4px' }}>原因：{item.reason}</p> : null}
                {item.priority_explanation ? <p style={{ fontSize: 13, color: THEME.textMuted, margin: '0 0 4px' }}>优先级说明：{item.priority_explanation}</p> : null}
                {item.collection_hint ? <p style={{ fontSize: 13, color: THEME.textMuted, margin: '0 0 4px' }}>建议题单：{formatCompanionCollectionHint(item.collection_hint)}</p> : null}
                {item.source_ref ? <p style={{ fontSize: 13, color: THEME.textMuted, margin: '0 0 4px' }}>来源引用：{item.source_ref}</p> : null}
                <div style={{ display: 'flex', gap: 16, fontSize: 12, color: THEME.textMuted, margin: '8px 0' }}>
                  <span>类型：{item.task_type || 'study'}</span>
                  <span>Day {item.day_number || 1}</span>
                </div>
                {item.collection_hint ? (
                  <div style={{ marginTop: 8 }}>
                    <Link
                      style={{ fontSize: 14, color: THEME.primary, textDecoration: 'none' }}
                      to="/practice"
                      search={buildPracticeRouteSearch({
                        questionSetSlug: item.collection_hint,
                        source: 'practice_recommendation',
                        title: item.title,
                        reason: item.reason,
                      })}
                    >
                      按建议题单去补练
                    </Link>
                  </div>
                ) : null}
                {props.onContinueTask ? (
                  <div style={{ marginTop: 8 }}>
                    <button
                      style={{
                        padding: '8px 16px',
                        borderRadius: THEME.radiusSm,
                        border: `1px solid ${THEME.border}`,
                        background: 'transparent',
                        color: THEME.textSecondary,
                        fontSize: 13,
                        cursor: 'pointer',
                      }}
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
                  <div style={{ display: 'flex', gap: 8, marginTop: 12, flexWrap: 'wrap' }}>
                    {buildTaskStatusActions(item.status).map((action) => (
                      <button
                        style={{
                          padding: '6px 14px',
                          borderRadius: THEME.radiusSm,
                          border: `1px solid ${THEME.border}`,
                          background: 'transparent',
                          color: THEME.textSecondary,
                          fontSize: 13,
                          cursor: props.pendingTaskId === item.id ? 'not-allowed' : 'pointer',
                          opacity: props.pendingTaskId === item.id ? 0.6 : 1,
                        }}
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
              </div>
            )}
          </article>
        )
      })}
    </div>
  )
}

/**
 * 渲染阶段时间线，展示计划各阶段的天数分布和当前进度。
 * compact 模式适用于侧边栏等紧凑空间。
 */
export function PhaseTimeline(props: {
  phases: CompanionPhaseBlueprintSummaryEntry[]
  currentPhase?: string
  compact?: boolean
}) {
  if (props.phases.length === 0) {
    return null
  }

  if (props.compact) {
    return (
      <div className="companion-phase-timeline-compact">
        {props.phases.map((item) => (
          <span
            className={`companion-phase-timeline-compact-pill${item.phase === props.currentPhase ? ' companion-phase-timeline-compact-active' : ''}`}
            key={item.phase}
          >
            {formatCompanionPhaseLabel(item.phase)}
            <span className="companion-phase-timeline-compact-days">
              Day {item.start_day}-{item.end_day}
            </span>
          </span>
        ))}
      </div>
    )
  }

  const activePhase = props.phases.find((item) => item.phase === props.currentPhase)

  return (
    <div className="companion-phase-timeline">
      <div className="companion-phase-timeline-bar">
        {props.phases.map((item) => (
          <div
            className={`companion-phase-timeline-segment${item.phase === props.currentPhase ? ' companion-phase-timeline-active' : ''}`}
            key={item.phase}
          >
            <strong>{formatCompanionPhaseLabel(item.phase)}</strong>
            <span>Day {item.start_day} - Day {item.end_day}</span>
          </div>
        ))}
      </div>
      {activePhase?.phase_goal ? (
        <p className="companion-phase-timeline-goal">{activePhase.phase_goal}</p>
      ) : null}
    </div>
  )
}

/**
 * 将阶段调整原因码转换成前端可直接展示的中文标签。
 * 集中维护映射，页面不得各自写私有 if-else。
 */
export function formatCompanionReasonCodeLabel(code: string): string {
  const labelMap: Record<string, string> = {
    mock_not_stable: '模拟验证不稳定',
    weakness_unresolved: '弱项未解决',
    partial_mastery: '部分掌握',
    review_completed: '复盘完成',
    progress_verified: '进度验证通过',
  }
  return labelMap[String(code || '').trim()] || '阶段调整'
}

/**
 * 渲染阶段调整原因码标签列表，以 pill 形式展示"为什么进入/回到某个阶段"。
 */
export function ReasonCodeTags(props: {
  codes: string[]
}) {
  if (props.codes.length === 0) {
    return null
  }

  return (
    <div className="companion-topic-pills">
      {props.codes.map((code) => (
        <span className="companion-topic-pill" key={code}>
          {formatCompanionReasonCodeLabel(code)}
        </span>
      ))}
    </div>
  )
}

/**
 * 渲染计划的阶段相关信息：入口阶段、原因码标签、调整摘要、阶段时间线。
 * 统一 Hub 页和 Workspace 页的展示逻辑，消除重复代码。
 */
export function CompanionPlanPhaseSection(props: {
  plan: CompanionPlanDetail
  compact?: boolean
}) {
  const { plan, compact } = props

  return (
    <>
      {plan.entry_phase ? <p className="companion-empty-text">本轮入口阶段：{formatCompanionPhaseLabel(plan.entry_phase)}</p> : null}
      {plan.adjustment_reason_codes?.length ? (
        <ReasonCodeTags codes={plan.adjustment_reason_codes} />
      ) : null}
      {plan.adjustment_summaries?.slice(0, 3).map((item, index) => (
        <p className="companion-empty-text" key={`adjustment-summary-${index}`}>
          本轮调整依据{plan.adjustment_summaries && plan.adjustment_summaries.length > 1 ? ` ${index + 1}` : ''}：{item}
        </p>
      ))}
      {plan.phase_blueprint_summary?.length ? (
        <PhaseTimeline
          phases={plan.phase_blueprint_summary}
          currentPhase={plan.phase}
          compact={compact}
        />
      ) : null}
    </>
  )
}
