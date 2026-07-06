# Interview 模块问题修复清单

> 基于 2026-06-27 代码分析，按严重度排序。每项完成后勾选。

---

## 高优先级

### 1. 两套出题路径并存

**问题**：`SubmitAnswer` 走 `GetNextQuestionSession`（会话式），`GetNextQuestion` 走 `InterviewAgent`（老模式），AI 上下文可能不一致。

**位置**：
- `app/interview/internal/biz/usecase.go` L204（SubmitAnswer → GetNextQuestionSession）
- `app/interview/internal/biz/usecase.go` L366（GetNextQuestion → InterviewAgent）

**修复方向**：统一为一种出题路径，或明确两种路径的适用场景并在文档中标注。

- [ ] 分析两种路径的实际差异和调用方
- [ ] 决定统一方案（保留会话式 or 保留老模式 or 按场景区分）
- [ ] 实施修改

---

### 2. 报告生成 MQ 无幂等保护

**问题**：MQ 消费者 `queue.interview.report.generate` 重复消费时会重复调用 AI 生成报告，浪费 token 且可能覆盖数据。

**位置**：
- `app/interview/internal/server/mq.go` L47（消费者入口）
- `app/interview/internal/biz/usecase.go` L700/L726（发布报告生成消息）

**修复方向**：在 GenerateReport 入口检查面试状态，若已是 `completed` 则跳过；或在 report_repo 层做幂等判断。

- [ ] 在 GenerateReport 入口添加状态检查
- [ ] 验证重复消费不会重复调用 AI

---

## 中优先级

### 3. 学习档案异步写入静默失败

**问题**：`_ = uc.archive.WriteEntry(...)` goroutine 里错误被丢弃，数据丢失无感知。

**位置**：`app/interview/internal/biz/usecase.go` L253

**修复方向**：至少记录 warn 日志，或改为同步写入（如果延迟可接受）。

- [ ] 添加错误日志记录
- [ ] 评估是否需要改为同步

---

### 4. CodeRunner 不传测试用例

**问题**：`SubmitCodingAnswer` 传 `testCases=nil`，CodeRunner 只能返回 stdout，无法验证输出正确性。

**位置**：`app/interview/internal/biz/usecase.go` L794

**修复方向**：从题目 payload 中解析测试用例传给 CodeRunner。

- [ ] 确认编程题 payload 中是否携带测试用例
- [ ] 解析并传递给 CodeRunner.Execute

---

### 5. AI Gateway 连接无超时

**问题**：gRPC 连接未设 keepalive/deadline，LLM 调用可能长时间阻塞。

**位置**：`app/interview/internal/data/ai_client.go` L26

**修复方向**：添加 gRPC dial option（keepalive、timeout）或在每次调用时设 context deadline。

- [ ] 为 AI gRPC 连接添加 keepalive 配置
- [ ] 为每个 AI 调用添加 context.WithTimeout

---

### 6. HasCodingArchive 全量遍历

**问题**：加载用户 1000 条记录在内存做幂等检查，性能隐患。

**位置**：`app/interview/internal/biz/usecase.go` L877

**修复方向**：改为服务端过滤（在 LearningArchive 服务端按 source_type 过滤），或添加专用 RPC。

- [ ] 在 archive_client 中添加按 source_type 过滤的查询方法
- [ ] 替换 HasCodingArchive 的全量遍历逻辑

---

## 低优先级

### 7. Gateway 两套路由并存

**问题**：V0（L1595）和 V1（L1813）同时注册，维护成本高。

**位置**：`app/gateway/internal/proxy/handler.go`

**修复方向**：合并为一套路由，保留 V1 版本。

- [ ] 确认前端调用的是哪套路由
- [ ] 移除未使用的路由组

---

### 8. RAG 查询构造不一致

**问题**：SubmitAnswer 用「答案+行业」拼接查询，GetNextQuestion 用「职位描述」，语义差异大。

**位置**：
- `app/interview/internal/biz/usecase.go` L276（SubmitAnswer）
- `app/interview/internal/biz/usecase.go` L350（GetNextQuestion）

**修复方向**：统一查询构造策略，或明确两种构造各自的意图。

- [ ] 分析两种查询的实际检索效果
- [ ] 统一或明确文档标注

---

### 9. 超时硬编码

**问题**：CodeRunner 10s、面试 40min 都写死，不可配置。

**位置**：
- `app/interview/internal/data/code_runner_client.go` L57
- `app/interview/internal/biz/usecase.go`（autoFinishIfExpired 40min）

**修复方向**：移入配置文件。

- [ ] 将超时值移入 `conf.yaml`
- [ ] 通过依赖注入传入

---

## 状态跟踪

| # | 问题 | 状态 | 完成日期 |
|---|------|------|----------|
| 1 | 两套出题路径 | ✅ 已完成 | 2026-06-27 |
| 2 | 报告生成无幂等 | ✅ 已完成 | 2026-06-27 |
| 3 | 学习档案静默失败 | ✅ 已完成 | 2026-06-27 |
| 4 | CodeRunner 不传用例 | ✅ 已完成 | 2026-06-27 |
| 5 | AI 连接无超时 | ✅ 已完成 | 2026-06-27 |
| 6 | HasCodingArchive 全量遍历 | ✅ 已完成 | 2026-06-27 |
| 7 | Gateway 双路由 | ✅ 已完成 | 2026-06-27 |
| 8 | RAG 查询不一致 | ✅ 已完成 | 2026-06-27 |
| 9 | 超时硬编码 | ✅ 已完成 | 2026-06-27 |
