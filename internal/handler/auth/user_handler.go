package auth_handler

import (
	"github.com/ayxworxfr/go_admin/internal/dao"
	"github.com/ayxworxfr/go_admin/internal/domain/models"
	"github.com/ayxworxfr/go_admin/internal/domain/params"
	"github.com/ayxworxfr/go_admin/internal/domain/vo"
	"github.com/ayxworxfr/go_admin/internal/service"
	"github.com/ayxworxfr/go_admin/pkg/context"
	"github.com/ayxworxfr/go_admin/pkg/jwtauth"
	"github.com/jinzhu/copier"
)

type IUserHandler interface {
	GetUser(c *context.Context, req *params.GetUserRequest) *context.Response
	CreateUser(c *context.Context, req *params.CreateUserRequest) *context.Response
	UpdateUser(c *context.Context, req *params.UpdateUserRequest) *context.Response
	GetUserList(c *context.Context, req *params.GetUserListRequest) *context.Response
	DeleteUser(c *context.Context, req *params.DeleteUserRequest) *context.Response
	GetUserRoutes(c *context.Context) *context.Response
	GetUserCurrent(c *context.Context) *context.Response
}

type UserHandler struct{}

// @route Get /user
func (h *UserHandler) GetUser(c *context.Context, req *params.GetUserRequest) *context.Response {
	var query models.User
	if err := copier.Copy(&query, &req); err != nil {
		return context.ParamError(err)
	}

	data, err := dao.UserRepo.Find(c.Context(), &query)
	if err != nil {
		return context.DatabaseError(err) // 数据库错误专用响应
	}

	var result vo.User
	if err := copier.Copy(&result, &data); err != nil {
		return context.InternalError(err) // 内部逻辑错误
	}

	return context.Success(result)
}

// @route Post /user
func (h *UserHandler) CreateUser(c *context.Context, req *params.CreateUserRequest) *context.Response {
	var data models.User
	if err := copier.Copy(&data, &req); err != nil {
		return context.ParamError(err)
	}

	// 加密密码
	data.EncryptPassword()

	if err := dao.UserRepo.Create(c.Context(), &data); err != nil {
		return context.DatabaseError(err)
	}

	return context.Success(data)
}

// @route Put /user
func (h *UserHandler) UpdateUser(c *context.Context, req *params.UpdateUserRequest) *context.Response {
	query := models.User{Username: req.Username}

	data, err := dao.UserRepo.Find(c.Context(), &query)
	if err != nil {
		return context.DatabaseError(err) // 数据库错误专用响应
	}

	if err := copier.Copy(&data, &req); err != nil {
		return context.InternalError(err)
	}
	if data.Password != "" {
		data.EncryptPassword()
	}

	if err := dao.UserRepo.Update(c.Context(), data); err != nil {
		return context.DatabaseError(err)
	}

	return context.Success(data)
}

// @route Get /user/list
func (h *UserHandler) GetUserList(c *context.Context, req *params.GetUserListRequest) *context.Response {
	data, total, err := dao.UserRepo.FindPage(c.Context(), req, req.Limit, req.Offset)
	if err != nil {
		return context.DatabaseError(err)
	}

	var voList []vo.User
	if err := copier.Copy(&voList, &data); err != nil {
		return context.InternalError(err)
	}

	return context.PageSuccess(voList, total)
}

// @route Delete /user
func (h *UserHandler) DeleteUser(c *context.Context, req *params.DeleteUserRequest) *context.Response {
	var query models.User
	if err := copier.Copy(&query, &req); err != nil {
		return context.ParamError(err)
	}

	if err := dao.UserRepo.Delete(c.Context(), &query); err != nil {
		return context.DatabaseError(err)
	}

	return context.NoContent()
}

// @route Get /user/routes
func (h *UserHandler) GetUserRoutes(c *context.Context) *context.Response {
	claims, err := jwtauth.Instance.ContextClaims(c.RequestContext)
	if err != nil {
		return context.Unauthorized("Invalid token")
	}

	userID, err := jwtauth.Instance.GetUserIDUint64(c.RequestContext)
	if err != nil {
		return context.Unauthorized("Invalid token")
	}
	permissionPaths, err := service.PermissionServiceInstance.GetUserPermissionPaths(c.Context(), userID)
	if err != nil {
		return context.InternalError(err)
	}
	result := vo.NewUserRoutes(claims, permissionPaths)
	return context.Success(result)
}

func (h *UserHandler) GetUserCurrent(c *context.Context) *context.Response {
	claims, err := jwtauth.Instance.ContextClaims(c.RequestContext)
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
