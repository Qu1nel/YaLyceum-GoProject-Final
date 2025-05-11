# Chapter 9: Makefile

После того, как мы изучили [Логгер (Logger)](08_логгер__logger_.md), давайте поговорим об автоматизации задач в нашем проекте с помощью **Makefile**.

Представьте, что вы печете торт. Вам нужно выполнить множество шагов: достать ингредиенты, смешать их, поставить в духовку, украсить. Вы могли бы делать все это вручную каждый раз, но гораздо удобнее иметь список рецептов, в котором все шаги прописаны. **Makefile** – это как такой список рецептов для нашего проекта. Он позволяет автоматизировать часто выполняемые задачи, такие как сборка, тестирование, запуск и остановка сервисов.

Центральный пример: Допустим, вам нужно запустить все сервисы нашего приложения. Без **Makefile**, вам пришлось бы вручную вводить длинные команды для каждого сервиса. С **Makefile**, вы можете просто ввести `make up`, и все сервисы запустятся автоматически.

## Что такое Makefile?

**Makefile** – это текстовый файл, который содержит набор правил и команд для автоматизации задач. Каждое правило состоит из:

1.  **Цели (Target):** Это имя задачи, которую мы хотим выполнить (например, `up`, `test`, `clean`).
2.  **Зависимостей (Dependencies):** Это другие цели, которые должны быть выполнены перед выполнением данной цели.
3.  **Команд (Commands):** Это команды, которые выполняются для достижения цели.

Когда вы запускаете `make <цель>`, `make` читает **Makefile**, находит правило для указанной цели и выполняет команды, связанные с этой целью. Если у цели есть зависимости, `make` сначала выполняет эти зависимости.

## Ключевые концепции Makefile

Давайте рассмотрим ключевые концепции **Makefile** на примере нашего проекта.

1.  **Цели (Targets):** Как мы уже говорили, это имена задач, которые мы хотим выполнить. В нашем **Makefile** есть такие цели, как `up`, `down`, `test`, `clean`.
2.  **.PHONY:** Это атрибут, который указывает, что цель не является файлом. Это означает, что `make` всегда будет выполнять команды, связанные с этой целью, даже если существует файл с таким же именем. В нашем **Makefile** все цели объявлены как `.PHONY`.
3.  **Переменные (Variables):** Это имена, которым мы присваиваем значения. В нашем **Makefile** есть переменные для команд (`GO_CMD`, `DOCKER_COMPOSE`), флагов (`GO_TEST_FLAGS`, `GO_COVER_FLAGS`) и портов (`FRONTEND_PORT`, `AGENT_HTTP_PORT`). Использование переменных делает **Makefile** более читаемым и легким в обслуживании.
4.  **Команды (Commands):** Это команды, которые выполняются для достижения цели. Команды должны начинаться с символа табуляции (!!!) - это очень важно!

## Как использовать Makefile?

Давайте рассмотрим, как использовать **Makefile** для запуска всех сервисов нашего приложения.

**Задача:** Запустить все сервисы приложения.

1.  **Откройте терминал:** Откройте терминал в корневой директории проекта `YaLyceum-GoProject-Final`.
2.  **Выполните команду:** Введите команду `make up` и нажмите Enter.

    ```bash
    make up
    ```

    Это запустит все сервисы, определенные в файле `docker-compose.yml`, в фоновом режиме. Вы увидите сообщения о том, что сервисы запускаются и строятся, если необходимо.
    ```
    Starting services (building if necessary)...
    ...
    Services started successfully!
    ...
    ```

    `make up` автоматически построит (build) и запустит (up) все сервисы, указанные в `docker-compose.yml`. Ключ `-d` говорит Docker Compose запустить контейнеры в фоновом режиме (detached). После запуска сервисов, выводятся адреса Frontend и Agent API, а также полезные команды, такие как `make logs` и `make down`.

Давайте рассмотрим другую задачу.

**Задача:** Посмотреть логи Агента.

1.  **Откройте терминал:** Откройте терминал в корневой директории проекта `YaLyceum-GoProject-Final`.
2.  **Выполните команду:** Введите команду `make logs-agent` и нажмите Enter.

    ```bash
    make logs-agent
    ```

    Это покажет логи Агента. Вы сможете видеть, что происходит внутри Агента, какие запросы он получает и как он их обрабатывает.

    ```
    Following logs for 'agent' (press Ctrl+C to exit)...
    ...
    ```
    Эта команда покажет логи в режиме реального времени, пока вы не нажмете `Ctrl+C`.

