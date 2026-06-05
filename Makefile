.PHONY: init api build generate clean test

# 初始化
init:
	go mod tidy

# Proto 生成
api:
	buf generate

# 构建所有服务
build:
	go build -o bin/gateway ./app/gateway/cmd/server
	go build -o bin/user ./app/user/cmd/server
	go build -o bin/question ./app/question/cmd/server
	go build -o bin/interview ./app/interview/cmd/server
	go build -o bin/growth ./app/growth/cmd/server
	go build -o bin/admin ./app/admin/cmd/server
	go build -o bin/community ./app/community/cmd/server

# 单独构建
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
