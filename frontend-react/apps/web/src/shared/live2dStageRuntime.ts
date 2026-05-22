import type { Application } from 'pixi.js'
import type { Live2DModel as Cubism4Live2DModel } from 'pixi-live2d-display/cubism4'
import type { SelectableLive2DModelMotion } from './live2dModelCatalog'
import { loadLive2DRuntime } from './live2dRuntime'

export interface Live2DStageTransform {
  scale: number
  offsetX: number
  offsetY: number
}

export interface Live2DStageExpressionLayer {
  key: string
  weight: number
}

export interface Live2DStageParameterOverride {
  id: string
  value: number
}

export interface Live2DDiscoveredExpression {
  id: string
  label: string
  file: string
}

export interface Live2DDiscoveredMotion {
  key: string
  group: string
  runtimeGroup: string
  index: number
  label: string
  file: string
}

export interface Live2DStageModelMetadata {
  expressions: Live2DDiscoveredExpression[]
  motions: Live2DDiscoveredMotion[]
  parameterIds: string[]
}

export interface Live2DStageVisualState {
  expressionMix: Live2DStageExpressionLayer[]
  parameterOverrides: Live2DStageParameterOverride[]
  motion: {
    key: string
    group: string
    priority: 'normal' | 'force'
    durationMS: number
  } | null
}

export interface Live2DStageRuntime {
  app: Application
  model: Cubism4Live2DModel
  motionPriority: typeof import('pixi-live2d-display/cubism4').MotionPriority
  metadata: Live2DStageModelMetadata
  baseWidth: number
  baseHeight: number
  defaultTransform: Live2DStageTransform
  currentTransform: Live2DStageTransform
  baselineParams: Map<string, number>
  overlayCurrentParams: Map<string, number>
  overlayTargetParams: Map<string, number>
  trackedParamIds: Set<string>
  expressionFileCache: Map<string, Live2DExpressionFilePayload>
  currentMotionKey: string
  lastMotionStartedAt: number
}

interface Live2DModelSettingsPayload {
  url?: string
  FileReferences?: {
    DisplayInfo?: string
    Expressions?: Array<{
      Name?: string
      File?: string
    }>
    Motions?: Record<string, Array<{
      File?: string
    }>>
  }
}

interface Live2DDisplayInfoPayload {
  Parameters?: Array<{
    Id?: string
  }>
}

interface Live2DVtubePayload {
  ParameterSettings?: Array<{
    OutputLive2D?: string
  }>
}

interface Live2DExpressionFilePayload {
  Parameters?: Array<{
    Id?: string
    Value?: number
    Blend?: 'Add' | 'Multiply' | 'Overwrite' | string
  }>
}

interface Live2DCoreModelLike {
  getParameterValueById?: (parameterId: string) => number
  setParameterValueById?: (parameterId: string, value: number, weight?: number) => void
}

/**
 * 创建一套可复用的 Live2D 舞台运行时，并提前发现当前模型可用的表达式和参数元数据。
 */
