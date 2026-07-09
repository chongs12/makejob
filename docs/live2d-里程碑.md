# Live2D 功能改造里程碑文档

> ⚠️ **路径迁移说明（2026-07-09）**：本文档前半段（1-10 节）引用的单体 `backend/internal/...` 路径已归档至 `docs/backend/`；后半段（第 11 节起）已是微服务 `app/admin`、`app/gateway` 路径，仍为当前实现。阅读前半段时请将 `backend/internal/...` 对应到 `docs/backend/internal/...`。

## 1. 改造目标

本轮 Live2D 改造围绕一个核心目标展开：让当前项目的 Live2D 能力逐步对齐 `LLM_Live2D` 参考项目，最终实现“大模型根据回复语义直接控制 Live2D 的表情、参数与动作”，并且所有前台可见模型都必须经过后台确认与治理。

当前已经完成的重点，是把原先“前端直接扫描模型文件并展示”的模式，改造成“后台入库确认后才允许前台选择与 LLM 控制”的受管模式；并在此基础上完成第一阶段结构化表情控制，以及第二阶段动作控制主链路。

## 2. 后台模型治理改造

### 2.1 模型确认后才能进入前台

之前存在的问题是：只要模型文件放进资源目录，前端就能直接解析并展示，绕过了后台管理确认流程。  
这一问题已经修正为：

- 模型资源被扫描后，会自动补录到后台管理页对应的 Live2D 模型列表。
- 自动补录的新模型默认是未启用状态。
- 前台接口只返回数据库中已启用、且场景匹配的模型。
- 陪伴页、面试页的模型切换列表也只来自后台接口，不再直接依据本地文件夹内容展示。

这意味着：

- “文件存在”不再等于“前台可见”。
- 是否允许用户切换、是否允许 LLM 使用该模型，完全以后台数据库状态为准。

### 2.2 自动解析与后台入库

为了不增加人工维护成本，后台在列出 Live2D 模型时加入了自动补录逻辑：

- 扫描受管资源目录中的 `.model3.json`
- 识别缩略图、场景默认值、基础配置
- 对未入库资源创建待确认记录
- 已存在数据库记录的资源不会重复创建

这样保留了“自动发现资源”的便利，同时又把最终可见性收回到后台管理。

### 2.3 删除模型修复

此前删除后台模型后页面仍会再次出现，根因是：

- 删除时只删了数据库记录
- 本地资源目录仍然保留
- 后台再次扫描目录时会把模型重新补录回来

这部分已修正为：

- 删除模型时会同步删除对应受管资源目录
- 如果多个数据库记录共用同一资源目录，则只删除记录，不误删共享文件
- 删除后再次进入后台列表，不会被自动补回

## 3. 后台管理页与舞台 UI 改造

### 3.1 舞台背景上传修复

此前后台管理页“上传舞台背景图”按钮点击无反应，已做排查与修复。修复点包括：

- 上传按钮与实际文件选择逻辑重新连通
- 背景图上传后的后台配置写入恢复生效
- 前台陪伴页与面试页舞台支持读取后台配置的背景图地址
- 背景图加载失败时前台会自动降级，不阻塞模型舞台

### 3.2 后台按钮样式修复

此前后台管理页多个操作控件只是文字，没有明确按钮视觉样式。  
这一轮统一补了按钮样式，让操作控件具备清晰的按钮外观、层级和点击反馈，避免看起来像普通文本。

## 4. 第一阶段：LLM 结构化表情控制

### 4.1 后端生成结构化指令

第一阶段已经完成“后端生成、前端消费”的结构化 Live2D 指令链路，核心思路是：

- 后端根据当前已选模型解析出 manifest
- manifest 包含该模型真实支持的表达式白名单与参数白名单
- LLM 根据回复语义输出结构化 `live2d_directive`
- 后端对 LLM 输出做白名单过滤与归一化
- 前端只执行后端下发结果，不自行猜测

已支持的结构包括：

- `expression_mix`
- `parameter_overrides`
- `mouth_open`

### 4.2 接入范围

第一阶段已经接入：

- 陪伴聊天场景
- 模拟面试场景
- 面试刷新恢复链路
- 面试 WebSocket 表情/状态推送链路

### 4.3 第一阶段关键文件

后端关键文件包括：

