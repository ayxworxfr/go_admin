# ==============================
# 项目基础配置
# ==============================
PROJECT_NAME := go_admin
BINARY_NAME := $(PROJECT_NAME)
# 可用 make build VERSION=1.2.0 / make release VERSION=1.2.0 覆盖
VERSION ?= 0.1.0
BUILD := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
MAIN := cmd/main.go
REMOTE ?= origin

# ==============================
# 路径与平台配置
# ==============================
# 注意：BUILD_DIR 不能写成 "build"（引号会进变量值），也不能把目录本身
# 做成与 phony 目标同名的 make target，否则会变成 build: build 循环依赖。
BUILD_DIR := build
DIST_DIR := dist
PLATFORMS := windows/amd64 windows/386 darwin/amd64 linux/amd64 linux/386

# ==============================
# 测试参数配置
# ==============================
TEST_FLAGS := -short -cover -race -count=1

# 平台检测（二进制后缀 / race 等）；文件操作用下方统一的 POSIX 命令。
# Windows 请用 Git Bash / MSYS 跑 make（本文件已大量依赖 for/test/[ 语法）。
ifeq ($(OS),Windows_NT)
    DETECTED_OS := windows
    BINARY_EXT := .exe
    TEST_FLAGS := -short -cover -count=1
else
    DETECTED_OS := $(shell uname | tr '[:upper:]' '[:lower:]')
    BINARY_EXT :=
endif

# 统一 POSIX 文件操作，macOS / Linux / Git-Bash-on-Windows 均可
RMRF := rm -rf
MKDIR := mkdir -p
NULL_DEVICE := /dev/null

# Go 命令
GO := go
GOPATH := $(shell $(GO) env GOPATH)

# 目标文件路径
BINARY := $(BUILD_DIR)/$(BINARY_NAME)$(BINARY_EXT)

# ==============================
# 构建标志与依赖
# ==============================
LD_FLAGS := -X "main.version=$(VERSION)" -X "main.build=$(BUILD)"

# 开发工具版本（改这里会自动重装；golangci 用官方二进制以匹配 Go 1.24）
STATICCHECK_VERSION := v0.6.1
GOLANGCI_LINT_VERSION := v1.64.8
GOTESTSUM_VERSION := v1.12.3
TOOLS_STAMP := staticcheck@$(STATICCHECK_VERSION)+golangci-lint@$(GOLANGCI_LINT_VERSION)+gotestsum@$(GOTESTSUM_VERSION)

GOLANGCI_LINT := $(GOPATH)/bin/golangci-lint$(BINARY_EXT)
STATICCHECK := $(GOPATH)/bin/staticcheck$(BINARY_EXT)
GOTESTSUM := $(GOPATH)/bin/gotestsum$(BINARY_EXT)

# ==============================
# .PHONY 目标声明
# ==============================
.PHONY: all clean build generate run test lint lint-strict fmt help version update-deps deps check \
	docker-build docker-run docker-compose-up docker-compose-down docker-compose-logs \
	docker-compose-restart docker-compose-rebuild docker-compose-status docker-compose-clean \
	tag release tag-push

# ==============================
# 基础目标
# ==============================
all: help

# ==============================
# 依赖管理
# ==============================
DEPS_LOCK := .make_deps_installed

deps: ## Install dependencies and pinned dev tools
	@stamp="$(TOOLS_STAMP)"; \
	if [ -f "$(DEPS_LOCK)" ] && [ "$$(cat "$(DEPS_LOCK)" 2>$(NULL_DEVICE))" = "$$stamp" ]; then \
		exit 0; \
	fi; \
	echo "Installing module deps & tools ($$stamp)..."; \
	$(GO) mod download; \
	$(GO) install honnef.co/go/tools/cmd/staticcheck@$(STATICCHECK_VERSION); \
	$(GO) install gotest.tools/gotestsum@$(GOTESTSUM_VERSION); \
	echo "Installing golangci-lint $(GOLANGCI_LINT_VERSION) (official binary)..."; \
	if command -v curl >/dev/null 2>&1; then \
		curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh \
			| sh -s -- -b "$(GOPATH)/bin" $(GOLANGCI_LINT_VERSION); \
	else \
		$(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION); \
	fi; \
	printf '%s\n' "$$stamp" > "$(DEPS_LOCK)"; \
	echo "Tools ready."

update-deps: ## Update module deps and reinstall tools
	@echo "Updating dependencies..."
	@$(GO) get -u ./...
	@$(GO) mod tidy
	@$(RMRF) $(DEPS_LOCK)
	@$(MAKE) deps

# ==============================
# 代码质量检查
# ==============================
fmt: ## Format code (gofmt -s：含简化重写)
	@echo "Formatting code..."
	@gofmt -s -w .

fmt-check: ## Check code formatting
	@echo "Checking code format..."
	@test -z "$$(gofmt -s -l .)"

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
generate: ## Scan // @route and regenerate routes_gen.go
	@echo "Generating compiled route table..."
	@$(GO) run ./cmd/routegen -root .
	@echo "Route table up to date"

build: generate ## Build binary for current platform
	@echo "Building binary for current platform..."
	@$(MKDIR) $(BUILD_DIR)
	@$(GO) build -o $(BINARY) -ldflags "$(LD_FLAGS)" $(MAIN)
	@echo "Build complete: $(BINARY)"

cross-build: clean generate ## Build binaries for multiple platforms
	@echo "Starting cross-platform build..."
	@$(MKDIR) $(DIST_DIR)
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
# 发版（逻辑在 scripts/release-tag.sh）
# ==============================
#   make tag VERSION=1.2.0
#   make release VERSION=1.2.0
#   make release VERSION=1.2.0-rc.1 REMOTE=origin
tag: ## Create annotated tag; usage: make tag VERSION=1.2.0
	@test "$(origin VERSION)" = "command line" || { echo "usage: make tag VERSION=1.2.0" >&2; exit 1; }
	@./scripts/release-tag.sh tag "$(VERSION)"

release: ## Tag and push (triggers GitHub Release); usage: make release VERSION=1.2.0
	@test "$(origin VERSION)" = "command line" || { echo "usage: make release VERSION=1.2.0" >&2; exit 1; }
	@./scripts/release-tag.sh release "$(VERSION)" "$(REMOTE)"

tag-push: release ## Alias for release

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
