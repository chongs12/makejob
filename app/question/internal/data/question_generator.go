package data

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	aiv1 "makejob/api/makejob/ai/v1"
	"makejob/app/question/internal/biz"
	"makejob/app/question/internal/conf"
)

// questionGeneratorClient 实现 biz.QuestionGeneratorClient 接口
// 复用 AI Gateway 的 InterviewAgent RPC 生成独立题目
type questionGeneratorClient struct {
	client aiv1.AIServiceClient
	conn   *grpc.ClientConn
}

// NewQuestionGeneratorClient 创建题目生成客户端
func NewQuestionGeneratorClient(cfg *conf.AI) (biz.QuestionGeneratorClient, error) {
	conn, err := grpc.Dial(cfg.AIGatewayAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to dial AI Gateway at %s: %w", cfg.AIGatewayAddr, err)
	}
	return &questionGeneratorClient{
		client: aiv1.NewAIServiceClient(conn),
		conn:   conn,
	}, nil
}

func (c *questionGeneratorClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// generatedQuestion AI 返回的单个题目结构
type generatedQuestion struct {
	Title           string   `json:"title"`
	Content         string   `json:"content"`
	Difficulty      string   `json:"difficulty"`
	Type            string   `json:"type"`
	Topic           string   `json:"topic"`
	ReferenceAnswer string   `json:"reference_answer"`
	Explanation     string   `json:"explanation"`
	Tags            []string `json:"tags"`
}

// GenerateQuestions 调用 AI Gateway 生成题目
func (c *questionGeneratorClient) GenerateQuestions(ctx context.Context, req *biz.GenerateQuestionsRequest) ([]*biz.Question, error) {
	count := req.Count
	if count <= 0 {
		count = 5
	}
	topicsHint := ""
	if len(req.Topics) > 0 {
		topicsHint = fmt.Sprintf("重点考察以下知识点：%s", strings.Join(req.Topics, "、"))
	}

	// 构造 InterviewAgent 请求：用 history 中的 system message 指定生成任务
	systemPrompt := fmt.Sprintf(
		"你是一个专业的题库出题助手。请为「%s」行业生成 %d 道%s难度的面试/笔试题目。%s\n"+
			"请严格以 JSON 数组格式返回，每道题包含以下字段：\n"+
			"title(题目标题), content(题目正文), difficulty(easy/medium/hard), "+
			"type(coding/subjective/multiple_choice), topic(知识点), "+
			"reference_answer(参考答案), explanation(解析), tags(标签数组)\n"+
			"只返回 JSON 数组，不要包含其他文字。",
		req.IndustryCode, count, req.Difficulty, topicsHint,
	)

	resp, err := c.client.InterviewAgent(ctx, &aiv1.InterviewAgentRequest{
		IndustryCode: req.IndustryCode,
		Difficulty:   req.Difficulty,
		History: []*aiv1.Message{
			{Role: "system", Content: systemPrompt},
		},
		QuestionIndex: 0,
	})
	if err != nil {
		return nil, fmt.Errorf("AI InterviewAgent call failed: %w", err)
	}

	// 解析 AI 返回的题目 JSON
	questions, err := parseGeneratedQuestions(resp.GetQuestion(), req.IndustryCode)
	if err != nil {
		return nil, fmt.Errorf("failed to parse AI response: %w", err)
	}

	return questions, nil
}

// parseGeneratedQuestions 从 AI 响应中解析题目列表
func parseGeneratedQuestions(raw string, industryCode string) ([]*biz.Question, error) {
	// 尝试提取 JSON 数组（AI 可能在 JSON 前后添加说明文字）
	jsonStr := extractJSONArray(raw)
	if jsonStr == "" {
		return nil, fmt.Errorf("AI response does not contain valid JSON array: %s", raw[:min(len(raw), 200)])
	}

	var generated []generatedQuestion
	if err := json.Unmarshal([]byte(jsonStr), &generated); err != nil {
		return nil, fmt.Errorf("failed to unmarshal questions JSON: %w", err)
	}

	questions := make([]*biz.Question, 0, len(generated))
	for _, g := range generated {
		if g.Title == "" || g.Content == "" {
			continue // 跳过无效题目
		}
		q := &biz.Question{
			Title:           g.Title,
			Content:         g.Content,
			Difficulty:      g.Difficulty,
			Type:            g.Type,
			IndustryCode:    industryCode,
			ReferenceAnswer: g.ReferenceAnswer,
			Explanation:     g.Explanation,
		}
		if len(g.Tags) > 0 {
			q.Tags = g.Tags
		}
		if q.Difficulty == "" {
			q.Difficulty = "medium"
		}
		if q.Type == "" {
			q.Type = "subjective"
		}
		questions = append(questions, q)
	}

	return questions, nil
}

// extractJSONArray 从文本中提取第一个 JSON 数组
func extractJSONArray(s string) string {
	start := strings.Index(s, "[")
	if start == -1 {
		return ""
	}
	end := strings.LastIndex(s, "]")
	if end == -1 || end <= start {
		return ""
	}
	return s[start : end+1]
}
