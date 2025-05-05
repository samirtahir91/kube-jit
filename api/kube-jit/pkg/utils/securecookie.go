package utils

import (
	"sync"

	"github.com/gorilla/securecookie"
)

var (
	secureCookieInstance *securecookie.SecureCookie
	once                 sync.Once
)

func SecureCookie() *securecookie.SecureCookie {
	once.Do(func() {
		secret := MustGetEnv("HMAC_SECRET")
		secureCookieInstance = securecookie.New([]byte(secret), nil)
		secureCookieInstance.MaxLength(16384)
	})
	return secureCookieInstance
}
