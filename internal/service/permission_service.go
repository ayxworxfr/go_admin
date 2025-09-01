package service

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/ayxworxfr/go_admin/internal/dao"
	"github.com/ayxworxfr/go_admin/internal/domain/models"
	"github.com/ayxworxfr/go_admin/internal/domain/params"
	"github.com/ayxworxfr/go_admin/internal/domain/vo"
	"github.com/ayxworxfr/go_admin/pkg/logger"
	"github.com/ayxworxfr/go_admin/pkg/repository"
	"github.com/hashicorp/go-multierror"
	"github.com/jinzhu/copier"
	"github.com/pkg/errors"
	"github.com/samber/lo"
	"go.uber.org/zap"
)

// PermissionService 权限与角色管理服务 - 负责权限控制和角色管理
type PermissionService struct {
	// 数据访问对象
	roleRepo       repository.Repository[models.Role]
	permissionRepo repository.Repository[models.Permission]
	userRoleRepo   repository.Repository[models.UserRole]
	rolePermRepo   repository.Repository[models.RolePermission]

	// 缓存相关
	permissionCache     map[uint64]map[string]bool // 用户ID -> 权限路径映射
	permissionTreeCache map[uint64][]vo.Permission // 用户ID -> 权限树
	cacheMutex          sync.RWMutex               // 缓存锁
	cacheExpiration     time.Duration              // 缓存过期时间
}

// NewPermissionService 创建权限服务实例
func NewPermissionService() *PermissionService {
	return &PermissionService{
		roleRepo:            dao.RoleRepo,
		permissionRepo:      dao.PermissionRepo,
		userRoleRepo:        dao.UserRoleRepo,
		rolePermRepo:        dao.RolePermissionRepo,
		permissionCache:     make(map[uint64]map[string]bool),
		permissionTreeCache: make(map[uint64][]vo.Permission),
		cacheExpiration:     1 * time.Hour,
	}
}

// --------------------------- 权限管理 ---------------------------

// UpdateRole 更新角色
func (s *PermissionService) UpdateRole(ctx context.Context, req *params.UpdateRoleRequest) (*vo.Role, error) {
	role, err := s.roleRepo.FindByID(ctx, req.ID)
	if err != nil {
		logger.Error(ctx, "Failed to retrieve role", zap.Error(err), zap.Uint64("role_id", req.ID))
		return nil, errors.Wrap(err, "failed to retrieve role")
	}

	if err := copier.Copy(&role, &req); err != nil {
		return nil, errors.Wrap(err, "failed to copy request to role")
	}

	if err := s.roleRepo.Update(ctx, role); err != nil {
		logger.Error(ctx, "Failed to update role", zap.Error(err), zap.Uint64("role_id", req.ID))
		return nil, errors.Wrap(err, "failed to update role")
	}

	// 分配权限
	if req.PermissionIDs != nil {
		if err := s.AssignRolePermissions(ctx, role.ID, *req.PermissionIDs); err != nil {
			logger.Error(ctx, "Failed to assign permissions to role", zap.Error(err), zap.Uint64("role_id", req.ID))
			return nil, errors.Wrap(err, "failed to assign permissions to role")
		}
	}

	var result vo.Role
	if err := copier.Copy(&result, &role); err != nil {
		return nil, errors.Wrap(err, "failed to copy role to result")
	}

	// 获取角色的权限
	permissions, err := s.RetrievePermissionByRoleID(ctx, role.ID)
	if err != nil {
		logger.Error(ctx, "Failed to retrieve role permissions", zap.Error(err), zap.Uint64("role_id", req.ID))
		return nil, errors.Wrap(err, "failed to retrieve role permissions")
	}

	var permissionVOs []*vo.Permission
	if err := copier.Copy(&permissionVOs, &permissions); err != nil {
		return nil, errors.Wrap(err, "failed to copy permissions to permissionVOs")
	}

	result.Permissions = permissionVOs

	// 清除相关缓存
	s.ClearAllPermissionCache()
	return &result, nil
}

func (s *PermissionService) DeleteRoleBatch(ctx context.Context, ids []uint64) error {
	var errs multierror.Error
	for _, id := range ids {
		if err := s.DeleteRole(ctx, id); err != nil {
			errs.Errors = append(errs.Errors, err)
		}
	}
	return errs.ErrorOrNil()
}

