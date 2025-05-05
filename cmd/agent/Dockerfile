FROM golang:1.22-alpine AS builder

WORKDIR /app

# Копируем зависимости и скачиваем их
COPY go.mod go.sum ./
RUN go mod download

# Копируем исходный код
COPY . .

# Собираем приложение (статически линкуем для Alpine)
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /agent cmd/agent/main.go

# Этап выполнения
FROM alpine:latest

WORKDIR /app

# Копируем бинарник из этапа сборки
COPY --from=builder /agent /app/agent

# Копируем файл .env.example, чтобы можно было создать .env
# COPY .env.example .env # Мы будем передавать переменные через docker-compose

# Открываем порт (будет указан в конфигурации)
# EXPOSE 8080 # Лучше управлять этим через docker-compose

# Запускаем приложение
CMD ["/app/agent"]