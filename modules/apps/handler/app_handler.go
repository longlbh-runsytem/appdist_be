// Package handler là tầng duy nhất của module apps biết đến gin.
package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/longlbh/appdist-be/common"
	"github.com/longlbh/appdist-be/db/sqlc"
	"github.com/longlbh/appdist-be/middlewares"
	"github.com/longlbh/appdist-be/modules/apps/dto"
	"github.com/longlbh/appdist-be/modules/apps/services"
)

type AppHandler struct {
	svc *services.AppService
}

func NewAppHandler(svc *services.AppService) *AppHandler {
	return &AppHandler{svc: svc}
}

// Create godoc
//
//	@Summary		Tạo App (owner/admin)
//	@Description	Cần ít nhất một platform và một nhóm quản trị. Admin chỉ gán được nhóm mình là thành viên.
//	@Tags			apps
//	@Accept			json
//	@Produce		json
//	@Param			body	body		dto.CreateAppPayload	true	"Tên, platforms, nhóm quản trị"
//	@Success		201		{object}	common.SuccessRes{data=dto.AppDTO}
//	@Failure		400		{object}	common.AppError
//	@Failure		401		{object}	common.AppError
//	@Failure		403		{object}	common.AppError
//	@Failure		409		{object}	common.AppError
//	@Security		BearerAuth
//	@Router			/api/v1/apps [post]
func (h *AppHandler) Create() gin.HandlerFunc {
	return common.HandleCreated(func(c *gin.Context) (any, error) {
		user, err := middlewares.CurrentAccount(c)
		if err != nil {
			return nil, err
		}
		var p dto.CreateAppPayload
		if err := c.ShouldBindJSON(&p); err != nil {
			return nil, common.ErrInvalidRequest(err)
		}
		return h.svc.Create(c.Request.Context(), user, p)
	})
}

// List godoc
//
//	@Summary		Danh sách App (phân trang)
//	@Description	owner thấy tất cả; role khác chỉ thấy App thuộc nhóm quản trị của mình.
//	@Tags			apps
//	@Produce		json
//	@Param			search		query		string	false	"Lọc theo tên"
//	@Param			platform	query		string	false	"ios | android"				Enums(ios, android)
//	@Param			from		query		string	false	"createdAt >= (RFC3339)"	format(date-time)
//	@Param			to			query		string	false	"createdAt <= (RFC3339)"	format(date-time)
//	@Param			page		query		int		false	"Mặc định 1"				minimum(1)
//	@Param			pageSize	query		int		false	"Mặc định 20"				minimum(1)	maximum(100)
//	@Success		200			{object}	common.SuccessRes{data=AppPage}
//	@Failure		400			{object}	common.AppError
//	@Failure		401			{object}	common.AppError
//	@Security		BearerAuth
//	@Router			/api/v1/apps [get]
func (h *AppHandler) List() gin.HandlerFunc {
	return common.Handle(func(c *gin.Context) (any, error) {
		user, err := middlewares.CurrentAccount(c)
		if err != nil {
			return nil, err
		}
		var q dto.ListAppsQuery
		if err := c.ShouldBindQuery(&q); err != nil {
			return nil, common.ErrInvalidRequest(err)
		}
		return h.svc.List(c.Request.Context(), user, q)
	})
}

// AppPage chỉ để Swagger mô tả common.Page[dto.AppDTO].
type AppPage struct {
	Items      []dto.AppDTO `json:"items"`
	Page       int32        `json:"page"`
	PageSize   int32        `json:"pageSize"`
	Total      int64        `json:"total"`
	TotalPages int32        `json:"totalPages"`
	HasMore    bool         `json:"hasMore"`
}