// DeleteRole 删除角色
func (s *PermissionService) DeleteRole(ctx context.Context, id uint64) error {
	_, err := s.roleRepo.FindByID(ctx, id)
	if err != nil {
		logger.Error(ctx, "Failed to retrieve role", zap.Error(err), zap.Uint64("role_id", id))
		return errors.Wrap(err, "failed to retrieve role")
	}

	// 事务处理：先删除角色权限关联，再删除角色
	_, err = s.roleRepo.Transaction(ctx, func(txCtx context.Context) (any, error) {
		// 删除角色权限关联
		if err := s.rolePermRepo.QueryBuilder().
			Eq("role_id", id).
			Delete(txCtx); err != nil {
			return nil, errors.Wrap(err, "failed to delete role permissions")
		}

		// 删除角色
		if err := s.roleRepo.DeleteByID(txCtx, id); err != nil {
			return nil, errors.Wrap(err, "failed to delete role")
		}

		return nil, nil
	})

	// 清除相关缓存
	if err == nil {
		s.ClearAllPermissionCache()
	}
	return err
}

// CreatePermissions 批量创建权限
func (s *PermissionService) CreatePermissions(ctx context.Context, req *params.CreatePermissionsRequest) error {
	var permissions []models.Permission
	if err := copier.Copy(&permissions, &req.Permissions); err != nil {
		return errors.Wrap(err, "failed to copy requests to permissions")
	}

	if err := s.permissionRepo.BatchCreate(ctx, permissions); err != nil {
		logger.Error(ctx, "Failed to create permissions", zap.Error(err))
		return errors.Wrap(err, "failed to create permissions")
	}

	// 清除相关缓存
	s.ClearAllPermissionCache()
	return nil
}

// GetUserRoles 获取用户的角色列表
func (s *PermissionService) GetUserRoles(ctx context.Context, userID uint64) (*vo.User, error) {
	return s.GetUserRolesByFlags(ctx, userID, params.ALL_AUTH_FLAGS)
}

// GetUserRoles 获取用户的角色列表
func (s *PermissionService) GetUserRolesByFlags(ctx context.Context, userID uint64, flags int) (*vo.User, error) {
	user, err := dao.UserRepo.FindByID(ctx, userID)
	if err != nil {
		logger.Error(ctx, "Failed to retrieve user", zap.Error(err), zap.Uint64("user_id", userID))
		return nil, errors.Wrap(err, "failed to retrieve user")
	}

	var result vo.User
	if err := copier.Copy(&result, &user); err != nil {
		return nil, errors.Wrap(err, "failed to copy user to result")
	}

	searchFlag := params.NewResponseFlags(flags)
	if !searchFlag.Has(params.INCLUDE_ROLE) {
		return &result, nil
	}

	roles, err := s.RetrieveRoleVosByUserIDByFlags(ctx, userID, flags)
	if err != nil {
		logger.Error(ctx, "Failed to retrieve user roles", zap.Error(err), zap.Uint64("user_id", userID))
		return nil, errors.Wrap(err, "failed to retrieve user roles")
	}

	result.Roles = roles

	return &result, nil
}
func (s *PermissionService) DeletePermissionBatch(ctx context.Context, ids []uint64) error {
	sql := `delete from permission where id IN (?` + strings.Repeat(",?", len(ids)-1) + `)`
	_, err := s.permissionRepo.Exec(ctx, sql, lo.ToAnySlice(ids)...)
	return err
}

// DeletePermission 删除权限
func (s *PermissionService) DeletePermission(ctx context.Context, id uint64) error {
	_, err := s.permissionRepo.FindByID(ctx, id)
	if err != nil {
		logger.Error(ctx, "Failed to retrieve permission", zap.Error(err), zap.Uint64("permission_id", id))
		return errors.Wrap(err, "failed to retrieve permission")
	}

	if err := s.permissionRepo.DeleteByID(ctx, id); err != nil {
		logger.Error(ctx, "Failed to delete permission", zap.Error(err), zap.Uint64("permission_id", id))
		return errors.Wrap(err, "failed to delete permission")
	}

	// 清除相关缓存
	s.ClearAllPermissionCache()
	return nil
}

