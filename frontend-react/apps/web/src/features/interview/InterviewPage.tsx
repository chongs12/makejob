import type { DragEvent, FormEvent } from 'react'
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link, useNavigate } from '@tanstack/react-router'
import { Button, Input, Select, Tag, Empty, Spin } from 'antd'
import {
  PlayCircleOutlined,
  FileTextOutlined,
  HistoryOutlined,
  TrophyOutlined,
  ClockCircleOutlined,
  RightOutlined,
  UploadOutlined,
} from '@ant-design/icons'
import { extractErrorMessage } from '@makejob/api-client'
import { useAuthStore } from '../../state/auth'
import { requestLoginPrompt } from '../../shared/loginPrompt'
import { readSelectedLive2DModelKey } from '../../shared/live2dModelCatalog'
import {
  DEFAULT_FRONTEND_INDUSTRY_CODE as INTERVIEW_DEFAULT_INDUSTRY_CODE,
  formatFrontendIndustryLabel,
  persistSelectedFrontendIndustryCode,
  readSelectedFrontendIndustryCode,
  resolvePreferredFrontendIndustry,
} from '../../shared/industryContext'
import { useFrontendIndustriesQuery } from '../../shared/frontendQueries'
import { buildInterviewHistoryQueryKey, invalidateInterviewHistoryQueries } from '../../shared/queryKeys'
import { createInterviewRequest, fetchInterviewHistory } from './interviewApi'
import {
  buildDefaultInterviewTopics,
  buildInitialInterviewForm,
  formatInterviewDateTime,
  interviewDifficultyLabel,
  interviewStatusLabel,
  parseInterviewTopics,
} from './interviewHelpers'
import type { InterviewConfigForm, InterviewCreatePayload } from './interviewTypes'

const THEME = {
  bg: '#f8f9fa',
  cardBg: '#ffffff',
  primary: '#f97316',
  primaryLight: '#fff7ed',
  primaryDark: '#ea580c',
  textMain: '#1f2937',
  textSecondary: '#6b7280',
  textMuted: '#9ca3af',
  border: '#f3f4f6',
  borderHover: '#e5e7eb',
  shadow: '0 1px 2px rgba(0,0,0,0.05)',
  shadowCard: '0 4px 6px -1px rgba(0,0,0,0.07), 0 2px 4px -2px rgba(0,0,0,0.05)',
  shadowHover: '0 10px 15px -3px rgba(0,0,0,0.08), 0 4px 6px -4px rgba(0,0,0,0.05)',
  radius: 12,
  radiusSm: 8,
  success: '#22c55e',
  warning: '#f59e0b',
  danger: '#ef4444',
}

const PDF_MAX_SIZE_MB = 10
const PDF_MAX_BYTES = PDF_MAX_SIZE_MB * 1024 * 1024

/**
 * 使用 pdfjs-dist 从 PDF 文件中提取纯文本内容。
 */
async function extractTextFromPDF(file: File): Promise<string> {
  const pdfjsLib = await import('pdfjs-dist')
  pdfjsLib.GlobalWorkerOptions.workerSrc = new URL(
    'pdfjs-dist/build/pdf.worker.mjs',
    import.meta.url,
  ).toString()

  const arrayBuffer = await file.arrayBuffer()
  const pdf = await pdfjsLib.getDocument({ data: arrayBuffer }).promise

  const chunks: string[] = []
  for (let i = 1; i <= pdf.numPages; i++) {
    const page = await pdf.getPage(i)
    const content = await page.getTextContent()
    const pageText = content.items.map((item) => ('str' in item ? item.str : '')).join(' ')
    if (pageText.trim()) {
      chunks.push(pageText)
    }
  }

  return chunks.join('\n').trim()
}

/**
 * AI 面试入口页，采用简洁双栏布局，聚焦创建面试和历史记录。
 */
