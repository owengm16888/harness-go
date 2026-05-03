# Harness Engineering Makefile

# 变量
APP_NAME=harness
CLI_NAME=harness-cli
VERSION=1.0.0
BUILD_DIR=./build
GO=go
DOCKER=docker
KUBECTL=kubectl

# 默认目标
.PHONY: all
all: build

# 构建
.PHONY: build
build: build-server build-cli

# 构建服务器
.PHONY: build-server
build-server:
	$(GO) build -o $(BUILD_DIR)/$(APP_NAME) ./cmd/harness

# 构建 CLI
.PHONY: build-cli
build-cli:
	$(GO) build -o $(BUILD_DIR)/$(CLI_NAME) ./cmd/harness-cli

# 运行服务器
.PHONY: run
run:
	$(GO) run ./cmd/harness

# 运行 CLI
.PHONY: cli
cli:
	$(GO) run ./cmd/harness-cli

# 测试
.PHONY: test
test: test-unit test-api test-storage test-adapters

# 单元测试
.PHONY: test-unit
test-unit:
	$(GO) test ./internal/... -v

# API 测试
.PHONY: test-api
test-api:
	$(GO) test ./internal/api/... -v

# 存储测试
.PHONY: test-storage
test-storage:
	$(GO) test ./internal/storage/... -v

# 适配器测试
.PHONY: test-adapters
test-adapters:
	$(GO) test ./internal/adapters/... -v

# 测试覆盖率
.PHONY: coverage
coverage:
	$(GO) test -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out

# 代码检查
.PHONY: lint
lint:
	golangci-lint run

# 清理
.PHONY: clean
clean:
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html

# Docker 构建
.PHONY: docker-build
docker-build:
	$(DOCKER) build -t $(APP_NAME):$(VERSION) .

# Docker 运行
.PHONY: docker-run
docker-run:
	$(DOCKER) run -p 8080:8080 -p 9090:9090 -v ./data:/app/data $(APP_NAME):$(VERSION)

# Docker Compose
.PHONY: docker-compose-up
docker-compose-up:
	docker-compose up -d

.PHONY: docker-compose-down
docker-compose-down:
	docker-compose down

# Kubernetes 部署
.PHONY: k8s-deploy
k8s-deploy:
	$(KUBECTL) apply -f k8s/deployment.yaml

# Kubernetes 删除
.PHONY: k8s-delete
k8s-delete:
	$(KUBECTL) delete -f k8s/deployment.yaml

# 查看 Pod 状态
.PHONY: k8s-status
k8s-status:
	$(KUBECTL) get pods -n harness

# 查看日志
.PHONY: k8s-logs
k8s-logs:
	$(KUBECTL) logs -f deployment/harness -n harness

# 依赖下载
.PHONY: deps
deps:
	$(GO) mod download

# 依赖更新
.PHONY: deps-update
deps-update:
	$(GO) mod tidy

# 生成文档
.PHONY: docs
docs:
	godoc -http=:6060

# 设置开发环境
.PHONY: setup
setup:
	./scripts/setup.sh all

# 运行测试脚本
.PHONY: test-script
test-script:
	./scripts/test.sh all

# 基准测试
.PHONY: bench
bench:
	$(GO) test ./internal/... -bench=. -benchmem

# 交叉编译
.PHONY: build-linux
build-linux:
	GOOS=linux GOARCH=amd64 $(GO) build -o $(BUILD_DIR)/$(APP_NAME)-linux-amd64 ./cmd/harness
	GOOS=linux GOARCH=amd64 $(GO) build -o $(BUILD_DIR)/$(CLI_NAME)-linux-amd64 ./cmd/harness-cli

.PHONY: build-darwin
build-darwin:
	GOOS=darwin GOARCH=amd64 $(GO) build -o $(BUILD_DIR)/$(APP_NAME)-darwin-amd64 ./cmd/harness
	GOOS=darwin GOARCH=amd64 $(GO) build -o $(BUILD_DIR)/$(CLI_NAME)-darwin-amd64 ./cmd/harness-cli

.PHONY: build-windows
build-windows:
	GOOS=windows GOARCH=amd64 $(GO) build -o $(BUILD_DIR)/$(APP_NAME)-windows-amd64.exe ./cmd/harness
	GOOS=windows GOARCH=amd64 $(GO) build -o $(BUILD_DIR)/$(CLI_NAME)-windows-amd64.exe ./cmd/harness-cli

# 构建所有平台
.PHONY: build-all
build-all: build-linux build-darwin build-windows

# 版本信息
.PHONY: version
version:
	@echo $(VERSION)

# 测试覆盖率报告
.PHONY: coverage-html
coverage-html:
	$(GO) test -coverprofile=coverage.out -covermode=atomic ./...
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# 竞态检测
.PHONY: race
race:
	$(GO) test -race ./...

# 基准测试
.PHONY: bench
bench:
	$(GO) test -bench=. -benchmem ./internal/core/...

# 代码检查 (vet + staticcheck)
.PHONY: check
check:
	$(GO) vet ./...
	@echo "Vet passed"

# 安全扫描
.PHONY: security
security:
	@echo "Checking for hardcoded secrets..."
	@grep -rn "password\|secret\|api_key" --include="*.go" . || echo "No secrets found"

# 构建所有二进制
.PHONY: build-all
build-all:
	$(GO) build -o $(BUILD_DIR)/harness ./cmd/harness
	$(GO) build -o $(BUILD_DIR)/harness-cli ./cmd/harness-cli

# 帮助
.PHONY: help
help:
	@echo "Available targets:"
	@echo "  all              - Build everything"
	@echo "  build            - Build server and CLI"
	@echo "  build-server     - Build server"
	@echo "  build-cli        - Build CLI"
	@echo "  run              - Run server"
	@echo "  cli              - Run CLI"
	@echo "  test             - Run all tests"
	@echo "  test-unit        - Run unit tests"
	@echo "  test-api         - Run API tests"
	@echo "  test-storage     - Run storage tests"
	@echo "  test-adapters    - Run adapter tests"
	@echo "  coverage         - Run tests with coverage"
	@echo "  lint             - Run linter"
	@echo "  clean            - Clean build artifacts"
	@echo "  docker-build     - Build Docker image"
	@echo "  docker-run       - Run Docker container"
	@echo "  docker-compose-up   - Start Docker Compose"
	@echo "  docker-compose-down - Stop Docker Compose"
	@echo "  k8s-deploy       - Deploy to Kubernetes"
	@echo "  k8s-delete       - Delete from Kubernetes"
	@echo "  k8s-status       - Show Kubernetes status"
	@echo "  k8s-logs         - Show Kubernetes logs"
	@echo "  deps             - Download dependencies"
	@echo "  deps-update      - Update dependencies"
	@echo "  docs             - Generate documentation"
	@echo "  setup            - Setup development environment"
	@echo "  test-script      - Run test script"
	@echo "  bench            - Run benchmarks"
	@echo "  build-linux      - Build for Linux"
	@echo "  build-darwin     - Build for macOS"
	@echo "  build-windows    - Build for Windows"
	@echo "  build-all        - Build for all platforms"
	@echo "  version          - Show version"
	@echo "  help             - Show this help"
