# go_admin 重构设计文档：组合优先 + 模块化 DDD

- 状态：已落地（历史设计草案，部分细节以代码为准）
- 关联：本文档承接《架构评审》结论（分层脚手架 vs README 声称的 Clean Architecture 的差距）
- 范围：`internal/` 目录结构 + 依赖关系；`pkg/` 保持不变（见 Non-Goals）
- **落地偏差（以代码与 skill 为准，勿按本文旧表述实现）**：
  1. **仓储封装**：文中多处写的 `service/internal/repository/` 未采用。当前默认是 `service` 包内 unexported 的 `repo` 字段 / `newRepositories`，靠包可见性挡 handler；权威说明见 [`.cursor/skills/developing-go-admin-modules/references/module-structure.md`](../.cursor/skills/developing-go-admin-modules/references/module-structure.md) §2。
  2. **密码迁移**：文中 P5 的懒迁移 + `Sha384LegacyHasher` 未做；按"不需要向前兼容"直接切 Argon2id，并更新了 `mysql/init_data.sql` 种子哈希。

---

## 0. 为什么现在重构

现状问题（均有文件证据，详见第 3 节逐条设计）：

| 问题 | 证据 |
|---|---|
| Handler 跳过 Service 直连 DAO | `internal/handler/auth/user_handler.go:33,56,67,79,108`、`system_setting_handler.go:50,67,83`、`permission_handler.go:81` |
| `PermissionService` 承担 Role/Permission/UserRole/Cache 四类职责，960 行 | `internal/service/permission_service.go` |
| 全局单例（`dao.*Repo`、`service.*Instance`、`jwtauth.Instance`、`crypter.Instance`）取代依赖注入 | `internal/dao/dao.go`、`internal/service/init_service.go`、`pkg/jwtauth/jwtauth.go:24`、`pkg/crypter/crypter.go:10` |
| 密码用 HMAC-SHA384 + 硬编码 key，无法安全升级 | `pkg/crypter/sha384.go:10` |
| 权限缓存 `cacheExpiration` 字段声明但从未使用 | `permission_service.go:35,47` |
| 目录按技术层分（`handler/`、`service/`、`dao/`、`domain/{models,params,vo}/`），业务边界在代码里不可见 | 当前 `internal/` 顶层结构 |

这些问题的共同根因：**项目用了"面向对象"的语法（interface、struct），但没有用"组合优先"的设计纪律**——依赖通过全局变量隐式获取，而不是通过构造函数显式注入；模块边界通过目录名字隐式约定，而不是通过 Go 的可见性规则强制约束。

本文档的重构目标，直接对应你提出的两条设计理念：

1. **组合优先**：小接口、显式构造注入、策略/装饰器替代 if-else 分支和全局状态。
2. **模块化 DDD**：目录结构从"按技术层"改为"按业务模块"，让限界上下文在代码物理结构上可见。

---

## 1. 设计理念对齐

### 1.1 Go OOP 公式在本项目的落地检查

| 公式 | 本项目现状 | 结论 |
|---|---|---|
| 封装 = struct + 小写字段 + 包边界 | `dao.UserRepo` 等仓储是**包级导出变量**，任何包都能直连 | ❌ 封装形同虚设 |
| 多态 = interface 隐式满足 | `ILoginHandler`/`IUserHandler`/`Repository[T]` 已经在用 | ✅ 用得对 |
| 复用 = 组合而非继承 | Service 内部靠字段持有依赖（`PermissionService.roleRepo` 等），方向对，但依赖来源是全局变量而非注入 | ⚠️ 一半对 |
| 抽象 = 小接口（1~3 方法） | `Repository[T]` 接口有 17 个方法，`ORMProcessor` 更多 | ⚠️ 偏大，但这是通用仓储框架，本次不动（见 Non-Goals） |
| 构造 = `NewXxx()` 工厂函数 | 有 `NewAuthService`/`NewPermissionService`，但构造完之后又被塞进全局变量 | ⚠️ 构造对了，装配错了 |

**核心结论：本项目缺的不是"接口"，缺的是"把接口的实现通过构造函数传进去，而不是让调用方去全局变量里找"。** 这正是依赖倒置（DIP）在 Go 里最朴素的样子。

### 1.2 SOLID 违规点与本次修复目标

