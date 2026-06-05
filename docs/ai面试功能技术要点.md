# AI面试功能技术要点

## 1. 说明与范围

- 本报告仅基于 `backend` 目录进行分析。
- 分析对象是当前项目中原单体后端里的 AI 面试能力实现，不包含正在改造中的其他微服务目录。
- 目标不是做代码审计，而是提炼出后续面试展示时可直接讲清楚的技术亮点、架构思路和工程价值。

## 2. 一句话总结

当前项目里的 AI 面试功能，不只是“调用一次大模型生成题目”这么简单，而是已经形成了一条较完整的后端能力链：

- 支持普通文本面试和实时语音面试两条链路。
- 支持通用面试和简历驱动面试两种模式。
- 支持 RAG 增强、Live2D/TTS/ASR 联动、编程题过程采集、面试报告生成、学习档案沉淀、后续补练推荐与学习计划联动。
- 在实现方式上，已经体现出明显的“面向后续微服务拆分”的设计倾向，比如异步任务、统一消息模型、幂等键、任务状态机、可替换 AI Runtime 抽象等。

如果要用一句更适合面试场景的话来概括，这个模块的亮点是：

> 我们做的不是一个单点 AI 问答接口，而是一套可持续演进的 AI 面试业务闭环。

## 3. 从业务视角看，这个功能已经形成了什么闭环

### 3.1 面试启动

用户可以创建一场面试，后端支持两类入口：

- 普通技术面试：直接进入 `ongoing` 状态，由面试 Agent 生成首题。
- 简历驱动面试：先进入 `preparing` 状态，先做简历解析，得到结构化候选人画像后再进入正式面试。

对应核心实现：

- `backend/internal/service/interview_service.go`
- `backend/internal/service/interview_realtime.go`
- `backend/internal/service/interview_async_support.go`

### 3.2 面试过程

面试过程不只保存“题目和答案”，还保存了完整消息流和题目元数据：

- AI 出题消息
- 用户回答消息
- 题目结构化元数据
- 编程题最终代码
- 编程题过程事件
- 实时语音链路中的字幕、音频、会话恢复状态

这意味着后续不只是能“回看聊天记录”，而是能继续做：

- 报告生成
- 编程题过程诊断
- 会话恢复
- 学习档案沉淀
- 基于历史行为的二次推荐

### 3.3 面试结束后的后处理

面试结束后，系统会继续完成几件真正有价值的事情：

- 对全部回答补齐评分
- 生成结构化面试报告
- 对编程题做过程诊断
- 把薄弱点和建议沉淀到学习档案
- 供后续刷题推荐和学习计划生成复用

这一点非常适合在面试里强调，因为它体现的是“AI 面试不是终点，后续学习闭环才是产品价值”。

## 4. 核心技术架构亮点

## 4.1 双链路设计：文本面试链路 + 实时语音面试链路

这是当前模块最容易拿出来讲的第一个亮点。

### 文本链路

文本链路主要基于 HTTP 接口完成：

- 创建面试
- 提交回答
- 获取下一题
- 结束面试
- 获取报告

对应入口在：

- `backend/internal/handler/interview_handler.go`

这条链路的特点是：

- 简单稳定
- 易于测试
- 适合作为通用保底链路

### 实时语音链路

实时语音链路通过 WebSocket 建立长连接，支持多种事件流：

- 用户语音开始/结束
- 音频块上传
- ASR partial/final 转写
- 实时模型字幕流
- 实时模型音频流
- 打断事件
- Live2D 表情事件
- 会话状态事件

对应入口在：

- `backend/internal/handler/interview_handler.go`
- `backend/internal/handler/interview_handler_realtime.go`

它不是简单地把语音转文字再走 HTTP，而是做成了事件驱动的实时会话协议。这个设计的意义是：

- 交互体验更接近真实面试
- 后端能管理一轮轮 speaking / listening / thinking 状态
- 能处理打断、字幕、音频块、最终落库这些更复杂的实时交互问题

这说明项目在 AI 面试上已经不是“Demo 级调用”，而是开始处理真实产品级实时交互。

