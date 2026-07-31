package service

import (
	"context"

	"github.com/ayxworxfr/go_admin/internal/modules/iam/dto"
	"github.com/ayxworxfr/go_admin/internal/modules/iam/model"
	usersvc "github.com/ayxworxfr/go_admin/internal/modules/user/service"
	"github.com/ayxworxfr/go_admin/pkg/logger"
	pkgrepo "github.com/ayxworxfr/go_admin/pkg/repository"
	"github.com/jinzhu/copier"
	"github.com/pkg/errors"
	"github.com/samber/lo"
	"go.uber.org/zap"
)

// UserRoleService 负责"用户拥有哪些角色"这一件事：分配、查询、以及为登录/
// 鉴权场景提供角色数据。之所以从旧版 PermissionService 里单独拆出来，是因为
// 它的变更方向（哪个用户属于哪个角色）与 RoleService（角色本身长什么样）
// 是两个可以独立演化的关注点。
//
// 依赖方向：iam -> user.UserFinder（查用户基础信息），不反向依赖。
type UserRoleService struct {
	userRoleRepo *pkgrepo.Repository[model.UserRole]
	roleSvc      *RoleService
	userFinder   usersvc.UserFinder
}

// NewUserRoleService 创建用户角色分配服务
func NewUserRoleService(db *pkgrepo.DB, roleSvc *RoleService, userFinder usersvc.UserFinder) *UserRoleService {
	return &UserRoleService{
		userRoleRepo: newRepositories(db).userRole,
		roleSvc:      roleSvc,
		userFinder:   userFinder,
	}
}

// AssignRoles 为用户分配角色（先确认用户存在，再做增量分配）
func (s *UserRoleService) AssignRoles(ctx context.Context, userID uint64, roleIDs []uint64) error {
	if _, err := s.userFinder.FindByID(ctx, userID); err != nil {
		logger.Error(ctx, "Failed to retrieve user", zap.Error(err), zap.Uint64("user_id", userID))
		return errors.Wrap(err, "failed to retrieve user")
	}
	return s.assignUserRoles(ctx, userID, roleIDs)
}

// assignUserRoles 对比新旧角色集合，只做增量的删除与新增
func (s *UserRoleService) assignUserRoles(ctx context.Context, userID uint64, roleIDs []uint64) error {
	current, err := s.userRoleRepo.FindAll(ctx, &model.UserRole{UserID: userID})
	if err != nil {
		return errors.Wrap(err, "failed to retrieve user roles")
	}

	existingIDs := lo.Map(current, func(ur model.UserRole, _ int) uint64 { return ur.RoleID })
	toRemoveIDs := lo.Filter(existingIDs, func(id uint64, _ int) bool { return !lo.Contains(roleIDs, id) })
	toAddIDs := lo.Filter(roleIDs, func(id uint64, _ int) bool { return !lo.Contains(existingIDs, id) })

	return s.userRoleRepo.Transaction(ctx, func(txCtx context.Context) error {
		if len(toRemoveIDs) > 0 {
			if err := s.userRoleRepo.QueryBuilder().Eq("user_id", userID).In("role_id", toRemoveIDs).Delete(txCtx); err != nil {
				return errors.Wrap(err, "failed to delete user roles")
			}
		}
		if len(toAddIDs) > 0 {
			userRoles := lo.Map(toAddIDs, func(roleID uint64, _ int) model.UserRole {
				return model.UserRole{UserID: userID, RoleID: roleID}
			})
			if err := s.userRoleRepo.BatchCreate(txCtx, userRoles); err != nil {
				return errors.Wrap(err, "failed to create user roles")
			}
		}
		return nil
	})
}

// RetrieveRolesByUserID 通过用户 ID 查询关联角色（不含权限展开），
// 是本服务对外暴露的读接口，PermissionChecker 组合本方法计算用户权限集合。
func (s *UserRoleService) RetrieveRolesByUserID(ctx context.Context, userID uint64) ([]model.Role, error) {
	userRoles, err := s.userRoleRepo.QueryBuilder().Eq("user_id", userID).Find(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "query UserRole failed")
	}
	if len(userRoles) == 0 {
		return []model.Role{}, nil
	}

	roleIDs := lo.Map(userRoles, func(ur model.UserRole, _ int) uint64 { return ur.RoleID })
	roles, err := s.roleSvc.roleRepo.QueryBuilder().In("id", roleIDs).Find(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "query Role failed")
	}
	return roles, nil
}

// RetrieveRoleResponsesByUserID 查询用户角色并按 flags 决定是否展开权限
func (s *UserRoleService) RetrieveRoleResponsesByUserID(ctx context.Context, userID uint64, flags int) ([]*dto.RoleResponse, error) {
	roles, err := s.RetrieveRolesByUserID(ctx, userID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to retrieve user roles")
	}

	result := make([]*dto.RoleResponse, 0, len(roles))
	if err := copier.Copy(&result, &roles); err != nil {
		return nil, errors.Wrap(err, "failed to copy roles to response")
	}

	if !dto.NewResponseFlags(flags).Has(dto.INCLUDE_PERMISSION) {
		return result, nil
	}

	for i, role := range roles {
		permissions, err := s.roleSvc.RetrievePermissionByRoleID(ctx, role.ID)
		if err != nil {
			logger.Error(ctx, "Failed to retrieve role permissions", zap.Error(err), zap.Uint64("role_id", role.ID))
			return nil, errors.Wrap(err, "failed to retrieve role permissions")
		}
		if err := copier.Copy(&result[i].Permissions, &permissions); err != nil {
			return nil, errors.Wrap(err, "failed to copy permissions to response")
		}
	}
	return result, nil
}

// GetUserRoles 获取用户及其角色的组合视图（分配角色接口返回结果用），
// 组合 user.UserFinder 的基础字段与本服务的角色查询，不共享类型定义。
func (s *UserRoleService) GetUserRoles(ctx context.Context, userID uint64, flags int) (*dto.UserRolesResponse, error) {
	user, err := s.userFinder.FindByID(ctx, userID)
	if err != nil {
		logger.Error(ctx, "Failed to retrieve user", zap.Error(err), zap.Uint64("user_id", userID))
		return nil, errors.Wrap(err, "failed to retrieve user")
	}

	roles, err := s.RetrieveRoleResponsesByUserID(ctx, userID, flags)
	if err != nil {
		logger.Error(ctx, "Failed to retrieve user roles", zap.Error(err), zap.Uint64("user_id", userID))
		return nil, errors.Wrap(err, "failed to retrieve user roles")
	}

	return &dto.UserRolesResponse{
		ID:       user.ID,
		Username: user.Username,
		Email:    user.Email,
		Roles:    roles,
	}, nil
}

// fetchHighestPriorityRole 获取优先级最高的角色（假设 ID 越小优先级越高），
// 供 AuthService 登录时挑一个角色写进 JWT 的 rolekey。
func fetchHighestPriorityRole(roles []*dto.RoleResponse) *dto.RoleResponse {
	if len(roles) == 0 {
		return nil
	}
	highest := roles[0]
	for _, role := range roles {
		if role.ID < highest.ID {
			highest = role
		}
	}
	return highest
}
