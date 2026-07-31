# 配置目录结构说明

## 目录结构

```
conf/
├── README.md                    # 本说明
├── config.yaml                  # 本地开发配置（main 固定加载此路径）
├── config_docker.yaml           # Docker 应用配置（非密钥；密钥由环境变量注入）
├── config_test.yaml             # 测试环境应用配置
├── sentinel.yaml                # Sentinel 限流规则（应用侧加载，支持热更新）
└── common/                      # 基础设施 / 第三方服务配置
    ├── .env.example             # 多实例编排密钥模板 → 复制为 .env
    ├── mysql.cnf                # MySQL 8 自定义配置
    ├── redis.conf               # Redis 配置
    ├── otel-collector-config.yaml
    ├── prometheus.yml
    ├── Caddyfile                # 多实例入口（:80 + goadmin.com）
    ├── docker-compose.yml       # 多实例 HA（Caddy + app1/app2 + 监控）
    └── logs/                    # compose 挂载的运行时日志（勿提交密钥）
        ├── app1/
        ├── app2/
        └── caddy/               # access.log（需有请求后才会生成）
```

仓库根目录另有：

| 路径 | 用途 |
|------|------|
| `.env.example` / `.env` | **单实例**编排密钥（`docker-compose.yml`） |
| `docker-compose.yml` | 小生产 / 开发联调：单 app + MySQL + Redis + Jaeger + Collector |
| `DOCKER.md` | Docker 部署与排障详解 |

## 配置分类

### 应用配置（`conf/` 根目录）

与 Go 进程直接相关：

- `config.yaml` — 本地 `go run` / `make run`
- `config_docker.yaml` — Docker 内挂载为 `/app/conf/config.yaml`；**密码与 JWT 留空**，由环境变量覆盖
- `config_test.yaml` — 测试
- `sentinel.yaml` — 限流规则；支持文件监控热更新

环境变量覆盖 YAML（**非空才生效**，实现见 `internal/platform/config/env.go`）：

| 变量 | 覆盖字段 |
|------|----------|
| `APP_PORT` | `server.port` |
| `DATABASE_HOST` / `DATABASE_PORT` / `DATABASE_USER` / `DATABASE_PASSWORD` / `DATABASE_NAME` | `database.*` |
| `REDIS_HOST` / `REDIS_PORT` / `REDIS_PASSWORD` | `redis.*` |
| `JWT_SECRET` | `jwt.secret` |
| `INSTANCE_ID` | `opentelemetry.service`（多实例区分 app1/app2） |
| `OTEL_EXPORTER_OTLP_ENDPOINT` / `OTEL_EXPORTER_OTLP_PROTOCOL` | OTEL endpoint / protocol |

### 密钥唯一来源（`.env`）

| 编排 | 密钥文件 | 启动命令 |
|------|----------|----------|
| 单实例 | 仓库根目录 `.env` | `cp .env.example .env && docker compose up -d --build` |
| 多实例 HA | `conf/common/.env` | 见下方「Docker 多实例」 |

compose 将 `MYSQL_*` 注入 MySQL 容器，同时映射为应用的 `DATABASE_*` / `JWT_SECRET` / `REDIS_PASSWORD`。  
`mysql/init_user.sql` **不再写死密码**，只做 GRANT；账号密码以 `.env` 的 `MYSQL_USER` / `MYSQL_PASSWORD` 为准。

> 已有数据卷不会因改 `.env` 自动改库内密码；需 `docker compose down -v` 重建，或手动 `ALTER USER`。

### 基础设施配置（`conf/common/`）

由 Docker Compose 挂载，应用进程通常不直接解析：