| 原则 | 违规现状 | 证据 | 目标 |
|---|---|---|---|
| **S** 单一职责 | `PermissionService` 同时管 Role CRUD、Permission CRUD、UserRole 分配、权限缓存 | `permission_service.go` 全文 | 拆成 4 个协作对象（见 3.1） |
| **O** 开闭原则 | 密码算法、权限缓存后端、Token 撤销方式都硬编码在实现里，换算法/换存储要改调用方代码 | `sha384.go`、`permission_service.go:893-923` | 抽接口，新增实现不改调用方（见 3.2/3.3） |
| **L** 里氏替换 | 未发现违规（Go 无继承，天然规避） | — | 保持 |
| **I** 接口隔离 | `ORMProcessor`/`Repository[T]` 偏大，但服务于通用框架，可接受 | `pkg/repository/interface.go` | 不动（Non-Goals） |
| **D** 依赖倒置 | 高层模块（Service/Handler）依赖具体全局变量，而非自己声明的小接口 | `dao.go`、`init_service.go` | 消费方定义接口 + 构造注入（见 2.3） |

### 1.3 目录结构：为什么从"按技术层"改成"按模块"

当前 `internal/` 结构：

```
internal/
├── handler/        ← 所有 Handler
├── service/        ← 所有 Service
├── dao/            ← 所有 Repo
└── domain/
    ├── models/     ← 所有 Model
    ├── params/     ← 所有请求参数
    └── vo/         ← 所有响应视图
```

这就是"风格 A"：按技术层分。后果已经在代码里体现——`user_handler.go` 里混着 User CRUD 和"当前用户权限路由"两类完全不同的关注点；改一个"用户管理"功能，要同时碰 `handler/auth/user_handler.go`、`service/auth_service.go`、`service/permission_service.go`、`domain/{models,params,vo}` 四个目录。

目标结构按业务模块（限界上下文）划分，一个文件夹 = 一个可以独立理解、独立测试、未来可以独立拆出去的业务单元。具体划分见第 2.2 节。

---

## 2. 目标架构

### 2.1 目标目录结构

```
internal/
├── platform/                       # 应用装配与横切基础设施（原 app/ + middleware/ + config/）
│   ├── app/                        # Hertz 启动、生命周期钩子
│   ├── router/                     # 路由注册器（AutoRegister，原样保留）
│   ├── middleware/                 # CORS/JWT/日志/限流/错误处理
│   └── config/                     # 配置加载
│
├── bootstrap/
│   └── container.go                # 显式依赖装配，替代全局 Instance（见 2.3）
│
└── modules/                        # 按业务模块划分（本次重构核心）
    ├── user/                       # 用户生命周期管理
    │   ├── handler/
    │   ├── service/
    │   │   └── internal/repository/  # 仅 user/service/... 可 import（Go internal 规则强制）
    │   ├── model/
    │   └── dto/                    # 原 params + vo 拆到模块内，一个类型一个文件
    │
    ├── iam/                        # Identity & Access Management：角色/权限/分配/登录会话
    │   ├── handler/                 # LoginHandler / RoleHandler / PermissionHandler / UserRoleHandler
    │   ├── service/
    │   │   ├── role_service.go
    │   │   ├── permission_service.go     # 变窄：只做 Permission 元数据 CRUD
    │   │   ├── user_role_service.go      # 用户-角色分配
    │   │   ├── auth_service.go           # 登录 / 令牌刷新
    │   │   ├── permission_checker.go     # HasPermission，组合 repo + cache
    │   │   └── internal/repository/
    │   ├── cache/                  # PermissionCache 接口 + InMemory 实现（见 3.3）
    │   ├── model/
    │   └── dto/
    │
    └── systemsetting/               # 系统配置（与 iam/user 无耦合，独立限界上下文）
        ├── handler/ service/ model/ dto/
        └── service/internal/repository/
```

`pkg/` 保持不动：`jwtauth`、`logger`、`crypter`、`cron`、`httpclient`、`utils`、`repository`（泛型 ORM 引擎）继续作为跨模块可复用的通用基础设施——它们本来就不含业务语义，不需要按模块拆。

### 2.2 模块边界与依赖方向

| 模块 | 拥有的聚合/实体 | 对外暴露的最小接口（消费方视角） | 依赖 |
|---|---|---|---|
| `user` | `User` | `UserFinder`（`FindByID`/`FindByUsername`） | 无（只依赖 `pkg/repository`、`pkg/crypter`） |
| `iam` | `Role`、`Permission`、`UserRole`、`RolePermission` | `RoutePermissionResolver`（`GetUserPermissionPaths`）、`PermissionChecker`（`HasPermission`） | 依赖 `user.UserFinder`（登录时查用户），**不反向依赖** user 的内部实现 |
| `systemsetting` | `SystemSetting` | 无对外依赖，独立 | 依赖 `user.UserFinder`（展示 `create_by` 用户名） |

