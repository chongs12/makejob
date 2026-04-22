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
import { extractErrorMessage, requestJson } from '@makejob/api-client'
import { isSuccessCode, type ApiEnvelope } from '@makejob/shared-types'
import { useAuthStore } from './state/auth'
import { CompanionHubPage, CompanionWorkspacePage } from './features/companion/CompanionPage'
import { InterviewHubPage, InterviewReportPage, InterviewSessionPage } from './features/interview/InterviewPage'
import {
  DEFAULT_FRONTEND_INDUSTRY_CODE,
  type FrontendIndustry,
  fetchFrontendIndustries,
  formatFrontendIndustryLabel,
  persistSelectedFrontendIndustryCode,
  readSelectedFrontendIndustryCode,
  resolvePreferredFrontendIndustry,
  subscribeFrontendIndustryCodeChange,
} from './shared/industryContext'

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
const PENDING_PRACTICE_SEARCH_KEY = 'makejob.practice.pending-search'

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
 * 根据行业主键从前台行业列表中定位真实行业对象，供详情页展示真实方向名称。
 */
function findFrontendIndustryById(industries: FrontendIndustry[], industryId?: number): FrontendIndustry | null {
  if (!industryId) {
    return null
  }

  return industries.find((item) => item.id === industryId) || null
}

/**
 * 统一维护前台行业偏好，让导航、首页和工作台能共享同一份方向上下文。
 */
