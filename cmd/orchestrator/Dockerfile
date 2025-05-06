FROM golang:1.24.2-alpine AS builder

WORKDIR /app

# Копируем зависимости и скачиваем их (из корня проекта)
COPY go.mod go.sum ./
RUN go mod download

# Копируем исходный код (из корня проекта)
COPY . .

# Собираем приложение Оркестратора
# Путь к main.go относительно корня (/app)
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /orchestrator cmd/orchestrator/main.go

# Этап выполнения
FROM alpine:latest

WORKDIR /app

# Копируем бинарник из этапа сборки
COPY --from=builder /orchestrator /app/orchestrator

# Запускаем приложение Оркестратора
CMD ["/app/orchestrator"]