package handler

import (
	"github.com/ayxworxfr/go_admin/internal/modules/iam/dto"
	"github.com/ayxworxfr/go_admin/internal/modules/iam/service"
	"github.com/ayxworxfr/go_admin/pkg/reqctx"
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
func (h *PermissionHandler) CreatePermission(c *reqctx.Context, req *dto.CreatePermissionRequest) *reqctx.Response {
	permission, err := h.permSvc.CreatePermission(c.Context(), req)
	if err != nil {
		return reqctx.DatabaseError(err)
	}
	h.checker.InvalidateAll()
	return reqctx.Success(permission)
}

// @route Post /permission/batch
// CreatePermissionBatch 批量创建权限
func (h *PermissionHandler) CreatePermissionBatch(c *reqctx.Context, req *dto.CreatePermissionsRequest) *reqctx.Response {
	if err := h.permSvc.CreatePermissions(c.Context(), req); err != nil {
		return reqctx.DatabaseError(err)
	}
	h.checker.InvalidateAll()
	return reqctx.Success(nil)
}

// @route Put /permission
// UpdatePermission 更新权限
func (h *PermissionHandler) UpdatePermission(c *reqctx.Context, req *dto.UpdatePermissionRequest) *reqctx.Response {
	permission, err := h.permSvc.UpdatePermission(c.Context(), req)
	if err != nil {
		return reqctx.DatabaseError(err)
	}
	h.checker.InvalidateAll()
	return reqctx.Success(permission)
}

// @route Delete /permission
// DeletePermission 删除权限
func (h *PermissionHandler) DeletePermission(c *reqctx.Context, req *dto.DeletePermissionRequest) *reqctx.Response {
	if err := h.permSvc.DeletePermissionBatch(c.Context(), req.IDs); err != nil {
		return reqctx.DatabaseError(err)
	}
	h.checker.InvalidateAll()
	return reqctx.NoContent()
}

// @route Get /permission
// GetPermission 获取单个权限
func (h *PermissionHandler) GetPermission(c *reqctx.Context, req *dto.GetPermissionRequest) *reqctx.Response {
	permission, err := h.permSvc.GetPermission(c.Context(), req.ID)
	if err != nil {
		return reqctx.DatabaseError(err)
	}
	return reqctx.Success(permission)
}

// @route Get /permission/list
// GetPermissionList 获取权限列表
func (h *PermissionHandler) GetPermissionList(c *reqctx.Context, req *dto.GetPermissionListRequest) *reqctx.Response {
	permissions, total, err := h.permSvc.GetPermissionList(c.Context(), req)
	if err != nil {
		return reqctx.DatabaseError(err)
	}
	return reqctx.PageSuccess(permissions, total)
}
