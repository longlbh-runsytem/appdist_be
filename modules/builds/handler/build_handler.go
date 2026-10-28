// Package handler là tầng duy nhất của module builds biết đến gin.
package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/longlbh/appdist-be/common"
	"github.com/longlbh/appdist-be/db/sqlc"
	"github.com/longlbh/appdist-be/middlewares"
	"github.com/longlbh/appdist-be/modules/builds/dto"
	"github.com/longlbh/appdist-be/modules/builds/services"
)

type BuildHandler struct {
	svc       *services.BuildService
	approvals *services.ApprovalService
	maxBody   int64
}

func NewBuildHandler(svc *services.BuildService, approvals *services.ApprovalService, maxUpload int64) *BuildHandler {
	// Chừa 1 MB cho header multipart và các field chữ.
	return &BuildHandler{svc: svc, approvals: approvals, maxBody: maxUpload + 1<<20}
}

// InitUpload godoc
//
//	@Summary		Bước 1 (web): xin presigned URL để PUT file lên storage
//	@Description	Kiểm tra đuôi (.ipa/.apk) và dung lượng, trả uploadUrl (hạn 1 giờ).
//	@Description	Client PUT file thẳng lên uploadUrl, rồi gọi POST /api/v1/builds với uploadKey + appId.
//	@Tags			builds
//	@Accept			json
//	@Produce		json
//	@Param			body	body		dto.InitUploadPayload	true	"Tên và dung lượng file"
//	@Success		200		{object}	common.SuccessRes{data=dto.InitUploadResponse}
//	@Failure		400		{object}	common.AppError
//	@Failure		401		{object}	common.AppError
//	@Failure		403		{object}	common.AppError
//	@Failure		413		{object}	common.AppError
//	@Security		BearerAuth
//	@Router			/api/v1/builds/uploads [post]
func (h *BuildHandler) InitUpload() gin.HandlerFunc {
	return common.Handle(func(c *gin.Context) (any, error) {
		user, err := middlewares.CurrentAccount(c)
		if err != nil {
			return nil, err
		}

		var p dto.InitUploadPayload
		if err := c.ShouldBindJSON(&p); err != nil {
			return nil, common.ErrInvalidRequest(err)
		}
		return h.svc.InitUpload(c.Request.Context(), user, p)
	})
}

// UploadContent nhận thân file thô: PUT /builds/uploads/content?key=<uploadKey>.
//
//	@Summary		Bước 2 (dự phòng): gửi file qua server
//	@Description	Chỉ dùng khi không PUT thẳng lên uploadUrl được (bucket chưa bật CORS).
//	@Description	Body là nội dung file thô, bắt buộc có Content-Length.
//	@Tags			builds
//	@Accept			octet-stream
//	@Produce		json
//	@Param			key		query		string	true	"uploadKey nhận từ bước 1"
//	@Param			file	body		string	true	"Nội dung file .ipa/.apk"	format(binary)
//	@Success		200		{object}	common.SuccessRes{data=dto.UploadContentResult}
//	@Failure		400		{object}	common.AppError
//	@Failure		401		{object}	common.AppError
//	@Failure		403		{object}	common.AppError
//	@Failure		411		{object}	common.AppError
//	@Failure		413		{object}	common.AppError
//	@Security		BearerAuth
//	@Router			/api/v1/builds/uploads/content [put]
func (h *BuildHandler) UploadContent() gin.HandlerFunc {
	return common.Handle(func(c *gin.Context) (any, error) {
		user, err := middlewares.CurrentAccount(c)
		if err != nil {
			return nil, err
		}

		key := c.Query("key")
		if key == "" {
			return nil, common.ErrInvalidRequest(errors.New("missing key"))
		}

		// MaxBytesReader chặn cứng ở tầng HTTP: body dài hơn Content-Length
		// khai báo thì request bị cắt, không đọc tràn vào bộ nhớ.
		size := c.Request.ContentLength
		body := http.MaxBytesReader(c.Writer, c.Request.Body, max(size, 0))

		if err := h.svc.UploadContent(c.Request.Context(), user, key, body, size); err != nil {
			return nil, err
		}
		return dto.UploadContentResult{Uploaded: true, UploadKey: key}, nil
	})
}

