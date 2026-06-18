import { useMemo, useState } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Link, useNavigate } from '@tanstack/react-router'
import {
  Button,
  Input,
  Select,
  Tag,
  Avatar,
  Divider,
  Empty,
  Spin,
  Tabs,
  Progress,
  Timeline,
  Tooltip,
} from 'antd'
import type { TabsProps } from 'antd'
import {
  BookOutlined,
  CalendarOutlined,
  TrophyOutlined,
  CheckCircleOutlined,
  FireOutlined,
  RiseOutlined,
  StarOutlined,
  TagOutlined,
  ArrowRightOutlined,
  ClockCircleOutlined,
  UserOutlined,
  FileTextOutlined,
  BulbOutlined,
  HeartOutlined,
  LikeOutlined,
  MessageOutlined,
  EditOutlined,
  DeleteOutlined,
  PlusOutlined,
  PlayCircleOutlined,
  BarChartOutlined,
  LineChartOutlined,
  PieChartOutlined,
  AimOutlined,
  RocketOutlined,
  ThunderboltOutlined,
  FlagOutlined,
  AreaChartOutlined,
  ApartmentOutlined,
} from '@ant-design/icons'
import { extractErrorMessage, requestJson } from '@makejob/api-client'
import { isSuccessCode, type ApiEnvelope } from '@makejob/shared-types'
import { useAuthStore } from '../../state/auth'
import { AsyncEmptyState, AsyncInlineState } from '../../shared/asyncState'
import { buildGrowthCompanionContextDraft, persistCompanionPlanContext } from '../../shared/companionContext'
import { DEFAULT_FRONTEND_INDUSTRY_CODE, readSelectedFrontendIndustryCode } from '../../shared/industryContext'
import { requestLoginPrompt } from '../../shared/loginPrompt'
import { fetchMistakeTopics, pickMistakeTopicsByTags, resolveMistakeTopicRoute } from '../../shared/mistakeTopics'
import {
  fetchPracticeRecommendations,
  resolvePracticeRecommendationModeLabel,
  resolvePracticeRecommendationRoute,
  resolvePracticeRecommendationSourceLabel,
} from '../../shared/practiceRecommendations'
import {
  buildPracticeRouteSearch,
  buildPracticeRecommendationRouteSearch,
  buildWeeklyFocusPracticeRouteSearch,
  resolvePracticeQuestionSetTitle,
} from '../../shared/practiceRoute'
import { fetchWeeklyFocus } from '../../shared/weeklyFocus'

/* ---------- Types ---------- */

interface GrowthCategoryStat {
  category_id: number
  category_name: string
  total: number
  correct: number
  accuracy_rate: number
}

interface GrowthPracticeStats {
  total_answered: number
  correct_count: number
  wrong_count: number
  accuracy_rate: number
  today_count: number
  streak_days: number
  category_stats: GrowthCategoryStat[]
}

interface GrowthStudyLog {
  id: number
  date_key: string
  summary: string
  focus_task_title: string
  completed_count: number
  skipped_count: number
  completed_titles: string[]
  skipped_titles: string[]
  latest_action_text: string
  updated_at: string
}

interface GrowthInterviewSnapshot {
  id: number
  status: string
  score: number
  total_questions: number
  created_at?: string
  ended_at?: string
}

interface GrowthPlanSnapshot {
  id: number
  title: string
  status: string
  total_tasks: number
  completed_tasks: number
  progress: number
  start_date?: string
  end_date?: string
}

interface GrowthCurrentPlan {
  id: number
  title: string
  status: string
  total_tasks: number
  completed_tasks: number
  progress: number
  next_task_title: string
  next_task_source?: string
  next_task_reason?: string
  next_task_source_ref?: string
  next_task_collection_hint?: string
}

interface GrowthFocusSignal {
  focus_tag: string
  topic_code?: string
  topic_title?: string
  topic_problem_pattern?: string
  related_question_sets: string[]
  recommended_actions: string[]
  primary_question_set?: string
  dominant_archive_phase?: string
  dominant_archive_phase_label?: string
  occurrence_count: number
  archive_occurrence_count: number
  interview_occurrence_count: number
  source: string
  source_label: string
  reason: string
}

interface GrowthTrendSummary {
  dominant_source: string
  dominant_source_label: string
  top_focus_tag?: string
  top_topic_code?: string
  top_topic_title?: string
  summary: string
}

interface GrowthSummaryResponse {
  practice_stats?: GrowthPracticeStats | null
  study_days: number
  interview_count: number
  completed_interview_count: number
  average_interview_score: number
  plan_count: number
  current_plan?: GrowthCurrentPlan | null
  focus_signals: GrowthFocusSignal[]
  trend_summary?: GrowthTrendSummary | null
  recent_study_logs: GrowthStudyLog[]
  recent_interviews: GrowthInterviewSnapshot[]
  recent_plans: GrowthPlanSnapshot[]
}

/* ---------- Theme ---------- */

const THEME = {
  primary: '#3b82f6',
  primaryLight: '#eff6ff',
  primaryDark: '#1d4ed8',
  textPrimary: '#1f2937',
  textSecondary: '#6b7280',
  textTertiary: '#9ca3af',
  border: '#e5e7eb',
  borderLight: '#f3f4f6',
  bg: '#f8fafc',
  white: '#ffffff',
  radius: 12,
  radiusSm: 8,
  shadow: '0 1px 3px rgba(0,0,0,0.04), 0 1px 2px rgba(0,0,0,0.02)',
  shadowHover: '0 4px 12px rgba(0,0,0,0.06), 0 2px 4px rgba(0,0,0,0.04)',
  green: '#10b981',
  orange: '#f59e0b',
  red: '#ef4444',
  purple: '#8b5cf6',
}

