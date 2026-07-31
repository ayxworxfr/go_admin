# 仓储 / DTO / 测试规范

用于 [SKILL.md](../SKILL.md) §3.6。这是本项目在数据访问层、请求响应结构、测试隔离上的具体约定。

## 1. 泛型仓储 `Repository[T]` 用法

默认：`NewService` 里直接 `pkgrepo.NewRepository[T](db)`（见 [module-structure.md](module-structure.md) §2）：

```go
func NewService(db *pkgrepo.DB) *Service {
	return &Service{repo: pkgrepo.NewRepository[model.SystemSetting](db)}
}
```

只有一个模块内多个 service 文件要共用同一组仓储时（如 `iam` 的 4 个仓储被 `role_service.go`/`permission_service.go`/`user_role_service.go` 共用），才值得抽一个 unexported 的 `repositories` 结构体收敛构造逻辑（同样在 `service` 包内，不是子包）：

```go
// internal/modules/iam/service/repositories.go
type repositories struct {
	role       *pkgrepo.Repository[model.Role]
	permission *pkgrepo.Repository[model.Permission]
	// ...
}

func newRepositories(db *pkgrepo.DB) *repositories {
	return &repositories{
		role:       pkgrepo.NewRepository[model.Role](db),
		permission: pkgrepo.NewRepository[model.Permission](db),
		// ...
	}
}
```

Service 里的常见用法：

```go
// 按主键查
u, err := s.repo.FindByID(ctx, req.ID)

// 按字段查单条
u, err := s.repo.Find(ctx, &model.User{Username: username})

// 分页查询：req 本身实现分页参数（内嵌 apiparam.Page），xorm tag 里的 `op=like`/`op=eq`
// 由 QueryBuilder 自动读取字段上的 xorm tag 拼条件，不需要手写 WHERE
data, total, err := s.repo.FindPage(ctx, req, req.Limit, req.Offset)

// 链式查询：不满足于字段自动映射时，用 QueryBuilder 显式拼条件
count, err := s.repo.QueryBuilder().Eq("key", key).Ne("id", excludeID).Count(ctx)
settings, err := s.repo.QueryBuilder().Eq("category", category).OrderBy("key ASC").Find(ctx)

// AND/OR 与分组（默认 AND；Or() 影响下一条；括号用 AndGroup/OrGroup）
rows, err := s.repo.QueryBuilder().
	Eq("status", 1).
	AndGroup(func(g *pkgrepo.QueryBuilder[model.Role]) {
		g.Eq("code", "admin").Or().Eq("code", "owner")
	}).
	Find(ctx)

// 批量删除：按 ID 逐个删除并收集错误，不要把一个 IDs 切片直接 copier.Copy 进 model 再整体 Delete
// （字段名不匹配时 copier 会静默产出零值，导致删除条件变成"按零值匹配"，实际什么都没删掉）
var result *multierror.Error
for _, id := range ids {
	if err := s.repo.DeleteByID(ctx, id); err != nil {
		result = multierror.Append(result, errors.Wrapf(err, "failed to delete %d", id))
	}
}
return result.ErrorOrNil()
```

## 2. 事务用法

```go
err := repo.Transaction(ctx, func(txCtx context.Context) error {
	if err := repo.Create(txCtx, &user); err != nil {
		return err // 返回 error 触发回滚
	}
	if err := otherRepo.Update(txCtx, &order); err != nil {
		return err
	}
	return nil // 返回 nil 才会提交
})
```

**规则**：事务边界内的所有仓储调用必须使用 `Transaction` 回调传入的 `txCtx`，不能继续用外层的 `ctx`——事务对象挂在 `context.Context` 里传递，用错 ctx 会导致这次调用跑在事务外，破坏原子性。

## 3. DTO 分层与命名约定

每个模块的 `dto/` 按操作类型分文件内的多个 struct，命名固定：

