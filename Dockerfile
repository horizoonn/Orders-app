# Этап 1: Сборка
FROM golang:alpine3.22 AS builder
WORKDIR /app
ARG SERVICE_PATH=./cmd/order-service
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main ${SERVICE_PATH}/main.go

# Этап 2: Финальный образ
FROM alpine:3.22
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/main .
COPY ./web ./web
EXPOSE 8082
CMD ["./main"]