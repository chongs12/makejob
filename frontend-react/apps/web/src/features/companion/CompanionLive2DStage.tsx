import { useEffect, useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { extractErrorMessage } from '@makejob/api-client'
import { Live2DSceneStage } from '../../shared/Live2DSceneStage'
import {
  fetchSelectableLive2DModels,
  live2DMatchTypeLabel,
  live2DSourceLabel,
  persistSelectedLive2DModelKey,
  readSelectedLive2DModelKey,
  resolveSelectableLive2DBackgroundImageUrl,
} from '../../shared/live2dModelCatalog'
import type { Live2DStageTransform } from '../../shared/live2dStageRuntime'
import { buildCompanionLive2DModelsQueryKey } from '../../shared/queryKeys'

const COMPANION_STAGE_DEFAULT_TRANSFORM: Live2DStageTransform = {
  scale: 0.96,
  offsetX: 0,
  offsetY: 0,
}

/**
 * 将陪伴页情绪编码转换成更直观的舞台状态文案。
 */
function formatCompanionEmotionLabel(emotion: string): string {
  switch (emotion.trim().toLowerCase()) {
    case 'happy':
    case 'praise':
      return '积极'
    case 'encouraging':
    case 'encourage':
      return '鼓励'
    case 'thinking':
      return '思考'
    case 'warning':
    case 'serious':
      return '严肃'
    case 'tired':
      return '疲惫'
    default:
      return emotion.trim() || '平稳'
  }
}

/**
 * 将陪伴页动作标签转换成顶部状态条使用的可读中文。
 */
function formatCompanionActionLabel(action: string): string {
  switch (action.trim().toLowerCase()) {
    case 'wave':
      return '挥手'
    case 'nod':
      return '点头'
    case 'celebrate':
      return '庆祝'
    case 'thinking':
      return '思考'
    default:
      return action.trim() || '待机'
  }
}

/**
 * 构造陪伴舞台顶部说明，集中描述当前模型来源与命中方式。
 */
function buildCompanionStageNote(loggedIn: boolean, currentModelName: string, source: string, matchType: string): string {
  if (!currentModelName) {
    return loggedIn ? '当前计划已接入，正在等待可用模型。' : '当前未登录，舞台处于本地展示模式。'
  }

  return `当前已选择 ${currentModelName} · ${live2DSourceLabel(source)} / ${live2DMatchTypeLabel(matchType)}`
}

/**
 * 渲染陪伴页共享 Live2D 舞台，并复用统一模型切换与表情控制链路。
 */
export function CompanionLive2DStage(props: {
  dialogue: string
  emotion: string
  action: string
  loggedIn: boolean
  industryCode: string
}) {
  const [selectedModelKey, setSelectedModelKey] = useState(() => readSelectedLive2DModelKey('companion', props.industryCode))

  const modelOptionsQuery = useQuery({
    queryKey: buildCompanionLive2DModelsQueryKey(props.industryCode),
    queryFn: () => fetchSelectableLive2DModels('companion', props.industryCode),
    staleTime: 60 * 1000,
  })

  useEffect(() => {
    setSelectedModelKey(readSelectedLive2DModelKey('companion', props.industryCode))
  }, [props.industryCode])

  const modelOptions = modelOptionsQuery.data || []
  const currentModel = useMemo(() => {
    const explicitModel = modelOptions.find((item) => item.key === selectedModelKey)
    if (explicitModel) {
      return explicitModel
    }

    return modelOptions.find((item) => item.is_recommended) || modelOptions[0] || null
  }, [modelOptions, selectedModelKey])

  useEffect(() => {
    if (!currentModel?.key) {
      return
    }

    persistSelectedLive2DModelKey('companion', props.industryCode, currentModel.key)
    if (currentModel.key !== selectedModelKey) {
      setSelectedModelKey(currentModel.key)
    }
  }, [currentModel?.key, props.industryCode, selectedModelKey])

  const currentModelName = currentModel?.name || ''
  const stageNote = buildCompanionStageNote(
    props.loggedIn,
    currentModelName,
    currentModel?.source || '',
    currentModel?.match_type || '',
  )
  const statusPills = useMemo(() => ([
    { label: '情绪', value: formatCompanionEmotionLabel(props.emotion) },
    { label: '动作', value: formatCompanionActionLabel(props.action) },
    { label: '连接', value: props.loggedIn ? '已接入计划' : '展示模式' },
  ]), [props.action, props.emotion, props.loggedIn])
  const errorMessage = modelOptionsQuery.isError
    ? extractErrorMessage(modelOptionsQuery.error, '读取陪伴页 Live2D 模型列表失败')
    : ''

  return (
    <Live2DSceneStage
      variant="companion"
      stageTitle="学习陪伴"
      stageNote={stageNote}
      backgroundImageUrl={resolveSelectableLive2DBackgroundImageUrl(currentModel)}
      dialogue={props.dialogue}
      modelOptions={modelOptions}
      currentModel={currentModel}
      onSelectModelKey={setSelectedModelKey}
      preset={{
        scene: 'companion',
        emotion: props.emotion,
        action: props.action,
      }}
      statusPills={statusPills}
      loading={modelOptionsQuery.isLoading}
      loadingText={modelOptionsQuery.isLoading ? '正在读取陪伴场景模型' : `正在加载 ${currentModel?.name || '模型'}`}
      errorMessage={errorMessage}
      defaultTransform={COMPANION_STAGE_DEFAULT_TRANSFORM}
    />
  )
}
