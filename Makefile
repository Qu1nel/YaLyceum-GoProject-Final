RED=\033[0;31m
GREEN=\033[1;32m
YELLOW=\033[1;33m
BLUE=\033[0;36m
RESET=\033[0m

GO_CMD := go
GO_TEST_FLAGS := -v -count=1
GO_COVER_FLAGS := -cover -coverprofile=coverage.out
GO_COVER_HTML_FLAGS := -html=coverage.out -o coverage.html

DEFAULT_UNIT_TEST_PACKAGES := ./internal/agent/service ./internal/orchestrator/grpc_handler ./internal/orchestrator/repository ./internal/orchestrator/service ./internal/worker/grpc_handler ./internal/worker/service
UNAME_S := $(shell uname -s 2>/dev/null || echo Unknown)

ifeq ($(OS),Windows_NT)
    UNIT_TEST_PACKAGES := $(DEFAULT_UNIT_TEST_PACKAGES)
else ifeq ($(UNAME_S),Linux)
    UNIT_TEST_PACKAGES_CMD := find ./internal -name '*_test.go' -print0 | xargs -0 -n1 dirname | sort -u | sed 's|^\./|./|'
    UNIT_TEST_PACKAGES := $(shell $(UNIT_TEST_PACKAGES_CMD))
else ifeq ($(UNAME_S),Darwin)
    UNIT_TEST_PACKAGES_CMD := find ./internal -name '*_test.go' -print0 | xargs -0 -n1 dirname | sort -u | sed 's|^\./|./|'
    UNIT_TEST_PACKAGES := $(shell $(UNIT_TEST_PACKAGES_CMD))
else
    ifneq ($(shell which find 2>/dev/null),)
        UNIT_TEST_PACKAGES_CMD := find ./internal -name '*_test.go' -print0 | xargs -0 -n1 dirname | sort -u | sed 's|^\./|./|'
        UNIT_TEST_PACKAGES := $(shell $(UNIT_TEST_PACKAGES_CMD))
    else
        UNIT_TEST_PACKAGES_GREP_CMD := $(GO_CMD) list ./internal/... | grep -v '/mocks' || echo "$(DEFAULT_UNIT_TEST_PACKAGES)"
        UNIT_TEST_PACKAGES := $(shell $(UNIT_TEST_PACKAGES_GREP_CMD))
    endif
endif

ifeq ($(strip $(UNIT_TEST_PACKAGES)),)
    UNIT_TEST_PACKAGES := $(DEFAULT_UNIT_TEST_PACKAGES)
endif


DEFAULT_FRONTEND_PORT := 80
DEFAULT_AGENT_HTTP_PORT := 8080

get_env_port = $(shell test -f .env && grep -E "^$(1)=" .env | grep -v '^\s*#' | cut -d'=' -f2- | cut -d'#' -f1 | tr -d '[:space:]' || echo "$(2)")

FRONTEND_PORT := $(call get_env_port,FRONTEND_PORT,$(DEFAULT_FRONTEND_PORT))
AGENT_HTTP_PORT := $(call get_env_port,AGENT_HTTP_PORT,$(DEFAULT_AGENT_HTTP_PORT))


DOCKER_COMPOSE := docker-compose
DOCKER := docker

.DEFAULT_GOAL := help


# Main commands
##############################################################################


.PHONY: help up ps ps-a down start stop build build-% logs logs-% \
        test test-unit test-integration test-coverage test-coverage-html \
        clean check-docker

up: check-docker ## Build and start all services in detached mode (-d)
	@echo -e "$(BLUE)Starting services (building if necessary)...$(RESET)"
	@$(DOCKER_COMPOSE) up --build -d
	@echo -e "$(GREEN)Services started successfully!$(RESET)"
	@echo -e "$(YELLOW)Access the application:$(RESET)"
	@echo -e "  Frontend:           $(BLUE)http://localhost:$(FRONTEND_PORT)$(RESET)"
	@echo -e "  Agent API (Swagger):$(BLUE)http://localhost:$(AGENT_HTTP_PORT)/swagger/$(RESET)"
	@echo -e "$(YELLOW)Use 'make logs' or 'make logs-<service_name>' to view logs.$(RESET)"
	@echo -e "$(YELLOW)Use 'make down' to stop and remove everything (incl. data).$(RESET)"
	@echo -e "$(YELLOW)Use 'make stop' to just stop containers.$(RESET)"

ps: check-docker ## Show the status of running Docker Compose containers
	@echo -e "$(BLUE)Current container status:$(RESET)"
	@$(DOCKER_COMPOSE) ps

ps-a: check-docker ## Show the status of all Docker Compose containers (including stopped)
	@echo -e "$(BLUE)All container status (including stopped):$(RESET)"
	@$(DOCKER_COMPOSE) ps -a

down: check-docker ## Stop and remove containers, networks, and volumes (DB data will be lost!)
	@echo -e "$(YELLOW)Stopping and removing containers, networks, and volumes...$(RESET)"
	@$(DOCKER_COMPOSE) down -v --remove-orphans
	@echo -e "$(RED)All services stopped and data removed.$(RESET)"

start: check-docker ## Start previously built containers
	@echo -e "$(BLUE)Starting existing containers...$(RESET)"
	@$(DOCKER_COMPOSE) start
	@echo -e "$(GREEN)Containers started successfully!$(RESET)"
	@echo -e "$(YELLOW)Access the application:$(RESET)"
	@echo -e "  Frontend:           $(BLUE)http://localhost:$(FRONTEND_PORT)$(RESET)"
	@echo -e "  Agent API (Swagger):$(BLUE)http://localhost:$(AGENT_HTTP_PORT)/swagger/$(RESET)"
	@echo -e "$(YELLOW)Use 'make logs' or 'make logs-<service_name>' to view logs.$(RESET)"
	@echo -e "$(YELLOW)Use 'make down' to stop and remove everything (incl. data).$(RESET)"
	@echo -e "$(YELLOW)Use 'make stop' to just stop containers.$(RESET)"

stop: check-docker ## Stop running containers (without removing them or their data)
	@echo -e "$(YELLOW)Stopping containers...$(RESET)"
	@$(DOCKER_COMPOSE) stop
	@echo -e "$(RED)Containers stopped.$(RESET)"

build: check-docker ## Build or rebuild service images
	@echo -e "$(YELLOW)Building Docker images for all services...$(RESET)"
	@$(DOCKER_COMPOSE) build
	@echo -e "$(GREEN)Build complete.$(RESET)"

build-%: check-docker ## Build a specific service (e.g., make build-agent)
	@echo -e "$(YELLOW)Building Docker image for '$*'...$(RESET)"
	@$(DOCKER_COMPOSE) build $*
	@echo -e "$(GREEN)Build complete for '$*'.$(RESET)"

logs: check-docker ## Show logs for all running services (follow)
	@echo -e "$(YELLOW)Following logs for all services (press Ctrl+C to exit)...$(RESET)"
	@$(DOCKER_COMPOSE) logs -f

logs-%: check-docker ## Show logs for a specific service (e.g., make logs-agent)
	@echo -e "$(YELLOW)Following logs for '$*' (press Ctrl+C to exit)...$(RESET)"
	@$(DOCKER_COMPOSE) logs -f $*


# Testing commands
##############################################################################


test: test-unit test-integration ## Run all tests (unit and integration)
	@echo -e "$(GREEN)All tests completed!$(RESET)" # Changed message slightly

test-unit: clean ## Run Go unit tests for relevant packages
	@echo -e "$(BLUE)Running unit tests...$(RESET)"
	@echo "Packages to test: $(UNIT_TEST_PACKAGES)"
	@if [ -z "$(UNIT_TEST_PACKAGES)" ]; then \
		echo -e "$(RED)No unit test packages found or determined. Skipping unit tests.$(RESET)"; \
	else \
		$(GO_CMD) test $(GO_TEST_FLAGS) $(UNIT_TEST_PACKAGES); \
	fi
	@echo -e "$(GREEN)Unit tests process finished.$(RESET)" # Changed message slightly

test-integration: clean check-docker ## Run Go integration tests (requires Docker)
	@echo -e "$(BLUE)Running integration tests...$(RESET)"
	@$(GO_CMD) test $(GO_TEST_FLAGS) -a ./tests/integration/...
	@echo -e "$(GREEN)Integration tests completed.$(RESET)"

test-coverage: clean ## Run unit tests with coverage and generate coverage.out
	@echo -e "$(BLUE)Running unit tests with coverage...$(RESET)"
	@echo "Packages to test for coverage: $(UNIT_TEST_PACKAGES)"
	@if [ -z "$(UNIT_TEST_PACKAGES)" ]; then \
		echo -e "$(RED)No unit test packages found or determined. Skipping coverage generation.$(RESET)"; \
	else \
		$(GO_CMD) test $(GO_TEST_FLAGS) $(GO_COVER_FLAGS) $(UNIT_TEST_PACKAGES); \
	fi
	@echo -e "$(GREEN)Unit tests with coverage completed. Profile: coverage.out$(RESET)"

test-coverage-html: test-coverage ## Generate HTML coverage report from coverage.out
	@echo -e "$(BLUE)Generating HTML coverage report...$(RESET)"
	@if [ ! -f coverage.out ]; then \
		echo -e "$(YELLOW)coverage.out not found. Run 'make test-coverage' first.$(RESET)"; \
	else \
		$(GO_CMD) tool cover $(GO_COVER_HTML_FLAGS); \
		echo -e "$(GREEN)HTML coverage report generated: coverage.html$(RESET)"; \
		echo -e "$(YELLOW)Open '$(GREEN)coverage.html$(YELLOW)' in your browser to view the report.$(RESET)"; \
	fi


# Clean commands
##############################################################################


clean: ## Clean Go build cache and test cache, remove coverage files
	@echo -e "$(YELLOW)Cleaning Go build cache, test cache, and coverage files...$(RESET)"
	@$(GO_CMD) clean -cache
	@$(GO_CMD) clean -testcache
	@rm -f coverage.out coverage.html
	@echo -e "$(GREEN)Cache and coverage files cleaned.$(RESET)"


# Helper commands
##############################################################################


.PHONY: check-docker

check-docker: ## Check if docker-compose and Docker daemon are available
	@which $(DOCKER_COMPOSE) > /dev/null || (echo -e "$(RED)Error: '$(DOCKER_COMPOSE)' not found! Please install Docker Compose.$(RESET)" && exit 1)
	@$(DOCKER) info > /dev/null 2>&1 || (echo -e "$(RED)Error: Docker daemon not running or unavailable! Please start Docker.$(RESET)" && exit 1)


# Help command (same as before, can be kept)
##############################################################################


help:
	@echo ""
	@echo -e "  $(BLUE)Distributed Expression Calculator - Makefile Help$(RESET)"
	@echo -e "  ----------------------------------------------------"
	@echo -e "  Usage: $(GREEN)make $(RESET)[\033[4mtarget\033[0m]"
	@echo ""
	@echo -e "  $(YELLOW)Commonly used targets:$(RESET)"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / { \
		if ($$1 ~ /^(up|down|logs|test|build|clean|help)$$/) \
			printf "    $(GREEN)%-20s$(RESET) %s\n", $$1, $$2 \
	}' $(MAKEFILE_LIST)
	@echo ""
	@echo -e "  $(YELLOW)Testing targets:$(RESET)"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / { \
		if ($$1 ~ /^test-.*/) \
			printf "    $(GREEN)%-20s$(RESET) %s\n", $$1, $$2 \
	}' $(MAKEFILE_LIST)
	@echo ""
	@echo -e "  $(YELLOW)Development & Other targets:$(RESET)"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / { \
		if (!($$1 ~ /^(up|down|logs|test|build|clean|help)$$/) && \
		    !($$1 ~ /^test-.*/) && \
		    !($$1 ~ /^(print-|info-|check-docker)$$/) && \
		    !($$1 ~ /build-.*/)) \
			printf "    $(GREEN)%-20s$(RESET) %s\n", $$1, $$2 \
	}' $(MAKEFILE_LIST)
	@echo -e "    $(GREEN)build-<service_name>  $(RESET) Build a specific service (e.g., make build-agent)"
	@echo -e "    $(GREEN)logs-<service_name>   $(RESET) Show logs for a specific service (e.g., make logs-agent)"
	@echo ""
	@echo -e "  $(YELLOW)Utility targets:$(RESET)"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / { \
		if ($$1 ~ /^(check-docker)$$/) \
			printf "    $(GREEN)%-20s$(RESET) %s\n", $$1, $$2 \
	}' $(MAKEFILE_LIST)
	@echo ""
	@echo -e "  Run $(GREEN)make help$(RESET) to see this message again."
	@echo ""


# Debugging commands
##############################################################################


print-%: ## Print the value of any Makefile variable (e.g., make print-GO_CMD)
	@echo -e "$(BLUE)$*$(RESET) = $($(*))"

info-%: ## Show the commands that would be executed for a target (dry run)
	@echo -e "$(YELLOW)Dry run for target '$*' (commands to be executed):$(RESET)"
	@$(MAKE) --dry-run --always-make $* | grep -v "info-" | grep -v "make\[" | sed 's/^echo/echo (simulated)/' | sed 's/^@//'