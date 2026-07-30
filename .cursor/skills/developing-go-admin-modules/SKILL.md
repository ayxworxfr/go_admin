---
name: developing-go-admin-modules
description: Guides feature development, refactors, and code review inside the go_admin backend's composition-first, module-based DDD architecture. Use when adding or modifying a business module under internal/modules/* (model/dto/service/handler), wiring a new dependency through internal/bootstrap/container.go, designing an interface that crosses module boundaries (e.g. iam depending on user), choosing among Strategy/Observer/State/Factory/Decorator/Singleton for a Go component, applying SOLID checks to a service, or verifying that new code respects unexported repo encapsulation and module boundaries. Encodes this repo's own conventions (not generic Go advice) for repositories, DTOs, route registration via @route comments, and testing.
---

# Developing go_admin Modules（go_admin 模块化开发指南）

## 设计理念

go_admin 的架构纪律只有两条，其余所有规则都是这两条的具体化：

1. **组合优先**：依赖通过构造函数显式传入，不靠全局变量隐式获取；接口只声明消费方真正用到的方法（1~3 个），不导出整个实现；可变行为（算法、存储后端、缓存策略）通过接口抽出去，而不是写成 if-else 分支或全局变量。
2. **模块化 DDD**：`internal/modules/` 下每个目录是一个限界上下文（`user`/`iam`/`systemsetting`），目录内部按技术层再分（`model`/`dto`/`service`/`handler`），模块间依赖必须通过对方导出的窄接口，且方向单向、不循环。

判断一处代码是否符合本项目风格，只需要问两件事：**这个依赖是构造函数传进来的，还是全局变量/包级变量找来的？这个接口是不是比调用方实际需要的方法更多？** 只要有一个答案不对，就是要修的信号，不是"能跑就行"。

## 1. 决策路由

### 1.1 是否走这个 skill

| 任务类型 | 走哪 |
|---|---|
| 在 `internal/modules/*` 新增/修改 model、dto、service、repository、handler | 本 skill（架构规则）+ `building`（具体编码执行） |
| 新增一个业务模块（一整套 model/dto/service/handler） | 本 skill §3.1 |
| 让模块 A 依赖模块 B 的某个能力 | 本 skill §3.3（窄接口范式） |
| 要不要引入 Strategy/Factory/Decorator 等模式 | 本 skill §3.4 |
| 检查一段 Service/Handler 是否违反 SOLID | 本 skill §3.5 |
| 泛型仓储怎么用、DTO 怎么命名、测试怎么写 | 本 skill §3.6 |
| 纯 bug 修复、不涉及架构决策 | `debugging`，本 skill 仅在触及模块边界时补充 |
| 只讨论方案不动代码 | `planning` |

本 skill 提供的是"这个项目的架构规则"，不是"怎么写 Go 代码"的通用知识；落地实现时仍应叠加 `building` 的编码执行流程。

### 1.2 子模式

| 用户信号 | 子模式 |
|---|---|
| 新增模块 / 新业务领域 / 新建 `internal/modules/xxx` | **add-module**（§3.1） |
| 在已有模块里加接口 / 加字段 / 加查询 | **add-feature**（§3.2） |
| A 模块要用 B 模块的数据或能力 / 模块间调用 | **cross-module**（§3.3） |
| 这里该用什么模式 / 硬编码要不要抽接口 | **pattern-selection**（§3.4） |
| review 一下这段代码是否符合项目架构 / 是否违反单一职责 | **review-boundaries**（§3.5） |

一个任务可能跨多个子模式，按 §3 对应小节顺序处理。

## 2. 核心架构规则（唯一声明处）

完整目录结构、`internal` 可见性强制机制、`Container` 装配顺序、跨模块依赖方向图，详见 [module-structure.md](references/module-structure.md)。这里只给最小心智模型：

```
internal/
├── platform/                # 横切基础设施：app 启动、路由注册、中间件、config、db、cron
├── bootstrap/container.go   # 唯一的显式依赖装配点，替代全局 XxxInstance
└── modules/
    ├── user/{model,dto,service,handler}/
    ├── iam/{model,dto,service,handler,cache,tokenstore}/
    └── systemsetting/{model,dto,service,handler}/
```

三条编译期/运行期都在强制的规则：

