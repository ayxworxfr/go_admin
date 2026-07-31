package handler

import (
	"github.com/ayxworxfr/go_admin/internal/modules/iam/dto"
	"github.com/ayxworxfr/go_admin/internal/modules/iam/service"
	"github.com/ayxworxfr/go_admin/pkg/reqctx"
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
func (h *RoleHandler) CreateRole(c *reqctx.Context, req *dto.CreateRoleRequest) *reqctx.Response {
	role, err := h.roleSvc.CreateRole(c.Context(), req)
	if err != nil {
		return reqctx.DatabaseError(err)
	}
	h.checker.InvalidateAll()
	return reqctx.Success(role)
}

// @route Post /role/batch
// CreateRoleBatch 批量创建角色
func (h *RoleHandler) CreateRoleBatch(c *reqctx.Context, req *dto.CreateRolesRequest) *reqctx.Response {
	result := make([]*dto.RoleResponse, 0, len(req.Roles))
	for _, roleReq := range req.Roles {
		role, err := h.roleSvc.CreateRole(c.Context(), roleReq)
		if err != nil {
			return reqctx.DatabaseError(err)
		}
		result = append(result, role)
	}
	h.checker.InvalidateAll()
	return reqctx.Success(result)
}

// @route Put /role
// UpdateRole 更新角色
func (h *RoleHandler) UpdateRole(c *reqctx.Context, req *dto.UpdateRoleRequest) *reqctx.Response {
	role, err := h.roleSvc.UpdateRole(c.Context(), req)
	if err != nil {
		return reqctx.DatabaseError(err)
	}
	h.checker.InvalidateAll()
	return reqctx.Success(role)
}

// @route Delete /role
// DeleteRole 删除角色
func (h *RoleHandler) DeleteRole(c *reqctx.Context, req *dto.DeleteRoleRequest) *reqctx.Response {
	if err := h.roleSvc.DeleteRoleBatch(c.Context(), req.IDs); err != nil {
		return reqctx.DatabaseError(err)
	}
	h.checker.InvalidateAll()
	return reqctx.NoContent()
}

// @route Get /role
// GetRole 获取单个角色
func (h *RoleHandler) GetRole(c *reqctx.Context, req *dto.GetRoleRequest) *reqctx.Response {
	role, err := h.roleSvc.GetRole(c.Context(), req.ID)
	if err != nil {
		return reqctx.DatabaseError(err)
	}
	return reqctx.Success(role)
}

// @route Get /role/list
// GetRoleList 获取角色列表
func (h *RoleHandler) GetRoleList(c *reqctx.Context, req *dto.GetRoleListRequest) *reqctx.Response {
	roles, total, err := h.roleSvc.GetRoleList(c.Context(), req)
	if err != nil {
		return reqctx.DatabaseError(err)
	}
	return reqctx.PageSuccess(roles, total)
}

// @route Get /role/permission/list
// GetRolePermissions 获取角色的权限列表
func (h *RoleHandler) GetRolePermissions(c *reqctx.Context, req *dto.GetRolePermissionsRequest) *reqctx.Response {
	permissions, err := h.roleSvc.GetRolePermissions(c.Context(), req.RoleID)
	if err != nil {
		return reqctx.DatabaseError(err)
	}
	return reqctx.Success(permissions)
}
