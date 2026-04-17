<template>
  <div class="live2d-canvas-container" :style="{ width: `${width}px`, height: `${height}px` }">
    <canvas
      ref="canvasRef"
      class="live2d-canvas"
      :class="{ 'live2d-canvas-visible': rendererReady }"
    />

    <div v-if="!rendererReady" class="live2d-stage">
      <img
        v-if="thumbnailUrl"
        :src="thumbnailUrl"
        :alt="titleText"
        class="live2d-thumbnail"
      >
      <div v-else class="live2d-fallback-icon">
        Live2D
      </div>
    </div>

    <div v-if="showLoading" class="absolute inset-0 flex items-center justify-center rounded-lg bg-white/72 backdrop-blur-sm">
      <el-icon class="is-loading" :size="32"><Loading /></el-icon>
    </div>

    <div v-if="mergedError" class="live2d-status live2d-status-error">
      <div class="live2d-status-title">{{ titleText }}</div>
      <div class="live2d-status-subtitle">{{ mergedError }}</div>
    </div>
    <div v-else-if="showReadyNotice" class="live2d-status live2d-status-toast">
      <div class="live2d-status-title">{{ titleText }}</div>
      <div class="live2d-status-subtitle">{{ subtitleText }}</div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { Loading } from '@element-plus/icons-vue'
import type { Live2DModelConfig } from './types'
import { LOCAL_CUBISM_CORE_PATH, loadCubismCore } from '~/utils/live2d-runtime'

type PixiNamespace = typeof import('pixi.js')

type PixiApp = {
  stage: { addChild: (child: unknown) => void }
  renderer: { resize: (width: number, height: number) => void }
  destroy: (removeView?: boolean, stageOptions?: Record<string, unknown>) => void
}

type PixiLive2DModel = {
  width: number
  height: number
  x: number
  y: number
  interactive?: boolean
  scale: { set: (x: number, y?: number) => void }
  anchor: { set: (x: number, y?: number) => void }
  on: (event: string, listener: (...args: any[]) => void) => void
  motion: (group: string, index?: number) => Promise<boolean>
  destroy: (options?: Record<string, unknown>) => void
}

type ModelBaseSize = {
  width: number
  height: number
}

const props = withDefaults(defineProps<{
  width?: number
  height?: number
  modelPath?: string
  modelConfig?: Live2DModelConfig | null
  loading?: boolean
  error?: string
}>(), {
  width: 300,
  height: 400,
  modelPath: '',
  modelConfig: null,
  loading: false,
  error: '',
})

const LOCAL_CUBISM_CORE_FILE = 'frontend/public/live2d-runtime/live2dcubismcore.min.js'

const canvasRef = ref<HTMLCanvasElement | null>(null)
const initializing = ref(false)
const rendererReady = ref(false)
const runtimeError = ref('')
const coreSource = ref<'window' | 'local' | 'remote' | ''>('')
const showReadyNotice = ref(false)

let app: PixiApp | null = null
let model: PixiLive2DModel | null = null
let modelBaseSize: ModelBaseSize = { width: 0, height: 0 }
let readyNoticeTimer: ReturnType<typeof setTimeout> | null = null

const thumbnailUrl = computed(() => props.modelConfig?.thumbnailUrl || '')
const titleText = computed(() => props.modelConfig?.name || 'Live2D 角色')
const resolvedModelPath = computed(() => props.modelConfig?.modelUrl || props.modelConfig?.path || props.modelPath || '')
const mergedError = computed(() => props.error || runtimeError.value)
const showLoading = computed(() => props.loading || initializing.value)
const rendererKey = computed(() => JSON.stringify({
  modelPath: resolvedModelPath.value,
  tapMotion: String(props.modelConfig?.config?.tap_motion || ''),
}))
const layoutSignature = computed(() => JSON.stringify({
  width: props.width,
  height: props.height,
  scale: Number(props.modelConfig?.scale ?? 0.4),
  offsetX: Number(props.modelConfig?.position?.x ?? 0),
  offsetY: Number(props.modelConfig?.position?.y ?? 0),
}))
const subtitleText = computed(() => {
  if (rendererReady.value) {
    if (coreSource.value === 'remote') {
      return '已加载真实 Live2D 渲染，当前使用远程 Core，建议补齐本地运行时文件'
    }

    return props.modelConfig?.source === 'database'
      ? '已接入后台配置的真实 Live2D 模型'
      : '已加载内置真实 Live2D 模型'
  }

  if (mergedError.value) {
    return '渲染失败，当前展示静态预览图'
  }

  if (showLoading.value) {
    return '正在加载 Live2D 渲染资源'
  }

  return props.modelConfig?.source === 'database'
    ? '准备加载后台配置的 Live2D 模型'
    : '准备加载默认 Live2D 模型'
})

