# Go OOP：组合优先的具体写法

本文件展开 [SKILL.md](../SKILL.md) 的核心理念——Go 没有继承，本项目也刻意不用 embedding 模拟继承；"面向对象"在这里等价于四个具体机制的组合，逐一说明。

## 1. 封装 = struct + 小写字段 + 包边界，不是"加 getter/setter"

Go 的封装单位是**包**，不是类。字段大小写决定的是"包外能不能访问"，与 Java/C++ 的 `private` 字段但配 `public` getter 的写法完全不同——本项目不给字段套 getter/setter，需要限制访问就把字段设为包内小写，需要暴露就直接导出字段或提供语义化方法。

真正的封装边界靠**包**实现，而不是字段大小写：`model.User` 的 `PasswordHash` 字段是导出的（xorm 需要），但"谁能拿到这个用户的仓储对象去改库"这件事，靠 `Service.repo` 是 unexported 字段控制——handler 是另一个包，编译期天然看不到这个字段（见 [module-structure.md](module-structure.md) §2）。字段可见性管的是"结构体内部数据"，包可见性管的是"谁能拿到这个能力"，两者不是同一件事，不要混淆；只有仓储代码本身复杂到值得独立成包时，才需要再叠加 `internal` 子包这一层更重的工具。

## 2. 多态 = interface 隐式满足，不声明"implements"

```go
// internal/modules/user/service/user_finder.go
type UserFinder interface {
	FindByID(ctx context.Context, id uint64) (*model.User, error)
	FindByUsername(ctx context.Context, username string) (*model.User, error)
	VerifyPassword(user *model.User, plainPassword string) bool
}
```

`*service.Service`（user 模块的具体实现）没有任何一行代码声明"我实现了 `UserFinder`"——只要方法签名对得上，赋值和函数传参时会自动满足接口。这意味着**接口可以在使用方需要它之前完全不存在**：先写具体实现，等出现第二个使用者（`iam.AuthService` 需要查用户）时才回头声明接口，不要在只有一个实现、也没有第二个消费方的阶段就先抽出接口——那是"为了看起来像 OOP"而抽象，不是真的需要多态。

## 3. 复用 = 组合而非继承

本项目的 Service/Handler 不使用 struct embedding 来复用行为（embedding 在 Go 里常被误用成"伪继承"）。复用的标准写法是**持有字段 + 委托调用**：

```go
// internal/modules/iam/service/permission_checker.go
type PermissionChecker struct {
	userRoleSvc *UserRoleService  // 组合：持有依赖，委托调用
	roleSvc     *RoleService
	cache       cache.PermissionCache
}

func (c *PermissionChecker) getUserAllPermissions(ctx context.Context, userID uint64) ([]model.Permission, error) {
	roles, err := c.userRoleSvc.RetrieveRolesByUserID(ctx, userID)   // 委托，不是继承覆写
	// ...
	perms, err := c.roleSvc.RetrievePermissionByRoleID(ctx, role.ID)
	// ...
}
```

`PermissionChecker` 不"继承" `UserRoleService` 的能力，而是持有一个 `*UserRoleService` 字段并调用它导出的方法。好处是依赖关系在类型签名上就能看到（`grep` 结构体字段就知道谁依赖谁），而 embedding 常常让调用者搞不清一个方法到底是自己实现的还是"继承"来的。

**唯一允许用 embedding 的场景**：纯数据结构的字段复用（如多个 DTO 都需要分页参数时嵌入 `apiparam.Page`），因为这里没有方法委托的语义，只是字段拼接：

```go
// internal/modules/user/dto/user.go
type GetUserListRequest struct {
	apiparam.Page      // 纯数据字段复用，不涉及方法委托
	Username string `query:"username" ...`
}
```

## 4. 抽象 = 小接口（1~3 方法）

本项目里跨模块的接口全部控制在 1~3 个方法：`UserFinder`（3 方法）、`RoleAssigner`（1 方法）、`PermissionPathResolver`（1 方法）、`PermissionCache`（4 方法，接近上限但每个方法职责单一）、`TokenStore`（2 方法）。

反例对照：`pkg/repository.Repository[T]` 方法面覆盖 CRUD + QueryBuilder + 事务 + 原生 SQL，这是**故意**的框架层能力集合——服务对象是"任意实体的任意查询组合"，不是某个业务消费方的窄依赖。判断一个接口该不该拆小，看它的**消费方**：如果消费方是"某个具体业务逻辑，只需要其中几个方法"，就该抽小接口；如果消费方是"通用框架的每一种可能用法"，较宽的能力面是合理的。

## 5. 构造 = `NewXxx()` 工厂函数，构造完之后不允许再塞进全局变量

```go
func NewService(db *pkgrepo.DB, hasher crypter.PasswordHasher) *Service {
	return &Service{
		repo:   pkgrepo.NewRepository[model.User](db),
		hasher: hasher,
	}
}
```

`NewXxx()` 本身不是本项目独有的规则（是 Go 社区通用惯例），本项目在此之上加的约束是：**构造出来的实例只能通过两条路径到达使用方——`Container` 的字段，或者作为构造参数逐层传递**，不允许出现第三条路径（包级 `var Instance = NewXxx()`）。旧版本 `pkg/jwtauth.Instance`、`crypter.Instance`、`service.PermissionServiceInstance` 都是这条规则被打破的产物：构造函数本身没写错，但构造完之后被塞进了全局变量，导致依赖来源从"看函数签名就知道"变成"要满项目搜索包级变量赋值才知道"。

## 6. 数据结构与算法解耦：model 不持有可替换的行为

```go
// internal/modules/user/model/user.go
// User 用户模型。不再持有密码哈希/校验方法——加密算法是可替换的策略，
// 由 Service 层持有 crypter.PasswordHasher 依赖，模型只保留纯数据结构，
// 避免"数据结构与具体加密实现耦合"导致以后换算法要改模型定义。
type User struct {
	ID           uint64 `xorm:"pk autoincr bigint unsigned 'id'" json:"id"`
	PasswordHash string `xorm:"varchar(255) notnull 'password_hash'" json:"-"`
	// ...
}
```

判断一个方法该挂在 `model` 上还是 `service` 上：这个方法的实现**今后会不会因为技术选型变化而整体替换**（哈希算法、缓存后端、序列化格式）？会替换就放 service 并通过接口注入（见 [design-patterns-catalog.md](design-patterns-catalog.md) 的策略模式一节）；纯粹是"根据自身字段计算一个派生值"（例如格式化展示字符串）且不涉及可替换的外部依赖，才可以挂在 model 上作为纯函数方法。