- **仓储只在 service 包内可见**：默认 unexported `repo` / `newRepositories`；禁止为纯包装建 `service/internal/repository/`。例外条件见 [module-structure.md](references/module-structure.md) §2。
- **模块间不共享具体类型，只共享窄接口**：`iam` 需要查用户时，依赖的是 `user` 模块导出的 2~3 方法接口（如 `UserFinder`），不是整个 `Repository[User]` 或 `user.Service`。接口在提供方模块里声明并导出（因为只有一个真实实现，省一次转发），消费方只对着接口编程。
- **没有全局单例，只有 `Container`**：`cmd/main.go` 创建好 `xorm.Engine`、`PasswordHasher`、`JWT` 后一次性调用 `bootstrap.NewContainer(...)`，所有 Service 在这里按依赖顺序显式构造并串联；`Container` 的字段是各 Handler 的唯一依赖来源。新增 Service 必须在这里挂上，不允许另起一个包级 `var XxxInstance`。

## 3. 标准流程

### 3.1 新增一个模块（add-module）

完整分步清单（含每步文件模板）见 [adding-a-module-checklist.md](references/adding-a-module-checklist.md)。核心顺序：

1. 判断边界：这个新领域和现有模块（`user`/`iam`/`systemsetting`）是否共享同一组实体？共享则加进现有模块（§3.2），不共享则新建 `internal/modules/<name>/`。
2. 建 `model/`：只放数据结构（xorm tag），不放密码校验之类的算法方法——算法通过接口注入到 service，不挂在 model 上。
3. 建 `dto/`：`CreateXxxRequest`/`UpdateXxxRequest`/`GetXxxRequest`/`GetXxxListRequest`/`XxxResponse`，命名与已有模块（`user/dto/user.go`）保持一致，用 `vd:` tag 做参数校验。
4. 建 `service/`：`NewService(processor pkgrepo.ORMProcessor, ...窄接口) *Service`，`repo` 用 `pkgrepo.NewRepository[model.Xxx](processor)` 当场构造（见 §2）；对外能力另声明窄接口（参考 `user_finder.go`）。
5. 建 `handler/`：每个方法一个 `// @route Verb /path` 注释，方法签名 `func (h *Handler) Xxx(c *context.Context, req *dto.XxxRequest) *context.Response`，构造函数只接收 `*service.Service` 或跨模块窄接口，不接收 repository。
6. 在 `internal/bootstrap/container.go` 里按依赖顺序装配新 Service，加进 `Container` 结构体字段，若有可持久化 model 加进 `Models()`。
7. 在 `cmd/main.go` 的 `setupRoutes` 里构造新 Handler，传给 `app.SetupRoutes(...)`。
8. 若涉及初始数据，更新 `mysql/schema.sql` + `mysql/init_data.sql`。
9. 补 service 层单测（参考 §3.6）。

### 3.2 在现有模块内新增功能（add-feature）

只涉及 3 步：model 加字段（同步 `mysql/schema.sql`）→ dto 加/改请求响应结构 → service 加方法（复用已有的 repo/跨模块依赖字段，不重新声明）→ handler 加 `@route` 方法。**不要**为了图方便让 handler 直接调用另一个模块的 service 具体类型——即使目标方法已经存在，也要通过窄接口注入（见 §3.3），否则会在模块边界上开一个未来无法收口的口子。

### 3.3 跨模块依赖：窄接口范式（cross-module）

依赖方向必须与业务真实依赖一致，且不能循环：本项目是 `user ← iam`、`user ← systemsetting`，`iam` 与 `systemsetting` 互不依赖。新增依赖前先确认方向不会成环，成环说明模块边界划错了，应该退回 §3.1 重新判断边界，而不是硬着头皮加双向依赖。

操作步骤：

1. 在**提供方**模块的 `service` 包内声明一个只包含消费方需要的方法的接口（1~3 个方法），命名体现语义而非"Interface"（如 `UserFinder`、`RoleAssigner`、`PermissionPathResolver`），可参考 `internal/modules/user/service/user_finder.go`。
2. 提供方的具体 `*Service` 隐式实现该接口，不需要显式声明 `var _ Interface = (*Service)(nil)` 之外的任何绑定代码。
3. 消费方的构造函数以接口类型接收依赖，不是具体的 `*user/service.Service` 指针类型。
4. 在 `Container` 里把提供方的具体实例传给消费方构造函数——因为具体类型满足接口，Go 会自动完成隐式转换，不需要显式适配层。

反例识别：如果发现某个 handler 或其他模块试图访问 `svc.repo` 之类的 unexported 字段（或极少数情况下导入了带 `internal` 段的仓储子包），这是编译期就会报错的违规（见 §2），修法是给对应 service 补一个窄接口，不是想办法绕开可见性限制。

### 3.4 选择设计模式（pattern-selection）

先判断"这里到底有没有需要抽象的可变点"，再选模式；六种模式与本项目落点对照表见 [design-patterns-catalog.md](references/design-patterns-catalog.md)。速查：

