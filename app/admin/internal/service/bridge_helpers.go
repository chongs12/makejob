package service

import (
	"encoding/json"
	"fmt"
	"strings"

	"google.golang.org/protobuf/types/known/structpb"

	"makejob-backend/bridge"
	adminv1 "makejob/api/makejob/admin/v1"
)

// requireBackendBridge 确保当前 admin 微服务已经完成 backend bridge 装配。
func (s *AdminService) requireBackendBridge() (*bridge.Runtime, error) {
	if s.backendBridge == nil {
		return nil, fmt.Errorf("backend bridge is not initialized")
	}
	return s.backendBridge, nil
}

// buildBridgePipelineRequest 将 gRPC 请求转换为 backend bridge 请求。
func buildBridgePipelineRequest(req *adminv1.GenerateQuestionPipelineRequest) bridge.QuestionPipelineGenerateRequest {
	return bridge.QuestionPipelineGenerateRequest{
		IndustryCode:     req.GetIndustryCode(),
		Requirement:      req.GetRequirement(),
		AgentPrompt:      req.GetAgentPrompt(),
		GenerationMode:   req.GetGenerationMode(),
		CandidateCount:   req.GetCandidateCount(),
		IncludeScraped:   req.GetIncludeScraped(),
		IncludeGenerated: req.GetIncludeGenerated(),
		Sources:          req.GetSources(),
	}
}

// buildPipelineResponse 将 bridge 题目流水线结果转换为 gRPC 响应。
func buildPipelineResponse(resp *bridge.QuestionPipelineGenerateResponse) (*adminv1.GenerateQuestionPipelineResponse, error) {
	stats, err := structpb.NewStruct(resp.Stats)
	if err != nil {
		return nil, fmt.Errorf("marshal pipeline stats failed: %w", err)
	}
	cards := make([]*adminv1.PipelineCard, 0, len(resp.Cards))
	for _, card := range resp.Cards {
		cards = append(cards, &adminv1.PipelineCard{
			Id:          card.ID,
			Title:       card.Title,
			Content:     card.Content,
			Type:        card.Type,
			Difficulty:  card.Difficulty,
			Category:    card.Category,
			Answer:      card.Answer,
			Solution:    card.Solution,
			Explanation: card.Explanation,
			Tags:        card.Tags,
			JudgeConfig: card.JudgeConfig,
			Confidence:  card.Confidence,
			SourceType:  card.SourceType,
			SourceLabel: card.SourceLabel,
			SourceTitle: card.SourceTitle,
			SourceUrl:   card.SourceURL,
		})
	}
	return &adminv1.GenerateQuestionPipelineResponse{
		IndustryCode:   resp.IndustryCode,
		Requirement:    resp.Requirement,
		GenerationMode: resp.GenerationMode,
		Cards:          cards,
		Warnings:       resp.Warnings,
		Stats:          stats,
	}, nil
}

// buildBridgeAIDebugRequest 将 gRPC AI 调试请求转换为 bridge 请求。
func buildBridgeAIDebugRequest(agentType string, prompt string, params map[string]string) bridge.AIDebugRequest {
	return bridge.AIDebugRequest{
		AgentType: agentType,
		Prompt:    prompt,
		Params:    params,
		RunModel:  true,
	}
}

// buildBridgeScraperImportRequest 将 gRPC 爬虫导入请求转换为 bridge 请求。
func buildBridgeScraperImportRequest(req *adminv1.ScraperImportRequest) bridge.ScraperImportRequest {
	items := make([]bridge.ScraperCleanedQuestion, 0, len(req.GetQuestions()))
	for _, question := range req.GetQuestions() {
		items = append(items, bridge.ScraperCleanedQuestion{
			CategoryName: question.GetCategoryName(),
			Type:         question.GetType(),
			Difficulty:   question.GetDifficulty(),
			Title:        question.GetTitle(),
			Content:      question.GetContent(),
			OptionsJSON:  question.GetOptionsJson(),
			Answer:       question.GetAnswer(),
			Explanation:  question.GetExplanation(),
			Tags:         question.GetTags(),
		})
	}
	return bridge.ScraperImportRequest{
		IndustryCode: req.GetIndustryCode(),
		SourceURL:    req.GetSourceUrl(),
		SourceTitle:  req.GetSourceTitle(),
		Questions:    items,
	}
}

// joinNonEmptyTags 统一清洗并拼接标签列表。
func joinNonEmptyTags(tags []string) string {
	filtered := make([]string, 0, len(tags))
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag != "" {
			filtered = append(filtered, tag)
		}
	}
	return strings.Join(filtered, ",")
}

// structFromMap 将 map 元数据安全转换为 protobuf Struct。
func structFromMap(input map[string]any) *structpb.Struct {
	if len(input) == 0 {
		return nil
	}
	value, err := structpb.NewStruct(input)
	if err != nil {
		return nil
	}
	return value
}

// stringifyMetadata 将 JSON 文本兼容地反序列化为 map，用于 RAG 搜索结果透传。
func stringifyMetadata(raw string) map[string]any {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var metadata map[string]any
	if err := json.Unmarshal([]byte(raw), &metadata); err != nil {
		return nil
	}
	return metadata
}

// metadataJSONFromStringMap 将字符串字典稳定序列化为后台持久化所需的 JSON 文本。
func metadataJSONFromStringMap(input map[string]string) (string, error) {
	if len(input) == 0 {
		return "", nil
	}
	data, err := json.Marshal(input)
	if err != nil {
		return "", fmt.Errorf("marshal metadata failed: %w", err)
	}
	return string(data), nil
}
