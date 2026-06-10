import type { FormEvent } from 'react'
import { useEffect, useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
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

const DOC_TYPE_LABELS: Record<DocType, string> = {
  tech_doc: '技术文档',
  interview_exp: '面经',
  job_requirement: '岗位要求',
}

const SYNC_STATUS_LABELS: Record<SyncStatus, string> = {
  pending: '待同步',
  synced: '已同步',
  failed: '同步失败',
}

const SYNC_STATUS_CLASS: Record<SyncStatus, string> = {
  pending: 'is-warning',
  synced: 'is-valid',
  failed: 'is-error',
}

const INITIAL_FORM: DocumentForm = {
  collection: 'interview_questions',
  doc_type: 'tech_doc',
  title: '',
  content: '',
  metadata: '{}',
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
  const [message, setMessage] = useState('')
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
      setMessage('创建成功')
      setSelectedId(null)
      setForm(INITIAL_FORM)
      await queryClient.invalidateQueries({ queryKey: ['admin', 'rag-documents'] })
    },
    onError: (error) => setMessage(extractErrorMessage(error, '创建失败')),
  })

  const updateMutation = useMutation({
    mutationFn: () => updateDocument(accessToken, selectedId!, form),
    onSuccess: async () => {
      setMessage('更新成功')
      await queryClient.invalidateQueries({ queryKey: ['admin', 'rag-documents'] })
    },
    onError: (error) => setMessage(extractErrorMessage(error, '更新失败')),
  })

  const deleteMutation = useMutation({
    mutationFn: () => deleteDocument(accessToken, selectedId!),
    onSuccess: async () => {
      setMessage('删除成功')
      setSelectedId(null)
      setForm(INITIAL_FORM)
      await queryClient.invalidateQueries({ queryKey: ['admin', 'rag-documents'] })
    },
    onError: (error) => setMessage(extractErrorMessage(error, '删除失败')),
  })

  const syncMutation = useMutation({
    mutationFn: () => syncDocuments(accessToken, [selectedId!]),
    onSuccess: async () => {
      setMessage('同步成功')
      await queryClient.invalidateQueries({ queryKey: ['admin', 'rag-documents'] })
    },
    onError: (error) => setMessage(extractErrorMessage(error, '同步失败')),
  })

  const syncAllMutation = useMutation({
    mutationFn: () => syncAllPending(accessToken),
    onSuccess: async () => {
      setMessage('同步成功')
      await queryClient.invalidateQueries({ queryKey: ['admin', 'rag-documents'] })
    },
    onError: (error) => setMessage(extractErrorMessage(error, '同步失败')),
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
      setMessage(`批量导入完成：成功 ${result.imported ?? 0} 条，失败 ${result.failed ?? 0} 条`)
      setBatchImportText('')
      setShowBatchImport(false)
      await queryClient.invalidateQueries({ queryKey: ['admin', 'rag-documents'] })
    },
    onError: (error) => setMessage(extractErrorMessage(error, '批量导入失败')),
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
    if (!window.confirm('确定删除此文档？删除后不可恢复。')) return
    deleteMutation.mutate()
  }

  function handleSync(): void {
    if (!selectedId) return
    syncMutation.mutate()
  }

  function handleSelectDoc(id: number): void {
    setSelectedId(id)
    setMessage('')
  }

  function handleNewDoc(): void {
    setSelectedId(null)
    setForm(INITIAL_FORM)
    setMessage('')
  }

  const mutationPending =
    createMutation.isPending ||
    updateMutation.isPending ||
    deleteMutation.isPending ||
    syncMutation.isPending ||
    syncAllMutation.isPending ||
    batchImportMutation.isPending

  return (
    <section className="admin-panel admin-rag-knowledge">
      <div className="admin-rag-knowledge__hero">
        <div>
          <span className="admin-tag">知识库</span>
          <h2>知识库管理</h2>
          <p className="admin-copy">
            管理RAG知识库文档，支持技术文档、面经、岗位要求等多种类型。添加文档后需要同步到向量库才能被检索使用。
          </p>
        </div>
        <div className="admin-rag-knowledge__status">
          <strong>{docsQuery.data?.total || 0}</strong>
          <span>篇文档</span>
        </div>
      </div>

      {/* 工具栏 */}
      <div className="admin-rag-knowledge__toolbar">
        <select
          value={filters.docType}
          onChange={(e) => {
            setFilters((prev) => ({ ...prev, docType: e.target.value as DocType | '' }))
            setPage(1)
          }}
        >
          <option value="">所有类型</option>
          <option value="tech_doc">技术文档</option>
          <option value="interview_exp">面经</option>
          <option value="job_requirement">岗位要求</option>
        </select>

        <select
          value={filters.syncStatus}
          onChange={(e) => {
            setFilters((prev) => ({ ...prev, syncStatus: e.target.value as SyncStatus | '' }))
            setPage(1)
          }}
        >
          <option value="">所有状态</option>
          <option value="pending">待同步</option>
          <option value="synced">已同步</option>
          <option value="failed">同步失败</option>
        </select>

        <input
          type="text"
          value={filters.keyword}
          onChange={(e) => {
            setFilters((prev) => ({ ...prev, keyword: e.target.value }))
            setPage(1)
          }}
          placeholder="搜索标题或内容"
        />

        <button
          className="admin-link"
          type="button"
          onClick={() => syncAllMutation.mutate()}
          disabled={mutationPending}
        >
          {syncAllMutation.isPending ? '同步中...' : '同步全部待同步'}
        </button>

        <button
          className="admin-link"
          type="button"
          onClick={() => setShowBatchImport(!showBatchImport)}
        >
          {showBatchImport ? '关闭导入' : '批量导入'}
        </button>
      </div>

      {/* 批量导入面板 */}
      {showBatchImport && (
        <div className="admin-rag-knowledge__batch-import">
          <h3>批量导入</h3>
          <p>输入JSON格式的文档数组，格式：[{"{"}"title": "标题", "content": "内容"{"}"}]</p>
          <textarea
            value={batchImportText}
            onChange={(e) => setBatchImportText(e.target.value)}
            placeholder='[{"title": "Redis缓存穿透", "content": "缓存穿透是指..."}]'
          />
          <button
            className="admin-link"
            type="button"
            onClick={() => batchImportMutation.mutate()}
            disabled={mutationPending || !batchImportText.trim()}
          >
            {batchImportMutation.isPending ? '导入中...' : '开始导入'}
          </button>
        </div>
      )}

      {/* 主体内容 */}
      <div className="admin-rag-knowledge__layout">
        {/* 左侧：文档列表 */}
        <div className="admin-rag-knowledge__list">
          <div className="admin-rag-knowledge__list-head">
            <span>文档列表</span>
            <button className="admin-link" type="button" onClick={handleNewDoc}>
              新建
            </button>
          </div>

          <div className="admin-rag-knowledge__list-body">
            {docsQuery.data?.list.length === 0 ? (
              <div className="admin-rag-knowledge__empty">
                <p>暂无文档</p>
              </div>
            ) : (
              docsQuery.data?.list.map((doc) => (
                <button
                  key={doc.id}
                  type="button"
                  className={`admin-rag-knowledge__card ${selectedId === doc.id ? 'admin-rag-knowledge__card--active' : ''}`}
                  onClick={() => handleSelectDoc(doc.id)}
                >
                  <div className="admin-rag-knowledge__card-head">
                    <strong>{doc.title}</strong>
                    <span className={SYNC_STATUS_CLASS[doc.sync_status]}>
                      {SYNC_STATUS_LABELS[doc.sync_status]}
                    </span>
                  </div>
                  <div className="admin-rag-knowledge__card-meta">
                    <span>{DOC_TYPE_LABELS[doc.doc_type]}</span>
                    <span>{formatDateTime(doc.updated_at)}</span>
                  </div>
                </button>
              ))
            )}
          </div>

          {/* 分页 */}
          <div className="admin-rag-knowledge__pagination">
            <button
              type="button"
              onClick={() => setPage((p) => Math.max(1, p - 1))}
              disabled={page <= 1}
            >
              上一页
            </button>
            <span>第 {page} / {totalPages} 页</span>
            <button
              type="button"
              onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
              disabled={page >= totalPages}
            >
              下一页
            </button>
          </div>
        </div>

        {/* 右侧：文档编辑器 */}
        <div className="admin-rag-knowledge__editor">
          <div className="admin-rag-knowledge__editor-head">
            <h3>{selectedId ? '编辑文档' : '新建文档'}</h3>
            {selectedId && docQuery.data && (
              <span className={SYNC_STATUS_CLASS[docQuery.data.sync_status || 'pending']}>
                {SYNC_STATUS_LABELS[docQuery.data.sync_status || 'pending']}
              </span>
            )}
          </div>

          <form className="admin-rag-knowledge__form" onSubmit={handleSubmit}>
            <div className="admin-rag-knowledge__form-grid">
              <label className="admin-ai-field">
                <span className="admin-ai-field__label">Collection</span>
                <input
                  type="text"
                  value={form.collection}
                  onChange={(e) => setForm((prev) => ({ ...prev, collection: e.target.value }))}
                  required
                />
              </label>

              <label className="admin-ai-field">
                <span className="admin-ai-field__label">文档类型</span>
                <select
                  value={form.doc_type}
                  onChange={(e) => setForm((prev) => ({ ...prev, doc_type: e.target.value as DocType }))}
                >
                  <option value="tech_doc">技术文档</option>
                  <option value="interview_exp">面经</option>
                  <option value="job_requirement">岗位要求</option>
                </select>
              </label>
            </div>

            <label className="admin-ai-field">
              <span className="admin-ai-field__label">标题</span>
              <input
                type="text"
                value={form.title}
                onChange={(e) => setForm((prev) => ({ ...prev, title: e.target.value }))}
                required
              />
            </label>

            <label className="admin-ai-field">
              <span className="admin-ai-field__label">内容</span>
              <textarea
                value={form.content}
                onChange={(e) => setForm((prev) => ({ ...prev, content: e.target.value }))}
                required
              />
            </label>

            <label className="admin-ai-field">
              <span className="admin-ai-field__label">元数据 (JSON)</span>
              <textarea
                value={form.metadata}
                onChange={(e) => setForm((prev) => ({ ...prev, metadata: e.target.value }))}
              />
            </label>

            <div className="admin-rag-knowledge__editor-actions">
              <button
                className="admin-link"
                type="submit"
                disabled={mutationPending}
              >
                {createMutation.isPending || updateMutation.isPending ? '保存中...' : selectedId ? '更新' : '创建'}
              </button>

              {selectedId && (
                <>
                  <button
                    className="admin-link"
                    type="button"
                    onClick={handleSync}
                    disabled={mutationPending}
                  >
                    {syncMutation.isPending ? '同步中...' : '同步到向量库'}
                  </button>
                  <button
                    className="admin-link"
                    type="button"
                    onClick={handleDelete}
                    disabled={mutationPending}
                  >
                    删除
                  </button>
                </>
              )}
            </div>
          </form>
        </div>
      </div>

      {/* 状态消息 */}
      <div className="admin-rag-knowledge__message">
        <p>{message}</p>
      </div>
    </section>
  )
}
