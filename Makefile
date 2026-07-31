# Mini-Orca Makefile
# Version: 2.0.0

# Variables
APP_NAME := mini-orca
VERSION := 2.0.0
IMAGE_NAME := $(APP_NAME)
IMAGE_TAG := $(VERSION)
DOCKER_IMAGE := $(IMAGE_NAME):$(IMAGE_TAG)
BUILD_DIR := build

# Go settings
GO := go
GOOS := linux
GOARCH := amd64
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
.PHONY: all build clean test run docker-build docker-run docker-stop docker-logs docker-clean help

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
build: ## Build the application locally
	@echo "$(COLOR_GREEN)Building $(APP_NAME) $(VERSION)...$(COLOR_RESET)"
	@mkdir -p $(BUILD_DIR)
	@CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) GOARCH=$(GOARCH) \
		$(GO) build -ldflags="-w -s" -o $(BUILD_DIR)/$(APP_NAME)-daemon ./cmd/daemon
	@echo "$(COLOR_GREEN)Build complete: $(BUILD_DIR)/$(APP_NAME)-daemon$(COLOR_RESET)"

clean: ## Remove build artifacts
	@echo "$(COLOR_YELLOW)Cleaning build artifacts...$(COLOR_RESET)"
	@rm -rf $(BUILD_DIR)
	@echo "$(COLOR_YELLOW)Clean complete$(COLOR_RESET)"

# ─── Testing ───────────────────────────────────────────────────────────────────
test: ## Run tests
	@echo "$(COLOR_GREEN)Running tests...$(COLOR_RESET)"
	@$(GO) test ./... -v -coverprofile=$(BUILD_DIR)/coverage.out
	@$(GO) tool cover -html=$(BUILD_DIR)/coverage.out -o $(BUILD_DIR)/coverage.html
	@echo "$(COLOR_GREEN)Tests complete. Coverage report: $(BUILD_DIR)/coverage.html$(COLOR_RESET)"

# ─── Docker Build ──────────────────────────────────────────────────────────────
docker-build: ## Build Docker image
	@echo "$(COLOR_GREEN)Building Docker image $(DOCKER_IMAGE)...$(COLOR_RESET)"
	@$(DOCKER) build \
		--build-arg BUILD_DATE=$$(date -u +"%Y-%m-%dT%H:%M:%SZ") \
		--build-arg VCS_REF=$$(git rev-parse --short HEAD) \
		--tag $(DOCKER_IMAGE) \
		.
	@echo "$(COLOR_GREEN)Docker image built: $(DOCKER_IMAGE)$(COLOR_RESET)"

docker-build-cache: ## Build Docker image with cache optimization
	@echo "$(COLOR_GREEN)Building Docker image with cache...$(COLOR_RESET)"
	@$(DOCKER) build \
		--cache-from $(IMAGE_NAME):cache \
		--cache-to $(IMAGE_NAME):cache \
		--build-arg BUILD_DATE=$$(date -u +"%Y-%m-%dT%H:%M:%SZ") \
		--build-arg VCS_REF=$$(git rev-parse --short HEAD) \
		--tag $(DOCKER_IMAGE) \
		.
	@echo "$(COLOR_GREEN)Docker image built with cache: $(DOCKER_IMAGE)$(COLOR_RESET)"

# ─── Docker Run ────────────────────────────────────────────────────────────────
docker-run: ## Run container in detached mode
	@echo "$(COLOR_GREEN)Starting container...$(COLOR_RESET)"
	@$(DOCKER) run -d \
		--name $(APP_NAME) \
		--restart unless-stopped \
		-p 8080:8080 \
		-v $$(pwd)/config.yaml:/app/config.yaml:ro \
		-v $$(pwd)/projects:/app/projects:rw \
		-v $$(pwd)/logs:/app/logs:rw \
		-e MINI_ORCA_CONFIG=/app/config.yaml \
		$(DOCKER_IMAGE)
	@echo "$(COLOR_GREEN)Container started: $(APP_NAME)$(COLOR_RESET)"
	@echo "$(COLOR_BLUE)Access the IDE at: http://localhost:8080$(COLOR_RESET)"

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
	@echo "$(COLOR_BLUE)Access the IDE at: http://localhost:8080$(COLOR_RESET)"

compose-up-llm: ## Start services with docker-compose including LM Studio
	@echo "$(COLOR_GREEN)Starting services with docker-compose (with LM Studio)...$(COLOR_RESET)"
	@$(DOCKER_COMPOSE) --profile with-llm up -d --build
	@echo "$(COLOR_GREEN)Services started (with LM Studio)$(COLOR_RESET)"
	@echo "$(COLOR_BLUE)Access the IDE at: http://localhost:8080$(COLOR_RESET)"

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
