import type { FormEvent } from 'react'
import { useEffect, useMemo, useState } from 'react'
import { Link, useNavigate, useParams } from '@tanstack/react-router'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import {
  Button,
  Input,
  Select,
  Tag,
  Avatar,
  Divider,
  Empty,
  Spin,
} from 'antd'
import {
  MessageOutlined,
  LikeOutlined,
  EyeOutlined,
  TagOutlined,
  UserOutlined,
  ClockCircleOutlined,
  EditOutlined,
  DeleteOutlined,
  ArrowLeftOutlined,
  FireOutlined,
  FileTextOutlined,
  PlusOutlined,
  HeartFilled,
  SendOutlined,
  CommentOutlined,
  PushpinFilled,
} from '@ant-design/icons'
import { extractErrorMessage, requestJson } from '@makejob/api-client'
import { isSuccessCode, type ApiEnvelope } from '@makejob/shared-types'
import { requestLoginPrompt } from '../../shared/loginPrompt'
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

const THEME = {
  primary: '#3b82f6',
  primaryLight: '#eff6ff',
  textPrimary: '#1f2937',
  textSecondary: '#6b7280',
  textTertiary: '#9ca3af',
  border: '#e5e7eb',
  borderLight: '#f3f4f6',
  bg: '#f8fafc',
  white: '#ffffff',
  radius: 12,
  radiusSm: 8,
  shadow: '0 1px 3px rgba(0,0,0,0.04), 0 1px 2px rgba(0,0,0,0.02)',
  shadowHover: '0 4px 12px rgba(0,0,0,0.06), 0 2px 4px rgba(0,0,0,0.04)',
  green: '#10b981',
  orange: '#f59e0b',
  red: '#ef4444',
}

function formatCommunityDateTime(value?: string): string {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  const now = new Date()
  const diff = now.getTime() - date.getTime()
  const minutes = Math.floor(diff / 60000)
  const hours = Math.floor(diff / 3600000)
  const days = Math.floor(diff / 86400000)
  if (minutes < 1) return '刚刚'
  if (minutes < 60) return `${minutes}分钟前`
  if (hours < 24) return `${hours}小时前`
  if (days < 7) return `${days}天前`
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}

function truncateCommunityText(value: string, maxLength: number): string {
  const normalized = value.trim()
  if (normalized.length <= maxLength) return normalized
  return `${normalized.slice(0, maxLength)}...`
}

function normalizeCommunityTags(raw: string): string[] {
  return raw
    .split(/[,，]/)
    .map((item) => item.trim())
    .filter(Boolean)
    .filter((item, index, list) => list.findIndex((entry) => entry.toLowerCase() === item.toLowerCase()) === index)
    .slice(0, 5)
}

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
  if (params.postType) searchParams.set('type', params.postType)
  if (params.keyword) searchParams.set('keyword', params.keyword)
  if (params.tag) searchParams.set('tag', params.tag)

  const response = await requestJson<ApiEnvelope<PageResult<CommunityPostItem>>>(`/community/posts?${searchParams.toString()}`, {
    token: params.token || undefined,
  })
  if (!isSuccessCode(response.code) || !response.data) {
    throw new Error(response.message || '获取社区帖子失败')
  }
  return response.data
}

async function fetchCommunityPostDetail(postId: string, token?: string | null): Promise<CommunityPostItem> {
  const response = await requestJson<ApiEnvelope<CommunityPostItem>>(`/community/posts/${postId}`, {
    token: token || undefined,
  })
  if (!isSuccessCode(response.code) || !response.data) {
    throw new Error(response.message || '获取帖子详情失败')
  }
  return response.data
}

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
  if (params.postType) searchParams.set('type', params.postType)
  if (params.keyword) searchParams.set('keyword', params.keyword)
  if (params.tag) searchParams.set('tag', params.tag)

  const response = await requestJson<ApiEnvelope<PageResult<CommunityPostItem>>>(`/community/my/posts?${searchParams.toString()}`, {
    token,
  })
  if (!isSuccessCode(response.code) || !response.data) {
    throw new Error(response.message || '获取我的帖子失败')
  }
  return response.data
}

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

async function deleteCommunityPost(token: string, postId: string): Promise<void> {
  const response = await requestJson<ApiEnvelope<{ id: number }>>(`/community/posts/${postId}`, {
    method: 'DELETE',
    token,
  })
  if (!isSuccessCode(response.code)) {
    throw new Error(response.message || '删除帖子失败')
  }
}

async function fetchCommunityComments(postId: string, token?: string | null): Promise<CommunityCommentItem[]> {
  const response = await requestJson<ApiEnvelope<CommunityCommentItem[]>>(`/community/posts/${postId}/comments`, {
    token: token || undefined,
  })
  if (!isSuccessCode(response.code)) {
    throw new Error(response.message || '获取评论失败')
  }
  return response.data || []
}

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

