declare global {
  interface Window {
    PIXI?: typeof import('pixi.js')
    Live2DCubismCore?: unknown
  }
}

let pixiRuntimePromise: Promise<typeof import('pixi.js')> | null = null
let cubismCoreScriptPromise: Promise<void> | null = null
let cubismModulePromise: Promise<typeof import('pixi-live2d-display/cubism4')> | null = null

/**
 * 预热 Pixi 运行时并挂到 window，满足后续 Cubism 模块初始化依赖。
 */
async function loadPixiRuntime(): Promise<typeof import('pixi.js')> {
  if (!pixiRuntimePromise) {
    pixiRuntimePromise = import('pixi.js').then((PIXI) => {
      if (typeof window !== 'undefined') {
        window.PIXI = PIXI
      }
      return PIXI
    })
  }

  return pixiRuntimePromise
}

/**
 * 动态加载 Cubism Core 脚本，确保浏览器端具备解析 Cubism4 模型的能力。
 */
async function ensureCubismCoreScript(): Promise<void> {
  if (typeof window === 'undefined') {
    return
  }

  if (window.Live2DCubismCore) {
    return
  }

  if (!cubismCoreScriptPromise) {
    cubismCoreScriptPromise = new Promise<void>((resolve, reject) => {
      const existingScript = document.querySelector<HTMLScriptElement>('script[data-live2d-cubism-core="true"]')
      if (existingScript) {
        if (window.Live2DCubismCore) {
          resolve()
          return
        }

        existingScript.addEventListener('load', () => resolve(), { once: true })
        existingScript.addEventListener('error', () => reject(new Error('Cubism Core 脚本加载失败')), { once: true })
        return
      }

      const script = document.createElement('script')
      script.src = '/live2d-assets/live2dcubismcore.min.js'
      script.async = true
      script.dataset.live2dCubismCore = 'true'
      script.onload = () => {
        if (!window.Live2DCubismCore) {
          reject(new Error('Cubism Core 已返回，但页面中未挂载 Live2DCubismCore'))
          return
        }

        resolve()
      }
      script.onerror = () => reject(new Error('Cubism Core 脚本加载失败'))
      document.head.appendChild(script)
    })
  }

  return cubismCoreScriptPromise
}

/**
 * 加载 Cubism Live2D 模块，并复用前面已经建立的 Pixi 与 Core 缓存。
 */
async function loadCubismModule(): Promise<typeof import('pixi-live2d-display/cubism4')> {
  if (!cubismModulePromise) {
    cubismModulePromise = Promise.all([
      loadPixiRuntime(),
      ensureCubismCoreScript(),
    ]).then(() => import('pixi-live2d-display/cubism4'))
  }

  return cubismModulePromise
}

/**
 * 统一返回 Live2D 舞台初始化所需运行时，避免多个页面重复串行加载。
 */
export async function loadLive2DRuntime(): Promise<{
  PIXI: typeof import('pixi.js')
  Live2DModel: typeof import('pixi-live2d-display/cubism4').Live2DModel
}> {
  const [PIXI, cubismModule] = await Promise.all([
    loadPixiRuntime(),
    loadCubismModule(),
  ])

  return {
    PIXI,
    Live2DModel: cubismModule.Live2DModel,
  }
}

/**
 * 提前启动 Live2D 运行时加载链，减少真正挂载舞台时的首次等待。
 */
export function prewarmLive2DRuntime(): void {
  void loadPixiRuntime()
  void ensureCubismCoreScript()
  void loadCubismModule()
}
