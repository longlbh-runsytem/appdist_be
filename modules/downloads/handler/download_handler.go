// Package handler phục vụ các route PUBLIC dưới /d/:token (plist, ảnh,
// redirect — không dùng common.Handle) và API JSON /releases, /quick-share.
package handler

import (
	"context"
	"errors"
	"mime/multipart"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	"github.com/longlbh/appdist-be/common"
	"github.com/longlbh/appdist-be/components/ticket"
	"github.com/longlbh/appdist-be/db/sqlc"
	"github.com/longlbh/appdist-be/middlewares"
	"github.com/longlbh/appdist-be/modules/downloads/services"
)

// QuickSharer tạo build quick share từ multipart — hiện thực bởi
// builds/services.BuildService, trả build token. uploader nil = ẩn danh.
type QuickSharer interface {
	QuickShare(ctx context.Context, uploader *sqlc.User, mr *multipart.Reader) (string, error)
}

type DownloadHandler struct {
	svc     *services.DownloadService
	sharer  QuickSharer
	maxBody int64
}

func NewDownloadHandler(svc *services.DownloadService, sharer QuickSharer, maxUpload int64) *DownloadHandler {
	return &DownloadHandler{svc: svc, sharer: sharer, maxBody: maxUpload + 1<<20}
}

var (
	errReleaseNotFound = common.ErrNotFound("release")
	errReleaseGone     = common.NewFullErrorResponse(
		http.StatusGone,
		services.ErrUnavailable,
		"link đã hết hạn, bị thu hồi hoặc chưa được duyệt",
		"release unavailable",
		"ErrReleaseUnavailable",
	)
)

// Release godoc
//
//	@Summary		Dữ liệu trang tải (public)
//	@Description	App public: ai cũng xem được. App không public: cần Bearer của owner, member trong scope,
//	@Description	hoặc tester được gán build — installUrl trả về kèm vé hạn 24 giờ. Không đủ quyền → 404.
//	@Description	Link chia sẻ của App không public kèm vé ?t= — trang FE chuyển tiếp vé đó vào đây thì không cần Bearer.
//	@Description	requiresPassword luôn false (chưa làm tính năng mật khẩu).
//	@Tags			releases
//	@Produce		json
//	@Param			token	path		string	true	"Share token"
//	@Param			t		query		string	false	"Vé trên link chia sẻ (App không public)"
//	@Success		200		{object}	common.SuccessRes{data=dto.ReleaseDTO}
//	@Failure		401		{object}	common.AppError
//	@Failure		404		{object}	common.AppError
//	@Failure		410		{object}	common.AppError
//	@Router			/api/v1/releases/{token} [get]
func (h *DownloadHandler) Release() gin.HandlerFunc {
	return common.Handle(func(c *gin.Context) (any, error) {
		viewer := middlewares.OptionalCurrentAccount(c)
		res, err := h.svc.Release(c.Request.Context(), c.Param("token"), c.Query(ticket.QueryParam), viewer)
		if err != nil {
			return nil, releaseError(err)
		}
		return res, nil
	})
}

// QuickShare godoc
//
//	@Summary		Chia sẻ nhanh một file .ipa/.apk (không gắn App, không cần đăng nhập)
//	@Description	Không cần đăng nhập. Có gửi Bearer hợp lệ thì ghi lại người upload, không thì lưu ẩn danh.
//	@Description	Thông tin app và icon đọc từ file phía server, không nhận từ client.
//	@Description	Link tự hết hạn sau 7 ngày, file bị dọn khỏi storage sau đó.
//	@Tags			releases
//	@Accept			multipart/form-data
//	@Produce		json
//	@Param			file		formData	file	true	"File .ipa hoặc .apk"
//	@Param			changelog	formData	string	false	"Ghi chú"
//	@Success		201			{object}	common.SuccessRes{data=dto.ReleaseDTO}
//	@Failure		400			{object}	common.AppError
//	@Failure		413			{object}	common.AppError
//	@Router			/api/v1/quick-share [post]
func (h *DownloadHandler) QuickShare() gin.HandlerFunc {
	return common.HandleCreated(func(c *gin.Context) (any, error) {
		uploader := middlewares.OptionalCurrentAccount(c)
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, h.maxBody)
		mr, err := c.Request.MultipartReader()
		if err != nil {
			return nil, common.ErrBadRequest("body phải là multipart/form-data", "ErrNotMultipart")
		}

		token, err := h.sharer.QuickShare(c.Request.Context(), uploader, mr)
		if err != nil {
			return nil, err
		}
		// Quick share luôn public nên viewer không ảnh hưởng kết quả.
		res, err := h.svc.Release(c.Request.Context(), token, "", uploader)
		if err != nil {
			return nil, releaseError(err)
		}
		return res, nil
	})
}

func releaseError(err error) error {
	switch {
	case errors.Is(err, services.ErrNotFound):
		return errReleaseNotFound
	case errors.Is(err, services.ErrUnavailable):
		return errReleaseGone
	default:
		return common.ErrInternal(err)
	}
}