## 4.2 AI Runtime 抽象清晰，Provider 可替换

AI 面试没有把模型调用硬编码进业务服务，而是抽象成了 `InterviewAgent`、`ResumeParser`、`QuizAnalyzer` 等接口。

相关实现：

- 抽象定义：`backend/internal/ai/interview_agent.go`、`backend/internal/ai/types.go`、`backend/internal/ai/resume_parser.go`
- 运行时实现：`backend/internal/ai/runtime/interview_agent.go`、`backend/internal/ai/runtime/resume_parser.go`

这种设计的价值有三层：

- 业务层只依赖接口，不依赖具体模型厂商。
- 面试题生成、答题评估、报告生成、简历解析都是独立能力点，可单独替换。
- 后续做微服务拆分时，AI Runtime 可以继续保留为统一能力层，而不需要重写业务逻辑。

这个点在面试里可以这样讲：

> 我们把 AI 能力做成了运行时抽象层，面试服务只关心“开始面试、评估答案、生成报告”这些领域动作，不关心底层到底接的是哪家模型。

## 4.3 结构化 JSON 输出约束，降低模型结果不稳定性

`providerInterviewAgent` 在三个关键环节都采用了结构化输出：

- 生成面试题
- 生成答案反馈
- 生成最终报告

对应实现：

- `interviewQuestionPayloadSchema`
- `interviewFeedbackPayloadSchema`
- `interviewReportPayloadSchema`

位置：

- `backend/internal/ai/runtime/interview_agent.go`

它不是让模型自由输出一段自然语言，而是要求返回固定 JSON 结构，再进行 normalize 和兜底补齐。这带来的工程收益很明显：

- 减少模型输出格式漂移
- 便于前端直接消费结构化字段
- 便于存储和复盘
- 便于加入编程题、Live2D 指令、难度、题型等扩展字段

这说明团队对“大模型不稳定输出”这个问题是有明确工程治理意识的。

## 4.4 本地兜底机制完善，不把核心流程完全绑死在模型上

项目里有多层 fallback：

- 面试题生成失败时，回退到本地题目模板。
- 答案评分失败时，回退到本地规则评分。
- 报告生成失败时，回退到本地聚合报告。
- 编程题 AI 诊断失败时，回退到本地过程诊断。
- MQ 不可用时，简历解析和报告生成可回退本地执行。
- TTS 不可用时，自动退回文本模式。

相关实现：

- `backend/internal/ai/runtime/interview_agent.go`
- `backend/internal/service/interview_async_support.go`
- `backend/internal/service/interview_coding_support.go`
- `backend/internal/handler/interview_handler.go`

这类兜底设计非常适合在面试时展示工程成熟度，因为它表明系统不是“AI 一挂全挂”，而是尽可能保证核心业务链路继续可用。

## 4.5 简历驱动面试模式，不是简单喂简历，而是先做画像结构化

简历驱动模式是当前功能里非常有展示价值的一部分。

实现过程不是把原始简历全文直接塞给模型，而是：

1. 先进入 `preparing` 状态。
2. 调用 `ResumeParser` 提取结构化画像。
3. 画像包含：
   - Summary
   - Skills
   - Projects
   - Strengths
   - WeakSignals
4. 再基于画像构建整场面试的 system prompt。

对应实现：

- `backend/internal/ai/types.go`
- `backend/internal/ai/runtime/resume_parser.go`
- `backend/internal/service/interview_async_support.go`
- `backend/internal/handler/interview_handler_realtime.go`

尤其值得讲的是：简历驱动模式下的 system prompt，不是“第 1 题、第 2 题”的考试式脚本，而是设计成阶段推进：

- 破冰与自我介绍
- 项目深挖与真实性验证
- 技术基础情景化考察
- 工程素养与开放题
- 结束收口

这说明系统开始从“题库问答”走向“更像真实面试官的面试流程控制”。

## 4.6 RAG 不是只在离线问答里用，而是已经接进面试链路

这部分是第二个非常适合重点展示的亮点。

