import { defineStore } from 'pinia'
import { ElMessage } from 'element-plus'

export interface UserProfile {
  id: number
  username: string
  email: string
  avatar?: string
  phone?: string
  bio?: string
  role: 'user' | 'admin'
  rawRole?: string
  membershipLevel: 'free' | 'pro'
  membershipExpireAt?: string
  industry?: string
  targetPosition?: string
  createdAt?: string
  updatedAt?: string
  stats?: {
    totalQuestions: number
    correctRate: number
    interviewCount: number
    studyDays: number
  }
}

interface RawUserProfile {
  id: number
  username: string
  email: string
  avatar?: string
  phone?: string
  bio?: string
  role?: string
  membership_level?: string
  membershipLevel?: string
  membership_expire_at?: string
  membershipExpireAt?: string
  industry?: string
  target_position?: string
  targetPosition?: string
  created_at?: string
  createdAt?: string
  updated_at?: string
  updatedAt?: string
  stats?: {
    totalQuestions: number
    correctRate: number
    interviewCount: number
    studyDays: number
  }
}

interface UserState {
  profile: UserProfile | null
  loading: boolean
}

const normalizeProfile = (profile: RawUserProfile | null | undefined): UserProfile | null => {
  if (!profile) return null

  return {
    id: profile.id,
    username: profile.username,
    email: profile.email,
    avatar: profile.avatar,
    phone: profile.phone,
    bio: profile.bio,
    role: profile.role === 'admin' ? 'admin' : 'user',
    rawRole: profile.role,
    membershipLevel: (profile.membership_level || profile.membershipLevel || 'free') as 'free' | 'pro',
    membershipExpireAt: profile.membership_expire_at || profile.membershipExpireAt,
    industry: profile.industry,
    targetPosition: profile.target_position || profile.targetPosition,
    createdAt: profile.created_at || profile.createdAt,
    updatedAt: profile.updated_at || profile.updatedAt,
    stats: profile.stats,
  }
}

export const useUserStore = defineStore('user', {
  state: (): UserState => ({
    profile: null,
    loading: false,
  }),

  getters: {
    hasProfile: (state): boolean => !!state.profile,
    membershipType: (state): string => state.profile?.membershipLevel || 'free',
    isPaidMember: (state): boolean => state.profile?.membershipLevel === 'pro',
    targetIndustry: (state): string => state.profile?.industry || '',
  },

  actions: {
    async fetchProfile(): Promise<boolean> {
      this.loading = true

      try {
        const { $api } = useNuxtApp()
        const response = await $api.get<RawUserProfile>('/user/profile')

        if (response.code === 200) {
          this.profile = normalizeProfile(response.data)
          return true
        }

        return false
      } catch (error: any) {
        ElMessage.error(error?.message || '获取用户信息失败')
        return false
      } finally {
        this.loading = false
      }
    },

    async updateProfile(data: Partial<UserProfile>): Promise<boolean> {
      this.loading = true

      try {
        const { $api } = useNuxtApp()
        const response = await $api.put('/user/profile', data as Record<string, unknown>)

        if (response.code === 200) {
          this.profile = {
            ...(this.profile || ({} as UserProfile)),
            ...data,
          } as UserProfile
          ElMessage.success('个人信息更新成功')
          return true
        }

        return false
      } catch (error: any) {
        ElMessage.error(error?.message || '更新失败')
        return false
      } finally {
        this.loading = false
      }
    },

    async uploadAvatar(file: File): Promise<string | null> {
      this.loading = true

      try {
        const formData = new FormData()
        formData.append('avatar', file)

        const { $api } = useNuxtApp()
        const response = await $api.post<{ url: string }>('/user/avatar', formData as any)

        if (response.code === 200) {
          const avatarUrl = response.data.url
          if (this.profile) {
            this.profile.avatar = avatarUrl
          }
          ElMessage.success('头像上传成功')
          return avatarUrl
        }

        return null
      } catch (error: any) {
        ElMessage.error(error?.message || '上传失败')
        return null
      } finally {
        this.loading = false
      }
    },

    async changePassword(oldPassword: string, newPassword: string): Promise<boolean> {
      try {
        const { $api } = useNuxtApp()
        const response = await $api.post('/user/change-password', {
          oldPassword,
          newPassword,
        })

        if (response.code === 200) {
          ElMessage.success('密码修改成功')
          return true
        }

        return false
      } catch (error: any) {
        ElMessage.error(error?.message || '修改失败')
        return false
      }
    },

    clearUserData() {
      this.profile = null
      this.loading = false
    },
  },
})
