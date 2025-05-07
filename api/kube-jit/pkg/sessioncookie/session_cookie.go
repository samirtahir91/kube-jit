package sessioncookie

import (
	"encoding/json"
	"fmt"
	"kube-jit/pkg/utils"
	"net/http"
	"strings"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

const maxCookieSize = 4000                // Max size for a single cookie
const SessionPrefix = "kube_jit_session_" // Prefix for session cookies to handle split into multiple cookies

// SplitAndCombineSessionMiddleware handles splitting and combining session cookies
// It combines session data from multiple cookies into one session and splits it back into multiple cookies if necessary
// after processing the request.
// This is useful for handling large session data that exceeds the cookie size limit.
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
// into a single session object. It reads the cookies with the session prefix,
// decodes the data, and sets it in the session.
func CombineSessionData(c *gin.Context) {
	var combinedData strings.Builder

	// Iterate through cookies with the session prefix
	for i := 0; ; i++ {
		cookieName := fmt.Sprintf("%s%d", SessionPrefix, i)
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
			logger.Error("Failed to decode session data", zap.Error(err))
			return
		}

		// Check if the decoded data is valid JSON
		if !json.Valid([]byte(decodedData)) {
			logger.Error("Decoded session data is not valid JSON")
			return
		}

		// Deserialize the JSON string into a map
		var sessionData map[string]interface{}
		err = json.Unmarshal([]byte(decodedData), &sessionData)
		if err != nil {
			logger.Error("Failed to deserialize session data", zap.Error(err))
			return
		}

		// Set the combined session data in the session
		session := sessions.Default(c)
		session.Set("data", sessionData)
	}
}

// SplitSessionData splits session data into multiple cookies if necessary
// and sets them in the response. It also deletes any leftover cookies from previous sessions.
// This is useful for handling large session data that exceeds the cookie size limit.
func SplitSessionData(c *gin.Context) {
	session := sessions.Default(c)
	data := session.Get("data")
	if data == nil {
		return
	}

	// Serialize session data to JSON
	sessionDataJSON, err := json.Marshal(data)
	if err != nil {
		logger.Error("Failed to serialize session data", zap.Error(err))
		return
	}

	// Encode the session data
	encodedData, err := encodeSessionData("session_data", string(sessionDataJSON))
	if err != nil {
		logger.Error("Failed to encode session data", zap.Error(err))
		return
	}

	// Split the encoded data into smaller chunks
	chunks := splitIntoChunks(encodedData, maxCookieSize)

	// Set the cookies for each chunk
	for i, chunk := range chunks {
		cookieName := fmt.Sprintf("%s%d", SessionPrefix, i)

		sameSiteEnv := utils.GetEnv("COOKIE_SAMESITE", "Lax")
		var sameSite http.SameSite
		switch sameSiteEnv {
		case "Strict":
			sameSite = http.SameSiteStrictMode
		case "None":
			sameSite = http.SameSiteNoneMode
		default:
			sameSite = http.SameSiteLaxMode
		}

		http.SetCookie(c.Writer, &http.Cookie{
			Name:     cookieName,
			Value:    chunk,
			Path:     "/",
			HttpOnly: true,
			Secure:   true,
			MaxAge:   3600, // Set cookie expiration time
			SameSite: sameSite,
		})
	}

	// Delete any leftover cookies from previous sessions
	for i := len(chunks); ; i++ {
		cookieName := fmt.Sprintf("%s%d", SessionPrefix, i)
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
// and returns a slice of the chunks.
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
// It uses the securecookie library to encode the data into a string format.
// It returns an error if the encoding fails.
// The encoded data can be stored in a cookie.
func encodeSessionData(name string, value interface{}) (string, error) {
	encoded, err := utils.SecureCookie().Encode(name, value)
	if err != nil {
		logger.Error("Failed to encode session data", zap.Error(err))
		return "", fmt.Errorf("failed to encode session data: %v", err)
	}
	return encoded, nil
}

// decodeSessionData decodes session data using securecookie.
// It uses the securecookie library to decode the data from a string format.
// It returns an error if the decoding fails.
// The decoded data can be used to retrieve the original session data.
func decodeSessionData(name, encodedValue string, dst interface{}) error {
	err := utils.SecureCookie().Decode(name, encodedValue, dst)
	if err != nil {
		logger.Error("Failed to decode session data", zap.Error(err))
		return fmt.Errorf("failed to decode session data: %v", err)
	}
	return nil
}
