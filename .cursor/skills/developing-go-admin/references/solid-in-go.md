# SOLID 原则在本项目的检查方法

逐条给出：怎么判断违规、违规信号、本项目的修复案例。用于 [SKILL.md](../SKILL.md) §3.5 的 review-boundaries 子模式。

## S —— 单一职责

**检查方法**：列出一个 Service 的所有导出方法，按"变更原因"分组——如果发现两组方法会因为完全不同的原因被修改（一组是"管理台要加个字段"，另一组是"鉴权性能要优化加缓存"），就是两个职责混在一起。

**违规信号**：
- 方法列表里同时出现低频 CRUD（`Create`/`Update`/`Delete`）和高频带状态的判断逻辑（方法名含 `Has`/`Check`/`Verify` 且持有 cache/map 字段）。
- struct 字段里既有"仅 CRUD 需要"的仓储引用，又有"仅鉴权判断需要"的缓存引用。

**本项目修复案例**：旧版 `PermissionService` 一个 struct 同时管 Role CRUD、Permission CRUD、UserRole 分配、权限缓存判断，拆分为四个协作对象：

| 新组件 | 职责 | 依赖 |
|---|---|---|
| `RoleService` | Role CRUD | `roleRepo` |
| `PermissionService`（变窄） | Permission 元数据 CRUD | `permissionRepo` |
| `UserRoleService` | 用户-角色分配 | `userRoleRepo`、`roleSvc`、`userFinder` |
| `PermissionChecker` | `HasPermission` 鉴权判断，带缓存 | `userRoleSvc`、`roleSvc`、`cache` |

拆分依据不是"行数太多"，是前三者是无状态的管理台操作、变更频率低，`PermissionChecker` 是有状态（缓存）、每请求必经的热路径——**变更原因和性能特征都不同，才是拆分的依据，不是任意按行数切**。

## O —— 开闭原则

**检查方法**：找出代码里"未来大概率会换实现"的判断点（算法选择、存储后端、外部服务地址），看它是写成 `if-else`/`switch` 分支，还是写成接口注入。

**违规信号**：新增一种实现需要修改调用方代码里的分支逻辑，而不是新增一个实现文件。

**本项目修复案例**：密码算法、权限缓存后端、Token 撤销存储方式全部抽成接口（`PasswordHasher`/`PermissionCache`/`TokenStore`），新增实现（比如以后接 Redis）只需要新建一个实现该接口的类型，在 `Container` 里换一行构造调用，`Service`/`Middleware` 内部代码不用改一行。

**例外**：`internal/modules/systemsetting/service/system_setting_service.go` 里的 `validateValue`/`typeDisplay` 用 `switch settingType` 分支处理 4 种固定的配置类型（文本/数字/布尔/JSON），这里**没有**抽策略接口——因为这 4 种类型是业务上封闭的枚举，不是"今后会不断新增实现"的可变点，硬抽接口反而是过度设计（违反 §3.4 的"没有第二个已知会发生的真实实现前不抽"）。

## L —— 里氏替换

Go 没有类继承，天然不存在"子类破坏父类契约"的传统里氏替换问题。本项目需要检查的等价场景是：**一个接口的多个实现是否遵守相同的调用约定**（例如错误处理方式、nil 参数处理方式一致）。检查方法：对着接口方法的注释逐条核对每个实现是否遵守（如 `TokenStore.IsRevoked` 的实现约定"对空 `jti` 调用方自行判断是否跳过检查"，`InMemoryTokenStore` 与未来的 Redis 实现都必须遵守同样的约定，不能有的实现对空 `jti` 报错、有的静默返回 `false`）。

## I —— 接口隔离

**检查方法**：见 [go-oop-composition.md](go-oop-composition.md) §4——消费方是具体业务逻辑就该拆小接口，消费方是通用框架代码可以接受大接口。

**本项目的例外记录**：`pkg/repository.Repository[T]` 作为通用仓储具体类型，方法面覆盖 CRUD/分页/QueryBuilder/事务，这是框架层能力集合而非业务跨模块接口，见 go-oop-composition.md。**不要**以这个例外为理由，在写业务模块的跨模块接口时也放宽到十几个方法。

## D —— 依赖倒置

**检查方法**：看一个 Service/Handler 的依赖来源——构造函数参数列表，还是包级变量/全局函数调用？

**违规信号**：Service 方法内部出现 `xxx.Instance.Yyy(...)` 这种调用；Handler 直接 `import` 另一个模块的具体 `service.Service` 类型而不是接口。

**本项目修复案例**：见 [module-structure.md](module-structure.md) §4（`Container` 替代全局 Instance）与 [SKILL.md](../SKILL.md) §3.3（跨模块窄接口）。JWT 身份从请求上下文的 `ClaimsKey` 读取，不再保留 `jwtauth.Instance`。