| 文件 | 说明 |
|------|------|
| `mysql.cnf` | MySQL 8 参数；compose 先挂到 `/tmp` 再以 0644 拷入 `conf.d`（避免 WSL 0777 被忽略） |
| `redis.conf` | Redis |
| `otel-collector-config.yaml` | OTLP → Jaeger；含 health_check / zpages |
| `prometheus.yml` | 抓取 app / caddy / collector 等 |
| `Caddyfile` | `:80` 与 `goadmin.com` 均写访问日志到 `/var/log/caddy/access.log` |
| `docker-compose.yml` | 网络三分（edge / data / obs）+ Caddy 负载均衡 |

## 使用说明

### 本地运行

```bash
# 程序固定加载 conf/config.yaml（无 -config 命令行参数）
# 本地密钥可直接写在 yaml；也可用环境变量覆盖
vim conf/config.yaml
go run cmd/main.go
# 或
make run
```

### Docker 单实例（仓库根目录）

```bash
cp .env.example .env
docker compose up -d --build
# 或 make docker-compose-up
```

挂载要点：

```yaml
volumes:
  - ./conf:/app/conf:ro
  - ./conf/config_docker.yaml:/app/conf/config.yaml:ro
  - ./logs:/app/logs
```

访问：

| 用途 | 地址 |
|------|------|
| API | http://localhost:8888 |
| 健康检查 | http://localhost:8888/health |
| Jaeger UI | http://localhost:16686 |
| Collector 探活 | （单实例未默认映射 13133；看链路用 Jaeger） |
| OTLP gRPC（本机进程） | `127.0.0.1:43170` |
| OTLP HTTP 上报 | `POST http://127.0.0.1:43180/v1/traces`（**不是网页**） |

默认管理员：`admin` / `admin123`。

### Docker 多实例 HA（`conf/common/`）

与根目录单实例**不要同时起**（会抢 3306/6379 等端口）。

```bash
docker compose down                                          # 先停单实例
cp conf/common/.env.example conf/common/.env
docker compose --env-file conf/common/.env \
  -f conf/common/docker-compose.yml up -d --build
```

| 用途 | 地址 |
|------|------|
| API（经 Caddy） | http://localhost/api/... |
| Caddy 健康检查 | http://localhost/health |
| Jaeger UI | http://127.0.0.1:16686 |
| Collector 探活 | http://127.0.0.1:13133/ → `{"status":"Server available",...}` |
| Collector zpages | http://127.0.0.1:55679/debug/tracez |
| Prometheus | http://127.0.0.1:9090 |
| Grafana | http://127.0.0.1:3000（账号见 `conf/common/.env`） |
| OTLP HTTP | `POST :43180/v1/traces`（浏览器打开无效） |

Caddy 访问日志：`conf/common/logs/caddy/access.log`（对 `:80` 发过请求后才会出现）。

## 修改配置

```bash
# 应用非密钥配置
vim conf/config.yaml            # 本地
vim conf/config_docker.yaml     # Docker
vim conf/sentinel.yaml

# 密钥（Docker）
vim .env                        # 单实例
vim conf/common/.env            # 多实例

# 基础设施
vim conf/common/mysql.cnf
vim conf/common/redis.conf
vim conf/common/Caddyfile
vim conf/common/otel-collector-config.yaml

# 单实例改完后
make docker-compose-restart
# 或重建
docker compose up -d --build

# 多实例改完后
docker compose --env-file conf/common/.env \
  -f conf/common/docker-compose.yml up -d --build
```

## 注意事项

1. **密钥不要写进 `config_docker.yaml`**，也不要提交 `.env`（已在 `.gitignore`）
2. 基础设施挂载尽量 `:ro`；MySQL cnf 因 WSL 权限特殊处理，见 compose entrypoint
3. OTLP `4317`/`4318`/`43170`/`43180` 是上报协议端口，**不是**浏览器 UI；看链路用 Jaeger `16686`
4. 改 `// @route` 后需重新构建镜像（Dockerfile 内会跑 `routegen`）
5. 更完整的 Docker 排障见仓库根目录 [DOCKER.md](../DOCKER.md)
