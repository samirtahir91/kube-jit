package utils

import (
	"os"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSecureCookie_Singleton(t *testing.T) {
	os.Setenv("HMAC_SECRET", "test-secret-key")
	// Reset singleton for test
	secureCookieInstance = nil
	once = *new(sync.Once)

	sc1 := SecureCookie()
	sc2 := SecureCookie()
	assert.NotNil(t, sc1)
	assert.Equal(t, sc1, sc2, "SecureCookie should return the same instance")
}

func TestSecureCookie_EncodeDecode(t *testing.T) {
	os.Setenv("HMAC_SECRET", "test-secret-key")
	// Reset singleton for test
	secureCookieInstance = nil
	once = *new(sync.Once)

	sc := SecureCookie()
	value := map[string]interface{}{"foo": "bar"}
	encoded, err := sc.Encode("test", value)
	assert.NoError(t, err)

	var decoded map[string]interface{}
	err = sc.Decode("test", encoded, &decoded)
	assert.NoError(t, err)
	assert.Equal(t, value["foo"], decoded["foo"])
}
