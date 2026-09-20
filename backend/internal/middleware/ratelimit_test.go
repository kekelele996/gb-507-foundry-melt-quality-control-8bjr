package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestLimiterProtectsLoginWithoutRedis(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.POST("/api/auth/login", NewLimiter(nil, 3).Middleware(), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
	for attempt := 1; attempt <= 4; attempt++ {
		request := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
		request.RemoteAddr = "192.0.2.10:1234"
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, request)
		if attempt <= 3 && response.Code != http.StatusNoContent {
			t.Fatalf("attempt %d unexpectedly blocked with %d", attempt, response.Code)
		}
		if attempt == 4 && response.Code != http.StatusTooManyRequests {
			t.Fatalf("expected fourth attempt to be rate limited, got %d", response.Code)
		}
	}
}