- `backend/internal/ai/types.go`
- `backend/internal/ai/runtime/live2d_director.go`
- `backend/internal/service/live2d_directive_service.go`
- `backend/internal/service/companion_service.go`
- `backend/internal/service/interview_service.go`

前端关键文件包括：

- `frontend-react/apps/web/src/shared/live2dDirective.ts`
- `frontend-react/apps/web/src/shared/live2dStagePresets.ts`
- `frontend-react/apps/web/src/shared/live2dStageRuntime.ts`
- `frontend-react/apps/web/src/shared/Live2DSceneStage.tsx`
- `frontend-react/apps/web/src/features/companion/CompanionLive2DStage.tsx`
- `frontend-react/apps/web/src/features/interview/InterviewLive2DStage.tsx`

## 5. 第二阶段：LLM 动作控制

### 5.1 本轮新增目标

本轮开始补齐与参考项目更接近的“动作控制”能力，也就是让聊天回复不只切表情和参数，还能真正播放 `motion3.json` 动作。

### 5.2 后端新增能力

后端本轮新增了 motion 相关的 manifest 与 directive 能力：

- `Live2DManifest` 新增 `motions`
- `Live2DDirective` 新增：
  - `motion_key`
  - `motion_group`
  - `motion_priority`
  - `motion_duration_ms`

后台解析动作时采用两级策略：

1. 优先解析 `model3.json` 中 `FileReferences.Motions`
2. 若模型未声明动作，则回退扫描模型目录中的 `.motion3.json`

同时坚持白名单原则：

- 只有后台已确认启用模型的动作才会进入 manifest
- LLM 只能从 manifest 提供的动作键里选择
- 后端会再次过滤非法 `motion_key`

### 5.3 前端新增能力

前端运行时本轮新增了真实动作播放链路：

- 从模型 settings 中发现动作列表
- 建立 `motion_key -> group/index` 映射
- 在收到后端 directive 时调用 `pixi-live2d-display` 的 motion API 播放动作
- 与现有表情层、参数层叠加共存

本轮动作执行策略是一个稳定、保守的 v1：

- 单条回复最多触发一个动作
- 同动作短时间重复下发会节流，避免抖动重播
- 动作播放失败时仅降级，不影响整页
- 不做复杂动作队列，保持当前架构简单稳定

### 5.4 动作状态展示

为了便于调试与验收，舞台控制面板也补充了动作展示：

- 可看到当前模型发现到的动作数量
- 可看到当前回复触发的动作标签

### 5.5 运行时加载时序修复进度

2026-05-19 对面试页与虚拟陪伴页新增排障修复，处理了打开页面时前端报错：

- `Could not find Cubism 4 runtime. This plugin requires live2dcubismcore.js to be loaded.`

本次排查结论是：

- `live2dcubismcore.min.js` 静态文件本身仍存在于受管资源目录 `live2d-src`
- 真正问题不是资源缺失，而是前端运行时在模块顶层提前静态导入了 `pixi-live2d-display/cubism4`
- 该静态导入会在动态注入 `live2dcubismcore.min.js` 之前就触发 Cubism4 模块初始化，从而在页面首屏直接报错

本次已完成修复：

- 将 `MotionPriority` 改为与 `Live2DModel` 一样通过 `loadLive2DRuntime()` 延迟获取
- 移除会导致 Cubism4 提前初始化的顶层值导入
- 前端 `npm run build:web` 已通过，说明当前面试页与虚拟陪伴页的 Live2D 运行时打包链路正常

### 5.6 动作发现补齐修复进度

2026-05-19 继续排查“模型控制概览始终显示 0 个动作”的问题，结论如下：

- 当前仓库内并不是所有模型都没有动作文件
- 实际检查结果为：
  - `yumi`：存在 `2` 个 `.motion3.json`
  - `藿藿`：存在 `7` 个 `.motion3.json`
  - `ariu` / `兔子洞` / `小恶魔` / `符玄`：当前未发现 `.motion3.json`
- 上述模型的原始 `model3.json` 都没有声明 `FileReferences.Motions`

这次导致前端一直显示 `0 个动作` 的根因是：

- 后端 manifest 已经支持“当 `model3.json` 未声明 `Motions` 时，回退扫描模型目录中的 `.motion3.json`”
- 但前端舞台此前只读取原始 `model3.json`，没有消费后端补录出的动作清单
- 因此前端面板看不到动作，`motion_key` 指令也无法在这些模型上正确落到运行时

