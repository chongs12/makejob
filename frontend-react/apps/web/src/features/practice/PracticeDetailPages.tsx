import type { FormEvent } from 'react'
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
async function fetchQuestionDetail(questionId: string): Promise<PracticeQuestionDetail> {
  const response = await requestJson<ApiEnvelope<PracticeQuestionDetail>>(`/questions/${questionId}`)
  if (!isSuccessCode(response.code) || !response.data) {
    throw new Error(response.message || '获取题目详情失败')
  }

  return response.data
}

/**
 * 拉取当前用户的收藏题目列表。
 */
async function fetchFavorites(token: string, page: number, pageSize: number): Promise<PageResult<FavoriteRecord>> {
  const response = await requestJson<ApiEnvelope<PageResult<FavoriteRecord>>>(`/user/favorites?page=${page}&page_size=${pageSize}`, {
    token,
  })

  if (!isSuccessCode(response.code) || !response.data) {
    throw new Error(response.message || '获取收藏列表失败')
  }

  return response.data
}

/**
 * 拉取当前用户的错题本列表。
 */
async function fetchWrongQuestions(token: string, page: number, pageSize: number): Promise<PageResult<WrongQuestionRecord>> {
  const response = await requestJson<ApiEnvelope<PageResult<WrongQuestionRecord>>>(`/user/wrong-questions?page=${page}&page_size=${pageSize}`, {
    token,
  })

  if (!isSuccessCode(response.code) || !response.data) {
    throw new Error(response.message || '获取错题本失败')
  }

  return response.data
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
  const response = await requestJson<ApiEnvelope<RunCodeResult>>(`/questions/${questionId}/run`, {
    method: 'POST',
    token,
    body: { answer, language },
  })

  if (!isSuccessCode(response.code) || !response.data) {
    throw new Error(response.message || '运行代码失败')
  }

  return response.data
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
    <section className="status-card note-panel">
      <div className="card-inline">
        <strong>题目笔记</strong>
        <span>{noteQuery.data ? '已有笔记' : '新建笔记'}</span>
      </div>

      <label className="field">
        <span>标题</span>
        <input value={title} onChange={(event) => setTitle(event.target.value)} placeholder="输入笔记标题" />
      </label>

      <label className="field">
        <span>内容</span>
        <textarea
          value={content}
          onChange={(event) => setContent(event.target.value)}
          rows={8}
          placeholder="记录思路、易错点或解题总结"
        />
      </label>

      <div className="card-inline">
        <span>{message}</span>
        <div className="page-actions">
          {noteQuery.data?.id ? (
            <button className="secondary-button" type="button" disabled={saving} onClick={() => void handleDelete()}>
              删除笔记
            </button>
          ) : null}
          <button className="primary-button" type="button" disabled={saving} onClick={() => void handleSave()}>
            {saving ? '保存中...' : '保存笔记'}
          </button>
        </div>
      </div>
    </section>
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
    <div style={{ marginTop: 16 }}>
      <strong>{props.title || '相关错因专题'}</strong>
      {topicsQuery.isLoading ? <p style={{ marginTop: 8 }}>正在加载错因专题...</p> : null}
      {topicsQuery.isError ? (
        <p style={{ marginTop: 8 }}>{extractErrorMessage(topicsQuery.error, '错因专题加载失败')}</p>
      ) : null}
      {matchedTopics.length ? (
        <div className="grid-cards" style={{ marginTop: 12 }}>
          {matchedTopics.map((topic) => (
            <article className="feature-card" key={topic.code}>
              <div className="card-inline">
                <strong>{topic.title}</strong>
                <span>{topic.tag}</span>
              </div>
              <p>{topic.problem_pattern}</p>
              <div className="page-actions">
                <Link className="secondary-link" to={resolveMistakeTopicRoute()} params={{ topicCode: topic.code }}>
                  打开专题
                </Link>
              </div>
            </article>
          ))}
        </div>
      ) : null}
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
  const [startedAt] = useState(() => Date.now())

  const detailQuery = useQuery({
    queryKey: ['practice-question-detail', questionId],
    queryFn: () => fetchQuestionDetail(questionId),
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
      await queryClient.invalidateQueries({ queryKey: ['practice-favorites'] })
    } catch (error) {
      if (!useAuthStore.getState().accessToken) {
        requestLoginPrompt(readCurrentBrowserPath(), 'expired')
        return
      }
      setFavoriteMessage(extractErrorMessage(error, '收藏操作失败'))
    }
  }

  return (
    <section className="page-panel">
      <span className="page-tag">刷题详情</span>
      <h1>题目 #{questionId}</h1>

      {detailQuery.isLoading ? (
        <div className="status-card" style={{ marginTop: 24 }}>题目详情加载中...</div>
      ) : null}

      {detailQuery.isError ? (
        <div className="status-card" style={{ marginTop: 24 }}>
          {detailQuery.error instanceof Error ? detailQuery.error.message : '题目详情加载失败'}
        </div>
      ) : null}

      {question ? (
        <>
          <div className="status-card" style={{ marginTop: 24 }}>
            <div className="card-inline">
              <strong>{question.title}</strong>
              <span>{difficultyLabel(question.difficulty)}</span>
            </div>
            <p>题型：{questionTypeLabel(question.type)}</p>
            <p>行业：{questionIndustryLabel}</p>
            <p>分类：{question.category_name || question.category_id}</p>
            {question.tag_list?.length ? (
              <div className="community-tag-row" style={{ marginTop: 12 }}>
                {question.tag_list.map((tag) => (
                  <span key={`question-tag-${tag}`}>{tag}</span>
                ))}
              </div>
            ) : null}
            <div className="question-content">{question.content}</div>
            <div className="page-actions" style={{ marginTop: 16 }}>
              <button className="secondary-button" type="button" onClick={() => void handleToggleFavorite()}>
                {favoriteState ? '取消收藏' : '加入收藏'}
              </button>
              {question.type === 'code' ? (
                <Link className="secondary-link" to="/practice/editor/$questionId" params={{ questionId: String(question.id) }}>
                  进入代码编辑器
                </Link>
              ) : null}
            </div>
            <div style={{ marginTop: 12 }}>收藏状态：{favoriteMessage}</div>
          </div>

          <form className="stack-form" onSubmit={handleSubmit}>
            {question.type === 'choice' ? (
              <div className="option-list">
                {options.map((option) => (
                  <label className="option-item" key={option.label}>
                    <input
                      type="radio"
                      name="choice-answer"
                      checked={singleAnswer === option.label}
                      onChange={() => setSingleAnswer(option.label)}
                    />
                    <span>{option.label}. {option.text}</span>
                  </label>
                ))}
              </div>
            ) : null}

            {question.type === 'multi' ? (
              <div className="option-list">
                {options.map((option) => (
                  <label className="option-item" key={option.label}>
                    <input
                      type="checkbox"
                      checked={multiAnswers.includes(option.label)}
                      onChange={() => toggleMultiAnswer(option.label)}
                    />
                    <span>{option.label}. {option.text}</span>
                  </label>
                ))}
              </div>
            ) : null}

            {question.type === 'subjective' ? (
              <label className="field">
                <span>你的答案</span>
                <textarea
                  value={textAnswer}
                  onChange={(event) => setTextAnswer(event.target.value)}
                  placeholder="请输入你的分析或答案"
                  rows={10}
                />
              </label>
            ) : null}

            {!accessToken ? (
              <div className="status-card">需要先登录后才能提交答案、收藏和记录笔记。</div>
            ) : null}

            <div className="card-inline">
              <Link className="secondary-link" to="/practice">返回题目列表</Link>
              <button className="primary-button" type="submit" disabled={submitting}>
                {submitting ? '提交中...' : '提交答案'}
              </button>
            </div>
          </form>

          <div className="status-card" style={{ marginTop: 24 }}>
            <div>提交状态：{submitMessage}</div>
            {submitResult ? (
              <>
                <div>判定结果：{submitResult.is_correct ? '正确' : '错误'}</div>
                <div>正确答案：{submitResult.correct_answer || '未返回'}</div>
                <div>解析说明：{submitResult.explanation || '暂无解析'}</div>
                {submitResult.ai_analysis ? (
                  <pre className="analysis-block">{submitResult.ai_analysis}</pre>
                ) : null}
                {question.solution ? (
                  <div style={{ marginTop: 16 }}>
                    <strong>结构化解析</strong>
                    <p>题意总结：{question.solution.summary || '暂无'}</p>
                    <p>解题思路：{question.solution.approach || '暂无'}</p>
                    <p>复杂度分析：{question.solution.complexity || '暂无'}</p>
                    {question.solution.key_steps.length ? (
                      <div style={{ marginTop: 8 }}>
                        <strong>关键步骤</strong>
                        <ul>
                          {question.solution.key_steps.map((item) => <li key={item}>{item}</li>)}
                        </ul>
                      </div>
                    ) : null}
                    {question.solution.edge_cases.length ? (
                      <div style={{ marginTop: 8 }}>
                        <strong>边界条件</strong>
                        <ul>
                          {question.solution.edge_cases.map((item) => <li key={item}>{item}</li>)}
                        </ul>
                      </div>
                    ) : null}
                    {question.solution.common_mistakes.length ? (
                      <div style={{ marginTop: 8 }}>
                        <strong>常见错法</strong>
                        <ul>
                          {question.solution.common_mistakes.map((item) => <li key={item}>{item}</li>)}
                        </ul>
                      </div>
                    ) : null}
                  </div>
                ) : null}
                {question.answer_template ? (
                  <div style={{ marginTop: 16 }}>
                    <strong>参考回答模板</strong>
                    <p>核心结论：{question.answer_template.core_conclusion || '暂无'}</p>
                    <p>面试表达示例：{question.answer_template.sample_answer || '暂无'}</p>
                    {question.answer_template.key_points.length ? (
                      <div style={{ marginTop: 8 }}>
                        <strong>关键展开点</strong>
                        <ul>
                          {question.answer_template.key_points.map((item) => <li key={item}>{item}</li>)}
                        </ul>
                      </div>
                    ) : null}
                    {question.answer_template.follow_ups.length ? (
                      <div style={{ marginTop: 8 }}>
                        <strong>高频追问点</strong>
                        <ul>
                          {question.answer_template.follow_ups.map((item) => <li key={item}>{item}</li>)}
                        </ul>
                      </div>
                    ) : null}
                    {question.answer_template.pitfalls.length ? (
                      <div style={{ marginTop: 8 }}>
                        <strong>易答偏点</strong>
                        <ul>
                          {question.answer_template.pitfalls.map((item) => <li key={item}>{item}</li>)}
                        </ul>
                      </div>
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

          <QuestionNotePanel
            questionId={question.id}
            questionTitle={question.title}
            token={accessToken}
          />
        </>
      ) : null}
    </section>
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
  const [startedAt] = useState(() => Date.now())
  const [leftPanelWidth, setLeftPanelWidth] = useState(40)
  const [editorHeight, setEditorHeight] = useState(60)

  const detailQuery = useQuery({
    queryKey: ['practice-code-question-detail', questionId],
    queryFn: () => fetchQuestionDetail(questionId),
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
      if (!editorContainerRef.current || editorInstanceRef.current) {
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
      await queryClient.invalidateQueries({ queryKey: ['practice-stats'] })
      await queryClient.invalidateQueries({ queryKey: ['practice-wrong'] })
      await queryClient.invalidateQueries({ queryKey: ['practice-recommendations'] })
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

      <div className="editor-body">
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
            <button className="secondary-button" type="button" onClick={() => void handleToggleFavorite()}>
              {favoriteState ? '取消收藏' : '加入收藏'}
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
              <>
                <div className={`output-line ${submitResult.is_correct ? 'success' : 'error'}`}>
                  {submitResult.is_correct ? '解答正确' : '解答错误'}
                </div>
                {submitResult.correct_answer ? (
                  <div className="output-line">参考答案：{submitResult.correct_answer}</div>
                ) : null}
                {submitResult.explanation ? (
                  <div className="output-line">解析：{submitResult.explanation}</div>
                ) : null}
                {submitResult.judge_summary ? (
                  <div className="output-section">
                    <div className="output-section-title">判题汇总</div>
                    <div className="output-line">
                      通过 {submitResult.judge_summary.passed_count}/{submitResult.judge_summary.total_count} 条测试用例
                    </div>
                    {submitResult.judge_summary.case_results?.length ? (
                      <ul className="interview-bullet-list" style={{ marginTop: 12 }}>
                        {submitResult.judge_summary.case_results.map((item) => (
                          <li key={`judge-case-${item.index}`}>
                            用例 #{item.index}：{item.passed ? '通过' : '失败'}
                            {item.description ? `，${item.description}` : ''}
                            {item.expected_output ? `，期望 ${item.expected_output}` : ''}
                            {item.actual_output ? `，实际 ${item.actual_output}` : ''}
                            {item.error_output ? `，错误 ${item.error_output}` : ''}
                          </li>
                        ))}
                      </ul>
                    ) : null}
                  </div>
                ) : null}
                {submitResult.ai_analysis ? (
                  <div className="output-section">
                    <div className="output-section-title">AI 分析</div>
                    <pre className="output-json">{submitResult.ai_analysis}</pre>
                  </div>
                ) : null}
                {question?.solution ? (
                  <div className="output-section output-solution">
                    <div className="output-section-title">结构化解析</div>
                    <p>题意总结：{question.solution.summary || '暂无'}</p>
                    <p>解题思路：{question.solution.approach || '暂无'}</p>
                    <p>复杂度分析：{question.solution.complexity || '暂无'}</p>
                    {question.solution.key_steps.length ? (
                      <>
                        <strong>关键步骤</strong>
                        <ul>
                          {question.solution.key_steps.map((item) => <li key={item}>{item}</li>)}
                        </ul>
                      </>
                    ) : null}
                    {question.solution.edge_cases.length ? (
                      <>
                        <strong>边界条件</strong>
                        <ul>
                          {question.solution.edge_cases.map((item) => <li key={item}>{item}</li>)}
                        </ul>
                      </>
                    ) : null}
                    {question.solution.common_mistakes.length ? (
                      <>
                        <strong>常见错法</strong>
                        <ul>
                          {question.solution.common_mistakes.map((item) => <li key={item}>{item}</li>)}
                        </ul>
                      </>
                    ) : null}
                  </div>
                ) : null}
                {practiceAnalysis?.mistake_tags?.length ? (
                  <div className="output-section">
                    <MistakeTopicHighlights
                      tags={practiceAnalysis.mistake_tags}
                      title="建议继续补的错因专题"
                    />
                  </div>
                ) : null}
              </>
            ) : null}
          </div>
        </div>
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
                gridTemplateColumns: '44px 1fr 100px 80px 100px 140px 100px',
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
              <span style={{ textAlign: 'center' }}>我的答案</span>
              <span style={{ textAlign: 'center' }}>错题时间</span>
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
                      gridTemplateColumns: '44px 1fr 100px 80px 100px 140px 100px',
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

                    {/* User answer */}
                    <div style={{ textAlign: 'center', fontSize: 13, color: THEME.danger, fontWeight: 500 }}>
                      {item.user_answer || '-'}
                    </div>

                    {/* Time */}
                    <div style={{ textAlign: 'center', fontSize: 12, color: THEME.textMuted }}>
                      <ClockCircleOutlined style={{ marginRight: 4 }} />
                      {formatDateTime(item.created_at)}
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
  const accessToken = useAuthStore((state) => state.accessToken)
  const [page, setPage] = useState(1)

  const favoritesQuery = useQuery({
    queryKey: ['practice-favorites', accessToken, page],
    queryFn: () => fetchFavorites(accessToken as string, page, PRACTICE_PAGE_SIZE),
    enabled: Boolean(accessToken),
  })

  return (
    <section className="page-panel">
      <span className="page-tag">收藏夹</span>
      <h1>我收藏的题目</h1>
      <p className="page-copy">这里用于沉淀值得反复练习、回顾或面试前再过一遍的题目。</p>

      {favoritesQuery.isLoading ? (
        <div className="status-card" style={{ marginTop: 24 }}>收藏列表加载中...</div>
      ) : null}

      {favoritesQuery.isError ? (
        <div className="status-card" style={{ marginTop: 24 }}>
          {favoritesQuery.error instanceof Error ? favoritesQuery.error.message : '收藏列表加载失败'}
        </div>
      ) : null}

      {favoritesQuery.data ? (
        <>
          <div className="grid-cards">
            {favoritesQuery.data.list.map((item) => (
              <article className="feature-card" key={item.id}>
                <h2>{item.question?.title || `题目 #${item.question_id}`}</h2>
                <p>题型：{questionTypeLabel(item.question?.type || '')}</p>
                <p>难度：{difficultyLabel(item.question?.difficulty || '')}</p>
                <p>收藏时间：{formatDateTime(item.created_at)}</p>
                <Link
                  className="secondary-link"
                  to={(item.question?.type || '') === 'code' ? '/practice/editor/$questionId' : '/practice/$questionId'}
                  params={{ questionId: String(item.question_id) }}
                >
                  打开题目
                </Link>
              </article>
            ))}
          </div>

          <div className="card-inline" style={{ marginTop: 24 }}>
            <span>共 {favoritesQuery.data.total} 道收藏题目</span>
            <div className="page-actions">
              <button className="secondary-button" type="button" disabled={page <= 1} onClick={() => setPage((current) => current - 1)}>
                上一页
              </button>
              <span>第 {page} 页</span>
              <button className="secondary-button" type="button" disabled={favoritesQuery.data.list.length < PRACTICE_PAGE_SIZE} onClick={() => setPage((current) => current + 1)}>
                下一页
              </button>
            </div>
          </div>
        </>
      ) : null}
    </section>
  )
}

/**
 * 提供笔记列表页，集中查看所有题目笔记。
 */
export function PracticeNotesPage() {
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

  return (
    <section className="page-panel">
      <span className="page-tag">我的笔记</span>
      <h1>练习笔记</h1>
      <p className="page-copy">这里汇总了你在刷题过程中沉淀下来的题解、易错点和复盘内容。</p>
      <div className="status-card" style={{ marginTop: 24 }}>{message}</div>

      {notesQuery.isLoading ? (
        <div className="status-card" style={{ marginTop: 24 }}>笔记列表加载中...</div>
      ) : null}

      {notesQuery.isError ? (
        <div className="status-card" style={{ marginTop: 24 }}>
          {notesQuery.error instanceof Error ? notesQuery.error.message : '笔记列表加载失败'}
        </div>
      ) : null}

      {notesQuery.data ? (
        <>
          <div className="stack-list">
            {notesQuery.data.list.map((note) => (
              <article className="feature-card" key={note.id}>
                <div className="card-inline">
                  <strong>{note.title}</strong>
                  <span>{formatDateTime(note.updated_at || note.created_at)}</span>
                </div>
                <p>{note.content}</p>
                <div className="page-actions">
                  {note.question_id ? (
                    <Link
                      className="secondary-link"
                      to={(note.question?.type || '') === 'code' ? '/practice/editor/$questionId' : '/practice/$questionId'}
                      params={{ questionId: String(note.question_id) }}
                    >
                      打开题目
                    </Link>
                  ) : null}
                  <button className="secondary-button" type="button" onClick={() => void handleDelete(note.id)}>
                    删除笔记
                  </button>
                </div>
              </article>
            ))}
          </div>

          <div className="card-inline" style={{ marginTop: 24 }}>
            <span>共 {notesQuery.data.total} 条笔记</span>
            <div className="page-actions">
              <button className="secondary-button" type="button" disabled={page <= 1} onClick={() => setPage((current) => current - 1)}>
                上一页
              </button>
              <span>第 {page} 页</span>
              <button className="secondary-button" type="button" disabled={notesQuery.data.list.length < NOTE_PAGE_SIZE} onClick={() => setPage((current) => current + 1)}>
                下一页
              </button>
            </div>
          </div>
        </>
      ) : null}
    </section>
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
    <section className="page-panel">
      <span className="page-tag">错因专题</span>
      <h1>{topicQuery.data?.title || topicCode}</h1>
      <p className="page-copy">把高频错因从一个标签，展开成可复习、可自查、可继续补题的专题内容。</p>

      {topicQuery.isLoading ? (
        <div className="status-card" style={{ marginTop: 24 }}>专题内容加载中...</div>
      ) : null}

      {topicQuery.isError ? (
        <div className="status-card" style={{ marginTop: 24 }}>
          {extractErrorMessage(topicQuery.error, '专题内容加载失败')}
        </div>
      ) : null}

      {topicQuery.data ? (
        <>
          <article className="status-card" style={{ marginTop: 24 }}>
            <div className="card-inline">
              <strong>{topicQuery.data.title}</strong>
              <span>{topicQuery.data.tag}</span>
            </div>
            <p style={{ marginTop: 12 }}>{topicQuery.data.problem_pattern}</p>
          </article>

          <div className="grid-cards" style={{ marginTop: 24 }}>
            <article className="feature-card">
              <h2>常见根因</h2>
              <ul>
                {topicQuery.data.root_causes.map((item) => <li key={item}>{item}</li>)}
              </ul>
            </article>
            <article className="feature-card">
              <h2>自查清单</h2>
              <ul>
                {topicQuery.data.self_check_list.map((item) => <li key={item}>{item}</li>)}
              </ul>
            </article>
          </div>

          <div className="grid-cards" style={{ marginTop: 24 }}>
            <article className="feature-card">
              <h2>练习方向</h2>
              <ul>
                {topicQuery.data.practice_directions.map((item) => <li key={item}>{item}</li>)}
              </ul>
            </article>
            <article className="feature-card">
              <h2>建议动作</h2>
              <ul>
                {topicQuery.data.recommended_actions.map((item) => <li key={item}>{item}</li>)}
              </ul>
            </article>
          </div>

          <article className="status-card" style={{ marginTop: 24 }}>
            <div className="card-inline">
              <div>
                <span className="section-kicker">继续练习</span>
                <h2>回到题库继续补强</h2>
              </div>
              <button
                className="secondary-button"
                type="button"
                onClick={() => handleOpenTopicPractice(topicQuery.data.related_question_sets[0] || '')}
              >
                去题库补练
              </button>
            </div>
            {topicQuery.data.related_question_sets.length ? (
              <div className="page-actions" style={{ marginTop: 12, flexWrap: 'wrap' }}>
                {topicQuery.data.related_question_sets.map((item) => (
                  <button className="secondary-button" key={item} type="button" onClick={() => handleOpenTopicPractice(item)}>
                    {resolvePracticeQuestionSetTitle(item)}
                  </button>
                ))}
              </div>
            ) : (
              <p style={{ marginTop: 12 }}>当前专题暂未绑定题单，先回到题库按标签继续补练。</p>
            )}
          </article>
        </>
      ) : null}
    </section>
  )
}
