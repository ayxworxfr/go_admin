package handler

import (
	"github.com/ayxworxfr/go_admin/internal/modules/systemsetting/dto"
	"github.com/ayxworxfr/go_admin/internal/modules/systemsetting/service"
	"github.com/ayxworxfr/go_admin/pkg/api"
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
func (h *Handler) CreateSystemSetting(c *api.Context, req *dto.CreateSystemSettingRequest) *api.Response {
	userID, err := c.UserID()
	if err != nil {
		return api.Unauthorized(err)
	}
	result, err := h.svc.Create(c.Context(), req, userID)
	if err != nil {
		return api.BusinessError(err)
	}
	return api.Success(result)
}

// @route Get /system-setting
func (h *Handler) GetSystemSetting(c *api.Context, req *dto.GetSystemSettingRequest) *api.Response {
	result, err := h.svc.Get(c.Context(), req.ID)
	if err != nil {
		return api.DatabaseError(err)
	}
	return api.Success(result)
}

// @route Get /system-setting/list
func (h *Handler) GetSystemSettingList(c *api.Context, req *dto.GetSystemSettingListRequest) *api.Response {
	result, total, err := h.svc.List(c.Context(), req)
	if err != nil {
		return api.DatabaseError(err)
	}
	return api.PageSuccess(result, total)
}

// @route Put /system-setting
func (h *Handler) UpdateSystemSetting(c *api.Context, req *dto.UpdateSystemSettingRequest) *api.Response {
	result, err := h.svc.Update(c.Context(), req)
	if err != nil {
		return api.BusinessError(err)
	}
	return api.Success(result)
}

// @route Delete /system-setting
func (h *Handler) DeleteSystemSetting(c *api.Context, req *dto.DeleteSystemSettingRequest) *api.Response {
	if err := h.svc.DeleteBatch(c.Context(), req.IDs); err != nil {
		return api.BusinessError(err)
	}
	return api.Success(nil)
}

// @route Get /system-setting/by-category
func (h *Handler) GetSystemSettingByCategory(c *api.Context, req *dto.GetSystemSettingByCategoryRequest) *api.Response {
	result, err := h.svc.GetByCategory(c.Context(), req.Category)
	if err != nil {
		return api.BusinessError(err)
	}
	return api.Success(result)
}
