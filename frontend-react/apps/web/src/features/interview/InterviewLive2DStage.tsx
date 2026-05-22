import type { Live2DDirective } from '../../shared/live2dDirective'
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

const INTERVIEW_STAGE_DEFAULT_TRANSFORM: Live2DStageTransform = {
  scale: 0.92,
  offsetX: 0,
  offsetY: 0.01,
}

/**
 * 将面试场景情绪编码转换成用户更容易理解的顶部状态文案。
 */
function formatInterviewEmotionLabel(emotion: string): string {
  switch (emotion.trim().toLowerCase()) {
    case 'serious':
      return '严肃'
    case 'thinking':
      return '思考'
    case 'encourage':
      return '鼓励'
    case 'praise':
      return '肯定'
    case 'warning':
      return '提醒'
    default:
      return emotion.trim() || '平稳'
  }
}

/**
 * 根据当前模型信息生成面试舞台顶部说明，突出来源和推荐命中方式。
 */
function buildInterviewStageNote(currentModelName: string, source: string, matchType: string): string {
  if (!currentModelName) {
    return '正在等待面试场景可用模型。'
  }

  return `当前已选择 ${currentModelName} · ${live2DSourceLabel(source)} / ${live2DMatchTypeLabel(matchType)}`
}

/**
 * 渲染面试页共享 Live2D 舞台，并保持与对话打字机和嘴型状态联动。
 */
export function InterviewLive2DStage(props: {
  industryCode: string
  dialogue: string
  isTyping: boolean
  emotion: string
  mouthOpen: number
  directive?: Live2DDirective | null
  selectedModelKey: string
  onChangeModelKey: (modelKey: string) => void
}) {
  const [selectedModelKey, setSelectedModelKey] = useState(() => props.selectedModelKey || readSelectedLive2DModelKey('interview', props.industryCode))

  const modelOptionsQuery = useQuery({
    queryKey: ['interview-live2d-models', props.industryCode],
    queryFn: () => fetchSelectableLive2DModels('interview', props.industryCode),
    staleTime: 60 * 1000,
  })

  useEffect(() => {
    setSelectedModelKey(props.selectedModelKey || readSelectedLive2DModelKey('interview', props.industryCode))
  }, [props.industryCode, props.selectedModelKey])

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

    persistSelectedLive2DModelKey('interview', props.industryCode, currentModel.key)
    props.onChangeModelKey(currentModel.key)
    if (currentModel.key !== selectedModelKey) {
      setSelectedModelKey(currentModel.key)
    }
  }, [currentModel?.key, props.industryCode, props.onChangeModelKey, selectedModelKey])

  const statusPills = useMemo(() => ([
    { label: '状态', value: formatInterviewEmotionLabel(props.emotion) },
    { label: '口型', value: `${Math.round(Math.max(0, Math.min(props.mouthOpen, 1)) * 100)}%` },
    { label: '输出', value: props.isTyping ? '生成中' : '待命' },
  ]), [props.emotion, props.isTyping, props.mouthOpen])
  const errorMessage = modelOptionsQuery.isError
    ? extractErrorMessage(modelOptionsQuery.error, '读取面试 Live2D 模型失败')
    : ''

  return (
    <Live2DSceneStage
      variant="interview"
      stageTitle="AI 面试官"
      stageNote={buildInterviewStageNote(currentModel?.name || '', currentModel?.source || '', currentModel?.match_type || '')}
      backgroundImageUrl={resolveSelectableLive2DBackgroundImageUrl(currentModel)}
      dialogue={props.dialogue}
      isTyping={props.isTyping}
      modelOptions={modelOptions}
      currentModel={currentModel}
      onSelectModelKey={setSelectedModelKey}
      preset={{
        scene: 'interview',
        emotion: props.emotion,
        mouthOpen: props.mouthOpen,
        directive: props.directive,
      }}
      statusPills={statusPills}
      loading={modelOptionsQuery.isLoading}
      loadingText={modelOptionsQuery.isLoading ? '正在读取面试场景模型' : `正在加载 ${currentModel?.name || '模型'}`}
      errorMessage={errorMessage}
      defaultTransform={INTERVIEW_STAGE_DEFAULT_TRANSFORM}
    />
  )
}
