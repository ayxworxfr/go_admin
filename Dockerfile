# 构建阶段：使用 Go 1.24 构建二进制文件
FROM golang:1.24-alpine AS builder

# 安装 make 和其他必要工具
RUN apk add --no-cache make git

# 优先使用国内代理，失败时回源（direct）
ENV GOPROXY=https://goproxy.cn,https://mirrors.aliyun.com/goproxy/,direct

WORKDIR /app
COPY go.mod go.sum Makefile ./
RUN go mod download && make deps
COPY . .
RUN make build BINARY_EXT=""

# 生产阶段：使用轻量级 Alpine 镜像
FROM alpine:3.18

# 指定应用工作目录
WORKDIR /app

# 构建参数
ARG TZ=Asia/Shanghai
ENV TZ=$TZ

RUN apk update && apk add --no-cache curl tzdata \
    && ln -snf "/usr/share/zoneinfo/$TZ" /etc/localtime \
    && echo "$TZ" > /etc/timezone \
    && rm -rf /var/cache/apk/*

# 创建非 root 用户和配置目录
RUN addgroup -S app && adduser -S app -G app
RUN mkdir -p /app/conf /app/logs \
    && chown -R app:app /app/logs  # 递归设置目录及子文件权限
USER app

# 复制二进制文件
COPY --from=builder /app/build/go_admin /app/

# 定义默认端口
ENV APP_PORT=8888
# 设置容器启动命令
CMD ["/app/go_admin", "-p", "$APP_PORT"]