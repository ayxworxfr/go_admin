# Go Admin Scaffold - 企业级后台管理系统脚手架

Go Admin Scaffold 是基于 **Go + Hertz + Xorm** 构建的现代化后台管理系统脚手架，采用 Clean Architecture 架构设计，提供完整的 RBAC 权限管理、部门管理等基础功能，助力开发者快速构建企业级后台应用。

## 项目特点

✨ **开箱即用**: 集成用户管理、权限控制、部门管理等企业级基础功能  
⚡ **高性能**: 基于 Hertz 高性能HTTP框架，支持高并发场景  
🏗️ **Clean架构**: 分层设计，业务逻辑与技术实现完全解耦  
🔐 **安全可靠**: JWT + RBAC 权限体系，数据权限细粒度控制  
🚀 **易扩展**: 标准化的代码结构，快速开发新业务模块  
📊 **可观测**: 集成OpenTelemetry，完整的链路追踪和监控  
🐳 **容器化**: 完整的Docker部署方案，支持一键部署


## 技术架构

### 核心技术栈

| 层面       | 技术选型                | 核心优势                                                                 |
|------------|-------------------------|--------------------------------------------------------------------------|
| **后端**   | Go 1.20+                | 高性能、简洁、并发友好的现代编程语言                                      |
|            | Hertz                   | 字节跳动开源高性能 HTTP 框架，基于 Netpoll 网络模型，支持高并发场景       |
|            | Xorm                    | 轻量 ORM 框架，简化数据库操作，支持事务与复杂查询                         |
|            | MySQL 8.0+              | 成熟稳定的关系型数据库，支持复杂查询和事务                               |
|            | JWT + RBAC              | 基于令牌的身份认证，结合细粒度角色权限控制，保障 API 访问安全             |
|            | Clean Architecture      | 分层架构设计，实现业务逻辑与技术细节解耦，提升代码可维护性与扩展性       |
| **部署**   | Docker                  | 容器化部署支持多环境一致性                                               |
| **监控**   | OpenTelemetry           | 现代化可观测性框架，统一追踪、指标和日志                                  |
|            | Prometheus + Jaeger     | 实时监控系统性能指标，分布式追踪定位请求链路瓶颈，保障系统稳定性          |


### 架构设计

遵循 **Clean Architecture** 原则，系统分为五层架构，严格控制依赖方向，确保业务逻辑独立于技术实现：

```
┌─────────────────────────────────────────────────────────────┐
│ 表现层（Handler）          接收HTTP请求，参数校验，返回响应   │
├─────────────────────────────────────────────────────────────┤
│ 应用层（Service）          编排业务流程，协调领域对象交互     │
├─────────────────────────────────────────────────────────────┤
│ 领域层（Domain）           核心业务逻辑，实体与规则定义       │
│  ├─ models：数据实体       │
│  ├─ params：请求/响应参数  │
│  └─ vo：视图对象           │
├─────────────────────────────────────────────────────────────┤
│ 基础设施层（DAO）          数据库交互，外部服务调用           │
├─────────────────────────────────────────────────────────────┤
│ 跨域层（Pkg）              通用工具，如日志、加密、缓存       │
└─────────────────────────────────────────────────────────────┘
```

**目录结构**：
```
internal/
├── handler/       # HTTP处理器（控制器）
├── service/       # 业务逻辑层
├── domain/        # 领域模型与规则
│   ├── models/    # 数据库实体
│   ├── params/    # 请求参数
│   └── vo/        # 响应视图
├── dao/           # 数据访问层
├── middleware/    # HTTP中间件（认证、日志等）
├── config/        # 配置管理
└── app/           # 应用启动入口

pkg/               # 通用工具包
├── jwtauth/       # JWT认证
├── repository/    # 泛型仓储接口
├── logger/        # 结构化日志
└── utils/         # 工具函数
```


## 核心功能模块

### 1. 用户权限管理

#### 用户管理
- **用户生命周期**：用户创建、状态管理（启用/禁用）、信息维护
- **个人信息**：支持头像上传、基本信息编辑、密码修改
- **登录安全**：密码强度校验、登录记录、会话管理

#### 权限管理
- **RBAC模型**：用户-角色-权限三级授权体系
- **权限类型**：支持菜单权限、按钮权限、API接口权限
- **权限继承**：支持权限树形结构，子权限继承父权限
- **动态权限**：运行时动态加载用户权限，支持实时权限变更

