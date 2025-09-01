package service

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/ayxworxfr/go_admin/internal/dao"
	"github.com/ayxworxfr/go_admin/internal/domain/models"
	"github.com/ayxworxfr/go_admin/internal/domain/vo"
	"github.com/ayxworxfr/go_admin/pkg/logger"
	"github.com/hashicorp/go-multierror"
	"github.com/jinzhu/copier"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

// SystemSettingService 系统配置服务
type SystemSettingService struct{}

// NewSystemSettingService 创建系统配置服务实例
func NewSystemSettingService() *SystemSettingService {
	return &SystemSettingService{}
}

// CreateSystemSetting 创建系统配置
func (s *SystemSettingService) CreateSystemSetting(ctx context.Context, setting *models.SystemSetting) (*models.SystemSetting, error) {
	// 检查配置键是否重复
	if err := s.checkSettingKeyUnique(ctx, setting.Key, 0); err != nil {
		return nil, err
	}

	// 验证配置值格式
	if err := s.validateSettingValue(setting.Type, setting.Value); err != nil {
		return nil, err
	}

	// 创建系统配置
	if err := dao.SystemSettingRepo.Create(ctx, setting); err != nil {
		logger.Error(ctx, "Failed to create system setting", zap.Error(err))
		return nil, errors.Wrap(err, "failed to create system setting")
	}

	logger.Info(ctx, "System setting created successfully",
		zap.Uint64("setting_id", setting.ID),
		zap.String("key", setting.Key),
		zap.String("category", setting.Category))

	return setting, nil
}

// UpdateSystemSetting 更新系统配置
func (s *SystemSettingService) UpdateSystemSetting(ctx context.Context, setting *models.SystemSetting) (*models.SystemSetting, error) {
	// 获取原配置信息用于验证
	original, err := dao.SystemSettingRepo.FindByID(ctx, setting.ID)
	if err != nil {
		return nil, errors.Wrap(err, "system setting not found")
	}

	// 检查配置键是否重复（排除自己）
	if setting.Key != original.Key {
		if err := s.checkSettingKeyUnique(ctx, setting.Key, setting.ID); err != nil {
			return nil, err
		}
	}

	// 验证配置值格式
	if err := s.validateSettingValue(setting.Type, setting.Value); err != nil {
		return nil, err
	}

	// 更新系统配置
	if err := dao.SystemSettingRepo.Update(ctx, setting); err != nil {
		logger.Error(ctx, "Failed to update system setting", zap.Error(err))
		return nil, errors.Wrap(err, "failed to update system setting")
	}

	logger.Info(ctx, "System setting updated successfully",
		zap.Uint64("setting_id", setting.ID),
		zap.String("key", setting.Key))

	return setting, nil
}

// DeleteSystemSettingBatch 批量删除系统配置
func (s *SystemSettingService) DeleteSystemSettingBatch(ctx context.Context, ids []uint64) error {
	var errs multierror.Error
	for _, id := range ids {
		// 检查是否为系统核心配置（不允许删除）
		if err := s.checkCoreSettingDeletion(ctx, id); err != nil {
			errs = *multierror.Append(&errs, err)
			continue
		}

		// 删除系统配置
		if err := dao.SystemSettingRepo.DeleteByID(ctx, id); err != nil {
			errs = *multierror.Append(&errs, errors.Wrapf(err, "failed to delete system setting: %d", id))
		}
	}
	return errs.ErrorOrNil()
}

// GetSystemSettingByCategory 根据分类获取系统配置列表
func (s *SystemSettingService) GetSystemSettingByCategory(ctx context.Context, category string) ([]*vo.SystemSetting, error) {
	// 查询指定分类的配置项
	settings, err := dao.SystemSettingRepo.QueryBuilder().
		Eq("category", category).
		OrderBy("key ASC").
		Find(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to query system settings")
	}

	// 转换为VO列表
	voList, err := s.PackSystemSettingVOList(ctx, settings)
	if err != nil {
		return nil, err
	}

	return voList, nil
}

// GetSystemSettingByKey 根据键获取系统配置
func (s *SystemSettingService) GetSystemSettingByKey(ctx context.Context, key string) (*models.SystemSetting, error) {
	setting, err := dao.SystemSettingRepo.QueryBuilder().
		Eq("key", key).
		Find(ctx)
	if err != nil || len(setting) == 0 {
		return nil, errors.Wrap(err, "system setting not found")
	}

	return &setting[0], nil
}

// GetSystemSettingValue 获取系统配置值（泛型方法）
func (s *SystemSettingService) GetSystemSettingValue(ctx context.Context, key string, defaultValue any) any {
	setting, err := s.GetSystemSettingByKey(ctx, key)
	if err != nil {
		return defaultValue
	}

	// 根据类型转换值
	switch setting.Type {
	case 1: // 文本类型
		return setting.Value
	case 2: // 数字类型
		if val, err := strconv.ParseFloat(setting.Value, 64); err == nil {
			// 尝试解析为整数，如果可以则返回整数
			if intVal, intErr := strconv.Atoi(setting.Value); intErr == nil {
				return intVal
			}
			return val
		}
	case 3: // 布尔类型
		if val, err := strconv.ParseBool(setting.Value); err == nil {
			return val
		}
	case 4: // JSON类型
		var result any
		if err := json.Unmarshal([]byte(setting.Value), &result); err == nil {
			return result
		}
	}

	return defaultValue
}