// GetPermission 获取单个权限
func (s *PermissionService) GetPermission(ctx context.Context, id uint64) (*vo.Permission, error) {
	permission, err := s.permissionRepo.FindByID(ctx, id)
	if err != nil {
		logger.Error(ctx, "Failed to retrieve permission", zap.Error(err), zap.Uint64("permission_id", id))
		return nil, errors.Wrap(err, "failed to retrieve permission")
	}

	var result vo.Permission
	if err := copier.Copy(&result, &permission); err != nil {
		return nil, errors.Wrap(err, "failed to copy permission to result")
	}

	return &result, nil
}

// GetPermissionList 获取权限列表
func (s *PermissionService) GetPermissionList(ctx context.Context, req *params.GetPermissionListRequest) ([]vo.Permission, int64, error) {
	permissions, total, err := s.permissionRepo.FindPage(ctx, req, req.Limit, req.Offset)
	if err != nil {
		logger.Error(ctx, "Failed to retrieve permissions", zap.Error(err))
		return nil, 0, errors.Wrap(err, "failed to retrieve permissions")
	}

	var result []vo.Permission
	if err := copier.Copy(&result, &permissions); err != nil {
		return nil, 0, errors.Wrap(err, "failed to copy permissions to result")
	}

	return result, total, nil
}

// GetRole 获取单个角色
func (s *PermissionService) GetRole(ctx context.Context, id uint64) (*vo.Role, error) {
	role, err := s.roleRepo.FindByID(ctx, id)
	if err != nil {
		logger.Error(ctx, "Failed to retrieve role", zap.Error(err), zap.Uint64("role_id", id))
		return nil, errors.Wrap(err, "failed to retrieve role")
	}

	var result vo.Role
	if err := copier.Copy(&result, &role); err != nil {
		return nil, errors.Wrap(err, "failed to copy role to result")
	}

	// 获取角色的权限
	permissions, err := s.RetrievePermissionByRoleID(ctx, role.ID)
	if err != nil {
		logger.Error(ctx, "Failed to retrieve role permissions", zap.Error(err), zap.Uint64("role_id", id))
		return nil, errors.Wrap(err, "failed to retrieve role permissions")
	}

	var permissionVOs []*vo.Permission
	if err := copier.Copy(&permissionVOs, &permissions); err != nil {
		return nil, errors.Wrap(err, "failed to copy permissions to permissionVOs")
	}

	result.Permissions = permissionVOs

	return &result, nil
}

// GetRoleList 获取角色列表
func (s *PermissionService) GetRoleList(ctx context.Context, req *params.GetRoleListRequest) ([]vo.Role, int64, error) {
	roles, total, err := s.roleRepo.FindPage(ctx, req, req.Limit, req.Offset)
	if err != nil {
		logger.Error(ctx, "Failed to retrieve roles", zap.Error(err))
		return nil, 0, errors.Wrap(err, "failed to retrieve roles")
	}

	var result []vo.Role
	if err := copier.Copy(&result, &roles); err != nil {
		return nil, 0, errors.Wrap(err, "failed to copy roles to result")
	}

	searchFlag := params.NewResponseFlags(req.Flags)
	if !searchFlag.Has(params.INCLUDE_PERMISSION) {
		return result, total, nil
	}

	// 获取每个角色的权限
	for i, role := range roles {
		permissions, err := s.RetrievePermissionByRoleID(ctx, role.ID)
		if err != nil {
			logger.Error(ctx, "Failed to retrieve role permissions", zap.Error(err), zap.Uint64("role_id", role.ID))
			return nil, 0, errors.Wrap(err, "failed to retrieve role permissions")
		}

		var permissionVOs []*vo.Permission
		if err := copier.Copy(&permissionVOs, &permissions); err != nil {
			return nil, 0, errors.Wrap(err, "failed to copy permissions to permissionVOs")
		}

		result[i].Permissions = permissionVOs
	}

	return result, total, nil
}

// GetRolePermissions 获取角色的权限列表
func (s *PermissionService) GetRolePermissions(ctx context.Context, roleID uint64) ([]vo.Permission, error) {
	permissions, err := s.RetrievePermissionByRoleID(ctx, roleID)
	if err != nil {
		logger.Error(ctx, "Failed to retrieve role permissions", zap.Error(err), zap.Uint64("role_id", roleID))
		return nil, errors.Wrap(err, "failed to retrieve role permissions")
	}

	var result []vo.Permission
	if err := copier.Copy(&result, &permissions); err != nil {
		return nil, errors.Wrap(err, "failed to copy permissions to result")
	}

	return result, nil
}

