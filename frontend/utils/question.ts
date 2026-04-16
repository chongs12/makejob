type RawOption = string | { label?: string; text?: string }

const QUESTION_TYPE_ALIASES: Record<string, string> = {
  multiple: 'multi',
  coding: 'code',
}

const parseJSONArray = <T>(value: string | null | undefined): T[] => {
  if (!value) return []

  try {
    const parsed = JSON.parse(value)
    return Array.isArray(parsed) ? parsed : []
  } catch {
    return []
  }
}

export const normalizeQuestionType = (type: string | null | undefined) => {
  if (!type) return ''
  return QUESTION_TYPE_ALIASES[type] || type
}

export const normalizeQuestion = <T extends Record<string, any>>(question: T | null | undefined) => {
  if (!question) return question

  const options = Array.isArray(question.options)
    ? question.options
    : parseJSONArray<RawOption>(question.options_json || question.optionsJSON).map(option => {
        if (typeof option === 'string') return option
        return option.text || option.label || ''
      }).filter(Boolean)

  const tags = Array.isArray(question.tags)
    ? question.tags
    : String(question.tags || '')
        .split(',')
        .map((tag) => tag.trim())
        .filter(Boolean)

  return {
    ...question,
    type: normalizeQuestionType(question.type),
    options,
    tags,
    correct_answer: question.correct_answer || question.answer,
    analysis: question.analysis || question.explanation,
  }
}

export const questionTypeLabel = (type: string) => {
  const labels: Record<string, string> = {
    choice: '选择题',
    multi: '多选题',
    code: '编程题',
    subjective: '主观题',
  }
  return labels[normalizeQuestionType(type)] || type || '-'
}