// Get godoc
//
//	@Summary	Chi tiết App (ngoài scope → 404)
//	@Tags		apps
//	@Produce	json
//	@Param		id	path		string	true	"App ID"	format(uuid)
//	@Success	200	{object}	common.SuccessRes{data=dto.AppDTO}
//	@Failure	401	{object}	common.AppError
//	@Failure	404	{object}	common.AppError
//	@Security	BearerAuth
//	@Router		/api/v1/apps/{id} [get]
func (h *AppHandler) Get() gin.HandlerFunc {
	return h.withApp(func(c *gin.Context, user sqlc.User, id uuid.UUID) (any, error) {
		return h.svc.Get(c.Request.Context(), user, id)
	})
}

// GetPublic godoc
//
//	@Summary		Trang App công khai (không cần đăng nhập)
//	@Description	Chỉ trả khi App isPublic=true và chưa hết hạn; App private, hết hạn hoặc không tồn tại → 404.
//	@Description	builds: cùng query params và dạng response với GET /tester/my-apps/{appId}/builds —
//	@Description	chỉ bản ready còn cài được (còn hạn, chưa hết downloadLimit). counts không phụ thuộc bộ lọc platform.
//	@Description	Mở từng build bằng GET /api/v1/releases/{shareToken}.
//	@Tags			apps
//	@Produce		json
//	@Param			id			path		string	true	"App ID"				format(uuid)
//	@Param			platform	query		string	false	"Lọc theo loại build"	Enums(ios, android)
//	@Param			sort		query		string	false	"Mặc định buildNumber"	Enums(buildNumber, uploadedAt)
//	@Param			order		query		string	false	"Mặc định desc"			Enums(asc, desc)
//	@Param			page		query		int		false	"Mặc định 1"			minimum(1)
//	@Param			pageSize	query		int		false	"Mặc định 20"			minimum(1)	maximum(100)
//	@Success		200			{object}	common.SuccessRes{data=dto.PublicAppDTO}
//	@Failure		400			{object}	common.AppError
//	@Failure		404			{object}	common.AppError
//	@Router			/api/v1/apps/{id}/public [get]
func (h *AppHandler) GetPublic() gin.HandlerFunc {
	return common.Handle(func(c *gin.Context) (any, error) {
		id, err := common.ParamUUID(c, "id", "App")
		if err != nil {
			return nil, err
		}
		var q dto.ListAppBuildsQuery
		if err := c.ShouldBindQuery(&q); err != nil {
			return nil, common.ErrInvalidRequest(err)
		}
		return h.svc.GetPublic(c.Request.Context(), id, q)
	})
}

// UpdateInfo godoc
//
//	@Summary	Sửa tên và platforms (owner/admin trong scope)
//	@Tags		apps
//	@Accept		json
//	@Produce	json
//	@Param		id		path		string						true	"App ID"	format(uuid)
//	@Param		body	body		dto.UpdateAppInfoPayload	true	"Tên, platforms"
//	@Success	200		{object}	common.SuccessRes{data=dto.AppDTO}
//	@Failure	400		{object}	common.AppError
//	@Failure	401		{object}	common.AppError
//	@Failure	403		{object}	common.AppError
//	@Failure	404		{object}	common.AppError
//	@Failure	409		{object}	common.AppError
//	@Security	BearerAuth
//	@Router		/api/v1/apps/{id}/info [patch]
func (h *AppHandler) UpdateInfo() gin.HandlerFunc {
	return h.withApp(func(c *gin.Context, user sqlc.User, id uuid.UUID) (any, error) {
		var p dto.UpdateAppInfoPayload
		if err := c.ShouldBindJSON(&p); err != nil {
			return nil, common.ErrInvalidRequest(err)
		}
		return h.svc.UpdateInfo(c.Request.Context(), user, id, p)
	})
}

