import { useEffect, useRef } from 'react'
import { syncCompanionStudyLog } from './companionApi'
import { buildCompanionStudyLogPayload } from './companionHelpers'
import type {
  CompanionDailyDigest,
  CompanionPlanDetail,
  CompanionPlanTask,
} from './companionTypes'

/**
 * 在陪伴入口页和房间页内复用每日摘要自动同步逻辑，避免重复提交相同内容。
 */
export function useCompanionStudyLogSync(
  accessToken: string | null,
  plan: CompanionPlanDetail | null,
  digest: CompanionDailyDigest | null,
  focusedTask: CompanionPlanTask | null,
): void {
  const syncedSignatureRef = useRef('')
  const pendingSignatureRef = useRef('')

  useEffect(() => {
    const payload = accessToken ? buildCompanionStudyLogPayload(plan, digest, focusedTask) : null
    if (!accessToken || !payload) {
      pendingSignatureRef.current = ''
      return
    }

    const signature = JSON.stringify(payload)
    if (syncedSignatureRef.current === signature || pendingSignatureRef.current === signature) {
      return
    }

    pendingSignatureRef.current = signature
    void syncCompanionStudyLog(accessToken, payload)
      .then(() => {
        syncedSignatureRef.current = signature
      })
      .catch(() => {
        // 后台静默同步失败时不打断当前陪伴流程，后续有新动作会再次尝试。
      })
      .finally(() => {
        if (pendingSignatureRef.current === signature) {
          pendingSignatureRef.current = ''
        }
      })
  }, [accessToken, digest, focusedTask, plan])
}