项目中 RAG 在 AI 面试里用了两种方式：

### 方式一：提示词增强

在普通面试链路中，`InterviewRAGService` 作为 `PromptEnhancer` 接入：

- 出题时检索相关知识，增强出题 prompt
- 评分时检索相关知识，增强评估 prompt

对应接入位置：

- `backend/cmd/server/main.go`
- `backend/internal/rag/interview_rag.go`
- `backend/internal/ai/runtime/interview_agent.go`

### 方式二：实时语音 external RAG 注入

在实时语音面试链路中，用户回答结束后，会：

- 根据用户回答、当前话题、技能栈构造增强查询
- 检索相关文档
- 裁剪为 `external_rag` 格式
- 先发送一段安抚话术
- 再把 RAG 结果注入实时模型

对应实现：

- `backend/internal/handler/interview_handler_realtime.go`
- `backend/internal/rag/interview_rag.go`

这说明团队不是只在“静态生成阶段”用 RAG，而是已经在尝试把知识检索实时接入语音对话链路。这是一个很强的展示点，因为它体现了：

- 检索增强不只是服务搜索，也能服务实时对话质量
- 后端已经在处理检索时延和实时体验的平衡
- AI 面试的知识支撑是可演进的

## 4.7 编程题不只看最终答案，还采集过程事件做诊断

这部分是工程含量很高、也很有区分度的亮点。

项目里的编程题设计不是只保存最终代码，而是保留完整过程数据：

- 代码快照
- 运行代码
- 运行结果
- 提交代码
- 长时间停顿

数据模型：

- `backend/internal/model/interview_coding.go`

服务逻辑：

- `backend/internal/service/interview_coding_support.go`
- `backend/internal/service/interview_service.go`

仓储落库：

- `backend/internal/repository/interview_coding_repo.go`

后处理时，系统会结合：

- 题目
- 最终代码
- 最终文字说明
- 过程事件序列

生成 `CodingQuestionDiagnosis`，包括：

- 分数
- MistakeTags
- StrengthTags
- Evidence
- Suggestions
- ProcessSummary

这意味着系统开始具备“分析候选人解题过程”的能力，而不是只看结果对错。这个点在面试中很容易拉开层次：

> 我们采集的不只是答案，而是解题过程数据。这样报告里能分析候选人是卡在状态设计、边界处理、调试路径，还是工程表达上。

## 4.8 面试结果会沉淀为长期学习档案，形成学习闭环

面试报告不是一次性结果，而是会继续沉淀到学习档案表中。

对应实现：

- `persistLearningArchiveEntries`：`backend/internal/service/interview_service.go`
- 学习档案仓储：`backend/internal/repository/interview_coding_repo.go`

沉淀的内容包括：

- 错因标签
- 优势标签
- 建议
- 证据摘要
- 所属面试 ID
- 所属题目序号

这批学习档案后续会继续被两个模块复用：

- 刷题推荐：`backend/internal/service/question_service.go`
- 学习计划聚焦信号：`backend/internal/service/plan_service.go`

这说明 AI 面试模块并不是孤立的，它已经成为整个学习系统的上游信号源之一。

从面试表达上，这个点可以这样说：

> 面试结束后，系统会把弱项结构化沉淀成学习档案，后面的推荐题和学习计划都会消费这些数据，所以它形成了“面试诊断 -> 补练推荐 -> 计划调整”的闭环。

## 4.9 异步任务模型明显带有微服务化演进思路

虽然当前分析范围是单体 `backend`，但 AI 面试模块已经有比较清晰的异步化设计。

### 已异步化的关键后处理

- 简历解析任务
- 面试报告生成任务

相关实现：

- `backend/internal/service/interview_async_support.go`
- `backend/internal/mq/message.go`
- `backend/cmd/worker/main.go`

### 具体工程特征

- 有独立的 `AsyncTask` 记录。
- 有 `queued / running / succeeded / failed / dead` 等状态流转。
- 有幂等键，例如：
  - `interview-resume-parse:{interviewID}`
  - `interview-report-generate:{interviewID}`
