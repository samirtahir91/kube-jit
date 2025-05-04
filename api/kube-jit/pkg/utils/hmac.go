package utils

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"time"

	"go.uber.org/zap"
)

var (
	hmacKey = os.Getenv("HMAC_SECRET")
	logger  *zap.Logger
)

// InitLogger sets the zap logger for this package
func InitLogger(l *zap.Logger) {
	logger = l
}

// GenerateSignedURL creates a signed url with hmac key based on expiry
func GenerateSignedURL(baseURL string, expiryTime time.Time) (string, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		logger.Error("Failed to parse base URL for signed URL", zap.Error(err))
		return "", err
	}

	query := u.Query()
	query.Set("expiry", fmt.Sprintf("%d", expiryTime.Unix()))
	u.RawQuery = query.Encode()

	// Encode the URL before generating the HMAC signature
	encodedURL := u.String()
	signature := GenerateHMAC(encodedURL)

	query.Set("signature", signature)
	u.RawQuery = query.Encode()

	return u.String(), nil
}

// GenerateHMAC creates/returns hash string
func GenerateHMAC(data string) string {
	key := []byte(hmacKey)
	h := hmac.New(sha256.New, key)
	h.Write([]byte(data))
	return hex.EncodeToString(h.Sum(nil))
}

// ValidateSignedURL returns true/false if a url matches the hmac sig
func ValidateSignedURL(u *url.URL) bool {
	query := u.Query()
	expiry := query.Get("expiry")
	signature := query.Get("signature")

	// Check if the URL has expired
	expiryTime, err := strconv.ParseInt(expiry, 10, 64)
	if err != nil {
		logger.Warn("Failed to parse expiry time in signed URL", zap.Error(err))
		return false
	}

	currentTime := time.Now().Unix()
	if currentTime > expiryTime {
		logger.Warn("Signed URL has expired")
		return false
	}

	// Remove the signature from the query parameters
	query.Del("signature")
	u.RawQuery = query.Encode()

	// Generate the expected signature
	callbackBaseURL := "http://localhost:8589/kube-jit-api/k8s-callback"
	u, _ = url.Parse(callbackBaseURL)
	query = u.Query()
	query.Set("expiry", fmt.Sprintf("%d", expiryTime))
	u.RawQuery = query.Encode()
	encodedURL := u.String()
	expectedSignature := GenerateHMAC(encodedURL)

	logger.Debug("Validating signed URL",
		zap.String("expectedSignature", expectedSignature),
		zap.String("providedSignature", signature),
	)

	if !hmac.Equal([]byte(expectedSignature), []byte(signature)) {
		logger.Warn("Invalid signature in signed URL")
		return false
	}

	return true
}
