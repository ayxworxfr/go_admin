package service

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/ayxworxfr/go_admin/internal/modules/systemsetting/dto"
	"github.com/ayxworxfr/go_admin/internal/modules/systemsetting/model"
	usersvc "github.com/ayxworxfr/go_admin/internal/modules/user/service"
	"github.com/ayxworxfr/go_admin/pkg/logger"
	pkgrepo "github.com/ayxworxfr/go_admin/pkg/repository"
	"github.com/hashicorp/go-multierror"
	"github.com/jinzhu/copier"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

// 配置类型常量，与前端约定的枚举保持一致
const (
	TypeText   uint8 = 1
	TypeNumber uint8 = 2
	TypeBool   uint8 = 3
	TypeJSON   uint8 = 4
)

// coreSettingKeys 核心配置不允许被删除
var coreSettingKeys = map[string]struct{}{
	"system.name":         {},
	"system.version":      {},
	"database.version":    {},
	"security.jwt_secret": {},
}

// Service 系统配置服务：与 iam/user 之间无业务耦合，是一个独立的限界上下文，
// 唯一的跨模块依赖是展示 create_by 时需要查一下用户名，通过 user.UserFinder
// 这个最小接口完成，不直连 user 的仓储。
type Service struct {
	repo       pkgrepo.Repository[model.SystemSetting]
	userFinder usersvc.UserFinder
}

// NewService 创建系统配置服务。repo 直接调用 pkg/repository 的泛型构造函数
// 生成，不再单独包一层 internal/repository 子包，理由见 user 模块 NewService 的注释。
func NewService(processor pkgrepo.ORMProcessor, userFinder usersvc.UserFinder) *Service {
	return &Service{
		repo:       pkgrepo.NewRepository[model.SystemSetting](processor),
		userFinder: userFinder,
	}
}

// Create 创建系统配置
func (s *Service) Create(ctx context.Context, req *dto.CreateSystemSettingRequest, createBy uint64) (*dto.SystemSettingResponse, error) {
	if err := s.checkKeyUnique(ctx, req.Key, 0); err != nil {
		return nil, err
	}
	if err := validateValue(req.Type, req.Value); err != nil {
		return nil, err
	}

	setting := &model.SystemSetting{
		Category:    req.Category,
		Key:         req.Key,
		Value:       req.Value,
		Type:        req.Type,
		Description: req.Description,
		CreateBy:    createBy,
	}
	if err := s.repo.Create(ctx, setting); err != nil {
		logger.Error(ctx, "Failed to create system setting", zap.Error(err))
		return nil, errors.Wrap(err, "failed to create system setting")
	}

	logger.Info(ctx, "System setting created successfully",
		zap.Uint64("setting_id", setting.ID), zap.String("key", setting.Key), zap.String("category", setting.Category))
	return s.toResponse(ctx, setting)
}

// Update 更新系统配置
func (s *Service) Update(ctx context.Context, req *dto.UpdateSystemSettingRequest) (*dto.SystemSettingResponse, error) {
	setting, err := s.repo.FindByID(ctx, req.ID)
	if err != nil {
		return nil, errors.Wrap(err, "system setting not found")
	}

	if req.Key != "" && req.Key != setting.Key {
		if err := s.checkKeyUnique(ctx, req.Key, req.ID); err != nil {
			return nil, err
		}
	}
	if err := validateValue(req.Type, req.Value); err != nil {
		return nil, err
	}

	if err := copier.Copy(setting, req); err != nil {
		return nil, errors.Wrap(err, "failed to copy request to system setting")
	}

	if err := s.repo.Update(ctx, setting); err != nil {
		logger.Error(ctx, "Failed to update system setting", zap.Error(err))
		return nil, errors.Wrap(err, "failed to update system setting")
	}

	logger.Info(ctx, "System setting updated successfully", zap.Uint64("setting_id", setting.ID), zap.String("key", setting.Key))
	return s.toResponse(ctx, setting)
}

// DeleteBatch 批量删除系统配置（核心配置不允许删除）
func (s *Service) DeleteBatch(ctx context.Context, ids []uint64) error {
	var result *multierror.Error
	for _, id := range ids {
		if err := s.delete(ctx, id); err != nil {
			result = multierror.Append(result, err)
		}
	}
	return result.ErrorOrNil()
}

func (s *Service) delete(ctx context.Context, id uint64) error {
	setting, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return errors.Wrapf(err, "setting not found: %d", id)
	}
	if _, isCore := coreSettingKeys[setting.Key]; isCore {
		return errors.Errorf("core setting '%s' cannot be deleted", setting.Key)
	}
	if err := s.repo.DeleteByID(ctx, id); err != nil {
		return errors.Wrapf(err, "failed to delete system setting: %d", id)
	}
	return nil
}

// Get 获取单个系统配置
func (s *Service) Get(ctx context.Context, id uint64) (*dto.SystemSettingResponse, error) {
	setting, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, errors.Wrap(err, "system setting not found")
	}
	return s.toResponse(ctx, setting)
}