依赖方向图：

```
user  ←── iam            （iam 登录需要查用户，通过 user 导出的小接口，不直连 user 的 repository）
user  ←── systemsetting   （展示创建人信息，同上）
iam 与 systemsetting 之间无依赖
```

**关键设计动作：`user` 模块对外只导出一个 2 方法的接口，而不是整个 Repository。**

```go
// internal/modules/user/service/user_service.go
package service

// UserFinder 是消费方（iam、systemsetting）需要的最小契约。
// 按 Go 惯例，接口由消费方在自己的包里声明；这里放在 user 包导出，
// 是因为 user 是"提供方"，且只有一个真实实现，直接导出更省一次转发。
type UserFinder interface {
    FindByID(ctx context.Context, id uint64) (*model.User, error)
    FindByUsername(ctx context.Context, username string) (*model.User, error)
}
```

这样 `iam.AuthService` 依赖的是 2 个方法的 `UserFinder`，而不是 `user` 模块内部 17 个方法的 `Repository[User]`——即使 `user` 模块未来把仓储换成别的实现、加几十个内部方法，`iam` 都不受影响。这就是"接口由消费方定义，而非实现方"在跨模块场景下的具体应用。

### 2.3 依赖注入：用 Container 替代全局 Instance

不引入 wire/fx 等 DI 框架（见 Non-Goals），只用一个显式装配文件：

```go
// internal/bootstrap/container.go
package bootstrap

type Container struct {
    User          *userservice.Service
    Auth          *iamservice.AuthService
    Role          *iamservice.RoleService
    Permission    *iamservice.PermissionService
    UserRole      *iamservice.UserRoleService
    Checker       *iamservice.PermissionChecker
    SystemSetting *ssservice.Service
}

func NewContainer(engine *xorm.Engine, hasher crypter.PasswordHasher, jwt *jwtauth.JWT) *Container {
    processor := repository.NewXormProcessor(engine)

    userRepo := userrepo.New(processor)
    userSvc := userservice.New(userRepo, hasher)

    roleRepo := iamrepo.NewRoleRepo(processor)
    permRepo := iamrepo.NewPermissionRepo(processor)
    userRoleRepo := iamrepo.NewUserRoleRepo(processor)

    roleSvc := iamservice.NewRoleService(roleRepo)
    permSvc := iamservice.NewPermissionService(permRepo)
    userRoleSvc := iamservice.NewUserRoleService(userRoleRepo, roleRepo)
    cache := iamcache.NewInMemoryCache(1 * time.Hour)
    checker := iamservice.NewPermissionChecker(permRepo, userRoleRepo, cache)
    authSvc := iamservice.NewAuthService(userSvc /* UserFinder */, checker, jwt)

    ssRepo := ssrepo.New(processor)
    ssSvc := ssservice.New(ssRepo, userSvc /* UserFinder，用于展示创建人 */)

    return &Container{
        User: userSvc, Auth: authSvc, Role: roleSvc,
        Permission: permSvc, UserRole: userRoleSvc, Checker: checker,
        SystemSetting: ssSvc,
    }
}
```

`cmd/main.go` 只多做一件事：构造 `Container`，再把 `Container` 里的服务传给各模块的 `NewXxxHandler(svc)`，最后把 Handler 实例交给现有的 `router.AutoRegister.RegisterStruct(...)`（路由注册机制本次不改，见 Non-Goals）。JWT 中间件里对 `service.PermissionServiceInstance.HasPermission(...)` 的调用，改成中间件持有一个 `checker iam.PermissionChecker` 字段，由 `NewJWTMiddleware(checker)` 注入。

### 2.4 设计模式在具体问题上的落点

| 问题 | 应用的模式 | 落点 |
|---|---|---|
| 密码算法写死、无法安全升级 | 策略模式 + 组合 | `PasswordHasher` 接口，3.2 |
| 权限缓存字段声明未用、无 TTL、无法换后端 | 策略模式 + 组合 | `PermissionCache` 接口，3.3 |
| 登出无法让 token 失效 | 策略模式 | `TokenStore` 接口，3.4 |
| Handler 能绕过 Service 直连 DAO | 封装（Go `internal` 包可见性，编译期强制） | 3.5 |
| 中间件链（CORS/JWT/日志/限流） | 装饰器模式 | 已经是这么做的（`app.Use(...)` 顺序叠加），**保持不变，无需重构** |
| 路由按方法名反射推断 | 工厂/注册表模式 | 已经是这么做的（`AutoRegister`），本次不换（见 Non-Goals） |

