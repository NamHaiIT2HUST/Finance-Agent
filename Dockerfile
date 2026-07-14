FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o finance-bot ./cmd/agent/main.go
FROM alpine:latest
WORKDIR /app
RUN apk --no-cache add ca-certificates tzdata
ENV TZ=Asia/Ho_Chi_Minh
COPY --from=builder /app/finance-bot .
COPY web/ ./web/
CMD ["./finance-bot"]