| 信号 | 模式 | 本项目已有落点 |
|---|---|---|
| 同一件事有多种算法/后端，且今后会换 | 策略 | `PasswordHasher`、`PermissionCache`、`TokenStore` |
| 状态变化后一组订阅者要联动，但联动逻辑与状态本身无关 | 观察者 | 暂无落点，新增前先确认是否只是"调用完再调一次方法"这种更简单的组合就能解决 |
| 同一对象在不同阶段允许的操作集合不同 | 状态 | 暂无落点，`SystemSetting.Type` 目前只是枚举分支，字段/校验分支 <5 种时不必上状态模式 |
| 按运行期条件构造不同实现，调用方不关心具体类型 | 工厂/注册表 | `router.AutoRouterRegister`（按方法名反射推断路由） |
| 在不改变调用方式的前提下叠加横切能力 | 装饰器 | 中间件链 `app.Use(...)`，顺序即叠加顺序 |
| 全局唯一且需要在装配时显式控制生命周期 | 组合根（不是全局单例） | `bootstrap.Container`——注意这不是传统单例模式，是显式构造后传递，禁止再退化成包级 `var Instance` |

**不要在没有第二个真实实现之前就抽接口**——`PasswordHasher`/`PermissionCache`/`TokenStore` 之所以值得抽，是因为项目明确要支持"以后换 Argon2 参数""以后接 Redis"这类已知会发生的替换，不是"也许将来需要"的假设性扩展。

### 3.5 SOLID 自检（review-boundaries）

逐条检查方法、违规信号与本项目修复案例见 [solid-in-go.md](references/solid-in-go.md)。最容易复发的一条是单一职责：一个 Service 同时管理"低频管理台 CRUD"和"高频带缓存的判断逻辑"（`PermissionService` 拆分为 `RoleService`/`PermissionService`/`UserRoleService`/`PermissionChecker` 就是这个信号的修复案例），新写一个 Service 时如果发现方法列表里既有 `Create/Update/Delete` 又有一个被高频调用且带缓存字段的方法，就是该拆的时候。

### 3.6 Repository / DTO / 测试规范

泛型 `Repository[T]`/`ORMProcessor` 的用法、事务写法、DTO 命名与校验 tag 约定、单元与集成测试的隔离手法，见 [repository-and-testing.md](references/repository-and-testing.md)。

## 4. 禁止输出

| 别做 | 改做 |
|---|---|
| "先加个全局变量方便调用，以后再改" | 直接在 `Container` 里装配并通过构造函数注入，多写几个参数不是问题（§2） |
| "这个接口先把 Service 的所有公开方法都放进去，用不上的以后删" | 只声明消费方当前真正调用的方法，1~3 个（§3.3） |
| "为了多加一道防线，给纯包装仓储单独开 `service/internal/repository`" | `repo` 字段小写 + `NewService` 里直接 `pkgrepo.NewRepository[T]`；只有仓储本身复杂到值得独立成包时才拆子包（§2） |
| "这里可能以后要支持多种实现，先抽个接口" | 没有第二个已知会发生的真实实现前不抽（§3.4 末尾） |
| "两个模块都要用就建个 common 包放共享类型" | 提供方导出窄接口给消费方，不新建共享类型包（§3.3） |
| "model 上加个 Verify 方法方便调用" | 算法/校验逻辑放 service，通过接口注入，model 只保留数据结构（§3.1 第 2 步） |

## 5. 输出骨架

完成一次模块新增或跨模块依赖改动后，用下面的清单自检并汇报：

```markdown
## 子模式
- <add-module / add-feature / cross-module / pattern-selection / review-boundaries>

## 模块边界确认
- 新增/改动落在哪个模块：
- 是否新增跨模块依赖：<无 / 依赖方向 A -> B，接口名 XxxFinder/XxxAssigner>
- 依赖方向是否会成环：<否，理由>

## 改动清单
- model / dto / service / handler：<各自改了什么；仓储写在哪个 service 文件里>
- Container 装配：<新增字段 / 新增构造调用，或"无新增依赖，未改动">
- cmd/main.go 路由挂载：<改动或"无需改动">

## 架构规则核对
- repo 是否 unexported、是否多余拆了 `internal/repository`：<否多余拆分 / 已按 §2 收敛>
- 是否新增全局包级变量承载依赖：<否 / 是，已改为构造注入>
- 新接口方法数：<N，是否 ≤3>

## 验证
- go build ./...：<结果>
- go vet ./...：<结果>
- 相关单测：<命令与结果>
```
