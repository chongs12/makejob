import type { ComputedRef, Ref } from 'vue'
import type { ApiResponse } from '~/composables/useApi'
import type { Live2DModelConfig } from '~/components/live2d/types'

/* Live2DCurrentModelResponse 描述后端返回的当前模型结构。 */
interface Live2DCurrentModelResponse {
  name: string
  scene: string
  industry_code: string
  path: string
  model_url: string
  thumbnail_url: string
  config: Record<string, unknown>
  source: 'database' | 'bundled'
}

type MaybeModelRef = Ref<string | undefined> | ComputedRef<string | undefined> | string | undefined

/* 判断统一 API 响应是否成功。*/
const isSuccessResponse = (code: number | undefined): boolean => code === 0 || code === 200

/* resolveModelInput 统一读取普通值或响应式值。 */
const resolveModelInput = (value: MaybeModelRef): string => {
  if (typeof value === 'string') {
    return value
  }
  return value?.value || ''
}

/* resolveBackendBase 推导后端静态资源的基准地址。 */
const resolveBackendBase = (apiBase: string): string => {
  if (!apiBase) {
    return ''
  }

  if (process.client) {
    const parsed = new URL(apiBase, window.location.origin)
    parsed.pathname = parsed.pathname.replace(/\/api\/?$/, '/')
    return parsed.toString().replace(/\/$/, '')
  }

  return apiBase.replace(/\/api\/?$/, '')
}

/* resolveAssetUrl 将后端返回的相对资源地址转换为前端可访问地址。 */
const resolveAssetUrl = (backendBase: string, assetPath: string): string => {
  const target = assetPath.trim()
  if (!target) {
    return ''
  }
  if (/^https?:\/\//i.test(target)) {
    return target
  }
  if (!backendBase) {
    return target
  }
  if (!target.startsWith('/')) {
    return `${backendBase}/${target}`.replace(/([^:]\/)\/+/g, '$1')
  }
  return `${backendBase}${target}`
}

/* mapLive2DModelConfig 将接口响应映射为组件侧配置。 */
const mapLive2DModelConfig = (payload: Live2DCurrentModelResponse, backendBase: string): Live2DModelConfig => {
  const offsetX = Number(payload.config?.offset_x ?? 0)
  const offsetY = Number(payload.config?.offset_y ?? 0)
  const scale = Number(payload.config?.scale ?? 1)

  return {
    name: payload.name,
    path: resolveAssetUrl(backendBase, payload.path),
    modelUrl: resolveAssetUrl(backendBase, payload.model_url),
    thumbnailUrl: resolveAssetUrl(backendBase, payload.thumbnail_url),
    scene: payload.scene,
    industryCode: payload.industry_code,
    source: payload.source,
    config: payload.config || {},
    scale,
    position: { x: offsetX, y: offsetY },
  }
}

/* useLive2DModel 加载指定场景的当前 Live2D 模型配置。 */
export const useLive2DModel = (scene: MaybeModelRef, industryCode?: MaybeModelRef) => {
  const { $api } = useNuxtApp()
  const config = useRuntimeConfig()

  const modelConfig = ref<Live2DModelConfig | null>(null)
  const loading = ref(false)
  const error = ref('')

  /* fetchModel 拉取当前场景的 Live2D 模型。 */
  const fetchModel = async () => {
    const currentScene = resolveModelInput(scene).trim()
    if (!currentScene) {
      modelConfig.value = null
      error.value = '缺少 Live2D 场景参数'
      return
    }

    loading.value = true
    error.value = ''

    try {
      const response = await $api.get<Live2DCurrentModelResponse>('/live2d/current', {
        scene: currentScene,
        industry_code: resolveModelInput(industryCode).trim() || undefined,
      }) as ApiResponse<Live2DCurrentModelResponse>

      if (isSuccessResponse(response.code) && response.data) {
        modelConfig.value = mapLive2DModelConfig(response.data, resolveBackendBase(config.public.apiBase))
      } else {
        modelConfig.value = null
        error.value = response.message || 'Live2D 模型加载失败'
      }
    } catch (err: any) {
      modelConfig.value = null
      error.value = err?.data?.message || err?.message || 'Live2D 模型加载失败'
    } finally {
      loading.value = false
    }
  }

  if (process.client) {
    watch(
      () => [resolveModelInput(scene), resolveModelInput(industryCode)],
      () => {
        fetchModel()
      },
      { immediate: true },
    )
  }

  return {
    modelConfig,
    loading,
    error,
    refresh: fetchModel,
  }
}
