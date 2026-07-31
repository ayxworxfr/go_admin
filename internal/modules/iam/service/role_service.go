package service

import (
	"context"
	"strings"

	"github.com/ayxworxfr/go_admin/internal/modules/iam/dto"
	"github.com/ayxworxfr/go_admin/internal/modules/iam/model"
	"github.com/ayxworxfr/go_admin/pkg/logger"
	pkgrepo "github.com/ayxworxfr/go_admin/pkg/repository"
	"github.com/hashicorp/go-multierror"
	"github.com/jinzhu/copier"
	"github.com/pkg/errors"
	"github.com/samber/lo"
	"go.uber.org/zap"
)

// RoleService 角色管理服务：只负责角色本身的 CRUD 与角色-权限分配，
// 不再像旧版 PermissionService 那样兼管用户分配和鉴权缓存——
// 那两类关注点的变更频率、性能敏感度都完全不同，混在一起正是旧版本
// 膨胀到 960 行的根因。
type RoleService struct {
	roleRepo       *pkgrepo.Repository[model.Role]
	permissionRepo *pkgrepo.Repository[model.Permission]
	rolePermRepo   *pkgrepo.Repository[model.RolePermission]
}

// NewRoleService 创建角色服务。之所以在这里而不是在 bootstrap 里调用
// newRepositories(db)，是因为 repositories 是 service 包内的
// unexported 类型，Container 只能传 *DB 进来，没有办法绕过 Service
// 直连仓储（见 repositories.go 的封装说明）。
func NewRoleService(db *pkgrepo.DB) *RoleService {
	repos := newRepositories(db)
	return &RoleService{
		roleRepo:       repos.role,
		permissionRepo: repos.permission,
		rolePermRepo:   repos.rolePermission,
	}
}