export async function createLive2DStageRuntime(
  host: HTMLDivElement,
  modelUrl: string,
  defaultTransform: Live2DStageTransform,
  serverMotions: SelectableLive2DModelMotion[] = [],
): Promise<Live2DStageRuntime> {
  const { PIXI, Live2DModel, MotionPriority } = await loadLive2DRuntime()
  const modelSettings = await fetchJson<Live2DModelSettingsPayload>(modelUrl)
  const hydratedModelSettings = hydrateLive2DModelSettings(modelUrl, modelSettings, serverMotions)
  const metadata = await discoverLive2DModelMetadata(modelUrl, hydratedModelSettings)

  const app = new PIXI.Application({
    width: Math.max(host.clientWidth, 320),
    height: Math.max(host.clientHeight, 320),
    autoStart: true,
    backgroundAlpha: 0,
    antialias: true,
    autoDensity: true,
    resolution: Math.min(window.devicePixelRatio || 1, 2),
  })

  const canvas = app.view as HTMLCanvasElement
  canvas.style.width = '100%'
  canvas.style.height = '100%'
  canvas.style.display = 'block'
  host.replaceChildren(canvas)

  const model = await Live2DModel.from(hydratedModelSettings)
  app.stage.addChild(model)

  const runtime: Live2DStageRuntime = {
    app,
    model,
    motionPriority: MotionPriority,
    metadata: {
      expressions: mergeModelExpressions(modelUrl, hydratedModelSettings, metadata.expressions),
      motions: mergeModelMotions(modelUrl, hydratedModelSettings, metadata.motions),
      parameterIds: metadata.parameterIds,
    },
    baseWidth: Math.max(model.width, 1),
    baseHeight: Math.max(model.height, 1),
    defaultTransform: { ...defaultTransform },
    currentTransform: { ...defaultTransform },
    baselineParams: new Map<string, number>(),
    overlayCurrentParams: new Map<string, number>(),
    overlayTargetParams: new Map<string, number>(),
    trackedParamIds: new Set<string>(),
    expressionFileCache: new Map<string, Live2DExpressionFilePayload>(),
    currentMotionKey: '',
    lastMotionStartedAt: 0,
  }

  app.ticker.add(() => {
    applyRuntimeOverlayFrame(runtime)
  })

  updateLive2DStageTransform(runtime, host, defaultTransform)
  focusLive2DStage(runtime, host.clientWidth * 0.5, host.clientHeight * 0.58, true)
  return runtime
}

/**
 * 销毁当前 Live2D 运行时实例，避免页面切换或模型切换后残留 Pixi 资源。
 */
export function destroyLive2DStageRuntime(runtime: Live2DStageRuntime | null): void {
  if (!runtime) {
    return
  }

  runtime.app.destroy(true, { children: true, texture: false, baseTexture: false })
}

/**
 * 根据容器最新尺寸和目标变换值，重新计算模型在舞台中的缩放和站位。
 */
export function updateLive2DStageTransform(
  runtime: Live2DStageRuntime,
  host: HTMLDivElement,
  transform: Live2DStageTransform,
): void {
  runtime.app.renderer.resize(Math.max(host.clientWidth, 320), Math.max(host.clientHeight, 320))

  const safeBaseWidth = Math.max(runtime.baseWidth, 1)
  const safeBaseHeight = Math.max(runtime.baseHeight, 1)
  const widthScale = (host.clientWidth * 0.82) / safeBaseWidth
  const heightScale = (host.clientHeight * 1.04) / safeBaseHeight
  const fitScale = Math.max(Math.min(widthScale, heightScale), 0.1)
  const stageScale = fitScale * transform.scale

  runtime.model.scale.set(stageScale)
  runtime.model.anchor.set(0.5, 1)
  runtime.model.position.set(
    host.clientWidth * (0.5 + transform.offsetX),
    host.clientHeight * (0.94 + transform.offsetY),
  )
  runtime.currentTransform = { ...transform }
}

/**
 * 将当前舞台视觉状态应用到模型参数目标值中，后续由 ticker 逐帧平滑逼近。
 */
export async function applyLive2DStageVisualState(
  runtime: Live2DStageRuntime,
  visualState: Live2DStageVisualState,
): Promise<void> {
  const nextTargets = new Map<string, number>()

  for (const layer of visualState.expressionMix) {
    await mergeExpressionLayerIntoTargets(runtime, nextTargets, layer)
  }

  for (const parameterOverride of visualState.parameterOverrides) {
    ensureTrackedParameter(runtime, parameterOverride.id)
    nextTargets.set(parameterOverride.id, parameterOverride.value)
  }

  runtime.overlayTargetParams = nextTargets
  await playLive2DStageMotion(runtime, visualState.motion)
}

/**
 * 让模型朝向舞台中的指定像素坐标，保持与鼠标或默认视线中心同步。
 */
export function focusLive2DStage(
  runtime: Live2DStageRuntime,
  targetX: number,
  targetY: number,
  instant = false,
): void {
  runtime.model.focus(targetX, targetY, instant)
}

/**
 * 将模型视线缓慢恢复到舞台默认中心，避免鼠标离开后角色仍停留在边角。
 */
