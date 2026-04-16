import { ElMessage } from 'element-plus'
import { useAuthStore } from '~/stores/auth'

export interface ApiResponse<T = unknown> {
  code: number
  message: string
  data: T
}

interface RequestOptions {
  method?: 'GET' | 'POST' | 'PUT' | 'DELETE' | 'PATCH'
  body?: Record<string, unknown> | FormData
  params?: Record<string, string | number | boolean | undefined>
  headers?: Record<string, string>
  skipAuth?: boolean
}

const normalizeApiUrl = (baseURL: string, url: string): string => {
  if (!url.startsWith('/api/')) return url

  const normalizedBase = baseURL.replace(/\/+$/, '')
  if (normalizedBase.endsWith('/api')) {
    return url.slice(4)
  }

  return url
}

const getToken = (): string | null => {
  if (process.client) {
    return useCookie<string | null>('token').value || localStorage.getItem('token')
  }

  return useCookie<string | null>('token').value
}

const handleUnauthorized = async () => {
  if (!process.client) return

  const authStore = useAuthStore()
  authStore.clearAuthState()

  ElMessage.error('登录状态已失效，请重新登录')

  if (useRoute().path !== '/auth/login') {
    await navigateTo('/auth/login')
  }
}

const handleError = (error: any) => {
  if (process.client) {
    const message = error?.data?.message || error?.message || '请求失败，请稍后重试'
    ElMessage.error(message)
  }

  throw error
}

export const useApi = () => {
  const config = useRuntimeConfig()
  const baseURL = config.public.apiBase

  const request = async <T = unknown>(
    url: string,
    options: RequestOptions = {},
  ): Promise<ApiResponse<T>> => {
    const { method = 'GET', body, params, headers = {}, skipAuth = false } = options
    const normalizedUrl = normalizeApiUrl(baseURL, url)

    const requestHeaders: Record<string, string> = {
      ...headers,
    }

    if (!(body instanceof FormData)) {
      requestHeaders['Content-Type'] = 'application/json'
    }

    if (!skipAuth) {
      const token = getToken()
      if (token) {
        requestHeaders.Authorization = `Bearer ${token}`
      }
    }

    try {
      const response = await $fetch<ApiResponse<T>>(normalizedUrl, {
        baseURL,
        method,
        body,
        params,
        headers: requestHeaders,
        onResponseError: async ({ response }) => {
          if (response.status === 401) {
            await handleUnauthorized()
          }
        },
      })

      return response
    } catch (error: any) {
      if (error?.response?.status === 401) {
        await handleUnauthorized()
      }

      return handleError(error)
    }
  }

  const get = <T = unknown>(url: string, params?: Record<string, string | number | boolean | undefined>) => {
    return request<T>(url, { method: 'GET', params })
  }

  const post = <T = unknown>(url: string, body?: Record<string, unknown> | FormData) => {
    return request<T>(url, { method: 'POST', body })
  }

  const put = <T = unknown>(url: string, body?: Record<string, unknown> | FormData) => {
    return request<T>(url, { method: 'PUT', body })
  }

  const del = <T = unknown>(url: string, params?: Record<string, string | number | boolean | undefined>) => {
    return request<T>(url, { method: 'DELETE', params })
  }

  const patch = <T = unknown>(url: string, body?: Record<string, unknown> | FormData) => {
    return request<T>(url, { method: 'PATCH', body })
  }

  return {
    request,
    get,
    post,
    put,
    delete: del,
    patch,
  }
}

export const useApiFetch = <T = unknown>(
  url: string,
  options: RequestOptions = {},
) => {
  const config = useRuntimeConfig()
  const normalizedUrl = normalizeApiUrl(config.public.apiBase, url)
  const { skipAuth = false, ...fetchOptions } = options

  const headers: Record<string, string> = {
    ...options.headers,
  }

  if (!(options.body instanceof FormData)) {
    headers['Content-Type'] = 'application/json'
  }

  if (!skipAuth) {
    const token = getToken()
    if (token) {
      headers.Authorization = `Bearer ${token}`
    }
  }

  return useFetch<ApiResponse<T>>(normalizedUrl, {
    baseURL: config.public.apiBase,
    ...fetchOptions,
    headers,
    onResponseError: async ({ response }) => {
      if (response.status === 401) {
        await handleUnauthorized()
      }
    },
  })
}