// SetSystemSetting 设置系统配置（如果不存在则创建，存在则更新）
func (s *SystemSettingService) SetSystemSetting(ctx context.Context, category, key, value, description string, settingType uint8, createBy uint64) error {
	// 尝试获取现有配置
	existing, err := s.GetSystemSettingByKey(ctx, key)
	if err != nil {
		// 配置不存在，创建新配置
		newSetting := &models.SystemSetting{
			Category:    category,
			Key:         key,
			Value:       value,
			Type:        settingType,
			Description: description,
			CreateBy:    createBy,
		}
		_, err = s.CreateSystemSetting(ctx, newSetting)
		return err
	}

	// 配置存在，更新配置
	existing.Category = category
	existing.Value = value
	existing.Type = settingType
	existing.Description = description
	_, err = s.UpdateSystemSetting(ctx, existing)
	return err
}

// PackSystemSettingVO 转换系统配置模型为VO（包含关联信息）
func (s *SystemSettingService) PackSystemSettingVO(ctx context.Context, setting *models.SystemSetting) (*vo.SystemSetting, error) {
	var voSetting vo.SystemSetting
	if err := copier.Copy(&voSetting, setting); err != nil {
		return nil, errors.Wrap(err, "failed to convert system setting model to VO")
	}

	// 查询创建人信息
	if createBy, err := dao.UserRepo.FindByID(ctx, setting.CreateBy); err == nil {
		var createByVO vo.User
		copier.Copy(&createByVO, createBy)
		voSetting.CreateBy = &createByVO
	}

	// 设置类型显示名称
	voSetting.TypeDisplay = s.getTypeDisplay(setting.Type)

	return &voSetting, nil
}

// PackSystemSettingVOList 批量转换系统配置模型为VO列表
func (s *SystemSettingService) PackSystemSettingVOList(ctx context.Context, settings []models.SystemSetting) ([]*vo.SystemSetting, error) {
	voList := make([]*vo.SystemSetting, 0, len(settings))

	for _, setting := range settings {
		voSetting, err := s.PackSystemSettingVO(ctx, &setting)
		if err != nil {
			return nil, err
		}
		voList = append(voList, voSetting)
	}

	return voList, nil
}

// 辅助方法

// checkSettingKeyUnique 检查配置键是否唯一
func (s *SystemSettingService) checkSettingKeyUnique(ctx context.Context, key string, excludeID uint64) error {
	query := dao.SystemSettingRepo.QueryBuilder().Eq("key", key)
	if excludeID > 0 {
		query = query.Ne("id", excludeID)
	}

	count, err := query.Count(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to check setting key uniqueness")
	}
	if count > 0 {
		return errors.New("setting key already exists")
	}
	return nil
}

// validateSettingValue 验证配置值格式
func (s *SystemSettingService) validateSettingValue(settingType uint8, value string) error {
	switch settingType {
	case 1: // 文本类型
		// 文本类型无需特殊验证
		return nil
	case 2: // 数字类型
		if _, err := strconv.ParseFloat(value, 64); err != nil {
			return errors.New("invalid number value")
		}
	case 3: // 布尔类型
		if _, err := strconv.ParseBool(value); err != nil {
			return errors.New("invalid boolean value")
		}
	case 4: // JSON类型
		var jsonData any
		if err := json.Unmarshal([]byte(value), &jsonData); err != nil {
			return errors.New("invalid JSON value")
		}
	default:
		return errors.New("invalid setting type")
	}
	return nil
}

// checkCoreSettingDeletion 检查是否为核心配置（不允许删除）
func (s *SystemSettingService) checkCoreSettingDeletion(ctx context.Context, id uint64) error {
	setting, err := dao.SystemSettingRepo.FindByID(ctx, id)
	if err != nil {
		return errors.Wrap(err, "setting not found")
	}

	// 定义不允许删除的核心配置
	coreSettings := []string{
		"system.name",
		"system.version",
		"database.version",
		"security.jwt_secret",
	}

	for _, coreKey := range coreSettings {
		if setting.Key == coreKey {
			return errors.Errorf("core setting '%s' cannot be deleted", setting.Key)
		}
	}

	return nil
}

// getTypeDisplay 获取类型显示名称
func (s *SystemSettingService) getTypeDisplay(settingType uint8) string {
	typeMap := map[uint8]string{
		1: "文本",
		2: "数字",
		3: "布尔",
		4: "JSON",
	}

	if display, exists := typeMap[settingType]; exists {
		return display
	}
	return "未知类型"
}
