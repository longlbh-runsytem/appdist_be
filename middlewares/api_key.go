package middlewares

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/longlbh/appdist-be/common"
	"github.com/longlbh/appdist-be/components/appctx"
	"github.com/longlbh/appdist-be/db"
	"github.com/longlbh/appdist-be/utils"
)

// APIKeyHeader là header CI/CD gửi API key của App.
const APIKeyHeader = "X-Api-Key"

const apiKeyAppKey = "api_key_app_id"

var errInvalidAPIKey = common.NewUnauthorized(
	errors.New("invalid api key"),
	"API key không hợp lệ hoặc đã bị thu hồi",
	"ErrInvalidApiKey",
)

// APIKeyOrBearer xác thực bằng X-Api-Key (CI) nếu có, ngược lại bằng Bearer
// token với role nội bộ (web).
//
// Với API key: App lấy từ key (CurrentAPIKeyApp), người upload là người đã
// tạo key — vẫn phải còn active, khoá người đó là chặn luôn key của họ.
func APIKeyOrBearer(appCtx appctx.AppContext) gin.HandlerFunc {
	store := appCtx.GetStore()
	tokenProvider := appCtx.GetTokenMaker()

	return func(c *gin.Context) {
		raw := c.GetHeader(APIKeyHeader)
		if raw == "" {
			payload, err := authenticate(c, tokenProvider)
			if err != nil {
				common.WriteError(c, err)
				return
			}
			user, err := loadActiveUser(c, store, payload.UserID)
			if err != nil {
				common.WriteError(c, err)
				return
			}
			if !slices.Contains(InternalRoles, user.Role) {
				common.WriteError(c, common.ErrNoPermission(errors.New("role not allowed")))
				return
			}
			c.Set(authPayloadKey, payload)
			c.Set(currentAccountKey, user)
			c.Next()
			return
		}

		key, err := store.GetActiveAPIKeyByHash(c.Request.Context(), utils.SHA256Hex(raw))
		if err != nil {
			if db.IsNotFound(err) {
				common.WriteError(c, errInvalidAPIKey)
				return
			}
			common.WriteError(c, common.ErrDB(err))
			return
		}

		user, err := loadActiveUser(c, store, key.CreatedBy)
		if err != nil {
			common.WriteError(c, err)
			return
		}

		// Ghi last_used_at chạy nền: lỗi ghi không đáng để chặn upload.
		go func(id uuid.UUID) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := store.TouchAPIKey(ctx, id); err != nil {
				log.Warn().Err(err).Msg("touch api key failed")
			}
		}(key.ID)

		c.Set(currentAccountKey, user)
		c.Set(apiKeyAppKey, key.AppID)
		c.Next()
	}
}

// CurrentAPIKeyApp trả App của API key, nil nếu request dùng Bearer.
func CurrentAPIKeyApp(c *gin.Context) *uuid.UUID {
	v, ok := c.Get(apiKeyAppKey)
	if !ok {
		return nil
	}
	id, ok := v.(uuid.UUID)
	if !ok {
		return nil
	}
	return &id
}
