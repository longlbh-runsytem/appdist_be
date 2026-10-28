package routers

import (
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	"github.com/longlbh/appdist-be/components/appctx"
)

// SetupSwaggerRoute gắn Swagger UI tại /swagger/index.html. Tắt mặc định:
// production không cần phơi toàn bộ API ra ngoài, bật bằng SWAGGER_ENABLED=true.
//
// Tài liệu sinh từ annotation trên handler — chạy `make swagger` sau khi sửa.
func SetupSwaggerRoute(appCtx appctx.AppContext, r *gin.Engine) {
	if !appCtx.GetConfig().SwaggerEnabled {
		return
	}
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
}
