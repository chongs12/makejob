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
async function fetchCategories(): Promise<CategoryNode[]> {
  const response = await requestJson<ApiEnvelope<CategoryNode[]>>('/categories?industry_id=1')
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
  categoryId: number | null
}): Promise<ExamResponse> {
  const endpoint = params.mode === 'timed' ? '/exams/timed' : '/exams/random'
  const body = params.mode === 'timed'
    ? {
        count: 5,
        difficulty: params.difficulty || 'medium',
        category_id: params.categoryId || undefined,
        time_limit_minutes: 30,
      }
    : {
        count: 5,
        difficulty: params.difficulty || 'medium',
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
            <div className="brand-subtitle">刷题、面试、学习陪伴一体化入口</div>
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
              placeholder="搜索题目、面经、学习主题"
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
  return (
    <div className="home-shell">
      <section className="hero-panel">
        <div className="hero-content">
          <span className="page-tag">Offer 导向学习平台</span>
          <h1>把题库训练、AI 面试和学习陪伴放在同一条成长链路里</h1>
          <p className="page-copy">
            首页改成内容门户结构，不再像后台管理。后续这里可以继续接说说、论坛、面经和动态流，现在先把用户进入后的首屏氛围、业务入口和节奏做对。
          </p>

          <div className="hero-actions">
            <Link className="primary-button hero-link-button" to="/practice">
              进入题库
            </Link>
            <Link className="secondary-button hero-link-button" to="/interview">
              打开面试入口
            </Link>
          </div>

          <div className="hero-metrics">
            <article className="metric-card">
              <strong>真题训练</strong>
              <span>刷题、错题、收藏、笔记统一沉淀</span>
            </article>
            <article className="metric-card">
              <strong>AI 面试</strong>
              <span>后续承接流式问答、追问与评分</span>
            </article>
            <article className="metric-card">
              <strong>学习陪伴</strong>
              <span>学习计划、Live2D、提醒与反馈联动</span>
            </article>
          </div>
        </div>

        <aside className="hero-aside">
          <div className="section-card spotlight-card">
            <span className="section-kicker">今日主线</span>
            <h2>先把题库刷顺，再进 AI 面试</h2>
            <p>题库页已经是当前最完整的业务域，后续论坛与说说也会从首页继续向牛客式信息流靠拢。</p>
          </div>
          <div className="section-card mini-feed-card">
            <span className="section-kicker">近期规划</span>
            <ul className="mini-feed-list">
              <li>题库体验统一收口到 React 版前台</li>
              <li>接入首页内容流与发布入口</li>
              <li>面试页挂载 AI 流式交互</li>
            </ul>
          </div>
        </aside>
      </section>

      <section className="channel-grid">
        <article className="channel-card channel-card-practice">
          <span className="section-kicker">题库</span>
          <h2>把刷题、复盘、错题本串起来</h2>
          <p>这里承接题目列表、筛选、代码题、收藏、错题、笔记和模拟练习。</p>
          <Link className="secondary-link" to="/practice">前往题库</Link>
        </article>
        <article className="channel-card channel-card-interview">
          <span className="section-kicker">面试</span>
          <h2>AI 面试页面入口</h2>
          <p>后续承接岗位定制追问、语音链路、流式输出和结构化点评。</p>
          <Link className="secondary-link" to="/interview">查看入口</Link>
        </article>
        <article className="channel-card channel-card-companion">
          <span className="section-kicker">学习陪伴</span>
          <h2>学习计划与 Live2D</h2>
          <p>学习计划、提醒机制、陪伴角色和学习反馈都会聚合在这里。</p>
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
                <h2>后续会接成类似牛客的内容广场</h2>
              </div>
              <button className="secondary-button" type="button">查看全部</button>
            </div>

            <div className="feed-stack">
              <article className="feed-item">
                <div className="feed-item-head">
                  <strong>题库训练建议</strong>
                  <span>刚刚更新</span>
                </div>
                <p>把刷题模式和错题复盘放在首页首屏之后，可以让新用户先看到“内容氛围”，再进入具体训练流程。</p>
              </article>
              <article className="feed-item">
                <div className="feed-item-head">
                  <strong>面试入口预告</strong>
                  <span>准备中</span>
                </div>
                <p>AI 面试页会挂在顶部一级导航，不藏在后台子菜单里，保证整站信息架构从一开始就是面向业务展示的。</p>
              </article>
              <article className="feed-item">
                <div className="feed-item-head">
                  <strong>学习陪伴板块</strong>
                  <span>规划中</span>
                </div>
                <p>学习计划、学习状态和 Live2D 陪伴组件会集中到一个入口，而不是散落在多个工具页里。</p>
              </article>
            </div>
          </article>

          <article className="section-card section-card-large">
            <div className="section-head">
              <div>
                <span className="section-kicker">快速开始</span>
                <h2>先完成这一条主流程</h2>
              </div>
            </div>
            <div className="timeline-list">
              <div className="timeline-item">
                <strong>1. 进入题库筛选方向</strong>
                <p>按难度、分类、关键词快速筛出当前阶段该刷的题。</p>
              </div>
              <div className="timeline-item">
                <strong>2. 用错题和笔记做复盘</strong>
                <p>做题后立刻沉淀错题与个人笔记，避免只刷不总结。</p>
              </div>
              <div className="timeline-item">
                <strong>3. 再进入 AI 面试</strong>
                <p>等题库链路稳定后，把刷题结果带到 AI 面试页进行追问与复述训练。</p>
              </div>
            </div>
          </article>
        </div>

        <aside className="home-side-column">
          <article className="section-card sidebar-card">
            <span className="section-kicker">热门方向</span>
            <div className="tag-cloud">
              <span>Go 后端</span>
              <span>算法高频</span>
              <span>系统设计</span>
              <span>前端性能</span>
              <span>数据库</span>
              <span>项目复盘</span>
            </div>
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
            <span className="section-kicker">发布预留</span>
            <p>顶部“发布”按钮先接到工作台，后续会扩成说说、论坛帖子、面经与学习动态发布。</p>
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
  const [keywordInput, setKeywordInput] = useState('')
  const [keyword, setKeyword] = useState('')
  const [difficulty, setDifficulty] = useState('')
  const [categoryId, setCategoryId] = useState<number | null>(null)
  const [page, setPage] = useState(1)
  const [examMessage, setExamMessage] = useState('等待组卷')

  const categoriesQuery = useQuery({
    queryKey: ['practice-categories'],
    queryFn: fetchCategories,
  })

  const questionsQuery = useQuery({
    queryKey: ['practice-questions', page, difficulty, keyword, categoryId],
    queryFn: () => fetchQuestions({
      page,
      pageSize: PRACTICE_PAGE_SIZE,
      difficulty,
      keyword,
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
        这一版已经接入真实题目列表、练习统计、错题本、收藏夹、笔记和代码题编辑器。
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

  const question = detailQuery.data
  const options = useMemo(() => parseQuestionOptions(question?.options_json), [question?.options_json])

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

  const question = detailQuery.data

  useEffect(() => {
    setSubmitResult(null)
    setSubmitMessage('等待运行')
    setFavoriteMessage('未操作')
  }, [questionId])

  useEffect(() => {
    setFavoriteState(Boolean(question?.is_favorited))
  }, [question?.is_favorited])

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
 * 提供 AI 面试频道首页，集中承接岗位模拟、面试流程与报告入口的设计。
 */
function InterviewPage() {
  return (
    <section className="page-panel">
      <span className="page-tag">面试频道</span>
      <h1>AI 面试入口</h1>
      <p className="page-copy">
        这一层不再只是一个占位页，而是后续 AI 面试主链路的入口首页。这里会承接岗位选择、面试进行页、面试报告和面经沉淀。
      </p>

      <div className="channel-overview-grid">
        <article className="channel-entry-card">
          <span className="section-kicker">岗位定制</span>
          <h2>按岗位生成题纲</h2>
          <p>后续会支持后端、前端、测试、算法等岗位入口，并按目标岗位生成更贴近真实场景的面试路径。</p>
          <Link className="secondary-link" to="/workspace">查看工作台</Link>
        </article>
        <article className="channel-entry-card">
          <span className="section-kicker">面试过程</span>
          <h2>流式追问与多轮对话</h2>
          <p>文本会走流式输出，后续再叠加语音输入、语音播报和动作反馈，形成更接近真实面试的节奏。</p>
          <Link className="secondary-link" to="/practice">先补题库基础</Link>
        </article>
        <article className="channel-entry-card">
          <span className="section-kicker">报告沉淀</span>
          <h2>报告、面经、复盘</h2>
          <p>每次模拟结束后都会沉淀成结构化报告，并能继续扩展为面经帖子与学习记录内容。</p>
          <button className="secondary-button" type="button">报告设计中</button>
        </article>
      </div>

      <div className="channel-split">
        <article className="section-card section-card-large">
          <div className="section-head">
            <div>
              <span className="section-kicker">面试流程</span>
              <h2>这一栏后续会承接完整面试闭环</h2>
            </div>
          </div>
          <div className="timeline-list">
            <div className="timeline-item">
              <strong>1. 选择岗位与方向</strong>
              <p>先确定岗位、技术栈和面试风格，再生成当前会话需要覆盖的能力维度。</p>
            </div>
            <div className="timeline-item">
              <strong>2. 进入 AI 模拟面试</strong>
              <p>支持连续追问、基于上一轮回答即时调整深度，不做机械的一问一答脚本页。</p>
            </div>
            <div className="timeline-item">
              <strong>3. 输出报告与面经</strong>
              <p>沉淀为可复盘的结构化结果，后续可继续发到首页内容流和个人成长记录里。</p>
            </div>
          </div>
        </article>

        <aside className="home-side-column">
          <article className="section-card sidebar-card">
            <span className="section-kicker">当前设计原则</span>
            <div className="sidebar-links">
              <div className="sidebar-link">入口显眼，不藏在工具页</div>
              <div className="sidebar-link">报告可沉淀，可继续发布</div>
              <div className="sidebar-link">与题库练习结果互相导流</div>
            </div>
          </article>
          <article className="section-card sidebar-card">
            <span className="section-kicker">后续挂载</span>
            <p>面试首页后面会继续拆成岗位入口页、面试进行页、结果页、面经列表页，不会再沿用后台式菜单组织。</p>
          </article>
        </aside>
      </div>
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

  return (
    <section className="page-panel">
      <span className="page-tag">已接管链路</span>
      <h1>用户工作台</h1>
      <p className="page-copy">
        当前页面用于验证 React 前台已经具备登录、会话恢复和资料同步能力，后续会接入统计卡片、学习计划和最近练习记录。
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
  component: InterviewPage,
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
