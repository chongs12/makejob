import type { FormEvent } from 'react'
import { useEffect, useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  Button,
  Input,
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
  Pagination,
} from 'antd'
import {
  PlusOutlined,
  DeleteOutlined,
  SaveOutlined,
  ReloadOutlined,
  InfoCircleOutlined,
  BookOutlined,
  CodeOutlined,
  FileTextOutlined,
  CheckCircleOutlined,
  QuestionCircleOutlined,
  TagsOutlined,
  ImportOutlined,
  ExportOutlined,
  DownOutlined,
  RightOutlined,
} from '@ant-design/icons'
import { extractErrorMessage, requestJson } from '@makejob/api-client'
import { isSuccessCode, type ApiEnvelope } from '@makejob/shared-types'
import { useAdminAuthStore } from '../../state/auth'

type QuestionType = 'choice' | 'multi' | 'code' | 'subjective'
type QuestionDifficulty = 'easy' | 'medium' | 'hard'

interface Industry {
  id: number
  code: string
  name: string
  is_active: boolean
}

interface Category {
  id: number
  industry_id: number
  name: string
  parent_id?: number | null
  sort_order: number
}

interface QuestionListItem {
  id: number
  category_id: number
  category_name: string
  industry_id: number
  type: QuestionType
  difficulty: QuestionDifficulty
  title: string
  content: string
  options: string[]
  answer: string
  explanation: string
  solution?: QuestionSolution | null
  judge_config?: QuestionJudgeConfig | null
  answer_template?: QuestionAnswerTemplate | null
  tags: string[]
  is_active: boolean
  created_at?: string
  updated_at?: string
}

interface QuestionSolution {
  summary: string
  approach: string
  key_steps: string[]
  edge_cases: string[]
  complexity: string
  common_mistakes: string[]
  recommended_tags: string[]
}

interface QuestionAnswerTemplate {
  core_conclusion: string
  key_points: string[]
  sample_answer: string
  follow_ups: string[]
  pitfalls: string[]
}

interface QuestionTestCase {
  input: string
  expected_output: string
  description?: string
}

interface QuestionReferenceSolution {
  language: string
  title?: string
  code: string
  explanation?: string
}

interface QuestionJudgeConfig {
  evaluation_mode: 'analysis_only' | 'testcase'
  default_language: string
  allowed_languages: string[]
  starter_code: string
  public_test_cases: QuestionTestCase[]
  hidden_test_cases?: QuestionTestCase[]
  reference_solutions?: QuestionReferenceSolution[]
  time_limit_ms: number
  memory_limit_mb: number
}

interface QuestionTagTaxonomyGroup {
  group: string
  description: string
  tags: string[]
}

interface PageResult<T> {
  list: T[]
  total: number
  page: number
  page_size: number
}

interface QuestionFilters {
  keyword: string
  difficulty: string
  categoryId: string
  page: number
}

interface QuestionFormState {
  industryId: string
  categoryId: string
  type: QuestionType
  difficulty: QuestionDifficulty
  title: string
  content: string
  optionsText: string
  answer: string
  explanation: string
  solutionSummary: string
  solutionApproach: string
  solutionStepsText: string
  solutionEdgeCasesText: string
  solutionComplexity: string
  solutionMistakesText: string
  evaluationMode: 'analysis_only' | 'testcase'
  defaultLanguage: string
  allowedLanguagesText: string
  starterCode: string
  publicCasesText: string
  hiddenCasesText: string
  referenceSolutionsText: string
  timeLimitMs: string
  memoryLimitMb: string
  answerTemplateConclusion: string
  answerTemplateKeyPointsText: string
  answerTemplateSampleAnswer: string
  answerTemplateFollowUpsText: string
  answerTemplatePitfallsText: string
  tagsText: string
  isActive: boolean
}

interface BatchImportResponse {
  total_count: number
  success_count: number
  fail_count: number
  errors?: string[]
}

