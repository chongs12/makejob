import CompanionWorkspacePageImpl, { CompanionWorkspacePage as NamedCompanionWorkspacePage } from './CompanionPage'

/**
 * 提供学习陪伴房间页独立导出，便于后续继续把房间视图从共享实现中拆离。
 */
export function CompanionWorkspacePage() {
  return <NamedCompanionWorkspacePage />
}

export default CompanionWorkspacePageImpl