/* ---------- Helpers ---------- */

async function fetchGrowthSummary(token: string): Promise<GrowthSummaryResponse> {
  const response = await requestJson<ApiEnvelope<GrowthSummaryResponse>>('/growth/summary', { token })
  if (!isSuccessCode(response.code) || !response.data) {
    throw new Error(response.message || '获取成长档案失败')
  }
  return response.data
}

function formatGrowthDateTime(value?: string): string {
  if (!value) return '--'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  })
}

function formatGrowthDate(value?: string): string {
  if (!value) return '--'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleDateString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
  })
}

function growthPlanStatusLabel(status: string): string {
  const map: Record<string, string> = {
    active: '进行中',
    paused: '已暂停',
    completed: '已完成',
    draft: '草稿',
  }
  return map[status] || status || '未定义'
}

function growthInterviewStatusLabel(status: string): string {
  const map: Record<string, string> = {
    preparing: '准备中',
    ongoing: '进行中',
    report_generating: '报告生成中',
    completed: '已完成',
    cancelled: '已取消',
  }
  return map[status] || status || '未定义'
}

function formatGrowthScore(score: number): string {
  if (!Number.isFinite(score)) return '--'
  return Number(score).toFixed(1)
}

function buildGrowthLogSummary(log: GrowthStudyLog): string {
  if (log.summary.trim()) return log.summary.trim()
  const fragments = [`完成 ${log.completed_count} 项`]
  if (log.skipped_count > 0) fragments.push(`跳过 ${log.skipped_count} 项`)
  if (log.focus_task_title.trim()) fragments.push(`聚焦「${log.focus_task_title.trim()}」`)
  return fragments.join('，')
}

function formatGrowthQuestionSets(questionSets: string[]): string {
  return questionSets.map((item) => resolvePracticeQuestionSetTitle(item)).filter(Boolean).join('、')
}

function resolveGrowthTaskSourceLabel(source?: string): string {
  const map: Record<string, string> = {
    weak_topic: '当前弱项',
    goal: '阶段目标',
    default: '默认计划任务',
    practice_recommendation: '练习推荐',
    weekly_focus: '本周重点补强',
    plan_feedback_diagnosis: '训练反馈诊断',
  }
  return map[String(source || '').trim()] || source || '未标注来源'
}

function resolveGrowthCompanionIndustryCode(): string {
  return readSelectedFrontendIndustryCode().trim() || DEFAULT_FRONTEND_INDUSTRY_CODE
}

/* ---------- Sub Components ---------- */

function CoreStatCard({ icon, label, value, suffix, color }: {
  icon: React.ReactNode
  label: string
  value: string | number
  suffix?: string
  color?: string
}) {
  return (
    <div
      style={{
        background: THEME.white,
        borderRadius: THEME.radius,
        border: `1px solid ${THEME.border}`,
        padding: '18px 20px',
        display: 'flex',
        alignItems: 'center',
        gap: 14,
        transition: 'box-shadow .2s',
      }}
      onMouseEnter={(e) => { e.currentTarget.style.boxShadow = THEME.shadowHover }}
      onMouseLeave={(e) => { e.currentTarget.style.boxShadow = 'none' }}
    >
      <div
        style={{
          width: 44,
          height: 44,
          borderRadius: 10,
          background: color ? `${color}15` : THEME.primaryLight,
          color: color || THEME.primary,
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          fontSize: 20,
          flexShrink: 0,
        }}
      >
        {icon}
      </div>
      <div>
        <div style={{ fontSize: 13, color: THEME.textSecondary, marginBottom: 2 }}>{label}</div>
        <div style={{ fontSize: 22, fontWeight: 700, color: THEME.textPrimary, lineHeight: 1.2 }}>
          {value}
          {suffix ? <span style={{ fontSize: 13, fontWeight: 400, color: THEME.textTertiary, marginLeft: 4 }}>{suffix}</span> : null}
        </div>
      </div>
    </div>
  )
}

