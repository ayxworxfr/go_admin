package handler

import (
	stdctx "context"

	"github.com/ayxworxfr/go_admin/internal/modules/user/dto"
	"github.com/ayxworxfr/go_admin/internal/modules/user/model"
	"github.com/ayxworxfr/go_admin/internal/modules/user/service"
	"github.com/ayxworxfr/go_admin/pkg/reqctx"
	"github.com/jinzhu/copier"
)

// RoleAssigner 是创建/更新用户时分配角色所需的最小接口，由 iam 模块的
// UserRoleService 实现。user 模块的 handler 只知道"分配一组角色 ID"这一个
// 动作，不感知角色分配的存储细节，避免 user -> iam 产生反向依赖。
type RoleAssigner interface {
	AssignRoles(ctx stdctx.Context, userID uint64, roleIDs []uint64) error
}

// PermissionPathResolver 是渲染用户可访问路由所需的最小接口，由 iam 模块的
// PermissionChecker 实现。
type PermissionPathResolver interface {
	GetUserPermissionPaths(ctx stdctx.Context, userID uint64) ([]string, error)
}

// Handler 用户管理接口。当前用户信息只从中间件注入的 JWT 载荷读取，
// 不持有 *jwtauth.JWT——签名校验与签发都在 iam/auth 侧完成。
type Handler struct {
	svc          *service.Service
	roleAssigner RoleAssigner
	permResolver PermissionPathResolver
}

// NewHandler 创建用户处理器，依赖均由 Container 装配注入。
func NewHandler(svc *service.Service, roleAssigner RoleAssigner, permResolver PermissionPathResolver) *Handler {
	return &Handler{svc: svc, roleAssigner: roleAssigner, permResolver: permResolver}
}

func toUserResponse(u *model.User) (*dto.UserResponse, error) {
	var resp dto.UserResponse
	if err := copier.Copy(&resp, u); err != nil {
		return nil, err
	}
	return &resp, nil
}

// @route Get /user
func (h *Handler) GetUser(c *reqctx.Context, req *dto.GetUserRequest) *reqctx.Response {
	var (
		u   *model.User
		err error
	)
	switch {
	case req.ID > 0:
		u, err = h.svc.FindByID(c.Context(), req.ID)
	case req.Username != "":
		u, err = h.svc.FindByUsername(c.Context(), req.Username)
	default:
		return reqctx.ParamError("id or username is required")
	}
	if err != nil {
		return reqctx.DatabaseError(err)
	}

	resp, err := toUserResponse(u)
	if err != nil {
		return reqctx.InternalError(err)
	}
	return reqctx.Success(resp)
}

// @route Post /user
func (h *Handler) CreateUser(c *reqctx.Context, req *dto.CreateUserRequest) *reqctx.Response {
	u, err := h.svc.Create(c.Context(), req)
	if err != nil {
		return reqctx.DatabaseError(err)
	}

	if err := h.roleAssigner.AssignRoles(c.Context(), u.ID, req.RoleIDs); err != nil {
		return reqctx.DatabaseError(err)
	}

	resp, err := toUserResponse(u)
	if err != nil {
		return reqctx.InternalError(err)
	}
	return reqctx.Success(resp)
}

// @route Put /user
func (h *Handler) UpdateUser(c *reqctx.Context, req *dto.UpdateUserRequest) *reqctx.Response {
	u, err := h.svc.Update(c.Context(), req)
	if err != nil {
		return reqctx.DatabaseError(err)
	}

	if req.RoleIDs != nil {
		if err := h.roleAssigner.AssignRoles(c.Context(), u.ID, *req.RoleIDs); err != nil {
			return reqctx.DatabaseError(err)
		}
	}

	resp, err := toUserResponse(u)
	if err != nil {
		return reqctx.InternalError(err)
	}
	return reqctx.Success(resp)
}

// @route Get /user/list
func (h *Handler) GetUserList(c *reqctx.Context, req *dto.GetUserListRequest) *reqctx.Response {
	data, total, err := h.svc.List(c.Context(), req)
	if err != nil {
		return reqctx.DatabaseError(err)
	}

	voList := make([]*dto.UserResponse, 0, len(data))
	for i := range data {
		resp, err := toUserResponse(&data[i])
		if err != nil {
			return reqctx.InternalError(err)
		}
		voList = append(voList, resp)
	}
	return reqctx.PageSuccess(voList, total)
}

// @route Delete /user
func (h *Handler) DeleteUser(c *reqctx.Context, req *dto.DeleteUserRequest) *reqctx.Response {
	if err := h.svc.DeleteUsers(c.Context(), req.IDs); err != nil {
		return reqctx.DatabaseError(err)
	}
	return reqctx.NoContent()
}

// @route Get /user/routes
func (h *Handler) GetUserRoutes(c *reqctx.Context) *reqctx.Response {
	claims, err := c.Claims()
	if err != nil {
		return reqctx.Unauthorized("Invalid token")
	}
	userID, err := c.UserID()
	if err != nil {
		return reqctx.Unauthorized("Invalid token")
	}

	permissionPaths, err := h.permResolver.GetUserPermissionPaths(c.Context(), userID)
	if err != nil {
		return reqctx.InternalError(err)
	}

	result := &dto.UserRoutes{
		Username: claims.Nice,
		Role:     claims.RoleKey,
		Routes:   permissionPaths,
	}
	return reqctx.Success(result)
}

// @route Get /user/current
func (h *Handler) GetUserCurrent(c *reqctx.Context) *reqctx.Response {
	claims, err := c.Claims()
	if err != nil {
		return reqctx.Unauthorized("Invalid token")
	}
	result := map[string]any{
		"name":   claims.Nice,
		"avatar": "https://gw.alipayobjects.com/zos/antfincdn/XAosXuNZyF/BiazfanxmamNRoxxVxka.png",
		"userid": claims.Identity,
		"email":  "antdesign@alipay.com",
		"access": claims.RoleKey,
	}
	return reqctx.Success(result)
}
