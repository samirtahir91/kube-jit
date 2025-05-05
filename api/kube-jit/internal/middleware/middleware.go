package middleware

import (
	"encoding/gob"
	"encoding/json"
	"kube-jit/pkg/utils"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/securecookie"
	"go.uber.org/zap"
)

var (
	cookieSecret = utils.MustGetEnv("HMAC_SECRET")
	secureCookie = securecookie.New([]byte(cookieSecret), nil)
)

func init() {
	// Register the types for gob encoding
	gob.Register(map[string]any{})

	// Increase the maximum length for securecookie since it is split into multiple cookies
	secureCookie.MaxLength(16384)
}

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
	store := cookie.NewStore([]byte(cookieSecret))
	r.Use(sessions.Sessions("mysession", store))

	logger.Info("Middleware setup complete", zap.Strings("allowOrigins", allowOrigins))
}
