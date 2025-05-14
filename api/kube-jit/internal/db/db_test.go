package db

import (
	"os"
	"testing"

	"go.uber.org/zap"
)

func TestInitLogger(t *testing.T) {
	l, _ := zap.NewDevelopment()
	InitLogger(l)
	if logger == nil {
		t.Error("Logger was not initialized")
	}
}

func TestInitDB_MissingEnvVars(t *testing.T) {
	// Unset required env vars to simulate missing configuration
	os.Unsetenv("DB_HOST")
	os.Unsetenv("DB_USER")
	os.Unsetenv("DB_PASSWORD")
	os.Unsetenv("DB_NAME")
	os.Unsetenv("DB_PORT")
	os.Unsetenv("DB_SSLMODE")
	os.Unsetenv("DB_TIMEZONE")
	os.Unsetenv("DB_CONN_TIMEOUT")

	// Set up logger to avoid nil pointer
	l, _ := zap.NewDevelopment()
	InitLogger(l)

	defer func() {
		if r := recover(); r == nil {
			t.Error("InitDB did not panic or exit on missing env vars")
		}
	}()

	InitDB() // Should panic or exit due to missing env vars
}
