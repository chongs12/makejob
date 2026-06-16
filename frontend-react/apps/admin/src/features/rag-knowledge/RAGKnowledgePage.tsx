import type { FormEvent } from 'react'
import { useEffect, useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  BookOutlined,
  FileTextOutlined,
  MessageOutlined,
  ContainerOutlined,
  SyncOutlined,
  PlusOutlined,
  DeleteOutlined,
  SaveOutlined,
  ImportOutlined,
  SearchOutlined,
  InboxOutlined,
  ThunderboltOutlined,
  ExclamationCircleOutlined,
} from '@ant-design/icons'
import { Badge, Button, Card, Input, Modal, Select, Tag, Tooltip } from 'antd'
import { extractErrorMessage, requestJson } from '@makejob/api-client'
import { isSuccessCode, type ApiEnvelope } from '@makejob/shared-types'
import { useAdminAuthStore } from '../../state/auth'

type DocType = 'tech_doc' | 'interview_exp' | 'job_requirement'
type SyncStatus = 'pending' | 'synced' | 'failed'

interface RAGDocument {
  id: number
  collection: string
  doc_type: DocType
  title: string
  content: string
  metadata: string
  vector_id: string
  sync_status: SyncStatus
  is_active: boolean
  created_at: string
  updated_at: string
}

interface PageResult<T> {
  list: T[]
  total: number
  page: number
  page_size: number
}

interface DocumentForm {
  collection: string
  doc_type: DocType
  title: string
  content: string
  metadata: string
}

interface Filters {
  docType: DocType | ''
  keyword: string
  syncStatus: SyncStatus | ''
}

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

const DOC_TYPE_CONFIG: Record<DocType, { label: string; color: string; icon: React.ReactNode }> = {
  tech_doc: { label: '技术文档', color: '#3b82f6', icon: <FileTextOutlined /> },
  interview_exp: { label: '面经', color: '#8b5cf6', icon: <MessageOutlined /> },
  job_requirement: { label: '岗位要求', color: '#10b981', icon: <ContainerOutlined /> },
}

const SYNC_STATUS_CONFIG: Record<SyncStatus, { label: string; color: string }> = {
  pending: { label: '待同步', color: THEME.warning },
  synced: { label: '已同步', color: THEME.success },
  failed: { label: '同步失败', color: THEME.danger },
}

