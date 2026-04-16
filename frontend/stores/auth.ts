import { defineStore } from 'pinia'
import { ElMessage } from 'element-plus'
import { useUserStore } from './user'

export interface User {
  id: number
  username: string
  email: string
  avatar?: string
  role: 'user' | 'admin'
  rawRole?: string
  membershipLevel?: 'free' | 'pro'
  membershipExpireAt?: string
  createdAt?: string
}

interface RawUser {
  id: number
  username: string
  email: string
  avatar?: string
  role?: string
  membership_level?: string
  membershipLevel?: string
  membership_expire_at?: string
  membershipExpireAt?: string
  created_at?: string
  createdAt?: string
}

interface AuthState {
  token: string | null
  refreshToken: string | null
  user: User | null
  loading: boolean
}

interface LogoutOptions {
  redirectTo?: string
  silent?: boolean
  callApi?: boolean
}

const isSuccessCode = (code: number): boolean => code === 0 || code === 200

const normalizeUser = (user: RawUser | null | undefined): User | null => {
  if (!user) return null

  return {
    id: user.id,
    username: user.username,
    email: user.email,
    avatar: user.avatar,
    role: user.role === 'admin' ? 'admin' : 'user',
    rawRole: user.role,
    membershipLevel: (user.membership_level || user.membershipLevel || 'free') as 'free' | 'pro',
    membershipExpireAt: user.membership_expire_at || user.membershipExpireAt,
    createdAt: user.created_at || user.createdAt,
  }
}

export const useAuthStore = defineStore('auth', {
  state: (): AuthState => ({
    token: null,
    refreshToken: null,
    user: null,
    loading: false,
  }),

  getters: {
    isLoggedIn: (state): boolean => !!state.token,
    isAdmin: (state): boolean => state.user?.role === 'admin',
    username: (state): string => state.user?.username || '',
    avatar: (state): string => state.user?.avatar || '',
  },

  actions: {
    initAuth() {
      if (!process.client) return

      const tokenCookie = useCookie<string | null>('token')
      const refreshTokenCookie = useCookie<string | null>('refreshToken')

      this.token = tokenCookie.value || localStorage.getItem('token')
      this.refreshToken = refreshTokenCookie.value || localStorage.getItem('refreshToken')

      const userStr = localStorage.getItem('user')
      if (userStr) {
        try {
          this.user = normalizeUser(JSON.parse(userStr))
        } catch {
          localStorage.removeItem('user')
        }
      }

      if (this.token && !this.user) {
        void this.fetchUserInfo()
      }
    },

    async login(email: string, password: string): Promise<boolean> {
      this.loading = true

      try {
        const { $api } = useNuxtApp()
        const response = await $api.post<{
          token: string
          refresh_token?: string
          refreshToken?: string
          user: RawUser
        }>('/auth/login', { email, password })

        if (!isSuccessCode(response.code)) return false

        const token = response.data.token
        const refreshToken = response.data.refresh_token || response.data.refreshToken || ''

        this.setTokens(token, refreshToken)
        this.setUser(normalizeUser(response.data.user))

        ElMessage.success('登录成功')
        return true
      } catch {
        return false
      } finally {
        this.loading = false
      }
    },

    async register(username: string, email: string, password: string): Promise<boolean> {
      this.loading = true

      try {
        const { $api } = useNuxtApp()
        const response = await $api.post<{
          token: string
          refresh_token?: string
          refreshToken?: string
          user: RawUser
        }>('/auth/register', { username, email, password })

        if (!isSuccessCode(response.code)) return false

        const token = response.data.token
        const refreshToken = response.data.refresh_token || response.data.refreshToken || ''

        this.setTokens(token, refreshToken)
        this.setUser(normalizeUser(response.data.user))

        ElMessage.success('注册成功')
        return true
      } catch {
        return false
      } finally {
        this.loading = false
      }
    },

    async logout(options: LogoutOptions = {}) {
      const { redirectTo = '/', silent = false, callApi = true } = options

      if (callApi && this.token) {
        try {
          const { $api } = useNuxtApp()
          await $api.post('/auth/logout', {})
        } catch {
          // Best effort logout. Local cleanup is the real source of truth for now.
        }
      }

      this.clearAuthState()

      if (redirectTo) {
        await navigateTo(redirectTo)
      }

      if (!silent) {
        ElMessage.success('已退出登录')
      }
    },

    async fetchUserInfo(): Promise<void> {
      if (!this.token) return

      try {
        const { $api } = useNuxtApp()
        const response = await $api.get<RawUser>('/user/profile')

        if (isSuccessCode(response.code)) {
          this.setUser(normalizeUser(response.data))
          return
        }

        this.clearAuthState()
      } catch {
        this.clearAuthState()
      }
    },

    async refreshAuthToken(): Promise<boolean> {
      if (!this.refreshToken) return false

      try {
        const { $api } = useNuxtApp()
        const response = await $api.post<{
          token: string
          refresh_token?: string
          refreshToken?: string
        }>('/auth/refresh', {
          refresh_token: this.refreshToken,
          refreshToken: this.refreshToken,
        })

        if (!isSuccessCode(response.code)) {
          this.clearAuthState()
          return false
        }

        const token = response.data.token
        const refreshToken = response.data.refresh_token || response.data.refreshToken || ''

        this.setTokens(token, refreshToken)
        return true
      } catch {
        this.clearAuthState()
        return false
      }
    },

    updateUserInfo(userInfo: Partial<User>) {
      if (!this.user) return
      this.setUser({ ...this.user, ...userInfo })
    },

    setTokens(token: string, refreshToken: string) {
      this.token = token
      this.refreshToken = refreshToken

      const tokenCookie = useCookie<string | null>('token', { maxAge: 60 * 60 * 24 * 7 })
      const refreshTokenCookie = useCookie<string | null>('refreshToken', { maxAge: 60 * 60 * 24 * 30 })

      tokenCookie.value = token
      refreshTokenCookie.value = refreshToken

      if (!process.client) return

      localStorage.setItem('token', token)
      localStorage.setItem('refreshToken', refreshToken)
    },

    setUser(user: User | null) {
      this.user = user

      if (!process.client) return

      if (user) {
        localStorage.setItem('user', JSON.stringify(user))
      } else {
        localStorage.removeItem('user')
      }
    },

    clearAuthState() {
      this.token = null
      this.refreshToken = null
      this.user = null
      this.loading = false

      const tokenCookie = useCookie<string | null>('token')
      const refreshTokenCookie = useCookie<string | null>('refreshToken')
      tokenCookie.value = null
      refreshTokenCookie.value = null

      const userStore = useUserStore()
      userStore.clearUserData()

      if (!process.client) return

      localStorage.removeItem('token')
      localStorage.removeItem('refreshToken')
      localStorage.removeItem('user')
    },
  },
})
