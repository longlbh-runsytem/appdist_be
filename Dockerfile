# Build trên máy Mac (ARM) cho server x86:
#   docker buildx build --platform linux/amd64 -t appdist-be:<version> --load .

FROM golang:1.26-alpine AS build
WORKDIR /src

# Tải module trước để cache được khi chỉ sửa code.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/server . \
 && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/createadmin ./cmd/createadmin

FROM alpine:3.22
# ca-certificates: gọi HTTPS tới Backblaze B2. tzdata: giờ Asia/Ho_Chi_Minh cho thống kê.
RUN apk add --no-cache ca-certificates tzdata \
 && adduser -D -H -u 10001 app
WORKDIR /app
COPY --from=build /out/ /app/
USER app

# Cấu hình chỉ đọc từ biến môi trường (env_file của compose) — image không chứa app.env.
ENV GIN_MODE=release
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s \
  CMD wget -qO- http://127.0.0.1:8080/health >/dev/null || exit 1
ENTRYPOINT ["/app/server"]
