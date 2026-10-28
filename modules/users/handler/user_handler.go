package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/longlbh/appdist-be/common"
	"github.com/longlbh/appdist-be/db/sqlc"
	"github.com/longlbh/appdist-be/middlewares"
	"github.com/longlbh/appdist-be/modules/users/dto"
	"github.com/longlbh/appdist-be/modules/users/services"
)

type UserHandler struct {
	svc *services.UserService
}

func NewUserHandler(svc *services.UserService) *UserHandler {
	return &UserHandler{svc: svc}
}

// Me godoc
//
//	@Summary	Thông tin tài khoản đang đăng nhập
//	@Tags		users
//	@Produce	json
//	@Success	200	{object}	common.SuccessRes{data=dto.UserDTO}
//	@Failure	401	{object}	common.AppError
//	@Failure	403	{object}	common.AppError
//	@Failure	500	{object}	common.AppError
//	@Security	BearerAuth
//	@Router		/api/v1/users/me [get]
func (h *UserHandler) Me() gin.HandlerFunc {
	return common.Handle(func(c *gin.Context) (any, error) {
		user, err := middlewares.CurrentAccount(c)
		if err != nil {
			return nil, err
		}
		return dto.FromSQLC(user), nil
	})
}

// ChangePassword godoc
//
//	@Summary	Đổi mật khẩu của chính mình
//	@Tags		users
//	@Accept		json
//	@Produce	json
//	@Param		body	body		dto.ChangePasswordPayload	true	"Mật khẩu cũ và mới"
//	@Success	200		{object}	common.SuccessRes{data=SuccessResult}
//	@Failure	400		{object}	common.AppError
//	@Failure	401		{object}	common.AppError
//	@Failure	403		{object}	common.AppError
//	@Failure	500		{object}	common.AppError
//	@Security	BearerAuth
//	@Router		/api/v1/users/me/password [post]
func (h *UserHandler) ChangePassword() gin.HandlerFunc {
	return common.Handle(func(c *gin.Context) (any, error) {
		user, err := middlewares.CurrentAccount(c)
		if err != nil {
			return nil, err
		}

		var p dto.ChangePasswordPayload
		if err := c.ShouldBindJSON(&p); err != nil {
			return nil, common.ErrInvalidRequest(err)
		}

		if err := h.svc.ChangePassword(c.Request.Context(), user, p); err != nil {
			return nil, err
		}
		return SuccessResult{Success: true}, nil
	})
}

// Create godoc
//
//	@Summary		Tạo tài khoản (owner/admin)
//	@Description	Tài khoản dùng được ngay với mật khẩu đặt lúc tạo. owner tạo được mọi role, admin chỉ tạo role member hoặc tester.
//	@Description	Tester chỉ đăng nhập để xem build được mời (/tester/*), không vào được API quản trị; gán qua tester group.
//	@Description	Muốn user vào nhóm quản trị: POST /api/v1/groups/{id}/members/{userId}.
//	@Tags			users
//	@Accept			json
//	@Produce		json
//	@Param			body	body		dto.CreateUserPayload	true	"Email, tên, mật khẩu, role"
//	@Success		201		{object}	common.SuccessRes{data=dto.UserDTO}
//	@Failure		400		{object}	common.AppError
//	@Failure		401		{object}	common.AppError
//	@Failure		403		{object}	common.AppError
//	@Failure		409		{object}	common.AppError
//	@Security		BearerAuth
//	@Router			/api/v1/users/create [post]
func (h *UserHandler) Create() gin.HandlerFunc {
	return common.HandleCreated(func(c *gin.Context) (any, error) {
		actor, err := middlewares.CurrentAccount(c)
		if err != nil {
			return nil, err
		}
		var p dto.CreateUserPayload
		if err := c.ShouldBindJSON(&p); err != nil {
			return nil, common.ErrInvalidRequest(err)
		}
		return h.svc.Create(c.Request.Context(), actor, p)
	})
}

