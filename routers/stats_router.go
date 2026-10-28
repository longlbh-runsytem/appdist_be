package routers

import (
	"github.com/gin-gonic/gin"

	"github.com/longlbh/appdist-be/components/appctx"
	"github.com/longlbh/appdist-be/middlewares"
	statsHandler "github.com/longlbh/appdist-be/modules/stats/handler"
	"github.com/longlbh/appdist-be/modules/stats/services"
)

func SetupStatsRoute(appCtx appctx.AppContext, v1 *gin.RouterGroup) {
	h := statsHandler.NewStatsHandler(services.NewStatsService(appCtx.GetStore()))

	s := v1.Group("/stats", middlewares.RequireInternal(appCtx)...)
	{
		s.GET("/summary", h.Summary())
		s.GET("/downloads", h.Downloads())
	}
}
