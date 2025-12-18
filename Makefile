.PHONY: help up down restart logs app-refresh build clean db-migrate db-reset db-shell test fmt vet

# Colors for terminal output
CYAN := \033[0;36m
GREEN := \033[0;32m
YELLOW := \033[1;33m
NC := \033[0m # No Color

help: ## Show this help message
	@echo "$(CYAN)CaspianEx OTC Exchange - Available Commands:$(NC)"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  $(GREEN)%-20s$(NC) %s\n", $$1, $$2}'
	@echo ""

# Docker Compose Commands
up: ## Start all services (postgres, redis, migrate, app)
	@echo "$(CYAN)Starting all services...$(NC)"
	docker-compose up -d
	@echo "$(GREEN)✓ All services started$(NC)"
	@echo "Run 'make logs' to view logs"

down: ## Stop all services
	@echo "$(CYAN)Stopping all services...$(NC)"
	docker-compose down
	@echo "$(GREEN)✓ All services stopped$(NC)"

restart: down up ## Restart all services

logs: ## Follow logs for all services
	docker-compose logs -f

logs-app: ## Follow logs for app only
	docker-compose logs -f app

logs-db: ## Follow logs for postgres only
	docker-compose logs -f postgres

# App Development Commands
app-refresh: ## Rebuild and restart ONLY the app (fast refresh after code changes)
	@echo "$(CYAN)Rebuilding and restarting app...$(NC)"
	docker-compose up -d --no-deps --build app
	@echo "$(GREEN)✓ App refreshed successfully$(NC)"

app-shell: ## Open shell in running app container
	docker-compose exec app sh

# Local Build Commands
build: ## Build the Go binary locally
	@echo "$(CYAN)Building Go binary...$(NC)"
	go build -o bin/server ./cmd/server
	@echo "$(GREEN)✓ Build complete: bin/server$(NC)"

run: ## Run the app locally (without Docker)
	@echo "$(CYAN)Running app locally...$(NC)"
	go run ./cmd/server

# Database Commands
db-migrate: ## Run database migrations
	@echo "$(CYAN)Running migrations...$(NC)"
	docker-compose up migrate
	@echo "$(GREEN)✓ Migrations complete$(NC)"

db-reset: ## Reset database (WARNING: deletes all data)
	@echo "$(YELLOW)⚠️  This will delete ALL data. Press Ctrl+C to cancel...$(NC)"
	@sleep 3
	docker-compose down -v
	docker-compose up -d postgres
	@echo "Waiting for postgres to be ready..."
	@sleep 5
	docker-compose up migrate
	@echo "$(GREEN)✓ Database reset complete$(NC)"

db-shell: ## Open PostgreSQL shell
	docker-compose exec postgres psql -U exchange -d exchange_db

# Code Quality Commands
fmt: ## Format Go code
	@echo "$(CYAN)Formatting code...$(NC)"
	go fmt ./...
	@echo "$(GREEN)✓ Code formatted$(NC)"

vet: ## Run go vet
	@echo "$(CYAN)Running go vet...$(NC)"
	go vet ./...
	@echo "$(GREEN)✓ go vet passed$(NC)"

lint: fmt vet ## Run all linters

test: ## Run tests
	@echo "$(CYAN)Running tests...$(NC)"
	go test -v ./...

# Cleanup Commands
clean: ## Remove built binaries and Docker volumes
	@echo "$(CYAN)Cleaning up...$(NC)"
	rm -rf bin/
	docker-compose down -v
	@echo "$(GREEN)✓ Cleanup complete$(NC)"

clean-cache: ## Clear Go build cache
	go clean -cache -modcache -testcache

# Development Workflow Commands
dev: ## Start development environment (up + logs)
	@make up
	@make logs-app

refresh: app-refresh ## Alias for app-refresh

rebuild: ## Full rebuild (down, clean, up)
	@make down
	@make clean
	@make up

# Docker Commands
docker-prune: ## Clean up Docker (removes unused images, containers, volumes)
	@echo "$(YELLOW)⚠️  This will remove unused Docker resources. Press Ctrl+C to cancel...$(NC)"
	@sleep 3
	docker system prune -af --volumes
	@echo "$(GREEN)✓ Docker cleanup complete$(NC)"

# Environment Commands
env-check: ## Check if .env file exists
	@if [ -f .env ]; then \
		echo "$(GREEN)✓ .env file exists$(NC)"; \
	else \
		echo "$(YELLOW)⚠️  .env file not found. Creating from .env.example...$(NC)"; \
		cp .env.example .env; \
		echo "$(GREEN)✓ .env file created. Please update with your values.$(NC)"; \
	fi

# Status Commands
status: ## Show status of all services
	@docker-compose ps

ps: status ## Alias for status

# Installation Commands
install: ## Install dependencies and setup project
	@echo "$(CYAN)Installing dependencies...$(NC)"
	go mod download
	@make env-check
	@echo "$(GREEN)✓ Dependencies installed$(NC)"
	@echo ""
	@echo "$(YELLOW)Next steps:$(NC)"
	@echo "  1. Update .env with your configuration"
	@echo "  2. Run 'make up' to start services"
	@echo "  3. Run 'make logs' to view logs"

# Default target
.DEFAULT_GOAL := help