// List godoc
//
//	@Summary	Danh sách tài khoản (không gồm tài khoản đã xoá)
//	@Tags		users
//	@Produce	json
//	@Param		search		query		string	false	"Tìm theo tên hoặc email (không phân biệt hoa thường)"
//	@Param		role		query		string	false	"Lọc theo role"			Enums(owner, admin, member, tester)
//	@Param		sort		query		string	false	"Mặc định createdAt"	Enums(createdAt, name, email, role)
//	@Param		order		query		string	false	"Mặc định desc"			Enums(asc, desc)
//	@Param		page		query		int		false	"Mặc định 1"			minimum(1)
//	@Param		pageSize	query		int		false	"Mặc định 20"			minimum(1)	maximum(100)
//	@Success	200			{object}	common.SuccessRes{data=UserPage}
//	@Failure	400			{object}	common.AppError
//	@Failure	401			{object}	common.AppError
//	@Failure	403			{object}	common.AppError
//	@Security	BearerAuth
//	@Router		/api/v1/users [get]
func (h *UserHandler) List() gin.HandlerFunc {
	return common.Handle(func(c *gin.Context) (any, error) {
		var q dto.ListUsersQuery
		if err := c.ShouldBindQuery(&q); err != nil {
			return nil, common.ErrInvalidRequest(err)
		}
		return h.svc.List(c.Request.Context(), false, q)
	})
}

// UserPage chỉ để Swagger mô tả common.Page[dto.UserDTO].
type UserPage struct {
	Items      []dto.UserDTO `json:"items"`
	Page       int32         `json:"page"`
	PageSize   int32         `json:"pageSize"`
	Total      int64         `json:"total"`
	TotalPages int32         `json:"totalPages"`
	HasMore    bool          `json:"hasMore"`
}

// ListDeleted godoc
//
//	@Summary	Danh sách tài khoản đã xoá (owner/admin)
//	@Tags		users
//	@Produce	json
//	@Param		search		query		string	false	"Tìm theo tên hoặc email (không phân biệt hoa thường)"
//	@Param		role		query		string	false	"Lọc theo role"			Enums(owner, admin, member, tester)
//	@Param		sort		query		string	false	"Mặc định createdAt"	Enums(createdAt, name, email, role)
//	@Param		order		query		string	false	"Mặc định desc"			Enums(asc, desc)
//	@Param		page		query		int		false	"Mặc định 1"			minimum(1)
//	@Param		pageSize	query		int		false	"Mặc định 20"			minimum(1)	maximum(100)
//	@Success	200			{object}	common.SuccessRes{data=UserPage}
//	@Failure	400			{object}	common.AppError
//	@Failure	401			{object}	common.AppError
//	@Failure	403			{object}	common.AppError
//	@Security	BearerAuth
//	@Router		/api/v1/users/deleted [get]
func (h *UserHandler) ListDeleted() gin.HandlerFunc {
	return common.Handle(func(c *gin.Context) (any, error) {
		var q dto.ListUsersQuery
		if err := c.ShouldBindQuery(&q); err != nil {
			return nil, common.ErrInvalidRequest(err)
		}
		return h.svc.List(c.Request.Context(), true, q)
	})
}

// Get godoc
//
//	@Summary	Chi tiết tài khoản
//	@Tags		users
//	@Produce	json
//	@Param		id	path		string	true	"User ID"	format(uuid)
//	@Success	200	{object}	common.SuccessRes{data=dto.UserDTO}
//	@Failure	401	{object}	common.AppError
//	@Failure	404	{object}	common.AppError
//	@Security	BearerAuth
//	@Router		/api/v1/users/{id} [get]
func (h *UserHandler) Get() gin.HandlerFunc {
	return common.Handle(func(c *gin.Context) (any, error) {
		id, err := common.ParamUUID(c, "id", "tài khoản")
		if err != nil {
			return nil, err
		}
		return h.svc.Get(c.Request.Context(), id)
	})
}

// Groups godoc
//
//	@Summary	Các nhóm quản trị mà user là thành viên
//	@Tags		users
//	@Produce	json
//	@Param		id	path		string	true	"User ID"	format(uuid)
//	@Success	200	{object}	common.SuccessRes{data=[]dto.UserGroupDTO}
//	@Failure	401	{object}	common.AppError
//	@Failure	404	{object}	common.AppError
//	@Security	BearerAuth
//	@Router		/api/v1/users/{id}/groups [get]
func (h *UserHandler) Groups() gin.HandlerFunc {
	return common.Handle(func(c *gin.Context) (any, error) {
		id, err := common.ParamUUID(c, "id", "tài khoản")
		if err != nil {
			return nil, err
		}
		return h.svc.Groups(c.Request.Context(), id)
	})
}

