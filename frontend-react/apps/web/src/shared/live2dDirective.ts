export interface Live2DDirectiveExpressionLayer {
  key: string
  weight: number
}

export interface Live2DDirectiveParameterOverride {
  id: string
  value: number
}

export interface Live2DDirective {
  reply?: string
  emotion?: string
  action?: string
  expression_mix?: Live2DDirectiveExpressionLayer[]
  parameter_overrides?: Live2DDirectiveParameterOverride[]
  motion_key?: string
  motion_group?: string
  motion_priority?: 'normal' | 'force' | string
  motion_duration_ms?: number
  intensity?: number
  duration_ms?: number
  mouth_open?: number
  source?: string
}
