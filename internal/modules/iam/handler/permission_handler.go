package handler

import (
	"github.com/ayxworxfr/go_admin/internal/modules/iam/dto"
	"github.com/ayxworxfr/go_admin/internal/modules/iam/service"
	"github.com/ayxworxfr/go_admin/pkg/context"
)

// PermissionHandler 权限元数据管理接口
type PermissionHandler struct {
	permSvc *service.PermissionService
	checker *service.PermissionChecker
}

// NewPermissionHandler 创建权限处理器
func NewPermissionHandler(permSvc *service.PermissionService, checker *service.PermissionChecker) *PermissionHandler {
	return &PermissionHandler{permSvc: permSvc, checker: checker}
}

// @route Post /permission
// CreatePermission 创建权限
func (h *PermissionHandler) CreatePermission(c *context.Context, req *dto.CreatePermissionRequest) *context.Response {
	permission, err := h.permSvc.CreatePermission(c.Context(), req)
	if err != nil {
		return context.DatabaseError(err)
	}
	h.checker.InvalidateAll()
	return context.Success(permission)
}

// @route Post /permission/batch
// CreatePermissionBatch 批量创建权限
func (h *PermissionHandler) CreatePermissionBatch(c *context.Context, req *dto.CreatePermissionsRequest) *context.Response {
	if err := h.permSvc.CreatePermissions(c.Context(), req); err != nil {
		return context.DatabaseError(err)
	}
	h.checker.InvalidateAll()
	return context.Success(nil)
}

// @route Put /permission
// UpdatePermission 更新权限
func (h *PermissionHandler) UpdatePermission(c *context.Context, req *dto.UpdatePermissionRequest) *context.Response {
	permission, err := h.permSvc.UpdatePermission(c.Context(), req)
	if err != nil {
		return context.DatabaseError(err)
	}
	h.checker.InvalidateAll()
	return context.Success(permission)
}

// @route Delete /permission
// DeletePermission 删除权限
func (h *PermissionHandler) DeletePermission(c *context.Context, req *dto.DeletePermissionRequest) *context.Response {
	if err := h.permSvc.DeletePermissionBatch(c.Context(), req.IDs); err != nil {
		return context.DatabaseError(err)
	}
	h.checker.InvalidateAll()
	return context.NoContent()
}

// @route Get /permission
// GetPermission 获取单个权限
func (h *PermissionHandler) GetPermission(c *context.Context, req *dto.GetPermissionRequest) *context.Response {
	permission, err := h.permSvc.GetPermission(c.Context(), req.ID)
	if err != nil {
		return context.DatabaseError(err)
	}
	return context.Success(permission)
}

// @route Get /permission/list
// GetPermissionList 获取权限列表
func (h *PermissionHandler) GetPermissionList(c *context.Context, req *dto.GetPermissionListRequest) *context.Response {
	permissions, total, err := h.permSvc.GetPermissionList(c.Context(), req)
	if err != nil {
		return context.DatabaseError(err)
	}
	return context.PageSuccess(permissions, total)
}
