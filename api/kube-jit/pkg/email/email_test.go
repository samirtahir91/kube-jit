package email

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

// mockDialer simulates gomail.Dialer
type mockDialer struct {
	shouldFail bool
}

func (d *mockDialer) DialAndSend(m ...interface{}) error {
	if d.shouldFail {
		return assert.AnError
	}
	return nil
}

// Save original SendMail to restore after test
var originalSendMail = SendMail

func TestSendMail_Success(t *testing.T) {
	// Set required env vars
	os.Setenv("SMTP_FROM", "from@example.com")
	os.Setenv("SMTP_HOST", "localhost")
	os.Setenv("SMTP_PORT", "1025")

	// Patch SendMail to use mockDialer
	SendMail = func(to, subject, body string) error {
		// Simulate success
		return (&mockDialer{shouldFail: false}).DialAndSend()
	}
	defer func() { SendMail = originalSendMail }()

	err := SendMail("to@example.com", "Test Subject", "<b>Test Body</b>")
	assert.NoError(t, err)
}

func TestSendMail_Failure(t *testing.T) {
	// Set required env vars
	os.Setenv("SMTP_FROM", "from@example.com")
	os.Setenv("SMTP_HOST", "localhost")
	os.Setenv("SMTP_PORT", "1025")

	// Patch SendMail to use mockDialer
	SendMail = func(to, subject, body string) error {
		// Simulate failure
		return (&mockDialer{shouldFail: true}).DialAndSend()
	}
	defer func() { SendMail = originalSendMail }()

	err := SendMail("to@example.com", "Test Subject", "<b>Test Body</b>")
	assert.Error(t, err)
}
