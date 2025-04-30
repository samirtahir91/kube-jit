package middleware

import (
	"encoding/gob"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
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

const maxCookieSize = 4000 // Max size for a single cookie
const sessionPrefix = "kube_jit_session_"

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

// SplitAndCombineSessionMiddleware handles splitting and combining session cookies
func SplitAndCombineSessionMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Combine session data from multiple cookies
		CombineSessionData(c)

		// Process the request
		c.Next()

		// Split session data into multiple cookies if necessary
		SplitSessionData(c)
	}
}

// CombineSessionData combines session data from multiple cookies
func CombineSessionData(c *gin.Context) {
	var combinedData strings.Builder

	// Iterate through cookies with the session prefix
	for i := 0; ; i++ {
		cookieName := fmt.Sprintf("%s%d", sessionPrefix, i)
		chunk, err := c.Cookie(cookieName)
		if err != nil {
			break // Stop when no more cookies are found
		}

		combinedData.WriteString(chunk)
	}

	// Decode the combined session data
	if combinedData.Len() > 0 {
		var decodedData string
		err := decodeSessionData("session_data", combinedData.String(), &decodedData)
		if err != nil {
			c.Error(fmt.Errorf("failed to decode session data: %v", err))
			return
		}

		// Check if the decoded data is valid JSON
		if !json.Valid([]byte(decodedData)) {
			c.Error(fmt.Errorf("decoded session data is not valid JSON"))
			return
		}

		// Deserialize the JSON string into a map
		var sessionData map[string]interface{}
		err = json.Unmarshal([]byte(decodedData), &sessionData)
		if err != nil {
			c.Error(fmt.Errorf("failed to deserialize session data: %v", err))
			return
		}

		// Set the combined session data in the session
		session := sessions.Default(c)
		session.Set("data", sessionData)
	}
}

// SplitSessionData splits session data into multiple cookies if necessary
func SplitSessionData(c *gin.Context) {
	session := sessions.Default(c)
	data := session.Get("data")
	if data == nil {
		return
	}

	// Serialize session data to JSON
	sessionDataJSON, err := json.Marshal(data)
	if err != nil {
		c.Error(fmt.Errorf("failed to serialize session data: %v", err))
		return
	}

	// Encode the session data
	encodedData, err := encodeSessionData("session_data", string(sessionDataJSON))
	if err != nil {
		c.Error(fmt.Errorf("failed to encode session data: %v", err))
		return
	}

	// Split the encoded data into smaller chunks
	chunks := splitIntoChunks(encodedData, maxCookieSize)

	// Set the cookies for each chunk
	for i, chunk := range chunks {
		cookieName := fmt.Sprintf("%s%d", sessionPrefix, i)

		http.SetCookie(c.Writer, &http.Cookie{
			Name:     cookieName,
			Value:    chunk,
			Path:     "/",
			HttpOnly: true,
			Secure:   true,
			MaxAge:   3600, // Set cookie expiration time
		})
	}

	// Delete any leftover cookies from previous sessions
	for i := len(chunks); ; i++ {
		cookieName := fmt.Sprintf("%s%d", sessionPrefix, i)
		_, err := c.Cookie(cookieName)
		if err != nil {
			break
		}
		http.SetCookie(c.Writer, &http.Cookie{
			Name:     cookieName,
			Value:    "",
			Path:     "/",
			HttpOnly: true,
			Secure:   true,
			MaxAge:   -1, // Delete the cookie
		})
	}
}

// splitIntoChunks splits a string into chunks of a specified size
func splitIntoChunks(data string, chunkSize int) []string {
	var chunks []string
	for len(data) > chunkSize {
		chunks = append(chunks, data[:chunkSize])
		data = data[chunkSize:]
	}
	chunks = append(chunks, data)
	return chunks
}

// encodeSessionData encodes session data using securecookie.
func encodeSessionData(name string, value interface{}) (string, error) {
	encoded, err := secureCookie.Encode(name, value)
	if err != nil {
		return "", fmt.Errorf("failed to encode session data: %v", err)
	}
	return encoded, nil
}

// decodeSessionData decodes session data using securecookie.
func decodeSessionData(name, encodedValue string, dst interface{}) error {
	err := secureCookie.Decode(name, encodedValue, dst)
	if err != nil {
		return fmt.Errorf("failed to decode session data: %v", err)
	}
	return nil
}
