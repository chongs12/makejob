package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	adminv1 "makejob/api/makejob/admin/v1"
	communityv1 "makejob/api/makejob/community/v1"
	companionv1 "makejob/api/makejob/companion/v1"
	growthv1 "makejob/api/makejob/growth/v1"
	interviewv1 "makejob/api/makejob/interview/v1"
	membershipv1 "makejob/api/makejob/membership/v1"
	planv1 "makejob/api/makejob/plan/v1"
	questionv1 "makejob/api/makejob/question/v1"
	sharedv1 "makejob/api/makejob/shared/v1"
	userv1 "makejob/api/makejob/user/v1"
	"makejob/app/gateway/internal/conf"
	"makejob/pkg/auth"
)

// Gateway HTTP → gRPC 代理
type Gateway struct {
	conns            []*grpc.ClientConn
	userClient       userv1.UserServiceClient
	questionClient   questionv1.QuestionServiceClient
	interviewClient  interviewv1.InterviewServiceClient
	membershipClient membershipv1.MembershipServiceClient
	growthClient     growthv1.GrowthServiceClient
	planClient       planv1.PlanServiceClient
	adminClient      adminv1.AdminServiceClient
	companionClient  companionv1.CompanionServiceClient
	communityClient  communityv1.CommunityServiceClient
	realtimeWSAddr   string
	jwtSecret        string
	serviceToken     string
}

// legacyResponse 复刻原单体统一响应结构，供系统健康检查接口继续复用。
type legacyResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

const (
	questionPipelineTaskType = "question_pipeline_build"
)

// questionPipelineGeneratePayload 描述题目流水线生成接口的 JSON 请求体，供同步、异步与直连 SSE 三种入口共用。
type questionPipelineGeneratePayload struct {
	IndustryCode     string   `json:"industry_code"`
	Requirement      string   `json:"requirement"`
	AgentPrompt      string   `json:"agent_prompt"`
	GenerationMode   string   `json:"generation_mode"`
	CandidateCount   int32    `json:"candidate_count"`
	IncludeScraped   bool     `json:"include_scraped"`
	IncludeGenerated bool     `json:"include_generated"`
	Sources          []string `json:"sources"`
}

// toProto 将网关侧题目流水线请求载荷转换为 Admin gRPC 所需的 protobuf 请求。
func (payload questionPipelineGeneratePayload) toProto() *adminv1.GenerateQuestionPipelineRequest {
	return &adminv1.GenerateQuestionPipelineRequest{
		IndustryCode:     payload.IndustryCode,
		Requirement:      payload.Requirement,
		AgentPrompt:      payload.AgentPrompt,
		GenerationMode:   payload.GenerationMode,
		CandidateCount:   payload.CandidateCount,
		IncludeScraped:   payload.IncludeScraped,
		IncludeGenerated: payload.IncludeGenerated,
		Sources:          payload.Sources,
	}
}

// NewGateway 创建网关实例
func NewGateway(cfg *conf.Bootstrap) (*Gateway, error) {
	serviceToken, err := buildGatewayServiceToken(cfg.JWT.Secret)
	if err != nil {
		return nil, err
	}
	gw := &Gateway{jwtSecret: cfg.JWT.Secret, serviceToken: serviceToken}

	type clientSetup struct {
		name  string
		setup func(addr string) error
	}
	setups := []clientSetup{
		{"user", func(addr string) error {
			conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				return err
			}
			gw.conns = append(gw.conns, conn)
			gw.userClient = userv1.NewUserServiceClient(conn)
			return nil
		}},
		{"question", func(addr string) error {
			conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				return err
			}
			gw.conns = append(gw.conns, conn)
			gw.questionClient = questionv1.NewQuestionServiceClient(conn)
			return nil
		}},
		{"interview", func(addr string) error {
			conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				return err
			}
			gw.conns = append(gw.conns, conn)
			gw.interviewClient = interviewv1.NewInterviewServiceClient(conn)
			return nil
		}},
		{"membership", func(addr string) error {
			conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				return err
			}
			gw.conns = append(gw.conns, conn)
			gw.membershipClient = membershipv1.NewMembershipServiceClient(conn)
			return nil
		}},
		{"growth", func(addr string) error {
			conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				return err
			}
			gw.conns = append(gw.conns, conn)
			gw.growthClient = growthv1.NewGrowthServiceClient(conn)
			return nil
		}},
		{"plan", func(addr string) error {
			conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				return err
			}
			gw.conns = append(gw.conns, conn)
			gw.planClient = planv1.NewPlanServiceClient(conn)
			return nil
		}},
		{"admin", func(addr string) error {
			conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				return err
			}
			gw.conns = append(gw.conns, conn)
			gw.adminClient = adminv1.NewAdminServiceClient(conn)
			return nil
		}},
		{"companion", func(addr string) error {
			conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				return err
			}
			gw.conns = append(gw.conns, conn)
			gw.companionClient = companionv1.NewCompanionServiceClient(conn)
			return nil
		}},
		{"community", func(addr string) error {
			conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				return err
			}
			gw.conns = append(gw.conns, conn)
			gw.communityClient = communityv1.NewCommunityServiceClient(conn)
			return nil
		}},
	}

	for _, s := range setups {
		if svc, ok := cfg.Services[s.name]; ok && svc.Addr != "" {
			if err := s.setup(svc.Addr); err != nil {
				gw.Close()
				return nil, err
			}
		}
	}
	if svc, ok := cfg.Services["realtime"]; ok {
		gw.realtimeWSAddr = strings.TrimSpace(svc.Addr)
	}

	return gw, nil
}

// buildGatewayServiceToken 为 Gateway 生成内部服务调用令牌，供无用户上下文的受保护下游 RPC 使用。
func buildGatewayServiceToken(jwtSecret string) (string, error) {
	return auth.GenerateToken(0, "gateway@internal", "admin", jwtSecret, 24*time.Hour)
}

// Close 关闭所有连接
func (gw *Gateway) Close() {
	for _, conn := range gw.conns {
		conn.Close()
	}
}

// WrapResponseMiddleware 自动将所有 JSON 响应包装为 { code, message, data } 格式，
// 对齐前端 ApiEnvelope<T> 协议。同时解包 gRPC 单字段包装对象（如 { categories: [...] } → [...]）
// 并将 camelCase 字段名转为 snake_case。
func WrapResponseMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		w := &envelopeWriter{ResponseWriter: c.Writer, status: http.StatusOK}
		c.Writer = w
		c.Next()

		if w.passthrough {
			return
		}

		ct := w.Header().Get("Content-Type")
		if !strings.Contains(ct, "application/json") {
			w.writeBufferedBody()
			return
		}

		if !w.wroteBody {
			if w.status == http.StatusNoContent || c.Request.Method == http.MethodHead {
				return
			}
			w.writeEnvelope(buildEnvelopePayload(w.status, nil, nil))
			return
		}

		// 已经是 envelope 格式的不重复包装
		var check struct {
			Code *int `json:"code"`
		}
		if json.Unmarshal(w.body.Bytes(), &check) == nil && check.Code != nil {
			w.writeBufferedBody()
			return
		}

		// 解析原始响应
		var raw json.RawMessage
		if err := json.Unmarshal(w.body.Bytes(), &raw); err != nil {
			w.writeBufferedBody()
			return
		}

		// 解包 gRPC 单字段包装对象、分页结构与时间戳对象，并按页面旧协议做必要兼容。
		data := normalizeGatewayResponse(c.Request.URL.Path, raw)
		w.writeEnvelope(buildEnvelopePayload(w.status, data, w.body.Bytes()))
	}
}

// buildEnvelopePayload 根据 HTTP 状态码与原始错误体构造统一响应包装。
func buildEnvelopePayload(statusCode int, data interface{}, rawBody []byte) gin.H {
	code := 0
	message := "success"
	if statusCode >= 400 {
		code = statusCode
		var errResp struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(rawBody, &errResp) == nil && errResp.Error != "" {
			message = errResp.Error
		} else {
			message = http.StatusText(statusCode)
		}
	}
	return gin.H{
		"code":    code,
		"message": message,
		"data":    data,
	}
}

// normalizeGatewayResponse 将 gRPC/legacy JSON 响应统一转换为前端旧协议所需的数据结构。
func normalizeGatewayResponse(path string, raw json.RawMessage) interface{} {
	var decoded interface{}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		var fallback interface{}
		_ = json.Unmarshal(raw, &fallback)
		return fallback
	}
	return adaptLegacyResponseByPath(path, normalizeJSONValue(decoded))
}

// normalizeJSONValue 递归归一化 JSON 值，处理 snake_case、分页结构、单字段列表与 protobuf 时间戳。
func normalizeJSONValue(value interface{}) interface{} {
	switch typedValue := value.(type) {
	case []interface{}:
		result := make([]interface{}, len(typedValue))
		for i, item := range typedValue {
			result[i] = normalizeJSONValue(item)
		}
		return result
	case map[string]interface{}:
		if timestamp, ok := normalizeProtoTimestamp(typedValue); ok {
			return timestamp
		}

		result := make(map[string]interface{}, len(typedValue))
		for key, item := range typedValue {
			result[camelToSnake(key)] = normalizeJSONValue(item)
		}
		if flattened, ok := flattenPageResult(result); ok {
			return flattened
		}
		if unwrapped, ok := unwrapSingleListField(result); ok {
			return unwrapped
		}
		return result
	default:
		return typedValue
	}
}

// normalizeProtoTimestamp 将 protobuf 的 {seconds,nanos} 时间对象转换为 RFC3339 字符串。
func normalizeProtoTimestamp(value map[string]interface{}) (string, bool) {
	if len(value) == 0 || len(value) > 2 {
		return "", false
	}
	secondsValue, hasSeconds := value["seconds"]
	if !hasSeconds {
		return "", false
	}
	nanosValue := value["nanos"]

	seconds, ok := toInt64(secondsValue)
	if !ok {
		return "", false
	}
	nanos, ok := toInt64(nanosValue)
	if !ok {
		nanos = 0
	}
	return time.Unix(seconds, nanos).UTC().Format(time.RFC3339Nano), true
}

// flattenPageResult 将 { items/documents/questions..., page_result } 统一扁平为前端约定的分页结构。
func flattenPageResult(value map[string]interface{}) (map[string]interface{}, bool) {
	pageResultRaw, ok := value["page_result"]
	if !ok {
		return nil, false
	}
	pageResult, ok := pageResultRaw.(map[string]interface{})
	if !ok {
		return nil, false
	}

	listFieldCount := 0
	var listValue interface{}
	for key, candidate := range value {
		if key == "page_result" {
			continue
		}
		if _, ok := candidate.([]interface{}); !ok {
			return nil, false
		}
		listFieldCount++
		listValue = candidate
	}
	if listFieldCount != 1 {
		return nil, false
	}

	return map[string]interface{}{
		"list":      listValue,
		"total":     pageResult["total"],
		"page":      pageResult["page"],
		"page_size": pageResult["page_size"],
	}, true
}

// unwrapSingleListField 将仅包含一个数组字段的对象解包成数组，兼容旧单体直接返回列表的行为。
func unwrapSingleListField(value map[string]interface{}) (interface{}, bool) {
	if len(value) != 1 {
		return nil, false
	}
	for _, candidate := range value {
		if listValue, ok := candidate.([]interface{}); ok {
			return listValue, true
		}
	}
	return nil, false
}

// adaptLegacyResponseByPath 按接口路径补齐少量前端仍依赖的历史字段结构。
func adaptLegacyResponseByPath(path string, value interface{}) interface{} {
	switch {
	case strings.HasSuffix(path, "/admin/ai-configs"), strings.Contains(path, "/admin/ai-config-presets/") && strings.HasSuffix(path, "/apply"):
		return normalizeAdminAIConfigPayload(value)
	case strings.HasSuffix(path, "/admin/ai-config-presets"):
		return normalizeAdminAIPresetPayload(value)
	case strings.HasSuffix(path, "/admin/questions/tag-taxonomy"):
		return normalizeAdminQuestionTagTaxonomy(value)
	case strings.HasSuffix(path, "/admin/questions"):
		return normalizeAdminQuestionPagePayload(value)
	case strings.HasSuffix(path, "/admin/rag-configs"):
		return normalizeAdminRAGConfigPayload(value)
	default:
		return value
	}
}

// normalizeAdminAIConfigPayload 将 AI 配置响应修正为后台页面当前依赖的字段命名与默认支持信息。
func normalizeAdminAIConfigPayload(value interface{}) interface{} {
	payload, ok := value.(map[string]interface{})
	if !ok {
		return value
	}
	items, _ := payload["items"].([]interface{})
	normalizedItems := make([]interface{}, 0, len(items))
	for _, item := range items {
		typedItem, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		normalizedItems = append(normalizedItems, gin.H{
			"config_key":   typedItem["key"],
			"config_value": typedItem["value"],
			"config_type":  typedItem["config_type"],
			"description":  typedItem["description"],
		})
	}
	payload["items"] = normalizedItems
	if _, ok := payload["support"]; !ok {
		payload["support"] = gin.H{
			"primary_providers":  []string{"eino"},
			"fallback_providers": []string{},
			"notes": []string{
				"当前网关兼容层按旧后台协议补齐了运行时支持信息。",
				"当前后端仅支持 ai_provider=eino，fallback provider 暂未启用。",
			},
		}
	}
	if _, ok := payload["warnings"]; !ok {
		payload["warnings"] = []string{}
	}
	return payload
}

// normalizeAdminAIPresetPayload 将 AI 预设列表和单条预设都归一化为后台页直接消费的结构。
func normalizeAdminAIPresetPayload(value interface{}) interface{} {
	switch typedValue := value.(type) {
	case []interface{}:
		result := make([]interface{}, len(typedValue))
		for i, item := range typedValue {
			result[i] = normalizeAdminAIPresetPayload(item)
		}
		return result
	case map[string]interface{}:
		if _, ok := typedValue["configs"]; !ok {
			typedValue["configs"] = map[string]interface{}{}
		}
		return typedValue
	default:
		return value
	}
}

// normalizeAdminQuestionTagTaxonomy 将标签词典字段从 category 改回前端现用的 group。
func normalizeAdminQuestionTagTaxonomy(value interface{}) interface{} {
	items, ok := value.([]interface{})
	if !ok {
		return value
	}
	result := make([]interface{}, 0, len(items))
	for _, item := range items {
		typedItem, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		result = append(result, gin.H{
			"group":       typedItem["category"],
			"description": "",
			"tags":        typedItem["tags"],
		})
	}
	return result
}

// normalizeAdminQuestionPagePayload 解析题库管理页中的 JSON 字符串字段，避免编辑态直接读取时报错。
func normalizeAdminQuestionPagePayload(value interface{}) interface{} {
	pageResult, ok := value.(map[string]interface{})
	if !ok {
		return value
	}
	listItems, ok := pageResult["list"].([]interface{})
	if !ok {
		return value
	}
	normalizedList := make([]interface{}, 0, len(listItems))
	for _, item := range listItems {
		typedItem, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		normalizedList = append(normalizedList, normalizeAdminQuestionRecord(typedItem))
	}
	pageResult["list"] = normalizedList
	return pageResult
}

// normalizeAdminQuestionRecord 将后台题目记录中的字符串化结构字段恢复成前端编辑页直接可用的对象。
func normalizeAdminQuestionRecord(value map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{}, len(value)+4)
	for key, item := range value {
		result[key] = item
	}
	result["options"] = parseJSONArrayOrEmpty(toString(value["options_json"]))
	result["solution"] = parseJSONObjectOrNil(toString(value["solution_json"]))
	result["judge_config"] = parseJSONObjectOrNil(toString(value["judge_config_json"]))
	result["answer_template"] = parseJSONObjectOrNil(toString(value["answer_template_json"]))
	result["tags"] = splitTagString(toString(value["tags"]))
	return result
}

// normalizeAdminRAGConfigPayload 兼容后台 RAG 配置页当前依赖的 item 字段命名与 warnings 默认值。
func normalizeAdminRAGConfigPayload(value interface{}) interface{} {
	payload, ok := value.(map[string]interface{})
	if !ok {
		return value
	}
	configs, _ := payload["configs"].(map[string]interface{})
	status, _ := payload["status"].(map[string]interface{})
	payload["configs"] = gin.H{
		"ai_rag_enabled":         fmt.Sprint(status["enabled"]),
		"ai_rag_collection":      firstNonEmpty(toString(configs["ai_rag_collection"]), toString(configs["rag_collection_name"])),
		"ai_rag_embed_model":     firstNonEmpty(toString(configs["ai_rag_embed_model"]), toString(configs["rag_embedding_model"])),
		"ai_rag_top_k":           toString(configs["ai_rag_top_k"]),
		"ai_rag_score_threshold": toString(configs["ai_rag_score_threshold"]),
		"ai_rag_milvus_addr":     toString(configs["ai_rag_milvus_addr"]),
		"ai_rag_milvus_user":     toString(configs["ai_rag_milvus_user"]),
		"ai_rag_milvus_password": toString(configs["ai_rag_milvus_password"]),
		"ai_rag_embed_api_key":   toString(configs["ai_rag_embed_api_key"]),
		"ai_rag_embed_base_url":  toString(configs["ai_rag_embed_base_url"]),
	}
	items, _ := payload["items"].([]interface{})
	normalizedItems := make([]interface{}, 0, len(items))
	for _, item := range items {
		typedItem, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		configKey := firstNonEmpty(
			mapLegacyRAGConfigKey(toString(typedItem["key"])),
			toString(typedItem["key"]),
		)
		normalizedItems = append(normalizedItems, gin.H{
			"config_key":   configKey,
			"config_value": typedItem["value"],
			"config_type":  typedItem["config_type"],
			"description":  typedItem["description"],
		})
	}
	payload["items"] = normalizedItems
	if _, ok := payload["warnings"]; !ok {
		payload["warnings"] = []string{}
	}
	return payload
}

// mapLegacyRAGConfigKey 将当前 RAG 配置键映射回旧后台字段名，减少前端已有表单改动。
func mapLegacyRAGConfigKey(key string) string {
	switch strings.TrimSpace(key) {
	case "rag_collection_name":
		return "ai_rag_collection"
	case "rag_embedding_model":
		return "ai_rag_embed_model"
	default:
		return key
	}
}

// toInt64 将 JSON 反序列化后的数字统一转换为 int64，兼容 protobuf 时间戳字段。
func toInt64(value interface{}) (int64, bool) {
	switch typedValue := value.(type) {
	case nil:
		return 0, false
	case float64:
		return int64(typedValue), true
	case int64:
		return typedValue, true
	case int:
		return int64(typedValue), true
	case json.Number:
		parsed, err := typedValue.Int64()
		return parsed, err == nil
	default:
		return 0, false
	}
}

// toString 将接口值安全转换为字符串，供兼容层解析历史 JSON 字段。
func toString(value interface{}) string {
	if text, ok := value.(string); ok {
		return text
	}
	return ""
}

// parseJSONArrayOrEmpty 将 JSON 数组字符串解析为数组，失败时返回空数组避免页面崩溃。
func parseJSONArrayOrEmpty(raw string) []interface{} {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return []interface{}{}
	}
	var result []interface{}
	if err := json.Unmarshal([]byte(trimmed), &result); err != nil {
		return []interface{}{}
	}
	return result
}

// parseJSONObjectOrNil 将 JSON 对象字符串解析为对象，失败时返回 nil 保持旧页面的空值语义。
func parseJSONObjectOrNil(raw string) interface{} {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || trimmed == "null" {
		return nil
	}
	var result interface{}
	if err := json.Unmarshal([]byte(trimmed), &result); err != nil {
		return nil
	}
	return result
}

// splitTagString 将逗号分隔的标签字符串拆分为前端编辑页可直接使用的标签数组。
func splitTagString(raw string) []string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return []string{}
	}
	parts := strings.FieldsFunc(trimmed, func(r rune) bool {
		return r == ',' || r == '，'
	})
	result := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, item := range parts {
		tag := strings.TrimSpace(item)
		if tag == "" {
			continue
		}
		if _, exists := seen[tag]; exists {
			continue
		}
		seen[tag] = struct{}{}
		result = append(result, tag)
	}
	return result
}

