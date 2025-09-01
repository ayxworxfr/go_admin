package handler

import (
	"github.com/ayxworxfr/go_admin/internal/dao"
	"github.com/ayxworxfr/go_admin/internal/domain/models"
	"github.com/ayxworxfr/go_admin/internal/domain/params"
	"github.com/ayxworxfr/go_admin/internal/service"
	"github.com/ayxworxfr/go_admin/pkg/context"
	"github.com/jinzhu/copier"
)

// ISystemSettingHandler 系统配置处理器接口
type ISystemSettingHandler interface {
	CreateSystemSetting(c *context.Context, req *params.CreateSystemSettingRequest) *context.Response
	GetSystemSetting(c *context.Context, req *params.GetSystemSettingRequest) *context.Response
	GetSystemSettingList(c *context.Context, req *params.GetSystemSettingListRequest) *context.Response
	UpdateSystemSetting(c *context.Context, req *params.UpdateSystemSettingRequest) *context.Response
	DeleteSystemSetting(c *context.Context, req *params.DeleteSystemSettingRequest) *context.Response
	GetSystemSettingByCategory(c *context.Context, req *params.GetSystemSettingByCategoryRequest) *context.Response
}

// SystemSettingHandler 系统配置处理器实现
type SystemSettingHandler struct{}

// @route Post /system-setting
func (h *SystemSettingHandler) CreateSystemSetting(c *context.Context, req *params.CreateSystemSettingRequest) *context.Response {
	// 参数转换为模型
	var setting models.SystemSetting
	if err := copier.Copy(&setting, req); err != nil {
		return context.ParamError(err)
	}

	// 调用服务层创建系统配置
	result, err := service.SystemSettingServiceInstance.CreateSystemSetting(c.Context(), &setting)
	if err != nil {
		return context.BusinessError(err)
	}

	// 转换为VO返回
	voResult, err := service.SystemSettingServiceInstance.PackSystemSettingVO(c.Context(), result)
	if err != nil {
		return context.InternalError(err)
	}
	return context.Success(voResult)
}

// @route Get /system-setting
func (h *SystemSettingHandler) GetSystemSetting(c *context.Context, req *params.GetSystemSettingRequest) *context.Response {
	// 查询系统配置
	setting, err := dao.SystemSettingRepo.FindByID(c.Context(), req.ID)
	if err != nil {
		return context.DatabaseError(err)
	}

	// 补充关联信息
	voSetting, err := service.SystemSettingServiceInstance.PackSystemSettingVO(c.Context(), setting)
	if err != nil {
		return context.InternalError(err)
	}

	return context.Success(voSetting)
}

// @route Get /system-setting/list
func (h *SystemSettingHandler) GetSystemSettingList(c *context.Context, req *params.GetSystemSettingListRequest) *context.Response {
	// 分页查询系统配置列表
	settings, total, err := dao.SystemSettingRepo.FindPage(c.Context(), req, req.Limit, req.Offset)
	if err != nil {
		return context.DatabaseError(err)
	}

	// 转换为VO列表
	voList, err := service.SystemSettingServiceInstance.PackSystemSettingVOList(c.Context(), settings)
	if err != nil {
		return context.InternalError(err)
	}
	return context.PageSuccess(voList, total)
}

// @route Put /system-setting
func (h *SystemSettingHandler) UpdateSystemSetting(c *context.Context, req *params.UpdateSystemSettingRequest) *context.Response {
	// 查询原系统配置信息
	oldSetting, err := dao.SystemSettingRepo.FindByID(c.Context(), req.ID)
	if err != nil {
		return context.DatabaseError(err)
	}

	// 复制更新字段
	if err := copier.Copy(&oldSetting, req); err != nil {
		return context.ParamError(err)
	}

	// 调用服务层更新
	updated, err := service.SystemSettingServiceInstance.UpdateSystemSetting(c.Context(), oldSetting)
	if err != nil {
		return context.BusinessError(err)
	}

	// 转换为VO返回
	voResult, err := service.SystemSettingServiceInstance.PackSystemSettingVO(c.Context(), updated)
	if err != nil {
		return context.InternalError(err)
	}
	return context.Success(voResult)
}

// @route Delete /system-setting
func (h *SystemSettingHandler) DeleteSystemSetting(c *context.Context, req *params.DeleteSystemSettingRequest) *context.Response {
	// 调用服务层批量删除
	err := service.SystemSettingServiceInstance.DeleteSystemSettingBatch(c.Context(), req.IDs)
	if err != nil {
		return context.BusinessError(err)
	}
	return context.Success(nil)
}

// @route Get /system-setting/by-category
func (h *SystemSettingHandler) GetSystemSettingByCategory(c *context.Context, req *params.GetSystemSettingByCategoryRequest) *context.Response {
	// 根据分类获取系统配置列表
	voList, err := service.SystemSettingServiceInstance.GetSystemSettingByCategory(c.Context(), req.Category)
	if err != nil {
		return context.BusinessError(err)
	}
	return context.Success(voList)
}