本次已完成修复：

- 前台 `/live2d/models` 响应新增携带当前模型的动作清单
- 前端在加载 Live2D 模型前，会把后端返回的动作清单补进运行时 settings
- 对于原始 `model3.json` 未声明动作、但目录内真实存在 `.motion3.json` 的模型，前端现在可以正确识别动作数量并参与 LLM 动作控制
- 后端 `go test ./internal/service` 已通过，前端 `npm run build:web` 已通过

## 6. 当前代码数据流

### 6.1 陪伴聊天链路

1. 前端选择后台允许的模型，并把 `live2d_model_key` 传给后端
2. 后端按该模型生成 Live2D manifest
3. LLM 输出 `live2d_directive`
4. 后端过滤后返回前端
5. 前端舞台运行时同时执行：
   - 表情混合
   - 参数覆盖
   - 嘴型
   - 动作播放

### 6.2 模拟面试链路

1. 面试记录中持久化当前使用的 `live2d_model_key`
2. 出题或表达更新时，后端附带 `live2d_directive`
3. 前端刷新恢复或 WebSocket 推送后继续按 directive 恢复舞台状态

## 7. 当前与参考项目的差距

虽然当前已经进入“LLM 可直接控制表情 + 参数 + 动作”的阶段，但与 `LLM_Live2D` 参考项目相比，仍有差距：

- 还没有做复杂动作编排和排队
- 还没有做更完整的持续状态协议
- 还没有实现更细粒度的多层动作语义控制

但核心控制闭环已经建立完成：

- 后台治理
- 白名单 manifest
- LLM 结构化输出
- 前端真实动作执行

这也是后续继续逼近参考项目效果的基础。

## 8. 本轮重点文件清单

本轮与 Live2D 直接相关的主要修改文件包括：

- `backend/internal/ai/types.go`
- `backend/internal/ai/runtime/live2d_director.go`
- `backend/internal/service/live2d_directive_service.go`
- `backend/internal/service/live2d_directive_service_test.go`
- `frontend-react/apps/web/src/shared/live2dDirective.ts`
- `frontend-react/apps/web/src/shared/live2dStagePresets.ts`
- `frontend-react/apps/web/src/shared/live2dStageRuntime.ts`
- `frontend-react/apps/web/src/shared/Live2DSceneStage.tsx`

## 9. 当前结论

截至本里程碑，Live2D 相关能力已从“资源能显示”推进到“后台治理 + LLM 控制”的完整主链路：

- 模型必须后台确认后才能进入前台
- 自动解析会补录后台记录
- 删除会同步处理受管资源，避免复活
- 舞台背景管理恢复可用
- 后台按钮具备明确 UI
- 第一阶段已实现结构化表情控制
- 第二阶段已开始接入真实动作控制

后续若继续追平参考项目，重点会落在更复杂的持续状态、多动作编排和更细腻的跨轮次动作语义控制上。
## 10. 2026-05-19 运行时补充修复

### 10.1 AI 预设切换立即生效

- 根因确认：后端之前只在启动时构建一次 AI runtime，后续陪伴、面试、刷题分析、学习计划等服务拿到的都是启动时那一份静态 Agent
- 结果表现：后台“应用预设”虽然已经把新配置写入数据库，实际请求仍继续使用旧 Agent，只有重启后端后才会切到新配置
- 本次修复：新增动态 runtime manager，在每次调用时按当前后台配置复用或重建 AI client
- 面试场景额外保留“会话绑定 Agent”，避免切换配置后把进行中的面试 session 直接切断
- 修复后效果：新预设应用成功后，新的 AI 请求可立即命中新配置，不再依赖后端重启

### 10.2 Live2D 指令生成延迟兜底

- 当前陪伴页与面试页在主回复之外，还会额外触发一轮 `live2d_directive` LLM 生成
- 这条额外链路一旦较慢，会直接拉长首个可见回复时间
- 本次修复：为陪伴场景和面试场景的 Live2D 指令生成增加短超时兜底
- 超时或失败时，主回复/主问题仍直接返回，Live2D 指令静默降级，不再阻塞主链路

### 10.3 本轮验证

- `go test ./internal/ai/runtime`
- `go test ./internal/service`
- `go build ./...`

补充说明：

- 这次改造解决的是“配置切换不生效”和“额外 Live2D 指令拖慢整体响应”的问题
- 但若主回复本身使用的远端模型较慢，整体响应时间仍主要受上游模型速度影响，无法仅靠本地改造硬性保证 2 秒内返回

