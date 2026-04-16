/**
 * API请求封装
 * 基于 Nuxt3 的 $fetch 封装，支持自动Token携带和统一错误处理
 */

import { ElMessage } from 'element-plus'

// API响应类型
export interface ApiResponse<T = unknown> {
  code: number
  message: string
  data: T
}

// 请求配置类型
interface RequestOptions {
  method?: 'GET' | 'POST' | 'PUT' | 'DELETE' | 'PATCH'
  body?: Record<string, unknown> | FormData
  params?: Record<string, string | number | boolean>
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

/**
 * 获取JWT Token
 * 优先从cookie读取，兼容SSR
 */
const getToken = (): string | null => {
  // 客户端环境
  if (process.client) {
    return useCookie('token').value || localStorage.getItem('token')
  }
  // SSR环境
  return useCookie('token').value
}

/**
 * 处理401未授权错误
 * 清除登录态并跳转登录页
 */
const handleUnauthorized = () => {
  if (process.client) {
    // 清除token
    useCookie('token').value = null
    useCookie('refreshToken').value = null
    localStorage.removeItem('token')
    localStorage.removeItem('refreshToken')
    localStorage.removeItem('user')
    
    // 显示提示
    ElMessage.error('登录已过期，请重新登录')
    
    // 跳转登录页
    navigateTo('/auth/login')
  }
}

/**
 * 统一错误处理
 */
const handleError = (error: any) => {
  if (process.client) {
    const message = error?.data?.message || error?.message || '请求失败，请稍后重试'
    ElMessage.error(message)
  }
  throw error
}

/**
 * 基础请求函数
 */
export const useApi = () => {
  const config = useRuntimeConfig()
  const baseURL = config.public.apiBase

  /**
   * 发送HTTP请求
   */
  const request = async <T = unknown>(
    url: string,
    options: RequestOptions = {}
  ): Promise<ApiResponse<T>> => {
    const { method = 'GET', body, params, headers = {}, skipAuth = false } = options
    const normalizedUrl = normalizeApiUrl(baseURL, url)

    // 构建请求头
    const requestHeaders: Record<string, string> = {
      'Content-Type': 'application/json',
      ...headers,
    }

    // 自动添加Token
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
        // 请求拦截器
        onRequest({ options }) {
          console.log(`[API Request] ${method} ${normalizedUrl}`)
        },
        // 响应拦截器
        onResponse({ response }) {
          console.log(`[API Response] ${response.status}`)
        },
        // 错误处理
        onResponseError({ response }) {
          if (response.status === 401) {
            handleUnauthorized()
          }
        },
      })

      return response
    } catch (error: any) {
      // 处理401错误
      if (error?.response?.status === 401) {
        handleUnauthorized()
      }
      return handleError(error)
    }
  }

  // GET请求
  const get = <T = unknown>(url: string, params?: Record<string, string | number | boolean>) => {
    return request<T>(url, { method: 'GET', params })
  }

  // POST请求
  const post = <T = unknown>(url: string, body?: Record<string, unknown>) => {
    return request<T>(url, { method: 'POST', body })
  }

  // PUT请求
  const put = <T = unknown>(url: string, body?: Record<string, unknown>) => {
    return request<T>(url, { method: 'PUT', body })
  }

  // DELETE请求
  const del = <T = unknown>(url: string, params?: Record<string, string | number | boolean>) => {
    return request<T>(url, { method: 'DELETE', params })
  }

  // PATCH请求
  const patch = <T = unknown>(url: string, body?: Record<string, unknown>) => {
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

/**
 * 使用useFetch的封装（支持SSR）
 */
export const useApiFetch = <T = unknown>(
  url: string,
  options: RequestOptions = {}
) => {
  const config = useRuntimeConfig()
  const normalizedUrl = normalizeApiUrl(config.public.apiBase, url)
  const { skipAuth = false, ...fetchOptions } = options

  // 构建请求头
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...options.headers,
  }

  // 添加Token
  if (!skipAuth && process.client) {
    const token = getToken()
    if (token) {
      headers.Authorization = `Bearer ${token}`
    }
  }

  return useFetch<ApiResponse<T>>(normalizedUrl, {
    baseURL: config.public.apiBase,
    ...fetchOptions,
    headers,
    onResponseError({ response }) {
      if (response.status === 401) {
        handleUnauthorized()
      }
    },
  })
}
