import type { CSSProperties, FormEvent } from 'react'
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { Link, useNavigate, useParams } from '@tanstack/react-router'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { extractErrorMessage, requestJson } from '@makejob/api-client'
import { isSuccessCode, type ApiEnvelope } from '@makejob/shared-types'
import { useAuthStore } from '../../state/auth'
import { readCurrentBrowserPath } from '../../shared/authRedirect'
import { findFrontendIndustryById } from '../../shared/frontendIndustryPreference'
import {
  fetchFrontendIndustries,
  formatFrontendIndustryLabel,
  persistSelectedFrontendIndustryCode,
} from '../../shared/industryContext'
import { useFrontendIndustriesQuery } from '../../shared/frontendQueries'
import { requestLoginPrompt } from '../../shared/loginPrompt'
import { fetchMistakeTopic, fetchMistakeTopics, parsePracticeAnalysis, pickMistakeTopicsByTags, resolveMistakeTopicRoute } from '../../shared/mistakeTopics'
import {
  buildMistakeTopicPracticeRouteSearch,
  resolvePracticeQuestionSetTitle,
} from '../../shared/practiceRoute'
import {
  difficultyLabel,
  PRACTICE_PAGE_SIZE,
  type PracticeQuestion,
  questionTypeLabel,
} from '../../shared/practiceCatalog'
import { Button, Empty, Pagination, Spin, Tag } from 'antd'
import {
  CloseCircleOutlined,
  RedoOutlined,
  BookOutlined,
  ClockCircleOutlined,
  CheckCircleOutlined,
  CloseOutlined,
  FireOutlined,
  TagOutlined,
  EditOutlined,
  DeleteOutlined,
  SaveOutlined,
  ArrowLeftOutlined,
  StarOutlined,
  StarFilled,
  MessageOutlined,
  TrophyOutlined,
  ThunderboltOutlined,
  RightOutlined,
  ExclamationCircleOutlined,
  BulbOutlined,
  ProfileOutlined,
  FileTextOutlined,
} from '@ant-design/icons'

interface PracticeQuestionSolution {
  summary: string
  approach: string
  key_steps: string[]
  edge_cases: string[]
  complexity: string
  common_mistakes: string[]
  recommended_tags: string[]
}

interface PracticeQuestionAnswerTemplate {
  core_conclusion: string
  key_points: string[]
  sample_answer: string
  follow_ups: string[]
  pitfalls: string[]
}

interface PracticeQuestionTestCase {
  input: string
  expected_output: string
  description?: string
}

interface PracticeQuestionReferenceSolution {
  language: string
  title?: string
  code: string
  explanation?: string
}

interface PracticeQuestionJudgeConfig {
  evaluation_mode: 'analysis_only' | 'testcase'
  default_language: string
  allowed_languages: string[]
  starter_code: string
  public_test_cases: PracticeQuestionTestCase[]
  time_limit_ms: number
  memory_limit_mb: number
}

interface PracticeJudgeCaseResult {
  index: number
  description?: string
  input?: string
  expected_output?: string
  actual_output?: string
  passed: boolean
  error_output?: string
}

interface PracticeJudgeSummary {
  mode: string
  passed_count: number
  total_count: number
  all_passed: boolean
  case_results?: PracticeJudgeCaseResult[]
  time_limit_ms?: number
  memory_limit_mb?: number
}

interface PageResult<T> {
  list: T[]
  total: number
  page: number
  page_size: number
}

interface QuestionOption {
  label: string
  text: string
}

interface PracticeNote {
  id: number
  user_id: number
  question_id?: number
  title: string
  content: string
  created_at?: string
  updated_at?: string
  question?: PracticeQuestion
}

interface PracticeQuestionDetail extends PracticeQuestion {
  content: string
  options_json?: string
  answer?: string
  explanation?: string
  tag_list?: string[]
  solution?: PracticeQuestionSolution | null
  judge_config?: PracticeQuestionJudgeConfig | null
  answer_template?: PracticeQuestionAnswerTemplate | null
  is_favorited?: boolean
  user_note?: PracticeNote | null
}

interface SubmitAnswerResult {
  is_correct: boolean
  correct_answer: string
  explanation: string
  ai_analysis?: string
  evaluation_mode?: string
  judge_summary?: PracticeJudgeSummary | null
}

interface RunCodeResult {
  output: string
  passed: boolean
  evaluation_mode?: string
  judge_summary?: PracticeJudgeSummary | null
}

interface FavoriteRecord {
  id: number
  question_id: number
  created_at?: string
  question?: PracticeQuestion
}

interface WrongQuestionRecord {
  id: number
  question_id: number
  user_answer: string
  is_correct: boolean
  time_spent: number
  created_at: string
  question?: PracticeQuestion
}

const NOTE_PAGE_SIZE = 20

/**
 * 将后端时间字段格式化为更适合前台阅读的时间文本。
 */
function formatDateTime(value?: string | number): string {
  if (!value) {
    return '-'
  }

  const date = typeof value === 'number' ? new Date(value * 1000) : new Date(value)
  if (Number.isNaN(date.getTime())) {
    return String(value)
  }

  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  const hour = String(date.getHours()).padStart(2, '0')
  const minute = String(date.getMinutes()).padStart(2, '0')
  return `${year}-${month}-${day} ${hour}:${minute}`
}

/**
 * 将后端返回的 options_json 兼容解析为统一选项结构。
 */
function parseQuestionOptions(raw?: string): QuestionOption[] {
  if (!raw) {
    return []
  }

  try {
    const parsed = JSON.parse(raw) as Array<string | { label?: string; text?: string }>
    return parsed.map((item, index) => {
      if (typeof item === 'string') {
        return {
          label: String.fromCharCode(65 + index),
          text: item,
        }
      }

      return {
        label: item.label || String.fromCharCode(65 + index),
        text: item.text || '',
      }
    })
  } catch {
    return []
  }
}

/**
 * 根据题目类型和当前表单值构造最终提交答案。
 */
function buildSubmittedAnswer(type: string, singleAnswer: string, multiAnswers: string[], textAnswer: string): string {
  if (type === 'choice') {
    return singleAnswer.trim()
  }

  if (type === 'multi') {
    return [...multiAnswers].sort().join(',')
  }

  return textAnswer.trim()
}

/**
 * 为代码题生成默认模板，确保编辑器初次打开时不会是空白状态。
 */
function buildDefaultCodeTemplate(): string {
  return `package main

import "fmt"

func solution() {
    // 在这里编写你的代码
    fmt.Println("Hello, MakeJob!")
}

func main() {
    solution()
}
`
}

/**
 * 根据题目判题配置生成更贴近真实题目的默认模板，并兼容旧题回退模板。
 */
function buildCodeTemplateFromQuestion(question?: PracticeQuestionDetail | null): string {
  const starterCode = question?.judge_config?.starter_code?.trim() || ''
  if (starterCode) {
    return starterCode
  }
  return buildDefaultCodeTemplate()
}

/**
 * 为代码题草稿生成稳定的本地缓存键，按题目和语言隔离存储。
 */
function buildCodeDraftStorageKey(questionId: number | string, language: string): string {
  return `makejob.practice.code-draft.${questionId}.${language}`
}

/**
 * 读取代码题本地草稿，优先恢复用户上次未完成的编辑内容。
 */
function readCodeDraft(questionId: number | string, language: string, fallback: string): string {
  if (typeof window === 'undefined') {
    return fallback
  }

  const raw = window.localStorage.getItem(buildCodeDraftStorageKey(questionId, language))
  return raw && raw.trim() ? raw : fallback
}

/**
 * 持久化代码题草稿，避免页面刷新或切题后输入内容直接丢失。
 */
function persistCodeDraft(questionId: number | string, language: string, content: string): void {
  if (typeof window === 'undefined') {
    return
  }

  window.localStorage.setItem(buildCodeDraftStorageKey(questionId, language), content)
}

/**
 * 拉取题目详情，供普通练习页和代码题编辑器共用。
 */
async function fetchQuestionDetail(questionId: string, token?: string | null): Promise<PracticeQuestionDetail> {
  const response = await requestJson<ApiEnvelope<PracticeQuestionDetail>>(`/questions/${questionId}`, {
    token: token || undefined,
  })
  if (!isSuccessCode(response.code) || !response.data) {
    throw new Error(response.message || '获取题目详情失败')
  }

  return response.data
}

/**
 * 拉取当前用户的收藏题目列表。
 */
async function fetchFavorites(token: string, page: number, pageSize: number): Promise<PageResult<FavoriteRecord>> {
  const response = await requestJson<ApiEnvelope<{ list: Array<{
    id: number
    title: string
    difficulty: string
    type: string
  }>; total: number; page: number; page_size: number }>>(`/user/favorites?page=${page}&page_size=${pageSize}`, {
    token,
  })

  if (!isSuccessCode(response.code) || !response.data) {
    throw new Error(response.message || '获取收藏列表失败')
  }

  const items = response.data.list || []

  return {
    list: items.map((item) => ({
      id: item.id,
      question_id: item.id,
      question: {
        id: item.id,
        title: item.title,
        difficulty: item.difficulty,
        type: item.type,
        category_id: 0,
        industry_id: 0,
      },
    })),
    total: response.data.total || 0,
    page: response.data.page || page,
    page_size: response.data.page_size || pageSize,
  }
}

/**
 * 拉取当前用户的错题本列表。
 */
async function fetchWrongQuestions(token: string, page: number, pageSize: number): Promise<PageResult<WrongQuestionRecord>> {
  const response = await requestJson<ApiEnvelope<{ list: Array<{
    question_id: number
    title: string
    difficulty: string
    type: string
    category_name: string
    category_id: number
    wrong_count: number
    last_wrong_at?: string
    last_answer: string
  }>; total: number; page: number; page_size: number }>>(`/user/wrong-questions?page=${page}&page_size=${pageSize}`, {
    token,
  })

  if (!isSuccessCode(response.code) || !response.data) {
    throw new Error(response.message || '获取错题本失败')
  }

  const items = response.data.list || []

  return {
    list: items.map((item) => ({
      id: item.question_id,
      question_id: item.question_id,
      user_answer: item.last_answer || '',
      is_correct: false,
      time_spent: 0,
      created_at: item.last_wrong_at || '',
      question: {
        id: item.question_id,
        title: item.title,
        difficulty: item.difficulty,
        type: item.type,
        category_id: item.category_id,
        industry_id: 0,
        category_name: item.category_name,
      },
    })),
    total: response.data.total || 0,
    page: response.data.page || page,
    page_size: response.data.page_size || pageSize,
  }
}

/**
 * 拉取当前用户的笔记列表。
 */
async function fetchNotes(token: string, page: number, pageSize: number): Promise<PageResult<PracticeNote>> {
  const response = await requestJson<ApiEnvelope<PageResult<PracticeNote>>>(`/user/notes?page=${page}&page_size=${pageSize}`, {
    token,
  })

  if (!isSuccessCode(response.code) || !response.data) {
    throw new Error(response.message || '获取笔记列表失败')
  }

  return response.data
}

/**
 * 在当前用户的笔记列表中查找某道题已有的笔记，必要时顺序翻页直到命中。
 */
async function fetchQuestionNote(token: string, questionId: number): Promise<PracticeNote | null> {
  const pageSize = 100
  let page = 1
  let total = 0

  do {
    const notes = await fetchNotes(token, page, pageSize)
    total = notes.total

    const matched = notes.list.find((item) => item.question_id === questionId)
    if (matched) {
      return matched
    }

    page += 1
  } while ((page - 1) * pageSize < total)

  return null
}

/**
 * 切换题目收藏状态，并返回最新是否收藏的结果。
 */
async function toggleFavoriteRequest(token: string, questionId: number): Promise<boolean> {
  const response = await requestJson<ApiEnvelope<{ is_favorited: boolean }>>(`/questions/${questionId}/favorite`, {
    method: 'POST',
    token,
    body: {},
  })

  if (!isSuccessCode(response.code) || !response.data) {
    throw new Error(response.message || '收藏操作失败')
  }

  return Boolean(response.data.is_favorited)
}

/**
 * 创建题目笔记，用于普通题和代码题的过程沉淀。
 */
async function createQuestionNote(token: string, questionId: number, title: string, content: string): Promise<PracticeNote> {
  const response = await requestJson<ApiEnvelope<PracticeNote>>('/user/notes', {
    method: 'POST',
    token,
    body: {
      question_id: questionId,
      title,
      content,
    },
  })

  if (!isSuccessCode(response.code) || !response.data) {
    throw new Error(response.message || '创建笔记失败')
  }

  return response.data
}

/**
 * 更新已有题目笔记，避免重复创建同题多份内容。
 */
async function updateQuestionNote(token: string, noteId: number, title: string, content: string): Promise<void> {
  const response = await requestJson<ApiEnvelope<null>>(`/user/notes/${noteId}`, {
    method: 'PUT',
    token,
    body: {
      title,
      content,
    },
  })

  if (!isSuccessCode(response.code)) {
    throw new Error(response.message || '更新笔记失败')
  }
}

/**
 * 删除指定笔记，供笔记列表与题目页共用。
 */
async function deleteQuestionNote(token: string, noteId: number): Promise<void> {
  const response = await requestJson<ApiEnvelope<null>>(`/user/notes/${noteId}`, {
    method: 'DELETE',
    token,
  })

  if (!isSuccessCode(response.code)) {
    throw new Error(response.message || '删除笔记失败')
  }
}

/**
 * 提交题目答案，并返回统一的判题结果结构。
 */
async function submitAnswerRequest(
  token: string,
  questionId: number,
  answer: string,
  timeSpent: number,
  language?: string,
): Promise<SubmitAnswerResult> {
  const response = await requestJson<ApiEnvelope<SubmitAnswerResult>>(`/questions/${questionId}/submit`, {
    method: 'POST',
    token,
    body: {
      answer,
      time_spent: timeSpent,
      language,
    },
  })

  if (!isSuccessCode(response.code) || !response.data) {
    throw new Error(response.message || '提交答案失败')
  }

  return response.data
}

async function runCodeRequest(token: string, questionId: number, answer: string, language?: string): Promise<RunCodeResult> {
  const response = await requestJson<ApiEnvelope<{ output: string; success: boolean; execution_time_ms: number }>>(`/questions/${questionId}/run`, {
    method: 'POST',
    token,
    body: { code: answer, language },
  })

  if (!isSuccessCode(response.code) || !response.data) {
    throw new Error(response.message || '运行代码失败')
  }

  return {
    output: response.data.output || '',
    passed: response.data.success || false,
  }
}

/* ------------------------------------------------------------------ */
/*  视觉 token（做题详情页重构）                                        */
/* ------------------------------------------------------------------ */

const THEME = {
  bg: '#fafafa',
  cardBg: '#ffffff',
  primary: '#f97316',
  primaryDark: '#ea580c',
  primaryLight: '#fff7ed',
  accent: '#3b82f6',
  textMain: '#1c1917',
  textSecondary: '#57534e',
  textMuted: '#a8a29e',
  border: '#e7e5e4',
  success: '#22c55e',
  warning: '#f59e0b',
  danger: '#ef4444',
  shadow: '0 1px 3px rgba(0,0,0,0.04)',
  shadowCard: '0 4px 20px rgba(0,0,0,0.06)',
  radius: 16,
} as const

