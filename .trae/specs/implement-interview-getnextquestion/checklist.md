# Checklist

## P4-1: GetNextQuestion RPC

- [x] conf/conf.go 添加 RAG 服务地址配置
- [x] biz/interview.go 定义 RAGClient 接口
- [x] biz/interview.go 实现 GetNextQuestion UseCase
- [x] GetNextQuestion 验证面试 status == "ongoing"
- [x] GetNextQuestion 保存 user_answer（如果非空）
- [x] GetNextQuestion 加载最近 20 条消息
- [x] GetNextQuestion 调用 RAG.Retrieve（降级处理）
- [x] GetNextQuestion 调用 AI Gateway.InterviewAgent
- [x] GetNextQuestion 保存 AI 回复消息
- [x] GetNextQuestion 更新 current_question_index
- [x] service/interview.go 实现 GetNextQuestion handler
- [x] service/interview.go 添加 toProtoQuestion 转换函数
- [x] main.go 注入 RAG 客户端
- [x] go build 编译通过
- [x] go vet 通过
