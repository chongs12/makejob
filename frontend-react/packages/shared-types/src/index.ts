export interface ApiEnvelope<T> {
  code: number
  message: string
  data: T
}

export interface LoginPayload {
  email: string
  password: string
}

export interface LoginResult {
  token?: string
  access_token?: string
  accessToken?: string
  refresh_token?: string
  refreshToken?: string
}

export interface AuthSession {
  accessToken: string | null
  refreshToken: string | null
}

/**
 * 会员套餐等级，与后端 UserMembership.Level 一致。
 * free = 无有效付费套餐；其余为付费套餐。是否付费由 tier !== 'free' 派生。
 */
export type MembershipTier = 'free' | 'monthly' | 'quarterly' | 'yearly'

export interface UserProfile {
  id: number
  username: string
  email: string
  avatar?: string
  phone?: string
  bio?: string
  role: 'user' | 'admin'
  rawRole?: string
  membershipLevel: MembershipTier
  membershipExpireAt?: string
  industry?: string
  targetPosition?: string
  createdAt?: string
  updatedAt?: string
}

export interface RawUserProfile {
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
}

/**
 * 判断当前接口响应是否属于后端定义的成功状态。
 */
export function isSuccessCode(code: number | undefined): boolean {
  return code === 0 || code === 200
}

/**
 * 将后端原始用户结构转换为前端统一可用的资料结构。
 */
export function normalizeUserProfile(profile?: RawUserProfile | null): UserProfile | null {
  if (!profile) {
    return null
  }

  return {
    id: profile.id,
    username: profile.username,
    email: profile.email,
    avatar: profile.avatar,
    phone: profile.phone,
    bio: profile.bio,
    role: profile.role === 'admin' ? 'admin' : 'user',
    rawRole: profile.role,
    membershipLevel: (profile.membership_level || profile.membershipLevel || 'free') as MembershipTier,
    membershipExpireAt: profile.membership_expire_at || profile.membershipExpireAt,
    industry: profile.industry,
    targetPosition: profile.target_position || profile.targetPosition,
    createdAt: profile.created_at || profile.createdAt,
    updatedAt: profile.updated_at || profile.updatedAt,
  }
}
