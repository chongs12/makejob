package biz

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"

	"github.com/go-kratos/kratos/v2/log"
)

const (
	// maxExternalRAGLen 火山引擎 external_rag 最大字符长度
	maxExternalRAGLen = 4000
	// defaultInterviewRetrieveTopK 面试场景默认检索数量
	defaultInterviewRetrieveTopK = 3
)

// InterviewRAGService 面试场景 RAG 服务（对齐单体 InterviewRAGService）
type InterviewRAGService struct {
	retrieveUC *RetrieveUseCase
	logger     *log.Helper
}

// NewInterviewRAGService 创建面试 RAG 服务
func NewInterviewRAGService(retrieveUC *RetrieveUseCase, logger log.Logger) *InterviewRAGService {
	return &InterviewRAGService{
		retrieveUC: retrieveUC,
		logger:     log.NewHelper(logger),
	}
}

// InterviewRAGResult 面试 RAG 检索结果
type InterviewRAGResult struct {
	Documents []Document
	Query     string
}

// RetrieveForInterview 根据面试上下文检索相关知识（对齐单体 InterviewRAGService.RetrieveForInterview）
func (s *InterviewRAGService) RetrieveForInterview(
	ctx context.Context,
	query string,
	industry string,
	currentTopic string,
	skills []string,
) (*InterviewRAGResult, error) {
	if s.retrieveUC == nil {
		return nil, fmt.Errorf("RAG服务未初始化")
	}

	enhancedQuery := s.buildEnhancedQuery(query, industry, currentTopic, skills)

	docs, err := s.retrieveUC.Retrieve(ctx, enhancedQuery, defaultInterviewRetrieveTopK, nil)
	if err != nil {
		if err == ErrNoResults {
			return nil, nil
		}
		return nil, fmt.Errorf("RAG检索失败: %w", err)
	}

	if len(docs) == 0 {
		return nil, nil
	}

	return &InterviewRAGResult{
		Documents: docs,
		Query:     enhancedQuery,
	}, nil
}

// buildEnhancedQuery 构建增强查询（对齐单体 InterviewRAGService.buildEnhancedQuery）
func (s *InterviewRAGService) buildEnhancedQuery(
	query string,
	industry string,
	currentTopic string,
	skills []string,
) string {
	var parts []string

	if currentTopic != "" {
		parts = append(parts, currentTopic)
	}
	if query != "" {
		parts = append(parts, query)
	}
	if len(skills) > 0 {
		limit := 3
		if len(skills) < limit {
			limit = len(skills)
		}
		parts = append(parts, strings.Join(skills[:limit], " "))
	}

	return strings.Join(parts, " ")
}

// FormatForExternalRAG 格式化为火山引擎 external_rag 格式（对齐单体 InterviewRAGService.FormatForExternalRAG）
func (s *InterviewRAGService) FormatForExternalRAG(docs []Document, maxLen int) string {
	if maxLen <= 0 {
		maxLen = maxExternalRAGLen
	}

	type ragItem struct {
		Title   string `json:"title"`
		Content string `json:"content"`
	}

	var items []ragItem
	currentLen := 2 // "[]"

	for _, doc := range docs {
		content := doc.Content
		if len(content) > 500 {
			content = content[:500] + "..."
		}

		item := ragItem{Title: doc.ID, Content: content}
		itemJSON, err := json.Marshal(item)
		if err != nil {
			continue
		}

		if currentLen+len(itemJSON)+1 > maxLen {
			break
		}

		items = append(items, item)
		currentLen += len(itemJSON) + 1
	}

	if len(items) == 0 {
		return "[]"
	}

	result, err := json.Marshal(items)
	if err != nil {
		return "[]"
	}
	return string(result)
}

// GetSoothingPhrase 获取随机安抚话术（对齐单体 InterviewRAGService.GetSoothingPhrase）
func (s *InterviewRAGService) GetSoothingPhrase() string {
	phrases := []string{
		"好的，让我想想。",
		"嗯，这个问题很好。",
		"我来分析一下你的回答。",
		"让我整理一下思路。",
		"好的，我来想想怎么回答。",
	}
	return phrases[rand.Intn(len(phrases))]
}

// EnhanceQuestionPrompt 增强出题提示词（对齐单体 InterviewRAGService.EnhanceQuestionPrompt）
func (s *InterviewRAGService) EnhanceQuestionPrompt(
	ctx context.Context,
	originalPrompt string,
	topic string,
	industry string,
	skills []string,
) string {
	if s.retrieveUC == nil {
		return originalPrompt
	}

	query := topic
	if industry != "" {
		query = industry + " " + query
	}

	docs, err := s.retrieveUC.Retrieve(ctx, query, 3, nil)
	if err != nil {
		return originalPrompt
	}
	if len(docs) == 0 {
		return originalPrompt
	}

	var refs []string
	for _, doc := range docs {
		content := doc.Content
		if len(content) > 200 {
			content = content[:200] + "..."
		}
		refs = append(refs, fmt.Sprintf("- %s: %s", doc.ID, content))
	}

	enhanced := originalPrompt + "\n\n## 参考知识（用于出题参考）:\n"
	enhanced += strings.Join(refs, "\n")
	enhanced += "\n\n要求：结合上述参考知识，出更有针对性的面试题。"

	return enhanced
}

// EnhanceFeedbackPrompt 增强评估提示词（对齐单体 InterviewRAGService.EnhanceFeedbackPrompt）
func (s *InterviewRAGService) EnhanceFeedbackPrompt(
	ctx context.Context,
	originalPrompt string,
	question string,
	answer string,
) string {
	if s.retrieveUC == nil {
		return originalPrompt
	}

	query := question + " " + answer
	if len(query) > 200 {
		query = query[:200]
	}

	docs, err := s.retrieveUC.Retrieve(ctx, query, 2, nil)
	if err != nil {
		return originalPrompt
	}
	if len(docs) == 0 {
		return originalPrompt
	}

	var refs []string
	for _, doc := range docs {
		content := doc.Content
		if len(content) > 150 {
			content = content[:150] + "..."
		}
		refs = append(refs, fmt.Sprintf("- %s", content))
	}

	enhanced := originalPrompt + "\n\n## 参考知识（用于评估参考）:\n"
	enhanced += strings.Join(refs, "\n")

	return enhanced
}