// List 分页查询系统配置列表
func (s *Service) List(ctx context.Context, req *dto.GetSystemSettingListRequest) ([]*dto.SystemSettingResponse, int64, error) {
	settings, total, err := s.repo.FindPage(ctx, req, req.Limit, req.Offset)
	if err != nil {
		return nil, 0, errors.Wrap(err, "failed to retrieve system settings")
	}

	result, err := s.toResponseList(ctx, settings)
	if err != nil {
		return nil, 0, err
	}
	return result, total, nil
}

// GetByCategory 根据分类获取系统配置列表
func (s *Service) GetByCategory(ctx context.Context, category string) ([]*dto.SystemSettingResponse, error) {
	settings, err := s.repo.QueryBuilder().Eq("category", category).OrderBy("key ASC").Find(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to query system settings")
	}
	return s.toResponseList(ctx, settings)
}

// GetValue 获取系统配置值并按配置类型转换，取不到或转换失败时返回 defaultValue
func (s *Service) GetValue(ctx context.Context, key string, defaultValue any) any {
	setting, err := s.getByKey(ctx, key)
	if err != nil {
		return defaultValue
	}

	switch setting.Type {
	case TypeText:
		return setting.Value
	case TypeNumber:
		if intVal, err := strconv.Atoi(setting.Value); err == nil {
			return intVal
		}
		if val, err := strconv.ParseFloat(setting.Value, 64); err == nil {
			return val
		}
	case TypeBool:
		if val, err := strconv.ParseBool(setting.Value); err == nil {
			return val
		}
	case TypeJSON:
		var result any
		if err := json.Unmarshal([]byte(setting.Value), &result); err == nil {
			return result
		}
	}
	return defaultValue
}

// Set 设置系统配置：存在则更新，不存在则创建
func (s *Service) Set(ctx context.Context, category, key, value, description string, settingType uint8, createBy uint64) error {
	existing, err := s.getByKey(ctx, key)
	if err != nil {
		_, err = s.Create(ctx, &dto.CreateSystemSettingRequest{
			Category: category, Key: key, Value: value, Type: settingType, Description: description,
		}, createBy)
		return err
	}

	_, err = s.Update(ctx, &dto.UpdateSystemSettingRequest{
		ID: existing.ID, Category: category, Key: key, Value: value, Type: settingType, Description: description,
	})
	return err
}

func (s *Service) getByKey(ctx context.Context, key string) (*model.SystemSetting, error) {
	settings, err := s.repo.QueryBuilder().Eq("key", key).Find(ctx)
	if err != nil || len(settings) == 0 {
		return nil, errors.Wrap(err, "system setting not found")
	}
	return &settings[0], nil
}

func (s *Service) checkKeyUnique(ctx context.Context, key string, excludeID uint64) error {
	query := s.repo.QueryBuilder().Eq("key", key)
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

func validateValue(settingType uint8, value string) error {
	switch settingType {
	case TypeText:
		return nil
	case TypeNumber:
		if _, err := strconv.ParseFloat(value, 64); err != nil {
			return errors.New("invalid number value")
		}
	case TypeBool:
		if _, err := strconv.ParseBool(value); err != nil {
			return errors.New("invalid boolean value")
		}
	case TypeJSON:
		var jsonData any
		if err := json.Unmarshal([]byte(value), &jsonData); err != nil {
			return errors.New("invalid JSON value")
		}
	default:
		return errors.New("invalid setting type")
	}
	return nil
}

func typeDisplay(settingType uint8) string {
	switch settingType {
	case TypeText:
		return "文本"
	case TypeNumber:
		return "数字"
	case TypeBool:
		return "布尔"
	case TypeJSON:
		return "JSON"
	default:
		return "未知类型"
	}
}

// toResponse 转换为响应对象，并按 create_by 附加创建人信息
func (s *Service) toResponse(ctx context.Context, setting *model.SystemSetting) (*dto.SystemSettingResponse, error) {
	var resp dto.SystemSettingResponse
	if err := copier.Copy(&resp, setting); err != nil {
		return nil, errors.Wrap(err, "failed to convert system setting to response")
	}
	resp.TypeDisplay = typeDisplay(setting.Type)

	if creator, err := s.userFinder.FindByID(ctx, setting.CreateBy); err == nil {
		resp.CreateBy = &dto.CreatorResponse{ID: creator.ID, Username: creator.Username}
	}
	return &resp, nil
}

func (s *Service) toResponseList(ctx context.Context, settings []model.SystemSetting) ([]*dto.SystemSettingResponse, error) {
	result := make([]*dto.SystemSettingResponse, 0, len(settings))
	for i := range settings {
		resp, err := s.toResponse(ctx, &settings[i])
		if err != nil {
			return nil, err
		}
		result = append(result, resp)
	}
	return result, nil
}