function promptCommunityLogin(redirectTarget: string, reason: 'missing' | 'expired' = 'missing'): void {
  requestLoginPrompt(redirectTarget, reason)
}

/* ---------- Shared Components ---------- */

function PostTypeTag({ type }: { type: string }) {
  if (type === 'article') {
    return (
      <Tag color="blue" style={{ margin: 0, fontSize: 12, fontWeight: 500 }}>
        <FileTextOutlined style={{ marginRight: 4 }} />
        文章
      </Tag>
    )
  }
  return (
    <Tag color="default" style={{ margin: 0, fontSize: 12, fontWeight: 500 }}>
      <MessageOutlined style={{ marginRight: 4 }} />
      动态
    </Tag>
  )
}

function PostMeta({ post, showAuthor = true }: { post: CommunityPostItem; showAuthor?: boolean }) {
  return (
    <div style={{ display: 'flex', alignItems: 'center', gap: 16, flexWrap: 'wrap', color: THEME.textTertiary, fontSize: 13 }}>
      {showAuthor ? (
        <span style={{ display: 'inline-flex', alignItems: 'center', gap: 6 }}>
          <Avatar size={18} icon={<UserOutlined />} src={post.author?.avatar} style={{ background: THEME.primaryLight, color: THEME.primary }} />
          <span style={{ color: THEME.textSecondary }}>{post.author?.username || '匿名用户'}</span>
        </span>
      ) : null}
      <span style={{ display: 'inline-flex', alignItems: 'center', gap: 4 }}>
        <EyeOutlined />
        {post.view_count}
      </span>
      <span style={{ display: 'inline-flex', alignItems: 'center', gap: 4 }}>
        <CommentOutlined />
        {post.comment_count}
      </span>
      <span style={{ display: 'inline-flex', alignItems: 'center', gap: 4 }}>
        <LikeOutlined />
        {post.like_count}
      </span>
      <span style={{ display: 'inline-flex', alignItems: 'center', gap: 4 }}>
        <ClockCircleOutlined />
        {formatCommunityDateTime(post.created_at)}
      </span>
    </div>
  )
}

/* ---------- Editor ---------- */

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
  const [message, setMessage] = useState('')

  useEffect(() => {
    setPostType(props.initialData?.post_type || props.initialPayload?.post_type || 'article')
    setTitle(props.initialData?.title || props.initialPayload?.title || '')
    setContent(props.initialData?.content || props.initialPayload?.content || '')
    setTagsInput((props.initialData?.tags || props.initialPayload?.tags || []).join(', '))
  }, [props.initialData, props.initialPayload])

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
      setMessage('')
    } catch (error) {
      setMessage(extractErrorMessage(error, '提交失败，请稍后重试'))
    }
  }

  return (
    <div style={{ maxWidth: 800, margin: '0 auto', padding: '24px 0' }}>
      <div style={{ marginBottom: 24 }}>
        <h1 style={{ fontSize: 24, fontWeight: 700, color: THEME.textPrimary, margin: '0 0 8px' }}>{props.title}</h1>
        <p style={{ color: THEME.textSecondary, margin: 0, fontSize: 14 }}>{props.subtitle}</p>
      </div>

      <div style={{ background: THEME.white, borderRadius: THEME.radius, border: `1px solid ${THEME.border}`, padding: 24 }}>
        <form onSubmit={handleSubmit} style={{ display: 'flex', flexDirection: 'column', gap: 20 }}>
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(200px, 1fr))', gap: 16 }}>
            <div>
              <label style={{ display: 'block', fontSize: 13, fontWeight: 600, color: THEME.textSecondary, marginBottom: 6 }}>内容类型</label>
              <Select
                value={postType}
                onChange={(v) => setPostType(v)}
                options={[
                  { value: 'article', label: '文章' },
                  { value: 'moment', label: '动态' },
                ]}
                style={{ width: '100%' }}
              />
            </div>
            <div>
              <label style={{ display: 'block', fontSize: 13, fontWeight: 600, color: THEME.textSecondary, marginBottom: 6 }}>标签</label>
              <Input
                value={tagsInput}
                onChange={(e) => setTagsInput(e.target.value)}
                placeholder="最多 5 个，使用逗号分隔"
              />
            </div>
          </div>

          <div>
            <label style={{ display: 'block', fontSize: 13, fontWeight: 600, color: THEME.textSecondary, marginBottom: 6 }}>标题</label>
            <Input
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              maxLength={120}
              placeholder={postType === 'article' ? '请输入文章标题' : '动态标题可留空'}
            />
          </div>

          <div>
            <label style={{ display: 'block', fontSize: 13, fontWeight: 600, color: THEME.textSecondary, marginBottom: 6 }}>正文内容</label>
            <Input.TextArea
              value={content}
              onChange={(e) => setContent(e.target.value)}
              maxLength={5000}
              rows={10}
              placeholder="请输入帖子正文，支持经验总结、问题讨论和刷题复盘"
              showCount
            />
          </div>

          <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
            <Button type="primary" htmlType="submit" loading={props.pending} icon={<SendOutlined />}>
              {props.submitLabel}
            </Button>
            <span style={{ color: THEME.textTertiary, fontSize: 13 }}>内容上限 5000 字，标签自动去重并限制 5 个</span>
          </div>

          {message ? (
            <div style={{ padding: '10px 14px', borderRadius: THEME.radiusSm, background: message.includes('成功') ? '#f0fdf4' : '#fef2f2', color: message.includes('成功') ? '#166534' : '#991b1b', fontSize: 13 }}>
              {message}
            </div>
          ) : null}
        </form>
      </div>
    </div>
  )
}

