export const LOCAL_CUBISM_CORE_PATH = '/live2d-runtime/live2dcubismcore.min.js'
export const REMOTE_CUBISM_CORE_PATH = 'https://cubism.live2d.com/sdk-web/cubismcore/live2dcubismcore.min.js'

let cubismCorePromise: Promise<string> | null = null

/**
 * 返回 Cubism Core 的候选地址，优先使用本地静态资源。
 */
const getCubismCoreCandidates = (): string[] => [LOCAL_CUBISM_CORE_PATH, REMOTE_CUBISM_CORE_PATH]

/**
 * 将运行时脚本注入页面，并等待浏览器完成加载。
 */
const appendRuntimeScript = (src: string): Promise<string> => new Promise((resolve, reject) => {
  const selector = `script[data-live2d-core="${src}"]`
  const existing = document.querySelector(selector) as HTMLScriptElement | null

  if (existing?.dataset.loaded === 'true') {
    resolve(src)
    return
  }

  const handleLoad = () => {
    if (existing) {
      existing.dataset.loaded = 'true'
    }
    resolve(src)
  }

  const handleError = () => {
    existing?.remove()
    reject(new Error(`Cubism Core 加载失败: ${src}`))
  }

  if (existing) {
    existing.addEventListener('load', handleLoad, { once: true })
    existing.addEventListener('error', handleError, { once: true })
    return
  }

  const script = document.createElement('script')
  script.src = src
  script.async = true
  script.crossOrigin = 'anonymous'
  script.dataset.live2dCore = src
  script.onload = () => {
    script.dataset.loaded = 'true'
    resolve(src)
  }
  script.onerror = () => {
    script.remove()
    reject(new Error(`Cubism Core 加载失败: ${src}`))
  }
  document.head.appendChild(script)
})

/**
 * 检查页面环境里是否已经存在 Cubism Core。
 */
const hasCubismCore = (): boolean => Boolean((window as Window & typeof globalThis & {
  Live2DCubismCore?: unknown
}).Live2DCubismCore)

/**
 * 加载 Live2D 所需的 Cubism Core，返回实际命中的脚本地址。
 */
export const loadCubismCore = async (): Promise<string> => {
  if (!process.client) {
    throw new Error('Live2D Core 只能在浏览器环境加载')
  }

  if (hasCubismCore()) {
    return 'window'
  }

  if (!cubismCorePromise) {
    cubismCorePromise = (async () => {
      let lastError: Error | null = null

      for (const candidate of getCubismCoreCandidates()) {
        try {
          const loadedFrom = await appendRuntimeScript(candidate)
          if (hasCubismCore()) {
            return loadedFrom
          }
        } catch (error) {
          lastError = error as Error
        }
      }

      throw lastError || new Error('无法加载 Cubism Core')
    })()
  }

  try {
    return await cubismCorePromise
  } catch {
    cubismCorePromise = null
    throw new Error(
      '未找到 Live2D Core，请将 live2dcubismcore.min.js 放到 frontend/public/live2d-runtime/ 目录后再重试',
    )
  }
}