// Create godoc
//
//	@Summary		Bước 3 (web): đọc thông tin từ file đã upload và tạo build cho App
//	@Description	Thông tin app (bundle id, version, build number, icon...) đọc từ nội dung file.
//	@Description	Bundle id trong file phải khớp platform tương ứng của App. File không hợp lệ bị xoá khỏi storage.
//	@Description	App bật requiresApproval → status pending_approval; autoAssignTesterGroups → kế thừa tester group mặc định.
//	@Tags			builds
//	@Accept			json
//	@Produce		json
//	@Param			body	body		dto.CreateBuildPayload	true	"uploadKey, appId, changelog"
//	@Success		201		{object}	common.SuccessRes{data=dto.BuildDTO}
//	@Failure		400		{object}	common.AppError
//	@Failure		401		{object}	common.AppError
//	@Failure		403		{object}	common.AppError
//	@Failure		404		{object}	common.AppError
//	@Failure		409		{object}	common.AppError
//	@Failure		413		{object}	common.AppError
//	@Security		BearerAuth
//	@Router			/api/v1/builds [post]
func (h *BuildHandler) Create() gin.HandlerFunc {
	return common.HandleCreated(func(c *gin.Context) (any, error) {
		user, err := middlewares.CurrentAccount(c)
		if err != nil {
			return nil, err
		}

		var p dto.CreateBuildPayload
		if err := c.ShouldBindJSON(&p); err != nil {
			return nil, common.ErrInvalidRequest(err)
		}
		return h.svc.Create(c.Request.Context(), user, p)
	})
}

// Upload godoc
//
//	@Summary		Upload build bằng multipart (CI hoặc web)
//	@Description	Xác thực bằng header X-Api-Key (CI — appId lấy từ key) hoặc Bearer (web — bắt buộc appId).
//	@Description	version/buildNumber client gửi bị bỏ qua: luôn đọc từ file. platform nếu gửi phải khớp file.
//	@Description	File stream thẳng lên storage, không buffer trên server.
//	@Tags			builds
//	@Accept			multipart/form-data
//	@Produce		json
//	@Param			file		formData	file	true	"File .ipa hoặc .apk"
//	@Param			appId		formData	string	false	"Bắt buộc khi dùng Bearer"					format(uuid)
//	@Param			platform	formData	string	false	"ios | android — kiểm tra khớp với file"	Enums(ios, android)
//	@Param			changelog	formData	string	false	"Ghi chú bản build"
//	@Success		201			{object}	common.SuccessRes{data=dto.BuildDTO}
//	@Failure		400			{object}	common.AppError
//	@Failure		401			{object}	common.AppError
//	@Failure		403			{object}	common.AppError
//	@Failure		404			{object}	common.AppError
//	@Failure		413			{object}	common.AppError
//	@Security		BearerAuth
//	@Security		ApiKeyAuth
//	@Router			/api/v1/builds/upload [post]
func (h *BuildHandler) Upload() gin.HandlerFunc {
	return common.HandleCreated(func(c *gin.Context) (any, error) {
		user, err := middlewares.CurrentAccount(c)
		if err != nil {
			return nil, err
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, h.maxBody)
		mr, err := c.Request.MultipartReader()
		if err != nil {
			return nil, common.ErrBadRequest("body phải là multipart/form-data", "ErrNotMultipart")
		}
		return h.svc.Upload(c.Request.Context(), user, middlewares.CurrentAPIKeyApp(c), mr)
	})
}

// List godoc
//
//	@Summary		Danh sách build của các App trong scope (lọc, sắp xếp, phân trang)
//	@Description	Không gồm quick share. Mặc định mới upload nhất trước.
//	@Tags			builds
//	@Produce		json
//	@Param			appId		query		string	false	"Lọc theo App"	format(uuid)
//	@Param			search		query		string	false	"Tìm trong version, buildNumber, changelog (không phân biệt hoa thường)"
//	@Param			platform	query		string	false	"Lọc theo platform"								Enums(ios, android)
//	@Param			status		query		string	false	"Lọc theo trạng thái"							Enums(processing, ready, pending_approval, rejected, archived)
//	@Param			from		query		string	false	"uploadedAt >= (RFC3339)"						format(date-time)
//	@Param			to			query		string	false	"uploadedAt <= (RFC3339)"						format(date-time)
//	@Param			sort		query		string	false	"Mặc định uploadedAt; version so kiểu semver"	Enums(uploadedAt, version, buildNumber)
//	@Param			order		query		string	false	"Mặc định desc"									Enums(asc, desc)
//	@Param			page		query		int		false	"Mặc định 1"									minimum(1)
//	@Param			pageSize	query		int		false	"Mặc định 20"									minimum(1)	maximum(100)
//	@Success		200			{object}	common.SuccessRes{data=BuildPage}
//	@Failure		400			{object}	common.AppError
//	@Failure		401			{object}	common.AppError
//	@Failure		404			{object}	common.AppError
//	@Security		BearerAuth
//	@Router			/api/v1/builds [get]
func (h *BuildHandler) List() gin.HandlerFunc {
	return common.Handle(func(c *gin.Context) (any, error) {
		user, err := middlewares.CurrentAccount(c)
		if err != nil {
			return nil, err
		}

		var q dto.ListBuildsQuery
		if err := c.ShouldBindQuery(&q); err != nil {
			return nil, common.ErrInvalidRequest(err)
		}
		return h.svc.List(c.Request.Context(), user, q)
	})
}

