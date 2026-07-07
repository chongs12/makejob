import { Tag } from 'antd'
import type { JobReportData } from './interviewTypes'

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

function hireColor(rec: string): string {
  switch (rec) {
    case '建议录用':
      return THEME.success
    case '建议复试考察':
      return THEME.warning
    case '人才储备':
      return THEME.primary
    case '不予录用':
      return THEME.danger
    default:
      return THEME.textSecondary
  }
}

function riskColor(level: string): string {
  switch (level) {
    case '致命':
      return 'error'
    case '轻微':
      return 'warning'
    default:
      return 'default'
  }
}

function formatDuration(seconds: number): string {
  if (!seconds || seconds <= 0) return '--'
  return `${Math.round(seconds / 60)} 分钟`
}

function formatWeight(w: number): string {
  if (!w) return '--'
  return `${Math.round(w * 100)}%`
}

/**
 * 岗位求职面试报告视图，渲染 9 大板块。
 * 仅在 report.report_template === 'job' 时使用。
 */
export function JobReportView({ data }: { data: JobReportData }) {
  const info = data.basic_info
  return (
    <div>
      {/* 板块1：面试档案基础信息 */}
      <div style={sectionCardStyle}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 12 }}>
          <h3 style={{ ...sectionTitleStyle, margin: 0 }}>面试档案</h3>
          {data.hire_recommendation ? (
            <Tag style={{ fontSize: 13, fontWeight: 700, padding: '4px 12px', color: hireColor(data.hire_recommendation), borderColor: hireColor(data.hire_recommendation) }}>
              {data.hire_recommendation}
            </Tag>
          ) : null}
        </div>
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(140px, 1fr))', gap: 12 }}>
          <MetricCard label="候选人" value={info?.candidate_name || '候选人'} />
          <MetricCard label="应聘岗位" value={info?.target_position || '--'} />
          <MetricCard label="面试类型" value={info?.interview_type || '--'} />
          <MetricCard label="面试时长" value={formatDuration(info?.duration_seconds || 0)} />
          <MetricCard label="总题数" value={String(info?.total_questions ?? '--')} />
          <MetricCard label="综合评分" value={String(data.overall_score ?? '--')} valueColor={scoreColor(data.overall_score)} />
          <MetricCard label="评级" value={data.rating || '--'} valueColor={scoreColor(data.overall_score)} />
        </div>
      </div>

      {/* 板块2：简历&JD 匹配总览 */}
      <div style={sectionCardStyle}>
        <h3 style={sectionTitleStyle}>简历 & 岗位 JD 匹配总览</h3>
        {data.jd_match_overview ? (
          <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
            <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
              <Tag color={data.jd_match_overview.hard_requirements_met ? 'success' : 'error'}>
                硬性条件{data.jd_match_overview.hard_requirements_met ? '达标' : '未达标'}
              </Tag>
            </div>
            <TagGroup label="核心匹配项" items={data.jd_match_overview.matched_items} color={THEME.success} />
            <TagGroup label="核心缺失项" items={data.jd_match_overview.missing_items} color={THEME.danger} />
            <TagGroup label="简历优势" items={data.jd_match_overview.resume_highlights} color={THEME.primary} />
            <TagGroup label="简历硬伤" items={data.jd_match_overview.resume_hard_wounds} color={THEME.warning} />
          </div>
        ) : (
          <p style={{ fontSize: 13, color: THEME.textMuted }}>暂无匹配分析</p>
        )}
      </div>

      {/* 板块3：全面试问答复盘 */}
      <div style={sectionCardStyle}>
        <h3 style={sectionTitleStyle}>全面试问答复盘</h3>
        {data.question_reviews?.length ? (
          <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
            {data.question_reviews.map((r, i) => (
              <div key={i} style={{ padding: 14, borderRadius: THEME.radiusSm, background: '#fafaf9', border: `1px solid ${THEME.border}` }}>
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 8 }}>
                  <span style={{ fontSize: 13, fontWeight: 600, color: THEME.textMain }}>第 {(r.question_index ?? i) + 1} 题</span>
                  <span style={{ fontSize: 13, fontWeight: 700, color: scoreColor(r.score) }}>{r.score}/{r.max_score || 100}</span>
                </div>
                <ReviewLine label="问题" value={r.question} />
                <ReviewLine label="回答" value={r.user_answer || '（未作答）'} />
                {r.highlights?.length ? <ReviewList label="面试亮点" items={r.highlights} color={THEME.success} /> : null}
                {r.loopholes?.length ? <ReviewList label="回答漏洞" items={r.loopholes} color={THEME.danger} /> : null}
                {r.pitfalls?.length ? <ReviewList label="踩坑点" items={r.pitfalls} color={THEME.warning} /> : null}
                {r.taboos?.length ? <ReviewList label="职场禁忌" items={r.taboos} color={THEME.danger} /> : null}
              </div>
            ))}
          </div>
        ) : (
          <p style={{ fontSize: 13, color: THEME.textMuted }}>暂无问答复盘</p>
        )}
      </div>

      {/* 板块4：六大维度量化评分 */}
      <div style={sectionCardStyle}>
        <h3 style={sectionTitleStyle}>六大维度量化评分</h3>
        {data.dimension_scores?.length ? (
          <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
            {data.dimension_scores.map((d, i) => (
              <div key={i} style={{ padding: 14, borderRadius: THEME.radiusSm, background: '#fafaf9', border: `1px solid ${THEME.border}` }}>
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 6 }}>
                  <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
                    <span style={{ fontSize: 13, fontWeight: 600, color: THEME.textMain }}>{d.dimension}</span>
                    <Tag>{formatWeight(d.weight)}</Tag>
                  </div>
                  <span style={{ fontSize: 18, fontWeight: 800, color: scoreColor(d.score) }}>{d.score}</span>
                </div>
                <p style={{ fontSize: 12, color: THEME.textSecondary, margin: 0, lineHeight: 1.5 }}>{d.comment || '暂无解读'}</p>
              </div>
            ))}
          </div>
        ) : (
          <p style={{ fontSize: 13, color: THEME.textMuted }}>暂无维度评分</p>
        )}
      </div>

      {/* 板块5：核心求职优势 */}
      <div style={sectionCardStyle}>
        <h3 style={sectionTitleStyle}>核心求职优势</h3>
        {data.core_advantages?.length ? (
          <ul style={{ margin: 0, paddingLeft: 18 }}>
            {data.core_advantages.map((a, i) => (
              <li key={i} style={{ fontSize: 13, color: THEME.textMain, marginBottom: 6, lineHeight: 1.6 }}>{a}</li>
            ))}
          </ul>
        ) : (
          <p style={{ fontSize: 13, color: THEME.textMuted }}>暂无明显优势</p>
        )}
      </div>

      {/* 板块6：面试短板 & 求职风险 */}
      <div style={sectionCardStyle}>
        <h3 style={sectionTitleStyle}>面试短板 & 求职风险</h3>
        {data.weaknesses_risks?.length ? (
          <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
            {data.weaknesses_risks.map((w, i) => (
              <div key={i} style={{ padding: 12, borderRadius: THEME.radiusSm, background: '#fafaf9', border: `1px solid ${THEME.border}` }}>
                <div style={{ display: 'flex', gap: 8, alignItems: 'center', marginBottom: 4 }}>
                  <Tag color={riskColor(w.level)}>{w.level}</Tag>
                  <span style={{ fontSize: 13, fontWeight: 600, color: THEME.textMain }}>{w.item}</span>
                </div>
                {w.impact ? <p style={{ fontSize: 12, color: THEME.textSecondary, margin: 0 }}>{w.impact}</p> : null}
              </div>
            ))}
          </div>
        ) : (
          <p style={{ fontSize: 13, color: THEME.textMuted }}>暂无明显短板</p>
        )}
      </div>

      {/* 板块7：最终面试决策 */}
      <div style={{ ...sectionCardStyle, background: THEME.primaryLight, border: 'none' }}>
        <h3 style={sectionTitleStyle}>最终面试决策建议</h3>
        {data.hire_decision ? (
          <div>
            <div style={{ fontSize: 18, fontWeight: 800, color: hireColor(data.hire_decision.decision), marginBottom: 8 }}>
              {data.hire_decision.decision || '--'}
            </div>
            <p style={{ fontSize: 13, color: THEME.textMain, margin: 0, lineHeight: 1.6 }}>{data.hire_decision.rationale || '暂无依据'}</p>
          </div>
        ) : (
          <p style={{ fontSize: 13, color: THEME.textMuted }}>暂无决策</p>
        )}
      </div>

      {/* 板块8：针对性面试优化方案 */}
      <div style={sectionCardStyle}>
        <h3 style={sectionTitleStyle}>针对性面试优化方案</h3>
        {data.optimization_plan?.length ? (
          <ul style={{ margin: 0, paddingLeft: 18 }}>
            {data.optimization_plan.map((o, i) => (
              <li key={i} style={{ fontSize: 13, color: THEME.textMain, marginBottom: 8, lineHeight: 1.6 }}>
                <strong style={{ color: THEME.primary }}>{o.aspect}：</strong>{o.detail}
              </li>
            ))}
          </ul>
        ) : (
          <p style={{ fontSize: 13, color: THEME.textMuted }}>暂无优化方案</p>
        )}
      </div>

      {/* 板块9：下一轮复试预测题库 */}
      <div style={sectionCardStyle}>
        <h3 style={sectionTitleStyle}>下一轮复试预测题库</h3>
        {data.next_round_questions?.length ? (
          <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
            {data.next_round_questions.map((q, i) => (
              <div key={i} style={{ padding: 12, borderRadius: THEME.radiusSm, background: '#fafaf9', border: `1px solid ${THEME.border}` }}>
                <div style={{ fontSize: 13, fontWeight: 600, color: THEME.textMain, marginBottom: 4 }}>{q.question}</div>
                <div style={{ display: 'flex', gap: 8 }}>
                  {q.focus ? <Tag>{q.focus}</Tag> : null}
                  {q.difficulty ? <Tag color="warning">{q.difficulty}</Tag> : null}
                </div>
              </div>
            ))}
          </div>
        ) : (
          <p style={{ fontSize: 13, color: THEME.textMuted }}>暂无预测题</p>
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

function TagGroup({ label, items, color }: { label: string; items: string[]; color: string }) {
  if (!items?.length) return null
  return (
    <div>
      <span style={{ fontSize: 12, color: THEME.textMuted, marginRight: 6 }}>{label}：</span>
      {items.map((it, i) => (
        <Tag key={i} style={{ marginBottom: 4, color, borderColor: color }}>{it}</Tag>
      ))}
    </div>
  )
}
