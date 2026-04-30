import { describe, expect, it } from 'vitest'
import { filterPracticeCollectionQuestions } from './practiceCollectionMode'

describe('practiceCollectionMode', () => {
  it('filters formal collection questions by difficulty and keyword locally', () => {
    expect(filterPracticeCollectionQuestions([
      { id: 1, title: 'Go slice 扩容分析', type: 'code', difficulty: 'medium' },
      { id: 2, title: 'channel 阻塞排查', type: 'code', difficulty: 'hard' },
      { id: 3, title: '并发基础判断题', type: 'choice', difficulty: 'easy' },
    ], {
      keyword: 'channel',
      difficulty: 'hard',
    })).toEqual([
      { id: 2, title: 'channel 阻塞排查', type: 'code', difficulty: 'hard' },
    ])
  })

  it('returns the original formal collection when no filter is applied', () => {
    const questions = [
      { id: 1, title: 'Go slice 扩容分析', type: 'code', difficulty: 'medium' },
      { id: 2, title: 'channel 阻塞排查', type: 'code', difficulty: 'hard' },
    ]

    expect(filterPracticeCollectionQuestions(questions, {
      keyword: '',
      difficulty: '',
    })).toEqual(questions)
  })
})
