package routers

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/longlbh/appdist-be/components/appctx"
	"github.com/longlbh/appdist-be/components/ticket"
	appServices "github.com/longlbh/appdist-be/modules/apps/services"
	buildServices "github.com/longlbh/appdist-be/modules/builds/services"
	userServices "github.com/longlbh/appdist-be/modules/users/services"
)

// SetupRoute dựng các service dùng chung giữa module MỘT LẦN rồi gắn mọi route.
//
// Trả về việc chạy nền (dọn quick share hết hạn, session hết hạn) để main
// khởi động cùng vòng đời server.
func SetupRoute(appCtx appctx.AppContext, r *gin.Engine) func(ctx context.Context) {
	store, st, cfg := appCtx.GetStore(), appCtx.GetStorage(), appCtx.GetConfig()

	inviter := userServices.NewInviter(cfg) // mời tester qua tester group
	tickets := ticket.NewSigner(cfg.TokenSymmetricKey)
	appSvc := appServices.NewAppService(store, st)
	buildSvc := buildServices.NewBuildService(store, st, appSvc, tickets, cfg)
	approvalSvc := buildServices.NewApprovalService(store, appSvc)

	v1 := r.Group("/api/v1")
	SetupUserRoute(appCtx, v1)
	SetupGroupRoute(appCtx, v1)
	SetupTesterRoute(appCtx, v1, inviter)
	SetupAppRoute(appCtx, v1, appSvc)
	SetupBuildRoute(appCtx, v1, buildSvc, approvalSvc)
	SetupStatsRoute(appCtx, v1)
	SetupDownloadRoute(appCtx, r, v1, tickets, buildSvc)

	return func(ctx context.Context) {
		buildSvc.RunCleanup(ctx, time.Hour)
	}
}
