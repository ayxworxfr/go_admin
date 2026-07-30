package service

import (
	"context"

	"github.com/ayxworxfr/go_admin/internal/modules/iam/dto"
	"github.com/ayxworxfr/go_admin/internal/modules/iam/model"
	"github.com/ayxworxfr/go_admin/pkg/logger"
	pkgrepo "github.com/ayxworxfr/go_admin/pkg/repository"
	"github.com/jinzhu/copier"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

// PermissionService 权限元数据管理服务：只做 Permission 的 CRUD，
// 不再兼管"用户是否有权限"的判断——那是鉴权热路径，交给 PermissionChecker。
// 两者虽然都叫"权限"，但一个是管理台配置操作，一个是每次请求都要走的判断逻辑，
// 变更频率和性能要求完全不同，这正是拆分的依据。
type PermissionService struct {
	permissionRepo pkgrepo.Repository[model.Permission]
}

// NewPermissionService 创建权限元数据服务
func NewPermissionService(processor pkgrepo.ORMProcessor) *PermissionService {
	return &PermissionService{permissionRepo: newRepositories(processor).permission}
}

// CreatePermission 创建权限
func (s *PermissionService) CreatePermission(ctx context.Context, req *dto.CreatePermissionRequest) (*dto.PermissionResponse, error) {
	var permission model.Permission
	if err := copier.Copy(&permission, req); err != nil {
		return nil, errors.Wrap(err, "failed to copy request to permission")
	}

	if err := s.permissionRepo.Create(ctx, &permission); err != nil {
		logger.Error(ctx, "Failed to create permission", zap.Error(err))
		return nil, errors.Wrap(err, "failed to create permission")
	}

	var result dto.PermissionResponse
	if err := copier.Copy(&result, &permission); err != nil {
		return nil, errors.Wrap(err, "failed to copy permission to result")
	}
	return &result, nil
}

// CreatePermissions 批量创建权限
func (s *PermissionService) CreatePermissions(ctx context.Context, req *dto.CreatePermissionsRequest) error {
	permissions := make([]model.Permission, 0, len(req.Permissions))
	if err := copier.Copy(&permissions, &req.Permissions); err != nil {
		return errors.Wrap(err, "failed to copy requests to permissions")
	}

	if err := s.permissionRepo.BatchCreate(ctx, permissions); err != nil {
		logger.Error(ctx, "Failed to create permissions", zap.Error(err))
		return errors.Wrap(err, "failed to create permissions")
	}
	return nil
}

// UpdatePermission 更新权限
func (s *PermissionService) UpdatePermission(ctx context.Context, req *dto.UpdatePermissionRequest) (*dto.PermissionResponse, error) {
	permission, err := s.permissionRepo.FindByID(ctx, req.ID)
	if err != nil {
		logger.Error(ctx, "Failed to retrieve permission", zap.Error(err), zap.Uint64("permission_id", req.ID))
		return nil, errors.Wrap(err, "failed to retrieve permission")
	}

	if err := copier.Copy(permission, req); err != nil {
		return nil, errors.Wrap(err, "failed to copy request to permission")
	}

	if err := s.permissionRepo.Update(ctx, permission); err != nil {
		logger.Error(ctx, "Failed to update permission", zap.Error(err), zap.Uint64("permission_id", req.ID))
		return nil, errors.Wrap(err, "failed to update permission")
	}

	var result dto.PermissionResponse
	if err := copier.Copy(&result, permission); err != nil {
		return nil, errors.Wrap(err, "failed to copy permission to result")
	}
	return &result, nil
}

// DeletePermissionBatch 批量删除权限
func (s *PermissionService) DeletePermissionBatch(ctx context.Context, ids []uint64) error {
	if len(ids) == 0 {
		return nil
	}
	err := s.permissionRepo.QueryBuilder().In("id", ids).Delete(ctx)
	if err != nil {
		logger.Error(ctx, "Failed to delete permissions", zap.Error(err), zap.Uint64s("permission_ids", ids))
		return errors.Wrap(err, "failed to delete permissions")
	}
	return nil
}

// GetPermission 获取单个权限
func (s *PermissionService) GetPermission(ctx context.Context, id uint64) (*dto.PermissionResponse, error) {
	permission, err := s.permissionRepo.FindByID(ctx, id)
	if err != nil {
		logger.Error(ctx, "Failed to retrieve permission", zap.Error(err), zap.Uint64("permission_id", id))
		return nil, errors.Wrap(err, "failed to retrieve permission")
	}

	var result dto.PermissionResponse
	if err := copier.Copy(&result, permission); err != nil {
		return nil, errors.Wrap(err, "failed to copy permission to result")
	}
	return &result, nil
}

// GetPermissionList 获取权限列表
func (s *PermissionService) GetPermissionList(ctx context.Context, req *dto.GetPermissionListRequest) ([]*dto.PermissionResponse, int64, error) {
	permissions, total, err := s.permissionRepo.FindPage(ctx, req, req.Limit, req.Offset)
	if err != nil {
		logger.Error(ctx, "Failed to retrieve permissions", zap.Error(err))
		return nil, 0, errors.Wrap(err, "failed to retrieve permissions")
	}

	result := make([]*dto.PermissionResponse, 0, len(permissions))
	if err := copier.Copy(&result, &permissions); err != nil {
		return nil, 0, errors.Wrap(err, "failed to copy permissions to result")
	}
	return result, total, nil
}
