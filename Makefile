# ==============================
# 项目基础配置
# ==============================
PROJECT_NAME := go_admin
BINARY_NAME := $(PROJECT_NAME)
VERSION := 0.1.0
BUILD := $(shell git rev-parse --short HEAD)
MAIN := cmd/main.go

# ==============================
# 路径与平台配置
# ==============================
BUILD_DIR := "build"
DIST_DIR := dist
PLATFORMS := windows/amd64 windows/386 darwin/amd64 linux/amd64 linux/386

# ==============================
# 测试参数配置
# ==============================
TEST_FLAGS := -short -cover -race -count=1

# 平台检测
ifeq ($(OS),Windows_NT)
    DETECTED_OS := windows
    BINARY_EXT := .exe
    RMRF := powershell -Command "Remove-Item -Recurse -Force"
    MKDIR := mkdir
    NULL_DEVICE := NUL
    TEST_FLAGS := -short -cover -count=1
else
    DETECTED_OS := $(shell uname | tr '[:upper:]' '[:lower:]')
    BINARY_EXT :=
    RMRF := rm -rf
    MKDIR := mkdir -p
    NULL_DEVICE := /dev/null
endif

# Go 命令
GO := go
GOPATH := $(shell $(GO) env GOPATH)

# 目标文件路径
BINARY := $(BUILD_DIR)/$(BINARY_NAME)$(BINARY_EXT)

# ==============================
# 构建标志与依赖
# ==============================
LD_FLAGS := -X "main.version=$(VERSION)" -X "main.build=$(BUILD)"

# 依赖工具
GOLANGCI_LINT := $(GOPATH)/bin/golangci-lint$(BINARY_EXT)
STATICCHECK := $(GOPATH)/bin/staticcheck$(BINARY_EXT)
GOTESTSUM := $(GOPATH)/bin/gotestsum$(BINARY_EXT)

# ==============================
# .PHONY 目标声明
# ==============================
.PHONY: all clean build run test lint fmt help version update-deps check docker-build docker-run docker-compose-up docker-compose-down docker-compose-logs docker-compose-restart docker-compose-rebuild docker-compose-status docker-compose-clean

# ==============================
# 基础目标
# ==============================
all: help

$(BUILD_DIR) $(DIST_DIR):
	@$(MKDIR) $@

# ==============================
# 依赖管理
# ==============================
DEPS_LOCK := .make_deps_installed

