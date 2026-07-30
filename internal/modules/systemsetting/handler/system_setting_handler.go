package handler

import (
	"github.com/ayxworxfr/go_admin/internal/modules/systemsetting/dto"
	"github.com/ayxworxfr/go_admin/internal/modules/systemsetting/service"
	"github.com/ayxworxfr/go_admin/pkg/context"
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
func (h *Handler) CreateSystemSetting(c *context.Context, req *dto.CreateSystemSettingRequest) *context.Response {
	result, err := h.svc.Create(c.Context(), req, c.GetUserID())
	if err != nil {
		return context.BusinessError(err)
	}
	return context.Success(result)
}

// @route Get /system-setting
func (h *Handler) GetSystemSetting(c *context.Context, req *dto.GetSystemSettingRequest) *context.Response {
	result, err := h.svc.Get(c.Context(), req.ID)
	if err != nil {
		return context.DatabaseError(err)
	}
	return context.Success(result)
}

// @route Get /system-setting/list
func (h *Handler) GetSystemSettingList(c *context.Context, req *dto.GetSystemSettingListRequest) *context.Response {
	result, total, err := h.svc.List(c.Context(), req)
	if err != nil {
		return context.DatabaseError(err)
	}
	return context.PageSuccess(result, total)
}

// @route Put /system-setting
func (h *Handler) UpdateSystemSetting(c *context.Context, req *dto.UpdateSystemSettingRequest) *context.Response {
	result, err := h.svc.Update(c.Context(), req)
	if err != nil {
		return context.BusinessError(err)
	}
	return context.Success(result)
}

// @route Delete /system-setting
func (h *Handler) DeleteSystemSetting(c *context.Context, req *dto.DeleteSystemSettingRequest) *context.Response {
	if err := h.svc.DeleteBatch(c.Context(), req.IDs); err != nil {
		return context.BusinessError(err)
	}
	return context.Success(nil)
}

// @route Get /system-setting/by-category
func (h *Handler) GetSystemSettingByCategory(c *context.Context, req *dto.GetSystemSettingByCategoryRequest) *context.Response {
	result, err := h.svc.GetByCategory(c.Context(), req.Category)
	if err != nil {
		return context.BusinessError(err)
	}
	return context.Success(result)
}
