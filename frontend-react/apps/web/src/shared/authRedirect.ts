export interface LoginRedirectSearch {
  redirect?: string
}

/**
 * 规范化登录成功后的回跳地址，只允许站内相对路径，避免跳出当前站点。
 */
function normalizeLoginRedirectTarget(target: string): string {
  const normalizedTarget = target.trim()
  if (!normalizedTarget || !normalizedTarget.startsWith('/') || normalizedTarget.startsWith('//')) {
    return ''
  }

  return normalizedTarget
}

/**
 * 将 pathname 与 search 串回可复用的站内回跳地址。
 */
export function buildCurrentLocationPath(pathname: string, search = ''): string {
  const normalizedPathname = pathname.trim() || '/'
  const normalizedSearch = search.trim()
  return normalizeLoginRedirectTarget(`${normalizedPathname}${normalizedSearch}`)
}

/**
 * 读取浏览器当前地址，供按钮点击后跳登录时携带回跳信息。
 */
export function readCurrentBrowserPath(): string {
  if (typeof window === 'undefined') {
    return '/'
  }

  return buildCurrentLocationPath(window.location.pathname, window.location.search)
}

/**
 * 为登录页生成统一的回跳查询参数，缺省时省略无效字段。
 */
export function buildLoginRedirectSearch(redirectTarget: string): LoginRedirectSearch {
  const normalizedTarget = normalizeLoginRedirectTarget(redirectTarget)
  if (!normalizedTarget) {
    return {}
  }

  return {
    redirect: normalizedTarget,
  }
}

/**
 * 解析登录页中的回跳参数，并在无效时回退到默认页面。
 */
export function resolveLoginRedirectTarget(redirectTarget: unknown, fallback = '/workspace'): string {
  if (typeof redirectTarget !== 'string') {
    return fallback
  }

  return normalizeLoginRedirectTarget(redirectTarget) || fallback
}

/**
 * 判断当前是否处于必须维持登录态的受保护页面。
 */
export function isProtectedWebPath(pathname: string): boolean {
  if (pathname === '/workspace') {
    return true
  }

  if (pathname === '/practice/wrong' || pathname === '/practice/favorites' || pathname === '/practice/notes') {
    return true
  }

  if (pathname.startsWith('/practice/editor/')) {
    return true
  }

  if (pathname === '/community/create' || pathname === '/community/mine') {
    return true
  }

  if (pathname.startsWith('/community/') && pathname.endsWith('/edit')) {
    return true
  }

  if (pathname.startsWith('/interview/') && pathname !== '/interview') {
    return true
  }

  return false
}
