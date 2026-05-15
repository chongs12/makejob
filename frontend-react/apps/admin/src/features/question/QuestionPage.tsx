import type { FormEvent } from 'react'
import { useEffect, useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
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

/**
 * 获取后台行业列表，供题目编辑表单做行业选择。
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
 * 获取后台分类列表，供筛选器和题目表单复用。
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
 * 按分页和筛选条件获取后台题库列表。
 */
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

/**
 * 获取后台题目标签词典，辅助题库治理阶段统一标签口径。
 */
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

/**
 * 创建新的题目记录，并返回服务端保存后的结果。
 */
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

/**
 * 更新指定题目，保持后台题库和前端编辑结果一致。
 */
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

/**
 * 删除指定题目，供后台快速清理无效内容。
 */
async function deleteQuestion(token: string | null, id: number): Promise<void> {
  const response = await requestJson<ApiEnvelope<null>>(`/admin/questions/${id}`, {
    method: 'DELETE',
    token,
  })

  if (!isSuccessCode(response.code)) {
    throw new Error(response.message || '删除题目失败')
  }
}

/**
 * 调用后台批量导入接口，导入指定行业的题目集合。
 */
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

/**
 * 构造题目表单初始值，避免新建态出现 undefined。
 */
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

/**
 * 将题目记录转换成可编辑表单，统一编辑态与新建态的数据模型。
 */
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
    tagsText: question.tags.join(', '),
    isActive: question.is_active,
  }
}

/**
 * 将多行选项输入解析为数组，兼容空行和首尾空白。
 */
function parseQuestionOptionsText(value: string): string[] {
  return value
    .split(/\r?\n/)
    .map((item) => item.trim())
    .filter(Boolean)
}

/**
 * 将标签输入解析为去重后的字符串数组，兼容中文逗号与英文逗号。
 */
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

/**
 * 将多行文本解析为去重后的字符串数组，供结构化解析和模板字段复用。
 */
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

/**
 * 解析测试用例 JSON 文本，供编程题判题配置表单复用。
 */
function parseQuestionCasesText(value: string): QuestionTestCase[] {
  if (!value.trim()) {
    return []
  }
  const parsed = JSON.parse(value) as unknown
  return Array.isArray(parsed) ? (parsed as QuestionTestCase[]) : []
}

/**
 * 解析参考实现 JSON 文本，供编程题判题配置表单复用。
 */
function parseQuestionReferenceSolutionsText(value: string): QuestionReferenceSolution[] {
  if (!value.trim()) {
    return []
  }
  const parsed = JSON.parse(value) as unknown
  return Array.isArray(parsed) ? (parsed as QuestionReferenceSolution[]) : []
}

/**
 * 根据题型判断当前题目是否需要选项列表。
 */
function requiresQuestionOptions(questionType: QuestionType): boolean {
  return questionType === 'choice' || questionType === 'multi'
}

/**
 * 将题目表单转换为后端可直接消费的请求体。
 */
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

/**
 * 将题型值转换为后台列表可读的中文标签。
 */
function questionTypeLabel(type: string): string {
  return QUESTION_TYPE_OPTIONS.find((item) => item.value === type)?.label || type
}

/**
 * 将难度值转换为后台列表可读的中文标签。
 */
function questionDifficultyLabel(difficulty: string): string {
  return QUESTION_DIFFICULTY_OPTIONS.find((item) => item.value === difficulty)?.label || difficulty
}

/**
 * 按父子分类关系拍平分类选项，便于表单中展示层级结构。
 */
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

  /**
   * 深度优先展开分类树，并为子节点补上层级缩进。
   */
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

/**
 * 截断较长的题干摘要，减少列表区阅读噪音。
 */
function summarizeQuestionContent(content: string): string {
  const compact = content.replace(/\s+/g, ' ').trim()
  if (compact.length <= 96) {
    return compact
  }

  return `${compact.slice(0, 96)}...`
}

/**
 * 解析批量导入 JSON 文本，并校验其是否为对象数组。
 */
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

/**
 * 校验当前题目表单，提前发现缺字段或选择题选项不足的问题。
 */
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

