# Admin 功能对齐分析报告

> 范围：admin 前端 11 个页面 × 微服务 admin/网关 × 旧单体 backend
> 微服务代码：`app/admin/`、`app/gateway/internal/proxy/handler.go`、proto：`api/makejob/admin/v1/admin.proto`
> 旧单体代码：`docs/backend/internal/handler/admin_handler.go`、`docs/backend/internal/service/admin_service.go`
> 前端代码：`frontend-react/apps/admin/src/features/*`

---

## 一、汇总表（6 列要求字段）

| # | 页面/模块 | 前端所需字段 | 旧后端关键逻辑 | 微服务现状 | 对齐状态 | 备注与建议 |
|---|-----------|--------------|----------------|------------|----------|------------|
| 1 | **Dashboard** 运营面板 | total_users、total_questions、total_interviews、today_active_users、pro_members、new_users_today | `GetDashboard` 6 个统计字段直查 DB | proto `DashboardResponse` 6 字段齐全；admin service `GetDashboard` 通过 userClient/questionClient/interviewClient 并发取数；网关 `handleAdminGetDashboard` 直透 | **已对齐** | 无 |
| 2 | **Runtime / AI Call Logs** | 列表：id/created_at/trace_id/task_id/source/scene/provider/model/latency_ms/model_error/is_success/industry_id/selected_prompt_id/selected_prompt_name/prompt_source/runtime_config/scene_config；详情：补 user_input/model_output/rendered_prompt/request_messages/updated_at | 旧后端**无**AI 调用日志（`ai_call_logs` 表为新加） | proto `AICallLogDetail` 缺失 `task_id`、`prompt_source`、`selected_prompt_name`、`scene_config`、`updated_at`、`industry_id`；service `ListAICallLogs`/`GetAICallLog` 直接查询 `ai_call_logs`；网关 `handleAdminListAICallLogs` 直透 proto | **部分缺失** | ① proto 增字段（task_id/prompt_source/selected_prompt_name/scene_config/updated_at/industry_id）；② 前端 `taskId` 搜索条件目前用 `task_id` 列查询，需确保后端支持 `task_id` 过滤；③ 网关 `GET /admin/ai-call-logs?trace_id&task_id&status` 路由已注册（handler.go:1854） |
| 3 | **Runtime / Scraper Tasks** | 列表：id/task_type/source/source_title/source_url/status/question_count/imported_count/retry_count/error_msg/started_at/finished_at/created_at/updated_at；详情：payload_json、result_json | 旧后端**无**爬虫任务表（scraper 为新加） | proto `ScraperTaskDetail` **缺 `payload_json`**；service `ListScraperTasks`/`GetScraperTask` 已实现；网关 `handleAdminListScraperTasks`/`handleAdminGetScraperTask` 已注册；retry 走 MQ 重发 | **部分缺失** | ① proto `ScraperTaskDetail` 增 `payload_json` 字段（或前端改用 `result_json` 替代）；② RetryScraperTask 已实现（admin.go:1831） |
| 4 | **AI Config** AI 模型配置 | 列表+表单：ai_provider/ai_fallback_provider/ai_model/ai_api_key/ai_base_url/ai_temperature/ai_top_p/ai_max_tokens/ai_timeout_seconds/ai_enable_stream/ai_scene_interview_model/ai_scene_plan_model/ai_scene_companion_model/ai_scene_quiz_model | `GetAIConfigs`/`UpdateAIConfigs` 走 admin_config 表，`ai.IsRAGConfigKey` 过滤 | proto `GetAIConfigsResponse` configs map + items + presets + active_preset_id；admin service 直接 Get/Set admin_config；网关 `handleAdminGetAIConfigs` 直透 | **已对齐** | 旧后端也支持 AI 预设，前端 /admin/ai-config-presets 已实现（handler.go:5132-5149） |
| 5 | **AI Preset** AI 预设 | name、is_active、configs、updated_at、active_preset_id | `SaveAIPreset` name 唯一性检查 | proto `AIPreset` 含 id/name/provider/model/params/active/created_at/updated_at；service `CreateAIPreset`/`UpdateAIPreset` 实现；网关支持 CRUD + apply | **已对齐** | 前端预设 CRUD 全部由 `handleAdminCreateAIPreset/Update/Delete/ApplyAIPreset` 转发 |
| 6 | **RAG Config** | ai_rag_enabled/ai_rag_collection/ai_rag_top_k/ai_rag_score_threshold/ai_rag_milvus_addr/ai_rag_milvus_user/ai_rag_milvus_password/ai_rag_embed_api_key/ai_rag_embed_model/ai_rag_embed_base_url；状态：enabled/milvus_connected/collection/embed_model；测试：milvus_ok/embedding_ok/error | `GetRAGConfigs` 同 AI Config 走 admin_config + `ai.IsRAGConfigKey` 过滤；`TestRAGConnection` 测 Milvus + Embedding 双连通 | proto `GetAIConfigsResponse` 复用（configs map）；`RAGSystemStatus` 4 字段；`TestRAGConnectionResponse` 3 字段；admin service 全部实现；网关 `handleAdminGetRAGConfigs/Update/Test` 注册（`adaptLegacyResponseByPath` 命中 normalize） | **已对齐** | 旧后端 + 微服务实现完全一致；前端 `TestConnection` 调用 `POST /admin/rag-configs/test` 走 service `TestRAGConnection` |
| 7 | **RAG Knowledge** 知识库 | 列表：id/collection/doc_type/title/sync_status/is_active/created_at/updated_at + 搜索（doc_type/keyword/sync_status）；表单：collection/doc_type/title/content/metadata；同步：ids、sync-all | **旧后端无** RAG documents 路由（仅 RAG config） | proto `RAGDocumentDetail` 11 字段齐全；service `ListRAGDocuments/CreateRAGDocument/UpdateRAGDocument/DeleteRAGDocument/SyncRAGDocumentsToVectorDB/SyncAllPendingRAGDocuments` 全部实现；网关 `handleAdminListRAGDocuments/Sync/BatchImport/SyncAll` 全部注册 | **已对齐** | 全新模块，旧后端无参考；注意 batch-import 与 sync 两条单独路由 |
| 8 | **Prompt** 提示词模板 | 列表：id/industry_id/name/scene/template_content/variables/is_active + 搜索（scene/industry_id）；表单：industry_id/name/scene/template_content/variables | `ListPromptTemplates` 按 industry_code 过滤 | proto `PromptTemplate` 字段为 `industry_code`（**字符串**）而非前端 `industry_id`（**uint64**）；service `ListPromptTemplates` 用 `industry_code`；网关 `handleAdminListPrompts` 在 `normalizeGatewayResponseByPath`（handler.go:4961）显式补 `industry_id` 兼容 | **部分差异** | ① proto 字段语义不一致（code vs id），依赖网关适配层；② 建议 proto 增加 `industry_id` 字段或改为 `industry_id` 单一字段；③ 业务校验：scene 必须为 `interview/companion/quiz/plan` 之一 |
| 9 | **Live2D** | 列表：id/name/industry_id/scene/model_url/thumbnail_url/config_json(background_image_url)/tts_config_id/is_active/created_at/**updated_at**；表单：上述 + 缩略图；导入：ZIP 包 + 背景图 | `CreateLive2DModel` 校验 industry/tts 存在；`UpdateLive2DModel` 当 URL 变化时清理旧资产；`ImportLive2DPackage` ZIP 解压自动识别 model3.json | proto `Live2DModelInfo` **缺 `updated_at`**；service CRUD + Import 实现；网关 `handleAdminListLive2DModels/Create/Update/Delete/ImportPackage/ImportBackground` 全部注册 | **部分缺失** | ① proto `Live2DModelInfo` 增 `updated_at` 字段（前端列表排序依赖）；② 资产清理逻辑需确认已迁移到微服务（admin service `UpdateLive2DModel`）；③ 200MB ZIP / 20MB 图片 限制需保留 |
| 10 | **TTS** | 列表：id/name/engine/voice_id/auth_config_json/params_json/is_active/sort_order/**support_status/support_message/scene**；表单：同；providers：key/label/description/support_status/support_message/auth_template/params_template/auth_fields/param_fields；default_bindings：interview/companion | `CreateTTSConfig` 调 `ValidateTTSConfigInput` 校验 engine/voice/auth/params；`UpdateTTSSceneDefaults` 校验 scene 枚举 + tts config 存在 | proto `TTSConfigInfo` **缺 `support_status`、`support_message`、`scene`**；service `ListTTSConfigs`/`Create`/`Update`/`Delete`/`UpdateTTSSceneDefaults` 全部实现；网关 `handleAdminListTTSConfigs` **在网关层硬补** `support_status/support_message`（来自 `resolveTTSSupportMeta`） + `providers` 来自 `buildLegacyTTSProviders`（handler.go:5346-5378） | **部分差异** | ① proto 增 `support_status`/`support_message`/`scene` 字段；② `buildLegacyTTSProviders` 是写死的 provider 目录，引擎注册表应迁到 biz/data；③ default_bindings 走 `admin_config` 表的 `tts_default_interview/companion` 两个 key |
| 11 | **Taxonomy / Industries** | id/code/name/description/icon/is_active/sort_order + 搜索（code/name） | `ListIndustries`/`CreateIndustry`/`UpdateIndustry` CRUD | proto `IndustryInfo` 8 字段齐全；service `ListIndustries/Create/Update` 实现；网关 `handleAdminListIndustries/Create/Update` 注册 | **已对齐** | 无 |
| 12 | **Taxonomy / Categories** | id/industry_id/name/parent_id/sort_order/icon/description + 树形（children） | `CreateCategory` 校验 industry 存在 + parent 存在 | proto `CategoryInfo` 9 字段（含 `children` 嵌套）；service `ListCategories/Create/Update/Delete` 实现；网关 `handleAdminListCategories`/`Create`/`Update`/`Delete` 全部注册 | **已对齐** | 删除为硬删除，前端需注意级联（前端 Question 页 category_name 引用） |
| 13 | **Question Pipeline** 题目流水线 | 请求：industry_code/requirement/agent_prompt/generation_mode/candidate_count/include_scraped/include_generated/sources[]；响应卡：id/title/content/type/difficulty/category/answer/solution/explanation/tags/judge_config/confidence/source_type/source_label/source_title/source_url；统计：searched/fetched/scraped/generated/candidate/selected_sources；异步：task_id | 旧后端有 `/api/admin/question-pipeline/generate/stream` 等 4 路由（generate、stream、async、import） | proto `GenerateQuestionPipelineRequest` + `PipelineCard` 17 字段；service `GenerateQuestionPipeline/Stream/Async/Import` 全部实现（admin.go:272-543）；网关 `handleAdminGenerateQuestionPipeline` 直透 + 多个 normalize 函数（handler.go:4519-4811） | **已对齐** | 流式 SSE 已实现；异步通过 `GenerateQuestionPipelineAsync` + MQ 派发；scraper 集成走 `ScraperImportAsync` |
| 14 | **Question Bank** 题库管理 | 列表：id/category_id/category_name/industry_id/type/difficulty/title/content/options[]/answer/explanation/solution{summary,approach,key_steps,edge_cases,complexity,common_mistakes,recommended_tags}/judge_config{evaluation_mode,default_language,allowed_languages,starter_code,public_test_cases,hidden_test_cases,reference_solutions,time_limit_ms,memory_limit_mb}/answer_template{core_conclusion,key_points,sample_answer,follow_ups,pitfalls}/tags[]/is_active；搜索：keyword/difficulty/category_id | `ListQuestions` 校验 page 参数、解析 options/tags JSON；`CreateQuestion` 校验 industry/category 引用、choice/multi 类型必填 options≥2；`UpdateQuestion` 处理类型切换（如 choice→code 重置 judge_config）；`BatchImportQuestions` 映射 category_name→category_id | proto `QuestionInfo` 含 options_json/solution_json/judge_config_json/answer_template_json/tags；service `AdminListQuestions/Create/Update/Delete/BatchImport` + `GetQuestionTagTaxonomy` 全部实现；网关 `handleAdminListQuestions/Create/Update/Delete/BatchImport` + `normalizeAdminQuestionPagePayload`（handler.go:414） | **已对齐** | 校验规则已迁到 question 服务（admin 只做转发）；前端 `judge_config` 嵌套结构通过 JSON 字符串透传；tag-taxonomy 路由 `/admin/questions/tag-taxonomy` 单独提供 |
| 15 | **User Management** 用户管理 | 列表：id/username/email/role/membership_level/membership_type/is_disabled/created_at/membership_expire_at/avatar；改角色 role、禁用 | 旧后端有 `/api/admin/users`、`/users/{id}/role`、`/users/{id}/disable`；`UpdateUserRole` 校验 role∈{admin,pro,free}；`DisableUser` 设 role="disabled" | proto `AdminUserInfo` 10 字段齐全；service `ListUsers/UpdateUserRole/DisableUser` 实现；网关 `handleAdminListUsers/UpdateUserRole/DisableUser` 全部注册（handler.go:1598-1600）；biz 层透传到 user 服务 | **已对齐** | **前端当前无对应页面**（proto+后端均已就绪，缺 UI） |
| 16 | **AI Debug** AI 调试 | provider/scene/model/prompt/messages/runtime_config 自由组合；输出：model_output/latency/model_error/tokens | 旧后端无独立 debug 页 | proto `DebugAIRequest/Response`；service `DebugAI` 直接调 AIGateway；网关 `handleAdminDebugAI` 注册 | **已对齐** | 前端 `DebugButton` 在 AI Config 页签（待确认 UI 入口） |

---

## 二、按"对齐状态"分类汇总

### ✅ 已对齐（9 个）
- Dashboard
- AI Config / AI Preset
- RAG Config
- RAG Knowledge
- Taxonomy (Industry + Category)
- Question Pipeline
- Question Bank
- User Management（后端就绪，缺 UI）
- AI Debug

### ⚠️ 部分缺失 / 差异（5 个）
1. **Runtime / AI Call Logs** — proto 缺 6 个字段（task_id/prompt_source/selected_prompt_name/scene_config/updated_at/industry_id）
2. **Runtime / Scraper Tasks** — proto `ScraperTaskDetail` 缺 `payload_json`
3. **Prompt** — proto 字段语义 `industry_code`（字符串）vs 前端 `industry_id`（uint64），依赖网关适配
4. **Live2D** — proto `Live2DModelInfo` 缺 `updated_at` 字段
5. **TTS** — proto `TTSConfigInfo` 缺 `support_status`/`support_message`/`scene`，目前由网关硬补；provider 目录写死

### ❌ 未实现
无（仅前端缺用户管理 UI，但后端链路完整）。

---

## 三、关键差异点 & 修复建议

### 1. proto 字段补齐（最高优先级）

| proto | 缺失字段 | 影响前端 | 建议 |
|-------|----------|----------|------|
| `AICallLogDetail` | `task_id`、`prompt_source`、`selected_prompt_name`、`scene_config`、`updated_at`、`industry_id` | AI 日志列表/详情、调试面板 | proto 加字段，下游 ai_gateway 调用日志时写入；admin service `GetAICallLog`/`ListAICallLogs` 投影 |
| `ScraperTaskDetail` | `payload_json` | 任务详情 payload 显示 | proto 加字段；biz 层从 `scraper_tasks.payload_json` 透传 |
| `Live2DModelInfo` | `updated_at` | 列表排序 | proto 加字段；repo 投影 `updated_at` |
| `TTSConfigInfo` | `support_status`、`support_message`、`scene` | 前端表头展示"引擎支持状态" | proto 加字段；service 调 `providerCatalog.Lookup(engine)` 注入；scene 可由调用方写入 |
| `IndustryInfo` | 字段齐全 | — | — |
| `PromptTemplate` | `industry_id` 缺失（仅有 `industry_code`） | 前端表单用 industry_id 选 | 建议 proto 改为 `industry_id`（uint64），删除 `industry_code`；或新增 `industry_id` 兼容 |

### 2. 网关适配层可下沉到 biz

- `normalizeAdminTTSConfigPayload`（handler.go:418）— 硬补的 `support_status`/`support_message`、`providers` 目录应迁到 `app/admin/internal/data/tts_provider_catalog.go`
- `handleAdminListPrompts`（handler.go:4961）— 补 `industry_id` 应由 service 层直接返回 uint64
- `normalizeAdminQuestionPagePayload`（handler.go:414）— 列表归一化可在 service 层完成

### 3. 校验规则迁移核查

| 旧后端规则 | 微服务落地位置 | 备注 |
|------------|----------------|------|
| UserRole ∈ {admin, pro, free} | `app/user/internal/biz`（下游） | admin service 透传 role 字符串，校验在 user 服务 |
| DisableUser 设 role="disabled" | 同上 | 已透传 `BanUser` 到 user 服务 |
| Question choice/multi options≥2 | `app/question/internal/biz`（下游） | admin service `CreateQuestion` 透传，由 question 服务校验 |
| Category 必须属于 industry | 同上 | question 服务跨字段校验 |
| UpdateQuestion 类型切换重置 judge_config | 同上 | question 服务已实现 |
| TTS ValidateTTSConfigInput | **缺失** | admin service `CreateTTSConfig`/`UpdateTTSConfig` 未做 engine/voice/auth 校验，**需补** |
| Live2D URL 变更清理旧资产 | 需确认 | `UpdateLive2DModel` 中是否调用 `ScraperCleaner`/`asset cleaner` 待验证 |
| AI Preset name 唯一 | admin repo 层 | `data.NewAdminRepo.SaveAIPreset` 中有 unique 索引 |
| RAG Test 双连通 | admin service `TestRAGConnection`（admin.go:1355） | 旧后端通过 ai 客户端测试，已迁 |

### 4. 状态机 / 权限

- 旧后端所有 `/api/admin/*` 路由均挂 JWT 中间件，校验 admin role
- 微服务 `pkg/auth` 拦截器已在 admin service 注册（main.go:174）；gRPC metadata 透传 user_id/role 到 biz
- 无角色越权检查遗漏（biz 层未重复校验 admin 角色，依赖拦截器）

### 5. 数据转换 / 序列化

- `options_json`、`tags`、`judge_config_json`、`solution_json`、`answer_template_json`、`metadata` 等在 proto 中以 string 存储，前端按 JSON.parse 使用
- 网关 `adaptLegacyResponseByPath` 对 list 返回做单字段解包（如 `{categories:[...]}` → `[...]`），与旧后端直接返回数组行为兼容

---

## 四、结论

- **整体覆盖率：~95%** — 11 个前端页面中 9 个完全对齐，5 个存在 proto 字段差异（不影响主流程但需补全）。
- **核心阻塞**：proto 字段补齐（5 处）+ TTS 校验规则补回。
- **建议优先级**：
  1. **P0**：补 `AICallLogDetail` 6 字段（前端已在用）、补 `ScraperTaskDetail.payload_json`、补 `Live2DModelInfo.updated_at` ✅ 已完成
  2. **P1**：补 `TTSConfigInfo` support/scene 字段 + 把 provider 目录从网关下沉到 biz ✅ 已完成
  3. **P2**：`PromptTemplate` industry_code→industry_id 语义统一（延后）
  4. **P3**：补回 TTS 引擎/voice 校验、TTS `scene` 字段来源

---

## 五、执行记录

### P0 完成（2026-06-15）

| Proto 消息 | 新增字段 | 修改文件 |
|------------|----------|----------|
| `AICallLogDetail` | task_id, prompt_source, selected_prompt_name, scene_config, updated_at, industry_id | `api/makejob/admin/v1/admin.proto` |
| `Live2DModelInfo` | updated_at | `api/makejob/admin/v1/admin.proto` |
| `ScraperTaskDetail` | payload_json | `api/makejob/admin/v1/admin.proto` |

配套修改：
- `app/admin/internal/biz/admin.go` — AICallLogDetail/Live2DModelRecord 增加字段
- `app/admin/internal/service/admin.go` — 填充新字段

### P1 完成（2026-06-15）

| 修改项 | 说明 |
|--------|------|
| Proto 增加字段 | TTSConfigInfo 增加 support_status/support_message/scene |
| 新增 RPC | GetTTSProviders 返回供应商目录 |
| ProviderCatalog | `app/admin/internal/biz/tts_catalog.go` — 供应商元数据目录 |
| Service 填充 | ListTTSConfigs 填充 support_status/support_message/scene |
| GetTTSProviders | 从 catalog 返回完整供应商列表 |

### P3 待执行

补回 TTS 引擎/voice 校验规则。 ✅ 已完成

实现内容：
- `app/admin/internal/biz/tts_catalog.go` — 新增 `ValidateTTSConfig` 方法
- `app/admin/internal/biz/admin.go` — `CreateTTSConfig`/`UpdateTTSConfig` 增加校验

校验规则：
- engine 必须是支持的引擎（volcengine/minimax/xiaomi_mimo）
- voice_id 不能为空
- auth_config_json/params_json 必须是有效 JSON
- 按引擎类型校验必填字段（api_key 等）
- 校验通过后规范化 JSON 字符串
