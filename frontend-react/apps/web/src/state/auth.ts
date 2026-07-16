import { create } from 'zustand'
import { AUTH_EXPIRED_EVENT_NAME, createApiClient, extractErrorMessage, getApiBaseUrl, requestJson } from '@makejob/api-client'
import {
  isSuccessCode,
  normalizeUserProfile,
  type ApiEnvelope,
  type LoginResult,
  type MembershipTier,
  type RawUserProfile,
  type UserProfile,
} from '@makejob/shared-types'

const TOKEN_KEY = 'makejob.web.access-token'
const REFRESH_TOKEN_KEY = 'makejob.web.refresh-token'
const USER_KEY = 'makejob.web.user'

/**
 * 检查 JWT 是否已过期。解析 payload 中的 exp 字段，与当前时间比较。
 */
function isTokenExpired(token: string | null): boolean {
  if (!token) {
    return true
  }
  try {
    const parts = token.split('.')
    if (parts.length !== 3) {
      return true
    }
    const payload = JSON.parse(atob(parts[1]))
    return typeof payload.exp === 'number' && payload.exp * 1000 < Date.now()
  } catch {
    return true
  }
}

interface AuthState {
  accessToken: string | null
  refreshToken: string | null
  user: UserProfile | null
  membershipLevel: MembershipTier | null
  loading: boolean
  initialized: boolean
  profileLoaded: boolean
  initAuth: () => void
  login: (email: string, password: string) => Promise<{ ok: boolean; message: string }>
  register: (username: string, email: string, password: string) => Promise<{ ok: boolean; message: string }>
  fetchProfile: () => Promise<boolean>
  fetchMembership: () => Promise<boolean>
  ensureProfile: () => Promise<boolean>
  refreshSession: () => Promise<boolean>
  clearSession: () => void
  logout: () => void
}

let profilePromise: Promise<boolean> | null = null
let refreshPromise: Promise<boolean> | null = null
let authExpiredListenerBound = false

/**
 * 读取浏览器本地保存的访问令牌。
 */
function readToken(): string | null {
  if (typeof window === 'undefined') {
    return null
  }

  return window.localStorage.getItem(TOKEN_KEY)
}

/**
 * 读取浏览器本地保存的刷新令牌。
 */
function readRefreshToken(): string | null {
  if (typeof window === 'undefined') {
    return null
  }

  return window.localStorage.getItem(REFRESH_TOKEN_KEY)
}

/**
 * 恢复浏览器本地缓存的用户资料，减少首次进入时的界面抖动。
 */
function readStoredUser(): UserProfile | null {
  if (typeof window === 'undefined') {
    return null
  }

  try {
    const raw = window.localStorage.getItem(USER_KEY)
    if (!raw) {
      return null
    }

    return JSON.parse(raw) as UserProfile
  } catch {
    return null
  }
}

/**
 * 将最新会话信息同步到本地存储，供刷新恢复使用。
 */
function persistSession(session: {
  accessToken: string | null
  refreshToken: string | null
  user: UserProfile | null
}): void {
  if (typeof window === 'undefined') {
    return
  }

  if (session.accessToken) {
    window.localStorage.setItem(TOKEN_KEY, session.accessToken)
  } else {
    window.localStorage.removeItem(TOKEN_KEY)
  }

  if (session.refreshToken) {
    window.localStorage.setItem(REFRESH_TOKEN_KEY, session.refreshToken)
  } else {
    window.localStorage.removeItem(REFRESH_TOKEN_KEY)
  }

  if (session.user) {
    window.localStorage.setItem(USER_KEY, JSON.stringify(session.user))
  } else {
    window.localStorage.removeItem(USER_KEY)
  }
}

/**
 * 从登录接口响应中提取访问令牌，兼容后端已有多种字段命名。
 */
function extractAccessToken(payload?: LoginResult | null): string {
  return payload?.token || payload?.access_token || payload?.accessToken || ''
}

/**
 * 从登录接口响应中提取刷新令牌，后续可用于续期。
 */
function extractRefreshToken(payload?: LoginResult | null): string | null {
  return payload?.refresh_token || payload?.refreshToken || null
}

/**
 * 生成当前前台会话专用的 API 客户端。
 */
function getApi() {
  return createApiClient(() => useAuthStore.getState().accessToken)
}

