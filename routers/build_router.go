package routers

import (
	"github.com/gin-gonic/gin"

	"github.com/longlbh/appdist-be/components/appctx"
	"github.com/longlbh/appdist-be/middlewares"
	buildHandler "github.com/longlbh/appdist-be/modules/builds/handler"
	"github.com/longlbh/appdist-be/modules/builds/services"
)

func SetupBuildRoute(appCtx appctx.AppContext, v1 *gin.RouterGroup, svc *services.BuildService, approvals *services.ApprovalService) {
	h := buildHandler.NewBuildHandler(svc, approvals, appCtx.GetConfig().MaxUploadSize())

	authed := middlewares.RequiredAuth(appCtx)
	internal := middlewares.RequireRole(appCtx, middlewares.InternalRoles...)
	managers := middlewares.RequireRole(appCtx, middlewares.ManagerRoles...)

	// CI gọi bằng X-Api-Key, web gọi bằng Bearer — cùng một endpoint.
	v1.POST("/builds/upload", middlewares.APIKeyOrBearer(appCtx), h.Upload())

	// Build của một App. Dùng :id cho khớp các route khác dưới /apps (gin bắt
	// buộc cùng tên tham số ở cùng vị trí).
	v1.GET("/apps/:id/builds", authed, internal, h.ListByApp())

	builds := v1.Group("/builds", authed)
	{
		builds.POST("/uploads", internal, h.InitUpload())
		builds.PUT("/uploads/content", internal, h.UploadContent())
		builds.POST("", internal, h.Create())
		builds.GET("", internal, h.List())
		builds.PATCH("/:id/archive", managers, h.Archive())
		builds.POST("/:id/tester-groups/:testerGroupId", internal, h.AddTesterGroup())
		builds.DELETE("/:id/tester-groups/:testerGroupId", internal, h.RemoveTesterGroup())
	}

	approval := v1.Group("/approvals", authed, managers)
	{
		approval.GET("", h.ListApprovals())
		approval.POST("/:id/review", h.Review())
	}
}
