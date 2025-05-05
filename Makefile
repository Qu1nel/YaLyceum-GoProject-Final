# Colors (might not work on macOS)
RED=\033[0;31m
GREEN=\033[1;32m
YELLOW=\033[1;33m
BLUE=\033[0;36m
RESET=\033[0m

# Utilities
DOCKER_COMPOSE := docker-compose
DOCKER := docker

# Main commands
##############################################################################

.PHONY: help up down start stop build logs logs-agent logs-postgres test clean test-coverage check-docker

.DEFAULT_GOAL := help

up: check-docker ## Build and start all services in detached mode (-d)
	@echo -e "$(BLUE)Starting services (building if necessary)...$(RESET)"
	@$(DOCKER_COMPOSE) up --build -d
	@echo -e "$(GREEN)Services started!$(RESET)"
	@echo -e "$(YELLOW)Use 'make logs' or 'make logs-<service>' to view logs.$(RESET)"
	@echo -e "$(YELLOW)Use 'make down' to stop and remove containers.$(RESET)"

ps: check-docker ## Show the status of running Docker Compose containers
	@echo -e "$(BLUE)Container status:$(RESET)"
	@$(DOCKER_COMPOSE) ps

down: check-docker ## Stop and remove containers, networks, and volumes (DB data will be lost!)
	@echo -e "$(YELLOW)Stopping and removing containers, networks, and volumes...$(RESET)"
	@$(DOCKER_COMPOSE) down -v --remove-orphans
	@echo -e "$(RED)Everything stopped and removed.$(RESET)"

start: check-docker ## Start previously built containers
	@echo -e "$(BLUE)Starting existing containers...$(RESET)"
	@$(DOCKER_COMPOSE) start
	@echo -e "$(GREEN)Containers started.$(RESET)"

stop: check-docker ## Stop running containers (without removing)
	@echo -e "$(YELLOW)Stopping containers...$(RESET)"
	@$(DOCKER_COMPOSE) stop
	@echo -e "$(RED)Containers stopped.$(RESET)"

build: check-docker ## Build or rebuild service images
	@echo -e "$(YELLOW)Building Docker images...$(RESET)"
	@$(DOCKER_COMPOSE) build
	@echo -e "$(GREEN)Build complete.$(RESET)"

logs: check-docker ## Show logs for all running services (follow)
	@echo -e "$(YELLOW)Following logs (press Ctrl+C to exit)...$(RESET)"
	@$(DOCKER_COMPOSE) logs -f

logs-agent: check-docker ## Show logs for the 'agent' service (follow)
	@echo -e "$(YELLOW)Following logs for 'agent' (press Ctrl+C to exit)...$(RESET)"
	@$(DOCKER_COMPOSE) logs -f agent

logs-postgres: check-docker ## Show logs for the 'postgres' service (follow)
	@echo -e "$(YELLOW)Following logs for 'postgres' (press Ctrl+C to exit)...$(RESET)"
	@$(DOCKER_COMPOSE) logs -f postgres

# logs-orchestrator: ... (add when the service appears)
# logs-worker: ... (add when the service appears)

test: check-docker ## Run Go unit tests for all packages
	@echo -e "$(YELLOW)Running unit tests...$(RESET)"
	@$(DOCKER) exec -it calculator_agent sh -c "go test -v ./..." || echo -e "$(RED)Tests failed or the 'agent' service is not running.$(RESET)" # Run inside the agent container
	# @go test -v ./... # Alternative: run locally if Go is installed
	@echo -e "$(GREEN)Test execution finished.$(RESET)"

test-coverage: check-docker ## Run tests with coverage report (generated inside container)
	@echo -e "$(YELLOW)Running tests with coverage analysis...$(RESET)"
	@$(DOCKER) exec -it calculator_agent sh -c "go test -v -coverpkg=./... -coverprofile=coverage.out ./... && go tool cover -html=coverage.out -o coverage.html" || echo -e "$(RED)Failed to run tests or generate coverage report.$(RESET)"
	@echo -e "$(GREEN)Coverage report 'coverage.html' generated inside the agent container.$(RESET)"
	@echo -e "$(YELLOW)To view it, copy it using: 'docker cp calculator_agent:/app/coverage.html .' and open it in your browser.$(RESET)"
	# @go test -v -coverpkg=./... -coverprofile=coverage.out ./... # Run locally
	# @go tool cover -html=coverage.out # Open report locally

clean: check-docker ## Clean the Go build cache
	@echo -e "$(YELLOW)Cleaning Go build cache...$(RESET)"
	@go clean -cache
	# Optional: remove binaries: rm -f agent orchestrator worker
	@echo -e "$(GREEN)Cache cleaned.$(RESET)"


# Helper commands
##############################################################################

.PHONY: check-docker

check-docker: ## Check if docker-compose and Docker daemon are available
	@which $(DOCKER_COMPOSE) > /dev/null || (echo -e "$(RED)Error: '$(DOCKER_COMPOSE)' not found! Please install Docker Compose.$(RESET)" && exit 1)
	@$(DOCKER) info > /dev/null 2>&1 || (echo -e "$(RED)Error: Docker daemon not running or unavailable! Please start Docker.$(RESET)" && exit 1)


# Help command
##############################################################################

help: ## Show this help message
	@echo -e "\n$(BLUE)Available targets for "make":$(RESET)"
	@echo "--------------------------------"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  $(GREEN)%-20s$(RESET) %s\n", $$1, $$2}' $(MAKEFILE_LIST)
	@echo -e "\n$(BLUE)Usage example:$(RESET)"
	@echo "  make up       # Build and run all services"
	@echo "  make test     # Run unit tests"
	@echo "  make logs     # Follow logs"
	@echo "  make down     # Stop and remove all (including DB data)"

# Debugging commands (from your example)
##############################################################################
print-%: ## Print the value of any Makefile variable (e.g., make print-DOCKER_COMPOSE)
	@echo -e "$(BLUE)$*$(RESET) = $($*)"

info-%: ## Show the commands that would be executed for a target (dry run)
	@$(MAKE) --dry-run --always-make $* | grep -v "info-"