// CreatePermission 创建权限
func (s *PermissionService) CreatePermission(ctx context.Context, req *params.CreatePermissionRequest) (*vo.Permission, error) {
	var permission models.Permission
	if err := copier.Copy(&permission, &req); err != nil {
		return nil, errors.Wrap(err, "failed to copy request to permission")
	}

	if err := s.permissionRepo.Create(ctx, &permission); err != nil {
		logger.Error(ctx, "Failed to create permission", zap.Error(err))
		return nil, errors.Wrap(err, "failed to create permission")
	}

	var result vo.Permission
	if err := copier.Copy(&result, &permission); err != nil {
		return nil, errors.Wrap(err, "failed to copy permission to result")
	}

	// 清除相关缓存
	s.ClearAllPermissionCache()
	return &result, nil
}

// UpdatePermission 更新权限
func (s *PermissionService) UpdatePermission(ctx context.Context, req *params.UpdatePermissionRequest) (*vo.Permission, error) {
	permission, err := s.permissionRepo.FindByID(ctx, req.ID)
	if err != nil {
		logger.Error(ctx, "Failed to retrieve permission", zap.Error(err), zap.Uint64("permission_id", req.ID))
		return nil, errors.Wrap(err, "failed to retrieve permission")
	}

	if err := copier.Copy(&permission, &req); err != nil {
		return nil, errors.Wrap(err, "failed to copy request to permission")
	}

	if err := s.permissionRepo.Update(ctx, permission); err != nil {
		logger.Error(ctx, "Failed to update permission", zap.Error(err), zap.Uint64("permission_id", req.ID))
		return nil, errors.Wrap(err, "failed to update permission")
	}

	var result vo.Permission
	if err := copier.Copy(&result, &permission); err != nil {
		return nil, errors.Wrap(err, "failed to copy permission to result")
	}

	// 清除相关缓存
	s.ClearAllPermissionCache()
	return &result, nil
}

// --------------------------- 角色管理 ---------------------------

// CreateRole 创建角色（使用事务保证数据一致性）
func (s *PermissionService) CreateRole(ctx context.Context, req *params.CreateRoleRequest) (*vo.Role, error) {
	var role models.Role
	if err := copier.Copy(&role, &req); err != nil {
		return nil, errors.Wrap(err, "failed to copy request to role")
	}

	var result vo.Role
	var permissionVOs []*vo.Permission
	_, err := s.roleRepo.Transaction(ctx, func(txCtx context.Context) (any, error) {
		if err := s.roleRepo.Create(txCtx, &role); err != nil {
			logger.Error(txCtx, "Failed to create role", zap.Error(err))
			return nil, errors.Wrap(err, "failed to create role")
		}

		// 分配权限
		if len(req.PermissionIDs) > 0 {
			if err := s.AssignRolePermissions(txCtx, role.ID, req.PermissionIDs); err != nil {
				logger.Error(txCtx, "Failed to assign permissions to role", zap.Error(err), zap.Uint64("role_id", role.ID))
				return nil, errors.Wrap(err, "failed to assign permissions to role")
			}
		}

		if err := copier.Copy(&result, &role); err != nil {
			return nil, errors.Wrap(err, "failed to copy role to result")
		}

		// 获取角色的权限
		permissions, err := s.RetrievePermissionByRoleID(txCtx, role.ID)
		if err != nil {
			logger.Error(txCtx, "Failed to retrieve role permissions", zap.Error(err), zap.Uint64("role_id", role.ID))
			return nil, errors.Wrap(err, "failed to retrieve role permissions")
		}

		if err := copier.Copy(&permissionVOs, &permissions); err != nil {
			return nil, errors.Wrap(err, "failed to copy permissions to permissionVOs")
		}
		return nil, nil
	})
	if err != nil {
		return nil, err
	}
	result.Permissions = permissionVOs

	// 清除相关缓存
	s.ClearAllPermissionCache()
	return &result, nil
}