// camelToSnake 将 camelCase 转为 snake_case
func camelToSnake(s string) string {
	var buf strings.Builder
	buf.Grow(len(s) + 4)
	for i, r := range s {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				buf.WriteByte('_')
			}
			buf.WriteByte(byte(r - 'A' + 'a'))
		} else {
			buf.WriteRune(r)
		}
	}
	return buf.String()
}

// envelopeWriter 捕获 handler 写入的响应体，供中间件二次处理。
type envelopeWriter struct {
	gin.ResponseWriter
	body        bytes.Buffer
	status      int
	wroteBody   bool
	passthrough bool
}

func (w *envelopeWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code) // 立即委托，让 Gin 正常设置 headers
}

func (w *envelopeWriter) Write(b []byte) (int, error) {
	w.wroteBody = true
	if w.passthrough {
		return w.ResponseWriter.Write(b)
	}
	return w.body.Write(b)
}

func (w *envelopeWriter) WriteString(s string) (int, error) {
	w.wroteBody = true
	if w.passthrough {
		return w.ResponseWriter.WriteString(s)
	}
	return w.body.WriteString(s)
}

// enablePassthrough 让 writer 从当前时刻开始直接透传到底层响应，供 SSE/流式输出使用。
func (w *envelopeWriter) enablePassthrough() {
	if w.passthrough {
		return
	}
	w.passthrough = true
	w.writeBufferedBody()
}

// Flush 在进入流式响应时立即把已缓冲数据写出，并将后续写入改为直通模式。
func (w *envelopeWriter) Flush() {
	w.enablePassthrough()
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// writeBufferedBody 将当前缓冲区内容原样写回底层响应。
func (w *envelopeWriter) writeBufferedBody() {
	if w.body.Len() == 0 {
		return
	}
	_, _ = w.ResponseWriter.Write(w.body.Bytes())
	w.body.Reset()
}

// writeEnvelope 将统一 envelope 结构写回底层响应。
func (w *envelopeWriter) writeEnvelope(payload gin.H) {
	envelope, err := json.Marshal(payload)
	if err != nil {
		w.writeBufferedBody()
		return
	}
	w.ResponseWriter.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, _ = w.ResponseWriter.Write(envelope)
	w.body.Reset()
}

// grpcErrorToHTTP 将 gRPC 状态码映射为 HTTP 状态码
func grpcErrorToHTTP(err error) (int, string) {
	st, ok := status.FromError(err)
	if !ok {
		return http.StatusInternalServerError, "internal error"
	}
	switch st.Code() {
	case codes.NotFound:
		return http.StatusNotFound, st.Message()
	case codes.InvalidArgument, codes.FailedPrecondition:
		return http.StatusBadRequest, st.Message()
	case codes.Unauthenticated:
		return http.StatusUnauthorized, st.Message()
	case codes.PermissionDenied:
		return http.StatusForbidden, st.Message()
	case codes.AlreadyExists, codes.Aborted:
		return http.StatusConflict, st.Message()
	case codes.ResourceExhausted:
		return http.StatusTooManyRequests, st.Message()
	default:
		return http.StatusInternalServerError, "internal error"
	}
}

// parseID 解析路径参数中的 ID，无效时返回错误响应
func parseID(c *gin.Context, param string) (uint64, bool) {
	id, err := strconv.ParseUint(c.Param(param), 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid " + param})
		return 0, false
	}
	return id, true
}

// getUserID 从上下文中获取用户 ID，缺失时返回未认证错误
func getUserID(c *gin.Context) (uint64, bool) {
	val, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not authenticated"})
		return 0, false
	}
	switch typedValue := val.(type) {
	case uint64:
		return typedValue, true
	case uint:
		return uint64(typedValue), true
	case int:
		if typedValue > 0 {
			return uint64(typedValue), true
		}
	}
	c.JSON(http.StatusUnauthorized, gin.H{"error": "user not authenticated"})
	return 0, false
}

// grpcErr 处理 gRPC 错误并返回 HTTP 响应
func grpcErr(c *gin.Context, err error) {
	code, msg := grpcErrorToHTTP(err)
	c.JSON(code, gin.H{"error": msg})
}

// loadAdminIndustryMaps 读取后台行业列表并构建 ID/Code 双向映射，供旧页面兼容层复用。
func (gw *Gateway) loadAdminIndustryMaps(ctx context.Context) (map[uint64]string, map[string]uint64, error) {
	resp, err := gw.adminClient.AdminListIndustries(ctx, &emptypb.Empty{})
	if err != nil {
		return nil, nil, err
	}
	idToCode := make(map[uint64]string, len(resp.GetIndustries()))
	codeToID := make(map[string]uint64, len(resp.GetIndustries()))
	for _, industry := range resp.GetIndustries() {
		code := strings.TrimSpace(industry.GetCode())
		idToCode[industry.GetId()] = code
		if code != "" {
			codeToID[code] = industry.GetId()
		}
	}
	return idToCode, codeToID, nil
}

// getOptionalAdminConfigUint64 尝试读取后台配置中的整型值，缺失或非法时回退为 0。
func (gw *Gateway) getOptionalAdminConfigUint64(ctx context.Context, key string) uint64 {
	resp, err := gw.adminClient.GetAdminConfig(ctx, &adminv1.GetAdminConfigRequest{Key: key})
	if err != nil {
		if st, ok := status.FromError(err); ok && st.Code() == codes.NotFound {
			return 0
		}
		return 0
	}
	value, parseErr := strconv.ParseUint(strings.TrimSpace(resp.GetValue()), 10, 64)
	if parseErr != nil {
		return 0
	}
	return value
}

// firstNonEmpty 返回第一个非空字符串，方便兼容旧字段和新字段并存的情况。
func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

// resolveTTSSupportMeta 根据引擎名称补齐后台页面展示所需的支持状态与说明。
func resolveTTSSupportMeta(engine string) (string, string) {
	switch strings.TrimSpace(engine) {
	case "volcengine":
		return "ready", "已按旧后台协议补齐火山引擎模板字段，可直接用于当前管理页。"
	case "xiaomi_mimo":
		return "planned", "已保留历史字段结构，但当前运行时默认仍以火山引擎链路为主。"
	default:
		return "legacy_unsupported", "该引擎缺少完整的旧后台元数据描述，请确认运行时实现是否仍可用。"
	}
}

// buildLegacyTTSProviders 构造旧后台 TTS 页面需要的供应商目录与字段模板。
func buildLegacyTTSProviders() []gin.H {
	return []gin.H{
		{
			"key":             "volcengine",
			"label":           "火山引擎",
			"description":     "当前后台默认维护的在线 TTS 供应商。",
			"support_status":  "ready",
			"support_message": "已按旧后台协议补齐模板字段，可直接创建与编辑配置。",
			"auth_template":   "{\n  \"app_id\": \"\",\n  \"token\": \"\",\n  \"cluster\": \"volcano_tts\"\n}",
			"params_template": "{\n  \"encoding\": \"mp3\",\n  \"speed_ratio\": 1\n}",
			"auth_fields": []gin.H{
				{"key": "app_id", "label": "App ID", "description": "火山引擎应用标识。", "required": true},
				{"key": "token", "label": "Token", "description": "火山引擎访问令牌。", "required": true, "secret": true},
				{"key": "cluster", "label": "Cluster", "description": "默认可使用 volcano_tts。", "required": false},
			},
			"param_fields": []gin.H{
				{"key": "encoding", "label": "编码格式", "description": "默认输出 mp3。", "required": false},
				{"key": "speed_ratio", "label": "语速", "description": "1 为默认语速。", "required": false},
			},
		},
		{
			"key":             "xiaomi_mimo",
			"label":           "小米 Mimo",
			"description":     "保留给历史配置和后续接入使用的兼容占位符。",
			"support_status":  "planned",
			"support_message": "当前仅保留旧页面结构占位，运行时链路默认未启用。",
			"auth_template":   "{\n  \"api_key\": \"\"\n}",
			"params_template": "{\n  \"voice\": \"\"\n}",
			"auth_fields": []gin.H{
				{"key": "api_key", "label": "API Key", "description": "供应商访问密钥。", "required": true, "secret": true},
			},
			"param_fields": []gin.H{
				{"key": "voice", "label": "Voice", "description": "运行时消费的音色编码。", "required": false},
			},
		},
	}
}

// RegisterRoutes 注册 HTTP 路由（对齐 backend 单体路由）
func (gw *Gateway) RegisterRoutes(r *gin.Engine) {
	gw.registerSystemRoutes(r)
	gw.registerV1Routes(r)
	gw.registerLegacyPublicRoutes(r)
	gw.registerLegacyAdminRoutes(r)

	// 保留独立的 admin SSE 端点（不经过 /api 前缀，供 Gateway 直接代理 SSE 流）
	adminSSE := r.Group("/admin")
	adminSSE.Use(gw.JWTMiddleware(), gw.AdminMiddleware())
	adminSSE.GET("/question-pipeline/generate/stream", gw.requireService("admin", gw.adminClient != nil, gw.handleAdminGenerateQuestionPipelineStream))
	adminSSE.POST("/question-pipeline/generate/stream", gw.requireService("admin", gw.adminClient != nil, gw.handleAdminGenerateQuestionPipelineDirectStream))
	return

	_ = r // suppress unused warning for dead code below
	api := r.Group("/api")

	// ========== 公开接口（无需认证，对齐 backend OptionalAuth） ==========
	public := api.Group("")
	{
		// 认证
		if gw.userClient != nil {
			public.POST("/auth/register", gw.handleRegister)
			public.POST("/auth/login", gw.handleLogin)
			public.POST("/auth/refresh", gw.handleRefreshToken)
		}
		// 题库公开接口
		if gw.questionClient != nil {
			public.GET("/questions", gw.handleListQuestions)
			public.GET("/questions/:id", gw.handleGetQuestion)
			public.GET("/industries", gw.handleListIndustries)
			public.GET("/categories", gw.handleListCategories)
		}
		// 社区公开接口
		if gw.communityClient != nil {
			public.GET("/community/posts", gw.handleListPosts)
			public.GET("/community/posts/:id", gw.handleGetPost)
			public.GET("/community/posts/:id/comments", gw.handleListComments)
		}
	}

	// ========== 需要认证的接口 ==========
	protected := api.Group("")
	protected.Use(gw.JWTMiddleware())
	{
		// --- 用户 ---
		if gw.userClient != nil {
			protected.GET("/auth/me", gw.handleGetProfile)
			protected.GET("/user/profile", gw.handleGetProfile)
			protected.PUT("/user/profile", gw.handleUpdateProfile)
		}

		// --- 题库（需认证部分） ---
		if gw.questionClient != nil {
			protected.POST("/questions/:id/submit", gw.handleSubmitAnswer)
			protected.POST("/questions/:id/run", gw.handleRunCode)
			protected.POST("/questions/:id/favorite", gw.handleToggleFavorite)
			protected.GET("/user/favorites", gw.handleListFavorites)
			protected.GET("/user/wrong-questions", gw.handleGetWrongQuestions)
			protected.GET("/user/notes", gw.handleListNotes)
			protected.POST("/user/notes", gw.handleCreateNote)
			protected.PUT("/user/notes/:id", gw.handleUpdateNote)
			protected.DELETE("/user/notes/:id", gw.handleDeleteNote)
			protected.GET("/user/practice-stats", gw.handleGetPracticeStats)
			protected.GET("/user/practice-recommendations", gw.handleGetPracticeRecommendations)
			protected.POST("/exams/random", gw.handleGetRandomExam)
		}

		// --- 面试 ---
		if gw.interviewClient != nil {
			protected.POST("/interviews", gw.handleCreateInterview)
			protected.GET("/interviews", gw.handleListInterviews)
			protected.GET("/interviews/:id", gw.handleGetInterview)
			protected.POST("/interviews/:id/answer", gw.handleSubmitInterviewAnswer)
			protected.GET("/interviews/:id/next", gw.handleGetNextQuestion)
			protected.POST("/interviews/:id/finish", gw.handleFinishInterview)
			protected.GET("/interviews/:id/report", gw.handleGetReport)
			protected.POST("/interviews/:id/coding", gw.handleSubmitCodingAnswer)
		}

		// --- 学习计划 ---
		if gw.planClient != nil {
			protected.POST("/plans", gw.handleCreatePlan)
			protected.GET("/plans/current", gw.handleGetCurrentPlan)
			protected.GET("/plans/:id", gw.handleGetPlan)
			protected.PUT("/plans/:id/tasks/:taskId", gw.handleUpdateTaskStatus)
			protected.POST("/plans/:id/tasks/:taskId/feedback", gw.handleSubmitTaskFeedback)
		}

		// --- 成长 ---
		if gw.growthClient != nil {
			protected.PUT("/user/study-logs/daily", gw.handleSyncStudyLog)
			protected.GET("/user/growth-summary", gw.handleGetGrowthSummary)
			protected.GET("/user/weekly-focus", gw.handleGetWeeklyFocus)
		}

		// --- 陪伴助手 ---
		if gw.companionClient != nil {
			protected.POST("/companion/chat", gw.handleCompanionChat)
		}

		// --- 社区（需认证部分） ---
		if gw.communityClient != nil {
			protected.POST("/community/posts", gw.handleCreatePost)
			protected.PUT("/community/posts/:id", gw.handleUpdatePost)
			protected.DELETE("/community/posts/:id", gw.handleDeletePost)
			protected.POST("/community/posts/:id/comments", gw.handleCreateComment)
			protected.POST("/community/posts/:id/like", gw.handleToggleLike)
		}

		// --- 管理后台 ---
		if gw.adminClient != nil {
			admin := protected.Group("/admin")
			admin.Use(gw.AdminMiddleware())
			{
				// 仪表盘
				admin.GET("/dashboard", gw.handleAdminGetDashboard)

				// 用户管理
				admin.GET("/users", gw.handleAdminListUsers)
				admin.PUT("/users/:id/role", gw.handleAdminUpdateUserRole)
				admin.PUT("/users/:id/disable", gw.handleAdminDisableUser)

				// 题库管理
				admin.GET("/questions", gw.handleAdminListQuestions)
				admin.POST("/questions", gw.handleAdminCreateQuestion)
				admin.PUT("/questions/:id", gw.handleAdminUpdateQuestion)
				admin.DELETE("/questions/:id", gw.handleAdminDeleteQuestion)
				admin.POST("/questions/import", gw.handleAdminBatchImportQuestions)
				admin.GET("/questions/tag-taxonomy", gw.handleAdminGetQuestionTagTaxonomy)

				// 题目流水线
				admin.POST("/question-pipeline/generate", gw.handleAdminGenerateQuestionPipeline)
				admin.POST("/question-pipeline/generate/async", gw.handleAdminGenerateQuestionPipelineAsync)
				admin.GET("/question-pipeline/generate/stream", gw.handleAdminGenerateQuestionPipelineStream)
				admin.POST("/question-pipeline/generate/stream", gw.handleAdminGenerateQuestionPipelineDirectStream)
				admin.POST("/question-pipeline/import", gw.handleAdminImportQuestionPipeline)

				// 分类管理
				admin.GET("/categories", gw.handleAdminListCategories)
				admin.POST("/categories", gw.handleAdminCreateCategory)
				admin.PUT("/categories/:id", gw.handleAdminUpdateCategory)
				admin.DELETE("/categories/:id", gw.handleAdminDeleteCategory)

				// 行业管理
				admin.GET("/industries", gw.handleAdminListIndustries)
				admin.POST("/industries", gw.handleAdminCreateIndustry)
				admin.PUT("/industries/:id", gw.handleAdminUpdateIndustry)

				// Prompt 模板
				admin.GET("/prompt-templates", gw.handleAdminListPromptTemplates)
				admin.GET("/prompts", gw.handleAdminListPrompts)
				admin.POST("/prompt-templates", gw.handleAdminSavePromptTemplate)
				admin.POST("/prompts", gw.handleAdminCreatePrompt)
				admin.PUT("/prompts/:id", gw.handleAdminUpdatePrompt)
				admin.DELETE("/prompts/:id", gw.handleAdminDeletePrompt)
				admin.POST("/prompts/test-render", gw.handleAdminTestRenderPrompt)

				// AI 配置
				admin.GET("/ai-configs", gw.handleAdminGetAIConfigs)
				admin.PUT("/ai-configs", gw.handleAdminUpdateAIConfigs)

				// AI 预设
				admin.GET("/ai-config-presets", gw.handleAdminListAIPresets)
				admin.POST("/ai-config-presets", gw.handleAdminCreateAIPreset)
				admin.PUT("/ai-config-presets/:id", gw.handleAdminUpdateAIPreset)
				admin.DELETE("/ai-config-presets/:id", gw.handleAdminDeleteAIPreset)
				admin.POST("/ai-config-presets/:id/apply", gw.handleAdminApplyAIPreset)

				// AI 调试 & 日志
				admin.POST("/ai/debug", gw.handleAdminDebugAI)
				admin.GET("/ai-call-logs", gw.handleAdminListAICallLogs)
				admin.GET("/ai-call-logs/:id", gw.handleAdminGetAICallLog)

				// Live2D 管理
				admin.GET("/live2d-models", gw.handleAdminListLive2DModels)
				admin.POST("/live2d-models", gw.handleAdminCreateLive2DModel)
				admin.PUT("/live2d-models/:id", gw.handleAdminUpdateLive2DModel)
				admin.DELETE("/live2d-models/:id", gw.handleAdminDeleteLive2DModel)
				admin.POST("/live2d-models/import", gw.handleAdminImportLive2DPackage)
				admin.POST("/live2d-models/backgrounds/import", gw.handleAdminImportLive2DBackground)

				// TTS 管理
				admin.GET("/tts-configs", gw.handleAdminListTTSConfigs)
				admin.POST("/tts-configs", gw.handleAdminCreateTTSConfig)
				admin.PUT("/tts-configs/:id", gw.handleAdminUpdateTTSConfig)
				admin.DELETE("/tts-configs/:id", gw.handleAdminDeleteTTSConfig)
				admin.PUT("/tts-configs/defaults", gw.handleAdminUpdateTTSSceneDefaults)

				// RAG 配置
				admin.GET("/rag-configs", gw.handleAdminGetRAGConfigs)
				admin.PUT("/rag-configs", gw.handleAdminUpdateRAGConfigs)
				admin.POST("/rag-configs/test", gw.handleAdminTestRAGConnection)

				// RAG 索引
				admin.POST("/rag/index-all", gw.handleAdminIndexAllQuestions)
				admin.POST("/rag/index", gw.handleAdminIndexQuestions)
				admin.DELETE("/rag/index", gw.handleAdminDeleteRAGIndex)
				admin.GET("/rag/search", gw.handleAdminSearchRAGQuestions)

				// RAG 文档
				admin.GET("/rag-documents", gw.handleAdminListRAGDocuments)
				admin.GET("/rag-documents/stats", gw.handleAdminGetRAGDocumentStats)
				admin.GET("/rag-documents/:id", gw.handleAdminGetRAGDocument)
				admin.POST("/rag-documents", gw.handleAdminCreateRAGDocument)
				admin.PUT("/rag-documents/:id", gw.handleAdminUpdateRAGDocument)
				admin.DELETE("/rag-documents/:id", gw.handleAdminDeleteRAGDocument)
				admin.POST("/rag-documents/batch-import", gw.handleAdminBatchImportRAGDocuments)
				admin.POST("/rag-documents/sync", gw.handleAdminSyncRAGDocuments)
				admin.POST("/rag-documents/sync-all", gw.handleAdminSyncAllPendingRAGDocuments)

				// 面经爬虫
				admin.GET("/scraper/sources", gw.handleAdminGetScraperSources)
				admin.POST("/scraper/search", gw.handleAdminScraperSearch)
				admin.POST("/scraper/fetch", gw.handleAdminScraperFetch)
				admin.POST("/scraper/clean", gw.handleAdminScraperClean)
				admin.POST("/scraper/import", gw.handleAdminScraperImport)
				admin.POST("/scraper/import/async", gw.handleAdminScraperImportAsync)
				admin.GET("/scraper/tasks", gw.handleAdminListScraperTasks)
				admin.GET("/scraper/tasks/:id", gw.handleAdminGetScraperTask)
				admin.POST("/scraper/tasks/:id/retry", gw.handleAdminRetryScraperTask)

				// 系统配置
				admin.GET("/configs/:key", gw.handleAdminGetConfig)
				admin.PUT("/configs/:key", gw.handleAdminSetConfig)
			}
		}
	}
}