interface CategoryOption {
  id: number
  industry_id: number
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

const QUESTION_TYPE_OPTIONS: Array<{ value: QuestionType; label: string }> = [
  { value: 'choice', label: '单选题' },
  { value: 'multi', label: '多选题' },
  { value: 'code', label: '编程题' },
  { value: 'subjective', label: '主观题' },
]

const QUESTION_DIFFICULTY_OPTIONS: Array<{ value: QuestionDifficulty; label: string }> = [
  { value: 'easy', label: '简单' },
  { value: 'medium', label: '中等' },
  { value: 'hard', label: '困难' },
]

const QUESTION_PAGE_SIZE = 10

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

async function fetchQuestions(token: string | null, filters: QuestionFilters): Promise<PageResult<QuestionListItem>> {
  const searchParams = new URLSearchParams({
    page: String(filters.page),
    page_size: String(QUESTION_PAGE_SIZE),
  })

  if (filters.keyword.trim()) {
    searchParams.set('keyword', filters.keyword.trim())
  }
  if (filters.difficulty) {
    searchParams.set('difficulty', filters.difficulty)
  }
  if (filters.categoryId) {
    searchParams.set('category_id', filters.categoryId)
  }

  const response = await requestJson<ApiEnvelope<PageResult<QuestionListItem>>>(`/admin/questions?${searchParams}`, {
    method: 'GET',
    token,
  })

  if (!isSuccessCode(response.code)) {
    throw new Error(response.message || '获取题库列表失败')
  }

  return response.data
}

async function fetchQuestionTagTaxonomy(token: string | null): Promise<QuestionTagTaxonomyGroup[]> {
  const response = await requestJson<ApiEnvelope<QuestionTagTaxonomyGroup[]>>('/admin/questions/tag-taxonomy', {
    method: 'GET',
    token,
  })

  if (!isSuccessCode(response.code)) {
    throw new Error(response.message || '获取题目标签词典失败')
  }

  return response.data
}

async function createQuestion(token: string | null, payload: Record<string, unknown>): Promise<QuestionListItem> {
  const response = await requestJson<ApiEnvelope<QuestionListItem>>('/admin/questions', {
    method: 'POST',
    token,
    body: payload,
  })

  if (!isSuccessCode(response.code)) {
    throw new Error(response.message || '创建题目失败')
  }

  return response.data
}

async function updateQuestion(token: string | null, id: number, payload: Record<string, unknown>): Promise<void> {
  const response = await requestJson<ApiEnvelope<null>>(`/admin/questions/${id}`, {
    method: 'PUT',
    token,
    body: payload,
  })

  if (!isSuccessCode(response.code)) {
    throw new Error(response.message || '更新题目失败')
  }
}

async function deleteQuestion(token: string | null, id: number): Promise<void> {
  const response = await requestJson<ApiEnvelope<null>>(`/admin/questions/${id}`, {
    method: 'DELETE',
    token,
  })

  if (!isSuccessCode(response.code)) {
    throw new Error(response.message || '删除题目失败')
  }
}

async function batchImportQuestions(
  token: string | null,
  industryCode: string,
  questions: Array<Record<string, unknown>>,
): Promise<BatchImportResponse> {
  const response = await requestJson<ApiEnvelope<BatchImportResponse>>('/admin/questions/import', {
    method: 'POST',
    token,
    body: {
      industry_code: industryCode,
      questions,
    },
  })

  if (!isSuccessCode(response.code)) {
    throw new Error(response.message || '批量导入题目失败')
  }

  return response.data
}

function buildInitialQuestionForm(): QuestionFormState {
  return {
    industryId: '',
    categoryId: '',
    type: 'choice',
    difficulty: 'medium',
    title: '',
    content: '',
    optionsText: '选项 A\n选项 B',
    answer: '',
    explanation: '',
    solutionSummary: '',
    solutionApproach: '',
    solutionStepsText: '',
    solutionEdgeCasesText: '',
    solutionComplexity: '',
    solutionMistakesText: '',
    evaluationMode: 'analysis_only',
    defaultLanguage: 'go',
    allowedLanguagesText: 'go',
    starterCode: '',
    publicCasesText: '',
    hiddenCasesText: '',
    referenceSolutionsText: '',
    timeLimitMs: '2000',
    memoryLimitMb: '128',
    answerTemplateConclusion: '',
    answerTemplateKeyPointsText: '',
    answerTemplateSampleAnswer: '',
    answerTemplateFollowUpsText: '',
    answerTemplatePitfallsText: '',
    tagsText: '',
    isActive: true,
  }
}

function buildQuestionForm(question?: QuestionListItem | null): QuestionFormState {
  if (!question) {
    return buildInitialQuestionForm()
  }

  return {
    industryId: String(question.industry_id),
    categoryId: String(question.category_id),
    type: question.type,
    difficulty: question.difficulty,
    title: question.title,
    content: question.content,
    optionsText: question.options.join('\n'),
    answer: question.answer,
    explanation: question.explanation || '',
    solutionSummary: question.solution?.summary || '',
    solutionApproach: question.solution?.approach || '',
    solutionStepsText: (question.solution?.key_steps || []).join('\n'),
    solutionEdgeCasesText: (question.solution?.edge_cases || []).join('\n'),
    solutionComplexity: question.solution?.complexity || '',
    solutionMistakesText: (question.solution?.common_mistakes || []).join('\n'),
    evaluationMode: question.judge_config?.evaluation_mode || 'analysis_only',
    defaultLanguage: question.judge_config?.default_language || 'go',
    allowedLanguagesText: (question.judge_config?.allowed_languages || []).join(', '),
    starterCode: question.judge_config?.starter_code || '',
    publicCasesText: JSON.stringify(question.judge_config?.public_test_cases || [], null, 2),
    hiddenCasesText: JSON.stringify(question.judge_config?.hidden_test_cases || [], null, 2),
    referenceSolutionsText: JSON.stringify(question.judge_config?.reference_solutions || [], null, 2),
    timeLimitMs: String(question.judge_config?.time_limit_ms || 2000),
    memoryLimitMb: String(question.judge_config?.memory_limit_mb || 128),
    answerTemplateConclusion: question.answer_template?.core_conclusion || '',
    answerTemplateKeyPointsText: (question.answer_template?.key_points || []).join('\n'),
    answerTemplateSampleAnswer: question.answer_template?.sample_answer || '',
    answerTemplateFollowUpsText: (question.answer_template?.follow_ups || []).join('\n'),
    answerTemplatePitfallsText: (question.answer_template?.pitfalls || []).join('\n'),
    tagsText: (question.tags || []).join(', '),
    isActive: question.is_active,
  }
}

function parseQuestionOptionsText(value: string): string[] {
  return value
    .split(/\r?\n/)
    .map((item) => item.trim())
    .filter(Boolean)
}

function parseQuestionTagsText(value: string): string[] {
  return Array.from(
    new Set(
      value
        .split(/[，,]/)
        .map((item) => item.trim())
        .filter(Boolean),
    ),
  )
}

function parseQuestionLineListText(value: string): string[] {
  return Array.from(
    new Set(
      value
        .split(/\r?\n/)
        .map((item) => item.trim())
        .filter(Boolean),
    ),
  )
}

function parseQuestionCasesText(value: string): QuestionTestCase[] {
  if (!value.trim()) {
    return []
  }
  const parsed = JSON.parse(value) as unknown
  return Array.isArray(parsed) ? (parsed as QuestionTestCase[]) : []
}

function parseQuestionReferenceSolutionsText(value: string): QuestionReferenceSolution[] {
  if (!value.trim()) {
    return []
  }
  const parsed = JSON.parse(value) as unknown
  return Array.isArray(parsed) ? (parsed as QuestionReferenceSolution[]) : []
}

function requiresQuestionOptions(questionType: QuestionType): boolean {
  return questionType === 'choice' || questionType === 'multi'
}

function buildQuestionPayload(form: QuestionFormState): Record<string, unknown> {
  const options = parseQuestionOptionsText(form.optionsText)
  const solution =
    form.type === 'code'
      ? {
          summary: form.solutionSummary.trim(),
          approach: form.solutionApproach.trim(),
          key_steps: parseQuestionLineListText(form.solutionStepsText),
          edge_cases: parseQuestionLineListText(form.solutionEdgeCasesText),
          complexity: form.solutionComplexity.trim(),
          common_mistakes: parseQuestionLineListText(form.solutionMistakesText),
          recommended_tags: parseQuestionTagsText(form.tagsText),
        }
      : null
  const answerTemplate =
    form.type === 'subjective'
      ? {
          core_conclusion: form.answerTemplateConclusion.trim(),
          key_points: parseQuestionLineListText(form.answerTemplateKeyPointsText),
          sample_answer: form.answerTemplateSampleAnswer.trim(),
          follow_ups: parseQuestionLineListText(form.answerTemplateFollowUpsText),
          pitfalls: parseQuestionLineListText(form.answerTemplatePitfallsText),
        }
      : null
  const judgeConfig =
    form.type === 'code'
      ? {
          evaluation_mode: form.evaluationMode,
          default_language: form.defaultLanguage.trim() || 'go',
          allowed_languages: parseQuestionTagsText(form.allowedLanguagesText),
          starter_code: form.starterCode,
          public_test_cases: parseQuestionCasesText(form.publicCasesText),
          hidden_test_cases: parseQuestionCasesText(form.hiddenCasesText),
          reference_solutions: parseQuestionReferenceSolutionsText(form.referenceSolutionsText),
          time_limit_ms: Number(form.timeLimitMs) || 2000,
          memory_limit_mb: Number(form.memoryLimitMb) || 128,
        }
      : null

  return {
    industry_id: Number(form.industryId),
    category_id: Number(form.categoryId),
    type: form.type,
    difficulty: form.difficulty,
    title: form.title.trim(),
    content: form.content.trim(),
    options_json: requiresQuestionOptions(form.type) ? JSON.stringify(options) : '',
    answer: form.answer.trim(),
    explanation: form.explanation.trim(),
    solution: solution || undefined,
    judge_config: judgeConfig || undefined,
    answer_template: answerTemplate || undefined,
    tags: parseQuestionTagsText(form.tagsText).join(','),
    is_active: form.isActive,
  }
}

function questionTypeLabel(type: string): string {
  return QUESTION_TYPE_OPTIONS.find((item) => item.value === type)?.label || type
}

function questionDifficultyLabel(difficulty: string): string {
  return QUESTION_DIFFICULTY_OPTIONS.find((item) => item.value === difficulty)?.label || difficulty
}

function buildCategoryOptions(categories: Category[], industryId: string): CategoryOption[] {
  const targetIndustryId = Number(industryId)
  const filtered = categories
    .filter((category) => !targetIndustryId || category.industry_id === targetIndustryId)
    .sort((left, right) => left.sort_order - right.sort_order || left.id - right.id)

  const childrenMap = new Map<number, Category[]>()
  const rootCategories: Category[] = []

  for (const category of filtered) {
    if (!category.parent_id) {
      rootCategories.push(category)
      continue
    }

    const currentChildren = childrenMap.get(category.parent_id) || []
    currentChildren.push(category)
    childrenMap.set(category.parent_id, currentChildren)
  }

  const result: CategoryOption[] = []

  function visitCategory(category: Category, depth: number): void {
    result.push({
      id: category.id,
      industry_id: category.industry_id,
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

function summarizeQuestionContent(content: string): string {
  const compact = content.replace(/\s+/g, ' ').trim()
  if (compact.length <= 96) {
    return compact
  }

  return `${compact.slice(0, 96)}...`
}

function parseBatchImportText(raw: string): Array<Record<string, unknown>> {
  const trimmed = raw.trim()
  if (!trimmed) {
    return []
  }

  const parsed = JSON.parse(trimmed) as unknown
  if (!Array.isArray(parsed)) {
    throw new Error('批量导入内容必须是 JSON 数组')
  }

  return parsed.map((item) => {
    if (!item || typeof item !== 'object' || Array.isArray(item)) {
      throw new Error('批量导入数组中的每一项都必须是对象')
    }

    return item as Record<string, unknown>
  })
}

function validateQuestionForm(form: QuestionFormState): string {
  if (!form.industryId) {
    return '请选择所属行业'
  }
  if (!form.categoryId) {
    return '请选择题目分类'
  }
  if (!form.title.trim()) {
    return '题目标题不能为空'
  }
  if (!form.content.trim()) {
    return '题目内容不能为空'
  }
  if (!form.answer.trim()) {
    return '题目答案不能为空'
  }
  if (requiresQuestionOptions(form.type) && parseQuestionOptionsText(form.optionsText).length < 2) {
    return '选择题至少需要两个选项'
  }
  if (form.type === 'code' && (!form.solutionSummary.trim() || !form.solutionApproach.trim())) {
    return '编程题至少需要补齐结构化解析中的“题意总结”和“解题思路”'
  }
  if (form.type === 'code' && form.evaluationMode === 'testcase') {
    try {
      const publicCases = parseQuestionCasesText(form.publicCasesText)
      const hiddenCases = parseQuestionCasesText(form.hiddenCasesText)
      const referenceSolutions = parseQuestionReferenceSolutionsText(form.referenceSolutionsText)
      if (!form.defaultLanguage.trim()) {
        return '测试用例判题模式必须填写默认语言'
      }
      if (publicCases.length === 0) {
        return '测试用例判题模式至少需要一条公开样例'
      }
      if (hiddenCases.length === 0) {
        return '测试用例判题模式至少需要一条隐藏用例'
      }
      if (referenceSolutions.length === 0) {
        return '测试用例判题模式至少需要一份参考实现'
      }
    } catch {
      return '测试用例和参考实现必须是合法 JSON'
    }
  }
  if (form.type === 'subjective' && !form.answerTemplateConclusion.trim()) {
    return '主观题至少需要补齐参考作答模板中的“核心结论”'
  }

  return ''
}

export function QuestionPage() {
  const accessToken = useAdminAuthStore((state) => state.accessToken)
  const queryClient = useQueryClient()
  const [filters, setFilters] = useState<QuestionFilters>({
    keyword: '',
    difficulty: '',
    categoryId: '',
    page: 1,
  })
  const [selectedQuestionId, setSelectedQuestionId] = useState<number | null>(null)
  const [form, setForm] = useState<QuestionFormState>(buildInitialQuestionForm())
  const [editorMessage, setEditorMessage] = useState('读取题库列表中')
  const [importIndustryCode, setImportIndustryCode] = useState('')
  const [importText, setImportText] = useState(
    '[\n  {\n    "category_name": "Go 基础",\n    "type": "choice",\n    "difficulty": "easy",\n    "title": "Go 的切片底层是什么？",\n    "content": "下面关于 slice 的说法，哪一个更准确？",\n    "options_json": "[\\"动态数组视图\\",\\"固定长度数组\\"]",\n    "answer": "动态数组视图",\n    "explanation": "slice 本质上是对底层数组的描述结构。",\n    "tags": "slice,基础"\n  }\n]',
  )
  const [expandedGroups, setExpandedGroups] = useState<Set<string>>(new Set())

  const toggleGroup = (group: string) => {
    setExpandedGroups((prev) => {
      const next = new Set(prev)
      if (next.has(group)) {
        next.delete(group)
      } else {
        next.add(group)
      }
      return next
    })
  }

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

  const questionTagTaxonomyQuery = useQuery({
    queryKey: ['admin', 'question-tag-taxonomy', accessToken],
    queryFn: () => fetchQuestionTagTaxonomy(accessToken),
    enabled: Boolean(accessToken),
  })

  const questionsQuery = useQuery({
    queryKey: [
      'admin',
      'questions',
      accessToken,
      filters.keyword,
      filters.difficulty,
      filters.categoryId,
      filters.page,
    ],
    queryFn: () => fetchQuestions(accessToken, filters),
    enabled: Boolean(accessToken),
  })

  const formCategoryOptions = useMemo(
    () => buildCategoryOptions(categoriesQuery.data || [], form.industryId),
    [categoriesQuery.data, form.industryId],
  )
  const filterCategoryOptions = useMemo(
    () => buildCategoryOptions(categoriesQuery.data || [], ''),
    [categoriesQuery.data],
  )
  const formError = useMemo(() => validateQuestionForm(form), [form])
  const totalPages = useMemo(() => {
    const total = questionsQuery.data?.total || 0
    return Math.max(1, Math.ceil(total / QUESTION_PAGE_SIZE))
  }, [questionsQuery.data?.total])
  const importPreview = useMemo(() => {
    try {
      return {
        valid: true,
        count: parseBatchImportText(importText).length,
        error: '',
      }
    } catch (error) {
      return {
        valid: false,
        count: 0,
        error: extractErrorMessage(error, '批量导入 JSON 解析失败'),
      }
    }
  }, [importText])

  useEffect(() => {
    if (!questionsQuery.data) {
      return
    }

    if (selectedQuestionId === null) {
      setEditorMessage((current) => (current === '读取题库列表中' ? '已同步题库列表。' : current))
      return
    }

    const nextQuestion = questionsQuery.data.list.find((item) => item.id === selectedQuestionId)
    if (nextQuestion) {
      setForm(buildQuestionForm(nextQuestion))
    }
  }, [questionsQuery.data, selectedQuestionId])

  useEffect(() => {
    if (importIndustryCode || !(industriesQuery.data || []).length) {
      return
    }

    setImportIndustryCode(industriesQuery.data?.[0]?.code || '')
  }, [importIndustryCode, industriesQuery.data])

  useEffect(() => {
    if (!form.industryId || !form.categoryId) {
      return
    }

    const currentCategory = (categoriesQuery.data || []).find((item) => item.id === Number(form.categoryId))
    if (currentCategory && currentCategory.industry_id !== Number(form.industryId)) {
      setForm((current) => ({
        ...current,
        categoryId: '',
      }))
    }
  }, [categoriesQuery.data, form.categoryId, form.industryId])

  const saveMutation = useMutation({
    mutationFn: async () => {
      const payload = buildQuestionPayload(form)

      if (selectedQuestionId) {
        await updateQuestion(accessToken, selectedQuestionId, payload)
        return selectedQuestionId
      }

      const created = await createQuestion(accessToken, payload)
      return created?.id
    },
    onSuccess: async (questionId) => {
      setSelectedQuestionId(questionId)
      const msg = selectedQuestionId ? '题目已更新。' : '题目已创建。'
      setEditorMessage(msg)
      message.success(msg)
      await queryClient.invalidateQueries({
        queryKey: ['admin', 'questions'],
      })
    },
    onError: (error) => {
      const msg = extractErrorMessage(error, '保存题目失败，请稍后重试')
      setEditorMessage(msg)
      message.error(msg)
    },
  })

  const deleteMutation = useMutation({
    mutationFn: async (id: number) => {
      await deleteQuestion(accessToken, id)
    },
    onSuccess: async () => {
      setSelectedQuestionId(null)
      setForm(buildInitialQuestionForm())
      setEditorMessage('题目已删除。')
      message.success('题目已删除')
      await queryClient.invalidateQueries({
        queryKey: ['admin', 'questions'],
      })
    },
    onError: (error) => {
      const msg = extractErrorMessage(error, '删除题目失败，请稍后重试')
      setEditorMessage(msg)
      message.error(msg)
    },
  })

  const importMutation = useMutation({
    mutationFn: async () => {
      if (!importIndustryCode) {
        throw new Error('请选择导入目标行业')
      }

      const questions = parseBatchImportText(importText)
      if (questions.length === 0) {
        throw new Error('批量导入内容不能为空')
      }

      return batchImportQuestions(accessToken, importIndustryCode, questions)
    },
    onSuccess: async (result) => {
      const msg = `批量导入完成：共 ${result.total_count ?? 0} 条，成功 ${result.success_count ?? 0} 条，失败 ${result.fail_count ?? 0} 条。`
      setEditorMessage(msg)
      message.success(msg)
      await queryClient.invalidateQueries({
        queryKey: ['admin', 'questions'],
      })
    },
    onError: (error) => {
      const msg = extractErrorMessage(error, '批量导入题目失败，请稍后重试')
      setEditorMessage(msg)
      message.error(msg)
    },
  })

  function startCreatingQuestion(): void {
    setSelectedQuestionId(null)
    setForm(buildInitialQuestionForm())
    setEditorMessage('已切换到新建题目模式。')
  }

  function startEditingQuestion(question: QuestionListItem): void {
    setSelectedQuestionId(question.id)
    setForm(buildQuestionForm(question))
    setEditorMessage(`正在编辑题目：${question.title}`)
  }

  function updateQuestionField<Key extends keyof QuestionFormState>(key: Key, value: QuestionFormState[Key]): void {
    setForm((current) => ({
      ...current,
      [key]: value,
    }))
  }

  function appendQuestionTag(tag: string): void {
    const nextTags = Array.from(new Set([...parseQuestionTagsText(form.tagsText), tag]))
    updateQuestionField('tagsText', nextTags.join(', '))
  }

  function handleSubmit(event: FormEvent<HTMLFormElement>): void {
    event.preventDefault()

    if (formError) {
      message.warning(formError)
      return
    }

    setEditorMessage(selectedQuestionId ? '正在更新题目。' : '正在创建题目。')
    saveMutation.mutate()
  }

  function handleDelete(): void {
    if (!selectedQuestionId) {
      return
    }

    Modal.confirm({
      title: '确认删除题目',
      content: '删除后不可恢复，确定要继续吗？',
      okText: '删除',
      okType: 'danger',
      cancelText: '取消',
      onOk: () => {
        setEditorMessage('正在删除题目。')
        deleteMutation.mutate(selectedQuestionId)
      },
    })
  }

  function handleImport(event: FormEvent<HTMLFormElement>): void {
    event.preventDefault()
    setEditorMessage('正在批量导入题目。')
    importMutation.mutate()
  }

  if (industriesQuery.isLoading || categoriesQuery.isLoading || questionsQuery.isLoading || questionTagTaxonomyQuery.isLoading) {
    return (
      <div style={{ padding: 40, display: 'flex', justifyContent: 'center', alignItems: 'center', minHeight: 400 }}>
        <Spin size="large" tip="正在加载题目、行业和分类数据..." />
      </div>
    )
  }

  if (industriesQuery.isError || categoriesQuery.isError || questionsQuery.isError || questionTagTaxonomyQuery.isError) {
    return (
      <div style={{ padding: 40 }}>
        <div style={{ ...solidCard, padding: 40, textAlign: 'center' }}>
          <InfoCircleOutlined style={{ fontSize: 48, color: THEME.danger, marginBottom: 16 }} />
          <h3 style={{ color: THEME.textMain, marginBottom: 8 }}>数据加载失败</h3>
          <p style={{ color: THEME.textSecondary }}>
            {extractErrorMessage(
              questionsQuery.error || categoriesQuery.error || industriesQuery.error || questionTagTaxonomyQuery.error,
              '读取题库管理数据失败',
            )}
          </p>
        </div>
      </div>
    )
  }

  const typeColorMap: Record<QuestionType, string> = {
    choice: '#4f46e5',
    multi: '#6366f1',
    code: '#f59e0b',
    subjective: '#10b981',
  }

  const difficultyColorMap: Record<QuestionDifficulty, string> = {
    easy: '#10b981',
    medium: '#f59e0b',
    hard: '#ef4444',
  }

  return (
    <div style={{ padding: '32px 28px', background: THEME.bg, minHeight: '100vh' }}>
      {/* Hero */}
      <div style={{ ...glassCard, padding: '28px 32px', marginBottom: 24, display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
        <div>
          <Space align="center" style={{ marginBottom: 8 }}>
            <Tag color="processing" style={{ fontSize: 12, fontWeight: 600, borderRadius: 20, padding: '2px 12px' }}>
              题库中心
            </Tag>
          </Space>
          <h2 style={{ margin: 0, fontSize: 24, fontWeight: 700, color: THEME.textMain }}>题库管理</h2>
          <p style={{ margin: '8px 0 0', color: THEME.textSecondary, fontSize: 14, maxWidth: 600 }}>
            当前页支持题目分页筛选、手动维护和快速编辑。右侧表单会根据题型自动切换选择题选项区，避免在字符串字段上来回猜格式。
          </p>
        </div>
        <div style={{ textAlign: 'right', flexShrink: 0, marginLeft: 24 }}>
          <div style={{ fontSize: 32, fontWeight: 800, color: THEME.primary, lineHeight: 1 }}>
            {questionsQuery.data?.total || 0}
          </div>
          <div style={{ fontSize: 12, color: THEME.textMuted, marginTop: 4 }}>道题</div>
        </div>
      </div>

      {/* Toolbar */}
      <div style={{ ...solidCard, padding: '18px 24px', marginBottom: 24, display: 'flex', alignItems: 'center', gap: 16, flexWrap: 'wrap' }}>
        <div style={{ minWidth: 200, flex: 1 }}>
          <div style={{ marginBottom: 4, fontSize: 12, fontWeight: 600, color: THEME.textMuted }}>关键词</div>
          <Input.Search
            value={filters.keyword}
            onChange={(e) => setFilters((current) => ({ ...current, keyword: e.target.value, page: 1 }))}
            placeholder="标题或内容关键词"
            allowClear
            style={{ borderRadius: 10 }}
          />
        </div>
        <div style={{ minWidth: 140 }}>
          <div style={{ marginBottom: 4, fontSize: 12, fontWeight: 600, color: THEME.textMuted }}>难度</div>
          <Select
            value={filters.difficulty || undefined}
            onChange={(val) => setFilters((current) => ({ ...current, difficulty: val || '', page: 1 }))}
            placeholder="全部难度"
            allowClear
            style={{ width: '100%', borderRadius: 10 }}
            dropdownStyle={{ borderRadius: 10 }}
          >
            {QUESTION_DIFFICULTY_OPTIONS.map((option) => (
              <Select.Option key={option.value} value={option.value}>
                {option.label}
              </Select.Option>
            ))}
          </Select>
        </div>
        <div style={{ minWidth: 180 }}>
          <div style={{ marginBottom: 4, fontSize: 12, fontWeight: 600, color: THEME.textMuted }}>分类</div>
          <Select
            value={filters.categoryId || undefined}
            onChange={(val) => setFilters((current) => ({ ...current, categoryId: val || '', page: 1 }))}
            placeholder="全部分类"
            allowClear
            style={{ width: '100%', borderRadius: 10 }}
            dropdownStyle={{ borderRadius: 10 }}
          >
            {filterCategoryOptions.map((option) => (
              <Select.Option key={option.id} value={String(option.id)}>
                {option.label}
              </Select.Option>
            ))}
          </Select>
        </div>
        <Button
          type="primary"
          icon={<PlusOutlined />}
          onClick={startCreatingQuestion}
          style={{
            borderRadius: 10,
            background: THEME.primary,
            borderColor: THEME.primary,
            fontWeight: 600,
            marginTop: 20,
          }}
        >
          新建题目
        </Button>
      </div>

      {/* Batch Import */}
      <div style={{ ...solidCard, padding: 24, marginBottom: 24 }}>
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 16, flexWrap: 'wrap', gap: 12 }}>
          <div>
            <h4 style={{ margin: 0, fontSize: 15, fontWeight: 700, color: THEME.textMain }}>
              <ImportOutlined style={{ marginRight: 8, color: THEME.primary }} />
              批量导入题目
            </h4>
            <p style={{ margin: '4px 0 0', fontSize: 13, color: THEME.textMuted }}>
              直接粘贴 JSON 数组，结构与 /api/admin/questions/import 保持一致。
            </p>
          </div>
          <div style={{ minWidth: 180 }}>
            <div style={{ marginBottom: 4, fontSize: 12, fontWeight: 600, color: THEME.textMuted }}>目标行业</div>
            <Select
              value={importIndustryCode || undefined}
              onChange={(val) => setImportIndustryCode(val || '')}
              placeholder="请选择行业"
              style={{ width: '100%', borderRadius: 10 }}
              dropdownStyle={{ borderRadius: 10 }}
            >
              {(industriesQuery.data || []).map((industry) => (
                <Select.Option key={industry.code} value={industry.code}>
                  {industry.name}
                </Select.Option>
              ))}
            </Select>
          </div>
        </div>

        <form onSubmit={handleImport}>
          <Input.TextArea
            value={importText}
            onChange={(e) => setImportText(e.target.value)}
            rows={6}
            style={{ borderRadius: 10, resize: 'none', fontFamily: 'monospace', fontSize: 13 }}
          />

          <div
            style={{
              marginTop: 12,
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'space-between',
              flexWrap: 'wrap',
              gap: 12,
            }}
          >
            <span
              style={{
                fontSize: 13,
                color: importPreview.valid ? THEME.success : THEME.danger,
                display: 'flex',
                alignItems: 'center',
                gap: 6,
              }}
            >
              <InfoCircleOutlined />
              {importPreview.valid
                ? `当前已识别 ${importPreview.count} 条待导入题目。`
                : importPreview.error}
            </span>
            <Button
              type="primary"
              htmlType="submit"
              icon={<ExportOutlined />}
              loading={importMutation.isPending}
              disabled={!importPreview.valid || !importIndustryCode}
              style={{
                borderRadius: 10,
                background: THEME.primary,
                borderColor: THEME.primary,
                fontWeight: 600,
              }}
            >
              {importMutation.isPending ? '导入中...' : '开始批量导入'}
            </Button>
          </div>
        </form>
      </div>

      {/* Main Layout */}
      <Row gutter={[24, 24]}>
        {/* Question List */}
        <Col xs={24} lg={10}>
          <div style={{ ...solidCard, padding: 24 }}>
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 16 }}>
              <h3 style={{ margin: 0, fontSize: 16, fontWeight: 700, color: THEME.textMain }}>
                <BookOutlined style={{ marginRight: 8, color: THEME.primary }} />
                题目列表
              </h3>
              <span style={{ fontSize: 12, color: THEME.textMuted }}>
                共 {questionsQuery.data?.total || 0} 条
              </span>
            </div>

            <div style={{ display: 'flex', flexDirection: 'column', gap: 10, marginBottom: 20, maxHeight: 600, overflowY: 'auto', paddingRight: 4 }}>
              {(questionsQuery.data?.list || []).length === 0 ? (
                <Empty
                  image={Empty.PRESENTED_IMAGE_SIMPLE}
                  description={
                    <div>
                      <strong style={{ color: THEME.textMain }}>当前筛选条件下没有题目</strong>
                      <p style={{ color: THEME.textMuted, fontSize: 13, margin: '4px 0 0' }}>
                        可以先调整筛选条件，或者直接在右侧创建新题目。
                      </p>
                    </div>
                  }
                />
              ) : (
                (questionsQuery.data?.list || []).map((question) => {
                  const isActive = selectedQuestionId === question.id
                  return (
                    <button
                      key={question.id}
                      type="button"
                      onClick={() => startEditingQuestion(question)}
                      style={{
                        display: 'flex',
                        flexDirection: 'column',
                        gap: 8,
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
                        <strong style={{ fontSize: 14, color: THEME.textMain, fontWeight: 600 }}>
                          {question.title}
                        </strong>
                        <Tag color={question.is_active ? 'success' : 'default'} style={{ fontSize: 11, borderRadius: 10, flexShrink: 0, marginLeft: 8 }}>
                          {question.is_active ? '启用中' : '已停用'}
                        </Tag>
                      </div>
                      <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
                        <Tag
                          style={{
                            borderRadius: 10,
                            fontSize: 11,
                            color: typeColorMap[question.type],
                            background: `${typeColorMap[question.type]}15`,
                            border: `1px solid ${typeColorMap[question.type]}30`,
                          }}
                        >
                          {questionTypeLabel(question.type)}
                        </Tag>
                        <Tag
                          style={{
                            borderRadius: 10,
                            fontSize: 11,
                            color: difficultyColorMap[question.difficulty],
                            background: `${difficultyColorMap[question.difficulty]}15`,
                            border: `1px solid ${difficultyColorMap[question.difficulty]}30`,
                          }}
                        >
                          {questionDifficultyLabel(question.difficulty)}
                        </Tag>
                        <span style={{ fontSize: 12, color: THEME.textMuted }}>{question.category_name}</span>
                      </div>
                      <p style={{ margin: 0, fontSize: 12, color: THEME.textSecondary, lineHeight: 1.5 }}>
                        {summarizeQuestionContent(question.content)}
                      </p>
                      {(question.tags || []).length > 0 && (
                        <div style={{ display: 'flex', gap: 6, flexWrap: 'wrap' }}>
                          {(question.tags || []).map((tag) => (
                            <Tag key={tag} style={{ borderRadius: 10, fontSize: 11, border: `1px solid ${THEME.border}` }}>
                              {tag}
                            </Tag>
                          ))}
                        </div>
                      )}
                    </button>
                  )
                })
              )}
            </div>

            {/* Pagination */}
            <div style={{ display: 'flex', justifyContent: 'center' }}>
              <Pagination
                current={questionsQuery.data?.page || filters.page}
                total={questionsQuery.data?.total || 0}
                pageSize={QUESTION_PAGE_SIZE}
                onChange={(page) => setFilters((current) => ({ ...current, page }))}
                showSizeChanger={false}
                style={{ textAlign: 'center' }}
              />
            </div>
          </div>
        </Col>

        {/* Question Editor */}
        <Col xs={24} lg={14}>
          <div style={{ ...solidCard, padding: 24 }}>
            <form onSubmit={handleSubmit}>
              <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 20, flexWrap: 'wrap', gap: 12 }}>
                <div>
                  <h3 style={{ margin: 0, fontSize: 16, fontWeight: 700, color: THEME.textMain }}>
                    {selectedQuestionId ? '编辑题目' : '新建题目'}
                  </h3>
                  <p style={{ margin: '4px 0 0', fontSize: 13, color: THEME.textMuted }}>{editorMessage}</p>
                </div>
                <Tag color="processing" style={{ borderRadius: 10, fontWeight: 600 }}>
                  {selectedQuestionId ? `ID #${selectedQuestionId}` : '新题目'}
                </Tag>
              </div>

              <Row gutter={[16, 16]}>
                <Col span={12}>
                  <div style={{ marginBottom: 4, fontSize: 13, fontWeight: 600, color: THEME.textMain }}>所属行业</div>
                  <Select
                    value={form.industryId || undefined}
                    onChange={(val) => updateQuestionField('industryId', val || '')}
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
                  <div style={{ marginBottom: 4, fontSize: 13, fontWeight: 600, color: THEME.textMain }}>分类</div>
                  <Select
                    value={form.categoryId || undefined}
                    onChange={(val) => updateQuestionField('categoryId', val || '')}
                    placeholder="请选择分类"
                    style={{ width: '100%', borderRadius: 10 }}
                    dropdownStyle={{ borderRadius: 10 }}
                  >
                    {formCategoryOptions.map((option) => (
                      <Select.Option key={option.id} value={String(option.id)}>
                        {option.label}
                      </Select.Option>
                    ))}
                  </Select>
                </Col>
                <Col span={8}>
                  <div style={{ marginBottom: 4, fontSize: 13, fontWeight: 600, color: THEME.textMain }}>题型</div>
                  <Select
                    value={form.type}
                    onChange={(val) => updateQuestionField('type', val as QuestionType)}
                    style={{ width: '100%', borderRadius: 10 }}
                    dropdownStyle={{ borderRadius: 10 }}
                  >
                    {QUESTION_TYPE_OPTIONS.map((option) => (
                      <Select.Option key={option.value} value={option.value}>
                        {option.label}
                      </Select.Option>
                    ))}
                  </Select>
                </Col>
                <Col span={8}>
                  <div style={{ marginBottom: 4, fontSize: 13, fontWeight: 600, color: THEME.textMain }}>难度</div>
                  <Select
                    value={form.difficulty}
                    onChange={(val) => updateQuestionField('difficulty', val as QuestionDifficulty)}
                    style={{ width: '100%', borderRadius: 10 }}
                    dropdownStyle={{ borderRadius: 10 }}
                  >
                    {QUESTION_DIFFICULTY_OPTIONS.map((option) => (
                      <Select.Option key={option.value} value={option.value}>
                        {option.label}
                      </Select.Option>
                    ))}
                  </Select>
                </Col>
                <Col span={8}>
                  <div style={{ marginBottom: 4, fontSize: 13, fontWeight: 600, color: THEME.textMain }}>标签</div>
                  <Input
                    value={form.tagsText}
                    onChange={(e) => updateQuestionField('tagsText', e.target.value)}
                    placeholder="并发, channel, context"
                    style={{ borderRadius: 10 }}
                  />
                </Col>
                <Col span={24}>
                  <div style={{ marginBottom: 4, fontSize: 13, fontWeight: 600, color: THEME.textMain }}>题目标题</div>
                  <Input
                    value={form.title}
                    onChange={(e) => updateQuestionField('title', e.target.value)}
                    placeholder="请输入题目标题"
                    style={{ borderRadius: 10 }}
                  />
                </Col>
                <Col span={24}>
                  <div style={{ marginBottom: 4, fontSize: 13, fontWeight: 600, color: THEME.textMain }}>题目内容</div>
                  <Input.TextArea
                    value={form.content}
                    onChange={(e) => updateQuestionField('content', e.target.value)}
                    placeholder="请输入完整题干内容"
                    rows={4}
                    style={{ borderRadius: 10, resize: 'none' }}
                  />
                </Col>

                {requiresQuestionOptions(form.type) && (
                  <Col span={24}>
                    <div style={{ marginBottom: 4, fontSize: 13, fontWeight: 600, color: THEME.textMain }}>选项列表</div>
                    <Input.TextArea
                      value={form.optionsText}
                      onChange={(e) => updateQuestionField('optionsText', e.target.value)}
                      placeholder={'每行一个选项，例如：\n选项 A\n选项 B'}
                      rows={4}
                      style={{ borderRadius: 10, resize: 'none' }}
                    />
                  </Col>
                )}

                <Col span={24}>
                  <div style={{ marginBottom: 4, fontSize: 13, fontWeight: 600, color: THEME.textMain }}>答案</div>
                  <Input.TextArea
                    value={form.answer}
                    onChange={(e) => updateQuestionField('answer', e.target.value)}
                    placeholder="请输入标准答案"
                    rows={3}
                    style={{ borderRadius: 10, resize: 'none' }}
                  />
                </Col>
                <Col span={24}>
                  <div style={{ marginBottom: 4, fontSize: 13, fontWeight: 600, color: THEME.textMain }}>解析</div>
                  <Input.TextArea
                    value={form.explanation}
                    onChange={(e) => updateQuestionField('explanation', e.target.value)}
                    placeholder="请输入题目解析"
                    rows={3}
                    style={{ borderRadius: 10, resize: 'none' }}
                  />
                </Col>
              </Row>

              {/* Code Question Sections */}
              {form.type === 'code' && (
                <>
                  <Divider style={{ margin: '24px 0 16px' }} />
                  <div style={{ marginBottom: 16 }}>
                    <h4 style={{ margin: 0, fontSize: 15, fontWeight: 700, color: THEME.textMain }}>
                      <CodeOutlined style={{ marginRight: 8, color: THEME.accent }} />
                      编程题结构化解析
                    </h4>
                    <p style={{ margin: '4px 0 0', fontSize: 13, color: THEME.textMuted }}>
                      这里维护 P0 阶段要求的统一解析结构，前台会按这个结构直接展示。
                    </p>
                  </div>
                  <Row gutter={[16, 16]}>
                    <Col span={24}>
                      <div style={{ marginBottom: 4, fontSize: 13, fontWeight: 600, color: THEME.textMain }}>题意总结</div>
                      <Input.TextArea
                        value={form.solutionSummary}
                        onChange={(e) => updateQuestionField('solutionSummary', e.target.value)}
                        placeholder="一句话说明这道题核心在考什么"
                        rows={2}
                        style={{ borderRadius: 10, resize: 'none' }}
                      />
                    </Col>
                    <Col span={24}>
                      <div style={{ marginBottom: 4, fontSize: 13, fontWeight: 600, color: THEME.textMain }}>解题思路</div>
                      <Input.TextArea
                        value={form.solutionApproach}
                        onChange={(e) => updateQuestionField('solutionApproach', e.target.value)}
                        placeholder="说明为什么采用这套解法，以及关键策略是什么"
                        rows={2}
                        style={{ borderRadius: 10, resize: 'none' }}
                      />
                    </Col>
                    <Col span={24}>
                      <div style={{ marginBottom: 4, fontSize: 13, fontWeight: 600, color: THEME.textMain }}>关键步骤</div>
                      <Input.TextArea
                        value={form.solutionStepsText}
                        onChange={(e) => updateQuestionField('solutionStepsText', e.target.value)}
                        placeholder={'每行一条，例如：\n确定状态定义\n初始化边界\n按转移关系推进'}
                        rows={3}
                        style={{ borderRadius: 10, resize: 'none' }}
                      />
                    </Col>
                    <Col span={24}>
                      <div style={{ marginBottom: 4, fontSize: 13, fontWeight: 600, color: THEME.textMain }}>边界条件</div>
                      <Input.TextArea
                        value={form.solutionEdgeCasesText}
                        onChange={(e) => updateQuestionField('solutionEdgeCasesText', e.target.value)}
                        placeholder={'每行一条，例如：\n空输入\n长度为 1\n重复元素'}
                        rows={3}
                        style={{ borderRadius: 10, resize: 'none' }}
                      />
                    </Col>
                    <Col span={24}>
                      <div style={{ marginBottom: 4, fontSize: 13, fontWeight: 600, color: THEME.textMain }}>复杂度分析</div>
                      <Input.TextArea
                        value={form.solutionComplexity}
                        onChange={(e) => updateQuestionField('solutionComplexity', e.target.value)}
                        placeholder="例如：时间复杂度 O(n)，空间复杂度 O(1)"
                        rows={2}
                        style={{ borderRadius: 10, resize: 'none' }}
                      />
                    </Col>
                    <Col span={24}>
                      <div style={{ marginBottom: 4, fontSize: 13, fontWeight: 600, color: THEME.textMain }}>常见错法</div>
                      <Input.TextArea
                        value={form.solutionMistakesText}
                        onChange={(e) => updateQuestionField('solutionMistakesText', e.target.value)}
                        placeholder={'每行一条，例如：\n漏掉边界判断\n索引越界\n复杂度分析缺失'}
                        rows={3}
                        style={{ borderRadius: 10, resize: 'none' }}
                      />
                    </Col>
                  </Row>

                  <Divider style={{ margin: '24px 0 16px' }} />
                  <div style={{ marginBottom: 16 }}>
                    <h4 style={{ margin: 0, fontSize: 15, fontWeight: 700, color: THEME.textMain }}>
                      <CheckCircleOutlined style={{ marginRight: 8, color: THEME.success }} />
                      编程题判题配置
                    </h4>
                    <p style={{ margin: '4px 0 0', fontSize: 13, color: THEME.textMuted }}>
                      测试用例模式下将使用公开样例运行、隐藏用例提交判题；AI 只负责讲解，不负责最终裁决。
                    </p>
                  </div>
                  <Row gutter={[16, 16]}>
                    <Col span={12}>
                      <div style={{ marginBottom: 4, fontSize: 13, fontWeight: 600, color: THEME.textMain }}>判题模式</div>
                      <Select
                        value={form.evaluationMode}
                        onChange={(val) => updateQuestionField('evaluationMode', val as 'analysis_only' | 'testcase')}
                        style={{ width: '100%', borderRadius: 10 }}
                        dropdownStyle={{ borderRadius: 10 }}
                      >
                        <Select.Option value="analysis_only">AI 分析模式</Select.Option>
                        <Select.Option value="testcase">测试用例判题模式</Select.Option>
                      </Select>
                    </Col>
                    <Col span={12}>
                      <div style={{ marginBottom: 4, fontSize: 13, fontWeight: 600, color: THEME.textMain }}>默认语言</div>
                      <Input
                        value={form.defaultLanguage}
                        onChange={(e) => updateQuestionField('defaultLanguage', e.target.value)}
                        placeholder="go"
                        style={{ borderRadius: 10 }}
                      />
                    </Col>
                    <Col span={12}>
                      <div style={{ marginBottom: 4, fontSize: 13, fontWeight: 600, color: THEME.textMain }}>允许语言</div>
                      <Input
                        value={form.allowedLanguagesText}
                        onChange={(e) => updateQuestionField('allowedLanguagesText', e.target.value)}
                        placeholder="go, python, javascript"
                        style={{ borderRadius: 10 }}
                      />
                    </Col>
                    <Col span={12}>
                      <div style={{ marginBottom: 4, fontSize: 13, fontWeight: 600, color: THEME.textMain }}>起始模板代码</div>
                      <Input
                        value={form.starterCode}
                        onChange={(e) => updateQuestionField('starterCode', e.target.value)}
                        placeholder="可选，用于初始化编辑器内容"
                        style={{ borderRadius: 10 }}
                      />
                    </Col>
                    <Col span={12}>
                      <div style={{ marginBottom: 4, fontSize: 13, fontWeight: 600, color: THEME.textMain }}>时间限制(ms)</div>
                      <Input
                        value={form.timeLimitMs}
                        onChange={(e) => updateQuestionField('timeLimitMs', e.target.value)}
                        placeholder="2000"
                        style={{ borderRadius: 10 }}
                      />
                    </Col>
                    <Col span={12}>
                      <div style={{ marginBottom: 4, fontSize: 13, fontWeight: 600, color: THEME.textMain }}>内存限制(MB)</div>
                      <Input
                        value={form.memoryLimitMb}
                        onChange={(e) => updateQuestionField('memoryLimitMb', e.target.value)}
                        placeholder="128"
                        style={{ borderRadius: 10 }}
                      />
                    </Col>
                    <Col span={24}>
                      <div style={{ marginBottom: 4, fontSize: 13, fontWeight: 600, color: THEME.textMain }}>公开样例 JSON</div>
                      <Input.TextArea
                        value={form.publicCasesText}
                        onChange={(e) => updateQuestionField('publicCasesText', e.target.value)}
                        placeholder={'[\n  {\n    "input": "3\\n1 2 3",\n    "expected_output": "6",\n    "description": "基础样例"\n  }\n]'}
                        rows={5}
                        style={{ borderRadius: 10, resize: 'none', fontFamily: 'monospace', fontSize: 13 }}
                      />
                    </Col>
                    <Col span={24}>
                      <div style={{ marginBottom: 4, fontSize: 13, fontWeight: 600, color: THEME.textMain }}>隐藏用例 JSON</div>
                      <Input.TextArea
                        value={form.hiddenCasesText}
                        onChange={(e) => updateQuestionField('hiddenCasesText', e.target.value)}
                        placeholder={'[\n  {\n    "input": "0",\n    "expected_output": "0",\n    "description": "边界场景"\n  }\n]'}
                        rows={5}
                        style={{ borderRadius: 10, resize: 'none', fontFamily: 'monospace', fontSize: 13 }}
                      />
                    </Col>
                    <Col span={24}>
                      <div style={{ marginBottom: 4, fontSize: 13, fontWeight: 600, color: THEME.textMain }}>参考实现 JSON</div>
                      <Input.TextArea
                        value={form.referenceSolutionsText}
                        onChange={(e) => updateQuestionField('referenceSolutionsText', e.target.value)}
                        placeholder={'[\n  {\n    "language": "go",\n    "title": "Go 参考实现",\n    "code": "package main\\n\\nfunc main() {}"\n  }\n]'}
                        rows={5}
                        style={{ borderRadius: 10, resize: 'none', fontFamily: 'monospace', fontSize: 13 }}
                      />
                    </Col>
                  </Row>
                </>
              )}

              {/* Subjective Question Sections */}
              {form.type === 'subjective' && (
                <>
                  <Divider style={{ margin: '24px 0 16px' }} />
                  <div style={{ marginBottom: 16 }}>
                    <h4 style={{ margin: 0, fontSize: 15, fontWeight: 700, color: THEME.textMain }}>
                      <QuestionCircleOutlined style={{ marginRight: 8, color: THEME.success }} />
                      主观题参考回答模板
                    </h4>
                    <p style={{ margin: '4px 0 0', fontSize: 13, color: THEME.textMuted }}>
                      这里维护面试化回答模板，前台会按"结论 + 展开 + 追问"结构展示。
                    </p>
                  </div>
                  <Row gutter={[16, 16]}>
                    <Col span={24}>
                      <div style={{ marginBottom: 4, fontSize: 13, fontWeight: 600, color: THEME.textMain }}>核心结论</div>
                      <Input.TextArea
                        value={form.answerTemplateConclusion}
                        onChange={(e) => updateQuestionField('answerTemplateConclusion', e.target.value)}
                        placeholder="先给出这道题最关键的结论"
                        rows={2}
                        style={{ borderRadius: 10, resize: 'none' }}
                      />
                    </Col>
                    <Col span={24}>
                      <div style={{ marginBottom: 4, fontSize: 13, fontWeight: 600, color: THEME.textMain }}>关键展开点</div>
                      <Input.TextArea
                        value={form.answerTemplateKeyPointsText}
                        onChange={(e) => updateQuestionField('answerTemplateKeyPointsText', e.target.value)}
                        placeholder={'每行一条，例如：\n解释原理\n说明适用场景\n补充优缺点'}
                        rows={3}
                        style={{ borderRadius: 10, resize: 'none' }}
                      />
                    </Col>
                    <Col span={24}>
                      <div style={{ marginBottom: 4, fontSize: 13, fontWeight: 600, color: THEME.textMain }}>面试表达示例</div>
                      <Input.TextArea
                        value={form.answerTemplateSampleAnswer}
                        onChange={(e) => updateQuestionField('answerTemplateSampleAnswer', e.target.value)}
                        placeholder="写一版更接近真实面试表达的完整回答"
                        rows={4}
                        style={{ borderRadius: 10, resize: 'none' }}
                      />
                    </Col>
                    <Col span={24}>
                      <div style={{ marginBottom: 4, fontSize: 13, fontWeight: 600, color: THEME.textMain }}>高频追问点</div>
                      <Input.TextArea
                        value={form.answerTemplateFollowUpsText}
                        onChange={(e) => updateQuestionField('answerTemplateFollowUpsText', e.target.value)}
                        placeholder={'每行一条，例如：\n为什么这样设计？\n边界是什么？\n替代方案是什么？'}
                        rows={3}
                        style={{ borderRadius: 10, resize: 'none' }}
                      />
                    </Col>
                    <Col span={24}>
                      <div style={{ marginBottom: 4, fontSize: 13, fontWeight: 600, color: THEME.textMain }}>易答偏点</div>
                      <Input.TextArea
                        value={form.answerTemplatePitfallsText}
                        onChange={(e) => updateQuestionField('answerTemplatePitfallsText', e.target.value)}
                        placeholder={'每行一条，例如：\n只背定义\n不讲场景\n忽略权衡'}
                        rows={3}
                        style={{ borderRadius: 10, resize: 'none' }}
                      />
                    </Col>
                  </Row>
                </>
              )}

              {/* Tag Taxonomy Suggestions */}
              {questionTagTaxonomyQuery.data?.length ? (
                <>
                  <Divider style={{ margin: '24px 0 16px' }} />
                  <div style={{ marginBottom: 16 }}>
                    <h4 style={{ margin: 0, fontSize: 15, fontWeight: 700, color: THEME.textMain }}>
                      <TagsOutlined style={{ marginRight: 8, color: THEME.primary }} />
                      标准标签建议
                    </h4>
                    <p style={{ margin: '4px 0 0', fontSize: 13, color: THEME.textMuted }}>
                      P0 阶段优先复用这套标签，避免同义词、英文大小写和临时口径继续扩散。点击分组展开查看标签。
                    </p>
                  </div>
                  <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
                    {questionTagTaxonomyQuery.data.map((group) => {
                      const isExpanded = expandedGroups.has(group.group)
                      return (
                        <div
                          key={group.group}
                          style={{
                            borderRadius: 12,
                            border: `1px solid ${THEME.border}`,
                            background: THEME.cardBg,
                            overflow: 'hidden',
                            transition: 'all 0.2s ease',
                          }}
                        >
                          <button
                            type="button"
                            onClick={() => toggleGroup(group.group)}
                            style={{
                              width: '100%',
                              display: 'flex',
                              alignItems: 'center',
                              justifyContent: 'space-between',
                              padding: '12px 16px',
                              border: 'none',
                              background: 'transparent',
                              cursor: 'pointer',
                              textAlign: 'left',
                              fontSize: 14,
                              fontWeight: 600,
                              color: THEME.textMain,
                            }}
                          >
                            <span style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
                              <span
                                style={{
                                  width: 28,
                                  height: 28,
                                  borderRadius: 8,
                                  background: THEME.primaryLight,
                                  color: THEME.primary,
                                  display: 'flex',
                                  alignItems: 'center',
                                  justifyContent: 'center',
                                  fontSize: 12,
                                  fontWeight: 700,
                                }}
                              >
                                {group.group[0]?.toUpperCase()}
                              </span>
                              <span>{group.group}</span>
                              <span style={{ fontSize: 11, color: THEME.textMuted, fontWeight: 400 }}>
                                ({group.tags.length} 个标签)
                              </span>
                            </span>
                            <span
                              style={{
                                color: THEME.textMuted,
                                fontSize: 12,
                                transition: 'transform 0.2s ease',
                                transform: isExpanded ? 'rotate(0deg)' : 'rotate(-90deg)',
                              }}
                            >
                              {isExpanded ? <DownOutlined /> : <RightOutlined />}
                            </span>
                          </button>
                          {isExpanded && (
                            <div
                              style={{
                                padding: '0 16px 14px',
                                borderTop: `1px solid ${THEME.border}`,
                              }}
                            >
                              <p style={{ margin: '10px 0', fontSize: 12, color: THEME.textSecondary }}>
                                {group.description}
                              </p>
                              <div style={{ display: 'flex', flexWrap: 'wrap', gap: 8 }}>
                                {group.tags.map((tag) => (
                                  <Button
                                    key={`${group.group}-${tag}`}
                                    size="small"
                                    onClick={() => appendQuestionTag(tag)}
                                    style={{
                                      borderRadius: 10,
                                      fontSize: 12,
                                      border: `1px solid ${THEME.border}`,
                                      color: THEME.textSecondary,
                                    }}
                                  >
                                    + {tag}
                                  </Button>
                                ))}
                              </div>
                            </div>
                          )}
                        </div>
                      )
                    })}
                  </div>
                </>
              ) : null}

              {/* Form Status */}
              <div
                style={{
                  marginTop: 24,
                  padding: '12px 16px',
                  borderRadius: 10,
                  background: formError ? 'rgba(239,68,68,0.06)' : 'rgba(16,185,129,0.06)',
                  border: formError ? '1px solid rgba(239,68,68,0.15)' : '1px solid rgba(16,185,129,0.15)',
                  fontSize: 13,
                  color: formError ? THEME.danger : THEME.success,
                  display: 'flex',
                  alignItems: 'center',
                  gap: 8,
                }}
              >
                <InfoCircleOutlined />
                <strong style={{ marginRight: 4 }}>表单检查</strong>
                {formError || '当前题目表单已通过基础校验，可以提交保存。'}
              </div>

              <div style={{ marginTop: 16, display: 'flex', alignItems: 'center', gap: 12 }}>
                <Switch
                  checked={form.isActive}
                  onChange={(checked) => updateQuestionField('isActive', checked)}
                />
                <span style={{ fontSize: 13, color: THEME.textSecondary }}>
                  {form.isActive ? '当前题目启用中' : '当前题目已停用'}
                </span>
              </div>

              {/* Actions */}
              <div style={{ marginTop: 24, display: 'flex', justifyContent: 'flex-end', gap: 12 }}>
                <Button
                  icon={<ReloadOutlined />}
                  onClick={startCreatingQuestion}
                  disabled={saveMutation.isPending || deleteMutation.isPending}
                  style={{ borderRadius: 10, fontWeight: 600 }}
                >
                  重置为新建
                </Button>
                {selectedQuestionId && (
                  <Button
                    danger
                    icon={<DeleteOutlined />}
                    onClick={handleDelete}
                    loading={deleteMutation.isPending}
                    disabled={saveMutation.isPending}
                    style={{ borderRadius: 10, fontWeight: 600 }}
                  >
                    {deleteMutation.isPending ? '删除中...' : '删除题目'}
                  </Button>
                )}
                <Button
                  type="primary"
                  htmlType="submit"
                  icon={<SaveOutlined />}
                  loading={saveMutation.isPending}
                  disabled={Boolean(formError) || deleteMutation.isPending}
                  style={{
                    borderRadius: 10,
                    background: THEME.primary,
                    borderColor: THEME.primary,
                    fontWeight: 600,
                    minWidth: 120,
                  }}
                >
                  {saveMutation.isPending ? '保存中...' : selectedQuestionId ? '保存修改' : '创建题目'}
                </Button>
              </div>
            </form>
          </div>
        </Col>
      </Row>
    </div>
  )
}