---

## 3. 关键问题的具体设计

### 3.1 拆分 `PermissionService`

现状一个 960 行的 struct 做四件事。拆分方案：

| 新组件 | 职责 | 依赖 |
|---|---|---|
| `RoleService` | Role CRUD | `RoleRepository` |
| `PermissionService`（变窄） | Permission 元数据 CRUD | `PermissionRepository` |
| `UserRoleService` | 用户-角色分配（`AssignRoles`/`GetUserRoles`） | `UserRoleRepository`、`RoleRepository` |
| `PermissionChecker` | `HasPermission`（鉴权判断，带缓存） | `PermissionRepository`、`UserRoleRepository`、`PermissionCache` |

拆分依据：前三者是"管理台 CRUD"，变更频率低、无状态；`PermissionChecker` 是"每次请求都要走的鉴权热路径"，有状态（缓存）、对性能敏感——**这是两类完全不同的关注点，混在一个 struct 里正是当前 960 行的根因**。拆分后，`JWTMiddleware` 只依赖 `PermissionChecker` 一个 2 方法的接口，不再依赖整个 `PermissionService`。

### 3.2 密码哈希策略化（P0，需要数据迁移）

**现状问题**：`pkg/crypter/sha384.go` 用 HMAC-SHA384 + 硬编码 salt `"ServerName@2025"`，本质是"带密钥的哈希"而非密码哈希算法——没有 per-user salt、没有工作因子，可离线暴力破解；且硬编码密钥意味着**换算法本身就是一次破坏性变更**。

**目标接口**：

```go
package crypter

type PasswordHasher interface {
    Hash(plain string) (string, error)
    Verify(plain, hashed string) bool
    Algo() string // 写入 DB，用于识别用旧算法验证的记录
}
```

`Argon2Hasher`（新默认）与 `Sha384LegacyHasher`（现有算法，仅用于兼容旧数据）都实现该接口。

**⚠️ 这是唯一一处不能"直接切换"的改动**——现有用户密码全部是 SHA384 哈希，直接换算法会导致所有人无法登录。迁移方案（懒迁移，不需要停机脚本）：

1. `user` 表新增 `password_algo VARCHAR(20)`，历史数据回填为 `sha384-legacy`。
2. 登录校验时，按 `user.PasswordAlgo` 选择对应 `PasswordHasher.Verify`。
3. 若验证通过且 `PasswordAlgo != "argon2id"`：用新算法重新 `Hash`，更新 `password` 与 `password_algo`，本次登录静默完成迁移。
4. 新建用户 / 改密码，一律用新算法。

这一步是**行为改**（P0 安全修复），必须单独一个 PR，且必须有迁移前后对比测试（见第 6 节失败模式表）。

### 3.3 权限缓存接口化

现状 `permissionCache map[uint64]map[string]bool` 是裸 map，`cacheExpiration` 字段声明了但从未被读取——即缓存永不过期，权限变更后依赖 `ClearAllPermissionCache()` 全量清空，多实例部署时其他实例的缓存不会被清（进程内 map 天然做不到跨实例同步）。

```go
package cache

type PermissionCache interface {
    Get(userID uint64) (map[string]bool, bool)
    Set(userID uint64, perms map[string]bool)
    InvalidateUser(userID uint64)
    InvalidateAll()
}
```

本次先提供 `InMemoryCache`（真正实现 TTL：存储时记录写入时间，`Get` 时判断是否超过 `cacheExpiration`），接口留好以后接 Redis。**多实例一致性问题本次不解决**（见 Non-Goals），但接口化之后，接 Redis 只需新增一个实现，`PermissionChecker` 不用改一行。

### 3.4 登出与 Token 撤销

`LoginOut` 目前是 `// todo 让token失效` 的空实现。方案：

```go
package iam

type TokenStore interface {
    Revoke(ctx context.Context, jti string, exp time.Time) error
    IsRevoked(ctx context.Context, jti string) (bool, error)
}
```

需要 `pkg/jwtauth.Claims` 增加 `jti`（JWT ID）字段（**这是对 `pkg/jwtauth` 的行为改动，影响所有已签发 token 的解析逻辑，需要单独评估兼容性**——旧 token 没有 `jti`，`IsRevoked` 对空 `jti` 应放行而非报错，避免灰度发布期间已登录用户被误判下线）。`JWTMiddleware` 校验 token 后追加一次 `IsRevoked` 检查。本次先给 `InMemoryTokenStore`（定期清理过期项），Redis 版本留作后续。

