package sessioncookie

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSplitIntoChunks(t *testing.T) {
	data := "abcdefghijklmnopqrstuvwxyz"
	chunks := splitIntoChunks(data, 5)
	assert.Equal(t, []string{"abcde", "fghij", "klmno", "pqrst", "uvwxy", "z"}, chunks)
}

func TestEncodeDecodeSessionData(t *testing.T) {
	// Set HMAC_SECRET so utils.SecureCookie works
	os.Setenv("HMAC_SECRET", "a-valid-32-byte-hmac-secret-key")
	type testStruct struct {
		Foo string
		Bar int
	}
	val := testStruct{Foo: "baz", Bar: 42}
	encoded, err := encodeSessionData("test", val)
	assert.NoError(t, err)
	var decoded testStruct
	err = decodeSessionData("test", encoded, &decoded)
	assert.NoError(t, err)
	assert.Equal(t, val, decoded)
}

func TestSplitAndCombineLogic(t *testing.T) {
	os.Setenv("HMAC_SECRET", "a-valid-32-byte-hmac-secret-key")
	original := map[string]interface{}{"foo": "bar"}

	// Encode
	encoded, err := encodeSessionData("test", original)
	assert.NoError(t, err)

	// Split
	chunks := splitIntoChunks(encoded, 10)
	assert.True(t, len(chunks) > 0)

	// Combine
	combined := ""
	for _, chunk := range chunks {
		combined += chunk
	}

	// Decode
	var decoded map[string]interface{}
	err = decodeSessionData("test", combined, &decoded)
	assert.NoError(t, err)
	assert.Equal(t, original["foo"], decoded["foo"])
}
