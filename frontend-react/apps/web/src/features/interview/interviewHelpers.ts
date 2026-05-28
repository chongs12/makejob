import { getApiBaseUrl } from '@makejob/api-client'
import { DEFAULT_FRONTEND_INDUSTRY_CODE as INTERVIEW_DEFAULT_INDUSTRY_CODE } from '../../shared/industryContext'
import type {
  InterviewConfigForm,
  InterviewDetailResponse,
  InterviewMessage,
  InterviewQuestion,
  InterviewReport,
} from './interviewTypes'

export const INTERVIEW_AUTO_STOP_SILENCE_MS = 1800
export const INTERVIEW_AUTO_STOP_LEVEL_THRESHOLD = 0.008
export const INTERVIEW_MAX_RECORDING_MS = 60000

/**
 * 根据当前行业生成更贴近方向语境的默认面试主题，减少用户首次填写成本。
 */
export function buildDefaultInterviewTopics(industryCode: string): string {
  const topicMap: Record<string, string> = {
    go: 'Go基础, 并发编程, Gin, 数据库',
    java: 'Java基础, JVM, Spring, 数据库',
    frontend: 'HTML/CSS, JavaScript, React, 工程化',
  }

  return topicMap[industryCode.trim()] || '基础概念, 核心框架, 数据结构, 工程实践'
}

/**
 * 创建 AI 面试入口页默认表单，保证首次进入时就能直接提交。
 */
export function buildInitialInterviewForm(): InterviewConfigForm {
  return {
    difficulty: 'medium',
    questionCount: '5',
    topicsText: buildDefaultInterviewTopics(INTERVIEW_DEFAULT_INDUSTRY_CODE),
    live2dModelKey: '',
    interviewMode: 'general',
    resumeText: '',
    jobDescription: '',
  }
}

/**
 * 将主题输入拆成后端需要的数组结构，兼容逗号和换行分隔。
 */
export function parseInterviewTopics(value: string): string[] {
  return Array.from(
    new Set(
      value
        .split(/[\n,，]/)
        .map((item) => item.trim())
        .filter(Boolean),
    ),
  )
}

/**
 * 将面试难度枚举转换为前台可读文案。
 */
export function interviewDifficultyLabel(difficulty: string): string {
  const map: Record<string, string> = {
    easy: '简单',
    medium: '中等',
    hard: '困难',
    mixed: '混合',
  }

  return map[difficulty] || difficulty || '未知'
}

/**
 * 将面试状态转换成更清晰的中文标签。
 */
export function interviewStatusLabel(status: string): string {
  const map: Record<string, string> = {
    ongoing: '进行中',
    completed: '已完成',
    cancelled: '已取消',
  }

  return map[status] || status || '未知状态'
}

/**
 * 将题目类型转换成前台可读标签。
 */
export function interviewQuestionTypeLabel(type: string): string {
  const map: Record<string, string> = {
    technical: '技术题',
    behavioral: '行为题',
    coding: '编程题',
  }

  return map[type] || type || '未分类'
}

/**
 * 格式化面试页面中的时间文本，保持整体显示口径一致。
 */
export function formatInterviewDateTime(value?: string): string {
  if (!value) {
    return '--'
  }

  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return value
  }

  return date.toLocaleString('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  })
}

/**
 * 将秒数转换为更易读的分钟文本，用于面试报告展示。
 */
export function formatInterviewDuration(seconds: number): string {
  if (!seconds) {
    return '不足 1 分钟'
  }

  const minutes = Math.max(Math.round(seconds / 60), 1)
  return `${minutes} 分钟`
}

/**
 * 将维度键名转换成更适合报告页展示的中文标签。
 */
export function interviewDimensionLabel(key: string): string {
  const normalizedKey = key.trim().toLowerCase()
  const map: Record<string, string> = {
    foundation: '基础知识',
    basics: '基础知识',
    knowledge: '知识掌握',
    technical: '技术深度',
    coding: '编码能力',
    communication: '表达沟通',
    architecture: '架构理解',
    problem_solving: '问题分析',
    'problem-solving': '问题分析',
    behavioral: '行为表达',
  }

  return map[normalizedKey] || key || '未命名维度'
}

/**
 * 根据总分生成更适合报告总览展示的准备度结论。
 */
export function buildInterviewReadiness(score: number): { label: string; description: string } {
  if (score >= 85) {
    return {
      label: '可直接冲刺',
      description: '整体表现已经具备较强竞争力，建议开始强化追问深度和临场表达稳定性。',
    }
  }

  if (score >= 70) {
    return {
      label: '接近可投递',
      description: '主体能力已形成，但仍有若干薄弱点需要集中补强后再去冲高质量面试。',
    }
  }

  if (score >= 55) {
    return {
      label: '需要补强',
      description: '当前更适合先回到题库和学习计划，把关键薄弱项补齐后再继续模拟面试。',
    }
  }

  return {
    label: '建议先夯实基础',
    description: '基础稳定性还不够，优先回练核心知识点，再用面试场景验证提升效果。',
  }
}

