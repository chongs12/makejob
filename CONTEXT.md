# MakeJob 项目术语表

## 核心概念

### 可观测性 (Observability)
系统内部状态的可见性程度，通过外部输出（指标、日志、链路追踪）推断系统内部状态的能力。

### 三大支柱 (Three Pillars)
1. **指标 (Metrics)**: 数值型数据，用于监控系统性能和业务指标
2. **日志 (Logs)**: 文本型记录，用于记录事件和错误信息
3. **链路追踪 (Traces)**: 分布式请求的完整路径，用于理解请求流程

### OpenTelemetry
云原生可观测性标准框架，提供统一的API、SDK和工具集，用于生成、收集和导出遥测数据。

## 技术组件

### OTLP (OpenTelemetry Protocol)
OpenTelemetry的标准协议，用于传输遥测数据。支持gRPC和HTTP两种传输方式。

### 跨度 (Span)
链路追踪的基本单位，代表一个操作单元。包含操作名、开始时间、结束时间、属性和事件。

### 采样策略 (Sampling Strategy)
决定哪些请求的链路数据需要采集的策略。包括全量采集、概率采样、智能采样等。

### 上下文传播 (Context Propagation)
在分布式系统中，将链路上下文从一个服务传递到另一个服务的机制。

## 项目特定术语

### 微服务架构
MakeJob采用15个微服务的架构，每个服务负责特定的业务领域。

### Gateway服务
HTTP入口点，负责接收客户端请求并转发到相应的后端服务。

### Kratos框架
Go微服务框架，提供服务发现、负载均衡、中间件等功能。

### gRPC服务间通信
微服务之间使用gRPC进行通信，通过Protobuf定义服务接口。

## 面试域术语

### 简历驱动面试 (Resume-Driven Interview)
用户上传简历文本，系统解析出候选人技能、项目经历、薄弱信号，生成针对性面试问题的面试模式。

### RAG 增强 (RAG Enhancement)
在 AI 出题和评分前，从向量数据库（Milvus）检索相关知识，注入到 LLM prompt 中提升生成质量的机制。Interview 服务负责检索，通过 gRPC `rag_context` 字段传给 AI Gateway。

### 实时语音面试 (Realtime Voice Interview)
通过 WebSocket 连接火山引擎实时语音 API，集成 ASR（语音识别）+ LLM（对话生成）+ TTS（语音合成）的端到端语音面试模式。

### PromptEnhancer 模式
单体架构中 RAG 嵌入在 InterviewAgent 内部的增强模式。微服务中改为 Interview 服务外部检索 + proto 字段传递的解耦模式。

### WeakSignals（候选人薄弱信号）
简历解析时 LLM 提取的候选人潜在薄弱点，用于在简历驱动面试中引导 AI 针对性追问。

## 会员域术语

### 会员套餐 (Membership Tier)
用户当前持有的有效套餐等级，取值为 `free | monthly | quarterly | yearly`。`free` 表示无有效付费套餐。套餐等级是会员状态的唯一事实来源，与后端 `UserMembership.Level` 一致。
_Avoid_: pro（不存在该等级）、VIP、会员等级（歧义）

### 是否付费 (Paid)
由会员套餐派生的布尔状态：`tier !== 'free'` 即为付费。功能门禁（如实时语音面试）只依赖此派生值，不单独持久化。
_Avoid_: isPro、isVip、高级用户

### 实时语音面试门禁 (Realtime Interview Gate)
领域规则：仅付费用户可创建实时语音面试（携带 Live2D 模型 / realtime 模式）；免费用户只能使用 HTTP 文字面试。门禁在面试服务的 `CreateInterview` 域层执行，realtime 服务的 `GetRealtimeContext` 中既有的 `isRealtimeInterview` 校验作为 WebSocket 入口的天然 backstop。

### 模拟支付 (Mock Pay)
预支付阶段用于打通“建单→支付→会员生效”事务的临时机制：前端建单后调用专用的 mock-pay 端点，内部复用 `HandlePaymentCallback` 完成订单转 paid 与会员 upsert。真实支付接入后删除该端点，替换为支付方跳转。

## 学习计划域术语

### 学习阶段 (Learning Phase)
学习计划的阶段划分，包括：foundation（打基础）、drill（专项突破）、review（复盘纠偏）、mock（模拟验证）。阶段推进必须遵循固定顺序，严禁跨阶段或乱序安排。

### 阶段蓝图 (Phase Blueprint)
根据学习计划总时长动态生成的阶段编排规则，定义每个阶段的 day range、预期任务类型和退出标准。

### 任务类型 (Task Type)
学习任务的分类：study（学习）、practice（练习）、interview（模拟面试）、review（复盘）。不同阶段对任务类型有严格约束。

### 学习建议 (Study Suggestion)
基于用户画像（水平、目标、强弱项）生成的 3-5 条简短、可执行的学习建议，使用 Markdown 列表格式返回。

## 刷题分析域术语

### 代码分析 (Code Analysis)
对用户提交的代码或文本答案进行分析评估，返回正确性、评分、问题点、改进建议、错因标签、优势标签、时间复杂度和空间复杂度。

### 编程面试诊断 (Interview Coding Diagnosis)
基于题目、最终代码和过程事件（如编辑、运行、调试）生成的编程面试表现诊断，用于评估候选人的调试路径、问题解决能力和代码质量。

### 过程事件 (Coding Process Event)
编程面试过程中采集的事件，包括代码编辑、运行、调试等操作，用于分析候选人的解题思路和调试能力。

### 答案解析 (Answer Explanation)
对标准答案的结构化解释，帮助用户理解关键思路、常见误区和解题方法。

### 答题提示 (Quiz Hint)
循序渐进的提示，帮助用户推进思路而不直接泄露答案。

## 简历解析域术语

### 简历解析 (Resume Parsing)
从候选人简历文本中提取结构化画像，包括技能、项目经历、优势和薄弱信号。

### 岗位匹配度 (Job Fit)
结合目标岗位描述，分析候选人与岗位的匹配程度和潜在薄弱点。

### 候选人画像 (Resume Profile)
从简历中提取的结构化信息，包括摘要、技能、项目、优势和薄弱信号，用于简历驱动面试模式。