/* ---------- Community Page ---------- */

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

  function handleFilterSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setPage(1)
    setKeyword(keywordInput.trim())
    setTag(tagInput.trim())
  }

  const hotTags = useMemo(() => {
    const allTags = postsQuery.data?.list?.flatMap((p) => p.tags) || []
    const counts = new Map<string, number>()
    for (const t of allTags) {
      counts.set(t, (counts.get(t) || 0) + 1)
    }
    return Array.from(counts.entries())
      .sort((a, b) => b[1] - a[1])
      .slice(0, 10)
      .map(([name]) => name)
  }, [postsQuery.data])

  return (
    <div style={{ maxWidth: 1200, margin: '0 auto', padding: '24px 16px', display: 'grid', gridTemplateColumns: '1fr 300px', gap: 24 }}>
      {/* Left: Feed */}
      <div>
        {/* Header */}
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 20 }}>
          <div>
            <h1 style={{ fontSize: 22, fontWeight: 700, color: THEME.textPrimary, margin: '0 0 4px' }}>社区广场</h1>
            <p style={{ color: THEME.textSecondary, margin: 0, fontSize: 13 }}>刷题经验、面经总结和问题讨论都在这里沉淀</p>
          </div>
          <Button
            type="primary"
            icon={<PlusOutlined />}
            onClick={() => {
              if (accessToken) {
                navigate({ to: '/community/create' })
              } else {
                promptCommunityLogin('/community/create', 'missing')
              }
            }}
          >
            发布帖子
          </Button>
        </div>

        {/* Filter bar */}
        <div style={{ background: THEME.white, borderRadius: THEME.radius, border: `1px solid ${THEME.border}`, padding: '12px 16px', marginBottom: 16 }}>
          <form onSubmit={handleFilterSubmit} style={{ display: 'flex', alignItems: 'center', gap: 12, flexWrap: 'wrap' }}>
            <Select
              value={postType || 'all'}
              onChange={(v) => { setPostType(v === 'all' ? '' : v); setPage(1) }}
              options={[
                { value: 'all', label: '全部类型' },
                { value: 'article', label: '文章' },
                { value: 'moment', label: '动态' },
              ]}
              style={{ width: 120 }}
              size="small"
            />
            <Input
              size="small"
              placeholder="搜索标题或正文"
              value={keywordInput}
              onChange={(e) => setKeywordInput(e.target.value)}
              style={{ width: 180 }}
              allowClear
            />
            <Input
              size="small"
              placeholder="按标签筛选"
              value={tagInput}
              onChange={(e) => setTagInput(e.target.value)}
              style={{ width: 140 }}
              allowClear
              prefix={<TagOutlined />}
            />
            <Button size="small" htmlType="submit">筛选</Button>
            {accessToken ? (
              <Button size="small" onClick={() => navigate({ to: '/community/mine' })}>我的帖子</Button>
            ) : null}
          </form>
        </div>

        {/* Loading */}
        {postsQuery.isLoading ? (
          <div style={{ textAlign: 'center', padding: 48 }}>
            <Spin />
          </div>
        ) : null}
        {postsQuery.isError ? (
          <div style={{ padding: 24, textAlign: 'center', color: THEME.red }}>
            {extractErrorMessage(postsQuery.error, '社区帖子加载失败')}
          </div>
        ) : null}

        {/* Post list */}
        {postsQuery.data?.list?.length ? (
          <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
            {postsQuery.data.list.map((post) => (
              <div
                key={post.id}
                style={{
                  background: THEME.white,
                  borderRadius: THEME.radius,
                  border: `1px solid ${THEME.border}`,
                  padding: '16px 20px',
                  transition: 'box-shadow .2s, border-color .2s',
                  cursor: 'pointer',
                }}
                onMouseEnter={(e) => {
                  e.currentTarget.style.boxShadow = THEME.shadowHover
                  e.currentTarget.style.borderColor = '#d1d5db'
                }}
                onMouseLeave={(e) => {
                  e.currentTarget.style.boxShadow = 'none'
                  e.currentTarget.style.borderColor = THEME.border
                }}
                onClick={() => navigate({ to: '/community/$postId', params: { postId: String(post.id) } })}
              >
                <div style={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', gap: 12, marginBottom: 8 }}>
                  <div style={{ display: 'flex', alignItems: 'center', gap: 10, flexWrap: 'wrap' }}>
                    <PostTypeTag type={post.post_type} />
                    {post.is_pinned ? (
                      <Tag color="red" style={{ margin: 0, fontSize: 12 }}><PushpinFilled /> 置顶</Tag>
                    ) : null}
                    <h2
                      style={{
                        fontSize: 16,
                        fontWeight: 600,
                        color: THEME.textPrimary,
                        margin: 0,
                        lineHeight: 1.4,
                      }}
                    >
                      {post.title || truncateCommunityText(post.summary || post.content, 32)}
                    </h2>
                  </div>
                </div>

                <p style={{ color: THEME.textSecondary, fontSize: 14, margin: '0 0 12px', lineHeight: 1.6 }}>
                  {truncateCommunityText(post.summary || post.content, 120)}
                </p>

                {post.tags.length > 0 ? (
                  <div style={{ display: 'flex', gap: 6, flexWrap: 'wrap', marginBottom: 10 }}>
                    {post.tags.map((t) => (
                      <Tag key={`${post.id}-${t}`} style={{ margin: 0, fontSize: 12, cursor: 'pointer' }} onClick={(e) => { e.stopPropagation(); setTagInput(t); setTag(t); setPage(1) }}>
                        {t}
                      </Tag>
                    ))}
                  </div>
                ) : null}

                <PostMeta post={post} />
              </div>
            ))}
          </div>
        ) : postsQuery.data ? (
          <Empty description="当前筛选条件下还没有帖子" image={Empty.PRESENTED_IMAGE_SIMPLE} style={{ marginTop: 48 }} />
        ) : null}

        {/* Pagination */}
        {postsQuery.data?.list?.length ? (
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginTop: 20, padding: '12px 0' }}>
            <span style={{ color: THEME.textTertiary, fontSize: 13 }}>共 {postsQuery.data.total} 条内容</span>
            <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
              <Button size="small" disabled={page <= 1} onClick={() => setPage((c) => c - 1)}>上一页</Button>
              <span style={{ fontSize: 13, color: THEME.textSecondary }}>第 {page} 页</span>
              <Button size="small" disabled={postsQuery.data.list.length < COMMUNITY_PAGE_SIZE} onClick={() => setPage((c) => c + 1)}>下一页</Button>
            </div>
          </div>
        ) : null}
      </div>

      {/* Right sidebar */}
      <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
        <div style={{ background: THEME.white, borderRadius: THEME.radius, border: `1px solid ${THEME.border}`, padding: 16 }}>
          <h3 style={{ fontSize: 14, fontWeight: 700, color: THEME.textPrimary, margin: '0 0 12px', display: 'flex', alignItems: 'center', gap: 6 }}>
            <FireOutlined style={{ color: THEME.orange }} />
            快速入口
          </h3>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
            <Button
              type="primary"
              block
              icon={<PlusOutlined />}
              onClick={() => {
                if (accessToken) navigate({ to: '/community/create' })
                else promptCommunityLogin('/community/create', 'missing')
              }}
            >
              发布新帖
            </Button>
            {accessToken ? (
              <Button block icon={<FileTextOutlined />} onClick={() => navigate({ to: '/community/mine' })}>
                我的帖子
              </Button>
            ) : (
              <Button block icon={<UserOutlined />} onClick={() => promptCommunityLogin('/community', 'missing')}>
                登录 / 注册
              </Button>
            )}
          </div>
        </div>

        {hotTags.length > 0 ? (
          <div style={{ background: THEME.white, borderRadius: THEME.radius, border: `1px solid ${THEME.border}`, padding: 16 }}>
            <h3 style={{ fontSize: 14, fontWeight: 700, color: THEME.textPrimary, margin: '0 0 12px', display: 'flex', alignItems: 'center', gap: 6 }}>
              <TagOutlined style={{ color: THEME.primary }} />
              热门标签
            </h3>
            <div style={{ display: 'flex', flexWrap: 'wrap', gap: 8 }}>
              {hotTags.map((t) => (
                <Tag
                  key={t}
                  style={{ cursor: 'pointer' }}
                  onClick={() => { setTagInput(t); setTag(t); setPage(1) }}
                >
                  {t}
                </Tag>
              ))}
            </div>
          </div>
        ) : null}

        <div style={{ background: THEME.white, borderRadius: THEME.radius, border: `1px solid ${THEME.border}`, padding: 16 }}>
          <h3 style={{ fontSize: 14, fontWeight: 700, color: THEME.textPrimary, margin: '0 0 8px' }}>社区指南</h3>
          <ul style={{ margin: 0, paddingLeft: 18, color: THEME.textSecondary, fontSize: 13, lineHeight: 1.8 }}>
            <li>发布高质量刷题复盘和面经总结</li>
            <li>友善交流，尊重每一位讨论者</li>
            <li>使用标签帮助内容被发现</li>
            <li>文章适合深度内容，动态适合短分享</li>
          </ul>
        </div>
      </div>
    </div>
  )
}