const glassCard = {
  background: 'rgba(255,255,255,0.85)',
  backdropFilter: 'blur(20px) saturate(180%)',
  WebkitBackdropFilter: 'blur(20px) saturate(180%)',
  borderRadius: THEME.radius,
  border: '1px solid rgba(255,255,255,0.6)',
  boxShadow: THEME.shadowCard,
} as CSSProperties

const solidCard = {
  background: THEME.cardBg,
  borderRadius: THEME.radius,
  border: `1px solid ${THEME.border}`,
  boxShadow: THEME.shadow,
} as CSSProperties

const difficultyColorMap: Record<string, string> = {
  easy: THEME.success,
  medium: THEME.warning,
  hard: THEME.danger,
}

const difficultyBgMap: Record<string, string> = {
  easy: '#f0fdf4',
  medium: '#fffbeb',
  hard: '#fef2f2',
}

/**
 * 提供题目页与编辑器页共用的笔记面板，支持创建、更新和删除。
 */
function QuestionNotePanel(props: { questionId: number; questionTitle: string; token: string | null }) {
  const queryClient = useQueryClient()
  const [title, setTitle] = useState(props.questionTitle)
  const [content, setContent] = useState('')
  const [message, setMessage] = useState('未保存')
  const [saving, setSaving] = useState(false)

  const noteQuery = useQuery({
    queryKey: ['practice-question-note', props.questionId, props.token],
    queryFn: () => fetchQuestionNote(props.token as string, props.questionId),
    enabled: Boolean(props.token),
    staleTime: 5 * 60 * 1000,
    refetchOnWindowFocus: false,
  })

  useEffect(() => {
    setTitle(noteQuery.data?.title || props.questionTitle)
    setContent(noteQuery.data?.content || '')
  }, [noteQuery.data, props.questionTitle])

  /**
   * 保存当前题目笔记，存在则更新，不存在则创建。
   */
  async function handleSave() {
    if (!props.token) {
      requestLoginPrompt(readCurrentBrowserPath(), 'missing')
      return
    }

    if (!content.trim()) {
      setMessage('请输入笔记内容')
      return
    }

    setSaving(true)
    try {
      if (noteQuery.data?.id) {
        await updateQuestionNote(props.token, noteQuery.data.id, title.trim() || props.questionTitle, content.trim())
      } else {
        await createQuestionNote(props.token, props.questionId, title.trim() || props.questionTitle, content.trim())
      }

      setMessage('笔记已保存')
      await queryClient.invalidateQueries({ queryKey: ['practice-question-note', props.questionId, props.token] })
      await queryClient.invalidateQueries({ queryKey: ['practice-notes'] })
    } catch (error) {
      if (!useAuthStore.getState().accessToken) {
        requestLoginPrompt(readCurrentBrowserPath(), 'expired')
        return
      }
      setMessage(extractErrorMessage(error, '保存笔记失败'))
    } finally {
      setSaving(false)
    }
  }

  /**
   * 删除当前题目的已有笔记，并同步清空编辑内容。
   */
  async function handleDelete() {
    if (!props.token) {
      requestLoginPrompt(readCurrentBrowserPath(), 'missing')
      return
    }

    if (!noteQuery.data?.id) {
      setMessage('当前没有可删除的笔记')
      return
    }

    setSaving(true)
    try {
      await deleteQuestionNote(props.token, noteQuery.data.id)
      setContent('')
      setTitle(props.questionTitle)
      setMessage('笔记已删除')
      await queryClient.invalidateQueries({ queryKey: ['practice-question-note', props.questionId, props.token] })
      await queryClient.invalidateQueries({ queryKey: ['practice-notes'] })
    } catch (error) {
      if (!useAuthStore.getState().accessToken) {
        requestLoginPrompt(readCurrentBrowserPath(), 'expired')
        return
      }
      setMessage(extractErrorMessage(error, '删除笔记失败'))
    } finally {
      setSaving(false)
    }
  }

  return (
    <div style={{ ...solidCard, padding: 24 }}>
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 20 }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
          <div
            style={{
              width: 36,
              height: 36,
              borderRadius: 10,
              background: THEME.primaryLight,
              color: THEME.primary,
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              fontSize: 16,
            }}
          >
            <EditOutlined />
          </div>
          <div>
            <div style={{ fontSize: 15, fontWeight: 700, color: THEME.textMain }}>题目笔记</div>
            <div style={{ fontSize: 12, color: THEME.textMuted }}>
              {noteQuery.data ? '已有笔记，可直接编辑' : '新建笔记，记录思路与总结'}
            </div>
          </div>
        </div>
        {noteQuery.data ? (
          <Tag
            style={{
              borderRadius: 8,
              fontSize: 12,
              color: THEME.success,
              background: '#f0fdf4',
              border: 'none',
              fontWeight: 600,
            }}
          >
            <CheckCircleOutlined style={{ marginRight: 4 }} />
            已保存
          </Tag>
        ) : (
          <Tag
            style={{
              borderRadius: 8,
              fontSize: 12,
              color: THEME.textMuted,
              background: '#fafaf9',
              border: `1px solid ${THEME.border}`,
            }}
          >
            未保存
          </Tag>
        )}
      </div>

      <div style={{ marginBottom: 16 }}>
        <div style={{ fontSize: 13, fontWeight: 600, color: THEME.textMain, marginBottom: 6 }}>标题</div>
        <input
          value={title}
          onChange={(event) => setTitle(event.target.value)}
          placeholder="输入笔记标题"
          disabled={!props.token}
          style={{
            width: '100%',
            padding: '10px 14px',
            borderRadius: 10,
            border: `1px solid ${THEME.border}`,
            fontSize: 14,
            color: THEME.textMain,
            background: props.token ? '#fff' : '#fafaf9',
            outline: 'none',
            transition: 'border-color 0.2s',
          }}
          onFocus={(e) => { e.currentTarget.style.borderColor = THEME.primary }}
          onBlur={(e) => { e.currentTarget.style.borderColor = THEME.border }}
        />
      </div>

      <div style={{ marginBottom: 16 }}>
        <div style={{ fontSize: 13, fontWeight: 600, color: THEME.textMain, marginBottom: 6 }}>内容</div>
        <textarea
          value={content}
          onChange={(event) => setContent(event.target.value)}
          rows={8}
          placeholder={props.token ? '记录思路、易错点或解题总结' : '登录后可记录笔记'}
          disabled={!props.token}
          style={{
            width: '100%',
            padding: '10px 14px',
            borderRadius: 10,
            border: `1px solid ${THEME.border}`,
            fontSize: 14,
            color: THEME.textMain,
            background: props.token ? '#fff' : '#fafaf9',
            outline: 'none',
            resize: 'vertical',
            lineHeight: 1.6,
            transition: 'border-color 0.2s',
          }}
          onFocus={(e) => { e.currentTarget.style.borderColor = THEME.primary }}
          onBlur={(e) => { e.currentTarget.style.borderColor = THEME.border }}
        />
      </div>

      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', flexWrap: 'wrap', gap: 10 }}>
        <span style={{ fontSize: 13, color: THEME.textMuted }}>{message}</span>
        <div style={{ display: 'flex', gap: 10 }}>
          {noteQuery.data?.id ? (
            <button
              type="button"
              disabled={saving}
              onClick={() => void handleDelete()}
              style={{
                padding: '8px 16px',
                borderRadius: 10,
                border: `1px solid ${THEME.border}`,
                background: '#fff',
                color: THEME.danger,
                fontSize: 13,
                fontWeight: 600,
                cursor: 'pointer',
                display: 'flex',
                alignItems: 'center',
                gap: 6,
              }}
              onMouseEnter={(e) => { e.currentTarget.style.background = '#fef2f2' }}
              onMouseLeave={(e) => { e.currentTarget.style.background = '#fff' }}
            >
              <DeleteOutlined />
              删除
            </button>
          ) : null}
          <button
            type="button"
            disabled={saving || !props.token}
            onClick={() => void handleSave()}
            style={{
              padding: '8px 16px',
              borderRadius: 10,
              border: 'none',
              background: THEME.primary,
              color: '#fff',
              fontSize: 13,
              fontWeight: 600,
              cursor: 'pointer',
              display: 'flex',
              alignItems: 'center',
              gap: 6,
              boxShadow: '0 4px 12px rgba(249,115,22,0.25)',
            }}
          >
            <SaveOutlined />
            {saving ? '保存中...' : '保存笔记'}
          </button>
        </div>
      </div>
    </div>
  )
}

/**
 * 根据练习分析里的错因标签展示可继续深挖的专题卡片。
 */
function MistakeTopicHighlights(props: { tags: string[]; title?: string }) {
  const accessToken = useAuthStore((state) => state.accessToken)
  const topicsQuery = useQuery({
    queryKey: ['mistake-topics-catalog', accessToken],
    queryFn: () => fetchMistakeTopics([], accessToken),
    enabled: props.tags.length > 0,
    staleTime: 5 * 60 * 1000,
  })

  const matchedTopics = useMemo(
    () => pickMistakeTopicsByTags(props.tags, topicsQuery.data || []),
    [props.tags, topicsQuery.data],
  )

  if (!props.tags.length) {
    return null
  }

  return (
    <div style={{ marginTop: 24 }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 12 }}>
        <BulbOutlined style={{ fontSize: 16, color: THEME.warning }} />
        <span style={{ fontSize: 15, fontWeight: 700, color: THEME.textMain }}>
          {props.title || '相关错因专题'}
        </span>
      </div>
      {topicsQuery.isLoading ? (
        <div style={{ padding: 16, textAlign: 'center' }}>
          <Spin size="small" />
          <p style={{ marginTop: 8, fontSize: 13, color: THEME.textMuted }}>正在加载错因专题...</p>
        </div>
      ) : null}
      {topicsQuery.isError ? (
        <div style={{ padding: 16, borderRadius: 12, background: '#fef2f2', color: THEME.danger, fontSize: 13 }}>
          {extractErrorMessage(topicsQuery.error, '错因专题加载失败')}
        </div>
      ) : null}
      {matchedTopics.length ? (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
          {matchedTopics.map((topic) => (
            <div
              key={topic.code}
              style={{
                ...solidCard,
                padding: '16px 18px',
                cursor: 'pointer',
                transition: 'all 0.2s ease',
              }}
              onMouseEnter={(e) => {
                e.currentTarget.style.borderColor = THEME.primary
                e.currentTarget.style.boxShadow = '0 4px 16px rgba(249,115,22,0.1)'
              }}
              onMouseLeave={(e) => {
                e.currentTarget.style.borderColor = THEME.border
                e.currentTarget.style.boxShadow = THEME.shadow
              }}
            >
              <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 6 }}>
                <strong style={{ fontSize: 14, color: THEME.textMain }}>{topic.title}</strong>
                <Tag
                  style={{
                    borderRadius: 8,
                    fontSize: 11,
                    color: THEME.primary,
                    background: THEME.primaryLight,
                    border: 'none',
                    fontWeight: 600,
                  }}
                >
                  {topic.tag}
                </Tag>
              </div>
              <p style={{ margin: '0 0 10px', fontSize: 13, color: THEME.textSecondary, lineHeight: 1.5 }}>
                {topic.problem_pattern}
              </p>
              <Link
                to={resolveMistakeTopicRoute()}
                params={{ topicCode: topic.code }}
                style={{
                  fontSize: 13,
                  fontWeight: 600,
                  color: THEME.primary,
                  textDecoration: 'none',
                  display: 'inline-flex',
                  alignItems: 'center',
                  gap: 4,
                }}
              >
                打开专题 <RightOutlined style={{ fontSize: 10 }} />
              </Link>
            </div>
          ))}
        </div>
      ) : null}
    </div>
  )
}

/* ------------------------------------------------------------------ */
/*  辅助组件                                                            */
/* ------------------------------------------------------------------ */

function InfoRow(props: { label: string; value: string }) {
  return (
    <div
      style={{
        display: 'flex',
        flexDirection: 'column',
        gap: 4,
        padding: '12px 16px',
        borderRadius: 10,
        background: '#fafaf9',
        border: `1px solid ${THEME.border}`,
      }}
    >
      <span style={{ fontSize: 12, fontWeight: 600, color: THEME.textMuted, textTransform: 'uppercase', letterSpacing: 0.5 }}>
        {props.label}
      </span>
      <span style={{ fontSize: 14, color: THEME.textMain, lineHeight: 1.6 }}>{props.value}</span>
    </div>
  )
}

function ListCard(props: { title: string; items: string[]; icon?: React.ReactNode }) {
  return (
    <div style={{ marginTop: 8 }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 6, marginBottom: 8 }}>
        <span style={{ fontSize: 14, color: THEME.primary }}>{props.icon}</span>
        <span style={{ fontSize: 13, fontWeight: 700, color: THEME.textMain }}>{props.title}</span>
      </div>
      <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
        {props.items.map((item) => (
          <div
            key={item}
            style={{
              display: 'flex',
              alignItems: 'flex-start',
              gap: 8,
              padding: '10px 14px',
              borderRadius: 10,
              background: '#fff',
              border: `1px solid ${THEME.border}`,
              fontSize: 13,
              color: THEME.textSecondary,
              lineHeight: 1.5,
            }}
          >
            <span
              style={{
                width: 6,
                height: 6,
                borderRadius: '50%',
                background: THEME.primary,
                marginTop: 6,
                flexShrink: 0,
              }}
            />
            {item}
          </div>
        ))}
      </div>
    </div>
  )
}

/**
 * 提供题目详情与答题页，支持单选、多选和主观题提交。
 */
