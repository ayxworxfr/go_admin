# 模块结构与依赖装配

[SKILL.md](../SKILL.md) §2 的细则。

## 1. 目录结构全貌

```
internal/
├── platform/                          # 横切基础设施，不含业务语义
│   ├── app/                           # Hertz 生命周期、SetupRoutes 挂载
│   ├── router/                        # AutoRouterRegister（按方法名反射推断路由）
│   ├── middleware/                    # CORS / JWT / 日志 / 限流 / 全局错误处理
│   ├── config/                        # 配置加载
│   ├── db/                            # xorm.Engine 构造、表结构同步、SQL 日志钩子
│   └── cron/                          # 定时任务
│
├── bootstrap/
│   └── container.go                   # 唯一的显式依赖装配点
│
└── modules/                           # 按业务模块（限界上下文）划分
    ├── user/
    │   ├── model/                     # User
    │   ├── dto/                       # CreateUserRequest / UpdateUserRequest / ...
    │   ├── service/
    │   │   ├── user_service.go        # repo 字段直接 pkgrepo.NewRepository[model.User](processor) 构造
    │   │   └── user_finder.go         # 对外窄接口：UserFinder
    │   └── handler/
    │
    ├── iam/                           # Identity & Access Management
    │   ├── model/                     # Role / Permission / UserRole / RolePermission
    │   ├── dto/
    │   ├── cache/                     # PermissionCache 接口 + InMemoryCache
    │   ├── tokenstore/                # TokenStore 接口 + InMemoryTokenStore
    │   ├── service/
    │   │   ├── repositories.go            # unexported repositories + newRepositories
    │   │   ├── role_service.go            # Role CRUD
    │   │   ├── permission_service.go      # Permission 元数据 CRUD（变窄）
    │   │   ├── user_role_service.go       # 用户-角色分配
    │   │   ├── auth_service.go            # 登录 / 刷新令牌 / 登出
    │   │   └── permission_checker.go      # HasPermission 鉴权热路径，带缓存
    │   └── handler/
    │
    └── systemsetting/                 # 独立限界上下文，与 iam 无耦合
        ├── model/ dto/ handler/
        └── service/system_setting_service.go  # repo 字段同样直接 pkgrepo.NewRepository 构造
```

`pkg/` 保持技术基础设施定位，不按模块拆：`jwtauth`（JWT 编解码）、`crypter`（密码哈希/加密策略）、`logger`、`repository`（泛型 ORM 引擎）、`cron`、`httpclient`、`utils`、`context`（Hertz 请求上下文与统一响应结构）、`apiparam`（分页等跨模块共享的请求 DTO 片段）。这些包本身不含业务语义，任何模块都可以直接依赖，不需要经过窄接口转发。

## 2. 仓储封装

### 必须

| 场景 | 写法 | 范本 |
|---|---|---|
| 单实体、无自定义查询 | `Service` 内 unexported `repo`，`NewService` 里 `pkgrepo.NewRepository[T](processor)` | `user`、`systemsetting` |
| 同模块多 Service 共用多仓储 | 同包 unexported `repositories` + `newRepositories` | `iam/service/repositories.go` |

```go
// 默认：单实体
type Service struct {
	repo   pkgrepo.Repository[model.User]
	hasher crypter.PasswordHasher
}

func NewService(processor pkgrepo.ORMProcessor, hasher crypter.PasswordHasher) *Service {
	return &Service{
		repo:   pkgrepo.NewRepository[model.User](processor),
		hasher: hasher,
	}
}
```

```go
// 同包多仓储共享
type repositories struct {
	role           pkgrepo.Repository[model.Role]
	permission     pkgrepo.Repository[model.Permission]
	userRole       pkgrepo.Repository[model.UserRole]
	rolePermission pkgrepo.Repository[model.RolePermission]
}

func newRepositories(processor pkgrepo.ORMProcessor) *repositories {
	return &repositories{
		role:           pkgrepo.NewRepository[model.Role](processor),
		permission:     pkgrepo.NewRepository[model.Permission](processor),
		userRole:       pkgrepo.NewRepository[model.UserRole](processor),
		rolePermission: pkgrepo.NewRepository[model.RolePermission](processor),
	}
}
```

