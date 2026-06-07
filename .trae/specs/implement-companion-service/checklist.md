# Checklist

## P5-9: Companion Service - Complete Implementation

### 配置
- [x] conf/conf.go 添加 AI 配置（service_addr）
- [x] conf/conf.go 添加 TTS 配置（api_key, voice）
- [x] configs/config.yaml 包含所有配置

### biz 层
- [x] biz/companion.go 扩展 CompanionSession 实体
- [x] biz/companion.go 添加 CompanionClient 接口
- [x] biz/companion.go 添加 TTSClient 接口
- [x] biz/companion.go 实现 Chat UseCase
- [x] biz/companion.go 实现 GetCompanionState UseCase
- [x] biz/companion.go 实现 SynthesizeSpeech UseCase

### data 层
- [x] data/companion_repo.go 实现 CompanionRepo
- [x] data/tts_client.go 实现 TTSClient
- [x] data/ai_client.go 实现 CompanionClient

### service 层
- [x] service/companion.go 实现 Chat handler
- [x] service/companion.go 实现 GetCompanionState handler
- [x] service/companion.go 实现 SynthesizeSpeech handler

### 启动入口
- [x] main.go 注入新依赖
- [x] go build 编译通过
- [x] go vet 通过