export function PracticeQuestionPage() {
  const queryClient = useQueryClient()
  const { questionId } = useParams({ from: '/practice/$questionId' })
  const accessToken = useAuthStore((state) => state.accessToken)
  const [singleAnswer, setSingleAnswer] = useState('')
  const [multiAnswers, setMultiAnswers] = useState<string[]>([])
  const [textAnswer, setTextAnswer] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [submitResult, setSubmitResult] = useState<SubmitAnswerResult | null>(null)
  const [submitMessage, setSubmitMessage] = useState('等待提交')
  const [favoriteMessage, setFavoriteMessage] = useState('未操作')
  const [favoriteState, setFavoriteState] = useState(false)
  const [isFavPressed, setIsFavPressed] = useState(false)
  const [showParticles, setShowParticles] = useState(false)
  const [startedAt] = useState(() => Date.now())

  const detailQuery = useQuery({
    queryKey: ['practice-question-detail', questionId, accessToken],
    queryFn: () => fetchQuestionDetail(questionId, accessToken),
    staleTime: 5 * 60 * 1000,
    refetchOnWindowFocus: false,
  })
  const industriesQuery = useFrontendIndustriesQuery()

  const question = detailQuery.data
  const practiceAnalysis = useMemo(
    () => parsePracticeAnalysis(submitResult?.ai_analysis),
    [submitResult?.ai_analysis],
  )
  const options = useMemo(() => parseQuestionOptions(question?.options_json), [question?.options_json])
  const questionIndustry = useMemo(
    () => findFrontendIndustryById(industriesQuery.data || [], question?.industry_id),
    [industriesQuery.data, question?.industry_id],
  )
  const questionIndustryLabel = questionIndustry
    ? formatFrontendIndustryLabel(questionIndustry, questionIndustry.code)
    : (question?.industry_id ? `方向 #${question.industry_id}` : '未标注方向')

  useEffect(() => {
    setSingleAnswer('')
    setMultiAnswers([])
    setTextAnswer('')
    setSubmitResult(null)
    setSubmitMessage('等待提交')
    setFavoriteMessage('未操作')
  }, [questionId])

  useEffect(() => {
    setFavoriteState(Boolean(question?.is_favorited))
  }, [question?.is_favorited])

  useEffect(() => {
    if (!questionIndustry?.code) {
      return
    }

    persistSelectedFrontendIndustryCode(questionIndustry.code)
  }, [questionIndustry?.code])

  /**
   * 切换多选题选项时，保持前端提交值与后端逗号分隔格式一致。
   */
  function toggleMultiAnswer(label: string) {
    setMultiAnswers((current) => (
      current.includes(label)
        ? current.filter((item) => item !== label)
        : [...current, label]
    ))
  }

  /**
   * 提交当前题目答案，并展示判题结果或解析内容。
   */
  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()

    if (!question) {
      return
    }

    if (!accessToken) {
      requestLoginPrompt(readCurrentBrowserPath(), 'missing')
      return
    }

    const answer = buildSubmittedAnswer(question.type, singleAnswer, multiAnswers, textAnswer)
    if (!answer) {
      setSubmitMessage('请先填写答案')
      return
    }

    setSubmitting(true)
    setSubmitResult(null)
    setSubmitMessage('提交中...')

    try {
      const result = await submitAnswerRequest(
        accessToken,
        question.id,
        answer,
        Math.max(Math.round((Date.now() - startedAt) / 1000), 1),
      )

      setSubmitResult(result)
      setSubmitMessage(result.is_correct ? '回答正确' : '回答完成')
      await queryClient.invalidateQueries({ queryKey: ['practice-stats'] })
      await queryClient.invalidateQueries({ queryKey: ['practice-wrong'] })
      await queryClient.invalidateQueries({ queryKey: ['practice-recommendations'] })
      await queryClient.invalidateQueries({ queryKey: ['practice', 'questions'] })
    } catch (error) {
      if (!useAuthStore.getState().accessToken) {
        requestLoginPrompt(readCurrentBrowserPath(), 'expired')
        return
      }
      setSubmitMessage(extractErrorMessage(error, '提交答案失败'))
    } finally {
      setSubmitting(false)
    }
  }

  /**
   * 切换当前题目的收藏状态，并同步更新收藏页缓存。
   */
  async function handleToggleFavorite() {
    if (!accessToken || !question) {
      requestLoginPrompt(readCurrentBrowserPath(), 'missing')
      return
    }

    try {
      const nextState = await toggleFavoriteRequest(accessToken, question.id)
      setFavoriteState(nextState)
      setFavoriteMessage(nextState ? '已加入收藏夹' : '已移出收藏夹')
      if (nextState) {
        setShowParticles(true)
        setTimeout(() => setShowParticles(false), 1000)
      }
      await queryClient.invalidateQueries({ queryKey: ['practice-favorites'] })
      await queryClient.invalidateQueries({ queryKey: ['practice-question-detail', question.id] })
      await queryClient.invalidateQueries({ queryKey: ['practice', 'questions'] })
    } catch (error) {
      if (!useAuthStore.getState().accessToken) {
        requestLoginPrompt(readCurrentBrowserPath(), 'expired')
        return
      }
      setFavoriteMessage(extractErrorMessage(error, '收藏操作失败'))
    }
  }

  return (
    <div style={{ background: THEME.bg, minHeight: '100vh' }}>
      {/* ===== Back Link ===== */}
      <div style={{ padding: '24px 24px 0', maxWidth: 1200, margin: '0 auto' }}>
        <Link
          to="/practice"
          style={{
            display: 'inline-flex',
            alignItems: 'center',
            gap: 6,
            color: THEME.textSecondary,
            fontSize: 14,
            fontWeight: 500,
            textDecoration: 'none',
            transition: 'color 0.2s',
          }}
          onMouseEnter={(e) => { e.currentTarget.style.color = THEME.primary }}
          onMouseLeave={(e) => { e.currentTarget.style.color = THEME.textSecondary }}
        >
          <ArrowLeftOutlined />
          返回题库
        </Link>
      </div>

      {/* ===== Hero Header ===== */}
      <div style={{ padding: '16px 24px 24px', maxWidth: 1200, margin: '0 auto' }}>
        <div
          style={{
            ...glassCard,
            padding: '24px 28px',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            flexWrap: 'wrap',
            gap: 16,
          }}
        >
          <div style={{ flex: 1, minWidth: 280 }}>
            <div
              style={{
                display: 'inline-flex',
                alignItems: 'center',
                gap: 6,
                padding: '4px 12px',
                borderRadius: 20,
                background: THEME.primaryLight,
                color: THEME.primaryDark,
                fontSize: 12,
                fontWeight: 700,
                marginBottom: 12,
              }}
            >
              <FireOutlined />
              刷题详情
            </div>
            <h1
              style={{
                margin: 0,
                fontSize: 'clamp(24px, 3vw, 32px)',
                fontWeight: 800,
                color: THEME.textMain,
                lineHeight: 1.2,
                letterSpacing: -0.5,
              }}
            >
              {question?.title || `题目 #${questionId}`}
            </h1>
            <p style={{ margin: '8px 0 0', fontSize: 14, color: THEME.textSecondary, lineHeight: 1.6 }}>
              {question
                ? `当前为${questionTypeLabel(question.type)}，请仔细阅读题干后作答。`
                : '题目加载中，请稍候...'}
            </p>
          </div>
          <div
            style={{
              width: 72,
              height: 72,
              borderRadius: 20,
              background: THEME.primaryLight,
              color: THEME.primary,
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              fontSize: 28,
              fontWeight: 800,
              flexShrink: 0,
            }}
          >
            #{questionId}
          </div>
        </div>
      </div>

      {/* ===== Loading / Error ===== */}
      {detailQuery.isLoading ? (
        <div style={{ maxWidth: 1200, margin: '0 auto', padding: '0 24px' }}>
          <div style={{ ...solidCard, padding: 40, textAlign: 'center' }}>
            <Spin size="large" tip="题目详情加载中..." />
          </div>
        </div>
      ) : null}

      {detailQuery.isError ? (
        <div style={{ maxWidth: 1200, margin: '0 auto', padding: '0 24px' }}>
          <div
            style={{
              ...solidCard,
              padding: 32,
              textAlign: 'center',
              borderColor: 'rgba(239,68,68,0.2)',
              background: '#fef2f2',
            }}
          >
            <ExclamationCircleOutlined style={{ fontSize: 40, color: THEME.danger, marginBottom: 12 }} />
            <div style={{ fontSize: 16, fontWeight: 700, color: THEME.textMain, marginBottom: 6 }}>
              题目加载失败
            </div>
            <div style={{ fontSize: 14, color: THEME.textSecondary }}>
              {detailQuery.error instanceof Error ? detailQuery.error.message : '题目详情加载失败'}
            </div>
          </div>
        </div>
      ) : null}

      {/* ===== Main Content ===== */}
      {question ? (
        <div
          style={{
            maxWidth: 1200,
            margin: '0 auto',
            padding: '0 24px 64px',
            display: 'grid',
            gridTemplateColumns: '1fr 340px',
            gap: 24,
          }}
        >
          {/* Left Column */}
          <div style={{ display: 'flex', flexDirection: 'column', gap: 24 }}>
            {/* Question Info Card */}
            <div style={{ ...solidCard, padding: '24px 28px' }}>
              <div
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'space-between',
                  marginBottom: 16,
                  flexWrap: 'wrap',
                  gap: 10,
                }}
              >
                <div style={{ display: 'flex', alignItems: 'center', gap: 10, flexWrap: 'wrap' }}>
                  <Tag
                    style={{
                      borderRadius: 8,
                      fontSize: 12,
                      fontWeight: 600,
                      color: difficultyColorMap[question.difficulty] || THEME.textMuted,
                      background: difficultyBgMap[question.difficulty] || '#fafaf9',
                      border: 'none',
                      padding: '2px 10px',
                    }}
                  >
                    {difficultyLabel(question.difficulty)}
                  </Tag>
                  <Tag
                    style={{
                      borderRadius: 8,
                      fontSize: 12,
                      fontWeight: 600,
                      color: THEME.accent,
                      background: '#eff6ff',
                      border: 'none',
                      padding: '2px 10px',
                    }}
                  >
                    {questionTypeLabel(question.type)}
                  </Tag>
                  <span style={{ fontSize: 13, color: THEME.textMuted }}>
                    {questionIndustryLabel} · {question.category_name || `分类 #${question.category_id}`}
                  </span>
                </div>
              </div>

              {question.tag_list?.length ? (
                <div style={{ display: 'flex', flexWrap: 'wrap', gap: 8, marginBottom: 16 }}>
                  {question.tag_list.map((tag) => (
                    <Tag
                      key={`question-tag-${tag}`}
                      style={{
                        borderRadius: 8,
                        fontSize: 12,
                        color: THEME.textSecondary,
                        background: '#fafaf9',
                        border: `1px solid ${THEME.border}`,
                      }}
                    >
                      {tag}
                    </Tag>
                  ))}
                </div>
              ) : null}

              <div
                style={{
                  fontSize: 15,
                  color: THEME.textMain,
                  lineHeight: 1.7,
                  background: '#fafaf9',
                  borderRadius: 12,
                  padding: '20px 24px',
                  border: `1px solid ${THEME.border}`,
                }}
              >
                {question.content}
              </div>

              <div style={{ display: 'flex', alignItems: 'center', gap: 12, marginTop: 20, flexWrap: 'wrap' }}>
                <div style={{ position: 'relative' }}>
                  <button
                    type="button"
                    onClick={() => void handleToggleFavorite()}
                    onMouseDown={() => setIsFavPressed(true)}
                    onMouseUp={() => setIsFavPressed(false)}
                    onMouseLeave={() => setIsFavPressed(false)}
                    style={{
                      display: 'flex',
                      alignItems: 'center',
                      gap: 6,
                      padding: '8px 16px',
                      borderRadius: 10,
                      border: favoriteState ? 'none' : `1px solid ${THEME.border}`,
                      background: favoriteState ? THEME.primary : '#fff',
                      color: favoriteState ? '#fff' : THEME.textSecondary,
                      fontSize: 13,
                      fontWeight: 600,
                      cursor: 'pointer',
                      transition: 'all 0.15s cubic-bezier(0.4,0,0.2,1)',
                      boxShadow: favoriteState ? '0 4px 14px rgba(249,115,22,0.4)' : 'none',
                      transform: isFavPressed ? 'scale(0.92)' : 'scale(1)',
                    }}
                  >
                    {favoriteState ? <StarFilled /> : <StarOutlined />}
                    {favoriteState ? '已收藏' : '加入收藏'}
                  </button>
                  {showParticles && (
                    <div style={{ position: 'absolute', top: '50%', left: '50%', pointerEvents: 'none' }}>
                      {Array.from({ length: 8 }).map((_, i) => (
                        <div
                          key={i}
                          style={{
                            position: 'absolute',
                            width: 6,
                            height: 6,
                            borderRadius: '50%',
                            background: THEME.primary,
                            animation: `particle-${i} 0.6s ease-out forwards`,
                            opacity: 0,
                          }}
                        />
                      ))}
                      <style>{`
                        ${Array.from({ length: 8 }).map((_, i) => {
                          const angle = (i * 45) * Math.PI / 180
                          const distance = 30 + Math.random() * 20
                          return `
                            @keyframes particle-${i} {
                              0% { transform: translate(0, 0) scale(1); opacity: 1; }
                              100% { transform: translate(${Math.cos(angle) * distance}px, ${Math.sin(angle) * distance}px) scale(0); opacity: 0; }
                            }
                          `
                        }).join('')}
                      `}</style>
                    </div>
                  )}
                </div>
                {question.type === 'code' ? (
                  <Link
                    to="/practice/editor/$questionId"
                    params={{ questionId: String(question.id) }}
                    style={{
                      display: 'inline-flex',
                      alignItems: 'center',
                      gap: 6,
                      padding: '8px 16px',
                      borderRadius: 10,
                      border: `1px solid ${THEME.border}`,
                      background: '#fff',
                      color: THEME.textSecondary,
                      fontSize: 13,
                      fontWeight: 600,
                      textDecoration: 'none',
                    }}
                  >
                    <ThunderboltOutlined />
                    进入代码编辑器
                  </Link>
                ) : null}
                <span style={{ fontSize: 12, color: THEME.textMuted, marginLeft: 'auto' }}>
                  {favoriteMessage}
                </span>
              </div>
            </div>

            {/* Answer Area Card */}
            <form style={{ ...solidCard, padding: '24px 28px' }} onSubmit={handleSubmit}>
              <div style={{ display: 'flex', alignItems: 'center', gap: 10, marginBottom: 20 }}>
                <div
                  style={{
                    width: 36,
                    height: 36,
                    borderRadius: 10,
                    background: THEME.primaryLight,
                    color: THEME.primary,
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'center',
                    fontSize: 16,
                  }}
                >
                  <EditOutlined />
                </div>
                <div>
                  <div style={{ fontSize: 15, fontWeight: 700, color: THEME.textMain }}>作答区</div>
                  <div style={{ fontSize: 12, color: THEME.textMuted }}>
                    {question.type === 'choice'
                      ? '请选择唯一正确选项'
                      : question.type === 'multi'
                        ? '请选择所有正确选项'
                        : '请输入你的分析或答案'}
                  </div>
                </div>
              </div>

              {question.type === 'choice' ? (
                <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
                  {options.map((option) => {
                    const isSelected = singleAnswer === option.label
                    return (
                      <label
                        key={option.label}
                        style={{
                          display: 'flex',
                          alignItems: 'flex-start',
                          gap: 12,
                          padding: '14px 18px',
                          borderRadius: 12,
                          border: isSelected ? `2px solid ${THEME.primary}` : `1px solid ${THEME.border}`,
                          background: isSelected ? THEME.primaryLight : '#fff',
                          cursor: 'pointer',
                          transition: 'all 0.2s ease',
                          position: 'relative',
                        }}
                        onMouseEnter={(e) => {
                          if (!isSelected) {
                            e.currentTarget.style.borderColor = THEME.primary
                            e.currentTarget.style.background = '#fff7ed'
                          }
                        }}
                        onMouseLeave={(e) => {
                          if (!isSelected) {
                            e.currentTarget.style.borderColor = THEME.border
                            e.currentTarget.style.background = '#fff'
                          }
                        }}
                      >
                        <input
                          type="radio"
                          name="choice-answer"
                          checked={isSelected}
                          onChange={() => setSingleAnswer(option.label)}
                          style={{ marginTop: 3, accentColor: THEME.primary, cursor: 'pointer' }}
                        />
                        <span style={{ fontSize: 14, color: THEME.textMain, lineHeight: 1.5 }}>
                          <strong style={{ color: THEME.primary, marginRight: 6 }}>{option.label}.</strong>
                          {option.text}
                        </span>
                      </label>
                    )
                  })}
                </div>
              ) : null}

              {question.type === 'multi' ? (
                <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
                  {options.map((option) => {
                    const isSelected = multiAnswers.includes(option.label)
                    return (
                      <label
                        key={option.label}
                        style={{
                          display: 'flex',
                          alignItems: 'flex-start',
                          gap: 12,
                          padding: '14px 18px',
                          borderRadius: 12,
                          border: isSelected ? `2px solid ${THEME.primary}` : `1px solid ${THEME.border}`,
                          background: isSelected ? THEME.primaryLight : '#fff',
                          cursor: 'pointer',
                          transition: 'all 0.2s ease',
                        }}
                        onMouseEnter={(e) => {
                          if (!isSelected) {
                            e.currentTarget.style.borderColor = THEME.primary
                            e.currentTarget.style.background = '#fff7ed'
                          }
                        }}
                        onMouseLeave={(e) => {
                          if (!isSelected) {
                            e.currentTarget.style.borderColor = THEME.border
                            e.currentTarget.style.background = '#fff'
                          }
                        }}
                      >
                        <input
                          type="checkbox"
                          checked={isSelected}
                          onChange={() => toggleMultiAnswer(option.label)}
                          style={{ marginTop: 3, accentColor: THEME.primary, cursor: 'pointer' }}
                        />
                        <span style={{ fontSize: 14, color: THEME.textMain, lineHeight: 1.5 }}>
                          <strong style={{ color: THEME.primary, marginRight: 6 }}>{option.label}.</strong>
                          {option.text}
                        </span>
                      </label>
                    )
                  })}
                </div>
              ) : null}

              {question.type === 'subjective' ? (
                <div>
                  <div style={{ fontSize: 13, fontWeight: 600, color: THEME.textMain, marginBottom: 8 }}>你的答案</div>
                  <textarea
                    value={textAnswer}
                    onChange={(event) => setTextAnswer(event.target.value)}
                    placeholder="请输入你的分析或答案"
                    rows={10}
                    style={{
                      width: '100%',
                      padding: '12px 16px',
                      borderRadius: 12,
                      border: `1px solid ${THEME.border}`,
                      fontSize: 14,
                      color: THEME.textMain,
                      lineHeight: 1.7,
                      outline: 'none',
                      resize: 'vertical',
                      transition: 'border-color 0.2s',
                    }}
                    onFocus={(e) => { e.currentTarget.style.borderColor = THEME.primary }}
                    onBlur={(e) => { e.currentTarget.style.borderColor = THEME.border }}
                  />
                </div>
              ) : null}

              {!accessToken ? (
                <div
                  style={{
                    marginTop: 20,
                    padding: '16px 20px',
                    borderRadius: 12,
                    background: '#fffbeb',
                    border: '1px solid #fef3c7',
                    color: THEME.warning,
                    fontSize: 13,
                    display: 'flex',
                    alignItems: 'center',
                    gap: 8,
                  }}
                >
                  <ExclamationCircleOutlined />
                  需要先登录后才能提交答案、收藏和记录笔记。
                </div>
              ) : null}

              <div
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'space-between',
                  marginTop: 24,
                  flexWrap: 'wrap',
                  gap: 12,
                }}
              >
                <Link
                  to="/practice"
                  style={{
                    fontSize: 14,
                    fontWeight: 600,
                    color: THEME.textSecondary,
                    textDecoration: 'none',
                    display: 'inline-flex',
                    alignItems: 'center',
                    gap: 6,
                  }}
                >
                  <ArrowLeftOutlined />
                  返回题目列表
                </Link>
                <button
                  type="submit"
                  disabled={submitting}
                  style={{
                    padding: '10px 28px',
                    borderRadius: 12,
                    border: 'none',
                    background: THEME.primary,
                    color: '#fff',
                    fontSize: 14,
                    fontWeight: 700,
                    cursor: 'pointer',
                    boxShadow: '0 4px 16px rgba(249,115,22,0.3)',
                    opacity: submitting ? 0.7 : 1,
                  }}
                >
                  {submitting ? '提交中...' : '提交答案'}
                </button>
              </div>
            </form>

            {/* Result Card */}
            <div style={{ ...solidCard, padding: '24px 28px' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: 10, marginBottom: 20 }}>
                <div
                  style={{
                    width: 36,
                    height: 36,
                    borderRadius: 10,
                    background: submitResult?.is_correct ? '#f0fdf4' : submitResult ? '#fef2f2' : '#fafaf9',
                    color: submitResult?.is_correct ? THEME.success : submitResult ? THEME.danger : THEME.textMuted,
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'center',
                    fontSize: 16,
                  }}
                >
                  {submitResult?.is_correct ? <CheckCircleOutlined /> : submitResult ? <CloseOutlined /> : <ClockCircleOutlined />}
                </div>
                <div>
                  <div style={{ fontSize: 15, fontWeight: 700, color: THEME.textMain }}>提交状态</div>
                  <div style={{ fontSize: 12, color: THEME.textMuted }}>{submitMessage}</div>
                </div>
              </div>

              {submitResult ? (
                <>
                  {/* Result Banner */}
                  <div
                    style={{
                      padding: '16px 20px',
                      borderRadius: 12,
                      background: submitResult.is_correct ? '#f0fdf4' : '#fef2f2',
                      border: `1px solid ${submitResult.is_correct ? 'rgba(34,197,94,0.2)' : 'rgba(239,68,68,0.2)'}`,
                      marginBottom: 20,
                      display: 'flex',
                      alignItems: 'center',
                      gap: 10,
                    }}
                  >
                    <div
                      style={{
                        width: 32,
                        height: 32,
                        borderRadius: '50%',
                        background: submitResult.is_correct ? THEME.success : THEME.danger,
                        color: '#fff',
                        display: 'flex',
                        alignItems: 'center',
                        justifyContent: 'center',
                        fontSize: 16,
                        fontWeight: 700,
                      }}
                    >
                      {submitResult.is_correct ? <CheckCircleOutlined /> : <CloseOutlined />}
                    </div>
                    <div>
                      <div style={{ fontSize: 15, fontWeight: 700, color: submitResult.is_correct ? THEME.success : THEME.danger }}>
                        {submitResult.is_correct ? '回答正确' : '回答错误'}
                      </div>
                      <div style={{ fontSize: 13, color: THEME.textSecondary }}>
                        正确答案：{submitResult.correct_answer || '未返回'}
                      </div>
                    </div>
                  </div>

                  {submitResult.explanation ? (
                    <div style={{ marginBottom: 20 }}>
                      <div style={{ fontSize: 14, fontWeight: 700, color: THEME.textMain, marginBottom: 8 }}>
                        <FileTextOutlined style={{ marginRight: 6, color: THEME.accent }} />
                        解析说明
                      </div>
                      <div
                        style={{
                          fontSize: 14,
                          color: THEME.textSecondary,
                          lineHeight: 1.7,
                          background: '#fafaf9',
                          borderRadius: 10,
                          padding: '14px 18px',
                          border: `1px solid ${THEME.border}`,
                        }}
                      >
                        {submitResult.explanation}
                      </div>
                    </div>
                  ) : null}

                  {submitResult.ai_analysis ? (
                    <div style={{ marginBottom: 20 }}>
                      <div style={{ fontSize: 14, fontWeight: 700, color: THEME.textMain, marginBottom: 8 }}>
                        <MessageOutlined style={{ marginRight: 6, color: THEME.primary }} />
                        AI 分析
                      </div>
                      <pre
                        style={{
                          margin: 0,
                          padding: '14px 18px',
                          borderRadius: 10,
                          background: '#fafaf9',
                          border: `1px solid ${THEME.border}`,
                          fontSize: 13,
                          lineHeight: 1.7,
                          color: THEME.textSecondary,
                          whiteSpace: 'pre-wrap',
                          wordBreak: 'break-word',
                          overflow: 'auto',
                          maxHeight: 400,
                        }}
                      >
                        {submitResult.ai_analysis}
                      </pre>
                    </div>
                  ) : null}

                  {/* Structured Solution */}
                  {question.solution ? (
                    <div style={{ marginBottom: 20 }}>
                      <div style={{ fontSize: 14, fontWeight: 700, color: THEME.textMain, marginBottom: 12 }}>
                        <BulbOutlined style={{ marginRight: 6, color: THEME.warning }} />
                        结构化解析
                      </div>
                      <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
                        {question.solution.summary ? (
                          <InfoRow label="题意总结" value={question.solution.summary} />
                        ) : null}
                        {question.solution.approach ? (
                          <InfoRow label="解题思路" value={question.solution.approach} />
                        ) : null}
                        {question.solution.complexity ? (
                          <InfoRow label="复杂度分析" value={question.solution.complexity} />
                        ) : null}
                      </div>
                      {question.solution.key_steps.length ? (
                        <ListCard title="关键步骤" items={question.solution.key_steps} icon=<ProfileOutlined /> />
                      ) : null}
                      {question.solution.edge_cases.length ? (
                        <ListCard title="边界条件" items={question.solution.edge_cases} icon=<ExclamationCircleOutlined /> />
                      ) : null}
                      {question.solution.common_mistakes.length ? (
                        <ListCard title="常见错法" items={question.solution.common_mistakes} icon=<CloseOutlined /> />
                      ) : null}
                    </div>
                  ) : null}

                  {/* Answer Template */}
                  {question.answer_template ? (
                    <div style={{ marginBottom: 20 }}>
                      <div style={{ fontSize: 14, fontWeight: 700, color: THEME.textMain, marginBottom: 12 }}>
                        <BookOutlined style={{ marginRight: 6, color: THEME.accent }} />
                        参考回答模板
                      </div>
                      <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
                        {question.answer_template.core_conclusion ? (
                          <InfoRow label="核心结论" value={question.answer_template.core_conclusion} />
                        ) : null}
                        {question.answer_template.sample_answer ? (
                          <InfoRow label="面试表达示例" value={question.answer_template.sample_answer} />
                        ) : null}
                      </div>
                      {question.answer_template.key_points.length ? (
                        <ListCard title="关键展开点" items={question.answer_template.key_points} icon=<ProfileOutlined /> />
                      ) : null}
                      {question.answer_template.follow_ups.length ? (
                        <ListCard title="高频追问点" items={question.answer_template.follow_ups} icon=<MessageOutlined /> />
                      ) : null}
                      {question.answer_template.pitfalls.length ? (
                        <ListCard title="易答偏点" items={question.answer_template.pitfalls} icon=<CloseOutlined /> />
                      ) : null}
                    </div>
                  ) : null}

                  <MistakeTopicHighlights
                    tags={practiceAnalysis?.mistake_tags || []}
                    title="继续深挖这些错因"
                  />
                </>
              ) : null}
            </div>
          </div>

          {/* Right Column — Sticky Notes */}
          <div style={{ position: 'sticky', top: 24, alignSelf: 'start' }}>
            <QuestionNotePanel
              questionId={question.id}
              questionTitle={question.title}
              token={accessToken}
            />
          </div>
        </div>
      ) : null}
    </div>
  )
}

