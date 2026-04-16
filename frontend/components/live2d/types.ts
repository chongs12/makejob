// Live2D 相关类型定义

export interface Live2DModelConfig {
  name: string
  path: string
  scale?: number
  position?: { x: number; y: number }
  expressions?: string[]
  motions?: Record<string, string[]>
}

export interface Live2DState {
  isLoaded: boolean
  currentExpression: string
  currentMotion: string
  isSpeaking: boolean
}

export interface Live2DInterviewerProps {
  modelConfig?: Live2DModelConfig
  width?: number
  height?: number
  autoSpeak?: boolean
}

export interface Live2DCompanionProps {
  modelConfig?: Live2DModelConfig
  width?: number
  height?: number
  mood?: 'happy' | 'neutral' | 'thinking' | 'encouraging'
}
