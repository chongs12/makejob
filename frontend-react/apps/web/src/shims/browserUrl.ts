interface BrowserParsedUrl {
  href: string
  protocol: string
  host: string
  hostname: string
  port: string
  pathname: string
  path: string
  search: string
  query: string
  hash: string
  origin: string
  slashes: boolean
}

/**
 * 生成浏览器环境下用于 URL 解析的基准地址，避免依赖 Node 的 url/punycode 链路。
 */
function resolveBaseHref(): string {
  if (typeof window !== 'undefined' && window.location?.href) {
    return window.location.href
  }

  return 'http://localhost/'
}

/**
 * 将浏览器原生 URL 对象转换为兼容 Node url.parse 关键字段的结果结构。
 */
function buildParsedUrl(url: URL): BrowserParsedUrl {
  return {
    href: url.href,
    protocol: url.protocol,
    host: url.host,
    hostname: url.hostname,
    port: url.port,
    pathname: url.pathname,
    path: `${url.pathname}${url.search}`,
    search: url.search,
    query: url.search.startsWith('?') ? url.search.slice(1) : url.search,
    hash: url.hash,
    origin: url.origin,
    slashes: url.href.includes('//'),
  }
}

/**
 * 在原生 URL 无法解析时返回最小可用结构，确保调用方至少拿到稳定字符串。
 */
function buildFallbackParsedUrl(input: string): BrowserParsedUrl {
  return {
    href: input,
    protocol: '',
    host: '',
    hostname: '',
    port: '',
    pathname: input,
    path: input,
    search: '',
    query: '',
    hash: '',
    origin: '',
    slashes: input.includes('//'),
  }
}

/**
 * 统一补齐查询串或 hash 的前缀，兼容 format 时传入裸值的情况。
 */
function withPrefix(value: string, prefix: '?' | '#'): string {
  if (!value) {
    return ''
  }

  return value.startsWith(prefix) ? value : `${prefix}${value}`
}

/**
 * 使用浏览器原生 URL 实现 parse，替代浏览器端不稳定的 Node url 包。
 */
export function parse(input: string): BrowserParsedUrl {
  const normalizedInput = String(input || '')

  try {
    return buildParsedUrl(new URL(normalizedInput, resolveBaseHref()))
  } catch {
    return buildFallbackParsedUrl(normalizedInput)
  }
}

/**
 * 使用浏览器原生 URL 组合相对路径，满足 Pixi/Live2D 对 url.resolve 的核心诉求。
 */
export function resolve(from: string, to: string): string {
  try {
    return new URL(String(to || ''), parse(from).href || resolveBaseHref()).toString()
  } catch {
    return String(to || '')
  }
}

/**
 * 根据 parse 后的字段重新拼接 URL 字符串，补齐 @pixi/utils 对 url.format 的依赖。
 */
export function format(value: Partial<BrowserParsedUrl> | string): string {
  if (typeof value === 'string') {
    return value
  }

  if (value.href) {
    return value.href
  }

  const protocol = value.protocol || ''
  const host = value.host || [value.hostname || '', value.port || ''].filter(Boolean).join(':')
  const pathname = value.pathname || ''
  const search = withPrefix(value.search || value.query || '', '?')
  const hash = withPrefix(value.hash || '', '#')

  if (host) {
    const normalizedProtocol = protocol ? (protocol.endsWith(':') ? protocol : `${protocol}:`) : ''
    return `${normalizedProtocol}//${host}${pathname}${search}${hash}`
  }

  return `${pathname}${search}${hash}`
}