/**
 * 直接调用刷新令牌接口，避免 requestJson 在 401 时再次广播失效事件造成循环。
 */
async function requestRefreshSession(refreshToken: string): Promise<ApiEnvelope<LoginResult>> {
  const response = await fetch(`${getApiBaseUrl().replace(/\/+$/, '')}/auth/refresh`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({
      refresh_token: refreshToken,
    }),
  })
  const contentType = response.headers.get('content-type') || ''
  if (!contentType.includes('application/json')) {
    throw new Error(`刷新令牌接口未返回 JSON，状态码：${response.status}`)
  }
  return (await response.json()) as ApiEnvelope<LoginResult>
}

/**
 * 监听全局登录态失效事件，并在收到后统一清空当前前台会话。
 */
function bindAuthExpiredListener(handleAuthExpired: () => void): void {
  if (authExpiredListenerBound || typeof window === 'undefined') {
    return
  }

  window.addEventListener(AUTH_EXPIRED_EVENT_NAME, () => {
    handleAuthExpired()
  })
  authExpiredListenerBound = true
}

export const useAuthStore = create<AuthState>((set, get) => ({
  accessToken: null,
  refreshToken: null,
  user: null,
  membershipLevel: null,
  loading: false,
  initialized: false,
  profileLoaded: false,

  /**
   * 初始化前台登录态，只从本地缓存恢复，不主动阻塞页面渲染。
   * 会检查 JWT 是否过期，过期则清空 token 避免后续 API 调用触发 401 级联。
   */
  initAuth() {
    if (get().initialized) {
      bindAuthExpiredListener(() => {
        void get().refreshSession()
      })
      return
    }

    bindAuthExpiredListener(() => {
      void get().refreshSession()
    })
    const accessToken = readToken()
    const refreshToken = readRefreshToken()
    const user = readStoredUser()

    // 检查 JWT 是否过期，过期则不设置 token，避免页面渲染后触发 401 级联
    if (isTokenExpired(accessToken)) {
      set({
        accessToken: null,
        refreshToken,
        user: null,
        initialized: true,
        profileLoaded: false,
      })
      return
    }

    set({
      accessToken,
      refreshToken,
      user,
      initialized: true,
      profileLoaded: Boolean(user),
    })
  },

  /**
   * 提交前台登录请求，并在成功后立即同步用户资料。
   */
  async login(email: string, password: string) {
    set({
      loading: true,
    })

    try {
      const response = await requestJson<ApiEnvelope<LoginResult & { user?: RawUserProfile }>>('/auth/login', {
        method: 'POST',
        body: { email, password },
      })

      if (!isSuccessCode(response.code)) {
        return {
          ok: false,
          message: response.message || '登录失败',
        }
      }

      const accessToken = extractAccessToken(response.data)
      if (!accessToken) {
        return {
          ok: false,
          message: '登录成功但未返回 token',
        }
      }

      const user = normalizeUserProfile(response.data.user)
      const refreshToken = extractRefreshToken(response.data)

      persistSession({
        accessToken,
        refreshToken,
        user,
      })

      set({
        accessToken,
        refreshToken,
        user,
        loading: false,
        initialized: true,
        profileLoaded: Boolean(user),
      })

      if (!user) {
        await get().fetchProfile()
      }

      return {
        ok: true,
        message: '登录成功',
      }
    } catch (error) {
      set({
        loading: false,
      })

      return {
        ok: false,
        message: extractErrorMessage(error, '登录失败，请稍后重试'),
      }
    } finally {
      set({
        loading: false,
      })
    }
  },

  /**
   * 提交注册请求，成功后直接用返回的会话令牌登录（后端 Register 返回 AuthResponse）。
   */
  async register(username: string, email: string, password: string) {
    set({
      loading: true,
    })

    try {
      const response = await requestJson<ApiEnvelope<LoginResult & { user?: RawUserProfile }>>('/auth/register', {
        method: 'POST',
        body: { username, email, password },
      })

      if (!isSuccessCode(response.code)) {
        return {
          ok: false,
          message: response.message || '注册失败',
        }
      }

      const accessToken = extractAccessToken(response.data)
      if (!accessToken) {
        return {
          ok: false,
          message: '注册成功但未返回 token',
        }
      }

      const user = normalizeUserProfile(response.data.user)
      const refreshToken = extractRefreshToken(response.data)

      persistSession({
        accessToken,
        refreshToken,
        user,
      })

      set({
        accessToken,
        refreshToken,
        user,
        loading: false,
        initialized: true,
        profileLoaded: Boolean(user),
      })

      if (!user) {
        await get().fetchProfile()
      }

      return {
        ok: true,
        message: '注册成功',
      }
    } catch (error) {
      set({
        loading: false,
      })

      return {
        ok: false,
        message: extractErrorMessage(error, '注册失败，请稍后重试'),
      }
    } finally {
      set({
        loading: false,
      })
    }
  },

  /**
   * 拉取当前登录用户资料，并更新本地缓存。
   */
  async fetchProfile() {
    const accessToken = get().accessToken
    if (!accessToken) {
      return false
    }

    if (profilePromise) {
      return profilePromise
    }

    profilePromise = (async () => {
      try {
        const response = await getApi().get<ApiEnvelope<RawUserProfile>>('/user/profile')
        if (!isSuccessCode(response.code)) {
          return false
        }

        const user = normalizeUserProfile(response.data)
        persistSession({
          accessToken: get().accessToken,
          refreshToken: get().refreshToken,
          user,
        })

        set({
          user,
          profileLoaded: true,
        })

        // 会员等级以 membership 服务为权威来源，资料加载后异步拉取一次供前端门禁使用。
        void get().fetchMembership()

        return Boolean(user)
      } catch {
        return false
      } finally {
        profilePromise = null
      }
    })()

    return profilePromise
  },

  /**
   * 拉取当前用户会员等级（来自 membership 服务，升级后即时反映）。
   */
  async fetchMembership() {
    const accessToken = get().accessToken
    if (!accessToken) {
      return false
    }
    try {
      const response = await requestJson<ApiEnvelope<{ level: string }>>('/membership/info', { token: accessToken })
      if (!isSuccessCode(response.code) || !response.data) {
        return false
      }
      const level = (response.data.level || 'free') as MembershipTier
      set({ membershipLevel: level })
      return true
    } catch {
      return false
    }
  },

  /**
   * 确保当前会话已经具备完整用户资料，供需要鉴权的页面复用。
   */
  async ensureProfile() {
    if (!get().accessToken) {
      return false
    }

    if (get().user && get().profileLoaded) {
      return true
    }

    if (await get().fetchProfile()) {
      return true
    }
    if (!(await get().refreshSession())) {
      return false
    }
    if (get().user && get().profileLoaded) {
      return true
    }
    return get().fetchProfile()
  },

  /**
   * 使用本地持久化的 refresh token 续期会话，尽量避免用户因 access token 过期被迫重复登录。
   */
  async refreshSession() {
    const currentRefreshToken = get().refreshToken || readRefreshToken()
    if (!currentRefreshToken) {
      get().clearSession()
      return false
    }
    if (refreshPromise) {
      return refreshPromise
    }

    refreshPromise = (async () => {
      try {
        const response = await requestRefreshSession(currentRefreshToken)
        if (!isSuccessCode(response.code)) {
          get().clearSession()
          return false
        }

        const accessToken = extractAccessToken(response.data)
        if (!accessToken) {
          get().clearSession()
          return false
        }

        const nextRefreshToken = extractRefreshToken(response.data) || currentRefreshToken
        persistSession({
          accessToken,
          refreshToken: nextRefreshToken,
          user: get().user,
        })

        set({
          accessToken,
          refreshToken: nextRefreshToken,
          initialized: true,
        })

        if (get().user && get().profileLoaded) {
          return true
        }
        return get().fetchProfile()
      } catch {
        get().clearSession()
        return false
      } finally {
        refreshPromise = null
      }
    })()

    return refreshPromise
  },

  /**
   * 清空前台会话与缓存，避免失效令牌继续参与请求。
   */
  clearSession() {
    persistSession({
      accessToken: null,
      refreshToken: null,
      user: null,
    })

    set({
      accessToken: null,
      refreshToken: null,
      user: null,
      membershipLevel: null,
      loading: false,
      initialized: true,
      profileLoaded: false,
    })
  },

  /**
   * 对外暴露统一退出动作，当前阶段以本地清理为主。
   */
  logout() {
    get().clearSession()
  },
}))
