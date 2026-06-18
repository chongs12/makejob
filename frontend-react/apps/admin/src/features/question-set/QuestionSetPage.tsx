import type { FormEvent } from 'react'
import { useEffect, useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  Button,
  Input,
  Select,
  Tag,
  Modal,
  Drawer,
  Table,
  Space,
  Spin,
  Empty,
  Pagination,
  message,
  Tooltip,
} from 'antd'
import type { ColumnsType } from 'antd/es/table'
import {
  PlusOutlined,
  ReloadOutlined,
  EditOutlined,
  DeleteOutlined,
  OrderedListOutlined,
  SearchOutlined,
  BookOutlined,
  CloseOutlined,
  ArrowRightOutlined,
} from '@ant-design/icons'
import { extractErrorMessage, requestJson } from '@makejob/api-client'
import { isSuccessCode, type ApiEnvelope } from '@makejob/shared-types'
import { useAdminAuthStore } from '../../state/auth'

/* ---------- Types ---------- */

interface QuestionSetItem {
  id: number
  slug: string
  title: string
  description: string
  industry_code: string
  cover_image?: string
  question_count: number
  created_at?: string
  updated_at?: string
}

interface QuestionSetDetail extends QuestionSetItem {
  questions: Array<{
    id: number
    title: string
    type: string
    difficulty: string
    sort_order: number
  }>
}

interface PageResult<T> {
  list: T[]
  total: number
  page: number
  page_size: number
}

interface Industry {
  id: number
  code: string
  name: string
  is_active: boolean
}

interface QuestionListItem {
  id: number
  title: string
  type: string
  difficulty: string
  category_name: string
}

/* ---------- Theme ---------- */

const THEME = {
  bg: '#f4f7fe',
  cardBg: '#ffffff',
  primary: '#4f46e5',
  primaryLight: '#e0e7ff',
  accent: '#f59e0b',
  textMain: '#1e293b',
  textSecondary: '#64748b',
  textMuted: '#94a3b8',
  border: '#e2e8f0',
  success: '#10b981',
  warning: '#f59e0b',
  danger: '#ef4444',
  shadow: '0 8px 32px rgba(31, 38, 135, 0.07)',
  radius: 16,
}

const glassCard = {
  background: 'rgba(255,255,255,0.85)',
  backdropFilter: 'blur(20px) saturate(180%)',
  WebkitBackdropFilter: 'blur(20px) saturate(180%)',
  borderRadius: THEME.radius,
  border: '1px solid rgba(255,255,255,0.6)',
  boxShadow: THEME.shadow,
}

const solidCard = {
  background: THEME.cardBg,
  borderRadius: THEME.radius,
  boxShadow: THEME.shadow,
  border: '1px solid ' + THEME.border,
}

/* ---------- Helpers ---------- */

function formatDateTime(value?: string): string {
  if (!value) return '--'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  })
}

const QUESTION_TYPE_LABEL: Record<string, string> = {
  choice: '单选',
  multi: '多选',
  code: '编程',
  subjective: '主观',
}

const QUESTION_DIFFICULTY_LABEL: Record<string, string> = {
  easy: '简单',
  medium: '中等',
  hard: '困难',
}

/* ---------- API ---------- */

async function fetchIndustries(token: string): Promise<Industry[]> {
  const response = await requestJson<ApiEnvelope<Industry[]>>('/admin/industries', { token })
  if (!isSuccessCode(response.code)) throw new Error(response.message || '获取行业失败')
  return response.data || []
}

async function fetchQuestionSets(params: {
  token: string
  page: number
  pageSize: number
  industryCode?: string
  keyword?: string
}): Promise<PageResult<QuestionSetItem>> {
  const searchParams = new URLSearchParams({
    page: String(params.page),
    page_size: String(params.pageSize),
  })
  if (params.industryCode) searchParams.set('industry_code', params.industryCode)
  if (params.keyword) searchParams.set('keyword', params.keyword)

  const response = await requestJson<ApiEnvelope<PageResult<QuestionSetItem>>>(
    `/admin/question-sets?${searchParams.toString()}`,
    { token: params.token },
  )
  if (!isSuccessCode(response.code) || !response.data) {
    throw new Error(response.message || '获取题单列表失败')
  }
  return response.data
}