- 有 RabbitMQ 路由键和统一任务消息结构。
- worker 端单独消费面试任务。

这部分很适合在面试里强调“为什么这样设计”：

- 简历解析和报告生成都可能比较慢，放同步链路会拉长接口响应时间。
- 实时语音场景下，更需要把重处理动作剥离出去。
- 即使现在还是单体目录，这种设计也能为后续拆服务保留边界。

也就是说，这个模块已经不是传统 CRUD + AI 调用，而是已经在为分布式演进做铺垫。

## 4.10 状态恢复与幂等意识较强

实时面试场景最怕断线、重复提交、重复结束和上下文丢失。这个项目在这方面已经做了不少处理。

### 会话恢复

WebSocket 建立后会先做 `bootstrap`，恢复：

- 当前模式是文本还是实时
- 当前题目
- 当前 Live2D 状态
- 当前 dialog_id
- 当前消息历史

对应实现：

- `backend/internal/handler/interview_handler.go`
- `backend/internal/handler/interview_handler_realtime.go`
- `backend/internal/service/interview_realtime.go`

### 状态判断

面试记录有比较清晰的状态：

- `preparing`
- `ongoing`
- `report_generating`
- `completed`

对应模型：

- `backend/internal/model/mock_interview.go`

### 幂等/重复请求处理

- 已结束面试再次 `FinishInterview`，会直接回到报告读取逻辑。
- 异步任务通过幂等键避免重复创建。
- worker 通过 `ClaimByID` 方式避免重复消费。

这些设计非常有利于展示你对“真实线上业务状态管理”的理解。

## 4.11 多模态体验不是前端单独做的，后端也参与了统一编排

AI 面试还有一个很有产品感的亮点：后端不是只返回文本，而是在统一编排多模态体验。

后端参与的内容包括：

- 为题目生成 TTS 音频
- 处理 ASR 流式识别
- 推送 Live2D 表情/动作指令
- 推送字幕和音频块
- 管理 speaking / listening / thinking / ready 等状态

相关实现：

- `backend/internal/handler/interview_handler.go`
- `backend/internal/handler/interview_handler_realtime.go`
- `backend/internal/service/interview_service.go`
- `backend/cmd/server/main.go`

这意味着后端承担了“互动编排层”的职责，而不是只做普通 API。这个定位本身就很有展示价值。

## 5. 适合在面试时重点强调的 8 个亮点

如果时间有限，建议优先讲下面 8 个点：

1. 支持文本面试和实时语音面试双链路，不是单一问答接口。
2. 简历驱动面试先做结构化画像，再做阶段化追问，更接近真实面试。
3. AI Runtime 做了抽象，Provider、解析器、诊断器可替换。
4. 用结构化 JSON 输出约束模型，降低不稳定性。
5. RAG 不只做静态增强，还接到了实时语音面试链路。
6. 编程题采集过程事件，能分析解题过程而不只是最终答案。
7. 面试结果会沉淀为学习档案，并驱动后续刷题推荐和学习计划。
8. 简历解析和报告生成已经异步化，天然适合后续微服务拆分。

## 6. 一段适合直接复述的项目亮点介绍

可以直接按下面这段思路表达：

> 我负责的 AI 面试模块，不是简单调模型出几道题，而是做成了一条完整业务链。后端同时支持普通文本面试和实时语音面试；在面试模式上既支持通用题模式，也支持基于简历画像的面试模式。  
>  
> 在技术实现上，我们把题目生成、答案评估、报告生成、简历解析都抽象成独立 AI 能力，通过统一 Runtime 来接入模型；为了控制模型输出不稳定，又把题目、反馈、报告都做成了结构化 JSON 协议。  
>  
> 另外，我们把 RAG 接进了面试链路，不只是给模型加知识，还在实时语音面试里做了 external_rag 注入。编程题部分也不是只看最终代码，而是采集了解题过程事件，再做过程诊断。最后这些诊断结果会沉淀成学习档案，继续驱动题目推荐和学习计划，所以它本质上是一个 AI 诊断闭环系统。  
>  
> 从工程上看，这块还做了异步任务、幂等键、worker 消费和状态机设计，所以虽然当前还在单体后端里，但已经为后续微服务拆分预留了清晰边界。

