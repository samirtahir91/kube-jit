package middleware

import (
	"encoding/json"
	"os"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

var cookieSecret = os.Getenv("COOKIE_SECRET")

func SetupMiddleware(r *gin.Engine) {

	// get allowed origins from env var ALLOW_ORIGINS
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

	// Session middleware
	store := cookie.NewStore([]byte(cookieSecret))
	r.Use(sessions.Sessions("mysession", store))
}