/**
 * 读取当前设备像素比，并限制到清晰度和性能都可接受的范围。
 */
const resolveRenderResolution = (): number => {
  if (!process.client) {
    return 1
  }

  return Math.min(Math.max(window.devicePixelRatio || 1, 1), 2)
}

/**
 * 仅在成功加载后短暂展示一次状态提示，避免长期遮挡角色。
 */
const showReadyStatusOnce = () => {
  if (readyNoticeTimer) {
    clearTimeout(readyNoticeTimer)
  }

  showReadyNotice.value = true
  readyNoticeTimer = setTimeout(() => {
    showReadyNotice.value = false
    readyNoticeTimer = null
  }, 2200)
}

/**
 * 检查当前环境是否能创建 WebGL 上下文。
 */
const supportsWebGL = (): boolean => {
  if (!process.client) {
    return false
  }

  const probeCanvas = document.createElement('canvas')
  const context = probeCanvas.getContext('webgl') || probeCanvas.getContext('experimental-webgl')

  if (context && 'getExtension' in context) {
    const loseContext = context.getExtension('WEBGL_lose_context')
    loseContext?.loseContext()
  }

  return Boolean(context)
}

/**
 * 重置画布尺寸，用于销毁后清空残留内容。
 * 这里不能调用 2D context，否则同一个 canvas 后续无法创建 WebGL context。
 */
const clearCanvas = () => {
  if (!canvasRef.value) {
    return
  }

  canvasRef.value.width = props.width
  canvasRef.value.height = props.height
}

/**
 * 释放 Pixi 和 Live2D 实例，避免重复初始化时泄漏资源。
 */
const destroyRenderer = () => {
  if (readyNoticeTimer) {
    clearTimeout(readyNoticeTimer)
    readyNoticeTimer = null
  }

  showReadyNotice.value = false

  if (model) {
    model.destroy({ children: true })
    model = null
  }

  if (app) {
    app.destroy(false, { children: true, texture: true, baseTexture: true })
    app = null
  }

  rendererReady.value = false
  coreSource.value = ''
  modelBaseSize = { width: 0, height: 0 }
  clearCanvas()
}

/**
 * 记录模型的原始尺寸，避免窗口缩放时基于已缩放结果重复计算。
 */
const syncModelBaseSize = (target: PixiLive2DModel) => {
  modelBaseSize = {
    width: Math.max(target.width, 1),
    height: Math.max(target.height, 1),
  }
}

/**
 * 在不重建 WebGL 上下文的前提下刷新画布尺寸和模型布局。
 */
const resizeRenderer = () => {
  if (!app || !model || props.width <= 0 || props.height <= 0) {
    return
  }

  app.renderer.resize(props.width, props.height)
  layoutModel(model)
}

/**
 * 将 Cubism Core 的命中结果转成页面可读状态。
 */
const resolveCoreSource = (loadedFrom: string): 'window' | 'local' | 'remote' => {
  if (loadedFrom === 'window') {
    return 'window'
  }

  if (loadedFrom === LOCAL_CUBISM_CORE_PATH) {
    return 'local'
  }

  return 'remote'
}

/**
 * 统一整理运行时异常，输出用户可执行的排查提示。
 */
const normalizeRendererError = (error: unknown): string => {
  const message = error instanceof Error ? error.message : String(error || '')

  if (message.includes('live2dcubismcore') || message.includes('Live2D Core')) {
    return `未找到 Live2D Core，请将运行时文件放到 ${LOCAL_CUBISM_CORE_FILE}`
  }

  if (message.includes('WebGL')) {
    return '当前浏览器未能创建 WebGL 上下文，请关闭旧页面后刷新；如果仍失败，请检查浏览器硬件加速是否开启'
  }

  if (message.includes('404') || message.includes('.model3.json')) {
    return 'Live2D 模型文件不存在，请检查模型地址和静态资源映射'
  }

  return message || 'Live2D 渲染初始化失败，请检查模型资源和浏览器控制台'
}

/**
 * 按后端下发的缩放和偏移参数摆放模型。
 */
const layoutModel = (target: PixiLive2DModel) => {
  const configScale = Number(props.modelConfig?.scale ?? 0.4)
  const offsetX = Number(props.modelConfig?.position?.x ?? 0)
  const offsetY = Number(props.modelConfig?.position?.y ?? 0)
  const baseWidth = Math.max(modelBaseSize.width || target.width, 1)
  const baseHeight = Math.max(modelBaseSize.height || target.height, 1)
  const baseScale = Math.min(props.width / baseWidth, props.height / baseHeight)
  const finalScale = baseScale * Math.max(configScale, 0.01)

  target.anchor.set(0.5, 0.5)
  target.scale.set(finalScale)
  target.x = props.width * (0.5 + offsetX)
  target.y = props.height * (0.56 + offsetY)
}

