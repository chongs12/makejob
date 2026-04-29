export type LoginPromptReason = 'missing' | 'expired'

export interface LoginPromptDetail {
  redirectTarget: string
  reason: LoginPromptReason
}

export const LOGIN_REQUIRED_PROMPT_EVENT_NAME = 'makejob:web-login-required'

/**
 * 在浏览器环境广播统一的登录提示事件，由根布局负责弹窗和跳转。
 */
export function requestLoginPrompt(redirectTarget: string, reason: LoginPromptReason = 'missing'): void {
  if (typeof window === 'undefined' || typeof window.dispatchEvent !== 'function') {
    return
  }

  window.dispatchEvent(new CustomEvent<LoginPromptDetail>(LOGIN_REQUIRED_PROMPT_EVENT_NAME, {
    detail: {
      redirectTarget: redirectTarget.trim() || '/',
      reason,
    },
  }))
}