/* ---------- Detail Page ---------- */

export function CommunityPostDetailPage() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const accessToken = useAuthStore((state) => state.accessToken)
  const params = useParams({ strict: false })
  const postId = String(params.postId || '')
  const [commentInput, setCommentInput] = useState('')
  const [message, setMessage] = useState('')
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

  async function refreshCommunityQueries() {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: ['community-post', postId] }),
      queryClient.invalidateQueries({ queryKey: ['community-post-comments', postId] }),
      queryClient.invalidateQueries({ queryKey: ['community-posts'] }),
      queryClient.invalidateQueries({ queryKey: ['community-my-posts'] }),
      queryClient.invalidateQueries({ queryKey: ['home-community-posts'] }),
    ])
  }

  async function handleToggleLike() {
    if (!accessToken) {
      requestLoginPrompt(`/community/${postId}`, 'missing')
      return
    }
    setPending(true)
    try {
      const result = await toggleCommunityLike(accessToken, postId)
      setMessage(result.liked ? '已点赞这条帖子' : '已取消点赞')
      await refreshCommunityQueries()
    } catch (error) {
      if (!useAuthStore.getState().accessToken) {
        requestLoginPrompt(`/community/${postId}`, 'expired')
        return
      }
      setMessage(extractErrorMessage(error, '点赞操作失败'))
    } finally {
      setPending(false)
    }
  }

  async function handleCommentSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!accessToken) {
      requestLoginPrompt(`/community/${postId}`, 'missing')
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
        requestLoginPrompt(`/community/${postId}`, 'expired')
        return
      }
      setMessage(extractErrorMessage(error, '发表评论失败'))
    } finally {
      setPending(false)
    }
  }

  async function handleDeletePost() {
    if (!accessToken) {
      requestLoginPrompt(`/community/${postId}`, 'missing')
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
        requestLoginPrompt(`/community/${postId}`, 'expired')
        return
      }
      setMessage(extractErrorMessage(error, '删除帖子失败'))
    } finally {
      setPending(false)
    }
  }

  if (detailQuery.isLoading) {
    return (
      <div style={{ maxWidth: 900, margin: '0 auto', padding: '48px 16px', textAlign: 'center' }}>
        <Spin />
      </div>
    )
  }
  if (detailQuery.isError || !detailQuery.data) {
    return (
      <div style={{ maxWidth: 900, margin: '0 auto', padding: '48px 16px', textAlign: 'center', color: THEME.red }}>
        {extractErrorMessage(detailQuery.error, '帖子详情加载失败')}
      </div>
    )
  }

  const post = detailQuery.data

  return (
    <div style={{ maxWidth: 900, margin: '0 auto', padding: '24px 16px' }}>
      {/* Back */}
      <div style={{ marginBottom: 16 }}>
        <Button size="small" icon={<ArrowLeftOutlined />} onClick={() => navigate({ to: '/community' })}>
          返回社区
        </Button>
      </div>

      {/* Post card */}
      <div style={{ background: THEME.white, borderRadius: THEME.radius, border: `1px solid ${THEME.border}`, padding: '28px 32px' }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 10, marginBottom: 12, flexWrap: 'wrap' }}>
          <PostTypeTag type={post.post_type} />
          {post.is_pinned ? <Tag color="red" style={{ margin: 0, fontSize: 12 }}><PushpinFilled /> 置顶</Tag> : null}
        </div>

        <h1 style={{ fontSize: 24, fontWeight: 700, color: THEME.textPrimary, margin: '0 0 16px', lineHeight: 1.4 }}>
          {post.title || truncateCommunityText(post.summary || post.content, 40)}
        </h1>

        <div style={{ display: 'flex', alignItems: 'center', gap: 16, marginBottom: 20, flexWrap: 'wrap' }}>
          <span style={{ display: 'inline-flex', alignItems: 'center', gap: 6 }}>
            <Avatar size={24} icon={<UserOutlined />} src={post.author?.avatar} style={{ background: THEME.primaryLight, color: THEME.primary }} />
            <span style={{ color: THEME.textPrimary, fontWeight: 600, fontSize: 14 }}>{post.author?.username || '匿名用户'}</span>
          </span>
          <PostMeta post={post} showAuthor={false} />
        </div>

        {post.tags.length > 0 ? (
          <div style={{ display: 'flex', gap: 6, flexWrap: 'wrap', marginBottom: 20 }}>
            {post.tags.map((t) => (
              <Tag key={`${post.id}-${t}`} style={{ margin: 0 }}>{t}</Tag>
            ))}
          </div>
        ) : null}

        <Divider style={{ margin: '16px 0' }} />

        <div style={{ fontSize: 15, lineHeight: 1.8, color: THEME.textPrimary, whiteSpace: 'pre-wrap' }}>
          {post.content}
        </div>

        <Divider style={{ margin: '24px 0 16px' }} />

        <div style={{ display: 'flex', alignItems: 'center', gap: 12, flexWrap: 'wrap' }}>
          <Button
            type={post.is_liked ? 'primary' : 'default'}
            icon={post.is_liked ? <HeartFilled /> : <LikeOutlined />}
            loading={pending}
            onClick={() => void handleToggleLike()}
          >
            {post.is_liked ? '已点赞' : '点赞'} {post.like_count > 0 ? `(${post.like_count})` : ''}
          </Button>
          {post.is_author ? (
            <>
              <Button icon={<EditOutlined />} onClick={() => navigate({ to: '/community/$postId/edit', params: { postId } })}>
                编辑
              </Button>
              <Button danger icon={<DeleteOutlined />} loading={pending} onClick={() => void handleDeletePost()}>
                删除
              </Button>
            </>
          ) : null}
        </div>

        {message ? (
          <div style={{ marginTop: 12, padding: '8px 12px', borderRadius: THEME.radiusSm, background: message.includes('已') || message.includes('成功') ? '#f0fdf4' : '#fef2f2', color: message.includes('已') || message.includes('成功') ? '#166534' : '#991b1b', fontSize: 13 }}>
            {message}
          </div>
        ) : null}
      </div>

      {/* Comments */}
      <div style={{ background: THEME.white, borderRadius: THEME.radius, border: `1px solid ${THEME.border}`, padding: '24px 32px', marginTop: 16 }}>
        <h2 style={{ fontSize: 18, fontWeight: 700, color: THEME.textPrimary, margin: '0 0 16px', display: 'flex', alignItems: 'center', gap: 8 }}>
          <CommentOutlined />
          评论
          <span style={{ fontSize: 14, color: THEME.textTertiary, fontWeight: 400 }}>({post.comment_count})</span>
        </h2>

        <form onSubmit={handleCommentSubmit} style={{ marginBottom: 24 }}>
          <Input.TextArea
            value={commentInput}
            onChange={(e) => setCommentInput(e.target.value)}
            maxLength={1000}
            rows={3}
            placeholder="写下你的刷题经验、追问建议或补充观点"
            showCount
          />
          <div style={{ display: 'flex', justifyContent: 'flex-end', marginTop: 8 }}>
            <Button type="primary" htmlType="submit" loading={pending} icon={<SendOutlined />}>
              发布评论
            </Button>
          </div>
        </form>

        {commentsQuery.isLoading ? (
          <div style={{ textAlign: 'center', padding: 24 }}><Spin /></div>
        ) : null}
        {commentsQuery.isError ? (
          <div style={{ color: THEME.red, textAlign: 'center', padding: 16 }}>{extractErrorMessage(commentsQuery.error, '评论加载失败')}</div>
        ) : null}

        {commentsQuery.data?.length ? (
          <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
            {commentsQuery.data.map((comment) => (
              <div key={comment.id} style={{ display: 'flex', gap: 12, paddingBottom: 16, borderBottom: `1px solid ${THEME.borderLight}` }}>
                <Avatar size={32} icon={<UserOutlined />} src={comment.author?.avatar} style={{ background: THEME.primaryLight, color: THEME.primary, flexShrink: 0 }} />
                <div style={{ flex: 1, minWidth: 0 }}>
                  <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 4 }}>
                    <span style={{ fontWeight: 600, fontSize: 14, color: THEME.textPrimary }}>{comment.author?.username || '匿名用户'}</span>
                    {comment.is_author ? <Tag color="blue" style={{ margin: 0, fontSize: 11 }}>作者</Tag> : null}
                    <span style={{ color: THEME.textTertiary, fontSize: 12 }}>{formatCommunityDateTime(comment.created_at)}</span>
                  </div>
                  <p style={{ margin: 0, color: THEME.textPrimary, fontSize: 14, lineHeight: 1.7, whiteSpace: 'pre-wrap' }}>{comment.content}</p>
                </div>
              </div>
            ))}
          </div>
        ) : commentsQuery.data ? (
          <Empty description="还没有评论，来抢沙发吧" image={Empty.PRESENTED_IMAGE_SIMPLE} />
        ) : null}
      </div>
    </div>
  )
}