// registerV1Routes 注册 P6-4 要求的 `/api/v1` 业务路由，并保留少量仍被页面依赖的兼容路径。
func (gw *Gateway) registerV1Routes(r *gin.Engine) {
	api := r.Group("/api/v1")

	public := api.Group("")
	{
		public.POST("/auth/register", gw.requireService("user", gw.userClient != nil, gw.handleRegister))
		public.POST("/auth/login", gw.requireService("user", gw.userClient != nil, gw.handleLogin))
		public.POST("/auth/refresh", gw.requireService("user", gw.userClient != nil, gw.handleRefreshToken))
		public.GET("/questions", gw.requireService("question", gw.questionClient != nil, gw.handleListQuestions))
		public.GET("/questions/:id", gw.requireService("question", gw.questionClient != nil, gw.handleGetQuestion))
		public.GET("/question-sets", gw.requireService("question", gw.questionClient != nil, gw.handleListQuestionSets))
		public.GET("/question-sets/:id", gw.requireService("question", gw.questionClient != nil, gw.handleGetQuestionSetDetail))
		public.GET("/industries", gw.requireService("question", gw.questionClient != nil, gw.handleListIndustries))
		public.GET("/categories", gw.requireService("question", gw.questionClient != nil, gw.handleListCategories))
		public.GET("/community/posts", gw.requireService("community", gw.communityClient != nil, gw.handleListPosts))
		public.GET("/community/posts/:id", gw.requireService("community", gw.communityClient != nil, gw.handleGetPost))
		public.GET("/community/posts/:id/comments", gw.requireService("community", gw.communityClient != nil, gw.handleListComments))
		public.POST("/membership/callback", gw.requireService("membership", gw.membershipClient != nil, gw.handlePaymentCallback))
		public.GET("/live2d/models", gw.requireService("admin", gw.adminClient != nil, gw.handleListPublicLive2DModels))
		public.GET("/live2d/current", gw.requireService("admin", gw.adminClient != nil, gw.handleGetPublicCurrentLive2DModel))
	}

	protected := api.Group("")
	protected.Use(gw.JWTMiddleware())
	{
		protected.POST("/auth/logout", gw.requireService("user", gw.userClient != nil, gw.handleLogout))
		protected.GET("/auth/me", gw.requireService("user", gw.userClient != nil, gw.handleGetProfile))
		protected.GET("/user/profile", gw.requireService("user", gw.userClient != nil, gw.handleGetProfile))
		protected.PUT("/user/profile", gw.requireService("user", gw.userClient != nil, gw.handleUpdateProfile))

		protected.POST("/questions/submit", gw.requireService("question", gw.questionClient != nil, gw.handleSubmitAnswerV1))
		protected.POST("/questions/run-code", gw.requireService("question", gw.questionClient != nil, gw.handleRunCodeV1))
		protected.GET("/questions/recommendations", gw.requireService("question", gw.questionClient != nil, gw.handleGetPracticeRecommendations))
		protected.GET("/mistakes/topics", gw.requireService("question", gw.questionClient != nil, gw.handleListMistakeTopics))
		protected.GET("/mistakes/topics/:code", gw.requireService("question", gw.questionClient != nil, gw.handleGetMistakeTopic))
		protected.GET("/mistake-topics", gw.requireService("question", gw.questionClient != nil, gw.handleListMistakeTopics))
		protected.GET("/mistake-topics/:code", gw.requireService("question", gw.questionClient != nil, gw.handleGetMistakeTopic))
		protected.POST("/exams/timed", gw.requireService("question", gw.questionClient != nil, gw.handleGenerateTimedExam))
		protected.POST("/exams/:id/submit", gw.requireService("question", gw.questionClient != nil, gw.handleSubmitExam))
		protected.POST("/notes", gw.requireService("question", gw.questionClient != nil, gw.handleCreateNote))
		protected.DELETE("/notes/:id", gw.requireService("question", gw.questionClient != nil, gw.handleDeleteNote))

		protected.POST("/interviews", gw.requireService("interview", gw.interviewClient != nil, gw.handleCreateInterview))
		protected.GET("/interviews", gw.requireService("interview", gw.interviewClient != nil, gw.handleListInterviews))
		protected.GET("/interviews/:id", gw.requireService("interview", gw.interviewClient != nil, gw.handleGetInterview))
		protected.POST("/interviews/:id/next-question", gw.requireService("interview", gw.interviewClient != nil, gw.handleGetNextQuestionV1))
		protected.POST("/interviews/:id/finish", gw.requireService("interview", gw.interviewClient != nil, gw.handleFinishInterview))
		protected.GET("/interviews/:id/report", gw.requireService("interview", gw.interviewClient != nil, gw.handleGetReport))
		protected.POST("/interviews/:id/coding", gw.requireService("interview", gw.interviewClient != nil, gw.handleSubmitCodingAnswer))
		protected.GET("/interviews/:id/ws", gw.handleProxyRealtimeInterviewWS)

		protected.POST("/plans", gw.requireService("plan", gw.planClient != nil, gw.handleCreatePlan))
		protected.GET("/plans", gw.requireService("plan", gw.planClient != nil, gw.handleListPlans))
		protected.GET("/plans/current", gw.requireService("plan", gw.planClient != nil, gw.handleGetCurrentPlan))
		protected.GET("/plans/:id", gw.requireService("plan", gw.planClient != nil, gw.handleGetPlan))
		protected.PUT("/plans/:id/tasks/:tid/status", gw.requireService("plan", gw.planClient != nil, gw.handleUpdateTaskStatusV1))
		protected.POST("/plans/:id/tasks/:tid/feedback", gw.requireService("plan", gw.planClient != nil, gw.handleSubmitTaskFeedbackV1))
		protected.POST("/plans/:id/adjust", gw.requireService("plan", gw.planClient != nil, gw.handleAdjustPlan))
		protected.GET("/plans/:id/progress", gw.requireService("plan", gw.planClient != nil, gw.handleGetPlanProgress))

		protected.GET("/growth/summary", gw.requireService("growth", gw.growthClient != nil, gw.handleGetGrowthSummary))
		protected.GET("/growth/weekly-focus", gw.requireService("growth", gw.growthClient != nil, gw.handleGetWeeklyFocus))
		protected.POST("/growth/study-log", gw.requireService("growth", gw.growthClient != nil, gw.handleSyncStudyLogV1))

		protected.POST("/companion/chat", gw.requireService("companion", gw.companionClient != nil, gw.handleCompanionChat))
		protected.GET("/companion/state", gw.requireService("companion", gw.companionClient != nil, gw.handleGetCompanionState))
		protected.POST("/companion/tts", gw.requireService("companion", gw.companionClient != nil, gw.handleSynthesizeSpeech))

		protected.GET("/community/my/posts", gw.requireService("community", gw.communityClient != nil, gw.handleListMyPosts))
		protected.POST("/community/posts", gw.requireService("community", gw.communityClient != nil, gw.handleCreatePost))
		protected.PUT("/community/posts/:id", gw.requireService("community", gw.communityClient != nil, gw.handleUpdatePost))
		protected.POST("/community/posts/:id/like", gw.requireService("community", gw.communityClient != nil, gw.handleToggleLike))

		protected.POST("/membership/orders", gw.requireService("membership", gw.membershipClient != nil, gw.handleCreateOrder))
		protected.GET("/membership/info", gw.requireService("membership", gw.membershipClient != nil, gw.handleMembershipInfo))
		protected.POST("/membership/check-access", gw.requireService("membership", gw.membershipClient != nil, gw.handleCheckFeatureAccess))
		protected.GET("/membership/plans", gw.requireService("membership", gw.membershipClient != nil, gw.handleListMembershipPlans))
		protected.GET("/membership/orders", gw.requireService("membership", gw.membershipClient != nil, gw.handleListOrders))
		protected.GET("/membership/orders/:id", gw.requireService("membership", gw.membershipClient != nil, gw.handleGetOrder))
		protected.POST("/membership/upgrade", gw.requireService("membership", gw.membershipClient != nil, gw.handleUpgradeMembership))

		protected.POST("/questions/:id/submit", gw.requireService("question", gw.questionClient != nil, gw.handleSubmitAnswer))
		protected.POST("/questions/:id/run", gw.requireService("question", gw.questionClient != nil, gw.handleRunCode))
		protected.POST("/questions/:id/favorite", gw.requireService("question", gw.questionClient != nil, gw.handleToggleFavorite))
		protected.GET("/user/favorites", gw.requireService("question", gw.questionClient != nil, gw.handleListFavorites))
		protected.GET("/user/wrong-questions", gw.requireService("question", gw.questionClient != nil, gw.handleGetWrongQuestions))
		protected.GET("/user/notes", gw.requireService("question", gw.questionClient != nil, gw.handleListNotes))
		protected.POST("/user/notes", gw.requireService("question", gw.questionClient != nil, gw.handleCreateNote))
		protected.PUT("/user/notes/:id", gw.requireService("question", gw.questionClient != nil, gw.handleUpdateNote))
		protected.DELETE("/user/notes/:id", gw.requireService("question", gw.questionClient != nil, gw.handleDeleteNote))
		protected.GET("/user/practice-stats", gw.requireService("question", gw.questionClient != nil, gw.handleGetPracticeStats))
		protected.GET("/user/practice-recommendations", gw.requireService("question", gw.questionClient != nil, gw.handleGetPracticeRecommendations))
		protected.POST("/exams/random", gw.requireService("question", gw.questionClient != nil, gw.handleGetRandomExam))
		protected.POST("/interviews/:id/answer", gw.requireService("interview", gw.interviewClient != nil, gw.handleSubmitInterviewAnswer))
		protected.GET("/interviews/:id/next", gw.requireService("interview", gw.interviewClient != nil, gw.handleGetNextQuestion))
		protected.PUT("/user/study-logs/daily", gw.requireService("growth", gw.growthClient != nil, gw.handleSyncStudyLog))
		protected.GET("/user/growth-summary", gw.requireService("growth", gw.growthClient != nil, gw.handleGetGrowthSummary))
		protected.GET("/user/weekly-focus", gw.requireService("growth", gw.growthClient != nil, gw.handleGetWeeklyFocus))
		protected.PUT("/plans/:id/tasks/:tid", gw.requireService("plan", gw.planClient != nil, gw.handleUpdateTaskStatusV1))
		protected.DELETE("/community/posts/:id", gw.requireService("community", gw.communityClient != nil, gw.handleDeletePost))
		protected.POST("/community/posts/:id/comments", gw.requireService("community", gw.communityClient != nil, gw.handleCreateComment))
	}

	// Admin BFF 路由
	admin := api.Group("/admin")
	admin.Use(gw.JWTMiddleware(), gw.AdminMiddleware())
	{
		admin.GET("/dashboard", gw.requireService("admin", gw.adminClient != nil, gw.handleAdminGetDashboard))
		admin.GET("/users", gw.requireService("admin", gw.adminClient != nil, gw.handleAdminListUsers))
		admin.PUT("/users/:id/role", gw.requireService("admin", gw.adminClient != nil, gw.handleAdminUpdateUserRole))
		admin.PUT("/users/:id/disable", gw.requireService("admin", gw.adminClient != nil, gw.handleAdminDisableUser))
		admin.GET("/questions", gw.requireService("admin", gw.adminClient != nil, gw.handleAdminListQuestions))
		admin.POST("/questions", gw.requireService("admin", gw.adminClient != nil, gw.handleAdminCreateQuestion))
		admin.PUT("/questions/:id", gw.requireService("admin", gw.adminClient != nil, gw.handleAdminUpdateQuestion))
		admin.DELETE("/questions/:id", gw.requireService("admin", gw.adminClient != nil, gw.handleAdminDeleteQuestion))
		admin.POST("/questions/import", gw.requireService("admin", gw.adminClient != nil, gw.handleAdminBatchImportQuestions))
		admin.GET("/questions/tag-taxonomy", gw.requireService("admin", gw.adminClient != nil, gw.handleAdminGetQuestionTagTaxonomy))
		admin.POST("/question-pipeline/generate", gw.requireService("admin", gw.adminClient != nil, gw.handleAdminGenerateQuestionPipeline))
		admin.POST("/question-pipeline/generate/async", gw.requireService("admin", gw.adminClient != nil, gw.handleAdminGenerateQuestionPipelineAsync))
		admin.GET("/question-pipeline/generate/stream", gw.requireService("admin", gw.adminClient != nil, gw.handleAdminGenerateQuestionPipelineStream))
		admin.POST("/question-pipeline/generate/stream", gw.requireService("admin", gw.adminClient != nil, gw.handleAdminGenerateQuestionPipelineDirectStream))
		admin.POST("/question-pipeline/import", gw.requireService("admin", gw.adminClient != nil, gw.handleAdminImportQuestionPipeline))
		admin.GET("/categories", gw.requireService("admin", gw.adminClient != nil, gw.handleAdminListCategories))
		admin.POST("/categories", gw.requireService("admin", gw.adminClient != nil, gw.handleAdminCreateCategory))
		admin.PUT("/categories/:id", gw.requireService("admin", gw.adminClient != nil, gw.handleAdminUpdateCategory))
		admin.DELETE("/categories/:id", gw.requireService("admin", gw.adminClient != nil, gw.handleAdminDeleteCategory))
		admin.GET("/industries", gw.requireService("admin", gw.adminClient != nil, gw.handleAdminListIndustries))
		admin.POST("/industries", gw.requireService("admin", gw.adminClient != nil, gw.handleAdminCreateIndustry))
		admin.PUT("/industries/:id", gw.requireService("admin", gw.adminClient != nil, gw.handleAdminUpdateIndustry))
		admin.GET("/prompt-templates", gw.requireService("admin", gw.adminClient != nil, gw.handleAdminListPromptTemplates))
		admin.GET("/prompts", gw.requireService("admin", gw.adminClient != nil, gw.handleAdminListPrompts))
		admin.POST("/prompt-templates", gw.requireService("admin", gw.adminClient != nil, gw.handleAdminSavePromptTemplate))
		admin.POST("/prompts", gw.requireService("admin", gw.adminClient != nil, gw.handleAdminCreatePrompt))
		admin.PUT("/prompts/:id", gw.requireService("admin", gw.adminClient != nil, gw.handleAdminUpdatePrompt))
		admin.DELETE("/prompts/:id", gw.requireService("admin", gw.adminClient != nil, gw.handleAdminDeletePrompt))
		admin.POST("/prompts/test-render", gw.requireService("admin", gw.adminClient != nil, gw.handleAdminTestRenderPrompt))
		admin.GET("/ai-configs", gw.requireService("admin", gw.adminClient != nil, gw.handleAdminGetAIConfigs))
		admin.PUT("/ai-configs", gw.requireService("admin", gw.adminClient != nil, gw.handleAdminUpdateAIConfigs))
		admin.GET("/ai-config-presets", gw.requireService("admin", gw.adminClient != nil, gw.handleAdminListAIPresets))
		admin.POST("/ai-config-presets", gw.requireService("admin", gw.adminClient != nil, gw.handleAdminCreateAIPreset))
		admin.PUT("/ai-config-presets/:id", gw.requireService("admin", gw.adminClient != nil, gw.handleAdminUpdateAIPreset))
		admin.DELETE("/ai-config-presets/:id", gw.requireService("admin", gw.adminClient != nil, gw.handleAdminDeleteAIPreset))
		admin.POST("/ai-config-presets/:id/apply", gw.requireService("admin", gw.adminClient != nil, gw.handleAdminApplyAIPreset))
		admin.POST("/ai/debug", gw.requireService("admin", gw.adminClient != nil, gw.handleAdminDebugAI))
		admin.GET("/ai-call-logs", gw.requireService("admin", gw.adminClient != nil, gw.handleAdminListAICallLogs))
		admin.GET("/ai-call-logs/:id", gw.requireService("admin", gw.adminClient != nil, gw.handleAdminGetAICallLog))
		admin.GET("/live2d-models", gw.requireService("admin", gw.adminClient != nil, gw.handleAdminListLive2DModels))
		admin.POST("/live2d-models", gw.requireService("admin", gw.adminClient != nil, gw.handleAdminCreateLive2DModel))
		admin.PUT("/live2d-models/:id", gw.requireService("admin", gw.adminClient != nil, gw.handleAdminUpdateLive2DModel))
		admin.DELETE("/live2d-models/:id", gw.requireService("admin", gw.adminClient != nil, gw.handleAdminDeleteLive2DModel))
		admin.POST("/live2d-models/import", gw.requireService("admin", gw.adminClient != nil, gw.handleAdminImportLive2DPackage))
		admin.POST("/live2d-models/backgrounds/import", gw.requireService("admin", gw.adminClient != nil, gw.handleAdminImportLive2DBackground))
		admin.GET("/tts-configs", gw.requireService("admin", gw.adminClient != nil, gw.handleAdminListTTSConfigs))
		admin.POST("/tts-configs", gw.requireService("admin", gw.adminClient != nil, gw.handleAdminCreateTTSConfig))
		admin.PUT("/tts-configs/:id", gw.requireService("admin", gw.adminClient != nil, gw.handleAdminUpdateTTSConfig))
		admin.DELETE("/tts-configs/:id", gw.requireService("admin", gw.adminClient != nil, gw.handleAdminDeleteTTSConfig))
		admin.PUT("/tts-configs/defaults", gw.requireService("admin", gw.adminClient != nil, gw.handleAdminUpdateTTSSceneDefaults))
		admin.GET("/rag-configs", gw.requireService("admin", gw.adminClient != nil, gw.handleAdminGetRAGConfigs))
		admin.PUT("/rag-configs", gw.requireService("admin", gw.adminClient != nil, gw.handleAdminUpdateRAGConfigs))
		admin.POST("/rag-configs/test", gw.requireService("admin", gw.adminClient != nil, gw.handleAdminTestRAGConnection))
		admin.POST("/rag/index-all", gw.requireService("admin", gw.adminClient != nil, gw.handleAdminIndexAllQuestions))
		admin.POST("/rag/index", gw.requireService("admin", gw.adminClient != nil, gw.handleAdminIndexQuestions))
		admin.DELETE("/rag/index", gw.requireService("admin", gw.adminClient != nil, gw.handleAdminDeleteRAGIndex))
		admin.GET("/rag/search", gw.requireService("admin", gw.adminClient != nil, gw.handleAdminSearchRAGQuestions))
		admin.GET("/rag-documents", gw.requireService("admin", gw.adminClient != nil, gw.handleAdminListRAGDocuments))
		admin.GET("/rag-documents/stats", gw.requireService("admin", gw.adminClient != nil, gw.handleAdminGetRAGDocumentStats))
		admin.GET("/rag-documents/:id", gw.requireService("admin", gw.adminClient != nil, gw.handleAdminGetRAGDocument))
		admin.POST("/rag-documents", gw.requireService("admin", gw.adminClient != nil, gw.handleAdminCreateRAGDocument))
		admin.PUT("/rag-documents/:id", gw.requireService("admin", gw.adminClient != nil, gw.handleAdminUpdateRAGDocument))
		admin.DELETE("/rag-documents/:id", gw.requireService("admin", gw.adminClient != nil, gw.handleAdminDeleteRAGDocument))
		admin.POST("/rag-documents/batch-import", gw.requireService("admin", gw.adminClient != nil, gw.handleAdminBatchImportRAGDocuments))
		admin.POST("/rag-documents/sync", gw.requireService("admin", gw.adminClient != nil, gw.handleAdminSyncRAGDocuments))
		admin.POST("/rag-documents/sync-all", gw.requireService("admin", gw.adminClient != nil, gw.handleAdminSyncAllPendingRAGDocuments))
		admin.GET("/scraper/sources", gw.requireService("admin", gw.adminClient != nil, gw.handleAdminGetScraperSources))
		admin.POST("/scraper/search", gw.requireService("admin", gw.adminClient != nil, gw.handleAdminScraperSearch))
		admin.POST("/scraper/fetch", gw.requireService("admin", gw.adminClient != nil, gw.handleAdminScraperFetch))
		admin.POST("/scraper/clean", gw.requireService("admin", gw.adminClient != nil, gw.handleAdminScraperClean))
		admin.POST("/scraper/import", gw.requireService("admin", gw.adminClient != nil, gw.handleAdminScraperImport))
		admin.POST("/scraper/import/async", gw.requireService("admin", gw.adminClient != nil, gw.handleAdminScraperImportAsync))
		admin.GET("/scraper/tasks", gw.requireService("admin", gw.adminClient != nil, gw.handleAdminListScraperTasks))
		admin.GET("/scraper/tasks/:id", gw.requireService("admin", gw.adminClient != nil, gw.handleAdminGetScraperTask))
		admin.POST("/scraper/tasks/:id/retry", gw.requireService("admin", gw.adminClient != nil, gw.handleAdminRetryScraperTask))
		admin.GET("/configs/:key", gw.requireService("admin", gw.adminClient != nil, gw.handleAdminGetConfig))
		admin.PUT("/configs/:key", gw.requireService("admin", gw.adminClient != nil, gw.handleAdminSetConfig))
	}
}

