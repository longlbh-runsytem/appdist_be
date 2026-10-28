package routers

import (
	"github.com/gin-gonic/gin"

	"github.com/longlbh/appdist-be/components/appctx"
	"github.com/longlbh/appdist-be/middlewares"
	testerHandler "github.com/longlbh/appdist-be/modules/testers/handler"
	"github.com/longlbh/appdist-be/modules/testers/services"
)

// SetupTesterRoute gắn quản lý tester group (mọi role nội bộ) và phía tester
// (/tester/*, mọi tài khoản đăng nhập — kết quả lọc theo tester group của chính họ).
func SetupTesterRoute(appCtx appctx.AppContext, v1 *gin.RouterGroup, inviter services.UserInviter) {
	store := appCtx.GetStore()
	h := testerHandler.NewTesterHandler(services.NewTesterService(store, inviter))
	my := testerHandler.NewMyAppsHandler(services.NewMyAppsService(store, appCtx.GetStorage()))

	tg := v1.Group("/tester-groups", middlewares.RequireInternal(appCtx)...)
	{
		tg.POST("", h.CreateGroup())
		tg.GET("", h.ListGroups())
		tg.GET("/:id", h.GetGroup())
		tg.GET("/:id/testers", h.ListTesters())
		tg.GET("/:id/testers/deleted", h.ListDeletedTesters())
		tg.POST("/:id/testers", h.AddTesters())
		tg.PATCH("/:id/testers/:testerId/status", h.UpdateTesterStatus())
		tg.DELETE("/:id/testers/:testerId", h.DeleteTester())
		tg.POST("/:id/testers/:testerId/restore", h.RestoreTester())
	}

	t := v1.Group("/tester", middlewares.RequiredAuth(appCtx), middlewares.RequireRole(appCtx))
	{
		t.GET("/my-apps", my.ListApps())
		t.GET("/my-apps/:appId/builds", my.ListBuilds())
	}
}