### 禁止

- 为纯包装仓储新建 `service/internal/repository/`
- Handler 持有或导入仓储类型
- 导出 `Repo` / `NewXxxRepository` 供 handler 或其它模块使用

### 例外（允许拆 `service/internal/...`）

同时满足才允许：

1. 仓储含大量自定义查询，无法用 `QueryBuilder` / 通用 `Repository[T]` 表达
2. 需要独立于业务 Service 的仓储层单测
3. 体量已使 `service/` 目录难以浏览

拆包后路径须含 `internal`，且仅本模块 `service` 树可导入。

## 3. 模块边界判断标准

新功能该加进现有模块还是新建模块，按以下顺序判断：

1. **是否引入新的持久化实体**：没有新表/新 model，只是给已有实体加字段或加查询方式，一律加进现有模块（不新建）。
2. **是否与现有实体强耦合**：新实体的生命周期是否完全依附于某个现有实体（例如"用户的头像变更历史"依附于 `User`）？强耦合就加进现有模块；能独立存在、独立测试、未来可能被拆成独立服务，才新建模块。
3. **对外依赖是否单向**：新模块画出依赖图后必须能排出拓扑顺序（无环）。本项目当前的图：

```
user  ←── iam            （iam 登录需要查用户，通过 user 导出的 UserFinder，不直连 user 仓储）
user  ←── systemsetting   （展示 create_by 用户名，同上）
iam 与 systemsetting 之间无依赖
```

若新模块需要同时依赖 `iam` 和 `systemsetting`，或者 `iam`/`systemsetting` 需要反向依赖新模块，先重新检查边界划分，而不是接受一个环——环意味着这两个"模块"实际上是一个模块被错误拆开了，应该合并或者重新切割聚合边界。

## 4. `Container`：显式依赖装配

```go
// internal/bootstrap/container.go
package bootstrap

type Container struct {
	Engine *xorm.Engine

	User          *userservice.Service
	Role          *iamservice.RoleService
	Permission    *iamservice.PermissionService
	UserRole      *iamservice.UserRoleService
	Checker       *iamservice.PermissionChecker
	Auth          *iamservice.AuthService
	SystemSetting *ssservice.Service

	JWT        *jwtauth.JWT
	TokenStore iamtokenstore.TokenStore
}

func NewContainer(engine *xorm.Engine, hasher crypter.PasswordHasher, jwt *jwtauth.JWT) *Container {
	processor := repository.NewXormProcessor(engine)

	userSvc := userservice.NewService(processor, hasher)

	roleSvc := iamservice.NewRoleService(processor)
	permSvc := iamservice.NewPermissionService(processor)
	userRoleSvc := iamservice.NewUserRoleService(processor, roleSvc, userSvc)

	permCache := iamcache.NewInMemoryCache(permissionCacheTTL)
	checker := iamservice.NewPermissionChecker(userRoleSvc, roleSvc, permCache)

	tokenStore := iamtokenstore.NewInMemoryTokenStore()
	authSvc := iamservice.NewAuthService(userSvc, userRoleSvc, tokenStore, jwt)

	ssSvc := ssservice.NewService(processor, userSvc)

	return &Container{ /* ... */ }
}

func (c *Container) Models() []any {
	return []any{
		new(usermodel.User),
		new(iammodel.Role), new(iammodel.Permission),
		new(iammodel.UserRole), new(iammodel.RolePermission),
		new(ssmodel.SystemSetting),
	}
}
```

装配顺序：**被依赖者先构造**。新增 Service 时确认依赖关系后插入正确位置，禁止随手追加到函数末尾。

`Container` 构造完成后，`cmd/main.go`：用 `Container` 字段构造各模块 Handler → 交给 `App.SetupRoutes`；`JWTAuthMiddleware` 同样构造注入 `checker`/`tokenStore`。

## 5. 全局状态例外：`pkg/jwtauth.Instance`

| 允许 | 禁止 |
|---|---|
| `pkg/jwtauth.Instance` 仅供 `pkg/context.Context.GetUserID()` 等无法注入的工具方法 | `internal/modules/*` 的 Service/Handler 读取任何包级 Instance |
| — | 新增第二个「全局方便访问」先例 |

新场景默认走构造注入。
