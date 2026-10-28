package routers

import (
	"github.com/gin-gonic/gin"

	"github.com/longlbh/appdist-be/components/appctx"
	"github.com/longlbh/appdist-be/middlewares"
	appHandler "github.com/longlbh/appdist-be/modules/apps/handler"
	"github.com/longlbh/appdist-be/modules/apps/services"
)

// SetupAppRoute gắn /apps. FE ghi /apps/:appId/tokens; gin bắt buộc cùng tên
// tham số ở cùng vị trí nên mọi route dưới /apps dùng :id — URL không đổi.
func SetupAppRoute(appCtx appctx.AppContext, v1 *gin.RouterGroup, svc *services.AppService) {
	h := appHandler.NewAppHandler(svc)

	internal := middlewares.RequireRole(appCtx, middlewares.InternalRoles...)
	managers := middlewares.RequireRole(appCtx, middlewares.ManagerRoles...)

	// Trang App công khai: KHÔNG qua RequiredAuth nên phải gắn ngoài group /apps.
	v1.GET("/apps/:id/public", h.GetPublic())

	a := v1.Group("/apps", middlewares.RequiredAuth(appCtx))
	{
		a.GET("", internal, h.List())
		a.GET("/:id", internal, h.Get())
		a.POST("", managers, h.Create())
		a.PATCH("/:id/info", managers, h.UpdateInfo())
		a.POST("/:id/managing-groups/:groupId", managers, h.AddManagingGroup())
		a.DELETE("/:id/managing-groups/:groupId", managers, h.RemoveManagingGroup())
		a.PATCH("/:id/visibility", managers, h.UpdateVisibility())
		a.PATCH("/:id/approval", managers, h.UpdateApproval())
		a.PATCH("/:id/auto-assign-tester-groups", managers, h.UpdateAutoAssign())
		a.POST("/:id/default-tester-groups/:testerGroupId", managers, h.AddDefaultTesterGroup())
		a.DELETE("/:id/default-tester-groups/:testerGroupId", managers, h.RemoveDefaultTesterGroup())

		a.POST("/:id/tokens", managers, h.CreateAPIKey())
		a.GET("/:id/tokens", managers, h.ListAPIKeys())
		a.DELETE("/:id/tokens/:tokenId", managers, h.RevokeAPIKey())
	}
}
