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
  useLocation,
} from '@tanstack/react-router'
import type { QueryClient } from '@tanstack/react-query'
import {
  Button,
  Input,
  Spin,
  Tag,
} from 'antd'
import {
  DashboardOutlined,
  RocketOutlined,
  ThunderboltOutlined,
  DatabaseOutlined,
  BookOutlined,
  FileTextOutlined,
  SmileOutlined,
  SoundOutlined,
  AppstoreOutlined,
  QuestionCircleOutlined,
  OrderedListOutlined,
  LoginOutlined,
  LogoutOutlined,
  SafetyOutlined,
  UserOutlined,
  LockOutlined,
  SafetyCertificateOutlined,
  KeyOutlined,
} from '@ant-design/icons'
import { useAdminAuthStore } from './state/auth'
import { AIConfigPage } from './features/ai-config/AIConfigPage'
import { Live2DPage } from './features/live2d/Live2DPage'
import { PromptPage } from './features/prompt/PromptPage'
import { RAGConfigPage } from './features/rag-config/RAGConfigPage'
import { RAGKnowledgePage } from './features/rag-knowledge/RAGKnowledgePage'
import { QuestionPipelinePage } from './features/question-pipeline/QuestionPipelinePage'
import { QuestionPage } from './features/question/QuestionPage'
import { QuestionSetPage } from './features/question-set/QuestionSetPage'
import { RuntimeOverviewPage } from './features/runtime/RuntimeOverviewPage'
import { RuntimeTasksPage } from './features/runtime/RuntimeTasksPage'
import { TaxonomyPage } from './features/taxonomy/TaxonomyPage'
import { TTSPage } from './features/tts/TTSPage'

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
  const location = useLocation()
  const pathname = location.pathname

  const navItems = [
    { to: '/dashboard', label: '运行总览', icon: <DashboardOutlined /> },
    { to: '/runtime', label: '运行任务', icon: <RocketOutlined /> },
    { to: '/ai-configs', label: 'AI 配置', icon: <ThunderboltOutlined /> },
    { to: '/rag-configs', label: 'RAG 配置', icon: <DatabaseOutlined /> },
    { to: '/rag-knowledge', label: '知识库管理', icon: <BookOutlined /> },
    { to: '/prompts', label: 'Prompt 管理', icon: <FileTextOutlined /> },
    { to: '/live2d', label: 'Live2D 管理', icon: <SmileOutlined /> },
    { to: '/tts', label: 'TTS 配置', icon: <SoundOutlined /> },
    { to: '/taxonomy', label: '行业/分类', icon: <AppstoreOutlined /> },
    { to: '/question-pipeline', label: '题目流水线', icon: <ThunderboltOutlined /> },
    { to: '/questions', label: '题库管理', icon: <QuestionCircleOutlined /> },
    { to: '/question-sets', label: '题单管理', icon: <OrderedListOutlined /> },
  ]

  const sidebarWidth = 248

  return (
    <div style={{ minHeight: '100vh', display: 'flex' }}>
      <AdminAuthBootstrap />

      <aside
        style={{
          width: sidebarWidth,
          minHeight: '100vh',
          background: '#0f172a',
          color: '#e2e8f0',
          display: 'flex',
          flexDirection: 'column',
          padding: '24px 14px 16px',
          position: 'fixed',
          left: 0,
          top: 0,
          bottom: 0,
          zIndex: 100,
        }}
      >
        {/* Logo */}
        <div style={{ display: 'flex', alignItems: 'center', gap: 12, marginBottom: 28, padding: '0 6px' }}>
          <div
            style={{
              width: 36,
              height: 36,
              borderRadius: 10,
              background: 'linear-gradient(135deg, #3b82f6, #6366f1)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              flexShrink: 0,
            }}
          >
            <SafetyOutlined style={{ fontSize: 18, color: '#fff' }} />
          </div>
          <div>
            <div style={{ fontSize: 16, fontWeight: 700, color: '#fff', lineHeight: 1.2 }}>MakeJob</div>
            <div style={{ fontSize: 11, color: '#64748b', lineHeight: 1.2 }}>Admin Panel</div>
          </div>
        </div>

        {/* Nav */}
        <nav style={{ flex: 1, display: 'flex', flexDirection: 'column', gap: 2, overflowY: 'auto' }}>
          {navItems.map((item) => {
            const isActive = pathname === item.to || pathname.startsWith(item.to + '/')
            return (
              <Link
                key={item.to}
                to={item.to}
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  gap: 10,
                  padding: '10px 14px',
                  borderRadius: 10,
                  textDecoration: 'none',
                  fontSize: 14,
                  transition: 'all 0.2s ease',
                  border: '1px solid transparent',
                  color: isActive ? '#fff' : '#94a3b8',
                  background: isActive ? 'rgba(99, 102, 241, 0.2)' : 'transparent',
                  fontWeight: isActive ? 600 : 500,
                }}
                onMouseEnter={(e) => {
                  if (!isActive) {
                    e.currentTarget.style.background = 'rgba(255,255,255,0.06)'
                    e.currentTarget.style.color = '#e2e8f0'
                  }
                }}
                onMouseLeave={(e) => {
                  if (!isActive) {
                    e.currentTarget.style.background = 'transparent'
                    e.currentTarget.style.color = '#94a3b8'
                  }
                }}
              >
                <span style={{ fontSize: 16, opacity: isActive ? 1 : 0.7, display: 'flex', alignItems: 'center' }}>
                  {item.icon}
                </span>
                <span>{item.label}</span>
                {isActive && (
                  <span
                    style={{
                      marginLeft: 'auto',
                      width: 6,
                      height: 6,
                      borderRadius: '50%',
                      background: '#6366f1',
                      flexShrink: 0,
                    }}
                  />
                )}
              </Link>
            )
          })}
        </nav>

        {/* User Card */}
        <div
          style={{
            marginTop: 'auto',
            padding: '14px 12px',
            borderRadius: 12,
            background: 'rgba(255,255,255,0.04)',
            border: '1px solid rgba(255,255,255,0.06)',
          }}
        >
          <div style={{ display: 'flex', alignItems: 'center', gap: 10, marginBottom: 10 }}>
            <div
              style={{
                width: 32,
                height: 32,
                borderRadius: '50%',
                background: 'linear-gradient(135deg, #3b82f6, #8b5cf6)',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                fontSize: 13,
                color: '#fff',
                fontWeight: 600,
                flexShrink: 0,
              }}
            >
              {(user?.username?.[0] || 'A').toUpperCase()}
            </div>
            <div style={{ minWidth: 0 }}>
              <div
                style={{
                  fontSize: 13,
                  fontWeight: 600,
                  color: '#e2e8f0',
                  overflow: 'hidden',
                  textOverflow: 'ellipsis',
                  whiteSpace: 'nowrap',
                }}
              >
                {user?.username || '未同步'}
              </div>
              <div style={{ fontSize: 11, color: '#64748b' }}>{user?.role || '未登录'}</div>
            </div>
          </div>
          {accessToken ? (
            <button
              onClick={() => logout()}
              style={{
                width: '100%',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                gap: 6,
                padding: '8px 0',
                borderRadius: 8,
                background: 'rgba(239, 68, 68, 0.1)',
                border: '1px solid rgba(239, 68, 68, 0.2)',
                color: '#f87171',
                fontSize: 12,
                fontWeight: 600,
                cursor: 'pointer',
                transition: 'all 0.2s ease',
              }}
              onMouseEnter={(e) => {
                e.currentTarget.style.background = 'rgba(239, 68, 68, 0.2)'
              }}
              onMouseLeave={(e) => {
                e.currentTarget.style.background = 'rgba(239, 68, 68, 0.1)'
              }}
            >
              <LogoutOutlined /> 退出登录
            </button>
          ) : (
            <Link
              to="/auth/login"
              style={{
                width: '100%',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                gap: 6,
                padding: '8px 0',
                borderRadius: 8,
                background: 'rgba(59, 130, 246, 0.1)',
                border: '1px solid rgba(59, 130, 246, 0.2)',
                color: '#60a5fa',
                fontSize: 12,
                fontWeight: 600,
                textDecoration: 'none',
                cursor: 'pointer',
                transition: 'all 0.2s ease',
              }}
              onMouseEnter={(e) => {
                e.currentTarget.style.background = 'rgba(59, 130, 246, 0.2)'
              }}
              onMouseLeave={(e) => {
                e.currentTarget.style.background = 'rgba(59, 130, 246, 0.1)'
              }}
            >
              <LoginOutlined /> 前往登录
            </Link>
          )}
        </div>
      </aside>

      <main style={{ flex: 1, marginLeft: sidebarWidth, minHeight: '100vh' }}>
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

  const statusColor = message.includes('失败') || message.includes('错误')
    ? '#ef4444'
    : message.includes('成功') || message.includes('通过')
      ? '#10b981'
      : '#94a3b8'

  return (
    <div
      style={{
        minHeight: '100vh',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        background: 'linear-gradient(135deg, #0f172a 0%, #1e1b4b 50%, #312e81 100%)',
        position: 'relative',
        overflow: 'hidden',
        padding: 24,
      }}
    >
      {/* Decorative orbs */}
      <div
        style={{
          position: 'absolute',
          top: '-10%',
          left: '-5%',
          width: 400,
          height: 400,
          borderRadius: '50%',
          background: 'radial-gradient(circle, rgba(99,102,241,0.3) 0%, transparent 70%)',
          filter: 'blur(60px)',
        }}
      />
      <div
        style={{
          position: 'absolute',
          bottom: '-15%',
          right: '-10%',
          width: 500,
          height: 500,
          borderRadius: '50%',
          background: 'radial-gradient(circle, rgba(59,130,246,0.25) 0%, transparent 70%)',
          filter: 'blur(80px)',
        }}
      />
      <div
        style={{
          position: 'absolute',
          top: '40%',
          right: '15%',
          width: 200,
          height: 200,
          borderRadius: '50%',
          background: 'radial-gradient(circle, rgba(139,92,246,0.2) 0%, transparent 70%)',
          filter: 'blur(50px)',
        }}
      />

      <div
        style={{
          position: 'relative',
          zIndex: 1,
          width: '100%',
          maxWidth: 440,
        }}
      >
        {/* Brand Header */}
        <div style={{ textAlign: 'center', marginBottom: 32 }}>
          <div
            style={{
              width: 64,
              height: 64,
              borderRadius: 20,
              background: 'linear-gradient(135deg, #3b82f6, #6366f1)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              margin: '0 auto 20px',
              boxShadow: '0 8px 32px rgba(99, 102, 241, 0.3)',
            }}
          >
            <SafetyCertificateOutlined style={{ fontSize: 28, color: '#fff' }} />
          </div>
          <h1 style={{ margin: 0, fontSize: 28, fontWeight: 800, color: '#fff', letterSpacing: -0.5 }}>
            MakeJob
          </h1>
          <p style={{ margin: '8px 0 0', fontSize: 14, color: '#94a3b8' }}>管理员控制台登录</p>
        </div>

        {/* Login Card */}
        <div
          style={{
            background: 'rgba(255,255,255,0.95)',
            backdropFilter: 'blur(20px) saturate(180%)',
            WebkitBackdropFilter: 'blur(20px) saturate(180%)',
            borderRadius: 20,
            border: '1px solid rgba(255,255,255,0.15)',
            boxShadow: '0 24px 64px rgba(0,0,0,0.25)',
            padding: '36px 32px',
          }}
        >
          <form onSubmit={handleSubmit}>
            <div style={{ marginBottom: 20 }}>
              <div
                style={{
                  marginBottom: 8,
                  fontSize: 13,
                  fontWeight: 600,
                  color: '#1e293b',
                  display: 'flex',
                  alignItems: 'center',
                  gap: 6,
                }}
              >
                <UserOutlined style={{ fontSize: 12, color: '#64748b' }} />
                邮箱
              </div>
              <Input
                size="large"
                value={form.email}
                onChange={(event) => setForm((current) => ({ ...current, email: event.target.value }))}
                placeholder="请输入管理员邮箱"
                prefix={<UserOutlined style={{ color: '#94a3b8' }} />}
                style={{ borderRadius: 12 }}
              />
            </div>

            <div style={{ marginBottom: 24 }}>
              <div
                style={{
                  marginBottom: 8,
                  fontSize: 13,
                  fontWeight: 600,
                  color: '#1e293b',
                  display: 'flex',
                  alignItems: 'center',
                  gap: 6,
                }}
              >
                <LockOutlined style={{ fontSize: 12, color: '#64748b' }} />
                密码
              </div>
              <Input.Password
                size="large"
                value={form.password}
                onChange={(event) => setForm((current) => ({ ...current, password: event.target.value }))}
                placeholder="请输入密码"
                prefix={<LockOutlined style={{ color: '#94a3b8' }} />}
                style={{ borderRadius: 12 }}
              />
            </div>

            <Button
              type="primary"
              size="large"
              htmlType="submit"
              block
              loading={loading}
              icon={<LoginOutlined />}
              style={{
                borderRadius: 12,
                height: 48,
                fontSize: 15,
                fontWeight: 600,
                background: 'linear-gradient(135deg, #4f46e5, #6366f1)',
                border: 'none',
                boxShadow: '0 4px 16px rgba(79, 70, 229, 0.35)',
              }}
            >
              {loading ? '登录中...' : '登录'}
            </Button>
          </form>

          {/* Status Panel */}
          <div
            style={{
              marginTop: 24,
              padding: '14px 16px',
              borderRadius: 12,
              background: '#f8fafc',
              border: '1px solid #e2e8f0',
            }}
          >
            <div
              style={{
                display: 'flex',
                alignItems: 'center',
                gap: 8,
                marginBottom: 8,
                fontSize: 12,
                color: '#64748b',
              }}
            >
              <KeyOutlined />
              <span>令牌状态：</span>
              <Tag
                style={{
                  margin: 0,
                  borderRadius: 8,
                  fontSize: 11,
                  fontFamily: 'monospace',
                  background: accessToken ? 'rgba(16,185,129,0.08)' : 'rgba(148,163,184,0.08)',
                  color: accessToken ? '#10b981' : '#94a3b8',
                  border: accessToken ? '1px solid rgba(16,185,129,0.2)' : '1px solid rgba(148,163,184,0.2)',
                }}
              >
                {tokenPreview}
              </Tag>
            </div>
            <div
              style={{
                display: 'flex',
                alignItems: 'center',
                gap: 8,
                marginBottom: 8,
                fontSize: 12,
                color: '#64748b',
              }}
            >
              <UserOutlined />
              <span>当前用户：{user?.username || '未同步'}</span>
            </div>
            <div
              style={{
                display: 'flex',
                alignItems: 'center',
                gap: 8,
                fontSize: 12,
                color: statusColor,
                fontWeight: 500,
              }}
            >
              <Spin
                spinning={loading}
                size="small"
                indicator={<SafetyCertificateOutlined spin style={{ fontSize: 12, color: statusColor }} />}
              />
              <span>{message}</span>
            </div>
          </div>
        </div>

        {/* Footer */}
        <p
          style={{
            textAlign: 'center',
            marginTop: 24,
            fontSize: 12,
            color: 'rgba(148,163,184,0.6)',
          }}
        >
          登录后自动校验管理员角色权限
        </p>
      </div>
    </div>
  )
}