export function resetLive2DStageFocus(runtime: Live2DStageRuntime, host: HTMLDivElement): void {
  focusLive2DStage(runtime, host.clientWidth * 0.5, host.clientHeight * 0.58)
}

/**
 * 拉取当前模型的表达式和参数元数据，为后续匹配和控制抽屉提供真实依据。
 */
async function discoverLive2DModelMetadata(
  modelUrl: string,
  modelSettings: Live2DModelSettingsPayload,
): Promise<Live2DStageModelMetadata> {
  const expressions = extractModelExpressions(modelUrl, modelSettings)
  const motions = extractModelMotions(modelUrl, modelSettings)
  const [displayInfo, vtubeInfo] = await Promise.all([
    resolveDisplayInfoPayload(modelUrl, modelSettings),
    resolveVtubePayload(modelUrl),
  ])

  const parameterIds = new Set<string>()
  for (const parameter of displayInfo?.Parameters || []) {
    if (parameter.Id?.trim()) {
      parameterIds.add(parameter.Id.trim())
    }
  }
  for (const parameter of vtubeInfo?.ParameterSettings || []) {
    if (parameter.OutputLive2D?.trim()) {
      parameterIds.add(parameter.OutputLive2D.trim())
    }
  }

  return {
    expressions,
    motions,
    parameterIds: [...parameterIds],
  }
}

/**
 * 从模型自身声明的表达式列表中提取可用文件，并转换成前端更容易匹配的标签结构。
 */
function extractModelExpressions(
  modelUrl: string,
  modelSettings: Live2DModelSettingsPayload,
): Live2DDiscoveredExpression[] {
  const result: Live2DDiscoveredExpression[] = []
  const seen = new Set<string>()

  for (const expression of modelSettings.FileReferences?.Expressions || []) {
    if (!expression.File?.trim()) {
      continue
    }

    const rawName = expression.Name?.trim() || expression.File.split('/').pop() || 'expression'
    const id = normalizeLive2DExpressionId(rawName)
    if (!id || seen.has(id)) {
      continue
    }

    seen.add(id)
    result.push({
      id,
      label: formatLive2DExpressionLabel(rawName),
      file: resolveRelativeLive2DAssetUrl(modelUrl, expression.File),
    })
  }

  return result
}

/**
 * 合并模型运行前后两次发现结果，优先保留模型原始引用顺序，并补上缺失文件。
 */
function mergeModelExpressions(
  modelUrl: string,
  modelSettings: Live2DModelSettingsPayload,
  discoveredExpressions: Live2DDiscoveredExpression[],
): Live2DDiscoveredExpression[] {
  const baseExpressions = extractModelExpressions(modelUrl, modelSettings)
  if (baseExpressions.length === 0) {
    return discoveredExpressions
  }

  const merged = [...baseExpressions]
  const seen = new Set(merged.map((item) => item.id))
  for (const expression of discoveredExpressions) {
    if (seen.has(expression.id)) {
      continue
    }
    seen.add(expression.id)
    merged.push(expression)
  }

  return merged
}

/**
 * 从模型 settings 里提取可播放动作列表，并为后续指令执行建立稳定 key 到 group/index 的映射。
 */
function extractModelMotions(
  modelUrl: string,
  modelSettings: Live2DModelSettingsPayload,
): Live2DDiscoveredMotion[] {
  const motions: Live2DDiscoveredMotion[] = []
  const seenKeys = new Set<string>()
  const groups = modelSettings.FileReferences?.Motions || {}

  for (const group of Object.keys(groups).sort()) {
    const definitions = groups[group] || []
    definitions.forEach((motion, index) => {
      if (!motion.File?.trim()) {
        return
      }

      const rawName = motion.File.split('/').pop() || `motion_${index + 1}`
      const key = normalizeLive2DMotionKey(group, rawName, index)
      if (!key || seenKeys.has(key)) {
        return
      }

      seenKeys.add(key)
      motions.push({
        key,
        group: normalizeLive2DMotionToken(group) || 'auto',
        runtimeGroup: group,
        index,
        label: formatLive2DMotionLabel(rawName),
        file: resolveRelativeLive2DAssetUrl(modelUrl, motion.File),
      })
    })
  }

  return motions
}

