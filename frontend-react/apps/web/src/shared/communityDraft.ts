export interface CommunityDraftPayload {
  postType: string
  title: string
  content: string
  tags: string[]
}

const COMMUNITY_DRAFT_KEY = 'makejob.community.create-draft'

/**
 * 暂存社区发帖草稿，供其他页面跳转到发帖页时自动预填内容。
 */
export function persistCommunityDraft(payload: CommunityDraftPayload): void {
  if (typeof window === 'undefined') {
    return
  }

  window.localStorage.setItem(COMMUNITY_DRAFT_KEY, JSON.stringify(payload))
}

/**
 * 读取当前待使用的社区草稿，并在数据损坏时自动清理。
 */
export function readCommunityDraft(): CommunityDraftPayload | null {
  if (typeof window === 'undefined') {
    return null
  }

  try {
    const raw = window.localStorage.getItem(COMMUNITY_DRAFT_KEY)
    if (!raw) {
      return null
    }

    const parsed = JSON.parse(raw) as Partial<CommunityDraftPayload>
    return {
      postType: parsed.postType?.trim() || 'article',
      title: parsed.title?.trim() || '',
      content: parsed.content?.trim() || '',
      tags: Array.isArray(parsed.tags) ? parsed.tags.map((item) => String(item).trim()).filter(Boolean) : [],
    }
  } catch {
    clearCommunityDraft()
    return null
  }
}

/**
 * 清空已消费的社区草稿，避免旧内容再次污染发帖表单。
 */
export function clearCommunityDraft(): void {
  if (typeof window === 'undefined') {
    return
  }

  window.localStorage.removeItem(COMMUNITY_DRAFT_KEY)
}
