import { create } from 'zustand'
import { createApiClient, extractErrorMessage, requestJson } from '@makejob/api-client'
import {
  isSuccessCode,
  normalizeUserProfile,
  type ApiEnvelope,
  type LoginResult,
  type RawUserProfile,
  type UserProfile,
} from '@makejob/shared-types'

const TOKEN_KEY = 'makejob.admin.access-token'
const REFRESH_TOKEN_KEY = 'makejob.admin.refresh-token'
const USER_KEY = 'makejob.admin.user'

interface AdminAuthState {
  accessToken: string | null
  refreshToken: string | null
  user: UserProfile | null
  loading: boolean
  initialized: boolean
  profileLoaded: boolean
  initAuth: () => void
  login: (email: string, password: string) => Promise<{ ok: boolean; message: string }>
  fetchProfile: () => Promise<boolean>
  ensureAdmin: () => Promise<boolean>
  clearSession: () => void
  logout: () => void
}

let profilePromise: Promise<boolean> | null = null

/**
 * 读取后台访问令牌。
 */
function readToken(): string | null {
  if (typeof window === 'undefined') {
    return null
  }

  return window.localStorage.getItem(TOKEN_KEY)
}

/**
 * 读取后台刷新令牌。
 */
function readRefreshToken(): string | null {
  if (typeof window === 'undefined') {
    return null
  }

  return window.localStorage.getItem(REFRESH_TOKEN_KEY)
}

/**
 * 恢复后台本地缓存的用户资料。
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
 * 持久化后台会话，供刷新后恢复管理员状态。
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
 * 提取后台登录接口返回的访问令牌。
 */
function extractAccessToken(payload?: LoginResult | null): string {
  return payload?.token || payload?.access_token || payload?.accessToken || ''
}

/**
 * 提取后台登录接口返回的刷新令牌。
 */
function extractRefreshToken(payload?: LoginResult | null): string | null {
  return payload?.refresh_token || payload?.refreshToken || null
}

/**
 * 生成当前后台会话专用的 API 客户端。
 */
function getApi() {
  return createApiClient(() => useAdminAuthStore.getState().accessToken)
}

export const useAdminAuthStore = create<AdminAuthState>((set, get) => ({
  accessToken: null,
  refreshToken: null,
  user: null,
  loading: false,
  initialized: false,
  profileLoaded: false,

  /**
   * 初始化后台登录态，优先恢复已有管理员会话。
   */
  initAuth() {
    if (get().initialized) {
      return
    }

    const accessToken = readToken()
    const refreshToken = readRefreshToken()
    const user = readStoredUser()

    set({
      accessToken,
      refreshToken,
      user,
      initialized: true,
      profileLoaded: Boolean(user),
    })
  },

  /**
   * 提交后台登录请求，并在成功后补齐管理员资料。
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
          message: response.message || '后台登录失败',
        }
      }

      const accessToken = extractAccessToken(response.data)
      if (!accessToken) {
        return {
          ok: false,
          message: '后台登录成功但未返回 token',
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
        initialized: true,
        profileLoaded: Boolean(user),
      })

      const adminReady = user?.role === 'admin' ? true : await get().ensureAdmin()
      if (!adminReady) {
        get().clearSession()
        return {
          ok: false,
          message: '当前账号不是管理员，无法进入后台',
        }
      }

      return {
        ok: true,
        message: '后台登录成功',
      }
    } catch (error) {
      return {
        ok: false,
        message: extractErrorMessage(error, '后台登录失败，请稍后重试'),
      }
    } finally {
      set({
        loading: false,
      })
    }
  },

  /**
   * 拉取后台当前用户资料，并校验管理员角色。
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
          if (response.code === 401) {
            get().clearSession()
          }
          return false
        }

        const user = normalizeUserProfile(response.data)
        if (!user || user.role !== 'admin') {
          return false
        }

        persistSession({
          accessToken: get().accessToken,
          refreshToken: get().refreshToken,
          user,
        })

        set({
          user,
          profileLoaded: true,
        })

        return true
      } catch {
        return false
      } finally {
        profilePromise = null
      }
    })()

    return profilePromise
  },

  /**
   * 确保当前后台会话已经确认管理员身份。
   */
  async ensureAdmin() {
    if (!get().accessToken) {
      return false
    }

    if (get().user?.role === 'admin' && get().profileLoaded) {
      return true
    }

    return get().fetchProfile()
  },

  /**
   * 清空后台会话与缓存，避免普通用户或失效令牌进入管理页。
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
      loading: false,
      initialized: true,
      profileLoaded: false,
    })
  },

  /**
   * 对外提供统一退出入口，当前以本地清理为主。
   */
  logout() {
    get().clearSession()
  },
}))
