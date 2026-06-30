import { Progress, Tag } from 'antd'
import {
  CalendarOutlined,
  CheckCircleOutlined,
  ClockCircleOutlined,
  FireOutlined,
  RocketOutlined,
} from '@ant-design/icons'
import type { CompanionPlanDetail, CompanionPlanTask } from './companionTypes'
import { formatCompanionDateTime, formatCompanionPhaseLabel } from './companionShared'
import type { WeeklyFocusTheme } from '../../shared/weeklyFocus'
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
  radius: 12,
  success: '#22c55e',
}

interface PlanStats {
  completed_tasks: number
  total_tasks: number
  progress: number
}

interface PlanOverviewProps {
  plan: CompanionPlanDetail | null
  planStats: PlanStats | null
  weeklyFocusThemes: WeeklyFocusTheme[]
  latestCompletedTask: CompanionPlanTask | null
  isLoading: boolean
}

/**
 * 计划概览，显示计划信息、进度和本周补强主题。
 */
export function PlanOverview({
  plan,
  planStats,
  weeklyFocusThemes,
  latestCompletedTask,
  isLoading,
}: PlanOverviewProps) {
  const cardStyle = {
    background: THEME.cardBg,
    borderRadius: THEME.radius,
    border: `1px solid ${THEME.border}`,
    boxShadow: THEME.shadow,
    padding: '20px',
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
      {/* 计划信息卡片 */}
      <div style={cardStyle}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 16 }}>
          <RocketOutlined style={{ color: THEME.primary, fontSize: 16 }} />
          <span style={{ fontSize: 12, fontWeight: 600, color: THEME.textMuted, textTransform: 'uppercase' }}>
            当前计划
          </span>
        </div>

        {plan ? (
          <>
            <h3 style={{ fontSize: 16, fontWeight: 700, color: THEME.textMain, margin: '0 0 8px' }}>
              {plan.title}
            </h3>

            {plan.phase && (
              <p style={{ fontSize: 13, color: THEME.textSecondary, margin: '0 0 16px' }}>
                {formatCompanionPhaseLabel(plan.phase)}
              </p>
            )}

            <Progress
              percent={planStats?.progress ?? plan.progress ?? 0}
              strokeColor={THEME.primary}
              trailColor="#f3f4f6"
              style={{ marginBottom: 16 }}
            />

            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 12 }}>
              <div style={{ textAlign: 'center' }}>
                <div style={{ fontSize: 24, fontWeight: 700, color: THEME.success }}>
                  {planStats?.completed_tasks ?? plan.completed_tasks ?? 0}
                </div>
                <div style={{ fontSize: 12, color: THEME.textMuted }}>已完成</div>
              </div>
              <div style={{ textAlign: 'center' }}>
                <div style={{ fontSize: 24, fontWeight: 700, color: THEME.textMain }}>
                  {planStats?.total_tasks ?? plan.total_tasks ?? 0}
                </div>
                <div style={{ fontSize: 12, color: THEME.textMuted }}>总任务</div>
              </div>
            </div>
          </>
        ) : (
          <p style={{ fontSize: 14, color: THEME.textMuted, margin: 0 }}>
            {isLoading ? '加载中...' : '还没有学习计划'}
          </p>
        )}
      </div>

      {/* 本周补强 */}
      {weeklyFocusThemes.length > 0 && (
        <div style={cardStyle}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 16 }}>
            <FireOutlined style={{ color: THEME.primary, fontSize: 16 }} />
            <span style={{ fontSize: 12, fontWeight: 600, color: THEME.textMuted, textTransform: 'uppercase' }}>
              本周补强
            </span>
          </div>

          <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
            {weeklyFocusThemes.slice(0, 3).map((theme) => (
              <div
                key={theme.title}
                style={{
                  padding: '12px',
                  borderRadius: 8,
                  background: '#fafaf9',
                  border: `1px solid ${THEME.border}`,
                }}
              >
                <div style={{ fontSize: 14, fontWeight: 600, color: THEME.textMain, marginBottom: 4 }}>
                  {theme.title}
                </div>
                <div style={{ fontSize: 12, color: THEME.textSecondary }}>
                  {theme.reason}
                </div>
                {theme.focus_tags.length > 0 && (
                  <div style={{ display: 'flex', gap: 6, marginTop: 8, flexWrap: 'wrap' }}>
                    {theme.focus_tags.slice(0, 3).map((tag) => (
                      <Tag key={tag} style={{ margin: 0, fontSize: 11 }}>{tag}</Tag>
                    ))}
                  </div>
                )}
              </div>
            ))}
          </div>
        </div>
      )}

      {/* 最近完成 */}
      <div style={cardStyle}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 16 }}>
          <CheckCircleOutlined style={{ color: THEME.success, fontSize: 16 }} />
          <span style={{ fontSize: 12, fontWeight: 600, color: THEME.textMuted, textTransform: 'uppercase' }}>
            最近完成
          </span>
        </div>

        {latestCompletedTask ? (
          <div>
            <div style={{ fontSize: 14, fontWeight: 600, color: THEME.textMain, marginBottom: 4 }}>
              {latestCompletedTask.title}
            </div>
            <div style={{ fontSize: 12, color: THEME.textMuted, display: 'flex', alignItems: 'center', gap: 6 }}>
              <ClockCircleOutlined />
              {formatCompanionDateTime(latestCompletedTask.completed_at)}
            </div>
          </div>
        ) : (
          <p style={{ fontSize: 14, color: THEME.textMuted, margin: 0 }}>
            还没有完成记录
          </p>
        )}
      </div>
    </div>
  )
}
