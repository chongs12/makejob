import type { FormEvent } from 'react'
import { useEffect, useMemo, useState } from 'react'
import {
  createRootRouteWithContext,
  createRoute,
  createRouter,
  Link,
  Outlet,
  redirect,
  useNavigate,
} from '@tanstack/react-router'
import type { QueryClient } from '@tanstack/react-query'
import { useAdminAuthStore } from './state/auth'

interface RouterContext {
  queryClient: QueryClient
}

/**
 * 在后台根布局初始化时恢复管理员会话，并补拉角色资料。
 */
function AdminAuthBootstrap() {
  const initialized = useAdminAuthStore((state) => state.initialized)
  const accessToken = useAdminAuthStore((state) => state.accessToken)
  const user = useAdminAuthStore((state) => state.user)
  const initAuth = useAdminAuthStore((state) => state.initAuth)
  const ensureAdmin = useAdminAuthStore((state) => state.ensureAdmin)

  useEffect(() => {
    initAuth()
  }, [initAuth])

  useEffect(() => {
    if (!initialized || !accessToken || user?.role === 'admin') {
      return
    }

    void ensureAdmin()
  }, [accessToken, ensureAdmin, initialized, user])

  return null
}

/**
 * 提供后台统一布局和管理员导航。
 */
function AdminLayout() {
  const user = useAdminAuthStore((state) => state.user)
  const accessToken = useAdminAuthStore((state) => state.accessToken)
  const logout = useAdminAuthStore((state) => state.logout)

  return (
    <div className="admin-shell">
      <AdminAuthBootstrap />

      <aside className="admin-sidebar">
        <h1>MakeJob Admin</h1>
        <p>后台管理 React 主干</p>
        <nav className="admin-nav">
          <Link className="admin-link" to="/dashboard">总览</Link>
          <Link className="admin-link" to="/live2d">Live2D 管理</Link>
          <Link className="admin-link" to="/questions">题库管理</Link>
          <Link className="admin-link" to="/auth/login">后台登录</Link>
        </nav>
        <p>当前管理员：{user?.username || '未同步'}</p>
        <p>权限角色：{user?.role || '未登录'}</p>
        {accessToken ? (
          <button className="admin-link" onClick={() => logout()}>
            退出登录
          </button>
        ) : null}
      </aside>

      <main className="admin-content">
        <Outlet />
      </main>
    </div>
  )
}

/**
 * 提供后台登录页，并在成功后进入后台总览。
 */
function LoginPage() {
  const navigate = useNavigate()
  const login = useAdminAuthStore((state) => state.login)
  const loading = useAdminAuthStore((state) => state.loading)
  const accessToken = useAdminAuthStore((state) => state.accessToken)
  const user = useAdminAuthStore((state) => state.user)
  const [form, setForm] = useState({
    email: '',
    password: '',
  })
  const [message, setMessage] = useState('等待登录')

  /**
   * 提交后台登录表单，并在管理员校验通过后跳转到后台总览。
   */
  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const result = await login(form.email.trim(), form.password)
    setMessage(result.message)

    if (result.ok) {
      navigate({
        to: '/dashboard',
      })
    }
  }

  /**
   * 对后台令牌做短截断展示，便于判断本地会话是否已经建立。
   */
  function maskToken(token: string | null): string {
    if (!token) {
      return '未写入'
    }

    return `${token.slice(0, 12)}...`
  }

  const tokenPreview = useMemo(() => maskToken(accessToken), [accessToken])

  return (
    <section className="admin-panel">
      <span className="admin-tag">后台登录</span>
      <h2>后台登录入口</h2>
      <p className="admin-copy">这个页面会在登录后自动校验 `/user/profile` 中的管理员角色。</p>
      <form onSubmit={handleSubmit}>
        <label className="admin-field">
          <span>邮箱</span>
          <input
            value={form.email}
            onChange={(event) => setForm((current) => ({ ...current, email: event.target.value }))}
            placeholder="请输入管理员邮箱"
          />
        </label>
        <label className="admin-field">
          <span>密码</span>
          <input
            type="password"
            value={form.password}
            onChange={(event) => setForm((current) => ({ ...current, password: event.target.value }))}
            placeholder="请输入密码"
          />
        </label>
        <p>
          <button className="admin-link" type="submit" disabled={loading}>
            {loading ? '提交中...' : '登录'}
          </button>
        </p>
      </form>
      <p>令牌状态：{tokenPreview}</p>
      <p>当前用户：{user?.username || '未同步'}</p>
      <p>{message}</p>
    </section>
  )
}

/**
 * 预留后台总览页，验证管理员链路已经打通。
 */
function DashboardPage() {
  const user = useAdminAuthStore((state) => state.user)

  return (
    <section className="admin-panel">
      <span className="admin-tag">优先迁移</span>
      <h2>后台总览</h2>
      <p className="admin-copy">这里后续接入仪表盘、系统状态、订单与用户统计。</p>
      <p>当前管理员：{user?.username || '-'}</p>
      <p>邮箱：{user?.email || '-'}</p>
    </section>
  )
}

/**
 * 预留 Live2D 资产管理页。
 */
function Live2DPage() {
  return (
    <section className="admin-panel">
      <span className="admin-tag">核心资产</span>
      <h2>Live2D 管理</h2>
      <p className="admin-copy">这里后续接入模型导入、场景绑定、缩放参数、缩略图和动作配置。</p>
    </section>
  )
}

/**
 * 预留题库管理页。
 */
function QuestionsPage() {
  return (
    <section className="admin-panel">
      <span className="admin-tag">题库中心</span>
      <h2>题库管理</h2>
      <p className="admin-copy">这里后续接入题目编辑、标签管理、难度策略和行业分类。</p>
    </section>
  )
}

const rootRoute = createRootRouteWithContext<RouterContext>()({
  component: AdminLayout,
})

const loginRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: 'auth/login',
  component: LoginPage,
})

const dashboardRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: 'dashboard',
  beforeLoad: async () => {
    const authStore = useAdminAuthStore.getState()
    authStore.initAuth()

    if (!authStore.accessToken) {
      throw redirect({
        to: '/auth/login',
      })
    }

    const ready = await useAdminAuthStore.getState().ensureAdmin()
    if (!ready) {
      throw redirect({
        to: '/auth/login',
      })
    }
  },
  component: DashboardPage,
})

const live2DRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: 'live2d',
  beforeLoad: async () => {
    const ready = await useAdminAuthStore.getState().ensureAdmin()
    if (!ready) {
      throw redirect({
        to: '/auth/login',
      })
    }
  },
  component: Live2DPage,
})

const questionsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: 'questions',
  beforeLoad: async () => {
    const ready = await useAdminAuthStore.getState().ensureAdmin()
    if (!ready) {
      throw redirect({
        to: '/auth/login',
      })
    }
  },
  component: QuestionsPage,
})

const routeTree = rootRoute.addChildren([
  loginRoute,
  dashboardRoute,
  live2DRoute,
  questionsRoute,
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
