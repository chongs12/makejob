# Tasks

## P5-9: Companion Service - Complete Implementation

- [x] Task 1: 修改 conf/conf.go - 添加 AI 和 TTS 配置
- [x] Task 2: 重写 biz/companion.go - 核心业务逻辑
  - [x] SubTask 2.1: 扩展 CompanionSession 实体
  - [x] SubTask 2.2: 添加 CompanionClient 接口（CompanionAgent, Live2DDirector）
  - [x] SubTask 2.3: 添加 TTSClient 接口
  - [x] SubTask 2.4: 实现 Chat UseCase
  - [x] SubTask 2.5: 实现 GetCompanionState UseCase
  - [x] SubTask 2.6: 实现 SynthesizeSpeech UseCase

- [x] Task 3: 重写 data/companion_repo.go - GORM 实现
- [x] Task 4: 新建 data/tts_client.go - TTS 客户端
- [x] Task 5: 新建 data/ai_client.go - AI 客户端
- [x] Task 6: 重写 service/companion.go - gRPC handler
- [x] Task 7: 修改 main.go - 注入新依赖
- [x] Task 8: 更新 configs/config.yaml

# Task Dependencies
- Task 1 无依赖
- Task 2 依赖 Task 1
- Task 3-5 依赖 Task 2
- Task 6 依赖 Task 2
- Task 7 依赖 Task 2-6
- Task 8 依赖 Task 1
