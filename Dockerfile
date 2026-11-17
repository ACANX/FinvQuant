# FinvQuant All-in-One 镜像：一个容器 = 服务端 + 前端
#   - Go 服务端（Gin）监听 16001（API）
#   - 内嵌前端（Vue3+Vite8+Vuetify4 构建产物）监听 16002
# 镜像：ghcr.io/acanx/finvquant（docker pull 后直接部署）

# ---- 阶段 1：前端构建（BUILDPLATFORM=runner 原生架构，仅构建一次，跨平台复用产物）----
FROM --platform=$BUILDPLATFORM node:24-alpine AS web-builder

WORKDIR /web
COPY Web/package.json Web/package-lock.json ./
RUN npm ci
COPY Web/ ./
RUN npm run build

# ---- 阶段 2：Go 服务端构建（TARGETPLATFORM=目标架构，按平台编译）----
FROM --platform=$TARGETPLATFORM golang:1.25.3-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
# 将前端构建产物放入 embed 位置（internal/webui/dist）——来自 BUILDPLATFORM 阶段，双平台共享
COPY --from=web-builder /web/dist /app/internal/webui/dist

RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags "-s -w -X main.version=$(cat VERSION) -X main.commit=$(git rev-parse --short HEAD 2>/dev/null || echo dev)" \
    -o /finvquant ./cmd/server

# ---- 阶段 3：运行（单镜像单进程）----
FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S finvquant && adduser -S finvquant -G finvquant

COPY --from=builder /finvquant /usr/local/bin/finvquant
# 数据库迁移脚本（启动时自动建表，如 finv_quote_secu_kline_min）
COPY --from=builder /app/Deploy/Migrations /etc/finvquant/Migrations

ENV FINV_MIGRATIONS_DIR=/etc/finvquant/Migrations

USER finvquant
EXPOSE 16001 16002

ENTRYPOINT ["finvquant"]
