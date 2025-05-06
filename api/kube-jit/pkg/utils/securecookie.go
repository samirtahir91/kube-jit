package utils

import (
	"sync"

	"github.com/gorilla/securecookie"
)

var (
	secureCookieInstance *securecookie.SecureCookie
	once                 sync.Once // Ensure that the SecureCookie instance is created only once
)

// SecureCookie returns a singleton instance of securecookie.SecureCookie
// It uses the HMAC_SECRET environment variable for the secret key
// and sets the maximum length for the cookie to 16384 bytes.
// Uses sync.Once to ensure that the instance is created only once.
func SecureCookie() *securecookie.SecureCookie {
	once.Do(func() {
		secret := MustGetEnv("HMAC_SECRET")
		secureCookieInstance = securecookie.New([]byte(secret), nil)
		secureCookieInstance.MaxLength(16384)
	})
	return secureCookieInstance
}
