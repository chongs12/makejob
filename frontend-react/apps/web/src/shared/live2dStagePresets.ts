import type {
  Live2DDiscoveredExpression,
  Live2DDiscoveredMotion,
  Live2DStageExpressionLayer,
  Live2DStageModelMetadata,
  Live2DStageParameterOverride,
} from './live2dStageRuntime'
import type { Live2DStageScene } from './live2dModelCatalog'
import type { Live2DDirective } from './live2dDirective'

export interface Live2DStagePresetInput {
  scene: Live2DStageScene
  emotion: string
  action?: string
  mouthOpen?: number
  directive?: Live2DDirective | null
}

export interface ResolvedLive2DStagePreset {
  expressionMix: Live2DStageExpressionLayer[]
  parameterOverrides: Live2DStageParameterOverride[]
  activeExpressionLabels: string[]
  activeMotionLabel: string
  motion: {
    key: string
    group: string
    priority: 'normal' | 'force'
    durationMS: number
  } | null
}

/**
 * 根据当前页面传入的粗粒度状态，推导舞台最终要应用的表情层和参数覆盖值。
 */
export function resolveLive2DStagePreset(
  metadata: Live2DStageModelMetadata | null,
  preset: Live2DStagePresetInput,
): ResolvedLive2DStagePreset {
  const directivePreset = resolveDirectivePreset(metadata, preset)
  if (directivePreset) {
    return directivePreset
  }

  const expressions = metadata?.expressions || []
  const usedExpressionIds = new Set<string>()
  const expressionMix: Live2DStageExpressionLayer[] = []
  const activeExpressionLabels: string[] = []

  const emotionExpression = pickExpressionForEmotion(expressions, preset.emotion, usedExpressionIds)
  if (emotionExpression) {
    expressionMix.push({
      key: emotionExpression.id,
      weight: 1,
    })
    activeExpressionLabels.push(emotionExpression.label)
  }

  const actionExpression = pickExpressionForAction(expressions, preset.action || '', usedExpressionIds)
  if (actionExpression) {
    expressionMix.push({
      key: actionExpression.id,
      weight: 1,
    })
    activeExpressionLabels.push(actionExpression.label)
  }

  const parameterOverrides = buildStageParameterOverrides(preset)
  return {
    expressionMix,
    parameterOverrides,
    activeExpressionLabels,
    activeMotionLabel: '',
    motion: null,
  }
}

/**
 * 当后端已直接返回结构化指令时，优先使用后端结果，旧的情绪动作映射只作为回退。
 */
function resolveDirectivePreset(
  metadata: Live2DStageModelMetadata | null,
  preset: Live2DStagePresetInput,
): ResolvedLive2DStagePreset | null {
  const directive = preset.directive
  if (!directive) {
    return null
  }

  const expressionMap = new Map((metadata?.expressions || []).map((item) => [item.id, item]))
  const motionMap = new Map((metadata?.motions || []).map((item) => [item.key, item]))
  const expressionMix = (directive.expression_mix || [])
    .filter((item) => item && typeof item.key === 'string' && expressionMap.has(item.key))
    .slice(0, 3)
    .map((item) => ({
      key: item.key,
      weight: clampStageParameter(item.weight, 0, 1),
    }))
  const activeExpressionLabels = expressionMix.map((item) => expressionMap.get(item.key)?.label || item.key)

  const parameterOverrides = [...(directive.parameter_overrides || [])]
  if (typeof directive.mouth_open === 'number') {
    parameterOverrides.push({
      id: 'ParamMouthOpenY',
      value: clampStageParameter(directive.mouth_open, 0, 1),
    })
  } else if (typeof preset.mouthOpen === 'number') {
    parameterOverrides.push({
      id: 'ParamMouthOpenY',
      value: clampStageParameter(preset.mouthOpen, 0, 1),
    })
  }

  let motion: ResolvedLive2DStagePreset['motion'] = null
  let activeMotionLabel = ''
  if (typeof directive.motion_key === 'string' && motionMap.has(directive.motion_key)) {
    const discoveredMotion = motionMap.get(directive.motion_key) as Live2DDiscoveredMotion
    motion = {
      key: discoveredMotion.key,
      group: discoveredMotion.group,
      priority: directive.motion_priority === 'force' ? 'force' : 'normal',
      durationMS: typeof directive.motion_duration_ms === 'number'
        ? Math.max(0, Math.min(directive.motion_duration_ms, 12000))
        : 0,
    }
    activeMotionLabel = discoveredMotion.label
  }

  return {
    expressionMix,
    parameterOverrides: parameterOverrides.map((item) => ({
      id: item.id,
      value: item.value,
    })),
    activeExpressionLabels,
    activeMotionLabel,
    motion,
  }
}

