import type { FormEvent } from 'react'
import { useEffect, useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  Button,
  Input,
  InputNumber,
  Select,
  Switch,
  Tag,
  Modal,
  message,
  Row,
  Col,
  Space,
  Spin,
  Empty,
  Divider,
} from 'antd'
import {
  PlusOutlined,
  DeleteOutlined,
  SaveOutlined,
  AppstoreOutlined,
  TagOutlined,
  SortAscendingOutlined,
  InfoCircleOutlined,
} from '@ant-design/icons'
import { extractErrorMessage, requestJson } from '@makejob/api-client'
import { isSuccessCode, type ApiEnvelope } from '@makejob/shared-types'
import { useAdminAuthStore } from '../../state/auth'

interface Industry {
  id: number
  code: string
  name: string
  description: string
  icon: string
  is_active: boolean
  sort_order: number
}

interface Category {
  id: number
  industry_id: number
  name: string
  parent_id?: number | null
  sort_order: number
  icon: string
  description: string
}

interface IndustryFormState {
  code: string
  name: string
  description: string
  icon: string
  sortOrder: string
  isActive: boolean
}

interface CategoryFormState {
  industryId: string
  name: string
  parentId: string
  sortOrder: string
  icon: string
  description: string
}

interface CategoryOption {
  id: number
  label: string
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

async function fetchIndustries(token: string | null): Promise<Industry[]> {
  const response = await requestJson<ApiEnvelope<Industry[]>>('/admin/industries', {
    method: 'GET',
    token,
  })

  if (!isSuccessCode(response.code)) {
    throw new Error(response.message || '获取行业列表失败')
  }

  const data = response.data
  if (Array.isArray(data)) {
    return data
  }
  if (data && typeof data === 'object') {
    const firstArray = Object.values(data).find(Array.isArray)
    if (firstArray) {
      return firstArray as Industry[]
    }
  }
  return []
}

async function fetchCategories(token: string | null): Promise<Category[]> {
  const response = await requestJson<ApiEnvelope<Category[]>>('/admin/categories', {
    method: 'GET',
    token,
  })

  if (!isSuccessCode(response.code)) {
    throw new Error(response.message || '获取分类列表失败')
  }

  const data = response.data
  if (Array.isArray(data)) {
    return data
  }
  if (data && typeof data === 'object') {
    const firstArray = Object.values(data).find(Array.isArray)
    if (firstArray) {
      return firstArray as Category[]
    }
  }
  return []
}

async function createIndustry(token: string | null, payload: Record<string, unknown>): Promise<Industry> {
  const response = await requestJson<ApiEnvelope<Industry>>('/admin/industries', {
    method: 'POST',
    token,
    body: payload,
  })

  if (!isSuccessCode(response.code)) {
    throw new Error(response.message || '创建行业失败')
  }

  return response.data
}

async function updateIndustry(token: string | null, id: number, payload: Record<string, unknown>): Promise<void> {
  const response = await requestJson<ApiEnvelope<null>>(`/admin/industries/${id}`, {
    method: 'PUT',
    token,
    body: payload,
  })

  if (!isSuccessCode(response.code)) {
    throw new Error(response.message || '更新行业失败')
  }
}

async function createCategory(token: string | null, payload: Record<string, unknown>): Promise<Category> {
  const response = await requestJson<ApiEnvelope<Category>>('/admin/categories', {
    method: 'POST',
    token,
    body: payload,
  })

  if (!isSuccessCode(response.code)) {
    throw new Error(response.message || '创建分类失败')
  }

  return response.data
}

async function updateCategory(token: string | null, id: number, payload: Record<string, unknown>): Promise<void> {
  const response = await requestJson<ApiEnvelope<null>>(`/admin/categories/${id}`, {
    method: 'PUT',
    token,
    body: payload,
  })

  if (!isSuccessCode(response.code)) {
    throw new Error(response.message || '更新分类失败')
  }
}

async function deleteCategory(token: string | null, id: number): Promise<void> {
  const response = await requestJson<ApiEnvelope<null>>(`/admin/categories/${id}`, {
    method: 'DELETE',
    token,
  })

  if (!isSuccessCode(response.code)) {
    throw new Error(response.message || '删除分类失败')
  }
}

function buildInitialIndustryForm(): IndustryFormState {
  return {
    code: '',
    name: '',
    description: '',
    icon: '',
    sortOrder: '0',
    isActive: true,
  }
}

function buildIndustryForm(industry?: Industry | null): IndustryFormState {
  if (!industry) {
    return buildInitialIndustryForm()
  }

  return {
    code: industry.code || '',
    name: industry.name || '',
    description: industry.description || '',
    icon: industry.icon || '',
    sortOrder: String(industry.sort_order || 0),
    isActive: industry.is_active ?? true,
  }
}

function buildInitialCategoryForm(industryId = ''): CategoryFormState {
  return {
    industryId,
    name: '',
    parentId: '0',
    sortOrder: '0',
    icon: '',
    description: '',
  }
}

function buildCategoryForm(category?: Category | null): CategoryFormState {
  if (!category) {
    return buildInitialCategoryForm()
  }

  return {
    industryId: String(category.industry_id || 0),
    name: category.name || '',
    parentId: String(category.parent_id || 0),
    sortOrder: String(category.sort_order || 0),
    icon: category.icon || '',
    description: category.description || '',
  }
}

function buildIndustryPayload(form: IndustryFormState): Record<string, unknown> {
  return {
    code: form.code.trim(),
    name: form.name.trim(),
    description: form.description.trim(),
    icon: form.icon.trim(),
    sort_order: Number(form.sortOrder) || 0,
    is_active: form.isActive,
  }
}

function buildCategoryPayload(form: CategoryFormState): Record<string, unknown> {
  return {
    industry_id: Number(form.industryId),
    name: form.name.trim(),
    parent_id: Number(form.parentId) || 0,
    sort_order: Number(form.sortOrder) || 0,
    icon: form.icon.trim(),
    description: form.description.trim(),
  }
}

function buildCategoryOptions(categories: Category[], industryId: string, currentCategoryId: number | null): CategoryOption[] {
  const targetIndustryId = Number(industryId)
  if (!targetIndustryId) {
    return []
  }

  const filtered = categories
    .filter((category) => category.industry_id === targetIndustryId && category.id !== currentCategoryId)
    .sort((left, right) => left.sort_order - right.sort_order || left.id - right.id)

  const childrenMap = new Map<number, Category[]>()
  const rootCategories: Category[] = []

  for (const category of filtered) {
    if (!category.parent_id) {
      rootCategories.push(category)
      continue
    }

    const siblings = childrenMap.get(category.parent_id) || []
    siblings.push(category)
    childrenMap.set(category.parent_id, siblings)
  }

  const result: CategoryOption[] = []

  function visitCategory(category: Category, depth: number): void {
    result.push({
      id: category.id,
      label: `${'　'.repeat(depth)}${category.name}`,
    })

    const children = (childrenMap.get(category.id) || []).sort(
      (left, right) => left.sort_order - right.sort_order || left.id - right.id,
    )
    for (const child of children) {
      visitCategory(child, depth + 1)
    }
  }

  for (const category of rootCategories) {
    visitCategory(category, 0)
  }

  return result
}

function listIndustryCategories(categories: Category[], industryId: number): Category[] {
  return categories
    .filter((category) => category.industry_id === industryId)
    .sort((left, right) => left.sort_order - right.sort_order || left.id - right.id)
}

function buildCategoryDepthMap(categories: Category[], industryId: number): Map<number, number> {
  const targetCategories = categories.filter((category) => category.industry_id === industryId)
  const categoryMap = new Map(targetCategories.map((category) => [category.id, category]))
  const depthMap = new Map<number, number>()

  function resolveDepth(category: Category): number {
    const cachedDepth = depthMap.get(category.id)
    if (cachedDepth !== undefined) {
      return cachedDepth
    }

    if (!category.parent_id) {
      depthMap.set(category.id, 0)
      return 0
    }

    const parent = categoryMap.get(category.parent_id)
    const depth = parent ? resolveDepth(parent) + 1 : 0
    depthMap.set(category.id, depth)
    return depth
  }

  for (const category of targetCategories) {
    resolveDepth(category)
  }

  return depthMap
}

function validateIndustryForm(form: IndustryFormState): string {
  if (!form.code.trim()) {
    return '行业代码不能为空'
  }
  if (!form.name.trim()) {
    return '行业名称不能为空'
  }

  return ''
}

function validateCategoryForm(form: CategoryFormState): string {
  if (!form.industryId) {
    return '请先选择所属行业'
  }
  if (!form.name.trim()) {
    return '分类名称不能为空'
  }

  return ''
}

export function TaxonomyPage() {
  const accessToken = useAdminAuthStore((state) => state.accessToken)
  const queryClient = useQueryClient()
  const [selectedIndustryId, setSelectedIndustryId] = useState<number | null>(null)
  const [selectedCategoryId, setSelectedCategoryId] = useState<number | null>(null)
  const [industryForm, setIndustryForm] = useState<IndustryFormState>(buildInitialIndustryForm())
  const [categoryForm, setCategoryForm] = useState<CategoryFormState>(buildInitialCategoryForm())
  const [editorMessage, setEditorMessage] = useState('读取行业与分类中')

  const industriesQuery = useQuery({
    queryKey: ['admin', 'industries', accessToken],
    queryFn: () => fetchIndustries(accessToken),
    enabled: Boolean(accessToken),
  })

  const categoriesQuery = useQuery({
    queryKey: ['admin', 'categories', accessToken],
    queryFn: () => fetchCategories(accessToken),
    enabled: Boolean(accessToken),
  })

  const currentIndustry = useMemo(
    () => (industriesQuery.data || []).find((industry) => industry.id === selectedIndustryId) || null,
    [industriesQuery.data, selectedIndustryId],
  )
  const currentCategory = useMemo(
    () => (categoriesQuery.data || []).find((category) => category.id === selectedCategoryId) || null,
    [categoriesQuery.data, selectedCategoryId],
  )
  const industryCategories = useMemo(
    () => listIndustryCategories(categoriesQuery.data || [], selectedIndustryId || 0),
    [categoriesQuery.data, selectedIndustryId],
  )
  const categoryDepthMap = useMemo(
    () => buildCategoryDepthMap(categoriesQuery.data || [], selectedIndustryId || 0),
    [categoriesQuery.data, selectedIndustryId],
  )
  const parentCategoryOptions = useMemo(
    () => buildCategoryOptions(categoriesQuery.data || [], categoryForm.industryId, selectedCategoryId),
    [categoriesQuery.data, categoryForm.industryId, selectedCategoryId],
  )
  const industryFormError = useMemo(() => validateIndustryForm(industryForm), [industryForm])
  const categoryFormError = useMemo(() => validateCategoryForm(categoryForm), [categoryForm])

  useEffect(() => {
    if (!industriesQuery.data?.length) {
      return
    }

    if (selectedIndustryId === null) {
      const firstIndustryId = industriesQuery.data[0].id
      setSelectedIndustryId(firstIndustryId)
      setIndustryForm(buildIndustryForm(industriesQuery.data[0]))
      setCategoryForm(buildInitialCategoryForm(String(firstIndustryId)))
      setEditorMessage((current) => (current === '读取行业与分类中' ? '已同步行业与分类列表。' : current))
      return
    }

    const nextIndustry = industriesQuery.data.find((industry) => industry.id === selectedIndustryId)
    if (nextIndustry) {
      setIndustryForm(buildIndustryForm(nextIndustry))
    }
  }, [industriesQuery.data, selectedIndustryId])

  useEffect(() => {
    if (!selectedIndustryId) {
      return
    }

    if (!selectedCategoryId) {
      setCategoryForm((current) =>
        current.industryId === String(selectedIndustryId)
          ? current
          : buildInitialCategoryForm(String(selectedIndustryId)),
      )
      return
    }

    const nextCategory = (categoriesQuery.data || []).find((category) => category.id === selectedCategoryId)
    if (nextCategory) {
      setCategoryForm(buildCategoryForm(nextCategory))
    }
  }, [categoriesQuery.data, selectedCategoryId, selectedIndustryId])

  const industrySaveMutation = useMutation({
    mutationFn: async () => {
      const payload = buildIndustryPayload(industryForm)

      if (selectedIndustryId) {
        await updateIndustry(accessToken, selectedIndustryId, payload)
        return selectedIndustryId
      }

      const created = await createIndustry(accessToken, payload)
      return created?.id
    },
    onSuccess: async (industryId) => {
      setSelectedIndustryId(industryId)
      const msg = selectedIndustryId ? '行业已更新。' : '行业已创建。'
      setEditorMessage(msg)
      message.success(msg)
      await queryClient.invalidateQueries({
        queryKey: ['admin', 'industries'],
      })
    },
    onError: (error) => {
      const msg = extractErrorMessage(error, '保存行业失败，请稍后重试')
      setEditorMessage(msg)
      message.error(msg)
    },
  })

  const categorySaveMutation = useMutation({
    mutationFn: async () => {
      const payload = buildCategoryPayload(categoryForm)

      if (selectedCategoryId) {
        await updateCategory(accessToken, selectedCategoryId, payload)
        return selectedCategoryId
      }

      const created = await createCategory(accessToken, payload)
      return created?.id
    },
    onSuccess: async (categoryId) => {
      setSelectedCategoryId(categoryId)
      const msg = selectedCategoryId ? '分类已更新。' : '分类已创建。'
      setEditorMessage(msg)
      message.success(msg)
      await queryClient.invalidateQueries({
        queryKey: ['admin', 'categories'],
      })
    },
    onError: (error) => {
      const msg = extractErrorMessage(error, '保存分类失败，请稍后重试')
      setEditorMessage(msg)
      message.error(msg)
    },
  })

  const categoryDeleteMutation = useMutation({
    mutationFn: async (categoryId: number) => {
      await deleteCategory(accessToken, categoryId)
    },
    onSuccess: async () => {
      setSelectedCategoryId(null)
      setCategoryForm(buildInitialCategoryForm(selectedIndustryId ? String(selectedIndustryId) : ''))
      setEditorMessage('分类已删除。')
      message.success('分类已删除')
      await queryClient.invalidateQueries({
        queryKey: ['admin', 'categories'],
      })
    },
    onError: (error) => {
      const msg = extractErrorMessage(error, '删除分类失败，请稍后重试')
      setEditorMessage(msg)
      message.error(msg)
    },
  })

  function startCreatingIndustry(): void {
    setSelectedIndustryId(null)
    setIndustryForm(buildInitialIndustryForm())
    setEditorMessage('已切换到新建行业模式。')
  }

  function startEditingIndustry(industry: Industry): void {
    setSelectedIndustryId(industry.id)
    setSelectedCategoryId(null)
    setIndustryForm(buildIndustryForm(industry))
    setCategoryForm(buildInitialCategoryForm(String(industry.id)))
    setEditorMessage(`正在编辑行业：${industry.name}`)
  }

  function startCreatingCategory(): void {
    setSelectedCategoryId(null)
    setCategoryForm(buildInitialCategoryForm(selectedIndustryId ? String(selectedIndustryId) : categoryForm.industryId))
    setEditorMessage('已切换到新建分类模式。')
  }

  function startEditingCategory(category: Category): void {
    setSelectedCategoryId(category.id)
    setCategoryForm(buildCategoryForm(category))
    setEditorMessage(`正在编辑分类：${category.name}`)
  }

  function updateIndustryField<Key extends keyof IndustryFormState>(key: Key, value: IndustryFormState[Key]): void {
    setIndustryForm((current) => ({
      ...current,
      [key]: value,
    }))
  }

  function updateCategoryField<Key extends keyof CategoryFormState>(key: Key, value: CategoryFormState[Key]): void {
    setCategoryForm((current) => ({
      ...current,
      [key]: value,
    }))
  }

  function handleIndustrySubmit(event: FormEvent<HTMLFormElement>): void {
    event.preventDefault()

    if (industryFormError) {
      message.warning(industryFormError)
      return
    }

    setEditorMessage(selectedIndustryId ? '正在更新行业。' : '正在创建行业。')
    industrySaveMutation.mutate()
  }

  function handleCategorySubmit(event: FormEvent<HTMLFormElement>): void {
    event.preventDefault()

    if (categoryFormError) {
      message.warning(categoryFormError)
      return
    }

    setEditorMessage(selectedCategoryId ? '正在更新分类。' : '正在创建分类。')
    categorySaveMutation.mutate()
  }

  function handleDeleteCategory(): void {
    if (!selectedCategoryId) {
      return
    }

    Modal.confirm({
      title: '确认删除分类',
      content: '删除后不可恢复，确定要继续吗？',
      okText: '删除',
      okType: 'danger',
      cancelText: '取消',
      onOk: () => {
        setEditorMessage('正在删除分类。')
        categoryDeleteMutation.mutate(selectedCategoryId)
      },
    })
  }

  if (industriesQuery.isLoading || categoriesQuery.isLoading) {
    return (
      <div style={{ padding: 40, display: 'flex', justifyContent: 'center', alignItems: 'center', minHeight: 400 }}>
        <Spin size="large" tip="正在加载行业和分类数据..." />
      </div>
    )
  }

  if (industriesQuery.isError || categoriesQuery.isError) {
    return (
      <div style={{ padding: 40 }}>
        <div style={{ ...solidCard, padding: 40, textAlign: 'center' }}>
          <InfoCircleOutlined style={{ fontSize: 48, color: THEME.danger, marginBottom: 16 }} />
          <h3 style={{ color: THEME.textMain, marginBottom: 8 }}>数据加载失败</h3>
          <p style={{ color: THEME.textSecondary }}>
            {extractErrorMessage(industriesQuery.error || categoriesQuery.error, '读取行业与分类数据失败')}
          </p>
        </div>
      </div>
    )
  }

  const industryCount = industriesQuery.data?.length || 0
  const categoryCount = categoriesQuery.data?.length || 0

  return (
    <div style={{ padding: '32px 28px', background: THEME.bg, minHeight: '100vh' }}>
      {/* Hero */}
      <div style={{ ...glassCard, padding: '28px 32px', marginBottom: 24, display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
        <div>
          <Space align="center" style={{ marginBottom: 8 }}>
            <Tag color="processing" style={{ fontSize: 12, fontWeight: 600, borderRadius: 20, padding: '2px 12px' }}>
              基础数据
            </Tag>
          </Space>
          <h2 style={{ margin: 0, fontSize: 24, fontWeight: 700, color: THEME.textMain }}>行业与分类</h2>
          <p style={{ margin: '8px 0 0', color: THEME.textSecondary, fontSize: 14, maxWidth: 600 }}>
            集中维护题库、Prompt、Live2D 等功能依赖的底层行业与分类数据。左侧优先处理行业，右侧在当前行业上下文里维护分类树。
          </p>
        </div>
        <div style={{ textAlign: 'right', flexShrink: 0, marginLeft: 24 }}>
          <div style={{ display: 'flex', gap: 24, alignItems: 'center' }}>
            <div style={{ textAlign: 'center' }}>
              <div style={{ fontSize: 32, fontWeight: 800, color: THEME.primary, lineHeight: 1 }}>{industryCount}</div>
              <div style={{ fontSize: 12, color: THEME.textMuted, marginTop: 4 }}>个行业</div>
            </div>
            <Divider type="vertical" style={{ height: 40 }} />
            <div style={{ textAlign: 'center' }}>
              <div style={{ fontSize: 32, fontWeight: 800, color: THEME.accent, lineHeight: 1 }}>{categoryCount}</div>
              <div style={{ fontSize: 12, color: THEME.textMuted, marginTop: 4 }}>个分类</div>
            </div>
          </div>
        </div>
      </div>

      {/* Main Layout */}
      <Row gutter={[24, 24]}>
        {/* Industry Section */}
        <Col xs={24} lg={12}>
          <div style={{ ...solidCard, padding: 24 }}>
            {/* Section Header */}
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 20 }}>
              <div>
                <h3 style={{ margin: 0, fontSize: 16, fontWeight: 700, color: THEME.textMain }}>行业管理</h3>
                <p style={{ margin: '4px 0 0', fontSize: 13, color: THEME.textMuted }}>控制题库和学习路径的一级行业入口</p>
              </div>
              <Button
                type="primary"
                icon={<PlusOutlined />}
                onClick={startCreatingIndustry}
                style={{
                  borderRadius: 10,
                  background: THEME.primary,
                  borderColor: THEME.primary,
                  fontWeight: 600,
                }}
              >
                新建行业
              </Button>
            </div>

            {/* Industry List */}
            <div style={{ display: 'flex', flexDirection: 'column', gap: 10, marginBottom: 24, maxHeight: 320, overflowY: 'auto', paddingRight: 4 }}>
              {(industriesQuery.data || []).map((industry) => {
                const isActive = currentIndustry?.id === industry.id
                return (
                  <button
                    key={industry.id}
                    type="button"
                    onClick={() => startEditingIndustry(industry)}
                    style={{
                      display: 'flex',
                      flexDirection: 'column',
                      gap: 6,
                      padding: '14px 18px',
                      borderRadius: 12,
                      border: isActive ? '1px solid ' + THEME.primary : '1px solid ' + THEME.border,
                      background: isActive ? THEME.primaryLight : THEME.cardBg,
                      cursor: 'pointer',
                      textAlign: 'left',
                      transition: 'all 0.2s ease',
                      position: 'relative',
                    }}
                    onMouseEnter={(e) => {
                      if (!isActive) {
                        e.currentTarget.style.background = '#f8fafc'
                      }
                    }}
                    onMouseLeave={(e) => {
                      if (!isActive) {
                        e.currentTarget.style.background = THEME.cardBg
                      }
                    }}
                  >
                    {isActive && (
                      <span
                        style={{
                          position: 'absolute',
                          left: 0,
                          top: 10,
                          bottom: 10,
                          width: 4,
                          borderRadius: '0 4px 4px 0',
                          background: THEME.primary,
                        }}
                      />
                    )}
                    <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                      <strong style={{ fontSize: 14, color: THEME.textMain, fontWeight: 600 }}>{industry.name}</strong>
                      <Tag color={industry.is_active ? 'success' : 'default'} style={{ fontSize: 11, borderRadius: 10 }}>
                        {industry.is_active ? '启用中' : '已停用'}
                      </Tag>
                    </div>
                    <div style={{ display: 'flex', gap: 12, fontSize: 12, color: THEME.textMuted }}>
                      <span><TagOutlined style={{ marginRight: 4, fontSize: 10 }} />代码 {industry.code}</span>
                      <span><SortAscendingOutlined style={{ marginRight: 4, fontSize: 10 }} />排序 {industry.sort_order}</span>
                    </div>
                    <p style={{ margin: 0, fontSize: 12, color: THEME.textSecondary, lineHeight: 1.5 }}>
                      {industry.description || '当前行业暂无补充说明。'}
                    </p>
                  </button>
                )
              })}
            </div>

            {/* Industry Editor */}
            <form onSubmit={handleIndustrySubmit}>
              <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 16 }}>
                <h4 style={{ margin: 0, fontSize: 15, fontWeight: 700, color: THEME.textMain }}>
                  {selectedIndustryId ? '编辑行业' : '新建行业'}
                </h4>
                {selectedIndustryId && (
                  <Tag color="processing" style={{ borderRadius: 10 }}>ID #{selectedIndustryId}</Tag>
                )}
              </div>

