# syntax=docker/dockerfile:1
# 多阶段构建：builder 只编译产物，不安装 lint / staticcheck 等开发工具。

FROM golang:1.24-alpine AS builder

RUN apk add --no-cache git ca-certificates

# 模块代理：国内优先；校验库走国内镜像，避免 sum.golang.org 被 RST
ENV CGO_ENABLED=0 \
    GOOS=linux \
    GOFLAGS="-trimpath" \
    GOPROXY=https://goproxy.cn,https://mirrors.aliyun.com/goproxy/,direct \
    GOSUMDB=sum.golang.google.cn

WORKDIR /src

# 依赖层单独缓存：go.mod/go.sum 不变时跳过 download
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .

# 先把 @route 编译进 routes_gen.go，再构建服务（运行镜像无源码，不能 ParseFile）
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go run ./cmd/routegen -root . && \
    go build -ldflags="-s -w" -o /out/go_admin ./cmd

# --- 运行镜像：仅含二进制与运行时依赖 ---
FROM alpine:3.21

ARG TZ=Asia/Shanghai
ENV TZ=$TZ

RUN apk add --no-cache ca-certificates curl tzdata \
    && ln -snf "/usr/share/zoneinfo/$TZ" /etc/localtime \
    && echo "$TZ" > /etc/timezone \
    && addgroup -S app \
    && adduser -S app -G app \
    && mkdir -p /app/conf /app/logs \
    && chown -R app:app /app

WORKDIR /app

COPY --from=builder /out/go_admin /app/go_admin
COPY --chown=app:app conf/ /app/conf/
# Docker 环境默认配置；compose 也可挂载覆盖 /app/conf/config.yaml
RUN cp /app/conf/config_docker.yaml /app/conf/config.yaml \
    && chown app:app /app/conf/config.yaml

USER app

# EXPOSE 仅作文档声明，不绑定、不限制端口；真正对外映射靠 compose/k8s 的 ports。
EXPOSE 8888

CMD ["/app/go_admin"]