/**
 * 合并模型原始 settings 与运行前发现到的动作数据，优先保留 settings 中的稳定顺序。
 */
function mergeModelMotions(
  modelUrl: string,
  modelSettings: Live2DModelSettingsPayload,
  discoveredMotions: Live2DDiscoveredMotion[],
): Live2DDiscoveredMotion[] {
  const baseMotions = extractModelMotions(modelUrl, modelSettings)
  if (baseMotions.length === 0) {
    return discoveredMotions
  }

  const merged = [...baseMotions]
  const seen = new Set(merged.map((item) => item.key))
  for (const motion of discoveredMotions) {
    if (seen.has(motion.key)) {
      continue
    }
    seen.add(motion.key)
    merged.push(motion)
  }
  return merged
}

/**
 * 按指令触发一条动作播放，重复动作会在短时间内节流，避免回复频繁到来时出现抖动重播。
 */
async function playLive2DStageMotion(
  runtime: Live2DStageRuntime,
  motion: Live2DStageVisualState['motion'],
): Promise<void> {
  if (!motion) {
    return
  }

  const discoveredMotion = runtime.metadata.motions.find((item) => item.key === motion.key)
  if (!discoveredMotion) {
    return
  }

  const now = Date.now()
  const throttleWindow = Math.max(motion.durationMS || 0, 900)
  if (runtime.currentMotionKey === motion.key && now-runtime.lastMotionStartedAt < throttleWindow) {
    return
  }

  const priority = motion.priority === 'force' ? runtime.motionPriority.FORCE : runtime.motionPriority.NORMAL
  const started = await runtime.model.motion(discoveredMotion.runtimeGroup, discoveredMotion.index, priority)
  if (started) {
    runtime.currentMotionKey = motion.key
    runtime.lastMotionStartedAt = now
  }
}

/**
 * 用后端已回退发现的动作清单补齐前端模型 settings，避免原始 model3.json 未声明 Motions 时舞台无法识别和播放动作。
 */
function hydrateLive2DModelSettings(
  modelUrl: string,
  modelSettings: Live2DModelSettingsPayload,
  serverMotions: SelectableLive2DModelMotion[],
): Live2DModelSettingsPayload {
  const nextSettings: Live2DModelSettingsPayload = {
    ...modelSettings,
    url: modelUrl,
    FileReferences: {
      ...(modelSettings.FileReferences || {}),
    },
  }

  const currentMotions = nextSettings.FileReferences?.Motions || {}
  const mergedMotions = mergeLive2DModelSettingMotions(modelUrl, currentMotions, serverMotions)
  if (Object.keys(mergedMotions).length > 0 && nextSettings.FileReferences) {
    nextSettings.FileReferences.Motions = mergedMotions
  }
  return nextSettings
}

/**
 * 合并 settings 原始动作定义与后端补录动作，优先保留模型自带顺序，并补齐缺失的目录扫描结果。
 */
function mergeLive2DModelSettingMotions(
  modelUrl: string,
  currentMotions: Record<string, Array<{ File?: string }>>,
  serverMotions: SelectableLive2DModelMotion[],
): Record<string, Array<{ File?: string }>> {
  const merged: Record<string, Array<{ File?: string }>> = {}
  const seenFiles = new Set<string>()

  for (const [group, definitions] of Object.entries(currentMotions)) {
    const safeGroup = group.trim()
    if (!safeGroup) {
      continue
    }

    merged[safeGroup] = []
    for (const definition of definitions || []) {
      if (!definition?.File?.trim()) {
        continue
      }
      const normalizedFile = definition.File.trim()
      seenFiles.add(`${safeGroup}::${normalizedFile}`)
      merged[safeGroup].push({ File: normalizedFile })
    }
  }

  for (const motion of serverMotions) {
    const runtimeGroup = (motion.group || 'auto').trim() || 'auto'
    const motionFile = resolveLive2DSettingMotionFile(modelUrl, motion.file)
    if (!motionFile) {
      continue
    }

    const dedupeKey = `${runtimeGroup}::${motionFile}`
    if (seenFiles.has(dedupeKey)) {
      continue
    }

    seenFiles.add(dedupeKey)
    if (!merged[runtimeGroup]) {
      merged[runtimeGroup] = []
    }
    merged[runtimeGroup].push({ File: motionFile })
  }

  return merged
}