/**
 * 提供代码题专用编辑器，基于 Monaco 承载输入、运行和提交。
 */
export function PracticeEditorPage() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const { questionId } = useParams({ from: '/practice/editor/$questionId' })
  const accessToken = useAuthStore((state) => state.accessToken)
  const user = useAuthStore((state) => state.user)
  const logout = useAuthStore((state) => state.logout)
  const editorContainerRef = useRef<HTMLDivElement | null>(null)
  const editorInstanceRef = useRef<any>(null)
  const monacoRef = useRef<any>(null)
  const [editorLanguage, setEditorLanguage] = useState('go')
  const [codeContent, setCodeContent] = useState(buildDefaultCodeTemplate())
  const [submitting, setSubmitting] = useState(false)
  const [running, setRunning] = useState(false)
  const [submitMessage, setSubmitMessage] = useState('等待运行')
  const [submitResult, setSubmitResult] = useState<SubmitAnswerResult | null>(null)
  const [runOutput, setRunOutput] = useState<string | null>(null)
  const [runPassed, setRunPassed] = useState<boolean | null>(null)
  const [runJudgeSummary, setRunJudgeSummary] = useState<PracticeJudgeSummary | null>(null)
  const [favoriteMessage, setFavoriteMessage] = useState('未操作')
  const [favoriteState, setFavoriteState] = useState(false)
  const [isFavPressed, setIsFavPressed] = useState(false)
  const [startedAt] = useState(() => Date.now())
  const [leftPanelWidth, setLeftPanelWidth] = useState(40)
  const [editorHeight, setEditorHeight] = useState(60)
  const [showResultPanel, setShowResultPanel] = useState(false)

  const detailQuery = useQuery({
    queryKey: ['practice-code-question-detail', questionId, accessToken],
    queryFn: () => fetchQuestionDetail(questionId, accessToken),
    staleTime: 5 * 60 * 1000,
    refetchOnWindowFocus: false,
  })
  const industriesQuery = useFrontendIndustriesQuery()

  const question = detailQuery.data
  const evaluationMode = question?.judge_config?.evaluation_mode || 'analysis_only'
  const practiceAnalysis = useMemo(
    () => parsePracticeAnalysis(submitResult?.ai_analysis),
    [submitResult?.ai_analysis],
  )
  const questionIndustry = useMemo(
    () => findFrontendIndustryById(industriesQuery.data || [], question?.industry_id),
    [industriesQuery.data, question?.industry_id],
  )
  const questionIndustryLabel = questionIndustry
    ? formatFrontendIndustryLabel(questionIndustry, questionIndustry.code)
    : (question?.industry_id ? `方向 #${question.industry_id}` : '未标注方向')

  useEffect(() => {
    setSubmitResult(null)
    setSubmitMessage('等待运行')
    setRunOutput(null)
    setRunPassed(null)
    setRunJudgeSummary(null)
    setFavoriteMessage('未操作')
  }, [questionId])

  useEffect(() => {
    setFavoriteState(Boolean(question?.is_favorited))
  }, [question?.is_favorited])

  useEffect(() => {
    if (!question?.judge_config?.default_language) {
      return
    }
    setEditorLanguage(question.judge_config.default_language)
  }, [question?.judge_config?.default_language])

  useEffect(() => {
    if (!questionIndustry?.code) {
      return
    }

    persistSelectedFrontendIndustryCode(questionIndustry.code)
  }, [questionIndustry?.code])

  useEffect(() => {
    if (!question || question.type !== 'code') {
      return
    }

    let disposed = false

    /**
     * 初始化 Monaco 编辑器实例，并把输入变化同步回页面状态。
     */
    async function initializeEditor() {
      if (!question || !editorContainerRef.current || editorInstanceRef.current) {
        return
      }

      const loader = await import('@monaco-editor/loader')
      const monaco = await loader.default.init()
      if (disposed || !editorContainerRef.current) {
        return
      }

      monacoRef.current = monaco
      const initialValue = readCodeDraft(question.id, editorLanguage, buildCodeTemplateFromQuestion(question))
      setCodeContent(initialValue)
      editorInstanceRef.current = monaco.editor.create(editorContainerRef.current, {
        value: initialValue,
        language: editorLanguage,
        theme: 'vs-dark',
        fontSize: 14,
        minimap: { enabled: false },
        automaticLayout: true,
        scrollBeyondLastLine: false,
        lineNumbers: 'on',
        tabSize: 4,
      })

      editorInstanceRef.current.onDidChangeModelContent(() => {
        setCodeContent(editorInstanceRef.current.getValue())
      })
    }

    void initializeEditor()

    return () => {
      disposed = true
      if (editorInstanceRef.current) {
        editorInstanceRef.current.dispose()
        editorInstanceRef.current = null
      }
    }
  }, [question?.id, question?.type, editorLanguage])

  useEffect(() => {
    if (!question || question.type !== 'code') {
      return
    }

    const draft = readCodeDraft(question.id, editorLanguage, buildCodeTemplateFromQuestion(question))
    setCodeContent(draft)

    if (editorInstanceRef.current && editorInstanceRef.current.getValue() !== draft) {
      editorInstanceRef.current.setValue(draft)
    }

    if (editorInstanceRef.current?.getModel() && monacoRef.current) {
      monacoRef.current.editor.setModelLanguage(editorInstanceRef.current.getModel(), editorLanguage)
    }
  }, [editorLanguage, question?.id, question?.type])

  useEffect(() => {
    if (!question || question.type !== 'code') {
      return
    }

    persistCodeDraft(question.id, editorLanguage, codeContent)
  }, [codeContent, editorLanguage, question?.id, question?.type])

  /**
   * 切换编辑器语言高亮，不改变实际提交给后端的内容。
   */
  function handleLanguageChange(value: string) {
    setEditorLanguage(value)

    if (editorInstanceRef.current?.getModel() && monacoRef.current) {
      monacoRef.current.editor.setModelLanguage(editorInstanceRef.current.getModel(), value)
    }
  }

  /**
   * 重置代码内容为默认模板，方便重新开始作答。
   */
  function handleResetCode() {
    const template = buildCodeTemplateFromQuestion(question)
    setCodeContent(template)

    if (editorInstanceRef.current) {
      editorInstanceRef.current.setValue(template)
    }
  }

  /**
   * 从代码题编辑器返回上一页；若没有可用历史记录，则退回题库首页。
   */
  function handleGoBackFromEditor() {
    if (typeof window !== 'undefined' && window.history.length > 1) {
      window.history.back()
      return
    }

    navigate({
      to: '/practice',
    })
  }

  /**
   * 运行代码：仅返回基础输出，不触发AI分析，不保存记录。
   */
  async function handleRunCode() {
    if (!question) {
      return
    }

    if (!accessToken) {
      requestLoginPrompt(readCurrentBrowserPath(), 'missing')
      return
    }

    if (!codeContent.trim()) {
      setSubmitMessage('请先输入代码')
      return
    }

    setRunning(true)
    setSubmitMessage('运行代码中...')
    setSubmitResult(null)

    try {
      const result = await runCodeRequest(accessToken, question.id, codeContent, editorLanguage)
      setRunOutput(result.output)
      setRunPassed(result.passed)
      setRunJudgeSummary(result.judge_summary || null)
      setSubmitMessage('运行完成')
    } catch (error) {
      if (!useAuthStore.getState().accessToken) {
        requestLoginPrompt(readCurrentBrowserPath(), 'expired')
        return
      }
      setSubmitMessage(extractErrorMessage(error, '运行代码失败'))
    } finally {
      setRunning(false)
    }
  }

  /**
   * 提交代码题答案，并返回AI分析结果。
   */
  async function handleSubmitCode() {
    if (!question) {
      return
    }

    if (!accessToken) {
      requestLoginPrompt(readCurrentBrowserPath(), 'missing')
      return
    }

    if (!codeContent.trim()) {
      setSubmitMessage('请先输入代码')
      return
    }

    setSubmitting(true)
    setSubmitMessage('提交代码中...')

    try {
      const result = await submitAnswerRequest(
        accessToken,
        question.id,
        codeContent,
        Math.max(Math.round((Date.now() - startedAt) / 1000), 1),
        editorLanguage,
      )

      setSubmitResult(result)
      setRunOutput(null)
      setRunPassed(null)
      setRunJudgeSummary(null)
      setSubmitMessage(result.is_correct ? '提交通过' : '提交完成')
      setShowResultPanel(true)
      await queryClient.invalidateQueries({ queryKey: ['practice-stats'] })
      await queryClient.invalidateQueries({ queryKey: ['practice-wrong'] })
      await queryClient.invalidateQueries({ queryKey: ['practice-recommendations'] })
      await queryClient.invalidateQueries({ queryKey: ['practice', 'questions'] })
    } catch (error) {
      if (!useAuthStore.getState().accessToken) {
        requestLoginPrompt(readCurrentBrowserPath(), 'expired')
        return
      }
      setSubmitMessage(extractErrorMessage(error, '提交代码失败'))
    } finally {
      setSubmitting(false)
    }
  }

  /**
   * 切换代码题收藏状态，并刷新收藏列表缓存。
   */
  async function handleToggleFavorite() {
    if (!accessToken || !question) {
      requestLoginPrompt(readCurrentBrowserPath(), 'missing')
      return
    }

    try {
      const nextState = await toggleFavoriteRequest(accessToken, question.id)
      setFavoriteState(nextState)
      setFavoriteMessage(nextState ? '已加入收藏夹' : '已移出收藏夹')
      await queryClient.invalidateQueries({ queryKey: ['practice-favorites'] })
      await queryClient.invalidateQueries({ queryKey: ['practice-code-question-detail', question.id] })
      await queryClient.invalidateQueries({ queryKey: ['practice', 'questions'] })
    } catch (error) {
      if (!useAuthStore.getState().accessToken) {
        requestLoginPrompt(readCurrentBrowserPath(), 'expired')
        return
      }
      setFavoriteMessage(extractErrorMessage(error, '收藏操作失败'))
    }
  }

  const handleVerticalResize = useCallback((e: React.MouseEvent) => {
    e.preventDefault()
    const startX = e.clientX
    const containerWidth = document.querySelector('.editor-body')?.clientWidth || window.innerWidth
    const startWidth = leftPanelWidth

    const onMouseMove = (moveEvent: MouseEvent) => {
      const delta = moveEvent.clientX - startX
      const newPercent = Math.min(60, Math.max(20, startWidth + (delta / containerWidth) * 100))
      setLeftPanelWidth(newPercent)
    }

    const onMouseUp = () => {
      document.removeEventListener('mousemove', onMouseMove)
      document.removeEventListener('mouseup', onMouseUp)
      document.body.style.cursor = ''
      document.body.style.userSelect = ''
    }

    document.body.style.cursor = 'col-resize'
    document.body.style.userSelect = 'none'
    document.addEventListener('mousemove', onMouseMove)
    document.addEventListener('mouseup', onMouseUp)
  }, [leftPanelWidth])

  const handleHorizontalResize = useCallback((e: React.MouseEvent) => {
    e.preventDefault()
    const startY = e.clientY
    const workspaceEl = document.querySelector('.editor-workspace')
    const containerHeight = workspaceEl?.clientHeight || window.innerHeight * 0.6
    const startHeight = editorHeight

    const onMouseMove = (moveEvent: MouseEvent) => {
      const delta = moveEvent.clientY - startY
      const newPercent = Math.min(85, Math.max(25, startHeight + (delta / containerHeight) * 100))
      setEditorHeight(newPercent)
    }

    const onMouseUp = () => {
      document.removeEventListener('mousemove', onMouseMove)
      document.removeEventListener('mouseup', onMouseUp)
      document.body.style.cursor = ''
      document.body.style.userSelect = ''
    }

    document.body.style.cursor = 'row-resize'
    document.body.style.userSelect = 'none'
    document.addEventListener('mousemove', onMouseMove)
    document.addEventListener('mouseup', onMouseUp)
  }, [editorHeight])

  if (detailQuery.isLoading) {
    return (
      <div className="editor-immersive">
        <header className="editor-topbar">
          <div className="editor-topbar-left">
            <button type="button" onClick={handleGoBackFromEditor}>← 退出</button>
          </div>
          <div className="editor-topbar-right">
            <span>{user?.username || '游客'}</span>
          </div>
        </header>
        <div className="editor-body" style={{ alignItems: 'center', justifyContent: 'center' }}>
          <div style={{ color: '#969696', fontSize: 14 }}>题目详情加载中...</div>
        </div>
      </div>
    )
  }

  if (detailQuery.isError) {
    return (
      <div className="editor-immersive">
        <header className="editor-topbar">
          <div className="editor-topbar-left">
            <button type="button" onClick={handleGoBackFromEditor}>← 退出</button>
          </div>
          <div className="editor-topbar-right">
            <span>{user?.username || '游客'}</span>
          </div>
        </header>
        <div className="editor-body" style={{ alignItems: 'center', justifyContent: 'center' }}>
          <div style={{ color: '#f44747', fontSize: 14 }}>
            {detailQuery.error instanceof Error ? detailQuery.error.message : '题目详情加载失败'}
          </div>
        </div>
      </div>
    )
  }

  if (question && question.type !== 'code') {
    return (
      <div className="editor-immersive">
        <header className="editor-topbar">
          <div className="editor-topbar-left">
            <button type="button" onClick={handleGoBackFromEditor}>← 退出</button>
          </div>
          <div className="editor-topbar-right">
            <span>{user?.username || '游客'}</span>
          </div>
        </header>
        <div className="editor-body" style={{ alignItems: 'center', justifyContent: 'center', flexDirection: 'column', gap: 16 }}>
          <div style={{ color: '#d4d4d4', fontSize: 16 }}>当前题目不是代码题</div>
          <Link style={{ color: '#569cd6', textDecoration: 'none' }} to="/practice/$questionId" params={{ questionId }}>
            返回普通题目页 →
          </Link>
        </div>
      </div>
    )
  }

  return (
    <div className="editor-immersive">
      <header className="editor-topbar">
        <div className="editor-topbar-left">
          <button type="button" onClick={handleGoBackFromEditor}>← 退出</button>
          <span style={{ fontSize: 13, color: '#969696' }}>{question?.title || `题目 #${questionId}`}</span>
        </div>
        <div className="editor-topbar-right">
          <span>{user?.username || '游客'}</span>
          {accessToken ? (
            <button type="button" onClick={() => logout()}>退出登录</button>
          ) : null}
        </div>
      </header>

      <div className="editor-body" style={{ position: 'relative' }}>
        <div className="editor-problem" style={{ width: `${leftPanelWidth}%` }}>
          <h1>{question?.title || `题目 #${questionId}`}</h1>
          <p className="page-copy">{question?.content || '题目详情加载中...'}</p>
          <p className="companion-empty-text">当前行业：{questionIndustryLabel}</p>
          {question?.tag_list?.length ? (
            <div className="community-tag-row">
              {question.tag_list.map((tag) => (
                <span key={`editor-question-tag-${tag}`}>{tag}</span>
              ))}
            </div>
          ) : null}
          <div className="page-actions">
            <button
              type="button"
              onClick={() => void handleToggleFavorite()}
              onMouseDown={() => setIsFavPressed(true)}
              onMouseUp={() => setIsFavPressed(false)}
              onMouseLeave={() => setIsFavPressed(false)}
              style={{
                display: 'flex',
                alignItems: 'center',
                gap: 6,
                padding: '8px 16px',
                borderRadius: 10,
                border: favoriteState ? 'none' : '1px solid #555',
                background: favoriteState ? THEME.primary : 'transparent',
                color: favoriteState ? '#fff' : '#d4d4d4',
                fontSize: 13,
                fontWeight: 600,
                cursor: 'pointer',
                transition: 'all 0.15s cubic-bezier(0.4,0,0.2,1)',
                boxShadow: favoriteState ? '0 4px 14px rgba(249,115,22,0.4)' : 'none',
                transform: isFavPressed ? 'scale(0.92)' : 'scale(1)',
              }}
            >
              {favoriteState ? <StarFilled /> : <StarOutlined />}
              {favoriteState ? '已收藏' : '加入收藏'}
            </button>
            <Link className="secondary-link" to="/practice/$questionId" params={{ questionId }}>
              查看题目详情
            </Link>
          </div>
          {favoriteMessage !== '未操作' ? (
            <div style={{ fontSize: 12, color: '#888', marginTop: 4 }}>{favoriteMessage}</div>
          ) : null}
          {question ? (
            <QuestionNotePanel questionId={question.id} questionTitle={question.title} token={accessToken} />
          ) : null}
        </div>

        <div className="editor-resize-handle-vertical" onMouseDown={handleVerticalResize} />

        <div className="editor-workspace">
          <div className="editor-toolbar">
            <select value={editorLanguage} onChange={(event) => handleLanguageChange(event.target.value)}>
              <option value="go">Go</option>
              <option value="python">Python</option>
              <option value="javascript">JavaScript</option>
              <option value="java">Java</option>
              <option value="cpp">C++</option>
            </select>
            <div className="page-actions">
              <button className="secondary-button" type="button" onClick={handleResetCode}>重置代码</button>
              <button className="secondary-button" type="button" disabled={running} onClick={() => void handleRunCode()}>
                {running ? '运行中...' : '运行代码'}
              </button>
              <button className="primary-button" type="button" disabled={submitting} onClick={() => void handleSubmitCode()}>
                {submitting ? '提交中...' : '提交代码'}
              </button>
            </div>
          </div>

          <div className="editor-surface" ref={editorContainerRef} style={{ flex: `0 0 ${editorHeight}%` }} />

          <div className="editor-resize-handle-horizontal" onMouseDown={handleHorizontalResize} />

          <div className="editor-output-panel">
            <div className="editor-output-panel-title">
              <span>输出</span>
              <span style={{ fontWeight: 400, color: '#666' }}>{submitMessage}</span>
            </div>

            {!runOutput && !submitResult ? (
              <div className="output-line" style={{ color: '#666' }}>
                {evaluationMode === 'testcase'
                  ? '点击「运行代码」执行固定 3 条公开测试用例，点击「提交代码」执行隐藏用例并生成反馈'
                  : '点击「运行代码」查看运行结果，或点击「提交代码」获取AI分析'}
              </div>
            ) : null}

            {runOutput ? (
              <pre className={`output-pre ${runPassed ? 'success' : 'error'}`}>{runOutput}</pre>
            ) : null}

            {runJudgeSummary ? (
              <div className="output-section">
                <div className="output-section-title">运行结果</div>
                <div className="output-line">
                  公开用例通过 {runJudgeSummary.passed_count}/{runJudgeSummary.total_count}
                </div>
                {runJudgeSummary.case_results?.length ? (
                  <ul className="interview-bullet-list" style={{ marginTop: 12 }}>
                    {runJudgeSummary.case_results.map((item) => (
                      <li key={`run-case-${item.index}`}>
                        用例 #{item.index}：{item.passed ? '通过' : '失败'}
                        {item.description ? `，${item.description}` : ''}
                        {item.input ? `，输入 ${item.input}` : ''}
                        {item.expected_output ? `，期望 ${item.expected_output}` : ''}
                        {item.actual_output ? `，实际 ${item.actual_output}` : ''}
                        {item.error_output ? `，错误 ${item.error_output}` : ''}
                      </li>
                    ))}
                  </ul>
                ) : null}
              </div>
            ) : null}

            {question?.judge_config ? (
              <div className="output-section output-solution">
                <div className="output-section-title">判题配置</div>
                <p>模式：{question.judge_config.evaluation_mode === 'testcase' ? '测试用例判题' : 'AI 分析'}</p>
                <p>默认语言：{question.judge_config.default_language || 'go'}</p>
                <p>支持语言：{(question.judge_config.allowed_languages || []).join('、') || 'go'}</p>
                <p>时间限制：{question.judge_config.time_limit_ms || 2000}ms</p>
                <p>内存限制：{question.judge_config.memory_limit_mb || 128}MB</p>
                {question.judge_config.public_test_cases?.length ? (
                  <>
                    <strong>公开样例（运行时固定取前 3 条）</strong>
                    <ul>
                      {question.judge_config.public_test_cases.slice(0, 3).map((item, index) => (
                        <li key={`public-case-${index}`}>
                          样例 #{index + 1}
                          {item.description ? ` - ${item.description}` : ''}：
                          输入 `{item.input || '(空)'}`，期望输出 `{item.expected_output || '(空)'}`
                        </li>
                      ))}
                    </ul>
                  </>
                ) : null}
              </div>
            ) : null}

            {submitResult ? (
              <div className="output-section">
                <div className={`output-line ${submitResult.is_correct ? 'success' : 'error'}`}>
                  {submitResult.is_correct ? '✅ 解答正确' : '❌ 解答错误'}
                </div>
                <button
                  type="button"
                  onClick={() => setShowResultPanel(true)}
                  style={{
                    marginTop: 8,
                    padding: '6px 12px',
                    background: '#0078d4',
                    color: '#fff',
                    border: 'none',
                    borderRadius: 4,
                    cursor: 'pointer',
                    fontSize: 12,
                  }}
                >
                  查看详细结果 →
                </button>
              </div>
            ) : null}
          </div>
        </div>

        {/* 结果侧边栏 */}
        {showResultPanel && submitResult ? (
          <div
            style={{
              position: 'absolute',
              top: 0,
              right: 0,
              width: 420,
              height: '100%',
              background: '#252526',
              borderLeft: '1px solid #3c3c3c',
              display: 'flex',
              flexDirection: 'column',
              zIndex: 10,
              animation: 'slideInRight 0.3s ease',
            }}
          >
            {/* 侧边栏头部 */}
            <div
              style={{
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'space-between',
                padding: '12px 16px',
                borderBottom: '1px solid #3c3c3c',
                background: '#2d2d2d',
              }}
            >
              <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                <span style={{ fontSize: 16, color: submitResult.is_correct ? '#4ec9b0' : '#f44747' }}>
                  {submitResult.is_correct ? '✅' : '❌'}
                </span>
                <span style={{ fontSize: 14, fontWeight: 600, color: '#d4d4d4' }}>
                  {submitResult.is_correct ? '解答正确' : '解答错误'}
                </span>
              </div>
              <button
                type="button"
                onClick={() => setShowResultPanel(false)}
                style={{
                  background: 'none',
                  border: 'none',
                  color: '#969696',
                  cursor: 'pointer',
                  fontSize: 18,
                  padding: '0 4px',
                }}
              >
                ✕
              </button>
            </div>

            {/* 侧边栏内容 */}
            <div style={{ flex: 1, overflow: 'auto', padding: 16 }}>
              {/* 测试用例结果 */}
              {submitResult.judge_summary?.case_results?.length ? (
                <div style={{ marginBottom: 20 }}>
                  <div style={{ fontSize: 13, fontWeight: 600, color: '#d4d4d4', marginBottom: 8 }}>
                    测试用例 {submitResult.judge_summary.passed_count}/{submitResult.judge_summary.total_count} 通过
                  </div>
                  <div style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
                    {submitResult.judge_summary.case_results.map((item) => (
                      <div
                        key={`judge-case-${item.index}`}
                        style={{
                          padding: '6px 10px',
                          background: item.passed ? '#1e3a2f' : '#3a1e1e',
                          borderRadius: 4,
                          fontSize: 12,
                          color: item.passed ? '#4ec9b0' : '#f44747',
                        }}
                      >
                        用例 #{item.index}：{item.passed ? '通过' : '失败'}
                        {item.description ? ` - ${item.description}` : ''}
                      </div>
                    ))}
                  </div>
                </div>
              ) : null}

              {/* 参考答案 */}
              {submitResult.correct_answer ? (
                <div style={{ marginBottom: 20 }}>
                  <div style={{ fontSize: 13, fontWeight: 600, color: '#d4d4d4', marginBottom: 8 }}>
                    参考答案
                  </div>
                  <pre
                    style={{
                      padding: 12,
                      background: '#1e1e1e',
                      borderRadius: 6,
                      fontSize: 13,
                      color: '#d4d4d4',
                      overflow: 'auto',
                      maxHeight: 200,
                      whiteSpace: 'pre-wrap',
                      wordBreak: 'break-all',
                    }}
                  >
                    {submitResult.correct_answer}
                  </pre>
                </div>
              ) : null}

              {/* 解析 */}
              {submitResult.explanation ? (
                <div style={{ marginBottom: 20 }}>
                  <div style={{ fontSize: 13, fontWeight: 600, color: '#d4d4d4', marginBottom: 8 }}>
                    解析
                  </div>
                  <div
                    style={{
                      padding: 12,
                      background: '#1e1e1e',
                      borderRadius: 6,
                      fontSize: 13,
                      color: '#d4d4d4',
                      lineHeight: 1.6,
                      whiteSpace: 'pre-wrap',
                    }}
                  >
                    {submitResult.explanation}
                  </div>
                </div>
              ) : null}

              {/* AI 分析 */}
              {submitResult.ai_analysis ? (
                <div style={{ marginBottom: 20 }}>
                  <div style={{ fontSize: 13, fontWeight: 600, color: '#d4d4d4', marginBottom: 8 }}>
                    AI 分析
                  </div>
                  <pre
                    style={{
                      padding: 12,
                      background: '#1e1e1e',
                      borderRadius: 6,
                      fontSize: 13,
                      color: '#d4d4d4',
                      overflow: 'auto',
                      maxHeight: 300,
                      whiteSpace: 'pre-wrap',
                      wordBreak: 'break-all',
                    }}
                  >
                    {submitResult.ai_analysis}
                  </pre>
                </div>
              ) : null}
            </div>
          </div>
        ) : null}
      </div>
    </div>
  )
}

