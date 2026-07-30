package handler

import (
	"github.com/ayxworxfr/go_admin/internal/modules/iam/dto"
	"github.com/ayxworxfr/go_admin/internal/modules/iam/service"
	"github.com/ayxworxfr/go_admin/pkg/context"
	"github.com/jinzhu/copier"
)

// UserRoleHandler 用户-角色分配、用户权限查询接口
type UserRoleHandler struct {
	userRoleSvc *service.UserRoleService
	checker     *service.PermissionChecker
}

// NewUserRoleHandler 创建用户角色处理器
func NewUserRoleHandler(userRoleSvc *service.UserRoleService, checker *service.PermissionChecker) *UserRoleHandler {
	return &UserRoleHandler{userRoleSvc: userRoleSvc, checker: checker}
}

// @route Get /user/assign/roles
// UserAssignRoles 为用户分配角色
func (h *UserRoleHandler) UserAssignRoles(c *context.Context, req *dto.AssignRolesRequest) *context.Response {
	if err := h.userRoleSvc.AssignRoles(c.Context(), req.UserID, req.RoleIDs); err != nil {
		return context.DatabaseError(err)
	}
	h.checker.InvalidateUser(req.UserID)

	user, err := h.userRoleSvc.GetUserRoles(c.Context(), req.UserID, dto.ALL_AUTH_FLAGS)
	if err != nil {
		return context.DatabaseError(err)
	}
	return context.Success(user)
}

// @route Get /user/roles
// GetUserRoles 获取用户的角色列表
func (h *UserRoleHandler) GetUserRoles(c *context.Context, req *dto.GetUserRolesRequest) *context.Response {
	user, err := h.userRoleSvc.GetUserRoles(c.Context(), req.UserID, req.Flags)
	if err != nil {
		return context.DatabaseError(err)
	}
	return context.Success(user)
}

// @route Get /user/permissions
// GetUserPermissions 获取用户的权限列表
func (h *UserRoleHandler) GetUserPermissions(c *context.Context, req *dto.GetUserPermissionsRequest) *context.Response {
	permissions, err := h.checker.GetUserPermissions(c.Context(), req.UserID)
	if err != nil {
		return context.DatabaseError(err)
	}

	var result []*dto.PermissionResponse
	if err := copier.Copy(&result, &permissions); err != nil {
		return context.InternalError(err)
	}
	return context.Success(result)
}
