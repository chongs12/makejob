import { useEffect, useMemo, useRef, useState, type PointerEvent as ReactPointerEvent } from 'react'
import type { SelectableLive2DModel } from './live2dModelCatalog'
import type { Live2DStagePresetInput } from './live2dStagePresets'
import { resolveLive2DStagePreset } from './live2dStagePresets'
import {
  applyLive2DStageVisualState,
  createLive2DStageRuntime,
  destroyLive2DStageRuntime,
  focusLive2DStage,
  resetLive2DStageFocus,
  type Live2DStageModelMetadata,
  type Live2DStageRuntime,
  type Live2DStageTransform,
  updateLive2DStageTransform,
} from './live2dStageRuntime'

export interface Live2DStageStatusPill {
  label: string
  value: string
}

/**
 * 渲染统一的 Live2D 舞台，并把模型切换、交互和状态浮层收口到一个共享组件里。
 */
export function Live2DSceneStage(props: {
  variant: 'companion' | 'interview'
  stageTitle: string
  stageNote: string
  backgroundImageUrl?: string
  dialogue: string
  isTyping?: boolean
  modelOptions: SelectableLive2DModel[]
  currentModel: SelectableLive2DModel | null
  onSelectModelKey: (modelKey: string) => void
  preset: Live2DStagePresetInput
  statusPills: Live2DStageStatusPill[]
  loading: boolean
  loadingText: string
  errorMessage: string
  defaultTransform: Live2DStageTransform
}) {
  const hostRef = useRef<HTMLDivElement | null>(null)
  const runtimeRef = useRef<Live2DStageRuntime | null>(null)
  const dragStateRef = useRef<{
    pointerId: number
    mode: 'move' | 'scale'
    startX: number
    startY: number
    origin: Live2DStageTransform
  } | null>(null)
  const [metadata, setMetadata] = useState<Live2DStageModelMetadata | null>(null)
  const [transform, setTransform] = useState<Live2DStageTransform>({ ...props.defaultTransform })
  const [stageStatus, setStageStatus] = useState('等待加载模型')
  const [runtimeLoading, setRuntimeLoading] = useState(false)
  const [runtimeError, setRuntimeError] = useState('')
  const [backgroundLoadFailed, setBackgroundLoadFailed] = useState(false)
  const [controlDrawerOpen, setControlDrawerOpen] = useState(false)

  useEffect(() => {
    setTransform({ ...props.defaultTransform })
  }, [props.currentModel?.key, props.defaultTransform])

  const resolvedPreset = useMemo(() => {
    return resolveLive2DStagePreset(metadata, props.preset)
  }, [metadata, props.preset])

  useEffect(() => {
    setBackgroundLoadFailed(false)
  }, [props.backgroundImageUrl])

  useEffect(() => {
    const host = hostRef.current
    if (!host || !props.currentModel?.model_url) {
      setRuntimeLoading(false)
      setRuntimeError('')
      setMetadata(null)
      setStageStatus(props.loading ? '正在加载模型' : '当前没有可用模型')
      return undefined
    }

    let disposed = false
    let mountedRuntime: Live2DStageRuntime | null = null
    const resizeObserver = new ResizeObserver(() => {
      if (!hostRef.current || !runtimeRef.current) {
        return
      }

      updateLive2DStageTransform(runtimeRef.current, hostRef.current, runtimeRef.current.currentTransform)
    })

    /**
     * 创建当前舞台模型实例，并在完成后同步发现结果和默认视觉状态。
     */
    async function mountRuntime(): Promise<void> {
      setRuntimeLoading(true)
      setRuntimeError('')
      setStageStatus(`正在加载 ${props.currentModel?.name || '模型'}`)

      try {
        const runtime = await createLive2DStageRuntime(host, props.currentModel?.model_url || '', props.defaultTransform)
        if (disposed) {
          destroyLive2DStageRuntime(runtime)
          return
        }

        mountedRuntime = runtime
        runtimeRef.current = runtime
        setMetadata(runtime.metadata)
        setControlDrawerOpen(false)
        updateLive2DStageTransform(runtime, host, props.defaultTransform)
        setStageStatus('舞台已就绪')
        setRuntimeLoading(false)
        resizeObserver.observe(host)
      } catch (error) {
        if (disposed) {
          return
        }

        const nextError = error instanceof Error ? error.message : '模型加载失败'
        setMetadata(null)
        setRuntimeError(nextError)
        setRuntimeLoading(false)
        setStageStatus(nextError)
      }
    }

    void mountRuntime()

    return () => {
      disposed = true
      resizeObserver.disconnect()
      destroyLive2DStageRuntime(mountedRuntime)
      if (runtimeRef.current === mountedRuntime) {
        runtimeRef.current = null
      }
    }
  }, [props.currentModel?.key, props.currentModel?.model_url, props.defaultTransform])

  useEffect(() => {
    if (!hostRef.current || !runtimeRef.current) {
      return
    }

    updateLive2DStageTransform(runtimeRef.current, hostRef.current, transform)
  }, [transform])

  useEffect(() => {
    if (!runtimeRef.current) {
      return
    }

    /**
     * 将新的业务状态同步到舞台运行时，统一通过共享引擎做平滑过渡。
     */
    async function syncVisualState(): Promise<void> {
      try {
        setRuntimeError('')
        await applyLive2DStageVisualState(runtimeRef.current as Live2DStageRuntime, resolvedPreset)
        setStageStatus('舞台已就绪')
      } catch (error) {
        const nextError = error instanceof Error ? error.message : '表情应用失败'
        setRuntimeError(nextError)
        setStageStatus(nextError)
      }
    }

    void syncVisualState()
  }, [resolvedPreset])

  /**
   * 开始一轮拖拽交互，并根据是否按下 Shift 决定是移动还是缩放舞台。
   */
  function handlePointerDown(event: ReactPointerEvent<HTMLDivElement>): void {
    dragStateRef.current = {
      pointerId: event.pointerId,
      mode: event.shiftKey ? 'scale' : 'move',
      startX: event.clientX,
      startY: event.clientY,
      origin: { ...transform },
    }
    event.currentTarget.setPointerCapture(event.pointerId)
    setStageStatus(event.shiftKey ? '缩放中' : '拖拽中')
  }

  /**
   * 处理舞台指针移动，空闲时同步视线跟随，拖拽时实时更新偏移和缩放。
   */
  function handlePointerMove(event: ReactPointerEvent<HTMLDivElement>): void {
    const host = hostRef.current
    const runtime = runtimeRef.current
    if (host && runtime && (event.pointerType === 'mouse' || event.pointerType === 'pen')) {
      const rect = host.getBoundingClientRect()
      focusLive2DStage(runtime, event.clientX - rect.left, event.clientY - rect.top)
    }

    const dragState = dragStateRef.current
    if (!host || !dragState || dragState.pointerId !== event.pointerId) {
      return
    }

    const deltaX = event.clientX - dragState.startX
    const deltaY = event.clientY - dragState.startY

    if (dragState.mode === 'scale') {
      setTransform({
        ...dragState.origin,
        scale: clampStageTransformValue(dragState.origin.scale - deltaY / 140, 0.58, 1.72),
      })
      return
    }

    setTransform({
      ...dragState.origin,
      offsetX: clampStageTransformValue(dragState.origin.offsetX + deltaX / Math.max(host.clientWidth, 1), -0.26, 0.26),
      offsetY: clampStageTransformValue(dragState.origin.offsetY + deltaY / Math.max(host.clientHeight, 1), -0.18, 0.18),
    })
  }

  /**
   * 结束当前拖拽会话，并恢复舞台状态文案为就绪态。
   */
  function handlePointerUp(event: ReactPointerEvent<HTMLDivElement>): void {
    if (dragStateRef.current?.pointerId !== event.pointerId) {
      return
    }

    dragStateRef.current = null
    event.currentTarget.releasePointerCapture(event.pointerId)
    setStageStatus('舞台已就绪')
  }

  /**
   * 鼠标离开舞台时让角色视线回到默认中心，避免长时间停在边角方向。
   */
  function handlePointerLeave(): void {
    if (!hostRef.current || !runtimeRef.current) {
      return
    }

    resetLive2DStageFocus(runtimeRef.current, hostRef.current)
  }

  const activeParameterEntries = resolvedPreset.parameterOverrides.slice(0, 6)
  const activeErrorMessage = props.errorMessage || runtimeError

  return (
    <section className={`live2d-stage-panel live2d-stage-panel-${props.variant}`}>
      <div className="live2d-stage-topbar">
        <div className="live2d-stage-head">
          <div>
            <span className="section-kicker">{props.stageTitle}</span>
            <h2>{props.currentModel?.name || '等待模型加载'}</h2>
          </div>
          <div className="live2d-stage-pill-row">
            {props.statusPills.map((item) => (
              <span className="live2d-stage-pill" key={`${item.label}-${item.value}`}>
                {item.label}：{item.value}
              </span>
            ))}
          </div>
        </div>
        <div className="live2d-stage-side">
          <span className="live2d-stage-copy">{props.stageNote}</span>
          <div className="live2d-stage-toolbar">
            {props.modelOptions.length > 1 ? (
              <label className="live2d-stage-selector">
                <span>切换模型</span>
                <select
                  value={props.currentModel?.key || ''}
                  onChange={(event) => props.onSelectModelKey(event.target.value)}
                >
                  {props.modelOptions.map((item) => (
                    <option key={item.key} value={item.key}>
                      {item.name}
                    </option>
                  ))}
                </select>
              </label>
            ) : null}
            <button
              className="ghost-button live2d-stage-tool-button"
              type="button"
              onClick={() => setTransform({ ...props.defaultTransform })}
            >
              重置站位
            </button>
          </div>
        </div>
      </div>

      <div
        className="live2d-stage-shell"
        onPointerDown={handlePointerDown}
        onPointerMove={handlePointerMove}
        onPointerUp={handlePointerUp}
        onPointerCancel={handlePointerUp}
        onPointerLeave={handlePointerLeave}
      >
        {props.backgroundImageUrl && !backgroundLoadFailed ? (
          <img
            className="live2d-stage-background"
            src={props.backgroundImageUrl}
            alt=""
            aria-hidden="true"
            onError={() => setBackgroundLoadFailed(true)}
          />
        ) : null}
        <div className="live2d-stage-canvas" ref={hostRef} />

        <div className="live2d-stage-hint">移动鼠标可跟随视线，拖拽可移动站位，按住 Shift 拖拽可缩放。</div>
        <div className="live2d-stage-status">{stageStatus}</div>

        {props.loading || runtimeLoading ? (
          <div className="live2d-stage-overlay">
            <strong>{props.loading ? props.loadingText : `正在加载 ${props.currentModel?.name || '模型'}`}</strong>
            <span>舞台正在准备模型和资源，请稍等片刻。</span>
          </div>
        ) : null}

        {!props.loading && !runtimeLoading && activeErrorMessage ? (
          <div className="live2d-stage-overlay live2d-stage-overlay-error">
            <strong>模型加载失败</strong>
            <span>{activeErrorMessage}</span>
          </div>
        ) : null}

        <div className="live2d-stage-dialogue">
          <span className="section-kicker">{props.currentModel?.name || props.stageTitle}</span>
          <p className={props.isTyping ? 'live2d-stage-dialogue-text is-typing' : 'live2d-stage-dialogue-text'}>{props.dialogue}</p>
        </div>
      </div>

      <section className={`live2d-stage-control-drawer${controlDrawerOpen ? ' is-open' : ''}`}>
        <button
          className="live2d-stage-control-toggle"
          type="button"
          onClick={() => setControlDrawerOpen((current) => !current)}
        >
          <span>模型控制概览</span>
          <span>{metadata?.expressions.length || 0} 个表达式 · {metadata?.parameterIds.length || 0} 个参数</span>
        </button>
        {controlDrawerOpen ? (
          <div className="live2d-stage-control-body">
            <div className="live2d-stage-control-section">
              <strong>当前表达式</strong>
              <div className="live2d-stage-chip-row">
                {resolvedPreset.activeExpressionLabels.length ? (
                  resolvedPreset.activeExpressionLabels.map((label) => (
                    <span className="live2d-stage-chip live2d-stage-chip-active" key={label}>{label}</span>
                  ))
                ) : (
                  <span className="live2d-stage-chip">当前主要依赖参数混控</span>
                )}
              </div>
            </div>
            <div className="live2d-stage-control-section">
              <strong>当前参数</strong>
              <div className="live2d-stage-chip-row">
                {activeParameterEntries.length ? (
                  activeParameterEntries.map((item) => (
                    <span className="live2d-stage-chip" key={`${item.id}-${item.value}`}>
                      {item.id} {item.value.toFixed(2)}
                    </span>
                  ))
                ) : (
                  <span className="live2d-stage-chip">暂无额外参数覆盖</span>
                )}
              </div>
            </div>
          </div>
        ) : null}
      </section>
    </section>
  )
}

/**
 * 约束舞台拖拽与缩放结果，避免模型被移动到画布外或缩放到失真范围。
 */
function clampStageTransformValue(value: number, min: number, max: number): number {
  return Math.min(Math.max(value, min), max)
}
