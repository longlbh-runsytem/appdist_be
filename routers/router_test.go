package routers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/longlbh/appdist-be/components/appctx"
	"github.com/longlbh/appdist-be/utils"
)

// TestRoutesRegister dựng toàn bộ route: gin panic ngay khi hai route xung
// đột (vd tên tham số khác nhau ở cùng vị trí), nên test này bắt lỗi đó mà
// không cần DB hay storage.
func TestRoutesRegister(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	ctx := appctx.NewAppContext(nil, nil, nil, utils.Config{TokenSymmetricKey: "12345678901234567890123456789012"})

	SetupRoute(ctx, r)

	want := map[string]bool{
		"POST /api/v1/users/create":                 false,
		"GET /api/v1/users/me":                      false,
		"GET /api/v1/users/:id":                     false,
		"GET /api/v1/users/:id/groups":              false,
		"GET /api/v1/apps/:id/builds":               false,
		"GET /api/v1/apps/:id/public":               false,
		"POST /api/v1/groups/:id/members/:userId":   false,
		"DELETE /api/v1/groups/:id/members/:userId": false,
	}
	for _, rt := range r.Routes() {
		if _, ok := want[rt.Method+" "+rt.Path]; ok {
			want[rt.Method+" "+rt.Path] = true
		}
	}
	for route, found := range want {
		if !found {
			t.Errorf("thiếu route %s", route)
		}
	}

	// Trang App công khai không đòi token (id sai định dạng → 404 chứ không phải 401).
	w0 := httptest.NewRecorder()
	r.ServeHTTP(w0, httptest.NewRequest(http.MethodGet, "/api/v1/apps/not-a-uuid/public", nil))
	if w0.Code != http.StatusNotFound {
		t.Errorf("GET /apps/x/public không token = %d, want 404", w0.Code)
	}

	// /users/me phải khớp route "me" chứ không rơi vào /users/:id.
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/users/me", nil))
	if w.Code != http.StatusUnauthorized {
		t.Errorf("GET /users/me không token = %d, want 401", w.Code)
	}
}
