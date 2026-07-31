package handler

import (
	"github.com/ayxworxfr/go_admin/internal/modules/systemsetting/dto"
	"github.com/ayxworxfr/go_admin/internal/modules/systemsetting/service"
	"github.com/ayxworxfr/go_admin/pkg/reqctx"
)

// Handler 系统配置管理接口
type Handler struct {
	svc *service.Service
}

// NewHandler 创建系统配置处理器
func NewHandler(svc *service.Service) *Handler {
	return &Handler{svc: svc}
}

// @route Post /system-setting
func (h *Handler) CreateSystemSetting(c *reqctx.Context, req *dto.CreateSystemSettingRequest) *reqctx.Response {
	userID, err := c.UserID()
	if err != nil {
		return reqctx.Unauthorized(err)
	}
	result, err := h.svc.Create(c.Context(), req, userID)
	if err != nil {
		return reqctx.BusinessError(err)
	}
	return reqctx.Success(result)
}

// @route Get /system-setting
func (h *Handler) GetSystemSetting(c *reqctx.Context, req *dto.GetSystemSettingRequest) *reqctx.Response {
	result, err := h.svc.Get(c.Context(), req.ID)
	if err != nil {
		return reqctx.DatabaseError(err)
	}
	return reqctx.Success(result)
}

// @route Get /system-setting/list
func (h *Handler) GetSystemSettingList(c *reqctx.Context, req *dto.GetSystemSettingListRequest) *reqctx.Response {
	result, total, err := h.svc.List(c.Context(), req)
	if err != nil {
		return reqctx.DatabaseError(err)
	}
	return reqctx.PageSuccess(result, total)
}

// @route Put /system-setting
func (h *Handler) UpdateSystemSetting(c *reqctx.Context, req *dto.UpdateSystemSettingRequest) *reqctx.Response {
	result, err := h.svc.Update(c.Context(), req)
	if err != nil {
		return reqctx.BusinessError(err)
	}
	return reqctx.Success(result)
}

// @route Delete /system-setting
func (h *Handler) DeleteSystemSetting(c *reqctx.Context, req *dto.DeleteSystemSettingRequest) *reqctx.Response {
	if err := h.svc.DeleteBatch(c.Context(), req.IDs); err != nil {
		return reqctx.BusinessError(err)
	}
	return reqctx.Success(nil)
}

// @route Get /system-setting/by-category
func (h *Handler) GetSystemSettingByCategory(c *reqctx.Context, req *dto.GetSystemSettingByCategoryRequest) *reqctx.Response {
	result, err := h.svc.GetByCategory(c.Context(), req.Category)
	if err != nil {
		return reqctx.BusinessError(err)
	}
	return reqctx.Success(result)
}