export function InterviewHubPage() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const accessToken = useAuthStore((state) => state.accessToken)
  const user = useAuthStore((state) => state.user)
  const [form, setForm] = useState<InterviewConfigForm>(() => buildInitialInterviewForm())
  const [selectedIndustryCode, setSelectedIndustryCode] = useState(() => readSelectedFrontendIndustryCode() || INTERVIEW_DEFAULT_INDUSTRY_CODE)
  const [channelChoice, setChannelChoice] = useState<'voice' | 'text'>('voice')
  const [message, setMessage] = useState('')
  const [pdfFileName, setPdfFileName] = useState('')
  const [pdfLoading, setPdfLoading] = useState(false)
  const [pdfError, setPdfError] = useState('')
  const [pdfDragOver, setPdfDragOver] = useState(false)
  const pdfInputRef = useRef<HTMLInputElement>(null)
  const industriesQuery = useFrontendIndustriesQuery()

  const historyQuery = useQuery({
    queryKey: buildInterviewHistoryQueryKey(accessToken),
    queryFn: () => fetchInterviewHistory(accessToken as string),
    enabled: Boolean(accessToken),
    retry: false,
  })

  const selectedIndustry = useMemo(
    () => resolvePreferredFrontendIndustry(industriesQuery.data || [], selectedIndustryCode, INTERVIEW_DEFAULT_INDUSTRY_CODE),
    [industriesQuery.data, selectedIndustryCode],
  )
  const effectiveIndustryCode = selectedIndustry?.code || selectedIndustryCode.trim() || INTERVIEW_DEFAULT_INDUSTRY_CODE
  const effectiveIndustryLabel = formatFrontendIndustryLabel(selectedIndustry, effectiveIndustryCode)

  // 实时语音面试为会员专属：免费用户固定文字模式，付费用户可在语音/文字间选择（默认语音）。
  // 会员等级经 /user/profile 由 membership 服务回源，升级后即时反映。
  const isPaidMember = Boolean(user?.membershipLevel) && user.membershipLevel !== 'free'
  const deliveryChannel: 'voice' | 'text' = isPaidMember ? channelChoice : 'text'

  const createMutation = useMutation({
    mutationFn: (payload: InterviewCreatePayload) => createInterviewRequest(accessToken as string, payload),
    onSuccess: async (data) => {
      setMessage('面试会话已创建，正在进入...')
      await invalidateInterviewHistoryQueries(queryClient)
      navigate({
        to: '/interview/$interviewId',
        params: { interviewId: String(data.interview_id) },
      })
    },
    onError: (error) => {
      setMessage(extractErrorMessage(error, '创建面试失败，请稍后重试'))
    },
  })

  const ongoingInterview = useMemo(
    () => historyQuery.data?.list.find((item) => item.status === 'ongoing' || item.status === 'preparing') || null,
    [historyQuery.data],
  )

  /**
   * 行业列表加载后归一化当前选中的行业编码。
   */
  useEffect(() => {
    const normalizedIndustryCode = effectiveIndustryCode.trim()
    if (!normalizedIndustryCode) return
    persistSelectedFrontendIndustryCode(normalizedIndustryCode)
    if (normalizedIndustryCode !== selectedIndustryCode) {
      setSelectedIndustryCode(normalizedIndustryCode)
    }
  }, [effectiveIndustryCode, selectedIndustryCode])

  /**
   * 切换目标行业时重置推荐主题。
   */
  function handleIndustryChange(nextIndustryCode: string): void {
    setSelectedIndustryCode(nextIndustryCode)
    setForm((current) => ({
      ...current,
      topicsText: buildDefaultInterviewTopics(nextIndustryCode),
    }))
  }

  /**
   * 处理 PDF 文件选择，提取文本并填入简历字段。
   */
  const handlePdfFile = useCallback(async (file: File) => {
    setPdfError('')
    setPdfFileName('')

    if (!file.name.toLowerCase().endsWith('.pdf')) {
      setPdfError('请上传 PDF 格式的文件。')
      return
    }
    if (file.size > PDF_MAX_BYTES) {
      setPdfError(`文件大小不能超过 ${PDF_MAX_SIZE_MB}MB。`)
      return
    }

    setPdfLoading(true)
    setPdfFileName(file.name)
    try {
      const text = await extractTextFromPDF(file)
      if (!text || text.length < 20) {
        setPdfError('未能从 PDF 中提取到有效文本，请检查文件或手动粘贴。')
        setPdfFileName('')
        return
      }
      setForm((current) => ({ ...current, resumeText: text }))
      setMessage(`已从 "${file.name}" 提取 ${text.length} 个字符`)
    } catch {
      setPdfError('PDF 解析失败，请尝试手动粘贴。')
      setPdfFileName('')
    } finally {
      setPdfLoading(false)
    }
  }, [])

  const handlePdfDrop = useCallback((event: DragEvent<HTMLDivElement>) => {
    event.preventDefault()
    setPdfDragOver(false)
    const file = event.dataTransfer.files[0]
    if (file) handlePdfFile(file)
  }, [handlePdfFile])

  const handlePdfInputChange = useCallback((event: React.ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0]
    if (file) handlePdfFile(file)
  }, [handlePdfFile])

  /**
   * 提交面试配置表单，创建新的 AI 面试会话。
   */
  async function handleCreateInterview(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!accessToken) {
      requestLoginPrompt('/interview', 'missing')
      return
    }

    const isResumeMode = form.interviewMode === 'resume_driven'
    if (isResumeMode) {
      if (form.resumeText.trim().length < 50) {
        setMessage('简历文本至少需要 50 个字符')
        return
      }
    } else {
      const topics = parseInterviewTopics(form.topicsText)
      if (topics.length === 0) {
        setMessage('至少填写一个主题')
        return
      }
    }

    setMessage('正在创建面试...')
    try {
      await createMutation.mutateAsync({
        industry_code: effectiveIndustryCode,
        interview_type: form.interviewType,
        live2d_model_key: deliveryChannel === 'voice'
          ? readSelectedLive2DModelKey('interview', effectiveIndustryCode)
          : '',
        ...(isResumeMode
          ? {
              interview_mode: 'resume_driven',
              resume_text: form.resumeText.trim(),
              job_description: form.jobDescription.trim() || undefined,
            }
          : {
              difficulty: form.difficulty,
              topics: parseInterviewTopics(form.topicsText),
              question_count: Number(form.questionCount) || 5,
            }),
      })
    } catch {
      if (!useAuthStore.getState().accessToken) {
        requestLoginPrompt('/interview', 'expired')
      }
    }
  }

  const cardStyle = {
    background: THEME.cardBg,
    borderRadius: THEME.radius,
    border: `1px solid ${THEME.border}`,
    boxShadow: THEME.shadow,
    padding: '24px',
  }

  const recentHistory = historyQuery.data?.list.slice(0, 5) || []

  return (
    <div style={{ minHeight: '100vh', background: THEME.bg }}>
      {/* 顶部标题栏 */}
      <div style={{
        background: THEME.cardBg,
        borderBottom: `1px solid ${THEME.border}`,
        boxShadow: THEME.shadow,
      }}>
        <div style={{
          maxWidth: 1200,
          margin: '0 auto',
          padding: '20px 24px',
        }}>
          <h1 style={{ fontSize: 22, fontWeight: 700, color: THEME.textMain, margin: '0 0 8px' }}>
            {user?.username ? `${user.username}，开始一场模拟面试` : '开始一场模拟面试'}
          </h1>
          <p style={{ fontSize: 14, color: THEME.textSecondary, margin: '0 0 16px' }}>
            选择行业方向，配置面试参数，开始 AI 模拟面试
          </p>

          {/* 指标卡片 */}
          <div style={{ display: 'flex', gap: 16 }}>
            <div style={{
              padding: '12px 16px',
              borderRadius: THEME.radiusSm,
              background: THEME.primaryLight,
              display: 'flex',
              alignItems: 'center',
              gap: 10,
            }}>
              <HistoryOutlined style={{ color: THEME.primary, fontSize: 18 }} />
              <div>
                <div style={{ fontSize: 18, fontWeight: 700, color: THEME.textMain }}>
                  {historyQuery.data?.total ?? '--'}
                </div>
                <div style={{ fontSize: 12, color: THEME.textSecondary }}>历史面试</div>
              </div>
            </div>

            <div style={{
              padding: '12px 16px',
              borderRadius: THEME.radiusSm,
              background: ongoingInterview ? '#f0fdf4' : '#fafaf9',
              display: 'flex',
              alignItems: 'center',
              gap: 10,
            }}>
              <PlayCircleOutlined style={{ color: ongoingInterview ? THEME.success : THEME.textMuted, fontSize: 18 }} />
              <div>
                <div style={{ fontSize: 18, fontWeight: 700, color: THEME.textMain }}>
                  {ongoingInterview ? '1' : '0'}
                </div>
                <div style={{ fontSize: 12, color: THEME.textSecondary }}>进行中</div>
              </div>
            </div>

            <div style={{
              padding: '12px 16px',
              borderRadius: THEME.radiusSm,
              background: '#fafaf9',
              display: 'flex',
              alignItems: 'center',
              gap: 10,
            }}>
              <TrophyOutlined style={{ color: THEME.textMuted, fontSize: 18 }} />
              <div>
                <div style={{ fontSize: 14, fontWeight: 600, color: THEME.textMain }}>
                  {effectiveIndustryLabel}
                </div>
                <div style={{ fontSize: 12, color: THEME.textSecondary }}>当前方向</div>
              </div>
            </div>
          </div>
        </div>
      </div>

      {/* 主内容区 */}
      <div style={{
        maxWidth: 1200,
        margin: '0 auto',
        padding: '24px',
        display: 'grid',
        gridTemplateColumns: '1fr 360px',
        gap: 24,
      }}>
        {/* 左侧：创建面试 */}
        <div style={cardStyle}>
          <div style={{ marginBottom: 24 }}>
            <h2 style={{ fontSize: 18, fontWeight: 700, color: THEME.textMain, margin: '0 0 4px' }}>
              创建面试
            </h2>
            <p style={{ fontSize: 13, color: THEME.textSecondary, margin: 0 }}>
              {form.interviewMode === 'resume_driven' ? '上传简历，AI 围绕你的经历出题' : '配置难度、题量和主题'}
            </p>
          </div>

          {/* 模式切换 */}
          <div style={{
            display: 'flex',
            gap: 8,
            marginBottom: 24,
            padding: 4,
            background: '#fafaf9',
            borderRadius: THEME.radiusSm,
          }}>
            <button
              type="button"
              onClick={() => setForm((current) => ({ ...current, interviewMode: 'general', interviewType: 'knowledge' }))}
              style={{
                flex: 1,
                padding: '10px 16px',
                borderRadius: 8,
                border: 'none',
                background: form.interviewMode === 'general' ? THEME.cardBg : 'transparent',
                boxShadow: form.interviewMode === 'general' ? THEME.shadow : 'none',
                fontSize: 14,
                fontWeight: form.interviewMode === 'general' ? 600 : 500,
                color: form.interviewMode === 'general' ? THEME.textMain : THEME.textSecondary,
                cursor: 'pointer',
                transition: 'all 0.2s ease',
              }}
            >
              知识练习
            </button>
            <button
              type="button"
              onClick={() => setForm((current) => ({ ...current, interviewMode: 'resume_driven', interviewType: 'job' }))}
              style={{
                flex: 1,
                padding: '10px 16px',
                borderRadius: 8,
                border: 'none',
                background: form.interviewMode === 'resume_driven' ? THEME.cardBg : 'transparent',
                boxShadow: form.interviewMode === 'resume_driven' ? THEME.shadow : 'none',
                fontSize: 14,
                fontWeight: form.interviewMode === 'resume_driven' ? 600 : 500,
                color: form.interviewMode === 'resume_driven' ? THEME.textMain : THEME.textSecondary,
                cursor: 'pointer',
                transition: 'all 0.2s ease',
              }}
            >
              实战面试
            </button>
          </div>

          {/* 交付通道：语音面试（会员专属）/ 文字面试 */}
          <div style={{ marginBottom: 8 }}>
            <label style={{ display: 'block', fontSize: 13, fontWeight: 600, color: THEME.textMain, marginBottom: 8 }}>
              面试方式
            </label>
            <div style={{
              display: 'flex',
              gap: 8,
              padding: 4,
              background: '#fafaf9',
              borderRadius: THEME.radiusSm,
            }}>
              <button
                type="button"
                disabled={!isPaidMember}
                onClick={() => isPaidMember && setChannelChoice('voice')}
                style={{
                  flex: 1,
                  padding: '10px 16px',
                  borderRadius: 8,
                  border: 'none',
                  background: deliveryChannel === 'voice' ? THEME.cardBg : 'transparent',
                  boxShadow: deliveryChannel === 'voice' ? THEME.shadow : 'none',
                  fontSize: 14,
                  fontWeight: deliveryChannel === 'voice' ? 600 : 500,
                  color: deliveryChannel === 'voice' ? THEME.textMain : THEME.textSecondary,
                  cursor: isPaidMember ? 'pointer' : 'not-allowed',
                  opacity: isPaidMember ? 1 : 0.55,
                  transition: 'all 0.2s ease',
                }}
              >
                语音面试{isPaidMember ? '' : '（会员）'}
              </button>
              <button
                type="button"
                onClick={() => setChannelChoice('text')}
                style={{
                  flex: 1,
                  padding: '10px 16px',
                  borderRadius: 8,
                  border: 'none',
                  background: deliveryChannel === 'text' ? THEME.cardBg : 'transparent',
                  boxShadow: deliveryChannel === 'text' ? THEME.shadow : 'none',
                  fontSize: 14,
                  fontWeight: deliveryChannel === 'text' ? 600 : 500,
                  color: deliveryChannel === 'text' ? THEME.textMain : THEME.textSecondary,
                  cursor: 'pointer',
                  transition: 'all 0.2s ease',
                }}
              >
                文字面试
              </button>
            </div>
          </div>
          {!isPaidMember && (
            <p style={{ fontSize: 12, color: THEME.primary, margin: '0 0 20px' }}>
              <Link to="/membership" style={{ color: THEME.primary, textDecoration: 'none' }}>
                实时语音面试是会员专属功能，升级会员解锁 →
              </Link>
            </p>
          )}

          <form onSubmit={handleCreateInterview}>
            {/* 行业选择 */}
            <div style={{ marginBottom: 20 }}>
              <label style={{ display: 'block', fontSize: 13, fontWeight: 600, color: THEME.textMain, marginBottom: 8 }}>
                目标方向
              </label>
              <Select
                value={effectiveIndustryCode}
                onChange={handleIndustryChange}
                style={{ width: '100%' }}
                loading={industriesQuery.isLoading}
                options={(industriesQuery.data || []).map((i) => ({ value: i.code, label: i.name }))}
              />
            </div>

            {/* 知识练习模式 */}
            {form.interviewMode !== 'resume_driven' && (
              <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 20, marginBottom: 20 }}>
                <div>
                  <label style={{ display: 'block', fontSize: 13, fontWeight: 600, color: THEME.textMain, marginBottom: 8 }}>
                    难度
                  </label>
                  <Select
                    value={form.difficulty}
                    onChange={(value) => setForm((current) => ({ ...current, difficulty: value }))}
                    style={{ width: '100%' }}
                    options={[
                      { value: 'easy', label: interviewDifficultyLabel('easy') },
                      { value: 'medium', label: interviewDifficultyLabel('medium') },
                      { value: 'hard', label: interviewDifficultyLabel('hard') },
                      { value: 'mixed', label: interviewDifficultyLabel('mixed') },
                    ]}
                  />
                </div>
                <div>
                  <label style={{ display: 'block', fontSize: 13, fontWeight: 600, color: THEME.textMain, marginBottom: 8 }}>
                    题量
                  </label>
                  <Input
                    type="number"
                    min={3}
                    max={20}
                    value={form.questionCount}
                    onChange={(e) => setForm((current) => ({ ...current, questionCount: e.target.value }))}
                    style={{ borderRadius: 8 }}
                  />
                </div>
              </div>
            )}

            {/* 实战面试模式 */}
            {form.interviewMode === 'resume_driven' && (
              <>
                <div
                  style={{
                    marginBottom: 20,
                    padding: '24px',
                    borderRadius: THEME.radiusSm,
                    border: `2px dashed ${pdfDragOver ? THEME.primary : THEME.border}`,
                    background: pdfDragOver ? THEME.primaryLight : '#fafaf9',
                    textAlign: 'center',
                    cursor: 'pointer',
                    transition: 'all 0.2s ease',
                  }}
                  onDragOver={(e) => { e.preventDefault(); setPdfDragOver(true) }}
                  onDragLeave={() => setPdfDragOver(false)}
                  onDrop={handlePdfDrop}
                  onClick={() => pdfInputRef.current?.click()}
                >
                  <input
                    ref={pdfInputRef}
                    type="file"
                    accept=".pdf"
                    style={{ display: 'none' }}
                    onChange={handlePdfInputChange}
                  />
                  {pdfLoading ? (
                    <Spin tip="正在解析 PDF..." />
                  ) : pdfFileName ? (
                    <div>
                      <FileTextOutlined style={{ fontSize: 24, color: THEME.success, marginBottom: 8 }} />
                      <div style={{ fontSize: 14, color: THEME.textMain }}>已选择：{pdfFileName}</div>
                    </div>
                  ) : (
                    <div>
                      <UploadOutlined style={{ fontSize: 24, color: THEME.textMuted, marginBottom: 8 }} />
                      <div style={{ fontSize: 14, color: THEME.textSecondary }}>点击或拖拽上传 PDF 简历</div>
                      <div style={{ fontSize: 12, color: THEME.textMuted, marginTop: 4 }}>最大 {PDF_MAX_SIZE_MB}MB</div>
                    </div>
                  )}
                </div>
                {pdfError && (
                  <p style={{ fontSize: 13, color: THEME.danger, margin: '-12px 0 16px' }}>{pdfError}</p>
                )}

                <div style={{ marginBottom: 20 }}>
                  <label style={{ display: 'block', fontSize: 13, fontWeight: 600, color: THEME.textMain, marginBottom: 8 }}>
                    简历文本（必填）
                  </label>
                  <Input.TextArea
                    rows={6}
                    value={form.resumeText}
                    onChange={(e) => setForm((current) => ({ ...current, resumeText: e.target.value }))}
                    placeholder="粘贴你的简历内容，AI 将根据你的项目经历和技术栈出题"
                    style={{ borderRadius: 8 }}
                  />
                </div>

                <div style={{ marginBottom: 20 }}>
                  <label style={{ display: 'block', fontSize: 13, fontWeight: 600, color: THEME.textMain, marginBottom: 8 }}>
                    目标岗位描述（可选）
                  </label>
                  <Input.TextArea
                    rows={3}
                    value={form.jobDescription}
                    onChange={(e) => setForm((current) => ({ ...current, jobDescription: e.target.value }))}
                    placeholder="粘贴目标岗位的 JD，AI 会结合岗位要求出更有针对性的题目"
                    style={{ borderRadius: 8 }}
                  />
                </div>
              </>
            )}

            {/* 知识练习模式 - 主题输入 */}
            {form.interviewMode !== 'resume_driven' && (
              <div style={{ marginBottom: 20 }}>
                <label style={{ display: 'block', fontSize: 13, fontWeight: 600, color: THEME.textMain, marginBottom: 8 }}>
                  主题（逗号或换行分隔）
                </label>
                <Input.TextArea
                  rows={4}
                  value={form.topicsText}
                  onChange={(e) => setForm((current) => ({ ...current, topicsText: e.target.value }))}
                  placeholder={`例如：${buildDefaultInterviewTopics(effectiveIndustryCode)}`}
                  style={{ borderRadius: 8 }}
                />
              </div>
            )}

            {/* 操作按钮 */}
            <div style={{ display: 'flex', gap: 12, alignItems: 'center' }}>
              <Button
                type="primary"
                htmlType="submit"
                size="large"
                icon={<PlayCircleOutlined />}
                loading={createMutation.isPending}
                style={{
                  background: THEME.primary,
                  borderColor: THEME.primary,
                  borderRadius: 8,
                  fontWeight: 600,
                  height: 48,
                  padding: '0 32px',
                }}
              >
                {createMutation.isPending ? '创建中...' : '开始面试'}
              </Button>

              {ongoingInterview && (
                <Link
                  to="/interview/$interviewId"
                  params={{ interviewId: String(ongoingInterview.id) }}
                  style={{ textDecoration: 'none' }}
                >
                  <Button
                    size="large"
                    icon={<RightOutlined />}
                    style={{ borderRadius: 8, height: 48 }}
                  >
                    继续进行中的会话
                  </Button>
                </Link>
              )}
            </div>

            {message && (
              <p style={{ fontSize: 13, color: THEME.textSecondary, margin: '12px 0 0' }}>{message}</p>
            )}
          </form>
        </div>

        {/* 右侧：历史记录 */}
        <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
          <div style={cardStyle}>
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 20 }}>
              <h2 style={{ fontSize: 16, fontWeight: 700, color: THEME.textMain, margin: 0 }}>
                历史记录
              </h2>
              {historyQuery.data?.list && historyQuery.data.list.length > 5 && (
                <Link
                  to="/interview/history"
                  style={{ fontSize: 13, color: THEME.primary, textDecoration: 'none' }}
                >
                  查看全部
                </Link>
              )}
            </div>

            {!accessToken ? (
              <div style={{ textAlign: 'center', padding: '24px 0' }}>
                <p style={{ fontSize: 14, color: THEME.textSecondary, marginBottom: 16 }}>登录后查看面试记录</p>
                <Button onClick={() => requestLoginPrompt('/interview', 'missing')}>
                  前往登录
                </Button>
              </div>
            ) : historyQuery.isLoading ? (
              <div style={{ textAlign: 'center', padding: '24px 0' }}><Spin /></div>
            ) : recentHistory.length === 0 ? (
              <Empty
                image={Empty.PRESENTED_IMAGE_SIMPLE}
                description="还没有面试记录"
                style={{ padding: '24px 0' }}
              />
            ) : (
              <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
                {recentHistory.map((item) => (
                  <div
                    key={item.id}
                    style={{
                      padding: '14px 16px',
                      borderRadius: THEME.radiusSm,
                      border: `1px solid ${THEME.border}`,
                      transition: 'all 0.2s ease',
                    }}
                    onMouseEnter={(e) => {
                      e.currentTarget.style.borderColor = THEME.borderHover
                      e.currentTarget.style.boxShadow = THEME.shadow
                    }}
                    onMouseLeave={(e) => {
                      e.currentTarget.style.borderColor = THEME.border
                      e.currentTarget.style.boxShadow = 'none'
                    }}
                  >
                    <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 8 }}>
                      <span style={{ fontSize: 14, fontWeight: 600, color: THEME.textMain }}>
                        面试 #{item.id}
                      </span>
                      <Tag
                        color={item.status === 'completed' ? 'success' : item.status === 'ongoing' ? 'processing' : 'default'}
                        style={{ margin: 0 }}
                      >
                        {interviewStatusLabel(item.status)}
                      </Tag>
                    </div>

                    <div style={{ fontSize: 13, color: THEME.textSecondary, marginBottom: 8 }}>
                      {item.total_questions} 题
                      {item.score ? ` · 得分 ${Math.round(item.score)}` : ''}
                    </div>

                    <div style={{ fontSize: 12, color: THEME.textMuted, marginBottom: 12 }}>
                      <ClockCircleOutlined style={{ marginRight: 4 }} />
                      {formatInterviewDateTime(item.started_at || item.created_at)}
                    </div>

                    {item.status === 'ongoing' || item.status === 'preparing' ? (
                      <Link
                        to="/interview/$interviewId"
                        params={{ interviewId: String(item.id) }}
                        style={{ textDecoration: 'none' }}
                      >
                        <Button size="small" type="primary" block style={{ borderRadius: 6 }}>
                          {item.status === 'preparing' ? '查看准备进度' : '继续面试'}
                        </Button>
                      </Link>
                    ) : (
                      <Link
                        to="/interview/$interviewId/report"
                        params={{ interviewId: String(item.id) }}
                        style={{ textDecoration: 'none' }}
                      >
                        <Button size="small" block style={{ borderRadius: 6 }}>
                          查看报告
                        </Button>
                      </Link>
                    )}
                  </div>
                ))}
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  )
}

export { InterviewSessionPage } from './InterviewSessionPage'
export { InterviewReportPage } from './InterviewReportPage'
