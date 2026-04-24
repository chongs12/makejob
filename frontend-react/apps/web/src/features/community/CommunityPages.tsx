import type { FormEvent } from 'react'
import { useEffect, useMemo, useState } from 'react'
import { Link, useNavigate, useParams } from '@tanstack/react-router'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { extractErrorMessage, requestJson } from '@makejob/api-client'
import { isSuccessCode, type ApiEnvelope } from '@makejob/shared-types'
import { buildLoginRedirectSearch, readCurrentBrowserPath } from '../../shared/authRedirect'
import { clearCommunityDraft, readCommunityDraft } from '../../shared/communityDraft'
import { useAuthStore } from '../../state/auth'

interface PageResult<T> {
  list: T[]
  total: number
  page: number
  page_size: number
}

interface CommunityPostAuthor {
  id: number
  username: string
  avatar?: string
  role?: string
}

interface CommunityPostItem {
  id: number
  post_type: string
  title: string
  content: string
  summary: string
  tags: string[]
  view_count: number
  comment_count: number
  like_count: number
  is_pinned: boolean
  is_recommended: boolean
  created_at: string
  updated_at: string
  is_liked: boolean
  is_author: boolean
  author: CommunityPostAuthor
}

interface CommunityCommentItem {
  id: number
  content: string
  created_at: string
  updated_at: string
  is_author: boolean
  author: CommunityPostAuthor
}

interface CommunityLikeToggleResponse {
  liked: boolean
  like_count: number
}

interface CommunityPostPayload {
  post_type: string
  title: string
  content: string
  tags: string[]
}

const COMMUNITY_PAGE_SIZE = 10

/**
 * 统一格式化社区页使用的时间文本，避免各卡片展示口径不一致。
 */
function formatCommunityDateTime(value?: string): string {
  if (!value) {
    return '-'
  }

  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return value
  }

  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  const hour = String(date.getHours()).padStart(2, '0')
  const minute = String(date.getMinutes()).padStart(2, '0')
  return `${year}-${month}-${day} ${hour}:${minute}`
}

/**
 * 将社区正文裁剪成列表卡片可承载的摘要长度。
 */
function truncateCommunityText(value: string, maxLength: number): string {
  const normalized = value.trim()
  if (normalized.length <= maxLength) {
    return normalized
  }

  return `${normalized.slice(0, maxLength)}...`
}

/**
 * 规范化标签输入，兼容中文逗号、空格和重复值。
 */
function normalizeCommunityTags(raw: string): string[] {
  return raw
    .split(/[,，]/)
    .map((item) => item.trim())
    .filter(Boolean)
    .filter((item, index, list) => list.findIndex((entry) => entry.toLowerCase() === item.toLowerCase()) === index)
    .slice(0, 5)
}

/**
 * 拉取社区帖子分页列表，并在已登录时附带个性化状态。
 */
async function fetchCommunityPosts(params: {
  page: number
  pageSize: number
  postType: string
  keyword: string
  tag: string
  token?: string | null
}): Promise<PageResult<CommunityPostItem>> {
  const searchParams = new URLSearchParams({
    page: String(params.page),
    page_size: String(params.pageSize),
  })

  if (params.postType) {
    searchParams.set('type', params.postType)
  }
  if (params.keyword) {
    searchParams.set('keyword', params.keyword)
  }
  if (params.tag) {
    searchParams.set('tag', params.tag)
  }

  const response = await requestJson<ApiEnvelope<PageResult<CommunityPostItem>>>(`/community/posts?${searchParams.toString()}`, {
    token: params.token || undefined,
  })
  if (!isSuccessCode(response.code) || !response.data) {
    throw new Error(response.message || '获取社区帖子失败')
  }

  return response.data
}

/**
 * 拉取社区帖子详情，用于详情页和编辑页预填充。
 */
async function fetchCommunityPostDetail(postId: string, token?: string | null): Promise<CommunityPostItem> {
  const response = await requestJson<ApiEnvelope<CommunityPostItem>>(`/community/posts/${postId}`, {
    token: token || undefined,
  })
  if (!isSuccessCode(response.code) || !response.data) {
    throw new Error(response.message || '获取帖子详情失败')
  }

  return response.data
}

/**
 * 拉取当前登录用户自己的帖子列表。
 */
