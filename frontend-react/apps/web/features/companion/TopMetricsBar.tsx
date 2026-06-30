import { FireOutlined, BarChartOutlined } from '@ant-design/icons'

const THEME = {
  bg: '#f8f9fa',
  cardBg: '#ffffff',
  primary: '#f97316',
  primaryLight: '#fff7ed',
  textMain: '#1f2937',
  textSecondary: '#6b7280',
  textMuted: '#9ca3af',
  border: '#f3f4f6',
  shadow: '0 1px 2px rgba(0,0,0,0.05)',
  radius: 12,
}

interface TopMetricsBarProps {
  streakDays: number
  planProgress: number
}

/**
 * 顶部指标栏，显示连续天数和计划进度。
 */
export function TopMetricsBar({ streakDays, planProgress }: TopMetricsBarProps) {
  return (
    <div style={{
      position: 'sticky',
      top: 56,
      zIndex: 40,
      background: THEME.cardBg,
      borderBottom: `1px solid ${THEME.border}`,
      boxShadow: THEME.shadow,
    }}>
      <div style={{
        maxWidth: 1200,
        margin: '0 auto',
        padding: '12px 24px',
        display: 'flex',
        alignItems: 'center',
        gap: 24,
      }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          <FireOutlined style={{ color: THEME.primary, fontSize: 18 }} />
          <span style={{ fontSize: 15, fontWeight: 600, color: THEME.textMain }}>
            连续 {streakDays} 天
          </span>
        </div>

        <div style={{ width: 1, height: 24, background: THEME.border }} />

        <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          <BarChartOutlined style={{ color: THEME.textMuted, fontSize: 16 }} />
          <span style={{ fontSize: 14, color: THEME.textSecondary }}>
            计划进度 <strong style={{ color: THEME.textMain }}>{planProgress}%</strong>
          </span>
        </div>
      </div>
    </div>
  )
}
