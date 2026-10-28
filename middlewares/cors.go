package middlewares

import (
	"slices"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"github.com/longlbh/appdist-be/utils"
)

// CORS cho web FE gọi API từ domain khác.
//
// Xác thực bằng header (Bearer / X-Api-Key), không dùng cookie, nên không
// bật AllowCredentials — nhờ vậy "*" dùng được cho môi trường dev.
// Đặt trước mọi middleware khác để preflight OPTIONS được trả lời ngay,
// không bị chặn bởi middleware xác thực.
func CORS(cfg utils.Config) gin.HandlerFunc {
	c := cors.Config{
		AllowMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"},
		// ngrok-skip-browser-warning: ngrok free chặn request từ trình duyệt
		// bằng trang cảnh báo HTML (không có header CORS → trình duyệt báo lỗi
		// CORS). FE gửi header này để bỏ qua, nên preflight phải cho phép nó.
		AllowHeaders:  []string{"Authorization", "Content-Type", "X-Api-Key", "ngrok-skip-browser-warning"},
		ExposeHeaders: []string{"Content-Disposition", "Content-Length"},
		MaxAge:        12 * time.Hour,
	}
	if slices.Contains(cfg.CORSAllowedOrigins, "*") {
		c.AllowAllOrigins = true
	} else {
		c.AllowOrigins = cfg.CORSAllowedOrigins
	}
	return cors.New(c)
}
