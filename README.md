# Go Admin Scaffold - 企业级后台管理系统脚手架

Go Admin Scaffold 是基于 **Go + Hertz + Xorm** 构建的现代化后台管理系统脚手架，采用**组合优先（Composition First）+ 模块化 DDD** 的架构设计，提供完整的 RBAC 权限管理、系统设置等基础功能，助力开发者快速构建企业级后台应用。

## 项目特点

✨ **开箱即用**: 集成用户管理、权限控制、系统设置等企业级基础功能  
⚡ **高性能**: 基于 Hertz 高性能HTTP框架，支持高并发场景  
🏗️ **模块化 DDD**: 按业务限界上下文（`user`/`iam`/`systemsetting`）划分目录，用 Go `internal` 包可见性在编译期强制模块边界，而非仅靠约定  
🔐 **安全可靠**: JWT + RBAC 权限体系，Argon2id 密码哈希，登出即时撤销令牌（JTI 黑名单）  
🧩 **组合优先**: 无全局单例，依赖通过 `internal/bootstrap.Container` 显式装配注入，可变行为（密码算法/权限缓存/令牌存储）全部策略化，可独立替换  
🚀 **易扩展**: 标准化的模块模板，新增业务模块只需遵循固定的 model/dto/service/handler 结构  
📊 **可观测**: 集成OpenTelemetry，完整的链路追踪和监控  
🐳 **容器化**: 完整的Docker部署方案，支持一键部署


## 技术架构

### 核心技术栈

| 层面       | 技术选型                | 核心优势                                                                 |
|------------|-------------------------|--------------------------------------------------------------------------|
| **后端**   | Go 1.24+                | 高性能、简洁、并发友好的现代编程语言                                      |
|            | Hertz                   | 字节跳动开源高性能 HTTP 框架，基于 Netpoll 网络模型，支持高并发场景       |
|            | Xorm                    | 轻量 ORM 框架，简化数据库操作，支持事务与复杂查询                         |
|            | MySQL 8.0+              | 成熟稳定的关系型数据库，支持复杂查询和事务                               |
|            | JWT + RBAC              | 基于令牌的身份认证，结合细粒度角色权限控制，保障 API 访问安全             |
|            | Argon2id                | OWASP 推荐的密码哈希算法，per-user 随机 salt + 可调工作因子，替代早期版本的 HMAC-SHA384 方案 |
|            | 组合优先 + 模块化 DDD    | 按限界上下文划分目录，依赖通过构造函数显式注入，无全局单例，提升可维护性与可测试性 |
| **部署**   | Docker                  | 容器化部署支持多环境一致性                                               |
| **监控**   | OpenTelemetry           | 现代化可观测性框架，统一追踪、指标和日志                                  |
|            | Prometheus + Jaeger     | 实时监控系统性能指标，分布式追踪定位请求链路瓶颈，保障系统稳定性          |


### 架构设计

架构设计遵循两条纪律：**组合优先**（依赖显式构造注入，可变行为策略化，接口只暴露消费方需要的方法）与**模块化 DDD**（`internal/modules/` 下每个目录是一个独立的业务限界上下文，模块间只能通过对方导出的窄接口互相依赖）。完整设计理念与落地细则见 [`.cursor/skills/developing-go-admin-modules/`](.cursor/skills/developing-go-admin-modules/SKILL.md)（同时也是给 AI 辅助开发加载的项目专属知识库）。

```
┌─────────────────────────────────────────────────────────────────────┐
│ internal/platform/          横切基础设施：路由/中间件/配置/DB/定时任务 │
├─────────────────────────────────────────────────────────────────────┤
│ internal/bootstrap/         Container：唯一的显式依赖装配点           │
├─────────────────────────────────────────────────────────────────────┤
│ internal/modules/<mod>/     业务模块：每个模块自带完整的四层结构       │
│  ├─ handler/                HTTP 接口，请求响应处理                  │
│  ├─ service/                业务逻辑 + 仓储构造（repo 字段 unexported）│
│  ├─ model/                  数据库实体（纯数据结构）                  │
│  └─ dto/                    请求参数与响应视图对象                    │
├─────────────────────────────────────────────────────────────────────┤
│ pkg/                        跨模块复用的技术基础设施（无业务语义）     │
└─────────────────────────────────────────────────────────────────────┘
```

