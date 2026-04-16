import { defineStore } from 'pinia'
import { ElMessage } from 'element-plus'

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
    avatar: (state): string => state.user?.avatar || '/default-avatar.png',
  },

  actions: {
    initAuth() {
      if (!process.client) return

      const tokenCookie = useCookie('token')
      const refreshTokenCookie = useCookie('refreshToken')

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
        this.fetchUserInfo()
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

        if (response.code === 200) {
          const token = response.data.token
          const refreshToken = response.data.refresh_token || response.data.refreshToken || ''

          this.setTokens(token, refreshToken)
          this.setUser(normalizeUser(response.data.user))

          ElMessage.success('登录成功')
          return true
        }

        return false
      } catch (error: any) {
        ElMessage.error(error?.message || '登录失败')
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

        if (response.code === 200) {
          const token = response.data.token
          const refreshToken = response.data.refresh_token || response.data.refreshToken || ''

          this.setTokens(token, refreshToken)
          this.setUser(normalizeUser(response.data.user))

          ElMessage.success('注册成功')
          return true
        }

        return false
      } catch (error: any) {
        ElMessage.error(error?.message || '注册失败')
        return false
      } finally {
        this.loading = false
      }
    },

    logout() {
      this.token = null
      this.refreshToken = null
      this.setUser(null)

      const tokenCookie = useCookie('token')
      const refreshTokenCookie = useCookie('refreshToken')
      tokenCookie.value = null
      refreshTokenCookie.value = null

      if (process.client) {
        localStorage.removeItem('token')
        localStorage.removeItem('refreshToken')
        localStorage.removeItem('user')
      }

      navigateTo('/')
      ElMessage.success('已退出登录')
    },

    async fetchUserInfo(): Promise<void> {
      if (!this.token) return

      try {
        const { $api } = useNuxtApp()
        const response = await $api.get<RawUser>('/user/profile')

        if (response.code === 200) {
          this.setUser(normalizeUser(response.data))
        }
      } catch {
        this.logout()
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

        if (response.code === 200) {
          const token = response.data.token
          const refreshToken = response.data.refresh_token || response.data.refreshToken || ''
          this.setTokens(token, refreshToken)
          return true
        }

        return false
      } catch {
        this.logout()
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

      const tokenCookie = useCookie('token', { maxAge: 60 * 60 * 24 * 7 })
      const refreshTokenCookie = useCookie('refreshToken', { maxAge: 60 * 60 * 24 * 30 })
      tokenCookie.value = token
      refreshTokenCookie.value = refreshToken

      if (process.client) {
        localStorage.setItem('token', token)
        localStorage.setItem('refreshToken', refreshToken)
      }
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
  },
})
