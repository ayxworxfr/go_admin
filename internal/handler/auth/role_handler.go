package auth_handler

import (
	"github.com/ayxworxfr/go_admin/internal/domain/params"
	"github.com/ayxworxfr/go_admin/internal/domain/vo"
	"github.com/ayxworxfr/go_admin/internal/service"
	"github.com/ayxworxfr/go_admin/pkg/context"
)

type IRoleHandler interface {
	CreateRole(c *context.Context, req *params.CreateRoleRequest) *context.Response
	CreateRoleBatch(c *context.Context, req *params.CreateRolesRequest) *context.Response
	UpdateRole(c *context.Context, req *params.UpdateRoleRequest) *context.Response
	DeleteRole(c *context.Context, req *params.DeleteRoleRequest) *context.Response
	GetRole(c *context.Context, req *params.GetRoleRequest) *context.Response
	GetRoleList(c *context.Context, req *params.GetRoleListRequest) *context.Response
	GetRolePermissions(c *context.Context, req *params.GetRolePermissionsRequest) *context.Response
}

type RoleHandler struct{}

// @route Post /role
// CreateRole 创建角色
func (h *RoleHandler) CreateRole(c *context.Context, req *params.CreateRoleRequest) *context.Response {
	role, err := service.PermissionServiceInstance.CreateRole(c.Context(), req)
	if err != nil {
		return context.DatabaseError(err)
	}

	return context.Success(role)
}

// @route Post /role/batch
// CreateRoleBatch 批量创建角色
func (h *RoleHandler) CreateRoleBatch(c *context.Context, req *params.CreateRolesRequest) *context.Response {
	var result []*vo.Role
	for _, roleReq := range req.Roles {
		role, err := service.PermissionServiceInstance.CreateRole(c.Context(), roleReq)
		if err != nil {
			return context.DatabaseError(err)
		}
		result = append(result, role)
	}

	return context.Success(result)
}

// @route Put /role
// UpdateRole 更新角色
func (h *RoleHandler) UpdateRole(c *context.Context, req *params.UpdateRoleRequest) *context.Response {
	role, err := service.PermissionServiceInstance.UpdateRole(c.Context(), req)
	if err != nil {
		return context.DatabaseError(err)
	}

	return context.Success(role)
}

// @route Delete /role
// DeleteRole 删除角色
func (h *RoleHandler) DeleteRole(c *context.Context, req *params.DeleteRoleRequest) *context.Response {
	if err := service.PermissionServiceInstance.DeleteRoleBatch(c.Context(), req.IDs); err != nil {
		return context.DatabaseError(err)
	}

	return context.NoContent()
}

// @route Get /role
// GetRole 获取单个角色
func (h *RoleHandler) GetRole(c *context.Context, req *params.GetRoleRequest) *context.Response {
	role, err := service.PermissionServiceInstance.GetRole(c.Context(), req.ID)
	if err != nil {
		return context.DatabaseError(err)
	}

	return context.Success(role)
}

// @route Get /role/list
// GetRoleList 获取角色列表
func (h *RoleHandler) GetRoleList(c *context.Context, req *params.GetRoleListRequest) *context.Response {
	roles, total, err := service.PermissionServiceInstance.GetRoleList(c.Context(), req)
	if err != nil {
		return context.DatabaseError(err)
	}

	return context.PageSuccess(roles, total)
}

// @route Get /role/permission/list
// GetRolePermissions 获取角色的权限列表
func (h *RoleHandler) GetRolePermissions(c *context.Context, req *params.GetRolePermissionsRequest) *context.Response {
	permissions, err := service.PermissionServiceInstance.GetRolePermissions(c.Context(), req.RoleID)
	if err != nil {
		return context.DatabaseError(err)
	}

	return context.Success(permissions)
}