async function fetchMyCommunityPosts(token: string, params: {
  page: number
  pageSize: number
  postType: string
  keyword: string
  tag: string
}): Promise<PageResult<CommunityPostItem>> {
  const searchParams = new URLSearchParams({
    page: String(params.page),
    page_size: String(params.pageSize),
  })

  if (params.postType) {
    searchParams.set('type', params.postType)
  }
  if (params.keyword) {
    searchParams.set('keyword', params.keyword)
  }
  if (params.tag) {
    searchParams.set('tag', params.tag)
  }

  const response = await requestJson<ApiEnvelope<PageResult<CommunityPostItem>>>(`/community/my-posts?${searchParams.toString()}`, {
    token,
  })
  if (!isSuccessCode(response.code) || !response.data) {
    throw new Error(response.message || '获取我的帖子失败')
  }

  return response.data
}

/**
 * 创建社区帖子并返回最新帖子数据。
 */
async function createCommunityPost(token: string, payload: CommunityPostPayload): Promise<CommunityPostItem> {
  const response = await requestJson<ApiEnvelope<CommunityPostItem>>('/community/posts', {
    method: 'POST',
    token,
    body: payload,
  })
  if (!isSuccessCode(response.code) || !response.data) {
    throw new Error(response.message || '发布帖子失败')
  }

  return response.data
}

/**
 * 更新社区帖子并返回最新详情。
 */
async function updateCommunityPost(token: string, postId: string, payload: CommunityPostPayload): Promise<CommunityPostItem> {
  const response = await requestJson<ApiEnvelope<CommunityPostItem>>(`/community/posts/${postId}`, {
    method: 'PUT',
    token,
    body: payload,
  })
  if (!isSuccessCode(response.code) || !response.data) {
    throw new Error(response.message || '更新帖子失败')
  }

  return response.data
}

/**
 * 删除社区帖子并返回成功结果。
 */
async function deleteCommunityPost(token: string, postId: string): Promise<void> {
  const response = await requestJson<ApiEnvelope<{ id: number }>>(`/community/posts/${postId}`, {
    method: 'DELETE',
    token,
  })
  if (!isSuccessCode(response.code)) {
    throw new Error(response.message || '删除帖子失败')
  }
}

/**
 * 拉取帖子评论列表，支持公开浏览和登录态标识。
 */
async function fetchCommunityComments(postId: string, token?: string | null): Promise<CommunityCommentItem[]> {
  const response = await requestJson<ApiEnvelope<CommunityCommentItem[]>>(`/community/posts/${postId}/comments`, {
    token: token || undefined,
  })
  if (!isSuccessCode(response.code)) {
    throw new Error(response.message || '获取评论失败')
  }

  return response.data || []
}

/**
 * 创建帖子评论并返回最新评论内容。
 */
async function createCommunityComment(token: string, postId: string, content: string): Promise<CommunityCommentItem> {
  const response = await requestJson<ApiEnvelope<CommunityCommentItem>>(`/community/posts/${postId}/comments`, {
    method: 'POST',
    token,
    body: { content },
  })
  if (!isSuccessCode(response.code) || !response.data) {
    throw new Error(response.message || '发表评论失败')
  }

  return response.data
}

/**
 * 切换帖子点赞状态并返回最新点赞计数。
 */
async function toggleCommunityLike(token: string, postId: string): Promise<CommunityLikeToggleResponse> {
  const response = await requestJson<ApiEnvelope<CommunityLikeToggleResponse>>(`/community/posts/${postId}/like`, {
    method: 'POST',
    token,
  })
  if (!isSuccessCode(response.code) || !response.data) {
    throw new Error(response.message || '点赞操作失败')
  }

  return response.data
}

/**
 * 统一跳转登录页并保留当前社区页面地址，供登录后原路返回。
 */
function redirectToCommunityLogin(navigate: ReturnType<typeof useNavigate>): void {
  navigate({
    to: '/auth/login',
    search: buildLoginRedirectSearch(readCurrentBrowserPath()),
  })
}

/**
 * 渲染社区统一发帖表单，供发布和编辑两种场景共用。
 */
