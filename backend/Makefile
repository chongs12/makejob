# MakeJob Backend Makefile
# 提供常用的构建、运行和清理命令

# 应用名称
APP_NAME = server
BINARY = bin/$(APP_NAME)

# Go 命令
GOCMD = go
GOBUILD = $(GOCMD) build
GOCLEAN = $(GOCMD) clean
GOTEST = $(GOCMD) test
GOGET = $(GOCMD) get
GOMOD = $(GOCMD) mod

# 主程序入口
MAIN_FILE = cmd/server/main.go

.PHONY: all build run clean test tidy fmt vet lint

# 默认目标：构建
all: build

# 构建可执行文件
build:
	@echo "正在构建 $(APP_NAME)..."
	@mkdir -p bin
	$(GOBUILD) -o $(BINARY) $(MAIN_FILE)
	@echo "构建完成: $(BINARY)"

# 运行开发服务器
run:
	@echo "启动开发服务器..."
	$(GOCMD) run $(MAIN_FILE)

# 清理构建产物
clean:
	@echo "清理构建产物..."
	$(GOCLEAN)
	@rm -rf bin/
	@echo "清理完成"

# 运行测试
test:
	@echo "运行测试..."
	$(GOTEST) -v ./...

# 整理依赖
tidy:
	@echo "整理 Go 模块依赖..."
	$(GOMOD) tidy
	@echo "依赖整理完成"

# 格式化代码
fmt:
	@echo "格式化代码..."
	$(GOCMD) fmt ./...

# 静态分析
vet:
	@echo "运行 go vet..."
	$(GOCMD) vet ./...

# 下载依赖
deps:
	@echo "下载依赖..."
	$(GOMOD) download

# 完整检查（格式化 + 静态分析 + 测试）
check: fmt vet test
	@echo "代码检查完成"

# 生产构建（优化）
build-prod:
	@echo "构建生产版本..."
	@mkdir -p bin
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) -ldflags="-w -s" -o $(BINARY) $(MAIN_FILE)
	@echo "生产版本构建完成"

# Windows 生产构建
build-windows:
	@echo "构建 Windows 生产版本..."
	@mkdir -p bin
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GOBUILD) -ldflags="-w -s" -o $(BINARY).exe $(MAIN_FILE)
	@echo "Windows 版本构建完成"

# 帮助信息
help:
	@echo "MakeJob Backend 可用命令:"
	@echo "  make build        - 构建可执行文件"
	@echo "  make run          - 运行开发服务器"
	@echo "  make clean        - 清理构建产物"
	@echo "  make test         - 运行所有测试"
	@echo "  make tidy         - 整理 Go 模块依赖"
	@echo "  make fmt          - 格式化代码"
	@echo "  make vet          - 运行静态分析"
	@echo "  make check        - 完整代码检查（fmt + vet + test）"
	@echo "  make build-prod   - 构建 Linux 生产版本"
	@echo "  make build-windows- 构建 Windows 生产版本"
	@echo "  make help         - 显示此帮助信息"
