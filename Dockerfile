# 构建阶段 — 使用多阶段构建减小镜像体积
FROM golang:1.21-alpine AS builder

WORKDIR /app

# 安装 CGO 依赖（SQLite 需要）
RUN apk add --no-cache gcc musl-dev sqlite-dev

# 先复制依赖文件，利用 Docker 缓存层
COPY go.mod go.sum ./
RUN go mod download && go mod verify

# 复制源代码
COPY . .

# 构建二进制（静态链接 + 去调试信息）
RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o harness ./cmd/harness

# 运行阶段 — 最小化镜像
FROM alpine:3.19

RUN apk --no-cache add ca-certificates sqlite tzdata \
    && addgroup -S harness && adduser -S harness -G harness

WORKDIR /app

# 从构建阶段复制二进制
COPY --from=builder /app/harness .
COPY --from=builder /app/harness.yaml .

# 创建数据目录并设置权限
RUN mkdir -p /app/data && chown -R harness:harness /app

USER harness

EXPOSE 8080 9090

HEALTHCHECK --interval=30s --timeout=3s --retries=3 \
    CMD wget -qO- http://localhost:8080/api/monitor/health || exit 1

CMD ["./harness"]