#### 角色管理
- **角色定义**：灵活的角色创建和权限分配
- **角色继承**：支持角色间的权限继承关系
- **批量授权**：支持批量用户角色分配

### 2. 系统管理

#### 系统设置
- **参数配置**：系统级参数配置管理
- **配置分类**：支持不同类型的配置项（字符串、数字、布尔值、JSON）
- **配置权限**：区分公开配置和私有配置

#### 数据权限
- **权限范围控制**：支持细粒度的数据访问控制
- **字段级权限**：支持字段级别的数据访问控制
- **行级权限**：基于条件的数据行过滤

#### 日志审计
- **操作日志**：记录用户的所有操作行为
- **登录日志**：用户登录、退出记录
- **系统日志**：系统运行状态和错误日志


## 项目优势

### 🚀 开发效率
- **标准化结构**：遵循Clean Architecture，代码结构清晰，新人快速上手
- **代码生成**：提供完整的CRUD模板，快速生成业务代码
- **丰富中间件**：内置认证、日志、错误处理、限流等常用中间件
- **类型安全**：完整的参数验证和类型定义，减少运行时错误

### ⚡ 高性能
- **Hertz框架**：基于Netpoll的高性能HTTP框架，QPS可达10万+
- **连接池优化**：数据库连接池、Redis连接池优化配置
- **查询优化**：合理的数据库索引设计和查询优化
- **并发安全**：Goroutine安全的代码设计

### 🔐 企业级安全
- **JWT认证**：无状态认证，支持Token刷新机制
- **RBAC权限**：细粒度的权限控制，支持动态权限分配
- **数据权限**：基于组织架构的数据访问控制
- **安全审计**：完整的操作日志和审计追踪

### 🛠️ 运维友好
- **健康检查**：完整的健康检查端点
- **优雅关闭**：支持优雅关闭和重启
- **配置管理**：支持多环境配置和热重载
- **监控集成**：集成Prometheus指标和Jaeger追踪


## 快速开始

### 环境要求
```bash
Go 1.20+
MySQL 8.0+  
Git
```

### 1. 克隆项目
```bash
git clone https://github.com/your-username/go-admin-scaffold.git
cd go-admin-scaffold
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

访问地址：http://localhost:8080

默认管理员账号：
- 用户名：admin
- 密码：admin123

### 5. API测试
```bash
# 登录获取token
curl -X POST http://localhost:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}'

# 使用token访问用户列表
curl -X GET http://localhost:8080/api/user \
  -H "Authorization: Bearer YOUR_TOKEN"
```

## Docker 部署

### 使用 Docker Compose
```bash
# 启动所有服务（包括MySQL）
docker-compose up -d

# 查看服务状态
docker-compose ps

# 停止服务
docker-compose down
```

### 单独使用 Docker
```bash
# 构建镜像
docker build -t go-admin-scaffold .

# 运行容器
docker run -d \
  --name go-admin \
  -p 8080:8080 \
  -e DB_HOST=your-mysql-host \
  -e DB_USER=your-mysql-user \
  -e DB_PASSWORD=your-mysql-password \
  go-admin-scaffold
```

## 开发指南

详细的开发文档请查看：[开发指南](development-guide.md)

### 目录结构说明
```
├── cmd/                    # 程序入口
├── internal/              # 私有代码
│   ├── app/               # 应用层（路由、中间件）
│   ├── handler/           # 控制器层
│   ├── service/           # 业务逻辑层
│   ├── dao/               # 数据访问层
│   ├── domain/            # 领域模型
│   ├── middleware/        # 中间件
│   └── config/            # 配置
├── pkg/                   # 公共代码库
├── mysql/                 # 数据库脚本
├── conf/                  # 配置文件
└── frontend/              # 前端代码（可选）
```

### 代码生成
```bash
# 生成CRUD代码（开发中）
go run tools/generator.go -model=User -table=user
```

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
- 🐛 [问题反馈](https://github.com/your-username/go-admin-scaffold/issues)
- 💬 [讨论区](https://github.com/your-username/go-admin-scaffold/discussions)
- ⭐ 如果这个项目对你有帮助，请给个 Star！

---

**Made with ❤️ by Go Community**