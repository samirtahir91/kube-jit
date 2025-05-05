package utils

import (
	"os"

	"go.uber.org/zap"
)

// MustGetEnv returns the value of the environment variable or logs fatal and exits if not set
func MustGetEnv(key string) string {
	val := os.Getenv(key)
	if val == "" {
		if logger != nil {
			logger.Fatal("Missing required environment variable", zap.String("key", key))
		} else {
			panic("Missing required environment variable: " + key)
		}
	}
	return val
}

// GetEnv reads an environment variable or returns a default value if not set
func GetEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}