// ListByApp godoc
//
//	@Summary		Tất cả build của một App, mới nhất trước
//	@Description	Gồm mọi trạng thái (ready, pending_approval, rejected, archived) trừ khi lọc bằng status.
//	@Description	App ngoài scope hoặc không tồn tại → 404.
//	@Tags			builds
//	@Produce		json
//	@Param			id			path		string	true	"App ID"	format(uuid)
//	@Param			search		query		string	false	"Tìm trong version, buildNumber, changelog (không phân biệt hoa thường)"
//	@Param			platform	query		string	false	"Lọc theo platform"								Enums(ios, android)
//	@Param			status		query		string	false	"Lọc theo trạng thái"							Enums(processing, ready, pending_approval, rejected, archived)
//	@Param			from		query		string	false	"uploadedAt >= (RFC3339)"						format(date-time)
//	@Param			to			query		string	false	"uploadedAt <= (RFC3339)"						format(date-time)
//	@Param			sort		query		string	false	"Mặc định uploadedAt; version so kiểu semver"	Enums(uploadedAt, version, buildNumber)
//	@Param			order		query		string	false	"Mặc định desc"									Enums(asc, desc)
//	@Param			page		query		int		false	"Mặc định 1"									minimum(1)
//	@Param			pageSize	query		int		false	"Mặc định 20"									minimum(1)	maximum(100)
//	@Success		200			{object}	common.SuccessRes{data=BuildPage}
//	@Failure		400			{object}	common.AppError
//	@Failure		401			{object}	common.AppError
//	@Failure		404			{object}	common.AppError
//	@Security		BearerAuth
//	@Router			/api/v1/apps/{id}/builds [get]
func (h *BuildHandler) ListByApp() gin.HandlerFunc {
	return common.Handle(func(c *gin.Context) (any, error) {
		user, err := middlewares.CurrentAccount(c)
		if err != nil {
			return nil, err
		}
		appID, err := common.ParamUUID(c, "id", "App")
		if err != nil {
			return nil, err
		}

		var q dto.ListBuildsQuery
		if err := c.ShouldBindQuery(&q); err != nil {
			return nil, common.ErrInvalidRequest(err)
		}
		q.AppID = appID.String() // path thắng query
		return h.svc.List(c.Request.Context(), user, q)
	})
}

// BuildPage chỉ để Swagger mô tả common.Page[dto.BuildDTO].
type BuildPage struct {
	Items      []dto.BuildDTO `json:"items"`
	Page       int32          `json:"page"`
	PageSize   int32          `json:"pageSize"`
	Total      int64          `json:"total"`
	TotalPages int32          `json:"totalPages"`
	HasMore    bool           `json:"hasMore"`
}

// Archive godoc
//
//	@Summary	Lưu trữ build — link tải ngừng hoạt động (owner/admin)
//	@Tags		builds
//	@Produce	json
//	@Param		id	path		string	true	"Build ID"
//	@Success	200	{object}	common.SuccessRes{data=dto.BuildDTO}
//	@Failure	401	{object}	common.AppError
//	@Failure	403	{object}	common.AppError
//	@Failure	404	{object}	common.AppError
//	@Security	BearerAuth
//	@Router		/api/v1/builds/{id}/archive [patch]
func (h *BuildHandler) Archive() gin.HandlerFunc {
	return common.Handle(func(c *gin.Context) (any, error) {
		user, err := middlewares.CurrentAccount(c)
		if err != nil {
			return nil, err
		}
		id, err := buildID(c)
		if err != nil {
			return nil, err
		}
		return h.svc.Archive(c.Request.Context(), user, id)
	})
}