/**
 * 提供错题本页面，帮助用户回看最近仍未纠正的题目。
 */
export function PracticeWrongPage() {
  const navigate = useNavigate()
  const accessToken = useAuthStore((state) => state.accessToken)
  const [page, setPage] = useState(1)

  const wrongQuery = useQuery({
    queryKey: ['practice-wrong', accessToken, page],
    queryFn: () => fetchWrongQuestions(accessToken as string, page, PRACTICE_PAGE_SIZE),
    enabled: Boolean(accessToken),
  })

  const THEME = {
    bg: '#fafafa',
    cardBg: '#ffffff',
    primary: '#f97316',
    textMain: '#1c1917',
    textSecondary: '#57534e',
    textMuted: '#a8a29e',
    border: '#e7e5e4',
    success: '#22c55e',
    warning: '#f59e0b',
    danger: '#ef4444',
    info: '#3b82f6',
    shadow: '0 1px 3px rgba(0,0,0,0.04)',
    radius: 12,
  }

  const cardBase = {
    background: THEME.cardBg,
    borderRadius: THEME.radius,
    boxShadow: THEME.shadow,
    border: `1px solid ${THEME.border}`,
  }

  const difficultyColorMap: Record<string, string> = {
    easy: '#22c55e',
    medium: '#f59e0b',
    hard: '#ef4444',
  }

  const difficultyBgMap: Record<string, string> = {
    easy: '#f0fdf4',
    medium: '#fffbeb',
    hard: '#fef2f2',
  }

  if (!accessToken) {
    return (
      <div style={{ maxWidth: 800, margin: '0 auto', padding: '64px 16px', textAlign: 'center' }}>
        <Empty description="登录后查看错题本" image={Empty.PRESENTED_IMAGE_SIMPLE} />
        <Button
          type="primary"
          style={{ marginTop: 16 }}
          onClick={() => requestLoginPrompt('/practice/wrong', 'missing')}
        >
          去登录
        </Button>
      </div>
    )
  }

  return (
    <div style={{ maxWidth: 1200, margin: '0 auto', padding: '24px 16px' }}>
      {/* Header */}
      <div style={{ marginBottom: 24 }}>
        <h1 style={{ fontSize: 22, fontWeight: 700, color: THEME.textMain, margin: '0 0 4px' }}>
          <BookOutlined style={{ marginRight: 8, color: THEME.danger }} />错题本
        </h1>
        <p style={{ color: THEME.textSecondary, margin: 0, fontSize: 13 }}>集中回顾和重做答错的题目，巩固薄弱环节</p>
      </div>

      {/* Back to Practice */}
      <div style={{ marginBottom: 20 }}>
        <Link
          to="/practice"
          style={{
            fontSize: 14,
            fontWeight: 600,
            color: THEME.textSecondary,
            textDecoration: 'none',
            display: 'inline-flex',
            alignItems: 'center',
            gap: 6,
          }}
        >
          <ArrowLeftOutlined />
          返回题库主页面
        </Link>
      </div>

      {/* Stats */}
      <div style={{ ...cardBase, padding: '18px 24px', marginBottom: 20, display: 'flex', alignItems: 'center', gap: 32, flexWrap: 'wrap' }}>
        <div>
          <div style={{ fontSize: 13, color: THEME.textMuted, marginBottom: 2 }}>累计错题</div>
          <div style={{ fontSize: 28, fontWeight: 800, color: THEME.danger }}>{wrongQuery.data?.total || 0}</div>
        </div>
        <div style={{ width: 1, height: 40, background: THEME.border }} />
        <div>
          <div style={{ fontSize: 13, color: THEME.textMuted, marginBottom: 2 }}>当前页</div>
          <div style={{ fontSize: 28, fontWeight: 800, color: THEME.textMain }}>{wrongQuery.data?.list?.length || 0}</div>
        </div>
      </div>

      {wrongQuery.isLoading ? (
        <div style={{ textAlign: 'center', padding: 48 }}><Spin /></div>
      ) : wrongQuery.isError ? (
        <div style={{ padding: 24, textAlign: 'center', color: THEME.danger }}>
          {wrongQuery.error instanceof Error ? wrongQuery.error.message : '错题列表加载失败'}
        </div>
      ) : wrongQuery.data?.list?.length ? (
        <>
          {/* Table */}
          <div style={{ ...cardBase, overflow: 'hidden' }}>
            {/* Header */}
            <div
              style={{
                display: 'grid',
                gridTemplateColumns: '44px 1fr 100px 80px 100px',
                gap: 12,
                padding: '12px 20px',
                background: '#fafaf9',
                borderBottom: `1px solid ${THEME.border}`,
                fontSize: 12,
                fontWeight: 600,
                color: THEME.textMuted,
              }}
            >
              <span></span>
              <span>题目</span>
              <span style={{ textAlign: 'center' }}>题型</span>
              <span style={{ textAlign: 'center' }}>难度</span>
              <span style={{ textAlign: 'center' }}>操作</span>
            </div>

            {/* Rows */}
            <div>
              {wrongQuery.data.list.map((item, index) => {
                const question = item.question
                const diffColor = difficultyColorMap[question?.difficulty || ''] || THEME.textMuted
                const diffBg = difficultyBgMap[question?.difficulty || ''] || '#f5f5f4'
                const isCode = (question?.type || '') === 'code'

                return (
                  <div
                    key={item.id}
                    style={{
                      display: 'grid',
                      gridTemplateColumns: '44px 1fr 100px 80px 100px',
                      gap: 12,
                      alignItems: 'center',
                      padding: '12px 20px',
                      borderBottom: index === wrongQuery.data.list.length - 1 ? 'none' : `1px solid ${THEME.border}`,
                      transition: 'background 0.15s ease',
                    }}
                    onMouseEnter={(e) => { e.currentTarget.style.background = '#fafaf9' }}
                    onMouseLeave={(e) => { e.currentTarget.style.background = 'transparent' }}
                  >
                    {/* Status */}
                    <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
                      <CloseCircleOutlined style={{ fontSize: 16, color: THEME.danger }} />
                    </div>

                    {/* Title */}
                    <div style={{ minWidth: 0 }}>
                      <div
                        style={{
                          fontSize: 14,
                          fontWeight: 600,
                          color: THEME.textMain,
                          overflow: 'hidden',
                          textOverflow: 'ellipsis',
                          whiteSpace: 'nowrap',
                          marginBottom: 2,
                          cursor: 'pointer',
                        }}
                        onClick={() => navigate({
                          to: isCode ? '/practice/editor/$questionId' : '/practice/$questionId',
                          params: { questionId: String(item.question_id) },
                        })}
                      >
                        {question?.title || `题目 #${item.question_id}`}
                      </div>
                      <div style={{ fontSize: 11, color: THEME.textMuted }}>
                        {question?.category_name || '未分类'} · #{item.question_id}
                      </div>
                    </div>

                    {/* Type */}
                    <div style={{ textAlign: 'center' }}>
                      <Tag size="small" style={{ margin: 0, fontSize: 11 }}>{questionTypeLabel(question?.type || '')}</Tag>
                    </div>

                    {/* Difficulty */}
                    <div style={{ textAlign: 'center' }}>
                      <span
                        style={{
                          display: 'inline-block',
                          padding: '2px 8px',
                          borderRadius: 6,
                          fontSize: 11,
                          fontWeight: 600,
                          color: diffColor,
                          background: diffBg,
                        }}
                      >
                        {difficultyLabel(question?.difficulty || '')}
                      </span>
                    </div>

                    {/* Action */}
                    <div style={{ textAlign: 'center' }}>
                      <Button
                        size="small"
                        type="primary"
                        icon={<RedoOutlined />}
                        onClick={() => navigate({
                          to: isCode ? '/practice/editor/$questionId' : '/practice/$questionId',
                          params: { questionId: String(item.question_id) },
                        })}
                      >
                        重做
                      </Button>
                    </div>
                  </div>
                )
              })}
            </div>
          </div>

          {/* Pagination */}
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginTop: 20, padding: '12px 0' }}>
            <span style={{ color: THEME.textMuted, fontSize: 13 }}>共 {wrongQuery.data.total} 条错题记录</span>
            <Pagination
              current={page}
              pageSize={PRACTICE_PAGE_SIZE}
              total={wrongQuery.data.total}
              onChange={setPage}
              showSizeChanger={false}
              showTotal={(total) => `共 ${total} 条`}
            />
          </div>
        </>
      ) : (
        <div style={{ ...cardBase, padding: 48, textAlign: 'center' }}>
          <Empty description="暂无错题记录，继续保持！" image={Empty.PRESENTED_IMAGE_SIMPLE} />
        </div>
      )}
    </div>
  )
}

