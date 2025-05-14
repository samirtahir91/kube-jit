package utils

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func init() {
	logger = zap.NewNop()
}

func TestGenerateHMAC(t *testing.T) {
	os.Setenv("HMAC_SECRET", "test-secret-key")
	hmacKey = MustGetEnv("HMAC_SECRET") // force reload if needed

	data := "some-data"
	sig := GenerateHMAC(data)
	assert.NotEmpty(t, sig)
	assert.Len(t, sig, 64) // sha256 hex string
}

func TestGenerateSignedURL_And_ValidateSignedURL(t *testing.T) {
	os.Setenv("HMAC_SECRET", "test-secret-key")
	hmacKey = MustGetEnv("HMAC_SECRET") // force reload if needed

	base := "http://localhost/callback"
	expiry := time.Now().Add(1 * time.Hour)
	signedURL, err := GenerateSignedURL(base, expiry)
	assert.NoError(t, err)
	assert.Contains(t, signedURL, "expiry=")
	assert.Contains(t, signedURL, "signature=")

	// Parse the signed URL
	u, err := url.Parse(signedURL)
	assert.NoError(t, err)

	// Should validate with correct callbackHostOverride
	valid := ValidateSignedURL(u, base)
	assert.True(t, valid)

	// Should fail if expired
	expired := time.Now().Add(-1 * time.Hour)
	expiredURL, _ := GenerateSignedURL(base, expired)
	u2, _ := url.Parse(expiredURL)
	assert.False(t, ValidateSignedURL(u2, base))

	// Should fail if signature is tampered
	u3, _ := url.Parse(signedURL)
	q := u3.Query()
	q.Set("signature", strings.Repeat("0", 64))
	u3.RawQuery = q.Encode()
	assert.False(t, ValidateSignedURL(u3, base))

	// Remove the signature from the query parameters
	query := u.Query()
	query.Del("signature")
	u.RawQuery = query.Encode()

	// Generate the expected signature
	callbackBaseURL := base + "/k8s-callback"
	u, _ = url.Parse(callbackBaseURL)
	query = u.Query()
	query.Set("expiry", fmt.Sprintf("%d", expiry.Unix()))
	u.RawQuery = query.Encode()
	encodedURL := u.String()
	expectedSignature := GenerateHMAC(encodedURL)
	assert.NotEmpty(t, expectedSignature)
}

func TestGenerateSignedURL_InvalidBase(t *testing.T) {
	os.Setenv("HMAC_SECRET", "test-secret-key")
	hmacKey = MustGetEnv("HMAC_SECRET") // force reload if needed

	_, err := GenerateSignedURL("://bad-url", time.Now())
	assert.Error(t, err)
}
