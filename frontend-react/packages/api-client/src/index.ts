export interface RequestOptions {
  method?: 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE'
  body?: BodyInit | Record<string, unknown> | null
  headers?: Record<string, string>
  token?: string | null
  signal?: AbortSignal
}

/**
 * 读取当前运行环境中的 API 根地址，默认回退到本地代理入口。
 */
export function getApiBaseUrl(): string {
  const envBase =
    typeof import.meta !== 'undefined' && import.meta.env
      ? import.meta.env.VITE_API_BASE_URL
      : undefined

  return typeof envBase === 'string' && envBase.trim() ? envBase.trim() : '/api'
}

/**
 * 统一生成需要鉴权时的 Authorization 请求头。
 */
export function buildAuthorizationHeader(token?: string | null): Record<string, string> {
  if (!token) {
    return {}
  }

  return {
    Authorization: `Bearer ${token}`,
  }
}

/**
 * 从常见错误结构中提取更适合页面展示的提示文案。
 */
export function extractErrorMessage(error: unknown, fallback: string): string {
  if (error instanceof Error && error.message) {
    return error.message
  }

  return fallback
}

/**
 * 将普通对象请求体转换为浏览器可直接发送的 JSON 载荷。
 */
function normalizeBody(body?: BodyInit | Record<string, unknown> | null): BodyInit | undefined {
  if (!body) {
    return undefined
  }

  if (body instanceof FormData || body instanceof URLSearchParams || body instanceof Blob) {
    return body
  }

  return JSON.stringify(body)
}

/**
 * 判断当前请求体是否需要自动补充 JSON Content-Type。
 */
function shouldUseJsonContentType(body?: BodyInit | Record<string, unknown> | null): boolean {
  if (!body) {
    return false
  }

  return !(body instanceof FormData || body instanceof URLSearchParams || body instanceof Blob)
}

/**
 * 执行统一的 HTTP 请求，并优先返回后端 JSON 包装结果。
 */
export async function requestJson<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const baseUrl = getApiBaseUrl().replace(/\/+$/, '')
  const requestPath = path.startsWith('/') ? path : `/${path}`
  const body = normalizeBody(options.body)
  const headers: Record<string, string> = {
    ...buildAuthorizationHeader(options.token),
    ...options.headers,
  }

  if (shouldUseJsonContentType(options.body) && !headers['Content-Type']) {
    headers['Content-Type'] = 'application/json'
  }

  const response = await fetch(`${baseUrl}${requestPath}`, {
    method: options.method ?? 'GET',
    headers,
    body,
    signal: options.signal,
  })

  const contentType = response.headers.get('content-type') || ''
  if (contentType.includes('application/json')) {
    return (await response.json()) as T
  }

  throw new Error(`接口未返回 JSON，状态码：${response.status}`)
}

/**
 * 创建一个自动读取令牌的轻量 API 客户端，减少页面层重复传参。
 */
export function createApiClient(getToken: () => string | null) {
  return {
    request: <T>(path: string, options: Omit<RequestOptions, 'token'> = {}) =>
      requestJson<T>(path, {
        ...options,
        token: getToken(),
      }),
    get: <T>(path: string, headers?: Record<string, string>) =>
      requestJson<T>(path, {
        method: 'GET',
        headers,
        token: getToken(),
      }),
    post: <T>(path: string, body?: BodyInit | Record<string, unknown> | null, headers?: Record<string, string>) =>
      requestJson<T>(path, {
        method: 'POST',
        body,
        headers,
        token: getToken(),
      }),
    put: <T>(path: string, body?: BodyInit | Record<string, unknown> | null, headers?: Record<string, string>) =>
      requestJson<T>(path, {
        method: 'PUT',
        body,
        headers,
        token: getToken(),
      }),
    patch: <T>(path: string, body?: BodyInit | Record<string, unknown> | null, headers?: Record<string, string>) =>
      requestJson<T>(path, {
        method: 'PATCH',
        body,
        headers,
        token: getToken(),
      }),
    delete: <T>(path: string, headers?: Record<string, string>) =>
      requestJson<T>(path, {
        method: 'DELETE',
        headers,
        token: getToken(),
      }),
  }
}