/**
 * 提供收藏夹页面，集中展示用户保留待复习的题目。
 */
export function PracticeFavoritesPage() {
  const navigate = useNavigate()
  const accessToken = useAuthStore((state) => state.accessToken)
  const [page, setPage] = useState(1)

  const favoritesQuery = useQuery({
    queryKey: ['practice-favorites', accessToken, page],
    queryFn: () => fetchFavorites(accessToken as string, page, PRACTICE_PAGE_SIZE),
    enabled: Boolean(accessToken),
  })

  if (!accessToken) {
    return (
      <div style={{ maxWidth: 800, margin: '0 auto', padding: '64px 16px', textAlign: 'center' }}>
        <Empty description="登录后查看收藏夹" image={Empty.PRESENTED_IMAGE_SIMPLE} />
        <Button
          type="primary"
          style={{ marginTop: 16, background: THEME.primary, borderColor: THEME.primary }}
          onClick={() => requestLoginPrompt('/practice/favorites', 'missing')}
        >
          去登录
        </Button>
      </div>
    )
  }

  return (
    <div style={{ background: THEME.bg, minHeight: '100vh' }}>
      <div style={{ maxWidth: 1200, margin: '0 auto', padding: '40px 24px 64px' }}>
        {/* ===== Hero Header ===== */}
        <div
          style={{
            ...glassCard,
            padding: '24px 28px',
            marginBottom: 24,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            flexWrap: 'wrap',
            gap: 16,
          }}
        >
          <div style={{ flex: 1, minWidth: 280 }}>
            <div
              style={{
                display: 'inline-flex',
                alignItems: 'center',
                gap: 6,
                padding: '4px 12px',
                borderRadius: 20,
                background: THEME.primaryLight,
                color: THEME.primaryDark,
                fontSize: 12,
                fontWeight: 700,
                marginBottom: 12,
              }}
            >
              <StarFilled />
              收藏夹
            </div>
            <h1
              style={{
                margin: 0,
                fontSize: 'clamp(24px, 3vw, 32px)',
                fontWeight: 800,
                color: THEME.textMain,
                lineHeight: 1.2,
                letterSpacing: -0.5,
              }}
            >
              我收藏的题目
            </h1>
            <p style={{ margin: '8px 0 0', fontSize: 14, color: THEME.textSecondary, lineHeight: 1.6 }}>
              这里用于沉淀值得反复练习、回顾或面试前再过一遍的题目。
            </p>
          </div>
          <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
            <div
              style={{
                textAlign: 'center',
                padding: '12px 20px',
                borderRadius: 14,
                background: THEME.primaryLight,
              }}
            >
              <div style={{ fontSize: 28, fontWeight: 800, color: THEME.primary, lineHeight: 1 }}>
                {favoritesQuery.data?.total || 0}
              </div>
              <div style={{ fontSize: 12, color: THEME.textSecondary, marginTop: 4 }}>道收藏</div>
            </div>
          </div>
        </div>

        {/* Back to Practice */}
        <div style={{ marginBottom: 20 }}>
          <Link
            to="/practice"
            style={{
              fontSize: 14,
              fontWeight: 600,
              color: THEME.textSecondary,
              textDecoration: 'none',
              display: 'inline-flex',
              alignItems: 'center',
              gap: 6,
            }}
          >
            <ArrowLeftOutlined />
            返回题库主页面
          </Link>
        </div>

        {favoritesQuery.isLoading ? (
          <div style={{ ...solidCard, padding: 48, textAlign: 'center' }}>
            <Spin size="large" tip="收藏列表加载中..." />
          </div>
        ) : null}

        {favoritesQuery.isError ? (
          <div
            style={{
              ...solidCard,
              padding: 32,
              textAlign: 'center',
              borderColor: 'rgba(239,68,68,0.2)',
              background: '#fef2f2',
            }}
          >
            <ExclamationCircleOutlined style={{ fontSize: 40, color: THEME.danger, marginBottom: 12 }} />
            <div style={{ fontSize: 16, fontWeight: 700, color: THEME.textMain, marginBottom: 6 }}>
              加载失败
            </div>
            <div style={{ fontSize: 14, color: THEME.textSecondary }}>
              {favoritesQuery.error instanceof Error ? favoritesQuery.error.message : '收藏列表加载失败'}
            </div>
          </div>
        ) : null}

        {favoritesQuery.data ? (
          <>
            {favoritesQuery.data.list.length ? (
              <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
                {favoritesQuery.data.list.map((item) => {
                  const isCode = (item.question?.type || '') === 'code'
                  return (
                    <div
                      key={item.id}
                      style={{
                        ...solidCard,
                        padding: '18px 22px',
                        display: 'flex',
                        alignItems: 'center',
                        gap: 16,
                        transition: 'all 0.2s ease',
                        cursor: 'pointer',
                      }}
                      onClick={() =>
                        navigate({
                          to: isCode ? '/practice/editor/$questionId' : '/practice/$questionId',
                          params: { questionId: String(item.question_id) },
                        })
                      }
                      onMouseEnter={(e) => {
                        e.currentTarget.style.borderColor = THEME.primary
                        e.currentTarget.style.boxShadow = '0 4px 16px rgba(249,115,22,0.1)'
                      }}
                      onMouseLeave={(e) => {
                        e.currentTarget.style.borderColor = THEME.border
                        e.currentTarget.style.boxShadow = THEME.shadow
                      }}
                    >
                      <div
                        style={{
                          width: 44,
                          height: 44,
                          borderRadius: 12,
                          background: THEME.primaryLight,
                          color: THEME.primary,
                          display: 'flex',
                          alignItems: 'center',
                          justifyContent: 'center',
                          fontSize: 18,
                          flexShrink: 0,
                        }}
                      >
                        <StarFilled />
                      </div>

                      <div style={{ flex: 1, minWidth: 0 }}>
                        <div
                          style={{
                            fontSize: 15,
                            fontWeight: 700,
                            color: THEME.textMain,
                            marginBottom: 4,
                            overflow: 'hidden',
                            textOverflow: 'ellipsis',
                            whiteSpace: 'nowrap',
                          }}
                        >
                          {item.question?.title || `题目 #${item.question_id}`}
                        </div>
                        <div style={{ display: 'flex', alignItems: 'center', gap: 10, flexWrap: 'wrap' }}>
                          <Tag
                            style={{
                              borderRadius: 8,
                              fontSize: 11,
                              color: THEME.accent,
                              background: '#eff6ff',
                              border: 'none',
                              fontWeight: 600,
                            }}
                          >
                            {questionTypeLabel(item.question?.type || '')}
                          </Tag>
                          <Tag
                            style={{
                              borderRadius: 8,
                              fontSize: 11,
                              color: difficultyColorMap[item.question?.difficulty || ''] || THEME.textMuted,
                              background: difficultyBgMap[item.question?.difficulty || ''] || '#fafaf9',
                              border: 'none',
                              fontWeight: 600,
                            }}
                          >
                            {difficultyLabel(item.question?.difficulty || '')}
                          </Tag>
                          <span style={{ fontSize: 12, color: THEME.textMuted }}>
                            <ClockCircleOutlined style={{ marginRight: 4 }} />
                            收藏于 {formatDateTime(item.created_at)}
                          </span>
                        </div>
                      </div>

                      <div
                        style={{
                          display: 'flex',
                          alignItems: 'center',
                          justifyContent: 'center',
                          width: 32,
                          height: 32,
                          borderRadius: 8,
                          background: '#fafaf9',
                          color: THEME.textMuted,
                          flexShrink: 0,
                        }}
                      >
                        <RightOutlined />
                      </div>
                    </div>
                  )
                })}
              </div>
            ) : (
              <div style={{ ...solidCard, padding: 48, textAlign: 'center' }}>
                <StarOutlined style={{ fontSize: 48, color: THEME.border, marginBottom: 16 }} />
                <div style={{ fontSize: 16, fontWeight: 700, color: THEME.textMain, marginBottom: 6 }}>
                  暂无收藏题目
                </div>
                <p style={{ margin: 0, fontSize: 14, color: THEME.textSecondary }}>
                  在刷题时遇到值得回顾的题目，点击"加入收藏"即可沉淀到这里。
                </p>
                <Button
                  type="primary"
                  style={{ marginTop: 20, background: THEME.primary, borderColor: THEME.primary }}
                  onClick={() => navigate({ to: '/practice' })}
                >
                  去题库刷题
                </Button>
              </div>
            )}

            {/* Pagination */}
            {favoritesQuery.data.list.length > 0 ? (
              <div
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'space-between',
                  marginTop: 24,
                  flexWrap: 'wrap',
                  gap: 12,
                }}
              >
                <span style={{ fontSize: 13, color: THEME.textMuted }}>
                  共 {favoritesQuery.data.total} 道收藏题目
                </span>
                <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
                  <button
                    type="button"
                    disabled={page <= 1}
                    onClick={() => setPage((current) => current - 1)}
                    style={{
                      padding: '8px 16px',
                      borderRadius: 10,
                      border: `1px solid ${THEME.border}`,
                      background: '#fff',
                      color: page <= 1 ? THEME.textMuted : THEME.textMain,
                      fontSize: 13,
                      fontWeight: 600,
                      cursor: page <= 1 ? 'not-allowed' : 'pointer',
                    }}
                  >
                    上一页
                  </button>
                  <span style={{ fontSize: 14, fontWeight: 700, color: THEME.textMain, minWidth: 48, textAlign: 'center' }}>
                    第 {page} 页
                  </span>
                  <button
                    type="button"
                    disabled={favoritesQuery.data.list.length < PRACTICE_PAGE_SIZE}
                    onClick={() => setPage((current) => current + 1)}
                    style={{
                      padding: '8px 16px',
                      borderRadius: 10,
                      border: `1px solid ${THEME.border}`,
                      background: '#fff',
                      color: favoritesQuery.data.list.length < PRACTICE_PAGE_SIZE ? THEME.textMuted : THEME.textMain,
                      fontSize: 13,
                      fontWeight: 600,
                      cursor: favoritesQuery.data.list.length < PRACTICE_PAGE_SIZE ? 'not-allowed' : 'pointer',
                    }}
                  >
                    下一页
                  </button>
                </div>
              </div>
            ) : null}
          </>
        ) : null}
      </div>
    </div>
  )
}

