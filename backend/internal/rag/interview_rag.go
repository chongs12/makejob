package rag

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"

	applogger "makejob-backend/pkg/logger"

	"go.uber.org/zap"
)

const (
	// maxExternalRAGLen 火山引擎external_rag最大字符长度
	maxExternalRAGLen = 4000

	// defaultRetrieveTopK 默认检索数量
	defaultRetrieveTopK = 3
)

// InterviewRAGService 面试场景RAG服务
type InterviewRAGService struct {
	service *Service
}

// NewInterviewRAGService 创建面试RAG服务
func NewInterviewRAGService(service *Service) *InterviewRAGService {
	return &InterviewRAGService{service: service}
}

// InterviewRAGResult 面试RAG检索结果
type InterviewRAGResult struct {
	Documents []Document // 检索到的文档
	Query     string     // 实际查询文本
}

// RetrieveForInterview 根据面试上下文检索相关知识
func (s *InterviewRAGService) RetrieveForInterview(
	ctx context.Context,
	query string,
	industry string,
	currentTopic string,
	skills []string,
) (*InterviewRAGResult, error) {
	if s.service == nil {
		return nil, fmt.Errorf("RAG服务未初始化")
	}

	// 构建增强查询
	enhancedQuery := s.buildEnhancedQuery(query, industry, currentTopic, skills)

	// 语义检索
	docs, err := s.service.RetrieveByQuery(ctx, enhancedQuery, defaultRetrieveTopK)
	if err != nil {
		return nil, fmt.Errorf("RAG检索失败: %w", err)
	}

	if len(docs) == 0 {
		return nil, nil
	}

	applogger.Debug("面试RAG检索完成",
		zap.String("query", enhancedQuery),
		zap.Int("results", len(docs)),
	)

	return &InterviewRAGResult{
		Documents: docs,
		Query:     enhancedQuery,
	}, nil
}

// buildEnhancedQuery 构建增强查询
func (s *InterviewRAGService) buildEnhancedQuery(
	query string,
	industry string,
	currentTopic string,
	skills []string,
) string {
	var parts []string

	// 添加当前话题
	if currentTopic != "" {
		parts = append(parts, currentTopic)
	}

	// 添加用户回答
	if query != "" {
		parts = append(parts, query)
	}

	// 添加技术栈关键词（取前3个）
	if len(skills) > 0 {
		limit := 3
		if len(skills) < limit {
			limit = len(skills)
		}
		parts = append(parts, strings.Join(skills[:limit], " "))
	}

	return strings.Join(parts, " ")
}

// FormatForExternalRAG 格式化为火山引擎external_rag格式
// 返回: [{"title":"...","content":"..."}]
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

// GetSoothingPhrase 获取随机安抚话术
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

// EnhanceQuestionPrompt 增强出题提示词（实现ai.PromptEnhancer接口）
func (s *InterviewRAGService) EnhanceQuestionPrompt(
	ctx context.Context,
	originalPrompt string,
	topic string,
	industry string,
	skills []string,
) string {
	if s.service == nil {
		return originalPrompt
	}

	// 构建检索查询
	query := topic
	if industry != "" {
		query = industry + " " + query
	}

	// 语义检索
	docs, err := s.service.RetrieveByQuery(ctx, query, 3)
	if err != nil {
		applogger.Warn("RAG出题增强检索失败", zap.Error(err), zap.String("query", query))
		return originalPrompt
	}
	if len(docs) == 0 {
		return originalPrompt
	}

	// 构建参考知识
	var refs []string
	for _, doc := range docs {
		content := doc.Content
		if len(content) > 200 {
			content = content[:200] + "..."
		}
		refs = append(refs, fmt.Sprintf("- %s: %s", doc.ID, content))
	}

	// 增强提示词
	enhanced := originalPrompt + "\n\n## 参考知识（用于出题参考）:\n"
	enhanced += strings.Join(refs, "\n")
	enhanced += "\n\n要求：结合上述参考知识，出更有针对性的面试题。"

	return enhanced
}

// EnhanceFeedbackPrompt 增强评估提示词（实现ai.PromptEnhancer接口）
func (s *InterviewRAGService) EnhanceFeedbackPrompt(
	ctx context.Context,
	originalPrompt string,
	question string,
	answer string,
) string {
	if s.service == nil {
		return originalPrompt
	}

	// 构建检索查询
	query := question + " " + answer
	if len(query) > 200 {
		query = query[:200]
	}

	// 语义检索
	docs, err := s.service.RetrieveByQuery(ctx, query, 2)
	if err != nil {
		applogger.Warn("RAG评估增强检索失败", zap.Error(err))
		return originalPrompt
	}
	if len(docs) == 0 {
		return originalPrompt
	}

	// 构建参考知识
	var refs []string
	for _, doc := range docs {
		content := doc.Content
		if len(content) > 150 {
			content = content[:150] + "..."
		}
		refs = append(refs, fmt.Sprintf("- %s", content))
	}

	// 增强提示词
	enhanced := originalPrompt + "\n\n## 参考知识（用于评估参考）:\n"
	enhanced += strings.Join(refs, "\n")

	return enhanced
}