function CommunityPostEditor(props: {
  title: string
  subtitle: string
  submitLabel: string
  initialData?: CommunityPostItem | null
  initialPayload?: Partial<CommunityPostPayload> | null
  pending?: boolean
  onSubmit: (payload: CommunityPostPayload) => Promise<void>
}) {
  const [postType, setPostType] = useState(props.initialData?.post_type || props.initialPayload?.post_type || 'article')
  const [title, setTitle] = useState(props.initialData?.title || props.initialPayload?.title || '')
  const [content, setContent] = useState(props.initialData?.content || props.initialPayload?.content || '')
  const [tagsInput, setTagsInput] = useState((props.initialData?.tags || props.initialPayload?.tags || []).join(', '))
  const [message, setMessage] = useState('填写完成后即可提交')

  useEffect(() => {
    setPostType(props.initialData?.post_type || props.initialPayload?.post_type || 'article')
    setTitle(props.initialData?.title || props.initialPayload?.title || '')
    setContent(props.initialData?.content || props.initialPayload?.content || '')
    setTagsInput((props.initialData?.tags || props.initialPayload?.tags || []).join(', '))
  }, [props.initialData, props.initialPayload])

  /**
   * 提交帖子表单，并将表单校验信息反馈给用户。
   */
  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()

    const payload: CommunityPostPayload = {
      post_type: postType,
      title: title.trim(),
      content: content.trim(),
      tags: normalizeCommunityTags(tagsInput),
    }

    if (!payload.content) {
      setMessage('请输入帖子内容')
      return
    }

    if (payload.post_type === 'article' && !payload.title) {
      setMessage('文章类型需要填写标题')
      return
    }

    try {
      await props.onSubmit(payload)
      setMessage('提交成功')
    } catch (error) {
      setMessage(extractErrorMessage(error, '提交失败，请稍后重试'))
    }
  }

  return (
    <section className="page-panel community-page-panel">
      <span className="page-tag">社区发布</span>
      <h1>{props.title}</h1>
      <p className="page-copy">{props.subtitle}</p>

      <form className="stack-form" onSubmit={handleSubmit}>
        <div className="community-filter-grid">
          <label className="field">
            <span>内容类型</span>
            <select value={postType} onChange={(event) => setPostType(event.target.value)}>
              <option value="article">文章</option>
              <option value="moment">动态</option>
            </select>
          </label>
          <label className="field">
            <span>标签</span>
            <input
              value={tagsInput}
              onChange={(event) => setTagsInput(event.target.value)}
              placeholder="最多 5 个，使用逗号分隔"
            />
          </label>
        </div>

        <label className="field">
          <span>标题</span>
          <input
            value={title}
            onChange={(event) => setTitle(event.target.value)}
            maxLength={120}
            placeholder={postType === 'article' ? '请输入文章标题' : '动态标题可留空'}
          />
        </label>

        <label className="field">
          <span>正文内容</span>
          <textarea
            value={content}
            onChange={(event) => setContent(event.target.value)}
            maxLength={5000}
            placeholder="请输入帖子正文，支持经验总结、问题讨论和刷题复盘"
          />
        </label>

        <div className="card-inline">
          <button className="primary-button" type="submit" disabled={props.pending}>
            {props.pending ? '提交中...' : props.submitLabel}
          </button>
          <span className="community-editor-tip">内容上限 5000 字，标签会自动去重并限制为 5 个。</span>
        </div>
      </form>

      <div className="status-card" style={{ marginTop: 24 }}>{message}</div>
    </section>
  )
}

/**
 * 提供社区广场页，承接公开帖子列表、搜索和筛选能力。
 */