              <Row gutter={[16, 16]}>
                <Col span={12}>
                  <div style={{ marginBottom: 4, fontSize: 13, fontWeight: 600, color: THEME.textMain }}>行业代码</div>
                  <Input
                    value={industryForm.code}
                    onChange={(e) => updateIndustryField('code', e.target.value)}
                    placeholder="例如 go"
                    style={{ borderRadius: 10 }}
                  />
                </Col>
                <Col span={12}>
                  <div style={{ marginBottom: 4, fontSize: 13, fontWeight: 600, color: THEME.textMain }}>行业名称</div>
                  <Input
                    value={industryForm.name}
                    onChange={(e) => updateIndustryField('name', e.target.value)}
                    placeholder="例如 Go 开发"
                    style={{ borderRadius: 10 }}
                  />
                </Col>
                <Col span={12}>
                  <div style={{ marginBottom: 4, fontSize: 13, fontWeight: 600, color: THEME.textMain }}>图标</div>
                  <Input
                    value={industryForm.icon}
                    onChange={(e) => updateIndustryField('icon', e.target.value)}
                    placeholder="图标 URL 或标识"
                    style={{ borderRadius: 10 }}
                  />
                </Col>
                <Col span={12}>
                  <div style={{ marginBottom: 4, fontSize: 13, fontWeight: 600, color: THEME.textMain }}>排序权重</div>
                  <InputNumber
                    value={Number(industryForm.sortOrder)}
                    onChange={(val) => updateIndustryField('sortOrder', String(val ?? 0))}
                    style={{ width: '100%', borderRadius: 10 }}
                  />
                </Col>
                <Col span={24}>
                  <div style={{ marginBottom: 4, fontSize: 13, fontWeight: 600, color: THEME.textMain }}>行业说明</div>
                  <Input.TextArea
                    value={industryForm.description}
                    onChange={(e) => updateIndustryField('description', e.target.value)}
                    placeholder="补充这个行业的范围和用途"
                    rows={3}
                    style={{ borderRadius: 10, resize: 'none' }}
                  />
                </Col>
                <Col span={24}>
                  <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
                    <Switch
                      checked={industryForm.isActive}
                      onChange={(checked) => updateIndustryField('isActive', checked)}
                    />
                    <span style={{ fontSize: 13, color: THEME.textSecondary }}>
                      {industryForm.isActive ? '当前行业启用中' : '当前行业已停用'}
                    </span>
                  </div>
                </Col>
              </Row>

