# ---- Stage 1: сборка ----
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Копируем go.mod и go.sum для тогo, чтобы закешировать зависимости
COPY go.mod go.sum ./
RUN go mod download

# Копируем исходники и собираем бинарник
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o server main.go

# ---- Stage 2: минимальный образ ----
FROM alpine:3.18

# Копируем бинарь из builder
COPY --from=builder /app/server /usr/local/bin/server

# Рабочая директория
WORKDIR /app

# Копируем файл .env (если нужно)
# на продакшене лучше монтировать через docker-compose, но можно и так:
COPY .env .env

# Открываем порт (настройте, если иначе)
EXPOSE 8080

# Запуск
CMD ["server"]