async function fetchQuestionSetDetail(token: string, id: number): Promise<QuestionSetDetail> {
  const response = await requestJson<ApiEnvelope<QuestionSetDetail>>(`/admin/question-sets/${id}`, { token })
  if (!isSuccessCode(response.code) || !response.data) {
    throw new Error(response.message || '获取题单详情失败')
  }
  return response.data
}

async function createQuestionSet(token: string, payload: {
  title: string
  description: string
  industry_code: string
  cover_image?: string
}): Promise<QuestionSetItem> {
  const response = await requestJson<ApiEnvelope<QuestionSetItem>>('/admin/question-sets', {
    method: 'POST',
    token,
    body: payload,
  })
  if (!isSuccessCode(response.code) || !response.data) {
    throw new Error(response.message || '创建题单失败')
  }
  return response.data
}

async function updateQuestionSet(token: string, id: number, payload: {
  title: string
  description: string
  industry_code: string
  cover_image?: string
}): Promise<QuestionSetItem> {
  const response = await requestJson<ApiEnvelope<QuestionSetItem>>(`/admin/question-sets/${id}`, {
    method: 'PUT',
    token,
    body: payload,
  })
  if (!isSuccessCode(response.code) || !response.data) {
    throw new Error(response.message || '更新题单失败')
  }
  return response.data
}

async function deleteQuestionSet(token: string, id: number): Promise<void> {
  const response = await requestJson<ApiEnvelope<null>>(`/admin/question-sets/${id}`, {
    method: 'DELETE',
    token,
  })
  if (!isSuccessCode(response.code)) {
    throw new Error(response.message || '删除题单失败')
  }
}

async function addQuestionsToSet(token: string, setId: number, questionIds: number[]): Promise<void> {
  const response = await requestJson<ApiEnvelope<null>>(`/admin/question-sets/${setId}/questions`, {
    method: 'POST',
    token,
    body: { question_ids: questionIds },
  })
  if (!isSuccessCode(response.code)) {
    throw new Error(response.message || '添加题目失败')
  }
}

async function removeQuestionsFromSet(token: string, setId: number, questionIds: number[]): Promise<void> {
  const response = await requestJson<ApiEnvelope<null>>(`/admin/question-sets/${setId}/questions`, {
    method: 'DELETE',
    token,
    body: { question_ids: questionIds },
  })
  if (!isSuccessCode(response.code)) {
    throw new Error(response.message || '移除题目失败')
  }
}

async function fetchQuestionsForPicker(token: string, params: {
  page: number
  pageSize: number
  keyword?: string
  industryCode?: string
}): Promise<PageResult<QuestionListItem>> {
  const searchParams = new URLSearchParams({
    page: String(params.page),
    page_size: String(params.pageSize),
  })
  if (params.keyword) searchParams.set('keyword', params.keyword)
  if (params.industryCode) searchParams.set('industry_code', params.industryCode)

  const response = await requestJson<ApiEnvelope<PageResult<QuestionListItem>>>(
    `/admin/questions?${searchParams.toString()}`,
    { token },
  )
  if (!isSuccessCode(response.code) || !response.data) {
    throw new Error(response.message || '获取题目列表失败')
  }
  return response.data
}

/* ---------- Main Page ---------- */