// AssignRolePermissions 为角色分配权限（使用位标志优化）
func (s *PermissionService) AssignRolePermissions(ctx context.Context, roleID uint64, permissionIDs []uint64) error {
	// 1. 查询当前角色已分配的权限
	rolePermissionList, err := s.rolePermRepo.FindAll(ctx, &models.RolePermission{RoleID: roleID})
	if err != nil {
		return errors.Wrap(err, "failed to retrieve role permissions")
	}

	// 2. 计算需要删除和新增的权限ID
	existingPermissionIDs := lo.Map(rolePermissionList, func(rp models.RolePermission, _ int) uint64 {
		return rp.PermissionID
	})
	toRemoveIDs := lo.Filter(existingPermissionIDs, func(id uint64, _ int) bool {
		return !lo.Contains(permissionIDs, id)
	})
	toAddIDs := lo.Filter(permissionIDs, func(id uint64, _ int) bool {
		return !lo.Contains(existingPermissionIDs, id)
	})

	// 3. 事务处理
	_, err = s.rolePermRepo.Transaction(ctx, func(txCtx context.Context) (any, error) {
		// 4.1 删除旧关联
		if len(toRemoveIDs) > 0 {
			err := s.rolePermRepo.QueryBuilder().
				Eq("role_id", roleID).
				In("permission_id", toRemoveIDs).
				Delete(txCtx)
			if err != nil {
				return nil, errors.Wrap(err, "failed to delete role permissions")
			}
		}

		// 4.2 创建新关联
		if len(toAddIDs) > 0 {
			rolePermissions := lo.Map(toAddIDs, func(permissionID uint64, _ int) models.RolePermission {
				return models.RolePermission{
					RoleID:       roleID,
					PermissionID: permissionID,
				}
			})
			err := s.rolePermRepo.BatchCreate(txCtx, rolePermissions)
			if err != nil {
				return nil, errors.Wrap(err, "failed to create role permissions")
			}
		}

		return nil, nil
	})

	// 清除相关缓存
	if err == nil {
		s.ClearAllPermissionCache()
	}
	return err
}

// --------------------------- 用户权限管理 ---------------------------

// AssignRoles 为用户分配角色
func (s *PermissionService) AssignRoles(ctx context.Context, userID uint64, roleIDs []uint64) error {
	// 检查用户是否存在
	_, err := dao.UserRepo.FindByID(ctx, userID)
	if err != nil {
		logger.Error(ctx, "Failed to retrieve user", zap.Error(err), zap.Uint64("user_id", userID))
		return errors.Wrap(err, "failed to retrieve user")
	}

	err = s.AssignUserRoles(ctx, userID, roleIDs)
	if err == nil {
		// 清除用户缓存
		s.ClearUserPermissionCache(userID)
	}
	return err
}

// AssignUserRoles 为用户分配角色（包含事务逻辑）
func (s *PermissionService) AssignUserRoles(ctx context.Context, userID uint64, roleIDs []uint64) error {
	// 1. 查询当前用户已分配的角色
	userRoleList, err := s.userRoleRepo.FindAll(ctx, &models.UserRole{UserID: userID})
	if err != nil {
		return errors.Wrap(err, "failed to retrieve user roles")
	}

	// 2. 计算需要删除和新增的角色ID
	existingRoleIDs := lo.Map(userRoleList, func(ur models.UserRole, _ int) uint64 {
		return ur.RoleID
	})
	toRemoveIDs := lo.Filter(existingRoleIDs, func(id uint64, _ int) bool {
		return !lo.Contains(roleIDs, id)
	})
	toAddIDs := lo.Filter(roleIDs, func(id uint64, _ int) bool {
		return !lo.Contains(existingRoleIDs, id)
	})

	// 3. 事务处理
	_, err = s.userRoleRepo.Transaction(ctx, func(txCtx context.Context) (any, error) {
		// 3.1 删除旧关联
		if len(toRemoveIDs) > 0 {
			err := s.userRoleRepo.QueryBuilder().
				Eq("user_id", userID).
				In("role_id", toRemoveIDs).
				Delete(txCtx)
			if err != nil {
				return nil, errors.Wrap(err, "failed to delete user roles")
			}
		}

		// 3.2 创建新关联
		if len(toAddIDs) > 0 {
			userRoles := make([]models.UserRole, len(toAddIDs))
			for i, roleID := range toAddIDs {
				userRoles[i] = models.UserRole{
					UserID: userID,
					RoleID: roleID,
				}
			}
			err := s.userRoleRepo.BatchCreate(txCtx, userRoles)
			if err != nil {
				return nil, errors.Wrap(err, "failed to create user roles")
			}
		}

		return nil, nil
	})

	// 清除用户缓存
	if err == nil {
		s.ClearUserPermissionCache(userID)
	}
	return err
}

