package email

import (
	"fmt"
	"os"
	"strings"
	"time"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var emailLoc *time.Location

func init() {
	locName := os.Getenv("EMAIL_TIMEZONE")
	if locName == "" {
		locName = "Europe/London" // default fallback
	}
	var err error
	emailLoc, err = time.LoadLocation(locName)
	if err != nil {
		emailLoc = time.UTC // fallback to UTC if invalid
	}
}

// EmailRequestDetails holds the details for the email template
type EmailRequestDetails struct {
	Username      string
	ClusterName   string
	Namespaces    []string
	RoleName      string
	Justification string
	StartDate     time.Time
	EndDate       time.Time
	Status        string
	Message       string // extra notes or custom message
}

// BuildRequestEmail generates an HTML email template for a JIT access request
// It includes details such as the username, cluster name, namespaces, role name,
// justification, start and end dates, status, and any additional message
// The email is styled with inline CSS for better compatibility across email clients
// The function returns the generated HTML string
func BuildRequestEmail(details EmailRequestDetails) string {
	caser := cases.Title(language.English)
	startLocal := details.StartDate.In(emailLoc)
	endLocal := details.EndDate.In(emailLoc)
	return fmt.Sprintf(`
        <div style="font-family: Arial, sans-serif; max-width: 600px; margin: auto; border:1px solid #e0e0e0; border-radius:8px; overflow:hidden;">
            <div style="background: #1b4fa4; color: #fff; padding: 18px 24px;">
                <h2 style="margin:0; font-size: 1.3em;">JIT Access Request - %s</h2>
            </div>
            <div style="background: #f9f9f9; padding: 24px;">
                <p style="font-size: 1.1em; margin-bottom: 18px;">
                    Hello <b>%s</b>,
                </p>
                <p style="margin-bottom: 18px;">
                    Your request for <b>cluster:</b> %s<br>
                    <b>Namespaces:</b> %s<br>
                    <b>Role:</b> %s<br>
                    <b>Status:</b> <span style="color: #1b4fa4; font-weight: bold;">%s</span>
                </p>
                <p style="margin-bottom: 18px;">
                    <b>Justification:</b> %s<br>
                    <b>Start:</b> %s<br>
                    <b>End:</b> %s
                </p>
                %s
            </div>
            <div style="background: #f1f1f1; color: #888; font-size: 0.95em; padding: 10px 24px;">
                This is an automated notification from Kube-JIT.
            </div>
        </div>
    `,
		caser.String(details.Status),
		details.Username,
		details.ClusterName,
		strings.Join(details.Namespaces, ", "),
		details.RoleName,
		caser.String(details.Status),
		details.Justification,
		startLocal.Format("2006-01-02 15:04 MST"),
		endLocal.Format("2006-01-02 15:04 MST"),
		func() string {
			if details.Message != "" {
				return fmt.Sprintf(`<div style="margin-top: 18px; padding: 12px; background: #fffbe6; border-left: 4px solid #ffe066;"><b>Notes:</b> %s</div>`, details.Message)
			}
			return ""
		}(),
	)
}
