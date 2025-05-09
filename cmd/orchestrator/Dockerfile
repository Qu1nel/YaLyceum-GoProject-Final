FROM golang:1.24.2-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /orchestrator cmd/orchestrator/main.go

FROM alpine:latest

WORKDIR /app

COPY --from=builder /orchestrator /app/orchestrator

CMD ["/app/orchestrator"]