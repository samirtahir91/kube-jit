package utils

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetEnv_ReturnsValue(t *testing.T) {
	os.Setenv("FOO", "bar")
	defer os.Unsetenv("FOO")
	val := GetEnv("FOO", "default")
	assert.Equal(t, "bar", val)
}

func TestGetEnv_ReturnsDefault(t *testing.T) {
	os.Unsetenv("FOO")
	val := GetEnv("FOO", "default")
	assert.Equal(t, "default", val)
}

func TestMustGetEnv_ReturnsValue(t *testing.T) {
	os.Setenv("FOO", "bar")
	defer os.Unsetenv("FOO")
	val := MustGetEnv("FOO")
	assert.Equal(t, "bar", val)
}

func TestMustGetEnv_PanicsIfUnset(t *testing.T) {
	os.Unsetenv("FOO")
	logger = nil // Ensure panic, not os.Exit
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("MustGetEnv did not panic when env var was missing")
		}
	}()
	_ = MustGetEnv("FOO")
}
