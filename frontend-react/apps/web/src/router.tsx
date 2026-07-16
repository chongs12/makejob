import type { CSSProperties, ComponentType, FormEvent } from 'react'
import { Suspense, lazy, useEffect, useState } from 'react'
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
import { Button, Input, Avatar, Dropdown, Spin } from 'antd'
import { SearchOutlined, EditOutlined, UserOutlined, DownOutlined, LogoutOutlined, ProfileOutlined } from '@ant-design/icons'
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

const ROUTER_THEME = {
  bg: '#fafafa',
  cardBg: '#ffffff',
  primary: '#f97316',
  primaryLight: '#fff7ed',
  textMain: '#1c1917',
  textSecondary: '#57534e',
  textMuted: '#a8a29e',
  border: '#e7e5e4',
  radius: 12,
  shadow: '0 1px 3px rgba(0,0,0,0.04), 0 1px 2px rgba(0,0,0,0.02)',
}

/**
 * 为路由级异步页面提供统一加载占位，避免切页时出现明显空白。
 */
function RouteLoadingFallback() {
  return (
    <div style={{ minHeight: '60vh', display: 'flex', alignItems: 'center', justifyContent: 'center', background: ROUTER_THEME.bg }}>
      <div style={{
        background: ROUTER_THEME.cardBg,
        borderRadius: ROUTER_THEME.radius,
        border: `1px solid ${ROUTER_THEME.border}`,
        padding: '32px 40px',
        boxShadow: ROUTER_THEME.shadow,
        textAlign: 'center',
      }}>
        <Spin />
        <p style={{ margin: '12px 0 0', fontSize: 14, color: ROUTER_THEME.textSecondary }}>页面加载中...</p>
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
const InterviewHistoryPageRoute = createLazyRouteComponent(async () => ({ default: (await import('./features/interview/InterviewHistoryPage')).InterviewHistoryPage }))
const PrototypeUIPageRoute = createLazyRouteComponent(async () => ({ default: (await import('./features/prototype/PrototypeUIPage')).PrototypeUIPage }))
const InterviewSessionPageRoute = createLazyRouteComponent(async () => ({ default: (await import('./features/interview/InterviewSessionPage')).InterviewSessionPage }))
const InterviewReportPageRoute = createLazyRouteComponent(async () => ({ default: (await import('./features/interview/InterviewReportPage')).InterviewReportPage }))
const CompanionHubPageRoute = createLazyRouteComponent(async () => ({ default: (await import('./features/companion/CompanionHubPage')).CompanionHubPage }))
const GrowthPageRoute = createLazyRouteComponent(() => import('./features/growth/GrowthPage'))
const MembershipPageRoute = createLazyRouteComponent(async () => ({ default: (await import('./features/membership/MembershipPage')).MembershipPage }))

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
    <div
      role="presentation"
      style={{
        position: 'fixed',
        inset: 0,
        zIndex: 1000,
        background: 'rgba(0,0,0,0.45)',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        backdropFilter: 'blur(4px)',
      }}
      onClick={props.onCancel}
    >
      <div
        role="dialog"
        aria-modal="true"
        aria-labelledby="login-required-title"
        style={{
          background: ROUTER_THEME.cardBg,
          borderRadius: ROUTER_THEME.radius,
          padding: '32px 36px',
          maxWidth: 400,
          width: '90%',
          boxShadow: '0 20px 60px rgba(0,0,0,0.15)',
        }}
        onClick={(e) => e.stopPropagation()}
      >
        <div style={{
          display: 'inline-block',
          padding: '3px 10px',
          borderRadius: 6,
          background: ROUTER_THEME.primaryLight,
          color: ROUTER_THEME.primary,
          fontSize: 12,
          fontWeight: 600,
          marginBottom: 16,
        }}>
          登录提示
        </div>
        <h2 id="login-required-title" style={{ margin: '0 0 12px', fontSize: 20, fontWeight: 700, color: ROUTER_THEME.textMain }}>
          {title}
        </h2>
        <p style={{ margin: '0 0 24px', fontSize: 14, color: ROUTER_THEME.textSecondary, lineHeight: 1.6 }}>
          {description}
        </p>
        <div style={{ display: 'flex', gap: 12 }}>
          <Button type="primary" onClick={props.onConfirm} style={{ flex: 1, borderRadius: 8, background: ROUTER_THEME.primary, borderColor: ROUTER_THEME.primary, fontWeight: 600 }}>
            立即登录
          </Button>
          <Button onClick={props.onCancel} style={{ flex: 1, borderRadius: 8, fontWeight: 600 }}>
            稍后登录
          </Button>
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
  const isPaidMember = Boolean(user?.membershipLevel) && user.membershipLevel !== 'free'
  const isStandaloneCompanionRoom = pathname.startsWith('/companion/room')
  const isStandaloneEditor = pathname.startsWith('/practice/editor')
  const isPrototypeUI = pathname.startsWith('/prototype-ui')
  const loginPromptDialog = (
    <LoginRequiredDialog
      open={loginPromptState.open}
      reason={loginPromptState.reason}
      onConfirm={handleConfirmLogin}
      onCancel={closeLoginPrompt}
    />
  )

  if (isStandaloneCompanionRoom || isStandaloneEditor || isPrototypeUI) {
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

      <header
        style={{
          position: 'sticky',
          top: 0,
          zIndex: 50,
          background: 'rgba(255,255,255,0.92)',
          backdropFilter: 'blur(16px) saturate(180%)',
          WebkitBackdropFilter: 'blur(16px) saturate(180%)',
          borderBottom: '1px solid rgba(0,0,0,0.06)',
        }}
      >
        <div
          style={{
            maxWidth: 1280,
            margin: '0 auto',
            padding: '0 24px',
            height: 56,
            display: 'flex',
            alignItems: 'center',
            gap: 32,
          }}
        >
          {/* Logo */}
          <Link to="/" style={{ textDecoration: 'none', display: 'flex', alignItems: 'center', gap: 8, flexShrink: 0 }}>
            <div
              style={{
                width: 32,
                height: 32,
                borderRadius: 8,
                background: 'linear-gradient(135deg, #f97316, #fb923c)',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                fontSize: 14,
                fontWeight: 800,
                color: '#fff',
              }}
            >
              M
            </div>
            <span style={{ fontSize: 20, fontWeight: 800, color: '#1c1917', letterSpacing: -0.5 }}>MakeJob</span>
          </Link>

          {/* Nav */}
          <nav style={{ display: 'flex', alignItems: 'center', gap: 4, flexShrink: 0 }}>
            {navigationItems.map((item) => (
              <Link
                key={item.to}
                to={item.to}
                style={{
                  padding: '6px 14px',
                  borderRadius: 8,
                  fontSize: 14,
                  fontWeight: item.match ? 600 : 500,
                  color: item.match ? '#1c1917' : '#78716c',
                  background: item.match ? '#f5f5f4' : 'transparent',
                  textDecoration: 'none',
                  transition: 'all 0.2s ease',
                }}
                onMouseEnter={(e) => {
                  if (!item.match) {
                    e.currentTarget.style.background = '#fafaf9'
                    e.currentTarget.style.color = '#1c1917'
                  }
                }}
                onMouseLeave={(e) => {
                  if (!item.match) {
                    e.currentTarget.style.background = 'transparent'
                    e.currentTarget.style.color = '#78716c'
                  }
                }}
              >
                {item.label}
              </Link>
            ))}
          </nav>

          {/* Search */}
          <form onSubmit={handleHeaderSearch} style={{ flex: 1, maxWidth: 360, display: 'flex', alignItems: 'center' }}>
            <Input
              prefix={<SearchOutlined style={{ color: '#a8a29e' }} />}
              placeholder="搜索题目、面经..."
              value={headerKeyword}
              onChange={(e) => setHeaderKeyword(e.target.value)}
              style={{ borderRadius: 10, background: '#f5f5f4', border: '1px solid transparent' }}
              onFocus={(e) => { e.currentTarget.style.borderColor = '#e7e5e4' }}
              onBlur={(e) => { e.currentTarget.style.borderColor = 'transparent' }}
            />
          </form>

          {/* Actions */}
          <div style={{ display: 'flex', alignItems: 'center', gap: 12, flexShrink: 0 }}>
            {accessToken ? (
              <>
                {!isPaidMember && (
                  <Link to="/membership" style={{ textDecoration: 'none' }}>
                    <Button
                      size="small"
                      style={{
                        borderRadius: 8,
                        border: '1px solid #f97316',
                        color: '#f97316',
                        fontWeight: 600,
                      }}
                    >
                      升级会员
                    </Button>
                  </Link>
                )}
                <Button
                  type="primary"
                  size="small"
                  icon={<EditOutlined />}
                  onClick={handlePublish}
                  style={{
                    borderRadius: 8,
                    background: '#f97316',
                    borderColor: '#f97316',
                    fontWeight: 600,
                  }}
                >
                  发布
                </Button>
                <Dropdown
                  menu={{
                    items: [
                      {
                        key: 'membership',
                        icon: <ProfileOutlined />,
                        label: '会员中心',
                        onClick: () => navigate({ to: '/membership' }),
                      },
                      {
                        key: 'profile',
                        icon: <ProfileOutlined />,
                        label: '成长档案',
                        onClick: () => navigate({ to: '/growth' }),
                      },
                      {
                        key: 'logout',
                        icon: <LogoutOutlined />,
                        label: '退出登录',
                        danger: true,
                        onClick: () => logout(),
                      },
                    ],
                  }}
                  trigger={['click']}
                >
                  <div style={{ display: 'flex', alignItems: 'center', gap: 8, cursor: 'pointer' }}>
                    <Avatar size={28} icon={<UserOutlined />} style={{ background: '#e7e5e4', color: '#78716c' }} />
                    <span style={{ fontSize: 13, fontWeight: 600, color: '#1c1917' }}>{user?.username || '用户'}</span>
                    <DownOutlined style={{ fontSize: 10, color: '#a8a29e' }} />
                  </div>
                </Dropdown>
              </>
            ) : (
              <>
                <Link to="/auth/login" style={{ textDecoration: 'none' }}>
                  <Button
                    type="text"
                    style={{ fontWeight: 600, color: '#78716c', borderRadius: 8 }}
                  >
                    登录
                  </Button>
                </Link>
                <Button
                  type="primary"
                  size="small"
                  onClick={handlePublish}
                  style={{
                    borderRadius: 8,
                    background: '#f97316',
                    borderColor: '#f97316',
                    fontWeight: 600,
                  }}
                >
                  发布
                </Button>
              </>
            )}
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
 * 提供前台登录页面，并在成功后跳转到指定地址或成长档案页。
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
  const [form, setForm] = useState({ email: '', password: '' })
  const [message, setMessage] = useState('')

  /**
   * 提交登录表单，成功后跳转到目标页面。
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

      navigate({ to: '/growth', replace: true })
    }
  }

  const fieldLabelStyle: CSSProperties = {
    display: 'block',
    fontSize: 13,
    fontWeight: 600,
    color: ROUTER_THEME.textMain,
    marginBottom: 6,
  }

  return (
    <div style={{ minHeight: 'calc(100vh - 56px)', display: 'flex', alignItems: 'center', justifyContent: 'center', background: ROUTER_THEME.bg, padding: '40px 16px' }}>
      <div style={{
        background: ROUTER_THEME.cardBg,
        borderRadius: ROUTER_THEME.radius,
        border: `1px solid ${ROUTER_THEME.border}`,
        padding: '40px 36px',
        maxWidth: 400,
        width: '100%',
        boxShadow: ROUTER_THEME.shadow,
      }}>
        <div style={{ textAlign: 'center', marginBottom: 32 }}>
          <div style={{
            width: 48,
            height: 48,
            borderRadius: 12,
            background: 'linear-gradient(135deg, #f97316, #fb923c)',
            display: 'inline-flex',
            alignItems: 'center',
            justifyContent: 'center',
            fontSize: 20,
            fontWeight: 800,
            color: '#fff',
            marginBottom: 16,
          }}>
            M
          </div>
          <h1 style={{ margin: 0, fontSize: 22, fontWeight: 700, color: ROUTER_THEME.textMain }}>登录 MakeJob</h1>
          <p style={{ margin: '8px 0 0', fontSize: 14, color: ROUTER_THEME.textSecondary }}>
            登录后解锁完整功能，继续你的学习之旅
          </p>
        </div>

        <form onSubmit={handleSubmit}>
          <div style={{ marginBottom: 20 }}>
            <label style={fieldLabelStyle}>邮箱</label>
            <Input
              size="large"
              value={form.email}
              onChange={(e) => setForm((current) => ({ ...current, email: e.target.value }))}
              placeholder="请输入邮箱"
              style={{ borderRadius: 8 }}
            />
          </div>
          <div style={{ marginBottom: 24 }}>
            <label style={fieldLabelStyle}>密码</label>
            <Input.Password
              size="large"
              value={form.password}
              onChange={(e) => setForm((current) => ({ ...current, password: e.target.value }))}
              placeholder="请输入密码"
              style={{ borderRadius: 8 }}
            />
          </div>
          <Button
            type="primary"
            htmlType="submit"
            size="large"
            block
            loading={loading}
            style={{ borderRadius: 8, background: ROUTER_THEME.primary, borderColor: ROUTER_THEME.primary, fontWeight: 600, height: 44 }}
          >
            {loading ? '登录中...' : '登录'}
          </Button>
        </form>

        {message && (
          <div style={{
            marginTop: 20,
            padding: '10px 14px',
            borderRadius: 8,
            background: ROUTER_THEME.primaryLight,
            fontSize: 13,
            color: ROUTER_THEME.textSecondary,
            textAlign: 'center',
          }}>
            {message}
          </div>
        )}

        <div style={{
          marginTop: 24,
          padding: '12px 16px',
          borderRadius: 8,
          background: ROUTER_THEME.bg,
          fontSize: 12,
          color: ROUTER_THEME.textMuted,
          lineHeight: 1.8,
        }}>
          <div>令牌状态：{accessToken ? `${accessToken.slice(0, 12)}...` : '未写入'}</div>
          <div>用户资料：{user?.username || '未同步'}</div>
        </div>

        <div style={{ marginTop: 20, textAlign: 'center', fontSize: 13, color: ROUTER_THEME.textSecondary }}>
          还没账号？{' '}
          <Link to="/auth/register" style={{ color: ROUTER_THEME.primary, fontWeight: 600, textDecoration: 'none' }}>
            去注册
          </Link>
        </div>
      </div>
    </div>
  )
}

/**
 * 提供前台注册页面，注册成功后自动登录并跳转到目标地址或成长档案页。
 */
function RegisterPage() {
  const navigate = useNavigate()
  const redirectTarget = useRouterState({
    select: (state) => resolveLoginRedirectTarget((state.location.search as Record<string, unknown> | undefined)?.redirect),
  })
  const register = useAuthStore((state) => state.register)
  const loading = useAuthStore((state) => state.loading)
  const [form, setForm] = useState({ username: '', email: '', password: '', confirm: '' })
  const [message, setMessage] = useState('')

  /**
   * 提交注册表单，前端校验通过后调用注册并自动登录。
   */
  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const username = form.username.trim()
    const email = form.email.trim()
    const password = form.password

    if (!username) {
      setMessage('请输入用户名')
      return
    }
    if (!email.includes('@')) {
      setMessage('请输入合法的邮箱')
      return
    }
    if (password.length < 6) {
      setMessage('密码至少 6 位')
      return
    }
    if (password !== form.confirm) {
      setMessage('两次输入的密码不一致')
      return
    }

    const result = await register(username, email, password)
    setMessage(result.message)

    if (result.ok) {
      if (typeof window !== 'undefined') {
        window.location.replace(redirectTarget)
        return
      }
      navigate({ to: '/growth', replace: true })
    }
  }

  const fieldLabelStyle: CSSProperties = {
    display: 'block',
    fontSize: 13,
    fontWeight: 600,
    color: ROUTER_THEME.textMain,
    marginBottom: 6,
  }

  return (
    <div style={{ minHeight: 'calc(100vh - 56px)', display: 'flex', alignItems: 'center', justifyContent: 'center', background: ROUTER_THEME.bg, padding: '40px 16px' }}>
      <div style={{
        background: ROUTER_THEME.cardBg,
        borderRadius: ROUTER_THEME.radius,
        border: `1px solid ${ROUTER_THEME.border}`,
        padding: '40px 36px',
        maxWidth: 400,
        width: '100%',
        boxShadow: ROUTER_THEME.shadow,
      }}>
        <div style={{ textAlign: 'center', marginBottom: 32 }}>
          <div style={{
            width: 48,
            height: 48,
            borderRadius: 12,
            background: 'linear-gradient(135deg, #f97316, #fb923c)',
            display: 'inline-flex',
            alignItems: 'center',
            justifyContent: 'center',
            fontSize: 20,
            fontWeight: 800,
            color: '#fff',
            marginBottom: 16,
          }}>
            M
          </div>
          <h1 style={{ margin: 0, fontSize: 22, fontWeight: 700, color: ROUTER_THEME.textMain }}>注册 MakeJob</h1>
          <p style={{ margin: '8px 0 0', fontSize: 14, color: ROUTER_THEME.textSecondary }}>
            注册即登录，立刻开始你的学习之旅
          </p>
        </div>

        <form onSubmit={handleSubmit}>
          <div style={{ marginBottom: 20 }}>
            <label style={fieldLabelStyle}>用户名</label>
            <Input
              size="large"
              value={form.username}
              onChange={(e) => setForm((current) => ({ ...current, username: e.target.value }))}
              placeholder="请输入用户名"
              style={{ borderRadius: 8 }}
            />
          </div>
          <div style={{ marginBottom: 20 }}>
            <label style={fieldLabelStyle}>邮箱</label>
            <Input
              size="large"
              value={form.email}
              onChange={(e) => setForm((current) => ({ ...current, email: e.target.value }))}
              placeholder="请输入邮箱（作为登录账号）"
              style={{ borderRadius: 8 }}
            />
          </div>
          <div style={{ marginBottom: 20 }}>
            <label style={fieldLabelStyle}>密码</label>
            <Input.Password
              size="large"
              value={form.password}
              onChange={(e) => setForm((current) => ({ ...current, password: e.target.value }))}
              placeholder="至少 6 位"
              style={{ borderRadius: 8 }}
            />
          </div>
          <div style={{ marginBottom: 24 }}>
            <label style={fieldLabelStyle}>确认密码</label>
            <Input.Password
              size="large"
              value={form.confirm}
              onChange={(e) => setForm((current) => ({ ...current, confirm: e.target.value }))}
              placeholder="再次输入密码"
              style={{ borderRadius: 8 }}
            />
          </div>
          <Button
            type="primary"
            htmlType="submit"
            size="large"
            block
            loading={loading}
            style={{ borderRadius: 8, background: ROUTER_THEME.primary, borderColor: ROUTER_THEME.primary, fontWeight: 600, height: 44 }}
          >
            {loading ? '注册中...' : '注册并登录'}
          </Button>
        </form>

        {message && (
          <div style={{
            marginTop: 20,
            padding: '10px 14px',
            borderRadius: 8,
            background: ROUTER_THEME.primaryLight,
            fontSize: 13,
            color: ROUTER_THEME.textSecondary,
            textAlign: 'center',
          }}>
            {message}
          </div>
        )}

        <div style={{ marginTop: 20, textAlign: 'center', fontSize: 13, color: ROUTER_THEME.textSecondary }}>
          已有账号？{' '}
          <Link to="/auth/login" style={{ color: ROUTER_THEME.primary, fontWeight: 600, textDecoration: 'none' }}>
            去登录
          </Link>
        </div>
      </div>
    </div>
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

const interviewHistoryRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: 'interview/history',
  beforeLoad: async ({ location }) => {
    if (!await getLatestAccessToken()) {
      throw redirect({
        to: '/auth/login',
        search: buildLoginRedirectSearch(buildCurrentLocationPath(location.pathname, location.searchStr || '')),
      })
    }
  },
  component: InterviewHistoryPageRoute,
})

const prototypeUIRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: 'prototype-ui',
  component: PrototypeUIPageRoute,
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
  component: PrototypeUIPageRoute,
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

const membershipRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: 'membership',
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
  component: MembershipPageRoute,
})

const loginRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: 'auth/login',
  validateSearch: (search: Record<string, unknown>) => ({
    redirect: typeof search.redirect === 'string' ? search.redirect : undefined,
  }),
  component: LoginPage,
})

const registerRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: 'auth/register',
  validateSearch: (search: Record<string, unknown>) => ({
    redirect: typeof search.redirect === 'string' ? search.redirect : undefined,
  }),
  component: RegisterPage,
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
  interviewHistoryRoute,
  interviewSessionRoute,
  interviewReportRoute,
  companionRoute,
  companionRoomRoute,
  growthRoute,
  membershipRoute,
  loginRoute,
  registerRoute,
  workspaceRoute,
  prototypeUIRoute,
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
