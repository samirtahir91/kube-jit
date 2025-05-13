package handlers

import (
	"os"
	"testing"
	// "go.uber.org/zap" // Assuming getTestLogger is defined in auth_test.go or another shared file
)

// TestMain will be called before any tests in this package are run.
// It's now simplified to only handle truly package-global setup, like logging.
func TestMain(m *testing.M) {
	// 1. Initialize Logger (if this is a package-wide concern)
	testLogger := getTestLogger() // Ensure this function is available from auth_test.go or similar
	InitLogger(testLogger)

	// All provider-specific OAuth setup has been moved to respective
	// provider_test.go files (e.g., azure_test.go, github_test.go).

	// Run the tests
	exitCode := m.Run()

	os.Exit(exitCode)
}

/*
// Example getTestLogger if it needs to be defined here and isn't in auth_test.go
func getTestLogger() *zap.Logger {
    logger, _ := zap.NewDevelopment() // Or zap.NewNop() for quieter tests
    return logger
}
*/
