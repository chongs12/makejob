import { Component, type ErrorInfo, type ReactNode } from 'react'

interface SectionErrorBoundaryProps {
  children: ReactNode
  title: string
  description: string
  retryLabel?: string
  className?: string
  onRetry?: () => void
  resetKeys?: unknown[]
}

interface SectionErrorBoundaryState {
  hasError: boolean
}

/**
 * 比较两组重置键是否发生变化，供分区错误边界判断何时自动恢复。
 */
function areResetKeysEqual(previousKeys: unknown[], nextKeys: unknown[]): boolean {
  if (previousKeys.length !== nextKeys.length) {
    return false
  }

  return previousKeys.every((value, index) => Object.is(value, nextKeys[index]))
}

/**
 * 为局部业务分区提供错误兜底，避免单块渲染异常拖垮整个页面。
 */
export class SectionErrorBoundary extends Component<SectionErrorBoundaryProps, SectionErrorBoundaryState> {
  state: SectionErrorBoundaryState = {
    hasError: false,
  }

  /**
   * 在子树抛出渲染异常后切换到降级视图，保证页面其余区域仍可继续使用。
   */
  static getDerivedStateFromError(): SectionErrorBoundaryState {
    return {
      hasError: true,
    }
  }

  /**
   * 记录当前分区的渲染异常，便于开发阶段定位具体失败位置。
   */
  componentDidCatch(error: Error, errorInfo: ErrorInfo): void {
    console.error('SectionErrorBoundary captured an error:', error, errorInfo)
  }

  /**
   * 当外部关键参数变化时自动清空错误态，允许分区在新上下文下重新挂载。
   */
  componentDidUpdate(previousProps: SectionErrorBoundaryProps): void {
    const previousKeys = previousProps.resetKeys || []
    const nextKeys = this.props.resetKeys || []

    if (this.state.hasError && !areResetKeysEqual(previousKeys, nextKeys)) {
      this.setState({
        hasError: false,
      })
    }
  }

  /**
   * 手动重试当前分区渲染，并在需要时联动执行外部恢复逻辑。
   */
  handleRetry = (): void => {
    this.props.onRetry?.()
    this.setState({
      hasError: false,
    })
  }

  render() {
    if (!this.state.hasError) {
      return this.props.children
    }

    return (
      <section className={this.props.className}>
        <article className="status-card">
          <div className="companion-card-head">
            <div>
              <span className="section-kicker">分区异常</span>
              <h2>{this.props.title}</h2>
            </div>
          </div>
          <p className="companion-empty-text">{this.props.description}</p>
          <div className="page-actions">
            <button className="primary-button" type="button" onClick={this.handleRetry}>
              {this.props.retryLabel || '重新加载'}
            </button>
          </div>
        </article>
      </section>
    )
  }
}