// AddTesterGroup godoc
//
//	@Summary	Gán 1 tester group cho build (mọi role nội bộ, trong scope)
//	@Tags		builds
//	@Produce	json
//	@Param		id				path		string	true	"Build ID"
//	@Param		testerGroupId	path		string	true	"Tester group ID"	format(uuid)
//	@Success	200				{object}	common.SuccessRes{data=dto.BuildDTO}
//	@Failure	401				{object}	common.AppError
//	@Failure	404				{object}	common.AppError
//	@Security	BearerAuth
//	@Router		/api/v1/builds/{id}/tester-groups/{testerGroupId} [post]
func (h *BuildHandler) AddTesterGroup() gin.HandlerFunc {
	return h.testerGroupAction(h.svc.AddTesterGroup)
}

// RemoveTesterGroup godoc
//
//	@Summary	Gỡ 1 tester group khỏi build (mọi role nội bộ, trong scope)
//	@Tags		builds
//	@Produce	json
//	@Param		id				path		string	true	"Build ID"
//	@Param		testerGroupId	path		string	true	"Tester group ID"	format(uuid)
//	@Success	200				{object}	common.SuccessRes{data=dto.BuildDTO}
//	@Failure	401				{object}	common.AppError
//	@Failure	404				{object}	common.AppError
//	@Security	BearerAuth
//	@Router		/api/v1/builds/{id}/tester-groups/{testerGroupId} [delete]
func (h *BuildHandler) RemoveTesterGroup() gin.HandlerFunc {
	return h.testerGroupAction(h.svc.RemoveTesterGroup)
}

func (h *BuildHandler) testerGroupAction(fn func(ctx context.Context, user sqlc.User, id int64, tgID uuid.UUID) (*dto.BuildDTO, error)) gin.HandlerFunc {
	return common.Handle(func(c *gin.Context) (any, error) {
		user, err := middlewares.CurrentAccount(c)
		if err != nil {
			return nil, err
		}
		id, err := buildID(c)
		if err != nil {
			return nil, err
		}
		tgID, err := common.ParamUUID(c, "testerGroupId", "tester group")
		if err != nil {
			return nil, err
		}
		return fn(c.Request.Context(), user, id, tgID)
	})
}

// ListApprovals godoc
//
//	@Summary	Yêu cầu duyệt build đang chờ (owner/admin, trong scope)
//	@Tags		approvals
//	@Produce	json
//	@Success	200	{object}	common.SuccessRes{data=[]dto.ApprovalDTO}
//	@Failure	401	{object}	common.AppError
//	@Failure	403	{object}	common.AppError
//	@Security	BearerAuth
//	@Router		/api/v1/approvals [get]
func (h *BuildHandler) ListApprovals() gin.HandlerFunc {
	return common.Handle(func(c *gin.Context) (any, error) {
		user, err := middlewares.CurrentAccount(c)
		if err != nil {
			return nil, err
		}
		return h.approvals.ListPending(c.Request.Context(), user)
	})
}

// Review godoc
//
//	@Summary		Duyệt hoặc từ chối build (owner/admin, trong scope)
//	@Description	approve → build ready; reject → build rejected. Đã duyệt rồi → 409.
//	@Tags			approvals
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string				true	"Approval request ID"	format(uuid)
//	@Param			body	body		dto.ReviewPayload	true	"Quyết định và ghi chú"
//	@Success		200		{object}	common.SuccessRes{data=dto.ReviewResultDTO}
//	@Failure		400		{object}	common.AppError
//	@Failure		401		{object}	common.AppError
//	@Failure		403		{object}	common.AppError
//	@Failure		404		{object}	common.AppError
//	@Failure		409		{object}	common.AppError
//	@Security		BearerAuth
//	@Router			/api/v1/approvals/{id}/review [post]
func (h *BuildHandler) Review() gin.HandlerFunc {
	return common.Handle(func(c *gin.Context) (any, error) {
		user, err := middlewares.CurrentAccount(c)
		if err != nil {
			return nil, err
		}
		id, err := common.ParamUUID(c, "id", "yêu cầu duyệt")
		if err != nil {
			return nil, err
		}
		var p dto.ReviewPayload
		if err := c.ShouldBindJSON(&p); err != nil {
			return nil, common.ErrInvalidRequest(err)
		}
		return h.approvals.Review(c.Request.Context(), user, id, p)
	})
}

func buildID(c *gin.Context) (int64, error) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, common.ErrNotFound("build")
	}
	return id, nil
}
