# 设计模式落点对照表

六种模式在本项目里的 Go 惯用写法与真实落点，用于 [SKILL.md](../SKILL.md) §3.4。每种模式先给判断信号，再给本项目代码，帮助区分"该用"与"过度设计"。

## 1. 策略模式（Strategy）—— 本项目用得最多的模式

**判断信号**：同一件事存在多种做法，且做法之间可以互相替换，调用方不应该关心具体是哪一种。

**写法**：定义一个小接口，多个实现互相替换，调用方持有接口类型字段，通过构造函数注入具体实现。

```go
// pkg/crypter/password_hasher.go
type PasswordHasher interface {
	Hash(plain string) (string, error)
	Verify(plain, hashed string) bool
}

// internal/modules/user/service/user_service.go
type Service struct {
	repo   pkgrepo.Repository[model.User]
	hasher crypter.PasswordHasher // 策略：具体是 Argon2Hasher 还是别的，Service 不关心
}
```

本项目的三处落点：`crypter.PasswordHasher`（`Argon2Hasher` 实现）、`iam/cache.PermissionCache`（`InMemoryCache` 实现，接口留好接 Redis）、`iam/tokenstore.TokenStore`（`InMemoryTokenStore` 实现）。三者的共同特征：**接口方法数都 ≤4，且都明确"以后要换实现"**（换算法参数、换缓存后端为 Redis）——这是"值得抽策略接口"的判断依据，不是"随便一个 if-else 都要抽"。

## 2. 观察者模式（Observer）—— 本项目暂无落点，谨慎引入

**判断信号**：一个状态变化后，需要通知一组数量不固定、互相独立、事后可能动态增减的订阅者，且发布方不应该硬编码知道每个订阅者是谁。

**引入前先排除更简单的方案**：如果订阅者数量固定且已知（比如"角色权限变更后清一下缓存"只有一个动作），直接在调用处显式调用对应方法（如 `PermissionChecker.InvalidateAll()`）就够了，不需要发布-订阅机制。本项目目前所有"变更后要联动"的场景都只有 1~2 个固定动作，因此没有引入观察者模式；如果未来出现"审计日志""消息通知""缓存失效"等 3 个以上互相独立、需要可插拔增减的联动方，才是引入的时机。

## 3. 状态模式（State）—— 本项目暂无落点

**判断信号**：同一个实体在不同状态下，同名操作的行为完全不同（不是"多几个 if 分支"，是"整套允许的操作集合都不同"），且状态会随时间推移单向或多向迁移。

**引入前先排除更简单的方案**：`SystemSetting.Type`（文本/数字/布尔/JSON）看起来像状态，但实际只是"根据类型选择不同的校验/展示函数"，用一个 `switch` 分支（见 `validateValue`/`typeDisplay`）就足够清晰——因为这 4 种类型不会互相"迁移"，也没有"某个类型下才允许某个操作"的语义。真正需要状态模式的信号是类似"订单状态机"这种：状态之间有迁移路径、每个状态下允许的操作集合不同、状态种类会持续增长。

## 4. 工厂/注册表模式（Factory / Registry）

**判断信号**：需要按运行期条件构造出不同的具体类型，调用方只依赖抽象结果，不关心构造细节；或者需要维护一个"名字/规则 → 处理函数"的映射表。

**本项目落点**：`internal/platform/router.AutoRouterRegister`——按方法名前缀（`Get`/`Post`/`Create`/`Update`/`Delete`）和反射得到的方法签名，推断出 HTTP method 与路径并注册路由，调用方（`SetupRoutes`）只管把 Handler 实例扔给 `RegisterStruct`，不关心每个方法最终映射到哪个路径：

```go
func (r *AutoRouterRegister) inferMethodAndPathBase(funcName string) (RouterMethod, string) {
	switch {
	case strings.HasPrefix(funcName, "Get"):
		return GET, strings.TrimPrefix(funcName, "Get")
	case strings.HasPrefix(funcName, "Create"):
		return POST, strings.TrimPrefix(funcName, "Create")
	// ...
	}
}
```

新增 Handler 方法只需要遵守命名约定（`Get`/`Create`/`Update`/`Delete` 前缀 + `@route` 注释），不需要改动 `AutoRouterRegister` 本身——这也是开闭原则的体现。

## 5. 装饰器模式（Decorator）

**判断信号**：需要在不改变调用方式（同样的输入输出签名）的前提下，给一个操作叠加若干横切能力（日志、鉴权、限流、追踪），且这些能力的组合顺序需要可配置。

**本项目落点**：中间件链，`app.Use(...)` 按调用顺序层层包裹请求处理：

```go
app.Use(middleware.CorsMiddleware())
app.Use(sentinel.SentinelMiddleware())
app.Use(middleware.GlobalErrorHandlerMiddleware())
app.Use(middleware.LogMiddleware())
app.Use(middleware.TraceContextMiddleware())
app.Use(middleware.BindAndValidateMiddleware())
```

每个中间件签名一致（`app.HandlerFunc`），彼此不知道对方存在，靠注册顺序决定包裹顺序——这正是装饰器模式在 HTTP 框架里的标准形态，新增一种横切能力（比如限流）只需要新写一个同签名的中间件插进链里，不需要修改其他中间件。

## 6. 单例陷阱与它的正确替代：组合根（Composition Root）

**判断信号出现时，先警惕，不要直接抄传统单例写法**：需要某个对象"全局唯一"时。传统单例（包级 `var Instance = New()` + 各处直接读取）在本项目里是明确反对的写法（见 [module-structure.md](module-structure.md) §4 与 [solid-in-go.md](solid-in-go.md) D 项）。

**正确替代**：`internal/bootstrap.Container` 承担"全局唯一"的语义，但获取方式是**显式传递**，不是"各处读取包级变量"：

```go
container := bootstrap.NewContainer(engine, crypter.NewArgon2Hasher(), jwt)
// container 作为参数逐层传给 setupRoutes、各 Handler 构造函数
// 没有任何代码写 bootstrap.GetContainer() 或 bootstrap.Instance
```

区分两者的关键：单例模式的问题不是"只有一个实例"，而是"任何代码都能不经声明地拿到它"，导致依赖关系在类型签名上消失。`Container` 同样只构造一次，但每个需要它（或它的某个字段）的函数都必须在参数列表里显式声明，依赖关系永远可以通过读函数签名看出来。
