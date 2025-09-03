FROM golang:1.21-alpine AS builder

# 安装必要的包
RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /app

# 复制 go mod 文件并下载依赖
COPY go.mod go.sum ./
RUN go mod download

# 复制源代码
COPY . .

# 构建应用
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o tesla-token-service ./cmd/server

# 最终镜像
FROM alpine:latest

# 安装必要的包
RUN apk --no-cache add ca-certificates tzdata

WORKDIR /root/

# 复制二进制文件
COPY --from=builder /app/tesla-token-service .

# 创建非 root 用户
RUN addgroup -g 1001 appgroup && \
    adduser -D -s /bin/sh -u 1001 -G appgroup appuser

# 切换到非 root 用户
USER appuser

# 暴露端口
EXPOSE 8080

# 健康检查
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# 启动应用
CMD ["./tesla-token-service"]