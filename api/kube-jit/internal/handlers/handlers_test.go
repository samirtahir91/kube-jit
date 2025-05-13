package handlers

import (
	"encoding/gob"
	"kube-jit/internal/models"
	"os"
	"testing"
)

// TestMain will be called before any tests in this package are run.
func TestMain(m *testing.M) {
	// Register for session encoding
	gob.Register(map[string]interface{}{})
	gob.Register([]models.Team{})
	gob.Register(models.Team{})

	testLogger := getTestLogger()
	InitLogger(testLogger)

	// Run the tests
	exitCode := m.Run()

	os.Exit(exitCode)
}
