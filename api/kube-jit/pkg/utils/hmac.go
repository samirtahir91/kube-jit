package utils

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"net/url"
	"os"
	"strconv"
	"time"
)

var hmacKey = os.Getenv("HMAC_SECRET")

// GenerateSignedURL creates a signed url with hmac key based on expiry
func GenerateSignedURL(baseURL string, expiryTime time.Time) (string, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
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

// GenerateHMAC creates/returs hash string
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
		log.Printf("Failed to parse expiry time: %v\n", err)
		return false
	}

	currentTime := time.Now().Unix()
	if currentTime > expiryTime {
		log.Println("URL has expired")
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

	log.Printf("Expected signature: %s\n", expectedSignature)
	log.Printf("Provided signature: %s\n", signature)

	if !hmac.Equal([]byte(expectedSignature), []byte(signature)) {
		log.Println("Invalid signature")
		return false
	}

	return true
}
