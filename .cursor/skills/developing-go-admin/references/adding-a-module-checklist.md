# 新增业务模块：完整清单

[SKILL.md](../SKILL.md) §3.1 的展开版，每一步给最小可编译模板。以新增一个假想的 `notice`（站内通知）模块为例。

## 步骤 0：边界判断（先做，不要跳过）

按 [module-structure.md](module-structure.md) §3 的三条标准过一遍：是否新表？是否与现有实体强耦合？依赖图是否成环？只有三条都通过（新表、不强耦合、依赖单向）才继续下面的步骤；否则回 §3.2（加进现有模块）。

## 步骤 1：`model/`

```go
// internal/modules/notice/model/notice.go
package model

import "time"

// Notice 站内通知模型。只放数据结构，不放业务规则方法。
type Notice struct {
	ID         uint64    `xorm:"pk autoincr bigint unsigned 'id'" json:"id"`
	ReceiverID uint64    `xorm:"bigint unsigned notnull 'receiver_id'" json:"receiver_id"`
	Title      string    `xorm:"varchar(100) notnull 'title'" json:"title"`
	Content    string    `xorm:"text 'content'" json:"content"`
	IsRead     bool      `xorm:"bool notnull default 0 'is_read'" json:"is_read"`
	CreateTime time.Time `xorm:"created" json:"create_time"`
}
```

## 步骤 2：`dto/`

```go
// internal/modules/notice/dto/notice.go
package dto

import (
	"time"

	"github.com/ayxworxfr/go_admin/pkg/apiparam"
)

type CreateNoticeRequest struct {
	ReceiverID uint64 `json:"receiver_id" vd:"$>0"`
	Title      string `json:"title" vd:"len($)>0&&len($)<100"`
	Content    string `json:"content"`
}

type GetNoticeListRequest struct {
	apiparam.Page
	ReceiverID uint64 `query:"receiver_id" vd:"$>0" xorm:"receiver_id op=eq"`
	IsRead     *bool  `query:"is_read" xorm:"is_read op=eq"`
}

type NoticeResponse struct {
	ID         uint64    `json:"id"`
	Title      string    `json:"title"`
	Content    string    `json:"content"`
	IsRead     bool      `json:"is_read"`
	CreateTime time.Time `json:"create_time"`
}
```

## 步骤 3：`service/`

按 [module-structure.md](module-structure.md) §2：`repo` 字段 unexported，`NewService` 里直接 `pkgrepo.NewRepository[T](db)`。禁止为纯包装仓储建 `service/internal/repository/`。

若本模块能力会被其它模块消费（例如未读数），同包声明窄接口：

```go
// internal/modules/notice/service/notice_finder.go
package service

import "context"

// NoticeFinder 是消费方需要的最小契约，仅在确实出现第二个消费方时才新增方法。
type NoticeFinder interface {
	CountUnread(ctx context.Context, userID uint64) (int64, error)
}
```

主服务：

```go
// internal/modules/notice/service/notice_service.go
package service

import (
	"context"

	"github.com/ayxworxfr/go_admin/internal/modules/notice/dto"
	"github.com/ayxworxfr/go_admin/internal/modules/notice/model"
	"github.com/ayxworxfr/go_admin/pkg/logger"
	pkgrepo "github.com/ayxworxfr/go_admin/pkg/repository"
	"github.com/jinzhu/copier"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

type Service struct {
	repo *pkgrepo.Repository[model.Notice]
}

// NewService 创建通知服务。跨模块依赖用窄接口参数，不传具体 *Service。
func NewService(db *pkgrepo.DB) *Service {
	return &Service{repo: pkgrepo.NewRepository[model.Notice](db)}
}

func (s *Service) Create(ctx context.Context, req *dto.CreateNoticeRequest) (*model.Notice, error) {
	var n model.Notice
	if err := copier.Copy(&n, req); err != nil {
		return nil, errors.Wrap(err, "failed to copy request to notice")
	}
	if err := s.repo.Create(ctx, &n); err != nil {
		logger.Error(ctx, "Failed to create notice", zap.Error(err))
		return nil, errors.Wrap(err, "failed to create notice")
	}
	return &n, nil
}

func (s *Service) CountUnread(ctx context.Context, userID uint64) (int64, error) {
	return s.repo.QueryBuilder().Eq("receiver_id", userID).Eq("is_read", false).Count(ctx)
}
```

## 步骤 4：`handler/`

```go
// internal/modules/notice/handler/notice_handler.go
package handler

import (
	"github.com/ayxworxfr/go_admin/internal/modules/notice/dto"
	"github.com/ayxworxfr/go_admin/internal/modules/notice/service"
	"github.com/ayxworxfr/go_admin/pkg/api"
	"github.com/jinzhu/copier"
)

type Handler struct {
	svc *service.Service
}

func NewHandler(svc *service.Service) *Handler {
	return &Handler{svc: svc}
}

// @route Post /notice
func (h *Handler) CreateNotice(c *api.Context, req *dto.CreateNoticeRequest) *api.Response {
	n, err := h.svc.Create(c.Context(), req)
	if err != nil {
		return api.DatabaseError(err)
	}
	var resp dto.NoticeResponse
	if err := copier.Copy(&resp, n); err != nil {
		return api.InternalError(err)
	}
	return api.Success(&resp)
}
```

写完 handler 后执行 `make generate`，把 `@route` 编进 `internal/platform/router/routes_gen.go`。

## 步骤 5：`Container` 装配

在 `internal/bootstrap/container.go` 里：

```go
import noticeservice "github.com/ayxworxfr/go_admin/internal/modules/notice/service"
import noticemodel "github.com/ayxworxfr/go_admin/internal/modules/notice/model"

type Container struct {
	// ... 已有字段
	Notice *noticeservice.Service
}

func NewContainer(engine *xorm.Engine, hasher crypter.PasswordHasher, jwt *jwtauth.JWT) *Container {
	// ... 已有构造逻辑
	noticeSvc := noticeservice.NewService(db) // 放在它依赖的对象之后构造

	return &Container{
		// ... 已有字段
		Notice: noticeSvc,
	}
}

func (c *Container) Models() []any {
	return []any{
		// ... 已有 model
		new(noticemodel.Notice),
	}
}
```

## 步骤 6：`internal/bootstrap/routes.go` 挂载路由

```go
func setupRoutes(app *myapp.App, c *Container) {
	// ... 已有 handler 构造
	noticeHandler := noticehandler.NewHandler(c.Notice)

	app.SetupRoutes(authHandler, jwtMiddleware,
		userHandler, roleHandler, permissionHandler, userRoleHandler,
		systemSettingHandler, noticeHandler) // 追加到业务 handler 列表末尾
}
```

## 步骤 7：数据库脚本

在 `mysql/schema.sql` 追加 `CREATE TABLE notice (...)`，在 `mysql/init_data.sql` 按需追加初始数据；两者都是手写 SQL（本项目不用 ORM 自动迁移生产表结构，`db.SyncModels` 仅用于本地开发/测试环境同步）。

## 步骤 8：测试

Service 层至少覆盖：一个正常创建路径、一个依赖跨模块接口的路径（若有）。仓储/事务相关测试参考 [repository-and-testing.md](repository-and-testing.md) §5 的隔离手法，用 `-short` 区分是否需要真实数据库。

## 完成后自检

对照 [SKILL.md](../SKILL.md) §5 自检：仓储是否按 §2 封装；跨模块接口方法数是否 ≤3。