/* ---------- Create Page ---------- */

export function CommunityCreatePostPage() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const accessToken = useAuthStore((state) => state.accessToken)
  const [pending, setPending] = useState(false)
  const [draftPayload, setDraftPayload] = useState(() => readCommunityDraft())
  const initialPayload = useMemo(() => {
    if (!draftPayload) return null
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

  async function handleCreatePost(payload: CommunityPostPayload) {
    if (!accessToken) {
      requestLoginPrompt('/community/create', 'missing')
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
      navigate({ to: '/community/$postId', params: { postId: String(post.id) } })
    } finally {
      setPending(false)
    }
  }

  return (
    <div style={{ maxWidth: 900, margin: '0 auto', padding: '24px 16px' }}>
      <div style={{ marginBottom: 16 }}>
        <Button size="small" icon={<ArrowLeftOutlined />} onClick={() => navigate({ to: '/community' })}>
          返回社区
        </Button>
      </div>
      <CommunityPostEditor
        title="发布社区帖子"
        subtitle="支持文章和动态两种内容形态，适合沉淀刷题复盘、问题讨论和面经总结。"
        submitLabel="立即发布"
        initialPayload={initialPayload}
        pending={pending}
        onSubmit={handleCreatePost}
      />
    </div>
  )
}

/* ---------- Edit Page ---------- */

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

  async function handleUpdatePost(payload: CommunityPostPayload) {
    if (!accessToken) {
      requestLoginPrompt(`/community/${postId}/edit`, 'missing')
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
      navigate({ to: '/community/$postId', params: { postId: String(post.id) } })
    } finally {
      setPending(false)
    }
  }

  if (detailQuery.isLoading) {
    return <div style={{ maxWidth: 900, margin: '0 auto', padding: '48px 16px', textAlign: 'center' }}><Spin /></div>
  }
  if (detailQuery.isError || !detailQuery.data) {
    return <div style={{ maxWidth: 900, margin: '0 auto', padding: '48px 16px', textAlign: 'center', color: THEME.red }}>{extractErrorMessage(detailQuery.error, '帖子内容加载失败')}</div>
  }
  if (!detailQuery.data.is_author) {
    return <div style={{ maxWidth: 900, margin: '0 auto', padding: '48px 16px', textAlign: 'center', color: THEME.red }}>只有帖子作者可以编辑这条内容。</div>
  }

  return (
    <div style={{ maxWidth: 900, margin: '0 auto', padding: '24px 16px' }}>
      <div style={{ marginBottom: 16 }}>
        <Button size="small" icon={<ArrowLeftOutlined />} onClick={() => navigate({ to: '/community' })}>
          返回社区
        </Button>
      </div>
      <CommunityPostEditor
        title="编辑社区帖子"
        subtitle="修改后会直接覆盖原帖内容，评论和点赞数据会继续保留。"
        submitLabel="保存修改"
        initialData={detailQuery.data}
        pending={pending}
        onSubmit={handleUpdatePost}
      />
    </div>
  )
}