const INITIAL_FORM: DocumentForm = {
  collection: 'interview_questions',
  doc_type: 'tech_doc',
  title: '',
  content: '',
  metadata: '{}',
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

async function fetchDocuments(
  token: string | null,
  page: number,
  filters: Filters,
): Promise<PageResult<RAGDocument>> {
  const params = new URLSearchParams()
  params.set('page', String(page))
  params.set('page_size', '20')
  if (filters.docType) params.set('doc_type', filters.docType)
  if (filters.keyword) params.set('keyword', filters.keyword)
  if (filters.syncStatus) params.set('sync_status', filters.syncStatus)

  const response = await requestJson<ApiEnvelope<PageResult<RAGDocument>>>(
    `/admin/rag-documents?${params}`,
    { method: 'GET', token },
  )
  if (!isSuccessCode(response.code)) {
    throw new Error(response.message || '获取文档列表失败')
  }
  return response.data
}

async function fetchDocument(token: string | null, id: number): Promise<RAGDocument> {
  const response = await requestJson<ApiEnvelope<RAGDocument>>(`/admin/rag-documents/${id}`, {
    method: 'GET',
    token,
  })
  if (!isSuccessCode(response.code)) {
    throw new Error(response.message || '获取文档详情失败')
  }
  return response.data
}

function buildDocumentPayload(form: DocumentForm): Record<string, unknown> {
  let metadata: Record<string, unknown> = {}
  if (form.metadata && form.metadata.trim() !== '') {
    try {
      metadata = JSON.parse(form.metadata)
    } catch {
      metadata = {}
    }
  }
  return {
    collection: form.collection,
    doc_type: form.doc_type,
    title: form.title,
    content: form.content,
    metadata,
  }
}

async function createDocument(token: string | null, form: DocumentForm): Promise<RAGDocument> {
  const response = await requestJson<ApiEnvelope<RAGDocument>>('/admin/rag-documents', {
    method: 'POST',
    token,
    body: buildDocumentPayload(form),
  })
  if (!isSuccessCode(response.code)) {
    throw new Error(response.message || '创建文档失败')
  }
  return response.data
}

async function updateDocument(token: string | null, id: number, form: DocumentForm): Promise<void> {
  const response = await requestJson<ApiEnvelope<unknown>>(`/admin/rag-documents/${id}`, {
    method: 'PUT',
    token,
    body: buildDocumentPayload(form),
  })
  if (!isSuccessCode(response.code)) {
    throw new Error(response.message || '更新文档失败')
  }
}

async function deleteDocument(token: string | null, id: number): Promise<void> {
  const response = await requestJson<ApiEnvelope<unknown>>(`/admin/rag-documents/${id}`, {
    method: 'DELETE',
    token,
  })
  if (!isSuccessCode(response.code)) {
    throw new Error(response.message || '删除文档失败')
  }
}

async function syncDocuments(token: string | null, ids: number[]): Promise<void> {
  const response = await requestJson<ApiEnvelope<unknown>>('/admin/rag-documents/sync', {
    method: 'POST',
    token,
    body: { ids },
  })
  if (!isSuccessCode(response.code)) {
    throw new Error(response.message || '同步失败')
  }
}

async function syncAllPending(token: string | null): Promise<void> {
  const response = await requestJson<ApiEnvelope<unknown>>('/admin/rag-documents/sync-all', {
    method: 'POST',
    token,
  })
  if (!isSuccessCode(response.code)) {
    throw new Error(response.message || '同步失败')
  }
}

async function batchImport(
  token: string | null,
  collection: string,
  docType: DocType,
  documents: { title: string; content: string; metadata?: Record<string, unknown> }[],
): Promise<{ imported: number; failed: number }> {
  const response = await requestJson<ApiEnvelope<{ imported: number; failed: number }>>(
    '/admin/rag-documents/batch-import',
    {
      method: 'POST',
      token,
      body: { collection, doc_type: docType, documents },
    },
  )
  if (!isSuccessCode(response.code)) {
    throw new Error(response.message || '批量导入失败')
  }
  return response.data
}

function formatDateTime(value?: string): string {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '-'
  return date.toLocaleString('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  })
}

export function RAGKnowledgePage() {
  const accessToken = useAdminAuthStore((state) => state.accessToken)
  const queryClient = useQueryClient()
  const [page, setPage] = useState(1)
  const [filters, setFilters] = useState<Filters>({ docType: '', keyword: '', syncStatus: '' })
  const [selectedId, setSelectedId] = useState<number | null>(null)
  const [form, setForm] = useState<DocumentForm>(INITIAL_FORM)
  const [messageText, setMessageText] = useState('')
  const [batchImportText, setBatchImportText] = useState('')
  const [showBatchImport, setShowBatchImport] = useState(false)

  const docsQuery = useQuery({
    queryKey: ['admin', 'rag-documents', accessToken, page, filters.docType, filters.keyword, filters.syncStatus],
    queryFn: () => fetchDocuments(accessToken, page, filters),
    enabled: Boolean(accessToken),
  })

  const docQuery = useQuery({
    queryKey: ['admin', 'rag-documents', accessToken, selectedId],
    queryFn: () => fetchDocument(accessToken, selectedId!),
    enabled: selectedId !== null,
  })

  useEffect(() => {
    if (docQuery.data) {
      const doc = docQuery.data
      setForm({
        collection: doc.collection,
        doc_type: doc.doc_type,
        title: doc.title,
        content: doc.content,
        metadata: doc.metadata || '{}',
      })
    } else if (selectedId === null) {
      setForm(INITIAL_FORM)
    }
  }, [docQuery.data, selectedId])

  const createMutation = useMutation({
    mutationFn: () => createDocument(accessToken, form),
    onSuccess: async () => {
      setMessageText('创建成功')
      setSelectedId(null)
      setForm(INITIAL_FORM)
      await queryClient.invalidateQueries({ queryKey: ['admin', 'rag-documents'] })
    },
    onError: (error) => setMessageText(extractErrorMessage(error, '创建失败')),
  })

  const updateMutation = useMutation({
    mutationFn: () => updateDocument(accessToken, selectedId!, form),
    onSuccess: async () => {
      setMessageText('更新成功')
      await queryClient.invalidateQueries({ queryKey: ['admin', 'rag-documents'] })
    },
    onError: (error) => setMessageText(extractErrorMessage(error, '更新失败')),
  })

  const deleteMutation = useMutation({
    mutationFn: () => deleteDocument(accessToken, selectedId!),
    onSuccess: async () => {
      setMessageText('删除成功')
      setSelectedId(null)
      setForm(INITIAL_FORM)
      await queryClient.invalidateQueries({ queryKey: ['admin', 'rag-documents'] })
    },
    onError: (error) => setMessageText(extractErrorMessage(error, '删除失败')),
  })

  const syncMutation = useMutation({
    mutationFn: () => syncDocuments(accessToken, [selectedId!]),
    onSuccess: async () => {
      setMessageText('同步成功')
      await queryClient.invalidateQueries({ queryKey: ['admin', 'rag-documents'] })
    },
    onError: (error) => setMessageText(extractErrorMessage(error, '同步失败')),
  })

  const syncAllMutation = useMutation({
    mutationFn: () => syncAllPending(accessToken),
    onSuccess: async () => {
      setMessageText('同步成功')
      await queryClient.invalidateQueries({ queryKey: ['admin', 'rag-documents'] })
    },
    onError: (error) => setMessageText(extractErrorMessage(error, '同步失败')),
  })

  const batchImportMutation = useMutation({
    mutationFn: async () => {
      try {
        const parsed = JSON.parse(batchImportText)
        const docs = Array.isArray(parsed) ? parsed : parsed.documents || []
        return batchImport(accessToken, form.collection, form.doc_type, docs)
      } catch {
        throw new Error('JSON格式错误')
      }
    },
    onSuccess: async (result) => {
      setMessageText(`批量导入完成：成功 ${result.imported ?? 0} 条，失败 ${result.failed ?? 0} 条`)
      setBatchImportText('')
      setShowBatchImport(false)
      await queryClient.invalidateQueries({ queryKey: ['admin', 'rag-documents'] })
    },
    onError: (error) => setMessageText(extractErrorMessage(error, '批量导入失败')),
  })

  const totalPages = useMemo(() => {
    if (!docsQuery.data) return 0
    return Math.ceil(docsQuery.data.total / docsQuery.data.page_size)
  }, [docsQuery.data])

  function handleSubmit(event: FormEvent): void {
    event.preventDefault()
    if (selectedId) {
      updateMutation.mutate()
    } else {
      createMutation.mutate()
    }
  }

  function handleDelete(): void {
    if (!selectedId) return
    Modal.confirm({
      title: '确认删除',
      icon: <ExclamationCircleOutlined />,
      content: '确定删除此文档？删除后不可恢复。',
      okText: '删除',
      okType: 'danger',
      cancelText: '取消',
      onOk: () => deleteMutation.mutate(),
    })
  }

  function handleSync(): void {
    if (!selectedId) return
    syncMutation.mutate()
  }

  function handleSelectDoc(id: number): void {
    setSelectedId(id)
    setMessageText('')
  }

  function handleNewDoc(): void {
    setSelectedId(null)
    setForm(INITIAL_FORM)
    setMessageText('')
  }

  const mutationPending =
    createMutation.isPending ||
    updateMutation.isPending ||
    deleteMutation.isPending ||
    syncMutation.isPending ||
    syncAllMutation.isPending ||
    batchImportMutation.isPending

  const total = docsQuery.data?.total || 0
  const pendingCount = useMemo(() => {
    return docsQuery.data?.list.filter((d) => d.sync_status === 'pending').length || 0
  }, [docsQuery.data])

  return (
    <div style={{ padding: '24px 32px 32px', background: THEME.bg, minHeight: '100vh' }}>
      {/* Header */}
      <div
        style={{
          ...glassCard,
          padding: '24px 28px',
          marginBottom: 20,
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          gap: 16,
          flexWrap: 'wrap',
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', gap: 16 }}>
          <div
            style={{
              width: 48,
              height: 48,
              borderRadius: 14,
              background: 'linear-gradient(135deg, #f59e0b, #d97706)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              boxShadow: '0 4px 14px rgba(245, 158, 11, 0.35)',
              flexShrink: 0,
            }}
          >
            <BookOutlined style={{ fontSize: 22, color: '#fff' }} />
          </div>
          <div>
            <h1
              style={{
                margin: 0,
                fontSize: 22,
                fontWeight: 700,
                color: THEME.textMain,
                lineHeight: 1.3,
              }}
            >
              知识库管理
            </h1>
            <p style={{ margin: '4px 0 0', fontSize: 13, color: THEME.textSecondary }}>
              管理 RAG 知识库文档，支持技术文档、面经、岗位要求等多种类型
            </p>
          </div>
        </div>

        <div style={{ display: 'flex', gap: 12, flexWrap: 'wrap' }}>
          <div
            style={{
              ...solidCard,
              padding: '12px 20px',
              display: 'flex',
              alignItems: 'center',
              gap: 8,
              minWidth: 120,
            }}
          >
            <span style={{ fontSize: 24, fontWeight: 700, color: THEME.primary }}>{total}</span>
            <span style={{ fontSize: 12, color: THEME.textSecondary }}>篇文档</span>
          </div>
          {pendingCount > 0 && (
            <div
              style={{
                ...solidCard,
                padding: '12px 20px',
                display: 'flex',
                alignItems: 'center',
                gap: 8,
                minWidth: 120,
              }}
            >
              <span style={{ fontSize: 24, fontWeight: 700, color: THEME.warning }}>{pendingCount}</span>
              <span style={{ fontSize: 12, color: THEME.textSecondary }}>待同步</span>
            </div>
          )}
        </div>
      </div>

      {/* Toolbar */}
      <div
        style={{
          ...solidCard,
          padding: '14px 20px',
          marginBottom: 20,
          display: 'flex',
          alignItems: 'center',
          gap: 12,
          flexWrap: 'wrap',
        }}
      >
        <Select
          value={filters.docType || undefined}
          placeholder="所有类型"
          allowClear
          style={{ width: 140 }}
          onChange={(v) => {
            setFilters((prev) => ({ ...prev, docType: (v as DocType) || '' }))
            setPage(1)
          }}
          options={[
            { value: 'tech_doc', label: '技术文档' },
            { value: 'interview_exp', label: '面经' },
            { value: 'job_requirement', label: '岗位要求' },
          ]}
        />

        <Select
          value={filters.syncStatus || undefined}
          placeholder="所有状态"
          allowClear
          style={{ width: 140 }}
          onChange={(v) => {
            setFilters((prev) => ({ ...prev, syncStatus: (v as SyncStatus) || '' }))
            setPage(1)
          }}
          options={[
            { value: 'pending', label: '待同步' },
            { value: 'synced', label: '已同步' },
            { value: 'failed', label: '同步失败' },
          ]}
        />

        <Input
          placeholder="搜索标题或内容"
          value={filters.keyword}
          onChange={(e) => {
            setFilters((prev) => ({ ...prev, keyword: e.target.value }))
            setPage(1)
          }}
          prefix={<SearchOutlined style={{ color: THEME.textMuted }} />}
          style={{ width: 220 }}
        />

        <div style={{ flex: 1 }} />

        <Button
          icon={<SyncOutlined spin={syncAllMutation.isPending} />}
          onClick={() => syncAllMutation.mutate()}
          loading={syncAllMutation.isPending}
          disabled={mutationPending}
        >
          同步全部待同步
        </Button>

        <Button
          icon={<ImportOutlined />}
          onClick={() => setShowBatchImport(!showBatchImport)}
        >
          {showBatchImport ? '关闭导入' : '批量导入'}
        </Button>
      </div>

      {/* Batch Import Panel */}
      {showBatchImport && (
        <Card
          title="批量导入"
          style={{ marginBottom: 20, ...solidCard }}
          bodyStyle={{ padding: '16px 20px' }}
          extra={
            <Button
              type="primary"
              icon={<ImportOutlined />}
              loading={batchImportMutation.isPending}
              disabled={mutationPending || !batchImportText.trim()}
              onClick={() => batchImportMutation.mutate()}
              style={{
                borderRadius: 10,
                background: 'linear-gradient(135deg, #3b82f6, #2563eb)',
                border: 'none',
                boxShadow: '0 4px 14px rgba(37, 99, 235, 0.35)',
              }}
            >
              开始导入
            </Button>
          }
        >
          <p style={{ margin: '0 0 12px', fontSize: 13, color: THEME.textSecondary }}>
            {'输入 JSON 格式的文档数组，格式：[{"title": "标题", "content": "内容"}]'}
          </p>
          <Input.TextArea
            value={batchImportText}
            onChange={(e) => setBatchImportText(e.target.value)}
            placeholder='[{"title": "Redis缓存穿透", "content": "缓存穿透是指..."}]'
            rows={6}
            style={{ fontFamily: 'monospace', fontSize: 13 }}
          />
        </Card>
      )}

      {/* Main Content */}
      <div style={{ display: 'flex', gap: 20, flexWrap: 'wrap' }}>
        {/* Left: Document List */}
        <div style={{ flex: '1 1 380px', maxWidth: 480, minWidth: 320 }}>
          <div
            style={{
              ...solidCard,
              overflow: 'hidden',
              display: 'flex',
              flexDirection: 'column',
              height: 'calc(100vh - 260px)',
              minHeight: 500,
            }}
          >
            <div
              style={{
                padding: '16px 20px',
                borderBottom: '1px solid ' + THEME.border,
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'space-between',
              }}
            >
              <span style={{ fontSize: 15, fontWeight: 600, color: THEME.textMain }}>文档列表</span>
              <Button
                type="primary"
                icon={<PlusOutlined />}
                size="small"
                onClick={handleNewDoc}
                style={{
                  borderRadius: 8,
                  background: 'linear-gradient(135deg, #3b82f6, #2563eb)',
                  border: 'none',
                }}
              >
                新建
              </Button>
            </div>

            <div style={{ flex: 1, overflowY: 'auto', padding: '8px' }}>
              {docsQuery.data?.list.length === 0 ? (
                <div
                  style={{
                    height: '100%',
                    display: 'flex',
                    flexDirection: 'column',
                    alignItems: 'center',
                    justifyContent: 'center',
                    color: THEME.textMuted,
                    gap: 12,
                  }}
                >
                  <InboxOutlined style={{ fontSize: 40 }} />
                  <span style={{ fontSize: 14 }}>暂无文档</span>
                </div>
              ) : (
                <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
                  {docsQuery.data?.list.map((doc) => {
                    const cfg = DOC_TYPE_CONFIG[doc.doc_type]
                    const syncCfg = SYNC_STATUS_CONFIG[doc.sync_status]
                    const isActive = selectedId === doc.id
                    return (
                      <div
                        key={doc.id}
                        role="button"
                        tabIndex={0}
                        onClick={() => handleSelectDoc(doc.id)}
                        onKeyDown={(e) => {
                          if (e.key === 'Enter' || e.key === ' ') {
                            handleSelectDoc(doc.id)
                          }
                        }}
                        style={{
                          padding: '14px 16px',
                          borderRadius: 12,
                          cursor: 'pointer',
                          border: isActive
                            ? '1.5px solid ' + THEME.primary
                            : '1.5px solid transparent',
                          background: isActive ? '#f5f3ff' : '#fafafa',
                          transition: 'all 0.2s ease',
                          position: 'relative',
                          overflow: 'hidden',
                        }}
                        onMouseEnter={(e) => {
                          if (!isActive) {
                            e.currentTarget.style.background = '#f1f5f9'
                          }
                        }}
                        onMouseLeave={(e) => {
                          if (!isActive) {
                            e.currentTarget.style.background = '#fafafa'
                          }
                        }}
                      >
                        {isActive && (
                          <div
                            style={{
                              position: 'absolute',
                              left: 0,
                              top: '12px',
                              bottom: '12px',
                              width: 3,
                              borderRadius: '0 3px 3px 0',
                              background: THEME.primary,
                            }}
                          />
                        )}
                        <div
                          style={{
                            display: 'flex',
                            alignItems: 'flex-start',
                            justifyContent: 'space-between',
                            gap: 8,
                            marginBottom: 6,
                          }}
                        >
                          <span
                            style={{
                              fontWeight: 600,
                              fontSize: 14,
                              color: THEME.textMain,
                              lineHeight: 1.4,
                              wordBreak: 'break-all',
                            }}
                          >
                            {doc.title}
                          </span>
                          <Tag
                            color={syncCfg.color}
                            style={{ fontSize: 11, padding: '0 6px', margin: 0, flexShrink: 0 }}
                          >
                            {syncCfg.label}
                          </Tag>
                        </div>
                        <div
                          style={{
                            display: 'flex',
                            alignItems: 'center',
                            gap: 8,
                            fontSize: 12,
                            color: THEME.textSecondary,
                          }}
                        >
                          <span style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
                            <span style={{ color: cfg.color }}>{cfg.icon}</span>
                            {cfg.label}
                          </span>
                          <span style={{ color: THEME.textMuted }}>{formatDateTime(doc.updated_at)}</span>
                        </div>
                      </div>
                    )
                  })}
                </div>
              )}
            </div>

            {/* Pagination */}
            <div
              style={{
                padding: '12px 16px',
                borderTop: '1px solid ' + THEME.border,
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                gap: 12,
              }}
            >
              <Button
                size="small"
                onClick={() => setPage((p) => Math.max(1, p - 1))}
                disabled={page <= 1}
              >
                上一页
              </Button>
              <span style={{ fontSize: 13, color: THEME.textSecondary }}>
                第 {page} / {totalPages || 1} 页
              </span>
              <Button
                size="small"
                onClick={() => setPage((p) => Math.min(totalPages || 1, p + 1))}
                disabled={page >= totalPages}
              >
                下一页
              </Button>
            </div>
          </div>
        </div>

        {/* Right: Document Editor */}
        <div style={{ flex: '2 1 480px', minWidth: 360 }}>
          <div
            style={{
              ...solidCard,
              overflow: 'hidden',
              display: 'flex',
              flexDirection: 'column',
              height: 'calc(100vh - 260px)',
              minHeight: 500,
            }}
          >
            <div
              style={{
                padding: '16px 20px',
                borderBottom: '1px solid ' + THEME.border,
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'space-between',
              }}
            >
              <span style={{ fontSize: 15, fontWeight: 600, color: THEME.textMain }}>
                {selectedId ? '编辑文档' : '新建文档'}
              </span>
              {selectedId && docQuery.data && (
                <Tag
                  color={SYNC_STATUS_CONFIG[docQuery.data.sync_status || 'pending'].color}
                  style={{ fontSize: 12, padding: '2px 10px' }}
                >
                  {SYNC_STATUS_CONFIG[docQuery.data.sync_status || 'pending'].label}
                </Tag>
              )}
            </div>

            <form
              onSubmit={handleSubmit}
              style={{ flex: 1, overflowY: 'auto', padding: '20px', display: 'flex', flexDirection: 'column', gap: 16 }}
            >
              <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 16 }}>
                <div>
                  <label
                    style={{
                      display: 'block',
                      fontSize: 13,
                      fontWeight: 500,
                      color: THEME.textSecondary,
                      marginBottom: 6,
                    }}
                  >
                    Collection
                  </label>
                  <Input
                    value={form.collection}
                    onChange={(e) => setForm((prev) => ({ ...prev, collection: e.target.value }))}
                    required
                    style={{ borderRadius: 10 }}
                  />
                </div>
                <div>
                  <label
                    style={{
                      display: 'block',
                      fontSize: 13,
                      fontWeight: 500,
                      color: THEME.textSecondary,
                      marginBottom: 6,
                    }}
                  >
                    文档类型
                  </label>
                  <Select
                    value={form.doc_type}
                    onChange={(v) => setForm((prev) => ({ ...prev, doc_type: v as DocType }))}
                    style={{ width: '100%' }}
                    options={[
                      { value: 'tech_doc', label: '技术文档' },
                      { value: 'interview_exp', label: '面经' },
                      { value: 'job_requirement', label: '岗位要求' },
                    ]}
                  />
                </div>
              </div>

              <div>
                <label
                  style={{
                    display: 'block',
                    fontSize: 13,
                    fontWeight: 500,
                    color: THEME.textSecondary,
                    marginBottom: 6,
                  }}
                >
                  标题
                </label>
                <Input
                  value={form.title}
                  onChange={(e) => setForm((prev) => ({ ...prev, title: e.target.value }))}
                  required
                  style={{ borderRadius: 10 }}
                />
              </div>

              <div style={{ flex: 1, display: 'flex', flexDirection: 'column', minHeight: 120 }}>
                <label
                  style={{
                    display: 'block',
                    fontSize: 13,
                    fontWeight: 500,
                    color: THEME.textSecondary,
                    marginBottom: 6,
                  }}
                >
                  内容
                </label>
                <Input.TextArea
                  value={form.content}
                  onChange={(e) => setForm((prev) => ({ ...prev, content: e.target.value }))}
                  required
                  style={{ flex: 1, borderRadius: 10, minHeight: 120 }}
                />
              </div>

              <div>
                <label
                  style={{
                    display: 'block',
                    fontSize: 13,
                    fontWeight: 500,
                    color: THEME.textSecondary,
                    marginBottom: 6,
                  }}
                >
                  元数据 (JSON)
                </label>
                <Input.TextArea
                  value={form.metadata}
                  onChange={(e) => setForm((prev) => ({ ...prev, metadata: e.target.value }))}
                  style={{ borderRadius: 10, fontFamily: 'monospace', fontSize: 13 }}
                  rows={4}
                />
              </div>

              <div style={{ display: 'flex', gap: 10, flexWrap: 'wrap', paddingTop: 4 }}>
                <Button
                  type="primary"
                  htmlType="submit"
                  icon={<SaveOutlined />}
                  loading={createMutation.isPending || updateMutation.isPending}
                  disabled={mutationPending}
                  style={{
                    borderRadius: 10,
                    background: 'linear-gradient(135deg, #3b82f6, #2563eb)',
                    border: 'none',
                    boxShadow: '0 4px 14px rgba(37, 99, 235, 0.35)',
                  }}
                >
                  {createMutation.isPending || updateMutation.isPending
                    ? '保存中...'
                    : selectedId
                      ? '更新'
                      : '创建'}
                </Button>

                {selectedId && (
                  <>
                    <Button
                      icon={<ThunderboltOutlined />}
                      onClick={handleSync}
                      loading={syncMutation.isPending}
                      disabled={mutationPending}
                      style={{ borderRadius: 10 }}
                    >
                      同步到向量库
                    </Button>
                    <Button
                      danger
                      icon={<DeleteOutlined />}
                      onClick={handleDelete}
                      loading={deleteMutation.isPending}
                      disabled={mutationPending}
                      style={{ borderRadius: 10 }}
                    >
                      删除
                    </Button>
                  </>
                )}
              </div>
            </form>
          </div>
        </div>
      </div>

      {/* Message Toast */}
      {messageText && (
        <div
          style={{
            position: 'fixed',
            bottom: 24,
            left: '50%',
            transform: 'translateX(-50%)',
            zIndex: 1000,
            padding: '12px 24px',
            borderRadius: 12,
            background: messageText.includes('失败') ? '#fef2f2' : '#f0fdf4',
            border: `1px solid ${messageText.includes('失败') ? '#fecaca' : '#bbf7d0'}`,
            color: messageText.includes('失败') ? '#dc2626' : '#16a34a',
            fontSize: 14,
            fontWeight: 500,
            boxShadow: THEME.shadow,
            animation: 'fadeInUp 0.3s ease',
          }}
        >
          {messageText}
        </div>
      )}

      <style>{`
        @keyframes fadeInUp {
          from { opacity: 0; transform: translateX(-50%) translateY(10px); }
          to { opacity: 1; transform: translateX(-50%) translateY(0); }
        }
      `}</style>
    </div>
  )
}