/**
 * 将后端返回的动作资源地址转换成相对当前模型 settings 可解析的路径，避免绝对地址与相对地址混用带来的重复项。
 */
function resolveLive2DSettingMotionFile(modelUrl: string, motionFile: string): string {
  const trimmedFile = motionFile.trim()
  if (!trimmedFile) {
    return ''
  }

  if (/^https?:\/\//i.test(trimmedFile)) {
    return trimmedFile
  }

  const normalizedModelUrl = modelUrl.replace(/\\/g, '/')
  const normalizedMotionFile = trimmedFile.replace(/\\/g, '/')
  const baseDirectory = normalizedModelUrl.slice(0, normalizedModelUrl.lastIndexOf('/') + 1)
  if (normalizedMotionFile.startsWith(baseDirectory)) {
    return normalizedMotionFile.slice(baseDirectory.length)
  }

  return normalizedMotionFile
}

/**
 * 读取模型 DisplayInfo 文件，为参数发现和后续调试面板提供基础参数集合。
 */
async function resolveDisplayInfoPayload(
  modelUrl: string,
  modelSettings: Live2DModelSettingsPayload,
): Promise<Live2DDisplayInfoPayload | null> {
  const displayInfoPath = modelSettings.FileReferences?.DisplayInfo?.trim()
  if (!displayInfoPath) {
    return null
  }

  return fetchOptionalJson<Live2DDisplayInfoPayload>(resolveRelativeLive2DAssetUrl(modelUrl, displayInfoPath))
}

/**
 * 读取同目录下的 vtube 参数映射文件，补充模型额外暴露的 Live2D 输出参数。
 */
async function resolveVtubePayload(modelUrl: string): Promise<Live2DVtubePayload | null> {
  const vtubeUrl = modelUrl.replace(/\.model3\.json$/i, '.vtube.json')
  return fetchOptionalJson<Live2DVtubePayload>(vtubeUrl)
}

/**
 * 把表达式层文件里的参数定义合并到目标参数表中，并按权重叠加最终值。
 */
async function mergeExpressionLayerIntoTargets(
  runtime: Live2DStageRuntime,
  nextTargets: Map<string, number>,
  layer: Live2DStageExpressionLayer,
): Promise<void> {
  const expression = runtime.metadata.expressions.find((item) => item.id === layer.key)
  if (!expression) {
    return
  }

  const payload = await loadExpressionFilePayload(runtime, expression.file)
  for (const parameter of payload.Parameters || []) {
    if (!parameter.Id?.trim() || typeof parameter.Value !== 'number') {
      continue
    }

    const parameterId = parameter.Id.trim()
    ensureTrackedParameter(runtime, parameterId)
    const baseline = runtime.baselineParams.get(parameterId) ?? 0
    const currentValue = nextTargets.get(parameterId) ?? baseline
    nextTargets.set(
      parameterId,
      mixExpressionParameterValue(currentValue, parameter.Value, layer.weight, parameter.Blend),
    )
  }
}

/**
 * 读取并缓存表达式文件内容，避免同一模型上频繁切换状态时重复请求 exp3.json。
 */
async function loadExpressionFilePayload(
  runtime: Live2DStageRuntime,
  expressionFileUrl: string,
): Promise<Live2DExpressionFilePayload> {
  const cachedPayload = runtime.expressionFileCache.get(expressionFileUrl)
  if (cachedPayload) {
    return cachedPayload
  }

  const payload = await fetchJson<Live2DExpressionFilePayload>(expressionFileUrl)
  runtime.expressionFileCache.set(expressionFileUrl, payload)
  return payload
}

/**
 * 确保运行时已经记录目标参数的基线值，后续过渡动画会以该基线作为回落目标。
 */
