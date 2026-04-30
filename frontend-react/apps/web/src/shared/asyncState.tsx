import type { CSSProperties, ReactNode } from 'react'

interface AsyncInlineStateProps {
  message: ReactNode
  tone?: 'default' | 'error'
  className?: string
  style?: CSSProperties
}

interface AsyncStatusCardProps {
  title?: ReactNode
  message: ReactNode
  tone?: 'default' | 'error'
  className?: string
  style?: CSSProperties
  action?: ReactNode
}

interface AsyncEmptyStateProps {
  title: ReactNode
  message: ReactNode
  className?: string
  style?: CSSProperties
  action?: ReactNode
}

/**
 * 渲染轻量级异步状态文案，统一列表或卡片内部的加载与报错提示样式。
 */
export function AsyncInlineState(props: AsyncInlineStateProps) {
  return (
    <p
      className={`async-state-inline${props.tone === 'error' ? ' async-state-inline-error' : ''}${props.className ? ` ${props.className}` : ''}`}
      style={props.style}
    >
      {props.message}
    </p>
  )
}

/**
 * 渲染标准状态卡片，用于整块区域的加载中或失败提示。
 */
export function AsyncStatusCard(props: AsyncStatusCardProps) {
  return (
    <div
      className={`status-card async-state-card${props.tone === 'error' ? ' async-state-card-error' : ''}${props.className ? ` ${props.className}` : ''}`}
      style={props.style}
    >
      {props.title ? <strong>{props.title}</strong> : null}
      <p className="async-state-card-message">{props.message}</p>
      {props.action ? <div className="async-state-action">{props.action}</div> : null}
    </div>
  )
}

/**
 * 渲染统一的空状态块，避免各页面重复手写“暂无数据”结构。
 */
export function AsyncEmptyState(props: AsyncEmptyStateProps) {
  return (
    <div className={`timeline-item async-state-empty${props.className ? ` ${props.className}` : ''}`} style={props.style}>
      <strong>{props.title}</strong>
      <p>{props.message}</p>
      {props.action ? <div className="async-state-action">{props.action}</div> : null}
    </div>
  )
}