### 10.4 舞台台词遮挡与陪伴 TTS 复用

- 共享 `Live2DSceneStage` 对话框新增最大可视高度限制，按当前行高控制在约 `4` 行以内
- 长文本不再继续撑高舞台浮层遮挡模型，而是在对话框内部滚动，并在流式输出时自动滚动到最新内容
- 面试页原有的“字幕打字机 + TTS 播放 + 音频驱动嘴型”逻辑已抽成共享 hook，避免陪伴页再维护一套独立实现
- 陪伴页右侧舞台不再一次性展示整段回复，而是改为和面试页一致的流式字幕效果
- 陪伴聊天接口响应新增可选 TTS 字段：
  - `audio_url`
  - `audio_duration`
- `audio_format`
- `audio_sample_rate`
- 陪伴场景现已复用与面试场景相同的 TTS Provider；若未配置 TTS 或播放失败，会自动降级为纯文本流式字幕，不阻塞主对话

### 10.5 TTS 配置改为由 Live2D 绑定驱动

- 后台 `TTS 配置` 页不再按 `scene` 直接声明用途，而是改为维护“供应商 + 鉴权 JSON + 官方参数 JSON”的真实可运行配置
- 当前运行时正式支持并校验：
  - `volcengine`：豆包语音 / 火山语音
  - `minimax`：MiniMax 官方 TTS
  - `xiaomi_mimo`：Xiaomi MiMo 官方 TTS，当前仅接入 `mimo-v2-tts` 与 `mimo-v2.5-tts`
- 旧的固定引擎项（如 `elevenlabs`、`aliyun`、`xunfei`）仍可作为遗留记录显示，但不会再作为新的可运行配置入口
- 后台 `Live2D 管理` 页新增 `绑定 TTS 配置` 字段；模型一旦绑定，陪伴页与面试页的语音播报都会优先使用该绑定配置
- 若当前 Live2D 模型未绑定 TTS，则按场景默认配置回退；场景默认也未配置时，最后回退到 `backend/config.yaml`
- 公开 Live2D 配置响应中的旧 `voice_source` 默认值已移除，改为透出实际绑定的 `tts_config_id`

### 10.6 本轮验证

- `go test ./...`
- `frontend-react: npm.cmd run build:admin`

补充说明：

- 当前豆包语音可直接参考 `backend/config.yaml` 中已验证可用的 `resource_id`、`voice_type` 与鉴权方式录入后台配置
- Xiaomi MiMo 现已按官方 OpenAI 风格接口接入，鉴权头使用 `api-key`，默认地址为 `https://api.xiaomimimo.com/v1/chat/completions`
- 当前 MiMo 后台模板仅开放本项目确认会用到的两个模型：`mimo-v2-tts`、`mimo-v2.5-tts`
- MiMo 的官方 `audio.voice` 参数统一复用后台表单顶部的 `Voice ID` 字段，不再在 `params_json` 里重复声明
- MiniMax 后台模板已同步切到当前官方地址 `https://api.minimax.io/v1/t2a_v2`，`group_id` 改为旧版兼容可选项
- MiniMax 后台模板的默认模型值已同步更新为当前实现使用的 `speech-2.8-turbo`

## 11. 2026-06-08 P1 补齐

- `Live2D 前台查询` 已正式接入微服务：
  - `Admin` 成为前台 Live2D 查询的唯一后端真相源
  - 新增前台 RPC：`ListSelectableLive2DModels`、`GetCurrentLive2DModel`
  - `Gateway` 公开路由已补齐：`/api/v1/live2d/models`、`/api/v1/live2d/current`
  - 为兼容当前 Web 端，保留旧路径：`/api/live2d/models`、`/api/live2d/current`
- `Live2D 导入链路` 已补齐：
  - `ImportLive2DPackage` 可直接导入 ZIP 包并生成待确认模型记录
  - `ImportLive2DBackground` 可上传舞台背景图并返回静态访问地址
  - `Gateway` 已挂载 `/live2d-assets/*`，前台可直接访问导入后的模型和背景资源
- 本轮后端验证：
  - `go test ./pkg/live2dassets ./app/admin/... ./app/gateway/...`
  - `go build ./app/admin/cmd/server`
  - `go build ./app/gateway/cmd/server`