export function CommunityPage() {
  const navigate = useNavigate()
  const accessToken = useAuthStore((state) => state.accessToken)
  const [page, setPage] = useState(1)
  const [postType, setPostType] = useState('')
  const [keywordInput, setKeywordInput] = useState('')
  const [keyword, setKeyword] = useState('')
  const [tagInput, setTagInput] = useState('')
  const [tag, setTag] = useState('')

  const postsQuery = useQuery({
    queryKey: ['community-posts', page, postType, keyword, tag, accessToken],
    queryFn: () => fetchCommunityPosts({
      page,
      pageSize: COMMUNITY_PAGE_SIZE,
      postType,
      keyword,
      tag,
      token: accessToken,
    }),
  })

  /**
   * 提交社区筛选条件，并重置到第一页重新查询。
   */
  function handleFilterSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setPage(1)
    setKeyword(keywordInput.trim())
    setTag(tagInput.trim())
  }

  return (
    <section className="page-panel community-page-panel">
      <div className="section-head">
        <div>
          <span className="page-tag">社区广场</span>
          <h1>刷题经验、面经总结和问题讨论都在这里沉淀</h1>
          <p className="page-copy">现在首页动态流只做轻入口，完整浏览、搜索、发帖、评论和互动都统一收口到社区频道。</p>
        </div>
        <div className="page-actions">
          <button className="primary-button" type="button" onClick={() => navigate({ to: '/community/create' })}>
            发布帖子
          </button>
          {accessToken ? (
            <button className="secondary-button" type="button" onClick={() => navigate({ to: '/community/mine' })}>
              我的帖子
            </button>
          ) : (
            <button className="secondary-button" type="button" onClick={() => redirectToCommunityLogin(navigate)}>
              登录后发帖
            </button>
          )}
        </div>
      </div>

      <form className="stack-form" onSubmit={handleFilterSubmit}>
        <div className="community-filter-grid">
          <label className="field">
            <span>内容类型</span>
            <select value={postType} onChange={(event) => { setPostType(event.target.value); setPage(1) }}>
              <option value="">全部</option>
              <option value="article">文章</option>
              <option value="moment">动态</option>
            </select>
          </label>
          <label className="field">
            <span>关键词</span>
            <input value={keywordInput} onChange={(event) => setKeywordInput(event.target.value)} placeholder="搜索标题或正文" />
          </label>
          <label className="field">
            <span>标签</span>
            <input value={tagInput} onChange={(event) => setTagInput(event.target.value)} placeholder="按标签筛选" />
          </label>
        </div>
        <div className="page-actions">
          <button className="secondary-button" type="submit">应用筛选</button>
        </div>
      </form>

      {postsQuery.isLoading ? <div className="status-card" style={{ marginTop: 24 }}>社区帖子加载中...</div> : null}
      {postsQuery.isError ? (
        <div className="status-card" style={{ marginTop: 24 }}>{extractErrorMessage(postsQuery.error, '社区帖子加载失败')}</div>
      ) : null}

      {postsQuery.data?.list?.length ? (
        <>
          <div className="community-post-list">
            {postsQuery.data.list.map((post) => (
              <article className="feature-card community-post-card" key={post.id}>
                <div className="community-post-head">
                  <div>
                    <span className="section-kicker">{post.post_type === 'article' ? '文章' : '动态'}</span>
                    <h2>{post.title || truncateCommunityText(post.summary || post.content, 28)}</h2>
                  </div>
                  <span>{formatCommunityDateTime(post.created_at)}</span>
                </div>
                <p>{truncateCommunityText(post.summary || post.content, 140)}</p>
                <div className="community-tag-row">
                  {post.tags.map((item) => (
                    <span key={`${post.id}-${item}`}>{item}</span>
                  ))}
                </div>
                <div className="community-meta-row">
                  <span>{post.author?.username || '匿名用户'}</span>
                  <span>浏览 {post.view_count}</span>
                  <span>评论 {post.comment_count}</span>
                  <span>点赞 {post.like_count}</span>
                </div>
                <div className="page-actions">
                  <Link className="secondary-link" to="/community/$postId" params={{ postId: String(post.id) }}>
                    查看详情
                  </Link>
                  {post.is_author ? (
                    <Link className="secondary-link" to="/community/$postId/edit" params={{ postId: String(post.id) }}>
                      编辑
                    </Link>
                  ) : null}
                </div>
              </article>
            ))}
          </div>

          <div className="card-inline" style={{ marginTop: 24 }}>
            <span>共 {postsQuery.data.total} 条社区内容</span>
            <div className="page-actions">
              <button className="secondary-button" type="button" disabled={page <= 1} onClick={() => setPage((current) => current - 1)}>
                上一页
              </button>
              <span>第 {page} 页</span>
              <button
                className="secondary-button"
                type="button"
                disabled={postsQuery.data.list.length < COMMUNITY_PAGE_SIZE}
                onClick={() => setPage((current) => current + 1)}
              >
                下一页
              </button>
            </div>
          </div>
        </>
      ) : postsQuery.data ? (
        <div className="timeline-item" style={{ marginTop: 24 }}>
          <strong>当前筛选条件下还没有帖子</strong>
          <p>你可以先发布一条刷题复盘、面经记录或问题讨论，社区入口已经完整接通。</p>
        </div>
      ) : null}
    </section>
  )
}

/**
 * 提供社区详情页，承接点赞、评论和作者编辑删除动作。
 */