### 3.5 用 Go `internal` 包可见性强制"Handler 不能跳过 Service"

这是本次重构里**唯一一个不依赖人工 review 纪律、由编译器强制**的设计点。

Go 的规则：路径中包含 `internal` 目录的包，只能被"该 `internal` 目录的父目录"为根的代码树导入。据此，把仓储放在：

```
internal/modules/user/service/internal/repository/
```

那么：

- `internal/modules/user/service/...` 内的代码可以导入它（正常用法）；
- `internal/modules/user/handler/...`（同模块的兄弟包）**编译期禁止导入它**——因为 handler 不在 `.../service` 这棵树下；
- `internal/modules/iam/...`（跨模块）同样禁止导入。

对照当前问题：`user_handler.go` 里 `dao.UserRepo.Find(...)` 这类调用，迁移后会变成"编译不通过"，而不是"等 code review 时才发现"。这比写代码规范或加 lint 规则更彻底——**这正是你文章里说的"包级可见性实现封装"，用在了它本该被用的地方**。

---

## 4. 迁移计划（结构改与行为改分离）

按 Tidy First 原则：结构改（不改行为）先做完、测试跑绿，再做行为改；每个 Phase 是一个独立可回滚的 PR。

| Phase | 内容 | 类型 | 验证方式 |
|---|---|---|---|
| **P0** | 补 `AuthService.Login`、`PermissionService.HasPermission`、`RoleService.CRUD` 等关键路径的特征测试（固定现有行为） | 无改动，仅加测试 | `go test ./...` 全绿，记录覆盖率基线 |
| **P1** | 按 2.1 目录结构搬迁文件，只改 import 路径，不改函数体 | 结构改 | `go build ./...` 通过；`git diff` 人工确认无逻辑变更；P0 测试全绿 |
| **P2** | 引入 `Container`，Service 构造函数改为接收接口参数；保留 `service.XxxInstance` 作为过渡期别名，避免一次性改光所有调用点 | 结构改 | P0 测试全绿；`grep` 确认无新增 handler 直连 dao |
| **P3** | 拆分 `PermissionService` 为四个组件（3.1） | 行为改（小步） | 每拆一组补单测；Postman collection 中 role/permission 相关请求手动过一遍 |
| **P4** | 仓储下沉到 `service/internal/repository`，删除 P2 的过渡别名 | 结构改 | 故意在 handler 里加一行跨层 import，确认编译报错，验证机制生效 |
| **P5** | 密码哈希策略化 + 迁移（3.2） | 行为改，高风险，独立 PR | 迁移前后对比测试：旧密码旧算法用户仍能登录 → 登录后 `password_algo` 变为新值 → 新建用户直接用新算法 |
| **P6** | 权限缓存接口化 + TTL（3.3）、Token 撤销（3.4） | 行为改 | 单测覆盖：缓存过期后重新查库；撤销后的 token 被 401 拒绝；无 `jti` 的旧 token 不被误判撤销 |
| **P7** | README / development-guide.md 对齐：去掉未实现的"数据权限""Clean Architecture"表述，或标注为 Roadmap | 文档 | 人工检查文档与代码一致 |

---

## 5. 影响范围与 diff 预算

| 维度 | 结果 |
|---|---|
| 直接影响文件（Handler 跳层） | `user_handler.go`（5 处）、`system_setting_handler.go`（3 处）、`permission_handler.go`（1 处） |
| 直接影响文件（全局 Instance 调用点） | `middleware/jwt.go:86`、`auth_handler.go`、`permission_handler.go`、`role_handler.go`、`user_role_handler.go`、`system_setting_handler.go` |
| `internal/` 目录搬迁 | 几乎全部 `internal/**/*.go` 的 import 路径（P1 阶段，机械变更，无逻辑 diff） |
| `pkg/` | 不改动（`pkg/crypter` 除外，见 P5） |
| 数据库 | `user` 表加 `password_algo` 字段（P5） |
| 未知/待确认 | 生产环境当前是否已多实例部署（影响 P6 是否要在本次就接 Redis，而不是先上 InMemory） |

diff 量级估计：P1（纯搬迁）改动文件数最多但每个文件 diff 极小；P2-P4 单个 PR 预计 300-600 行；P5 预计 150 行 + 1 条迁移 SQL；P6 预计 200 行。

