// Live2DModelConfig 描述前端渲染层使用的模型配置。
export interface Live2DModelConfig {
  name: string
  path: string
  modelUrl?: string
  thumbnailUrl?: string
  scene?: string
  industryCode?: string
  source?: 'database' | 'bundled'
  config?: Record<string, unknown>
  scale?: number
  position?: { x: number; y: number }
  expressions?: string[]
  motions?: Record<string, string[]>
}

// Live2DState 描述组件内部的基本状态。
export interface Live2DState {
  isLoaded: boolean
  currentExpression: string
  currentMotion: string
  isSpeaking: boolean
}

// Live2DInterviewerProps 描述面试官组件入参。
export interface Live2DInterviewerProps {
  modelConfig?: Live2DModelConfig | null
  width?: number
  height?: number
  autoSpeak?: boolean
  loading?: boolean
  speaking?: boolean
  error?: string
}

// Live2DCompanionProps 描述陪伴组件入参。
export interface Live2DCompanionProps {
  modelConfig?: Live2DModelConfig | null
  width?: number
  height?: number
  mood?: 'happy' | 'neutral' | 'thinking' | 'encouraging'
  loading?: boolean
  error?: string
}