export function CommunityPostDetailPage() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const accessToken = useAuthStore((state) => state.accessToken)
  const params = useParams({ strict: false })
  const postId = String(params.postId || '')
  const [commentInput, setCommentInput] = useState('')
  const [message, setMessage] = useState('欢迎参与讨论')
  const [pending, setPending] = useState(false)

  const detailQuery = useQuery({
    queryKey: ['community-post', postId, accessToken],
    queryFn: () => fetchCommunityPostDetail(postId, accessToken),
    enabled: Boolean(postId),
  })

  const commentsQuery = useQuery({
    queryKey: ['community-post-comments', postId, accessToken],
    queryFn: () => fetchCommunityComments(postId, accessToken),
    enabled: Boolean(postId),
  })

  /**
   * 刷新与当前帖子相关的社区缓存，避免详情和列表状态不一致。
   */
  async function refreshCommunityQueries() {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: ['community-post', postId] }),
      queryClient.invalidateQueries({ queryKey: ['community-post-comments', postId] }),
      queryClient.invalidateQueries({ queryKey: ['community-posts'] }),
      queryClient.invalidateQueries({ queryKey: ['community-my-posts'] }),
      queryClient.invalidateQueries({ queryKey: ['home-community-posts'] }),
    ])
  }

  /**
   * 处理帖子点赞动作，并在失效登录态下回跳到登录页。
   */
  async function handleToggleLike() {
    if (!accessToken) {
      redirectToCommunityLogin(navigate)
      return
    }

    setPending(true)
    try {
      const result = await toggleCommunityLike(accessToken, postId)
      setMessage(result.liked ? '已点赞这条帖子' : '已取消点赞')
      await refreshCommunityQueries()
    } catch (error) {
      if (!useAuthStore.getState().accessToken) {
        redirectToCommunityLogin(navigate)
        return
      }
      setMessage(extractErrorMessage(error, '点赞操作失败'))
    } finally {
      setPending(false)
    }
  }

  /**
   * 提交评论内容，并在成功后刷新评论列表和帖子计数。
   */
  async function handleCommentSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()

    if (!accessToken) {
      redirectToCommunityLogin(navigate)
      return
    }

    if (!commentInput.trim()) {
      setMessage('请输入评论内容')
      return
    }

    setPending(true)
    try {
      await createCommunityComment(accessToken, postId, commentInput.trim())
      setCommentInput('')
      setMessage('评论已发布')
      await refreshCommunityQueries()
    } catch (error) {
      if (!useAuthStore.getState().accessToken) {
        redirectToCommunityLogin(navigate)
        return
      }
      setMessage(extractErrorMessage(error, '发表评论失败'))
    } finally {
      setPending(false)
    }
  }

  /**
   * 删除作者自己的帖子，并在成功后返回社区广场。
   */
  async function handleDeletePost() {
    if (!accessToken) {
      redirectToCommunityLogin(navigate)
      return
    }

    if (typeof window !== 'undefined' && !window.confirm('确认删除这条帖子吗？删除后无法恢复。')) {
      return
    }

    setPending(true)
    try {
      await deleteCommunityPost(accessToken, postId)
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['community-posts'] }),
        queryClient.invalidateQueries({ queryKey: ['community-my-posts'] }),
        queryClient.invalidateQueries({ queryKey: ['home-community-posts'] }),
      ])
      navigate({ to: '/community' })
    } catch (error) {
      if (!useAuthStore.getState().accessToken) {
        redirectToCommunityLogin(navigate)
        return
      }
      setMessage(extractErrorMessage(error, '删除帖子失败'))
    } finally {
      setPending(false)
    }
  }

  if (detailQuery.isLoading) {
    return <section className="page-panel community-page-panel"><div className="status-card">帖子详情加载中...</div></section>
  }

  if (detailQuery.isError || !detailQuery.data) {
    return (
      <section className="page-panel community-page-panel">
        <div className="status-card">{extractErrorMessage(detailQuery.error, '帖子详情加载失败')}</div>
      </section>
    )
  }

  const post = detailQuery.data

  return (
    <section className="page-panel community-page-panel">
      <div className="page-actions" style={{ marginBottom: 16 }}>
        <Link className="secondary-link" to="/community">返回社区</Link>
        {post.is_author ? (
          <Link className="secondary-link" to="/community/$postId/edit" params={{ postId }}>
            编辑帖子
          </Link>
        ) : null}
      </div>

      <article className="section-card section-card-large">
        <div className="community-post-head">
          <div>
            <span className="section-kicker">{post.post_type === 'article' ? '文章' : '动态'}</span>
            <h1>{post.title || truncateCommunityText(post.summary || post.content, 36)}</h1>
          </div>
          <span>{formatCommunityDateTime(post.created_at)}</span>
        </div>

        <div className="community-meta-row">
          <span>作者：{post.author?.username || '匿名用户'}</span>
          <span>浏览 {post.view_count}</span>
          <span>评论 {post.comment_count}</span>
          <span>点赞 {post.like_count}</span>
          <span>更新于 {formatCommunityDateTime(post.updated_at)}</span>
        </div>

        <div className="community-tag-row">
          {post.tags.map((item) => (
            <span key={`${post.id}-${item}`}>{item}</span>
          ))}
        </div>

        <div className="question-content community-detail-content">{post.content}</div>

        <div className="page-actions" style={{ marginTop: 24 }}>
          <button className="primary-button" type="button" disabled={pending} onClick={() => void handleToggleLike()}>
            {post.is_liked ? '取消点赞' : '点赞支持'}
          </button>
          {post.is_author ? (
            <button className="secondary-button" type="button" disabled={pending} onClick={() => void handleDeletePost()}>
              删除帖子
            </button>
          ) : null}
        </div>
      </article>

      <div className="status-card" style={{ marginTop: 24 }}>{message}</div>

      <article className="section-card section-card-large" style={{ marginTop: 24 }}>
        <div className="section-head">
          <div>
            <span className="section-kicker">评论区</span>
            <h2>共 {post.comment_count} 条评论</h2>
          </div>
        </div>

        <form className="stack-form" onSubmit={handleCommentSubmit}>
          <label className="field">
            <span>发表评论</span>
            <textarea
              value={commentInput}
              onChange={(event) => setCommentInput(event.target.value)}
              maxLength={1000}
              placeholder="写下你的刷题经验、追问建议或补充观点"
            />
          </label>
          <div className="page-actions">
            <button className="secondary-button" type="submit" disabled={pending}>
              {pending ? '提交中...' : '发布评论'}
            </button>
          </div>
        </form>

        {commentsQuery.isLoading ? <div className="status-card" style={{ marginTop: 24 }}>评论加载中...</div> : null}
        {commentsQuery.isError ? (
          <div className="status-card" style={{ marginTop: 24 }}>{extractErrorMessage(commentsQuery.error, '评论加载失败')}</div>
        ) : null}
        {commentsQuery.data?.length ? (
          <div className="community-comment-list">
            {commentsQuery.data.map((comment) => (
              <article className="timeline-item" key={comment.id}>
                <div className="community-post-head">
                  <strong>{comment.author?.username || '匿名用户'}{comment.is_author ? ' · 我' : ''}</strong>
                  <span>{formatCommunityDateTime(comment.created_at)}</span>
                </div>
                <p>{comment.content}</p>
              </article>
            ))}
          </div>
        ) : commentsQuery.data ? (
          <div className="timeline-item" style={{ marginTop: 24 }}>
            <strong>还没有评论</strong>
            <p>这条帖子已经开放评论，你可以先留下第一条交流内容。</p>
          </div>
        ) : null}
      </article>
    </section>
  )
}

