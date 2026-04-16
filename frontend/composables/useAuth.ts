/**
 * 认证相关组合函数
 * 提供登录态检查、权限验证等功能
 */

import { useAuthStore } from '~/stores/auth'

/**
 * 检查用户是否已登录
 */
export const useIsLoggedIn = (): boolean => {
  const authStore = useAuthStore()
  return authStore.isLoggedIn
}

/**
 * 检查用户是否为管理员
 */
export const useIsAdmin = (): boolean => {
  const authStore = useAuthStore()
  return authStore.isAdmin
}

/**
 * 获取当前用户信息
 */
export const useCurrentUser = () => {
  const authStore = useAuthStore()
  return computed(() => authStore.user)
}

/**
 * 执行登录
 */
export const useLogin = () => {
  const authStore = useAuthStore()
  
  return async (email: string, password: string) => {
    return await authStore.login(email, password)
  }
}

/**
 * 执行注册
 */
export const useRegister = () => {
  const authStore = useAuthStore()
  
  return async (username: string, email: string, password: string) => {
    return await authStore.register(username, email, password)
  }
}

/**
 * 执行登出
 */
export const useLogout = () => {
  const authStore = useAuthStore()
  
  return () => {
    authStore.logout()
  }
}

/**
 * 检查登录态并跳转
 * 未登录时跳转到登录页
 */
export const useRequireAuth = () => {
  const authStore = useAuthStore()
  const router = useRouter()
  
  return () => {
    if (!authStore.isLoggedIn) {
      router.push('/auth/login')
      return false
    }
    return true
  }
}

/**
 * 检查管理员权限并跳转
 * 非管理员跳转到首页
 */
export const useRequireAdmin = () => {
  const authStore = useAuthStore()
  const router = useRouter()
  
  return () => {
    if (!authStore.isLoggedIn) {
      router.push('/auth/login')
      return false
    }
    if (!authStore.isAdmin) {
      router.push('/')
      return false
    }
    return true
  }
}
