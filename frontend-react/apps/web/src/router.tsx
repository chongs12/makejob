import type { ComponentType, FormEvent } from 'react'
import { Suspense, lazy, useEffect, useMemo, useState } from 'react'
import {
  createRootRouteWithContext,
  createRoute,
  createRouter,
  Link,
  Outlet,
  redirect,
  useNavigate,
  useRouterState,
} from '@tanstack/react-router'
import { type QueryClient } from '@tanstack/react-query'
import { AUTH_EXPIRED_EVENT_NAME } from '@makejob/api-client'
import { useAuthStore } from './state/auth'
import { useFrontendIndustryPreference } from './shared/frontendIndustryPreference'
import {
  buildCurrentLocationPath,
  buildLoginRedirectSearch,
  resolveLoginRedirectTarget,
} from './shared/authRedirect'
import { LOGIN_REQUIRED_PROMPT_EVENT_NAME, type LoginPromptDetail, type LoginPromptReason, requestLoginPrompt } from './shared/loginPrompt'
import { buildPracticeRouteSearch, validatePracticeRouteSearch } from './shared/practiceRoute'

interface RouterContext {
  queryClient: QueryClient
}

/**
 * 初始化前台登录态并返回最新的访问令牌，避免路由守卫读取到旧快照。
 * 如果 access token 过期但 refresh token 存在，会尝试同步刷新。
 */
async function getLatestAccessToken(): Promise<string | null> {
  const authStore = useAuthStore.getState()
  authStore.initAuth()
  let state = useAuthStore.getState()
  // 如果 access token 为空但 refresh token 存在，等待刷新完成
  if (!state.accessToken && state.refreshToken) {
    await state.refreshSession()
    state = useAuthStore.getState()
  }
  return state.accessToken
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
 * 为路由级异步页面提供统一加载占位，避免切页时出现明显空白。
 */
function RouteLoadingFallback() {
  return (
    <div className="page-shell">
      <div className="page-card">
        <p>页面加载中...</p>
      </div>
    </div>
  )
}

/**
 * 把页面模块包装成路由可直接使用的懒加载组件，统一复用 Suspense 过渡态。
 */
function createLazyRouteComponent(loader: () => Promise<{ default: ComponentType }>) {
  const LazyComponent = lazy(loader)

  /**
   * 在路由命中后按需加载目标页面，并在资源返回前渲染统一占位。
   */
  function LazyRouteComponent() {
    return (
      <Suspense fallback={<RouteLoadingFallback />}>
        <LazyComponent />
      </Suspense>
    )
  }

  return LazyRouteComponent
}

const HomePageRoute = createLazyRouteComponent(() => import('./features/home/HomePage'))
const CommunityPageRoute = createLazyRouteComponent(async () => ({ default: (await import('./features/community/CommunityPages')).CommunityPage }))
const CommunityCreatePostPageRoute = createLazyRouteComponent(async () => ({ default: (await import('./features/community/CommunityPages')).CommunityCreatePostPage }))
const CommunityMyPostsPageRoute = createLazyRouteComponent(async () => ({ default: (await import('./features/community/CommunityPages')).CommunityMyPostsPage }))
const CommunityEditPostPageRoute = createLazyRouteComponent(async () => ({ default: (await import('./features/community/CommunityPages')).CommunityEditPostPage }))
const CommunityPostDetailPageRoute = createLazyRouteComponent(async () => ({ default: (await import('./features/community/CommunityPages')).CommunityPostDetailPage }))
const PracticePageRoute = createLazyRouteComponent(async () => ({ default: (await import('./features/practice/PracticePage')).PracticePage }))
const PracticeQuestionPageRoute = createLazyRouteComponent(async () => ({ default: (await import('./features/practice/PracticeDetailPages')).PracticeQuestionPage }))
const PracticeEditorPageRoute = createLazyRouteComponent(async () => ({ default: (await import('./features/practice/PracticeDetailPages')).PracticeEditorPage }))
const PracticeWrongPageRoute = createLazyRouteComponent(async () => ({ default: (await import('./features/practice/PracticeDetailPages')).PracticeWrongPage }))
const PracticeFavoritesPageRoute = createLazyRouteComponent(async () => ({ default: (await import('./features/practice/PracticeDetailPages')).PracticeFavoritesPage }))
const PracticeNotesPageRoute = createLazyRouteComponent(async () => ({ default: (await import('./features/practice/PracticeDetailPages')).PracticeNotesPage }))
const MistakeTopicPageRoute = createLazyRouteComponent(async () => ({ default: (await import('./features/practice/PracticeDetailPages')).MistakeTopicPage }))
const InterviewHubPageRoute = createLazyRouteComponent(async () => ({ default: (await import('./features/interview/InterviewPage')).InterviewHubPage }))
const InterviewSessionPageRoute = createLazyRouteComponent(async () => ({ default: (await import('./features/interview/InterviewSessionPage')).InterviewSessionPage }))
const InterviewReportPageRoute = createLazyRouteComponent(async () => ({ default: (await import('./features/interview/InterviewReportPage')).InterviewReportPage }))
const CompanionHubPageRoute = createLazyRouteComponent(async () => ({ default: (await import('./features/companion/CompanionHubPage')).CompanionHubPage }))
const CompanionWorkspacePageRoute = createLazyRouteComponent(async () => ({ default: (await import('./features/companion/CompanionWorkspacePage')).CompanionWorkspacePage }))
const GrowthPageRoute = createLazyRouteComponent(() => import('./features/growth/GrowthPage'))

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
    navigate({
      to: '/practice',
      search: buildPracticeRouteSearch({
        keyword: headerKeyword,
        source: 'header_search',
        title: '顶部搜索',
      }),
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
  const isStandaloneEditor = pathname.startsWith('/practice/editor')
  const loginPromptDialog = (
    <LoginRequiredDialog
      open={loginPromptState.open}
      reason={loginPromptState.reason}
      onConfirm={handleConfirmLogin}
      onCancel={closeLoginPrompt}
    />
  )

  if (isStandaloneCompanionRoom || isStandaloneEditor) {
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
  component: HomePageRoute,
})

const communityRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: 'community',
  component: CommunityPageRoute,
})

const communityCreateRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: 'community/create',
  beforeLoad: async ({ location }) => {
    if (!await getLatestAccessToken()) {
      throw redirect({
        to: '/auth/login',
        search: buildLoginRedirectSearch(buildCurrentLocationPath(location.pathname, location.searchStr || '')),
      })
    }
  },
  component: CommunityCreatePostPageRoute,
})

const communityMineRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: 'community/mine',
  beforeLoad: async ({ location }) => {
    if (!await getLatestAccessToken()) {
      throw redirect({
        to: '/auth/login',
        search: buildLoginRedirectSearch(buildCurrentLocationPath(location.pathname, location.searchStr || '')),
      })
    }
  },
  component: CommunityMyPostsPageRoute,
})

const communityEditRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: 'community/$postId/edit',
  beforeLoad: async ({ location }) => {
    if (!await getLatestAccessToken()) {
      throw redirect({
        to: '/auth/login',
        search: buildLoginRedirectSearch(buildCurrentLocationPath(location.pathname, location.searchStr || '')),
      })
    }
  },
  component: CommunityEditPostPageRoute,
})

const communityDetailRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: 'community/$postId',
  component: CommunityPostDetailPageRoute,
})

const practiceRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: 'practice',
  validateSearch: validatePracticeRouteSearch,
  component: PracticePageRoute,
})

const practiceQuestionRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: 'practice/$questionId',
  component: PracticeQuestionPageRoute,
})

const practiceEditorRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: 'practice/editor/$questionId',
  beforeLoad: async ({ location }) => {
    if (!await getLatestAccessToken()) {
      throw redirect({
        to: '/auth/login',
        search: buildLoginRedirectSearch(buildCurrentLocationPath(location.pathname, location.searchStr || '')),
      })
    }
  },
  component: PracticeEditorPageRoute,
})

const practiceWrongRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: 'practice/wrong',
  beforeLoad: async ({ location }) => {
    if (!await getLatestAccessToken()) {
      throw redirect({
        to: '/auth/login',
        search: buildLoginRedirectSearch(buildCurrentLocationPath(location.pathname, location.searchStr || '')),
      })
    }
  },
  component: PracticeWrongPageRoute,
})

const practiceFavoritesRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: 'practice/favorites',
  beforeLoad: async ({ location }) => {
    if (!await getLatestAccessToken()) {
      throw redirect({
        to: '/auth/login',
        search: buildLoginRedirectSearch(buildCurrentLocationPath(location.pathname, location.searchStr || '')),
      })
    }
  },
  component: PracticeFavoritesPageRoute,
})

const practiceNotesRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: 'practice/notes',
  beforeLoad: async ({ location }) => {
    if (!await getLatestAccessToken()) {
      throw redirect({
        to: '/auth/login',
        search: buildLoginRedirectSearch(buildCurrentLocationPath(location.pathname, location.searchStr || '')),
      })
    }
  },
  component: PracticeNotesPageRoute,
})

const practiceTopicRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: 'practice/topics/$topicCode',
  component: MistakeTopicPageRoute,
})

const interviewRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: 'interview',
  component: InterviewHubPageRoute,
})

const interviewSessionRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: 'interview/$interviewId',
  beforeLoad: async ({ location }) => {
    if (!await getLatestAccessToken()) {
      throw redirect({
        to: '/auth/login',
        search: buildLoginRedirectSearch(buildCurrentLocationPath(location.pathname, location.searchStr || '')),
      })
    }
  },
  component: InterviewSessionPageRoute,
})

const interviewReportRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: 'interview/$interviewId/report',
  beforeLoad: async ({ location }) => {
    if (!await getLatestAccessToken()) {
      throw redirect({
        to: '/auth/login',
        search: buildLoginRedirectSearch(buildCurrentLocationPath(location.pathname, location.searchStr || '')),
      })
    }
  },
  component: InterviewReportPageRoute,
})

const companionRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: 'companion',
  component: CompanionHubPageRoute,
})

const companionRoomRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: 'companion/room',
  beforeLoad: async ({ location }) => {
    if (!await getLatestAccessToken()) {
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
  component: CompanionWorkspacePageRoute,
})

const growthRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: 'growth',
  beforeLoad: async ({ location }) => {
    if (!await getLatestAccessToken()) {
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
  component: GrowthPageRoute,
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
    if (!await getLatestAccessToken()) {
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
  component: GrowthPageRoute,
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
  defaultPreload: 'intent',
  defaultPreloadDelay: 80,
  context: {
    queryClient: undefined as never,
  },
})

declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router
  }
}