/**
 * 提供社区发布页，承接顶部发布按钮和社区入口的发帖动作。
 */
export function CommunityCreatePostPage() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const accessToken = useAuthStore((state) => state.accessToken)
  const [pending, setPending] = useState(false)
  const [draftPayload, setDraftPayload] = useState(() => readCommunityDraft())
  const initialPayload = useMemo(() => {
    if (!draftPayload) {
      return null
    }

    return {
      post_type: draftPayload.postType,
      title: draftPayload.title,
      content: draftPayload.content,
      tags: draftPayload.tags,
    }
  }, [draftPayload])

  useEffect(() => {
    setDraftPayload(readCommunityDraft())
  }, [])

  /**
   * 提交新帖子并在成功后跳转到对应详情页。
   */
  async function handleCreatePost(payload: CommunityPostPayload) {
    if (!accessToken) {
      redirectToCommunityLogin(navigate)
      return
    }

    setPending(true)
    try {
      const post = await createCommunityPost(accessToken, payload)
      clearCommunityDraft()
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['community-posts'] }),
        queryClient.invalidateQueries({ queryKey: ['community-my-posts'] }),
        queryClient.invalidateQueries({ queryKey: ['home-community-posts'] }),
      ])
      navigate({
        to: '/community/$postId',
        params: { postId: String(post.id) },
      })
    } finally {
      setPending(false)
    }
  }

  return (
    <CommunityPostEditor
      title="发布社区帖子"
      subtitle="支持文章和动态两种内容形态，适合沉淀刷题复盘、问题讨论和面经总结。"
      submitLabel="立即发布"
      initialPayload={initialPayload}
      pending={pending}
      onSubmit={handleCreatePost}
    />
  )
}