// AddManagingGroup godoc
//
//	@Summary	Gán thêm 1 nhóm quản trị (owner/admin trong scope)
//	@Tags		apps
//	@Produce	json
//	@Param		id		path		string	true	"App ID"	format(uuid)
//	@Param		groupId	path		string	true	"Group ID"	format(uuid)
//	@Success	200		{object}	common.SuccessRes{data=dto.AppDTO}
//	@Failure	400		{object}	common.AppError
//	@Failure	401		{object}	common.AppError
//	@Failure	403		{object}	common.AppError
//	@Failure	404		{object}	common.AppError
//	@Security	BearerAuth
//	@Router		/api/v1/apps/{id}/managing-groups/{groupId} [post]
func (h *AppHandler) AddManagingGroup() gin.HandlerFunc {
	return h.withApp(func(c *gin.Context, user sqlc.User, id uuid.UUID) (any, error) {
		gid, err := common.ParamUUID(c, "groupId", "nhóm")
		if err != nil {
			return nil, err
		}
		return h.svc.AddManagingGroup(c.Request.Context(), user, id, gid)
	})
}

// RemoveManagingGroup godoc
//
//	@Summary	Gỡ 1 nhóm quản trị (không gỡ được nhóm cuối cùng)
//	@Tags		apps
//	@Produce	json
//	@Param		id		path		string	true	"App ID"	format(uuid)
//	@Param		groupId	path		string	true	"Group ID"	format(uuid)
//	@Success	200		{object}	common.SuccessRes{data=dto.AppDTO}
//	@Failure	400		{object}	common.AppError
//	@Failure	401		{object}	common.AppError
//	@Failure	403		{object}	common.AppError
//	@Failure	404		{object}	common.AppError
//	@Security	BearerAuth
//	@Router		/api/v1/apps/{id}/managing-groups/{groupId} [delete]
func (h *AppHandler) RemoveManagingGroup() gin.HandlerFunc {
	return h.withApp(func(c *gin.Context, user sqlc.User, id uuid.UUID) (any, error) {
		gid, err := common.ParamUUID(c, "groupId", "nhóm")
		if err != nil {
			return nil, err
		}
		return h.svc.RemoveManagingGroup(c.Request.Context(), user, id, gid)
	})
}

// UpdateVisibility godoc
//
//	@Summary		Đặt chế độ public, hạn link, giới hạn lượt tải (owner/admin trong scope)
//	@Description	isPublic=false: link tải chỉ mở được qua installUrl do GET /releases/{token} cấp cho người có quyền.
//	@Description	expiresAt/downloadLimit null = bỏ giới hạn. downloadLimit tính cho MỖI build.
//	@Tags			apps
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string					true	"App ID"	format(uuid)
//	@Param			body	body		dto.VisibilityPayload	true	"Cấu hình hiển thị"
//	@Success		200		{object}	common.SuccessRes{data=dto.AppDTO}
//	@Failure		400		{object}	common.AppError
//	@Failure		401		{object}	common.AppError
//	@Failure		403		{object}	common.AppError
//	@Failure		404		{object}	common.AppError
//	@Security		BearerAuth
//	@Router			/api/v1/apps/{id}/visibility [patch]
func (h *AppHandler) UpdateVisibility() gin.HandlerFunc {
	return h.withApp(func(c *gin.Context, user sqlc.User, id uuid.UUID) (any, error) {
		var p dto.VisibilityPayload
		if err := c.ShouldBindJSON(&p); err != nil {
			return nil, common.ErrInvalidRequest(err)
		}
		return h.svc.UpdateVisibility(c.Request.Context(), user, id, p)
	})
}

// UpdateApproval godoc
//
//	@Summary		Bật/tắt yêu cầu duyệt build (owner/admin trong scope)
//	@Description	Bật → build mới có status pending_approval và một yêu cầu duyệt ở GET /approvals.
//	@Tags			apps
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string				true	"App ID"	format(uuid)
//	@Param			body	body		dto.ApprovalPayload	true	"requiresApproval"
//	@Success		200		{object}	common.SuccessRes{data=dto.AppDTO}
//	@Failure		400		{object}	common.AppError
//	@Failure		401		{object}	common.AppError
//	@Failure		403		{object}	common.AppError
//	@Failure		404		{object}	common.AppError
//	@Security		BearerAuth
//	@Router			/api/v1/apps/{id}/approval [patch]
func (h *AppHandler) UpdateApproval() gin.HandlerFunc {
	return h.withApp(func(c *gin.Context, user sqlc.User, id uuid.UUID) (any, error) {
		var p dto.ApprovalPayload
		if err := c.ShouldBindJSON(&p); err != nil {
			return nil, common.ErrInvalidRequest(err)
		}
		return h.svc.UpdateApproval(c.Request.Context(), user, id, p)
	})
}