// requireService 在具体业务服务未接通时统一返回 503，避免将缺失能力误暴露为 404。
// registerLegacyPublicRoutes 保留仍被前端直接使用的 `/api/...` 公开业务路径。
func (gw *Gateway) registerLegacyPublicRoutes(r *gin.Engine) {
	api := r.Group("/api")
	public := api.Group("")
	{
		public.GET("/live2d/models", gw.requireService("admin", gw.adminClient != nil, gw.handleListPublicLive2DModels))
		public.GET("/live2d/current", gw.requireService("admin", gw.adminClient != nil, gw.handleGetPublicCurrentLive2DModel))
	}
}

func (gw *Gateway) requireService(service string, available bool, next gin.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !available {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error":   "service unavailable",
				"service": service,
			})
			return
		}
		next(c)
	}
}

// registerLegacyAdminRoutes 保留现有后台管理 REST 路径，避免 P6-4 影响已接通的管理端页面。
func (gw *Gateway) registerLegacyAdminRoutes(r *gin.Engine) {
	admin := r.Group("/api/admin")
	admin.Use(gw.JWTMiddleware(), gw.AdminMiddleware(), gw.requireService("admin", gw.adminClient != nil, func(c *gin.Context) {
		c.Next()
	}))
	{
		admin.GET("/dashboard", gw.handleAdminGetDashboard)
		admin.GET("/users", gw.handleAdminListUsers)
		admin.PUT("/users/:id/role", gw.handleAdminUpdateUserRole)
		admin.PUT("/users/:id/disable", gw.handleAdminDisableUser)
		admin.GET("/questions", gw.handleAdminListQuestions)
		admin.POST("/questions", gw.handleAdminCreateQuestion)
		admin.PUT("/questions/:id", gw.handleAdminUpdateQuestion)
		admin.DELETE("/questions/:id", gw.handleAdminDeleteQuestion)
		admin.POST("/questions/import", gw.handleAdminBatchImportQuestions)
		admin.GET("/questions/tag-taxonomy", gw.handleAdminGetQuestionTagTaxonomy)
		admin.POST("/question-pipeline/generate", gw.handleAdminGenerateQuestionPipeline)
		admin.POST("/question-pipeline/generate/async", gw.handleAdminGenerateQuestionPipelineAsync)
		admin.GET("/question-pipeline/generate/stream", gw.handleAdminGenerateQuestionPipelineStream)
		admin.POST("/question-pipeline/generate/stream", gw.handleAdminGenerateQuestionPipelineDirectStream)
		admin.POST("/question-pipeline/import", gw.handleAdminImportQuestionPipeline)
		admin.GET("/categories", gw.handleAdminListCategories)
		admin.POST("/categories", gw.handleAdminCreateCategory)
		admin.PUT("/categories/:id", gw.handleAdminUpdateCategory)
		admin.DELETE("/categories/:id", gw.handleAdminDeleteCategory)
		admin.GET("/industries", gw.handleAdminListIndustries)
		admin.POST("/industries", gw.handleAdminCreateIndustry)
		admin.PUT("/industries/:id", gw.handleAdminUpdateIndustry)
		admin.GET("/prompt-templates", gw.handleAdminListPromptTemplates)
		admin.GET("/prompts", gw.handleAdminListPrompts)
		admin.POST("/prompt-templates", gw.handleAdminSavePromptTemplate)
		admin.POST("/prompts", gw.handleAdminCreatePrompt)
		admin.PUT("/prompts/:id", gw.handleAdminUpdatePrompt)
		admin.DELETE("/prompts/:id", gw.handleAdminDeletePrompt)
		admin.POST("/prompts/test-render", gw.handleAdminTestRenderPrompt)
		admin.GET("/ai-configs", gw.handleAdminGetAIConfigs)
		admin.PUT("/ai-configs", gw.handleAdminUpdateAIConfigs)
		admin.GET("/ai-config-presets", gw.handleAdminListAIPresets)
		admin.POST("/ai-config-presets", gw.handleAdminCreateAIPreset)
		admin.PUT("/ai-config-presets/:id", gw.handleAdminUpdateAIPreset)
		admin.DELETE("/ai-config-presets/:id", gw.handleAdminDeleteAIPreset)
		admin.POST("/ai-config-presets/:id/apply", gw.handleAdminApplyAIPreset)
		admin.POST("/ai/debug", gw.handleAdminDebugAI)
		admin.GET("/ai-call-logs", gw.handleAdminListAICallLogs)
		admin.GET("/ai-call-logs/:id", gw.handleAdminGetAICallLog)
		admin.GET("/live2d-models", gw.handleAdminListLive2DModels)
		admin.POST("/live2d-models", gw.handleAdminCreateLive2DModel)
		admin.PUT("/live2d-models/:id", gw.handleAdminUpdateLive2DModel)
		admin.DELETE("/live2d-models/:id", gw.handleAdminDeleteLive2DModel)
		admin.POST("/live2d-models/import", gw.handleAdminImportLive2DPackage)
		admin.POST("/live2d-models/backgrounds/import", gw.handleAdminImportLive2DBackground)
		admin.GET("/tts-configs", gw.handleAdminListTTSConfigs)
		admin.POST("/tts-configs", gw.handleAdminCreateTTSConfig)
		admin.PUT("/tts-configs/:id", gw.handleAdminUpdateTTSConfig)
		admin.DELETE("/tts-configs/:id", gw.handleAdminDeleteTTSConfig)
		admin.PUT("/tts-configs/defaults", gw.handleAdminUpdateTTSSceneDefaults)
		admin.GET("/rag-configs", gw.handleAdminGetRAGConfigs)
		admin.PUT("/rag-configs", gw.handleAdminUpdateRAGConfigs)
		admin.POST("/rag-configs/test", gw.handleAdminTestRAGConnection)
		admin.POST("/rag/index-all", gw.handleAdminIndexAllQuestions)
		admin.POST("/rag/index", gw.handleAdminIndexQuestions)
		admin.DELETE("/rag/index", gw.handleAdminDeleteRAGIndex)
		admin.GET("/rag/search", gw.handleAdminSearchRAGQuestions)
		admin.GET("/rag-documents", gw.handleAdminListRAGDocuments)
		admin.GET("/rag-documents/stats", gw.handleAdminGetRAGDocumentStats)
		admin.GET("/rag-documents/:id", gw.handleAdminGetRAGDocument)
		admin.POST("/rag-documents", gw.handleAdminCreateRAGDocument)
		admin.PUT("/rag-documents/:id", gw.handleAdminUpdateRAGDocument)
		admin.DELETE("/rag-documents/:id", gw.handleAdminDeleteRAGDocument)
		admin.POST("/rag-documents/batch-import", gw.handleAdminBatchImportRAGDocuments)
		admin.POST("/rag-documents/sync", gw.handleAdminSyncRAGDocuments)
		admin.POST("/rag-documents/sync-all", gw.handleAdminSyncAllPendingRAGDocuments)
		admin.GET("/scraper/sources", gw.handleAdminGetScraperSources)
		admin.POST("/scraper/search", gw.handleAdminScraperSearch)
		admin.POST("/scraper/fetch", gw.handleAdminScraperFetch)
		admin.POST("/scraper/clean", gw.handleAdminScraperClean)
		admin.POST("/scraper/import", gw.handleAdminScraperImport)
		admin.POST("/scraper/import/async", gw.handleAdminScraperImportAsync)
		admin.GET("/scraper/tasks", gw.handleAdminListScraperTasks)
		admin.GET("/scraper/tasks/:id", gw.handleAdminGetScraperTask)
		admin.POST("/scraper/tasks/:id/retry", gw.handleAdminRetryScraperTask)
		admin.GET("/configs/:key", gw.handleAdminGetConfig)
		admin.PUT("/configs/:key", gw.handleAdminSetConfig)
	}
}

// ========== 中间件 ==========

// JWTMiddleware JWT 认证中间件
func (gw *Gateway) JWTMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString, err := extractAccessToken(c.Request)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "missing authorization header"})
			c.Abort()
			return
		}

		claims, err := auth.ParseToken(tokenString, gw.jwtSecret)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			c.Abort()
			return
		}

		injectIdentityContext(c, claims, tokenString)
		c.Next()
	}
}

// OptionalJWTMiddleware 在保留匿名访问的同时尽量补齐已登录用户上下文。
func (gw *Gateway) OptionalJWTMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString, err := extractAccessToken(c.Request)
		if err == nil && tokenString != "" {
			if claims, parseErr := auth.ParseToken(tokenString, gw.jwtSecret); parseErr == nil {
				injectIdentityContext(c, claims, tokenString)
			}
		}
		c.Next()
	}
}

// AdminMiddleware 检查用户是否具有管理员角色
func (gw *Gateway) AdminMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, exists := c.Get("role")
		if !exists || role.(string) != "admin" {
			c.JSON(http.StatusForbidden, gin.H{"error": "admin access required"})
			c.Abort()
			return
		}
		c.Next()
	}
}

// extractAccessToken 统一提取 Bearer Token，并兼容 WebSocket 握手时通过 Query 透传 token。
func extractAccessToken(r *http.Request) (string, error) {
	authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
	if authHeader != "" {
		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == authHeader {
			return "", errors.New("Authorization格式错误，应为Bearer {token}")
		}
		if strings.TrimSpace(tokenString) == "" {
			return "", errors.New("Authorization格式错误，应为Bearer {token}")
		}
		return strings.TrimSpace(tokenString), nil
	}

	if isWebSocketUpgradeRequest(r) {
		for _, key := range []string{"token", "access_token"} {
			if token := strings.TrimSpace(r.URL.Query().Get(key)); token != "" {
				return token, nil
			}
		}
	}

	return "", errors.New("缺少Authorization请求头")
}

// isWebSocketUpgradeRequest 判断当前请求是否为 WebSocket 升级请求。
func isWebSocketUpgradeRequest(r *http.Request) bool {
	return strings.EqualFold(strings.TrimSpace(r.Header.Get("Upgrade")), "websocket")
}

// injectIdentityContext 将网关解析出的 JWT 声明和访问令牌写回请求上下文，供下游 gRPC 继续透传鉴权。
func injectIdentityContext(c *gin.Context, claims *auth.Claims, tokenString string) {
	userID := safeLegacyUserID(claims.UserID)
	c.Set("user_id", userID)
	c.Set("role", claims.Role)
	c.Set("email", claims.Email)

	ctx := context.WithValue(c.Request.Context(), auth.ContextKeyUserID, claims.UserID)
	ctx = context.WithValue(ctx, auth.ContextKeyRole, claims.Role)
	ctx = context.WithValue(ctx, auth.ContextKeyEmail, claims.Email)
	ctx = auth.WithAccessToken(ctx, tokenString)
	ctx = auth.WithOutgoingAccessToken(ctx, tokenString)
	c.Request = c.Request.WithContext(ctx)
}

// serviceAuthContext 为无用户 JWT 的下游调用补内部服务令牌。
func (gw *Gateway) serviceAuthContext(ctx context.Context) context.Context {
	return auth.WithOutgoingAccessToken(ctx, gw.serviceToken)
}

// safeLegacyUserID 将 JWT 中的 uint64 用户 ID 安全收敛为 legacy handler 可读取的 uint。
func safeLegacyUserID(userID uint64) uint {
	maxUint := ^uint(0)
	if uint64(maxUint) < userID {
		return maxUint
	}
	return uint(userID)
}

// registerSystemRoutes 注册与单体保持一致的基础健康检查与重定向路由。
func (gw *Gateway) registerSystemRoutes(engine *gin.Engine) {
	engine.GET("/api/health", gw.handleHealthLiveness)
	engine.GET("/api/health/ready", gw.handleHealthReadiness)
	engine.GET("/metrics", gin.WrapH(promhttp.Handler()))
	engine.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/api/health")
	})
}

// handleHealthLiveness 返回轻量级存活探针，响应结构与单体保持一致。
func (gw *Gateway) handleHealthLiveness(c *gin.Context) {
	legacySuccess(c, map[string]any{
		"status":    "ok",
		"version":   "",
		"timestamp": time.Now().Unix(),
	})
}

// handleHealthReadiness 返回 gateway 自身已完成启动的就绪状态，不再探测 bridge 数据库依赖。
func (gw *Gateway) handleHealthReadiness(c *gin.Context) {
	checks := map[string]string{
		"gateway": "ok",
	}

	legacySuccess(c, map[string]any{
		"status": "ok",
		"checks": checks,
	})
}

// legacyError 以单体相同结构返回错误响应。
func legacyError(c *gin.Context, httpStatus int, code int, message string, data interface{}) {
	c.JSON(httpStatus, legacyResponse{
		Code:    code,
		Message: message,
		Data:    data,
	})
}

// legacySuccess 以单体相同结构返回成功响应。
func legacySuccess(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, legacyResponse{
		Code:    0,
		Message: "success",
		Data:    data,
	})
}