function ensureTrackedParameter(runtime: Live2DStageRuntime, parameterId: string): void {
  if (runtime.trackedParamIds.has(parameterId)) {
    return
  }

  runtime.trackedParamIds.add(parameterId)
  runtime.baselineParams.set(parameterId, readLive2DCoreParameter(runtime, parameterId))
}

/**
 * 每帧平滑推进当前参数值，避免表情和姿态在状态切换时出现明显硬切。
 */
function applyRuntimeOverlayFrame(runtime: Live2DStageRuntime): void {
  if (runtime.trackedParamIds.size === 0) {
    return
  }

  for (const parameterId of runtime.trackedParamIds) {
    const baseline = runtime.baselineParams.get(parameterId) ?? 0
    const target = runtime.overlayTargetParams.get(parameterId) ?? baseline
    const current = runtime.overlayCurrentParams.get(parameterId) ?? baseline
    const next = easeLive2DParameter(current, target, getLive2DOverlayFactor(parameterId, runtime.overlayTargetParams.has(parameterId)))

    runtime.overlayCurrentParams.set(parameterId, next)
    writeLive2DCoreParameter(runtime, parameterId, next)
  }
}

/**
 * 按参数类型选取更贴近视觉直觉的平滑系数，让视线、头部和嘴型过渡速度有所区分。
 */
function getLive2DOverlayFactor(parameterId: string, hasExplicitTarget: boolean): number {
  if (/^ParamEyeBall[XY]$/i.test(parameterId)) {
    return hasExplicitTarget ? 0.26 : 0.18
  }

  if (/^(ParamAngle|ParamBodyAngle)/i.test(parameterId)) {
    return hasExplicitTarget ? 0.14 : 0.1
  }

  if (/^ParamMouthOpenY$/i.test(parameterId)) {
    return hasExplicitTarget ? 0.24 : 0.16
  }

  if (/^ParamMouthForm$/i.test(parameterId)) {
    return hasExplicitTarget ? 0.18 : 0.13
  }

  return hasExplicitTarget ? 0.18 : 0.12
}

/**
 * 将当前值向目标值逼近，直到足够接近时直接收敛，减少尾部抖动。
 */
function easeLive2DParameter(current: number, target: number, factor: number): number {
  if (Math.abs(target - current) <= 0.0005) {
    return target
  }

  return current + (target - current) * factor
}

/**
 * 根据表达式文件里声明的 Blend 模式，计算当前层叠加后的参数目标值。
 */
function mixExpressionParameterValue(
  currentValue: number,
  expressionValue: number,
  weight: number,
  blend: string | undefined,
): number {
  const normalizedWeight = Math.min(Math.max(weight, 0), 1)
  switch (blend) {
    case 'Multiply':
      return currentValue * (1 + (expressionValue - 1) * normalizedWeight)
    case 'Overwrite':
      return currentValue + (expressionValue - currentValue) * normalizedWeight
    default:
      return currentValue + expressionValue * normalizedWeight
  }
}

/**
 * 安全读取当前模型参数值，避免个别模型缺参时直接抛出异常中断整个舞台。
 */
function readLive2DCoreParameter(runtime: Live2DStageRuntime, parameterId: string): number {
  const coreModel = resolveLive2DCoreModel(runtime)
  if (!coreModel?.getParameterValueById) {
    return 0
  }

  try {
    return coreModel.getParameterValueById(parameterId)
  } catch {
    return 0
  }
}

/**
 * 安全写入当前模型参数值，缺失参数时直接忽略，保证舞台其余控制仍然可用。
 */
function writeLive2DCoreParameter(runtime: Live2DStageRuntime, parameterId: string, value: number): void {
  const coreModel = resolveLive2DCoreModel(runtime)
  if (!coreModel?.setParameterValueById) {
    return
  }

  try {
    coreModel.setParameterValueById(parameterId, value)
  } catch {
    // 某些模型没有对应参数时直接忽略即可。
  }
}

/**
 * 获取当前 Cubism CoreModel 的安全代理，供参数读写辅助函数统一使用。
 */
