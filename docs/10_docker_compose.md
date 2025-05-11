# Chapter 10: Docker Compose

После того, как мы освоили [Makefile](09_makefile.md), который помогает нам автоматизировать задачи, давайте изучим инструмент, который упрощает развертывание и управление нашим многокомпонентным приложением: **Docker Compose**.

Представьте, что вы организуете вечеринку. У вас есть разные элементы: музыка, еда, напитки, гости. Вы могли бы настраивать каждый элемент по отдельности, но гораздо проще использовать список, в котором все элементы и их настройки перечислены вместе. **Docker Compose** – это как такой список для нашего приложения. Он позволяет определить и запустить все наши сервисы (Агент, Оркестратор, Воркер, PostgreSQL, Frontend) как одно приложение.

Центральный пример: Допустим, вам нужно запустить все сервисы нашего приложения, чтобы проверить его работу. Без **Docker Compose**, вам пришлось бы вручную запускать каждый контейнер по отдельности, настраивать сети и связи между ними. С **Docker Compose**, вы можете просто выполнить одну команду, и все сервисы запустятся автоматически.

## Что такое Docker Compose?

**Docker Compose** – это инструмент для определения и запуска многоконтейнерных Docker-приложений. Он использует YAML-файл (обычно называемый `docker-compose.yml`) для определения сервисов, сетей и томов, необходимых для вашего приложения.

**Ключевые концепции Docker Compose:**

1.  **Сервис (Service):** Это как отдельный компонент нашего приложения, например, Агент, Оркестратор, Воркер, PostgreSQL или Frontend. Каждый сервис определяется как контейнер Docker, который запускается из определенного образа (image).
2.  **Образ (Image):** Это как шаблон для создания контейнера. Он содержит все необходимые файлы и инструкции для запуска сервиса.
3.  **Сеть (Network):** Это как виртуальная сеть, которая позволяет сервисам общаться друг с другом.
4.  **Том (Volume):** Это как хранилище данных, которое сохраняется даже после остановки и удаления контейнера.

## Как использовать Docker Compose?

Давайте рассмотрим, как использовать **Docker Compose** для запуска всех сервисов нашего приложения.

**Задача:** Запустить все сервисы приложения.

1.  **Создайте файл `docker-compose.yml`:** В корневой директории проекта `YaLyceum-GoProject-Final` должен быть файл `docker-compose.yml`. Откройте его и убедитесь, что он содержит описания всех необходимых сервисов, сетей и томов. Вот упрощенный пример:

    ```yaml
    version: "3.9"
    services:
      postgres:
        image: postgres:15-alpine
        ports:
          - "5432:5432" # Keep port definition inline, simpler for beginners
        environment:
          POSTGRES_PASSWORD: password # Keep password inline for simplicity
      agent:
        build:
          context: . # current dir
          dockerfile: cmd/agent/Dockerfile
        ports:
          - "8080:8080"  # Keep port definition inline, simpler for beginners
        depends_on:
          - postgres
        environment:
          POSTGRES_DSN: "postgres://user:password@postgres:5432/calculator_db?sslmode=disable" #Keep DSN string inline
    networks:
      default: # Default bridge network.
    ```

    Этот файл определяет два сервиса: `postgres` и `agent`. Сервис `postgres` использует образ `postgres:15-alpine` и экспортирует порт 5432. Сервис `agent` собирается из Dockerfile в директории `cmd/agent` и зависит от сервиса `postgres`.

2.  **Откройте терминал:** Откройте терминал в корневой директории проекта `YaLyceum-GoProject-Final`.

3.  **Выполните команду:** Введите команду `docker-compose up` и нажмите Enter.

    ```bash
    docker-compose up
    ```

    Эта команда запустит все сервисы, определенные в файле `docker-compose.yml`. Вы увидите сообщения о том, что сервисы запускаются и строятся, если необходимо.

    ```
    Creating network "yalyceum-goproject-final_default" with the default driver
    Creating volume "yalyceum-goproject-final_postgres_data" with default driver
    Creating calculator_postgres ... done
    Creating calculator_agent    ... done
    Attaching to calculator_postgres, calculator_agent
    postgres            | 2024-11-05 14:30:00.000 UTC [1] LOG:  listening on IPv4 address "0.0.0.0", port 5432
    postgres            | 2024-11-05 14:30:00.000 UTC [1] LOG:  listening on IPv6 address "::", port 5432
    agent               | 2024/11/05 14:30:01 Agent started
    ```

    `docker-compose up` автоматически построит (build) и запустит (up) все сервисы, указанные в `docker-compose.yml`.

    Если вы хотите запустить сервисы в фоновом режиме, добавьте флаг `-d`:

    ```bash
    docker-compose up -d
    ```

4. **Остановка сервисов:** Чтобы остановить сервисы, выполните команду `docker-compose down`:

   ```bash
   docker-compose down
   ```

   Эта команда остановит и удалит все контейнеры, сети и тома, созданные **Docker Compose**.

## Что происходит под капотом?