---

## 6. 失败模式与验证

| 失败模式 | 级别 | 场景 | 验证项 |
|---|---|---|---|
| P5 上线后老用户全部无法登录 | High | 未做算法兼容判断，直接切 Argon2 校验旧哈希 | 上线前用生产库脱敏样本跑一遍"旧哈希 + 正确密码 → Verify 通过"的回归测试 |
| P4 下沉 repository 后，跨模块代码编译不过但被误改成"曲线导入" | Medium | 有人在 handler 里新建一个转发函数绕过 internal 限制 | code review checklist 显式检查；CI 增加一条 `go vet`/自定义脚本扫描 handler 包是否新增对 repository 类型的引用 |
| P6 Token 撤销上线后，灰度期间旧 token（无 `jti`）被误判为已撤销 | High | `IsRevoked` 对空 `jti` 返回 true 而非"跳过检查" | 单测显式覆盖"空 jti → 不查撤销表，直接放行" |
| P2 引入 Container 后，某个模块漏挂载，运行时 panic | Medium | Container 组装顺序错误，或某 Service 遗漏注入 | `main.go` 启动自检：Container 构造完成后遍历关键字段做非 nil 断言，失败则 fail-fast 而非线上 panic |
| P3 拆分 PermissionService 后，`ClearAllPermissionCache` 遗漏某个新组件的缓存 | Medium | 拆分后缓存分散在多处，清理逻辑没跟上 | 缓存清理统一收口到 `PermissionChecker`，其它组件不直接持有缓存引用（组合关系单向） |

---

## 7. Non-Goals（本次明确不做）

- 不引入完整 DDD 四层架构（interfaces/application/domain/infrastructure 全套分层）——项目规模（7 个实体）撑不起，违反 YAGNI。
- 不引入 wire/fx 等代码生成式 DI 框架——`Container` 手写装配几十行代码就能解决，没必要多一个依赖。
- 不重写 `pkg/repository` 泛型仓储引擎——它是通用框架代码，问题在"怎么用"（被全局变量暴露），不在"怎么写"。
- 不实现 `DataPermission` 的业务逻辑（查询拦截/数据范围过滤）——现状只有模型和空 DTO，属于"文档超前于代码"，本次只在 README 里如实标注为 Roadmap，具体实现是另一个独立设计任务。
- 不更换路由注册机制（反射推断 method/path）——现有机制的可审计性问题是真实的，但和模块化重构无直接冲突，留作独立的小改动。
- 不在本次解决权限缓存/Token 撤销的多实例一致性问题——先把接口设计对，Redis 实现是后续按需替换的第二步，不因为"以后可能要分布式"而在第一步就上 Redis。

---

## 8. 决策记录

**背景**：README 声称 Clean Architecture，实际是全局单例三层结构；用户明确希望按"组合优先 + 模块化 DDD"重构。

**Decision Drivers**：维护成本、学习曲线、与当前 7 实体规模的匹配度、diff 风险可控性。

| 候选 | 维护成本 | 学习曲线 | 规模匹配度 | diff 风险 | 结论 |
|---|---|---|---|---|---|
| A. 维持按技术层分，只做局部修补 | 低 | 低 | 中 | 低 | ❌ 不解决"业务边界不可见"的根本问题 |
| **B. 按模块重构 + 构造注入（本方案）** | 中 | 中 | 高 | 中（分阶段可控） | ✅ 采纳 |
| C. 全套 DDD 四层 + CQRS | 高 | 高 | 低（过度设计） | 高 | ❌ 违反 KISS/YAGNI |

**决策结果**：采纳方案 B，按第 4 节分阶段实施，结构改与行为改分 PR。

**重新评估条件**：若后续业务实体数量突破 20+，或团队扩张到需要多组并行开发不同模块，重新评估是否需要在 `iam` 内部进一步拆分（例如 Role 和 Permission 分成独立模块）。

---

## 9. Open Questions

- 生产环境当前是单实例还是多实例部署？决定 P6 是否需要跳过 InMemory 直接上 Redis。**未验证，需要用户确认。**
- `DataPermission` 是否有明确的近期业务需求？决定 P7 阶段 README 措辞是"移除"还是"标注 Roadmap 并排期"。**未验证，需要用户确认。**
- 现有 Postman collection（`go_admin.postman_collection.json`）是否会随本次模块重构同步更新路径？路由路径本身不因模块重构而改变（`AutoRegister` 行为不变），预期无需更新，但需要在 P4 完成后实际跑一遍 collection 确认。
