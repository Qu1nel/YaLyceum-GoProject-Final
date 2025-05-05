# Цвета (вроде на маке не работают)
RED=\033[0;31m
GREEN=\033[1;32m
YELLOW=\033[1;33m
BLUE=\033[0;36m
RESET=\033[0m

# Утилиты
DOCKER_COMPOSE := docker-compose
DOCKER := docker

# Основные команды
##############################################################################

.PHONY: help up down start stop build logs logs-agent logs-postgres test clean test-coverage check-docker

.DEFAULT_GOAL := help

up: check-docker ## Собрать и запустить все сервисы в фоновом режиме (-d)
	@echo -e "$(BLUE)Запуск сервисов (сборка при необходимости)...$(RESET)"
	@$(DOCKER_COMPOSE) up --build -d
	@echo -e "$(GREEN)Сервисы запущены!$(RESET)"
	@echo -e "$(YELLOW)Используйте 'make logs' или 'make logs-<service>' для просмотра логов.$(RESET)"
	@echo -e "$(YELLOW)Используйте 'make down' для остановки и удаления контейнеров.$(RESET)"

ps: check-docker ## Показать статус запущенных контейнеров Docker Compose
	@echo -e "$(BLUE)Статус контейнеров:$(RESET)"
	@$(DOCKER_COMPOSE) ps

down: check-docker ## Остановить и удалить контейнеры, сети и тома (данные БД будут удалены!)
	@echo -e "$(YELLOW)Остановка и удаление контейнеров, сетей и томов...$(RESET)"
	@$(DOCKER_COMPOSE) down -v --remove-orphans
	@echo -e "$(RED)Все остановлено и удалено.$(RESET)"

start: check-docker ## Запустить ранее собранные контейнеры
	@echo -e "$(BLUE)Запуск существующих контейнеров...$(RESET)"
	@$(DOCKER_COMPOSE) start
	@echo -e "$(GREEN)Контейнеры запущены.$(RESET)"

stop: check-docker ## Остановить запущенные контейнеры (без удаления)
	@echo -e "$(YELLOW)Остановка контейнеров...$(RESET)"
	@$(DOCKER_COMPOSE) stop
	@echo -e "$(RED)Контейнеры остановлены.$(RESET)"

build: check-docker ## Собрать или пересобрать образы сервисов
	@echo -e "$(YELLOW)Сборка Docker образов...$(RESET)"
	@$(DOCKER_COMPOSE) build
	@echo -e "$(GREEN)Сборка завершена.$(RESET)"

logs: check-docker ## Показать логи всех запущенных сервисов (follow)
	@echo -e "$(YELLOW)Просмотр логов (нажмите Ctrl+C для выхода)...$(RESET)"
	@$(DOCKER_COMPOSE) logs -f

logs-agent: check-docker ## Показать логи сервиса 'agent' (follow)
	@echo -e "$(YELLOW)Просмотр логов 'agent' (нажмите Ctrl+C для выхода)...$(RESET)"
	@$(DOCKER_COMPOSE) logs -f agent

logs-postgres: check-docker ## Показать логи сервиса 'postgres' (follow)
	@echo -e "$(YELLOW)Просмотр логов 'postgres' (нажмите Ctrl+C для выхода)...$(RESET)"
	@$(DOCKER_COMPOSE) logs -f postgres

# logs-orchestrator: ... (добавить по мере появления сервиса)
# logs-worker: ... (добавить по мере появления сервиса)

test: check-docker ## Запустить Go юнит-тесты для всех пакетов
	@echo -e "$(YELLOW)Запуск юнит-тестов...$(RESET)"
	@$(DOCKER) exec -it calculator_agent sh -c "go test -v ./..." || echo -e "$(RED)Тесты не пройдены или сервис 'agent' не запущен.$(RESET)" # Запускаем внутри контейнера агента
	# @go test -v ./... # Альтернатива: запуск локально, если Go установлен
	@echo -e "$(GREEN)Запуск тестов завершен.$(RESET)"

test-coverage: check-docker ## Запустить тесты с отчетом о покрытии (откроется в браузере)
	@echo -e "$(YELLOW)Запуск тестов с анализом покрытия...$(RESET)"
	@$(DOCKER) exec -it calculator_agent sh -c "go test -v -coverpkg=./... -coverprofile=coverage.out ./... && go tool cover -html=coverage.out -o coverage.html" || echo -e "$(RED)Не удалось выполнить тесты/сгенерировать отчет.$(RESET)"
	@echo -e "$(GREEN)Отчет о покрытии 'coverage.html' сгенерирован внутри контейнера agent.$(RESET)"
	@echo -e "$(YELLOW)Для просмотра скопируйте его: 'docker cp calculator_agent:/app/coverage.html .' и откройте в браузере.$(RESET)"
	# @go test -v -coverpkg=./... -coverprofile=coverage.out ./... # Локальный запуск
	# @go tool cover -html=coverage.out # Открыть отчет локально

clean: check-docker ## Очистить кэш сборки Go
	@echo -e "$(YELLOW)Очистка Go build cache...$(RESET)"
	@go clean -cache
	# Можно добавить удаление бинарников: rm -f agent orchestrator worker
	@echo -e "$(GREEN)Кэш очищен.$(RESET)"


# Вспомогательные команды
##############################################################################

.PHONY: check-docker

check-docker: ## Проверить наличие docker-compose
	@which $(DOCKER_COMPOSE) > /dev/null || (echo -e "$(RED)Ошибка: '$(DOCKER_COMPOSE)' не найден! Установите Docker Compose.$(RESET)" && exit 1)
	@$(DOCKER) info > /dev/null 2>&1 || (echo -e "$(RED)Ошибка: Docker демон не запущен или недоступен!$(RESET)" && exit 1)


# Помощь по командам
##############################################################################

help: ## Показать это справочное сообщение
	@echo -e "\n$(BLUE)Available targets for "make":$(RESET)"
	@echo "--------------------------------"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  $(GREEN)%-20s$(RESET) %s\n", $$1, $$2}' $(MAKEFILE_LIST)
	@echo -e "\n$(BLUE)Using example:$(RESET)"
	@echo "  make up       # Build and run all"
	@echo "  make test     # Run unit-tests"
	@echo "  make logs     # See logs"
	@echo "  make down     # Stop and remove all (include DB datas)"

# Отладочные команды (из твоего примера)
##############################################################################
print-%: ## Напечатать значение любой переменной Makefile (например, make print-DOCKER_COMPOSE)
	@echo -e "$(BLUE)$*$(RESET) = $($*)"

info-%: ## Показать команды, которые будут выполнены для цели (dry run)
	@$(MAKE) --dry-run --always-make $* | grep -v "info-"