/**
 * 将维度评分整理成排序后的数组，便于报告页同时展示优势项和薄弱项。
 */
export function normalizeInterviewDimensions(report?: InterviewReport | null): Array<{ key: string; label: string; score: number }> {
  return Object.entries(report?.dimension_scores || {})
    .map(([key, value]) => ({
      key,
      label: interviewDimensionLabel(key),
      score: Math.round(value || 0),
    }))
    .sort((left, right) => right.score - left.score)
}

/**
 * 从面试消息历史中还原题目与回答轨迹，供报告页做复盘回放。
 */
export function buildInterviewReplayItems(messages: InterviewMessage[]): Array<{
  question: string
  answer: string
  askedAt: string
  answeredAt: string
}> {
  const result: Array<{
    question: string
    answer: string
    askedAt: string
    answeredAt: string
  }> = []

  let currentQuestion: InterviewMessage | null = null

  for (const item of messages) {
    if (item.role === 'ai' && item.message_type === 'text' && item.content.trim()) {
      currentQuestion = item
      continue
    }

    if (item.role === 'user' && currentQuestion) {
      result.push({
        question: currentQuestion.content,
        answer: item.content,
        askedAt: currentQuestion.created_at,
        answeredAt: item.created_at,
      })
      currentQuestion = null
    }
  }

  return result
}

/**
 * 生成可直接带入社区发帖页的面试复盘草稿。
 */
export function buildInterviewReviewDraft(
  report: InterviewReport,
  detail: InterviewDetailResponse | undefined,
  industryLabel: string,
): {
  postType: string
  title: string
  content: string
  tags: string[]
} {
  const topWeaknesses = (report.weaknesses || []).slice(0, 3)
  const topSuggestions = (report.suggestions || []).slice(0, 3)
  const topDimensions = normalizeInterviewDimensions(report).slice(-3).map((item) => `${item.label} ${item.score}分`)

  return {
    postType: 'article',
    title: `${industryLabel} 面试复盘：第 ${detail?.id || '-'} 场`,
    content: [
      `这次 ${industryLabel} 模拟面试已经结束，先记录本场复盘。`,
      '',
      `一、结果概览`,
      `- 总分：${Math.round(report.overall_score || 0)}`,
      `- 题量：${report.total_questions || detail?.total_questions || 0}`,
      `- 命中题数：${report.correct_count || 0}`,
      `- 总结：${report.summary || '暂无总结'}`,
      '',
      `二、当前最需要补强的点`,
      ...(topWeaknesses.length ? topWeaknesses.map((item) => `- ${item}`) : ['- 暂无明确薄弱项，后续可继续挑战更深问题。']),
      '',
      `三、低分维度`,
      ...(topDimensions.length ? topDimensions.map((item) => `- ${item}`) : ['- 暂无维度评分数据']),
      '',
      `四、下一步行动`,
      ...(topSuggestions.length ? topSuggestions.map((item) => `- ${item}`) : ['- 先回题库补强，再进行下一场模拟面试。']),
    ].join('\n'),
    tags: Array.from(new Set([industryLabel, '面试复盘', 'AI面试', ...topWeaknesses.slice(0, 2)])).slice(0, 5),
  }
}

/**
 * 统计消息列表中已经提交过多少条用户回答，用于恢复当前题号。
 */
export function countInterviewAnswers(messages: InterviewMessage[]): number {
  return messages.filter((item) => item.role === 'user').length
}

/**
 * 从后端返回的面试详情中恢复当前待答题目，兼容刷新页面后的状态恢复。
 */
export function resolveCurrentInterviewQuestion(detail: InterviewDetailResponse | undefined): InterviewQuestion | null {
  if (!detail || detail.status !== 'ongoing') {
    return null
  }

  if (detail.current_question) {
    return detail.current_question
  }

  const answeredCount = countInterviewAnswers(detail.messages)
  if (answeredCount >= detail.total_questions) {
    return null
  }

  const latestQuestionMessage = [...detail.messages]
    .reverse()
    .find((item) => item.role === 'ai' && item.message_type === 'text')

  if (!latestQuestionMessage) {
    return null
  }

  return latestQuestionMessage.question || {
    question: latestQuestionMessage.content,
    topic: '',
    difficulty: '',
    type: 'technical',
    hints: '',
  }
}

/**
 * 从运行时消息列表恢复当前待回答题目，兼容 WebSocket 实时追加的新消息。
 */
export function resolveCurrentInterviewQuestionFromMessages(
  messages: InterviewMessage[],
  status: string,
  totalQuestions: number,
): InterviewQuestion | null {
  if (status !== 'ongoing') {
    return null
  }

  const answeredCount = countInterviewAnswers(messages)
  if (answeredCount >= totalQuestions) {
    return null
  }

  const latestQuestionMessage = [...messages]
    .reverse()
    .find((item) => item.role === 'ai' && item.message_type === 'text')

  if (!latestQuestionMessage) {
    return null
  }

  return latestQuestionMessage.question || {
    question: latestQuestionMessage.content,
    topic: '',
    difficulty: '',
    type: 'technical',
    hints: '',
  }
}