// Manifest trả manifest.plist cho iOS.
//
//	@Summary	manifest.plist cho itms-services (iOS)
//	@Tags		downloads
//	@Produce	xml
//	@Param		token	path		string	true	"Token trên link tải"
//	@Param		t		query		string	false	"Vé tải — bắt buộc nếu App không public (lấy từ installUrl của GET /releases)"
//	@Success	200		{string}	string	"plist XML"
//	@Failure	404		{string}	string	"not found"
//	@Failure	410		{string}	string	"link unavailable"
//	@Router		/d/{token}/manifest.plist [get]
func (h *DownloadHandler) Manifest() gin.HandlerFunc {
	return func(c *gin.Context) {
		body, err := h.svc.Manifest(c.Request.Context(), c.Param("token"), c.Query(ticket.QueryParam))
		if err != nil {
			h.plainError(c, err)
			return
		}
		c.Header("Cache-Control", "no-store")
		c.Data(http.StatusOK, "text/xml; charset=utf-8", body)
	}
}

// File ghi nhận lượt tải rồi redirect sang link tạm trên storage — file
// không đi qua server.
//
// Chỉ đếm request GET tải từ ĐẦU file: trình tải có thể gửi thêm HEAD, hoặc
// request tiếp tục từ giữa (Range: bytes=N-) khi mạng chập chờn, những lần
// đó không phải lượt tải mới.
//
//	@Summary	Tải file cài đặt (redirect sang link tạm trên storage)
//	@Tags		downloads
//	@Param		token	path	string	true	"Token trên link tải"
//	@Param		t		query	string	false	"Vé tải — bắt buộc nếu App không public (lấy từ installUrl của GET /releases)"
//	@Success	302		"Location: presigned URL, hạn 15 phút"
//	@Failure	404		{string}	string	"not found"
//	@Failure	410		{string}	string	"link unavailable"
//	@Router		/d/{token}/app.ipa [get]
//	@Router		/d/{token}/app.ipa [head]
//	@Router		/d/{token}/app.apk [get]
//	@Router		/d/{token}/app.apk [head]
func (h *DownloadHandler) File() gin.HandlerFunc {
	return func(c *gin.Context) {
		rangeHeader := c.GetHeader("Range")
		count := c.Request.Method == http.MethodGet &&
			(rangeHeader == "" || strings.HasPrefix(rangeHeader, "bytes=0-"))

		client := services.ClientInfo{IP: c.ClientIP(), UserAgent: c.Request.UserAgent()}
		url, err := h.svc.FileURL(c.Request.Context(), c.Param("token"), c.Query(ticket.QueryParam), client, count)
		if err != nil {
			h.plainError(c, err)
			return
		}

		c.Header("Cache-Control", "no-store")
		c.Redirect(http.StatusFound, url)
	}
}

// Icon godoc
//
//	@Summary	Icon của app (redirect sang link tạm trên storage)
//	@Tags		downloads
//	@Param		token	path	string	true	"Token trên link tải"
//	@Success	302		"Location: presigned URL của icon"
//	@Failure	404		{string}	string	"not found"
//	@Failure	410		{string}	string	"link unavailable"
//	@Router		/d/{token}/icon.png [get]
func (h *DownloadHandler) Icon() gin.HandlerFunc {
	return func(c *gin.Context) {
		url, err := h.svc.IconURL(c.Request.Context(), c.Param("token"))
		if err != nil {
			h.plainError(c, err)
			return
		}
		// Link tạm sống 1 giờ; cho trình duyệt nhớ redirect ngắn hơn thế.
		c.Header("Cache-Control", "private, max-age=600")
		c.Redirect(http.StatusFound, url)
	}
}

// QRCode godoc
//
//	@Summary	Mã QR trỏ tới trang tải
//	@Tags		downloads
//	@Produce	png
//	@Param		token	path		string	true	"Token trên link tải"
//	@Param		t		query		string	false	"Vé tải — bắt buộc nếu App không public (lấy từ installUrl của GET /releases)"
//	@Success	200		{file}		binary	"Ảnh PNG"
//	@Failure	404		{string}	string	"not found"
//	@Failure	410		{string}	string	"link unavailable"
//	@Router		/d/{token}/qr.png [get]
func (h *DownloadHandler) QRCode() gin.HandlerFunc {
	return func(c *gin.Context) {
		png, err := h.svc.QRCode(c.Request.Context(), c.Param("token"), c.Query(ticket.QueryParam))
		if err != nil {
			h.plainError(c, err)
			return
		}
		c.Header("Cache-Control", "private, max-age=3600")
		c.Data(http.StatusOK, "image/png", png)
	}
}

func (h *DownloadHandler) plainError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, services.ErrNotFound):
		c.String(http.StatusNotFound, "not found")
	case errors.Is(err, services.ErrUnavailable):
		c.String(http.StatusGone, "link unavailable")
	default:
		log.Error().Err(err).Str("path", c.Request.URL.Path).Msg("download request failed")
		c.String(http.StatusInternalServerError, "internal error")
	}
}
