FROM golang:1.24.2-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /agent cmd/agent/main.go

FROM alpine:latest

WORKDIR /app

COPY --from=builder /agent /app/agent

CMD ["/app/agent"]