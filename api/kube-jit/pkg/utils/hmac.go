package utils

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"go.uber.org/zap"
)

var (
	hmacKey = MustGetEnv("HMAC_SECRET") // HMAC key for signing URLs
)

// GenerateSignedURL creates a signed url with hmac key based on expiry
// It takes a base URL and an expiry time as input and returns the signed URL
// or an error if the URL cannot be generated.
var GenerateSignedURL = func(baseURL string, expiryTime time.Time) (string, error) {
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
// It takes a string as input and returns the HMAC hash as a hexadecimal string.
func GenerateHMAC(data string) string {
	key := []byte(hmacKey)
	h := hmac.New(sha256.New, key)
	h.Write([]byte(data))
	return hex.EncodeToString(h.Sum(nil))
}

// ValidateSignedURL returns true/false if a url matches the hmac sig
// It takes a signed URL as input and validates its expiry time and signature.
// It returns true if the URL is valid and not expired, false otherwise.
func ValidateSignedURL(u *url.URL, _ string) bool {
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

	// Use the actual callback URL (minus signature) for validation
	encodedURL := u.String()
	expectedSignature := GenerateHMAC(encodedURL)

	logger.Debug("Validating signed URL",
		zap.String("expectedSignature", expectedSignature),
		zap.String("providedSignature", signature),
		zap.String("signedString", encodedURL),
	)

	if !hmac.Equal([]byte(expectedSignature), []byte(signature)) {
		logger.Warn("Invalid signature in signed URL")
		return false
	}

	return true
}