/**
 * 根据陪伴页或面试页的情绪值，尝试从当前模型表达式列表里匹配最像的文件表达式。
 */
function pickExpressionForEmotion(
  expressions: Live2DDiscoveredExpression[],
  emotion: string,
  usedExpressionIds: Set<string>,
): Live2DDiscoveredExpression | null {
  const normalizedEmotion = emotion.trim().toLowerCase()
  switch (normalizedEmotion) {
    case 'happy':
    case 'encouraging':
    case 'encourage':
    case 'praise':
      return findBestExpression(expressions, usedExpressionIds, ['爱心', '星星', '笑', 'wink', 'hah', 'happy', 'smile'])
    case 'thinking':
    case 'confused':
      return findBestExpression(expressions, usedExpressionIds, ['圈圈', '白眼', 'haoqi', 'thinking', 'question', 'look'])
    case 'warning':
    case 'serious':
      return findBestExpression(expressions, usedExpressionIds, ['黑', '嫌弃', 'angry', 'black', 'shock', '严肃'])
    case 'sad':
    case 'tired':
      return findBestExpression(expressions, usedExpressionIds, ['泪', 'tear', 'sad', '哭', '流汗'])
    default:
      return null
  }
}

/**
 * 根据动作标签补一层更像姿态或道具效果的表达式，优先选择抬手、挥手或表演类文件。
 */
function pickExpressionForAction(
  expressions: Live2DDiscoveredExpression[],
  action: string,
  usedExpressionIds: Set<string>,
): Live2DDiscoveredExpression | null {
  const normalizedAction = action.trim().toLowerCase()
  switch (normalizedAction) {
    case 'wave':
      return findBestExpression(expressions, usedExpressionIds, ['wave', '抬手', '挥', '手', '拿话筒'])
    case 'celebrate':
      return findBestExpression(expressions, usedExpressionIds, ['星星', '爱心', 'wave', 'wink', '抬手'])
    case 'thinking':
      return findBestExpression(expressions, usedExpressionIds, ['俯身', '思', 'haoqi', 'thinking'])
    default:
      return null
  }
}

/**
 * 在表达式列表中挑选和关键词最接近的一项，并避免与已经使用过的表达式重复。
 */
function findBestExpression(
  expressions: Live2DDiscoveredExpression[],
  usedExpressionIds: Set<string>,
  keywords: string[],
): Live2DDiscoveredExpression | null {
  let bestExpression: Live2DDiscoveredExpression | null = null
  let bestScore = 0

  for (const expression of expressions) {
    if (usedExpressionIds.has(expression.id)) {
      continue
    }

    const haystack = `${expression.id} ${expression.label}`.toLowerCase()
    const score = keywords.reduce((total, keyword) => {
      return haystack.includes(keyword.toLowerCase()) ? total + 1 : total
    }, 0)
    if (score <= bestScore) {
      continue
    }

    bestExpression = expression
    bestScore = score
  }

  if (!bestExpression) {
    return null
  }

  usedExpressionIds.add(bestExpression.id)
  return bestExpression
}

/**
 * 将现有业务场景的情绪和动作，收敛成一组可平滑驱动的通用 Cubism 参数。
 */
