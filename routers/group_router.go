package routers

import (
	"github.com/gin-gonic/gin"

	"github.com/longlbh/appdist-be/components/appctx"
	"github.com/longlbh/appdist-be/middlewares"
	groupHandler "github.com/longlbh/appdist-be/modules/groups/handler"
	"github.com/longlbh/appdist-be/modules/groups/services"
)

func SetupGroupRoute(appCtx appctx.AppContext, v1 *gin.RouterGroup) {
	h := groupHandler.NewGroupHandler(services.NewGroupService(appCtx.GetStore()))

	internal := middlewares.RequireRole(appCtx, middlewares.InternalRoles...)
	managers := middlewares.RequireRole(appCtx, middlewares.ManagerRoles...)

	g := v1.Group("/groups", middlewares.RequiredAuth(appCtx))
	{
		g.GET("", internal, h.List())
		g.GET("/deleted", managers, h.ListDeleted())
		g.GET("/:id", internal, h.Get())
		g.POST("", managers, h.Create())
		g.POST("/:id/members/:userId", managers, h.AddMember())
		g.DELETE("/:id/members/:userId", managers, h.RemoveMember())
		g.PATCH("/:id/status", managers, h.UpdateStatus())
		g.DELETE("/:id", managers, h.Delete())
		g.POST("/:id/restore", managers, h.Restore())
	}
}