// UpdateAutoAssign godoc
//
//	@Summary	Bật/tắt tự gán tester group mặc định cho build mới (owner/admin trong scope)
//	@Tags		apps
//	@Accept		json
//	@Produce	json
//	@Param		id		path		string					true	"App ID"	format(uuid)
//	@Param		body	body		dto.AutoAssignPayload	true	"autoAssignTesterGroups"
//	@Success	200		{object}	common.SuccessRes{data=dto.AppDTO}
//	@Failure	400		{object}	common.AppError
//	@Failure	401		{object}	common.AppError
//	@Failure	403		{object}	common.AppError
//	@Failure	404		{object}	common.AppError
//	@Security	BearerAuth
//	@Router		/api/v1/apps/{id}/auto-assign-tester-groups [patch]
func (h *AppHandler) UpdateAutoAssign() gin.HandlerFunc {
	return h.withApp(func(c *gin.Context, user sqlc.User, id uuid.UUID) (any, error) {
		var p dto.AutoAssignPayload
		if err := c.ShouldBindJSON(&p); err != nil {
			return nil, common.ErrInvalidRequest(err)
		}
		return h.svc.UpdateAutoAssign(c.Request.Context(), user, id, p)
	})
}

// AddDefaultTesterGroup godoc
//
//	@Summary	Thêm 1 tester group vào danh sách mặc định (owner/admin trong scope)
//	@Tags		apps
//	@Produce	json
//	@Param		id				path		string	true	"App ID"			format(uuid)
//	@Param		testerGroupId	path		string	true	"Tester group ID"	format(uuid)
//	@Success	200				{object}	common.SuccessRes{data=dto.AppDTO}
//	@Failure	401				{object}	common.AppError
//	@Failure	403				{object}	common.AppError
//	@Failure	404				{object}	common.AppError
//	@Security	BearerAuth
//	@Router		/api/v1/apps/{id}/default-tester-groups/{testerGroupId} [post]
func (h *AppHandler) AddDefaultTesterGroup() gin.HandlerFunc {
	return h.withApp(func(c *gin.Context, user sqlc.User, id uuid.UUID) (any, error) {
		tgID, err := common.ParamUUID(c, "testerGroupId", "tester group")
		if err != nil {
			return nil, err
		}
		return h.svc.AddDefaultTesterGroup(c.Request.Context(), user, id, tgID)
	})
}

// RemoveDefaultTesterGroup godoc
//
//	@Summary	Bỏ 1 tester group khỏi danh sách mặc định (owner/admin trong scope)
//	@Tags		apps
//	@Produce	json
//	@Param		id				path		string	true	"App ID"			format(uuid)
//	@Param		testerGroupId	path		string	true	"Tester group ID"	format(uuid)
//	@Success	200				{object}	common.SuccessRes{data=dto.AppDTO}
//	@Failure	401				{object}	common.AppError
//	@Failure	403				{object}	common.AppError
//	@Failure	404				{object}	common.AppError
//	@Security	BearerAuth
//	@Router		/api/v1/apps/{id}/default-tester-groups/{testerGroupId} [delete]
func (h *AppHandler) RemoveDefaultTesterGroup() gin.HandlerFunc {
	return h.withApp(func(c *gin.Context, user sqlc.User, id uuid.UUID) (any, error) {
		tgID, err := common.ParamUUID(c, "testerGroupId", "tester group")
		if err != nil {
			return nil, err
		}
		return h.svc.RemoveDefaultTesterGroup(c.Request.Context(), user, id, tgID)
	})
}

