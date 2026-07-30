package service

import (
	"context"
	"strings"

	"github.com/ayxworxfr/go_admin/internal/modules/iam/cache"
	"github.com/ayxworxfr/go_admin/internal/modules/iam/model"
	"github.com/ayxworxfr/go_admin/pkg/logger"
	"github.com/pkg/errors"
	"github.com/samber/lo"
	"go.uber.org/zap"
)

// PermissionChecker 是鉴权热路径：JWTAuthMiddleware 每个受保护请求都会调用
// HasPermission。它组合 UserRoleService + RoleService 复用"用户->角色->权限"
// 的查询逻辑，自己只额外持有 permissionRepo（用于递归 CTE 查子权限）和缓存，
// 不重新实现一遍角色/权限的表连接查询。
//
// 这是旧版 PermissionService 里"管理台 CRUD"与"每请求鉴权判断"两类关注点中
// 后一类的归宿：有状态（缓存）、对性能敏感，理应独立成一个组件。
type PermissionChecker struct {
	userRoleSvc *UserRoleService
	roleSvc     *RoleService
	cache       cache.PermissionCache
}

// NewPermissionChecker 创建权限检查器
func NewPermissionChecker(userRoleSvc *UserRoleService, roleSvc *RoleService, permCache cache.PermissionCache) *PermissionChecker {
	return &PermissionChecker{
		userRoleSvc: userRoleSvc,
		roleSvc:     roleSvc,
		cache:       permCache,
	}
}

// HasPermission 检查用户是否拥有指定方法+路径的权限（带缓存）
func (c *PermissionChecker) HasPermission(ctx context.Context, userID uint64, method, path string) (bool, error) {
	if permissionMap, ok := c.cache.Get(userID); ok {
		return matchPath(permissionMap, method, path), nil
	}

	permissions, err := c.getUserAllPermissions(ctx, userID)
	if err != nil {
		logger.Error(ctx, "Failed to retrieve user permissions", zap.Error(err), zap.Uint64("user_id", userID))
		return false, errors.Wrap(err, "failed to retrieve user permissions")
	}

	permissionMap := make(map[string]bool, len(permissions))
	for _, perm := range permissions {
		if perm.Method != "" && perm.Path != "" {
			permissionMap[perm.Method+":"+perm.Path] = true
		}
	}
	c.cache.Set(userID, permissionMap)

	return matchPath(permissionMap, method, path), nil
}

// GetUserPermissionPaths 获取用户有权限访问的所有 "method:path"，
// 供 user 模块渲染前端路由/菜单用（不走缓存，调用频率远低于 HasPermission）
func (c *PermissionChecker) GetUserPermissionPaths(ctx context.Context, userID uint64) ([]string, error) {
	permissions, err := c.getUserAllPermissions(ctx, userID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to retrieve user permissions")
	}

	paths := make([]string, 0, len(permissions))
	for _, perm := range permissions {
		if perm.Method != "" && perm.Path != "" {
			paths = append(paths, perm.Method+":"+perm.Path)
		}
	}
	return paths, nil
}

// GetUserPermissions 获取用户的权限详情列表（供管理台展示，非鉴权路径，不走缓存）
func (c *PermissionChecker) GetUserPermissions(ctx context.Context, userID uint64) ([]model.Permission, error) {
	return c.getUserAllPermissions(ctx, userID)
}

// InvalidateUser 供角色/权限分配变更后清除单个用户的缓存
func (c *PermissionChecker) InvalidateUser(userID uint64) {
	c.cache.InvalidateUser(userID)
}

// InvalidateAll 供角色-权限关系整体变更后清空缓存。
// 缓存清理统一收口在这里，其它组件（RoleService/UserRoleService）不直接
// 持有缓存引用，避免"拆分后清理逻辑散落在多处、漏掉某个新组件"。
func (c *PermissionChecker) InvalidateAll() {
	c.cache.InvalidateAll()
}

// getUserAllPermissions 获取用户的所有权限，包含通过父子关系递归展开的子权限
func (c *PermissionChecker) getUserAllPermissions(ctx context.Context, userID uint64) ([]model.Permission, error) {
	roles, err := c.userRoleSvc.RetrieveRolesByUserID(ctx, userID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to retrieve user roles")
	}
	if len(roles) == 0 {
		return []model.Permission{}, nil
	}

	var permissions []model.Permission
	for _, role := range roles {
		perms, err := c.roleSvc.RetrievePermissionByRoleID(ctx, role.ID)
		if err != nil {
			return nil, errors.Wrap(err, "failed to retrieve role permissions")
		}
		permissions = append(permissions, perms...)
	}
	if len(permissions) == 0 {
		return []model.Permission{}, nil
	}

	parentIDs := lo.Map(permissions, func(p model.Permission, _ int) uint64 { return p.ID })
	childPermissions, err := c.roleSvc.getChildPermissionsWithCTE(ctx, parentIDs)
	if err != nil {
		return nil, errors.Wrap(err, "failed to retrieve child permissions")
	}
	permissions = append(permissions, childPermissions...)

	return lo.UniqBy(permissions, func(p model.Permission) uint64 { return p.ID }), nil
}

// matchPath 检查权限映射中是否有匹配的 method+path，支持方法通配符 "*"
// 与路径尾部通配符 "/*"
func matchPath(permissionMap map[string]bool, method, path string) bool {
	if permissionMap[method+":"+path] {
		return true
	}

	for permKey := range permissionMap {
		parts := strings.SplitN(permKey, ":", 2)
		if len(parts) != 2 {
			continue
		}
		permMethod, permPath := parts[0], parts[1]

		if permMethod != method && permMethod != "*" {
			continue
		}
		if strings.HasSuffix(permPath, "/*") {
			prefix := strings.TrimSuffix(permPath, "/*")
			if strings.HasPrefix(path, prefix) {
				return true
			}
		}
	}
	return false
}
