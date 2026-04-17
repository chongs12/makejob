package runtime

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"

	"makejob-backend/internal/ai"
	"makejob-backend/internal/model"
	"makejob-backend/internal/repository"
)

// aiCallLogRecorder 负责把运行时 AI 调用写入统一日志表。
type aiCallLogRecorder struct {
	repo          repository.AICallLogRepository
	source        string
	scene         string
	provider      string
	runtimeConfig map[string]string
	sceneConfig   map[string]string
}

// runtimeCallLogEntry 描述一次待落库的运行时 AI 调用。
type runtimeCallLogEntry struct {
	TraceID       string
	IndustryID    *uint
	PromptDetails resolvedPromptDetails
	Request       []ai.Message
	UserInput     string
	Model         string
	Output        string
	Err           error
	StartedAt     time.Time
}

// newAICallLogRecorder 创建运行时 AI 调用日志记录器。
func newAICallLogRecorder(
	repo repository.AICallLogRepository,
	source string,
	scene string,
	runtimeConfig map[string]string,
	sceneConfig map[string]string,
) *aiCallLogRecorder {
	if repo == nil {
		return nil
	}

	return &aiCallLogRecorder{
		repo:          repo,
		source:        strings.TrimSpace(source),
		scene:         strings.TrimSpace(scene),
		provider:      strings.TrimSpace(sceneConfig[ai.ConfigKeyProvider]),
		runtimeConfig: cloneStringMap(runtimeConfig),
		sceneConfig:   cloneStringMap(sceneConfig),
	}
}

// Record 写入一条运行时 AI 调用日志。
func (r *aiCallLogRecorder) Record(ctx context.Context, entry runtimeCallLogEntry) {
	if r == nil || r.repo == nil {
		return
	}

	traceID := strings.TrimSpace(entry.TraceID)
	if traceID == "" {
		traceID = uuid.NewString()
	}

	modelError := ""
	if entry.Err != nil {
		modelError = strings.TrimSpace(entry.Err.Error())
	}

	latency := int64(0)
	if !entry.StartedAt.IsZero() {
		latency = time.Since(entry.StartedAt).Milliseconds()
	}

	log := &model.AICallLog{
		TraceID:            traceID,
		Source:             r.source,
		Scene:              r.scene,
		IndustryID:         entry.IndustryID,
		PromptSource:       strings.TrimSpace(entry.PromptDetails.Source),
		SelectedPromptID:   entry.PromptDetails.TemplateID,
		SelectedPromptName: strings.TrimSpace(entry.PromptDetails.TemplateName),
		RenderedPrompt:     strings.TrimSpace(entry.PromptDetails.Prompt),
		RequestMessages:    marshalRuntimeLogJSON(entry.Request),
		RuntimeConfig:      marshalRuntimeLogJSON(r.runtimeConfig),
		SceneConfig:        marshalRuntimeLogJSON(r.sceneConfig),
		Provider:           r.provider,
		Model:              strings.TrimSpace(entry.Model),
		UserInput:          strings.TrimSpace(entry.UserInput),
		ModelOutput:        strings.TrimSpace(entry.Output),
		ModelError:         modelError,
		LatencyMS:          latency,
		IsSuccess:          modelError == "",
	}

	_ = r.repo.Create(ctx, log)
}

// cloneStringMap 复制字符串 map，避免后续修改影响日志快照。
func cloneStringMap(source map[string]string) map[string]string {
	if len(source) == 0 {
		return map[string]string{}
	}

	result := make(map[string]string, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}

// marshalRuntimeLogJSON 将运行时日志字段序列化为 JSON 字符串。
func marshalRuntimeLogJSON(value interface{}) string {
	if value == nil {
		return ""
	}

	data, err := json.Marshal(value)
	if err != nil {
		return ""
	}

	return string(data)
}
