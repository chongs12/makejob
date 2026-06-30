import { Button, Checkbox, Empty, Spin } from 'antd'
import {
  AimOutlined,
  CheckCircleOutlined,
  PlayCircleOutlined,
  ForwardOutlined,
  BookOutlined,
} from '@ant-design/icons'
import type { CompanionPlanTask } from './companionTypes'
import { formatCompanionPhaseLabel } from './companionShared'
import { resolvePracticeQuestionSetTitle } from '../../shared/practiceRoute'

const THEME = {
  cardBg: '#ffffff',
  primary: '#f97316',
  primaryLight: '#fff7ed',
  textMain: '#1f2937',
  textSecondary: '#6b7280',
  textMuted: '#9ca3af',
  border: '#f3f4f6',
  shadow: '0 1px 2px rgba(0,0,0,0.05)',
  shadowHover: '0 10px 15px -3px rgba(0,0,0,0.08)',
  radius: 12,
  success: '#22c55e',
}

interface TodayTaskFlowProps {
  focusedTask: CompanionPlanTask | null
  todayGoals: CompanionPlanTask[]
  isLoading: boolean
  onContinue: (task: CompanionPlanTask) => void
  onComplete: (task: CompanionPlanTask) => void
  onSkip: (task: CompanionPlanTask) => void
}

/**
 * 今日任务流，显示聚焦任务和待办列表。
 */
export function TodayTaskFlow({
  focusedTask,
  todayGoals,
  isLoading,
  onContinue,
  onComplete,
  onSkip,
}: TodayTaskFlowProps) {
  if (isLoading) {
    return (
      <div style={{ textAlign: 'center', padding: '48px 0' }}>
        <Spin />
      </div>
    )
  }

  if (!focusedTask && todayGoals.length === 0) {
    return (
      <div style={{
        background: THEME.cardBg,
        borderRadius: THEME.radius,
        border: `1px solid ${THEME.border}`,
        boxShadow: THEME.shadow,
        padding: '48px 24px',
        textAlign: 'center',
      }}>
        <Empty
          image={<AimOutlined style={{ fontSize: 48, color: THEME.textMuted }} />}
          description="今天没有待推进的任务"
        />
      </div>
    )
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
      {/* 聚焦任务卡片 */}
      {focusedTask && (
        <div style={{
          background: THEME.cardBg,
          borderRadius: THEME.radius,
          border: `1px solid ${THEME.primary}`,
          boxShadow: `0 4px 12px rgba(249, 115, 22, 0.15)`,
          padding: '24px',
        }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 12 }}>
            <AimOutlined style={{ color: THEME.primary, fontSize: 18 }} />
            <span style={{ fontSize: 12, fontWeight: 600, color: THEME.primary, textTransform: 'uppercase' }}>
              当前聚焦
            </span>
          </div>

          <h3 style={{ fontSize: 18, fontWeight: 700, color: THEME.textMain, margin: '0 0 8px' }}>
            {focusedTask.title}
          </h3>

          {focusedTask.phase && (
            <p style={{ fontSize: 13, color: THEME.textSecondary, margin: '0 0 16px' }}>
              {formatCompanionPhaseLabel(focusedTask.phase)}
              {focusedTask.day_number ? ` · Day ${focusedTask.day_number}` : ''}
            </p>
          )}

          {focusedTask.collection_hint && (
            <div style={{
              display: 'inline-flex',
              alignItems: 'center',
              gap: 6,
              padding: '6px 12px',
              borderRadius: 8,
              background: THEME.primaryLight,
              fontSize: 13,
              color: THEME.primary,
              marginBottom: 16,
            }}>
              <BookOutlined />
              {resolvePracticeQuestionSetTitle(focusedTask.collection_hint)}
            </div>
          )}

          <div style={{ display: 'flex', gap: 12 }}>
            <Button
              type="primary"
              icon={<PlayCircleOutlined />}
              onClick={() => onContinue(focusedTask)}
              style={{
                background: THEME.primary,
                borderColor: THEME.primary,
                borderRadius: 8,
                fontWeight: 600,
              }}
            >
              进入陪伴页
            </Button>
            <Button
              icon={<CheckCircleOutlined />}
              onClick={() => onComplete(focusedTask)}
              style={{ borderRadius: 8 }}
            >
              完成
            </Button>
          </div>
        </div>
      )}

      {/* 今日待办列表 */}
      {todayGoals.length > 0 && (
        <div style={{
          background: THEME.cardBg,
          borderRadius: THEME.radius,
          border: `1px solid ${THEME.border}`,
          boxShadow: THEME.shadow,
          overflow: 'hidden',
        }}>
          <div style={{
            padding: '16px 20px',
            borderBottom: `1px solid ${THEME.border}`,
            fontSize: 14,
            fontWeight: 600,
            color: THEME.textMain,
          }}>
            今日待办 ({todayGoals.length})
          </div>

          <div>
            {todayGoals.map((task, index) => (
              <div
                key={task.id}
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  gap: 12,
                  padding: '14px 20px',
                  borderBottom: index < todayGoals.length - 1 ? `1px solid ${THEME.border}` : 'none',
                  transition: 'background 0.15s ease',
                }}
                onMouseEnter={(e) => { e.currentTarget.style.background = '#fafaf9' }}
                onMouseLeave={(e) => { e.currentTarget.style.background = 'transparent' }}
              >
                <Checkbox
                  checked={task.status === 'completed'}
                  onChange={() => onComplete(task)}
                />

                <div style={{ flex: 1, minWidth: 0 }}>
                  <div style={{
                    fontSize: 14,
                    fontWeight: 500,
                    color: task.status === 'completed' ? THEME.textMuted : THEME.textMain,
                    textDecoration: task.status === 'completed' ? 'line-through' : 'none',
                    overflow: 'hidden',
                    textOverflow: 'ellipsis',
                    whiteSpace: 'nowrap',
                  }}>
                    {task.title}
                  </div>
                  {task.phase && (
                    <div style={{ fontSize: 12, color: THEME.textMuted, marginTop: 2 }}>
                      {formatCompanionPhaseLabel(task.phase)}
                    </div>
                  )}
                </div>

                <div style={{ display: 'flex', gap: 8 }}>
                  <Button
                    type="text"
                    size="small"
                    icon={<PlayCircleOutlined />}
                    onClick={() => onContinue(task)}
                    style={{ color: THEME.textMuted }}
                  />
                  <Button
                    type="text"
                    size="small"
                    icon={<ForwardOutlined />}
                    onClick={() => onSkip(task)}
                    style={{ color: THEME.textMuted }}
                  />
                </div>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  )
}
