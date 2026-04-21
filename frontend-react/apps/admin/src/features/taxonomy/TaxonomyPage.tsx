import type { FormEvent } from 'react'
import { useEffect, useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
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

/**
 * 获取后台行业列表，供基础数据页统一维护。
 */
async function fetchIndustries(token: string | null): Promise<Industry[]> {
  const response = await requestJson<ApiEnvelope<Industry[]>>('/admin/industries', {
    method: 'GET',
    token,
  })

  if (!isSuccessCode(response.code)) {
    throw new Error(response.message || '获取行业列表失败')
  }

  return response.data
}

/**
 * 获取后台分类列表，供行业页联动管理题库分类。
 */
async function fetchCategories(token: string | null): Promise<Category[]> {
  const response = await requestJson<ApiEnvelope<Category[]>>('/admin/categories', {
    method: 'GET',
    token,
  })

  if (!isSuccessCode(response.code)) {
    throw new Error(response.message || '获取分类列表失败')
  }

  return response.data
}

/**
 * 创建新的行业记录。
 */
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

/**
 * 更新指定行业记录。
 */
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

/**
 * 创建新的分类记录。
 */
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

/**
 * 更新指定分类记录。
 */
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

/**
 * 删除指定分类记录。
 */
async function deleteCategory(token: string | null, id: number): Promise<void> {
  const response = await requestJson<ApiEnvelope<null>>(`/admin/categories/${id}`, {
    method: 'DELETE',
    token,
  })

  if (!isSuccessCode(response.code)) {
    throw new Error(response.message || '删除分类失败')
  }
}

/**
 * 构造行业表单初始值，便于新建时复用。
 */
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

/**
 * 将行业记录转换为可编辑表单。
 */
function buildIndustryForm(industry?: Industry | null): IndustryFormState {
  if (!industry) {
    return buildInitialIndustryForm()
  }

  return {
    code: industry.code,
    name: industry.name,
    description: industry.description || '',
    icon: industry.icon || '',
    sortOrder: String(industry.sort_order),
    isActive: industry.is_active,
  }
}

/**
 * 构造分类表单初始值，默认绑定到当前选中的行业。
 */
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

/**
 * 将分类记录转换为可编辑表单。
 */
function buildCategoryForm(category?: Category | null): CategoryFormState {
  if (!category) {
    return buildInitialCategoryForm()
  }

  return {
    industryId: String(category.industry_id),
    name: category.name,
    parentId: String(category.parent_id || 0),
    sortOrder: String(category.sort_order),
    icon: category.icon || '',
    description: category.description || '',
  }
}

/**
 * 将行业表单转换为后端请求体。
 */
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

/**
 * 将分类表单转换为后端请求体，兼容顶级分类的 parent_id=0。
 */
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

/**
 * 生成指定行业下的层级分类选项，便于分类编辑时选择父级。
 */
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

  /**
   * 深度优先展开分类层级，并补上缩进标签。
   */
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

/**
 * 返回指定行业下的分类列表，供右侧分类列表展示。
 */
function listIndustryCategories(categories: Category[], industryId: number): Category[] {
  return categories
    .filter((category) => category.industry_id === industryId)
    .sort((left, right) => left.sort_order - right.sort_order || left.id - right.id)
}

/**
 * 计算分类的层级深度，用于列表区视觉缩进。
 */
function buildCategoryDepthMap(categories: Category[], industryId: number): Map<number, number> {
  const targetCategories = categories.filter((category) => category.industry_id === industryId)
  const categoryMap = new Map(targetCategories.map((category) => [category.id, category]))
  const depthMap = new Map<number, number>()

  /**
   * 递归计算单个分类的层级深度，并缓存结果避免重复遍历。
   */
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

/**
 * 校验行业表单，提前发现缺少必填项的问题。
 */
function validateIndustryForm(form: IndustryFormState): string {
  if (!form.code.trim()) {
    return '行业代码不能为空'
  }
  if (!form.name.trim()) {
    return '行业名称不能为空'
  }

  return ''
}

/**
 * 校验分类表单，提前发现行业和分类名称等必填项问题。
 */
function validateCategoryForm(form: CategoryFormState): string {
  if (!form.industryId) {
    return '请先选择所属行业'
  }
  if (!form.name.trim()) {
    return '分类名称不能为空'
  }

  return ''
}

/**
 * 提供行业与分类基础数据管理页，集中维护题库依赖的底层数据。
 */
export function TaxonomyPage() {
  const accessToken = useAdminAuthStore((state) => state.accessToken)
  const queryClient = useQueryClient()
  const [selectedIndustryId, setSelectedIndustryId] = useState<number | null>(null)
  const [selectedCategoryId, setSelectedCategoryId] = useState<number | null>(null)
  const [industryForm, setIndustryForm] = useState<IndustryFormState>(buildInitialIndustryForm())
  const [categoryForm, setCategoryForm] = useState<CategoryFormState>(buildInitialCategoryForm())
  const [message, setMessage] = useState('读取行业与分类中')

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
      setMessage((current) => (current === '读取行业与分类中' ? '已同步行业与分类列表。' : current))
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
      return created.id
    },
    onSuccess: async (industryId) => {
      setSelectedIndustryId(industryId)
      setMessage(selectedIndustryId ? '行业已更新。' : '行业已创建。')
      await queryClient.invalidateQueries({
        queryKey: ['admin', 'industries'],
      })
    },
    onError: (error) => {
      setMessage(extractErrorMessage(error, '保存行业失败，请稍后重试'))
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
      return created.id
    },
    onSuccess: async (categoryId) => {
      setSelectedCategoryId(categoryId)
      setMessage(selectedCategoryId ? '分类已更新。' : '分类已创建。')
      await queryClient.invalidateQueries({
        queryKey: ['admin', 'categories'],
      })
    },
    onError: (error) => {
      setMessage(extractErrorMessage(error, '保存分类失败，请稍后重试'))
    },
  })

  const categoryDeleteMutation = useMutation({
    mutationFn: async (categoryId: number) => {
      await deleteCategory(accessToken, categoryId)
    },
    onSuccess: async () => {
      setSelectedCategoryId(null)
      setCategoryForm(buildInitialCategoryForm(selectedIndustryId ? String(selectedIndustryId) : ''))
      setMessage('分类已删除。')
      await queryClient.invalidateQueries({
        queryKey: ['admin', 'categories'],
      })
    },
    onError: (error) => {
      setMessage(extractErrorMessage(error, '删除分类失败，请稍后重试'))
    },
  })

  /**
   * 切换到新建行业模式，并清空行业表单。
   */
  function startCreatingIndustry(): void {
    setSelectedIndustryId(null)
    setIndustryForm(buildInitialIndustryForm())
    setMessage('已切换到新建行业模式。')
  }

  /**
   * 装载指定行业到编辑区，同时刷新右侧分类上下文。
   */
  function startEditingIndustry(industry: Industry): void {
    setSelectedIndustryId(industry.id)
    setSelectedCategoryId(null)
    setIndustryForm(buildIndustryForm(industry))
    setCategoryForm(buildInitialCategoryForm(String(industry.id)))
    setMessage(`正在编辑行业：${industry.name}`)
  }

  /**
   * 切换到新建分类模式，并继承当前行业上下文。
   */
  function startCreatingCategory(): void {
    setSelectedCategoryId(null)
    setCategoryForm(buildInitialCategoryForm(selectedIndustryId ? String(selectedIndustryId) : categoryForm.industryId))
    setMessage('已切换到新建分类模式。')
  }

  /**
   * 装载指定分类到右侧编辑区。
   */
  function startEditingCategory(category: Category): void {
    setSelectedCategoryId(category.id)
    setCategoryForm(buildCategoryForm(category))
    setMessage(`正在编辑分类：${category.name}`)
  }

  /**
   * 更新行业表单字段，集中处理输入状态。
   */
  function updateIndustryField<Key extends keyof IndustryFormState>(key: Key, value: IndustryFormState[Key]): void {
    setIndustryForm((current) => ({
      ...current,
      [key]: value,
    }))
  }

  /**
   * 更新分类表单字段，集中处理输入状态。
   */
  function updateCategoryField<Key extends keyof CategoryFormState>(key: Key, value: CategoryFormState[Key]): void {
    setCategoryForm((current) => ({
      ...current,
      [key]: value,
    }))
  }

  /**
   * 提交行业表单并执行创建或更新。
   */
  function handleIndustrySubmit(event: FormEvent<HTMLFormElement>): void {
    event.preventDefault()

    if (industryFormError) {
      setMessage(industryFormError)
      return
    }

    setMessage(selectedIndustryId ? '正在更新行业。' : '正在创建行业。')
    industrySaveMutation.mutate()
  }

  /**
   * 提交分类表单并执行创建或更新。
   */
  function handleCategorySubmit(event: FormEvent<HTMLFormElement>): void {
    event.preventDefault()

    if (categoryFormError) {
      setMessage(categoryFormError)
      return
    }

    setMessage(selectedCategoryId ? '正在更新分类。' : '正在创建分类。')
    categorySaveMutation.mutate()
  }

  /**
   * 删除当前选中的分类记录。
   */
  function handleDeleteCategory(): void {
    if (!selectedCategoryId) {
      return
    }

    if (!window.confirm('确认删除当前分类吗？删除后不可恢复。')) {
      return
    }

    setMessage('正在删除分类。')
    categoryDeleteMutation.mutate(selectedCategoryId)
  }

  if (industriesQuery.isLoading || categoriesQuery.isLoading) {
    return (
      <section className="admin-panel">
        <span className="admin-tag">基础数据</span>
        <h2>行业与分类</h2>
        <p className="admin-copy">正在加载行业和分类数据。</p>
      </section>
    )
  }

  if (industriesQuery.isError || categoriesQuery.isError) {
    return (
      <section className="admin-panel">
        <span className="admin-tag">基础数据</span>
        <h2>行业与分类</h2>
        <p className="admin-copy">
          {extractErrorMessage(industriesQuery.error || categoriesQuery.error, '读取行业与分类数据失败')}
        </p>
      </section>
    )
  }

  return (
    <section className="admin-panel admin-taxonomy-page">
      <div className="admin-taxonomy-page__hero">
        <div>
          <span className="admin-tag">基础数据</span>
          <h2>行业与分类</h2>
          <p className="admin-copy">
            这里集中维护题库、Prompt、Live2D 等功能依赖的底层行业与分类数据。左侧优先处理行业，右侧在当前行业上下文里维护分类树。
          </p>
        </div>
        <div className="admin-taxonomy-page__summary">
          <strong>{industriesQuery.data?.length || 0}</strong>
          <span>个行业</span>
          <small>{categoriesQuery.data?.length || 0} 个分类</small>
        </div>
      </div>

      <p className="admin-taxonomy-page__message">{message}</p>

      <div className="admin-taxonomy-page__layout">
        <section className="admin-taxonomy-section">
          <div className="admin-taxonomy-section__head">
            <div>
              <h3>行业管理</h3>
              <p>控制题库和学习路径的一级行业入口。</p>
            </div>
            <button className="admin-link" type="button" onClick={startCreatingIndustry}>
              新建行业
            </button>
          </div>

          <div className="admin-taxonomy-list">
            {(industriesQuery.data || []).map((industry) => (
              <button
                key={industry.id}
                type="button"
                className={`admin-taxonomy-card ${
                  currentIndustry?.id === industry.id ? 'admin-taxonomy-card--active' : ''
                }`}
                onClick={() => startEditingIndustry(industry)}
              >
                <div className="admin-taxonomy-card__head">
                  <strong>{industry.name}</strong>
                  <span>{industry.is_active ? '启用中' : '已停用'}</span>
                </div>
                <div className="admin-taxonomy-card__meta">
                  <span>代码 {industry.code}</span>
                  <span>排序 {industry.sort_order}</span>
                </div>
                <p>{industry.description || '当前行业暂无补充说明。'}</p>
              </button>
            ))}
          </div>

          <form className="admin-taxonomy-editor" onSubmit={handleIndustrySubmit}>
            <div className="admin-taxonomy-editor__head">
              <h4>{selectedIndustryId ? '编辑行业' : '新建行业'}</h4>
            </div>

            <div className="admin-taxonomy-editor__grid">
              <label className="admin-field">
                <span>行业代码</span>
                <input
                  value={industryForm.code}
                  onChange={(event) => updateIndustryField('code', event.target.value)}
                  placeholder="例如 go"
                />
              </label>

              <label className="admin-field">
                <span>行业名称</span>
                <input
                  value={industryForm.name}
                  onChange={(event) => updateIndustryField('name', event.target.value)}
                  placeholder="例如 Go 开发"
                />
              </label>
            </div>

            <div className="admin-taxonomy-editor__grid">
              <label className="admin-field">
                <span>图标</span>
                <input
                  value={industryForm.icon}
                  onChange={(event) => updateIndustryField('icon', event.target.value)}
                  placeholder="图标 URL 或标识"
                />
              </label>

              <label className="admin-field">
                <span>排序权重</span>
                <input
                  type="number"
                  value={industryForm.sortOrder}
                  onChange={(event) => updateIndustryField('sortOrder', event.target.value)}
                />
              </label>
            </div>

            <label className="admin-field">
              <span>行业说明</span>
              <textarea
                value={industryForm.description}
                onChange={(event) => updateIndustryField('description', event.target.value)}
                placeholder="补充这个行业的范围和用途"
              />
            </label>

            <label className="admin-taxonomy-editor__switch">
              <input
                type="checkbox"
                checked={industryForm.isActive}
                onChange={(event) => updateIndustryField('isActive', event.target.checked)}
              />
              <span>{industryForm.isActive ? '当前行业启用中' : '当前行业已停用'}</span>
            </label>

            <div className={`admin-taxonomy-editor__status ${industryFormError ? 'is-error' : 'is-valid'}`}>
              <span>{industryFormError || '行业表单已通过基础校验，可以提交保存。'}</span>
            </div>

            <div className="admin-taxonomy-editor__actions">
              <button className="admin-link" type="submit" disabled={Boolean(industryFormError) || industrySaveMutation.isPending}>
                {industrySaveMutation.isPending ? '保存中...' : selectedIndustryId ? '保存行业' : '创建行业'}
              </button>
            </div>
          </form>
        </section>

        <section className="admin-taxonomy-section">
          <div className="admin-taxonomy-section__head">
            <div>
              <h3>分类管理</h3>
              <p>{currentIndustry ? `当前行业：${currentIndustry.name}` : '先选择一个行业后再维护分类。'}</p>
            </div>
            <button className="admin-link" type="button" onClick={startCreatingCategory} disabled={!currentIndustry}>
              新建分类
            </button>
          </div>

          <div className="admin-taxonomy-list">
            {currentIndustry && industryCategories.length === 0 ? (
              <div className="admin-taxonomy-card admin-taxonomy-card--empty">
                <strong>当前行业还没有分类</strong>
                <p>可以先创建顶级分类，再继续补充子分类。</p>
              </div>
            ) : null}

            {industryCategories.map((category) => (
              <button
                key={category.id}
                type="button"
                className={`admin-taxonomy-card ${
                  currentCategory?.id === category.id ? 'admin-taxonomy-card--active' : ''
                }`}
                onClick={() => startEditingCategory(category)}
              >
                <div className="admin-taxonomy-card__head">
                  <strong style={{ paddingLeft: `${(categoryDepthMap.get(category.id) || 0) * 16}px` }}>
                    {category.name}
                  </strong>
                  <span>排序 {category.sort_order}</span>
                </div>
                <div className="admin-taxonomy-card__meta">
                  <span>父级 {category.parent_id || '顶级'}</span>
                </div>
                <p>{category.description || '当前分类暂无补充说明。'}</p>
              </button>
            ))}
          </div>

          <form className="admin-taxonomy-editor" onSubmit={handleCategorySubmit}>
            <div className="admin-taxonomy-editor__head">
              <h4>{selectedCategoryId ? '编辑分类' : '新建分类'}</h4>
            </div>

            <div className="admin-taxonomy-editor__grid">
              <label className="admin-field">
                <span>所属行业</span>
                <select
                  value={categoryForm.industryId}
                  onChange={(event) => updateCategoryField('industryId', event.target.value)}
                >
                  <option value="">请选择行业</option>
                  {(industriesQuery.data || []).map((industry) => (
                    <option key={industry.id} value={industry.id}>
                      {industry.name}
                    </option>
                  ))}
                </select>
              </label>

              <label className="admin-field">
                <span>父级分类</span>
                <select
                  value={categoryForm.parentId}
                  onChange={(event) => updateCategoryField('parentId', event.target.value)}
                >
                  <option value="0">顶级分类</option>
                  {parentCategoryOptions.map((option) => (
                    <option key={option.id} value={option.id}>
                      {option.label}
                    </option>
                  ))}
                </select>
              </label>
            </div>

            <div className="admin-taxonomy-editor__grid">
              <label className="admin-field">
                <span>分类名称</span>
                <input
                  value={categoryForm.name}
                  onChange={(event) => updateCategoryField('name', event.target.value)}
                  placeholder="例如 并发编程"
                />
              </label>

              <label className="admin-field">
                <span>排序权重</span>
                <input
                  type="number"
                  value={categoryForm.sortOrder}
                  onChange={(event) => updateCategoryField('sortOrder', event.target.value)}
                />
              </label>
            </div>

            <label className="admin-field">
              <span>图标</span>
              <input
                value={categoryForm.icon}
                onChange={(event) => updateCategoryField('icon', event.target.value)}
                placeholder="图标 URL 或标识"
              />
            </label>

            <label className="admin-field">
              <span>分类说明</span>
              <textarea
                value={categoryForm.description}
                onChange={(event) => updateCategoryField('description', event.target.value)}
                placeholder="补充该分类的题目范围"
              />
            </label>

            <div className={`admin-taxonomy-editor__status ${categoryFormError ? 'is-error' : 'is-valid'}`}>
              <span>{categoryFormError || '分类表单已通过基础校验，可以提交保存。'}</span>
            </div>

            <div className="admin-taxonomy-editor__actions">
              {selectedCategoryId ? (
                <button
                  className="admin-link"
                  type="button"
                  onClick={handleDeleteCategory}
                  disabled={categorySaveMutation.isPending || categoryDeleteMutation.isPending}
                >
                  {categoryDeleteMutation.isPending ? '删除中...' : '删除分类'}
                </button>
              ) : null}
              <button
                className="admin-link"
                type="submit"
                disabled={Boolean(categoryFormError) || categorySaveMutation.isPending}
              >
                {categorySaveMutation.isPending ? '保存中...' : selectedCategoryId ? '保存分类' : '创建分类'}
              </button>
            </div>
          </form>
        </section>
      </div>
    </section>
  )
}
