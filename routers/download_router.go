package routers

import (
	"github.com/gin-gonic/gin"

	"github.com/longlbh/appdist-be/components/appctx"
	"github.com/longlbh/appdist-be/components/ticket"
	"github.com/longlbh/appdist-be/middlewares"
	downloadHandler "github.com/longlbh/appdist-be/modules/downloads/handler"
	"github.com/longlbh/appdist-be/modules/downloads/services"
)

// SetupDownloadRoute gắn các URL PUBLIC dưới /d/:token mà thiết bị của khách
// gọi thẳng (manifest, file, icon, QR) — nằm ngoài /api/v1 vì iOS/Android mở
// trực tiếp — cùng API JSON /releases và /quick-share dưới /api/v1. Trang tải
// HTML do FE phục vụ.
func SetupDownloadRoute(appCtx appctx.AppContext, r *gin.Engine, v1 *gin.RouterGroup, tickets *ticket.Signer, sharer downloadHandler.QuickSharer) {
	svc := services.NewDownloadService(appCtx.GetStore(), appCtx.GetStorage(), tickets, appCtx.GetConfig())
	h := downloadHandler.NewDownloadHandler(svc, sharer, appCtx.GetConfig().MaxUploadSize())

	d := r.Group("/d/:token")
	{
		d.GET("/manifest.plist", h.Manifest())
		d.GET("/app.ipa", h.File())
		d.HEAD("/app.ipa", h.File())
		d.GET("/app.apk", h.File())
		d.HEAD("/app.apk", h.File())
		d.GET("/icon.png", h.Icon())
		d.GET("/qr.png", h.QRCode())
	}

	v1.GET("/releases/:token", middlewares.OptionalAccount(appCtx), h.Release())
	// Quick share không cần đăng nhập; có Bearer hợp lệ thì ghi lại người upload.
	v1.POST("/quick-share", middlewares.OptionalAccount(appCtx), h.QuickShare())
}
