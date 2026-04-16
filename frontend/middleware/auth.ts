/**
 * 认证中间件
 * 检查用户是否已登录，未登录则跳转到登录页
 */

export default defineNuxtRouteMiddleware(async (to, from) => {
  // 跳过登录和注册页面
  if (to.path.startsWith('/auth/')) {
    return
  }

  // 获取token（优先从cookie）
  const authStore = useAuthStore()
  if (!authStore.token) {
    authStore.initAuth()
  }

  const token = authStore.token || useCookie('token').value
  
  // 未登录，重定向到登录页
  if (!token) {
    return navigateTo('/auth/login', { replace: true })
  }

  if (!authStore.user) {
    await authStore.fetchUserInfo()
  }
})