**目录结构**：
```
internal/
├── platform/               # 横切基础设施
│   ├── app/                # Hertz 启动、生命周期、路由挂载
│   ├── router/              # 路由注册器（按方法名反射推断路由）
│   ├── middleware/          # CORS/JWT/日志/限流/全局错误处理
│   ├── config/              # 配置加载
│   ├── db/                  # ORM 引擎构造、表结构同步、SQL 日志钩子
│   └── cron/                # 定时任务
├── bootstrap/
│   └── container.go          # 显式依赖装配，替代全局单例
└── modules/                 # 按业务限界上下文划分
    ├── user/                 # 用户生命周期管理
    ├── iam/                  # 角色/权限/用户角色分配/登录鉴权
    └── systemsetting/        # 系统配置

pkg/                       # 跨模块技术基础设施（无业务语义）
├── jwtauth/                 # JWT 编解码
├── crypter/                  # 密码哈希策略（Argon2id）
├── repository/               # 泛型仓储接口 + Xorm 实现
├── logger/                   # 结构化日志
└── utils/                    # 工具函数
```


## 核心功能模块

### 1. 用户权限管理

#### 用户管理
- **用户生命周期**：用户创建、状态管理（启用/禁用）、信息维护、批量删除
- **个人信息**：支持头像 URL、基本信息编辑、密码修改
- **登录安全**：密码强度校验（长度约束）、Argon2id 哈希存储、登出即时撤销当前令牌

#### 权限管理
- **RBAC模型**：用户-角色-权限三级授权体系，权限支持父子级联（菜单/按钮/接口三种类型）
- **权限控制**：中间件按请求 method+path 做接口级鉴权，支持路径通配符
- **动态权限**：角色/权限变更后主动失效对应用户的权限缓存，无需重启

#### 角色管理
- **角色定义**：灵活的角色创建和权限分配
- **用户授权**：支持用户角色分配（创建/更新用户时一并指定）

### 2. 系统管理

#### 系统设置
- **参数配置**：系统级参数配置管理，按 `category` 分类
- **配置分类**：支持不同类型的配置项（文本、数字、布尔值、JSON），写入时按类型校验取值合法性
- **核心配置保护**：内置核心配置项（如系统名称/版本）禁止被删除

> 📍 **Roadmap**：细粒度数据权限（按数据范围过滤查询结果）当前只是设计草案，尚无落地实现，如需该能力请单独排期设计，不要假设已存在。


## 项目优势

### 🚀 开发效率
- **标准化结构**：模块化 DDD 目录结构 + 固定的 model/dto/service/handler 模板，新增业务模块有章可循（见 [`.cursor/skills/developing-go-admin-modules/`](.cursor/skills/developing-go-admin-modules/SKILL.md)）
- **编译期约束**：Service 的 `repo` 字段 unexported，handler 另一包天然拿不到仓储实例；复杂仓储才考虑再拆 `internal` 子包
- **丰富中间件**：内置认证、日志、错误处理、限流等常用中间件，装饰器式顺序叠加
- **类型安全**：完整的参数验证（`vd` tag）和类型定义，减少运行时错误

### ⚡ 高性能
- **Hertz框架**：基于Netpoll的高性能HTTP框架
- **连接池**：数据库连接池参数可配置（`max_idle_conns`/`max_open_conns`/`conn_max_lifetime`）
- **权限缓存**：进程内 TTL 缓存降低鉴权热路径的数据库查询频率，接口已预留未来接入 Redis 的替换点
- **并发安全**：缓存/令牌存储等有状态组件均用锁保护并发访问

### 🔐 企业级安全
- **JWT认证**：无状态认证，支持 Access/Refresh Token 双令牌机制
- **令牌撤销**：登出后基于 JWT `jti` 的黑名单立即使已签发令牌失效，而非等待自然过期
- **密码哈希**：Argon2id（OWASP 推荐参数），per-user 随机 salt，自描述哈希串便于未来平滑调整参数
- **RBAC权限**：接口级权限控制，支持路径通配符与权限父子级联

### 🛠️ 运维友好
- **健康检查**：`/health`、`/metrics`、`/api/hello` 探活端点
- **优雅关闭**：监听 SIGINT/SIGTERM，超时窗口内等待在途请求完成
- **多环境配置**：`conf/config.yaml`/`config_docker.yaml`/`config_test.yaml` 按环境区分；限流规则（Sentinel）额外支持文件监控热更新
- **监控集成**：集成 Prometheus 指标和 Jaeger 分布式追踪（OpenTelemetry）


## 快速开始

### 环境要求
```bash
Go 1.24+
MySQL 8.0+  
Git
```

### 1. 克隆项目
```bash
git clone https://github.com/ayxworxfr/go_admin.git
cd go_admin
```

