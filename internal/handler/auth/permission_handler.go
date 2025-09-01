package auth_handler

import (
	"github.com/ayxworxfr/go_admin/internal/dao"
	"github.com/ayxworxfr/go_admin/internal/domain/params"
	"github.com/ayxworxfr/go_admin/internal/service"
	"github.com/ayxworxfr/go_admin/pkg/context"
)

type IPermissionHandler interface {
	CreatePermission(c *context.Context) *context.Response
	CreatePermissionBatch(c *context.Context) *context.Response
	UpdatePermission(c *context.Context) *context.Response
	DeletePermission(c *context.Context, req *params.DeletePermissionRequest) *context.Response
	GetPermission(c *context.Context, req *params.GetPermissionRequest) *context.Response
	GetPermissionList(c *context.Context) *context.Response
}

type PermissionHandler struct{}

// @route Post /permission
// CreatePermission 创建权限
func (h *PermissionHandler) CreatePermission(c *context.Context) *context.Response {
	var req params.CreatePermissionRequest
	if err := c.BindAndValidate(&req); err != nil {
		return context.ParamError(err)
	}

	permission, err := service.PermissionServiceInstance.CreatePermission(c.Context(), &req)
	if err != nil {
		return context.DatabaseError(err)
	}

	return context.Success(permission)
}

// @route Post /permission/batch
// CreatePermissionBatch 批量创建权限
func (h *PermissionHandler) CreatePermissionBatch(c *context.Context) *context.Response {
	var req params.CreatePermissionsRequest
	if err := c.BindAndValidate(&req); err != nil {
		return context.ParamError(err)
	}

	if err := service.PermissionServiceInstance.CreatePermissions(c.Context(), &req); err != nil {
		return context.DatabaseError(err)
	}

	return context.Success(nil)
}

// @route Put /permission
// UpdatePermission 更新权限
func (h *PermissionHandler) UpdatePermission(c *context.Context) *context.Response {
	var req params.UpdatePermissionRequest
	if err := c.BindAndValidate(&req); err != nil {
		return context.ParamError(err)
	}

	permission, err := service.PermissionServiceInstance.UpdatePermission(c.Context(), &req)
	if err != nil {
		return context.DatabaseError(err)
	}

	return context.Success(permission)
}

// @route Delete /permission
// DeletePermission 删除权限
func (h *PermissionHandler) DeletePermission(c *context.Context, req *params.DeletePermissionRequest) *context.Response {
	if err := service.PermissionServiceInstance.DeletePermissionBatch(c.Context(), req.IDs); err != nil {
		return context.DatabaseError(err)
	}

	return context.NoContent()
}

// @route Get /permission
// GetPermission 获取单个权限
func (h *PermissionHandler) GetPermission(c *context.Context, req *params.GetPermissionRequest) *context.Response {
	permission, err := dao.PermissionRepo.FindByID(c.Context(), req.ID)
	if err != nil {
		return context.DatabaseError(err)
	}

	return context.Success(permission)
}

// @route Get /permission/list
// GetPermissionList 获取权限列表
func (h *PermissionHandler) GetPermissionList(c *context.Context) *context.Response {
	var req params.GetPermissionListRequest
	if err := c.BindQuery(&req); err != nil {
		return context.ParamError(err)
	}

	permissions, total, err := service.PermissionServiceInstance.GetPermissionList(c.Context(), &req)
	if err != nil {
		return context.DatabaseError(err)
	}

	return context.PageSuccess(permissions, total)
}
