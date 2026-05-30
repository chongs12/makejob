package rag

import (
	"fmt"
	"strconv"
	"strings"

	"makejob-backend/internal/model"
)

// BuildQuestionDocuments 将题目列表转换为待索引的文档列表。
// Content拼接规则: Title + "\n" + Content（如果有）。
// MetaData: question_id, type, difficulty, tags, category_id, industry_id, answer。
func BuildQuestionDocuments(questions []model.Question) []IndexDocument {
	docs := make([]IndexDocument, 0, len(questions))

	for _, q := range questions {
		content := q.Title
		if q.Content != "" {
			content += "\n" + q.Content
		}

		meta := map[string]any{
			"question_id": q.ID,
			"type":        q.Type,
			"difficulty":  q.Difficulty,
		}
		if len(q.Tags) > 0 {
			meta["tags"] = q.Tags
		}
		if q.CategoryID > 0 {
			meta["category_id"] = q.CategoryID
		}
		if q.IndustryID > 0 {
			meta["industry_id"] = q.IndustryID
		}
		if q.Answer != "" {
			meta["answer"] = q.Answer
		}

		docs = append(docs, IndexDocument{
			ID:       QuestionIDToDocID(q.ID),
			Content:  content,
			MetaData: meta,
		})
	}

	return docs
}

// QuestionIDToDocID 题目ID转文档ID（格式: "q-{id}"）
func QuestionIDToDocID(questionID uint) string {
	return fmt.Sprintf("q-%d", questionID)
}

// DocIDToQuestionID 文档ID转题目ID
func DocIDToQuestionID(docID string) (uint, error) {
	docID = strings.TrimSpace(docID)
	if !strings.HasPrefix(docID, "q-") {
		return 0, fmt.Errorf("无效的文档ID格式: %s", docID)
	}

	idStr := strings.TrimPrefix(docID, "q-")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("解析文档ID失败: %w", err)
	}

	return uint(id), nil
}