// CreateRole 创建角色（使用事务保证角色与权限分配的一致性）
func (s *RoleService) CreateRole(ctx context.Context, req *dto.CreateRoleRequest) (*dto.RoleResponse, error) {
	var role model.Role
	if err := copier.Copy(&role, req); err != nil {
		return nil, errors.Wrap(err, "failed to copy request to role")
	}

	var result dto.RoleResponse
	var permissionResponses []*dto.PermissionResponse
	err := s.roleRepo.Transaction(ctx, func(txCtx context.Context) error {
		if err := s.roleRepo.Create(txCtx, &role); err != nil {
			logger.Error(txCtx, "Failed to create role", zap.Error(err))
			return errors.Wrap(err, "failed to create role")
		}

		if len(req.PermissionIDs) > 0 {
			if err := s.AssignRolePermissions(txCtx, role.ID, req.PermissionIDs); err != nil {
				logger.Error(txCtx, "Failed to assign permissions to role", zap.Error(err), zap.Uint64("role_id", role.ID))
				return errors.Wrap(err, "failed to assign permissions to role")
			}
		}

		if err := copier.Copy(&result, &role); err != nil {
			return errors.Wrap(err, "failed to copy role to result")
		}

		permissions, err := s.RetrievePermissionByRoleID(txCtx, role.ID)
		if err != nil {
			logger.Error(txCtx, "Failed to retrieve role permissions", zap.Error(err), zap.Uint64("role_id", role.ID))
			return errors.Wrap(err, "failed to retrieve role permissions")
		}
		if err := copier.Copy(&permissionResponses, &permissions); err != nil {
			return errors.Wrap(err, "failed to copy permissions to response")
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	result.Permissions = permissionResponses
	return &result, nil
}

// UpdateRole 更新角色
func (s *RoleService) UpdateRole(ctx context.Context, req *dto.UpdateRoleRequest) (*dto.RoleResponse, error) {
	role, err := s.roleRepo.FindByID(ctx, req.ID)
	if err != nil {
		logger.Error(ctx, "Failed to retrieve role", zap.Error(err), zap.Uint64("role_id", req.ID))
		return nil, errors.Wrap(err, "failed to retrieve role")
	}

	if err := copier.Copy(role, req); err != nil {
		return nil, errors.Wrap(err, "failed to copy request to role")
	}

	if err := s.roleRepo.Update(ctx, role); err != nil {
		logger.Error(ctx, "Failed to update role", zap.Error(err), zap.Uint64("role_id", req.ID))
		return nil, errors.Wrap(err, "failed to update role")
	}

	if req.PermissionIDs != nil {
		if err := s.AssignRolePermissions(ctx, role.ID, *req.PermissionIDs); err != nil {
			logger.Error(ctx, "Failed to assign permissions to role", zap.Error(err), zap.Uint64("role_id", req.ID))
			return nil, errors.Wrap(err, "failed to assign permissions to role")
		}
	}

	var result dto.RoleResponse
	if err := copier.Copy(&result, role); err != nil {
		return nil, errors.Wrap(err, "failed to copy role to result")
	}

	permissions, err := s.RetrievePermissionByRoleID(ctx, role.ID)
	if err != nil {
		logger.Error(ctx, "Failed to retrieve role permissions", zap.Error(err), zap.Uint64("role_id", req.ID))
		return nil, errors.Wrap(err, "failed to retrieve role permissions")
	}
	if err := copier.Copy(&result.Permissions, &permissions); err != nil {
		return nil, errors.Wrap(err, "failed to copy permissions to response")
	}

	return &result, nil
}

// DeleteRoleBatch 批量删除角色，逐个删除并收集错误
func (s *RoleService) DeleteRoleBatch(ctx context.Context, ids []uint64) error {
	var result *multierror.Error
	for _, id := range ids {
		if err := s.DeleteRole(ctx, id); err != nil {
			result = multierror.Append(result, err)
		}
	}
	return result.ErrorOrNil()
}

// DeleteRole 删除角色（事务内先删角色-权限关联，再删角色本身）
func (s *RoleService) DeleteRole(ctx context.Context, id uint64) error {
	if _, err := s.roleRepo.FindByID(ctx, id); err != nil {
		logger.Error(ctx, "Failed to retrieve role", zap.Error(err), zap.Uint64("role_id", id))
		return errors.Wrap(err, "failed to retrieve role")
	}

	return s.roleRepo.Transaction(ctx, func(txCtx context.Context) error {
		if err := s.rolePermRepo.QueryBuilder().Eq("role_id", id).Delete(txCtx); err != nil {
			return errors.Wrap(err, "failed to delete role permissions")
		}
		if err := s.roleRepo.DeleteByID(txCtx, id); err != nil {
			return errors.Wrap(err, "failed to delete role")
		}
		return nil
	})
}

// GetRole 获取单个角色（附带权限列表）
func (s *RoleService) GetRole(ctx context.Context, id uint64) (*dto.RoleResponse, error) {
	role, err := s.roleRepo.FindByID(ctx, id)
	if err != nil {
		logger.Error(ctx, "Failed to retrieve role", zap.Error(err), zap.Uint64("role_id", id))
		return nil, errors.Wrap(err, "failed to retrieve role")
	}

	var result dto.RoleResponse
	if err := copier.Copy(&result, role); err != nil {
		return nil, errors.Wrap(err, "failed to copy role to result")
	}

	permissions, err := s.RetrievePermissionByRoleID(ctx, role.ID)
	if err != nil {
		logger.Error(ctx, "Failed to retrieve role permissions", zap.Error(err), zap.Uint64("role_id", id))
		return nil, errors.Wrap(err, "failed to retrieve role permissions")
	}
	if err := copier.Copy(&result.Permissions, &permissions); err != nil {
		return nil, errors.Wrap(err, "failed to copy permissions to response")
	}

	return &result, nil
}

// GetRoleList 获取角色列表，Flags 控制是否展开每个角色的权限
func (s *RoleService) GetRoleList(ctx context.Context, req *dto.GetRoleListRequest) ([]*dto.RoleResponse, int64, error) {
	roles, total, err := s.roleRepo.FindPage(ctx, req, req.Limit, req.Offset)
	if err != nil {
		logger.Error(ctx, "Failed to retrieve roles", zap.Error(err))
		return nil, 0, errors.Wrap(err, "failed to retrieve roles")
	}

	result := make([]*dto.RoleResponse, 0, len(roles))
	if err := copier.Copy(&result, &roles); err != nil {
		return nil, 0, errors.Wrap(err, "failed to copy roles to result")
	}

	if !dto.NewResponseFlags(req.Flags).Has(dto.INCLUDE_PERMISSION) {
		return result, total, nil
	}

	for i, role := range roles {
		permissions, err := s.RetrievePermissionByRoleID(ctx, role.ID)
		if err != nil {
			logger.Error(ctx, "Failed to retrieve role permissions", zap.Error(err), zap.Uint64("role_id", role.ID))
			return nil, 0, errors.Wrap(err, "failed to retrieve role permissions")
		}
		if err := copier.Copy(&result[i].Permissions, &permissions); err != nil {
			return nil, 0, errors.Wrap(err, "failed to copy permissions to response")
		}
	}

	return result, total, nil
}

// GetRolePermissions 获取角色的权限列表
func (s *RoleService) GetRolePermissions(ctx context.Context, roleID uint64) ([]*dto.PermissionResponse, error) {
	permissions, err := s.RetrievePermissionByRoleID(ctx, roleID)
	if err != nil {
		logger.Error(ctx, "Failed to retrieve role permissions", zap.Error(err), zap.Uint64("role_id", roleID))
		return nil, errors.Wrap(err, "failed to retrieve role permissions")
	}

	var result []*dto.PermissionResponse
	if err := copier.Copy(&result, &permissions); err != nil {
		return nil, errors.Wrap(err, "failed to copy permissions to result")
	}
	return result, nil
}

// AssignRolePermissions 为角色分配权限：对比新旧权限集合，只做增量的删除与新增
func (s *RoleService) AssignRolePermissions(ctx context.Context, roleID uint64, permissionIDs []uint64) error {
	current, err := s.rolePermRepo.FindAll(ctx, &model.RolePermission{RoleID: roleID})
	if err != nil {
		return errors.Wrap(err, "failed to retrieve role permissions")
	}

	existingIDs := lo.Map(current, func(rp model.RolePermission, _ int) uint64 { return rp.PermissionID })
	toRemoveIDs := lo.Filter(existingIDs, func(id uint64, _ int) bool { return !lo.Contains(permissionIDs, id) })
	toAddIDs := lo.Filter(permissionIDs, func(id uint64, _ int) bool { return !lo.Contains(existingIDs, id) })

	return s.rolePermRepo.Transaction(ctx, func(txCtx context.Context) error {
		if len(toRemoveIDs) > 0 {
			if err := s.rolePermRepo.QueryBuilder().Eq("role_id", roleID).In("permission_id", toRemoveIDs).Delete(txCtx); err != nil {
				return errors.Wrap(err, "failed to delete role permissions")
			}
		}
		if len(toAddIDs) > 0 {
			rolePermissions := lo.Map(toAddIDs, func(permissionID uint64, _ int) model.RolePermission {
				return model.RolePermission{RoleID: roleID, PermissionID: permissionID}
			})
			if err := s.rolePermRepo.BatchCreate(txCtx, rolePermissions); err != nil {
				return errors.Wrap(err, "failed to create role permissions")
			}
		}
		return nil
	})
}

// RetrievePermissionByRoleID 通过角色 ID 查询关联权限，是 RoleService 对外
// 暴露的读接口——iam 内的 UserRoleService/PermissionChecker 都组合本服务
// 来复用这段查询逻辑，而不是各自重新连表查询。
func (s *RoleService) RetrievePermissionByRoleID(ctx context.Context, roleID uint64) ([]model.Permission, error) {
	rolePermissions, err := s.rolePermRepo.QueryBuilder().Eq("role_id", roleID).Find(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "query RolePermission failed")
	}
	if len(rolePermissions) == 0 {
		return []model.Permission{}, nil
	}

	permissionIDs := lo.Map(rolePermissions, func(rp model.RolePermission, _ int) uint64 { return rp.PermissionID })
	permissions, err := s.permissionRepo.QueryBuilder().In("id", permissionIDs).Find(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "query Permission failed")
	}
	return permissions, nil
}

// getChildPermissionsWithCTE 使用递归 CTE 查询所有子权限（供 PermissionChecker 复用）
func (s *RoleService) getChildPermissionsWithCTE(ctx context.Context, parentIDs []uint64) ([]model.Permission, error) {
	if len(parentIDs) == 0 {
		return []model.Permission{}, nil
	}

	placeholders := make([]string, len(parentIDs))
	for i := range placeholders {
		placeholders[i] = "?"
	}

	query := `
	WITH RECURSIVE permission_tree AS (
		SELECT id, parent_id
		FROM permission
		WHERE id IN (` + strings.Join(placeholders, ", ") + `)
		UNION ALL
		SELECT p.id, p.parent_id
		FROM permission p
		JOIN permission_tree pt ON p.parent_id = pt.id
	)
	SELECT p.* FROM permission p
	JOIN permission_tree pt ON p.id = pt.id
	`

	childPermissions, err := s.permissionRepo.Query(ctx, query, lo.ToAnySlice(parentIDs)...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to retrieve child permissions with CTE")
	}
	return childPermissions, nil
}