## 7. 如果面试官继续深问，可以从哪些角度展开

### 7.1 为什么要区分文本链路和实时语音链路

可以回答：

- 文本链路适合作为稳定主链路和保底链路。
- 实时语音链路更接近真实面试体验，但工程复杂度高很多。
- 两条链路共享同一份面试记录、报告和后处理模型，可以兼顾稳定性与体验。

### 7.2 为什么要先做简历结构化，而不是直接把简历塞给模型

可以回答：

- 结构化画像更稳定，便于存储、恢复和复用。
- 便于生成更可控的系统提示词。
- 便于后续做项目深挖、技能对齐、JD 匹配等扩展能力。

### 7.3 为什么要做编程题过程采集

可以回答：

- 最终代码只能看结果，看不到候选人卡在什么地方。
- 过程数据能反映调试习惯、边界意识、状态设计能力和自测习惯。
- 这类诊断结果比“对/错”更适合后续训练闭环。

### 7.4 为什么要异步化简历解析和报告生成

可以回答：

- 这两个动作都偏重，容易拉高接口耗时。
- 异步后用户可以更快进入前台状态流转。
- 异步任务天然有利于重试、幂等和后续服务拆分。

## 8. 当前阶段的真实边界与可诚实说明的问题

为了让表达更真实，面试时也可以适度说明当前仍在演进中的部分，这反而会让人觉得你对系统理解更扎实。

### 已经做得比较成熟的部分

- 双链路能力已经成型。
- 简历驱动面试已打通。
- RAG 已接入面试。
- 编程题过程诊断已落库。
- 学习闭环已初步跑通。
- 异步化和微服务演进边界比较清晰。

### 仍然可以继续优化的部分

- 实时语音链路中的部分报告生成仍带有兜底式启发式评分。
- 当前有些字段仍存在“复用旧字段兼容历史数据”的过渡性写法，例如 `AISessionID` / `AIFeedback` 的兼容处理。
- 当前项目还处于微服务改造阶段，模块边界虽然已经显现，但还没有完全物理拆分。

这种回答方式的好处是：既展示亮点，也表现出你知道系统当前在哪个演进阶段。

## 9. 建议重点看的代码文件

如果后续还要继续准备面试，建议优先熟悉这些文件：

- `backend/internal/handler/interview_handler.go`
- `backend/internal/handler/interview_handler_realtime.go`
- `backend/internal/service/interview_service.go`
- `backend/internal/service/interview_realtime.go`
- `backend/internal/service/interview_async_support.go`
- `backend/internal/service/interview_coding_support.go`
- `backend/internal/ai/runtime/interview_agent.go`
- `backend/internal/ai/runtime/resume_parser.go`
- `backend/internal/rag/interview_rag.go`
- `backend/internal/model/mock_interview.go`
- `backend/internal/model/interview_coding.go`
- `backend/internal/repository/interview_repo.go`
- `backend/internal/repository/interview_coding_repo.go`
- `backend/cmd/server/main.go`
- `backend/cmd/worker/main.go`

## 10. 结论

从当前 `backend` 目录的实现来看，AI 面试功能最值得展示的不是“会调大模型”，而是以下三件事已经做出来了：

- 交互层面，做出了文本 + 实时语音 + Live2D/TTS/ASR 联动的多模态面试体验。
- 能力层面，做出了简历驱动、RAG 增强、编程题过程诊断、结构化报告生成等高价值 AI 能力。
- 工程层面，做出了异步任务、状态机、幂等、会话恢复、能力抽象和学习闭环，为后续微服务拆分和持续演进打下了基础。

如果要把这个模块包装成一句最强的面试亮点，可以用下面这句话收束：

> 这个 AI 面试模块已经具备“真实可交互、结果可沉淀、能力可扩展、架构可拆分”的四个特征，它更像一个完整的 AI 面试系统，而不是单点调用模型的功能页。
