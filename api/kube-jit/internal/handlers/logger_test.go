package handlers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zaptest"
)

func TestInitLoggerAndLogger(t *testing.T) {
	// Store the original logger to restore it later, ensuring test isolation
	originalLogger := logger

	defer func() {
		// Restore the original logger
		logger = originalLogger
	}()

	t.Run("initialize with a valid logger", func(t *testing.T) {
		// Create a new test logger instance
		testLoggerInstance := zaptest.NewLogger(t)

		// Initialize the package logger with the test instance
		InitLogger(testLoggerInstance)

		// Retrieve the logger using the Logger() function
		retrievedLogger := Logger()

		// Assert that the retrieved logger is the same instance we initialized with
		assert.Same(t, testLoggerInstance, retrievedLogger, "Logger() should return the logger set by InitLogger()")
		assert.NotNil(t, retrievedLogger, "Logger() should not return nil after InitLogger with a valid logger")
	})

	t.Run("initialize with nil logger", func(t *testing.T) {
		// Initialize the package logger with nil
		InitLogger(nil)

		// Retrieve the logger
		retrievedLogger := Logger()

		// Assert that the retrieved logger is nil
		assert.Nil(t, retrievedLogger, "Logger() should return nil if InitLogger was called with nil")
	})

	t.Run("logger not initialized (should be nil initially or after reset)", func(t *testing.T) {
		// Explicitly set the package logger to nil to simulate uninitialized state for this sub-test
		logger = nil

		retrievedLogger := Logger()
		assert.Nil(t, retrievedLogger, "Logger() should return nil if not initialized")
	})
}