/* ---------- My Posts Page ---------- */

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
  const [message, setMessage] = useState('')

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

  function handleFilterSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setPage(1)
    setKeyword(keywordInput.trim())
    setTag(tagInput.trim())
  }

  async function handleDeletePost(postId: number) {
    if (!accessToken) {
      requestLoginPrompt('/community/mine', 'missing')
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
        requestLoginPrompt('/community/mine', 'expired')
        return
      }
      setMessage(extractErrorMessage(error, '删除帖子失败'))
    }
  }

  return (
    <div style={{ maxWidth: 1200, margin: '0 auto', padding: '24px 16px', display: 'grid', gridTemplateColumns: '1fr 300px', gap: 24 }}>
      <div>
        <div style={{ marginBottom: 20 }}>
          <h1 style={{ fontSize: 22, fontWeight: 700, color: THEME.textPrimary, margin: '0 0 4px' }}>我的帖子</h1>
          <p style={{ color: THEME.textSecondary, margin: 0, fontSize: 13 }}>管理你发布过的社区内容</p>
        </div>

        {message ? (
          <div style={{ marginBottom: 16, padding: '10px 14px', borderRadius: THEME.radiusSm, background: message.includes('删除') ? '#fef2f2' : '#f0fdf4', color: message.includes('删除') ? '#991b1b' : '#166534', fontSize: 13 }}>
            {message}
          </div>
        ) : null}

        <div style={{ background: THEME.white, borderRadius: THEME.radius, border: `1px solid ${THEME.border}`, padding: '12px 16px', marginBottom: 16 }}>
          <form onSubmit={handleFilterSubmit} style={{ display: 'flex', alignItems: 'center', gap: 12, flexWrap: 'wrap' }}>
            <Select
              value={postType || 'all'}
              onChange={(v) => { setPostType(v === 'all' ? '' : v); setPage(1) }}
              options={[
                { value: 'all', label: '全部类型' },
                { value: 'article', label: '文章' },
                { value: 'moment', label: '动态' },
              ]}
              style={{ width: 120 }}
              size="small"
            />
            <Input
              size="small"
              placeholder="搜索我的帖子"
              value={keywordInput}
              onChange={(e) => setKeywordInput(e.target.value)}
              style={{ width: 180 }}
              allowClear
            />
            <Input
              size="small"
              placeholder="按标签筛选"
              value={tagInput}
              onChange={(e) => setTagInput(e.target.value)}
              style={{ width: 140 }}
              allowClear
              prefix={<TagOutlined />}
            />
            <Button size="small" htmlType="submit">筛选</Button>
            <Button size="small" type="primary" icon={<PlusOutlined />} onClick={() => navigate({ to: '/community/create' })}>发布</Button>
          </form>
        </div>

        {postsQuery.isLoading ? (
          <div style={{ textAlign: 'center', padding: 48 }}><Spin /></div>
        ) : null}
        {postsQuery.isError ? (
          <div style={{ padding: 24, textAlign: 'center', color: THEME.red }}>{extractErrorMessage(postsQuery.error, '我的帖子加载失败')}</div>
        ) : null}

        {postsQuery.data?.list?.length ? (
          <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
            {postsQuery.data.list.map((post) => (
              <div
                key={post.id}
                style={{
                  background: THEME.white,
                  borderRadius: THEME.radius,
                  border: `1px solid ${THEME.border}`,
                  padding: '16px 20px',
                  transition: 'box-shadow .2s',
                }}
                onMouseEnter={(e) => { e.currentTarget.style.boxShadow = THEME.shadowHover }}
                onMouseLeave={(e) => { e.currentTarget.style.boxShadow = 'none' }}
              >
                <div style={{ display: 'flex', alignItems: 'center', gap: 10, marginBottom: 8, flexWrap: 'wrap' }}>
                  <PostTypeTag type={post.post_type} />
                  <h3 style={{ fontSize: 15, fontWeight: 600, color: THEME.textPrimary, margin: 0, lineHeight: 1.4, flex: 1, minWidth: 0 }}>
                    {post.title || truncateCommunityText(post.summary || post.content, 32)}
                  </h3>
                </div>
                <p style={{ color: THEME.textSecondary, fontSize: 13, margin: '0 0 10px', lineHeight: 1.6 }}>
                  {truncateCommunityText(post.summary || post.content, 100)}
                </p>
                <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', flexWrap: 'wrap', gap: 8 }}>
                  <PostMeta post={post} />
                  <div style={{ display: 'flex', gap: 8 }}>
                    <Button size="small" icon={<EyeOutlined />} onClick={() => navigate({ to: '/community/$postId', params: { postId: String(post.id) } })}>查看</Button>
                    <Button size="small" icon={<EditOutlined />} onClick={() => navigate({ to: '/community/$postId/edit', params: { postId: String(post.id) } })}>编辑</Button>
                    <Button size="small" danger icon={<DeleteOutlined />} onClick={() => void handleDeletePost(post.id)}>删除</Button>
                  </div>
                </div>
              </div>
            ))}
          </div>
        ) : postsQuery.data ? (
          <Empty description="你还没有发布过帖子" image={Empty.PRESENTED_IMAGE_SIMPLE} style={{ marginTop: 48 }} />
        ) : null}

        {postsQuery.data?.list?.length ? (
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginTop: 20, padding: '12px 0' }}>
            <span style={{ color: THEME.textTertiary, fontSize: 13 }}>共 {postsQuery.data.total} 条内容</span>
            <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
              <Button size="small" disabled={page <= 1} onClick={() => setPage((c) => c - 1)}>上一页</Button>
              <span style={{ fontSize: 13, color: THEME.textSecondary }}>第 {page} 页</span>
              <Button size="small" disabled={postsQuery.data.list.length < COMMUNITY_PAGE_SIZE} onClick={() => setPage((c) => c + 1)}>下一页</Button>
            </div>
          </div>
        ) : null}
      </div>

      {/* Right sidebar */}
      <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
        <div style={{ background: THEME.white, borderRadius: THEME.radius, border: `1px solid ${THEME.border}`, padding: 16 }}>
          <h3 style={{ fontSize: 14, fontWeight: 700, color: THEME.textPrimary, margin: '0 0 12px' }}>快捷操作</h3>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
            <Button type="primary" block icon={<PlusOutlined />} onClick={() => navigate({ to: '/community/create' })}>
              发布新帖
            </Button>
            <Button block icon={<ArrowLeftOutlined />} onClick={() => navigate({ to: '/community' })}>
              返回社区
            </Button>
          </div>
        </div>

        <div style={{ background: THEME.white, borderRadius: THEME.radius, border: `1px solid ${THEME.border}`, padding: 16 }}>
          <h3 style={{ fontSize: 14, fontWeight: 700, color: THEME.textPrimary, margin: '0 0 8px' }}>小贴士</h3>
          <ul style={{ margin: 0, paddingLeft: 18, color: THEME.textSecondary, fontSize: 13, lineHeight: 1.8 }}>
            <li>定期回顾和更新旧内容</li>
            <li>好的标题能带来更多阅读</li>
            <li>善用标签增加曝光</li>
          </ul>
        </div>
      </div>
    </div>
  )
}
