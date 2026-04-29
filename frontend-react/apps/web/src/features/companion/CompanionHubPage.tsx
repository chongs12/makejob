import { CompanionHubPage as CompanionHubPageImpl } from './CompanionPage'

/**
 * 提供学习陪伴入口页独立导出，便于后续继续把入口视图从共享实现中拆离。
 */
export function CompanionHubPage() {
  return <CompanionHubPageImpl />
}