function resolveLive2DCoreModel(runtime: Live2DStageRuntime): Live2DCoreModelLike | null {
  return (runtime.model.internalModel as { coreModel?: Live2DCoreModelLike } | undefined)?.coreModel || null
}

/**
 * 按模型所在目录解析相对资源地址，兼容 model3.json 内部对表达式和 DisplayInfo 的引用。
 */
function resolveRelativeLive2DAssetUrl(modelUrl: string, relativeAssetPath: string): string {
  if (/^https?:\/\//i.test(relativeAssetPath) || relativeAssetPath.startsWith('/')) {
    return relativeAssetPath
  }

  const normalizedModelUrl = modelUrl.replace(/\\/g, '/')
  const baseDirectory = normalizedModelUrl.slice(0, normalizedModelUrl.lastIndexOf('/') + 1)
  return `${baseDirectory}${relativeAssetPath.replace(/^\.?\/+/, '')}`
}

/**
 * 将表达式名转换成前端内部稳定 ID，便于跨模型和跨页面统一匹配。
 */
function normalizeLive2DExpressionId(rawExpressionName: string): string {
  return rawExpressionName
    .replace(/\.exp3\.json$/i, '')
    .trim()
    .toLowerCase()
    .replace(/[\s-]+/g, '_')
    .replace(/[^a-z0-9_\u4e00-\u9fff]+/gi, '_')
    .replace(/^_+|_+$/g, '')
}

/**
 * 将表达式文件名整理成更可读的展示标签，供控制抽屉和状态面板直接使用。
 */
function formatLive2DExpressionLabel(rawExpressionName: string): string {
  const cleaned = rawExpressionName.replace(/\.exp3\.json$/i, '').trim()
  if (/[\u4e00-\u9fff]/u.test(cleaned)) {
    return cleaned
  }

  return cleaned
    .split(/[_\-\s]+/)
    .filter(Boolean)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(' ')
}

/**
 * 将动作组名和文件名转换成前端内部稳定动作键，保证与后端 manifest 生成规则一致。
 */
function normalizeLive2DMotionKey(group: string, rawMotionName: string, index: number): string {
  const groupToken = normalizeLive2DMotionToken(group)
  const nameToken = normalizeLive2DMotionToken(rawMotionName.replace(/\.motion3\.json$/i, ''))
  const merged = [groupToken && groupToken !== 'auto' ? groupToken : '', nameToken].filter(Boolean).join('_')
  return merged || `motion_${index}`
}

/**
 * 规整动作名或分组名，便于跨端统一匹配。
 */
function normalizeLive2DMotionToken(rawMotionName: string): string {
  return rawMotionName
    .trim()
    .toLowerCase()
    .replace(/[\s.-]+/g, '_')
    .replace(/[^a-z0-9_\u4e00-\u9fff]+/gi, '_')
    .replace(/^_+|_+$/g, '')
}

/**
 * 把动作文件名整理成更适合控制面板展示的标签。
 */
function formatLive2DMotionLabel(rawMotionName: string): string {
  const cleaned = rawMotionName.replace(/\.motion3\.json$/i, '').trim()
  if (/[\u4e00-\u9fff]/u.test(cleaned)) {
    return cleaned
  }

  return cleaned
    .split(/[_\-\s]+/)
    .filter(Boolean)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(' ')
}

/**
 * 拉取一个 JSON 资源，失败时直接抛出错误，让舞台调用方统一决定如何展示加载失败状态。
 */
async function fetchJson<T>(url: string): Promise<T> {
  const response = await fetch(url)
  if (!response.ok) {
    throw new Error(`读取 Live2D 资源失败：${url}`)
  }

  return await response.json() as T
}

/**
 * 尝试读取一个可选 JSON 资源，失败时直接回退为空，避免模型缺少辅助文件时整页报错。
 */
async function fetchOptionalJson<T>(url: string): Promise<T | null> {
  try {
    const response = await fetch(url)
    if (!response.ok) {
      return null
    }

    return await response.json() as T
  } catch {
    return null
  }
}
