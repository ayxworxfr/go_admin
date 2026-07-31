# Docker 部署指南

两套编排二选一，**不要同时启动**（会抢 3306 / 6379 等端口）：

| 编排 | 文件 | 场景 |
|------|------|------|
| 单实例 | 根目录 `docker-compose.yml` + `.env` | 开发联调 / 小生产 |
| 多实例 HA | `conf/common/docker-compose.yml` + `conf/common/.env` | Caddy 负载均衡 + 双 app + 监控 |

配置目录说明见 [conf/README.md](conf/README.md)。

---

## 一、单实例（推荐入门）

### 1. 准备密钥（首次）

```bash
cp .env.example .env
# 按需修改 MYSQL_*、JWT_SECRET
# 应用通过 DATABASE_* / JWT_SECRET 注入，不以 config_docker.yaml 明文为准
```

### 2. 启动

```bash
# Makefile
make docker-compose-up

# 或
docker compose up --build -d
```

> **构建**：`go mod download` → `go run ./cmd/routegen` → `go build`。需要 BuildKit。

### 3. 状态 / 日志 / 停止

```bash
make docker-compose-status
make docker-compose-logs
make docker-compose-down
```

### 服务与端口

| 服务 | 端口 | 说明 |
|------|------|------|
| app | `${APP_PORT:-8888}` | Go Admin API |
| mysql | 3306 | MySQL 8.0（`go_mysql`） |
| redis | 6379 | Redis 7（`go_redis`） |
| jaeger | 16686 | Jaeger UI（看链路） |
| jaeger | 4317 / 4318 | OTLP（容器内 / 可选直连） |
| otel-collector | 43170→4317 | 本机应用 OTLP gRPC |
| otel-collector | 43180→4318 | OTLP HTTP **上报**（`POST /v1/traces`，不是网页） |

### 环境变量（`.env`）

| 变量 | 默认 | 作用 |
|------|------|------|
| `APP_PORT` | `8888` | 监听端口；与映射、healthcheck 同步 |
| `MYSQL_ROOT_PASSWORD` | `123456` | MySQL root |
| `MYSQL_USER` / `MYSQL_PASSWORD` / `MYSQL_DATABASE` | `go_user` / `go_user123` / `go_admin` | 业务库；映射为 app 的 `DATABASE_*` |
| `JWT_SECRET` | （必填） | JWT 签名密钥 |
| `REDIS_PASSWORD` | 空 | Redis 密码 |
| `INSTANCE_ID` | `app` | OTEL service 名 |

```bash
APP_PORT=9000 make docker-compose-up
```

### 访问地址

- API: http://localhost:8888
- 健康检查: http://localhost:8888/health
- Jaeger UI: http://localhost:16686
- MySQL: `localhost:3306`（账号见 `.env`）
- Redis: `localhost:6379`

默认管理员：`admin` / `admin123`（`mysql/init_data.sql`）。

---

## 二、多实例 HA（Caddy）

```bash
# 先停单实例
docker compose down

cp conf/common/.env.example conf/common/.env
docker compose --env-file conf/common/.env \
  -f conf/common/docker-compose.yml up -d --build
```

架构：`Caddy(edge)` → `app1/app2` → `MySQL/Redis(data)`；链路经 `otel-collector` → `Jaeger(obs)`。

### 访问地址

| 用途 | 地址 |
|------|------|
| API | http://localhost/api/... |
| Caddy 探活 | http://localhost/health |
| Jaeger UI | http://127.0.0.1:16686 |
| Collector 探活 | http://127.0.0.1:13133/ → `{"status":"Server available",...}` |
| Collector zpages | http://127.0.0.1:55679/debug/tracez |
| Prometheus | http://127.0.0.1:9090 |
| Grafana | http://127.0.0.1:3000 |
| OTLP HTTP | `POST http://127.0.0.1:43180/v1/traces`（浏览器打开无效） |

Caddy 访问日志：`conf/common/logs/caddy/access.log`（对 `:80` 产生请求后生成）。  
应用日志：`conf/common/logs/app1`、`app2`。

MySQL / Redis / Grafana / Prometheus 默认只绑 `127.0.0.1`，不对局域网暴露。

---

## 配置文件索引

| 路径 | 用途 |
|------|------|
| `.env` / `.env.example` | 单实例密钥 |
| `conf/common/.env` / `.env.example` | 多实例密钥 |
| `conf/config_docker.yaml` | Docker 应用非密钥配置 |
| `conf/sentinel.yaml` | Sentinel 限流 |
| `conf/common/mysql.cnf` | MySQL 8（经 entrypoint 以 0644 安装） |
| `conf/common/redis.conf` | Redis |
| `conf/common/otel-collector-config.yaml` | Collector |
| `conf/common/Caddyfile` | 负载均衡与访问日志 |
| `conf/common/prometheus.yml` | Prometheus 抓取 |
| `conf/common/docker-compose.yml` | HA 编排 |

密钥覆盖逻辑：`internal/platform/config/env.go`（`applyEnvOverrides`）。

---

## 常用命令（单实例）

```bash
make docker-compose-rebuild
make docker-compose-logs
make docker-compose-restart
docker compose ps

docker compose exec mysql mysql -u go_user -p"$MYSQL_PASSWORD" go_admin
docker compose exec redis redis-cli
docker compose logs app
docker compose restart app
```

## 数据持久化

- 卷：`mysql_data`、`redis_data`（HA 另有 `caddy_*`、`grafana_data`、`prometheus_data`）
- 单实例应用日志：`./logs`
- HA 应用 / Caddy 日志：`conf/common/logs/`

> 改 `.env` 里的 `MYSQL_PASSWORD` **不会**自动更新已有数据卷中的账号，需 `docker compose down -v` 或手动 `ALTER USER`。

---

## 故障排除

### 1. 端口冲突

改 `.env` 的 `APP_PORT`，或停掉另一套编排。

### 2. `go_mysql is unhealthy` / 初始化失败

常见原因：

- `init_user.sql` 失败（不要对 `information_schema` 做 GRANT）
- WSL 下 `mysql.cnf` 呈 0777 被忽略（当前 compose 已用 `/tmp` + `install -m 0644`）

重建数据卷：

```bash
docker compose down -v
docker compose up -d --build
```

### 3. 应用连不上库

确认：

1. MySQL 已 `healthy`
2. 容器内有 `DATABASE_HOST=mysql`、`DATABASE_PASSWORD`（与 `.env` 一致）
3. 未把密钥只写在 yaml 却忘记注入环境变量

### 4. `http://localhost:43180/` 浏览器打不开

正常。那是 OTLP 上报口。看链路用 Jaeger；Collector 探活用 `:13133`（HA 栈）。

### 5. Caddy 目录没有日志

1. 确认跑的是 HA 栈且 Caddy 已起
2. 对 `http://localhost/health` 发过请求
3. 查看 `conf/common/logs/caddy/access.log`（`:80` 已配置 access_log）

### 6. 路由 404

改过 `// @route` 后需 `docker compose up -d --build`（镜像内跑 routegen）。

### 7. 查看日志

```bash
docker compose logs
docker compose logs app
docker compose logs mysql
```

---

## 清理

```bash
make docker-compose-clean
# 或
docker compose down --volumes --rmi all
```

HA：

```bash
docker compose --env-file conf/common/.env \
  -f conf/common/docker-compose.yml down --volumes
```

---

## API 冒烟

```bash
# 单实例
curl -sS http://localhost:8888/health
curl -sS -X POST http://localhost:8888/api/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}'

# HA（经 Caddy）
curl -sS http://localhost/health
curl -sS -X POST http://localhost/api/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}'
```
