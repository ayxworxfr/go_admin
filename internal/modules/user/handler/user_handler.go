package handler

import (
	stdctx "context"

	"github.com/ayxworxfr/go_admin/internal/modules/user/dto"
	"github.com/ayxworxfr/go_admin/internal/modules/user/model"
	"github.com/ayxworxfr/go_admin/internal/modules/user/service"
	"github.com/ayxworxfr/go_admin/pkg/context"
	"github.com/ayxworxfr/go_admin/pkg/jwtauth"
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

// Handler 用户管理接口
type Handler struct {
	svc          *service.Service
	roleAssigner RoleAssigner
	permResolver PermissionPathResolver
	jwt          *jwtauth.JWT
}

// NewHandler 创建用户处理器，依赖均由 Container 装配注入。
func NewHandler(svc *service.Service, roleAssigner RoleAssigner, permResolver PermissionPathResolver, jwt *jwtauth.JWT) *Handler {
	return &Handler{svc: svc, roleAssigner: roleAssigner, permResolver: permResolver, jwt: jwt}
}

func toUserResponse(u *model.User) (*dto.UserResponse, error) {
	var resp dto.UserResponse
	if err := copier.Copy(&resp, u); err != nil {
		return nil, err
	}
	return &resp, nil
}

// @route Get /user
func (h *Handler) GetUser(c *context.Context, req *dto.GetUserRequest) *context.Response {
	u, err := h.svc.FindByID(c.Context(), req.ID)
	if err != nil {
		return context.DatabaseError(err)
	}

	resp, err := toUserResponse(u)
	if err != nil {
		return context.InternalError(err)
	}
	return context.Success(resp)
}

// @route Post /user
func (h *Handler) CreateUser(c *context.Context, req *dto.CreateUserRequest) *context.Response {
	u, err := h.svc.Create(c.Context(), req)
	if err != nil {
		return context.DatabaseError(err)
	}

	if err := h.roleAssigner.AssignRoles(c.Context(), u.ID, req.RoleIDs); err != nil {
		return context.DatabaseError(err)
	}

	resp, err := toUserResponse(u)
	if err != nil {
		return context.InternalError(err)
	}
	return context.Success(resp)
}

// @route Put /user
func (h *Handler) UpdateUser(c *context.Context, req *dto.UpdateUserRequest) *context.Response {
	u, err := h.svc.Update(c.Context(), req)
	if err != nil {
		return context.DatabaseError(err)
	}

	if req.RoleIDs != nil {
		if err := h.roleAssigner.AssignRoles(c.Context(), u.ID, *req.RoleIDs); err != nil {
			return context.DatabaseError(err)
		}
	}

	resp, err := toUserResponse(u)
	if err != nil {
		return context.InternalError(err)
	}
	return context.Success(resp)
}

// @route Get /user/list
func (h *Handler) GetUserList(c *context.Context, req *dto.GetUserListRequest) *context.Response {
	data, total, err := h.svc.List(c.Context(), req)
	if err != nil {
		return context.DatabaseError(err)
	}

	voList := make([]*dto.UserResponse, 0, len(data))
	for i := range data {
		resp, err := toUserResponse(&data[i])
		if err != nil {
			return context.InternalError(err)
		}
		voList = append(voList, resp)
	}
	return context.PageSuccess(voList, total)
}

// @route Delete /user
func (h *Handler) DeleteUser(c *context.Context, req *dto.DeleteUserRequest) *context.Response {
	if err := h.svc.DeleteUsers(c.Context(), req.IDs); err != nil {
		return context.DatabaseError(err)
	}
	return context.NoContent()
}

// @route Get /user/routes
func (h *Handler) GetUserRoutes(c *context.Context) *context.Response {
	claims, err := h.jwt.ContextClaims(c.RequestContext)
	if err != nil {
		return context.Unauthorized("Invalid token")
	}

	userID, err := h.jwt.GetUserIDUint64(c.RequestContext)
	if err != nil {
		return context.Unauthorized("Invalid token")
	}

	permissionPaths, err := h.permResolver.GetUserPermissionPaths(c.Context(), userID)
	if err != nil {
		return context.InternalError(err)
	}

	result := &dto.UserRoutes{
		Username: claims.Nice,
		Role:     claims.RoleKey,
		Routes:   permissionPaths,
	}
	return context.Success(result)
}

// @route Get /user/current
func (h *Handler) GetUserCurrent(c *context.Context) *context.Response {
	claims, err := h.jwt.ContextClaims(c.RequestContext)
	if err != nil {
		return context.Unauthorized("Invalid token")
	}
	result := map[string]any{
		"name":   claims.Nice,
		"avatar": "https://gw.alipayobjects.com/zos/antfincdn/XAosXuNZyF/BiazfanxmamNRoxxVxka.png",
		"userid": claims.Identity,
		"email":  "antdesign@alipay.com",
		"access": claims.RoleKey,
	}
	return context.Success(result)
}