Давайте разберемся, что происходит, когда мы выполняем команду `docker-compose up`.

1.  **Чтение `docker-compose.yml`:** `docker-compose` читает файл `docker-compose.yml` в корневой директории проекта.
2.  **Создание сети:** Если сеть не существует, `docker-compose` создает новую сеть для приложения.
3.  **Создание томов:** Если тома не существуют, `docker-compose` создает новые тома для хранения данных.
4.  **Сборка образов:** Если указан `build`, `docker-compose` собирает образы из Dockerfile.
5.  **Запуск контейнеров:** `docker-compose` запускает контейнеры для каждого сервиса, используя указанные образы, сети, тома и переменные окружения.
6.  **Управление зависимостями:** `docker-compose` управляет зависимостями между сервисами, запуская сервисы в правильном порядке. Например, сервис `agent` зависит от сервиса `postgres`, поэтому `docker-compose` сначала запустит `postgres`, а затем `agent`.

```mermaid
sequenceDiagram
    participant Пользователь
    participant DockerCompose
    participant Docker
    participant Сеть as Network
    participant Том as Volume
    participant Контейнер as Container

    Пользователь->DockerCompose: docker-compose up
    DockerCompose->DockerCompose: Прочитать docker-compose.yml
    DockerCompose->Сеть: Создать сеть (если не существует)
    Сеть-->DockerCompose:
    DockerCompose->Том: Создать том (если не существует)
    Том-->DockerCompose:
    DockerCompose->Docker: Собрать образ (если build указан)
    Docker-->DockerCompose: Образ
    DockerCompose->Docker: Запустить контейнер (postgres)
    Docker-->Контейнер: Запущен postgres
    DockerCompose->Docker: Запустить контейнер (agent)
    Docker-->Контейнер: Запущен agent
    DockerCompose-->Пользователь:
```

Теперь давайте посмотрим на код `docker-compose.yml`, чтобы увидеть, как это реализовано. Полный `docker-compose.yaml` можно увидеть в корне репозитория.

```yaml
version: "3.9"
services:
  postgres:
    image: postgres:15-alpine
    container_name: calculator_postgres
    environment:
      POSTGRES_DB: ${POSTGRES_DB:-calculator_db} # default DB name
      POSTGRES_USER: ${POSTGRES_USER:-user}  # default user name
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD:-password} # default password
    ports:
      - "${POSTGRES_PORT:-5432}:5432"  # default PostgreSQL port
    volumes:
      - postgres_data:/var/lib/postgresql/data # Persistant volume
      - ./migrations:/docker-entrypoint-initdb.d  # init database scripts.
    healthcheck: # Check if postgres is healthy.
      test: ["CMD-SHELL", "pg_isready -U $${POSTGRES_USER} -d $${POSTGRES_DB}"]
      interval: 10s
      timeout: 5s
      retries: 5
    networks:
      - calculator_net
```

Этот код описывает сервис `postgres`.  `image` указывает на образ, который будет использоваться для запуска контейнера.  `environment` указывает переменные окружения, которые будут установлены в контейнере.  `ports` указывает, какие порты будут экспортированы из контейнера.  `${POSTGRES_PORT:-5432}` использует значение переменной окружения `POSTGRES_PORT`, если она задана, или значение по умолчанию `5432`. Этот механизм использует [Конфигурация (Config)](01_конфигурация__config_.md) для настройки переменных окружения.

```yaml
  agent:
    container_name: calculator_agent
    build:
      context: .
      dockerfile: cmd/agent/Dockerfile
    depends_on:
      postgres:
        condition: service_healthy
    ports:
      - "${AGENT_HTTP_PORT:-8080}:8080"
    environment:
      POSTGRES_DSN: "postgres://${POSTGRES_USER:-user}:${POSTGRES_PASSWORD:-password}@postgres:5432/${POSTGRES_DB:-calculator_db}?sslmode=disable"
      JWT_SECRET: ${JWT_SECRET:-your_very_secret_key_change_me} # NEVER push secret keys in plain text.
      ORCHESTRATOR_GRPC_ADDRESS: ${ORCHESTRATOR_GRPC_ADDRESS:-orchestrator:50051}
    networks:
      - calculator_net
```

Этот код описывает сервис `agent`. `build` указывает, что образ должен быть собран из Dockerfile. `depends_on` указывает, что сервис `agent` зависит от сервиса `postgres`.  Сервис `agent` не запустится, пока сервис `postgres` не станет здоровым (healthy), как указано в `healthcheck` сервиса `postgres`.

## Заключение

В этой главе мы узнали, что такое **Docker Compose** и как он работает. Мы рассмотрели, как использовать **Docker Compose** для определения и запуска многоконтейнерных Docker-приложений. Теперь вы знаете, как использовать **Docker Compose** для упрощения развертывания и управления нашим приложением.

Вы закончили все главы этого туториала! Надеюсь, что это поможет вам разрабатывать сложные распределенные системы в будущем.


---

Generated by [AI Codebase Knowledge Builder](https://github.com/The-Pocket/Tutorial-Codebase-Knowledge)