              <div
                style={{
                  marginTop: 16,
                  padding: '12px 16px',
                  borderRadius: 10,
                  background: industryFormError ? 'rgba(239,68,68,0.06)' : 'rgba(16,185,129,0.06)',
                  border: industryFormError ? '1px solid rgba(239,68,68,0.15)' : '1px solid rgba(16,185,129,0.15)',
                  fontSize: 13,
                  color: industryFormError ? THEME.danger : THEME.success,
                  display: 'flex',
                  alignItems: 'center',
                  gap: 8,
                }}
              >
                <InfoCircleOutlined />
                {industryFormError || '行业表单已通过基础校验，可以提交保存。'}
              </div>

              <div style={{ marginTop: 16, display: 'flex', justifyContent: 'flex-end' }}>
                <Button
                  type="primary"
                  htmlType="submit"
                  icon={<SaveOutlined />}
                  loading={industrySaveMutation.isPending}
                  disabled={Boolean(industryFormError)}
                  style={{
                    borderRadius: 10,
                    background: THEME.primary,
                    borderColor: THEME.primary,
                    fontWeight: 600,
                    minWidth: 120,
                  }}
                >
                  {industrySaveMutation.isPending ? '保存中...' : selectedIndustryId ? '保存行业' : '创建行业'}
                </Button>
              </div>
            </form>
          </div>
        </Col>

