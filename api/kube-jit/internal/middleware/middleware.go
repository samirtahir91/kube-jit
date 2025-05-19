package middleware

import (
	"encoding/json"
	"kube-jit/pkg/utils"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// SetupMiddleware sets up the middleware for the Gin engine
func SetupMiddleware(r *gin.Engine) {
	// Get allowed origins from env var ALLOW_ORIGINS
	var allowOrigins []string
	allowOriginsStr := utils.MustGetEnv("ALLOW_ORIGINS")
	if err := json.Unmarshal([]byte(allowOriginsStr), &allowOrigins); err != nil {
		logger.Error("Failed to parse ALLOW_ORIGINS env var", zap.Error(err))
		panic(err)
	}

	// CORS middleware
	r.Use(cors.New(cors.Config{
		AllowOrigins:     allowOrigins,
		AllowCredentials: true,
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		MaxAge:           12 * time.Hour,
	}))

	// Session middleware with custom logic
	cookieSecret := utils.MustGetEnv("HMAC_SECRET")
	store := cookie.NewStore([]byte(cookieSecret))
	r.Use(sessions.Sessions("mysession", store))

	logger.Info("Middleware setup complete", zap.Strings("allowOrigins", allowOrigins))
}