/**
 * 提供后台题库管理页，支持分页筛选、创建、编辑和删除题目。
 */
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
  const [message, setMessage] = useState('读取题库列表中')
  const [importIndustryCode, setImportIndustryCode] = useState('')
  const [importText, setImportText] = useState(
    '[\n  {\n    "category_name": "Go 基础",\n    "type": "choice",\n    "difficulty": "easy",\n    "title": "Go 的切片底层是什么？",\n    "content": "下面关于 slice 的说法，哪一个更准确？",\n    "options_json": "[\\"动态数组视图\\",\\"固定长度数组\\"]",\n    "answer": "动态数组视图",\n    "explanation": "slice 本质上是对底层数组的描述结构。",\n    "tags": "slice,基础"\n  }\n]',
  )

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
      setMessage((current) => (current === '读取题库列表中' ? '已同步题库列表。' : current))
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
      return created.id
    },
    onSuccess: async (questionId) => {
      setSelectedQuestionId(questionId)
      setMessage(selectedQuestionId ? '题目已更新。' : '题目已创建。')
      await queryClient.invalidateQueries({
        queryKey: ['admin', 'questions'],
      })
    },
    onError: (error) => {
      setMessage(extractErrorMessage(error, '保存题目失败，请稍后重试'))
    },
  })

  const deleteMutation = useMutation({
    mutationFn: async (id: number) => {
      await deleteQuestion(accessToken, id)
    },
    onSuccess: async () => {
      setSelectedQuestionId(null)
      setForm(buildInitialQuestionForm())
      setMessage('题目已删除。')
      await queryClient.invalidateQueries({
        queryKey: ['admin', 'questions'],
      })
    },
    onError: (error) => {
      setMessage(extractErrorMessage(error, '删除题目失败，请稍后重试'))
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
      setMessage(
        `批量导入完成：共 ${result.total_count} 条，成功 ${result.success_count} 条，失败 ${result.fail_count} 条。`,
      )
      await queryClient.invalidateQueries({
        queryKey: ['admin', 'questions'],
      })
    },
    onError: (error) => {
      setMessage(extractErrorMessage(error, '批量导入题目失败，请稍后重试'))
    },
  })

  /**
   * 切换到题目新建模式，并重置当前编辑表单。
   */
  function startCreatingQuestion(): void {
    setSelectedQuestionId(null)
    setForm(buildInitialQuestionForm())
    setMessage('已切换到新建题目模式。')
  }

  /**
   * 把指定题目装载到右侧编辑区，继续维护已有题目。
   */
  function startEditingQuestion(question: QuestionListItem): void {
    setSelectedQuestionId(question.id)
    setForm(buildQuestionForm(question))
    setMessage(`正在编辑题目：${question.title}`)
  }

  /**
   * 更新题目表单字段，集中管理输入状态。
   */
  function updateQuestionField<Key extends keyof QuestionFormState>(key: Key, value: QuestionFormState[Key]): void {
    setForm((current) => ({
      ...current,
      [key]: value,
    }))
  }

  /**
   * 将标准标签追加进当前表单，减少后台手动输入时的同义词漂移。
   */
  function appendQuestionTag(tag: string): void {
    const nextTags = Array.from(new Set([...parseQuestionTagsText(form.tagsText), tag]))
    updateQuestionField('tagsText', nextTags.join(', '))
  }

  /**
   * 提交题目表单并执行创建或更新。
   */
  function handleSubmit(event: FormEvent<HTMLFormElement>): void {
    event.preventDefault()

    if (formError) {
      setMessage(formError)
      return
    }

    setMessage(selectedQuestionId ? '正在更新题目。' : '正在创建题目。')
    saveMutation.mutate()
  }

  /**
   * 删除当前选中的题目记录。
   */
  function handleDelete(): void {
    if (!selectedQuestionId) {
      return
    }

    if (!window.confirm('确认删除当前题目吗？删除后不可恢复。')) {
      return
    }

    setMessage('正在删除题目。')
    deleteMutation.mutate(selectedQuestionId)
  }

  /**
   * 提交批量导入表单，并调用后台题库导入接口。
   */
  function handleImport(event: FormEvent<HTMLFormElement>): void {
    event.preventDefault()
    setMessage('正在批量导入题目。')
    importMutation.mutate()
  }

  if (industriesQuery.isLoading || categoriesQuery.isLoading || questionsQuery.isLoading || questionTagTaxonomyQuery.isLoading) {
    return (
      <section className="admin-panel">
        <span className="admin-tag">题库中心</span>
        <h2>题库管理</h2>
        <p className="admin-copy">正在加载题目、行业和分类数据。</p>
      </section>
    )
  }

  if (industriesQuery.isError || categoriesQuery.isError || questionsQuery.isError || questionTagTaxonomyQuery.isError) {
    return (
      <section className="admin-panel">
        <span className="admin-tag">题库中心</span>
        <h2>题库管理</h2>
        <p className="admin-copy">
          {extractErrorMessage(
            questionsQuery.error || categoriesQuery.error || industriesQuery.error || questionTagTaxonomyQuery.error,
            '读取题库管理数据失败',
          )}
        </p>
      </section>
    )
  }

  return (
    <section className="admin-panel admin-question-page">
      <div className="admin-question-page__hero">
        <div>
          <span className="admin-tag">题库中心</span>
          <h2>题库管理</h2>
          <p className="admin-copy">
            当前页支持题目分页筛选、手动维护和快速编辑。右侧表单会根据题型自动切换选择题选项区，避免在字符串字段上来回猜格式。
          </p>
        </div>
        <div className="admin-question-page__summary">
          <strong>{questionsQuery.data?.total || 0}</strong>
          <span>道题</span>
        </div>
      </div>

      <div className="admin-question-page__toolbar">
        <label className="admin-field">
          <span>关键词</span>
          <input
            value={filters.keyword}
            onChange={(event) =>
              setFilters((current) => ({
                ...current,
                keyword: event.target.value,
                page: 1,
              }))
            }
            placeholder="标题或内容关键词"
          />
        </label>

        <label className="admin-field">
          <span>难度</span>
          <select
            value={filters.difficulty}
            onChange={(event) =>
              setFilters((current) => ({
                ...current,
                difficulty: event.target.value,
                page: 1,
              }))
            }
          >
            <option value="">全部难度</option>
            {QUESTION_DIFFICULTY_OPTIONS.map((option) => (
              <option key={option.value} value={option.value}>
                {option.label}
              </option>
            ))}
          </select>
        </label>

        <label className="admin-field">
          <span>分类</span>
          <select
            value={filters.categoryId}
            onChange={(event) =>
              setFilters((current) => ({
                ...current,
                categoryId: event.target.value,
                page: 1,
              }))
            }
          >
            <option value="">全部分类</option>
            {filterCategoryOptions.map((option) => (
              <option key={option.id} value={option.id}>
                {option.label}
              </option>
            ))}
          </select>
        </label>

        <button className="admin-link" type="button" onClick={startCreatingQuestion}>
          新建题目
        </button>
      </div>

      <form className="admin-question-import" onSubmit={handleImport}>
        <div className="admin-question-import__head">
          <div>
            <strong>批量导入题目</strong>
            <p>这里直接粘贴 JSON 数组，结构与 `/api/admin/questions/import` 保持一致。</p>
          </div>
          <label className="admin-field">
            <span>目标行业</span>
            <select value={importIndustryCode} onChange={(event) => setImportIndustryCode(event.target.value)}>
              <option value="">请选择行业</option>
              {(industriesQuery.data || []).map((industry) => (
                <option key={industry.code} value={industry.code}>
                  {industry.name}
                </option>
              ))}
            </select>
          </label>
        </div>

        <textarea
          className="admin-question-import__editor"
          value={importText}
          onChange={(event) => setImportText(event.target.value)}
        />

        <div className={`admin-question-import__status ${importPreview.valid ? 'is-valid' : 'is-error'}`}>
          <span>
            {importPreview.valid
              ? `当前已识别 ${importPreview.count} 条待导入题目。`
              : importPreview.error}
          </span>
          <button
            className="admin-link"
            type="submit"
            disabled={!importPreview.valid || !importIndustryCode || importMutation.isPending}
          >
            {importMutation.isPending ? '导入中...' : '开始批量导入'}
          </button>
        </div>
      </form>

      <div className="admin-question-page__layout">
        <div className="admin-question-list-wrap">
          <div className="admin-question-list">
            {(questionsQuery.data?.list || []).length === 0 ? (
              <div className="admin-question-card admin-question-card--empty">
                <strong>当前筛选条件下没有题目</strong>
                <p>可以先调整筛选条件，或者直接在右侧创建新题目。</p>
              </div>
            ) : (
              (questionsQuery.data?.list || []).map((question) => (
                <button
                  key={question.id}
                  type="button"
                  className={`admin-question-card ${
                    selectedQuestionId === question.id ? 'admin-question-card--active' : ''
                  }`}
                  onClick={() => startEditingQuestion(question)}
                >
                  <div className="admin-question-card__head">
                    <strong>{question.title}</strong>
                    <span>{question.is_active ? '启用中' : '已停用'}</span>
                  </div>
                  <div className="admin-question-card__meta">
                    <span>{questionTypeLabel(question.type)}</span>
                    <span>{questionDifficultyLabel(question.difficulty)}</span>
                    <span>{question.category_name}</span>
                  </div>
                  <p>{summarizeQuestionContent(question.content)}</p>
                  {question.tags.length > 0 ? (
                    <div className="admin-question-card__tags">
                      {question.tags.map((tag) => (
                        <span key={tag}>{tag}</span>
                      ))}
                    </div>
                  ) : null}
                </button>
              ))
            )}
          </div>

          <div className="admin-question-pagination">
            <button
              className="admin-link"
              type="button"
              disabled={filters.page <= 1}
              onClick={() => setFilters((current) => ({ ...current, page: Math.max(1, current.page - 1) }))}
            >
              上一页
            </button>
            <span>
              第 {questionsQuery.data?.page || filters.page} / {totalPages} 页
            </span>
            <button
              className="admin-link"
              type="button"
              disabled={filters.page >= totalPages}
              onClick={() => setFilters((current) => ({ ...current, page: Math.min(totalPages, current.page + 1) }))}
            >
              下一页
            </button>
          </div>
        </div>

        <form className="admin-question-editor" onSubmit={handleSubmit}>
          <div className="admin-question-editor__head">
            <div>
              <h3>{selectedQuestionId ? '编辑题目' : '新建题目'}</h3>
              <p>{message}</p>
            </div>
            <span className="admin-tag">{selectedQuestionId ? `ID #${selectedQuestionId}` : '新题目'}</span>
          </div>

          <div className="admin-question-editor__grid">
            <label className="admin-field">
              <span>所属行业</span>
              <select
                value={form.industryId}
                onChange={(event) => updateQuestionField('industryId', event.target.value)}
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
              <span>分类</span>
              <select
                value={form.categoryId}
                onChange={(event) => updateQuestionField('categoryId', event.target.value)}
              >
                <option value="">请选择分类</option>
                {formCategoryOptions.map((option) => (
                  <option key={option.id} value={option.id}>
                    {option.label}
                  </option>
                ))}
              </select>
            </label>
          </div>

          <div className="admin-question-editor__grid admin-question-editor__grid--triple">
            <label className="admin-field">
              <span>题型</span>
              <select
                value={form.type}
                onChange={(event) => updateQuestionField('type', event.target.value as QuestionType)}
              >
                {QUESTION_TYPE_OPTIONS.map((option) => (
                  <option key={option.value} value={option.value}>
                    {option.label}
                  </option>
                ))}
              </select>
            </label>

            <label className="admin-field">
              <span>难度</span>
              <select
                value={form.difficulty}
                onChange={(event) => updateQuestionField('difficulty', event.target.value as QuestionDifficulty)}
              >
                {QUESTION_DIFFICULTY_OPTIONS.map((option) => (
                  <option key={option.value} value={option.value}>
                    {option.label}
                  </option>
                ))}
              </select>
            </label>

            <label className="admin-field">
              <span>标签</span>
              <input
                value={form.tagsText}
                onChange={(event) => updateQuestionField('tagsText', event.target.value)}
                placeholder="并发, channel, context"
              />
            </label>
          </div>

          <label className="admin-field">
            <span>题目标题</span>
            <input
              value={form.title}
              onChange={(event) => updateQuestionField('title', event.target.value)}
              placeholder="请输入题目标题"
            />
          </label>

          <label className="admin-field">
            <span>题目内容</span>
            <textarea
              className="admin-question-editor__content"
              value={form.content}
              onChange={(event) => updateQuestionField('content', event.target.value)}
              placeholder="请输入完整题干内容"
            />
          </label>

          {requiresQuestionOptions(form.type) ? (
            <label className="admin-field">
              <span>选项列表</span>
              <textarea
                className="admin-question-editor__options"
                value={form.optionsText}
                onChange={(event) => updateQuestionField('optionsText', event.target.value)}
                placeholder={'每行一个选项，例如：\n选项 A\n选项 B'}
              />
            </label>
          ) : null}

          <label className="admin-field">
            <span>答案</span>
            <textarea
              className="admin-question-editor__answer"
              value={form.answer}
              onChange={(event) => updateQuestionField('answer', event.target.value)}
              placeholder="请输入标准答案"
            />
          </label>

          <label className="admin-field">
            <span>解析</span>
            <textarea
              className="admin-question-editor__answer"
              value={form.explanation}
              onChange={(event) => updateQuestionField('explanation', event.target.value)}
              placeholder="请输入题目解析"
            />
          </label>

          {form.type === 'code' ? (
            <div className="admin-question-import">
              <div className="admin-question-import__head">
                <div>
                  <strong>编程题结构化解析</strong>
                  <p>这里维护 P0 阶段要求的统一解析结构，前台会按这个结构直接展示。</p>
                </div>
              </div>

              <label className="admin-field">
                <span>题意总结</span>
                <textarea
                  className="admin-question-editor__answer"
                  value={form.solutionSummary}
                  onChange={(event) => updateQuestionField('solutionSummary', event.target.value)}
                  placeholder="一句话说明这道题核心在考什么"
                />
              </label>

              <label className="admin-field">
                <span>解题思路</span>
                <textarea
                  className="admin-question-editor__answer"
                  value={form.solutionApproach}
                  onChange={(event) => updateQuestionField('solutionApproach', event.target.value)}
                  placeholder="说明为什么采用这套解法，以及关键策略是什么"
                />
              </label>

              <label className="admin-field">
                <span>关键步骤</span>
                <textarea
                  className="admin-question-editor__answer"
                  value={form.solutionStepsText}
                  onChange={(event) => updateQuestionField('solutionStepsText', event.target.value)}
                  placeholder={'每行一条，例如：\n确定状态定义\n初始化边界\n按转移关系推进'}
                />
              </label>

              <label className="admin-field">
                <span>边界条件</span>
                <textarea
                  className="admin-question-editor__answer"
                  value={form.solutionEdgeCasesText}
                  onChange={(event) => updateQuestionField('solutionEdgeCasesText', event.target.value)}
                  placeholder={'每行一条，例如：\n空输入\n长度为 1\n重复元素'}
                />
              </label>

              <label className="admin-field">
                <span>复杂度分析</span>
                <textarea
                  className="admin-question-editor__answer"
                  value={form.solutionComplexity}
                  onChange={(event) => updateQuestionField('solutionComplexity', event.target.value)}
                  placeholder="例如：时间复杂度 O(n)，空间复杂度 O(1)"
                />
              </label>

              <label className="admin-field">
                <span>常见错法</span>
                <textarea
                  className="admin-question-editor__answer"
                  value={form.solutionMistakesText}
                  onChange={(event) => updateQuestionField('solutionMistakesText', event.target.value)}
                  placeholder={'每行一条，例如：\n漏掉边界判断\n索引越界\n复杂度分析缺失'}
                />
              </label>

              <div className="admin-question-import__head">
                <div>
                  <strong>编程题判题配置</strong>
                  <p>测试用例模式下将使用公开样例运行、隐藏用例提交判题；AI 只负责讲解，不负责最终裁决。</p>
                </div>
              </div>

              <label className="admin-field">
                <span>判题模式</span>
                <select
                  value={form.evaluationMode}
                  onChange={(event) => updateQuestionField('evaluationMode', event.target.value as 'analysis_only' | 'testcase')}
                >
                  <option value="analysis_only">AI 分析模式</option>
                  <option value="testcase">测试用例判题模式</option>
                </select>
              </label>

              <div className="admin-question-editor__grid admin-question-editor__grid--triple">
                <label className="admin-field">
                  <span>默认语言</span>
                  <input
                    value={form.defaultLanguage}
                    onChange={(event) => updateQuestionField('defaultLanguage', event.target.value)}
                    placeholder="go"
                  />
                </label>

                <label className="admin-field">
                  <span>允许语言</span>
                  <input
                    value={form.allowedLanguagesText}
                    onChange={(event) => updateQuestionField('allowedLanguagesText', event.target.value)}
                    placeholder="go, python, javascript"
                  />
                </label>

                <label className="admin-field">
                  <span>起始模板代码</span>
                  <input
                    value={form.starterCode}
                    onChange={(event) => updateQuestionField('starterCode', event.target.value)}
                    placeholder="可选，用于初始化编辑器内容"
                  />
                </label>
              </div>

              <div className="admin-question-editor__grid admin-question-editor__grid--triple">
                <label className="admin-field">
                  <span>时间限制(ms)</span>
                  <input
                    value={form.timeLimitMs}
                    onChange={(event) => updateQuestionField('timeLimitMs', event.target.value)}
                    placeholder="2000"
                  />
                </label>

                <label className="admin-field">
                  <span>内存限制(MB)</span>
                  <input
                    value={form.memoryLimitMb}
                    onChange={(event) => updateQuestionField('memoryLimitMb', event.target.value)}
                    placeholder="128"
                  />
                </label>
              </div>

              <label className="admin-field">
                <span>公开样例 JSON</span>
                <textarea
                  className="admin-question-editor__content"
                  value={form.publicCasesText}
                  onChange={(event) => updateQuestionField('publicCasesText', event.target.value)}
                  placeholder={'[\n  {\n    "input": "3\\n1 2 3",\n    "expected_output": "6",\n    "description": "基础样例"\n  }\n]'}
                />
              </label>

              <label className="admin-field">
                <span>隐藏用例 JSON</span>
                <textarea
                  className="admin-question-editor__content"
                  value={form.hiddenCasesText}
                  onChange={(event) => updateQuestionField('hiddenCasesText', event.target.value)}
                  placeholder={'[\n  {\n    "input": "0",\n    "expected_output": "0",\n    "description": "边界场景"\n  }\n]'}
                />
              </label>

              <label className="admin-field">
                <span>参考实现 JSON</span>
                <textarea
                  className="admin-question-editor__content"
                  value={form.referenceSolutionsText}
                  onChange={(event) => updateQuestionField('referenceSolutionsText', event.target.value)}
                  placeholder={'[\n  {\n    "language": "go",\n    "title": "Go 参考实现",\n    "code": "package main\\n\\nfunc main() {}"\n  }\n]'}
                />
              </label>
            </div>
          ) : null}

          {form.type === 'subjective' ? (
            <div className="admin-question-import">
              <div className="admin-question-import__head">
                <div>
                  <strong>主观题参考回答模板</strong>
                  <p>这里维护面试化回答模板，前台会按“结论 + 展开 + 追问”结构展示。</p>
                </div>
              </div>

              <label className="admin-field">
                <span>核心结论</span>
                <textarea
                  className="admin-question-editor__answer"
                  value={form.answerTemplateConclusion}
                  onChange={(event) => updateQuestionField('answerTemplateConclusion', event.target.value)}
                  placeholder="先给出这道题最关键的结论"
                />
              </label>

              <label className="admin-field">
                <span>关键展开点</span>
                <textarea
                  className="admin-question-editor__answer"
                  value={form.answerTemplateKeyPointsText}
                  onChange={(event) => updateQuestionField('answerTemplateKeyPointsText', event.target.value)}
                  placeholder={'每行一条，例如：\n解释原理\n说明适用场景\n补充优缺点'}
                />
              </label>

              <label className="admin-field">
                <span>面试表达示例</span>
                <textarea
                  className="admin-question-editor__content"
                  value={form.answerTemplateSampleAnswer}
                  onChange={(event) => updateQuestionField('answerTemplateSampleAnswer', event.target.value)}
                  placeholder="写一版更接近真实面试表达的完整回答"
                />
              </label>

              <label className="admin-field">
                <span>高频追问点</span>
                <textarea
                  className="admin-question-editor__answer"
                  value={form.answerTemplateFollowUpsText}
                  onChange={(event) => updateQuestionField('answerTemplateFollowUpsText', event.target.value)}
                  placeholder={'每行一条，例如：\n为什么这样设计？\n边界是什么？\n替代方案是什么？'}
                />
              </label>

              <label className="admin-field">
                <span>易答偏点</span>
                <textarea
                  className="admin-question-editor__answer"
                  value={form.answerTemplatePitfallsText}
                  onChange={(event) => updateQuestionField('answerTemplatePitfallsText', event.target.value)}
                  placeholder={'每行一条，例如：\n只背定义\n不讲场景\n忽略权衡'}
                />
              </label>
            </div>
          ) : null}

          {questionTagTaxonomyQuery.data?.length ? (
            <div className="admin-question-import">
              <div className="admin-question-import__head">
                <div>
                  <strong>标准标签建议</strong>
                  <p>P0 阶段优先复用这套标签，避免同义词、英文大小写和临时口径继续扩散。</p>
                </div>
              </div>

              {questionTagTaxonomyQuery.data.map((group) => (
                <div key={group.group} style={{ marginBottom: 16 }}>
                  <strong>{group.group}</strong>
                  <p style={{ margin: '6px 0 10px' }}>{group.description}</p>
                  <div className="admin-question-card__tags">
                    {group.tags.map((tag) => (
                      <button
                        key={`${group.group}-${tag}`}
                        type="button"
                        className="admin-link"
                        style={{ marginRight: 8, marginBottom: 8 }}
                        onClick={() => appendQuestionTag(tag)}
                      >
                        {tag}
                      </button>
                    ))}
                  </div>
                </div>
              ))}
            </div>
          ) : null}

          <div className={`admin-question-editor__status ${formError ? 'is-error' : 'is-valid'}`}>
            <strong>表单检查</strong>
            <span>{formError || '当前题目表单已通过基础校验，可以提交保存。'}</span>
          </div>

          <label className="admin-question-editor__switch">
            <input
              type="checkbox"
              checked={form.isActive}
              onChange={(event) => updateQuestionField('isActive', event.target.checked)}
            />
            <span>{form.isActive ? '当前题目启用中' : '当前题目已停用'}</span>
          </label>

          <div className="admin-question-editor__actions">
            <button
              className="admin-link"
              type="button"
              onClick={startCreatingQuestion}
              disabled={saveMutation.isPending || deleteMutation.isPending}
            >
              重置为新建
            </button>
            {selectedQuestionId ? (
              <button
                className="admin-link"
                type="button"
                onClick={handleDelete}
                disabled={saveMutation.isPending || deleteMutation.isPending}
              >
                {deleteMutation.isPending ? '删除中...' : '删除题目'}
              </button>
            ) : null}
            <button
              className="admin-link"
              type="submit"
              disabled={Boolean(formError) || saveMutation.isPending || deleteMutation.isPending}
            >
              {saveMutation.isPending ? '保存中...' : selectedQuestionId ? '保存修改' : '创建题目'}
            </button>
          </div>
        </form>
      </div>
    </section>
  )
}
