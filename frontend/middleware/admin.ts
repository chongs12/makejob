export default defineNuxtRouteMiddleware(async () => {
  const token = useCookie('token').value

  if (!token) {
    return navigateTo('/auth/login', { replace: true })
  }

  const authStore = useAuthStore()

  if (!authStore.token) {
    authStore.initAuth()
  }

  if (!authStore.user) {
    await authStore.fetchUserInfo()
  }

  if (!authStore.isAdmin) {
    return navigateTo('/', { replace: true })
  }
})
