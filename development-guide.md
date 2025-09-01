# Go Admin 脚手架开发指南

## 项目概述

本项目是基于 Go + Hertz + Xorm 的企业级后台管理系统脚手架，采用 Clean Architecture 架构设计，提供完整的用户权限管理、部门管理等基础功能，可快速扩展业务模块。

## 技术栈

### 后端框架
- **Go 1.20+**: 主要编程语言
- **Hertz**: 字节跳动开源的高性能HTTP框架
- **Xorm**: 轻量级ORM框架
- **MySQL**: 数据库

### 架构设计
- **Clean Architecture**: 分层架构，业务逻辑与技术实现解耦
- **RBAC**: 基于角色的访问控制
- **JWT**: 无状态身份认证
- **OpenTelemetry**: 分布式追踪和监控

## 项目结构

```
internal/
├── handler/       # HTTP处理器（控制器层）
│   └── auth/      # 认证相关handler
├── service/       # 业务逻辑层
├── domain/        # 领域模型
│   ├── models/    # 数据库实体
│   ├── params/    # 请求参数
│   └── vo/        # 响应视图对象
├── dao/           # 数据访问层
├── middleware/    # HTTP中间件
├── config/        # 配置管理
└── app/           # 应用启动和路由

pkg/               # 通用工具包
├── jwtauth/       # JWT认证
├── repository/    # 泛型仓储接口
├── logger/        # 结构化日志
└── utils/         # 工具函数
```

## 快速开始

### 1. 环境准备
- Go 1.20+
- MySQL 8.0+
- Git

### 2. 初始化数据库
```bash
# 创建数据库
mysql -u root -p -e "CREATE DATABASE go_admin DEFAULT CHARSET utf8mb4"

# 导入基础表结构
mysql -u root -p go_admin < mysql/schema.sql

# 导入初始化数据
mysql -u root -p go_admin < mysql/init_data.sql
```

### 3. 配置文件
复制并修改配置文件：
```bash
cp conf/config.yaml conf/config_local.yaml
```

修改数据库连接信息等配置。

### 4. 启动项目
```bash
# 安装依赖
go mod tidy

# 启动服务
go run cmd/main.go
```

默认访问地址: http://localhost:8080

默认管理员账号:
- 用户名: admin
- 密码: admin123

## 开发新模块

### 1. 数据模型设计
在 `internal/domain/models/` 目录下创建新的实体模型：

```go
type YourEntity struct {
    BaseModel
    Name        string `xorm:"varchar(100) not null" json:"name"`
    Description string `xorm:"varchar(500)" json:"description"`
    Status      int    `xorm:"tinyint default(1)" json:"status"`
}
```

### 2. 创建Handler
在 `internal/handler/` 目录下创建业务handler：

```go
type IYourHandler interface {
    List(c *context.Context) *context.Response
    Create(c *context.Context) *context.Response
    Update(c *context.Context) *context.Response
    Delete(c *context.Context) *context.Response
}

type YourHandler struct{}

func (h *YourHandler) List(c *context.Context) *context.Response {
    // 实现业务逻辑
    return context.Success(data)
}
```

### 3. 创建Service
在 `internal/service/` 目录下创建业务service：

```go
type IYourService interface {
    GetList(params YourListParams) ([]models.YourEntity, int64, error)
    Create(entity *models.YourEntity) error
    Update(id int64, entity *models.YourEntity) error
    Delete(id int64) error
}

type YourService struct{}

func (s *YourService) GetList(params YourListParams) ([]models.YourEntity, int64, error) {
    // 实现业务逻辑
}
```

### 4. 注册Handler
在 `internal/handler/export.go` 中注册新的handler：

```go
var (
    YourHandlerInstance IYourHandler
)

func init() {
    createAndRegister(&YourHandlerInstance, &YourHandler{})
}
```

### 5. 数据库迁移
在 `mysql/` 目录下创建SQL文件，并通过迁移工具应用到数据库。

## 架构特点

### 1. 分层架构
- **Handler层**: 处理HTTP请求，参数校验，返回响应
- **Service层**: 业务逻辑处理，事务管理
- **DAO层**: 数据访问，数据库操作封装
- **Domain层**: 业务实体和规则定义

### 2. 依赖注入
- 通过工厂模式管理依赖关系
- 接口驱动，便于单元测试

### 3. 中间件支持
- CORS处理
- JWT认证
- 全局错误处理
- 请求日志记录
- 限流和熔断

### 4. 配置管理
- 支持多环境配置
- 热重载配置
- 敏感信息加密

## 最佳实践

### 1. 错误处理
使用统一的错误响应格式：
```go
return context.Error(code, message)
return context.Success(data)
```

### 2. 参数验证
使用结构体标签进行参数验证：
```go
type CreateRequest struct {
    Name string `json:"name" validate:"required,min=2,max=100"`
}
```

### 3. 权限控制
为新的API添加权限校验：
```go
// 在handler方法上添加权限注解
func (h *YourHandler) Create(c *context.Context) *context.Response {
    // 权限检查在中间件中完成
}
```

### 4. 代码规范
- 遵循Go语言规范
- 使用有意义的变量名
- 添加必要的注释
- 编写单元测试

## 部署说明

### Docker部署
```bash
# 构建镜像
docker build -t go-admin .

# 启动容器
docker-compose up -d
```

### 生产环境
1. 修改生产环境配置
2. 设置环境变量
3. 配置反向代理
4. 设置监控和日志收集

## 贡献指南

1. Fork项目
2. 创建功能分支
3. 提交代码
4. 创建Pull Request

## 许可证

MIT License
