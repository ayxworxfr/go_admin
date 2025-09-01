# Docker 部署指南

本项目提供了完整的 Docker 环境，包含 MySQL、Redis、Jaeger 追踪系统和应用本身。

## 🚀 快速启动

### 1. 启动所有服务
```bash
# 方式一：使用 Makefile
make docker-compose-up

# 方式二：直接使用 docker-compose
docker-compose up --build -d
```

### 2. 查看服务状态
```bash
# 查看所有服务状态
make docker-compose-status

# 查看日志
make docker-compose-logs
```

### 3. 停止服务
```bash
make docker-compose-down
```

## 📋 服务列表

| 服务名 | 容器名 | 端口 | 描述 |
|--------|--------|------|------|
| app | go_admin_scaffold_app | 8888 | Go Admin 脚手架应用 |
| mysql | go_mysql | 3306 | MySQL 8.0 数据库 |
| redis | go_redis | 6379 | Redis 7 缓存 |
| jaeger | jaeger | 16686 | Jaeger UI 追踪系统 |
| otel-collector | otel-collector | 4317/4318 | OpenTelemetry 收集器 |

## 🔧 配置说明

### 数据库配置
- **主机**: mysql (容器内网络)
- **端口**: 3306
- **数据库**: go_admin
- **用户名**: admin
- **密码**: admin123456
- **Root密码**: root123456

### Redis配置
- **主机**: redis (容器内网络)
- **端口**: 6379
- **密码**: 无

### 配置文件
- `conf/config_docker.yaml`: Docker 环境专用配置
- `conf/mysql.cnf`: MySQL 自定义配置
- `conf/redis.conf`: Redis 自定义配置

## 🌐 访问地址

启动成功后，可以访问以下地址：

- **应用 API**: http://localhost:8888
- **健康检查**: http://localhost:8888/api/hello
- **Jaeger UI**: http://localhost:16686
- **MySQL**: localhost:3306
- **Redis**: localhost:6379

## 📝 常用命令

### 开发调试
```bash
# 重新构建并启动
make docker-compose-rebuild

# 查看实时日志
make docker-compose-logs

# 重启服务
make docker-compose-restart

# 查看容器状态
docker-compose ps
```

### 数据库操作
```bash
# 进入 MySQL 容器
docker-compose exec mysql mysql -u admin -padmin123456 go_admin

# 进入 Redis 容器
docker-compose exec redis redis-cli

# 查看 MySQL 日志
docker-compose logs mysql

# 查看 Redis 日志
docker-compose logs redis
```

### 应用调试
```bash
# 查看应用日志
docker-compose logs app

# 进入应用容器
docker-compose exec app sh

# 重启应用服务
docker-compose restart app
```

## 🗂️ 数据持久化

项目使用 Docker 卷进行数据持久化：

- `mysql_data`: MySQL 数据目录
- `redis_data`: Redis 数据目录
- `./logs`: 应用日志目录

## 🔧 故障排除

### 1. 端口冲突
如果端口被占用，可以修改 `docker-compose.yml` 中的端口映射。

### 2. 数据库初始化失败
```bash
# 清理所有数据重新开始
make docker-compose-clean
make docker-compose-up
```

### 3. 应用无法连接数据库
检查 `conf/config_docker.yaml` 中的数据库配置是否正确。

### 4. 查看详细错误日志
```bash
# 查看所有服务日志
docker-compose logs

# 查看特定服务日志
docker-compose logs app
docker-compose logs mysql
docker-compose logs redis
```

## 🧹 清理资源

```bash
# 停止并清理所有资源（包括数据卷）
make docker-compose-clean

# 或者手动清理
docker-compose down --volumes --rmi all
docker system prune -f
```

## 📚 API 测试

启动成功后，可以使用以下方式测试 API：

### 1. 健康检查
```bash
curl http://localhost:8888/api/hello
```

### 2. 用户登录
```bash
curl -X POST http://localhost:8888/api/login \
  -H "Content-Type: application/json" \
  -d '{
    "username": "admin",
    "password": "123456"
  }'
```

### 3. 获取用户列表（需要先登录获取 token）
```bash
curl -X GET http://localhost:8888/api/users \
  -H "Authorization: Bearer YOUR_JWT_TOKEN"
```