/**
 * 提供社区编辑页，复用同一套表单修改已有帖子内容。
 */
export function CommunityEditPostPage() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const accessToken = useAuthStore((state) => state.accessToken)
  const params = useParams({ strict: false })
  const postId = String(params.postId || '')
  const [pending, setPending] = useState(false)

  const detailQuery = useQuery({
    queryKey: ['community-post', postId, accessToken],
    queryFn: () => fetchCommunityPostDetail(postId, accessToken),
    enabled: Boolean(postId),
  })

  /**
   * 提交帖子更新并返回详情页。
   */
  async function handleUpdatePost(payload: CommunityPostPayload) {
    if (!accessToken) {
      redirectToCommunityLogin(navigate)
      return
    }

    setPending(true)
    try {
      const post = await updateCommunityPost(accessToken, postId, payload)
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['community-post', postId] }),
        queryClient.invalidateQueries({ queryKey: ['community-posts'] }),
        queryClient.invalidateQueries({ queryKey: ['community-my-posts'] }),
        queryClient.invalidateQueries({ queryKey: ['home-community-posts'] }),
      ])
      navigate({
        to: '/community/$postId',
        params: { postId: String(post.id) },
      })
    } finally {
      setPending(false)
    }
  }

  if (detailQuery.isLoading) {
    return <section className="page-panel community-page-panel"><div className="status-card">帖子内容加载中...</div></section>
  }

  if (detailQuery.isError || !detailQuery.data) {
    return (
      <section className="page-panel community-page-panel">
        <div className="status-card">{extractErrorMessage(detailQuery.error, '帖子内容加载失败')}</div>
      </section>
    )
  }

  if (!detailQuery.data.is_author) {
    return (
      <section className="page-panel community-page-panel">
        <div className="status-card">只有帖子作者可以编辑这条内容。</div>
      </section>
    )
  }

  return (
    <CommunityPostEditor
      title="编辑社区帖子"
      subtitle="修改后会直接覆盖原帖内容，评论和点赞数据会继续保留。"
      submitLabel="保存修改"
      initialData={detailQuery.data}
      pending={pending}
      onSubmit={handleUpdatePost}
    />
  )
}

/**
 * 提供我的帖子页，集中管理当前用户发布过的内容。
 */
