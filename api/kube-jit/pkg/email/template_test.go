package email

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func reloadEmailLoc() {
	locName := os.Getenv("EMAIL_TIMEZONE")
	if locName == "" {
		locName = "Europe/London"
	}
	var err error
	emailLoc, err = time.LoadLocation(locName)
	if err != nil {
		emailLoc = time.UTC
	}
}

func TestBuildRequestEmail_BasicFields(t *testing.T) {
	// Set timezone for deterministic output
	os.Setenv("EMAIL_TIMEZONE", "UTC")
	reloadEmailLoc()

	details := EmailRequestDetails{
		Username:      "alice",
		ClusterName:   "prod-cluster",
		Namespaces:    []string{"ns1", "ns2"},
		RoleName:      "admin",
		Justification: "Routine maintenance",
		StartDate:     time.Date(2024, 5, 1, 10, 0, 0, 0, time.UTC),
		EndDate:       time.Date(2024, 5, 1, 18, 0, 0, 0, time.UTC),
		Status:        "approved",
		Message:       "",
	}

	html := BuildRequestEmail(details)

	assert.Contains(t, html, "alice")
	assert.Contains(t, html, "prod-cluster")
	assert.Contains(t, html, "ns1, ns2")
	assert.Contains(t, html, "admin")
	assert.Contains(t, html, "Routine maintenance")
	assert.Contains(t, html, "Approved") // Title-cased by caser
	assert.Contains(t, html, "2024-05-01 10:00 UTC")
	assert.Contains(t, html, "2024-05-01 18:00 UTC")
	assert.NotContains(t, html, "Notes:") // No message, so notes section should not appear
}

func TestBuildRequestEmail_WithMessage(t *testing.T) {
	os.Setenv("EMAIL_TIMEZONE", "UTC")
	reloadEmailLoc()

	details := EmailRequestDetails{
		Username:      "bob",
		ClusterName:   "dev-cluster",
		Namespaces:    []string{"dev"},
		RoleName:      "viewer",
		Justification: "Debugging",
		StartDate:     time.Date(2024, 6, 1, 9, 30, 0, 0, time.UTC),
		EndDate:       time.Date(2024, 6, 1, 17, 0, 0, 0, time.UTC),
		Status:        "pending",
		Message:       "Please review ASAP.",
	}

	html := BuildRequestEmail(details)

	assert.Contains(t, html, "bob")
	assert.Contains(t, html, "dev-cluster")
	assert.Contains(t, html, "dev")
	assert.Contains(t, html, "viewer")
	assert.Contains(t, html, "Debugging")
	assert.Contains(t, html, "Pending")
	assert.Contains(t, html, "2024-06-01 09:30 UTC")
	assert.Contains(t, html, "2024-06-01 17:00 UTC")
	assert.Contains(t, html, "Notes:")
	assert.Contains(t, html, "Please review ASAP.")
}

func TestBuildRequestEmail_TimezoneFallback(t *testing.T) {
	os.Setenv("EMAIL_TIMEZONE", "Invalid/Zone") // Should fallback to UTC
	reloadEmailLoc()

	details := EmailRequestDetails{
		Username:      "carol",
		ClusterName:   "qa-cluster",
		Namespaces:    []string{"qa"},
		RoleName:      "tester",
		Justification: "QA checks",
		StartDate:     time.Date(2024, 7, 1, 8, 0, 0, 0, time.UTC),
		EndDate:       time.Date(2024, 7, 1, 16, 0, 0, 0, time.UTC),
		Status:        "denied",
		Message:       "",
	}

	html := BuildRequestEmail(details)
	assert.Contains(t, html, "2024-07-01 08:00 UTC")
	assert.Contains(t, html, "2024-07-01 16:00 UTC")
}

func TestBuildRequestEmail_NamespaceJoin(t *testing.T) {
	os.Setenv("EMAIL_TIMEZONE", "UTC")
	reloadEmailLoc()

	details := EmailRequestDetails{
		Username:      "dave",
		ClusterName:   "test-cluster",
		Namespaces:    []string{"foo", "bar", "baz"},
		RoleName:      "user",
		Justification: "Testing",
		StartDate:     time.Now(),
		EndDate:       time.Now(),
		Status:        "approved",
		Message:       "",
	}
	html := BuildRequestEmail(details)
	assert.Contains(t, html, "foo, bar, baz")
	assert.True(t, strings.Contains(html, "Dave") || strings.Contains(html, "dave"))
}
