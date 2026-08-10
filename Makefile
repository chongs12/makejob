.PHONY: init api build generate clean test docker-build docker-build-all docker-build-frontend

# 服务清单（15 个后端微服务，用于 docker-build-all）
SERVICES := gateway user membership question interview realtime growth plan companion community learning_archive ai_gateway rag coderunner admin

# 初始化
init:
	go mod tidy

# Proto 生成
api:
	buf generate

# 构建所有服务（本地二进制，输出到 bin/）
build:
	go build -o bin/gateway ./app/gateway/cmd/server
	go build -o bin/user ./app/user/cmd/server
	go build -o bin/membership ./app/membership/cmd/server
	go build -o bin/question ./app/question/cmd/server
	go build -o bin/interview ./app/interview/cmd/server
	go build -o bin/realtime ./app/realtime/cmd/server
	go build -o bin/growth ./app/growth/cmd/server
	go build -o bin/plan ./app/plan/cmd/server
	go build -o bin/companion ./app/companion/cmd/server
	go build -o bin/community ./app/community/cmd/server
	go build -o bin/learning_archive ./app/learning_archive/cmd/server
	go build -o bin/ai_gateway ./app/ai_gateway/cmd/server
	go build -o bin/rag ./app/rag/cmd/server
	go build -o bin/coderunner ./app/coderunner/cmd/server
	go build -o bin/admin ./app/admin/cmd/server

# 单独构建（示例）
build-interview:
	go build -o bin/interview ./app/interview/cmd/server

build-question:
	go build -o bin/question ./app/question/cmd/server

# Wire 依赖注入生成
wire:
	cd app/interview && wire ./...
	cd app/question && wire ./...
	cd app/user && wire ./...
	cd app/growth && wire ./...
	cd app/admin && wire ./...
	cd app/community && wire ./...

# 测试
test:
	go test ./...

# 清理
clean:
	rm -rf bin/

# ===== Docker 构建（对应 checklist 4.4）=====

# 单个后端服务: make docker-build SERVICE=interview
docker-build:
	docker build --build-arg SERVICE_NAME=$(SERVICE) -t makejob/$(SERVICE) .

# 全部 15 个后端服务
docker-build-all:
	@for svc in $(SERVICES); do \
		echo "===== building $$svc ====="; \
		docker build --build-arg SERVICE_NAME=$$svc -t makejob/$$svc . || exit 1; \
	done

# 前端
docker-build-frontend:
	docker build -f frontend-react/Dockerfile -t makejob/frontend .