$(DEPS_LOCK):
	@echo "Installing dependencies..."
	@$(GO) mod download
	@echo "Installing development tools..."
	@$(GO) install honnef.co/go/tools/cmd/staticcheck@latest
	@$(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.54.2
	@$(GO) install gotest.tools/gotestsum@latest
	@touch $@

.PHONY: deps
deps: $(DEPS_LOCK) ## Install dependencies and dev tools

.PHONY: update-deps
update-deps: ## Update dependencies and reinstall tools
	@echo "Updating dependencies..."
	@$(GO) get -u ./...
	@$(GO) mod tidy
	@$(RMRF) $(DEPS_LOCK)
	@$(MAKE) deps

# ==============================
# 代码质量检查
# ==============================
fmt: ## Format code
	@echo "Formatting code..."
	@$(GO) fmt -s -w ./

fmt-check: ## Check code formatting
	@echo "Checking code format..."
	@test -z "$$($(GO) fmt -s -l .)"

lint: deps ## Run linters
	@echo "Running linters..."
	@$(STATICCHECK) ./...
	@$(GO) vet ./...

lint-strict: deps ## Run strict linters
	@echo "Running strict linters..."
	@$(GOLANGCI_LINT) run --config .golangci.yml

# ==============================
# 测试与覆盖率
# ==============================
test: deps ## Run tests (short mode)
	@echo "Running tests (short mode, with race detection)..."
	@$(GOTESTSUM) --format testname -- $(TEST_FLAGS) ./...

test-long: deps ## Run tests (verbose mode)
	@echo "Running tests (verbose mode, with race detection)..."
	@$(GOTESTSUM) --format standard-verbose -- -cover -race ./...

coverage: test ## Generate test coverage report
	@echo "Generating test coverage report..."
	@$(GO) test $(TEST_FLAGS) -coverprofile=coverage.out ./...
	@$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

test-xml: deps ## Generate JUnit format test report
	@echo "Running tests and generating JUnit report..."
	@$(GOTESTSUM) --format standard-verbose --junitfile test-results.xml -- $(TEST_FLAGS) ./...

# ==============================
# 构建目标
# ==============================
build: $(BUILD_DIR) ## Build binary for current platform
	@echo "Building binary for current platform..."
	@$(GO) build -o $(BINARY) -ldflags "$(LD_FLAGS)" $(MAIN)
	@echo "Build complete: $(BINARY)"

cross-build: clean $(DIST_DIR) ## Build binaries for multiple platforms
	@echo "Starting cross-platform build..."
	@for platform in $(PLATFORMS); do \
		OS=$$(echo $$platform | cut -f1 -d'/'); \
		ARCH=$$(echo $$platform | cut -f2 -d'/'); \
		output_name=$(DIST_DIR)/$(BINARY_NAME)-$$OS-$$ARCH; \
		if [ $$OS = "windows" ]; then output_name=$$output_name.exe; fi; \
		echo "Building for $$OS/$$ARCH..."; \
		GOOS=$$OS GOARCH=$$ARCH $(GO) build -ldflags "$(LD_FLAGS)" -o $$output_name $(MAIN); \
	done
	@echo "Cross-platform build complete, artifacts in $(DIST_DIR)"

run: build ## Build and run the program
	@echo "Running the program..."
	@./$(BINARY)

# ==============================
# 清理与帮助
# ==============================
CLEAN_TARGETS := $(BUILD_DIR) $(DIST_DIR) coverage.* test-results.xml $(DEPS_LOCK)

clean: ## Clean all build artifacts
	@echo "Cleaning build artifacts..."
	@for target in $(CLEAN_TARGETS); do \
		if [ -e "$$target" ]; then \
			$(RMRF) "$$target" 2>/dev/null || true; \
		fi; \
	done
	@echo "Clean complete"

help: ## Display this help screen
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

check: fmt-check lint test ## Run format check, lint, and tests

# ==============================
# 平台特定命令
# ==============================
ifeq ($(DETECTED_OS),windows)
    # Windows 特定命令
    OPEN_CMD := start
else ifeq ($(DETECTED_OS),darwin)
    # macOS 特定命令
    OPEN_CMD := open
else
    # Linux 和其他 Unix-like 系统
    OPEN_CMD := xdg-open
endif

# ==============================
# 额外的便利目标
# ==============================
open-coverage: coverage ## Open coverage report in browser
	@echo "Opening coverage report in browser..."
	@$(OPEN_CMD) coverage.html 2>$(NULL_DEVICE) || echo "Unable to open coverage report automatically. Please open coverage.html manually."

docker-build: ## Build Docker image
	@echo "Building Docker image..."
	@docker build -t $(PROJECT_NAME):$(VERSION) .

docker-run: docker-build ## Run Docker container
	@echo "Running Docker container..."
	@docker run -it --rm $(PROJECT_NAME):$(VERSION)

# ==============================
# Docker Compose 管理
# ==============================
docker-compose-up: ## Start all services with docker-compose
	@echo "Starting all services..."
	@docker-compose up --build -d

docker-compose-down: ## Stop all services
	@echo "Stopping all services..."
	@docker-compose down

docker-compose-logs: ## View logs from all services
	@echo "Viewing logs..."
	@docker-compose logs -f

docker-compose-restart: ## Restart all services
	@echo "Restarting all services..."
	@docker-compose restart

docker-compose-rebuild: ## Rebuild and restart all services
	@echo "Rebuilding and restarting all services..."
	@docker-compose down
	@docker-compose up --build -d

docker-compose-status: ## Show status of all services
	@echo "Service status:"
	@docker-compose ps

docker-compose-clean: ## Clean up docker resources
	@echo "Cleaning up Docker resources..."
	@docker-compose down --volumes --rmi all
	@docker system prune -f

# ==============================
# 开发工作流目标
# ==============================
dev: fmt lint test build ## Run development cycle
	@echo "Development cycle complete"

ci: fmt-check lint-strict test-xml coverage ## Run CI checks
	@echo "CI checks complete"

# ==============================
# 文档生成目标（如果适用）
# ==============================
docs: ## Generate documentation
	@echo "Generating documentation..."
	@if command -v godoc > $(NULL_DEVICE) 2>&1; then \
		echo "Running godoc server on http://localhost:6060"; \
		godoc -http=:6060; \
	else \
		echo "godoc not installed. Run 'go install golang.org/x/tools/cmd/godoc@latest' to install."; \
	fi

# ==============================
# 默认目标
# ==============================
.DEFAULT_GOAL := help