/**
 * 绑定点击动作，让配置里的 tap_motion 能在真实渲染下触发。
 */
const bindTapMotion = (target: PixiLive2DModel) => {
  const tapMotion = String(props.modelConfig?.config?.tap_motion || '').trim()
  if (!tapMotion) {
    return
  }

  target.interactive = true
  target.on('pointertap', () => {
    void target.motion(tapMotion).catch(() => undefined)
  })
}

/**
 * 初始化 Pixi 和 Live2D 模型，并挂载到当前画布。
 */
const createRenderer = async () => {
  const modelPath = resolvedModelPath.value
  if (!process.client || !canvasRef.value || !modelPath || props.width <= 0 || props.height <= 0) {
    destroyRenderer()
    return
  }

  initializing.value = true
  runtimeError.value = ''

  try {
    destroyRenderer()

    if (!supportsWebGL()) {
      throw new Error('WebGL unsupported')
    }

    const loadedFrom = await loadCubismCore()
    coreSource.value = resolveCoreSource(loadedFrom)

    const PIXI = await import('pixi.js')
    ;(window as Window & typeof globalThis & { PIXI?: PixiNamespace }).PIXI = PIXI

    const { Live2DModel } = await import('pixi-live2d-display/cubism4')

    app = new PIXI.Application({
      view: canvasRef.value,
      width: props.width,
      height: props.height,
      backgroundAlpha: 0,
      antialias: true,
      autoDensity: true,
      resolution: resolveRenderResolution(),
      powerPreference: 'high-performance',
    }) as unknown as PixiApp

    model = await Live2DModel.from(modelPath, { autoInteract: false }) as unknown as PixiLive2DModel
    app.stage.addChild(model)
    syncModelBaseSize(model)

    resizeRenderer()
    bindTapMotion(model)
    rendererReady.value = true
    showReadyStatusOnce()
  } catch (error) {
    destroyRenderer()
    runtimeError.value = normalizeRendererError(error)
  } finally {
    initializing.value = false
  }
}

watch(
  rendererKey,
  () => {
    void createRenderer()
  },
  { immediate: true },
)

watch(
  layoutSignature,
  () => {
    resizeRenderer()
  },
)

onUnmounted(() => {
  destroyRenderer()
})
</script>

<style scoped>
.live2d-canvas-container {
  position: relative;
  overflow: hidden;
  border-radius: 1rem;
  background:
    radial-gradient(circle at 20% 18%, rgba(255, 255, 255, 0.96), rgba(255, 255, 255, 0) 34%),
    linear-gradient(180deg, #eff6ff 0%, #eef2ff 45%, #e0e7ff 100%);
  border: 1px solid rgba(148, 163, 184, 0.22);
  box-shadow: 0 18px 48px rgba(79, 70, 229, 0.14);
}

.live2d-canvas {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  opacity: 0;
  transition: opacity 0.24s ease;
  image-rendering: auto;
  transform: translateZ(0);
}

.live2d-canvas-visible {
  opacity: 1;
}

.live2d-stage {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: flex-end;
  justify-content: center;
  padding: 20px 20px 56px;
}

.live2d-thumbnail {
  max-width: 100%;
  max-height: 100%;
  object-fit: contain;
  filter: drop-shadow(0 18px 30px rgba(79, 70, 229, 0.18));
}

.live2d-fallback-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 128px;
  height: 128px;
  border-radius: 999px;
  background: linear-gradient(135deg, rgba(99, 102, 241, 0.14), rgba(244, 114, 182, 0.16));
  color: #4f46e5;
  font-size: 20px;
  font-weight: 700;
  letter-spacing: 0.04em;
}

.live2d-status {
  position: absolute;
  left: 14px;
  right: 14px;
  bottom: 14px;
  padding: 10px 12px;
  border-radius: 14px;
  background: rgba(255, 255, 255, 0.82);
  border: 1px solid rgba(255, 255, 255, 0.92);
  backdrop-filter: blur(10px);
  box-shadow: 0 12px 28px rgba(15, 23, 42, 0.08);
}

.live2d-status-toast {
  left: auto;
  right: 16px;
  top: 16px;
  bottom: auto;
  max-width: min(320px, calc(100% - 32px));
  background: rgba(255, 255, 255, 0.9);
  pointer-events: none;
}

.live2d-status-error {
  color: #b91c1c;
}

.live2d-status-title {
  color: #111827;
  font-size: 14px;
  font-weight: 700;
}

.live2d-status-subtitle {
  margin-top: 2px;
  font-size: 12px;
}
</style>