function useFrontendIndustryPreference() {
  const [selectedIndustryCode, setSelectedIndustryCode] = useState(() => readSelectedFrontendIndustryCode() || DEFAULT_FRONTEND_INDUSTRY_CODE)

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

  useEffect(() => {
    const unsubscribe = subscribeFrontendIndustryCodeChange((industryCode) => {
      setSelectedIndustryCode(industryCode || DEFAULT_FRONTEND_INDUSTRY_CODE)
    })

    return unsubscribe
  }, [])

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

  return {
    industriesQuery,
    selectedIndustry,
    selectedIndustryCode,
    setSelectedIndustryCode,
    effectiveIndustryCode,
    effectiveIndustryLabel,
  }
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
 * 暂存顶部导航发起的题库搜索词，供题库页在跳转后立即接管并执行筛选。
 */
function persistPendingPracticeSearch(keyword: string): void {
  if (typeof window === 'undefined') {
    return
  }

  if (keyword.trim()) {
    window.localStorage.setItem(PENDING_PRACTICE_SEARCH_KEY, keyword.trim())
  } else {
    window.localStorage.removeItem(PENDING_PRACTICE_SEARCH_KEY)
  }
}

/**
 * 读取并清空待执行的题库搜索词，避免同一关键字在后续页面切换中反复触发。
 */
function consumePendingPracticeSearch(): string {
  if (typeof window === 'undefined') {
    return ''
  }

  const keyword = window.localStorage.getItem(PENDING_PRACTICE_SEARCH_KEY) || ''
  window.localStorage.removeItem(PENDING_PRACTICE_SEARCH_KEY)
  return keyword.trim()
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
      setMessage('请先登录后再保存笔记')
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
      setMessage(extractErrorMessage(error, '保存笔记失败'))
    } finally {
      setSaving(false)
    }
  }

  /**
   * 删除当前题目的已有笔记，并同步清空编辑内容。
   */
  async function handleDelete() {
    if (!props.token || !noteQuery.data?.id) {
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
 * 提供前台统一外壳，承载导航、状态展示和子路由出口。
 */
function RootLayout() {
  const navigate = useNavigate()
  const pathname = useRouterState({
    select: (state) => state.location.pathname,
  })
  const accessToken = useAuthStore((state) => state.accessToken)
  const user = useAuthStore((state) => state.user)
  const logout = useAuthStore((state) => state.logout)
  const [headerKeyword, setHeaderKeyword] = useState('')
  const { effectiveIndustryLabel } = useFrontendIndustryPreference()

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
   * 处理顶部“发布”入口，当前阶段统一引导到登录或工作台，后续再接入说说与论坛发布流。
   */
  function handlePublish() {
    navigate({
      to: accessToken ? '/workspace' : '/auth/login',
    })
  }

  const navigationItems = [
    { to: '/', label: '首页', match: pathname === '/' },
    { to: '/practice', label: '题库', match: pathname.startsWith('/practice') },
    { to: '/interview', label: '面试', match: pathname.startsWith('/interview') },
    { to: '/companion', label: '学习陪伴', match: pathname.startsWith('/companion') },
  ]
  const accountLabel = accessToken ? (user?.username || '工作台') : '登录'
  const isStandaloneCompanionRoom = pathname.startsWith('/companion/room')

  if (isStandaloneCompanionRoom) {
    return (
      <div className="app-shell companion-room-shell">
        <AuthBootstrap />
        <Outlet />
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
            <Link className="nav-action-link" to={accessToken ? '/workspace' : '/auth/login'}>
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
      pageSize: 4,
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
  const currentPlanNextTask = currentPlanQuery.data?.tasks.find((task) => task.status !== 'completed' && task.status !== 'skipped') || currentPlanQuery.data?.tasks[0] || null
  const latestInterview = interviewHistoryQuery.data?.list?.[0]

  return (
    <div className="home-shell">
      <section className="hero-panel">
        <div className="hero-content">
          <span className="page-tag">{effectiveIndustryLabel} Offer 导向学习平台</span>
          <h1>围绕 {effectiveIndustryLabel} 把题库训练、AI 面试和学习陪伴放在同一条成长链路里</h1>
          <p className="page-copy">
            当前首页会沿用你最近选择的行业方向，把题库、面试和学习陪伴统一收拢到同一条主线里；后续继续补说说、论坛、面经和动态流时，也会默认围绕这条方向展开。
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
              <li>{effectiveIndustryLabel} 题库体验统一收口到 React 版前台</li>
              <li>首页内容流按当前方向给出更贴近的引导</li>
              <li>{effectiveIndustryLabel} 面试页挂载 AI 流式交互</li>
            </ul>
          </div>
        </aside>
      </section>

      <section className="channel-grid">
        <article className="channel-card channel-card-practice">
          <span className="section-kicker">题库</span>
          <h2>把 {effectiveIndustryLabel} 刷题、复盘、错题本串起来</h2>
          <p>这里承接当前方向的题目列表、筛选、代码题、收藏、错题、笔记和模拟练习。</p>
          <Link className="secondary-link" to="/practice">前往题库</Link>
        </article>
        <article className="channel-card channel-card-interview">
          <span className="section-kicker">面试</span>
          <h2>{effectiveIndustryLabel} AI 面试页面入口</h2>
          <p>后续承接岗位定制追问、语音链路、流式输出和结构化点评。</p>
          <Link className="secondary-link" to="/interview">查看入口</Link>
        </article>
        <article className="channel-card channel-card-companion">
          <span className="section-kicker">学习陪伴</span>
          <h2>{effectiveIndustryLabel} 学习计划与 Live2D</h2>
          <p>学习计划、提醒机制、陪伴角色和学习反馈都会沿用当前方向聚合在这里。</p>
          <Link className="secondary-link" to="/companion">进入陪伴区</Link>
        </article>
      </section>

      <section className="section-card section-card-large">
        <div className="section-head">
          <div>
            <span className="section-kicker">站点主结构</span>
            <h2>一级栏目按照学习闭环来组织，而不是按照后台菜单来组织</h2>
          </div>
        </div>

        <div className="site-map-grid">
          <article className="architecture-card">
            <span className="section-kicker">首页</span>
            <h3>内容流与发布入口</h3>
            <p>承接说说、论坛、面经、推荐内容和用户动态，先让站点具备“社区首页”的内容氛围。</p>
          </article>
          <article className="architecture-card">
            <span className="section-kicker">题库</span>
            <h3>训练与复盘主战场</h3>
            <p>题目列表、做题、错题、收藏、笔记、模拟练习全部聚合到同一业务域，形成高频使用区。</p>
          </article>
          <article className="architecture-card">
            <span className="section-kicker">面试</span>
            <h3>AI 面试与面经沉淀</h3>
            <p>后续承接岗位模拟、实时追问、结构化报告和面经内容，不再作为边角功能存在。</p>
          </article>
          <article className="architecture-card">
            <span className="section-kicker">学习陪伴</span>
            <h3>计划、提醒与陪伴角色</h3>
            <p>学习计划、学习状态、Live2D 和反馈机制集中在一个入口，避免功能分散。</p>
          </article>
        </div>
      </section>

      <section className="home-board">
        <div className="home-main-column">
          <article className="section-card section-card-large">
            <div className="section-head">
              <div>
                <span className="section-kicker">首页动态流</span>
                <h2>{effectiveIndustryLabel} 首页动态流已经开始接真实内容</h2>
              </div>
              <Link className="secondary-button hero-link-button" to="/practice">进入题库主战场</Link>
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
                  </article>
                ))}
              </div>
            ) : (
              <div className="timeline-item">
                <strong>内容流还没有帖子</strong>
                <p>当前接口已经接通，等社区内容开始沉淀后，这里会直接展示真实动态，而不是继续写死演示文案。</p>
              </div>
            )}
          </article>

          <article className="section-card section-card-large">
            <div className="section-head">
              <div>
                <span className="section-kicker">当前推进</span>
                <h2>围绕 {effectiveIndustryLabel} 的下一步操作</h2>
              </div>
            </div>
            <div className="timeline-list">
              <div className="timeline-item">
                <strong>1. 进入 {effectiveIndustryLabel} 题库开始练习</strong>
                <p>
                  {practicePreviewQuery.data?.total
                    ? `当前方向已经可用 ${practicePreviewQuery.data.total} 道题，先从首页推荐题单切入。`
                    : '按难度、分类、关键词快速筛出当前阶段该刷的题。'}
                </p>
              </div>
              <div className="timeline-item">
                <strong>2. 用学习计划把训练节奏固定下来</strong>
                <p>
                  {currentPlanQuery.data
                    ? `当前计划《${currentPlanQuery.data.title}》正在推进，${Math.round(planProgressQuery.data?.progress || currentPlanQuery.data.progress || 0)}% 已完成。`
                    : '如果还没有计划，直接进入学习陪伴页生成一份当前方向的学习计划。'}
                </p>
              </div>
              <div className="timeline-item">
                <strong>3. 把刷题结果带到 {effectiveIndustryLabel} AI 面试</strong>
                <p>
                  {latestInterview
                    ? `最近一场面试状态为${latestInterview.status === 'ongoing' ? '进行中' : '已完成'}，可以继续进入面试链路做追问和复述训练。`
                    : '等题库链路稳定后，再进入 AI 面试页做追问与复述训练。'}
                </p>
              </div>
            </div>
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
              <div className="grid-cards">
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
            <span className="section-kicker">当前计划摘要</span>
            {accessToken && currentPlanQuery.data ? (
              <div className="timeline-item">
                <strong>{currentPlanQuery.data.title}</strong>
                <p>{truncateText(currentPlanQuery.data.description || '当前计划暂无说明。', 72)}</p>
                <p>
                  进度 {Math.round(planProgressQuery.data?.progress || currentPlanQuery.data.progress || 0)}% ·
                  已完成 {planProgressQuery.data?.completed_tasks ?? currentPlanQuery.data.completed_tasks}/{planProgressQuery.data?.total_tasks ?? currentPlanQuery.data.total_tasks}
                </p>
                <p>{currentPlanNextTask ? `下一项：Day ${currentPlanNextTask.day_number} · ${currentPlanNextTask.title}` : '当前计划没有可展示任务。'}</p>
              </div>
            ) : (
              <div className="sidebar-links">
                <Link className="sidebar-link" to="/companion">生成学习计划</Link>
                <Link className="sidebar-link" to="/companion">打开陪伴页</Link>
                <Link className="sidebar-link" to="/practice/wrong">查看错题复盘</Link>
                <Link className="sidebar-link" to="/practice/notes">查看学习笔记</Link>
              </div>
            )}
          </article>

          <article className="section-card sidebar-card">
            <span className="section-kicker">学习陪伴入口</span>
            <div className="sidebar-links">
              <Link className="sidebar-link" to="/companion">学习计划</Link>
              <Link className="sidebar-link" to="/companion">Live2D 展示</Link>
              <Link className="sidebar-link" to="/practice/wrong">错题复盘</Link>
              <Link className="sidebar-link" to="/practice/notes">学习笔记</Link>
            </div>
          </article>

          <article className="section-card sidebar-card">
            <span className="section-kicker">首页定位</span>
            <p>这一版首页已经不只是文案占位，而是开始承接真实动态、题库推荐和个人推进状态。下一步再补独立社区页与发布流。</p>
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
      navigate({
        to: '/auth/login',
      })
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
          <Link className="secondary-link" to="/practice/notes">直接看笔记</Link>
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
        <Link className="secondary-link" to="/practice/wrong">进入错题本</Link>
        <Link className="secondary-link" to="/practice/favorites">查看收藏夹</Link>
        <Link className="secondary-link" to="/practice/notes">查看笔记</Link>
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
      navigate({
        to: '/auth/login',
      })
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
    } catch (error) {
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
      navigate({
        to: '/auth/login',
      })
      return
    }

    try {
      const nextState = await toggleFavoriteRequest(accessToken, question.id)
      setFavoriteState(nextState)
      setFavoriteMessage(nextState ? '已加入收藏夹' : '已移出收藏夹')
      await queryClient.invalidateQueries({ queryKey: ['practice-favorites'] })
    } catch (error) {
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
   * 提交代码题答案，并返回后端当前的分析结果。
   */
  async function handleEvaluate(label: '运行代码' | '提交代码') {
    if (!question) {
      return
    }

    if (!accessToken) {
      navigate({
        to: '/auth/login',
      })
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
    } catch (error) {
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
      navigate({
        to: '/auth/login',
      })
      return
    }

    try {
      const nextState = await toggleFavoriteRequest(accessToken, question.id)
      setFavoriteState(nextState)
      setFavoriteMessage(nextState ? '已加入收藏夹' : '已移出收藏夹')
      await queryClient.invalidateQueries({ queryKey: ['practice-favorites'] })
    } catch (error) {
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
    <section className="editor-layout">
      <div className="editor-sidebar">
        <span className="page-tag">代码练习</span>
        <h1>{question?.title || `题目 #${questionId}`}</h1>
        <p className="page-copy">{question?.content || '题目详情加载中...'}</p>
        <p className="companion-empty-text">当前行业：{questionIndustryLabel}</p>
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
            </>
          ) : null}
        </div>
      </div>
    </section>
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
      setMessage('请先登录')
      return
    }

    try {
      await deleteQuestionNote(accessToken, noteId)
      setMessage('笔记已删除')
      await queryClient.invalidateQueries({ queryKey: ['practice-notes'] })
    } catch (error) {
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
 * 提供学习陪伴频道首页，整合学习计划、陪伴角色和成长记录的入口设计。
 */
function CompanionPage() {
  return <CompanionHubPage />
}

/**
 * 展示登录后已经同步到前端的用户资料，验证工作台主链路是否打通。
 */
function WorkspacePage() {
  const user = useAuthStore((state) => state.user)
  const { effectiveIndustryLabel } = useFrontendIndustryPreference()

  return (
    <section className="page-panel">
      <span className="page-tag">已接管链路</span>
      <h1>用户工作台</h1>
      <p className="page-copy">
        当前页面用于验证 React 前台已经具备登录、会话恢复、资料同步和统一行业偏好能力，后续会接入统计卡片、学习计划和最近练习记录。
      </p>
      <div className="grid-cards">
        <article className="feature-card">
          <h2>用户名</h2>
          <p>{user?.username || '-'}</p>
        </article>
        <article className="feature-card">
          <h2>邮箱</h2>
          <p>{user?.email || '-'}</p>
        </article>
        <article className="feature-card">
          <h2>角色</h2>
          <p>{user?.role || '-'}</p>
        </article>
        <article className="feature-card">
          <h2>当前方向</h2>
          <p>{effectiveIndustryLabel}</p>
        </article>
      </div>
    </section>
  )
}

/**
 * 提供前台登录页面，并在成功后跳转到用户工作台。
 */
function LoginPage() {
  const navigate = useNavigate()
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
   * 提交登录表单，并在成功后进入已受保护的工作台页面。
   */
  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const result = await login(form.email.trim(), form.password)
    setMessage(result.message)

    if (result.ok) {
      navigate({
        to: '/workspace',
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
  beforeLoad: async () => {
    const authStore = useAuthStore.getState()
    authStore.initAuth()

    if (!authStore.accessToken) {
      throw redirect({
        to: '/auth/login',
      })
    }
  },
  component: PracticeEditorPage,
})

const practiceWrongRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: 'practice/wrong',
  beforeLoad: async () => {
    const authStore = useAuthStore.getState()
    authStore.initAuth()

    if (!authStore.accessToken) {
      throw redirect({
        to: '/auth/login',
      })
    }
  },
  component: PracticeWrongPage,
})

const practiceFavoritesRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: 'practice/favorites',
  beforeLoad: async () => {
    const authStore = useAuthStore.getState()
    authStore.initAuth()

    if (!authStore.accessToken) {
      throw redirect({
        to: '/auth/login',
      })
    }
  },
  component: PracticeFavoritesPage,
})

const practiceNotesRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: 'practice/notes',
  beforeLoad: async () => {
    const authStore = useAuthStore.getState()
    authStore.initAuth()

    if (!authStore.accessToken) {
      throw redirect({
        to: '/auth/login',
      })
    }
  },
  component: PracticeNotesPage,
})

const interviewRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: 'interview',
  component: InterviewHubPage,
})

const interviewSessionRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: 'interview/$interviewId',
  beforeLoad: async () => {
    const authStore = useAuthStore.getState()
    authStore.initAuth()

    if (!authStore.accessToken) {
      throw redirect({
        to: '/auth/login',
      })
    }
  },
  component: InterviewSessionPage,
})

const interviewReportRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: 'interview/$interviewId/report',
  beforeLoad: async () => {
    const authStore = useAuthStore.getState()
    authStore.initAuth()

    if (!authStore.accessToken) {
      throw redirect({
        to: '/auth/login',
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

const loginRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: 'auth/login',
  component: LoginPage,
})

const workspaceRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: 'workspace',
  beforeLoad: async () => {
    const authStore = useAuthStore.getState()
    authStore.initAuth()

    if (!authStore.accessToken) {
      throw redirect({
        to: '/auth/login',
      })
    }

    const ready = await useAuthStore.getState().ensureProfile()
    if (!ready) {
      throw redirect({
        to: '/auth/login',
      })
    }
  },
  component: WorkspacePage,
})

const routeTree = rootRoute.addChildren([
  indexRoute,
  practiceRoute,
  practiceQuestionRoute,
  practiceEditorRoute,
  practiceWrongRoute,
  practiceFavoritesRoute,
  practiceNotesRoute,
  interviewRoute,
  interviewSessionRoute,
  interviewReportRoute,
  companionRoute,
  companionRoomRoute,
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
