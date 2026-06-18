package biz

import "strings"

// standardQuestionTagAlias 标签别名映射，将用户输入或爬取的非标准标签收敛为标准标签
var standardQuestionTagAlias = map[string]string{
	"go":         "Go基础",
	"golang":     "Go基础",
	"基础概念":       "Go基础",
	"grpc":       "gRPC",
	"rest":       "REST",
	"jwt":        "JWT",
	"gorm":       "GORM",
	"gin":        "Gin",
	"preload":    "Preload",
	"joins":      "Joins",
	"context":    "context",
	"channel":    "channel",
	"goroutine":  "goroutine",
	"mutex":      "Mutex",
	"rwmutex":    "RWMutex",
	"atomic":     "atomic",
	"skiplist":   "跳表",
	"workerpool": "WorkerPool",
	"ioc":        "IoC",
	"di":         "DI",
	"http/2":     "HTTP/2",
	"http/3":     "HTTP/3",
	"quic":       "QUIC",
	"sql注入":      "SQL注入",
}

// NormalizeQuestionTagStrings 将原始标签数组收敛为标准标签集合
func NormalizeQuestionTagStrings(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		tag := strings.TrimSpace(value)
		if tag == "" {
			continue
		}
		lowered := strings.ToLower(tag)
		if mapped, ok := standardQuestionTagAlias[lowered]; ok {
			tag = mapped
		}
		if _, exists := seen[tag]; exists {
			continue
		}
		seen[tag] = struct{}{}
		result = append(result, tag)
	}
	return result
}

// ParseQuestionTagsFromStorage 解析并标准化数据库中的题目标签
func ParseQuestionTagsFromStorage(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return []string{}
	}
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == '，'
	})
	return NormalizeQuestionTagStrings(parts)
}
