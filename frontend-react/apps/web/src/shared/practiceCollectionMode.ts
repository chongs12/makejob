import type { PracticeQuestionSetPreview } from './practiceCatalog'

interface FilterPracticeCollectionQuestionsParams {
  keyword: string
  difficulty: string
}

/**
 * 在正式题单模式下按当前搜索词与难度对题单内题目做本地过滤，避免退化回全量题库搜索。
 */
export function filterPracticeCollectionQuestions(
  questions: PracticeQuestionSetPreview[],
  params: FilterPracticeCollectionQuestionsParams,
): PracticeQuestionSetPreview[] {
  const normalizedKeyword = params.keyword.trim().toLowerCase()
  const normalizedDifficulty = params.difficulty.trim()

  return questions.filter((question) => {
    if (normalizedDifficulty && question.difficulty !== normalizedDifficulty) {
      return false
    }

    if (!normalizedKeyword) {
      return true
    }

    return question.title.toLowerCase().includes(normalizedKeyword)
  })
}