// --------------------------- 权限检查与缓存 ---------------------------

// RetrieveRoleVosByUserID 通过用户ID查询关联角色VO
func (s *PermissionService) RetrieveRoleVosByUserID(ctx context.Context, userID uint64) ([]*vo.Role, error) {
	return s.RetrieveRoleVosByUserIDByFlags(ctx, userID, params.ALL_AUTH_FLAGS)
}

// RetrieveRoleVosByUserID 通过用户ID查询关联角色VO
func (s *PermissionService) RetrieveRoleVosByUserIDByFlags(ctx context.Context, userID uint64, flags int) ([]*vo.Role, error) {
	roles, err := s.RetrieveRolesByUserID(ctx, userID)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to retrieve user roles")
	}

	var result []*vo.Role
	if err := copier.Copy(&result, &roles); err != nil {
		return nil, errors.Wrap(err, "Failed to copy user roles")
	}

	searchFlag := params.NewResponseFlags(flags)
	if !searchFlag.Has(params.INCLUDE_PERMISSION) {
		return result, nil
	}

	// 获取每个角色的权限
	for i, role := range roles {
		permissions, err := s.RetrievePermissionByRoleID(ctx, role.ID)
		if err != nil {
			logger.Error(ctx, "Failed to retrieve role permissions", zap.Error(err), zap.Uint64("role_id", role.ID))
			return nil, errors.Wrap(err, "Failed to retrieve role permissions")
		}

		var permissionVOs []*vo.Permission
		if err := copier.Copy(&permissionVOs, &permissions); err != nil {
			return nil, errors.Wrap(err, "Failed to copy role permissions")
		}

		result[i].Permissions = permissionVOs
	}
	return result, nil
}

// RetrieveRolesByUserID 通过用户ID查询关联角色
func (s *PermissionService) RetrieveRolesByUserID(ctx context.Context, userID uint64) ([]models.Role, error) {
	// 1. 查询UserRole表
	userRoles, err := s.userRoleRepo.QueryBuilder().
		Eq("user_id", userID).
		Find(ctx)
	if err != nil {
		return nil, fmt.Errorf("query UserRole failed: %w", err)
	}

	if len(userRoles) == 0 {
		return []models.Role{}, nil
	}

	// 2. 提取角色ID
	roleIDs := lo.Map(userRoles, func(ur models.UserRole, _ int) uint64 {
		return ur.RoleID
	})

	// 3. 查询Role表
	roles, err := s.roleRepo.QueryBuilder().
		In("id", roleIDs).
		Find(ctx)
	if err != nil {
		return nil, fmt.Errorf("query Role failed: %w", err)
	}

	return roles, nil
}

// RetrievePermissionByRoleID 通过角色ID查询关联权限
func (s *PermissionService) RetrievePermissionByRoleID(ctx context.Context, roleID uint64) ([]models.Permission, error) {
	// 1. 查询RolePermission表
	rolePermissions, err := s.rolePermRepo.QueryBuilder().
		Eq("role_id", roleID).
		Find(ctx)
	if err != nil {
		return nil, fmt.Errorf("query RolePermission failed: %w", err)
	}

	if len(rolePermissions) == 0 {
		return []models.Permission{}, nil
	}

	// 2. 提取权限ID
	permissionIDs := lo.Map(rolePermissions, func(rp models.RolePermission, _ int) uint64 {
		return rp.PermissionID
	})

	// 3. 查询Permission表
	permissions, err := s.permissionRepo.QueryBuilder().
		In("id", permissionIDs).
		Find(ctx)
	if err != nil {
		return nil, fmt.Errorf("query Permission failed: %w", err)
	}

	return permissions, nil
}

