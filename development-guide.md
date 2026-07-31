# 模块代码开发指南

## 权威来源

架构规则（模块边界、依赖注入、设计模式选型、SOLID 自检、仓储/DTO/测试规范）的**唯一权威来源**是 [`.cursor/skills/developing-go-admin-modules/SKILL.md`](.cursor/skills/developing-go-admin-modules/SKILL.md)。无论是人工开发还是加载 AI 辅助编码，都应先读这份文档；本文件只保留与「怎么协作」相关的流程性内容，不重复架构细节。

## 开发要求

### 框架

- Go 1.24+、Hertz、Xorm、MySQL 8.0+

### 新增/修改代码前必读

1. [`developing-go-admin-modules/SKILL.md`](.cursor/skills/developing-go-admin-modules/SKILL.md) —— 判断该新建模块还是加进现有模块、跨模块依赖怎么设计、该不该抽接口
2. `mysql/schema.sql` —— 了解当前数据库表结构
3. 现有同类模块的实现（如新增功能与用户管理类似，直接参考 `internal/modules/user/`；与权限相关，参考 `internal/modules/iam/`）

### 核心任务

1. 按 SKILL.md §3.1（新增模块）或 §3.2（现有模块加功能）确定改动范围
2. 编写 `internal/modules/<mod>/` 下的 model/dto/service/handler（仓储在 service 包内 unexported 构造，默认不单独开 `internal/repository` 子包）
3. Handler 方法写好 `// @route Verb /path` 后执行 **`make generate`**（或 `go generate ./internal/platform/router/...`），更新 `routes_gen.go`
4. 在 `internal/bootstrap/container.go` 装配依赖；在 `internal/bootstrap/routes.go` 构造 Handler 并交给 `app.SetupRoutes`（**不要**在 `cmd/main.go` 挂路由）
5. 涉及新表/新字段时同步更新 `mysql/schema.sql` 与 `mysql/init_data.sql`

### 常用命令

```bash
make generate          # 扫描 @route → routes_gen.go
make build             # 先 generate 再编译
make test              # 短测（含 TestCompiledRoutesFresh）
make docker-compose-up # 构建并启动整套 Docker 环境
```

## 开发流程

1. 阅读 `mysql/schema.sql` 理解现有数据库结构
2. 分析业务逻辑并设计功能方案（跨模块依赖、是否需要新接口，对照 SKILL.md §3.3/§3.4）
3. 提交功能设计方案供讨论
4. 获得批准后开始编码，完成后对照 SKILL.md §5 的输出骨架自检
5. 若改动了 `@route`，确认 `make generate` 已执行且相关测试通过

## 编码标准

- 遵循项目现有代码风格与目录结构
- 确保新功能与现有系统集成，不引入循环依赖
- 遇到疑问及时讨论，不要在架构边界不清楚的情况下先写代码
- 仓储用法：`pkgrepo.New(engine)` → `NewRepository[T](db)` + `QueryBuilder`；事务用 `db.Transaction(ctx, fn)`，写入路径按需开事务，不要默认每写必事务

## 设计理念

以下十条是通用软件工程原则，本项目在此基础上如何具体落地（哪些该抽接口、哪些不该抽、什么时候该拆分 Service），详见 SKILL.md 及其 `references/` 目录，这里不重复展开。

1. **DRY**：避免代码重复，通过组合（持有字段 + 委托调用）复用行为，而非继承/embedding
2. **单一职责**：一个 Service 只负责一类变更原因的逻辑；无状态管理台 CRUD 与有状态的高频判断逻辑要分开
3. **开放/封闭**：已知会替换的实现（算法、存储后端）抽接口，未知是否会替换的不抽
4. **依赖倒置**：依赖通过构造函数注入，不读全局变量；跨模块依赖窄接口，不依赖具体类型
5. **KISS**：没有第二个真实实现前不抽接口，没有真实的多状态迁移前不上状态模式
6. **YAGNI**：只实现当前需要的功能，"未来可能需要"不是抽象的理由
7. **关注点分离**：模块边界即限界上下文边界，用 Go `internal` 可见性在编译期强制
8. **可测试性**：Service 层依赖通过接口注入，便于单测替换；集成测试用 `-short` 区分是否需要真实数据库
9. **错误处理与日志**：统一用 `github.com/pkg/errors` 包装错误并保留调用栈，关键路径用 `pkg/logger` 记录结构化日志
10. **性能考虑**：区分低频管理台操作与高频请求路径，只在真正的热路径上引入缓存等复杂度
