import type { FormEvent } from 'react'
import { useEffect, useMemo, useRef, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  SmileOutlined,
  PlusOutlined,
  DeleteOutlined,
  SaveOutlined,
  ReloadOutlined,
  UploadOutlined,
  CheckCircleOutlined,
  ExclamationCircleOutlined,
  WarningOutlined,
  InboxOutlined,
  FileZipOutlined,
  PictureOutlined,
} from '@ant-design/icons'
import { Button, Card, Input, Modal, Select, Switch, Tag, Tooltip } from 'antd'
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
  tts_config_id: number | null
  is_active: boolean
  created_at?: string
  updated_at?: string
}

interface TTSConfigOption {
  id: number
  name: string
  support_status: string
}

interface TTSConfigListResponse {
  configs: TTSConfigOption[]
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
  ttsConfigId: string
  isActive: boolean
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
 * 获取后台可绑定到 Live2D 的 TTS 配置列表。
 */
async function fetchTTSConfigs(token: string | null): Promise<TTSConfigOption[]> {
  const response = await requestJson<ApiEnvelope<TTSConfigListResponse>>('/admin/tts-configs', {
    method: 'GET',
    token,
  })

  if (!isSuccessCode(response.code)) {
    throw new Error(response.message || '获取 TTS 配置列表失败')
  }

  return (response.data?.configs || []).filter((item) => item.support_status === 'supported')
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
    ttsConfigId: '0',
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
    }
  }

  return {
    scale: 0.4,
    offset_x: 0,
    offset_y: 0.08,
    idle_motion: 'companion_idle',
    tap_motion: 'wave',
    background: 'transparent',
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
    ttsConfigId: String(model.tts_config_id ?? 0),
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
    tts_config_id: Number(form.ttsConfigId) > 0 ? Number(form.ttsConfigId) : null,
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
  const [messageText, setMessageText] = useState('读取 Live2D 模型中')
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
  const ttsConfigsQuery = useQuery({
    queryKey: ['admin', 'tts-configs-for-live2d', accessToken],
    queryFn: () => fetchTTSConfigs(accessToken),
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
      setMessageText((current) => (current === '读取 Live2D 模型中' ? '已同步 Live2D 模型列表。' : current))
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
      setMessageText(
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
      setMessageText(extractErrorMessage(error, '导入 Live2D 模型包失败，请稍后重试'))
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
      setMessageText(`背景图导入完成：${result.file_name}`)
      setBackgroundImportFile(null)
      if (backgroundInputRef.current) {
        backgroundInputRef.current.value = ''
      }
    },
    onError: (error) => {
      setMessageText(extractErrorMessage(error, '导入 Live2D 背景图失败，请稍后重试'))
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
      return created?.id
    },
    onSuccess: async (modelId) => {
      setSelectedModelId(modelId)
      setMessageText(selectedModelId ? 'Live2D 模型已更新。' : 'Live2D 模型已创建。')
      await queryClient.invalidateQueries({
        queryKey: ['admin', 'live2d-models'],
      })
    },
    onError: (error) => {
      setMessageText(extractErrorMessage(error, '保存 Live2D 模型失败，请稍后重试'))
    },
  })

  const deleteMutation = useMutation({
    mutationFn: async (id: number) => {
      await deleteLive2DModel(accessToken, id)
    },
    onSuccess: async () => {
      setSelectedModelId(null)
      setForm(buildInitialLive2DForm())
      setMessageText('Live2D 模型已删除。')
      await queryClient.invalidateQueries({
        queryKey: ['admin', 'live2d-models'],
      })
    },
    onError: (error) => {
      setMessageText(extractErrorMessage(error, '删除 Live2D 模型失败，请稍后重试'))
    },
  })

  /**
   * 切换到新建模式，并清空当前表单。
   */
  function startCreatingModel(): void {
    setSelectedModelId(null)
    setForm(buildInitialLive2DForm())
    setMessageText('已切换到新建 Live2D 模型。')
  }

  /**
   * 载入指定模型到右侧编辑区。
   */
  function startEditingModel(model: Live2DModel): void {
    setSelectedModelId(model.id)
    setForm(buildLive2DForm(model))
    setMessageText(`正在编辑模型：${model.name}`)
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
    setMessageText(selectedModelId ? '正在更新 Live2D 模型。' : '正在创建 Live2D 模型。')
    saveMutation.mutate()
  }

  /**
   * 提交模型包导入表单，并把自动识别结果回填到当前编辑表单。
   */
  function handleImport(event: FormEvent<HTMLFormElement>): void {
    event.preventDefault()
    setMessageText('正在导入 Live2D 模型包。')
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
      setMessageText('请先选择一张舞台背景图，再执行上传。')
      openBackgroundPicker()
      return
    }

    setMessageText('正在导入 Live2D 背景图。')
    importBackgroundMutation.mutate()
  }

  /**
   * 删除当前选中的 Live2D 模型记录。
   */
  function handleDelete(): void {
    if (!selectedModelId) {
      return
    }

    Modal.confirm({
      title: '确认删除',
      icon: <ExclamationCircleOutlined />,
      content: '确认删除当前 Live2D 模型吗？删除后不可恢复。',
      okText: '删除',
      okType: 'danger',
      cancelText: '取消',
      onOk: () => {
        setMessageText('正在删除 Live2D 模型。')
        deleteMutation.mutate(selectedModelId)
      },
    })
  }

  if (modelsQuery.isLoading || industriesQuery.isLoading || ttsConfigsQuery.isLoading) {
    return (
      <div style={{ padding: '24px 32px 32px', background: THEME.bg, minHeight: '100vh' }}>
        <div style={{ ...glassCard, padding: '24px 28px' }}>
          <h2 style={{ margin: 0, fontSize: 22, fontWeight: 700, color: THEME.textMain }}>Live2D 管理</h2>
          <p style={{ margin: '8px 0 0', color: THEME.textSecondary }}>正在加载模型资产与行业数据...</p>
        </div>
      </div>
    )
  }

  if (modelsQuery.isError || industriesQuery.isError || ttsConfigsQuery.isError) {
    return (
      <div style={{ padding: '24px 32px 32px', background: THEME.bg, minHeight: '100vh' }}>
        <div style={{ ...glassCard, padding: '24px 28px' }}>
          <h2 style={{ margin: 0, fontSize: 22, fontWeight: 700, color: THEME.textMain }}>Live2D 管理</h2>
          <p style={{ margin: '8px 0 0', color: THEME.danger }}>
            {extractErrorMessage(modelsQuery.error || industriesQuery.error || ttsConfigsQuery.error, '读取 Live2D 管理数据失败')}
          </p>
        </div>
      </div>
    )
  }

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
              background: 'linear-gradient(135deg, #ec4899, #db2777)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              boxShadow: '0 4px 14px rgba(236, 72, 153, 0.35)',
              flexShrink: 0,
            }}
          >
            <SmileOutlined style={{ fontSize: 22, color: '#fff' }} />
          </div>
          <div>
            <h1 style={{ margin: 0, fontSize: 22, fontWeight: 700, color: THEME.textMain, lineHeight: 1.3 }}>
              Live2D 管理
            </h1>
            <p style={{ margin: '4px 0 0', fontSize: 13, color: THEME.textSecondary }}>
              维护陪伴与面试场景的 Live2D 模型，支持 ZIP 导入和手动配置
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
            <span style={{ fontSize: 24, fontWeight: 700, color: THEME.primary }}>{modelsQuery.data?.length || 0}</span>
            <span style={{ fontSize: 12, color: THEME.textSecondary }}>个模型</span>
          </div>
        </div>
      </div>

      {/* Import Toolbar */}
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
        <form
          onSubmit={handleImport}
          style={{ display: 'flex', alignItems: 'center', gap: 10, flexWrap: 'wrap' }}
        >
          <input
            ref={packageInputRef}
            type="file"
            accept=".zip"
            onChange={(event) => setImportFile(event.target.files?.[0] || null)}
            style={{ display: 'none' }}
          />
          <div
            style={{
              padding: '6px 12px',
              borderRadius: 8,
              background: '#f8fafc',
              border: '1px solid ' + THEME.border,
              fontSize: 13,
              color: importFile ? THEME.textMain : THEME.textMuted,
              minWidth: 180,
              display: 'flex',
              alignItems: 'center',
              gap: 6,
            }}
          >
            <FileZipOutlined />
            {importFile?.name || '尚未选择模型包'}
          </div>
          <Button
            icon={<UploadOutlined />}
            onClick={openPackagePicker}
            disabled={importMutation.isPending}
            style={{ borderRadius: 10 }}
          >
            选择模型包
          </Button>
          <Button
            type="primary"
            icon={<UploadOutlined />}
            htmlType="submit"
            disabled={importMutation.isPending || !importFile}
            loading={importMutation.isPending}
            style={{
              borderRadius: 10,
              background: 'linear-gradient(135deg, #3b82f6, #2563eb)',
              border: 'none',
            }}
          >
            导入 ZIP
          </Button>
        </form>

        <div style={{ width: 1, height: 32, background: THEME.border }} />

        <div style={{ display: 'flex', alignItems: 'center', gap: 10, flexWrap: 'wrap' }}>
          <input
            ref={backgroundInputRef}
            type="file"
            accept=".png,.jpg,.jpeg,.webp"
            onChange={(event) => setBackgroundImportFile(event.target.files?.[0] || null)}
            style={{ display: 'none' }}
          />
          <div
            style={{
              padding: '6px 12px',
              borderRadius: 8,
              background: '#f8fafc',
              border: '1px solid ' + THEME.border,
              fontSize: 13,
              color: backgroundImportFile ? THEME.textMain : THEME.textMuted,
              minWidth: 180,
              display: 'flex',
              alignItems: 'center',
              gap: 6,
            }}
          >
            <PictureOutlined />
            {backgroundImportFile?.name || '尚未选择背景图'}
          </div>
          <Button
            icon={<UploadOutlined />}
            onClick={openBackgroundPicker}
            disabled={importBackgroundMutation.isPending}
            style={{ borderRadius: 10 }}
          >
            选择背景图
          </Button>
          <Button
            type="primary"
            icon={<UploadOutlined />}
            onClick={handleBackgroundImport}
            disabled={importBackgroundMutation.isPending || !backgroundImportFile}
            loading={importBackgroundMutation.isPending}
            style={{
              borderRadius: 10,
              background: 'linear-gradient(135deg, #3b82f6, #2563eb)',
              border: 'none',
            }}
          >
            上传背景图
          </Button>
        </div>

        <div style={{ flex: 1 }} />

        <Button
          type="primary"
          icon={<PlusOutlined />}
          onClick={startCreatingModel}
          style={{
            borderRadius: 10,
            background: 'linear-gradient(135deg, #3b82f6, #2563eb)',
            border: 'none',
          }}
        >
          新建模型
        </Button>
      </div>

      {/* Main Content */}
      <div style={{ display: 'flex', gap: 20, flexWrap: 'wrap' }}>
        {/* Left: Model List */}
        <div style={{ flex: '1 1 340px', maxWidth: 420, minWidth: 300 }}>
          <div
            style={{
              ...solidCard,
              overflow: 'hidden',
              display: 'flex',
              flexDirection: 'column',
              height: 'calc(100vh - 300px)',
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
              <span style={{ fontSize: 15, fontWeight: 600, color: THEME.textMain }}>模型列表</span>
              <span style={{ fontSize: 12, color: THEME.textMuted }}>共 {modelsQuery.data?.length || 0} 个</span>
            </div>

            <div style={{ flex: 1, overflowY: 'auto', padding: '8px' }}>
              {(modelsQuery.data || []).length === 0 ? (
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
                  <span style={{ fontSize: 14 }}>当前还没有 Live2D 模型记录</span>
                  <span style={{ fontSize: 12 }}>可以先导入 ZIP 模型包，再创建一条模型配置记录</span>
                </div>
              ) : (
                <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
                  {(modelsQuery.data || []).map((model) => {
                    const isActive = selectedModelId === model.id
                    return (
                      <div
                        key={model.id}
                        role="button"
                        tabIndex={0}
                        onClick={() => startEditingModel(model)}
                        onKeyDown={(e) => {
                          if (e.key === 'Enter' || e.key === ' ') {
                            startEditingModel(model)
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
                            alignItems: 'center',
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
                            }}
                          >
                            {model.name}
                          </span>
                          <Tag
                            color={model.is_active ? 'success' : 'default'}
                            style={{ fontSize: 11, padding: '0 6px', margin: 0, flexShrink: 0 }}
                          >
                            {model.is_active ? '启用中' : '已停用'}
                          </Tag>
                        </div>
                        <div
                          style={{
                            display: 'flex',
                            alignItems: 'center',
                            gap: 8,
                            fontSize: 12,
                            color: THEME.textSecondary,
                            flexWrap: 'wrap',
                          }}
                        >
                          <Tag
                            style={{
                              fontSize: 11,
                              margin: 0,
                              color: model.scene === 'interview' ? '#8b5cf6' : '#3b82f6',
                              background:
                                model.scene === 'interview' ? '#f3e8ff' : '#dbeafe',
                              border: 'none',
                            }}
                          >
                            {live2DSceneLabel(model.scene)}
                          </Tag>
                          <span>{resolveIndustryName(model.industry_id, industryMap)}</span>
                        </div>
                        <Tooltip title={model.model_url}>
                          <p
                            style={{
                              margin: '6px 0 0',
                              fontSize: 11,
                              color: THEME.textMuted,
                              fontFamily: 'monospace',
                              overflow: 'hidden',
                              textOverflow: 'ellipsis',
                              whiteSpace: 'nowrap',
                            }}
                          >
                            {shortenUrl(model.model_url)}
                          </p>
                        </Tooltip>
                      </div>
                    )
                  })}
                </div>
              )}
            </div>
          </div>
        </div>

        {/* Right: Editor */}
        <div style={{ flex: '2 1 520px', minWidth: 360 }}>
          <div
            style={{
              ...solidCard,
              overflow: 'hidden',
              display: 'flex',
              flexDirection: 'column',
              height: 'calc(100vh - 300px)',
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
              <div>
                <span style={{ fontSize: 15, fontWeight: 600, color: THEME.textMain }}>
                  {selectedModelId ? '编辑 Live2D 模型' : '新建 Live2D 模型'}
                </span>
                <div style={{ fontSize: 12, color: THEME.textMuted, marginTop: 2 }}>{messageText}</div>
              </div>
              <Tag
                style={{
                  fontSize: 12,
                  padding: '2px 10px',
                  color: selectedModelId ? THEME.primary : THEME.success,
                  background: selectedModelId ? THEME.primaryLight : '#dcfce7',
                  border: 'none',
                }}
              >
                {selectedModelId ? `ID #${selectedModelId}` : '新模型'}
              </Tag>
            </div>

            <form
              onSubmit={handleSubmit}
              style={{
                flex: 1,
                overflowY: 'auto',
                padding: '20px',
                display: 'flex',
                flexDirection: 'column',
                gap: 16,
              }}
            >
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
                  模型名称
                </label>
                <Input
                  value={form.name}
                  onChange={(e) => updateLive2DField('name', e.target.value)}
                  placeholder="例如 Ariu 陪伴版"
                  style={{ borderRadius: 10 }}
                />
              </div>

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
                    所属行业
                  </label>
                  <Select
                    value={form.industryId}
                    onChange={(v) => updateLive2DField('industryId', v)}
                    style={{ width: '100%' }}
                    options={[
                      { value: '0', label: '通用模型' },
                      ...(industriesQuery.data || []).map((i) => ({
                        value: String(i.id),
                        label: i.name,
                      })),
                    ]}
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
                    使用场景
                  </label>
                  <Select
                    value={form.scene}
                    onChange={(v) => updateLive2DField('scene', v as Live2DScene)}
                    style={{ width: '100%' }}
                    options={LIVE2D_SCENE_OPTIONS}
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
                  绑定 TTS 配置
                </label>
                <Select
                  value={form.ttsConfigId}
                  onChange={(v) => updateLive2DField('ttsConfigId', v)}
                  style={{ width: '100%' }}
                  options={[
                    { value: '0', label: '不绑定，回退到场景默认 / config.yaml' },
                    ...(ttsConfigsQuery.data || []).map((c) => ({
                      value: String(c.id),
                      label: c.name,
                    })),
                  ]}
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
                  模型地址
                </label>
                <Input
                  value={form.modelUrl}
                  onChange={(e) => updateLive2DField('modelUrl', e.target.value)}
                  placeholder="/live2d-assets/xxx/xxx.model3.json"
                  style={{ borderRadius: 10, fontFamily: 'monospace', fontSize: 13 }}
                />
              </div>

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
                    缩略图地址
                  </label>
                  <Input
                    value={form.thumbnailUrl}
                    onChange={(e) => updateLive2DField('thumbnailUrl', e.target.value)}
                    placeholder="/live2d-assets/xxx/preview.png"
                    style={{ borderRadius: 10, fontFamily: 'monospace', fontSize: 13 }}
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
                    舞台背景图地址
                  </label>
                  <Input
                    value={form.backgroundImageUrl}
                    onChange={(e) => updateLive2DField('backgroundImageUrl', e.target.value)}
                    placeholder="/live2d-assets/backgrounds/stage-cover.webp"
                    style={{ borderRadius: 10, fontFamily: 'monospace', fontSize: 13 }}
                  />
                </div>
              </div>

              {/* Image Previews */}
              {(form.thumbnailUrl || form.backgroundImageUrl) && (
                <div style={{ display: 'grid', gridTemplateColumns: form.thumbnailUrl && form.backgroundImageUrl ? '1fr 1fr' : '1fr', gap: 16 }}>
                  {form.thumbnailUrl && (
                    <div>
                      <div
                        style={{
                          fontSize: 12,
                          fontWeight: 600,
                          color: THEME.textSecondary,
                          marginBottom: 8,
                        }}
                      >
                        缩略图预览
                      </div>
                      <div
                        style={{
                          borderRadius: 12,
                          overflow: 'hidden',
                          border: '1px solid ' + THEME.border,
                          height: 160,
                          background: '#f8fafc',
                          display: 'flex',
                          alignItems: 'center',
                          justifyContent: 'center',
                        }}
                      >
                        <img
                          src={form.thumbnailUrl}
                          alt={form.name || '缩略图'}
                          style={{
                            maxWidth: '100%',
                            maxHeight: '100%',
                            objectFit: 'contain',
                          }}
                          onError={(e) => {
                            (e.target as HTMLImageElement).style.display = 'none'
                          }}
                        />
                      </div>
                    </div>
                  )}
                  {form.backgroundImageUrl && (
                    <div>
                      <div
                        style={{
                          fontSize: 12,
                          fontWeight: 600,
                          color: THEME.textSecondary,
                          marginBottom: 8,
                        }}
                      >
                        舞台背景预览
                      </div>
                      <div
                        style={{
                          borderRadius: 12,
                          overflow: 'hidden',
                          border: '1px solid ' + THEME.border,
                          height: 160,
                          background: '#f8fafc',
                          display: 'flex',
                          alignItems: 'center',
                          justifyContent: 'center',
                        }}
                      >
                        <img
                          src={form.backgroundImageUrl}
                          alt={form.name ? `${form.name} 舞台背景` : '舞台背景'}
                          style={{
                            maxWidth: '100%',
                            maxHeight: '100%',
                            objectFit: 'cover',
                          }}
                          onError={(e) => {
                            (e.target as HTMLImageElement).style.display = 'none'
                          }}
                        />
                      </div>
                    </div>
                  )}
                </div>
              )}

              {/* Status Checks */}
              <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(200px, 1fr))', gap: 10 }}>
                {[
                  {
                    ok: hasValidModelUrl,
                    title: '模型资源',
                    okText: '模型地址看起来是可加载的 .model3.json 文件。',
                    warnText: '模型地址建议指向 .model3.json 文件，避免前台无法加载。',
                  },
                  {
                    ok: hasBackgroundImageUrl,
                    title: '舞台背景',
                    okText: '前台舞台会优先加载该背景图，加载失败时自动回退默认渐变背景。',
                    warnText: '当前未配置背景图，前台会继续使用默认舞台渐变背景。',
                  },
                  {
                    ok: form.isActive,
                    title: '前台可见性',
                    okText: '当前模型已启用，满足场景与行业命中条件时会出现在前台切换列表。',
                    warnText: '当前模型未启用，只会保留在后台管理页，前台不会展示或允许切换。',
                  },
                  {
                    ok: configPreview.valid,
                    title: '配置 JSON',
                    okText: '配置 JSON 可解析，预览区展示的是合并默认值后的最终配置。',
                    warnText: configPreview.error,
                    isError: !configPreview.valid,
                  },
                ].map((check) => (
                  <div
                    key={check.title}
                    style={{
                      padding: '12px 14px',
                      borderRadius: 10,
                      background: check.ok ? '#f0fdf4' : check.isError ? '#fef2f2' : '#fffbeb',
                      border: `1px solid ${check.ok ? '#bbf7d0' : check.isError ? '#fecaca' : '#fde68a'}`,
                      display: 'flex',
                      alignItems: 'flex-start',
                      gap: 8,
                    }}
                  >
                    {check.ok ? (
                      <CheckCircleOutlined
                        style={{
                          fontSize: 16,
                          color: THEME.success,
                          marginTop: 2,
                          flexShrink: 0,
                        }}
                      />
                    ) : check.isError ? (
                      <ExclamationCircleOutlined
                        style={{
                          fontSize: 16,
                          color: THEME.danger,
                          marginTop: 2,
                          flexShrink: 0,
                        }}
                      />
                    ) : (
                      <WarningOutlined
                        style={{
                          fontSize: 16,
                          color: THEME.warning,
                          marginTop: 2,
                          flexShrink: 0,
                        }}
                      />
                    )}
                    <div>
                      <div style={{ fontSize: 12, fontWeight: 600, color: THEME.textMain }}>{check.title}</div>
                      <div style={{ fontSize: 11, color: THEME.textSecondary, lineHeight: 1.4 }}>
                        {check.ok ? check.okText : check.warnText}
                      </div>
                    </div>
                  </div>
                ))}
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
                  渲染配置 JSON
                </label>
                <Input.TextArea
                  value={form.configJson}
                  onChange={(e) => handleConfigJsonChange(e.target.value)}
                  placeholder='例如 {"scale":0.4,"offset_y":0.08}'
                  rows={6}
                  style={{
                    borderRadius: 10,
                    fontFamily: 'monospace',
                    fontSize: 13,
                    borderColor: configPreview.valid ? undefined : THEME.danger,
                  }}
                />
                {!configPreview.valid && (
                  <div style={{ marginTop: 4, fontSize: 12, color: THEME.danger }}>{configPreview.error}</div>
                )}
              </div>

              {/* Effective Config Preview */}
              <div>
                <div
                  style={{
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'space-between',
                    marginBottom: 6,
                  }}
                >
                  <span style={{ fontSize: 12, fontWeight: 600, color: THEME.textSecondary }}>最终生效配置预览</span>
                  <Tag
                    color={configPreview.valid ? 'success' : 'error'}
                    style={{ fontSize: 11, margin: 0 }}
                  >
                    {configPreview.valid ? '已通过校验' : '默认配置回退值'}
                  </Tag>
                </div>
                <pre
                  style={{
                    margin: 0,
                    padding: 14,
                    borderRadius: 10,
                    background: '#0f172a',
                    color: '#e2e8f0',
                    fontSize: 12,
                    lineHeight: 1.6,
                    overflowX: 'auto',
                    maxHeight: 200,
                    overflowY: 'auto',
                  }}
                >
                  {configPreview.formattedConfig}
                </pre>
              </div>

              <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
                <Switch
                  checked={form.isActive}
                  onChange={(checked) => updateLive2DField('isActive', checked)}
                />
                <span style={{ fontSize: 13, color: THEME.textSecondary }}>
                  {form.isActive ? '当前模型启用中' : '当前模型已停用'}
                </span>
              </div>

              <div style={{ display: 'flex', gap: 10, flexWrap: 'wrap', paddingTop: 4 }}>
                <Button
                  icon={<ReloadOutlined />}
                  onClick={startCreatingModel}
                  disabled={saveMutation.isPending || deleteMutation.isPending || importMutation.isPending || importBackgroundMutation.isPending}
                  style={{ borderRadius: 10 }}
                >
                  重置为新建
                </Button>
                {selectedModelId && (
                  <Button
                    danger
                    icon={<DeleteOutlined />}
                    onClick={handleDelete}
                    loading={deleteMutation.isPending}
                    disabled={saveMutation.isPending || deleteMutation.isPending || importMutation.isPending || importBackgroundMutation.isPending}
                    style={{ borderRadius: 10 }}
                  >
                    删除模型
                  </Button>
                )}
                <Button
                  type="primary"
                  htmlType="submit"
                  icon={<SaveOutlined />}
                  loading={saveMutation.isPending}
                  disabled={!canSubmit || saveMutation.isPending || deleteMutation.isPending || importMutation.isPending || importBackgroundMutation.isPending}
                  style={{
                    borderRadius: 10,
                    background: 'linear-gradient(135deg, #3b82f6, #2563eb)',
                    border: 'none',
                    boxShadow: '0 4px 14px rgba(37, 99, 235, 0.35)',
                  }}
                >
                  {saveMutation.isPending ? '保存中...' : selectedModelId ? '保存修改' : '创建模型'}
                </Button>
              </div>
            </form>
          </div>
        </div>
      </div>
    </div>
  )
}