// GetUserPermissions 获取用户的权限列表
func (s *PermissionService) GetUserPermissions(ctx context.Context, userID uint64) ([]vo.Permission, error) {
	// 检查用户是否存在
	_, err := dao.UserRepo.FindByID(ctx, userID)
	if err != nil {
		logger.Error(ctx, "Failed to retrieve user", zap.Error(err), zap.Uint64("user_id", userID))
		return nil, errors.Wrap(err, "failed to retrieve user")
	}

	// 获取用户角色
	roles, err := s.RetrieveRolesByUserID(ctx, userID)
	if err != nil {
		logger.Error(ctx, "Failed to retrieve user roles", zap.Error(err), zap.Uint64("user_id", userID))
		return nil, errors.Wrap(err, "failed to retrieve user roles")
	}

	// 收集所有角色的权限
	var permissions []models.Permission
	for _, role := range roles {
		perms, err := s.RetrievePermissionByRoleID(ctx, role.ID)
		if err != nil {
			logger.Error(ctx, "Failed to retrieve role permissions", zap.Error(err), zap.Uint64("role_id", role.ID))
			return nil, errors.Wrap(err, "failed to retrieve role permissions")
		}
		permissions = append(permissions, perms...)
	}

	var result []vo.Permission
	if err := copier.Copy(&result, &permissions); err != nil {
		return nil, errors.Wrap(err, "failed to copy permissions to result")
	}

	return result, nil
}

// GetUserAllPermissions 获取用户的所有权限，包括子权限（带缓存）
func (s *PermissionService) GetUserAllPermissions(ctx context.Context, userID uint64) ([]vo.Permission, error) {
	// 检查缓存
	s.cacheMutex.RLock()
	permissions, exists := s.permissionTreeCache[userID]
	s.cacheMutex.RUnlock()

	if exists {
		// 复制一份返回，避免外部修改影响缓存
		var result []vo.Permission
		copier.Copy(&result, &permissions)
		return result, nil
	}

	// 缓存未命中，查询数据库
	permissions, err := s.getUserAllPermissionsCTE(ctx, userID)
	if err != nil {
		return nil, err
	}

	// 写入缓存
	s.cacheMutex.Lock()
	s.permissionTreeCache[userID] = permissions
	s.cacheMutex.Unlock()

	return permissions, nil
}

// getUserAllPermissionsCTE 使用CTE查询用户所有权限（包括子权限）
func (s *PermissionService) getUserAllPermissionsCTE(ctx context.Context, userID uint64) ([]vo.Permission, error) {
	// 获取用户角色
	roles, err := s.RetrieveRolesByUserID(ctx, userID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to retrieve user roles")
	}

	if len(roles) == 0 {
		return []vo.Permission{}, nil
	}

	// 收集角色ID
	roleIDs := lo.Map(roles, func(role models.Role, _ int) uint64 {
		return role.ID
	})

	// 查询角色权限关系
	rolePermissions, err := s.rolePermRepo.QueryBuilder().
		In("role_id", roleIDs).
		Find(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to retrieve role permissions")
	}

	if len(rolePermissions) == 0 {
		return []vo.Permission{}, nil
	}

	// 收集权限ID
	permissionIDs := lo.Map(rolePermissions, func(rp models.RolePermission, _ int) uint64 {
		return rp.PermissionID
	})

	// 查询权限详情
	permissions, err := s.permissionRepo.QueryBuilder().
		In("id", permissionIDs).
		Find(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to retrieve permissions")
	}

	// 转换为VO
	var result []vo.Permission
	if err := copier.Copy(&result, &permissions); err != nil {
		return nil, errors.Wrap(err, "failed to copy permissions to VO")
	}

	// 收集父权限ID
	parentIDs := lo.Map(permissions, func(perm models.Permission, _ int) uint64 {
		return perm.ID
	})

	// 使用CTE获取所有子权限（需MySQL 8.0+）
	childPermissions, err := s.GetChildPermissionsWithCTE(ctx, parentIDs)
	if err != nil {
		return nil, errors.Wrap(err, "failed to retrieve child permissions")
	}

	// 转换子权限
	var childResult []vo.Permission
	if err := copier.Copy(&childResult, &childPermissions); err != nil {
		return nil, errors.Wrap(err, "failed to copy child permissions to VO")
	}

	result = append(result, childResult...)

	// 去重
	uniqueResult := lo.UniqBy(result, func(p vo.Permission) uint64 {
		return p.ID
	})

	return uniqueResult, nil
}