/**
 * 提供笔记列表页，集中查看所有题目笔记。
 */
export function PracticeNotesPage() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const accessToken = useAuthStore((state) => state.accessToken)
  const [page, setPage] = useState(1)
  const [message, setMessage] = useState('等待操作')

  const notesQuery = useQuery({
    queryKey: ['practice-notes', accessToken, page],
    queryFn: () => fetchNotes(accessToken as string, page, NOTE_PAGE_SIZE),
    enabled: Boolean(accessToken),
  })

  /**
   * 删除笔记列表中的某一项，并同步刷新当前分页数据。
   */
  async function handleDelete(noteId: number) {
    if (!accessToken) {
      requestLoginPrompt('/practice/notes', 'missing')
      return
    }

    try {
      await deleteQuestionNote(accessToken, noteId)
      setMessage('笔记已删除')
      await queryClient.invalidateQueries({ queryKey: ['practice-notes'] })
    } catch (error) {
      if (!useAuthStore.getState().accessToken) {
        requestLoginPrompt('/practice/notes', 'expired')
        return
      }
      setMessage(extractErrorMessage(error, '删除笔记失败'))
    }
  }

  if (!accessToken) {
    return (
      <div style={{ maxWidth: 800, margin: '0 auto', padding: '64px 16px', textAlign: 'center' }}>
        <Empty description="登录后查看笔记" image={Empty.PRESENTED_IMAGE_SIMPLE} />
        <Button
          type="primary"
          style={{ marginTop: 16, background: THEME.primary, borderColor: THEME.primary }}
          onClick={() => requestLoginPrompt('/practice/notes', 'missing')}
        >
          去登录
        </Button>
      </div>
    )
  }

  return (
    <div style={{ background: THEME.bg, minHeight: '100vh' }}>
      <div style={{ maxWidth: 1200, margin: '0 auto', padding: '40px 24px 64px' }}>
        {/* ===== Hero Header ===== */}
        <div
          style={{
            ...glassCard,
            padding: '24px 28px',
            marginBottom: 24,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            flexWrap: 'wrap',
            gap: 16,
          }}
        >
          <div style={{ flex: 1, minWidth: 280 }}>
            <div
              style={{
                display: 'inline-flex',
                alignItems: 'center',
                gap: 6,
                padding: '4px 12px',
                borderRadius: 20,
                background: THEME.primaryLight,
                color: THEME.primaryDark,
                fontSize: 12,
                fontWeight: 700,
                marginBottom: 12,
              }}
            >
              <EditOutlined />
              我的笔记
            </div>
            <h1
              style={{
                margin: 0,
                fontSize: 'clamp(24px, 3vw, 32px)',
                fontWeight: 800,
                color: THEME.textMain,
                lineHeight: 1.2,
                letterSpacing: -0.5,
              }}
            >
              练习笔记
            </h1>
            <p style={{ margin: '8px 0 0', fontSize: 14, color: THEME.textSecondary, lineHeight: 1.6 }}>
              这里汇总了你在刷题过程中沉淀下来的题解、易错点和复盘内容。
            </p>
          </div>
          <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
            <div
              style={{
                textAlign: 'center',
                padding: '12px 20px',
                borderRadius: 14,
                background: THEME.primaryLight,
              }}
            >
              <div style={{ fontSize: 28, fontWeight: 800, color: THEME.primary, lineHeight: 1 }}>
                {notesQuery.data?.total || 0}
              </div>
              <div style={{ fontSize: 12, color: THEME.textSecondary, marginTop: 4 }}>条笔记</div>
            </div>
          </div>
        </div>

        {/* Back to Practice */}
        <div style={{ marginBottom: 20 }}>
          <Link
            to="/practice"
            style={{
              fontSize: 14,
              fontWeight: 600,
              color: THEME.textSecondary,
              textDecoration: 'none',
              display: 'inline-flex',
              alignItems: 'center',
              gap: 6,
            }}
          >
            <ArrowLeftOutlined />
            返回题库主页面
          </Link>
        </div>

        {message !== '等待操作' ? (
          <div
            style={{
              ...solidCard,
              padding: '12px 16px',
              marginBottom: 16,
              display: 'flex',
              alignItems: 'center',
              gap: 8,
              fontSize: 13,
              color: message.includes('删除') ? THEME.danger : THEME.success,
              background: message.includes('删除') ? '#fef2f2' : '#f0fdf4',
              borderColor: message.includes('删除') ? 'rgba(239,68,68,0.2)' : 'rgba(34,197,94,0.2)',
            }}
          >
            {message.includes('删除') ? <CloseOutlined /> : <CheckCircleOutlined />}
            {message}
          </div>
        ) : null}

        {notesQuery.isLoading ? (
          <div style={{ ...solidCard, padding: 48, textAlign: 'center' }}>
            <Spin size="large" tip="笔记列表加载中..." />
          </div>
        ) : null}

        {notesQuery.isError ? (
          <div
            style={{
              ...solidCard,
              padding: 32,
              textAlign: 'center',
              borderColor: 'rgba(239,68,68,0.2)',
              background: '#fef2f2',
            }}
          >
            <ExclamationCircleOutlined style={{ fontSize: 40, color: THEME.danger, marginBottom: 12 }} />
            <div style={{ fontSize: 16, fontWeight: 700, color: THEME.textMain, marginBottom: 6 }}>
              加载失败
            </div>
            <div style={{ fontSize: 14, color: THEME.textSecondary }}>
              {notesQuery.error instanceof Error ? notesQuery.error.message : '笔记列表加载失败'}
            </div>
          </div>
        ) : null}

        {notesQuery.data ? (
          <>
            {notesQuery.data.list.length ? (
              <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
                {notesQuery.data.list.map((note) => (
                  <div key={note.id} style={{ ...solidCard, padding: '18px 22px' }}>
                    <div
                      style={{
                        display: 'flex',
                        alignItems: 'center',
                        justifyContent: 'space-between',
                        marginBottom: 10,
                        flexWrap: 'wrap',
                        gap: 8,
                      }}
                    >
                      <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
                        <div
                          style={{
                            width: 36,
                            height: 36,
                            borderRadius: 10,
                            background: THEME.primaryLight,
                            color: THEME.primary,
                            display: 'flex',
                            alignItems: 'center',
                            justifyContent: 'center',
                            fontSize: 16,
                            flexShrink: 0,
                          }}
                        >
                          <FileTextOutlined />
                        </div>
                        <div>
                          <div style={{ fontSize: 15, fontWeight: 700, color: THEME.textMain }}>
                            {note.title}
                          </div>
                          <div style={{ fontSize: 12, color: THEME.textMuted }}>
                            <ClockCircleOutlined style={{ marginRight: 4 }} />
                            {formatDateTime(note.updated_at || note.created_at)}
                          </div>
                        </div>
                      </div>

                      <button
                        type="button"
                        onClick={() => void handleDelete(note.id)}
                        style={{
                          padding: '6px 12px',
                          borderRadius: 8,
                          border: `1px solid ${THEME.border}`,
                          background: '#fff',
                          color: THEME.danger,
                          fontSize: 12,
                          fontWeight: 600,
                          cursor: 'pointer',
                          display: 'flex',
                          alignItems: 'center',
                          gap: 4,
                        }}
                        onMouseEnter={(e) => { e.currentTarget.style.background = '#fef2f2' }}
                        onMouseLeave={(e) => { e.currentTarget.style.background = '#fff' }}
                      >
                        <DeleteOutlined />
                        删除
                      </button>
                    </div>

                    <div
                      style={{
                        fontSize: 14,
                        color: THEME.textSecondary,
                        lineHeight: 1.7,
                        background: '#fafaf9',
                        borderRadius: 10,
                        padding: '12px 16px',
                        border: `1px solid ${THEME.border}`,
                        marginBottom: 12,
                      }}
                    >
                      {note.content}
                    </div>

                    <div style={{ display: 'flex', alignItems: 'center', gap: 12, flexWrap: 'wrap' }}>
                      {note.question_id ? (
                        <Link
                          to={(note.question?.type || '') === 'code' ? '/practice/editor/$questionId' : '/practice/$questionId'}
                          params={{ questionId: String(note.question_id) }}
                          style={{
                            fontSize: 13,
                            fontWeight: 600,
                            color: THEME.primary,
                            textDecoration: 'none',
                            display: 'inline-flex',
                            alignItems: 'center',
                            gap: 4,
                          }}
                        >
                          打开关联题目 <RightOutlined style={{ fontSize: 10 }} />
                        </Link>
                      ) : null}
                    </div>
                  </div>
                ))}
              </div>
            ) : (
              <div style={{ ...solidCard, padding: 48, textAlign: 'center' }}>
                <EditOutlined style={{ fontSize: 48, color: THEME.border, marginBottom: 16 }} />
                <div style={{ fontSize: 16, fontWeight: 700, color: THEME.textMain, marginBottom: 6 }}>
                  暂无笔记
                </div>
                <p style={{ margin: 0, fontSize: 14, color: THEME.textSecondary }}>
                  在刷题时记录思路与总结，笔记会自动汇总到这里。
                </p>
                <Button
                  type="primary"
                  style={{ marginTop: 20, background: THEME.primary, borderColor: THEME.primary }}
                  onClick={() => navigate({ to: '/practice' })}
                >
                  去题库刷题
                </Button>
              </div>
            )}

            {/* Pagination */}
            {notesQuery.data.list.length > 0 ? (
              <div
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'space-between',
                  marginTop: 24,
                  flexWrap: 'wrap',
                  gap: 12,
                }}
              >
                <span style={{ fontSize: 13, color: THEME.textMuted }}>
                  共 {notesQuery.data.total} 条笔记
                </span>
                <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
                  <button
                    type="button"
                    disabled={page <= 1}
                    onClick={() => setPage((current) => current - 1)}
                    style={{
                      padding: '8px 16px',
                      borderRadius: 10,
                      border: `1px solid ${THEME.border}`,
                      background: '#fff',
                      color: page <= 1 ? THEME.textMuted : THEME.textMain,
                      fontSize: 13,
                      fontWeight: 600,
                      cursor: page <= 1 ? 'not-allowed' : 'pointer',
                    }}
                  >
                    上一页
                  </button>
                  <span style={{ fontSize: 14, fontWeight: 700, color: THEME.textMain, minWidth: 48, textAlign: 'center' }}>
                    第 {page} 页
                  </span>
                  <button
                    type="button"
                    disabled={notesQuery.data.list.length < NOTE_PAGE_SIZE}
                    onClick={() => setPage((current) => current + 1)}
                    style={{
                      padding: '8px 16px',
                      borderRadius: 10,
                      border: `1px solid ${THEME.border}`,
                      background: '#fff',
                      color: notesQuery.data.list.length < NOTE_PAGE_SIZE ? THEME.textMuted : THEME.textMain,
                      fontSize: 13,
                      fontWeight: 600,
                      cursor: notesQuery.data.list.length < NOTE_PAGE_SIZE ? 'not-allowed' : 'pointer',
                    }}
                  >
                    下一页
                  </button>
                </div>
              </div>
            ) : null}
          </>
        ) : null}
      </div>
    </div>
  )
}

