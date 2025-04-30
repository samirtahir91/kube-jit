package middleware

import (
	"encoding/gob"
	"encoding/json"
	"os"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/securecookie"
)

var cookieSecret = os.Getenv("HMAC_SECRET")
var secureCookie = securecookie.New([]byte(cookieSecret), nil)

func init() {
	// Register map[string]interface{} with gob
	gob.Register(map[string]interface{}{})

	// Increase the maximum length for securecookie since it is split into multiple cookies
	secureCookie.MaxLength(16384)
}

// SetupMiddleware sets up the middleware for the Gin engine
func SetupMiddleware(r *gin.Engine) {
	// Get allowed origins from env var ALLOW_ORIGINS
	var allowOrigins []string
	allowOriginsStr := os.Getenv("ALLOW_ORIGINS")
	if err := json.Unmarshal([]byte(allowOriginsStr), &allowOrigins); err != nil {
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
}