// handleListPublicLive2DModels 返回前台可切换的 Live2D 模型列表，并兼容单体统一响应包裹结构。
func (gw *Gateway) handleListPublicLive2DModels(c *gin.Context) {
	resp, err := gw.adminClient.ListSelectableLive2DModels(c.Request.Context(), &adminv1.ListSelectableLive2DModelsRequest{
		Scene:        c.Query("scene"),
		IndustryCode: c.Query("industry_code"),
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	legacySuccess(c, resp.Models)
}

// handleGetPublicCurrentLive2DModel 返回前台当前场景默认使用的 Live2D 模型，并兼容单体统一响应包裹结构。
func (gw *Gateway) handleGetPublicCurrentLive2DModel(c *gin.Context) {
	resp, err := gw.adminClient.GetCurrentLive2DModel(c.Request.Context(), &adminv1.GetCurrentLive2DModelRequest{
		Scene:        c.Query("scene"),
		IndustryCode: c.Query("industry_code"),
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	legacySuccess(c, resp)
}

// ========== User 代理 ==========

func (gw *Gateway) handleRegister(c *gin.Context) {
	var req struct {
		Username string `json:"username"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Username == "" || req.Email == "" || req.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "username, email and password are required"})
		return
	}
	if !strings.Contains(req.Email, "@") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid email format"})
		return
	}
	if len(req.Password) < 6 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "password must be at least 6 characters"})
		return
	}
	resp, err := gw.userClient.Register(c.Request.Context(), &userv1.RegisterRequest{
		Username: req.Username, Email: req.Email, Password: req.Password,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleLogin(c *gin.Context) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, err := gw.userClient.Login(c.Request.Context(), &userv1.LoginRequest{
		Email: req.Email, Password: req.Password,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleRefreshToken(c *gin.Context) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, err := gw.userClient.RefreshToken(c.Request.Context(), &userv1.RefreshTokenRequest{
		RefreshToken: req.RefreshToken,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// handleLogout 透传登出请求，并将当前访问令牌一并交给 UserService 做会话清理。
func (gw *Gateway) handleLogout(c *gin.Context) {
	accessToken, err := extractAccessToken(c.Request)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing authorization header"})
		return
	}
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	_, err = gw.userClient.Logout(c.Request.Context(), &userv1.LogoutRequest{
		AccessToken:  accessToken,
		RefreshToken: strings.TrimSpace(req.RefreshToken),
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "logged_out"})
}

func (gw *Gateway) handleGetProfile(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	resp, err := gw.userClient.GetProfile(c.Request.Context(), &userv1.UserIDRequest{UserId: userID})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleUpdateProfile(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	var req struct {
		Username string `json:"username"`
		Avatar   string `json:"avatar"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, err := gw.userClient.UpdateProfile(c.Request.Context(), &userv1.UpdateProfileRequest{
		UserId: userID, Username: req.Username, Avatar: req.Avatar,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// handleMembershipInfo 读取当前用户的会员状态，并明确走 MembershipService 作为会员域的事实源。
func (gw *Gateway) handleMembershipInfo(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	resp, err := gw.membershipClient.GetMembershipStatus(c.Request.Context(), &membershipv1.UserIDRequest{UserId: userID})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// handleCreateOrder 创建会员订单，并兼容 `plan` 到 `plan_type` 的字段映射。
func (gw *Gateway) handleCreateOrder(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	var req struct {
		PlanType string `json:"plan_type"`
		Plan     string `json:"plan"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	planType := strings.TrimSpace(req.PlanType)
	if planType == "" {
		planType = strings.TrimSpace(req.Plan)
	}
	if planType == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "plan_type is required"})
		return
	}
	resp, err := gw.membershipClient.CreateOrder(c.Request.Context(), &membershipv1.CreateOrderRequest{
		UserId:   userID,
		PlanType: planType,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// handleCheckFeatureAccess 校验会员能力开通状态，补齐 `/api/v1/membership/check-access`。
func (gw *Gateway) handleCheckFeatureAccess(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	var req struct {
		Feature string `json:"feature"`
	}
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	feature := strings.TrimSpace(req.Feature)
	if feature == "" {
		feature = strings.TrimSpace(c.Query("feature"))
	}
	if feature == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "feature is required"})
		return
	}
	resp, err := gw.membershipClient.CheckFeatureAccess(c.Request.Context(), &membershipv1.CheckFeatureRequest{
		UserId:  userID,
		Feature: feature,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// handleListMembershipPlans 查询可用会员套餐列表。
func (gw *Gateway) handleListMembershipPlans(c *gin.Context) {
	resp, err := gw.membershipClient.ListPlans(c.Request.Context(), &membershipv1.ListPlansRequest{})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// handleListOrders 查询当前用户的订单列表。
func (gw *Gateway) handleListOrders(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	page, _ := strconv.ParseInt(c.DefaultQuery("page", "1"), 10, 32)
	pageSize, _ := strconv.ParseInt(c.DefaultQuery("page_size", "10"), 10, 32)
	resp, err := gw.membershipClient.ListOrders(c.Request.Context(), &membershipv1.ListOrdersRequest{
		UserId:   userID,
		Page:     int32(page),
		PageSize: int32(pageSize),
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// handleGetOrder 查询单个订单详情。
func (gw *Gateway) handleGetOrder(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	orderID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid order id"})
		return
	}
	resp, err := gw.membershipClient.GetOrder(c.Request.Context(), &membershipv1.GetOrderRequest{
		UserId:  userID,
		OrderId: orderID,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// handlePaymentCallback 处理支付回调（不需要 JWT 认证，由外部支付系统调用）。
func (gw *Gateway) handlePaymentCallback(c *gin.Context) {
	var req struct {
		OrderNo       string `json:"order_no"`
		Channel       string `json:"channel"`
		TransactionID string `json:"transaction_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.OrderNo == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "order_no is required"})
		return
	}
	resp, err := gw.membershipClient.HandlePaymentCallback(gw.serviceAuthContext(c.Request.Context()), &membershipv1.PaymentCallbackRequest{
		OrderNo:       req.OrderNo,
		Channel:       req.Channel,
		TransactionId: req.TransactionID,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleGetMembershipStatus(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	resp, err := gw.userClient.GetMembershipStatus(c.Request.Context(), &userv1.UserIDRequest{UserId: userID})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// handleUpgradeMembership 升级当前用户会员，并兼容旧版 `plan/plan_type` 到新 proto 契约的映射。
func (gw *Gateway) handleUpgradeMembership(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	var req struct {
		Level        string `json:"level"`
		DurationDays int32  `json:"duration_days"`
		Plan         string `json:"plan"`
		PlanType     string `json:"plan_type"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	level := strings.TrimSpace(req.Level)
	if level == "" {
		level = strings.TrimSpace(req.Plan)
	}
	if level == "" {
		level = strings.TrimSpace(req.PlanType)
	}
	if level == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "level is required"})
		return
	}

	durationDays := req.DurationDays
	if durationDays <= 0 {
		durationDays = defaultMembershipDurationDays(level)
	}
	if durationDays <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "duration_days is required"})
		return
	}

	resp, err := gw.membershipClient.UpgradeMembership(c.Request.Context(), &membershipv1.UpgradeRequest{
		UserId:       userID,
		Level:        level,
		DurationDays: durationDays,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// defaultMembershipDurationDays 根据套餐等级返回默认会员时长，兼容旧前端只传 plan_type 的场景。
func defaultMembershipDurationDays(level string) int32 {
	switch strings.TrimSpace(level) {
	case "monthly":
		return 30
	case "quarterly":
		return 90
	case "yearly":
		return 365
	default:
		return 0
	}
}

// ========== Question 代理 ==========

func (gw *Gateway) handleListQuestions(c *gin.Context) {
	page, _ := strconv.ParseInt(c.DefaultQuery("page", "1"), 10, 32)
	pageSize, _ := strconv.ParseInt(c.DefaultQuery("page_size", "20"), 10, 32)
	var categoryID uint64
	if cid := c.Query("category_id"); cid != "" {
		categoryID, _ = strconv.ParseUint(cid, 10, 64)
	}
	resp, err := gw.questionClient.ListQuestions(c.Request.Context(), &questionv1.ListQuestionsRequest{
		IndustryCode: c.Query("industry_code"),
		Difficulty:   c.Query("difficulty"),
		CategoryId:   categoryID,
		Keyword:      c.Query("keyword"),
		Page:         &sharedv1.PageParam{Page: int32(page), PageSize: int32(pageSize)},
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleGetQuestion(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	resp, err := gw.questionClient.GetQuestion(c.Request.Context(), &questionv1.GetQuestionRequest{Id: id})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// handleListQuestionSets 转发题单列表请求，补齐 `/api/v1/question-sets` 入口。
func (gw *Gateway) handleListQuestionSets(c *gin.Context) {
	page, _ := strconv.ParseInt(c.DefaultQuery("page", "1"), 10, 32)
	pageSize, _ := strconv.ParseInt(c.DefaultQuery("page_size", "20"), 10, 32)
	resp, err := gw.questionClient.ListQuestionSets(c.Request.Context(), &questionv1.ListQuestionSetsRequest{
		IndustryCode: c.Query("industry_code"),
		Page:         &sharedv1.PageParam{Page: int32(page), PageSize: int32(pageSize)},
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// handleGetQuestionSetDetail 读取指定题单详情，补齐 `/api/v1/question-sets/:id` 入口。
func (gw *Gateway) handleGetQuestionSetDetail(c *gin.Context) {
	setID, ok := parseID(c, "id")
	if !ok {
		return
	}
	resp, err := gw.questionClient.GetQuestionSetDetail(c.Request.Context(), &questionv1.GetQuestionSetDetailRequest{SetId: setID})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleListIndustries(c *gin.Context) {
	resp, err := gw.questionClient.ListIndustries(c.Request.Context(), &emptypb.Empty{})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleListCategories(c *gin.Context) {
	resp, err := gw.questionClient.ListCategories(c.Request.Context(), &questionv1.ListCategoriesRequest{
		IndustryCode: c.Query("industry_code"),
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleSubmitAnswer(c *gin.Context) {
	questionID, ok := parseID(c, "id")
	if !ok {
		return
	}
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	var req struct {
		Answer   string `json:"answer"`
		Language string `json:"language"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, err := gw.questionClient.SubmitAnswer(c.Request.Context(), &questionv1.SubmitAnswerRequest{
		QuestionId: questionID, UserId: userID, Answer: req.Answer, Language: req.Language,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleRunCode(c *gin.Context) {
	questionID, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req struct {
		Language string `json:"language"`
		Code     string `json:"code"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, err := gw.questionClient.RunCode(c.Request.Context(), &questionv1.RunCodeRequest{
		QuestionId: questionID, Language: req.Language, Code: req.Code,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// handleSubmitAnswerV1 兼容 `/api/v1/questions/submit` 的 body 传参形式。
func (gw *Gateway) handleSubmitAnswerV1(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	var req struct {
		QuestionID uint64 `json:"question_id"`
		Answer     string `json:"answer"`
		Language   string `json:"language"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.QuestionID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "question_id is required"})
		return
	}
	resp, err := gw.questionClient.SubmitAnswer(c.Request.Context(), &questionv1.SubmitAnswerRequest{
		QuestionId: req.QuestionID,
		UserId:     userID,
		Answer:     req.Answer,
		Language:   req.Language,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// handleRunCodeV1 兼容 `/api/v1/questions/run-code` 的 body 传参形式。
func (gw *Gateway) handleRunCodeV1(c *gin.Context) {
	var req struct {
		QuestionID uint64 `json:"question_id"`
		Language   string `json:"language"`
		Code       string `json:"code"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.QuestionID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "question_id is required"})
		return
	}
	resp, err := gw.questionClient.RunCode(c.Request.Context(), &questionv1.RunCodeRequest{
		QuestionId: req.QuestionID,
		Language:   req.Language,
		Code:       req.Code,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleToggleFavorite(c *gin.Context) {
	questionID, ok := parseID(c, "id")
	if !ok {
		return
	}
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	// 先尝试创建，如果已存在则删除（toggle 语义）
	_, err := gw.questionClient.CreateFavorite(c.Request.Context(), &questionv1.CreateFavoriteRequest{
		UserId: userID, QuestionId: questionID,
	})
	if err != nil {
		st, _ := status.FromError(err)
		if st.Code() == codes.AlreadyExists {
			_, err = gw.questionClient.DeleteFavorite(c.Request.Context(), &questionv1.DeleteFavoriteRequest{
				UserId: userID, QuestionId: questionID,
			})
			if err != nil {
				grpcErr(c, err)
				return
			}
			c.JSON(http.StatusOK, gin.H{"status": "removed"})
			return
		}
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "added"})
}

func (gw *Gateway) handleListFavorites(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	page, _ := strconv.ParseInt(c.DefaultQuery("page", "1"), 10, 32)
	pageSize, _ := strconv.ParseInt(c.DefaultQuery("page_size", "20"), 10, 32)
	resp, err := gw.questionClient.ListFavorites(c.Request.Context(), &questionv1.ListFavoritesRequest{
		UserId: userID,
		Page:   &sharedv1.PageParam{Page: int32(page), PageSize: int32(pageSize)},
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleGetWrongQuestions(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	page, _ := strconv.ParseInt(c.DefaultQuery("page", "1"), 10, 32)
	pageSize, _ := strconv.ParseInt(c.DefaultQuery("page_size", "20"), 10, 32)
	resp, err := gw.questionClient.GetWrongQuestions(c.Request.Context(), &questionv1.WrongQuestionRequest{
		UserId:       userID,
		IndustryCode: c.Query("industry_code"),
		Page:         &sharedv1.PageParam{Page: int32(page), PageSize: int32(pageSize)},
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleListNotes(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	page, _ := strconv.ParseInt(c.DefaultQuery("page", "1"), 10, 32)
	pageSize, _ := strconv.ParseInt(c.DefaultQuery("page_size", "20"), 10, 32)
	var questionID uint64
	if qid := c.Query("question_id"); qid != "" {
		questionID, _ = strconv.ParseUint(qid, 10, 64)
	}
	resp, err := gw.questionClient.ListNotes(c.Request.Context(), &questionv1.ListNotesRequest{
		UserId:     userID,
		QuestionId: questionID,
		Page:       &sharedv1.PageParam{Page: int32(page), PageSize: int32(pageSize)},
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleCreateNote(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	var req struct {
		QuestionID uint64 `json:"question_id"`
		Content    string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, err := gw.questionClient.CreateNote(c.Request.Context(), &questionv1.CreateNoteRequest{
		UserId: userID, QuestionId: req.QuestionID, Content: req.Content,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleUpdateNote(c *gin.Context) {
	noteID, ok := parseID(c, "id")
	if !ok {
		return
	}
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	var req struct {
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, err := gw.questionClient.UpdateNote(c.Request.Context(), &questionv1.UpdateNoteRequest{
		Id: noteID, UserId: userID, Content: req.Content,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// handleDeleteNote 删除当前用户的指定笔记，补齐 QuestionService 的删除能力。
func (gw *Gateway) handleDeleteNote(c *gin.Context) {
	noteID, ok := parseID(c, "id")
	if !ok {
		return
	}
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	_, err := gw.questionClient.DeleteNote(c.Request.Context(), &questionv1.DeleteNoteRequest{
		NoteId: noteID,
		UserId: userID,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

func (gw *Gateway) handleGetPracticeStats(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	resp, err := gw.questionClient.GetUserPracticeStats(c.Request.Context(), &questionv1.UserIDRequest{UserId: userID})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// handleGetPracticeRecommendations 透传练习推荐请求，并兼容 interview_id 查询参数。
func (gw *Gateway) handleGetPracticeRecommendations(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		return
	}

	var interviewID uint64
	if interviewIDValue := strings.TrimSpace(c.Query("interview_id")); interviewIDValue != "" {
		parsedInterviewID, err := strconv.ParseUint(interviewIDValue, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid interview_id"})
			return
		}
		interviewID = parsedInterviewID
	}

	resp, err := gw.questionClient.GetPracticeRecommendations(c.Request.Context(), &questionv1.PracticeRecommendationRequest{
		UserId:      userID,
		InterviewId: interviewID,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleGetRandomExam(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	var req struct {
		IndustryCode  string   `json:"industry_code"`
		QuestionCount int32    `json:"question_count"`
		Categories    []string `json:"categories"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, err := gw.questionClient.GetRandomExam(c.Request.Context(), &questionv1.RandomExamRequest{
		UserId: userID, IndustryCode: req.IndustryCode,
		QuestionCount: req.QuestionCount, Categories: req.Categories,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// handleListMistakeTopics 返回当前用户的错题主题聚合，补齐 `/api/v1/mistakes/topics` 入口。
func (gw *Gateway) handleListMistakeTopics(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	limit, _ := strconv.ParseInt(c.DefaultQuery("limit", "10"), 10, 32)
	resp, err := gw.questionClient.ListMistakeTopics(c.Request.Context(), &questionv1.ListMistakeTopicsRequest{
		UserId: userID,
		Limit:  int32(limit),
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// handleGetMistakeTopic 根据错因专题编码查询单个专题卡片详情。
func (gw *Gateway) handleGetMistakeTopic(c *gin.Context) {
	code := c.Param("code")
	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "code is required"})
		return
	}
	resp, err := gw.questionClient.GetMistakeTopic(c.Request.Context(), &questionv1.GetMistakeTopicRequest{
		Code: code,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// handleGenerateTimedExam 补齐限时练习入口，并兼容 count 到 question_count 的别名映射。
func (gw *Gateway) handleGenerateTimedExam(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	var req struct {
		IndustryCode     string `json:"industry_code"`
		QuestionCount    int32  `json:"question_count"`
		Count            int32  `json:"count"`
		TimeLimitMinutes int32  `json:"time_limit_minutes"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	questionCount := req.QuestionCount
	if questionCount == 0 {
		questionCount = req.Count
	}
	resp, err := gw.questionClient.GenerateTimedExam(c.Request.Context(), &questionv1.GenerateTimedExamRequest{
		UserId:           userID,
		IndustryCode:     req.IndustryCode,
		QuestionCount:    questionCount,
		TimeLimitMinutes: req.TimeLimitMinutes,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// handleSubmitExam 提交限时练习答案，并将 JSON map 键转换为 proto 需要的 uint64 map。
func (gw *Gateway) handleSubmitExam(c *gin.Context) {
	examID, ok := parseID(c, "id")
	if !ok {
		return
	}
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	var req struct {
		Answers map[string]string `json:"answers"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	answers := make(map[uint64]string, len(req.Answers))
	for questionIDText, answer := range req.Answers {
		questionID, err := strconv.ParseUint(questionIDText, 10, 64)
		if err != nil || questionID == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid answers question id"})
			return
		}
		answers[questionID] = answer
	}
	resp, err := gw.questionClient.SubmitExam(c.Request.Context(), &questionv1.SubmitExamRequest{
		ExamId:  examID,
		UserId:  userID,
		Answers: answers,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// ========== Interview 代理 ==========

func (gw *Gateway) handleCreateInterview(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	var req struct {
		IndustryCode   string   `json:"industry_code"`
		Difficulty     string   `json:"difficulty"`
		Topics         []string `json:"topics"`
		QuestionCount  int32    `json:"question_count"`
		InterviewMode  string   `json:"interview_mode"`
		ResumeText     string   `json:"resume_text"`
		JobDescription string   `json:"job_description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, err := gw.interviewClient.CreateInterview(c.Request.Context(), &interviewv1.CreateInterviewRequest{
		UserId: userID, IndustryCode: req.IndustryCode, Difficulty: req.Difficulty,
		Topics: req.Topics, QuestionCount: req.QuestionCount, InterviewMode: req.InterviewMode,
		ResumeText: req.ResumeText, JobDescription: req.JobDescription,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleListInterviews(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	page, _ := strconv.ParseInt(c.DefaultQuery("page", "1"), 10, 32)
	pageSize, _ := strconv.ParseInt(c.DefaultQuery("page_size", "20"), 10, 32)
	resp, err := gw.interviewClient.ListInterviews(c.Request.Context(), &interviewv1.ListInterviewsRequest{
		UserId: userID,
		Page:   &sharedv1.PageParam{Page: int32(page), PageSize: int32(pageSize)},
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleGetInterview(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	resp, err := gw.interviewClient.GetInterview(c.Request.Context(), &interviewv1.GetInterviewRequest{
		InterviewId: id, UserId: userID,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleSubmitInterviewAnswer(c *gin.Context) {
	interviewID, ok := parseID(c, "id")
	if !ok {
		return
	}
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	var req struct {
		QuestionIndex int32  `json:"question_index"`
		Answer        string `json:"answer"`
		Language      string `json:"language"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, err := gw.interviewClient.SubmitAnswer(c.Request.Context(), &interviewv1.SubmitAnswerRequest{
		InterviewId: interviewID, UserId: userID,
		QuestionIndex: req.QuestionIndex, Answer: req.Answer, Language: req.Language,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleGetNextQuestion(c *gin.Context) {
	interviewID, ok := parseID(c, "id")
	if !ok {
		return
	}
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	currentIndex, _ := strconv.ParseInt(c.DefaultQuery("current_index", "0"), 10, 32)
	resp, err := gw.interviewClient.GetNextQuestion(c.Request.Context(), &interviewv1.GetNextQuestionRequest{
		InterviewId: interviewID, UserId: userID, CurrentIndex: int32(currentIndex),
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// handleGetNextQuestionV1 兼容 `/api/v1/interviews/:id/next-question` 的 POST 触发方式。
func (gw *Gateway) handleGetNextQuestionV1(c *gin.Context) {
	interviewID, ok := parseID(c, "id")
	if !ok {
		return
	}
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	var req struct {
		CurrentIndex int32 `json:"current_index"`
	}
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, err := gw.interviewClient.GetNextQuestion(c.Request.Context(), &interviewv1.GetNextQuestionRequest{
		InterviewId:  interviewID,
		UserId:       userID,
		CurrentIndex: req.CurrentIndex,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleFinishInterview(c *gin.Context) {
	interviewID, ok := parseID(c, "id")
	if !ok {
		return
	}
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	resp, err := gw.interviewClient.FinishInterview(c.Request.Context(), &interviewv1.FinishInterviewRequest{
		InterviewId: interviewID, UserId: userID,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// handleProxyRealtimeInterviewWS 校验实时面试资格后，将前端 WebSocket 透明代理到 Realtime Service。
func (gw *Gateway) handleProxyRealtimeInterviewWS(c *gin.Context) {
	interviewID, ok := parseID(c, "id")
	if !ok {
		return
	}
	if gw.interviewClient == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "service unavailable", "service": "interview"})
		return
	}
	if strings.TrimSpace(gw.realtimeWSAddr) == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "service unavailable", "service": "realtime"})
		return
	}
	accessToken, err := extractAccessToken(c.Request)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing authorization header"})
		return
	}
	realtimeCheck, err := gw.interviewClient.IsRealtimeInterview(c.Request.Context(), &interviewv1.IsRealtimeRequest{
		InterviewId: interviewID,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	if !realtimeCheck.GetIsRealtime() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "interview is not realtime"})
		return
	}
	upstreamURL, err := gw.buildRealtimeInterviewWSURL(interviewID, accessToken, strings.TrimSpace(c.Query("session_id")))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid realtime target"})
		return
	}
	headers := http.Header{}
	headers.Set("Authorization", "Bearer "+accessToken)
	upstreamConn, _, err := websocket.DefaultDialer.DialContext(c.Request.Context(), upstreamURL, headers)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to connect realtime service"})
		return
	}
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}
	clientConn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		_ = upstreamConn.Close()
		return
	}
	gw.proxyWebSocketTraffic(clientConn, upstreamConn)
}

// buildRealtimeInterviewWSURL 组装 Realtime Service 的目标握手地址，并透传 token 与 session_id。
func (gw *Gateway) buildRealtimeInterviewWSURL(interviewID uint64, accessToken string, sessionID string) (string, error) {
	base := strings.TrimSpace(gw.realtimeWSAddr)
	if base == "" {
		return "", errors.New("missing realtime address")
	}
	if !strings.Contains(base, "://") {
		base = "ws://" + base
	}
	target, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	switch target.Scheme {
	case "http":
		target.Scheme = "ws"
	case "https":
		target.Scheme = "wss"
	case "ws", "wss":
	default:
		return "", errors.New("unsupported realtime scheme")
	}
	target.Path = strings.TrimRight(target.Path, "/") + "/ws/interview/" + strconv.FormatUint(interviewID, 10)
	query := target.Query()
	query.Set("token", accessToken)
	if sessionID != "" {
		query.Set("session_id", sessionID)
	}
	target.RawQuery = query.Encode()
	return target.String(), nil
}

// proxyWebSocketTraffic 双向转发前端与 Realtime Service 的 WebSocket 帧，并在任一侧结束时清理连接。
func (gw *Gateway) proxyWebSocketTraffic(clientConn *websocket.Conn, upstreamConn *websocket.Conn) {
	defer clientConn.Close()
	defer upstreamConn.Close()

	var once sync.Once
	closeBoth := func() {
		_ = clientConn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		_ = upstreamConn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		_ = clientConn.Close()
		_ = upstreamConn.Close()
	}
	errCh := make(chan struct{}, 2)
	go pipeWebSocketMessages(clientConn, upstreamConn, &once, closeBoth, errCh)
	go pipeWebSocketMessages(upstreamConn, clientConn, &once, closeBoth, errCh)
	<-errCh
	once.Do(closeBoth)
}

// pipeWebSocketMessages 按原始消息类型转发单向 WebSocket 帧，任何读写失败都触发代理结束。
func pipeWebSocketMessages(src *websocket.Conn, dst *websocket.Conn, once *sync.Once, closeBoth func(), errCh chan<- struct{}) {
	for {
		messageType, payload, err := src.ReadMessage()
		if err != nil {
			once.Do(closeBoth)
			errCh <- struct{}{}
			return
		}
		if err := dst.WriteMessage(messageType, payload); err != nil {
			once.Do(closeBoth)
			errCh <- struct{}{}
			return
		}
	}
}

func (gw *Gateway) handleGetReport(c *gin.Context) {
	interviewID, ok := parseID(c, "id")
	if !ok {
		return
	}
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	resp, err := gw.interviewClient.GetReport(c.Request.Context(), &interviewv1.GetReportRequest{
		InterviewId: interviewID, UserId: userID,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleSubmitCodingAnswer(c *gin.Context) {
	interviewID, ok := parseID(c, "id")
	if !ok {
		return
	}
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	var req struct {
		QuestionIndex int32  `json:"question_index"`
		Language      string `json:"language"`
		Code          string `json:"code"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, err := gw.interviewClient.SubmitCodingAnswer(c.Request.Context(), &interviewv1.SubmitCodingRequest{
		InterviewId: interviewID, UserId: userID,
		QuestionIndex: req.QuestionIndex, Language: req.Language, Code: req.Code,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// ========== Growth 代理 ==========

func (gw *Gateway) handleGetGrowthSummary(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	resp, err := gw.growthClient.GetGrowthSummary(c.Request.Context(), &growthv1.UserIDRequest{UserId: userID})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleGetWeeklyFocus(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	resp, err := gw.growthClient.GetWeeklyFocus(c.Request.Context(), &growthv1.UserIDRequest{UserId: userID})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleSyncStudyLog(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	var req struct {
		Action          string `json:"action"`
		RefID           uint64 `json:"ref_id"`
		DurationSeconds int32  `json:"duration_seconds"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, err := gw.growthClient.SyncStudyLog(c.Request.Context(), &growthv1.SyncStudyLogRequest{
		UserId: userID, Action: req.Action, RefId: req.RefID, DurationSeconds: req.DurationSeconds,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// handleCreatePlan 按当前 PlanService proto 组装建计划请求，并兼容旧 HTTP 字段别名。
// handleSyncStudyLogV1 兼容 `/api/v1/growth/study-log` 的新入口，并复用既有成长日志同步逻辑。
func (gw *Gateway) handleSyncStudyLogV1(c *gin.Context) {
	gw.handleSyncStudyLog(c)
}

func (gw *Gateway) handleCreatePlan(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	var req struct {
		WeakTopics        []string `json:"weak_topics"`
		Level             string   `json:"level"`
		DurationDays      int32    `json:"duration_days"`
		Industry          string   `json:"industry"`
		IndustryCode      string   `json:"industry_code"`
		DailyStudyMinutes int32    `json:"daily_study_minutes"`
		DailyHours        int32    `json:"daily_hours"`
		GoalDescription   string   `json:"goal_description"`
		Goal              string   `json:"goal"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	industry := strings.TrimSpace(req.Industry)
	if industry == "" {
		industry = strings.TrimSpace(req.IndustryCode)
	}
	goalDescription := strings.TrimSpace(req.GoalDescription)
	if goalDescription == "" {
		goalDescription = strings.TrimSpace(req.Goal)
	}
	dailyStudyMinutes := req.DailyStudyMinutes
	if dailyStudyMinutes == 0 && req.DailyHours > 0 {
		dailyStudyMinutes = req.DailyHours * 60
	}

	resp, err := gw.planClient.CreatePlan(c.Request.Context(), &planv1.CreatePlanRequest{
		UserId:            userID,
		WeakTopics:        req.WeakTopics,
		Level:             strings.TrimSpace(req.Level),
		DurationDays:      req.DurationDays,
		Industry:          industry,
		DailyStudyMinutes: dailyStudyMinutes,
		GoalDescription:   goalDescription,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// handleGetCurrentPlan 查询当前用户的活跃学习计划。
// handleListPlans 返回当前用户的计划列表，补齐 `/api/v1/plans` 查询入口。
func (gw *Gateway) handleListPlans(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	page, _ := strconv.ParseInt(c.DefaultQuery("page", "1"), 10, 32)
	pageSize, _ := strconv.ParseInt(c.DefaultQuery("page_size", "20"), 10, 32)
	resp, err := gw.planClient.ListPlans(c.Request.Context(), &planv1.ListPlansRequest{
		UserId:   userID,
		Page:     int32(page),
		PageSize: int32(pageSize),
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleGetCurrentPlan(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	resp, err := gw.planClient.GetCurrentPlan(c.Request.Context(), &planv1.GetCurrentPlanRequest{
		UserId: userID,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// handleGetPlan 查询指定学习计划详情。
func (gw *Gateway) handleGetPlan(c *gin.Context) {
	planID, ok := parseID(c, "id")
	if !ok {
		return
	}
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	resp, err := gw.planClient.GetPlan(c.Request.Context(), &planv1.GetPlanRequest{
		PlanId: planID, UserId: userID,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// handleGetPlanProgress 获取学习计划进度统计。
func (gw *Gateway) handleGetPlanProgress(c *gin.Context) {
	planID, ok := parseID(c, "id")
	if !ok {
		return
	}
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	resp, err := gw.planClient.GetProgress(c.Request.Context(), &planv1.GetProgressRequest{
		PlanId: planID, UserId: userID,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// handleUpdateTaskStatus 透传任务状态更新，并补齐 plan_id 路径参数。
func (gw *Gateway) handleUpdateTaskStatus(c *gin.Context) {
	planID, ok := parseID(c, "id")
	if !ok {
		return
	}
	taskID, ok := parseID(c, "taskId")
	if !ok {
		return
	}
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	var req struct {
		Status string `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, err := gw.planClient.UpdateTaskStatus(c.Request.Context(), &planv1.UpdateTaskStatusRequest{
		PlanId: planID, TaskId: taskID, UserId: userID, Status: req.Status,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// handleSubmitTaskFeedback 按当前 PlanService proto 提交任务反馈，并兼容旧字段别名。
// handleUpdateTaskStatusV1 兼容 `/api/v1/plans/:id/tasks/:tid/status` 的新参数名。
func (gw *Gateway) handleUpdateTaskStatusV1(c *gin.Context) {
	c.Params = append(c.Params, gin.Param{Key: "taskId", Value: c.Param("tid")})
	gw.handleUpdateTaskStatus(c)
}

func (gw *Gateway) handleSubmitTaskFeedback(c *gin.Context) {
	planID, ok := parseID(c, "id")
	if !ok {
		return
	}
	taskID, ok := parseID(c, "taskId")
	if !ok {
		return
	}
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	var req struct {
		DifficultyFeeling  string   `json:"difficulty_feeling"`
		FeedbackText       string   `json:"feedback_text"`
		ActualDurationMins int32    `json:"actual_duration_minutes"`
		ProblemAreas       []string `json:"problem_areas"`
		Content            string   `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	feedbackText := strings.TrimSpace(req.FeedbackText)
	if feedbackText == "" {
		feedbackText = strings.TrimSpace(req.Content)
	}

	resp, err := gw.planClient.SubmitTaskFeedback(c.Request.Context(), &planv1.SubmitFeedbackRequest{
		PlanId:                planID,
		TaskId:                taskID,
		UserId:                userID,
		DifficultyFeeling:     strings.TrimSpace(req.DifficultyFeeling),
		FeedbackText:          feedbackText,
		ActualDurationMinutes: req.ActualDurationMins,
		ProblemAreas:          req.ProblemAreas,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// handleCompanionChat 将聊天消息转发到 CompanionService。
// handleSubmitTaskFeedbackV1 兼容 `/api/v1/plans/:id/tasks/:tid/feedback` 的新参数名。
func (gw *Gateway) handleSubmitTaskFeedbackV1(c *gin.Context) {
	c.Params = append(c.Params, gin.Param{Key: "taskId", Value: c.Param("tid")})
	gw.handleSubmitTaskFeedback(c)
}

// handleAdjustPlan 触发学习计划调整，允许前端仅提交空对象也能走默认调整流程。
func (gw *Gateway) handleAdjustPlan(c *gin.Context) {
	planID, ok := parseID(c, "id")
	if !ok {
		return
	}
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	var req struct {
		Reason string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, err := gw.planClient.AdjustPlan(c.Request.Context(), &planv1.AdjustPlanRequest{
		PlanId: planID,
		Reason: strings.TrimSpace(req.Reason),
		UserId: userID,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleCompanionChat(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	var req struct {
		Message     string `json:"message"`
		ContextType string `json:"context_type"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, err := gw.companionClient.Chat(c.Request.Context(), &companionv1.CompanionChatRequest{
		UserId: userID, Message: req.Message, ContextType: req.ContextType,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// ========== Community 代理 ==========

// handleGetCompanionState 读取当前用户的陪伴状态快照，补齐 `/api/v1/companion/state`。
func (gw *Gateway) handleGetCompanionState(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	resp, err := gw.companionClient.GetCompanionState(c.Request.Context(), &companionv1.GetCompanionStateRequest{UserId: userID})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// handleSynthesizeSpeech 透传陪伴文案 TTS 合成请求，补齐 `/api/v1/companion/tts`。
func (gw *Gateway) handleSynthesizeSpeech(c *gin.Context) {
	var req struct {
		Text  string `json:"text"`
		Voice string `json:"voice"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, err := gw.companionClient.SynthesizeSpeech(c.Request.Context(), &companionv1.SynthesizeSpeechRequest{
		Text:  req.Text,
		Voice: req.Voice,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleListPosts(c *gin.Context) {
	page, _ := strconv.ParseInt(c.DefaultQuery("page", "1"), 10, 32)
	pageSize, _ := strconv.ParseInt(c.DefaultQuery("page_size", "20"), 10, 32)
	resp, err := gw.communityClient.ListPosts(c.Request.Context(), &communityv1.ListPostsRequest{
		Category: c.Query("category"),
		Page:     &sharedv1.PageParam{Page: int32(page), PageSize: int32(pageSize)},
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleGetPost(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	resp, err := gw.communityClient.GetPost(c.Request.Context(), &communityv1.GetPostRequest{Id: id})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleCreatePost(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	var req struct {
		Title    string `json:"title"`
		Content  string `json:"content"`
		Category string `json:"category"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Title == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "title is required"})
		return
	}
	resp, err := gw.communityClient.CreatePost(c.Request.Context(), &communityv1.CreatePostRequest{
		AuthorId: userID, Title: req.Title, Content: req.Content, Category: req.Category,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleUpdatePost(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	var req struct {
		Title   string `json:"title"`
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, err := gw.communityClient.UpdatePost(c.Request.Context(), &communityv1.UpdatePostRequest{
		Id: id, AuthorId: userID, Title: req.Title, Content: req.Content,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleDeletePost(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	_, err := gw.communityClient.DeletePost(c.Request.Context(), &communityv1.DeletePostRequest{Id: id, AuthorId: userID})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

func (gw *Gateway) handleListComments(c *gin.Context) {
	postID, ok := parseID(c, "id")
	if !ok {
		return
	}
	page, _ := strconv.ParseInt(c.DefaultQuery("page", "1"), 10, 32)
	pageSize, _ := strconv.ParseInt(c.DefaultQuery("page_size", "20"), 10, 32)
	resp, err := gw.communityClient.ListComments(c.Request.Context(), &communityv1.ListCommentsRequest{
		PostId: postID,
		Page:   &sharedv1.PageParam{Page: int32(page), PageSize: int32(pageSize)},
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleCreateComment(c *gin.Context) {
	postID, ok := parseID(c, "id")
	if !ok {
		return
	}
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	var req struct {
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, err := gw.communityClient.CreateComment(c.Request.Context(), &communityv1.CreateCommentRequest{
		PostId: postID, AuthorId: userID, Content: req.Content,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleToggleLike(c *gin.Context) {
	postID, ok := parseID(c, "id")
	if !ok {
		return
	}
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	resp, err := gw.communityClient.ToggleLike(c.Request.Context(), &communityv1.ToggleLikeRequest{
		UserId: userID, PostId: postID,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// ========== Admin 代理 ==========

// handleListMyPosts 返回当前用户发布的帖子列表，补齐 `/api/v1/community/my/posts`。
func (gw *Gateway) handleListMyPosts(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		return
	}
	page, _ := strconv.ParseInt(c.DefaultQuery("page", "1"), 10, 32)
	pageSize, _ := strconv.ParseInt(c.DefaultQuery("page_size", "20"), 10, 32)
	resp, err := gw.communityClient.ListMyPosts(c.Request.Context(), &communityv1.ListMyPostsRequest{
		AuthorId: userID,
		Page:     &sharedv1.PageParam{Page: int32(page), PageSize: int32(pageSize)},
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleAdminListUsers(c *gin.Context) {
	page, _ := strconv.ParseInt(c.DefaultQuery("page", "1"), 10, 32)
	pageSize, _ := strconv.ParseInt(c.DefaultQuery("page_size", "20"), 10, 32)
	resp, err := gw.adminClient.ListUsers(c.Request.Context(), &adminv1.ListUsersRequest{
		Keyword: c.Query("keyword"),
		Page:    &sharedv1.PageParam{Page: int32(page), PageSize: int32(pageSize)},
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleAdminUpdateUserRole(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req struct {
		Role string `json:"role"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	_, err := gw.adminClient.UpdateUserRole(c.Request.Context(), &adminv1.UpdateUserRoleRequest{
		UserId: id, Role: req.Role,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "updated"})
}

func (gw *Gateway) handleAdminListAIPresets(c *gin.Context) {
	resp, err := gw.adminClient.ListAIPresets(c.Request.Context(), &emptypb.Empty{})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleAdminSaveAIPreset(c *gin.Context) {
	var req struct {
		ID       uint64            `json:"id"`
		Name     string            `json:"name"`
		Provider string            `json:"provider"`
		Model    string            `json:"model"`
		Params   map[string]string `json:"params"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, err := gw.adminClient.SaveAIPreset(c.Request.Context(), &adminv1.SaveAIPresetRequest{
		Id: req.ID, Name: req.Name, Provider: req.Provider, Model: req.Model, Params: req.Params,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleAdminListPromptTemplates(c *gin.Context) {
	resp, err := gw.adminClient.ListPromptTemplates(c.Request.Context(), &adminv1.ListPromptTemplatesRequest{
		IndustryCode: c.Query("industry_code"),
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleAdminSavePromptTemplate(c *gin.Context) {
	var req struct {
		ID           uint64 `json:"id"`
		Name         string `json:"name"`
		IndustryCode string `json:"industry_code"`
		TemplateType string `json:"template_type"`
		Content      string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, err := gw.adminClient.SavePromptTemplate(c.Request.Context(), &adminv1.SavePromptTemplateRequest{
		Id: req.ID, Name: req.Name, IndustryCode: req.IndustryCode,
		TemplateType: req.TemplateType, Content: req.Content,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleAdminGetConfig(c *gin.Context) {
	key := c.Param("key")
	resp, err := gw.adminClient.GetAdminConfig(c.Request.Context(), &adminv1.GetAdminConfigRequest{Key: key})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleAdminSetConfig(c *gin.Context) {
	key := c.Param("key")
	var req struct {
		Value string `json:"value"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	_, err := gw.adminClient.SetAdminConfig(c.Request.Context(), &adminv1.SetAdminConfigRequest{
		Key: key, Value: req.Value,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "updated"})
}

func (gw *Gateway) handleAdminDebugAI(c *gin.Context) {
	var req struct {
		AgentType string            `json:"agent_type"`
		Prompt    string            `json:"prompt"`
		Params    map[string]string `json:"params"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, err := gw.adminClient.DebugAI(c.Request.Context(), &adminv1.DebugAIRequest{
		AgentType: req.AgentType, Prompt: req.Prompt, Params: req.Params,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// handleAdminListAICallLogs 在无 bridge 直挂时，将单体后台已有的日志筛选参数透传给 admin gRPC。
func (gw *Gateway) handleAdminListAICallLogs(c *gin.Context) {
	page, _ := strconv.ParseInt(c.DefaultQuery("page", "1"), 10, 32)
	pageSize, _ := strconv.ParseInt(c.DefaultQuery("page_size", "20"), 10, 32)

	var taskID uint64
	if rawTaskID := strings.TrimSpace(c.Query("task_id")); rawTaskID != "" {
		parsedTaskID, err := strconv.ParseUint(rawTaskID, 10, 64)
		if err != nil || parsedTaskID == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid task_id"})
			return
		}
		taskID = parsedTaskID
	}
	resp, err := gw.adminClient.ListAICallLogs(c.Request.Context(), &adminv1.ListAICallLogsRequest{
		Page:      &sharedv1.PageParam{Page: int32(page), PageSize: int32(pageSize)},
		AgentType: c.Query("agent_type"),
		Scene:     c.Query("scene"),
		Source:    c.Query("source"),
		Status:    c.Query("status"),
		TraceId:   c.Query("trace_id"),
		TaskId:    taskID,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// ==================== Admin: 仪表盘 ====================

func (gw *Gateway) handleAdminGetDashboard(c *gin.Context) {
	resp, err := gw.adminClient.GetDashboard(c.Request.Context(), &emptypb.Empty{})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// ==================== Admin: 用户管理 ====================

func (gw *Gateway) handleAdminDisableUser(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	_, err := gw.adminClient.DisableUser(c.Request.Context(), &adminv1.DisableUserRequest{UserId: id})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "updated"})
}

// ==================== Admin: 题库管理 ====================

func (gw *Gateway) handleAdminListQuestions(c *gin.Context) {
	page, _ := strconv.ParseInt(c.DefaultQuery("page", "1"), 10, 32)
	pageSize, _ := strconv.ParseInt(c.DefaultQuery("page_size", "20"), 10, 32)
	var categoryID uint64
	if cid := c.Query("category_id"); cid != "" {
		categoryID, _ = strconv.ParseUint(cid, 10, 64)
	}
	resp, err := gw.adminClient.AdminListQuestions(c.Request.Context(), &adminv1.AdminListQuestionsRequest{
		Page:         &sharedv1.PageParam{Page: int32(page), PageSize: int32(pageSize)},
		Keyword:      c.Query("keyword"),
		Difficulty:   c.Query("difficulty"),
		CategoryId:   categoryID,
		IndustryCode: c.Query("industry_code"),
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleAdminCreateQuestion(c *gin.Context) {
	var req struct {
		CategoryID         uint64 `json:"category_id"`
		IndustryID         uint64 `json:"industry_id"`
		Type               string `json:"type"`
		Difficulty         string `json:"difficulty"`
		Title              string `json:"title"`
		Content            string `json:"content"`
		OptionsJSON        string `json:"options_json"`
		Answer             string `json:"answer"`
		Explanation        string `json:"explanation"`
		SolutionJSON       string `json:"solution_json"`
		JudgeConfigJSON    string `json:"judge_config_json"`
		AnswerTemplateJSON string `json:"answer_template_json"`
		Tags               string `json:"tags"`
		IsActive           bool   `json:"is_active"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, err := gw.adminClient.CreateQuestion(c.Request.Context(), &adminv1.CreateQuestionRequest{
		CategoryId: req.CategoryID, IndustryId: req.IndustryID, Type: req.Type,
		Difficulty: req.Difficulty, Title: req.Title, Content: req.Content,
		OptionsJson: req.OptionsJSON, Answer: req.Answer, Explanation: req.Explanation,
		SolutionJson: req.SolutionJSON, JudgeConfigJson: req.JudgeConfigJSON,
		AnswerTemplateJson: req.AnswerTemplateJSON, Tags: req.Tags, IsActive: req.IsActive,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleAdminUpdateQuestion(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req struct {
		CategoryID         uint64 `json:"category_id"`
		IndustryID         uint64 `json:"industry_id"`
		Type               string `json:"type"`
		Difficulty         string `json:"difficulty"`
		Title              string `json:"title"`
		Content            string `json:"content"`
		OptionsJSON        string `json:"options_json"`
		Answer             string `json:"answer"`
		Explanation        string `json:"explanation"`
		SolutionJSON       string `json:"solution_json"`
		JudgeConfigJSON    string `json:"judge_config_json"`
		AnswerTemplateJSON string `json:"answer_template_json"`
		Tags               string `json:"tags"`
		IsActive           *bool  `json:"is_active"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	_, err := gw.adminClient.UpdateQuestion(c.Request.Context(), &adminv1.UpdateQuestionRequest{
		Id: id, CategoryId: req.CategoryID, IndustryId: req.IndustryID,
		Type: req.Type, Difficulty: req.Difficulty, Title: req.Title,
		Content: req.Content, OptionsJson: req.OptionsJSON, Answer: req.Answer,
		Explanation: req.Explanation, SolutionJson: req.SolutionJSON,
		JudgeConfigJson: req.JudgeConfigJSON, AnswerTemplateJson: req.AnswerTemplateJSON,
		Tags: req.Tags, IsActive: req.IsActive,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "updated"})
}

func (gw *Gateway) handleAdminDeleteQuestion(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	_, err := gw.adminClient.DeleteQuestion(c.Request.Context(), &adminv1.DeleteQuestionRequest{Id: id})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

func (gw *Gateway) handleAdminBatchImportQuestions(c *gin.Context) {
	var req struct {
		IndustryCode string `json:"industry_code"`
		Questions    []struct {
			CategoryName       string `json:"category_name"`
			Type               string `json:"type"`
			Difficulty         string `json:"difficulty"`
			Title              string `json:"title"`
			Content            string `json:"content"`
			OptionsJSON        string `json:"options_json"`
			Answer             string `json:"answer"`
			Explanation        string `json:"explanation"`
			SolutionJSON       string `json:"solution_json"`
			JudgeConfigJSON    string `json:"judge_config_json"`
			AnswerTemplateJSON string `json:"answer_template_json"`
			Tags               string `json:"tags"`
		} `json:"questions"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	items := make([]*adminv1.ImportQuestionItem, len(req.Questions))
	for i, q := range req.Questions {
		items[i] = &adminv1.ImportQuestionItem{
			CategoryName: q.CategoryName, Type: q.Type, Difficulty: q.Difficulty,
			Title: q.Title, Content: q.Content, OptionsJson: q.OptionsJSON,
			Answer: q.Answer, Explanation: q.Explanation, SolutionJson: q.SolutionJSON,
			JudgeConfigJson: q.JudgeConfigJSON, AnswerTemplateJson: q.AnswerTemplateJSON, Tags: q.Tags,
		}
	}
	resp, err := gw.adminClient.BatchImportQuestions(c.Request.Context(), &adminv1.BatchImportQuestionsRequest{
		IndustryCode: req.IndustryCode, Questions: items,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleAdminGetQuestionTagTaxonomy(c *gin.Context) {
	resp, err := gw.adminClient.GetQuestionTagTaxonomy(c.Request.Context(), &emptypb.Empty{})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// ==================== Admin: 题目流水线 ====================

// bindQuestionPipelineGenerateRequest 解析题目流水线请求体，并统一复用到同步、异步和 SSE 入口。
func bindQuestionPipelineGenerateRequest(c *gin.Context) (*adminv1.GenerateQuestionPipelineRequest, error) {
	var req questionPipelineGeneratePayload
	if err := c.ShouldBindJSON(&req); err != nil {
		return nil, err
	}
	return req.toProto(), nil
}

// startQuestionPipelineSSE 初始化题目流水线的 SSE 响应头，并返回可刷新的写入器。
func startQuestionPipelineSSE(c *gin.Context) (http.Flusher, bool) {
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "streaming is not supported"})
		return nil, false
	}
	headers := c.Writer.Header()
	headers.Set("Content-Type", "text/event-stream; charset=utf-8")
	headers.Set("Cache-Control", "no-cache, no-transform")
	headers.Set("Connection", "keep-alive")
	headers.Set("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)
	c.Writer.WriteHeaderNow()
	if err := writeQuestionPipelineSSEPrelude(c, flusher); err != nil {
		return nil, false
	}
	return flusher, true
}

// normalizeQuestionPipelineGenerateResponsePayload 将 protobuf 响应转换为前端流式处理复用的 snake_case JSON 结构。
func normalizeQuestionPipelineGenerateResponsePayload(resp *adminv1.GenerateQuestionPipelineResponse) map[string]interface{} {
	if resp == nil {
		return map[string]interface{}{}
	}
	raw, err := json.Marshal(resp)
	if err != nil {
		return map[string]interface{}{}
	}
	normalized, ok := normalizeGatewayResponse("/api/v1/admin/question-pipeline/generate", raw).(map[string]interface{})
	if !ok {
		return map[string]interface{}{}
	}
	return normalized
}

// streamQuestionPipelineGenerateResponse 将同步生成结果拆成前端已有的 SSE 事件序列，避免额外改动页面协议。
func streamQuestionPipelineGenerateResponse(c *gin.Context, flusher http.Flusher, payload map[string]interface{}) error {
	if err := writeQuestionPipelineSSEEvent(c, flusher, "status", gin.H{"message": "模型生成完成，正在整理候选题卡"}); err != nil {
		return err
	}
	if warnings, ok := payload["warnings"].([]interface{}); ok {
		for _, item := range warnings {
			message := strings.TrimSpace(fmt.Sprint(item))
			if message == "" {
				continue
			}
			if err := writeQuestionPipelineSSEEvent(c, flusher, "warning", gin.H{"message": message}); err != nil {
				return err
			}
		}
	}
	if cards, ok := payload["cards"].([]interface{}); ok {
		for _, item := range cards {
			if err := writeQuestionPipelineSSEEvent(c, flusher, "card", gin.H{"card": item}); err != nil {
				return err
			}
		}
	}
	return writeQuestionPipelineSSEEvent(c, flusher, "complete", gin.H{"response": payload})
}

// handleAdminGenerateQuestionPipeline 调用同步题目流水线生成接口，并直接返回完整候选题卡结果。
// handleAdminGenerateQuestionPipeline 调用同步题目流水线生成接口，并直接返回完整候选题卡结果。
func (gw *Gateway) handleAdminGenerateQuestionPipeline(c *gin.Context) {
	req, err := bindQuestionPipelineGenerateRequest(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, err := gw.adminClient.GenerateQuestionPipeline(c.Request.Context(), req)
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// handleAdminGenerateQuestionPipelineAsync 创建后台异步题目流水线任务，供长耗时场景排队执行。
// handleAdminGenerateQuestionPipelineAsync 创建后台异步题目流水线任务，供长耗时场景排队执行。
func (gw *Gateway) handleAdminGenerateQuestionPipelineAsync(c *gin.Context) {
	req, err := bindQuestionPipelineGenerateRequest(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, err := gw.adminClient.GenerateQuestionPipelineAsync(c.Request.Context(), req)
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// handleAdminGenerateQuestionPipelineStream 以 SSE 形式持续推送异步题目流水线任务进度。
// handleAdminGenerateQuestionPipelineDirectStream 兼容前端直连 POST SSE 生成接口，
// 底层仍复用同步生成 RPC，再由网关拆成 status/card/warning/complete 事件流。
// handleAdminGenerateQuestionPipelineDirectStream 兼容前端直连 POST SSE 生成接口，
// 底层仍复用同步生成 RPC，再由网关拆成 status/card/warning/complete 事件流。
func (gw *Gateway) handleAdminGenerateQuestionPipelineDirectStream(c *gin.Context) {
	req, err := bindQuestionPipelineGenerateRequest(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	flusher, ok := startQuestionPipelineSSE(c)
	if !ok {
		return
	}
	if err := writeQuestionPipelineSSEEvent(c, flusher, "status", gin.H{"message": "已提交生成请求，正在等待模型返回结果"}); err != nil {
		return
	}

	resp, err := gw.adminClient.GenerateQuestionPipeline(c.Request.Context(), req)
	if err != nil {
		_ = writeQuestionPipelineSSEEvent(c, flusher, "error", gin.H{"message": grpcErrorMessage(err)})
		return
	}
	_ = streamQuestionPipelineGenerateResponse(c, flusher, normalizeQuestionPipelineGenerateResponsePayload(resp))
}

// handleAdminGenerateQuestionPipelineStream 以 SSE 形式持续推送异步题目流水线任务进度。
// handleAdminGenerateQuestionPipelineStream 以 SSE 形式持续推送异步题目流水线任务进度。
func (gw *Gateway) handleAdminGenerateQuestionPipelineStream(c *gin.Context) {
	taskID, err := strconv.ParseUint(strings.TrimSpace(c.Query("task_id")), 10, 64)
	if err != nil || taskID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid task_id"})
		return
	}
	flusher, ok := startQuestionPipelineSSE(c)
	if !ok {
		return
	}
	task, err := gw.adminClient.GetScraperTask(c.Request.Context(), &adminv1.GetScraperTaskRequest{Id: taskID})
	if err != nil {
		grpcErr(c, err)
		return
	}
	if task.GetTaskType() != questionPipelineTaskType {
		c.JSON(http.StatusBadRequest, gin.H{"error": "task is not a question pipeline task"})
		return
	}

	streamCtx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Minute)
	defer cancel()
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	lastStatus := ""
	lastImported := int32(-1)
	lastQuestionCount := int32(-1)
	currentTask := task
	for {
		if currentTask.GetStatus() != lastStatus || currentTask.GetImportedCount() != lastImported || currentTask.GetQuestionCount() != lastQuestionCount {
			if err := writeQuestionPipelineSSEEvent(c, flusher, "progress", gin.H{
				"task_id": taskID,
				"current": currentTask.GetImportedCount(),
				"total":   currentTask.GetQuestionCount(),
				"status":  currentTask.GetStatus(),
			}); err != nil {
				return
			}
			lastStatus = currentTask.GetStatus()
			lastImported = currentTask.GetImportedCount()
			lastQuestionCount = currentTask.GetQuestionCount()
		}

		switch currentTask.GetStatus() {
		case "completed":
			if err := writeQuestionPipelineSSEEvent(c, flusher, "complete", buildQuestionPipelineCompletePayload(taskID, currentTask)); err != nil {
				return
			}
			return
		case "failed":
			_ = writeQuestionPipelineSSEEvent(c, flusher, "error", gin.H{
				"task_id": taskID,
				"message": currentTask.GetErrorMsg(),
				"status":  currentTask.GetStatus(),
			})
			return
		}

		select {
		case <-streamCtx.Done():
			if errors.Is(streamCtx.Err(), context.DeadlineExceeded) {
				_ = writeQuestionPipelineSSEEvent(c, flusher, "error", gin.H{
					"task_id": taskID,
					"message": "stream timeout",
					"status":  currentTask.GetStatus(),
				})
			}
			return
		case <-c.Request.Context().Done():
			return
		case <-ticker.C:
			nextTask, err := gw.adminClient.GetScraperTask(streamCtx, &adminv1.GetScraperTaskRequest{Id: taskID})
			if err != nil {
				_ = writeQuestionPipelineSSEEvent(c, flusher, "error", gin.H{
					"task_id": taskID,
					"message": grpcErrorMessage(err),
				})
				return
			}
			currentTask = nextTask
		}
	}
}

func (gw *Gateway) handleAdminImportQuestionPipeline(c *gin.Context) {
	var req struct {
		IndustryCode string `json:"industry_code"`
		Cards        []struct {
			Title       string   `json:"title"`
			Content     string   `json:"content"`
			Type        string   `json:"type"`
			Difficulty  string   `json:"difficulty"`
			Category    string   `json:"category"`
			Answer      string   `json:"answer"`
			Explanation string   `json:"explanation"`
			Tags        []string `json:"tags"`
		} `json:"cards"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	cards := make([]*adminv1.PipelineCard, len(req.Cards))
	for i, c := range req.Cards {
		cards[i] = &adminv1.PipelineCard{
			Title: c.Title, Content: c.Content, Type: c.Type,
			Difficulty: c.Difficulty, Category: c.Category,
			Answer: c.Answer, Explanation: c.Explanation, Tags: c.Tags,
		}
	}
	resp, err := gw.adminClient.ImportQuestionPipeline(c.Request.Context(), &adminv1.ImportQuestionPipelineRequest{
		IndustryCode: req.IndustryCode, Cards: cards,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// ==================== Admin: 分类管理 ====================

func (gw *Gateway) handleAdminListCategories(c *gin.Context) {
	resp, err := gw.adminClient.AdminListCategories(c.Request.Context(), &emptypb.Empty{})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleAdminCreateCategory(c *gin.Context) {
	var req struct {
		IndustryID  uint64 `json:"industry_id"`
		Name        string `json:"name"`
		ParentID    uint64 `json:"parent_id"`
		SortOrder   int32  `json:"sort_order"`
		Icon        string `json:"icon"`
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, err := gw.adminClient.CreateCategory(c.Request.Context(), &adminv1.CreateCategoryRequest{
		IndustryId: req.IndustryID, Name: req.Name, ParentId: req.ParentID,
		SortOrder: req.SortOrder, Icon: req.Icon, Description: req.Description,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleAdminUpdateCategory(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req struct {
		IndustryID  uint64 `json:"industry_id"`
		Name        string `json:"name"`
		ParentID    uint64 `json:"parent_id"`
		SortOrder   int32  `json:"sort_order"`
		Icon        string `json:"icon"`
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	_, err := gw.adminClient.UpdateCategory(c.Request.Context(), &adminv1.UpdateCategoryRequest{
		Id: id, IndustryId: req.IndustryID, Name: req.Name, ParentId: req.ParentID,
		SortOrder: req.SortOrder, Icon: req.Icon, Description: req.Description,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "updated"})
}

func (gw *Gateway) handleAdminDeleteCategory(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	_, err := gw.adminClient.DeleteCategory(c.Request.Context(), &adminv1.DeleteCategoryRequest{Id: id})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

// ==================== Admin: 行业管理 ====================

func (gw *Gateway) handleAdminListIndustries(c *gin.Context) {
	resp, err := gw.adminClient.AdminListIndustries(c.Request.Context(), &emptypb.Empty{})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleAdminCreateIndustry(c *gin.Context) {
	var req struct {
		Code        string `json:"code"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Icon        string `json:"icon"`
		SortOrder   int32  `json:"sort_order"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, err := gw.adminClient.CreateIndustry(c.Request.Context(), &adminv1.CreateIndustryRequest{
		Code: req.Code, Name: req.Name, Description: req.Description,
		Icon: req.Icon, SortOrder: req.SortOrder,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleAdminUpdateIndustry(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req struct {
		Code        string `json:"code"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Icon        string `json:"icon"`
		IsActive    *bool  `json:"is_active"`
		SortOrder   int32  `json:"sort_order"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	_, err := gw.adminClient.UpdateIndustry(c.Request.Context(), &adminv1.UpdateIndustryRequest{
		Id: id, Code: req.Code, Name: req.Name, Description: req.Description,
		Icon: req.Icon, IsActive: req.IsActive, SortOrder: req.SortOrder,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "updated"})
}

// ==================== Admin: Prompt 模板 ====================

// handleAdminListPrompts 兼容旧后台 `/admin/prompts` 结构，并补齐 scene/industry_id 等前端仍在使用的字段。
func (gw *Gateway) handleAdminListPrompts(c *gin.Context) {
	sceneFilter := strings.TrimSpace(c.Query("scene"))
	industryIDFilter := strings.TrimSpace(c.Query("industry_id"))

	industryIDToCode, industryCodeToID, err := gw.loadAdminIndustryMaps(c.Request.Context())
	if err != nil {
		grpcErr(c, err)
		return
	}

	industryCode := ""
	if industryIDFilter != "" {
		industryID, parseErr := strconv.ParseUint(industryIDFilter, 10, 64)
		if parseErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid industry_id"})
			return
		}
		industryCode = industryIDToCode[industryID]
		if industryCode == "" {
			c.JSON(http.StatusOK, []gin.H{})
			return
		}
	}

	resp, err := gw.adminClient.ListPromptTemplates(c.Request.Context(), &adminv1.ListPromptTemplatesRequest{
		IndustryCode: industryCode,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}

	items := make([]gin.H, 0, len(resp.GetTemplates()))
	for _, template := range resp.GetTemplates() {
		if sceneFilter != "" && strings.TrimSpace(template.GetScene()) != sceneFilter {
			continue
		}
		var industryID interface{}
		if mappedID := industryCodeToID[template.GetIndustryCode()]; mappedID > 0 {
			industryID = mappedID
		}
		items = append(items, gin.H{
			"id":               template.GetId(),
			"industry_id":      industryID,
			"name":             template.GetName(),
			"scene":            firstNonEmpty(template.GetScene(), template.GetTemplateType()),
			"template_content": template.GetContent(),
			"variables":        template.GetVariables(),
			"is_active":        template.GetIsActive(),
			"updated_at":       template.GetUpdatedAt(),
		})
	}
	c.JSON(http.StatusOK, items)
}

func (gw *Gateway) handleAdminCreatePrompt(c *gin.Context) {
	var req struct {
		IndustryID      uint64 `json:"industry_id"`
		Name            string `json:"name"`
		Scene           string `json:"scene"`
		TemplateContent string `json:"template_content"`
		Variables       string `json:"variables"`
		IsActive        bool   `json:"is_active"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, err := gw.adminClient.CreatePrompt(c.Request.Context(), &adminv1.CreatePromptRequest{
		IndustryId: req.IndustryID, Name: req.Name, Scene: req.Scene,
		TemplateContent: req.TemplateContent, Variables: req.Variables, IsActive: req.IsActive,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleAdminUpdatePrompt(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req struct {
		IndustryID      uint64 `json:"industry_id"`
		Name            string `json:"name"`
		Scene           string `json:"scene"`
		TemplateContent string `json:"template_content"`
		Variables       string `json:"variables"`
		IsActive        *bool  `json:"is_active"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	_, err := gw.adminClient.UpdatePrompt(c.Request.Context(), &adminv1.UpdatePromptRequest{
		Id: id, IndustryId: req.IndustryID, Name: req.Name, Scene: req.Scene,
		TemplateContent: req.TemplateContent, Variables: req.Variables, IsActive: req.IsActive,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "updated"})
}

func (gw *Gateway) handleAdminDeletePrompt(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	_, err := gw.adminClient.DeletePrompt(c.Request.Context(), &adminv1.DeletePromptRequest{Id: id})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

func (gw *Gateway) handleAdminTestRenderPrompt(c *gin.Context) {
	var req struct {
		AgentType string            `json:"agent_type"`
		Prompt    string            `json:"prompt"`
		Params    map[string]string `json:"params"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, err := gw.adminClient.TestRenderPrompt(c.Request.Context(), &adminv1.TestRenderPromptRequest{
		AgentType: req.AgentType, Prompt: req.Prompt, Params: req.Params,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// ==================== Admin: AI 配置 ====================

func (gw *Gateway) handleAdminGetAIConfigs(c *gin.Context) {
	resp, err := gw.adminClient.GetAIConfigs(c.Request.Context(), &emptypb.Empty{})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// handleAdminUpdateAIConfigs 在无 bridge 直挂时转发 AI 配置更新请求，由 admin gRPC 复用单体验证规则。
func (gw *Gateway) handleAdminUpdateAIConfigs(c *gin.Context) {
	var req struct {
		Configs map[string]string `json:"configs"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	_, err := gw.adminClient.UpdateAIConfigs(c.Request.Context(), &adminv1.UpdateAIConfigsRequest{Configs: req.Configs})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "updated"})
}

// ==================== Admin: AI 预设 ====================

func (gw *Gateway) handleAdminCreateAIPreset(c *gin.Context) {
	var req struct {
		Name    string            `json:"name"`
		Configs map[string]string `json:"configs"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, err := gw.adminClient.CreateAIPreset(c.Request.Context(), &adminv1.CreateAIPresetRequest{
		Name: req.Name, Configs: req.Configs,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleAdminUpdateAIPreset(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req struct {
		Name    string            `json:"name"`
		Configs map[string]string `json:"configs"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, err := gw.adminClient.UpdateAIPreset(c.Request.Context(), &adminv1.UpdateAIPresetRequest{
		Id: id, Name: req.Name, Configs: req.Configs,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleAdminDeleteAIPreset(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	_, err := gw.adminClient.DeleteAIPreset(c.Request.Context(), &adminv1.DeleteAIPresetRequest{Id: id})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

func (gw *Gateway) handleAdminApplyAIPreset(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	resp, err := gw.adminClient.ApplyAIPreset(c.Request.Context(), &adminv1.ApplyAIPresetRequest{Id: id})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// ==================== Admin: AI 日志 ====================

func (gw *Gateway) handleAdminGetAICallLog(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	resp, err := gw.adminClient.GetAICallLog(c.Request.Context(), &adminv1.GetAICallLogRequest{Id: id})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// ==================== Admin: Live2D 管理 ====================

func (gw *Gateway) handleAdminListLive2DModels(c *gin.Context) {
	resp, err := gw.adminClient.ListLive2DModels(c.Request.Context(), &emptypb.Empty{})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleAdminCreateLive2DModel(c *gin.Context) {
	var req struct {
		Name         string `json:"name"`
		IndustryID   uint64 `json:"industry_id"`
		Scene        string `json:"scene"`
		ModelURL     string `json:"model_url"`
		ThumbnailURL string `json:"thumbnail_url"`
		ConfigJSON   string `json:"config_json"`
		TTSConfigID  uint64 `json:"tts_config_id"`
		IsActive     bool   `json:"is_active"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, err := gw.adminClient.CreateLive2DModel(c.Request.Context(), &adminv1.CreateLive2DModelRequest{
		Name: req.Name, IndustryId: req.IndustryID, Scene: req.Scene,
		ModelUrl: req.ModelURL, ThumbnailUrl: req.ThumbnailURL,
		ConfigJson: req.ConfigJSON, TtsConfigId: req.TTSConfigID, IsActive: req.IsActive,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleAdminUpdateLive2DModel(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req struct {
		Name         string `json:"name"`
		IndustryID   uint64 `json:"industry_id"`
		Scene        string `json:"scene"`
		ModelURL     string `json:"model_url"`
		ThumbnailURL string `json:"thumbnail_url"`
		ConfigJSON   string `json:"config_json"`
		TTSConfigID  uint64 `json:"tts_config_id"`
		IsActive     *bool  `json:"is_active"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	_, err := gw.adminClient.UpdateLive2DModel(c.Request.Context(), &adminv1.UpdateLive2DModelRequest{
		Id: id, Name: req.Name, IndustryId: req.IndustryID, Scene: req.Scene,
		ModelUrl: req.ModelURL, ThumbnailUrl: req.ThumbnailURL,
		ConfigJson: req.ConfigJSON, TtsConfigId: req.TTSConfigID, IsActive: req.IsActive,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "updated"})
}

func (gw *Gateway) handleAdminDeleteLive2DModel(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	_, err := gw.adminClient.DeleteLive2DModel(c.Request.Context(), &adminv1.DeleteLive2DModelRequest{Id: id})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

func (gw *Gateway) handleAdminImportLive2DPackage(c *gin.Context) {
	file, _, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file is required"})
		return
	}
	defer file.Close()
	// 读取文件内容
	fileBytes, err := io.ReadAll(file)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read file"})
		return
	}
	resp, err := gw.adminClient.ImportLive2DPackage(c.Request.Context(), &adminv1.ImportLive2DPackageRequest{
		FileContent: fileBytes,
		Filename:    c.PostForm("filename"),
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleAdminImportLive2DBackground(c *gin.Context) {
	file, _, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file is required"})
		return
	}
	defer file.Close()
	fileBytes, err := io.ReadAll(file)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read file"})
		return
	}
	resp, err := gw.adminClient.ImportLive2DBackground(c.Request.Context(), &adminv1.ImportLive2DBackgroundRequest{
		FileContent: fileBytes,
		Filename:    c.PostForm("filename"),
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// ==================== Admin: TTS 管理 ====================

func (gw *Gateway) handleAdminListTTSConfigs(c *gin.Context) {
	resp, err := gw.adminClient.ListTTSConfigs(c.Request.Context(), &emptypb.Empty{})
	if err != nil {
		grpcErr(c, err)
		return
	}
	defaultBindings := gin.H{
		"interview": gw.getOptionalAdminConfigUint64(c.Request.Context(), "tts_default_interview"),
		"companion": gw.getOptionalAdminConfigUint64(c.Request.Context(), "tts_default_companion"),
	}
	items := make([]gin.H, 0, len(resp.GetConfigs()))
	for _, config := range resp.GetConfigs() {
		supportStatus, supportMessage := resolveTTSSupportMeta(config.GetEngine())
		items = append(items, gin.H{
			"id":               config.GetId(),
			"name":             config.GetName(),
			"engine":           config.GetEngine(),
			"voice_id":         config.GetVoiceId(),
			"auth_config_json": config.GetAuthConfigJson(),
			"params_json":      config.GetParamsJson(),
			"is_active":        config.GetIsActive(),
			"sort_order":       config.GetSortOrder(),
			"support_status":   supportStatus,
			"support_message":  supportMessage,
			"created_at":       config.GetCreatedAt(),
		})
	}
	c.JSON(http.StatusOK, gin.H{
		"configs":          items,
		"providers":        buildLegacyTTSProviders(),
		"default_bindings": defaultBindings,
	})
}

func (gw *Gateway) handleAdminCreateTTSConfig(c *gin.Context) {
	var req struct {
		Name           string `json:"name"`
		Engine         string `json:"engine"`
		VoiceID        string `json:"voice_id"`
		AuthConfigJSON string `json:"auth_config_json"`
		ParamsJSON     string `json:"params_json"`
		IsActive       bool   `json:"is_active"`
		SortOrder      int32  `json:"sort_order"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, err := gw.adminClient.CreateTTSConfig(c.Request.Context(), &adminv1.CreateTTSConfigRequest{
		Name: req.Name, Engine: req.Engine, VoiceId: req.VoiceID,
		AuthConfigJson: req.AuthConfigJSON, ParamsJson: req.ParamsJSON,
		IsActive: req.IsActive, SortOrder: req.SortOrder,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleAdminUpdateTTSConfig(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req struct {
		Name           string `json:"name"`
		Engine         string `json:"engine"`
		VoiceID        string `json:"voice_id"`
		AuthConfigJSON string `json:"auth_config_json"`
		ParamsJSON     string `json:"params_json"`
		IsActive       *bool  `json:"is_active"`
		SortOrder      int32  `json:"sort_order"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	_, err := gw.adminClient.UpdateTTSConfig(c.Request.Context(), &adminv1.UpdateTTSConfigRequest{
		Id: id, Name: req.Name, Engine: req.Engine, VoiceId: req.VoiceID,
		AuthConfigJson: req.AuthConfigJSON, ParamsJson: req.ParamsJSON,
		IsActive: req.IsActive, SortOrder: req.SortOrder,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "updated"})
}

func (gw *Gateway) handleAdminDeleteTTSConfig(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	_, err := gw.adminClient.DeleteTTSConfig(c.Request.Context(), &adminv1.DeleteTTSConfigRequest{Id: id})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

func (gw *Gateway) handleAdminUpdateTTSSceneDefaults(c *gin.Context) {
	var req struct {
		DefaultBindings map[string]uint64 `json:"default_bindings"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	_, err := gw.adminClient.UpdateTTSSceneDefaults(c.Request.Context(), &adminv1.UpdateTTSSceneDefaultsRequest{
		DefaultBindings: req.DefaultBindings,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "updated"})
}

// ==================== Admin: RAG 配置 ====================

func (gw *Gateway) handleAdminGetRAGConfigs(c *gin.Context) {
	resp, err := gw.adminClient.GetRAGConfigs(c.Request.Context(), &emptypb.Empty{})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleAdminUpdateRAGConfigs(c *gin.Context) {
	var raw map[string]interface{}
	if err := c.ShouldBindJSON(&raw); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	configs := make(map[string]string)
	if nested, ok := raw["configs"].(map[string]interface{}); ok {
		for key, value := range nested {
			configs[key] = strings.TrimSpace(fmt.Sprint(value))
		}
	} else {
		for key, value := range raw {
			configs[key] = strings.TrimSpace(fmt.Sprint(value))
		}
	}
	if value := strings.TrimSpace(configs["ai_rag_collection"]); value != "" {
		configs["rag_collection_name"] = value
	}
	if value := strings.TrimSpace(configs["ai_rag_embed_model"]); value != "" {
		configs["rag_embedding_model"] = value
	}

	_, err := gw.adminClient.UpdateRAGConfigs(c.Request.Context(), &adminv1.UpdateRAGConfigsRequest{Configs: configs})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "updated"})
}

func (gw *Gateway) handleAdminTestRAGConnection(c *gin.Context) {
	resp, err := gw.adminClient.TestRAGConnection(c.Request.Context(), &emptypb.Empty{})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// ==================== Admin: RAG 索引 ====================

func (gw *Gateway) handleAdminIndexAllQuestions(c *gin.Context) {
	var req struct {
		IndustryID uint64 `json:"industry_id"`
	}
	c.ShouldBindJSON(&req)
	resp, err := gw.adminClient.IndexAllQuestions(c.Request.Context(), &adminv1.IndexAllQuestionsRequest{IndustryId: req.IndustryID})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleAdminIndexQuestions(c *gin.Context) {
	var req struct {
		QuestionIDs []uint64 `json:"question_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, err := gw.adminClient.IndexQuestions(c.Request.Context(), &adminv1.IndexQuestionsRequest{QuestionIds: req.QuestionIDs})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleAdminDeleteRAGIndex(c *gin.Context) {
	var req struct {
		QuestionIDs []uint64 `json:"question_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, err := gw.adminClient.DeleteRAGIndex(c.Request.Context(), &adminv1.DeleteRAGIndexRequest{QuestionIds: req.QuestionIDs})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleAdminSearchRAGQuestions(c *gin.Context) {
	query := c.Query("query")
	topK, _ := strconv.ParseInt(c.DefaultQuery("top_k", "5"), 10, 32)
	resp, err := gw.adminClient.SearchRAGQuestions(c.Request.Context(), &adminv1.SearchRAGQuestionsRequest{
		Query: query, TopK: int32(topK),
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// ==================== Admin: RAG 文档 ====================

func (gw *Gateway) handleAdminListRAGDocuments(c *gin.Context) {
	page, _ := strconv.ParseInt(c.DefaultQuery("page", "1"), 10, 32)
	pageSize, _ := strconv.ParseInt(c.DefaultQuery("page_size", "20"), 10, 32)
	resp, err := gw.adminClient.ListRAGDocuments(c.Request.Context(), &adminv1.ListRAGDocumentsRequest{
		Page:       &sharedv1.PageParam{Page: int32(page), PageSize: int32(pageSize)},
		Collection: c.Query("collection"),
		DocType:    c.Query("doc_type"),
		Keyword:    c.Query("keyword"),
		SyncStatus: c.Query("sync_status"),
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleAdminGetRAGDocumentStats(c *gin.Context) {
	resp, err := gw.adminClient.GetRAGDocumentStats(c.Request.Context(), &adminv1.GetRAGDocumentStatsRequest{
		Collection: c.Query("collection"),
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleAdminGetRAGDocument(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	resp, err := gw.adminClient.GetRAGDocument(c.Request.Context(), &adminv1.GetRAGDocumentRequest{Id: id})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleAdminCreateRAGDocument(c *gin.Context) {
	var req struct {
		Collection string            `json:"collection"`
		DocType    string            `json:"doc_type"`
		Title      string            `json:"title"`
		Content    string            `json:"content"`
		Metadata   map[string]string `json:"metadata"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, err := gw.adminClient.CreateRAGDocument(c.Request.Context(), &adminv1.CreateRAGDocumentRequest{
		Collection: req.Collection, DocType: req.DocType,
		Title: req.Title, Content: req.Content, Metadata: req.Metadata,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleAdminUpdateRAGDocument(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req struct {
		Collection string            `json:"collection"`
		DocType    string            `json:"doc_type"`
		Title      string            `json:"title"`
		Content    string            `json:"content"`
		Metadata   map[string]string `json:"metadata"`
		IsActive   *bool             `json:"is_active"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	_, err := gw.adminClient.UpdateRAGDocument(c.Request.Context(), &adminv1.UpdateRAGDocumentRequest{
		Id: id, Collection: req.Collection, DocType: req.DocType,
		Title: req.Title, Content: req.Content, Metadata: req.Metadata, IsActive: req.IsActive,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "updated"})
}

func (gw *Gateway) handleAdminDeleteRAGDocument(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	_, err := gw.adminClient.DeleteRAGDocument(c.Request.Context(), &adminv1.DeleteRAGDocumentRequest{Id: id})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

func (gw *Gateway) handleAdminBatchImportRAGDocuments(c *gin.Context) {
	var req struct {
		Collection string `json:"collection"`
		DocType    string `json:"doc_type"`
		Documents  []struct {
			Title    string            `json:"title"`
			Content  string            `json:"content"`
			Metadata map[string]string `json:"metadata"`
		} `json:"documents"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	docs := make([]*adminv1.BatchImportDocItem, len(req.Documents))
	for i, d := range req.Documents {
		docs[i] = &adminv1.BatchImportDocItem{
			Title: d.Title, Content: d.Content, Metadata: d.Metadata,
		}
	}
	resp, err := gw.adminClient.BatchImportRAGDocuments(c.Request.Context(), &adminv1.BatchImportRAGDocumentsRequest{
		Collection: req.Collection, DocType: req.DocType, Documents: docs,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleAdminSyncRAGDocuments(c *gin.Context) {
	var req struct {
		IDs []uint64 `json:"ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	_, err := gw.adminClient.SyncRAGDocumentsToVectorDB(c.Request.Context(), &adminv1.SyncRAGDocumentsRequest{Ids: req.IDs})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "synced"})
}

func (gw *Gateway) handleAdminSyncAllPendingRAGDocuments(c *gin.Context) {
	_, err := gw.adminClient.SyncAllPendingRAGDocuments(c.Request.Context(), &emptypb.Empty{})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "synced"})
}

// ==================== Admin: 面经爬虫 ====================

func (gw *Gateway) handleAdminGetScraperSources(c *gin.Context) {
	resp, err := gw.adminClient.GetScraperSources(c.Request.Context(), &emptypb.Empty{})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleAdminScraperSearch(c *gin.Context) {
	var req struct {
		Keyword  string `json:"keyword"`
		Source   string `json:"source"`
		Page     int32  `json:"page"`
		PageSize int32  `json:"page_size"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, err := gw.adminClient.ScraperSearch(c.Request.Context(), &adminv1.ScraperSearchRequest{
		Keyword: req.Keyword, Source: req.Source, Page: req.Page, PageSize: req.PageSize,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleAdminScraperFetch(c *gin.Context) {
	var req struct {
		URL    string `json:"url"`
		Source string `json:"source"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, err := gw.adminClient.ScraperFetch(c.Request.Context(), &adminv1.ScraperFetchRequest{
		Url: req.URL, Source: req.Source,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleAdminScraperClean(c *gin.Context) {
	var req struct {
		Content      string `json:"content"`
		IndustryCode string `json:"industry_code"`
		Source       string `json:"source"`
		SourceURL    string `json:"source_url"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, err := gw.adminClient.ScraperClean(c.Request.Context(), &adminv1.ScraperCleanRequest{
		Content: req.Content, IndustryCode: req.IndustryCode,
		Source: req.Source, SourceUrl: req.SourceURL,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleAdminScraperImport(c *gin.Context) {
	var req struct {
		IndustryCode string `json:"industry_code"`
		Questions    []struct {
			CategoryName string `json:"category_name"`
			Type         string `json:"type"`
			Difficulty   string `json:"difficulty"`
			Title        string `json:"title"`
			Content      string `json:"content"`
			Answer       string `json:"answer"`
			Explanation  string `json:"explanation"`
			Tags         string `json:"tags"`
		} `json:"questions"`
		SourceURL   string `json:"source_url"`
		SourceTitle string `json:"source_title"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	items := make([]*adminv1.ScraperCleanedQuestion, len(req.Questions))
	for i, q := range req.Questions {
		items[i] = &adminv1.ScraperCleanedQuestion{
			CategoryName: q.CategoryName, Type: q.Type, Difficulty: q.Difficulty,
			Title: q.Title, Content: q.Content, Answer: q.Answer,
			Explanation: q.Explanation, Tags: q.Tags,
		}
	}
	resp, err := gw.adminClient.ScraperImport(c.Request.Context(), &adminv1.ScraperImportRequest{
		IndustryCode: req.IndustryCode, Questions: items,
		SourceUrl: req.SourceURL, SourceTitle: req.SourceTitle,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleAdminScraperImportAsync(c *gin.Context) {
	var req struct {
		IndustryCode string `json:"industry_code"`
		Questions    []struct {
			CategoryName string `json:"category_name"`
			Type         string `json:"type"`
			Difficulty   string `json:"difficulty"`
			Title        string `json:"title"`
			Content      string `json:"content"`
			Answer       string `json:"answer"`
			Explanation  string `json:"explanation"`
			Tags         string `json:"tags"`
		} `json:"questions"`
		SourceURL   string `json:"source_url"`
		SourceTitle string `json:"source_title"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	items := make([]*adminv1.ScraperCleanedQuestion, len(req.Questions))
	for i, q := range req.Questions {
		items[i] = &adminv1.ScraperCleanedQuestion{
			CategoryName: q.CategoryName, Type: q.Type, Difficulty: q.Difficulty,
			Title: q.Title, Content: q.Content, Answer: q.Answer,
			Explanation: q.Explanation, Tags: q.Tags,
		}
	}
	resp, err := gw.adminClient.ScraperImportAsync(c.Request.Context(), &adminv1.ScraperImportRequest{
		IndustryCode: req.IndustryCode, Questions: items,
		SourceUrl: req.SourceURL, SourceTitle: req.SourceTitle,
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleAdminListScraperTasks(c *gin.Context) {
	page, _ := strconv.ParseInt(c.DefaultQuery("page", "1"), 10, 32)
	pageSize, _ := strconv.ParseInt(c.DefaultQuery("page_size", "20"), 10, 32)
	resp, err := gw.adminClient.ListScraperTasks(c.Request.Context(), &adminv1.ListScraperTasksRequest{
		Page:     &sharedv1.PageParam{Page: int32(page), PageSize: int32(pageSize)},
		Status:   c.Query("status"),
		TaskType: c.Query("task_type"),
	})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleAdminGetScraperTask(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	resp, err := gw.adminClient.GetScraperTask(c.Request.Context(), &adminv1.GetScraperTaskRequest{Id: id})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (gw *Gateway) handleAdminRetryScraperTask(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	resp, err := gw.adminClient.RetryScraperTask(c.Request.Context(), &adminv1.RetryScraperTaskRequest{Id: id})
	if err != nil {
		grpcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// writeQuestionPipelineSSEEvent 将题目流水线事件编码成标准 SSE 消息并立即刷新。
func writeQuestionPipelineSSEEvent(c *gin.Context, flusher http.Flusher, event string, payload interface{}) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if _, err := c.Writer.Write([]byte("event: " + event + "\ndata: " + string(body) + "\n\n")); err != nil {
		return err
	}
	flusher.Flush()
	return nil
}

// writeQuestionPipelineSSEPrelude 先输出一段注释填充，降低代理对小块 SSE 的缓冲概率。
func writeQuestionPipelineSSEPrelude(c *gin.Context, flusher http.Flusher) error {
	padding := ":" + strings.Repeat(" ", 2048) + "\n\n"
	if _, err := c.Writer.Write([]byte(padding)); err != nil {
		return err
	}
	flusher.Flush()
	return nil
}

// buildQuestionPipelineCompletePayload 组装 complete 事件载荷，并尽量透传 Admin 侧结果快照。
func buildQuestionPipelineCompletePayload(taskID uint64, task *adminv1.ScraperTaskDetail) gin.H {
	payload := gin.H{
		"task_id":         taskID,
		"total_generated": task.GetImportedCount(),
		"total_failed":    maxInt32(task.GetQuestionCount()-task.GetImportedCount(), 0),
		"status":          task.GetStatus(),
	}
	if strings.TrimSpace(task.GetResultJson()) == "" {
		return payload
	}
	var result interface{}
	if err := json.Unmarshal([]byte(task.GetResultJson()), &result); err == nil {
		payload["result"] = result
	}
	return payload
}

// grpcErrorMessage 提取 gRPC 错误的可读消息，供 SSE error 事件使用。
func grpcErrorMessage(err error) string {
	if st, ok := status.FromError(err); ok {
		return st.Message()
	}
	return "internal error"
}

// maxInt32 返回两个 int32 中较大的值，避免 complete 事件出现负数统计。
func maxInt32(left int32, right int32) int32 {
	if left > right {
		return left
	}
	return right
}