### 2. 初始化数据库
```bash
# 创建数据库
mysql -u root -p -e "CREATE DATABASE go_admin DEFAULT CHARSET utf8mb4"

# 导入基础表结构
mysql -u root -p go_admin < mysql/schema.sql

# 导入初始化数据
mysql -u root -p go_admin < mysql/init_data.sql
```

### 3. 配置项目
```bash
# 复制配置文件
cp conf/config.yaml conf/config_local.yaml

# 编辑配置文件，修改数据库连接信息
vim conf/config_local.yaml
```

### 4. 启动项目
```bash
# 安装依赖
go mod tidy

# 启动服务
go run cmd/main.go
```

访问地址：http://localhost:8888

默认管理员账号（密码为 Argon2id 哈希，见 `mysql/init_data.sql`）：
- 用户名：admin
- 密码：admin123

### 5. API测试
```bash
# 登录获取token
curl -X POST http://localhost:8888/api/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}'

# 使用token访问用户列表
curl -X GET http://localhost:8888/api/protected/user/list \
  -H "Authorization: Bearer YOUR_TOKEN"
```

## Docker 部署

### 使用 Docker Compose
```bash
# 方式一：使用 Makefile (推荐)
make docker-compose-up      # 启动所有服务
make docker-compose-status  # 查看服务状态
make docker-compose-logs    # 查看日志
make docker-compose-down    # 停止服务

# 方式二：直接使用 docker-compose
docker-compose up -d        # 启动所有服务
docker-compose ps           # 查看服务状态
docker-compose down         # 停止服务
```

服务地址：
- 🌐 应用: http://localhost:8888
- 📊 Jaeger: http://localhost:16686
- 💾 MySQL: localhost:3306 (go_user/go_user123)
- 🔴 Redis: localhost:6379

### 单独使用 Docker
```bash
# 构建镜像
docker build -t go_admin .

# 运行容器
docker run -d \
  --name go-admin \
  -p 8888:8888 \
  -e DB_HOST=your-mysql-host \
  -e DB_USER=your-mysql-user \
  -e DB_PASSWORD=your-mysql-password \
  go_admin
```

## 开发指南

- **架构规则与新增模块的标准流程**：[`.cursor/skills/developing-go-admin-modules/SKILL.md`](.cursor/skills/developing-go-admin-modules/SKILL.md) —— 这是本项目架构决策的唯一权威来源，人工开发或加载 AI 辅助编码时都应先读这份文档；下面的目录结构只是速览。
- **通用开发流程/编码标准**：[开发指南](development-guide.md)

### 目录结构说明
```
├── cmd/                     # 程序入口（main.go：初始化 Container、挂载路由、启停）
├── internal/                # 私有代码
│   ├── platform/            # 横切基础设施：app/router/middleware/config/db/cron
│   ├── bootstrap/           # Container：显式依赖装配
│   └── modules/             # 业务模块（限界上下文）
│       ├── user/            # 用户生命周期
│       ├── iam/             # 角色/权限/登录鉴权
│       └── systemsetting/   # 系统配置
│       每个模块内部固定为 model/ dto/ service/ handler/
├── pkg/                     # 跨模块技术基础设施（jwtauth/crypter/repository/logger/...）
├── mysql/                   # 数据库脚本（schema.sql + init_data.sql）
└── conf/                    # 配置文件
```

### 代码生成

> 📍 **Roadmap**：CRUD 代码生成工具尚未实现，当前新增模块请按 [`.cursor/skills/developing-go-admin-modules/references/adding-a-module-checklist.md`](.cursor/skills/developing-go-admin-modules/references/adding-a-module-checklist.md) 的步骤手动搭建。

## 贡献指南

我们欢迎所有形式的贡献，包括但不限于：
- 🐛 Bug 修复
- ✨ 新功能开发  
- 📚 文档改进
- 🎨 代码优化

### 贡献流程
1. Fork 本项目
2. 创建功能分支 (`git checkout -b feature/AmazingFeature`)
3. 提交更改 (`git commit -m 'Add some AmazingFeature'`)
4. 推送到分支 (`git push origin feature/AmazingFeature`)
5. 创建 Pull Request

## 许可证

本项目基于 [MIT 许可证](LICENSE) 开源，可自由用于商业和个人项目。

## 支持与反馈

- 📖 [开发文档](development-guide.md)
- 🐛 [问题反馈](https://github.com/ayxworxfr/go_admin/issues)
- 💬 [讨论区](https://github.com/ayxworxfr/go_admin/discussions)
- ⭐ 如果这个项目对你有帮助，请给个 Star！

---

**Made with ❤️ by Go Community**