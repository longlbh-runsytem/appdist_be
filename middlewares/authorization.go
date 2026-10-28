package middlewares

import (
	"errors"
	"slices"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/longlbh/appdist-be/common"
	"github.com/longlbh/appdist-be/components/appctx"
	"github.com/longlbh/appdist-be/db"
	"github.com/longlbh/appdist-be/db/sqlc"
)

// currentAccountKey lưu bản ghi user đã nạp từ DB trong gin.Context.
const currentAccountKey = "current_account"

// InternalRoles là mọi role nội bộ — tester (người ngoài) không nằm trong đây.
var InternalRoles = []sqlc.UserRole{sqlc.UserRoleOwner, sqlc.UserRoleAdmin, sqlc.UserRoleMember}

// ManagerRoles là các role được quản lý member, nhóm, App.
var ManagerRoles = []sqlc.UserRole{sqlc.UserRoleOwner, sqlc.UserRoleAdmin}

var errAccountInactive = common.NewFullErrorResponse(
	403,
	errors.New("account inactive"),
	"tài khoản đã bị khóa",
	"account inactive",
	common.ErrNoPermissionCode,
)

// RequireRole nạp user từ DB và chặn nếu bị khoá hoặc không đúng role.
//
// Đặt SAU RequiredAuth. Đọc DB mỗi request thay vì nhét role vào token:
// admin khoá tài khoản hay hạ quyền thì có hiệu lực ngay, không phải chờ
// access token hết hạn. Không truyền role nào = chỉ cần tài khoản còn active.
func RequireRole(appCtx appctx.AppContext, roles ...sqlc.UserRole) gin.HandlerFunc {
	store := appCtx.GetStore()

	return func(c *gin.Context) {
		payload, err := CurrentUser(c)
		if err != nil {
			common.WriteError(c, err)
			return
		}

		user, err := loadActiveUser(c, store, payload.UserID)
		if err != nil {
			common.WriteError(c, err)
			return
		}

		if len(roles) > 0 && !slices.Contains(roles, user.Role) {
			common.WriteError(c, common.ErrNoPermission(errors.New("role not allowed")))
			return
		}

		c.Set(currentAccountKey, user)
		c.Next()
	}
}

// RequireInternal = RequiredAuth + RequireRole(owner, admin, member).
func RequireInternal(appCtx appctx.AppContext) []gin.HandlerFunc {
	return []gin.HandlerFunc{RequiredAuth(appCtx), RequireRole(appCtx, InternalRoles...)}
}

// OptionalAccount nạp user nếu request có Bearer token, không có thì đi tiếp
// ẩn danh. Token có mà sai/hết hạn vẫn trả 401 — để client biết mà refresh,
// thay vì âm thầm bị coi là khách.
func OptionalAccount(appCtx appctx.AppContext) gin.HandlerFunc {
	store := appCtx.GetStore()
	tokenProvider := appCtx.GetTokenMaker()

	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" {
			c.Next()
			return
		}

		raw, err := extractTokenFromHeaderString(header)
		if err != nil {
			common.WriteError(c, err)
			return
		}
		payload, err := tokenProvider.VerifyToken(raw)
		if err != nil {
			common.WriteError(c, err)
			return
		}
		user, err := loadActiveUser(c, store, payload.UserID)
		if err != nil {
			common.WriteError(c, err)
			return
		}

		c.Set(currentAccountKey, user)
		c.Next()
	}
}

func loadActiveUser(c *gin.Context, store db.Store, id uuid.UUID) (sqlc.User, error) {
	user, err := store.GetUserByID(c.Request.Context(), id)
	if err != nil {
		if db.IsNotFound(err) {
			return sqlc.User{}, common.NewUnauthorized(err, "tài khoản không tồn tại", "ErrUserNotFound")
		}
		return sqlc.User{}, common.ErrDB(err)
	}
	if user.Status != sqlc.EntityStatusActive {
		return sqlc.User{}, errAccountInactive
	}
	return user, nil
}

// CurrentAccount lấy bản ghi user mà RequireRole đã nạp.
func CurrentAccount(c *gin.Context) (sqlc.User, error) {
	v, ok := c.Get(currentAccountKey)
	if !ok {
		return sqlc.User{}, common.NewUnauthorized(
			errors.New("missing current account"),
			"chưa đăng nhập",
			"ErrMissingAuthPayload",
		)
	}

	user, ok := v.(sqlc.User)
	if !ok {
		return sqlc.User{}, common.ErrInternal(errors.New("current account has wrong type"))
	}
	return user, nil
}

// OptionalCurrentAccount trả nil nếu request ẩn danh (sau OptionalAccount).
func OptionalCurrentAccount(c *gin.Context) *sqlc.User {
	v, ok := c.Get(currentAccountKey)
	if !ok {
		return nil
	}
	user, ok := v.(sqlc.User)
	if !ok {
		return nil
	}
	return &user
}