/**
 * 展示单个错因专题详情，承接报告页、成长页和练习反馈页的深挖入口。
 */
export function MistakeTopicPage() {
  const navigate = useNavigate()
  const accessToken = useAuthStore((state) => state.accessToken)
  const { topicCode } = useParams({ from: '/practice/topics/$topicCode' })
  const topicQuery = useQuery({
    queryKey: ['mistake-topic-detail', topicCode, accessToken],
    queryFn: () => fetchMistakeTopic(topicCode, accessToken),
    enabled: Boolean(topicCode),
  })

  /**
   * 以当前专题标签和关联题单构造正式题库路由，帮助用户稳定进入补练集合。
   */
  function handleOpenTopicPractice(questionSetSlug = ''): void {
    navigate({
      to: '/practice',
      search: topicQuery.data
        ? buildMistakeTopicPracticeRouteSearch(topicQuery.data, questionSetSlug)
        : {},
    })
  }

  return (
    <div style={{ background: THEME.bg, minHeight: '100vh' }}>
      <div style={{ maxWidth: 1200, margin: '0 auto', padding: '40px 24px 64px' }}>
        {/* ===== Hero Header ===== */}
        <div
          style={{
            ...glassCard,
            padding: '24px 28px',
            marginBottom: 24,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            flexWrap: 'wrap',
            gap: 16,
          }}
        >
          <div style={{ flex: 1, minWidth: 280 }}>
            <div
              style={{
                display: 'inline-flex',
                alignItems: 'center',
                gap: 6,
                padding: '4px 12px',
                borderRadius: 20,
                background: THEME.primaryLight,
                color: THEME.primaryDark,
                fontSize: 12,
                fontWeight: 700,
                marginBottom: 12,
              }}
            >
              <BulbOutlined />
              错因专题
            </div>
            <h1
              style={{
                margin: 0,
                fontSize: 'clamp(24px, 3vw, 32px)',
                fontWeight: 800,
                color: THEME.textMain,
                lineHeight: 1.2,
                letterSpacing: -0.5,
              }}
            >
              {topicQuery.data?.title || topicCode}
            </h1>
            <p style={{ margin: '8px 0 0', fontSize: 14, color: THEME.textSecondary, lineHeight: 1.6 }}>
              把高频错因从一个标签，展开成可复习、可自查、可继续补题的专题内容。
            </p>
          </div>
          {topicQuery.data ? (
            <Tag
              style={{
                borderRadius: 8,
                fontSize: 12,
                fontWeight: 600,
                color: THEME.primary,
                background: THEME.primaryLight,
                border: 'none',
                padding: '4px 12px',
              }}
            >
              {topicQuery.data.tag}
            </Tag>
          ) : null}
        </div>

        {/* Back to Practice */}
        <div style={{ marginBottom: 20 }}>
          <Link
            to="/practice"
            style={{
              fontSize: 14,
              fontWeight: 600,
              color: THEME.textSecondary,
              textDecoration: 'none',
              display: 'inline-flex',
              alignItems: 'center',
              gap: 6,
            }}
          >
            <ArrowLeftOutlined />
            返回题库主页面
          </Link>
        </div>

        {topicQuery.isLoading ? (
          <div style={{ ...solidCard, padding: 48, textAlign: 'center' }}>
            <Spin size="large" tip="专题内容加载中..." />
          </div>
        ) : null}

        {topicQuery.isError ? (
          <div
            style={{
              ...solidCard,
              padding: 32,
              textAlign: 'center',
              borderColor: 'rgba(239,68,68,0.2)',
              background: '#fef2f2',
            }}
          >
            <ExclamationCircleOutlined style={{ fontSize: 40, color: THEME.danger, marginBottom: 12 }} />
            <div style={{ fontSize: 16, fontWeight: 700, color: THEME.textMain, marginBottom: 6 }}>
              加载失败
            </div>
            <div style={{ fontSize: 14, color: THEME.textSecondary }}>
              {extractErrorMessage(topicQuery.error, '专题内容加载失败')}
            </div>
          </div>
        ) : null}

        {topicQuery.data ? (
          <>
            {/* Problem Pattern */}
            <div style={{ ...solidCard, padding: '24px 28px', marginBottom: 24 }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: 10, marginBottom: 12 }}>
                <div
                  style={{
                    width: 36,
                    height: 36,
                    borderRadius: 10,
                    background: '#fef2f2',
                    color: THEME.danger,
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'center',
                    fontSize: 16,
                  }}
                >
                  <ExclamationCircleOutlined />
                </div>
                <div>
                  <div style={{ fontSize: 15, fontWeight: 700, color: THEME.textMain }}>{topicQuery.data.title}</div>
                  <div style={{ fontSize: 12, color: THEME.textMuted }}>标签：{topicQuery.data.tag}</div>
                </div>
              </div>
              <div
                style={{
                  fontSize: 14,
                  color: THEME.textSecondary,
                  lineHeight: 1.7,
                  background: '#fafaf9',
                  borderRadius: 10,
                  padding: '14px 18px',
                  border: `1px solid ${THEME.border}`,
                }}
              >
                {topicQuery.data.problem_pattern}
              </div>
            </div>

            {/* Four Cards Grid */}
            <div
              style={{
                display: 'grid',
                gridTemplateColumns: 'repeat(auto-fit, minmax(280px, 1fr))',
                gap: 16,
                marginBottom: 24,
              }}
            >
              <TopicCard
                title="常见根因"
                items={topicQuery.data.root_causes}
                icon=<CloseOutlined style={{ color: THEME.danger }} />
                accentColor={THEME.danger}
              />
              <TopicCard
                title="自查清单"
                items={topicQuery.data.self_check_list}
                icon=<CheckCircleOutlined style={{ color: THEME.success }} />
                accentColor={THEME.success}
              />
              <TopicCard
                title="练习方向"
                items={topicQuery.data.practice_directions}
                icon=<BookOutlined style={{ color: THEME.accent }} />
                accentColor={THEME.accent}
              />
              <TopicCard
                title="建议动作"
                items={topicQuery.data.recommended_actions}
                icon=<ThunderboltOutlined style={{ color: THEME.warning }} />
                accentColor={THEME.warning}
              />
            </div>

            {/* Practice CTA */}
            <div style={{ ...solidCard, padding: '24px 28px' }}>
              <div
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'space-between',
                  marginBottom: 16,
                  flexWrap: 'wrap',
                  gap: 12,
                }}
              >
                <div>
                  <div style={{ fontSize: 15, fontWeight: 700, color: THEME.textMain }}>继续练习</div>
                  <div style={{ fontSize: 13, color: THEME.textMuted, marginTop: 2 }}>回到题库继续补强这个薄弱环节</div>
                </div>
                <button
                  type="button"
                  onClick={() => handleOpenTopicPractice(topicQuery.data.related_question_sets[0] || '')}
                  style={{
                    padding: '10px 24px',
                    borderRadius: 12,
                    border: 'none',
                    background: THEME.primary,
                    color: '#fff',
                    fontSize: 14,
                    fontWeight: 700,
                    cursor: 'pointer',
                    boxShadow: '0 4px 16px rgba(249,115,22,0.3)',
                    display: 'flex',
                    alignItems: 'center',
                    gap: 6,
                  }}
                >
                  <ThunderboltOutlined />
                  去题库补练
                </button>
              </div>

              {topicQuery.data.related_question_sets.length ? (
                <div style={{ display: 'flex', flexWrap: 'wrap', gap: 10 }}>
                  {topicQuery.data.related_question_sets.map((item) => (
                    <button
                      key={item}
                      type="button"
                      onClick={() => handleOpenTopicPractice(item)}
                      style={{
                        padding: '8px 16px',
                        borderRadius: 10,
                        border: `1px solid ${THEME.border}`,
                        background: '#fff',
                        color: THEME.textSecondary,
                        fontSize: 13,
                        fontWeight: 600,
                        cursor: 'pointer',
                        transition: 'all 0.2s',
                      }}
                      onMouseEnter={(e) => {
                        e.currentTarget.style.borderColor = THEME.primary
                        e.currentTarget.style.color = THEME.primary
                        e.currentTarget.style.background = THEME.primaryLight
                      }}
                      onMouseLeave={(e) => {
                        e.currentTarget.style.borderColor = THEME.border
                        e.currentTarget.style.color = THEME.textSecondary
                        e.currentTarget.style.background = '#fff'
                      }}
                    >
                      {resolvePracticeQuestionSetTitle(item)}
                    </button>
                  ))}
                </div>
              ) : (
                <p style={{ margin: 0, fontSize: 13, color: THEME.textMuted }}>
                  当前专题暂未绑定题单，先回到题库按标签继续补练。
                </p>
              )}
            </div>
          </>
        ) : null}
      </div>
    </div>
  )
}

function TopicCard(props: { title: string; items: string[]; icon: React.ReactNode; accentColor: string }) {
  return (
    <div style={{ ...solidCard, padding: '20px 24px' }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 10, marginBottom: 14 }}>
        <div
          style={{
            width: 36,
            height: 36,
            borderRadius: 10,
            background: `${props.accentColor}15`,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            fontSize: 16,
          }}
        >
          {props.icon}
        </div>
        <span style={{ fontSize: 15, fontWeight: 700, color: THEME.textMain }}>{props.title}</span>
      </div>
      <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
        {props.items.map((item) => (
          <div
            key={item}
            style={{
              display: 'flex',
              alignItems: 'flex-start',
              gap: 8,
              padding: '10px 14px',
              borderRadius: 10,
              background: '#fafaf9',
              border: `1px solid ${THEME.border}`,
              fontSize: 13,
              color: THEME.textSecondary,
              lineHeight: 1.5,
            }}
          >
            <span
              style={{
                width: 6,
                height: 6,
                borderRadius: '50%',
                background: props.accentColor,
                marginTop: 6,
                flexShrink: 0,
              }}
            />
            {item}
          </div>
        ))}
      </div>
    </div>
  )
}
