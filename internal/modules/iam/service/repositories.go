package service

import (
	"github.com/ayxworxfr/go_admin/internal/modules/iam/model"
	pkgrepo "github.com/ayxworxfr/go_admin/pkg/repository"
)

// repositories 打包 iam 模块内部用到的四个仓储。
//
// 之前这段代码单独放在 service/internal/repository 子包下，靠 Go 的
// internal 可见性规则挡住 handler 的越权 import——但这四个仓储全是对
// pkg/repository.NewRepository 的直接调用，没有任何自定义查询，从来
// 不会被 service 包以外的代码引用。unexported（小写）已经是 Go 里最强、
// 最直接的封装手段：handler 是另一个包，编译期天然看不到 repositories
// 这个类型和 newRepositories 这个函数，不需要再叠一层只有"这是个独立包"
// 时才用得上的 internal 目录。子包该不该拆，看的是"是否需要被拆出去独立
// 编译/测试/复用"，不是看"想不想加一道防线"——多一层目录不会让保护变强，
// 只会让读者多跳一次文件。
type repositories struct {
	role           *pkgrepo.Repository[model.Role]
	permission     *pkgrepo.Repository[model.Permission]
	userRole       *pkgrepo.Repository[model.UserRole]
	rolePermission *pkgrepo.Repository[model.RolePermission]
}

// newRepositories 基于同一个 *DB 构造四个仓储实例。
func newRepositories(db *pkgrepo.DB) *repositories {
	return &repositories{
		role:           pkgrepo.NewRepository[model.Role](db),
		permission:     pkgrepo.NewRepository[model.Permission](db),
		userRole:       pkgrepo.NewRepository[model.UserRole](db),
		rolePermission: pkgrepo.NewRepository[model.RolePermission](db),
	}
}
