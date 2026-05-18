import type { FormEvent } from 'react'
import { useEffect, useMemo, useRef, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { extractErrorMessage, requestJson } from '@makejob/api-client'
import { isSuccessCode, type ApiEnvelope } from '@makejob/shared-types'
import { useAdminAuthStore } from '../../state/auth'

type Live2DScene = 'interview' | 'companion'

interface Industry {
  id: number
  code: string
  name: string
  is_active: boolean
}

interface Live2DModel {
  id: number
  name: string
  industry_id: number | null
  scene: Live2DScene
  model_url: string
  thumbnail_url: string
  config_json: string
  is_active: boolean
  created_at?: string
  updated_at?: string
}

interface ImportedLive2DPackage {
  name: string
  asset_dir: string
  model_url: string
  thumbnail_url?: string
  model_id?: number
  created: boolean
  is_active: boolean
}

interface ImportedLive2DBackground {
  file_name: string
  asset_url: string
}

interface Live2DFormState {
  name: string
  industryId: string
  scene: Live2DScene
  modelUrl: string
  thumbnailUrl: string
  backgroundImageUrl: string
  configJson: string
  isActive: boolean
}

const LIVE2D_SCENE_OPTIONS: Array<{ value: Live2DScene; label: string }> = [
  { value: 'companion', label: '学习陪伴' },
  { value: 'interview', label: '面试' },
]

interface Live2DConfigPreview {
  valid: boolean
  error: string
  mergedConfig: Record<string, unknown>
  formattedConfig: string
}

/**
 * 获取后台行业列表，供 Live2D 表单选择行业归属。
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
 * 获取后台当前维护的 Live2D 模型列表。
 */
async function fetchLive2DModels(token: string | null): Promise<Live2DModel[]> {
  const response = await requestJson<ApiEnvelope<Live2DModel[]>>('/admin/live2d-models', {
    method: 'GET',
    token,
  })

  if (!isSuccessCode(response.code)) {
    throw new Error(response.message || '获取 Live2D 模型列表失败')
  }

  return response.data
}

/**
 * 上传 ZIP 模型包并获取后端自动识别出的模型资源地址。
 */
async function importLive2DPackage(token: string | null, file: File): Promise<ImportedLive2DPackage> {
  const body = new FormData()
  body.append('file', file)

  const response = await requestJson<ApiEnvelope<ImportedLive2DPackage>>('/admin/live2d-models/import', {
    method: 'POST',
    token,
    body,
  })

  if (!isSuccessCode(response.code)) {
    throw new Error(response.message || '导入 Live2D 模型包失败')
  }

  return response.data
}

/**
 * 上传舞台背景图并获取后端分配好的静态资源地址。
 */
async function importLive2DBackground(token: string | null, file: File): Promise<ImportedLive2DBackground> {
  const body = new FormData()
  body.append('file', file)

  const response = await requestJson<ApiEnvelope<ImportedLive2DBackground>>('/admin/live2d-models/backgrounds/import', {
    method: 'POST',
    token,
    body,
  })

  if (!isSuccessCode(response.code)) {
    throw new Error(response.message || '导入 Live2D 背景图失败')
  }

  return response.data
}

/**
 * 创建新的 Live2D 模型记录。
 */
async function createLive2DModel(token: string | null, payload: Record<string, unknown>): Promise<Live2DModel> {
  const response = await requestJson<ApiEnvelope<Live2DModel>>('/admin/live2d-models', {
    method: 'POST',
    token,
    body: payload,
  })

  if (!isSuccessCode(response.code)) {
    throw new Error(response.message || '创建 Live2D 模型失败')
  }

  return response.data
}

/**
 * 更新指定的 Live2D 模型记录。
 */
async function updateLive2DModel(token: string | null, id: number, payload: Record<string, unknown>): Promise<void> {
  const response = await requestJson<ApiEnvelope<null>>(`/admin/live2d-models/${id}`, {
    method: 'PUT',
    token,
    body: payload,
  })

  if (!isSuccessCode(response.code)) {
    throw new Error(response.message || '更新 Live2D 模型失败')
  }
}

/**
 * 删除指定的 Live2D 模型记录。
 */
async function deleteLive2DModel(token: string | null, id: number): Promise<void> {
  const response = await requestJson<ApiEnvelope<null>>(`/admin/live2d-models/${id}`, {
    method: 'DELETE',
    token,
  })

  if (!isSuccessCode(response.code)) {
    throw new Error(response.message || '删除 Live2D 模型失败')
  }
}

/**
 * 构造 Live2D 表单的默认空态，供新建模型时复用。
 */
function buildInitialLive2DForm(): Live2DFormState {
  return {
    name: '',
    industryId: '0',
    scene: 'companion',
    modelUrl: '',
    thumbnailUrl: '',
    backgroundImageUrl: '',
    configJson: '{\n  "scale": 0.4,\n  "offset_x": 0,\n  "offset_y": 0.08\n}',
    isActive: true,
  }
}

/**
 * 返回指定场景下的默认 Live2D 渲染配置，便于后台页预览最终生效值。
 */
function buildDefaultLive2DConfig(scene: Live2DScene): Record<string, unknown> {
  if (scene === 'interview') {
    return {
      scale: 0.34,
      offset_x: 0,
      offset_y: 0.02,
      idle_motion: 'interview_idle',
      tap_motion: 'greeting',
      background: 'transparent',
      voice_source: 'volcengine',
    }
  }

  return {
    scale: 0.4,
    offset_x: 0,
    offset_y: 0.08,
    idle_motion: 'companion_idle',
    tap_motion: 'wave',
    background: 'transparent',
    voice_source: 'volcengine',
  }
}

/**
 * 解析配置 JSON 为对象，失败时返回空对象，避免单次编辑态被异常输入打断。
 */
function parseLive2DConfigObject(rawConfig: string): Record<string, unknown> {
  const trimmed = rawConfig.trim()
  if (!trimmed) {
    return {}
  }

  try {
    const parsed = JSON.parse(trimmed) as unknown
    if (!parsed || Array.isArray(parsed) || typeof parsed !== 'object') {
      return {}
    }

    return parsed as Record<string, unknown>
  } catch {
    return {}
  }
}

/**
 * 从配置对象里提取背景图地址，供后台独立输入框回填和前台舞台复用。
 */
function resolveLive2DBackgroundImageUrl(rawConfig: string): string {
  const value = parseLive2DConfigObject(rawConfig).background_image_url
  return typeof value === 'string' ? value.trim() : ''
}

/**
 * 将数据库中的 Live2D 模型记录转换为可编辑表单。
 */
function buildLive2DForm(model?: Live2DModel | null): Live2DFormState {
  if (!model) {
    return buildInitialLive2DForm()
  }

  return {
    name: model.name,
    industryId: String(model.industry_id ?? 0),
    scene: model.scene,
    modelUrl: model.model_url,
    thumbnailUrl: model.thumbnail_url || '',
    backgroundImageUrl: resolveLive2DBackgroundImageUrl(model.config_json || ''),
    configJson: model.config_json || '',
    isActive: model.is_active,
  }
}

/**
 * 将 Live2D 表单状态转换为后端请求体。
 */
function buildLive2DPayload(form: Live2DFormState): Record<string, unknown> {
  const industryId = Number(form.industryId) || 0
  const configObject = parseLive2DConfigObject(form.configJson)
  const backgroundImageUrl = form.backgroundImageUrl.trim()

  if (backgroundImageUrl) {
    configObject.background_image_url = backgroundImageUrl
  } else {
    delete configObject.background_image_url
  }

  return {
    name: form.name.trim(),
    industry_id: industryId > 0 ? industryId : null,
    scene: form.scene,
    model_url: form.modelUrl.trim(),
    thumbnail_url: form.thumbnailUrl.trim(),
    config_json: JSON.stringify(configObject),
    is_active: form.isActive,
  }
}

/**
 * 把场景值转换为更可读的中文标签。
 */
function live2DSceneLabel(scene: string): string {
  return LIVE2D_SCENE_OPTIONS.find((option) => option.value === scene)?.label || scene
}

/**
 * 根据行业 ID 解析模型所归属的行业名。
 */
function resolveIndustryName(industryId: number | null, industryMap: Map<number, Industry>): string {
  if (!industryId) {
    return '通用模型'
  }

  return industryMap.get(industryId)?.name || `行业 #${industryId}`
}

/**
 * 截断较长的资源地址，减少列表区的信息噪音。
 */
function shortenUrl(value: string): string {
  const trimmed = value.trim()
  if (trimmed.length <= 68) {
    return trimmed
  }

  return `${trimmed.slice(0, 68)}...`
}

/**
 * 判断当前模型地址是否像一个可直接加载的 Live2D model3.json 资源。
 */
function looksLikeLive2DModelUrl(value: string): boolean {
  return /\.model3\.json(\?.*)?$/i.test(value.trim())
}

/**
 * 解析当前配置 JSON，并合并场景默认值生成后台页预览。
 */
function buildLive2DConfigPreview(
  scene: Live2DScene,
  rawConfig: string,
  backgroundImageUrl: string,
): Live2DConfigPreview {
  const baseConfig = buildDefaultLive2DConfig(scene)
  const trimmed = rawConfig.trim()

  if (!trimmed) {
    const mergedConfig = {
      ...baseConfig,
      ...(backgroundImageUrl.trim() ? { background_image_url: backgroundImageUrl.trim() } : {}),
    }
    return {
      valid: true,
      error: '',
      mergedConfig,
      formattedConfig: JSON.stringify(mergedConfig, null, 2),
    }
  }

  try {
    const parsed = JSON.parse(trimmed) as unknown
    if (!parsed || Array.isArray(parsed) || typeof parsed !== 'object') {
      return {
        valid: false,
        error: '配置 JSON 必须是对象结构，例如 {"scale":0.4}',
        mergedConfig: baseConfig,
        formattedConfig: JSON.stringify(baseConfig, null, 2),
      }
    }

    const mergedConfig = {
      ...baseConfig,
      ...(parsed as Record<string, unknown>),
    }
    if (backgroundImageUrl.trim()) {
      mergedConfig.background_image_url = backgroundImageUrl.trim()
    } else {
      delete mergedConfig.background_image_url
    }

    return {
      valid: true,
      error: '',
      mergedConfig,
      formattedConfig: JSON.stringify(mergedConfig, null, 2),
    }
  } catch (error) {
    return {
      valid: false,
      error: extractErrorMessage(error, '配置 JSON 解析失败'),
      mergedConfig: baseConfig,
      formattedConfig: JSON.stringify(baseConfig, null, 2),
    }
  }
}

/**
 * 提供后台 Live2D 模型管理页，支持导入、维护和删除模型。
 */
export function Live2DPage() {
  const accessToken = useAdminAuthStore((state) => state.accessToken)
  const queryClient = useQueryClient()
  const packageInputRef = useRef<HTMLInputElement | null>(null)
  const backgroundInputRef = useRef<HTMLInputElement | null>(null)
  const [selectedModelId, setSelectedModelId] = useState<number | null>(null)
  const [form, setForm] = useState<Live2DFormState>(buildInitialLive2DForm())
  const [message, setMessage] = useState('读取 Live2D 模型中')
  const [importFile, setImportFile] = useState<File | null>(null)
  const [backgroundImportFile, setBackgroundImportFile] = useState<File | null>(null)

  const industriesQuery = useQuery({
    queryKey: ['admin', 'industries', accessToken],
    queryFn: () => fetchIndustries(accessToken),
    enabled: Boolean(accessToken),
  })

  const modelsQuery = useQuery({
    queryKey: ['admin', 'live2d-models', accessToken],
    queryFn: () => fetchLive2DModels(accessToken),
    enabled: Boolean(accessToken),
  })

  const industryMap = useMemo(() => {
    return new Map((industriesQuery.data || []).map((industry) => [industry.id, industry]))
  }, [industriesQuery.data])
  const configPreview = useMemo(
    () => buildLive2DConfigPreview(form.scene, form.configJson, form.backgroundImageUrl),
    [form.backgroundImageUrl, form.configJson, form.scene],
  )
  const hasValidModelUrl = useMemo(() => looksLikeLive2DModelUrl(form.modelUrl), [form.modelUrl])
  const hasBackgroundImageUrl = useMemo(() => Boolean(form.backgroundImageUrl.trim()), [form.backgroundImageUrl])
  const canSubmit = useMemo(() => {
    return Boolean(form.name.trim()) && Boolean(form.modelUrl.trim()) && configPreview.valid
  }, [configPreview.valid, form.modelUrl, form.name])

  useEffect(() => {
    if (!modelsQuery.data) {
      return
    }

    if (selectedModelId === null) {
      setMessage((current) => (current === '读取 Live2D 模型中' ? '已同步 Live2D 模型列表。' : current))
      return
    }

    const nextModel = modelsQuery.data.find((item) => item.id === selectedModelId)
    if (nextModel) {
      setForm(buildLive2DForm(nextModel))
    }
  }, [modelsQuery.data, selectedModelId])

  const importMutation = useMutation({
    mutationFn: async () => {
      if (!importFile) {
        throw new Error('请先选择一个 .zip 模型包')
      }

      return importLive2DPackage(accessToken, importFile)
    },
    onSuccess: (result) => {
      setForm((current) => ({
        ...current,
        name: result.name || current.name,
        modelUrl: result.model_url || current.modelUrl,
        thumbnailUrl: result.thumbnail_url || current.thumbnailUrl,
        isActive: result.is_active,
      }))
      if (result.model_id) {
        setSelectedModelId(result.model_id)
      }
      setMessage(
        result.created
          ? `模型包已导入并加入后台待确认列表，资源目录：${result.asset_dir}。当前默认未启用，启用后前台才可见。`
          : `模型资源已存在于后台列表，资源目录：${result.asset_dir}。当前仍需在后台确认启用后前台才可见。`,
      )
      setImportFile(null)
      if (packageInputRef.current) {
        packageInputRef.current.value = ''
      }
      void queryClient.invalidateQueries({
        queryKey: ['admin', 'live2d-models'],
      })
    },
    onError: (error) => {
      setMessage(extractErrorMessage(error, '导入 Live2D 模型包失败，请稍后重试'))
    },
  })

  const importBackgroundMutation = useMutation({
    mutationFn: async () => {
      if (!backgroundImportFile) {
        throw new Error('请先选择一张背景图')
      }

      return importLive2DBackground(accessToken, backgroundImportFile)
    },
    onSuccess: (result) => {
      setForm((current) => ({
        ...current,
        backgroundImageUrl: result.asset_url || current.backgroundImageUrl,
      }))
      setMessage(`背景图导入完成：${result.file_name}`)
      setBackgroundImportFile(null)
      if (backgroundInputRef.current) {
        backgroundInputRef.current.value = ''
      }
    },
    onError: (error) => {
      setMessage(extractErrorMessage(error, '导入 Live2D 背景图失败，请稍后重试'))
    },
  })

  const saveMutation = useMutation({
    mutationFn: async () => {
      const payload = buildLive2DPayload(form)

      if (selectedModelId) {
        await updateLive2DModel(accessToken, selectedModelId, payload)
        return selectedModelId
      }

      const created = await createLive2DModel(accessToken, payload)
      return created.id
    },
    onSuccess: async (modelId) => {
      setSelectedModelId(modelId)
      setMessage(selectedModelId ? 'Live2D 模型已更新。' : 'Live2D 模型已创建。')
      await queryClient.invalidateQueries({
        queryKey: ['admin', 'live2d-models'],
      })
    },
    onError: (error) => {
      setMessage(extractErrorMessage(error, '保存 Live2D 模型失败，请稍后重试'))
    },
  })

  const deleteMutation = useMutation({
    mutationFn: async (id: number) => {
      await deleteLive2DModel(accessToken, id)
    },
    onSuccess: async () => {
      setSelectedModelId(null)
      setForm(buildInitialLive2DForm())
      setMessage('Live2D 模型已删除。')
      await queryClient.invalidateQueries({
        queryKey: ['admin', 'live2d-models'],
      })
    },
    onError: (error) => {
      setMessage(extractErrorMessage(error, '删除 Live2D 模型失败，请稍后重试'))
    },
  })

  /**
   * 切换到新建模式，并清空当前表单。
   */
  function startCreatingModel(): void {
    setSelectedModelId(null)
    setForm(buildInitialLive2DForm())
    setMessage('已切换到新建 Live2D 模型。')
  }

  /**
   * 载入指定模型到右侧编辑区。
   */
  function startEditingModel(model: Live2DModel): void {
    setSelectedModelId(model.id)
    setForm(buildLive2DForm(model))
    setMessage(`正在编辑模型：${model.name}`)
  }

  /**
   * 更新 Live2D 表单字段，统一管理输入状态。
   */
  function updateLive2DField<Key extends keyof Live2DFormState>(key: Key, value: Live2DFormState[Key]): void {
    setForm((current) => ({
      ...current,
      [key]: value,
    }))
  }

  /**
   * 更新原始配置 JSON，并在解析成功时同步回填背景图独立输入框。
   */
  function handleConfigJsonChange(value: string): void {
    setForm((current) => ({
      ...current,
      configJson: value,
      backgroundImageUrl: resolveLive2DBackgroundImageUrl(value) || current.backgroundImageUrl,
    }))
  }

  /**
   * 提交 Live2D 表单并执行创建或更新。
   */
  function handleSubmit(event: FormEvent<HTMLFormElement>): void {
    event.preventDefault()
    setMessage(selectedModelId ? '正在更新 Live2D 模型。' : '正在创建 Live2D 模型。')
    saveMutation.mutate()
  }

  /**
   * 提交模型包导入表单，并把自动识别结果回填到当前编辑表单。
   */
  function handleImport(event: FormEvent<HTMLFormElement>): void {
    event.preventDefault()
    setMessage('正在导入 Live2D 模型包。')
    importMutation.mutate()
  }

  /**
   * 触发模型包文件选择框，并让用户先完成 ZIP 选择再上传。
   */
  function openPackagePicker(): void {
    packageInputRef.current?.click()
  }

  /**
   * 触发舞台背景图文件选择框，避免用户点击上传按钮时没有任何反馈。
   */
  function openBackgroundPicker(): void {
    backgroundInputRef.current?.click()
  }

  /**
   * 执行背景图上传；若尚未选择文件，则直接打开文件选择框并给出提示。
   */
  function handleBackgroundImport(): void {
    if (!backgroundImportFile) {
      setMessage('请先选择一张舞台背景图，再执行上传。')
      openBackgroundPicker()
      return
    }

    setMessage('正在导入 Live2D 背景图。')
    importBackgroundMutation.mutate()
  }

  /**
   * 删除当前选中的 Live2D 模型记录。
   */
  function handleDelete(): void {
    if (!selectedModelId) {
      return
    }

    if (!window.confirm('确认删除当前 Live2D 模型吗？删除后不可恢复。')) {
      return
    }

    setMessage('正在删除 Live2D 模型。')
    deleteMutation.mutate(selectedModelId)
  }

  if (modelsQuery.isLoading || industriesQuery.isLoading) {
    return (
      <section className="admin-panel">
        <span className="admin-tag">Live2D 中心</span>
        <h2>Live2D 管理</h2>
        <p className="admin-copy">正在加载模型资产与行业数据。</p>
      </section>
    )
  }

  if (modelsQuery.isError || industriesQuery.isError) {
    return (
      <section className="admin-panel">
        <span className="admin-tag">Live2D 中心</span>
        <h2>Live2D 管理</h2>
        <p className="admin-copy">
          {extractErrorMessage(modelsQuery.error || industriesQuery.error, '读取 Live2D 管理数据失败')}
        </p>
      </section>
    )
  }

  return (
    <section className="admin-panel admin-live2d-page">
      <div className="admin-live2d-page__hero">
        <div>
          <span className="admin-tag">Live2D 中心</span>
          <h2>Live2D 管理</h2>
          <p className="admin-copy">
            当前页用于维护陪伴与面试场景的 Live2D 模型。ZIP 导入或本地资源自动识别后，只会先加入后台待确认列表，默认未启用；只有管理员手动启用后，前台用户才可以切换到这些模型。
          </p>
        </div>
        <div className="admin-live2d-page__summary">
          <strong>{modelsQuery.data?.length || 0}</strong>
          <span>个模型</span>
        </div>
      </div>

      <div className="admin-live2d-page__toolbar">
        <form className="admin-live2d-import" onSubmit={handleImport}>
          <input
            ref={packageInputRef}
            className="admin-upload-input"
            type="file"
            accept=".zip"
            onChange={(event) => setImportFile(event.target.files?.[0] || null)}
          />
          <div className="admin-upload-meta">
            <strong>模型 ZIP</strong>
            <span>{importFile?.name || '尚未选择模型包'}</span>
          </div>
          <button className="admin-link" type="button" onClick={openPackagePicker} disabled={importMutation.isPending}>
            选择模型包
          </button>
          <button className="admin-link" type="submit" disabled={importMutation.isPending || !importFile}>
            {importMutation.isPending ? '导入中...' : '导入 ZIP 模型包'}
          </button>
        </form>

        <div className="admin-live2d-import">
          <input
            ref={backgroundInputRef}
            className="admin-upload-input"
            type="file"
            accept=".png,.jpg,.jpeg,.webp"
            onChange={(event) => setBackgroundImportFile(event.target.files?.[0] || null)}
          />
          <div className="admin-upload-meta">
            <strong>舞台背景图</strong>
            <span>{backgroundImportFile?.name || '尚未选择背景图'}</span>
          </div>
          <button className="admin-link" type="button" onClick={openBackgroundPicker} disabled={importBackgroundMutation.isPending}>
            选择背景图
          </button>
          <button className="admin-link" type="button" onClick={handleBackgroundImport} disabled={importBackgroundMutation.isPending}>
            {importBackgroundMutation.isPending ? '上传中...' : '上传舞台背景图'}
          </button>
        </div>

        <button className="admin-link" type="button" onClick={startCreatingModel}>
          新建模型
        </button>
      </div>

      <div className="admin-live2d-page__layout">
        <div className="admin-live2d-list">
          {(modelsQuery.data || []).length === 0 ? (
            <div className="admin-live2d-card admin-live2d-card--empty">
              <strong>当前还没有 Live2D 模型记录</strong>
              <p>可以先导入 ZIP 模型包，再创建一条模型配置记录。</p>
            </div>
          ) : (
            (modelsQuery.data || []).map((model) => (
              <button
                key={model.id}
                type="button"
                className={`admin-live2d-card ${selectedModelId === model.id ? 'admin-live2d-card--active' : ''}`}
                onClick={() => startEditingModel(model)}
              >
                <div className="admin-live2d-card__head">
                  <strong>{model.name}</strong>
                  <span>{model.is_active ? '启用中' : '已停用'}</span>
                </div>
                <div className="admin-live2d-card__meta">
                  <span>{live2DSceneLabel(model.scene)}</span>
                  <span>{resolveIndustryName(model.industry_id, industryMap)}</span>
                </div>
                <p>{shortenUrl(model.model_url)}</p>
              </button>
            ))
          )}
        </div>

        <form className="admin-live2d-editor" onSubmit={handleSubmit}>
          <div className="admin-live2d-editor__head">
            <div>
              <h3>{selectedModelId ? '编辑 Live2D 模型' : '新建 Live2D 模型'}</h3>
              <p>{message}</p>
            </div>
            <span className="admin-tag">{selectedModelId ? `ID #${selectedModelId}` : '新模型'}</span>
          </div>

          <label className="admin-field">
            <span>模型名称</span>
            <input
              value={form.name}
              onChange={(event) => updateLive2DField('name', event.target.value)}
              placeholder="例如 Ariu 陪伴版"
            />
          </label>

          <div className="admin-live2d-editor__grid">
            <label className="admin-field">
              <span>所属行业</span>
              <select
                value={form.industryId}
                onChange={(event) => updateLive2DField('industryId', event.target.value)}
              >
                <option value="0">通用模型</option>
                {(industriesQuery.data || []).map((industry) => (
                  <option key={industry.id} value={industry.id}>
                    {industry.name}
                  </option>
                ))}
              </select>
            </label>

            <label className="admin-field">
              <span>使用场景</span>
              <select
                value={form.scene}
                onChange={(event) => updateLive2DField('scene', event.target.value as Live2DScene)}
              >
                {LIVE2D_SCENE_OPTIONS.map((option) => (
                  <option key={option.value} value={option.value}>
                    {option.label}
                  </option>
                ))}
              </select>
            </label>
          </div>

          <label className="admin-field">
            <span>模型地址</span>
            <input
              value={form.modelUrl}
              onChange={(event) => updateLive2DField('modelUrl', event.target.value)}
              placeholder="/live2d-assets/xxx/xxx.model3.json"
            />
          </label>

          <label className="admin-field">
            <span>缩略图地址</span>
            <input
              value={form.thumbnailUrl}
              onChange={(event) => updateLive2DField('thumbnailUrl', event.target.value)}
              placeholder="/live2d-assets/xxx/preview.png"
            />
          </label>

          <label className="admin-field">
            <span>舞台背景图地址</span>
            <input
              value={form.backgroundImageUrl}
              onChange={(event) => updateLive2DField('backgroundImageUrl', event.target.value)}
              placeholder="/live2d-assets/backgrounds/stage-cover.webp"
            />
          </label>

          {form.thumbnailUrl ? (
            <div className="admin-live2d-editor__preview">
              <strong>缩略图预览</strong>
              <img src={form.thumbnailUrl} alt={form.name || 'Live2D 缩略图'} />
            </div>
          ) : null}

          {form.backgroundImageUrl ? (
            <div className="admin-live2d-editor__preview admin-live2d-editor__preview-stage">
              <strong>舞台背景预览</strong>
              <img src={form.backgroundImageUrl} alt={form.name ? `${form.name} 舞台背景` : 'Live2D 舞台背景'} />
            </div>
          ) : null}

          <div className="admin-live2d-editor__resource-check">
            <div className={`admin-live2d-editor__status ${hasValidModelUrl ? 'is-valid' : 'is-warning'}`}>
              <strong>模型资源</strong>
              <span>
                {hasValidModelUrl
                  ? '模型地址看起来是可加载的 .model3.json 文件。'
                  : '模型地址建议指向 .model3.json 文件，避免前台无法加载。'}
              </span>
            </div>
            <div className={`admin-live2d-editor__status ${hasBackgroundImageUrl ? 'is-valid' : 'is-warning'}`}>
              <strong>舞台背景</strong>
              <span>
                {hasBackgroundImageUrl
                  ? '前台舞台会优先加载该背景图，加载失败时自动回退默认渐变背景。'
                  : '当前未配置背景图，前台会继续使用默认舞台渐变背景。'}
              </span>
            </div>
            <div className={`admin-live2d-editor__status ${form.isActive ? 'is-valid' : 'is-warning'}`}>
              <strong>前台可见性</strong>
              <span>
                {form.isActive
                  ? '当前模型已启用，满足场景与行业命中条件时会出现在前台切换列表。'
                  : '当前模型未启用，只会保留在后台管理页，前台不会展示或允许切换。'}
              </span>
            </div>
            <div className={`admin-live2d-editor__status ${configPreview.valid ? 'is-valid' : 'is-error'}`}>
              <strong>配置 JSON</strong>
              <span>
                {configPreview.valid
                  ? '配置 JSON 可解析，预览区展示的是合并默认值后的最终配置。'
                  : configPreview.error}
              </span>
            </div>
          </div>

          <label className="admin-field">
            <span>渲染配置 JSON</span>
            <textarea
              className="admin-live2d-editor__config"
              value={form.configJson}
              onChange={(event) => handleConfigJsonChange(event.target.value)}
              placeholder='例如 {"scale":0.4,"offset_y":0.08}'
            />
          </label>

          <div className="admin-live2d-editor__effective-config">
            <div className="admin-live2d-editor__effective-head">
              <strong>最终生效配置预览</strong>
              <span>{configPreview.valid ? '已通过校验' : '当前展示默认配置回退值'}</span>
            </div>
            <pre>{configPreview.formattedConfig}</pre>
          </div>

          <label className="admin-live2d-editor__switch">
            <input
              type="checkbox"
              checked={form.isActive}
              onChange={(event) => updateLive2DField('isActive', event.target.checked)}
            />
            <span>{form.isActive ? '当前模型启用中' : '当前模型已停用'}</span>
          </label>

          <div className="admin-live2d-editor__actions">
            <button
              className="admin-link"
              type="button"
              onClick={startCreatingModel}
              disabled={saveMutation.isPending || deleteMutation.isPending || importMutation.isPending || importBackgroundMutation.isPending}
            >
              重置为新建
            </button>
            {selectedModelId ? (
              <button
                className="admin-link"
                type="button"
                onClick={handleDelete}
                disabled={saveMutation.isPending || deleteMutation.isPending || importMutation.isPending || importBackgroundMutation.isPending}
              >
                {deleteMutation.isPending ? '删除中...' : '删除模型'}
              </button>
            ) : null}
            <button
              className="admin-link"
              type="submit"
              disabled={!canSubmit || saveMutation.isPending || deleteMutation.isPending || importMutation.isPending || importBackgroundMutation.isPending}
            >
              {saveMutation.isPending ? '保存中...' : selectedModelId ? '保存修改' : '创建模型'}
            </button>
          </div>
        </form>
      </div>
    </section>
  )
}
