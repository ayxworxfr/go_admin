package bootstrap

import (
	"time"

	iamcache "github.com/ayxworxfr/go_admin/internal/modules/iam/cache"
	iammodel "github.com/ayxworxfr/go_admin/internal/modules/iam/model"
	iamservice "github.com/ayxworxfr/go_admin/internal/modules/iam/service"
	iamtokenstore "github.com/ayxworxfr/go_admin/internal/modules/iam/tokenstore"
	ssmodel "github.com/ayxworxfr/go_admin/internal/modules/systemsetting/model"
	ssservice "github.com/ayxworxfr/go_admin/internal/modules/systemsetting/service"
	usermodel "github.com/ayxworxfr/go_admin/internal/modules/user/model"
	userservice "github.com/ayxworxfr/go_admin/internal/modules/user/service"
	"github.com/ayxworxfr/go_admin/pkg/crypter"
	"github.com/ayxworxfr/go_admin/pkg/jwtauth"
	"github.com/ayxworxfr/go_admin/pkg/repository"
	"xorm.io/xorm"
)

// permissionCacheTTL 用户权限缓存的存活时间，与旧版 `cacheExpiration` 字段
// 想表达但从未生效的语义一致——这里是真正会被读取判断的 TTL。
const permissionCacheTTL = 1 * time.Hour

// Container 是全项目唯一的显式依赖装配点，取代原来"构造完就塞进全局变量"
// 的一堆 XxxInstance。它不是 DI 框架——只是把原来隐藏在各处的 `New*` 调用
// 集中到一个地方按依赖顺序显式串联，任何人看这一个文件就能看出完整的依赖图。
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

// NewContainer 按依赖顺序装配全部服务。engine/hasher/jwt/tokenStore 由 Run 在
// 创建基础设施后传入，Container 本身不关心它们是怎么来的。
func NewContainer(engine *xorm.Engine, hasher crypter.PasswordHasher, jwt *jwtauth.JWT, tokenStore iamtokenstore.TokenStore) *Container {
	db := repository.New(engine)

	userSvc := userservice.NewService(db, hasher)

	roleSvc := iamservice.NewRoleService(db)
	permSvc := iamservice.NewPermissionService(db)
	userRoleSvc := iamservice.NewUserRoleService(db, roleSvc, userSvc)

	permCache := iamcache.NewInMemoryCache(permissionCacheTTL)
	checker := iamservice.NewPermissionChecker(userRoleSvc, roleSvc, permCache)

	authSvc := iamservice.NewAuthService(userSvc, userSvc, userRoleSvc, tokenStore, jwt)

	ssSvc := ssservice.NewService(db, userSvc)

	return &Container{
		Engine:        engine,
		User:          userSvc,
		Role:          roleSvc,
		Permission:    permSvc,
		UserRole:      userRoleSvc,
		Checker:       checker,
		Auth:          authSvc,
		SystemSetting: ssSvc,
		JWT:           jwt,
		TokenStore:    tokenStore,
	}
}

// Models 汇总所有模块的可持久化模型，供 db.SyncModels 同步表结构。
// 只有 bootstrap 知道全部模块，db 包本身对 modules/* 一无所知。
func (c *Container) Models() []any {
	return []any{
		new(usermodel.User),
		new(iammodel.Role),
		new(iammodel.Permission),
		new(iammodel.UserRole),
		new(iammodel.RolePermission),
		new(ssmodel.SystemSetting),
	}
}
