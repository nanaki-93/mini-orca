# Mini-Orca Makefile

# Variables
APP_NAME := mini-orca
VERSION := $(shell sed -n 's/.*Version = "\([^"]*\)"/\1/p' internal/version/version.go)
IMAGE_NAME := $(APP_NAME)
IMAGE_TAG := $(VERSION)
DOCKER_IMAGE := $(IMAGE_NAME):$(IMAGE_TAG)
BUILD_DIR := build

# Go settings
GO := go
CGO_ENABLED := 0

# Docker settings
DOCKER := docker
DOCKER_COMPOSE := docker compose

# Colors for output
COLOR_RESET := \033[0m
COLOR_GREEN := \033[32m
COLOR_YELLOW := \033[33m
COLOR_BLUE := \033[34m

# ─── Phony Targets ────────────────────────────────────────────────────────────
.PHONY: all build install build-linux clean test test-race vet fmt-check quality check agent-dispatcher tui-test tui-build tui-run docker-build docker-build-cache docker-run docker-run-detached docker-stop docker-logs docker-restart docker-clean compose-up compose-up-llm compose-down compose-logs compose-restart compose-clean dev dev-watch version help
.PHONY: build-agents build-agents-linux

# ─── Default Target ────────────────────────────────────────────────────────────
all: help

# ─── Help ──────────────────────────────────────────────────────────────────────
help: ## Show this help message
	@echo "$(COLOR_BLUE)Mini-Orca $(VERSION) - Makefile Targets$(COLOR_RESET)"
	@echo ""
	@echo "$(COLOR_GREEN)Build Targets:$(COLOR_RESET)"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  $(COLOR_BLUE)%-20s$(COLOR_RESET) %s\n", $$1, $$2}'
	@echo ""

# ─── Local Build ───────────────────────────────────────────────────────────────
build: build-agents ## Build daemon, TUI, evaluation and agent command binaries locally
	@echo "$(COLOR_GREEN)Building $(APP_NAME) $(VERSION)...$(COLOR_RESET)"
	@mkdir -p $(BUILD_DIR)
	@CGO_ENABLED=$(CGO_ENABLED) $(GO) build -ldflags="-w -s -X github.com/nanaki-93/mini-orca/v2/internal/version.Version=$(VERSION)" -o $(BUILD_DIR)/$(APP_NAME)-daemon ./cmd/daemon
	@CGO_ENABLED=$(CGO_ENABLED) $(GO) build -ldflags="-w -s -X github.com/nanaki-93/mini-orca/v2/internal/version.Version=$(VERSION)" -o $(BUILD_DIR)/$(APP_NAME)-tui ./cmd/tui
	@cp -f $(BUILD_DIR)/$(APP_NAME)-tui ./$(APP_NAME)
	@cp -f $(BUILD_DIR)/$(APP_NAME)-daemon ./$(APP_NAME)-daemon
	@echo "$(COLOR_GREEN)Build complete: ./$(APP_NAME), ./$(APP_NAME)-daemon ($(BUILD_DIR)/)$(COLOR_RESET)"

install: build ## Build and substitute mini-orca into ~/DEV/lab/mini-orca (and MINI_ORCA_HOME if present)
	@target_dir="$$HOME/DEV/lab/mini-orca"; \
	if [ -d "$$target_dir" ] && [ "$$(cd "$$target_dir" && pwd)" != "$$(pwd)" ]; then \
		cp -f $(BUILD_DIR)/$(APP_NAME)-tui "$$target_dir/$(APP_NAME)"; \
		cp -f $(BUILD_DIR)/$(APP_NAME)-daemon "$$target_dir/$(APP_NAME)-daemon"; \
	fi
	@if [ -n "$${MINI_ORCA_HOME:-}" ] && [ -d "$${MINI_ORCA_HOME}" ]; then \
		cp -f $(BUILD_DIR)/$(APP_NAME)-tui "$${MINI_ORCA_HOME}/$(APP_NAME)"; \
		cp -f $(BUILD_DIR)/$(APP_NAME)-daemon "$${MINI_ORCA_HOME}/$(APP_NAME)-daemon"; \
		echo "$(COLOR_GREEN)Installed $(APP_NAME) into $${MINI_ORCA_HOME}$(COLOR_RESET)"; \
	elif [ -d "$$HOME/DEV/programs/mini-orca" ]; then \
		cp -f $(BUILD_DIR)/$(APP_NAME)-tui "$$HOME/DEV/programs/mini-orca/$(APP_NAME)"; \
		cp -f $(BUILD_DIR)/$(APP_NAME)-daemon "$$HOME/DEV/programs/mini-orca/$(APP_NAME)-daemon"; \
		echo "$(COLOR_GREEN)Installed $(APP_NAME) into $$HOME/DEV/programs/mini-orca$(COLOR_RESET)"; \
	fi
	@echo "$(COLOR_GREEN)Updated $(APP_NAME) in $$HOME/DEV/lab/mini-orca/$(APP_NAME)$(COLOR_RESET)"

build-linux: build-agents-linux ## Build all Linux amd64 release command binaries
	@mkdir -p $(BUILD_DIR)
	@CGO_ENABLED=$(CGO_ENABLED) GOOS=linux GOARCH=amd64 $(GO) build -ldflags="-w -s -X github.com/nanaki-93/mini-orca/v2/internal/version.Version=$(VERSION)" -o $(BUILD_DIR)/$(APP_NAME)-daemon-linux-amd64 ./cmd/daemon
	@CGO_ENABLED=$(CGO_ENABLED) GOOS=linux GOARCH=amd64 $(GO) build -ldflags="-w -s -X github.com/nanaki-93/mini-orca/v2/internal/version.Version=$(VERSION)" -o $(BUILD_DIR)/$(APP_NAME)-tui-linux-amd64 ./cmd/tui
	@echo "$(COLOR_GREEN)Linux build complete: $(BUILD_DIR)/$(APP_NAME)-daemon-linux-amd64, $(BUILD_DIR)/$(APP_NAME)-tui-linux-amd64$(COLOR_RESET)"

build-agents: ## Build evaluation, worker and inspector commands without CGO
	@mkdir -p $(BUILD_DIR)
	@for agent_command in engineering-insight-eval agent-eval agent-worker agent-inspect; do \
		CGO_ENABLED=0 $(GO) build -o $(BUILD_DIR)/$(APP_NAME)-$$agent_command ./cmd/$$agent_command || exit $$?; \
	done

build-agents-linux: ## Cross-build evaluation, worker and inspector for Linux amd64 without CGO
	@mkdir -p $(BUILD_DIR)
	@for agent_command in engineering-insight-eval agent-eval agent-worker agent-inspect; do \
		CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build -o $(BUILD_DIR)/$(APP_NAME)-$$agent_command-linux-amd64 ./cmd/$$agent_command || exit $$?; \
	done

clean: ## Remove build artifacts
	@echo "$(COLOR_YELLOW)Cleaning build artifacts...$(COLOR_RESET)"
	@rm -rf $(BUILD_DIR)
	@echo "$(COLOR_YELLOW)Clean complete$(COLOR_RESET)"

# ─── Testing ───────────────────────────────────────────────────────────────────
test: ## Run Go tests and write the coverage report
	@echo "$(COLOR_GREEN)Running tests...$(COLOR_RESET)"
	@mkdir -p $(BUILD_DIR)
	@$(GO) test ./... -v -coverprofile=$(BUILD_DIR)/coverage.out
	@$(GO) tool cover -html=$(BUILD_DIR)/coverage.out -o $(BUILD_DIR)/coverage.html
	@echo "$(COLOR_GREEN)Tests complete. Coverage report: $(BUILD_DIR)/coverage.html$(COLOR_RESET)"

test-race: ## Run all Go tests under the race detector
	@$(GO) test -race -timeout=15m ./...

vet: ## Vet all Go packages
	@$(GO) vet ./...

agent-dispatcher: ## Run the local dispatcher tests without paid agent calls
	@python3 -m unittest discover -s scripts/tests -p 'test_*.py'

fmt-check: ## Verify Go formatting without modifying files
	@files="$$(gofmt -l $$(find . -path '*/testdata/*' -prune -o -name '*.go' -type f -not -path './build/*' -print))"; test -z "$$files" || { echo "Run go fmt ./...:"; echo "$$files"; exit 1; }

quality: ## Run Go static, reachability, complexity and clone checks
	@./scripts/quality.sh

tui-test: ## Run Go TUI workflow, state and renderer tests
	@$(GO) test -race ./internal/tui/... ./cmd/tui/...

tui-build: ## Build the Go TUI binary
	@mkdir -p $(BUILD_DIR)
	@CGO_ENABLED=$(CGO_ENABLED) $(GO) build -ldflags="-w -s -X github.com/nanaki-93/mini-orca/v2/internal/version.Version=$(VERSION)" -o $(BUILD_DIR)/$(APP_NAME)-tui ./cmd/tui

tui-run: ## Launch the Go TUI client
	@$(GO) run ./cmd/tui

check: fmt-check test test-race vet agent-dispatcher tui-test ## Run the complete supported validation path

# ─── Docker Build ──────────────────────────────────────────────────────────────
docker-build: ## Build Docker image
	@echo "$(COLOR_GREEN)Building Docker image $(DOCKER_IMAGE)...$(COLOR_RESET)"
	@$(DOCKER) build \
		--tag $(DOCKER_IMAGE) \
		.
	@echo "$(COLOR_GREEN)Docker image built: $(DOCKER_IMAGE)$(COLOR_RESET)"

docker-build-cache: ## Build Docker image with cache optimization
	@echo "$(COLOR_GREEN)Building Docker image with cache...$(COLOR_RESET)"
	@$(DOCKER) build \
		--cache-from $(IMAGE_NAME):cache \
		--cache-to $(IMAGE_NAME):cache \
		--tag $(DOCKER_IMAGE) \
		.
	@echo "$(COLOR_GREEN)Docker image built with cache: $(DOCKER_IMAGE)$(COLOR_RESET)"

# ─── Docker Run ────────────────────────────────────────────────────────────────
docker-run: ## Run container in detached mode
	@echo "$(COLOR_GREEN)Starting container...$(COLOR_RESET)"
	@$(DOCKER) run -d \
		--name $(APP_NAME) \
		--restart unless-stopped \
		-p 127.0.0.1:9090:9090 \
		-v $$(pwd)/config.yaml:/app/config.yaml:ro \
		-v $$(pwd)/projects:/app/projects:rw \
		-v $$(pwd)/logs:/app/logs:rw \
		-e MINI_ORCA_CONFIG=/app/config.yaml \
		-e MINI_ORCA_BIND_ADDRESS=0.0.0.0:9090 \
		$(DOCKER_IMAGE)
	@echo "$(COLOR_GREEN)Container started: $(APP_NAME)$(COLOR_RESET)"
	@echo "$(COLOR_BLUE)Daemon API available to the TUI client at: http://localhost:9090$(COLOR_RESET)"

docker-run-detached: docker-run ## Run container in detached mode (alias)

docker-stop: ## Stop and remove container
	@echo "$(COLOR_YELLOW)Stopping container...$(COLOR_RESET)"
	@$(DOCKER) stop $(APP_NAME) 2>/dev/null || true
	@$(DOCKER) rm $(APP_NAME) 2>/dev/null || true
	@echo "$(COLOR_YELLOW)Container stopped$(COLOR_RESET)"

docker-logs: ## Show container logs
	@$(DOCKER) logs -f $(APP_NAME)

docker-restart: docker-stop docker-run ## Restart container

# ─── Docker Compose ───────────────────────────────────────────────────────────
compose-up: ## Start services with docker-compose
	@echo "$(COLOR_GREEN)Starting services with docker-compose...$(COLOR_RESET)"
	@$(DOCKER_COMPOSE) up -d --build
	@echo "$(COLOR_GREEN)Services started$(COLOR_RESET)"
	@echo "$(COLOR_BLUE)Daemon API available to the TUI client at: http://localhost:9090$(COLOR_RESET)"

compose-up-llm: ## Start services with docker-compose including LM Studio
	@echo "$(COLOR_GREEN)Starting services with docker-compose (with LM Studio)...$(COLOR_RESET)"
	@$(DOCKER_COMPOSE) --profile with-llm up -d --build
	@echo "$(COLOR_GREEN)Services started (with LM Studio)$(COLOR_RESET)"
	@echo "$(COLOR_BLUE)Daemon API available to the TUI client at: http://localhost:9090$(COLOR_RESET)"

compose-down: ## Stop services with docker-compose
	@echo "$(COLOR_YELLOW)Stopping services...$(COLOR_RESET)"
	@$(DOCKER_COMPOSE) down
	@echo "$(COLOR_YELLOW)Services stopped$(COLOR_RESET)"

compose-logs: ## Show docker-compose logs
	@$(DOCKER_COMPOSE) logs -f

compose-restart: compose-down compose-up ## Restart services

# ─── Docker Clean ──────────────────────────────────────────────────────────────
docker-clean: ## Remove Docker images and containers
	@echo "$(COLOR_YELLOW)Cleaning Docker resources...$(COLOR_RESET)"
	@$(DOCKER) stop $(APP_NAME) 2>/dev/null || true
	@$(DOCKER) rm $(APP_NAME) 2>/dev/null || true
	@$(DOCKER) rmi $(DOCKER_IMAGE) 2>/dev/null || true
	@$(DOCKER) image prune -f
	@echo "$(COLOR_YELLOW)Docker cleanup complete$(COLOR_RESET)"

compose-clean: ## Remove docker-compose resources
	@echo "$(COLOR_YELLOW)Cleaning docker-compose resources...$(COLOR_RESET)"
	@$(DOCKER_COMPOSE) down -v --rmi all --remove-orphans
	@echo "$(COLOR_YELLOW)Docker-compose cleanup complete$(COLOR_RESET)"

# ─── Development ───────────────────────────────────────────────────────────────
dev: ## Run locally for development
	@echo "$(COLOR_GREEN)Running in development mode...$(COLOR_RESET)"
	@$(GO) run ./cmd/daemon

dev-watch: ## Run with air for hot reload (if installed)
	@which air > /dev/null 2>&1 || { echo "Installing air..."; go install github.com/air-verse/air@latest; }
	@air

# ─── Version Info ──────────────────────────────────────────────────────────────
version: ## Show version information
	@echo "$(COLOR_BLUE)Mini-Orca Version: $(VERSION)$(COLOR_RESET)"
	@echo "$(COLOR_BLUE)Go Version: $$(go version)$(COLOR_RESET)"
	@echo "$(COLOR_BLUE)Docker Version: $$(docker --version 2>/dev/null || echo 'not installed')$(COLOR_RESET)"
	@echo "$(COLOR_BLUE)Docker Compose: $$(docker compose version 2>/dev/null || echo 'not installed')$(COLOR_RESET)"
