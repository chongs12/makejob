import { useEffect, useMemo, useRef, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import type { Application } from 'pixi.js'
import type { Live2DModel as Cubism4Live2DModel } from 'pixi-live2d-display/cubism4'
import { extractErrorMessage, requestJson } from '@makejob/api-client'
import { isSuccessCode, type ApiEnvelope } from '@makejob/shared-types'
import { loadLive2DRuntime } from '../../shared/live2dRuntime'

const INTERVIEW_SELECTED_MODEL_KEY_PREFIX = 'makejob.interview.selected-live2d:'

interface InterviewSelectableLive2DModel {
  key: string
  name: string
  scene: string
  model_url: string
  thumbnail_url: string
  source: string
  match_type: string
  is_generic: boolean
  is_recommended: boolean
}

/**
 * 获取面试场景可用的 Live2D 模型列表，并沿用后端推荐顺序。
 */
async function fetchSelectableInterviewLive2DModels(industryCode: string): Promise<InterviewSelectableLive2DModel[]> {
  const params = new URLSearchParams({
    scene: 'interview',
  })

  if (industryCode.trim()) {
    params.set('industry_code', industryCode.trim())
  }

  const response = await requestJson<ApiEnvelope<InterviewSelectableLive2DModel[]>>(`/live2d/models?${params.toString()}`)
  if (!isSuccessCode(response.code) || !response.data) {
    throw new Error(response.message || '获取面试 Live2D 模型列表失败')
  }

  return response.data
}

/**
 * 从本地缓存读取当前行业上次选择的面试模型。
 */
function readSelectedInterviewModelKey(industryCode: string): string {
  if (typeof window === 'undefined') {
    return ''
  }

  return window.localStorage.getItem(`${INTERVIEW_SELECTED_MODEL_KEY_PREFIX}${industryCode}`) || ''
}

/**
 * 持久化当前行业选择的面试模型，避免刷新后重新选择。
 */
function persistSelectedInterviewModelKey(industryCode: string, key: string): void {
  if (typeof window === 'undefined') {
    return
  }

  window.localStorage.setItem(`${INTERVIEW_SELECTED_MODEL_KEY_PREFIX}${industryCode}`, key)
}

/**
 * 按当前容器大小重新布局模型，让面试舞台在桌面和移动端都保持稳定站位。
 */
function layoutInterviewModel(model: Cubism4Live2DModel, host: HTMLDivElement, baseWidth: number, baseHeight: number): void {
  const safeBaseWidth = Math.max(baseWidth, 1)
  const safeBaseHeight = Math.max(baseHeight, 1)
  const widthScale = (host.clientWidth * 0.84) / safeBaseWidth
  const heightScale = (host.clientHeight * 1.08) / safeBaseHeight
  const scale = Math.max(Math.min(widthScale, heightScale) * 0.88, 0.1)

  model.scale.set(scale)
  model.anchor.set(0.5, 1)
  model.position.set(host.clientWidth * 0.5, host.clientHeight * 0.94)
}

/**
 * 向核心模型安全写入参数，避免单个模型缺失参数时抛出异常。
 */
function safeSetParameter(coreModel: {
  setParameterValueById?: (parameterId: string, value: number, weight?: number) => void
} | null | undefined, parameterId: string, value: number, weight = 1): void {
  if (!coreModel?.setParameterValueById) {
    return
  }

  try {
    coreModel.setParameterValueById(parameterId, value, weight)
  } catch {
    // 某些模型没有对应参数时直接忽略。
  }
}

/**
 * 按当前情绪与嘴型开合值更新常见 Cubism 参数，实现基础表情和口型联动。
 */
function applyInterviewExpression(model: Cubism4Live2DModel | null, emotion: string, mouthOpen: number): void {
  const coreModel = (model?.internalModel as { coreModel?: { setParameterValueById?: (parameterId: string, value: number, weight?: number) => void } } | undefined)?.coreModel
  if (!coreModel) {
    return
  }

  const normalizedMouth = Math.max(0, Math.min(mouthOpen, 1))
  safeSetParameter(coreModel, 'ParamMouthOpenY', normalizedMouth)
  safeSetParameter(coreModel, 'ParamEyeLOpen', 1)
  safeSetParameter(coreModel, 'ParamEyeROpen', 1)
  safeSetParameter(coreModel, 'ParamMouthForm', 0)
  safeSetParameter(coreModel, 'ParamBrowLY', 0)
  safeSetParameter(coreModel, 'ParamBrowRY', 0)
  safeSetParameter(coreModel, 'ParamAngleX', 0)
  safeSetParameter(coreModel, 'ParamBodyAngleX', 0)

  switch (emotion) {
    case 'serious':
      safeSetParameter(coreModel, 'ParamBrowLY', -0.35)
      safeSetParameter(coreModel, 'ParamBrowRY', -0.35)
      break
    case 'thinking':
      safeSetParameter(coreModel, 'ParamBrowLY', 0.2)
      safeSetParameter(coreModel, 'ParamBrowRY', -0.1)
      safeSetParameter(coreModel, 'ParamAngleX', 8)
      safeSetParameter(coreModel, 'ParamBodyAngleX', 4)
      break
    case 'encourage':
      safeSetParameter(coreModel, 'ParamMouthForm', 0.35)
      safeSetParameter(coreModel, 'ParamEyeLOpen', 0.92)
      safeSetParameter(coreModel, 'ParamEyeROpen', 0.92)
      break
    case 'praise':
      safeSetParameter(coreModel, 'ParamMouthForm', 0.75)
      safeSetParameter(coreModel, 'ParamEyeLOpen', 0.88)
      safeSetParameter(coreModel, 'ParamEyeROpen', 0.88)
      break
    case 'warning':
      safeSetParameter(coreModel, 'ParamBrowLY', -0.52)
      safeSetParameter(coreModel, 'ParamBrowRY', -0.52)
      safeSetParameter(coreModel, 'ParamMouthForm', -0.28)
      break
    default:
      break
  }
}

/**
 * 渲染面试页 Live2D 舞台，并实时响应情绪和嘴型状态变化。
 */
export function InterviewLive2DStage(props: {
  industryCode: string
  dialogue: string
  isTyping: boolean
  emotion: string
  mouthOpen: number
}) {
  const hostRef = useRef<HTMLDivElement | null>(null)
  const modelRef = useRef<Cubism4Live2DModel | null>(null)
  const [selectedModelKey, setSelectedModelKey] = useState(() => readSelectedInterviewModelKey(props.industryCode))
  const [stageLoading, setStageLoading] = useState(false)
  const [stageError, setStageError] = useState('')

  const modelOptionsQuery = useQuery({
    queryKey: ['interview-live2d-models', props.industryCode],
    queryFn: () => fetchSelectableInterviewLive2DModels(props.industryCode),
    staleTime: 60 * 1000,
  })

  useEffect(() => {
    setSelectedModelKey(readSelectedInterviewModelKey(props.industryCode))
  }, [props.industryCode])

  const modelOptions = modelOptionsQuery.data || []
  const currentModel = useMemo(() => {
    const explicitModel = modelOptions.find((item) => item.key === selectedModelKey)
    if (explicitModel) {
      return explicitModel
    }

    return modelOptions.find((item) => item.is_recommended) || modelOptions[0] || null
  }, [modelOptions, selectedModelKey])
  const currentModelName = currentModel?.name || '面试官'
  const isLoading = modelOptionsQuery.isLoading || stageLoading
  const errorMessage = modelOptionsQuery.isError
    ? extractErrorMessage(modelOptionsQuery.error, '读取面试 Live2D 模型失败')
    : stageError

  useEffect(() => {
    if (!currentModel?.key) {
      return
    }

    persistSelectedInterviewModelKey(props.industryCode, currentModel.key)
    if (currentModel.key !== selectedModelKey) {
      setSelectedModelKey(currentModel.key)
    }
  }, [currentModel?.key, props.industryCode, selectedModelKey])

  useEffect(() => {
    const host = hostRef.current
    if (!host || !currentModel?.model_url) {
      return undefined
    }

    let destroyed = false
    let app: Application | null = null
    let baseWidth = 1
    let baseHeight = 1

    /**
     * 同步 Pixi 舞台尺寸与模型站位，避免窗口变化后布局漂移。
     */
    function syncStageLayout(): void {
      if (!host || !app || !modelRef.current) {
        return
      }

      app.renderer.resize(host.clientWidth, host.clientHeight)
      layoutInterviewModel(modelRef.current, host, baseWidth, baseHeight)
    }

    const resizeObserver = new ResizeObserver(() => {
      syncStageLayout()
    })

    /**
     * 初始化面试页 Live2D 舞台并异步加载模型资源。
     */
    async function mountStage(): Promise<void> {
      setStageLoading(true)
      setStageError('')

      try {
        const { PIXI, Live2DModel } = await loadLive2DRuntime()

        app = new PIXI.Application({
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
        resizeObserver.observe(host)

        modelRef.current = await Live2DModel.from(currentModel.model_url)
        if (destroyed || !app || !modelRef.current) {
          modelRef.current?.destroy()
          modelRef.current = null
          return
        }

        baseWidth = Math.max(modelRef.current.width, 1)
        baseHeight = Math.max(modelRef.current.height, 1)
        app.stage.addChild(modelRef.current)
        syncStageLayout()
        modelRef.current.focus(host.clientWidth * 0.5, host.clientHeight * 0.58, true)
        setStageLoading(false)
      } catch (error) {
        if (destroyed) {
          return
        }

        setStageError(error instanceof Error ? error.message : 'Live2D 模型加载失败')
        setStageLoading(false)
      }
    }

    void mountStage()

    return () => {
      destroyed = true
      resizeObserver.disconnect()

      if (modelRef.current && app) {
        app.stage.removeChild(modelRef.current)
        modelRef.current.destroy()
        modelRef.current = null
      }

      app?.destroy(true)
    }
  }, [currentModel?.model_url])

  useEffect(() => {
    let frameId = 0

    /**
     * 每帧把最新情绪和嘴型值写入当前模型参数，形成平滑联动。
     */
    function tick(): void {
      applyInterviewExpression(modelRef.current, props.emotion, props.mouthOpen)
      frameId = window.requestAnimationFrame(tick)
    }

    frameId = window.requestAnimationFrame(tick)
    return () => {
      window.cancelAnimationFrame(frameId)
    }
  }, [props.emotion, props.mouthOpen])

  return (
    <section className="interview-live2d-panel">
      <div className="interview-live2d-canvas-wrap">
        <div className="interview-live2d-canvas" ref={hostRef} />

        {isLoading ? (
          <div className="companion-stage-overlay">
            <strong>{modelOptionsQuery.isLoading ? '正在读取可用模型' : `正在加载 ${currentModelName}`}</strong>
            <span>{modelOptionsQuery.isLoading ? '前台正在读取面试场景模型列表。' : '模型资源加载中，请稍等片刻。'}</span>
          </div>
        ) : null}

        {errorMessage ? (
          <div className="companion-stage-overlay companion-stage-overlay-error">
            <strong>模型加载失败</strong>
            <span>{errorMessage}</span>
          </div>
        ) : null}

        <div className="interview-live2d-dialogue">
          <span className="section-kicker">AI 面试官</span>
          <p className={`interview-live2d-dialogue-text${props.isTyping ? ' is-typing' : ''}`}>{props.dialogue}</p>
        </div>
      </div>
    </section>
  )
}
