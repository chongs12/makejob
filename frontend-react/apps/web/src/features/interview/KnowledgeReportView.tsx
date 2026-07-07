import { Tag } from 'antd'
import type { KnowledgeReportData } from './interviewTypes'

const THEME = {
  cardBg: '#ffffff',
  border: '#e5e7eb',
  textMain: '#1f2937',
  textSecondary: '#6b7280',
  textMuted: '#9ca3af',
  primary: '#f97316',
  primaryLight: '#fff7ed',
  success: '#16a34a',
  warning: '#d97706',
  danger: '#dc2626',
  radius: 12,
  radiusSm: 8,
  shadow: '0 1px 3px rgba(0,0,0,0.06)',
}

const sectionCardStyle = {
  background: THEME.cardBg,
  borderRadius: THEME.radius,
  border: `1px solid ${THEME.border}`,
  boxShadow: THEME.shadow,
  padding: 20,
  marginBottom: 16,
}

const sectionTitleStyle = {
  fontSize: 16,
  fontWeight: 700,
  color: THEME.textMain,
  margin: '0 0 12px',
}

function scoreColor(score: number): string {
  if (score >= 80) return THEME.success
  if (score >= 60) return THEME.warning
  return THEME.danger
}

function levelColor(level: string): string {
  switch (level) {
    case '完全不会':
      return 'error'
    case '一知半解':
    case '容易混淆':
      return 'warning'
    case '答题不严谨':
      return 'processing'
    default:
      return 'default'
  }
}

function formatDuration(seconds: number): string {
  if (!seconds || seconds <= 0) return '--'
  const minutes = Math.round(seconds / 60)
  return `${minutes} 分钟`
}

/**
 * 知识点专项面试报告视图，渲染 8 大板块。
 * 仅在 report.report_template === 'knowledge' 时使用。
 */
