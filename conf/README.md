# 配置目录结构说明

## 📁 目录结构

```
conf/
├── README.md                    # 配置说明文档
├── config.yaml                  # 应用主配置文件
├── config_docker.yaml           # Docker 环境应用配置
├── config_test.yaml             # 测试环境应用配置
└── common/                      # 非应用配置目录
    ├── mysql.cnf                # MySQL 自定义配置
    ├── redis.conf               # Redis 配置
    ├── otel-collector-config.yaml  # OpenTelemetry 收集器配置
    ├── prometheus.yml           # Prometheus 监控配置
    ├── sentinel.yaml            # Sentinel 限流配置
    ├── Caddyfile                # Caddy 服务器配置
    └── docker-compose.yml       # 多实例部署配置
```

## 🎯 配置分类

### **应用配置** (conf/ 根目录)
存放与 Go 应用直接相关的配置文件：
- `config.yaml` - 应用主配置
- `config_docker.yaml` - Docker 环境配置
- `config_test.yaml` - 测试环境配置

### **基础设施配置** (conf/common/ 目录)
存放第三方服务和基础设施的配置文件：
- `mysql.cnf` - MySQL 数据库配置
- `redis.conf` - Redis 缓存配置  
- `otel-collector-config.yaml` - 链路追踪配置
- `prometheus.yml` - 监控配置
- `sentinel.yaml` - 限流配置
- `Caddyfile` - Web 服务器配置
- `docker-compose.yml` - 多实例部署配置

## 💡 使用说明

### **应用配置**
```bash
# 开发环境
go run cmd/main.go -config=conf/config.yaml

# Docker 环境
# 自动使用 conf/config_docker.yaml
```

### **基础设施配置**
这些配置文件由 Docker Compose 和相应的服务自动加载：

```yaml
# docker-compose.yml 中的引用
volumes:
  - ./conf/common/mysql.cnf:/etc/mysql/conf.d/custom.cnf:ro
  - ./conf/common/redis.conf:/etc/redis/redis.conf:ro
  - ./conf/common/otel-collector-config.yaml:/etc/otelcol-contrib/config.yaml
```

## 🔧 配置修改

### **修改应用配置**
直接编辑 `conf/` 目录下的配置文件：
```bash
# 修改数据库连接
vim conf/config.yaml

# 修改Docker环境配置  
vim conf/config_docker.yaml
```

### **修改基础设施配置**
编辑 `conf/common/` 目录下的对应配置文件：
```bash
# 修改MySQL配置
vim conf/common/mysql.cnf

# 修改Redis配置
vim conf/common/redis.conf

# 重启服务生效
make docker-compose-restart
```

## 📝 注意事项

1. **权限设置**: 基础设施配置文件建议设为只读权限 `:ro`
2. **环境隔离**: 不同环境使用不同的应用配置文件
3. **版本控制**: 所有配置文件都应纳入版本控制
