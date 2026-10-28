package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	"github.com/longlbh/appdist-be/components/appctx"
	"github.com/longlbh/appdist-be/components/storage"
	"github.com/longlbh/appdist-be/components/token"
	"github.com/longlbh/appdist-be/db"
	"github.com/longlbh/appdist-be/db/connections"
	_ "github.com/longlbh/appdist-be/docs"
	"github.com/longlbh/appdist-be/middlewares"
	"github.com/longlbh/appdist-be/routers"
	"github.com/longlbh/appdist-be/utils"
)

// @title			AppDist API
// @version		1.0
// @description	Lưu file cài đặt (IPA/APK) và tạo link tải cho khách.
// @description	Mọi response JSON thành công bọc trong common.SuccessRes; lỗi trả common.AppError.
// @description
// @description	**Tài khoản test** (body mẫu của POST /api/v1/auth/login đã điền sẵn):
// @description
// @description				| Email    | tester@appdist.local |
// @description				| -------- | -------------------- |
// @description				| Mật khẩu | Test@e41ca7          |
// @BasePath					/
// @securityDefinitions.apikey	BearerAuth
// @in							header
// @name						Authorization
// @description				Nhập "Bearer <accessToken>" lấy từ POST /api/v1/auth/login.
// @securityDefinitions.apikey	ApiKeyAuth
// @in							header
// @name						X-Api-Key
// @description				API key của App (POST /api/v1/apps/{id}/tokens) — chỉ dùng cho POST /api/v1/builds/upload.
func main() {
	cfg, err := utils.LoadConfig(".", "app")
	if err != nil {
		log.Fatal().Err(err).Msg("Cannot load config")
	}

	pool, err := connections.NewPool(cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("Cannot connect DB")
	}
	defer pool.Close()

	tokenMaker, err := token.NewPasetoMaker(cfg.TokenSymmetricKey)
	if err != nil {
		log.Fatal().Err(err).Msg("Cannot create token maker")
	}

	fileStorage, err := storage.NewB2(cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("Cannot connect Backblaze B2")
	}

	appContext := appctx.NewAppContext(db.NewStore(pool), fileStorage, tokenMaker, cfg)

	r := gin.Default()
	r.Use(middlewares.CORS(cfg), middlewares.Recover())

	r.GET("/health", health)

	background := routers.SetupRoute(appContext, r)
	routers.SetupSwaggerRoute(appContext, r)

	addr := cfg.HTTPServerAddress
	if addr == "" {
		addr = "0.0.0.0:8080"
	}

	// Ctrl+C / SIGTERM: ngừng nhận request mới, chờ request đang chạy (upload
	// lớn) xong tối đa 30 giây, dừng job nền, rồi mới thoát.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go background(ctx)

	srv := &http.Server{Addr: addr, Handler: r}
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal().Err(err).Msg("Cannot start server")
		}
	}()
	log.Info().Str("addr", addr).Msg("Server started")

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("Server shutdown")
	}
}

// health godoc
//
//	@Summary	Kiểm tra server còn sống
//	@Tags		system
//	@Produce	json
//	@Success	200	{object}	map[string]string	"{"status":"ok"}"
//	@Router		/health [get]
func health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