export function KnowledgeReportView({ data }: { data: KnowledgeReportData }) {
  const info = data.basic_info
  return (
    <div>
      {/* 板块1：面试基础信息 */}
      <div style={sectionCardStyle}>
        <h3 style={sectionTitleStyle}>面试基础信息</h3>
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(140px, 1fr))', gap: 12 }}>
          <MetricCard label="考核知识点" value={info?.knowledge_topics?.join('、') || '--'} />
          <MetricCard label="面试题型" value={info?.question_type || '--'} />
          <MetricCard label="答题时长" value={formatDuration(info?.duration_seconds || 0)} />
          <MetricCard label="总题数" value={String(info?.total_questions ?? '--')} />
          <MetricCard label="正确率" value={info?.accuracy > 0 ? `${Math.round(info.accuracy * 100)}%` : '--'} />
          <MetricCard label="整体得分" value={String(data.overall_score ?? '--')} valueColor={scoreColor(data.overall_score)} />
          <MetricCard label="评级" value={data.rating || '--'} valueColor={scoreColor(data.overall_score)} />
        </div>
      </div>

      {/* 板块2：整体考核结论 */}
      <div style={sectionCardStyle}>
        <h3 style={sectionTitleStyle}>整体考核结论</h3>
        <p style={{ fontSize: 14, color: THEME.textMain, lineHeight: 1.7, margin: 0 }}>{data.conclusion || '暂无结论'}</p>
      </div>

      {/* 板块3：逐题答题复盘 */}
      <div style={sectionCardStyle}>
        <h3 style={sectionTitleStyle}>逐题答题复盘</h3>
        {data.question_reviews?.length ? (
          <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
            {data.question_reviews.map((r, i) => (
              <div key={i} style={{ padding: 14, borderRadius: THEME.radiusSm, background: '#fafaf9', border: `1px solid ${THEME.border}` }}>
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 8 }}>
                  <span style={{ fontSize: 13, fontWeight: 600, color: THEME.textMain }}>第 {(r.question_index ?? i) + 1} 题</span>
                  <span style={{ fontSize: 13, fontWeight: 700, color: scoreColor(r.score) }}>{r.score}/{r.max_score || 100}</span>
                </div>
                <ReviewLine label="题目" value={r.question} />
                <ReviewLine label="用户作答" value={r.user_answer || '（未作答）'} />
                {r.errors?.length ? <ReviewList label="错误点" items={r.errors} color={THEME.danger} /> : null}
                {r.omissions?.length ? <ReviewList label="遗漏点" items={r.omissions} color={THEME.warning} /> : null}
                {r.highlights?.length ? <ReviewList label="亮点" items={r.highlights} color={THEME.success} /> : null}
                <ReviewLine label="标准答案" value={r.standard_answer} />
                {r.key_points?.length ? <ReviewList label="核心得分点" items={r.key_points} color={THEME.primary} /> : null}
              </div>
            ))}
          </div>
        ) : (
          <p style={{ fontSize: 13, color: THEME.textMuted }}>暂无逐题复盘</p>
        )}
      </div>

      {/* 板块4：知识点能力四维评分 */}
      <div style={sectionCardStyle}>
        <h3 style={sectionTitleStyle}>知识点能力四维评分</h3>
        {data.dimension_scores?.length ? (
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(220px, 1fr))', gap: 12 }}>
            {data.dimension_scores.map((d, i) => (
              <div key={i} style={{ padding: 14, borderRadius: THEME.radiusSm, background: '#fafaf9', border: `1px solid ${THEME.border}` }}>
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 6 }}>
                  <span style={{ fontSize: 13, fontWeight: 600, color: THEME.textMain }}>{d.dimension}</span>
                  <span style={{ fontSize: 18, fontWeight: 800, color: scoreColor(d.score) }}>{d.score}</span>
                </div>
                <p style={{ fontSize: 12, color: THEME.textSecondary, margin: 0, lineHeight: 1.5 }}>{d.comment || '暂无评语'}</p>
              </div>
            ))}
          </div>
        ) : (
          <p style={{ fontSize: 13, color: THEME.textMuted }}>暂无维度评分</p>
        )}
      </div>

      {/* 板块5：已掌握知识点清单 */}
      <div style={sectionCardStyle}>
        <h3 style={sectionTitleStyle}>已掌握知识点</h3>
        {data.mastered_points?.length ? (
          <div style={{ display: 'flex', flexWrap: 'wrap', gap: 8 }}>
            {data.mastered_points.map((p, i) => (
              <Tag key={i} color="success">{p}</Tag>
            ))}
          </div>
        ) : (
          <p style={{ fontSize: 13, color: THEME.textMuted }}>暂无明显掌握项</p>
        )}
      </div>

      {/* 板块6：知识盲区 & 薄弱点 */}
      <div style={sectionCardStyle}>
        <h3 style={sectionTitleStyle}>知识盲区 & 薄弱点</h3>
        {data.blind_spots?.length ? (
          <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
            {data.blind_spots.map((b, i) => (
              <div key={i} style={{ padding: 12, borderRadius: THEME.radiusSm, background: '#fafaf9', border: `1px solid ${THEME.border}` }}>
                <div style={{ display: 'flex', gap: 8, alignItems: 'center', marginBottom: 4 }}>
                  <Tag color={levelColor(b.level)}>{b.level}</Tag>
                  <span style={{ fontSize: 13, fontWeight: 600, color: THEME.textMain }}>{b.topic}</span>
                </div>
                {b.detail ? <p style={{ fontSize: 12, color: THEME.textSecondary, margin: 0 }}>{b.detail}</p> : null}
              </div>
            ))}
          </div>
        ) : (
          <p style={{ fontSize: 13, color: THEME.textMuted }}>暂无明显盲区</p>
        )}
      </div>

      {/* 板块7：针对性补强学习建议 */}
      <div style={sectionCardStyle}>
        <h3 style={sectionTitleStyle}>针对性补强学习建议</h3>
        {data.study_suggestions?.length ? (
          <ul style={{ margin: 0, paddingLeft: 18 }}>
            {data.study_suggestions.map((s, i) => (
              <li key={i} style={{ fontSize: 13, color: THEME.textMain, marginBottom: 8, lineHeight: 1.6 }}>
                <strong style={{ color: THEME.primary }}>{s.focus}：</strong>
                {s.detail}
              </li>
            ))}
          </ul>
        ) : (
          <p style={{ fontSize: 13, color: THEME.textMuted }}>暂无建议</p>
        )}
      </div>

      {/* 板块8：二次考核出题建议 */}
      <div style={sectionCardStyle}>
        <h3 style={sectionTitleStyle}>二次考核出题建议</h3>
        {data.next_quiz_topics?.length ? (
          <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
            {data.next_quiz_topics.map((t, i) => (
              <div key={i} style={{ padding: 10, borderRadius: THEME.radiusSm, background: THEME.primaryLight, border: `1px solid ${THEME.border}` }}>
                <span style={{ fontSize: 13, fontWeight: 600, color: THEME.textMain }}>{t.topic}</span>
                {t.reason ? <span style={{ fontSize: 12, color: THEME.textSecondary }}> — {t.reason}</span> : null}
              </div>
            ))}
          </div>
        ) : (
          <p style={{ fontSize: 13, color: THEME.textMuted }}>暂无建议</p>
        )}
      </div>
    </div>
  )
}

function MetricCard({ label, value, valueColor }: { label: string; value: string; valueColor?: string }) {
  return (
    <div style={{ padding: 12, borderRadius: THEME.radiusSm, background: '#fafaf9', border: `1px solid ${THEME.border}` }}>
      <div style={{ fontSize: 12, color: THEME.textMuted, marginBottom: 4 }}>{label}</div>
      <div style={{ fontSize: 15, fontWeight: 700, color: valueColor || THEME.textMain, wordBreak: 'break-word' }}>{value}</div>
    </div>
  )
}

function ReviewLine({ label, value }: { label: string; value: string }) {
  if (!value) return null
  return (
    <div style={{ marginBottom: 6 }}>
      <span style={{ fontSize: 12, color: THEME.textMuted, marginRight: 6 }}>{label}：</span>
      <span style={{ fontSize: 13, color: THEME.textMain, whiteSpace: 'pre-wrap' }}>{value}</span>
    </div>
  )
}

function ReviewList({ label, items, color }: { label: string; items: string[]; color: string }) {
  return (
    <div style={{ marginBottom: 6 }}>
      <span style={{ fontSize: 12, color: THEME.textMuted, marginRight: 6 }}>{label}：</span>
      {items.map((it, i) => (
        <Tag key={i} style={{ marginBottom: 4, color, borderColor: color }}>{it}</Tag>
      ))}
    </div>
  )
}