function PlanBanner({ plan, onFollowUp }: {
  plan?: GrowthCurrentPlan | null
  onFollowUp: (opts: { summary: string; focusTitle: string; weakTopics: string[]; suggestions: string[] }) => void
}) {
  const navigate = useNavigate()

  if (!plan) {
    return (
      <div style={{ background: THEME.primaryLight, borderRadius: THEME.radius, border: `1px solid ${THEME.primary}20`, padding: '24px 28px', display: 'flex', alignItems: 'center', justifyContent: 'space-between', flexWrap: 'wrap', gap: 16 }}>
        <div>
          <div style={{ fontSize: 13, color: THEME.primary, fontWeight: 600, marginBottom: 4 }}>当前主计划</div>
          <div style={{ fontSize: 16, fontWeight: 700, color: THEME.textPrimary }}>暂时没有进行中的计划</div>
          <p style={{ color: THEME.textSecondary, fontSize: 13, margin: '4px 0 0' }}>如果你已经有面试报告和练习记录，下一步最值得继续用学习陪伴页把计划执行闭环补齐。</p>
        </div>
        <Button type="primary" icon={<RocketOutlined />} onClick={() => onFollowUp({
          summary: '根据成长档案里的最近练习和面试结果，先整理一份可执行的学习计划。',
          focusTitle: '当前趋势主线',
          weakTopics: [],
          suggestions: ['生成一份围绕当前弱项的学习计划'],
        })}>
          生成学习计划
        </Button>
      </div>
    )
  }

  return (
    <div style={{ background: THEME.primaryLight, borderRadius: THEME.radius, border: `1px solid ${THEME.primary}20`, padding: '24px 28px' }}>
      <div style={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', flexWrap: 'wrap', gap: 16, marginBottom: 16 }}>
        <div style={{ flex: 1, minWidth: 0 }}>
          <div style={{ fontSize: 13, color: THEME.primary, fontWeight: 600, marginBottom: 4 }}>当前主计划 · {growthPlanStatusLabel(plan.status)}</div>
          <div style={{ fontSize: 18, fontWeight: 700, color: THEME.textPrimary, marginBottom: 6 }}>{plan.title}</div>
          <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
            <div style={{ flex: 1, maxWidth: 300 }}>
              <Progress percent={Math.round(plan.progress)} size="small" strokeColor={THEME.primary} />
            </div>
            <span style={{ fontSize: 13, color: THEME.textSecondary }}>{plan.completed_tasks}/{plan.total_tasks} 任务</span>
          </div>
        </div>
        <div style={{ display: 'flex', gap: 8, flexShrink: 0 }}>
          {plan.next_task_collection_hint ? (
            <Button
              type="primary"
              icon={<BookOutlined />}
              onClick={() => navigate({
                to: '/practice',
                search: buildPracticeRouteSearch({
                  questionSetSlug: plan.next_task_collection_hint,
                  source: 'practice_recommendation',
                  title: plan.next_task_title,
                  reason: plan.next_task_reason,
                }),
              })}
            >
              按建议题单补练
            </Button>
          ) : null}
          <Button
            icon={<RocketOutlined />}
            onClick={() => onFollowUp({
              summary: plan.next_task_reason || '继续围绕当前主计划的下一任务推进，并根据最近趋势收口学习节奏。',
              focusTitle: plan.next_task_title || plan.title || '当前主计划',
              weakTopics: [plan.next_task_title || '', plan.next_task_reason || ''],
              suggestions: [plan.next_task_reason || '优先推进当前计划里的下一项任务。'],
            })}
          >
            去陪伴页
          </Button>
        </div>
      </div>

      <div style={{ display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap', fontSize: 13, color: THEME.textSecondary }}>
        <BulbOutlined />
        <span>下一步：</span>
        <span style={{ fontWeight: 600, color: THEME.textPrimary }}>{plan.next_task_title || '当前没有待推进任务'}</span>
        {plan.next_task_source ? <Tag size="small" style={{ margin: 0 }}>{resolveGrowthTaskSourceLabel(plan.next_task_source)}</Tag> : null}
        {plan.next_task_reason ? <span>· {plan.next_task_reason}</span> : null}
      </div>
    </div>
  )
}

function TrendSignalsPanel({ signals, mistakeTopicMap, onFollowUp }: {
  signals: GrowthFocusSignal[]
  mistakeTopicMap: Map<string, { code: string; title: string; problem_pattern: string; tag: string }>
  onFollowUp: (opts: { summary: string; focusTitle: string; weakTopics: string[]; suggestions: string[] }) => void
}) {
  const navigate = useNavigate()

  if (!signals.length) {
    return <Empty description="还没有趋势信号，先做几道题或完成一场面试" image={Empty.PRESENTED_IMAGE_SIMPLE} />
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
      {signals.map((item, index) => (
        <div
          key={`${item.focus_tag}-${index}`}
          style={{
            background: THEME.white,
            borderRadius: THEME.radius,
            border: `1px solid ${THEME.border}`,
            padding: '16px 20px',
            transition: 'box-shadow .2s',
          }}
          onMouseEnter={(e) => { e.currentTarget.style.boxShadow = THEME.shadowHover }}
          onMouseLeave={(e) => { e.currentTarget.style.boxShadow = 'none' }}
        >
          <div style={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', gap: 12, marginBottom: 8 }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: 10, flexWrap: 'wrap' }}>
              <Tag color="red" style={{ margin: 0, fontSize: 12 }}>{item.focus_tag}</Tag>
              <span style={{ fontSize: 15, fontWeight: 600, color: THEME.textPrimary }}>{item.topic_title || item.focus_tag}</span>
              {item.dominant_archive_phase_label ? <Tag size="small" style={{ margin: 0 }}>{item.dominant_archive_phase_label}</Tag> : null}
            </div>
            <span style={{ fontSize: 12, color: THEME.textTertiary, flexShrink: 0 }}>{item.source_label}</span>
          </div>

          <div style={{ fontSize: 13, color: THEME.textSecondary, lineHeight: 1.7, marginBottom: 10 }}>
            {item.reason ? <p style={{ margin: '0 0 4px' }}>{item.reason}</p> : null}
            <p style={{ margin: 0 }}>
              最近出现 <strong style={{ color: THEME.textPrimary }}>{item.occurrence_count}</strong> 次
              {item.archive_occurrence_count > 0 ? `，练习暴露 ${item.archive_occurrence_count} 次` : ''}
              {item.interview_occurrence_count > 0 ? `，面试暴露 ${item.interview_occurrence_count} 次` : ''}
            </p>
          </div>

          {item.recommended_actions.length > 0 ? (
            <ul style={{ margin: '0 0 12px', paddingLeft: 18, color: THEME.textSecondary, fontSize: 13, lineHeight: 1.8 }}>
              {item.recommended_actions.map((action) => (
                <li key={action}>{action}</li>
              ))}
            </ul>
          ) : null}

          <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
            <Button
              size="small"
              type="primary"
              icon={<BookOutlined />}
              onClick={() => navigate({
                to: '/practice',
                search: buildPracticeRouteSearch({
                  questionSetSlug: item.primary_question_set || item.related_question_sets?.[0] || '',
                  topicCode: item.topic_code,
                  focusTags: [item.focus_tag],
                  source: 'practice_recommendation',
                  title: item.topic_title || item.focus_tag,
                  reason: item.reason,
                }),
              })}
            >
              去题库补练
            </Button>
            <Button
              size="small"
              icon={<RocketOutlined />}
              onClick={() => onFollowUp({
                summary: item.reason || `围绕「${item.topic_title || item.focus_tag}」继续收束当前学习主线。`,
                focusTitle: item.topic_title || item.focus_tag,
                weakTopics: [item.focus_tag, item.topic_title || '', item.topic_problem_pattern || ''],
                suggestions: item.recommended_actions || [],
              })}
            >
              带入学习计划
            </Button>
            {item.topic_code ? (
              <Button size="small" onClick={() => navigate({ to: resolveMistakeTopicRoute(), params: { topicCode: item.topic_code } })}>
                看专题
              </Button>
            ) : null}
          </div>
        </div>
      ))}
    </div>
  )
}

function WeeklyFocusPanel({ themes, weeklyFocusTopicMap, onFollowUp, onOpenPractice }: {
  themes: Array<{
    title: string
    source_label: string
    dominant_archive_phase_label?: string
    reason: string
    occurrence_count: number
    interview_occurrence_count: number
    focus_tags: string[]
    related_question_sets?: string[]
    suggestions: string[]
    topic_codes: string[]
  }>
  weeklyFocusTopicMap: Map<string, { code: string; title: string; problem_pattern: string; tag: string } | null>
  onFollowUp: (opts: { summary: string; focusTitle: string; weakTopics: string[]; suggestions: string[] }) => void
  onOpenPractice: (themeTitle: string) => void
}) {
  if (!themes.length) {
    return <Empty description="本周还没有明确主攻主题" image={Empty.PRESENTED_IMAGE_SIMPLE} />
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
      {themes.map((theme) => {
        const linkedTopic = weeklyFocusTopicMap.get(theme.title)
        return (
          <div
            key={theme.title}
            style={{
              background: THEME.white,
              borderRadius: THEME.radius,
              border: `1px solid ${THEME.border}`,
              padding: '16px 20px',
              transition: 'box-shadow .2s',
            }}
            onMouseEnter={(e) => { e.currentTarget.style.boxShadow = THEME.shadowHover }}
            onMouseLeave={(e) => { e.currentTarget.style.boxShadow = 'none' }}
          >
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 8 }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: 10, flexWrap: 'wrap' }}>
                <Tag color="orange" style={{ margin: 0, fontSize: 12 }}>本周重点</Tag>
                <span style={{ fontSize: 15, fontWeight: 600, color: THEME.textPrimary }}>{theme.title}</span>
                {theme.dominant_archive_phase_label ? <Tag size="small" style={{ margin: 0 }}>{theme.dominant_archive_phase_label}</Tag> : null}
              </div>
              <span style={{ fontSize: 12, color: THEME.textTertiary }}>{theme.source_label}</span>
            </div>

            <p style={{ color: THEME.textSecondary, fontSize: 13, lineHeight: 1.7, margin: '0 0 10px' }}>{theme.reason}</p>

            {theme.focus_tags.length > 0 ? (
              <div style={{ display: 'flex', gap: 6, flexWrap: 'wrap', marginBottom: 10 }}>
                {theme.focus_tags.map((tag) => <Tag key={tag} size="small" style={{ margin: 0 }}>{tag}</Tag>)}
              </div>
            ) : null}

            {theme.suggestions.length > 0 ? (
              <ul style={{ margin: '0 0 12px', paddingLeft: 18, color: THEME.textSecondary, fontSize: 13, lineHeight: 1.8 }}>
                {theme.suggestions.map((s) => <li key={s}>{s}</li>)}
              </ul>
            ) : null}

            <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
              <Button size="small" type="primary" icon={<RocketOutlined />} onClick={() => onFollowUp({
                summary: theme.reason,
                focusTitle: theme.title,
                weakTopics: [theme.title, ...theme.focus_tags],
                suggestions: theme.suggestions,
              })}>
                生成补强计划
              </Button>
              <Button size="small" icon={<BookOutlined />} onClick={() => onOpenPractice(theme.title)}>去题库补练</Button>
              {linkedTopic ? (
                <Button size="small" onClick={() => { /* navigate handled in parent */ }}>打开专题</Button>
              ) : null}
            </div>
          </div>
        )
      })}
    </div>
  )
}

function RecommendationsPanel({ items, mistakeTopicMap }: {
  items: Array<{
    question: { id: number; title: string; difficulty: string; type: string }
    focus_tag: string
    topic_title?: string
    topic_code?: string
    dominant_archive_phase_label?: string
    reason: string
    priority_explanation?: string
    recommendation_mode: string
    source_type: string
    primary_question_set?: string
    topic_problem_pattern?: string
    related_question_sets?: string[]
    recommended_actions?: string[]
  }>
  mistakeTopicMap: Map<string, { code: string; title: string; problem_pattern: string; tag: string }>
}) {
  const navigate = useNavigate()

  if (!items.length) {
    return <Empty description="还没有足够的推荐依据，先在题库里完成几道题" image={Empty.PRESENTED_IMAGE_SIMPLE} />
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
      {items.map((item) => {
        const linkedTopic = item.topic_code ? mistakeTopicMap.get(item.topic_code) || null : null
        return (
          <div
            key={item.question.id}
            style={{
              background: THEME.white,
              borderRadius: THEME.radius,
              border: `1px solid ${THEME.border}`,
              padding: '14px 18px',
              transition: 'box-shadow .2s',
            }}
            onMouseEnter={(e) => { e.currentTarget.style.boxShadow = THEME.shadowHover }}
            onMouseLeave={(e) => { e.currentTarget.style.boxShadow = 'none' }}
          >
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 12, marginBottom: 6 }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap' }}>
                <Tag color="blue" style={{ margin: 0, fontSize: 12 }}>{item.focus_tag}</Tag>
                <span style={{ fontSize: 14, fontWeight: 600, color: THEME.textPrimary }}>{item.question.title}</span>
                <Tag size="small" style={{ margin: 0 }}>{item.question.difficulty || '未标注'}</Tag>
              </div>
            </div>

            <p style={{ color: THEME.textSecondary, fontSize: 13, lineHeight: 1.6, margin: '0 0 8px' }}>{item.reason}</p>

            {item.recommended_actions && item.recommended_actions.length > 0 ? (
              <ul style={{ margin: '0 0 10px', paddingLeft: 18, color: THEME.textSecondary, fontSize: 13, lineHeight: 1.7 }}>
                {item.recommended_actions.map((a) => <li key={a}>{a}</li>)}
              </ul>
            ) : null}

            <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
              <Button size="small" type="primary" icon={<PlayCircleOutlined />} onClick={() => navigate({
                to: resolvePracticeRecommendationRoute(item.question.type),
                params: { questionId: String(item.question.id) },
              })}>
                去做这题
              </Button>
              <Button size="small" icon={<BookOutlined />} onClick={() => navigate({
                to: '/practice',
                search: buildPracticeRecommendationRouteSearch({
                  focus_tag: item.focus_tag,
                  topic_code: item.topic_code,
                  primary_question_set: item.primary_question_set,
                  reason: item.reason,
                  question_title: item.question.title,
                }, linkedTopic),
              })}>
                进入这组补练
              </Button>
              {item.topic_code ? (
                <Button size="small" onClick={() => navigate({ to: resolveMistakeTopicRoute(), params: { topicCode: item.topic_code } })}>
                  看错因专题
                </Button>
              ) : null}
            </div>
          </div>
        )
      })}
    </div>
  )
}

