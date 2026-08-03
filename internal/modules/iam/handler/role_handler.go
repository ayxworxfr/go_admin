package handler

import (
	"github.com/ayxworxfr/go_admin/internal/modules/iam/dto"
	"github.com/ayxworxfr/go_admin/internal/modules/iam/service"
	"github.com/ayxworxfr/go_admin/pkg/api"
)

// RoleHandler 角色管理接口
type RoleHandler struct {
	roleSvc *service.RoleService
	checker *service.PermissionChecker
}

// NewRoleHandler 创建角色处理器
func NewRoleHandler(roleSvc *service.RoleService, checker *service.PermissionChecker) *RoleHandler {
	return &RoleHandler{roleSvc: roleSvc, checker: checker}
}

// @route Post /role
// CreateRole 创建角色
func (h *RoleHandler) CreateRole(c *api.Context, req *dto.CreateRoleRequest) *api.Response {
	role, err := h.roleSvc.CreateRole(c.Context(), req)
	if err != nil {
		return api.DatabaseError(err)
	}
	h.checker.InvalidateAll()
	return api.Success(role)
}

// @route Post /role/batch
// CreateRoleBatch 批量创建角色
func (h *RoleHandler) CreateRoleBatch(c *api.Context, req *dto.CreateRolesRequest) *api.Response {
	result := make([]*dto.RoleResponse, 0, len(req.Roles))
	for _, roleReq := range req.Roles {
		role, err := h.roleSvc.CreateRole(c.Context(), roleReq)
		if err != nil {
			return api.DatabaseError(err)
		}
		result = append(result, role)
	}
	h.checker.InvalidateAll()
	return api.Success(result)
}

// @route Put /role
// UpdateRole 更新角色
func (h *RoleHandler) UpdateRole(c *api.Context, req *dto.UpdateRoleRequest) *api.Response {
	role, err := h.roleSvc.UpdateRole(c.Context(), req)
	if err != nil {
		return api.DatabaseError(err)
	}
	h.checker.InvalidateAll()
	return api.Success(role)
}

// @route Delete /role
// DeleteRole 删除角色
func (h *RoleHandler) DeleteRole(c *api.Context, req *dto.DeleteRoleRequest) *api.Response {
	if err := h.roleSvc.DeleteRoleBatch(c.Context(), req.IDs); err != nil {
		return api.DatabaseError(err)
	}
	h.checker.InvalidateAll()
	return api.NoContent()
}

// @route Get /role
// GetRole 获取单个角色
func (h *RoleHandler) GetRole(c *api.Context, req *dto.GetRoleRequest) *api.Response {
	role, err := h.roleSvc.GetRole(c.Context(), req.ID)
	if err != nil {
		return api.DatabaseError(err)
	}
	return api.Success(role)
}

// @route Get /role/list
// GetRoleList 获取角色列表
func (h *RoleHandler) GetRoleList(c *api.Context, req *dto.GetRoleListRequest) *api.Response {
	roles, total, err := h.roleSvc.GetRoleList(c.Context(), req)
	if err != nil {
		return api.DatabaseError(err)
	}
	return api.PageSuccess(roles, total)
}

// @route Get /role/permission/list
// GetRolePermissions 获取角色的权限列表
func (h *RoleHandler) GetRolePermissions(c *api.Context, req *dto.GetRolePermissionsRequest) *api.Response {
	permissions, err := h.roleSvc.GetRolePermissions(c.Context(), req.RoleID)
	if err != nil {
		return api.DatabaseError(err)
	}
	return api.Success(permissions)
}