## Что происходит под капотом?

Давайте разберемся, что происходит, когда мы выполняем команду `make up`.

1.  **Чтение Makefile:** `make` читает файл **Makefile** в корневой директории проекта.
2.  **Поиск цели `up`:** `make` находит правило для цели `up`.
3.  **Проверка зависимостей:** Цель `up` зависит от цели `check-docker`. `make` проверяет, нужно ли выполнять цель `check-docker`.
4.  **Выполнение `check-docker`:** Цель `check-docker` проверяет, установлены ли `docker-compose` и Docker. Если они не установлены, `make` выдает ошибку и останавливается.

    ```mermaid
    sequenceDiagram
        participant Пользователь
        participant Make
        participant DockerCompose
        participant Docker

        Пользователь->Make: make up
        Make->Make: Прочитать Makefile
        Make->Make: Найти цель up
        Make->Make: Зависимость: check-docker
        Make->Make: Найти цель check-docker
        Make->DockerCompose: which docker-compose
        alt docker-compose не найден
            DockerCompose-->>Make: Error
            Make-->>Пользователь: Error
        else docker-compose найден
            DockerCompose->Docker: docker info
            alt Docker не запущен
                Docker-->>Make: Error
                Make-->>Пользователь: Error
            else Docker запущен
                Docker-->>Make: OK
                Make->DockerCompose: docker-compose up --build -d
                DockerCompose->Docker: Запуск сервисов
                DockerCompose-->>Make: OK
                Make-->>Пользователь: OK
            end
        end
    ```

5.  **Выполнение `up`:** Если `check-docker` выполнена успешно, `make` выполняет команды, связанные с целью `up`. Эти команды запускают `docker-compose up --build -d`, что строит (если нужно) и запускает все сервисы, определенные в `docker-compose.yml`.

Теперь давайте посмотрим на код **Makefile**, чтобы увидеть, как это реализовано.

```makefile
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
```

Здесь мы видим правило для цели `up`. Оно указывает, что сначала нужно выполнить `check-docker`, а затем выполнить команды для запуска сервисов с помощью `docker-compose`.  `## Build and start all services in detached mode (-d)` - это комментарий, который отображается, когда вы запускаете `make help`.

```makefile
check-docker: ## Check if docker-compose and Docker daemon are available
	@which $(DOCKER_COMPOSE) > /dev/null || (echo -e "$(RED)Error: '$(DOCKER_COMPOSE)' not found! Please install Docker Compose.$(RESET)" && exit 1)
	@$(DOCKER) info > /dev/null 2>&1 || (echo -e "$(RED)Error: Docker daemon not running or unavailable! Please start Docker.$(RESET)" && exit 1)
```

Эта цель проверяет, установлен ли `docker-compose` и запущен ли Docker daemon. Если что-то не так, выводится сообщение об ошибке и выполнение прерывается.

```makefile
FRONTEND_PORT := $(call get_env_port,FRONTEND_PORT,$(DEFAULT_FRONTEND_PORT))
AGENT_HTTP_PORT := $(call get_env_port,AGENT_HTTP_PORT,$(DEFAULT_AGENT_HTTP_PORT))
```

Эти строки показывают, как **Makefile** использует функцию `get_env_port` для получения значения порта из переменной окружения или из `.env` файла, если переменная окружения не задана. Если ни переменная окружения, ни `.env` файл не содержат значения, используется значение по умолчанию.  Это использует [Конфигурация (Config)](01_конфигурация__config_.md), которую мы обсуждали ранее.

## Заключение

В этой главе мы узнали, что такое **Makefile** и как он работает. Мы рассмотрели, как использовать **Makefile** для автоматизации сборки, запуска и тестирования нашего приложения. Теперь вы знаете, как использовать **Makefile** для упрощения разработки и повышения эффективности работы с проектом.

В следующей главе мы поговорим о [Docker Compose](10_docker_compose.md), который используется для определения и управления многоконтейнерными Docker-приложениями.