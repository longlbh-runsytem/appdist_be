package middlewares

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/longlbh/appdist-be/utils"
)

func newCORSRouter(origins ...string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(CORS(utils.Config{CORSAllowedOrigins: origins}))
	r.PATCH("/api/v1/apps/:id/info", func(c *gin.Context) { c.Status(http.StatusOK) })
	return r
}

func TestCORSPreflight(t *testing.T) {
	r := newCORSRouter("http://localhost:5173")

	req := httptest.NewRequest(http.MethodOptions, "/api/v1/apps/x/info", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	req.Header.Set("Access-Control-Request-Method", "PATCH")
	req.Header.Set("Access-Control-Request-Headers", "authorization,content-type,ngrok-skip-browser-warning")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("preflight status = %d", w.Code)
	}
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Fatalf("Allow-Origin = %q", got)
	}
	if got := w.Header().Get("Access-Control-Allow-Headers"); !strings.Contains(strings.ToLower(got), "ngrok-skip-browser-warning") {
		t.Fatalf("Allow-Headers = %q, thiếu ngrok-skip-browser-warning", got)
	}
}

func TestCORSRejectsUnknownOrigin(t *testing.T) {
	r := newCORSRouter("http://localhost:5173")

	req := httptest.NewRequest(http.MethodOptions, "/api/v1/apps/x/info", nil)
	req.Header.Set("Origin", "https://evil.example")
	req.Header.Set("Access-Control-Request-Method", "PATCH")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatal("origin lạ được cho phép")
	}
}

func TestCORSWildcard(t *testing.T) {
	r := newCORSRouter("*")

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/apps/x/info", nil)
	req.Header.Set("Origin", "http://any.local:3000")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK || w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Fatalf("status=%d allow-origin=%q", w.Code, w.Header().Get("Access-Control-Allow-Origin"))
	}
}