function CategoryMiniBar({ name, total, correct, accuracy }: { name: string; total: number; correct: number; accuracy: number }) {
  return (
    <div style={{ marginBottom: 12 }}>
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 4 }}>
        <span style={{ fontSize: 13, color: THEME.textPrimary, fontWeight: 500 }}>{name}</span>
        <span style={{ fontSize: 12, color: THEME.textTertiary }}>{correct}/{total} · {formatGrowthScore(accuracy)}%</span>
      </div>
      <div style={{ height: 6, background: THEME.borderLight, borderRadius: 3, overflow: 'hidden' }}>
        <div
          style={{
            height: '100%',
            width: `${Math.min(accuracy, 100)}%`,
            background: accuracy >= 70 ? THEME.green : accuracy >= 40 ? THEME.orange : THEME.red,
            borderRadius: 3,
            transition: 'width .4s ease',
          }}
        />
      </div>
    </div>
  )
}

/* ---------- Main Page ---------- */

export default function GrowthPage() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const accessToken = useAuthStore((state) => state.accessToken)
  const [activeTab, setActiveTab] = useState('trends')

  const growthSummaryQuery = useQuery({
    queryKey: ['growth-summary', accessToken],
    queryFn: () => fetchGrowthSummary(accessToken as string),
    enabled: Boolean(accessToken),
    retry: false,
  })

  const practiceRecommendationsQuery = useQuery({
    queryKey: ['growth-practice-recommendations', accessToken],
    queryFn: () => fetchPracticeRecommendations(accessToken as string, 4),
    enabled: Boolean(accessToken),
    retry: false,
  })

  const weeklyFocusQuery = useQuery({
    queryKey: ['growth-weekly-focus', accessToken],
    queryFn: () => fetchWeeklyFocus(accessToken as string),
    enabled: Boolean(accessToken),
    retry: false,
  })

  const mistakeTopicsQuery = useQuery({
    queryKey: ['growth-mistake-topics', accessToken],
    queryFn: () => fetchMistakeTopics([], accessToken),
    staleTime: 5 * 60 * 1000,
  })

  const mistakeTopicMap = useMemo(
    () => new Map((mistakeTopicsQuery.data || []).map((topic) => [topic.code, topic])),
    [mistakeTopicsQuery.data],
  )

  const topCategoryStats = useMemo(
    () => (growthSummaryQuery.data?.practice_stats?.category_stats || []).slice(0, 4),
    [growthSummaryQuery.data?.practice_stats?.category_stats],
  )

  const focusTopics = useMemo(
    () => pickMistakeTopicsByTags(practiceRecommendationsQuery.data?.focus_tags || [], mistakeTopicsQuery.data || []),
    [practiceRecommendationsQuery.data?.focus_tags, mistakeTopicsQuery.data],
  )

  const weeklyFocusTopicMap = useMemo(
    () =>
      new Map(
        (weeklyFocusQuery.data?.themes || [])
          .map((theme) => {
            const topicCode = theme.topic_codes[0]
            return [theme.title, topicCode ? mistakeTopicMap.get(topicCode) || null : null] as const
          }),
      ),
    [mistakeTopicMap, weeklyFocusQuery.data?.themes],
  )

  const data = growthSummaryQuery.data
  const stats = data?.practice_stats

  function handleOpenWeeklyFocusPractice(themeTitle: string): void {
    const theme = weeklyFocusQuery.data?.themes.find((item) => item.title === themeTitle)
    if (!theme) {
      navigate({ to: '/practice' })
      return
    }
    const linkedTopic = weeklyFocusTopicMap.get(themeTitle)
    navigate({
      to: '/practice',
      search: buildWeeklyFocusPracticeRouteSearch(theme, linkedTopic),
    })
  }

  function handleCompanionFollowUp(options: {
    summary: string
    focusTitle: string
    weakTopics: string[]
    suggestions: string[]
  }): void {
    const industryCode = resolveGrowthCompanionIndustryCode()
    persistCompanionPlanContext(buildGrowthCompanionContextDraft({
      industryCode,
      industryLabel: industryCode.toUpperCase(),
      summary: options.summary,
      focusTitle: options.focusTitle,
      weakTopics: options.weakTopics,
      suggestions: options.suggestions,
    }))
    navigate({ to: '/companion' })
  }

  const tabItems: TabsProps['items'] = [
    {
      key: 'trends',
      label: (
        <span style={{ display: 'inline-flex', alignItems: 'center', gap: 6 }}>
          <LineChartOutlined />趋势信号
        </span>
      ),
      children: data ? (
        <TrendSignalsPanel
          signals={data.focus_signals}
          mistakeTopicMap={mistakeTopicMap}
          onFollowUp={handleCompanionFollowUp}
        />
      ) : null,
    },
    {
      key: 'weekly',
      label: (
        <span style={{ display: 'inline-flex', alignItems: 'center', gap: 6 }}>
          <FireOutlined />本周补强
        </span>
      ),
      children: weeklyFocusQuery.data ? (
        <WeeklyFocusPanel
          themes={weeklyFocusQuery.data.themes}
          weeklyFocusTopicMap={weeklyFocusTopicMap}
          onFollowUp={handleCompanionFollowUp}
          onOpenPractice={handleOpenWeeklyFocusPractice}
        />
      ) : weeklyFocusQuery.isLoading ? (
        <div style={{ textAlign: 'center', padding: 40 }}><Spin /></div>
      ) : null,
    },
    {
      key: 'recommendations',
      label: (
        <span style={{ display: 'inline-flex', alignItems: 'center', gap: 6 }}>
          <AimOutlined />推荐补练
        </span>
      ),
      children: practiceRecommendationsQuery.data ? (
        <RecommendationsPanel
          items={practiceRecommendationsQuery.data.items}
          mistakeTopicMap={mistakeTopicMap}
        />
      ) : practiceRecommendationsQuery.isLoading ? (
        <div style={{ textAlign: 'center', padding: 40 }}><Spin /></div>
      ) : null,
    },
  ]

  return (
    <div style={{ maxWidth: 1200, margin: '0 auto', padding: '24px 16px' }}>
      {/* Header */}
      <div style={{ marginBottom: 24 }}>
        <h1 style={{ fontSize: 22, fontWeight: 700, color: THEME.textPrimary, margin: '0 0 4px' }}>成长档案</h1>
        <p style={{ color: THEME.textSecondary, margin: 0, fontSize: 13 }}>把练习、面试、计划和每日推进沉淀成一份可回看的成长记录</p>
      </div>

      {!accessToken ? (
        <div style={{ background: THEME.white, borderRadius: THEME.radius, border: `1px solid ${THEME.border}`, padding: 48, textAlign: 'center' }}>
          <CalendarOutlined style={{ fontSize: 48, color: THEME.border, marginBottom: 16 }}></CalendarOutlined>
          <h2 style={{ fontSize: 18, fontWeight: 600, color: THEME.textPrimary, margin: '0 0 8px' }}>登录后查看成长档案</h2>
          <p style={{ color: THEME.textSecondary, fontSize: 14, margin: '0 0 20px' }}>成长档案会把练习、面试、学习计划和每日推进统一沉淀到这里。</p>
          <Button type="primary" onClick={() => requestLoginPrompt('/growth', 'missing')}>去登录</Button>
        </div>
      ) : null}

      {growthSummaryQuery.isLoading ? (
        <div style={{ textAlign: 'center', padding: 48 }}><Spin /></div>
      ) : null}

      {growthSummaryQuery.isError ? (
        <div style={{ padding: 24, textAlign: 'center', color: THEME.red }}>
          {extractErrorMessage(growthSummaryQuery.error, '成长档案读取失败，请稍后重试')}
        </div>
      ) : null}

      {data ? (
        <>
          {/* Core stats */}
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(220px, 1fr))', gap: 16, marginBottom: 20 }}>
            <CoreStatCard
              icon={<CalendarOutlined />}
              label="累计学习天数"
              value={data.study_days}
              suffix="天"
              color={THEME.primary}
            />
            <CoreStatCard
              icon={<FireOutlined />}
              label="连续打卡"
              value={stats?.streak_days || 0}
              suffix="天"
              color={THEME.orange}
            />
            <CoreStatCard
              icon={<TrophyOutlined />}
              label="面试场次"
              value={data.interview_count}
              suffix={`已完成 ${data.completed_interview_count}`}
              color={THEME.purple}
            />
            <CoreStatCard
              icon={<CheckCircleOutlined />}
              label="总正确率"
              value={data.completed_interview_count > 0 ? formatGrowthScore(data.average_interview_score) : '--'}
              suffix="%"
              color={THEME.green}
            />
          </div>

          {/* Plan banner */}
          <div style={{ marginBottom: 20 }}>
            <PlanBanner plan={data.current_plan} onFollowUp={handleCompanionFollowUp} />
          </div>

          {/* Action center tabs */}
          <div style={{ background: THEME.white, borderRadius: THEME.radius, border: `1px solid ${THEME.border}`, padding: '20px 24px', marginBottom: 20 }}>
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 16 }}>
              <h2 style={{ fontSize: 16, fontWeight: 700, color: THEME.textPrimary, margin: 0, display: 'flex', alignItems: 'center', gap: 8 }}>
                <ThunderboltOutlined style={{ color: THEME.orange }} />
                行动中心
              </h2>
              {data.trend_summary?.summary ? (
                <Tooltip title={data.trend_summary.summary}>
                  <Tag color="blue" style={{ cursor: 'help' }}>{data.trend_summary.dominant_source_label}</Tag>
                </Tooltip>
              ) : null}
            </div>
            <Tabs
              activeKey={activeTab}
              onChange={setActiveTab}
              items={tabItems}
              size="small"
            />
          </div>

          {/* Two-column data area */}
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(320px, 1fr))', gap: 16, marginBottom: 20 }}>
            {/* Practice overview */}
            <div style={{ background: THEME.white, borderRadius: THEME.radius, border: `1px solid ${THEME.border}`, padding: '20px 24px' }}>
              <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 16 }}>
                <h3 style={{ fontSize: 15, fontWeight: 700, color: THEME.textPrimary, margin: 0, display: 'flex', alignItems: 'center', gap: 6 }}>
                  <BarChartOutlined style={{ color: THEME.primary }} />
                  练习概览
                </h3>
                <Link to="/practice" style={{ fontSize: 13, color: THEME.primary, textDecoration: 'none' }}>去题库 →</Link>
              </div>

              {stats ? (
                <>
                  <div style={{ display: 'flex', alignItems: 'center', gap: 20, marginBottom: 20 }}>
                    <div style={{ textAlign: 'center' }}>
                      <Progress
                        type="circle"
                        percent={Math.round(stats.accuracy_rate)}
                        size={80}
                        strokeColor={stats.accuracy_rate >= 70 ? THEME.green : stats.accuracy_rate >= 40 ? THEME.orange : THEME.red}
                        format={(percent) => <span style={{ fontSize: 16, fontWeight: 700 }}>{percent}%</span>}
                      />
                      <div style={{ fontSize: 12, color: THEME.textTertiary, marginTop: 4 }}>正确率</div>
                    </div>
                    <div style={{ flex: 1 }}>
                      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 8 }}>
                        <span style={{ fontSize: 13, color: THEME.textSecondary }}>累计答题</span>
                        <span style={{ fontSize: 13, fontWeight: 600, color: THEME.textPrimary }}>{stats.total_answered}</span>
                      </div>
                      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 8 }}>
                        <span style={{ fontSize: 13, color: THEME.textSecondary }}>答对</span>
                        <span style={{ fontSize: 13, fontWeight: 600, color: THEME.green }}>{stats.correct_count}</span>
                      </div>
                      <div style={{ display: 'flex', justifyContent: 'space-between' }}>
                        <span style={{ fontSize: 13, color: THEME.textSecondary }}>答错</span>
                        <span style={{ fontSize: 13, fontWeight: 600, color: THEME.red }}>{stats.wrong_count}</span>
                      </div>
                    </div>
                  </div>

                  <Divider style={{ margin: '12px 0' }} />

                  <div style={{ fontSize: 13, fontWeight: 600, color: THEME.textPrimary, marginBottom: 10 }}>分类表现</div>
                  {topCategoryStats.length > 0 ? (
                    topCategoryStats.map((item) => (
                      <CategoryMiniBar
                        key={item.category_id}
                        name={item.category_name}
                        total={item.total}
                        correct={item.correct}
                        accuracy={item.accuracy_rate}
                      />
                    ))
                  ) : (
                    <Empty description="还没有分类统计" image={Empty.PRESENTED_IMAGE_SIMPLE} />
                  )}
                </>
              ) : (
                <Empty description="还没有练习数据" image={Empty.PRESENTED_IMAGE_SIMPLE} />
              )}
            </div>

            {/* Recent activity */}
            <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
              {/* Recent interviews */}
              <div style={{ background: THEME.white, borderRadius: THEME.radius, border: `1px solid ${THEME.border}`, padding: '20px 24px' }}>
                <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 12 }}>
                  <h3 style={{ fontSize: 15, fontWeight: 700, color: THEME.textPrimary, margin: 0, display: 'flex', alignItems: 'center', gap: 6 }}>
                    <TrophyOutlined style={{ color: THEME.purple }} />
                    最近面试
                  </h3>
                  <Link to="/interview" style={{ fontSize: 13, color: THEME.primary, textDecoration: 'none' }}>更多 →</Link>
                </div>

                {data.recent_interviews.length > 0 ? (
                  <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
                    {data.recent_interviews.map((interview) => (
                      <div
                        key={interview.id}
                        style={{
                          display: 'flex',
                          alignItems: 'center',
                          justifyContent: 'space-between',
                          padding: '10px 12px',
                          background: THEME.bg,
                          borderRadius: THEME.radiusSm,
                          cursor: 'pointer',
                        }}
                        onClick={() => navigate({
                          to: interview.status === 'ongoing' || interview.status === 'preparing'
                            ? '/interview/$interviewId'
                            : '/interview/$interviewId/report',
                          params: { interviewId: String(interview.id) },
                        })}
                      >
                        <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
                          <Avatar size={28} icon={<TrophyOutlined />} style={{ background: THEME.primaryLight, color: THEME.primary, fontSize: 14 }} />
                          <div>
                            <div style={{ fontSize: 13, fontWeight: 600, color: THEME.textPrimary }}>面试 #{interview.id}</div>
                            <div style={{ fontSize: 12, color: THEME.textTertiary }}>{formatGrowthDateTime(interview.created_at)}</div>
                          </div>
                        </div>
                        <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                          <Tag size="small" style={{ margin: 0 }}>{growthInterviewStatusLabel(interview.status)}</Tag>
                          <span style={{ fontSize: 13, fontWeight: 700, color: THEME.textPrimary }}>{formatGrowthScore(interview.score)}</span>
                        </div>
                      </div>
                    ))}
                  </div>
                ) : (
                  <Empty description="还没有面试记录" image={Empty.PRESENTED_IMAGE_SIMPLE} />
                )}
              </div>

              {/* Recent plans */}
              <div style={{ background: THEME.white, borderRadius: THEME.radius, border: `1px solid ${THEME.border}`, padding: '20px 24px' }}>
                <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 12 }}>
                  <h3 style={{ fontSize: 15, fontWeight: 700, color: THEME.textPrimary, margin: 0, display: 'flex', alignItems: 'center', gap: 6 }}>
                    <FlagOutlined style={{ color: THEME.primary }} />
                    最近计划
                  </h3>
                  <Link to="/companion" style={{ fontSize: 13, color: THEME.primary, textDecoration: 'none' }}>更多 →</Link>
                </div>

                {data.recent_plans.length > 0 ? (
                  <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
                    {data.recent_plans.map((plan) => (
                      <div key={plan.id} style={{ padding: '10px 12px', background: THEME.bg, borderRadius: THEME.radiusSm }}>
                        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 6 }}>
                          <span style={{ fontSize: 13, fontWeight: 600, color: THEME.textPrimary }}>{plan.title}</span>
                          <Tag size="small" style={{ margin: 0 }}>{growthPlanStatusLabel(plan.status)}</Tag>
                        </div>
                        <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                          <div style={{ flex: 1 }}>
                            <Progress percent={Math.round(plan.progress)} size="small" strokeColor={THEME.primary} showInfo={false} />
                          </div>
                          <span style={{ fontSize: 12, color: THEME.textTertiary, flexShrink: 0 }}>{plan.completed_tasks}/{plan.total_tasks}</span>
                        </div>
                      </div>
                    ))}
                  </div>
                ) : (
                  <Empty description="还没有学习计划" image={Empty.PRESENTED_IMAGE_SIMPLE} />
                )}
              </div>
            </div>
          </div>

          {/* Study log timeline */}
          <div style={{ background: THEME.white, borderRadius: THEME.radius, border: `1px solid ${THEME.border}`, padding: '20px 24px' }}>
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 16 }}>
              <h3 style={{ fontSize: 15, fontWeight: 700, color: THEME.textPrimary, margin: 0, display: 'flex', alignItems: 'center', gap: 6 }}>
                <ClockCircleOutlined style={{ color: THEME.primary }} />
                学习日志
              </h3>
              <Link to="/companion" style={{ fontSize: 13, color: THEME.primary, textDecoration: 'none' }}>去记录 →</Link>
            </div>

            {data.recent_study_logs.length > 0 ? (
              <Timeline mode="left">
                {data.recent_study_logs.map((log) => (
                  <Timeline.Item
                    key={log.id}
                    label={<span style={{ fontSize: 12, color: THEME.textTertiary }}>{log.date_key}</span>}
                    dot={<div style={{ width: 10, height: 10, borderRadius: '50%', background: log.completed_count > 0 ? THEME.green : THEME.orange, border: `2px solid ${THEME.white}` }} />}
                  >
                    <div style={{ padding: '8px 12px', background: THEME.bg, borderRadius: THEME.radiusSm, marginBottom: 8 }}>
                      <div style={{ fontSize: 13, fontWeight: 600, color: THEME.textPrimary, marginBottom: 4 }}>
                        {buildGrowthLogSummary(log)}
                      </div>
                      <div style={{ fontSize: 12, color: THEME.textSecondary }}>
                        聚焦：{log.focus_task_title || '未记录'} · 完成 {log.completed_count} · 跳过 {log.skipped_count}
                      </div>
                    </div>
                  </Timeline.Item>
                ))}
              </Timeline>
            ) : (
              <Empty description="还没有学习日志，去陪伴页记录今天" image={Empty.PRESENTED_IMAGE_SIMPLE} />
            )}
          </div>
        </>
      ) : null}
    </div>
  )
}