| 命名 | 用途 | 示例字段模式 |
|---|---|---|
| `CreateXxxRequest` | 创建 | 必填字段用 `vd:"len($)>0&&..."` |
| `UpdateXxxRequest` | 更新 | 主键字段 `vd:"$>0"`；允许不修改的字段（如密码）用宽松校验 `vd:"len($)>=0||(...)"` |
| `DeleteXxxRequest` | 批量删除 | `IDs []uint64` |
| `GetXxxRequest` | 单条查询 | `ID uint64 \`query:"id" vd:"$>0"\`` |
| `GetXxxListRequest` | 分页列表 | 内嵌 `apiparam.Page`，查询字段加 `xorm:"字段名 op=like/eq"` 供 `QueryBuilder` 自动拼条件 |
| `XxxResponse` | 响应视图 | 不直接返回 model，避免把内部字段（如密码哈希）泄露到接口 |

指针字段表达"未设置"与"清空"的区别：

```go
// internal/modules/user/dto/user.go
RoleIDs *[]uint64 `json:"role_ids"` // 指针区分"未设置"（nil，不改角色）和"清空角色"（空切片）
```

model → dto 的转换统一用 `copier.Copy`，不手写字段搬运；转换失败要 `errors.Wrap` 后返回，不要吞掉。

## 4. Handler 与路由注册约定

```go
// @route Post /user
func (h *Handler) CreateUser(c *reqctx.Context, req *dto.CreateUserRequest) *reqctx.Response {
	u, err := h.svc.Create(c.Context(), req)
	if err != nil {
		return reqctx.DatabaseError(err)
	}
	// ...
	return reqctx.Success(resp)
}
```

- 路由注册优先查编译期表（`routes_gen.go`，来源是 `// @route Verb /path`）；无表项时才按方法名前缀（`Get`/`Post`/`Create`/`Update`/`Delete`/`Put`）推断。改 `@route` 后必须 `make generate`，否则本地/CI 的 `TestCompiledRoutesFresh` 会失败。
- 返回值统一用 `pkg/reqctx` 包提供的构造函数：`reqctx.Success`/`reqctx.PageSuccess`/`reqctx.NoContent`/`reqctx.DatabaseError`/`reqctx.BusinessError`/`reqctx.InternalError`/`reqctx.Unauthorized`，不要手写 `map[string]any` 拼响应体。
- Handler 构造函数只接收 `*service.Service` 或跨模块窄接口，不接收 repository 类型（也拿不到，repo 字段是 unexported，见 [module-structure.md](module-structure.md) §2）。

## 5. 测试隔离手法

集成测试（需要真实数据库）与单元测试用 `testing.Short()` 区分，跑 `go test -short ./...` 时跳过集成测试：

```go
func TestUserRepo(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	// ...
}
```

需要真实 ORM 引擎的测试，用一个与业务 model 结构兼容但**不直接依赖业务模块**的本地 `testModel`，避免测试基础设施（如 `internal/platform/db`）反向依赖某个具体业务模块：

```go
// internal/platform/db/db_repository_test.go
// testModel 是通用仓储引擎的集成测试用模型，字段与 user 模块的 User 兼容，
// 但刻意不依赖 modules/user——db 包测试的是"仓储引擎能不能正确工作"这件事，
// 不应该关心某个具体业务模块的表结构。
type testModel struct {
	ID       uint64 `xorm:"pk autoincr bigint unsigned 'id'" json:"id"`
	Username string `xorm:"varchar(50) notnull unique 'username'" json:"username"`
}

func (testModel) TableName() string { return "user" }

var (
	once       sync.Once
	testEngine *xorm.Engine
)

func TestMain(m *testing.M) {
	setupTestDB()
	code := m.Run()
	clearTestDB()
	testEngine.Close()
	os.Exit(code)
}
```

事务一致性测试的标准模式（验证 commit/rollback 行为，不是验证业务逻辑本身）：

```go
t.Run("Rollback", func(t *testing.T) {
	clearTestDB()
	_, err := repo.Transaction(ctx, func(txCtx context.Context) (any, error) {
		repo.Create(txCtx, user)
		return nil, errors.New("business error") // 故意返回错误触发回滚
	})
	assert.Error(t, err)
	count, _ := repo.QueryBuilder().Count(ctx)
	assert.Equal(t, int64(0), count) // 验证确实回滚了，而不是"跑起来没报错就算过"
})
```

Service 层的单元测试优先针对纯逻辑分支（如密码校验、权限路径匹配的 `matchPath` 通配符逻辑），不需要真实数据库时不要引入 `-short` 之外的隔离机制。
