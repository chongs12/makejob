import type { FormEvent } from 'react'
import { useEffect, useMemo, useRef, useState } from 'react'
import {
  createRootRouteWithContext,
  createRoute,
  createRouter,
  Link,
  Outlet,
  redirect,
  useNavigate,
  useParams,
  useRouterState,
} from '@tanstack/react-router'
import { useQuery, useQueryClient, type QueryClient } from '@tanstack/react-query'
import { AUTH_EXPIRED_EVENT_NAME, extractErrorMessage, requestJson } from '@makejob/api-client'
import { isSuccessCode, type ApiEnvelope } from '@makejob/shared-types'
import { useAuthStore } from './state/auth'
import { CompanionHubPage } from './features/companion/CompanionHubPage'
import { CompanionWorkspacePage } from './features/companion/CompanionWorkspacePage'
import {
  CommunityCreatePostPage,
  CommunityEditPostPage,
  CommunityMyPostsPage,
  CommunityPage,
  CommunityPostDetailPage,
} from './features/community/CommunityPages'
import GrowthPage from './features/growth/GrowthPage'
import { InterviewHubPage, InterviewReportPage, InterviewSessionPage } from './features/interview/InterviewPage'
import {
  DEFAULT_FRONTEND_INDUSTRY_CODE,
  type FrontendIndustry,
  fetchFrontendIndustries,
  formatFrontendIndustryLabel,
  persistSelectedFrontendIndustryCode,
  readSelectedFrontendIndustryCode,
  resolvePreferredFrontendIndustry,
} from './shared/industryContext'
import { findFrontendIndustryById, useFrontendIndustryPreference } from './shared/frontendIndustryPreference'
import {
  buildCurrentLocationPath,
  buildLoginRedirectSearch,
  readCurrentBrowserPath,
  resolveLoginRedirectTarget,
} from './shared/authRedirect'
import { LOGIN_REQUIRED_PROMPT_EVENT_NAME, type LoginPromptDetail, type LoginPromptReason, requestLoginPrompt } from './shared/loginPrompt'
import { fetchMistakeTopic, fetchMistakeTopics, parsePracticeAnalysis, pickMistakeTopicsByTags, resolveMistakeTopicRoute } from './shared/mistakeTopics'
import { persistPracticeFocusSearch, resolvePracticeQuestionSetTitle } from './shared/practiceFocus'
import { fetchPracticeRecommendations, resolvePracticeRecommendationRoute } from './shared/practiceRecommendations'
import { consumePendingPracticeSearch, persistPendingPracticeSearch } from './shared/practiceSearch'

interface RouterContext {
  queryClient: QueryClient
}

interface CategoryNode {
  id: number
  name: string
  children?: CategoryNode[]
}

interface PracticeQuestion {
  id: number
  title: string
  difficulty: string
  type: string
  category_id: number
  industry_id: number
  category_name?: string
  pass_rate?: number
  is_favorite?: boolean
  tags?: string
}

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

interface PracticeQuestionSetPreview {
  id: number
  title: string
  type: string
  difficulty: string
}

interface PracticeQuestionSetSummary {
  slug: string
  title: string
  description: string
  focus_tags: string[]
  question_count: number
  questions: PracticeQuestionSetPreview[]
}

interface PageResult<T> {
  list: T[]
  total: number
  page: number
  page_size: number
}

/**
 * 初始化前台登录态并返回最新的访问令牌，避免路由守卫读取到旧快照。
 */
function getLatestAccessToken(): string | null {
  const authStore = useAuthStore.getState()
  authStore.initAuth()
  return useAuthStore.getState().accessToken
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
  answer_template?: PracticeQuestionAnswerTemplate | null
  is_favorited?: boolean
  user_note?: PracticeNote | null
}