/**
 * 统一校验后台受保护路由的管理员权限，避免每个页面重复实现守卫逻辑。
 */
async function ensureAdminRouteAccess() {
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
}

const rootRoute = createRootRouteWithContext<RouterContext>()({
  component: AdminLayout,
})

const indexRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/',
  beforeLoad: () => {
    throw redirect({ to: '/dashboard' })
  },
})

const loginRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: 'auth/login',
  component: LoginPage,
})

const dashboardRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: 'dashboard',
  beforeLoad: ensureAdminRouteAccess,
  component: RuntimeOverviewPage,
})

const runtimeRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: 'runtime',
  beforeLoad: ensureAdminRouteAccess,
  component: RuntimeTasksPage,
})

const aiConfigsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: 'ai-configs',
  beforeLoad: ensureAdminRouteAccess,
  component: AIConfigPage,
})

const ragConfigsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: 'rag-configs',
  beforeLoad: ensureAdminRouteAccess,
  component: RAGConfigPage,
})

const ragKnowledgeRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: 'rag-knowledge',
  beforeLoad: ensureAdminRouteAccess,
  component: RAGKnowledgePage,
})

const promptsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: 'prompts',
  beforeLoad: ensureAdminRouteAccess,
  component: PromptPage,
})

const live2DRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: 'live2d',
  beforeLoad: ensureAdminRouteAccess,
  component: Live2DPage,
})

const ttsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: 'tts',
  beforeLoad: ensureAdminRouteAccess,
  component: TTSPage,
})

const taxonomyRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: 'taxonomy',
  beforeLoad: ensureAdminRouteAccess,
  component: TaxonomyPage,
})

const questionsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: 'questions',
  beforeLoad: ensureAdminRouteAccess,
  component: QuestionPage,
})

const questionSetsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: 'question-sets',
  beforeLoad: ensureAdminRouteAccess,
  component: QuestionSetPage,
})

const questionPipelineRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: 'question-pipeline',
  beforeLoad: ensureAdminRouteAccess,
  component: QuestionPipelinePage,
})

const routeTree = rootRoute.addChildren([
  indexRoute,
  loginRoute,
  dashboardRoute,
  runtimeRoute,
  aiConfigsRoute,
  ragConfigsRoute,
  ragKnowledgeRoute,
  promptsRoute,
  live2DRoute,
  ttsRoute,
  taxonomyRoute,
  questionPipelineRoute,
  questionsRoute,
  questionSetsRoute,
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
