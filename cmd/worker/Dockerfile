FROM golang:1.24.2-alpine AS builder

WORKDIR /app

# Копируем зависимости и скачиваем их (из корня проекта)
COPY go.mod go.sum ./
RUN go mod download

# Копируем исходный код (из корня проекта)
COPY . .

# Собираем приложение Воркера
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /worker cmd/worker/main.go

# Этап выполнения
FROM alpine:latest

WORKDIR /app

# Копируем бинарник из этапа сборки
COPY --from=builder /worker /app/worker

# Запускаем приложение Воркера
CMD ["/app/worker"]