function buildStageParameterOverrides(preset: Live2DStagePresetInput): Live2DStageParameterOverride[] {
  const parameters = new Map<string, number>()
  const normalizedEmotion = preset.emotion.trim().toLowerCase()
  const normalizedAction = (preset.action || '').trim().toLowerCase()

  setStageParameter(parameters, 'ParamEyeLOpen', 1)
  setStageParameter(parameters, 'ParamEyeROpen', 1)
  setStageParameter(parameters, 'ParamMouthForm', 0)
  setStageParameter(parameters, 'ParamBrowLY', 0)
  setStageParameter(parameters, 'ParamBrowRY', 0)
  setStageParameter(parameters, 'ParamAngleX', 0)
  setStageParameter(parameters, 'ParamAngleY', 0)
  setStageParameter(parameters, 'ParamAngleZ', 0)
  setStageParameter(parameters, 'ParamBodyAngleX', 0)

  switch (normalizedEmotion) {
    case 'happy':
    case 'praise':
      setStageParameter(parameters, 'ParamMouthForm', 0.72)
      setStageParameter(parameters, 'ParamCheek', 0.28)
      break
    case 'encouraging':
    case 'encourage':
      setStageParameter(parameters, 'ParamMouthForm', 0.34)
      setStageParameter(parameters, 'ParamEyeLOpen', 0.94)
      setStageParameter(parameters, 'ParamEyeROpen', 0.94)
      break
    case 'thinking':
      setStageParameter(parameters, 'ParamBrowLY', 0.22)
      setStageParameter(parameters, 'ParamBrowRY', -0.12)
      setStageParameter(parameters, 'ParamAngleX', 8)
      setStageParameter(parameters, 'ParamBodyAngleX', 3.5)
      setStageParameter(parameters, 'ParamMouthForm', -0.16)
      break
    case 'warning':
    case 'serious':
      setStageParameter(parameters, 'ParamBrowLY', -0.42)
      setStageParameter(parameters, 'ParamBrowRY', -0.42)
      setStageParameter(parameters, 'ParamMouthForm', -0.26)
      break
    case 'sad':
    case 'tired':
      setStageParameter(parameters, 'ParamEyeLOpen', 0.88)
      setStageParameter(parameters, 'ParamEyeROpen', 0.88)
      setStageParameter(parameters, 'ParamMouthForm', -0.12)
      break
    default:
      break
  }

  switch (normalizedAction) {
    case 'wave':
      setStageParameter(parameters, 'ParamAngleZ', 6)
      setStageParameter(parameters, 'ParamBodyAngleX', 4)
      break
    case 'nod':
      setStageParameter(parameters, 'ParamAngleY', 5)
      break
    case 'celebrate':
      setStageParameter(parameters, 'ParamAngleZ', -4)
      setStageParameter(parameters, 'ParamBodyAngleX', 5)
      setStageParameter(parameters, 'ParamCheek', 0.36)
      break
    default:
      break
  }

  if (preset.scene === 'interview') {
    setStageParameter(parameters, 'ParamBodyAngleX', clampStageParameter(parameters.get('ParamBodyAngleX') || 0, -6, 6))
    setStageParameter(parameters, 'ParamAngleZ', clampStageParameter(parameters.get('ParamAngleZ') || 0, -8, 8))
  }

  if (typeof preset.mouthOpen === 'number') {
    setStageParameter(parameters, 'ParamMouthOpenY', clampStageParameter(preset.mouthOpen, 0, 1))
  }

  return [...parameters.entries()].map(([id, value]) => ({
    id,
    value,
  }))
}

/**
 * 安全写入一条参数覆盖值，后续统一转成数组交给舞台运行时。
 */
function setStageParameter(parameters: Map<string, number>, id: string, value: number): void {
  parameters.set(id, value)
}

/**
 * 约束参数范围，避免嘴型和姿态类参数被意外写出明显失真值。
 */
function clampStageParameter(value: number, min: number, max: number): number {
  return Math.min(Math.max(value, min), max)
}