        {/* Category Section */}
        <Col xs={24} lg={12}>
          <div style={{ ...solidCard, padding: 24 }}>
            {/* Section Header */}
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 20 }}>
              <div>
                <h3 style={{ margin: 0, fontSize: 16, fontWeight: 700, color: THEME.textMain }}>分类管理</h3>
                <p style={{ margin: '4px 0 0', fontSize: 13, color: THEME.textMuted }}>
                  {currentIndustry ? `当前行业：${currentIndustry.name}` : '先选择一个行业后再维护分类。'}
                </p>
              </div>
              <Button
                type="primary"
                icon={<PlusOutlined />}
                onClick={startCreatingCategory}
                disabled={!currentIndustry}
                style={{
                  borderRadius: 10,
                  background: THEME.primary,
                  borderColor: THEME.primary,
                  fontWeight: 600,
                }}
              >
                新建分类
              </Button>
            </div>

            {/* Category List */}
            <div style={{ display: 'flex', flexDirection: 'column', gap: 10, marginBottom: 24, maxHeight: 320, overflowY: 'auto', paddingRight: 4 }}>
              {currentIndustry && industryCategories.length === 0 ? (
                <Empty
                  image={Empty.PRESENTED_IMAGE_SIMPLE}
                  description={
                    <div>
                      <strong style={{ color: THEME.textMain }}>当前行业还没有分类</strong>
                      <p style={{ color: THEME.textMuted, fontSize: 13, margin: '4px 0 0' }}>可以先创建顶级分类，再继续补充子分类。</p>
                    </div>
                  }
                />
              ) : null}

              {industryCategories.map((category) => {
                const isActive = currentCategory?.id === category.id
                const depth = categoryDepthMap.get(category.id) || 0
                return (
                  <button
                    key={category.id}
                    type="button"
                    onClick={() => startEditingCategory(category)}
                    style={{
                      display: 'flex',
                      flexDirection: 'column',
                      gap: 6,
                      padding: '14px 18px',
                      borderRadius: 12,
                      border: isActive ? '1px solid ' + THEME.primary : '1px solid ' + THEME.border,
                      background: isActive ? THEME.primaryLight : THEME.cardBg,
                      cursor: 'pointer',
                      textAlign: 'left',
                      transition: 'all 0.2s ease',
                      position: 'relative',
                      paddingLeft: `${18 + depth * 14}px`,
                    }}
                    onMouseEnter={(e) => {
                      if (!isActive) {
                        e.currentTarget.style.background = '#f8fafc'
                      }
                    }}
                    onMouseLeave={(e) => {
                      if (!isActive) {
                        e.currentTarget.style.background = THEME.cardBg
                      }
                    }}
                  >
                    {isActive && (
                      <span
                        style={{
                          position: 'absolute',
                          left: 0,
                          top: 10,
                          bottom: 10,
                          width: 4,
                          borderRadius: '0 4px 4px 0',
                          background: THEME.primary,
                        }}
                      />
                    )}
                    <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                      <strong style={{ fontSize: 14, color: THEME.textMain, fontWeight: 600 }}>{category.name}</strong>
                      <span style={{ fontSize: 11, color: THEME.textMuted }}>排序 {category.sort_order}</span>
                    </div>
                    <div style={{ fontSize: 12, color: THEME.textMuted }}>
                      父级 {category.parent_id || '顶级'}
                    </div>
                    <p style={{ margin: 0, fontSize: 12, color: THEME.textSecondary, lineHeight: 1.5 }}>
                      {category.description || '当前分类暂无补充说明。'}
                    </p>
                  </button>
                )
              })}
            </div>

            {/* Category Editor */}
            <form onSubmit={handleCategorySubmit}>
              <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 16 }}>
                <h4 style={{ margin: 0, fontSize: 15, fontWeight: 700, color: THEME.textMain }}>
                  {selectedCategoryId ? '编辑分类' : '新建分类'}
                </h4>
                {selectedCategoryId && (
                  <Tag color="processing" style={{ borderRadius: 10 }}>ID #{selectedCategoryId}</Tag>
                )}
              </div>

              <Row gutter={[16, 16]}>
                <Col span={12}>
                  <div style={{ marginBottom: 4, fontSize: 13, fontWeight: 600, color: THEME.textMain }}>所属行业</div>
                  <Select
                    value={categoryForm.industryId || undefined}
                    onChange={(val) => updateCategoryField('industryId', val || '')}
                    placeholder="请选择行业"
                    style={{ width: '100%', borderRadius: 10 }}
                    dropdownStyle={{ borderRadius: 10 }}
                  >
                    {(industriesQuery.data || []).map((industry) => (
                      <Select.Option key={industry.id} value={String(industry.id)}>
                        {industry.name}
                      </Select.Option>
                    ))}
                  </Select>
                </Col>
                <Col span={12}>
                  <div style={{ marginBottom: 4, fontSize: 13, fontWeight: 600, color: THEME.textMain }}>父级分类</div>
                  <Select
                    value={categoryForm.parentId || undefined}
                    onChange={(val) => updateCategoryField('parentId', val || '0')}
                    placeholder="请选择父级分类"
                    style={{ width: '100%', borderRadius: 10 }}
                    dropdownStyle={{ borderRadius: 10 }}
                  >
                    <Select.Option value="0">顶级分类</Select.Option>
                    {parentCategoryOptions.map((option) => (
                      <Select.Option key={option.id} value={String(option.id)}>
                        {option.label}
                      </Select.Option>
                    ))}
                  </Select>
                </Col>
                <Col span={12}>
                  <div style={{ marginBottom: 4, fontSize: 13, fontWeight: 600, color: THEME.textMain }}>分类名称</div>
                  <Input
                    value={categoryForm.name}
                    onChange={(e) => updateCategoryField('name', e.target.value)}
                    placeholder="例如 并发编程"
                    style={{ borderRadius: 10 }}
                  />
                </Col>
                <Col span={12}>
                  <div style={{ marginBottom: 4, fontSize: 13, fontWeight: 600, color: THEME.textMain }}>排序权重</div>
                  <InputNumber
                    value={Number(categoryForm.sortOrder)}
                    onChange={(val) => updateCategoryField('sortOrder', String(val ?? 0))}
                    style={{ width: '100%', borderRadius: 10 }}
                  />
                </Col>
                <Col span={24}>
                  <div style={{ marginBottom: 4, fontSize: 13, fontWeight: 600, color: THEME.textMain }}>图标</div>
                  <Input
                    value={categoryForm.icon}
                    onChange={(e) => updateCategoryField('icon', e.target.value)}
                    placeholder="图标 URL 或标识"
                    style={{ borderRadius: 10 }}
                  />
                </Col>
                <Col span={24}>
                  <div style={{ marginBottom: 4, fontSize: 13, fontWeight: 600, color: THEME.textMain }}>分类说明</div>
                  <Input.TextArea
                    value={categoryForm.description}
                    onChange={(e) => updateCategoryField('description', e.target.value)}
                    placeholder="补充该分类的题目范围"
                    rows={3}
                    style={{ borderRadius: 10, resize: 'none' }}
                  />
                </Col>
              </Row>

              <div
                style={{
                  marginTop: 16,
                  padding: '12px 16px',
                  borderRadius: 10,
                  background: categoryFormError ? 'rgba(239,68,68,0.06)' : 'rgba(16,185,129,0.06)',
                  border: categoryFormError ? '1px solid rgba(239,68,68,0.15)' : '1px solid rgba(16,185,129,0.15)',
                  fontSize: 13,
                  color: categoryFormError ? THEME.danger : THEME.success,
                  display: 'flex',
                  alignItems: 'center',
                  gap: 8,
                }}
              >
                <InfoCircleOutlined />
                {categoryFormError || '分类表单已通过基础校验，可以提交保存。'}
              </div>

              <div style={{ marginTop: 16, display: 'flex', justifyContent: 'space-between' }}>
                {selectedCategoryId ? (
                  <Button
                    danger
                    icon={<DeleteOutlined />}
                    onClick={handleDeleteCategory}
                    loading={categoryDeleteMutation.isPending}
                    disabled={categorySaveMutation.isPending}
                    style={{ borderRadius: 10, fontWeight: 600 }}
                  >
                    {categoryDeleteMutation.isPending ? '删除中...' : '删除分类'}
                  </Button>
                ) : <span />}
                <Button
                  type="primary"
                  htmlType="submit"
                  icon={<SaveOutlined />}
                  loading={categorySaveMutation.isPending}
                  disabled={Boolean(categoryFormError)}
                  style={{
                    borderRadius: 10,
                    background: THEME.primary,
                    borderColor: THEME.primary,
                    fontWeight: 600,
                    minWidth: 120,
                  }}
                >
                  {categorySaveMutation.isPending ? '保存中...' : selectedCategoryId ? '保存分类' : '创建分类'}
                </Button>
              </div>
            </form>
          </div>
        </Col>
      </Row>
    </div>
  )
}
