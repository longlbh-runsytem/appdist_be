package routers

import (
	"github.com/gin-gonic/gin"

	"github.com/longlbh/appdist-be/components/appctx"
	"github.com/longlbh/appdist-be/db/sqlc"
	"github.com/longlbh/appdist-be/middlewares"
	userHandler "github.com/longlbh/appdist-be/modules/users/handler"
	"github.com/longlbh/appdist-be/modules/users/services"
)

// SetupUserRoute dựng service và handler MỘT LẦN lúc khởi động, rồi gắn route.
func SetupUserRoute(appCtx appctx.AppContext, v1 *gin.RouterGroup) {
	store := appCtx.GetStore()

	authH := userHandler.NewAuthHandler(services.NewAuthService(store, appCtx.GetTokenMaker(), appCtx.GetConfig()))
	userH := userHandler.NewUserHandler(services.NewUserService(store))

	// Không có đăng ký công khai: tài khoản do owner/admin tạo qua POST /users/create.
	// accept-invite dành cho tester (tài khoản tạo khi thêm email vào tester group).
	auth := v1.Group("/auth")
	{
		auth.POST("/login", authH.Login())
		auth.POST("/refresh", authH.RenewAccessToken())
		auth.POST("/logout", authH.Logout())
		auth.POST("/accept-invite", authH.AcceptInvite())
	}

	authed := middlewares.RequiredAuth(appCtx)
	anyRole := middlewares.RequireRole(appCtx)
	internal := middlewares.RequireRole(appCtx, middlewares.InternalRoles...)
	managers := middlewares.RequireRole(appCtx, middlewares.ManagerRoles...)
	owner := middlewares.RequireRole(appCtx, sqlc.UserRoleOwner)

	// Tài khoản (user). Thành viên nhóm (member) nằm ở /groups/:id/members.
	users := v1.Group("/users", authed)
	{
		// Mọi tài khoản đăng nhập được (kể cả tester) đều xem/đổi được của chính mình.
		// Gin ưu tiên đoạn tĩnh "me"/"deleted"/"create" trước tham số :id.
		users.GET("/me", anyRole, userH.Me())
		users.POST("/me/password", anyRole, userH.ChangePassword())

		users.GET("", internal, userH.List())
		users.GET("/deleted", managers, userH.ListDeleted())
		users.GET("/:id", internal, userH.Get())
		users.GET("/:id/groups", internal, userH.Groups())
		users.POST("/create", managers, userH.Create())
		users.PATCH("/:id/role", owner, userH.UpdateRole())
		users.PATCH("/:id/status", managers, userH.UpdateStatus())
		users.DELETE("/:id", managers, userH.Delete())
		users.POST("/:id/restore", managers, userH.Restore())
	}
}