export function QuestionSetPage() {
  const token = useAdminAuthStore((state) => state.accessToken) as string
  const queryClient = useQueryClient()

  const [page, setPage] = useState(1)
  const [pageSize] = useState(10)
  const [industryCode, setIndustryCode] = useState('')
  const [keyword, setKeyword] = useState('')
  const [searchKeyword, setSearchKeyword] = useState('')

  // Modal state
  const [modalOpen, setModalOpen] = useState(false)
  const [editingSet, setEditingSet] = useState<QuestionSetItem | null>(null)
  const [form, setForm] = useState({
    title: '',
    description: '',
    industry_code: '',
    cover_image: '',
  })
  const [formError, setFormError] = useState('')

  // Drawer state
  const [drawerOpen, setDrawerOpen] = useState(false)
  const [activeSetId, setActiveSetId] = useState<number | null>(null)

  // Question picker state
  const [pickerOpen, setPickerOpen] = useState(false)
  const [pickerPage, setPickerPage] = useState(1)
  const [pickerKeyword, setPickerKeyword] = useState('')
  const [selectedQuestionIds, setSelectedQuestionIds] = useState<number[]>([])

  const industriesQuery = useQuery({
    queryKey: ['admin-industries'],
    queryFn: () => fetchIndustries(token),
    enabled: Boolean(token),
  })

  const setsQuery = useQuery({
    queryKey: ['admin-question-sets', page, pageSize, industryCode, searchKeyword],
    queryFn: () => fetchQuestionSets({ token, page, pageSize, industryCode, keyword: searchKeyword }),
    enabled: Boolean(token),
  })

  const detailQuery = useQuery({
    queryKey: ['admin-question-set-detail', activeSetId],
    queryFn: () => fetchQuestionSetDetail(token, activeSetId as number),
    enabled: Boolean(activeSetId),
  })

  const pickerQuery = useQuery({
    queryKey: ['admin-questions-picker', pickerPage, pickerKeyword, industryCode],
    queryFn: () => fetchQuestionsForPicker(token, {
      page: pickerPage,
      pageSize: 10,
      keyword: pickerKeyword,
      industryCode,
    }),
    enabled: Boolean(token) && pickerOpen,
  })

  const createMutation = useMutation({
    mutationFn: (payload: Parameters<typeof createQuestionSet>[1]) => createQuestionSet(token, payload),
    onSuccess: () => {
      message.success('题单创建成功')
      setModalOpen(false)
      resetForm()
      queryClient.invalidateQueries({ queryKey: ['admin-question-sets'] })
    },
    onError: (error) => setFormError(extractErrorMessage(error, '创建失败')),
  })

  const updateMutation = useMutation({
    mutationFn: ({ id, payload }: { id: number; payload: Parameters<typeof updateQuestionSet>[2] }) =>
      updateQuestionSet(token, id, payload),
    onSuccess: () => {
      message.success('题单更新成功')
      setModalOpen(false)
      setEditingSet(null)
      resetForm()
      queryClient.invalidateQueries({ queryKey: ['admin-question-sets'] })
      if (activeSetId) {
        queryClient.invalidateQueries({ queryKey: ['admin-question-set-detail', activeSetId] })
      }
    },
    onError: (error) => setFormError(extractErrorMessage(error, '更新失败')),
  })

  const deleteMutation = useMutation({
    mutationFn: (id: number) => deleteQuestionSet(token, id),
    onSuccess: () => {
      message.success('题单已删除')
      queryClient.invalidateQueries({ queryKey: ['admin-question-sets'] })
    },
    onError: (error) => message.error(extractErrorMessage(error, '删除失败')),
  })

  const addQuestionsMutation = useMutation({
    mutationFn: ({ setId, questionIds }: { setId: number; questionIds: number[] }) =>
      addQuestionsToSet(token, setId, questionIds),
    onSuccess: () => {
      message.success('题目已添加')
      setPickerOpen(false)
      setSelectedQuestionIds([])
      queryClient.invalidateQueries({ queryKey: ['admin-question-set-detail', activeSetId] })
      queryClient.invalidateQueries({ queryKey: ['admin-question-sets'] })
    },
    onError: (error) => message.error(extractErrorMessage(error, '添加失败')),
  })

  const removeQuestionsMutation = useMutation({
    mutationFn: ({ setId, questionIds }: { setId: number; questionIds: number[] }) =>
      removeQuestionsFromSet(token, setId, questionIds),
    onSuccess: () => {
      message.success('题目已移除')
      queryClient.invalidateQueries({ queryKey: ['admin-question-set-detail', activeSetId] })
      queryClient.invalidateQueries({ queryKey: ['admin-question-sets'] })
    },
    onError: (error) => message.error(extractErrorMessage(error, '移除失败')),
  })

  function resetForm() {
    setForm({ title: '', description: '', industry_code: '', cover_image: '' })
    setFormError('')
  }

  function openCreateModal() {
    setEditingSet(null)
    resetForm()
    setModalOpen(true)
  }

  function openEditModal(item: QuestionSetItem) {
    setEditingSet(item)
    setForm({
      title: item.title,
      description: item.description,
      industry_code: item.industry_code,
      cover_image: item.cover_image || '',
    })
    setFormError('')
    setModalOpen(true)
  }

  function openDetailDrawer(item: QuestionSetItem) {
    setActiveSetId(item.id)
    setDrawerOpen(true)
  }

  function handleSubmit(event: FormEvent) {
    event.preventDefault()
    if (!form.title.trim()) {
      setFormError('请输入题单名称')
      return
    }
    if (!form.industry_code) {
      setFormError('请选择所属行业')
      return
    }

    const payload = {
      title: form.title.trim(),
      description: form.description.trim(),
      industry_code: form.industry_code,
      cover_image: form.cover_image.trim() || undefined,
    }

    if (editingSet) {
      updateMutation.mutate({ id: editingSet.id, payload })
    } else {
      createMutation.mutate(payload)
    }
  }

  function handleDelete(item: QuestionSetItem) {
    Modal.confirm({
      title: '确认删除题单？',
      content: `将删除题单「${item.title}」，其关联的题目关系也会被清除，但题目本身不会被删除。`,
      okText: '删除',
      okType: 'danger',
      cancelText: '取消',
      onOk: () => deleteMutation.mutate(item.id),
    })
  }

  function handleRemoveQuestion(questionId: number) {
    if (!activeSetId) return
    Modal.confirm({
      title: '确认移除题目？',
      content: '该题目将从题单中移除，题目本身不会被删除。',
      okText: '移除',
      okType: 'danger',
      cancelText: '取消',
      onOk: () => removeQuestionsMutation.mutate({ setId: activeSetId, questionIds: [questionId] }),
    })
  }

  function handlePickerSubmit() {
    if (!activeSetId || selectedQuestionIds.length === 0) return
    addQuestionsMutation.mutate({ setId: activeSetId, questionIds: selectedQuestionIds })
  }

  const industryMap = useMemo(() => {
    const map = new Map<string, Industry>()
    for (const item of industriesQuery.data || []) {
      map.set(item.code, item)
    }
    return map
  }, [industriesQuery.data])

  const columns: ColumnsType<QuestionSetItem> = [
    {
      title: 'ID',
      dataIndex: 'id',
      width: 60,
      render: (v: number) => <span style={{ color: THEME.textMuted, fontSize: 13 }}>{v}</span>,
    },
    {
      title: '题单名称',
      dataIndex: 'title',
      render: (v: string, record: QuestionSetItem) => (
        <div>
          <div style={{ fontWeight: 600, color: THEME.textMain, fontSize: 14 }}>{v}</div>
          {record.description ? (
            <Tooltip title={record.description}>
              <div style={{ fontSize: 12, color: THEME.textMuted, maxWidth: 300, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                {record.description}
              </div>
            </Tooltip>
          ) : null}
        </div>
      ),
    },
    {
      title: '行业',
      dataIndex: 'industry_code',
      width: 120,
      render: (v: string) => (
        <Tag style={{ margin: 0, fontSize: 12 }}>{industryMap.get(v)?.name || v}</Tag>
      ),
    },
    {
      title: '题目数',
      dataIndex: 'question_count',
      width: 90,
      render: (v: number) => (
        <Tag color={v > 0 ? 'blue' : 'default'} style={{ margin: 0, fontSize: 12, fontWeight: 600 }}>{v}</Tag>
      ),
    },
    {
      title: '创建时间',
      dataIndex: 'created_at',
      width: 160,
      render: (v: string) => <span style={{ fontSize: 13, color: THEME.textSecondary }}>{formatDateTime(v)}</span>,
    },
    {
      title: '操作',
      key: 'action',
      width: 200,
      render: (_: unknown, record: QuestionSetItem) => (
        <Space size="small">
          <Button size="small" icon={<EditOutlined />} onClick={() => openEditModal(record)}>编辑</Button>
          <Button size="small" icon={<OrderedListOutlined />} onClick={() => openDetailDrawer(record)}>管理题目</Button>
          <Button size="small" danger icon={<DeleteOutlined />} onClick={() => handleDelete(record)}>删除</Button>
        </Space>
      ),
    },
  ]

  const isSubmitting = createMutation.isPending || updateMutation.isPending

  return (
    <div style={{ padding: '28px 32px', background: THEME.bg, minHeight: '100vh' }}>
      {/* Header */}
      <div style={{ ...glassCard, padding: '28px 32px', marginBottom: 24, display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
        <div>
          <div style={{ fontSize: 24, fontWeight: 800, color: THEME.textMain }}>题单管理</div>
          <div style={{ fontSize: 13, color: THEME.textSecondary, marginTop: 4 }}>创建和管理题目集合，为前台用户提供结构化练习路径</div>
        </div>
        <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
          <div style={{ textAlign: 'center', padding: '0 16px' }}>
            <div style={{ fontSize: 28, fontWeight: 800, color: THEME.primary, lineHeight: 1 }}>{setsQuery.data?.total || 0}</div>
            <div style={{ fontSize: 12, color: THEME.textMuted, marginTop: 4 }}>个题单</div>
          </div>
        </div>
      </div>

      {/* Toolbar */}
      <div style={{ ...solidCard, padding: '18px 24px', marginBottom: 24, display: 'flex', alignItems: 'center', gap: 16, flexWrap: 'wrap' }}>
        <div style={{ minWidth: 200, flex: 1 }}>
          <div style={{ marginBottom: 4, fontSize: 12, fontWeight: 600, color: THEME.textMuted }}>关键词</div>
          <Input.Search
            value={keyword}
            onChange={(e) => setKeyword(e.target.value)}
            onSearch={(v) => { setSearchKeyword(v); setPage(1) }}
            placeholder="搜索题单名称"
            allowClear
            style={{ borderRadius: 10 }}
          />
        </div>
        <div style={{ minWidth: 160 }}>
          <div style={{ marginBottom: 4, fontSize: 12, fontWeight: 600, color: THEME.textMuted }}>行业</div>
          <Select
            value={industryCode || undefined}
            onChange={(val) => { setIndustryCode(val || ''); setPage(1) }}
            placeholder="全部行业"
            allowClear
            style={{ width: '100%', borderRadius: 10 }}
            dropdownStyle={{ borderRadius: 10 }}
          >
            {(industriesQuery.data || []).map((item) => (
              <Select.Option key={item.code} value={item.code}>{item.name}</Select.Option>
            ))}
          </Select>
        </div>
        <Button icon={<ReloadOutlined />} onClick={() => setsQuery.refetch()} style={{ marginTop: 20 }}>刷新</Button>
        <Button
          type="primary"
          icon={<PlusOutlined />}
          onClick={openCreateModal}
          style={{ borderRadius: 10, background: THEME.primary, borderColor: THEME.primary, fontWeight: 600, marginTop: 20 }}
        >
          新建题单
        </Button>
      </div>

      {/* Table */}
      <div style={{ ...solidCard, padding: 24 }}>
        {setsQuery.isLoading ? (
          <div style={{ textAlign: 'center', padding: 48 }}><Spin /></div>
        ) : setsQuery.isError ? (
          <div style={{ textAlign: 'center', padding: 48, color: THEME.danger }}>{extractErrorMessage(setsQuery.error, '加载失败')}</div>
        ) : (
          <>
            <Table
              dataSource={setsQuery.data?.list || []}
              columns={columns}
              rowKey="id"
              pagination={false}
              size="middle"
              locale={{ emptyText: <Empty description="暂无题单" /> }}
            />
            {(setsQuery.data?.total || 0) > 0 && (
              <div style={{ display: 'flex', justifyContent: 'flex-end', marginTop: 16 }}>
                <Pagination
                  current={page}
                  pageSize={pageSize}
                  total={setsQuery.data?.total || 0}
                  onChange={setPage}
                  showSizeChanger={false}
                  showTotal={(total) => `共 ${total} 条`}
                />
              </div>
            )}
          </>
        )}
      </div>

      {/* Create/Edit Modal */}
      <Modal
        title={<span style={{ fontSize: 16, fontWeight: 700 }}>{editingSet ? '编辑题单' : '新建题单'}</span>
        }
        open={modalOpen}
        onCancel={() => { setModalOpen(false); resetForm() }}
        footer={null}
        destroyOnClose
        width={520}
      >
        <form onSubmit={handleSubmit} style={{ marginTop: 16 }}>
          <div style={{ marginBottom: 16 }}>
            <div style={{ marginBottom: 6, fontSize: 13, fontWeight: 600, color: THEME.textMain }}>题单名称 *</div>
            <Input
              value={form.title}
              onChange={(e) => setForm((c) => ({ ...c, title: e.target.value }))}
              placeholder="请输入题单名称"
              maxLength={100}
              style={{ borderRadius: 10 }}
            />
          </div>

          <div style={{ marginBottom: 16 }}>
            <div style={{ marginBottom: 6, fontSize: 13, fontWeight: 600, color: THEME.textMain }}>所属行业 *</div>
            <Select
              value={form.industry_code || undefined}
              onChange={(val) => setForm((c) => ({ ...c, industry_code: val }))}
              placeholder="请选择行业"
              style={{ width: '100%', borderRadius: 10 }}
              dropdownStyle={{ borderRadius: 10 }}
            >
              {(industriesQuery.data || []).map((item) => (
                <Select.Option key={item.code} value={item.code}>{item.name}</Select.Option>
              ))}
            </Select>
          </div>

          <div style={{ marginBottom: 16 }}>
            <div style={{ marginBottom: 6, fontSize: 13, fontWeight: 600, color: THEME.textMain }}>描述</div>
            <Input.TextArea
              value={form.description}
              onChange={(e) => setForm((c) => ({ ...c, description: e.target.value }))}
              placeholder="请输入题单描述"
              rows={3}
              maxLength={500}
              style={{ borderRadius: 10, resize: 'none' }}
            />
          </div>

          <div style={{ marginBottom: 20 }}>
            <div style={{ marginBottom: 6, fontSize: 13, fontWeight: 600, color: THEME.textMain }}>
              封面图 URL
              <Tag color="default" style={{ marginLeft: 8, fontSize: 11, fontWeight: 400 }}>预留字段</Tag>
            </div>
            <Input
              value={form.cover_image}
              onChange={(e) => setForm((c) => ({ ...c, cover_image: e.target.value }))}
              placeholder="暂不支持上传，仅预留数据存储"
              disabled
              style={{ borderRadius: 10 }}
            />
            <div style={{ marginTop: 4, fontSize: 12, color: THEME.textMuted }}>
              该字段已存入数据库，但暂无图片上传 API 和存储服务（OSS/MinIO），前台也不会展示封面图。
            </div>
          </div>

          {formError ? (
            <div style={{ marginBottom: 16, padding: '10px 14px', borderRadius: 8, background: '#fef2f2', color: '#991b1b', fontSize: 13 }}>
              {formError}
            </div>
          ) : null}

          <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 10 }}>
            <Button onClick={() => { setModalOpen(false); resetForm() }}>取消</Button>
            <Button type="primary" htmlType="submit" loading={isSubmitting}>{editingSet ? '保存修改' : '创建题单'}</Button>
          </div>
        </form>
      </Modal>

      {/* Detail Drawer */}
      <Drawer
        title={<span style={{ fontSize: 16, fontWeight: 700 }}><OrderedListOutlined style={{ marginRight: 8 }} />题单详情</span>
        }
        placement="right"
        width={640}
        onClose={() => { setDrawerOpen(false); setActiveSetId(null) }}
        open={drawerOpen}
        destroyOnClose
      >
        {detailQuery.isLoading ? (
          <div style={{ textAlign: 'center', padding: 48 }}><Spin /></div>
        ) : detailQuery.isError ? (
          <div style={{ textAlign: 'center', padding: 48, color: THEME.danger }}>{extractErrorMessage(detailQuery.error, '加载失败')}</div>
        ) : detailQuery.data ? (
          <div>
            {/* Info */}
            <div style={{ ...solidCard, padding: 20, marginBottom: 20 }}>
              <div style={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', marginBottom: 12 }}>
                <div>
                  <div style={{ fontSize: 18, fontWeight: 700, color: THEME.textMain, marginBottom: 6 }}>{detailQuery.data.title}</div>
                  <div style={{ display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap' }}>
                    <Tag>{industryMap.get(detailQuery.data.industry_code)?.name || detailQuery.data.industry_code}</Tag>
                    <Tag color="blue">{detailQuery.data.question_count} 题</Tag>
                  </div>
                </div>
                <Button size="small" icon={<EditOutlined />} onClick={() => { setDrawerOpen(false); openEditModal(detailQuery.data as QuestionSetItem) }}>编辑</Button>
              </div>
              {detailQuery.data.description ? (
                <p style={{ margin: 0, fontSize: 13, color: THEME.textSecondary, lineHeight: 1.7 }}>{detailQuery.data.description}</p>
              ) : null}
            </div>

            {/* Questions header */}
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 12 }}>
              <div style={{ fontSize: 15, fontWeight: 700, color: THEME.textMain }}>关联题目 ({detailQuery.data.questions.length})</div>
              <Button type="primary" size="small" icon={<PlusOutlined />} onClick={() => {
                setPickerOpen(true)
                setPickerPage(1)
                setPickerKeyword('')
                setSelectedQuestionIds([])
              }}>添加题目</Button>
            </div>

            {/* Questions list */}
            {detailQuery.data.questions.length > 0 ? (
              <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
                {detailQuery.data.questions.map((q, index) => (
                  <div
                    key={q.id}
                    style={{
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'space-between',
                      padding: '10px 14px',
                      background: THEME.bg,
                      borderRadius: 10,
                    }}
                  >
                    <div style={{ display: 'flex', alignItems: 'center', gap: 10, minWidth: 0 }}>
                      <span style={{ fontSize: 12, color: THEME.textMuted, width: 24 }}>{index + 1}</span>
                      <div style={{ minWidth: 0 }}>
                        <div style={{ fontSize: 13, fontWeight: 600, color: THEME.textMain, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{q.title}</div>
                        <div style={{ display: 'flex', alignItems: 'center', gap: 6, marginTop: 2 }}>
                          <Tag size="small" style={{ margin: 0, fontSize: 11 }}>{QUESTION_TYPE_LABEL[q.type] || q.type}</Tag>
                          <Tag size="small" style={{ margin: 0, fontSize: 11 }}>{QUESTION_DIFFICULTY_LABEL[q.difficulty] || q.difficulty}</Tag>
                        </div>
                      </div>
                    </div>
                    <Button size="small" danger icon={<CloseOutlined />} onClick={() => handleRemoveQuestion(q.id)}>移除</Button>
                  </div>
                ))}
              </div>
            ) : (
              <Empty description="该题单暂无题目" image={Empty.PRESENTED_IMAGE_SIMPLE} />
            )}
          </div>
        ) : null}
      </Drawer>

      {/* Question Picker Modal */}
      <Modal
        title="添加题目到题单"
        open={pickerOpen}
        onCancel={() => { setPickerOpen(false); setSelectedQuestionIds([]) }}
        width={640}
        footer={(
          <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 10 }}>
            <Button onClick={() => { setPickerOpen(false); setSelectedQuestionIds([]) }}>取消</Button>
            <Button
              type="primary"
              disabled={selectedQuestionIds.length === 0}
              loading={addQuestionsMutation.isPending}
              onClick={handlePickerSubmit}
            >
              添加选中题目 ({selectedQuestionIds.length})
            </Button>
          </div>
        )}
        destroyOnClose
      >
        <div style={{ marginBottom: 12 }}>
          <Input.Search
            placeholder="搜索题目"
            allowClear
            onSearch={(v) => { setPickerKeyword(v); setPickerPage(1) }}
            style={{ borderRadius: 10 }}
          />
        </div>

        {pickerQuery.isLoading ? (
          <div style={{ textAlign: 'center', padding: 32 }}><Spin /></div>
        ) : pickerQuery.isError ? (
          <div style={{ textAlign: 'center', padding: 32, color: THEME.danger }}>{extractErrorMessage(pickerQuery.error, '加载失败')}</div>
        ) : (
          <>
            <Table
              rowSelection={{
                type: 'checkbox',
                selectedRowKeys: selectedQuestionIds,
                onChange: (keys) => setSelectedQuestionIds(keys as number[]),
              }}
              dataSource={pickerQuery.data?.list || []}
              rowKey="id"
              pagination={false}
              size="small"
              columns={[
                { title: 'ID', dataIndex: 'id', width: 60 },
                { title: '标题', dataIndex: 'title', ellipsis: true },
                { title: '类型', dataIndex: 'type', width: 80, render: (v: string) => QUESTION_TYPE_LABEL[v] || v },
                { title: '难度', dataIndex: 'difficulty', width: 80, render: (v: string) => QUESTION_DIFFICULTY_LABEL[v] || v },
              ]}
              locale={{ emptyText: <Empty description="没有找到题目" /> }}
              scroll={{ y: 320 }}
            />
            {(pickerQuery.data?.total || 0) > 0 && (
              <div style={{ display: 'flex', justifyContent: 'flex-end', marginTop: 12 }}>
                <Pagination
                  current={pickerPage}
                  pageSize={10}
                  total={pickerQuery.data?.total || 0}
                  onChange={setPickerPage}
                  showSizeChanger={false}
                  size="small"
                />
              </div>
            )}
          </>
        )}
      </Modal>
    </div>
  )
}