// UpdateRole godoc
//
//	@Summary		Đổi role tài khoản (chỉ owner)
//	@Description	Hạ xuống tester → tự gỡ khỏi mọi nhóm quản trị.
//	@Tags			users
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string					true	"User ID"	format(uuid)
//	@Param			body	body		dto.UpdateRolePayload	true	"Role mới"
//	@Success		200		{object}	common.SuccessRes{data=dto.UserDTO}
//	@Failure		400		{object}	common.AppError
//	@Failure		401		{object}	common.AppError
//	@Failure		403		{object}	common.AppError
//	@Failure		404		{object}	common.AppError
//	@Security		BearerAuth
//	@Router			/api/v1/users/{id}/role [patch]
func (h *UserHandler) UpdateRole() gin.HandlerFunc {
	return common.Handle(func(c *gin.Context) (any, error) {
		actor, id, err := actorAndID(c)
		if err != nil {
			return nil, err
		}
		var p dto.UpdateRolePayload
		if err := c.ShouldBindJSON(&p); err != nil {
			return nil, common.ErrInvalidRequest(err)
		}
		return h.svc.UpdateRole(c.Request.Context(), actor, id, p.Role)
	})
}

// UpdateStatus godoc
//
//	@Summary		Khoá / mở khoá tài khoản
//	@Description	owner tác động được mọi người, admin không đụng được owner. Không tự khoá chính mình (400).
//	@Description	Khoá tài khoản chặn đăng nhập và mọi phiên đang mở — khác với bỏ user khỏi một nhóm.
//	@Tags			users
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string					true	"User ID"	format(uuid)
//	@Param			body	body		dto.UpdateStatusPayload	true	"active hoặc disabled"
//	@Success		200		{object}	common.SuccessRes{data=dto.UserDTO}
//	@Failure		400		{object}	common.AppError
//	@Failure		401		{object}	common.AppError
//	@Failure		403		{object}	common.AppError
//	@Failure		404		{object}	common.AppError
//	@Security		BearerAuth
//	@Router			/api/v1/users/{id}/status [patch]
func (h *UserHandler) UpdateStatus() gin.HandlerFunc {
	return common.Handle(func(c *gin.Context) (any, error) {
		actor, id, err := actorAndID(c)
		if err != nil {
			return nil, err
		}
		var p dto.UpdateStatusPayload
		if err := c.ShouldBindJSON(&p); err != nil {
			return nil, common.ErrInvalidRequest(err)
		}
		return h.svc.SetStatus(c.Request.Context(), actor, id, p.Status)
	})
}

// Delete godoc
//
//	@Summary	Xoá mềm tài khoản (khôi phục được)
//	@Tags		users
//	@Param		id	path	string	true	"User ID"	format(uuid)
//	@Success	204
//	@Failure	400	{object}	common.AppError
//	@Failure	401	{object}	common.AppError
//	@Failure	403	{object}	common.AppError
//	@Failure	404	{object}	common.AppError
//	@Security	BearerAuth
//	@Router		/api/v1/users/{id} [delete]
func (h *UserHandler) Delete() gin.HandlerFunc {
	return common.HandleNoContent(func(c *gin.Context) error {
		actor, id, err := actorAndID(c)
		if err != nil {
			return err
		}
		_, err = h.svc.SetStatus(c.Request.Context(), actor, id, sqlc.EntityStatusDeleted)
		return err
	})
}

// Restore godoc
//
//	@Summary	Mở khoá hoặc khôi phục tài khoản đã xoá
//	@Tags		users
//	@Produce	json
//	@Param		id	path		string	true	"User ID"	format(uuid)
//	@Success	200	{object}	common.SuccessRes{data=dto.UserDTO}
//	@Failure	400	{object}	common.AppError
//	@Failure	401	{object}	common.AppError
//	@Failure	403	{object}	common.AppError
//	@Failure	404	{object}	common.AppError
//	@Security	BearerAuth
//	@Router		/api/v1/users/{id}/restore [post]
func (h *UserHandler) Restore() gin.HandlerFunc {
	return common.Handle(func(c *gin.Context) (any, error) {
		actor, id, err := actorAndID(c)
		if err != nil {
			return nil, err
		}
		return h.svc.SetStatus(c.Request.Context(), actor, id, sqlc.EntityStatusActive)
	})
}

func actorAndID(c *gin.Context) (sqlc.User, uuid.UUID, error) {
	actor, err := middlewares.CurrentAccount(c)
	if err != nil {
		return sqlc.User{}, uuid.Nil, err
	}
	id, err := common.ParamUUID(c, "id", "tài khoản")
	if err != nil {
		return sqlc.User{}, uuid.Nil, err
	}
	return actor, id, nil
}