export function CommunityMyPostsPage() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const accessToken = useAuthStore((state) => state.accessToken)
  const [page, setPage] = useState(1)
  const [postType, setPostType] = useState('')
  const [keywordInput, setKeywordInput] = useState('')
  const [keyword, setKeyword] = useState('')
  const [tagInput, setTagInput] = useState('')
  const [tag, setTag] = useState('')
  const [message, setMessage] = useState('这里会集中展示你发布过的社区内容')

  const postsQuery = useQuery({
    queryKey: ['community-my-posts', page, postType, keyword, tag, accessToken],
    queryFn: () => fetchMyCommunityPosts(accessToken as string, {
      page,
      pageSize: COMMUNITY_PAGE_SIZE,
      postType,
      keyword,
      tag,
    }),
    enabled: Boolean(accessToken),
  })

  /**
   * 提交我的帖子筛选条件，并从第一页开始重新查询。
   */
  function handleFilterSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setPage(1)
    setKeyword(keywordInput.trim())
    setTag(tagInput.trim())
  }

  /**
   * 删除自己的帖子，并同步刷新社区缓存。
   */
  async function handleDeletePost(postId: number) {
    if (!accessToken) {
      redirectToCommunityLogin(navigate)
      return
    }

    if (typeof window !== 'undefined' && !window.confirm('确认删除这条帖子吗？删除后无法恢复。')) {
      return
    }

    try {
      await deleteCommunityPost(accessToken, String(postId))
      setMessage('帖子已删除')
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['community-my-posts'] }),
        queryClient.invalidateQueries({ queryKey: ['community-posts'] }),
        queryClient.invalidateQueries({ queryKey: ['home-community-posts'] }),
      ])
    } catch (error) {
      if (!useAuthStore.getState().accessToken) {
        redirectToCommunityLogin(navigate)
        return
      }
      setMessage(extractErrorMessage(error, '删除帖子失败'))
    }
  }

  return (
    <section className="page-panel community-page-panel">
      <span className="page-tag">我的帖子</span>
      <h1>管理我发布过的社区内容</h1>
      <p className="page-copy">这里统一承接你的文章、动态、编辑和删除动作，避免回到首页动态流里逐条找帖子。</p>
      <div className="status-card" style={{ marginTop: 24 }}>{message}</div>

      <form className="stack-form" onSubmit={handleFilterSubmit}>
        <div className="community-filter-grid">
          <label className="field">
            <span>内容类型</span>
            <select value={postType} onChange={(event) => { setPostType(event.target.value); setPage(1) }}>
              <option value="">全部</option>
              <option value="article">文章</option>
              <option value="moment">动态</option>
            </select>
          </label>
          <label className="field">
            <span>关键词</span>
            <input value={keywordInput} onChange={(event) => setKeywordInput(event.target.value)} placeholder="搜索我的帖子" />
          </label>
          <label className="field">
            <span>标签</span>
            <input value={tagInput} onChange={(event) => setTagInput(event.target.value)} placeholder="按标签筛选" />
          </label>
        </div>
        <div className="page-actions">
          <button className="secondary-button" type="submit">应用筛选</button>
          <button className="primary-button" type="button" onClick={() => navigate({ to: '/community/create' })}>
            发布新帖子
          </button>
        </div>
      </form>

      {postsQuery.isLoading ? <div className="status-card" style={{ marginTop: 24 }}>我的帖子加载中...</div> : null}
      {postsQuery.isError ? (
        <div className="status-card" style={{ marginTop: 24 }}>{extractErrorMessage(postsQuery.error, '我的帖子加载失败')}</div>
      ) : null}

      {postsQuery.data?.list?.length ? (
        <>
          <div className="community-post-list">
            {postsQuery.data.list.map((post) => (
              <article className="feature-card community-post-card" key={post.id}>
                <div className="community-post-head">
                  <div>
                    <span className="section-kicker">{post.post_type === 'article' ? '文章' : '动态'}</span>
                    <h2>{post.title || truncateCommunityText(post.summary || post.content, 28)}</h2>
                  </div>
                  <span>{formatCommunityDateTime(post.updated_at || post.created_at)}</span>
                </div>
                <p>{truncateCommunityText(post.summary || post.content, 140)}</p>
                <div className="community-meta-row">
                  <span>浏览 {post.view_count}</span>
                  <span>评论 {post.comment_count}</span>
                  <span>点赞 {post.like_count}</span>
                </div>
                <div className="page-actions">
                  <Link className="secondary-link" to="/community/$postId" params={{ postId: String(post.id) }}>
                    查看
                  </Link>
                  <Link className="secondary-link" to="/community/$postId/edit" params={{ postId: String(post.id) }}>
                    编辑
                  </Link>
                  <button className="secondary-button" type="button" onClick={() => void handleDeletePost(post.id)}>
                    删除
                  </button>
                </div>
              </article>
            ))}
          </div>

          <div className="card-inline" style={{ marginTop: 24 }}>
            <span>共 {postsQuery.data.total} 条我的社区内容</span>
            <div className="page-actions">
              <button className="secondary-button" type="button" disabled={page <= 1} onClick={() => setPage((current) => current - 1)}>
                上一页
              </button>
              <span>第 {page} 页</span>
              <button
                className="secondary-button"
                type="button"
                disabled={postsQuery.data.list.length < COMMUNITY_PAGE_SIZE}
                onClick={() => setPage((current) => current + 1)}
              >
                下一页
              </button>
            </div>
          </div>
        </>
      ) : postsQuery.data ? (
        <div className="timeline-item" style={{ marginTop: 24 }}>
          <strong>你还没有发布过帖子</strong>
          <p>现在已经可以直接发文章、发动态、回来看自己的帖子并继续编辑。</p>
        </div>
      ) : null}
    </section>
  )
}
