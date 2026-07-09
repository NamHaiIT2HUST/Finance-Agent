# Stage 1: Build the Go binary
FROM golang:1.21-alpine AS builder

# Thiết lập thư mục làm việc trong container
WORKDIR /app

# Copy go.mod và go.sum trước để tận dụng cache của Docker
COPY go.mod go.sum ./
RUN go mod download

# Copy toàn bộ mã nguồn
COPY . .

# Build ứng dụng (CGO_ENABLED=0 để tạo ra binary độc lập, giảm dung lượng)
RUN CGO_ENABLED=0 GOOS=linux go build -o finance-bot ./cmd/agent/main.go

# Stage 2: Tạo image nhẹ nhàng để chạy
FROM alpine:latest

WORKDIR /app

# Cài đặt ca-certificates để có thể gọi API HTTPS (Gemini, Telegram, Google Sheets)
RUN apk --no-cache add ca-certificates tzdata

# Đặt timezone (tùy chọn, ví dụ cho múi giờ Việt Nam)
ENV TZ=Asia/Ho_Chi_Minh

# Copy binary từ bước builder sang
COPY --from=builder /app/finance-bot .

# Khai báo file credentials.json (bạn có thể mount file này vào khi chạy, hoặc copy thẳng nếu không ngại rủi ro bảo mật)
# Tốt nhất là sử dụng biến môi trường hoặc mount volume, nhưng để đơn giản, ta copy nếu có
COPY credentials.json .

# Nếu có file .env cũng có thể copy, nhưng tốt nhất nên truyền Env vars từ nền tảng Cloud
# COPY .env .

# Chạy bot
CMD ["./finance-bot"]