interface SubmitAnswerResult {
  is_correct: boolean
  correct_answer: string
  explanation: string
  ai_analysis?: string
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

interface CommunityPostAuthor {
  id: number
  username: string
  avatar?: string
  role?: string
}

interface CommunityPostItem {
  id: number
  post_type: string
  title: string
  content: string
  summary: string
  tags: string[]
  view_count: number
  comment_count: number
  like_count: number
  is_pinned: boolean
  is_recommended: boolean
  created_at: string
  author: CommunityPostAuthor
}

interface PracticeStats {
  total_answered: number
  correct_count: number
  wrong_count: number
  accuracy_rate: number
  today_count: number
  streak_days: number
}

interface ExamResponse {
  exam_id: string
  time_limit: number
  questions: PracticeQuestionDetail[]
}

interface HomePlanTask {
  id: number
  title: string
  status: string
  day_number: number
  due_date?: string
}

interface HomeCurrentPlan {
  id: number
  title: string
  description: string
  status: string
  progress: number
  total_tasks: number
  completed_tasks: number
  industry_code?: string
  tasks: HomePlanTask[]
}

interface HomePlanProgress {
  plan_id: number
  total_tasks: number
  completed_tasks: number
  skipped_tasks: number
  in_progress_tasks: number
  pending_tasks: number
  progress: number
}

interface HomeInterviewHistoryItem {
  id: number
  status: string
  score: number
  total_questions: number
  started_at?: string
  ended_at?: string
  created_at?: string
}

const PRACTICE_PAGE_SIZE = 10
const NOTE_PAGE_SIZE = 20

/**
 * 将树形分类拍平成选项列表，便于首版 React 页面快速挂接筛选器。
 */
function flattenCategories(nodes: CategoryNode[], level = 0): Array<{ id: number; name: string }> {
  return nodes.flatMap((node) => [
    { id: node.id, name: `${'　'.repeat(level)}${node.name}` },
    ...flattenCategories(node.children || [], level + 1),
  ])
}

/**
 * 将题目难度转换成中文标签，减少刷题页的阅读成本。
 */
function difficultyLabel(difficulty: string): string {
  const map: Record<string, string> = {
    easy: '简单',
    medium: '中等',
    hard: '困难',
  }

  return map[difficulty] || difficulty || '未知'
}

/**
 * 将题目类型转换成中文标签，统一前台显示口径。
 */
function questionTypeLabel(type: string): string {
  const map: Record<string, string> = {
    choice: '单选题',
    multi: '多选题',
    code: '编程题',
    subjective: '主观题',
  }

  return map[type] || type || '未知题型'
}

/**
 * 根据题目类型选择最合适的练习入口，编程题直接进入编辑器。
 */
function resolvePracticeTarget(questionId: number | string, questionType: string): string {
  if (questionType === 'code') {
    return `/practice/editor/${questionId}`
  }

  return `/practice/${questionId}`
}

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
 * 将时间格式化为相对时间，便于首页动态流展示“刚刚更新”等轻量信息。
 */
function formatRelativeTime(value?: string): string {
  if (!value) {
    return '--'
  }

  const timestamp = new Date(value).getTime()
  if (Number.isNaN(timestamp)) {
    return value
  }

  const diff = Date.now() - timestamp
  if (diff < 60 * 1000) {
    return '刚刚'
  }

  const minutes = Math.floor(diff / (60 * 1000))
  if (minutes < 60) {
    return `${minutes} 分钟前`
  }

  const hours = Math.floor(diff / (60 * 60 * 1000))
  if (hours < 24) {
    return `${hours} 小时前`
  }

  const days = Math.floor(diff / (24 * 60 * 60 * 1000))
  if (days < 7) {
    return `${days} 天前`
  }

  return formatDateTime(value)
}

/**
 * 将长文本裁剪成首页卡片更适合展示的摘要长度，避免卡片高度失控。
 */
function truncateText(value: string, maxLength: number): string {
  const normalized = value.trim()
  if (normalized.length <= maxLength) {
    return normalized
  }

  return `${normalized.slice(0, maxLength)}...`
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
 * 拉取题库分类树，供刷题页筛选器使用。
 */
async function fetchCategories(industryCode: string): Promise<CategoryNode[]> {
  const params = new URLSearchParams({
    industry_code: industryCode.trim() || DEFAULT_FRONTEND_INDUSTRY_CODE,
  })
  const response = await requestJson<ApiEnvelope<CategoryNode[]>>(`/categories?${params.toString()}`)
  if (!isSuccessCode(response.code)) {
    throw new Error(response.message || '获取分类失败')
  }

  return response.data || []
}

/**
 * 拉取题目分页列表，并将筛选条件统一映射到后端查询参数。
 */
async function fetchQuestions(params: {
  page: number
  pageSize: number
  difficulty: string
  keyword: string
  industryId: number | null
  categoryId: number | null
}): Promise<PageResult<PracticeQuestion>> {
  const searchParams = new URLSearchParams({
    page: String(params.page),
    page_size: String(params.pageSize),
  })

  if (params.difficulty) {
    searchParams.set('difficulty', params.difficulty)
  }

  if (params.keyword) {
    searchParams.set('keyword', params.keyword)
  }

  if (params.industryId) {
    searchParams.set('industry_id', String(params.industryId))
  }

  if (params.categoryId) {
    searchParams.set('category_id', String(params.categoryId))
  }

  const response = await requestJson<ApiEnvelope<PageResult<PracticeQuestion>>>(`/questions?${searchParams.toString()}`)
  if (!isSuccessCode(response.code) || !response.data) {
    throw new Error(response.message || '获取题目列表失败')
  }

  return response.data
}

/**
 * 拉取首页内容流使用的社区帖子列表，作为首版公开动态内容来源。
 */
async function fetchHomeCommunityPosts(params: {
  page: number
  pageSize: number
}): Promise<PageResult<CommunityPostItem>> {
  const searchParams = new URLSearchParams({
    page: String(params.page),
    page_size: String(params.pageSize),
  })

  const response = await requestJson<ApiEnvelope<PageResult<CommunityPostItem>>>(`/community/posts?${searchParams.toString()}`)
  if (!isSuccessCode(response.code) || !response.data) {
    throw new Error(response.message || '获取首页动态失败')
  }

  return response.data
}

/**
 * 拉取首页使用的最近面试记录，为“继续面试/查看报告”卡片提供数据。
 */
async function fetchHomeInterviewHistory(token: string): Promise<PageResult<HomeInterviewHistoryItem>> {
  const response = await requestJson<ApiEnvelope<PageResult<HomeInterviewHistoryItem>>>('/interviews?page=1&page_size=3', {
    token,
  })

  if (!isSuccessCode(response.code) || !response.data) {
    throw new Error(response.message || '获取最近面试记录失败')
  }

  return response.data
}

/**
 * 拉取当前进行中的学习计划，为首页工作台展示当前推进主线。
 */
async function fetchHomeCurrentPlan(token: string): Promise<HomeCurrentPlan | null> {
  const response = await requestJson<ApiEnvelope<HomeCurrentPlan>>('/plans/current', {
    token,
  })

  if (!isSuccessCode(response.code)) {
    if (response.code === 404) {
      return null
    }

    throw new Error(response.message || '获取当前学习计划失败')
  }

  return response.data || null
}

/**
 * 拉取当前计划进度统计，让首页可以直接显示任务推进状态。
 */
async function fetchHomePlanProgress(token: string, planId: number): Promise<HomePlanProgress> {
  const response = await requestJson<ApiEnvelope<HomePlanProgress>>(`/plans/${planId}/progress`, {
    token,
  })

  if (!isSuccessCode(response.code) || !response.data) {
    throw new Error(response.message || '获取学习计划进度失败')
  }

  return response.data
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
 * 拉取当前行业下的核心题单摘要，供刷题首页快速进入高价值主题。
 */
async function fetchQuestionSets(industryId: number | null): Promise<PracticeQuestionSetSummary[]> {
  const params = new URLSearchParams()
  if (industryId) {
    params.set('industry_id', String(industryId))
  }

  const suffix = params.toString() ? `?${params.toString()}` : ''
  const response = await requestJson<ApiEnvelope<PracticeQuestionSetSummary[]>>(`/question-sets${suffix}`)
  if (!isSuccessCode(response.code)) {
    throw new Error(response.message || '获取核心题单失败')
  }

  return response.data || []
}

/**
 * 拉取当前用户的练习统计，为练习首页提供概览卡片。
 */
async function fetchPracticeStats(token: string): Promise<PracticeStats> {
  const response = await requestJson<ApiEnvelope<PracticeStats>>('/user/practice-stats', {
    token,
  })

  if (!isSuccessCode(response.code) || !response.data) {
    throw new Error(response.message || '获取练习统计失败')
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
 * 汇总刷题域右侧常用列表数量，供总览页快速展示复习压力和沉淀规模。
 */
async function fetchPracticeCollectionsOverview(token: string): Promise<{
  favorites: number
  wrongQuestions: number
  notes: number
}> {
  const [favorites, wrongQuestions, notes] = await Promise.all([
    fetchFavorites(token, 1, 1),
    fetchWrongQuestions(token, 1, 1),
    fetchNotes(token, 1, 1),
  ])

  return {
    favorites: favorites.total,
    wrongQuestions: wrongQuestions.total,
    notes: notes.total,
  }
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
async function submitAnswerRequest(token: string, questionId: number, answer: string, timeSpent: number): Promise<SubmitAnswerResult> {
  const response = await requestJson<ApiEnvelope<SubmitAnswerResult>>(`/questions/${questionId}/submit`, {
    method: 'POST',
    token,
    body: {
      answer,
      time_spent: timeSpent,
    },
  })

  if (!isSuccessCode(response.code) || !response.data) {
    throw new Error(response.message || '提交答案失败')
  }

  return response.data
}

/**
 * 调用随机练习或限时模拟接口，并返回首个题目的跳转信息。
 */
async function generateExamRequest(params: {
  token: string
  mode: 'random' | 'timed'
  difficulty: string
  industryId: number | null
  categoryId: number | null
}): Promise<ExamResponse> {
  const endpoint = params.mode === 'timed' ? '/exams/timed' : '/exams/random'
  const body = params.mode === 'timed'
    ? {
        count: 5,
        difficulty: params.difficulty || 'medium',
        industry_id: params.industryId || undefined,
        category_id: params.categoryId || undefined,
        time_limit_minutes: 30,
      }
    : {
        count: 5,
        difficulty: params.difficulty || 'medium',
        industry_id: params.industryId || undefined,
        category_id: params.categoryId || undefined,
      }

  const response = await requestJson<ApiEnvelope<ExamResponse>>(endpoint, {
    method: 'POST',
    token: params.token,
    body,
  })

  if (!isSuccessCode(response.code) || !response.data) {
    throw new Error(response.message || '生成练习失败')
  }

  return response.data
}

/**
 * 在前台根布局初始化时恢复登录态，并在已有令牌时补拉用户资料。
 */
function AuthBootstrap() {
  const initialized = useAuthStore((state) => state.initialized)
  const accessToken = useAuthStore((state) => state.accessToken)
  const user = useAuthStore((state) => state.user)
  const initAuth = useAuthStore((state) => state.initAuth)
  const ensureProfile = useAuthStore((state) => state.ensureProfile)

  useEffect(() => {
    initAuth()
  }, [initAuth])

  useEffect(() => {
    if (!initialized || !accessToken || user) {
      return
    }

    void ensureProfile()
  }, [accessToken, ensureProfile, initialized, user])

  return null
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
 * 渲染全局统一的登录提示弹窗，引导用户在需要完整功能时主动进入登录页。
 */
function LoginRequiredDialog(props: {
  open: boolean
  reason: LoginPromptReason
  onConfirm: () => void
  onCancel: () => void
}) {
  if (!props.open) {
    return null
  }

  const title = props.reason === 'expired' ? '登录状态已失效' : '需要先登录'
  const description = props.reason === 'expired'
    ? '你的登录状态已经过期。想继续使用完整功能，请重新登录。'
    : '当前功能需要登录后才能完整使用。登录后你可以继续当前操作，并保留回跳位置。'

  return (
    <div className="login-required-overlay" role="presentation">
      <div className="login-required-dialog" role="dialog" aria-modal="true" aria-labelledby="login-required-title">
        <span className="page-tag">登录提示</span>
        <h2 id="login-required-title">{title}</h2>
        <p>{description}</p>
        <div className="page-actions login-required-actions">
          <button className="primary-button" type="button" onClick={props.onConfirm}>
            立即登录
          </button>
          <button className="secondary-button" type="button" onClick={props.onCancel}>
            稍后登录
          </button>
        </div>
      </div>
    </div>
  )
}

/**
 * 提供前台统一外壳，承载导航、状态展示和子路由出口。
 */
function RootLayout() {
  const navigate = useNavigate()
  const pathname = useRouterState({
    select: (state) => state.location.pathname,
  })
  const searchStr = useRouterState({
    select: (state) => state.location.searchStr,
  })
  const accessToken = useAuthStore((state) => state.accessToken)
  const user = useAuthStore((state) => state.user)
  const logout = useAuthStore((state) => state.logout)
  const [headerKeyword, setHeaderKeyword] = useState('')
  const [loginPromptState, setLoginPromptState] = useState<{
    open: boolean
    redirectTarget: string
    reason: LoginPromptReason
  }>({
    open: false,
    redirectTarget: '/',
    reason: 'missing',
  })
  const { effectiveIndustryLabel } = useFrontendIndustryPreference()
  const currentLocationPath = buildCurrentLocationPath(pathname, searchStr || '')

  /**
   * 打开全局登录提示弹窗，并记录登录后应该返回的目标地址。
   */
  function openLoginPrompt(redirectTarget: string, reason: LoginPromptReason): void {
    setLoginPromptState({
      open: true,
      redirectTarget: redirectTarget.trim() || currentLocationPath,
      reason,
    })
  }

  /**
   * 关闭当前登录提示弹窗，允许用户暂时留在现有页面继续浏览。
   */
  function closeLoginPrompt(): void {
    setLoginPromptState((current) => ({
      ...current,
      open: false,
    }))
  }

  /**
   * 统一处理顶部题库搜索，确保无论当前处于哪个页面都能直接跳转到题库页筛选。
   */
  function handleHeaderSearch(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    persistPendingPracticeSearch(headerKeyword)
    navigate({
      to: '/practice',
    })
  }

  /**
   * 处理顶部“发布”入口，统一跳到社区发帖页并保留登录回跳。
   */
  function handlePublish() {
    if (accessToken) {
      navigate({
        to: '/community/create',
      })
      return
    }

    openLoginPrompt('/community/create', 'missing')
  }

  /**
   * 监听显式登录提示和令牌失效事件，统一由根布局弹出登录引导。
   */
  useEffect(() => {
    if (typeof window === 'undefined') {
      return
    }

    /**
     * 接收业务侧主动发起的登录请求，并把目标地址带入统一弹窗。
     */
    function handleLoginRequired(event: Event): void {
      const detail = (event as CustomEvent<LoginPromptDetail | undefined>).detail
      const redirectTarget = typeof detail?.redirectTarget === 'string' ? detail.redirectTarget : currentLocationPath
      openLoginPrompt(redirectTarget, detail?.reason === 'expired' ? 'expired' : 'missing')
    }

    /**
     * 在请求层捕获到 401 后，直接提示用户重新登录即可继续当前页面操作。
     */
    function handleAuthExpired(): void {
      if (pathname === '/auth/login') {
        return
      }

      openLoginPrompt(currentLocationPath, 'expired')
    }

    window.addEventListener(LOGIN_REQUIRED_PROMPT_EVENT_NAME, handleLoginRequired)
    window.addEventListener(AUTH_EXPIRED_EVENT_NAME, handleAuthExpired)

    return () => {
      window.removeEventListener(LOGIN_REQUIRED_PROMPT_EVENT_NAME, handleLoginRequired)
      window.removeEventListener(AUTH_EXPIRED_EVENT_NAME, handleAuthExpired)
    }
  }, [currentLocationPath, pathname])

  /**
   * 用户重新拿到有效令牌或已经进入登录页时，自动收起旧的登录提示。
   */
  useEffect(() => {
    if (!accessToken && pathname !== '/auth/login') {
      return
    }

    closeLoginPrompt()
  }, [accessToken, pathname])

  /**
   * 确认跳转登录页，并把当前弹窗记录的回跳地址一并带上。
   */
  function handleConfirmLogin(): void {
    const redirectTarget = loginPromptState.redirectTarget.trim() || currentLocationPath
    closeLoginPrompt()
    navigate({
      to: '/auth/login',
      search: buildLoginRedirectSearch(redirectTarget),
    })
  }

  const navigationItems = [
    { to: '/', label: '首页', match: pathname === '/' },
    { to: '/practice', label: '题库', match: pathname.startsWith('/practice') },
    { to: '/community', label: '社区', match: pathname.startsWith('/community') },
    { to: '/interview', label: '面试', match: pathname.startsWith('/interview') },
    { to: '/companion', label: '学习陪伴', match: pathname.startsWith('/companion') },
    { to: '/growth', label: '成长档案', match: pathname.startsWith('/growth') || pathname.startsWith('/workspace') },
  ]
  const accountLabel = accessToken ? (user?.username || '成长档案') : '登录'
  const isStandaloneCompanionRoom = pathname.startsWith('/companion/room')
  const loginPromptDialog = (
    <LoginRequiredDialog
      open={loginPromptState.open}
      reason={loginPromptState.reason}
      onConfirm={handleConfirmLogin}
      onCancel={closeLoginPrompt}
    />
  )

  if (isStandaloneCompanionRoom) {
    return (
      <div className="app-shell companion-room-shell">
        <AuthBootstrap />
        <Outlet />
        {loginPromptDialog}
      </div>
    )
  }

  return (
    <div className="app-shell">
      <AuthBootstrap />

      <header className="topbar">
        <div className="topbar-inner">
          <Link className="brand-block" to="/">
            <div className="brand-mark">MakeJob</div>
            <div className="brand-subtitle">{effectiveIndustryLabel} 刷题、面试、学习陪伴一体化入口</div>
          </Link>

          <nav className="top-nav">
            {navigationItems.map((item) => (
              <Link
                className={`nav-link ${item.match ? 'nav-link-active' : ''}`}
                key={item.to}
                to={item.to}
              >
                {item.label}
              </Link>
            ))}
          </nav>

          <form className="search-form" onSubmit={handleHeaderSearch}>
            <input
              value={headerKeyword}
              onChange={(event) => setHeaderKeyword(event.target.value)}
              placeholder={`搜索 ${effectiveIndustryLabel} 题目、面经、学习主题`}
            />
            <button className="secondary-button" type="submit">搜索</button>
          </form>

          <div className="nav-actions">
            <Link className="nav-action-link" to={accessToken ? '/growth' : '/auth/login'}>
              {accountLabel}
            </Link>
            <button className="primary-button nav-publish-button" type="button" onClick={handlePublish}>
              发布
            </button>
          </div>
        </div>

        <div className="topbar-status">
          <div className="topbar-status-inner">
            <span>当前路径：{pathname}</span>
            <span>当前方向：{effectiveIndustryLabel}</span>
            <span>用户：{user?.username || '游客'}</span>
            <span>会员：{user?.membershipLevel || 'free'}</span>
            <span>状态：{accessToken ? '已登录' : '未登录'}</span>
            {accessToken ? (
              <button className="logout-link" type="button" onClick={() => logout()}>
                退出登录
              </button>
            ) : null}
          </div>
        </div>
      </header>

      <main className="page-content site-main">
        <Outlet />
      </main>
      {loginPromptDialog}
    </div>
  )
}

/**
 * 展示当前 React 前台重构的整体方向和优先迁移模块。
 */
function HomePage() {
  const accessToken = useAuthStore((state) => state.accessToken)
  const { effectiveIndustryCode, effectiveIndustryLabel, industriesQuery } = useFrontendIndustryPreference()
  const highlightedIndustries = industriesQuery.data?.length
    ? industriesQuery.data
    : [
        { id: 0, code: 'go', name: 'Go 后端' },
        { id: 1, code: 'frontend', name: '前端工程' },
        { id: 2, code: 'java', name: 'Java 后端' },
      ]
  const practicePreviewQuery = useQuery({
    queryKey: ['home-practice-preview', effectiveIndustryCode],
    queryFn: () => fetchQuestions({
      page: 1,
      pageSize: 3,
      difficulty: '',
      keyword: '',
      industryId: highlightedIndustries.find((item) => item.code === effectiveIndustryCode)?.id || null,
      categoryId: null,
    }),
    enabled: Boolean(highlightedIndustries.find((item) => item.code === effectiveIndustryCode)?.id),
  })
  const communityQuery = useQuery({
    queryKey: ['home-community-posts'],
    queryFn: () => fetchHomeCommunityPosts({
      page: 1,
      pageSize: 3,
    }),
    staleTime: 2 * 60 * 1000,
  })
  const statsQuery = useQuery({
    queryKey: ['home-practice-stats', accessToken],
    queryFn: () => fetchPracticeStats(accessToken as string),
    enabled: Boolean(accessToken),
    retry: false,
  })
  const collectionsOverviewQuery = useQuery({
    queryKey: ['home-practice-collections', accessToken],
    queryFn: () => fetchPracticeCollectionsOverview(accessToken as string),
    enabled: Boolean(accessToken),
    retry: false,
  })
  const interviewHistoryQuery = useQuery({
    queryKey: ['home-interview-history', accessToken],
    queryFn: () => fetchHomeInterviewHistory(accessToken as string),
    enabled: Boolean(accessToken),
    retry: false,
  })
  const currentPlanQuery = useQuery({
    queryKey: ['home-current-plan', accessToken],
    queryFn: () => fetchHomeCurrentPlan(accessToken as string),
    enabled: Boolean(accessToken),
    retry: false,
  })
  const planProgressQuery = useQuery({
    queryKey: ['home-plan-progress', accessToken, currentPlanQuery.data?.id],
    queryFn: () => fetchHomePlanProgress(accessToken as string, currentPlanQuery.data?.id as number),
    enabled: Boolean(accessToken && currentPlanQuery.data?.id),
    retry: false,
  })
  const latestInterview = interviewHistoryQuery.data?.list?.[0]

  return (
    <div className="home-shell">
      <section className="hero-panel">
        <div className="hero-content">
          <span className="page-tag">{effectiveIndustryLabel} Offer 导向学习平台</span>
          <h1>围绕 {effectiveIndustryLabel} 把题库训练、AI 面试和学习陪伴放在同一条成长链路里</h1>
          <p className="page-copy">
            当前首页会沿用你最近选择的行业方向，把题库、社区、AI 面试和学习陪伴统一收拢到同一条主线里；首页只保留轻入口，完整互动统一沉淀到独立社区频道。
          </p>

          <div className="hero-actions">
            <Link className="primary-button hero-link-button" to="/practice">
              进入 {effectiveIndustryLabel} 题库
            </Link>
            <Link className="secondary-button hero-link-button" to="/interview">
              打开 {effectiveIndustryLabel} 面试入口
            </Link>
          </div>

          <div className="hero-metrics">
            <article className="metric-card">
              <strong>{effectiveIndustryLabel} 真题训练</strong>
              <span>刷题、错题、收藏、笔记统一沉淀</span>
            </article>
            <article className="metric-card">
              <strong>{effectiveIndustryLabel} AI 面试</strong>
              <span>后续承接流式问答、追问与评分</span>
            </article>
            <article className="metric-card">
              <strong>统一方向上下文</strong>
              <span>学习计划、Live2D、提醒与反馈都沿用当前行业偏好</span>
            </article>
          </div>
        </div>

        <aside className="hero-aside">
          <div className="section-card spotlight-card">
            <span className="section-kicker">今日主线</span>
            <h2>先把 {effectiveIndustryLabel} 题库刷顺，再进 AI 面试</h2>
            <p>题库页已经是当前最完整的业务域，现在首页也会直接沿用当前方向，保证从首屏进入任何频道都不脱节。</p>
          </div>
          <div className="section-card mini-feed-card">
            <span className="section-kicker">近期规划</span>
            <ul className="mini-feed-list">
              <li>{effectiveIndustryLabel} 社区已经支持发帖、评论、点赞和我的帖子管理</li>
              <li>首页动态流只展示精简精选，完整浏览在独立社区页</li>
              <li>{effectiveIndustryLabel} 面试页继续补流式交互与报告体验</li>
            </ul>
          </div>
        </aside>
      </section>

      <section className="home-board">
        <div className="home-main-column">
          <article className="section-card section-card-large">
            <div className="section-head">
              <div>
                <span className="section-kicker">社区精选</span>
                <h2>{effectiveIndustryLabel} 社区最新内容</h2>
              </div>
              <Link className="secondary-button hero-link-button" to="/community">进入社区广场</Link>
            </div>

            {communityQuery.isLoading ? <p className="companion-empty-text">首页动态加载中...</p> : null}
            {communityQuery.isError ? (
              <p className="companion-empty-text">
                {extractErrorMessage(communityQuery.error, '首页动态读取失败，稍后重试。')}
              </p>
            ) : null}
            {communityQuery.data?.list?.length ? (
              <div className="feed-stack">
                {communityQuery.data.list.map((post) => (
                  <article className="feed-item" key={post.id}>
                    <div className="feed-item-head">
                      <strong>{post.title || truncateText(post.summary || post.content, 24)}</strong>
                      <span>{formatRelativeTime(post.created_at)}</span>
                    </div>
                    <p>{truncateText(post.summary || post.content, 120)}</p>
                    <div className="card-inline">
                      <span>{post.author?.username || '匿名用户'} · {post.post_type === 'article' ? '文章' : '动态'}</span>
                      <span>浏览 {post.view_count} · 点赞 {post.like_count}</span>
                    </div>
                    <Link className="secondary-link" to="/community/$postId" params={{ postId: String(post.id) }}>
                      查看帖子
                    </Link>
                  </article>
                ))}
              </div>
            ) : (
              <div className="timeline-item">
                <strong>内容流还没有帖子</strong>
                <p>社区闭环已经接通，后续有人发帖后首页这里会直接显示真实内容。</p>
              </div>
            )}
          </article>

          <article className="section-card section-card-large">
            <div className="section-head">
              <div>
                <span className="section-kicker">题库推荐</span>
                <h2>{effectiveIndustryLabel} 当前可直接开练的题目</h2>
              </div>
              <Link className="secondary-link" to="/practice">查看全部题目</Link>
            </div>

            {practicePreviewQuery.isLoading ? <p className="companion-empty-text">推荐题单加载中...</p> : null}
            {practicePreviewQuery.isError ? (
              <p className="companion-empty-text">
                {extractErrorMessage(practicePreviewQuery.error, '推荐题单读取失败')}
              </p>
            ) : null}
            {practicePreviewQuery.data?.list?.length ? (
              <div className="home-practice-preview-grid">
                {practicePreviewQuery.data.list.map((question) => (
                  <article className="feature-card" key={question.id}>
                    <div className="card-inline">
                      <strong>#{question.id}</strong>
                      <span>{difficultyLabel(question.difficulty)}</span>
                    </div>
                    <h2>{question.title}</h2>
                    <p>题型：{questionTypeLabel(question.type)}</p>
                    <p>分类：{question.category_name || question.category_id}</p>
                    <div className="page-actions">
                      <Link
                        className="secondary-link"
                        to={question.type === 'code' ? '/practice/editor/$questionId' : '/practice/$questionId'}
                        params={{ questionId: String(question.id) }}
                      >
                        进入做题
                      </Link>
                    </div>
                  </article>
                ))}
              </div>
            ) : (
              <div className="timeline-item">
                <strong>当前方向还没有题目推荐</strong>
                <p>如果行业已切换但这里为空，优先检查该行业下的题库数据是否已完成导入。</p>
              </div>
            )}
          </article>
        </div>

        <aside className="home-side-column">
          <article className="section-card sidebar-card">
            <span className="section-kicker">个人工作台</span>
            {accessToken ? (
              <div className="sidebar-links">
                <span className="sidebar-link">今日练习：{statsQuery.data?.today_count ?? '--'}</span>
                <span className="sidebar-link">连续打卡：{statsQuery.data?.streak_days ?? '--'} 天</span>
                <span className="sidebar-link">错题待复习：{collectionsOverviewQuery.data?.wrongQuestions ?? '--'}</span>
                <span className="sidebar-link">学习计划：{currentPlanQuery.data ? `${Math.round(planProgressQuery.data?.progress || currentPlanQuery.data.progress || 0)}%` : '未创建'}</span>
                {latestInterview ? (
                  <Link
                    className="sidebar-link"
                    to={latestInterview.status === 'ongoing' ? '/interview/$interviewId' : '/interview/$interviewId/report'}
                    params={{ interviewId: String(latestInterview.id) }}
                  >
                    {latestInterview.status === 'ongoing' ? '继续最近一场面试' : '查看最近一场报告'}
                  </Link>
                ) : (
                  <Link className="sidebar-link" to="/interview">开始第一场面试</Link>
                )}
              </div>
            ) : (
              <div className="timeline-item">
                <strong>登录后显示你的推进状态</strong>
                <p>首页会汇总今天练习、当前计划和最近面试，避免每次都从各频道单独查看。</p>
              </div>
            )}
          </article>

          <article className="section-card sidebar-card">
            <span className="section-kicker">热门方向</span>
            <div className="tag-cloud">
              {highlightedIndustries.map((industry) => (
                <span key={`${industry.id}-${industry.code}`}>
                  {industry.name}
                  {industry.code === effectiveIndustryCode ? ' · 当前' : ''}
                </span>
              ))}
            </div>
          </article>

          <article className="section-card sidebar-card">
            <span className="section-kicker">社区入口</span>
            <div className="sidebar-links">
              <Link className="sidebar-link" to="/community">浏览全部帖子</Link>
              {accessToken ? (
                <Link className="sidebar-link" to="/community/create">发布刷题复盘</Link>
              ) : (
                <button className="sidebar-link interactive-link-button" type="button" onClick={() => requestLoginPrompt('/community/create', 'missing')}>
                  发布刷题复盘
                </button>
              )}
              {accessToken ? (
                <Link className="sidebar-link" to="/community/mine">管理我的帖子</Link>
              ) : (
                <button className="sidebar-link interactive-link-button" type="button" onClick={() => requestLoginPrompt('/community/create', 'missing')}>
                  登录后发帖
                </button>
              )}
              {accessToken ? (
                <Link className="sidebar-link" to="/practice/notes">把笔记整理成帖子</Link>
              ) : (
                <button className="sidebar-link interactive-link-button" type="button" onClick={() => requestLoginPrompt('/practice/notes', 'missing')}>
                  把笔记整理成帖子
                </button>
              )}
            </div>
          </article>

          <article className="section-card sidebar-card">
            <span className="section-kicker">学习陪伴入口</span>
            <div className="sidebar-links">
              <Link className="sidebar-link" to="/companion">学习计划</Link>
              <Link className="sidebar-link" to="/companion">Live2D 展示</Link>
              {accessToken ? (
                <Link className="sidebar-link" to="/practice/wrong">错题复盘</Link>
              ) : (
                <button className="sidebar-link interactive-link-button" type="button" onClick={() => requestLoginPrompt('/practice/wrong', 'missing')}>
                  错题复盘
                </button>
              )}
              {accessToken ? (
                <Link className="sidebar-link" to="/practice/notes">学习笔记</Link>
              ) : (
                <button className="sidebar-link interactive-link-button" type="button" onClick={() => requestLoginPrompt('/practice/notes', 'missing')}>
                  学习笔记
                </button>
              )}
            </div>
          </article>
        </aside>
      </section>
    </div>
  )
}

/**
 * 提供刷题总入口，当前已接通列表、统计、组卷和练习入口。
 */
function PracticePage() {
  const navigate = useNavigate()
  const accessToken = useAuthStore((state) => state.accessToken)
  const [selectedIndustryCode, setSelectedIndustryCode] = useState(() => readSelectedFrontendIndustryCode() || DEFAULT_FRONTEND_INDUSTRY_CODE)
  const [keywordInput, setKeywordInput] = useState('')
  const [keyword, setKeyword] = useState('')
  const [difficulty, setDifficulty] = useState('')
  const [categoryId, setCategoryId] = useState<number | null>(null)
  const [page, setPage] = useState(1)
  const [examMessage, setExamMessage] = useState('等待组卷')

  const industriesQuery = useQuery({
    queryKey: ['frontend-industries'],
    queryFn: fetchFrontendIndustries,
    staleTime: 5 * 60 * 1000,
  })

  const selectedIndustry = useMemo(
    () => resolvePreferredFrontendIndustry(industriesQuery.data || [], selectedIndustryCode),
    [industriesQuery.data, selectedIndustryCode],
  )
  const effectiveIndustryCode = selectedIndustry?.code || selectedIndustryCode.trim() || DEFAULT_FRONTEND_INDUSTRY_CODE
  const effectiveIndustryLabel = formatFrontendIndustryLabel(selectedIndustry, effectiveIndustryCode)

  const categoriesQuery = useQuery({
    queryKey: ['practice-categories', effectiveIndustryCode],
    queryFn: () => fetchCategories(effectiveIndustryCode),
    enabled: Boolean(effectiveIndustryCode),
  })

  const questionsQuery = useQuery({
    queryKey: ['practice-questions', page, effectiveIndustryCode, selectedIndustry?.id, difficulty, keyword, categoryId],
    queryFn: () => fetchQuestions({
      page,
      pageSize: PRACTICE_PAGE_SIZE,
      difficulty,
      keyword,
      industryId: selectedIndustry?.id || null,
      categoryId,
    }),
  })

  const questionSetsQuery = useQuery({
    queryKey: ['practice-question-sets', selectedIndustry?.id],
    queryFn: () => fetchQuestionSets(selectedIndustry?.id || null),
    enabled: Boolean(selectedIndustry?.id),
  })

  const statsQuery = useQuery({
    queryKey: ['practice-stats', accessToken],
    queryFn: () => fetchPracticeStats(accessToken as string),
    enabled: Boolean(accessToken),
  })

  const collectionsOverviewQuery = useQuery({
    queryKey: ['practice-collections-overview', accessToken],
    queryFn: () => fetchPracticeCollectionsOverview(accessToken as string),
    enabled: Boolean(accessToken),
  })

  const practiceRecommendationsQuery = useQuery({
    queryKey: ['practice-recommendations', accessToken],
    queryFn: () => fetchPracticeRecommendations(accessToken as string, 6),
    enabled: Boolean(accessToken),
  })

  const categoryOptions = useMemo(
    () => flattenCategories(categoriesQuery.data || []),
    [categoriesQuery.data],
  )

  /**
   * 在行业列表恢复后同步前台公共偏好，保证刷题、面试和陪伴使用同一方向上下文。
   */
  useEffect(() => {
    const normalizedIndustryCode = effectiveIndustryCode.trim()
    if (!normalizedIndustryCode) {
      return
    }

    persistSelectedFrontendIndustryCode(normalizedIndustryCode)
    if (normalizedIndustryCode !== selectedIndustryCode) {
      setSelectedIndustryCode(normalizedIndustryCode)
    }
  }, [effectiveIndustryCode, selectedIndustryCode])

  useEffect(() => {
    const pendingKeyword = consumePendingPracticeSearch()
    if (!pendingKeyword) {
      return
    }

    setPage(1)
    setKeywordInput(pendingKeyword)
    setKeyword(pendingKeyword)
  }, [])

  /**
   * 应用搜索条件并回到第一页，避免保留过期分页状态。
   */
  function handleSearchSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setPage(1)
    setKeyword(keywordInput.trim())
  }

  /**
   * 切换刷题行业时重置分类和分页，避免沿用上一行业的筛选状态。
   */
  function handleIndustryChange(nextIndustryCode: string) {
    setPage(1)
    setCategoryId(null)
    setSelectedIndustryCode(nextIndustryCode)
    setExamMessage(`已切换到 ${formatFrontendIndustryLabel(resolvePreferredFrontendIndustry(industriesQuery.data || [], nextIndustryCode), nextIndustryCode)} 题库。`)
  }

  /**
   * 生成随机练习或限时模拟，并跳转到第一道题。
   */
  async function handleGenerateExam(mode: 'random' | 'timed') {
    if (!accessToken) {
      requestLoginPrompt('/practice', 'missing')
      return
    }

    try {
      const exam = await generateExamRequest({
        token: accessToken,
        mode,
        difficulty,
        industryId: selectedIndustry?.id || null,
        categoryId,
      })

      const firstQuestion = exam.questions?.[0]
      if (!firstQuestion) {
        setExamMessage('当前条件下没有可用题目')
        return
      }

      setExamMessage(mode === 'timed' ? '限时模拟已生成' : '随机练习已生成')
      navigate({
        to: firstQuestion.type === 'code' ? '/practice/editor/$questionId' : '/practice/$questionId',
        params: {
          questionId: String(firstQuestion.id),
        },
      })
    } catch (error) {
      if (!useAuthStore.getState().accessToken) {
        requestLoginPrompt('/practice', 'expired')
        return
      }
      setExamMessage(extractErrorMessage(error, '组卷失败'))
    }
  }

  return (
    <section className="page-panel">
      <span className="page-tag">刷题总览</span>
      <h1>刷题模式</h1>
      <p className="page-copy">
        这一版已经接入真实题目列表、练习统计、错题本、收藏夹、笔记和代码题编辑器。当前题库方向：{effectiveIndustryLabel}。
      </p>

      <div className="channel-portal-grid">
        <article className="channel-entry-card">
          <span className="section-kicker">题目训练</span>
          <h2>从筛题到做题</h2>
          <p>按关键词、难度、分类缩小范围，直接进入普通题详情页或代码题编辑器。</p>
          <Link className="secondary-link" to="/practice">当前页继续筛题</Link>
        </article>
        <article className="channel-entry-card">
          <span className="section-kicker">复盘沉淀</span>
          <h2>错题、收藏、笔记</h2>
          <p>把高频错题、值得重做的题和个人题解收束到同一个练习域里，形成复盘闭环。</p>
          {accessToken ? (
            <Link className="secondary-link" to="/practice/notes">直接看笔记</Link>
          ) : (
            <button className="secondary-link interactive-link-button" type="button" onClick={() => requestLoginPrompt('/practice/notes', 'missing')}>
              直接看笔记
            </button>
          )}
        </article>
        <article className="channel-entry-card">
          <span className="section-kicker">模拟练习</span>
          <h2>随机练习与限时模拟</h2>
          <p>从题库入口直接生成练习流，不额外跳后台工具页，保持单一训练主线。</p>
          <button className="secondary-button" type="button" onClick={() => void handleGenerateExam('timed')}>
            立即开始模拟
          </button>
        </article>
      </div>

      <div className="quick-links">
        {accessToken ? (
          <Link className="secondary-link" to="/practice/wrong">进入错题本</Link>
        ) : (
          <button className="secondary-link interactive-link-button" type="button" onClick={() => requestLoginPrompt('/practice/wrong', 'missing')}>
            进入错题本
          </button>
        )}
        {accessToken ? (
          <Link className="secondary-link" to="/practice/favorites">查看收藏夹</Link>
        ) : (
          <button className="secondary-link interactive-link-button" type="button" onClick={() => requestLoginPrompt('/practice/favorites', 'missing')}>
            查看收藏夹
          </button>
        )}
        {accessToken ? (
          <Link className="secondary-link" to="/practice/notes">查看笔记</Link>
        ) : (
          <button className="secondary-link interactive-link-button" type="button" onClick={() => requestLoginPrompt('/practice/notes', 'missing')}>
            查看笔记
          </button>
        )}
      </div>

      {accessToken && statsQuery.data ? (
        <div className="stats-grid">
          <article className="feature-card">
            <h2>累计作答</h2>
            <p>{statsQuery.data.total_answered}</p>
          </article>
          <article className="feature-card">
            <h2>正确数</h2>
            <p>{statsQuery.data.correct_count}</p>
          </article>
          <article className="feature-card">
            <h2>正确率</h2>
            <p>{statsQuery.data.accuracy_rate.toFixed(2)}%</p>
          </article>
          <article className="feature-card">
            <h2>连续练习</h2>
            <p>{statsQuery.data.streak_days} 天</p>
          </article>
          <article className="feature-card">
            <h2>错题待复习</h2>
            <p>{collectionsOverviewQuery.data?.wrongQuestions ?? '-'}</p>
          </article>
          <article className="feature-card">
            <h2>已收藏</h2>
            <p>{collectionsOverviewQuery.data?.favorites ?? '-'}</p>
          </article>
          <article className="feature-card">
            <h2>笔记沉淀</h2>
            <p>{collectionsOverviewQuery.data?.notes ?? '-'}</p>
          </article>
          <article className="feature-card">
            <h2>今日完成</h2>
            <p>{statsQuery.data.today_count}</p>
          </article>
        </div>
      ) : null}

      {accessToken ? (
        <article className="status-card" style={{ marginTop: 24 }}>
          <div className="card-inline">
            <div>
              <span className="section-kicker">对症练习推荐</span>
              <h2>先补最近反复暴露的问题</h2>
            </div>
            <Link className="secondary-link" to="/practice/wrong">查看错题本</Link>
          </div>

          {practiceRecommendationsQuery.isLoading ? (
            <p style={{ marginTop: 12 }}>正在根据最近错因生成推荐...</p>
          ) : null}

          {practiceRecommendationsQuery.isError ? (
            <p style={{ marginTop: 12 }}>
              {extractErrorMessage(practiceRecommendationsQuery.error, '练习推荐加载失败')}
            </p>
          ) : null}

          {practiceRecommendationsQuery.data?.focus_tags.length ? (
            <div className="community-tag-row" style={{ marginTop: 12 }}>
              {practiceRecommendationsQuery.data.focus_tags.map((tag) => (
                <span key={tag}>{tag}</span>
              ))}
            </div>
          ) : null}

          {practiceRecommendationsQuery.data?.items.length ? (
            <div className="grid-cards" style={{ marginTop: 18 }}>
              {practiceRecommendationsQuery.data.items.map((item) => (
                <article className="feature-card" key={`practice-recommendation-${item.question.id}`}>
                  <div className="card-inline">
                    <strong>{item.question.title}</strong>
                    <span>{difficultyLabel(item.question.difficulty)}</span>
                  </div>
                  <p>聚焦标签：{item.focus_tag}</p>
                  <p>{item.reason}</p>
                  <p>推荐优先级：第 {item.priority} 位 · 来源：{item.source_type === 'interview_archive' ? '本场面试' : '最近学习档案'}</p>
                  <p>题型：{questionTypeLabel(item.question.type)}</p>
                  <div className="page-actions" style={{ marginTop: 12 }}>
                    <Link
                      className="secondary-link"
                      to={resolvePracticeRecommendationRoute(item.question.type)}
                      params={{ questionId: String(item.question.id) }}
                    >
                      直接开始补练
                    </Link>
                    {item.topic_code ? (
                      <Link
                        className="secondary-link"
                        to={resolveMistakeTopicRoute()}
                        params={{ topicCode: item.topic_code }}
                      >
                        查看错因专题
                      </Link>
                    ) : null}
                  </div>
                </article>
              ))}
            </div>
          ) : null}

          {!practiceRecommendationsQuery.isLoading && !practiceRecommendationsQuery.isError && !practiceRecommendationsQuery.data?.items.length ? (
            <div className="timeline-item" style={{ marginTop: 18 }}>
              <strong>还没有形成推荐</strong>
              <p>先做几道编程题或主观题，系统会根据错因标签逐步给出更具体的补题建议。</p>
            </div>
          ) : null}
        </article>
      ) : null}

      <article className="status-card" style={{ marginTop: 24 }}>
        <div className="card-inline">
          <div>
            <span className="section-kicker">核心题单</span>
            <h2>{effectiveIndustryLabel} 最值得先打通的主题</h2>
          </div>
          <Link className="secondary-link" to="/practice">继续按筛选做题</Link>
        </div>

        {questionSetsQuery.isLoading ? (
          <p style={{ marginTop: 12 }}>正在整理当前方向的核心题单...</p>
        ) : null}

        {questionSetsQuery.isError ? (
          <p style={{ marginTop: 12 }}>
            {extractErrorMessage(questionSetsQuery.error, '核心题单加载失败')}
          </p>
        ) : null}

        {questionSetsQuery.data?.length ? (
          <div className="grid-cards" style={{ marginTop: 18 }}>
            {questionSetsQuery.data.map((set) => (
              <article className="feature-card" key={set.slug}>
                <div className="card-inline">
                  <strong>{set.title}</strong>
                  <span>{set.question_count} 题预览</span>
                </div>
                <p>{set.description}</p>
                {set.focus_tags.length ? (
                  <div className="community-tag-row" style={{ marginTop: 12 }}>
                    {set.focus_tags.map((tag) => (
                      <span key={`${set.slug}-${tag}`}>{tag}</span>
                    ))}
                  </div>
                ) : null}
                {set.questions.length ? (
                  <div style={{ marginTop: 12 }}>
                    {set.questions.map((item) => (
                      <div key={`${set.slug}-question-${item.id}`} style={{ marginBottom: 8 }}>
                        <Link
                          className="secondary-link"
                          to={resolvePracticeTarget(item.id, item.type)}
                          params={{ questionId: String(item.id) }}
                        >
                          {item.title}
                        </Link>
                      </div>
                    ))}
                  </div>
                ) : null}
              </article>
            ))}
          </div>
        ) : null}

        {!questionSetsQuery.isLoading && !questionSetsQuery.isError && !questionSetsQuery.data?.length ? (
          <div className="timeline-item" style={{ marginTop: 18 }}>
            <strong>当前方向还没有整理出核心题单</strong>
            <p>优先补齐该行业下的高价值题目后，这里会自动收敛成更稳定的主题入口。</p>
          </div>
        ) : null}
      </article>

      <form className="stack-form" onSubmit={handleSearchSubmit}>
        <label className="field">
          <span>行业筛选</span>
          <select
            value={effectiveIndustryCode}
            disabled={industriesQuery.isLoading || !industriesQuery.data?.length}
            onChange={(event) => handleIndustryChange(event.target.value)}
          >
            {industriesQuery.data?.map((industry) => (
              <option key={industry.id} value={industry.code}>
                {industry.name}
              </option>
            ))}
            {!industriesQuery.data?.length ? (
              <option value={effectiveIndustryCode}>{effectiveIndustryLabel}</option>
            ) : null}
          </select>
        </label>

        <label className="field">
          <span>搜索题目</span>
          <input
            value={keywordInput}
            onChange={(event) => setKeywordInput(event.target.value)}
            placeholder="输入关键词"
          />
        </label>

        <label className="field">
          <span>难度筛选</span>
          <select
            value={difficulty}
            onChange={(event) => {
              setPage(1)
              setDifficulty(event.target.value)
            }}
          >
            <option value="">全部</option>
            <option value="easy">简单</option>
            <option value="medium">中等</option>
            <option value="hard">困难</option>
          </select>
        </label>

        <label className="field">
          <span>分类筛选</span>
          <select
            value={categoryId || ''}
            onChange={(event) => {
              setPage(1)
              setCategoryId(event.target.value ? Number(event.target.value) : null)
            }}
          >
            <option value="">全部分类</option>
            {categoryOptions.map((item) => (
              <option key={item.id} value={item.id}>{item.name}</option>
            ))}
          </select>
        </label>

        {industriesQuery.isError ? (
          <p className="companion-empty-text">
            {extractErrorMessage(industriesQuery.error, '行业列表读取失败，当前将回退到默认题库方向。')}
          </p>
        ) : null}

        <div className="page-actions">
          <button className="primary-button" type="submit">
            搜索
          </button>
          <button className="secondary-button" type="button" onClick={() => void handleGenerateExam('random')}>
            随机练习
          </button>
          <button className="secondary-button" type="button" onClick={() => void handleGenerateExam('timed')}>
            限时模拟
          </button>
        </div>
      </form>

      <div className="status-card" style={{ marginTop: 24 }}>
        练习提示：{examMessage}
      </div>

      {questionsQuery.isLoading ? (
        <div className="status-card" style={{ marginTop: 24 }}>题目列表加载中...</div>
      ) : null}

      {questionsQuery.isError ? (
        <div className="status-card" style={{ marginTop: 24 }}>
          {questionsQuery.error instanceof Error ? questionsQuery.error.message : '题目列表加载失败'}
        </div>
      ) : null}

      {questionsQuery.data ? (
        <>
          <div className="grid-cards" style={{ marginTop: 24 }}>
            {questionsQuery.data.list.map((question) => (
              <article className="feature-card" key={question.id}>
                <div className="card-inline">
                  <strong>#{question.id}</strong>
                  <span>{difficultyLabel(question.difficulty)}</span>
                </div>
                <h2>{question.title}</h2>
                <p>行业：{formatFrontendIndustryLabel(findFrontendIndustryById(industriesQuery.data || [], question.industry_id), effectiveIndustryCode)}</p>
                <p>题型：{questionTypeLabel(question.type)}</p>
                <p>分类：{question.category_name || question.category_id}</p>
                <p>通过率：{typeof question.pass_rate === 'number' ? `${question.pass_rate}%` : '暂无'}</p>
                <div style={{ marginTop: 12 }}>
                  <Link className="secondary-link" to={question.type === 'code' ? '/practice/editor/$questionId' : '/practice/$questionId'} params={{ questionId: String(question.id) }}>
                    进入做题
                  </Link>
                </div>
              </article>
            ))}
          </div>

          <div className="card-inline" style={{ marginTop: 24 }}>
            <span>共 {questionsQuery.data.total} 题</span>
            <div className="page-actions">
              <button
                className="secondary-button"
                type="button"
                disabled={page <= 1}
                onClick={() => setPage((current) => Math.max(current - 1, 1))}
              >
                上一页
              </button>
              <span>第 {page} 页</span>
              <button
                className="secondary-button"
                type="button"
                disabled={questionsQuery.data.list.length < PRACTICE_PAGE_SIZE}
                onClick={() => setPage((current) => current + 1)}
              >
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
 * 提供题目详情与答题页，支持单选、多选和主观题提交。
 */
function PracticeQuestionPage() {
  const navigate = useNavigate()
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
  })
  const industriesQuery = useQuery({
    queryKey: ['frontend-industries'],
    queryFn: fetchFrontendIndustries,
    staleTime: 5 * 60 * 1000,
  })

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
function PracticeEditorPage() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const { questionId } = useParams({ from: '/practice/editor/$questionId' })
  const accessToken = useAuthStore((state) => state.accessToken)
  const editorContainerRef = useRef<HTMLDivElement | null>(null)
  const editorInstanceRef = useRef<any>(null)
  const monacoRef = useRef<any>(null)
  const [editorLanguage, setEditorLanguage] = useState('go')
  const [codeContent, setCodeContent] = useState(buildDefaultCodeTemplate())
  const [submitting, setSubmitting] = useState(false)
  const [submitMessage, setSubmitMessage] = useState('等待运行')
  const [submitResult, setSubmitResult] = useState<SubmitAnswerResult | null>(null)
  const [favoriteMessage, setFavoriteMessage] = useState('未操作')
  const [favoriteState, setFavoriteState] = useState(false)
  const [startedAt] = useState(() => Date.now())

  const detailQuery = useQuery({
    queryKey: ['practice-code-question-detail', questionId],
    queryFn: () => fetchQuestionDetail(questionId),
  })
  const industriesQuery = useQuery({
    queryKey: ['frontend-industries'],
    queryFn: fetchFrontendIndustries,
    staleTime: 5 * 60 * 1000,
  })

  const question = detailQuery.data
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
      const initialValue = readCodeDraft(question.id, editorLanguage, buildDefaultCodeTemplate())
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
  }, [question?.id, question?.type])

  useEffect(() => {
    if (!question || question.type !== 'code') {
      return
    }

    const draft = readCodeDraft(question.id, editorLanguage, buildDefaultCodeTemplate())
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
    const template = buildDefaultCodeTemplate()
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
   * 提交代码题答案，并返回后端当前的分析结果。
   */
  async function handleEvaluate(label: '运行代码' | '提交代码') {
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
    setSubmitMessage(`${label}中...`)

    try {
      const result = await submitAnswerRequest(
        accessToken,
        question.id,
        codeContent,
        Math.max(Math.round((Date.now() - startedAt) / 1000), 1),
      )

      setSubmitResult(result)
      setSubmitMessage(result.is_correct ? `${label}通过` : `${label}完成`)
      await queryClient.invalidateQueries({ queryKey: ['practice-stats'] })
      await queryClient.invalidateQueries({ queryKey: ['practice-wrong'] })
      await queryClient.invalidateQueries({ queryKey: ['practice-recommendations'] })
    } catch (error) {
      if (!useAuthStore.getState().accessToken) {
        requestLoginPrompt(readCurrentBrowserPath(), 'expired')
        return
      }
      setSubmitMessage(extractErrorMessage(error, `${label}失败`))
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

  if (detailQuery.isLoading) {
    return (
      <section className="page-panel">
        <span className="page-tag">代码编辑器</span>
        <div className="status-card" style={{ marginTop: 24 }}>题目详情加载中...</div>
      </section>
    )
  }

  if (detailQuery.isError) {
    return (
      <section className="page-panel">
        <span className="page-tag">代码编辑器</span>
        <div className="status-card" style={{ marginTop: 24 }}>
          {detailQuery.error instanceof Error ? detailQuery.error.message : '题目详情加载失败'}
        </div>
      </section>
    )
  }

  if (question && question.type !== 'code') {
    return (
      <section className="page-panel">
        <span className="page-tag">代码编辑器</span>
        <h1>当前题目不是代码题</h1>
        <p className="page-copy">这道题更适合走普通答题页，不需要 Monaco 编辑器。</p>
        <Link className="secondary-link" to="/practice/$questionId" params={{ questionId }}>
          返回普通题目页
        </Link>
      </section>
    )
  }

  return (
    <>
      <div className="companion-room-toolbar" style={{ marginBottom: 20 }}>
        <div className="page-actions">
          <button className="ghost-button" type="button" onClick={handleGoBackFromEditor}>
            返回上一页
          </button>
          <Link className="ghost-button" to="/practice">
            返回题库
          </Link>
        </div>
        <span className="companion-room-note">代码题编辑器 · {questionIndustryLabel}</span>
      </div>

      <section className="editor-layout">
        <div className="editor-sidebar">
          <span className="page-tag">代码练习</span>
          <h1>{question?.title || `题目 #${questionId}`}</h1>
          <p className="page-copy">{question?.content || '题目详情加载中...'}</p>
          <p className="companion-empty-text">当前行业：{questionIndustryLabel}</p>
          {question?.tag_list?.length ? (
            <div className="community-tag-row" style={{ marginTop: 12 }}>
              {question.tag_list.map((tag) => (
                <span key={`editor-question-tag-${tag}`}>{tag}</span>
              ))}
            </div>
          ) : null}
          <div className="page-actions" style={{ marginTop: 16 }}>
            <button className="secondary-button" type="button" onClick={() => void handleToggleFavorite()}>
              {favoriteState ? '取消收藏' : '加入收藏'}
            </button>
            <Link className="secondary-link" to="/practice/$questionId" params={{ questionId }}>
              查看题目详情
            </Link>
          </div>
          <div style={{ marginTop: 12 }}>收藏状态：{favoriteMessage}</div>
          {question ? (
            <QuestionNotePanel questionId={question.id} questionTitle={question.title} token={accessToken} />
          ) : null}
        </div>

        <div className="editor-main">
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
              <button className="secondary-button" type="button" disabled={submitting} onClick={() => void handleEvaluate('运行代码')}>
                运行代码
              </button>
              <button className="primary-button" type="button" disabled={submitting} onClick={() => void handleEvaluate('提交代码')}>
                提交代码
              </button>
            </div>
          </div>

          <div className="editor-surface" ref={editorContainerRef} />

          <div className="status-card" style={{ marginTop: 16 }}>
            <div>执行状态：{submitMessage}</div>
            {submitResult ? (
              <>
                <div>判定结果：{submitResult.is_correct ? '正确' : '错误'}</div>
                <div>正确答案：{submitResult.correct_answer || '未返回'}</div>
                <div>解析说明：{submitResult.explanation || '暂无解析'}</div>
                {submitResult.ai_analysis ? (
                  <pre className="analysis-block">{submitResult.ai_analysis}</pre>
                ) : null}
                {question?.solution ? (
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
                <MistakeTopicHighlights
                  tags={practiceAnalysis?.mistake_tags || []}
                  title="建议继续补的错因专题"
                />
              </>
            ) : null}
          </div>
        </div>
      </section>
    </>
  )
}

/**
 * 提供错题本页面，帮助用户回看最近仍未纠正的题目。
 */
function PracticeWrongPage() {
  const accessToken = useAuthStore((state) => state.accessToken)
  const [page, setPage] = useState(1)

  const wrongQuery = useQuery({
    queryKey: ['practice-wrong', accessToken, page],
    queryFn: () => fetchWrongQuestions(accessToken as string, page, PRACTICE_PAGE_SIZE),
    enabled: Boolean(accessToken),
  })

  return (
    <section className="page-panel">
      <span className="page-tag">错题本</span>
      <h1>我的错题</h1>
      <p className="page-copy">这里展示当前最新一次仍答错的题目，方便集中回顾和重做。</p>

      {wrongQuery.isLoading ? (
        <div className="status-card" style={{ marginTop: 24 }}>错题列表加载中...</div>
      ) : null}

      {wrongQuery.isError ? (
        <div className="status-card" style={{ marginTop: 24 }}>
          {wrongQuery.error instanceof Error ? wrongQuery.error.message : '错题列表加载失败'}
        </div>
      ) : null}

      {wrongQuery.data ? (
        <>
          <div className="grid-cards">
            {wrongQuery.data.list.map((item) => (
              <article className="feature-card" key={item.id}>
                <h2>{item.question?.title || `题目 #${item.question_id}`}</h2>
                <p>题型：{questionTypeLabel(item.question?.type || '')}</p>
                <p>我的答案：{item.user_answer || '-'}</p>
                <p>错题时间：{formatDateTime(item.created_at)}</p>
                <Link
                  className="secondary-link"
                  to={(item.question?.type || '') === 'code' ? '/practice/editor/$questionId' : '/practice/$questionId'}
                  params={{ questionId: String(item.question_id) }}
                >
                  重新练习
                </Link>
              </article>
            ))}
          </div>

          <div className="card-inline" style={{ marginTop: 24 }}>
            <span>共 {wrongQuery.data.total} 条错题记录</span>
            <div className="page-actions">
              <button className="secondary-button" type="button" disabled={page <= 1} onClick={() => setPage((current) => current - 1)}>
                上一页
              </button>
              <span>第 {page} 页</span>
              <button className="secondary-button" type="button" disabled={wrongQuery.data.list.length < PRACTICE_PAGE_SIZE} onClick={() => setPage((current) => current + 1)}>
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
 * 提供收藏夹页面，集中展示用户保留待复习的题目。
 */
function PracticeFavoritesPage() {
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
function PracticeNotesPage() {
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
 * 根据练习分析里的错因标签展示可继续深挖的专题卡片。
 */
function MistakeTopicHighlights(props: { tags: string[]; title?: string }) {
  const topicsQuery = useQuery({
    queryKey: ['mistake-topics-catalog'],
    queryFn: () => fetchMistakeTopics([]),
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
 * 展示单个错因专题详情，承接报告页、成长页和练习反馈页的深挖入口。
 */
function MistakeTopicPage() {
  const navigate = useNavigate()
  const { topicCode } = useParams({ from: '/practice/topics/$topicCode' })
  const topicQuery = useQuery({
    queryKey: ['mistake-topic-detail', topicCode],
    queryFn: () => fetchMistakeTopic(topicCode),
    enabled: Boolean(topicCode),
  })

  /**
   * 以当前专题标签或关联题单预填题库搜索，帮助用户直接进入补练状态。
   */
  function handleOpenTopicPractice(questionSetSlug = ''): void {
    if (topicQuery.data) {
      persistPracticeFocusSearch(questionSetSlug, [topicQuery.data.tag], topicQuery.data.title)
    }
    navigate({ to: '/practice' })
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
              <button className="secondary-button" type="button" onClick={() => handleOpenTopicPractice()}>
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

/**
 * 提供学习陪伴频道首页，整合学习计划、陪伴角色和成长记录的入口设计。
 */
function CompanionPage() {
  return <CompanionHubPage />
}

/**
 * 提供前台登录页面，并在成功后跳转到成长档案页。
 */
function LoginPage() {
  const navigate = useNavigate()
  const redirectTarget = useRouterState({
    select: (state) => resolveLoginRedirectTarget((state.location.search as Record<string, unknown> | undefined)?.redirect),
  })
  const login = useAuthStore((state) => state.login)
  const loading = useAuthStore((state) => state.loading)
  const accessToken = useAuthStore((state) => state.accessToken)
  const user = useAuthStore((state) => state.user)
  const [form, setForm] = useState({
    email: '',
    password: '',
  })
  const [message, setMessage] = useState('等待提交')

  /**
   * 提交登录表单，并在成功后进入已受保护的成长档案页面。
   */
  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const result = await login(form.email.trim(), form.password)
    setMessage(result.message)

    if (result.ok) {
      if (typeof window !== 'undefined') {
        window.location.replace(redirectTarget)
        return
      }

      navigate({
        to: '/growth',
        replace: true,
      })
    }
  }

  /**
   * 对令牌做短截断展示，便于判断写入状态而不直接暴露完整值。
   */
  function maskToken(token: string | null): string {
    if (!token) {
      return '未写入'
    }

    return `${token.slice(0, 12)}...`
  }

  const tokenPreview = useMemo(() => maskToken(accessToken), [accessToken])

  return (
    <section className="page-panel narrow-panel">
      <span className="page-tag">登录链路</span>
      <h1>前台登录</h1>
      <p className="page-copy">
        当前表单会直接请求 `/auth/login`，随后自动拉取 `/user/profile`，并把会话写入本地存储。
      </p>

      <form className="stack-form" onSubmit={handleSubmit}>
        <label className="field">
          <span>邮箱</span>
          <input
            value={form.email}
            onChange={(event) => setForm((current) => ({ ...current, email: event.target.value }))}
            placeholder="请输入邮箱"
          />
        </label>
        <label className="field">
          <span>密码</span>
          <input
            type="password"
            value={form.password}
            onChange={(event) => setForm((current) => ({ ...current, password: event.target.value }))}
            placeholder="请输入密码"
          />
        </label>
        <button className="primary-button" type="submit" disabled={loading}>
          {loading ? '提交中...' : '登录'}
        </button>
      </form>

      <div className="status-card">
        <div>令牌状态：{tokenPreview}</div>
        <div>用户资料：{user?.username || '未同步'}</div>
        <div>接口结果：{message}</div>
      </div>
    </section>
  )
}

const rootRoute = createRootRouteWithContext<RouterContext>()({
  component: RootLayout,
})

const indexRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/',
  component: HomePage,
})

const communityRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: 'community',
  component: CommunityPage,
})

const communityCreateRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: 'community/create',
  beforeLoad: async ({ location }) => {
    if (!getLatestAccessToken()) {
      throw redirect({
        to: '/auth/login',
        search: buildLoginRedirectSearch(buildCurrentLocationPath(location.pathname, location.searchStr || '')),
      })
    }
  },
  component: CommunityCreatePostPage,
})

const communityMineRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: 'community/mine',
  beforeLoad: async ({ location }) => {
    if (!getLatestAccessToken()) {
      throw redirect({
        to: '/auth/login',
        search: buildLoginRedirectSearch(buildCurrentLocationPath(location.pathname, location.searchStr || '')),
      })
    }
  },
  component: CommunityMyPostsPage,
})

const communityEditRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: 'community/$postId/edit',
  beforeLoad: async ({ location }) => {
    if (!getLatestAccessToken()) {
      throw redirect({
        to: '/auth/login',
        search: buildLoginRedirectSearch(buildCurrentLocationPath(location.pathname, location.searchStr || '')),
      })
    }
  },
  component: CommunityEditPostPage,
})

const communityDetailRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: 'community/$postId',
  component: CommunityPostDetailPage,
})

const practiceRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: 'practice',
  component: PracticePage,
})

const practiceQuestionRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: 'practice/$questionId',
  component: PracticeQuestionPage,
})

const practiceEditorRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: 'practice/editor/$questionId',
  beforeLoad: async ({ location }) => {
    if (!getLatestAccessToken()) {
      throw redirect({
        to: '/auth/login',
        search: buildLoginRedirectSearch(buildCurrentLocationPath(location.pathname, location.searchStr || '')),
      })
    }
  },
  component: PracticeEditorPage,
})

const practiceWrongRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: 'practice/wrong',
  beforeLoad: async ({ location }) => {
    if (!getLatestAccessToken()) {
      throw redirect({
        to: '/auth/login',
        search: buildLoginRedirectSearch(buildCurrentLocationPath(location.pathname, location.searchStr || '')),
      })
    }
  },
  component: PracticeWrongPage,
})

const practiceFavoritesRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: 'practice/favorites',
  beforeLoad: async ({ location }) => {
    if (!getLatestAccessToken()) {
      throw redirect({
        to: '/auth/login',
        search: buildLoginRedirectSearch(buildCurrentLocationPath(location.pathname, location.searchStr || '')),
      })
    }
  },
  component: PracticeFavoritesPage,
})

const practiceNotesRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: 'practice/notes',
  beforeLoad: async ({ location }) => {
    if (!getLatestAccessToken()) {
      throw redirect({
        to: '/auth/login',
        search: buildLoginRedirectSearch(buildCurrentLocationPath(location.pathname, location.searchStr || '')),
      })
    }
  },
  component: PracticeNotesPage,
})

const practiceTopicRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: 'practice/topics/$topicCode',
  component: MistakeTopicPage,
})

const interviewRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: 'interview',
  component: InterviewHubPage,
})

const interviewSessionRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: 'interview/$interviewId',
  beforeLoad: async ({ location }) => {
    if (!getLatestAccessToken()) {
      throw redirect({
        to: '/auth/login',
        search: buildLoginRedirectSearch(buildCurrentLocationPath(location.pathname, location.searchStr || '')),
      })
    }
  },
  component: InterviewSessionPage,
})

const interviewReportRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: 'interview/$interviewId/report',
  beforeLoad: async ({ location }) => {
    if (!getLatestAccessToken()) {
      throw redirect({
        to: '/auth/login',
        search: buildLoginRedirectSearch(buildCurrentLocationPath(location.pathname, location.searchStr || '')),
      })
    }
  },
  component: InterviewReportPage,
})

const companionRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: 'companion',
  component: CompanionPage,
})

const companionRoomRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: 'companion/room',
  component: CompanionWorkspacePage,
})

const growthRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: 'growth',
  beforeLoad: async ({ location }) => {
    if (!getLatestAccessToken()) {
      throw redirect({
        to: '/auth/login',
        search: buildLoginRedirectSearch(buildCurrentLocationPath(location.pathname, location.searchStr || '')),
      })
    }

    const ready = await useAuthStore.getState().ensureProfile()
    if (!ready) {
      throw redirect({
        to: '/auth/login',
        search: buildLoginRedirectSearch(buildCurrentLocationPath(location.pathname, location.searchStr || '')),
      })
    }
  },
  component: GrowthPage,
})

const loginRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: 'auth/login',
  validateSearch: (search: Record<string, unknown>) => ({
    redirect: typeof search.redirect === 'string' ? search.redirect : undefined,
  }),
  component: LoginPage,
})

const workspaceRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: 'workspace',
  beforeLoad: async ({ location }) => {
    if (!getLatestAccessToken()) {
      throw redirect({
        to: '/auth/login',
        search: buildLoginRedirectSearch(buildCurrentLocationPath(location.pathname, location.searchStr || '')),
      })
    }

    const ready = await useAuthStore.getState().ensureProfile()
    if (!ready) {
      throw redirect({
        to: '/auth/login',
        search: buildLoginRedirectSearch(buildCurrentLocationPath(location.pathname, location.searchStr || '')),
      })
    }
  },
  component: GrowthPage,
})

const routeTree = rootRoute.addChildren([
  indexRoute,
  communityRoute,
  communityCreateRoute,
  communityMineRoute,
  communityEditRoute,
  communityDetailRoute,
  practiceRoute,
  practiceQuestionRoute,
  practiceEditorRoute,
  practiceWrongRoute,
  practiceFavoritesRoute,
  practiceNotesRoute,
  practiceTopicRoute,
  interviewRoute,
  interviewSessionRoute,
  interviewReportRoute,
  companionRoute,
  companionRoomRoute,
  growthRoute,
  loginRoute,
  workspaceRoute,
])

export const router = createRouter({
  routeTree,
  context: {
    queryClient: undefined as never,
  },
})

declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router
  }
}
