import { requestJson } from '@makejob/api-client'
import { isSuccessCode, type ApiEnvelope } from '@makejob/shared-types'

export type Live2DStageScene = 'companion' | 'interview'

export interface SelectableLive2DModel {
  key: string
  name: string
  scene: Live2DStageScene
  model_url: string
  thumbnail_url: string
  config_json?: string
  source: string
  match_type: string
  is_generic: boolean
  is_recommended: boolean
}

const COMPANION_SELECTED_MODEL_KEY_PREFIX = 'makejob.companion.selected-live2d:'
const INTERVIEW_SELECTED_MODEL_KEY_PREFIX = 'makejob.interview.selected-live2d:'

/**
 * 获取指定场景下可切换的 Live2D 模型列表，并保留后端返回的推荐顺序。
 */
export async function fetchSelectableLive2DModels(
  scene: Live2DStageScene,
  industryCode: string,
): Promise<SelectableLive2DModel[]> {
  const params = new URLSearchParams({
    scene,
  })

  if (industryCode.trim()) {
    params.set('industry_code', industryCode.trim())
  }

  const response = await requestJson<ApiEnvelope<SelectableLive2DModel[]>>(`/live2d/models?${params.toString()}`)
  if (!isSuccessCode(response.code) || !response.data) {
    throw new Error(response.message || '获取 Live2D 模型列表失败')
  }

  return response.data
}

/**
 * 读取指定场景和行业下最近一次手动切换的模型键，便于刷新后恢复选择。
 */
export function readSelectedLive2DModelKey(scene: Live2DStageScene, industryCode: string): string {
  if (typeof window === 'undefined') {
    return ''
  }

  return window.localStorage.getItem(buildLive2DSelectedModelStorageKey(scene, industryCode)) || ''
}

/**
 * 记录当前场景和行业下的模型选择结果，供后续页面重进时直接恢复。
 */
export function persistSelectedLive2DModelKey(
  scene: Live2DStageScene,
  industryCode: string,
  modelKey: string,
): void {
  if (typeof window === 'undefined') {
    return
  }

  window.localStorage.setItem(buildLive2DSelectedModelStorageKey(scene, industryCode), modelKey)
}

/**
 * 构造指定场景的模型本地缓存键，兼容当前陪伴页和面试页各自已存在的命名方式。
 */
export function buildLive2DSelectedModelStorageKey(scene: Live2DStageScene, industryCode: string): string {
  const normalizedIndustryCode = industryCode.trim() || 'default'
  if (scene === 'interview') {
    return `${INTERVIEW_SELECTED_MODEL_KEY_PREFIX}${normalizedIndustryCode}`
  }

  return `${COMPANION_SELECTED_MODEL_KEY_PREFIX}${normalizedIndustryCode}`
}

/**
 * 将模型来源编码转换成更直观的中文说明，方便舞台顶部状态文案直接复用。
 */
export function live2DSourceLabel(source: string): string {
  if (source === 'database') {
    return '后台模型'
  }

  if (source === 'bundled') {
    return '内置模型'
  }

  return source || '未知来源'
}

/**
 * 将模型命中类型转换成前台可读标签，帮助用户理解当前模型为什么出现在列表里。
 */
export function live2DMatchTypeLabel(matchType: string): string {
  switch (matchType) {
    case 'industry':
      return '行业推荐'
    case 'generic':
      return '通用模型'
    case 'other':
      return '其他可选'
    case 'bundled':
      return '内置模型'
    default:
      return '可用模型'
  }
}

/**
 * 解析模型配置 JSON，失败时直接回退为空对象，避免单条异常配置影响整个舞台。
 */
export function parseSelectableLive2DModelConfig(model: SelectableLive2DModel | null): Record<string, unknown> {
  const rawConfig = model?.config_json?.trim()
  if (!rawConfig) {
    return {}
  }

  try {
    const parsed = JSON.parse(rawConfig) as unknown
    if (!parsed || Array.isArray(parsed) || typeof parsed !== 'object') {
      return {}
    }

    return parsed as Record<string, unknown>
  } catch {
    return {}
  }
}

/**
 * 提取当前模型配置中的背景图地址，供共享舞台在有值时渲染背景层。
 */
export function resolveSelectableLive2DBackgroundImageUrl(model: SelectableLive2DModel | null): string {
  const value = parseSelectableLive2DModelConfig(model).background_image_url
  return typeof value === 'string' ? value.trim() : ''
}
