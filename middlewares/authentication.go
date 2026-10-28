package middlewares

import (
	"errors"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/longlbh/appdist-be/common"
	"github.com/longlbh/appdist-be/components/appctx"
	"github.com/longlbh/appdist-be/components/token"
)

// authPayloadKey là khóa lưu payload token trong gin.Context.
// Không export để chỉ có CurrentUser() đọc được, tránh mỗi nơi tự đoán tên khóa.
const authPayloadKey = "auth_payload"

func extractTokenFromHeaderString(s string) (string, error) {
	parts := strings.Fields(s)
	if len(parts) < 2 || !strings.EqualFold(parts[0], "Bearer") || strings.TrimSpace(parts[1]) == "" {
		return "", common.ErrWrongAuthHeader(nil)
	}
	return parts[1], nil
}

// RequiredAuth chặn request không có token hợp lệ và gắn payload vào context.
func RequiredAuth(appCtx appctx.AppContext) gin.HandlerFunc {
	tokenProvider := appCtx.GetTokenMaker()

	return func(c *gin.Context) {
		payload, err := authenticate(c, tokenProvider)
		if err != nil {
			common.WriteError(c, err)
			return
		}

		c.Set(authPayloadKey, payload)
		c.Next()
	}
}

// authenticate đọc và xác thực Bearer token, không đụng tới chuỗi middleware
// — để middleware khác (APIKeyOrBearer) dùng lại mà không phải gọi lồng
// RequiredAuth (nó tự c.Next(), gọi lồng sẽ chạy handler quá sớm).
func authenticate(c *gin.Context, tokenProvider token.TokenMaker) (*token.TokenPayload, error) {
	raw, err := extractTokenFromHeaderString(c.GetHeader("Authorization"))
	if err != nil {
		return nil, err
	}
	return tokenProvider.VerifyToken(raw)
}

// CurrentUser lấy payload token mà RequiredAuth đã gắn.
//
// Trả lỗi thay vì panic khi thiếu: nguyên nhân luôn là route quên gắn
// RequiredAuth, và lỗi 401 rõ ràng hơn một stack trace.
func CurrentUser(c *gin.Context) (*token.TokenPayload, error) {
	v, ok := c.Get(authPayloadKey)
	if !ok {
		return nil, common.NewUnauthorized(
			errors.New("missing auth payload"),
			"chưa đăng nhập",
			"ErrMissingAuthPayload",
		)
	}

	payload, ok := v.(*token.TokenPayload)
	if !ok {
		return nil, common.ErrInternal(errors.New("auth payload has wrong type"))
	}
	return payload, nil
}