// CreateAPIKey godoc
//
//	@Summary		Tạo API key cho CI/CD (owner/admin trong scope)
//	@Description	token (plaintext) chỉ trả về đúng 1 lần này. Một key dùng chung cho mọi platform của App.
//	@Tags			api-keys
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string					true	"App ID"	format(uuid)
//	@Param			body	body		dto.CreateAPIKeyPayload	true	"Nhãn để nhận diện"
//	@Success		201		{object}	common.SuccessRes{data=dto.APIKeyDTO}
//	@Failure		400		{object}	common.AppError
//	@Failure		401		{object}	common.AppError
//	@Failure		403		{object}	common.AppError
//	@Failure		404		{object}	common.AppError
//	@Security		BearerAuth
//	@Router			/api/v1/apps/{id}/tokens [post]
func (h *AppHandler) CreateAPIKey() gin.HandlerFunc {
	return common.HandleCreated(func(c *gin.Context) (any, error) {
		user, id, err := userAndApp(c)
		if err != nil {
			return nil, err
		}
		var p dto.CreateAPIKeyPayload
		if err := c.ShouldBindJSON(&p); err != nil {
			return nil, common.ErrInvalidRequest(err)
		}
		return h.svc.CreateAPIKey(c.Request.Context(), user, id, p)
	})
}

// ListAPIKeys godoc
//
//	@Summary	Danh sách API key còn hiệu lực (không có plaintext)
//	@Tags		api-keys
//	@Produce	json
//	@Param		id	path		string	true	"App ID"	format(uuid)
//	@Success	200	{object}	common.SuccessRes{data=[]dto.APIKeyDTO}
//	@Failure	401	{object}	common.AppError
//	@Failure	403	{object}	common.AppError
//	@Failure	404	{object}	common.AppError
//	@Security	BearerAuth
//	@Router		/api/v1/apps/{id}/tokens [get]
func (h *AppHandler) ListAPIKeys() gin.HandlerFunc {
	return h.withApp(func(c *gin.Context, user sqlc.User, id uuid.UUID) (any, error) {
		return h.svc.ListAPIKeys(c.Request.Context(), user, id)
	})
}

// RevokeAPIKey godoc
//
//	@Summary	Thu hồi API key ngay lập tức
//	@Tags		api-keys
//	@Param		id		path	string	true	"App ID"		format(uuid)
//	@Param		tokenId	path	string	true	"API key ID"	format(uuid)
//	@Success	204
//	@Failure	401	{object}	common.AppError
//	@Failure	403	{object}	common.AppError
//	@Failure	404	{object}	common.AppError
//	@Security	BearerAuth
//	@Router		/api/v1/apps/{id}/tokens/{tokenId} [delete]
func (h *AppHandler) RevokeAPIKey() gin.HandlerFunc {
	return common.HandleNoContent(func(c *gin.Context) error {
		user, id, err := userAndApp(c)
		if err != nil {
			return err
		}
		keyID, err := common.ParamUUID(c, "tokenId", "API key")
		if err != nil {
			return err
		}
		return h.svc.RevokeAPIKey(c.Request.Context(), user, id, keyID)
	})
}

func (h *AppHandler) withApp(fn func(c *gin.Context, user sqlc.User, id uuid.UUID) (any, error)) gin.HandlerFunc {
	return common.Handle(func(c *gin.Context) (any, error) {
		user, id, err := userAndApp(c)
		if err != nil {
			return nil, err
		}
		return fn(c, user, id)
	})
}

func userAndApp(c *gin.Context) (sqlc.User, uuid.UUID, error) {
	user, err := middlewares.CurrentAccount(c)
	if err != nil {
		return sqlc.User{}, uuid.Nil, err
	}
	id, err := common.ParamUUID(c, "id", "App")
	if err != nil {
		return sqlc.User{}, uuid.Nil, err
	}
	return user, id, nil
}
