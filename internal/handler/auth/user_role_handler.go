package auth_handler

import (
	"github.com/ayxworxfr/go_admin/internal/domain/params"
	"github.com/ayxworxfr/go_admin/internal/service"
	"github.com/ayxworxfr/go_admin/pkg/context"
)

type IUserRoleHandler interface {
	UserAssignRoles(c *context.Context, req *params.AssignRolesRequest) *context.Response
	GetUserRoles(c *context.Context, req *params.GetUserRolesRequest) *context.Response
	GetUserPermissions(c *context.Context, req *params.GetUserPermissionsRequest) *context.Response
}

type UserRoleHandler struct{}

// @route Get /user/assign/roles
// UserAssignRoles 为用户分配角色
func (h *UserRoleHandler) UserAssignRoles(c *context.Context, req *params.AssignRolesRequest) *context.Response {
	if err := service.PermissionServiceInstance.AssignRoles(c.Context(), req.UserID, req.RoleIDs); err != nil {
		return context.DatabaseError(err)
	}

	// 获取用户的角色
	user, err := service.PermissionServiceInstance.GetUserRoles(c.Context(), req.UserID)
	if err != nil {
		return context.DatabaseError(err)
	}

	return context.Success(user)
}

// @route Get /user/roles
// GetUserRoles 获取用户的角色列表
func (h *UserRoleHandler) GetUserRoles(c *context.Context, req *params.GetUserRolesRequest) *context.Response {
	user, err := service.PermissionServiceInstance.GetUserRolesByFlags(c.Context(), req.UserID, req.Flags)
	if err != nil {
		return context.DatabaseError(err)
	}

	return context.Success(user)
}

// @route Get /user/permissions
// GetUserPermissions 获取用户的权限列表
func (h *UserRoleHandler) GetUserPermissions(c *context.Context, req *params.GetUserPermissionsRequest) *context.Response {
	permissions, err := service.PermissionServiceInstance.GetUserPermissions(c.Context(), req.UserID)
	if err != nil {
		return context.DatabaseError(err)
	}

	return context.Success(permissions)
}