/**
 * 生成浏览器可直连的面试 WebSocket 地址，并通过查询参数透传访问令牌。
 */
export function buildInterviewWebSocketUrl(interviewId: string, token: string): string {
  const baseUrl = getApiBaseUrl().replace(/\/+$/, '')
  const requestBase = typeof window !== 'undefined' ? new URL(baseUrl, window.location.origin) : new URL(baseUrl, 'http://localhost')
  requestBase.protocol = requestBase.protocol === 'https:' ? 'wss:' : 'ws:'
  requestBase.pathname = `${requestBase.pathname.replace(/\/+$/, '')}/interviews/${interviewId}/ws`
  requestBase.searchParams.set('token', token)
  return requestBase.toString()
}

/**
 * 构造一条前台本地即时消息，统一实时链路与 HTTP 回退链路的数据结构。
 */
export function buildRealtimeInterviewMessage(
  role: string,
  messageType: string,
  content: string,
  question?: InterviewQuestion | null,
): InterviewMessage {
  return {
    role,
    message_type: messageType,
    content,
    question,
    created_at: new Date().toISOString(),
  }
}

/**
 * 追加实时消息时做一次尾部去重，避免恢复消息与实时事件重复渲染。
 */
export function appendInterviewMessage(messages: InterviewMessage[], nextMessage: InterviewMessage): InterviewMessage[] {
  const lastMessage = messages[messages.length - 1]
  if (
    lastMessage &&
    lastMessage.role === nextMessage.role &&
    lastMessage.message_type === nextMessage.message_type &&
    lastMessage.content === nextMessage.content
  ) {
    return messages
  }

  return [...messages, nextMessage]
}

/**
 * 将 16bit PCM 数据编码成 base64，供 WebSocket 直接上传。
 */
export function encodePCM16Base64FromInt16(pcmBuffer: Int16Array): string {
  if (pcmBuffer.length === 0) {
    return ''
  }

  const bytes = new Uint8Array(pcmBuffer.buffer)
  let binary = ''
  const chunkSize = 0x8000
  for (let offset = 0; offset < bytes.length; offset += chunkSize) {
    const slice = bytes.subarray(offset, Math.min(offset + chunkSize, bytes.length))
    binary += String.fromCharCode(...slice)
  }

  return window.btoa(binary)
}

/**
 * 将浏览器采集的 Float32 音频按目标采样率重采样并转换为 16bit PCM。
 */
export function resampleFloat32ToPCM16(
  channelData: Float32Array,
  sourceSampleRate: number,
  targetSampleRate = 16000,
): Int16Array {
  if (!channelData.length) {
    return new Int16Array(0)
  }

  const normalizedSourceRate = sourceSampleRate > 0 ? sourceSampleRate : targetSampleRate
  if (normalizedSourceRate === targetSampleRate) {
    const pcmBuffer = new Int16Array(channelData.length)
    for (let index = 0; index < channelData.length; index += 1) {
      const sample = Math.max(-1, Math.min(1, channelData[index] || 0))
      pcmBuffer[index] = sample < 0 ? sample * 0x8000 : sample * 0x7fff
    }
    return pcmBuffer
  }

  const sampleRatio = normalizedSourceRate / targetSampleRate
  const targetLength = Math.max(1, Math.round(channelData.length / sampleRatio))
  const pcmBuffer = new Int16Array(targetLength)

  for (let targetIndex = 0; targetIndex < targetLength; targetIndex += 1) {
    const sourceIndex = Math.min(channelData.length - 1, Math.round(targetIndex * sampleRatio))
    const sample = Math.max(-1, Math.min(1, channelData[sourceIndex] || 0))
    pcmBuffer[targetIndex] = sample < 0 ? sample * 0x8000 : sample * 0x7fff
  }

  return pcmBuffer
}

/**
 * 将 Float32 单声道音频转换为 16bit PCM 并编码成 base64，供 WebSocket 直接上传。
 */
export function encodePCM16Base64(
  channelData: Float32Array,
  sourceSampleRate = 16000,
  targetSampleRate = 16000,
): string {
  const pcmBuffer = resampleFloat32ToPCM16(channelData, sourceSampleRate, targetSampleRate)
  return encodePCM16Base64FromInt16(pcmBuffer)
}

/**
 * 将字幕文本拆成逐步显示的最小单元，兼容中文字符和常见 emoji。
 */
export function splitInterviewDialogueUnits(text: string): string[] {
  return Array.from(text)
}

/**
 * 按文本长度与标点停顿粗略估算字幕播放时长，供无音频时兜底同步。
 */
export function estimateInterviewDialogueDurationMs(text: string): number {
  const units = splitInterviewDialogueUnits(text)
  const punctuationCount = units.filter((unit) => /[，。！？；：,.!?]/.test(unit)).length
  const baseDurationMs = units.length * 110 + punctuationCount * 140
  return Math.min(Math.max(baseDurationMs, 1400), 12000)
}
