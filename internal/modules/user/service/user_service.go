package service

import (
	"context"

	"github.com/ayxworxfr/go_admin/internal/modules/user/dto"
	"github.com/ayxworxfr/go_admin/internal/modules/user/model"
	"github.com/ayxworxfr/go_admin/pkg/crypter"
	"github.com/ayxworxfr/go_admin/pkg/logger"
	pkgrepo "github.com/ayxworxfr/go_admin/pkg/repository"
	"github.com/hashicorp/go-multierror"
	"github.com/jinzhu/copier"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

// Service 用户服务：负责用户 CRUD 与密码校验。
// 密码哈希算法通过 crypter.PasswordHasher 接口注入（策略模式），
// 替换旧版写死调用全局 crypter.Instance 的方式，换算法只需换一个实现，
// Service 本身不用改。
type Service struct {
	repo   *pkgrepo.Repository[model.User]
	hasher crypter.PasswordHasher
}

// NewService 创建用户服务。db 用于构造内部仓储，hasher 由 Container 统一装配。
//
// repo 字段直接调用 pkg/repository 的泛型构造函数生成，不再单独包一层
// internal/repository 子包——这里没有任何自定义查询，repo 字段本身是
// unexported，handler 拿不到 *Service 的内部字段，多一层子包只是重复
// Go 已经免费提供的封装，不需要为一个单行包装函数多开一个包。
func NewService(db *pkgrepo.DB, hasher crypter.PasswordHasher) *Service {
	return &Service{
		repo:   pkgrepo.NewRepository[model.User](db),
		hasher: hasher,
	}
}

// FindByID 实现 UserFinder：按 ID 查用户
func (s *Service) FindByID(ctx context.Context, id uint64) (*model.User, error) {
	return s.repo.FindByID(ctx, id)
}

// FindByUsername 实现 UserFinder：按用户名查用户
func (s *Service) FindByUsername(ctx context.Context, username string) (*model.User, error) {
	return s.repo.Find(ctx, &model.User{Username: username})
}

// VerifyPassword 实现 UserFinder：密码校验逻辑封闭在 user 模块内，
// 调用方（iam.AuthService）只拿到 true/false，不接触哈希细节。
func (s *Service) VerifyPassword(u *model.User, plainPassword string) bool {
	return s.hasher.Verify(plainPassword, u.PasswordHash)
}

// Create 创建用户（明文密码只存在于 DTO；落库前在此哈希为 PasswordHash）
func (s *Service) Create(ctx context.Context, req *dto.CreateUserRequest) (*model.User, error) {
	var u model.User
	// 字段名刻意不同：req.Password（明文）不会被 copier 拷进 u.PasswordHash（哈希）
	if err := copier.Copy(&u, req); err != nil {
		return nil, errors.Wrap(err, "failed to copy request to user")
	}

	hashed, err := s.hasher.Hash(req.Password)
	if err != nil {
		return nil, errors.Wrap(err, "failed to hash password")
	}
	u.PasswordHash = hashed

	if err := s.repo.Create(ctx, &u); err != nil {
		logger.Error(ctx, "Failed to create user", zap.Error(err))
		return nil, errors.Wrap(err, "failed to create user")
	}
	return &u, nil
}

// Update 更新用户。密码为空表示不修改，保留原 PasswordHash。
func (s *Service) Update(ctx context.Context, req *dto.UpdateUserRequest) (*model.User, error) {
	u, err := s.repo.FindByID(ctx, req.ID)
	if err != nil {
		logger.Error(ctx, "Failed to retrieve user", zap.Error(err), zap.Uint64("user_id", req.ID))
		return nil, errors.Wrap(err, "failed to retrieve user")
	}

	// copier 只按同名字段拷贝；Password ≠ PasswordHash，原哈希天然不会被明文覆盖
	if err := copier.Copy(u, req); err != nil {
		return nil, errors.Wrap(err, "failed to copy request to user")
	}

	if req.Password != "" {
		hashed, err := s.hasher.Hash(req.Password)
		if err != nil {
			return nil, errors.Wrap(err, "failed to hash password")
		}
		u.PasswordHash = hashed
	}

	if err := s.repo.Update(ctx, u); err != nil {
		logger.Error(ctx, "Failed to update user", zap.Error(err), zap.Uint64("user_id", req.ID))
		return nil, errors.Wrap(err, "failed to update user")
	}
	return u, nil
}

// DeleteUsers 按 ID 批量删除用户。
//
// 旧版实现把 DeleteUserRequest{IDs} 直接 copier.Copy 进 model.User 再整体
// Delete：由于两者字段名不匹配，拷贝后的 model.User 是零值，Delete 会按
// "ID=0" 这个（几乎必然不存在的）条件删除，实际上什么都没删掉——请求方
// 以为删除成功，数据库里记录原样还在。这里改成逐个按 ID 删除并收集错误，
// 才是"删除请求里的每一个 ID"这句需求本身该有的实现。
func (s *Service) DeleteUsers(ctx context.Context, ids []uint64) error {
	var result *multierror.Error
	for _, id := range ids {
		if err := s.repo.DeleteByID(ctx, id); err != nil {
			result = multierror.Append(result, errors.Wrapf(err, "failed to delete user %d", id))
		}
	}
	if err := result.ErrorOrNil(); err != nil {
		logger.Error(ctx, "Failed to delete users", zap.Error(err), zap.Uint64s("user_ids", ids))
		return err
	}
	return nil
}

// List 分页查询用户列表
func (s *Service) List(ctx context.Context, req *dto.GetUserListRequest) ([]model.User, int64, error) {
	return s.repo.FindPage(ctx, req, req.Limit, req.Offset)
}
