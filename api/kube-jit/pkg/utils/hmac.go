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
	signature := query.Get("signature")
	expiryStr := query.Get("expiry")
	if signature == "" || expiryStr == "" {
		logger.Debug("Missing signature or expiry in signed URL", zap.String("url", u.String()))
		return false
	}

	// Check expiry
	expiryInt, err := strconv.ParseInt(expiryStr, 10, 64)
	if err != nil {
		logger.Debug("Invalid expiry in signed URL", zap.String("expiry", expiryStr), zap.Error(err))
		return false
	}
	if time.Now().Unix() > expiryInt {
		logger.Debug("Signed URL expired", zap.Int64("expiry", expiryInt), zap.String("url", u.String()))
		return false
	}

	// Remove signature for validation
	query.Del("signature")
	u.RawQuery = query.Encode()
	expectedSig := GenerateHMAC(u.String())
	if !hmac.Equal([]byte(signature), []byte(expectedSig)) {
		logger.Debug("Signature mismatch in signed URL", zap.String("expected", expectedSig), zap.String("actual", signature), zap.String("url", u.String()))
		return false
	}

	logger.Debug("Signed URL validated successfully", zap.String("url", u.String()))
	return true
}
