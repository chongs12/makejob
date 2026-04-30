import { describe, expect, it } from 'vitest'
import {
  buildInterviewFollowUpPracticeRouteSearch,
  buildMistakeTopicPracticeRouteSearch,
  buildPracticeRecommendationRouteSearch,
  buildPracticeRouteSearch,
  buildWeeklyFocusPracticeRouteSearch,
} from './practiceRoute'

describe('practiceRoute helpers', () => {
  it('keeps formal question set routes free from keyword fallback', () => {
    expect(buildPracticeRouteSearch({
      questionSetSlug: 'go-runtime-core',
      focusTags: ['数组', '切片'],
      source: 'question_set',
      title: 'Go 运行时基础题单',
    })).toEqual({
      questionSet: 'go-runtime-core',
      focus: '数组,切片',
      source: 'question_set',
      title: 'Go 运行时基础题单',
    })
  })

  it('builds weekly focus routes with linked topic and question set context', () => {
    expect(buildWeeklyFocusPracticeRouteSearch({
      title: '并发阻塞排查',
      reason: '最近多次暴露 channel 阻塞问题。',
      focus_tags: ['channel', '阻塞'],
    }, {
      code: 'channel-blocking',
      related_question_sets: ['go-concurrency-debug'],
    })).toEqual({
      questionSet: 'go-concurrency-debug',
      topic: 'channel-blocking',
      focus: 'channel,阻塞',
      source: 'weekly_focus',
      title: '并发阻塞排查',
      reason: '最近多次暴露 channel 阻塞问题。',
    })
  })

  it('builds mistake topic routes around the formal collection first', () => {
    expect(buildMistakeTopicPracticeRouteSearch({
      code: 'slice-copy-mistake',
      tag: 'slice',
      title: '切片拷贝错因专题',
      problem_pattern: '经常没有处理底层数组共享问题。',
    }, 'go-runtime-core')).toEqual({
      questionSet: 'go-runtime-core',
      topic: 'slice-copy-mistake',
      focus: 'slice',
      source: 'mistake_topic',
      title: '切片拷贝错因专题',
      reason: '经常没有处理底层数组共享问题。',
    })
  })

  it('builds interview follow-up routes with linked topic priority', () => {
    expect(buildInterviewFollowUpPracticeRouteSearch('并发', {
      code: 'channel-blocking',
      tag: 'channel',
      title: '并发阻塞专题',
      problem_pattern: '最近在面试里多次卡在 channel 阻塞分析。',
      related_question_sets: ['go-concurrency-debug'],
    })).toEqual({
      questionSet: 'go-concurrency-debug',
      topic: 'channel-blocking',
      focus: 'channel',
      source: 'interview_follow_up',
      title: '面试后补练',
      reason: '最近在面试里多次卡在 channel 阻塞分析。',
    })
  })

  it('builds recommendation routes around linked topic collections first', () => {
    expect(buildPracticeRecommendationRouteSearch({
      focus_tag: 'slice',
      topic_code: 'slice-copy-mistake',
      reason: '最近多次在切片共享与拷贝边界上出错。',
      question_title: '切片底层数组共享会带来什么问题？',
    }, {
      code: 'slice-copy-mistake',
      tag: 'slice',
      title: '切片拷贝错因专题',
      problem_pattern: '经常忽略底层数组共享带来的副作用。',
      related_question_sets: ['go-runtime-core'],
    })).toEqual({
      questionSet: 'go-runtime-core',
      topic: 'slice-copy-mistake',
      focus: 'slice',
      source: 'practice_recommendation',
      title: '切片底层数组共享会带来什么问题？',
      reason: '最近多次在切片共享与拷贝边界上出错。',
    })
  })

  it('keeps recommendation topic context even when no formal question set is available', () => {
    expect(buildPracticeRecommendationRouteSearch({
      focus_tag: 'channel',
      topic_code: 'channel-blocking',
      reason: '最近多次在 channel 阻塞分析上失分。',
      question_title: '无缓冲 channel 为什么会阻塞？',
    })).toEqual({
      keyword: 'channel',
      topic: 'channel-blocking',
      focus: 'channel',
      source: 'practice_recommendation',
      title: '无缓冲 channel 为什么会阻塞？',
      reason: '最近多次在 channel 阻塞分析上失分。',
    })
  })
})