// GetChildPermissionsWithCTE 使用CTE查询所有子权限
func (s *PermissionService) GetChildPermissionsWithCTE(ctx context.Context, parentIDs []uint64) ([]models.Permission, error) {
	if len(parentIDs) == 0 {
		return []models.Permission{}, nil
	}

	// 构建IN条件占位符
	placeholders := make([]string, len(parentIDs))
	for i := range placeholders {
		placeholders[i] = "?"
	}

	// MySQL 8.0+ CTE查询
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

// checkPathPermission 检查用户是否拥有指定路径的权限（使用位标志优化）
func (s *PermissionService) checkPathPermission(permissionMap map[string]bool, method, path string) bool {
	// 检查直接匹配
	if permissionMap[method+":"+path] {
		return true
	}

	// 检查路径通配符匹配
	for permKey := range permissionMap {
		permParts := strings.SplitN(permKey, ":", 2)
		if len(permParts) != 2 {
			continue
		}

		permMethod := permParts[0]
		permPath := permParts[1]

		// 方法必须匹配（支持通配符*）
		if permMethod != method && permMethod != "*" {
			continue
		}

		// 检查路径是否匹配（支持尾部通配符）
		if strings.HasSuffix(permPath, "/*") {
			patternPrefix := strings.TrimSuffix(permPath, "/*")
			if strings.HasPrefix(path, patternPrefix) {
				return true
			}
		}
	}

	return false
}

// GetUserPermissionPaths 获取用户有权限的所有路径
func (s *PermissionService) GetUserPermissionPaths(ctx context.Context, userID uint64) ([]string, error) {
	// 获取用户的所有权限
	permissions, err := s.GetUserAllPermissions(ctx, userID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to retrieve user permissions")
	}

	// 提取权限路径
	paths := make([]string, 0, len(permissions))
	for _, perm := range permissions {
		if perm.Method != "" && perm.Path != "" {
			paths = append(paths, perm.Method+":"+perm.Path)
		}
	}

	return paths, nil
}

// HasPermission 检查用户是否拥有指定路径的权限（带缓存）
func (s *PermissionService) HasPermission(ctx context.Context, userID uint64, method, path string) (bool, error) {
	// 1. 检查缓存
	s.cacheMutex.RLock()
	permissionMap, exists := s.permissionCache[userID]
	s.cacheMutex.RUnlock()

	if exists {
		return s.checkPathPermission(permissionMap, method, path), nil
	}

	// 2. 缓存未命中，查询数据库
	permissions, err := s.GetUserAllPermissions(ctx, userID)
	if err != nil {
		logger.Error(ctx, "Failed to retrieve user permissions", zap.Error(err), zap.Uint64("user_id", userID))
		return false, errors.Wrap(err, "failed to retrieve user permissions")
	}

	// 3. 构建权限映射并缓存
	permissionMap = make(map[string]bool)
	for _, perm := range permissions {
		if perm.Method != "" && perm.Path != "" {
			permissionMap[perm.Method+":"+perm.Path] = true
		}
	}

	// 4. 写入缓存
	s.cacheMutex.Lock()
	s.permissionCache[userID] = permissionMap
	s.cacheMutex.Unlock()

	return s.checkPathPermission(permissionMap, method, path), nil
}

// --------------------------- 辅助方法 ---------------------------

// fetchHighestPriorityRole 获取优先级最高的角色（假设ID越小优先级越高）
func (s *PermissionService) fetchHighestPriorityRole(roles []*vo.Role) *vo.Role {
	if len(roles) == 0 {
		return nil
	}

	highestPriority := roles[0]
	for _, role := range roles {
		if role.ID < highestPriority.ID {
			highestPriority = role
		}
	}
	return highestPriority
}

// --------------------------- 缓存管理 ---------------------------

// ClearUserPermissionCache 清除用户权限缓存
func (s *PermissionService) ClearUserPermissionCache(userID uint64) {
	s.cacheMutex.Lock()
	delete(s.permissionCache, userID)
	delete(s.permissionTreeCache, userID)
	s.cacheMutex.Unlock()
}

// ClearAllPermissionCache 清除所有用户权限缓存
func (s *PermissionService) ClearAllPermissionCache() {
	s.cacheMutex.Lock()
	s.permissionCache = make(map[uint64]map[string]bool)
	s.permissionTreeCache = make(map[uint64][]vo.Permission)
	s.cacheMutex.Unlock()
}
